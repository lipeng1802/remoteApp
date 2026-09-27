using System.Buffers.Binary;
using System.Security.Cryptography;
using System.Text;

namespace RemoteProtocol;

public enum PeerRole : byte
{
    Controller = 1,
    Agent = 2,
}

[Flags]
public enum Capabilities : uint
{
    None = 0,
    Jpeg = 1 << 0,
    H264 = 1 << 1,
    Input = 1 << 2,
    Reconnect = 1 << 3,
}

public sealed record HelloPayload(
    PeerRole Role,
    ushort MinimumVersion,
    ushort MaximumVersion,
    Capabilities Capabilities,
    byte[] Nonce)
{
    public const int EncodedLength = 41;

    public byte[] Encode()
    {
        if (Nonce.Length != 32 || MinimumVersion > MaximumVersion)
        {
            throw Failure("HELLO payload values are invalid.");
        }

        var output = new byte[EncodedLength];
        output[0] = (byte)Role;
        BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(1), MinimumVersion);
        BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(3), MaximumVersion);
        BinaryPrimitives.WriteUInt32BigEndian(output.AsSpan(5), (uint)Capabilities);
        Nonce.CopyTo(output, 9);
        return output;
    }

    public static HelloPayload Decode(ReadOnlySpan<byte> data)
    {
        if (data.Length != EncodedLength || !Enum.IsDefined(typeof(PeerRole), data[0]))
        {
            throw Failure("HELLO payload has an invalid length or role.");
        }

        var minimumVersion = BinaryPrimitives.ReadUInt16BigEndian(data[1..]);
        var maximumVersion = BinaryPrimitives.ReadUInt16BigEndian(data[3..]);
        if (minimumVersion > maximumVersion)
        {
            throw Failure("HELLO version range is invalid.");
        }

        return new HelloPayload(
            (PeerRole)data[0],
            minimumVersion,
            maximumVersion,
            (Capabilities)BinaryPrimitives.ReadUInt32BigEndian(data[5..]),
            data[9..41].ToArray());
    }

    private static ProtocolException Failure(string message) =>
        new(ProtocolError.InvalidPayload, message);
}

public sealed record AuthChallengePayload(byte[] Challenge, byte[] AgentIdentifier)
{
    public const int EncodedLength = 48;

    public byte[] Encode()
    {
        if (Challenge.Length != 32 || AgentIdentifier.Length != 16)
        {
            throw Failure("AUTH_CHALLENGE payload values are invalid.");
        }
        return [.. Challenge, .. AgentIdentifier];
    }

    public static AuthChallengePayload Decode(ReadOnlySpan<byte> data)
    {
        if (data.Length != EncodedLength)
        {
            throw Failure("AUTH_CHALLENGE payload length is invalid.");
        }
        return new AuthChallengePayload(data[..32].ToArray(), data[32..48].ToArray());
    }

    private static ProtocolException Failure(string message) =>
        new(ProtocolError.InvalidPayload, message);
}

public enum AuthResultStatus : byte
{
    Success = 0,
    Rejected = 1,
    TemporarilyLocked = 2,
}

public sealed record AuthResultPayload(AuthResultStatus Status, uint RetryDelayMilliseconds)
{
    public const int EncodedLength = 5;

    public byte[] Encode()
    {
        if (!Enum.IsDefined(Status) || (Status == AuthResultStatus.Success && RetryDelayMilliseconds != 0))
        {
            throw Failure("AUTH_RESULT payload values are invalid.");
        }
        var output = new byte[EncodedLength];
        output[0] = (byte)Status;
        BinaryPrimitives.WriteUInt32BigEndian(output.AsSpan(1), RetryDelayMilliseconds);
        return output;
    }

    public static AuthResultPayload Decode(ReadOnlySpan<byte> data)
    {
        if (data.Length != EncodedLength || !Enum.IsDefined(typeof(AuthResultStatus), data[0]))
        {
            throw Failure("AUTH_RESULT payload has an invalid length or status.");
        }
        var status = (AuthResultStatus)data[0];
        var delay = BinaryPrimitives.ReadUInt32BigEndian(data[1..]);
        if (status == AuthResultStatus.Success && delay != 0)
        {
            throw Failure("A successful AUTH_RESULT cannot specify a retry delay.");
        }
        return new AuthResultPayload(status, delay);
    }

    private static ProtocolException Failure(string message) =>
        new(ProtocolError.InvalidPayload, message);
}

public static class Authentication
{
    private static readonly byte[] Context = Encoding.ASCII.GetBytes("PRD-AUTH-V1");

    public static byte[] CreateResponse(
        ReadOnlySpan<byte> deviceKey,
        ReadOnlySpan<byte> controllerNonce,
        ReadOnlySpan<byte> agentNonce,
        ReadOnlySpan<byte> challenge,
        ReadOnlySpan<byte> agentIdentifier)
    {
        if (deviceKey.Length != 32 ||
            controllerNonce.Length != 32 ||
            agentNonce.Length != 32 ||
            challenge.Length != 32 ||
            agentIdentifier.Length != 16)
        {
            throw new ProtocolException(ProtocolError.InvalidPayload, "Authentication input length is invalid.");
        }

        var message = new byte[Context.Length + 32 + 32 + 32 + 16];
        var offset = 0;
        Context.CopyTo(message, offset);
        offset += Context.Length;
        controllerNonce.CopyTo(message.AsSpan(offset));
        offset += controllerNonce.Length;
        agentNonce.CopyTo(message.AsSpan(offset));
        offset += agentNonce.Length;
        challenge.CopyTo(message.AsSpan(offset));
        offset += challenge.Length;
        agentIdentifier.CopyTo(message.AsSpan(offset));

        return HMACSHA256.HashData(deviceKey, message);
    }

    public static bool ConstantTimeEquals(ReadOnlySpan<byte> left, ReadOnlySpan<byte> right) =>
        left.Length == right.Length && CryptographicOperations.FixedTimeEquals(left, right);
}

public enum SessionPhase
{
    AwaitingHello,
    Authenticating,
    Authenticated,
    Closing,
}

public sealed class SessionGate(PeerRole localRole)
{
    private bool receivedChallenge;
    private byte[]? receivedResponse;

    public PeerRole LocalRole { get; } = localRole;
    public SessionPhase Phase { get; private set; } = SessionPhase.AwaitingHello;

    public void Receive(Frame frame)
    {
        if (frame.Type.RequiresAuthentication() && Phase != SessionPhase.Authenticated)
        {
            throw Failure(ProtocolError.AuthRequired, "Message requires an authenticated session.");
        }
        if (frame.Type is MessageType.Disconnect or MessageType.Error)
        {
            Phase = SessionPhase.Closing;
            return;
        }

        switch (Phase)
        {
            case SessionPhase.AwaitingHello:
                if (frame.Type != MessageType.Hello)
                {
                    throw Failure(ProtocolError.InvalidState, "Expected HELLO.");
                }
                var hello = HelloPayload.Decode(frame.Payload);
                if (hello.Role == LocalRole ||
                    hello.MinimumVersion > ProtocolConstants.Version ||
                    hello.MaximumVersion < ProtocolConstants.Version)
                {
                    throw Failure(ProtocolError.InvalidPayload, "HELLO peer role or version is invalid.");
                }
                Phase = SessionPhase.Authenticating;
                break;

            case SessionPhase.Authenticating:
                ReceiveDuringAuthentication(frame);
                break;

            case SessionPhase.Authenticated:
                if (frame.Type.IsHandshake())
                {
                    throw Failure(ProtocolError.InvalidState, "Handshake message received after authentication.");
                }
                break;

            case SessionPhase.Closing:
                throw Failure(ProtocolError.InvalidState, "Session is closing.");
        }
    }

    public void CompleteAgentAuthentication(ReadOnlySpan<byte> expectedResponse)
    {
        if (LocalRole != PeerRole.Agent || Phase != SessionPhase.Authenticating || receivedResponse is null)
        {
            throw Failure(ProtocolError.InvalidState, "No verified controller response is pending.");
        }
        var success = Authentication.ConstantTimeEquals(receivedResponse, expectedResponse);
        Phase = success ? SessionPhase.Authenticated : SessionPhase.Closing;
    }

    private void ReceiveDuringAuthentication(Frame frame)
    {
        if (LocalRole == PeerRole.Controller)
        {
            if (frame.Type == MessageType.AuthChallenge && !receivedChallenge)
            {
                _ = AuthChallengePayload.Decode(frame.Payload);
                receivedChallenge = true;
                return;
            }
            if (frame.Type == MessageType.AuthResult && receivedChallenge)
            {
                var result = AuthResultPayload.Decode(frame.Payload);
                Phase = result.Status == AuthResultStatus.Success
                    ? SessionPhase.Authenticated
                    : SessionPhase.Closing;
                return;
            }
        }
        else if (frame.Type == MessageType.AuthResponse && receivedResponse is null && frame.Payload.Length == 32)
        {
            receivedResponse = frame.Payload.ToArray();
            return;
        }

        throw Failure(ProtocolError.InvalidState, "Message is invalid during authentication.");
    }

    private static ProtocolException Failure(ProtocolError error, string message) => new(error, message);
}

public static class MessageTypeExtensions
{
    public static bool RequiresAuthentication(this MessageType type) => type is
        MessageType.ScreenInfo or MessageType.VideoFrameJpeg or MessageType.VideoFrameH264 or
        MessageType.MouseMove or MessageType.MouseButton or MessageType.MouseWheel or
        MessageType.KeyEvent or MessageType.Ping or MessageType.Pong;

    public static bool IsHandshake(this MessageType type) => type is
        MessageType.Hello or MessageType.AuthChallenge or MessageType.AuthResponse or MessageType.AuthResult;
}
