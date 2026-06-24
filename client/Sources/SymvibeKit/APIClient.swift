import Foundation

public actor APIClient {
    public let baseURL: URL
    public var token: String?

    private let session: URLSession
    private let decoder: JSONDecoder
    private let encoder: JSONEncoder

    public init(baseURL: URL, token: String? = nil, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.token = token
        self.session = session
        self.decoder = JSONDecoder()
        self.decoder.keyDecodingStrategy = .convertFromSnakeCase
        self.encoder = JSONEncoder()
        self.encoder.keyEncodingStrategy = .convertToSnakeCase
    }

    public func setToken(_ token: String?) {
        self.token = token
    }

    // MARK: - Meta

    public func version() async throws -> VersionResponse {
        try await get(path: "/api/version")
    }

    public func doctor() async throws -> DoctorResponse {
        try await get(path: "/api/doctor")
    }

    public func skills() async throws -> [Skill] {
        let resp: SkillsResponse = try await get(path: "/api/skills")
        return resp.skills
    }

    public func models() async throws -> ModelsResponse {
        try await get(path: "/api/models")
    }

    public func categories() async throws -> CategoriesResponse {
        try await get(path: "/api/categories")
    }

    public func runState() async throws -> RunState {
        try await get(path: "/api/runstate")
    }

    // MARK: - Cycle

    public func cycle() async throws -> Cycle {
        try await get(path: "/api/cycle")
    }

    public func updateCycle(_ cycle: Cycle) async throws -> Cycle {
        try await put(path: "/api/cycle", body: cycle)
    }

    // MARK: - Run

    public func runCycle() async throws {
        try await postVoid(path: "/api/run")
    }

    public func runStep(_ stepID: String) async throws {
        struct Body: Encodable { let step_id: String }
        try await postVoid(path: "/api/run/step", body: Body(step_id: stepID))
    }

    public func controlRun(action: String) async throws {
        struct Body: Encodable { let action: String }
        try await postVoid(path: "/api/run/control", body: Body(action: action))
    }

    // MARK: - Generic helpers

    private func get<T: Decodable>(path: String) async throws -> T {
        try await perform(makeRequest(path: path, method: "GET"))
    }

    private func post<T: Decodable>(path: String, body: some Encodable) async throws -> T {
        try await perform(makeRequest(path: path, method: "POST", body: body))
    }

    private func postVoid(path: String) async throws {
        _ = try await performRaw(makeRequest(path: path, method: "POST"))
    }

    private func postVoid(path: String, body: some Encodable) async throws {
        _ = try await performRaw(makeRequest(path: path, method: "POST", body: body))
    }

    private func put<T: Decodable>(path: String, body: some Encodable) async throws -> T {
        try await perform(makeRequest(path: path, method: "PUT", body: body))
    }

    private func makeRequest(path: String, method: String) -> URLRequest {
        guard let url = URL(string: path, relativeTo: baseURL)?.absoluteURL else {
            fatalError("invalid URL")
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        if let token {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return request
    }

    private func makeRequest(path: String, method: String, body: some Encodable) throws -> URLRequest {
        var request = makeRequest(path: path, method: method)
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        do {
            request.httpBody = try encoder.encode(body)
        } catch {
            throw SymvibeError.encoding(error)
        }
        return request
    }

    private func perform<T: Decodable>(_ request: URLRequest) async throws -> T {
        let data = try await performRaw(request)
        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw SymvibeError.decoding(error)
        }
    }

    private func performRaw(_ request: URLRequest) async throws -> Data {
        let (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            throw SymvibeError.transport(error)
        }
        guard let http = response as? HTTPURLResponse else {
            throw SymvibeError.transport(URLError(.badServerResponse))
        }
        guard (200..<300).contains(http.statusCode) else {
            if let server = try? decoder.decode(ServerError.self, from: data) {
                throw SymvibeError.server(server)
            }
            throw SymvibeError.http(status: http.statusCode, body: String(data: data, encoding: .utf8) ?? "")
        }
        return data
    }
}
