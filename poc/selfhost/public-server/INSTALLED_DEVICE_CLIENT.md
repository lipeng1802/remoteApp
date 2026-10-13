# r17：已安装信任设备 API 客户端（开发与构建通过，原生组合待验收）

## r23：直接签名网络授权门禁（开发/本机HTTPS通过）

新增后续私有帧 `Op=network`，仅允许 `Network.Action=grant_clock|state` 及签名 `Network.Grant`。open/stop/device不允许混带network操作；nonce/time/challenge/ticket/intent/certificate/proof由父帧注入均拒绝，enroll/bind/clock/publish_tls/tls_identity仍不开放。零Network字段在JSON输出中省略，保留旧设备帧形状。首帧仍禁止grant/网络操作和运行时根/地址覆盖。

客户端从当前安装profile固定issuer、捕获OS signer公钥/自身SHA身份，先验证grant签名/规范ID/独立target-controller/自身主体。签名结构在issuer颁发时点验证，并不据此承认仍在线有效：grant_clock由客户端生成32字节新挑战，5秒工作预算/完整单调RTT计入AuthorityClock；核验issuer签名/本次nonce/grant/颁发时间，再用该clock重验grant当前期限。它不能绕过设备请求时间窗或安装profile到期，不承诺任意系统时钟偏差修复。

state只允许已校时的同一grant，验证issuer签名、完整grant、合法绑定与严格3秒租约，并重验grant有效期；签名前后公钥改变、取消、超时、旧nonce proof、错误issuer/签名或混入credential/ticket/TLS字段均锁闭。真实签名inactive可报告但随即结束当前owner；HTTP/解析/验证失败同样结束，不自动重试。签名器CloseJoined后才释放owner资源。保留原device API请求失败处理，网络授权错误不沿用其“可继续发下一设备请求”行为。

父端只收到固定 `status=network_authorization`、`phase=grant_clock|state`、passed/active布尔值；不返回clock proof、签名state、绑定、ticket、注册credential或HMAC。clock passed不表示网络active，active观测也不是数据连接能力，不能交给另一runner作为可信授权。未实现连续watch、自动enroll/bind、节点Up、TLS/HMAC/GUI。

沿用原生入口的输入结束握手：父端发送stop，或收到网络终止结果（passed=false，或phase=state/active=false）后，应立即半关闭专用stdin，再等待final failed/closed与实际进程退出。不要等最终failed/closed后才EOF。Runner的终止/join测试不等于该新增网络错误的完整原生子进程退出验收，后续专用fixture必须覆盖这条路径。

NetworkClient两类HTTP回复共用既有严格JSON解析：嵌套重复/大小写重复键、未知/尾随字段拒绝，保留body限制；不以TLS200或reply shape代替签名校验。

证据：授权门禁7项（含真实本机HTTPS Runner测试normal/inactive、内存签名器、显式join屏障）与HTTP严格回复测试通过，相关5包race/vet通过。初始测试fixture grant域名前缀写错导致测试失败，按现有invite-grant协议纠正后重跑；未修改协议常量或放宽门禁。

fresh Mac/Windows TEST包 `artifacts/connection-poc/installed-device-network-r23-mac|win` 固定编译信任/同目录helper pin/manifest构建通过，仅build-only，未执行/上传/安装。其中validator仍是r18设备注册/邀请基线，**不验证新增grant_clock/state原生组合**；需要下一项专用原生授权fixture，不能直接把旧validator成功标记当网络授权通过。本轮Windows关机，服务端未改，整体4未完成。下一步把校时门禁与helper内部受控enroll/注册凭据消费相连，再接持有证明/bind/watch与产品凭据，不能将secret返回父端。

`cmd/installed-device-client` 是应用内的专用私有 IPC 子进程，不是让用户直接在终端操作的产品界面。设备结果含验证码或grant，只在可信本地管道传递，不作日志/诊断输出。

## 信任与能力

- 编译固定 release root/service/floor/channel/journal helper 摘要及 signer helper 摘要。
- 启动仅 `Open` 已有原生 journal，通过 clientowner 的 current durable profile 创建 HTTPS 客户端。不存在 initialize/repair/upgrade/下载信任配置路径。
- 首帧只接受 `open`、本机身份的 scope/expected public；固定同目录签名helper只重开既有OS身份，不创建/删除/导出设备key。
- 不接受 Endpoint/Certificate/Issuer/ReleaseRoot/ServiceProfile/MinimumEpoch/AppKey/VaultExecutable/CreateIdentity/Profile 等运行时信任输入。r23仅后续network帧内接受经过校验的签名Grant，不接受首帧/top-level Grant。sidecar、环境变量、父进程不能替换编译信任。scope和public仍来自可信本地应用身份管理，不是公开对端字段。
- 直接访问已安装发布签名 profile 的 HTTPS endpoint，沿用独立CA/域名校验、禁代理/重定向及签名nonce/时间协议，不内建SSH命令或本机转发。不代表远端端口已开放。
- 设备注册、invite/claim/incoming/approve/status/revoke与r23读授权门禁，不支持enroll/bind/watch/TLS/HMAC投递。nonce与时间由客户端生成；父帧提交nonce/time拒绝。明确approve是可信本地应用操作，GUI人工确认仍未接。
- `ready` 只表示安装配置与签名客户端已就绪，不证明服务器可达。API拒绝仅输出固定passed=false；不回显异常、原始HTTP响应或密钥。

## 生命周期

Owner租约覆盖HTTPS与签名helper。请求有期限，安装配置有单调到期期限；显式stop/取消必须实际退出Runner，关闭HTTP客户端并 `CloseJoined` 签名进程，随后native journal Close及读取线程join后才输出closed。到期、无效本地命令、EOF/坏帧、签名join失败锁闭/非零退出；不把cancel或强杀当正常join。没有自动重连、重新授权或原生状态修复。父进程不能在未知退出时删除凭据。

## 构建与证据

`trustinstall.CompileDeviceClient` 固定编译入口；`installed-session-build` 新增 `darwin-device-test` / `windows-device-test`，必须无产品参数。测试channel随机且Test=true，不复用生产命名空间。已有desktop/product构建路径和12文件borrower manifest未改。

本机构建检查脚本 `artifacts/connection-poc/build-installed-device-r17.py` actual exit0，两平台固定配置产物和精确5项manifest/TEST namespace PASS：

- `artifacts/connection-poc/installed-device-r17-darwin`
- `artifacts/connection-poc/installed-device-r17-windows`

上述仅build-only，没有initialize/install/open原生journal或OS设备身份，没有运行新产物、上传Windows包或改任何服务器监听/服务/Store/Nginx。不要直接运行它们代替正式安装/原生验收。

Go相关7包race与专用vet PASS；新增5个Runner测试包含真实本机独立HTTPS签名注册/邀请、profile endpoint/issuer/signature篡改拒绝、严格操作门禁、stop等签名进程实际join、join失败不成功、空闲cancel正常join/到期失败。签名身份使用test-only memory signer，服务使用临时隔离issuer/CA，未宣称OS signer+native journal+新入口组合已经通过。命令2项及构建1项回归覆盖空编译信任、运行时信任/私钥注入拒绝、device-only构建拒产品参数。既有Python78项PASS。

## 下一步

r18专用[原生联合验收](INSTALLED_DEVICE_NATIVE_VALIDATION.md)已完成Mac实际新命令/native journal/OS signer/HTTPS/稳定重启/清理，用户确认Windows普通桌面返回完整预期VALIDATION PASSED，双平台闭环。可信parent发送终止stop后须半关闭专用输入再等待closed/actual exit0，不能保持输入开启并与读取线程互等；EOF不代替实际join或成功回执。随后设计长期服务的受限直接HTTPS入口、operator opt-in和公网负向/预算/故障验收。不能直接把既有 `--private-network` 反代或放宽为公网。实际开放端口/部署前明确检查现有服务与签名配置有效期、备份/回退和用户授权；不改业务443。再接应用自主enroll/nodeproof/binding与受保护HMAC供应，最后GUI/安装。整体切片4未完成。
