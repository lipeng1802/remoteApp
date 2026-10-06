# 真实注册、节点持有证明与动态策略验收

独立 PoC，不接产品屏幕/键鼠，不公开注册服务。管理 SSH 用已有系统 Tailscale；证明与固定载荷使用各 helper 独立 tsnet，控制 URL 只允许 `https://mk.fengmap.com:8443`，保留正常证书校验。

helper/验证节点清除其本进程继承的系统登录/OAuth 环境、禁托管日志上传、禁缓存 map；保留真实 UDP 并抑制直连，不使用旧 SDK 中已复现泄漏的 ALWAYS_USE_DERP dummy socket 模式。不修改系统 Tailscale 配置。

## 当前证据（2026-10-07）

真实 Linux x64 验证节点、Windows x64 被控 helper、Mac Intel 控制 helper，通过同一已批准 grant 的真实 ProvisionBackend/Headscale Driver 闭环：

- helper 在获取网络凭据前，离线创建私有节点身份并自己签名注册意图。此时不发网络注册；真实注册后的 Self 公钥必须逐字一致，否则停止。三平台实际一致，Mac 进程重启后也一致、地址不增生。
- 管理端核对工单用户/节点/地址；随后通过实际 tsnet 的唯一可信验证节点连接进行新鲜 256-bit nonce 挑战。WhoIs/实际控制面节点公钥与应用 Ed25519 签名都匹配才绑定；不是读取节点行即放行。
- 故意给 Mac helper 错误应用私钥：真实挑战到达，helper 无签名并关闭连接；要求后端 `peer_rejected`、helper challenges=1 / signed=0、工单未过期。超时/未拨通不算负向成功。正确私钥重启同一节点后成功。
- 未绑定时零策略拒绝载荷；仅双方都绑定才投影控制端→目标:47476。真实正向载荷成功，反向发起拒绝且控制 helper 没有收到反向载荷。
- 清空策略必须回读精确 `{"acls":[]}`；连续三次真实 LocalAPI overlay 连接请求拒绝，仍存活的目标数据计数不增加。重新投影原批准规则后同一 helper 再次传载荷成功，排除只因程序停了/凭据错而“拒绝”。
- 撤销后策略与注册资源回收；重新打开持久 Store 并新建 Backend/Driver（无旧缓存）恢复有效规则、撤销后不复活。这里是磁盘重开/冷驱动，不是服务器断电、整机重启或生产 daemon 故障演练。
- 正常停止均零退出，无强杀/cleanup_timeout 计通过；临时用户/节点/两端 tsnet 私钥精确回收，服务恢复原 file 模式/原 ACL，业务 Nginx 两项哈希保持。

测试中 Headscale 会对空 ACL 撤回 peer 路由。`DialOnlyOverlay` 向该 helper 私有 LocalAPI 发出真实拨号请求；收到 Dial-Self 则明确拒绝，绝不跟随到系统网络。证据包括路由撤回拒绝/overlay 拨号拒绝和零载荷，不扩大为对端持续存在于网络图时的逐包丢弃抓包证据，更不宣称已有长连接必然由 ACL 即时切断。已有流的授权撤销仍依赖此前签名状态门禁；下一步将两条路径桥接。

## 固定版本与限制

`nodeidentity` 使用固定 tsnet 1.102.5 的公开 StateStore/Prefs/Persist/Profile 类型预置本应用的独立私有 profile，没有虚构注册 NodeID，也没有修改原 Tailscale 状态。状态格式是**版本敏感适配器**，不是 tsnet 官方稳定的“注册前取公钥”API；升级 SDK 必须重跑三平台公钥一致性门禁。已有状态损坏、已有非空目录缺失状态文件、符号链接或控制 URL 改变时失败，不重生成身份。当前要求单进程独占状态目录；尚无生产多进程锁/平台安全存储整合。

Headscale 0.29.4 配置值是 `policy.mode: database`，不是 CLI 帮助文案的 `db`；以 [该版本官方配置](https://github.com/juanfont/headscale/blob/v0.29.4/config-example.yaml) 和真实更新验收为准。证明时短暂只加可信验证节点→候选:47477，结束恢复原数据策略；恢复错误毒化驱动、同步共享 Store 失败栅栏，不再发凭据直到 Reconcile 成功。普通无效签名不会被当成策略故障。

Bind 的预算为 12 秒，包含临时策略投影/首次或重启后跨网握手/恢复；其他核心管理操作保持 5 秒，恢复兜底单独最多 5 秒。原注册 key ≤120 秒和 grant 300 秒的绝对到期不延长，最后绑定仍检查当前授权和原期限。挑战新鲜度由随机 nonce 和连接截止保证，不依赖用户机器时钟同步；注册意图仍遵循既有 ±30 秒签名窗口。

服务器 fixture 模拟应用设备，测试私钥通过私有父子管道交给 helper。正式产品应使用各设备自己的私钥/本地安全存储；不能把 fixture 的设备私钥返回流程开放公网。fixture 的 Reconcile 是显式测试操作，不是已上线的周期调度 daemon，也未做本轮新的 300 秒动态 ACL 长测。

## 复测

需要专用服务器/Windows SSH 均可用，隔离 Headscale 节点和用户均为空。证书续期由用户安排。构建以下程序，使用既有固定 Go、离线 module cache 和 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`（后端）、`GOOS=windows GOARCH=amd64`（Windows helper）：

- `./cmd/enrollment-fixture` → `artifacts/connection-poc/enrollment-fixture`（Linux x64）
- `./cmd/enrollment-helper` → `artifacts/connection-poc/enrollment-helper`（本机）
- `./cmd/enrollment-helper` → `artifacts/connection-poc/enrollment-helper-win.exe`（Windows x64）

在仓库根目录运行：

```sh
python3 -B poc/selfhost/public-server/enrollment-network.py
```

脚本先备份隔离 Headscale 配置，空网络时短暂切到 database 并显式写 deny-all，最终恢复原 file 配置/原策略；不改业务 Nginx/443、不开放新端口、不改 Windows 防火墙、系统 Tailscale 或安装包。证明端口仅 tsnet 私有虚拟监听，不是新增公网端口。后台 CLI 原始输出、bootstrap/注册凭据、应用私钥禁止打印或交互启动。

恢复若发现节点/用户未清空会拒绝还原，不以恢复成默认允许来掩盖清理错误；需按持久工单精确回收。脚本临时二进制可保留，私钥目录必须回收。

下一阶段：接上产品 TLS 固定载荷与既有签名状态门禁，消除旧 fixture 固定 `.1/.2` 地址假设；加入受控服务的周期 Reconcile/故障恢复，再考虑公开设备码入口、GUI/安装包和防滥用。当前不是最终用户免登录产品上线验收。
