using System.Diagnostics;
using RemoteProtocol;

namespace RemoteAgent;

internal readonly record struct InputAuditSnapshot(long Events, long Releases, int Held);

// Product-GUI validation sink. It intentionally never calls WindowsInputSink or
// any native API, and records counts only—not coordinates, scan codes or content.
internal sealed class SessionInputAuditSink : IInputSink
{
    private readonly Action<InputAuditSnapshot> report;
    private readonly HashSet<(ushort ScanCode, bool Extended)> keys = [];
    private readonly HashSet<MouseButton> buttons = [];
    private long events;
    private long releases;
    private long lastReport;

    internal SessionInputAuditSink(Action<InputAuditSnapshot> report)
    {
        this.report = report ?? throw new ArgumentNullException(nameof(report));
        Publish(force: true);
    }

    public void Move(MouseMovePayload point) { events++; Publish(); }
    public void Wheel(MouseWheelPayload delta) { events++; Publish(); }

    public void Button(MouseButtonPayload button)
    {
        events++;
        if (button.Action == ButtonAction.Down) buttons.Add(button.Button);
        else { buttons.Remove(button.Button); releases++; }
        Publish();
    }

    public void Key(KeyEventPayload key)
    {
        events++;
        var identity = (key.ScanCode, key.Extended);
        if (key.Action == KeyAction.Down) keys.Add(identity);
        else { keys.Remove(identity); releases++; }
        Publish();
    }

    public void ReleaseAllKeys()
    {
        releases += keys.Count;
        keys.Clear();
        Publish(force: true);
    }

    public void ReleaseAllButtons()
    {
        releases += buttons.Count;
        buttons.Clear();
        Publish(force: true);
    }

    private void Publish(bool force = false)
    {
        var now = Stopwatch.GetTimestamp();
        if (!force && lastReport != 0 && Stopwatch.GetElapsedTime(lastReport, now) < TimeSpan.FromSeconds(1)) return;
        lastReport = now;
        report(new InputAuditSnapshot(events, releases, keys.Count + buttons.Count));
    }
}
