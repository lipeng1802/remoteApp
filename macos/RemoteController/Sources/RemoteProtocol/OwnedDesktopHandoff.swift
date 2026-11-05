import CoreGraphics
import Foundation

public enum OwnedDesktopHandoffError: Error, Equatable {
    case invalidBootstrap, consumed, localControlRequired, controlNotGranted
}

/// One controller session delivered by the pinned local owner over its inherited
/// private pipe. Decoding does not authenticate the sender: the GUI integration
/// must establish that pipe and retain the owner's lifetime/authorization gate.
/// Never populate this from argv, preferences, a network reply or a public socket.
public final class OwnedDesktopHandoff: CustomStringConvertible, CustomDebugStringConvertible {
    private struct Launch: Decodable {
        let Port: UInt16
        let Key: String
        let Fingerprint: String
        let Target: String
        let Permission: String
    }

    private let lock = NSLock()
    private var launch: Launch?

    public var description: String { "OwnedDesktopHandoff(private)" }
    public var debugDescription: String { description }

    /// Requires a desktop-specific owner bootstrap, without the trailing LF.
    /// A legacy four-field fixed-payload bootstrap is deliberately insufficient.
    /// Exact canonical bytes reject duplicate/unknown fields, alternate types and
    /// endpoint selectors rather than letting a general JSON decoder ignore them.
    public init(privateBootstrap data: Data) throws {
        guard !data.isEmpty, data.count < 8192,
              let value = try? JSONDecoder().decode(Launch.self, from: data),
              value.Port != 0,
              let key = Data(base64Encoded: value.Key), key.count == 32,
              key.base64EncodedString() == value.Key,
              Self.hex(value.Fingerprint), Self.hex(value.Target),
              ["desktop-view", "desktop-control"].contains(value.Permission),
              let canonical = try? JSONSerialization.data(withJSONObject: [
                "Port": Int(value.Port), "Key": value.Key,
                "Fingerprint": value.Fingerprint, "Target": value.Target,
                "Permission": value.Permission
              ], options: [.sortedKeys, .withoutEscapingSlashes]),
              canonical == data else {
            throw OwnedDesktopHandoffError.invalidBootstrap
        }
        launch = value
    }

    private static func hex(_ value: String) -> Bool {
        value.utf8.count == 64 && value.utf8.allSatisfy {
            (48...57).contains($0) || (97...102).contains($0)
        }
    }

    private func take() throws -> (UInt16, Data, CertificateFingerprint, String, String) {
        lock.lock()
        defer { lock.unlock() }
        guard let value = launch else { throw OwnedDesktopHandoffError.consumed }
        launch = nil
        // Constructor has already enforced canonical ASCII hex and base64.
        let bytes = stride(from: 0, to: 64, by: 2).map { offset -> UInt8 in
            let start = value.Fingerprint.index(value.Fingerprint.startIndex, offsetBy: offset)
            let end = value.Fingerprint.index(start, offsetBy: 2)
            return UInt8(value.Fingerprint[start..<end], radix: 16)!
        }
        return (value.Port, Data(base64Encoded: value.Key)!,
                try CertificateFingerprint(bytes: Data(bytes)), value.Target, value.Permission)
    }

    /// Drops an unused handoff on owner cancellation; it can never be reopened.
    public func discard() {
        lock.lock()
        launch = nil
        lock.unlock()
    }

    /// Uses the existing certificate pin/HMAC/video protocol with no TOFU or
    /// Keychain fallback. The loopback port is fixed by the live local owner.
    @discardableResult
    public func startReadOnly(onJpegFrame: @escaping (ScreenInfoPayload, CGImage) -> Void,
                              onStatus: @escaping (String) -> Void,
                              completion: @escaping TLSControllerClient.Completion) throws -> () -> Void {
        let (port, key, fingerprint, target, _) = try take()
        let client = TLSControllerClient(trustStore: SessionPin(fingerprint, target: target))
        return client.runProbe(host: "127.0.0.1", port: port, deviceIdentifier: target,
            deviceKey: key, onJpegFrame: onJpegFrame, onStatus: onStatus,
            approveFirstUse: { _ in false }, completion: completion)
    }

    /// Consumes the handoff even on refusal. The UI must obtain fresh local
    /// consent for control, in addition to the owner's exact signed grant scope.
    public func makeControlClient(localControlAllowed: Bool,
                                  onJpegFrame: @escaping (ScreenInfoPayload, Data) -> Void,
                                  onAuthenticated: @escaping () -> Void,
                                  completion: @escaping (Result<Void, InputSimulationError>) -> Void) throws -> TLSInputSimulationClient {
        let (port, key, fingerprint, _, permission) = try take()
        guard permission == "desktop-control" else { throw OwnedDesktopHandoffError.controlNotGranted }
        guard localControlAllowed else { throw OwnedDesktopHandoffError.localControlRequired }
        return try TLSInputSimulationClient(port: port, expectedFingerprint: fingerprint,
            deviceKey: key, localControlAllowed: true, onJpegFrame: onJpegFrame,
            onAuthenticated: onAuthenticated, completion: completion)
    }
}

private final class SessionPin: TrustedFingerprintStore {
    private let fingerprint: CertificateFingerprint
    private let target: String
    init(_ fingerprint: CertificateFingerprint, target: String) {
        self.fingerprint = fingerprint
        self.target = target
    }
    func loadFingerprint(for deviceIdentifier: String) throws -> CertificateFingerprint? {
        guard deviceIdentifier == target else { throw OwnedDesktopHandoffError.invalidBootstrap }
        return fingerprint
    }
    func saveFingerprint(_ fingerprint: CertificateFingerprint, for deviceIdentifier: String) throws {
        throw OwnedDesktopHandoffError.invalidBootstrap
    }
    func removeFingerprint(for deviceIdentifier: String) throws {
        throw OwnedDesktopHandoffError.invalidBootstrap
    }
}
