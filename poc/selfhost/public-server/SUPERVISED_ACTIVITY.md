# r75：驻留双机 TEST 活动管理

`run_supervised_activity.py` 只供已授权的本地运维协调，不是产品的公开连接入口。服务器和 Windows SSH 地址、随机 TEST 根、管理通路均固定；不会开放公网端口、修改业务 Nginx 或重置控制面。Windows 原生身份仍在登录桌面的 Interactive 任务内使用。

在本目录运行 `python3 run_supervised_activity.py prepare`，输出本轮私有工作目录。它核对先前受信构建输入的 helper/TLS 哈希、读取现有服务公开 profile，并用当前构建器生成两个新签名 TEST 包。旧输入只提供不可变程序字节；包、root、身份、journal、node 和授权均重新创建。过期包不启动。

后续每个步骤都显式指定同一个工作目录：

```text
python3 run_supervised_activity.py WORK start
python3 run_supervised_activity.py WORK exercise
python3 run_supervised_activity.py WORK stop
python3 run_supervised_activity.py WORK retire
```

`prepare` 先只读核对 Windows 固定 r67 **包目录**中的不可变输入，再生成短包。它创建本轮独占的 TEST Go 包装器，仅在 `build` 时加入 `-s -w`；保留原来所有嵌入 anchor/root/digest 的 `-X` 参数，最终文件仍由原构建器签名和纳入哈希库存，原生 helper 字节不改动。生产构建器不使用此包装器。

`start` 在外部效果前持久化未知状态，上传 preset3 固实 tar.xz 包并核对整个归档哈希。仅复用上述固定**包目录**中与新包完全同哈希的三类 helper/TLS exe；复制前后再次核对，不执行旧文件、不读取旧原生状态。输入核对失败立即保留工单，不再把未知读取结果静默降级成大包上传。Windows bsdtar/liblzma 支持已实查；传输结束再次检查剩余有效期，不能通过增加传输超时延长 profile。Windows holder 为 `-StayReady -RetainUntilStop`，Mac holder 有独立实际 wait 监控；公开 bootstrap 不包含秘密。两条回环 SSH 通路只为本次 TEST HTTPS 协调服务。

`exercise` 采用 r74 阶段门禁及 r73 正常 quiesce 模式，要求真实双方 ACTIVE、PONG 连续增长至少三次且持续至少六秒，再实际正常 join 两个产品，保留独立 signer 明确撤销。未知回执不重发批准、不自动停止 owner、不关闭 tunnel。`stop` 只有在正常撤销已确认后才按 controller、target 顺序等待真实 owner closed/EOF/zero exit；Mac 独立监控实际进程退出，Windows 同时核对本轮 wrapper 退出材料与任务结果。SSH 成功、PID 消失或 marker 创建均不算 join。

每端独占的 `trace-target`/`trace-controller` 目录仅逐步写入固定白名单操作、action、阶段及外层状态，调用前先写calling，异常写outcome_unknown；不序列化请求、回执或异常文本，也不记录邀请、grant、身份或计数。已有目录拒绝覆盖。它是诊断材料，不能作为签名授权、PONG增长、撤销或实际join证据，缺失/未知不能授权重试mutation。

`retire` 仅在两端 owner 已 join 后进入一次性未知阶段，分别运行独立 `retire-node-test`。它们仍须原生独占租约、完整签名库存和当前 issuer 在线退休证明。只有工具明确返回 registered_test_retired 且精确 TEST 根不存在，才移除本轮非运行任务、关闭本轮管理通路、rmdir 本轮空目录。任何未知删除结果都不能自动重跑，需原生工具受限 recovery/finish-files 单独查证。管理归档和公开工单保留。

Windows 首次文件 sync 失败时，只有确认该任务失败且无本轮原生进程/READY、Mac 尚未安装、短包仍有效，才可显式执行 `WORK resume-windows`。仅允许一次，委托原 `-ResumeFiles` 的 Absent trust 门禁；不能覆盖未知 root 或重建未知身份。

这些步骤没有隐式 finally teardown。源代码回归、包压缩、SSH 上传或 HTTPS 探针都不代表双机活动通过。以工作目录 activity-state.json 及实际原生日志为准；只有 normal_activity_joined_and_test_roots_retired 才是本工具完整正常分支完成，GUI、签名发行及活撤销故障分支仍独立验收。


## r82 TEST 启动阶段诊断

fresh TEST supervisor 可通过同用户私有管理通路发送精确的 `{"Op":"product_stage"}`，读取原 product 的缓存固定阶段。该操作不进入 product 请求队列，不注册、不重发 provision、不恢复故障业务、不关闭 signer；含 selector/其它字段的请求仍拒绝。native 编译 TEST 门禁只向专用 stderr 写固定枚举，父进程以有界行缓冲丢弃所有未知/过长/未结束行和原始错误，普通产品默认不采集；stderr reader 必须实际 EOF/join，才能证明正常或故障收尾。

阶段包括 target_tls/listen/secret/serve 与 controller_listen/dial/secret/tls/accept/bridge（完整命名见 startuptrace）。它表示进入哪个操作，不能证明该操作成功，更不是 payload/PONG 证明。exercise 失败后 runner 只读查询两端并独占保存 startup-diagnostic.json，仅持久化白名单标签或 unavailable/unreported；诊断失败保留原异常和原未知工单，不代替撤销/join/退休。已有证据不可覆盖。


r83 将固定阶段细化到目标 ACTIVE/本机节点/IP/实际监听成功，以及控制端 ACTIVE/LocalAPI HTTP upgrade/禁止 Dial-Self/实际 overlay socket 与 WhoIs 门禁。只输出枚举，丢弃 HTTP response body、原始错误与地址。overlay_route_missing 仍严格拒绝系统网络回退；诊断不添加拨号重试、不延长期限、不使网络失败变成成功。target_listening 只证明监听创建与当时门禁成功，不能证明对端已经接入或存在 TLS payload。

r84 的 HTTP 拒绝分类仅在编译 TEST context 启用时，读取最多 513 字节本地 LocalAPI 非101响应；超过512或读取失败保持泛化拒绝。只识别 pinned SDK 的固定 bad-upgrade、permission、resolve/dial 前缀及拒绝/超时类别，白名单枚举之外的正文从不持久化。Dial-Self在读取正文前直接拒绝；生产 context 不读取诊断正文。分类不改变原返回错误、重试策略、连接或 ACTIVE 期限，也不提供网络授权。


Mac setup 启动前独占并同步 mac-setup-attempt.json，实际退出后只将严格固定 status/stage/退出码写入 mac-setup-result.json，不持久化原 stdout/stderr/异常。未知回执、已存在安装根或结果均禁止自动重放。初始化 pending 不等于未创建身份；不能仅凭没有驻留进程清理未知凭据或回写旧记录。
