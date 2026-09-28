import Foundation
import XCTest
@testable import RemoteProtocol

final class JpegVideoTests: XCTestCase {
    private func fixture() throws -> (Data, Data) {
        var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        for _ in 0..<4 { root.deleteLastPathComponent() }
        let data = try Data(contentsOf: root.appendingPathComponent("protocol/testdata/jpeg-v1.json"))
        let json = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        func hex(_ name: String) throws -> Data {
            let text = Array(try XCTUnwrap(json[name] as? String))
            return try Data(stride(from: 0, to: text.count, by: 2).map {
                try XCTUnwrap(UInt8(String(text[$0...$0 + 1]), radix: 16))
            })
        }
        return (try hex("screenInfoHex"), try hex("jpegHex"))
    }
    func testScreenMetadataGoldenVector() throws {
        let (metadata, _) = try fixture()
        let info = try ScreenInfoPayload.decode(metadata)
        XCTAssertEqual(info.width, 3840); XCTAssertEqual(info.height, 2160)
        XCTAssertEqual(info.dpiX100, 14400)
        XCTAssertEqual(try info.encode(), metadata)
        var invalid = metadata; invalid[17] = 1
        XCTAssertThrowsError(try ScreenInfoPayload.decode(invalid))
        invalid = metadata; invalid.replaceSubrange(0..<4, with: [0, 0, 0, 0])
        XCTAssertThrowsError(try ScreenInfoPayload.decode(invalid))
    }
    func testSyntheticJpegDecodes() throws {
        let (_, jpeg) = try fixture()
        let image = try XCTUnwrap(JpegImageDecoder.decode(jpeg))
        XCTAssertEqual(image.width, 16); XCTAssertEqual(image.height, 9)
    }
    func testCorruptJpegDropped() throws {
        XCTAssertNil(JpegImageDecoder.decode(Data([0xff, 0xd8, 0xff, 0xd9])))
        let (_, jpeg) = try fixture()
        XCTAssertNil(JpegImageDecoder.decode(Data(jpeg.dropLast(10))))
        XCTAssertNotNil(JpegImageDecoder.decode(jpeg))
    }
    func testOversizedDecodedDimensionsRejected() throws {
        var (_, jpeg) = try fixture()
        var found = false
        for index in 0..<jpeg.count - 8 where jpeg[index] == 0xff && jpeg[index + 1] == 0xc0 {
            jpeg[index + 7] = 0x7f; jpeg[index + 8] = 0xff
            found = true; break
        }
        XCTAssertTrue(found)
        XCTAssertNil(JpegImageDecoder.decode(jpeg))
    }
    func testVideoDecoderSplitsAndPreservesNextFrame() throws {
        let (_, jpeg) = try fixture()
        let frame = Frame(type: .videoFrameJPEG, sequence: 5, payload: jpeg)
        let ping = Frame(type: .ping, sequence: 6, payload: Data(count: 8))
        let wire = try FrameCodec.encode(frame) + FrameCodec.encode(ping)
        var decoder = ProbeFrameDecoder(); decoder.allowJpeg = true
        var frames: [Frame] = []
        for start in stride(from: 0, to: wire.count, by: 17) {
            frames += try decoder.append(wire.subdata(in: start..<min(start + 17, wire.count)))
        }
        XCTAssertEqual(frames, [frame, ping])
        var header = Data(try FrameCodec.encode(frame).prefix(28))
        header.replaceSubrange(12..<16, with: [0, 128, 0, 1])
        XCTAssertThrowsError(try decoder.append(header))
    }
    private func authenticated() throws -> AuthenticatedProbeSession {
        var session = try AuthenticatedProbeSession(deviceKey: Data(repeating: 1, count: 32), streaming: true)
        XCTAssertTrue(try HelloPayload.decode(session.start().payload).capabilities.contains(.jpeg))
        _ = try session.receive(Frame(type: .hello, sequence: 1,
            payload: HelloPayload(role: .agent, capabilities: [.jpeg], nonce: Data(count: 32)).encode()))
        _ = try session.receive(Frame(type: .authChallenge, sequence: 2,
            payload: AuthChallengePayload(challenge: Data(count: 32), agentIdentifier: Data(count: 16)).encode()))
        XCTAssertTrue(try session.receive(Frame(type: .authResult, sequence: 3,
            payload: AuthResultPayload(status: .success, retryDelayMilliseconds: 0).encode())).isEmpty)
        return session
    }
    func testStreamingMetadataVideoAndAcknowledgement() throws {
        var session = try authenticated()
        let (metadata, jpeg) = try fixture()
        _ = try session.receive(Frame(type: .screenInfo, sequence: 4, payload: metadata))
        _ = try session.receive(Frame(type: .videoFrameJPEG, sequence: 5, payload: jpeg))
        let token = Data(repeating: 9, count: 8)
        XCTAssertEqual(try session.receive(Frame(type: .ping, sequence: 6, payload: token)),
            [Frame(type: .pong, sequence: 3, payload: token)])
        XCTAssertFalse(session.isComplete)
        _ = try session.receive(Frame(type: .screenInfo, sequence: 7, payload: metadata))
    }
    func testVideoBeforeMetadataRejected() throws {
        var session = try authenticated()
        XCTAssertThrowsError(try session.receive(Frame(type: .videoFrameJPEG, sequence: 4, payload: Data())))
    }
    func testVideoBeforeAuthenticationRejected() throws {
        var session = try AuthenticatedProbeSession(deviceKey: Data(count: 32), streaming: true)
        _ = try session.start()
        XCTAssertThrowsError(try session.receive(Frame(type: .videoFrameJPEG, sequence: 1, payload: Data()))) {
            XCTAssertEqual($0 as? ProtocolError, .authRequired)
        }
    }
    func testLatestValueNeverAccumulatesQueue() {
        let slot = LatestValue<Int>()
        for value in 0..<10000 { slot.replace(value) }
        XCTAssertEqual(slot.take(), 9999)
        XCTAssertNil(slot.take())
        slot.replace(1); slot.clear(); XCTAssertNil(slot.take())
    }
}