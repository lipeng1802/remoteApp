# 4C-4I：独立进程产品 TLS 入口（实机待验）

新增 `split-consent.py --tls` / `split_tls.py`。代码准备与本地断言/Swift构建完成；没有执行新的双机TLS验收，不能引用旧fixture结果作为此入口通过证据。整体4未完成。

## 新链路

真实 Windows/Mac OS signer → 私有 HTTPS 非root frontend → kernel broker → 独立 root daemon + 真实 persistent verifier/supervisor；所有设备批准、enroll/bind、TLS发布及描述符仍走 OS 签名 API。复用 --nodes 的真实持有证明/错误身份拒绝阶段，不运行 enrollment-fixture。

TLS阶段正常停止旧固定载荷helper，同node key/IP重开为自主HTTPS模式。每个helper内负责签名时钟校准、状态watch、短lease与不可逆gate关闭。父进程无刷新线程、不代签、不通过state推送授权；明确检查parent state操作不能授权managed gate且没有旧Token fallback。

实际C# TlsBridgeProbe使用产品RemoteProtocol；Windows仅上传本轮源码archive并核对SHA256，在短期grant创建前运行65项bridge-validation与publish。DPAPI部分必须显式SKIP，不将65项协议测试冒称凭证持久化验收。实际Swift TLSBridgeProbe重新release构建。目标证书私钥证明由C#产生，Windows OS signer通过HTTPS发布；控制端取得新鲜issuer签名TLS描述符，指纹必须与实际证书匹配后连接。

计划依次验错pin、错HMAC、认证前PING；各自必须结束且两端protected pong增量零，核对专有拒绝结果。正常同一TLS连接至少3个pong后继续5秒并增加超过10个pong，跨越初始3秒lease，不重新连接。OS owner撤销后双端自主watch退出/join、真实Swift完成connection_closed而非timeout/cancelled，C# session和helper tunnel各关闭一次，后续pong不增加。

最后沿用精确node/verifier回收、所有进程正常退出/join、客户端cleaned日志核对、原模式/策略恢复、线上Store/Nginx核对。失败时仍best-effort OS签名撤销，未知节点/进程保留journal，不能盲删writer锁。

## 边界与验证

- Python4项纯断言通过：公共bootstrap/consent（含phase错误仍撤销）、TLS三种负向严格拒绝与零载荷、关闭不能是timeout/cancelled/缺计数；不是实际TLS证据。
- `swift build -c release --product TLSBridgeProbe` 成功。
- 本轮没有新Windows编译结果、没有服务器变更/新节点/ACL，真实双机待READY；Go服务源码未改，本轮没有重跑全量Go race。
- HMAC仍通过测试私有管道注入，不是最终产品秘密投递；API root/CA仍是私有测试bootstrap/SSH管理转发，不是正式信任安装/公网API。
- 本轮撤销关闭不保证当时旧ACL残留；故障导致旧规则留存时的安全关闭需后续专门验收。未添加自然300秒过期/硬崩溃/GUI/安装证据。

## 继续操作

用户确认上轮Windows STOPPED，需正常桌面重跑现有 `windows-native-network/start-interactive-vault.ps1`，保持READY且暂不Enter。总控先实际pinned probe，再运行：

```sh
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\chatgpt\远程软件开发\测试安装包\windows-native-network' python3 -B poc/selfhost/public-server/split-consent.py --tls
```

不要使用旧产品安装包/删除Tailscale，不向服务器上传正式release-root/CA私钥，不开启新公网端口或改业务443。完整实机退出0和独立复核后才记通过，并请用户最后Enter/STOPPED。
