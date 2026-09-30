using System.ComponentModel;
using System.Windows;
using RemoteProtocol;

namespace RemoteAgent;

public partial class MainWindow : Window
{
    private CancellationTokenSource? sharing;
    public MainWindow() { InitializeComponent(); }
    private async void Start_Click(object sender, RoutedEventArgs e)
    {
        if (sharing is not null) return;
        var allowControl = ControlConsent.IsChecked == true;
        if (allowControl && MessageBox.Show(this,
            "启用后，已认证的 Mac 可以真实移动鼠标、点击和输入按键。\n\n" +
            "许可仅限这一次共享；可随时点击“停止共享”立即结束。是否继续？",
            "确认允许本次远程控制", MessageBoxButton.YesNo, MessageBoxImage.Warning,
            MessageBoxResult.No) != MessageBoxResult.Yes)
        {
            ControlConsent.IsChecked = false;
            ControlText.Text = "未允许：本次共享只发送画面";
            return;
        }
        using var lifetime = new CancellationTokenSource();
        sharing = lifetime;
        var quality = QualitySelector.SelectedIndex switch { 0 => 40, 2 => 85, _ => 70 };
        QualitySelector.IsEnabled = false;
        ControlConsent.IsEnabled = false;
        StartButton.IsEnabled = false;
        StopButton.IsEnabled = true;
        MetricsText.Text = "等待首帧统计";
        try
        {
            StatusText.Text = "正在检查 Tailscale 和配对凭据";
            var endpoints = await TailscaleEndpoints.DiscoverAsync(lifetime.Token);
            using var certificate = AgentCertificateStore.LoadOrCreate();
            using var credentials = AgentCredentialStore.LoadOrCreate();
            FingerprintText.Text = CertificateFingerprint.FromCertificateDer(certificate.RawData).Hexadecimal;
            ControlText.Text = allowControl
                ? "本次已允许真实控制 · 等待已认证的 Mac"
                : "本次只读 · 不接受输入";
            StatusText.Text = "等待已配对的 Mac 连接 · 端口 47475";
            InputSimulationOptions? inputSession = allowControl
                ? new InputSimulationOptions(
                    () => new SessionNativeInputSink(snapshot => Dispatcher.BeginInvoke(() =>
                        ControlText.Text = $"远程控制中 · 事件 {snapshot.Events} · 释放 {snapshot.Releases} · 持有 {snapshot.Held}")),
                    LocalControlAllowed: true)
                : null;
            await Task.Run(() => TlsProbeServer.RunOnceAsync(endpoints.Local, endpoints.Peer, 47475,
                certificate, credentials.DeviceKey, credentials.AgentIdentifier, lifetime.Token,
                createJpegSource: () => new PrimaryScreenCapture(quality),
                reportStatus: text => Dispatcher.Invoke(() => StatusText.Text = text),
                reportMetrics: metrics => Dispatcher.Invoke(() => MetricsText.Text =
                    $"帧 {metrics.FrameNumber} · 采集+编码 {metrics.CaptureMilliseconds:F0} ms · 网络写入 {metrics.SendMilliseconds:F0} ms · 本帧总耗时 {metrics.FrameMilliseconds:F0} ms · 每帧 {metrics.JpegBytes / 1024.0:F1} KiB"),
                inputSession: inputSession), lifetime.Token);
            StatusText.Text = "会话已结束；再次共享请点击开始";
        }
        catch (JpegTransferTimeoutException)
        {
            StatusText.Text = "共享结束：发送画面超过 10 秒（网络发送超时）";
        }
        catch (OperationCanceledException) when (lifetime.IsCancellationRequested) { StatusText.Text = "共享已由本机停止"; }
        catch (OperationCanceledException) { StatusText.Text = "共享结束：等待连接或应用认证超时"; }
        catch (System.IO.EndOfStreamException) { StatusText.Text = "共享结束：Mac 已关闭连接"; }
        catch (Exception ex)
        {
            // Only fixed type labels: never echo raw remote data or credentials.
            StatusText.Text = "共享已结束（" + ex.GetType().Name + "）。请确认两端 Tailscale 在线、端口空闲且已配对。";
        }
        finally
        {
            if (MetricsText.Text.StartsWith("帧 ", StringComparison.Ordinal))
                MetricsText.Text = "最后成功发送：" + MetricsText.Text;
            sharing = null;
            QualitySelector.IsEnabled = true;
            ControlConsent.IsChecked = false;
            ControlConsent.IsEnabled = true;
            ControlText.Text = "未允许：下一次共享默认为只读";
            StartButton.IsEnabled = true;
            StopButton.IsEnabled = false;
        }
    }
    private void ControlConsent_Click(object sender, RoutedEventArgs e)
    {
        if (sharing is null)
            ControlText.Text = ControlConsent.IsChecked == true
                ? "已选择：开始共享时还需确认，之后会真实操作 Windows"
                : "未允许：本次共享只发送画面";
    }
    private void Stop_Click(object sender, RoutedEventArgs e) => sharing?.Cancel();
    private void Window_Closing(object? sender, CancelEventArgs e) => sharing?.Cancel();
}
