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

本轮新增9项Python测试（含多组故障子项），覆盖回执丢失只批准一次、精确撤销、未知撤销不假成功、不同对端/grant/过期结果、严格布尔、短有效期预拒绝、秘密只走stdin、超时无自动重试、重复/超大/多帧回复和shell注入拒绝。加既有JoinedPipe/compiled products回归共41项PASS。另Go当前HTTP/签名/注册授权重放及profile TLS回归通过。全部是本机协议/fixture回归，没有创建新的真实设备、grant、节点或桌面任务。
