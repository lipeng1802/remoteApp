# 4C-4I：独立进程产品 TLS（完整实机复测通过）

新增 `split-consent.py --tls` / `split_tls.py`。最新真实双机完整复测退出0，不使用旧fixture。整体4未完成。

## 实机记录

本轮Windows实际OS probe通过；archive SHA256核对、最新Windows65协议测试/publish通过（DPAPI明确SKIP）。真实OS consent/nodeproof及单向最小ACL后，同node/IP正常重开自主HTTPS helper，watch/gate active且parent state不能授予、无旧Token fallback。真实C#证书私钥证明由OS target签名发布，Mac fresh issuer描述符匹配真实证书。实际Swift/C#错pin、错HMAC、认证前PING均结束且零protected pong；正常同TLS连接跨初始lease5秒增加>10pong。OS owner撤销后双端watch done/join、Swift connection_closed非timeout/cancelled、C#/helper各关闭一次且没有后续pong。

首轮功能通过但回收提交竞态导致最终FAIL，不能算完整PASS：节点消失/零规则仍有一条cleanup_pending，终止daemon取消维护提交。保留实验目录后以原Store/verifier显式恢复，首Reconcile完成cleaned，正常停/join后精确Retire/原模式恢复/清理；未手动改日志状态。新增30秒只读cleaned等待屏障，让活维护完成记录再停止；超时/读取错误仍失败保留。Linux5项实际组件PASS无SKIP、构建/vet、Python4项断言通过。

修正后完整重跑全部PASS/退出0；所有helper/API/产品进程正常零退出，确认cleaned后daemon/frontend正常stop/join，verifier精确退役，本轮临时资源清理、原策略恢复、线上Store/Nginx/服务未变、控制资源零。Windows最终Enter/STOPPED待用户确认。

## 新链路

真实 Windows/Mac OS signer → 私有 HTTPS 非root frontend → kernel broker → 独立 root daemon + 真实 persistent verifier/supervisor；所有设备批准、enroll/bind、TLS发布及描述符仍走 OS 签名 API。复用 --nodes 的真实持有证明/错误身份拒绝阶段，不运行 enrollment-fixture。

TLS阶段正常停止旧固定载荷helper，同node key/IP重开为自主HTTPS模式。每个helper内负责签名时钟校准、状态watch、短lease与不可逆gate关闭。父进程无刷新线程、不代签、不通过state推送授权；明确检查parent state操作不能授权managed gate且没有旧Token fallback。

实际C# TlsBridgeProbe使用产品RemoteProtocol；Windows仅上传本轮源码archive并核对SHA256，在短期grant创建前运行65项bridge-validation与publish。DPAPI部分必须显式SKIP，不将65项协议测试冒称凭证持久化验收。实际Swift TLSBridgeProbe重新release构建。目标证书私钥证明由C#产生，Windows OS signer通过HTTPS发布；控制端取得新鲜issuer签名TLS描述符，指纹必须与实际证书匹配后连接。

计划依次验错pin、错HMAC、认证前PING；各自必须结束且两端protected pong增量零，核对专有拒绝结果。正常同一TLS连接至少3个pong后继续5秒并增加超过10个pong，跨越初始3秒lease，不重新连接。OS owner撤销后双端自主watch退出/join、真实Swift完成connection_closed而非timeout/cancelled，C# session和helper tunnel各关闭一次，后续pong不增加。

最后沿用精确node/verifier回收、所有进程正常退出/join、客户端cleaned日志核对、原模式/策略恢复、线上Store/Nginx核对。失败时仍best-effort OS签名撤销，未知节点/进程保留journal，不能盲删writer锁。

## 边界与验证

- Python4项纯断言通过：公共bootstrap/consent（含phase错误仍撤销）、TLS三种负向严格拒绝与零载荷、关闭不能是timeout/cancelled/缺计数；不是实际TLS证据。
- `swift build -c release --product TLSBridgeProbe` 成功。
- Windows新编译/双机私有隔离实验已实际执行；未改线上binary/unit/Store/profile/公网接口。Go服务源码未改，本轮未重跑全量Go race。
- HMAC仍通过测试私有管道注入，不是最终产品秘密投递；API root/CA仍是私有测试bootstrap/SSH管理转发，不是正式信任安装/公网API。
- 本轮撤销关闭不保证当时旧ACL残留；故障导致旧规则留存时的安全关闭需后续专门验收。未添加自然300秒过期/硬崩溃/GUI/安装证据。

## 继续操作

用户确认上轮Windows STOPPED，需正常桌面重跑现有 `windows-native-network/start-interactive-vault.ps1`，保持READY且暂不Enter。总控先实际pinned probe，再运行：

```sh
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\chatgpt\远程软件开发\测试安装包\windows-native-network' python3 -B poc/selfhost/public-server/split-consent.py --tls
```

不要使用旧产品安装包/删除Tailscale，不向服务器上传正式release-root/CA私钥，不开启新公网端口或改业务443。完整实机退出0和独立复核后才记通过，并请用户最后Enter/STOPPED。
