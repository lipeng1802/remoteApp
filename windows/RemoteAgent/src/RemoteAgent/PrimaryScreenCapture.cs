using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using RemoteProtocol;

namespace RemoteAgent;

public sealed class PrimaryScreenCapture : IJpegFrameSource
{
    private readonly int quality;
    public PrimaryScreenCapture(int quality = 70)
    {
        if (quality is < 1 or > 100) throw new ArgumentOutOfRangeException(nameof(quality));
        this.quality = quality;
    }
    public CapturedJpeg Capture(CancellationToken cancellationToken)
    {
        var snapshot = CaptureImage(cancellationToken);
        cancellationToken.ThrowIfCancellationRequested();
        var jpeg = EncodeImage(snapshot.Image, quality);
        cancellationToken.ThrowIfCancellationRequested();
        return new CapturedJpeg(snapshot.Screen, jpeg);
    }

    // Shared with the opt-in quality comparison: encode the same pixels in each mode.
    internal static byte[] EncodeImage(BitmapSource image, int quality)
    {
        if (quality is < 1 or > 100) throw new ArgumentOutOfRangeException(nameof(quality));
        var encoder = new JpegBitmapEncoder { QualityLevel = quality };
        encoder.Frames.Add(BitmapFrame.Create(image));
        using var output = new MemoryStream();
        encoder.Save(output);
        return output.ToArray();
    }

    internal static (ScreenInfoPayload Screen, BitmapSource Image) CaptureImage(CancellationToken cancellationToken)
    {
        cancellationToken.ThrowIfCancellationRequested();
        var previousDpi = SetThreadDpiAwarenessContext(new IntPtr(-4));
        IntPtr screen = IntPtr.Zero, memory = IntPtr.Zero, bitmap = IntPtr.Zero, old = IntPtr.Zero;
        try
        {
            var width = GetSystemMetrics(0);
            var height = GetSystemMetrics(1);
            var dpi = GetDpiForSystem();
            var info = new ScreenInfoPayload((uint)width, (uint)height, dpi * 100, dpi * 100);
            _ = info.Encode();
            var scale = Math.Min(1, Math.Min(1280.0 / width, 720.0 / height));
            var targetWidth = Math.Max(1, (int)Math.Round(width * scale));
            var targetHeight = Math.Max(1, (int)Math.Round(height * scale));
            screen = GetDC(IntPtr.Zero);
            memory = CreateCompatibleDC(screen);
            bitmap = CreateCompatibleBitmap(screen, targetWidth, targetHeight);
            if (screen == IntPtr.Zero || memory == IntPtr.Zero || bitmap == IntPtr.Zero) throw new Win32Exception();
            old = SelectObject(memory, bitmap);
            if (old == IntPtr.Zero || old == new IntPtr(-1)) throw new Win32Exception();
            _ = SetStretchBltMode(memory, 4); // HALFTONE
            _ = SetBrushOrgEx(memory, 0, 0, IntPtr.Zero);
            if (!StretchBlt(memory, 0, 0, targetWidth, targetHeight, screen, 0, 0, width, height, 0x40CC0020))
                throw new Win32Exception("Primary screen capture failed (StretchBlt)."); // SRCCOPY | CAPTUREBLT
            _ = SelectObject(memory, old);
            old = IntPtr.Zero;
            var header = new BitmapInfo { Size = 40, Width = targetWidth, Height = -targetHeight, Planes = 1, BitCount = 32 };
            var pixels = new byte[checked(targetWidth * targetHeight * 4)];
            if (GetDIBits(memory, bitmap, 0, (uint)targetHeight, pixels, ref header, 0) != targetHeight)
                throw new Win32Exception();
            cancellationToken.ThrowIfCancellationRequested();
            var image = BitmapSource.Create(targetWidth, targetHeight, 96, 96, PixelFormats.Bgr32, null, pixels, targetWidth * 4);
            image.Freeze();
            return (info, image);
        }
        finally
        {
            if (old != IntPtr.Zero && memory != IntPtr.Zero) _ = SelectObject(memory, old);
            if (bitmap != IntPtr.Zero) _ = DeleteObject(bitmap);
            if (memory != IntPtr.Zero) _ = DeleteDC(memory);
            if (screen != IntPtr.Zero) _ = ReleaseDC(IntPtr.Zero, screen);
            if (previousDpi != IntPtr.Zero) _ = SetThreadDpiAwarenessContext(previousDpi);
        }
    }
    public void Dispose() { } // Every frame releases all native handles in finally.
    [StructLayout(LayoutKind.Sequential)]
    private struct BitmapInfo
    {
        public uint Size;
        public int Width, Height;
        public ushort Planes, BitCount;
        public uint Compression, ImageSize;
        public int XPelsPerMeter, YPelsPerMeter;
        public uint ColorsUsed, ColorsImportant;
        public uint Color;
    }
    [DllImport("user32.dll")] private static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context);
    [DllImport("user32.dll")] private static extern int GetSystemMetrics(int index);
    [DllImport("user32.dll")] private static extern uint GetDpiForSystem();
    [DllImport("user32.dll")] private static extern IntPtr GetDC(IntPtr window);
    [DllImport("user32.dll")] private static extern int ReleaseDC(IntPtr window, IntPtr dc);
    [DllImport("gdi32.dll")] private static extern IntPtr CreateCompatibleDC(IntPtr dc);
    [DllImport("gdi32.dll")] private static extern IntPtr CreateCompatibleBitmap(IntPtr dc, int width, int height);
    [DllImport("gdi32.dll")] private static extern IntPtr SelectObject(IntPtr dc, IntPtr obj);
    [DllImport("gdi32.dll")] private static extern bool DeleteObject(IntPtr obj);
    [DllImport("gdi32.dll")] private static extern bool DeleteDC(IntPtr dc);
    [DllImport("gdi32.dll")] private static extern int SetStretchBltMode(IntPtr dc, int mode);
    [DllImport("gdi32.dll")] private static extern bool SetBrushOrgEx(IntPtr dc, int x, int y, IntPtr point);
    [DllImport("gdi32.dll")] private static extern bool StretchBlt(IntPtr target, int x, int y, int width, int height, IntPtr source, int sx, int sy, int sw, int sh, uint operation);
    [DllImport("gdi32.dll")] private static extern int GetDIBits(IntPtr dc, IntPtr bitmap, uint start, uint count, byte[] bits, ref BitmapInfo info, uint usage);
}