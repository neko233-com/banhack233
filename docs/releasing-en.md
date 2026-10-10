# Development and releases

banhack233 combines fail2ban / sshguard style SSH protection, host audits, keepalive, notifications, and autostart. HTML documentation and binaries are built from the same stable release tag.

## Trigger policy

| Event | Actions | HTML deployment |
| --- | --- | --- |
| Push to main or another branch | No | No |
| Open or update a PR | No | No |
| Ordinary tag or `v1.2.3-rc.1` prerelease tag | No | No |
| Push a stable `vX.Y.Z` tag | One release workflow | Only after checks, builds, and Release publication succeed |
| Edit a Release description | No | No |
| Rerun a tag older than the latest stable Release | Remaining jobs are skipped | Never replaces newer docs |

Tags must contain three non-negative integers, with no leading zeroes, prerelease suffixes, or build metadata. The tag filter limits workflow starts; a strict guard makes the final decision. There are no scheduled runs, manual dispatches, or separate documentation push workflows.

## Deployment gates

1. Validate the tag, its commit, and the latest stable Release. Reject moved tags.
2. Call reusable CI for Linux/macOS/Windows tests, Go vet, Linux race tests, ShellCheck, documentation tests, and release-guard tests.
3. Build all six Linux/macOS/Windows amd64/arm64 binaries, bilingual HTML, an offline docs archive, and checksums.
4. Recheck the tag and latest version. Create a draft Release, upload complete assets, then publish it as latest.
5. The Pages job uses `needs` to wait for successful preceding jobs, rechecks the published Release and latest tag, then deploys the HTML artifact from that same run. Rerunning only deployment repeats this check.

Failed checks or builds leave the existing Release and website unchanged. If Release publication succeeds but Pages fails, the previous website remains available: rerun the failed deployment job without recreating or moving the tag. A shared concurrency group serializes releases; push one release tag at a time.

The download latest points to the latest stable Release. The website's `version.json` records the **documentation version actually deployed**. They can temporarily differ during deployment or after a Pages failure.

## Local checks

Prerequisites: Go 1.22+, Git, Node.js for release-guard tests, actionlint, and ShellCheck. Program and documentation builds do not require Python.

```sh
go test ./...
go vet ./...
node --test .github/scripts/release-guard.test.cjs
actionlint
shellcheck scripts/*.sh build-all.sh
```

Combined Windows checks:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/validate-github-actions.ps1
```

The docs generator is a separate Go module. Test and preview:

```sh
cd tools/docs
go test ./...
go vet ./...
go run . -root ../.. -out ../../site -version dev -commit local -serve 127.0.0.1:8088
```

Open `http://127.0.0.1:8088`. Generated HTML/CSS/JS works offline, without remote fonts or runtime CDN dependencies. If clipboard access is unavailable, code is selected for manual copying.

## Publish a release

1. Update the [Chinese README](../README.md), [English README](../README-EN.md), relevant bilingual topics, and [CHANGELOG](../CHANGELOG.md). Keep the config example aligned with behavioral changes.
2. Write concrete changes and upgrade notes in `docs/releases/vX.Y.Z.md`. The workflow requires this file to avoid empty release descriptions.
3. Run local checks and push the reviewed commit to `main`. This does not trigger Actions.
4. Create an unused stable tag on that commit and push it:

```sh
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

Replace `vX.Y.Z` with a real increasing version. Do not reuse an existing tag. The workflow creates the Release after successful checks; a manually created Release is unnecessary.

Assets include the six binaries with their existing names, `banhack233-docs-vX.Y.Z.tar.gz`, and `SHA256SUMS.txt`. Install scripts keep their existing binary URLs. Extract the offline archive and open `index.html` or `en.html`.

## GitHub Pages setup

Set repository Settings → Pages → Source to **GitHub Actions**. The `github-pages` environment must permit `v*` tag deployments. Existing reviewer requirements must be satisfied before deployment.

Documentation: <https://neko233-com.github.io/banhack233/>.

- [Chinese guide](../README.md) and [English guide](../README-EN.md) are the source documents.
- Only designated documents and `configs/config.json.example` enter the site; the repository root and local configs are never uploaded.
- `site/`, `dist/`, `.local/`, real config and state files are ignored.
- Every page displays its version and commit; `version.json` also includes the build timestamp.

## Documentation maintenance

Record ordinary updates under `Unreleased`; branch commits do not update online HTML. When a stable release is ready, organize the completed changes into its changelog and nonempty release notes, then publish them together through the normal pipeline.

Add topics in both languages and register their paired `Alternate` pages in the generator's [pages list](../tools/docs/main.go), which also builds topic navigation. Use relative Markdown links; the build rewrites them to HTML and rejects unregistered `.md` targets.

Verify behavior against defaults, loading, execution paths, and tests. Keep current limits and proposed work in [architecture and limits](design-en.md); candidates must not be presented as supported features. Run documentation tests and check internal links, paired languages, desktop/mobile navigation, and code copying. Use placeholder addresses and credentials in all examples.

## Troubleshooting

- **No Actions after a main push:** expected. Push a stable tag when ready to release.
- **Skipped tag run:** check version ordering and the strict `vX.Y.Z` format.
- **Failed checks:** fix and verify locally, then publish a new version without moving an old tag.
- **Draft Release remains:** upload or publication did not complete; rerun failed jobs after fixing the cause.
- **Pages 403 or environment rejection:** inspect Pages Source, allowed deployment tags, and `pages: write` / `id-token: write` permissions.
- **Outdated website:** inspect the deployment job and website `version.json`, rather than assuming the newest Git tag is live.

References: [GitHub Pages custom workflows](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages), [Actions tag filtering](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#onpushbranchestagsbranches-ignoretags-ignore).
