# 签名授权到固定载荷连接的门禁

2026-10-07：独立 `session` 包在真实本机 TCP 连接上接入 `invite.Store`，验证签名授权不只是后端记录，而是实际限制数据流。没有接入双机 tsnet helper、Headscale 注册/动态 ACL、HTTP 远程授权查询或产品安装包，也没有修改服务器。

## 约束

调用方通过可信管理通道提供服务公钥、双方应用公钥、当前 grant 和预期网络地址；不能接受连接方自行声明这些绑定。双方验证连接的实际本地/远端地址，并核对应用私钥与指定角色。握手双方生成独立 256 位随机 nonce，签名覆盖完整 grant、地址、双方 nonce 和版本；目标/控制端使用不同签名域，避免反射和跨连接重放。控制端核对目标签名与完整预期 transcript 后才发持有证明。长度帧最多 2048 字节，拒绝未知字段、尾随 JSON 和畸形 nonce。

被控端在握手前及每次载荷响应前检查当前在线授权；握手完成后每 250ms 轮询，授权到期、撤销、存储停止服务、时间回退或查询不可用会取消上下文并关闭空闲/活动连接。一次授权查询最多等待 500ms；正常调度下空闲连接的撤销检测窗口约为轮询间隔加查询超时，并非零延迟撤销。握手 I/O 最多 3 秒，未完成握手也不能得到载荷。检查函数必须响应 context 取消；模块不能强杀不遵守约定的回调 goroutine。真实远程查询适配器及其认证、超时、恢复尚待实现。

`Probe` 只请求固定字符串 `remoteapp-authorized-poc-v1`，没有屏幕、键鼠或用户数据。**这个握手不是加密协议**：调用方必须提供可信加密传输（未来通过独立 tsnet 或产品 TLS）。本轮 loopback 明文 TCP 仅用于测试门禁，不能作为公网安全方案。IP 与公钥配置匹配也不证明 Headscale 节点所有权；可信节点注册、节点身份绑定凭据与受限网络策略尚未实现。配置及公钥切片在运行期间应保持不可变。

## 验证与下一切片

仓库根目录运行：

```bash
GOENV=off GOTOOLCHAIN=local GOPROXY=off \
GOMODCACHE="$PWD/artifacts/connection-poc/gomod" \
GOCACHE="$PWD/artifacts/connection-poc/gocache" \
artifacts/connection-poc/toolchain/go/bin/go -C poc/selfhost test -race ./session
```

16 项顶层测试涵盖批准后互证与载荷、取消、活动撤销与后端重启、空闲到期、后端不可用/关闭、在线拒绝、服务公钥/权限/身份/到期篡改、真实地址/错误本机私钥、第三设备/签名域、跨连接证明重放、帧边界、pending/拒绝不恢复旧授权、伪造目标/transcript、查询超时/缺少检查器、重新明确授权只允许新 grant、时钟回退。依赖和旧固定 token helper 未修改；原本机 DERP 回归独立运行。

本机实测 `go test -race -count=3 ./session` 三轮通过；最终版全量 `go test -race ./...` 与 `go vet ./...` 通过，原 `GOPROXY=off zsh poc/selfhost/verify.zsh` 完整回归零退出。没有新增 Windows 实机、公网双机授权或无缝路径迁移证据。

下一切片把本模块接入隔离双机 helper：受控下发服务公钥与设备/网络节点绑定，使用有认证且有界超时的当前授权查询，再在自建网络验证批准通行、拒绝/到期/撤销关闭、重启不复活。不能用父脚本先查一次 Authorized 然后放行原 token 流，冒充端到端授权闭环；也不能将此处本机 TCP 结果记为公网联动验收。
