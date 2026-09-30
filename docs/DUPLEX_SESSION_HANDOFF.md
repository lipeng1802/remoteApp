# JPEG + 输入单连接双向会话验证

## 本切片完成内容

Windows `TlsProbeServer.RunOnceAsync` 现在可以在调用方同时提供 JPEG source 和显式允许的 input session。缺少本机许可会在建立监听前拒绝；握手双方必须同时声明 `Jpeg | Input`，HMAC 认证成功前不会创建采集源或输入 sink。

认证后的连接遵循一个读取者、一个写入者：读取循环解析输入、心跳和断开；JPEG、SCREEN_INFO 与 PONG 统一由单一写循环发送并共享一条连续序号。PONG 队列固定 16 项，满时关闭会话，不允许无限堆积。视频写入仍有帧期限，输入仍有读取期限、速率限制和统一释放。任一方向失败或本机取消都会取消另一方向。

本轮没有把该入口接到 WPF RemoteAgent，也没有创建 `WindowsInputSink`；现有 GUI 仍应显示只读共享，自动测试使用内存 sink，不会移动真实鼠标或产生真实按键。

## Windows 验证

在仓库根目录执行：

~~~powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
dotnet run --project .\windows\RemoteAgent\tests\WindowsInput.Tests\WindowsInput.Tests.csproj -c Release
~~~

预期：

- Release 构建 0 警告、0 错误；
- 协议测试 `61/61 tests passed`；
- 原生输入边界保持 `6/6 tests passed`；
- 测试期间桌面没有真实键鼠动作；
- 现有 RemoteAgent 仍是只读界面。

新增的第 61 项使用真实 loopback TCP + TLS/HMAC：视频和输入能力同时协商，认证前不创建受保护资源；认证后先看到 SCREEN_INFO/JPEG，再发送扩展键按下、鼠标左键按下和 PING；PONG 与视频必须保持同一服务端序号，DISCONNECT 后键盘与按钮各释放一次，采集源释放且端口可复用。

## 下一切片

Windows 通过后，在 Mac 增加对应的单连接双向调度器：复用现有有界 `InputSendQueue` 的移动合并、单写入中和释放顺序，同时持续接收 JPEG。完成传输无关和 Network.framework 回环测试前，不接产品窗口事件，也不启用 Windows 原生 sink。
