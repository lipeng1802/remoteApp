# 720p 画质交接：0.2.4

## 当前结果与限制

用户澄清上一轮断开是 Mac 网络问题；恢复后低带宽和标准档均约 9.4 FPS，文字仍模糊。Mac 现在不可连接。本轮只改 Windows，不提高分辨率，不改配对或 Mac 协议，不开发输入/H.264。

三档 JPEG 质量：低带宽40、标准70（默认）、清晰85。均最高1280×720、10FPS，停止后才能换档。清晰档需要更多带宽，不能恢复4K缩成720p丢失的小字细节，不承诺不卡顿。结束后的指标明确标记“最后成功发送”。

Windows Release 0警告/错误、32/32协议测试通过；GUI默认标准、三档存在、可选清晰、正常关闭exit0通过。自包含0.2.4与Setup打包成功，未安装/卸载验收。

同一固定合成图1280×720每档编码10次并解码，尺寸保持一致：

|质量|字节/帧|编码中位ms|编码最大ms|10FPS估算载荷Mbps|
|---|---:|---:|---:|---:|
|40|138327|2.13|—|11.07|
|70|211244|2.37|2.42|16.90|
|85|288500|2.47|2.76|23.08|

仅合成纹理/渐变/细边缘，不是实际桌面、网络FPS或清晰度结论。此样本清晰档比标准多约37%字节。没有保存或发送屏幕图像。

**本轮真实主屏采集在 StretchBlt 返回失败，环境原因未确认，未通过。** 已使错误文字明确指出调用。采集与编码分离、GDI资源在编码前释放，仍需真实桌面回归；不能沿用旧版31次成功作为本轮证据。Mac源码未改，也未重跑Mac测试。

## 按顺序验证

### 1. Windows 本地真实采集复验

在普通已登录桌面、关闭安全桌面弹窗后，于仓库根目录运行：

~~~powershell
dotnet run --project .\windows\RemoteAgent\tests\ScreenCapture.Tests\ScreenCapture.Tests.csproj -c Release -- --capture-in-memory
dotnet run --project .\windows\RemoteAgent\tests\ScreenCapture.Tests\ScreenCapture.Tests.csproj -c Release -- --compare-quality-in-memory
~~~

预期第一条31次采集、解码、低带宽、取消检查通过，无GDI增长；第二条同一真实快照三档编码后尺寸一致，输出JSON统计，不保存图像。若仍有StretchBlt失败，先定位本地采集，暂停后续画面验收。无可用桌面时可独立运行合成对比：

~~~powershell
dotnet run --project .\windows\RemoteAgent\tests\ScreenCapture.Tests\ScreenCapture.Tests.csproj -c Release -- --compare-quality-synthetic
~~~

### 2. Mac 恢复后核对代码和构建

ce79973连续接收版本已兼容Windows三档。**本轮Windows改动和文档尚未提交；只有提交推送后，Mac pull才能获得新交接。** 有本地修改先保留处理，不用reset --hard。

Mac仓库根目录：

~~~bash
git status --short --branch
git pull --ff-only origin main
git log -1 --oneline
cd macos/RemoteController
swift test
swift build -c release
swift run -c release RemoteController
~~~

预期96 tests、0 failures、Build complete。保留原地址字符串、Keychain、证书。后续P2鼠标4项及键盘3项测试加入，再加Mac映射5项、输入采集10项及发送队列/状态机22项和网络调度12项，当前累计预期96项，Mac离线待验证。

### 3. 标准档60秒对照

关闭旧Windows Agent/占用47475的探针。在Windows仓库根目录执行：

~~~powershell
& .\artifacts\windows\jpeg-quality\RemoteAgent.exe
~~~

默认标准档，开始共享后Mac连接。使用同一普通文字窗口，记录60秒Mac FPS范围、清晰度、延迟以及Windows完整指标行。Windows可通过UI Automation仅仅读取StatusText/MetricsText，无需人工抄写；不要同时跑网络测速。记录是否仍不同物理网络。

### 4. 清晰档60秒对照

Windows停止→清晰档→开始→Mac重连。保持窗口、字体和查看器大小相同，比较每帧大小、写入耗时、Mac FPS和文字可读性。编码尺寸应不变。若持续降帧或写入阻塞，退回标准；不以本地编码速度推断网络不卡顿。若仍模糊，记录4K→720p缩小/显示放大的影响，不擅自提高分辨率。

### 5. 双向停止和重连

分别测试Mac断开、Windows停止：两端窗口保留，Mac清屏、Windows开始按钮恢复；再次开始/连接应重新认证。结束后Windows指标前应有“最后成功发送”。若再次意外超时，记录双方状态和当时网络，再增加Mac接收/解码诊断，不将旧成功帧0ms误当作阻塞帧。

### 6. 跨网络30分钟与安装

选可接受档位，保持不同网络运行30分钟，记录FPS、CPU/内存趋势、断开和清晰度。P1原目标仍为720p至少10FPS；约9.4FPS未满足字面目标，不将接近目标改记为通过，后续再检查节拍或接收表现。

最后验收 artifacts/windows/PersonalRemoteDesktopAgent-0.2.4-win-x64-Setup.exe 的安装/启动/卸载；无预装.NET环境独立保留。SHA-256：C24034E66EA1528C266ABF9D49EE0BB43E3A6B7C4C84D245E48E536C89B85B29。保留发布目录依赖，不单独移动exe。Mac .app/DMG回归尚未重做。

## Mac Codex接收提示

接收本文件与HANDOFF顶部最新记录，按1至6顺序验证。Windows真实采集复验优先，随后96项测试、标准/清晰对照、双方停止和30分钟验收。区分实测与待办，不把合成图基准或9.4FPS记成阶段通过，不开始输入、H.264或自动提高分辨率。
