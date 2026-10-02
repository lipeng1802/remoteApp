import Foundation

public enum PairingKeyParserError: Error, Equatable {
    case invalidEncoding
    case invalidLength
}

public enum PairingKeyParser {
    public static func parseBase64(_ text: String) throws -> Data {
        let compact = text.components(separatedBy: .whitespacesAndNewlines).joined()
        guard !compact.isEmpty, let key = Data(base64Encoded: compact) else {
            throw PairingKeyParserError.invalidEncoding
        }
        guard key.count == 32 else { throw PairingKeyParserError.invalidLength }
        return key
    }
}
