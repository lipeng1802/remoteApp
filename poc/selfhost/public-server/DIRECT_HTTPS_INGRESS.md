# r19：明确启用的直接 HTTPS 网络入口（代码/本机验证完成，未部署）

新增 `device-frontend` 运维参数 `--public-network` 与 `deviceservice.Config.PublicNetwork`。这是独立直接TLS接入模式，不是把旧 `--private-network` 改成公网，也不代表已经对外开放或客户端独立连接全链路完成。

## 固定门禁

- 默认无mode参数仅开放设备路由，`/v1/network`仍404；private模式保持loopback-only。public/private不能同时启用，未知/重复参数拒绝。
- public模式必须由可信运维显式配置，且有独立broker、规范绝对socket路径、编译外运维固定的发布根/service/floor与有效签名profile。启动验证kernel server UID与真实Store issuer，不创建本地Store或自动回退。
- 发布签名endpoint必须为非localhost的命名域名，不接受IP/localhost bootstrap作为公共服务配置；监听仍只能是字面IP，其端口必须与签名endpoint一致且规范非特权端口。0.0.0.0绑定需要该模式显式选择，不会自动发现地址或换端口。生产8444为候选；业务443未改。
- TLS1.3/独立CA与域名叶证书校验；服务绝对到期即停止。仅 `/v1/request` / `/v1/network` 路由，POST、精确Content-Type、无Origin/query；未知/重复/大小写重复/超长JSON拒绝。authority保留签名、nonce、明确审批、节点持有证明、绑定/租约和后台健康门禁。
- `broker.SignedNetworkHandler` 两路共享最多4个工作槽；保留32个已接受socket、header/read/idle/write期限。network broker调用14秒，响应write预算20秒；没有无界排队或秘密/原错误日志。
- admission在解析前共享真实socket peer配额：256桶（IPv6 /64）、peer600/min、global6000/min；忽略Forwarded/X-Forwarded-For。不宣称分布式DDoS防护、代理真实IP支持或跨实例配额；不支持直接用Nginx反代代替这些边界。
- backend无监督网络配置、authority不可用、issuer不符/UID错误都fail-closed，不因frontend READY或HTTP200启动网络。代理仅有签名请求权限，没有issuer/key/任意命令执行接口。

## 验证证据

本机相关5包race、专用vet、Python78项PASS。新增3项服务测试及CLI模式测试涵盖：public必须broker/命名profile、明确wildcard合法/private混用拒绝、relative socket拒绝、原loopback bootstrap拒public；真实本机TLS+独立Unix broker+唯一Store的签名注册/重放、重复alias/8193-byte/query/Origin拒绝、health404、关闭模式network404、frontend重启仍拒持久重放、authority停止两路503/正常join；四个阻塞network工作期间device请求503且实际broker调用仍为4。

服务测试使用保留测试域名的签名profile，本机HTTP测试客户端明确TLS ServerName访问127.0.0.1；这是服务边界测试，不是产品DNS解析/public可达验收。authority测试没有启用真实网络backend；预算测试使用memory executor，不把其503当Headscale授予/拒绝的证据。既有broker签名/UID/网络门禁回归继续通过。

Linux amd64静态前端构建 `artifacts/connection-poc/device-frontend-direct-r19` 成功，仅本机artifact，未上传替换或运行新服务。

## 实际服务器只读前置

`artifacts/connection-poc/direct-https-preflight-r19.py` actual exit0：正式签名profile epoch1当前有效；8444仅127.0.0.1绑定；frontend无private/public network flag，authority无network-config flag；authority/frontend/poc全部active，正式Store固定摘要与Nginx基线不变。初次检查误用/opt材料路径失败，改按已有部署的 `/var/lib/remoteapp-prerelease/client-public/service-profile.json` 后成功；未读取私钥或修改任何服务器内容。

没有开放公网/API安全组，没有更换binary/unit/profile/根、写设备状态、启动网络节点或修改Headscale策略，也没有新增Windows操作。

## 下一项

先准备长期authority监督网络配置/安装权限/固定peer与verifier生命周期装配、迁移预检/备份及不覆盖合法新状态的回退流程；再在明确授权下更新前端并开放候选8444，验实际公网负向/预算/服务故障与安装配置客户端直连。开启前后检查业务443与Nginx、服务状态、Store及网络资源；不把只读preflight、构建或frontend显式mode当已部署验收。

受保护产品凭据/HMAC供应、客户端自主enroll/bind、GUI、签名安装包仍未完成，整体切片4未完成。
