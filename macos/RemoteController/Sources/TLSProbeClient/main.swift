import Dispatch
import Darwin
import Foundation
import RemoteProtocol

let arguments = CommandLine.arguments
guard arguments.count == 2 || arguments.count == 3,
      let port = UInt16(arguments.count == 3 ? arguments[2] : "47475") else {
    print("Usage: swift run TLSProbeClient <Windows Tailscale host-or-IP> [port]")
    exit(2)
}

let host = arguments[1]
let completionSignal = DispatchSemaphore(value: 0)
let resultLock = NSLock()
var probeResult: Result<Void, TLSProbeError>?
let client = TLSControllerClient(trustStore: KeychainTrustedFingerprintStore())

client.runProbe(
    host: host,
    port: port,
    deviceIdentifier: host,
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

guard completionSignal.wait(timeout: .now() + 15) == .success else {
    print("FAIL TLS probe did not complete before the command timeout")
    exit(1)
}
resultLock.lock()
let finalResult = probeResult
resultLock.unlock()
switch finalResult {
case .success:
    print("PASS TLS handshake, stored fingerprint policy, and fixed probe response")
case let .failure(error):
    print("FAIL TLS probe: \(error)")
    exit(1)
case nil:
    print("FAIL TLS probe returned no result")
    exit(1)
}
