# 自主连接链与桌面界面接入

2026-11-05 r212（承接 r203–r211）。双端独立 GUI 已接入预备/同意与桌面适配，native 异步协调和 GUI 双机验收尚未完成。r201 的固定 TLS 载荷验收继续有效，不重跑已退休工单。

## 已落实的边界

Mac 新增 `OwnedDesktopHandoff`，仅供固定摘要产品进程从可信 owner 的继承匿名管道接收含明确桌面权限的五字段 bootstrap。接收方固定连接 `127.0.0.1`，端口来自当前 owner 已持有的本地桥；目标设备和证书 pin 由原签名身份/描述符验证路径提供。对象只消费一次，不读写旧配对 Keychain，不执行首次证书信任，不允许 host/root/控制/剪贴板等额外选择字段，不提供密钥序列化接口，普通打印已脱敏。

wire 要求 Go `json.Marshal` 的 `Fingerprint/Key/Permission/Port/Target` 五字段规范 JSON，不含末尾换行。`Permission` 只能为 owner 已核验的 `desktop-view` 或 `desktop-control`；旧四字段或 `fixed-payload` bootstrap 一律拒绝。严格比较规范字节，拒绝重复字段、未知字段、另类数值编码、非规范 base64/小写 hex 以及空/越界端口。管道 reader 负责有界取行和终端 EOF；不能从网络、argv、配置文件或公共本机 socket 获取这个对象。**解析成功不认证发送者，也不授予自主网络访问权限。** 原 owner 必须始终持有并验证会话，目标发布证明、双向身份交换、签名 TLS 描述符、ACTIVE watch、WhoIs 与截止保持原实现。

只读适配器复用原 TLS/JPEG 实现及仅当前目标可读的内存 pin；控制适配器复用原 TLS 输入驱动，同时要求 `desktop-control` 与本机控制许可；仅查看授权不能开启输入。两个入口互斥消费，拒绝/取消后不可重新开启，不提供剪贴板回调。此许可不替代 Windows 的明确同意。`discard` 只销毁未用 handoff；已创建客户端仍必须由 GUI 生命周期明确停止，网络完成 callback 不能冒充 owner/OS 进程的完整 join。

Windows 新增 `TlsProbeServer.RunOwnedDesktopOnceAsync`，固定监听/peer 为 loopback、端口 47478，匹配已验收 `streamgate.ServeTLS` 的固定本地目的地。只服务一次，结束即释放监听，不在同一次许可下自动接下一次会话。创建 listener 前必须有本机共享许可；必须提供合法的 `DesktopSessionPermission`；输入 options 还必须同时有 Control 范围与独立本机控制许可，剪贴板读取/写入回调一律在 bind 前拒绝。仍由原 TLS/HMAC/capability gate 通过后才构造 capture/input sink，复用原释放按键/按钮及 source disposal，不增加绕过认证的捕获接口。旧 `RunFixedPayloadBridgeAsync` 仍不能接受 capture/input 回调，旧 GUI 入口也未切换。

## r204：签名权限先于原生载荷

原 authority/transport 仅签发并接受 `fixed-payload`，因此不能仅替换 TLS 产品二进制就启动真实桌面。新增 `DesktopPermission` 的 `desktop-view` / `desktop-control`，不包含剪贴板或无人值守；`VerifyGrant` 保持固定载荷限定，新 `VerifyDesktopGrant` 必须由可信本机代码提供精确 expected scope，不把 control 和 view 互相替换。

独立 `OpenDesktopAuthorization` 仅供显式的新设备授权 Store 组合使用，目前没有线上入口调用。目标邀请、控制端 claim、目标 Allow 批准均签入相同 permission；incoming/status 返回原申请范围，省略或错范围的 Allow、自批准、第三端批准、签名后改范围均拒绝。Decline 可以不声明正向范围；不会产生 grant。原120秒邀请、300秒授权、nonce、持久速率预算、同目标单活动授权、撤销规则保持。

Store 将 DesktopAuthorization 模式和每个邀请/申请范围持久化，重新打开校验 scope 与同一申请/签名 grant 一致。默认 Open 不接受新模式，新构造器也不能打开已有 fixed-only Store，失败不改快照或重建 issuer。没有线上 Store 迁移或任何旧文件回写。既有 Authorized、默认网络后端与业务 IPC 继续拒绝桌面授权；r206 的显式桌面后端另行接通如下。AuthorizedDesktop 本身仍只核查设备授权账本。

**r210 已补同一固定摘要 child 的预备/同意/批准绑定/激活能力和 Mac 协议状态模型；下一组接真实 GUI 按钮、native 协调与新鲜服务组合。** 不得将 VerifyGrant 全局改成接受任意已知 scope，也不能从网络报文计算 expected scope 来代替本机角色/许可策略。新五字段 handoff 的 scope 只能来自该已校验 grant，不是父帧自己声明就有权访问。当前真实 GUI、受信桌面产品构建、人工同意及跨机验收尚未完成。

## r205：客户端明确策略、watch 与私有载荷接线

新增值类型 `invite.GrantPolicy`，其零值仅代表既有 fixed-payload，桌面策略必须由可信本机组合以精确 DesktopPermission 构造。字段私有，JSON 编解码均拒绝，不能把配置或 peer 报文解析成策略；session.Config 的策略字段也不参与 JSON。新 `ForPolicy` 验证入口覆盖 TLS 描述符、session identity、verifier permit，既有函数继续使用零值 fixed 策略，原签名域、nonce、角色、binding 和有效期检查未放宽。

明确策略贯穿 StateWatch 启动/每次查询/Snapshot、ManagedGate 校时/watch/gate、ProfileSession 的相同 join 生命周期，以及 session 双向证明/secret 交换/opaque Tunnel。StateCache 同时绑定本机策略、同一 grant/binding 与绝对有效期；inactive 仍永久锁闭，延迟收到的状态不重置 TTL。校时必须先验证本机 scope 下的 grant 签名，再发 fresh challenge；校时/状态响应混带其它回执类型立即拒绝。所有原 TCP/密钥交换3秒、状态3秒、授权300秒及产品启动8秒保持。

新增独立 `StartBoundDesktopProduct`，要求本机 expected scope，先核验 grant 与签名 TLS 描述符，再经同一匿名管道交付五字段 bootstrap。控制端和目标端 ready 均须回显完全一致的 Permission，旧 PONG ready、未知字段或错范围拒绝；目标错范围在发布证书/会话 secret 前拒绝。它不接受旧 PONG Observation 作为桌面遥测。失败仍返回同一已启动 child 供原 owner join，不另开进程、不强杀、不把超时当退出。

installeddevice 的内部授权/节点 verifier/session identity 验证点保留同一策略，既有 runner 继续构造 fixed 默认策略。r208 另行导出下述桌面组合；原 RunProduct 仍 fixed 默认。**既有默认服务端仍拒绝 desktop grant**；r206 增加显式后端支持，但尚未部署新 backend，没有改现有 Store、profile 或控制面。不能据此宣称已取得桌面网络身份或真实画面/输入。

## r206：显式服务端 scope 后端

`NewDesktopProvisionBackend` 仅接显式 desktop Store，私有指针绑定同一个 Store。普通 backend 在 desktop Store 上拒绝网络 mint/bind/state/校时/verifier/reconcile/maintenance/fence，不消费另一个 backend 的有效网络请求或触碰其 driver、全局 readiness、规则。公开 Store 既有正向网络方法也保留 fixed 模式；trusted backend 使用独立私有 mode 组合。修改公开 Store 字段不能把已授予的 opt-in 转移到另一 Store；没有新增 peer 可选 mode、地址、root 或 issuer 字段。

新后端从权威自身持久的**同一申请与完整 grant**选择范围，再核验原签名、nonce、身份、TTL 和批准状态。只有有效 issuer 签名但未入账的 scope 不获得凭据或证明。它接通 enroll/bind、共享最小规则投影、ACTIVE、grant clock、TLS possession/descriptor、registered session identity、verifier permit、撤销/未知 Mint 收尾及精确 node retirement。view/control/fixed 三类已批准 grant 可以共存于一个显式 Store；只撤销其中一个只回收对应两端。原共享串行锁、未知 Mint 不重发、两阶段 cleanup、driver 失败 fence、绝对过期和固定端口规则保持。

重开检查新增：非 fixed provision 必须仍关联同一持久已签 scope 申请；浮动/错 scope 记录拒绝打开，不重写或清理未知记录。正常已批准的混合范围重开后保持 issuer 与投影且不重新 Mint；已撤销记录不能复活。默认 Open 或旧 Store 不自动迁移为新模式。

此后端尚未部署或提供 GUI 新包。r207 增加下述显式 broker/headscale 函数；既有 daemon、deviceservice 与 native-enrollment-authority 可执行入口继续使用默认构造。不要为接线直接放宽默认函数，或在已部署旧 Store 上开启新模式。

## r207：同范围独立撤销、终态证明与服务组合

`grantrevocation.NewDesktop` 在可信本机组合中明确选择 view 或 control，再打开同一已验证 profile 下的独立 HTTPS 客户端。策略保存在私有值字段，不能由 grant、parent command 或响应推导；原 `New` 仍 fixed-only。观察、最多2秒 authentic not-before 等待与 provision 前 Pin 均校验相同本机策略、签名、角色、时间和完整 grant。Close 仍串行等待在途请求，signer 归外部 owner，不因业务 child 出错提前释放。

独立状态确认严格匹配已观察 RequestID 与该策略的 Permission：ConfirmBeforeProvision 仅接受明确 denied/revoked，不把 pending、过期、profile 回退、缺范围或混合回执当成收尾许可；ObserveRequest 更换旧请求也必须取得同范围明确终态。Pin 后两角色 ConfirmRevoked 只读，只有目标 owner 可显式 Revoke。丢失 mutation 回执时，下一次明确调用先查询同一申请；只有精确 revoked 才完成，只有精确 approved + 原完整 grant 才允许该次显式重发，未知继续保留。旧 fixed 路径也拒绝 scoped 终态和夹带其它结果字段。

revoke mutation 的服务端正常回执仍只有 `status=revoked`，不添加选择字段；此回执只确认已签的固定 GrantID 操作。它与携带 RequestID/Permission 的 status 响应严格区分，两者都不构成产品/watch/node/进程 join 证据。认证维护/原生删除的独立门禁保持。

`broker.StartDesktopHeadscaleNetworkSupervisor` 使用同一现有固定 Headscale driver、registered device proof、LiveVerifier 与监督收尾，显式构造桌面 backend。默认 StartHeadscaleNetworkSupervisor 仍 fixed；没有新增 HTTP/IPC/daemon 配置的 mode、endpoint、root、issuer 或 namespace 字段。新函数不创建 Store/verifier、不接管旧 owner；Stop 必须先 join network workers/maintenance、确认规则撤回，再由调用方停止 verifier/关闭 Store。测试覆盖 verifier fault 期间 Mint 未返回时 Stop 不能报已 join、取消后的凭据不返回、网络停止后独立 device revoke/status 仍可用。

**线上接入前的约束**：现有 `cmd/native-enrollment-authority` 是旧维护验收专用，绑定旧维护 Store 摘要、candidate 路径和持久 verifier；不能通过去掉其 hash/lock/paused 检查或改旧 Store 模式来复用。应给桌面准备明确的新鲜 TEST 服务根/profile，并先核对现有测试服务的规则 writer 与 verifier 实际所有权。Headscale driver 对隔离 PoC 的策略执行全量替换，另起不同 policy 目录并不自动隔离同一控制面的规则，禁止让两个 writer 并行争抢。此次没有 SSH 或修改远端服务，尚未验证新的线上组合。

## r208：独立桌面 runner 与维护 owner

新增 `CoordinatedDesktopSession` 与显式 TEST tunnel 版本，进入 runner 前固定 expected scope，生产/不完整组合拒绝，不能通过私有 open 帧选择策略。原默认 runner 保持 fixed。desktop runner 的 ready 必须回显同一 Permission；业务只允许该范围的邀请、claim、Allow，Decline 可不声明正向范围。身份/签名时刻/nonce/证明选择字段、错范围网络 grant 和生命周期命令不进入业务路径。成功回复按动作、同 RequestID、完整 grant 范围及精确结果字段校验，approved grant 还须签名/角色合法；pending/idle/expired 可作界面状态，不被当作活动或收尾证明。

相同私有策略贯穿校时、节点 verifier、ACTIVE watch、session identity、streamgate 和 `SessionRuntime`。新增 `DesktopProductSession` / `RunDesktopProduct`，只能消费同一 runner 的策略；默认 RunProduct 不能升级桌面，桌面不能消费旧 PONG Observation。双角色仍经相同 StartBoundDesktopProduct 完成签名 TLS 描述符/证书持有证明、单次 HMAC 交付与五字段 handoff，原监听/拨号/WhoIs/secret 截止及全部资源 join 顺序保留。双端并行 provision/ACTIVE 与“目标就绪后控制端新 identity/gate”协调不变。

`productmaintenance.StartDesktopTest` 在打开原生身份租约前固定本机范围，只重开既有 binding，核对原公钥，固定摘要 child 的 ready 必须回显范围。ConfigureRevocation 从该不可变策略建立独立 NewDesktop 通路，建立前拒绝业务；默认 StartTest/ConfigureRevocation 不升级。未知 provision 仍先 Pin 原 grant、锁住单次尝试并保留 child/signer；错误/混合业务回复锁闭但不提前丢失撤销 owner。维护器的 privatepipe 解码同步拒绝重复/未知字段，多余或错范围 ready 不成为可用 owner。

默认 `cmd/installed-autonomous-product`、旧构建器与两个正式 GUI 可执行入口仍走 fixed/旧路径，不能用旧二进制装入桌面 grant。r209 另行提供下述同源、独立编译的桌面 native 产品/监督器；本机 expected scope 固定在构建中，没有 parent/peer 的 mode 选择字段。

## r209：编译期 scope 与私有管理路由

新增 `CompileAutonomousDesktopProductTest` / `CompileDesktopProductSupervisorTest`：只接受 TEST、精确 view/control、本轮固定产品/签名 helper 摘要与合法安装路径；受信调用方可显式选原 TEST tunnel，约束 endpoint/端口不变。通过 `remoteapp_desktop_view` 或 `remoteapp_desktop_control` 构建标签，编入 `compiledDesktopPermission` **源码 const**，不能用 JSON、argv、环境或 `-X` 链接变量切换。没有新增可运行时解释的 mode。两个标签同时选会因 const 重复而编译失败；默认不带标签仍 fixed。

native 产品与监督器共享既有生命周期源码，按标签分别生成独立固定摘要产物。desktop native client 只选 r208 的 CoordinatedDesktopSession/桌面产品 work，禁止生产并不创建 PONG Observation；desktop supervisor 只选 StartDesktopTest、同范围独立撤销，ready metadata 回显范围。保留原生 binding、profile/租约、signer、维护与卸载协议。新构建器保持 Mac native client 的 CGO=1 和 Windows amd64/CGO=0，不因交叉构建静默丢失原生日志能力；optional removal 仍固定维护端口/独立卸载器摘要。

同时修复接线缺口：旧 `supervisedcontrol.Serve` 的业务过滤器会在 r208 维护器之前拒绝桌面 Permission。新增独立 `ServeDesktop`，桌面标签的 Windows 命名管道/Mac 私有 Unix listener 显式选择它；仍由原 OS listener 做 same-user peer 检查，响应/业务 scope 继续由 r208 owner 核验，不从来帧计算策略。既有 Serve 保持 fixed。新服务接受同 scope invite/Allow 与原受限 quiesce/revoke/start/preprovision-terminal；错 scope、身份选择、生命周期越界拒绝。

旧 `product_count` 在桌面 supervisor、ServeDesktop 和 Controller.Count 三处拒绝，不能到达 native child，更不能触发 nil PONG observation 导致桌面 owner 意外退出。原 fixed 查询保持。后续需要单独接真实帧/输入遥测，不能恢复 PONG 计数充当桌面验收。

当前新增的是 native 产品/监督器构建函数和标签接线，旧 fresh-builder 调用方尚未切换。Swift/C# GUI 尚无预备/请求确认/激活阶段的完整父管道实现，不能提前签发双机桌面包或把旧 TLSBridgeProbe 当真实桌面产品。r210 已实现下面的同 child 预备能力，但 native main 尚未传入它；仍需真实 GUI 和批准协调接线。

## r210：同一 child 的预备、同意与激活

`clientruntime.PrepareDesktop` 在 TLS 启动前按固定 Spec/SHA-256 启动一个 child，私有 `prepare` 命令只包含 Op/Permission/Role/Self，要求精确 `desktop_prepared` + 同 scope/role。准备阶段的8秒仅覆盖 GUI 初始化；child 此时不得打开 TLS、capture 或 input。准备返回失败时也可能持有 partial owner，必须 Stop/实际 join，不能丢弃。

`PreparedDesktop.Decide` 只针对经 HTTPS/owner 核对的同 RequestID/controller，要求本机 controller 身份或非自身的 target 请求。只有同 child 的完整 `desktop_decision` + 精确 RequestID/Permission + 显式 Allow bool 才是决定；不是 grant 或撤销证明。人工等待独立有界120秒、发生在 TLS 启动前，不能延长服务端原120秒申请/300秒grant或任何profile期限。拒绝、错字段、缺 Allow、未知回执均不可重做这次决定。

可信 owner 收到匹配 approve/status 成功回复后，才调用 `ObserveApproved`，核对签名、参与者、scope并保存完整 grant；**grant 本身不签 RequestID**，因此调用方必须按r208的真实 HTTPS 回复关联，不能自行给 grant 贴一个 RequestID。历史验签仅用于绑定，激活仍由原校时/ACTIVE/session/WhoIs与当前有效期检查把关。拒绝时需另行向真实 authority 明确签名 Deny；本地拒绝不能冒充服务端终态收尾。

`Product.Prepared` 是不参与JSON的受信能力，只允许桌面入口、同 Spec（含参数与摘要）、本机角色、issuer、完整 grant 和已同意请求。激活前先占用一次尝试，再查询 TLS 描述符；查询未知也不能换 child 或重试。目标对同 child 发 selector-free `activate`，控制端对同 child 交付原五字段 bootstrap。复用原证书持有证明/单次 HMAC/TLS协议和8秒启动预算，不调用第二次 process Start。当前 prepared 组合只允许内存临时证书，不接受旧测试证书 scope/reopen/open callback 的混用。

Stop 会取消在途同意/激活，等同一 child、EOF/reader及私有操作完成；超时仅保留该 join，后续 Stop 不重发决定/激活、不强杀。child 退出本身也不能代替仍在途的描述符查询完成，更不是全 session/watch/node/signer 的 join。outer owner 仍须按原生命周期停止并 join 所有资源。TLS 描述符与证书证明同时加严额外字段/混合回执和取消后返回；默认固定路径的正常回执不变。

Mac 新增 `OwnedControllerPreparation` 协议模型：仅controller角色、规范私有prepare/consent帧，绑定自身ID与同一请求，拒绝重复/未知字段、错scope和重放；只有显式本机决定之后才能一次消费同scope的 `OwnedDesktopHandoff`。取消/拒绝不创建客户端或网络。模型方法不是用户同意证据，真实App须从新鲜本机按钮事件调用，并拥有reader/客户端/进程join；本组没有修改或打开GUI窗口。

下一组需把此能力接入 native main/runner 的真实 incoming/claim/approve/status 与 Swift/C# App。注意旧业务管理Exchange有20秒预算，不能把120秒人工等待塞进它并超时后重发；应让同一个owner持有可取消且实际join的待决定操作，向限定管理端点提供短查询状态，最终仍只接受该GUI决定。保留原单次provision、独立撤销和取消/正常收尾门禁。之后再接真实连接发起、画面/输入遥测、窗口停止与fresh builder。

## 下一组管理与界面接线

1. 选择独立、固定摘要的桌面产品构建入口。不得用公开 IPC 的 `mode/native/host/path` 字段把旧 PONG fixture 或业务命令升级为画面/原生输入能力。
2. 接入已有 register/invite/claim/incoming/approve/status/revoke 与私有监督协议。把经过原 owner 验证的当前申请/角色显示给 Windows 用户，明确同意关联该申请；用户拒绝不能继续 provision 或启动桌面载荷。不允许自动化脚本模拟点击来充当本机许可证据。
3. 使用r210预启动并保持同一个固定摘要桌面 child，人工等待放在原8秒TLS启动之前，后续仍由owner交付bootstrap、发布证书证明并拥有该child。不能延长3秒TCP/密钥交换或有效期，不能将未知状态换进程重试。预启动失败、拒绝或过期也必须实际join。
4. Windows 复用每次共享许可、物理 Ctrl+Alt+Esc 和本机停止；后台管理保持隐藏 Interactive。真实 capture/input 只在允许的用户桌面执行，默认只读。Mac 复用现有画面 canvas/焦点输入释放，使用新的一次性 handoff，禁止旧地址/配对密钥回退。
5. 接线后验证取消/断网/撤销/窗口关闭与 owner 的正常及故障收尾，严格等待同一 child、watch/node/signer/reader 实际 join。再启动一份全新签名双机工单，记录实际画面增长、输入事件与本机可见效果、拒绝不产生输入，以及双端停止/撤销。组件 JPEG/sink fixture 与 PONG 都不是这些效果的替代。

GUI 的停止/窗口关闭还须先通知同一 owner 进入 quiesce/revoke/join；不能让 GUI child 自行退出后，把监督器检测到的意外进程结束改称正常成功。pre-arm 必须保留同一固定摘要 child，人工同意、拒绝、超时均要由该 owner 实际 join，不能另起替代进程。

## 当前验证与证据

- Mac：`nice -n 10 swift test --package-path macos/RemoteController --scratch-path artifacts/connection-poc/swift-desktop-cache --jobs 2 --filter OwnedDesktopHandoffTests`，r204 最终 4 项通过，含 view+本机许可仍不能开启输入、旧 fixed bootstrap 拒绝。构建也编译现有 GUI 及协议模块；测试只创建未启动的控制客户端，不读取 Keychain。
- Windows：唯一随机临时源码目录仅框架依赖构建，`RemoteProtocol.Tests.dll --owned-desktop-validation`，r204 最终 6 项通过：未同意在 bind 前拒绝、view/未知范围不得启用输入、view 只发布 JPEG、control 不包含剪贴板、错误 HMAC 不创建 source/sink、认证 TLS 同时承载 fixture JPEG/键鼠并在 Disconnect 后释放 held input、dispose source 及释放监听。没有真实 capture/input、DPAPI 或持久凭据操作。
- Windows 构建 `-m:1 -p:UseSharedCompilation=false --disable-build-servers`，PowerShell 为 BelowNormal/双核 affinity；本机 Swift nice 10、jobs 2，重型构建串行。未构建新双机产品包；r204 的 Go 组件检查见下。
- r203 原证据 `artifacts/connection-poc/r203-owned-desktop/`：`swift-component.txt`、`windows-component.txt`、`component-owner.json`、`windows-component-postcheck.json`。Windows 原组件命令实际 exit0；后续只读核对测试路径相关进程为 0、测试根与两个上传文件缺席、47478 空闲。首次清理客户端因非 UTF-8 PowerShell progress 的解码异常未收回执，已通过独立只读检查解决，未重发删除。

现有 remoteapp 自动续办仍 ACTIVE。无用户操作阻塞，当前没有新鲜双机工单；历史未知 owner、根、锁及故障材料均保留。

### r204 验证增量

Go invite/installeddevice 相关包回归通过，最终新桌面范围检查与旧签名 wire golden 检查通过；空 permission 不改变旧请求签名字节或旧结果 JSON。首次扩展负向形状 fixture 触发原持久邀请轮换预算，已用各检查间推进测试时钟隔离，保留原限额，无产品规则放宽。Go 全程 GOMAXPROCS=2 / GOFLAGS=-p=2 / nice10 / 受信离线缓存；未运行无变化完整项目回归。

材料在 `artifacts/connection-poc/r204-owned-desktop/`（Go/Swift/第一轮 Windows 组件与清理）及 `r204-desktop-final-owned-desktop/`（补齐剪贴板拒绝后的最终 Windows 六项与精确清理）。两个 Windows 组件命令均实际 exit0，无真实 capture/input 或持久凭据操作；组件临时根、上传和相关进程检查通过。未准备新鲜双机产品包，不计 GUI 整链通过，未改部署服务、Nginx/443 或历史现场。

### r205 验证增量

相关 invite/session/streamgate/clientruntime/installeddevice 五包检查通过；新范围的 HTTPS watch、双向 secret/本地 TCP opaque worker、state cache 与 ManagedGate 定向 race 检查通过。随后针对最后加严的混合回执拒绝复验既有 StateWatch/Managed/Profile 默认路径，并覆盖桌面私有 child 的明确范围/ready/正常退出与拒绝 PONG Observation。证据在 `artifacts/connection-poc/r205-client-policy/`。全部本机 Go 使用既有限额与离线缓存，重型检查串行；没有重新构建 Swift/Windows 组件或重开新鲜双机工单。

这些是内存 issuer/signers、私有 HTTPS fixture、本地 TCP 固定字节及 Go fixture child 的协议/生命周期检查；不是 OS signer 到线上服务、Headscale/WhoIs、实际 Swift/C# 桌面程序、真实屏幕键鼠或 GUI 整体验收。下一步按上面的服务端授权与限定 owner 入口继续，不得对旧 fixed Store 直接开启 desktop 模式，也不得复活已退休安装。

### r206 验证增量

invite 包回归通过；六个新后端边界（含两范围的实际本机 HTTPS/API 组合）定向 race 通过；deviceservice/headscale 消费方回归通过。增加 approved 混合范围重开及完整 RTT 起点检查后的定向复验通过。测试 driver 在内存中模拟 Mint/Observe/live-proof/Cleanup/投影，绝不代表实际 Headscale、节点持有证明、OS signer、画面键鼠或跨机 GUI 已验收。

覆盖包括：scope 批准→HTTPS enroll/verifier/bind→signed ACTIVE→TLS certificate possession/descriptor 与 registered session identity→scope watch 观察 revoke→精确 cleanup/node retirement→重开不复活；凭据不落盘；无关模拟资源保留。另覆盖错 mode/backend Store 重绑完全无 driver/fence/nonce 副作用、未入账 issuer-signed scope 拒绝、未知 Mint 后重开仅 cleanup 不 remint、三范围共存与单范围撤销、未知 scoped record 打开失败不回写。证据 `artifacts/connection-poc/r206-desktop-backend/`，全部限额本机检查，无 SSH、服务部署或新双机工单。

### r207 验证增量

grantrevocation/broker 包检查通过；新增 scope 边界、本机实际 HTTPS（含故意丢失撤销回执）及 kernel peer broker/HTTPS 接线定向 race 通过。覆盖 view/control/fixed 独立传输、原客户端关闭后同一授权撤销仅一次、双方只读终态确认、provision 前明确拒绝；授权 scope 互斥、混合 approved 回执不得引发重发、默认客户端不得接受 scoped 收尾证明。productmaintenance/installeddevice 消费方回归通过，保持原 lifecycle 与默认固定策略。

证据 `artifacts/connection-poc/r207-scoped-revocation/{revocation.txt,broker.txt,scope-race.txt,consumers.txt,final-boundaries.txt}`。内存 issuer/signers 与 fixture driver，真实本机 HTTPS/Unix socket/Store/监督线程 join；没有 Headscale CLI、OS signer、真实画面或键鼠，也没有新双机包。现有四组 TEST 完成证据不变，尚未完成本批 GUI 验收。

### r208 验证增量

installeddevice/productmaintenance 包回归与新桌面边界 race 通过。真实本机 HTTPS 下，两种范围均完成 runner ready→注册→邀请/claim/incoming→自批准拒绝→明确 Allow→批准后 status；runner/signer 实际结束后，同身份的独立租约/传输仍能撤销，两参与者可只读确认。签名者为内存 fixture，不是 OS signer。

另验证 scope 经 ACTIVE watch→新 session identity→streamgate/runtime 保留、控制端仅一次触发；同一固定 SHA-256 Go child 实际接收 scoped 私有 handoff 并正常 Stop/exit/reader join；默认产品/错 scope/PONG Observation 在网络前拒绝。维护 owner 测试只读既有内存 binding，检查 lease 先关闭，拒绝旧/重复 ready，独立撤销建好前拒绝业务，错 scope/混合回执锁闭，以及未知 provision 仍已 Pin 且保留 signer，显式 Release 等同一 child 退出。

材料 `artifacts/connection-poc/r208-desktop-runner/`。真实本机 HTTPS、签名协议、Go 子进程/监督器和 join，node/issuer/credential/capture/input 均为 fixture；没有真实 Headscale、Swift/C# 桌面载荷、画面键鼠或双机 GUI 验收，没有远程部署/新包或历史现场操作。全部本机构建和测试采用原限额、离线缓存与串行任务组，Windows vet 为交叉静态检查。

### r209 验证增量

构建器负向与 argv 捕获检查覆盖两 scope、Mac/Windows、direct/TEST tunnel、监督器/客户端、非法 scope/生产/未固定摘要/维护地址和只提供半边 removal 组合；捕获编译器为脚本 fixture，不计真正原生构建。default/view/control 三种 Go 命令包组合实际编译通过；同时两个 scope 标签的实际编译按预期拒绝，只出现重复 const 错误。两 scope 的 Windows amd64 交叉 vet 通过，不代表 Windows 原生运行。

supervisedcontrol 默认路由回归通过；新桌面管理路由、取消后在途 reader/handler join、维护 owner 拒绝 PONG 又不污染业务状态定向 race 通过。管理路由新测试使用真实本机 TCP + fixture backend，不包含 OS signer 或新 kernel peer 验收，原受保护监听器未改认证策略。证据 `artifacts/connection-poc/r209-desktop-build/`，无 SSH、服务部署、安装根或 GUI/真实画面键鼠操作。

### r210 验证增量

clientruntime/installeddevice包回归通过；prepared/desktop产品定向race通过。真实固定摘要Go子进程覆盖两scope×双角色：prepare→模拟决定→批准绑定→同一PID激活→certificate proof/descriptor/private handoff→正常Stop/实际exit/reader join；拒绝、缺Allow、错scope/请求/Spec、未知描述符不能发布或重试。对话取消的迟退出与描述符操作阻塞均测试了超时保留、原child/原操作完成后才join，无强杀。

Mac Swift增量nice10/jobs2编译现有GUI及协议模块，4项OwnedControllerPreparation检查通过；只创建未联网状态/handoff，不点击GUI、不读Keychain、不采集/注入。Go子进程的同意回复与issuer/API均为fixture，不能当真实人工许可或画面键鼠验收。Windows交叉vet通过，材料 `artifacts/connection-poc/r210-desktop-preparation/`，没有SSH或新双机包。

## r211：独立 Mac 查看器与私有连接意图

新增可执行目标 `OwnedDesktopController`，只从固定摘要 owner 的继承 stdin 收取预备、连接意图、同意和 bootstrap；不接受 argv 的 host/模式/配对密钥，不读旧 Keychain，不提供首次信任或剪贴板 UI。连接界面由本机用户输入设备码和本次验证码，再按连接按钮；Go `PreparedDesktop.ConnectIntent` 只在 controller 同一 child 上单次等待120秒，返回私有、不可 JSON 序列化且诊断脱敏的 intent。`Claim()` 只生成待 OS signer 发送的精确 claim，不授予网络权限；真实 claim/approve/status 关联、签名 grant 和当前 ACTIVE/watch/角色/有效期门禁仍须通过。

同一 App 另显示经 owner 关联的请求与精确范围，由本机确认或拒绝；bootstrap 只能在明确允许后一次消费。查看器接现有只读 TLS/JPEG 或控制 TLS 输入客户端，最多保留一张待显示画面，避免持续帧在 UI 队列积压；显示帧数与输入提交数只作遥测，后者不是 Windows 注入成功证明。`desktop_state` 仅作私有短状态回复，尚未接 native owner 的状态轮询。

原 `RemoteInputCanvas` 拆入共享 RemoteCanvas，旧 App 保留原默认行为。新查看器显式禁用剪贴板：Command-C/V 在读取本机 pasteboard 前被消费，不下发剪贴板操作；Go scope 和 Windows 服务端仍独立禁止剪贴板。窗口固定为一个，通过实际内容窗口挂接关闭委托；关闭窗口、退出菜单或停止按钮先释放本机按住输入、锁闭后续操作并记录停止意图，保留进程等 outer owner 观察后 quiesce/revoke/join。父 stop/EOF 才取消客户端，并等待网络 completion 和管道 reader 结束后发 closed、自然 exit；未知不强杀或重开。该子进程的 exit 不能替代 watch/node/signer/outer owner join。

验证：Swift 41项定向检查（预备、handoff、输入捕获、输入连接驱动）通过；窗口挂接修改后的6项预备复验及实际 App 两范围预备/正常收尾通过。真实 App SHA-256 `3ab3133182c090da8ad3ab55085ffd6bb5b8f3de18d0cfd2ee4a414f8a3961a5`，view PID48728/control PID48731，均在未激活时无网络端点，closed/EOF/实际exit0。脚本 `validation/owned_preparation.py` 没有点击确认、交付 bootstrap 或模拟键鼠；未知 join 会保留原 driver/进程，无 terminate/kill。Go prepared/intent定向race通过24.851秒，Windows vet为交叉静态检查。材料 `artifacts/connection-poc/r211-owned-controller/`。

本轮没有 SSH、真实授权、屏幕或输入操作，不计 GUI 双机验收。后续必须把新 App 接入 native Prepared 产品与异步本机等待，接 Windows 明确确认界面；短管理交换不能等待120秒人工决定，关闭/拒绝/启动失败须先 join 同一 prepared child，再释放 node/signer。真实服务与 fresh builder 接线仍遵守 r207 唯一规则 writer/verifier 所有权约束，原服务时效及3秒 TCP/密钥交换和8秒 TLS启动截止不变。

## r212：独立 Windows 确认界面与本机停止

新增 `src/OwnedDesktopAgent` WPF 程序（已加入解决方案），没有旧配对/首次信任/剪贴板、地址或连续重连入口，不接受 argv 模式。`OwnedAgentPreparation` 只解析继承管道中精确 Go 规范的 target prepare、单次 show_invite、同申请/controller 的 consent 和一次 activate；拒绝重复/未知字段、错角色、self-approval、非规范编码、提前/拒绝后/取消后/重复激活。此模型不验证 authority，本机决定也不代替真实 approve/status；签名、scope 和授权时效仍由原 owner/gate 执行。

Windows 显示本次设备码/验证码及当前请求，控制范围必须另勾选本次键鼠许可且物理紧急停止 hook 可用，才能点击允许。hook 复用 `PhysicalEmergencyChord`，不采纳注入的按键状态。父帧没有 Allow/本机控制许可字段可代选。明确允许后的同一 child 才生成一次临时证书/HMAC、固定 loopback47478 单会话监听并回 ready，同一证书响应一次限定 grant/target/challenge 的 possession proof；不从旧证书 Store 加载。`PrimaryScreenCapture` 与 `SessionNativeInputSink` 直接复用，source/sink 仍在真实 TLS/HMAC 认证后才构造，无剪贴板回调。

窗口关闭、停止或物理紧急停止立即取消共享/输入，保留进程供 owner 观察 stop_requested 并撤销/join。父 stop/EOF 后须等同一 server task、输入释放路径和 stdin reader 实际结束、hook 成功解除，才 closed/自然退出；hook join 未知时保留原进程。短状态包含帧/成功 native 调用计数、释放/held、停止/结束/失败，不能当画面可见或真实输入效果的独立验收证明。未知/重复命令只发一次 failed；双端均修复已打开确认框时错误帧触发拒绝再入的递归收尾，父故障不能制造一条迟到的本机决定。

构建及检查：Windows BelowNormal/双核、dotnet单并发/无共享构建服务，独立 WPF 与协议组件均0warning/0error；4项新协议检查通过。Release self-contained/PublishSingleFile/IncludeNativeLibrariesForSelfExtract 单文件 SHA-256 `0815407ec02be3b8131aa03023f4ffb0d6c7e7b56efe4d8f3c03de606a97e325`，避免只固定多文件 apphost 而遗漏托管依赖。隐藏 Interactive 管理驱动验证 normal_view PID40476、normal_control PID36200、窗口关闭 PID40564 均 closed/EOF/实际exit0；待确认错误帧 PID35280 单次 failed、closed/EOF/实际exit1。窗口关闭后进程仍在等原 owner，预备均无网络端点、无画面/输入。驱动没有点击允许或 activate，不是远控双机通过。

Mac 最终6项预备检查通过；实际 App 待确认错误帧组件 PID49263 单次 failed 后正常 stop/EOF/exit0，未联网/无 bootstrap。注意此组件的最终 exit0 只证明 child 收尾，原 owner 仍必须保留失败结果，不能改报整个会话成功。Windows 初轮驱动因默认 CRLF 被严格协议拒绝，改为 LF；另修正新任务脚本路径。旧尝试均已退出且证据保留，未重发同意/激活或复用未知工单。最终精确检查为零组件进程/任务、测试程序根与上传归档均已删除，历史现场未动。材料 `artifacts/connection-poc/r212-owned-agent/`，验证脚本 `windows/RemoteAgent/validation/owned-preparation.ps1` 未知 join 会保留同一 driver，不强杀或替换。

下一组应直接实现 native owner 可取消的异步准备/连接意图/本机决定与短状态轮询，把双方 `Product.Prepared` 接上：Mac ConnectIntent →真实 claim，Windows show_invite/incoming →本机决定→明确 Allow/Deny，approve/status 成功关联才 ObserveApproved。20秒管理交换不承载120秒人等待；拒绝、过期、窗口关闭与任何 provision 前失败也须先 join GUI，再 node/signer；已 provision 后保留独立撤销。随后接新 fresh builder/唯一 writer-verifier 服务组合和真实画面键鼠双机，不复用已退休 fixed TEST 工单、不放宽原截止。
