// Private development fixture: actual product TLS client, no keychain/UI/input.
import Foundation
import RemoteProtocol

func line() throws -> Data? {
    var data = Data()
    while data.count < 8192 {
        guard let b = try FileHandle.standardInput.read(upToCount: 1), !b.isEmpty else {
            if !data.isEmpty { throw NSError(domain: "fixture", code: 1) }
            return nil
        }
        if b[0] == 10 { return data }
        data.append(b)
    }
    throw NSError(domain: "fixture", code: 1)
}
func output(_ value: [String: Any]) { if let d = try? JSONSerialization.data(withJSONObject: value) { FileHandle.standardOutput.write(d + Data([10])) } }
final class Pin: TrustedFingerprintStore {
    let fingerprint: CertificateFingerprint
    let target: String
    init(_ data: Data, target: String) throws { fingerprint = try CertificateFingerprint(bytes: data); self.target = target }
    func loadFingerprint(for deviceIdentifier: String) throws -> CertificateFingerprint? {
        guard deviceIdentifier == target else { throw NSError(domain: "fixture", code: 1) }
        return fingerprint
    }
    func saveFingerprint(_ fingerprint: CertificateFingerprint, for deviceIdentifier: String) throws { throw NSError(domain: "fixture", code: 1) }
    func removeFingerprint(for deviceIdentifier: String) throws {}
}
struct Bootstrap: Decodable { let Port: UInt16; let Key: String; let Fingerprint: String; let Target: String; let Preauth: Bool? }
struct Command: Decodable { let Op: String }
do {
    guard let data = try line() else { exit(1) }
    let c = try JSONDecoder().decode(Bootstrap.self, from: data)
    guard let key = Data(base64Encoded: c.Key), key.count == 32, c.Fingerprint.count == 64 else { exit(1) }
    var pin = Data()
    for i in stride(from: 0, to: 64, by: 2) {
        let a = c.Fingerprint.index(c.Fingerprint.startIndex, offsetBy: i)
        let b = c.Fingerprint.index(a, offsetBy: 2)
        guard let byte = UInt8(c.Fingerprint[a..<b], radix: 16) else { exit(1) }
        pin.append(byte)
    }
    let lock = NSLock()
    let completed = DispatchSemaphore(value: 0)
    var pongs = 0
    var ended = false
    var result = "pending"
    var pinRejected = false
    let client = TLSControllerClient(trustStore: try Pin(pin, target: c.Target))
    let stop = client.runProbe(host: "127.0.0.1", port: c.Port, deviceIdentifier: c.Target, deviceKey: key,
        sendPreAuthenticationPing: c.Preauth ?? false, timeout: 10, onProtectedPong: {
            lock.lock(); pongs += 1; lock.unlock()
        }, onStatus: { status in
            if status == "证书指纹不匹配" { lock.lock(); pinRejected = true; lock.unlock() }
        }, approveFirstUse: { _ in false }, completion: { value in
            lock.lock(); ended = true
            switch value {
            case .success: result = "success"
            case .failure(.authenticationRejected): result = "authentication_rejected"
            case .failure(.timedOut): result = "timeout"
            case .failure(.cancelled): result = "cancelled"
            case .failure: result = "connection_closed"
            }
            lock.unlock()
            completed.signal()
        })
    output(["status": "ready"])
    while let d = try line() {
        let request = try JSONDecoder().decode(Command.self, from: d)
        if request.Op == "stop" { break }
        guard request.Op == "count" else { exit(1) }
        lock.lock(); let value: [String: Any] = ["status": "count", "pongs": pongs, "ended": ended, "result": result, "pin_rejected": pinRejected]; lock.unlock()
        output(value)
    }
    stop()
    guard completed.wait(timeout: .now() + 3) == .success else { exit(1) }
    output(["status": "closed"])
} catch { output(["status": "failed"]); exit(1) }
