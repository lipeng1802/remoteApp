import Foundation
import Network
import Security

public enum TLSProbeError: Error, Equatable {
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

    private static let request = Data("prd-tls-test\n".utf8)
    private static let expectedResponse = Data("prd-tls-ok\n".utf8)
    private static let maximumResponseLength = 32

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
        timeout: TimeInterval = 10,
        approveFirstUse: @escaping FirstUseApproval,
        completion: @escaping Completion
    ) {
        guard let networkPort = NWEndpoint.Port(rawValue: port) else {
            completion(.failure(.invalidPort))
            return
        }

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
        let state = ProbeState(connection: connection, completion: completion)

        connection.stateUpdateHandler = { [weak self, weak state] newState in
            guard let self, let state else { return }
            switch newState {
            case .ready:
                self.sendRequest(state)
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

    private func sendRequest(_ state: ProbeState) {
        state.connection.send(content: Self.request, completion: .contentProcessed { [weak self, weak state] error in
            guard let self, let state else { return }
            if error != nil {
                state.finish(.failure(.connectionFailed))
                return
            }
            self.receiveResponse(state, accumulated: Data())
        })
    }

    private func receiveResponse(_ state: ProbeState, accumulated: Data) {
        state.connection.receive(minimumIncompleteLength: 1, maximumLength: 32) { [weak self, weak state] data, _, isComplete, error in
            guard let self, let state else { return }
            if error != nil {
                state.finish(.failure(.connectionFailed))
                return
            }

            var response = accumulated
            if let data { response.append(data) }
            guard response.count <= Self.maximumResponseLength else {
                state.finish(.failure(.responseTooLarge))
                return
            }
            if response.contains(0x0a) || isComplete {
                state.finish(response == Self.expectedResponse ? .success(()) : .failure(.unexpectedResponse))
                return
            }
            self.receiveResponse(state, accumulated: response)
        }
    }
}

private final class ProbeState {
    let connection: NWConnection
    private let completion: TLSControllerClient.Completion
    private let lock = NSLock()
    private var finished = false

    init(connection: NWConnection, completion: @escaping TLSControllerClient.Completion) {
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
