# 下一次 Codex 会话交接

## 已通过：Mac Command+C/V 弹出 Windows 开始菜单修复（2026-10-02，最新）

双机首次验证发现，在远程画面按 Mac `Command+C/V` 会弹出 Windows 开始菜单；`Control+C/V` 仅能操作 Windows 自己的剪贴板，不能替代跨设备同步，因此该轮结果判定不通过。根因是 AppKit 先发出 Command 的 `flagsChanged`，旧实现立即将其映射为 Windows 键按下，随后即使 C/V 被特殊处理，Windows 键的按下/释放仍会打开开始菜单。

Mac 现改为延迟 Command：Command 刚按下时只在本地跟踪，不立即镜像到 Windows；若下一键是 C/V，则消费该 Command 并执行跨设备剪贴板，整个序列不产生任何 Windows 键事件；若下一键是其他键或鼠标按下，则先补发 Windows 键，再保持原有 Windows 快捷键行为。单独按下并释放 Command 仍补发完整 Windows 键按下/释放。失焦、停止、断开与发送失败会清空所有延迟/消费状态，不留下远程持有键。Mac 全量测试现为 **125/125 passed**，新增“剪贴板快捷键绝不镜像 Windows 键”和“其他快捷键仍可补发 Command”两项回归；Windows 源码与协议未改变，无需重装 Windows。

用户已在安装修订 `8d88ac047cc9` 后完成双机复测并确认通过；此前 `Command+C/V` 弹出 Windows 开始菜单的问题已关闭，双向复制粘贴按本节清单通过。下一项优先处理 macOS ad-hoc 签名每次重建都会改变应用身份、从而重复触发 Keychain 密码授权的问题。

Mac 重新安装本提交生成的包后复测：

1. 在远程画面按 `Command+C` 和 `Command+V`，Windows 开始菜单不得出现。
2. Mac 本地复制唯一文本，再在 Windows 记事本按 `Command+V`，内容必须来自 Mac，且 RemoteController 显示粘贴成功。
3. Windows 选择唯一文本，在远程画面按 `Command+C`，切到 Mac 本地应用粘贴，内容必须来自 Windows。
4. 验证 `Command+R` 等非 C/V 组合仍作为 Windows 键组合发送；单独点击 Command 仍可打开开始菜单。
5. 最后停止控制并确认 Windows“持有 0”。

## 当前：Mac → Windows 文本剪贴板切片（2026-10-02，最新）

在上一切片 Windows → Mac 显式复制的基础上，已补齐反向文本粘贴：RemoteController 正在真实控制且远程画面持有焦点时，用户按 Mac `Command+V`，应用读取一次本机剪贴板纯文本，通过既有认证 TLS 会话发送给 Windows；Windows 在 WPF STA 线程写入系统剪贴板，成功后才注入一次隔离的 `Ctrl+V`，并返回明确结果。它不是后台剪贴板同步，不监控历史，也不传文件、图片或富文本。

安全与状态边界：

- 新增认证后消息 `ClipboardSetText` / `ClipboardSetResult`，继续复用双方 `ClipboardText` 能力协商。未协商、未认证、非真实控制会话、未请求响应、错误方向响应、非 `Success` 请求载荷和超限载荷均拒绝。
- 仍只允许 UTF-8 纯文本，最大 **32 KiB**。Mac 无文本或过大时不发送并显示原因；Windows 剪贴板忙时有限重试，失败时不注入 `Ctrl+V`。
- Mac 在发送前临时释放本会话已跟踪的普通键与修饰键，Windows 写入成功后注入隔离 `Ctrl+V`，随后恢复物理仍按住的键；快速重复 `Command+C` / `Command+V` 只保留一个剪贴板事务，不堆积，也不因此断开控制。
- Windows 回归同时验证写入成功才产生 `Ctrl+V`、四个注入事件顺序正确、结束后无持有键，以及写入失败不会注入。

本机已完成 Mac `swift test`：**123/123 passed**；`swift build -c release`：**Build complete**。本机没有 .NET SDK，Windows 源码尚未编译；Windows 预期 RemoteProtocol 仍为 **65/65**（扩展既有 clipboard TLS 测试，不增加测试入口），WindowsInput 仍为 **10/10**。

Windows 接手后先运行：

~~~powershell
git pull --ff-only origin main
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
dotnet run --project .\windows\RemoteAgent\tests\WindowsInput.Tests\WindowsInput.Tests.csproj -c Release
~~~

预期 Release 0 错误、RemoteProtocol **65/65**、WindowsInput **10/10**。随后构建并覆盖安装双方匹配的新包，再进行双机验收：

1. Windows 授权远程控制，Mac 连接并点击“开始控制”；在 Mac 本地复制英文、中文和多行文本，在远程 Windows 记事本中按 Mac `Command+V`，逐字核对内容与换行，RemoteController 应提示已粘贴。
2. Mac 剪贴板无纯文本、超过 32 KiB、快速连续两次 `Command+V` 时，会话仍连接；过大内容不应改变 Windows 剪贴板或产生粘贴。
3. 分别按住 Shift、Control、Option、Command 再执行粘贴，之后继续键鼠操作；停止控制、失焦、断开后 Windows“持有”必须为 0。
4. 回归 Windows → Mac：在 Windows 选择文本后从远程画面按 Mac `Command+C`，再切换 Mac 本地应用粘贴；确认双向功能可在同一连接内交替使用。
5. 回归只读模式：Mac 未请求控制时不能触发任一方向的剪贴板传输，也不能改变 Windows。

双机通过前不要把本切片标记为安装包验收完成。现有已安装 0.3.0 不包含新协议，必须使用同一提交构建的两端版本，避免能力或消息不匹配。

## 当前：Mac 失焦自动恢复与 Windows → Mac 文本剪贴板切片（2026-10-02，最新）

用户实测发现 Mac 切换到其他应用时会安全释放输入，但返回 RemoteController 后仍需再次点击“开始控制”。随后澄清第二项不是 Windows 配对密钥复制：实际是在 Mac RemoteController 中操作 Windows，复制 Windows 应用里的内容后，无法粘贴到 Mac。

本切片已修改源码：

- Mac 仍在应用或窗口失焦时立即发送所有按键/鼠标释放，避免 Windows 留下“持有”；但保留本次控制意图。回到 RemoteController 且窗口重新成为 key window 后，画面输入视图自动重新取得焦点并恢复捕获，不再要求再次点击“开始控制”。用户主动按 Esc、点击“停止控制”、断开或发送失败仍会真正停止，不会自动恢复。
- 新增认证能力 `ClipboardText`、显式 `ClipboardRequest` 和 `ClipboardText` 响应。只有双方声明能力、HMAC/TLS 认证完成且 Windows 本次明确允许真实控制时才可使用；Mac 只接受与本机未完成请求匹配的响应，拒绝服务端主动推送剪贴板。
- RemoteController 捕获远程画面中的 `Command+C` 后，临时释放已映射到 Windows 的修饰键，发送隔离的 Windows `Ctrl+C`、一次剪贴板请求，再恢复仍物理按住的修饰键；重复按键不会堆积请求或断开会话。Windows 等待 120 ms 让前台程序处理复制，仅在 WPF STA 线程读取一次纯文本，剪贴板忙时有限重试。
- 只传 Windows → Mac 的 UTF-8 纯文本，最大 **32 KiB**；无文本、过大和 Mac 写入失败均显示固定状态。不支持 Mac → Windows、文件、图片、富文本或后台监听，也不把剪贴板内容写入状态、日志或指标。
- Mac `swift test` **119/119** 通过，`swift build -c release` 通过，包含 payload 边界、UTF-8、能力协商、未请求响应拒绝、重复请求、Command+C 修饰键恢复及驱动回调测试。Windows 新增两项协议测试，预期 RemoteProtocol 从 **63/63** 增至 **65/65**，WindowsInput 仍为 **10/10**；本机没有 .NET SDK，Windows 编译与实测待交接。
- 先前从产品提交 `1e6a224fb33c` 生成的 macOS 0.3.0 DMG 不包含本剪贴板切片，不再作为本轮候选；尚未覆盖 `/Applications` 中的当前安装版。待 Windows 编译门禁通过并提交交接后，再从本切片产品提交生成双方匹配的安装包，避免协议能力不匹配。

Windows 接手后运行默认打包脚本，预期 Release 0 错误、RemoteProtocol **65/65**、WindowsInput **10/10**。覆盖安装新版 Agent 后，Mac 也必须使用包含本切片的新版。双机检查：在 Windows 记事本分别选择英文、中文和多行文本，在远程画面按 Mac `Command+C`，RemoteController 应提示已写入 Mac 剪贴板，随后切换到本地文本编辑器用 `Command+V` 粘贴并逐字核对。再验证无选区/非文本、超过 32 KiB、快速连续两次复制、复制后继续键鼠控制，以及按住 Shift+Command+C 后双方最终“持有 0”。同时复测失焦自动恢复；Esc/“停止控制”后切换窗口不得自动恢复。

## 当前：Windows 图形化配对重装与三次无密码重连通过（2026-10-02，最新）

已完成 Windows 更新、构建、旧 Agent 卸载及新版 D 盘安装；用户已确认 Mac 应用内配对连接成功，并完成三次完全退出、重新启动应用后的连接，均无系统密码弹窗。真实控制释放及部分密钥显示生命周期仍待单独确认，不扩展本次通过范围。密钥只能由用户从 Windows 界面直接输入 Mac，不进入聊天、日志、截图或提交。

### 本机实测通过

- 工作区原先干净，拉取 main 已最新；构建 HEAD 为 `978f32a79417`，包含产品代码 `2eda171`。
- Release **0 错误**，RemoteProtocol **63/63**，WindowsInput **10/10**；现有 .NET、Inno Setup 未升级或卸载。
- 新 Setup：`PersonalRemoteDesktopAgent-0.3.0-win-x64-Setup.exe`，**49273469 bytes**，SHA-256 `2a1ea07ec21705bb0fb6c641111e3d5450a50dbc5797482149e800aa30671306`。
- 关闭旧 Agent 后运行其卸载器，退出码 0。原 D 盘 Agent 目录、开始菜单目录、桌面快捷方式和固定 AppId 卸载项均确认移除；没有手工递归清理其他目录。
- 新 Setup 安装到 `D:\Program Files\Personal Remote Desktop Agent`，退出码 0；卸载项和两类快捷方式恢复，均指向新版 exe。
- 安装版 FileVersion `0.3.0.0`、ProductVersion `0.3.0+978f32a79417`；安装/publish exe SHA-256 一致：`6d30d96afdca9b8a1b77297ce93070a4028f6ba1d84cb612127de0bedbaa1f55`。
- 卸载前、卸载后、新装后均确认：`PersonalRemoteDesktop/Agent/v1` 存在且元数据（类型、长度、持久化、最后写入时间）一致；没有解引用或复制凭据 Blob。证书指纹、私钥存在性和有效期元数据一致，没有删除证书/私钥。旧 Mac Keychain 未操作。
- 同三个检查点 Tailscale 均为 `Running / Online=True`，节点身份、程序版本不变；未删除、卸载、注销或重置 Tailscale，也未修改防火墙。
- 新安装版启动成功，图形化配对按钮存在。检查时配对区域已经展开（按钮为“隐藏配对密钥”），仅读取按钮、共享状态及区域可见性，没有读取密钥文本。随后观察到用户已开始只读共享，配对区域不可见，按钮恢复为“显示配对密钥”且禁用；开始共享后的隐藏状态已实测。用户已完成实际 GUI 配对。
- 构建、卸载、安装日志为本机忽略产物 `artifacts/windows/pairing-build.log`、`pairing-uninstall.log`、`pairing-install.log`；凭据保留检查仅使用元数据，不保存密钥。

首轮连接未出图已收口：用户确认此前复制配对内容有误，正确复制后配对连接已通过。无需修改产品或清理凭据。首次配对/连接成功及三次完全退出、重启后重连无系统密码弹窗，均由用户明确回报通过。Windows 同时实测已经发送画面，Mac 断开后显示“Mac 会话已结束 · 等待已配对的 Mac 重新连接”，保持共享。此结论针对当前同一开发包，不代表未来 ad-hoc 签名升级后也不会触发 Keychain 授权。

### 剩余验收

1. “开始共享后隐藏密钥”已实测；显式点击“隐藏”和关闭窗口后重新启动的界面检查尚未明确回报。源码三条路径均清空/隐藏，仍需用户确认其余两项可见性。
2. 当前安装组合的双方授权真实控制及停止后“持有 0”尚未单独确认；本轮 Mac 断开后 Windows 继续等待已通过。
3. Windows 实体键盘紧急停止继续因环境不可用待验收，RDP 不替代该结果。
4. 保留原有配对和 Tailscale；不需要因本次复制错误重装或清空凭据。后续升级的稳定 Keychain 身份仍依赖正式签名方案。

## 历史任务：交给 Windows 更新、重装并验证图形化配对（2026-10-02，结果见顶部）

用户当前在 Windows 侧，授权下一会话更新开发环境相关内容并重新安装测试，但明确要求**不得删除、卸载、注销或重置 Tailscale**。也不得删除 Windows Credential Manager 中的 PersonalRemoteDesktop/Agent/v1、证书/私钥或 Mac 旧 Keychain 条目；不得在聊天、提交、日志或截图中暴露 Base64 配对密钥。

Windows 接手顺序：

1. 在仓库根目录确认没有需要覆盖的本地修改，然后拉取 main；目标至少包含 5deda3c，产品代码提交为 2eda171。保留现有 .NET 8、Inno Setup、Tailscale 及其登录/网络状态；除非构建明确报缺失或版本不兼容，不升级或卸载这些系统依赖。
2. 关闭所有 RemoteAgent 实例，运行：

   ~~~powershell
   powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\build-installer.ps1
   ~~~

   预期 Release 0 错误，RemoteProtocol **63/63**，WindowsInput **10/10**。记录新 Setup 的大小、SHA-256 和 publish exe 的 FileVersion/ProductVersion；ProductVersion 必须对应实际构建 HEAD。
3. 先记录当前安装目录、卸载项、快捷方式和 Personal Remote Desktop Agent 进程状态。只卸载旧 **Personal Remote Desktop Agent**，确认程序文件、开始菜单和桌面快捷方式被移除；Tailscale 应继续安装、登录并在线，Windows 配对凭据应保留。不要以“清理”为由删除 Tailscale、Tailnet 状态、Windows Credential Manager 配对目标、项目源码或开发工具。
4. 运行新 Setup，优先复用此前 D 盘目录；启动安装版，确认新增“显示配对密钥”按钮存在。点击后只由用户本人读取并直接输入到 Mac RemoteController 的 SecureField；不要让 Codex、终端、日志或截图读取该值。隐藏按钮、开始共享和关闭窗口都应清空界面中的明文。
5. Mac 当前已安装修订 2eda17164d57。用相同 Windows Tailscale 地址保存一次新 GUI 配对密钥，首次只读连接核对证书指纹；完成后断开、退出并重新启动 Mac 应用，连续连接至少 3 次，预期不再要求输入 macOS 用户密码。若仍弹窗，只记录弹窗标题、触发步骤和按钮，不提供密码或密钥。
6. 验证只读画面、双方授权真实控制、Mac 断开后 Windows 仍等待重新连接，以及停止共享后“持有 0”。Windows 实体键盘 Ctrl+Alt+Esc 因当前无实体键盘继续标记为环境受限待验收，不得用 RDP 冒充通过。

完成标准：Windows 构建门禁全部通过；旧 Agent 卸载和新版 D 盘安装通过；Tailscale 全程未删除且仍在线；Windows GUI 密钥显示/隐藏生命周期符合预期；Mac v2 配对成功并在至少 3 次重启/重连中不再弹系统密码框；真实控制停止后持有 0。完成后更新本文件顶部，提交并推送远程，交回 Mac。

## 当前：图形化配对与 Mac Keychain 弹窗收口（2026-10-02，最新）

用户反馈 Mac 连接时频繁要求输入 macOS 用户密码。根因与当前开发流程一致：设备密钥由 TLSProbeClient 命令行程序创建，图形应用是另一个 Keychain 访问主体；同时开发 DMG 仅 ad-hoc 签名，指定要求是会随产物变化的 CDHash，不能提供正式签名应用的稳定身份。

本切片已实现：

- Windows WPF 增加显式“显示/隐藏配对密钥”，仅用户点击后才展示；启动共享或关闭窗口会清空并隐藏文本。界面明确警告密钥不得发送到聊天、邮件或日志。
- Mac GUI 增加 SecureField 和“保存配对”；只接受解码后精确 32 字节的 Base64，以 Windows Tailscale 地址为 account 存入应用专用 v2 Keychain service。
- Mac 不再自动读取旧命令行 v1 设备密钥或指纹条目，因此不会为了旧 ACL 弹出密码框。旧条目不删除，可回退；用户需在新 GUI 中重新输入一次配对密钥，首次连接再核对一次证书指纹。
- 新增配对密钥解析正反测试；Mac Release 全量 **114/114** 通过。Windows 源码只能在 Windows 实机编译，Release/63/10 及 GUI 展示仍待后续验证。
- 已从源码提交 2eda17164d57 生成并覆盖安装新版 macOS 0.3.0：DMG **488647 bytes**，SHA-256 883eda0d887d10092b9a96e261a49ffad1d28530c711edaacdae9ff35789c81c；安装版二进制 SHA-256 7bda01241b5a285e25bd50e0ea633df9b0d96f1c31e299668ada388ab3778e6e。签名、DMG、安装后修订与启动均通过。尚未把真实 Windows 配对密钥写入 v2 条目，因此“保存一次后重复连接不再弹系统密码框”仍需双机人工验证，不能仅凭启动记为通过。

曾尝试将新条目切到 macOS Data Protection Keychain，实测返回 -34018（缺少经配置文件授权的 Keychain entitlement），因此没有把不可用路径留在产品中。当前 v2 仍是标准 macOS Keychain，但由图形应用自己创建，解决同一开发包日常连接的反复授权。要保证升级后仍有稳定 Keychain 身份，必须完成 Apple Developer ID 签名、配置文件与公证，不伪造 entitlement。

Tailscale 本切片不直接嵌入：官方 tsnet 是 Go 库，嵌入后应用会成为独立 Tailnet 节点；macOS 完整客户端还需系统扩展/VPN 用户授权，不能静默合并或绕过系统确认。MVP 继续使用官方 Tailscale 客户端；下一个可独立切片可在两端 GUI 增加“未安装/未登录/已连接”状态与一键打开官方安装或登录入口，但不代为安装系统扩展。

## 当前：macOS 0.3.0 覆盖安装与可恢复卸载已验收（2026-10-02，最新）

Windows 实体键盘 `Ctrl + Alt + Esc` 仍为**待验收**：用户当前无法使用 Windows 实体键盘，本项因验证环境不可用而延期，不记为通过或失败。RDP 不能代替该证据。下方 `d1b536b` 的 Windows 自动测试、注入拒绝和安装证据保留，待可接触实体键盘时按原清单继续。

在不阻塞其他独立验收的前提下，macOS 安装包切片已继续并完成：

- 从当时 HEAD `91ecda122bd4` 重建 `0.3.0` DMG；正常 macOS 权限下 Release **111/111** 测试通过，Release 应用构建、ad-hoc 签名校验和 `hdiutil verify` 通过。首次在受限执行沙箱中的 Keychain/系统 TLS 失败仅是环境拒绝；切换到正常本机权限后同一套门禁全部通过。
- DMG：`artifacts/macos/PersonalRemoteDesktop-0.3.0-macOS.dmg`，**478945 bytes**，SHA-256 `8bc46d0c73a8ed4e426d0fb1b3978ed71247409d2ac5468289c863278043b758`，与 `.sha256` 文件一致。
- 从已校验 DMG 覆盖安装到 `/Applications/RemoteController.app`；安装后版本 `0.3.0`、源码修订 `91ecda122bd4`，二进制 SHA-256 `b42b2b0452cc44f56f476f6aa569de24241aea494f10794b891e902d881c4399`，与包内一致，签名校验和启动通过。
- 覆盖安装前后 Keychain 设备密钥和证书指纹条目均存在，没有读取或输出密钥内容。
- 可恢复卸载检查已完成：退出应用后将 bundle 暂时移出 `/Applications`，确认应用本体移除，Keychain 配对仍保留；随后从候选产物恢复，哈希/签名一致并再次启动成功。
- 静态依赖仅为 macOS 系统库/框架与系统 Swift 运行库，Mach-O 为 `x86_64`，`LC_BUILD_VERSION` 最低 macOS `13.0`、SDK `14.2`。这是干净环境前置检查，不代替新 macOS 用户/无 Xcode 机器的实际启动。

本轮不删除 Keychain 配对材料，不将 ad-hoc 签名冒充 Developer ID/公证。剩余安装包验收为：Windows 实体键盘与真实控制释放、Windows 卸载残留、无 .NET 干净 Windows 启动，以及新 macOS 用户/无开发工具环境启动。

## 当前：Windows 紧急停止诊断与注入隔离修复已安装，实体键盘验收待完成（2026-10-02，最新）

Windows 本轮已定位复现条件、修复独立的注入隔离漏洞、完成自动门禁及 D 盘覆盖安装。**尚未证明实体键盘紧急停止通过，不得沿用历史“P2 全部通过”的结论关闭本项。**

### 实机证据与结论

- 已拉取 `0c4ffd0`；开始排查时 D 盘实际安装版为 `0.3.0+518fb1b2a0b4`，不是旧版 `d769777`。基线打包 Release 0 错误、协议 **63/63**、输入 **7/7**；本轮基线 Setup SHA-256 为 `b63fb3187812fe597580a67df05063538ed0cc610c8ba8c3fd4a8acfeb6f05f0`。
- Debug、Release 诊断版本均实测 `RegisterHotKey success=True error=0`、`SetWindowsHookEx success=True error=0`，窗口初始化正常。
- 用户确认此前通过 **Windows RDP 远程桌面**操作，并非 Windows 实体键盘。`query session` 显示应用与 Explorer 位于活动 RDP 会话。Release 日志收到修饰键回调，却没有收到用户尝试的 Escape 或对应 WM_HOTKEY；因此这次复现停在输入到达应用之前，不是已命中后的取消失效。RDP 路径的具体截获点未进一步确定，不能断言实体键盘链路失效或已经修好。
- 独立代码问题已确认：旧 WM_HOTKEY 分支绕过 Hook 的注入标志检查，且 GetAsyncKeyState 会混入远程注入修饰键。修复后安装版原生 SendInput 实测同时产生 `Escape injected=True matched=False` 和 WM_HOTKEY，证明两条路径都需要阻止注入触发。

### 修复内容（源码提交 d1b536b53568）

- 使用非注入 Ctrl/Alt/Escape 的按下/松开状态识别组合，区分左右修饰键，拒绝重复 Escape 与两类注入标志；注入修饰键不能为物理 Escape 授权。
- WM_HOTKEY 只用于诊断，不再单独调用停止。Hook 安装失败时禁止真实控制，即使 RegisterHotKey 成功也不能放行。
- RDP 会话显示“组合键可能被截获、可使用停止共享、物理紧急停止需在实体键盘验证”的提示。RDP 输入可能没有 LLKHF_INJECTED，不能用该标志证明键盘来自本机硬件。
- 可选 `PRD_EMERGENCY_DIAGNOSTICS` 诊断：注册结果/Win32 错误、首个回调、Escape 注入与修饰状态、停止请求、取消及清理。默认关闭；异步写盘，队列最多 256 条，每进程最多写 512 条，不记录普通键码、屏幕、证书或配对密钥。
- 依据：[Microsoft LowLevelKeyboardProc 文档](https://learn.microsoft.com/en-us/windows/win32/winmsg/lowlevelkeyboardproc)说明 Hook 在异步键状态更新前调用，应避免依赖当前事件的 GetAsyncKeyState；本实现直接维护经过注入过滤的转换状态。

### 构建、安装与验证

- Release：**0 警告 / 0 错误**；RemoteProtocol **63/63**；WindowsInput **10/10**。新增转换/重复、注入和左右修饰键释放回归。
- Setup：`artifacts/windows/PersonalRemoteDesktopAgent-0.3.0-win-x64-Setup.exe`，**49278970 bytes**。
- Setup SHA-256：`7c347e2802f6749cdb1f5853785f34ac3987698ac93fc44343c8cf448cdaf68f`，与校验文件一致。
- 同 AppId 直接覆盖安装，退出码 **0**；自动保留 `D:\Program Files\Personal Remote Desktop Agent\`。
- 实际运行 exe：FileVersion `0.3.0.0`，ProductVersion `0.3.0+d1b536b53568`。安装 exe 与 publish exe SHA-256 均为 `08beb252653a7bf2b15b4dd5a46bef3638ec497e8182f8957c39e6523f0c3d8c`。
- 覆盖前后证书元数据一致；未删除/重建配对项。**已有 Mac 配对能否继续认证仍需双机验证**，本轮没有以此代替实际认证结果。
- 安装版只读等待：单独投递 WM_HOTKEY 不停止；聚焦测试窗口后原生 SendInput 发送完整组合（包含释放）不停止；日志确实收到注入 Escape 并拒绝。最后点击“停止共享”，返回“共享已由本机停止”，开始按钮恢复。未开启真实控制或使用 Mac 注入。
- 本轮日志位于被忽略的 `artifacts/windows/emergency-*-build.log`、`emergency-install.log`、`emergency-diagnostics/`。原生负向检查脚本为本机产物 `artifacts/windows/emergency-installed-smoke.ps1`，未纳入源码。安装版当前保留打开、未共享。

用户已确认暂时不能使用 Windows 实体键盘，并要求记录待验收后提交推送。本轮不继续声称或尝试以 RDP 代替该验收。

### 剩余验收与接手顺序

1. 在 Windows **控制台会话和实体键盘**上启动上述 D 盘安装版，确保没有其他 Agent 实例。不要把 RDP 中的按键算作实体键盘证据。
2. 不连接 Mac，只读开始共享后按左 Ctrl + 左 Alt + Esc，必须立即结束共享、恢复开始按钮。必要时收集下列诊断日志，核对 Escape matched=True → RequestEmergencyStop → cancellation → cleanup。
3. Mac 使用原配对连接安装版并经双方许可开始真实控制；Windows 实体键盘组合必须结束共享、Mac 断开、最终持有 0。Mac 注入同组合应保持连接；这项双机结果不能由本轮本机 SendInput 负向检查替代。
4. 复测关闭/重启、只读和控制两种状态。实体键盘与真实控制释放验证完成前，本故障仍为**待验收**；之后再继续安装包卸载/干净环境检查。

诊断启动命令（先关闭旧窗口；环境变量仅影响当前 PowerShell 及其子进程）：

~~~powershell
Set-Location -LiteralPath 'H:\chatgpt\远程软件开发\remoteApp'
$env:PRD_EMERGENCY_DIAGNOSTICS = Join-Path (Get-Location) 'artifacts\windows\emergency-diagnostics'
& 'D:\Program Files\Personal Remote Desktop Agent\RemoteAgent.exe'
~~~

## 历史：Windows 安装版紧急停止兜底修复（2026-10-01，已被顶部修复替代）

两端 0.3.0 安装版核心回归中，默认只读、Mac 断开后 Windows 持续共享、授权真实控制三项通过；Windows 物理键盘 `Ctrl + Alt + Esc` 未结束共享。已排除多实例和远程修饰键：只有 1 个 D 盘安装版进程，且在不连接 Mac 的只读等待阶段按键仍无任何反应，窗口继续显示“等待已配对的 Mac 连接”。

当前安装版注册 `RegisterHotKey` 时未显示失败，但 WPF 没有收到可观察的 `WM_HOTKEY`。具体系统消息丢失原因尚未证明，不能继续把“注册成功”当作紧急停止可用的充分条件。现保留 `RegisterHotKey` 主路径，并增加 `WH_KEYBOARD_LL` 物理键盘 Hook 兜底；两条路径统一调用幂等停止函数。兜底明确拒绝 `LLKHF_INJECTED` 和低完整性注入事件，因此 Mac 经 `SendInput` 发送相同组合不能触发本机紧急停止。

新增纯逻辑测试覆盖物理 `Ctrl + Alt + Esc` 命中，以及注入标志、缺少修饰键和其他键拒绝。WindowsInput 预期由 **6/6** 增至 **7/7**，RemoteProtocol 仍为 **63/63**。本机没有 .NET SDK，C# 尚未编译；当前已安装的 `d7697772f98d` 候选包已被本修复取代，不能继续作为最终候选。

Windows 拉取后先运行默认打包脚本；它会完成 Release、协议 63/63、WindowsInput 7/7 和版本门禁，并生成带新源码修订的新 0.3.0 Setup。不要先卸载现有 D 盘版本，直接用相同 AppId 覆盖安装，以同时验证升级路径与安装目录保留。覆盖后先在不连接 Mac 的只读共享中按物理左 Ctrl + 左 Alt + Esc，确认共享结束；再在真实控制中复测，确认 Mac 断开且最终“持有 0”。

## Windows 安装路径问题已定位，安装版启动通过（2026-10-01，最新）

本机已完成只读排查并启动已安装的 0.3.0。自定义 D 盘安装正常，无需重装或修改注册表。此前精确筛选 `DisplayName = Personal Remote Desktop Agent` 得到 **0 项**，实际显示名称为 `Personal Remote Desktop Agent version 0.3.0`，宽松筛选得到 **1 项**，且 `InstallLocation` 正确。第二次原始错误全文仍未取得，但旧查询未命中的问题已实机复现。

实测结果：

- 安装位置：`D:\Program Files\Personal Remote Desktop Agent\RemoteAgent.exe`。
- FileVersion：`0.3.0.0`；ProductVersion：`0.3.0+d7697772f98d`。
- 安装 exe 与最终候选 publish exe 的 SHA-256 一致：`4763efc01680517bca5ae6747d03d2892c850057896c206e9e8684656be429ae`。
- Setup SHA-256 与校验文件一致：`ae9be3f93e2a182d8bd1bb224dae634e9ac32cfb11f5b41c07465f0df41313d7`。
- 开始菜单和公共桌面快捷方式均指向上述 D 盘 exe，目标存在。
- 已实际启动安装版，窗口响应正常；状态“未共享”，控制许可 Off，开始按钮可用、停止按钮禁用，无紧急停止快捷键不可用提示。应用保持打开，未启动共享、未授权或注入输入。

本机以后启动安装版使用：

~~~powershell
& 'D:\Program Files\Personal Remote Desktop Agent\RemoteAgent.exe'
~~~

跨机器定位应使用固定 Inno Setup AppId，避免依赖包含版本的显示名称：

~~~powershell
$keys = @(
    'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\{756FE82F-3D9F-4AB1-9652-3532142CB7A7}_is1',
    'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\{756FE82F-3D9F-4AB1-9652-3532142CB7A7}_is1',
    'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\{756FE82F-3D9F-4AB1-9652-3532142CB7A7}_is1'
)
$entries = @(Get-ItemProperty -LiteralPath $keys -ErrorAction SilentlyContinue)
if ($entries.Count -ne 1 -or [string]::IsNullOrWhiteSpace($entries[0].InstallLocation)) {
    throw 'Expected one installed Agent with a valid InstallLocation.'
}
$installed = Join-Path $entries[0].InstallLocation 'RemoteAgent.exe'
if (-not (Test-Path -LiteralPath $installed -PathType Leaf)) {
    throw 'Installed RemoteAgent.exe was not found.'
}
(Get-Item -LiteralPath $installed).VersionInfo | Select-Object FileVersion, ProductVersion
# 确认旧窗口已关闭后再启动：
# & $installed
~~~

本次仅修正文档中的定位方法与验收状态，没有修改产品或重新打包；未重跑未变更的源码测试。下一步按 [INSTALLER_PHASE_HANDOFF.md](INSTALLER_PHASE_HANDOFF.md) 使用两端安装版验证默认只读、双方授权控制、Mac 断开后 Windows 持续等待、Windows 紧急停止。随后验证覆盖安装与卸载、干净环境启动；这些尚未完成，不能标记安装包整体验收通过。Tailscale 连接、配对保留和紧急停止实际效果需要后续双机验证，不能由启动冒烟替代。

## 当前：0.3.0 双平台安装包基线（2026-10-01，最新）

P2 源码验收完成后，安装包阶段已开始。首个切片统一使用根目录 `VERSION`（当前 `0.3.0`），并加固 macOS DMG 与 Windows Setup 构建：打包前自动测试、产物版本与 Git 修订标识、产物存在性校验及 SHA-256 文件。详细命令、产物和后续安装验收见 [INSTALLER_PHASE_HANDOFF.md](INSTALLER_PHASE_HANDOFF.md)。

Mac 可在本机完成脚本实测；Windows 脚本涉及 .NET 8、Inno Setup 和 WPF，只能在 Windows 验证。开发包仍分别使用 ad-hoc 签名和未签名 Setup，不代表正式分发签名完成。

macOS `0.3.0` 已从提交 `50b959147609` 实际生成：Release **111/111**、构建、`.app` 签名、DMG 校验及只读挂载内复核全部通过。产物大小 `478800` bytes，SHA-256 `09a4e02d954f8d5204b19d722c34f8873efaa009ea2f3e38b09948b78999833b`。下一步在 Windows 拉取后运行 `packaging\windows\build-installer.ps1`，预期协议 **63/63**、WindowsInput **6/6** 并生成 `0.3.0` Setup 与校验文件。

Windows 首次生成的 Setup 为 `49277853` bytes，FileVersion 正确，但 ProductVersion 因 .NET SDK 自动追加修订而重复为“短哈希 + 完整哈希”。现已关闭重复追加并加入严格版本自检；首次包不作为最终候选。Windows 需拉取最新提交后重新运行脚本，ProductVersion 必须精确为 `0.3.0+<当前 12 位提交>`，随后再进入安装/卸载验收。

Windows 已从 `d7697772f98d` 重建最终候选：`49280682` bytes，SHA-256 `ae9be3f93e2a182d8bd1bb224dae634e9ac32cfb11f5b41c07465f0df41313d7`，FileVersion `0.3.0.0`，ProductVersion `0.3.0+d7697772f98d`，全部符合预期。下一步按 [INSTALLER_PHASE_HANDOFF.md](INSTALLER_PHASE_HANDOFF.md) 执行安装、安装版核心双机、覆盖安装和卸载验收。

## P2 真实键鼠 MVP 验收完成（2026-10-01，最新）

用户已完成最后的 RemoteAgent 关闭/重启检查，未出现“紧急停止快捷键不可用”提示。结合此前协议 **63/63 passed**、WindowsInput fake 边界 **6/6**、Mac 全量测试及 Release 构建，以及 [REAL_INPUT_HANDOFF.md](REAL_INPUT_HANDOFF.md) 第 1–10 项双机人工结果，P2 真实键鼠 MVP 现已全部验收通过。

P2 的开发进度、关键故障、根因、修复、最终证据和遗留边界已汇总到 [P2_PROGRESS_SUMMARY.md](P2_PROGRESS_SUMMARY.md)，后续阶段优先以该总结和本文件顶部状态为准。

已验证默认只读、双方显式许可、真实鼠标/键盘/修饰键/快捷键、黑边拒绝、触控板纵横滚动、所有释放路径、Windows 本机 `Ctrl + Alt + Esc`、Windows 主动停止与许可重置、Mac 断开后 Windows 持续共享、同一共享重新连接、Windows 已许可时 Mac 只读能力降级，以及 RemoteAgent 重启后的快捷键重新注册。所有停止路径最终“持有 0”。

当前仍是源码运行验证，旧安装包不包含这些最新改动。下一阶段应进入 Windows 与 macOS 各自安装包的重建、版本标识、安装/升级/卸载和干净环境验收；不要把源码验收结论直接视为安装包已通过。

## 已完成：Windows 已许可时兼容 Mac 只读连接（2026-10-01）

Windows 拉取持续共享版本后反馈：Windows 勾选“允许远程控制”，但 Mac 不勾选“请求远程控制”时连接失败，Mac 显示 `unexpectedResponse`。根因是 Agent 只要存在本机输入许可就强制要求 Controller 声明 Input 能力，把 Windows 的“最多允许控制”错误地当成双方必须控制。

现改为能力协商降级：Agent 仍声明 `JPEG | Input`，但 Mac 只声明 `JPEG` 时认证成功并进入只读 JPEG 会话，不创建 `SessionNativeInputSink`/`WindowsInputSink`；只有 Mac 同时声明 Input 才进入双向控制会话。纯 input-only 开发服务器仍强制要求 Input，原有负向门禁不放宽。新增“control-enabled agent accepts a read-only controller”真实 TLS/HMAC 回归，协议测试预期由 **62** 增至 **63**。本机无 .NET SDK，待 Windows 编译验证。

Windows 更新后执行 Release、协议 **63/63**、WindowsInput **6/6**，再勾选 Windows 控制许可并开始共享；Mac 不勾控制请求应能正常只读连接、显示画面且键鼠不影响 Windows。Mac 断开后 Windows 仍应继续等待，随后 Mac 勾选控制请求重新连接，应能手动“开始控制”。

Windows 首次运行新增协议测试时，两项分别报 `Probe payload exceeds 64 bytes`。原因是测试客户端误用仅供握手/输入小帧的默认 `ProbeFrameStream.ReadAsync` 读取 JPEG；产品服务端及 Mac 客户端不受影响。两项测试现改用与既有双向视频测试相同的完整帧读取方式，仍校验最大协议载荷和连续序号。Windows 需拉取最新提交后重新执行协议测试，预期 **63/63**。

用户已在 Windows 拉取测试修复并确认协议测试 **63/63 passed**。下一步只需启动最新 RemoteAgent，完成“Windows 已许可控制 + Mac 未请求控制”的只读双机连接，并继续验证 Mac 断开后 Windows 保持共享、同一共享可直接重新连接。

用户随后确认本轮 6 项双机验证全部通过：Windows 已许可控制时，Mac 不请求控制可以正常只读连接并显示画面，Mac 键鼠不影响 Windows；Mac 主动断开后 Windows 继续共享；无需 Windows 操作即可再次只读连接；再次断开后，Mac 改为请求控制也可在同一 Windows 共享中重新连接并手动开始控制。能力降级与持续共享修复均已通过实机验证。

最后的 RemoteAgent 关闭/重启检查已通过，没有出现“紧急停止快捷键不可用”提示。本节及持续共享修复现已完成。

## 已完成：Mac 断开后 Windows 持续共享修复（2026-10-01）

最终只读回归发现：Mac 主动断开后 Windows 同时结束了整个共享。产品此前调用单会话 `TlsProbeServer.RunOnceAsync`，客户端发送 DISCONNECT 后方法正常返回，WPF 因而进入共享结束清理；这不符合“Windows 持续等待、Mac 可重新连接”的产品行为。

现新增 `TlsProbeServer.RunContinuousAsync` 并由 RemoteAgent 使用：正常 DISCONNECT、客户端 EOF、认证/协议错误、会话读写超时只结束并释放当前会话，然后重新监听同一端口；Windows“停止共享”、本机 `Ctrl + Alt + Esc`、关闭应用或其他共享级取消才退出持续监听并清空本次许可。新增双会话回归，验证第一次 Mac 断开后第二次仍可认证、收图，且每次使用独立并正确释放的采集源与输入 sink。协议测试预期由 **61** 增至 **62**。

本机没有 .NET SDK，尚未编译。Windows 拉取后需运行 Release 构建和协议测试，再启动最新源码：Mac 主动断开后 Windows 应显示“Mac 会话已结束 · 等待已配对的 Mac 重新连接”，开始/停止按钮状态及本次控制许可保持；Mac 无需让 Windows 重新点击“开始共享”即可再次连接。Windows 主动停止和紧急停止仍必须真正结束共享并撤销许可。

### Windows 接手清单

在仓库根目录 PowerShell 中执行：

~~~powershell
git pull --ff-only origin main
git log -1 --oneline
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
dotnet run --project .\windows\RemoteAgent\tests\WindowsInput.Tests\WindowsInput.Tests.csproj -c Release
dotnet run --project .\windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj -c Release
~~~

预期 HEAD 至少包含 `afae283 fix: keep Windows sharing after Mac disconnect`，Release 0 错误；加入只读协商回归后协议预期 **63/63**，WindowsInput **6/6**。随后执行：

1. Windows 不授权控制并开始共享；Mac 以只读方式连接，确认画面正常。
2. Mac 点击“断开”；确认 Windows 不退出共享，开始按钮仍禁用、停止按钮仍可用，状态变为等待 Mac 重新连接。
3. Windows 不做任何操作，Mac 直接再次连接；确认画面恢复，键鼠仍不能操作 Windows。
4. 再次从 Mac 断开，然后由 Windows 点击“停止共享”；确认这次才真正结束监听，开始按钮恢复，控制许可为未选中。
5. 可选控制态回归：Windows 授权、Mac 控制后断开，确认“持有 0”且 Windows 继续等待；Mac 再连后需要重新点击“开始控制”，Windows 本次共享许可无需重复确认。最后用 Windows 停止或本机 `Ctrl + Alt + Esc`，确认共享结束并清空许可。

回传 Release、协议、WindowsInput 三组结果，以及第 2–5 项是否通过；任何失败需同时记录 Windows 状态文字和按钮状态。

## Mac 默认连接地址（2026-09-30，最新）

Mac RemoteController 的 Windows 地址输入框默认填写 `100.73.4.118`，仍允许用户手动修改。该值是当前 Windows 设备的 Tailscale IP，仅用于减少双机复测时的重复输入，不改变 TLS 指纹、设备密钥、HMAC 认证或控制许可边界。

## 当前：Mac 触控板滚轮拥塞修复，双机复测通过（2026-09-30，最新）

真实输入首轮人工测试中，鼠标移动/三键/拖动、黑边、文字/退格/Enter、四类修饰键、快捷键和最终持有 0 均通过；Mac 触控板双指纵向滚动稳定导致 Mac `congested` 并主动关闭，Windows 因此显示“Mac 已关闭连接”。这不是 `SendInput` 拒绝。

根因是每个精细滚轮事件生成 `move + wheel` 两帧，高频输入超过 100 帧/秒发送节奏并填满 64 项安全队列。现对连续、尚未发送的触控板 `move + wheel` 批次合并滚轮增量，保留第一次位置；按键、按钮、心跳等仍是不可跨越的顺序屏障，Int32 累加使用饱和边界。新增一万批次有界及边界/屏障 2 项回归：`InputSendQueueTests` **11/11**，全量 **111/111**、0 failures，Release `Build complete! (37.60s)`。

用户已用最新 Mac RemoteController 完成双机复测：触控板纵向滚动正常且不再断开；横向滚动正常且不再断开；停止滚动后连接保持，Windows 状态最终为“持有 0”。滚轮拥塞问题可以关闭。

真实输入释放路径第 7 项也已通过：Mac Esc、“停止控制”、切换应用/窗口失焦以及主动断开四种路径最终均为“持有 0”，停止后输入不再影响 Windows。失焦只释放输入并暂停捕获，远程画面和认证连接保持；返回 RemoteController 后只需再次点击“开始控制”，无需重新连接。只有单独执行“断开”才结束会话。

Windows 本机紧急停止第 8 项已通过：真实控制期间从 Windows 物理键盘按 `Ctrl + Alt + Esc`，强制停止按预期生效，共享结束、最终“持有 0”且 Mac 连接终止，远程输入不再生效。

Windows 主动停止与许可重置第 9 项已通过：Windows 点击“停止共享”后输入立即停止、最终“持有 0”且 Mac 自动断开；再次开始共享时控制许可已自动取消，未重新授权不能控制 Windows。

下一步只剩 [REAL_INPUT_HANDOFF.md](REAL_INPUT_HANDOFF.md) 第 10 项：双方均未授权的默认只读回归，以及 RemoteAgent 重启后全局紧急停止快捷键可再次正常注册。

## 当前：Windows 本机紧急停止切片，随真实输入一起验收（2026-09-30，最新）

为在首次真实 `SendInput` 验收前补齐独立于 Mac 焦点和远程鼠标的停止路径，RemoteAgent 现注册全局 `Ctrl + Alt + Esc`。收到快捷键后立即取消共享，沿既有会话清理释放键鼠，并显示“本机紧急停止”结果；关闭窗口时注销热键。若热键因冲突或系统错误无法注册，真实控制会被拒绝启动并提示原因，只读共享仍可用。

该增量仅修改 Windows WPF/PInvoke，本机没有 Windows SDK，尚未编译。它与下方真实输入接线合并按 [REAL_INPUT_HANDOFF.md](REAL_INPUT_HANDOFF.md) 验收：除原 1–9 项外，需要在持有远程输入时从 Windows 物理键盘按 `Ctrl + Alt + Esc`，确认共享结束、持有 0、Mac 断开且下一次仍需重新授权。不要使用 Mac 发送该组合来替代本机路径。

## 当前：真实 Windows 输入已接线，待 Windows/双机受控验收（2026-09-30，最新）

在产品 GUI 内存 sink 双机检查点全部通过后，本轮将 Windows 已认证、双方显式许可的输入分支接入 `WindowsInputSink`。Windows 复选框已改为明确警告“会真实操作此 Windows”，点击开始后还必须在本机警告框再次确认；只有随后通过 TLS/HMAC 认证且 Mac 再点击“开始控制”，才创建会话级 `SessionNativeInputSink` 并调用 Win32 `SendInput`。默认未勾选仍为只读。

新增会话状态包装器只记录事件/释放/持有数量，原生调用成功后才更新状态；停止、失焦、断线、协议/注入错误仍由现有双层持有状态尽力释放并失败关闭。每次共享结束都会自动清空 Windows 许可，下一次必须重新勾选并确认。Mac 文案已从 mock 改为真实控制警告，认证后仍需手动开始，Esc 仍立即暂停并释放。

Mac 全量 **109/109**、0 failures，Release `Build complete! (36.51s)`。本机没有 Windows/.NET/Win32 环境，Windows 代码尚未编译，真实桌面动作尚未执行，不能标记 P2 真实控制通过。Windows 必须按 [REAL_INPUT_HANDOFF.md](REAL_INPUT_HANDOFF.md) 先完成 Release、协议 61/61 和 fake 原生边界 6/6，再在已保存工作的安全窗口中执行逐项人工验收。未重新打包，旧安装包不含本轮接线。

## 当前：产品 GUI 控制 mock 双机验收通过（2026-09-30，最新）

用户已完成首轮双机 GUI 检查：自动构建/测试、连接、普通输入及默认只读回归通过；黑边以外操作正常。失败集中在 Windows 状态栏：Shift/Control 的持有变化不稳定，Esc、停止或断开后可能残留显示持有 1–2，Command 相对正常。

根因是 `SessionInputAuditSink` 对所有状态报告统一做一秒限流。短促的修饰键 Down/Up 已被服务端处理，但末尾 Up 状态可能没有再次刷新；显式 Up 已清空 `InputDispatcher` 持有集，断开清理便不会重复调用 ReleaseAll，界面因此保留旧快照。现改为键/按钮持有集合发生变化时立即发布，只有移动、滚轮和重复 Down 继续限流；Release 计数也只统计实际移除的持有项。本修复不改变协议、Mac 捕获或输入执行边界，Windows 仍只统计、不调用 `SendInput`。

用户随后拉取并运行修复版，确认全部检查项通过：Shift/Control/Command 持有变化可见；普通键鼠、黑边、拖出边界、滚轮、Esc、停止、失焦和断开释放均符合预期，最终持有为 0；暂停后不会自动恢复；默认只读回归通过；Windows 桌面没有真实键鼠动作。至此产品 GUI 内存 sink 双机检查点完成。下一切片可以把本次许可分支接入已有 `WindowsInputSink`，但必须作为单独、可立即停止的真实桌面人工验收，不能沿用本 mock 的通过结论。

## 历史验证：Windows GUI mock e91db1d 启动（2026-09-30）

用户报告找不到控制许可选项。当前 HEAD e91db1d 源码已有 ControlConsent；检查时未发现运行中的 RemoteAgent，无法确认先前打开的二进制版本。已实际完成 Release（0 警告/错误）、协议 61/61、WindowsInput fake 测试 6/6，并发布独立自包含目录 artifacts/windows/gui-control-mock-e91db1d。

已为用户打开该目录的 RemoteAgent.exe，UI Automation 确认控制许可复选框存在、可见、启用且默认 Off；未代为勾选或开始共享。旧 0.2.4 安装包不包含此 GUI 更新；双机 GUI 控制/释放验收仍待用户操作。本轮仅增加验证文档，按用户要求纳入本地提交；远程推送另行执行。

## 当前：产品 GUI 控制 mock 已接线，待 Windows/双机验收（2026-09-30，最新）

上一切片 Windows Release、协议 61/61、输入边界 6/6 已由用户确认通过。现已把同一连接的 JPEG + 输入能力接入两端产品 GUI，但仍停留在安全 mock：Windows 新增默认关闭的“允许本次远程控制测试”，仅勾选后才创建会话级内存统计 sink；该 sink 不引用 `WindowsInputSink`，不调用 `SendInput`，也不记录坐标、扫描码或按键内容。未勾选时保持原只读链路。

Mac 查看器新增默认关闭的控制请求、认证后“开始控制”二次操作，以及覆盖远程画面的输入画布。只有 Windows 本次许可、Mac 请求控制、TLS/HMAC 认证成功且用户再次点击开始后才捕获窗口内输入。Esc、停止、窗口/应用失焦会发送释放并暂停，恢复后必须手动再次开始；主动断开先排空释放再发送 DISCONNECT，异常断线由 Windows 服务端兜底释放。首次连接仍必须先走只读模式核对并保存证书指纹，控制模式不会自动信任新证书。

Mac 最终全量 **109/109**、0 failures；Release `Build complete! (6.54s)`，最终重跑无 Swift 并发告警。Windows 代码已做静态检查，但本机没有 .NET/Windows 环境，尚未构建；双机 GUI 人工验收也尚未执行。准确 Windows 命令、操作顺序与 PASS 标准见 [GUI_CONTROL_MOCK_HANDOFF.md](GUI_CONTROL_MOCK_HANDOFF.md)。通过前不得启用真实 `SendInput`。

## 当前：Mac 单连接双向调度器完成（2026-09-30，最新）

用户确认上一切片 Windows Release、协议 61/61、输入边界 6/6 全部通过。Mac 现可在同一 TLS/HMAC 连接中协商 `Jpeg | Input`、分片接收 SCREEN_INFO/JPEG，并继续使用有界输入队列、单写入、独立双向序号、心跳和释放后 DISCONNECT。开发用 Network.framework 客户端新增可选 JPEG 回调；input-only 行为保持兼容。

新增 5 项测试后，本机受控沙箱外全量 **109/109**、0 failures；真实 TLS 双向用例同时验证 JPEG 接收与 input drain。Release `Build complete! (19.33s)`。第一次普通文件沙箱运行的 8 个 Keychain/PKCS#12 权限失败不作为代码失败，第二次完整运行全部通过。准确范围见 [MAC_DUPLEX_HANDOFF.md](MAC_DUPLEX_HANDOFF.md)。

产品 RemoteController/RemoteAgent GUI 仍未接线，Windows 不会创建原生 sink，当前仍是只读。下一切片是产品级显式许可和生命周期接线；先以内存 sink 完成 GUI/双机验证，再单独开启真实 `SendInput` 人工验收。

## 当前：JPEG + 输入单连接双向服务端，待 Windows 验证（2026-09-30，最新）

用户确认上一切片 Windows prescribed tests 全部通过：Release solution、原生输入边界 6/6、既有协议 60/60，记录为 Windows 原生 `SendInput` 边界编译与 fake API 回归通过；仍未执行真实桌面注入。

本轮新增认证后的 JPEG + 输入双向服务端：输入读取与 JPEG/PONG 单写入分离，所有服务端出站帧共享连续序号；PONG 使用固定 16 项有界队列，视频帧期限、输入读取期限/限速和断线释放保持。新增真实 loopback TLS/HMAC 回归，预期协议测试由 60 增至 61。详见 [DUPLEX_SESSION_HANDOFF.md](DUPLEX_SESSION_HANDOFF.md)。

当前 WPF GUI 仍未传入 input session，不会创建 `WindowsInputSink`，产品行为继续只读。本机没有 .NET SDK，本轮 C# 尚未编译；Windows 需验证 Release、61/61 和原生边界 6/6。通过后的下一切片是 Mac 单连接双向调度器及 Network.framework 回环测试，之后才接 GUI 授权和真实输入。

## 当前：Windows 原生输入边界已实现，待 Windows 验证（2026-09-30，最新）

新增独立 `WindowsInput` 项目及 6 项无注入自动测试：协议绝对坐标到 Win32 `SendInput`、三键、横/纵滚轮、扫描码/扩展键/KeyUp、失败后尽力释放与重试，以及 win-x64 ABI 尺寸。原生调用逐事件检查返回值，sink 只在成功后更新持有状态。RemoteAgent 仅引用该模块但产品 GUI/TLS 尚未创建 sink，现有共享仍严格只读；自动测试使用 fake API，不会真实移动键鼠。

本机没有 .NET SDK，尚未编译 C#。Windows 拉取本轮代码后按 [WINDOWS_NATIVE_INPUT_HANDOFF.md](WINDOWS_NATIVE_INPUT_HANDOFF.md) 运行 Release solution、`WindowsInput.Tests`（预期 6/6）和原协议测试（预期仍 60/60）。未通过前不接生产 GUI。

下一切片是同一认证连接内的 JPEG 发送 + 输入接收双向会话；当前 `TlsProbeServer` 的视频和输入模拟仍为互斥分支，不能直接启用真实控制。完成双向传输后再接 Windows 本次会话许可和 Mac 查看器输入捕获，并验证所有停止/失焦/断线释放。

## 当前：双机 Tailscale input mock 验收通过（2026-09-30，最新）

用户确认 Windows 和 Mac 端测试均全部通过。按 [TAILSCALE_INPUT_MOCK.md](TAILSCALE_INPUT_MOCK.md) 的验收口径，记录为：Windows 预期 60 项协议测试通过，Mac 合成客户端 PASS，Windows 内存 sink 对 12 个合成事件的顺序和最终释放校验 PASS。一次性监听结束后关闭。

本结果完成了跨网 TLS/HMAC 输入 mock 检查点，但 Windows 端仍是内存 sink；产品查看器没有输入接线，没有 `SendInput`，不能记为真实桌面控制通过。下一阶段应先设计产品 GUI 的明确授权、可见控制状态、停止/失焦/断线释放和 Windows 原生 sink 边界，再实现真实输入。

## 历史验证：Windows c85d4bf 构建与测试（2026-09-30）

按用户要求从 71ed896 安全快进至 origin/main 的 c85d4bf，接收 Mac 验证、修复和双机 Tailscale input mock。Windows 实际 Release 构建 0 警告/错误，包含新增 InputMockServer；协议测试 60/60 全部通过，无需编译修正。Mac 104/104、Release、InputPreview 和本机真实 TLS 结果来自已拉取的 Mac 交接记录，本机未重复执行。

本轮未启动 Tailscale 监听或产品共享、未修改防火墙；双机 input mock 仍待按 TAILSCALE_INPUT_MOCK.md 进行。下一步启动一次性 mock 后让 Mac 发送合成序列，核对双端 PASS/12 项事件/释放，不启用真实输入。以上为 c85d4bf 时的验证及待办，后续进展以文档顶部最新交接为准；本次从暂存备份恢复并纳入提交。

## 当前：双机 Tailscale input mock 已实现，待 Windows/双机验收（2026-09-30，最新）

新增 Windows `InputMockServer` 与 `scripts/p2/Start-TailscaleInputMock.ps1`：只绑定本机 Tailscale IPv4，只接受唯一在线 Mac Tailscale IPv4，脚本和工具双层要求显式本地 mock 许可；处理器仅使用内存 `IInputSink`，逐项校验 12 个合成事件和最终释放，没有 `SendInput`。Windows 协议测试新增 Tailscale 端点边界，预期由 59 增至 60；当前 Mac 没有 .NET SDK，尚未执行 Windows 编译/测试。

新增 Mac `TailscaleInputMockClient`：只接受字面量 `100.64.0.0/10` IPv4，只读取该地址已有的 Keychain 设备密钥和已批准证书指纹，不能首次配对或自动信任。两端均以现有 `controller-input-v1.json` 约束同一串左/右 Ctrl、A 重复按下、鼠标、滚轮和释放序列。Mac 全量 **104/104** 通过，最终 Release `Build complete! (0.65s)`。

下一步在 Windows 拉取后严格按 [TAILSCALE_INPUT_MOCK.md](TAILSCALE_INPUT_MOCK.md) 执行：Windows Release + 60 项测试 → 带 `-AllowLocalMock` 启动一次性监听 → Mac 运行合成客户端 → 双端 PASS，且 Windows 桌面必须没有真实键鼠动作。完成前不接产品 GUI，不实现 `SendInput`。

## 当前：Mac 真实 NWConnection TLS 输入 mock 通过（2026-09-30，最新）

新增 `RealTLSInputSimulationTests`，使用真实 Network.framework `NWListener` / `NWConnection` 和仅测试自签名身份，严格绑定 `127.0.0.1`。5/5 实际通过：正确指纹+密钥完成 HMAC 认证并按顺序排空按下/释放/断开；错误指纹、错误设备密钥、认证后半帧断线和客户端取消均失败关闭。慢写入/背压继续由确定性替身测试覆盖，避免本机 TCP 内核缓冲导致假通过。

错误指纹在 Network.framework 上可能停留在建连阶段而不立即回调失败，因此 `TLSInputSimulationClient` 增加可配置连接期限，产品默认仍为 15 秒且只允许 `(0, 300]`。全量 Swift 实测 **103/103** 通过，Release `Build complete! (25.48s)`。测试身份只存在测试 target，不进入产品运行时。

安全边界不变：客户端仍固定回环，Windows 模拟服务仍固定回环，没有跨网输入、产品 GUI 接线或 `SendInput`。下一任务是在不启用真实注入的前提下，设计并实现双机 Tailscale 一次性 input mock：Windows 只记录/校验收到的合成事件，严格限定 Tailscale 绑定地址、Mac 对端地址、证书指纹、设备密钥、显式本地许可和单次会话。

最后整理：2026-09-29。请以顶部「当前」小节为准；分隔线以下的目标、测试数量、失败及待确认描述均为历史记录。

## 当前提交检查点（2026-09-29）

用户要求先提交代码并等待后续验证。本次提交包含此前累积的 JPEG 三档画质、P2 协议/输入预览/队列/TLS 模拟适配及全部测试和交接文档。仅提交本地 Git，未推送远程；不继续开发新功能。Windows 最近 Release 0 警告/错误、59/59 已通过；Mac 累计预期 96 项仍待执行。下一步按 WORKFLOW_REVIEW.md 同步并验证；下文中尚未提交的描述是本次提交前的历史状态。

## 当前：Mac 自动验证完成（2026-09-30，最新）

用户已在 Mac 实际完成本轮第一检查点：`InputSendQueueTests` 9 项、`AuthenticatedInputSenderTests` 13 项、`InputConnectionDriverTests` 12 项均全部通过；全量 `swift test` 为 96 项全部通过；`swift build -c release` 成功，输出 `Build complete! (18.76s)`。这确认了当前提交的 Swift 编译、共享向量、队列/认证发送状态及调度替身测试，但不等于 InputPreview 人工交互、真实 NWConnection TLS mock、真实 Windows 输入或 P1 画质稳定性已经通过。

下一检查点：运行 `swift run -c release InputPreview`，逐项完成 [P2_INPUT_HANDOFF.md](P2_INPUT_HANDOFF.md) 中键鼠、左右修饰键、黑边、拖出边界、滚轮、失焦、Esc/停止及释放清单。完成前不扩展新的 P2 功能，不接生产 GUI 或真实 SendInput。

## 当前：InputPreview 部分人工验收（2026-09-30，最新）

用户已确认普通键鼠事件正常，Esc 与“停止并释放”正常。截图显示最终为“已停止 · 事件 3347 · 释放 20 · 持有 0”，左右 letterbox 黑边可见，证明该次停止清理完成；截图不能单独证明每种输入路径。左右修饰键、黑边拒绝、按下后拖出边界再释放、滚轮及失焦释放因测试方法不明确，暂未给出逐项通过证据。当前只能标记部分通过，下一步按状态栏“事件 / 释放 / 持有”计数逐项复测，尤其要求每种释放路径最终回到“持有 0”。

## 当前：InputPreview 左右修饰键修复（2026-09-30，最新）

用户复测确认黑边拒绝、拖出边界释放、滚轮、失焦释放均通过；普通键鼠、Esc/停止此前已通过。唯一失败为同时按同类左右 Shift、Control 或 Command 时“持有”只显示 1，而不同类型组合能显示 2。根因是 InputPreview 使用系统聚合修饰键状态，无法区分同类左右键。

已改为按每次 AppKit `flagsChanged` 的物理 keyCode 维护左右独立集合，停止/失焦时清空；新增同类双侧按下、单侧释放时聚合标志仍为真的回归测试。修饰键专项 7/7、全量 Swift 98/98 通过，Release `Build complete! (18.30s)`。同时显式拒绝 TLS 模拟和控制客户端的端口 0，修复全量重跑暴露的端口校验差异。当前只剩新版 InputPreview 同类左右修饰键实机复测，预期持有顺序为 1→2→1→0。

用户随后完成新版实机复测，同类左右修饰键均按 `1→2→1→0` 变化，确认修复有效。至此 InputPreview 的普通键鼠、左右修饰键、黑边拒绝、拖出边界释放、滚轮、失焦、Esc 和停止释放清单全部通过。下一检查点转为真实 NWConnection TLS mock 正负向验证；InputPreview 通过不代表生产 GUI 输入或 Windows SendInput 已启用。

## 当前：流程审查与 TLS 模拟适配（2026-09-29）

审查结论：产品方向无偏离，但 Mac 验证积压，P1/P2 不能宣称整阶段完成。Mac 最后实际通过 40 项；本轮继续既定 TLS 模拟适配切片后，累计预期 96 项，均需 Mac 重新运行。以 [WORKFLOW_REVIEW.md](WORKFLOW_REVIEW.md) 为当前接收入口，包含模块证据表、准确命令和下一步顺序。

新增 TLSInputSimulationClient / InputConnectionDriver：固定回环、显式证书指纹和本地许可、TLS/HMAC、同步有界输入入口、20ms 定时器、单读/单写和统一取消。新增 12 项 Swift 调度/替身测试，尚未在 Mac 编译执行；未接 GUI、未完成真实 Swift TLS 正负向握手，更不是跨网输入通过。Windows 本轮 Release 0 警告/错误，59/59 重新通过；git diff --check 通过。

下一优先检查点：同步完整代码 → Swift 筛选测试（9/13/12）→ 全量 96 → Release → InputPreview 实机 → 真实 TLS mock → P1 采集/画质/30 分钟。完成当前 Mac 验证前不再扩展新的 P2 功能或启用真实 SendInput；Mac 不可用期间可继续 Windows 采集诊断和现有问题修复。这是本轮审查后的开发顺序，不是额外的用户许可要求。

P1 StretchBlt 失败复验和约 9.4 FPS 的验收差距保留；GUI 仍只读，0.2.4 安装包未重打。现有累积修改尚未提交/推送，Mac 直接 pull 不能取得它们。未安装系统软件、改防火墙、开启跨网输入监听或开发中继。

---

以下为历史记录。

## 当前：有界输入队列与认证发送状态机（2026-09-29）

本轮完成传输无关的 InputSendQueue / AuthenticatedInputSender：默认 64 条待发 + 1 条写入中，相邻移动合并且不跨键鼠边界，唯一出口分配序号，最多 100 帧/秒；认证/能力/本地许可默认拒绝，心跳和认证/写入/结束期限、正常释放顺序及失败清空已实现。实际 TLS 适配器和调度器尚未接入，不能把状态机的失败标记称为真实 socket 已取消。

Windows Release 0 警告/错误、59/59 测试通过；新增共享 input-queue-v1.json 的合并后序列经真实回环 TLS 顺序/释放验证。Mac 新增 22 项，累计预期 84；Mac 不可连接，本机无 Swift/macOS SDK，未编译执行，不代表 Swift 合并或实际双机输入已通过。

Mac 同步后先运行 swift test --filter InputSendQueueTests（预期 9）、swift test --filter AuthenticatedInputSenderTests（预期 13），再 swift test（预期 84）和 swift build -c release；随后按 P2 清单验证本地 InputPreview。当前代码未提交/推送，直接 pull 不会取得新文件。准确步骤、接口约定与限制见 [INPUT_SENDER_HANDOFF.md](INPUT_SENDER_HANDOFF.md) 和 [P2_INPUT_HANDOFF.md](P2_INPUT_HANDOFF.md)。

下一切片：真实 TLS mock 适配器，有界 UI 入口、单一串行所有者/唤醒、定时 poll、单次写完成/接收/取消调度。InputPreview 本轮仍是本地同步统计；生产 GUI/TLS 保持只读，不含真实 SendInput。Windows mock 仍固定 loopback，未开跨网输入监听。P1 GDI 采集失败复验、画质/跨网 30 分钟与约 9.4 FPS 的前置问题仍保留；独立体验最终目标不变，未开发中继。未重新打包，累积改动未提交。

---

以下均为历史记录。

## 当前：独立产品目标与 Mac 输入预览（2026-09-29）

用户确认最终目标为接近 AnyDesk 的独立安装/连接体验，已记录 [产品路线](PRODUCT_ROADMAP.md)，长期不要求最终用户安装 Tailscale 或配置 SSH。当前阶段继续借助 Tailscale 验证基础功能，未开始自建穿透/中继或服务部署。

本轮新增 ControllerInputCapture 与独立 InputPreview：窗口内键鼠转换、左右修饰键快照、长按、黑边拒绝、边界外按钮释放、滚轮小数累积及停止/失焦统一释放；仅同步消费并显示计数，不存储原始输入，不连接网络、不注入桌面。产品查看器仍只读。

验证：Windows Release 0 警告/错误，58/58 自动测试实际通过；新增共享 controller-input-v1.json 的真实回环 TLS 测试确认客户端释放在连接保持时已完成，不依赖断线清理。Mac 新增 10 项状态机测试，累计预期 62；本机无 Swift/macOS SDK、Mac 仍不可连接，未编译运行，不能称 Mac UI 或跨语言实机联调通过。

Mac 接收后按 [P2_INPUT_HANDOFF.md](P2_INPUT_HANDOFF.md) 顺序：先同步完整代码 → swift test（预期 62）→ swift build -c release → swift run -c release InputPreview → 键鼠/修饰键/黑边/失焦/停止验收 → JPEG 画面前置验收。预览不需要 Windows 在线。具体命令和逐项复选清单已列出。

阻塞：GDI StretchBlt 实际采集失败仍待复验；画质和跨网 30 分钟未收口，约 9.4 FPS 不代表原 10 FPS 目标已通过。AppKit 焦点/组合键/修饰键状态、滚轮方向速度需 Mac 实测。现有 Windows 模拟 TLS 仅 loopback，Mac 预览尚未连接模拟 TLS。

下一开发切片：有界单写入输入发送、移动合并、释放顺序与拥塞退出，再接专用认证 TLS mock。真实 SendInput 留待画面前置验收后。没有开启共享、安装软件或修改网络设置。0.2.4 仍为旧只读画质安装包，本轮未重打。累积改动尚未提交/推送。

---

以下为历史记录；旧“当前唯一目标”和旧测试数量不覆盖上面的最新状态。

## Mac键码映射与回环TLS模拟输入（当前最新）

已按用户指示实现Mac物理ANSI键码映射、左右修饰键快照及失焦释放；Control→Ctrl、Option→Alt、Command→Windows，未自动交换Command/Ctrl，不支持Caps/Fn/媒体/额外布局键和IME。使用SDK Carbon常量，新增5项Swift测试，共预期52；Mac离线，未实际编译。

Windows复用真实TLS/HMAC认证新增固定loopback的输入模拟入口，能力/本机许可默认拒绝，未认证不创建sink。连续控制读取、序号/长度验证、令牌桶限速和空闲期限完成，所有退出统一释放FakeSink键鼠。新增11项TLS测试，Release 0警告/错误、57/57全部通过；其中使用共享mac-keymap-v1.json快捷键字节，不声称执行了Swift或真实双机互操作。生产GUI/TLS仍只读，没有SendInput和全局事件监听，未修改Tailscale/防火墙。

完整接收步骤及策略见 [P2_INPUT_HANDOFF.md](P2_INPUT_HANDOFF.md)。恢复Mac后先同步代码（当前尚未提交/推送），运行swift test预期52项、swift build -c release；此前47/44/40为历史记录。Windows测试预期57项。之后先完成GDI实际采集/画质/稳定性，再接Mac UI事件与模拟发送端，不直接启用原生桌面输入。0.2.4安装包仍为旧只读画质版本，本轮未重打。

## P2继续：键盘协议及统一释放（当前最新）

在已授权的P2基础范围内，新增Swift/C# KeyEventPayload及keyboard-v1.json：4字节扫描码/扩展标志/动作；基础set-1 make code 1...0x7f，严格拒绝0、前缀打包、非法标志/动作和错误长度。尚未做Mac键码映射、中文输入法、E1/Pause或真实注入。

原鼠标分发器扩展并重命名为InputDispatcher，IInputSink统一鼠标/键盘。普通键和扩展键分别跟踪；长按重复Down照常转发，重复或未匹配Up忽略。停止、断开、ERROR、非法输入、输入接口异常及Dispose统一释放两组；一组失败仍尝试另一组，保留失败组供Dispose重试且禁止重新授权。GUI/TLS保持只读、未声明Input能力、没有SendInput调用。

Windows Release 0警告/错误，新增7项键盘/混合释放测试，46/46通过（保留原39项）。Swift新增3项，累计预期47，Mac仍离线，本轮未编译/执行Swift。未重新打包；0.2.4仍是已有只读画质包，不含P2基础库增量。所有累积改动未提交。

Mac恢复后先安全同步已提交代码，再在macos/RemoteController运行swift test（预期47项、0失败）和swift build -c release。重点验证mouse-v1.json与keyboard-v1.json、扫描码范围及扩展键身份。具体步骤见 [P2_INPUT_HANDOFF.md](P2_INPUT_HANDOFF.md)；此前44/40项为历史期望。

下一可独立开发切片：控制端Mac键码/修饰键映射及纯函数测试，然后认证TLS控制消息读取/序号/限速与mock端到端，不直接接通OS输入。实际桌面控制前仍按JPEG_QUALITY_HANDOFF.md完成GDI采集复验、清晰度、停止重连和稳定性收尾，9.4FPS不记作原10FPS目标通过。

## P2启动：鼠标协议与模拟测试（最新交接）

用户要求记录下一阶段计划并开始后续开发，已授权P2基础模块。此前“不开始输入开发”的历史限制由本次指示更新为：可以开发协议/映射/模拟测试，真实输入接入仍须先收尾P1画面采集和稳定性。

### 后续路线（记录用户确认的安排）

1. 修复并复验 Windows GDI 真实屏幕采集失败。
2. Mac恢复后按JPEG_QUALITY_HANDOFF.md完成画质、双方停止/重连和连续运行；约9.4FPS尚未达到原至少10FPS目标。
3. 先做鼠标坐标、黑边、DPI映射，消息规范和Swift/C#共享向量、Windows模拟输入接口。
4. 再开发键盘扫描码及按键释放；真实鼠标/键盘接入前落实认证、输入能力协商、本机显式授权、断开释放和立即停止。
5. 最后双机验证点击、拖动、滚轮、文字与组合键，不开始H.264或自建中继。

### 本轮已完成

Swift/C#鼠标移动/按钮/滚轮严格编解码；aspect-fit黑边拒绝、边缘/中心归一化、Windows物理像素映射；mouse-v1.json共享向量（含有符号滚轮边界）。独立IMouseInputSink与MouseInputDispatcher只在mock测试调用，认证/输入能力/本机授权默认拒绝；停止、断开、异常、Dispose释放已跟踪鼠标按钮。产品GUI/TLS未接入，仍只读，未声明input能力，没有SendInput、键盘或真实桌面注入。

Windows Release 0警告/错误；新增7项，39/39全部通过。Swift新增4项，当前预期44，Mac离线无法编译/运行，不沿用旧40项通过为本轮证据。现有0.2.4安装包仍是此前只读画质版，不包含本轮P2基础库增量；本轮不改变可安装产品行为。

### Mac恢复后的顺序

本轮代码尚未提交/推送，先完成同步再在Mac仓库根目录运行：

~~~bash
cd macos/RemoteController
swift test
swift build -c release
~~~

预期44 tests、0 failures和Build complete。重点检查mouse-v1.json解析、Int32滚轮边界、横/竖屏黑边和非法几何测试；遇到编译差异先修正再继续。JPEG_QUALITY_HANDOFF.md中的历史40项应以本轮44项为准。

Windows复验命令：

~~~powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
~~~

预期39/39。然后继续P1真实采集、画质/性能验收；这些独立阻塞并未被鼠标模拟测试解决。后续接入TLS时还需持续读取控制消息、连接结束统一Dispose、限速与UI坐标方向适配，本轮不声称已能远程控制。所有累积改动未提交。

## 2026-09-29 Mac离线期间：720p三档画质（最新，以本节为准）

用户补充此前突发断开当时为Mac网络问题，恢复后低带宽/标准均约9.4FPS，但文字仍模糊；目前Mac不可连接，授权先完成Windows可验证工作。

新增低带宽40/标准70（默认）/清晰85三档，仍最高720p/10FPS；结束后指标标记为最后成功发送。采集与编码分离，新增同一快照三档对比工具。Release 0警告/错误、32/32通过，GUI三档/默认/关闭通过；自包含0.2.4及Setup已生成，未安装。合成图各档10次编码解码通过，清晰档在该样本较标准增加约37%字节，不能推断网络FPS或真实清晰度。

本轮真实采集在StretchBlt失败，原因未确认，不能标记回归通过。Mac源码未改，Mac测试/双机画面未执行。下一步严格按 [JPEG_QUALITY_HANDOFF.md](JPEG_QUALITY_HANDOFF.md)：1.Windows真实采集 → 2.Mac同步/40项测试 → 3.标准60秒 → 4.清晰60秒 → 5.双方停止重连 → 6.跨网30分钟及安装。9.4FPS未达到原至少10FPS目标，不标记P1完成。

新程序 artifacts/windows/jpeg-quality/RemoteAgent.exe，安装包0.2.4。当前改动未提交，Mac直接pull无法取得新文档；下方均为历史记录，以本节和专用清单为准。

## 2026-09-29 网线下真实共享仍发生突发超时（最新阻塞）

用户重新连接后，只读 Windows UI 每 2 秒采样，未并行执行测速。初始帧 420，30.154 秒帧 700（约 9.3 FPS）；40.182 秒帧 790。42.192 秒读到帧 800 后持续不更新，52.248 秒状态变为“共享结束：发送画面超过 10 秒（网络发送超时）”，采样提前结束，不能记作 60 秒通过。

成功帧诊断采样显示采集编码 59–89 ms、网络写入按整数显示 0 ms、每帧约 86–87.5 KiB；0 ms 是四舍五入显示，不代表绝对零耗时。停止期间界面保留最后成功帧 800 的 82/0/82 ms，因此不能用这些旧值排除正在写入的后续帧阻塞。包含冻结阶段的 7.27 FPS 汇总不作为正常传输帧率。

网线改善了正常阶段吞吐，但不能宣称已解决突发超时。Mac FPS/观感已通过异步问题询问，尚待反馈。下一步仍需 Mac 接收间隔、解码耗时与断开原因观测，区分链路突发停顿和接收端阻塞；不继续用降低画质或扩大超时掩盖。日志仅诊断文字，临时采样文件在 Windows TEMP，不纳入 Git，不含图像/地址/凭据。

## 2026-09-29 用户接入网线后的同方法复测（最新）

用户反馈连接网线，按此前相同 SSH 方法复测 3 轮各 1 MiB 合成传输（Compression=no）。Windows→Mac 写入至收满确认分别为 0.179/0.368/0.182 秒，对应 46.87/22.81/46.02 Mbps；此前为 3.015/4.512/3.007 秒、2.78/1.86/2.79 Mbps。远端收满耗时 0.185/0.196/0.184 秒，反向读取耗时 0.156/0.621/0.856 秒。每轮完整收到 1048576 字节，全部正常退出。

测试开始前 Windows Agent 已处于停止状态，仍保留旧帧 4 的发送超时指标；本轮未启动真实共享，旧帧指标不能作为接网线后的结果。当前短样本吞吐明显改善，支持先前网络条件是重要限制因素；未核验具体哪端网卡/路由切换，不宣称唯一根因，也不能将 SSH 短样本当成稳定带宽或 JPEG 验收。下一步重启一次共享并由 Mac 连接，重新采集 60 秒真实会话，再测双方停止与 30 分钟耐久。本轮只读诊断和文档更新，未修改应用代码或网络配置。

## 2026-09-29 SSH 联调通道与链路基线（最新）

用户确认 Mac 未合盖或休眠；Mac 同时显示连接结束 timeout。用户自行启用 Mac 远程登录、安装专用公钥并允许远程用户完全访问磁盘后，Windows 工具已能以 lipeng 独立 SSH 登录并读取项目目录。Mac 工作区干净、HEAD ce79973，RemoteController Release 进程存在。专用私钥仅存 Windows 用户 .ssh 目录，不提交地址、密钥或凭据；不需要使用用户终端窗口或传递密码。

通过已认证 SSH（Compression=no），进行了 3 轮各 1 MiB Windows→Mac 随机合成字节传输，不读取/传输文件或屏幕、不新增监听端口。上传从开始写入到收到远端收满确认分别 3.015/4.512/3.007 秒，折算 2.78/1.86/2.79 Mbps；远端收满耗时 2.854/4.411/2.795 秒。每轮反向另传 1 MiB 合成零字节，Windows 读取耗时 1.991/1.644/0.648 秒；包含 SSH 缓冲影响，不视作严格方向对称带宽测量。

结论：当前 SSH 路径吞吐偏低，与 JPEG 写入阻塞相符；100 KiB × 10 FPS 约需 8.2 Mbps，不含开销。此测试不是 TLS 流媒体或专用网络基准，不能证明具体为丢包、Tailscale、物理链路或接收端问题，也不能单独解释 10 秒停顿。未修改 Mac 代码、重启查看器或放宽超时。下一步对同一次 JPEG 会话观察 Mac 接收间隔和解码耗时，再结合链路对照定位；稳定性验收仍未通过。

## 2026-09-29 发送超时已实测确认（当前阻塞）

用户表示已连接后，Windows UI Automation 连续只读诊断文字 60 秒：采样开始时会话已经结束，状态为“共享结束：发送画面超过 10 秒（网络发送超时）”。末次成功帧 4：采集+编码 72 ms，网络写入 7064 ms，本帧总耗时 7136 ms，99.9 KiB。其后 60 秒均是保留的旧读数，不是连续传输样本；用户随后确认 Windows 又自动结束共享。不能将这一轮记作有效性能/耐久测试。

Tailscale 脱敏状态快照：Running，Windows 与 Mac 均在线、Mac Active、有直接端点、无 PeerRelay。该快照不能证明链路吞吐或丢包正常，也不能排除接收端停顿。静态检查 Mac 接收回调会在同步 JPEG 解码后发起下一次读取，但尚无实测解码耗时，不能归因为某一端。当前没有放宽 10 秒超时或继续降低画质。

已询问用户：Mac 结束时完整状态文字、当时是否保持唤醒且窗口正常打开。下一步结合 Mac 状态，安排同步的接收/解码耗时和网络诊断；先解决发送超时，再谈稳定 10 FPS/30 分钟。Windows 指标可由本机 UI Automation 按 AutomationId StatusText/MetricsText 直接读取，无需用户抄写；只读取这两个字段，不读取地址、证书、密钥或图像。

## 2026-09-29 连续发送实测与低带宽对照版（最新）

用户报告连续发送版 Mac FPS 在 0.6–10 之间波动，Windows 采集+编码约 80 ms、网络写入 200–900 ms、本帧总耗时 300–3000 ms、每帧约 180 KB。不同读数可能来自不同帧，不能直接相加；网络写入阶段存在明显阻塞，不能仅凭此区分链路吞吐、丢包、relay 或接收端处理。180 KB × 10 FPS 约 14.4 Mbps（不含协议开销），尚未达到稳定 10 FPS。当前场景是否视频/普通窗口及是否仍不同网络已询问，未收到答复。

已增加 Windows 画质选择：默认低带宽 JPEG quality 40，可切回原标准 quality 70。两档均保持最高 1280×720、10 FPS，认证和连续发送协议不变；共享期间禁用切换，停止后可切换。不是自动码率控制，也不承诺低带宽链路一定达到 10 FPS。诊断行增加实际采集帧号，便于记录同帧耗时。

验证：Release 构建 0 警告/错误，32/32 协议测试通过（含同帧指标与帧号校验）；31 次标准画质主屏内存采集通过，采集+解码约 13.5 FPS、GDI 增长 0；另一次 quality 40 内存采集/解码通过（118738 字节，只是当时画面样本，不可与此前 180 KB 作严格对照）。未保存或发送真实屏幕。self-contained 0.2.3 发布成功，路径 artifacts/windows/jpeg-low-bandwidth；未覆盖用户正在运行的旧目录，未自动共享。Setup 未重打，代码和记录尚未提交。

下一步：关闭旧 Agent，在仓库根目录执行下面命令，选择默认低带宽后开始共享，现有 ce79973 Mac 可直接连接，无需改 Mac 或重置配对。

```powershell
& .\artifacts\windows\jpeg-low-bandwidth\RemoteAgent.exe
```

在相同普通窗口画面观察约 60 秒，记录 Mac FPS 范围及 Windows 同一帧整行；停止后切标准模式再比较相同场景。重点观察帧大小、写入耗时是否下降和文字是否可读。若仍长时间阻塞，再依据 direct/relay、链路状况和接收端耗时细分原因；不以平滑 FPS 数值、放宽超时或缩小验收分辨率代替修复。双方主动停止和不同网络 30 分钟稳定性仍待完成。

## Windows 接收完成（2026-09-29，当前最新结果）

本节优先于下方接收清单及历史记录。已从 4475b04 快进同步到 origin/main 的 ce79973，接收连续 JPEG 发送改动。Mac 40 项测试、Release、输入交互与原停等版本 3.5 FPS 的结果来自 Mac 交接，本机未重复执行 Mac 验证。

- 实际 Windows 构建发现 MainWindow.xaml.cs 的 catch (JpegTransferTimeoutException ex) 存在未使用变量，因 warnings-as-errors 触发 CS0168。已移除未使用变量，不改变异常处理行为。
- 修正后 dotnet build windows/RemoteAgent/RemoteAgent.sln -c Release：0 警告、0 错误。
- dotnet run --project windows/RemoteAgent/tests/RemoteProtocol.Tests/RemoteProtocol.Tests.csproj -c Release：32/32 tests passed，含连续发送、分辨率变化顺序、512 KiB TLS 帧、慢接收写入超时和取消释放。
- 已成功发布 self-contained win-x64 0.2.2 到 artifacts/windows/jpeg-continuous。未覆盖旧诊断目录、未安装软件或修改防火墙、未自动开始共享。Setup.exe 未重打，本次修正和记录尚未提交。

下一步由双机操作完成：关闭旧 Windows Agent，在仓库根目录执行以下命令并点击开始只读共享；Mac 运行 ce79973 中的连续接收版本，使用原配对地址连接。

```powershell
& .\artifacts\windows\jpeg-continuous\RemoteAgent.exe
```

记录 Mac FPS 与 Windows“采集+编码 / 网络写入 / 本帧总耗时 / 每帧”整行；分别验证 Mac 断开、Windows 停止和再次连接，再进行不同网络 30 分钟验收。当前自动化结果不能代替真实 FPS、停止行为或耐久测试。保留整个发布目录，不单独移动 exe；原配对密钥及证书不变。

## Mac 接收任务（2026-09-29，本轮最新交接）

本节优先于下方历史记录。当前目标是验证地址输入与 FPS 修正，并定位 0.2 FPS、Windows 自动停止共享；尚未通过性能或稳定性验收。Windows 已完成 Release 构建、32/32 测试、自包含 0.2.1 诊断版发布。Mac 此前 37 项测试通过，本轮增加 3 项，预期 40 项，尚未在 Mac 运行。用户已授权本轮代码提交并推送；接收时核对远程同步结果。

1. 在 Mac 仓库根目录检查工作区并同步；如有本地修改先保留处理，不使用 reset --hard 或覆盖修改。

```bash
cd ~/Documents/ChatGPT/远程软件开发
git status --short
git pull --ff-only origin main
git log -1 --oneline
```

2. 阅读 README.md、docs/DEVELOPMENT_PLAN.md、docs/TEST_PLAN.md、本文件与 docs/JPEG_VALIDATION.md。检查本轮文件 FrameRateMeter.swift、FrameRateMeterTests.swift、RemoteControllerApp.swift、TLSControllerClient.swift 已同步。
3. 退出旧 Mac 查看器，再执行：

```bash
cd macos/RemoteController
swift test
swift build -c release
swift run -c release RemoteController
```

预期 40 tests、0 failures，Release 输出 Build complete，查看器正常打开。如果编译或测试失败，在 Mac 修正并记录真实结果，不用此前 37 项通过代替当前验证。

4. 未连接时验证地址可直接键入、退格及粘贴；连接期间地址锁定，断开后重新可编辑。使用原来配对的完全相同地址，不重置 Keychain 密钥或 Windows 证书。输入焦点修改尚未实机确认；后续打包 .app 也需回归。
5. Windows 侧关闭旧 Agent，运行 artifacts/windows/jpeg-diagnostics/RemoteAgent.exe，点击开始只读共享。旧 TLS 探针不能同时占用 47475。诊断产物仅在 Windows 本地 artifacts 中，不纳入 Git；0.2.0 Setup 不含本轮修改。
6. Mac 连接后确认真实画面更新，记录稳定接收后的 FPS（首帧开始统计，不能用连接等待时间稀释）。同时记录 Windows “采集+编码 / 发送 / 等待 Mac 确认 / 每帧大小”整行。若再次自动断开，记录两端完整状态文字、持续时间和当时网络；不记录密钥或屏幕内容。TCP noDelay 只是待实测优化，不能预判其已解决低帧率。
7. 分别验证 Mac 主动断开、Windows 主动停止：双方窗口保留、Mac 清屏，Windows 再次点击开始后可重连。用户已确认自动结束时两端窗口仍在，不应误判为程序崩溃。
8. 自动停止和低帧率解决后，才进行不同网络 30 分钟性能/稳定性验收。把 Mac 编译、测试、交互、FPS、两端停止状态及阻塞项更新到本文件。不要开始输入控制、H.264、自建穿透或中继，不安装大型系统软件、不修改防火墙。

可交给 Mac Codex：

> 接收 docs/HANDOFF.md 顶部“Mac 接收任务”的交接，先检查 git status 并安全同步 origin/main，阅读指定文档，执行本轮 40 项测试和 Release 构建，修正实际编译问题，再按清单验证地址输入、FPS 与 Windows 诊断版双机停止原因。当前 0.2 FPS 和自动停止尚未解决；保留现有配对与证书，不扩展输入/H.264 范围。完成后更新交接，区分实测结果和待验证事项。

## Windows 接收任务（2026-09-29，最新）

本节优先于上方已经完成的 Mac 接收任务。Mac 真实 Keychain 环境下 `40/40 tests passed`，Release 构建通过；用户确认地址键入、退格、粘贴、连接锁定及断开恢复全部合格。真实跨网画面持续显示，FPS 3.5；Windows 诊断为采集编码约 80 ms、发送约 0 ms、等待逐帧确认 106–800 ms、约 71 KiB/帧。未连接等待后停止属于初始监听期限，已连接会话本次未自动停止。

上述数据证明逐帧 PING/PONG 停等是主要吞吐瓶颈。本轮已改为 JPEG 连续串行发送：无捕获队列、一次只等待一个网络写入、TCP 发送缓冲 256 KiB、每次写入 10 秒期限；Mac 仍只保留最新解码图。认证探针 PING/PONG不变。协议、两端状态机、测试、Windows 指标文案和 JPEG 验收文档已同步。

Windows 下一步：

```powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
dotnet publish .\windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj -c Release -r win-x64 --self-contained true -o .\artifacts\windows\jpeg-continuous
```

预期 Release 0 警告/错误、`32/32 tests passed`。重点确认连续发送、慢接收网络反压超时、512 KiB TLS 帧及取消释放资源测试。通过后关闭旧 Agent，运行 `artifacts/windows/jpeg-continuous/RemoteAgent.exe`，与同一 Mac/Keychain 配对地址复测；记录 Mac FPS 和 Windows“采集+编码 / 网络写入 / 本帧总耗时 / 每帧”整行。再验证双方主动停止和不同网络 30 分钟稳定性。不要删除证书/配对密钥，不开发输入、H.264 或自建中继。

## 当前状态

- 认证收口提交为 `344e0b4`；用户已授权将其与本轮 JPEG 提交一起推送远程。P0 与 TLS 内应用认证三轮双机验收已完成。
- 本轮已实现 P1 JPEG 只读切片：Windows WPF 开始/停止共享，认证后 GDI 主屏采集、最高 1280×720/10 FPS、逐帧 PING/PONG 确认；Mac SwiftUI 查看器、尺寸校验、最新图像缓存及断开清屏。Windows Release 0 警告/错误、32/32 测试通过；真实主屏内存采集 31 次通过，未保存图像。
- 用户提供 Mac 本轮日志：37 项测试、0 失败（0.187/0.193 秒），JPEG 自动测试已通过。用户已完成 Release 启动并确认 Mac 显示真实 Windows 画面、持续实时更新。实际 FPS、停止/清屏与 30 分钟性能验收待完成，不能宣称 P1 整阶段通过。没有输入控制或 H.264。
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

Mac 40 项测试、Release 和输入交互已通过；Windows 连续发送版本已完成构建、32 项测试和发布。当前进行连续发送版本双机 FPS、双方停止及 30 分钟复测，按 [JPEG_VALIDATION.md](JPEG_VALIDATION.md) 继续记录 FPS、停止/断开及不同网络 30 分钟验收。此目标已由用户授权，不需要再询问是否开始 JPEG。

Windows 代码与采集已在本机验证，Mac 本轮自动测试已由用户日志确认通过，用户已确认 Release 图形应用显示 Windows 实时画面；余下交互及稳定性待验收。保留现有配对密钥和证书，GUI 复用同一 Tailscale 地址条目。旧 TLS 探针只验认证，不显示画面，不能与 GUI 同时占用 47475。不开发输入、H.264 或自建穿透，不修改防火墙或安装系统软件。
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
Windows Release 0 警告/错误、30/30 测试通过；真实 4K/150% DPI 主屏在内存采集 31 次通过，GDI 句柄无增长。Mac 已由用户确认37项测试、0失败。下一步在Mac运行swift run -c release RemoteController，并启动Windows RemoteAgent图形应用验证只读画面；Release构建与真实显示尚待确认。
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

## Mac JPEG 自动测试通过（2026-09-29）

- 用户提供实际日志：Executed 37 tests, with 0 failures (0 unexpected) in 0.187 (0.193) seconds。记录为本轮 Mac JPEG 自动测试通过，不扩大为 Release 或真实双机画面通过。
- JPEG 提交 e21e49d 与认证收口344e0b4已于此前成功推送并核对远程main；Git当时直接连接GitHub失败，临时使用现有系统代理127.0.0.1:10710后成功，未修改全局代理配置。
- 下一步：Windows运行RemoteAgent并点击开始只读共享，Mac运行swift run -c release RemoteController，输入此前配对的相同Windows地址连接。确认实际画面、FPS、停止与清屏，再做不同网络30分钟验收。旧TLS探针不得同时占用47475。
- 此次仅更新文档，未重复运行未变更的Windows代码测试，未启动监听。记录尚未提交。
## Windows 启动提示 Desktop Runtime（2026-09-29）

- 用户反馈 Windows 开发启动命令弹出需要 .NET 8 Desktop Runtime；Mac 已成功运行命令并打开查看器，显示等待已认证的 Windows 画面。Mac 窗口打开不等于网络认证或 JPEG 显示通过。
- 本机检查 dotnet --list-runtimes 确认 Microsoft.WindowsDesktop.App 8.0.31 与 Microsoft.NETCore.App 8.0.31 已在 C:\Program Files\dotnet，SDK 8.0.425 可用，PresentationFramework.dll 存在。不能据提示直接认定运行时未安装；用户终端/apphost 运行时发现路径的具体原因尚未复现确认。
- 已确认 artifacts/windows/publish/RemoteAgent.exe 自包含发布版和 0.2.0 Setup.exe 均存在。先直接启动发布版完成联调，不要求重装运行时、不修改注册表或全局环境。
- Windows 显示等待连接后，Mac 使用同一配对地址重新连接；当前尚无真实画面结果。文档记录未提交。
## 首轮真实 JPEG 画面通过与地址输入反馈（2026-09-29）

- 用户反馈 Mac 地址框不能直接键入，粘贴地址后能够连接，出现真实 Windows 画面且实时更新。记录首轮认证后的 JPEG 实时显示通过；尚未收到 FPS、持续时间、停止清屏等结果，不标记 P1 整阶段完成。
- 已检查 TextField 使用可写 host 绑定，仅在 connected=true 时禁用。尚需区分未连接时的键盘焦点问题与已连接后的预期锁定，已向用户询问。终端直接启动时缺少明确 AppKit 激活策略是候选原因，当前未在 Mac 复现。
- 当前 Windows 会话不能编译或实际验证 Mac UI。已有 37 项测试通过是此前版本证据，后续焦点修改需 Mac 实机重测；无需重置配对或证书。
### 地址输入焦点修正（待 Mac 实机验证）

- 根据用户“粘贴后可连接”的描述，先按未连接时键盘焦点问题处理；询问尚未收到回复，仍保留连接后禁用输入为预期行为。
- RemoteControllerApp 增加 AppKit delegate：仅 SwiftPM 非 .app 启动时设置 regular 激活策略、激活应用并将可接收键盘的窗口置为 key；地址 TextField 增加 FocusState，首次呈现和断开后获取焦点，连接期间继续锁定。
- API 依据：https://developer.apple.com/documentation/appkit/nsapplication/activationpolicy-swift.enum 。当前 Windows 不能执行 Mac GUI 编译/键盘验证，不能宣称问题已修复；git diff --check 通过。不新增模拟 SwiftUI 焦点的无效单元测试。
- Mac 同步后需 swift test（预期仍37项）、swift build -c release，退出旧查看器并运行 swift run -c release RemoteController。未连接时检查直接键入、退格、粘贴；连接后仍锁定，断开后可再编辑。也检查打包 .app 正常启动不受影响。
- 修正与最新实测文档尚未提交或推送。当前不重启正在使用的查看器或 Windows 共享服务。
## 2026-09-29 低帧率和自动结束诊断：当前优先事项

用户确认实际画面更新，但报告 FPS 0.2 和 Windows 自动结束共享。Mac 随之断开清空画面，两端窗口保留，并非程序退出；Mac 主动断开也会结束共享。自动停止时的完整状态与发生时间仍待采集，不能认定原因已解决，也不能标记稳定性通过。

本轮修改：Mac 从首张有效画面起使用单调时钟计算 FPS，排除连接等待对首个读数的影响；持续低帧率仍如实显示。两端启用 TCP noDelay，尚无跨网络性能结论。Windows 新增采集+编码、发送、等待 Mac 确认耗时及每帧大小；区分本机停止、连接/认证超时、发送超时及帧确认超时。保留每帧发送+确认总期限 10 秒。

Windows Release 构建通过，32/32 自动测试通过，新增帧确认超时分类/释放与 512 KiB 合成帧 TLS 传输测试。Mac 原有 37 项测试由用户确认通过；本轮新增 3 项 FPS 测试，预期共 40 项，尚待 Mac 验证，Windows 无法运行 Swift/macOS 测试。输入焦点修复也仍待 Mac 验证。

已成功发布独立 self-contained 0.2.1 诊断版，未覆盖旧运行目录，无需安装 .NET；保留整个目录依赖文件。先关闭旧 Windows Agent，在仓库根目录运行：

```powershell
& .\artifacts\windows\jpeg-diagnostics\RemoteAgent.exe
```

开始共享，由 Mac 连接，记录 Windows 耗时整行、结束后的完整状态及持续时间。旧 Mac 可以先配合收集指标。两端代码当前尚未提交，仅在 Mac git pull 不会获得本轮修改；同步后运行 swift test（预期 40 项）、swift build -c release、swift run -c release RemoteController，复测输入、FPS 和停止。诊断版双机实测、30 分钟性能及稳定性验收尚待完成。0.2.0 Setup 不含本轮修改，本次未重打安装包。
