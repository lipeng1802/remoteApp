# 公网服务准备切片（2026-10-06）

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
