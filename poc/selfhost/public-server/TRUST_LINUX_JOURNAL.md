# 4C-4L：Linux root 专属信任 journal

实现：`trustprofile/journal_linux.go`，仅Linux构建。接上4C-4K的 `TrustJournal`，不是macOS/Windows普通用户存储，也未接GUI、安装器或线上daemon。

## 威胁模型与供应

可信安装器以root明确创建一个全新目录，再用固定安装锚调用 `InitializeInstallation`。恢复只调用 `OpenLinuxRootJournal` 和 `OpenInstallation`，不能在打开失败时调用Create。没有删除、重置、修复API。安装锚依然由调用方可信二进制提供，本轮没有正式安装锚供应工具或根轮换。

状态是签名profile记录，完整性由安装核心固定根校验，存储访问由Linux权限保护；不是加密密钥保险箱。每级路径必须绝对、root所有、不可group/world写；描述符锚定遍历且不跟随symlink，明确拒绝/tmp作为状态父路径。专属目录0700、普通文件0600、root所有且单硬链接。活owner每次读取还检查目录/锁权限及锁链接数。

面向非特权本地用户，**不抵抗root、受控root程序、原始磁盘写入、整盘快照回滚**。同一特权用户能绕过advisory lock或恢复合法旧记录；这些不在保护承诺内。只支持本地可靠文件系统，不以NFS/SMB锁语义替代本验收。目录不能授权给不可信的root插件/脚本。

## 生命周期与提交

创建目录使用排他mkdir，初始pending记录仅创建owner可以识别为未安装；中断后新owner不把pending当全新安装。缺失record/lock、损坏记录、错误权限或链接均拒绝，不自动重建。

owner用非阻塞独占flock持续持有至Close；调用方必须在所有会话退出后释放。flock是协作式锁，不能宣称阻止忽略锁的特权写入。[Linux flock说明](https://man7.org/linux/man-pages/man2/flock.2.html)

提交核对expected旧字节，以随机排他临时文件写完整next、fsync文件、原子rename覆盖record、fsync目录，再返回成功。文件fsync不替代目录fsync。[Linux fsync说明](https://man7.org/linux/man-pages/man2/fsync.2.html)

读取/提交故障关闭当前journal；提交结果不确定也不自动恢复。精确临时文件unlink不会删除既有状态；进程硬崩溃遗留的pending文件不自动扫除。下一次明确重开仍按已提交record验证。所有write/sync/rename错误走拒绝路径，本轮未注入真实断电、EIO/ENOSPC或rename后sync失败。

## 验收

Linux amd64静态测试程序在现有服务器执行4项 `TestLinuxRootJournalNative*`，完整通过、无SKIP：

- 真正子进程重开保留epoch，拒绝旧profile；本进程与第二进程同时争抢均拒绝。
- UID/GID65534子进程无法打开root状态；测试使用随机/root目录，不接触已有身份或配置。
- 未完成安装、缺失记录/锁、损坏、symlink/hardlink、不安全权限均拒绝Open和Create重置。
- CAS失败锁闭、明确重开保留旧状态；活owner记录/锁丢失及目录/文件权限变化拒绝；symlink目录和可写祖先拒绝。

测试程序上传SHA-256核对，程序和随机测试目录精确清理。没有新跑Headscale或TLS双机流程、没有Windows运行、没有改线上服务/策略/Store/Nginx。Mac全量Go race回归通过（Linux文件不在Mac编译范围），Linux全量vet和测试构建通过；这不是Linux race或断电耐久证明。

复测入口：用Linux root在可信来源、所有用户可执行的本轮测试程序上执行 `./trust-journal-tests-linux '-test.run=^TestLinuxRootJournalNative' '-test.v=true'`。程序自动创建并清理随机 `/root/remoteapp-trust-journal-test-*`，不能把旧安装状态作为测试目录。普通用户不应运行此入口或把SKIP记为验收。

下一步仍需macOS/Windows保护存储方案与原生验收、固定安装锚供应和客户端会话owner接入，不能将该root组件直接作为桌面应用提权安装方案。
