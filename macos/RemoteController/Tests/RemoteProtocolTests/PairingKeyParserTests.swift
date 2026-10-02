import XCTest
@testable import RemoteProtocol

final class PairingKeyParserTests: XCTestCase {
    func testAcceptsExact32ByteBase64AndWhitespace() throws {
        let expected = Data(0..<32)
        let encoded = expected.base64EncodedString()
        XCTAssertEqual(try PairingKeyParser.parseBase64(encoded), expected)
        XCTAssertEqual(try PairingKeyParser.parseBase64("  \n\(encoded)\t"), expected)
    }

    func testRejectsMalformedBase64() {
        XCTAssertThrowsError(try PairingKeyParser.parseBase64("not base64!")) { error in
            XCTAssertEqual(error as? PairingKeyParserError, .invalidEncoding)
        }
    }

    func testRejectsWrongLength() {
        XCTAssertThrowsError(try PairingKeyParser.parseBase64(Data(repeating: 7, count: 31).base64EncodedString())) { error in
            XCTAssertEqual(error as? PairingKeyParserError, .invalidLength)
        }
    }

}
