namespace RemoteProtocol;

// Native injection is isolated in the WindowsInput project. Production GUI/TLS
// remains read-only until the authenticated duplex session explicitly wires it.
public interface IMouseInputSink
{
    void Move(MouseMovePayload point);
    void Button(MouseButtonPayload button);
    void Wheel(MouseWheelPayload delta);
    void ReleaseAllButtons();
}

public interface IInputSink : IMouseInputSink
{
    void Key(KeyEventPayload key);
    void ReleaseAllKeys();
}

// Single-session, serial caller only. Lifecycle owner must Dispose on every connection exit.
public sealed class InputDispatcher(SessionGate gate, IInputSink sink, bool inputNegotiated = false) : IDisposable
{
    private readonly HashSet<MouseButton> pressedButtons = [];
    private readonly HashSet<(ushort ScanCode, bool Extended)> pressedKeys = [];
    private bool locallyAllowed;
    private bool disposed;

    public void GrantLocalControl()
    {
        if (disposed || gate.LocalRole != PeerRole.Agent || gate.Phase != SessionPhase.Authenticated ||
            !inputNegotiated || pressedKeys.Count != 0 || pressedButtons.Count != 0)
            throw new ProtocolException(ProtocolError.InvalidState, "Control is not available.");
        locallyAllowed = true;
    }

    public void Apply(Frame frame)
    {
        try
        {
            if (disposed) throw new ProtocolException(ProtocolError.InvalidState, "Input session closed.");
            gate.Receive(frame);
            if (frame.Type is MessageType.Disconnect or MessageType.Error) { RevokeControl(); return; }
            if (gate.LocalRole != PeerRole.Agent || !inputNegotiated || !locallyAllowed)
                throw new ProtocolException(ProtocolError.InvalidState, "Control not enabled locally.");
            switch (frame.Type)
            {
                case MessageType.MouseMove:
                    sink.Move(MouseMovePayload.Decode(frame.Payload));
                    break;
                case MessageType.MouseButton:
                    var button = MouseButtonPayload.Decode(frame.Payload);
                    if (button.Action == ButtonAction.Down)
                    {
                        if (pressedButtons.Add(button.Button)) sink.Button(button);
                    }
                    else if (pressedButtons.Contains(button.Button))
                    {
                        sink.Button(button);
                        pressedButtons.Remove(button.Button);
                    }
                    break;
                case MessageType.MouseWheel:
                    sink.Wheel(MouseWheelPayload.Decode(frame.Payload));
                    break;
                case MessageType.KeyEvent:
                    var key = KeyEventPayload.Decode(frame.Payload);
                    var identity = (key.ScanCode, key.Extended);
                    if (key.Action == KeyAction.Down)
                    {
                        // Repeated downs preserve typematic behavior, but track one held key.
                        pressedKeys.Add(identity);
                        sink.Key(key);
                    }
                    else if (pressedKeys.Contains(identity))
                    {
                        sink.Key(key);
                        pressedKeys.Remove(identity);
                    }
                    break;
                default:
                    throw new ProtocolException(ProtocolError.InvalidState, "Unsupported input.");
            }
        }
        catch (Exception inputError)
        {
            try { RevokeControl(); }
            catch (Exception cleanupError) { throw new AggregateException(inputError, cleanupError); }
            throw;
        }
    }

    public void RevokeControl()
    {
        locallyAllowed = false;
        List<Exception>? errors = null;
        // Attempt both groups even if one fails. Retain failed groups so Dispose can retry.
        if (pressedKeys.Count != 0)
        {
            try { sink.ReleaseAllKeys(); pressedKeys.Clear(); }
            catch (Exception ex) { (errors ??= []).Add(ex); }
        }
        if (pressedButtons.Count != 0)
        {
            try { sink.ReleaseAllButtons(); pressedButtons.Clear(); }
            catch (Exception ex) { (errors ??= []).Add(ex); }
        }
        if (errors is not null) throw new AggregateException("Input cleanup failed.", errors);
    }

    public void Dispose()
    {
        disposed = true;
        RevokeControl();
    }
}
