# registered TEST 节点的独立退休

`retire-node-test` 是固定编译、无运行时选择参数的破坏性 TEST 工具，不是生产卸载器，也不把运行中的产品停止当作自己的职责。它必须先获得原安装的 trust、binding 和 enrollment 独占租约，读取原生 registered 节点及设备身份，核对完整签名程序库存与当前有效 profile。活 owner、未知文件、attempted/未知注册、pending maintenance 或过期 profile 均不能进入这条清理路径；idle 安装继续使用独立的 `retire-idle-test`。

工具以该设备真实 OS signer 通过原 profile 的 HTTPS、CA、issuer 向 `node_retirement` 请求新鲜证明。请求只带本机原生记录的 node key 与随机 challenge，不接受操作者选择 grant/节点/身份。authority 必须找到属于这个设备的唯一 provision，确认原授权已明确 revoked、该节点状态已由 Reconcile 在 Driver.Cleanup 成功后写为 cleaned，再签发绑定 device/node/address/grant/challenge 的三秒证明。只有授权过期、仅收到 revoke 回执、cleanup_pending 或删除失败均不能换取证明。该证明不是网络许可。

客户端必须先实际 join 临时 signer，再验证证明与 profile。完整请求往返及 signer join 的单调耗时全部从证明三秒期限扣除，issuer 时间通过新鲜 challenge 绑定；不会用固定“现在”延长有效期。profile 的墙钟、epoch floor 与独立单调 deadline 继续生效。任何未知回执、取消、混合响应、签名/身份/节点/challenge 不符或超时均不开始删除，也不自动重试。成功后才按既有 TEST 门禁清理精确 SDK 材料、测试身份/记录和签名程序文件；未知 SDK 文件仍阻止删除。

服务的 cleaned provision、授权及证书公开历史记录保留，不能为得到“空 Store”恢复旧快照。此证明只覆盖一个节点的远端清理，整轮验收仍须分别证明两端 worker/node/signer/product 实际 join、进程退出与独立删除。程序删除后的不确定文件回执仅可沿原 `finish-files` 受限门禁明确收尾，不能借此初始化身份或回收未知注册阶段。

当前已完成协议/本地真实 TLS 与 fixture authority 测试及兼容服务部署；真实 registered OS 节点与双机活动清理验收仍待进行。详见仓库 HANDOFF 的 r72 记录。
