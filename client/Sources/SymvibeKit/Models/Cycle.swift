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
    public var sensor: String
    public var when: String

    public init(sensor: String, when: String) {
        self.sensor = sensor
        self.when = when
    }
}

/// Declarative per-step risk gate (`requires_review` on the wire). The client
/// does not edit it, but it must round-trip so a full `PUT /api/cycle` does not
/// silently drop the rule.
public struct RequiresReview: Codable, Sendable, Equatable {
    public var when: String

    public init(when: String) {
        self.when = when
    }
}

public struct StepModelOverride: Codable, Sendable, Equatable {
    public var id: String
    public var temperature: Double?
    public var variant: String?
    public var fallbackModels: [String]?

    public init(id: String, temperature: Double? = nil, variant: String? = nil, fallbackModels: [String]? = nil) {
        self.id = id
        self.temperature = temperature
        self.variant = variant
        self.fallbackModels = fallbackModels
    }
}

public struct Step: Codable, Sendable, Identifiable, Equatable {
    public var id: String
    public var name: String
    public var order: Int
    public var skill: String
    public var category: String
    public var agent: String?
    public var promptSuffix: String?
    public var enabled: Bool
    public var modelOverride: StepModelOverride?
    public var backendOverride: String?
    public var autoSkip: AutoSkip?
    public var requiresReview: RequiresReview?
    public var dependsOn: [String]?
    public var parallelSafe: Bool?
    public var status: StepStatus

    public init(
        id: String,
        name: String,
        order: Int,
        skill: String,
        category: String,
        agent: String? = nil,
        promptSuffix: String? = nil,
        enabled: Bool = true,
        modelOverride: StepModelOverride? = nil,
        backendOverride: String? = nil,
        autoSkip: AutoSkip? = nil,
        requiresReview: RequiresReview? = nil,
        dependsOn: [String]? = nil,
        parallelSafe: Bool? = nil,
        status: StepStatus = .pending
    ) {
        self.id = id
        self.name = name
        self.order = order
        self.skill = skill
        self.category = category
        self.agent = agent
        self.promptSuffix = promptSuffix
        self.enabled = enabled
        self.modelOverride = modelOverride
        self.backendOverride = backendOverride
        self.autoSkip = autoSkip
        self.requiresReview = requiresReview
        self.dependsOn = dependsOn
        self.parallelSafe = parallelSafe
        self.status = status
    }

    /// Steps embedded in an exported template carry no runtime status and may
    /// omit most fields, so decode defensively — the same type serves the board
    /// and the import/library payloads.
    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
        name = try c.decodeIfPresent(String.self, forKey: .name) ?? ""
        order = try c.decodeIfPresent(Int.self, forKey: .order) ?? 0
        skill = try c.decodeIfPresent(String.self, forKey: .skill) ?? ""
        category = try c.decodeIfPresent(String.self, forKey: .category) ?? ""
        agent = try c.decodeIfPresent(String.self, forKey: .agent)
        promptSuffix = try c.decodeIfPresent(String.self, forKey: .promptSuffix)
        enabled = try c.decodeIfPresent(Bool.self, forKey: .enabled) ?? true
        modelOverride = try c.decodeIfPresent(StepModelOverride.self, forKey: .modelOverride)
        backendOverride = try c.decodeIfPresent(String.self, forKey: .backendOverride)
        autoSkip = try c.decodeIfPresent(AutoSkip.self, forKey: .autoSkip)
        requiresReview = try c.decodeIfPresent(RequiresReview.self, forKey: .requiresReview)
        dependsOn = try c.decodeIfPresent([String].self, forKey: .dependsOn)
        parallelSafe = try c.decodeIfPresent(Bool.self, forKey: .parallelSafe)
        status = try c.decodeIfPresent(StepStatus.self, forKey: .status) ?? .pending
    }
}

public struct Phase: Codable, Sendable, Identifiable, Equatable {
    public var id: String
    public var name: String
    public var order: Int
    public var steps: [Step]

    public init(id: String, name: String, order: Int, steps: [Step]) {
        self.id = id
        self.name = name
        self.order = order
        self.steps = steps
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
        name = try c.decodeIfPresent(String.self, forKey: .name) ?? ""
        order = try c.decodeIfPresent(Int.self, forKey: .order) ?? 0
        steps = try c.decodeIfPresent([Step].self, forKey: .steps) ?? []
    }
}

public struct Cycle: Codable, Sendable, Identifiable, Equatable {
    public var schemaVersion: Int
    public var id: String
    public var name: String
    public var description: String
    public var phases: [Phase]

    public init(schemaVersion: Int, id: String, name: String, description: String, phases: [Phase]) {
        self.schemaVersion = schemaVersion
        self.id = id
        self.name = name
        self.description = description
        self.phases = phases
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try c.decodeIfPresent(Int.self, forKey: .schemaVersion) ?? 1
        id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
        name = try c.decodeIfPresent(String.self, forKey: .name) ?? ""
        description = try c.decodeIfPresent(String.self, forKey: .description) ?? ""
        phases = try c.decodeIfPresent([Phase].self, forKey: .phases) ?? []
    }

    // MARK: - Lookup helpers

    public func step(id stepID: String) -> Step? {
        for phase in phases {
            if let match = phase.steps.first(where: { $0.id == stepID }) { return match }
        }
        return nil
    }

    public func phase(containing stepID: String) -> Phase? {
        phases.first { $0.steps.contains { $0.id == stepID } }
    }

    /// Replaces the step carrying the same id, in place. No-op if it is unknown.
    public mutating func replace(_ step: Step) {
        for phaseIdx in phases.indices {
            if let stepIdx = phases[phaseIdx].steps.firstIndex(where: { $0.id == step.id }) {
                phases[phaseIdx].steps[stepIdx] = step
                return
            }
        }
    }
}
