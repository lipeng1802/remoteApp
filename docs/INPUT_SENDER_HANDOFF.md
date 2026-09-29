# 当前增量：TLS 适配器已编写，Mac 验证优先

2026-09-29：新增固定回环 TLSInputSimulationClient / InputConnectionDriver，具备自动定时器、证书固定、串行读写和实际 transport.cancel 接线，新增 12 项测试。Mac 累计预期 96，尚未编译运行；不是实际 TLS 握手已通过。当前准确步骤以 [WORKFLOW_REVIEW.md](WORKFLOW_REVIEW.md) 为准。

以下保留上一轮状态机设计说明；其中“尚无网络适配器”及 84 项为历史状态。下一任务优先完成 Mac 验证，不继续扩展功能。

---

# 输入发送状态机交接

日期：2026-09-29。当前完成传输无关的队列/发送状态切片；尚未将 Mac 输入接入 NWConnection 或实际 TLS。此文与 P2_INPUT_HANDOFF.md 配合使用。

## 本轮实现

- InputSendQueue 默认最多 64 个待发消息，可配置 2–1024；控制消息也占容量，每条载荷最多 64 字节。队列之外最多一个正在写入的帧。
- 只替换相邻、尚未发送的鼠标移动。键盘、按钮、滚轮及心跳等消息是顺序边界；不跨越边界合并，不修改已取出的帧。
- 队列满时清空并永久停止本队列；不丢弃 Up 后继续会话。整个批次同步追加，即使中途溢出，也不留下批次的部分消息等待发送。
- AuthenticatedInputSender 复用 SessionGate、HELLO 与 HMAC-SHA256，要求本地显式许可，双方 Input 能力及认证成功后才允许 enqueue。
- poll 是唯一取帧/分配序号入口，从 1 开始且最大值回到 1。didWrite 释放唯一写入槽。每次写入至少间隔 10ms（最多 100 帧/秒），不追赶停顿期间的发送额度。
- HELLO/挑战、PONG 可先于本地写完成回调到达；控制响应等待同一写入槽。错误序号/完成编号、非法时钟、认证拒绝等使状态失败。
- 正常 finish：已接受输入 → capture.stop() 产生的 Up → DISCONNECT。finish 仅接受释放事件，之后不再接收新输入；finished 仅表示断开帧的本地写完成，不是对端确认。
- 使用调用者提供的单调时钟：认证 15 秒，单次写入 5 秒，PING 发出后等待 PONG 5 秒，正常结束排空 5 秒。认证后空闲每 5 秒安排心跳，只保留一个待确认 token。
- abort 清空待发消息，迟到的写回调不恢复会话。远端 ERROR/DISCONNECT 也标记失败，不能被误报为本地释放已送达。

**当前没有网络适配器或自动定时器。** 这些期限由调用者持续 poll/receive/didWrite 时检查；不能称已经在真实 Mac 网络会话强制执行了超时或取消。失败之后必须由适配器取消实际连接，Windows 端的断线清理仍必不可少。

## 验证证据

Windows Release 0 警告/错误，59/59 实际通过。新增 input-queue-v1.json 共享合成序列，C# 通过真实回环 TLS 发送其中 expected 序列，核对 sink 的消息顺序和断线前已释放。该测试不执行 Swift 合并算法，不代表跨语言实机通过。

Mac 新增队列 9 项、发送状态机 13 项测试，累计预期 84，尚未编译运行。其中包括一万个移动的常数容量、按键/按钮/滚轮边界、满队列替换及溢出、单写入/节奏/序号、HMAC、认证前拒绝、心跳、各期限、结束与迟到回调，以及 Swift 合并结果对照同一 JSON。没有真实事件或凭据进入测试向量。

## Mac 先执行

在已同步全部本轮代码的 Mac 仓库根目录：

~~~bash
cd macos/RemoteController
swift test --filter InputSendQueueTests
swift test --filter AuthenticatedInputSenderTests
swift test
swift build -c release
~~~

预期分别 9、13、84 项测试，0 failures，Release Build complete。新增模块仅做了 Windows 上的源码审查；Mac 编译或测试失败时先修正，不以预期数量当作结果。

随后依 P2_INPUT_HANDOFF.md 验证 InputPreview。预览工具仍同步消费本地事件，没有使用本轮发送器，不应期待看到 TLS、队列或拥塞提示。

## 下一开发切片：实际网络适配

1. 专用 mock 客户端接入 TLS 指纹验证与现有应用认证；不得通过“仅测试”绕过证书检查。
2. 同一个串行所有者处理接收、出队和写完成；定时 poll 建议每 20ms，send 后只由 contentProcessed 调用 didWrite。使用单调 uptime，不能用墙上时钟。
3. UI 到网络层也必须有界。禁止先为每个移动 dispatch 一个无限积压的闭包再进入 InputSendQueue；采用有界入口和至多一个唤醒任务。接收端用有界 ProbeFrameDecoder。
4. 捕获停止/失焦调用 capture.stop，再 finish；失败/超时/EOF 时 abort + cancel TLS，并停止捕获、清空界面状态。关闭后不能自动重放旧输入；重复或迟到回调只处理一次。
5. 用可控制延迟/失败的传输替身验证异步调度，再验证真正的 TLS mock。Windows 当前入口仍只绑定 loopback，没有供 Mac 直接跨网连接的专用输入监听。
6. 后续明确实现双机 mock 验证入口，保留身份、能力及本地许可检查；通过画面前置验收后才启用真实 SendInput。

当前 GUI 仍只读，不启动监听、不修改网络/防火墙，0.2.4 安装包未重打。本轮及此前累积修改尚未提交/推送。
