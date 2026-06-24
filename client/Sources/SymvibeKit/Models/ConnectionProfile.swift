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
