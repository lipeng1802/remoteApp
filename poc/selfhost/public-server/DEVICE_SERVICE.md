# 切片 4C-2B/2C：独立设备-only 服务与回环实际部署

## 2026-10-07 实际部署（当前）

用户明确允许上传预发布在线 TLS 私钥与 issuer 状态至其服务器 182.92.117.114，随后完成独立部署。仅 server-private/client-public 与程序/unit/脚本上传；发布根和 CA 私钥没有离开本机。服务 remoteapp-device 使用 remoteapp-pre 专用无特权用户，最终 active，开机 disabled，仅 127.0.0.1:8444。CentOS 7 实机确认 capability 空、NoNewPrivileges、只写 authority 状态目录，在线 TLS 私钥/state 为 600。

通过临时 SSH forward 首次及重启后两轮发布签名 profile/独立 CA/真实域名 HTTPS 探针均 PASS，malformed/重复/network/health 固定拒绝；不注册设备或写设备 state。实际停止后监听关闭/锁文件消失，systemd Result=success、ExecMainCode=0、ExecMainStatus=0，重启恢复同一身份。状态文件哈希前后相同。现有 Nginx 主配置/所有 conf.d 文件哈希核对通过，remoteapp-poc 保持 active；未修改 Headscale 策略或节点、业务443、公网端口、安装 GUI。

明确核对并删除本轮 staging（含冗余上传私钥副本），正式材料及本地原件保留；Nginx 基线保存 /opt/remoteapp-prerelease/nginx-baseline.sha256。临时 SSH forward 已关闭。该验收不是公网可达/Headscale 全链路/客户端安装信任验收。服务仅 device-only，network 路径仍 404。

新增 `deviceservice` 与 `cmd/device-service`，使用独立预发布材料。**不等于公网最终产品或整体切片 4 完成**。没有调用 fixture 特权自动审批，没有接入网络 Mint/Bind/Verifier；仅启用设备签名的注册、一次性邀请、明确批准/拒绝、状态与撤销路径。`/v1/network` 和 `/health` 不公开，返回 404。邀请产生的 signed grant 本身不能打开网络授权。

## 信任与状态

运维通过公开命令参数提供固定 release root/service/minimum epoch，不能从 API 自取安装锚。服务校验签名 profile、有效期/epoch、指定监听端口、独立 API CA/域名叶证书/私钥和真正 Store issuer 一致后才能监听。要求绝对真实目录、非符号链接文件/私有目录、在线私钥和 state owner-only。只重开既有 state；缺失文件拒绝，不初始化新 issuer、不默默恢复或换根。进程参数无私钥/验证码，日志仅固定 READY/STOPPED/FAIL，不输出请求、grant、TLS/issuer 密钥或底层错误。

客户端安装锚/floor 持久化、防重装/快照回退、发布私钥加密备份仍未实现；运维的固定参数不是这些功能的替代。

## 边界与生命周期

- TLS 1.3 及 HTTP/1.1，不信任代理地址头；复用签名时间窗/持久重放和匿名注册预算。
- process admission：最多 256 peer 桶（IPv6 /64）、每 peer 600/min、全局 6000/min；不是分布式 DDoS 防护，重启会重置 process quota，但持久设备/注册/重放预算不重置。
- 4 个同时处理工作槽、32 个已接受 socket 上限；超额 socket 直接关闭；禁用 keepalive，不无限积压工作。
- header/read/write/idle/shutdown 期限；配置绝对到期/信号取消后关闭监听并正常 join HTTP，最后 Close Store。文件系统本身的阻塞不被宣称有强制取消保证。
- HTTP 文档长度限制保留 4096/8192；解析改为单 JSON、拒绝重复/大小写别名（所有嵌套层）、未知字段及尾随文档，允许正常 pretty JSON。旧回环 handler 不是公网入口；新 wrapper 仍必须与 Server/TLS/admission 一起部署。

## 验证

新增真实本机独立 HTTPS 测试：域名/CA/发布签名可信配置，设备注册、重放拒绝、pretty JSON、重复别名/超长拒绝、network/health 404、Stop 后连接失败，重启保持 durable replay；配置到期自动停止；错误根/floor/端口、私钥宽权限及 state 缺失拒绝且不重生成。测试身份/CA 都在临时隔离目录，不涉及既有预发布 Store 或用户 OS 配对凭据。

完整 Go race 回归和专用 vet、Linux 静态构建为本轮验证项；本轮不代表 Windows 新服务实测、公网/Headscale 联合验收或安装 GUI 已完成。

## 部署准备与审批阻塞（下述阻塞为历史，已获准部署）

新增 `remoteapp-device.service`（独立无特权用户 remoteapp-pre、无 capability、只写 authority 状态、无重启/开机自动启用）及 `deploy-device-service.sh`（拒绝既有安装/账号/端口，校验二进制哈希，部署失败停止新服务保留状态，校验 Nginx 配置哈希）。首轮仅 `127.0.0.1:8444`，不新增公网监听或 Nginx/安全组；CentOS 7 systemd 实际兼容性尚待部署验证，不用模板审查当实测。

2026-10-07 安全审查拒绝 scp：用户已选自托管，但未明确授权具体服务器 TLS 私钥与在线 issuer 私钥上传到 182.92.117.114。**scp 未执行，安装脚本未执行，无服务/用户/配置/状态上传或部署。** 本次新建的 root-only 空临时目录 `/tmp/remoteapp-device-deploy.kGGw7Gd6` 已使用 rmdir 删除（没有材料被删除）。不得绕过拒绝改用其他传输或间接操作；待用户明确具体目的地与 payload 授权。

部署获准后，仅上传 `server-private`（在线 TLS 私钥/在线 issuer 与空初始状态）、`client-public`、服务二进制/脚本/unit。**禁止上传 `release-local-only` 的发布私钥/CA 私钥**，也不应上传整个 prerelease-v1 目录。首次部署后服务器 Store 是实际 authority，不能用本地初始 snapshot 覆盖服务器最新状态。

`device-service-probe` 仅在可信 SSH forward 下验证真实服务器 TLS/签名配置与 malformed/duplicate/network/health 拒绝；固定 loopback 转发目的地、不跟随重定向、不受系统代理影响，不注册设备/写状态。这是管理通道诊断，不代表公网可达或数据绕开 Tailscale 的最终验收。

## 下一步

回环部署/实际 HTTPS/停止重启验收已通过。随后接服务自身的 verifier 长期身份/故障撤销与 Headscale backend；网络授权仍须保持关闭直到该生命周期通过。然后才评估公网 admission/开放 8444、客户端安装信任持久化与受保护产品凭据投递。不能因为设备 API 启动就开放尚未收口的网络能力。
