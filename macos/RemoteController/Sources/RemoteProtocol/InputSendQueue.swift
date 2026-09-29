import Foundation

enum InputSendError: Error, Equatable {
    case invalidConfiguration, queueOverflow, stopped, invalidState, invalidClock
    case authenticationTimeout, writeTimeout, heartbeatTimeout, closingTimeout
    case authenticationRejected
}

struct QueuedInputMessage: Equatable {
    let type: MessageType
    let payload: Data
}

/// Serial-owner only. Bound counts include controls, but exclude the one in-flight
/// frame owned by AuthenticatedInputSender. Only adjacent unsent moves coalesce.
struct InputSendQueue {
    let capacity: Int
    private var messages: [QueuedInputMessage] = []
    private(set) var isStopped = false
    var count: Int { messages.count }

    init(capacity: Int = 64) throws {
        guard (2...1024).contains(capacity) else { throw InputSendError.invalidConfiguration }
        self.capacity = capacity
    }

    mutating func append(_ inputs: [CapturedInput]) throws {
        guard !isStopped else { throw InputSendError.stopped }
        for input in inputs {
            try append(QueuedInputMessage(type: input.messageType, payload: input.payload))
        }
    }

    mutating func append(_ message: QueuedInputMessage) throws {
        guard !isStopped else { throw InputSendError.stopped }
        guard message.payload.count <= 64 else {
            stop()
            throw ProtocolError.messageTooLarge(message.payload.count)
        }
        if message.type == .mouseMove, messages.last?.type == .mouseMove {
            messages[messages.count - 1] = message
            return
        }
        guard messages.count < capacity else {
            // Fail closed. Never continue after silently dropping a key/button Up.
            stop()
            throw InputSendError.queueOverflow
        }
        messages.append(message)
    }

    mutating func take() -> QueuedInputMessage? {
        guard !isStopped, !messages.isEmpty else { return nil }
        return messages.removeFirst() // At most capacity entries.
    }

    mutating func stop() {
        messages.removeAll()
        isStopped = true
    }
}
