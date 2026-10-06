'use strict';
const stable = /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
function compareVersions(a, b) {
  if (!stable.test(a) || !stable.test(b)) throw new Error('Expected stable vX.Y.Z versions');
  const left = a.slice(1).split('.').map(BigInt), right = b.slice(1).split('.').map(BigInt);
  for (let i = 0; i < 3; i++) { if (left[i] !== right[i]) return left[i] > right[i] ? 1 : -1; }
  return 0;
}
async function guard({ github, context, core, requirePublished = false }) {
  core.setOutput('publish', 'false');
  if (context.eventName !== 'push' || !context.ref.startsWith('refs/tags/') || context.payload.deleted) throw new Error('Only a pushed stable release tag can publish');
  const tag = context.ref.slice('refs/tags/'.length);
  if (!stable.test(tag)) throw new Error(`Not a stable release tag: ${tag}`);
  const commit = await github.rest.repos.getCommit({ ...context.repo, ref: tag });
  if (commit.data.sha !== context.sha) throw new Error('Tag moved after this workflow started; refusing publication');
  try {
    const latest = await github.rest.repos.getLatestRelease(context.repo);
    if (compareVersions(tag, latest.data.tag_name) < 0) {
      core.notice(`${tag} is older than ${latest.data.tag_name}; keeping the current release and documentation`);
      return false;
    }
  } catch (error) { if (error.status !== 404) throw error; }
  try {
    const release = await github.rest.repos.getReleaseByTag({ ...context.repo, tag });
    if (release.data.prerelease) throw new Error('This tag belongs to a prerelease; refusing to promote it automatically');
    if (requirePublished && release.data.draft) throw new Error('Documentation requires a published Release');
  } catch (error) {
    if (error.status !== 404) throw error;
    if (requirePublished) throw new Error('Documentation requires a published Release');
  }
  core.setOutput('publish', 'true');
  return true;
}
module.exports = guard;
module.exports.compareVersions = compareVersions;
