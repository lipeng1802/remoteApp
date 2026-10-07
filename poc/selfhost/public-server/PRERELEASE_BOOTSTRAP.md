# 独立自托管预发布：配置材料准备（4C-2A）

用户选择独立自托管预发布，不复用业务正式信任配置。服务标识 `remoteapp-prerelease`，epoch 1，候选 API 地址 `https://mk.fengmap.com:8444`。只读 SSH 确认当前 8443 在监听，8444 未在本次端口检查结果中出现；**未开放 8444、未部署 API、未改业务 443/现有 Nginx/Headscale**。客户端本轮 Windows 四项服务信任验证已由用户确认 `VALIDATION PASSED`，仍不是生产信任安装。

## 生成入口与隔离

`cmd/prerelease-bootstrap` 参数为全新绝对目录、明确非 443 HTTPS 端口、服务标识。拒绝已有目录、符号链接父目录、不合法地址。所有目录 0700、文件 0600；文件 CreateNew + Sync，READY 最后写。失败保留部分材料并报错，禁止自动覆盖、重试时默默换身份。工具不上传、不打印密钥、不部署服务、不修改 OS 信任。

生成目录（artifacts 已被 Git 忽略）：

```text
artifacts/connection-poc/prerelease-v1/
  release-local-only/        # 绝不可上传服务器/包含于客户端或验证 zip
    release-root.pem        # Ed25519 发布私钥，签服务配置
    api-ca-key.pem          # 独立 API CA 私钥，签服务器 TLS 证书
  server-private/           # 后续只可安全部署这一子目录，不能上传整个父目录
    api-key.pem             # ECDSA P-256 API TLS 私钥
    api-cert.pem            # 专属域名服务器证书
    authority/state.json    # 独立在线 issuer 与空的持久状态；含私钥
  client-public/            # 仅公开字段，但安装真实性仍需可信分发
    installation-anchor.json
    service-profile.json
  READY
```

发布根、CA、在线 issuer/TLS 私钥各自独立。CA 90 天，服务证书与 profile 30 天；profile 验证后写入，issuer 来自真正可重开的 Store，不是临时未落盘随机值。READY 仅表示材料完整，不能代表部署/安装授权。安装锚和 profile 同目录不能给自身建立信任；正式安装仍需独立可信分发并持久化 epoch floor。

**本轮发布私钥是本地未加密 owner-only PKCS#8 文件，不是 Keychain/硬件不可导出，也不是断网离线计算机。** 文件权限不抵抗当前用户恶意程序/管理员/备份泄漏。生产前必须完成受保护离线保管、加密备份、恢复演练和发布签名流程。本轮不提交、不读取输出私钥，不自动移动到未知备份位置。不要删除材料后重新运行来“恢复”同一身份。

## 已完成验证

新包 race 测试、trustprofile 回归、专用 vet 通过。验证 profile/独立根/实际在线 Store issuer 一致，TLS 私钥匹配域名叶证书/独立 CA，public 子目录只有两份公开文件、所有路径 owner-only，重跑不覆盖，错误端口/443/URL/符号链接拒绝且不创建状态。实际本机生成 `prerelease-v1` 返回 READY。没有用这个结果宣称公网服务可访问。

## 下步（整体 4 尚未完成）

1. 先实现独立服务进程的有界设备 API、真实 Headscale backend 与故障关闭、持久状态/最小权限部署；现有 `invite-api` 明确仅限回环 fixture，**禁止直接公网反代**。
2. 再部署 server-private，仅打开需要的 8444；不改业务 443，服务更新/证书续期前核对签名 profile 与发布材料。
3. 完成客户端可信安装锚/floor 持久化、更新停止旧会话及恢复流程；受保护 HMAC grant/peer/TLS 绑定投递。
4. 接产品 IPC/GUI/独立包，Windows/Mac 实机验收。
