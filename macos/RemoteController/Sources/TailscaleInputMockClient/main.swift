import Darwin
import Foundation
import RemoteProtocol

let arguments = Array(CommandLine.arguments.dropFirst())
guard arguments.count == 1 || arguments.count == 2,
      let port = UInt16(arguments.count == 2 ? arguments[1] : "47475"), port != 0 else {
    print("Usage: swift run -c release TailscaleInputMockClient <Windows-Tailscale-IPv4> [port]")
    exit(2)
}
let host = arguments[0]
let key: Data
let fingerprint: CertificateFingerprint
do {
    guard let storedKey = try KeychainDeviceKeyStore().loadKey(for: host) else {
        print("FAIL no paired device key for this exact host; use the existing TLSProbeClient pairing flow first")
        exit(2)
    }
    guard let storedFingerprint = try KeychainTrustedFingerprintStore().loadFingerprint(for: host) else {
        print("FAIL no approved certificate fingerprint for this exact host; complete the existing TLS probe first")
        exit(2)
    }
    key = storedKey
    fingerprint = storedFingerprint
} catch {
    print("FAIL could not read the paired key or certificate fingerprint from Keychain")
    exit(1)
}

let inputs: [CapturedInput]
let releases: [CapturedInput]
do {
    (inputs, releases) = try SyntheticInputMockVector.make()
} catch {
    print("FAIL built-in synthetic input vector is invalid")
    exit(1)
}

var finished = false
var exitCode: Int32 = 1
var client: TLSInputSimulationClient!
do {
    client = try TLSInputSimulationClient(tailscaleHost: host, port: port,
        expectedFingerprint: fingerprint, deviceKey: key, localControlAllowed: true,
        connectionTimeout: 30,
        onAuthenticated: {
            print("AUTHENTICATED input mock; sending \(inputs.count + releases.count) synthetic events")
            guard client.submit(inputs) else { return }
            client.finish(releases: releases)
        },
        completion: { result in
            switch result {
            case .success:
                print("PASS synthetic input sent and release frames drained; no native input requested")
                exitCode = 0
            case .failure(let error):
                print("FAIL input mock: \(error)")
            }
            finished = true
        })
} catch TLSProbeError.invalidHost {
    print("FAIL host must be a literal Tailscale IPv4 address in 100.64.0.0/10")
    exit(2)
} catch {
    print("FAIL invalid input mock configuration")
    exit(2)
}

print("CONNECTING input mock to \(host):\(port); synthetic events only")
client.start()
let commandDeadline = Date().addingTimeInterval(40)
while !finished && Date() < commandDeadline {
    _ = RunLoop.current.run(mode: .default, before: Date().addingTimeInterval(0.05))
}
if !finished {
    client.cancel()
    let cancellationDeadline = Date().addingTimeInterval(2)
    while !finished && Date() < cancellationDeadline {
        _ = RunLoop.current.run(mode: .default, before: Date().addingTimeInterval(0.05))
    }
    print("FAIL input mock command timed out")
    exitCode = 1
}
withExtendedLifetime(client) {}
exit(exitCode)
