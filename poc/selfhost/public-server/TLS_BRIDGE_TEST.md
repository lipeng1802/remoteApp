# 真实产品 TLS 桥接（独立开发验收）

此切片复用真实 Swift `TLSControllerClient` 和 C# `TlsProbeServer`，不是用 Go 模拟 TLS/framing。仅开发固定 8 字节 PING/PONG；不构造 JPEG、输入、剪贴板能力，不进入 GUI/安装包。

数据链路为：Swift TLS → Mac 回环临时端口 → controller helper → 独立 tsnet/Headscale/DERP → target helper → Windows `127.0.0.1:47478` → C# TLS。Go 只转发不透明密文，不解密、不保存产品密钥，目标端口固定且不可由网络方指定。没有新增公网端口或修改防火墙/业务 Nginx。

桥接前核对签名动态绑定、实际 Self/IP、WhoIs 节点及双方新鲜 Ed25519 证明。TLS 使用独立 `product-tls-target/controller` 签名域，旧固定载荷证明不能打开产品上游；激活后不能退回旧 token 流。建立后两端 250ms watchdog 核验当前授权、3 秒绝对签名状态租期和 grant 有效期；父脚本每 500ms 刷新。Go 不检查加密 PONG 内容，也不声称能逐帧查询授权。失效关闭两向 socket，并等待两向 copy worker 退出；没有自动重拨。

## 可信初始化与限制

Windows 开发进程临时生成证书、32 字节应用 HMAC 密钥和身份，仅通过捕获的私有 SSH 管道传给测试父进程。Swift 内存 pin 精确绑定当前签名 grant 的 target subject，首次使用不自动信任、没有“接受任意证书”，不触碰钥匙串或生产 DPAPI 凭据。私有初始化将本轮 TLS 实例与已批准 Windows 角色绑定；它**不是**公开 issuer 签名的 TLS 证书身份分发协议，也不是最终用户设备码流程。

现有 GUI 仍使用系统 Tailscale。公开注册/认证状态服务、证书与设备身份分发、安全本机 IPC/私钥存储、周期 Reconcile/故障恢复、GUI/安装集成仍待实现。管理 SSH 使用既有系统 Tailscale；实际数据用独立 tsnet。此处不能据此宣布最终免账号产品完成。

## 复测

按 [注册验收](ENROLLMENT_TEST.md) 准备固定版本 Go/Headscale 与三个 enrollment 程序，并重新编译两个 helper（本轮有变化）。将 Mac `TLSBridgeProbe` Release 构建完成后运行：

```sh
swift build --package-path macos/RemoteController -c release --product TLSBridgeProbe
python3 -B poc/selfhost/public-server/enrollment-network.py --tls
```

编排会上传纯源码到 Windows 本轮独立目录，运行协议测试并发布自包含 win-x64 开发探针；不覆盖 Windows 仓库或已安装程序。测试 stdout 仅计数/固定结果，不能手工运行探针并公开含密钥的 ready/bootstrap。测试结束精确回收私有节点状态，恢复原 file 策略、检查资源和业务 Nginx 哈希。请勿与其他会修改专用 Headscale 的实验并行。

Windows SSH 登录没有可用 DPAPI logon session，默认协议套件的凭据持久化测试因此失败。`--bridge-validation` 是明确的开发测试配置，只跳过该项并打印 SKIP，65 项协议测试运行；默认无参数仍要求全部 66 项，生产 DPAPI 不改动。仍需 Windows 交互式登录执行默认套件验证该项，不能把 SSH 的 65/65 写成 66/66。

## 验收要求

- 原真实注册/实时持有证明、单向最小 ACL、零策略拒绝及正向恢复仍通过。
- 错误证书指纹立即明确拒绝；错误 HMAC 返回认证拒绝；认证前 PING 在真实 C# gate 得到 AuthRequired。三者双方保护 PONG 增量为零，不以拨号超时作为通过。
- 正确连接用真实产品 TLS、framing/序号、HMAC 认证得到连续 PONG，保持同一连接超过初始状态租期。
- 停止刷新、真实 Store 关闭但 ACL 保留、签名 inactive 撤销但节点/ACL 保留，均关闭实际 TLS 流，两端不再产生 PONG；不能用客户端 10/15 秒超时替代 gate 关闭。
- Store 重开/Reconcile、显式 helper 重启保持身份并恢复 TLS。有效旧正向状态在 inactive 后仍被拒绝；随后回收资源，旧 grant 不复活。
- 每个 helper/产品探针正常关闭、零退出；强杀只可用于失败清理，不作为成功证据。

实机发现 NWConnection 在证书拒绝后不一定立即进入 failed，曾导致客户端等启动超时。本轮在真实 verify 回调拒绝后明确完成并取消连接，且保持 verify options 在连接创建之前配置；错误指纹通过实时负向验收。另排除 Mac tar 的 `._*.cs` 元数据文件，避免 Windows 编译器把 AppleDouble 当源码。
