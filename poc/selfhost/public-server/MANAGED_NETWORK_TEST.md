# 切片 4B-3：真实双机自主授权与产品 TLS

2026-10-07 `python3 -B poc/selfhost/public-server/enrollment-network.py --managed` 完整退出码 0。服务器/Windows SSH 在线；复用用户已授权的独立 Headscale/tsnet 与私有 HTTPS 入口，不改安装包或系统 Tailscale。

## 本轮实测

- 真实设备申请/明确确认、网络注册及应用持有证明、同网络身份重启、动态最小单向规则、零 ACL 真实网络拒绝、恢复均通过。
- 绑定完成后两端显式重启为自主 helper：预置 API TLS DER/独立 issuer/固定 grant ID、Token 为空，各自签名 `grant_clock` 并启动 StateWatch。父端没有 Refresh 线程/HTTP 轮询/状态投递，独立 API 客户端没有开启状态 watch。
- 通过固定 count 诊断观察两端自身 active；父端的有效 state 命令 passed=false，不影响正常连接。旧 token probe 固定拒绝但 helper 不崩溃。
- 每次 TLS 连接使用客户端生成新挑战、issuer 验签的 TLS 描述确定 pin；三项真实负向（pin、HMAC、认证前 PING）零保护 PONG。正确 Swift/C# 产品 TLS 固定载荷持续超过初始 3 秒 lease。
- 停止 helper 自己的 watch 后 watch cancel/join，双方实际 TLS 会话结束、PONG 计数不增长，不用客户端 timeout 作通过。
- 周期 policy runner 故障后保留最后 ACL，但自主查询观察 inactive 并关闭 TLS。恢复与周期重试/冷后端重开/显式重启后恢复同绑定。
- 实际关闭 Store 而不撤 ACL，两端自身 watch 终止并关闭真实 TLS；服务恢复后显式重启恢复。
- 设备签名 HTTPS 撤销后不投递父端 inactive：确认两端 managed_done 且 inactive，再将尚有效旧正向快照送入，两端均 fresh=true/passed=false；实际 TLS 已关闭，双方无新 PONG。周期回收/零规则、全部正常关闭、资源回收通过。
- Windows SSH profile 65/65，DPAPI 明确 SKIP；Go 全量竞态/静态检查、managed 两轮竞态/诊断一致性回归通过。

结束恢复原 file 策略，原 PoC 策略保持，服务 active、业务 Nginx 配置哈希一致。独立只读复核 nodes/users/preauthkeys 均 null（0）。清理仅本轮随机节点状态、短期授权/节点/用户；不操作现有生产配对密钥或用户 Tailscale。测试身份已销毁，不供下轮复用。

## 实现差异

`--managed` 在原 `--identity` 验收上复用真实申请/确认/TLS 门禁。初始注册/网络负向阶段仍使用旧隔离 fixture 模式以验证已有 token ACL 门禁；之后同节点显式重启自主模式，无 token 服务或父端状态授权。

`watch_stop` 仅可信继承私有管道操作，永久关闭本次 gate，不开放公网控制接口。`managed_active`/`managed_done` 是固定非秘密诊断，不替代签名校验或数据 watchdog。自主 `state` 命令只验证并报告 fresh，永远不调用 gate.Update；真实新状态由 helper HTTPS watch 自己投递。

## 不扩张的边界

**两端应用私钥仍由私有 fixture 提供 AppKey。** 不是双机 OS signer 全链路通过；Windows SSH 的 Credential Manager 会话限制没有解除，没有导出 OS 用户私钥或转用明文生产存储。先前 Windows 正常用户 native 和 Mac Keychain HTTPS native 是独立已通过证据。

API 根/issuer、产品 HMAC 仍来自可信 SSH fixture，API 只在服务器回环并通过 SSH 转发。这不是最终安装信任锚/HMAC 分发，也不是最终用户需要 SSH 的方案；GUI/安装包没有更新。策略故障是可信 runner 拒绝命令、服务故障是实际 Store 关闭，不是整个进程崩溃/断电。没有本轮新数据路径 direct/DERP 观测或 300 秒 grant 自然到期验收。

## 复测准备

构建并核对当前 `enrollment-helper`（Mac 和 Windows x64）、`enrollment-fixture`（Linux x64）以及已有 device-api-client、Swift TLSBridgeProbe。使用仓库隔离 Go 工具链/缓存构建；通过 `go test -race ./...` 和 `go vet ./...`。

确认隔离数据库没有其他 nodes/users，SSH/域名 HTTPS 可用，再执行 `python3 -B poc/selfhost/public-server/enrollment-network.py --managed`。该入口会临时切换隔离策略模式/创建独立节点、上传 Windows 开发探针并执行 SSH profile，结束恢复策略和精确回收；不对含其他业务节点的数据库运行。禁止公开 fixture stdout 或把它当生产入口。
