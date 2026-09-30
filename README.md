# Personal Remote Desktop MVP

从个人使用 MVP 起步：在 macOS 控制端通过外网控制一台 Windows 电脑。最终面向其他用户提供接近 AnyDesk 的独立安装/连接体验，见 [产品路线](docs/PRODUCT_ROADMAP.md)。当前开发仍使用 Tailscale。

## MVP 边界

- macOS 只作为控制端。
- Windows 只作为被控端。
- 使用 Tailscale 提供外网寻址、NAT 穿透和加密网络。
- 应用负责屏幕传输、认证、鼠标键盘控制和断线恢复。
- 第一条可运行链路先使用 JPEG；链路稳定后替换为 H.264。

暂不开发自建信令/中继、Windows 控制 Mac、文件传输、剪贴板、多显示器、声音和企业功能。

## 当前进度

P0、TLS 和应用认证双机验收已通过；P1 JPEG 能显示 Windows 画面，画质/真实采集复验和跨网 30 分钟稳定性仍待完成。P2 已把键鼠捕获和同连接 JPEG + 输入接入产品 GUI，目前 Windows 只统计输入、不执行原生注入，待双机 GUI 验收。

上一切片 Windows Release、协议 61/61 和输入边界 6/6 已通过；本切片 Mac 109/109 和 Release 已通过，Windows 构建与双机产品 GUI 操作待验证。按 [GUI 控制 mock 交接](docs/GUI_CONTROL_MOCK_HANDOFF.md) 执行，最新状态以 [HANDOFF](docs/HANDOFF.md) 顶部为准。

最终交付为两个平台各自的安装包：macOS 控制端 `.dmg` 和 Windows 被控端 `Setup.exe`。开发阶段从 P0 起持续验证打包，不等到功能全部完成后再处理安装问题。

## 构建与测试

macOS（需要完整 Xcode）：

```bash
cd macos/RemoteController
swift test
swift build
```

Windows（需要 .NET 8 SDK）：

```powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
```

生成自带 .NET 运行时的 Windows x64 发布目录：

```powershell
dotnet publish .\windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj -c Release -r win-x64 --self-contained true
```

## 开发版安装包

生成 macOS 开发版 `.dmg`：

```bash
./packaging/macos/build-dmg.zsh 0.2.0
```

默认使用 ad-hoc 签名，仅供本机和开发测试。正式对外分发需要 Developer ID 签名和 Apple 公证。

在仓库根目录执行以下命令，使用已安装的 .NET 8 SDK 与 Inno Setup 6 生成自带运行时的 `Setup.exe`：

```powershell
.\packaging\windows\build-installer.ps1 -Version 0.2.0
```

产物统一写入被 Git 忽略的 `artifacts/` 目录。Tailscale 不捆绑进安装包，应用只负责检测并引导安装。

## 文档

- [开发计划](docs/DEVELOPMENT_PLAN.md)
- [测试计划](docs/TEST_PLAN.md)
- [下一会话交接](docs/HANDOFF.md)

## 计划中的目录结构

```text
macos/RemoteController/     Swift/SwiftUI 控制端
windows/RemoteAgent/        C#/.NET 8/WPF 被控端
protocol/                   跨平台协议和共享测试向量
docs/                       规划、测试与交接文档
```

继续开发前先阅读 `docs/HANDOFF.md`，其中记录了实际验证结果和下一步。

### 720p画质对照（0.2.4）

Windows现提供低带宽、标准（默认）、清晰三档，仍最高1280×720/10FPS。自包含目录和Setup已生成；32项协议测试与合成画质对比通过，真实采集待复验、Mac当前离线。按 [画质交接清单](docs/JPEG_QUALITY_HANDOFF.md) 顺序验证，清晰档不保证跨网不卡顿。

### Mac 本地输入预览（开发工具）

同步本轮代码后，在 macos/RemoteController 运行：

~~~bash
swift run -c release InputPreview
~~~

点击开始后测试窗口内键鼠、黑边与失焦释放；只显示本地事件计数，不连接 Windows，不记录按键内容。该工具的人工清单已通过；产品 GUI 的跨机 mock 仍按 [GUI 控制 mock 交接](docs/GUI_CONTROL_MOCK_HANDOFF.md) 单独验收。

### 输入发送状态机

有界队列、相邻移动合并、单写入序号、认证门禁、心跳与结束期限已加入。固定回环的 TLS 网络适配器已编写，Mac 新增代码仍待编译和实测；当前优先完成 [流程检查点](docs/WORKFLOW_REVIEW.md)，GUI 仍只读。
