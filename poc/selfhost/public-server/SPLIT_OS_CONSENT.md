# 4C-4G：独立进程双机 OS 授权入口

真实双机 OS consent 验收已通过，完整退出码0。不是整体切片 4 完成。

路径：Windows 桌面 OS signer + Mac 随机测试 OS signer → 私有 loopback HTTPS frontend（nobody）→ kernel broker → 独立 root authority-daemon → 真实 persistent verifier/supervisor。不启动 enrollment-fixture；协调进程只创建隔离测试材料、启动/停止子进程，收到公钥及公共 API CA，不持有设备私钥，也无批准、签名、enroll 或 Store 并发写接口。

`daemon-network-check ... --frontend ... --os-consent <target-public> <controller-public>` 仅输出严格公共 readiness，接受精确 stop 指令；EOF/超长/其他操作/5 分钟超时都走失败退出与保守清理。只有实际 daemon/frontend STOPPED、正常退出与 join、零 ACL 后才清理 verifier。未 join/精确 Retire 失败保留实验目录，协调端不在进程生存未知时恢复策略或删除证据。

`split-consent.py` 先验证 Windows 桌面 pinned signer，再切空的隔离控制面 database。仅私有 SSH 正反向转发，使用本轮服务器随机测试 root/CA/profile，**不是安装可信锚、真实公网准入或最终 SSH-free 产品**。不上传发布根/CA 私钥，不替换线上服务、Store、Nginx，不增加公网端口。

验收计划：两端 OS 签名注册；邀请只产生 pending；明确拒绝无 grant；controller 自批准拒绝；owner 明确批准、双端一致 grant 与签名/主体/期限验证；无节点绑定仍不能取得网络状态；owner 签名撤销，状态 revoked 且无 grant。随后 join API、正常停止独立进程、精确 Retire、原策略恢复，核对资源零和线上 hash/Nginx。

本入口**不注册客户端网络节点、不验 nodeproof、最小规则真实网络拒绝或产品 TLS**；这些是下一阶段，不能沿用旧 fixture 通过证据。客户端 `use_grant` 先请求签名 grant_clock 校时，再验 grant 签名/主体/期限；本入口撤销另以权威签名请求/status 验证，不用单纯 grant 签名判断撤销。

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
