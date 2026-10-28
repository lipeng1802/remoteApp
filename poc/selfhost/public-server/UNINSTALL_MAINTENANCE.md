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
