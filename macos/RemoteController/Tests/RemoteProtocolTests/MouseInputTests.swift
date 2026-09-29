import Foundation
import XCTest
@testable import RemoteProtocol

final class MouseInputTests: XCTestCase {
    private func vectors() throws -> [String: Any] {
        var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        for _ in 0..<4 { root.deleteLastPathComponent() }
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf:
            root.appendingPathComponent("protocol/testdata/mouse-v1.json"))) as? [String: Any])
    }
    private func bytes(_ text: String) throws -> Data {
        let chars = Array(text)
        return try Data(stride(from: 0, to: chars.count, by: 2).map {
            try XCTUnwrap(UInt8(String(chars[$0...$0 + 1]), radix: 16))
        })
    }
    func testGoldenPayloads() throws {
        let all = try vectors()
        for v in try XCTUnwrap(all["move"] as? [[String: Any]]) {
            let data = try bytes(XCTUnwrap(v["hex"] as? String))
            let decoded = try MouseMovePayload.decode(data)
            XCTAssertEqual(Int(decoded.x), v["x"] as? Int)
            XCTAssertEqual(Int(decoded.y), v["y"] as? Int)
            XCTAssertEqual(decoded.encode(), data)
        }
        for v in try XCTUnwrap(all["button"] as? [[String: Any]]) {
            let data = try bytes(XCTUnwrap(v["hex"] as? String))
            let decoded = try MouseButtonPayload.decode(data)
            XCTAssertEqual(Int(decoded.button.rawValue), v["button"] as? Int)
            XCTAssertEqual(Int(decoded.action.rawValue), v["action"] as? Int)
            XCTAssertEqual(decoded.encode(), data)
        }
        for v in try XCTUnwrap(all["wheel"] as? [[String: Any]]) {
            let data = try bytes(XCTUnwrap(v["hex"] as? String))
            let decoded = try MouseWheelPayload.decode(data)
            XCTAssertEqual(Int(decoded.horizontal), v["horizontal"] as? Int)
            XCTAssertEqual(Int(decoded.vertical), v["vertical"] as? Int)
            XCTAssertEqual(decoded.encode(), data)
        }
    }
    func testSharedCoordinateVectors() throws {
        for v in try XCTUnwrap(vectors()["mapping"] as? [[String: Any]]) {
            func number(_ key: String) throws -> Double { try XCTUnwrap(v[key] as? NSNumber).doubleValue }
            let point = try MouseCoordinates.map(x: number("x"), y: number("y"),
                viewWidth: number("viewWidth"), viewHeight: number("viewHeight"),
                screenWidth: Int(number("screenWidth")), screenHeight: Int(number("screenHeight")))
            if let hex = v["hex"] as? String { XCTAssertEqual(point?.encode(), try bytes(hex)) }
            else { XCTAssertNil(point) }
        }
    }
    func testMalformedMousePayloads() {
        for count in [0, 3, 5] { XCTAssertThrowsError(try MouseMovePayload.decode(Data(count: count))) }
        for count in [0, 7, 9] { XCTAssertThrowsError(try MouseWheelPayload.decode(Data(count: count))) }
        for data in [Data(), Data([1]), Data([1, 1, 0]), Data([0, 1]), Data([4, 1]), Data([1, 0]), Data([1, 3])] {
            XCTAssertThrowsError(try MouseButtonPayload.decode(data))
        }
    }
    func testInvalidGeometryRejected() {
        XCTAssertNil(MouseCoordinates.map(x: .nan, y: 1, viewWidth: 800, viewHeight: 800, screenWidth: 1920, screenHeight: 1080))
        XCTAssertNil(MouseCoordinates.map(x: 1, y: 1, viewWidth: .infinity, viewHeight: 800, screenWidth: 1920, screenHeight: 1080))
        XCTAssertNil(MouseCoordinates.map(x: 1, y: 1, viewWidth: 0, viewHeight: 800, screenWidth: 1920, screenHeight: 1080))
        XCTAssertNil(MouseCoordinates.map(x: 1, y: 1, viewWidth: 800, viewHeight: 800, screenWidth: 0, screenHeight: 1080))
    }
}
