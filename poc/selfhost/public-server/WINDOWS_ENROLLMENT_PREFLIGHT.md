# Windows 1、2 验收准备（r31）

## 2026-10-26 Windows 实机结果

第一步本机预检已在 Windows 普通非管理员桌面完成：源码 `ee8f03b4d834`，fresh 包 `artifacts/connection-poc/windows-preflight-20261026-a`。9 个顶层原生存储测试、存储/租约多进程组合、本机 OS 签名 HTTPS 正常与未知回复组合、真实 SDK 固定失败服务取消/join/清理全部通过；脚本实际退出码 0，最终完整 `PREFLIGHT PASSED` 标记出现，无 FAIL/SKIP。结束后 18443 已释放、测试/helper 无剩余进程，旧配对/证书及在线 Tailscale 保持不变。

该证据关闭 r31 的 Windows 本机预检与 PowerShell 执行待办；**第二步真实服务器联合验收尚未执行**。用户本轮明确只完成本机预检，没有服务器维护许可或操作。完整构建身份、日志路径与下一步见 [HANDOFF.md](../../../docs/HANDOFF.md) 顶部。以下 r31 包名为历史准备说明，不能在 23 小时有效期外直接复用。

Windows 关机期间只构建新包，不连接 Windows，也不暂停服务器。3（持续 watch/撤销/实际拒绝）和4（产品 TLS/HMAC 数据面）仍暂停。

## 第一步：独立 Windows 预检包

新包为 `artifacts/connection-poc/windows-enrollment-preflight-r31.zip`，不是旧安装包。解压到一个新的目录，用**普通已登录桌面 PowerShell**进入目录执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run-windows-enrollment-preflight.ps1
```

无需 Go、旧 RemoteAgent 安装包、SSH 桌面进程或两个 READY 窗口。不要删除或退出现有 Tailscale；本轮只使用隔离 TEST 命名空间。

脚本按顺序执行完整 enrollmentstore 原生测试（包括最新空缓存/未知文件门禁）、多进程存储租约与重开拒绝、原生 profile/OS signer＋本机签名 HTTPS 正常与未知回复组合、真实 SDK 对固定本机失败服务的取消/同步退出/精确清理。HTTPS 中的注册节点仍是内部替身，SDK 检查是失败路径，两者不能拼接成成功入网证明。使用固定本机端口18443；若被占用则拒绝，不停止其它服务。

完整结束标记：

```text
PREFLIGHT PASSED: Windows native storage + OS signed local HTTPS + actual SDK cancel/join. NOT real Headscale enrollment, verifier bind, public service, GUI or installer acceptance.
```

任一 FAIL、SKIP、stderr、非零退出或退出超时都拒绝通过，后续步骤不执行；不 kill、不删除锁、不自动换 scope 重试。保留当前包与 TEST 证据，反馈固定诊断输出。实际退出未确认时，不能认为 owner 或 SDK 已 join。

包内 SHA256.json 与严格文件集合用于传输完整性检查，**不是签名发行/安装信任锚**。不要把截图、日志或旧包放进解压目录。23小时 TEST profile 过期后需要重新构建，不能调系统时间、用旧 profile 或修改生产 epoch/root。

## 第二步：Windows＋真实服务器联合验收（尚未执行）

`autonomous-native-validation` 和 builder 已支持 Windows amd64/CGO0、固定 `.exe` helper 路径。它使用同 Mac r30 的 BoundEnrollmentRunner：OS 签名 HTTPS 注册/批准/校时/enroll → 真实 SDK → issuer 签名 verifier permit → 实时证明 → backend bind → 实际 join 与精确本机 TEST 清理。claimant 仍是本机 memory signer 测试角色，没有第二节点或产品 payload。

待预检通过后，协调人需要在新的获准预发布维护窗口建立独立 stage、读取其公开 issuer、构建新的短期包与独立 server-only TLS 材料，准备 Windows 侧可达且匹配编译 profile 的私有 HTTPS 转发。不要复用 r30 已 restored 的 stage/端点或本轮 builder-smoke 包。不向 Windows 复制管理 SSH 私钥或 server TLS/issuer 私钥，不将注册凭据经 SSH 交付给客户端。

联合客户端完成后还会显式检查：profile helper已实际退出、精确删除成功，并且同一TEST命名空间的原生属性为空。删除回执本身不再算清理PASS；Windows必须给出自己的结果，不能用Mac的Keychain证据替代Credential Manager证据。

新联合包运行 `run-autonomous-native-validation.ps1`，但只有协调人确认维护/转发/后端 READY 后才执行。此脚本只接受 fresh one-shot 包；生成 public-recovery.json 后不允许自动再跑。必须等实际客户端退出，再允许服务器回收和恢复。服务器新版 operator 的当前 PID READY 顺序修复也需在该次完整往返复测；r30 手动恢复不是新版 operator 全流程通过。

新版operator的READY/STOPPED回执绑定本boot、当前PID和本次ExecStart单调时间，防止PID复用或旧日志误判。恢复中断后checkpoint保持`resuming`，只能由操作者明确执行`finish-resume`续接；它不重启failed实例、不跳过candidate join或baseline检查。服务恢复后再执行只读`verify-restored`，确认当前实例READY及正式Store/资源/零规则/unit/Nginx基线仍一致。

可信构建命令接口现为：

```text
autonomous-native-validation-build GO_EXE MODULE TARGET DEST JOURNAL SIGNER PUBLIC_BOOTSTRAP SERVER_ONLY_TLS_OUT
```

TARGET 仅 darwin/windows。TLS_OUT 必须在分发目录外，父目录无 symlink；写入0600且排他新建。公开 bootstrap 拒绝未知字段、尾随 JSON、零/错误 issuer。客户端包只包含公开 profile/证书、helper 与摘要，不包含 TLS/issuer 私钥。

## 本轮证据与边界

Mac 上新增 builder 两包测试、devicevalidation/installeddevice/enrollmentstore/trustprofile 六包 race 通过；Windows preflight 新 helper/四个验收入口交叉构建、完整 Windows 联合 builder smoke 构建通过。smoke 仅复用 r30 **公开** issuer 做编译检查，无服务器联系/部署/执行，不能用于真实验收。

本机无 PowerShell，两个脚本尚未做 Windows PowerShell 运行验收。Windows 原生执行、成功入网/绑定、服务器维护往返均仍待，不能以交叉构建代替 PASS。正式 installed-device-client/GUI/安装程序入口未变。
