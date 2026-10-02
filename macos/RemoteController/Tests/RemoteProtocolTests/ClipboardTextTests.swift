import Foundation
import XCTest
@testable import RemoteProtocol

final class ClipboardTextTests: XCTestCase {
    func testRoundTripStatusesAndUnicode() throws {
        let success = ClipboardTextPayload(status: .success, text: "Hello，世界\n")
        XCTAssertEqual(try ClipboardTextPayload.decode(success.encode()), success)
        for status in [ClipboardTextStatus.unavailable, .tooLarge] {
            let payload = ClipboardTextPayload(status: status)
            XCTAssertEqual(try ClipboardTextPayload.decode(payload.encode()), payload)
        }
    }

    func testRejectsMalformedStatusUtf8AndOversize() throws {
        XCTAssertThrowsError(try ClipboardTextPayload.decode(Data()))
        XCTAssertThrowsError(try ClipboardTextPayload.decode(Data([9])))
        XCTAssertThrowsError(try ClipboardTextPayload.decode(Data([ClipboardTextStatus.unavailable.rawValue, 1])))
        XCTAssertThrowsError(try ClipboardTextPayload.decode(Data([ClipboardTextStatus.success.rawValue, 0xff])))
        XCTAssertThrowsError(try ClipboardTextPayload(status: .success,
            text: String(repeating: "a", count: ClipboardTextPayload.maximumTextBytes + 1)).encode())
    }
}
