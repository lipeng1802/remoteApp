# 4C-4R：双机协调的专用管道、进程对与启动顺序

新增 `processowner`，为下一段真实协调提供可复用的本地进程管理组件。**不是完整双机协调命令，尚未将现有 split-consent.py 切换到 compiled installed-session，也未启动真实 Headscale/OS signer/Swift-Schannel 组合。** 本轮不需要用户重新 READY 或运行旧验证包。

## 管理边界

`Child.Start` 只运行可信本地调用方给出的规范绝对路径、SHA-256 固定的可执行文件；不经过 shell。摘要不是发行者签名或同用户恶意替换防御。参数不得包含秘密；authkey、HMAC 和启动配置只可通过继承的匿名管道传递，不记录 stdout/stderr。子进程仍继承本地环境，调用方必须使用受控环境和专用桌面启动器，不能把此组件当沙箱。

`Exchange` 使用 8KiB 严格 JSON framing，拒绝重复键、截断、超限；回复队列上限 16。读取超时关闭 stdin、锁闭后续请求，不能将迟到响应当作新请求授权。原始回复只交给可信回调，产品 ready 可能含测试 HMAC，不得打印或写交接清单。协议/输出故障不会被当作正常关闭。

`Stop` 只发一次停止请求；成功必须同时满足唯一 closed 回复、子进程零退出、stdout EOF、读取线程 join。收到 closed 但进程继续运行、非零退出或异常输出均不算成功。超时保留 Child 与未完成 join；重试只等待同一次退出，不 kill、不假报关闭、不自动重启。子孙进程若保留 stdout，EOF 屏障继续等待；**没有 OS Job Object/进程组封闭，也不能据此证明所有任意后代均已退出。**

`Pair` 成对管理已经启动的 bridge/product。任一进程退出或输出管道故障触发另一端停止；正常关闭并发停止双方，防止一端卡住导致另一端继续服务。双方均 join、监视线程 join 后才关闭 Pair.Done；异常触发保持失败，不因另一端正常停止而改报成功。该库没有自动 enrollment、信任重置或凭据清理。

## 启动适配

`StartEndpoint` 接受可信本地 `Plan`，不接受网络请求。两种顺序：

- Target：产品启动并取得私有 ready → PrepareTarget 回调完成证书持有证明发布/校验 → compiled bridge start/ready(port=0) → Pair。
- Controller：compiled bridge start/ready(合法 loopback port) → ControllerConfig 回调取并验证新鲜签名 TLS 描述符/组成私有配置 → 产品 start/ready → Pair。

回调是同步可信代码，必须遵守 context；本轮没有实现真实 OS API 的证书发布/描述符获取适配。产品 ready 本身只检查状态，不等于证书、HMAC、身份或 TLS 会话已验。调用方仍须校验真实产品字段与绑定。启动失败 best-effort 等待 8 秒关闭已启动进程，并始终返回包含部分 Child 的 Launch；如果未 join，调用方必须保留它，不能删除节点状态/凭据或重新注册。后续 Close 可继续等待。

**Pair 不持有 native trust journal。** 现有 installed-session 在自己 runner join 后就关闭其 journal；独立产品进程可能晚于它退出。因此不能把 Pair 的最终屏障冒充“journal 租约已覆盖产品整个生命周期”。下一段必须把产品所有权与退出屏障移入编译客户端 owner 的 runner，再进行活跃升级/故障验收。

## 本轮验证

10 项行为测试使用真实测试子进程和匿名管道：正常停止/拒复用；closed 后延迟进程退出、超时保留/重试 join；closed+非零退出拒绝；读取超时锁闭；错误摘要和重复 JSON 拒绝；bridge 崩溃停止 product；正常 Pair 双端/监视线程 join；Target 准备顺序；Controller 端口后配置顺序；Target 准备失败只关闭已启动 product。

macOS 新模块 race、全量 race，以及 Mac/Windows 模块 vet 通过。Windows SSH 管理通道运行上述 10 项行为测试全部 PASS、退出 0、无 SKIP（另有 TestProcessFixture 自举入口，不计为第 11 项行为测试）。最终文件 `C:\Users\jarvis\remoteapp-processowner-tests-4r-20261008-final.exe`，SHA-256 `a98aac065996db0978ef8b3ea7065c4c02062a9dcf67e9e5b798246644ff3258`。这些是本机进程测试，不是凭据库、Headscale、真实 TLS 产品或安装验收。

本轮只上传独立测试程序，没有替换 Windows 安装、启动桌面凭据进程或真实节点；没有修改线上服务/Nginx/ACL，未读取发布 root/CA 私钥。README/HANDOFF 记录边界；完成后仅本地提交，不推送。

## 下一段（仍未闭环）

1. 将实际产品进程启动/退出接入 compiled owner，保证 journal 在旧产品、节点、watch、流和 signer 全部 join 后才释放/提交升级。
2. 实现可信配置供应及正常桌面 Windows 专用入口；替换旧 split 协调流程的 root/CA bootstrap，不通过 SSH 假冒桌面凭据验收。
3. 接真实 OS 证书发布/签名 TLS 描述符回调，执行新客户端双机正常 TLS、撤销、watch 故障、升级前 join、显式新会话及精确清理。

整体切片 4、GUI、正式签名安装包、公共接入与最终秘密投递仍未完成。
