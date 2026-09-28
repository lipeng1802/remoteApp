import Dispatch
import Darwin
import Foundation
import RemoteProtocol

var arguments = Array(CommandLine.arguments.dropFirst())
let mode = arguments.first?.hasPrefix("--") == true ? arguments.removeFirst() : "--probe"
guard ["--probe", "--pair", "--wrong-key", "--preauth"].contains(mode),
      arguments.count == 1 || arguments.count == 2,
      let port = UInt16(arguments.count == 2 ? arguments[1] : "47475"), port != 0 else {
    print("Usage: swift run TLSProbeClient [--pair|--wrong-key|--preauth] <Windows Tailscale host-or-IP> [port]")
    exit(2)
}
let host = arguments[0]
let keys = KeychainDeviceKeyStore()
if mode == "--pair" {
    guard isatty(STDIN_FILENO) != 0, let input = getpass("Windows pairing key (Base64, hidden): ") else {
        print("FAIL pairing requires an interactive terminal")
        exit(2)
    }
    let length = strlen(input)
    let encoded = String(cString: input)
    memset(input, 0, length)
    guard let key = Data(base64Encoded: encoded), key.count == 32 else {
        print("FAIL expected a Base64-encoded 32-byte key")
        exit(2)
    }
    do { try keys.saveKey(key, for: host) }
    catch { print("FAIL could not store pairing key in Keychain"); exit(1) }
    print("PAIRED device key stored in Keychain; TLS certificate approval remains separate")
    exit(0)
}
let deviceKey: Data
do {
    guard let stored = try keys.loadKey(for: host) else {
        print("FAIL no pairing key; run TLSProbeClient --pair with the same host first")
        exit(2)
    }
    if mode == "--wrong-key" {
        var wrong = stored
        wrong[0] ^= 1 // Deliberately wrong for one probe; never overwrite Keychain.
        deviceKey = wrong
    } else { deviceKey = stored }
} catch { print("FAIL could not load pairing key from Keychain"); exit(1) }
let completionSignal = DispatchSemaphore(value: 0)
let resultLock = NSLock()
var probeResult: Result<Void, TLSProbeError>?
let client = TLSControllerClient(trustStore: KeychainTrustedFingerprintStore())

client.runProbe(
    host: host,
    port: port,
    deviceIdentifier: host,
    deviceKey: deviceKey,
    sendPreAuthenticationPing: mode == "--preauth",
    timeout: 180,
    approveFirstUse: { fingerprint in
        print("First connection. Windows certificate SHA-256:")
        print(fingerprint.hexadecimal)
        print("Approve and store this fingerprint in Keychain? [y/N] ", terminator: "")
        return readLine()?.lowercased() == "y"
    },
    completion: { result in
        resultLock.lock()
        probeResult = result
        resultLock.unlock()
        completionSignal.signal()
    }
)

guard completionSignal.wait(timeout: .now() + 185) == .success else {
    print("FAIL TLS probe did not complete before the command timeout")
    exit(1)
}
resultLock.lock()
let finalResult = probeResult
resultLock.unlock()
switch finalResult {
case .success:
    print("PASS TLS handshake, stored fingerprint policy, application authentication, and PING/PONG")
case let .failure(error):
    print("FAIL TLS probe: \(error)")
    exit(1)
case nil:
    print("FAIL TLS probe returned no result")
    exit(1)
}
