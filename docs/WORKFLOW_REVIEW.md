# 当前流程审查与 Mac 接收检查点

2026-09-30 最新：双机 Tailscale input mock 已实现，但尚未在 Windows 编译或双机运行。Windows 仅内存 sink，Mac 仅固定合成序列；两端限制 Tailscale IPv4，复用已批准指纹/配对密钥，要求显式本地许可且单次会话。Mac 104/104 和 Release 已通过；Windows 预期 60/60，待按 `TAILSCALE_INPUT_MOCK.md` 验收。产品 GUI 和 `SendInput` 仍未接入。

2026-09-30 更新：第 2 个网络检查点已在 Mac 本机收口。真实 NWListener/NWConnection TLS 专项 5/5，正确指纹+密钥可认证并排空输入，错误指纹/密钥、半帧断线、取消均失败关闭；慢写入保留确定性替身覆盖。全量 103/103，Release `Build complete! (25.48s)`。回环限制、无 GUI 接线、无 SendInput 的边界不变。

下一检查点为执行已实现的双机 Tailscale input mock：Windows Release/60 项测试先通过，再确认双端 PASS、12 个合成事件一致、全部释放且 Windows 桌面没有真实动作；通过后才能评估产品查看器接线。

## 2026-09-30 Mac 自动检查结果

用户实际运行结果：`InputSendQueueTests` 9/9、`AuthenticatedInputSenderTests` 13/13、`InputConnectionDriverTests` 12/12、全量 `swift test` 96/96 均通过；Release 构建成功，`Build complete! (18.76s)`。因此下文“尚未在 Mac 编译”的描述保留为执行前历史背景。当前应继续 InputPreview 本地人工清单；这些自动结果不能替代真实 AppKit 交互或 NWConnection TLS 正负向验证。

后续 InputPreview 实机发现同类左右修饰键被聚合。修复物理 keyCode 事件状态并新增 2 项测试后，当前全量为 98/98，Release `Build complete! (18.30s)`；黑边、拖出、滚轮、失焦及既有键鼠/停止均已通过，只剩修饰键新版实机复测。端口 0 显式拒绝也在本轮全量重跑中修复。

用户已确认修饰键新版实机复测通过，InputPreview 人工清单全部完成。当前流程进入第 2 个网络检查点：在受控 mock 环境验证真实 NWConnection TLS 的正确/错误指纹与密钥、慢写入、半帧断线及取消；现有回环限制继续保留，尚未授权跨网输入或真实 SendInput。

日期：2026-09-29。用户要求检查是否偏离并继续下一步。

## 审查结论

产品方向没有偏离：Mac 控制 Windows，先用 Tailscale 验证基础远程桌面，再追求接近 AnyDesk 的独立体验；没有提前开发 H.264、自建穿透/中继、账户服务或真实输入注入。

流程存在验证积压，需要收口：Mac 最后实际通过的是 40 项测试及当时的 Release；之后的新增代码均未在 Mac 编译。Windows 回环 TLS 和 JSON 回放不能替代 Swift 执行。P1 的 GDI 真实采集复验、画质与跨网 30 分钟也尚未完成。因此不能把 P1/P2 记作阶段完成，也不应持续扩展新的 Mac 产品功能。

本轮补齐既定的 TLS 模拟适配器及调度测试后，将下面的 Mac 检查点作为下一优先任务。Mac 仍不可用时，只继续 Windows 采集诊断、已有代码修复和文档整理；不要继续增加新的 P2 功能、开放跨网输入监听或接 SendInput。

## 已有实现与待验证范围

| 模块 | 当前证据 | 尚缺的结果 |
|---|---|---|
| P0 / TLS / HMAC / JPEG 基线 | 历史双机认证及 JPEG 显示通过；Mac 历史 40 项通过 | 当前增量 Release 和兼容性回归 |
| Windows 协议、mock 输入/TLS | 本轮重新构建 0 警告/错误，59/59 通过 | 真实双机输入 mock，不等于已能控制桌面 |
| Mac 鼠标/键盘协议与键位 | 源码、共享向量；新增 4+3+5 项 | Mac 编译和执行 |
| Mac 输入采集 / InputPreview | 源码及新增 10 项 | 窗口焦点、左右修饰键、拖出边界、滚轮、停止 |
| Mac 有界队列 / 认证发送状态 | 源码及新增 9+13 项；Windows 回放合并后向量通过 | Swift 合并、认证/心跳/取消的实际运行 |
| Mac TLS 模拟适配 / 调度 | 本轮源码及新增 12 项 | Swift 编译、替身测试、真实 NWConnection TLS 正负向链路 |
| P1 画质 / 稳定性 | 用户历史约 9.4 FPS，三档画质包 0.2.4 | GDI StretchBlt 失败复验、画质、至少 10 FPS / 30 分钟 |
| 安装包 | 既有只读 0.2.4 已生成 | 新版本安装回归、无预装 .NET 环境；本轮未重打 |

Mac 当前预期总数为 **96**（40 + 56），不是已通过 96 项。代码仍为未提交/推送的累积修改；先安全同步，不能仅让 Mac pull 旧远程分支。

## 本轮 TLS 适配器

TLSInputSimulationClient 是库 API，固定 127.0.0.1，只用于显式模拟；没有 host 参数、CLI 或 GUI 接线。构造时要求预先人工核对的证书 SHA-256 指纹、设备密钥及 localControlAllowed=true；使用 TLS 1.2+，复用证书固定策略，认证前不接受输入。不读取或写入生产 Keychain，不自动信任证书。

InputConnectionDriver 使用一条串行队列、20ms 定时器、一个接收请求和一个写入槽；submit 同步进入状态机，没有每个鼠标事件一个待执行闭包。每批最多 256 项，待发队列仍为 64。读取每次最多 4096 字节，帧控制载荷仍限 64。过载、错误、EOF、超时和取消调用真实 transport.cancel；正常释放排空后关闭。结束回调只调用一次，客户端释放也取消；迟到回调不会重启。

连接及应用认证共用初始化起算的 15 秒期限，写入/PONG/排空各 5 秒。contentProcessed 仅说明本地网络栈处理完成，不代表对端已经确认释放；异常时仍依赖服务端断线清理。现有 59 项 Windows 测试包含这类服务端清理证据，但没有运行这个 Swift 客户端。

参考：[Apple NWConnection](https://developer.apple.com/documentation/network/nwconnection)、[contentProcessed](https://developer.apple.com/documentation/network/nwconnection/sendcompletion)。

## Mac 按顺序执行

先在两端保留各自工作区修改并同步全部新文件。尚未提交时不能把下述 pull 当作同步已完成：

~~~bash
git status --short
# 仅在远程已包含本轮代码且可安全快进后：
git pull --ff-only origin main
cd macos/RemoteController
swift test --filter InputSendQueueTests
swift test --filter AuthenticatedInputSenderTests
swift test --filter InputConnectionDriverTests
swift test
swift build -c release
swift run -c release InputPreview
~~~

筛选预期分别 9、13、12 项；全量 96、0 failures，Release Build complete。实际结果必须重新记录，遇到错误先修复再继续。InputConnectionDriverTests 使用合成凭据和传输替身（包括真实 Dispatch 定时器）；没有真正握手到 Windows，不能把它的通过当作跨设备 TLS 完成。

InputPreview 仍是本地同步统计，按 P2_INPUT_HANDOFF.md 的复选项目测焦点/键鼠/失焦释放；不会出现网络或队列状态。

之后的顺序：

1. 修复 Mac 编译/测试/预览发现的问题，记录完整结果。
2. 在受控 mock 环境验证新的 NWConnection TLS：正确指纹/密钥、错误指纹/密钥、慢写入、半帧断线和取消。当前两端模拟 API 都是回环限定，不能直接输入 Windows Tailscale IP；测试宿主或专用转发安排尚未提供，不擅自放宽监听。
3. 按 JPEG_QUALITY_HANDOFF.md 收尾 Windows 实际采集、标准/清晰画质、双端停止重连与跨网 30 分钟。
4. 以上结果明确后才接查看器输入 UI 与 Windows 真实 SendInput，继续保留认证、显式许可、状态提示和立即停止。
5. P2 验收通过后再按开发计划进入重连/发布/独立连接服务，不跳过阶段退出条件。
