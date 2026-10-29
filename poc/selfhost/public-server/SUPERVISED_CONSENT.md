# r66：当前自主产品的私有授权协调组件

`supervised_consent.consent_round(target, controller, profile_expires_at=...)`仅用于明确授权的TEST双机验证。两端都是接收installeddevice.Request的受信私有callable。它不导入旧split-consent控制面脚本，不重置Store/用户/节点，不管理服务，也不打开公网端口。自动发送allow=True是本地测试步骤，不能作为生产用户同意UI。

流程为两端注册、邀请/claim、核对准确incoming对端、自行批准必须拒绝、owner拒绝必须不给grant；随后重新邀请并由owner批准，两端返回同一grant，检查subjects/expiry，最后owner明确撤销并由controller status确认终态无grant。返回值仅包含三个公共布尔证据，邀请/code/grant不输出日志或进入命令行。Python只比较结构与一致性，不负责验证issuer签名，真实产品在provision前仍须独立验签。

已知request_id后，每条请求均在finally中收尾。批准响应丢失时先向owner查询原请求status：approved则按该确切grant ID撤销；pending则显式否决；denied/revoked则检查controller终态。不会重发allow=True。状态或撤销结果未知必须报错，不能在finally中打印“成功”，也不能自动销毁仍可能需要签名撤销的OS身份。注册成功的设备记录没有删除API，本轮若实际执行会保留在Store；不能回写旧Store快照来清理。

`supervised_requests.WindowsRelay`把一个私有JSON请求经SSH stdin交给r61同用户relay，pipe与工具路径来自受信公共桌面bootstrap。命令行只含公共路径/pipe；拒绝shell字符、非规范pipe和SSH选项伪装。35秒管理调用期限，stdout按8192字节单JSON/重复字段/UTF8限制解析，异常只报固定标签，不输出stderr或回执。SSH调用超时只影响管理调用，**不等于桌面进程退出或授权失败已确认**，必须通过后续状态查询处理。`LocalSupervisor`复用JoinedPipe的串行、有界私有协议，进程join仍由调用者承担。这些适配器不提供publisher认证，也不能信任任意网络来历的工具路径。

## 刻意保留的边界

Controller成功provision后仍封闭普通业务转发，consent_round继续仅做consent-only，不提供work/provision回调；不能把旧use_grant/network_state接口套进当前产品。r67新增run_supervised_consent.py及Windows -StayReady入口，已实际完成双OS signer的consent-only episode与双方join/TEST清理；尚未完成双机provision协调或PONG。

## r68：provision 后独立撤销

当前 TEST supervisor 在发布 READY 前配置独立 HTTPS client，复用其持有的原生 signer；只有从产品成功 approve/status 回执观察到的 request_id 和 issuer 签名 grant 才能被绑定。第一次 provision 调用发给产品前即固定该 grant，包括启动回执超时的情况。此后仅接受无附加字段的 `{"Op":"revoke_session"}`，不接受调用者提供 grant、request、身份、root 或目的地址。controller 角色不能代替 target 撤销。普通业务路径保持封闭，产品/watch 故障不会抢先销毁独立撤销需要的 signer；实际 Release/join 后该通路关闭。

首次撤销的回执未知时不自动重发。下一次显式调用必须先用原生签名查询同一 request：确认 revoked 且无 grant 才返回成功；只有仍为 approved 且 grant 完全相同，才允许本次显式重试。其他状态或查询失败均保留未知结果。操作与 Close 串行，Close 等待正在执行的请求，不能提前关闭 signer。profile 过期、时钟早于签发、取消、其他设备/授权及已关闭通路均拒绝；不放宽生产信任或短包有效期。成功回执只证明撤销，不证明 watch/payload/node 已 join 或当前有 PONG。

构建器随新包编译当前 relay，显式 TEST tunnel 模式也同步编译到 supervisor；不能再复制 r61 relay（不认识新操作）或混用旧短包。现有 consent-only runner 要求 fresh 包自带 relay。实际批准回执在观察 grant 前丢失、后续 profile 到期仍无法确认撤销等情况没有通用自动恢复承诺，必须保留现场和身份，不能当作已清理。

验证：Mac 相关 Go race 回归、Windows vet 和 47 项 Python 回归通过。真实本地 TLS/authority 测试在普通产品 HTTP client 已关闭后，故意丢弃已执行撤销的回执，独立 client 经签名 status 确认终态且实际 mutation 仅一次；该测试使用内存设备密钥，不是 OS signer 双机验收。Windows 本轮 4 项撤销状态/生命周期测试与 3 项真实 IPC/relay 测试全部通过，无 SKIP/exit0；后者 backend 为 fixture，不调用 Credential Manager。真实双机 provision、独立撤销和 PONG/维护收尾仍待后续协调。

## r69：活动协调与接受标准

`supervised_session.active_round` 是独立的 TEST 管理组件，consent-only runner 没有自动切换到它。它用当前私有协议完成新邀请/明确批准、双方 grant_clock，然后并行发起两端 provision 并等待两个管理调用都完成。即使一端失败也不能丢弃另一条仍可能注册了节点的调用。普通 child 业务在 provision 后保持封闭，finally 只通过 target 的 selector-free revoke_session 撤销一次；未知回执报 preserve_signer，不重试 mutation、不 stop 或删除 owner。尚未尝试 provision 的失败仍用原 request 精确查询/撤销。

两端 network_watch/ACTIVE 不算通过。必须读取实际 product_count，先观察双方 PONG > 0，再至少三次双方都增加，且持续至少六秒（超过三秒 state 租约）；计数倒退、布尔冒充数字、ended、pin_rejected、未知字段/秘密字段、恒定的单个 PONG 都拒绝。开始时允许有限期 product_unavailable，开始增长后失去 payload 立即失败。返回仅公开布尔/样本数，不含 grant/邀请码，且不声称 owner/node 已 join。

新增七项协调故障回归，与原测试联合 54 项 PASS；全部为协议/并发 fixture，不是实际双机 PONG。当前尚需补齐首次绑定的 readiness 竞争：早完成一端 bind 后立刻 state，服务端在另一端尚未 bound 时返回拒绝，现有 watch 会终止；不能通过忽略 HTTPS/验签失败或放宽 binding 门禁处理。还需要持久管理父进程在撤销未知时保留 signer，以及明确区分正常维护零退出与撤销触发的 fault 非零退出，再接真实 fresh 双机 runner。未在这些边界解决前用活动组件启动实机 enrollment。

## r70：readiness 与独立驻留管理

新增内部网络操作 await_binding，沿用原生设备签名、持久 nonce/限流与原 grant 授权检查；仅当完整双边 binding 尚缺失时在服务端等待，最长10秒，每轮重新检查授权/健康状态，且不持锁睡眠。取消、撤销、过期、倒退、非参与者、损坏/未知 grant、driver/storage 故障均拒绝。只返回原格式、原3秒有效期的完整 issuer 签名 NetworkState，没有部分 binding 或 unsigned pending 能力。普通 state 路径行为不变。native watch 仅首次调用 await_binding；拿到完整 pair 后继续普通 state，HTTPS/签名/身份等失败不重试。此服务协议尚未部署，必须先升级预发布服务再启动新的实际产品包，不能拿旧服务403当作可忽略的 pending。

新增 Unix/macOS TEST 管理驻留组件 persistent_supervisor：只在受信0700目录监听0600 socket，公開 bootstrap 只含PID/原生scope/public，启动前拒绝旧socket/metadata。管理客户端断开或业务调用未知不会自动 stop；只有显式 owner_stop 尝试实际 closed/EOF/exit0/join，失败不宣称 joined、不删测试根。它必须作为独立管理进程启动，当前尚未接入 fresh 活动 runner；不是原生 OS 身份/发行认证的替代品。Windows holder 增加显式 -StayReady -RetainUntilStop，仅该 TEST 模式取消原10分钟管理超时，不延长 profile 或许可；只接受空、非目录/非重解析点 stop marker。

修复 LocalSupervisor 超时后的迟到回执混淆：下一条业务请求发出前先消费旧回执，绝不能把旧批准当新撤销结果。写入未知/畸形回复永久封闭该继承管道，并保留 owner；驻留组件同样不把遗留业务回复当作 stop ACK。此故障分支仍需要独立本地控制端点或专用恢复协议，不能通过无限重试解除封闭。正常 Windows 业务管道每请求独立连接，不复用这类继承流。

验证：真实本地 TLS/Store 的首端等待、后端实际 Bind 不被锁住、完整验签回复、等待中撤销、取消/权限/重放/时钟/健康拒绝通过（设备与 driver 为 fixture）；相关 Go race/vet通过。Python 60项回归含真实本地子进程跨客户端存活、断开管理连接后显式零退出、私有路径/旧metadata拒绝、未知回执不乱序和非零退出不冒充成功。Windows 新 holder AST 及5项 native watch协议测试无SKIP/exit0，未执行原生身份安装或新Headscale会话。真实预发布服务与控制面本轮未改。

## r71：macOS 原生独立业务 socket 与实际驻留验收

最新 TEST supervisor 在 Unix/macOS 创建全新0700目录中的0600原生业务 socket，并经继承 READY 提供 control_socket。与 Windows 同样每连接一条有界请求，拒绝stop/maintenance/身份覆盖，不占用生命周期stdin。关闭先 cancel/等待全部连接handler退出，再按原inode精确移除自己的socket/空目录；替换对象拒绝，不递归删除。Windows管道行为保持。socket仅提供同用户隔离，不是publisher身份或生产发现认证。

persistent_supervisor 的正常入口现在必须使用这个独立 NativeUnix adapter，不再把业务写入继承管道。每次请求用新连接，并核对原canonical目录/socket的owner、模式与inode；失去一个业务回复不会把另一个回复当stop或revoke结果。驻留bootstrap增加公开supervisor PID/control_socket，并通过先flush私有临时文件、再原子无覆盖发布，避免读到半帧。继承流只用于可信open/stop与实际join；旧 LocalSupervisor仍仅供明确的consent-only/fixture使用。

本轮实际 fresh macOS TEST安装及原生Keychain signer、真实supervisor/product已验证：独立业务计数、非法stop/foreign grant选择拒绝、管理客户端断开后原owner仍存活，显式owner_stop后closed/EOF/实际exit0，业务handler/socket目录清理，随后独立idle retirement及空管理目录精确回收。没有注册在线设备或启动Headscale节点/TLS payload，不能将此结果当作双机PONG。预发布 await_binding authority已兼容升级；原Store、verifier及业务Nginx基线保持，细节见BINARY_UPGRADE.md。相关Go race/Windows vet与71项Python回归通过，真实活动runner接入/活动收尾仍待完成。

本轮新增9项Python测试（含多组故障子项），覆盖回执丢失只批准一次、精确撤销、未知撤销不假成功、不同对端/grant/过期结果、严格布尔、短有效期预拒绝、秘密只走stdin、超时无自动重试、重复/超大/多帧回复和shell注入拒绝。加既有JoinedPipe/compiled products回归共41项PASS。另Go当前HTTP/签名/注册授权重放及profile TLS回归通过。全部是本机协议/fixture回归，没有创建新的真实设备、grant、节点或桌面任务。
