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

## 已验证与未验证

Go测试覆盖错根/channel/floor、非规范包装、额外/缺失/篡改文件、commit失败、commit已落盘但回执不确定、降级拒绝、旧版本保留与active文件重验。macOS真实Keychain TEST journal已完成epoch1首次激活、关闭重开、epoch2 CAS升级、再次重开及精确清理。Windows x64只有交叉编译，尚未在Credential Manager运行。

未完成的安装产品工作包括：

- 用compiled release根和固定activation scope接入真实updater/launcher；
- 安装目录所有者、ACL、硬链接/重解析竞争及跨文件系统拒绝验收；
- macOS Developer ID/公证和Windows Authenticode的真实证书链验证；
- 连接现有DMG/Inno、安装/升级/回退UI与卸载保留策略；
- Windows实机原生journal、进程占用时升级和重启验收。

在这些完成前，不能声称已完成签名安装包或产品级原子升级。
