using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Windows.Media.Imaging;
using RemoteAgent;

// Opt-in smoke test: captures the current primary screen only into memory.
// Never saves or prints screen content and never opens a network listener.
if (args.Length != 1 || args[0] != "--capture-in-memory")
{
    Console.WriteLine("Usage: ScreenCapture.Tests --capture-in-memory");
    return 2;
}
using var source = new PrimaryScreenCapture();
using var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(30));
using var process = Process.GetCurrentProcess();
var first = source.Capture(deadline.Token);
Validate(first.Jpeg);
var handlesBefore = Native.GetGuiResources(process.Handle, 0);
var watch = Stopwatch.StartNew();
for (var i = 0; i < 30; i++) Validate(source.Capture(deadline.Token).Jpeg);
var handlesAfter = Native.GetGuiResources(process.Handle, 0);
if (handlesAfter > handlesBefore + 2) throw new Exception("GDI handles grew during capture.");
Console.WriteLine($"PASS 31 in-memory captures; source={first.Screen.Width}x{first.Screen.Height}; dpi100={first.Screen.DpiX100}; capture_fps={30 / watch.Elapsed.TotalSeconds:F1}; GDI growth={handlesAfter - handlesBefore}");
using var cancelled = new CancellationTokenSource();
cancelled.Cancel();
try { source.Capture(cancelled.Token); throw new Exception("Cancelled capture accepted"); }
catch (OperationCanceledException) { Console.WriteLine("PASS capture cancellation"); }
using var fixture = JsonDocument.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "testdata", "jpeg-v1.json")));
Validate(Convert.FromHexString(fixture.RootElement.GetProperty("jpegHex").GetString()!));
Console.WriteLine("PASS shared synthetic JPEG decodes");
return 0;

static void Validate(byte[] jpeg)
{
    using var stream = new MemoryStream(jpeg);
    var decoder = new JpegBitmapDecoder(stream, BitmapCreateOptions.None, BitmapCacheOption.OnLoad);
    var image = decoder.Frames.Single();
    if (image.PixelWidth is < 1 or > 1280 || image.PixelHeight is < 1 or > 720)
        throw new Exception("JPEG dimensions exceed configured limit.");
}
internal static class Native
{
    [DllImport("user32.dll")] internal static extern uint GetGuiResources(IntPtr process, uint flags);
}