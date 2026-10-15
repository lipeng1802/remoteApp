# 自主注册后的持续授权（切片3A）

本切片把1/2的新自主注册入口从“完成bind即返回”推进到“首次ACTIVE后持续持有授权watch”。它没有启用产品GUI或正式客户端入口，也不是双机实际断网验收。

## 客户端状态机

`WatchedEnrollmentRunner`是新的显式可信组合，旧`Runner`、`EnrollmentRunner`和`BoundEnrollmentRunner`行为保持不变。父进程仍只能提交已批准grant；不能提交binding、节点、IP、ticket、verifier、clock或watch参数。

内部顺序固定为：

```text
OS signer/profile lease
  -> signed grant clock
  -> native node prepare/enroll/SDK Up
  -> verifier permit/live proof/bind
  -> bounded initial activation
  -> terminal continuous state watch
  -> watch join -> proof cancel/join -> SDK close/join -> signer join
```

bind后允许最长12秒的初始激活窗口。此时只接受issuer签名、当前grant、当前设备node/IP严格匹配的state；可看到inactive，等待另一端完成bind，但不会启动payload worker。首次ACTIVE后binding被固定，之后每500ms刷新三秒签名state。以下任一情况永久结束owner：

- 签名、grant、设备公钥或本机node/IP变化；
- binding变化、inactive、撤销或绝对到期；
- HTTPS请求失败、超时或混合回复；
- caller deadline、proof/node身份变化。

显式stop/cancel会join但不记为授权故障；deadline或watch终止记为fault。watch输出只有固定`network_watch/passed/active`，不包含grant ID/签名、binding、节点、IP、ticket或credential。没有自动重试、重新enroll、inactive后恢复或换scope。

## 服务端关系

服务端现有`NetworkSupervisor`和`ProvisionBackend.Reconcile`仍是唯一策略writer：只有两端verified bound记录和成功规则投影才签ACTIVE；撤销/过期/策略、存储或verifier故障会签inactive或关闭API，并尝试投影零规则。客户端watch不会把签名state当成Headscale规则已经从内核移除的独立证据；真实验收必须同时观察客户端owner join、服务端零规则成功以及旧overlay连接不可达。

## 当前证据和剩余项

新增5组测试覆盖inactive→ACTIVE一次性转换、ACTIVE→inactive终止、binding替换、签名/混字段/传输/身份/no-proof拒绝、caller deadline和显式stop join；相关installeddevice/invite/broker/streamgate/clientruntime/nodeproof race以及Windows/Linux vet通过。

当前没有入口调用`WatchedEnrollmentRunner`，因为真实ACTIVE要求两个实际节点；r30 claimant只有memory signer而没有第二SDK。下一步构建双native borrower协调入口，让双方各自完成自主enroll/bind/watch，再验收撤销/过期后的实际零规则和旧连接拒绝。随后才能把watch借给切片4的产品TLS/HMAC/画面键鼠流。Windows关机期间可先完成双端协调代码与本机fixture，但Windows实机和跨机网络验收仍待。
