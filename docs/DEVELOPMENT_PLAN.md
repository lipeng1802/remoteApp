# 开发计划

## 1. 项目目标

开发一个仅供个人使用的远程桌面 MVP：

- 当前这台 Intel Mac（macOS 13.7.6）作为控制端。
- 一台 Windows 10/11 x64 电脑作为被控端。
- 两台设备不在同一局域网时仍可连接。
- 外网连接由 Tailscale 提供；本项目不在 MVP 阶段实现 NAT 穿透和中继。
- 用户能在 Mac 上查看 Windows 主显示器，并操作鼠标和键盘。

## 2. 成功标准

MVP 完成时必须满足：

1. Mac 和 Windows 均登录同一个 Tailscale 网络后，可通过设备名或 Tailscale IP 建立连接。
2. Windows Agent 未认证时不发送屏幕，也不接受输入。
3. Mac 能持续显示 Windows 主显示器，最低达到 720p、15 FPS。
4. 支持鼠标移动、左右键、双击、滚轮及常用键盘输入。
5. 网络短暂中断后可自动重连，或给出明确的可重试状态。
6. Windows 本机始终显示正在被远程控制，并可立即断开。
7. 连续运行 4 小时不崩溃、无持续性内存增长。

## 3. 明确不做

- Windows 控制 Mac 或 Mac 被控。
- 自建账户、设备 ID、信令、STUN/TURN 或中继服务。
- 绕过 Tailscale 直接暴露公网端口。
- 多显示器、文件传输、剪贴板、声音、远程打印。
- Windows UAC 安全桌面、登录界面控制。
- 移动端、浏览器端和企业管理后台。

## 4. 技术方案

### 4.1 macOS 控制端

- Swift 5.8+。
- SwiftUI 管理连接界面和远程画布。
- Network.framework 或 URLSession WebSocket 建立长连接。
- 第一阶段用 ImageIO/CoreGraphics 解码 JPEG。
- 第二阶段用 VideoToolbox 解码 H.264。
- 目标系统为 macOS 13，先支持当前 Intel Mac；稳定后再构建 Universal Binary。

### 4.2 Windows 被控端

- C#、.NET 8、WPF。
- 屏幕采集使用 DXGI Desktop Duplication；首个垂直切片可临时使用较简单的截图方式验证协议。
- 第一阶段输出 JPEG 帧。
- 第二阶段使用 Media Foundation H.264 硬件编码，并保留软件回退策略。
- 输入注入使用 Win32 `SendInput`。
- 正式监听时仅绑定 Tailscale 地址，避免暴露在普通局域网和公网接口上。

### 4.3 连接与安全

- Tailscale 负责设备身份、寻址和 WireGuard 网络加密。
- 应用仍实现自己的会话认证，不把“能访问端口”视为已授权。
- Windows 首次启动生成至少 32 字节的随机密钥，以安全的可复制形式展示。
- Mac 将设备地址、认证信息和设备公钥/证书指纹存入 Keychain。
- 使用 TLS 长连接；MVP 可使用首次信任后固定证书指纹的方式防止错误连接。
- 连续认证失败需要延迟或临时锁定。
- 不记录密码、原始键盘内容或屏幕帧。

### 4.4 应用协议

协议采用固定长度头部加二进制载荷，避免把视频帧编码为 Base64。所有整数明确字节序，协议必须包含版本号和最大消息长度。

首版消息：

| 消息 | 方向 | 用途 |
|---|---|---|
| `HELLO` | 双向 | 协议版本和能力协商 |
| `AUTH_CHALLENGE` | Agent → Controller | 认证随机数 |
| `AUTH_RESPONSE` | Controller → Agent | 认证响应 |
| `AUTH_RESULT` | Agent → Controller | 认证结果 |
| `SCREEN_INFO` | Agent → Controller | 尺寸、DPI、像素格式 |
| `VIDEO_FRAME` | Agent → Controller | JPEG 或 H.264 帧 |
| `MOUSE_MOVE` | Controller → Agent | 归一化绝对坐标 |
| `MOUSE_BUTTON` | Controller → Agent | 按下和释放 |
| `MOUSE_WHEEL` | Controller → Agent | 滚轮增量 |
| `KEY_EVENT` | Controller → Agent | 扫描码、按下和释放 |
| `PING/PONG` | 双向 | 心跳和延迟检测 |
| `DISCONNECT` | 双向 | 主动结束会话 |
| `ERROR` | 双向 | 可显示错误代码 |

协议细节应在编码前落到 `protocol/PROTOCOL.md`，并生成双方共享的二进制测试向量。

## 5. 分阶段实施

### P0：环境与协议基线（1–2 天）

交付物：

- 确认完整 Xcode、Swift、Windows .NET 8 SDK 和构建工具。
- 两台设备安装并登录同一 Tailscale 网络。
- 验证 Mac 可通过 Tailscale 地址连接 Windows 测试端口。
- 创建 `protocol/PROTOCOL.md` 和 `protocol/testdata/`。
- 创建 Mac、Windows 两端最小工程和统一格式化/测试命令。
- 建立 macOS `.dmg` 与 Windows `Setup.exe` 的开发版打包入口；每个后续阶段都产出可安装测试包。

退出条件：两个工程均能独立构建，协议帧编解码单元测试通过。

### P1：JPEG 只读垂直切片（3–5 天）

交付物：

- Windows 捕获主显示器并压缩为 JPEG。
- Windows 以长度前缀发送帧。
- Mac 接收、校验长度并显示最新帧。
- 接收端采用“只保留最新帧”策略，禁止帧队列无限增长。
- UI 显示连接、认证、播放和断开状态。

退出条件：不同网络下通过 Tailscale 达到 720p、至少 10 FPS，并连续运行 30 分钟。

### P2：鼠标和键盘控制（3–5 天）

交付物：

- 远程画布坐标转换及 letterbox 区域处理。
- 鼠标移动、左右键、双击、滚轮。
- 键盘按下/释放，正确处理修饰键释放。
- Windows 输入注入封装成可替换接口，测试时使用 mock，避免自动测试误操作真实桌面。
- Windows 本地控制状态提示和断开按钮。

退出条件：可从 Mac 完成“打开记事本、输入文字、选择文本、滚动窗口”的验收场景。

### P3：认证、重连与安全收口（3–4 天）

交付物：

- 随机设备密钥、挑战响应认证或等价的成熟方案。
- TLS 与证书指纹固定。
- Keychain/Windows 安全存储。
- 心跳、超时、指数退避重连。
- 认证失败限速、最大消息尺寸、畸形消息处理。
- 日志脱敏。

退出条件：测试计划中的认证、重放、畸形消息和断网恢复用例通过。

### P4：H.264 视频链路（4–7 天）

交付物：

- Windows Media Foundation H.264 编码。
- Mac VideoToolbox 解码。
- 关键帧请求、时间戳和分辨率变化处理。
- 720p/1080p、低/中/高三档质量。
- JPEG 回退开关保留至 H.264 稳定。

退出条件：720p 达到 15–30 FPS；正常家庭宽带下交互延迟达到测试目标，接收端不积压旧帧。

### P5：个人发布版（3–5 天）

交付物：

- Windows 开机启动，可选择最小化到托盘。
- Mac 保存常用 Windows 设备。
- macOS Developer ID 签名并公证的 `.dmg`，以及代码签名的 Windows `Setup.exe`；开发版打包流程已在 P0 建立。
- 版本号、诊断日志导出、清晰错误提示。
- 完成 4 小时耐久测试和真实外网测试。

退出条件：全部 MVP 验收用例通过，无 P0/P1 级缺陷。

## 6. Codex vibe coding 工作方式

后续 Codex 会话遵循以下节奏：

1. 每个会话只选择一个可在本次验证的里程碑或子任务。
2. 编码前阅读本文件、`TEST_PLAN.md`、`HANDOFF.md` 和当前 `git status`。
3. 先写或更新可执行测试，再完成最小实现。
4. 每次只改一个层次，例如协议、采集、解码或 UI，不同时重写多个模块。
5. 使用真实命令验证，不用“理论上可编译”作为完成标准。
6. 无法在当前 Mac 验证的 Windows 代码，必须明确列出 Windows 上要运行的命令和预期结果。
7. 不擅自安装系统软件、不提交密钥、不开放公网端口。
8. 保留现有用户修改；提交前查看 diff，不使用破坏性 Git 命令。
9. 每轮结束更新 `docs/HANDOFF.md` 的状态、测试结果和下一步。

## 7. 完成定义

一个任务只有同时满足以下条件才算完成：

- 功能有明确验收场景。
- 新逻辑有单元测试或说明为何必须手工验证。
- 当前平台能够运行的构建、静态检查和测试全部通过。
- 协议变更同步更新规范和共享测试向量。
- 未引入明文密钥、凭据或个人设备信息。
- 文档记录已知限制和下一步。

## 8. 风险控制

- **跨平台无法一次验证**：协议测试向量必须双方共享；Windows 构建结果不能由 Mac 端猜测。
- **TCP 视频积压**：发送端限速，接收端只展示最新完整帧；H.264 阶段再评估 UDP/QUIC。
- **键盘卡键**：断开时主动释放所有已按下修饰键。
- **DPI/坐标错误**：协议使用归一化坐标，Windows 端最终映射到物理像素。
- **安全范围扩大**：Agent 仅绑定 Tailscale 接口，并保持 Windows 防火墙默认拒绝公网访问。
- **H.264 阶段卡住**：JPEG 垂直切片保持可运行，禁止因编码优化阻塞基本控制功能。
