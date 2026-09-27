# 下一次 Codex 会话交接

最后整理：2026-09-27。请以顶部的当前状态、下一会话唯一目标和末尾的跨网络 TCP 复测通过记录为准；中间按时间保留的失败、待确认及下一步描述均为历史记录。

## 当前状态

- P0 已完成：两端构建、6 项帧协议测试、开发版打包、Windows 本机安装冒烟、跨网络 Tailscale 双向直连和 TCP 请求/响应均已通过。P1 已开始安全会话门禁切片：Swift 与 C# 已实现 HELLO/认证载荷、HMAC-SHA256 和认证前消息拒绝；Mac 端累计 12 项测试及 Release 构建通过，Windows 端新增 6 项测试尚待目标机实际验证。TLS 与真实 JPEG 屏幕链路尚未开始。
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

## 下一会话唯一目标

在 Windows 11 目标机实际构建当前提交并运行扩展后的 12 项协议/认证测试，修正任何 C# 编译或跨语言 HMAC 差异。该验证通过后，下一切片才实现最低 TLS 传输与证书首次信任/指纹固定。当前不得启动真实屏幕采集、应用监听或发送 JPEG；不开发输入控制、H.264 或自建穿透/中继。

独立保留的安装验收待办：在无预装 .NET 的 Windows 11 x64 环境确认 self-contained 安装、启动与卸载。当前开发机已安装 .NET，因此这项仍未完成，不影响已获得的 P0 网络验证结论。

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

P0 历史预期为 `6/6 tests passed`。P1 当前代码在 Windows 复验时应输出 `12/12 tests passed`；本轮无需重新生成安装包，除非 Windows 构建修正影响发布内容。

## 可直接复制到新会话的提示词

```text
请继续开发当前仓库中的 Personal Remote Desktop MVP。

先完整阅读 README.md、docs/DEVELOPMENT_PLAN.md、docs/TEST_PLAN.md 和 docs/HANDOFF.md，并检查 git status。P0 两端构建、各 6 项协议测试、DMG/Setup.exe 打包、Windows 本机安装/启动/重复打开/卸载、跨网络 Tailscale 双向直连以及 Mac 手机热点到 Windows 的 TCP 请求/响应已通过。一次性 TCP 监听已关闭，无需重复这些验证，除非相关代码改变。

下一目标是确定并实现 P1 JPEG 只读链路的第一个可验证切片。先检查现有协议和认证状态机设计，明确最小交付与测试；真实屏幕帧只能在 TLS 与应用认证成功后发送。未认证时不能发送屏幕或接受输入，正式监听只绑定 Tailscale 地址。

不开发 H.264、鼠标键盘控制、自建穿透或中继；不擅自安装系统软件或修改防火墙。无预装 .NET 的 Windows 安装验收仍单独保留，不得标为完成。当前平台不能验证的部分给出另一端准确命令和预期结果。结束时更新 HANDOFF，记录改动、实际验证结果、阻塞及下一步。
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
