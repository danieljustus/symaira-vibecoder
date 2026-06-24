import Foundation

public actor SSEClient {
    public let baseURL: URL
    public var token: String?

    private let session: URLSession
    private let decoder: JSONDecoder

    public init(baseURL: URL, token: String? = nil, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.token = token
        self.session = session
        self.decoder = JSONDecoder()
        self.decoder.keyDecodingStrategy = .convertFromSnakeCase
    }

    public func setToken(_ token: String?) {
        self.token = token
    }

    /// Opens `/events` and returns an async stream of server-sent events.
    /// Reconnects automatically with exponential backoff up to `maxDelay`.
    public func events(
        reconnect: Bool = true,
        initialDelay: Duration = .seconds(1),
        maxDelay: Duration = .seconds(30),
        maxRetries: Int = 10
    ) -> AsyncThrowingStream<Event, Error> {
        AsyncThrowingStream { continuation in
            let task = Task {
                var delay = initialDelay
                var retries = 0
                while !Task.isCancelled {
                    do {
                        let request = self.makeRequest()
                        let (bytes, response) = try await self.session.bytes(for: request)
                        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
                            throw SymvibeError.http(status: (response as? HTTPURLResponse)?.statusCode ?? 0, body: "")
                        }
                        retries = 0
                        delay = initialDelay
                        var buffer: [String] = []
                        for try await line in bytes.lines {
                            if Task.isCancelled { break }
                            let trimmed = line.trimmingCharacters(in: .whitespaces)
                            if trimmed.isEmpty {
                                if let event = self.parseEvent(buffer) {
                                    continuation.yield(event)
                                }
                                buffer.removeAll()
                            } else if trimmed.hasPrefix(":") {
                                // ping/comment — ignore (tolerates 15s keep-alive pings)
                                continue
                            } else if trimmed.hasPrefix("data: ") {
                                buffer.append(String(trimmed.dropFirst(6)))
                            }
                        }
                        if !reconnect { break }
                    } catch {
                        continuation.yield(with: .failure(error))
                        if !reconnect { break }
                        retries += 1
                        if retries > maxRetries { break }
                        try? await Task.sleep(for: delay)
                        delay = min(delay * 2, maxDelay)
                    }
                }
                continuation.finish()
            }
            continuation.onTermination = { _ in
                task.cancel()
            }
        }
    }

    private func makeRequest() -> URLRequest {
        let url = baseURL.appendingPathComponent("/events")
        var request = URLRequest(url: url)
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        request.timeoutInterval = 300
        if let token {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return request
    }

    private func parseEvent(_ lines: [String]) -> Event? {
        let data = lines.joined(separator: "\n")
        guard !data.isEmpty else { return nil }
        do {
            return try decoder.decode(Event.self, from: Data(data.utf8))
        } catch {
            return Event(type: "raw", line: data, ts: nil)
        }
    }
}
