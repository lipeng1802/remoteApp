# 4C-4A：真实后端装配与生命周期组件

本轮仅完成代码/自动测试。**未更新线上二进制、未启用 /v1/network、未创建长期 verifier、未修改 Headscale ACL。** 线上保持4C-3D的device-only拆分服务；整体切片4尚未完成。

## 装配

StartHeadscaleNetworkSupervisor 接受可信单Store和已运行的 verifier.Runtime，固定使用 server-local Headscale CLI。driver的VerifierIP来自runtime，Proof从Store.DevicePublicKey取成功签名注册的绑定公钥，再调用runtime.Verify。客户端不能提供就绪标志、公钥替代、控制命令或地址。真实overlay.Verify复用nodeproof.Verify，包含新鲜挑战、独立overlay peer身份及应用签名；不把健康行/节点记录当持有证明。

Runtime记录其独立Node，验证期间关联runtime取消、12秒操作期限，停止后不再接收proof。停止时先取消/Live=false，等待proof退出，之后关闭overlay、释放owner.lock；阻塞的非合作worker不能假装join。默认系统Tailscale不参与。

Supervisor要求**专用且尚未被其他worker使用的backend/driver**。先同步完整Reconcile，随后维护worker启动，首个维护周期成功前不Ready。使用Supervisor.Authority生成broker路径，不直接构造NetworkReady=true。后台出现任意维护失败、verifier退出/不健康、Store关闭或driver proof/policy健康故障后锁闭；不可自动恢复，需检查状态后显式创建新的backend/supervisor实例。不能共享同一driver或在运行中替换driver。

driver外包装把生命周期取消传至Mint/Observe/Verify/Cleanup/ReplaceRules，并在操作结束锁存故障。Healthy仅访问原子状态，不争用底层CLI操作mutex，避免长Verify阻塞verifier故障检测。故障路径关闭接收、取消执行中的请求/维护，join后Fence单Store；随后有界尝试显式零ACL。执行结束重新检查就绪/取消，取消操作获得的credential不回包。Stop超时返回unavailable，调用者不能提前关闭Store/driver；正确顺序是supervisor.Stop成功，再runtime.Stop成功，再Store.Close。

监控100ms是轮询配置，**不是严格故障时延保证**；底层/OS调度与不合作driver可能延长join。Stop成功仅证明join；DenyAllConfirmed仅证明最终driver ReplaceRules(nil)返回成功，不能等同真实网络拒绝。provider不可用时旧ACL可能留存，API仍锁闭；既发signed state有原有绝对lease，本组件不宣称撤回签名或立即关闭所有产品TLS。

## 测试

Mac race测试通过：周期policy失败后锁闭且provider恢复不自动开门；执行中Mint遇verifier fault被取消、即使driver在取消后返回secret也不回包；阻塞driver的Stop超时不能假join，释放后join与零投影；撤策略失败不报告DenyAllConfirmed。Runtime真实生命周期测试使用受控Node替身：proof取消后尚未退出时保留owner.lock且overlay未关闭，拒绝新proof；退出后关闭overlay/释放锁，取消后true不能算proof成功。

上述是生命周期组件测试，非本轮真实Headscale/跨机/产品TLS验收。真实nodeproof/driver既有证据保留，不将它们当本轮supervisor装配实机证据。

## 下一项

在隔离服务端实验Store/临时verifier中接通该装配（不要改线上device-only Store/ACL）。经kernel UID broker完成签名device/network全链路，使用真实Headscale/nodeproof，验收故障/撤销/到期/维护失败、旧ACL遗留时TLS关闭，以及真实零ACL双向拒绝与重启。通过后再设计daemon长期拥有verifier/backend与权限配置；上线network、公网端口、客户端信任锚/floor、产品HMAC投递及GUI安装仍待。
