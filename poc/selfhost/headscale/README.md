# 真实 Headscale 管理驱动（私有、服务器本地）

`Driver` 实现 `invite.ProvisionDriver`，用受信任 CLI 执行隔离 Headscale 0.29.4 的管理操作。`LocalRunner` 固定 `/opt/remoteapp-poc/headscale` 和 `/etc/remoteapp-poc/config.yaml`；只能部署到独占的 PoC 控制平面，不能指向共享生产 tailnet。不是公网 HTTP 注册接口，不把管理员权限下发给客户端。

已接入：

- Mint 创建 `remoteapp-ticket-<32hex>` 专属用户、默认非 reusable 单次 key；发现已有同名工单拒绝重发。CLI 只支持时长，按绝对剩余期限向下取整减一秒；回读核对实际 expiry 不超过工单绝对截止时间，且 user/key 归属正确，否则不给客户端 secret。时间支持 protobuf seconds/nanos 和 RFC3339 两种格式。
- Observe 精确核对工单用户、节点 key、唯一节点、隔离 IPv4，不相信客户端上报 IP。管理观察并非持有证明。
- Verify 必须注入可信 `LiveProof`；未接入或上下文取消时拒绝。实现者须通过实际 tsnet 加密连接的 WhoIs 及应用身份 challenge 验证，不能传一个无条件 true 的回调。
- Cleanup 根据工单名重新发现用户，即使未知 Mint 结果没有返回句柄也能按工单 expire keys / 删除节点 / 删除用户；不存在时成功，不清其他工单或普通用户。
- ReplaceRules 只允许隔离 IPv4 单地址、目标端口 47476；零规则显式 `{"acls":[]}`。私有 0700 目录写临时 0600 文件，policy check → policy set → 严格回读核对。只支持 db 模式，当前公网仍为 file 模式，拒绝假成功。独占整个隔离策略；不是共享策略的部分合并器。更新错误不得视为原规则已撤销，调用方保持失败栅栏及应用授权门禁。

输出/错误不包含 provider 原始响应，注册 secret 不进命令参数、日志或持久化账本。CLI 响应上限 1MiB，exec.CommandContext 支持取消。可信回调/Run 仍须遵守上下文；不能靠接口强制杀死任意不守约回调。

## 本轮证据与未完成项

2026-10-07：独立 Linux x64 `cmd/headscale-driver-check` 在真实隔离服务器验证随机工单的单次凭据创建、到期上界和精确/幂等回收。调试暴露 CLI 不接受 RFC3339 expiration，以及 CLI protobuf Timestamp 对象格式，已修复。两次因解析失败遗留的已确认工单也精确回收；最终 users/nodes=null，服务 active，原两项业务 Nginx 哈希 OK。

驱动单测覆盖发放、重复拒绝、观察与持有门禁、孤儿隔离回收、空规则/最小规则/拒绝越界、失败回读和脱敏。策略更新当前仅有单测，不是公网动态 ACL 证据。没有改公网策略模式、原策略、产品安装包、系统 Tailscale 或 Windows。

下一步仍需：注册前获取 tsnet 节点公钥（不能使用未证实 API 或假 key）、实际 LiveProof 接入，以及受控切换隔离服务到 db 模式后真实“允许→零规则拒绝→撤销/重启恢复”验收。零规则网络语义不能仅根据 JSON 回读判定通过；动态策略与工单后端联动未完成前不得开放注册服务或宣布完整免登录闭环。

服务器检查程序 stdout 只输出固定 PASS/FAIL，不可改成打印 key JSON。`--cleanup-ticket <32hex>` 仅用于已明确识别的工单恢复。它是管理员 fixture，不是最终用户安装体验。
