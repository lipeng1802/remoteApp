# r60：长期 verifier 部署的二进制兼容升级

`binary-upgrade.py`为受信root本地运维工具，只替换已核对的authority、frontend与无副作用的network-deployment-check。它不创建身份、不注册节点、不恢复Store快照、不删除锁、不修改unit/config/profile/Headscale策略模式，不操作业务Nginx或开放端口。它不是远程设备可调用的HTTP/IPC入口，也不是签名发行安装器。

工单stage必须是root-owned 0700的`/run/remoteapp-binary-upgrade-<16hex>`，manifest固定三个候选二进制、已有维护原语helper的SHA256和三个旧二进制的预期SHA256。候选路径拒绝symlink/非普通文件/非root/可被group或other写入。stage、manifest和本地调用者属于受信运维配置，不构成publisher签名认证。不得以新工单自动接管未知状态。

每一步独立执行`python <stage>/binary-upgrade.py <stage> <step>`：

| 步骤 | 门禁与结果 |
| --- | --- |
| prepare | 三个服务健康、旧binary精确匹配、无网络工单/TLS身份或未过期授权；备份原binary、记录Store/config/verifier摘要与身份 |
| quiesce | 记录当前服务实例；依次向frontend、authority的主进程发送TERM，等待同一实例STOPPED与实际PID消失，再收尾systemd状态；最终无owner锁 |
| retry-quiesce | 仅处理stopping工单；要求运维已明确恢复两服务并验证当前READY后，重新完整停止；不能认可先前缺失的回执 |
| check | 服务无MainPID、无锁、零ACL；当前候选只读空闲预检通过 |
| install | 复核空闲、原binary与候选；同目录临时文件fsync/rename替换三个binary，权限/UID/GID沿用原安装 |
| start | 候选或明确回退的binary必须精确匹配；先authority READY，再frontend READY |
| verify | 当前实例READY、内核UID验证broker issuer、固定CA/TLS HTTPS负向请求、Store/config/verifier摘要不变、零ACL及Nginx摘要通过；可重复只读复核 |
| rollback | 仅已确认空闲且无锁时恢复备份binary；当前文件只能是已知原版/候选，Store必须未变。随后显式start/verify |

READY/STOPPED取当前boot的journal，并绑定PID与当前ExecMainStartTimestampMonotonic；拒绝旧实例或复用PID的历史日志。TERM只发主owner，不向其整个cgroup同时取消，避免抢先打断嵌套helper；45秒内未取得真实退出与回执就暂停，不SIGKILL、不把inactive当join。脚本不自动回退/重启；失败时保持原检查点，运行状态由失败所在步骤决定。若start只启动了authority，恢复前必须先明确停止并取得新实例真实收尾证据；不能删除锁继续rollback。

r71 实际发现 CentOS journald 在进程退出后保留 stdout STOPPED，但丢失 `_SYSTEMD_UNIT`（cgroup变成`/`），因此原 `journalctl -u` 查询漏掉回执。现在查询当前boot的可信 `_PID`，通常仍要求正确unit；仅已知 STOPPED 标签允许unit缺失分支，必须与同一当前实例的实际 READY 记录在 `_STREAM_ID`、`_BOOT_ID`、`_UID`、`SYSLOG_IDENTIFIER` 完全匹配且均为stdout，并验证start/stop单调时间顺序。不接受不同unit、外来stream/uid、旧READY或无READY；实际PID消失门禁保持。不是用服务inactive/状态0替代回执，也没有停止或修改journald。

新 readiness authority 已通过工单 `remoteapp-binary-upgrade-3e3d5c1a66528b66` 部署/verify，SHA256 `28fb8d40ab28a3f56d6419efb9e6dbeaca13d5de28c348ab0bc3a202bbdb4ea0`，frontend/探针沿用r60字节。两次漏查STOPPED后均只明确恢复原frontend，未换binary或认可失败；找到原始stream证据、增加回归并修复工具后，fresh实例retry-quiesce/check/install/start/verify通过。Store沿用r67合法摘要74a47e...，非下方r60旧摘要；完整值见HANDOFF。服务配置、verifier identity/journal、1 user/node/key、零规则及Nginx基线保持。工单含原binary、checkpoint与最终recovery-stage，不能据此恢复旧Store。

备份位于root-only `/var/lib/remoteapp-binary-upgrades/<stage basename>`，包括原三个binary和原子checkpoint。本轮另将完整已核对stage保存到其`recovery-stage/`，原binary备份与候选分目录。/run重启会消失；恢复时先核验持久材料摘要，在相同规范stage路径重建工具，再检查实际服务/进程/锁/Store；复制材料本身不授权接管未知owner。不覆盖当前合法Store或verifier身份，也不能运行旧empty-control清理脚本。本轮未实际执行回退或整机重启验收。

## 实机结果与边界

本轮工单`remoteapp-binary-upgrade-db26f37aca14f01f`已到verified。首次旧systemctl stop缺STOPPED证据，工单停在stopping；旧前端正常重启后，以main-only TERM复测得到真实STOPPED/退出；最终retry-quiesce/check/install/start/verify全部通过。未认可第一次缺失回执，也没有强杀或删锁。

authority新SHA256 `42cd8144095fcea9a0510147c46d8f8b1aafe63298d952437411bcad6e531c92`；frontend `619b3ae57d3a1535a8fd7061812a67a099558e596f9828b88b4294a5f5a65f90`；探针`5223127e3f2bfa580b7643eeefd0870df58fc5fbd068d2dc6f7168c9496d4b7a`。已部署authority含session-identity协议domain，Go签名/授权回归通过；域字符串存在只用于版本检查，不是跨机会话成功证明。

Store保持`5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca`；verifier journal保持`91dccf4683e7b7a3ccb464b7f93e7ff2c9edf732b56a86f72662a8a6118f022b`，没有Mint。Headscale仍1 user/1 node/1 preauthkey，零ACL，三服务active，8444仅回环。Nginx基线完整通过。profile仍沿用原有效期，用户负责续期，不能拿旧工单绕过过期验证。

8项Python失败/时序门禁及broker/deviceservice/invite/verifier四包race通过。正式服务器跨UID HTTPS负向探针通过；本轮未注册测试设备、发grant、启动客户端overlay或产品payload。接下来须为r59监督入口接Windows同用户可信控制通道，用匹配新鲜profile的双机实际产品跑ACTIVE watch、实际PONG增长、维护撤销/网络join/进程退出/独立删除整链。旧desktop-session/compiled pair依赖另一套Config与已有绑定node的模型，不可直接混为本轮自主产品入口。
