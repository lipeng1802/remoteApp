# 双机 Tailscale 输入 Mock 验证

本检查只验证 Mac→Windows 的 TLS、证书固定、HMAC 认证、输入顺序和释放。Windows 使用内存 sink，不调用 `SendInput`，不会移动鼠标、点击或键入内容。工具为单次会话，最长等待 5 分钟，结束后关闭端口。

## 安全边界

- Windows 只绑定本机唯一的 `100.64.0.0/10` Tailscale IPv4，并只接受唯一在线 Mac 的 Tailscale IPv4。
- PowerShell 必须显式传入 `-AllowLocalMock`；底层工具还必须收到 `--allow-local-mock`。
- Mac 只接受字面量 `100.64.0.0/10` IPv4，不接受主机名、回环、局域网或公网地址。
- Mac 必须已有该地址对应的 Keychain 配对密钥和已批准证书指纹；本工具不能首次配对或批准新证书。
- 双端使用共享 `controller-input-v1.json` 对 12 个合成事件逐项校验，包括左右 Ctrl、A 重复按下、移动、左键、滚轮以及全部释放。
- 不修改防火墙，不启动产品 GUI，不启用原生输入注入。

## 1. Windows 拉取、构建和测试

在仓库根目录的 PowerShell 中执行：

```powershell
git pull
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
```

预期解决方案构建为 0 警告、0 错误；协议测试预期 **60/60**。Mac 当前没有 .NET SDK，因此该结果必须由 Windows 实际记录，不能以 Mac 检查代替。

## 2. Windows 启动一次性 mock

先关闭 RemoteAgent、“只读共享”和旧 TLS 探针，确保 47475 没有其他监听。然后执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\p2\Start-TailscaleInputMock.ps1 -AllowLocalMock
```

看到以下三类输出后保持窗口开启：

```text
READY input-mock port=47475 wait_seconds=300 expected_events=12
CERTIFICATE_SHA256 ...
NO_NATIVE_INPUT memory sink only
```

脚本自动选择唯一在线 Mac，不需要手工输入任一真实地址，也不要把证书指纹、设备密钥或完整 Tailscale 状态发到聊天中。

## 3. Mac 发送合成序列

在 `macos/RemoteController` 目录运行；地址必须与此前配对和批准证书时使用的 Windows Tailscale IPv4 完全一致：

```bash
swift run -c release TailscaleInputMockClient <Windows-Tailscale-IPv4>
```

Mac 预期：

```text
AUTHENTICATED input mock; sending 12 synthetic events
PASS synthetic input sent and release frames drained; no native input requested
```

Windows 随后预期：

```text
PASS input mock authenticated; 12 synthetic events verified; all held input released
CLOSED temporary input mock listener
```

Windows 桌面在整个过程中不应出现真实鼠标或键盘动作。若任一端失败，只记录固定的 `FAIL ...` 行和测试计数；不要发送密钥、完整 Tailscale JSON 或证书私钥。不要自行放宽地址限制或添加防火墙规则。

## 通过条件

只有双方 PASS、Windows 报告 12 个事件顺序匹配、全部释放且桌面没有真实动作，才记为双机 input mock 通过。这仍不代表产品 GUI 或 `SendInput` 已启用。
