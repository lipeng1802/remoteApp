import Foundation
import XCTest
@testable import RemoteProtocol

final class KeyboardInputTests: XCTestCase {
    private func vectors() throws -> [String: Any] {
        var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        for _ in 0..<4 { root.deleteLastPathComponent() }
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf:
            root.appendingPathComponent("protocol/testdata/keyboard-v1.json"))) as? [String: Any])
    }
    private func bytes(_ text: String) throws -> Data {
        let chars = Array(text)
        return try Data(stride(from: 0, to: chars.count, by: 2).map {
            try XCTUnwrap(UInt8(String(chars[$0...$0 + 1]), radix: 16))
        })
    }
    func testKeyboardGoldenVectors() throws {
        for vector in try XCTUnwrap(vectors()["valid"] as? [[String: Any]]) {
            let data = try bytes(XCTUnwrap(vector["hex"] as? String))
            let key = try KeyEventPayload.decode(data)
            XCTAssertEqual(Int(key.scanCode), vector["scanCode"] as? Int)
            XCTAssertEqual(key.extended, vector["extended"] as? Bool)
            XCTAssertEqual(Int(key.action.rawValue), vector["action"] as? Int)
            XCTAssertEqual(key.encode(), data)
        }
    }
    func testMalformedKeyboardVectors() throws {
        for hex in try XCTUnwrap(vectors()["invalidHex"] as? [String]) {
            let data = try bytes(hex)
            XCTAssertThrowsError(try KeyEventPayload.decode(data))
        }
    }
    func testKeyboardConstructionAndExtendedIdentity() throws {
        for code in [UInt16(0), 0x80, 0xe01d, UInt16.max] {
            XCTAssertThrowsError(try KeyEventPayload(scanCode: code, extended: false, action: .down))
        }
        let left = try KeyEventPayload(scanCode: 0x1d, extended: false, action: .down)
        let right = try KeyEventPayload(scanCode: 0x1d, extended: true, action: .down)
        XCTAssertNotEqual(left, right)
        XCTAssertNotEqual(left.encode(), right.encode())
    }
}
