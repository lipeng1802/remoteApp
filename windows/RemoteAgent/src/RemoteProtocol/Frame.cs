using System.Buffers.Binary;

namespace RemoteProtocol;

public static class ProtocolConstants
{
    public static ReadOnlySpan<byte> Magic => "PRD1"u8;
    public const ushort Version = 1;
    public const int HeaderLength = 28;
    public const int MaximumPayloadLength = 8 * 1024 * 1024;
    public const int MaximumControlPayloadLength = 64 * 1024;
}

public enum MessageType : ushort
{
    Hello = 0x0001,
    AuthChallenge = 0x0002,
    AuthResponse = 0x0003,
    AuthResult = 0x0004,
    ScreenInfo = 0x0005,
    VideoFrameJpeg = 0x0006,
    VideoFrameH264 = 0x0007,
    MouseMove = 0x0008,
    MouseButton = 0x0009,
    MouseWheel = 0x000a,
    KeyEvent = 0x000b,
    Ping = 0x000c,
    Pong = 0x000d,
    Disconnect = 0x000e,
    Error = 0x000f,
}

public sealed record Frame(
    MessageType Type,
    ushort Flags,
    uint Sequence,
    ulong TimestampMicros,
    byte[] Payload);

public enum ProtocolError
{
    InvalidMagic,
    UnsupportedVersion,
    InvalidHeaderLength,
    UnsupportedMessageType,
    NonzeroFlags,
    MessageTooLarge,
    IncompleteFrame,
    InvalidPayload,
    AuthRequired,
    InvalidState,
}

public sealed class ProtocolException(ProtocolError error, string message) : Exception(message)
{
    public ProtocolError Error { get; } = error;
}

public static class FrameCodec
{
    public static byte[] Encode(Frame frame)
    {
        var limit = IsVideo(frame.Type)
            ? ProtocolConstants.MaximumPayloadLength
            : ProtocolConstants.MaximumControlPayloadLength;
        if (frame.Payload.Length > limit)
        {
            throw new ProtocolException(ProtocolError.MessageTooLarge, "Payload exceeds its message limit.");
        }

        if (frame.Flags != 0)
        {
            throw new ProtocolException(ProtocolError.NonzeroFlags, "Version 1 flags must be zero.");
        }

        var output = new byte[ProtocolConstants.HeaderLength + frame.Payload.Length];
        ProtocolConstants.Magic.CopyTo(output);
        BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(4), ProtocolConstants.Version);
        BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(6), ProtocolConstants.HeaderLength);
        BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(8), (ushort)frame.Type);
        BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(10), frame.Flags);
        BinaryPrimitives.WriteUInt32BigEndian(output.AsSpan(12), (uint)frame.Payload.Length);
        BinaryPrimitives.WriteUInt32BigEndian(output.AsSpan(16), frame.Sequence);
        BinaryPrimitives.WriteUInt64BigEndian(output.AsSpan(20), frame.TimestampMicros);
        frame.Payload.CopyTo(output, ProtocolConstants.HeaderLength);
        return output;
    }

    internal static bool IsVideo(MessageType type) =>
        type is MessageType.VideoFrameJpeg or MessageType.VideoFrameH264;
}

public sealed class FrameDecoder
{
    private readonly List<byte> buffer = [];

    public IReadOnlyList<Frame> Append(ReadOnlySpan<byte> data)
    {
        buffer.AddRange(data.ToArray());
        var frames = new List<Frame>();

        while (buffer.Count >= ProtocolConstants.HeaderLength)
        {
            var header = buffer.GetRange(0, ProtocolConstants.HeaderLength).ToArray();
            if (!header.AsSpan(0, 4).SequenceEqual(ProtocolConstants.Magic))
            {
                throw Failure(ProtocolError.InvalidMagic, "Invalid frame magic.");
            }

            var version = BinaryPrimitives.ReadUInt16BigEndian(header.AsSpan(4));
            if (version != ProtocolConstants.Version)
            {
                throw Failure(ProtocolError.UnsupportedVersion, $"Unsupported version {version}.");
            }

            var headerLength = BinaryPrimitives.ReadUInt16BigEndian(header.AsSpan(6));
            if (headerLength != ProtocolConstants.HeaderLength)
            {
                throw Failure(ProtocolError.InvalidHeaderLength, $"Invalid header length {headerLength}.");
            }

            var rawType = BinaryPrimitives.ReadUInt16BigEndian(header.AsSpan(8));
            if (!Enum.IsDefined(typeof(MessageType), rawType))
            {
                throw Failure(ProtocolError.UnsupportedMessageType, $"Unsupported message type {rawType}.");
            }
            var type = (MessageType)rawType;

            var flags = BinaryPrimitives.ReadUInt16BigEndian(header.AsSpan(10));
            if (flags != 0)
            {
                throw Failure(ProtocolError.NonzeroFlags, "Version 1 flags must be zero.");
            }

            var declaredLength = BinaryPrimitives.ReadUInt32BigEndian(header.AsSpan(12));
            var limit = FrameCodec.IsVideo(type)
                ? ProtocolConstants.MaximumPayloadLength
                : ProtocolConstants.MaximumControlPayloadLength;
            if (declaredLength > limit)
            {
                throw Failure(ProtocolError.MessageTooLarge, $"Payload length {declaredLength} exceeds {limit}.");
            }

            var frameLength = checked(ProtocolConstants.HeaderLength + (int)declaredLength);
            if (buffer.Count < frameLength)
            {
                break;
            }

            var payload = declaredLength == 0
                ? []
                : buffer.GetRange(ProtocolConstants.HeaderLength, (int)declaredLength).ToArray();
            frames.Add(new Frame(
                type,
                flags,
                BinaryPrimitives.ReadUInt32BigEndian(header.AsSpan(16)),
                BinaryPrimitives.ReadUInt64BigEndian(header.AsSpan(20)),
                payload));
            buffer.RemoveRange(0, frameLength);
        }

        return frames;
    }

    public void Finish()
    {
        if (buffer.Count != 0)
        {
            throw Failure(ProtocolError.IncompleteFrame, "The stream ended inside a frame.");
        }
    }

    private static ProtocolException Failure(ProtocolError error, string message) => new(error, message);
}
