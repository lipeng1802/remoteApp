import Foundation
import XCTest
@testable import RemoteProtocol

final class CertificateTrustTests: XCTestCase {
    func testFingerprintGoldenVector() throws {
        let vector = try loadTLSVector()
        let fingerprint = try CertificateFingerprint.sha256(
            certificateDER: Data(tlsHexadecimal: vector.certificateDerHex)
        )

        XCTAssertEqual(fingerprint.hexadecimal, vector.sha256FingerprintHex)
    }

    func testFirstUseReturnsFingerprintForExplicitApproval() throws {
        let certificate = Data(repeating: 7, count: 64)
        let expected = try CertificateFingerprint.sha256(certificateDER: certificate)

        XCTAssertEqual(
            try CertificateTrustPolicy.evaluate(storedFingerprint: nil, certificateDER: certificate),
            .trustOnFirstUse(expected)
        )
    }

    func testMatchingFingerprintIsTrustedAndChangeIsRejected() throws {
        let certificate = Data(repeating: 8, count: 64)
        let stored = try CertificateFingerprint.sha256(certificateDER: certificate)

        XCTAssertEqual(
            try CertificateTrustPolicy.evaluate(storedFingerprint: stored, certificateDER: certificate),
            .trusted
        )
        XCTAssertEqual(
            try CertificateTrustPolicy.evaluate(
                storedFingerprint: stored,
                certificateDER: Data(repeating: 9, count: 64)
            ),
            .rejectFingerprintMismatch
        )
    }
}

private struct TLSVector: Decodable {
    let certificateDerHex: String
    let sha256FingerprintHex: String
}

private func loadTLSVector() throws -> TLSVector {
    var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
    for _ in 0..<4 { root.deleteLastPathComponent() }
    let data = try Data(contentsOf: root.appendingPathComponent("protocol/testdata/tls-v1.json"))
    return try JSONDecoder().decode(TLSVector.self, from: data)
}

private extension Data {
    init(tlsHexadecimal: String) {
        self.init(capacity: tlsHexadecimal.count / 2)
        var index = tlsHexadecimal.startIndex
        while index < tlsHexadecimal.endIndex {
            let next = tlsHexadecimal.index(index, offsetBy: 2)
            append(UInt8(tlsHexadecimal[index..<next], radix: 16)!)
            index = next
        }
    }
}
