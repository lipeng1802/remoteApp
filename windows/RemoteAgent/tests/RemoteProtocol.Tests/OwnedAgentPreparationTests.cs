using System.Text.Json;
using RemoteProtocol;

internal static class OwnedAgentPreparationTests
{
    private static readonly string Self = new('a', 64), Controller = new('b', 64), Request = new('c', 32);
    private static string Frame(params (string Key, string Value)[] values) =>
        JsonSerializer.Serialize(new SortedDictionary<string, string>(values.ToDictionary(v => v.Key, v => v.Value), StringComparer.Ordinal));
    private static string Prepare(string scope = "desktop-view", string role = "target") =>
        Frame(("Op", "prepare"), ("Permission", scope), ("Role", role), ("Self", Self));
    private static string Consent(string? controller = null) =>
        Frame(("Op", "consent"), ("RequestID", Request), ("Controller", controller ?? Controller));
    private static void Reject(Action f)
    {
        try { f(); }
        catch (Exception e) when (e is InvalidDataException or InvalidOperationException or JsonException) { return; }
        throw new Exception("Unsafe owned preparation accepted.");
    }
    public static void StrictTargetOnly()
    {
        foreach (var json in new[] { Prepare(role: "controller"), Prepare("fixed-payload"), " " + Prepare(),
            Prepare()[..^1] + ",\"Op\":\"prepare\"}", Prepare()[..^1] + ",\"Allow\":\"true\"}" })
            Reject(() => _ = new OwnedAgentPreparation(json));
        if (new OwnedAgentPreparation(Prepare()).ToString() != "OwnedAgentPreparation(private)") throw new Exception("Diagnostic leak.");
    }
    public static void ExplicitScopedSingleUse()
    {
        foreach (var scope in new[] { "desktop-view", "desktop-control" })
        {
            var p = new OwnedAgentPreparation(Prepare(scope));
            Reject(() => p.Activate("{\"Op\":\"activate\"}"));
            var request = p.BeginRequest(Consent());
            if (request.RequestID != Request || request.Controller != Controller) throw new Exception("Unbound preview.");
            using var reply = JsonDocument.Parse(JsonSerializer.Serialize(p.Decide(true, scope == "desktop-control")));
            if (!reply.RootElement.GetProperty("Allow").GetBoolean() || reply.RootElement.GetProperty("Permission").GetString() != scope ||
                reply.RootElement.GetProperty("RequestID").GetString() != Request) throw new Exception("Unbound consent.");
            Reject(() => p.Decide(true, true));
            var expected = scope == "desktop-control" ? DesktopSessionPermission.Control : DesktopSessionPermission.View;
            if (p.Activate("{\"Op\":\"activate\"}") != expected) throw new Exception("Scope changed.");
            Reject(() => p.Activate("{\"Op\":\"activate\"}"));
        }
        var noControl = new OwnedAgentPreparation(Prepare("desktop-control"));
        noControl.BeginRequest(Consent()); Reject(() => noControl.Decide(true, false));
        Reject(() => noControl.Activate("{\"Op\":\"activate\"}"));
    }
    public static void DenyCancelMalformedNeverActivate()
    {
        foreach (var mode in new[] { "deny", "cancel", "self", "extra", "wrong-op" })
        {
            var p = new OwnedAgentPreparation(Prepare());
            if (mode == "self") Reject(() => p.BeginRequest(Consent(Self)));
            else if (mode == "extra") Reject(() => p.BeginRequest(Consent()[..^1] + ",\"Allow\":\"true\"}"));
            else
            {
                p.BeginRequest(Consent());
                if (mode == "deny") p.Decide(false, false);
                else if (mode == "cancel") p.Cancel();
                else { p.Decide(true, false); Reject(() => p.Activate("{\"Op\":\"activate\",\"Port\":\"47478\"}")); }
            }
            Reject(() => p.Activate("{\"Op\":\"activate\"}"));
            Reject(() => p.BeginRequest(Consent()));
        }
    }
    public static void InviteDisplayIsPrivateSingleUse()
    {
        var p = new OwnedAgentPreparation(Prepare());
        var command = Frame(("Op", "show_invite"), ("DeviceCode", "123456abcdef"), ("Code", "12345678"));
        var display = p.ShowInvite(command);
        if (display.DeviceCode != "123456abcdef" || display.Code != "12345678") throw new Exception("Incorrect invite.");
        Reject(() => p.ShowInvite(command));
        p.BeginRequest(Consent()); p.Decide(false, false);
        var malformed = new OwnedAgentPreparation(Prepare());
        Reject(() => malformed.ShowInvite(command.Replace("12345678", "1234567x")));
        Reject(() => malformed.BeginRequest(Consent()));
    }
}
