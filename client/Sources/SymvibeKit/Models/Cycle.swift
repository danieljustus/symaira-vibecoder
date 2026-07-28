import Foundation

public enum StepStatus: String, Codable, Sendable, CaseIterable {
    case pending
    case inProgress = "in_progress"
    case done
    case skipped
    case failed
    case blocked
    case needsReview = "needs_review"

    public var effective: StepStatus { self }

    public var isTerminal: Bool {
        self == .done || self == .skipped
    }

    public var isHalting: Bool {
        switch self {
        case .failed, .blocked, .needsReview, .inProgress:
            true
        default:
            false
        }
    }

    /// Status glyph for display (matches README mapping: ○ ◐ ✓ – ✕ ⦸ !).
    public var glyph: String {
        switch self {
        case .pending: "○"
        case .inProgress: "◐"
        case .done: "✓"
        case .skipped: "–"
        case .failed: "✕"
        case .blocked: "⦸"
        case .needsReview: "!"
        }
    }
}

public struct AutoSkip: Codable, Sendable, Equatable {
    public let sensor: String
    public let when: String

    public init(sensor: String, when: String) {
        self.sensor = sensor
        self.when = when
    }
}

public struct StepModelOverride: Codable, Sendable, Equatable {
    public let id: String
    public let temperature: Double?
    public let variant: String?
    public let fallbackModels: [String]?

    public init(id: String, temperature: Double? = nil, variant: String? = nil, fallbackModels: [String]? = nil) {
        self.id = id
        self.temperature = temperature
        self.variant = variant
        self.fallbackModels = fallbackModels
    }
}

public struct Step: Codable, Sendable, Identifiable, Equatable {
    public let id: String
    public let name: String
    public let order: Int
    public var skill: String
    public var category: String
    public let agent: String?
    public let promptSuffix: String?
    public var enabled: Bool
    public var modelOverride: StepModelOverride?
    public let autoSkip: AutoSkip?
    public let dependsOn: [String]?
    public let parallelSafe: Bool?
    public var status: StepStatus

    public init(id: String, name: String, order: Int, skill: String, category: String, agent: String? = nil, promptSuffix: String? = nil, enabled: Bool = true, modelOverride: StepModelOverride? = nil, autoSkip: AutoSkip? = nil, dependsOn: [String]? = nil, parallelSafe: Bool? = nil, status: StepStatus = .pending) {
        self.id = id
        self.name = name
        self.order = order
        self.skill = skill
        self.category = category
        self.agent = agent
        self.promptSuffix = promptSuffix
        self.enabled = enabled
        self.modelOverride = modelOverride
        self.autoSkip = autoSkip
        self.dependsOn = dependsOn
        self.parallelSafe = parallelSafe
        self.status = status
    }
}

public struct Phase: Codable, Sendable, Identifiable, Equatable {
    public let id: String
    public let name: String
    public let order: Int
    public var steps: [Step]

    public init(id: String, name: String, order: Int, steps: [Step]) {
        self.id = id
        self.name = name
        self.order = order
        self.steps = steps
    }
}

public struct Cycle: Codable, Sendable, Identifiable, Equatable {
    public let schemaVersion: Int
    public let id: String
    public let name: String
    public let description: String
    public var phases: [Phase]

    public init(schemaVersion: Int, id: String, name: String, description: String, phases: [Phase]) {
        self.schemaVersion = schemaVersion
        self.id = id
        self.name = name
        self.description = description
        self.phases = phases
    }
}
