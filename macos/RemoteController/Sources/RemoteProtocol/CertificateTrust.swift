import CryptoKit
import Foundation

public struct CertificateFingerprint: Equatable, Sendable {
    public static let byteCount = 32
    public let bytes: Data

    public init(bytes: Data) throws {
        guard bytes.count == Self.byteCount else { throw ProtocolError.invalidPayload }
        self.bytes = bytes
    }

    public static func sha256(certificateDER: Data) throws -> CertificateFingerprint {
        guard !certificateDER.isEmpty else { throw ProtocolError.invalidPayload }
        return try CertificateFingerprint(bytes: Data(SHA256.hash(data: certificateDER)))
    }

    public var hexadecimal: String {
        bytes.map { String(format: "%02x", $0) }.joined()
    }
}

public enum CertificateTrustDecision: Equatable {
    case trustOnFirstUse(CertificateFingerprint)
    case trusted
    case rejectFingerprintMismatch
}

public enum CertificateTrustPolicy {
    public static func evaluate(
        storedFingerprint: CertificateFingerprint?,
        certificateDER: Data
    ) throws -> CertificateTrustDecision {
        let presented = try CertificateFingerprint.sha256(certificateDER: certificateDER)
        guard let storedFingerprint else {
            return .trustOnFirstUse(presented)
        }
        return Authentication.constantTimeEquals(storedFingerprint.bytes, presented.bytes)
            ? .trusted
            : .rejectFingerprintMismatch
    }
}
