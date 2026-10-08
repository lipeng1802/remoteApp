# Personal Remote Desktop MVP

4W 最新实测：Windows 第二 borrower 窗口真实原生身份/独占存储 probe 通过，两个 API signer READY；首个签名注册失败，未进入节点绑定/TLS。服务器随后自然回收测试进程/verifier/临时目录，管理端精确恢复原策略，节点/用户/票据为空，既有服务/Nginx/Store不变。新增固定注册失败分类和实际远端回收确认；**原因仍待新一轮诊断，不算联合验收通过，保留包不能续跑。** 等待 Windows borrower 先 STOPPED、身份 creator 后 STOPPED。详见 [交接](docs/HANDOFF.md)。

4C-4W：新增 `split-consent.py --compiled` 联合协调入口，串接匹配真实服务的 fresh test pair、Windows 桌面 borrower、OS 签名授权与 bound-node 交接、实际产品计数持续增长、正常 join 后签名撤销。私有管道有界且不强杀冒充完成。23项 Python 回归与7个跨平台helper构建通过；**真实联合执行仍待 Windows READY，未验收活会话撤销/故障/升级，整体4未完成。** 操作与边界见 [交接](docs/HANDOFF.md)。

4C-4V：补齐 compiled 两端实际产品的私有计数观测（不暴露 HMAC/证书），重复查询后仍走真实停止/升级 join 屏障；新增显式隔离 compiled 协调模式，旧模式期限不变。Mac全量race、双平台vet、11项Python、Windows5项局部测试与Linux3项纯协议测试通过。**完整新compiled双机协调、真实TLS持续payload与borrower桌面实测仍待，整体4未完成。** 见 [交接](docs/HANDOFF.md)。

4C-4U（联合验收准备）：补齐隔离双端可信构建、Windows 借用桌面身份入口、独立 session/identity scope 与一次性 bound-node 交接。构建时固定匹配的 CA/issuer/两端 SSH 回环端点，测试 root 只在内存生成，包均 Test=true；不替换正式信任。Mac 全量 race、双平台 vet、11 项 Python 回归、Windows 两项新信任材料测试和包校验/三脚本语法检查通过。双端包仅 build-only，**真实新 compiled Headscale/产品 TLS 联合会话与借用入口原生实测仍待**，整体 4 未完成。见 [交接](docs/HANDOFF.md)。

4C-4T：新增 Windows **测试专用 compiled 桌面入口**，固定 root/配置/签名器/产品摘要，重开桌面测试身份并持有独占原生 journal，通过同用户本地命名管道协调启动、停止和升级；停止真正 join 后才确认，清理仅针对本轮测试数据。Mac 全量 Go race、双平台 vet 与 Windows 4 项管道测试/包校验/PowerShell 语法检查通过。用户已确认 r2 正常 Windows 桌面的初始化、原生签名/独占存储 probe 与 STOPPED 清理全部通过，**桌面原生入口验收闭环**；新 compiled 双机 Headscale/产品 TLS 尚未验，整体 4 未完成。结果见 [交接](docs/HANDOFF.md)。

4C-4S：[产品进程进入 compiled owner 租约屏障](poc/selfhost/public-server/INSTALLED_PRODUCT_OWNER.md) 已接线：产品/网络/节点/watch/native signer 实际 join 后才能提交升级或释放；补取消错误传播、命令不因 Close 超时提前退出、同一编译配置客户端的 OS 签名证书发布/新鲜 TLS 描述符校验。Mac 全量回归、Windows 11 项局部验证及 Mac 原生 signer 严格退出测试通过；真实 Swift/C# 两端固定产品摘要包仅构建，未安装或跑新双机会话，整体 4 未完成。

4C-4R：新增[专用管道/进程对和启动适配](poc/selfhost/public-server/INSTALLED_PROCESS_COORDINATION.md)，macOS/Windows 的 10 项进程行为测试通过。尚未切换真实双机协调、接 OS TLS 供应回调，或将独立产品生命周期纳入 compiled owner 的 journal 租约；不算 Headscale/产品 TLS/安装验收。

4C-4Q：[编译锚节点客户端入口](poc/selfhost/public-server/INSTALLED_NODE_RUNTIME.md) 已开发接线：installed-session与双helper摘要固定、只open已安装配置、OS signer重开、签名ACTIVE binding先于实际tsnet.Up、角色node/IP检查、nodeproof绑定当前grant、TLS监听/workers/节点关闭纳入owner。两端包构建与Mac/Windows局部运行时测试通过；实际双机Headscale/OS signer/产品TLS进程组合仍待，未初始化真实信任channel或改旧GUI，整体4未完成。

4C-4P Windows r3完整桌面验收已由用户确认VALIDATION PASSED，第一段闭环。r2的Unix服务器Store测试依赖问题已通过仅内存临时授权签名fixture修复，正式Store权限未放宽；不代表实际Headscale、产品TLS/GUI或签名安装包验收。见[交接](docs/HANDOFF.md)。

4C-4P：[live配置owner与退出屏障](poc/selfhost/public-server/LIVE_CONFIGURATION_OWNER.md) 第一段已闭环：已安装配置与会话串行、cancel后真实join再commit、超时保留原生租约并锁闭；补齐连接/watchdog/授权callback退出等待。Mac新增原生实测、真实本地HTTPS/opaque tunnel升级集成、全量race及双平台vet通过；用户确认Windows r3完整桌面通过。未切换现有enrollment-helper/GUI或完成实际Headscale节点/产品TLS组合验收，整体4未完成。

4C-4O：[编译固定安装锚/离线配置升级](poc/selfhost/public-server/TRUST_INSTALLATION_RELEASE.md) 已闭环：固定root/service/floor/channel/helper摘要、明确首次安装、原生升级与拒降级、失败保留原提交，换根不选择新存储。Mac编译产物真实验收/全量race通过；用户确认Windows新版桌面验收返回本轮VALIDATION PASSED。现有预发布公开材料已构建维护工具但未安装，未接实际客户端会话或修改产品包，整体4未完成。

4C-4M/4N：[macOS、Windows原生信任存储](poc/selfhost/public-server/TRUST_DESKTOP_JOURNAL.md) 两端实现与原生验收闭环：Keychain/Credential Manager、独占helper、分块提交索引、防降级重开、异常拒重置。Mac4项原生验收/全量race通过；用户确认Windows桌面新包返回VALIDATION PASSED，SSH凭据库不可用记录仅为历史。下一步可信安装锚供应/配置升级，未接产品客户端或完成安装验收，整体4未完成。

4C-4L：[Linux root专属信任journal](poc/selfhost/public-server/TRUST_LINUX_JOURNAL.md) 已实现并通过4项Linux真实跨进程验收：独占owner、重开保留版本下限、非特权拒绝、缺失/损坏拒重置和故障锁闭。只保护非特权本地用户，不防root/整盘快照回滚；未接桌面存储、正式安装锚供应或客户端。整体4未完成。

4C-4K：[可信安装与版本下限核心](poc/selfhost/public-server/TRUST_INSTALLATION_CORE.md) 已完成：先验证、等待旧会话退出、持久化提交，再允许使用新配置；拒绝降级/同版本改写，故障锁闭。全量 Go race/vet 和 Linux/Windows 测试构建通过，存储测试使用内存模拟，尚无生产 OS 存储适配或安装包接入。用户已确认4C-4J Windows STOPPED，整体4未完成。

4C-4J：[旧ACL留存时真实TLS关闭/显式恢复](poc/selfhost/public-server/SPLIT_TLS_FAULT.md) 完整双机实测通过，退出0。隔离daemon实际暂停、HTTPS503且旧最小规则仍在，真实TLS非timeout关闭；同verifier身份显式恢复后旧gate/连接不自动重开，显式新helper/TLS可恢复，最终撤销/清理/原策略恢复。线上Store/Nginx未变，Windows最终STOPPED待确认；非正式信任安装/公网产品，整体4未完成。

4C-4I：[独立 daemon/frontend 的真实产品 TLS](poc/selfhost/public-server/SPLIT_PRODUCT_TLS.md) 完整双机复测通过，退出0：真实Swift/C#、OS签名证书私钥证明/新鲜描述符、错pin/错HMAC/认证前载荷拒绝、自主HTTPS刷新跨lease、撤销关闭非timeout、精确回收恢复。修复先停daemon导致cleanup_pending尚未提交的清理竞争，Linux5项断言通过。Windows最终STOPPED待确认；旧ACL残留故障/正式信任安装等仍待，整体4未完成。

4C-4H：独立 daemon/frontend 链路的[真实双机节点证明](poc/selfhost/public-server/SPLIT_OS_CONSENT.md) 完整复测通过，退出码0。注册前公钥/单次签名意图、两端真实节点与 OS 身份持有证明、错误身份拒绝、同节点恢复、单向最小 ACL 与正反载荷、撤销后回收/拒绝及精确清理通过。首轮日志清理断言失败已修正并完整重跑；产品 TLS 尚未接此新链路，整体4未完成。

4C-4G：[独立 daemon/frontend 双机 OS 授权](poc/selfhost/public-server/SPLIT_OS_CONSENT.md) 真实双机通过，退出码0：两端 OS 签名注册、pending/拒绝/自批准拦截、明确批准/同grant校时验签、撤销及清理恢复。协调端只有公共 bootstrap/stop，无代签批准接口。此阶段只验设备 consent，不宣称客户端 nodeproof、产品 TLS 或可信安装验收；Windows 最终 Enter/STOPPED 待确认，整体 4 未完成。

4C-4F：[私有前端网络路由/跨进程HTTPS](poc/selfhost/public-server/PRIVATE_FRONTEND.md) 完整实机通过，退出码0；默认network404，显式开关仅回环+broker。真实root daemon/nonroot frontend/签名设备HTTPS、无grant拒绝、错误CA拒绝、故障503/显式恢复、重放保持和零退出通过。未部署线上或开放公网，整体4未完成。

4C-4E：[真实独立daemon/非root IPC/恢复验收](poc/selfhost/public-server/DAEMON_NETWORK.md) 已通过，退出码0；修复已join健康故障遗留writer.lock导致显式重启失败。同verifier key/IP恢复、故障非零关闭、不自动重启、精确清理与原策略恢复通过。未启用线上network或完成新daemon双机授权，整体4未完成。

4C-4D：[独立权威服务可选网络装配](poc/selfhost/public-server/DAEMON_NETWORK.md) 代码、全量race/vet、Linux构建及三项Linux隔离组件测试通过；默认device-only不变，尚未部署真实daemon网络后端或开放公网接口。4C-4C Windows最终STOPPED已由用户确认。下一步隔离实机装配验收，整体4未完成。

最新 4C-3D：[单一权威 daemon / 无特权 HTTPS frontend](poc/selfhost/public-server/AUTHORITY_MIGRATION.md) 已受控迁移至服务器，真实权限隔离、回环HTTPS、第二writer拒绝、权威退出503/重启恢复通过，线上State哈希未变。root-only备份保留，新服务active但未开机启用；网络仍关闭。下一步接 verifier/nodeproof/maintenance/fence 真实后端。整体切片 4 未完成。

4C-4A：[真实后端装配与生命周期 supervisor](poc/selfhost/public-server/NETWORK_SUPERVISOR.md) 组件已接入固定Headscale驱动、可信Store公钥与runtime-owned nodeproof；故障取消/join/锁闭测试完成。尚未部署或完成该装配的真实双机验收，线上network继续404。

4C-4B 补齐私有HTTPS→kernel broker验证适配，修正策略目录/database配置与network执行期限；Linux五项隔离组件测试及Windows桌面签名探针通过。真实双机新版协调流程仍待，未改线上network授权。

4C-4C：[新版 supervised 真实双机协调验收](poc/selfhost/public-server/SUPERVISED_NETWORK_TEST.md) 已完整实机通过（退出码0）：真实Runtime/kernel broker/supervisor、两端OS签名/nodeproof、最小规则/零规则真实拒绝、产品TLS、故障锁闭/显式恢复、撤销及同身份verifier重启。实验资源全部回收、原策略恢复，线上State/Nginx未变；Windows最终STOPPED已确认。仍为私有协调实验，非公网产品或整体切片4完成。

最新 4C-3A：[持久 verifier](poc/selfhost/public-server/PERSISTENT_VERIFIER.md) 的明确创建/同身份重启/故障停止/精确回收代码与真实 Headscale 验收通过，测试资源已回收。尚未部署长期 verifier 或接通设备服务网络接口；下一步是固定权限 broker/可信设备公钥绑定，再接实际后端。整体切片 4 未完成。

最新 4C-2C：用户授权在线服务私钥上传后，[独立设备-only 服务](poc/selfhost/public-server/DEVICE_SERVICE.md) 已实际部署到服务器回环 127.0.0.1:8444，首次/重启后 profile HTTPS 拒绝探针通过，状态稳定、停止释放锁与监听，最小权限及原 Nginx 配置哈希核对通过。服务 active，未开机启用/开放公网或网络授权，不改业务 443；整体切片 4 仍未完成。

当前收口 4C-2A：Windows [发布签名服务信任配置](poc/selfhost/public-server/SERVICE_TRUST_TEST.md) 实际验证通过；用户选择独立自托管预发布，隔离本地配置材料已生成并验证。尚未部署公网设备 API、安装可信锚或完成整体切片 4。密钥保管、候选端口 8444 与下一步边界见 [预发布配置](poc/selfhost/public-server/PRERELEASE_BOOTSTRAP.md)。

最新切片 4B-4B：两端真实 OS 身份贯穿设备 HTTPS、Headscale 持有证明、自主刷新与真实产品 TLS 故障/撤销关闭，完整双机验收零退出，资源回收及策略恢复通过。用户确认本轮 Windows STOPPED/随机测试身份清理通过，本轮验收闭环；正式信任锚/HMAC 分发、产品 GUI/安装仍待，整体切片 4 未完成。见 [OS 双机验收](poc/selfhost/public-server/NATIVE_NETWORK_TEST.md) 和 [当前 Handoff](docs/HANDOFF.md)。

另提供 [Windows 本机原生生命周期独立入口](poc/selfhost/public-server/NATIVE_WINDOWS_ENTRY_TEST.md)：无需旧安装包/SDK，Windows 交互式实际验收已通过；此入口不是完整跨网双机 OS signer 启动验收。

Windows [桌面 OS 身份私有桥接（4B-4A）](poc/selfhost/public-server/INTERACTIVE_VAULT_BRIDGE_TEST.md) 的桌面、中文路径启动、SSH pinned 探针和前一轮 STOPPED 均已通过。[真实 OS 双机协调（4B-4B）](poc/selfhost/public-server/NATIVE_NETWORK_TEST.md) 已实际运行通过；独立新包仍是测试入口，不是旧安装包的升级。

从个人使用 MVP 起步：在 macOS 控制端通过外网控制一台 Windows 电脑。最终面向其他用户提供接近 AnyDesk 的独立安装/连接体验，见 [产品路线](docs/PRODUCT_ROADMAP.md)。当前产品基线仍使用系统 Tailscale；独立连接 PoC 验证自托管 Headscale/DERP，数据通道不依赖 Tailscale 托管账号，SSH 仅用于测试管理。

2026-10-07 独立 PoC 进展：helper 注册前公钥与自签意图、真实网络/应用持有证明、单向动态策略、零 ACL 网络拒绝/正向恢复、撤销及磁盘/冷驱动重开已双机验收。产品 TLS 固定载荷桥接已通过，仍未接 GUI 或公开注册服务，不把管理员 fixture 当最终安装体验。见 [验收记录](poc/selfhost/public-server/ENROLLMENT_TEST.md) 和 [当前交接](docs/HANDOFF.md)；下方保留早期 MVP 边界记录。

## MVP 边界

- macOS 只作为控制端。
- Windows 只作为被控端。
- 使用 Tailscale 提供外网寻址、NAT 穿透和加密网络。
- 应用负责屏幕传输、认证、鼠标键盘控制和断线恢复。
- 第一条可运行链路先使用 JPEG；链路稳定后替换为 H.264。

暂不开发自建信令/中继、Windows 控制 Mac、文件传输、剪贴板、多显示器、声音和企业功能。

## 当前进度

P0、TLS 和应用认证双机验收已通过；P1 JPEG 能显示 Windows 画面，画质/真实采集复验和跨网 30 分钟稳定性仍待完成。P2 已把键鼠捕获和同连接 JPEG + 输入接入产品 GUI，目前 Windows 只统计输入、不执行原生注入，待双机 GUI 验收。

上一切片 Windows Release、协议 61/61 和输入边界 6/6 已通过；本切片 Mac 109/109 和 Release 已通过，Windows 构建与双机产品 GUI 操作待验证。按 [GUI 控制 mock 交接](docs/GUI_CONTROL_MOCK_HANDOFF.md) 执行，最新状态以 [HANDOFF](docs/HANDOFF.md) 顶部为准。

最终交付为两个平台各自的安装包：macOS 控制端 `.dmg` 和 Windows 被控端 `Setup.exe`。开发阶段从 P0 起持续验证打包，不等到功能全部完成后再处理安装问题。

## 构建与测试

macOS（需要完整 Xcode）：

```bash
cd macos/RemoteController
swift test
swift build
```

Windows（需要 .NET 8 SDK）：

```powershell
dotnet build .\windows\RemoteAgent\RemoteAgent.sln -c Release
dotnet run --project .\windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj -c Release
```

生成自带 .NET 运行时的 Windows x64 发布目录：

```powershell
dotnet publish .\windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj -c Release -r win-x64 --self-contained true
```

## 开发版安装包

生成 macOS 开发版 `.dmg`：

```bash
./packaging/macos/build-dmg.zsh 0.2.0
```

默认使用 ad-hoc 签名，仅供本机和开发测试。正式对外分发需要 Developer ID 签名和 Apple 公证。

在仓库根目录执行以下命令，使用已安装的 .NET 8 SDK 与 Inno Setup 6 生成自带运行时的 `Setup.exe`：

```powershell
.\packaging\windows\build-installer.ps1 -Version 0.2.0
```

产物统一写入被 Git 忽略的 `artifacts/` 目录。Tailscale 不捆绑进安装包，应用只负责检测并引导安装。

## 文档

- [开发计划](docs/DEVELOPMENT_PLAN.md)
- [测试计划](docs/TEST_PLAN.md)
- [下一会话交接](docs/HANDOFF.md)

## 计划中的目录结构

```text
macos/RemoteController/     Swift/SwiftUI 控制端
windows/RemoteAgent/        C#/.NET 8/WPF 被控端
protocol/                   跨平台协议和共享测试向量
docs/                       规划、测试与交接文档
```

继续开发前先阅读 `docs/HANDOFF.md`，其中记录了实际验证结果和下一步。

### 720p画质对照（0.2.4）

Windows现提供低带宽、标准（默认）、清晰三档，仍最高1280×720/10FPS。自包含目录和Setup已生成；32项协议测试与合成画质对比通过，真实采集待复验、Mac当前离线。按 [画质交接清单](docs/JPEG_QUALITY_HANDOFF.md) 顺序验证，清晰档不保证跨网不卡顿。

### Mac 本地输入预览（开发工具）

同步本轮代码后，在 macos/RemoteController 运行：

~~~bash
swift run -c release InputPreview
~~~

点击开始后测试窗口内键鼠、黑边与失焦释放；只显示本地事件计数，不连接 Windows，不记录按键内容。该工具的人工清单已通过；产品 GUI 的跨机 mock 仍按 [GUI 控制 mock 交接](docs/GUI_CONTROL_MOCK_HANDOFF.md) 单独验收。

### 输入发送状态机

有界队列、相邻移动合并、单写入序号、认证门禁、心跳与结束期限已加入。固定回环的 TLS 网络适配器已编写，Mac 新增代码仍待编译和实测；当前优先完成 [流程检查点](docs/WORKFLOW_REVIEW.md)，GUI 仍只读。

git -c http.proxy=http://127.0.0.1:1080 pull --ff-only origin main

git -c http.proxy=http://127.0.0.1:1080 push origin main
