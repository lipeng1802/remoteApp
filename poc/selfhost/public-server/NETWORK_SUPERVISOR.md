# 4C-4A：真实后端装配与生命周期组件

本轮仅完成代码/自动测试。**未更新线上二进制、未启用 /v1/network、未创建长期 verifier、未修改 Headscale ACL。** 线上保持4C-3D的device-only拆分服务；整体切片4尚未完成。

## 装配

StartHeadscaleNetworkSupervisor 接受可信单Store和已运行的 verifier.Runtime，固定使用 server-local Headscale CLI。driver的VerifierIP来自runtime，Proof从Store.DevicePublicKey取成功签名注册的绑定公钥，再调用runtime.Verify。客户端不能提供就绪标志、公钥替代、控制命令或地址。真实overlay.Verify复用nodeproof.Verify，包含新鲜挑战、独立overlay peer身份及应用签名；不把健康行/节点记录当持有证明。

4C-4B修正前置：构造函数新增可信policyDir参数，必须真实绝对非symlink的0700目录，driver显式PolicyDir/PolicyMode=database。4C-4A漏配这些字段会在首Reconcile拒绝启动（不是错误放行），此前没有真实driver装配实机成功证据。服务端必须在独立控制平面确认database模式，不能只靠此字段声明改变线上配置。

Runtime记录其独立Node，验证期间关联runtime取消、12秒操作期限，停止后不再接收proof。停止时先取消/Live=false，等待proof退出，之后关闭overlay、释放owner.lock；阻塞的非合作worker不能假装join。默认系统Tailscale不参与。

Supervisor要求**专用且尚未被其他worker使用的backend/driver**。先同步完整Reconcile，随后维护worker启动，首个维护周期成功前不Ready。使用Supervisor.Authority生成broker路径，不直接构造NetworkReady=true。后台出现任意维护失败、verifier退出/不健康、Store关闭或driver proof/policy健康故障后锁闭；不可自动恢复，需检查状态后显式创建新的backend/supervisor实例。不能共享同一driver或在运行中替换driver。

driver外包装把生命周期取消传至Mint/Observe/Verify/Cleanup/ReplaceRules，并在操作结束锁存故障。Healthy仅访问原子状态，不争用底层CLI操作mutex，避免长Verify阻塞verifier故障检测。故障路径关闭接收、取消执行中的请求/维护，join后Fence单Store；随后有界尝试显式零ACL。执行结束重新检查就绪/取消，取消操作获得的credential不回包。Stop超时返回unavailable，调用者不能提前关闭Store/driver；正确顺序是supervisor.Stop成功，再runtime.Stop成功，再Store.Close。

监控100ms是轮询配置，**不是严格故障时延保证**；底层/OS调度与不合作driver可能延长join。Stop成功仅证明join；DenyAllConfirmed仅证明最终driver ReplaceRules(nil)返回成功，不能等同真实网络拒绝。provider不可用时旧ACL可能留存，API仍锁闭；既发signed state有原有绝对lease，本组件不宣称撤回签名或立即关闭所有产品TLS。

## 测试

Mac race测试通过：周期policy失败后锁闭且provider恢复不自动开门；执行中Mint遇verifier fault被取消、即使driver在取消后返回secret也不回包；阻塞driver的Stop超时不能假join，释放后join与零投影；撤策略失败不报告DenyAllConfirmed。Runtime真实生命周期测试使用受控Node替身：proof取消后尚未退出时保留owner.lock且overlay未关闭，拒绝新proof；退出后关闭overlay/释放锁，取消后true不能算proof成功。

上述是生命周期组件测试，非本轮真实Headscale/跨机/产品TLS验收。真实nodeproof/driver既有证据保留，不将它们当本轮supervisor装配实机证据。

## 下一项

在隔离服务端实验Store/临时verifier中接通该装配（不要改线上device-only Store/ACL）。经kernel UID broker完成签名device/network全链路，使用真实Headscale/nodeproof，验收故障/撤销/到期/维护失败、旧ACL遗留时TLS关闭，以及真实零ACL双向拒绝与重启。通过后再设计daemon长期拥有verifier/backend与权限配置；上线network、公网端口、客户端信任锚/floor、产品HMAC投递及GUI安装仍待。

## 4C-4B：私有验证HTTP入口与Linux组件验收

新增PrivateNetworkHandler，仅供受信TLS/admission包裹的隔离集成，不被线上device-frontend路由。设备与network都通过kernel UID broker，前端无Store；严格JSON/未知别名尾随/超长拒绝、4槽、有界14秒请求、完整IPC包必须<=8191字节、issuer匹配、权威丢失503无回退。安装中device-frontend仍只处理/v1/request，network404。

broker保持请求解析5秒上限；解析后仅固定network操作14秒工作/15秒I/O，设备和未知操作保持4秒工作。客户端期限依然可缩短所有预算。修正旧4秒执行期限无法覆盖12秒节点证明的问题，不扩大单帧/连接/工作槽数。

Mac race新HTTP路径与期限测试通过。实际Linux静态非race测试五项PASS、无SKIP：PrivateNetworkHTTPSViaKernelBroker、NetworkBudgetBoundedWithoutChangingDeviceBudget、SupervisorPolicyFailureLatchesNoRecovery、SupervisorVerifierFaultCancelsMintNoCredentialAndJoins、SupervisorFailedWithdrawalNeverReportedAsACLSuccess。真实HTTPS+kernel通道，但临时Store/替身driver与LiveVerifier，**没有本轮真实Headscale/节点proof/双机TLS验收**。线上三个服务active，state哈希仍为5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca，Nginx基线全部OK；测试未调用Headscale或更新线上服务。

用户提供Windows bootstrap路径 H:\chatgpt\远程软件开发\测试安装包\windows-native-network\interactive-bootstrap.json；实际SSH执行该bundle的pinned probe PASS（无SKIP），仅同用户桌面OS签名验收，不等同网络通过。用户拥有桌面进程，本轮未代为停止或删除身份。

后续仍需新版协调fixture/runner：旧enrollment-network.py --native直接调用旧backend并期望policy自动恢复，不能原样运行后当supervisor新验收。新版必须真实Runtime+broker HTTP，并将故障恢复改为显式新supervisor/backend实例，先确认退出join；再做零ACL/故障/TLS闭环。没有把用户提供路径或旧流程PASS冒充该步骤完成。
