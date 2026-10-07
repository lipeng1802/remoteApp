# 切片 4A：签名 TLS 身份、安全存储签名与私有 IPC

更新：用户 Windows 正常登录 PowerShell 已完成实际 `TestNativeVaultIdentityAndIsolation` PASS、退出码 0，4A 原生存储人工验收通过。第一次缺少环境变量的 SKIP 不计通过。下方 SSH 失败保留为历史；产品默认 DPAPI 测试仍独立待验。后续 helper 接入见 [4B-1](HELPER_SIGNER_TEST.md)。

2026-10-07 完成私有核心与部分实机验收，**整体切片 4 尚未闭环，不能直接进入 GUI/公网发布**。Windows 安全存储交互式验收、正式安装信任锚/HMAC 投递以及 helper 的全生命周期接入仍待完成。

## 签名 TLS 身份

`publish_tls` 只接受当前批准 grant 的目标设备签名调用，且双方真实网络绑定及最小策略已 ready。目标提交公开 RSA 2048–4096 位服务器证书及证书私钥 PSS/SHA-256 持有证明；证明独立域绑定 grant ID、目标身份、证书 SHA-256 和挑战，并受设备外层时间/nonce/签名覆盖。检查 DER/服务器 EKU/数字签名用途/非 CA/有效期；不让控制方、仅持有 grant 者或只有应用私钥但没有 TLS 私钥者替换证书。

每个 grant 的公开证书最多一次建立，重复相同证书可确认但不同证书拒绝，无静默轮换；只持久化公开证书/ID，512 条和 1 MiB 总快照约束仍在。重开校验历史证书结构/关联授权，但不因正常证书过期拒绝整个历史账本；查询仍检查当前证书有效期、网络 ready 和在线授权。

`tls_identity` 通过设备签名 HTTPS 获取 issuer 签名描述：完整 grant、已证明双方节点/IP、指纹、新鲜设备自生成挑战、绝对起止时间；最多 3 秒且不超 grant 到期。客户端独立预置 issuer、公钥/授权/绑定/挑战/时间全核验；不从返回包信任新 issuer，不能用旧描述跨设备/跨授权/错误绑定/过期恢复连接。描述用于一次连接前确定 pin，活动流继续使用原 grant/状态 watchdog；不是每次加密 PONG 查询描述。

真实 Windows 开发 TLS 服务自己用证书私钥生成证明，Go 服务不拿私钥；Mac 的真实产品 TLS pin 来自已验证描述，不再直接采用父脚本看到的 Windows 指纹。测试 HMAC 密钥仍来自私有开发管道，正式 HMAC/身份包投递未完成。正式 issuer/API TLS 根仍由可信测试 bootstrap 提供，没有安装信任锚分发/轮换或生产部署。

## 安全存储与签名接口

新增 `DeviceSigner`，客户端核验每个返回签名是否匹配固定设备公钥。macOS `ConnectionIdentityVault` 用独立 Keychain generic-password 存 32 字节 Ed25519 seed，WhenUnlockedThisDeviceOnly，LAContext 非交互；Windows `identity-vault` 用当前用户 Credential Manager、命名 mutex 串行创建，清理 Go 中 seed/私钥副本。没有明文文件替代、密码参数或私钥输出。这里“不导出”是辅助协议不返回私钥，不是硬件不可导出/Secure Enclave，也不抵抗同用户恶意程序或 root。

只接受固定签名域的邀请/网络请求，返回公钥/签名。首次创建必须显式 Create=true、Expected 为空；重开必须 Create=false、校验预先保存的 expected public key，丢失/损坏/拒绝/错身份都失败，不自动生成。Keychain atomic add 重复时加载获胜者而不覆盖；Windows mutex 下读/创建。测试 scope 是随机 128 位 hex，命名空间完全独立于现有 GUI 配对密钥；delete 仅创建模式开放供精确测试清理，不删除现有用户凭据。

`device-api-client` 可通过可信初始化的 VaultExecutable/VaultScope/ExpectedPublic 使用上述签名进程，禁止同时传 AppKey。旧 AppKey 分支仅保留 fixture 回归，完整双机 helper 目前仍使用它；**OS 密钥尚未贯穿 nodeproof/helper**。Linux native vault 不支持且拒绝，不伪装成跨平台安全存储完成。helper 路径来自可信本地安装/初始化，不接受远端选择；最终安装签名/哈希和 expected-public 持久元数据仍待产品集成。

## 私有 IPC 与刷新

新增有界 anonymous-pipe codec：8 KiB 单帧、必需换行、严格未知字段/尾随 JSON、嵌套重复键与大小写别名拒绝。接入设备 API 客户端和 Go vault；不是公开 TCP/命名 socket，也不是已经完成所有旧 Swift/C#/网络 helper IPC 审计。签名桥接只用 exec 的已知绝对文件、不用 shell/密钥参数、不转发 stderr，公钥固定且返回签名再验，RPC 3 秒预算，异常关闭管道/只终止自己启动的辅助进程，正常取消等待退出。原始安全存储种子不进入父管道。

设备 API 进程新增串行 `StateWatch` 每 500ms 自己签名 HTTPS 查询，校验 issuer/完整 grant/绑定/绝对状态期限；失败清空缓存/停止，inactive 终止而不自动复活。Stop 取消并 join。实机父脚本仅取缓存再送 helper，不再负责该 watch 的 HTTP 轮询；helper 的绝对租期/管道结束门禁不变。**完整父脚本消除、helper 自主耦合和统一客户端连接关闭还未完成**。

## 验证与复测

- 全量 Go race/vet 通过；Windows vault 交叉构建和专用 GOOS=windows vet 通过。新增证书持有/跨角色/错钥匙/篡改/绑定/挑战/过期/禁止轮换/重开/撤销、watch 刷新/撤销终止/故障清缓存、严格管道测试。
- Mac Swift 126/126，通过 ConnectionIdentityVault Release 构建；两项显式 native Keychain 测试通过，真实 HTTPS 注册和跨进程重开设备码保持，不导出私钥、错公钥/丢失身份拒绝，精确测试删除通过。全量 Go 默认跳 native，不用其替代实机证据。
- 最终 `enrollment-network.py --identity` 零退出，真实设备申请/确认、证书私钥证明/签名描述进入 Swift/C# TLS、自主 watch/cancel、原正负向/周期恢复/Store 故障/撤销/旧状态拒绝/自动回收门禁通过。Windows 产品 SSH profile 65/65，默认 DPAPI 人工待验仍保留。
- Windows native 验证未通过：先定位 PowerShell 将未加引号的 `-test.run` 拆成 `-test`，导致测试没有运行；使用引用参数后真正运行，明确得到 `interactive_logon_required`。Credential Manager 在该 SSH 登录没有可用登录会话，不放宽安全要求或改用明文。测试程序上传 hash 核对、每轮精确临时文件清理完成；失败发生在凭据创建阶段，不能记身份签名/重开成功。
- 双机退出节点/用户/preauthkeys 均 0，独立复核同样全 0，原 file 策略/PoC 策略恢复、业务 Nginx 哈希不变、服务 active。无安装包/系统 Tailscale/业务 443 修改；旧 RemoteController Keychain 密码弹窗不因新增 helper 就被宣称修复。

Mac 显式 native 测试：设置 `REMOTEAPP_VAULT_TEST_EXECUTABLE` 为已构建 ConnectionIdentityVault 的绝对路径，执行 `go -C poc/selfhost test -race ./privatepipe -run TestNativeVault -v`（使用仓库隔离工具链/缓存）。Windows 用当前用户正常交互式 PowerShell，在已构建/核对的两份 exe 所在目录：

```powershell
$env:REMOTEAPP_VAULT_TEST_EXECUTABLE = (Resolve-Path .\identity-vault-win.exe).Path
& .\privatepipe-tests-win.exe '-test.run=TestNativeVaultIdentityAndIsolation' '-test.v=true'
```

仅随机测试命名空间，正常通过后删除该测试凭据；已有产品默认 DPAPI 测试仍需单独运行。两份 Windows exe 是忽略目录下交叉构建产物，不在 Git；构建入口为 `cmd/identity-vault` 和 `go test -c ./privatepipe`。不能让用户运行未核对来源的 helper。

下一步仍是切片 4B：Windows 交互式 native 验收；正式 issuer/API TLS 信任锚及 HMAC/身份包可信分发；OS signer 替换 nodeproof/helper 的测试私钥入口和全链路安全 IPC/连接生命周期，再考虑切片 5 GUI。当前不公开部署 fixture。
