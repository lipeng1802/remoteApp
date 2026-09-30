import Foundation

public enum InputSimulationError: Error, Equatable {
    case cancelled, connectionFailed, protocolFailure, timedOut, congested
}

// Test seam: callbacks may be delayed or synchronous. No desktop input here.
enum InputTransportState { case ready, failed }
protocol InputTransport: AnyObject {
    func start(queue: DispatchQueue, state: @escaping (InputTransportState) -> Void)
    func send(_ bytes: Data, completion: @escaping (Bool) -> Void)
    func receive(completion: @escaping (Data?, Bool, Bool) -> Void)
    func cancel()
}

final class InputConnectionDriver {
    private let queue: DispatchQueue
    private let queueKey = DispatchSpecificKey<UInt8>()
    private let transport: InputTransport
    private let clock: () -> Double
    private let callbackQueue: DispatchQueue
    private let automaticTimer: Bool
    private var sender: AuthenticatedInputSender
    private var decoder = ProbeFrameDecoder()
    private var timer: DispatchSourceTimer?
    private var started = false
    private var ready = false
    private var ended = false
    private var reading = false
    private var announcedAuthentication = false
    private let receivesJpeg: Bool
    private var onJpegFrame: ((AuthenticatedInputSender.ReceivedJpegFrame) -> Void)?
    private let connectDeadline: Double
    private var onAuthenticated: (() -> Void)?
    private var completion: ((Result<Void, InputSimulationError>) -> Void)?

    init(transport: InputTransport, deviceKey: Data, localControlAllowed: Bool,
         queue: DispatchQueue = DispatchQueue(label: "prd.input.mock"),
         callbackQueue: DispatchQueue = .main, automaticTimer: Bool = true,
         clock: @escaping () -> Double = { ProcessInfo.processInfo.systemUptime },
         connectTimeout: Double = 15,
         onJpegFrame: ((AuthenticatedInputSender.ReceivedJpegFrame) -> Void)? = nil,
         onAuthenticated: @escaping () -> Void,
         completion: @escaping (Result<Void, InputSimulationError>) -> Void) throws {
        guard connectTimeout.isFinite, connectTimeout > 0, connectTimeout <= 300 else {
            throw InputSendError.invalidConfiguration
        }
        self.transport = transport
        self.queue = queue
        self.clock = clock
        self.callbackQueue = callbackQueue
        self.automaticTimer = automaticTimer
        let shouldReceiveJpeg = onJpegFrame != nil
        receivesJpeg = shouldReceiveJpeg
        self.onJpegFrame = onJpegFrame
        self.onAuthenticated = onAuthenticated
        self.completion = completion
        let now = clock()
        sender = try AuthenticatedInputSender(deviceKey: deviceKey,
            localControlAllowed: localControlAllowed, acceptsJpeg: shouldReceiveJpeg, now: now)
        decoder.allowJpeg = shouldReceiveJpeg
        connectDeadline = now + connectTimeout
        queue.setSpecific(key: queueKey, value: 1)
    }

    private func own<T>(_ body: () -> T) -> T {
        if DispatchQueue.getSpecific(key: queueKey) != nil { return body() }
        return queue.sync(execute: body)
    }

    func start() {
        own {
            guard !started, !ended else { return }
            started = true
            if automaticTimer {
                let source = DispatchSource.makeTimerSource(queue: queue)
                source.schedule(deadline: .now(), repeating: .milliseconds(20))
                // Keep the session alive until completion, including a stalled connect.
                source.setEventHandler { [self] in tickOwned() }
                timer = source
                source.resume()
            }
            transport.start(queue: queue) { [weak self] state in
                guard let self else { return }
                self.own {
                    guard !self.ended else { return }
                    switch state {
                    case .ready:
                        guard !self.ready else { return }
                        self.ready = true
                        self.tickOwned() // Submit HELLO before processing the peer HELLO.
                        self.readNext()
                    case .failed: self.end(.failure(.connectionFailed))
                    }
                }
            }
        }
    }

    /// Synchronous, bounded admission. Does not queue a closure per mouse event.
    /// Call only after onAuthenticated; a rejected batch terminates this session.
    @discardableResult
    func submit(_ inputs: [CapturedInput]) -> Bool {
        own {
            guard started, !ended else { return false }
            guard inputs.count <= 256 else { end(.failure(.congested)); return false }
            do {
                try sender.enqueue(inputs, now: clock())
                return true
            } catch { fail(error); return false }
        }
    }

    func finish(releases: [CapturedInput]) {
        own {
            guard !ended else { return }
            guard releases.count <= 256 else { end(.failure(.congested)); return }
            do { try sender.finish(releases: releases, now: clock()) }
            catch { fail(error) }
        }
    }

    func cancel() { own { end(.failure(.cancelled)) } }
    func tick() { own { tickOwned() } } // Deterministic clock/transport testing.
    var isEnded: Bool { own { ended } }
    var pendingCount: Int { own { sender.pendingCount } }

    private func tickOwned() {
        guard started, !ended else { return }
        guard ready else {
            if clock() >= connectDeadline { end(.failure(.timedOut)) }
            return
        }
        do {
            guard let frame = try sender.poll(now: clock()) else { return }
            let bytes = try FrameCodec.encode(frame)
            transport.send(bytes) { [weak self] success in
                guard let self else { return }
                self.own {
                    guard !self.ended else { return }
                    guard success else { self.end(.failure(.connectionFailed)); return }
                    do {
                        try self.sender.didWrite(sequence: frame.sequence, now: self.clock())
                        if self.sender.state == .finished { self.end(.success(())) }
                    } catch { self.fail(error) }
                }
            }
        } catch { fail(error) }
    }

    private func readNext() {
        guard ready, !ended, !reading else { return }
        reading = true
        transport.receive { [weak self] data, eof, success in
            guard let self else { return }
            self.own {
                guard !self.ended else { return }
                self.reading = false
                guard success else { self.end(.failure(.connectionFailed)); return }
                do {
                    // Bound each adapter callback even though a JPEG may span many callbacks.
                    let callbackLimit = self.receivesJpeg ? 65_536 : 4_096
                    guard (data?.count ?? 0) <= callbackLimit else {
                        throw ProtocolError.messageTooLarge(data?.count ?? 0)
                    }
                    for frame in try self.decoder.append(data ?? Data()) {
                        if let video = try self.sender.receive(frame, now: self.clock()) {
                            self.onJpegFrame?(video)
                        }
                        if self.sender.state == .failed { self.end(.failure(.protocolFailure)); return }
                    }
                    if self.sender.state == .active, !self.announcedAuthentication {
                        self.announcedAuthentication = true
                        self.callbackQueue.async { [weak self] in
                            guard let self else { return }
                            let callback = self.own { self.ended ? nil : self.onAuthenticated }
                            callback?()
                        }
                    }
                    if eof { self.end(.failure(.connectionFailed)); return }
                    // One receive request at a time; defer to avoid recursion with synchronous fakes.
                    self.queue.async { [weak self] in self?.readNext() }
                } catch { self.fail(error) }
            }
        }
    }

    private func fail(_ error: Error) {
        switch error as? InputSendError {
        case .queueOverflow: end(.failure(.congested))
        case .authenticationTimeout, .writeTimeout, .heartbeatTimeout, .closingTimeout:
            end(.failure(.timedOut))
        default: end(.failure(.protocolFailure))
        }
    }

    private func end(_ result: Result<Void, InputSimulationError>) {
        guard !ended else { return }
        ended = true // Set before cancellation can synchronously reenter.
        sender.abort()
        timer?.setEventHandler {}
        timer?.cancel()
        timer = nil
        transport.cancel()
        let callback = completion
        completion = nil
        onAuthenticated = nil
        onJpegFrame = nil
        callbackQueue.async { callback?(result) }
    }
}
