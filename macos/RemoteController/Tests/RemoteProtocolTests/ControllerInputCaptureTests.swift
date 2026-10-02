import Foundation
import XCTest
@testable import RemoteProtocol

final class ControllerInputCaptureTests: XCTestCase {
    private let center = MouseMovePayload(x: 32768, y: 32768)

    func testInactiveCaptureRejectsEveryEvent() {
        var capture = ControllerInputCapture()
        XCTAssertTrue(capture.key(keyCode: 0, action: .down).isEmpty)
        XCTAssertTrue(capture.modifierSnapshot([59]).isEmpty)
        XCTAssertTrue(capture.move(to: center).isEmpty)
        XCTAssertTrue(capture.button(.left, action: .down, at: center).isEmpty)
        XCTAssertTrue(capture.wheel(horizontal: 0, vertical: 120, at: center).isEmpty)
        XCTAssertEqual(capture.heldCount, 0)
    }

    func testRepeatAndUnmatchedUpDoNotCreateStuckKeys() {
        var capture = ControllerInputCapture()
        capture.start()
        XCTAssertTrue(capture.key(keyCode: 0, action: .down, isRepeat: true).isEmpty)
        XCTAssertTrue(capture.key(keyCode: 0, action: .up).isEmpty)
        XCTAssertEqual(capture.key(keyCode: 0, action: .down).count, 1)
        XCTAssertTrue(capture.key(keyCode: 0, action: .down).isEmpty)
        XCTAssertEqual(capture.key(keyCode: 0, action: .down, isRepeat: true).count, 1)
        XCTAssertEqual(capture.heldCount, 1)
        XCTAssertEqual(capture.key(keyCode: 0, action: .up).count, 1)
        XCTAssertEqual(capture.heldCount, 0)
    }

    func testModifiersUseSnapshotsAndPreserveSides() throws {
        var capture = ControllerInputCapture()
        capture.start()
        XCTAssertTrue(capture.key(keyCode: 59, action: .down).isEmpty)
        XCTAssertEqual(capture.modifierSnapshot([59, 62]).count, 2)
        let released = try KeyEventPayload.decode(XCTUnwrap(capture.modifierSnapshot([62]).first).payload)
        XCTAssertFalse(released.extended)
        XCTAssertEqual(released.action, .up)
        XCTAssertEqual(capture.heldCount, 1)
        XCTAssertTrue(try KeyEventPayload.decode(XCTUnwrap(capture.stop().first).payload).extended)
    }

    func testCommandCIsIsolatedCtrlCClipboardRequestAndRestoresModifiers() throws {
        var capture = ControllerInputCapture()
        capture.start()
        _ = capture.modifierSnapshot([55, 56])
        let commands = capture.clipboardCopyShortcut()
        XCTAssertEqual(commands.map(\.messageType), [
            .keyEvent, .keyEvent,
            .keyEvent, .keyEvent, .keyEvent, .keyEvent,
            .clipboardRequest,
            .keyEvent, .keyEvent,
        ])
        let keys = try commands.compactMap { command -> KeyEventPayload? in
            guard case .key = command else { return nil }
            return try KeyEventPayload.decode(command.payload)
        }
        XCTAssertEqual(keys.map(\.action), [.up, .up, .down, .down, .up, .up, .down, .down])
        XCTAssertEqual(Array(keys[2...5].map(\.scanCode)), [0x1d, 0x2e, 0x2e, 0x1d])
        XCTAssertEqual(capture.heldCount, 2)
        XCTAssertTrue(capture.clipboardCopyShortcut().contains(.clipboardRequest))
        _ = capture.modifierSnapshot([])
        XCTAssertTrue(capture.clipboardCopyShortcut().isEmpty)
    }

    func testBlackBarsRejectDownButAllowTrackedRelease() {
        var capture = ControllerInputCapture()
        capture.start()
        XCTAssertTrue(capture.button(.left, action: .down, at: nil).isEmpty)
        XCTAssertEqual(capture.button(.left, action: .down, at: center),
                       [.move(center), .button(MouseButtonPayload(button: .left, action: .down))])
        XCTAssertTrue(capture.button(.left, action: .down, at: center).isEmpty)
        XCTAssertEqual(capture.button(.left, action: .up, at: nil),
                       [.button(MouseButtonPayload(button: .left, action: .up))])
        XCTAssertTrue(capture.button(.left, action: .up, at: nil).isEmpty)
        XCTAssertEqual(capture.heldCount, 0)
    }

    func testStopReleasesMixedInputsOnceAndRequiresRestart() throws {
        var capture = ControllerInputCapture()
        capture.start()
        _ = capture.key(keyCode: 0, action: .down)
        _ = capture.modifierSnapshot([59, 62])
        _ = capture.button(.left, action: .down, at: center)
        _ = capture.button(.right, action: .down, at: center)
        capture.start() // Does not reset tracking.
        XCTAssertEqual(capture.heldCount, 5)
        let released = capture.stop()
        XCTAssertEqual(released.count, 5)
        for command in released {
            switch command {
            case .key(let key): XCTAssertEqual(key.action, .up)
            case .button(let button): XCTAssertEqual(button.action, .up)
            default: XCTFail("stop must only release held input")
            }
        }
        XCTAssertEqual(capture.heldCount, 0)
        XCTAssertFalse(capture.isActive)
        XCTAssertTrue(capture.stop().isEmpty)
        XCTAssertTrue(capture.key(keyCode: 0, action: .down).isEmpty)
        capture.start()
        XCTAssertTrue(capture.key(keyCode: 0, action: .down, isRepeat: true).isEmpty)
        XCTAssertEqual(capture.key(keyCode: 0, action: .down).count, 1)
    }

    func testWheelFractionsBoundsAndInvalidInput() throws {
        var capture = ControllerInputCapture()
        capture.start()
        XCTAssertTrue(capture.wheel(horizontal: 0.4, vertical: -0.4, at: center).isEmpty)
        let fractional = capture.wheel(horizontal: 0.7, vertical: -0.7, at: center)
        XCTAssertEqual(fractional, [.move(center), .wheel(MouseWheelPayload(horizontal: 1, vertical: -1))])
        XCTAssertTrue(capture.wheel(horizontal: .nan, vertical: 1, at: center).isEmpty)
        XCTAssertTrue(capture.wheel(horizontal: 1, vertical: .infinity, at: center).isEmpty)
        XCTAssertTrue(capture.wheel(horizontal: 20, vertical: 20, at: nil).isEmpty)
        let bounded = capture.wheel(horizontal: 1e100, vertical: -1e100, at: center)
        XCTAssertEqual(bounded.last, .wheel(MouseWheelPayload(horizontal: 1200, vertical: -1200)))
    }

    func testStopDropsFractionalScrollAcrossSessions() {
        var capture = ControllerInputCapture()
        capture.start()
        _ = capture.wheel(horizontal: 0, vertical: 0.75, at: center)
        _ = capture.stop()
        capture.start()
        XCTAssertTrue(capture.wheel(horizontal: 0, vertical: 0.5, at: center).isEmpty)
    }

    func testCapturedCommandsRoundTripThroughExistingWireCodec() throws {
        var capture = ControllerInputCapture()
        capture.start()
        var commands = capture.modifierSnapshot([59])
        commands += capture.key(keyCode: 8, action: .down)
        commands += capture.move(to: center)
        commands += capture.button(.middle, action: .down, at: center)
        commands += capture.wheel(horizontal: -120, vertical: 120, at: center)
        commands += capture.stop()
        var decoder = FrameDecoder()
        for (index, command) in commands.enumerated() {
            let frame = Frame(type: command.messageType, sequence: UInt32(index + 1), payload: command.payload)
            XCTAssertEqual(try decoder.append(FrameCodec.encode(frame)), [frame])
        }
        try decoder.finish()
    }

    func testUnsupportedKeysAndInvalidPointAreIgnored() {
        var capture = ControllerInputCapture()
        capture.start()
        XCTAssertTrue(capture.key(keyCode: 65535, action: .down).isEmpty)
        XCTAssertTrue(capture.key(keyCode: 57, action: .down).isEmpty)
        XCTAssertTrue(capture.modifierSnapshot([0, 57, 65535]).isEmpty)
        XCTAssertTrue(capture.move(to: nil).isEmpty)
        XCTAssertEqual(capture.heldCount, 0)
    }

    func testFocusLossMatchesSharedCaptureSequence() throws {
        var capture = ControllerInputCapture()
        capture.start()
        var commands = capture.modifierSnapshot([59, 62])
        commands += capture.key(keyCode: 0, action: .down)
        commands += capture.key(keyCode: 0, action: .down, isRepeat: true)
        commands += capture.button(.left, action: .down, at: center)
        commands += capture.wheel(horizontal: 0, vertical: -120, at: center)
        commands += capture.stop()
        var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        for _ in 0..<4 { root.deleteLastPathComponent() }
        struct Manifest: Decodable {
            struct Event: Decodable { let type: String; let payloadHex: String }
            let events: [Event]
        }
        let manifest = try JSONDecoder().decode(Manifest.self, from: Data(contentsOf:
            root.appendingPathComponent("protocol/testdata/controller-input-v1.json")))
        let mock = try SyntheticInputMockVector.make()
        XCTAssertEqual(mock.inputs + mock.releases, commands)
        XCTAssertEqual(commands.count, manifest.events.count)
        for (command, expected) in zip(commands, manifest.events) {
            XCTAssertEqual(String(describing: command.messageType), expected.type)
            XCTAssertEqual(command.payload.map { String(format: "%02x", $0) }.joined(), expected.payloadHex)
        }
        XCTAssertEqual(capture.heldCount, 0)
    }
}
