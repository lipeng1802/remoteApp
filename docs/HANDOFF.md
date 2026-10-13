# 下一次 Codex 会话交接

## 2026-10-13 r24 已推送r23，内部注册/凭据消费组件开发与本机回归通过

用户要求先推送再继续任务。fetch后HEAD比origin/codex/connection-architecture-poc领先131、落后0，工作区干净；使用http.proxy=http://127.0.0.1:1080非force push成功，origin从4d08bdd更新到f4bb336（含README）。r24新代码仍本地提交，未再次推送；不把未推送r24当作远端最新包。

新增installeddevice EnrollmentRunner / EnrollmentNode可信内部组合；原Runner与compiled device-client不启用provision，首帧/根/地址/helper pins不变。可选组合仅新增provision+签名Grant，拒绝父端凭据/路径/ticket/intent/challenge/nonce/time/action/混字段。经过r23签名校时后内部离线公钥、fresh16byte nonce/OS signer接口签署intent、直接签名HTTPS enroll、仅内存交给Register。90秒/grant期限总预算与5秒HTTP预算、前后公钥/主体/期限/节点key/规范100.120 IPv4核验。输出固定network_enrollment/passed/active=false，ticket/key/IP留内部，Up不是active或持有证明。

尝试副作用前one-shot锁闭只限当前owner；工厂部分资源带error也要join，未知enroll/Up不重试、不删state或冒充远端cleanup，归issuer reconciliation。credential字段所有返回分支清空、tsnet AuthKey字段Up后清空，但Go/SDK/HTTP内部string副本没有物理擦除保证。注册后本阶段仅stop可继续，grant到期timer fault关闭，node CloseJoined先于HTTP/signer实际join，未强杀。node接口要求CloseJoined即使error也真实等待完成，不能把“发出了cancel”实现为此契约。

固定controlURL tsnet adapter只在可信调用者已持有原生目录lease/OS ACL的条件下使用，拒绝明显未准备/相对/不规范/末级symlink/POSIX开放路径，Windows必须另行原生DACL验证；只允许受保护空目录，任何留存材料阻止再次注册，避免旧登录使Up忽略新AuthKey。保留材料，不通过删state或换目录自动恢复。adapter不提供生产路径布局或durable attempt journal。**因此本轮不启用compiled entry**：必须先补跨重启未知结果门禁与双平台原生节点存储租约/ACL，再接native组合验收，不能靠内存attempted宣称重启安全。真实Up、实时nodeproof/bind/watch/产品TLS与GUI也未实现；已有正式服务器仍r22长期database/零ACL/回环8444，无本轮线上操作。

新增7函数覆盖签署intent/credential内部消费/重试拒绝、6前置条件、9HTTP异常、6注册结果异常、实际离线节点Prepare/key重开/损坏不修复、3工厂故障、11IPC注入拒绝。真实本机HTTPS Runner扩为5组，enroll正常/fault/idle授权过期用node fixture与memory signer，两个真实join屏障；不冒充OS signer、批准Store/Mint/Headscale。5相关包race/vet PASS；darwin/windows installeddevice测试二进制build-only，未上传/执行。没有新验证安装包或Windows动作。

下一项把原生节点布局/DACL/独占lease、跨重启attempt/unknown outcome和r23终止EOF native进程验收一起收口，然后启用正式入口并继续proof/bind/watch。要准备专用新验证入口，不跑旧r18设备validator冒充网络验收。整体4仍未完成，Windows暂时无需开机。详见public-server/INSTALLED_DEVICE_CLIENT.md r24。

## 2026-10-13 r23 已安装客户端直接签名网络校时/状态门禁开发完成

本轮选择自主enroll前的授权门禁，不开放公网、不改服务器、不运行或上传Windows/旧安装包。已安装device-client仍固定编译root/helper摘要，当前native owner profile与OS signer接口派生直接HTTPS客户端；后续新增network帧仅grant_clock/state+签名Grant，拒绝首帧网络、混device/stop、外部nonce/time/challenge/ticket/intent/certificate及enroll/bind/TLS动作；JSON omitzero保留旧设备帧形状。

授权gate校验固定issuer、canonical grant/ID/两个不同主体、自身公钥SHA主体与签名。签名结构初验使用issuer颁发时点，不当作仍有效授权；grant_clock独立32byte随机挑战、5秒budget/完整monotonic RTT校验issuer clock proof，再用AuthorityClock重验grant期限。每次state要求已校时同grant、签名/合法binding/严格3秒租约及grant未过期。重复旧proof、错误issuer/签名/grant、父字段注入、签名前后身份改变、HTTP/解析/ctx取消/超时均失败清clock并结束owner；真实signed inactive可报告passed=true/active=false但同样终止。没有缓存状态放行、自动恢复、连续watch或node启动。

输出仅固定network_authorization、phase、passed/active，不包含proof/state/binding/ticket/credential/TLS/HMAC；clock passed不是network active，active观测不是隧道能力。NetworkClient两类HTTP回复复用decodeHTTP严格嵌套重复/大小写重复/未知/尾随检查，不以TLS/JSON shape当签名授权。原设备API失败处理保留，网络授权失败独立fault语义。

native command沿用输入EOF握手：父端发送stop或收到网络终止结果（passed=false，或state/active=false）后，先半关闭专用输入，再等最终failed/closed及actual exit；不能反过来。当前证据仅Runner/signer join，未覆盖新增网络故障的真实native子进程退出，需后续专用fixture收口，不宣称自主完整process join已经通过。

新授权7个测试函数（含normal/inactive两组真实本机HTTPS Runner+memory signer+实际signer join屏障、nonce replay/cancel/issuer/identity/期限/secret混入等子项）；HTTP回复1个测试含9组真实TLS parser正负项。fixture无真实批准Store/Headscale，不冒充OS signer/native journal/完整注册验收。测试最初grant域名前缀误写导致失败，按现有remoteapp-invite-grant-v1纠正fixture后全部重跑，没有改协议或弱化校验。

installeddevice/invite/installed-device-client/devicevalidation/clientowner五包race与专用vet PASS。fresh `installed-device-network-r23-mac` / `installed-device-network-r23-win` 用r18既有native helper bytes、固定编译TEST profile/root/helper pin与manifest build-only通过，未执行/安装/上传。validator仍是旧设备API基线，不验证新增network授权，不要要求用户跑它后声称新原生网络通过。Windows可继续关机，GUI/安装包未更新。

下一项把已验证校时门禁接入helper内部受控enroll/注册凭据消费（不经父帧/日志/文件传secret），再接实时持有证明/bind与连续状态watch、TLS/HMAC供应；需要专用native授权组合fixture，不复用r18验证码测试冒充新能力。r22长期后台继续database/1节点/零ACL/回环8444运行；未经授权不得公开端口或Retire长期身份。整体4仍未完成。本地提交，不推送。

## 2026-10-13 r22 用户批准维护窗口，长期后台部署与真实回环故障闭环通过

用户明确回答“允许现在维护预发布服务”，授权持久Headscale database切换/长期verifier/预发布authority与回环前端配置更换，未授权公网端口/业务Nginx与443改变；Windows关机，本轮没有Windows动作。

新增root-local Python2兼容 `network-maintenance.py` 与 `deploy-network.py` 显式 `--maintenance-approved`/固定root-private stage/逐文件SHA manifest门禁；阶段prepare/activate/fault/resume/rollback，服务器私有检查点持久化。备份原unit/二进制/control/policy及停止后Store审计副本，不自动rollback、不kill、不删锁、不恢复Store旧快照。现有provision工作/控制面非空/路径symlink/未知材料拒绝；显式rollback仅允许原或候选摘要匹配的配置，先实际退出/无锁/精确Retire本轮身份，再恢复原配置，保留当前Store与退休journal。备份不传Mac或客户端。

新Linux `network-deployment-check` 以remoteapp-pre实际UID运行：固定现有安装anchor/profile，验证root/service/floor/有效期与TLS CA/hostname，固定Dial回环8444、禁代理/重定向；内核broker issuer匹配，signed unknown network state被真实ready后台403，空device envelope在签名门禁前拒绝403。不得用签名unknown device请求（会写durable replay/budget）；无register/approve/grant/凭据。网络alias/超长/Origin/query/GET/health负向检查通过，不修改正式Store。最终检查程序额外编译固定operator发行根（`-X main.releaseRoot=<既有可信公开根>`），空pin或前端可写anchor与编译pin不匹配直接拒绝，不能只信任前端可写anchor/profile自洽。`refresh-probe` 逐文件验证后仅更新检查程序/manifest/持久operator副本，不更改服务，实际独立UID/TLS/负向与Store不变复测通过。

初次prepare只停止旧前端即暂停：旧二进制不输出STOPPED frontend。维护工具拒绝计PASS，显式rollback实际恢复旧服务/原配置；没有切换策略或准备身份。新增仅针对备份摘要匹配、旧device-only模板的迁移分支：实际PID消失/systemd状态0/8444无监听，且旧binary摘要保持一致；新前端仍强制STOPPED回执。对应5种负向/正常条件回归通过，未以缺失回执冒充新前端回执。

第二轮stage `/run/remoteapp-network-maint-a3817bf2d4c690e5` 四阶段actual exit0：旧前端/authority正常退出、writer锁释放，Headscale切database/显式零ACL；verifier首次create READY→TERM→STOPPED/actual0，长期身份留存，daemon启动仅重开无Mint；安装r21能力/停止预算候选unit和新frontend，Wants而非Requires避免故障依赖停止吞掉503。前端显式public-network协议模式但仅绑定127.0.0.1（共享4预算，不代表公网开放）。三服务active，独立UID/TLS/负向/issuer通过，正式Store摘要完全未变。

真实fault阶段stop remoteapp-poc后authority FAIL/非零实际退出、writer/owner锁释放、broker消失；前端继续运行，device/network均503。恢复provider后无自动后台重启、两路仍503；resume明确启动新authority实例后原verifier key/IP恢复、两路拒绝/负向通过、显式零ACL、Store原摘要不变，checkpoint为verified_active。原业务Nginx所有基线OK；8444仅127.0.0.1，三个服务仍disabled（未enable自启）。service profile仍当前有效epoch1/原issuer/root，没有重签或换信任根。

恢复材料在服务器root私有 `/var/lib/remoteapp-network-maintenance/remoteapp-network-maint-a3817bf2d4c690e5`：original-*配置/旧binary、store-for-audit-only.json（严禁恢复覆盖）、checkpoint.json/deny.json及operator/完整工具/manifest/产物。operator拷贝持久保存，因为/run stage重启会丢失。第一次失败并回退的备份位于同父目录 `remoteapp-network-maint-6f291da4ec30b872`。两份备份和stage有意保留，未删除材料；后续清理必须精确且确认维护完成。长期 `/var/lib/remoteapp-verifier`、`/var/lib/remoteapp-network-policy`、`/etc/remoteapp-authority/network.json` 为正式私有材料，**不能用r21临时Retire/空控制面restore脚本或r19空资源断言清理**。现在长期verifier占有节点/工单，运行中有锁是正常状态，不再期待全空；实际只读计数及零ACL复核见本轮收尾输出。

Go deviceservice/broker/verifier三包race、Linux入口vet/静态构建通过；Python81项PASS（新增maintenance路径/atomic/旧前端迁移门禁）；Linux检查程序无独立本机fixture单测，以实际部署UID/TLS负向与故障往返为证据，不冒充native OS signer/设备客户端/跨机注册或产品TLS。

下一项：先明确凭据供应和客户端自主注册/网络绑定的端到端入口（不再依赖SSH输送注册凭据/HMAC），或补服务重启运维入口与公网admission门禁；公网8444开放仍需单独授权及信任分发验收。未做自然过期、实际重启整台服务器/自启、真实授权数据面双机、GUI/签名安装包，整体4未完成。Windows可保持关机，需要双机时提前通知。本地提交，不推送。

## 2026-10-13 r21 隔离候选systemd服务真实生命周期验收通过

新增 `daemon-network.py --systemd` 和Linux checker随机runtime unit后端，用仓库候选模板替换为隔离public pins/路径，禁止参数注入/正式路径残留及不安全Restart/停止预算/依赖。实验位于/run，保留PrivateTmp/PrivateDevices/ProtectHome/ProtectSystem/NoNewPrivileges；不安装正式/etc unit或enable。用户补充Windows已关机，本輪不依赖Windows，也未使用桌面签名器。

实机暴露并修复：r20 Store父目录组应为effective GID而不是socket共享GID；Requires改Wants/After避免provider停止吞掉Runtime故障语义；私有Headscale admin socket跨服务UID访问需要DAC override；pinned root/CentOS overlay绑定另需NET_RAW（相同身份仅加该能力后READY/STOPPED成功，不授予NET_ADMIN，不改provider私有权限）；systemd正常停止会清空ExecMainPID/Code，验收器必须保留实际运行PID并检查同PID STOPPED/真实退出，而不是只看inactive。新增固定阶段诊断，不输出身份/地址/错误正文/私钥。没有把第一次失败或单次诊断成功当组合PASS。

最终 `daemon-network.py --systemd` actual exit0：Linux候选渲染/受限精确恢复/正常回执三项测试PASS无SKIP；真实verifier首次create后stop，unit ExecStartPre通过、后台仅重开既有journal，三次独立随机unit实际READY/显式零ACL；批准nobody身份/无grant403、私有读取拒绝，错误root peer由kernel拒绝。正常stop/join保留key/IP，实际stop remoteapp-poc后后台非零FAIL/实际PID退出；provider恢复不自动重启broker，旧身份恢复，显式新unit重启成功，issuer/实验Store未变，最终正常join/零ACL。精确Retire本轮ticket、实验目录/runtime units及上传程序删除，原file模式/原PoC策略恢复，控制nodes/users/preauthkeys归零，三个正式服务active，Nginx原摘要通过。

失败两轮曾保留实验材料和策略备份；显式恢复核对精确随机unit/实际退出/无锁后Retire工单，未kill或删锁/恢复正式Store快照，两轮资源和策略均恢复后再做最终验收。该过程的操作脚本留在ignored artifacts，正式维护入口仅限checker `cleanup-failed-systemd`，拒绝正式路径/单位或活进程。

broker/verifier/authority-daemon相关race PASS，新增诊断隐私/失败门禁回归PASS；Linux静态checker/daemon/test编译与Linux vet通过；Python既有78项PASS。服务端无grant、跨机nodeproof、产品TLS/GUI或签名安装，不把控制面零ACL回读当真实双向网络拒绝。详情见 `poc/selfhost/public-server/SYSTEMD_NETWORK_ACCEPTANCE.md`。

下一项长期后台部署门禁：确定维护窗口与正式verifier所有权/策略database迁移，先保持前端回环完成独立UID真实后台接通与故障闭环，再考虑公网8444。现网正式authority/frontend仍device-only、8444仍127.0.0.1，HMAC/凭据供应、自主enroll/bind、GUI/安装与整体4仍未完成。Windows继续保持关机即可，双机/客户端任务前需提前通知用户。本地提交，不推送。

## 2026-10-13 r20 长期authority网络后端只读预检与候选部署配置

新增 `--check-network-config`：复用严格网络配置，检查签名profile/root/service/floor/有效期、同UID私有Store/恢复journal/node文件及目录隔离，拒绝任何writer/owner锁。只读，不打开Store/verifier、不创建锁/身份/节点、不执行CLI或网络，不自动修复。成功仅固定CHECK PASSED（明确不是live provider/policy/issuer/network验收），真实启动仍重验与独占。

新增候选unit（不安装），ExecStartPre+network-config、专属可写目录、停止预算45秒避免旧10秒早于daemon30秒join。候选仍需目标CentOS7解析与隔离真实网络实测，不把模板当部署。部署/备份/权限/迁移/回退步骤见 `poc/selfhost/public-server/NETWORK_DEPLOYMENT_PREPARATION.md`：不自动删遗留锁，不用旧Store快照覆盖新合法授权/撤销，故障或强杀不算join。现网file策略与database要求仍是启用前门禁。

两项新增预检测试（12类负向子项）确认成功重复运行/失败不改写文件或创建锁；broker/verifier/authority-daemon相关race及vet PASS，Linux amd64静态authority-daemon-preflight-r20 build-only通过。authority-daemon入口包本身无独立CLI测试；候选unit没有目标systemd实机解析或启动证据，不将本机预检材料当真实长期node一致性验收。

本轮无线上改动或Windows操作。下一项隔离Linux候选unit+真实长期后台生命周期组合，再明确维护窗口部署；公网8444/凭据供应/GUI/签名安装包及整体4仍待。本地提交、不推送。

## 2026-10-12 r19 直接HTTPS网络前端operator模式/有界工作槽开发完成，未部署

新增独立 `--public-network`/PublicNetwork opt-in；保留原private loopback-only和默认network404，混用/未知模式拒绝。必须独立broker/规范absolute socket，signed profile非localhost命名域名、规范非特权且精确匹配endpoint端口，TLS/发行根/service/floor/有效期校验及kernel UID/Store issuer握手保留。broker.SignedNetworkHandler设备与网络共享4槽；所有实际授权仍由sole authority完成，没有新Store/issuer初始化或根/审批绕过。保留32socket、admission peer/global预算、JSON/body/IPC限制和network14s/write20s预算；Origin/query/代理头不作为授权或可信peer，不支持未经设计的直接反代或分布式DDoS保证。

新增服务3项+CLI模式回归；deviceservice/broker/device-frontend/installeddevice/invite五包race、专用vet及Python78项PASS。实TLS+Unix broker+独立Store验证register/replay、frontend重启durable replay、JSON alias/超长/query/Origin/health拒绝、默认network404、authority停止两路503与normaljoin；预算memory executor证明四个network工作占满后device也503，不能绕共享限制。不把本机保留测试DNS名/显式ServerName、无网络backend测试当公网或Headscale证据。Linux amd64 `device-frontend-direct-r19` 静态编译成功，未部署。

服务器只读preflight actual exit0： `/var/lib/remoteapp-prerelease/client-public/service-profile.json` epoch1且当前有效，8444仅127.0.0.1绑定，frontend无private/public network参数，authority无network-config，三个正式服务active，Store固定摘要/Nginx baseline不变。初次错误/opt profile路径导致检查失败，纠正到既有/var/lib路径后成功；没有读私钥、发设备请求、上传更换binary/unit、开放端口、安全组或修改Store/策略。当前正式网络后端仍未启用，不可仅加public flag就称自主网络已完成。

本轮无需用户Windows操作。下一任务先补长期authority supervised backend/verifier/权限装配与预检/备份/安全回退，再明确授权部署/候选8444公开监听并实测负向预算/故障与已安装客户端直连；不能改业务443或直接反代private模式。产品HMAC/凭据供应、自主enroll/bind、GUI/签名安装包与整体4仍待。本地提交，不推送。详细边界见 `poc/selfhost/public-server/DIRECT_HTTPS_INGRESS.md`。

## 2026-10-12 r18 Windows普通桌面联合验收通过，双平台入口闭环

用户反馈执行新入口后返回完整预期 `VALIDATION PASSED: actual compiled device client + native trust lease + OS signature direct local HTTPS + stable restart + joined test cleanup. NOT public service, Headscale, product TLS/GUI or signed installer acceptance.`。该成功标记只在validator真实compiled客户端/native journal/OS signer签名请求/稳定重启/closed+actual exit0、TLS server join及精确TEST journal/random identity清理均完成且无error后输出；PowerShell另核验actual exit0与无FAIL/SKIP。证据来自用户桌面反馈，未独立获取逐项耗时，不用此前SSH纯协议测试冒充该结果。

结合Mac fresh r4 actual exit0，本轮新设备客户端的双平台原生联合入口闭环。自动清理已纳入成功门禁，无需额外READY/Enter或再次运行旧包。不修改或删除未知历史凭据，公开测试包保留；本机TLS fixture不是正式authority Store，不能据此宣称公网设备服务或Headscale/product/GUI/安装通过。

下一项优先推进长期服务直接HTTPS入口、客户端自主接入与受保护凭据供应。服务公开监听、operator opt-in、预算/负向及故障退出、profile时效/信任分发与部署回退要分别收口；不能直接反代private-network或默默改业务443/开放端口，正式部署前核验并获取必要授权。整体切片4未完成，仅本地记录提交、不推送。下方Windows待桌面为历史。

## 2026-10-12 r18 新已安装设备客户端原生联合入口，Mac实际PASS/exit0，Windows待桌面

新增TEST-only devicevalidation、installed-device-validation与显式builder及PowerShell自动入口。实际运行已编译r17 installed-device-client，而非用memory signer代替新命令：固定compiled profile/根/原生journal helper/signer pin，随机测试OS identity重开签名，独立loopback TLS fixture校验真实signature/identity/nonce/replay/time；缺原生状态不repair、地址注入拒绝、live原生排他租约、register/invite、stop后closed/actual exit0/原生重开、第二进程稳定身份、最后TLSjoin/精确原生journal/identity删除都验。fixture无真实authority Store/批准grant，不代表服务器durable重启或Headscale。

仅throwaway TEST根与CA在构建内存生成且不写出；loopback leaf TEST私钥编译入validator，无正式密钥上传。约23小时profile与固定回环端口，过期/占用拒绝，不换地址/时间或repair旧包。日志固定类别，不输出注册码、grant、pub/private key或raw HTTP。父子管道私有，测试数据不记终端。child deadline30秒只半关闭输入、超时判失败，实际Wait后才释放测试资源，无kill当成功；原生helper自身失败处置也不能计正常join。

历史首轮/r2未通过；r3分段证据证明register/invite签名通过，失败在parent等待closed却不发EOF，child inherited-stdin读取线程无法join。旧三轮均确认精确测试journal和random identity清理，不把失败计PASS。修正validator的终止握手：stop后半关闭专用input，再严格closed/actual exit0；添加独立进程回归，不改r17生产信任/身份/退出门禁。调用新客户端的可信parent同样应遵守这一终止顺序。

Mac fresh r4 `artifacts/connection-poc/installed-device-native-r18-mac-r4/installed-device-validation` actual exit0、所有检查及两类原生清理PASS，无SKIP。相关7包race/专用vet和Python78项PASS。Windows新包构建、exact manifest、逐文件upload/hash、PS解析PASS；SSH仅运行3项memory fixture/TEST guard/真实pipe half-close-zeroexit纯测试，实际PASS，不读取native credentials。未在SSH运行新native validator，没有改线上服务/Store/Nginx/ACL或Tailscale。

用户普通登录Windows桌面只需一个PowerShell窗口执行（不需要READY/Enter）：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-installed-device-native-20261012-r18\run-installed-device-validation.ps1"
```

末尾必须 `VALIDATION PASSED: actual compiled device client + native trust lease + OS signature direct local HTTPS + stable restart + joined test cleanup. NOT public service, Headscale, product TLS/GUI or signed installer acceptance.`；脚本只有actual exit0/成功标记且无FAIL/SKIP才返回。回复结果后记录联合入口闭环，再继续自主长期HTTPS服务与凭据供应/公开入口，不用旧双窗口或旧r17 build-only包替代。失败不要盲删Keychain/Credential Manager/lock。整体4未完成，本地提交不推送；详细边界见 `poc/selfhost/public-server/INSTALLED_DEVICE_NATIVE_VALIDATION.md`。

## 2026-10-12 r17 已安装配置设备 API 客户端开发/构建通过，原生联合验收待补

第二条线开始收口SSH测试协调依赖，先补客户端信任入口，不开放公网。新增installeddevice Runner + cmd/installed-device-client：编译固定release/signer摘要，同目录helper校验、clientowner.Open/Start只读取已有原生journal的durable profile；首帧只有open/scope/expected public，OS signer重开不create/delete/export。原profile直连HTTPS，无SSH代码/Endpoint/CA/issuer/root/key/helper路径覆盖。只device注册/invite/claim/incoming/approve/status/revoke，没有enroll/bind/watch/product/HMAC。设备结果可能含邀请码/grant，只在trusted local IPC，不能当终端日志。

Owner覆盖client/signer生命周期，ready不证明远端可达；nonce/time不能由父帧传入，未知op/身份替换/initialize/upgrade/信任注入拒绝。stop/信号正常cancel实际join，安装到期fault；HTTP请求退出后client.Close、signer.CloseJoined实际ack/exit，再读取线程join和owner.Close，最后closed。EOF/坏帧fail-closed，不自动重连或修复。原生helper仍有其既有故障处置，强杀或失败join不可输出本轮正常closed。

新可信CompileDeviceClient与builder两种device-test模式，随机TEST channel、无产品参数、精确5项公开manifest。没有改原desktop/product manifest，因此此前双窗口回归不受新制品数量影响。脚本 `artifacts/connection-poc/build-installed-device-r17.py` actual exit0，`installed-device-r17-darwin` / `installed-device-r17-windows`构建、逐文件manifest、Test命名空间与无product字段PASS；只build-only，未安装信任/创建身份/运行新产物/上传Windows包。

验证：installeddevice/installed-device-client/trustinstall/installed-session-build/clientowner/clientruntime/invite七包race PASS，专用vet PASS；新Runner5项（含真实本机HTTPS签名register/invite，stop等待signerjoin、join失败拒成功、profile三种篡改拒绝、命令门禁、闲置cancel与deadline）及命令2项/构建1项通过。测试身份为memory signer、临时私有服务，不能当OS signer/native journal/new command联合验收。Python既有78项PASS。未访问/修改线上服务、监听、防火墙、正式Store/Nginx或Tailscale。

本轮不需要用户启动Windows两个窗口；那些旧r16已经消费不能复用。下一项先准备新入口的原生保护存储/OS signer联合验收专用私有管道入口，不输出验证码/grant；再收口长期服务直接HTTPS入口与公网安全预算/负向、故障退出。正式部署/开放端口需核验当前配置时效、服务状态/备份回退和明确授权，不能把private-network直接反代或默默放宽限制。后续才接自主enroll/nodeproof/binding与产品凭据供应、GUI/安装。整体4未完成，仅本地提交、不推送。详细边界见 `poc/selfhost/public-server/INSTALLED_DEVICE_CLIENT.md`。

## 2026-10-12 r16 两桌面 STOPPED / 收尾核验 PASS，活会话失效后新授权组合闭环

用户确认两窗口STOPPED后，只读 `artifacts/connection-poc/check-r16-stopped.py` 实际exit0。精确creator `windows-desktop-live-reauthorize-20261012-r16` 与borrower `remoteapp-compiled-stage-9c42f5b6a810de73/run/compiled-win` 下Windows子进程0，三个public bootstrap不存在；CurrentUser/My测试证书前缀枚举0，覆盖本轮旧/新两grant scope。远端borrower manifest摘要匹配唯一local compiled pair，对应Mac产品子进程0。Headscale nodes/users/preauthkeys全空，原policy不变，三个正式服务active，Store固定摘要与Nginx基线再次PASS。

原生journal/test identity删除依据普通桌面严格closed/zero-exit STOPPED与bootstrap移除；certificate不存在另经只读枚举确认，不以SSH另一凭据上下文冒充原生journal验收。本轮核验没有额外删除任何文件/凭据；公开包与work保留供审计，未知历史凭据未批量删除或宣称全清理。不重启已消费r16，两窗口可关闭。

结合原协调器actual exit0，r16“活跃旧TLS签名撤销 → 两端自主fault join/native retirement → 新邀请/明确审批/不同grant → 新node scope/idle owner/process/certificate/HMAC → 实际新TLS → normaljoin/新grant签名撤销/完整收尾”已闭环。自然过期实机仍未执行，GUI/正式公网无SSH/发布签名安装包及整体4仍未完成。下一阶段优先收口客户端自主服务接入/正式信任与凭据供应，再接GUI和安装；不将SSH测试协调工具当最终用户入口。仅本地提交，不推送；下方待STOPPED为历史。

## 2026-10-12 r16 真实双机活会话撤销后新授权/新TLS PASS，actual exit0，待两桌面STOPPED

用户两窗口就绪后，实际运行对应creator `windows-desktop-live-reauthorize-20261012-r16`、stage `remoteapp-compiled-stage-9c42f5b6a810de73` 的 `--compiled-live-reauthorize`，原协调actual exit0。桌面waiter live/creator native probe、当前9helpers、fresh pair构建上传逐文件摘要、borrower native probe PASS。上传耗时但本次完整核验成功，不把上一准备上传超时算本次证据。

首轮OS签名注册/明确拒绝/批准、实际enroll/node持有证明/错误身份拒绝/签名binding/正向payload与反向拒绝 PASS。旧产品真实TLS pongs跨lease且两端保持live后，owner签名撤销；未预发stop，两端实际自主fault completion/join/native retirement PASS，Mac旧process实际fault exit1，旧owner保持锁闭、节点回收zero ACL PASS。旧grant online拒绝、old request/code不可复用，旧API实际join/freshAPI READY、新邀请pending/确切peer/拒自批/明确批准/不同grant、旧grant仍拒绝和新grant校时验签 PASS。

新授权第二轮独立节点目录、新node、enroll/持有证明/binding、正反向网络payload与bound helper实际join PASS。旧Mac实际退出、新subject一致/不同grant/不同节点与目录/禁certificate reopen、双APIfresh signed recheck/binding、Mac原生floor inspect、Windows显式新idle owner ACK、新证书fingerprint与新HMAC不同、Mac新PID，随后新产品TLS pongs跨初始lease与两端正常join全部PASS，输出 `compiled_live_revocation_new_approval_new_tls_and_normal_join`。仅不同grant允许不同scope pin，同grant门禁不变，无安装floor重置。

新grant owner签名撤销/status revoked、节点回收zero ACL、durable客户端清理、exact verifier退役、隔离目录移除与原协调STOPPED/actual exit0 PASS。control nodes/users/preauthkeys归零，原policy/线上三个服务/Store/Nginx基线不变，上传程序精确移除。Windows package/work保留等待桌面Close，不提前宣称两份certificate scope/journal最终清理。

请窗口2borrower先Enter直到STOPPED，然后窗口1creator Enter直到STOPPED；收到确认后精确核验本轮Windows/Mac子进程、bootstrap、测试证书和基线。不复用已消费r16。自然过期实机、GUI、公网无SSH/正式部署与签名安装包均不在本轮，整体4未完成。仅本地提交，不推送；下方组合待验收为历史。

## 2026-10-12 r16 活TLS撤销 → 新授权/新会话组合切片，实机待验收

新增独立 `--compiled-live-reauthorize`，不混用fault/reconnect/upgrade。首份已明确批准grant先真实enroll/bind/产品TLS pongs跨lease并保持两会话live，然后owner OS-signed revoke。等待两端自主fault completion/actual退出与native retirement，不预发stop/不强杀；确认节点回收零ACL和旧owner锁闭，才join旧API并开启fresh API、新邀请/pending/明确批准/不同grant。旧请求/验证码不能复用，旧grant在线拒绝检查保留。

两轮Windows节点目录使用各自经过格式校验的grant-ID后缀，Mac独立随机目录；禁止旧目录或旧node key复用。新的BoundHandoff先实际join注册helper，再检查相同subject/不同grant/不同节点和目录/不得CertificateReopen；通过两端fresh signed grant+binding在线验证、Mac原生journal inspect、Windows显式新idle owner ACK后才启动。旧Mac进程必须actual fault exit1，新PID必须不同；新grant使用新scope证书与新HMAC，不允许复用旧fingerprint/key。同grant重连原pin不变门禁仍保留，没有增加生产换pin或重置安装floor接口。新会话真实TLS pongs跨lease后normaljoin，最后再签名撤销并全量收尾。

Python全套78 PASS，覆盖无效live callback组合、撤销/faultjoin先于freshAPI、新grant ID/subject/目录/node拒复用、旧进程未退出/旧grant误放行/inactive binding/numeric ACK拒启动和失败不重试、新pin/HMAC不能与旧相同。相关8个Go包race与vet PASS；没有Go生产变更。回归不等于新组合真实双机结果，自然过期仍未实机，本轮也不包含GUI/公网无SSH/签名安装包或整体4完成。

r16 creator本机构建及TEST namespace、9helpers与服务器空资源/基线前置PASS。初次批量公开文件上传180秒超时，不能算准备成功；用独立resume脚本精确核验目标目录无运行进程/bootstrap/重定向，只补传缺失或摘要不符的manifest白名单公开文件，没有修改任何身份/凭据。resume实际exit0：全部公开文件摘要、fresh waiter摘要、实际Windows原regex路径规则/非reparse/无run和3个入口解析均PASS，输出PREPARATION_READY。尚未启动桌面或网络，旧包保留。

新的窗口1命令（公开包已最终核验成功，不复用r15）：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-desktop-live-reauthorize-20261012-r16\start-desktop-session.ps1"
```

READY后窗口2：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-compiled-stage-9c42f5b6a810de73\wait-for-compiled-borrower.ps1" -Root "C:\Users\jarvis\remoteapp-compiled-stage-9c42f5b6a810de73" -IdentityBootstrap "C:\Users\jarvis\windows-desktop-live-reauthorize-20261012-r16\interactive-bootstrap.json"
```

用户两窗口READY/WATCH_READY后运行：

```sh
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='C:\Users\jarvis\windows-desktop-live-reauthorize-20261012-r16' REMOTEAPP_WINDOWS_DESKTOP_STAGE='C:\Users\jarvis\remoteapp-compiled-stage-9c42f5b6a810de73' python3 -B poc/selfhost/public-server/split-consent.py --compiled-live-reauthorize
```

actual exit0后borrower先STOPPED再creator STOPPED，然后精确核验本轮进程/bootstrap/两份grant证书scope最终清理和服务器基线。仅本地提交，不推送。

## 2026-10-11 r15 两桌面 STOPPED / 收尾核验 PASS，重新授权子切片闭环

用户确认两窗口STOPPED后，执行只读 `artifacts/connection-poc/check-r15-stopped.py`，实际exit0。精确creator `windows-desktop-reauthorize-20261011-r15` 与borrower `remoteapp-compiled-stage-6d93c428f1a7be05/run/compiled-win` 下Windows子进程0，三个public bootstrap均不存在；CurrentUser/My测试证书前缀枚举0。远端borrower manifest摘要匹配唯一local compiled pair，对应Mac产品子进程0。Headscale nodes/users/preauthkeys全空，原policy不变，三个正式服务active，Store固定摘要与Nginx基线再次PASS。

本轮原生journal/test identity清理依据普通桌面严格closed/zero-exit STOPPED与bootstrap移除；测试证书不存在另经只读枚举确认。不以SSH凭据上下文冒充桌面原生journal读取验收。本次核验没有额外删除任何数据，公开包/work保留用于审计，错误命名旧staging仍保留；未知历史凭据未宣称全部清理。不重启已消费r15，两窗口可以关闭。

结合原协调器actual exit0，本轮“网络注册前撤销旧批准 → 新邀请/明确审批/不同grant → 实际Headscale/产品TLS → 正常join/签名撤销/完整收尾”子切片闭环。仍不是旧活TLS撤销后重新授权重连或自然过期实机验收。整体4未完成；后续优先补活会话失效后新授权/新owner的组合流程，再推进正式公开接入、GUI与签名安装包。仅本地提交、不推送；下方待STOPPED为历史。

## 2026-10-11 r15 真实双机重新授权 PASS / 实际 exit0，等待两桌面 STOPPED

用户确认修正路径后两窗口就绪，实际运行 `--compiled-reauthorize`，creator `windows-desktop-reauthorize-20261011-r15` 与stage `remoteapp-compiled-stage-6d93c428f1a7be05`。协调器实际exit0。waiter live/native creator probe、当前9helpers重建、fresh matching pair构建与逐文件上传校验、borrower real desktop probe PASS。

初始设备OS签名注册/拒绝/明确批准 PASS。首份批准在网络注册前签名撤销；两端旧grant online recheck拒绝/无网络、旧status仍revoked无grant、旧request不可重批、旧邀请码不可claim全部PASS。旧API实际join、同身份fresh API READY PASS；新邀请pending无grant/不同request、incoming确切peer、controller自批拒绝、owner明确批准双方同一不同ID新grant PASS。fresh API拒旧grant、接受新grant online校时验签，未bind仍无网络 PASS。

仅新grant执行真实enroll/单次intent、OS/node持有证明、错误身份拒绝、Mac同node重启、signed binding、正向实际固定payload/反向拒绝和bound helper join PASS。双端产品启动及真实TLS pongs持续跨初始lease、controller与target正常stop/join、node/watch/signer正常join PASS。随后OS owner signed revoke/status revoked、客户端节点回收零ACL、durable journal清理、exact verifier退役、隔离资源移除和原协调器 `STOPPED isolated_coordinator_resources_joined` / actual exit0 PASS；原policy、线上服务/Store/Nginx基线恢复不变、控制nodes/users/preauthkeys全空。

本轮Windows公开compiled package/work保留待桌面最终Close。请窗口2borrower先Enter并确认STOPPED，再窗口1creator Enter并确认STOPPED；收到后核验精确本轮Windows/Mac进程、bootstrap、测试证书及基线。不提前记原生certificate/journal清理完成，不重用已消费r15。没有建立首份grant的旧TLS，所以不是旧活TLS撤销后新授权重连；自然过期仅Go fixture覆盖、未实机等待。GUI、正式公网准入、签名安装包及整体4仍未完成。仅本地提交，不推送；下方待实机为历史。

## 2026-10-11 r15 等待窗口 staging root 修正，尚未启动网络验收

用户窗口2返回Invalid staging root。根因是总控准备/交接错误使用描述性后缀，违反原PowerShell与DesktopStage一致的16位小写hex路径门禁；之前语法/hash检查不能证明参数合法。保持原门禁，不放宽用户路径或生产授权。新fresh目录为 `C:\Users\jarvis\remoteapp-compiled-stage-6d93c428f1a7be05`，窗口1creator不变，可继续READY，不按Enter。

新目录已创建/限制ACL并上传原waiter，摘要一致；实际Windows按原waiter中抽取的regex核验root、非reparse/container/无run及脚本解析PASS，最终verify-only实际exit0。首次修复辅助检查自身regex转义错误，未通过不能算验收；随后改用原waiter精确regex并只读复核通过。旧错误命名目录保留，仅公开脚本，没有启动borrower/network，未删除身份或Tailscale。preparer现在先调用共享validate_stage_root再做远端操作；新增描述性r15错误路径拒绝回归，Python全套74 PASS。

用户只需窗口2重新执行下节已更新的hex路径命令，等WATCH_READY后确认两窗口就绪；没有运行r15实际OS授权/Headscale/TLS。修复本地提交，不推送。

## 2026-10-11 r15 新邀请/新审批/独立新授权切片开发完成，等待 fresh 两桌面

新增独立 `split-consent.py --compiled-reauthorize`，不混用reconnect/fault/upgrade。初始拒绝和批准流程之后、任何客户端节点注册之前，owner签名撤销首份grant；两端recheck_grant必须在线拒绝，network_state仍不可用；old status为revoked且无grant，旧request不可再次approve、旧邀请码不可claim。旧API客户端实际stop/join后，从相同OS签名身份/固定私有HTTPS参数开启全新API进程，新邀请必须pending无grant/不同request，owner发现确切peer，controller不得自批；明确批准后双方同一不同ID新grant，旧grant仍不可use，新grant在线验证成功且未bind不开放网络。仅新grant送入现有compiled节点/TLS流程，最终正常join和owner签名撤销/完整资源恢复。

没有放宽device-api-client既有“use_grant只接受空current、recheck不得换grant”的门禁。fresh客户端启动先于新邀请，以免启动失败时已产生新批准授权；正常失败仍走现有owner撤销和保守退出证据门禁。本轮不先建立旧TLS会话，因此不能称旧活TLS撤销后重连；实机自然过期未执行。

新增Go两分支回归：revoked/expired grant在Store持久化重开后保持拒绝、旧request/邀请码不能复用、fresh pending在重开后不自动批准、controller自批拒绝、fresh grant不同ID且重开后active，旧grant在线签名clock proof拒绝，新grant实际在线proof成功。时间推进仅fixture clock，不修改系统或生产时钟。测试使用memory Driver而非真实Headscale；实机节点/TLS证据待运行新入口。Python73项全部PASS（含旧在线误放行、自动grant、同ID新grant、phase失败仍撤销），invite/API全包race、vet PASS。首次sandbox HTTPS监听拒绝后获准重跑；补充positive proof时发现测试Driver未配置，修正fixture后全量通过，不变更生产逻辑。

准备脚本 `artifacts/connection-poc/prepare-reauthorize-r15.py` 实际exit0：服务器资源空/基线不变、当前9helpers构建、固定Windows产品摘要、fresh creator BUILD_READY/隔离TEST namespace、公开包逐文件摘要、waiter摘要与3个PowerShell入口解析全部PASS。仅上传公开测试包，不启动桌面identity/网络；r14已消费不能重用，不删Tailscale或旧包。

Windows普通桌面窗口1：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-desktop-reauthorize-20261011-r15\start-desktop-session.ps1"
```

READY后窗口2：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-compiled-stage-6d93c428f1a7be05\wait-for-compiled-borrower.ps1" -Root "C:\Users\jarvis\remoteapp-compiled-stage-6d93c428f1a7be05" -IdentityBootstrap "C:\Users\jarvis\windows-desktop-reauthorize-20261011-r15\interactive-bootstrap.json"
```

两窗口READY/WATCH_READY后才运行：

```sh
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='C:\Users\jarvis\windows-desktop-reauthorize-20261011-r15' REMOTEAPP_WINDOWS_DESKTOP_STAGE='C:\Users\jarvis\remoteapp-compiled-stage-6d93c428f1a7be05' python3 -B poc/selfhost/public-server/split-consent.py --compiled-reauthorize
```

协调actual exit0后先borrower再creator STOPPED，再精确核验原生测试证书/进程/bootstrap及正式基线。尚未真实双机验收，整体4未完成；不包含公网无SSH准入、自然过期实机、旧活TLS撤销重连、GUI或签名安装包。本地提交，不推送。

## 2026-10-11 r14 两桌面 STOPPED / 收尾核验 PASS，升级子切片闭环

用户确认两窗口STOPPED后，运行只读 `artifacts/connection-poc/check-r14-stopped.py`，实际exit0。精确r14 creator/borrower可执行目录下Windows子进程0，三个public bootstrap均不存在；CurrentUser/My中 `CN=Personal Remote Desktop Test ` 前缀测试证书0。远端borrower manifest摘要匹配唯一local compiled pair，该pair目录下Mac产品子进程0。Headscale nodes/users/preauthkeys全空，原policy不变，三个正式服务active，Store固定摘要及Nginx基线复查PASS。

原生journal/test identity清理证据为用户普通桌面严格closed/zero-exit STOPPED与bootstrap移除；不以SSH另一凭据上下文冒充原生存储读取验收。测试证书不存在另经只读枚举确认。本轮没有额外删除任何资源，公开包与work保留作审计，不重启已消费r14；未知历史凭据未清理或宣称归零。

结合上一节原协调器实际exit0，r14真实TLS升级/epoch2 floor/拒降级/显式新进程会话/正常join/签名撤销子切片闭环。独立native不确定回执测试也已由两平台验收，但不代表真实OS写失败或断电验证。整体4未完成，后续重点新的授权生命周期与公开接入、GUI接线、签名安装包验收。仅本地提交、不推送；下方等待桌面STOPPED为历史记录。

## 2026-10-11 r14 真实双机 TLS 升级 PASS / 实际 exit0，等待两桌面 STOPPED

用户确认 r14 两窗口就绪后，实际执行对应 fresh creator/stage 的 `split-consent.py --compiled-upgrade`，协调器实际 exit0。desktop waiter live、creator OS identity probe、当前9个helper构建、matching fresh pair构建/上传摘要、borrower real desktop probe全部PASS。OS签名注册/拒绝/审批、enrollment与单次intent、真实节点与OS持有证明、错误身份拒绝、Mac同节点重启、signed binding、正向固定payload/反向拒绝及bound helper实际join交接均PASS。

初始真实产品 TLS pongs 跨初始lease PASS；双方无效升级候选拒绝后旧TLS payload持续PASS；双方原生epoch2、epoch1降级拒绝PASS；显式新Mac进程/产品会话再次真实TLS pongs跨初始lease、两端正常stop/join PASS，输出 `compiled_live_upgrade_explicit_new_session_tls_and_normal_join`。不能把成功升级后的新runner称为故障owner复活或新授权审批；本轮不是native commit失败注入，原生不确定回执测试证据见下节。

owner OS-signed revoke、durable客户端清理、zero ACL、exact verifier retirement、隔离实验目录移除及原协调器 `STOPPED isolated_coordinator_resources_joined` / zero exit均PASS。Headscale控制资源归零，原file policy、三个正式服务、Store/Nginx基线恢复/不变，上传执行程序精确移除。Windows compiled package/work保留等待桌面最终Close，不能提前宣称borrower原生证书/journal已清理。

下一步窗口2（borrower/waiter，`C:\Users\jarvis\remoteapp-compiled-stage-b9274dc518a06e3f\run\compiled-win`）先按Enter并确认STOPPED；然后窗口1（creator，`C:\Users\jarvis\windows-desktop-upgrade-20261011-r14`）按Enter并确认STOPPED。收到两窗口STOPPED后精确核验本轮进程/bootstrap/测试证书清理与正式基线，不删除未知历史凭据，不重启已消费r14。整体4未完成，不包含真实OS写失败、GUI或签名安装包。仅本地提交，不推送；下方双机待验收为历史状态。

## 2026-10-11 r14 Windows 五项原生验收 PASS，下一步真实双机 TLS 升级

用户反馈修复后的普通桌面验证入口完整输出：TestNativeLiveOwnerRetainsLeaseOnTimeoutAndReopensEpoch、TestOwnedProductUpgradeWaitsForProductAndNetworkJoin、TestOwnedProductNonzeroExitRejectsUpgrade、TestInstalledEpochRejectsBrokenOwnerAndInvalidCandidateKeepsRunner、TestNativeUpgradeUncertainReceiptLatchesClosedAndPreservesFloor 全部 PASS，无 SKIP/FAIL；末尾返回本轮预期 VALIDATION PASSED。native durable commit + uncertain receipt latch + explicit idle reopen + persisted epoch2 floor 的 CHECK 同样 PASS。验收入口只在 actual exit0 和全部 required PASS 满足后输出该成功标记；此处证据来自用户提供的执行结果。

JOIN_STAGE 普通 stderr 日志已正常作为文本输出，不再触发 NativeCommandError；负向产品退出用例仍必须拒升级，不能仅凭某条阶段日志认定完整 join。此前捕获自测和中止运行的记录保留，不能倒推未知历史测试资源已清理。

尚未执行 r14 真实双机 TLS 升级。下一步按下节命令在正常 Windows 桌面开启 fresh r14 creator 与 waiter，分别等待 READY / WATCH_READY，暂不要按 Enter。用户确认两窗口就绪后才运行对应 `split-consent.py --compiled-upgrade`；最后 borrower 先 STOPPED、creator 后 STOPPED，再核验本轮资源与正式基线。不复用已消费 r13，不通过 SSH 冒充普通桌面身份。不包含真实 OS 写失败、GUI、签名安装包或整体切片4完成。本轮仅记录并本地提交，不推送。

## 2026-10-11 r14 PowerShell stderr 入口修复，等待普通桌面重新验收（历史，现已通过）

用户运行新native入口时报 `NativeCommandError: JOIN_STAGE product_stopping`。原脚本 `$ErrorActionPreference='Stop'` + native `2>&1` 将正常stderr阶段日志提升为PowerShell ErrorRecord，不能据此认定原生断言失败，也不能计验收通过。只读检查精确验证目录子进程0，原二进制/脚本/manifest摘要一致；没有删除未知或部分TEST凭据。

修复仅脚本：ProcessStartInfo重定向stdout/stderr并发ReadToEndAsync；实际WaitForExit，不强杀、不把cancel当join；作为普通文本打印日志，以actual exit0、5个required PASS、无FAIL/SKIP/no-tests严格验收。测试EXE及journal helper未改变。增加独立 `-TestProcessCapture` 自测，不读取journal/key，输出不同的PROCESS_CAPTURE_PASSED，不能冒充native VALIDATION。实际Windows使用原生cmd文本fixture（0与7两个退出码）验证并全部PASS；嵌套encoded PowerShell自测未通过后改用与Go日志匹配的原生文本fixture。普通直接脚本调用被execution policy拒绝已确认，最终自测使用进程级Bypass成功，未修改系统策略。

远端原目录脚本/manifest已更新并逐文件hash核验，old `.original.ps1` / `SHA256.original.json` 与中间 capture-v1 版本备份保留。本地public bundle同步新摘要。下一步用户在普通Windows桌面重新执行同一命令：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-upgrade-floor-20261011-r14\run-upgrade-floor-validation.ps1"
```

必须5个PASS、actual exit0、无SKIP，才可继续fresh两桌面真实升级；目前native Windows和双机升级仍待验收。此前中止的测试产生的未知随机命名空间不批量清理、不宣称全部清理。脚本捕获自测通过不替代原生验收。仅本地提交、不推送。

## 2026-10-11 r14 升级/floor/不确定提交回执切片开发完成，Windows与真实双机验收待执行

本轮补齐两组入口，证据分开：真实双机 TLS 升级使用 `--compiled-upgrade`；提交不确定性仅在独立 native unit executable 中模拟，不开放产品远程 fault 注入。整体4未完成。

修复 Mac installed-session 无效升级候选会终止仍有效旧会话的问题：只有 Upgrade 返回错误且安装仍可用时输出固定 `upgrade_rejected`、继续旧 runner/管道；join/commit/expiry/损坏导致不可用仍失败锁闭。Windows同样提供明确拒绝回执。新增只读 Owner.InstalledEpoch / installation_state，仅返回经 durable journal 核验的 epoch，broken/closed 不返回可用值；查询拒绝 profile/config override。relay 新回执 exact schema、正确 op、整数epoch；拒绝浮点/布尔/额外字段，不放宽退出屏障。

真实升级流程：初始 pongs 跨 lease；双方错误签名拒绝后旧 TLS payload 仍继续；Mac 有效 epoch2 升级实际 join/commit，然后 upgraded/closed + zero exit（EOF仅解锁reader，不发送stop冒充升级）；Windows有效升级、实际正常join和durable epoch2回执；Mac native inspect重开读2；双方epoch1降级拒绝、Mac再次inspect2；双方重新核验仍有效同一grant，再显式新Mac进程/新产品会话、原Windows测试证书/新HMAC、新pongs跨lease，正常join/签名撤销/完整资源及基线恢复。Windows成功升级保留健康安装owner、新runner需显式启动，不能称为故障owner复活或新授权审批。以上真实跨机流程尚待执行。

独立 native 不确定回执测试在真实原生 durable Commit 后故意返回error：正常旧runner必须先join；旧owner锁闭、拒Start/epoch查询，Close前仍持有排他租约；显式Close后新idle owner重开读epoch2并拒降级。随机TEST root/journal精确清理；不是实际OS写失败/断电验证。其他验收覆盖无效candidate保留runner、native超时保留租约、测试product先closed但未退出时不得commit、nonzero退出拒upgrade。

验证：Python全套72 PASS；clientowner/desktopcontrol/desktop-session/desktop-relay/installed-session/clientruntime六包race与相关vet PASS。Mac端运行了与Windows新入口对应的五项测试，实际全部PASS且无SKIP，含两个native测试；Windows只交叉编译，普通桌面尚未运行。r14准备实际exit0：9helpers、fresh creator BUILD_READY/test namespace、逐文件hash、waiter/3个入口解析、新native包hash与入口解析均PASS，无新身份/网络会话启动。

先请用户在 Windows 普通桌面 PowerShell 执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-upgrade-floor-20261011-r14\run-upgrade-floor-validation.ps1"
```

必须5个明确PASS、实际exit0、无SKIP。成功输出 `VALIDATION PASSED: native durable upgrade + uncertain receipt locks owner + explicit idle reopen and epoch floor + actual product fixture join. NOT cross-machine TLS upgrade, real OS write failure, GUI or signed installer acceptance.`。成功后才启动新的两桌面入口，不复用已消费r13：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-desktop-upgrade-20261011-r14\start-desktop-session.ps1"
```

窗口1 READY 后窗口2：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-compiled-stage-b9274dc518a06e3f\wait-for-compiled-borrower.ps1" -Root "C:\Users\jarvis\remoteapp-compiled-stage-b9274dc518a06e3f" -IdentityBootstrap "C:\Users\jarvis\windows-desktop-upgrade-20261011-r14\interactive-bootstrap.json"
```

确认READY/WATCH_READY后才用相应 REMOTEAPP_WINDOWS_INTERACTIVE_DIR/REMOTEAPP_WINDOWS_DESKTOP_STAGE运行 split-consent.py --compiled-upgrade，不混用fault/reconnect。最后borrower先STOPPED、creator后STOPPED，复核本轮原生证书/进程/bootstrap、节点、正式基线。不把上传或模拟升级写为真实验收。仅本地提交，不推送。

## 2026-10-11 r13 两桌面 STOPPED / 收尾核验 PASS，显式重连子切片闭环

用户确认两窗口 STOPPED 后，只读 `check-r13-stopped.py` 实际 exit0：精确 r13 creator/borrower 可执行路径的 Windows 子进程为0，两个 owner 的3个 public bootstrap 均由成功停止入口移除；CurrentUser/My 中 `CN=Personal Remote Desktop Test ` 前缀的测试证书为0。使用远端 borrower SHA256.json 匹配唯一 local compiled pair，Mac 对应产品子进程为0。Headscale nodes/users/preauthkeys 全空，原 file policy 不变，三个正式服务 active、Store 固定摘要及 Nginx 基线均再次 PASS。未删除生产身份或无法归属的历史凭据/文件。

本轮 native key 删除依据：借用桌面 STOPPED 仅在 compiled desktop 实际 zero exit + closed 后输出；backend.Close 在实际 product join 后执行精确 scope certificate/key 删除，失败不能 closed/exit0；独立只读证书枚举再次确认不存在。native journal/test identity 删除同样依据两个普通桌面严格 closed/zero-exit STOPPED 与 bootstrap 成功清理，未通过 SSH 的另一凭据上下文冒充原生保护存储清理验证。保留 public package/work 供审计；它们不是活跃凭据，不重启已消费 r13 入口。

结合上一节原协调器实际 STOPPED/exit0，本轮“同一仍有效授权的显式新 owner/process + 原证书/新 HMAC TLS 重连”已闭环。整体4仍未完成；下一项优先真实 live upgrade/floor/fault-commit（先 join 再提交、失败锁闭、显式新 owner），再收口新的授权/公开接入、GUI、签名安装包。仅记录本地提交，不推送；下面等待两 STOPPED 的描述为历史。

## 2026-10-11 r13 真实显式 TLS 重连验收 PASS / 实际 exit0；等待两桌面 STOPPED

用户确认已启动两 PowerShell 窗口后，实际执行对应 r13 的 `split-consent.py --compiled-reconnect`，进程 exit0。等待窗口 live challenge、creator native probe、matching fresh pair 构建/逐文件摘要上传、borrower real desktop probe（含随机 CurrentUser certificate create/reopen/native RSA proof/negative/delete）全部通过。OS 签名注册/拒绝/审批、免登录 enrollment/单次 intent、节点持有证明/错误签名拒绝、Mac 同节点身份重启、signed binding、正向固定 payload 与反向拒绝 PASS；旧 helper 实际 join/交接与 Mac native initialize PASS。

初始真实 TLS/pongs 持续跨初始 lease PASS；隔离 authority 暂停/HTTPS503、旧 ACL 残留时两端自主 fault completion/join/native retirement PASS；同 verifier 显式恢复、fresh signed same binding/minimum policy、旧 owner 保持 closed PASS。双方重新核验仍有效同一 grant、Mac journal inspect、新 Windows owner idle ACK PASS。随后新 target 产品成功重开原生证书，通过同 pin 检查和新 HMAC 非复用检查，启动新 Mac 进程（不同 PID，旧进程保持实际 fault exit1），真实新 TLS/pongs 再次跨初始 lease；两端正常 stop/join，`compiled_explicit_new_owner_product_reconnect_and_normal_join` PASS。未修改同 grant 禁止换 pin 门禁；不能把本轮成功倒推为 r12 泛化失败的唯一根因证明。

在线 owner OS-signed revoke、durable clients 清理、nodes/users/preauthkeys 归零、zero ACL、exact verifier ticket retirement、隔离实验目录移除均 PASS。原协调器输出 `STOPPED isolated_coordinator_resources_joined`，随后 zero exit PASS；原 file policy、三个正式服务及 Store/Nginx 基线全部恢复/不变，公共上传程序精确移除。不是独立恢复冒充原协调成功。Windows public compiled package/work 为桌面最终关闭保留，不提前删除。

当前窗口2 borrower：`C:\Users\jarvis\remoteapp-compiled-stage-a84f231c07d693e5\run\compiled-win`；窗口1 creator：`C:\Users\jarvis\windows-desktop-reconnect-20261011-r13`。请先窗口2按 Enter 并确认 STOPPED，再窗口1按 Enter 并确认 STOPPED。最终 borrower Close 才在产品实际 join 后删除本轮 scoped native certificate/key；目前不能记该项已完成。收到两窗口 STOPPED 后，再精确核验本轮进程退出、原生测试证书及 journal 清理，不检查/删除无关历史或生产身份。

本轮验证范围是同一仍有效授权的显式新 owner/process 重连，不包含新邀请/审批、expired/revoked grant 重新授权、升级/floor/fault-commit、GUI picture/input/clipboard、签名安装包或整体切片4完成。本地记录提交，不推送。以下 r13 待验收及 r12 失败记录为历史，以本节新证据为准。

## 2026-10-11 r12 精确恢复完成；r13 TEST 证书复用修复待桌面/双机验收

用户确认两窗口 STOPPED。独立恢复程序实际 exit0：精确匹配 r12 issuer/Store/verifier/backup，durable clients 和 verifier 退役、Headscale nodes/users/preauthkeys 归零、原 file policy 与三个正式服务恢复、正式 Store 和 Nginx 摘要未变；本轮 Mac 隔离 journal inspect/cleanup-test 实际通过。不把独立恢复写成原失败协调器 STOPPED/exit0；未知历史身份和证据保留，未批量删除。

实现：Windows TLSBridgeProbe 增加仅 TEST 的 CurrentUser/My 原生证书命名空间，由 compiled 随机 release channel + 已验证 grant ID 推导。首次创建拒绝既有项，重连仅重开已由本桌面 owner 持有的 scope，缺失/重复/无私钥/过期均拒绝，不自动重建或更换 pin。原 production Agent 证书逻辑与服务端同 grant 禁止换 pin 门禁未改；证书私钥不出 OS container。每次产品启动仍生成新的随机 HMAC。最终桌面关闭在产品实际 join 后精确清理本轮证书及私钥；故障退役不删除重连所需证书。

桌面 probe 新增随机原生证书 selftest：缺失拒绝、创建/重开同证书、原生 RSA 持有证明、重复创建与外 scope 拒绝、删除后重开拒绝。maintenance 需要 exact closed + actual zero exit/join；deadline 不丢失子进程 handle。启动诊断仅允许 product_certificate_open/product_tls_publish 固定阶段，不返回异常/密钥/指纹。协调器要求重连 fingerprint 不变、新 HMAC 不复用；任一失败不得启动 Mac 新产品。

Windows 独立构建目录 `C:\Users\jarvis\windows-certificate-reuse-build-20261011-r13`：公共源码上传摘要校验、65/65 protocol bridge tests、实际 .NET 单文件编译及下载摘要一致均 PASS；credential persistence 在 SSH 下仍 SKIP，不能冒充普通桌面原生验收。新公共产品 `artifacts/connection-poc/TlsBridgeProbe-win-reconnect-v2.exe`，不覆盖旧 4s。Python 全套 67 tests PASS，增加重连换指纹拒绝与 target-only reopen 检查；Go 增加 canonical scope/固定参数拒绝、固定诊断无私密错误、变更证书拒绝发布新会话密钥的回归。原生桌面与真实双机新 TLS/pongs 尚未验证。

下一步使用全新 r13 creator 与 stage（不复用 r12 已消费身份）：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-desktop-reconnect-20261011-r13\start-desktop-session.ps1"
```

窗口1 READY 后普通桌面窗口2：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-compiled-stage-a84f231c07d693e5\wait-for-compiled-borrower.ps1" -Root "C:\Users\jarvis\remoteapp-compiled-stage-a84f231c07d693e5" -IdentityBootstrap "C:\Users\jarvis\windows-desktop-reconnect-20261011-r13\interactive-bootstrap.json"
```

r13 准备实际 exit0：9 个 Linux/Windows/Darwin helpers 构建、fresh creator BUILD_READY、隔离 test namespace/minimum epoch=1、上传所有文件摘要、独立等待入口与3个 PowerShell脚本语法检查全部 PASS。未启动身份或网络。相关 clientruntime/desktop-session/desktopcontrol/clientowner/invite 五包 race 全部 PASS；新固定诊断测试先被沙箱禁止回环监听，提升权限后实际 PASS，不能把初次沙箱失败混为代码通过。

确认两窗口 READY/WATCH_READY 后，才运行相应环境变量的 split-consent.py --compiled-reconnect。需实际验收：两桌面新 certificate probe、初始真实 TLS/pongs、authority fault join/retirement、同 verifier 恢复、旧 owner 保持关闭、同授权证书重开/新 HMAC/新 Mac 进程、新 pongs 跨 lease、正常 join/签名 revoke、资源归零、原协调器 STOPPED/exit0、两桌面最终 STOPPED、原生证书精确清理。现有泛化失败不能断言唯一根因；新阶段诊断辅助定位。整体切片4、GUI和签名安装包均未完成。

## 2026-10-11 r12 relay修复实测通过，但新target产品启动失败；待桌面STOPPED和精确恢复

用户确认r12两窗口就绪后，实际split-consent.py --compiled-reconnect退出1。在线probe、空控制面、当前helper、matching fresh pair上传校验、OS签名授权/负向、节点证明/同节点重启/绑定、正向payload和反向拒绝、旧helper实际join/可信交接、初始product TLS/pongs跨lease全部PASS。旧产品fault join/native retirement、旧ACL残留、同verifier恢复/fresh签名binding、旧owners保持closed、reconnect双方授权重核验/Mac journal inspect全部PASS。

本轮reconnect_target_reopen实际completed，严格owner_reopened/started=false及新状态idle通过，确认333931e修复的relay通道在新回执后继续可用。随后target_start_send完成，在target_product_ready读取helper_unknown失败；尚未取得第二个产品HMAC/fingerprint，没有启动新Mac产品进程或验证新会话pongs。owner在线签名revoke PASS；finally再次stop_session因原relay退出报BrokenPipe，原协调未走STOPPED/exit0与最终资源回收，不能标成功。

通过独立fresh relay核验新Windows Handle实际session_state joined=true/fault=true，再原retire_fault严格ACK通过，relay实际exit0；这是已完成故障的新owner原生退役，不是复活/重试。Headscale零规则策略确认，nodes/users/preauthkeys保留3/3/3。下一步用户窗口2borrower按Enter确认STOPPED，再窗口1creator按Enter确认STOPPED；随后沿r11的精确issuer/账本/原生租约及策略备份绑定方式恢复本轮，不能硬编码重用r11恢复目录或删除未知历史资源。

代码检查发现确定的产品生命周期冲突：Windows TlsBridgeProbe Program.cs每次进程启动CreateSelfSigned产生新证书；Product.start每次publish_tls，但ProvisionBackend.publishTLS在同一grant下已有记录不相等时ErrDenied（禁止静默换pin）。已有TestTLSIdentityPossessionScopeFreshnessRestartRevocation在本轮重新race运行PASS，明确覆盖第二个有效证书被拒，不应放宽为任意覆盖。该冲突必须解决，但现泛化failed未携带实际启动阶段，尚不足断言它是本次现场唯一根因；须补受控无密钥的startup分段诊断。下一开发组先明确同一grant证书复用或显式验证轮换协议，并补负向/原生生命周期回归，之后fresh双机验收；不能简单删除旧TLSIdentity或改publishTLS允许静默换pin，也不能导出/日志记录私钥。

当前creator `C:\Users\jarvis\windows-desktop-reconnect-20261011-r12`、stage `C:\Users\jarvis\remoteapp-compiled-stage-78b659da10e34fc2` 已消费，不复用。本轮没有新增产品行为代码，记录本地提交不推送；整体4仍未完成。

## 2026-10-11 r11 独立恢复完成；r12 relay修复包已准备，等待fresh两桌面

用户确认r11两窗口STOPPED；只读Get-CimInstance按r11 creator/stage可执行路径确认子进程0，Mac本轮compiled目录无产品子进程。精确匹配本地compiled-live-25b41d45f41da489公开issuer与服务器 `/tmp/remoteapp-daemon-check-4212800235/config/profile.json`，相关隔离服务进程0、所有Requests已revoked/denied、2条Provisions仍bound；唯一策略备份 `/var/lib/remoteapp-poc/enrollment-mode-FC2hne` 包含原file模式。未依据mtime单独授权删除资源。

可信临时恢复工具只打开该精确authority Store（取得原排他租约）、重核issuer及2条记录均不再Authorized，通过原ProvisionBackend.Reconcile/Headscale Driver按durable ticket回收并确认2条cleaned，关闭Store，再verifier.Retire精确原journal（原排他租约）退役。工具实际exit0，Headscale nodes/users/preauthkeys全部为空。原enrollment-mode恢复工具摘要核验后，恢复精确备份并正常restart PoC；原file policy、三正式服务active、Store固定SHA及Nginx基线全部PASS。Mac本轮compiled trust-install inspect/cleanup-test实际PASS，清理其随机测试journal。不是原协调STOPPED/exit0的证明，也未确认所有历史Mac随机身份或客户端状态均清理，未知凭据/旧实验目录不删除。服务器本轮已终止的恢复证据及公开制品暂保留；精确策略备份由原恢复流程消费删除，不再可用于重复恢复。

随后准备全新r12：空控制资源/原策略/正式基线再次PASS，当前9helper重建PASS。fresh creator BUILD_READY/exit0，manifest/test_namespace=true/minimum_epoch=1核验PASS；从已提交333931e修复构建，公开文件仅精确摘要复用，远端逐文件SHA均PASS。fresh stage当前SID/SYSTEM ACL、waiter摘要及三PowerShell入口解析PASS。未启动新身份、节点或隔离服务。

新creator：`C:\Users\jarvis\windows-desktop-reconnect-20261011-r12`；新stage：`C:\Users\jarvis\remoteapp-compiled-stage-78b659da10e34fc2`。

窗口1普通桌面PowerShell：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-desktop-reconnect-20261011-r12\start-desktop-session.ps1"
```

窗口1READY后，另开窗口2：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-compiled-stage-78b659da10e34fc2\wait-for-compiled-borrower.ps1" -Root "C:\Users\jarvis\remoteapp-compiled-stage-78b659da10e34fc2" -IdentityBootstrap "C:\Users\jarvis\windows-desktop-reconnect-20261011-r12\interactive-bootstrap.json"
```

收到READY/WATCH_READY后才使用对应r12环境变量执行split-consent.py --compiled-reconnect。r11旧stage/bootstrap已消费不得复用；新grant/HMAC不得复用。须实际验收新owner/new Mac PID/TLS pongs、旧owner不复活、正常join/签名revoke/durable节点回收、原协调实际STOPPED/exit0及最终基线；修复上传不代表通过。最终borrower先STOPPED、creator后STOPPED。整体4仍未完成，本次记录本地提交不推送。

## 2026-10-11 r11 真实显式重连失败：relay回执遗漏已修复，现场收尾待完成

用户确认两个新窗口就绪后，实际执行r11 split-consent.py --compiled-reconnect，最终exit1。在线waiter/native probe、matching fresh pair构建上传、OS签名邀请/拒绝/审批、节点注册/错误签名拒绝/同身份重启/绑定、精确正向payload与反向拒绝、两个旧helper实际join/交接均PASS。初始真实product TLS/pongs跨初始lease、隔离authority暂停/HTTPS503且旧ACL残留、Mac实际fault join/native retirement/exit、Windowsfault join/native retirement、同verifier恢复/fresh signed same binding/旧owners仍closed全部PASS。双方reconnect授权重新核验及Mac已提交journal inspect PASS。

在reconnect_target_reopen阶段收到helper_unknown，随后协调finally的CompiledPair.close因relay实际exit非0失败，原协调未完成STOPPED/exit0。在线owner签名revoke已PASS，但未进入后续节点回收/原policy恢复/正式基线最终验收，不能算已清理或重连通过。保留Windows stage/run与公开包，未强杀或清理未知凭据。

代码定位：desktop-relay转发owner_reopened回执后，terminal status列表缺少该状态，立即退出private_protocol_denied并输出failed；协调读到首次ACK后再查session_state失败。新的只读relay连接实际确认BORROWER_IDLE_CONFIRMED=True、READONLY_RELAY_EXIT_ZERO=True，证明Windows新owner已实际成功重开idle，不是native replacement失败。尚未发起新target产品Start或新Mac产品进程，旧Mac产品已经实际故障退出。

已修复desktop-relay：reopen_fault仅接受exact owner_reopened/started=false两字段回执，保持通道用于后续state/start；其他op不能接受该ACK，非idle/错误bool/缺字段/额外字段/ready/failed均拒绝。原privatepipe仍拒重复JSON键。新增2组relay回归。desktop-relay/desktopcontrol/desktop-session/clientowner race全部PASS，relay vet PASS，Windows amd64交叉编译PASS（仅编译制品不是可信发布包），66项Python回归PASS。未覆盖运行中的旧包或部署新包，真实修复复测待fresh入口。

失败后只读核验Headscale nodes/users/preauthkeys分别3/3/3，policy为zero ACL；这不是资源归零。先用户在窗口2借用owner按Enter并确认STOPPED，再窗口1creator按Enter确认STOPPED。不要强关、不要重启旧stage。随后必须以本轮公开身份/精确命名空间绑定现场，确认所有实际worker退出、回收本轮节点/凭据/journals、恢复本轮policy备份并复核原服务/Store/Nginx基线；不删除无法确认归属的历史资源。若原协调已退出，不得把独立恢复冒充原协调成功。

当前r11 creator `C:\Users\jarvis\windows-desktop-reconnect-20261011-r11`，stage `C:\Users\jarvis\remoteapp-compiled-stage-3c881ec2b58f41a7` 已消费。本轮结果不包含新会话pongs、正常重连join、完整资源回收或桌面STOPPED；整体4未完成。本地提交、不推送。

## 2026-10-11 r11 显式重连验证入口已上传；等待两桌面就绪

用户加载服务器专用密钥后，实际运行prepare-reconnect-r11.py --upload-built退出0。Windows管理SSH、Headscale nodes/users/preauthkeys全空、原policy、三正式服务active、Store固定SHA与Nginx基线全部PASS。当前9helper跨平台重建PASS，现有本地fresh creator manifest及隔离test_namespace/minimum_epoch=1核验通过，固定Windows产品摘要通过。仅上传公开包，所有远端文件逐个摘要校验PASS；fresh stage以当前SID/SYSTEM ACL创建，waiter摘要和三个PowerShell入口解析均PASS。未启动桌面身份、网络节点或隔离故障服务，未修改SSH持久配置或系统Tailscale。

新creator：`C:\Users\jarvis\windows-desktop-reconnect-20261011-r11`；新stage：`C:\Users\jarvis\remoteapp-compiled-stage-3c881ec2b58f41a7`。不得使用旧r10等已消费入口。

普通Windows桌面窗口1：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-desktop-reconnect-20261011-r11\start-desktop-session.ps1"
```

窗口1显示READY后，另开普通桌面窗口2：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-compiled-stage-3c881ec2b58f41a7\wait-for-compiled-borrower.ps1" -Root "C:\Users\jarvis\remoteapp-compiled-stage-3c881ec2b58f41a7" -IdentityBootstrap "C:\Users\jarvis\windows-desktop-reconnect-20261011-r11\interactive-bootstrap.json"
```

等待用户确认READY/WATCH_READY，保持两窗口打开、暂不Enter。届时在线probe并使用对应r11的REMOTEAPP_WINDOWS_INTERACTIVE_DIR/REMOTEAPP_WINDOWS_DESKTOP_STAGE运行split-consent.py --compiled-reconnect；此时才生成fresh borrower并进行故障退役/恢复/新产品重连。实际验收仍待执行，不用上传成功替代TLS/pongs/旧owner锁闭/资源清理证据。最终borrower先STOPPED，creator后STOPPED。本次记录本地提交、不推送；下方SSH阻塞记录为历史。

## 2026-10-11 重启后 SSH 密钥加载诊断：Windows 已恢复，服务器待加载

只读诊断确认Windows接受原公钥；此前失败不能解释为jarvis公钥被删除。Mac重启后的agent为空，加密私钥无法在BatchMode下交互解锁，有效AddKeysToAgent=false。用户手动执行ssh-add --apple-use-keychain加载Windows专用密钥后，实际SSH_READY/exit0通过。未修改SSH配置、账户、公钥或持久化设置。

重新执行r11上传准备，windows_management_ready通过，但首次服务器admin读取失败；上传尚未开始，未创建远端creator/stage或启动身份/网络实验。独立服务器echo检查exit255；过滤-vv结果确认server_accepts_offered_key=True、authentication_succeeded=False，不输出密钥/指纹。下一步用户在Mac终端加载 `/Users/lipeng/.ssh/remoteapp_poc_server_ed25519`，再重测服务器认证和全部基线。尚不能将服务器认证失败归因于公钥不存在。

本地忽略目录准备脚本新增--upload-built，仅核验既有fresh包后使用，不覆盖重建；基线全部通过后才上传。r11仍仅本地构建通过，真实显式重连待验收。下方“Windows原公钥授权被拒绝”是此前不准确归因，以本节实际诊断为准。本次文档本地提交、不推送。

## 2026-10-11 完整显式重连开发入口完成；Windows SSH公钥拒绝，上传/实机待恢复

用户要求扩大切片后，本轮联合接入Windows新owner、Mac新进程、私有授权重核验、独立 `--compiled-reconnect` 协调入口和负向/退出屏障回归。该模式复用原隔离fault服务协议，先执行完整fault join/native retirement、同verifier恢复、旧owners保持锁闭检查，再显式调用新重连；不影响原compiled/fault/revoke模式、不混用CLI参数、不改原期限或放宽ACK/实际退出。

device-api-client新增 `recheck_grant`：只允许与已接受cfg.Grant完全一致的grant，不能初始化/替换；仍必须从服务获得fresh随机challenge绑定的grant_clock签名证明、AuthorityClock及VerifyGrant验证才更新校准时钟。另补所有network_state返回值的VerifyGrant、VerifyNetworkState、精确grant/subject及当前租约检查后才passed，不能仅凭TLS/HTTP200或active字段当签名通过。新增Go测试覆盖grant substitution、未知/重复use、状态签名、过期、错误grant/subject/issuer拒绝。

CompiledPair仅显式重连模式在私有内存保留初始可信Config副本；旧MAC实际fault exit1后保留Popen退出证据，不保留旧HMAC。重连一次性尝试（失败不可重试），先断言old fault latched，再双方实际API拒篡改grant、重新核验同一grant和fresh signed active exact binding/grant；并非重新邀请/审批，也不代表可用过期/撤销grant。Mac仅inspect既有已提交保护存储，不initialize/floor reset；Windows严格reopen_fault idle ACK且新状态idle，才显式start产品。新target HMAC的私有内存摘要必须不同，Mac新Popen PID不得等于已退出进程，原进程仍必须exit1。两端实际pongs继续跨初始lease后正常join，随后原流程在线签名revoke、durable清理和coordinator实际STOPPED/exit0。私有Config/key不落文件/日志，最后清除待重连Config。任何新产品send前先标sessions_joined=false，失败不能跳过实际join清理屏障。

66项Python全套实际PASS：双方重核验/篡改拒绝、inactive/foreign binding/grant/整数bool拒绝、单次交接与失败不重试、idle ACK、实际旧进程exit必要、send失败保持join pending、新HMAC拒复用/不启动Mac、不泄露私有输入。相关7包Go race全部PASS，device-api-client/desktop-session/clientowner/clientruntime vet PASS。当前9helper跨平台构建PASS、fresh Windows creator BUILD_READY/exit0，隔离test_namespace/min_epoch=1、本地manifest PASS。仍无新双机重连实测，不把上述模拟/编译记完整验收。

准备Windows上传时第一步SSH失败，随后只读echo诊断实际exit255：`Permission denied (publickey,password,keyboard-interactive)`。Windows服务可达但jarvis原公钥授权被拒绝；不是网络超时。未自行修复SSH配置/账户/公钥，也未启动身份、节点、隔离服务或故障实验；这次未执行服务器基线核验或远端包创建/上传。

本地fresh creator目录 `artifacts/connection-poc/windows-desktop-reconnect-20261011-r11` 已build-only完成；预定远端creator `C:\Users\jarvis\windows-desktop-reconnect-20261011-r11`，stage `C:\Users\jarvis\remoteapp-compiled-stage-3c881ec2b58f41a7`，目前未创建/上传，不能让用户先运行它们。恢复SSH后先重新只读确认空控制资源/原policy/正式服务/Store/Nginx基线，再manifest核验并上传当前fresh公开包（若源码已变则新目录重新可信build，不覆盖旧包），创建fresh stage及SID/SYSTEM ACL，核验waiter摘要和三入口解析；仅复用精确摘要匹配公开文件，不复用旧bootstrap/身份/state。

本地prepare-reconnect-r11.py的--build-only明确跳过远端检查/上传；现local out已存在，不能直接重跑其默认路径（会拒fresh_build_destination_exists），需核验现包后单独上传或新鲜目录重新构建。用户恢复原有jarvis SSH公钥授权后再继续。两桌面正常READY/WATCH_READY且在线probe后才执行REMOTEAPP_WINDOWS_INTERACTIVE_DIR/REMOTEAPP_WINDOWS_DESKTOP_STAGE对应r11的 `split-consent.py --compiled-reconnect`。最终依旧borrower先STOPPED、creator后STOPPED；r10等已消费入口不可重用。

本轮完成完整重连开发和自动回归、本地入口构建；验证包上传及真实两端验收被SSH认证阻塞，不能记该切片整体通过。不包含new invitation/approval、live upgrade、GUI/签名安装包，整体4仍未完成。本地提交不推送。

## 2026-10-11 显式重连切片B1：Windows桌面backend替换接口接入；双机重连仍待

用户继续后接入desktopcontrol同用户本地可选能力 `reopen_fault`。仅Op且Config/Profile全零才调用backend.ReopenFault；未知能力、注入Config/Profile、替换失败均返回严格failed。成功固定ACK为 `owner_reopened/started=false`，不是session ready、授权或TLS通过；不传新root/profile/grant/authkey，不启动任何worker。

Windows desktop backend增加ownerMu，串行Start/Stop/RetireFault/Upgrade/Probe/ReopenFault/最终Close。ReopenFault调用切片A的旧owner.OpenReplacement，成功后发布独立新owner并清空旧handle/产品观测，失败保留旧owner和终止遥测。Close先标closing阻止后续替换，最终退出改为关闭backend当前owner，不再只关闭初始化时捕获的旧owner；原Serve所有连接退出屏障和signer/test journal清理顺序保留。旧owner仍不能Start，重建只打开idle对象，产品Start仍走原RunnerWithProduct授权/绑定门禁。

新增4组backend内存生命周期回归：替换后idle/旧owner拒Start/旧count不可用/重复重开拒绝/最终关闭当前租约；取消失败保留旧终止状态；8并发替换仅1成功；替换与最终Close竞态不留租约、不允许closing后重开。新增control测试覆盖idle ACK、Config/Profile注入、能力缺失及替换失败。测试使用签名测试profile和内存租约模拟，不伪称Windows原生或网络证明。

最终clientowner/desktopcontrol/desktop-session/installed-session相关race回归PASS，desktopcontrol/desktop-session vet PASS，Windows amd64桌面程序交叉编译及diff检查PASS。交叉编译产物仅编译检查、没有可信compiled release，不能当可运行产品安装包；本轮没有上传/启动新桌面、节点、故障服务或真实TLS，不复用旧r10入口。

下一子切片接入Mac新进程和可信私有授权重交接，开发独立compiled重连协调模式；实际网络仍须fresh两窗口、成功fault join/retirement、同verifier恢复、重新核验有效授权与节点绑定、显式新会话TLS/pongs、旧owner拒复活及正常收尾。不能用owner_reopened或内存测试替代这些证据。整体4及整体显式重连均未完成，本轮本地提交不推送，用户暂无需Windows操作。

## 2026-10-11 显式重连切片A：退役后新owner安全重建接口；Windows桌面验证通过

用户随后提供普通Windows桌面脚本最终输出：`VALIDATION PASSED: explicit fresh owner after fault retirement + native lease and epoch floor. NOT cross-machine reconnect, fresh authorization, GUI or signed installer acceptance.`。记录为用户执行的本轮Windows桌面验证通过；脚本要求全部5项测试PASS、无SKIP/FAIL及exit0才输出该摘要。连同macOS原生测试与相关race结果，切片A安全重建基础验收收口。不把该结果倒推为先前SSH初始化/精确清理已成功，未知SSH测试状态仍不擅自删除。

下一子切片接入desktop backend显式新owner替换、Mac新进程启动与私有授权重新交接，再准备fresh双机故障恢复后真实产品重连验收。当前没有cross-machine reconnect/fresh authorization证据，整体4仍未完成；不启动旧r10窗口、不复用其已消费stage/bootstrap。本次仅记录结果并本地提交，不推送或自动开启网络实验。

用户开始下一切片后，先补clientowner.OpenReplacement(ctx)可信进程内接口。Owner保留Open时已验证的release/factory/clock；调用者不能注入新root/profile/factory。仅RetireFault实际成功设置retiredFault，pending、未退役fault、normal Close、退役journal Close失败均拒绝重建。旧owner不重置closed/broken，也不继承runner/Handle或Config/grant/authkey；新Owner是独立对象，从同一已提交原生journal重新读取，Current额外核验当前有效性/expiry/floor，失败关闭新journal，不初始化/修复/降级。接口只返回idle新owner，不自动Start；后续真实runner仍须独立核验有效授权、节点和绑定。

新增4组内存回归：退役前各状态拒绝且不开factory；新对象idle/旧对象Start继续拒绝；缺失/损坏/工厂失败拒绝且不修复；取消与过期配置拒绝。另新增TestNativeFaultReplacementPreservesFloorAndExclusiveLease：实际原生helper下pending不释放租约、故障完成和显式退役、重建新对象、第二replacement被独占租约拒绝、旧对象不可Start、持久epoch2/拒epoch1降级、显式新runner读取epoch2。此runner仅测试常量，不是网络重连或有效grant验收。

实际本地clientowner/desktopcontrol/desktop-session/installed-session相关race回归PASS（最初沙箱回环监听拒绝，放开仅回环测试后PASS）。macOS随机独立namespace原生replacement测试实际PASS/exit0、自己的测试journal清理PASS。Windows测试交叉编译与公开包逐文件摘要、脚本解析PASS，上传 `C:\Users\jarvis\windows-fault-replacement-20261011-r1`。脚本严格要求全部5测试PASS/无SKIP及exit0，恢复原进程环境变量；没有启动身份窗口/Headscale/产品TLS或故障服务。

通过管理SSH执行该Windows包时，4组基础回归实际PASS，但原生测试在owner_test.go:367 offline.Initialize失败，随后精确测试清理在:359也失败；实际exit1。没有进入replacement原生断言，不能标Windows保护存储通过，也不能只凭SSH上下文断言唯一原因。没有删除未知凭据/journal。必须在普通Windows桌面执行新脚本（不要旧r10窗口或安装包）：

`powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-fault-replacement-20261011-r1\run-fault-replacement-validation.ps1"`

预期 `VALIDATION PASSED: explicit fresh owner after fault retirement + native lease and epoch floor. NOT cross-machine reconnect, fresh authorization, GUI or signed installer acceptance.`。本轮只完成重连安全基础接口和验证入口；尚未接入desktop backend、新Mac进程重启、私有授权重交接或--compiled重连协调。收到Windows桌面结果后再开发下一子切片，不能把本轮记整体重连通过。整体4未完成，本地提交不推送。

## 2026-10-11 r10 compiled/native authority outage 实机通过；两桌面STOPPED已确认

用户随后明确反馈“两窗口已STOPPED”，记录为本轮borrower与identity creator桌面最终退出确认。未提供完整退出日志，不扩大为未知历史身份/journal均已清理。r10服务故障自主关闭/同verifier恢复验收与人工桌面退出收口完成，stage/bootstrap已消费，不再要求重开旧窗口。下一独立切片优先补服务恢复后的fresh product owner显式重连验收，仍须新owner及有效授权，禁止复活退役owner；之后推进live upgrade/floor等验收。整体4未完成。

用户确认r10两窗口就绪后，实际执行 `split-consent.py --compiled-fault`，Python实际exit0。在线等待器挑战/native creator与borrower probe、当前9helper、OS签名授权/拒自批准/单次enrollment、错误OS节点证明拒绝、同节点重启/持有证明、双方签名binding均PASS。精确本地peer route ready、单次正向固定payload、反向拒绝均PASS。

固定阶段证据实际确认bound_controller_join与bound_target_join均completed，bound_handoff、Mac native initialize、target start/product_ready/session_ready、controller start/session_ready、product_observe全部completed；随后actual product pongs跨初始lease持续增长、双方故障注入前仍live均PASS。r9失败本轮未复现；不能据此确定r9历史根因或声称间歇问题永久根治。

仅本轮隔离authority实际暂停、HTTPS503、旧精确ACL仍残留PASS。Mac实际自主fault completion/native retirement/严格终止帧与实际退出，Windows实际自主fault join/native retirement均PASS；两端join后旧ACL仍残留。显式同verifier authority恢复、最小策略恢复、双端fresh signed same binding、旧产品owners保持closed均PASS。没有重新启动旧owner；不包含新产品会话显式重连。

随后owner在线OS签名revoke、client节点回收/零ACL、client journals durable clean、daemon join、精确verifier退役、实验目录删除均PASS，实际收到 `STOPPED isolated_coordinator_resources_joined` 和coordinator zero exit。原file policy恢复、上传实验程序删除、nodes/users/preauthkeys全0、原policy、正式服务/Store/Nginx基线全部PASS。不是独立事后恢复冒充原协调成功。

本轮creator `C:\Users\jarvis\windows-desktop-phase-diag-20261010-r10`；stage `C:\Users\jarvis\remoteapp-compiled-stage-6f417f3c2e9d8a10` 已消费，公开Windows包/run按设计保留。协调结束后已要求borrower先STOPPED、creator后STOPPED；用户现已确认两窗口STOPPED。不得复用该stage/bootstrap，也不清理未知旧凭据。

本轮通过范围：compiled native双机、真实Headscale/product TLS、残留旧ACL下服务故障自主关闭/退役、同verifier恢复且旧owner锁闭。仍不包含new session reconnect、live upgrade、公开准入、GUI或可信签名安装包；整体4未完成。先完成桌面STOPPED收口，再安排下一独立验收切片。本地提交不推送。

## 2026-10-10 r10 全新阶段诊断双窗口入口准备完成

用户继续下一步后，Windows SSH可用；Headscale nodes/users/preauthkeys全0，原policy、三正式服务、Store固定SHA及Nginx基线只读核验PASS。当前9项helper全部重建PASS；全新Windows creator实际BUILD_READY/exit0、隔离test_namespace=true/minimum_epoch=1、manifest验证PASS。公开文件上传仅复用与本轮manifest精确匹配的旧公开文件，远端所有文件摘要PASS。fresh stage仅当前SID/SYSTEM ACL，waiter摘要及start/probe/waiter三个PowerShell解析PASS。58项Python回归PASS。没有启动桌面身份、网络节点、隔离服务或故障实验；不代表根因或整体验收完成。

creator：`C:\Users\jarvis\windows-desktop-phase-diag-20261010-r10`；stage：`C:\Users\jarvis\remoteapp-compiled-stage-6f417f3c2e9d8a10`。

普通Windows桌面窗口1运行creator下start-desktop-session.ps1至READY，保持打开。窗口2运行stage下wait-for-compiled-borrower.ps1，Root为该stage，IdentityBootstrap为creator下interactive-bootstrap.json，至WATCH_READY。暂不要Enter。r9及更早入口已消费，不得复用。

收到两窗口就绪后，设置REMOTEAPP_WINDOWS_INTERACTIVE_DIR为本轮creator、REMOTEAPP_WINDOWS_DESKTOP_STAGE为本轮stage，运行 `python3 poc/selfhost/public-server/split-consent.py --compiled-fault`。当前代码固定role/stage标记须用于区分bound_controller_join/bound_target_join、Mac初始化、target及controller启动和持续观察。completed标记不能替代ACK/实际退出；若产品未通过，不注入故障、不扩大期限或强杀。最终仍borrower先STOPPED、creator后STOPPED。

本轮只完成新入口准备与文档收口，本地提交不推送；整体4仍未完成。等待用户两窗口就绪后才进行真实阶段定位。

## 2026-10-10 产品交接角色/阶段诊断完成；纠正r9失败位置推断

用户继续下一步后，对照调用链发现：r9统一read报错(joined_pipe.py:112)也可能来自compiled target/controller startup，不足以证明BoundHandoff.take中的helper stop失败。原“确定旧helper退出失败、未初始化Mac journal、未启动产品”的说法缺独立阶段证据，现纠正为“路由/payload之后的产品交接链路失败，具体阶段待定位；没有产品持续pongs通过或故障注入证据”。保留原实测失败/exit1和恢复记录，不改为成功。

新增coordination_diagnostics.run_stage：只允许固定阶段枚举，只输出begin/completed/failed，不输出操作结果、错误文本、argv、路径或任何secret；异常原样传播，completed只在对应同步调用真实返回后输出，不能代替运行时ACK/退出证明。stop_bound_helpers分别记录bound_controller_join和bound_target_join；CompiledPair.run记录bound_handoff、mac_native_initialize、target_start_send/product_ready/session_ready、controller_start/session_ready、product_observe；正常停止记录两端stop阶段。失败不能推进到后续初始化/启动；key/config仍只留私有pipe，诊断不放宽故障/normal ACK、实际exit、native lease、时间预算或权限。

58项Python全套实际PASS，新增固定日志拒未知stage、不泄露返回值/错误、保留原异常对象、交接失败阻止初始化、target product_ready失败与helper交接失败区分。diff检查PASS。本轮未启动Windows窗口/新网络/服务故障。只对保留包 `artifacts/connection-poc/compiled-live-9c83059972d478f7/mac` 做manifest核验和trust-install inspect，实际非零/not usable；不能从这一结果反推r9从未初始化或作为清理成功证明，未删除或覆盖现场。

下一步准备fresh creator/stage双机阶段诊断实测；不可复用已STOPPED r9端点或已消费stage。完整故障验收仍待。整体4未完成，本地提交不推送。

## 2026-10-10 r9 路由/真实payload通过；产品交接链路失败，故障注入尚未到达

用户随后确认已STOPPED，记录为本轮两个桌面窗口最终退出确认。未附完整退出日志，不扩大为未知旧凭据/journal全部已清理；不能把本次helper交接失败或未执行的故障注入改为通过。当前无需重开Windows窗口。

用户确认r9双窗口就绪后实际运行--compiled-fault。在线挑战/native creator probe、9helper、空控制资源、隔离服务、fresh匹配包摘要/borrower实际probe、OS授权/拒自批准/单次enrollment、错误OS证明拒绝、同节点重启/真实节点持有、双端签名active binding均PASS。新增`split_controller_exact_peer_route_ready`实际PASS，随后单次正向探测为payload_verified/none、完整固定payload PASS、反向拒绝PASS。r8的路由缺失本轮未复现，可记本轮新就绪屏障/真实网络这一段通过，不能宣称间歇性问题永久根治。

随后产品交接链路出现`DIAGNOSTIC helper_unknown`，primary为joined_pipe.py:112的严格失败帧拒绝；node_phase RuntimeError，owner OS签名revoke随后实际PASS。finally尝试compiled_pair.close又遇BrokenPipeError(joined_pipe.py:79)，Python实际exit1。原推断为BoundHandoff helper退出失败已纠正：没有独立阶段日志，不能区分helper join、后续初始化或target/controller启动失败，也不能确认Mac journal初始化和产品启动是否发生。本轮没有实际持续pongs通过，freeze/recover未执行。尚缺role/精确原因，不把次要BrokenPipe掩盖原失败。没有放宽ACK/exit、重试或强杀。

独立只读审计发现本轮prefix `/tmp/remoteapp-split-consent-c8d73346cb3480f0` 的check/daemon/frontend仍存活，PoC零ACL、client节点已回收但verifier资源仍1。未在writer存活时恢复file配置或打开第二Store。向精确唯一check的现有operator FIFO发送固定stop（该fault模式尚未freeze，stop走严格失败/正常defer清理，不伪造整轮成功），检查coordinator退出；随后独立审计三隔离进程无存活、临时base `/tmp/remoteapp-daemon-check-1626306220`已不存在，nodes/users/preauthkeys全0。未捕获原coordinator STOPPED/退出码为0，不把独立审计算原自动成功。

此后才用本轮mode脚本消费精确备份 `/var/lib/remoteapp-poc/enrollment-mode-O_mzew`恢复file模式；原policy、三正式服务、Store固定SHA/Nginx基线均PASS。本地相关installed-session/enrollment-helper/device-api-client/ConnectionIdentityVault进程统计全0，但不据此证明未知旧test身份已经删除。公开管理程序/Windows包及不确定client journal/凭据保留。本轮stage `C:\Users\jarvis\remoteapp-compiled-stage-a9d60174b30f287c`已消费，creator `C:\Users\jarvis\windows-desktop-route-ready-20261010-r9`。用户已确认两个窗口STOPPED，不复用该stage/bootstrap。

下一项优先对bound helper正常退出补role/固定阶段证据，并本地排查失败帧和清理异常传播；不能盲目继续故障实验或将路由就绪当全产品通过。整体4未完成，本地提交不推送。

## 2026-10-10 路由诊断版 r9 全新双窗口入口已准备；待用户就绪

用户继续后当前9项helper（含route_status）重建PASS；Windows SSH可用，Headscale nodes/users/preauthkeys全0、原policy和正式三服务/Store固定SHA/Nginx基线只读复核PASS。全新creator build实际BUILD_READY/exit0、test_namespace=true/minimum_epoch=1和manifest确认。公开差异上传仅复用与本轮manifest完全一致的旧公开文件，远端全部文件摘要PASS。fresh stage仅当前SID/SYSTEM ACL、waiter摘要及start/probe/waiter三脚本Windows解析PASS。没有复制旧bootstrap/身份/journal、启动运行身份/节点/隔离服务或故障实验。

creator：`C:\Users\jarvis\windows-desktop-route-ready-20261010-r9`；stage：`C:\Users\jarvis\remoteapp-compiled-stage-a9d60174b30f287c`。普通Windows桌面第一窗口运行creator下start-desktop-session.ps1至READY；第二窗口运行stage下wait-for-compiled-borrower.ps1，Root=该stage，IdentityBootstrap=creator下interactive-bootstrap.json，至WATCH_READY。保持两窗口打开，暂不要Enter。

收到用户两窗口就绪后，设置REMOTEAPP_WINDOWS_INTERACTIVE_DIR=本轮creator、REMOTEAPP_WINDOWS_DESKTOP_STAGE=本轮stage，执行 `python3 poc/selfhost/public-server/split-consent.py --compiled-fault`。协调前会再次重建helper并上传最新enrollment-helper到本轮run；fresh服务匹配borrower仍在在线挑战/native probe后生成。重点先观察`split_controller_exact_peer_route_ready`，仍需唯一真实payload和产品TLS独立通过才允许注入故障；超时/状态不一致/后续payload失败均不能标成功。失败固定快照不输出原始provider数据，不放宽probe/系统fallback边界。

准备包不代表实机路由问题根治或authority-outage通过。最终仍borrower先STOPPED、creator后STOPPED，r8已消费stage/bootstrap不可复用。整体4未完成，当前仅本地提交不推送。

## 2026-10-10 overlay 客户端就绪屏障与无秘密诊断完成；尚待fresh双机实测

用户授权继续下一步后，补`enrollment-helper route_status`元数据请求，使用当前嵌入式tsnet的LocalClient.Status，单次查询750ms上限（继承主ctx取消）。只返回严格固定schema布尔：query_ok/running/peer_known/ip_matches/in_network_map/allowed_ip/in_engine/ready。预期节点公钥/IP由可信协调端已有binding输入，helper核对精确公钥、对应IP、当前netmap及精确/32 AllowedIP；不输出公钥、IP、端点、原始错误或凭据，不拨号、不签名、不改授权。in_engine仅诊断不参与ready（idle peer可能未在engine中），状态就绪不等于ACL或payload通过。

split_nodes现在与原12秒服务端projection预算共用同一deadline，完成精确policy和双端签名binding后，轮询Mac元数据直至目标peer已进入本地map/host route。没有新增12秒等待或延长原3秒probe/2秒交换deadline；过期后不得查询或接受迟到ready。拒未知字段/整数bool/ready不一致。只执行一次真实payload；失败后再取一次元数据用于固定非秘密诊断，不重试payload、不跟随Dial-Self、不采用系统Tailscale fallback、不放宽零ACL/反向拒绝/撤销门禁。补这一就绪屏障是针对已发现的客户端状态检查缺口，并不能证明r8唯一根因就是传播延迟。

实际本地Go nodeproof/enrollment-helper完整测试及race PASS；新增覆盖错误查询/公钥/IP/旧map/缺AllowedIPs/宽网段不能ready、idle engine标记不误拒。Python全54项PASS，新增元数据严格schema、只查metadata不probe、迟到响应和过期预算拒绝。最终Windows helper交叉编译PASS，diff检查PASS。没有上传新helper、启动Windows身份、注册节点、暂停服务或执行故障实验。r8 stage已消费、两窗口已STOPPED；不得复用旧bootstrap。整体4仍未完成，本地提交不推送。

下一步准备全新creator/stage，协调前重建当前helpers，再让用户READY/WATCH_READY进行fresh --compiled-fault。必须区分客户端ready、本次单payload通过、真实产品TLS与故障join的独立证据；若仍失败，用固定状态快照定位，不标记双机问题根治。

## 2026-10-10 r8 服务故障实验在注入前失败；独立恢复闭环，两桌面STOPPED已确认

用户随后确认“两窗口已STOPPED”，本轮桌面人工最终退出确认已记录。未附完整退出字符串，不扩大为旧未知身份/journal已全部清理；该确认不能把正向overlay探测失败或缺失的故障注入验收改为通过。当前无需用户重开旧窗口，下一项仍为overlay路由缺失诊断。

用户确认r8两窗口就绪后实际执行 `--compiled-fault`。在线等待器挑战/native creator probe、当前9helper、隔离服务、fresh匹配包公开摘要/borrower probe、OS签名授权/拒自批准/单次enrollment、错误OS节点证明拒绝、同节点重启/实际持有证明、双端签名active binding均PASS。随后正向固定payload探测失败：`forward_probe_stage_overlay_route_missing_reason_route_denied`；运行时精确含义为LocalAPI返回HTTP200+Dial-Self，DialOnlyOverlay拒绝系统socket fallback，未放宽边界。尚无证据确定是policy/netmap传播或其他原因，不以SSH可达替代overlay路由，不重试/扩大期限冒充原验收。

本轮**未进入 compiled pair 产品start/pongs，也未执行freeze/recover**，故authority-outage/TLS故障join没有实测证据。在线owner签名revoke实际PASS，随后finally的server资源清理缺STOPPED标记，Python实际exit1。严格保留状态；修正新模式最终成功摘要为专用authority outage范围，不能误报旧正常join模式（本轮并未走到成功摘要）。

本轮确切server实验目录 `/tmp/remoteapp-daemon-check-935587794`，管理prefix `/tmp/remoteapp-split-consent-dc5bb48d57fcec12`，备份 `/var/lib/remoteapp-poc/enrollment-mode-vLKMUN`。只读审计原check/daemon/frontend无存活，正式三服务active/Store固定SHA/Nginx基线通过，PoC零ACL但残留本轮3节点/3用户/3key，故不能直接恢复file策略。独立用原profile/Store/同verifier启动sole daemon，实际READY、全部Provisions cleaned、正常STOPPED和真实exit0；没有写假cleaned或删除锁。随后当前源码构建/上传公开recovery-verifier并SHA验证，精确Retire本轮verifier实际PASS，控制资源均0。此后才用本轮精确mode脚本消费上述备份恢复file模式，原policy/正式三服务/Store固定SHA/Nginx再次PASS。独立恢复不算原协调自动收尾或故障验收通过。

本轮实验目录、公开管理/恢复程序、Windows新包及不确定client journal/身份现场保留，不删除未知凭据。Windows stage `C:\Users\jarvis\remoteapp-compiled-stage-f83074d0c9a3158b` 已消费；creator `C:\Users\jarvis\windows-desktop-authority-fault-20261010-r8`。用户已确认两窗口STOPPED。不可复用旧stage/bootstrap。下一项优先获取“policy readback和签名binding通过，但LocalAPI无overlay route”的层级证据；不要盲目再跑故障实验。整体4未完成，本地提交不推送。

## 2026-10-10 authority outage r8 双窗口入口准备完成；待用户就绪

用户授权开始下一项后，全9项当前helper离线重建PASS，Windows SSH实际可用。全新creator可信build实际BUILD_READY/exit0，Test命名空间与minimum_epoch=1确认，本地11文件manifest校验PASS。公开文件上传仅复用与本轮manifest完全一致的旧公开二进制，不复制旧bootstrap、身份或journal。远端全部文件摘要PASS；fresh stage仅当前SID/SYSTEM ACL，waiter摘要PASS；start/probe/waiter三个PowerShell语法检查PASS。没有启动桌面身份、网络会话、Headscale节点或隔离服务，更没有暂停正式服务；不计故障实验已通过。

creator目录：`C:\Users\jarvis\windows-desktop-authority-fault-20261010-r8`；stage：`C:\Users\jarvis\remoteapp-compiled-stage-f83074d0c9a3158b`。用户普通Windows桌面第一窗口运行creator下start-desktop-session.ps1，保持READY；第二窗口运行stage下wait-for-compiled-borrower.ps1，Root为上述stage、IdentityBootstrap为creator下interactive-bootstrap.json，保持WATCH_READY。就绪前不要按Enter；不可启动旧r7或复用已消费stage。

收到两窗口就绪后，协调端设置REMOTEAPP_WINDOWS_INTERACTIVE_DIR=本轮creator、REMOTEAPP_WINDOWS_DESKTOP_STAGE=本轮stage，执行 `python3 poc/selfhost/public-server/split-consent.py --compiled-fault`（当前helpers仍在协调前重建）。实际服务故障实验由可信协调端执行，不让用户手工停止authority。最终依旧borrower先STOPPED、creator后STOPPED。新鲜匹配服务的borrower包在等待器在线挑战/native probe之后生成并发布，不把本轮creator准备算产品TLS或故障join证明。

该准备切片仅本地提交，未推送；正式服务、Store及Nginx未修改。下一轮需严格保留服务恢复而旧产品owner锁闭、新产品显式重连仍待的验收边界。

## 2026-10-10 compiled/native authority outage 验证入口开发完成；双机实测待准备

用户“继续”后按优先级开发服务故障关闭，不启动 Windows 会话，不暂停正式服务。新增唯一显式 `split-consent.py --compiled-fault`（不允许与其他模式混用），server 对应 `--os-compiled-fault`；严格要求 node/compiled/fault 三标记、完整隔离 freeze/recover 回调。旧 compiled、compiled-revoke、tls-fault 模式保持分别独立。

新入口先实际两端产品持续 pongs 跨初始 lease，并确认双方仍 pending；再冻结本轮独立 authority 子进程，复用真实内核 stopped/HTTPS503/原 ACL 不变检查。旧 ACL 必须在两端完整故障 join/原生退役前后仍残留，不能提前 stop 或签名 revoke 替代故障关闭。Mac继续严格三帧+父EOF顺序+actual exit1，Windows继续严格 Handle.Done/fault/显式 retire_fault。只有确认 join 后才显式恢复同 verifier authority，双端 API 必须返回新鲜签名的同 active binding；恢复后两次检查旧 Windows owner 故障完成保持锁闭、Mac 子进程已退出、原生journal可检查，不重新 Start。随后在线签名撤销和原资源/策略收尾照常执行。

范围：本入口验证服务恢复与旧产品 owner 不自动复活；**不包含新产品 owner 的显式重连、真实活升级、GUI、公开准入或可信签名安装包**。恢复后需要新 owner 并重新验证有效授权；不能重用退役 owner。失败时保留不确定资源证据，不强杀、伪造 join 或把 finally 的正常 stop 算故障通过。

本轮实际回归：50项 Python 全套 PASS；Linux 10 项纯协议测试实际 PASS/exit0，包括 combined 标记、回调缺失拒绝、freeze→recover→stop 严格顺序、旧路径兼容、持久清理屏障。测试程序公开上传至 `/tmp/remoteapp-compiled-fault-protocol-20261010-tests`，只执行模拟回调协议测试，未调用真实暂停/恢复或创建节点。Linux 测试程序交叉编译 PASS，diff 检查 PASS。**未执行新双机 outage，不能标记该待验收项完成。**

下一步准备全新 creator+stage（不可复用已 STOPPED 的 r7 身份/bootstrap 或已消费 stage）；复用公开二进制必须核对当前摘要，网络 helper 在协调运行前重建。用户按新路径启动正常 Windows 桌面第一窗口 READY、第二窗口 WATCH_READY 后再执行新模式。真实成功还需精确旧 ACL/503、实际双端故障 join、同 verifier 恢复且旧 owner 锁闭、在线最终撤销、durable 客户端清理、coordinator STOPPED/exit0、原正式服务/Store/Nginx/policy 基线恢复和用户 borrower→creator 顺序 STOPPED。仅本地提交，未推送。

## 2026-10-10 r7真实双机live revoke验收exit0；最终两个桌面STOPPED已确认

用户确认r7 creator READY与stage `C:\Users\jarvis\remoteapp-compiled-stage-d21bc13211ad57af` WATCH_READY后，实际执行修复后的split-consent.py --compiled-revoke。实时等待器/OS creator probe、9项helper、fresh pair全摘要、自动桌面borrower启动及实际native probe PASS；OS签名授权/拒绝/单次enrollment、错误OS身份拒绝、真实node持有证明、同节点重启、精确binding和策略readback PASS。正向诊断实际payload_verified/none、固定payload通过，反向payload仍拒绝，权限/期限未放宽。

真实Swift/C# TLS产品持续pongs跨初始lease、双端在撤销前实际pending、在线owner OS签名revoke和signed status PASS。Mac分帧标记依次fault_wait_completion→fault_completion_verified_wait_retirement→fault_retirement_verified_wait_terminal，随后compiled_controller_autonomous_fault_join_and_native_retirement PASS：严格completed+fault/retired两帧后父EOF、精确failed末帧及actual exit1全部确认，非正常closed、未发Stop或强杀。Windows实际Handle.Done+fault和稳定终态、retire_fault/native journal释放 PASS。

撤销节点回收零ACL、两个provision durable cleaned后再停止daemon、精确verifier ticket退役、隔离目录移除、STOPPED isolated_coordinator_resources_joined、coordinator实际exit0全部PASS。Mac本轮compiled journal inspect/cleanup-test和随机Mac test signer清理顺序正常完成；公开Windows包/未知旧证据保留。原file policy恢复、本轮公开上传程序移除、控制面nodes/users/preauthkeys均0、原公开policy、三正式服务/Store固定SHA/Nginx基线全部PASS；不需要本轮独立恢复来代替自动收尾。Python协调器实际exit0并输出VALIDATION PASSED。

用户随后确认两个窗口均已STOPPED，记录本轮Windows桌面人工最终退出确认；此次未附完整提示，不扩大为旧未知凭据清理全部通过。stage不复用。**4X修复后的真实Headscale/product TLS+活会话签名撤销+自主故障join/native退役和本轮桌面退出已验收通过，但整体4未完成。** 不得把r5/r6失败或本地实验算成功基线。仅本地提交，不推送。

切片4后续按以下顺序收口（不是全部已有可运行入口）：1）将authority-outage/旧ACL仍残留时的拒绝、TLS断流、完整join及显式同身份恢复接到最新compiled/native双机链路，旧fixture故障证据不能替代；2）真实活会话升级、epoch floor/拒降级、提交失败/超时后锁闭与明确恢复、升级后重启，不能自动恢复旧权限；3）正式公网接入与peer/grant/TLS绑定的秘密安全投递，替换受信SSH/脚本私有HMAC测试分发，补公网准入与防滥用；4）把新网络/原生存储/session owner接到实际RemoteController/RemoteAgent GUI，真实画面/键鼠/剪贴板、连接/断线/撤销可见状态与重连授权；5）正式签名/可信发行安装包，信任初始化/升级保留身份、Keychain升级授权、无开发环境且无需另装/登录Tailscale的双机安装验收。自然到期、网络/服务重启、睡眠唤醒及硬崩溃/损坏恢复按风险补最终异常矩阵，不以一次live revoke替代。近期优先1→2，其余含产品化开发，不能称只剩人工点验。

## 2026-10-10 故障管道EOF顺序修复完成；r7入口待真实双机验收

用户明确授权实施修复。JoinedPipe.join_fault现顺序为：严格completion(joined/fault真布尔)→严格fault_retired→关闭本父进程stdin写端→精确failed末帧→actual wait/exit1→fault_joined。EOF不发送Stop、不取消pending runner、不替代完成/原生journal释放或末帧；只在前两帧已确认后让继承命令reader退出。20/12秒期限和故障语义不变，没有强杀。补verified_fault_receipt精确schema和实际布尔类型，避免Python dict equality将整数1当True。只修受信协调父端协议，不宣称通用所有宿主stdin问题或GUI产品接入已完成。

44项Python回归实际PASS：收到完整两帧后EOF才输出terminal的真实子进程通过且未发stop；pending/数值布尔/额外字段/错误retirement不提前EOF；EOF后缺末帧、错误末帧/额外字段/exit0不能fault_joined。用上一轮复现问题的真实Go测试子进程，修复后的Python父端连续3次完成严格三帧和actual exit1（不是Python模拟子进程替代），无网络/凭据/强杀。diff检查PASS。installed-session子进程reader行为未改，原诊断实验仍用于证明该继承描述符问题。

当前9项helper重建；Windows和server SSH检查实际PASS，Headscale资源全0。新creator `C:\Users\jarvis\windows-desktop-session-eof-fix-20261010-r7` 可信构建实际exit0、公开差异上传全manifest摘要及start/probe Windows解析PASS；新stage `C:\Users\jarvis\remoteapp-compiled-stage-d21bc13211ad57af` 本用户/SYSTEM ACL，等待脚本摘要/解析PASS。未复制旧bootstrap/身份/journal、未启动新服务/节点/grant，本轮局部修复不计4X通过。

下一步第一窗口运行r7\start-desktop-session.ps1至READY，第二窗口运行新stage\wait-for-compiled-borrower.ps1，Root同stage、IdentityBootstrap=r7\interactive-bootstrap.json至WATCH_READY。用户确认后使用REMOTEAPP_WINDOWS_INTERACTIVE_DIR=r7、REMOTEAPP_WINDOWS_DESKTOP_STAGE=d21bc新stage执行split-consent.py --compiled-revoke。分帧诊断必须显示实际完整协议和exit1，再确认Windows退役/节点回收/正常server恢复；最终仍borrower先STOPPED、creator后STOPPED。整体4仍未完成；本地提交，不推送。

## 2026-10-10 本地复现继承stdin收尾互等；仅诊断，尚未修改收尾行为

用户要求继续排查。对照当前installed-session.run defer `cancel(); os.Stdin.Close(); <-readerDone` 与JoinedPipe.join_fault的顺序：父进程在收到completion/retirement/failed三帧后才关闭stdin写端。Go本地工具链os/file_unix.go表明阻塞继承fd通过kindNewFile创建，不自动成为pollable；关闭这样的文件不能当作等待中Read已退出的证明。

新增无网络/无journal/无signer的独立测试子进程，保持与现有stdin close/join同样顺序：先输出completion+retirement，继承stdin的reader等待父EOF，第三帧被close/join阻塞；父写端保持打开300ms确认无第三帧，然后只关闭实验父写端，立即获得failed第三帧并实际exit1，无强杀。Mac实际复现，3次Go race重复通过。它证明该收尾顺序存在互等，并与实机超时后父stop/EOF才收到failed吻合；仍不是原实机全部帧的直接证据，不能宣称唯一根因已实机确认。

纠正旧记录：joined_pipe.py统一read超时行号不能区分第1/2/3帧，原“首帧20秒超时”没有分帧证据，应称故障协议帧等待超时。新增固定非秘密fault_wait_completion、fault_completion_verified_wait_retirement、fault_retirement_verified_wait_terminal诊断，原严格frame schema/20与12秒期限/真实exit1要求不变。41项Python回归、Windows installed-session vet与diff检查PASS；本轮未启动server/Windows/节点/产品，未改生产reader、父EOF或native journal行为，不发布新验证包。

建议下一步修复：仅在验证真实joined+fault和fault_retired两帧后关闭父stdin写端，再等待精确failed末帧和actual exit1。此时实际runner已join且native journal明确退役，EOF只结束命令reader，不代替Stop/worker join，不允许缺前两帧提前EOF或弱化三帧/非零退出要求。需补“末帧等待父EOF”的真实子进程正向回归及缺/伪造前置帧不关闭写端负向测试，再做fresh双机验收。当前只完成诊断；整体4仍未完成。本地提交，不推送。

## 2026-10-10 r6正向payload/TLS/签名撤销通过；自主回报超时，最后阶段signer_closed

用户确认r6 creator READY与stage `C:\Users\jarvis\remoteapp-compiled-stage-ec429e80730b06e7` WATCH_READY后实际执行--compiled-revoke。实时等待器/native probe、当前helper/fresh pair摘要/borrower真实probe、OS授权/单次enrollment/错误身份拒绝/真实节点持有证明/同节点重启/精确binding PASS。正向诊断实际 `forward_probe_stage_payload_verified_reason_none`，完整固定payload通过；反向payload拒绝通过。没有扩大期限、重试或放宽ACL。r5正向失败本轮未复现，不据此证明间歇性问题根治。

实际产品pongs跨初始lease、双端会话仍pending、在线owner OS签名revoke、signed status及无离线替代检查全部PASS。随后Mac join_fault故障协议帧等待超时（没有分帧证据，不能确定是首帧），最后固定阶段 `signer_closed`；primary joined_pipe.py:83，finally正常stop又遇failed帧(line104)，Python实际exit1。此阶段仅说明对应signer关闭屏障已返回，不能代替Handle.Done/故障三帧/actual exit1证据，不能仅凭阶段断定所有退出协议成功或唯一根因。尚需检查installed-session完成状态输出/command reader及journal退役顺序；未修改这些实现，不能称4X通过。

本轮Mac compiled目录 `artifacts/connection-poc/compiled-live-4bc14bf48365e99b/mac`。原finally在compiled.close处失败，没有原协调器STOPPED/正常恢复完整证据。独立审计：本輪prefix `/tmp/remoteapp-split-consent-3a68caf4d1c48bd3` 下server进程0、Headscale nodes/users/preauthkeys均0、当时零ACL；本地本轮installed-session/管理子进程/ConnectionIdentityVault均无存活。新的诊断relay核对r6公开身份与borrower bootstrap，实际Windows state=(joined=true,fault=true)，明确retire_fault获准确ACK，relay实际exit0。Mac manifest校验后trust-install inspect实际usable/epoch1，证明native journal可重开，不算原自动回报通过。

在上述资源和原生租约审计后，用本轮精确mode脚本restore `/var/lib/remoteapp-poc/enrollment-mode-W1kz0U`，实际file_mode_restored，备份由脚本消费；原公开单向policy、三正式服务active、Store固定SHA及Nginx基线PASS。本轮公开程序/Windows包/client journal和未知test身份均保留，不删除未知凭据，也不把无marker的原清理计为通过。

用户提供完整退出日志：第二窗口包括watch/product/workers/node/signer各阶段及 `STOPPED: borrowed desktop owner joined and its isolated test journal cleaned. Identity creator remains READY; not deleted.`；第一窗口实际 `STOPPED: desktop workers and native signer joined; isolated test journal and random test identity cleaned.`。本轮Windows桌面/测试journal及creator随机身份人工清理确认闭环，不能据此将Mac故障三帧超时或旧未知凭据审计改为通过。新stage不可复用。

用户补充Mac曾息屏。只读pmset电源日志：最近整机Sleep为09:24:11、Wake为09:30:08；r6公开build文件时间10:24:15、本轮结果记录提交10:37:56，随后Display off首条为10:40:18。该区间未见整机Sleep/Wake，现有时间证据不支持把本轮超时归因于整机睡眠；息屏与系统睡眠须区分，没有修改电源设置或关闭安全锁屏。仍需本地协议证据才能确定根因，后续可在保持清醒条件下排除变量。

下一步优先收口“runtime signer_closed之后为何无完成帧”的本地协议证据/测试，不盲目扩大20秒预算或再次无诊断live revoke。整体4仍未完成。本轮仅记录并本地提交，不推送。

## 2026-10-10 正向probe固定阶段诊断完成；r6与新等待入口待桌面就绪

用户要求继续并说明网络/SSH已通。实际通过Windows SSH公开响应及服务器管理调用，Headscale nodes/users/preauthkeys全0；管理通道可达不替代隔离tsnet正向传输验收。未复用已STOPPED的r5 bootstrap或已消费stage。

enrollment-helper的probe新增固定stage：local_client_failed、overlay_dial_failed、overlay_route_missing、deadline_failed、write_failed、read_failed、payload_mismatch、payload_verified；reason仍为固定枚举，成功改为none。保留3秒probe和2秒读写deadline，拒短写及非精确payload，不输出token、payload、IP或原始网络错误。split_nodes正向检查输出白名单阶段/原因，不认识或类型不符只打印unknown；实际通过要求严格四字段schema、passed真布尔、payload_verified+none。诊断不提供重试/延长等待或管理Tailscale/系统socket fallback，负向零规则/错误身份/反向拒绝检查不放宽。

41项Python回归通过，enrollment-helper/nodeproof Go race通过，两包Windows vet退出0，diff检查PASS；Go补精确payload/短写/deadline/写失败/读失败/内容不符测试，Python补类型/schema/未知字段和诊断不泄露测试。当前9项helper重新构建。新creator `C:\Users\jarvis\windows-desktop-session-probe-diag-20261010-r6` 构建实际exit0，公开差异上传后全部manifest摘要与start/probe脚本Windows解析PASS；新stage `C:\Users\jarvis\remoteapp-compiled-stage-ec429e80730b06e7` 本用户/SYSTEM ACL创建，等待脚本摘要/解析PASS。未上传私钥/状态，未启动新桌面/节点/grant或更改正式服务，不能称网络问题已修复或4X通过。

下一步第一窗口运行r6\start-desktop-session.ps1至READY；第二窗口运行新stage\wait-for-compiled-borrower.ps1，Root为新stage、IdentityBootstrap=r6\interactive-bootstrap.json，至WATCH_READY后用户确认。随后REMOTEAPP_WINDOWS_INTERACTIVE_DIR=r6、REMOTEAPP_WINDOWS_DESKTOP_STAGE=ec429e新stage，以split-consent.py --compiled-revoke重新验收；若正向再次失败，依据固定stage定位，不盲目反复重跑。整体4仍未完成；只本地提交，不推送。

## 2026-10-10 r5复测在正向网络payload前置项失败；独立回收与原策略恢复完成

用户确认两个新窗口READY/WATCH_READY后，实际执行r5 + stage `C:\Users\jarvis\remoteapp-compiled-stage-879ec86330cbf6ad` 的--compiled-revoke。实时nonce、creator native probe、9项helper、fresh pair全摘要、等待器自动启动borrower及真实borrower probe全部PASS；OS签名注册/拒绝/批准、单次enrollment、真实持有证明/错误身份拒绝、同节点重启、精确单向策略readback和双端signed binding均PASS。

最后通过项为两次split_signed_binding_matches_actual_pair，随后split_nodes.run中的split_forward_actual_fixed_payload_allowed检查失败（t.check统一抛异常位置cross-network.py:35）；未保留probe具体reason，不推断是唯一ACL/DERP/传播问题。没有进入CompiledPair.run，未启动本轮产品TLS或自主退出故障验证，不属于上次Mac20秒join超时复现。owner实际签名revoke已返回成功；finally borrower空会话关闭/relay退出完成并保留公开包，但coordinator没有STOPPED完整marker，保留 `/tmp/remoteapp-daemon-check-1825219874` 和备份 `/var/lib/remoteapp-poc/enrollment-mode-VKmQpQ`，Python实际exit1。不可称4X通过或原自动清理成功。

独立只读审计：本轮server prefix下进程0、本轮本地installed-session/ConnectionIdentityVault无存活进程；控制平面最初nodes/users/preauthkeys=3/3/3、零ACL，原正式Store SHA/Nginx及三服务active基线PASS。隔离journal两个provision仍bound。仅以本轮原profile/anchor/network配置重新启动sole authority supervisor，实际READY，SIGTERM正常stop并实际STOPPED/exit0；临时检查脚本误读小写Version/Provisions，因此打印pending，随后的独立大小写正确读取确认两条均cleaned，控制资源变为1/1/1（只剩verifier），没有把错误检查伪改成成功或重跑节点。

当前源码重建verifier-owner-linux、上传本轮唯一recovery-verifier并SHA确认；精确 `retire /tmp/remoteapp-daemon-check-1825219874/verifier` 实际RETIRED/exit0，nodes/users/preauthkeys全部0。此后才用本轮mode脚本restore精确备份，实际file_mode_restored；备份由脚本消费，原公开策略 `100.120.0.2 -> 100.120.0.1:47476`、三正式服务/Store固定SHA/Nginx基线再次PASS。现场目录和公开程序保留，未删除未知身份或client journal，独立恢复不计本轮自动验收通过。

用户随后确认两个窗口均已STOPPED，本轮桌面人工退出确认已记录；未提供完整退出字符串，不扩展为旧未知身份/journal清理审计全部通过。本stage已消费不可重用，r5旧bootstrap不再作为在线身份入口。在再次live revoke之前，先给正向probe补固定非秘密reason/阶段证据，区分网络建立与policy readback，不盲目延时或降低负向拒绝要求。4X和整体4仍未完成。本轮仅记录并本地提交，不推送。

## 2026-10-10 两窗口用户报告STOPPED；分层退出诊断与全新复测入口

用户报告上轮两个窗口均Stopped，要求重新验证。未提供本次完整退出字符串，不据此推断旧未知凭据全部清理；此前未完成的Mac随机test身份清理审计仍保留。旧r4 creator/bootstrap和已消费的stage不复用。

新增jointrace仅向stderr输出固定白名单阶段：product_stopping/product_joined、watch_stopping/watch_joined、workers_joined、node_closing/node_closed、signer_closing/signer_closed。只有既有实际join返回后输出对应完成阶段；错误语义/超时/授权/租约屏障不变。Python并行排空stderr，只保留最多16个白名单阶段，超长行/未知文本不输出；协议stdout和故障三帧校验独立，超时仅报告最后已见阶段。诊断不是完成收据或成功退出证据，不扩20秒等待。

40项Python回归、6包Go race与Windows相关vet退出0，diff检查通过。当前9项helper重建，全新creator本地构建退出0。新入口为 `C:\Users\jarvis\windows-desktop-session-live-revoke-20261010-r5\start-desktop-session.ps1`；等待目录为 `C:\Users\jarvis\remoteapp-compiled-stage-879ec86330cbf6ad`（本用户/SYSTEM受限ACL），等待脚本摘要与Windows解析通过。creator公开文件差异上传后全部摘要核验与start/probe两脚本解析PASS；没有复制身份/bootstrap/journal。尚未启动新桌面/服务/节点/grant，不算4X实际验收。

下一步第一窗口先运行新creator至READY；第二窗口运行新目录wait-for-compiled-borrower.ps1，Root为上述stage、IdentityBootstrap为r5\interactive-bootstrap.json，至WATCH_READY。用户确认两者就绪后，以REMOTEAPP_WINDOWS_INTERACTIVE_DIR=r5、REMOTEAPP_WINDOWS_DESKTOP_STAGE=上述stage运行split-consent.py --compiled-revoke。失败保留实际阶段/资源证据，不以强杀、延时或事后退役算自主退出通过；最终仍先borrower Enter后creator Enter。仅本地提交，不推送。

## 2026-10-09 4X预启动/真实live revoke通过前半段；自主join等待超时，独立恢复不计验收成功

用户预先WATCH_READY后实际执行r4 creator + stage `C:\Users\jarvis\remoteapp-compiled-stage-4f7e7f183e4e7a28` 的--compiled-revoke。随机挑战证明等待器当前在线、真实native creator probe、所有helper构建/隔离服务/提前helper传输、fresh pair全摘要PASS；等待窗口自动启动borrower，真实borrower probe PASS，不再需要服务计时内人工启动。Windows run固定为stage\\run；Mac本轮compiled目录 `artifacts/connection-poc/compiled-live-a52b5f6632adfaf7/mac`。

OS注册/拒绝/批准及验签、双端离线intent/单次enrollment、真实节点/OS持有证明、错误身份拒绝、同节点重启、精确binding/最小单向ACL/反向拒绝PASS。真实产品持续pongs跨初始leasePASS；双端Handle仍pending确认后在线OS签名撤销PASS，状态确认撤销PASS。随后controller.join_fault等待首个完成帧20秒超时（joined_pipe.py:51）；finally正常stop补清理又遇failed帧（line72），Pythonexit1。没有完整故障三帧+actual exit1证据，不能将非零退出/事后恢复算4X通过，不能归因于唯一具体worker或简单扩时重试。

原finally在compiled owner join未确认处停止了后续清理，没有原coordinator STOPPED marker/策略恢复/身份删除完整证据。独立只读检查：无本轮server check/daemon/frontend进程、无本轮本地installed-session或SSH转发；Headscale nodes/users/preauthkeys均0，公开策略当时零ACL。Windows保留creator和borrower进程。使用新只读relay实际观察borrower session_state(joined=true,fault=true)，再明确retire_fault获fault_retired，诊断relay真实exit0；Mac本轮trust-install inspect实际usable/epoch1，证明journal可重开。它们是独立恢复证据，不是原自动join完成。

在上述资源为空/进程不存活/双端原生租约可释放证据成立后，使用本轮精确 `/tmp/remoteapp-split-consent-f514ed1c57c8efd5-mode.py restore /var/lib/remoteapp-poc/enrollment-mode-BkJ9Lg`，实际exit0/file_mode_restored；恢复脚本消费并移除该备份（原配置已复制恢复），没有删除未知client状态。随后公开原策略 `100.120.0.2 -> 100.120.0.1:47476`、三正式服务active、Store固定SHA与Nginx基线检查PASS。四个本轮公开server程序保留，未用无来源marker宣称正常daemon join。Mac随机test身份删除未走完整finally，清理审计未确认；本轮client目录/compiled journal/未知随机身份均保留，不批量删除凭据。

下一步先让第二窗口Enter并确认borrower STOPPED，再第一窗口Enter确认creator STOPPED（故障owner已通过明确retire_fault释放journal）；尚待用户最终提示。后续优先给Mac runner产品/watch/node/signer各层加入固定非秘密join阶段诊断，定位20秒无完成帧及finally失败，严格区分产品已停止、实际进程Wait、原生lease释放；不要盲目继续故障503或扩大时间/降低三帧门槛。预启动桌面机制本轮实际闭环，4X活会话撤销完整验收仍未完成，整体4未完成。仅本地记录提交，不推送。

## 2026-10-09 本轮borrower STOPPED；预启动等待窗口已准备，尚待WATCH_READY

用户确认333a86第二窗口完整STOPPED：borrowed desktop owner joined、isolated test journal cleaned、identity creator remains READY/not deleted。r4第一窗口继续READY。用户另说明已打开新窗口；尚未提供新等待脚本WATCH_READY，不当作它已启动。

收口流程新增开发测试专用wait-for-compiled-borrower.ps1和DesktopStage：先在正常Windows桌面启动等待脚本，最多等待30分钟；协调器在服务器/节点/grant变更前发新的公开随机nonce挑战并核对waiting回应，随后仍执行真实native creator probe。预启动就绪后才启动临时服务并构建/上传匹配fresh pair；所有文件摘要成功后，以固定stage\\run\\compiled-win路径和SHA256.json自身摘要发布一次性launch请求，桌面等待窗口重新验证严格12文件清单/摘要/拒reparse，再在自身正常桌面上下文启动既有borrower脚本。用户不再在服务倒计时内手动输入启动命令；原600秒就绪预算、900秒服务期限、profile/grant有效期不变。最终仍需用户Enter正常STOPPED。

管理文件仅含nonce/公开摘要，不提供runtime CA/root替换或签名/授权接口；这是受信SSH管理和同用户开发验收，不是生产发行信任。manifest只校验传输完整性，不能单独证明发行者。固定路径拒注入/旧run复用，nonce不符/未运行等待器不能启动服务；只有实际borrower bootstrap/probe通过才接OS consent。旧手工流程保持兼容；新方式用REMOTEAPP_WINDOWS_DESKTOP_STAGE环境变量显式选择。

39项Python回归及diff检查通过。新目录 `C:\Users\jarvis\remoteapp-compiled-stage-4f7e7f183e4e7a28` 已创建受限本用户/SYSTEM ACL，等待脚本上传后SHA核验和实际PowerShell解析PASS；未启动等待脚本/服务/桌面borrower/节点/grant，未读取任何私钥。本轮只有语法/编排局部验证，自动桌面启动与4X真实live revoke尚待。

下一步用户在新第二窗口运行该目录wait-for-compiled-borrower.ps1，Root同目录、IdentityBootstrap为r4\\interactive-bootstrap.json，返回WATCH_READY后再启动：REMOTEAPP_WINDOWS_INTERACTIVE_DIR=r4、REMOTEAPP_WINDOWS_DESKTOP_STAGE=上方stage目录、split-consent.py --compiled-revoke。自动启动后不需用户再次READY消息才能推进；两个窗口暂不Enter。失败保留stage与未知状态，不复用launch请求。仅本地提交，不推送。

## 2026-10-09 4X本轮就绪期限耗尽：未进入活会话验收

用户确认r4 creator READY；实际原生身份/独占journal probe、当前9项helper构建、隔离空控制平面、独立daemon/frontend、节点helper提前上传/摘要核验、匹配fresh pair构建、Windows全文件摘要全部PASS。第二窗口目标 `C:\Users\jarvis\remoteapp-split-consent-333a86e747fa214b\compiled-win`。随后绝对readiness预算耗尽，compiled_products.py:196 borrower_ready_deadline，协调exit1；用户随后回复第二窗口已就绪。没有borrower probe/注册/consent/grant/enrollment/产品TLS或live revoke通过结果，不能套用此前4W通过。

实际日志确认verifier retire、隔离目录移除、STOPPED资源join、原策略恢复和上传server程序移除；Windows公开work/客户端journal证据保留。不复用已回收端点/过期包。下一步先让本轮第二窗口Enter并确认STOPPED，r4 creator可保持READY。本轮borrower清理尚待用户确认。不得立即盲目重跑：人工桌面就绪与临时服务/配置倒计时的耦合需要先收口；单纯再次延长授权或忽略到期检查不是修复。4X真实活会话撤销仍待，整体4未完成。按既有授权本地提交，不推送。

## 2026-10-09 4X-2 compiled活会话撤销编排接线，新creator r4待READY

新增显式 `split-consent.py --compiled-revoke`，保持既有--compiled基线模式不变，也不把此模式当authority fault。新模式在真实产品持续pongs跨初始lease后确认双端Handle仍pending，保持产品活跃返回consent；在线OS签名撤销后才观测自主结束，不预发stop/关闭控制管道。Mac必须报告精确session_state(joined=true,fault=true)、fault_retired、failed三帧并真实exit1，缺帧/exit0/未知字段拒绝；该expected-fault join不设置closed ACK、不等于normal成功，无force kill。Windows等待真实Handle.Done且fault、稳定快照，再通过受保护同用户pipe请求retire_fault，原生journal.Close成功才确认；未结束/无会话/正常结束/重复退役拒绝，Start/Upgrade仍锁闭。Mac inspect确认租约可重开后才标两端join完成；后续节点回收/服务退出/策略恢复与桌面清理仍需实际验收。

Owner.RetireFault是明确受信维护操作，不取消pending工作、不修复状态、不清除故障使owner可用；journal释放失败不能确认退役。Windows桌面launcher最终Close在明确退役后可正常join signer并清理本轮test journal，但不抹掉协调端已记录的故障结束。操作/范围更新INSTALLED_PRODUCT_OWNER.md。

相关7包Mac Go race通过、最新Windows同包vet退出0；补测Owner pending/正常/缺会话/重复/原生journal Close失败拒绝及不可重启、管道不同ACK/注入配置拒绝。36项Python回归PASS，含真实子进程三帧+exit1、缺退役/exit0/额外字段拒绝、撤销前不能Stop、未确认退役不清理、显式模式检查；diff检查PASS。**这些是实现/局部验证，不是活会话撤销或authority故障双机通过，整体4未完成。**

当前源码9项helper实际重建。准备creator时临时检查误用另一构建器的BUILD_READY冒号标记，r3本地目录保留未上传/未运行；随后在独立r4目录重新构建，正确BUILD_READY与实际exit0确认，全文件manifest核验并用公开差异复用上传。Windows新目录 `C:\Users\jarvis\windows-desktop-session-live-revoke-20261009-r4`：全部文件摘要PASS、start/probe两公开PowerShell脚本解析PASS（首次手工SSH诊断引用错误未执行脚本，改用编码命令核对成功）。未读取发布root/CA私钥、未启动服务器/桌面或创建节点/grant。旧r2已STOPPED不可用；r4尚待用户正常桌面启动start-desktop-session.ps1/READY。

下一步：收到r4 READY后先真实probe，然后用r4环境启动新的--compiled-revoke隔离服务，提供本轮fresh borrower第二窗口命令；只有实际输出才能记通过。结束仍borrower先STOPPED、creator后STOPPED。若fault退役/进程join未确认，保留journal/签名器/隧道/策略证据，不强杀或伪造正常cleanup。仅本地提交，不推送。

## 2026-10-09 4W桌面清理闭环；4X-1故障join观测入口开发完成

用户确认65abe99f本轮第二窗口 `STOPPED: borrowed desktop owner joined and its isolated test journal cleaned. Identity creator remains READY; not deleted.`，随后第一窗口 `STOPPED: desktop workers and native signer joined; isolated test journal and random test identity cleaned.`。记录本轮borrower/creator按顺序正常join及随机test身份清理闭环；旧r1/4T审计不与此混同。r2现在已停止，不可继续使用旧bootstrap发起协调。

下个切片4X分段推进：第一段新增clientowner.Handle.State只读完成快照，状态严格仅status/joined/fault，Done未闭合不能称join，即便已有内部err也不提前报告fault；fault=true要求实际完成，不暴露错误详情或授权。Windows桌面backend保留当前Handle，以同用户本地pipe的session_state请求查询；拒带Config/Profile，旧不支持backend失败关闭。Mac installed-session可显式查询，且自主Handle.Done后在保留原非零失败返回前报告实际完成快照。未改Stop/Close租约屏障、重连或故障锁闭，不将此快照当closed ACK、正常exit或TLS断流证据。Python增加精确schema检查，未启用活会话验收模式。

Mac相关5包Go race通过，Windows同包vet退出0，Python31项回归通过，diff检查通过。新增pending/正常/fault完成、只读请求不Stop、缺能力/注入配置拒绝、字段/布尔类型检查。**这是4X-1观测基础实现，不是compiled活会话撤销或故障实机验收通过；整体4未完成。** 本轮未启动服务、Windows桌面、新节点或授权，没有新验证包可让用户运行。

下一段需串接--compiled活会话撤销：保持两端实际pongs增长时撤销，观测自主join，严格区分预期fault terminal和正常closed/exit0，实际Wait进程并验证target原生owner/租约回收；故障done不能豁免正常cleanup或强杀。随后接authority freeze/HTTPS503、旧ACL保留时关闭及显式恢复不复活旧owner。需要当前source全新creator/borrower包及双机原生验收，不能拿旧r2/4T执行本轮。按既有授权本地提交，不推送。

## 2026-10-09 4W fresh compiled双机联合验收通过，桌面最终STOPPED待确认

实际执行修正后的split-consent.py --compiled，Windows work `C:\Users\jarvis\remoteapp-split-consent-65abe99f91398019`，协调exit0。节点helper在fresh borrower启动/授权前已压缩上传并摘要核验。fresh包全清单摘要、borrower真实桌面probe、双OS注册/拒绝无grant/明确批准/双端验签通过；双端注册前节点公钥及OS签名intent、单次HTTPS enrollment、实际节点key与准备值一致、未绑定零规则拒绝、Windows真实节点/OS持有证明、错误Mac身份拒绝/真实未签名挑战、Mac同节点重启及正确身份绑定均PASS。

实际签名binding与双机节点/IP一致；最小单向ACL允许正向合成payload、反向拒绝。compiled pair接收bound handoff，真实产品TLS pongs持续增长并跨初始lease自主刷新；产品/node/watch/signer正常join后才进行OS签名撤销，在线状态确认撤销、客户端节点回收/零ACL、客户端journal持久清理先于daemon停止全部PASS。实际verifier retire、隔离目录移除、STOPPED资源join、原策略恢复、上传server程序移除、control resources zero、线上services/Store/Nginx不变均PASS。Windows公开compiled包/work保留，不复用其已回收端点。

准确范围：compiled native pair + real Headscale binding + product TLS payload/autonomous refresh + normal join + post-stop signed revoke通过。**NOT live-session revocation、upgrade/fault、GUI或signed installer acceptance；整体切片4尚未完成。** 桌面borrower和r2 creator最终STOPPED仍待用户确认：必须第二窗口borrower先Enter/STOPPED，再第一窗口creator Enter/STOPPED。旧r1/4T清理审计独立保留，不能据本轮通过认定已删除。完成此正常双机基线后，下一切片优先compiled活会话撤销与故障关闭，而不是直接安装包验收。本轮结果本地提交，不推送。

## 2026-10-09 4d354c 本轮borrower清理确认

用户提供第二窗口完整提示：`STOPPED: borrowed desktop owner joined and its isolated test journal cleaned. Identity creator remains READY; not deleted.`。据此确认4d354c本轮borrower真实join和隔离test journal清理，不扩大为节点/TLS通过；r2 creator继续READY，旧身份清理审计仍单独保留。下一轮使用已提前上传helper的修正代码及全新服务/compiled包，不复用已回收端点。仅本地提交，不推送。

## 2026-10-09 新borrower与OS授权通过，节点helper重复上传超时

本轮work `C:\Users\jarvis\remoteapp-split-consent-4d354c329175c557`：fresh包完整摘要、borrower真实桌面probe、双OS注册、邀请/claim、拒绝无grant、明确批准及双端grant验签全部PASS。随后split_nodes.run在首个60秒未压缩enrollment-helper SCP超时；尚无split_node_helper_uploaded，不标离线prepare、节点绑定或产品TLS通过。授权真实签名撤销PASS；verifier retire、隔离目录移除、STOPPED资源join、原策略恢复、服务器上传程序移除均实际PASS。客户端journal/Windows公开work保留；本轮borrower正常STOPPED待用户确认，creator仍READY。

修正剩余上传路径：split_nodes.upload_helper改压缩、180秒有界传输及摘要核验，调用移到fresh compiled包准备及设备注册/批准之前，所有nodes模式都不再在短期grant期间上传该helper；run仍重验摘要后才启动。没有延长grant、忽略摘要或降低退出门槛。30项Python回归PASS，新增传输参数和错摘要拒绝测试，diff检查PASS。下一轮实测待borrower正常STOPPED后使用fresh包；本轮不是完整4W通过。仅本地提交，不推送。

## 2026-10-09 borrower 正常STOPPED，修正人工就绪预算

用户提供本轮第二窗口真实退出提示：`STOPPED: borrowed desktop owner joined and its isolated test journal cleaned. Identity creator remains READY; not deleted.`，记录borrower退出与隔离journal清理闭环，第一窗口r2身份仍READY，不删除。此次不是TLS验收通过。

把上传后固定240秒等待改为从服务READY起计算的绝对600秒就绪预算，包含构建/上传/人工启动；既有compiled服务900秒期限不变，保留约300秒用于后续验收。超时查询即使返回public bootstrap也不接纳；未改grant/profile有效期、不复用上一轮包。29项Python回归和diff检查通过（新增到期不查询、跨期限响应拒绝）；下一轮真实联合结果待实际输出。按既有授权仅本地提交。

## 2026-10-09 fresh borrower 就绪等待超时：服务已回收，桌面退出待确认

差异公共传输后的真实重跑，Windows 全部文件摘要与 fresh pair 上传通过，目标为 `C:\Users\jarvis\remoteapp-split-consent-028d716e2c688a98\compiled-win`。协调器在 compiled_products.py 的240秒 borrower bootstrap等待期限耗尽，退出1；没有进入borrower native probe、设备注册、grant、enrollment或产品TLS，不算联合验收通过。实际日志确认exact verifier retire、隔离目录移除、STOPPED资源join、原file策略恢复和上传服务器程序移除；客户端node journals保留。

用户随后回复第二窗口已就绪；只读SSH核对该目标desktop-bootstrap.json现在存在，但不能据此补算已退出协调器的连接成功，也不继续使用已回收的服务/配置。下一步先让第二窗口Enter并确认STOPPED；r2身份creator第一窗口可保持READY供后续新协调使用。若停止两窗口，必须borrower先、creator后。当前尚未收到本轮borrower STOPPED，旧r1清理审计也仍待反馈。需要收口就绪等待流程，避免在用户启动期间过早回收后反复重跑；不得延长已签发grant或复用过期测试包。

## 2026-10-09 公共传输复用验收：整包两次超时已回收，差异传输实测通过

压缩180秒的第二轮也在整包SCP超时，部分Windows work `C:\Users\jarvis\remoteapp-split-consent-f2dfaae6b41dbcb4` 保留不可启动。没有register/approve/grant/enroll/TLS；server实际verifier retire/目录移除/STOPPED marker、原策略恢复和上传程序移除均PASS。没有第三次盲目整包重试。只读检查fresh Win包约144MB，失败目标已经包含多个公共文件；仅凭文件Length不能证明内容完整，不用部分包验收。

新增upload_public_bundle：目标必须不存在；仅在本轮可信manifest明确列出的公共文件中，r2源文件实际SHA等于fresh SHA时才在Windows本地Copy-Item并验副本SHA。分区响应严格拒重复/遗漏/未知文件名，其他文件用-C/180秒SCP传输；SHA256.json自身也单独计算摘要传输，最终逐项核验全部文件。源r2只读，不扫描/复制bootstrap、journal、node state、设备凭据，不覆盖运行中的creator，不在运行时覆盖root/CA/授权。

独立实际公共传输检查目录 `C:\Users\jarvis\remoteapp-public-copy-check-ee9e5b12697a026b`，复用3文件/上传10文件，最终完整清单全部SHA通过、工具exit0。该包来自上轮已退出端点，明确NOT LIVE/不可启动；没有启动服务器/桌面产品/创建节点。27项Python回归PASS，新增仅上传差异+manifest本身、源复制哈希检查与非法分区拒绝覆盖；diff检查PASS。随后可重跑真正fresh --compiled，本轮网络结果待实际输出，不将公共传输测试算TLS或权限验收。r2身份creator仍READY（未用户STOPPED）；旧r1清理提示仍待反馈。仅本地提交，不推送。

## 2026-10-09 r2 网络重跑：首轮公共包上传超时，已回收后压缩重试

用户授权“继续”，r2仍READY。首轮--compiled的8域原生probe、9项helper构建、隔离空控制平面、server/verifier/nonroot frontend/双forward与fresh pair构建PASS；Windows compiled整包未压缩scp在90秒期限超时。未提供borrower命令、未register/claim/approve/enroll或启动TLS。Windows随机work `C:\Users\jarvis\remoteapp-split-consent-931e2c00cea4f865` 的部分公开包保留不可启动；Mac本轮随机identity按finally关闭。server输出实际verifier retire、目录移除及STOPPED marker，原file策略恢复、上传server程序移除PASS，验收FAIL不改判。

仅将compiled公共整包scp改为-C压缩/180秒传输期限；不延长任何grant、profile或网络授权有效期。26项Python回归全部PASS，diff检查PASS；上传仍核对清单每个文件，不能凭scp退出就认可包。由于此前资源真实回收且控制平面为空，在用户仍保持r2 READY时启动独立新的--compiled重试；此轮状态待后续输出，不复用部分上传包或上一轮CA/issuer。

## 2026-10-09 r2 原生离线修正验收通过，下一步重跑真实双机网络

用户确认 `C:\Users\jarvis\windows-desktop-session-restart-20261009-r2` 桌面READY。实际执行该包probe-desktop-session.ps1返回VALIDATION PASSED；它使用本轮已摘要核验的新desktop-session/native helper，Probe逐项签署并验签全部8类域，再验证独占原生journal，非旧版仅invite域探测。

offline-prepare-diagnostic.py改用r2公开bootstrap及独立随机work，helper上传/摘要核对后实际输出：`PASS offline_retained_signer_before_prepare`、`PASS offline_native_intent_prepare_no_registration`、`PASS offline_retained_signer_after_prepare`，协调exit0。prepare helper真实prepared/exit0，retained vault-relay真实closed/exit0。当前源码native signer既能签provision intent，又不会因另一个helper准备/退出破坏保留连接，旧artifact故障的原生离线修正验证闭环。无server/grant/enrollment/bind/客户端节点/ACL/产品TLS，不将它扩大为整体4或网络通过。

本轮已知独立离线work `C:\Users\jarvis\remoteapp-offline-restart-4d91bc2ba439d348` 和未联网test node state `...\artifacts\connection-poc\nodes\diagnostic-1654e3b15d6f198f` 保留为证据；不复用该state。r2桌面creator仍READY，尚未用户STOPPED，随机test身份和journal仍归launcher持有。上一轮r1退出最后提示用户尚未提供，清理审计待反馈，不将新r2通过当作旧凭据已删除。

下一步可以在此r2仍READY时运行 `REMOTEAPP_WINDOWS_INTERACTIVE_DIR='C:\Users\jarvis\windows-desktop-session-restart-20261009-r2' python3 -B poc/selfhost/public-server/split-consent.py --compiled`，它现在构建9个当前helper（包括Windows native vault/journal），新服务/fresh pair就绪后再提供新borrower第二窗口命令。不要复用旧4T/r1或此前失效compiled包，不自动启动桌面borrower。当前没有启动此网络重跑。仅更新本地验收记录并提交，不推送。

## 2026-10-09 离线复现定位：旧 Windows signer artifact 缺少 intent/node/TLS 签名域

用户确认restart-r1桌面READY，真实desktop native probe PASS。离线诊断首轮公共helper未加压缩的60秒上传超时，未启动签名/节点helper，部分公开work `C:\Users\jarvis\remoteapp-offline-restart-37a843df98af3052` 保留。改仅公共artifact scp压缩与180秒上限后，新work `C:\Users\jarvis\remoteapp-offline-restart-b78659eeb9841fb5` 摘要核对完成；retained vault-relay签普通invite请求PASS，helper node状态准备后明确固定失败 `helper_intent_sign`。helper真实非零退出已Wait，retained relay正常closed/退出0；不存在网络注册/approval/grant/ACL/TLS。离线节点状态 `...\artifacts\connection-poc\nodes\diagnostic-dfd7c6e3014a1932` 保留，不用未知旧state重试。

核对r1实际identity-vault.exe（复制旧4T r2输入，SHA `38ad9eb62c3aaab6e68fb663854ca73fec826845a99611ceea9b253a60a2397f`）的可执行签名域字符串，仅invite-request/network-api，缺provision-intent/node-possession/session/product-TLS，而当前源码明确支持这些域。与离线intent_sign失败相互印证：构建流程使用了过期的原生helper，文件摘要匹配不等于接口能力符合当前协议。不能把重启或SSH故障当作本次intent失败原因；普通desktop probe此前只签invite，因而漏检旧artifact。错误域请求可使旧native子进程拒绝并退出，导致bridge后续签名失败；当前creator的干净STOPPED/随机凭据删除不能预先保证。

修正：compiled build_helpers除既有7项外，增加从当前可信源码重建Windows identity-vault-win.exe/trust-journal-win.exe，fresh pair不再读取过期这两artifact。desktop native probe扩展为8种精确协议签名域逐项真实验签，新增旧signer只能签invite/network时必须在第三项intent拒绝的单测；拒任意添加域或删除校验。Mac desktop-session/identity-vault Go测试PASS、Windows原生两helper与fresh独立restart-r2实际构建PASS；Python测试更新预期9项helper并检查这两包包含在列表，不将fixture测试标原生验收。

新r2使用当前source native signer（SHA `93261414cceb2f9a45079a839ea920264b4157792006a7c1a78590a6768e3a5d`），新的随机channel/scope，不覆盖r1/旧4T credentials或journal。远端 `C:\Users\jarvis\windows-desktop-session-restart-20261009-r2` 上传退出0，本地/远端全部清单摘要核验PASS，两个启动/probe脚本PowerShell语法PASS；26项Python回归重跑全部PASS，git diff --check PASS。新包桌面启动和8域原生签名尚待。请用户先在当前r1窗口Enter让launcher正常Wait；若警告Acceptance/cleanup NOT confirmed，记录并保留旧bootstrap/随机test凭据，不要求强杀或批量删凭据来伪造STOPPED。确认窗口退出后再新r2 READY，先重做离线intent与 retained signer前后验证，再进真实网络切片。整体4/TLS仍未完成，当前没有新服务器资源。本轮按既有授权本地提交，不推送。

## 2026-10-09 Windows 重启恢复：保留旧状态，准备独立身份入口

用户确认Windows重启后已恢复。实际SSH返回SSH_OK；精确查询interactive-vault/identity-vault/vault-relay/desktop-session/enrollment-helper未列出存活进程，但旧4T r2的interactive-bootstrap.json、desktop-bootstrap.json仍存在。该结果证明已无这些进程，不证明旧随机凭据与journal走正常STOPPED清理。旧文件/凭据/journal保留，不删除、不伪造STOPPED，不直接重跑旧initialize。

为后续离线桥接诊断构建独立 `windows-desktop-session-restart-20261009-r1`：installed-session-build windows-desktop-test生成新的随机compiled channel/journal scope，使用既有可信公共prerelease-v1 anchor/profile与固定Windows native helpers/真实C#测试产品；未读取release root/CA私钥、未启动服务器或新客户端网络。实际BUILD_READY及本地SHA256清单核验PASS。远端目标 `C:\Users\jarvis\windows-desktop-session-restart-20261009-r1` 上传前Test-Path=False，避免覆盖旧包。上传退出0、远端清单自身摘要与全部文件摘要核验PASS，两启动/probe脚本PowerShell语法PASS；尚未桌面启动/native probe或完成故障复现。

下一步用户在正常桌面启动该新目录的start-desktop-session.ps1并保持READY；收到READY后**先做离线节点prepare/保留签名连接复现**，不立即启动--compiled服务器、grant或TLS。协调环境REMOTEAPP_WINDOWS_INTERACTIVE_DIR须改用新目录，旧4T r2及上一轮随机compiled-win包不再作为本次入口。重启清理审计另留：旧凭据/journal不可因进程消失就批量删除。

## 2026-10-09 4W 本轮授权通过，offline prepare 失败；Windows SSH 后续超时

用户启动creator并确认新borrower READY；fresh Windows包在 `C:\Users\jarvis\remoteapp-split-consent-9a6b047819edbbb1\compiled-win`。真实borrower native probe、双API signer、双register、邀请/claim、拒绝/批准、grant校验及未绑定拒网络均PASS，上一轮首register失败没有复现。进入split_nodes后helper上传/摘要PASS，但未出现offline准备PASS；异常finally撤销时输出固定 `device_request_signer`。原准备异常被撤销异常遮蔽，不能据此确定节点准备的首因，也不能断言身份损坏。尚未enroll/join客户端节点或启动compiled产品TLS。

本轮新回收marker实际出现：verifier精确retire、实验目录移除、`STOPPED isolated_coordinator_resources_joined`；验收自身FAIL不因此变PASS。原file策略恢复、临时上传程序移除PASS。后续服务器只读复核nodes/users/preauthkeys均null、原accept策略、三个既有服务active、Nginx所有基线OK、正式Store SHA仍为 `5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca`。Windows公开包/work及Mac离线节点目录保守保留，不删除未知state，不重开已有节点。

随后旧4T r2 desktop probe实际PASS，只证明该桌面owner的native signer仍可签名，不证明interactive-vault桥接健康。准备离线并行诊断：新诊断helper上传且摘要核对完成，但在新vault-relay连接hello时失败（helper_unknown），没有启动节点helper或创建该诊断的随机node目录；finally关闭未获closed确认，不把它当成功join。该临时脚本位于忽略产物 `artifacts/connection-poc/offline-prepare-diagnostic.py`，不传authkey/approve或调用网络注册，不输出签名/私有回复。新的 `enrollment-helper-diagnostic.exe` 仅在本轮Windows随机work中保留，原binary未覆盖。再核对精确测试进程时Windows SSH `100.73.4.118:22` 超时，因此不能判断桥接是离线、退出、slot耗尽或签名子进程失败。禁止把它推断为唯一根因。

本轮代码补充固定node-helper阶段（signer_open/node_state/intent_sign/protocol）；JoinedPipe只输出白名单失败标签。failed one-shot通过实际Wait且非零退出可确认进程结束，但不会宣称prepared/closed或自动删state；退出0反而拒绝。consent记录node阶段的脱敏首异常，撤销失败单列，不能遮蔽首因。26项Python回归PASS，新增failed one-shot非零join/零退出拒绝及撤销错误不遮蔽首错误；Mac enrollment-helper Go测试PASS，Windows helper交叉构建PASS。整体4/双机compiled TLS仍未完成。

用户当前Windows两窗口的STOPPED尚待确认：先本轮borrower Enter→STOPPED，再creator Enter→STOPPED（launcher正常join，勿强关/强杀）。之后需恢复Windows在线/SSH才能继续桥接诊断，不能启动本轮已失效连接包。此次仅本地提交，不推送。

## 2026-10-08 4W 第二窗口实测：native probe 通过，首个 register 失败，服务器恢复完成

用户确认第二窗口READY，本轮 compiled borrower真实原生身份与独占journal probe PASS；Windows API binary上传/摘要PASS，Windows/Mac两个API signer READY。首个OS签名register未通过；未创建客户端grant/enrollment/node，也未初始化Mac compiled信任journal或启动产品TLS。不能把borrower probe当作联合验收通过。原API仅返回passed=false，无法确定是签名器、传输还是响应拒绝；之后只读核对Windows/Mac当前UTC秒一致，但这不是失败时完整时钟证据。

异常finally初次回收未确认，服务器仍有本轮check/daemon/frontend及verifier节点134，原策略备份保留；没有强杀或重注册。后续只读核对确认三个测试进程已自然退出，实验目录 `/tmp/remoteapp-daemon-check-4073748079` 不存在、nodes为空。管理端使用本轮准确备份 `/var/lib/remoteapp-poc/enrollment-mode-IQN_Rc` 恢复成功，随后nodes/users/preauthkeys均null、原accept策略恢复。三个既有服务active、Nginx基线全部OK、正式Store SHA仍为 `5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca`。本轮四个准确临时上传文件移除，不可恢复的仅这些可重新构建公共测试artifact；没有删除用户Windows包或凭据。

Windows保留目录 `C:\Users\jarvis\remoteapp-split-consent-de5883471d2ae3e7\compiled-win`。用户现已回复“两窗口已 STOPPED”，按用户反馈记录 borrower 与原4T r2 creator停止闭环，不再记作READY；未额外读取Windows凭据验证。此包端点已退出，不可继续使用或重复initialize。Mac测试身份owner及API/relay在协调退出时走正常关闭。

下一轮先在正常Windows桌面用 `C:\Users\jarvis\windows-desktop-session-4t-r2\start-desktop-session.ps1` 创建新的随机测试身份并保持READY；此旧入口仅用作身份creator，可以复用，不是复用已失效的compiled连接包。收到READY后才运行带新诊断的 `split-consent.py --compiled`，构建匹配本轮服务的新borrower包并提供第二窗口命令；不得提前启动保留的旧borrower。当前还没有启动新服务、创建grant/节点或重跑验收。

修补：Coordinator.join不再把SSH wrapper退出当作远端资源回收，只接受服务器在实际child join、票据retire、实验目录移除后发出的精确STOPPED marker。缺marker时保留backup/program，禁止自动restore。非零验收退出即使有回收marker也不能宣称验收通过。首要异常在finally前输出脱敏类型/源码位置，避免策略恢复错误遮蔽原失败。DeviceRequest新增固定signer/transport/response分类，保留errors.Is(ErrDenied)，不输出raw error、HTTP响应体、密钥或签名；下一轮才能据此确定失败层。未自动重试或扩大权限/有效期。

回归：24项Python测试PASS，包含SSH退出0但无marker拒绝、伪marker拒绝及有marker非零退出仍保留失败结果；Mac invite全量race PASS，新增三类失败仍为ErrDenied且不传播原材料；device-api-client编译PASS（无测试）。Linux daemon-network-check和Windows device-api-client交叉构建PASS，git diff --check PASS。测试回环监听首次被sandbox拒绝，授权后完整重跑PASS，不把首次失败当代码缺陷。仅本地提交，未推送；整体4仍未完成。


## 2026-10-08 4W 首轮真实执行：服务与新包通过，borrower READY 超时，已回收

用户再次确认旧4T r2身份READY。本轮真实启动 --compiled；首轮桌面探测未通过（泛化FAIL，未进入资源创建），单独SSH运行probe实际PASS；重试桌面probe和7个helper构建PASS，第二次服务上传阶段失败、临时上传程序精确移除，未切换策略/创建verifier。上传新增scp压缩与180秒上限（仅公共artifact），后续4个服务artifact均上传/摘要通过。不要将此前上传失败具体归因为认证或带宽，未单独复现其原因。

第三次进入真实隔离策略/authority，verifier与daemon ready后server退出，所有verifier/临时目录已回收、原policy恢复。检查发现新构建artifact原有权限0700且scp保留它，刻意非root probe/frontend无法执行；修正仅对本轮3个**公共程序** chmod755，不放宽material/config/journal/key权限。Coordinator.read现在明确处理server固定FAIL标签，避免伪装为JSONDecodeError；主异常只输出类型及末帧basename/行号，不输出原异常/私有回复。

第四次非root probe、错误root UID拒绝、frontend、公共bootstrap与双forward真实PASS，fresh pair构建成功后本地verify_bundle失败：darwin-test未生成SHA256.json（FileNotFoundError）。补齐installed-session-build条件desktop||macTest，生产模式行为不变。此失败没有上传Windows新包或申请grant/client enrollment；server实际退出、verifier回收与原policy还原完成。

第五次全部服务前置真实PASS，fresh匹配Test=true双包构建成功，Mac清单核验成功；Windows上传到 **C:\Users\jarvis\remoteapp-split-consent-47431f15558e5e16\compiled-win** 并逐文件摘要核验PASS。已向用户在commentary与异步问题提供第二正常桌面窗口borrower命令，但240秒内未观察到desktop-bootstrap READY，也未收到用户第二窗口确认；最终RuntimeError compiled_products.py:185，即borrower_ready_deadline。

超时结果：没有执行register/invite/claim/approve/enroll/bind/start产品；没有新客户端grant/节点或Mac信任journal初始化。两本轮随机Mac测试identity已按finally正常delete/exit回收；server verifier精确retire、隔离experiment目录移除、原policy恢复、上传server程序移除，所有管理forward已退出。Windows随机work/公开compiled包保留，未递归删除；日志PRESERVED_CLIENT_NODE_JOURNALS在本轮实际没有client-node目录创建，是保守work保留标签，不代表有活节点。旧身份creator及任何用户迟启动borrower的状态仍需STOPPED反馈，未擅自清理Windows原生凭据。

结束后再次SSH只读核对：三个既有服务remoteapp-authority/remoteapp-frontend/remoteapp-poc active；Headscale nodes/users/preauthkeys均null（为空）；原policy `100.120.0.2 -> 100.120.0.1:47476` 一条accept；nginx-baseline.sha256所有项OK；正式Store SHA仍为`5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca`。未读取root/CA私钥。23项Python回归、compiled-pair-build Go测试PASS，installed-session-build编译PASS（无测试），git diff --check通过。

### 用户当前操作与下次执行

已通知用户**不要再启动本轮刚才的新包**。若第二borrower窗口已迟启动，先在它Enter并确认STOPPED，再在旧creator Enter并确认STOPPED；若没有第二窗口，只需creator停止。等待确认，不猜测已清理。当前所有临时服务已退出，本轮匹配CA/issuer端点不再在线，不能用保留包继续验；不要重复initialize或复用旧profile。下次在双方准备好能及时操作第二窗口的情况下重新运行，重新生成fresh匹配材料，不自动重连/延长grant。

**本轮部分前置实测通过，但整体4和compiled双机TLS/正常join/撤销都未通过。** 后续仍需新borrower真实native probe、OS签名授权/真实node绑定、bound交接、产品持续TLS/正常join/post-stop撤销；live revoke/fault/upgrade仍按专门切片验证。代码/README/HANDOFF按既有授权本地提交，不推送。

## 2026-10-08 4C-4W：compiled 联合协调入口已开发，真实执行待 Windows READY

用户已确认上一轮 Windows 返回 **STOPPED**，4V保留的4T r2测试身份/桌面journal按launcher正常清理闭环；不再记作仍READY。本轮继续代码开发，尚未运行新隔离服务、创建新grant/节点或生成可联网fresh pair，未修改线上服务/ACL/Nginx/旧产品/Tailscale，也未读取发布root/CA私钥。

新增 split-consent.py --compiled，明确不是旧 --tls 的验收别名。首步必须旧4T r2正常桌面probe PASS后才创建两Mac随机测试身份；从当前可信源码离线构建7个Linux/Windows/Mac API/node/server helper，避免旧artifact不认识--os-compiled，Mac架构按当前host x86_64/arm64选择。原流程无参数/--nodes/--tls/--tls-fault保持不变。

compiled_products.prepare接真实管理SSH公共frontend bootstrap与两端回环forward，调用可信compiled-pair-build生成匹配公共CA/issuer/endpoints的fresh Test=true两端包，临时test root仍只在Go构建器内存生成。核对包文件摘要、上传到独立随机Windows work的compiled-win，再核对所有远端文件摘要，公开输出正常桌面borrower命令（仅路径，不含HMAC/authkey/grant），最多等待240秒。不能在SSH启动借用入口。读取公共bootstrap必须匹配设备identity_scope/pub、新compiled channel/epoch=1、由channel推导的journal scope、固定relay路径；还必须通过真实本地desktop relay hello/probe，不能仅凭JSON认为READY。旧4U build-only包不使用。

随后复用OS签名register/invite/claim/owner approve/enroll/bind/真实节点持有证明和最小ACL；先take单次bound handoff实际join旧helper，再Mac明确initialize隔离journal。target由同用户desktop relay启动固定compiled runtime，私有product_ready HMAC仅转交Mac installed-session stdin，父不替换指纹/root/CA/helper。严格验证两端product_count，要求两端至少3个pongs并在同一会话继续至少5秒、各再增长超过10，拒ready/零计数/失败结果/角色混用/计数倒退。此观测在真实执行后才是实际TLS证据；当前23项Python测试使用fixture，不能标实机通过。

**本首轮联合切片的退出顺序是正常停止两产品/node/watch/signer→实际closed+exit0或Windows session_closed→返回consent后owner签名撤销→节点回收零ACL→Mac journal inspect证明租约释放后cleanup-test→API clients/协调server实际join→还原原隔离policy/检查既有store与Nginx。不能称为活会话撤销验收。** 现有ProfileSession将活watch拒绝锁闭为ErrDenied，Owner也会保留故障语义；不能把该错误或非零退出改判正常清理，本轮故障路径保留证据。下一切片需专门接故障join/锁闭journal报告与恢复，验证真正live revoke、升级与故障，不改弱原安全屏障。

新增JoinedPipe仅用于compiled阶段，严格8192字节单帧/重复字段拒绝/部分行deadline/双帧缓冲/实际closed与exit0。stop失败保留handle，不kill；旧one-shot prepare只有prepared+已exit0才认可回收。compiled失败清理优先join新Pair；任何join不明就停止后续删除/密钥释放/policy恢复，不把旧Pipe.emergency强杀算完成。窗口creator只能在API签名worker全join之后停止；最终ACTION指令放在所有server/client清理及baseline检查后，而不是Pair刚停止时。

Windows compiled包/整个随机work保留，不在借用桌面owner仍活着时递归删除；Mac公开构建包保留，节点state仅在所有owner已join且服务确认回收后才删。Windows borrower原生journal仍由其正常桌面launcher持有，最终需要用户**先borrower Enter→STOPPED，再creator Enter→STOPPED**。脚本最终通过消息明确不包含borrower STOPPED清理、live revoke/upgrade/fault/GUI/签名安装包；任何超时/失败必须原样记录，不能重复initialize或自动重连。

证据：`python3 -B -m unittest discover -s poc/selfhost/public-server -p 'test_*.py'` **23项全部PASS**，含真实隔离Python子进程部分帧/双帧/退出协议、纯协调fixture的先join后initialize/双端持续增长/角色与secret过滤/故障阻止native清理/旧scope伪装拒绝；不含原生凭据或网络。实际离线构建Linux3、Windows2、Mac2个Go helper全部成功。没有更改Go/Swift/C#源，本轮不冒充重跑平台原生测试或真实产品传输。git diff --check通过；按既有授权本地提交，不推送。

### 下一轮真实执行（本轮已询问用户启动，尚未收到READY确认）

Windows正常桌面（身份creator窗口）：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\jarvis\windows-desktop-session-4t-r2\start-desktop-session.ps1
```

收到READY后Mac管理端运行：

```bash
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='C:\Users\jarvis\windows-desktop-session-4t-r2' python3 -B poc/selfhost/public-server/split-consent.py --compiled
```

该命令会运行隔离服务并构建/上传新匹配包，再输出第二Windows正常桌面窗口的具体borrower命令。用户看到该第二窗口READY后暂不Enter；协调端继续实际授权/node/TLS测试。保持管理脚本活着并及时沟通，server模式固定15分钟，不能离开活server跨turn等待到过期。失败只输出固定FAIL，秘密/原始异常不得打印；保留路径是公开诊断信息，不读取持有的秘密文件。全流程PASS后按最后ACTION顺序关闭两窗口，收集两条STOPPED再记录原生清理。

**整体4仍未完成；新联合入口开发和回归完成，不代表跨机实测通过。**

## 2026-10-08 4C-4V：compiled 产品计数观测与显式协调窗口，联合实测仍待

用户回复“已就绪”；通过 SSH 再次调用旧 4T r2 正常桌面 owner 的 probe，确认原生签名重开与独占 journal PASS。未直接在 SSH 下打开原生凭据，未启动新 borrower，也未创建新 grant/Headscale 节点。本轮结束时旧身份窗口仍由用户持有；请用户 Enter 并确认 STOPPED，不能未经反馈记录为已清理。

新增 clientruntime.ProductObservation，只绑定 runtime 已固定摘要启动的当前 TLS 产品，通过同一 Child.Exchange 查询 count（最多2秒）；未启动/已join拒绝，单session不重绑。严格区分 C# target 与 Swift controller 返回格式，拒未知/重复/秘密字段、负数、缺字段和未知结果，仅投影 product_count/pongs/拒绝与关闭计数/ended/result/pin_rejected；不暴露 HMAC、证书或私钥，不启动/替换/停止 child，不释放安装租约。现有子进程真实 Stop+Wait/join 屏障不变。count超时沿现有 Exchange 锁闭语义，不能静默重试恢复。

desktopcontrol/desktop-relay/desktop-session 新增同用户私有 count 入口，拒携带 Config/Profile 覆盖。每次获准 Start 新建观察器；失败 Start 不替换既有 observer。installed-session 改为命令循环，允许重复 count 后 stop/upgrade；错误/EOF/退出仍经过 Owner.Close 实际 join。bridge ready 或零计数不代表 TLS 成功。新的验收协调必须确认双端真实 pongs 增长、跨初始 lease 持续传输，然后签名撤销并加入所有 worker；当前没有跑这个联合流程。

daemon-network-check 新增显式 --os-compiled 模式：必须真实 Nodes、禁止 Fault/actions 混用，公共 bootstrap 增 compiled_phase=true；只此隔离测试模式固定15分钟协调窗口，旧模式仍5分钟。只增加准备时间，不延长 grant/watch 期限、不给自动批准或 grant。Python checked_ready严格区分新旧marker；现有 split-consent主流程未自动切换，完整 compiled 双机协调命令仍需开发。

证据：Mac全量 Go race PASS；新增控制参数拒绝测试另行 race PASS；Mac/Windows全量vet PASS；11项Python回归 PASS。Windows经固定摘要 run-product-observation-validation.ps1 运行5项必需测试全部PASS/退出0/无SKIP：格式投影、实际fixture子进程重复count与join后拒查询、私有start/count/stop直到join才ack、Config/Profile覆盖拒绝、空闲连接取消join。产品/API是fixture，不是实际Swift/C#网络、OS存储或双机Headscale。Windows测试 exe SHA256：product-observation-tests-win-4v.exe=`542b49053e611f3d2bfd8c30124f248c1d4ff759e5819f394c8510ea5b935231`，desktop-count-tests-win-4v.exe=`045018884e7d655af22d85ffcde38b1a966e0d5ac17fa885865858c172c9a3f1`，远端仅 C:\Users\jarvis 独立测试文件，旧包未覆盖。

Linux实际运行3项筛选纯协议测试全部PASS/退出0：新compiled marker/固定窗口与混合模式拒绝、旧operator不能批准或接受秘密、node marker必须有verifier。测试文件 /tmp/remoteapp-compiled-phase-tests-4v-20261008，SHA256=`d6f46e6c4812f58aa3abf57dfe661c58d281550350181616623734e8cc383876`；没有启动authority/Headscale、修改Nginx/ACL/数据库或读取release root/CA私钥。测试二进制保留为可复现产物。

下一步：开发完整私有compiled协调入口，串接真实frontend公共bootstrap→匹配fresh pair构建/上传→新Windows borrower桌面READY→OS签名consent/enroll/bind→旧helper真实join后的bound handoff→Mac测试journal明确Initialize→产品HMAC私有转交与本轮count观测→跨lease/撤销/真实join/精确清理。还要原生实测新borrower；旧4U build-only包不可启动。失败不准用 emergency强杀冒充join，不清理生命周期不明的身份/state/journal。**整体4未完成，本轮不宣称真实产品TLS/GUI或签名安装包验收。** 按既有授权本地提交，不自动推送。

## 2026-10-08 4C-4U：匹配隔离服务的双端构建/借用入口/节点交接，联合实测仍待

新增 trustinstall.NewTestPairMaterial 与 cmd/compiled-pair-build，明确是受信操作者从管理 SSH 得到公共 CA/issuer/两端管理回环端点后的 **test-only build**，不是在线TOFU/运行时root覆盖。input严格单一JSON，拒重复/额外root/private-key字段，允许文本尾换行；端点固定HTTPS127.0.0.1显式规范端口、非443。只在内存新生成一次性测试release root，签两端epoch1及预签epoch2候选（各自endpoint，同一CA/issuer、20分钟），只保存公共anchor/profiles；未读取既有release root/CA私钥，也没把签名器或root生成器加入运行时。构建器新增darwin-test/windows-desktop-borrowed-test，两端必须真实standalone产品，random测试channel/Test=true；构建后检查root/profile/Test与输入对应，拒用旧文件混搭。生产darwin/windows构建模式不变。

新增 start-borrowed-desktop-session.ps1，只借用已有READY随机测试identity，先连接实际creator pipe确认身份，再启动固定新compiled desktop owner。不会create/delete/stop其creator。新session pipe/scope取compiled journal scope，identity_scope另行标注，避免同设备身份的旧4T桌面入口和新包碰撞。正常launcher已适配新scope；已通过的4T r2旧binary/脚本没被覆盖，native_bootstrap新compiled_desktop模式仍可读取旧r2公共bootstrap做桌面probe（不是在SSH直接打开凭据）。新borrower退出须真正join/close0并清理自己的测试journal，之后才能停creator。该新借用流程还没有桌面实测。

compiled_coordination.runtime_inputs只整理已验证真实节点阶段的private Config（规范平台路径/节点键/IP/设备subject/hash/双ticket、固定角色）；不传Endpoint/CA/root/helper覆盖。split_nodes.run新增compiled_handoff，显式take仅在两旧node helper真实closed+exit0后才返回authkey/state/grant/binding；失败latch，不能重试自愈。无首次prepare/rekey/自动网络重开；compiled runtime仍自行检查state、实际node/IP、签名ACTIVE watch/绑定/grant。旧split流程未切换成新联合验收。

证据：Mac全量Go race与Mac/Windows全量vet通过；输入尾换行首构建失败已修复，增加单测并重新实际构建成功。11项Python协调回归通过；Window两项 TestTestPairMatchingIssuerCAAndAdvance/TestTestPairRejectsBadTrustedBootstrap 全PASS/退出0无SKIP（纯crypto fixture，不是OS存储或真实API），exe SHA256 `af2b471d4a6bfdb393b8d643baafe1e358085174072f4a1a364ae6a612f7e8af`；再次实际运行4T四项Windows本地pipe测试全PASS。新的build-only Windows整包SHA与三脚本（含borrower）PowerShell语法检查通过。不要求用户重复这些局部测试。

实际构建产物 artifacts/connection-poc/compiled-pair-build-only-4u/{mac,win}，用已有公共CA/issuer与不监听的回环41001/41002作 **build-only fixture**；已传Windows `C:\Users\jarvis\remoteapp-compiled-pair-build-only-4u\win`，没有运行trust-install/桌面启动、节点、TLS产品或修改服务器/Nginx/ACL/Tailscale/旧安装。该fixture不是可联网验收包，20分钟profile也会到期；不要让用户启动它或用它替代下一轮真实匹配包。旧4T r2在 `C:\Users\jarvis\windows-desktop-session-4t-r2` 保留可用，仅用来创建/持有测试identity。仅本地提交，未推送。

### 下一步与尚未闭环

整体4未完成；新compiled双机联合协调尚未跑通，仍需把这些组件串入独立协调命令：实际隔离frontend公共bootstrap+两端forward→fresh test pair build/upload→Windows桌面borrower READY→OS签名consent/enroll/bind→停止旧helper并重开bound state→private product_ready HMAC转交→真实TLS受保护payload/跨lease刷新/撤销及完整join→精确回收。还需新增产品状态/实际payload观测，不能用bridge ready代替TLS成功；协调端失败不能用旧Pipe.emergency强杀当join或删除不明状态。旧daemon-network-check operator阶段仍固定5分钟，须为新compiled协调提供明确的测试期限/超时清理，不能临时全局放宽。接入Mac测试journal的Initialize与session真实join后cleanup-test，同样不得自动reset。

需要用户在Windows正常桌面重新启动 **已通过的4T r2身份入口**并保持READY（非重复验收，不按Enter、不跑本轮build-only包）：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\jarvis\windows-desktop-session-4t-r2\start-desktop-session.ps1
```

得到READY后下一轮再从真实隔离服务公共材料编译新包，给用户新的borrower命令；本轮尚未启动隔离服务器等待用户，没有live grant/节点/私钥后台持有。先停新borrower再停identity窗口的顺序不能反转。

## 2026-10-08 4C-4T：新 compiled Windows 桌面入口，桌面原生验收通过

用户已反馈 r2 正常桌面执行通过：窗口1正常启动；窗口2返回 `VALIDATION PASSED: compiled desktop owner + reopened OS signature + exclusive native journal lease. NOT Headscale/product TLS/GUI or signed installer acceptance.`；窗口1 Enter 后返回 `STOPPED: desktop workers and native signer joined; isolated test journal and random test identity cleaned.` 这是用户桌面实测证据，不是 SSH 推断。原生身份重开/验签、独占 journal、停止退出及精确清理这一段闭环，不再要求重复；没有真实网络/产品会话，不能据此验收 Headscale、TLS、GUI 或正式安装。

已开发 desktop-session/desktop-relay/desktopcontrol 及新 start/probe 脚本。trusted builder target=windows-desktop-test（必须提供 standalone TLS product）生成独立随机 Test=true channel，固定公开 anchor/profile/journal/signer/product 摘要；runtime 不接受路径、root/CA、helper digest 覆盖或生产 journal 初始化。桌面 interactive-vault 创建随机测试 key，desktop-session 只重开，明确初始化测试 journal并持有独占租约。protected SID-only 本地 pipe 分开控制与旧签名 relay，不是公网 peer API。

probe 对随机挑战执行 OS 签名/验签并确认第二 journal owner 被拒绝，不开网络。start 固定 Windows target 与借用设备身份，复用 RunnerWithProduct；product_ready 测试 HMAC只私有pipe交付，禁止公开bootstrap/日志。stop_session真正join后确认并保留journal，超时锁闭不可自动恢复；升级沿 Owner.Upgrade屏障。父停止先joins reader/control monitors/connections，随后Owner.Close背景等产品/node/watch/网络，再signer.CloseJoined，全部成功才精确删除该测试journal；launcher先等desktop实际退出后才停随机key creator，仅两child closed回复和exit0打印STOPPED，失败不强杀冒充成功。vaultbridge也补齐monitor实际join。

Mac全量Go race、Mac/Windows全量vet与新增role/未编译拒绝测试通过（旧native默认未开，不声称本轮实测）。Windows实际SSH下运行四项协议测试全PASS无SKIP退出0：启动两条私有ready、stop实际join之前不ack、错误身份/额外路径字段拒绝、取消空闲连接join，以及真实本地pipe namespace/首实例碰撞检查。`check-desktop-session-bundle.ps1`全包SHA和两脚本PowerShell5.1语法解析通过。没有在SSH下创建/读取Windows原生journal/设备key，没有更改服务器/ACL/旧GUI/安装或Tailscale，没有读取release root/CA私钥。

新包已上传 **C:\Users\jarvis\windows-desktop-session-4t-r2**（本机 artifacts/connection-poc/windows-desktop-session-4t-r2），用户已执行桌面初始化/probe/停止清理。Windows protocol exe SHA256 `4266abe160f8333a8b40865fd139f7e9238c68bb7d642a4aea8f8971f229d47b`；只本地提交，不推送。r1仅本地早期构建，用户使用r2。

### 已通过的操作记录（无需重复；正常 Windows 桌面，非 SSH）

窗口1执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\jarvis\windows-desktop-session-4t-r2\start-desktop-session.ps1
```

看到READY后保持窗口打开；窗口2执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\jarvis\windows-desktop-session-4t-r2\probe-desktop-session.ps1
```

应返回 `VALIDATION PASSED: compiled desktop owner + reopened OS signature + exclusive native journal lease. NOT Headscale/product TLS/GUI or signed installer acceptance.` 再窗口1Enter，应返回STOPPED，两个本轮公共bootstrap删除。任何失败提供错误，不手动删除系统凭据或反复initialize；局部/部分journal保留供精确诊断。

**整体4未完成。** 本轮Windows桌面initialize/probe/STOPPED已由用户确认通过；新compiled客户端实际Headscale+产品TLS/活会话升级/故障/撤销尚待。既有预发布profile绑定既有issuer/CA，旧split-consent.py动态隔离frontend材料不匹配，不能混用。下一段从匹配隔离frontend的公共anchor/profile构建两端可信包，再新协调准确接bound node state（先停旧helper）、签名grant/binding与private product_ready。当前probe与fixture不替代联合双机验收。详见[实现说明](../poc/selfhost/public-server/INSTALLED_PRODUCT_OWNER.md)。

## 2026-10-08 4C-4S：产品进入compiled owner屏障，联合实机仍待

新增processowner.Own、clientruntime.RunnerWithProduct/Product、ProfileSessionWithClient、CompileProductSession。实际产品child在同一runner内准备/运行/Stop+Wait，product/网络worker都join后才能runner返还，之后watch/client/node/native signer.CloseJoined，Owner最后提交/释放。partial product启动失败同样Stop背景真实join，无内层timeout冒充完成。外部Pair仍只是历史协调组件，不再靠它提前释放bridge租约。ProfileSession取消保留非取消类worker错误；installed-session最终Close背景实际join不再8s超时直接exit；升级请求8s期限失败进入同一关闭等待，不自动恢复。native signer可重复Wait/检查closedAck非forced零exit，启动失败也等进程退出；LegacyClose emergency强杀不是成功。

Product准备用同一compiled profile NetworkClient/OS signer/校准clock。Target固定产品ready HMAC/fp→证书私钥证明→OS publish_tls→新鲜issuer描述符校验→私有product_ready(Key/Fingerprint)，再bridge ready；Controller只匿名pipe接测试ProductKey、自取并验描述符后传给固定Swift子进程，无parent pin/root/CA/endpoint覆盖。HMAC仍私有测试投递，非最终秘密分发；product_ready不可日志/落盘。新增embeddedProductDigest/sibling tls-product，编译产品模式不可runtime降桥接；模式固定Windows target/Mac controller。可信builder可追加standalone product路径；.NET必须单文件self-contained，含native抽取，不能仅copy apphost。build-public只是追溯。

Mac全量race与Mac/Windows全量vet PASS；真实Mac Keychain/helper StrictJoinedReopen单独PASS无SKIP0.26s，随机身份精确清理。Windows新入口 C:\Users\jarvis\run-owned-product-validation-4s.ps1 -BundleDirectory C:\Users\jarvis，4包固定摘要、11项必需top-level全PASS/exit0无SKIP，fixture API/journal/设备signer，真实子进程/匿名pipe/本地HTTPS，不是Windows native或真实产品/headscale。首脚本PASS regex漏转义失败，修正后整轮重跑，不计首轮PASS。详见[本轮证据](../poc/selfhost/public-server/INSTALLED_PRODUCT_OWNER.md)与script内4exe pin；不需要用户重复这11项。

真实Swift TLS产品重新release build，Windows当前源码self-contained单文件publish通过并核对hash 2fe320ff54d78dc77f4d69142284197758d289781f88810debae2ffde7550d47；两端新包 artifacts/connection-poc/installed-product-mac-4s、installed-product-win-4s 均BUILD_READY，现有prerelease公开anchor/profile/helpers，只构建。未initialize真实channel、未启动compiled会话/线上节点、未读发布root/CA私钥或copy节点state/authkey/grant/设备key到包，旧安装/GUI/线上服务NginxACL未改。本輪仅本地提交，不推送。

**下一段必须新协调+正常Windows桌面入口/隔离可信安装+旧helper停止/bound state重开+product_ready私有HMAC转交，再实际compiled双机正常/OS撤销/watch故障/升级前全join/显式新profile恢复/精确清理。** 现有旧split协议不能直接替代新产品模式，当前局部测试不能当联合验收；整体4、GUI/正式签名安装/公共接入/最终秘密投递仍待。未提供新的桌面READY包，暂不要求用户操作；没有JobObject/任意后代/硬kill耐久证明。

## 2026-10-08 4C-4R：专用管道/进程对和两角色启动适配

新增processowner.Child/Pair/StartEndpoint。可信本地规范exe+SHA256，无shell/秘密argv；8KiB严格匿名JSON pipe、16回复上限；timeout锁闭且不复用迟到回复。Stop成功必须closed/零exit/EOF/reader join；超时留handle，重试只join，不kill/自动重启。Pair任一exit/管道故障停止另一端，双端与monitor全部join才Done；正常并发停防单端卡住继续服务，异常永远FAIL。StartEndpoint Target先product→同步准备证书回调→bridge；Controller先bridge port→签名descriptor配置回调→product。失败保留Launch/部分handle，best-effort8s停止，不删node/key/journal。回调是真实接线待实现的可信代码，不把测试counter当证书API；产品ready只验status。

Mac新模块race、全量race及Mac/Windows模块vet PASS。真实Windows SSH执行10项行为测试全PASS/exit0，无SKIP，TestProcessFixture仅自举；最终测试exe C:\Users\jarvis\remoteapp-processowner-tests-4r-20261008-final.exe，hash a98aac065996db0978ef8b3ea7065c4c02062a9dcf67e9e5b798246644ff3258。测试是实际子进程/匿名pipe，但不是native signer/Headscale/Swift-C#产品验收。只上传测试exe，没有桌面READY/信任安装/线上节点操作，旧程序和服务未改，未读发布私钥。

**不是完整协调命令，也未把独立产品生命周期纳入journal。** Pair只管process；installed-session目前自身runner退出后释放journal，可能早于product。因此下一段先把真实product启动/退出移入compiled owner runner的租约屏障，再补可信配置供应/正常Windows桌面入口和OS TLS回调，替换旧split bootstrap，实际双机正常/撤销/故障/升级/join/显式恢复/精确清理。子进程环境继承、没有Job Object/进程组封闭，不能宣称任意后代已退出或当前用户恶意隔离。详见[本轮说明](../poc/selfhost/public-server/INSTALLED_PROCESS_COORDINATION.md)。整体4未完成；本轮仅本地提交，不推送，不要求用户重复旧r3/READY。

## 2026-10-08 4C-4Q：compiled节点运行时入口，局部通过/实际组合待验

新增clientruntime.Runner/NativeSigner、cmd/installed-session及可信installed-session-build、trustinstall.CompileSessionBridge。root/service/floor/channel/journal与signer摘要来自编译，不接受parent root/CA/state授权，普通空构建锁闭。命令只open既有journal，不安装/create/reset或创建/删设备key；first start使用已准备StateDir、signedGrant/Binding、role、公钥、VaultScope/Expected。先验profile/grant/设备公钥和nodekey，再真实HTTPS校时/watch ACTIVE，才实际tsnet.Up校验self/IP。运行时拥有OS signer、tsnet server、nodeproof与目标overlay47476/控制端loopback TLS桥接worker，grant撤销/故障或取消后join，再node.Close检查、最后signer.Close；整个runner返还前journal租约不释放。

nodeproof新增ServeBoundWithSigner，拒当前grant以外/非规范挑战；原接口保留给历史协调流程。关闭监视器加join，WhoIs nil/Node nil拒绝。stop/EOF/signal取消会话；upgrade成功提交后命令退出，显式新启动，不自动reconnect；错误候选/命令终止helper而非保持服务。stdin是专用父子pipe，不是通用交互控制台。现有node state必须可信/独占，先停旧helper再重开；只恢复已bound节点，不做首次enroll/bind。

4项Runtime局部测试Mac/真实Windows SSH全部PASS，无SKIP；Windows程序摘要014856b0c9c2b2667e1dcb9cecbfbe66f68c435b6d4dac05bc6044406d1c9430。普通未编译入口锁闭、bound grant单元PASS；Mac全量race、Mac/Windows全量vet PASS。Windows测试只本地socket/隔离文件，不是native signer或Headscale启动验收。

现有prerelease-v1/client-public公开材料与现有helpers生成两端包 artifacts/connection-poc/installed-session-mac-4q-ready 和 installed-session-win-4q-ready，BUILD_READY；未运行真实channel安装/升级或compiled会话，未读root/CA私钥，未把node state/grant/authkey/OSkey写包。仅runtime测试exe上传Windows，未替换旧安装或启动线上节点。

**实际双机组合未闭环，产品Swift/C#进程和秘密供应尚不归此helper所有。** 下一任务补协调验收入口（隔离可信安装、Windows桌面owner、停旧helper/重开bound节点、TLS描述符和证书私钥证明/HMAC、实际产品进程退出），再正常TLS/撤销/故障/升级/join/显式恢复与最终清理。不要直接运行新helper初始化正式信任，不能用旧r3脚本或局部mock代替这一轮，不要求用户现在重复旧测试。未改线上服务/Nginx/ACL/GUI/DMG/Setup；整体4未完成，仅本地提交不推送。详见[本轮边界](../poc/selfhost/public-server/INSTALLED_NODE_RUNTIME.md)。

## 2026-10-08 4C-4P：用户确认Windows r3完整桌面通过，第一段闭环

用户回传 `VALIDATION PASSED: native live owner lease + HTTPS watch/tunnel cancel and join. NOT cross-machine Headscale, product TLS/GUI or signed installer acceptance.`。按r3脚本必需项门禁，5项owner（含Windows native）、1项HTTPS/watch/tunnel集成及4项session回归均通过且无SKIP才输出该结果。记录为用户确认的正常桌面整轮验收，不捏造独立日志或耗时，不用之前SSH非native结果代替。脚本包含精确随机原生测试凭据清理和环境变量恢复，本轮未重新运行测试或修改程序/线上服务。

4C-4P第一段已闭环，整体切片4仍未完成。下一任务：编译锚/已安装配置绑定的新客户端节点启动入口，将OS signer、实际Headscale节点、watch和产品TLS工作进程纳入同一owner，再进行双机退出/升级/故障验收。GUI、正式签名安装包、秘密分发和public admission仍待。下方r3待验及r2失败段落为历史准备/修复记录，以本节为准；本轮仅本地提交文档，不推送。

## 2026-10-08 4C-4P Windows r2失败修复：r3可移植验证入口

用户截图确认r2五项owner测试PASS（含Windows native lease/reopen）；接着TestInstalledOwnerJoinsRealHTTPSWatchAndOpaqueTunnelBeforeUpgrade在managedSetup的invite.Open报authorization_storage_unavailable。定位为测试错误复用服务器Unix文件Store：Open要求目录权限无group/other位，Windows Go权限语义无法满足，不是native journal或实际网络拒绝。后续4项session选中测试也依赖同Store，原交付前只cross-build未实际跑这些Windows入口，不能算Windows验收完成。

修复只改_test夹具：新增profile_fixture_test.go和portable_fixture_test.go，仅内存生成临时issuer/grant/state/clock签名，保留真实HTTPS校时/自主watch/签名校验/流持有证明/升级JOIN/负向断流。服务器Store及生产权限检查未改，不新增Windows明文存储或放宽权限；fixture不验证持久化/注册/重放/admission，不能充当正式authority。原服务器存储测试与其余Store回归保留。

Mac session/streamgate race与全量race PASS，Windows两包vet/新包cross-build PASS。通过既有SSH在真实Windows运行r3无需桌面凭据的入口：HTTPS/流升级集成1.11s PASS，callback不脱离join0.50s、opaque流/关闭0.25s、拒upstream0.00s、跨域proof拒绝0.00s全部PASS；退出0，无SKIP。未在SSH运行native验收，不以r2的native PASS冒充r3整轮PASS。

r3包 artifacts/connection-poc/windows-live-owner-4p-r3.zip，SHA256 `21170dab857413e9cf0543d4edbb31019c5873e0a8e830fe152416d1c498ce81`，已上传核对、解压至当前用户/SYSTEM隔离目录、PS语法检查。**不再使用r2，保留旧包不覆盖安装。** 正常Windows桌面执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-live-owner-4p-20261008-r3\run-live-owner-validation.ps1"
```

预期仍为 `VALIDATION PASSED: native live owner lease + HTTPS watch/tunnel cancel and join. NOT cross-machine Headscale, product TLS/GUI or signed installer acceptance.`。等待r3整轮桌面结果，后续任务及产品接线边界不变。本轮只本地提交不推送，未改线上服务/旧GUI/发布私钥/安装。

## 2026-10-08 切片4C-4P：live owner/真实退出屏障第一段，Mac通过/Windows待桌面

新增clientowner.Owner/Handle/Group和streamgate.ProfileSession：只打开已初始化的compiled Release journal，不自动create；显式Start只取Current提交配置，一次一个runner/单调profile期限，Upgrade先验候选再cancel+JOIN，成功才提交/使用。超时或runner/commit故障锁闭，退出未完成不释放原生lease；Close重试只join/回收，恢复须显式可信状态重开。Runner/Factory仅可信本地注入，任意callback必须自行join嵌套节点/listener/子进程，借用signer须存活到runner退出。

ProfileSession是真实ProfileNetworkClient/authority校时/签名StateWatch/ManagedGate适配，不接受raw端点/CA/issuer替代；先Deny/cancel、join watch+work，最后close client。补齐session连接关闭监视器、Serve/Tunnel和Gate.Probe watchdog的join。authorized原先deadline后可能脱离callback，现改同步等待（500ms context契约仍需遵守）；坏callback不假报退出，而由外层owner超时保留lease。新增负向测试锁住这条边界。

Mac4项owner核心race PASS（内存journal）；新增1项真实Keychain/helper owner测试PASS/无SKIP：超时拒第二owner，join后Close/reopen/epoch2升级使用，精确随机测试channel清理。真实本地HTTPS校时/watch与opaque tunnel集成PASS：在途查询/流随升级关闭，故意延迟runner仍不commit，join后升级成功，旧watch无后续查询/流关闭、降级拒绝及显式新epoch启动。该集成的installation journal/overlay元数据/设备signer是fixture，不是实际Headscale/OS signer/产品TLS实测。新增授权callback测试和隧道回归通过；Mac全量race及Mac/Windows全量vet PASS。首次沙箱bind拒绝，许可本地监听后通过。普通全量未启用旧native环境项，勿称旧native全实测。

Windows新版构建入口cmd/live-owner-validation-build生成3个test exe+原生helper+必需项零SKIP脚本，运行时内存测试root/signatures，不读取现有私钥。最终包 artifacts/connection-poc/windows-live-owner-4p-r2.zip，ZIP SHA256 `77dc07f52a9f5d4b2d778b1a093ca07a6a50874d1e70f7dcbfce9993e46023fa`。用r2，初构包保留但不用于验收；r2含启动前取消不误报故障竞争修正，集成重复3次通过。桌面新入口（不使用4o/4mn或旧安装包，无需READY/SDK/管理员）：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-live-owner-4p-20261008-r2\run-live-owner-validation.ps1"
```

应为 `VALIDATION PASSED: native live owner lease + HTTPS watch/tunnel cancel and join. NOT cross-machine Headscale, product TLS/GUI or signed installer acceptance.`。r2已经既有SSH通道上传、ZIP hash核对、当前用户/SYSTEM隔离目录解压及PS语法检查通过，未在SSH里运行桌面凭据验收。本轮Windows桌面结果待用户回传。

本轮**未切换旧enrollment-helper或GUI到新owner，没有完整实际节点启动owner入口**；这段先建立并验证关闭屏障，不宣称整个切片4完成。下一段收Windows结果，提供compiled锚/已安装配置绑定的新客户端启动入口，将OS signer、实际Headscale节点、watch、产品TLS工作进程纳入同一owner，再双机升级/退出/故障验收。线上服务/Nginx/ACL/发布根与CA/旧DMG和Setup均未改，正式发布认证、秘密分发、public admission、产品安装仍待。见[本轮说明](../poc/selfhost/public-server/LIVE_CONFIGURATION_OWNER.md)。本轮仅本地提交，不推送。

## 2026-10-08 切片4C-4O：Windows桌面通过，编译安装锚/离线升级闭环

用户确认本轮新入口返回 `VALIDATION PASSED: compiled anchor + native first install + upgrade/floor/recovery. NOT signed product installer or live-session acceptance.`。记录为用户反馈的Windows桌面验收，不捏造逐项日志或退出码；不是上一轮4M/4N结果替代。本流程覆盖首次安装、升级/最低epoch/失败保留、防降级、固定根与helper拒绝及损坏锁闭，并执行精确测试凭据清理。本轮只更新验收记录，未重新运行程序或修改线上服务。

新增trustinstall、cmd/trust-install、trusted build及隔离validation build/check入口。规格root/service/最低epoch/channel/helper SHA与签名bundled profile只编译进入程序；不读运行时anchor/env或接受root/floor/scope/helper参数，空编译规格失败。scope仅依赖固定channel，因此换根/service不能跳新存储；普通公开材料构建器Test=false，只测试编译产物允许精确测试清理/损坏注入。

initialize先验证当前时效再明确新建；inspect/upgrade只打开已有journal，从不自动转create/重置。离线维护owner不创建watch/TLS/tunnel，空quiescer不得作为后续活跃客户端join适配。升级提交/读回/关闭成功才CLI报告usable；新程序floor提高拒旧配置，旧程序也不能回退已有新epoch。同epoch改写、错根/service/时效及helper摘要变化拒绝。固定helper同目录路径需规范/无symlink，不宣称hash提供发布者认证或抵抗同用户替换竞态。

4项新核心race通过；初轮测试fixture位于macOS symlink临时路径导致摘要正向case拒绝，规范化测试目录后复测通过，生产路径规则未放宽。Mac真实固定v1/v2/other-root程序+原生Keychain完整验收退出0：首次/拒覆盖、floor2拒epoch1/失败保留原提交、升级2/跨进程读取/旧程序降级与签名负向拒绝、sidecar/env忽略/换根同scope/改helper拒绝、原生索引损坏后所有恢复/初始化拒绝与精确随机测试凭据清理；无SKIP。Mac全量race/Mac和Windows全量vet通过。普通全量未开启旧journal原生env，不能冒充其本轮实机重跑；新编译流程已单独真实验收。

实际现有prerelease-v1/client-public两份公开材料构建 artifacts/connection-poc/prerelease-trust-install-4o，BUILD_READY；未执行真实channel安装/升级，未读取/上传任何现有release/CA私钥，也未重新签真实epoch2或改线上服务。所有新旧测试签名由隔离内存测试根生成，仅公开fixture进入验证包。

Windows新完整包 artifacts/connection-poc/windows-trust-install-validation-4o.zip 已上传ZIP hash核对、解压、PS语法检查：C:\Users\jarvis\remoteapp-trust-install-4o-20261008。正常桌面入口为 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-trust-install-4o-20261008\run-trust-install-validation.ps1"`，用户已确认本轮通过。测试profile约23小时有效，后续复测如过期须重建，勿改时间/放宽验签。

Formal代码签名/可信分发与受保护安装仍待，不能把编进root当发行者真实性已验证；同用户/管理员删改及整盘快照回滚、原生断电/EIO/硬崩溃仍不在已验边界。不改现有GUI/DMG/Setup或live会话，整体4未完成。切片3已闭环；下一步切片4实际会话owner/真实cancel+join接入。本轮只本地提交不推送。见[本轮说明](../poc/selfhost/public-server/TRUST_INSTALLATION_RELEASE.md)。

## 2026-10-08 切片4C-4M/4N：Windows桌面通过，两端存储闭环

用户明确反馈本轮新包执行返回 `VALIDATION PASSED: Windows native trust journal + exclusive helper + persistent epoch floor. NOT installer trust provisioning or product acceptance.`。按验证脚本门禁，Windows4项必需Native均PASS、无SKIP才会输出此结果；记录为用户确认的正常桌面验收，不捏造逐项耗时/独立抓取日志。此前SSH interactive_logon_required是历史环境限制，不再阻塞本切片。

macOS与Windows原生保护存储切片1/2已闭环。该脚本流程包括精确随机测试凭据清理和恢复环境变量；本轮没有重新运行测试、修改程序或服务。下一步切片3：可信安装锚与配置升级（首次安装、可信最低epoch、升级/拒降级及失败恢复），然后切片4：真实客户端会话owner接入。没有正式安装锚供应、GUI/安装包/公网产品验收或整体4完成；保护边界及包hash见下方与两端说明。本轮仅本地提交文档，不推送。

## 2026-10-08 切片4C-4M/4N：两端原生journal代码完成，Mac通过/Windows待桌面

新增nativejournal + cmd/trust-journal + trustprofile.DesktopJournal。Mac Keychain（不交互/不同步/ThisDeviceOnly）与Windows当前用户Credential Manager，命名空间独立于现有设备和配对密钥。原生manifest最后更新，2048字节内容寻址块/整条digest读回，精确旧块清理失败也锁闭；新安装明确create，open绝不转create。helper继承私有管道/无网络，Windows固定OS线程+同SID Global mutex；MacOS账户Home而非HOME环境变量、0700专属目录/0600单链接flock，权限异常拒绝。私有journal帧显式16KiB，既有默认8KiB不变。

Mac开发ad-hoc签名helper实测4项Native全部PASS无SKIP：跨进程floor/独占/旧profile/错根拒绝；manifest缺失/损坏、分块丢失/等长篡改拒重置；8191字节原生持久重开；pending及CAS失败后关闭/明确旧状态重开。最终增补owner退出后create仍不能覆盖已提交记录复测通过。精确测试Keychain凭据清理；保留非敏感scope锁文件，避免删除锁导致双owner。旧RemoteController密码弹窗未改，不把ad-hoc成功当正式签名升级授权通过。

共享Store3项race内存故障测试PASS（块/索引/写后不确定/旧块清理）；Mac全量race及Mac/Windows全量vet通过，Windows x64 helper/测试构建通过。本輪Windows SSH上传/hash及实际测试启动成功，但4项Native都在访问凭据库前置步骤返回interactive_logon_required，exit1，**不记通过或SKIP**。Empty失败发生在凭据写入前，临时SSH测试目录精确删除；必须正常登录桌面验收。

全新Windows包已ZIP hash校验、解压到 C:\Users\jarvis\remoteapp-trust-journal-4mn-20261008，PS语法检查通过。本机 artifacts/connection-poc/windows-trust-journal-validation.zip。桌面PowerShell执行 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-trust-journal-4mn-20261008\run-trust-journal-validation.ps1"`；4项PASS无SKIP及VALIDATION PASSED才闭环。包helper hash 2d14dc64a28ec53ca7bd4c82f6425a8341c539dbeb714ee0552dc4820b2a9ca4、测试hash1468d299b418fa908c1ee8c4b57fb28421c429d1ab7cfe7bc4e446a7d4f02395。不要使用旧安装包，不需Windows READY/Tailscale/SDK。

保护边界是其他OS用户/意外损坏/协作并发，不防当前用户恶意程序、管理员、完整快照回滚或删光namespace后再次明确create；没有原生断电/EIO/硬崩溃耐久验收。Windows凭据是用户级不是应用级隔离；Mac正式签名升级和Keychain访问授权仍待。可信锚供应、会话owner/GUI/安装包/秘密分发/public admission未接，整体4未完成。先收Windows桌面结果，再开始切片3/4；本轮本地提交不推送。详见[两端说明](../poc/selfhost/public-server/TRUST_DESKTOP_JOURNAL.md)。

## 2026-10-08 切片4C-4L：Linux保护journal实机通过

新增linux-only LinuxRootJournal，接TrustJournal；可信root全新目录明确创建，恢复不创建文件、不重置pending/损坏/缺失状态。root所有安全祖先逐段openat/no-follow，专属0700/文件0600/单链接，flock生命周期独占；随机临时文件写入+文件fsync+rename+目录fsync完成后才提交成功。读/提交/CAS故障锁闭，Close无删除API，调用者必须先join会话。安装锚仍由可信调用方提供，未实现正式安装器供应路径。

最终Linux amd64测试上传hash核对后，4项Native测试全部PASS无SKIP，含真实子进程重开floor/旧profile拒绝、同/跨进程owner冲突、UID65534拒绝、8类恢复异常拒重置、CAS失败旧状态保留及6类活owner/路径异常。随机/root测试目录由测试精确删除，上传程序精确删除；未接线上服务、未改Headscale/ACL/Store/Nginx或使用WindowsREADY。

Mac全量 go test -race ./...通过（Linux适配不在其编译范围）；Linux全量vet/测试构建通过，真实Linux执行不是race。独立SSH复核三服务active、线上State仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca、Nginx基线全部OK。未验断电/磁盘EIO/ENOSPC或rename后目录sync失败，不宣称这些故障的实机耐久已闭环。

保护非特权本地用户，不抵抗root/忽略协作锁的特权进程/原始磁盘或快照回滚；状态非加密保险箱。普通JSON文件无权限保护不能替代该适配，本组件也不是macOS/Windows桌面提权方案。下一步需两端原生保护存储、可信固定安装锚供应与会话owner接入；产品GUI/安装/秘密分发/public admission和整体4仍未完成。Windows保持STOPPED。本轮完成后仅本地提交，未推送。见[Linux说明](../poc/selfhost/public-server/TRUST_LINUX_JOURNAL.md)。

## 2026-10-08 切片4C-4K：可信安装与持久版本下限核心

用户已确认上一轮4C-4J Windows STOPPED，桌面签名器停止闭环；下面历史段落中的待确认不再是当前状态。本轮不需要READY，未启动Windows桌面进程或修改线上服务。

新增 trustprofile/installation.go 和 installation_test.go。安装锚必须来自可信安装器/二进制，不从网络或journal取根；首次初始化仅接受明确全新安装。恢复严格解析已提交的签名profile，缺失/损坏不能自动重置；历史过期配置可恢复用于显式轮换，但不能作为Current使用。MinimumEpoch可由可信软件提高，根轮换仍需另行设计。

新配置先Advance验签/防降级/拒同epoch改写，再取消并join旧会话，重新检查时间/签名，原子持久CAS提交后才发布Current。join失败、提交失败（含写入后结果不确定）、外部状态变化均锁闭当前owner，不自动重连或重置。SessionQuiescer必须覆盖实际watch/TLS/tunnel全部worker，现阶段仅接口和测试，未接现有客户端。

全量 go test -race ./... 通过；最后加固后 trustprofile race复测通过；go vet ./...、Linux amd64/Windows x64测试二进制交叉构建和diff检查通过。5项新增测试使用内存journal模拟持久字节、重开及CAS，不是实际OS安全存储测试；本轮没有Linux/Windows原生运行或真实双机验收。

重要边界：没有生产TrustJournal适配；独占lease、durable CAS以及未授权重置/回滚保护是下一切片的平台责任。核心不能识别两个owner之间恢复整个旧journal，不宣称已具磁盘快照防回滚能力。没有信任锚正式安装、秘密分发、GUI/安装包集成、公网admission或整体4完成。

下一切片先设计并实现平台保护journal和可信安装锚供应路径，再接客户端会话owner。需真实进程重开、缺失/损坏拒重置、并发owner拒绝和提交失败验证；系统权限/回滚威胁模型要明确，不能把普通JSON文件包装为安全存储。详见[核心说明](../poc/selfhost/public-server/TRUST_INSTALLATION_CORE.md)。本轮完成后只本地提交，未推送。

## 2026-10-07 切片4C-4J：旧ACL留存TLS故障关闭/显式恢复完整通过

用户重新READY，实际Windows pinned probe通过，最新 split-consent.py --tls-fault 完整退出0。本轮Windows源码archive/hash、65/65协议测试/publish通过（DPAPI明确SKIP）；OS consent/nodeproof/最小规则与真实Swift/C#负向/正常跨lease仍通过，未使用旧fixture。

实际本轮隔离daemon SIGSTOP并确认内核T态；非root frontend真实HTTPS/network在broker期限后503，暂停前后非零policy相同。外部控制面核对精确controllerIP→targetIP:47476，TLS关闭前后旧规则仍在；两端自主watch退出/join，真实Swift connection_closed非timeout/cancelled，C#session/helper tunnel各关闭一次且无新pong。线上authority/frontend/poc/provider均未被暂停。

明确recover CONT→原daemon正常TERM/STOPPED/零退出/join→同Store/issuer与原verifier key/IP显式新daemon；批准最小规则恢复，fresh signed active binding与原节点一致。旧两端managed gate仍done/不active，fresh parent state拒绝重开，旧TLS不增加pong。显式新helper/clock/watch/gate和新Swift TLS连接才恢复，同node/IP、fresh descriptor匹配原C#证书，原连接之外新会话持续跨lease；最后Windows真实OS签名撤销，自主watch关闭、TLS非timeout关闭/零新增pong。

所有helper/API/产品进程正常closed/零退出，客户端cleaned日志提交屏障通过；独立daemon/frontend正常STOPPED/join/零ACL、精确verifier Retire，临时节点/目录/上传程序清理，原file模式/策略恢复。独立SSH复核三服务active、线上State哈希仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca、Nginx全部OK、nodes/users/preauthkeys null（零），原100.120.0.2→100.120.0.1:47476规则完整。

本轮未修改服务逻辑/新跑全量组件；Linux8/Python5是上一准备轮证据，此轮新增是完整实际双机零退出。**Windows桌面owner未被远程停止，需用户最后Enter/STOPPED确认。** 未验provider断网/磁盘损坏/硬崩溃/自然300秒到期，不把SIGSTOP场景扩展宣称到这些；HMAC/SSH/test root仍私有验证边界，正式信任安装/admission/安全秘密投递/GUI/安装仍待，整体4未完成。下方待验记录为历史。见[实机记录](../poc/selfhost/public-server/SPLIT_TLS_FAULT.md)。

## 2026-10-07 切片4C-4J：旧ACL留存TLS故障关闭入口准备，待READY

用户确认4C-4I Windows STOPPED，上一轮桌面停止闭环。新增--tls-fault/--os-tls-fault及fault_phase公共marker；私有stdio严格freeze→recover→stop一轮，拒乱序/扩展/重复/代批准，普通模式只stop。仅实际隔离daemon PID SIGSTOP并核对内核T态，不改线上服务/provider，不清规则/删节点；私有frontend真HTTPS/network在broker预算后必须503，控制policy暂停前后精确同一非零语义。异常仍暂停则先SIGCONT再正常stop，join/cleaned未确认保留journal。

双机计划核对精确旧controller→target:47476规则关闭前后留存，helper自主HTTPS watch/lease关闭实际Swift/C#TLS且无新pong/非timeout；recover只先CONT→正常TERM/STOPPED/join再原Store/verifier key/IP显式新daemon，fresh active signed binding同原pair。旧gate保持done/不active、fresh parent state不能复活、旧TLS不再增pong；显式新helper/clock/watch/gate/新TLS才恢复。split_nodes重复managed restart修正捕获引用更新；split_tls复用既有C#证书/HMAC、不重复publish/负向测试，fresh descriptor仍必须核对。最终OS撤销与30秒日志提交屏障/精确回收原策略恢复。

本轮Linux构建/vet、实际Linux8项组件PASS无SKIP（仅自建sleep暂停，不触控制面）、Python5项纯断言/diff通过，测试binary已删。没有运行新双机TLS故障、没有本轮Windows编译/服务器配置或节点/ACL修改；Go服务未改、未重跑全量race。不把模拟进程暂停等同磁盘故障/provider断网/硬崩溃/自然到期。HMAC/trust/SSH仍test边界，整体4未完成。

下一步请Windows正常桌面重跑现有windows-native-network start-interactive-vault.ps1保持READY，实际probe后 `REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\chatgpt\远程软件开发\测试安装包\windows-native-network' python3 -B poc/selfhost/public-server/split-consent.py --tls-fault`；需要最新daemon-network-check已构建。完整实机零退出/独立复核/用户最终STOPPED才记通过。见[新入口](../poc/selfhost/public-server/SPLIT_TLS_FAULT.md)。

## 2026-10-07 切片4C-4I：独立进程真实产品TLS完整复测通过

用户重新READY后实际pinned Windows probe通过，执行 split-consent.py --tls。最新完整复测退出0：本轮源码archive哈希、Windows65/65 bridge协议测试和publish通过（DPAPI明确SKIP，非凭证持久化）；真实OS consent/nodeproof/最小规则流程仍通过。两端同node/IP正常重开managed helper，旧Token fallback拒绝、自主HTTPS校时/watch/gate active，父state不能授权。实际C#证书私钥证明由Windows OS签名HTTPS发布，Mac取得fresh issuer描述符且指纹匹配实际C#证书；真实Swift错pin/错HMAC/认证前PING各结束且零protected pong。正常TLS至少3pong再持续5秒增加>10，原连接跨初始lease；Windows OS签名撤销后双端watch done/join，Swift connection_closed非timeout/cancelled，helper tunnel与C#session各关闭一次，后续pong无增长。

首轮功能通过但清理失败不算完整PASS：节点已回收/ACL零但一条Store记录cleanup_pending，停止daemon取消了维护尚未完成的日志提交。原实验目录/tmp/remoteapp-daemon-check-2748000149/模式backup保留；只读确认无writer/socket/verifier owner，用原Store/同verifier恢复独立daemon首Reconcile，实际两条记录cleaned后正常STOPPED/零退出，精确Retire并恢复原模式/删除本轮已验证目录和程序。未伪造记录、未删线上writer锁。

check新增30秒awaitClientCleanup：收到stop后先让活supervisor完成全部cleaned提交，再停frontend/daemon；终止后仍核对日志。超时/读取错误保留证据，不以节点消失代替日志确认。Linux5项真实组件PASS无SKIP（新屏障等commit/取消/读取错误）、Linux构建/vet、Python4项断言/diff通过；本轮Go服务逻辑未改，未重跑Mac全量race。

最新整轮helper/API/产品进程均closed/正常零退出；确认cleaned后独立daemon/frontend正常STOPPED/join/零ACL、精确verifier Retire，所有本轮节点/目录/上传程序清理，原file配置/策略恢复，线上State原hash/Nginx与服务未变，控制资源零。**Windows桌面owner仍由用户持有，待最后Enter/STOPPED确认**，未远程停止/删除。

不证明旧ACL残留时安全关闭、自然300秒到期、硬崩溃恢复；HMAC仍私有测试投递，root/CA/SSH为私有测试bootstrap，不是最终信任安装/公网API/GUI/安装。下一步旧ACL留存的权威故障503→自主TLS关闭、显式同身份恢复（先准备入口，不贸然改线上Store/业务443），随后正式trust/admission/HMAC产品化；整体4未完成。见[实机记录](../poc/selfhost/public-server/SPLIT_PRODUCT_TLS.md)。

## 2026-10-07 切片 4C-4I：独立进程产品TLS入口准备，待READY实测

用户确认4C-4H Windows STOPPED，上一轮最终桌面退出确认闭环。新增 split-consent.py --tls/split_tls.py；split_nodes返回实际binding与显式同node/IP重开managed helper能力。产品源码archive/Windows65协议验证（DPAPI明确SKIP）与publish安排在创建短期grant及启动隔离daemon前，不消耗grant编译。实际C#与Swift探针，OS target发布证书私钥证明，controller fresh issuer描述符指纹核对；helper自主HTTPS校时/watch/gate，无父进程刷新线程/state授予，无旧Token fallback。

计划负向错pin/错HMAC/认证前PING分别结束且零protected pong；正常TLS3个pong后再运行5秒增加>10、同连接跨lease；OS撤销后两端watch done/join、真实TLS connection_closed而非timeout/cancelled，helper/C#各关闭一次且后续pong不增，再节点回收/原策略恢复。失败仍OS签名撤销，沿用cleaned journal保守清理，不修改线上Store/服务或用fixture代签。HMAC为私有测试注入，非最终秘密分发；无旧ACL残留故障/自然300s到期/硬崩溃/GUI证据。

本轮Python4项纯断言PASS（新增负向/关闭计数严格测试）；Swift release TLSBridgeProbe构建成功；diff检查通过。没有运行新双机TLS、没有本轮Windows编译/服务器修改/新ACL证据，Go服务未改、未重跑全量race。下一步Windows正常桌面重跑现有windows-native-network start-interactive-vault.ps1保持READY，实际probe后运行 `REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\chatgpt\远程软件开发\测试安装包\windows-native-network' python3 -B poc/selfhost/public-server/split-consent.py --tls`。完整实机退出0后独立复核/用户EnterSTOPPED才记通过。整体4未完成。见[入口和边界](../poc/selfhost/public-server/SPLIT_PRODUCT_TLS.md)。

## 2026-10-07 切片 4C-4H：独立进程真实客户端 nodeproof/最小ACL通过

新增 split-consent.py --nodes / split_nodes.py，check --os-nodes只额外公开verifier节点公钥与node_phase标记，无admin enroll/bind/批准/签名。沿用实际独立root daemon/nobody frontend+kernel broker+真实Runtime/Headscale supervisor，enroll/bind全部走两端OS签名HTTPS；无旧fixture。用户继续时实际Windows pinned probe仍通过，因此没有远程重建桌面owner。

最新完整复测退出码0：Windows/Mac离线准备节点公钥与OS签名意图，prepare不注册客户端；两端HTTPS enroll通过、同意图重放拒绝；实际join key匹配；未绑定零规则拒绝载荷；Windows真实网络+OS持有证明通过；故意错误Mac OS身份bind拒绝，实际收到challenge但signed=0、ACL零；停止错误helper、同node key/IP恢复正确身份后bind通过。周期投影精确controllerIP→targetIP:47476；两端签名active binding与实际node/IP一致；正向实际固定载荷通过，反向route/dial拒绝且peer载荷计数0。

Windows真实OS签名撤销、权威status revoked不回grant；周期回收客户端节点、ACL零；实际载荷再拒绝。两个节点helper与两个API closed/正常零退出；frontend/daemon正常STOPPED/join/零ACL；精确verifier Retire、隔离目录/上传程序/Windows随机客户端目录/Mac随机身份与节点目录回收，原file配置/策略恢复。线上服务active、State原hash/Nginx未变、控制资源零。

首轮前述功能通过但最终清理断言失败，不算完整PASS：checker要求Provisions空，实际后端保留两条Status=cleaned审计记录。修正为writer join后只读诊断核对Version1且所有记录cleaned，不接受bound/cleanup_pending/缺Status；不并发打开Store或改journal。失败轮保留/tmp/remoteapp-daemon-check-1640063052与模式backup；只读确认两记录cleaned、无writer/socket/verifier owner及子进程后，用精确verifier-owner Retire，控制资源零，恢复backup并删除该已验证实验目录/程序；之后完整重跑成功。Linux4项协议/清理断言真实PASS无SKIP，Linux构建/vet、Python2项断言（含phase失败仍签名撤销）与diff检查通过；本轮未重跑Mac全量Go race，不沿用旧记录当新证据。

**本轮无产品TLS/可信安装锚/最终公网API证据。** Token只在私有管道用于固定载荷，不是最终HMAC分发。撤销后网络拒绝发生于节点回收/ACL零，不等同节点保留时单独零ACL拒绝验收。下一步把Swift/C#产品TLS及旧ACL留存时签名刷新/撤销关闭接入独立进程，再补纯零规则/故障/信任安装/GUI等。Windows桌面owner未被远程停止，仍需用户Enter/STOPPED确认；整体4未完成。

## 2026-10-07 切片 4C-4G：独立进程真实双机 OS consent 通过

用户确认 Windows READY 后执行最新 split-consent.py，完整退出码0。实际桌面 pinned probe通过，两个API进程使用各自真实Windows/Mac OS signer，管理SSH私有转发到实际nobody frontend→kernel broker→独立root daemon/真实persistent verifier/supervisor；不运行旧enrollment-fixture，不代签或批准。

两端签名注册及不同主体通过；无grant network拒绝；邀请兑换仅pending无grant，owner发现精确peer；controller自批准拒绝；owner明确拒绝不回grant；新邀请仍pending，owner批准后两端同grant、签名grant_clock校时/主体/期限验签；没有客户端节点绑定仍拒绝network；Windows owner真实签名撤销后权威status revoked且不回grant。没有客户端enroll/nodeproof/产品TLS，本轮不能冒称这些已接独立新进程。

两个API helper closed/零退出，frontend与daemon正常STOPPED/join/零ACL，精确verifier Retire、本轮隔离目录/上传程序/Windows随机客户端目录删除，随机Mac测试身份清理；原file策略恢复。独立SSH复核三服务active、线上State哈希仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca、Nginx基线全部OK、nodes/users/preauthkeys null（零）、原100.120.0.2→100.120.0.1:47476规则完整。未改线上binary/unit/Store/profile/业务443或开放公网网络路由。

Windows桌面owner未远程停止/删除，**需用户此时Enter并确认STOPPED**。下一步把真实客户端enroll/bind/nodeproof、最小规则/零ACL网络拒绝和产品TLS接到同一独立daemon/frontend；再做正式trust/admission/HMAC/GUI/安装，整体4未完成。下方“待验”是历史准备记录。见[实机记录](../poc/selfhost/public-server/SPLIT_OS_CONSENT.md)。

## 2026-10-07 切片 4C-4G：独立进程 OS consent 入口准备，双机待验

用户继续后新增 daemon-network-check --os-consent 公钥对模式与 split-consent.py，不使用 enrollment-fixture。复用真实独立 root daemon/nobody frontend、服务器临时测试 profile/root/CA、persistent verifier/supervisor；协调进程只公共 readiness+精确 stop，不能代签、批准或 enroll，不与 daemon 并发打开 Store。EOF/其他指令/5 分钟超时失败；进程停止/join/零 ACL 与精确 Retire 才清理。未知进程生存保留程序/模式 backup，不盲恢复。

新流程先实际 Windows pinned desktop probe，再切空隔离数据库；两端 OS 签名注册/邀请/pending/拒绝/自批准拒绝/明确批准/同 grant/签名 grant_clock 校时和验签；无客户端绑定仍拒绝 network；OS owner 签名撤销，权威 status revoked 不回 grant；无设备私钥输入。SSH 仅私有管理转发、随机 CA 私有 bootstrap，非正式 trust 安装/最终产品。

Linux check 构建、两项 Linux 真组件 PASS 无 SKIP（纯公钥/operator协议，不访问控制面），上传测试 binary 已删；Python两项模拟断言 PASS，不算实机。未运行新双机，不注册客户端节点、不验 nodeproof/产品 TLS，不改线上服务/Store/Nginx。**下一步请 Windows 重新 READY 并保持，再运行 split-consent.py；成功后仍请用户 Enter/STOPPED。** 路径 H:\chatgpt\远程软件开发\测试安装包\windows-native-network；用户之前 STOPPED，不能假设仍活。详见[新入口](../poc/selfhost/public-server/SPLIT_OS_CONSENT.md)。整体4未完成。

## 2026-10-07 切片 4C-4F：私有前端网络路由与跨进程HTTPS通过

device-frontend增加显式--private-network；默认network404，启用必须loopback+独立broker，无standalone/public监听回退。沿用profile/root/service/floor/TLS/kernel UID/issuer核对，frontend不拥有Store或privileged CLI。私有模式WriteTimeout20秒覆盖5秒读+14秒network执行，默认仍5秒，device预算4秒不变。请求body/frame/JSON/配额/socket/slot限制保留。

全量Go race/vet及Linux新frontend/check构建通过；Mac新测试先修正编译问题后通过。实际Linux两项新组件PASS无SKIP：loopback/broker强制、6秒模拟network操作正常返回503且期限13–14秒、严格JSON/大小写别名/body、unknown404、broker停止无回退503。模拟executor不算真实后端验收。

扩展daemon-network.py --frontend，最新完整真实Linux验收退出0。root独立daemon+实际Headscale verifier，nobody独立frontend持TLS材料，无authority，随机回环端口；服务器随机测试root/CA/profile，客户端root/service/floor验签+CA验证。内存随机设备Ed25519（非OS signer）HTTPS注册200/重放403、注册设备签名无grant network403非503；重复/尾随/8193body400、health404、错误API CA拒绝。停daemon前端仍活且device/network503；正常恢复仍拒绝注册重放。

真实stop PoC provider触发verifier/daemon非零故障退出，前端network503；provider恢复继续503、不自动重建daemon。原key/IP显式恢复后frontend重连IPC，无grant仍403、重放仍403。最终daemon/frontend正常STOPPED/零退出/join，注册之后Store字节不变，无额外issuer/设备状态重建。首次末尾runuser包装TERM退出判断失败不算完整PASS；改SysProcAttr Credential直接非root启动并对真实PID停止后复测通过，同时scanner读完stdout后Wait防末行丢失。

每轮精确Retire与实验目录/上传程序清理，原file模式/原PoC规则恢复；独立SSH三服务active、state哈希仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca、Nginx全部OK、nodes/users/preauthkeys零。未安装线上binary/unit、未改在线Store/profile/业务443/公网路由。

本轮未批准grant、未验双机OS signer/nodeproof/产品TLS，不算最终产品。下一步把双机完整流程接到独立daemon+frontend而不是旧fixture（届时才需Windows READY），再做受控部署；公网admission、安装锚/floor/轮换、HMAC安全投递、GUI/安装仍待，整体4未完成。见 [私有前端验收](../poc/selfhost/public-server/PRIVATE_FRONTEND.md)。

## 2026-10-07 切片 4C-4E：真实独立daemon跨UID/故障恢复验收通过

新增daemon-network-check Linux程序及daemon-network.py协调入口；上传check/最新daemon/mode工具并验哈希，在空的PoC控制面临时切database后运行。服务器随机生成隔离材料，未使用或上传本地发布根/CA私钥、未复制线上Store。独立root daemon，nobody为准许peer、root作为错误peer；nobody核对root kernel UID与精确issuer，network无grant403非503、不能读现有Store/profile/node私有文件，root客户端内核UID拒绝。

最新完整实机退出0：真实verifier创建后Stop，daemon只重开→Ready/精确零ACL；正常TERM与STOPPED/零退出，同key/IP恢复；再次启动后真实systemctl stop remoteapp-poc触发Runtime健康故障，daemon FAIL/非零并join，恢复provider不自动重启broker；同verifier身份恢复、显式第三次启动成功，私有IPC仍通过，issuer/设备Store字节不变。最终正常退出零ACL，精确Retire本轮ticket，实验节点/users/preauthkeys零，临时目录与上传程序删除，原file模式/100.120.0.2→100.120.0.1:47476恢复。

前两轮失败不计PASS：实验目录位于既有PoC 0700私有父目录导致非root无入口，改独立/tmp，不放宽既有目录；真实控制面故障暴露4C-4D以Runtime.Stop健康错误判定未join，保留writer.lock导致显式重启拒绝。新增Joined独立原子标记：supervisor+Runtime确认退出后，健康/ACL错误仍报非零但Close Store；只有实际未join才保留writer锁。补joined_fault回归；writer.lock为O_EXCL持久文件，不会随OS退出自动删除，下方4C-4D旧判断已被本记录更正。失败轮确认进程退出，无socket/verifier owner后按精确journal Retire并恢复原模式，清理孤立测试目录/持久测试writer标记；没有自动删除线上锁。

本轮全量Go race、vet及Linux构建复测；Linux专用零ACL纯断言拒绝缺字段/null/额外规则。独立SSH三服务active，线上State哈希仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca，Nginx全部OK，控制资源零。未改线上binary/unit/Store/profile、业务443或公网路由。

本轮不涉及Windows/设备grant/真实客户端nodeproof/产品TLS，不把4C-4C fixture结果算新daemon全链路结果。下一步受控前端私有network路由与跨进程可信HTTPS/故障503验收，再安排独立daemon双机授权/nodeproof/TLS与部署；公网admission、安装根/floor/轮换、HMAC安全投递、GUI/安装仍待。整体4未完成。见 [实际记录](../poc/selfhost/public-server/DAEMON_NETWORK.md)。

## 2026-10-07 切片 4C-4D：daemon可选网络装配与Linux组件通过，未部署

用户确认本轮Windows已STOPPED，4C-4C最终桌面停止确认闭环（无额外退出码/详细清理输出），下方待确认记录为历史。

新增authority-daemon可选 `--network-config`；原默认device-only行为不变，frontend network仍404。可信同UID0600单行严格配置/父目录0700，verifier/policy已有0700绝对无symlink、Store/配置/socket目录不可重叠。固定Headscale CLI/control/database模式，程序不切模式。现网仍file，禁止直接追加参数。

仅重开既有verifier(create=false)，无Mint/rekey；Runtime overlay/Store可信公钥nodeproof与supervisor装配、1秒维护，首投影/维护Ready才broker监听。后端故障关闭broker并错误退出，不自动恢复；正常TERM先broker请求join，再supervisor join/零ACL确认，再Runtime stop，最后Store close。join/零规则确认或Runtime停止失败不关闭Store、不报正常STOPPED，journal保留；OS最终进程退出释放锁不能当清理成功。Runtime正常停止顺序不由父context抢先关闭，原有30秒重开预算仍在。

全量Go race/vet通过，Linux amd64新入口构建通过；实际Linux三项隔离组件PASS无SKIP（严格配置；真实Store/kernel socket+替身owner四种生命周期；实际入口缺verifier不创建）。不调用真实Headscale，不能算daemon真实网络后端/跨UID新验收。临时测试程序已删除，本地构建可恢复。只读复核authority/frontend/poc均active，线上State哈希仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca，Nginx全部OK，nodes/users/preauthkeys零。

未安装新binary/unit、未创建长期节点/切Headscale database、未改线上Store/profile/公网路由。下一步隔离环境准备既有verifier，真实daemon+不同UID broker/故障退出/同身份重启验收，再决定前端网络路由与受控部署。公网admission、安装锚/floor/轮换、HMAC安全投递及GUI安装仍待，整体4未完成。见 [装配与边界](../poc/selfhost/public-server/DAEMON_NETWORK.md)。

## 2026-10-07 切片 4C-4C：新版真实双机验收通过，Windows最终STOPPED待确认

用户重新READY后，使用现有windows-native-network包和实际pinned probe，执行新版 `enrollment-network.py --supervised`，最终完整退出码0；没有沿用旧--native结果。真实persistent Runtime、Unix kernel broker、supervisor、Store可信公钥/overlay nodeproof、两端OS signer设备注册/邀请/拒绝/明确批准及签名授权全链路通过。fixture同root/UID的kernel IPC不等同线上不同UID新后端部署，正式network接口仍未开启。

实机通过：错误Mac应用身份真实challenge拒绝且不能投影规则；同节点重启；精确单向47476批准规则、正向载荷/反向拒绝、零ACL三轮真实拒绝且target无收到载荷；实际Swift/C#产品TLS错pin/错HMAC/认证前PING零保护载荷，正常TLS持续超过初始lease。周期维护保留正常TLS；policy命令故障时拒绝签名刷新，旧ACL仍在TLS关闭，provider恢复不自动开门，显式Stop/join并新backend/supervisor恢复。真实Store关闭同样拒绝刷新/旧ACL仍在TLS关闭。

撤销经Windows真实OS签名HTTPS：双方立即拒绝仍有效positive父快照重放，自主观察撤销且watch退出；退出后的旧快照不能重开，TLS关闭不是client timeout。周期回收两份record并下发零规则、maintenance取消join；冷重启不能复活已撤销grant。真实verifier SDK stop/join后Live/Ready=false、拒绝新state；显式同key/IP resume+新supervisor仍不复活撤销grant。所有helper/API/TLS/fixture正常零退出，owned控制资源清零。

本轮三个前置失败不记PASS：首次绑定后旧断言要求inactive，新supervisor可能已投影，改核对精确绑定与实际最小ACL（或inactive+零ACL）；后续撤销退出等待沿用2秒而真实HTTPS预算15秒，supervised验收改20秒且仍严格要求双端deny+join，不当生产时延指标；再后续3秒snapshot在join后过期，改撤销后立即验证fresh重放拒绝，退出后另验不能重开，不延长授权。最终完整复测通过，Python4+2测试通过、diff检查通过；本轮未改Go服务实现，先前race/vet证据不冒充新执行。

独立SSH复核 nodes/users/preauthkeys均null（零），原file policy/100.120.0.2→100.120.0.1:47476恢复；authority/frontend/poc均active，线上state哈希仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca，Nginx基线全部OK。正常清理Mac随机身份/本轮节点目录，Windows桌面owner仍属于用户，未远程停止/删除；已请用户Enter并回传STOPPED，**尚无本轮Windows最终停止确认**。

下一步先收Windows STOPPED，再推进独立权威daemon的持久verifier/真实backend装配与受控部署（先隔离验收，不直接公网开network）。公网admission/可信安装根与floor/轮换恢复、产品HMAC安全投递、GUI/安装仍待；300秒自然grant到期、断电/崩溃恢复、DERP/直连分类未新增证据。整体4未完成。下方“新版待验”为历史。

## 2026-10-07 切片 4C-4C：新版双机协调入口代码完成，实机待验

用户确认Windows已停止，先改造入口，未改服务器。enrollment-network.py新增--supervised（native/managed/TLS仍真实OS），启动fixture --native-supervised并要求ready supervised=true。新supervisedFixture用persistent Runtime+kernel broker+PrivateNetworkHandler+supervisor；可信Store取pub、runtime proof/report固定标签。早期一分钟维护以显式控制zero/reconcile阶段；周期实验切500ms。Stop后显式新backend/supervisor才恢复，policy服务恢复不能自动开门；Store故障Stop临时policy不可用保留旧ACL，便于TLS关闭验收。撤销接受签名inactive或拒绝（周期可能已清binding），仍禁止active，旧ACL关闭证据由fault阶段区分。末尾真实SDK stop/join+same key/IP explicit resume，已撤销grant不可复活。

constructor未知Mint保留Runtime journal；teardown join/资源cleanup/retire失败保留实验目录/lease而非盲删，不宣称零退出。新fixture自动测试：替身控制面、真实Runtime生命周期/HTTPS/kernel注册重放/策略fault锁闭/显式恢复/冷Store重放通过；Python纯断言3测试防缺marker/混入active/错误恢复。完整Go race/vet/Linux fixture构建通过。未运行新的完整真实Headscale/双机TLS验收，未新增自然到期/断电证据。

下一步用户正常桌面重新运行已有windows-native-network/start-interactive-vault.ps1，确认READY并保持窗口，总控先probe然后使用 REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\chatgpt\远程软件开发\测试安装包\windows-native-network' python3 -B poc/selfhost/public-server/enrollment-network.py --supervised。需最新Linux fixture构建和既有匹配helper/API/OS vault；测试未成功不能报完整通过，现场可能需按实际输出修复。运行结束确认所有owned节点/users/preauthkeys零、原policy/Nginx恢复，再请用户Enter/STOPPED。线上network仍404、整体4未完成。见[新版入口](../poc/selfhost/public-server/SUPERVISED_NETWORK_TEST.md)。

## 2026-10-07 切片 4C-4B：私有HTTP/broker验收前置与实机组件

接续4C-4A发现StartHeadscaleNetworkSupervisor漏PolicyDir/Mode，实际driver会首Reconcile拒绝；新增可信真实绝对0700 policyDir参数+database配置，不自动改Headscale模式。broker仅network固定14秒工作/15秒I/O，解析仍5秒、其他操作4秒，兼容12秒nodeproof，不放宽槽/帧。新增PrivateNetworkHandler device/network均kernel IPC、严格JSON/完整包8191限、issuer核对/503无回退；生产device-frontend不接此路由。

Mac新HTTP/期限race测试通过。实际Linux五项隔离组件测试均PASS无SKIP：PrivateNetworkHTTPSViaKernelBroker、NetworkBudgetBoundedWithoutChangingDeviceBudget、SupervisorPolicyFailureLatchesNoRecovery、SupervisorVerifierFaultCancelsMintNoCredentialAndJoins、SupervisorFailedWithdrawalNeverReportedAsACLSuccess。替身driver/verifier+临时Store，不调用Headscale，不冒称真实proof/双机TLS。三个线上服务active、state既有哈希不变、Nginx全部OK。

用户提供 Windows H:\chatgpt\远程软件开发\测试安装包\windows-native-network\interactive-bootstrap.json，存在324字节。实际同用户pinned probe PASS无SKIP，确认桌面OS signer可用；初次Get-Process按旧名称未匹配不能推断进程未运行，最终以实际probe为准。本轮没停止用户桌面owner/删除Windows身份。

**真实双机新版runner尚未完成/运行**：旧--native直接backend且期待自动恢复，不得原样重跑当新证据。下一步改隔离fixture为Runtime+kernel broker+supervisor，协调流程policy故障后显式重建新backend/supervisor、join后恢复，再零ACL/TLS/撤销/到期完整验收；不改线上device-only。Windows可能需要重新READY再probe，不擅自重建/代签或沿用过期scope。整体4未完成。详见[新入口边界](../poc/selfhost/public-server/NETWORK_SUPERVISOR.md)。

## 2026-10-07 切片 4C-4A：后端 supervisor / runtime nodeproof 组件

新增StartHeadscaleNetworkSupervisor固定CLI/RuntimeIP/Store.DevicePublicKey/runtime独立overlay nodeproof装配；Runtime Verify有12秒期限及生命周期取消，proof join后才能overlay.Close/解owner.lock。supervisor先同步Reconcile再维护，首成功周期前不可用；backend专用未运行。通过Supervisor.Authority接broker：fault锁闭、取消维护/请求、join/Fence、最佳努力显式零ACL；取消返回credential不回包。driver健康用原子latch，避免底层mutex阻塞fault观察，policy/proof故障不自动恢复；必须检查后显式新backend实例。

组件race测试：policy故障/恢复仍关闭，verifier fault取消执行中Mint/secret不回包，Stop超时不假join，释放后join；最终零策略失败不报告成功。Runtime proof替身测试：取消未join保留lock/overlay，停后拒绝新proof，退出才close/unlock，取消后true拒绝。Stop成功=join，DenyAllConfirmed=driver最终投影成功而非真实网络拒绝；旧ACL/既发lease可能仍存，不宣称直接关闭全部TLS或100ms严格时延。

完整Go race/vet和Linux三个服务入口构建通过，最终broker/verifier组件追加测试复测通过。未将构建等同Linux实机supervisor验收。

**本轮无服务器修改/新verifier/Headscale ACL变化，线上保持4C-3D device-only，network404；整体4未完成。** 下一步隔离实验Store+临时真实verifier+kernel broker+签名客户端+真实Headscale/nodeproof全链路，零ACL/故障/撤销/到期及旧ACL留存时TLS关闭验收后才推进长期daemon授权。见[组件与待验边界](../poc/selfhost/public-server/NETWORK_SUPERVISOR.md)。

## 2026-10-07 切片 4C-3D：服务器权威/前端受控迁移通过

用户“继续”后完成线上检查与迁移。新增migrate-authority.sh：核对新binary哈希，正常停旧remoteapp-device、listener与writer.lock释放，root-only备份线上最新Store及旧binary/unit，再原状态移动至/var/lib/remoteapp-authority（root700/state600）；可信profile在/etc/remoteapp-authority root700。没有上传/覆盖本地初始snapshot，没有发布根/CA私钥上传。备份保留 /var/lib/remoteapp-authority-backup.WUGsLoM5，含在线issuer，禁止日志/外传；回滚不可恢复过时state/不兼容旧binary。

新remoteapp-authority root CAP_CHOWN=1、remoteapp-frontend remoteapp-pre UID995/GID993 capabilities0，两者NoNewPrivileges=yes，active、disabled（未开机启用）；旧remoteapp-device停止。socket root:remoteapp-pre目录0710/socket0660，tmpfiles规则已安装。真实frontend用户不能读issuer/profile或存在的Headscale db.sqlite/config、不能写socket目录/var/lib；frontend目录无authority残留。第二authority实例固定FAIL（预期）且正常实例/锁不受影响。

真实TLS签名profile/CA/域名+malformed403/duplicate400/network404/health404通过；单独停frontend保留authority锁，重启通过；authority正常TERM退出锁/socket释放，frontend仍active，authority-down探针严格要求503通过，重启authority后默认403探针恢复。线上state所有阶段哈希5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca未变；issuer未变，无设备注册/nonce持久写。durable replay证据为4C-3C联调，不冒充线上新增验收。探针本次服务器直接loopback，旧打印SSH forward不准确，已改explicit loopback并输出预期状态。服务Result success/ExecMainCode0/ExecMainStatus0，Nginx全部基线OK、remoteapp-poc active。

未改公网8444/代理/业务443/Headscale ACL/系统Tailscale；无断电/崩溃锁或实际备份恢复验收。整体4仍未完成，network仍404；下一步接持久verifier/新鲜nodeproof/maintenance/fence backend，先回环真实拒绝/零规则/故障测试，不能只凭NetworkReady布尔值开放。详见[迁移实际记录](../poc/selfhost/public-server/AUTHORITY_MIGRATION.md)。

## 2026-10-07 切片 4C-3C：daemon/frontend 入口及迁移前置

新增 authority-daemon 和 device-frontend：前者核验固定 root/service/floor/签名 profile/既有 Store issuer，独占已有状态、到期关闭，不创建新 issuer，无 TLS 和 network backend；后者仅 TLS/严格JSON/预算与固定IPC，不打开 Store，启动和成功回包核对 issuer，缺权威服务503，不回退。identity IPC仅公钥；双向kernel UID边界沿用4C-3B。

新增 Unix + HTTPS 联调：前端目录已移除authority，注册/重放/重复JSON、两进程分别停止/重启、单writer、authority退出503、不重建state、错误server UID及缺state拒绝。单UID Mac测试不能等同Linux root/non-root部署验收。新增两份未安装unit模板与受控迁移/备份/回滚说明；Store必须移出frontend可写父目录，使用线上最新状态，不得用本地初始snapshot覆盖；写入后不可回滚旧snapshot/不兼容旧binary。

完整 Go race 回归与 vet 通过，Linux amd64 两个新入口构建通过。首次联调暴露Mac socket路径过长，改用短路径后复测通过；没有将此失败误记为权限拒绝验收。尚无本轮Linux新unit/实际进程隔离证据。

**尚未切换线上服务或迁移Store，当前remoteapp-device保留；network404、整体4未完成。** 下一步在CentOS核对材料/权限，正常停机释放锁、备份最新线上state、迁移至root独占目录、渲染unit，实机非root读写拒绝+HTTPS/重启/503+Nginx基线验收。详见 [迁移边界](../poc/selfhost/public-server/AUTHORITY_MIGRATION.md)。

## 2026-10-07 切片 4C-3B：固定操作 broker/设备公钥绑定组件完成

新增 broker typed device/network authority/client/server，仅固定签名操作，无命令/路径/任意脚本。Authority 单 Store 重验签，network 默认503，需同一 Store backend+可信 NetworkReady+已绑定公钥再验 network signature/grant。NetworkReady 不替代 supervisor cancellation/fence；尚未接 verifier/maintenance/Headscale，不宣称故障自动关闭产品链路。Store device 新增可选 PublicKey，成功签名 register 后绑定并保存；getter 校验 deviceID hash/返回副本/关闭与旧记录拒绝，重开错误绑定拒绝。旧设备需明确 re-register（设备码不变）；旧二进制不兼容新字段，不能盲目降级。

Unix 双向真实 kernel UID（Linux SO_PEERCRED、Mac LOCAL_PEERCRED）在解析前核对，不信任客户端UID字段；真实绝对 broker-owned 目录、固定GID、无group/world写、0660 socket、碰撞拒绝。8连接/4工作槽、单帧8KiB+半关闭/EOF、重复别名未知尾随/超长拒绝、期限/取消关闭连接与worker join。其他平台 fail unavailable，不回退TCP。未来 root socket group仅供frontend连接，仍匹配精确UID。

Mac race IPC/签名/公钥/饱和取消及完整Go race/vet通过，Linux/Windows专用vet和Linux制品通过。真实服务器固定四父测试均PASS无SKIP，其中root->nobody跨用户：实际组权限能连接socket、kernel确认root server，但请求UID不符、executor0。仅临时fixture Store，未触线上Store/Headscale/ACL。清理本轮测试制品stage及fixture，Nginx基线通过、device/poc active。Linux制品非race，不伪称Linux race或Windows新增broker实测。

**组件还未部署接管线上服务/未迁移Store owner，网络仍404，整体4未完成**。下一项authority daemon/无特权HTTPfrontend适配+单Store停机迁移/备份回滚；frontend不得读issuer/state或访问HeadscaleDB。再接verifier/nodeproof/maintenance/fence，回环真实网络与零规则验收后部署network；公网/安装根floor/HMAC/GUI安装仍待。见 [broker验收边界](../poc/selfhost/public-server/AUTHORITY_BROKER.md)。

## 2026-10-07 切片 4C-3A：持久 verifier 生命周期/真实服务器验收通过

新增 verifier/verifier-owner；显式 create/resume/retire，planned-before-Mint/issued/ready/retired owner-only journal，不持久化注册 secret、不自动重试 unknown Mint；恢复同一 StateStore 节点及 user/node/IP 绑定，不新 Mint；缺失/损坏/已退休拒绝。独占 owner.lock，Stop/fault 先 Live/Info.Healthy=false，再监控取消/SDK关闭/join 后解锁；超时保留锁，崩溃遗留锁不自动接管。周期有界 SDK健康+CLI绑定异常 latch，不自动恢复/换身份；不把健康/控制行当节点实时证明，没有调用 ReplaceRules/Verify/批准。

完整 Go race、专用 vet、Linux 静态构建通过。真实服务器两个隔离 verifier：第一份 create 正常 READY/STOPPED 0，同身份 resume READY/STOPPED 0、journal 哈希稳定，幂等 retire 通过；第二份 resume 后删除仅其 journal 的真实 NodeID，进程固定 FAIL/退出1（预期），不重建、释放锁，再 scoped retire。初次30秒窗口已正常停止导致删除前置锁检查失败，没有删除发生；90秒 resume 窗口完成实际故障注入。最终独立 nodes/users/preauthkeys 均0，Nginx基线通过，device与poc服务active，无ACL/业务443/系统Tailscale/用户密钥更改。已删除仅本轮 retired 随机身份/工具临时目录，测试节点私钥不可复用；预发布服务材料未删除。

**未正式部署长期 verifier，未接通设备服务网络授权**；/v1/network 仍404。健康检测不代表严格1秒控制新鲜度，实际 Bind 仍须 nodeproof+设备签名。下一子切片固定操作 server-local broker/Unix OS peer credentials 与单 Store owner（frontend 不得获 root/HeadscaleDB）；补设备公钥可信持久绑定，再接 verifier/maintenance/fence 和真实backend/零规则验收。安装根/floor/HMAC/GUI/安装继续待收口，整体4未完成。见 [verifier边界及实际结果](../poc/selfhost/public-server/PERSISTENT_VERIFIER.md)。

## 2026-10-07 切片 4C-2C：用户授权在线私钥上传，独立回环服务部署/HTTPS/重启通过

用户明确回复“允许上传”，授权将预发布 TLS 私钥和在线 issuer 状态上传其 182.92.117.114；**发布根/CA 私钥未上传**。scp 仅 server-private/client-public/二进制/unit/部署脚本。实际执行首次隔离部署，新增专用 remoteapp-pre（nologin）及 remoteapp-device；/opt/remoteapp-prerelease 程序和 /var/lib/remoteapp-prerelease 状态独立于原 PoC。服务 active、disabled（未开机启用）、仅 127.0.0.1:8444，无公网监听/代理/安全组改动。

CentOS 7 实机确认 NoNewPrivileges=yes、CapabilityBoundingSet=0、专用 User/Group，ReadOnlyDirectories=/var/lib/remoteapp-prerelease，仅 authority 状态 ReadWriteDirectories；TLS 私钥/state 均 600 remoteapp-pre。通过临时 SSH forward 两次 device-service-probe（首次/重启后），发布根验证 profile/独立 CA/真实域名 TLS 和 malformed403/duplicate400/network404/health404 均 PASS，无注册/设备持久写入。此转发只是管理验证，不证明公网或最终 Headscale 数据通道。

实际 systemctl stop 后监听消失、writer.lock 移除；状态哈希停止前/后/重启后相同；systemctl show 记录 Result=success/ExecMainCode=0/ExecMainStatus=0，随后显式 start 和二次 probe 通过。未用 journal 中缺失的 STOPPED 行杜撰输出。Nginx 主配置及所有 conf.d 哈希核对全部 OK，remoteapp-poc active；服务最终 active。Headscale 节点/策略未修改，也未进行本轮新网络授权测试。

本轮新 staging /tmp/remoteapp-device-deploy.Jc4DTtmk 明确核对文件后删除（含冗余在线私钥副本；正式部署材料和本地原件保留）；非秘密 Nginx 基线另存 root-only /opt/remoteapp-prerelease/nginx-baseline.sha256。临时 SSH 转发已 Ctrl-C 关闭（SSH 255 为该主动结束，不是服务验证失败）。线上 Store 从此为 authority，严禁用本地初始 snapshot 覆盖线上 state。下方“上传授权待确认/未部署”为历史。

下一步服务 verifier 的长期身份/重启恢复/故障撤销和真实 Headscale backend，network 路径继续 404，不能把设备 grant 当网络授权；再公网 admission/8444、可信安装根/floor 持久化轮换、HMAC 安全投递、产品 GUI/安装。整体切片 4 未完成。见 [设备服务实际部署](../poc/selfhost/public-server/DEVICE_SERVICE.md)。

## 2026-10-07 切片 4C-2B：独立设备服务代码完成，私钥上传需具体授权

新增 deviceservice/device-service，固定运维 release root/service/floor 校验预发布 profile、TLS 证书/CA/域名/期限/私钥、实际 Store issuer；仅重开既有 state、缺失拒绝。TLS1.3/HTTP1.1、32 socket/4 work slots、process admission 与既有 durable 签名/预算/重放；到期/信号停止监听及正常 HTTP join 后 Close Store。设备-only，network/health 404，未挂 Headscale/Mint/Bind，不自动批准。HTTP 两个入口补重复/大小写别名/未知/尾随 JSON 拒绝，仍允许 pretty JSON。新增真实回环服务/重启保持 replay/Stop/到期/错误根 floor 端口与私钥权限/缺 state 不 rekey 测试；不代表安装/OS signer 新双机/公网验收。

新增 Linux 无特权 systemd 与 fail-safe 首次部署脚本、只读 SSH-forward profile HTTPS 探针。尝试 scp 被安全审查拒绝（具体在线 TLS/issuer 私钥外发目的地授权不足），**上传/安装未执行**。本轮只创建服务器空 staging，然后用 rmdir 删除；无新账号/服务/Nginx/端口/Headscale/业务443变更，未删除任何密钥。需用户明确同意上传 `server-private` 中 TLS 私钥/在线 issuer 私钥至 182.92.117.114 专用服务；绝不上传本地 release-local-only 发布/CA 私钥，也不能通过其他传输绕过拒绝。此授权后才进行真实回环部署和服务停止重启/证书验证。

后续服务 verifier 长期身份与真实 Headscale backend 尚未实现，网络授权继续关闭；再公网 admission/8444、安装根/floor 持久化轮换/HMAC 受保护投递/产品 GUI 安装。整体 4 未完成。详见 [设备服务边界](../poc/selfhost/public-server/DEVICE_SERVICE.md)。

## 2026-10-07 切片 4C-2A：选择独立自托管预发布，材料已生成（未部署）

用户回传 Windows `VALIDATION PASSED: signed release profile + OS signer HTTPS. NOT production trust installation or full product acceptance.`，据此闭环 4C-1 Windows 四项实际测试，不记录未提供的耗时。用户选择“使用独立托管”，按独立自托管预发布推进。新增 `prerelease`/`cmd/prerelease-bootstrap`，拒绝已有目录、符号链接父目录、443/无端口/不合法 URL；0700/0600、CreateNew/Sync/READY-last，失败保留状态、不默默重生成身份。分别生成本地发布根和 API CA 私钥、server-private TLS/真实持久 issuer Store、client-public 签名配置与公开安装锚，测试确认实际 TLS/签名/issuer 一致和无覆盖。race 专用测试/trustprofile/vet 通过。

实际本机创建 Git ignored `artifacts/connection-poc/prerelease-v1`，服务 remoteapp-prerelease、epoch 1、候选 API https://mk.fengmap.com:8444、profile/叶证书 30 天、CA 90 天。发布私钥仅本地文件，**未加密 owner-only PKCS#8，不是 OS vault/断网离线/硬件保护**；绝不上传 release-local-only，也不压入用户包，生产前需保管备份恢复流程。server-private 含在线私钥；client-public 仅公开字段但不能自证安装信任。只读服务器 ss 确认 8443 占用、8444 未见监听，无上传/服务部署/端口放行/Nginx/Headscale/业务443变更。现有 invite-api 是回环 fixture，不能直接公开反代。

下一步先实现有界独立设备 API+真实 backend+最小权限持久服务，再部署 server-private/8444；同时收口可信安装锚与 floor 持久化、轮换恢复/HMAC 受保护投递/产品 IPC GUI 安装。整体 4 仍未完成，不让旧 fixture 测试充当新预发布实际验收。详见 [预发布材料边界](../poc/selfhost/public-server/PRERELEASE_BOOTSTRAP.md)。下方“待用户选择/Windows待回传”为历史。

## 2026-10-07 切片 4C-1：服务信任配置核心完成，整体 4 未完成

用户要求继续整体 4。新增发布根签名 `trustprofile`（域 remoteapp-service-trust-v1），绑定 HTTPS endpoint/API CA/独立 online issuer/service/epoch/有效期；8 KiB 严格解析，篡改/未知根/回退/同 epoch 换配置/到期拒绝。`Advance` 仅验证，**不等于安装锚/floor 持久化或跨重启防回退**。新增 `NewProfileNetworkClient`，原构造器仍限制回环；profile 到期拒绝请求并以独立单调期限取消 HTTP/ManagedGate，Deny/join。helper 和 device-api-client 私有 ServiceProfile/ReleaseRoot/Service/MinimumEpoch 模式只允许 OS signer，拒绝 raw endpoint/cert/issuer override、AppKey、孤立配置。

Go 全量 race/vet、Windows helper/API/trust 编译与专用 vet 通过；本机真实 HTTPS 正负向及签名/轮换边界通过；真实 Mac Keychain→发布签名 profile→独立 CA→HTTPS 验签 PASS（0.15s），随机测试身份精确清理。Windows 独立 `windows-service-trust.zip` 已生成、PowerShell AST 语法通过，**Windows 四项实际执行待回传**。无服务器部署/配置变更、生产根创建、已安装 GUI/安装包改动；profile 发布根/issuer/CA 测试均在内存，native network runner 尚未切换到新 profile 模式。

下一步需要用户选择创建独立自托管预发布配置（推荐，不改业务443）或提供既有正式配置；已用非阻塞问题请求确认，未默认生成/部署生产发布身份。然后收口可信安装锚/floor 安全持久化与轮换恢复，产品 HMAC 受保护投递，Swift/C# IPC/生命周期、GUI/独立安装及实机验收。**不能宣称整体 4 完成或最终免登录产品已交付。** 新入口与边界见 [服务信任验证](../poc/selfhost/public-server/SERVICE_TRUST_TEST.md)。

## 2026-10-07 切片 4B-4B：真实 OS 身份双机网络验收与清理闭环

用户使用新 `windows-native-network` 包保持桌面 owner，提供中文目录 READY。实际执行 `REMOTEAPP_WINDOWS_INTERACTIVE_DIR=... enrollment-network.py --native`，完整退出码 0。独立 Windows pinned probe 先通过，authority 确认只有两端固定 OS 公钥，无 target_key/controller_key；设备 HTTPS 注册、邀请单次兑换、拒绝/再次明确批准、issuer/grant 校准均使用真实 OS signer。无批准无授权、未知设备码/复用验证码/自批拒绝。Mac 负向使用另一份真实随机 OS 身份，不是 fixture AppKey。

实机通过：注册前节点 key/OS 签名意图、实际 Windows/Mac 网络+应用实时持有证明、错误身份真实挑战拒绝、同节点重启；只投影单向批准端口、实际正向载荷/反向拒绝、零 ACL 三轮拒绝且 target 无收到载荷、恢复正向。两端自主 HTTPS 校准/刷新，父 state 不可授权、token probe 禁用；发布 TLS RSA 持有证明由 target OS app signer 签名，controller 验新鲜 issuer 描述。真实 Swift/C# TLS 的错 pin/错 HMAC/认证前 PING 均零保护载荷，正确连接持续超过初始 lease。

生命周期实机通过：helper watch stop/join 关闭活动 TLS，双方无新增 PONG且不是 client timeout；周期策略命令失败保留旧 ACL 时 TLS 仍关闭，周期恢复/冷 Store 重开+显式 helper 重启恢复同一绑定；真实 Store 关闭时不能刷新、TLS 关闭；OS 设备 HTTPS revoke 后两端自身观察终止，无父 inactive 投递，尚有效旧正向 state 不能复活，旧 ACL 尚在时 TLS 仍关闭。周期回收/零规则/所有 helper、API、TLS 与 fixture 正常零退出。

正常 teardown 完成两份 Mac 随机 OS 身份的 delete/零退出与节点状态回收，恢复原 file policy/原 PoC policy，service active、业务 Nginx 哈希一致。独立 SSH 再读 nodes/users/preauthkeys 数量均 0；未新增公网端口、未改系统 Tailscale/安装 GUI/生产配对身份。Windows 桌面 owner 属用户，本脚本只关闭自身 relay；在要求按 Enter 并回传 STOPPED 后，用户回复“通过”，据此记录本轮 Windows 正常停止/随机测试身份清理验收通过，无附详细输出，不记录额外退出码或耗时。本轮网络与清理验收闭环，不复用已删除的测试身份。

边界：这是 **私有协调 fixture 的真实 OS 全链路 Headscale/产品 TLS 验收**，不是最终 GUI/安装免登录产品。API 根/issuer/HMAC/RSA 产品测试身份仍由可信测试 bootstrap 提供，确认仍由协调器发起；正式信任锚安装/轮换/恢复、HMAC 可信投递、产品 IPC/GUI/独立安装仍待。Windows SSH 产品测试 65/65、默认产品 DPAPI 明确 SKIP，不能当默认 66 项通过。未新增数据直连/DERP 分类证据或整个 daemon 断电/300 秒自然到期测试；整体切片 4 未完成。见 [实际验收记录](../poc/selfhost/public-server/NATIVE_NETWORK_TEST.md)。

## 2026-10-07 切片 4B-4B：协调入口实现记录（下方待验为历史）

用户确认前一轮按 Enter 后 STOPPED，桌面 owner 正常退出与随机测试身份清理验收闭环。新增 `enrollment-network.py --native`、`native_bootstrap.py` 及 `enrollment-fixture --native`。Windows 先 SSH pinned probe，严格匹配本机公开 bootstrap/包路径，helper/API 使用 relay+固定 scope/public；Mac 创建随机 controller/独立错误身份两份 OS 凭据，helper 重开 signer，最后 join 后删除。authority 只接两端 public，不生成/输出应用私钥，不预注册/批准 OS 设备，native 模式拒绝特权代签 enroll/revoke；正常授权与撤销均走 OS 签名 HTTPS。父管道不调用 OS sign 或刷新 state。异常特权 fixture teardown 先零 ACL 再清理自己 journal 节点，不伪造设备 revoke。

Bridge 原 30 秒空闲期限会影响较长协调验收，改为有界 10 分钟空闲读取，stop 仍立即关闭连接，不延长在线 gate/grant。新增独立 `windows-native-network.zip` 保留 UTF-8 修复和严格 hash/no-SKIP 脚本。Go 全量 race/vet、native 公钥拒绝测试、Python 两组角色/bootstrap 回归及 Mac/Windows helper/API/bridge、Linux fixture 编译通过。**未实际运行 --native 或部署服务/改服务器策略；Windows owner 已停止，待用户新包保持启动 READY 后由 Mac 执行完整网络验收。** 不得合并之前 fixture 与 OS 本机结果宣称通过。步骤/边界见 [native 双机入口](../poc/selfhost/public-server/NATIVE_NETWORK_TEST.md)。正式 API/issuer 信任分发、HMAC 投递、GUI/安装与整体切片 4 仍未完成。

## 2026-10-07 桌面保持启动与同用户 SSH 桥接探针实际通过

用户在中文路径 `H:\chatgpt\远程软件开发\测试安装包\windows-interactive-bridge` 使用更新脚本，截图确认 READY 并保持正常桌面会话。随后通过既有 jarvis SSH 执行本包 `probe-interactive-vault.ps1`：`TestPinnedInteractiveVaultFromBootstrap` 真正 PASS（0.02s），无 SKIP，`PROBE PASSED`，SSH 进程退出码 0。两次重开同一 pinned 桌面 native 身份及 HTTPS/enrollment/node/TLS 域签名通过，无 SSH 内新建身份/导出私钥。首次直接调用脚本被执行策略阻止；使用子 PowerShell `-ExecutionPolicy Bypass -File` 通过，不改变永久策略。

用户尚需在保持启动窗口按 Enter，回传 `STOPPED` 确认该次实际持有进程退出/随机测试身份回收；本次 SSH 探针只关闭自己的 relay，未停止桌面 owner。Windows 桌面与 SSH 桥接功能验收通过，**完整 Headscale 双机 OS signer 协调器/真实产品 TLS 联合验收仍未实现或通过**。后续须改 authority 只接公开 OS 身份及网络 runner，不合并 fixture 证据。下方待验描述为历史。

## 2026-10-07 桌面启动脚本中文路径编码修复

桌面桥接测试用户已确认通过，但保持启动脚本报 `Interactive vault unavailable`。只读实机诊断确认 Windows PowerShell 的 Process.StandardInput 默认编码为 gb2312；原 WriteLine 将中文 VaultExecutable 路径写成非 UTF-8，native JSON 读入后路径损坏。改为直接 BaseStream 写入显式 UTF-8/no-BOM/换行并 Flush，初始化与 stop 共用函数；增加固定阶段/状态/退出码诊断及避免启动退出竞态的二次 broken-pipe 掩盖。不改 exe/身份存储策略，不导出私钥、不重置凭据。

实际 Windows PowerShell 加载新脚本的 writer 函数，发送中文/空格字符串到临时子进程并逐字节核对 UTF-8+换行通过；该检查不执行 broker、不创建凭据或写远端文件。新增 `test-launcher-utf8.ps1` 保存中文 JSON 路径传输回归。独立 zip 更新同一启动脚本，四份 exe/hash 不变。用户需替换脚本后复试 `start-interactive-vault.ps1`；READY/SSH 探针与完整 OS 双机验收仍待，不能用编码回归宣布通过。

## 2026-10-07 切片 4B-4A：Windows 桌面 OS 身份桥接入口（桌面与 SSH 通过）

为跨网 OS signer 启动补上 Windows SSH/交互式安全存储的会话隔离桥：桌面 `interactive-vault` 显式创建并持有本轮随机 native 身份，同用户 SSH helper 使用 `vault-relay` 重开固定 scope/public，私钥不导出、不退回 fixture/明文。仅随机本机 named pipe、当前用户 protected DACL、拒绝 remote client/名称碰撞，四连接上限、严格 8 KiB JSON/期限；拒绝 create/delete/export/任意消息，不让非法消息毒化持有进程。停止关闭连接/join 后精确删除测试身份；不抵抗同用户恶意代码或代替业务门禁。

提供独立 `windows-interactive-bridge.zip`、严格 hash/PASS/no-SKIP 包装脚本、桌面保持启动脚本及同用户 SSH pinned probe。不依赖旧安装包/SDK/管理员，公开 bootstrap 不含私钥，启动 CreateNew、不覆盖其他运行实例。Mac 全量 Go race/vet、边界/连接上限/取消 join 回归及真实 Keychain handler 两次连接/签名/精确删除通过；Windows x64 四制品交叉编译与专用 vet 通过。用户回复“验证通过”，按上一条指定的 `run-interactive-bridge-validation.ps1` 将 **Windows 桌面 named pipe + Credential Manager 桥接验收记为用户确认通过**；本次没有附详细输出，不记录耗时或逐项日志。**同用户 SSH pinned probe 仍待验**，不得与桌面结果混为一项。

这是联合入口的前置子切片，不是全部完成：`enrollment-network.py --managed` 仍用 fixture AppKey；下一子切片需改 authority 只接受两端 OS 公开身份，并接入显式邀请/确认、真实 Headscale 持有证明/自主 watch/真实产品 TLS。不得把已通过的 Windows 本机生命周期和此前 fixture 网络结果合并成 OS 双机验收。详见 [入口与步骤](../poc/selfhost/public-server/INTERACTIVE_VAULT_BRIDGE_TEST.md)。本轮无服务器部署/网络规则/安装 GUI/系统 Tailscale/生产凭据修改。正常 teardown 删除的仅是各次随机测试凭据，不可恢复，不涉及现有配对。

## 2026-10-07 Windows 原生生命周期独立入口（当前入口，双机入口尚未补齐）

按用户“不使用旧安装包”要求新增便携 `TestNativeVaultPortableLifecycle` 和 `run-native-validation.ps1`，制品为独立 zip，不改安装产品。脚本核对两份 exe 的 hash，必须有指定测试 PASS、无 SKIP、退出码 0；设置 vault 绝对路径并恢复原环境变量，无需 SDK/管理员。两个应用身份必须真实 OS vault，临时 issuer 在测试内存中；不依赖 Unix Store，也不放宽生产存储权限。

Mac 真实 OS 存储执行通过：两个身份重开、两端 HTTPS 自主刷新超过初始租期、签名撤销、双向会话证明/opaque 隧道、服务故障关闭与 worker join/无后续查询，正常精确清理随机身份。Go 完整 race/vet 通过；Windows x64 编译、包内 hash 核对清单/五文件 zip、Windows PowerShell 只读 ParseInput 语法检查通过。2026-10-07 用户回传 Windows 正常交互式实际验收截图：`TestNativeVaultPortableLifecycle` PASS（5.61s），四组 CHECK 均通过、无 SKIP，脚本输出 `VALIDATION PASSED`，确认内部测试退出码为 0、制品 hash 核对通过，测试凭据由 teardown 清理。Windows 本机原生生命周期项已闭环。本轮补齐的是 **Windows 本机 OS 集成入口，不是完整跨网双机 OS signer 联合启动入口**；那项仍待，不应宣布全部验证入口完成。详见 [入口清单与边界](../poc/selfhost/public-server/NATIVE_WINDOWS_ENTRY_TEST.md)。旧安装包、配对密钥、系统 Tailscale、服务器和防火墙未改。

## 2026-10-07 切片 4B-3：真实双机自主刷新/产品 TLS 门禁通过（当前入口）

新增 `enrollment-network.py --managed` 并实际完整零退出。沿用真实设备 HTTPS 申请/明确确认、注册前节点 key、网络节点与应用持有证明、动态单向 ACL/真实零 ACL 拒绝；通过后同身份显式重启两端 helper 到自主模式，Token 为空，API TLS 根/issuer 固定私有 bootstrap。父脚本没有状态刷新线程，也不让独立 device-api-client 开启 watch；每个网络 helper 自生成挑战校准、自己签名 HTTPS 刷新并验证 gate。

实机通过：两端自主 gate active；有效父管道 state 更新拒绝；旧 token probe 固定拒绝（不令进程崩溃）；真实 Swift/C# TLS 证书 pin/HMAC/认证前 PING 三项负向零保护 PONG，正确连接持续超过初始 3 秒 lease。可信私有 `watch_stop` 取消/join helper 自身 watch，真实 TLS 关闭且双方 PONG 不增长、不是客户端 timeout。新增固定 `managed_active`/`managed_done` 诊断不是授权依据；自主 state 请求只验签报告 fresh，不改变 gate。

周期 policy 故障时最后网络 ACL 尚在，helper 自取 inactive 关闭 TLS；恢复 CLI/周期自动重试、冷后端重开后保留同网络身份并由显式重启恢复。真实 Store 关闭、保留 ACL 时，两端自身 watch 终止、真实 TLS 关闭；服务恢复后显式重启。设备 HTTPS 撤销后，没有父管道 inactive 投递；独立确认两端 managed_done 且 inactive，再将尚有效的旧正向状态交给两端都得到 fresh=true/passed=false，不能复活；数据 TLS 已关闭，PONG 不增长。周期精确回收、显式零规则、全部正常退出通过。

Windows SSH 产品 profile 65/65（DPAPI 明确 SKIP，默认 66 项仍独立待验）；Go 全量 race/vet、managed 两轮竞态及诊断一致性回归通过。结束后节点/用户/preauthkeys 全部回收，独立 SSH 三类资源均 null（0）；原 file 策略/PoC 策略恢复，服务 active，业务 Nginx 哈希一致。无生产配置/安装包/系统 Tailscale/防火墙修改；仅删除本轮独立节点状态和临时授权资源，不保留测试身份供复用。

**边界必须保留：本轮两端 AppKey 均是隔离 fixture 测试密钥，不是 Windows Credential Manager/双机 OS signer 全链路实测。** Windows SSH 登录缺 Credential Manager 会话，未改变安全策略、未导出用户 OS 私钥、未用明文替代生产存储；之前 Windows 交互式 native 通过是独立证据。API 入口仍是私有 HTTPS+SSH 管理转发，HMAC/信任锚仍由可信 fixture 提供，不是最终用户连接方案。故障是 policy runner/真实 Store，不是整个 daemon 断电；本轮没有新验证数据直连/DERP 路径或 300 秒 grant 自然到期。复测与边界见 [4B-3 验收](../poc/selfhost/public-server/MANAGED_NETWORK_TEST.md)。

剩余 4B：正式安装 issuer/API 根信任与轮换/恢复，产品 HMAC/身份包可信投递，Windows 正常用户会话中的双机 OS signer 完整启动与真实网络验收，旧 Swift/C# IPC/GUI/安装收口。不能因为自主链路已通过就宣布整体 4 完成/进入公网发布。下方 4B-2 的双机待验为历史，OS signer 待验仍有效。

## 2026-10-07 切片 4B-2：helper 自主授权状态生命周期（当前入口）

用户提供 4B 新制品实际输出：`TestNativeVaultIdentityAndIsolation` PASS（0.35s），`$LASTEXITCODE=0`。**Windows 4B-1 的安全存储/新增六个签名域人工验收通过**，不是 SKIP；不是 Windows/Headscale 双机 OS signer 全链路验收，也不等于默认产品 DPAPI 66 项测试。

新增 `streamgate.ManagedGate` 与 helper 私有自主模式：可信初始化提供回环 HTTPS Endpoint/API 证书/独立 issuer/固定 GrantID。helper 用同一个设备 signer 自生成挑战、查询 `grant_clock` 验签/保守校准，并自行启动 StateWatch。必须从 HTTPS 获取匹配完整 grant/真实绑定的首个有效 active 状态，才创建 gate，不把父管道给的状态当初始网络授权。随后自主串行签名查询，不接父脚本状态刷新；保留绝对 lease/grant watchdog。

错误响应/签名或绑定错误/服务故障/撤销 inactive/到期/取消均永久 Deny；关闭 gate 先于等待 HTTP watch join，Stop 可重复/并发调用。关闭后旧正向状态不可复活、没有自动 enrollment/Mint/重连。helper 正常退出先 cancel、Stop/join watch，再关闭网络和 workers，最后关闭 API client 和 signer；取消时关闭自身继承 stdin、等待尚在读取的 decoder。自主模式禁止父管道 clock/state 授权注入、激活前 token 服务及旧 token probe；旧 fixture 模式仅保留兼容。

本轮完整 Go race/vet 通过，managed 新增门禁两轮竞态重复通过；真实 Mac Keychain→签名 HTTPS→自主管理 gate/Stop 实测通过并精确删除随机凭据。私有 opaque 隧道集成证实已获准连接在服务故障后关闭双向 worker、后续数据失败；不等于真实 Swift/C# TLS。Windows helper 交叉构建/vet 通过，没有本轮 Windows 新实机结果。本轮测试边界与复测见 [4B-2 验收](../poc/selfhost/public-server/MANAGED_HELPER_TEST.md)。未更新双机 Python runner（仍为父脚本转发模式），**未宣布真实 Headscale 双机自主链路通过**。无服务器部署/端口、安装包/已安装 GUI/现有 Tailscale/生产密钥更改；正式信任锚/HMAC 投递、完整启动配置与公开注册/GUI 仍未交付，不进入切片 5。

## 2026-10-07 切片 4B-1：OS signer 接入网络 helper（当前入口）

用户提供 Windows 正常交互式 PowerShell 截图：设置 vault 环境变量后，`TestNativeVaultIdentityAndIsolation` 真正 PASS（0.35s），退出码 0。之前未设置变量的 SKIP 不计通过。**4A Windows Credential Manager 创建/签名/重开/身份隔离人工验收已闭环**；这不等于产品默认 DPAPI 66 项验收，也不替代本轮新增签名域的 Windows 复测。下方 4A 的“未通过”是此前 SSH 上下文历史。

新增 `SignProvisionWithSigner`、nodeproof `ServeWithSigner`、session Config.Signer、streamgate `NewWithSigner`。网络注册意图、在线节点持有证明、固定载荷互证、产品 TLS 隧道的独立互证域都可调用 OS 辅助进程，调用方校验固定身份和返回签名；失败不得输出伪证明或打开产品上游。原测试私钥入口保留为 fixture 兼容；同时配置 Key/Signer 拒绝。

`enrollment-helper` 支持可信初始化的 VaultExecutable/VaultScope/ExpectedPublic，**仅重开已创建且固定公钥的身份**；不能在此静默创建，不能混入 AppKey 或缺少 expected。关闭顺序保留网络取消/worker join 后关闭 signer。初始化及后续命令已用 privatepipe 严格 8 KiB 解码，拒绝嵌套重复键/大小写别名/未知字段/尾随 JSON。Mac/Windows vault 新增六个精确域，不开放任意签名或私钥导出；不是对 JSON 内容作独立业务授权，业务门禁仍在可信 helper/在线服务。

本轮完整 Go race/vet 通过；Windows x64 vault/helper/原生测试交叉构建及 Windows 专用 vet 通过；Swift 126/126、vault Release 构建通过。Mac 串行显式 native 三组均通过：HTTPS 注册/重开稳定身份，存储隔离及新增域签名，真实本机 TCP 会话及产品 opaque 隧道认证/到期关闭；正常通过的随机身份精确清理。早期无签名开发程序及并行 native 初始化出现 3 秒超时，施加本地 ad-hoc 签名后串行复测通过；不将超时计通过，不扩展为正式分发签名保证。早期超时未能确认是否已写入随机测试凭据，不广泛删除 Keychain 命名空间；不存在生产密钥删除。

本轮验证记录见 [4B-1 验收](../poc/selfhost/public-server/HELPER_SIGNER_TEST.md)。未执行新的 Windows 实机或真实 Headscale 双机网络验收；现有双机 Python runner 仍使用 fixture AppKey，不能记为“OS signer 双机全链路已完成”。正式 issuer/API TLS 信任锚安装/轮换/恢复、产品 HMAC 安全分发、helper 自主 API 生命周期、旧 Swift/C# IPC 审计仍待后续 4B。无公网部署、安装包/已安装 GUI/系统 Tailscale/服务器业务修改，不进入切片 5。

## 2026-10-07 切片 4A：身份/存储签名/私有 IPC 核心通过，整体切片 4 未闭环（当前入口）

新增受设备签名保护的 `publish_tls`/`tls_identity`：目标必须属于真实绑定、策略 ready、在线批准授权，并证明持有其 RSA 服务器证书私钥；PSS/SHA-256 独立域绑定 grant/目标/指纹/挑战。仅持久化公开证书，不能让控制方发布/无 TLS 私钥发布/静默换证书；重开校验关联授权与历史证书结构。issuer 签名描述包含完整 grant、真实节点/IP、指纹、设备自生成挑战及绝对最多 3 秒期限（不超 grant），设备客户端以独立 issuer/校准时间逐项验签。真实 TLS 的 pin 改来自已验证描述，不再直接取父脚本观察的 Windows 指纹，每次新连接重新查询。活动流仍由原状态/grant watchdog 约束。

新增 `DeviceSigner` 与两个不返回私钥的独立辅助进程：Mac Keychain/Curve25519 Ed25519、Windows 当前用户 Credential Manager/Go Ed25519。明确首次创建、重开 expected-public 校验、缺失/损坏/权限拒绝停止，不明文替代。新 namespace 与旧产品配对凭据分离；Mac 非交互 LAContext，atomic add 不覆盖竞争获胜者；Windows mutex 串行初始化、清理内存 seed。客户端可配置可信 vault 可执行文件，不接受同时 AppKey；这是协议不导出私钥，不是硬件不可导出，也不抵抗同用户恶意代码。完整双机 helper 仍用私有 fixture 测试钥匙，尚未贯穿 nodeproof。

Go 私有管道新增 8 KiB、换行、严格未知/尾随 JSON、嵌套重复键/大小写别名拒绝；签名桥接不经 shell/密钥参数/公开监听，签名结果再验，超时只终止自己启动的辅助进程、正常关闭 join。新增设备 API 自主 StateWatch 串行每 500ms 签名 HTTPS 查询并验 issuer/grant/绑定/绝对期限，失败清缓存、inactive 终止；Stop 取消/join，无自动复活或重试 Mint。实机父脚本取缓存再发 helper，未完成父脚本消除/所有旧 helper、Swift/C# IPC 收口。

Go 全量 race/vet 与 Windows vault 交叉构建/专用 Windows vet 通过；Mac Swift 126/126 与 ConnectionIdentityVault Release 通过。两项显式 Mac native Keychain 测试通过：真实 HTTPS 签名注册、跨进程同公钥/设备码、缺失/错身份拒绝、域限制和精确测试删除。默认 Go suite skip native，不作为 OS 实机证据。真实 `--identity` 已完整零退出，证书私钥证明/issuer 描述进入真实 Swift/C# TLS、设备 watch/取消、原申请/确认、动态 ACL/零规则、TLS 正负向、周期恢复/Store 故障/撤销/旧状态拒绝与回收通过；Windows 产品 SSH profile 65/65，默认 DPAPI 人工验收仍待。测试资源清空、file 策略恢复、业务 Nginx 哈希一致/active，独立复核三类资源全 0。

Windows native **未通过**：早期 PowerShell 把未引用 `-test.run` 拆成 `-test`，未真正执行测试；补充非私钥启动诊断后已修正为引用 `'-test.run=...'`。真正运行明确得到 `interactive_logon_required`，Credential Manager 在 SSH 登录上下文没有可用登录会话，失败在创建阶段。不能将早期无结果归因于 DPAPI，也不能把最终未通过记成功。各轮上传 hash 核对、精确临时文件清理完成；未改生产存储/权限或转明文。需用户正常交互式 Windows 会话复测，命令及产物构建在 [切片 4A 验收](../poc/selfhost/public-server/IDENTITY_IPC_TEST.md)。

尚未收口：正式安装 issuer/API TLS 信任锚及轮换/恢复、产品 HMAC/身份包可信投递、OS signer 贯穿网络证明 helper、所有本机 IPC/整个连接生命周期接入。GUI/安装/现有系统 Tailscale/业务 443 未改；旧 RemoteController Keychain 密码弹窗不能因新增辅助进程就宣布修复。当前仍仅私有回环 API/SSH 测试转发，禁止公开 fixture。下一步仍是切片 4B，不自动进入切片 5；先 Windows 交互式存储验收和剩余信任/全链路接入。下方为历史。

## 2026-10-07 切片 3：设备码/确认与有界防滥用通过（当前入口）

新增设备签名 `incoming`，被控设备只能发现自己当前邀请未过期的 pending 请求和申请方应用身份哈希；不给公共目录/任意目标查询，不回验证码或 grant。已有 12 位设备码、8 位一次显示验证码/120 秒期限、单次原子兑换、明确 approve 后 300 秒固定载荷 grant 协议已进入实际设备 HTTPS 流程。控制方不能自批，pending/拒绝/过期/新邀请替换不授予控制，批准后控制方签名 status 获取同一 grant。有人值守“确认动作”仍由父测试脚本模拟，尚非 GUI 人工弹窗；身份哈希不是可信姓名。

新增解析/验签前有界 Admission：真实 socket IPv4 /32、IPv6 /64 每 600 次/分钟、全局 6000 次/分钟、最多 256 活跃来源前缀，满则拒绝，绝不信任 Forwarded/X-Forwarded-For；Store/handler 重开共享同一实例。持久新身份全局注册 10 次/分钟、一般设备请求 240 次/分钟、邀请轮换 3 次/分钟，原兑换/网络注册预算不变。未知非注册身份/未接纳新注册不分配持久 nonce；已接纳的签名失败仍消费。修正 claim 同时新增多个桶可超过 1024 的边界，容量耗尽拒绝，不自动重置身份/归档。未知设备码、缺失/错误验证码统一固定拒绝，未宣称时间侧信道恒定。

实机初次设备审批流程通过，但 Mac 拒绝刚签发 grant：服务时钟约快 2 秒。新增签名 `grant_clock` 给在线已批准、尚未绑定设备校准，要求设备新鲜挑战/单调起点、独立 issuer 验签、5 秒响应及完整 RTT 保守计时，再核验 grant；不放宽时间或授予网络 active。原 `clock` 仍仅 active 网络可用，撤销/过期不能获得新 grant_clock。另有早期新测试误将未绑定 NetworkState 的拒绝返回当失败，修正为不得得到 active 的断言，未放宽授权门禁。

最终全量 Go `test -race ./...` 与 `vet ./...` 通过，新增七组测试覆盖真实 HTTPS 流程/第三方隔离、pending 重开/过期、持久预算与容量、入口 IPv6/伪造头/全局/表上限/handler 重开、未绑定时间证明与撤销拒绝、统一错误与多桶容量。真实 `enrollment-network.py --device-flow` 完整零退出：fixture 不预创建批准 grant，Windows/Mac 设备进程各自签名 HTTPS 邀请、兑换、被控发现、拒绝、再次明确批准、查询、签名校准；无批准不能取得网络状态。随后原双机持有证明、动态单向 ACL/零 ACL、真实 Swift/C# TLS 正负向、周期失败关闭/恢复、Store 不可用、设备 HTTPS 撤销/inactive/仍有效旧状态拒绝、自动回收/正常退出通过。Windows SSH profile 65/65，本轮无 Swift/C# 代码变化，默认 DPAPI 人工待验保留。全部测试资源回收、原 file 策略恢复、原 PoC 策略/业务 Nginx 哈希不变、服务 active。复测与边界见 [切片 3 验收](../poc/selfhost/public-server/DEVICE_FLOW_TEST.md)。

上线限制：仍仅私有回环 HTTPS/SSH 测试转发，不开放公网，不改 GUI/安装/系统 Tailscale/业务 443。设备测试密钥、可信根/issuer 来自私有 bootstrap，父测试仍协调 helper。Admission 全进程重启会重置，持久设备/注册预算不重置；反代共享代理来源额度，多实例/DDoS/连接与握手 admission 及经认证代理来源未实现，当前配额为小规模 PoC 且可被恶意设备耗尽，不能称生产级防滥用完成。

下一切片 4：正式 issuer/TLS 身份分发、平台设备密钥存储、安全 IPC 与客户端自主生命周期；之后 GUI 和独立安装/干净机器验收。下方为历史。

## 2026-10-07 切片 2：设备签名 HTTPS 接口通过（当前入口）

新增正式设备侧 `POST /v1/request` 与 `POST /v1/network` 协议、受限 HTTPS handler/client、独立设备签名客户端。设备注册/撤销沿用原签名协议；网络 enroll/bind/state/clock 签名绑定路径、动作、时间、nonce、完整 grant 及注册意图。只有已注册且属于服务已知 grant 的设备可操作本人工单；第三设备、篡改、跨角色和持久重放拒绝。nonce 在特权操作前持久消费，失败注册调用也消费预算，未知设备不能耗用持久容量；网络注册每设备 6 次/分钟，状态等 240 次/分钟。TLS 验证独立可信根和 SAN，不跳过验证、不继承代理、不跟随重定向；固定错误不暴露驱动诊断。短期凭据单次返回，不提供再次取回。

全量 Go race/vet 通过；新增认证/范围/重放重开、真实 HTTPS/客户端与双端注册、严格 HTTP/持久预算测试通过。最终真实 `enrollment-network.py --api` 完整零退出：Mac/Windows 设备进程各自签名 HTTPS 注册、enroll/bind/时间校准/状态刷新；实际持有证明、动态 ACL/零规则、真实 Swift/C# TLS 正负向、周期恢复/后端重开、Store 不可用停止刷新、设备 HTTPS 撤销/inactive/旧正向拒绝和 TLS 关闭全部通过。两端 API 客户端正常关闭，临时 SSH 转发退出，节点/用户/注册 key 清空，恢复原 file 策略、服务 active、业务 Nginx 哈希保持。独立复核三类资源数量均为 0。Windows SSH profile 65/65 门禁保留，默认 DPAPI 交互式人工待验不变；本轮无 Swift/C# 产品代码变化，不记作重新完成默认 66 项。

边界：接口只部署测试服务器回环，用 SSH 回环转发私有访问；数据 TLS 仍走独立 tsnet/Headscale/DERP。不是公开生产入口，也不是最终用户需要 SSH 的方案。初始设备密钥/可信根/grant、有人值守批准及 helper 刷新调度仍由可信 fixture/父测试提供；正式身份分发、安全 IPC/平台存储、helper 自主刷新、GUI/安装包尚未交付。匿名/IP admission、容量滥用和防枚举不能由当前签名预算代替，禁止直接公开 fixture。后端重开保留 HTTPS listener/证书，未验证整个进程崩溃或证书轮换。详见 [接口与实测记录](../poc/selfhost/public-server/DEVICE_API_TEST.md)。

下一切片 3：设备码/一次性验证码与有人值守确认的设备侧流程，以及公开接入前的防枚举、匿名/IP admission、容量与滥用防护；正式 issuer/证书分发及安全存储/IPC 按切片 4 收口。下方为历史。

## 2026-10-07 切片 1：周期维护与故障恢复通过（当前入口）

新增私有 `ProvisionBackend.StartMaintenance`：同步关闭就绪栅栏，立即后台完整 Reconcile；每 Backend 一个串行任务，工作结束后再等待，不重叠/不积压。失败按间隔翻倍退避至配置上限，成功恢复正常间隔；完整规则投影/回读及精确回收完成才恢复就绪，不重试 Mint。原驱动 5 秒预算、签名状态 3 秒租期及 grant 期限不变。Stop 取消并 join，退出再次关闭共享 Store 的签发栅栏；必须成功 join 后才关闭/替换 Store、Driver。状态只有固定计数/近期结果，不是授权依据，也无驱动原始错误或凭据。

本机五组新增竞态测试与全量 Go race/vet 通过：策略失败、退避恢复与停止栅栏；grant 到期规则先撤回、回收失败重试且不重发；未绑定凭据到期自动回收而不撤销仍有效 grant；活动 driver 取消、冷 Store 重开后先同步才恢复；重复任务/非法配置拒绝。

真实 `enrollment-network.py --maintenance` 完整零退出，保留注册/动态 ACL/零规则/真实 TLS 负向和授权关闭门禁。周期同步与状态刷新并行时 TLS 持续；可信 runner 拒绝真实 Driver 的 policy 命令后停止签 active、保留旧 ACL 时 TLS 关闭；恢复命令后由周期任务自动重试成功，无手工 Reconcile。持久后端重开先停止旧任务，新任务自动启动同步恢复同一绑定。Store 关闭、保留 ACL 的撤销/重放拒绝继续通过；两份已撤销工单由周期任务自动 cleaned，回读显式零 ACL，任务正常取消/join。所有进程零退出，节点/用户/注册 key 清空，原 file 策略、服务 active、业务 Nginx 哈希保持。Windows SSH profile 65/65 与自包含发布通过；默认 DPAPI 交互式人工待验保留。

两次早期实测在旧“helper 已关即检查产品计数”的断言失败：Mac 无新 PONG，但 C# 已读入的最后 PING 可稍后完成一次发送计数。新增开发 TLS 服务会话结束计数，等待 Mac/helper/C# 任务全部退出后再断言双方计数不增长；未放宽增长断言，未用客户端超时作为通过。最终完整实机通过。细节见 [周期维护验收](../poc/selfhost/public-server/MAINTENANCE_TEST.md)。

边界：周期任务已接私有后端/实机 fixture，未部署公开生产守护服务；policy 故障是可信 CLI runner 拒绝，不是关闭整个 Headscale；后端重开不是杀进程/断电演练。自然到期本轮为真实 Store/可控时钟测试，不算新一轮公网 300 秒到期验收。无 GUI/安装包/屏幕/输入/系统 Tailscale/防火墙变化。下一切片 2：正式设备注册与认证状态接口，替代私有 SSH/父脚本初始化与刷新；仍须正式证书身份分发/安全 IPC 后再开放产品能力。下方为历史。

## 2026-10-07 真实产品 TLS 桥接通过（当前入口）

动态 gate 已桥接真实 Swift `TLSControllerClient` ↔ C# `TlsProbeServer`，通过独立 tsnet/Headscale/DERP 传输 TLS 密文；不是 Go 模拟认证。仅连续固定 8 字节 PING/PONG，不接屏幕/输入/剪贴板/GUI/安装包。Mac 仅监听回环临时端口，Windows 产品开发服务只监听 `127.0.0.1:47478`；目标固定不可由 peer 选择。原 Self/IP、WhoIs、签名绑定和新鲜应用互证保留，TLS 桥接有独立签名域，旧固定载荷证明不能打开上游。

两端 250ms watchdog 检查 grant 和绝对 3 秒签名状态，父管道每 500ms 刷新。失效关闭两向 socket、等待 copy worker 退出，无自动重连。Go 不解 TLS，不声称逐个加密 PONG 前查询授权。开发证书/产品 HMAC 密钥由 Windows 临时生成，通过捕获的可信 SSH 私有管道传给父测试和 Swift 内存 pin；绑定本轮批准目标，但不是正式 issuer 签名的 TLS 身份分发服务。无生产钥匙串/DPAPI 修改。

真实 `enrollment-network.py --tls` 完整零退出：保留原注册/持有证明/动态 ACL/零规则与恢复；错误指纹明确拒绝、错误 HMAC 返回认证拒绝、认证前 PING 被真实 C# gate 拒绝，三者零保护 PONG；正确 TLS/framing/认证持续同一连接超过初始租期。停止刷新、真实 Store 关闭但 ACL 保留、撤销 inactive 但节点/ACL 保留都关闭真实 TLS 且双端 PONG 不增长，不以客户端超时替代授权关闭。Store 重开/Reconcile、同节点显式重启恢复；有效旧正向状态仍被 inactive 锁定拒绝；撤销精确回收、旧 grant 不复活、全部进程正常关闭。恢复原 file 策略、服务 active、业务 Nginx 哈希保持。

Go 全量竞态/静态检查通过，Mac 126/126、TLSBridgeProbe 与 RemoteController Release 通过。Windows 自包含 win-x64 开发探针发布与 SSH profile 65/65 协议测试通过；**默认套件共 66 项，其中 DPAPI 凭据持久化在 SSH 登录上下文报“登录会话不存在”，仍需交互式 Windows 验证**。新增明确 `--bridge-validation` 只跳该项且打印 SKIP；默认测试及生产 DPAPI 未削弱。不能记为默认 66/66。

主要问题：Mac tar 的 `._*.cs` 元数据被 Windows 当源码，已排除；错误证书指纹已经拒绝但 NWConnection 没有及时回报 failed，现于 verify 拒绝后明确结束并取消连接，并在创建连接前配置安全选项。三项真实负向复测通过。细节、复测命令和边界见 [TLS 桥接验收](../poc/selfhost/public-server/TLS_BRIDGE_TEST.md)。

下一切片优先：周期受控 Reconcile/故障恢复，随后公开认证状态/设备码接口、防滥用、正式证书身份分发与本机安全 IPC/存储，最后 GUI/安装集成。现有产品仍依赖系统 Tailscale；没有宣布最终免登录安装体验完成。下方为历史记录。

## 2026-10-07 动态注册活动流闭环（当前入口）

新增 `streamgate`，将真实注册账本派生的签名绑定/3 秒状态接到现有 session 固定载荷长连接；不是产品 TLS。helper 私有管道初始化 issuer/双方公钥、刷新状态、启动探针和读取固定计数，网络方不能自行选择绑定。握手验证 Self/IP、实际入站地址、WhoIs 对端节点公钥及双方应用 challenge 签名。控制端 LocalAPI 是本地代理 socket，先认证显式 overlay 目的节点，才提供认证地址给 session；不跟随 Dial-Self 到系统网络。

被控端每次响应前检查状态并有 watchdog，控制端也有 watchdog；inactive/畸形更新拒绝、停止刷新或后端不可用到期关闭。激活不可退回 token 模式，跨激活的旧 token 写入也被禁止。停止先取消并等待证明/数据/探针任务，沿用 8 秒正常关闭预算。

实机初测发现 Linux 比 Mac 快约 2 秒，严格快照拒绝未来时间。新增绑定 helper 新鲜 nonce/单调请求起点的 issuer 签名 ClockProof，5 秒内核验并单次消费，应用时钟按“服务签名时间+请求起点完整单调经过时间”保守计时，不改系统时间、不扩大 3 秒 lease、不按接收重新计租期。原注册意图 ±30 秒、控制 HTTPS 系统时间要求仍保留；不是任意时钟错乱的通用注册方案。

三平台构建、全量 Go 竞态/静态检查通过。真实 `enrollment-network.py --streams` 校准后完整两轮零退出，最后一轮加强为两端独立确认旧正向状态签名/租期仍有效却被 inactive 锁定拒绝，不用已过期状态代替重放负向。其余覆盖：动态身份互证载荷、刷新后同一连接持续超过初始租期、停止刷新后双端关闭且无载荷增长/自动重连；实际关闭 Store、规则仍存在时拒绝刷新并到期关闭；重开/Reconcile/同节点重启恢复；仅撤销 grant、节点与 ACL 尚在时 inactive 关闭活动流，再回收。服务恢复原 file 策略、active、资源回收且业务 Nginx 哈希保持。详见 [实现与校准边界](../poc/selfhost/streamgate/README.md)。

尚未接 Swift/C# 产品 TLS framing/证书 pin/认证，不改安装包、GUI、屏幕、输入、系统 Tailscale、业务 443。状态刷新/设备私钥仍是私有管理 fixture，不能公开；故障为真实 Store 关闭而非断电/整个 daemon 崩溃。公开认证状态服务、周期 Reconcile、设备安全存储仍待实现。下一切片：桥接真实产品 TLS 固定载荷与此动态 gate，并验证无 token/地址旁路、活动 TLS 流撤销/停止刷新/故障关闭；之后周期 Reconcile/故障恢复与公开设备码。下方为历史记录。

## 2026-10-07 动态注册签名状态前置通过

上一切片 `4d08bdd` 已推送到 `origin/codex/connection-architecture-poc`。本轮先消除旧签名状态 target=.1/controller=.2 假设，不误报产品 TLS 已完成。

新增私有 `ProvisionBackend.NetworkState(grant)`：不接受调用者指定 IP/节点公钥，只从同一完整 grant 的两个唯一 bound 记录派生绑定。地址限定 canonical `100.120.0.0/24` 主机地址、不同 IP/非零不同节点公钥。只有最后成功 Reconcile 的最小规则包含该绑定且共享策略栅栏就绪，才签 active=true；刚绑定未投影只能签 inactive。策略失败、Healthy=false、撤销/过期不签新有效状态；缺失/已清理/歧义记录、存储损坏或时钟回退拒绝。就绪/投影缓存不落盘，磁盘重开先 inactive，成功 Reconcile 后恢复。

绝对 3 秒 lease/签名域不变；故障前下发状态最多仍有原租期，session 另核 grant 到期。inactive 锁住该 grant 的当前 StateCache，不可重放旧正向状态解锁，恢复需重建 cache/连接。旧 Store 手工绑定签名口仅历史管理员 fixture 使用，新接入禁止使用。

全量 `go test -race ./...`、`go vet ./...` 通过，新增定向竞态再次通过。真实 Linux/Windows/Mac 注册脚本零退出：未证明双方拒签、刚绑定未投影 inactive、投影后签名节点/IP 与真实动态分配完全一致、冷驱动重开同步恢复同一绑定、撤销清理后拒签。原正向/反向/零 ACL/恢复/退出/清理全通过；服务恢复原 file 策略且 active，业务 Nginx 哈希保持。

尚未把动态签名状态送入 helper 活动流，未接 Swift/C# 产品 TLS；无安装包/GUI/屏幕/输入变化。投影缓存是受控后端最后同步结果，不是每次签名实时查询 Headscale；管理员绕过驱动手改 ACL 不会自动更新缓存。下一切片：动态双机固定载荷活动流的状态刷新与撤销/停止刷新/故障关闭，再桥接真实产品 TLS framing、证书 pin、认证。之后周期 Reconcile/故障恢复、公开设备码/防滥用、GUI/安装包。下方为历史。

## 2026-10-07 真实注册与动态策略闭环通过

新增 nodeidentity 版本敏感的离线私有 profile 适配（公开 StateStore/Prefs/Persist/Profile 类型，没有 fake 注册 NodeID，不用测试专用 hooks）；helper 在无凭据/未注册时自己签名节点公钥意图。真实 Linux 验证节点、Windows target 和 Mac controller 注册后公钥与准备值一致；Mac 重启后原身份/地址一致。SDK 仍固定 1.102.5，升级必须回归实际公钥一致性，不能当官方稳定注册前 API。已有状态损坏/符号链接/控制入口变化拒绝且不重生身份。

两类新 helper 均清除本进程继承的登录/OAuth/强制登录变量、禁托管日志上传，禁用缓存 map 与已复现泄漏的 ALWAYS_USE_DERP dummy sockets，使用真实 UDP 的直连抑制测试路径；不改系统环境或系统客户端。已有非空状态目录丢失 state 文件也失败关闭，不重生身份。

真实 Driver.Verify 注入 nodeproof：仅临时允许可信验证节点→候选:47477，实际 tsnet map/WhoIs、256-bit 新鲜 challenge、设备 Ed25519 互绑定后才 bound，随后恢复数据策略；恢复错误同步驱动 Healthy/共享 Store 禁发栅栏。错误应用私钥实机必须收到 challenge 且 signed=0，后端明确 peer_rejected、工单未过期，不以拨号超时当负向通过。无缺省 true 证明。

enrollment-network.py 实机完整门禁通过：未绑定无载荷、双方绑定仅 controller→target:47476、真实跨网固定载荷、反向拒绝、持久 Store/全新 Backend+Driver 重开恢复批准规则；显式空 ACL 回读后连续 3 次 LocalAPI overlay 连接请求拒绝且存活 target 计数不增加；恢复规则同进程载荷再次成功；撤销精确清理、磁盘重开不复活旧 grant。拒绝包括 ACL 撤回 peer 路由/Dial-Self 拒绝和 overlay 拨号拒绝，绝不跟随系统网络；不扩大为抓包证明或已有流 ACL 即时切断。既有流授权门禁仍保留，尚未与本轮动态注册桥接。

遇到两项实际问题并修正：Headscale 0.29.4 配置实际为 policy.mode=database，而 CLI 帮助说 db；模式切换失败自动还原。Mac 重启后初次证明出现 dial 超时且工单未过期，Bind 预算从含策略/握手的 5 秒改为有界 12 秒，并等待可信 map 中 peer online；其他管理 5 秒、key ≤120 秒/grant 300 秒绝对期限不变。随后加强负向及反向门禁的连续复测通过。挑战不要求用户时钟同步，靠 nonce+连接截止；签名意图仍有原 ±30 秒窗口。

测试后恢复服务原 file 配置/静态 ACL，临时 key/node/user 与两端节点私钥回收，服务 active、原 Nginx 两项哈希 OK；没有改业务 443、Windows 防火墙、系统 Tailscale、产品安装包或证书（续期用户安排）。公开入口、周期 Reconcile daemon、生产权限/设备本地安全存储、多进程锁与 SDK 升级仍待后续，fixture 测试私钥交付不能公开。见 [完整验收/复测/边界](../poc/selfhost/public-server/ENROLLMENT_TEST.md)。

最终全量 `go test -race ./...`、`go vet ./...` 通过；`verify.zsh` 原 Headscale/DERP/双端重启/第三节点 ACL/正常清理零退出。修正后完整真实注册脚本多轮零退出，最后一轮包含共享策略失败栅栏、helper 自签意图、反向网络负向与真实错误私钥挑战断言。没有借此宣称新动态 ACL 的 300 秒长测、服务器断电恢复或产品安装包验收。

下一切片：产品 TLS 固定载荷与现有签名状态门禁桥接，消除旧 `.1/.2` 固定地址假设；补受控周期 Reconcile/故障恢复，再做公开设备码服务/防滥用和 GUI/安装包。不跳过到宣称最终免账号产品上线。下方“闭环待验收”均为历史。

## 2026-10-07 真实 Headscale 驱动接入（管理操作已实测，闭环待验收）

新增 `poc/selfhost/headscale.Driver` 实现 ProvisionDriver：服务器本地受限 CLI、工单独占用户/短期非 reusable key、严格观察、按工单发现未知结果孤儿并精确回收，以及 db 模式策略 check/set/严格回读。没有 live proof 回调默认拒绝绑定。注册 secret 不进日志/参数/持久账本；策略错误继续失败关闭。详见 [真实驱动边界与复测](../poc/selfhost/headscale/README.md)。

Linux x64 检查程序在真实隔离服务器验证随机工单发钥匙/实际到期上界/精确回收/重复回收通过。真实接口差异：expiration CLI 只收时长，现按剩余绝对期限保守取整并核对服务返回截止时间；时间 JSON 是 protobuf seconds/nanos，已适配。调试失败留下的两个已确认工单也已精确回收；最终 nodes/users=null、服务 active、业务 Nginx 两项哈希 OK。未改公网策略模式（仍 file）、原 ACL、业务 443、Windows、产品安装包或系统 Tailscale。证书续期由用户安排。

尚未完成：策略动态更新/零规则的真实数据通道拒绝证据；helper 注册前节点公钥获取与真实 LiveProof；ProvisionBackend 与 helper 的完整注册/策略/重启恢复闭环。不能把本轮管理 CLI 实测或策略 JSON 单测当最终免登录连接验收。下一切片优先这些，不进入产品 TLS 桥接。历史“真实驱动未实现”现仅指上述未接入闭环，不指管理适配器。

最终验证：新增 7 项驱动测试，全量 `go test -race ./...`、`go vet ./...`、`GOPROXY=off zsh poc/selfhost/verify.zsh` 均通过（含原 token DERP/双端重启/第三节点 ACL/正常清理）。最终真实驱动检查再次零退出，同工单重复 Mint 拒绝、用户归属和期限核验、精确幂等回收通过，users/nodes 再次为空。没有本轮动态策略或 Windows 新注册证据。

## 2026-10-07 私有受控注册账本核心（当前入口）

本轮先收口注册/策略闭环的持久化后端核心，不直接部署或修改服务器策略。新增 invite.ProvisionBackend、独立签名注册意图与单次短期凭据驱动接口，复用现有私有快照/单写者 lease；旧快照兼容，不重置身份。凭据明文不落盘，工单在驱动副作用前持久化；状态 planned/issued/bound/cleanup_pending/cleaned。所有网络观察/真实节点持有证明必须来自可信驱动，不能信客户端报文里的布尔或 IP。

仅当前授权双方都绑定才投影控制端→目标:47476 最小规则；先撤无效规则再回收资源、最终重算，回收确认持久化。启动必须 Reconcile 成功才发凭据；未知 Mint 结果/崩溃孤儿按精确工单回收，不重发。key 注册到期与 bound grant 到期分开处理；策略/清理失败阻止发新凭据，不能把驱动错误当 ACL 已撤回。完整边界与测试见 [邀请核心](../poc/selfhost/invite/README.md)。

新增 13 项核心测试通过，涉及内存驱动/私有磁盘账本，并非真实 Headscale API/持有挑战/动态策略证据；包括共享 Store 同步失败禁止其他 wrapper 继续发钥匙。最终全量 `go test -race ./...`、`go vet ./...` 和本机 `verify.zsh` 零退出（依赖校验、原 token 自建 DERP/双端重启/ACL/退出清理）。本轮没有使用服务器 SSH、公开端口、Windows 防火墙或产品安装包变更，没有新增公网注册验收。

下一切片：实现受限 Headscale 驱动，在隔离资源上验证单次凭据/工单隔离、节点观察与实际网络持有证明、策略原子更新/撤销与重启恢复，再把 helper 私有注册流程接上。需先确认节点公钥如何在内嵌 helper 注册前可信取得，不假设 tsnet 有未经验证的 API；阶段完成前保持现有审批/管理 fixture 可回归。证书续期由用户安排。

## 2026-10-07 公网负向门禁与真实 grant 到期通过（当前入口）

在用户确认继续切片后新增 `authorized-network.py --negative-expiry`，真实 Windows serve / Mac probe 实测退出码 0。基础门禁回归仍通过；补齐 pending/明确拒绝均无新 grant 且无法用旧 grant 传载荷、服务签名正确但节点公钥绑定错误的连接拒绝、已允许网络节点上第三应用私钥对真实 challenge 的证明拒绝（要求 EOF 和 server session_denied，不将超时当成功），以及撤销后旧正向状态重放不能恢复 helper。旧正向重放触发明确 invalid_authorization_update，helper 正常关闭，没有强杀/cleanup_timeout。

真实 300 秒 grant 长测保持正常固定载荷及每 0.5 秒状态刷新，不改机器时钟/不缩短 grant。约 30 秒输出一次无凭据进度；连接在签名绝对到期附近关闭（不是 360 秒 helper 上限）。后端无故障，最后正向状态有效期跨过 grant 到期，独立断言证明并非 3 秒状态超时导致结束。真实已过期 grant 可继续获得签名 inactive 状态，后端不退出；篡改 grant 仍拒绝。

新增过期 inactive/篡改拒绝、负向探针实际 EOF/server 拒绝、serve 禁止负向开关单测；最终全量 `go test -race ./...`、`go vet ./...` 通过。Mac/Windows helper 构建、完整原 token 本机 `verify.zsh` 回归零退出。公网测试节点数保持 2；临时 key/节点/用户/两端节点状态和私有 fixture 精确回收。清理后独立复核 nodes/users=null、服务 active、原两项 Nginx 配置哈希 OK。现有产品/安装包/系统 Tailscale/Windows 防火墙不变。证书续期由用户安排，本切片不修改证书。

下一切片：受控设备注册/短期网络凭据与最小网络策略的后端闭环，逐步替代当前可信管理员读取节点身份/审批 fixture，先私有入口，不直接发布已有 loopback 注册服务。需要明确设备持有证明与网络节点所有权绑定、凭据回收、授权到期/撤销的网络规则收敛及故障恢复。再进入产品 TLS 固定载荷桥接。当前仍不宣称最终用户无 SSH 设备码 GUI、多租户公开服务、动态 ACL 或已有流无缝直连→中继迁移完成。第三私钥证明在已允许节点上实测，不扩张为本切片新注册第三网络节点测试。复测说明见 [授权验证](../poc/selfhost/public-server/AUTHORIZATION_TEST.md)。下方“负向/300 秒待验收”为历史。

## 2026-10-07 双机签名授权门禁实测通过（当前入口）

用户恢复两把专用 SSH 密钥后，服务器和 Windows SSH 均认证成功；隔离数据库无节点。使用 `0d79fb0` 实际执行 `python3 -B poc/selfhost/public-server/authorized-network.py`，退出码 0。Windows x64 独立 helper 上传和校验通过；两节点归属确认、Headscale 输出适配、实际 tsnet status/WhoIs 节点公钥绑定，以及应用设备互证在本轮真实连接中通过，不再只是编译/本机证据。

通过项：签名授权后真实固定载荷且自建 DERP 路径确认；后端撤销关闭活动流；后端重启保持撤销；Windows serve 重启后旧授权被拒绝且零载荷；新的明确 grant 成功；停止后端状态刷新后，3 秒绝对有效期使活动流关闭；节点数保持 2，没有身份增生。所有正常停止、私有后端关闭及本轮短期 key/节点/用户/两端测试节点状态清理均成功，没有 cleanup_timeout 或强杀计通过。

清理后独立 SSH 复核 Headscale nodes/users 均为 null，PoC 服务 active，原 nginx.conf 和 mk.fengmap.com.conf 哈希均 OK。原业务 443、产品安装包、Windows 防火墙和系统 Tailscale 未修改。测试目录仅保留二进制等非私密制品。证书实测 `notAfter=Oct 13 23:59:59 2026 GMT`，需及时续期；本轮没有执行证书更新。

边界：仍是管理员受控的私有后端/审批 fixture，不是最终用户 SSH 流程，不是公网注册服务或动态 ACL。跨网到期证据为 3 秒状态到期，300 秒 grant 实际到期仍待实机验收。第三设备/错误网络绑定负向、未批准/明确拒绝、旧正向状态重放在公网 helper 的完整门禁覆盖尚待补充；不能用以前 token ACL 或本机测试代替。下一切片优先补齐这些双机负向与真实 grant 到期（可复用此入口），再进入正式受控注册/网络策略和产品 TLS 桥接。记录与复测见 [授权验证](../poc/selfhost/public-server/AUTHORIZATION_TEST.md)。下方 SSH 阻塞描述为已解除的历史。

## 2026-10-07 双机授权接入：代码就绪、实机验收待恢复（历史，现已解除）

在 `1129ce3` 的 session 门禁上新增 helper authorization 模式，私有管道签名状态刷新、3 秒绝对到期、撤销不可被旧状态恢复；可信管理绑定设备 grant 到固定测试网络节点，并在 tsnet status/WhoIs 中核对实际双方节点公钥，实际 socket 地址/设备互证仍由 session 检查。不是仅父脚本查一次状态再运行旧 token 流。原 token 模式保留回归，不改安装包/服务器业务/现有 Tailscale。

新增 `cmd/grant-fixture`（私有 stdout 含凭据，禁止交互输出）、`public-server/authorized-network.py`。本机签名缓存、bootstrap 边界、真实 TCP 活动流撤销/到期、私有 fixture 的撤销→后端重启→新授权已测试；双机入口的预期场景/构建/边界见 [授权复测](../poc/selfhost/public-server/AUTHORIZATION_TEST.md)。网络绑定签发仅受控管理 fixture，不是正式注册服务/动态 ACL，grant 到期与 3 秒状态到期要分别记录。

本机验证：最终 `go test -race ./...` 通过，`go vet ./...` 通过，完整 `verify.zsh` 零退出（原 token 自建 DERP/双端重启/ACL/退出清理）；私有 fixture 协议 bind/lease/revoke/restart/renew 实际零退出，两端 helper 构建/Windows x64 交叉编译通过。新的公网编排只做语法与其私有 Authority 适配验证，没有真实 Windows/WhoIs/网络绑定签名流的通过证据。

**公网双机本轮未验收**：SSH agent `The agent has no identities`，服务器 `182.92.117.114` 公钥认证失败，Windows `100.73.4.118:22` 超时。本轮尚未部署、上传或创建远端测试身份。恢复专用 SSH 密钥、Windows 在线后优先执行授权复测，不直接跳到产品桥接。新编排的 Headscale 字段/WhoIs/时钟兼容仍待实机校验。证书记录为 2026-10-13 到期，复测前确认续期。未完成的双机验收不记作通过。

## 2026-10-07 签名授权连接门禁（当前入口）

上一切片 `7280cfc` 已推送 `origin/codex/connection-architecture-poc`。上次中断留下未提交的 `session` 草稿，本轮先修复编译并收口为独立授权连接模块；不跳过未完成工作。模块在真实本机 TCP 中接入邀请持久化核心，固定服务公钥/双方设备公钥/grant/实际网络地址，双方随机 challenge 和分角色签名互证，帧有界；当前在线授权允许才回应固定载荷。活动/空闲连接通过有界查询及轮询处理撤销、到期、后端故障、时钟回退，取消时主动关闭连接；旧 grant 重启后不复活，新明确批准只允许新 grant。

实现/复测与安全边界见 [授权门禁](../poc/selfhost/session/README.md)。本切片仅本机门禁，不是双机公网后端联动：地址匹配不证明 Headscale 节点所有权，握手不加密，未来必须使用独立 tsnet/产品 TLS 和可信注册绑定。没有开放邀请服务、没有修改服务器/原 Tailscale/现有安装包。

验收：新增 16 项顶层测试通过，`go test -race -count=3 ./session` 三轮通过；随后强化私钥一致性和查询返回时到期复核，最终 `go test -race ./...`、`go vet ./...` 通过。`GOPROXY=off zsh poc/selfhost/verify.zsh` 原本机自建 DERP、双端重启/身份保留、错误 token、第三节点 ACL、正常退出和临时身份清理均零退出。此次没有执行新的 Windows 实机或双机公网授权测试，不扩张既有证据。

下一切片：把 session 接入隔离双机 helper，完成可信服务公钥/应用设备到网络节点的受控绑定，以及有认证、超时和故障关闭的后端授权查询；在自建跨网通道验收批准、拒绝、到期、撤销活动流与重启。公网注册滥用防护/动态 ACL、最终 GUI/安装包及生产运维仍待开发。服务器现有证书 2026-10-13 到期，双机测试前仍须确认续期。

## 2026-10-06 设备码/一次性邀请后端核心（当前入口）

本切片在独立 PoC 中新增 `invite` 授权核心和仅绑定 `127.0.0.1:18445` 的 `cmd/invite-api`，没有公网部署、没有接 Headscale 注册/ACL 或真实 helper，也不改产品、系统 Tailscale、服务器和凭据。

实现：Ed25519 设备持有证明、稳定设备码、120 秒一次性 8 位验证码、只用设备码+验证码申请、被控端明确批准/拒绝、绑定双方公钥身份/固定载荷权限的 300 秒签名授权、单活动授权、到期/重放/限速/撤销持久化。新邀请使旧邀请/未批准请求失效。请求的签名/action/nonce/时间一起验证；有效签名的失败请求也记录 nonce/限速。权限不足统一拒绝，不输出私钥/验证码/授权日志。

持久化采用单写者私有快照与原子同步；状态缺失/损坏不重置身份，写失败停止授权；并发兑换只有一个成功，重启不丢失拒绝/撤销/限速。离线签名校验与在线 Authorized 状态检查明确分开，后续不能把 VerifyGrant 当作未撤销证明。邀请包 15 项测试通过，包括真实 loopback HTTP 全流程/重启；最终 `go test -race ./...` 通过，现有 verify.zsh 回归通过。Linux x64 后端命令只完成交叉构建，不宣称已部署。

执行与限制见 [邀请后端](../poc/selfhost/invite/README.md)。下一切片接入隔离双机 PoC：可信服务公钥下发、应用设备公钥↔Headscale 节点绑定、受限注册凭据/网络策略下发、双方批准后固定载荷通行、拒绝/到期/撤销停止活动流与重启不复活。先做受控入口/滥用防护，再考虑公网注册；不直接开放当前 loopback 服务。GUI 设备码、最终安装包、多平台双向角色、生产持久化/运维仍未完成。

## 2026-10-06 直连/新连接中继回退与活动授权撤销（当前入口）

上一切片 `4723f2d` 已按用户要求推送到 `origin/codex/connection-architecture-poc`。本切片只改独立 Go helper/隔离公网编排，不改产品安装包、原 Tailscale、Windows 防火墙或原 Nginx 443。

新增明确测试模式：允许直连时用受限 disco 探测、再核对真实载荷路径；持续认证心跳流用于验证已有连接；私有父子管道 `revoke` 撤销本次授权，取消上下文并显式关闭活动连接/监听/节点。与注册 key 撤销区分。新增配置冲突拒绝、活动流取消单测；原 `stop` 和本机 fixture 兼容。

公网门禁连续两轮零退出，均观测真实直连 `direct`；随后抑制直连的新连接明确走自建 DERP；此前重启/错误 token/第三节点门禁均通过。撤销注册 key 后活动流仍正常传输（不能当作会话撤销）；显式授权撤销后双方活动流关闭、后续连接被拒绝；相同设备身份以新 token 重新授权后，旧 token 双端拒绝，新 token 成功。身份/节点数无增生，正常关闭、短期 key/节点/用户及两端测试私钥清理通过。最终版强化的新连接拒绝断言要求注册成功且 dial_failed，第二轮已实测通过。

复测发现地址游标的准备问题：节点删除后隔离服务仍保留内存分配游标，固定 ACL 的 `.1` 门禁失败并安全清理。本脚本现在要求数据库无节点，随后只重启 `remoteapp-poc` 服务，保留数据库/Noise/DERP 私钥，重新健康检查；不是重置生产身份。原服务器业务不重启。

`GOPROXY=off zsh poc/selfhost/verify.zsh` 依赖校验、Go 单测、完整本机回归零退出；`go test -race ./...` 通过。公网入口 `python3 -B poc/selfhost/public-server/cross-network.py --network-lifecycle` 连续两轮通过，清理后独立复核 nodes/users 为空、原业务配置哈希一致、服务 active。证据与边界见 [公网记录](../poc/selfhost/public-server/README.md)。尚未验收已有长连接直连→中继的无缝迁移；撤销入口只是被控端私有控制管道，不是后端签名撤销通知或持久授权服务。下一步设备码/一次性邀请后端及其授权生命周期，再接产品 TLS 固定载荷；不把本轮记为完整设备码产品。

## 2026-10-06 双机公网自建中继通过（当前入口）

在用户授权资源上实际执行 Windows serve / Mac probe 固定载荷，独立 helper 严格增加唯一公网 HTTPS URL，保持证书验证，并按控制入口确认本项目 DERP region。`cross-network.py` 零退出：公网自建 DERP、错误 token 双端拒绝、Mac 客户端连续三次重启、Windows serve 重启、身份/地址/节点数稳定、第三节点 ACL 拒绝、正常退出均通过，无 cleanup_timeout 或强杀后计通过。SSH/现有 Tailscale 仅用于管理/上传，helper 数据由独立 tsnet 身份承载，不改产品/安装包。

精确撤销本轮短期 key、删除测试节点/用户、清理两端测试节点私钥，独立复核 nodes/users 均为空、服务 active、原 443 配置哈希一致。Windows 专用测试目录仅保留二进制/空目录。服务未设置开机启动，现有证书仍须在 2026-10-13 到期前续期。

入口阻塞已解除：Mac/Windows HTTPS 8443 通过，UDP 3478 有效 STUN 响应通过。此前缺少 SOFTWARE/FINGERPRINT 的最小探测会被固定库拒绝，不能把超时直接解释为 UDP 未开放。Windows 初次目录 ACL SID 格式错误在身份创建前已修复。本机 `GOPROXY=off zsh poc/selfhost/verify.zsh` 依赖验证/单测/完整回归零退出；沙箱 PermissionError 经正常权限执行通过。

执行与边界见 [公网记录](../poc/selfhost/public-server/README.md)。下一切片优先直连/NAT 探测与中继回退、授权撤销关闭活动流；设备码邀请后端/产品 TLS 桥接仍未实现。Linux/Apple Silicon 实际运行、双向角色、长期耐久、生产运维仍待验收，不宣布最终架构完成。下方公网入口阻塞和本机阶段描述是历史。

## 2026-10-06 公网 PoC 服务准备（当前入口）

用户提供并授权服务器 `182.92.117.114`、域名 `mk.fengmap.com`，已配置专用 Mac SSH 公钥，并确认 TCP 8443/UDP 3478 安全组放行。已部署独立 Headscale v0.29.4 + 开启客户端身份检查的嵌入 DERP，独立低权限用户、私有数据库/密钥；未安装 Docker，未修改系统 Tailscale、原 443 业务或云安全组。服务 active、没有设置开机启动。服务器内受信任 HTTPS 健康检查、DERP 探测与管理路径拒绝通过，原业务配置哈希一致。

**公网可达性未通过**：Mac/Windows HTTPS 8443 超时，Mac 标准 STUN 3478 超时；DNS 正确，指定服务器 IP 仍超时；服务器 iptables ACCEPT。需用户核对安全组实际绑定实例、入方向规则、源地址及其他云侧限制。不要声称双机跨网、中继载荷或免登录最终架构已完成。服务尚未注册节点；helper 仍限定本机 fixture URL，下一步须在公网可达后增加严格 HTTPS allowlist 与跨网编排。

现有证书 2026-10-13 到期，必须续期。CentOS 7 只做短期测试，不作为生产选择。首次因 Nginx reload 异步就绪触发自动回退，已补等待并安全激活；没有关闭 TLS 校验。配置、部署/回退边界和实际结果见 [公网服务记录](../poc/selfhost/public-server/README.md)。

## 2026-10-06 Windows x64 实机 PoC 验收（当前入口）

Windows 实机验证已完成：PowerShell 5.1、Docker Desktop 本机 npipe / Linux 引擎。通过用户授权的 SSH 管理/传包，**没有用系统 Tailscale 承载 PoC 数据流**；Headscale 宿主映射 127.0.0.1:18443，DERP 127.0.0.1:18444、临时证书 SHA-256 pin。未改产品、系统 Tailscale、防火墙、WSL 或原 Docker 配置。

新增 `verify-windows.ps1` 与内部 Windows ZIP 打包入口，Windows 不需 Go/Python/.NET SDK。Go helper 新增 `--control-stdin`：一行受限 JSON bootstrap，随后私有管道 `stop` 或父进程 EOF 正常停止，避免 Windows 没有 SIGTERM 而使用强杀作为验收。原 Mac bootstrap 兼容；新增协议边界单测。

实际结果：Mac `verify.zsh`（依赖/单测/默认回归）零退出；`smoke.py --check-control`（同一私有管道加连续三次重启）零退出；官方便携 PowerShell 7.4.13 语法解析和目标机 PowerShell 5.1 解析通过；Windows 完整脚本最终 `PASS windows_smoke_complete`、退出码 0。客户端连续三次重启、服务端重启后数据与错误 token 双端拒绝、第三节点拒绝、地址/节点数不变、所有节点正常关闭及 fixture 清理均通过。没有 `cleanup_timeout`，没有强杀节点后记为通过。

Windows 初次镜像下载碰到公钥 SSH 会话不可访问 Windows Docker credential helper 的 logon session 错误。采用单次临时 CLI 配置，显式匿名 `auths.ghcr.io={}` + 已确认本机 Linux Engine 下载固定镜像，完成后删除临时配置；不删除用户凭据，不变更 SSH/Docker 认证。空 `{}` 仍自动探测 wincred，不能当作已解决。测试脚本只检查镜像存在，不隐式下载。

测试包仅用于内部验证，含固定 allowlist、ZIP SHA-256 和编译依赖根目录 LICENSE/NOTICE 等文本，不打包节点私钥/注册 key；远端保留独立解压测试目录和已下载镜像，临时节点状态与本轮容器已清理。执行和边界见 [Windows 测试说明](../poc/selfhost/WINDOWS_TEST.md)。

当前仅 Mac Intel 和 Windows x64 实际运行通过。Linux x64 / Mac ARM64 仍是构建证据，验证码/授权后端、产品 TLS 桥接、多租户撤销和真实跨网未验收。补平台运行与双机服务环境准备仍是后续任务；公开服务部署、资源购买与防火墙变更需要另行确认。下一步不重复本轮 Windows 本机验收、不宣称最终架构已选定。

## 2026-10-05 自托管连接生命周期修复（历史）

当前仍为独立 `codex/connection-architecture-poc` 分支、tsnet v1.102.5 / Headscale v0.29.4 / Go 1.26.8；只修改本机隔离 PoC，不改产品安装包、系统 Tailscale 或真实凭据。下方首轮未通过描述保留为历史。

退出阻塞定位：`TS_DEBUG_ALWAYS_USE_DERP` 的模拟 UDP 连接在 rebind 时被替换但原连接没有关闭，接收线程等待 `blockForeverConn.ReadFromUDPAddrPort`，WireGuard 关闭等待 `closeBindLocked`。改用 `TS_DEBUG_NEVER_DIRECT_UDP`：保留真实 socket、禁止直连探测，仍检查实际 DERP 路径；没有 fork/修改依赖。8 秒关闭兜底仍保留，超时不计通过。

单独复现了正常退出后的重连失败；禁用 `TS_USE_CACHED_NETMAP` 后恢复。该开关同时影响磁盘网络图恢复与 TSMP disco 广告，所以目前只确认这组缓存功能相关，未将某条内部路径断言为唯一根因。本 PoC 在线启动须取得当前控制面网络图，不承诺离线缓存启动；保留节点私有状态，不生成替代身份。新增目标网络图成员检查，避免未知 IP 拨号回退到系统网络。

回归入口：仓库根目录 `zsh poc/selfhost/verify.zsh`；在 `poc/selfhost` 执行 `python3 smoke.py --check-reconnect` 加测连续三次客户端重启。默认门禁已纳入客户端重启、服务端重启后固定数据与错误 token 检查、原 IP/节点数和正常关闭。构建命令与边界见 [PoC README](../poc/selfhost/README.md)。跨平台仍须区分交叉编译和实体运行；下阶段先做各平台独立 helper 的运行验收，再推进授权后端/真实 TLS 桥接；公网部署及资源购买须另行确认。

本轮实际结果：`GOPROXY=off zsh poc/selfhost/verify.zsh` 零退出，依赖校验、Go 单元测试及完整本机门禁通过；`python3 smoke.py --check-reconnect` 零退出，连续三次客户端重启及服务端重启后传输/错误 token 拒绝/身份保留/正常退出全部通过；Windows x64、Linux x64、Mac ARM64 交叉编译通过并核对文件架构，未进行目标机运行。所有测试 fixture 进程及临时身份已清理；无 `cleanup_timeout`，不靠强杀将节点记为通过。

## 2026-10-05 自托管连接 PoC 首轮（历史）

用户确认托管 Tailscale 商业许可/收费不符合规划，授权尝试新路线。候选改为 tsnet + 自托管 Headscale + 自托管 DERP；托管服务路线关闭。工作仍在 `codex/connection-architecture-poc`，产品 Swift/C#、安装包和原有 Tailscale 配对未修改。

新增隔离原型 `poc/selfhost`，固定 Go 1.26.8、Headscale 0.29.4、tsnet 1.102.5；工具链在忽略的 `artifacts/connection-poc`，官方下载校验通过，不安装系统 Go。Headscale/DERP 仅在 loopback 上运行，DERP 临时证书采用 SHA-256 pin；注册 key 与随机会话 token 仅在父子进程管道中传递。关闭日志上传，不使用托管控制面/公共 DERP；没有部署公网服务或购买资源。

本机已证实：免浏览器注册、自建 DERP 固定载荷、错误 token 双端拒绝、第三注册节点被 ACL 拒绝、被控节点重启后身份/地址保留且节点数仍为 3。Go 单元测试和 `go mod verify` 通过。Windows x64、Linux x64、Mac ARM64 helper 已交叉编译，不能记为各平台运行通过。既有产品 Mac 125/125 是收尾基线，本轮没有因 PoC 重测产品或 Windows。

**完整 PoC 验收未通过。** v1.98.4 首轮关闭阻塞，升级至 v1.102.5 后仍复现；栈停在 `tsnet.Close → LocalBackend.Shutdown → userspaceEngine.Close` 的 WireGuard 关闭调用。先停止 WantRunning 再关闭的试验未解决，未保留该无效改动。helper 现给 Close 8 秒期限，超时明确报 `cleanup_timeout` 并非零退出，由进程退出回收资源；不是优雅关闭已修复。`verify.zsh` 全部网络检查后仍非零退出，最终 `FAIL graceful_shutdown_unresolved_network_checks_passed`。

另一个未通过项：同一客户端退出后重新注册成功，但再次 Dial 报 `dial_failed`。错误 token 检查现放在同一存活节点中并由服务端 `token_rejected` 确认；这不关闭重连问题。所有本轮 fixture 进程和临时身份均已回收，没有清理原 Tailscale 或产品凭据。

下一切片先定位/解决独立 helper 的 WireGuard 退出和客户端重连问题，明确生命周期与状态持久化，再考虑跨网、设备码/验证码后端和真实 TLS 桥接。暂不把候选宣布为最终底座。复现：仓库根目录 `zsh poc/selfhost/verify.zsh`；跨平台构建：`zsh poc/selfhost/cross-build.zsh`。重连复现：在 `poc/selfhost` 运行 `python3 smoke.py --check-reconnect`，本机已确认输出 `registered,dialing,closing,cleanup_timeout,dial_failed` 后 `FAIL client_reconnect_failed`。准备条件和边界见 [PoC README](../poc/selfhost/README.md) 与 [架构记录](CONNECTION_ARCHITECTURE_POC.md)。

## 2026-10-05 收口与下一阶段入口（优先于下方历史记录）

用户已确认双向文本剪贴板及 Command+C/V 修复测试通过，当前产品基线为 `8d88ac0`，验收记录为 `dccda38`。剪贴板切片关闭；下方各轮“待双机验证”是历史状态，不再作为重复验收要求。功能范围限于显式纯文本复制粘贴，不扩大到文件/图片/富文本或后台同步。

本轮在正常 macOS 权限下运行 `swift test -c release`：构建成功，**125/125 passed，0 failures**，包括真实 TLS 回归。首次沙箱执行因 Swift 缓存访问受限失败，不是产品测试失败。Windows 本轮未重新执行，保留此前用户通过反馈，不新增 Windows 证据。

下一阶段调整为“免 Tailscale 登录的最终连接架构 PoC + 商业可行性确认”，建立独立 `codex/connection-architecture-poc` 分支。自动重连、外部 Tailscale 状态深度集成、H.264 与多平台角色扩展顺延；现有产品保持可回退基线。实施边界、验收和商务问题见 [连接架构 PoC](CONNECTION_ARCHITECTURE_POC.md)。

保留但不阻塞本次架构研究的待办：

- Windows 实体键盘 Ctrl+Alt+Esc：环境受限待验收，RDP 不替代证据。
- 干净 Windows（无 .NET SDK/运行时）和干净 macOS（无开发工具）安装启动：环境未准备，待验收。
- Apple Developer ID/公证、Windows Authenticode 及跨升级 Keychain 稳定身份：延期，未实现；用户先前明确暂缓签名切片。
- 长时间耐久、帧率指标及未明确反馈的密钥界面生命周期：保留待验证，不因剪贴板通过而关闭。

工作区 `README.md` 中代理 pull/push 命令是用户已有修改，保留且不纳入本次提交。不删除或重置现有 Tailscale、Keychain、Windows 配对凭据，不部署服务器或购买资源。

## 已通过：Mac Command+C/V 弹出 Windows 开始菜单修复（2026-10-02，最新）

双机首次验证发现，在远程画面按 Mac `Command+C/V` 会弹出 Windows 开始菜单；`Control+C/V` 仅能操作 Windows 自己的剪贴板，不能替代跨设备同步，因此该轮结果判定不通过。根因是 AppKit 先发出 Command 的 `flagsChanged`，旧实现立即将其映射为 Windows 键按下，随后即使 C/V 被特殊处理，Windows 键的按下/释放仍会打开开始菜单。

Mac 现改为延迟 Command：Command 刚按下时只在本地跟踪，不立即镜像到 Windows；若下一键是 C/V，则消费该 Command 并执行跨设备剪贴板，整个序列不产生任何 Windows 键事件；若下一键是其他键或鼠标按下，则先补发 Windows 键，再保持原有 Windows 快捷键行为。单独按下并释放 Command 仍补发完整 Windows 键按下/释放。失焦、停止、断开与发送失败会清空所有延迟/消费状态，不留下远程持有键。Mac 全量测试现为 **125/125 passed**，新增“剪贴板快捷键绝不镜像 Windows 键”和“其他快捷键仍可补发 Command”两项回归；Windows 源码与协议未改变，无需重装 Windows。

用户已在安装修订 `8d88ac047cc9` 后完成双机复测并确认通过；此前 `Command+C/V` 弹出 Windows 开始菜单的问题已关闭，双向复制粘贴按本节清单通过。签名切片已按用户要求暂缓；下一项以本文件顶部 2026-10-05 的连接架构入口为准。

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
