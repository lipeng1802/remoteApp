# 双机签名授权联动：待实机验收

2026-10-07：helper 新增独立 authorization 模式，接入 session 门禁及签名状态缓存；提供 `authorized-network.py` 编排。**公网双机授权尚未执行**：本机 SSH agent 没有身份，服务器公钥认证失败；Windows `100.73.4.118:22` 超时。不得将此前 token 中继测试当作本轮签名授权证据。

## 当前实现

私有 Go `grant-fixture` 在真实 invite.Store 中完成设备签名注册、设备码邀请、申请与明确批准，保留状态验证撤销/重启。仅父子 stdin/stdout 管道，stdout 含私钥/授权，只能被编排私有读取，**禁止直接在终端运行、输出响应或接公开 HTTP**。它是有人值守批准的测试替身，不是已完成的用户 GUI/通知系统。

专用 SSH 管理路径读取本轮 Headscale 两个节点的 ID/地址/节点公钥，确认唯一测试用户后提交受控绑定。后端以独立签名域签发包含完整 grant、双方网络节点公钥/固定地址和当前 active 状态的 3 秒快照；不修改或暴露既有 HTTP 注册接口。绑定固定为测试 `.1/.2`，不是通用多租户动态网络策略。该签发函数是可信进程内管理接口，不接受用户自报的网络节点身份；未来正式注册/所有权证明仍待设计。

helper bootstrap 固定可信服务公钥/双方设备公钥/角色私钥及绑定，通过 session 双向挑战验证应用私钥。联网后实际使用本节点 tsnet status 及远端 WhoIs 的节点公钥核对管理绑定；仍检查真实 socket 地址。只要信息不匹配就拒绝连接，不仅比较主机名或用户提供的 IP。PoC 数据走独立 tsnet，自建 DERP 路径在首个授权载荷后核对，SSH/已安装 Tailscale 只是管理路径。

后端状态通过私有控制管道每 0.5 秒刷新；每个快照的有效期为签名中的绝对时间，不以接收时间续期。收到签名撤销后对本 grant 永久拒绝，旧正向快照不能恢复它；新 grant 必须重新 bootstrap。没有刷新、后端停止、到期时门禁检查关闭活动/空闲连接。尚未收到撤销通知时，最近有效快照有最多约 3 秒有效窗口，再加门禁轮询/调度；不是零延迟撤销。两机时钟需同步，未来需设计正式时钟偏差与故障策略。grant 自身仍为 300 秒；本机测试验证 grant 到期，跨网脚本验证的是真实 3 秒状态到期，不能混称同一项。

原 token 模式用于回归与注册测试节点，不与 authorization 模式混用。静态 Headscale ACL 不在本切片变成动态 ACL；注册测试网络允许管理员控制的节点上线，不意味着已批准应用控制权限。安装包、产品 TLS/键鼠和服务器业务保持不变。

## 本机证据

全量 Go 单元/竞态及已有本机 DERP 回归；新增 bootstrap/签名刷新、配置冲突拒绝、快照篡改/绝对到期/撤销重放、重启不复活，以及真实回环活动连接受签名状态撤销/到期关闭测试。私有 fixture 管道已实际执行 bind→lease→revoke→restart→renew，校验同服务公钥/旧授权仍撤销/新 grant 不同，正常关闭及自己的临时状态清理。Windows x64 helper 交叉构建不能代替 Windows 实机测试。

## 恢复条件与复测入口

在 Mac 终端恢复专用 SSH 密钥（如提示口令，在本机输入，不发到聊天）：

```bash
ssh-add /Users/lipeng/.ssh/remoteapp_poc_server_ed25519
ssh-add /Users/lipeng/.ssh/remoteapp_win_ed25519
```

确认 Windows 开机、现有 Tailscale 在线、SSH 可达；不卸载它。检查服务证书续期：现有证书记录为 2026-10-13 到期，运行前必须确认仍有效。编排要求隔离 Headscale 无节点，才重启这一个 PoC 服务重置内存地址游标；不重置数据库/身份密钥，不碰原 Nginx 443。

在仓库根目录使用现有隔离工具链构建（不下载依赖）：

```bash
export GOENV=off GOTOOLCHAIN=local GOPROXY=off
export GOMODCACHE="$PWD/artifacts/connection-poc/gomod"
export GOCACHE="$PWD/artifacts/connection-poc/gocache"
artifacts/connection-poc/toolchain/go/bin/go -C poc/selfhost build -o ../../artifacts/connection-poc/selfhost-node .
artifacts/connection-poc/toolchain/go/bin/go -C poc/selfhost build -o ../../artifacts/connection-poc/grant-fixture ./cmd/grant-fixture
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 artifacts/connection-poc/toolchain/go/bin/go -C poc/selfhost build -o ../../artifacts/connection-poc/selfhost-node-public-win-x64.exe .
python3 -B poc/selfhost/public-server/authorized-network.py
```

预期批准后的真实载荷/自建中继路径、后端撤销活动流关闭、后端/serve 重启后旧授权无载荷、新明确 grant 成功、停止刷新后的状态到期、身份/节点数不增生，以及精确清理测试节点/用户/key/两端状态。必须全部零退出，不将强杀算正常关闭。脚本入口已做语法/私有 fixture 协议检查，实机适配（含 Headscale node_key 输出、WhoIs 及两机时钟）仍须以真实运行校验，不承诺脚本首次执行必过。

编排错误仅输出固定标签，清理逐项尝试并报告清理不完整；若连接在中途丢失，人工复核本轮唯一 `grant-poc-*` 用户，按准确 ID 回收，不全局删除。后续还需第三节点/错误节点绑定实机负向测试、300 秒 grant 实际到期、拒绝请求双机联动、正式注册防滥用/动态 ACL 和产品桥接。
