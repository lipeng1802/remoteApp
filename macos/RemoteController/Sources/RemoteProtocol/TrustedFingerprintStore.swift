import Foundation
import Security

public protocol TrustedFingerprintStore {
    func loadFingerprint(for deviceIdentifier: String) throws -> CertificateFingerprint?
    func saveFingerprint(_ fingerprint: CertificateFingerprint, for deviceIdentifier: String) throws
    func removeFingerprint(for deviceIdentifier: String) throws
}

public enum FingerprintStoreError: Error, Equatable {
    case invalidDeviceIdentifier
    case unexpectedData
    case keychainStatus(OSStatus)
}

public enum FingerprintKeychainService {
    public static let legacy = "com.personalremotedesktop.controller.certificate-fingerprint"
    public static let application = "com.personalremotedesktop.controller.certificate-fingerprint.v2"
}

public final class KeychainTrustedFingerprintStore: TrustedFingerprintStore {
    private let service: String

    public init(service: String = FingerprintKeychainService.legacy) {
        self.service = service
    }

    public func loadFingerprint(for deviceIdentifier: String) throws -> CertificateFingerprint? {
        try validate(deviceIdentifier)
        var result: CFTypeRef?
        var query = baseQuery(deviceIdentifier)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        let status = SecItemCopyMatching(query as CFDictionary, &result)

        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess else { throw FingerprintStoreError.keychainStatus(status) }
        guard let data = result as? Data else { throw FingerprintStoreError.unexpectedData }
        do {
            return try CertificateFingerprint(bytes: data)
        } catch {
            throw FingerprintStoreError.unexpectedData
        }
    }

    public func saveFingerprint(
        _ fingerprint: CertificateFingerprint,
        for deviceIdentifier: String
    ) throws {
        try validate(deviceIdentifier)
        let query = baseQuery(deviceIdentifier) as CFDictionary
        let attributes = [kSecValueData: fingerprint.bytes] as CFDictionary

        let updateStatus = SecItemUpdate(query, attributes)
        if updateStatus == errSecSuccess { return }
        guard updateStatus == errSecItemNotFound else {
            throw FingerprintStoreError.keychainStatus(updateStatus)
        }

        var add = baseQuery(deviceIdentifier)
        add[kSecValueData as String] = fingerprint.bytes
        add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        let addStatus = SecItemAdd(add as CFDictionary, nil)
        guard addStatus == errSecSuccess else {
            throw FingerprintStoreError.keychainStatus(addStatus)
        }
    }

    public func removeFingerprint(for deviceIdentifier: String) throws {
        try validate(deviceIdentifier)
        let status = SecItemDelete(baseQuery(deviceIdentifier) as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw FingerprintStoreError.keychainStatus(status)
        }
    }

    private func validate(_ deviceIdentifier: String) throws {
        guard !deviceIdentifier.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw FingerprintStoreError.invalidDeviceIdentifier
        }
    }

    private func baseQuery(_ deviceIdentifier: String) -> [String: Any] {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: deviceIdentifier
        ]
        return query
    }
}

public enum StoredCertificateTrustResult: Equatable {
    case approvedAndStored(CertificateFingerprint)
    case trusted
    case rejectedFirstUse(CertificateFingerprint)
    case rejectedFingerprintMismatch
}

public struct StoredCertificateTrustCoordinator {
    private let store: TrustedFingerprintStore

    public init(store: TrustedFingerprintStore) {
        self.store = store
    }

    public func evaluate(
        deviceIdentifier: String,
        certificateDER: Data,
        approveFirstUse: (CertificateFingerprint) -> Bool
    ) throws -> StoredCertificateTrustResult {
        let stored = try store.loadFingerprint(for: deviceIdentifier)
        switch try CertificateTrustPolicy.evaluate(
            storedFingerprint: stored,
            certificateDER: certificateDER
        ) {
        case .trusted:
            return .trusted
        case .rejectFingerprintMismatch:
            return .rejectedFingerprintMismatch
        case let .trustOnFirstUse(presented):
            guard approveFirstUse(presented) else {
                return .rejectedFirstUse(presented)
            }
            try store.saveFingerprint(presented, for: deviceIdentifier)
            return .approvedAndStored(presented)
        }
    }
}
