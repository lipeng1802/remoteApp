import Foundation
import CoreGraphics
import Network
import Security

public enum TLSProbeError: Error, Equatable {
    case cancelled
    case authenticationRejected
    case invalidDeviceKey
    case invalidHost
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

    @discardableResult
    public func runProbe(
        host: String,
        port: UInt16,
        deviceIdentifier: String,
        deviceKey: Data,
        sendPreAuthenticationPing: Bool = false,
        timeout: TimeInterval = 10,
        onJpegFrame: ((ScreenInfoPayload, CGImage) -> Void)? = nil,
        onStatus: @escaping (String) -> Void = { _ in },
        approveFirstUse: @escaping FirstUseApproval,
        completion: @escaping Completion
    ) -> () -> Void {
        guard port != 0, let networkPort = NWEndpoint.Port(rawValue: port) else {
            completion(.failure(.invalidPort))
            return {}
        }

        let session: AuthenticatedProbeSession
        do { session = try AuthenticatedProbeSession(deviceKey: deviceKey, streaming: onJpegFrame != nil) }
        catch { completion(.failure(.invalidDeviceKey)); return {} }
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

        let tcpOptions = NWProtocolTCP.Options()
        tcpOptions.noDelay = true
        let parameters = NWParameters(tls: tlsOptions, tcp: tcpOptions)
        parameters.allowLocalEndpointReuse = false
        let connection = NWConnection(
            host: NWEndpoint.Host(host),
            port: networkPort,
            using: parameters
        )
        let state = ProbeState(connection: connection, session: session, onJpegFrame: onJpegFrame, onStatus: onStatus, completion: completion)

        connection.stateUpdateHandler = { [weak self, weak state] newState in
            guard let self, let state else { return }
            switch newState {
            case .ready:
                onStatus("正在验证应用密钥" )
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
            if !state.session.streaming || !state.session.authenticated { state.finish(.failure(.timedOut)) }
        }
        onStatus("正在连接 TLS" )
        connection.start(queue: queue)
        return { [state, queue] in queue.async { state.finish(.failure(.cancelled)) } }
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
        state.readGeneration += 1
        let generation = state.readGeneration
        queue.asyncAfter(deadline: .now() + 15) { [weak state] in
            guard let state, state.readGeneration == generation else { return }
            state.finish(.failure(.timedOut))
        }
        state.connection.receive(minimumIncompleteLength: 1, maximumLength: state.session.streaming ? 65536 : 92) { [weak self, weak state] data, _, isComplete, error in
            guard let self, let state else { return }
            if error != nil { state.finish(.failure(.connectionFailed)); return }
            do {
                var replies: [Frame] = []
                for frame in try state.decoder.append(data ?? Data()) {
                    replies += try state.session.receive(frame)
                    if frame.type == .authResult { state.onStatus("已认证，等待画面") }
                    if frame.type == .videoFrameJPEG, let screen = state.session.screen {
                        // Invalid images are dropped; the frame boundary and PING remain usable.
                        if let image = JpegImageDecoder.decode(frame.payload) { state.onJpegFrame?(screen, image) }
                    }
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
    var readGeneration = 0
    let onJpegFrame: ((ScreenInfoPayload, CGImage) -> Void)?
    let onStatus: (String) -> Void
    let connection: NWConnection
    private let completion: TLSControllerClient.Completion
    private let lock = NSLock()
    private var finished = false

    init(connection: NWConnection, session: AuthenticatedProbeSession, onJpegFrame: ((ScreenInfoPayload, CGImage) -> Void)?, onStatus: @escaping (String) -> Void, completion: @escaping TLSControllerClient.Completion) {
        self.onJpegFrame = onJpegFrame
        self.onStatus = onStatus
        self.session = session
        self.connection = connection
        self.completion = completion
        self.decoder.allowJpeg = onJpegFrame != nil
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
