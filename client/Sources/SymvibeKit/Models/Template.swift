import Foundation

/// Wire format of an exported cycle (`GET /api/cycle/export`) and the payload
/// accepted by `POST /api/cycle/import`.
public struct Template: Codable, Sendable, Equatable {
    public static let expectedKind = "symvibe.template"

    public var kind: String
    public var schemaVersion: Int
    public var manifest: TemplateManifest
    public var requires: TemplateRequires
    public var phases: [Phase]

    public init(
        kind: String = Template.expectedKind,
        schemaVersion: Int = 1,
        manifest: TemplateManifest,
        requires: TemplateRequires = TemplateRequires(),
        phases: [Phase]
    ) {
        self.kind = kind
        self.schemaVersion = schemaVersion
        self.manifest = manifest
        self.requires = requires
        self.phases = phases
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        kind = try c.decodeIfPresent(String.self, forKey: .kind) ?? ""
        schemaVersion = try c.decodeIfPresent(Int.self, forKey: .schemaVersion) ?? 1
        manifest = try c.decodeIfPresent(TemplateManifest.self, forKey: .manifest) ?? TemplateManifest(id: "", name: "")
        requires = try c.decodeIfPresent(TemplateRequires.self, forKey: .requires) ?? TemplateRequires()
        phases = try c.decodeIfPresent([Phase].self, forKey: .phases) ?? []
    }

    public var isValidKind: Bool { kind == Template.expectedKind }
}

public struct TemplateManifest: Codable, Sendable, Equatable {
    public var id: String
    public var name: String
    public var version: String
    public var author: String
    public var tags: [String]
    public var description: String

    public init(
        id: String,
        name: String,
        version: String = "1.0.0",
        author: String = "symvibe",
        tags: [String] = [],
        description: String = ""
    ) {
        self.id = id
        self.name = name
        self.version = version
        self.author = author
        self.tags = tags
        self.description = description
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
        name = try c.decodeIfPresent(String.self, forKey: .name) ?? ""
        version = try c.decodeIfPresent(String.self, forKey: .version) ?? "1.0.0"
        author = try c.decodeIfPresent(String.self, forKey: .author) ?? ""
        tags = try c.decodeIfPresent([String].self, forKey: .tags) ?? []
        description = try c.decodeIfPresent(String.self, forKey: .description) ?? ""
    }
}

public struct TemplateRequires: Codable, Sendable, Equatable {
    public var skills: [String]?
    public var categories: [String]?
    public var agents: [String]?
    public var sensors: [String]?

    public init(skills: [String]? = nil, categories: [String]? = nil, agents: [String]? = nil, sensors: [String]? = nil) {
        self.skills = skills
        self.categories = categories
        self.agents = agents
        self.sensors = sensors
    }
}

/// Locally available building blocks, returned alongside a failed import.
public struct Catalog: Codable, Sendable, Equatable {
    public var skills: [String]
    public var categories: [String]
    public var agents: [String]
    public var sensors: [String]

    public init(skills: [String] = [], categories: [String] = [], agents: [String] = [], sensors: [String] = []) {
        self.skills = skills
        self.categories = categories
        self.agents = agents
        self.sensors = sensors
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        skills = try c.decodeIfPresent([String].self, forKey: .skills) ?? []
        categories = try c.decodeIfPresent([String].self, forKey: .categories) ?? []
        agents = try c.decodeIfPresent([String].self, forKey: .agents) ?? []
        sensors = try c.decodeIfPresent([String].self, forKey: .sensors) ?? []
    }
}

/// Requirements a template needs that this machine does not provide.
public struct MissingRequirements: Codable, Sendable, Equatable {
    public var skills: [String]
    public var categories: [String]
    public var agents: [String]
    public var sensors: [String]

    public init(skills: [String] = [], categories: [String] = [], agents: [String] = [], sensors: [String] = []) {
        self.skills = skills
        self.categories = categories
        self.agents = agents
        self.sensors = sensors
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        skills = try c.decodeIfPresent([String].self, forKey: .skills) ?? []
        categories = try c.decodeIfPresent([String].self, forKey: .categories) ?? []
        agents = try c.decodeIfPresent([String].self, forKey: .agents) ?? []
        sensors = try c.decodeIfPresent([String].self, forKey: .sensors) ?? []
    }

    public var isEmpty: Bool {
        skills.isEmpty && categories.isEmpty && agents.isEmpty && sensors.isEmpty
    }

    /// Only categories can be remapped by the server (`ApplyRemap`); everything
    /// else has to be installed locally first.
    public var isRemappable: Bool {
        !categories.isEmpty && skills.isEmpty && agents.isEmpty && sensors.isEmpty
    }
}

/// Body returned by `POST /api/cycle/import` when the template needs building
/// blocks the local machine does not provide.
public struct MissingRequirementsResponse: Codable, Sendable, Equatable {
    public var error: String
    public var missing: MissingRequirements
    public var available: Catalog

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        error = try c.decodeIfPresent(String.self, forKey: .error) ?? ""
        missing = try c.decodeIfPresent(MissingRequirements.self, forKey: .missing) ?? MissingRequirements()
        available = try c.decodeIfPresent(Catalog.self, forKey: .available) ?? Catalog()
    }
}

/// A single entry in the community template library (`GET /api/library/index`).
public struct LibraryEntry: Codable, Sendable, Equatable, Identifiable {
    public var id: String
    public var name: String
    public var author: String
    public var tags: [String]
    public var description: String
    public var url: String

    public init(id: String, name: String, author: String = "", tags: [String] = [], description: String = "", url: String) {
        self.id = id
        self.name = name
        self.author = author
        self.tags = tags
        self.description = description
        self.url = url
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
        name = try c.decodeIfPresent(String.self, forKey: .name) ?? ""
        author = try c.decodeIfPresent(String.self, forKey: .author) ?? ""
        tags = try c.decodeIfPresent([String].self, forKey: .tags) ?? []
        description = try c.decodeIfPresent(String.self, forKey: .description) ?? ""
        url = try c.decodeIfPresent(String.self, forKey: .url) ?? ""
    }
}
