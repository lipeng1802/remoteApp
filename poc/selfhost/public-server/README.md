# 公网服务准备切片（2026-10-06）

## 直连/新连接回退与活动授权撤销切片

增加 `--network-lifecycle` 门禁（仍只有固定载荷，无屏幕/输入）。允许直连时进行最多 8 秒 disco 探测，使用实际数据交换后的路径状态判断；首轮观测真实 `direct`。之后抑制直连，另一条新连接确认 `selfhost_relay`，不能把未知路径当作成功。此证据不是已有长连接的无缝直连→中继迁移，也不是任意运营商/NAT 成功率承诺。

持续流先验证随机 token，再每 300 ms 交换固定心跳。私有父子管道 `revoke` 取消本次授权：立即取消上下文、显式关闭活动连接，停止监听并关闭节点；不得只关闭 listener 后保留活动连接。单测用已交换数据的 net.Pipe 验证取消同时结束双方。stream 模式与第二连接错误 token 测试不能同时开启，直连探测与强制中继也互斥。

完整门禁连续两轮零退出，均观测真实直连：撤销注册 key 后持续心跳仍可传输；显式 revoke 后服务器输出 authorization_revoked、双方 stream_closed/closed；后续连接拒绝；以同一设备身份重新启动但换新 token，旧 token 在服务器确认拒绝，新显式授权成功。身份/节点数仍稳定，无 cleanup_timeout、无强杀计通过；本轮短期 key/节点/用户及两端节点私钥清理通过。最终强化后的新连接断言要求 registered + dial_failed，第二轮已实测通过。

空数据库复测时发现 Headscale 活进程中的分配游标未回到 `.1`，严格地址门禁正确失败并完成清理。复测脚本现在先确认无节点，再仅重启隔离 `remoteapp-poc`，保留数据库和控制/中继私钥，不重置身份文件；健康检查通过后才创建本轮短期用户。若存在节点则拒绝重启。现有业务服务、Nginx 443、系统 Tailscale、防火墙均不改动。

撤销只是当前独立 helper 的私有本地管理命令；尚无设备码、签名后端撤销推送、撤销持久化/重放防护或产品 TLS 桥接。仅 token 轮换试验证明“重启设备身份不自动恢复旧会话权限”，不能推定服务端邀请生命周期已实现。本机 verify.zsh/Go 单测和 go test -race ./... 均通过，新增未授权 stream token 拒绝单测。Windows x64 二进制从本轮源码构建并上传校验后实测。两轮清理后独立复核 nodes/users 均为空、服务 active、原业务哈希一致；系统 Tailscale、业务 Nginx 和防火墙未修改。

## 最新结果：双机公网中继通过

以下部署/入口阻塞段落保留为历史，以本节为当前状态。用户完成云入口调整后，Mac/Windows 公网 HTTPS 8443 健康检查通过，DERP 探测 200，管理路径 404；UDP 3478 有效 STUN Binding 响应也通过。此前最小 STUN 探测缺少固定网络库要求的 SOFTWARE=`tailnode` 和 FINGERPRINT，服务器会拒绝；不能仅凭该超时断言端口未放行。按固定版本 net/stun 源码补齐属性后通过。

`cross-network.py` 实际零退出，全部通过：Windows serve / Mac probe 的固定载荷观测 `selfhost_relay`，经过本项目 `remoteapp-poc` 中继；正确 token 传输/错误 token 拒绝由双方状态确认；Mac 连续三次重启、Windows serve 重启后再次传输成功；身份/地址不变；第三节点 `.3` 被 ACL 拒绝访问 `.1:47476`，节点数稳定为 3；双方正常关闭，无 cleanup_timeout 或强杀后计通过。Windows helper 上传后的 SHA-256 一致。SSH/现有 Tailscale 仅用于管理，helper 使用独立身份，不依赖用户 Tailscale 登录。

收尾：本轮短期 key 按 ID 撤销，本轮节点/用户删除，两端节点私钥清理。独立复核服务器 nodes/users 均为空、服务 active、原业务配置哈希一致。Windows 专用 `remoteapp-public-poc-*` 目录保留二进制/空目录，不保留节点私钥。首次 Windows 目录 ACL 的数字 SID 缺少 `*`，导致 icacls 失败，发生在身份创建前；修正后完整验证通过，未修改已有目录权限。

helper 额外仅允许精确 `https://mk.fengmap.com:8443`，保留正常 HTTPS 证书验证，拒绝其他域名/端口、HTTP、URL 用户信息/路径/query/fragment。路径检查区分本机 `lab` 与公网 `remoteapp-poc`。`GOPROXY=off zsh poc/selfhost/verify.zsh` 在正常网络权限下零退出，包含依赖验证、单测与完整本机回归；首次沙箱 PermissionError 是 fixture 权限限制。

复测前先使用 `verify.zsh` 构建最新 Mac helper，再用隔离 Go 工具链、CGO_ENABLED=0、GOOS=windows/GOARCH=amd64 将源码编译为 `artifacts/connection-poc/selfhost-node-public-win-x64.exe`，然后执行 `python3 -B poc/selfhost/public-server/cross-network.py`。须具备本轮服务器和 Windows SSH 授权及对应密钥；脚本要求空的隔离数据库，发现已有节点就拒绝。注册 key/token 只存在管理响应和私有管道，不写配置或打印日志。

本轮不代表直连/NAT 穿透、撤销活动流、多租户设备码/邀请、产品 TLS 桥接、真实画面键鼠、多平台双向角色、长期耐久或生产运维完成。下一切片优先直连/中继回退与授权撤销，再推进邀请后端和产品桥接。

此目录是已授权测试服务器的隔离部署记录，不是通用生产安装器。固定 Headscale v0.29.4；无 Docker、无系统 Tailscale 登录、无公共 DERP。配置按该版本官方 `config-example.yaml` 核对，嵌入 DERP 开启 `verify_clients`，清空外部 DERP URL。CentOS 7 已停止维护，此环境只用于短期 PoC，不作为生产底座。

## 已部署

- 服务器 `182.92.117.114`，公网入口 `https://mk.fengmap.com:8443`，STUN UDP 3478。
- 独立系统用户 `remoteapp-poc`，二进制 `/opt/remoteapp-poc/headscale`，配置 `/etc/remoteapp-poc`，私有状态 `/var/lib/remoteapp-poc`（0700）；不创建设备注册 key，不输出或复制服务密钥。
- 官方 Linux amd64 二进制 SHA-256 `212ed0a884c0d3541e094c4bebbe94397df6f4e01bd3d7f059c520cb55e0d757`，与官方 checksums.txt 一致。服务器实际可运行该版本。
- systemd `remoteapp-poc.service` 已启动，但**没有设置开机启动**。运行用户不是 root；不修改原服务。
- 新增 Nginx `conf.d/remoteapp-poc.conf`，监听 8443；原 nginx.conf 与 mk.fengmap.com.conf 的前后 SHA-256 一致。原 443 保持原来的 HTTP 403 响应。
- 后端 HTTP 18443、metrics 19090 只绑定 loopback；管理 Unix socket 0600，不向公网暴露管理接口。公网反代 `/api`、`/swagger`、`/metrics`、`/debug`、浏览器 `/register` 返回 404。
- 仅复用服务器上的现有证书路径，未读取私钥；证书覆盖 mk.fengmap.com，2026-10-13 到期，需用户续期。

## 实际验证与当前阻塞

服务器内通过 Nginx 的受信任 HTTPS `/health` 返回 pass，`/derp/probe` HTTP 200，管理路径 HTTP 404；Headscale 处于 active，STUN 和 Nginx 监听存在。服务仍没有注册节点，本切片不声称中继数据传输验收通过。

Mac 和 Windows 对公网 8443 的请求均超时，Mac 的标准 STUN Binding 请求也超时。DNS 解析正确指向 182.92.117.114，指定 IP 的 HTTPS 访问仍超时。服务器 iptables INPUT/FORWARD/OUTPUT 全部 ACCEPT，无 firewalld 活跃规则；没有擅自修改防火墙或云安全组。用户已反馈安全组放行，但外部可达性尚未证实，需核对实际绑定实例的入方向规则、协议和源地址范围，以及其他云侧防火墙。

额外进行 12 秒同步入站抓包检查，仅输出是否收到 PoC 端口数据、不保存包或载荷。Mac 同期发起 TCP 8443 与 UDP 3478 探测，服务器返回 `NO_INBOUND_POC_PACKET_SEEN`；证据指向服务器外部入口限制，具体云规则仍须核对。

首次部署在 Nginx 异步 reload 新监听尚未就绪时检查失败，已自动停止新服务、把新增站点移动到 `/etc/remoteapp-poc/nginx.disabled.conf`，保留状态；原业务哈希不变。随后加入就绪等待，经 `activate.sh` 检查既有本轮文件一致后恢复成功。没有靠关闭 TLS 验证绕过问题。

## 脚本边界与回退

`deploy.sh` 仅允许专用 `/tmp/remoteapp-poc-deploy.*` 暂存目录，检查二进制哈希、目标不存在、端口空闲；目标存在时拒绝重复覆盖。`activate.sh` 仅用于本轮已回退部署，先比较配置和原业务哈希，不重建身份。两者语法检查通过，实际部署/激活已执行；不是幂等生产安装器。暂存记录在服务器 `/tmp/remoteapp-poc-deploy.hbMUEVFe`，其中没有设备 key 或会话 token。

撤回时仅停止 `remoteapp-poc.service`，将**新增的** `conf.d/remoteapp-poc.conf` 移出 include 目录，再对正在运行的 `/fm_inetpub/software/nginx/sbin/nginx` 使用原 nginx.conf 执行 `-t` 和平滑 reload；保留数据库和密钥，不删除业务文件。该回退路径首次部署中已实际执行。

## 后续

先解决公网可达性，再为独立 helper 增加严格限定的 HTTPS 控制入口。当前 `main.go` 仍仅允许本机 HTTP fixture，不能直接拿现有测试包连接此服务。之后通过短期单次注册 key/私有管道执行 Mac↔Windows 固定载荷、错误 token、第三节点拒绝、连续重启、正常退出和 DERP 路径验证；SSH/Tailscale 仅用于管理，不作为 PoC 数据路径。此时仍不接真实键鼠/画面，不宣称最终架构完成。

官方配置依据：https://github.com/juanfont/headscale/blob/v0.29.4/config-example.yaml
