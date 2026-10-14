# r27 原生租约与真实 SDK 失败注册生命周期

新入口 `cmd/enrollment-sdk-validation` 固定使用 TEST 原生 namespace 和
`http://127.0.0.1:18443` 本机失败服务，不接受参数、父帧、路径、地址或实际凭据。
端口被占用即失败，不复用/停止已有服务。无需 Tailscale 登录、旧安装包、Go 或两窗口 READY。

验证路径：显式 TEST Initialize → 原生 Begin/attempted → 真实 tsnet SDK Up
（公开无效字符串，不是 Mint 密钥）→ 确认实际请求到达失败服务 → 4秒预算取消 →
同步 SDK Close → 原生 lease 重开 → attempted 保留/Begin与factory再次拒绝 →
SDK 与本机服务实际 join 后的精确 TEST 清理。
不能以超时、cancel 发出、PID 消失或强杀算 join。失败保留隔离证据，不自动删锁/换 scope 重试。

实际 SDK 会创建 `tailscaled.state`、`tailscaled.log.conf`、`tailscaled.log1.txt`、
`tailscaled.log2.txt`。TEST 清理先验证整个文件集与原有 UID/DACL/链接/路径门禁，
仅允许这四个精确名称；发现未知文件/目录时不删除任何一个。原生 SDK key 仍是权限保护文件，
不宣称已额外加密。空 TEST 目录/锁保留，生产仍没有 reset/delete 接口。

Pinned tsnet 的 startLogger 创建独立日志器，不使用环境 no-logs 开关配置其 Disabled；
ConfigureOnline 因此额外调用 logtail.Disable，进程级不可重启上传。环境开关仍保留，
本地 SDK 日志文件仍会创建。此初始化只适用于独立 helper，不在 GUI 内调用。

Mac：CGO=1 构建 fresh `enrollment-sdk-validation` 后直接运行。
Windows：fresh `enrollment-sdk-validation.exe` 与 `run-enrollment-sdk-validation.ps1`
同目录，在普通已登录桌面运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run-enrollment-sdk-validation.ps1
```

最终标记必须是 `VALIDATION PASSED: native TEST lease + actual SDK Up failure/cancellation ...`，
并且进程实际 exit0。Windows 当前只构建，不计桌面通过。

本入口不是 Headscale 服务端拒绝/成功注册：它没有批准 Store、真实签名 grant/enroll、
Mint、nodeproof/bind/watch、产品 TLS/GUI 或签名安装器。r26 的签名 HTTPS 组合也不能
与这里拼接后宣称端到端成功。接下来需独立受控资源的真实 credential→Up→Observe→
精确 Retire 与退出组合，保护 r22 长期 verifier/database 零 ACL，不运行旧空控制面脚本。
