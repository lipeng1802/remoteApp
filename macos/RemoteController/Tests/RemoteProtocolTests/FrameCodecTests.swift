import Foundation
import XCTest
@testable import RemoteProtocol

final class FrameCodecTests: XCTestCase {
    func testGoldenVectorsDecodeAndReencode() throws {
        for vector in try loadManifest().vectors {
            let wire = try Data(hex: vector.frameHex)
            var decoder = FrameDecoder()
            let frames = try decoder.append(wire)

            XCTAssertEqual(frames.count, 1, vector.name)
            let frame = try XCTUnwrap(frames.first)
            XCTAssertEqual(frame.type.rawValue, vector.messageType, vector.name)
            XCTAssertEqual(frame.flags, vector.flags, vector.name)
            XCTAssertEqual(frame.payload.count, vector.payloadLength, vector.name)
            XCTAssertEqual(frame.sequence, vector.sequence, vector.name)
            XCTAssertEqual(frame.timestampMicros, vector.timestampMicros, vector.name)
            XCTAssertEqual(frame.payload, try Data(hex: vector.payloadHex), vector.name)
            XCTAssertEqual(try FrameCodec.encode(frame), wire, vector.name)
            XCTAssertNoThrow(try decoder.finish(), vector.name)
        }
    }

    func testOneByteStreamSplits() throws {
        let wire = try Data(hex: try loadManifest().vectors[0].frameHex)
        var decoder = FrameDecoder()
        var decoded: [Frame] = []

        for byte in wire {
            decoded.append(contentsOf: try decoder.append(Data([byte])))
        }

        XCTAssertEqual(decoded.count, 1)
        XCTAssertNoThrow(try decoder.finish())
    }

    func testCoalescedFrames() throws {
        let vectors = try loadManifest().vectors
        let wire = try Data(hex: vectors[1].frameHex) + Data(hex: vectors[2].frameHex)
        var decoder = FrameDecoder()

        let frames = try decoder.append(wire)

        XCTAssertEqual(frames.map(\.type), [.ping, .disconnect])
        XCTAssertNoThrow(try decoder.finish())
    }

    func testRejectsInvalidMagic() throws {
        var wire = try Data(hex: try loadManifest().vectors[0].frameHex)
        wire[0] = 0
        var decoder = FrameDecoder()

        XCTAssertThrowsError(try decoder.append(wire)) { error in
            XCTAssertEqual(error as? ProtocolError, .invalidMagic)
        }
    }

    func testRejectsOversizedPayloadBeforeBodyArrives() throws {
        var wire = try Data(hex: try loadManifest().vectors[1].frameHex)
        wire.replaceSubrange(12..<16, with: [0x00, 0x01, 0x00, 0x01])
        var decoder = FrameDecoder()

        XCTAssertThrowsError(try decoder.append(wire.prefix(28))) { error in
            XCTAssertEqual(error as? ProtocolError, .messageTooLarge(65_537))
        }
    }

    func testFinishRejectsIncompleteFrame() throws {
        let wire = try Data(hex: try loadManifest().vectors[0].frameHex)
        var decoder = FrameDecoder()
        XCTAssertTrue(try decoder.append(wire.dropLast()).isEmpty)
        XCTAssertThrowsError(try decoder.finish()) { error in
            XCTAssertEqual(error as? ProtocolError, .incompleteFrame)
        }
    }
}

private struct Manifest: Decodable {
    let vectors: [Vector]
}

private struct Vector: Decodable {
    let name: String
    let frameHex: String
    let messageType: UInt16
    let flags: UInt16
    let payloadLength: Int
    let sequence: UInt32
    let timestampMicros: UInt64
    let payloadHex: String
}

private func loadManifest() throws -> Manifest {
    var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
    for _ in 0..<4 { root.deleteLastPathComponent() }
    let data = try Data(contentsOf: root.appendingPathComponent("protocol/testdata/v1.json"))
    return try JSONDecoder().decode(Manifest.self, from: data)
}

private extension Data {
    init(hex: String) throws {
        guard hex.count.isMultiple(of: 2) else { throw HexError.invalidLength }
        self.init(capacity: hex.count / 2)
        var index = hex.startIndex
        while index < hex.endIndex {
            let next = hex.index(index, offsetBy: 2)
            guard let byte = UInt8(hex[index..<next], radix: 16) else {
                throw HexError.invalidCharacter
            }
            append(byte)
            index = next
        }
    }
}

private enum HexError: Error {
    case invalidLength
    case invalidCharacter
}
