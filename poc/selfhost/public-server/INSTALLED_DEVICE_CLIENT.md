# r17：已安装信任设备 API 客户端（开发与构建通过，原生组合待验收）

`cmd/installed-device-client` 是应用内的专用私有 IPC 子进程，不是让用户直接在终端操作的产品界面。设备结果含验证码或grant，只在可信本地管道传递，不作日志/诊断输出。

## 信任与能力

- 编译固定 release root/service/floor/channel/journal helper 摘要及 signer helper 摘要。
- 启动仅 `Open` 已有原生 journal，通过 clientowner 的 current durable profile 创建 HTTPS 客户端。不存在 initialize/repair/upgrade/下载信任配置路径。
- 首帧只接受 `open`、本机身份的 scope/expected public；固定同目录签名helper只重开既有OS身份，不创建/删除/导出设备key。
- 不接受 Endpoint/Certificate/Issuer/ReleaseRoot/ServiceProfile/MinimumEpoch/AppKey/VaultExecutable/CreateIdentity/Grant/Profile 等运行时输入。sidecar、环境变量、父进程不能替换编译信任。scope和public仍来自可信本地应用身份管理，不是公开对端字段。
- 直接访问已安装发布签名 profile 的 HTTPS endpoint，沿用独立CA/域名校验、禁代理/重定向及签名nonce/时间协议，不内建SSH命令或本机转发。不代表远端端口已开放。
- 仅设备注册、invite/claim/incoming/approve/status/revoke，不支持enroll/bind/watch/TLS/HMAC投递。nonce与时间由客户端生成；父帧提交nonce/time拒绝。明确approve是可信本地应用操作，GUI人工确认仍未接。
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

先补新入口与原生安装存储/OS signer联合的专用验收（父进程私有管道，不打印注册码/grant），再设计长期服务的受限直接HTTPS入口、operator opt-in和公网负向/预算/故障验收。不能直接把既有 `--private-network` 反代或放宽为公网。实际开放端口/部署前明确检查现有服务与签名配置有效期、备份/回退和用户授权；不改业务443。随后接应用自主enroll/nodeproof/binding与受保护HMAC供应，最后GUI/安装。整体切片4未完成。
