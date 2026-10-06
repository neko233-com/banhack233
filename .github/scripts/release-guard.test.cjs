'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const guard = require('./release-guard.cjs');
const notFound = () => { throw Object.assign(new Error('Not found'), { status: 404 }); };
function fixture(tag = 'v0.1.22', latest = 'v0.1.21') {
  const output = {};
  return { output, core: { setOutput: (k, v) => { output[k] = v; }, notice: () => {} },
    context: { eventName: 'push', ref: 'refs/tags/' + tag, sha: 'abc123', payload: {}, repo: { owner: 'test', repo: 'test' } },
    github: { rest: { repos: {
      getCommit: async () => ({ data: { sha: 'abc123' } }),
      getLatestRelease: async () => latest ? ({ data: { tag_name: latest } }) : notFound(),
      getReleaseByTag: async () => notFound(),
    } } },
  };
}
test('new stable release, retry, and first release are allowed', async () => {
  for (const latest of ['v0.1.21', 'v0.1.22', null]) {
    const f = fixture('v0.1.22', latest); assert.equal(await guard(f), true); assert.equal(f.output.publish, 'true');
  }
});
test('older tags never overwrite latest docs', async () => {
  const f = fixture('v0.1.9', 'v0.1.22'); assert.equal(await guard(f), false); assert.equal(f.output.publish, 'false');
});
test('numeric version order handles multi-digit components', () => {
  assert.equal(guard.compareVersions('v1.10.0', 'v1.9.9'), 1);
  assert.equal(guard.compareVersions('v10.0.0', 'v2.99.99'), 1);
});
test('prerelease, ordinary, and malformed tags are rejected', async () => {
  for (const tag of ['v1.2.3-rc.1', 'v1.2.3+build', 'docs', 'v01.2.3', 'v1.2', 'v1.2.3.4']) {
    const f = fixture(tag); await assert.rejects(guard(f), /stable release tag/); assert.equal(f.output.publish, 'false');
  }
});
test('branch pushes, tag deletion, and manual events cannot publish', async () => {
  for (const patch of [{ ref: 'refs/heads/main' }, { payload: { deleted: true } }, { eventName: 'workflow_dispatch' }]) {
    const f = fixture(); Object.assign(f.context, patch); await assert.rejects(guard(f), /Only a pushed/);
  }
});
test('moved tags and an existing prerelease fail closed', async () => {
  const f = fixture(); f.github.rest.repos.getCommit = async () => ({ data: { sha: 'different' } });
  await assert.rejects(guard(f), /Tag moved/);
  const p = fixture(); p.github.rest.repos.getReleaseByTag = async () => ({ data: { prerelease: true } });
  await assert.rejects(guard(p), /prerelease/);
});
test('API errors never masquerade as a first release', async () => {
  const f = fixture(); f.github.rest.repos.getLatestRelease = async () => { throw Object.assign(new Error('API unavailable'), { status: 503 }); };
  await assert.rejects(guard(f), /API unavailable/); assert.equal(f.output.publish, 'false');
});

test('deployment requires a published release, including failed-job retries', async () => {
  const f = fixture(); f.requirePublished = true;
  await assert.rejects(guard(f), /requires a published/);
  f.github.rest.repos.getReleaseByTag = async () => ({ data: { draft: true, prerelease: false } });
  await assert.rejects(guard(f), /requires a published/);
  f.github.rest.repos.getReleaseByTag = async () => ({ data: { draft: false, prerelease: false } });
  assert.equal(await guard(f), true);
  f.github.rest.repos.getLatestRelease = async () => ({ data: { tag_name: 'v0.1.23' } });
  assert.equal(await guard(f), false);
  assert.equal(f.output.publish, 'false');
});
