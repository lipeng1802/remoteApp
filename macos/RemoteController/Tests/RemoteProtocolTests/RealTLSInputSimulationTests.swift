import Foundation
import Network
import Security
import XCTest
@testable import RemoteProtocol

final class RealTLSInputSimulationTests: XCTestCase {
    private let key = Data(repeating: 0x41, count: 32)

    func testRealTLSAuthenticatesAndDrainsInput() throws {
        let server = try LoopbackInputTLSServer(deviceKey: key)
        let received = expectation(description: "server received input and disconnect")
        server.onFinished = { frames in
            XCTAssertEqual(frames.map(\.type), [.mouseButton, .mouseButton, .disconnect])
            received.fulfill()
        }
        let port = try server.start()
        let authenticated = expectation(description: "authenticated")
        let completed = expectation(description: "client completed")
        var client: TLSInputSimulationClient!
        client = try TLSInputSimulationClient(
            port: port, expectedFingerprint: server.fingerprint, deviceKey: key,
            localControlAllowed: true,
            onAuthenticated: {
                authenticated.fulfill()
                let down = CapturedInput.button(MouseButtonPayload(button: .left, action: .down))
                let up = CapturedInput.button(MouseButtonPayload(button: .left, action: .up))
                XCTAssertTrue(client.submit([down]))
                client.finish(releases: [up])
            },
            completion: { result in
                if case .failure(let error) = result { XCTFail("Unexpected failure: \(error)") }
                completed.fulfill()
            })
        client.start()
        wait(for: [authenticated, received, completed], timeout: 5)
        withExtendedLifetime(client) {}
        server.stop()
    }

    func testRealTLSRejectsWrongFingerprint() throws {
        let server = try LoopbackInputTLSServer(deviceKey: key)
        let port = try server.start()
        let completed = expectation(description: "fingerprint rejected")
        let wrong = try CertificateFingerprint(bytes: Data(repeating: 0xff, count: 32))
        let client = try TLSInputSimulationClient(port: port, expectedFingerprint: wrong, deviceKey: key,
            localControlAllowed: true, connectionTimeout: 0.5,
            onAuthenticated: { XCTFail("Must not authenticate") }) { result in
                if case .failure(let error) = result { XCTAssertEqual(error, .timedOut) }
                else { XCTFail("Expected fingerprint rejection") }
                completed.fulfill()
            }
        client.start()
        wait(for: [completed], timeout: 2)
        withExtendedLifetime(client) {}
        server.stop()
    }

    func testRealTLSRejectsWrongDeviceKey() throws {
        let server = try LoopbackInputTLSServer(deviceKey: key)
        let port = try server.start()
        let completed = expectation(description: "application key rejected")
        let client = try TLSInputSimulationClient(port: port, expectedFingerprint: server.fingerprint,
            deviceKey: Data(repeating: 0x42, count: 32), localControlAllowed: true,
            onAuthenticated: { XCTFail("Must not authenticate") }) { result in
                if case .failure(let error) = result { XCTAssertEqual(error, .protocolFailure) }
                else { XCTFail("Expected rejection") }
                completed.fulfill()
            }
        client.start()
        wait(for: [completed], timeout: 5)
        withExtendedLifetime(client) {}
        server.stop()
    }

    func testRealTLSPartialFrameThenCloseFailsClosed() throws {
        let server = try LoopbackInputTLSServer(deviceKey: key, closeWithPartialFrame: true)
        let port = try server.start()
        let completed = expectation(description: "partial frame rejected")
        let client = try TLSInputSimulationClient(port: port, expectedFingerprint: server.fingerprint,
            deviceKey: key, localControlAllowed: true,
            onAuthenticated: {}) { result in
                if case .failure(let error) = result { XCTAssertEqual(error, .connectionFailed) }
                else { XCTFail("Expected closed connection") }
                completed.fulfill()
            }
        client.start()
        wait(for: [completed], timeout: 5)
        withExtendedLifetime(client) {}
        server.stop()
    }

    func testRealTLSClientCancelCancelsExactlyOnce() throws {
        let server = try LoopbackInputTLSServer(deviceKey: key)
        let port = try server.start()
        let completed = expectation(description: "cancelled")
        completed.assertForOverFulfill = true
        let client = try TLSInputSimulationClient(port: port, expectedFingerprint: server.fingerprint,
            deviceKey: key, localControlAllowed: true,
            onAuthenticated: { XCTFail("Cancelled client must not authenticate") }) { result in
                if case .failure(let error) = result { XCTAssertEqual(error, .cancelled) }
                else { XCTFail("Expected cancellation") }
                completed.fulfill()
            }
        client.start()
        client.cancel()
        wait(for: [completed], timeout: 2)
        withExtendedLifetime(client) {}
        server.stop()
    }
}

private final class LoopbackInputTLSServer {
    private enum ServerError: Error { case invalidFixture, listenerFailed, invalidProtocol }

    private let queue = DispatchQueue(label: "prd.tests.real-tls-server")
    private let identity: SecIdentity
    private let certificateDER: Data
    private let deviceKey: Data
    private let closeWithPartialFrame: Bool
    private var listener: NWListener?
    private var connection: NWConnection?
    private var decoder = FrameDecoder()
    private var controllerHello: HelloPayload?
    private var authenticated = false
    private var receivedInput: [Frame] = []
    var onFinished: (([Frame]) -> Void)?

    var fingerprint: CertificateFingerprint {
        get throws { try CertificateFingerprint.sha256(certificateDER: certificateDER) }
    }

    init(deviceKey: Data, closeWithPartialFrame: Bool = false) throws {
        self.deviceKey = deviceKey
        self.closeWithPartialFrame = closeWithPartialFrame
        guard let archive = Data(base64Encoded: Self.legacyPKCS12Base64) else { throw ServerError.invalidFixture }
        var imported: CFArray?
        let options = [kSecImportExportPassphrase as String: "test-only"] as CFDictionary
        guard SecPKCS12Import(archive as CFData, options, &imported) == errSecSuccess,
              let item = (imported as? [NSDictionary])?.first,
              let identity = item[kSecImportItemIdentity] as! SecIdentity? else {
            throw ServerError.invalidFixture
        }
        var certificate: SecCertificate?
        guard SecIdentityCopyCertificate(identity, &certificate) == errSecSuccess,
              let certificate else { throw ServerError.invalidFixture }
        self.identity = identity
        certificateDER = SecCertificateCopyData(certificate) as Data
    }

    func start() throws -> UInt16 {
        let tls = NWProtocolTLS.Options()
        guard let protocolIdentity = sec_identity_create(identity) else { throw ServerError.invalidFixture }
        sec_protocol_options_set_local_identity(tls.securityProtocolOptions, protocolIdentity)
        sec_protocol_options_set_min_tls_protocol_version(tls.securityProtocolOptions, .TLSv12)
        let parameters = NWParameters(tls: tls, tcp: NWProtocolTCP.Options())
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
        let listener = try NWListener(using: parameters)
        self.listener = listener
        let ready = DispatchSemaphore(value: 0)
        var startError: Error?
        listener.stateUpdateHandler = { state in
            switch state {
            case .ready: ready.signal()
            case .failed(let error): startError = error; ready.signal()
            default: break
            }
        }
        listener.newConnectionHandler = { [weak self] connection in self?.accept(connection) }
        listener.start(queue: queue)
        guard ready.wait(timeout: .now() + 2) == .success,
              startError == nil, let port = listener.port?.rawValue, port != 0 else {
            listener.cancel()
            throw startError ?? ServerError.listenerFailed
        }
        return port
    }

    func stop() {
        queue.sync {
            connection?.cancel()
            listener?.cancel()
            connection = nil
            listener = nil
        }
    }

    private func accept(_ connection: NWConnection) {
        guard self.connection == nil else { connection.cancel(); return }
        self.connection = connection
        connection.stateUpdateHandler = { [weak self, weak connection] state in
            guard let self, let connection else { return }
            if case .ready = state {
                do {
                    try self.sendHandshake(on: connection)
                    self.receive(on: connection)
                } catch { connection.cancel() }
            }
        }
        connection.start(queue: queue)
    }

    private func sendHandshake(on connection: NWConnection) throws {
        let hello = HelloPayload(role: .agent, capabilities: [.input], nonce: Self.agentNonce)
        let challenge = AuthChallengePayload(challenge: Self.challenge, agentIdentifier: Self.agentIdentifier)
        let bytes = try FrameCodec.encode(Frame(type: .hello, sequence: 1, payload: hello.encode()))
            + FrameCodec.encode(Frame(type: .authChallenge, sequence: 2, payload: challenge.encode()))
        send(bytes, on: connection)
    }

    private func receive(on connection: NWConnection) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 4096) { [weak self, weak connection] data, _, eof, error in
            guard let self, let connection else { return }
            do {
                for frame in try self.decoder.append(data ?? Data()) { try self.handle(frame, on: connection) }
                if !eof && error == nil { self.receive(on: connection) }
            } catch { connection.cancel() }
        }
    }

    private func handle(_ frame: Frame, on connection: NWConnection) throws {
        if controllerHello == nil {
            guard frame.type == .hello, frame.sequence == 1 else { throw ServerError.invalidProtocol }
            let hello = try HelloPayload.decode(frame.payload)
            guard hello.role == .controller, hello.capabilities.contains(.input) else { throw ServerError.invalidProtocol }
            controllerHello = hello
            return
        }
        if !authenticated {
            guard frame.type == .authResponse, frame.sequence == 2, let controllerHello else {
                throw ServerError.invalidProtocol
            }
            let expected = try Authentication.response(deviceKey: deviceKey,
                controllerNonce: controllerHello.nonce, agentNonce: Self.agentNonce,
                challenge: Self.challenge, agentIdentifier: Self.agentIdentifier)
            let accepted = Authentication.constantTimeEquals(frame.payload, expected)
            authenticated = accepted
            let result = AuthResultPayload(status: accepted ? .success : .rejected,
                retryDelayMilliseconds: accepted ? 0 : 1_000)
            var bytes = try FrameCodec.encode(Frame(type: .authResult, sequence: 3, payload: result.encode()))
            if accepted && closeWithPartialFrame { bytes.append(0x50) }
            send(bytes, on: connection) { [weak connection] in
                if self.closeWithPartialFrame || !accepted { connection?.cancel() }
            }
            return
        }
        guard frame.sequence >= 3 else { throw ServerError.invalidProtocol }
        receivedInput.append(frame)
        if frame.type == .disconnect {
            let frames = receivedInput
            onFinished?(frames)
        }
    }

    private func send(_ data: Data, on connection: NWConnection, completion: (() -> Void)? = nil) {
        connection.send(content: data, completion: .contentProcessed { _ in completion?() })
    }

    private static let agentNonce = Data(repeating: 0x11, count: 32)
    private static let challenge = Data(repeating: 0x22, count: 32)
    private static let agentIdentifier = Data(repeating: 0x33, count: 16)

    // Self-signed identity generated solely for loopback XCTest. It is not trusted,
    // loaded, or referenced by any production target. Password: "test-only".
    private static let legacyPKCS12Base64 = [
        "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",
    ].joined()
}
