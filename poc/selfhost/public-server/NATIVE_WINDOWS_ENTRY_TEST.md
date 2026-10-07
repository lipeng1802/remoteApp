# Windows 交互式原生生命周期验收入口

独立制品 `windows-native-validation.zip`，不使用旧安装包，不需要 Go/.NET、管理员权限或更改系统 Tailscale。解压后在正常 Windows 桌面 PowerShell 执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run-native-validation.ps1
```

仅当前子进程执行策略，不修改持久系统策略。脚本核对两份 exe 的 SHA-256，设置正确 vault 路径，运行唯一 `TestNativeVaultPortableLifecycle`、60 秒测试上限；要求真实 PASS、无 SKIP、退出码 0。成功输出 `VALIDATION PASSED`。两份制品和脚本必须来自同一个可信测试包；哈希清单防混用/传输损坏，不是正式代码签名或恶意替换防护。

自动检查两个真实 OS 身份创建/固定公钥重开、独立签名 HTTPS 校准与状态刷新超过 3 秒、签名撤销及永久拒绝、两角色 opaque 隧道签名/载荷、服务故障关闭双向 worker/无后续查询。正常完成精确清理随机测试凭据，拒绝私钥/密码导出，不使用内存应用身份替代 OS 存储；测试 issuer 临时在内存中。默认 Go suite 没有原生环境时明确跳过，专用脚本禁止把跳过当通过。

本轮 Mac 原生执行 PASS；Windows x64 交叉构建完成，**Windows 交互式实际结果待用户回传**。用户只需发送完整固定测试输出，无需发送任何密码或私钥。测试期间不要强杀进程；若失败，不广泛删除凭据命名空间。

这只是 Windows 本机原生集成入口：没有 Unix Store 依赖，不放宽生产后端存储权限。网络元数据来自受控 fixture；不验证真实 Headscale/WhoIs、Swift/C# Schannel 或跨网双机 OS signer。本轮没有把完整双机入口补齐，交互式 Windows 与 Mac 的联合启动/协调入口仍需后续开发；不能据此关闭该待办。已有 4B-3 跨网验收使用 fixture AppKey，不能与本项合并冒充双机 OS 身份通过。
