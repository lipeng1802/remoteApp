# 4C-3C：daemon / HTTPS frontend 适配与迁移前置

4C-3C 完成进程入口；4C-3D 已在服务器迁移 Store 并安装两份 unit。旧 remoteapp-device 已停止，新 authority/frontend active、均 disabled（未开机启用）。network 仍关闭；不是整体切片 4 完成。

authority-daemon 只打开已有 Store，核对签名 profile / root / service / floor / issuer；独占直到 socket worker join 后释放。profile 到期停止，缺状态不创建新 issuer。无 TLS listener、无 Headscale backend。frontend 不打开 Store，只通过 kernel peer UID 认证的固定 broker 操作处理设备 HTTPS，启动及每次成功响应核对 issuer；authority 不可用返回 503，没有本地状态回退。独立 identity 操作只返回公钥，不导出 issuer 私钥。

模板中的 root、精确 frontend UID/GID 必须替换为已验证值。本次 CentOS 实机 frontend UID=995/GID=993，authority CAP_CHOWN=1、frontend capabilities=0、两者 NoNewPrivileges=yes。authority 暂保留 CAP_CHOWN，仅用于创建 root:frontend-group socket；没有开放网络授权权限。frontend 仍持有其 TLS 私钥，但不得持有 issuer/state 或访问 Headscale 数据库。

## 受控部署步骤（本次已执行）

1. 核对现有 issuer/profile、线上 binary/state 哈希、服务与 Nginx 基线。构建与暂存两个新 binary，验证哈希；不要复制本地初始 authority snapshot 覆盖线上状态。
2. 停止旧 remoteapp-device，确认 8444 listener 消失、writer.lock 正常释放。锁未释放时停止迁移，不能盲删锁。确认没有第二个 Store writer。
3. 在 root-only、全新备份目录保留当前线上 state、原 binary、unit 与材料权限清单。不打印 state。备份也包含在线私钥，需限制读取和后续管理。
4. 建立 /etc/remoteapp-authority（root:root 0700）及 /var/lib/remoteapp-authority（root:root 0700）。将**刚停止的线上最新 Store**迁移到后者，state 0600 root:root。profile 复制到前者并验证签名。检查真实路径、祖先所有者及无非 root 写权限。
5. socket 目录 /run/remoteapp-authority 必须 root:remoteapp-pre 0710（重启后需可信 tmpfiles 配置重建）；socket 由 daemon 创建为 0660。不能把 root Store 留在 frontend 可写父目录里：只 chown 子目录不构成隔离。
6. 旧 frontend 目录不得残留 authority state/备份。安装渲染后的 unit，daemon 先启动并检查固定 READY，再启动 frontend。Type=simple/After 不保证 socket ready，必须显式确认；启动失败无自动退回旧服务。
7. 以 remoteapp-pre 实际测试不能读取 state、不能替换 root profile/socket目录、不能读取 Headscale DB；验证签名 HTTPS、nonce replay、frontend/daemon分别停止与重启、state issuer不变、单writer，以及 authority退出后的503。维持回环8444、network404、Nginx基线不变。未通过不得开放公网或启用network。

## 回滚边界

- 启动失败且**从未产生新写入**时，可在所有 writer 停止后核对 state 哈希与备份，人工选择回到原部署；这会撤销新权限隔离，必须显式记录。
- 新服务写入后不可直接恢复旧 snapshot：否则丢失 nonce、批准/撤销与设备绑定。旧 binary 不兼容新增 PublicKey 字段。应保留最新 state，使用兼容版本修复或保持 fail-closed；不得删字段、重置 issuer、自动再配对。
- 崩溃遗留 writer.lock 必须确认进程已退出、没有并行 writer、备份最新 state 后显式恢复，不能自动抢锁。

## 本轮测试证据

Mac Unix + HTTPS race 联调：frontend目录移除authority后签名注册成功、重放403、严格JSON拒绝；frontend停止不能释放daemon Store锁；daemon停止后HTTP503且无state回退；同一Store重启保留nonce；错误server UID启动拒绝；缺state拒绝且不重新生成。此单UID测试验证链路与生命周期，不冒充Linux root/non-root实际部署隔离。跨UID组件验收见 AUTHORITY_BROKER.md，线上结果见下节。

## 4C-3D 服务器实际结果

通过 migrate-authority.sh 核对二进制哈希、正常停旧服务、确认listener/锁释放，备份**线上最新** Store 后原路径迁移，不导入本地snapshot。root-only备份保留于 /var/lib/remoteapp-authority-backup.WUGsLoM5（含在线issuer，禁止上传/打印）。root Store/profile 0700目录、state0600；socket目录root:remoteapp-pre0710、socket0660。tmpfiles已安装，仅重建目录，不开启服务。

真实remoteapp-pre无法读取root Store/profile及存在的Headscale db.sqlite/config，不能写/替换socket目录或 /var/lib。前端无authority残留。实际TLS profile/独立CA/域名核验、malformed403、duplicate400、network404、health404通过；只读探针未注册或持久写设备。旧探针文字写“SSH forward”，本次实际运行于服务器指定回环，并非SSH数据转发；新版已更正为explicit loopback并输出预期device状态。

单独停止/重启frontend时authority active且writer.lock保留。systemctl kill --signal=TERM authority 后锁/socket消失、frontend仍active，authority-down探针要求503并通过；同Store启动authority后默认403探针恢复通过。state迁移前/后/所有探针/两种重启哈希均为5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca，issuer未变。服务Result=success/ExecMainCode=0/ExecMainStatus=0；Nginx主配置/conf.d基线全部OK，remoteapp-poc保持active。

没有新增公网8444/代理规则、改业务443、启用Headscale授权、使用系统Tailscale、上传发布根/CA钥匙。线上没有新增签名注册/nonce写入证据，durable replay来自4C-3C联调。未做断电/崩溃锁恢复/实际恢复备份；回滚为上方受控流程，不宣称实测恢复。下一步接verifier/nodeproof/maintenance/fence真实backend仍需独立验收。
