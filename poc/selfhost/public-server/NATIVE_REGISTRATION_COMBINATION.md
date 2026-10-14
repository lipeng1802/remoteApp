# r26 原生注册组合验证

本轮是独立CGO原生TEST程序，不修改普通device-client或GUI、不接正式服务、不开放端口，也不要求Windows开机。它验证此前分离的OS signer、已安装profile owner、原生节点目录/结果journal与HTTPS注册流程，**不是实际Headscale入网**。

## 流程与边界

编译固定TEST release/profile/root/issuer与同目录journal/signer摘要，不接受运行参数或父端trust/credential/path。Root/CA私钥只在builder内存；已有TLSFixture私有leaf key仅用于隔离TEST本机HTTPS，此程序另将该leaf作为TEST issuer，必须release.Test=true、匹配TEST channel、profile issuer等于leaf公钥且链/域名合法。生产不得复用TLS与issuer私钥的这种fixture设计。原设备API NewMaterial默认issuer独立，行为未改。

每种情形使用新随机TEST身份，明确初始化原生profile与由稳定channel/vault/public派生的节点存储。真实native signer签名NetworkRequest与内部provision intent，authority fixture验证签名/nonce重放、Grant、内层意图；客户端实际HTTPS校验CA/127.0.0.1、校验issuer签名clock proof与grant期限。NativeEnrollmentValidationFactory只打开TEST存储，先提交attempted，再实际离线生成SDK node key。

credential从HTTPS直接在此受租约进程内消费，哈希比较/一次性CAS，不走命令行、文件、环境、父帧或日志。**Register回调返回fixture公钥/IP，不调用SDK Up**；registered只是本轮替身结果，不能当作真实网络观测/策略/持有证明。消费成功只输出active=false。普通NativeEnrollmentFactory继续固定真实SDK adapter，不能由运行帧切换到TEST替身。

两组各执行两次完整owner：正常组第一次成功/消费一次，重开后提交注册被原生registered门禁拒绝；unknown组模拟authority已处理注册但返回503，消费0次、原生attempted保留，重开后拒绝第二次请求。两组服务计数均为grant_clock=2、enroll=1。Unknown是计数副作用fixture，不是实际Headscale Mint或真实断网。

每轮实际等待Runner/signer join，再正常Close或明确RetireFault并确认profile native helper关闭；服务器Shutdown/join、随机TEST节点key与native record、随机OS身份、profile namespace清理均在成功门禁内。没有device-client继承stdin/EOF IPC；r25已做的无效Grant退出路径不能推广成此入口已接受有效enroll IPC。

## 本轮发现的清理问题

早期r2无阶段标记失败，不计PASS；r3/r4首次执行组合成功，但同包重跑在profile create被拒绝。新增诊断/TEST只读属性检查确认：删除回执后profile已无法Load，namespace仍非空。不能以“无法读回”推断所有条目已删，也不把单次成功当可重复通过。

Mac ClearTest现在在精确TEST service/账户作用域内逐项查找属性、验证已知manifest/data账户名、精确删除并确认耗尽；128项上限/重复账户/未知name均失败，不枚举或读取value data。通用CleanupTest另复核Empty，新增内存回归证明成功删除回执却残留记录仍要拒绝。修复的是本项目清理证据与实现，不宣称Apple API默认只删除一个条目；[Apple SecItemDelete说明](https://developer.apple.com/documentation/security/secitemdelete(_:))规定默认匹配所有条目，实际仍须复核。

使用新journal helper编译摘要的 `enrollment-registration-r26-mac-r6` 两次连续actual exit0：正常消费/unknown拒绝、profile无可读record且namespace Empty、所有实际join与清理均通过。早期失败包r2-r5不再作为新验证入口，其TEST残留公有配置材料保留供审计，未扫除未知Keychain或生产记录。空TEST目录/native锁文件依旧保留。Windows仅交叉build，未执行/上传；不能声称Windows新清理或组合已经验收。

## 新入口

新builder `cmd/enrollment-registration-validation-build` 参数：绝对go、module、darwin|windows、全新绝对dest、pinned native journal helper、pinned signer helper。Darwin程序CGO=1，Windows amd64/CGO=0；输出2个固定helper、validator、Windows脚本与SHA256.json。旧helper清理没有Empty确认，不能拿旧helper产物当修复版。

Windows开机后使用完整新 `enrollment-registration-r26-win-r4` 包，在普通已登录桌面PowerShell执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run-enrollment-registration-validation.ps1
```

标记须以 `VALIDATION PASSED: native profile/node leases` 开头、actual exit0、无FAIL/SKIP；不需要READY窗口，不需要Go或旧安装包。Profile约23小时有效，过期必须fresh rebuild。SHA manifest不是发行者签名/安装信任。Windowsr25存储/DACL与有效注册组合的桌面反馈均仍待。

下一项：Windows桌面组合，以及固定编译入口/可信首次存储初始化与实际SDK入网及退出。稳定macOS签名/受保护原生helper选择尚未完成；prod entry仍关闭provision。随后真实持有证明/bind/watch与数据面凭据供应不可省略，不走父端AuthKey/HMAC或换scope自动重试。整体4未完成。
