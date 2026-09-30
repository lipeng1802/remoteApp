# Windows 原生输入边界验证

## 本切片范围

新增独立 `WindowsInput` 项目，把已经通过认证、能力协商和 Windows 本机显式许可的协议输入转换为 Win32 `SendInput`。当前产品 GUI 和 TLS 视频会话**尚未接线**，因此 RemoteAgent 仍显示并保持“只读共享”；运行下方自动测试不会移动鼠标、按键或调用真实 `SendInput`。

原生边界使用扫描码键盘事件，保留左右扩展键标志；鼠标移动使用协议的 0...65535 绝对坐标；左右/中键和横向/纵向滚轮分别映射。sink 自己保留一份已按下状态，释放时逐项尝试，即使其中一项失败也继续释放其他项，并允许之后重试失败项。Win32 返回值不是 1 时以错误退出，不把失败事件记作已成功注入。

## Windows 验证命令

在仓库根目录执行：

~~~powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\WindowsInput.Tests\WindowsInput.Tests.csproj -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
~~~

预期结果：

- solution 构建 0 警告、0 错误；
- `WindowsInput.Tests` 显示 `6/6 tests passed`；
- `RemoteProtocol.Tests` 保持 `60/60 tests passed`；
- 测试期间 Windows 桌面没有鼠标移动、点击或键盘输入；
- 启动现有 RemoteAgent 时仍显示“只读共享”，没有控制授权入口。

如任一数量不同或桌面发生真实输入，立即停止并保留完整错误文字；不要先接产品 GUI。

## 尚未完成

这不是远程控制验收。下一切片需要把视频发送和输入接收合并为同一条 TLS/HMAC 认证后的双向会话，保证单一帧序号和串行写入；之后才加入 Windows“仅本次允许控制”开关、可见控制状态，以及 Mac 查看器的聚焦/失焦输入生命周期。默认仍须只读，停止、失焦、断线、协议错误和注入错误都必须释放已持有键鼠。
