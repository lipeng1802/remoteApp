# 下一次 Codex 会话交接

## 当前状态

- P0 协议规范、共享测试向量和两端工程骨架已创建；Mac 构建/测试/DMG 与 Windows Release 构建、6 项协议测试、Setup.exe 打包均已通过。用户已确认当前 Windows 机器安装、启动、关闭后重复打开及卸载正常；Mac 使用手机热点、Windows 位于另一网络时，Tailscale 双向 ping 均已直连成功。无预装 .NET 环境及应用层 TCP/TLS 可达性仍待验证。
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
- 用户已确认 Mac 和 Windows 均安装 Tailscale、登录同一账号并能看到两台设备；Mac 使用手机热点，与 Windows 不在同一物理网络。Windows 到 Mac 的 Tailscale ping 直连成功（71 ms），Mac 到 Windows 的反向 ping 也直连成功（最近一次约 5 ms），跨外网 Tailscale 层验证通过。

已知 Windows 目标环境：

- Windows 11 25H2，x64。
- 主显示器为 4K；具体 DPI 缩放由 Agent 运行时检测，不写死。
- 用户已安装 .NET SDK 8.0.425 x64 与 Inno Setup 6.7.3；2026-09-27 已实际通过构建、测试和打包。发布配置为 self-contained win-x64，包含 .NET / Windows Desktop 8.0.31；无预装运行时机器上的安装启动尚待验收。
- 两台设备可以处于不同物理网络，但跨设备联调前必须授权加入同一个 Tailscale tailnet。

已创建：

- `protocol/PROTOCOL.md`：28 字节大端序帧头、消息类型、认证状态机、大小限制及错误码。
- `protocol/testdata/v1.json`：三组跨语言帧 golden vectors，以十六进制文本保存准确线缆字节。
- `macos/RemoteController`：SwiftPM、SwiftUI 占位应用、Swift 协议编解码器及 XCTest。
- `windows/RemoteAgent`：.NET 8 WPF 占位应用、C# 协议编解码器及无外部测试包的控制台测试运行器。
- `packaging/macos`：从 Swift Release 构建组装 `.app`、签名并生成 `.dmg` 的脚本。
- `packaging/windows`：发布 self-contained win-x64 Agent 并使用 Inno Setup 生成 `Setup.exe` 的脚本。

## 下一会话唯一目标

完成 P0 最后一项应用层基线：设计一个仅绑定 Windows Tailscale 地址的临时 TCP 测试监听，并从 Mac 验证跨外网端口可达。不得绑定所有接口、修改公网防火墙规则或把真实地址写入仓库。Tailscale 网络层跨外网双向直连已经通过，不需要重复。

Windows 安装、启动、重复打开及卸载已获用户确认正常；无预装 .NET 环境验证仍是待办。本次不开始屏幕采集、输入控制、H.264 或自建穿透/中继。

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

## Windows 端下一步：临时 TCP 端口验证

先在 Windows 仓库执行 `git pull --ff-only origin main` 并确认工作区干净。随后在 PowerShell 运行以下一次性监听器。它只绑定 Windows 的 Tailscale IPv4 地址，收到一个请求后自动关闭，不安装服务、不绑定所有接口。

```powershell
$port = 47474
$tailscale = "$env:ProgramFiles\Tailscale\tailscale.exe"
$tailscaleIp = (& $tailscale ip -4 | Select-Object -First 1).Trim()

if (!$tailscaleIp) {
    throw "未找到 Windows 的 Tailscale IPv4 地址"
}

$listener = [System.Net.Sockets.TcpListener]::new(
    [System.Net.IPAddress]::Parse($tailscaleIp),
    $port
)

try {
    $listener.Start()
    Write-Host "READY: 临时监听已启动，端口 $port"

    $client = $listener.AcceptTcpClient()
    try {
        $stream = $client.GetStream()
        $reader = [System.IO.StreamReader]::new($stream)
        $writer = [System.IO.StreamWriter]::new($stream)
        $writer.AutoFlush = $true

        $message = $reader.ReadLine()
        Write-Host "收到测试消息：" $message
        $writer.WriteLine("prd-p0-ok")
    }
    finally {
        $client.Dispose()
    }
}
finally {
    $listener.Stop()
    Write-Host "临时监听已关闭"
}
```

看到 `READY: 临时监听已启动，端口 47474` 后，保持 PowerShell 窗口运行并通知 Mac 端。Mac 将通过已登录的 Tailscale peer 信息发送一行 `prd-p0-test`，预期收到 `prd-p0-ok`；真实设备名和地址不得写入仓库。

如果 Windows 弹出防火墙提示或 Mac 连接超时，不要全局放行、不要修改公网规则。记录提示或错误后停止，等待设计仅限 Tailscale 地址、测试后立即删除的临时规则。
