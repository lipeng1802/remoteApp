using System.Buffers.Binary;

namespace RemoteProtocol;

// This bounded transport is for the authentication probe, not the future video session.
public sealed class ProbeFrameStream(Stream stream, bool allowOutgoingJpeg = false,
    bool allowOutgoingClipboardText = false)
{
    public const int MaximumPayloadLength = 64;
    private uint incomingSequence = 1;
    private uint outgoingSequence = 1;

    public async Task<Frame> ReadAsync(CancellationToken cancellationToken)
    {
        var header = new byte[ProtocolConstants.HeaderLength];
        await stream.ReadExactlyAsync(header, cancellationToken).ConfigureAwait(false);
        var decoder = new FrameDecoder();
        var frames = decoder.Append(header); // Validate header before allocating a body.
        var length = BinaryPrimitives.ReadUInt32BigEndian(header.AsSpan(12));
        if (length > MaximumPayloadLength)
            throw new ProtocolException(ProtocolError.MessageTooLarge, "Probe payload exceeds 64 bytes.");
        if (length != 0)
        {
            var payload = new byte[(int)length];
            await stream.ReadExactlyAsync(payload, cancellationToken).ConfigureAwait(false);
            frames = decoder.Append(payload);
        }
        decoder.Finish();
        var frame = frames.Single();
        if (frame.Sequence != incomingSequence)
            throw new ProtocolException(ProtocolError.InvalidState, "Unexpected frame sequence.");
        incomingSequence = incomingSequence == uint.MaxValue ? 1 : incomingSequence + 1;
        return frame;
    }

    public async Task WriteAsync(MessageType type, byte[] payload, CancellationToken cancellationToken)
    {
        var limit = allowOutgoingJpeg && type == MessageType.VideoFrameJpeg
            ? ProtocolConstants.MaximumPayloadLength
            : allowOutgoingClipboardText && type == MessageType.ClipboardText
                ? ClipboardTextPayload.MaximumTextBytes + 1
                : MaximumPayloadLength;
        if (payload.Length > limit)
            throw new ProtocolException(ProtocolError.MessageTooLarge, "Probe payload exceeds its allowed size.");
        var frame = new Frame(type, 0, outgoingSequence, type == MessageType.VideoFrameJpeg ? (ulong)(System.Diagnostics.Stopwatch.GetTimestamp() * (1_000_000.0 / System.Diagnostics.Stopwatch.Frequency)) : 0, payload);
        outgoingSequence = outgoingSequence == uint.MaxValue ? 1 : outgoingSequence + 1;
        await stream.WriteAsync(FrameCodec.Encode(frame), cancellationToken).ConfigureAwait(false);
        await stream.FlushAsync(cancellationToken).ConfigureAwait(false);
    }
}
