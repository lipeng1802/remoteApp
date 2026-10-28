# 卸载前的 owner 维护屏障

`clientowner.Owner.Maintain(ctx, action)`是可信进程内接口。它复用运行中owner持有的原生journal独占租约，不发放能脱离owner使用的“已停止”回执。

流程：永久封闭本owner的新Start/Upgrade → cancel当前runner → 等待runner实际join → 读取仍持租约的committed epoch → 在同一串行锁和原生租约下执行一次维护action。Close必须先取得同一串行锁，因此action未返回时不能释放租约。返回后仍持有租约，须显式Close才能释放；本owner不可重新启动会话。

join超时只允许显式重新进入维护以完成同一次join，action尚未执行。runner故障拒绝action。调用action之前即记录attempted；action错误、取消或不确定回执都不能在同一owner重复执行。失败不会reset、初始化或改写原生trust/identity/activation/floor。该状态仅在本进程中保持，不能当成跨崩溃的持久卸载事务。

action必须遵守context，结束前join其子任务，不得重入owner或删除原生身份、信任、activation、防回退记录。只有可信本地实现可提供action；网络/IPC不能上传回调、路径或epoch。当前验收action只删除测试临时目录内的固定marker，不删除产品文件。

测试覆盖：join前marker不可删除；超时保持native lease；另一helper进程持租约失败；join后执行action；Close不能超越阻塞的action；action错误或重复调用拒绝；runner fault拒绝；原生journal字节保持一致；显式重开仍为epoch2并拒降级。macOS/Windows原生测试需要指定当前trust-journal helper；Windows须用普通登录桌面或Interactive计划任务，SSH只负责管理上传。

Windows便携验收包需要同目录的`maintenance-tests-win.exe`、`trust-journal.exe`及`run-maintenance-validation.ps1`。runner显式提供helper，不允许因缺少环境变量跳过原生项，并在结束后恢复进程环境。测试只使用随机TEST命名空间，清理由test teardown完成。

r53–r55依次补上继承管道握手、持久维护阻断及TEST文件删除恢复，详见下面各节。独立卸载器到live owner的认证连接、生产文件删除边界与完整发行集成仍待完成，实验DMG/Inno继续不提供产品卸载入口。此屏障以Runner“返回前全部join”的契约为前提，不能证明任意逃逸子进程或被强杀进程的后代已退出。

## 实际自主产品的维护控制（r53）

`installed-autonomous-product`现使用`installeddevice.ControlProduct`在实际owner之外路由专用继承管道，普通device/network/stop请求仍交原runner。管道必须由可信本地父进程建立；没有新增网络接口或可由任意本机进程连接的socket，也没有另行实现同用户IPC身份认证。

在原`open`及产品`ready`之后，父进程发送严格的`{"Op":"maintenance_begin"}`。产品调用当前owner.Maintain，等runner与原生signer实际join后，在仍持有native journal租约时返回`{"status":"maintenance_ready","epoch":N,"joined":true}`。父进程必须在同一管道发送严格的`{"Op":"maintenance_finish"}`，随后关闭stdin。完成当前进程的binding、journal、读取线程收尾后，产品才输出终态`closed`并零退出；父进程仍须观察stdout EOF与实际进程退出。

整个维护握手上限40秒，含join与等待finish。begin/finish不能附带身份、公钥、设备请求或网络grant；提前finish、断管、取消、写输出失败、runner fault均拒绝。握手期间不再转发普通操作，不可借此启动新session或替换owner。`maintenance_ready`只是本连接正在持租约的状态，不是独立删除许可；目前既不接收删除路径，也不删除程序文件。

控制层错误可以早于runner完成，因此实际产品在所有控制返回路径先Stop/join，再关闭安装binding，最后由原收尾释放trust journal。不能因参数拒绝或管道断开而提前释放runner借用的资源。

原生组合验收在两代trust/identity升级之后启动真实compiled product，先验证带身份覆盖的begin失败且非零退出，再验证正常begin/finish、握手前后另一helper争租约失败、closed/EOF/actual exit0、退出后trust仍epoch2且设备公钥/idle状态不变。该场景使用已安装的真实OS身份和idle runner，不建立Headscale节点或活跃远程TLS会话。

## 持久维护意图（r54，取代r53结束后可直接重开的行为）

产品控制管道现在调用`Owner.MaintainDurable`。它先在原生trust journal的同一条installation record中提交`maintenance={version:1,state:"pending",id:<random 128-bit>}`，再取消并join runner；只有意图提交、原记录读回核对及join都成功才报告ready。这样进程在等待join时崩溃也不会留下允许新会话启动的窗口。同一owner因join超时而重试，只继续原事务的join，不能改为非持久模式或生成第二个事务。原有`Maintain`仍是可信进程内的非持久屏障，产品管道不再使用它。

marker保留原profile、accepted_at、anchor与epoch；不改OS设备身份、binding或activation。普通`OpenInstallation`、新owner、offline inspect/init/upgrade都会拒绝pending。旧严格schema解析器也不能忽略附加maintenance字段。显式null、未知阶段/version、错误ID及未知字段一律拒绝，不能降级为旧active格式。提交回执不确定时原owner封闭且不执行action；即使写入实际完成，仍需要明确诊断而非猜测成功。

`trust-install maintenance-state`只读验证历史签名记录并输出`maintenance_required`与epoch，不返回可用profile、密钥或独立删除许可。它不创建、不修复、不清除marker。`maintenance_finish`只结束当前握手/进程，不代表卸载完成，不清除持久pending。本切片没有生产resume/reset入口；后续必须根据签名程序文件集及真实事务状态设计显式恢复。

原生测试覆盖正常关闭、真实子进程exit77跳过defer（发生于已取消但尚未join的runner）、helper观察EOF后释放租约、新进程拒绝启动/重装/升级，以及真实native commit后模拟丢失回执。后者是回执故障夹具，不宣称真实磁盘写失败。原生epoch2及同一事务ID在重开后保留。

组合validator在原身份升级/拒绝非法请求与pending-binding回归结束后，显式建立第二个随机TEST身份验证durable产品握手。退出后普通product重开与两代trust init/upgrade都拒绝；只读maintenance诊断仍为epoch2。`inspect-maintenance-test`仅在编译Test=true且已验证pending后核对原公钥与idle状态；TEST-only teardown在取得native租约后才能清理这些随机scope。生产入口不会获得任何重置或删除能力，pending保留到未来明确恢复流程。

## 签名程序文件删除与显式继续（r55，TEST-only）

`programremoval.RunTest`只接受编译Test=true的release和可信固定安装根，使用factory(false)重开既有原生独占租约，绝不初始化或修复缺失journal。调用进程／helper必须在目标根外；固定编译`program-removal-test`没有运行时参数，不接受网络、环境或命令行提供的root、identity或profile。它不是生产卸载器，DMG/Inno不调用此入口。

| 原生maintenance阶段 | 允许行为 |
| --- | --- |
| pending | 完整校验当前签名文件集合，持久保存计划；所有程序文件必须仍存在 |
| deleting + plan_sha256 | 只加载该摘要绑定的原计划，校验剩余内容后显式继续删除 |
| files_removed + plan_sha256 | 校验计划内程序文件全部缺失；重复执行幂等，文件重现拒绝 |

阶段只向前推进，原随机transaction ID、profile、anchor、accepted_at、epoch不变。任一阶段都拒绝普通产品重开、初始化和升级。原生CAS回执不确定会封闭当前事务对象，必须取得新lease并重读实际阶段；没有清除marker、降低floor或恢复active的方法。

固定`.program-removal.json`包含原始签名release/bootstrap manifests、bootstrap pointer及安装根／目录／文件volume/object IDs、内容大小和摘要。所有允许路径重新从经过公钥验证的固定文件清单推导，不能从计划的任意path字段获得删除权限。最多各8个release/bootstrap版本，拒绝staging、未知文件、未来epoch、目录替换、内容变化、硬链接、symlink／Windows reparse点及跨事务计划。计划先写O_EXCL、flush；原生journal提交其SHA-256成功之后才允许删除。删除期间清单文件本身即使已删除，计划仍保留其原签名供重验。

删除前全树校验，每个unlink前再校验；只逐个移除签名清单内程序文件和bootstrap pointer，不用RemoveAll、不删除目录。r56增加下面的句柄绑定，不再用绝对路径os.Remove删除。根、版本目录空壳和计划保留，以便绑定原对象身份继续；任何清单外内容会阻止删除。OS设备私钥、trust、activation、binding及epoch floor不在文件计划中，默认全部保留。

Windows目录刷新可能在写入／unlink已完成后返回不确定回执。`program_removal_receipt_uncertain`立即结束当前尝试；下一次明确调用必须取得新原生lease、验证原计划、摘要、事务和实际剩余文件，不能自动重捕获计划。pending阶段缺文件仍拒绝，绑定后计划丢失／损坏也拒绝。本实现覆盖进程崩溃，不声明断电持久性、恶意管理员隔离或安全擦除。路径校验和unlink之间仍有同用户恶意并发的TOCTOU边界；正式卸载需要平台handle-relative删除／独占写入边界及签名独立卸载器认证，不能把TEST标志改成false即发行。

原生验收创建两代真实签名文件集合，实际删除首文件后由子进程exit78跳过defer，helper观察EOF释放lease；新进程沿同一transaction／plan digest继续，重复完成幂等且仍拒绝产品启动。验证原生OS身份可重新签名、activation原字节保持、trust除maintenance外原字节字段不变且epoch2。另验完整pending计划续接、计划替换／丢失拒绝、原生丢回执封闭及目录／文件篡改。macOS额外实际编译固定入口并核验运行时覆盖拒绝、删除、重复重开。Windows包由`run-removal-validation.ps1`在登录桌面或Interactive任务运行，不需要Go／开发环境。native测试只用随机TEST状态，结束精确清理测试凭据；不是卸载用户凭据的实现。

## 句柄绑定与活动资源到删除的联动（r56，仍TEST-only）

删除批次先按根到叶顺序打开并固定全部已签名目录，逐一核验volume/object ID与平台保护策略；目标文件打开后在同一handle上核验ID、单硬链接、类型、大小和SHA-256，保持打开直到删除结束。不支持的平台操作直接拒绝，不退回绝对路径删除。

- Windows目录handle不允许FILE_SHARE_DELETE，防止批次期间祖先被改名；目标handle同时不允许写入／删除共享，已有写handle也会阻止取得删除handle。通过核验后的同一handle调用FileDispositionInfoEx，支持只读签名文件，无DeleteFile／路径chmod回退。实测覆盖目录／文件rename阻止、已有写入者拒绝、实际只读文件删除及结束后handle释放。API依据：[Microsoft SetFileInformationByHandle](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle)。
- macOS／Unix逐级openat + O_NOFOLLOW固定目录，目标仅相对已持有父目录打开；unlinkat以同一父目录fd执行，并在unlink前再次检查basename仍指向已核验inode。实测在打开目标后把原父目录改名、原路径替换成指向外部目录的链接，仍只删除原固定目录中的签名对象，外部同名文件保留；替换最后一级文件inode时拒绝。Unix的最后一级检查与unlink仍非原子，正式发行仍需保护根／排他写入权限边界，不能宣称同用户恶意并发问题全部解决。

`Owner.MaintainDurableState`只向可信进程内callback提供实际join后、仍持原生lease的精确pending transaction，先核对调用者编译release与owner自身完全一致。`programremoval.QuiesceAndRemoveTest`将它与删除组合：预检固定根 → durable intent → cancel + 实际join → 取得transaction/epoch → 明确Close成功 → 新lease重开并核对同一transaction/epoch → 原签名计划删除。join／Close失败不进入删除；join中取消保留lease、不得写计划，可显式继续同一次join。runner fault保留pending并拒绝删除；已完成fault的显式Close仍报告错误。无法靠同epoch或另一事务的“已停止”状态混用owner。

文件系统回执不确定时不重新驱动已退休owner；由调用者明确使用`RunExpectedTest`核对原transaction/epoch（若已有plan digest也必须一致）后继续。该snapshot不是跨进程删除能力，不可替代独立卸载器身份认证。未新增可由任意本机进程连接的socket、网络维护接口或生产卸载前端。

新增原生联动测试使用两个实际本地TLS HTTP流，维持请求直到取消。即使两路网络worker已经返回，只要runner最终join仍被测试闸门阻塞，另一native helper取不到lease、计划不存在、程序文件仍在；正常join与join中取消后显式继续都完成相同事务的删除。另验错编译release不会取消活动owner，错误transaction/epoch不写计划，runner fault不删文件，以及r55真实崩溃／身份／floor回归。这是原生owner与真实本地TLS资源的联动，不是实际`tls-product`/Headscale跨机远控会话或独立卸载器认证验收。

## 独立进程的设备密钥认证（r57，TEST-only）

`localmaintenance.NewTest/RequestTest`新增独立本地进程间的双向证明；仅允许编译Test=true和明确的数值loopback TCP地址。没有Tailscale／公网维护端口，也不把现有产品stdin重定向成网络接口。地址、公钥和OS signer均由可信本地安装／测试配置提供，不能从peer frame选择。当前授权principal是同一安装用户已有的OS保护设备密钥，获准使用该密钥的进程属于同一principal；**不等同于只准某发行方签名卸载器连接**，没有声称已验证调用进程的发行方、OS peer PID或用户同意UI。

线序为hello → 已签名challenge → 已签名authenticate → owner durable维护／actual join → 已签名ready → 已签名finish → owner.Close成功 → 已签名closed。各签名使用独立phase domain和canonical frame的SHA-256；绑定完整编译release specification摘要、channel scope、服务实例随机ID、双方新鲜256-bit nonce，ready/finish/closed额外绑定真实native transaction和epoch。相同channel但不同编译release也拒绝。签名公钥始终来自既有可信OS身份配置，不从challenge取得；challenge、ready或closed被替换均无法通过client验证。

未经认证的错误不会调用Maintain、取消runner或写pending，可在最多16次无效尝试的预算内继续。认证前读取预算3秒，认证及收尾整体40秒；所有读写有deadline和context关闭，Close会取消当前交换并解除阻塞，调用者仍须join Serve。一次authenticate验证成功后端点即消费，不因join、签名、输出、finish或Close错误重新武装。ready后断连不自动Close owner／释放native租约，也不进入文件删除；须明确收尾，pending保持。

`identity-vault`新增五个严格签名domain：`remoteapp-local-maintenance-{challenge,authenticate,ready,finish,closed}-v1\n`，后面只能跟64字符小写SHA-256 hex；不能签任意JSON、reset或其他新动作。设备私钥仍仅在原生helper内使用，不向client／server发送私钥。认证signer由endpoint独立持有，不能借用会在runner取消时关闭的网络signer；必须等Serve返回后再CloseJoined。

`cmd/authenticated-removal-test`与`CompileAuthenticatedRemovalTest`是固定编译的独立TEST客户端，pin root、endpoint、release、OS vault scope/public、两个同目录helper摘要，不接受运行时参数或环境选择。只重开既有OS身份，不创建密钥；确认signed closed后才调用`RunExpectedTest`重新取得原生lease、核对相同transaction并删除。目录刷新回执不确定仍停止本次尝试，不重复认证已消费的endpoint；显式恢复使用原native计划。本入口没有接入DMG/Inno或默认MVP产品。

原生跨进程验收用真实Keychain／Windows Credential Manager签名helper：独立子进程重开同一身份，双向认证后取消真实本地TLS流；runner最终join前lease不释放、计划不存在。成功路径由子进程取得新native lease完成同一事务删除，原key可继续签名且epoch2保留；finish签名前中断时client失败、owner仍持lease、程序文件与pending保留，只有测试显式Close后才能重开诊断。协议测试另外覆盖重放、reflection、错误key／release／scope／epoch、路径覆盖、duplicate/oversize帧、伪造server transcript、stalled reader关闭和生产模式拒绝。

macOS还实际编译并执行固定客户端，验证运行时覆盖拒绝、独立认证和删除。Windows便携runner须增加`protocol-tests.exe`、`vault-domain-tests.exe`，并使用含新domain的identity-vault；不能混用旧helper后声称通过。真实`installed-autonomous-product`目前仍用原可信继承管道，未自动开启该监听器。后续须做既有binding选择身份、受保护endpoint发现、发行方／OS peer身份与明确同意策略、实际产品binding／所有子进程／实际退出后再交接删除，以及Headscale跨机活会话验收；loopback+设备密钥PoC不替代这些生产边界。

## 实际产品进程退出交接（r58，TEST-only）

认证服务由产品外的`productmaintenance.Controller`监督层持有，不能随产品一起退出。`StartTest`先从既有原生autonomous binding读取并校验身份，关闭这次检查租约，再由可信callback重开对应OS signer；不创建身份，也不接受网络frame选择vault/public。随后启动hash-pinned实际产品，并在专用匿名管道发送无身份覆盖的open，验证ready。部分启动失败仍返回所持Controller，必须明确Release并join，不可因超时丢弃子进程或signer。

实际`installed-autonomous-product`改用`ControlProductWithState`。原maintenance_begin/finish保持兼容；仅编译Test=true额外接受严格无附加字段的`maintenance_prepare`，由真实owner的MaintainDurableState在actual join后、仍持原生lease时返回transaction/epoch/joined。该命令只在原可信继承管道，产品本身不新增TCP listener，生产Test=false不能开启prepare。

监督层将认证服务的MaintainDurableState映射到这个prepare，并把同一事务交给signed ready/finish回调。finish验证成功后，Controller.Close调用`Child.FinishMaintenance`：发送一次终态maintenance_finish，必须收到唯一、无附加字段的closed，等待stdout EOF、读取线程结束及真实进程零退出。不能用Exchange读到closed即放行，也不能把之前的普通Stop升级成maintenance finish。完成后还要取得新原生trust lease核对原pending transaction/epoch，以及重开binding租约核对记录完全不变；任一失败不签closed、不授权删除。认证signer位于产品根外，保持到Serve实际返回，再由明确Release关闭。

确认中断不发送finish：实际产品继续持有trust/binding租约、等待可信管道，程序文件保留。监督层显式失败清理可发送普通stop使该未完成握手失败退出；即使后来join，也只代表清理结束，不变成卸载授权。Close期间超时或非零退出会锁闭该Controller，不会因晚到的零退出重新放行。Release遇到join deadline保留child/signer，必须对同一对象继续等待，不能强杀当作成功。

原生验收用实际编译后的产品程序和原生binding/OS身份/enrollment idle状态，独立客户端完成双向认证，监督层等待完整退出后才删除包括实际产品exe在内的签名TEST文件集；然后binding租约可重新取得、记录及epoch2保留。中断finish分支验证实际产品仍存活、租约仍持有、exe不删，显式失败退出后pending保留。另有真实子进程夹具测试closed后迟迟不退出、非零退出、重复/额外字段closed、原生退出后复核失败和signer在deadline后的持有，均不能变成成功。

`cmd/product-maintenance-validation-build`为两平台生成单代公开TEST fixture，离线签名key仅在内存，编译实际产品并保留已有helper/TLS payload字节。未使用的installer/launcher文件明确为fixture占位，不能运行或当作安装包；真实产品处于idle，不启动Headscale节点或实际远程TLS payload。Windows runner还要求`child-tests.exe`、`supervisor-tests.exe`及`fixture/fixture.json`，缺少入口必须报错而不是跳过。此轮证明实际产品生命周期/身份选择/退出交接，不替代跨机活跃产品会话、正式发现/同意/publisher策略、未知逃逸后代进程隔离或签名安装器验收。默认launcher/GUI未自动接入该监督层。

### r59 可信业务转发与独立TEST启动入口

`Controller.Forward`仅转发产品原有普通device/network命令与一次provision，拒绝生命周期、维护和身份选择字段，与维护共用串行锁。provision不论成败均消费机会，成功必须严格network_watch/passed/active，随后普通业务封闭；超时/异常回执锁闭，仍需Release实际join。业务回执可能含邀请或grant，必须保留在可信继承管道，不能记录或转发到公开日志。`Count`为仅TEST的实际payload聚合计数，不暴露密钥；未启动返回product_unavailable且保持owner。配置ACTIVE不证明实际payload，跨机验收须观察PONG真实增长。

`cmd/supervised-autonomous-product-test`将此路由与r58外部维护监督接在一起，编译固定release、实际产品路径及hash，身份只从原生binding选取。维护服务监听数值loopback，仍以同一安装用户设备密钥为principal；没有新增公网维护接口或publisher认证。关闭顺序为reader/endpoint实际join、产品实际退出、独立signer join；中断认证不自动停产品，由可信父管道明确stop。macOS已实际编译运行并验证普通stop后trust/binding保持；Windows验证了路由library与实际产品维护链，尚未执行新编译launcher。默认GUI、生产发现/同意与Test=false仍未接入。服务端旧部署缺少session-identity能力，须兼容升级后再验真实Headscale活会话。
