# 产品 GUI 控制 mock 交接

日期：2026-09-30。本切片验证真正的 RemoteAgent/RemoteController 窗口、同一条 TLS/HMAC 连接和真实 Mac 窗口输入，但 Windows 端仅统计收到的事件，不执行系统输入。它不是 `SendInput` 验收。

## 首轮结果与复测重点

首轮双机测试中连接、普通输入、黑边以外操作和只读回归通过，但 Windows 状态栏对 Shift/Control 的短促持有变化未可靠刷新，Esc/停止/断开后可能残留显示 1–2。原因是内存统计 sink 的一秒 UI 限流遮蔽了最后一次状态变化，并非协议或系统输入仍被持有。

修复版在键/按钮持有集合每次变化时立即更新 UI，移动、滚轮及重复 Down 才继续按一秒限流。用户已完成复测并确认全部通过：Shift、Control、Command 的持有变化正确，逐个松开以及三者同时按住后的 Esc、停止、失焦和断开最终均显示 0。

## 最终验收结果

用户确认下方完整清单全部通过：自动构建/测试、双端显式许可、认证后手动开始、普通键鼠、左右修饰键、黑边、拖出边界、滚轮、Esc、停止、失焦、断开、暂停后手动恢复及默认只读回归均符合预期。Windows 只显示统计，桌面没有真实输入动作。因此本文件对应的产品 GUI 内存 sink 双机检查点已完成；此结论不包含真实 `SendInput`。

## 安全边界

- Windows 控制许可默认关闭，并在每次共享开始时锁定；下一次共享需要重新确认。
- 未许可时服务端不声明/接受输入能力，行为保持只读。
- 许可时只创建 `SessionInputAuditSink`；只显示事件、释放、持有数量，不记录坐标、扫描码或按键内容，也不创建 `WindowsInputSink`。
- Mac 控制请求默认关闭；认证成功后仍须点击“开始控制”。首次使用的新证书只能先通过只读连接核对并保存，控制模式不提供自动信任入口。
- Esc、停止控制、窗口或应用失焦均释放并暂停，不能自动恢复。主动断开先发送释放，再发送 DISCONNECT；异常退出由服务端统一释放。

## 已完成的 Mac 自动验证

在 `macos/RemoteController`：

~~~bash
swift test --disable-sandbox
swift build -c release --disable-sandbox
~~~

实际结果：全量 109/109、0 failures；Release `Build complete! (6.54s)`。最终重跑没有 Swift 并发隔离告警。测试包含左右修饰键、黑边、拖出边界、滚轮、失焦/停止释放、队列/认证调度和真实 loopback TLS 双向 JPEG + 输入；它们不能替代下方双机 GUI 人工操作。

## Windows 接收后自动检查

在仓库根目录的 PowerShell 执行：

~~~powershell
git status --short
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
dotnet run --project .\windows\RemoteAgent\tests\WindowsInput.Tests\WindowsInput.Tests.csproj -c Release
~~~

预期：Release 构建 0 错误；协议 61/61；WindowsInput 6/6。若数量因后续提交变化，以 0 failures 和实际日志为准并记录新数量。本轮没有新增 Windows 自动测试，因为 GUI/WPF 和真实跨机事件必须人工覆盖。

## 双机 Tailscale GUI 验收

先确保两端 Tailscale 在线，Mac 已为同一 Windows 地址保存配对密钥和已核对的证书指纹。

1. Windows 启动最新 RemoteAgent，勾选“允许本次远程控制测试（仅统计输入，不注入 Windows）”，再点击“开始共享”。
2. Mac 启动最新 RemoteController，填写 Windows Tailscale IPv4，勾选“请求远程控制测试”，点击“连接”。认证并出现画面后，“开始控制”才应可用。
3. 点击“开始控制”，依次测试普通键、左右 Shift/Control/Option/Command、鼠标移动和三键、拖出画面再释放、黑边、纵横滚轮。Windows 的事件数应增长，所有释放后“持有”为 0；Windows 桌面本身不得移动鼠标、输入字符或执行快捷键。
4. 黑边按下不得建立持有；画面内按下后拖到黑边/窗口外再松开，最终持有必须为 0。
5. 按住键/鼠标时按 Esc、点击“停止控制”、切换到其他应用或让窗口失焦，Windows 最终都应显示持有 0。Mac 返回后不得自动恢复，必须再次点击“开始控制”；视频应继续显示。
6. 再次开始控制，按住输入时点击“断开”。Mac 应先显示释放/断开状态，Windows 最终持有 0 并结束该次会话。
7. 双端重新开始，但这次 Windows 不勾许可、Mac 不勾控制请求。只读画面应正常，双方不出现可用控制入口，作为回归验证。

PASS 必须同时满足：自动检查全部通过；控制需两端显式选择且认证后手动开始；事件能跨网到达统计 sink；所有停止/失焦/断线路径最终持有 0；Windows 桌面没有任何真实键鼠动作；默认只读回归正常。

## 下一切片

上述双机验收已经通过。下一切片可把 Windows 本次许可分支从内存统计 sink 切换到已有的 `WindowsInputSink`，并进行可立即停止的真实桌面人工验收。真实注入切片仍需保留默认只读、每次会话许可、可见控制状态、失焦/停止/断线释放和失败关闭；不得把本 mock 的“事件已到达”记录成桌面控制已通过。

## Windows 实测启动入口（2026-09-30）

Windows Release、61/61 协议及 6/6 fake 输入测试已实际通过。独立 self-contained 发布目录为 artifacts/windows/gui-control-mock-e91db1d，运行其中 RemoteAgent.exe（保留整个目录）。已通过 UI Automation 确认控制许可复选框可见、启用、默认未选中。更新 Git 不会更新旧安装快捷方式或旧 artifacts 可执行文件；该目录随后用于双机 GUI 验收，并在拉取统计刷新修复后完成全部复测。
