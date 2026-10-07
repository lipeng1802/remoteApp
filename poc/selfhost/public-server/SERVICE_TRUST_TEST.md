# 切片 4C-1：发布签名的服务信任配置核心

整体切片 4 **尚未完成**。本项交付配置验证/客户端接入与验证包；没有创建生产发布根、安装正式信任锚、部署公开 API 或改现有 GUI/安装包。

## 信任边界

新增 `trustprofile.Profile`：独立离线发布根 Ed25519 签名绑定 version/service/epoch/HTTPS endpoint/API CA DER/online issuer/绝对有效期，域 `remoteapp-service-trust-v1`。发布根与在线 issuer 必须不同。配置不能给自己提供可信根：release root、预期 service 和最低 epoch 必须来自独立可信安装/bootstrap，不能来自网络对端/服务响应。

解析有界且拒绝重复键/重复大小写别名/未知字段/尾随 JSON；验证签名、固定服务、最低 epoch、时间窗（最长 180 天且不超 CA 有效期）、CA 约束、规范公钥/base64/HTTPS 地址，禁止 userinfo/path/query/fragment。未知根、字段篡改、过期/未来、回退、同 epoch 换配置均拒绝。同根签发更高 epoch 可通过 `Advance`，但必须先停止旧会话、先持久提交新的可信 floor/profile，再显式启动新 helper。

**持久化 floor/安装根没有在本项实现**：`Advance` 是验证函数，不是跨重启防回退存储；调用者不能让同一签名配置自己降低 floor。不能由这些测试宣称防重装/磁盘快照回退或失去根密钥后的恢复已完成。根失陷/丢失需要独立发布/恢复流程，不自动从在线服务信任新根。

## 客户端和 helper

既有 `NewNetworkClientWithSigner` 仍仅允许回环 HTTPS，未放开默认网络边界。新增 `NewProfileNetworkClient` 仅在发布根验证后允许签名指定的非回环 HTTPS endpoint；仍独立 CA 验证、无代理、不跟随重定向，OS 签名请求协议不变。profile 到期后拒绝请求；捕获独立单调期限约束 HTTP 和 `StartManaged` 生命周期，取消时 Deny/join，不由迟到响应或运行中墙钟回退延长期限。

helper 与 device-api-client 新增私有初始化字段：`ServiceProfile`（签名 JSON）、`ReleaseRoot`（安装锚 32-byte hex）、`Service`、`MinimumEpoch`。仅 OS vault 模式可用，禁止 AppKey 与 raw Endpoint/Certificate/Issuer 覆盖或孤立信任字段；校验后使用 profile 的 issuer/certificate/endpoint。这些字段仍属于**可信本机初始化**，不是公共 IPC/用户随意输入的信任源。当前 GUI/安装和 network runner 仍未使用此模式，旧 fixture 入口保留兼容；没有将生产路径默认回退到 fixture。

## 验证

Go 全量 race/vet、Windows x64 helper/API/trust 交叉编译及 Windows vet 通过。纯回归覆盖有效配置、未知根/服务、篡改/格式/到期、最低 epoch/同版本变更/更高版本，真实本机 HTTPS 证明签名配置才可启用 localhost DNS endpoint，legacy 构造器仍拒绝、到期请求不抵达服务器。显式 Mac 原生 Keychain `TestNativeOSSignerReleaseProfileHTTPS` PASS（0.15s），真实 OS 签名经独立 TLS 根与服务签名验证，并精确删除随机测试凭据。测试发布根、issuer 与 CA 都为内存 fixture，不代表正式生产信任发布。

Windows 新独立包 `artifacts/connection-poc/windows-service-trust.zip`：三份 exe、manifest、脚本与本文，无 SDK/管理员要求，无旧安装包依赖。正常桌面 PowerShell 解压目录运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run-service-trust-validation.ps1
```

要求四个固定测试全部 PASS、无 SKIP、退出码 0、最终 `VALIDATION PASSED`。脚本逐份核对 hash，恢复原环境变量；OS 随机身份 teardown 精确清理，不广泛删除 Credential Manager。测试用途包含本机回环 HTTPS，不是公网/Headscale/GUI 验收。Windows 实际结果待回传；包装脚本已在 Windows 只读 AST ParseInput 通过。hash 是完整性检查，不是分发真实性/代码签名。

## 整体切片 4 剩余收口

1. 确定独立预发布/正式信任配置：真实发布根保管、在线 issuer、稳定 API CA/endpoint；不把本轮内存 fixture 当安装锚。
2. 平台可信锚和 epoch floor 的安全持久化、更新/轮换/失去配置后的显式恢复；活动会话更新先关闭，禁止旧 grant/旧 issuer 自动复活。
3. 消除产品 HMAC 的父 fixture 分发：grant/peer/证书绑定的受保护会话凭据取得及产品私有子进程投递，拒绝日志/argv/全局可读文件泄漏。
4. 产品 Swift/C# IPC/生命周期、GUI 设备码/明确授权与独立安装打包；完成相应 Windows/Mac 实机验证。旧安装产品仍为系统 Tailscale 基线，不要用旧包验证新架构。

已向用户请求选择创建独立自托管预发布配置或提供既有正式配置，未收到确认前不生成/部署生产发布身份，不改业务 443。需要根/服务配置选择才能继续正式发布收口，不能宣布整体 4 已完成。
