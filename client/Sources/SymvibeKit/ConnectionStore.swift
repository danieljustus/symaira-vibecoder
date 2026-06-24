import Foundation
import Observation

/// Manages connection profiles and the active connection.
///
/// Persists profile metadata to UserDefaults; device tokens are stored
/// in Keychain (see ``KeychainHelper``).
@Observable
@MainActor
public final class ConnectionStore {
    @ObservationIgnored private let defaults: UserDefaults
    @ObservationIgnored private let decoder: JSONDecoder
    @ObservationIgnored private let encoder: JSONEncoder

    public var profiles: [ConnectionProfile] = []
    public var activeProfileID: UUID?

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
        self.decoder = JSONDecoder()
        self.decoder.keyDecodingStrategy = .convertFromSnakeCase
        self.encoder = JSONEncoder()
        self.encoder.keyEncodingStrategy = .convertToSnakeCase
        self.load()
    }

    // MARK: - Active Connection

    public var activeProfile: ConnectionProfile? {
        profiles.first { $0.id == activeProfileID }
    }

    public func setActive(_ profileID: UUID?) {
        activeProfileID = profileID
        persist()
    }

    // MARK: - CRUD

    public func add(_ profile: ConnectionProfile) {
        profiles.append(profile)
        if activeProfileID == nil {
            activeProfileID = profile.id
        }
        persist()
    }

    public func remove(_ profileID: UUID) {
        if let profile = profiles.first(where: { $0.id == profileID }) {
            KeychainHelper.delete(key: profile.keychainAccount)
        }
        profiles.removeAll { $0.id == profileID }
        if activeProfileID == profileID {
            activeProfileID = profiles.first?.id
        }
        persist()
    }

    public func update(_ profile: ConnectionProfile) {
        if let idx = profiles.firstIndex(where: { $0.id == profile.id }) {
            profiles[idx] = profile
            persist()
        }
    }

    // MARK: - Device Token

    public func deviceToken(for profileID: UUID) throws -> String? {
        guard let profile = profiles.first(where: { $0.id == profileID }) else { return nil }
        return try KeychainHelper.read(key: profile.keychainAccount)
    }

    public func saveDeviceToken(_ token: String, for profileID: UUID) throws {
        guard let profile = profiles.first(where: { $0.id == profileID }) else { return }
        try KeychainHelper.save(key: profile.keychainAccount, value: token)
    }

    // MARK: - Persistence

    private let profilesKey = "com.symvibe.connectionProfiles"
    private let activeProfileKey = "com.symvibe.activeProfileID"

    private func load() {
        guard let data = defaults.data(forKey: profilesKey),
              let decoded = try? decoder.decode([ConnectionProfile].self, from: data)
        else {
            return
        }
        profiles = decoded
        if let idString = defaults.string(forKey: activeProfileKey),
           let id = UUID(uuidString: idString) {
            activeProfileID = id
        }
    }

    private func persist() {
        guard let data = try? encoder.encode(profiles) else { return }
        defaults.set(data, forKey: profilesKey)
        if let activeProfileID {
            defaults.set(activeProfileID.uuidString, forKey: activeProfileKey)
        }
    }
}
