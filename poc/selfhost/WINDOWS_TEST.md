# Windows x64 本机隔离 PoC 验证

本包不是远控安装包。只传固定测试数据，不采集屏幕或注入键鼠；无需 Go、Python、.NET SDK。
要求 Windows x64、PowerShell 5.1+、运行中的 Docker Desktop（Linux 容器模式、本机 npipe context）。不修改 Docker 代理、WSL、防火墙或已有 Tailscale/配对身份。

## 执行

1. 对照交付的 `.zip.sha256` 核对 ZIP 的 `Get-FileHash -Algorithm SHA256`，再解压到一个独立目录。内部清单用于完整性检查，不能替代可信交付渠道或代码签名。
2. 启动 Docker Desktop，确认 Linux 容器模式。先显式下载本次所需镜像：

   ```powershell
   docker pull ghcr.io/juanfont/headscale:0.29.4
   ```

3. 在解压根目录执行（不要在原远控安装目录运行）：

   ```powershell
   powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\poc\selfhost\verify-windows.ps1
   $LASTEXITCODE
   ```

   `Bypass` 只作用于这个 PowerShell 进程，不改变系统执行策略。脚本不会自动下载镜像；检查本机镜像后固定本轮 image ID。测试前需本机 18443/18444/19090/15443 端口空闲，不自动停止占用进程。

## 预期

注册无需 Tailscale 登录；固定数据经过本机自建 DERP；错误 token 双端拒绝；客户端连续三次重启、服务端重启后仍能传输，地址/节点数不变；第三节点拒绝；节点通过私有 stdin `stop` 指令正常关闭，无 `cleanup_timeout`。

最后 `PASS windows_smoke_complete`，`$LASTEXITCODE` 为 `0` 才算通过。只回传 PASS/FAIL 输出；不要回传节点状态文件、注册 key 或 Docker 原始日志。

如果出现 FAIL，请保留固定失败标签。`headscale_image_missing_pull_first` 表示镜像缺失，`docker_linux_containers` 表示不是 Linux 容器模式，`docker_engine` 表示 SSH/当前用户会话不能访问运行中的 Docker；不自动切换引擎、提权或重配代理。

公钥 SSH 会话中，`docker pull` 可能报 Windows 凭据助手的 logon session 错误。本轮使用独立临时 Docker CLI 配置，放入 `{"auths":{"ghcr.io":{}}}` 显式匿名条目，并显式连接已确认的本机 Linux 引擎，成功下载公共镜像；空 `{}` 配置仍可能自动探测 wincred，所以不够。临时配置随即删除，用户原 Docker config/登录保持不变。也可以在 Windows 桌面会话中执行上面的 `docker pull` 后再 SSH 测试。不要为此删除原 credentials 或更改认证方式。

脚本仅清理本轮创建的进程、精确容器 ID 和私有临时目录，不删除已有镜像。失败时强杀仅为回收资源，不计正常关闭通过；清理不完整会额外输出 FAIL。

Headscale 发布到 Windows 宿主的 **127.0.0.1:18443**；容器内部 0.0.0.0 监听不代表公开宿主端口。DERP 只监听本机 127.0.0.1:18444，临时 TLS 证书使用 SHA-256 pin。随机会话 token 不是产品验证码。

这只验证 Windows 实机的本机网络栈和生命周期，不证明 Mac↔Windows 跨网。即使用 Tailscale 地址 SSH 管理/传包，PoC 节点仍使用独立身份和本机服务；没有删除或借用系统 Tailscale 身份。

2026-10-06 已在 Windows x64 / PowerShell 5.1 / Docker Desktop Linux 引擎上通过完整实机验证，最终零退出及 `PASS windows_smoke_complete`。macOS 默认回归、私有停止管道加连续三次重启也通过。这不代表其他 Windows 环境、Linux、Apple Silicon 或真实跨网已验收。
