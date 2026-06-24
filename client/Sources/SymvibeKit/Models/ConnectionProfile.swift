import Foundation

/// Represents a paired connection to a symvibe server (typically a Mac).
///
/// Stores connection metadata; the actual device token lives in Keychain
/// referenced by `keychainAccount`.
public struct ConnectionProfile: Codable, Sendable, Identifiable, Hashable {
    public let id: UUID
    public var name: String
    public var hostCandidates: [String]
    public var port: Int
    public var certFingerprint: String
    public var keychainAccount: String
    public var accountNodeID: String?
    public var createdAt: Date

    /// Well-known ID for the built-in demo profile.
    public static let demoID = UUID(uuidString: "00000000-0000-0000-0000-000000000001")!

    /// Pre-configured demo profile. Selecting this profile activates demo mode
    /// (sample data, no network calls) — required for App Store review.
    public static let demoProfile = ConnectionProfile(
        id: demoID,
        name: "Demo Mode",
        hostCandidates: [],
        port: 0,
        certFingerprint: ""
    )

    /// Whether this profile is the built-in demo profile.
    public var isDemo: Bool { id == Self.demoID }

    public init(
        id: UUID = UUID(),
        name: String,
        hostCandidates: [String],
        port: Int,
        certFingerprint: String,
        keychainAccount: String? = nil,
        accountNodeID: String? = nil,
        createdAt: Date = Date()
    ) {
        self.id = id
        self.name = name
        self.hostCandidates = hostCandidates
        self.port = port
        self.certFingerprint = certFingerprint
        self.keychainAccount = keychainAccount ?? "symvibe.device.\(id.uuidString)"
        self.accountNodeID = accountNodeID
        self.createdAt = createdAt
    }

    /// Build a base URL for a specific host candidate.
    public func baseURL(for host: String) -> URL? {
        URL(string: "https://\(host):\(port)")
    }
}
