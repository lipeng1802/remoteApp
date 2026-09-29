# Personal Remote Desktop MVP

个人使用的最小远程桌面项目：在 macOS 控制端通过外网控制一台 Windows 电脑。

## MVP 边界

- macOS 只作为控制端。
- Windows 只作为被控端。
- 使用 Tailscale 提供外网寻址、NAT 穿透和加密网络。
- 应用负责屏幕传输、认证、鼠标键盘控制和断线恢复。
- 第一条可运行链路先使用 JPEG；链路稳定后替换为 H.264。

暂不开发自建信令/中继、Windows 控制 Mac、文件传输、剪贴板、多显示器、声音和企业功能。

## 当前进度

P0“环境与协议基线”已经完成：

- `protocol/PROTOCOL.md` 定义 v1 二进制帧和认证状态机。
- `protocol/testdata/v1.json` 保存 Swift/C# 共用的 golden vectors。
- `macos/RemoteController` 是 SwiftPM 管理的 SwiftUI/协议骨架。
- `windows/RemoteAgent` 是 .NET 8 WPF/协议骨架。

P0 已完成，TLS 与应用认证的三轮双机验收通过。P1 JPEG 只读切片已实现：Windows 认证后采集主屏并缩放编码，Mac 显示最新图像，提供开始/停止与连接/断开界面。Windows Release 和 30 项测试通过，真实主屏内存采集检查通过；用户已确认 Mac 本轮 37 项测试通过，Release 查看器已显示真实 Windows 画面并实时更新，地址键入问题及 FPS/30 分钟验收尚待完成，不能视为 P1 整阶段完成。

配对步骤见 [TLS 应用认证验收](docs/TLS_AUTH_VALIDATION.md)，构建、启动和剩余验收见 [JPEG 只读画面验收](docs/JPEG_VALIDATION.md)。尚未实现输入控制或 H.264。
网络监听、屏幕采集和输入注入在 P0 中均未启用。

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
