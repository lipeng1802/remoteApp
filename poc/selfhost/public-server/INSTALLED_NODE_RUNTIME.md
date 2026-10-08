# 4C-4Q：编译锚客户端入口与节点运行时

开发接线已完成，Mac/Windows运行时局部测试通过，compiled两端包构建通过。**实际Headscale节点/OS signer/watch/Swift-Schannel进程组合尚未运行；不算双机验收闭环或整体切片4完成。** 不要执行旧r3脚本来代替这一轮，也不要直接运行新helper来初始化正式信任。

## 独立入口

`cmd/installed-session` 只能由可信编译步骤注入embeddedRelease和embeddedSignerDigest；普通空参数go build产物锁闭。root/service/floor/channel/journal摘要、bundled签名profile和签名程序摘要不从stdin/env/sidecar取得。运行只open已有journal，不create/reset，不创建/删除OS设备身份。两个helper来自同目录，摘要匹配才启动；这仍是内容完整性，不是正式发布者签名或同用户恶意替换防御。

通过专用父子stdin/stdout pipe运行，首条 `start` 含Config、VaultScope、VaultExpected；Config是StateDir/AuthKey/Verifier/Target/Controller/Grant/Binding/Server。不接受旧helper的AppKey、ReleaseRoot、CA、MinimumEpoch、ClockProof或parent state推送。私有authkey仅在stdin内存，不回显/日志/写build-public。

Config的StateDir必须是已显式准备的规范绝对目录、非symlink且存在有效大小的普通tailscaled.state文件。运行时不会初始化缺失节点状态；已准备state依照固定ControlURL读取，节点公钥必须匹配角色binding。**节点文件本身需可信供应和OS用户目录保护/独占；journal lease不是该路径的ACL，也不能阻止旧helper同时操作同一StateDir。协调端必须先停止旧helper并等待退出，不复制/覆盖已用节点state。** 首次注册/enroll/bind不由此入口完成。

## 实际资源接线与退出

1. 安装profile及grant验签、角色设备公钥检查，再用编译摘要固定的签名程序重开OS signer。
2. 真实HTTPS authority clock校准和自主签名watch取得ACTIVE绑定后，才调用实际tsnet.Server.Up。固定ControlURL为 `https://mk.fengmap.com:8443`，没有系统tailnet登录或任意控制服务器回退。
3. Up返回的self node key及overlay IP必须匹配已签名watch的角色binding，然后启动nodeproof与TLS桥接监听。目标端overlay47476→固定本地产品端口47478；控制端临时IPv4 loopback端口→overlay，继续由现有Gate进行WhoIs/互证/短lease检查，不采用裸IP授权。
4. nodeproof新增ServeBoundWithSigner，挑战只能引用当前grant ID；其他grant、非规范ID拒签。保留verifier节点和实际WhoIs/self/IP校验，nil WhoIs/Node响应拒绝而非panic。nodeproof监听关闭监视器也加入join。
5. cancel/fault先Deny，关闭listener并等证明/流workers；watch/work都join后关闭HTTPS，再同步tsnet.Close并检查错误，最后close signer。Owner在整个runner返回之前持有journal；不以后台close goroutine或等待超时冒充资源已关闭。

命令后续只接受stop或已签名profile的upgrade。upgrade成功后退出，必须显式启动新进程，不自动重连。无效命令/候选导致整个命令锁闭并关闭旧会话（Owner API的“无效候选不停止旧会话”并不代表命令会在协议错误后继续服务）。EOF/信号取消、关闭stdin并join读取线程；owner.Close有8秒调用期限，未join成功返回failed，不能输出closed。专用pipe读写契约不等于直接交互控制台或OS硬kill耐久保证。

**当前拥有的是TLS桥接stream，不拥有独立Swift/C# TLS应用进程及其证书/HMAC供应。** 产品进程协调、安装信任初始化/升级供应、首次设备接入与GUI仍需下一段，不能把该Runner当作都已实现。

## 构建和证据

`cmd/installed-session-build` 参数依次为：Go绝对路径、module绝对路径、darwin/windows、新目录、可信公开anchor、可信公开profile、trusted journal binary、trusted signer binary、channel。拒已有输出；验证时效，复制两helper，编译installed-session和trust-install，写只用于追溯的build-public.json。runtime不读取该文件。无现有发布root/CA私钥读写或签名。

本轮用现有prerelease-v1/client-public公开材料、现有Mac/Windows helpers构建：

- `artifacts/connection-poc/installed-session-mac-4q-ready`
- `artifacts/connection-poc/installed-session-win-4q-ready`

返回BUILD_READY。**未执行真实channel的trust-install initialize/upgrade，未在这些包中复制节点状态、grant、authkey、OS私钥；未运行compiled客户端会话。**

Mac全量race和两平台全量vet通过；4项Runtime本地测试：listener/proof/stream真实join、proof故障关闭、缺失状态不初始化/不打开signer、错signer摘要拒启动。普通未编译命令锁闭测试及bound grant单元测试通过。新增runtime测试在真实Windows SSH下全部PASS/退出0，无SKIP，但只用本机socket和隔离文件；不访问native凭据或线上Headscale。exe摘要 `014856b0c9c2b2667e1dcb9cecbfbe66f68c435b6d4dac05bc6044406d1c9430`，Windows位置 `C:\Users\jarvis\remoteapp-clientruntime-tests-4q-20261008.exe`。

未改线上authority/Headscale/Nginx/ACL、旧GUI/DMG/Setup或现有设备身份；旧4C-4P两端通过证据仍有效但不冒充新组合验收。只本地提交，不推送。

## 下一步

补双机协调入口：隔离可信配置安装、正常桌面Windows journal/signer进程所有权、旧节点helper精确停止、重开已bound节点、TLS描述符/证书私钥证明和HMAC供应、实际产品进程启动/退出。再验正常TLS、OS撤销/watch故障、升级前旧节点/流/签名进程全join、新profile显式恢复和最终清理。需要桌面操作时提供新入口，不要求用户重复旧包，也不使用SSH凭据库失败充当桌面结果。
