# 自主入口离线安装与升级（Windows 本机切片）

日期：2026-10-26。本轮不联系服务器，不入网、不替换现有 RemoteAgent，不删除 Tailscale 或既有配对。

## 已实现

autonomousinstall.Owner 连接现有 OS 身份、trust journal 和 enrollmentstore。必须由可信安装代码驱动，提供通过既有显式 provisioning 创建的身份 scope/公钥及固定 helper；不自动创建或替换丢失的身份。

- Initialize：核对 OS 公钥，取得新的独占 trust journal，显式创建 enrollment namespace，最后提交签名 profile。旧 journal/节点拒绝覆盖；部分失败保留证据，不删库或补建。
- Upgrade：只打开现有 trust/node 独占存储，验证身份和节点，再接受编译进维护程序的新 profile。保留 idle/attempted/registered，不重置终态；拒绝较低 epoch、同 epoch 不同配置和不同根/服务。
- Inspect：不创建、不改写，要求当前 profile 与编译值完全一致且节点可验证。
- 依次释放 node、trust，再等待 signer 实际 join；关闭错误不报告 ready。Windows mutex 的打开/关闭保持同一 goroutine。
- 独立 autonomous-install 仅通过受限单帧 IPC 接收 initialize/inspect/upgrade、公开 vault scope、期望公钥。没有 argv 配置入口，不接受私钥、端点、路径或 root；返回固定 ready/failed。这是可信本机安装接口，不是网络 API。
- installed-session-build 的自主模式新增同包 autonomous-install[.exe]，与产品固定同一 release/signer digest；普通模式不变。
- 仅新 installed-autonomous-product 使用 clientowner.OpenExactRelease：持有 journal 租约时，当前 profile 必须匹配编译内容。普通产品保留原有动态 profile 轮转契约。

## Windows 实测

4 项协调回归通过：安装升级与终态保留、部分初始化不修复、错误身份/坏节点/租约冲突、未知提交保留持久门槛并要求显式重开。

TestWindowsCompiledAutonomousInstall 无 SKIP、退出码 0，原生测试耗时 13.78 秒。现场生成随机 TEST OS 身份和短期 release，编译两个 epoch 的维护 exe，再验证：未安装拒绝 inspect/upgrade、错误身份拒绝、首次创建、重复拒绝、活客户端/节点锁阻止升级、升级后 attempted 不重置、旧维护程序和旧 runtime owner 拒绝、新 runtime owner 可打开、公钥不变、精确 TEST 清理及 signer join。日志为 artifacts/connection-poc/autonomous-install-native-20261026.log。

clientowner/trustinstall/trustprofile/installed-session-build 回归、相关 go vet 通过。Windows 新自主入口及 macOS arm64 两入口编译通过；Mac 未原生运行，本轮 CGO=0 未执行 race。

从 poc/selfhost 执行原生测试：

~~~powershell
$root = 'H:\chatgpt\远程软件开发\remoteApp\artifacts\connection-poc'
$go = Join-Path $root 'toolchains\go1.26.6\go\bin\go.exe'
$env:GOENV = 'off'
$env:GOTOOLCHAIN = 'local'
$env:GOPROXY = 'off'
$env:GOPATH = Join-Path $root 'go-work'
$env:GOCACHE = Join-Path $root 'go-cache'
& $go test ./autonomousinstall -run '^TestWindowsCompiledAutonomousInstall$' -count=1 -v -native-install-helpers (Join-Path $root 'windows-preflight-20261026-a\registration') -native-install-go $go
~~~

此目录只借用已校验的 trust-journal/identity-vault 二进制，不使用旧 registration profile。测试在内存生成 fresh TEST release；失败保留 TEST 状态，不立即重跑或自动删除。

## 未完成边界与下一项

本切片不是正式安装器验收。维护程序复用既有显式 OS 身份创建步骤；身份首次创建、公开绑定回执持久保存和维护入口尚未组合成最终 GUI/Setup。部分首次安装拒绝启动并保留状态，尚无自动恢复或回滚机制。

profile/epoch 防回退不等于任意产品二进制版本防回退。r39 等缺少新门禁的历史程序不能被追溯保护；同 profile 下不同产品版本也尚无独立持久下限。正式防回滚仍需签名发行、受保护安装目录及版本迁移策略，helper 哈希不等于发行信任。

下一本机切片：可信安装 owner 衔接身份创建、公开绑定回执和安装结果，持久标记部分安装并提供只读恢复诊断；设计产品版本与 profile epoch 的独立升级门槛。Mac 原生维护、真实注册/绑定、双机 overlay/TLS/HMAC、撤销及正式 Setup/GUI 等待对应环境，不由本轮离线证据代替。
