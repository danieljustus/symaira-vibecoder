import Foundation

// MARK: - Pairing Errors

/// Errors specific to the pairing flow.
public enum PairingError: LocalizedError, Sendable {
    case invalidPayload(String)
    case noHostCandidates
    case hostUnreachable(String, Error)
    case tlsPinningFailed(String)
    case serverRejected(String)
    case pairingCodeExpired
    case keychainSaveFailed(OSStatus)
    case keychainReadFailed(OSStatus)
    case unexpectedResponse

    public var errorDescription: String? {
        switch self {
        case .invalidPayload(let detail):
            "Invalid QR payload: \(detail)"
        case .noHostCandidates:
            "No host candidates in QR payload"
        case .hostUnreachable(let host, let error):
            "Host \(host) unreachable: \(error.localizedDescription)"
        case .tlsPinningFailed(let host):
            "TLS certificate mismatch for \(host)"
        case .serverRejected(let message):
            "Server rejected pairing: \(message)"
        case .pairingCodeExpired:
            "Pairing code has expired (TTL ~120s)"
        case .keychainSaveFailed(let status):
            "Keychain save failed (status \(status))"
        case .keychainReadFailed(let status):
            "Keychain read failed (status \(status))"
        case .unexpectedResponse:
            "Unexpected server response"
        }
    }
}

// MARK: - QR Pairing Payload

/// Parsed `symvibe://pair?…` URI.
///
/// Format: `symvibe://pair?n=<name>&p=<port>&h=<host>&h=<host2>&fp=<fingerprint>&c=<code>`
///
/// Multiple `h=` values are supported (LAN IP, mDNS `.local`, Tailscale IP).
public struct PairingPayload: Sendable, Equatable {
    public let name: String
    public let port: Int
    public let hostCandidates: [String]
    public let certFingerprint: String
    public let pairingCode: String

    /// Parse a `symvibe://pair?…` URI string.
    public static func parse(_ urlString: String) throws -> PairingPayload {
        guard let url = URL(string: urlString),
              url.scheme == "symvibe",
              url.path == "/pair" else {
            throw PairingError.invalidPayload("Not a symvibe://pair URL")
        }

        let items = URLComponents(url: url, resolvingAgainstBaseURL: false)?
            .queryItems ?? []

        var hosts: [String] = []
        var name = "Mac"
        var port = 4317
        var fingerprint = ""
        var code = ""

        for item in items {
            switch item.name {
            case "n":
                name = item.value ?? name
            case "p":
                port = Int(item.value ?? "") ?? port
            case "h":
                if let v = item.value, !v.isEmpty { hosts.append(v) }
            case "fp":
                fingerprint = item.value ?? fingerprint
            case "c":
                code = item.value ?? code
            default:
                break
            }
        }

        guard !hosts.isEmpty else {
            throw PairingError.noHostCandidates
        }
        guard !fingerprint.isEmpty else {
            throw PairingError.invalidPayload("Missing fp (fingerprint)")
        }
        guard !code.isEmpty else {
            throw PairingError.invalidPayload("Missing c (pairing code)")
        }

        return PairingPayload(
            name: name,
            port: port,
            hostCandidates: hosts,
            certFingerprint: fingerprint,
            pairingCode: code
        )
    }
}
