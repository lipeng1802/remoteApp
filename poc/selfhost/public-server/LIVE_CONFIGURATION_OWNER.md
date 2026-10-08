# 4C-4P：已安装配置的 live owner 与退出屏障

2026-10-08用户确认r3完整Windows桌面入口返回 `VALIDATION PASSED: native live owner lease + HTTPS watch/tunnel cancel and join. NOT cross-machine Headscale, product TLS/GUI or signed installer acceptance.`，本子切片闭环。此为用户反馈的整轮桌面结果，覆盖脚本全部必需项，不捏造逐项日志；下方待验描述是历史准备记录。实际Headscale/产品TLS进程组合、GUI和签名安装包仍未验收。

最新修复：用户r2桌面5项owner（含native）PASS，但HTTPS集成复用Unix服务器Store初始化失败。r3使用仅内存临时issuer的跨平台测试夹具；4项session入口也去除Unix Store依赖，正式存储权限未放宽。Mac全量race/Windows vet通过，真实Windows SSH下本轮HTTPS/流集成及4项session回归全部PASS；r3完整桌面脚本仍待，不将r2部分成功或SSH非native测试冒充整轮完成。

切片4第一段已开发：统一配置/会话owner和真实HTTPS watch、opaque tunnel生命周期适配。Mac原生实测与本地HTTPS/流集成通过，Windows新包桌面待验。**未切换现有enrollment-helper命令或产品GUI到新owner，未完成实际Headscale节点/产品TLS进程组合验收，也不是签名安装包或整体切片4完成。**

## 升级与所有权

- `clientowner.Open` 只打开已初始化journal，以编译Release提供anchor，不自动create/reset。Factory和Runner是可信本地代码注入点，不由peer/manifest提供。正常客户端应使用Release.NativeFactory，测试内存journal不能作为保护存储替代。
- `Start` 取得已提交且当前有效的profile再传给runner；调用方不能传另一份profile。一次只允许一个runner，会话绑定profile单调到期deadline，Start/Upgrade/Close串行，操作锁等待尊重context。
- `Upgrade` 先验证候选，再cancel旧runner并等待Handle.Done，复检时间、原生提交/读回后才成功。无效候选不停止已有健康会话；降级和同epoch改写拒绝。不会自动enroll/reconnect或回退。
- 正常取消必须在所有子watch/连接/隧道/节点/进程join后返回nil。非nil runner错误、退出超时或提交故障锁闭owner。超时不提交新配置、不释放journal租约；Close可重试仅用于join/回收。恢复需要先关闭并显式检查/重新打开可信已提交状态。
- `Group` 封口后拒新增worker，再cancel/join。它不是callback内部线程的自动发现机制，runner仍须join嵌套工作；借用OS signer须在runner返回后才能销毁。

`streamgate.ProfileSession` 从安装profile与编译root/service/floor构建真实ProfileNetworkClient，校准authority clock、启动签名StateWatch/ManagedGate，没有raw endpoint/CA/issuer回退。watch异常先Deny/cancel，再join watch和work，最后关闭HTTPS client。work可运行Gate.ServeTLS/ConnectTLS，但必须拥有并join其准备的tsnet节点/listener/嵌套worker；**本轮尚无完整实际节点启动入口**。

原有连接取消监视器、Serve/Tunnel与Gate.Probe授权watchdog已补显式join。session.authorized不再在deadline后脱离callback返回；callback收到500ms deadline且必须尊重取消。坏callback会延迟关闭，由外层owner超时锁闭并保留租约，不越过升级屏障。

## 验证证据与边界

4项owner核心测试通过：join先于commit/显式重启，超时保留租约，runner/commit故障拒重用，Group封口/join。存储为内存故障模型。

`TestNativeLiveOwnerRetainsLeaseOnTimeoutAndReopensEpoch` 在Mac真实Keychain/helper通过，无SKIP：超时仍拒第二原生owner，旧runner结束后Close、显式reopen、epoch2升级/使用成功，并精确清理随机测试channel。

`TestInstalledOwnerJoinsRealHTTPSWatchAndOpaqueTunnelBeforeUpgrade` 通过：真实本地HTTPS校准/签名watch、双端持有证明/opaque数据交换；有查询在途时升级关闭隧道，故意延迟runner退出仍不commit；watch/stream join后成功，旧watch无后续查询，旧流关闭，降级拒绝和显式epoch2启动。**该测试的存储、overlay端点元数据和设备签名器是私有fixture，不是实际Headscale WhoIs、OS signer或Swift/Schannel产品TLS证据。**

新增授权callback deadline不脱离join测试及已有隧道负向回归通过。Mac全量race、Mac/Windows全量vet通过。初次受限沙箱回环bind失败，允许隔离本地监听后复测通过，未放宽判断。普通全量未开启旧native环境项，不能宣称旧native项本轮全部实测；新增Mac原生owner单独启用真实helper验收。

未验硬崩溃、断电/EIO、管理员/同用户恶意篡改、快照回滚、正式发布认证或跨网性能。未改线上服务/Nginx/ACL、真实发布根/CA、旧GUI或安装包。

## Windows新验证入口

由 `cmd/live-owner-validation-build` 新建隔离包，测试release签名运行时在内存生成，只操作精确随机native测试namespace，不依赖旧4o/4mn。脚本核对程序/helper摘要，5项owner测试（含1项native）、1项HTTPS/流集成、4项session回归均PASS且无SKIP/退出0才输出成功；恢复原环境变量，原生测试清理精确测试凭据。摘要是交付完整性，不是发行者代码签名。

正常已登录Windows桌面PowerShell（无需SDK/管理员/Tailscale/旧包/READY）执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\remoteapp-live-owner-4p-20261008-r3\run-live-owner-validation.ps1"
```

预期：

`VALIDATION PASSED: native live owner lease + HTTPS watch/tunnel cancel and join. NOT cross-machine Headscale, product TLS/GUI or signed installer acceptance.`

本机最新包 `artifacts/connection-poc/windows-live-owner-4p-r3.zip`；使用r3而非r2/初构包，保留启动前取消不误报故障修正并去除Unix Store测试依赖。已上传核对ZIP摘要，解压至当前用户/SYSTEM隔离目录，PS语法检查通过；SSH仅执行不涉及桌面凭据的5项网络/session测试。profile执行时生成，不受上一轮fixture过期影响，但系统时间仍需正常。本轮整轮桌面结果待回传。只本地提交，不推送远程。

下一段：收Windows结果，提供编译锚绑定的新客户端/节点启动入口，将安装配置、OS signer、实际Headscale节点、watch及产品TLS工作进程纳入同owner，再双机验收。不能将当前Runner契约与本地fixture当作该接线已完成。
