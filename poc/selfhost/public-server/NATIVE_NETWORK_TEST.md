# 切片 4B-4B：双机真实 OS 身份协调入口

代码/本地回归完成；**真实 OS 身份双机网络验收已完整运行通过，退出码 0**。这是独立实际证据，不是合并之前 fixture 网络和本机桥接 PASS。用户已确认本轮 Windows STOPPED/随机测试身份清理通过，本轮验收闭环。

## Windows 操作

前一轮用户已确认按 Enter 后出现 STOPPED，桌面 owner 正常退出、该轮随机测试身份清理通过。新一轮使用独立 `artifacts/connection-poc/windows-native-network.zip`，不要混用旧包或旧安装目录 exe。它保留 UTF-8 中文路径修复，并将 bridge 空闲读取延长到有界 10 分钟，避免 device API signer 在产品构建/其他 helper 自主刷新期间被 30 秒空闲期限关闭；任何 owner stop 仍立即关闭所有连接，不延长网络授权 lease/grant。

解压后在正常桌面 PowerShell 执行（无需 SDK/管理员）：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\start-interactive-vault.ps1
```

看到 READY 后保持窗口，向 Mac 总控回传 bootstrap 的绝对路径。协调验收结束才按 Enter，必须有 STOPPED。包内也保留 `run-interactive-bridge-validation.ps1` 与 SSH `probe-interactive-vault.ps1`，需要独立复测时执行；完整 network runner 会先运行 SSH pinned probe。Hash 验证只是完整性检查，不是正式分发签名。

## Mac 协调入口

需要既有可信 Mac vault、Mac/Linux/Windows helper 及 fixture 构建；固定目标服务器与已授权 Windows SSH 不变。Windows 正常桌面 owner 未就绪就拒绝，不新建替代身份或使用 AppKey。

```sh
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\实际解压目录\windows-native-network' \
python3 -B poc/selfhost/public-server/enrollment-network.py --native
```

默认 Mac vault 是本仓库 `macos/RemoteController/.build/release/ConnectionIdentityVault` 绝对路径；如需其他可信构建，可显式设置 `REMOTEAPP_MAC_VAULT_EXECUTABLE` 绝对路径。不接受来自网络对端的配置。

协调器先执行 Windows 同用户 pinned probe、校验本机公开 bootstrap（pipe/scope/public/relay 路径），创建两个随机 Mac Keychain 测试身份（正常 controller + 负向 wrong identity）。父进程只持有创建 owner 控制管道/public，不调用其 sign 或取得私钥；helper/API client 自行重开 OS signer。Windows 使用用户启动的桌面 owner，协调器不停止它；Mac owner 在各 helper 退出/join 后精确 delete，失败不广泛删除凭据。

`enrollment-fixture --native` 仅接受严格 8 KiB JSON 的两端不同 canonical 公钥，不生成/输出设备 private key、不预注册/预批准 OS 设备，也不能代签设备 revoke。邀请/申请/拒绝/批准/校准由两端原设备 HTTPS client 用 OS signer 完成；显式确认仍由可信测试协调器发起，不是 GUI 人工批准界面。issuer/API TLS 临时信任仍是私有测试 bootstrap，产品 HMAC/RSA 测试桥仍保留，并非正式用户信任分发。

之后沿用完整 managed 验收：注册前节点公钥和 OS 签名意图、真实挑战持有证明（独立错误 Mac OS 身份拒绝）、实际单向动态 ACL/零规则拒绝、同节点重启、两端自主签名 HTTPS 刷新、真实 Swift/C# TLS pin/HMAC/认证前负向、停止/策略/Store 故障/撤销关闭、旧正向状态不可复活、回收。父脚本没有 watch 线程/state 授权转发。管理仍通过已安装 Tailscale 的 SSH，数据走独立 Headscale/tsnet；这不是最终公网免账号产品。

native fixture 的异常 teardown 是特权测试管理动作：先零 ACL，然后对自己的注册 journal 逐项回收，不伪造 OS 撤销签名；正常撤销仍走设备 HTTPS。恢复原 PoC policy/业务 Nginx 检查保留。Mac 凭据清理即便失败，也不能跳过已进入的 policy 恢复流程。

## 本轮证据与待验

Go 全量 race、公开身份 bootstrap 格式/同身份/私钥长度拒绝回归、Python 两组 bootstrap/角色隔离测试通过；Go vet、Windows/Mac helper/API/bridge 与 Linux fixture 构建通过。

用户保持新 owner 后实际 `--native` 完整退出码 0。输出确认仅 OS public bootstrap、真实设备注册/邀请拒绝与批准、节点签名意图/实时持有证明及独立错误 OS 身份拒绝，单向 ACL/反向拒绝/真实零规则拒绝，同身份重启，两端自主 HTTPS gate 与父授权拒绝。真实产品 TLS 的 pin/HMAC/认证前 PING 负向、正确流持续超过初始 lease、watch stop、策略/Store 故障、设备 OS 签名 revoke、旧正向状态不可复活及 TLS 关闭无新增 PONG/非 timeout 均通过。Windows 产品 SSH profile 65/65，默认产品 DPAPI 明确 SKIP，不是 66/66。

输出 `all_owned_control_resources_reclaimed`、`original_file_policy_mode_restored`、`original_poc_policy_unchanged`、`service_active_business_nginx_hashes_unchanged` 全通过。Mac 两份随机测试身份 teardown 收到 deleted 并零退出，helper/node 状态与管理转发均回收。另起独立 SSH 查 nodes/users/preauthkeys 均 0。服务器测试策略已恢复，不改业务 443/生产密钥/系统 Tailscale/安装包。用户桌面 owner 未由协调器停止；随后要求用户按 Enter 并回传 STOPPED，用户回复“通过”，本轮 Windows 正常停止/随机测试身份最终回收记为用户确认通过。未附详细输出，不记录额外退出码或耗时。

此结果不宣布整体切片 4 完成：正式 API/issuer 信任分发、HMAC 投递、GUI/IPC/独立安装仍待。仍由可信 SSH 协调 fixture/bootstrap 发起明确批准；没有新增直连/DERP 类型证明、整个 daemon 断电或 300 秒自然到期证据。
