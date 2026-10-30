# Personal Remote Desktop MVP

r78 增加不含秘密的 TEST 活动阶段诊断，137 项 Python 回归通过。首次真实并行 provision 后未观察到双方持续 PONG；已明确撤销，实际验证两平台 registered 在线退休门禁并回收安装根。正常活动零退出及故障收尾协议仍待完成，不能标为整体验收通过。

r77 修复 Windows PowerShell 中文公开诊断的 UTF-8 管理通道；已在 Interactive 桌面完成旧中断 TEST 安装的历史 idle 退休及受限文件收尾。它是异常回收证据，不代表活动会话或正常退出通过；自动续办已恢复。

r76 在签发 TEST 短包前先核验 Windows 不可变输入，管理读取失败不再降级上传完整大包；仅该 TEST runner 去掉 Go 调试信息并降低压缩成本，原签名/库存/信任门禁保持。传输优化不代表双机 PONG 或产品验收通过。

r75 补齐显式 fresh 双机包、驻留 owner、正常活动收尾及独立在线退休的管理入口：[活动 runner](poc/selfhost/public-server/SUPERVISED_ACTIVITY.md)。阶段未知时保留 signer 和 HTTPS 通路；Windows 大包改用固实压缩和逐文件哈希核验的不可变 helper 字节复用，不复用旧身份或安装根。此入口的构建/管理回归不能替代真实持续 PONG、双机正常退出或注册节点退休验收。

r74 增加驻留双机活动协调的持久阶段门禁：业务开始前记录未知状态，只有严格的 PONG/正常 join/撤销证据齐备才允许停止父 owner；停止回执未知时保留已确认角色和后续显式收尾入口，HTTPS通路留给独立退休工具。76项Python回归通过；当前仍未接通fresh双机启动/退休管理适配器，不代表实机PONG验收。

r73 增加 TEST `quiesce_session`：实际正常停止并 join 产品后，保留独立撤销 signer，供明确撤销和随后最终退出。并行 PONG 协调可显式选择此正常收尾模式；活撤销故障分支保持独立。组件测试通过不代表真实双机活动验收，当前仍待接入 fresh 驻留包进行实测。

r72 增加 registered TEST 节点的在线退休门禁：只有原生绑定选定的节点，其授权已明确撤销且服务端确认清理完成，才能取得短期 issuer 签名证明并进入本地清理。预发布 authority/frontend 已兼容部署；协议回归通过，真实 registered 节点和双机活动验收仍待完成。工具不接入生产卸载器。

r71 已部署预发布 readiness 协议，Store、长期 verifier、零规则与业务 Nginx 基线保持。macOS 新原生独立业务 socket、驻留管理 owner、客户端断开后存活、实际零退出及精确 TEST 回收已实测通过，71 项 Python 回归通过。r67 双 OS 身份的授权/拒绝/撤销已通过，r68/r69 独立撤销与并行 provision/PONG 协调已实现；当前仍待接真实双机 ACTIVE/PONG、活动维护与删除整链，未完成 GUI 或签名发行验收。见[授权协调边界](poc/selfhost/public-server/SUPERVISED_CONSENT.md)和[产品退出交接边界](poc/selfhost/public-server/UNINSTALL_MAINTENANCE.md)。

r57 增加独立本地进程的 OS 设备密钥双向认证及固定编译客户端：绑定双方 nonce、服务实例、完整编译 release 和原生维护事务，认证失败不停止会话，确认中断保留租约与 pending。当前是 TEST 授权策略，尚未验证发行方签名、正式发现／同意流程或跨机产品卸载。见[独立维护认证边界](poc/selfhost/public-server/UNINSTALL_MAINTENANCE.md)。

r56 增加删除句柄绑定与可信进程内的活动 owner 收尾联动：真实本地 TLS 流全部停止、runner 实际 join、原生租约重新取得且核对同一事务之后，才进入 TEST 删除。Windows 使用核验后的文件句柄删除，macOS 使用固定父目录 fd；实际跨机产品会话及正式发行仍待完成。见[维护与删除边界](poc/selfhost/public-server/UNINSTALL_MAINTENANCE.md)。

r55 新增 TEST 程序文件删除与中断恢复：签名文件集、目录／文件对象身份形成固定计划，摘要先绑定原生维护事务，真实删除中进程退出后仅可继续原计划；完成后仍保留设备身份、激活记录和 epoch floor。macOS／Windows 原生验证及 macOS 固定编译入口通过。目录空壳和恢复计划保留，正式卸载器及发行签名仍待完成。见[删除事务边界](poc/selfhost/public-server/UNINSTALL_MAINTENANCE.md)。

r54 产品维护现先向原生journal提交持久pending，再cancel/join；结束握手不会清除意图，进程崩溃后新owner、重装和升级继续被拒绝。macOS/Windows维护10项组合通过，含真实未join时进程exit77与模拟丢回执；最终双端idle产品/身份/安装升级组合均PASS。信任、设备身份与epoch保留，生产恢复仍待实现。见[持久维护协议](poc/selfhost/public-server/UNINSTALL_MAINTENANCE.md)。

r53 实际自主产品已接专用继承管道的维护握手：join后才报告ready，finish前保持原生租约，所有错误路径先join再释放安装binding。macOS和Windows桌面完整原生组合PASS，覆盖身份参数覆盖拒绝、真实产品退出及身份/epoch保持。当前是可信父进程的idle产品控制验收；独立卸载器认证、持久事务、活会话维护和真实文件删除仍待。见[维护协议](poc/selfhost/public-server/UNINSTALL_MAINTENANCE.md)。

r52 新增卸载前owner维护屏障：封闭新会话、实际join后才执行维护action，操作期间保留原生独占租约，默认保留信任和防回退记录。macOS/Windows桌面原生四项测试均PASS，相关race回归通过。当前只验证TEST程序marker删除，产品卸载IPC、持久事务及真实文件删除仍待接入。见[维护屏障](poc/selfhost/public-server/UNINSTALL_MAINTENANCE.md)。

r51 独立实验安装前端已实际验证：Windows Inno Setup与macOS只读DMG都能调用固定adapter完成epoch1安装、epoch2升级；旧包降级被拒绝且active保持2，测试资源清理完成。源码见[实验打包入口](packaging/experimental/README.md)。正式签名、提权根、GUI安装及带会话join屏障的卸载仍待完成。

r50 新增供平台安装器共用的编译固定安装/升级入口，预检三个子程序摘要，串联release准备/激活、bootstrap切换与重开确认。Windows桌面Session 6完整原生组合exit0，macOS最终fresh组合PASS，参数覆盖拒绝和TEST清理通过。当前仍未接DMG/Inno前端或正式发布签名；下一步接平台安装与卸载保留策略。见[安装器契约](poc/selfhost/public-server/INSTALLER_ADAPTER.md)。

r49 Windows完整validator已在普通登录桌面Session 4完成fresh r16验收，返回`VALIDATION PASSED`且exit 0。实测收口了Windows目录持久化回执不确定、bootstrap指针只读属性阻止`MoveFileEx(REPLACE_EXISTING)`、分阶段fresh-owner重开及TEST清理最终状态证明；没有延时或把任意失败当成成功。macOS x86_64最终fresh原生组合同样PASS。覆盖两代签名release、原生trust/identity、可中断重开、bootstrap set原子升级、新旧稳定入口跟随active release及TEST状态清理。尚未接DMG/Inno、提权、Developer ID/Authenticode或完整产品会话。

r49 新增受保护稳定bootstrap入口：launcher/`trust-journal`作为独立Ed25519签名、内容寻址且不可变的set导入，`current.json`只在旧set重验签后原子单调切换；稳定dispatcher和新旧launcher每次重开当前重签名bootstrap/release，不回到自身旧版本。macOS原生组合已通过v1/v2重复安装、升级、拒绝回滚与精确清理；合并后 `fa40fc1` 的 Windows r3 因打包时父目录缺失被正确拒绝，无残留，r4/r5/r5b均作为诊断包退出。尚未接DMG/Inno、提权、Developer ID/Authenticode或完整产品会话。

r48 将安装根回执、签名payload导入、原生trust安装/升级与release activation串成编译固定的无参数transaction owner。它仅接受唯一root/staging/manifest/epoch/operation及摘要固定的provisioner/manager；重启时只允许继续exact staging或content-addressed final，相同版本已激活视为幂等成功，混合/篡改/回退状态锁闭。macOS原生组合已通过v1 install两次、v2 upgrade两次、精确回收与原身份升级回归；Windows x64仅交叉build/vet。固定bootstrap launcher/helper安装、真实提权、代码签名及DMG/Inno尚未接入。

r47 新增固定目标、无运行时路径参数的install-root provisioner：生产macOS/Unix只能以root设置`0755`可读执行根，Windows设置受保护DACL（SYSTEM/Administrators可写，Users只读执行）；TEST根保持当前用户私有。回执不含明文路径，绑定路径摘要、卷和两个目录对象ID，必须立即重开核验；TEST清理仅接受当前回执绑定的空布局。macOS编译固定入口原生验证已通过，Windows x64仅交叉构建；真实提权、代码签名和DMG/Inno尚未验收。

r46 完成installer staging交接边界：外部签名payload必须先全量验签，再以独占创建、单硬链接、同卷方式复制到安装根内隐藏staging，并fsync文件/目录后重验。manager/launcher每次检查安装根、`releases`及活动版本的owner/权限、链接/重解析点、同卷与单链接属性；生产macOS根只接受root owner，Windows只接受SYSTEM/Administrators owner且拒绝Everyone/Authenticated Users/Users的写权。macOS用户所有TEST保护根原生组合通过，Windows仅交叉构建；正式特权根创建、DMG/Inno和代码签名仍未接入。

r45 已把签名不可变release核心接到两个固定入口：管理器只接受`install|upgrade|status`操作，release root/channel/floor/helper/隐藏候选名/期望manifest摘要均在构建时固定；无参启动器从自身目录打开原生CAS activation，每次重验签全部文件后才启动当前产品。macOS隔离原生组合验收已通过v1首次激活、v2升级、拒绝回滚、旧/新启动器同指当前产品拒绝路径、身份跨升级与TEST清理。仍未改DMG/Inno，未做受保护安装根、正式代码签名、完整产品会话或Windows实机验收。详见[release布局](poc/selfhost/public-server/INSTALLED_RELEASE_LAYOUT.md)。

r44 开始签名安装布局核心：新`installrelease`对自主六件运行时定义严格、规范、Ed25519签名的release manifest，候选完整验证后进入epoch+内容摘要命名的不可变目录，当前版本只通过CAS activation journal切换；失败/不确定回执锁闭，降级拒绝，旧版本保留，每次打开active都重验签和全文件哈希。macOS真实Keychain TEST journal已完成epoch1→epoch2激活/重开/清理；Windows仅交叉构建。尚未接入现有DMG/Inno，没有正式签名证书或安装目录ACL验收，不宣称签名安装包已完成。详见[release布局](poc/selfhost/public-server/INSTALLED_RELEASE_LAYOUT.md)。

r43 收口自主安装最小诊断协议：`status`不再输出稳定设备公钥，只返回`diagnostic_version=1`、状态/stage、必要的enrollment phase与当前原生journal已验证的签名`trust_epoch`。macOS fresh TEST验收证明epoch1升到epoch2后，无论旧/新安装入口都只能报告epoch2，不能通过旧程序产生降级诊断；pending仍拒绝重装。显式`inspect`仍保留公钥供受控身份核对。Windows关机，实机验收仍待补。

r42 继续不依赖Windows的自主安装收口：`autonomous-enrollment-install status`现在通过摘要固定的原生helper只读区分`not_installed`、`usable`与六个`maintenance_required`阶段，不返回保护记录、不创建/修复/重置身份。TEST-only中断夹具证明pending安装不能重试生成第二身份，只能对精确pending测试命名空间显式清理。macOS随机TEST channel已完成原生Keychain的absent→usable→签名升级保持同身份→pending锁闭→精确清理验收；Windows关机，实机同类验收仍待补。

r38 继续切片4B：新自主`ProductSession`已把固定摘要的真实产品子进程接入`SessionRuntime`。Target发布并复核证书持有后，随机HMAC与证书指纹只经已验证overlay上的一次性双端OS签名握手交付；目标签名绑定secret hash，Controller独立核验issuer短期TLS描述符同指纹后才把HMAC写入产品匿名管道。父帧提供HMAC/回调被拒；watch失效仍先锁gate，再取消并实际join tunnel与产品。相关race/交叉vet通过；正式入口仍未启用，Windows关机，真实双机产品流待验。

r37 继续切片4B：新自主SessionWork改为只接收不透明`SessionRuntime`，不再直接接收可自行组合的peer值。目标角色只能在自主tsnet节点固定监听`:47476`，控制角色只能拨向issuer签名binding中的目标`:47476`；监听/拨号均重核本机node/IP与远端WhoIs，持续watch逐次更新数据gate并在退出前锁闭。listener随session context关闭，owner仍按session→watch→proof→SDK→signer顺序实际join。六包race及Windows/Linux交叉vet通过；Windows关机，真实双机overlay、产品TLS/HMAC/payload仍待。

r36 继续切片4B：新增issuer签名、3秒有效的`session_identity`，只在已批准grant和动态策略ACTIVE时，向参与者绑定当前binding、fresh challenge及Store注册的两端公钥。新自主SessionWork启动前在owner内部验签并确认本机角色，父进程不能提供或替换peer key/binding；撤销、外来设备、混合reply及篡改全部拒绝。尚未接overlay listener/dialer、TLS product/HMAC/payload；Windows验收按用户要求继续记录为待办。

r35 开始切片4A的[会话生命周期桥](poc/selfhost/public-server/CONTINUOUS_AUTHORIZATION.md)：新的显式opt-in runner只在同一自主enrollment watch首次ACTIVE后启动编译期可信session work；watch fault/到期/stop/cancel先取消并实际join session，再依次释放watch、proof、SDK、signer。提前退出或取消后真错误均锁闭owner。尚未接TLS descriptor/HMAC/overlay/payload，不是产品会话验收。

r34 继续切片3B的本机跨层闭环：真实`invite.Store`双设备批准与签名撤销、真实`ProvisionBackend` enroll/bind/动态状态与两个持续watch串联；撤销后必须先同步投影零规则，再观察两端watch fault及owner资源join。策略driver/节点仍为本机fixture，不是真实Headscale内核拒绝或双机验收；Windows 1/2仍待开机执行，4的payload还未进入新自主入口。

r33 开始切片3的[持续授权桥接](poc/selfhost/public-server/CONTINUOUS_AUTHORIZATION.md)：新增显式`WatchedEnrollmentRunner`，自主enroll/proof/bind后先在12秒内等待双方策略ACTIVE，首次ACTIVE后固定binding并持续刷新签名state；inactive、撤销/到期、binding/身份变化或请求失败永久fault，退出顺序为watch→proof→SDK→signer实际join。5组新回归及相关6包race/双平台vet通过。尚未接正式入口或双实际节点，未作真实零规则/旧连接拒绝，切片3未完成；Windows 1/2验收仍待，4未启用。

r32 继续收口1/2的不依赖Windows部分：原生联合验收在helper实际join/删除回执后，再以native TEST属性只读确认profile命名空间为空；Mac同scope连续两轮真实Keychain创建/重开/清理通过。预发布维护operator把READY/STOPPED绑定当前boot、PID与ExecStart单调时间，新增显式finish-resume与verify-restored、中途失败/基线变化/candidate仍live门禁；18项operator及全部22项Python回归、相关9包race和双平台vet通过。服务器仅只读确认现运行服务具备所需systemd属性及当前READY回执，未上传/重启/维护；Windows仍关机待r31预检和真实联合验收，3/4未开始。

r31 [Windows 1/2 新版验收准备](poc/selfhost/public-server/WINDOWS_ENROLLMENT_PREFLIGHT.md)：已构建独立单窗口预检包，合并最新原生存储/权限、OS签名本机HTTPS与真实SDK取消/join检查；另补齐联合入口的Windows helper/构建支持及严格包清单。Windows关机，只有构建证据，原生执行与真实服务器注册/持有证明绑定仍待；未联系Windows/服务器，3/4保持暂停。

r30 切片4的1/2已完成Mac＋真实服务器的隔离联合验收：原生OS签名HTTPS注册/批准/校时/enroll→真实SDK→issuer verifier permit→实时nodeproof/backend bind→实际join/本机清理及服务端工单回收，actual exit0。客户端不通过SSH接收凭据；私有HTTPS暂经SSH回环转发，claimant为memory signer测试角色，无第二SDK/数据规则。维护后原服务、长期1资源、零ACL、正式Store、Nginx均恢复；修复清理状态转换与READY恢复顺序。Windows/正式客户端入口/公网/GUI安装仍待，3/4未开展。

r29 优先推进切片4的1/2：新增authority签名的短期verifier信息、nonce/授权/设备/工单/节点绑定，可信内部注册→proof listener就绪→bind路径及退出join。下列r29组件证据已由r30隔离联合验收补齐Mac＋server链路，仍不是Windows或产品安装完成。

r28 [原生真实Headscale注册/回收](poc/selfhost/public-server/NATIVE_HEADSCALE_REGISTRATION.md)：Mac真实Mint→SDK Up→Observe→同步退出→精确回收→原生TEST清理连续两轮通过；原长期资源、零ACL、正式Store及Nginx保持原状。修复管理传输期限余量与成功登录空缓存目录清理；失败工单已显式收口。仅管理员SSH验收，不是最终客户端凭据供应、签名grant/enroll、实时proof/bind/watch或GUI通过；Windows未执行，整体4未完成。

r27 [原生真实SDK退出验证](poc/selfhost/public-server/NATIVE_SDK_LIFECYCLE.md)：真实tsnet Up访问固定本机失败服务、取消后同步退出、原生attempted保留/拒绝再次注册及精确清理；Mac重复通过，Windows新入口仅构建。补齐SDK四个已知文件的TEST清理、未知文件整组拒绝，以及独立helper进程级禁止日志上传。这不是成功Headscale入网，不与r26替身测试拼接为端到端通过。

r26 [原生注册组合验证](poc/selfhost/public-server/NATIVE_REGISTRATION_COMBINATION.md)：真实OS签名器/已安装profile租约/节点原生存储、签名HTTPS校时与enroll、内部TEST凭据消费及重开不重试串联；Mac同包重复通过，注册响应未知仍只触发一次请求。发现并修复TEST清理回执后仍残留记录的问题，改逐项删除/复核空。Windows新包仅构建；节点Up是离线替身，不是Headscale、正式客户端IPC或GUI验收。

r25 [原生注册状态与节点目录](poc/selfhost/public-server/ENROLLMENT_STORAGE.md)：macOS Keychain/私有目录与Windows Credential Manager/受保护DACL，稳定身份scope、原生独占lease、注册前提交attempted，重启后未知/已注册均拒绝重试。Mac实际原生存储/多进程及新客户端网络拒绝→EOF→真实退出组合通过，相关race/vet通过；Windows新包build-only，待桌面验证。正式enroll入口仍未启用，真实Up/proof/bind/watch与整体4未完成。

r24 [内部受控注册与凭据消费](poc/selfhost/public-server/INSTALLED_DEVICE_CLIENT.md)：新增可信组合 EnrollmentRunner，已验证grant_clock后内部准备公钥/签署intent/直接HTTPS enroll/内存交给节点Register；仅固定passed/active=false输出，节点与签名器实际join后结束。7组新增测试及本机HTTPS正常/故障/到期组合通过。**正式编译入口未启用**：跨重启结果记录、节点原生ACL/租约、真实Headscale Up/持有证明/bind仍待，不是新的GUI/安装包验收。Windows无需开机，服务器未改。

r23 [已安装客户端网络授权门禁](poc/selfhost/public-server/INSTALLED_DEVICE_CLIENT.md)：直接OS签名器接口/已安装profile请求grant_clock与state，客户端自行挑战、核验issuer/自身主体/期限/3秒状态；inactive或错误结束owner并join，不回传注册凭据/HMAC。真实本机HTTPS+内存signer联合测试、相关race/vet、双平台fresh TEST构建通过，原生网络组合/enroll/bind/watch仍待；本轮未改服务器/安装包，Windows无需开机。

r22 经用户明确批准维护窗口，预发布长期网络后台已部署并完成回环故障闭环：Headscale持久database/显式零ACL、长期verifier、sole authority及独立UID前端已接通；provider故障非零退出/两路503、恢复不自动重启、显式同身份恢复通过。8444仍127.0.0.1，业务Nginx/443与Store摘要未变，未enable自启。备份/检查点与边界见 [部署交接](docs/HANDOFF.md)。Windows本轮关机且无需操作，公网/客户端自主凭据与GUI仍待。

r21 [候选systemd网络服务验收](poc/selfhost/public-server/SYSTEMD_NETWORK_ACCEPTANCE.md) 完成：目标CentOS随机runtime unit/真实verifier与Headscale、跨UID拒绝、正常join、provider故障非零退出、无自动恢复、显式同身份重启及精确清理通过。修复组权限、依赖停止语义、CLI/overlay所需能力及systemd停止回执兼容。正式后台仍device-only，未开放公网；Windows本轮关机且无需参与。

r20 [长期后端部署准备](poc/selfhost/public-server/NETWORK_DEPLOYMENT_PREPARATION.md)：新增只读authority预检、严格权限/锁/恢复材料检查及候选systemd unit与回退门禁。不会启用网络、创建verifier或修改策略；候选unit与长期后端实机组合仍待验证。

r19 [直接HTTPS网络入口](poc/selfhost/public-server/DIRECT_HTTPS_INGRESS.md) 代码与本机验证完成：独立运维public模式，签名域名profile/独立UID broker必需，两路共享4工作槽，默认network仍404、authority不可用503。相关race/vet及Python78项通过，Linux前端build-only；服务器只读确认8444仍回环、正式network backend未配置、基线不变。尚未部署或开放公网，本轮Windows无操作。

r18 [新客户端原生联合验收](poc/selfhost/public-server/INSTALLED_DEVICE_NATIVE_VALIDATION.md) 双平台已闭环：Mac实际compiled客户端/native journal/OS签名直连本机HTTPS、稳定重启、实际退出及精确清理全PASS/exit0；用户确认Windows普通桌面入口返回完整预期VALIDATION PASSED。下一项长期服务自主HTTPS接入与受保护凭据供应；不包含公网/Headscale/GUI/安装，整体4未完成。

r17 新增[已安装信任设备客户端](poc/selfhost/public-server/INSTALLED_DEVICE_CLIENT.md)：从固定编译锚和原生journal当前配置创建直连HTTPS设备API，不接受父进程地址/CA/issuer/key或helper路径；仅OS身份重开、设备授权API、实际HTTP/signer join。相关Go race/vet、本机隔离HTTPS与Python78项通过，Mac/Windows TEST包build-only通过。新入口原生组合、公网服务、自主enroll/产品凭据、GUI/安装仍待；没有改线上服务或要求新双窗口。

r16 真实双机活TLS撤销后重新授权组合验收已闭环：协调器actual exit0、两桌面STOPPED及收尾核验exit0。旧会话自主fault join/原生退役、新邀请审批/不同grant、新node scope/idle owner/Mac进程/证书/HMAC、新TLS持续pongs、正常join/签名撤销通过。本轮双端进程/bootstrap/两份grant测试证书scope无残留，控制资源归零/正式基线不变；不包含自然过期、GUI/公网无SSH或签名安装包，整体4未完成。

r16 新增活TLS签名撤销/自主fault join后新邀请审批、新grant/node scope/idle owner/process/certificate/HMAC及新TLS组合入口。Python78项、相关8包Go race/vet通过，真实双机组合验收仍待新两桌面入口；不放宽同grant pin门禁，不包含自然过期、GUI/公网或签名安装包。准备与操作见 [HANDOFF](docs/HANDOFF.md)。

r15 真实双机重新授权已闭环，协调器实际exit0、两桌面STOPPED及收尾核验exit0：旧grant在线拒绝、旧请求/邀请码不可复用，旧API实际join后新邀请/明确审批产生不同grant；新grant真实Headscale绑定及产品TLS pongs跨租约、正常join/签名撤销均PASS。本轮双端子进程/bootstrap/测试证书无残留，控制资源归零、正式基线不变。不是旧活TLS撤销后重连或自然过期实机验收，整体4未完成。

r15 等待窗口目录名已修正为原规则要求的16位hex路径，实际Windows参数规则/非重定向/无run/脚本摘要与解析检查PASS，Python74项通过。creator保持不变，重新执行HANDOFF中窗口2更新命令即可；真实双机重新授权仍待验收。

r15 重新授权切片已开发并准备 fresh Windows 双窗口入口：旧批准先签名撤销，旧grant在线拒绝/旧请求不可重批/旧码不可复用；旧API实际join后新邀请与明确审批生成不同grant，再接compiled Headscale/TLS。Python73项、invite/API race及vet通过，撤销/过期与持久化重开的Go回归通过；新公开包摘要及入口解析已核验，尚未真实双机验收。不代表旧活TLS撤销后重连、实机自然过期或整体4完成。操作见 [HANDOFF](docs/HANDOFF.md)。

r14 真实双机 TLS 升级验收已闭环，协调器实际 exit0、用户确认两桌面 STOPPED，收尾只读核验 exit0：本轮双端子进程/桌面 bootstrap/测试证书无残留，控制资源归零、原服务/策略/Store/Nginx 基线不变。无效候选保持旧 TLS payload、两端原生 epoch2 与拒降级、显式新进程/会话的 TLS pongs 跨租约、正常 join及签名撤销均 PASS。不包含真实 OS 写失败、GUI或签名安装包，整体4未完成。

r14 Windows 五项原生升级/lifecycle 验收已由用户反馈全部 PASS，修复后的入口返回预期 VALIDATION PASSED。覆盖原生租约、产品实际 join、非零退出拒升级、无效候选保留旧 runner、不确定提交回执锁闭及重开 epoch2 floor。真实双机 TLS 升级仍待验收；不代表真实 OS 写失败、GUI或签名安装包通过。

当前 r14 升级切片：已补齐无效候选拒绝但保持旧会话、只读 durable epoch、显式 `--compiled-upgrade` 双机入口及原生不确定提交回执锁闭测试。相关 race/vet、Python 72 项和 macOS、Windows 各五项原生/生命周期验收通过；下一步启动新的两桌面入口进行真实双机 TLS 升级验收。未增加生产远程故障注入，不把模拟回执错误视为真实 OS 写失败。详见 [HANDOFF](docs/HANDOFF.md)。

r13 真实双机显式重连验收已闭环（协调器实际 exit0，两桌面 STOPPED）：原生桌面证书探测、Headscale 授权网络、初始 TLS、authority outage 下自主 fault join/退役、同 verifier 恢复后旧 owner 保持关闭、原证书复用/新 HMAC/新 Mac 进程、重连 pongs 跨租约、正常 join、签名撤销和节点回收均通过。本轮子进程/桌面 bootstrap/原生测试证书已无残留，原策略、正式服务、Store/Nginx 基线恢复；禁止静默换 pin 门禁不变。不包含新邀请审批、升级、GUI或签名安装包，整体切片4未完成。详见 [HANDOFF](docs/HANDOFF.md)。

显式重连B1：已接入Windows桌面backend的同用户本地 `reopen_fault`，成功只返回idle新owner；生命周期操作串行，最终关闭当前替换owner，失败保留旧终止状态，不自动启动或复活旧会话。相关race/vet与Windows交叉编译通过；本轮未进行原生桌面或双机验收。下一步Mac新进程/授权重交接及compiled重连协调，详见 [HANDOFF](docs/HANDOFF.md)。

显式重连切片A基础验收通过：只有故障成功退役后才能从同一原生保护存储打开独立idle owner，旧owner仍锁闭；拒过期/损坏/未join/退役失败，不自动启动或继承旧授权。相关Go race及macOS原生租约/epoch floor测试通过，用户已反馈Windows普通桌面验证通过。先前SSH原生初始化失败记录仍保留；尚未接入双机重连协调，不代表整体重连验收完成。下一子切片接入新owner/进程与授权交接，见 [HANDOFF](docs/HANDOFF.md)。

r10真实双机authority outage验收通过/实际exit0：helper交接与产品启动全部阶段完成，真实TLS/pongs跨初始lease，隔离authority不可用且旧ACL残留时两端自主fault join/native retirement，同verifier恢复后旧owner保持关闭；在线撤销、资源归零与正式基线恢复全部通过。用户已确认两个桌面窗口STOPPED；不包含新产品重连、升级、GUI或签名安装包，整体4未完成。下一独立切片优先fresh owner显式重连。r9历史失败根因不能据本轮成功倒推。详见 [HANDOFF](docs/HANDOFF.md)。

r10全新阶段诊断双窗口入口构建、摘要与脚本校验及58项Python回归通过，已完成上述实测并消费stage。r10/r9端点均不可复用。历史准备步骤见 [HANDOFF](docs/HANDOFF.md)。

新增产品交接角色/阶段诊断，58项Python回归通过。纠正r9定位：统一失败帧行号不能证明旧helper退出失败，也可能是后续产品启动失败；现已补两侧helper实际join、初始化和产品启动的独立固定阶段标记，尚待fresh双机实测，未确认根因或修复。详见 [HANDOFF](docs/HANDOFF.md)。

r9实机路由就绪、单次正向固定payload、反向拒绝均通过；随后产品交接链路失败，没有产品持续TLS/pongs或服务故障注入通过证据。已独立完成隔离进程/控制资源清理及原策略、正式服务/Store/Nginx恢复，不计原协调完整验收。用户已确认两个桌面窗口STOPPED，下一项定位具体失败阶段，整体4未完成。

r9路由诊断复测入口曾构建并上传，已完成上述失败实测并消费，不可再启动或复用其bootstrap/stage。历史准备证据与具体边界见 [HANDOFF](docs/HANDOFF.md)。

新增 overlay 客户端就绪检查：精确匹配预期节点/IP、本地netmap与/32路由，在原12秒投影预算内等待；保留单次payload和禁止系统网络回退。Go相关测试/race、54项Python及Windows helper编译通过，**尚未fresh双机实测，未证明r8根因或故障验收通过**。下一步准备全新双窗口入口，详见 [HANDOFF](docs/HANDOFF.md)。

r8真实尝试在服务故障注入前失败：OS授权/节点持有证明/签名binding通过，但正向探测为 `overlay_route_missing / route_denied`，产品TLS和authority暂停尚未执行。已独立恢复原Store/同verifier完成客户端清理、精确退役、控制资源归零和原策略/正式基线恢复；不算原自动验收成功。用户已确认两个桌面窗口STOPPED，下一项排查overlay路由缺失，整体4未完成。见 [最新交接](docs/HANDOFF.md)。

authority outage r8 验证入口已准备并上传：当前9项helper重建、Windows公开包全部摘要、fresh stage等待脚本摘要和三个PowerShell入口语法均通过。下一步用户在正常Windows桌面启动全新creator `windows-desktop-authority-fault-20261010-r8` 与新stage双窗口至READY/WATCH_READY，再运行真实故障验收；**尚未启动会话或执行服务故障实验**。命令与目录见 [HANDOFF](docs/HANDOFF.md)。

2026-10-10 当前状态：r7 compiled/native 双机真实产品 TLS、活会话签名撤销、实际故障 join/原生退役和两个桌面 STOPPED 已验收。新增 `split-consent.py --compiled-fault` 隔离服务故障验证入口；50 项 Python 与 Linux 10 项协议回归通过，**新入口尚未真实双机验收**。只暂停本轮隔离 authority 子进程，不暂停正式服务；验证旧 ACL 残留期间两端自主关闭、同 verifier 服务显式恢复后旧会话仍锁闭，不宣称新产品会话重连。整体切片 4 未完成。后续步骤见 [交接](docs/HANDOFF.md)，下方旧记录按历史保留。

4W 旧Windows signer artifact故障已完成原生离线修正验收：restart-r2实际8类签名域验证、节点intent准备及准备前后保留连接签名全部PASS，helper真实退出，未注册网络节点。当前源码native helpers重建已加入构建流程，Go与26项Python回归通过。r2身份窗口仍READY，旧r1/4T状态保留，清理审计不与新测试混同。**下一步重跑fresh双机网络；节点绑定/产品TLS联合验收仍未通过。** 详见 [交接](docs/HANDOFF.md)。

4C-4W：新增 `split-consent.py --compiled` 联合协调入口，串接匹配真实服务的 fresh test pair、Windows 桌面 borrower、OS 签名授权与 bound-node 交接、实际产品计数持续增长、正常 join 后签名撤销。私有管道有界且不强杀冒充完成。23项 Python 回归与7个跨平台helper构建通过；**真实联合执行仍待 Windows READY，未验收活会话撤销/故障/升级，整体4未完成。** 操作与边界见 [交接](docs/HANDOFF.md)。

4C-4V：补齐 compiled 两端实际产品的私有计数观测（不暴露 HMAC/证书），重复查询后仍走真实停止/升级 join 屏障；新增显式隔离 compiled 协调模式，旧模式期限不变。Mac全量race、双平台vet、11项Python、Windows5项局部测试与Linux3项纯协议测试通过。**完整新compiled双机协调、真实TLS持续payload与borrower桌面实测仍待，整体4未完成。** 见 [交接](docs/HANDOFF.md)。

4C-4U（联合验收准备）：补齐隔离双端可信构建、Windows 借用桌面身份入口、独立 session/identity scope 与一次性 bound-node 交接。构建时固定匹配的 CA/issuer/两端 SSH 回环端点，测试 root 只在内存生成，包均 Test=true；不替换正式信任。Mac 全量 race、双平台 vet、11 项 Python 回归、Windows 两项新信任材料测试和包校验/三脚本语法检查通过。双端包仅 build-only，**真实新 compiled Headscale/产品 TLS 联合会话与借用入口原生实测仍待**，整体 4 未完成。见 [交接](docs/HANDOFF.md)。

4C-4T：新增 Windows **测试专用 compiled 桌面入口**，固定 root/配置/签名器/产品摘要，重开桌面测试身份并持有独占原生 journal，通过同用户本地命名管道协调启动、停止和升级；停止真正 join 后才确认，清理仅针对本轮测试数据。Mac 全量 Go race、双平台 vet 与 Windows 4 项管道测试/包校验/PowerShell 语法检查通过。用户已确认 r2 正常 Windows 桌面的初始化、原生签名/独占存储 probe 与 STOPPED 清理全部通过，**桌面原生入口验收闭环**；新 compiled 双机 Headscale/产品 TLS 尚未验，整体 4 未完成。结果见 [交接](docs/HANDOFF.md)。

4C-4S：[产品进程进入 compiled owner 租约屏障](poc/selfhost/public-server/INSTALLED_PRODUCT_OWNER.md) 已接线：产品/网络/节点/watch/native signer 实际 join 后才能提交升级或释放；补取消错误传播、命令不因 Close 超时提前退出、同一编译配置客户端的 OS 签名证书发布/新鲜 TLS 描述符校验。Mac 全量回归、Windows 11 项局部验证及 Mac 原生 signer 严格退出测试通过；真实 Swift/C# 两端固定产品摘要包仅构建，未安装或跑新双机会话，整体 4 未完成。

4C-4R：新增[专用管道/进程对和启动适配](poc/selfhost/public-server/INSTALLED_PROCESS_COORDINATION.md)，macOS/Windows 的 10 项进程行为测试通过。尚未切换真实双机协调、接 OS TLS 供应回调，或将独立产品生命周期纳入 compiled owner 的 journal 租约；不算 Headscale/产品 TLS/安装验收。

4C-4Q：[编译锚节点客户端入口](poc/selfhost/public-server/INSTALLED_NODE_RUNTIME.md) 已开发接线：installed-session与双helper摘要固定、只open已安装配置、OS signer重开、签名ACTIVE binding先于实际tsnet.Up、角色node/IP检查、nodeproof绑定当前grant、TLS监听/workers/节点关闭纳入owner。两端包构建与Mac/Windows局部运行时测试通过；实际双机Headscale/OS signer/产品TLS进程组合仍待，未初始化真实信任channel或改旧GUI，整体4未完成。

4C-4P Windows r3完整桌面验收已由用户确认VALIDATION PASSED，第一段闭环。r2的Unix服务器Store测试依赖问题已通过仅内存临时授权签名fixture修复，正式Store权限未放宽；不代表实际Headscale、产品TLS/GUI或签名安装包验收。见[交接](docs/HANDOFF.md)。

4C-4P：[live配置owner与退出屏障](poc/selfhost/public-server/LIVE_CONFIGURATION_OWNER.md) 第一段已闭环：已安装配置与会话串行、cancel后真实join再commit、超时保留原生租约并锁闭；补齐连接/watchdog/授权callback退出等待。Mac新增原生实测、真实本地HTTPS/opaque tunnel升级集成、全量race及双平台vet通过；用户确认Windows r3完整桌面通过。未切换现有enrollment-helper/GUI或完成实际Headscale节点/产品TLS组合验收，整体4未完成。

4C-4O：[编译固定安装锚/离线配置升级](poc/selfhost/public-server/TRUST_INSTALLATION_RELEASE.md) 已闭环：固定root/service/floor/channel/helper摘要、明确首次安装、原生升级与拒降级、失败保留原提交，换根不选择新存储。Mac编译产物真实验收/全量race通过；用户确认Windows新版桌面验收返回本轮VALIDATION PASSED。现有预发布公开材料已构建维护工具但未安装，未接实际客户端会话或修改产品包，整体4未完成。

4C-4M/4N：[macOS、Windows原生信任存储](poc/selfhost/public-server/TRUST_DESKTOP_JOURNAL.md) 两端实现与原生验收闭环：Keychain/Credential Manager、独占helper、分块提交索引、防降级重开、异常拒重置。Mac4项原生验收/全量race通过；用户确认Windows桌面新包返回VALIDATION PASSED，SSH凭据库不可用记录仅为历史。下一步可信安装锚供应/配置升级，未接产品客户端或完成安装验收，整体4未完成。

4C-4L：[Linux root专属信任journal](poc/selfhost/public-server/TRUST_LINUX_JOURNAL.md) 已实现并通过4项Linux真实跨进程验收：独占owner、重开保留版本下限、非特权拒绝、缺失/损坏拒重置和故障锁闭。只保护非特权本地用户，不防root/整盘快照回滚；未接桌面存储、正式安装锚供应或客户端。整体4未完成。

4C-4K：[可信安装与版本下限核心](poc/selfhost/public-server/TRUST_INSTALLATION_CORE.md) 已完成：先验证、等待旧会话退出、持久化提交，再允许使用新配置；拒绝降级/同版本改写，故障锁闭。全量 Go race/vet 和 Linux/Windows 测试构建通过，存储测试使用内存模拟，尚无生产 OS 存储适配或安装包接入。用户已确认4C-4J Windows STOPPED，整体4未完成。

4C-4J：[旧ACL留存时真实TLS关闭/显式恢复](poc/selfhost/public-server/SPLIT_TLS_FAULT.md) 完整双机实测通过，退出0。隔离daemon实际暂停、HTTPS503且旧最小规则仍在，真实TLS非timeout关闭；同verifier身份显式恢复后旧gate/连接不自动重开，显式新helper/TLS可恢复，最终撤销/清理/原策略恢复。线上Store/Nginx未变，Windows最终STOPPED待确认；非正式信任安装/公网产品，整体4未完成。

4C-4I：[独立 daemon/frontend 的真实产品 TLS](poc/selfhost/public-server/SPLIT_PRODUCT_TLS.md) 完整双机复测通过，退出0：真实Swift/C#、OS签名证书私钥证明/新鲜描述符、错pin/错HMAC/认证前载荷拒绝、自主HTTPS刷新跨lease、撤销关闭非timeout、精确回收恢复。修复先停daemon导致cleanup_pending尚未提交的清理竞争，Linux5项断言通过。Windows最终STOPPED待确认；旧ACL残留故障/正式信任安装等仍待，整体4未完成。

4C-4H：独立 daemon/frontend 链路的[真实双机节点证明](poc/selfhost/public-server/SPLIT_OS_CONSENT.md) 完整复测通过，退出码0。注册前公钥/单次签名意图、两端真实节点与 OS 身份持有证明、错误身份拒绝、同节点恢复、单向最小 ACL 与正反载荷、撤销后回收/拒绝及精确清理通过。首轮日志清理断言失败已修正并完整重跑；产品 TLS 尚未接此新链路，整体4未完成。

4C-4G：[独立 daemon/frontend 双机 OS 授权](poc/selfhost/public-server/SPLIT_OS_CONSENT.md) 真实双机通过，退出码0：两端 OS 签名注册、pending/拒绝/自批准拦截、明确批准/同grant校时验签、撤销及清理恢复。协调端只有公共 bootstrap/stop，无代签批准接口。此阶段只验设备 consent，不宣称客户端 nodeproof、产品 TLS 或可信安装验收；Windows 最终 Enter/STOPPED 待确认，整体 4 未完成。

4C-4F：[私有前端网络路由/跨进程HTTPS](poc/selfhost/public-server/PRIVATE_FRONTEND.md) 完整实机通过，退出码0；默认network404，显式开关仅回环+broker。真实root daemon/nonroot frontend/签名设备HTTPS、无grant拒绝、错误CA拒绝、故障503/显式恢复、重放保持和零退出通过。未部署线上或开放公网，整体4未完成。

4C-4E：[真实独立daemon/非root IPC/恢复验收](poc/selfhost/public-server/DAEMON_NETWORK.md) 已通过，退出码0；修复已join健康故障遗留writer.lock导致显式重启失败。同verifier key/IP恢复、故障非零关闭、不自动重启、精确清理与原策略恢复通过。未启用线上network或完成新daemon双机授权，整体4未完成。

4C-4D：[独立权威服务可选网络装配](poc/selfhost/public-server/DAEMON_NETWORK.md) 代码、全量race/vet、Linux构建及三项Linux隔离组件测试通过；默认device-only不变，尚未部署真实daemon网络后端或开放公网接口。4C-4C Windows最终STOPPED已由用户确认。下一步隔离实机装配验收，整体4未完成。

最新 4C-3D：[单一权威 daemon / 无特权 HTTPS frontend](poc/selfhost/public-server/AUTHORITY_MIGRATION.md) 已受控迁移至服务器，真实权限隔离、回环HTTPS、第二writer拒绝、权威退出503/重启恢复通过，线上State哈希未变。root-only备份保留，新服务active但未开机启用；网络仍关闭。下一步接 verifier/nodeproof/maintenance/fence 真实后端。整体切片 4 未完成。

4C-4A：[真实后端装配与生命周期 supervisor](poc/selfhost/public-server/NETWORK_SUPERVISOR.md) 组件已接入固定Headscale驱动、可信Store公钥与runtime-owned nodeproof；故障取消/join/锁闭测试完成。尚未部署或完成该装配的真实双机验收，线上network继续404。

4C-4B 补齐私有HTTPS→kernel broker验证适配，修正策略目录/database配置与network执行期限；Linux五项隔离组件测试及Windows桌面签名探针通过。真实双机新版协调流程仍待，未改线上network授权。

4C-4C：[新版 supervised 真实双机协调验收](poc/selfhost/public-server/SUPERVISED_NETWORK_TEST.md) 已完整实机通过（退出码0）：真实Runtime/kernel broker/supervisor、两端OS签名/nodeproof、最小规则/零规则真实拒绝、产品TLS、故障锁闭/显式恢复、撤销及同身份verifier重启。实验资源全部回收、原策略恢复，线上State/Nginx未变；Windows最终STOPPED已确认。仍为私有协调实验，非公网产品或整体切片4完成。

最新 4C-3A：[持久 verifier](poc/selfhost/public-server/PERSISTENT_VERIFIER.md) 的明确创建/同身份重启/故障停止/精确回收代码与真实 Headscale 验收通过，测试资源已回收。尚未部署长期 verifier 或接通设备服务网络接口；下一步是固定权限 broker/可信设备公钥绑定，再接实际后端。整体切片 4 未完成。

最新 4C-2C：用户授权在线服务私钥上传后，[独立设备-only 服务](poc/selfhost/public-server/DEVICE_SERVICE.md) 已实际部署到服务器回环 127.0.0.1:8444，首次/重启后 profile HTTPS 拒绝探针通过，状态稳定、停止释放锁与监听，最小权限及原 Nginx 配置哈希核对通过。服务 active，未开机启用/开放公网或网络授权，不改业务 443；整体切片 4 仍未完成。

当前收口 4C-2A：Windows [发布签名服务信任配置](poc/selfhost/public-server/SERVICE_TRUST_TEST.md) 实际验证通过；用户选择独立自托管预发布，隔离本地配置材料已生成并验证。尚未部署公网设备 API、安装可信锚或完成整体切片 4。密钥保管、候选端口 8444 与下一步边界见 [预发布配置](poc/selfhost/public-server/PRERELEASE_BOOTSTRAP.md)。

最新切片 4B-4B：两端真实 OS 身份贯穿设备 HTTPS、Headscale 持有证明、自主刷新与真实产品 TLS 故障/撤销关闭，完整双机验收零退出，资源回收及策略恢复通过。用户确认本轮 Windows STOPPED/随机测试身份清理通过，本轮验收闭环；正式信任锚/HMAC 分发、产品 GUI/安装仍待，整体切片 4 未完成。见 [OS 双机验收](poc/selfhost/public-server/NATIVE_NETWORK_TEST.md) 和 [当前 Handoff](docs/HANDOFF.md)。

另提供 [Windows 本机原生生命周期独立入口](poc/selfhost/public-server/NATIVE_WINDOWS_ENTRY_TEST.md)：无需旧安装包/SDK，Windows 交互式实际验收已通过；此入口不是完整跨网双机 OS signer 启动验收。

Windows [桌面 OS 身份私有桥接（4B-4A）](poc/selfhost/public-server/INTERACTIVE_VAULT_BRIDGE_TEST.md) 的桌面、中文路径启动、SSH pinned 探针和前一轮 STOPPED 均已通过。[真实 OS 双机协调（4B-4B）](poc/selfhost/public-server/NATIVE_NETWORK_TEST.md) 已实际运行通过；独立新包仍是测试入口，不是旧安装包的升级。

从个人使用 MVP 起步：在 macOS 控制端通过外网控制一台 Windows 电脑。最终面向其他用户提供接近 AnyDesk 的独立安装/连接体验，见 [产品路线](docs/PRODUCT_ROADMAP.md)。当前产品基线仍使用系统 Tailscale；独立连接 PoC 验证自托管 Headscale/DERP，数据通道不依赖 Tailscale 托管账号，SSH 仅用于测试管理。

2026-10-07 独立 PoC 进展：helper 注册前公钥与自签意图、真实网络/应用持有证明、单向动态策略、零 ACL 网络拒绝/正向恢复、撤销及磁盘/冷驱动重开已双机验收。产品 TLS 固定载荷桥接已通过，仍未接 GUI 或公开注册服务，不把管理员 fixture 当最终安装体验。见 [验收记录](poc/selfhost/public-server/ENROLLMENT_TEST.md) 和 [当前交接](docs/HANDOFF.md)；下方保留早期 MVP 边界记录。

## MVP 边界

- macOS 只作为控制端。
- Windows 只作为被控端。
- 使用 Tailscale 提供外网寻址、NAT 穿透和加密网络。
- 应用负责屏幕传输、认证、鼠标键盘控制和断线恢复。
- 第一条可运行链路先使用 JPEG；链路稳定后替换为 H.264。

暂不开发自建信令/中继、Windows 控制 Mac、文件传输、剪贴板、多显示器、声音和企业功能。

## 当前进度

P0、TLS 和应用认证双机验收已通过；P1 JPEG 能显示 Windows 画面，画质/真实采集复验和跨网 30 分钟稳定性仍待完成。P2 已把键鼠捕获和同连接 JPEG + 输入接入产品 GUI，目前 Windows 只统计输入、不执行原生注入，待双机 GUI 验收。

上一切片 Windows Release、协议 61/61 和输入边界 6/6 已通过；本切片 Mac 109/109 和 Release 已通过，Windows 构建与双机产品 GUI 操作待验证。按 [GUI 控制 mock 交接](docs/GUI_CONTROL_MOCK_HANDOFF.md) 执行，最新状态以 [HANDOFF](docs/HANDOFF.md) 顶部为准。

最终交付为两个平台各自的安装包：macOS 控制端 `.dmg` 和 Windows 被控端 `Setup.exe`。开发阶段从 P0 起持续验证打包，不等到功能全部完成后再处理安装问题。

## 构建与测试

macOS（需要完整 Xcode）：

```bash
cd macos/RemoteController
swift test
swift build
```

Windows（需要 .NET 8 SDK）：

```powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
```

生成自带 .NET 运行时的 Windows x64 发布目录：

```powershell
dotnet publish .\windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj -c Release -r win-x64 --self-contained true
```

## 开发版安装包

生成 macOS 开发版 `.dmg`：

```bash
./packaging/macos/build-dmg.zsh 0.2.0
```

默认使用 ad-hoc 签名，仅供本机和开发测试。正式对外分发需要 Developer ID 签名和 Apple 公证。

在仓库根目录执行以下命令，使用已安装的 .NET 8 SDK 与 Inno Setup 6 生成自带运行时的 `Setup.exe`：

```powershell
.\packaging\windows\build-installer.ps1 -Version 0.2.0
```

产物统一写入被 Git 忽略的 `artifacts/` 目录。Tailscale 不捆绑进安装包，应用只负责检测并引导安装。

## 文档

- [开发计划](docs/DEVELOPMENT_PLAN.md)
- [测试计划](docs/TEST_PLAN.md)
- [下一会话交接](docs/HANDOFF.md)

## 计划中的目录结构

```text
macos/RemoteController/     Swift/SwiftUI 控制端
windows/RemoteAgent/        C#/.NET 8/WPF 被控端
protocol/                   跨平台协议和共享测试向量
docs/                       规划、测试与交接文档
```

继续开发前先阅读 `docs/HANDOFF.md`，其中记录了实际验证结果和下一步。

### 720p画质对照（0.2.4）

Windows现提供低带宽、标准（默认）、清晰三档，仍最高1280×720/10FPS。自包含目录和Setup已生成；32项协议测试与合成画质对比通过，真实采集待复验、Mac当前离线。按 [画质交接清单](docs/JPEG_QUALITY_HANDOFF.md) 顺序验证，清晰档不保证跨网不卡顿。

### Mac 本地输入预览（开发工具）

同步本轮代码后，在 macos/RemoteController 运行：

~~~bash
swift run -c release InputPreview
~~~

点击开始后测试窗口内键鼠、黑边与失焦释放；只显示本地事件计数，不连接 Windows，不记录按键内容。该工具的人工清单已通过；产品 GUI 的跨机 mock 仍按 [GUI 控制 mock 交接](docs/GUI_CONTROL_MOCK_HANDOFF.md) 单独验收。

### 输入发送状态机

有界队列、相邻移动合并、单写入序号、认证门禁、心跳与结束期限已加入。固定回环的 TLS 网络适配器已编写，Mac 新增代码仍待编译和实测；当前优先完成 [流程检查点](docs/WORKFLOW_REVIEW.md)，GUI 仍只读。

git -c http.proxy=http://127.0.0.1:1080 pull --ff-only origin main

git -c http.proxy=http://127.0.0.1:1080 push origin main
# 2026-10-10 验证入口状态

最新r7真实双机验收实际exit0：Headscale精确绑定/单向payload、真实产品TLS持续传输、在线签名撤销、严格故障三帧/actual exit1与双端原生退役全部通过。节点回收、协调器正常join、原策略恢复及正式服务基线全部PASS，未使用独立恢复代替自动收尾。**用户已确认两个Windows桌面均STOPPED；authority-outage、真实升级、正式公网/秘密投递、GUI和可信签名安装包仍待，整体4未完成。** 已完成与剩余清单见[交接](docs/HANDOFF.md)。下方为历史记录。

故障管道收尾修复已实施：严格验证完成+原生退役两帧后才父端EOF，再验证末帧和actual exit1，不发Stop/强杀或放宽要求。44项Python回归及真实Go子进程/Python父端连续3次收尾通过；新r7公共包与等待入口全摘要/脚本解析通过，待Windows READY/WATCH_READY后真实双机验收。**局部修复通过，不等于4X或整体4通过。** 新入口见[交接](docs/HANDOFF.md)。下方为历史记录。

本地独立子进程已复现继承stdin的close/join等待父EOF，而父端等待末帧才EOF的互等：关闭实验父写端后末帧与actual exit1立即完成，3次race及41项Python回归通过。只新增诊断测试和分帧等待标记，尚未修改收尾顺序或进行新双机验收。旧统一超时行号不能证明“首帧超时”，已纠正。下一步修复方案及安全前置条件见[交接](docs/HANDOFF.md)。

最新r6真实复测已通过正向精确payload、反向拒绝、产品TLS持续传输及在线签名撤销；Mac自主完成帧仍超时，最后固定阶段为signer_closed，**完整验收失败**。已独立核对资源为零、Windows故障join/退役及Mac native journal可重开，并恢复原策略/核验正式基线，不计为自动退出通过。用户完整日志确认两窗口STOPPED及本轮Windows测试身份/journal清理；Mac电源日志未见本轮区间整机睡眠，不能把息屏当作根因。下一步检查本地完成状态回报与收尾协议。详见[交接](docs/HANDOFF.md)。下方为历史记录。

正向网络probe已补固定非秘密阶段/原因，只有完整精确payload才通过，不扩时/不重试或回退管理Tailscale。41项Python回归、相关Go race/Windows vet通过；已实际确认双方SSH和空控制平面。新r6 creator与新等待目录上传、全摘要及脚本解析通过，待正常Windows桌面READY/WATCH_READY后复测。**尚未证明网络失败已修复或4X通过。** 启动入口见[交接](docs/HANDOFF.md)。下方为历史记录。

最新r5真实复测通过等待器自动启动、OS授权/节点持有证明及signed binding，但正向网络payload前置检查失败，未进入产品TLS和自主join验证。原协调器清理未完整通过；已用本轮监督器独立回收已撤销节点、精确退役verifier，确认资源为零后恢复原策略并核验正式基线。**复测失败，不计4X通过；用户已确认两个桌面窗口均STOPPED。** 旧入口不复用，下一步补正向网络probe诊断。详细证据及下一步见[交接](docs/HANDOFF.md)。

用户已报告上轮两个窗口均STOPPED。已补产品/watch/worker/node/signer固定非秘密退出阶段诊断，40项Python回归、相关Go race及Windows vet通过；仍要求原故障三帧与实际exit1，不延长退出期限。新r5 creator与唯一等待目录用于重新验收；真实双机结果尚待，不能将诊断或包构建算4X通过。启动入口见[交接](docs/HANDOFF.md)。下方为历史记录。

最新4X实机已通过预启动等待器自动启动、真实节点绑定/TLS持续传输及活会话签名撤销，但Mac自主join故障协议帧等待超时（未分帧，不能确认首帧），**完整验收失败**。已独立核对资源为空、Windows实际故障join/原生退役、Mac journal可重开，并恢复原策略/核验线上基线；不将事后恢复算自动退出通过。两桌面最终STOPPED待确认，下一步定位分层join阻塞，不盲目扩时重试。详见[交接](docs/HANDOFF.md)。

人工第二窗口启动曾在服务就绪预算后发生，未进入4X验收；该borrower已确认正常STOPPED。已新增开发专用预启动等待窗口：先确认等待器在线，再启动服务，新包摘要通过后自动在正常桌面上下文启动borrower，授权和服务期限不延长。39项Python回归/Windows脚本解析通过，新等待脚本已上传，**自动启动和活会话撤销仍待实测**。详见[交接](docs/HANDOFF.md)。

4X-2已接线显式compiled活会话撤销模式：先保持实际TLS活跃，再OS签名撤销，要求自主join/原生journal退役及真实非零终止，不预发stop或以强杀代替。36项Python回归、相关Mac race/Windows vet通过；新Windows creator r4公共包摘要/脚本解析通过，待用户READY与新双机实测。**尚未验收活会话撤销、authority故障、升级/GUI/签名安装包，整体4未完成。** 下方为此前分段记录。

4W两桌面窗口最终STOPPED已确认，随机test身份和journal清理闭环。4X-1已补只读实际join/fault状态，Mac局部race、Windows vet及31项Python回归通过；**活会话撤销/故障编排及新双机原生验收仍待，不算4X或整体4完成。** 当前旧creator已停止，没有需要运行的新验证包。

最新真实联合轮次已退出0：fresh compiled原生双端、Headscale节点/OS身份绑定、最小单向ACL、真实产品TLS持续payload与自主刷新、正常join后签名撤销及节点回收/策略恢复通过。桌面两个窗口最终STOPPED待确认。**活会话撤销、升级/故障、GUI和签名安装包尚未验收，整体切片4未完成。** 下一切片优先compiled活会话撤销与故障关闭；下方失败轮次为历史记录。

最新轮次已通过 fresh borrower 原生探测与完整 OS 授权流程；节点 helper 的旧上传路径超时，尚无节点绑定/TLS结果。已将其压缩上传与摘要检查提前到授权前，30项协调回归通过；撤销和服务端资源回收实际通过，Windows borrower STOPPED待确认。

上一轮 borrower 已由用户确认正常 STOPPED、测试 journal 清理，身份 creator 仍 READY。就绪等待改为服务启动后的绝对 10 分钟预算（包含构建和上传），保留原 15 分钟服务期限及授权有效期；29 项协调回归通过。真实 compiled 双机联合验收尚未通过，须使用全新匹配验证包，不能复用已回收端点。详见 [交接](docs/HANDOFF.md)。
