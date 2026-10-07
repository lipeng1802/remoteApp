# 切片 4B-4A：Windows 桌面 OS 身份桥接入口

这是跨网双机 OS signer 联合入口的前置切片，不是完整联合验收，也不更新旧安装包。

## 为什么需要这个入口

Windows SSH 登录上下文无法创建/读取交互式 Credential Manager 身份。由正常桌面用户启动 `interactive-vault.exe`，显式创建一个随机 PoC 身份并持有 native vault 进程；同一用户的 SSH 网络 helper 通过 `vault-relay.exe` 重开固定公钥。应用私钥仍留在桌面 vault，不发送到 Mac、服务器或 SSH 管道。

桥接仅使用本机随机命名管道，受保护 DACL 只授权当前用户，拒绝远程管道客户端与名称碰撞；不监听 TCP、不开放防火墙。最多四个连接，初始请求/回复有期限；早期包空闲读取为 30 秒，后续 native network 包为有界 10 分钟，以容纳产品构建/独立 API signer 空闲。停止先关闭连接/join，再删除本轮随机测试凭据，不延长网络 gate/grant。只接已固定 scope/public 的重开和既有签名域，不提供 create/delete/export 或任意消息签名。拒绝非法消息不会杀死持有身份的进程。它不抵抗同用户恶意代码，不提供每个调用方独立业务授权；网络授权仍由在线服务和 helper gate 校验。

## 独立 Windows 包

`artifacts/connection-poc/windows-interactive-bridge.zip`，解压后含四份 exe、三份 PowerShell 脚本、SHA256.json 和本文。无 SDK/Go/管理员要求；只允许在正常已登录的桌面 PowerShell 启动持有进程。制品 hash 核对用于完整性/防混包，不等于正式分发签名。不要从旧安装目录拷贝 exe。

第一步在解压目录执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run-interactive-bridge-validation.ps1
```

必须看到 `TestInteractiveVaultBridge` 真正 PASS、无 SKIP，以及 `VALIDATION PASSED`。覆盖原生身份通过 relay 重开/签名，create/错 scope/错公钥/任意消息拒绝、拒绝后身份仍可用、停止时关闭已连接 relay/join/精确删除测试身份。进程级 Bypass 不改变永久执行策略。此步骤会自行停止持有进程，不能当作跨网通过。

第二步（协调 SSH 探针时）在桌面 PowerShell 执行并保持窗口：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\start-interactive-vault.ps1
```

将 `READY` 路径回传。`interactive-bootstrap.json` 只含随机 pipe/scope、公钥和本包 relay 路径，无私钥/验证码。必须是本机操作者信任的包/路径；校验 manifest 并不是不可信文件的真实性保证。启动脚本使用 CreateNew，不覆盖已有 bootstrap；若遗留文件，先确认原进程已经结束，再仅删除该 bootstrap 文件，不删 Credential Manager 命名空间。

中文目录兼容：启动脚本控制管道显式写 UTF-8/no-BOM，不使用 Windows PowerShell 默认 gb2312 的 StandardInput.WriteLine。早期版本在中文目录报 `Interactive vault unavailable`，已修复并通过实际 Windows 子进程字节回归；需使用更新脚本。此回归不是桌面保持启动或 SSH 探针验收。

同一 Windows 用户的 SSH 会话可以执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "实际解压目录\probe-interactive-vault.ps1"
```

必须有 `TestPinnedInteractiveVaultFromBootstrap` PASS、无 SKIP、`PROBE PASSED`。探针重开同一个桌面 OS 身份并验证 HTTPS/enrollment/node/TLS 签名，不创建替代身份或读取 native 私钥。协调结束，在桌面窗口按 Enter，必须出现 `STOPPED`；该启动脚本只删除自己创建的公开 bootstrap 文件，broker 删除本轮随机凭据。不要强杀；若报告 cleanup unconfirmed，不广泛删除任何安全存储。

## 网络 helper 的接入方式与剩余项

可信启动者读取上述本机 bootstrap，仅为子进程设置 `REMOTEAPP_INTERACTIVE_VAULT_PIPE`，将 helper 的 `VaultExecutable` 固定成本包 relay 绝对路径，`VaultScope`/`ExpectedPublic` 固定到 bootstrap；`AppKey` 必须为空。现有 `enrollment-helper` 已支持此配置。不要公开签名桥、不接受网络对端提供这些字段、不静默新建身份。

**尚未改造 `enrollment-network.py --managed` 的 fixture AppKey/authority bootstrap**；完整双机协调器仍须先取得两端 OS 公钥，让 authority 只接公开身份，明确邀请/确认，然后贯穿真实 Headscale 节点持有证明、自主刷新和 Swift/C# TLS。这些尚未实测；本入口的签名成功不是该验收的替代证据。正式信任锚安装/轮换、HMAC 安全投递与 GUI/安装同样待后续收口。

## 本轮结果

Mac 全量 Go race/vet 通过；桥接固定身份/严格解析、禁止操作、四连接上限、取消 blocked reader/join 回归通过。Mac 原生 Keychain 经桥接 handler 的重复固定身份连接/签名通过并精确删除随机凭据（只用测试回环 TCP，Windows 实现无 TCP）。Windows x64 四制品交叉构建与专用 vet 通过。用户对指定的 `run-interactive-bridge-validation.ps1` 回复“验证通过”，Windows 桌面 named pipe/Credential Manager 桥接验收记为用户确认通过；该次未附详细测试输出，不记录耗时或逐项日志。

更新 UTF-8 启动脚本后，用户中文目录桌面截图确认 READY；同用户 jarvis SSH 中实际运行 `probe-interactive-vault.ps1`：`TestPinnedInteractiveVaultFromBootstrap` PASS（0.02s），无 SKIP，`PROBE PASSED`，SSH 退出码 0，两次固定身份重开及四类域签名通过。脚本采用子进程级 ExecutionPolicy Bypass，不改永久策略。此时桌面 owner 仍保持运行，用户按 Enter 后的 STOPPED/该次正常清理回传待确认。完整跨网 Headscale/TLS 验收未完成。

随后用户确认按 Enter 返回 STOPPED，通过该轮正常退出/随机测试身份回收验收。下一入口为 [4B-4B 真实 OS 双机协调](NATIVE_NETWORK_TEST.md)，需新包及新的 READY；旧运行已结束，不复用已删除的 scope/public。
