# TLS 内应用认证验收

当前实现是命令行安全切片，未接入 WPF/SwiftUI、屏幕或输入。旧固定字符串探针已替换；两端必须使用本轮相同代码。原有 v1 HELLO/HMAC/PING 格式不变，共享 golden vectors 仅供测试。

## 1. 构建与自动测试

Windows 仓库根目录（PowerShell）：

```powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
```

本机已验证：0 警告/错误，`25/25 tests passed`。测试只开放临时回环端口；凭据测试使用随机测试名称并在 finally 删除，不读写真实配对密钥。

将本轮代码同步到 Mac 后，在 Mac 仓库根目录运行：

```bash
cd macos/RemoteController
swift test
swift build -c release
```

预期：`28 tests, 0 failures`、`Build complete!`。新增 9 项测试覆盖完整认证状态、错误认证结果、认证前 PONG、重复挑战、错误序号、拆包/粘包、超限头部、错误 PONG、隔离 Keychain 密钥增删改查。Keychain 测试使用随机 service/account，结束后删除。当前 Windows 无法执行这些 Swift/macOS 测试，28 是预期数量，不是已验证结果。

## 2. 首次配对（不需要开监听）

Windows 在自己打开的交互式 PowerShell 中执行：

```powershell
.\scripts\p1\Start-TailscaleTlsProbe.ps1 -ShowPairing
```

首次生成 32 字节随机设备密钥与 16 字节稳定 Agent 标识，保存到当前用户 Windows Credential Manager（target `PersonalRemoteDesktop/Agent/v1`）；以后复用。命令仅在交互终端显示 Base64 密钥，拒绝重定向输入/输出；普通服务日志不显示密钥。不要把密钥粘贴到聊天、仓库、日志或命令参数。

Mac 继续在 `macos/RemoteController` 目录，输入 Windows 的 Tailscale 地址（沿用此前批准证书时的同一地址字符串）：

```bash
printf 'Windows Tailscale IP: '
read -r WIN_TS_IP
swift run -c release TLSProbeClient --pair "$WIN_TS_IP"
```

在隐藏输入提示里粘贴 Windows 显示的密钥并回车；预期 `PAIRED device key stored in Keychain; TLS certificate approval remains separate`。这里不连接服务器。密钥存入独立 Keychain service `com.personalremotedesktop.controller.device-key`，设备地址作为 account；不会修改已经固定的证书指纹。重跑配对会更新该地址的设备密钥。需换地址时应明确重新配对/核对证书，不能自动继承其他地址的信任。

## 3. 不同网络下的三轮验收

保持 Windows 原网络、Mac 手机热点。**每轮都先在 Windows 重启一次**：

```powershell
.\scripts\p1\Start-TailscaleTlsProbe.ps1
```

等待 `READY tls-auth` 和 `CERTIFICATE_SHA256`。脚本只绑定本机 Tailscale IPv4，并限定唯一在线 Mac 的源地址；总等待最多 300 秒，TLS 建立后应用握手和 PING/PONG 总期限 20 秒。不需防火墙更改。

Mac 分别执行（每条对应一次新 Windows 监听）：

```bash
# 第一轮：正确密钥
swift run -c release TLSProbeClient "$WIN_TS_IP"
# 第二轮：临时翻转内存密钥的一位，不修改 Keychain
swift run -c release TLSProbeClient --wrong-key "$WIN_TS_IP"
# 第三轮：刻意在认证前发送 PING
swift run -c release TLSProbeClient --preauth "$WIN_TS_IP"
```

预期：

| 轮次 | Mac | Windows |
|---|---|---|
| 正确密钥 | `PASS TLS handshake, stored fingerprint policy, application authentication, and PING/PONG`，退出 0 | `PASS TLS application authentication; protected PONG sent`，退出 0 |
| 错误密钥 | `FAIL TLS probe: authenticationRejected`，退出 1 | `FAIL TLS probe stopped (AuthenticationException)`，退出 1 |
| 认证前 PING | `FAIL TLS probe`，退出 1（可能为 unexpectedResponse 或 connectionFailed） | `FAIL TLS protocol rejected (AuthRequired)`，退出 1 |

负向验收的 FAIL 是预期拒绝，PowerShell 包装脚本随后抛出 `TLS probe server failed` 也属预期。每轮 Windows 都应打印 `CLOSED temporary TLS listener`；不能仅凭 Mac 连接失败认定认证门禁通过，必须核对对应 Windows 拒绝原因。可再重启一次并运行正确密钥命令，确认负向测试没有改坏存储。

已有相同地址的证书指纹应自动匹配。首次使用新地址时必须人工核对两端 SHA-256 后才输入 `y`；指纹变化应拒绝，勿删除原证书来重测。

## 4. 实现边界与后续

- 双方每次会话生成新的 32 字节安全随机 nonce；Windows 另生成新的 32 字节 challenge。HMAC-SHA256 绑定双方 nonce、challenge 和 Agent 标识，恒定时间比较；旧响应不可复用。
- TLS 成功后双方发 HELLO，Agent 发 AUTH_CHALLENGE，Controller 发 AUTH_RESPONSE，Agent 发 AUTH_RESULT。仅 Success 后 Controller 发随机 8 字节 PING，Agent 回同一载荷 PONG；不发真实屏幕或输入。
- 探针额外限制每帧载荷最多 64 字节，在完整帧头到达时检查，独立方向序号从 1 开始。此限制只适用于探针，未改变协议的全局视频/控制消息限额。
- 错误密钥延迟 1 秒后发 Rejected（retryDelayMilliseconds=1000）并关闭；每进程只接收一次连接，没有后台自动重连。未来常驻服务还需跨连接失败计数/退避，不能把当前延迟当成完整 P3 限速。
- 本轮 Windows 自动测试已覆盖重放、半帧断开、超限头、序号、期限与密钥复用。真实 Mac Keychain、Swift 编译与上述三轮跨网络认证仍需实机验收。
- 旧 Setup.exe/DMG 未重建，不包含此命令行认证切片；无预装 .NET 的安装验收独立保留。

Windows 存储使用 [CredWriteW](https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credwritew) 和 [CREDENTIALW 的本机持久化语义](https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw)。