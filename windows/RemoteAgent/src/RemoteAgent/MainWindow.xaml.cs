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
        using var lifetime = new CancellationTokenSource();
        sharing = lifetime;
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
            StatusText.Text = "等待已配对的 Mac 连接 · 端口 47475";
            await Task.Run(() => TlsProbeServer.RunOnceAsync(endpoints.Local, endpoints.Peer, 47475,
                certificate, credentials.DeviceKey, credentials.AgentIdentifier, lifetime.Token,
                createJpegSource: () => new PrimaryScreenCapture(),
                reportStatus: text => Dispatcher.Invoke(() => StatusText.Text = text),
                reportMetrics: metrics => Dispatcher.Invoke(() => MetricsText.Text =
                    $"采集+编码 {metrics.CaptureMilliseconds:F0} ms · 网络写入 {metrics.SendMilliseconds:F0} ms · 本帧总耗时 {metrics.FrameMilliseconds:F0} ms · 每帧 {metrics.JpegBytes / 1024.0:F1} KiB")), lifetime.Token);
            StatusText.Text = "会话已结束；再次共享请点击开始";
        }
        catch (JpegTransferTimeoutException ex)
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
            sharing = null;
            StartButton.IsEnabled = true;
            StopButton.IsEnabled = false;
        }
    }
    private void Stop_Click(object sender, RoutedEventArgs e) => sharing?.Cancel();
    private void Window_Closing(object? sender, CancelEventArgs e) => sharing?.Cancel();
}
