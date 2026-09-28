import Foundation
import Network
import Security

public enum TLSProbeError: Error, Equatable {
    case authenticationRejected
    case invalidDeviceKey
    case invalidPort
    case missingPeerCertificate
    case firstUseRejected
    case fingerprintMismatch
    case trustStoreFailure
    case connectionFailed
    case timedOut
    case responseTooLarge
    case unexpectedResponse
}

public final class TLSControllerClient {
    public typealias FirstUseApproval = (CertificateFingerprint) -> Bool
    public typealias Completion = (Result<Void, TLSProbeError>) -> Void

    private let queue: DispatchQueue
    private let trustCoordinator: StoredCertificateTrustCoordinator

    public init(
        trustStore: TrustedFingerprintStore,
        queue: DispatchQueue = DispatchQueue(label: "com.personalremotedesktop.tls-client")
    ) {
        self.trustCoordinator = StoredCertificateTrustCoordinator(store: trustStore)
        self.queue = queue
    }

    public func runProbe(
        host: String,
        port: UInt16,
        deviceIdentifier: String,
        deviceKey: Data,
        sendPreAuthenticationPing: Bool = false,
        timeout: TimeInterval = 10,
        approveFirstUse: @escaping FirstUseApproval,
        completion: @escaping Completion
    ) {
        guard let networkPort = NWEndpoint.Port(rawValue: port) else {
            completion(.failure(.invalidPort))
            return
        }

        let session: AuthenticatedProbeSession
        do { session = try AuthenticatedProbeSession(deviceKey: deviceKey) }
        catch { completion(.failure(.invalidDeviceKey)); return }
        let tlsOptions = NWProtocolTLS.Options()
        sec_protocol_options_set_min_tls_protocol_version(
            tlsOptions.securityProtocolOptions,
            .TLSv12
        )
        sec_protocol_options_set_verify_block(
            tlsOptions.securityProtocolOptions,
            { [trustCoordinator] _, trust, verifyComplete in
                let secTrust = sec_trust_copy_ref(trust).takeRetainedValue()
                guard let certificateChain = SecTrustCopyCertificateChain(secTrust) as? [SecCertificate],
                      let certificate = certificateChain.first else {
                    verifyComplete(false)
                    return
                }
                let certificateDER = SecCertificateCopyData(certificate) as Data
                do {
                    let result = try trustCoordinator.evaluate(
                        deviceIdentifier: deviceIdentifier,
                        certificateDER: certificateDER,
                        approveFirstUse: approveFirstUse
                    )
                    switch result {
                    case .approvedAndStored, .trusted:
                        verifyComplete(true)
                    case .rejectedFirstUse, .rejectedFingerprintMismatch:
                        verifyComplete(false)
                    }
                } catch {
                    verifyComplete(false)
                }
            },
            queue
        )

        let parameters = NWParameters(tls: tlsOptions)
        parameters.allowLocalEndpointReuse = false
        let connection = NWConnection(
            host: NWEndpoint.Host(host),
            port: networkPort,
            using: parameters
        )
        let state = ProbeState(connection: connection, session: session, completion: completion)

        connection.stateUpdateHandler = { [weak self, weak state] newState in
            guard let self, let state else { return }
            switch newState {
            case .ready:
                do {
                    let first: Frame
                    if sendPreAuthenticationPing {
                        first = Frame(type: .ping, sequence: 1, payload: Data(repeating: 0, count: 8))
                    } else {
                        first = try state.session.start()
                    }
                    self.send([first], state: state)
                } catch { state.finish(.failure(.unexpectedResponse)) }
            case .failed:
                state.finish(.failure(.connectionFailed))
            case .cancelled:
                break
            default:
                break
            }
        }

        queue.asyncAfter(deadline: .now() + timeout) { [state] in
            state.finish(.failure(.timedOut))
        }
        connection.start(queue: queue)
    }

    private func send(_ frames: [Frame], state: ProbeState) {
        do {
            let wire = try frames.reduce(into: Data()) { $0.append(try FrameCodec.encode($1)) }
            guard !wire.isEmpty else { receiveResponse(state); return }
            state.connection.send(content: wire, completion: .contentProcessed { [weak self, weak state] error in
                guard let self, let state else { return }
                if error != nil { state.finish(.failure(.connectionFailed)); return }
                self.receiveResponse(state)
            })
        } catch { state.finish(.failure(.unexpectedResponse)) }
    }

    private func receiveResponse(_ state: ProbeState) {
        state.connection.receive(minimumIncompleteLength: 1, maximumLength: 92) { [weak self, weak state] data, _, isComplete, error in
            guard let self, let state else { return }
            if error != nil { state.finish(.failure(.connectionFailed)); return }
            do {
                var replies: [Frame] = []
                for frame in try state.decoder.append(data ?? Data()) {
                    replies += try state.session.receive(frame)
                }
                if state.session.isComplete { state.finish(.success(())); return }
                if isComplete { state.finish(.failure(.unexpectedResponse)); return }
                self.send(replies, state: state)
            } catch let error as TLSProbeError {
                state.finish(.failure(error))
            } catch { state.finish(.failure(.unexpectedResponse)) }
        }
    }
}
private final class ProbeState {
    var session: AuthenticatedProbeSession
    var decoder = ProbeFrameDecoder()
    let connection: NWConnection
    private let completion: TLSControllerClient.Completion
    private let lock = NSLock()
    private var finished = false

    init(connection: NWConnection, session: AuthenticatedProbeSession, completion: @escaping TLSControllerClient.Completion) {
        self.session = session
        self.connection = connection
        self.completion = completion
    }

    func finish(_ result: Result<Void, TLSProbeError>) {
        lock.lock()
        guard !finished else {
            lock.unlock()
            return
        }
        finished = true
        lock.unlock()

        connection.stateUpdateHandler = nil
        connection.cancel()
        completion(result)
    }
}
