import Foundation

public struct ServerError: LocalizedError, Codable, Sendable, Equatable {
    public let message: String

    public var errorDescription: String? { message }

    enum CodingKeys: String, CodingKey {
        case message = "error"
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
        }
    }
}
