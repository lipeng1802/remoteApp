# 自主注册后的持续授权（切片3A/3B）

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

## 3B 跨层组合证据

新的本机组合测试使用真实`invite.Store`执行两个设备的注册、邀请、申请、明确批准和签名撤销；两端分别签署provision intent，由真实`ProvisionBackend`执行enroll/bind。双端绑定后同一backend必须先投影唯一方向规则，才能为两端watch签发ACTIVE。

撤销验收顺序被固定为：

```text
signed revoke
  -> ProvisionBackend.Reconcile
  -> policy driver receives zero rules
  -> both signed-state watches terminate as faults
  -> both enrolled owners close and join their node resources
```

测试不直接向watch注入手写inactive，而是让撤销经Store和Backend传播。这证明了服务端动态授权与新客户端watch的程序内闭环，并证明零规则投影的同步返回早于watch退出验收。

策略driver仍是内存替身，节点/proof仍是fixture；这不是Headscale内核规则已更新、旧overlay流量已被拒绝或双机原生保护存储已验收的证据。

## 当前证据和剩余项

新增5组watch单元测试覆盖inactive→ACTIVE一次性转换、ACTIVE→inactive终止、binding替换、签名/混字段/传输/身份/no-proof拒绝、caller deadline和显式stop join；另有1组Store/Backend/双watch撤销零规则跨层测试。相关installeddevice/invite/broker/streamgate/clientruntime/nodeproof race以及Windows/Linux vet通过。

当前没有正式入口调用`WatchedEnrollmentRunner`，r30 claimant只有memory signer而没有第二SDK。下一步构建双native borrower协调入口，让双方各自完成原生enroll/bind/watch，再在真实Headscale上同时验收撤销/过期后的零规则和旧连接拒绝。随后才能把watch借给切片4的产品TLS/HMAC/画面键鼠流。Windows关机期间可先完成协调代码与本机组合，但Windows实机和跨机网络验收仍待。

## 切片4A 会话生命周期桥

`WatchedEnrollmentRunnerWithSession`是不替换正式入口的显式opt-in组合。它只接受编译期可信的`SessionWork`，父进程帧不能传入argv、地址、密钥、binding或工作函数。`SessionWork`只获得生命周期context，enrollment credential、ticket、binding、authority client和签名器均不暴露。

会话只能在同一watch首次ACTIVE且active结果已成功交付后启动。watch fault、grant到期、显式stop或owner cancel后的回收顺序固定为：

```text
session cancel + actual join
  -> watch stop/join
  -> proof cancel/join
  -> node SDK close/join
  -> OS signer join
```

会话在授权仍有效时提前返回会终止owner；显式取消后的真实会话错误不能被取消隐藏。当前桥只收口生命周期和join次序，还没有向它提供新自主路径的TLS descriptor、HMAC、overlay listener/dialer或画面键鼠payload；因此不是产品会话验收。

## 切片4B 签名会话双端身份

Grant中的target/controller是设备公钥哈希，不能直接作为产品TLS会话的验签公钥。新的`session_identity`网络API只允许已注册、属于同一已批准grant的参与者在动态策略ACTIVE时请求。返回值由issuer签名，最长3秒，同时绑定：

- 完整grant和当前verified Headscale binding；
- Store中经设备签名注册绑定的两个ed25519公钥；
- 请求端新生成的32-byte challenge、issuer时间和grant绝对到期。

撤销、inactive策略、未绑定/外来设备、坏公钥记录、坏challenge或混合reply全部失败关闭。`SessionPeers`只在installed owner内部核验后创建，返回的公钥均为副本，不向父进程输出。SessionWork开始前再核对本机OS signer、node key、binding和grant时间；它可获得已验证的公开peer/grant/binding/role，仍不获得registration credential、ticket、authority client或签名私钥。

相关测试覆盖真实Store/Backend/HTTPS签名返回、两端公钥哈希绑定、签名/grant/binding/challenge/到期篡改、外来设备、撤销拒绝、重复/大小写字段与混合reply。这收口了产品会话所需的公钥来源，但还没有在新自主node上启动overlay listener/dialer、TLS product进程或HMAC交付。
