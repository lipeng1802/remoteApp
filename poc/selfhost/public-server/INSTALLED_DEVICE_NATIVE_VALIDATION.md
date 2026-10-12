# r18：新设备客户端与原生身份/信任存储联合验收

Mac实际新编译程序完整通过、actual exit0；Windows包上传/摘要/脚本解析与3项纯协议/进程测试通过，普通桌面原生联合验收尚待。不是公网服务、Headscale、产品TLS/GUI或安装验收。

## 本轮链路

已编译 `installed-device-client` → 已安装原生journal/编译固定profile → 重开真实OS身份签名helper → 本机独立TLS fixture → 验签/设备身份/nonce重放/时间校验。注册和邀请结果只在父子专用管道解析，不输出邀请码、设备标识、grant或原始错误。

fixture是有限的register/invite签名验证器，没有正式authority Store、审批grant或网络授权；稳定重启验证客户端OS身份不变，并非服务器durable Store重启。发布签名根与CA私钥只在可信构建内存生成，不读取正式材料；validator只内嵌一次性loopback TLS叶证书私钥，属于明确TEST制品，不能部署为正式服务。固定profile约23小时有效，回环端口由构建时选定；到期或端口占用必须准备fresh包，不调整系统时间/换运行时地址。

## 必须通过的门禁

1. 真实新程序缺少原生安装状态时failed/actual exit1，不自行initialize或修复。
2. validator明确初始化随机TEST journal；父帧地址注入被拒绝，failed/actual exit1，原生epoch1仍完整。
3. 客户端ready期间第二maintenance owner不能读取同journal；真实客户端持有独占租约。
4. 注册与邀请的签名由实际OS signer产生，HTTPS fixture验证实际公钥/签名/nonce/时间。
5. stop后父端半关闭专用输入，再要求exact closed/actual exit0；读取线程、HTTP与OS signer均已join，原生journal可再次Inspect。
6. 第二次客户端启动重开相同OS身份，设备注册结果一致，正常关闭并重开原生epoch1。
7. 同步TLS服务器关闭/join，然后精确删除本轮TEST journal与随机OS身份并确认helper退出，才输出最终VALIDATION PASSED。

没有强杀充当正常退出。30秒child检查期限只关闭专用输入，超时必定失败；仍等待实际Wait完成才清理。不删除未知历史凭据或生产安装。Native helper自身失败处理不被计为正常join。私有响应从不写终端/文件，日志仅固定PASS/诊断类别。

## 实测与失败历史

Mac旧r18/r2失败不能当PASS。r3固定阶段证据显示注册/邀请实际签名均通过，失败在终止握手：父端等待closed但保持输入开启，子读取线程等待EOF，30秒期限触发；随机测试journal/identity最终清理已确认。修正仅validator parent：发送stop后关闭专用输入，再检查closed和实际exit0，未放宽生产身份/配置/退出门禁。

新增独立子进程半关闭回归。Mac fresh r4 `artifacts/connection-poc/installed-device-native-r18-mac-r4/installed-device-validation` 全部上述检查actual exit0，无SKIP，最终两类原生清理PASS。相关7包race、专用vet与既有Python78项PASS。Windows三项仅memory signature/replay fixture、TEST namespace guard、真实父子输入半关闭/exit0，在SSH下执行且实际全PASS；它们不访问原生凭据，不冒充桌面联合验收。

## Windows操作

新包位置：`C:\Users\jarvis\windows-installed-device-native-20261012-r18`，公开文件摘要全部一致，入口解析通过。不要运行旧r17 build-only包或旧双窗口入口。

普通已登录Windows桌面PowerShell，**一个窗口**执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\jarvis\windows-installed-device-native-20261012-r18\run-installed-device-validation.ps1"
```

自动测试和精确清理，无READY/Enter流程。成功必须actual exit0，末尾：

```text
VALIDATION PASSED: actual compiled device client + native trust lease + OS signature direct local HTTPS + stable restart + joined test cleanup. NOT public service, Headscale, product TLS/GUI or signed installer acceptance.
```

失败保留包与固定日志，不删Credential Manager或writer.lock，不通过SSH反复执行原生验收。若包过期/端口占用需fresh build。PowerShell以ProcessStartInfo并发读取stdout/stderr普通文本，actual WaitForExit，不让普通stderr变成NativeCommandError，不以超时/kill当成功。

## 下一步

收到Windows原生通过结果后记录该联合入口闭环，继续长期服务自主HTTPS接入/正式信任与受保护凭据供应。不直接公网反代现有private-network；部署前确认备份/回退、端口、材料时效和授权，再进行公网负向与服务故障验收。整体切片4未完成。
