# RemoteApp 新 Mac 交接说明

2026-11-15。新 Mac 接手控制端、构建和双机验收协调。现有已登录的 Windows 测试机继续作为目标端。

## 迁移代码

- 从远端分支 `codex/connection-architecture-poc` 克隆，以包含本说明的提交为交接起点。`80805b4` 之后的交接改动包含 TLS 转发、断流诊断、Windows 故障退出回执和工单重入门禁。
- 旧 Mac 的 `artifacts/handoff/RemoteApp-80805b4.bundle` 与 `RemoteApp-80805b4-working-tree.patch` 是此前制作的离线快照。远端交接分支已经包含该补丁中的代码，按远端分支迁移时直接检出对应提交。
- 若 Windows 端已经产生更新提交，先核对双方实际提交和差异。`docs/archive/` 是旧 Mac 的用户对话归档，可单独迁移。

在新 Mac 上将 `REMOTE_URL` 设置为已授权访问的仓库地址，再执行以下命令。

```sh
git clone -b codex/connection-architecture-poc "$REMOTE_URL" RemoteApp
cd RemoteApp
git rev-parse HEAD
git status --short --branch
```

## 迁移构建输入与现场

| 路径 | 用途 |
|---|---|
| `artifacts/connection-poc/gomod/` | Go 离线依赖。当前 TEST 构建以 `GOPROXY=off` 运行。 |
| `artifacts/connection-poc/toolchain/` | 已有 Go 1.26.6 工具链、Headscale 0.29.4 和校验文件。按新 Mac 架构核对后使用。 |
| `artifacts/connection-poc/r214-desktop-stop/` | 工单脚本固定摘要的两端 GUI 输入 `OwnedDesktopController` 与 `OwnedDesktopAgent.exe`，以及发布回执。 |
| `artifacts/connection-poc/desktop-activity-49c74444e1e4b484/` | 仍待收尾的现场。迁移到项目同一相对路径供核对。原进程、任务、安装根和控制面归属仍以原机器为准。 |
| `artifacts/connection-poc/r217-desktop-authority/` | Linux 隔离 authority 旧构建和本机测试证据。下一次部署以已审阅的最新源代码重新构建。 |

旧 `gocache`、`swift-desktop-cache` 及已退役或未安装的历史包副本已经清理。新机的编译缓存可重建。SSH 访问通过新机已授权的密钥及可信主机指纹恢复。旧 Mac Keychain 签名身份和 Windows 原生凭据各自留在原平台。

启动前运行 `uname -m`、`xcode-select -p` 和 `file artifacts/connection-poc/toolchain/go/bin/go artifacts/connection-poc/r214-desktop-stop/OwnedDesktopController`。此前实测 Mac 和两个 Mach-O 输入均为 `x86_64`。若新 Mac 为 `arm64`，先确定沿用 x86_64 验证环境还是同步适配 Mac 工具链、helper、GUI 及构建脚本的架构，重核摘要后再签包。Windows 目标仍是 amd64。用支持 TLS 1.3 和 Ed25519 证书的 Python 3，按新机实际路径选解释器。Go 重型构建和测试串行，保持 `GOMAXPROCS=2`、`GOFLAGS=-p=2`、`nice -n 10`。

## 先处理上一份双机工单

11 月 13 日的 `49c74444e1e4b484` 已通过双机 HTTPS 预检和签名安装。操作者看到了 Windows 画面，Windows 最后记录 11 帧、0 次输入，之后异常断流。目标独立撤销已确认，Mac 本地故障退出已确认。Windows 本地故障停止被明确拒绝，最后核对时任务仍在运行，缺少退出回执。隔离端口 TCP 19443 此前由该隔离服务占用，最近一次预发布 SSH 公钥认证失败，当前监听归属待复核。本机组件测试已通过，真实键鼠、持续画面和正常双端退出待验收。详细进展见 [桌面接入说明](OWNED_DESKTOP_INTEGRATION.md)。

1. 恢复新 Mac 到既有 Windows 测试机和预发布服务器的管理访问。先只读核对原工单的 Windows 任务状态、PID、拒绝原因和退出回执，以及预发布 19443 的监听者与隔离目录归属。每个阻塞只在环境变化后重查一次。
2. 依据原进程与回执查明 Windows 拒绝原因，再处理原工单的收尾。原工单过期或回执仍不明时保留现场，走明确授权的历史恢复流程。未知任务、锁、控制面和现有 Nginx/443 保持原样。
3. 确认旧隔离资源的归属和收尾，具备独立可达的入口及受信 TLS 输入后，再开一份新鲜 TEST 工单。按 [r217–r219 操作步骤](OWNED_DESKTOP_INTEGRATION.md#r219-新鲜双机工单) 完成双端各一次 HTTPS 预检、签名构建、Mac 登录桌面启动、Windows 隐藏 Interactive 启动和本机明确批准。
4. 实际验收持续画面、Windows 鼠标和键盘效果。Mac 从 GUI 计划停止，核对双端退出、独立撤销和精确退休。把直接观察及退出结果补到 `docs/HANDOFF.md`，再推进正式签名安装包、干净机器升级与卸载、跨网与耐久验证。

旧 Mac 的 `remoteapp` 自动续办已暂停。新机与 Windows 之间确定一个协调方后再恢复续办，避免两边同时操作同一工单。正式发布及签名账户操作另行安排。
