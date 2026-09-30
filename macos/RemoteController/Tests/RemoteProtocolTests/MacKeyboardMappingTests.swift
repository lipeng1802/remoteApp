import Foundation
import XCTest
@testable import RemoteProtocol

final class MacKeyboardMappingTests: XCTestCase {
    private func fixture() throws -> [String: Any] {
        var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        for _ in 0..<4 { root.deleteLastPathComponent() }
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf:
            root.appendingPathComponent("protocol/testdata/mac-keymap-v1.json"))) as? [String: Any])
    }
    private func hex(_ data: Data?) -> String? { data?.map { String(format: "%02x", $0) }.joined() }
    func testPhysicalMappingSharedVectors() throws {
        for v in try XCTUnwrap(fixture()["vectors"] as? [[String: Any]]) {
            let code = UInt16(try XCTUnwrap(v["macKeyCode"] as? Int))
            XCTAssertEqual(hex(MacKeyboardMapper.event(keyCode: code, action: .down)?.encode()), v["downHex"] as? String)
            XCTAssertEqual(hex(MacKeyboardMapper.event(keyCode: code, action: .up)?.encode()), v["upHex"] as? String)
        }
    }
    func testUnsupportedKeysProduceNoEvent() throws {
        for code in try XCTUnwrap(fixture()["unsupported"] as? [Int]) {
            XCTAssertNil(MacKeyboardMapper.event(keyCode: UInt16(code), action: .down))
            XCTAssertNil(MacKeyboardMapper.event(keyCode: UInt16(code), action: .up))
        }
    }
    func testModifierSidesAndDuplicateSnapshots() throws {
        var tracker = MacModifierTracker()
        let left = tracker.update(pressedKeyCodes: [59])
        XCTAssertEqual(left.count, 1)
        XCTAssertEqual(left.first?.extended, false)
        XCTAssertTrue(tracker.update(pressedKeyCodes: [59]).isEmpty)
        let right = tracker.update(pressedKeyCodes: [59, 62])
        XCTAssertEqual(right.count, 1)
        XCTAssertEqual(right.first?.extended, true)
        let releaseLeft = tracker.update(pressedKeyCodes: [62])
        XCTAssertEqual(releaseLeft.first?.action, .up)
        XCTAssertEqual(releaseLeft.first?.extended, false)
        XCTAssertEqual(tracker.releaseAll().first?.extended, true)
        XCTAssertTrue(tracker.releaseAll().isEmpty)
    }
    func testModifierChangeReleasesBeforePressAndIgnoresOrdinaryKeys() {
        var tracker = MacModifierTracker()
        _ = tracker.update(pressedKeyCodes: [56])
        let changes = tracker.update(pressedKeyCodes: [60, 0, 57, 65535])
        XCTAssertEqual(changes.count, 2)
        XCTAssertEqual(changes.map { $0.action }, [.up, .down])
        XCTAssertEqual(changes.map { $0.scanCode }, [0x2a, 0x36])
    }
    func testFocusLossReleasesAllAndStartsFresh() {
        var tracker = MacModifierTracker()
        let down = tracker.update(pressedKeyCodes: [54,55,56,58,59,60,61,62])
        let up = tracker.releaseAll()
        XCTAssertEqual(down.count, 8)
        XCTAssertEqual(up.count, 8)
        XCTAssertTrue(up.allSatisfy { $0.action == .up })
        XCTAssertTrue(tracker.releaseAll().isEmpty)
        XCTAssertEqual(tracker.update(pressedKeyCodes: [55]).first?.scanCode, 0x5b)
    }

    func testModifierEventsTrackBothSidesWhenAggregateFlagStaysSet() {
        var state = MacModifierEventState()
        XCTAssertEqual(state.update(keyCode: 56, aggregatePressed: true), [56])
        XCTAssertEqual(state.update(keyCode: 60, aggregatePressed: true), [56, 60])
        // Releasing one Shift still leaves the aggregate Shift flag set.
        XCTAssertEqual(state.update(keyCode: 56, aggregatePressed: true), [60])
        XCTAssertEqual(state.update(keyCode: 60, aggregatePressed: false), [])
    }

    func testModifierEventResetAndUntrackedRelease() {
        var state = MacModifierEventState()
        XCTAssertEqual(state.update(keyCode: 59, aggregatePressed: true), [59])
        state.reset()
        XCTAssertEqual(state.update(keyCode: 59, aggregatePressed: false), [])
        XCTAssertEqual(state.update(keyCode: 0, aggregatePressed: true), [])
    }
}
