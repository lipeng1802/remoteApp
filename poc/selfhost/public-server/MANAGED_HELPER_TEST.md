# 切片 4B-2：helper 自主授权状态生命周期

更新：4B-3 已完成真实双机自主链路/产品 TLS 实测，见 [双机记录](MANAGED_NETWORK_TEST.md)。下方“未更新 runner/未实机”是本切片历史；OS signer 双机完整验收仍待。

4B-1 Windows 原生安全存储与新增签名域已由用户回传 PASS、退出码 0。无需重复跑上轮相同二进制。本轮仍是私有核心开发，不更新产品安装包，不开放服务。

## 模式与安全边界

`enrollment-helper` 的可信继承管道初始化增加 Endpoint、Certificate（API TLS DER 的 canonical base64）、Issuer（独立固定 Ed25519 公钥）、GrantID（canonical 128 位 hex）。只接受现阶段回环 HTTPS，既不从响应信任新 root/issuer，也不继承代理/重定向。此配置仍来自私有 bootstrap，正式安装信任锚/轮换未实现。OS signer 配置沿用 4B-1；旧 AppKey fixture 兼容不是最终产品交付。

自主模式在 `authorize` 时比对预置 issuer/GrantID；自生成随机挑战并签名请求 `grant_clock`，验证挑战/issuer/完整 grant/保守单调经过时间。随后自行取得与指定 grant/绑定匹配的首份有效 active 状态（最多 2 秒首态等待）；父管道的 State 只提供候选 grant/绑定，不能代替 HTTPS 授权。成功后同一 helper 的 signer、HTTPS client、StateWatch 和 gate 负责持续门禁。

每 500ms 串行发 HTTP 查询，25ms 本地轮询消费已验证缓存（不增加 HTTP 频率），有效期仍是 issuer 签名的绝对期限，不按接收续期。错误/服务不可用/撤销/租期及 grant 到期/取消清除授权并永久 Deny。先 Deny，再取消/join watch；旧有效正向快照也不能恢复该 gate，无自动重连或重发 Mint。数据隧道原 250ms watchdog 保留，所以拒绝到数据关闭有有界检查延迟，不声称每字节同步查询授权。

自主模式不接父管道 `state` 更新（passed=false），拒绝 `clock`/`clock_challenge`；激活前不提供旧 token 服务，也拒绝 token probe。仅旧 fixture 模式保留这些命令。正常退出顺序是 cancel、watch Stop/join、网络关闭与 worker join、HTTP client Close、signer Close；取消时关闭自身继承 stdin 并等待 decoder，超时失败不算正常结束。

自主 join 初始化要求 Token 为空；不能混入原 fixture token。仅旧模式要求 32 字节 token。

## 本轮验证

- 本机真实 HTTPS fixture 用 Store 签发状态/时间证明并验证设备请求签名，覆盖自主刷新超过初始 lease、真实 Store 撤销、503 故障、取消、重复 Stop、陈旧租期及正在查询时取消，关闭后查询计数不增长/旧正向不能复活。
- 启动时错误 issuer/绑定、inactive 拒绝；错误时钟 issuer、部分/非法 bootstrap 配置拒绝。
- opaque 隧道集成覆盖数据可交换后服务故障关闭双向 worker、后续无数据。网络端点元数据来自私有 fixture，不是新 Headscale WhoIs 或真实 Swift/C# TLS 验收。
- 显式 `TestNativeVaultManagedHTTPSAndStop` 使用真实 Mac Keychain signer→签名 HTTPS→自主管理 gate→Stop/join，通过后精确删除随机测试身份。未设置环境变量的默认 suite 明确 SKIP native。
- Go 完整 race 回归、针对 managed 的重复竞态回归与 vet；Windows x64 helper 构建及专用 vet。最终结果记录于 HANDOFF。

本轮没有在 Windows 实机运行新的 managed 链路，没有修改/执行旧双机 Python runner。真实公网双机 OS signer、自主管理 helper/真实产品 TLS 故障关闭仍需专门集成验收；不是最终用户需要 SSH 的方案。正式信任锚、HMAC 身份包分发和 GUI/安装仍未收口。

## 开发复测

使用仓库隔离 Go 工具链/缓存执行 `go -C poc/selfhost test -race ./...`、`go -C poc/selfhost vet ./...`。本机 loopback 测试需要允许监听权限。额外设置 `REMOTEAPP_VAULT_TEST_EXECUTABLE` 为已构建并本地签名的 Mac `ConnectionIdentityVault` 绝对路径，执行 `go -C poc/selfhost test -race -p 1 ./streamgate -run TestNativeVaultManagedHTTPSAndStop -v`。不使用明文/内存替代来让 native 测试通过。
