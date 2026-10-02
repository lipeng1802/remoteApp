import Foundation
import XCTest
@testable import RemoteProtocol

final class InputSendQueueTests: XCTestCase {
    private func move(_ x: UInt16) -> CapturedInput { .move(MouseMovePayload(x: x, y: 0)) }
    private func wheel(_ y: Int32, horizontal x: Int32 = 0) -> CapturedInput {
        .wheel(MouseWheelPayload(horizontal: x, vertical: y))
    }
    private let down = CapturedInput.button(MouseButtonPayload(button: .left, action: .down))
    private let up = CapturedInput.button(MouseButtonPayload(button: .left, action: .up))

    func testTenThousandMovesKeepOnlyLatest() throws {
        var queue = try InputSendQueue()
        for x in 0..<10000 { try queue.append([move(UInt16(x))]) }
        XCTAssertEqual(queue.count, 1)
        XCTAssertEqual(queue.take()?.payload, move(9999).payload)
        XCTAssertNil(queue.take())
    }

    func testTenThousandTouchpadWheelBatchesStayBounded() throws {
        var queue = try InputSendQueue()
        for index in 0..<10_000 {
            try queue.append([move(UInt16(index)), wheel(1)])
        }
        XCTAssertEqual(queue.count, 2)
        XCTAssertEqual(queue.take()?.payload, move(0).payload)
        let combined = try MouseWheelPayload.decode(XCTUnwrap(queue.take()).payload)
        XCTAssertEqual(combined, MouseWheelPayload(horizontal: 0, vertical: 10_000))
    }

    func testWheelCoalescingSaturatesAndNeverCrossesControlBarrier() throws {
        var queue = try InputSendQueue()
        try queue.append([move(1), wheel(Int32.max)])
        try queue.append([move(2), wheel(1)])
        try queue.append([down])
        try queue.append([move(3), wheel(-1)])
        try queue.append([move(4), wheel(Int32.min)])

        XCTAssertEqual(queue.take()?.payload, move(1).payload)
        XCTAssertEqual(try MouseWheelPayload.decode(XCTUnwrap(queue.take()).payload).vertical, Int32.max)
        XCTAssertEqual(queue.take()?.type, .mouseButton)
        XCTAssertEqual(queue.take()?.payload, move(3).payload)
        XCTAssertEqual(try MouseWheelPayload.decode(XCTUnwrap(queue.take()).payload).vertical, Int32.min)
        XCTAssertNil(queue.take())
    }

    func testMovesNeverCrossButtonsKeysOrWheel() throws {
        var queue = try InputSendQueue()
        let key = CapturedInput.key(try KeyEventPayload(scanCode: 30, extended: false, action: .down))
        let wheel = CapturedInput.wheel(MouseWheelPayload(horizontal: 0, vertical: 120))
        try queue.append([move(1), move(2), down, move(3), key, move(4), wheel, move(5), up])
        let expected = [move(2), down, move(3), key, move(4), wheel, move(5), up]
        XCTAssertEqual(queue.count, expected.count)
        for input in expected {
            XCTAssertEqual(queue.take(), QueuedInputMessage(type: input.messageType, payload: input.payload))
        }
    }

    func testOverflowDiscardsWholePendingBatchAndStops() throws {
        var queue = try InputSendQueue(capacity: 2)
        XCTAssertThrowsError(try queue.append([down, move(1), up])) {
            XCTAssertEqual($0 as? InputSendError, .queueOverflow)
        }
        XCTAssertEqual(queue.count, 0)
        XCTAssertTrue(queue.isStopped)
        XCTAssertNil(queue.take())
        XCTAssertThrowsError(try queue.append([up]))
    }

    func testFullQueueCanReplaceTailMoveButNotCrossBarrier() throws {
        var queue = try InputSendQueue(capacity: 2)
        try queue.append([down, move(1), move(2)])
        XCTAssertEqual(queue.count, 2)
        XCTAssertEqual(queue.take()?.type, .mouseButton)
        XCTAssertEqual(queue.take()?.payload, move(2).payload)
        try queue.append([move(3), down])
        XCTAssertThrowsError(try queue.append([move(4)]))
    }

    func testInFlightMoveIsNotReplaced() throws {
        var queue = try InputSendQueue()
        try queue.append([move(1)])
        let inFlight = queue.take()
        try queue.append([move(2), move(3)])
        XCTAssertEqual(inFlight?.payload, move(1).payload)
        XCTAssertEqual(queue.take()?.payload, move(3).payload)
    }

    func testControlsAreOrderingBarriersAndCountTowardCapacity() throws {
        var queue = try InputSendQueue(capacity: 3)
        try queue.append([move(1)])
        let ping = QueuedInputMessage(type: .ping, payload: Data(count: 8))
        try queue.append(ping)
        try queue.append([move(2)])
        XCTAssertEqual(queue.count, 3)
        XCTAssertEqual(queue.take()?.payload, move(1).payload)
        XCTAssertEqual(queue.take(), ping)
        XCTAssertEqual(queue.take()?.payload, move(2).payload)
    }

    func testConfigurationAndOversizeAreRejected() throws {
        for size in [0, 1, 1025] { XCTAssertThrowsError(try InputSendQueue(capacity: size)) }
        var queue = try InputSendQueue()
        XCTAssertThrowsError(try queue.append(QueuedInputMessage(type: .hello, payload: Data(count: 65))))
        XCTAssertTrue(queue.isStopped)
        XCTAssertEqual(queue.count, 0)
    }

    func testClipboardSetUsesBoundedTextPayloadLimit() throws {
        var queue = try InputSendQueue()
        let maximum = ClipboardTextPayload(status: .success,
            text: String(repeating: "a", count: ClipboardTextPayload.maximumTextBytes))
        try queue.append([.clipboardSet(maximum)])
        XCTAssertEqual(queue.take()?.payload.count, ClipboardTextPayload.maximumTextBytes + 1)
        XCTAssertThrowsError(try queue.append(QueuedInputMessage(type: .clipboardSetText,
            payload: Data(count: ClipboardTextPayload.maximumTextBytes + 2))))
        XCTAssertTrue(queue.isStopped)
    }

    func testStopIsTerminalAndIdempotent() throws {
        var queue = try InputSendQueue()
        try queue.append([down])
        queue.stop()
        queue.stop()
        XCTAssertNil(queue.take())
        XCTAssertThrowsError(try queue.append([]))
    }

    func testSharedMoveCoalescingVector() throws {
        let fixture = try InputQueueFixture.load()
        var queue = try InputSendQueue(capacity: fixture.capacity)
        try queue.append(fixture.inputs.map { try $0.captured() })
        XCTAssertEqual(queue.count, fixture.expected.count)
        for expected in fixture.expected {
            let message = try XCTUnwrap(queue.take())
            XCTAssertEqual(String(describing: message.type), expected.type)
            XCTAssertEqual(message.payload, expected.payload)
        }
    }
}
