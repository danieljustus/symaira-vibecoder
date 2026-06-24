import Foundation

public struct Event: Codable, Sendable, Equatable {
    public let type: String
    public let runID: String?
    public let stepID: String?
    public let status: String?
    public let kind: String?
    public let line: String?
    public let state: String?
    public let ts: Int64?

    public init(
        type: String,
        runID: String? = nil,
        stepID: String? = nil,
        status: String? = nil,
        kind: String? = nil,
        line: String? = nil,
        state: String? = nil,
        ts: Int64? = nil
    ) {
        self.type = type
        self.runID = runID
        self.stepID = stepID
        self.status = status
        self.kind = kind
        self.line = line
        self.state = state
        self.ts = ts
    }
}

public struct RunState: Codable, Sendable, Equatable {
    public let state: String
    public let runID: String?
    public let currentStep: String?
    public let cycle: String?
    public let mode: String?
}
