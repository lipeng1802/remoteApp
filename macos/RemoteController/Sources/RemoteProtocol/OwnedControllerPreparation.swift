import Foundation

public enum OwnedControllerPreparationError: Error, Equatable {
    case invalidFrame, invalidState
}

/// Protocol state for a separately pinned GUI child, using only its inherited
/// owner pipe. Parsing does not authenticate the parent. No TLS/client/capture
/// is created until a matching bootstrap follows explicit local consent.
public final class OwnedControllerPreparation: CustomStringConvertible, CustomDebugStringConvertible {
    public struct RequestPreview: Equatable {
        public let requestID: String
        public let controller: String
        public let permission: String
    }
    private struct Prepare: Decodable {
        let Op, Permission, Role, own: String
        enum CodingKeys: String, CodingKey { case Op, Permission, Role; case own = "Self" }
    }
    private struct Request: Decodable { let Op, RequestID, Controller: String }
    private struct Scope: Decodable { let Permission, Target: String }
    private enum Phase { case prepared, deciding, allowed, closed }
    private let lock = NSLock()
    private let ownID: String
    private let permission: String
    private var phase = Phase.prepared
    private var requestID: String?

    public var description: String { "OwnedControllerPreparation(private)" }
    public var debugDescription: String { description }

    /// Exact Go canonical JSON without LF; argv, preferences and public sockets
    /// must never be used as this source. The Mac child is controller-only.
    public init(privatePreparation data: Data) throws {
        guard !data.isEmpty, data.count < 8192,
              let p = try? JSONDecoder().decode(Prepare.self, from: data),
              p.Op == "prepare", p.Role == "controller", Self.hex(p.own, count: 64),
              ["desktop-view", "desktop-control"].contains(p.Permission),
              Self.canonical(["Op": p.Op, "Permission": p.Permission, "Role": p.Role, "Self": p.own]) == data else {
            throw OwnedControllerPreparationError.invalidFrame
        }
        ownID = p.own
        permission = p.Permission
    }

    public func preparedReply() -> Data {
        Self.canonical(["status": "desktop_prepared", "Permission": permission, "Role": "controller"])!
    }

    public func beginRequest(privateRequest data: Data) throws -> RequestPreview {
        lock.lock()
        defer { lock.unlock() }
        guard phase == .prepared else { throw OwnedControllerPreparationError.invalidState }
        phase = .closed // a malformed/unknown attempt cannot be replayed
        guard !data.isEmpty, data.count < 8192,
              let r = try? JSONDecoder().decode(Request.self, from: data),
              r.Op == "consent", Self.hex(r.RequestID, count: 32), r.Controller == ownID,
              Self.canonical(["Op": r.Op, "RequestID": r.RequestID, "Controller": r.Controller]) == data else {
            throw OwnedControllerPreparationError.invalidFrame
        }
        requestID = r.RequestID
        phase = .deciding
        return RequestPreview(requestID: r.RequestID, controller: ownID, permission: permission)
    }

    /// Invoke only from a fresh local GUI decision. No default/preselected
    /// approval and no script/peer frame can substitute for that user action.
    public func decide(localUserAllowed: Bool) throws -> Data {
        lock.lock()
        defer { lock.unlock() }
        guard phase == .deciding, let request = requestID else {
            throw OwnedControllerPreparationError.invalidState
        }
        phase = localUserAllowed ? .allowed : .closed
        return Self.canonical(["status": "desktop_decision", "RequestID": request,
                               "Permission": permission, "Allow": localUserAllowed])!
    }

    /// The Go owner must independently correlate the exact approved request to
    /// its signed grant and ACTIVE session. This is only the GUI half of the
    /// handoff, not another authorization source. Failure consumes the attempt.
    public func acceptBootstrap(privateBootstrap data: Data) throws -> OwnedDesktopHandoff {
        lock.lock()
        defer { lock.unlock() }
        guard phase == .allowed else { throw OwnedControllerPreparationError.invalidState }
        phase = .closed
        guard !data.isEmpty, data.count < 8192,
              let scope = try? JSONDecoder().decode(Scope.self, from: data),
              scope.Permission == permission, scope.Target != ownID else {
            throw OwnedControllerPreparationError.invalidFrame
        }
        return try OwnedDesktopHandoff(privateBootstrap: data)
    }

    /// Cancels an unused preparation. The caller must still join its pipe
    /// reader/GUI process, and explicitly stop any already-issued client.
    public func cancel() {
        lock.lock()
        phase = .closed
        requestID = nil
        lock.unlock()
    }

    private static func hex(_ value: String, count: Int) -> Bool {
        value.utf8.count == count && value.utf8.allSatisfy {
            (48...57).contains($0) || (97...102).contains($0)
        }
    }
    private static func canonical(_ object: [String: Any]) -> Data? {
        try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys, .withoutEscapingSlashes])
    }
}
