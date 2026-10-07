# 动态注册到固定载荷活动流

`streamgate` 将 `ProvisionBackend.NetworkState(grant)` 派生的动态绑定接到现有 `session`。仅独立 PoC，不是 Swift/C# 产品 TLS，不采集屏幕、不注入输入、不进入安装包。

私有父子管道提供单独可信的 issuer 公钥、双方应用公钥和签名状态；不是由网络连接方声明身份。初始化核对签名、有效租期、完整 grant/双方身份、本机应用私钥和角色。后续状态只能刷新同一 grant/绑定，inactive 锁定不能用旧正向状态解锁；畸形/拒绝的控制更新使当前 gate 永久拒绝。初始化一次后不退回旧 token 模式。

被控连接核对实际本地/远端 overlay 地址、本机真实 tsnet Self 公钥/IP，以及 WhoIs 的对端公钥。控制端只用 `DialOnlyOverlay`，路由撤回不跟随系统网络；LocalAPI 返回本地代理 socket，必须独立核对真实 Self/IP 与显式 overlay 目的地址的 WhoIs，才将已认证的 overlay 地址提供给 session 的地址/签名 transcript 检查。不能直接将任意本机 socket 包装成可信 overlay。

session 双方新鲜 challenge/不同域 Ed25519 证明成功才传固定载荷。被控端每次响应前重核当前状态并有 250ms watchdog；控制端也有状态 watchdog。状态停止刷新、后端不可用、grant 到期、inactive 或拒绝控制更新都会关闭活动流，不自动重拨。握手仍有 3 秒截止，拨号 3 秒预算只约束拨号，成功后保持升级连接 context 直到流退出，不把活动流错误地限为 3 秒。helper 停止先取消、关闭 tsnet、等待证明/数据/探针任务，在原 8 秒预算内正常退出；超时/强杀不计通过。

## 签名时间校准

实机发现服务器比 Mac 快约 2 秒，严格的 3 秒快照因此被判断为“来自未来”。未放宽签名快照有效期，也未改系统时间。helper 用新鲜 256-bit nonce、单调时钟起点，通过私有管道取得 issuer 绑定 grant/nonce/时间的独立域签名 `ClockProof`；响应须在 5 秒以内，nonce 单次消费、issuer 与授权初始化一致，激活后不能改时钟。

应用授权时钟为“签名服务时间 + 从请求起点的完整单调经过时间”，计入整个往返而不是在接收时重新计租期。稳定服务时钟下它保守提前了请求去程，不为旧状态增加有效时间；延迟过大宁可提前拒绝。证明不是授权，快照和 grant 仍独立检查、绝对 3 秒 lease 不变。它仅用于本切片的应用授权检查；注册意图原 ±30 秒窗口、tsnet 控制 HTTPS 证书的系统时钟要求仍然存在，不宣称任意错误系统日期均能注册。

## 实机复测与边界

构建三个 enrollment 程序后运行：

```sh
python3 -B poc/selfhost/public-server/enrollment-network.py --streams
```

在原注册/实时持有证明/动态 ACL 门禁之后，验证同一连接持续刷新超过初始租期、停止刷新关闭并无载荷增长；实际关闭 durable Store 使查询拒绝且保留网络规则，状态租期结束后双端关闭，重开/Reconcile/原节点身份恢复；只撤销 grant、不撤回 ACL/删除节点，投递签名 inactive 后关闭，再显式 Reconcile 回收。恢复须明确重新启动 helper，不自动恢复已关闭连接。

校准后完整实机两轮零退出，最后一轮两端各自以 gate 的 issuer/校准时钟确认旧正向状态仍具有有效签名/未过期，然后断言它不能解除 inactive 锁定；不是因已过期才拒绝。全量 `go test -race ./...`、`go vet ./...` 通过。

状态分发和注册仍由私有管理 fixture 模拟，设备测试私钥通过私有管道提供，不能公开该流程。本轮故障是关闭真实持久 Store，不是服务器断电/网络丢包/完整 daemon 崩溃演练；周期 Reconcile、公开认证查询、设备安全存储、生产 issuer 分发和产品 TLS 桥接仍未完成。数据使用隔离 tsnet 加密，不能用它替代产品 TLS 验收。
