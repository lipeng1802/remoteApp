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
- join/零ACL确认/Runtime停止失败返回错误，不记录正常STOPPED，不关闭Store；journal和身份保留，不Retire。进程退出会由OS释放进程锁，不是资源清理成功证明。

Runtime不随daemon父context直接关闭，避免正常TERM抢先关闭overlay；重开受原有30秒启动预算约束，自身健康故障仍取消证明工作。driver零规则确认不等同实际网络拒绝，后续需真实数据面验收。

## 验证与下一步

Mac全量 `go test -race ./...`、`go vet ./...`及Linux amd64构建通过。Linux临时测试程序三项PASS无SKIP：

- `TestDaemonNetworkConfigStrictAndSeparated`：严格JSON、权限、重叠、符号链接拒绝。
- `TestDaemonNetworkLifecycleAndFailedJoinRetainsStore`：正常退出、故障、启动失败、停止失败；真实Store/kernel socket、替身network owner，检查Stop时Store可用、失败不释放writer锁、无假READY。
- `TestDaemonMissingVerifierFailsWithoutCreation`：实际入口缺journal拒绝，无新身份/issuer，Store不变。

这些不调用真实Headscale，不等同运行中verifier或跨UID生产网络验收。临时测试程序已删除，本地构建可恢复；线上三服务active、State哈希未变、Nginx全部OK，nodes/users/preauthkeys零。

下一步隔离环境准备既有verifier身份，验真实daemon重开、故障关闭/同身份重启、root/nonroot broker；之后接前端路由与受控部署。公网admission、安装锚/floor/轮换、HMAC安全投递、GUI/安装仍待，整体4未完成。
