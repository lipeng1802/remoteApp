# 切片 4C-3A：持久 verifier 生命周期与真实 Headscale 验收

本项完成独立的 verifier 持久身份管理及服务器实机验证，**不是设备服务已经接通网络授权**。`remoteapp-device` 仍 device-only、`/v1/network` 404，整体切片 4 未完成。没有新增公网入口、业务 443 改动或长期 root HTTP 服务。

## 新入口

`verifier.Open` 和服务器本地 `verifier-owner create|resume|retire <绝对目录>`。目录与文件分别 0700/0600，拒绝父路径符号链接、已有目录 create、宽权限或损坏/未知字段/重复字段 journal。固定独立 Headscale 控制地址 `https://mk.fengmap.com:8443`。节点钥匙只在该目录的 tsnet StateStore，不使用系统 Tailscale 登录，不输出私钥、注册密钥或底层错误。

首次 create 先生成离线 node identity，持久 journal planned/ticket，再 Mint 单次最多 120 秒注册密钥，确认句柄后写 issued。注册密钥只驻留进程内存，不写 journal/argv/日志；不宣称 Go 字符串能可靠清零。tsnet 首次 Up 后验证实际 self key/IP 与 Headscale 的确切 user/node 观察，再写 ready。注册密钥到期不等于已注册节点到期：长期节点依赖真实 Headscale/tsnet 有效身份，过期/重登录失败拒绝，不静默再次 Mint。

resume 必须 journal 与已有 tailscaled.state 同时存在，重开同一个 node key；不用新 AuthKey、不 Mint；ready 恢复还要求原 user/node/IP 绑定不变。planned/unknown Mint、缺失/损坏身份、已退休状态拒绝，不自动重建或换控制服务。重建/更换身份属于独立显式管理员流程。

owner.lock 是独占 CreateNew 锁；正常 Stop/join 后才释放。Stop 先将 Live/Info.Healthy 置 false，再取消监控并关闭 SDK；超时不放弃锁、不允许另一实例抢占。崩溃遗留锁不自动接管，必须管理员核对原进程已退出、资源及 journal 后显式恢复；本项没有完成掉电/崩溃恢复自动化。

周期串行监控有期限的本地 SDK Running/health/self key/IP 与 CLI 绑定观察；异常永久关闭、join，不自动重连、恢复 Live 或 Mint。**这些健康与控制行信号不是当前应用/远程节点持有证明，也不能把正常 status/旧 ready journal 当授权。** 后续实际 Bind 必须继续使用 nodeproof 新鲜挑战与设备签名；控制断连到 SDK health 更新的延迟没有被宣称为严格 1 秒控制新鲜度。

retire 要求独占锁，按本机可信 journal 的 ticket 调用 driver.Cleanup；未知 Mint 可按 ticket 名发现并回收，不能捏造 provider IDs。成功写 retired，重复 retire 幂等；保持其他资源不动。此模块从不调用 ReplaceRules/Verify，不投影 payload ACL。未来接入 backend 后，必须先停止/fence backend，再 retire verifier。

## 验证结果

- Go 全量 race、专用 vet 与 Linux amd64 静态构建通过。
- 回归：稳定重开/不再次 Mint、不保存注册密钥、单实例/活动时拒绝 retire、未知 Mint 不重试及按 ticket 回收、故障 latch/关闭、不自动恢复、变更绑定/缺失身份拒绝、不 rekey，Stop 超时保持锁直到 SDK join。
- 实际服务器首次 create：READY 后正常 TERM，STOPPED，退出码 0；resume 不 Mint，READY/STOPPED、退出码 0，journal 哈希前后相同。幂等 retire 两次均通过。
- 第二个隔离测试身份首次正常启动/停止，然后 resume；只删除 journal 中该测试节点的真实 Headscale NodeID，运行中的 verifier 返回固定 FAIL、退出码 1（预期故障关闭，未自动重建），锁正常回收。最初 30 秒测试窗口正常结束、删除前置锁检查失败，无删除发生；改为 resume 后在足够窗口完成测试，不能把前一次未注入的流程当 fault 通过。
- 故障后精确 retire，通过独立 CLI 观察 nodes/users/preauthkeys 均 0。Nginx 基线哈希全通过，remoteapp-device/remoteapp-poc 均 active；没有 ACL 变更。
- 正常回收后删除本轮两个 root-only 隔离测试目录及工具临时目录（随机测试节点私钥删除，不可复用；不涉及既有产品/预发布服务密钥）。最终没有长期运行的 verifier，不把 smoke 当正式部署完成。

## 下一步权限边界

设备 HTTPS frontend 保持 remoteapp-pre 无特权，不能给它 Headscale DB/root 或通用命令执行权限。下一子切片设计并实现固定操作的 server-local broker：受 OS peer credential 保护的 Unix IPC，单一权威 Store owner，转交原始设备签名请求并由 authority 再验签；请求不能变成任意 Headscale 参数/路径/脚本。broker 持有 verifier/维护任务，故障先 fence 发行/状态，再关闭工作者，安装前完成负向测试。设备公钥的可信持久绑定也需补齐，不能从 deviceID 散列反推、信任父进程提供的任意公钥。

之后在隔离环境接真实 backend/新鲜 nodeproof、故障撤销与零 ACL，再更新部署，仍先回环验收；公网开放、安装根/floor 轮换恢复、HMAC 安全投递、GUI/安装仍待。
