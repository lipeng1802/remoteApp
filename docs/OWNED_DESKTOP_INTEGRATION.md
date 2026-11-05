# 自主连接链与桌面界面接入

2026-11-05 r203。当前是桌面协议组件接入，尚未完成 GUI 双机验收。r201 的固定 TLS 载荷验收继续有效，不重跑已退休工单。

## 已落实的边界

Mac 新增 `OwnedDesktopHandoff`，仅供固定摘要产品进程从可信 owner 的继承匿名管道接收原四字段 bootstrap。接收方固定连接 `127.0.0.1`，端口来自当前 owner 已持有的本地桥；目标设备和证书 pin 由原签名身份/描述符验证路径提供。对象只消费一次，不读写旧配对 Keychain，不执行首次证书信任，不允许 host/root/控制/剪贴板等额外选择字段，不提供密钥序列化接口，普通打印已脱敏。

wire 是 Go `json.Marshal` 的 `Fingerprint/Key/Port/Target` 四字段规范 JSON，不含末尾换行。严格比较规范字节，拒绝重复字段、未知字段、另类数值编码、非规范 base64/小写 hex 以及空/越界端口。管道 reader 负责有界取行和终端 EOF；不能从网络、argv、配置文件或公共本机 socket 获取这个对象。**解析成功不认证发送者，也不授予自主网络访问权限。** 原 owner 必须始终持有并验证会话，目标发布证明、双向身份交换、签名 TLS 描述符、ACTIVE watch、WhoIs 与截止保持原实现。

只读适配器复用原 TLS/JPEG 实现及仅当前目标可读的内存 pin；控制适配器复用原 TLS 输入驱动并再次要求本机控制许可。两个入口互斥消费，拒绝/取消后不可重新开启，不提供剪贴板回调。此许可不替代 Windows 的明确同意。`discard` 只销毁未用 handoff；已创建客户端仍必须由 GUI 生命周期明确停止，网络完成 callback 不能冒充 owner/OS 进程的完整 join。

Windows 新增 `TlsProbeServer.RunOwnedDesktopOnceAsync`，固定监听/peer 为 loopback、端口 47478，匹配已验收 `streamgate.ServeTLS` 的固定本地目的地。只服务一次，结束即释放监听，不在同一次许可下自动接下一次会话。创建 listener 前必须有本机共享许可；输入 options 还必须有独立控制许可。仍由原 TLS/HMAC/capability gate 通过后才构造 capture/input sink，复用原释放按键/按钮及 source disposal，不增加绕过认证的捕获接口。旧 `RunFixedPayloadBridgeAsync` 仍不能接受 capture/input 回调，旧 GUI 入口也未切换。

## 下一组管理与界面接线

1. 选择独立、固定摘要的桌面产品构建入口。不得用公开 IPC 的 `mode/native/host/path` 字段把旧 PONG fixture 或业务命令升级为画面/原生输入能力。
2. 接入已有 register/invite/claim/incoming/approve/status/revoke 与私有监督协议。把经过原 owner 验证的当前申请/角色显示给 Windows 用户，明确同意关联该申请；用户拒绝不能继续 provision 或启动桌面载荷。不允许自动化脚本模拟点击来充当本机许可证据。
3. 人工等待安排在已有 `StartBoundProduct` 的 8 秒 TLS 产品启动预算**之前**。优先研究预启动并保持同一个固定摘要桌面 child 的准备/许可阶段，后续仍由 owner 交付原 bootstrap、发布证书证明并拥有该 child。不能为了等待弹窗而延长原 3 秒 TCP/密钥交换或有效期，不能将未知状态换进程重试。预启动失败、拒绝或过期也必须实际 join。
4. Windows 复用每次共享许可、物理 Ctrl+Alt+Esc 和本机停止；后台管理保持隐藏 Interactive。真实 capture/input 只在允许的用户桌面执行，默认只读。Mac 复用现有画面 canvas/焦点输入释放，使用新的一次性 handoff，禁止旧地址/配对密钥回退。
5. 接线后验证取消/断网/撤销/窗口关闭与 owner 的正常及故障收尾，严格等待同一 child、watch/node/signer/reader 实际 join。再启动一份全新签名双机工单，记录实际画面增长、输入事件与本机可见效果、拒绝不产生输入，以及双端停止/撤销。组件 JPEG/sink fixture 与 PONG 都不是这些效果的替代。

## 当前验证与证据

- Mac：`nice -n 10 swift test --package-path macos/RemoteController --scratch-path artifacts/connection-poc/swift-desktop-cache --jobs 2 --filter OwnedDesktopHandoffTests`，3 项通过。构建也编译现有 GUI 及协议模块；测试只创建未启动的控制客户端，不读取 Keychain。
- Windows：唯一随机临时源码目录仅框架依赖构建，`RemoteProtocol.Tests.dll --owned-desktop-validation`，3 项通过：未同意在 bind 前拒绝、错误 HMAC 不创建 source/sink、认证 TLS 同时承载 fixture JPEG/键鼠并在 Disconnect 后释放 held input、dispose source 及释放监听。没有真实 capture/input、DPAPI 或持久凭据操作。
- Windows 构建 `-m:1 -p:UseSharedCompilation=false --disable-build-servers`，PowerShell 为 BelowNormal/双核 affinity；本机 Swift nice 10、jobs 2，重型构建串行。没有运行 Go 构建或新双机产品工单。
- 证据 `artifacts/connection-poc/r203-owned-desktop/`：`swift-component.txt`、`windows-component.txt`、`component-owner.json`、`windows-component-postcheck.json`。Windows 原组件命令实际 exit0；后续只读核对测试路径相关进程为 0、测试根与两个上传文件缺席、47478 空闲。首次清理客户端因非 UTF-8 PowerShell progress 的解码异常未收回执，已通过独立只读检查解决，未重发删除。

现有 remoteapp 自动续办仍 ACTIVE。无用户操作阻塞，当前没有新鲜双机工单；历史未知 owner、根、锁及故障材料均保留。
