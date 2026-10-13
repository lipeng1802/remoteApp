# r25 原生注册状态与节点目录

本轮补齐r24可信内部组合的存储契约，不修改生产GUI、安装包或服务器。Windows当前关机，仅交叉构建，不宣称Windows原生验收。

## 所有权与路径

`enrollmentstore.Scope(release.Scope(), localVaultScope, expectedPublic)` 使用独立域名哈希形成稳定scope，不包含grant、epoch、端口或可执行路径。运行时只 `Open` 已有状态；`Initialize` 是单独的可信安装/TEST操作，已存在、缺失部分材料或损坏均不自动重建。同scope原生lease覆盖整个节点生命周期；Windows固定同一goroutine/OS线程，并额外拒绝同进程递归mutex打开。

状态record使用既有nativejournal OS credential backend：macOS Keychain/Windows Credential Manager，immutable chunks/manifest/CAS/readback，失败或不确定回执锁闭。节点SDK自己的私有key/state仍在受保护目录中，**不是全部放入钥匙串，也不宣称文件内容经过额外加密**。目录由OS账户资料指定：macOS账户Home/Library/Application Support，Windows KnownFolder LocalAppData；不由HOME/APPDATA、父帧、下载profile或peer路径选择。生产与随机TEST命名空间隔离。

macOS逐层no-follow打开、当前UID/0700目录、0600单链接regular文件、inode/path一致性复核；Windows显式当前SID owner、protected inheritable DACL仅当前SID full access，拒绝reparse point、额外ACE、其他owner与多硬链接文件，持有目录handle不共享delete。不对未知已有目录chmod/修复DACL，不授予Everyone/Admin/System ACE。node目录检查有2048项上限；这是正常其他账户隔离与协作式lease，不抵抗同用户恶意代码、管理员、整机回滚或已入侵OS。

## 状态与注册组合

`idle → attempted → registered`；没有回到idle、删除生产材料或重新授权的API。

`Begin` 先提交attempted并回读确认，再允许离线Prepare或HTTP enroll。`NativeEnrollmentFactory` 固定Open/Begin/保护目录/tsnet adapter，真实注册结果仅在原始节点公钥、规范overlay IP及ctx有效时提交registered。失败/未知保留attempted，成功保留registered，正常stop/授权过期也不清除。重开可只读检查，但不能再次Begin。它不是远端票据清理证明；后续恢复需authority reconciliation及明确授权的流程，不能靠换目录/identity/scope重试。

节点先实际CloseJoined，才释放目录及原生record lease；节点join失败保留lease，不强杀。只有随机TEST验证可在offline节点已join后精确删除本轮tailscaled.state和native TEST record；空目录及native锁文件有意保留，避免删锁导致双owner。测试权限/ACL负向项只修改自己创建的随机TEST材料后还原，不是生产修复能力。

`compiled device-client` 仍使用普通Runner，不启用provision；当前Darwin客户端构建CGO=0，不能直接启用本轮in-process Keychain接口。后续要选择原生CGO编译/稳定签名，或固定摘要的专用原生storage helper，并完成组合验收，不能偷偷改构建后宣称已安装入口通过。

## 已有证据与未有证据

- 4组通用状态机测试：重开拒绝、写失败/已提交但回执未知/回读不匹配、9类坏材料不修复、scope/非法完成；macOS真实权限/硬链接/symlink/路径替换负向PASS。Windows新增Everyone DACL拒绝测试仅编译。
- Mac fresh `enrollment-storage-r25-mac-r2` 两个实际执行程序PASS：真实NativeEnrollmentFactory离线prepare、两次真实子进程锁拒绝（且释放后同子程序读状态成功）、attempted/registered跨进程重开拒绝、Keychain/native lease、精确随机测试node key/record清理。registered完成采用public IP fixture，不是Headscale观测，没有Up。
- Mac fresh `installed-device-native-r25-mac` 完整实际compiled入口/OS signer/native profile journal/直接本机HTTPS基线及新负向PASS：无效issuer Grant产生网络终止结果后立即半关闭stdin，再验证failed+actual exit1、原生profile lease可重开且epoch不变。新负向不发网络HTTP，不覆盖有效grant_clock/state、HTTP故障、inactive回复或真实注册退出。
- 相关6包race及专用vet PASS。Windows `enrollment-storage-r25-win-r3` / `installed-device-native-r25-win` 都仅build-only、未上传/执行。未修改业务443/公网8444/正式Store或Headscale节点，整体4未完成。

## Windows开机后的新验证入口

不使用旧安装包，不要求安装Go。复制完整新bundle到普通已登录桌面PowerShell，分别执行：

```powershell
# enrollment-storage-r25-win-r3 目录：先全部测试（含DACL负向），再原生跨进程生命周期
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run-enrollment-storage-validation.ps1

# installed-device-native-r25-win 目录：最新客户端拒绝/EOF/实际退出组合
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run-installed-device-validation.ps1
```

第一个完整成功标记以 `VALIDATION PASSED: native enrollment journal` 开头；第二个包含 `terminal network grant rejection/EOF/actual exit`。必须actual exit0、无FAIL/SKIP。SSH登录可能缺少Credential Manager交互凭据集，不能作为桌面结果。第二包TEST profile有效期约23小时，过期需用builder生成fresh包；不能改时钟或使用生产profile绕过。

构建：`cmd/enrollment-storage-validation-build` 参数为绝对go路径、module路径、darwin|windows、全新绝对输出目录，生成程序/单测程序/Windows脚本与SHA256.json。独立存储包没有在线profile过期。编译及hash manifest只证明构建与完整性，不是可信产品签名/安装器信任。

下一项：先Windows真实DACL/Credential Manager与退出验证，再收口固定编译原生存储初始化/打开组合及有效签名HTTPS enroll+实际node join；随后接实时持有证明/bind/持续watch。服务verifier身份与凭据仍须在内部从受信任服务获得，不能回退父端供应secret/可替换trust配置或直接开放数据面。
