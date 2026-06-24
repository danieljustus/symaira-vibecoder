import Foundation

public struct RegistryModel: Codable, Sendable, Equatable {
    public let id: String
    public let temperature: Double
    public let variant: String?
    public let fallbackModels: [String]?
}

public struct CategoryBinding: Codable, Sendable, Equatable {
    public let modelRef: String
    public let temperature: Double?
    public let variant: String?
    public let fallbackModels: [String]?
}

public struct ModelsResponse: Codable, Sendable, Equatable {
    public let registry: [String: RegistryModel]
    public let categories: [String: CategoryBinding]
    public let defaultCategory: String
    public let discovered: [DiscoveredModel]
    public let agents: [Agent]
}

public struct DiscoveredModel: Codable, Sendable, Equatable {
    public let id: String
    public let name: String?
}

public struct Agent: Codable, Sendable, Equatable {
    public let id: String
    public let name: String?
}

public struct CategoriesResponse: Codable, Sendable, Equatable {
    public let categories: [String: CategoryBinding]
    public let defaultCategory: String
}

public struct SkillsResponse: Codable, Sendable, Equatable {
    public let skills: [Skill]
}

public struct Skill: Codable, Sendable, Equatable {
    public let name: String
    public let description: String?
}

public struct DoctorResponse: Codable, Sendable, Equatable {
    public let opencode: RunnerInfo
    public let opencodeOk: Bool
    public let git: Bool
    public let gh: Bool
    public let runnable: Bool
    public let hints: [String: String]?
}

public struct RunnerInfo: Codable, Sendable, Equatable {
    public let name: String
    public let version: String?
    public let path: String?
    public let detail: String?
}

public struct VersionResponse: Codable, Sendable, Equatable {
    public let apiVersion: String
    public let serverVersion: String
    public let capabilities: [String]
    public let goVersion: String
    public let platform: String
}
