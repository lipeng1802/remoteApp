# r79：显式故障收尾协议组件

正常 `closed`、EOF、实际 exit0 的接受条件保持。故障收尾是另一个协议，不能把 exit1、PID消失、普通 `failed` 或管理进程中断当作全部工作已 join。

`grantrevocation.Channel.ConfirmRevoked` 用保留的 OS signer，只查询已观察并固定的同一 request。两位参与者都可查询；没有批准、撤销或外来选择字段。profile/grant/身份/时间及取消门禁保持；只有严格的 revoked、同一 request、无 grant 和无混合字段才通过。结果未知保留 signer。

`processowner.Child.JoinedFault` 仅接受受信产品的专用 `fault_joined` 终态、实际 exit1、stdout EOF、reader/stop 完成；普通 `failed` 一律拒绝。专用终态被在途计数请求读到时保留，封闭后续业务，供同一子进程收尾使用；额外/重复终态、exit0、信号退出或读取错误不能变成故障证明。

`Controller.ReleaseFault` 先作上述只读撤销确认，再停止并等待原来同一个 product；专用终态成立后才关闭独立 client 和实际 join supervisor signer。未知或失败不重建身份、不启动另一个 child。Signer join失败永久拒绝后续故障证明。child stop超时后只允许显式等待原child，不能恢复业务。

TEST supervisor 的继承生命周期流接受无额外字段的 `stop_fault`，业务 socket/pipe 仍拒绝。两个终态阶段解决 stdin EOF 与原生命令 reader 的相互等待：

1. 产品专用清理证明、实际子进程 join、独立 signer join 完成后，输出严格 `fault_stop_ready/joined=true/fault=true`。拒绝则输出 `fault_stop_rejected` 并保留生命周期输入。
2. 父进程此时才关闭 stdin；原生控制 handler、命令 reader、maintenance endpoint 实际结束后，输出严格 `fault_closed/joined=true/fault=true`，再实际 exit1。任何 defer 清理错误都会阻止最终证明。

Python `JoinedPipe.stop_fault` 不在授权未知时关闭stdin；需要两阶段严格布尔、无额外字段、EOF及真实exit1。迟到最终回执只能继续读取同一调用，不重发请求，不借用正常 stop。`Owner.owner_stop_fault` 完成后返回 `owner_fault_closed`，清理本对象的原 metadata/socket；管理主进程也以故障分支 exit1。正常 owner_stop 不能借用该证明。

**当前实际 native product 尚未实现嵌套清理 witness 和 `fault_joined` 生产者，故真实故障仍拒绝新证明。** 保持这一关闭状态，不得临时把普通failed当作替代。组件的真实fixture子进程、真实本地HTTPS/内存设备签名和Windows内存协议测试不是原生身份或双机 fault acceptance。

下一组先实现 native product 的独立清理 witness：区分业务故障与 product/watch/node/signer/原生日志清理失败，只有全部实际完成且未强制中断才可发专用终态；随后接 Windows retained holder、runner fault阶段和故障工单回收。Windows holder遇到未知回复必须保持输入/owner驻留，不能落入原正常 finally stop，也不能自动重试。完成前不要启动混用旧holder的新双机故障验收包。
