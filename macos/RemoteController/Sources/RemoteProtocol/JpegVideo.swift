import Foundation
import CoreGraphics
import ImageIO

public struct ScreenInfoPayload: Equatable {
    public let width: UInt32
    public let height: UInt32
    public let dpiX100: UInt32
    public let dpiY100: UInt32

    public static func decode(_ data: Data) throws -> ScreenInfoPayload {
        guard data.count == 18, data[16] == 1, data[17] == 0 else { throw ProtocolError.invalidPayload }
        func number(_ offset: Int) -> UInt32 { data[offset..<offset + 4].reduce(0) { ($0 << 8) | UInt32($1) } }
        let result = ScreenInfoPayload(width: number(0), height: number(4), dpiX100: number(8), dpiY100: number(12))
        guard (1...16384).contains(result.width), (1...16384).contains(result.height),
              (4800...96000).contains(result.dpiX100), (4800...96000).contains(result.dpiY100) else {
            throw ProtocolError.invalidPayload
        }
        return result
    }
    public func encode() throws -> Data {
        var bytes = Data()
        for value in [width, height, dpiX100, dpiY100] {
            bytes.append(contentsOf: [UInt8((value >> 24) & 255), UInt8((value >> 16) & 255),
                                      UInt8((value >> 8) & 255), UInt8(value & 255)])
        }
        bytes.append(contentsOf: [1, 0])
        _ = try Self.decode(bytes)
        return bytes
    }
}

public enum JpegImageDecoder {
    // Inspect dimensions before decompressing, even if the compressed payload is small.
    public static func decode(_ data: Data) -> CGImage? {
        guard data.count >= 4, data.count <= ProtocolConstants.maximumPayloadLength,
              data.prefix(2) == Data([0xff, 0xd8]), data.suffix(2) == Data([0xff, 0xd9]),
              let source = CGImageSourceCreateWithData(data as CFData, [kCGImageSourceShouldCache: false] as CFDictionary),
              CGImageSourceGetCount(source) == 1,
              let properties = CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as? [CFString: Any],
              let width = properties[kCGImagePropertyPixelWidth] as? NSNumber,
              let height = properties[kCGImagePropertyPixelHeight] as? NSNumber,
              (1...1280).contains(width.intValue), (1...720).contains(height.intValue) else { return nil }
        return CGImageSourceCreateImageAtIndex(source, 0, [kCGImageSourceShouldCacheImmediately: true] as CFDictionary)
    }
}

// A single replaceable value; UI polling never queues one main-thread closure per frame.
public final class LatestValue<Value> {
    private let lock = NSLock()
    private var value: Value?
    public init() {}
    public func replace(_ newValue: Value) { lock.lock(); value = newValue; lock.unlock() }
    public func take() -> Value? {
        lock.lock(); defer { lock.unlock() }
        let result = value; value = nil; return result
    }
    public func clear() { lock.lock(); value = nil; lock.unlock() }
}