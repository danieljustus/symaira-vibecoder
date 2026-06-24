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
}

public struct StepModelOverride: Codable, Sendable, Equatable {
    public let id: String
    public let temperature: Double?
    public let variant: String?
    public let fallbackModels: [String]?
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
}

public struct Phase: Codable, Sendable, Identifiable, Equatable {
    public let id: String
    public let name: String
    public let order: Int
    public var steps: [Step]
}

public struct Cycle: Codable, Sendable, Identifiable, Equatable {
    public let schemaVersion: Int
    public let id: String
    public let name: String
    public let description: String
    public var phases: [Phase]
}
