# 4C-4D：独立权威服务的可选网络装配

代码、全量Go race/vet、Linux入口构建和三项真实Linux隔离组件测试通过。**未替换线上daemon、未创建长期verifier、未切换Headscale模式、未公开network路由。** 先前4C-4C双机结果属于私有fixture，不算本入口部署验收。

## 显式入口与私有配置

原authority-daemon八个参数不变，默认device-only。可选追加 `--network-config /绝对路径/network.json`，仅可信运维本地配置，不接受HTTP/IPC路径。device-frontend仍未接network路由（404）。

配置文件必须为进程有效UID所有（部署root）、0600，父目录0700。紧凑单行JSON、8191字节上限，拒绝重复/大小写重复/未知/尾随字段：

```json
{"version":1,"verifier_directory":"/var/lib/remoteapp-verifier","policy_directory":"/var/lib/remoteapp-network-policy"}
```

目录必须已有、同UID、0700、规范绝对路径且无符号链接。verifier、policy、Store、配置、socket父目录不可重叠或嵌套；profile与network配置可共用可信配置目录。不能将policy写入Store或verifier journal目录。

CLI仍固定 `/opt/remoteapp-poc/headscale -c /etc/remoteapp-poc/config.yaml`，control为既有独立Headscale8443，policy要求database模式；程序不会修改模式。**现网仍file模式，不能直接追加参数。** 先准备隔离控制面与状态做验收，再制定受控迁移方案。

## 生命周期

- 验证release profile、issuer及配置，独占已有Store，不创建新issuer。
- `verifier.Open(create=false)`只重开既有journal/node身份；缺失拒绝，无Mint/rekey回退。
- 固定真实driver、Runtime overlay及Store可信设备公钥nodeproof装配supervisor，1秒串行维护；初始投影和维护Ready后才监听broker。
- supervisor故障关闭broker并非零退出，provider恢复不自动重建；维持 `Restart=no`，检查后明确重启。
- 正常退出先停止broker请求，再supervisor取消/join及最佳努力零ACL，再Runtime stop/join，最后关闭Store。
- join/零ACL确认/Runtime健康或停止失败返回错误，不记录正常STOPPED；journal和身份保留，不Retire。**只有未完成join时不关闭Store**；确认supervisor及Runtime全部退出后，即使健康或策略撤回报错，也安全关闭Store。writer.lock为O_EXCL持久标记，OS退出不会自动删除；不能用“进程已退出”代替join/Close或自动删锁。

Runtime不随daemon父context直接关闭，避免正常TERM抢先关闭overlay；重开受原有30秒启动预算约束，自身健康故障仍取消证明工作。driver零规则确认不等同实际网络拒绝，后续需真实数据面验收。

## 验证与下一步

Mac全量 `go test -race ./...`、`go vet ./...`及Linux amd64构建通过。Linux临时测试程序三项PASS无SKIP：

- `TestDaemonNetworkConfigStrictAndSeparated`：严格JSON、权限、重叠、符号链接拒绝。
- `TestDaemonNetworkLifecycleAndFailedJoinRetainsStore`：正常退出、故障、启动失败、停止失败；真实Store/kernel socket、替身network owner，检查Stop时Store可用、失败不释放writer锁、无假READY。
- `TestDaemonMissingVerifierFailsWithoutCreation`：实际入口缺journal拒绝，无新身份/issuer，Store不变。

这些不调用真实Headscale，不等同运行中verifier或跨UID生产网络验收。临时测试程序已删除，本地构建可恢复；线上三服务active、State哈希未变、Nginx全部OK，nodes/users/preauthkeys零。

## 4C-4E 真实 daemon 验收（2026-10-07）

新增 `cmd/daemon-network-check` 与 `public-server/daemon-network.py`。最新Linux daemon构建和check程序上传后核对哈希，仅在空的PoC控制面临时database模式执行；实验Store/profile/发布根/CA随机生成在服务器临时目录，不使用或上传真实产品发布私钥。独立root daemon进程，nobody为实际批准的kernel UID，另一root客户端作为错误peer被拒绝；nobody能读取issuer并得到无grant网络请求403（不是503），但不能读存在的Store/profile/node私有材料。

完整实际退出码0：真实verifier create/stop→daemon只重开、显式零ACL、正常TERM/零退出、同key/IP恢复；第二次启动后实际stop remoteapp-poc，Runtime健康故障使daemon明确FAIL/非零退出，broker socket消失。恢复provider没有自动重启daemon；原verifier同身份恢复，再显式启动daemon成功，issuer/设备Store字节不变，最后正常停止与零ACL。精确Retire仅本轮ticket，nodes/users/preauthkeys零，实验目录/上传程序删除，原file模式和原PoC策略恢复。线上State哈希仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca，Nginx全部OK、三服务active。

首次失败为实验目录在PoC的0700私有父目录，非root不能遍历；调整实验目录至独立/tmp、保持各私有目录0700，不放宽原权限。第二次真实失败发现4C-4D把已join的Runtime健康错误当未join，遗留writer.lock导致第三次显式重启失败；新增独立Joined标记，仅确认全部任务退出才Close Store，即使故障仍非零退出。新增joined_fault回归，未join仍保留writer锁。失败轮精确Retire/恢复策略后才重跑，不计为通过。

本轮没有批准设备grant/真实客户端nodeproof/产品TLS载荷，也没有Windows桌面签名器参与；旧fixture双机结果不冒充新daemon的这些证据。只接私有broker，不是线上前端或公网部署验收，整体4未完成。

下一步受控接入前端的私有网络路由，再验跨进程HTTPS签名设备与network请求、拒绝/故障503、可信profile；之后安排新daemon双机完整授权/真实nodeproof/TLS与部署。公网admission、安装锚/floor/轮换、HMAC安全投递、GUI/安装仍待。
