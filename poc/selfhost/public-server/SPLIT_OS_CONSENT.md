# 4C-4G：独立进程双机 OS 授权入口

## r16 活会话失效后重新授权（真实双机与最终收尾已闭环）

独立 `--compiled-live-reauthorize` 先保持首份grant真实TLS活跃，签名撤销后等两端自主fault join/native retirement和节点回收，旧owner保持锁闭；之后旧API实际join，新邀请/明确审批取得不同grant，用新节点目录/新node、新idle owner和Mac进程启动新TLS。新grant要求不同scope证书和HMAC，同grant禁止换pin限制不变。新TLS持续pongs后正常join，最后新grant签名撤销并恢复服务器基线。78项Python及相关8包Go race/vet已通过，真实双机协调actual exit0、上述组合流程PASS；两桌面STOPPED后只读核验actual exit0，本轮进程/bootstrap/旧新测试证书scope无残留，控制资源归零、正式基线不变。自然过期、GUI、公网/无SSH与安装验收不在本轮。状态以HANDOFF为准，不复用已消费r16。

## r15 重新授权入口（实机与收尾已通过）

`--compiled-reauthorize` 是独立入口：节点注册前撤销首份批准授权，验证旧grant在线拒绝、旧request不可重批与旧码不可复用；实际join旧API客户端后重新邀请/明确批准，必须得到不同request/grant。新API客户端使用相同OS签名身份和固定HTTPS参数，不允许现有客户端替换已接受grant。仅新grant进入compiled Headscale/产品TLS，最后正常join、签名撤销和基线恢复。

Python和Go回归已通过；r15真实双机协调器actual exit0，新授权Headscale绑定/产品TLS持续pongs、正常join/签名撤销和服务器恢复均PASS。用户确认两桌面STOPPED，收尾只读核验actual exit0，本轮双端子进程/bootstrap/测试证书无残留、控制资源归零、正式基线不变。Go过期分支仅fixture clock推进/持久化重开，不是实机自然过期；本入口不建立首份grant的旧TLS，因此不是旧活会话撤销后重连。操作和当前证据以 `docs/HANDOFF.md` 为准，不重用已消费桌面stage。

真实双机 OS consent 验收已通过，完整退出码0。不是整体切片 4 完成。

## 4C-4H：独立进程真实 nodeproof（已通过）

新增 `split-consent.py --nodes` 与 `daemon-network-check ... --os-nodes <target-public> <controller-public>`，只额外输出本轮公共 verifier node key/marker。设备注册/批准/enroll/bind全部由OS签名HTTPS进入独立进程，不提供协调端批准或Store写接口。

最新整轮复测退出0：两端离线公钥与签名意图、prepare不注册、enroll单次性、真实join key一致、绑定前零ACL拒绝、Windows持有证明；错误Mac OS身份实际challenge收到但未签、bind拒绝且无payload规则；同node key/IP换回正确OS身份恢复，Mac持有证明通过。两端active签名binding与实际node/IP一致，仅controller→target:47476 ACL；正向真实固定载荷允许、反向拒绝且接收计数0。OS owner撤销、权威revoked、周期节点回收/零ACL与实际payload拒绝通过。

全部helper/API及daemon/frontend正常关闭/零退出/join；确认所有客户端日志cleaned后精确Retire verifier、删除本轮隔离资源、原策略恢复，线上Store/Nginx/服务未变、nodes/users/preauthkeys零。首轮清理错误不算完整PASS：把审计日志非空误判未回收；只读确认cleaned及无owner后精确手动清理恢复，修正断言并完整重跑通过。Linux4项实际组件PASS无SKIP，Python2项断言（含phase失败仍撤销）、Linux构建/vet通过。

这里固定载荷Token是私有测试凭证，非产品HMAC分发；不包括产品TLS或可信安装。撤销拒绝时节点已回收，不冒称节点保留场景的纯零规则测试。下一步产品TLS/自主刷新/撤销及旧ACL保留故障关闭，整体4仍未完成；Windows最终Enter/STOPPED待确认。

路径：Windows 桌面 OS signer + Mac 随机测试 OS signer → 私有 loopback HTTPS frontend（nobody）→ kernel broker → 独立 root authority-daemon → 真实 persistent verifier/supervisor。不启动 enrollment-fixture；协调进程只创建隔离测试材料、启动/停止子进程，收到公钥及公共 API CA，不持有设备私钥，也无批准、签名、enroll 或 Store 并发写接口。

`daemon-network-check ... --frontend ... --os-consent <target-public> <controller-public>` 仅输出严格公共 readiness，接受精确 stop 指令；EOF/超长/其他操作/5 分钟超时都走失败退出与保守清理。只有实际 daemon/frontend STOPPED、正常退出与 join、零 ACL 后才清理 verifier。未 join/精确 Retire 失败保留实验目录，协调端不在进程生存未知时恢复策略或删除证据。

`split-consent.py` 先验证 Windows 桌面 pinned signer，再切空的隔离控制面 database。仅私有 SSH 正反向转发，使用本轮服务器随机测试 root/CA/profile，**不是安装可信锚、真实公网准入或最终 SSH-free 产品**。不上传发布根/CA 私钥，不替换线上服务、Store、Nginx，不增加公网端口。

验收计划：两端 OS 签名注册；邀请只产生 pending；明确拒绝无 grant；controller 自批准拒绝；owner 明确批准、双端一致 grant 与签名/主体/期限验证；无节点绑定仍不能取得网络状态；owner 签名撤销，状态 revoked 且无 grant。随后 join API、正常停止独立进程、精确 Retire、原策略恢复，核对资源零和线上 hash/Nginx。

默认入口不注册客户端网络节点；`--nodes` 扩展的真实 nodeproof 与最小规则证据见上方4C-4H。两个入口均不包含产品TLS/可信安装验收，不能沿用旧fixture证据。客户端 `use_grant` 先请求签名 grant_clock 校时，再验 grant 签名/主体/期限；撤销另以权威签名请求/status 验证，不用单纯 grant 签名判断撤销。

## 当前证据

- Linux 两项真实组件测试 PASS，无 SKIP：公钥校验，operator 协议/额外字段/代批准输入拒绝，readiness 无私钥/grant；不访问 Headscale/Store，临时测试 binary 已删除。
- Python 两项纯断言 PASS：严格 split readiness、模拟 consent 顺序，拒绝自批准/自动 grant；不是双机实机结果。
- Linux check 已重新构建；真实双机未运行，等待 Windows READY。
- 本轮完整 Go race/vet、Linux check vet 通过（Mac 全量不含 Linux 专属组件，后者以实际 Linux 两项结果为准）。

## 真实双机记录

用户重新 READY 后运行最新 split-consent.py，实际 pinned Windows 桌面探针通过。独立 root daemon、nobody frontend/内核 broker/真实 verifier ready；Windows/Mac 各自 OS 签名 API ready。注册、不同主体、授权前拒绝、pending 无 grant、精确 owner peer、controller 自批准拒绝、owner 拒绝、新邀请和明确批准、两端同 grant/签名校时验签、无客户端绑定网络拒绝、Windows owner 签名撤销及 status revoked 无 grant，全部 PASS。

两端 API 正常 closed/零退出，独立 frontend/daemon 正常停止/join/零ACL，精确 verifier ticket 退役、隔离目录/上传程序与 Windows 随机客户端目录回收、Mac 随机身份清理、原策略恢复，最终协调程序退出0。独立 SSH 复核 authority/frontend/poc 均 active，线上 State 原哈希不变，Nginx全部OK，nodes/users/preauthkeys零，原 file policy 完整。Windows 桌面 owner 保留，最终用户 Enter/STOPPED 待确认。

未执行客户端网络注册/nodeproof或产品TLS，未部署公网API或正式trust安装；这些仍待下一阶段。

## 下一次操作

Windows 正常桌面运行现有 windows-native-network 包 `start-interactive-vault.ps1`，保持 READY，不按 Enter。不用旧产品安装包，也不删除 Tailscale。总控随后运行：

```sh
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\chatgpt\远程软件开发\测试安装包\windows-native-network' python3 -B poc/selfhost/public-server/split-consent.py
```

运行成功仍需用户最终 Enter/STOPPED；不能远程停止用户桌面 owner。失败不要盲删 journal、writer.lock 或恢复模式，先确认所有 owner/子进程 join 并按精确资源处理。
