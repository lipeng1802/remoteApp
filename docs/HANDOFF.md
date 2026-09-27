# 下一次 Codex 会话交接

## 当前状态

- P0“环境与协议基线”已经开始，协议规范、共享测试向量和两端最小工程骨架已创建。
- 产品范围已经锁定：macOS 控制端通过 Tailscale 外网控制 Windows 被控端。
- 默认方向是单向控制，不开发 Windows 控制 Mac。
- 第一条垂直链路使用 JPEG，完成控制和稳定性后再升级 H.264。

当前 Mac 环境检查结果：

- 架构：`x86_64`
- macOS：`13.7.6`（Build `22H625`）
- 当前 Xcode Swift：`5.9.2`
- 开发目录：`/Users/lipeng/Documents/ChatGPT/远程软件开发`
- Git 分支：`main`
- 已安装并选中 Xcode 15.2（Build 15C500b），macOS SDK 14.2、Swift 5.9.2；Swift Debug 测试与 Release 构建均已通过。
- 当前 Mac 未检测到 `dotnet`；Windows 工程应在 Windows 设备安装 .NET 8 SDK 后验证。
- 当前 Mac 未检测到 Tailscale，Windows 也尚未安装 Tailscale。

已知 Windows 目标环境：

- Windows 11 25H2，x64。
- 主显示器为 4K；具体 DPI 缩放由 Agent 运行时检测，不写死。
- .NET 环境未知；开发验证需要 .NET 8 SDK，最终发布采用 self-contained x64，不要求用户预装运行时。
- 两台设备可以处于不同物理网络，但跨设备联调前必须授权加入同一个 Tailscale tailnet。

已创建：

- `protocol/PROTOCOL.md`：28 字节大端序帧头、消息类型、认证状态机、大小限制及错误码。
- `protocol/testdata/v1.json`：三组跨语言帧 golden vectors，以十六进制文本保存准确线缆字节。
- `macos/RemoteController`：SwiftPM、SwiftUI 占位应用、Swift 协议编解码器及 XCTest。
- `windows/RemoteAgent`：.NET 8 WPF 占位应用、C# 协议编解码器及无外部测试包的控制台测试运行器。
- `packaging/macos`：从 Swift Release 构建组装 `.app`、签名并生成 `.dmg` 的脚本。
- `packaging/windows`：发布 self-contained win-x64 Agent 并使用 Inno Setup 生成 `Setup.exe` 的脚本。

## 下一会话唯一目标

完成 P0 的 Windows 构建验证：在 Windows 安装 .NET 8 SDK 后运行 C# 构建与协议测试，并修正发现的跨语言差异。之后安装 Tailscale，在不同物理网络的两台设备上加入同一个 tailnet 并验证基础可达性。不要开始屏幕采集、输入控制或 H.264。

Mac 验证命令：

```bash
xcodebuild -version
cd macos/RemoteController
swift test
swift build
```

Windows 验证命令（PowerShell）：

```powershell
dotnet --info
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
.\packaging\windows\build-installer.ps1 -Version 0.1.0
```

预期：solution 无警告/错误完成构建，测试运行器输出 `6/6 tests passed`，`artifacts\windows` 生成可在未预装 .NET Runtime 的 Windows 11 x64 上安装的 `PersonalRemoteDesktopAgent-0.1.0-win-x64-Setup.exe`。

## 可直接复制到新会话的提示词

```text
请继续开发当前仓库中的 Personal Remote Desktop MVP。

先完整阅读 README.md、docs/DEVELOPMENT_PLAN.md、docs/TEST_PLAN.md 和 docs/HANDOFF.md，并检查 git status。P0 的协议规范、跨语言测试向量、两端工程骨架和打包入口已经创建，Mac 构建、测试和开发版 DMG 已通过；此次只完成 Windows 实际构建、6 项协议测试和 Setup.exe 打包验证，修正发现的编译或跨语言问题。

不要开始 H.264、鼠标键盘控制、自建公网穿透或中继。不要擅自安装大型系统软件或修改防火墙。Windows 代码若无法在当前 Mac 验证，请给出要在 Windows 机器运行的准确命令和预期结果。完成后更新 docs/HANDOFF.md，写明改动、验证结果、阻塞项和下一步。
```

## 每轮结束时更新格式

```text
日期：
完成的里程碑：
主要改动：
Mac 验证命令与结果：
Windows 验证命令与结果：
手工测试结果：
已知问题/阻塞：
下一轮唯一目标：
```

## 本轮记录

日期：2026-09-27

完成的里程碑：P0 协议契约、共享测试向量和双端最小工程骨架；macOS 构建验证已完成，Windows 构建验证尚未完成。

主要改动：新增协议文档、3 组 golden vectors、Swift 协议库与 SwiftUI 占位应用、C# 协议库与 WPF 占位应用，以及双端协议测试。

Mac 验证命令与结果：

- `xcodebuild -version`：通过，Xcode 15.2（Build 15C500b）。
- `xcrun --sdk macosx --show-sdk-version`：通过，SDK 14.2。
- `swift --version`：通过，Apple Swift 5.9.2，x86_64，目标 macOS 13.0。
- `swift test --disable-sandbox --scratch-path /tmp/prd-remotecontroller-build`：通过；构建 Swift 协议库、SwiftUI 可执行目标和测试目标，执行 6 项测试，0 失败。
- `swift build -c release --disable-sandbox --scratch-path /tmp/prd-remotecontroller-release`：通过；Release 版本完成链接。
- `./packaging/macos/build-dmg.zsh 0.1.0`：Release 编译、`.app` 组装和 ad-hoc 签名通过；Codex 沙箱内的 `hdiutil` 需要单独获得磁盘映像权限。
- `hdiutil verify artifacts/macos/PersonalRemoteDesktop-0.1.0-macOS.dmg`：通过，DMG 校验有效，大小约 45 KiB。
- DMG SHA-256：`4abc5a1a46b52b979ebcf52ff0352b73c5d7a39710f898a7de617e4d87e780df`。
- SwiftPM 的用户缓存不可写警告来自 Codex 文件沙箱；通过 `/tmp` 模块缓存完成验证，不是项目代码问题。
- `swiftc -frontend -parse ...`：通过；Swift 源文件和测试文件语法解析成功，但这不能代替类型检查与构建。
- Ruby 校验 `protocol/testdata/v1.json`：通过；全部 frame 长度等于 28 字节头部加声明 payload 长度。
- Ruby 逐字段解码 golden vectors：通过；magic、版本、头长、类型、flags、payload 长度、sequence、timestamp 和 payload 均匹配清单。
- XML/XAML 解析与尾随空白检查：通过。

Windows 验证命令与结果：未运行；当前 Mac 没有 `dotnet`，需要在 Windows 目标机安装 .NET 8 SDK 与 Inno Setup 6 后执行构建、测试和 `packaging\windows\build-installer.ps1`。

手工测试结果：尚未进行跨设备连接测试；两端尚未安装并加入同一 Tailscale tailnet。

已知问题/阻塞：Windows 侧尚无 .NET 8 SDK 和安装包验证结果；本机 Docker daemon 未运行，无法借助现有容器验证 C#；Tailscale 尚未安装。macOS 当前使用 ad-hoc 签名，只适合开发测试；公开分发前需要 Developer ID 签名与 Apple 公证。

下一轮唯一目标：在 Windows 11 x64 目标机完成 C# 实际构建与 6 项协议测试，并修正发现的问题。
