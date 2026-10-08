# 4C-4K：可信安装与版本下限核心

这是平台通用库，不是正式信任安装或产品发布。实现位于 `trustprofile/installation.go`，仍独立于GUI、安装包和网络客户端。

## 使用边界

可信安装器/二进制提供固定的 `InstallationAnchor`（发布根公钥、service、最低epoch）；不能从下载的profile或journal反向选择信任根。根替换需要单独的可信升级流程，本轮不实现。

`TrustJournal` 必须由平台适配器提供：owner整个生命周期独占、原子compare-and-swap、成功返回前持久提交，并保护未授权删除/重置/回滚。当前没有生产适配器，没有普通文件降级路径。首次安装的 `ErrNotInstalled` 只能表示明确全新安装，不能用于丢失/损坏后的恢复。

`InitializeInstallation` 验证初始profile并提交后才能打开。`OpenInstallation` 严格解析已提交记录，检查锚、epoch、签名和接受时间；过期历史仅用于显式更新，`Current`仍拒绝过期或低于可信软件最低epoch的配置。

## 轮换顺序

1. 校验候选签名、service、时效和版本；降级及同epoch不同配置拒绝，且不打断正常旧会话。
2. `SessionQuiescer` 取消并join全部旧连接、watch和tunnel；不能重入同一Installation。
3. 再检查候选时效，原子持久提交新记录，并核对读回结果。
4. 成功后才允许 `Current` 使用新配置；不自动创建新连接。

join失败、提交失败（包括可能已写入但无法确认）、状态突变或时钟回退关闭当前owner。恢复必须显式检查并重新打开可信journal，不能通过内存重置忽略异常。

## 验证与限制

新增5项测试覆盖提交先于使用、重开保留floor、首次初始化不能覆盖、严格记录解析、错误锚、外部状态变化、join/提交故障、过期历史显式恢复、可信floor提高、同epoch改写和join期间到期。

全量Go race回归、最终trustprofile race复测、vet及Linux amd64/Windows x64测试构建通过。未在这两个目标平台运行新测试；测试journal为内存模拟，不能证明OS持久化、ACL、安全存储或断电原子性。

特别是：核心只检测活owner期间journal变化，不能自行辨别两次打开之间被整体还原的合法旧journal。真实回滚保护依赖平台适配器和明确威胁模型，不承诺对管理员/root的任意磁盘还原提供保护。

下一步是平台journal、安装锚供应和真实客户端会话join接入；正式根/CA私钥、HMAC分发与公网接入不在本轮范围。
