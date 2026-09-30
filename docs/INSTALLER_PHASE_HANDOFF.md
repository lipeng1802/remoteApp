# 双平台安装包阶段交接

日期：2026-10-01  
目标版本：`0.3.0`  
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

预期自动门禁：Release 0 错误、RemoteProtocol `63/63`、WindowsInput `6/6`。预期产物：

~~~text
artifacts\windows\publish\RemoteAgent.exe
artifacts\windows\PersonalRemoteDesktopAgent-0.3.0-win-x64-Setup.exe
artifacts\windows\PersonalRemoteDesktopAgent-0.3.0-win-x64-Setup.exe.sha256
~~~

Windows 安装器当前未做 Authenticode 签名；SmartScreen 提示属于预期的开发包限制，不应误记为正式分发就绪。

Windows `0.3.0` 尚未构建；需要拉取包含本切片的最新提交后执行上述单一脚本，并回传三组自动门禁、安装器大小、SHA-256 和包内文件版本。

## 后续验收

1. 分别核对两个安装包内版本号、源码修订和 SHA-256。
2. 在开发机执行覆盖安装、启动入口和卸载残留检查。
3. 在干净 Windows 11 x64 与干净 macOS 用户环境验证无需开发工具即可启动；Tailscale 仍单独安装和登录。
4. 用安装版重复默认只读、授权控制、Mac 断开后 Windows 持续共享、Windows 紧急停止四条核心回归。
5. 正式外发前补齐 Apple Developer ID/公证和 Windows Authenticode 签名；本切片不伪造或绕过签名信任。
