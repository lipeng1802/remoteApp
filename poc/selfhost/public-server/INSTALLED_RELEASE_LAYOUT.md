# 签名发布布局与原子激活边界

## 目标

`installrelease`为新自主产品入口定义一个不依赖GUI安装器的发布核心：发布物先验签、全文件核验并放入不可变版本目录，然后才通过原生journal的CAS commit切换当前版本。旧进程继续使用旧目录，切换不删除旧版本。

这不是新的RemoteController/RemoteAgent安装包，也不会自动修改现有DMG或Inno Setup。

## 严格发布清单

平台只能是`darwin`或`windows`，布局固定为`autonomous-flat-v1`。清单中必须且只能出现：

- `autonomous-enrollment-install[.exe]`
- `identity-vault[.exe]`
- `installed-autonomous-product[.exe]`
- `tls-product[.exe]`
- `trust-install[.exe]`
- `trust-journal[.exe]`

每项固定文件名、大小和SHA-256；列表按固定顺序编码。`release-manifest.json`是严格规范JSON，签名使用compiled release根对带域分离前缀的规范manifest摘要进行Ed25519签名。核验同时绑定channel、platform与minimum epoch；错签名、降级、大小/哈希不同、额外/缺失文件、目录/符号链接、非规范JSON全部拒绝。单独SHA-256旁车文件不作为发布者身份证据。

## 不可变候选与激活

候选目录必须是releases root下的直接子目录，名称为`.staging-<32 lowercase hex>`。完整验证后移动为：

```text
<16-hex epoch>-<manifest SHA-256 first 16 hex>
```

移动后立即再做全清单验证。该步失败不会改变active记录；宕机最多留下未使用的已签名版本，不会自动删除或选中它。

active记录只包含version/layout/channel/platform/epoch/版本目录名/manifest摘要，不含私钥、网络凭据或设备身份。首次激活用nil期望值CAS，升级必须对当前记录精确CAS，并要求同channel/platform/layout且epoch严格增大。commit后立即读回核对；不确定回执必须返回错误，只能在重开journal后确认实际结果。

启动器不得仅信任active指针：`OpenActive`会重新验签manifest并计算全部文件哈希，然后才返回可执行目录。

## 固定管理器与启动器

`installed-release-manager`的可变输入只是操作名；一个构建产物只能处理一个编译固定的隐藏候选名、manifest摘要和epoch。它从自身目录固定定位`trust-journal`/`releases`，先核验已安装trust，再晋升候选并提交activation。只有候选确实不存在时，它才能按编译固定摘要重开已晋升目录，用于处理“已rename但activation未确认”的重试；存在但损坏的staging不会回退到最终目录。

`installed-release-launcher`不接受参数，不读sidecar root/path。它重开编译固定的trust和活动记录，每次重验签manifest和六个文件，然后启动活动目录内固定名产品并实际等待它退出。因此旧launcher不持有“回到旧目录”的运行时开关。

## 受保护 staging 交接

installer不得把外部解包目录直接rename为可激活版本。`ImportStaged`先在原目录核验签名和全固定文件集，拒绝符号链接、非常规文件、额外硬链接和可由group/other改写的Unix文件。它在受保护`releases`中独占创建staging，逐项复制为新单链接文件，同步文件与两层目录，再重验保护属性和内容摘要。因此实际晋升rename始终在同一`releases`文件系统内。

生产macOS/Unix安装根、`releases`及版本目录必须root owner且不得group/other writable；Windows生产根只允许SYSTEM/Administrators owner，并拒绝三类宽泛SID的写/删除/改ACL与owner权限。TEST模式可用当前用户的隔离根，但仍拒绝可宽泛写、链接/reparse、跨卷和多硬链接。manager和launcher每次操作都重做门禁；这些API只验证，不会自动提权或改权限。

## 已验证与未验证

Go测试额外覆盖外部payload导入后单链接、源硬链接、可宽泛写根、非root生产根、链接`releases`及被改宽的版本目录拒绝。macOS真实Keychain TEST组合已通过新导入交接、v1签名staging晋升/首次激活、身份安装、v2 trust与activation升级、旧/新manager读当前epoch、回滚拒绝、旧/新launcher执行当前产品拒绝探针及TEST清理。Windows x64只有交叉编译，尚未在Credential Manager运行。

未完成的安装产品工作包括：

- 把已验证的管理器/launcher放入受保护的真实安装根，并由正式installer/updater调用；
- 由特权正式installer创建真实生产owner/ACL，并在其他用户并发替换下做实机验收；
- macOS Developer ID/公证和Windows Authenticode的真实证书链验证；
- 连接现有DMG/Inno、安装/升级/回退UI与卸载保留策略；
- Windows实机原生journal、进程占用时升级和重启验收。

在这些完成前，不能声称已完成签名安装包或产品级原子升级。
