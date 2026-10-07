# 切片 4C-3B：固定操作本机 broker 与可信设备公钥绑定

后续 4C-3C 已新增 daemon/frontend 入口、只返回 public issuer 的 identity 操作及回环联调；部署与迁移仍未执行，见 [迁移前置](AUTHORITY_MIGRATION.md)。下方保留4C-3B原验收记录。

本项是 IPC/权限边界组件与验证，**未将线上服务拆成 frontend/broker、未迁移线上 Store owner、未接 verifier/维护/Headscale 或打开网络 API**。整体切片 4 尚未完成。已部署 remoteapp-device 仍是原 device-only 回环服务。

## 权威操作

`broker.Request` 仅支持 device/network 两种 typed 签名请求；没有路径、命令、参数向量、任意脚本、Root flag 或 Key 导出。混合字段/未知操作拒绝。`Authority` 持有单一 Store，设备操作重新执行 Store 的签名/时间窗/nonce/预算/明确批准验证。网络操作要求同一 Store 的 backend 与可信 NetworkReady，缺任一返回 unavailable；进一步要求可信公钥绑定，再由 ExecuteNetworkRequest 完整验签和核对 grant 权限。

NetworkReady 是部署内部可信调用点，**不是 verifier 健康本身就能授予控制**。本轮没有 supervisor wiring，尚未证明后台故障能通过 broker fence 全部发行/状态与关闭产品 TLS；未来必须取消/fence backend，不能仅检查一次布尔值后宣称故障收口。默认组件和测试网络路径保持关闭，不提供 fixture 自动批准。

## 设备公钥

Store 的 device 新增可选 PublicKey，仅在成功的设备签名 register 后绑定并持久保存。DevicePublicKey 只返回与 deviceID SHA-256 匹配的公钥副本；未知/关闭/旧无公钥记录拒绝，不接受 frontend 断言或从 hash 反推。重开校验规范公钥/设备ID绑定，损坏拒绝。旧 snapshot 能读取，但旧设备必须明确签名 re-register 才可参与新 nodeproof；设备码保持原值，不更换 issuer。

旧程序 DisallowUnknownFields 无法读取带新 PublicKey 的 state；升级后不能把旧二进制直接覆盖当无条件回滚，也不能删公钥来绕过检查。本轮未替换线上二进制或写线上 state，部署迁移/备份/回滚流程待下一子项。

## IPC 边界

- Linux SO_PEERCRED / Mac LOCAL_PEERCRED，在解析前核对 client UID；client 同样检查 server UID。不信任 JSON UID/role、socket 名称、组成员资格或 frontend 声明。
- socket 必须位于真实绝对、broker 当前 UID 拥有、指定 GID、非 group/world writable 的目录；socket 0660、已有目标拒绝，不覆盖符号链接/文件/其他实例。未来 Linux 安装由 root 持有目录和 socket，frontend 只能组权限连接，UID 仍必须精确匹配。
- 8 个连接/4 个执行工作槽，单连接单请求、8 KiB 帧（包括包装字段）、半关闭请求端和严格 EOF，拒绝重复/大小写别名、未知字段、尾随请求、超长数据。不是 streaming JSON 或可复用授权连接。
- I/O 和工作期限，取消关闭 listener/所有连接、取消 work context 并 join 后移除 socket。受信 Executor/文件系统阻塞不能被本组件硬杀；不能把工作 context 当强制文件写取消保证。
- 回包只允许固定 HTTP-style 200/403/503 和匹配操作的 typed 结果；错误无底层明文/请求日志。成功网络返回注册凭据只用于未来受信 frontend 到既有 TLS 设备路径，不能因此打印到日志/argv。
- 不支持的平台返回 unavailable，没有 TCP/明文/未验证 peer 回退。Windows 不是这个 Linux 服务器 broker 的部署对象；Windows 平台编译/vet 通过不代表已增加 Windows broker。

## 验证

Mac 真实 Unix peer credentials + signed Store 生命周期 race 测试通过：双向错误 UID 拒绝、重复/未知/尾随/超长帧不触达 executor、原目标碰撞不覆盖、宽权限父目录拒绝、半帧取消 join/移除 socket、真实签名 register/nonce replay、公钥绑定、默认网络 unavailable、混合操作拒绝。新增 4 work slots 饱和及取消释放全部 worker/client 测试通过。

公钥回归：签名绑定、返回副本不可修改 state、重启保留、旧行不推断公钥/明确 re-register 升级、错误持久绑定拒绝。完整 Go race/vet、Linux/Windows 专用 vet 与 Linux 测试构建通过。

真实 Linux 服务器运行四个固定父测试（无 SKIP）并 PASS：KernelPeerAndStrictFrames、SignedAuthorityReplayAndNetworkClosed、SocketCollisionPermissionsAndStopPendingReader、CrossUIDBoundary。最后一项在 root 测试进程下启动 nobody 子进程：组权限确实能 Dial socket，子进程实际 non-root 且核对 server UID=0；由于 server 要求 client UID=0，该子请求被 kernel peer gate 拒绝，executor 次数仍 0。helper-only 子测试不作为独立第五项或额外产品验收。

服务器仅上传/运行测试程序，测试 issuer/设备钥匙在随机临时 fixture 内；没有读取或改线上 Store/服务配置、Headscale 节点/ACL。测试 fixture 正常 teardown，测试程序 staging 删除；Nginx 基线核对通过，device/poc 服务均 active。此次没有运行 Linux race（静态非 race 制品），race 证据来自 Mac。

## 下一步

实现固定信任配置的 authority daemon 与无特权 HTTP frontend 适配；单 Store owner 的受控停机迁移，frontend 不可读 issuer/state 或访问 Headscale DB。随后接持久 verifier、新鲜设备 nodeproof 与周期 reconcile/fault fence，在回环完成故障/拒绝/重启/零 ACL 验收后再部署启用 network。公网、安装锚/floor 轮换恢复、HMAC 安全投递、GUI/安装仍未完成。
