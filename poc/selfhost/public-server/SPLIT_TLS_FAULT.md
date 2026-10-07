# 4C-4J：旧ACL留存时TLS关闭/显式恢复（入口完成，实机待验）

用户已确认4C-4I桌面STOPPED。本轮仅开发入口/组件，不暂停线上服务，不新增Headscale节点/修改规则或Store。整体4未完成。

## 故障范围

`split-consent.py --tls-fault` 先执行真实OS consent/nodeproof/正常Swift-C# TLS验证，再通过仅服务器本地私有stdio的 `daemon-network-check ... --os-tls-fault` 控制本轮隔离daemon。普通--os-consent/--os-nodes没有故障命令。

额外公共fault_phase marker必需；严格固定freeze→recover→stop，一轮一次，拒绝乱序/重复/字段扩展/代批准。freeze对本轮持有的实际daemon PID SIGSTOP并核对/proc的T状态，不停止线上authority/frontend/poc/provider，不注销节点或下发零规则。非root私有frontend继续运行，经真正TLS HTTPS/network→broker期限测试必须503；实际控制面policy在暂停前后语义完全一致且非零，否则失败。

Python侧核对旧ACL为精确controllerIP→targetIP:47476，关闭前后均仍存在；两端helper自主HTTPS watch/lease停止，实际Swift/C#连接关闭/无新增pong/非timeout，不能以清规则或删节点解释关闭。

recover先SIGCONT本轮daemon以便正常TERM/STOPPED/零退出/join，再以原Store、原配置/issuer及同verifier key/IP显式启动新daemon，非自动恢复。重新取得fresh signed active binding与原node/IP一致、恢复最小规则；旧managed gate仍done/不active、fresh parent state不能重开，旧TLS继续关闭/无新pong。只有显式新helper/clock/watch/gate和新TLS连接才能恢复，C#原测试证书/HMAC保留，由fresh issuer descriptor再次核对；最后真实OS撤销与清理。

异常退出若仍暂停，会先尝试SIGCONT再正常停止；不能确认join/cleaned就保留日志。沿用30秒cleanup提交等待屏障与停止后日志核对，严格精确Retire/恢复模式，不删除线上writer锁。EOF/超时不是完整成功，失败可能需以原Store/verifier恢复回收。

## 当前证据与边界

- Linux最新check构建/vet完成，8项真实组件PASS无SKIP：原5项、故障协议顺序/失败不报成功、精确非零policy保留断言、实际自建sleep子进程内核暂停确认。测试没有访问Headscale/Store或线上PID，临时binary已删除。
- Python5项纯断言PASS，包括fault bootstrap/ack必须精确且True、非超时关闭与无protected载荷。不是实际TLS故障证据。
- 没有执行新双机故障验收，没有本轮Windows编译/节点/ACL变化；服务源码未改，未重跑全量Go race。
- 这模拟“权威进程暂停、网络规则留存”，不是provider断网、真实磁盘损坏、硬崩溃或自然300秒到期。暂停会让同进程verifier也停顿，仍需实测证明旧规则存在时产品关闭。
- 管理SSH、私有test root/CA、HMAC测试管道依然不是最终安装/信任准入/秘密投递，整体4未完成。

## 下一步

Windows正常桌面重跑现有windows-native-network/start-interactive-vault.ps1并保持READY，不按Enter。总控实际pinned probe后运行：

```sh
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\chatgpt\远程软件开发\测试安装包\windows-native-network' python3 -B poc/selfhost/public-server/split-consent.py --tls-fault
```

只有完整零退出、独立检查资源零/原策略/线上hash/Nginx恢复后才记录通过，最后用户Enter/STOPPED。现场若有失败先按精确journal恢复，不宣称功能阶段PASS等于整体验收成功。
