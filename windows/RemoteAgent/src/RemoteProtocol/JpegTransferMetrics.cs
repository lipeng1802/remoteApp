namespace RemoteProtocol;

public enum JpegTransferStage { Sending, AwaitingAcknowledgement }
public sealed class JpegTransferTimeoutException(JpegTransferStage stage)
    : TimeoutException("JPEG frame exchange deadline exceeded.")
{
    public JpegTransferStage Stage { get; } = stage;
}
// Durations and size only. No addresses, image content, or authentication data.
public sealed record JpegTransferMetrics(int JpegBytes, double CaptureMilliseconds,
    double SendMilliseconds, double AcknowledgementMilliseconds);