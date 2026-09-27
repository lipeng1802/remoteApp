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

public final class KeychainTrustedFingerprintStore: TrustedFingerprintStore {
    private let service: String

    public init(service: String = "com.personalremotedesktop.controller.certificate-fingerprint") {
        self.service = service
    }

    public func loadFingerprint(for deviceIdentifier: String) throws -> CertificateFingerprint? {
        try validate(deviceIdentifier)
        var result: CFTypeRef?
        let status = SecItemCopyMatching([
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: service,
            kSecAttrAccount: deviceIdentifier,
            kSecReturnData: true,
            kSecMatchLimit: kSecMatchLimitOne
        ] as CFDictionary, &result)

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
        let query = [
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: service,
            kSecAttrAccount: deviceIdentifier
        ] as CFDictionary
        let attributes = [kSecValueData: fingerprint.bytes] as CFDictionary

        let updateStatus = SecItemUpdate(query, attributes)
        if updateStatus == errSecSuccess { return }
        guard updateStatus == errSecItemNotFound else {
            throw FingerprintStoreError.keychainStatus(updateStatus)
        }

        let addStatus = SecItemAdd([
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: service,
            kSecAttrAccount: deviceIdentifier,
            kSecValueData: fingerprint.bytes,
            kSecAttrAccessible: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        ] as CFDictionary, nil)
        guard addStatus == errSecSuccess else {
            throw FingerprintStoreError.keychainStatus(addStatus)
        }
    }

    public func removeFingerprint(for deviceIdentifier: String) throws {
        try validate(deviceIdentifier)
        let status = SecItemDelete([
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: service,
            kSecAttrAccount: deviceIdentifier
        ] as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw FingerprintStoreError.keychainStatus(status)
        }
    }

    private func validate(_ deviceIdentifier: String) throws {
        guard !deviceIdentifier.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw FingerprintStoreError.invalidDeviceIdentifier
        }
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
