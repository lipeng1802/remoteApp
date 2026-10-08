# 4C-4M / 4C-4N：macOS、Windows 原生信任 journal

两端代码与独立入口完成，**macOS原生验收通过，Windows桌面验收已由用户确认通过**。2026-10-08用户反馈本轮脚本返回下方完整VALIDATION PASSED；脚本要求4项必需Native全部PASS无SKIP。Windows SSH此前返回 `interactive_logon_required`、退出1仅为历史限制，不记其通过；当前结论不是安装成功或两端产品验收。

## 实现与边界

- `nativejournal`：独立OS存储，不访问现有device-key、identity-vault或配对记录。
- `cmd/trust-journal`：仅继承匿名stdin/stdout；无网络/公开IPC/命令行配置，固定OS线程持有锁。
- `trustprofile.DesktopJournal`：持有helper至Close，提供TrustJournal Load/CAS Commit；失败或超时关闭并等待helper，禁止文件回退/自动重建。
- 正常与测试命名空间分离；缺失状态只在明确create新安装owner中识别为ErrNotInstalled。重开pending/缺失/损坏均失败，不转create。只有测试命名空间提供精确scope故障注入与清理。

可信安装调用方必须固定helper来源、scope和安装锚。`NewDesktopJournal` 是PoC命名空间；`NewTestDesktopJournal` / `DeleteTestDesktopJournal`仅随机测试scope。安装锚正式供应、安装包、产品GUI和真实会话join接入仍未实现，不把这些API直接暴露给远程peer或把下载内容当执行路径。

## macOS

Keychain Generic Password，独立service由scope派生，禁止同步，新增条目使用WhenUnlockedThisDeviceOnly。调用设置 `LAContext.interactionNotAllowed=true`，锁定/拒绝授权时失败，不弹出后台密码输入框，不保存用户密码。[Apple SecItemUpdate](https://developer.apple.com/documentation/security/secitemupdate(_:_:))

独占锁放在OS账户资料指定的Application Support目录，**不使用HOME环境变量选择锁位置**。应用专属0700目录、当前UID/0600单链接锁文件、no-follow和flock生命周期锁，每次读写复核权限/链接；独立锁文件不是profile/版本下限存储。测试凭据精确删除后保留非敏感scope锁文件，避免删除锁造成并发双owner。

目前helper为本机开发ad-hoc签名，不是正式签名/公证。未来升级后的Keychain访问授权与正式签名策略需要安装切片验收。没有改旧RemoteController的device-key.v2访问逻辑，不能称旧密码弹窗问题已修复。

## Windows

使用当前用户Credential Manager的Generic Credential，LOCAL_MACHINE persist使同用户同电脑跨登录保留，不是所有用户共用。每块2048字节，低于Generic blob上限；索引和分块均在原生凭据库，无明文profile文件。[Microsoft CREDENTIALW](https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw)

同SID+scope的Global命名mutex覆盖同用户多个桌面session，DACL仅当前SID，持有线程固定。其他用户有自己的凭据集。网络/SSH登录可能没有凭据集，明确返回interactive_logon_required，不切换到文件或另一用户凭据。[Microsoft CredWriteW](https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credwritew)

这不是应用级隔离保险箱：当前用户其他程序可以访问该用户Generic Credential；不抵抗管理员/当前用户恶意程序主动删改全部状态，或整机快照回滚。两端保护承诺不包含这些威胁，未做实际第二OS用户入侵验收。

## 提交与恢复

记录最多8191字节，按SHA-256内容寻址分块。写入并读回核对全部新块后，才更新单一manifest；读回索引选中的整条记录并验digest，再精确删除旧块。只有该路径成功才能向Installation确认commit；join旧会话和发布新配置的顺序仍由安装核心约束。

写块/索引/读回/清理失败锁闭owner；不确定结果需明确重开检查，不自动重连。未完成块永远不能被旧manifest选中；孤立块不自动扫描删除。实际OS API成功并读回不是物理断电耐久的实机证明，没有注入断电/原生数据库EIO或硬崩溃。

journal IPC显式使用16KiB严格帧容纳base64，现有8KiB默认Reader不变；未知/重复JSON、超限和多值仍拒绝。

## 当前验证证据

macOS真实Keychain/helper流程4项Native全部PASS、无SKIP，最后新增“owner退出后仍拒create覆盖”复测也通过：

1. 明确首次初始化、跨helper独占、轮换前join、停止/join后新进程重开floor、降级与错误根拒绝。
2. 索引缺失/损坏、分块丢失/等长篡改拒绝Current/重开/重置。
3. 最大8191字节跨原生分块写入/新进程读回/安装核心验签。
4. pending安装不能重置，CAS失败锁闭，明确重开保留旧提交。

共享存储核心另有3项race测试，模拟块失败、manifest失败、写入后结果不确定及精确旧块清理失败；这些是内存故障模型，不冒充原生故障注入。Mac全量Go race和Mac/Windows全量vet通过。旧identity原生测试未提供对应exe会SKIP，不能混称其本轮通过。Windows x64 helper/测试构建通过，SSH真实调用明确失败于缺少交互登录；临时SSH二进制目录精确移除，创建凭据前Empty就已失败。

## Windows本轮桌面验收入口

全新四文件包已上传ZIP hash核对、解压及PowerShell语法检查，目录：

`C:\Users\jarvis\remoteapp-trust-journal-4mn-20261008`

正常登录Windows桌面，不需要管理员、Go/.NET SDK、Tailscale登录、旧安装包或保持READY。运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-trust-journal-4mn-20261008\run-trust-journal-validation.ps1"
```

脚本先核对两个exe的SHA-256，运行4项Native且拒绝SKIP/缺少任何必需PASS，恢复原环境变量；随机测试scope的原生凭据由测试精确清理，不操作既有产品凭据。成功必须输出：

`VALIDATION PASSED: Windows native trust journal + exclusive helper + persistent epoch floor. NOT installer trust provisioning or product acceptance.`

当前包helper hash `2d14dc64a28ec53ca7bd4c82f6425a8341c539dbeb714ee0552dc4820b2a9ca4`；测试hash `1468d299b418fa908c1ee8c4b57fb28421c429d1ab7cfe7bc4e446a7d4f02395`。构建变动需新包/新hash，不能复用旧包的PASS。

用户已确认本轮Windows桌面返回上述成功提示，未提供逐项耗时日志，不另记耗时或声称本机独立复测。切片1 macOS、切片2 Windows均已验收闭环；下一步可信安装锚供应/配置升级，再接产品会话owner。整体4未完成。
