# 4C-4C：新版真实双机协调入口（实机待验）

本轮完成代码、自动测试与Linux构建，**尚未运行新版完整真实双机验收**。Windows已由用户正常停止；线上authority/frontend/device-only配置与Headscale策略本轮未动。整体4未完成。

## Windows准备

沿用已有windows-native-network包，签名协议未修改，不要求装旧产品安装包。正常桌面PowerShell运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\start-interactive-vault.ps1
```

看到READY后保持窗口，不按Enter。用户上次目录：H:\chatgpt\远程软件开发\测试安装包\windows-native-network。只有重新运行实际pinned probe成功后才允许开始服务器策略实验；旧bootstrap文件存在不代表owner存活。结束后总控确认资源回收再按Enter，并确认STOPPED。

## Mac新入口

```sh
REMOTEAPP_WINDOWS_INTERACTIVE_DIR='H:\chatgpt\远程软件开发\测试安装包\windows-native-network' \
python3 -B poc/selfhost/public-server/enrollment-network.py --supervised
```

先使用最新源码构建artifacts/connection-poc/enrollment-fixture（Linux amd64）；沿用既有同协议Mac/Windows helper/API及OS vault构建。新模式固定启动 --native-supervised，并要求public ready supervised=true，不能用旧--native输出代替。仍通过可信SSH管理回环TLS/临时材料，非最终公网产品流程。

## 新链路与断言

- 服务端独立随机Store和persistent-verifier Runtime；真实注册/批准/网络签名由两端OS signer完成，不输出/代签设备私钥。
- HTTPdevice/network都经真正kernel UID Unix broker，再进入supervisor Authority；私有fixture仍同root进程/UID，不将其当线上不同UID部署验收。TLS/API信任仍测试bootstrap，产品根安装/HMAC分发未完成。
- Proof从Store取可信设备公钥，runtime-own overlay做nodeproof，固定诊断标签经受控report回传，不打印密钥/凭据。real Headscale driver使用database策略和0700实验目录。
- 早期绑定/真实零ACL阶段使用一分钟维护间隔并显式reconcile；zero先stop/join supervisor，再下发零规则，reconcile显式新backend/supervisor。周期故障阶段显式切换500ms维护。这个安排是测试阶段控制，不能当生产故障检测指标。
- policy fault期间旧ACL不能更新，network刷新应被拒绝。provider恢复后继续锁闭，必须显式Stop/join旧supervisor、打开原Store并创建新backend/supervisor，不能采用旧流程“自动retry恢复”断言。
- Store故障实验在stop时注入provider故障，确保最终零策略尝试不可用，保留旧ACL以检查产品TLS关闭；正常stop仍尝试零策略。
- 撤销阶段周期清理可能已移除绑定，因此接受明确签名inactive或拒绝刷新，但不能接受任何active。不能宣称该阶段必然保留旧ACL；旧ACL仍在时TLS关闭由policy/Store故障阶段单独验收。
- 完成撤销/回收后，额外停止真实verifier SDK并join、确认Live/Ready=false与刷新拒绝；显式同key/IP resume、新supervisor也不可复活已撤销grant。不是模拟Live布尔或重新Mint。
- 成功teardown：先supervisor/HTTP/broker join、实验授权资源零策略及逐ticket回收，再Runtime stop/retire；不合作join/未知注册结果或清理失败保留journal，不盲删身份目录或宣称清理成功。

## 自动测试

新fixture替身控制面+真实Runtime生命周期、真实HTTPS/kernel IPC race测试通过：设备签名注册/重放、policy故障拒绝、服务恢复不自动开门、显式新实例恢复、冷Store仍拒绝同nonce。Python纯断言测试覆盖缺失marker/假阳性/positive混入失败回包不得算拒绝，provider恢复必须显式false就绪，撤销只接受拒绝或明确inactive。完整Go race/vet与Linux fixture构建通过；这些不等同真实Headscale/双机/产品TLS验收。

还没有新增300秒自然grant到期、整机断电/崩溃恢复、DERP/直连分类证据，需另外计划。新版实机跑完记录实际退出、节点/users/preauthkeys回收和policy/Nginx恢复，并请求Windows正常STOPPED确认。
