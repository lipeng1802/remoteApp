# 自主连接链与桌面界面接入

2026-11-05 r204（承接 r203）。当前是桌面协议与签名权限组件接入，尚未完成 GUI 双机验收。r201 的固定 TLS 载荷验收继续有效，不重跑已退休工单。

## 已落实的边界

Mac 新增 `OwnedDesktopHandoff`，仅供固定摘要产品进程从可信 owner 的继承匿名管道接收含明确桌面权限的五字段 bootstrap。接收方固定连接 `127.0.0.1`，端口来自当前 owner 已持有的本地桥；目标设备和证书 pin 由原签名身份/描述符验证路径提供。对象只消费一次，不读写旧配对 Keychain，不执行首次证书信任，不允许 host/root/控制/剪贴板等额外选择字段，不提供密钥序列化接口，普通打印已脱敏。

wire 要求 Go `json.Marshal` 的 `Fingerprint/Key/Permission/Port/Target` 五字段规范 JSON，不含末尾换行。`Permission` 只能为 owner 已核验的 `desktop-view` 或 `desktop-control`；旧四字段或 `fixed-payload` bootstrap 一律拒绝。严格比较规范字节，拒绝重复字段、未知字段、另类数值编码、非规范 base64/小写 hex 以及空/越界端口。管道 reader 负责有界取行和终端 EOF；不能从网络、argv、配置文件或公共本机 socket 获取这个对象。**解析成功不认证发送者，也不授予自主网络访问权限。** 原 owner 必须始终持有并验证会话，目标发布证明、双向身份交换、签名 TLS 描述符、ACTIVE watch、WhoIs 与截止保持原实现。

只读适配器复用原 TLS/JPEG 实现及仅当前目标可读的内存 pin；控制适配器复用原 TLS 输入驱动，同时要求 `desktop-control` 与本机控制许可；仅查看授权不能开启输入。两个入口互斥消费，拒绝/取消后不可重新开启，不提供剪贴板回调。此许可不替代 Windows 的明确同意。`discard` 只销毁未用 handoff；已创建客户端仍必须由 GUI 生命周期明确停止，网络完成 callback 不能冒充 owner/OS 进程的完整 join。

Windows 新增 `TlsProbeServer.RunOwnedDesktopOnceAsync`，固定监听/peer 为 loopback、端口 47478，匹配已验收 `streamgate.ServeTLS` 的固定本地目的地。只服务一次，结束即释放监听，不在同一次许可下自动接下一次会话。创建 listener 前必须有本机共享许可；必须提供合法的 `DesktopSessionPermission`；输入 options 还必须同时有 Control 范围与独立本机控制许可，剪贴板读取/写入回调一律在 bind 前拒绝。仍由原 TLS/HMAC/capability gate 通过后才构造 capture/input sink，复用原释放按键/按钮及 source disposal，不增加绕过认证的捕获接口。旧 `RunFixedPayloadBridgeAsync` 仍不能接受 capture/input 回调，旧 GUI 入口也未切换。

## r204：签名权限先于原生载荷

原 authority/transport 仅签发并接受 `fixed-payload`，因此不能仅替换 TLS 产品二进制就启动真实桌面。新增 `DesktopPermission` 的 `desktop-view` / `desktop-control`，不包含剪贴板或无人值守；`VerifyGrant` 保持固定载荷限定，新 `VerifyDesktopGrant` 必须由可信本机代码提供精确 expected scope，不把 control 和 view 互相替换。

独立 `OpenDesktopAuthorization` 仅供显式的新设备授权 Store 组合使用，目前没有线上入口调用。目标邀请、控制端 claim、目标 Allow 批准均签入相同 permission；incoming/status 返回原申请范围，省略或错范围的 Allow、自批准、第三端批准、签名后改范围均拒绝。Decline 可以不声明正向范围；不会产生 grant。原120秒邀请、300秒授权、nonce、持久速率预算、同目标单活动授权、撤销规则保持。

Store 将 DesktopAuthorization 模式和每个邀请/申请范围持久化，重新打开校验 scope 与同一申请/签名 grant 一致。默认 Open 不接受新模式，新构造器也不能打开已有 fixed-only Store，失败不改快照或重建 issuer。没有线上 Store 迁移或任何旧文件回写。现有 Authorized、网络 provision/ACTIVE/TLS、业务 IPC 仍拒绝桌面授权；新的 AuthorizedDesktop 只核查设备授权账本，不开启这些网络能力。

**下一步必须先把明确 scope 贯穿独立的可信网络/会话/产品接线，再做 GUI pre-arm。** 不得将 VerifyGrant 全局改成接受任意已知 scope，也不能从网络报文计算 expected scope 来代替本机角色/许可策略。新五字段 handoff 的 scope 只能来自该已校验 grant，不是父帧自己声明就有权访问。当前真实 GUI、受信桌面产品构建、人工同意及跨机验收尚未完成。

## 下一组管理与界面接线

1. 选择独立、固定摘要的桌面产品构建入口。不得用公开 IPC 的 `mode/native/host/path` 字段把旧 PONG fixture 或业务命令升级为画面/原生输入能力。
2. 接入已有 register/invite/claim/incoming/approve/status/revoke 与私有监督协议。把经过原 owner 验证的当前申请/角色显示给 Windows 用户，明确同意关联该申请；用户拒绝不能继续 provision 或启动桌面载荷。不允许自动化脚本模拟点击来充当本机许可证据。
3. 人工等待安排在已有 `StartBoundProduct` 的 8 秒 TLS 产品启动预算**之前**。优先研究预启动并保持同一个固定摘要桌面 child 的准备/许可阶段，后续仍由 owner 交付原 bootstrap、发布证书证明并拥有该 child。不能为了等待弹窗而延长原 3 秒 TCP/密钥交换或有效期，不能将未知状态换进程重试。预启动失败、拒绝或过期也必须实际 join。
4. Windows 复用每次共享许可、物理 Ctrl+Alt+Esc 和本机停止；后台管理保持隐藏 Interactive。真实 capture/input 只在允许的用户桌面执行，默认只读。Mac 复用现有画面 canvas/焦点输入释放，使用新的一次性 handoff，禁止旧地址/配对密钥回退。
5. 接线后验证取消/断网/撤销/窗口关闭与 owner 的正常及故障收尾，严格等待同一 child、watch/node/signer/reader 实际 join。再启动一份全新签名双机工单，记录实际画面增长、输入事件与本机可见效果、拒绝不产生输入，以及双端停止/撤销。组件 JPEG/sink fixture 与 PONG 都不是这些效果的替代。

## 当前验证与证据

- Mac：`nice -n 10 swift test --package-path macos/RemoteController --scratch-path artifacts/connection-poc/swift-desktop-cache --jobs 2 --filter OwnedDesktopHandoffTests`，r204 最终 4 项通过，含 view+本机许可仍不能开启输入、旧 fixed bootstrap 拒绝。构建也编译现有 GUI 及协议模块；测试只创建未启动的控制客户端，不读取 Keychain。
- Windows：唯一随机临时源码目录仅框架依赖构建，`RemoteProtocol.Tests.dll --owned-desktop-validation`，r204 最终 6 项通过：未同意在 bind 前拒绝、view/未知范围不得启用输入、view 只发布 JPEG、control 不包含剪贴板、错误 HMAC 不创建 source/sink、认证 TLS 同时承载 fixture JPEG/键鼠并在 Disconnect 后释放 held input、dispose source 及释放监听。没有真实 capture/input、DPAPI 或持久凭据操作。
- Windows 构建 `-m:1 -p:UseSharedCompilation=false --disable-build-servers`，PowerShell 为 BelowNormal/双核 affinity；本机 Swift nice 10、jobs 2，重型构建串行。未构建新双机产品包；r204 的 Go 组件检查见下。
- r203 原证据 `artifacts/connection-poc/r203-owned-desktop/`：`swift-component.txt`、`windows-component.txt`、`component-owner.json`、`windows-component-postcheck.json`。Windows 原组件命令实际 exit0；后续只读核对测试路径相关进程为 0、测试根与两个上传文件缺席、47478 空闲。首次清理客户端因非 UTF-8 PowerShell progress 的解码异常未收回执，已通过独立只读检查解决，未重发删除。

现有 remoteapp 自动续办仍 ACTIVE。无用户操作阻塞，当前没有新鲜双机工单；历史未知 owner、根、锁及故障材料均保留。

### r204 验证增量

Go invite/installeddevice 相关包回归通过，最终新桌面范围检查与旧签名 wire golden 检查通过；空 permission 不改变旧请求签名字节或旧结果 JSON。首次扩展负向形状 fixture 触发原持久邀请轮换预算，已用各检查间推进测试时钟隔离，保留原限额，无产品规则放宽。Go 全程 GOMAXPROCS=2 / GOFLAGS=-p=2 / nice10 / 受信离线缓存；未运行无变化完整项目回归。

材料在 `artifacts/connection-poc/r204-owned-desktop/`（Go/Swift/第一轮 Windows 组件与清理）及 `r204-desktop-final-owned-desktop/`（补齐剪贴板拒绝后的最终 Windows 六项与精确清理）。两个 Windows 组件命令均实际 exit0，无真实 capture/input 或持久凭据操作；组件临时根、上传和相关进程检查通过。未准备新鲜双机产品包，不计 GUI 整链通过，未改部署服务、Nginx/443 或历史现场。
