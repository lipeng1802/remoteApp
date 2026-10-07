# 切片 4B-1：网络 helper 的 OS 签名器接入

更新：用户使用本轮两份 `-4b.exe` 新制品，在正常 Windows PowerShell 实际执行测试，PASS（0.35s）且退出码 0。新增域的 Windows native 人工验收通过；未扩张为双机 OS signer 验收。下一子切片见 [4B-2](MANAGED_HELPER_TEST.md)。

本轮是 4B 子切片，不是完整最终连接架构交付。4A Windows 原生存储已由用户交互式截图验收 PASS/退出码 0；旧 SSH 缺登录会话不再是这项人工验收的阻塞。

## 实现与边界

- `enrollment-helper` 的 prepare/join 可用可信本地绝对路径 VaultExecutable、随机测试 VaultScope、预先固定的 ExpectedPublic 重开 OS 身份。禁止 AppKey 同时配置、禁止缺少 pin、禁止孤立 vault 参数；这里不负责首次创建或保存 public 元数据。
- 注册意图、nodeproof 在线持有证明、session 两角色和产品 TLS 两角色签名都经 `DeviceSigner`，每次核验返回签名和预期公钥；旧内存私钥仅保留 fixture 回归。Config.Key/Signer 不能混用。
- Mac/Windows vault 只新增精确注册/持有/会话签名域，不能签任意域，不返回 seed。签名域限制不是抵抗同用户恶意程序的业务授权隔离。
- helper 继承管道接入严格 codec。网络地址/WhoIs/随机挑战、在线授权/绝对租期/watchdog 和 TLS 独立域门禁保留；正常退出先取消网络/等待 worker，再关闭 signer。

## 验证记录

最终完整 Go `test -race ./...` 通过（默认明确跳过 native）；新增提供者失败/错误身份/混合私钥拒绝、真实本机会话与撤销、注册意图字节兼容、精确域拒绝、helper 配置负向覆盖。早期新增撤销测试误用 RequestID，修正为协议要求的 GrantID 后回归通过，未放宽服务权限。

Windows x64 vault/helper/原生测试交叉构建及专用 vet 通过；这不是本轮 Windows 实机通过证据。Mac Release、Swift 126/126、Go vet 通过。原生测试使用真实 Keychain 辅助进程，不能换内存实现。并行首次运行出现 3 秒创建超时，未计为通过；开发二进制施加本地 ad-hoc 签名（非公证/正式签名），保留原 IPC 时限并串行复测。最终三项 native 测试全部通过：HTTPS 注册/重开、隔离/域签名、本机 TCP 会话和 opaque 产品隧道/到期关闭。成功轮次随机凭据精确清理；早期超时是否已写入随机测试项无法确认，不通过广泛删除命名空间处理。旧生产凭据未操作。

未运行新的真实 Headscale/Windows 双机 OS signer 场景：Python 双机 runner 仍用 AppKey fixture。正式信任锚及 HMAC 投递、helper 自主查询/API/退出生命周期整合、生产签名/安装包仍待完成。已安装产品、生产配对密钥、系统 Tailscale、业务 Nginx/防火墙未改。

## Windows 新签名域人工复测（本轮新制品）

两份制品不在 Git，需要独立复制；使用 `identity-vault-win-4b.exe` 和 `privatepipe-tests-win-4b.exe`，不要混用上轮二进制。在 Windows 正常登录会话同一文件夹中：

本轮制品 SHA-256：vault `0ed81922184f66a2b7045aa4eb4814edd0ad7b7915d73fa3387d4ee8ef67af8a`；tests `7ee6ca4d5f41b4fd4e088b34798cc0811ff251789e838e5b01d720e045b01f49`。

```powershell
$env:REMOTEAPP_VAULT_TEST_EXECUTABLE = (Resolve-Path .\identity-vault-win-4b.exe).Path
& .\privatepipe-tests-win-4b.exe '-test.run=TestNativeVaultIdentityAndIsolation' '-test.v=true'
$LASTEXITCODE
```

要求测试行 PASS、非 SKIP，退出码 0。仅随机独立测试身份，成功后精确删除；无网络或产品控制。它证明新增六个域的 OS 签名/验证能力，不证明真实节点 WhoIs 或完整跨网会话。
