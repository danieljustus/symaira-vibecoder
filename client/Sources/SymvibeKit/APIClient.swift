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

    // MARK: - Cycle structure

    /// Appends a step to a phase. Returns the server-assigned step id.
    @discardableResult
    public func addStep(phaseID: String, step: Step) async throws -> String {
        struct Body: Encodable {
            let phase_id: String
            let step: Step
        }
        struct Response: Decodable { let id: String }
        let resp: Response = try await post(path: "/api/cycle/step", body: Body(phase_id: phaseID, step: step))
        return resp.id
    }

    public func deleteStep(_ stepID: String) async throws {
        _ = try await performRaw(makeRequest(path: "/api/cycle/step/\(encode(stepID))", method: "DELETE"))
    }

    public func moveStep(_ stepID: String, toPhaseID: String, toIndex: Int) async throws {
        struct Body: Encodable {
            let to_phase_id: String
            let to_index: Int
        }
        try await postVoid(
            path: "/api/cycle/step/\(encode(stepID))/move",
            body: Body(to_phase_id: toPhaseID, to_index: toIndex)
        )
    }

    /// Duplicates a step in place. Returns the new step's id.
    @discardableResult
    public func duplicateStep(_ stepID: String) async throws -> String {
        struct Response: Decodable { let id: String }
        let resp: Response = try await perform(
            makeRequest(path: "/api/cycle/step/\(encode(stepID))/duplicate", method: "POST")
        )
        return resp.id
    }

    /// Appends a phase. Returns the server-assigned phase id.
    @discardableResult
    public func addPhase(name: String) async throws -> String {
        struct Body: Encodable { let name: String }
        struct Response: Decodable { let id: String }
        let resp: Response = try await post(path: "/api/cycle/phase", body: Body(name: name))
        return resp.id
    }

    public func deletePhase(_ phaseID: String) async throws {
        _ = try await performRaw(makeRequest(path: "/api/cycle/phase/\(encode(phaseID))", method: "DELETE"))
    }

    // MARK: - Import / Export / Assist

    /// Exports the active cycle as a template. Returns both the decoded value
    /// and the raw JSON so it can be written to disk byte-for-byte.
    public func exportCycle(id: String? = nil) async throws -> (template: Template, json: Data) {
        var path = "/api/cycle/export"
        if let id, !id.isEmpty {
            path += "?id=\(encode(id))"
        }
        let data = try await performRaw(makeRequest(path: path, method: "GET"))
        do {
            return (try decoder.decode(Template.self, from: data), data)
        } catch {
            throw SymvibeError.decoding(error)
        }
    }

    /// Imports a template, replacing the active cycle. `remap` maps template
    /// category names onto local ones. Throws `.missingRequirements` when the
    /// server rejects the template for unmet requirements.
    @discardableResult
    public func importCycle(template: Template, remap: [String: String] = [:]) async throws -> Cycle {
        struct Body: Encodable {
            let template: Template
            let remap: [String: String]
        }
        return try await perform(
            makeRequest(path: "/api/cycle/import", method: "POST", body: Body(template: template, remap: remap))
        )
    }

    /// Asks the configured coding agent to rewrite the cycle. Long-running:
    /// it drives a full runner invocation server-side.
    public func assistCycle(cycle: Cycle, instruction: String) async throws -> Cycle {
        struct Body: Encodable {
            let cycle: Cycle
            let instruction: String
        }
        return try await perform(
            makeRequest(
                path: "/api/cycle/assist",
                method: "POST",
                body: Body(cycle: cycle, instruction: instruction),
                timeout: 600
            )
        )
    }

    // MARK: - Library

    public func libraryIndex() async throws -> [LibraryEntry] {
        try await get(path: "/api/library/index")
    }

    /// Downloads a template published in the library index.
    public func fetchLibraryTemplate(from urlString: String) async throws -> Template {
        guard let url = URL(string: urlString), url.scheme == "https" else {
            throw SymvibeError.invalidURL
        }
        var request = URLRequest(url: url)
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        let data = try await performRaw(request)
        do {
            return try decoder.decode(Template.self, from: data)
        } catch {
            throw SymvibeError.decoding(error)
        }
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

    /// Percent-encodes a value destined for a single URL path segment or query
    /// value. Step and phase ids are server-generated slugs, but they still must
    /// not be able to escape their segment.
    private func encode(_ value: String) -> String {
        value.addingPercentEncoding(withAllowedCharacters: .alphanumerics.union(CharacterSet(charactersIn: "-._~"))) ?? value
    }

    private func makeRequest(path: String, method: String, timeout: TimeInterval? = nil) -> URLRequest {
        guard let url = URL(string: path, relativeTo: baseURL)?.absoluteURL else {
            fatalError("invalid URL")
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        if let timeout {
            request.timeoutInterval = timeout
        }
        if let token {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return request
    }

    private func makeRequest(path: String, method: String, body: some Encodable, timeout: TimeInterval? = nil) throws -> URLRequest {
        var request = makeRequest(path: path, method: method, timeout: timeout)
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
            // A rejected import carries a structured catalog diff alongside its
            // "error" message, so it has to be matched before the generic shape.
            if let detail = try? decoder.decode(MissingRequirementsResponse.self, from: data), !detail.missing.isEmpty {
                throw SymvibeError.missingRequirements(missing: detail.missing, available: detail.available)
            }
            if let server = try? decoder.decode(ServerError.self, from: data) {
                throw SymvibeError.server(server)
            }
            throw SymvibeError.http(status: http.statusCode, body: String(data: data, encoding: .utf8) ?? "")
        }
        return data
    }
}
