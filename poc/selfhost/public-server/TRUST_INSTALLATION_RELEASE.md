# 4C-4O：编译固定安装锚与离线配置升级

切片3的独立维护入口与构建路径已完成，Mac真实编译产物/Keychain验收通过，用户已确认Windows新版桌面入口返回本轮VALIDATION PASSED，切片3闭环。**不是正式签名的产品安装器，也未接活跃客户端会话或修改现有安装包。**

## 安装锚供应

`trustinstall.Specification` 包含固定发布根公钥、service、MinimumEpoch、channel、helper SHA-256和发布根签名的bundled profile。可信构建步骤将整份规格通过Go链接变量编译进 `cmd/trust-install`；空或损坏规格的普通未配置构建关闭失败。

运行时不读安装锚侧文件或环境变量，不接受root/service/floor/scope/helper路径参数。候选配置可以从文件读取，但只能改变发布根签名允许的profile，不能为自身选择新的信任根。旁边的 `build-public.json` 仅作构建追踪，不授予信任。

scope只从编译channel派生，跨root/service/floor/helper变更保持同一存储位置。换根或改service的程序因此会遇到原记录锚不匹配，而不是偷偷启用全新命名空间；根轮换仍需单独可信流程，不在本轮实现。

helper位于程序同目录固定名字 `trust-journal[.exe]`，每次启动前检查规范路径/无symlink/普通文件/大小和编译摘要。摘要核对不是发行者认证，也不能抵抗当前用户并发替换受其控制的目录；正式签名与受保护安装仍是发布要求。

## 维护模式

- `initialize`：预先以当前时间验证内置profile，再明确创建全新原生journal；已有状态不能覆盖。失效配置不会先创建pending原生状态。
- `inspect`：只打开已有原生状态并验证Current；缺失、损坏、低于当前编译floor都失败，绝不转initialize。
- `upgrade [candidate-profile.json]`：默认使用内置profile；可显式提供签名候选。验现有锚、签名、时间与epoch，持久提交并核对后才确认usable。降级、同epoch改写、错根/错service和过期候选拒绝，原提交不受影响。

升级程序是持有独占journal的**离线维护owner**，从不创建TLS/watch/tunnel，故它的quiescer只证明自身没有活会话。不能把这个空quiescer用于真正的客户端。后续切片4必须接实际取消/join，并在所有用户退出后释放journal；另一个owner仍持锁时维护工具无法接管。

新程序提高最低epoch后，旧记录即使仍签名有效也不能作为新程序Current。候选失败或提交结果不确定时不自动回退/重置：应明确检查已提交状态并提交合格的新profile。旧程序可以读取已提交新epoch，但不能以旧内置profile回退；这约束的是配置epoch，不宣称禁止回退整个可执行文件。

原生存储保护边界仍不包含当前用户恶意程序/管理员、删光整个命名空间或完整快照回滚。测试模式额外提供 `fault-test` 和 `cleanup-test`，只在编译Test=true且独立测试命名空间下可用；普通公开材料构建器不启用Test。

## 构建入口

`cmd/trust-install-build` 的参数顺序：明确Go程序、模块、新目标平台、新输出目录、可信公开anchor、公开profile、可信helper、channel。目标为darwin或Windows x64。拒绝已有输出目录，失败保留部分材料，不覆盖、重新生成根或自动安装。输入应是构建方核对过来源的本地公开材料，不是从任意服务端下载的自声明anchor。

本轮实际使用现有 `prerelease-v1/client-public` 两个公开文件与已签名Mac helper，生成 `artifacts/connection-poc/prerelease-trust-install-4o`，返回BUILD_READY。**未执行该真实预发布channel的initialize/upgrade，未读取/上传任何现有发布根或CA私钥，未改线上服务。** 测试新旧epoch签名使用隔离内存生成的测试根，绝不是重新签了真实预发布epoch2。

正式Installer/GUI嵌入、Developer ID/Authenticode、公证和升级授权真实性尚待。仅将公开root编进程序不能替代可信分发与代码签名，不能称现有产品安装包已免Tailscale登录。

## 验收证据

4项共享核心race测试：规格严格解析/稳定channel、提高floor/失败恢复/不转create、过期预检/换根拒绝、helper改动或symlink拒绝。首次发现测试临时目录含macOS系统symlink导致正常fixture误拒，规范化测试目录后通过；生产helper仍拒symlink路径，不放宽校验。

Mac独立真实工具包含固定同channel的v1/v2/other-root三个编译产物、发布签名负向候选和改动helper。完整验收退出0，五类门禁与精确测试凭据清理通过，无SKIP：

1. 首次原生安装、已有提交拒覆盖、新进程inspect。
2. v2编译floor2拒旧epoch1；失败候选保留旧提交，v1可明确检查原状态。
3. v2升级提交epoch2，新旧程序重开皆读epoch2；旧程序/旧文件降级、同epoch签名改写、错根/错service/过期候选拒绝，每次再inspect保持epoch2。
4. 换编译根不能启用新存储；伪造侧文件/环境变量无效，helper改动拒绝。
5. 真原生索引损坏后inspect/upgrade/重新initialize都拒绝；精确测试channel清理。

Mac全量Go race、Mac/Windows全量vet通过；普通全量测试未提供原生journal环境变量会SKIP旧原生项，不能当作本轮新实机证据，新流程由上述编译产物实际验收。Windows x64完整验证包构建通过；2026-10-08用户确认本轮桌面VALIDATION PASSED，不复用上一轮4M/4N的PASS，也不捏造逐项耗时或独立抓取日志。

## Windows新包

已ZIP SHA-256核对上传、解压与PS语法检查，位置：

`C:\Users\jarvis\remoteapp-trust-install-4o-20261008`

正常桌面PowerShell运行（无需SDK/管理员/Tailscale/旧安装包/READY）：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-trust-install-4o-20261008\run-trust-install-validation.ps1"
```

成功应为：

`VALIDATION PASSED: compiled anchor + native first install + upgrade/floor/recovery. NOT signed product installer or live-session acceptance.`

本机包 `artifacts/connection-poc/windows-trust-install-validation-4o.zip`。仅公开测试配置与程序；SHA256.json由新构建生成，验收器核对所有必需fixture摘要，PS先校验验收程序。测试profile约23小时有效，要求系统时间正常；若过期应重新生成新包，禁止延长旧配置或调整系统时钟绕过。

本轮Windows桌面结果已收口，下一步接切片4真实客户端会话owner；根轮换、正式安装包签名/发布、秘密分发、公网产品验收及整体4仍未完成。
