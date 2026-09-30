using System.Diagnostics;
using System.Net;
using System.Text;
using System.Text.Json;

namespace RemoteProtocol;

public static class TailscaleEndpoints
{
    public static bool IsTailscaleIPv4(IPAddress address)
    {
        ArgumentNullException.ThrowIfNull(address);
        var bytes = address.GetAddressBytes();
        return bytes.Length == 4 && bytes[0] == 100 && bytes[1] is >= 64 and <= 127;
    }

    public static async Task<(IPAddress Local, IPAddress Peer)> DiscoverAsync(CancellationToken cancellationToken)
    {
        var executable = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "Tailscale", "tailscale.exe");
        using var process = new Process { StartInfo = new ProcessStartInfo(executable, "status --json") {
            UseShellExecute = false, CreateNoWindow = true, RedirectStandardOutput = true,
            RedirectStandardError = true, StandardOutputEncoding = Encoding.UTF8, StandardErrorEncoding = Encoding.UTF8
        }};
        process.Start();
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        deadline.CancelAfter(TimeSpan.FromSeconds(10));
        try
        {
            var output = process.StandardOutput.ReadToEndAsync(deadline.Token);
            var error = process.StandardError.ReadToEndAsync(deadline.Token);
            await process.WaitForExitAsync(deadline.Token);
            if (process.ExitCode != 0) throw new InvalidOperationException("Tailscale 状态读取失败");
            _ = await error;
            return Parse(await output);
        }
        finally { if (!process.HasExited) process.Kill(); }
    }
    public static (IPAddress Local, IPAddress Peer) Parse(string json)
    {
        using var document = JsonDocument.Parse(json);
        var root = document.RootElement;
        if (root.GetProperty("BackendState").GetString() != "Running" || !root.GetProperty("Self").GetProperty("Online").GetBoolean())
            throw new InvalidOperationException("请先连接 Windows Tailscale");
        var peers = root.GetProperty("Peer").EnumerateObject().Select(p => p.Value)
            .Where(p => p.GetProperty("OS").GetString() == "macOS" && p.GetProperty("Online").GetBoolean()).ToArray();
        if (peers.Length != 1) throw new InvalidOperationException("需要恰好一台在线 Mac，请检查 Tailscale");
        return (Address(root.GetProperty("Self")), Address(peers[0]));
    }
    private static IPAddress Address(JsonElement peer)
    {
        var addresses = peer.GetProperty("TailscaleIPs").EnumerateArray().Select(p => IPAddress.Parse(p.GetString()!))
            .Where(IsTailscaleIPv4).ToArray();
        if (addresses.Length != 1) throw new InvalidOperationException("没有唯一的 Tailscale IPv4 地址");
        return addresses[0];
    }
}
