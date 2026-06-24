import Foundation
#if canImport(Security)
import Security
#endif

/// Minimal Keychain wrapper for storing device tokens.
///
/// Uses `kSecAttrAccessibleAfterFirstUnlock` — accessible after device first
/// unlock, appropriate for a developer tool that may use background fetch.
public enum KeychainHelper: Sendable {
    private static let service = "com.symvibe.device-tokens"

    /// Save a UTF-8 string to the Keychain under the given account.
    @discardableResult
    public static func save(key: String, value: String) throws -> Bool {
        let data = Data(value.utf8)

        // Delete any existing item first
        let deleteQuery: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
        ]
        SecItemDelete(deleteQuery as CFDictionary)

        let addQuery: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlock,
        ]

        let status = SecItemAdd(addQuery as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw PairingError.keychainSaveFailed(status)
        }
        return true
    }

    /// Read a UTF-8 string from the Keychain for the given account.
    public static func read(key: String) throws -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)

        guard status == errSecSuccess, let data = item as? Data else {
            if status == errSecItemNotFound { return nil }
            throw PairingError.keychainReadFailed(status)
        }

        return String(data: data, encoding: .utf8)
    }

    /// Delete an item from the Keychain.
    @discardableResult
    public static func delete(key: String) -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
        ]
        return SecItemDelete(query as CFDictionary) == errSecSuccess
    }
}
