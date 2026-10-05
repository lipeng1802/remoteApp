# 自托管连接最小实验

这是独立测试程序，不进入 RemoteController/RemoteAgent 安装包，不采集屏幕、不注入输入。
客户端内嵌 tsnet，注册到本机 Headscale；另一个本机进程提供 TLS DERP。没有 Tailscale 账号登录或公共 DERP 配置，系统已安装的 Tailscale 状态不参与实验。

固定版本：Go 1.26.8，Headscale 0.29.4，tailscale.com/tsnet 1.102.5。`go.mod`/`go.sum`固定 Go 依赖。
Headscale 发布说明中最低客户端为 1.80.0，但实际兼容性须以本实验结果为准。

## 本机运行

需要 macOS/Linux、Python 3、上述 Go 和 Headscale。当前 `smoke.py` 使用 Unix 管道与短 Unix socket 路径；Windows 暂仅交叉编译 helper，不能用它声称 Windows 已运行通过。

在仓库 `artifacts/connection-poc/toolchain/` 放置官方 Go 解压后的 `go/bin/go` 和 Headscale 二进制。当前 Mac Intel 二进制命名 `headscale-0.29.4`；Linux 则用同版本官方 Linux 二进制放到同一路径。首次下载后按官方校验清单验证 SHA-256；工具链不提交。

本机已核对：

- Go `go1.26.8.darwin-amd64.tar.gz`：`186be014105aa6542b767d2c6ed5cca10a0214bdff809ef1724022a8c7894150`。
- Headscale `headscale_0.29.4_darwin_amd64`：`06e4c94a8b9397ed8c2714a4cd484c998604dc884e9b5d4a186aef05f14047b1`。

依赖首次下载可按本机代理设置 `HTTPS_PROXY=http://127.0.0.1:1080`，它仅用于工具链下载，不是连接架构依赖。然后在仓库根目录执行：

```sh
zsh poc/selfhost/verify.zsh
```

仅检查跨平台构建可运行 `zsh poc/selfhost/cross-build.zsh`；它不会运行目标平台程序。

如 Go 位于别处，可设置 `GO_BIN`。缓存和构建产物都在 `artifacts/connection-poc/`。
测试开始前要求 18443、18444、19090、15443 本机端口空闲；不自动停止已有进程。
临时节点凭据、Headscale 数据库和会话 token 由 smoke 创建，管道传递注册信息，退出后仅清理本轮创建的临时目录和进程。输出固定 PASS/FAIL，不显示密钥。

## 实验边界

- Headscale HTTP 仅绑定 loopback，不能把此配置搬到公网。
- DERP 仅绑定 loopback，TLS 使用每轮临时证书的 SHA-256 pin，没有关闭证书验证；仅供本机 fixture，不是公开中继服务器。
- 测试 ACL 只允许新数据库分配的 `.2` 节点到 `.1:47476`；第三台 `.3` 必须拒绝。它不代表多租户动态授权已经完成。
- helper 仅传随机会话 token 并接收固定字串，不是 TLS 产品桥接。会话 token 不是用户验证码，固定载荷协议也不是正式远控认证协议。
- 重启沿用同一节点状态，避免新增身份；完整产品仍需专用凭据存储、撤销、验证码后端和本地桥接访问控制。
- 强制中继使用固定版本的调试开关，仅用于证明 DERP 路径，不写入最终用户配置。没有模拟真实 NAT/运营商，也不代表跨网验收。

## 首轮结果与未通过项

本机已证明免浏览器注册、自建 DERP 固定数据、错误 token 拒绝和第三节点 ACL 拒绝。Go 单元测试与依赖校验通过。Windows/Linux/Apple Silicon helper 已交叉编译；实际运行未验收。

v1.98.4 与升级后的 v1.102.5 均在 WireGuard 关闭中阻塞。helper 现给 `tsnet.Close` 8 秒期限，失败明确输出 `cleanup_timeout` 并以非零退出，进程退出后由 OS 回收资源；这只是防止上层等待无期限，不代表库的优雅关闭已修复。`verify.zsh` 仍应非零退出并报告该未解决项，不把网络 PASS 解释为全量通过。

首轮“退出后重启同一客户端，再做错误 token 检查”在注册成功后出现 `dial_failed`，因此改在同一存活节点内验证 token 拒绝；重连问题独立保留，未因调整测试关闭。完成关闭与重连可靠性验证前，不接真实键鼠、不宣称最终架构已选定。

复现客户端重连：构建完成后在 `poc/selfhost` 运行 `python3 smoke.py --check-reconnect`；复用本轮客户端私有状态，再尝试传输固定数据。所有状态仍属于临时 fixture，退出后删除；此命令失败不影响现有远控应用。

许可证依据：[Tailscale BSD-3-Clause](https://github.com/tailscale/tailscale/blob/v1.102.5/LICENSE)、[Headscale BSD-3-Clause](https://github.com/juanfont/headscale/blob/v0.29.4/LICENSE)。本切片不分发第三方二进制；对外打包前需提供完整第三方声明及依赖许可证清单。
