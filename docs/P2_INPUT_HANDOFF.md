# P2 输入接收与 Mac 验证清单

## 2026-09-30 产品 GUI 控制 mock

产品 RemoteAgent/RemoteController 已接入显式许可、认证后手动开始、失焦/停止释放及同连接 JPEG + 输入；Windows 本轮仍只使用不记录内容的会话级内存统计 sink，不调用 `SendInput`。Mac 全量 109/109、Release 构建通过；用户确认 Windows 自动检查和 [GUI_CONTROL_MOCK_HANDOFF.md](GUI_CONTROL_MOCK_HANDOFF.md) 双机操作全部通过，所有释放路径最终持有 0，默认只读正常且 Windows 桌面没有真实动作。下一阶段进入真实原生 sink 的独立人工验收切片。

## 2026-09-30 Mac 单连接双向调度器

Windows 上一切片 Release、协议 61/61、输入边界 6/6 已由用户确认通过。Mac 认证输入发送器和连接调度器现可同时协商/接收 JPEG，保持独立双向序号、有界队列、一个写入中和统一失败关闭；真实 Network.framework TLS 测试验证同一连接收视频并发送按下/释放/DISCONNECT。新增 5 项后本机全量 109/109、Release 构建成功。产品 GUI 尚未接线，详见 [MAC_DUPLEX_HANDOFF.md](MAC_DUPLEX_HANDOFF.md)。

## 2026-09-30 JPEG + 输入单连接服务端

用户确认上一切片 Windows Release、`WindowsInput.Tests` 6/6 和协议 60/60 全部通过。本轮把 JPEG 下行与输入上行合入同一认证会话，服务端仅一个写入者负责 JPEG/SCREEN_INFO/PONG 并维护连续序号，输入读取错误、断线或取消统一释放。新增真实 loopback TLS/HMAC 回归后协议预期 61/61，待 Windows 按 [DUPLEX_SESSION_HANDOFF.md](DUPLEX_SESSION_HANDOFF.md) 验证。GUI 仍只读，测试仍使用内存 sink。

## 2026-09-30 Windows 原生 sink 边界

独立 `WindowsInput` 模块已实现 `SendInput` 映射和自身持有状态/尽力释放，新增 6 项 fake API 与 win-x64 ABI 测试。生产 GUI/TLS 尚未接线，RemoteAgent 仍是只读；本机无 .NET SDK，待 Windows 按 [WINDOWS_NATIVE_INPUT_HANDOFF.md](WINDOWS_NATIVE_INPUT_HANDOFF.md) 完成 6/6、原协议 60/60 和 Release 构建。该结果不能记为真实桌面输入通过。

## 2026-09-30 双机 Tailscale mock 验收通过

用户确认 Windows 和 Mac 端测试均全部通过：跨网 TLS/HMAC 建连、12 个共享合成事件顺序、显式释放和单次会话关闭检查点完成。Windows 仍为内存 sink，下一阶段才能进入产品 GUI 授权/停止状态与原生 sink 设计；尚不能称为真实 Windows 键鼠控制通过。

## 2026-09-30 双机 Tailscale mock 已实现

Windows 一次性内存 sink 服务、双层显式许可脚本、Tailscale 单地址/单对端限制，以及 Mac 合成客户端已加入。Mac 全量 104/104 和 Release 通过；Windows 预期 60 项，待 Windows 实机构建、测试及双机验收。准确命令与 PASS 标准见 [TAILSCALE_INPUT_MOCK.md](TAILSCALE_INPUT_MOCK.md)。本切片没有 `SendInput`、产品 GUI 接线或真实用户输入。

## 2026-09-30 真实 Mac TLS mock 结果

Network.framework 回环集成测试 5/5 通过：正确指纹/密钥认证及输入排空、错误指纹、错误密钥、半帧断线、取消。慢写入由 `InputConnectionDriverTests.testSlowWriteKeepsOneFlightAndCoalescesIngress` 的可控写完成继续覆盖。全量 103/103、Release 构建成功。测试 TLS 身份仅编译到 test target，监听强制 `127.0.0.1`。

这些结果完成了本机真实 TLS 检查点，但不是双机验收。下一切片仅允许建立一次性、显式授权、Tailscale 单地址/单对端限定的双机 mock；Windows 继续使用内存 sink，禁止 `SendInput`。

## 2026-09-30 Mac 自动检查通过

用户已实际完成三组筛选测试：输入队列 9/9、认证发送器 13/13、连接调度器 12/12；全量 Swift 96/96 通过，Release 构建 `Build complete! (18.76s)`。当前进入下方第 2 项 InputPreview 人工验收。尚未完成的项目包括 AppKit 键鼠/焦点/释放实测、真实 NWConnection TLS mock、P1 画质与稳定性，以及生产 GUI/SendInput；不得因自动测试通过而将这些项目标记完成。

## 2026-09-30 InputPreview 部分结果

用户确认普通键鼠、Esc 和“停止并释放”通过；截图最终状态为“已停止 · 事件 3347 · 释放 20 · 持有 0”，可见左右黑边。左右修饰键、黑边、拖出边界、滚轮、失焦尚未按计数完成验证，保持待办。复测标准：有效操作应使事件/释放计数按预期变化，所有键和按钮释放、失焦、Esc 或停止后必须回到“持有 0”；黑边输入不得建立持有状态。

## 2026-09-30 InputPreview 修饰键修复

人工结果更新：黑边、拖出边界、滚轮、失焦均通过；同类左右修饰键被聚合为一个的问题已修复。输入预览现在以 `flagsChanged` 的物理 keyCode 维护左右状态，不再用聚合查询推断。新增 2 项回归后全量 98/98、Release 构建通过。待实机只复测 Shift、Control、Command 等可用左右键的“持有”1→2→1→0；硬件缺少右侧键的类型记录为不适用。

用户已完成修复版复测，同类左右修饰键 `1→2→1→0` 通过。InputPreview 本地人工验收现全部完成：键鼠、左右修饰键、黑边、拖出边界、滚轮、失焦、Esc/停止和最终释放均通过。后续进入真实 NWConnection TLS mock 检查，不将本地预览结果扩大为跨设备输入或 SendInput 通过。

最后更新：2026-09-29。以下为当前状态，HANDOFF 下方的旧测试数量仅为历史。

## 当前实现与证据

最新增量：有界输入队列和认证发送状态机已加入，新增 22 项 Swift 测试（队列 9、发送器 13），未在 Mac 执行。Windows 新增合并后共享序列 TLS 测试已通过。本轮已编写仅回环的 TLS 适配与调度器，新增 12 项，尚未在 Mac 编译或完成真实握手。按 [流程检查点](WORKFLOW_REVIEW.md) 优先验证，不继续堆积新功能。

- Windows Release：0 警告、0 错误；59/59 自动测试实际通过。
- Mac 新增 ControllerInputCapture 状态机、独立 AppKit InputPreview 工具及 10 项测试，累计预期 96 项。当前 Windows 无 Swift/macOS SDK，Mac 不可连接，未编译或运行这些新增代码。
- 新共享 controller-input-v1.json 描述合成的双侧 Ctrl、A 长按、鼠标按下、滚轮、失焦释放。Windows 用真实回环 TLS 回放并验证完整顺序、重复 Down、在断线前已无持有输入，且不依赖服务端 Dispose 补救。这不是 Swift 已执行或双机互操作通过。
- 产品 RemoteController/RemoteAgent 仍为只读。InputPreview 仅本地内存统计，不调用 TLS、Keychain、屏幕采集或 SendInput。没有全局键盘监听。
- 0.2.4 Setup 仍是既有只读画质版；此次未重新打包，所有累积改动仍未提交/推送。

## Mac 恢复后的顺序

### 1. 同步与构建

先确认 Windows 已提交/推送或通过约定方式同步完整代码；当前直接 pull 不会取得这些未提交文件。Mac 仓库有本地改动时先保留，不 reset --hard。

在 Mac 仓库根目录执行：

~~~bash
git status --short
# 仅在远程已包含本轮代码、工作区可安全快进后：
git pull --ff-only origin main
cd macos/RemoteController
swift test
swift build -c release
~~~

预期：96 tests、0 failures，Release Build complete。必须记录实际数量和日志结论；旧 40 项成功不能代替本轮结果。优先修复 Carbon/AppKit 类型检查、目标 SDK 差异和共享向量问题。

### 2. 单机输入预览（不需要 Windows 在线）

~~~bash
swift run -c release InputPreview
~~~

预期打开“输入预览 · 仅本地模拟”，初始“已停止”、持有 0。不会连接 Windows，也不显示远程画面。若系统报告输入权限限制，记录具体结果，不自行安装软件或放宽隐私设置。

按顺序手动验证：

- [ ] 点击“开始本地模拟”，键盘焦点进入画布。普通字母按下使持有增加，松开回到 0；长按时事件增加，持有仍为 1，不记录字符。
- [ ] 分别测试左右 Shift / Control / Option / Command；同时按左右同类键，松开其中一侧，持有应从 2 到 1 再到 0。重复快照不能增加持有。
- [ ] 验证 Control-C 与 Command-C 都能产生事件；物理 Command 对应 Windows 键，不自动变成 Ctrl。这里的统计不能证明 Windows 快捷键实际效果。
- [ ] 改变窗口宽高制造横/竖黑边；黑边按下不增加持有，图像内点击会增加；拖到黑边或窗口外再松开，持有回到 0。双击、右键、中键分别检查。
- [ ] 触控板细微滚动和鼠标滚轮可累计产生事件；黑边滚动不产生事件。实际远程水平/垂直方向、速度和自然滚动体验待双机验证，本地计数不代替它。
- [ ] 按住普通键/修饰键/鼠标时切到另一窗口：立刻“已停止”、持有 0，释放计数增加。返回后必须再次点击开始，不能自动恢复。
- [ ] 按 Esc 或点击“停止并释放”同样清零；连续停止不重复释放。最小化、窗口关闭、退出应用也要验证正常结束。
- [ ] 停止后在其他应用输入，不改变预览计数。重新开启后，未在本轮按下的普通键重复事件不建立新的持有状态。

AppKit 实际事件、Command 组合键分派、快速左右修饰键变化、窗口外松开和停止按钮焦点行为都未在 Windows 验证；若任一项失败，应先在 Mac 修复，再进行网络输入联调。

### 3. 画面前置验收

按 [JPEG_QUALITY_HANDOFF.md](JPEG_QUALITY_HANDOFF.md) 完成 Windows GDI 采集复验、标准/清晰画质、双端停止重连和跨网 30 分钟稳定性。此前 StretchBlt 失败原因仍待确认；约 9.4 FPS 不记作原至少 10 FPS 目标通过。

### 4. 下一检查点

队列、认证发送状态及 TLS mock 适配器源码均已加入，全部待 Mac 验证。下一步先运行全量 96 项、Release 和 InputPreview，再验证真实 TLS mock；当前不继续扩展新的 P2 功能。先模拟端到端，再做真实双机 mock；现有 Windows 模拟入口固定 loopback，不能直接输入 Tailscale 地址使用，不在此清单中临时开放端口。

画面可用后，再接生产 GUI 的明确授权/停止状态及真实 SendInput；保留默认拒绝、断开释放、可见状态，不在没有画面时启用真实控制。布局/IME、Caps Lock、Fn、媒体键、E1/Pause 等仍待后续。

## 输入策略与限制

ControllerInputCapture 只生成未分配序号的消息类型和载荷，不负责认证或赋予控制权限。串行调用；停用默认丢弃输入；重复普通 Down 仅在 isRepeat 且本轮已按下时通过；按钮重复 Down/未匹配 Up 丢弃。点击/滚轮先发坐标；黑边禁止 Down，已按下按钮在边界外仍允许 Up；停止先释放普通键再修饰键和按钮，并丢弃滚轮小数余量。

InputPreview 通过 NSView responder 接收自身窗口事件，以左上为原点映射 1280×720 模拟画布；仅活跃时查询 8 个左右修饰键状态。Esc 保留为本地停止，不向模拟 sink 传递。支持的 Command 组合在画布中处理；系统保留快捷键仍可能被系统接管，需要实机验收。

滚轮采用 NSEvent 已按用户自然滚动设置处理后的值，不二次反转；水平转为协议正向右。当前精细设备 1 点对应 1 wheel unit，普通设备 delta×120，每次每轴限制 1200，小数累积。这是待实机校准的初始策略，不承诺与真实 Windows 滚动速度完全一致。预览同步消费，不保留事件历史或网络队列。

MacKeyboardMapper 使用物理 ANSI 键位及 SDK Carbon 常量。Control→Ctrl、Option→Alt、Command→Win；不翻译字符、布局或 IME。失焦必须停止并释放，恢复需显式操作。

依据：[Apple NSEvent](https://developer.apple.com/documentation/appkit/nsevent)、[Apple CGEventSource](https://developer.apple.com/documentation/coregraphics/cgeventsource)、[Apple 自然滚动](https://developer.apple.com/documentation/appkit/nsevent/isdirectioninvertedfromdevice)、[Microsoft 鼠标单位](https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-mouseinput)。

## Windows 模拟 TLS 复验

~~~powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
~~~

预期 59/59，当前已实际通过。RunInputSimulationOnceAsync 固定绑定 127.0.0.1，复用 TLS/HMAC，要求双方 Input 能力和显式 LocalControlAllowed。默认限速 240 事件/秒、突发 120，每次读/回复默认 15 秒；序号、64 字节控制上限、认证前输入、错误密钥、空闲、取消、半帧、畸形/超限和断线释放均有测试。生产监听仍保持原只读接口。
