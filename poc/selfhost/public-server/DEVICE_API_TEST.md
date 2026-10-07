# 切片 2：设备签名注册与认证状态接口

本切片提供正式的设备侧请求协议、HTTPS handler/client，并用真实 Mac/Windows 客户端接真实 Headscale Driver。部署仍是私有验证，不是公开生产 API 或最终安装体验。

## 接口与信任

- `POST /v1/request`：保留原设备 Ed25519 签名注册/邀请/批准/撤销协议。稳定设备标识注册、HTTPS 签名撤销已接设备客户端；设备码交互流程属于下一切片。
- `POST /v1/network`：独立签名域绑定方法、路径、动作、时间、nonce、完整 grant、工单/挑战/注册意图。动作仅 enroll/bind/state/clock，无管理策略、回收、指定地址或操作系统输入接口。
- 必须是已注册设备签名，且身份属于服务已知完整 grant 的双方。持有 grant、IP 或工单不是调用凭据；bind 只能操作本人且属于同一 grant 的工单，enroll 要求内层意图与外层签名设备一致。第三设备/签名改动/跨角色工单拒绝。
- nonce 在特权操作前持久消费，重启不能重放；有效已注册设备的失败请求也消费。未注册或无效签名不能耗用持久 nonce/桶容量。沿用原时间 ±30 秒和状态绝对 3 秒租期，不扩大授权时间。
- 持久签名调用预算：每设备 enroll/bind 共 6 次/60 秒，状态/时间等 240 次/60 秒；已有注册/邀请协议的原预算不变。handler 最多四个在途请求，体积 8 KiB（原设备请求 4 KiB）、字段/尾随 JSON/Origin/查询参数/方法严格限制；错误只回固定 denied/unavailable/invalid_request，不暴露驱动诊断。
- 客户端只接受配置的回环 HTTPS 地址和单独可信 bootstrap 的证书根，验证真实 TLS 链/SAN，禁 HTTP、代理继承、重定向和不受限响应。无需跳过 TLS 验证，不从该端点自动获取/信任根。回复必须匹配请求 grant/挑战及动作结构；消费者继续核对独立预置 issuer 的状态/时间签名。
- 短期注册 secret 只通过 TLS 单次返回设备客户端，不落入授权账本或日志，不提供重新取回接口。响应丢失不能盲目重发 Mint，保留已有工单回收机制。

## 实机流程

新增 `cmd/device-api-client`，请求签名由 Windows/Mac 设备进程完成，不由特权服务替调用者签名。私有管道仅初始化设备测试密钥、已批准 grant 和可信证书；父测试编排触发请求并传递签名状态给 helper。enroll/bind/clock/state 已替换对应的特权 stdout RPC；策略故障、批准模拟和生命周期管理仍是私有管理员 fixture。

fixture HTTPS 只监听服务器回环，证书/私钥每轮随机生成；用 Mac→服务器本地转发及 Windows→Mac 反向回环转发访问，不新增公网端口、Nginx 路由或防火墙规则。数据 TLS 仍走独立 tsnet/Headscale/DERP，不走 SSH；SSH 隧道只是本轮控制 API 的私有测试访问方式，不是最终产品依赖。

后端重开前通过请求读写锁等待旧 handler，先停止/join 周期任务，再替换 Store/Driver/handler；不让 HTTP 操作穿越关闭/替换旧存储。HTTPS listener/证书在该持久后端重开测试中保留，未验证整个服务进程崩溃或证书轮换。

准备前两切片程序后，另编译 Darwin/Windows `cmd/device-api-client` 到 `artifacts/connection-poc/device-api-client` 和 `device-api-client-win.exe`，重建 Linux enrollment-fixture，再执行：

```sh
python3 -B poc/selfhost/public-server/enrollment-network.py --api
```

覆盖双方 HTTPS 稳定标识注册、签名单次网络注册、实际持有证明/动态 ACL、HTTPS 时间校准与刷新、真实产品 TLS 正负向、周期恢复、Store 不可用、设备 HTTPS 签名撤销/查询 inactive/关闭活动 TLS、自动回收和正常退出。Windows 上传客户端 SHA-256 核对，结束关闭两个 API 客户端与临时 SSH 转发，回收节点/用户/注册 key、恢复原 file 策略并复核业务 Nginx 哈希。不得将含 bootstrap/注册 secret 的原始管道输出打印给用户。

## 尚未交付

切片 3 已补齐设备签名 HTTPS 邀请/被控发现/确认流程、有界入口和持久注册/轮换预算、批准后 `grant_clock`；详见 [当前流程记录](DEVICE_FLOW_TEST.md)。下方“切片 3 全待完成”是历史；公开代理来源、多实例/DDoS、信任分发与 GUI 仍待交付。

2026-10-07 验收：全量 `go test -race ./...`、`go vet ./...` 通过，最终上述 `--api` 完整实机零退出。双方注册/绑定/查询及设备签名撤销均走真实 HTTPS；真实 TLS 正负向、周期失败关闭/自动恢复、持久后端重开、Store 不可用、inactive 拒绝仍有效旧正向状态及自动回收通过。两端 API 客户端正常关闭，临时转发退出，隔离 nodes/users/preauthkeys 均为 0（另行独立复核），原 file 策略恢复、服务 active、业务 Nginx 哈希一致。没有新增公网端口或改变产品安装/系统 Tailscale。后续未交付项如下。

公开注册入口的 IP/匿名 admission、防枚举、容量/滥用防护和设备码 UX（切片 3）；正式 issuer/证书信任分发、平台私钥存储、安全 IPC（切片 4）；helper 自主生命周期/自动刷新、GUI/安装集成。当前仍由父测试协调刷新和初始化，设备测试私钥/初始批准仍由 fixture 提供，不是正式设备制造/注册模式；不得公开部署当前 fixture。已有基线产品仍依赖系统 Tailscale。

Windows SSH profile 65/65 与开发探针发布复用原门禁；默认 66 项中的 DPAPI 交互式人工待验不变。
