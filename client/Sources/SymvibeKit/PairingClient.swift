import Foundation

/// Performs the pairing handshake against a symvibe server.
///
/// Probes host candidates sequentially, pins the TLS certificate, and
/// exchanges the pairing code for a device token.
public actor PairingClient {
    private let decoder: JSONDecoder
    private let encoder: JSONEncoder

    public init() {
        self.decoder = JSONDecoder()
        self.decoder.keyDecodingStrategy = .convertFromSnakeCase
        self.encoder = JSONEncoder()
        self.encoder.keyEncodingStrategy = .convertToSnakeCase
    }

    /// Try each host candidate and complete pairing against the first reachable one.
    ///
    /// - Returns: A tuple of the created ``ConnectionProfile`` and the raw device token.
    public func completePairing(payload: PairingPayload) async throws -> (profile: ConnectionProfile, token: String) {
        var lastError: Error = PairingError.noHostCandidates

        for host in payload.hostCandidates {
            do {
                return try await tryHost(host: host, payload: payload)
            } catch let error as PairingError {
                switch error {
                case .pairingCodeExpired:
                    throw error // Don't try other hosts
                default:
                    lastError = error
                    continue
                }
            }
        }
        throw lastError
    }

    // MARK: - Private

    private func tryHost(host: String, payload: PairingPayload) async throws -> (profile: ConnectionProfile, token: String) {
        guard let baseURL = URL(string: "https://\(host):\(payload.port)") else {
            throw PairingError.invalidPayload("Invalid host: \(host)")
        }

        let pinningDelegate = TLSPinningDelegate(expectedFingerprint: payload.certFingerprint)
        let session = URLSession(
            configuration: .ephemeral,
            delegate: pinningDelegate,
            delegateQueue: nil
        )

        // POST /api/pair/complete
        var request = URLRequest(url: baseURL.appendingPathComponent("/api/pair/complete"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.timeoutInterval = 10

        let body = PairingCompleteRequest(code: payload.pairingCode, name: payload.name)
        request.httpBody = try encoder.encode(body)

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            throw PairingError.hostUnreachable(host, error)
        }

        guard let http = response as? HTTPURLResponse else {
            throw PairingError.hostUnreachable(host, URLError(.badServerResponse))
        }

        guard (200..<300).contains(http.statusCode) else {
            if http.statusCode == 410 {
                throw PairingError.pairingCodeExpired
            }
            let message = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw PairingError.serverRejected("HTTP \(http.statusCode): \(message)")
        }

        let decoded = try decoder.decode(PairingCompleteResponse.self, from: data)
        let profile = ConnectionProfile(
            name: decoded.name,
            hostCandidates: payload.hostCandidates,
            port: payload.port,
            certFingerprint: payload.certFingerprint
        )

        return (profile, decoded.token)
    }
}

// MARK: - Request / Response types (private to this file)

private struct PairingCompleteRequest: Encodable, Sendable {
    let code: String
    let name: String
}

private struct PairingCompleteResponse: Decodable, Sendable {
    let id: String
    let name: String
    let token: String
}
