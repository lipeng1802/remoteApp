# 卸载前的 owner 维护屏障

`clientowner.Owner.Maintain(ctx, action)`是可信进程内接口。它复用运行中owner持有的原生journal独占租约，不发放能脱离owner使用的“已停止”回执。

流程：永久封闭本owner的新Start/Upgrade → cancel当前runner → 等待runner实际join → 读取仍持租约的committed epoch → 在同一串行锁和原生租约下执行一次维护action。Close必须先取得同一串行锁，因此action未返回时不能释放租约。返回后仍持有租约，须显式Close才能释放；本owner不可重新启动会话。

join超时只允许显式重新进入维护以完成同一次join，action尚未执行。runner故障拒绝action。调用action之前即记录attempted；action错误、取消或不确定回执都不能在同一owner重复执行。失败不会reset、初始化或改写原生trust/identity/activation/floor。该状态仅在本进程中保持，不能当成跨崩溃的持久卸载事务。

action必须遵守context，结束前join其子任务，不得重入owner或删除原生身份、信任、activation、防回退记录。只有可信本地实现可提供action；网络/IPC不能上传回调、路径或epoch。当前验收action只删除测试临时目录内的固定marker，不删除产品文件。

测试覆盖：join前marker不可删除；超时保持native lease；另一helper进程持租约失败；join后执行action；Close不能超越阻塞的action；action错误或重复调用拒绝；runner fault拒绝；原生journal字节保持一致；显式重开仍为epoch2并拒降级。macOS/Windows原生测试需要指定当前trust-journal helper；Windows须用普通登录桌面或Interactive计划任务，SSH只负责管理上传。

Windows便携验收包需要同目录的`maintenance-tests-win.exe`、`trust-journal.exe`及`run-maintenance-validation.ps1`。runner显式提供helper，不允许因缺少环境变量跳过原生项，并在结束后恢复进程环境。测试只使用随机TEST命名空间，清理由test teardown完成。

后续仍需：独立卸载器到live owner的认证连接；固定卸载器核验signed文件集合与对象身份；持久维护/卸载事务阻止新进程与崩溃重开竞争；删除中断后按精确剩余文件重开；默认保留原生身份及防回退记录。完成这些之前，实验DMG/Inno继续不提供产品卸载入口。此屏障以Runner“返回前全部join”的契约为前提，不能证明任意逃逸子进程或被强杀进程的后代已退出。

## 实际自主产品的维护控制（r53）

`installed-autonomous-product`现使用`installeddevice.ControlProduct`在实际owner之外路由专用继承管道，普通device/network/stop请求仍交原runner。管道必须由可信本地父进程建立；没有新增网络接口或可由任意本机进程连接的socket，也没有另行实现同用户IPC身份认证。

在原`open`及产品`ready`之后，父进程发送严格的`{"Op":"maintenance_begin"}`。产品调用当前owner.Maintain，等runner与原生signer实际join后，在仍持有native journal租约时返回`{"status":"maintenance_ready","epoch":N,"joined":true}`。父进程必须在同一管道发送严格的`{"Op":"maintenance_finish"}`，随后关闭stdin。完成当前进程的binding、journal、读取线程收尾后，产品才输出终态`closed`并零退出；父进程仍须观察stdout EOF与实际进程退出。

整个维护握手上限40秒，含join与等待finish。begin/finish不能附带身份、公钥、设备请求或网络grant；提前finish、断管、取消、写输出失败、runner fault均拒绝。握手期间不再转发普通操作，不可借此启动新session或替换owner。`maintenance_ready`只是本连接正在持租约的状态，不是独立删除许可；目前既不接收删除路径，也不删除程序文件。

控制层错误可以早于runner完成，因此实际产品在所有控制返回路径先Stop/join，再关闭安装binding，最后由原收尾释放trust journal。不能因参数拒绝或管道断开而提前释放runner借用的资源。

原生组合验收在两代trust/identity升级之后启动真实compiled product，先验证带身份覆盖的begin失败且非零退出，再验证正常begin/finish、握手前后另一helper争租约失败、closed/EOF/actual exit0、退出后trust仍epoch2且设备公钥/idle状态不变。该场景使用已安装的真实OS身份和idle runner，不建立Headscale节点或活跃远程TLS会话。

## 持久维护意图（r54，取代r53结束后可直接重开的行为）

产品控制管道现在调用`Owner.MaintainDurable`。它先在原生trust journal的同一条installation record中提交`maintenance={version:1,state:"pending",id:<random 128-bit>}`，再取消并join runner；只有意图提交、原记录读回核对及join都成功才报告ready。这样进程在等待join时崩溃也不会留下允许新会话启动的窗口。同一owner因join超时而重试，只继续原事务的join，不能改为非持久模式或生成第二个事务。原有`Maintain`仍是可信进程内的非持久屏障，产品管道不再使用它。

marker保留原profile、accepted_at、anchor与epoch；不改OS设备身份、binding或activation。普通`OpenInstallation`、新owner、offline inspect/init/upgrade都会拒绝pending。旧严格schema解析器也不能忽略附加maintenance字段。显式null、未知阶段/version、错误ID及未知字段一律拒绝，不能降级为旧active格式。提交回执不确定时原owner封闭且不执行action；即使写入实际完成，仍需要明确诊断而非猜测成功。

`trust-install maintenance-state`只读验证历史签名记录并输出`maintenance_required`与epoch，不返回可用profile、密钥或独立删除许可。它不创建、不修复、不清除marker。`maintenance_finish`只结束当前握手/进程，不代表卸载完成，不清除持久pending。本切片没有生产resume/reset入口；后续必须根据签名程序文件集及真实事务状态设计显式恢复。

原生测试覆盖正常关闭、真实子进程exit77跳过defer（发生于已取消但尚未join的runner）、helper观察EOF后释放租约、新进程拒绝启动/重装/升级，以及真实native commit后模拟丢失回执。后者是回执故障夹具，不宣称真实磁盘写失败。原生epoch2及同一事务ID在重开后保留。

组合validator在原身份升级/拒绝非法请求与pending-binding回归结束后，显式建立第二个随机TEST身份验证durable产品握手。退出后普通product重开与两代trust init/upgrade都拒绝；只读maintenance诊断仍为epoch2。`inspect-maintenance-test`仅在编译Test=true且已验证pending后核对原公钥与idle状态；TEST-only teardown在取得native租约后才能清理这些随机scope。生产入口不会获得任何重置或删除能力，pending保留到未来明确恢复流程。
