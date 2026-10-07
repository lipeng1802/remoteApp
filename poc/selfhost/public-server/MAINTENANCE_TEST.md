# 周期策略维护与恢复（切片 1）

2026-10-07：完整 `--maintenance` 实机零退出，全量 Go race/vet 与五组新增竞态测试通过；Windows SSH 协议配置 65/65、自包含开发探针发布通过。回收后节点、用户、注册 key 为空，原服务策略/业务哈希保持。

`ProvisionBackend.StartMaintenance(ctx, interval, maxBackoff)` 提供私有后端周期维护任务，不是新增公网 API 或已部署的生产守护服务。

## 生命周期与安全边界

- 启动同步关闭本 Backend/共享 Store 的签发栅栏，后台立即执行第一次完整 Reconcile；冷启动不得沿用上次就绪缓存。
- 每个 Backend 只允许一个维护任务。Reconcile 与 Enroll/Bind 及同 Store 的驱动调用继续串行化；不重叠、不积累 ticker 补跑任务。完成后才开始下一次等待。
- 周期间隔/最大退避限定 100ms–1 分钟且最大退避不小于间隔。失败等待按间隔逐次翻倍、封顶；成功回到正常间隔。已有 Reconcile 每轮驱动总预算仍为 5 秒，不放宽授权/状态租期。
- 每轮先撤回失效规则，再按持久工单精确清理、确认持久记录、最终投影并回读规则。只有完整成功才恢复就绪；清理失败保留 pending 重试。未知 Mint 结果仅回收，绝不重发凭据。
- 策略失败不签 active，仍有效的批准关系可签 inactive；旧 ACL 未必能撤回，活动 TLS 依赖签名门禁/原始 3 秒 lease 关闭，不能把命令失败当成网络拒绝证据。
- `Stop(ctx)` 取消并 join，退出再次关闭签发栅栏。返回超时不表示已退出，不可据此关闭或替换 Store/Driver。其他正在执行的管理操作仍有各自预算；不承诺任意并发情况下固定 1 秒停止。
- 周期任务状态仅返回完成周期/失败计数、最近结果、下次等待和停止标志；不是用于授权的凭证，不输出驱动原始错误/凭据。

存储损坏/关闭不是创建新身份的理由：旧任务必须停止并等待退出，明确重开同一持久 Store、重建 Backend/Driver，立即周期同步成功才恢复。授权仍有效可重新建立连接；撤销/过期授权不能复活。客户端没有自动重连，不把旧 inactive gate 解锁。

## 验证入口

按 [TLS 验收](TLS_BRIDGE_TEST.md) 准备 pinned toolchain、helper 和 Mac TLSBridgeProbe；本切片须重建 Linux `cmd/enrollment-fixture`。运行：

```sh
python3 -B poc/selfhost/public-server/enrollment-network.py --maintenance
```

该入口保留全部真实注册、动态 ACL、TLS pin/HMAC/认证负向与授权关闭门禁，并增加：后台正常周期与 TLS 刷新并行；真实 Headscale Driver 的 policy 命令失败注入；禁止 active 状态、原 ACL 保留时 TLS 关闭；恢复命令后不手工 Reconcile 而由退避任务恢复；停止旧任务后重开持久后端、启动新任务恢复；已撤销两份注册账本自动 cleaned、策略回读显式零 ACL，再正常取消/join。最后节点、用户、注册 key 清空，原 file 模式及业务 Nginx 哈希复核。

故障注入在可信 CLI runner 中拒绝 policy 命令，不是关闭整台服务器或 Headscale daemon；持久后端重开不是杀死整个 authority 进程。自然到期与清理失败重试用本机真实 Store/可控时钟验证，不能将其写成新一次公网 300 秒自然到期验收。旧切片的自然到期证据单独保留。

本机竞态测试包括策略故障/退避恢复、重复任务拒绝、停止栅栏、grant 到期后规则先撤回及清理重试、未绑定注册凭据到期自动回收、活动 driver 取消、冷 Store 重开与配置限制。默认 Windows DPAPI 人工待验仍保留；SSH profile 只运行明确的 65 项协议测试。

实机曾发现 helper 关闭后，Windows 仍可完成一个此前已读入 PING 的发送计数，而 Mac 没有收到新 PONG。新增产品开发服务会话结束计数；验收须同时确认 Mac、helper 和 C# 会话任务已退出，再断言双方计数不增长。并非忽略增长或用客户端超时代替 gate 关闭。

生产常驻服务、公开认证接口、告警/诊断、完整 daemon 崩溃/断网、容量与备份恢复仍需后续集成。本切片不改 GUI、安装包、系统 Tailscale、业务 443 或防火墙。
