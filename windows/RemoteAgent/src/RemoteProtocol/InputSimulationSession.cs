using System.Diagnostics;

namespace RemoteProtocol;

// Shared authenticated input policy. Production callers must supply explicit
// per-session local consent before constructing an input session.
public sealed record InputSimulationOptions(
    Func<IInputSink> CreateSink,
    bool LocalControlAllowed = false,
    int BurstLimit = 120,
    int EventsPerSecond = 240,
    TimeSpan? ReadTimeout = null,
    Func<CancellationToken, Task<ClipboardTextPayload>>? ReadClipboardText = null,
    Func<string, CancellationToken, Task<ClipboardTextStatus>>? WriteClipboardText = null)
{
    internal void Validate()
    {
        ArgumentNullException.ThrowIfNull(CreateSink);
        if (BurstLimit is < 1 or > 4096 || EventsPerSecond is < 1 or > 4096 ||
            (ReadTimeout is { } timeout && (timeout <= TimeSpan.Zero || timeout > TimeSpan.FromMinutes(5))))
            throw new ArgumentOutOfRangeException(nameof(InputSimulationOptions));
    }
}

internal static class InputSimulationSession
{
    internal static async Task RunAsync(ProbeFrameStream wire, SessionGate gate,
        InputSimulationOptions options, CancellationToken cancellationToken)
    {
        await RunReadLoopAsync(wire, gate, options,
            (type, payload, token) => wire.WriteAsync(type, payload, token),
            clipboardNegotiated: options.ReadClipboardText is not null || options.WriteClipboardText is not null,
            cancellationToken).ConfigureAwait(false);
    }

    // The duplex JPEG profile supplies a bounded callback so only its single
    // writer touches ProbeFrameStream's outgoing sequence and TLS writes.
    internal static async Task RunReadLoopAsync(ProbeFrameStream wire, SessionGate gate,
        InputSimulationOptions options, Func<MessageType, byte[], CancellationToken, Task> sendResponse,
        bool clipboardNegotiated,
        CancellationToken cancellationToken)
    {
        if (!options.LocalControlAllowed)
            throw new ProtocolException(ProtocolError.InvalidState, "Local input consent is required.");
        using var dispatcher = new InputDispatcher(gate, options.CreateSink(), inputNegotiated: true);
        dispatcher.GrantLocalControl();
        var tokens = (double)options.BurstLimit;
        var last = Stopwatch.GetTimestamp();
        while (true)
        {
            using var deadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            deadline.CancelAfter(options.ReadTimeout ?? TimeSpan.FromSeconds(15));
            var frame = await wire.ReadAsync(deadline.Token).ConfigureAwait(false);
            if (frame.Type is MessageType.Disconnect or MessageType.Error)
            {
                dispatcher.Apply(frame);
                return;
            }
            var now = Stopwatch.GetTimestamp();
            tokens = Math.Min(options.BurstLimit, tokens + Stopwatch.GetElapsedTime(last, now).TotalSeconds * options.EventsPerSecond);
            last = now;
            if (tokens < 1)
                throw new ProtocolException(ProtocolError.InvalidState, "Input simulation rate limit exceeded.");
            tokens--;
            if (frame.Type == MessageType.Ping)
            {
                gate.Receive(frame);
                if (frame.Payload.Length != 8)
                    throw new ProtocolException(ProtocolError.InvalidPayload, "Invalid heartbeat.");
                await sendResponse(MessageType.Pong, frame.Payload, deadline.Token).ConfigureAwait(false);
            }
            else if (frame.Type == MessageType.ClipboardRequest)
            {
                gate.Receive(frame);
                if (!clipboardNegotiated || options.ReadClipboardText is null || frame.Payload.Length != 0)
                    throw new ProtocolException(ProtocolError.InvalidState, "Clipboard text is not negotiated.");
                var clipboard = await options.ReadClipboardText(deadline.Token).ConfigureAwait(false);
                await sendResponse(MessageType.ClipboardText, clipboard.Encode(), deadline.Token).ConfigureAwait(false);
            }
            else if (frame.Type == MessageType.ClipboardSetText)
            {
                gate.Receive(frame);
                if (!clipboardNegotiated || options.WriteClipboardText is null)
                    throw new ProtocolException(ProtocolError.InvalidState, "Clipboard text is not negotiated.");
                var clipboard = ClipboardTextPayload.Decode(frame.Payload);
                if (clipboard.Status != ClipboardTextStatus.Success)
                    throw new ProtocolException(ProtocolError.InvalidPayload, "Clipboard set requires text.");
                var status = await options.WriteClipboardText(clipboard.Text, deadline.Token).ConfigureAwait(false);
                if (status == ClipboardTextStatus.Success) dispatcher.PasteClipboardText();
                var result = new ClipboardTextPayload(status, string.Empty).Encode();
                await sendResponse(MessageType.ClipboardSetResult, result, deadline.Token).ConfigureAwait(false);
            }
            else dispatcher.Apply(frame);
        }
    }
}
