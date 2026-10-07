# 4C-4F：私有前端网络路由与跨进程 HTTPS

2026-10-07代码、全量Go race/vet、Linux构建、两项实际Linux组件测试及完整真实隔离链路通过。**未安装线上新binary/unit，未开放公网network，整体4未完成。**

## 开关和边界

device-frontend原参数不变，默认 `/v1/network` 404。明确追加 `--private-network` 才允许转发；必须有独立broker且监听IP为loopback。standalone服务不能启用此模式，0.0.0.0/非回环地址拒绝，不提供额外HTTP控制开关。

仍验证发布签名profile、固定root/service/epoch floor、TLS证书/CA/域名/期限，以及broker的kernel UID和精确issuer。frontend没有Store、issuer私钥、verifier或Headscale CLI；只转发严格的device/network请求，实际授权仍在daemon内。

保留TLS1.3/HTTP1.1、禁keepalive、32 socket、Admission socket-peer配额、读header2秒/读body5秒、device 4秒和network 14秒预算。私有模式WriteTimeout为20秒，以覆盖读取+网络执行；默认仍5秒，device执行预算不变。设备和network handler各有4个解析/调用slot，broker工作slot仍共享4；不是无限并发。

网络JSON/body和完整IPC frame仍有8192/8191限，重复/大小写别名/未知/尾随拒绝；无Origin、无query、Content-Type和method严格。未知路径404。broker消失或拒绝刷新返回503，绝不回退本地Store/私钥/离线授权。

## 自动与真实 Linux 验证

新增 `TestPrivateFrontendRequiresLoopbackAndBroker`、`TestPrivateFrontendNetworkBudgetAndNoFallback`，实际Linux均PASS无SKIP：拒绝standalone/public监听；真实HTTPS→Unix broker，模拟network执行6秒能正常返回503且worker deadline仍13–14秒；严格解析、未知路径、停止broker后503。模拟executor结果不冒充真实Headscale证明。

完整真实入口：

```sh
python3 -B poc/selfhost/public-server/daemon-network.py --frontend
```

需先构建Linux `daemon-network-check`、`authority-daemon-network`、`device-frontend-private`。上传后核对哈希，空PoC控制面临时database模式；服务器新建随机隔离材料，root独立daemon+真实verifier，使用SysProcAttr直接以nobody UID/GID运行独立frontend，监听随机127.0.0.1端口。签名profile/CA/root是测试运维锚，不等于产品客户端安装信任；测试设备key为随机内存Ed25519，不是Windows/macOS OS vault。

最新完整退出码0：HTTPS签名设备注册200、同请求重放403；已注册签名设备的无grant网络请求403（不是503），不会Mint节点；重复/尾随/过大body400、未知路径404、错误API CA握手拒绝。正常停authority，frontend持续返回device/network503且不重建Store；重启authority仍拒绝旧注册请求重放。

实际停止PoC控制面使verifier/daemon故障非零退出，frontend network503；provider恢复仍503、不自动重启daemon；同key/IP显式恢复daemon后frontend自动重连IPC，无grant仍403，设备重放仍403。frontend本身未重启或被授予独立授权。最后authority/frontend正常STOPPED、零退出/join；注册后的Store字节保持不变，证明恢复没有额外状态/issuer重建。

首次完整跑到末尾失败：前端经runuser包装，向包装进程发送TERM影响退出判断；不记整轮PASS。改直接非root Credential启动，对真实PID TERM并验证STOPPED/零退出，完整复测通过。scanner先读完stdout再Wait，防止最后状态行被提前关闭丢失。

精确Retire本轮verifier ticket，实验节点/users/preauthkeys零、目录与上传程序移除，原file/原PoC规则恢复。独立SSH线上三服务active，State哈希仍5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca，Nginx全部OK；没有修改业务443、系统Tailscale或公网路由。

## 仍待

本轮没有Windows/macOS OS signer、设备批准grant、真实客户端nodeproof或产品TLS载荷。下一步将双机完整授权/nodeproof/TLS流程切换到独立daemon+frontend，不以旧fixture PASS代替；之后受控部署。公网admission/反代、安装锚/floor/轮换、HMAC安全投递、GUI/安装仍待。
