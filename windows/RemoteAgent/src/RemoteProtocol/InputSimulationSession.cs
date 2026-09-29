using System.Diagnostics;

namespace RemoteProtocol;

// Used only by the explicit loopback simulation entry point. Never wired to the desktop UI.
public sealed record InputSimulationOptions(
    Func<IInputSink> CreateSink,
    bool LocalControlAllowed = false,
    int BurstLimit = 120,
    int EventsPerSecond = 240,
    TimeSpan? ReadTimeout = null)
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
                await wire.WriteAsync(MessageType.Pong, frame.Payload, deadline.Token).ConfigureAwait(false);
            }
            else dispatcher.Apply(frame);
        }
    }
}
