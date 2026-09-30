# Windows 真实输入受控验收

日期：2026-09-30。本切片首次从产品 RemoteController 向产品 RemoteAgent 执行真实 Win32 `SendInput`。请先保存 Windows 上所有工作，关闭敏感或可能造成数据损失的应用，只在记事本和空白桌面中测试。

Mac RemoteController 当前默认填写 Windows Tailscale IP `100.73.4.118`；地址框仍可编辑，若 Windows 的 Tailscale IP 变化，请以 Windows 客户端显示的地址为准。

## 实现边界

- 默认只读；Windows 每次共享都要重新勾选“允许本次远程控制”，并在警告框再次确认。
- Mac 也要勾选控制请求；TLS 证书指纹、设备密钥和 HMAC 认证成功后，仍需点击“开始控制”。
- Windows 原生 sink 只在上述门禁全部通过后创建。状态栏只记录数量，不记录坐标、扫描码或输入内容。
- Esc、Mac“停止控制”、Mac 窗口/应用失焦会发送释放并暂停输入捕获，但保持远程画面和认证连接；返回后只需再次点击“开始控制”，无需重新连接。只有主动点击“断开”才会先排空释放再发送 DISCONNECT；异常断线由 Windows 服务端兜底释放。
- 会话结束后 Windows 许可自动清空。Win32 `SendInput` 受 UIPI 限制，不保证控制管理员权限窗口、UAC 安全桌面、锁屏界面或其他更高完整性目标；本轮不要尝试绕过这些限制。
- Windows 注册全局 `Ctrl + Alt + Esc` 作为本机紧急停止。注册失败时真实控制必须拒绝启动，但只读共享应继续可用；关闭 RemoteAgent 后快捷键必须释放给系统。

## Windows 自动检查

在仓库根目录 PowerShell 中执行：

~~~powershell
git status --short
git log -1 --oneline
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
dotnet run --project .\windows\RemoteAgent\tests\WindowsInput.Tests\WindowsInput.Tests.csproj -c Release
~~~

预期 Release 0 错误；协议 61/61；WindowsInput fake API 6/6。自动测试不会调用真实 `SendInput`。若数量因后续新增测试变化，以实际 0 failures 为准并记录数量。

启动最新源码版，不要使用旧安装快捷方式或旧 artifacts：

~~~powershell
dotnet run --project .\windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj -c Release
~~~

## 安全人工顺序

1. 首先不勾选 Windows 许可，开始共享；Mac 不勾控制请求并连接，确认只读画面正常。结束后确认 Windows 复选框仍为未选中。
2. Windows 勾选许可并点击开始。在本机警告框选择“否”，确认没有开始真实控制且复选框被清空。
3. 再次勾选并在警告框选择“是”。Mac 勾选“请求远程控制”后连接；画面出现但尚未点击“开始控制”时，Mac 键鼠不得操作 Windows。
4. **通过（2026-09-30）**。Windows 打开一个空白记事本并确保没有未保存的重要内容。Mac 点击“开始控制”，先轻微移动鼠标，再测试左/右/中键、拖动、纵横滚轮；动作必须落在预期坐标，黑边操作不得注入。触控板连续双指纵向、横向滚动均正常且不再断开，停止滚动后仍保持连接，最终“持有 0”。
5. 在空白记事本输入少量 ASCII 字母、数字、空格、退格和 Enter；再测试左右 Shift/Control/Option/Command。Command 映射 Windows 键、Option 映射 Alt、Control 映射 Ctrl，不测试输入法、Caps Lock、Fn、媒体键或 Pause。
6. 分别测试 Control+A、Control+C、Control+V 等无破坏性组合；不要测试关机、删除文件、系统管理或安全桌面快捷键。
7. **通过（2026-09-30）**。按住普通键、修饰键或鼠标按钮时分别执行 Mac Esc、Mac“停止控制”、切换 Mac 应用/窗口失焦、Mac 主动断开，四种路径最终均为“持有 0”，停止后 Mac 输入不再影响 Windows。Esc、停止和失焦只暂停控制并保持会话；返回后再次点击“开始控制”即可。主动断开才结束会话并要求重新连接。
8. **通过（2026-09-30）**。再次控制并按住一个无破坏性的普通键或修饰键，从 Windows 的物理键盘按 `Ctrl + Alt + Esc`；本机强制停止按预期生效，共享结束、最终“持有 0”、Mac 断开，之后的 Mac 输入不再影响 Windows。
9. **通过（2026-09-30）**。再次控制时由 Windows 点击“停止共享”，输入立即停止、最终“持有 0”且 Mac 自动断开。重新开始共享时控制许可已自动取消，未重新授权不能控制 Windows。
10. 最后重新建立一次双方均未授权的只读会话，确认画面和停止/重连正常。关闭 RemoteAgent 后重新启动，确认没有“紧急停止快捷键不可用”提示。

若出现鼠标持续移动、键或按钮未释放，立即在 Mac 按 Esc，并在 Windows 点击“停止共享”或关闭 RemoteAgent；记录双方状态文字和最后操作，不继续测试。若 `SendInput` 返回错误，会话应失败关闭并尝试释放，不能静默继续。

## PASS 标准与下一步

只有自动检查和 1–10 全部通过，且所有停止路径最终持有 0、Windows 本机紧急停止有效、下一次共享重新授权、未授权会话始终只读，才可记录 P2 真实键鼠 MVP 通过。此后再评估安装包重建、不同 DPI/分辨率、布局/IME、UIPI 限制及长时间稳定性；本切片不重打安装包。
