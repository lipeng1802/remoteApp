# r28 原生节点与真实 Headscale 注册 / 回收

这是 **Mac 管理员验收工具**，不是最终客户端的凭据供应架构。
普通 device-client 的 provision、GUI、安装包、正式签名/信任供应均没有在这里启用。
服务器仍保留 r22 长期 verifier、database 模式、零 ACL 和回环设备 API。

## 真实路径

`cmd/enrollment-headscale-validation` 显式 `--run-scoped-headscale` 才运行。
固定已知 SSH 主机/密钥路径、严格 known_hosts 和固定服务器 CLI；无新上传文件、服务重启、
策略写入或端口开放。真实 NativeEnrollmentFactory 使用原来的固定 HTTPS controlURL。

1. 只读记录原有一个用户/节点/密钥的稳定字段及零 ACL，核验正式 Store 固定摘要。
2. 新建独立随机 TEST scope / ticket；在每个副作用前 fsync / 回读公开恢复记录。
3. 原生 TEST Initialize / Begin / offline key；真正调用 Headscale Driver.Mint。
   凭据经管理 SSH stdout 仅留在同一验收进程内，直接交 SDK Register，不经父 IPC、文件或日志。
4. 真实 SDK Up 与 Headscale Observe 的公钥/IP相同；原生状态 registered。
5. 真正同步 SDK Close 后才释放原生 lease；重开验证结果、拒绝 Begin / 第二次 factory 注册。
6. 仅 expire/delete/destroy 此工单的已确认密钥/节点/用户；复核原有稳定资源、零 ACL、正式 Store。
7. SDK 实际 join 后精确清理本轮 TEST state/log/native record/已知空缓存目录；保留公开恢复审计。

Guard 禁止 policy set/check、服务命令、未知参数和外来 ID；删除前重新核对资源归属。
未知写入回执后 fence，不重试 Mint 或自动猜测清理。未回收工单使原有一资源基线不成立，
下一轮拒绝开始；不能靠换 scope 绕过失败。

SSH 子进程没有 CommandContext 强杀；传输失败不被解释为远端副作用已完成。
SDK 没有强杀/超时即成功；秘密字符串清空不代表 Go/SDK 所有副本物理擦除。
管理通道读取 provider keys 的原始 JSON只在进程内，Snapshot 只把摘要写审计，不输出数据。
基线排除 online/last_seen 动态字段；不是宣称整个 Headscale 数据库字节未变化。

## 实机发现与修复

- 远端归属检查会增加签发时间，原本适用于本地 CLI 的一秒期限余量不足。
  Guard 在检查完成后只缩短请求 duration，额外留五秒，并保留 Driver 的实际绝对期限校验。
  首轮 Mint 校验失败，没有 SDK 入网，不能计 PASS；确认 SDK 已 join 后显式按工单回收。
- 成功登录的 pinned SDK 即使禁用缓存使用，也会创建 `profile-data/<current-id>/netmap-cache`。
  清理只允许 state 当前 profile 所对应的最多一组、**空** cache 目录，按底向上精确删除。
  profile ID 从 existing state 只读取得；不接受外部 profile ID、不新建/repair store、不读出私钥。
  任意缓存内容、额外/不匹配 profile、未知文件/链接/不安全 OS 权限均在删除前拒绝。
  第二轮真实入网与远端回收已通过但本地清理失败，仍不计整体 PASS；精确清理恢复后再复测。

目录检查沿用 macOS UID/0700/0600/no-link，Windows current SID/protected DACL/no-reparse。
SDK private key 仍在权限保护文件；不是额外加密/全部 Keychain，也不抵抗恶意同用户/管理员。
只有 TEST cleanup，生产没有 reset/delete/retry 接口。

## 已 join 工单的显式恢复

公开恢复记录含 ticket/scope/public key/句柄/原始稳定基线摘要/实际 SDKJoined，没有凭据。
失败不自动删除材料；记录本身是本地管理员审计，不是签名的产品授权。

```sh
./enrollment-headscale-validation --retire-joined-test /absolute/path/headscale-sdk-r28-<random>/public-recovery.json
```

仅接受本程序旁的规范路径、无 symlink、已确认 join 的已知阶段；先原生排他重开/公钥匹配，
再对照原资源基线。未知副作用阶段不接受恢复，需进一步人工管理核查，不能强删锁。
远端已经回收且只剩本地清理时，仅核验当前基线，不再次远端删除。
`RECOVERY PASSED` 只代表恢复收口，不是注册验收。

## 验收边界

最终必须完整 `VALIDATION PASSED: native TEST journal + actual Headscale Mint/SDK Up/Observe ...`
并 actual exit0。Mac 真实验收，Windows 仅新存储测试构建/静态检查，没有桌面或双机结果。

**这里依赖私有管理 SSH 签发测试凭据**，没有 OS signer 签名 grant_clock/enroll、正式 authority
Store 批准请求、实时 nodeproof/bind/watch、产品 TLS/HMAC、GUI 或签名安装。
控制面 Observe 不是实时持有证明，零 ACL 回读不是实际双机数据面拒绝。
不能把 r26 的替身 HTTPS 与此工具拼成产品端到端验收。
下一项应将真实 credential / SDK / 持有证明接入固定原生签名 HTTPS 组合，解决 verifier 可信供应；
保护长期资源，不复用旧要求空控制面的 enrollment-network.py。
