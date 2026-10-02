import Foundation
import Security

public enum DeviceKeychainService {
    public static let legacy = "com.personalremotedesktop.controller.device-key"
    public static let application = "com.personalremotedesktop.controller.device-key.v2"
}

public final class KeychainDeviceKeyStore {
    private let service: String

    public init(service: String = DeviceKeychainService.legacy) {
        self.service = service
    }
    public func loadKey(for deviceIdentifier: String) throws -> Data? {
        var query = try baseQuery(deviceIdentifier)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess else { throw FingerprintStoreError.keychainStatus(status) }
        guard let data = result as? Data, data.count == 32 else { throw FingerprintStoreError.unexpectedData }
        return data
    }
    public func saveKey(_ key: Data, for deviceIdentifier: String) throws {
        guard key.count == 32 else { throw ProtocolError.invalidPayload }
        let query = try baseQuery(deviceIdentifier)
        let status = SecItemUpdate(query as CFDictionary, [kSecValueData: key] as CFDictionary)
        if status == errSecSuccess { return }
        guard status == errSecItemNotFound else { throw FingerprintStoreError.keychainStatus(status) }
        var attributes = query
        attributes[kSecValueData as String] = key
        attributes[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        let added = SecItemAdd(attributes as CFDictionary, nil)
        guard added == errSecSuccess else { throw FingerprintStoreError.keychainStatus(added) }
    }
    public func removeKey(for deviceIdentifier: String) throws {
        let status = SecItemDelete(try baseQuery(deviceIdentifier) as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw FingerprintStoreError.keychainStatus(status)
        }
    }
    private func baseQuery(_ deviceIdentifier: String) throws -> [String: Any] {
        guard !deviceIdentifier.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw FingerprintStoreError.invalidDeviceIdentifier
        }
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: deviceIdentifier
        ]
        return query
    }
}
