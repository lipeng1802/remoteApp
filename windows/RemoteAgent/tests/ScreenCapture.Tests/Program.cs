using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Windows.Media.Imaging;
using System.Windows.Media;
using RemoteAgent;

// Opt-in smoke test: captures the current primary screen only into memory.
// Never saves or prints screen content and never opens a network listener.
if (args.Length == 1 && args[0] == "--compare-quality-synthetic")
{
    CompareQuality(synthetic: true);
    return 0;
}
if (args.Length == 1 && args[0] == "--compare-quality-in-memory")
{
    CompareQuality(synthetic: false);
    return 0;
}
if (args.Length != 1 || args[0] != "--capture-in-memory")
{
    Console.WriteLine("Usage: ScreenCapture.Tests --capture-in-memory | --compare-quality-in-memory | --compare-quality-synthetic");
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
using (var lowBandwidth = new PrimaryScreenCapture(40))
{
    var reduced = lowBandwidth.Capture(deadline.Token);
    Validate(reduced.Jpeg);
    if (reduced.Screen != first.Screen) throw new Exception("Screen metadata changed during quality check; retry with stable display settings.");
    Console.WriteLine($"PASS low-bandwidth quality 40 in-memory capture and decode; bytes={reduced.Jpeg.Length}");
}
using var cancelled = new CancellationTokenSource();
cancelled.Cancel();
try { source.Capture(cancelled.Token); throw new Exception("Cancelled capture accepted"); }
catch (OperationCanceledException) { Console.WriteLine("PASS capture cancellation"); }
using var fixture = JsonDocument.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "testdata", "jpeg-v1.json")));
Validate(Convert.FromHexString(fixture.RootElement.GetProperty("jpegHex").GetString()!));
Console.WriteLine("PASS shared synthetic JPEG decodes");
return 0;

static void CompareQuality(bool synthetic)
{
    using var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(30));
    var captureWatch = Stopwatch.StartNew();
    var image = synthetic ? SyntheticImage() : PrimaryScreenCapture.CaptureImage(deadline.Token).Image;
    var captureMilliseconds = captureWatch.Elapsed.TotalMilliseconds;
    var results = new List<object>();
    foreach (var quality in new[] { 40, 70, 85 })
    {
        // Warm up each mode; every encode uses the very same in-memory snapshot.
        Validate(PrimaryScreenCapture.EncodeImage(image, quality));
        var durations = new List<double>();
        long bytes = 0;
        for (var repeat = 0; repeat < 10; repeat++)
        {
            deadline.Token.ThrowIfCancellationRequested();
            var timer = Stopwatch.StartNew();
            var jpeg = PrimaryScreenCapture.EncodeImage(image, quality);
            durations.Add(timer.Elapsed.TotalMilliseconds);
            bytes = jpeg.Length;
            using var stream = new MemoryStream(jpeg);
            var decoded = new JpegBitmapDecoder(stream, BitmapCreateOptions.None, BitmapCacheOption.OnLoad).Frames.Single();
            if (decoded.PixelWidth != image.PixelWidth || decoded.PixelHeight != image.PixelHeight)
                throw new Exception("Quality mode changed encoded dimensions.");
        }
        durations.Sort();
        results.Add(new {
            quality, bytes, encodeMedianMs = Math.Round((durations[4] + durations[5]) / 2, 2),
            encodeMaxMs = Math.Round(durations[^1], 2),
            estimatedPayloadMbpsAt10Fps = Math.Round(bytes * 8 * 10 / 1_000_000.0, 2)
        });
    }
    Console.WriteLine(JsonSerializer.Serialize(new {
        test = "same-snapshot-quality-comparison", source = synthetic ? "synthetic-pattern" : "primary-screen",
        width = image.PixelWidth, height = image.PixelHeight,
        snapshotPreparationMs = Math.Round(captureMilliseconds, 2),
        encodesPerQuality = 10, results,
        note = "In-memory only. Not network FPS or a visual clarity assessment."
    }));
}

static BitmapSource SyntheticImage()
{
    const int width = 1280, height = 720;
    var pixels = new byte[width * height * 4];
    var random = new Random(20260929);
    for (var y = 0; y < height; y++)
    for (var x = 0; x < width; x++)
    {
        var offset = (y * width + x) * 4;
        // Fixed gradients, fine edges and a textured patch; no captured content.
        var edge = x % 23 == 0 || y % 29 == 0;
        pixels[offset] = edge ? (byte)0 : (byte)(x * 255 / width);
        pixels[offset + 1] = edge ? (byte)0 : (byte)(y * 255 / height);
        pixels[offset + 2] = edge ? (byte)0 : (byte)(x > 960 ? random.Next(256) : 220);
    }
    var image = BitmapSource.Create(width, height, 96, 96, PixelFormats.Bgr32, null, pixels, width * 4);
    image.Freeze();
    return image;
}

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