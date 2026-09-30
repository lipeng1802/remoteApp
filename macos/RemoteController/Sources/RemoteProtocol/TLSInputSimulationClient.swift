import Foundation
import Network
import Security

/// Development-only TLS input client. The default initializer is loopback-only;
/// the explicitly named cross-device initializer accepts only a literal Tailscale
/// IPv4 address. Requires an already approved SHA-256 certificate fingerprint and
/// never automatically trusts or persists a cert. No production GUI integration.
public final class TLSInputSimulationClient {
    private let driver: InputConnectionDriver

    public convenience init(port: UInt16, expectedFingerprint: CertificateFingerprint, deviceKey: Data,
                localControlAllowed: Bool = false,
                connectionTimeout: TimeInterval = 15,
                onJpegFrame: ((ScreenInfoPayload, Data) -> Void)? = nil,
                onAuthenticated: @escaping () -> Void,
                completion: @escaping (Result<Void, InputSimulationError>) -> Void) throws {
        try self.init(host: "127.0.0.1", port: port, expectedFingerprint: expectedFingerprint,
            deviceKey: deviceKey, localControlAllowed: localControlAllowed,
            connectionTimeout: connectionTimeout, onJpegFrame: onJpegFrame,
            onAuthenticated: onAuthenticated,
            completion: completion)
    }

    public convenience init(tailscaleHost host: String, port: UInt16,
                expectedFingerprint: CertificateFingerprint, deviceKey: Data,
                localControlAllowed: Bool = false,
                connectionTimeout: TimeInterval = 15,
                onJpegFrame: ((ScreenInfoPayload, Data) -> Void)? = nil,
                onAuthenticated: @escaping () -> Void,
                completion: @escaping (Result<Void, InputSimulationError>) -> Void) throws {
        guard InputSimulationEndpointPolicy.isTailscaleIPv4(host) else {
            throw TLSProbeError.invalidHost
        }
        try self.init(host: host, port: port, expectedFingerprint: expectedFingerprint,
            deviceKey: deviceKey, localControlAllowed: localControlAllowed,
            connectionTimeout: connectionTimeout, onJpegFrame: onJpegFrame,
            onAuthenticated: onAuthenticated,
            completion: completion)
    }

    private init(host: String, port: UInt16, expectedFingerprint: CertificateFingerprint,
                deviceKey: Data, localControlAllowed: Bool,
                connectionTimeout: TimeInterval,
                onJpegFrame: ((ScreenInfoPayload, Data) -> Void)?,
                onAuthenticated: @escaping () -> Void,
                completion: @escaping (Result<Void, InputSimulationError>) -> Void) throws {
        let queue = DispatchQueue(label: "prd.input.tls-simulation")
        let transport = try NetworkInputTransport(host: host, port: port,
            expectedFingerprint: expectedFingerprint, queue: queue)
        driver = try InputConnectionDriver(transport: transport, deviceKey: deviceKey,
            localControlAllowed: localControlAllowed, queue: queue, connectTimeout: connectionTimeout,
            onJpegFrame: onJpegFrame.map { callback in
                { frame in callback(frame.screen, frame.jpeg) }
            },
            onAuthenticated: onAuthenticated, completion: completion)
    }
    deinit { driver.cancel() }
    public func start() { driver.start() }
    @discardableResult
    public func submit(_ inputs: [CapturedInput]) -> Bool { driver.submit(inputs) }
    public func finish(releases: [CapturedInput]) { driver.finish(releases: releases) }
    public func cancel() { driver.cancel() }
}

private final class NetworkInputTransport: InputTransport {
    private let connection: NWConnection

    init(host: String, port: UInt16, expectedFingerprint: CertificateFingerprint, queue: DispatchQueue) throws {
        guard port != 0, let networkPort = NWEndpoint.Port(rawValue: port) else {
            throw TLSProbeError.invalidPort
        }
        let tls = NWProtocolTLS.Options()
        sec_protocol_options_set_min_tls_protocol_version(tls.securityProtocolOptions, .TLSv12)
        sec_protocol_options_set_verify_block(tls.securityProtocolOptions, { _, trust, complete in
            let secTrust = sec_trust_copy_ref(trust).takeRetainedValue()
            guard let chain = SecTrustCopyCertificateChain(secTrust) as? [SecCertificate],
                  let certificate = chain.first else { complete(false); return }
            let der = SecCertificateCopyData(certificate) as Data
            let decision = try? CertificateTrustPolicy.evaluate(storedFingerprint: expectedFingerprint, certificateDER: der)
            complete(decision == .trusted)
        }, queue)
        let tcp = NWProtocolTCP.Options()
        tcp.noDelay = true
        let parameters = NWParameters(tls: tls, tcp: tcp)
        parameters.allowLocalEndpointReuse = false
        connection = NWConnection(host: NWEndpoint.Host(host), port: networkPort, using: parameters)
    }

    func start(queue: DispatchQueue, state: @escaping (InputTransportState) -> Void) {
        connection.stateUpdateHandler = { next in
            switch next {
            case .ready: state(.ready)
            case .failed, .cancelled: state(.failed)
            default: break
            }
        }
        connection.start(queue: queue)
    }
    func send(_ bytes: Data, completion: @escaping (Bool) -> Void) {
        connection.send(content: bytes, completion: .contentProcessed { completion($0 == nil) })
    }
    func receive(completion: @escaping (Data?, Bool, Bool) -> Void) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 65_536) { data, _, eof, error in
            completion(data, eof, error == nil)
        }
    }
    func cancel() {
        connection.stateUpdateHandler = nil
        connection.cancel()
    }
}

enum InputSimulationEndpointPolicy {
    static func isTailscaleIPv4(_ host: String) -> Bool {
        let parts = host.split(separator: ".", omittingEmptySubsequences: false)
        guard parts.count == 4 else { return false }
        let octets = parts.compactMap { part -> UInt8? in
            guard !part.isEmpty, part.allSatisfy(\.isNumber),
                  let value = UInt8(part), String(value) == part else { return nil }
            return value
        }
        return octets.count == 4 && octets[0] == 100 && (64...127).contains(octets[1])
    }
}
