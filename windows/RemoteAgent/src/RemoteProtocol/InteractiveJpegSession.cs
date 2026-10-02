using System.Diagnostics;
using System.Threading.Channels;

namespace RemoteProtocol;

internal static class InteractiveJpegSession
{
    internal static async Task RunAsync(
        ProbeFrameStream wire,
        SessionGate gate,
        Func<IJpegFrameSource> createJpegSource,
        InputSimulationOptions input,
        Action<string>? reportStatus,
        Action<JpegTransferMetrics>? reportMetrics,
        TimeSpan frameTimeout,
        CancellationToken cancellationToken)
    {
        if (!input.LocalControlAllowed)
            throw new ProtocolException(ProtocolError.InvalidState, "Local input consent is required.");
        var replies = Channel.CreateBounded<(MessageType Type, byte[] Payload)>(new BoundedChannelOptions(16)
        {
            SingleReader = true,
            SingleWriter = true,
            FullMode = BoundedChannelFullMode.Wait,
        });
        using var lifetime = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        var reader = InputSimulationSession.RunReadLoopAsync(wire, gate, input,
            (type, payload, _) =>
            {
                if (!replies.Writer.TryWrite((type, payload.ToArray())))
                    throw new ProtocolException(ProtocolError.InvalidState, "Reply queue is full.");
                return Task.CompletedTask;
            }, clipboardNegotiated: input.ReadClipboardText is not null, lifetime.Token);

        Exception? primaryFailure = null;
        try
        {
            reportStatus?.Invoke("正在共享主屏 · 远程控制已由本机允许");
            // Capture is still constructed only after TLS/HMAC authentication and local consent.
            using var source = createJpegSource();
            ScreenInfoPayload? previous = null;
            long lastReport = 0;
            long frameNumber = 0;
            long nextCapture = Stopwatch.GetTimestamp();
            while (true)
            {
                cancellationToken.ThrowIfCancellationRequested();
                if (reader.IsCompleted)
                {
                    await reader.ConfigureAwait(false);
                    return;
                }
                if (replies.Reader.TryRead(out var reply))
                {
                    using var replyDeadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
                    replyDeadline.CancelAfter(frameTimeout);
                    try
                    {
                        await wire.WriteAsync(reply.Type, reply.Payload, replyDeadline.Token).ConfigureAwait(false);
                    }
                    catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested && replyDeadline.IsCancellationRequested)
                    {
                        throw new JpegTransferTimeoutException(JpegTransferStage.Sending);
                    }
                    continue;
                }

                var now = Stopwatch.GetTimestamp();
                if (now < nextCapture)
                {
                    var remaining = TimeSpan.FromSeconds((nextCapture - now) / (double)Stopwatch.Frequency);
                    await Task.Delay(remaining < TimeSpan.FromMilliseconds(10)
                        ? remaining : TimeSpan.FromMilliseconds(10), cancellationToken).ConfigureAwait(false);
                    continue;
                }

                var started = now;
                var captured = source.Capture(cancellationToken);
                frameNumber++;
                if (captured.Jpeg.Length < 4 || captured.Jpeg[0] != 0xff || captured.Jpeg[1] != 0xd8 ||
                    captured.Jpeg[^2] != 0xff || captured.Jpeg[^1] != 0xd9)
                    throw new ProtocolException(ProtocolError.InvalidPayload, "Invalid captured JPEG.");
                using var deadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
                deadline.CancelAfter(frameTimeout);
                var captureMs = Stopwatch.GetElapsedTime(started).TotalMilliseconds;
                var sendStarted = Stopwatch.GetTimestamp();
                try
                {
                    if (previous != captured.Screen)
                    {
                        await wire.WriteAsync(MessageType.ScreenInfo, captured.Screen.Encode(), deadline.Token)
                            .ConfigureAwait(false);
                        previous = captured.Screen;
                    }
                    await wire.WriteAsync(MessageType.VideoFrameJpeg, captured.Jpeg, deadline.Token)
                        .ConfigureAwait(false);
                    var sendMs = Stopwatch.GetElapsedTime(sendStarted).TotalMilliseconds;
                    if (lastReport == 0 || Stopwatch.GetElapsedTime(lastReport) >= TimeSpan.FromSeconds(1))
                    {
                        reportMetrics?.Invoke(new JpegTransferMetrics(captured.Jpeg.Length, captureMs, sendMs,
                            Stopwatch.GetElapsedTime(started).TotalMilliseconds, frameNumber));
                        lastReport = Stopwatch.GetTimestamp();
                    }
                }
                catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested && deadline.IsCancellationRequested)
                {
                    throw new JpegTransferTimeoutException(JpegTransferStage.Sending);
                }
                nextCapture = started + Stopwatch.Frequency / 10;
            }
        }
        catch (Exception exception)
        {
            primaryFailure = exception;
            throw;
        }
        finally
        {
            lifetime.Cancel();
            replies.Writer.TryComplete();
            try { await reader.ConfigureAwait(false); }
            catch when (primaryFailure is not null || cancellationToken.IsCancellationRequested) { }
        }
    }
}
