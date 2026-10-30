# r79：显式故障收尾协议组件

正常 `closed`、EOF、实际 exit0 的接受条件保持。故障收尾是另一个协议，不能把 exit1、PID消失、普通 `failed` 或管理进程中断当作全部工作已 join。

`grantrevocation.Channel.ConfirmRevoked` 用保留的 OS signer，只查询已观察并固定的同一 request。两位参与者都可查询；没有批准、撤销或外来选择字段。profile/grant/身份/时间及取消门禁保持；只有严格的 revoked、同一 request、无 grant 和无混合字段才通过。结果未知保留 signer。

`processowner.Child.JoinedFault` 仅接受受信产品的专用 `fault_joined` 终态、实际 exit1、stdout EOF、reader/stop 完成；普通 `failed` 一律拒绝。专用终态被在途计数请求读到时保留，封闭后续业务，供同一子进程收尾使用；额外/重复终态、exit0、信号退出或读取错误不能变成故障证明。

`Controller.ReleaseFault` 先作上述只读撤销确认，再停止并等待原来同一个 product；专用终态成立后才关闭独立 client 和实际 join supervisor signer。未知或失败不重建身份、不启动另一个 child。Signer join失败永久拒绝后续故障证明。child stop超时后只允许显式等待原child，不能恢复业务。

TEST supervisor 的继承生命周期流接受无额外字段的 `stop_fault`，业务 socket/pipe 仍拒绝。两个终态阶段解决 stdin EOF 与原生命令 reader 的相互等待：

1. 产品专用清理证明、实际子进程 join、独立 signer join 完成后，输出严格 `fault_stop_ready/joined=true/fault=true`。拒绝则输出 `fault_stop_rejected` 并保留生命周期输入。
2. 父进程此时才关闭 stdin；原生控制 handler、命令 reader、maintenance endpoint 实际结束后，输出严格 `fault_closed/joined=true/fault=true`，再实际 exit1。任何 defer 清理错误都会阻止最终证明。

Python `JoinedPipe.stop_fault` 不在授权未知时关闭stdin；需要两阶段严格布尔、无额外字段、EOF及真实exit1。迟到最终回执只能继续读取同一调用，不重发请求，不借用正常 stop。`Owner.owner_stop_fault` 完成后返回 `owner_fault_closed`，清理本对象的原 metadata/socket；管理主进程也以故障分支 exit1。正常 owner_stop 不能借用该证明。

## r80：native product 的清理证据

当前源代码的 native product 已接入 `cleanupwitness`。它只通过可信 in-process context 传递，记录固定资源的开始/实际完成和永久失败，不接受 JSON/环境/root/身份选择字段，不保存错误文本或秘密。只有 TEST runner 写出初始ready后才可arm；根入口的全部defer结束后才seal。mandatory owner/binding日志、命令reader、runner和内部signer必须存在且实际结束；可选node/watch/session/TLS子进程/监听器一经开始也必须完成。重复生命周期、失败、未完成、迟到或非TEST全部拒绝，未启动资源不冒充已启动。

业务故障与清理错误分别处理：session工作出错不等于node关闭失败；内部signer只有CloseJoined无错误才完成，TLS子进程只有closed/EOF/实际exit0才完成。node包含proof/SDK/原生enrollment lease关闭；owner日志的关闭结果与runner业务错误分离，binding关闭也单独检查。监听器关闭返回最初结果且不可晚到变成功，controller监听取消worker和overlay listener的取消worker都加入实际join。原生根入口只有在这些证据全部成立时才用专用 `fault_joined` 代替普通failed并exit1；正常closed/exit0不改。清理未知、abort、强制/非零TLS子进程等仍仅普通failed，不能让supervisor生成新故障证明。旧包没有这些hook，仍不接受普通failed替代。

这些证据证明本地受信产品的收尾，不是授权、远程证明、活动PONG或文件退休许可。组件的真实fixture子进程、真实本地HTTPS/内存设备签名和Windows内存协议测试不是原生身份或双机 fault acceptance。

## r81：Windows retained holder 与独立故障工单

Windows holder 支持空普通文件 `stop-owner-fault.request`，仅新 retained 模式和随包的 `supervised-fault-stop.ps1` 可用。它保留同一个 ReadLineAsync，未就绪/未知不抛出到普通 finally、不关闭stdin、不发普通stop。严格拒绝、畸形、混合marker均驻留；pending的迟到回执只继续读原调用。只有明确rejected时，新的空 `retry-owner-fault.request` 才允许一次显式重试；不自动重查/重发。原生Go中这只重新确认同一授权并加入同一个child，不重新注册/批准。

PowerShell要求Go canonical map的精确两阶段JSON，拒绝整数bool、重复/额外字段；EOF及实际native exit1后原子发布 `owner-fault-joined.json`。父holder随后以预期故障抛出，wrapper/task必须exit1，不能写EXITCODE0。正常stop保持zero路径。Mac holder也持久化绑定原supervisor/holder PID的故障receipt；实际管理进程exit1且原metadata/socket消失才算管理收尾完成。

fresh双机包通过bootstrap的整数 `fault_protocol=1` 声明此能力。显式操作为：

```text
python3 run_supervised_activity.py WORK fault-revoke
python3 run_supervised_activity.py WORK fault-stop
python3 run_supervised_activity.py WORK fault-retire
```

只接受原活动的exercise_outcome_unknown。fault-revoke在任何效应前落盘未知状态，通过原bound通路明确撤销，未知时保留owner/tunnel；不重发批准/provision。fault-stop依次等待Mac/Windows的实际receipt、原PID、真实exit1与metadata清理，分别持久化已确认角色，不重复停止已确认角色。正常zero、truthy字段、旧bootstrap、未知工单均拒绝。fault-retire重新检查双方实际材料后才运行原独立registered退休工具，仍要求原生租约/签名库存/在线issuer清理证明；未知删除不自动重跑。

`fault-state.json` 与原 `activity-state.json` 分离；故障回收成功为fault_joined_and_test_roots_retired，不能改称正常活动/PONG/zero成功。部分删除或回执未知必须查看实际保存状态，不能假设所有owner/文件仍存在。Windows release_delete同步不确定仍使用原受限finish-files显式恢复，没有增加删除绕过。

接下来必须fresh重建当前签名TEST包做实际验收，不混用旧holder、不覆盖/复活已退休安装。当前Windows真实进程/同用户pipe/完整PowerShell脚本测试使用内存fixture，不操作原生凭据或Headscale，尚未替代双机原生fault或PONG验收。
