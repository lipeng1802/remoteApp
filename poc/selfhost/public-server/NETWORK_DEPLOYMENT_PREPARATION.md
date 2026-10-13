# r20 长期网络后端部署准备（未部署）

r20 提供只读代码预检与候选 unit，当时未上传/部署。后续 r21 通过随机 `/run/systemd/system` 测试unit进行目标CentOS候选验收，记录见 [候选服务验收](SYSTEMD_NETWORK_ACCEPTANCE.md)。正式服务仍 device-only；未创建长期 verifier、未持久迁移策略、未开放端口。候选仍不是已验收的生产配置。

## 只读入口

沿用 authority-daemon 的八个固定参数，追加：

```
--check-network-config /etc/remoteapp-authority/network.json
```

成功仅输出固定 `CHECK PASSED`，失败固定 `FAIL network_preparation`/exit1，不输出路径、journal、节点身份或密钥。入口使用现有严格 network JSON、同 effective UID、0600文件/0700目录、规范绝对路径/无symlink及目录隔离规则。检查发行根/service/floor/有效期签名profile、已有私有Store文件、可恢复issued/ready verifier journal和私有node状态文件。writer.lock/owner.lock即使是悬空链接或疑似遗留也拒绝，绝不清理。

不调用 invite.Open、verifier.Open、Headscale CLI、overlay 或 bind；不创建/初始化/重键/修改文件。预检要求空闲状态，不能拿它检查运行中服务。通过不证明 Store issuer 匹配、node私钥与记录一致、provider在线、database策略模式、零ACL网络拒绝或前端UID握手。检查是瞬时快照，不预留资源；真实启动仍重新校验并独占，不能用预检绕过启动门禁。

## 候选 unit 与启用前门禁

`remoteapp-authority-network.service.candidate` 增加 ExecStartPre、显式network-config、专属verifier/policy可写目录和45秒停止预算（旧unit为10秒，短于daemon的30秒join预算）。保留root authority、跨UID前端、Restart=no；不会自动enable。路径必须与严格network.json一致，替换公开root/UID/GID后再校验。使用Wants/After而不是Requires，避免provider停止被systemd转换为普通依赖停止；依赖并不代表provider健康。

r21 实测修正候选能力边界：`CAP_CHOWN CAP_DAC_OVERRIDE CAP_NET_RAW`。固定CLI需要访问另一服务UID的0700目录/0600 admin socket，DAC override保留原provider私有权限；pinned tsnet在root/CentOS且SO_MARK不可用时的SO_BINDTODEVICE回退需要NET_RAW，不授予NET_ADMIN。该能力组合只用于可信sole authority固定操作，不给前端或客户端；未来拆分CLI权限需要独立设计，不能宣称它是所有平台的最小能力。保留PrivateTmp、PrivateDevices、ProtectHome、ProtectSystem、NoNewPrivileges。正式安装/长期运行门禁仍需后续完成。

启用顺序必须在受控维护窗口执行，本轮没有执行：

1. 记录现有binary/unit/config/policy及业务Nginx摘要，核对旧服务无未经审批的会话。备份私有材料必须留在root私有服务器目录，不传客户端或提交仓库。
2. 在隔离控制面验证候选unit、准备精确长期verifier身份并确认stop/join。缺失材料不得以临时身份代替，不能从启动路径Mint。
3. 现网file策略模式必须独立评估database迁移；不能仅把driver参数设为database。验收投影、零ACL双向实际拒绝、重启与回退后才能迁移。
4. 停止前端接收并取消/join请求，停止authority并确认正常join、锁释放。检查实际进程退出，不能仅信任systemctl inactive。
5. 空闲预检通过、候选配置复核后替换原authority unit并显式启动；不能把候选安装成另一个并行authority。用批准跨UID验证issuer、实际supervisor ready，再保持前端仅回环进行真实签名请求/故障闭环。
6. 公网8444开放是另一门禁：profile有效性/信任供应、监听与安全组、负向预算/故障验证完成后才批准；业务443不修改。HMAC/产品凭据供应仍待，不算自主GUI会话。

## 安全回退原则

- 启动失败保持前端关闭；Restart=no，不自动重新注册verifier或删除锁。
- provider故障、撤策略失败、超时或被systemd强杀不算STOPPED。保留journal/Store/锁，核对全部实际worker退出及策略状态，再由明确恢复流程处置；预检不修复。
- 回退仅恢复审核过的binary/unit/控制配置，恢复前先确保新worker退出且访问已撤回；保留当前合法Store/journal及epoch floor。不得将维护前Store快照覆盖期间的新邀请、撤销或工单。
- 旧device-only服务恢复也必须重新独占当前Store、核验issuer并通过前端握手。不能把恢复旧file ACL当作安全零规则；残留旧授权需先确认真实数据面拒绝。

本轮测试为本机静态材料/race生命周期回归，Linux静态编译；不是线上database迁移、长期verifier或候选unit实机验收。下一项应在隔离Linux服务环境完成候选unit与真实后台生命周期组合，再决定维护窗口部署。
