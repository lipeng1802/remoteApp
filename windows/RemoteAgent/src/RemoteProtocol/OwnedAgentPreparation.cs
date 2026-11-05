using System.Text;
using System.Text.Json;

namespace RemoteProtocol;

// State only, not authority verification. A trusted, pinned owner must correlate
// the request with its real signed approve/status before sending activate.
// No capture, credentials, socket, or native input is created by this class.
public sealed class OwnedAgentPreparation
{
    private enum Phase { Prepared, Deciding, Allowed, Closed }
    private Phase phase;
    private bool inviteShown;
    private string? request;
    public string Permission { get; }
    public string Self { get; }
    public DesktopSessionPermission Scope => Permission == "desktop-control"
        ? DesktopSessionPermission.Control : DesktopSessionPermission.View;
    public override string ToString() => "OwnedAgentPreparation(private)";

    public OwnedAgentPreparation(string json)
    {
        var p = Parse(json, "Op", "Permission", "Role", "Self");
        if (p["Op"] != "prepare" || p["Role"] != "target" || !Hex(p["Self"], 64) ||
            p["Permission"] is not ("desktop-view" or "desktop-control")) throw new InvalidDataException();
        Self = p["Self"];
        Permission = p["Permission"];
    }

    public (string DeviceCode, string Code) ShowInvite(string json)
    {
        if (phase != Phase.Prepared || inviteShown) throw new InvalidOperationException();
        inviteShown = true;
        phase = Phase.Closed;
        var p = Parse(json, "Op", "DeviceCode", "Code");
        if (p["Op"] != "show_invite" || !Hex(p["DeviceCode"], 12) ||
            p["Code"].Length != 8 || !p["Code"].All(char.IsAsciiDigit))
        { phase = Phase.Closed; throw new InvalidDataException(); }
        phase = Phase.Prepared;
        return (p["DeviceCode"], p["Code"]);
    }

    public (string RequestID, string Controller) BeginRequest(string json)
    {
        if (phase != Phase.Prepared) throw new InvalidOperationException();
        phase = Phase.Closed; // any unknown request consumes the attempt
        var p = Parse(json, "Op", "RequestID", "Controller");
        if (p["Op"] != "consent" || !Hex(p["RequestID"], 32) ||
            !Hex(p["Controller"], 64) || p["Controller"] == Self) throw new InvalidDataException();
        request = p["RequestID"];
        phase = Phase.Deciding;
        return (request, p["Controller"]);
    }

    // LocalControlAllowed is a fresh local checkbox + physical emergency hook,
    // never a field accepted in a parent/peer command.
    public object Decide(bool allow, bool localControlAllowed)
    {
        if (phase != Phase.Deciding || request is null) throw new InvalidOperationException();
        phase = Phase.Closed;
        if (allow && Scope == DesktopSessionPermission.Control && !localControlAllowed)
            throw new InvalidOperationException();
        if (allow) phase = Phase.Allowed;
        return new { status = "desktop_decision", RequestID = request, Permission, Allow = allow };
    }

    public DesktopSessionPermission Activate(string json)
    {
        if (phase != Phase.Allowed) throw new InvalidOperationException();
        phase = Phase.Closed;
        if (Parse(json, "Op")["Op"] != "activate") throw new InvalidDataException();
        return Scope;
    }

    public void Cancel() { phase = Phase.Closed; request = null; }

    // Go map json.Marshal: exact ordinal keys, string values, no LF. Canonical
    // comparison rejects duplicates/unknown fields, whitespace and escaped IDs.
    public static Dictionary<string, string> Parse(string json, params string[] fields)
    {
        if (string.IsNullOrEmpty(json) || Encoding.UTF8.GetByteCount(json) >= 8192) throw new InvalidDataException();
        using var d = JsonDocument.Parse(json);
        if (d.RootElement.ValueKind != JsonValueKind.Object) throw new InvalidDataException();
        var values = new SortedDictionary<string, string>(StringComparer.Ordinal);
        foreach (var p in d.RootElement.EnumerateObject())
            if (!fields.Contains(p.Name, StringComparer.Ordinal) || p.Value.ValueKind != JsonValueKind.String ||
                !values.TryAdd(p.Name, p.Value.GetString()!)) throw new InvalidDataException();
        if (values.Count != fields.Length || JsonSerializer.Serialize(values) != json) throw new InvalidDataException();
        return new Dictionary<string, string>(values, StringComparer.Ordinal);
    }

    public static bool Hex(string value, int count) => value.Length == count &&
        value.All(c => c is >= '0' and <= '9' or >= 'a' and <= 'f');
}
