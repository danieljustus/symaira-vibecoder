import Foundation
import SymairaKeychain

/// Keychain wrapper delegating to SymairaKeychain.
public enum KeychainHelper: Sendable {
    private static let keychain = SymairaKeychain(service: "com.symvibe.device-tokens")

    /// Save a UTF-8 string to the Keychain under the given account.
    @discardableResult
    public static func save(key: String, value: String) throws -> Bool {
        do {
            return try keychain.save(value, key: key)
        } catch let error as SymairaKeychainError {
            if case .saveFailed(let status) = error {
                throw PairingError.keychainSaveFailed(status)
            }
            throw error
        }
    }

    /// Read a UTF-8 string from the Keychain for the given account.
    public static func read(key: String) throws -> String? {
        do {
            return try keychain.read(key: key)
        } catch let error as SymairaKeychainError {
            if case .readFailed(let status) = error {
                throw PairingError.keychainReadFailed(status)
            }
            throw error
        }
    }

    /// Delete an item from the Keychain.
    @discardableResult
    public static func delete(key: String) -> Bool {
        keychain.delete(key: key)
    }
}
