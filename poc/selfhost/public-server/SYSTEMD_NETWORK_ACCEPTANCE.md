# r21 隔离候选systemd网络服务验收

入口 `daemon-network.py --systemd`，使用新Linux编译的checker/daemon/test二进制和仓库候选unit，不调用Windows。Windows在本轮处于关机状态，无需启动。

## 范围与安全边界

- 仅空的已有PoC Headscale控制面短暂database模式；实验完精确回收并恢复原file模式/策略。不是新建第二个公开控制面，也不是正式database迁移。
- 随机 `/run/remoteapp-daemon-check-*` 实验Store/profile/CA/节点和随机 `/run/systemd/system/remoteapp-isolated-network-<16hex>.service`；没有写/etc正式unit或enable。保留PrivateTmp，因此实验路径不能放/tmp或/var/tmp。
- root supervisor使用本轮随机issuer，不打开正式Store。批准nobody为实际kernel peer，root错UID被拒绝；无设备grant请求由真实后台拒绝403。
- 真实verifier create/stop后，unit ExecStartPre只读检查，daemon只重开既有journal/身份，不由daemon Mint。
- provider故障要求实际PID退出、明确FAIL及非零状态；恢复provider不得自动复活broker。正常停止要求READY记录的PID实际消失、同PID STOPPED回执和锁释放，不把inactive当join。
- systemd 219正常停止会清空ExecMainPID/Code；验收器保留运行时PID，显式恢复只读取最近同PID READY→STOPPED journal记录，仍检查PID退出。journal和provider输出在进程内解析，只显示固定标签，不输出私有材料。
- 本轮无grant、跨机proof、产品TLS或GUI载荷；显式零ACL是控制面回读，不替代双向真实网络拒绝证据。先前双机验收不冒充本次candidate组合。

## 实机中暴露并修复的问题

1. r20预检把Store父目录组与socket共享组混用；实际root/nobody区分后改为Store使用effective GID、socket使用批准共享GID。
2. 原Requires依赖可能吞掉provider故障退出语义，改Wants/After并保留Runtime故障监督及Restart=no。
3. 原CAP_CHOWN不足以访问其他服务UID的私有管理socket；加DAC override，不放宽provider目录/文件权限。
4. 加DAC后overlay仍失败。固定阶段诊断定位verifier Up；只给候选额外NET_RAW后相同身份实际READY并正常STOPPED。结合pinned源码对应root的SO_BINDTODEVICE回退，不授予NET_ADMIN。
5. 实际正常停止后systemd清空PID字段，使旧验收器拒绝正常join；改用运行时PID+同PID回执+真实退出，新增最新回执/错误PID/缺STOPPED/后续READY负向测试。

首次失败轮和诊断轮不计PASS。失败保留资源，显式恢复先确认单个随机unit/实际退出/无锁，再精确Retire本轮verifier工单、删除本轮实验目录和runtime unit，最后恢复原策略；未kill或删锁，没有覆盖正式Store快照。

## 最终结果与待收口

最终协调器actual exit0，三项Linux新约束测试PASS无SKIP；候选三次启动、真实跨UID、显式零ACL、正常join/同key/IP重开、provider故障非零退出、恢复不自动重启、明确新unit重启、实验issuer/Store不变及正常停止通过。精确Retire工单/删除runtime unit和实验目录，nodes/users/preauthkeys零，原file模式/策略恢复，正式服务与业务摘要通过。详见主HANDOFF。

正式后台部署、长期verifier拥有权、维护窗口策略迁移、公网8444准入/客户端直连、凭据供应、自主enroll/bind、GUI/签名安装仍待；整体4未完成。
