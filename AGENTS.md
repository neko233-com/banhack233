caveman full. 精简、专业但完整的答复

项目文档规范：

- README.md 顶部必须先给安装命令；安装不能藏在中段。
- README.md 必须保留一行安装：
  `curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.sh | sh`
- README.md 需要覆盖：功能说明、命令演示、配置示例、飞书/邮箱通知、SSH root 密码登录、24h SSH 保活、TCP 长连接 keepalive、dry_run 到生产切换、排障、开发/发布。
- 不要把业务软件端口（Redis/MySQL 等）当默认问题写进文档；本项目只管主机、SSH、防爆破、通知、系统连接保活。
- root SSH 登录是受支持场景，不要写成必须禁止 root。
