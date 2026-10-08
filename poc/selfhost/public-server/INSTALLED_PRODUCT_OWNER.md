# 4C-4S：产品进程进入 compiled owner 的租约屏障

开发接线与两平台局部测试通过。**本轮没有运行新 compiled 客户端的真实双机 Headscale + OS signer + Swift/C# 会话，也未初始化真实 prerelease channel。** 整体切片 4 仍未完成；无需现在重复旧 READY/r3 验证。

## 同一 owner 的生命周期

`clientruntime.RunnerWithProduct` 在已提交配置的 runner 内部启动产品，不再由外部 Pair 提前释放 bridge 的 journal。ProfileSession 的签名校时/ACTIVE watch 仍先于实际节点 Up；随后固定摘要产品准备、证明/流监听与产品一起运行。`processowner.Own` 在取消、网络故障或产品退出时，取消网络 worker 并同步发起产品 Stop，等待两者实际 join 才返回。partial startup 失败也关闭监听器并无期限等待已启动产品的实际 Stop；外层 owner 的调用期限不能代替这个屏障。

随后 ProfileSession join watch/work、关闭共享 HTTPS client，runner 同步 node.Close，再 native signer.CloseJoined；全部返还才允许 Upgrade 提交或 Owner.Close 释放 journal。product closed 回复而进程未退出、非零退出等均失败，不能提交新 epoch。中途超时仍由 owner 保留 lease/锁闭；没有自动重开旧 gate 或重注册。

修正两个原有退出漏洞：ProfileSession 取消不再吞掉非取消类 worker 错误；installed-session 最终 Close 改为真实无期限 join，不会在 8 秒后进程退出而丢掉 OS lease。升级调用本身仍有 8 秒期限，失败后命令进入同一实际 Close 等待。parent 必须保留未退出 helper；不要强杀后将状态当作成功清理。

`privatepipe.Signer.CloseJoined` 使用可重复观察的同一个 Wait 完成信号，检查 closed 确认、非强制/非故障、进程零退出，并关闭输出句柄。legacy Close 的有界 emergency/kill 不是成功证据；新 runner 若经历它仍等待实际 Wait，并保持失败。NewSigner 启动失败也等待已启动 helper 实际退出，不丢失 owner。未引入 OS Job Object/进程组防护，不能宣称任意子孙或管理员硬杀已验。

## 真实 TLS 准备接线

新增 ProfileSessionWithClient，可信产品准备借用**同一个 compiled profile 验证后的 NetworkClient 和签名校准时钟**；不接受 parent root/CA/endpoint 或把 signer 交给产品。

- Windows Target：固定 sibling `tls-product.exe` 摘要启动；接私有 ready 中的随机测试 HMAC/指纹；产品产生 grant/target/nonce 绑定的真实证书私钥证明。helper 核对 DER 摘要与 ready 指纹，再通过 OS signer 的 publish_tls API 提交，authority 验 RSA-PSS 持有证明；重新请求并验证新鲜 issuer TLS 描述符、binding/challenge/期限/指纹一致。只有全部通过才输出私有 product_ready(Key/Fingerprint)。产品证书私钥不导出。
- macOS Controller：parent 只经匿名管道给测试 ProductKey（规范 base64、32 字节），helper 自行取 issuer TLS 描述符并验证签名/nonce/grant/binding/有效期，才将 loopback port/HMAC/已验指纹/target 通过匿名管道传给固定摘要 `tls-product`（Swift）。没有 parent 指纹覆盖或自动信任。

准备有 8 秒 context；任何失败仍等待已启动 child 清理。HMAC 继续是私有协调测试投递，**不是最终产品秘密供应**。product_ready 含秘密，只可在受控父子 pipe 消费，不打印/保存/转给公共 bootstrap。新协调端必须处理 target product_ready 后的 bridge ready 两条消息。普通 stdout 终端不是此协议。

## 固定构建入口

`CompileProductSession` 将 product SHA-256 也编译进 helper；不允许运行时选择 executable/digest/args。installed-session-build 保留原 9 个参数（不含命令名）的 bridge-only 构建模式；追加第 10 个参数为可信 standalone product 路径，复制固定 sibling 并编译 product pin，build-public.json 只追溯、不参与 runtime 授权。product 模式固定 Windows target / macOS controller，无法用 stdin 降为 bridge-only。

Windows 真实 TlsBridgeProbe 使用当前源码单文件 self-contained 构建（IncludeNativeLibrariesForSelfExtract）；不能只复制多文件 publish 的 apphost。`build-owned-tls-product.ps1` 先核对可信源码 archive hash，拒覆盖旧输出；没有安装、启动、凭据操作。

两端 build-only 包：

- `artifacts/connection-poc/installed-product-mac-4s`
- `artifacts/connection-poc/installed-product-win-4s`

用现有 prerelease-v1/client-public 公共 anchor/profile 和 helpers 构建，均 BUILD_READY；没有运行 trust-install initialize/upgrade 或客户端会话，没有复制节点 state、authkey、grant、OS key/证书私钥到包。Mac Swift 产品重新 release build；Windows 单文件产品重新编译并核对本机/远端 SHA-256 `2fe320ff54d78dc77f4d69142284197758d289781f88810debae2ffde7550d47`。Windows 仅保留独立源码构建目录 `C:\Users\jarvis\remoteapp-product-build-4s-20261008`；旧安装/节点没有改动。分发环境与依赖加载/发行者认证仍不由文件摘要解决。

## 已验证证据与未验证边界

macOS 全量 Go race、Mac/Windows 全量 vet、新增局部测试通过；另单独真实 Keychain/helper StrictJoinedReopen PASS/无 SKIP（0.26s），随机测试身份精确删除。全量默认未开其他 native 测试环境项，不冒充全部旧 OS 测试实测。

Windows 固定摘要入口 `C:\Users\jarvis\run-owned-product-validation-4s.ps1 -BundleDirectory C:\Users\jarvis` 已通过 SSH 实际整轮运行，11 项 top-level 必需测试全 PASS/退出 0/无 SKIP：

1. 内存安装 journal + 真实隔离产品子进程：升级等待产品与网络 join；Close 超时保留；产品非零退出拒升级（3 项）。
2. TLS 准备协议 fixture：真实 RSA-PSS 子进程证明、target 发布/描述符校验、坏证明拒秘密输出、controller 私有配置、四类描述符篡改及错 HMAC 在启动前拒绝（5 项，API/issuer 是内存 fixture，不是实际 authority/OS signer/产品）。
3. signer helper 实际进程 closed 后等待/非零退出拒绝（2 项，本地子进程 fixture，不是 Windows native 凭据）。
4. 实现级真实本地 HTTPS 校时/watch：取消不吞掉产品 join 错误（1 项，设备身份/授权是测试 fixture）。

首轮脚本因 PASS 正则漏转义在第一组后失败，未计整轮通过；修正为字符类匹配并全部重跑通过。验证脚本内固定四 exe 摘要、必需 PASS/拒 SKIP/拒空选择，不以输出末尾孤立 PASS 骗过门禁。源码见 run-owned-product-validation.ps1。

本轮没有改线上 authority/Headscale/Nginx/ACL、GUI/DMG/Setup，没有读取发布 root/CA 私钥。Windows native journal/signer 在正常桌面、新 compiled 客户端实际 node Up/TLS、活会话升级和进程故障的联合行为仍未验。只本地提交，不推送。

## 下一段

提供新的可信协调/Windows 桌面入口：隔离固定信任安装与设备身份 owner、准备/停止旧 node helper 并准确重开 bound state、新产品包上传核对、处理 product_ready 的测试 HMAC 私有转交。然后用新 compiled 产品模式验双机正常 TLS、OS 撤销/watch 故障、升级前全资源 join、显式新 profile 恢复及精确清理。不能继续用旧 split-consent.py 或以上 fixture 充当联合验收。
