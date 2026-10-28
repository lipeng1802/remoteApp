# 安装器统一调用入口（r50）

`installer-adapter` 为一个编译固定的 install 或 upgrade 操作协调三个子程序，供后续 DMG/PKG 与 Inno 前端复用。当前仅装入隔离 TEST 验收包，旧 GUI 安装包仍使用原有流程。

入口无参数，不提供运行时 root、channel、epoch、payload 路径或凭据选择。构建时固定 release specification、epoch、operation，以及同目录 `install-transaction`、`installed-release-manager`、`installed-bootstrap-manager` 的 SHA-256。启动先验证全部三个文件，再在每次执行前重新核对；manifest/payload/provisioner/helper 的约束由原有子程序继续执行。正式安装器还须在受保护且验证发布者签名的解包目录调用，当前哈希检查不能替代发布者代码签名，也不能消除可写目录中的检查与执行竞争。

顺序如下：

1. transaction 准备并返回 exact epoch 的 staged/promoted；允许一个新 owner 重开不确定写回执。
2. release manager 激活目标 epoch，必须正常退出。
3. 新 transaction 只读确认 exact promoted。
4. bootstrap manager 最多四个新 owner，覆盖 import、promotion、pointer commit 三个持久化边界及最终重开；最终必须正常返回 exact active epoch。
5. 分别执行 release/bootstrap status，均为 exact active epoch 才输出 `{"status":"active","epoch":N}` 与退出码 0。

失败输出固定 `installer_adapter_rejected`、退出码 1。失败不会自动删除候选、回滚、清空原生身份或启动 GUI。没有卸载操作；后续卸载需要明确区分程序文件移除与设备身份/防回退记录保留，并在活动 owner 已 join 后执行。

每个子程序的执行上限为 30 秒。组合入口最坏有十次子调用，验收器为 adapter 分配 330 秒总预算，其他单个工具仍使用原 20 秒预算。原生 macOS 组合首次操作超过 20 秒，因此不能套用单 helper 预算来监督组合入口。

`autonomous-upgrade-validation-build` 为 v1/v2 分别生成 adapter，签名在计算外层 SHA256 清单之前完成。原生组合验收先调用 adapter 完成操作，再沿既有低层流程重开核验幂等性；最终精确清理 TEST 状态。包为一次性消费，已运行的 staging 不可再次作为完整验收输入。
