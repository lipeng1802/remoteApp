# 下一次 Codex 会话交接

最后整理：2026-09-28。请以顶部的当前状态、当前唯一目标和文末最新记录为准；中间按时间保留的失败、待确认及下一步描述均为历史记录。

## 当前状态

- 认证收口提交为 `344e0b4`；用户已授权将其与本轮 JPEG 提交一起推送远程。P0 与 TLS 内应用认证三轮双机验收已完成。
- 本轮已实现 P1 JPEG 只读切片：Windows WPF 开始/停止共享，认证后 GDI 主屏采集、最高 1280×720/10 FPS、逐帧 PING/PONG 确认；Mac SwiftUI 查看器、尺寸校验、最新图像缓存及断开清屏。Windows Release 0 警告/错误、30/30 测试通过；真实主屏内存采集 31 次通过，未保存图像。
- Mac 历史 28 项测试和认证探针通过仍有效；本轮新增 9 项，预期 37 项，当前 Windows 无法执行 Mac 构建/测试。真实双机 JPEG 显示与 30 分钟/10 FPS 性能验收待完成，不能宣称 P1 整阶段通过。没有输入控制或 H.264。
- 产品范围已经锁定：macOS 控制端通过 Tailscale 外网控制 Windows 被控端。
- 默认方向是单向控制，不开发 Windows 控制 Mac。
- 第一条垂直链路使用 JPEG，完成控制和稳定性后再升级 H.264。

当前 Mac 环境检查结果：

- 架构：`x86_64`
- macOS：`13.7.6`（Build `22H625`）
- 当前 Xcode Swift：`5.9.2`
- 开发目录：`/Users/lipeng/Documents/ChatGPT/远程软件开发`
- Git 分支：`main`
- 已安装并选中 Xcode 15.2（Build 15C500b），macOS SDK 14.2、Swift 5.9.2；历史版本 Swift Debug 测试与 Release 构建均已通过；本轮 28 项测试已由用户确认通过，swift run -c release 的 Build complete 与实际探针结果也已确认。
- Mac 不承担 Windows 实际构建；Windows 工程已在目标机使用 .NET 8 SDK 完成验证。
- 用户已确认 Mac 和 Windows 均安装 Tailscale、登录同一账号并能看到两台设备；Mac 使用手机热点，与 Windows 不在同一物理网络。Windows 到 Mac 的 Tailscale ping 直连成功（71 ms），Mac 到 Windows 的反向 ping 也直连成功（最近一次约 5 ms），跨外网 Tailscale 层验证通过。

已知 Windows 目标环境：

- Windows 11 25H2，x64。
- 主显示器为 4K；具体 DPI 缩放由 Agent 运行时检测，不写死。
- 用户已安装 .NET SDK 8.0.425 x64 与 Inno Setup 6.7.3；2026-09-27 已实际通过构建、测试和打包。发布配置为 self-contained win-x64，包含 .NET / Windows Desktop 8.0.31；无预装运行时机器上的安装启动尚待验收。
- 两台设备已由用户安装 Tailscale 并加入同一个 tailnet，跨网络 ping 与临时 TCP 请求/响应均已通过。

已创建：

- `protocol/PROTOCOL.md`：28 字节大端序帧头、消息类型、认证状态机、大小限制及错误码。
- `protocol/testdata/v1.json`：三组跨语言帧 golden vectors，以十六进制文本保存准确线缆字节。
- `macos/RemoteController`：SwiftPM、SwiftUI 占位应用、Swift 协议编解码器及 XCTest。
- `windows/RemoteAgent`：.NET 8 WPF 占位应用、C# 协议编解码器及无外部测试包的控制台测试运行器。
- `packaging/macos`：从 Swift Release 构建组装 `.app`、签名并生成 `.dmg` 的脚本。
- `packaging/windows`：发布 self-contained win-x64 Agent 并使用 Inno Setup 生成 `Setup.exe` 的脚本。

## 当前唯一目标

同步本轮 JPEG 代码到 Mac，先运行 37 项测试与 Release 构建，修复真实编译问题；随后按 [JPEG_VALIDATION.md](JPEG_VALIDATION.md) 启动两端图形应用，完成画面、停止/断开与不同网络 30 分钟验收。此目标已由用户授权，不需要再询问是否开始 JPEG。

Windows 代码与采集已在本机验证，Mac 新代码目前仅静态检查。保留现有配对密钥和证书，GUI 复用同一 Tailscale 地址条目。旧 TLS 探针只验认证，不显示画面，不能与 GUI 同时占用 47475。不开发输入、H.264 或自建穿透，不修改防火墙或安装系统软件。
独立保留的安装验收待办：在无预装 .NET 的 Windows 11 x64 环境确认 self-contained 安装、启动与卸载。当前开发机已安装 .NET，因此这项仍未完成，不影响已获得的 P0 网络验证结论。

Mac 验证命令：

```bash
xcodebuild -version
cd macos/RemoteController
swift test
swift build -c release
```

Windows 验证命令（PowerShell）：

```powershell
dotnet --info
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
.\packaging\windows\build-installer.ps1 -Version 0.1.0
```

P0 历史预期为 `6/6 tests passed`。2026-09-28 JPEG 接入后当前 Windows 已输出 `30/30 tests passed`。JPEG 本轮已生成 0.2.0 Windows 开发安装包；旧 0.1.0 包不包含本轮功能。0.2.0 安装/卸载和 Mac DMG 尚待验收。

## 可直接复制到新会话的提示词

```text
请接收 Personal Remote Desktop MVP 的 JPEG 只读切片交接。

先阅读 README.md、docs/DEVELOPMENT_PLAN.md、docs/TEST_PLAN.md、docs/HANDOFF.md、docs/JPEG_VALIDATION.md，并检查 git status。认证收口已本地提交 344e0b4；JPEG 改动纳入本轮提交，用户已授权推送远程；同步时以实际 origin/main 为准。
Windows Release 0 警告/错误、30/30 测试通过；真实 4K/150% DPI 主屏在内存采集 31 次通过，GDI 句柄无增长。Mac GUI 和 JPEG 协议代码已编写，预期37项测试，但本Windows不能编译macOS Frameworks。下一步先在Mac执行swift test、swift build -c release，修复问题，再启动RemoteController与Windows RemoteAgent图形应用验证只读画面。
已有TLS证书和设备密钥不要重置；旧探针不显示画面，不与GUI同时运行。双机画面、断开/停止、真实分辨率变化和30分钟/10FPS验收仍待完成。允许继续JPEG，不开始鼠标键盘、H.264、自建穿透或中继，不修改防火墙。
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

## 历史记录：初始工程与 Mac 验证

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

## Windows 验证尝试与修正（2026-09-27）

完成的里程碑：完成 Windows 环境检查和两处静态修正；Windows 实际构建、6 项协议测试和 Setup.exe 成功打包仍未完成，不能标记 P0 通过。

环境与初始状态：

- 仓库位于工作目录下的 `remoteApp`，分支 `main`，开始时 `git status --short` 无输出。
- 已完整阅读 README、开发计划、测试计划和本交接文档；未找到仓库内 AGENTS.md。
- 当前运行于 Windows x64，DisplayVersion 25H2，Build 26200.9550。
- `dotnet --info`：Host 6.0.36，只有 Microsoft.NETCore.App / Microsoft.WindowsDesktop.App 6.0.36，`No SDKs were found`。
- PATH 与默认 `C:\Program Files (x86)\Inno Setup 6` 未发现 ISCC；不排除其他自定义安装位置。
- 普通执行器及备用读取工具因 `CryptUnprotectData failed: 2148073483` 无法启动；经工具审批的沙箱外命令可用。

主要改动：

- 测试运行器的 `Equal<T>` 改用 `EqualityComparer<T>.Default`，移除枚举无法满足的 `IEquatable<T>` 约束；保留原有 6 项测试。
- `build-installer.ps1` 在 `dotnet publish` 后立即检查退出码，失败就抛出错误，避免继续打包或误报成功。
- 未改协议、共享向量、Swift 或产品功能。共享向量项目路径经实际解析确认原有四层 `..` 正确，未保留路径修改。

Windows 验证命令与结果：

- `dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release`：失败，缺少 SDK，退出码 `-2147450735`；尚未进入 C# 编译。
- `dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release`：同样因缺少 SDK 失败；6 项测试均未运行，不能宣称 `6/6 tests passed`。
- `.\packaging\windows\build-installer.ps1 -Version 0.1.0`：实际调用后，在发布阶段按预期抛出 `dotnet publish failed with exit code -2147450735.`；未调用 ISCC、未生成 Setup.exe。
- PowerShell Parser：打包脚本语法通过。
- 测试 csproj 的共享向量路径解析：通过，确实指向仓库 `protocol/testdata/v1.json`。
- PowerShell 独立读取 3 组 golden vectors：magic、版本、头长、消息类型、flags、payload 长度、sequence、timestamp 和 payload 全部匹配；这不是 C# 协议测试，也不是新的 Swift/C# 互操作验收。
- `git diff --check`：通过。

Mac 验证命令与结果：本轮未运行；沿用上文历史结果，未将其当作 Windows 验证证据。

手工测试结果：未安装或启动 Agent，未执行安装/卸载、无预装运行时机器测试和跨设备连接。未安装系统软件、未修改防火墙。

已知问题/阻塞：缺少 .NET 8 SDK；未定位到 Inno Setup 6。静态修正尚需实际 .NET 构建确认；成功打包和安装验收均待工具就绪。

下一轮唯一目标：补齐或定位工具后，完成以下 Windows 复验及安装冒烟测试，仍不开展后续功能。

### Windows 工具就绪后的准确复验命令

在 PowerShell 中进入本仓库根目录（当前机器如下；其他机器替换为实际克隆路径）。需要 .NET 8 SDK 和 Inno Setup 6 已就绪；不应以 .NET Runtime 代替 SDK。

```powershell
Set-Location -LiteralPath 'H:\chatgpt\远程软件开发\remoteApp'
dotnet --info
dotnet --list-sdks
# 预期包含 8.0.x SDK。若工具安装在自定义位置，先将其目录加入当前会话 PATH。

dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
if ($LASTEXITCODE -ne 0) { throw 'Release build failed' }

dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
if ($LASTEXITCODE -ne 0) { throw 'Protocol tests failed' }

.\packaging\windows\build-installer.ps1 -Version 0.1.0

$installer = '.\artifacts\windows\PersonalRemoteDesktopAgent-0.1.0-win-x64-Setup.exe'
if (!(Test-Path -LiteralPath $installer)) { throw 'Installer not found' }
Get-Item -LiteralPath $installer | Select-Object Name, Length
Get-FileHash -LiteralPath $installer -Algorithm SHA256
```

预期结果：Release 构建 0 警告、0 错误；测试输出 6 条 PASS 和 `6/6 tests passed`；发布与 ISCC 编译成功，生成非空的指定 Setup.exe，并记录实际 SHA-256。安装包存在及哈希不能代替安装验收。

在没有预装 .NET 运行时的 Windows 11 x64 测试机上，手工运行生成的 Setup.exe，完成安装、启动 WPF 占位窗口、关闭及卸载；预期无需另装 .NET、无启动异常、卸载成功。本轮尚未验证这些预期。

## Windows 实际构建与打包完成（2026-09-27，工具安装后续验）

本节取代上文“Windows 验证尝试与修正”中的当前阻塞结论；此前失败记录保留为历史。

完成的里程碑：本次限定目标已完成——Windows Release 实际构建、原有 6 项协议测试及 Setup.exe 打包全部通过。P0 其余跨设备环境验证和安装冒烟不因此自动视为通过。

主要改动：沿用并验证本次已修正的两处代码：测试断言使用 `EqualityComparer<T>.Default` 支持枚举；打包入口在 `dotnet publish` 失败时立即终止。未发现需要修改协议或共享向量的跨语言问题。本次续验仅更新交接结果，未开发后续功能。

工具版本：用户自行安装 .NET SDK 8.0.425 x64、MSBuild 17.11.48 和 Inno Setup 6.7.3（默认安装目录）。

Windows 验证命令与结果：

- `dotnet --info`：确认 SDK 8.0.425、Host 8.0.31、RID win-x64。
- `dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release`：退出码 0，0 警告、0 错误，协议库、WPF 应用、测试运行器全部构建成功。
- `dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release`：退出码 0，`6/6 tests passed`。
  - golden vectors decode and re-encode
  - one-byte stream splits
  - coalesced frames
  - invalid magic
  - oversized payload rejected from header
  - incomplete frame rejected at end
- `.\packaging\windows\build-installer.ps1 -Version 0.1.0`：退出码 0；self-contained win-x64 发布成功；Inno Setup 6.7.3 输出 `Successful compile`。
- 发布运行时配置包含 Microsoft.NETCore.App / Microsoft.WindowsDesktop.App 8.0.31 的 `includedFrameworks`，与自带运行时的配置一致。
- `git diff --check`：通过。

产物：

- 路径：`artifacts/windows/PersonalRemoteDesktopAgent-0.1.0-win-x64-Setup.exe`。
- 版本：0.1.0。
- 大小：49,225,694 字节，约 46.9 MiB。
- SHA-256：`FE002EFDEFC04ABCDD835D6486816A54FE0A0FE5FC319127A13EC5AC151C4DDD`。
- Authenticode：NotSigned，当前为未签名开发版。
- 产物位于 Git 忽略的 artifacts 目录，未加入版本控制。

Mac 验证命令与结果：本次未重复执行，沿用历史 Mac 构建、6 项测试与 DMG 验证记录。

手工测试结果：本次未启动安装程序或 Agent，未进行安装/卸载及跨设备连接测试。当前机器已安装 .NET，不能充当“未预装运行时”的验收环境。

已知问题/阻塞：此前 SDK 和 Inno Setup 缺失已解决；本次构建、协议测试和打包无阻塞。尚待无预装 .NET 的 Windows 11 x64 环境完成安装、启动、关闭、卸载验证；正式签名、Tailscale 联调均不在本轮范围。首次调用 SDK 时，.NET CLI 自动创建了 ASP.NET Core HTTPS 开发证书；未运行 trust 命令，本项目未使用该证书。未修改防火墙或安装其他软件。

下一轮唯一目标：手工验收上述开发版 Setup.exe，预期能安装并显示 P0 占位窗口，无需另装 .NET，关闭及卸载正常。仍不开展 H.264、输入控制、自建公网穿透或中继。

## 用户手工验收反馈（2026-09-27）

- 用户确认：“测试安装和卸载都正常”。记录为当前 Windows 机器上的安装、卸载验收通过；结果来自用户手工测试，非自动化验证。
- 用户未单独确认占位窗口启动、关闭后重新启动的结果，因此不将这些检查标为已通过。
- 当前机器已安装 .NET SDK / Runtime；本次反馈不替代无预装 .NET 环境的 self-contained 验证。
- 本轮仅更新交接文档，未修改代码或重新构建安装包；`git diff --check` 通过。
- 下一步：补充启动/重新启动结果；有条件时在无预装 .NET 的 Windows 11 x64 环境验收。后续开发阶段先验证 Tailscale 基础可达性，再推进 JPEG 只读链路；安装 Tailscale 或改变网络配置前仍需用户授权。本轮未开始后续功能。

## 启动验收通过与网络准备（2026-09-27）

- 用户补充确认程序可以正常启动、重复打开；结合此前反馈，本机安装、启动、重复打开、卸载冒烟通过。
- 用户要求“继续下一步”，当前任务推进至 Tailscale 基础连通验证。
- Windows 只读检查：PATH、默认 Program Files 安装路径和服务列表均未检测到 Tailscale；默认 Downloads 目录也未发现 Tailscale 安装包。未据此断言其他自定义位置不存在。
- 已询问安装方式与 Mac 可操作状态，等待用户答复；未安装软件、修改防火墙或启动监听端口。
- 两端加入同一 tailnet 后先使用 `tailscale ping` 检查对端可达性；这不等同于应用 TCP/TLS 连接、认证或屏幕链路通过。
- 官方安装说明：https://tailscale.com/docs/install/windows 与 https://tailscale.com/docs/install/mac 。

## Tailscale 单向连通验证（2026-09-27）

- 用户确认两端均已安装 Tailscale，登录同一账号，设备列表能看到两台设备。
- Windows CLI 版本 1.102.4；`status --json` 显示 BackendState=Running、本机在线、Health 为空；唯一 macOS 对端在线。
- 执行 `tailscale ping --c 5 --timeout 5s <Mac Tailscale 地址>`：退出码 0，收到直连 pong，71 ms。默认遇到 direct 即停止，所以本次实际只有一条响应，并非 5 次延迟采样。
- 未把真实设备名、IP 或凭据写入仓库；未修改防火墙、未安装软件、未启动测试端口。
- 待用户在 Mac 反向运行 `tailscale ping --c 5 --timeout 5s <Windows Tailscale 地址>` 并反馈结果。需另行确认两台设备是否使用不同物理网络；当前不能将结果标记为外网测试通过。
- `tailscale ping` 检查 Tailscale 层路径，不证明 Windows 应用端口、TLS、认证或屏幕传输可用。
- 本轮只更新文档；`git diff --check` 通过。

## Tailscale 双向连通验证完成（2026-09-27）

- Mac 端已安装 Tailscale、完成登录，CLI 状态为 `Running`。
- Mac 端识别到唯一一台 Windows 对端，状态在线。
- Mac 到 Windows 的 `tailscale ping --c 5 --timeout 5s` 退出码为 0，确认路径为 direct；最近一次响应约 5 ms。Tailscale 在确认 direct 后提前结束，因此实际返回一条响应。
- 结合此前 Windows 到 Mac 的 direct 结果，当前已完成双向 Tailscale 层可达验证。
- 未输出或写入真实设备名、Tailscale 地址或凭据；未修改防火墙、网络配置或应用监听状态。
- 用户确认 Mac 使用手机热点，Windows 位于另一网络；因此本次可标记为“不同物理网络下的 Tailscale 双向直连验收通过”。
- 本次只证明 Tailscale 网络层可达，不证明应用 TCP/TLS、认证、屏幕帧或远程输入链路可用。
- 下一步唯一目标：完成仅绑定 Windows Tailscale 地址的临时 TCP 端口可达性验证；该项通过后结束 P0，进入 JPEG 只读链路。

## Windows 端：临时 TCP 端口验证

在 Windows 仓库根目录执行：

```powershell
.\scripts\p0\Test-TailscaleTcp.ps1 -WaitSeconds 300
```

脚本只绑定 Tailscale 分配给本机的 IPv4 地址、端口 47474，只接受唯一在线 Mac 对端的地址。最长等待 300 秒；请求读取总时限默认 5 秒，最多读取固定 11 字节（`prd-p0-test` 加 LF），精确匹配后返回 `prd-p0-ok` 加 LF。成功、失败或超时都会关闭客户端和监听器，不安装服务、不修改防火墙、不打印地址或请求内容。`-LoopbackTest` 仅供 Windows 本地行为检查，不算跨网络验证。

Mac 保持手机热点，在 Windows 输出 READY 后运行：

```bash
python3 - <<'PY'
import json, socket, subprocess
ts = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"
status = json.loads(subprocess.check_output([ts, "status", "--json"], timeout=10))
peers = [p for p in status.get("Peer", {}).values()
         if p.get("OS") == "windows" and p.get("Online")]
assert len(peers) == 1, "需要恰好一台在线 Windows 设备"
ip = next(a for a in peers[0]["TailscaleIPs"] if "." in a)
with socket.create_connection((ip, 47474), timeout=10) as s:
    s.sendall(b"prd-p0-test\n")
    reply = s.makefile("rb").readline(32)
    assert reply == b"prd-p0-ok\n", "响应不符合预期"
print("PASS: 跨网络 TCP 请求/响应成功")
PY
```

预期 Mac 输出 PASS，Windows 输出 PASS 和 CLOSED，最后检查端口不再监听。只有两端结果及不同物理网络条件均确认后，才记录跨网络 TCP 通过。本测试不验证 TLS、认证或视频传输。

如果 Windows 弹出防火墙提示或 Mac 连接超时，不要全局放行、不要修改公网规则；先记录错误并结束监听，再评估原因。

## Windows P0 剩余项核查（2026-09-27，基于 abdbc17）

- 核查开始时工作区干净，main 与本地 origin/main 跟踪引用一致。Mac 最新提交 abdbc17 仅修改 HANDOFF.md，未修改 Windows、协议或打包代码；因此沿用已通过的 Release 构建、6 项协议测试和打包结果，本次没有无必要地重复构建。
- 最新交接已记录：Mac 使用手机热点、Windows 位于另一网络，双向 Tailscale ping direct 通过。这是网络层结果，不是 TCP/TLS 或应用认证结果。
- 本次 Windows 实查：Tailscale 为 Running、本机与唯一 Mac 对端均在线、Health 为空；47474 无监听。未启动监听或修改防火墙。
- 原 Setup.exe 仍在本机，SHA-256 与已验收产物一致：FE002EFDEFC04ABCDD835D6486816A54FE0A0FE5FC319127A13EC5AC151C4DDD。

剩余验证：

1. P0 开发计划明确要求的 TCP 测试端口：仅绑定 Windows Tailscale 地址，由不同物理网络的 Mac 发出 `prd-p0-test` 并收到 `prd-p0-ok`，最后确认监听关闭。当前没有执行结果。
2. self-contained 安装验收：在未预装 .NET 的 Windows 11 x64 环境完成安装、启动、关闭和卸载；当前开发机已安装 .NET，不能替代此项。

当时的临时 TCP 脚本检查（问题已由当前脚本解决）：旧示例的 AcceptTcpClient 和 ReadLine 没有超时，ReadLine 也没有长度上限，且收到任意文本都会回复成功。实际执行前应补充等待/读取超时、固定消息校验、输入长度限制和 finally 清理；只记录固定状态，不回显任意输入。该测试不涉及屏幕、输入或应用凭据；即便通过，也不能宣称 TLS 或认证已经通过。

范围说明：现有 6 项测试是 P0 基线，并未覆盖 TEST_PLAN.md 中所有未来测试。未支持版本、未知类型、非零 flags、最大合法载荷等协议边界仍可在后续补测；认证、JPEG、输入、重连、H.264 及正式签名属于后续阶段，不应作为本次已有 P0 测试的通过项。

本轮改动仅为交接核查记录；git diff --check 通过。下一步唯一目标仍是临时 TCP 跨网络请求/响应验证，需 Mac 端配合，禁止擅自修改防火墙。

## 临时 TCP 首次实测（2026-09-27，用户后续确认实际为局域网）

- 准备阶段用户表示 Mac 已连接热点；后续明确反馈首次成功实际为局域网测试，因此本节只记录局域网通过，不作为跨网络 TCP 证据。
- 新增 `scripts/p0/Test-TailscaleTcp.ps1`：仅绑定本机 Tailscale IPv4，限定唯一在线 Mac 的源地址，固定 11 字节请求校验，300 秒连接等待上限、5 秒请求读取总时限；任何结束路径均释放监听。只传输测试常量，不涉及屏幕或凭据，不打印实际地址。
- Windows 回环行为验证 4/4 通过：正确请求收到 `prd-p0-ok`；错误请求退出码 1；读取超时退出码 1；等待连接超时退出码 1。每例均确认 finally 关闭并可重新绑定测试端口。
- 实际运行 `.\scripts\p0\Test-TailscaleTcp.ps1 -WaitSeconds 300`，输出 READY，系统只读检查确认 47474 仅绑定本机 Tailscale 地址，Mac 在线。
- 随后收到匹配 Mac 地址的连接和精确测试请求；Windows 输出 `PASS expected request received; response sent`、`CLOSED temporary listener`，退出码 0。
- 结束后通过 Get-NetTCPConnection 确认 47474 已无监听。
- 用户确认局域网测试成功，结合 Windows 请求/响应输出，局域网 TCP 双端验收通过。随后切换手机热点重试发生 ConnectionRefusedError（Errno 61）；首次成功后监听已自动结束，重试时只读确认 47474 无监听。优先重启监听后复测，不能据此判断跨网络不可达。
- 未修改防火墙、安装系统软件或启动常驻服务。回环与网络测试均未涉及 TLS、应用认证、视频或输入，不能替代这些后续验收。
- 本轮未重建 Agent 或 Setup.exe；应用代码未变。PowerShell 语法检查、git diff --check 通过。脚本和交接改动尚未提交。
- 剩余：保持 Mac 手机热点，重新启动一次性监听并重测跨网络 TCP；无预装 .NET 的 Windows 环境安装验收仍待完成。

## 跨网络 TCP 复测通过（2026-09-27）

- 首次局域网成功后监听按设计自动关闭；用户切换手机热点直接重试时收到 ConnectionRefusedError。Windows 随后确认端口无监听，未据此修改防火墙。
- 重新运行 `.\scripts\p0\Test-TailscaleTcp.ps1 -WaitSeconds 300`，确认 READY 后请用户保持 Mac 手机热点，重新运行相同 Python 请求命令。
- 用户反馈“已执行，显示成功”；Windows 同时输出 `PASS expected request received; response sent` 与 `CLOSED temporary listener`，退出码 0。
- Get-NetTCPConnection 再次确认 47474 无监听，临时测试已清理完成。
- 结论：Mac 手机热点到 Windows 原网络的 TCP 请求/响应验证通过。先前拒绝连接现象在重启一次性监听后消失，本轮无需防火墙或网络配置改动。
- P0 开发计划要求的测试端口可达性已验证；不将结果扩展为 TLS、认证或 JPEG 链路通过。无预装 .NET 安装验收仍待完成。
- 新增有界 TCP 验证脚本，修订交接示例和状态记录；本轮没有修改应用代码、重建安装包或启动后续功能。git diff --check 通过，脚本与最终交接记录纳入本次 Git 提交。

## P1 安全会话门禁切片（2026-09-27，Mac 实现与验证）

完成内容：

- 新增 `protocol/testdata/auth-v1.json`，固定测试专用 device key、双方 nonce、challenge、agent identifier 与预期 HMAC-SHA256，供 Swift/C# 跨语言读取。
- Swift 与 C# 均新增 HELLO、AUTH_CHALLENGE、AUTH_RESULT 载荷的严格长度/枚举/版本校验，以及协议规定的 HMAC-SHA256 响应计算和恒定时间比较。
- 两端均新增会话门禁：未认证时拒绝 SCREEN_INFO、JPEG/H.264、心跳和全部输入消息；握手消息顺序错误也被拒绝。
- Agent 门禁不接受调用方传入的“认证成功”布尔值，而是保存收到的 32 字节响应，并在完成认证时自行恒定时间比较预期响应；错误响应进入 closing。
- 本轮没有网络监听、TLS、证书、密钥持久化、屏幕采集、JPEG 发送或输入功能。测试向量不是真实设备凭据。

Mac 验证：

- `swift test --disable-sandbox --scratch-path /tmp/prd-p1-remotecontroller-build`：通过，原 6 项帧测试和新增 6 项认证/状态测试合计 `12 tests, 0 failures`。
- `swift build -c release --disable-sandbox --scratch-path /tmp/prd-p1-remotecontroller-release`：通过。
- Ruby/OpenSSL 独立复算 `auth-v1.json`：HMAC 与预期值一致。
- XML 项目文件解析和 `git diff --check`：通过。

Windows 待验证：

```powershell
git pull --ff-only origin main
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
if ($LASTEXITCODE -ne 0) { throw 'Release build failed' }

dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
if ($LASTEXITCODE -ne 0) { throw 'Protocol/auth tests failed' }
```

预期：构建 0 错误，测试打印 12 条 PASS 和 `12/12 tests passed`，其中 authentication golden vector 必须与 Swift 相同。本轮 Mac 没有 .NET SDK，不能把 C# 静态检查标记为实际构建通过。

下一步唯一目标：完成上述 Windows 构建与 12 项测试，修复发现的问题并更新本文件。通过后再设计最低 TLS 传输和证书首次信任/指纹固定；在此之前禁止真实屏幕帧传输。

## P1 认证门禁跨平台通过与 TLS 指纹切片（2026-09-27）

- 用户反馈 Windows 目标机实际运行扩展测试，`12/12 tests passed`；未报告编译错误或 HMAC 差异。因此认证门禁切片完成跨平台验证。
- 新增 `protocol/testdata/tls-v1.json`，固定测试 DER 字节和 SHA-256 指纹；它不是有效生产证书。
- Swift/C# 均新增证书指纹计算和 TOFU 决策：首次连接返回待用户批准的指纹，不自动持久化；匹配已固定指纹时放行；证书变化时拒绝。
- Mac `swift test`：累计 15 项测试、0 失败，其中新增指纹 golden vector、首次信任和匹配/变化拒绝 3 项。
- 本轮尚未建立实际 TLS socket、生成证书、访问 Keychain/Windows 证书存储或启动监听端口；不能宣称 TLS 已通过。

Windows 下一步验证：

```powershell
git pull --ff-only origin main
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
```

预期输出 15 条 PASS 与 `15/15 tests passed`。通过后下一唯一目标是实现最小 TLS 客户端/服务端连接，并将用户批准后的指纹分别持久化到 macOS Keychain 和 Windows 受保护存储；仍不发送屏幕帧。

## macOS Keychain 指纹持久化切片（2026-09-27）

- 用户确认 Windows 端证书指纹/TOFU 扩展测试 `15/15 tests passed`，该跨语言切片验证完成。
- 新增 `TrustedFingerprintStore` 抽象和 `KeychainTrustedFingerprintStore`。Keychain 条目使用固定 service、设备标识作为 account、32 字节指纹作为 data，并设置 `AfterFirstUnlockThisDeviceOnly`；支持查询、覆盖和移除。
- 新增 `StoredCertificateTrustCoordinator`：首次连接必须由上层明确批准才写入；拒绝时不写入；后续匹配自动信任；指纹变化直接拒绝且不会再次调用首次批准回调。
- 自动测试使用内存 store，避免测试污染用户真实 Keychain。Keychain API 已参与 Release 编译，但真实 Keychain 写入留到 TLS UI 集成时手工验证。
- Mac `swift test`：累计 `18 tests, 0 failures`。
- 尚未实现实际 TLS socket、Windows 服务端证书生成/持久化或网络握手，不发送屏幕数据。

下一步唯一目标：实现只传固定测试消息的最小 TLS Windows 服务端与 Mac 客户端，连接层调用现有 TOFU/Keychain 协调器；服务端仅绑定 Tailscale 地址。真实 JPEG 必须继续等待 TLS 和应用认证串联通过。

## 最小 TLS 客户端/服务端实现（2026-09-28）

- Windows 新增 RSA 2048 / SHA-256 自签名服务端证书，带服务端认证 EKU、非 CA 约束和数字签名/密钥交换用途。首次运行后保存到当前用户 `My` 证书库，后续运行复用同一有效证书，避免正常重启触发指纹变化。
- Windows 新增单次 TLS 1.2/1.3 探测服务：最多等待 5 分钟，只处理固定且最多 32 字节的请求，返回固定响应后关闭；启动工具拒绝非 Tailscale IPv4 绑定，脚本只选择本机 Tailscale 地址，并限定当前唯一在线 Mac 的 Tailscale 源地址。未修改防火墙。
- Mac 新增基于 Network.framework 的 TLS 客户端。验证回调提取叶证书 DER，调用现有 TOFU/Keychain 协调器；首次连接打印 SHA-256 指纹并要求输入 `y`，证书变化直接拒绝。响应读取上限为 32 字节。
- 新增 `TLSProbeClient` 与 `TlsProbeServer` 命令行验收工具；它们只发送 `prd-tls-test` / `prd-tls-ok` 常量，不发送屏幕、设备密钥或其他业务数据。
- Windows 测试新增“自签名证书约束”和“loopback TLS 握手/TOFU/固定消息”2 项，目标机预期累计 `17/17 tests passed`。本 Mac 没有 .NET SDK，本轮不能把 Windows 编译或测试标记为已通过。
- Mac `swift test`：18 项测试、0 失败；`swift build -c release`：通过。SwiftPM 用户缓存警告来自 Codex 沙箱，不影响构建结果。
- `git diff --check`：通过。

Windows 目标机先验证并启动服务（仓库根目录 PowerShell）：

```powershell
git pull --ff-only origin main
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
if ($LASTEXITCODE -ne 0) { throw 'Release build failed' }

dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
if ($LASTEXITCODE -ne 0) { throw 'Protocol/TLS tests failed' }

.\scripts\p1\Start-TailscaleTlsProbe.ps1
```

预期测试为 `17/17 tests passed`。服务打印 `READY` 和 `CERTIFICATE_SHA256 ...` 后保持窗口开启。在 Mac 仓库根目录，用 Tailscale 中显示的 Windows IPv4 运行：

```bash
cd macos/RemoteController
swift run TLSProbeClient <Windows-Tailscale-IPv4>
```

Mac 首次显示的指纹必须与 Windows 的 `CERTIFICATE_SHA256` 完全相同；相同才输入 `y`。预期 Mac 和 Windows 均打印 `PASS`，Windows 随后打印 `CLOSED`。再在 Windows 重启同一脚本，并在 Mac 重跑同一命令；第二次不应询问批准，且两端仍应 `PASS`。这两次真实跨网络握手尚待用户执行，未完成前不得宣称 TLS 联调通过。

下一轮唯一目标：完成 Windows 17 项测试和上述两次跨网络 TLS 握手；若发现编译或运行问题先修复。通过后才把应用认证帧接入 TLS 通道，屏幕帧仍继续禁止。

## Windows 接收 TLS 交接与实际检查（2026-09-28，基于 5894910）

- 接收时 main 与本地 origin/main 一致，工作区干净。已完整阅读四份项目文档，并检查新服务端、证书代码、测试及启动入口。
- `dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release`：成功，0 警告、0 错误，包含新 TlsProbeServer 工具。
- `dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release`：退出码 1，16/17 tests passed；前 16 项通过，loopback TLS probe 失败，客户端报告 Received an unexpected EOF or 0 bytes from the transport stream。
- 为查看被客户端错误掩盖的服务端异常，在 Git 忽略的 artifacts/tls-diagnostics 中执行最小回环诊断：服务端 AuthenticationException 明确为 Authentication failed because the platform does not support ephemeral keys；内部 Win32Exception 为安全包中没有可用的凭证。诊断只使用临时证书及回环端口，未调用 AgentCertificateStore.LoadOrCreate、未向当前用户 My 证书库添加应用证书。
- 当前阻塞是 Windows TLS 私钥兼容性，不是 Tailscale 或防火墙；新回环测试未通过前不启动双机探测。下一步应修复私钥创建/生命周期与 Schannel 兼容性，并验证证书跨进程重用后仍有可用私钥、指纹稳定。
- 待回环通过后再执行两次真实跨网络 TLS：首次核对指纹并明确批准，第二次重启服务后自动使用 Keychain 固定指纹。仍需 Mac 端配合；本轮没有启动 Tailscale TLS 监听或真实 Keychain 验收。
- 本轮未修改产品代码、未重建安装包、未修改防火墙；仅更新交接中的过期测试数量、续接提示和阻塞结论。git diff --check 通过。未提交 Git。

## Windows 回环 TLS 私钥兼容问题修复（2026-09-28）

主要改动：

- AgentCertificateFactory 在 Windows 上将新自签名证书导出为带随机密码的内存 PFX，再以 UserKeySet 导入，让 Schannel 使用当前用户的私钥容器；未指定 Exportable。内存 PFX 用完立即 ZeroMemory，不写入文件。
- 普通 CreateSelfSigned 用于测试，不使用 PersistKeySet，释放后由平台清理其临时容器；AgentCertificateStore 创建长期身份时使用 PersistKeySet，以便证书对象释放、进程退出后仍能使用私钥。非 Windows 创建路径保持原行为。
- 在现有证书测试内补充私钥签名/公钥验签，以及 Windows CNG 非 ephemeral、不可导出的断言。测试总数保持 17。
- 存储标志语义参考：https://learn.microsoft.com/en-us/dotnet/api/system.security.cryptography.x509certificates.x509keystorageflags 。

验证结果：

- Release solution 构建：0 警告、0 错误。
- 协议/认证/证书/TLS 测试：17/17 tests passed，退出码 0，原来失败的 loopback TLS probe 通过。
- 额外在 artifacts/tls-diagnostics 的本地诊断工具中调用实际 AgentCertificateStore.LoadOrCreate，两次独立 dotnet 进程均成功使用同一证书、同一 SHA-256 指纹，完成固定指纹校验和回环 TLS 请求/响应。真实指纹仅在进程间捕获比较，未写入仓库。
- 上述额外验证已在当前用户 My 证书库创建/复用本应用的持久化证书身份，供后续 TLS 服务继续使用；没有加入根信任库，不应为重测随意删除，否则 Mac 后续固定指纹会变化。
- git diff --check：通过。没有新开 Tailscale 监听、修改防火墙或安装系统软件。

限制与下一步：本轮只完成 Windows 修复与本机验证；Mac 测试未重跑，Swift 代码未改。两次真实跨网络 TLS 握手及 Keychain 首次批准/重连仍待完成，应用认证尚未接入 TLS，不传输屏幕。现有 Setup.exe 未重建，不能视为包含新修复。无预装 .NET 环境安装验收继续保留。改动未提交 Git。

## 跨网络 TLS 联调准备（2026-09-28）

- 用户授权推送 Windows 修复并开始两次 TLS 联调。Windows 修复提交为 c4a85fc。
- GitHub HTTPS 推送遇到连接重置和 443 连接超时，尚未确认成功；不能把本地 commit 视为远程已更新。
- 准备检查时 Windows Tailscale 为 Running、本机在线，但在线 Mac 对端数量为 0；47475 无监听。未启动 TLS 服务，待 Mac 热点及 Tailscale 就绪后再开启 5 分钟窗口。
- 静态检查发现 TLSControllerClient.runProbe 的 ProbeState 仅被回调弱引用捕获，函数返回后没有强引用维持异步状态，可能导致回调不执行、CLI 最终超时。修正为超时回调强持有 state，保留 finish 的单次完成和取消语义。
- 新增 TLSControllerClientTests.testProbeCompletesAfterRunProbeReturns：先暂停回调队列，待 runProbe 返回后恢复，验证失败/超时回调仍会完成。使用回环和测试 store，不访问真实 Keychain。
- TLSProbeClient 显式将请求期限设为 180 秒、命令总等待设为 185 秒，给首次人工核对 SHA-256 指纹留出时间；仍必须由用户输入 y，不自动批准。
- 本机没有可用 Swift/macOS Frameworks，新增 Swift 修改尚未实际编译或测试。Mac 原有 18 项通过是历史结果，新增回归后预期 19 项，需要 Mac 运行 `swift test` 和 `swift build -c release` 再开始真实握手。git diff --check 通过。
- 真实跨网络 TLS 的首轮与第二轮均未开始；Windows 17/17 和本地证书复用结果仍有效。本轮未修改防火墙或安装软件。

## 双机跨网络 TLS 与 Keychain 验收通过（2026-09-28）

- 联调开始前本地 main 与 origin/main 跟踪分支均已同步至 119fced（含 c4a85fc Windows 修复）。此前推送网络阻塞已不再阻止本次联调。
- 用户提供 Mac 实际日志：Executed 19 tests, with 0 failures，Release Build complete；增量编译与 whole module optimization 的 remark 不是编译错误。
- 用户明确确认 Mac 和 Windows 在不同网络。Windows Tailscale Running，两端在线。
- 第一轮运行 scripts/p1/Start-TailscaleTlsProbe.ps1：READY，打印现有证书 SHA-256。只读核对 47475 确实仅绑定 Tailscale 地址；用户在 Mac 核对指纹并反馈匹配完成。Windows 输出 PASS TLS request received; protected response sent，随后 CLOSED，退出码 0；重启前确认端口释放。
- 第二轮重新运行同一服务，证书指纹与第一轮完全相同；用户在 Mac 同一终端用同一 Windows 地址重跑 TLSProbeClient，提供完整输出：PASS TLS handshake, stored fingerprint policy, and fixed probe response，未再次出现批准提示。Windows 同样 PASS / CLOSED，退出码 0。
- 两轮后再次通过 Get-NetTCPConnection 确认 47475 无监听。未修改防火墙或网络规则，没有常驻服务。真实地址、设备名、证书指纹和私钥未写入仓库。
- 结论：当前版本的真实跨网络 TLS 固定消息、首次人工 TOFU 批准、真实 Keychain 写入/跨客户端进程复用、Windows 服务端重启复用同一证书均通过。
- 此结果不等于应用层挑战响应认证、JPEG 或输入可用；本次只传固定测试常量。实际证书更换后的端到端拒绝仍只具备策略单测证据，未通过替换真实证书重测。
- 下一唯一目标：TLS 内应用挑战响应认证切片；无预装 .NET 安装验收继续保留。旧安装包未重建。本轮仅更新交接结果，git diff --check 通过，文档改动尚未提交。

## TLS 内挑战响应接入（2026-09-28，Windows 开发）

- 替换旧固定文本探针，TLS 后执行双方 HELLO、一次性挑战、HMAC-SHA256 响应、认证结果，仅成功才发送 8 字节 PING/PONG。保留证书 TOFU/固定指纹、Tailscale 单地址绑定和对端源地址限制。
- Windows 新增有界帧读写（64 字节探针载荷、序号检查），TLS 后 20 秒总期限；错误密钥延迟 1 秒并关闭。会话认证前消息由现有门禁拒绝。恒定时间比较，新会话随机数由系统 CSPRNG 产生。
- 新增 Windows Credential Manager 存储随机 key32 + identifier16，显式 `-ShowPairing` 只允许交互终端展示 Base64 密钥；测试用独立随机 credential target，finally 清理。没有展示或读取真实生产密钥到聊天/工具日志，没有更换现有 TLS 证书。
- Mac 新增隐藏输入 `--pair` 和独立 Keychain 密钥 service；探针默认必须已配对，不降级到旧文本。新增 `--wrong-key`（内存翻转一位、不覆盖存储）、`--preauth` 供负向验收。
- Windows 实测：Release solution 构建 0 警告/错误，25/25 tests passed。新增真实 TLS 正确密钥、错误密钥、认证前消息、跨会话旧响应、序号、超长头、半帧断开和期限覆盖；凭据存取复用及测试条目清理通过。原黄金向量继续通过，v1 线缆格式未变。
- Mac：新增 9 项测试，预期累计 28 项；代码仅静态检查，未在本机编译/执行。历史 19 项成功不能替代本轮证据。准确命令、配对步骤及三轮双端预期见 [TLS_AUTH_VALIDATION.md](TLS_AUTH_VALIDATION.md)。
- 限制：本轮仅命令行探针，WPF/SwiftUI 仍是骨架，未重建 Setup.exe/DMG。没有 Tailscale 实际监听、跨网络应用认证结果或实际 Mac 密钥 Keychain 结果；无预装 .NET 安装验收保留。未来常驻服务仍需跨连接失败计数和退避。
- 下一步：同步代码到 Mac，完成测试/构建及不同网络下三轮认证验收；通过并记录前不开始 JPEG。PowerShell 脚本语法和 git diff --check 通过。本轮改动按用户要求纳入本地 Git 提交；远程由用户自行推送。
## Mac 应用认证测试通过（2026-09-28，用户反馈）

- 用户确认代码已提交、Mac 本轮 28 项测试验证完成。按用户反馈记录测试通过，包含新增认证状态、帧边界及隔离 Keychain 测试；未将此结果扩大为真实设备配对或跨网络认证通过。
- 本地 main 与 origin/main 跟踪分支一致。Windows 25/25 测试及 Release 通过结果沿用；此次仅更新验证记录。
- 待确认：Mac Release 构建。下一步私下配对，然后按 TLS_AUTH_VALIDATION.md 完成正确密钥、错误密钥、认证前 PING 三轮双机验收，每轮重启一次 Windows 监听。
- 本次未开启监听、读取或显示真实密钥、修改防火墙。文档更新尚未提交。
## 配对完成与 Windows 状态 JSON 读取修复（2026-09-28）

- 用户提供 Mac Release Build complete 与 PAIRED 输出，确认真实设备密钥配对成功；不再要求重新配对。尚无本轮跨网络应用认证 PASS。
- 用户使用 Windows PowerShell 启动服务时 ConvertFrom-Json 报错，服务尚未启动。本机原生捕获未复现该错误，不能断言唯一根因；已消除对控制台默认编码的依赖，使用 ProcessStartInfo 显式 UTF-8 读取 stdout/stderr，并加 10 秒进程期限。
- 解析失败只输出固定错误，不回显可能含个人设备信息的 JSON。未修改系统执行策略、防火墙、证书或配对密钥，未启动监听。
- PowerShell 5.1 与 7 分别验证真实 Tailscale JSON、中文 JSON 往返、畸形 JSON 脱敏拒绝，全部通过；git diff --check 通过。
- 下一步在更新后的仓库执行 powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\p1\Start-TailscaleTlsProbe.ps1，看到 READY 后才在 Mac 执行正确密钥探针；如失败只反馈固定错误，不发送完整 status JSON 或密钥。修复及验证记录未提交。
## 认证联调暂阻塞于 Mac 离线（2026-09-28）

- 用户重试后已通过 JSON 解析，停在在线 Mac 数量检查。本机只读查询确认 Tailscale Running、Windows Online=True；唯一 Mac 对端 OS=macOS、Online=False，在线 Mac 数量为 0。不是系统标识筛选不匹配，尚未进入 TLS/应用认证。
- 将脚本的零在线与多在线错误分开，零在线明确提示在 Mac 连接 Tailscale 并检查网络。保留在线及源地址限制，不绕过检查、不修改防火墙。脚本语法及 git diff --check 通过。
- 下一步由用户恢复 Mac Tailscale 在线后重启 Windows 探针，出现 READY 再运行 Mac TLSProbeClient；无需重新配对。当前未启动监听，跨网络认证仍待验证。
## 应用认证三轮双机验收通过（2026-09-28，用户提供双端输出）

- Mac：28 项测试由用户确认通过；Release 探针构建输出 Build complete，配对输出 PAIRED，实际设备密钥已存入 Keychain。Windows 25/25 与 Release 结果沿用。
- 正确密钥：Mac 输出 PASS TLS handshake, stored fingerprint policy, application authentication, and PING/PONG；Windows 输出 PASS TLS application authentication; protected PONG sent，随后 CLOSED temporary TLS listener。双方认证及测试数据链路通过。
- 错误密钥：Mac authenticationRejected；Windows AuthenticationException，随后 CLOSED。脚本 TLS probe server failed 是预期非零退出的包装提示，不是新的缺陷。
- 认证前 PING：用户先重复提供了错误密钥结果，随后说明 Mac 命令输入有误并重新执行 --preauth；最终 Mac 反馈 FAIL TLS probe: unexpectedRespon（原文截断），Windows 明确 FAIL TLS protocol rejected (AuthRequired)。以服务端 AuthRequired 为认证前业务拒绝的证据，不将前一次重复结果计为第三轮成功。
- 第三轮用户未提供 CLOSED 行；本机随后只读查询确认 47475 Listen 数量为 0，临时监听已释放。
- 按此前不同网络联调安排完成本次双机操作；结果来自用户提供的双方输出，本机未重新独立核验两端物理网络。TLS 内应用认证切片通过，不等于 JPEG、输入或完整远程桌面可用。
- 本轮仅补充验收记录，无需重复构建未变更的应用代码。git diff --check 通过。脚本修复与文档尚未提交；下一步按用户指示提交，之后再安排 JPEG 只读切片。

## 认证收口提交与 JPEG 开发授权（2026-09-28）

- 用户要求先提交认证收口，再开始 JPEG 只读画面传输。脚本修复及认证验收文档纳入本次本地提交；不推送远程。下一步实现 P1 JPEG 切片，不包含输入控制或 H.264。

## JPEG 只读垂直切片开发（2026-09-28，Windows）

- 先按用户要求提交认证验收与启动脚本修复，提交 `344e0b4`；未推送远程。
- 新增 SCREEN_INFO 严格编解码与 jpeg-v1.json 合成图像向量，两端使用同一数据；既有认证格式不变。JPEG 模式双方声明能力位，认证成功才创建采集器，错误密钥不会触发采集。
- Windows UI 提供开始与本地停止、共享状态及证书指纹；自动选择 Tailscale 本机地址和唯一在线 Mac。复用系统凭据与证书，单次会话。新增 GDI 主屏缩放采集，WPF JPEG 质量70，最高1280×720、10FPS，原生句柄每帧释放。
- 单帧 JPEG 后发 PING，匹配 PONG 后才采集下一帧；10秒帧交换期限，防止发送队列积压。屏幕物理尺寸/DPI变化前发新元数据。客户端输入不受理。
- Mac 新增图形只读查看器、显式首次指纹确认、Keychain密钥读取、取消入口、JPEG尺寸预检查/坏图丢弃、单个最新图像槽；UI定时取图，避免无限主线程任务队列。该代码本轮尚未在Mac编译。
- Windows Release solution：0警告/错误；协议/认证/TLS/JPEG `30/30 tests passed`。新增真实回环TLS JPEG、无认证不采集、未确认只保留一帧、分辨率变化顺序、取消释放资源、地址选择测试；原25项继续通过。
- 显式运行 ScreenCapture.Tests --capture-in-memory：31次真实主屏采集成功，3840×2160、DPI×100=14400，编码尺寸不超过1280×720，采集加解码循环约11.7FPS，GDI增长0；取消检查与共享合成JPEG解码通过。未保存或显示屏幕内容、未向网络发送真实屏幕。本结果不能代替双机FPS或耐久验收。
- Mac新增9项测试，累计预期37项；Windows无法验证Swift/macOS Frameworks。准确运行和打包命令见 JPEG_VALIDATION.md。下一步需用户在Mac同步本轮代码并运行测试/Release，之后双机查看、停止/关闭窗口、分辨率变化与30分钟验收。
- 当前JPEG实现尚未提交；未实现输入控制、H.264、DXGI优化、自动重连、常驻跨连接退避或并发忙响应。真实JPEG双机性能尚未验证；无预装.NET安装验收独立保留。
### 本轮打包与收尾检查

- `packaging/windows/build-installer.ps1 -Version 0.2.0` 实际成功，产物 `artifacts/windows/PersonalRemoteDesktopAgent-0.2.0-win-x64-Setup.exe`，49,244,077 字节；SHA-256 `36767344000ce7cc80bb0c6e155a8a73d545b5f7538b58672d843463aa756337`。
- 自包含发布版 WPF 主窗口在隐藏启动检查中完成初始化，并通过向该测试进程的窗口发送正常关闭消息退出（exit 0）。未点击开始、未自动共享。这个检查不是安装/卸载验收。
- 结束时本机 47475 无监听。`git diff --check` 通过，Swift 测试函数计数为37；未执行Mac编译、DMG或真实JPEG网络传输。
- 认证收口提交344e0b4已完成；JPEG实现、测试和交接仍为未提交修改，不自动推送远程。下一步先让Mac同步这些新文件并完成37项测试及Release构建。

## JPEG 提交与推送授权（2026-09-28）

- 用户要求提交远程代码。本轮 JPEG 实现、测试、共享合成向量及交接文档纳入独立提交，并连同认证收口提交推送 origin/main。此记录不预先断言网络推送成功，以 Git 实际结果为准。
- Windows 已完成的验证结果不变；Mac 37 项测试、Release 构建及双机 JPEG 验收仍待执行。安装包位于被忽略的 artifacts 目录，不纳入 Git。
