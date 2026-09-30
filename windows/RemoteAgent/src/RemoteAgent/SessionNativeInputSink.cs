using System.Diagnostics;
using RemoteProtocol;

namespace RemoteAgent;

internal readonly record struct InputStatusSnapshot(long Events, long Releases, int Held);

// Session-scoped status wrapper around the native sink. Protocol authentication,
// capability negotiation and the per-session local confirmation happen before
// this object is constructed. It records counts only—not input content.
internal sealed class SessionNativeInputSink : IInputSink
{
    private readonly WindowsInputSink native = new();
    private readonly Action<InputStatusSnapshot> report;
    private readonly HashSet<(ushort ScanCode, bool Extended)> keys = [];
    private readonly HashSet<MouseButton> buttons = [];
    private long events;
    private long releases;
    private long lastReport;

    internal SessionNativeInputSink(Action<InputStatusSnapshot> report)
    {
        this.report = report ?? throw new ArgumentNullException(nameof(report));
        Publish(force: true);
    }

    public void Move(MouseMovePayload point)
    {
        native.Move(point);
        events++;
        Publish();
    }

    public void Wheel(MouseWheelPayload delta)
    {
        native.Wheel(delta);
        events++;
        Publish();
    }

    public void Button(MouseButtonPayload button)
    {
        native.Button(button);
        events++;
        var changed = button.Action == ButtonAction.Down
            ? buttons.Add(button.Button)
            : buttons.Remove(button.Button);
        if (button.Action == ButtonAction.Up && changed) releases++;
        Publish(force: changed);
    }

    public void Key(KeyEventPayload key)
    {
        native.Key(key);
        events++;
        var identity = (key.ScanCode, key.Extended);
        var changed = key.Action == KeyAction.Down
            ? keys.Add(identity)
            : keys.Remove(identity);
        if (key.Action == KeyAction.Up && changed) releases++;
        Publish(force: changed);
    }

    public void ReleaseAllKeys()
    {
        native.ReleaseAllKeys();
        releases += keys.Count;
        keys.Clear();
        Publish(force: true);
    }

    public void ReleaseAllButtons()
    {
        native.ReleaseAllButtons();
        releases += buttons.Count;
        buttons.Clear();
        Publish(force: true);
    }

    private void Publish(bool force = false)
    {
        var now = Stopwatch.GetTimestamp();
        if (!force && lastReport != 0 && Stopwatch.GetElapsedTime(lastReport, now) < TimeSpan.FromSeconds(1)) return;
        lastReport = now;
        report(new InputStatusSnapshot(events, releases, keys.Count + buttons.Count));
    }
}
