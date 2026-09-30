# Mac 单连接双向调度器交接

## 已完成

Mac `AuthenticatedInputSender` 和 `InputConnectionDriver` 现可在显式启用 JPEG 时协商 `Jpeg | Input`，持续接收 SCREEN_INFO/JPEG，同时保留现有有界输入队列、相邻移动合并、最多一个写入中、100 帧/秒发送节奏、心跳和释放后 DISCONNECT。

服务端入站序号与客户端输入出站序号分别连续维护；收到 JPEG 不占用或改变输入发送序号。JPEG 可以跨多个 Network.framework 回调分片，单次回调最大 64 KiB，完整帧仍受协议 8 MiB 上限约束。视频必须在有效 SCREEN_INFO 后出现；能力缺失、顺序错误、超限回调、EOF、写入失败或超时均关闭连接并清空待发输入。

开发用 `TLSInputSimulationClient` 新增可选 JPEG 回调，用于验证同一真实 TLS 连接同时接收视频和发送输入。没有传 JPEG 回调时保持原 input-only 能力和 4 KiB 回调边界。

## 本机验证结果

- `AuthenticatedInputSenderTests`：15/15；
- `InputConnectionDriverTests`：15/15；
- `RealTLSInputSimulationTests`：6/6，包含新增真实 Network.framework TLS 双向用例；
- 全量：109/109，0 failures；
- Release：`Build complete! (19.33s)`。

第一次在文件沙箱内运行时，Keychain 和测试 PKCS#12 导入分别因系统权限返回 `keychainStatus(-50)` 与 `invalidFixture`，新增的非系统权限测试均已通过。按项目既定方式在受控沙箱外重跑后，全量 109/109 通过；上面的通过结论来自第二次完整运行。

## 仍未完成

产品 `RemoteController` SwiftUI 查看器仍使用原只读 `TLSControllerClient`，没有接入这个双向调度器或 `ControllerInputCapture`。Windows WPF 也尚未把 `WindowsInputSink` 传给双向服务端。因此当前安装包仍不能真实远控。

下一切片应先做产品级许可和生命周期接线：Windows 增加默认关闭的“允许本次远程控制”，仅勾选时创建 native sink；Mac 查看器只在认证成功、画面聚焦且用户显式开启控制时捕获输入。停止、失焦、窗口关闭、断线和错误必须先生成释放，再结束会话；发送失败仍依赖 Windows 服务端兜底释放。接线后先使用内存 sink 做双机 Tailscale 验证，再单独启用真实 `SendInput` 人工验收。
