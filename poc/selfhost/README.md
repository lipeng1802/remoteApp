# 自托管连接最小实验

切片 4 当前：独立、默认不启用的`installed-autonomous-product`已有显式安装期身份/enrollment绑定入口。runtime只能重开由compiled channel绑定的OS身份与原生存储，不能由parent选择、创建、reset或替换；macOS已真实完成首次安装、签名epoch升级、两代入口保持同一身份、拒绝降级、产品重启及精确清理，且统一helper避免跨可执行程序钥匙串密码提示。普通`installed-device-client`仍为device-only。Windows实机安装/重启/升级同身份、真实双机自主产品、撤销旧连接、GUI和签名安装包仍待，不能标记整体切片完成。详见[持续授权](public-server/CONTINUOUS_AUTHORIZATION.md)和[交接](../../docs/HANDOFF.md)。下方历史描述保留。

切片 3 当前入口：[设备码/一次性验证码、确认及防滥用](public-server/DEVICE_FLOW_TEST.md)。设备签名 HTTPS 申请/发现/拒绝/明确批准/查询，不再预先批准测试 grant；仍由私有父脚本模拟用户动作，未接 GUI 或公开服务。下一步正式身份分发、平台安全存储/IPC 与客户端自主生命周期；下方“下一步设备码”为历史。

当前：新增 [设备签名 HTTPS 接口](public-server/DEVICE_API_TEST.md)，真实 Mac/Windows 注册、网络注册/绑定、状态/时间查询和撤销，以及周期恢复/真实 TLS 门禁通过。接口仍只在私有回环测试，未公开生产部署或接 GUI；初始信任/批准/helper 调度仍由 fixture 提供。下一步设备码/验证码与防滥用；下方未实现接口/周期维护的内容为历史。

最新：真实 Swift/C# 产品 TLS 固定载荷已通过动态 gate 桥接，错误 pin/密钥/认证前 PING 拒绝、持续授权与停止刷新/Store 故障/撤销关闭及恢复通过。见 [TLS 桥接验收](public-server/TLS_BRIDGE_TEST.md)。无 GUI/安装集成或公开注册；下面未接 TLS 的内容为历史。

最新：动态注册签名绑定已接 [固定载荷活动流](streamgate/README.md)，真实双机持续刷新/停止刷新/实际 Store 故障与保留 ACL 的应用撤销关闭及恢复通过。新增新鲜签名授权时间校准，不扩大租期。产品 TLS/公开注册/周期 Reconcile 未完成；下方“未接动态活动流”为历史。

最新前置：注册账本派生动态签名绑定，不再要求旧 `.1/.2`；只有成功同步最小策略且授权有效才签 active。真实双机节点/IP 一致性、冷驱动恢复、撤销后拒签已通过，详见 [交接](../../docs/HANDOFF.md)。未接动态 helper 活动流或产品 TLS，周期 Reconcile 待实现。

2026-10-07 真实注册闭环：helper 注册前离线公钥/自己签名意图、实际网络+应用持有挑战、最小单向动态策略/零 ACL 真实拒绝、撤销及磁盘/冷驱动重开已在 Mac/Windows/隔离 Linux 验收。见 [验收记录](public-server/ENROLLMENT_TEST.md)。配置实际值为 `database`；测试后恢复 file 原策略。当前仍不开放公网注册、不改产品；下一步产品 TLS/签名状态门禁桥接。下方待闭环描述为历史。

2026-10-07 真实驱动更新：新增 [Headscale 管理适配器](headscale/README.md)，真实服务器随机工单短期单次凭据发放/精确幂等回收通过。动态策略实现要求 db 模式；公网仍 file 模式且未改动。实时持有证明/注册前节点公钥与双机注册闭环待接入，默认拒绝缺失证明，不能宣称完整免登录验收完成。下方内存驱动状态为此前切片记录。

2026-10-07 注册账本更新：新增私有受控注册后端核心/单次短期凭据与最小策略投影，先持久化工单、启动恢复与精确回收；见 [邀请核心](invite/README.md)。当前仅内存驱动验证，真实 Headscale 驱动和公网动态策略尚未实现，不开放注册端口、不改安装包。

2026-10-07 扩展实测：`authorized-network.py --negative-expiry` 公网负向与真实 300 秒 grant 自然到期零退出；状态持续刷新，区分了 grant 到期与 3 秒状态超时，清理/业务哈希复核通过。下一步私有受控注册/网络节点所有权与最小策略；见 [授权验证](public-server/AUTHORIZATION_TEST.md)，不将 fixture 当作最终用户产品。

2026-10-07 授权实测更新：SSH 恢复后，私有双机编排零退出，真实节点公钥绑定、签名授权中继载荷、后端撤销、重启拒绝旧授权、新 grant、3 秒状态到期与精确清理通过。见 [授权验证](public-server/AUTHORIZATION_TEST.md)。仍需补齐公网负向/300 秒 grant 到期；没有公开注册服务或产品桥接，不能宣布最终免账号连接产品完成。

2026-10-07 最新：新增独立 [签名授权连接门禁](session/README.md)，邀请后端的当前授权状态约束真实本机 TCP 固定载荷，处理互证、重放、撤销/到期与故障关闭。尚未接双机 helper/可信 Headscale 节点绑定，不开放公网；握手本身不提供加密。

2026-10-06 最新：新增独立 [设备码/一次性邀请后端](invite/README.md)，本机签名请求、明确批准、持久化授权/撤销、真实 loopback HTTP 与竞态检查通过。尚未接 Headscale 或双机 helper，不开放公网注册；既有中继/直连验证与产品安装包保持原状。

2026-10-06：已授权公网 Headscale/嵌入 DERP 的 Windows serve / Mac probe 实测通过，包括固定载荷、错误 token 双端拒绝、客户端连续三次重启/服务端重启、身份保留、第三节点拒绝、正常关闭与凭据清理。helper 除本机 fixture 外，仅额外允许 `https://mk.fengmap.com:8443`，保持证书验证。见 [公网验证记录](public-server/README.md)。下方本机 fixture 边界仍适用于本机脚本；公网通过不等于设备码产品或直连穿透验收完成。

这是独立测试程序，不进入 RemoteController/RemoteAgent 安装包，不采集屏幕、不注入输入。
客户端内嵌 tsnet，注册到本机 Headscale；另一个本机进程提供 TLS DERP。没有 Tailscale 账号登录或公共 DERP 配置，系统已安装的 Tailscale 状态不参与实验。

固定版本：Go 1.26.8，Headscale 0.29.4，tailscale.com/tsnet 1.102.5。`go.mod`/`go.sum`固定 Go 依赖。
Headscale 发布说明中最低客户端为 1.80.0，但实际兼容性须以本实验结果为准。

## 本机运行

Mac/Linux 本机脚本需要 Python 3、上述 Go 和 Headscale，使用 Unix 管道与短 Unix socket 路径。Windows 使用独立 PowerShell/Docker 入口，见 [Windows 实机验证](WINDOWS_TEST.md)，无需 Go/Python/.NET SDK。

在仓库 `artifacts/connection-poc/toolchain/` 放置官方 Go 解压后的 `go/bin/go` 和 Headscale 二进制。当前 Mac Intel 二进制命名 `headscale-0.29.4`；Linux 则用同版本官方 Linux 二进制放到同一路径。首次下载后按官方校验清单验证 SHA-256；工具链不提交。

本机已核对：

- Go `go1.26.8.darwin-amd64.tar.gz`：`186be014105aa6542b767d2c6ed5cca10a0214bdff809ef1724022a8c7894150`。
- Headscale `headscale_0.29.4_darwin_amd64`：`06e4c94a8b9397ed8c2714a4cd484c998604dc884e9b5d4a186aef05f14047b1`。

依赖首次下载可按本机代理设置 `HTTPS_PROXY=http://127.0.0.1:1080`，它仅用于工具链下载，不是连接架构依赖。然后在仓库根目录执行：

```sh
zsh poc/selfhost/verify.zsh
```

仅检查跨平台构建可运行 `zsh poc/selfhost/cross-build.zsh`；它不会运行目标平台程序。

如 Go 位于别处，可设置 `GO_BIN`。缓存和构建产物都在 `artifacts/connection-poc/`。
测试开始前要求 18443、18444、19090、15443 本机端口空闲；不自动停止已有进程。
临时节点凭据、Headscale 数据库和会话 token 由 smoke 创建，管道传递注册信息，退出后仅清理本轮创建的临时目录和进程。输出固定 PASS/FAIL，不显示密钥。

## 实验边界

- Headscale HTTP 仅绑定 loopback，不能把此配置搬到公网。
- DERP 仅绑定 loopback，TLS 使用每轮临时证书的 SHA-256 pin，没有关闭证书验证；仅供本机 fixture，不是公开中继服务器。
- 测试 ACL 只允许新数据库分配的 `.2` 节点到 `.1:47476`；第三台 `.3` 必须拒绝。它不代表多租户动态授权已经完成。
- helper 仅传随机会话 token 并接收固定字串，不是 TLS 产品桥接。会话 token 不是用户验证码，固定载荷协议也不是正式远控认证协议。
- 重启沿用同一节点状态，避免新增身份；完整产品仍需专用凭据存储、撤销、验证码后端和本地桥接访问控制。
- 强制中继使用固定版本的调试开关，仅用于证明 DERP 路径，不写入最终用户配置。没有模拟真实 NAT/运营商，也不代表跨网验收。

## 生命周期修复与回归

本机已证明免浏览器注册、自建 DERP 固定数据、错误 token 拒绝和第三节点 ACL 拒绝。Go 单元测试与依赖校验通过。Windows/Linux/Apple Silicon helper 已交叉编译；实际运行未验收。

首轮退出阻塞已定位到强制中继调试开关 `TS_DEBUG_ALWAYS_USE_DERP`：固定版本的 `bindSocket` 在调试分支替换模拟连接，却没有关闭原模拟连接，WireGuard 的接收线程因此仍等待在 `blockForeverConn.ReadFromUDPAddrPort`，关闭线程等待 `closeBindLocked`。现改为 `TS_DEBUG_NEVER_DIRECT_UDP`，保留真实 UDP socket、禁止直连探测；仍须由状态确认数据经过本项目 DERP。没有修改第三方库或放宽 TLS/ACL。

客户端保留身份重启时仍曾出现 `dial_failed`；关闭问题修复后可单独复现。禁用 `TS_USE_CACHED_NETMAP` 后重连恢复。该开关同时禁用磁盘网络图恢复和关联的 TSMP disco 广告路径，目前证据定位到这组缓存功能，尚不能断言其中某条内部路径是唯一原因。本 PoC 是在线模式，要求本轮控制面提供网络图，不承诺离线缓存启动；不删除节点身份、不重复注册。拨号前还检查目标存在于当前网络图，未知目标直接失败，避免 `tsnet.Dial` 回退到系统网络。

默认 `verify.zsh` 现包含客户端重启、服务端重启后真实数据交换、原地址/节点数不变及正常关闭门禁。构建完成后在 `poc/selfhost` 运行 `python3 smoke.py --check-reconnect`，增加连续三次客户端重启；每次均检查固定数据、错误 token 拒绝和 `closed`/零退出码。8 秒 Close 期限仍是失败兜底，不把超时退出算通过。全部临时 fixture 在退出后清理；不接真实键鼠、不宣称最终架构或跨网已验收。

2026-10-05 修复后：依赖校验、Go 单元测试、默认 smoke 与连续三次重启 smoke 均零退出通过；Windows x64、Linux x64、Mac ARM64 helper 重新交叉编译通过，目标机实际运行待验收。

2026-10-06 Windows x64 实机：PowerShell 5.1 + Docker Desktop Linux 引擎中运行独立测试包，完整脚本零退出并输出 `PASS windows_smoke_complete`；连续三次客户端重启、服务端重启、固定载荷、错误 token 双端拒绝、第三节点拒绝、身份/节点数不变、正常退出及 fixture 清理均通过。现有系统 Tailscale 保留；SSH 仅用于管理/传包，本轮不是双机跨网证据。Linux 与 Mac ARM64 实际运行仍未验收。

构建 Windows 内部测试包：`zsh poc/selfhost/package-windows.zsh`，生成唯一 ZIP、SHA-256 清单及内嵌依赖许可证文本，不携带节点状态或 key。Windows 通过私有 stdin 一行 JSON 引导，再通过 `stop`/管道 EOF 停止；默认 Mac 引导协议不变。Mac 可用 `python3 smoke.py --check-control` 验证同一控制管道和连续重启，本轮通过。

许可证依据：[Tailscale BSD-3-Clause](https://github.com/tailscale/tailscale/blob/v1.102.5/LICENSE)、[Headscale BSD-3-Clause](https://github.com/juanfont/headscale/blob/v0.29.4/LICENSE)。内部测试 ZIP 含 Go helper 与编译依赖根目录 LICENSE/NOTICE 等文本，Headscale 镜像由测试环境获取；不作为签名产品发布。对外发布前仍需审查第三方声明、依赖许可证及品牌使用。
