import Foundation
@testable import RemoteProtocol

enum InputQueueFixture {
    struct Manifest: Decodable {
        struct Event: Decodable {
            let type: String
            let payloadHex: String
            var payload: Data {
                var bytes = Data()
                var index = payloadHex.startIndex
                while index < payloadHex.endIndex {
                    let end = payloadHex.index(index, offsetBy: 2)
                    bytes.append(UInt8(payloadHex[index..<end], radix: 16)!)
                    index = end
                }
                return bytes
            }
            func captured() throws -> CapturedInput {
                switch type {
                case "mouseMove": return .move(try MouseMovePayload.decode(payload))
                case "mouseButton": return .button(try MouseButtonPayload.decode(payload))
                case "mouseWheel": return .wheel(try MouseWheelPayload.decode(payload))
                case "keyEvent": return .key(try KeyEventPayload.decode(payload))
                default: throw ProtocolError.invalidPayload
                }
            }
        }
        let capacity: Int
        let inputs: [Event]
        let expected: [Event]
    }
    static func load() throws -> Manifest {
        var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        for _ in 0..<4 { root.deleteLastPathComponent() }
        return try JSONDecoder().decode(Manifest.self, from: Data(contentsOf:
            root.appendingPathComponent("protocol/testdata/input-queue-v1.json")))
    }
}
