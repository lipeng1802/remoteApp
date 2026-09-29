# P1 JPEG 只读画面验收

本轮实现认证后的主屏 JPEG 采集、传输与 Mac 显示。输入控制、H.264、自动重连均未实现。旧 TLSProbeClient/TlsProbeServer 继续用于固定认证探针；看画面请使用两端图形应用，不能用旧探针代替查看器。

## 当前验证结果

- Windows Release solution：0 警告、0 错误；协议/认证/TLS/JPEG 自动测试 `32/32 tests passed`。
- 实际主屏内存采集：31 次，3840×2160、DPI×100=14400（150%），JPEG 解码尺寸均不超过 1280×720，GDI 句柄增长 0；本次采集加解码循环约 11.7 FPS。这不是双机显示 FPS 或 30 分钟耐久结果。
- 自动测试覆盖真实回环 TLS 中的 JPEG、错误密钥不创建采集器、未确认不再采集、分辨率变更元数据顺序、本地取消及释放监听。使用合成图像，不保存真实屏幕。
- 2026-09-29 用户日志确认 Mac 本轮 37 项测试、0 失败。当前 Windows 不能自行执行 Swift/macOS Frameworks；用户已确认 Release 查看器显示真实 Windows 画面并实时更新。地址框键入问题、停止/清屏、双机 FPS 及 30 分钟验收仍待完成。

## Windows 启动

仓库根目录 PowerShell：

```powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
dotnet run --project .\windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj -c Release
```

如果开发启动命令弹出缺少 Desktop Runtime，而本机已有本轮自包含产物，可在仓库根目录直接运行（无需额外安装运行时）：

```powershell
& .\artifacts\windows\publish\RemoteAgent.exe
```

发布目录需要保留其中的全部依赖文件，不单独移动 exe。2026-09-29 本机检查已存在 Desktop Runtime 8.0.31，用户启动提示的具体发现路径问题尚未确认；此方式使用已验证的自包含发布版继续联调。
在打开的窗口点击“开始只读共享”。应用读取 Tailscale UTF-8 状态，只选择本机 Tailscale IPv4 和唯一在线 Mac。窗口应显示“等待已配对的 Mac 连接 · 端口 47475”。不启动旧 TLS 探针，避免占用同一端口。

初次等待/TLS 最多 5 分钟，应用认证最多 20 秒；认证成功后窗口显示正在只读共享。停止按钮或关闭窗口会取消会话；不会自动重新等待下一连接。结束后再次共享需重新点击开始。复用此前 Windows Credential Manager 密钥和当前用户证书，无需重新配对。

可选真实采集检查（只在内存中处理主屏，不落图、不联网）：

```powershell
dotnet run --project .\windows\RemoteAgent\tests\ScreenCapture.Tests\ScreenCapture.Tests.csproj -c Release -- --capture-in-memory
```

此检查必须显式传参数，不随协议测试自动采集屏幕。

## Mac 构建与查看

同步本轮代码后，在 Mac 仓库根目录：

```bash
cd macos/RemoteController
swift test
swift build -c release
swift run -c release RemoteController
```

预期 `40 tests, 0 failures` 与 `Build complete!`。打开查看器后输入此前配对所用的 **完全相同的 Windows Tailscale 地址字符串**，点击“连接”。如果 Keychain 中没有该地址的密钥，先按 TLS_AUTH_VALIDATION.md 配对；已有配对无需重做。

同一证书应自动通过已固定的指纹；首次使用新地址则在弹窗中核对 Windows 窗口显示的 SHA-256，完全匹配后批准。Mac 窗口依次显示连接、认证、等待画面和正在查看；画面等比例显示，周围留黑，不接受任何鼠标键盘远程操作。画面上方状态显示接收图像尺寸、约每秒更新的有效接收 FPS 和远端主屏物理尺寸。

点击“断开”或关闭查看器应清空画面并结束连接；Windows 也应结束本次共享。在 Windows 点击“停止共享”时，Mac 应退出播放并清空画面，可手动重试。没有自动重连。

## 双机验收清单

1. Windows 开始共享但 Mac 未连接/未认证时，不应采集或发送屏幕（自动化测试已有门禁证据）。
2. 连接后在 Windows 移动一个普通窗口或播放非敏感动画，Mac 应实时更新；记录两端输出/状态和显示 FPS，不提交屏幕截图。
3. 检查 4K/150% DPI 下画面完整、无裁切；编码缩放保持比例，横屏上限 1280×720，不把 4K 图像原尺寸持续发送。
4. 如方便改变主屏分辨率/DPI，下一帧之前应收到新 SCREEN_INFO；此项本机只具备合成源测试，真实显示设置变化待人工验收。
5. 分别测试 Mac 断开、Windows 停止、关闭应用，确认状态恢复、旧画面消失、47475 监听释放；再次开始后应重新认证。
6. 两台设备保持不同物理网络，持续 30 分钟；目标为 720p、至少 10 FPS。记录实际 FPS、CPU/内存及网络类型。未完成前不能标记 P1 整阶段通过。

## 流量与资源约束

- 每帧：SCREEN_INFO（仅首次或变化）→ JPEG → PING → 对应 PONG；上帧未确认不采集下帧，单次帧交换最多 10 秒。
- Windows 使用 GDI 缩放采集和 WPF JPEG 编码，质量 70，最高 10 FPS。没有后台采集队列；截图只在内存中，原生 DC/bitmap 每帧 finally 释放。不是 DXGI 优化版，不支持安全桌面或登录界面。
- Mac 校验压缩尺寸上限 8 MiB，并在解码前检查 JPEG 尺寸不超过 1280×720；坏图丢弃并保持消息边界。解码在串行网络队列进行，显示端仅留一个可替换的最新图像，UI 定时取用，避免每帧排队到主线程。
- 720p/10 FPS 是目标，不是已验证跨网络性能；往返确认策略在高延迟 Tailscale relay 下可能达不到目标，需实测后调整。
- 只服务一次显式开始的会话；第二个控制端不能加入，尚未实现给并发连接返回 SESSION_BUSY 的完整处理。认证失败延迟沿用现有单次服务，跨连接递增退避属于后续加固。

## 安装包

Windows 开发包版本使用 0.2.0，构建命令：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\build-installer.ps1 -Version 0.2.0
```

本轮已实际生成 `artifacts/windows/PersonalRemoteDesktopAgent-0.2.0-win-x64-Setup.exe`（49,244,077 字节）。自包含发布版 WPF 窗口初始化及正常关闭通过，未点击共享。本轮不会自动安装或替换已安装应用；新包安装/卸载回归及无预装 .NET 环境仍待验证。

Mac 在本轮测试与 Release 构建通过后，从仓库根目录构建开发 DMG：

```bash
./packaging/macos/build-dmg.zsh 0.2.0
```

当前 Windows 不生成或验证 Mac DMG。

实现依据：[GDI StretchBlt](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-stretchblt)、[Apple ImageIO 图像属性](https://developer.apple.com/documentation/imageio/image-properties)。
## 地址输入焦点回归（2026-09-29，待 Mac 验证）

用户确认粘贴地址可连接并实时显示，但无法直接键入。已补充 SwiftPM 直接启动时的 AppKit 激活与 TextField 初始/断开焦点，尚未在 Mac 验证。同步代码后退出旧查看器，重跑40项测试、Release构建并启动；未连接时检查字母/数字输入、退格与粘贴，连接后输入框应锁定，断开后可编辑。正式 .app 使用系统默认激活方式，需一并回归。
## 2026-09-29 低帧率和自动结束诊断：当前优先事项

用户确认实际画面更新，但报告 FPS 0.2 和 Windows 自动结束共享。Mac 随之断开清空画面，两端窗口保留，并非程序退出；Mac 主动断开也会结束共享。自动停止时的完整状态与发生时间仍待采集，不能认定原因已解决，也不能标记稳定性通过。

本轮修改：Mac 从首张有效画面起使用单调时钟计算 FPS，排除连接等待对首个读数的影响；持续低帧率仍如实显示。两端启用 TCP noDelay，尚无跨网络性能结论。Windows 新增采集+编码、发送、等待 Mac 确认耗时及每帧大小；区分本机停止、连接/认证超时、发送超时及帧确认超时。保留每帧发送+确认总期限 10 秒。

Windows Release 构建通过，32/32 自动测试通过，新增帧确认超时分类/释放与 512 KiB 合成帧 TLS 传输测试。Mac 原有 37 项测试由用户确认通过；本轮新增 3 项 FPS 测试，预期共 40 项，尚待 Mac 验证，Windows 无法运行 Swift/macOS 测试。输入焦点修复也仍待 Mac 验证。

已成功发布独立 self-contained 0.2.1 诊断版，未覆盖旧运行目录，无需安装 .NET；保留整个目录依赖文件。先关闭旧 Windows Agent，在仓库根目录运行：

```powershell
& .\artifacts\windows\jpeg-diagnostics\RemoteAgent.exe
```

开始共享，由 Mac 连接，记录 Windows 耗时整行、结束后的完整状态及持续时间。旧 Mac 可以先配合收集指标。两端代码当前尚未提交，仅在 Mac git pull 不会获得本轮修改；同步后运行 swift test（预期 40 项）、swift build -c release、swift run -c release RemoteController，复测输入、FPS 和停止。诊断版双机实测、30 分钟性能及稳定性验收尚待完成。0.2.0 Setup 不含本轮修改，本次未重打安装包。