import Foundation

public struct ServerError: LocalizedError, Codable, Sendable, Equatable {
    public let message: String

    public var errorDescription: String? { message }

    enum CodingKeys: String, CodingKey {
        case message = "error"
    }
}

/// A already-humanized failure from a `BoardStore` operation, ready to show.
public struct OperationError: LocalizedError, Sendable, Equatable {
    public let message: String

    public var errorDescription: String? { message }

    public init(_ message: String) {
        self.message = message
    }
}

public enum SymvibeError: LocalizedError, Sendable {
    case invalidURL
    case encoding(Error)
    case transport(Error)
    case http(status: Int, body: String)
    case server(ServerError)
    case decoding(Error)
    case pinningFailed
    case notConnected
    /// `POST /api/cycle/import` rejected the template because this machine is
    /// missing skills/categories/agents/sensors it needs.
    case missingRequirements(missing: MissingRequirements, available: Catalog)

    public var errorDescription: String? {
        switch self {
        case .invalidURL:
            "Invalid URL"
        case .encoding(let error):
            "Encoding failed: \(error.localizedDescription)"
        case .transport(let error):
            "Network error: \(error.localizedDescription)"
        case .http(let status, let body):
            "HTTP \(status): \(body)"
        case .server(let error):
            error.message
        case .decoding(let error):
            "Decoding failed: \(error.localizedDescription)"
        case .pinningFailed:
            "TLS pinning failed"
        case .notConnected:
            "Not connected"
        case .missingRequirements(let missing, _):
            "Template requires building blocks this machine does not have: "
                + [
                    missing.categories.isEmpty ? nil : "categories (\(missing.categories.joined(separator: ", ")))",
                    missing.skills.isEmpty ? nil : "skills (\(missing.skills.joined(separator: ", ")))",
                    missing.agents.isEmpty ? nil : "agents (\(missing.agents.joined(separator: ", ")))",
                    missing.sensors.isEmpty ? nil : "sensors (\(missing.sensors.joined(separator: ", ")))",
                ].compactMap { $0 }.joined(separator: "; ")
        }
    }
}
