# 开发与发布

本项目将 fail2ban / sshguard 风格的 SSH 防爆破、系统巡检、保活、通知和自启动组合在一起。HTML 文档和程序资产来自同一个正式版本标签。

## 发布触发规则

| 操作 | 是否运行 Actions | 是否更新 HTML 文档 |
| --- | --- | --- |
| push 到 main 或其他分支 | 否 | 否 |
| 打开或更新 PR | 否 | 否 |
| 普通标签、`v1.2.3-rc.1` 预发布标签 | 否 | 否 |
| 推送正式 `vX.Y.Z` 标签 | 是，一次发布流程 | 全部检查、打包和 Release 发布成功后更新 |
| 编辑 Release 描述 | 否 | 否 |
| 重跑旧于最新正式版的标签任务 | 跳过后续任务 | 否，不能覆盖最新版 |

仅接受三段非负整数版本号，例如 `v0.1.22`；不接受前导零、预发布后缀或构建元数据。GitHub 的标签过滤器负责减少触发，工作流中的严格校验负责最终放行。没有定时任务、手动触发入口或额外的文档 push 工作流。

## 成功后才更新

1. 校验正式标签、对应提交和当前最新 Release，拒绝移动过的标签。
2. 调用可复用 CI：Linux/Windows 测试、Go vet、Linux race、ShellCheck、文档生成器测试和发布门禁测试。
3. 编译 Linux/macOS/Windows 的 amd64、arm64 六个二进制，构建中英文 HTML，并生成校验和与离线文档包。
4. 再次检查标签与最新版。先创建草稿 Release，上传完整资产后才发布为正式最新版。
5. Pages 部署任务通过 `needs` 等待前置任务成功，再核对已发布版本和最新标签，最后部署同一次运行的 HTML artifact；单独重跑部署任务也会重新校验。

测试或构建失败时，不会发布新的正式 Release，也不会替换已有网页。若 Release 已发布但 Pages 部署失败，原网页继续可用，在 Actions 中重跑失败的部署任务；不要重新创建或移动标签。不同正式发布共用并发组，避免文档部署互相覆盖；不要批量推送多个发布标签。

程序下载的 latest 指向最新正式 Release；网站 `version.json` 表示**实际成功部署的文档版本**。两者在 Pages 失败或部署期间可能暂时不同。

## 本地检查

需要 Go 1.22+、Git、Node.js（测试发布门禁）、actionlint 和 ShellCheck。程序及文档构建不依赖 Python。

```sh
go test ./...
go vet ./...
node --test .github/scripts/release-guard.test.cjs
actionlint
shellcheck scripts/*.sh build-all.sh
```

Windows 可运行组合检查：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/validate-github-actions.ps1
```

独立工具使用自己的 Go module。测试及预览文档：

```sh
cd tools/docs
go test ./...
go vet ./...
go run . -root ../.. -out ../../site -version dev -commit local -serve 127.0.0.1:8088
```

打开 `http://127.0.0.1:8088`。页面是静态 HTML/CSS/JS，无远程字体或运行时 CDN 依赖；离线打开也能阅读，浏览器不允许剪贴板时会选中代码供手动复制。

## 正式发布

1. 更新 [中文 README](../README.md)、[英文 README](../README-EN.md) 和 [CHANGELOG](../CHANGELOG.md)。涉及行为变化时同步示例配置。
2. 在 `docs/releases/vX.Y.Z.md` 写本次 Release 的具体变化与升级说明。工作流要求该文件存在，避免发布空说明。
3. 完成本地检查，将提交推送到 `main`；此时不会运行 Actions。
4. 对已审核的提交创建一个未使用的正式版本标签并推送。

```sh
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

将 `vX.Y.Z` 替换为实际递增版本，不要复用现有标签。只有步骤 4 启动发布；无需手动创建 Release，工作流会在检查通过后创建。

发布资产包括六个原有命名的二进制、`banhack233-docs-vX.Y.Z.tar.gz`、`SHA256SUMS.txt`。安装脚本继续使用原有二进制下载地址。解压离线文档包后打开 `index.html`（中文）或 `en.html`（英文）。

## GitHub Pages 设置

仓库 Settings → Pages 的 Source 设置为 **GitHub Actions**。`github-pages` environment 需要允许 `v*` 标签部署；如果仓库设置了审批人，需要满足其规则后部署才会执行。

线上文档：<https://neko233-com.github.io/banhack233/>。

- 首页：[中文说明](../README.md)；英文：[English guide](../README-EN.md)。
- 网站内容只读取预定的文档和 `configs/config.json.example`，不发布仓库根目录或本机配置。
- `site/`、`dist/`、`.local/`、真实 `config.json` 和状态文件均不提交。
- 每页显示版本与提交；`version.json` 提供版本、提交、构建时间。

## 故障处理

- **main push 没有 Actions**：这是频率策略的预期行为。需要发布时推送正式版本标签。
- **标签任务被跳过**：检查是否旧于最新正式 Release，或不是严格的 `vX.Y.Z`。
- **测试失败**：先修复并本地验证，再用新版本标签发布；不要强制移动旧标签。
- **Release 仍是草稿**：说明资产上传或发布步骤未完成，修复后重跑失败任务。
- **Pages 403 / environment 拒绝**：检查 Pages Source、标签部署规则，以及 `pages: write`、`id-token: write` 权限。
- **网页版本滞后**：检查 Pages 部署 job 与网站 `version.json`，不要仅凭最新 Git 标签判断网站版本。

设计依据：[GitHub Pages 自定义工作流](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages)、[GitHub Actions 标签过滤](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#onpushbranchestagsbranches-ignoretags-ignore)。
