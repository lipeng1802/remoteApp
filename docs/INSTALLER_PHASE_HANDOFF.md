# 双平台安装包阶段交接

日期：2026-10-01  
目标版本：`0.3.0`

> 2026-10-02 Windows 重装交接：最新 main 新增 Windows GUI 显示/隐藏配对密钥与 Mac GUI v2 Keychain 配对。Windows 可更新构建环境所需的仓库内容并重新安装 Agent，但必须保留 Tailscale 的安装、登录和网络状态，同时保留 `PersonalRemoteDesktop/Agent/v1` 凭据。完整步骤与保密边界见 [HANDOFF.md](HANDOFF.md) 顶部。

> 2026-10-02 更新：最新 Windows 候选已从 `d1b536b53568` 构建并覆盖安装到原 D 盘，Release 0 警告/错误、协议 63/63、WindowsInput 10/10。Setup 大小 49278970 bytes，SHA-256 `7c347e2802f6749cdb1f5853785f34ac3987698ac93fc44343c8cf448cdaf68f`。版本、安装/publish 文件一致性、只读等待、注入组合拒绝与停止按钮已实测。此前紧急停止复现使用 RDP，实体键盘及真实控制中的释放/断开、已有 Mac 配对认证仍待验证。详细证据以 [HANDOFF.md](HANDOFF.md) 顶部为准；下文 d769777 为历史候选。

> 2026-10-02 macOS 更新：用户暂时无法使用 Windows 实体键盘，该项保留为环境受限的待验收项，不用 RDP 替代。macOS `0.3.0` 已从 `91ecda122bd4` 重建，111/111、Release、签名及 DMG 校验通过；DMG 478945 bytes，SHA-256 `8bc46d0c73a8ed4e426d0fb1b3978ed71247409d2ac5468289c863278043b758`。开发机覆盖安装、启动、Keychain 配对保留和可恢复卸载/重装均通过；无开发工具的干净 macOS 环境仍待验收。

当前切片：可重复构建与产物身份基线。

## 本切片范围

- 仓库根目录 `VERSION` 成为双平台默认版本来源；命令行仍可显式覆盖，但只接受 `major.minor.patch` 数字格式。
- 打包会拒绝相关平台源码、打包脚本或 `VERSION` 存在未提交修改，避免产物内记录的 Git 修订与实际内容不一致；无关文档修改不影响构建。
- macOS 打包前运行 Release 测试，生成 `.app` 和 `.dmg`，同步设置短版本、Bundle 构建版本与 Git 源码修订，执行代码签名校验和 DMG 校验，并生成 SHA-256 文件。
- Windows 打包前依次执行 Release solution build、RemoteProtocol 和 WindowsInput 两组测试，再发布 self-contained win-x64 单文件程序并调用 Inno Setup。安装程序写入版本与 Git 源码修订，并生成 SHA-256 文件。
- 任一构建、测试、发布、签名、产物存在性或安装器编译失败，脚本都必须非零退出，不得留下“成功”结论。

## macOS 构建与检查

仓库根目录执行：

~~~bash
./packaging/macos/build-dmg.zsh
~~~

预期产物：

~~~text
artifacts/macos/RemoteController.app
artifacts/macos/PersonalRemoteDesktop-0.3.0-macOS.dmg
artifacts/macos/PersonalRemoteDesktop-0.3.0-macOS.dmg.sha256
~~~

当前默认是 ad-hoc 签名，仅用于开发验证。正式分发仍需要 `MACOS_SIGNING_IDENTITY`、Developer ID、公证和 stapling。

### macOS 实测结果

已从提交 `50b959147609` 执行默认命令并通过：

- Release 测试 **111/111 passed**，0 failures。
- Release `RemoteController` 构建成功。
- `.app` ad-hoc 签名通过 `codesign --verify --deep --strict`。
- `hdiutil verify` 确认 DMG 有效；只读挂载后再次确认镜像内应用签名有效且 Applications 快捷方式存在。
- 镜像内 `CFBundleShortVersionString` 和 `CFBundleVersion` 均为 `0.3.0`，`PRDSourceRevision` 为 `50b959147609`。
- DMG 大小：`478800` bytes。
- SHA-256：`09a4e02d954f8d5204b19d722c34f8873efaa009ea2f3e38b09948b78999833b`。

校验文件使用相对文件名，请在产物目录执行：

~~~bash
cd artifacts/macos
LC_ALL=C shasum -a 256 -c PersonalRemoteDesktop-0.3.0-macOS.dmg.sha256
~~~

## Windows 构建与检查

Windows 仓库根目录 PowerShell 执行：

~~~powershell
git pull --ff-only origin main
.\packaging\windows\build-installer.ps1
~~~

预期自动门禁：Release 0 错误、RemoteProtocol `63/63`、WindowsInput `7/7`。预期产物：

~~~text
artifacts\windows\publish\RemoteAgent.exe
artifacts\windows\PersonalRemoteDesktopAgent-0.3.0-win-x64-Setup.exe
artifacts\windows\PersonalRemoteDesktopAgent-0.3.0-win-x64-Setup.exe.sha256
~~~

Windows 安装器当前未做 Authenticode 签名；SmartScreen 提示属于预期的开发包限制，不应误记为正式分发就绪。

Windows `0.3.0` 最终候选已从 `d7697772f98d` 生成；版本与哈希结果见下文。无需因文档更新重复构建。

### Windows 首次构建反馈与修正

Windows 首次构建完成，安装器大小为 `49277853` bytes，FileVersion 为 `0.3.0.0`。ProductVersion 显示为 `0.3.0+490df3be3388.490df3be3388c47e86abc2a4d49d2ef5127e76c6`：脚本显式加入短修订后，.NET SDK 又自动追加了一次完整修订。功能未受影响，但该包不作为最终候选包。

脚本现显式关闭 SDK 的第二次追加，并在发布后强制检查：FileVersion 必须等于 `0.3.0.0`，ProductVersion 必须严格等于 `0.3.0+<12 位当前提交>`；不符合就停止，不再继续生成安装器。Windows 拉取后需要重建，旧 Setup 与校验文件会被同名最终候选包替换。

Windows 已从修复提交 `d7697772f98d` 完成重建，最终候选包元数据通过：

- 安装器：`PersonalRemoteDesktopAgent-0.3.0-win-x64-Setup.exe`
- 大小：`49280682` bytes。
- SHA-256：`ae9be3f93e2a182d8bd1bb224dae634e9ac32cfb11f5b41c07465f0df41313d7`。
- `RemoteAgent.exe` FileVersion：`0.3.0.0`。
- `RemoteAgent.exe` ProductVersion：`0.3.0+d7697772f98d`。

该文件现进入安装候选验收；未经安装、启动、覆盖安装和卸载验证前，仍不能标记 Windows 安装包完成。

若 PowerShell 执行策略阻止 `.ps1`，使用仅对本次进程生效的命令，不永久放宽系统策略：

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\build-installer.ps1
~~~

## 后续验收

1. 分别核对两个安装包内版本号、源码修订和 SHA-256。
2. 在开发机执行覆盖安装、启动入口和卸载残留检查。
3. 在干净 Windows 11 x64 与干净 macOS 用户环境验证无需开发工具即可启动；Tailscale 仍单独安装和登录。
4. 用安装版重复默认只读、授权控制、Mac 断开后 Windows 持续共享、Windows 紧急停止四条核心回归。
5. 正式外发前补齐 Apple Developer ID/公证和 Windows Authenticode 签名；本切片不伪造或绕过签名信任。

## Windows 安装候选验收顺序

1. 关闭所有源码版或发布目录版 RemoteAgent，复算 Setup SHA-256 并与 `.sha256` 文件比较。
2. 运行 Setup，可使用默认或自定义安装目录；未签名开发包出现 Windows/SmartScreen 提示属于已知限制，必须确认文件哈希正确后再继续。
3. 安装完成后从开始菜单启动，核对安装目录 exe 的 FileVersion/ProductVersion，确认窗口、Tailscale 检测和紧急停止快捷键正常。
4. 用安装版完成一次只读连接、一次授权控制、Mac 断开后 Windows 持续等待，以及 Windows `Ctrl + Alt + Esc` 停止。
5. 不先卸载，重复运行同一个 Setup 做覆盖安装；确认版本不变、程序仍可启动、已有证书和设备配对未被破坏。
6. 从“已安装的应用”卸载；确认程序、开始菜单/桌面快捷方式和安装目录移除。凭据是否保留需单独记录，本切片不擅自删除 Windows Credential Manager 中的配对材料。

## Windows 本机验收进展（2026-10-01）

自定义 D 盘安装路径问题已解决：卸载项 DisplayName 实际带有 `version 0.3.0`，旧命令的精确名称匹配返回 0 项；InstallLocation 正确，无需修改安装器。稳定定位命令见 [HANDOFF.md](HANDOFF.md) 顶部。

已验证安装目录 `D:\Program Files\Personal Remote Desktop Agent`，安装 exe 版本为 `0.3.0.0` / `0.3.0+d7697772f98d`，其 SHA-256 与 publish exe 完全一致，Setup 哈希与校验文件一致。开始菜单、桌面快捷方式目标正确。实际启动安装版后窗口响应正常，默认未共享、控制许可未勾选，开始按钮可用、停止按钮禁用，无紧急停止快捷键不可用提示。

安装版双机回归结果：默认只读、Mac 断开后 Windows 持续等待、双方授权真实控制三项通过；物理 `Ctrl + Alt + Esc` 紧急停止失败。单实例、无 Mac 连接的只读共享下同样无反应，已排除多实例和远程输入状态。

已增加低级物理键盘 Hook 作为 `RegisterHotKey/WM_HOTKEY` 的兜底，并拒绝所有注入标志；WindowsInput 自动测试预期增至 7/7。但用户于 2026-10-02 报告 `518fb1b` 方案实机仍未解决，不得将其标记为有效候选版。具体 Windows 本机诊断与完成标准见 [HANDOFF.md](HANDOFF.md) 顶部。

仍待验收：Windows 实机定位并修复紧急停止、覆盖安装后配对保留、卸载残留、无 .NET 开发环境的干净 Windows 启动。前三条已通过的安装版核心路径无需在旧包重复测试，但新包覆盖后至少做一次连接与真实控制冒烟。
