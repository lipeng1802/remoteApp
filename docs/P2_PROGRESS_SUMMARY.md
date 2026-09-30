# P2 真实键鼠 MVP 阶段总结

日期：2026-10-01  
状态：**源码开发与双机验收完成；双平台安装包尚未重建。**

## 阶段目标与完成范围

P2 在既有 Tailscale、TLS/HMAC 配对和 JPEG 画面链路上加入 Mac 到 Windows 的真实键鼠控制，同时保持默认只读和明确的双端授权边界。当前已经完成：

- Mac 窗口内鼠标移动、三键、拖动、纵横滚轮、普通键、左右修饰键和常用快捷键捕获与协议映射。
- 画面 aspect-fit 坐标转换和黑边拒绝；拖出画面、失焦、停止、Esc、断开及异常连接的统一释放。
- 有界输入发送队列、移动/滚轮合并、单写入顺序、心跳、连接期限和失败关闭。
- Windows 会话级原生输入边界与 Win32 `SendInput` 接线；只有 TLS/HMAC 认证、Windows 本次许可、Mac 请求控制和 Mac 手动开始全部满足时才创建原生输入 sink。
- Windows 全局 `Ctrl + Alt + Esc` 紧急停止；停止后释放全部输入、结束共享并撤销本次许可。
- Mac 断开只结束当前会话，Windows 保持共享并等待重新连接；同一次共享可在后续连接中选择只读或请求控制。
- Windows 允许控制但 Mac 不请求时自动降级为只读，且不创建原生输入 sink。
- Mac 默认 Windows Tailscale 地址为 `100.73.4.118`，输入框仍可编辑。

## 开发与验证路径

1. 用本地 `InputPreview` 验证捕获、坐标、黑边和释放，不连接网络、不操作 Windows。
2. 完成认证输入队列、发送器和真实 TLS 输入模拟，再通过不同网络的 Tailscale 双机 mock 验证。
3. 建立 Windows 原生输入 API 边界和 fake API 测试，随后完成 JPEG 下行与输入上行的单连接双向会话。
4. 先在产品 GUI 接入内存统计 sink，验证许可、状态和生命周期；通过后再切换到真实 `WindowsInputSink`。
5. 加入 Windows 本机紧急停止、触控板拥塞保护、持续共享和能力降级，完成真实双机第 1–10 项验收。

## 主要问题、根因与处理

| 问题 | 根因 | 处理与结果 |
|---|---|---|
| 左右 Shift、Control、Command 同时按下只显示持有 1 | AppKit 聚合修饰键标志不能区分同类左右物理键 | 改为按 `flagsChanged` 的物理 keyCode 维护左右集合，实测按 `1→2→1→0` 变化。 |
| GUI mock 中 Shift/Control 事件数不稳定，停止后偶尔显示持有 1–2 | 状态报告统一按一秒限流，短促 Up 已执行但末尾状态未刷新 | 持有集合变化立即发布，仅移动、滚轮和重复 Down 限流；复测所有释放路径持有 0。 |
| Mac 触控板纵向滚动稳定触发 `congested` 并断开 | 每个精细滚轮事件产生 `move + wheel` 两帧，超过发送节奏并填满 64 项队列 | 合并连续待发送的触控板移动/滚轮批次，保留顺序屏障并做 Int32 饱和；纵横滚动均不再断开。 |
| Mac 切换应用时对行为预期不清 | “失焦暂停”与“断开会话”措辞混淆 | 明确失焦只释放输入并暂停捕获，画面和认证连接保持；返回后手动再次开始控制。 |
| Mac 主动断开会让 Windows 整体停止共享 | 产品调用单会话 `RunOnceAsync`，DISCONNECT 返回后 WPF 进入共享结束清理 | 新增持续监听 `RunContinuousAsync`；会话结束后释放资源并重新监听，只有 Windows 停止、紧急停止或关闭应用才结束共享。 |
| Windows 已允许控制、Mac 未请求控制时出现 `unexpectedResponse` | Agent 把本机许可误当成 Controller 必须声明 Input，而不是可协商的能力上限 | 允许 `JPEG | Input` Agent 与仅 `JPEG` Controller 降级为只读；只在双方都声明 Input 时创建输入 sink。 |
| 新增两项 Windows 回归报 `Probe payload exceeds 64 bytes` | 测试客户端误用仅适合握手/输入小帧的探针读取器读取 JPEG | 测试改用完整视频帧读取，并继续校验协议最大载荷和连续序号；随后协议 63/63 通过。 |

## 最终验证证据

- Windows RemoteProtocol：**63/63 passed**。
- WindowsInput fake API：**6/6 passed**；不会在自动测试中调用真实 `SendInput`。
- Mac Swift 全量：**111/111 passed**；Release 构建通过。
- 双机位于不同网络，通过 Tailscale 连接。
- [真实输入第 1–10 项](REAL_INPUT_HANDOFF.md) 全部通过：默认只读、双端许可、鼠标与键盘、左右修饰键、快捷键、黑边、纵横滚轮、Esc/停止/失焦/断开释放、Windows 紧急停止、许可重置、持续共享、只读降级和进程重启。
- 所有停止与释放路径最终为“持有 0”；RemoteAgent 重启后未出现紧急停止快捷键注册失败提示。

## 当前安全边界

- 默认只读；Windows 每次新共享都必须重新勾选并确认真实控制，Mac 也必须请求并在认证后手动开始。
- 状态仅记录事件、释放和持有数量，不记录按键内容、扫描码或坐标。
- `SendInput` 遵守 Windows UIPI，不绕过 UAC 安全桌面、锁屏或更高完整性窗口。
- 输入法、Caps Lock、Fn、媒体键、额外键盘布局、多显示器和不同 DPI 组合不属于本轮通过范围。

## 下一阶段

源码验收不代表安装包验收。下一阶段应分别重建 Windows x64 安装包与 macOS 安装包，加入明确版本号，并在干净环境完成安装、首次权限、升级、卸载、启动入口、防火墙/签名提示和双机回归。之后再评估多显示器、DPI、IME/UIPI 边界与长时间稳定性。
