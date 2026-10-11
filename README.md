# Personal Remote Desktop MVP

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
