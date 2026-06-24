import Foundation
import Observation

/// Collects `log` and `error` SSE events for the currently running step.
///
/// The store is cleared whenever the active step changes or the run ends.
@Observable
@MainActor
public final class ActivityStore {
    public struct LogLine: Identifiable, Sendable, Equatable {
        public let id: UUID
        public let text: String
        public let kind: LogKind
        public let timestamp: Date

        public init(id: UUID = UUID(), text: String, kind: LogKind, timestamp: Date = .now) {
            self.id = id
            self.text = text
            self.kind = kind
            self.timestamp = timestamp
        }
    }

    public enum LogKind: String, Sendable, CaseIterable {
        case log
        case error
    }

    public private(set) var lines: [LogLine] = []
    public private(set) var currentStepID: String?

    private let maxLines = 500

    public init() {}

    // MARK: - Public

    /// Append a new log/error event. Automatically filters by the active step.
    public func append(event: Event) {
        guard let line = event.line, !line.isEmpty else { return }
        guard let stepID = event.stepID else { return }

        // Switch context when step changes
        if stepID != currentStepID {
            currentStepID = stepID
            lines.removeAll()
        }

        let kind: LogKind = event.type == "error" ? .error : .log
        let entry = LogLine(text: line, kind: kind)
        lines.append(entry)

        // Trim to max
        if lines.count > maxLines {
            lines.removeFirst(lines.count - maxLines)
        }
    }

    /// Clear all activity. Called when run ends or board resets.
    public func clear() {
        lines.removeAll()
        currentStepID = nil
    }

    /// Clear only if the given step matches the current step.
    public func clearIfCurrent(stepID: String) {
        if currentStepID == stepID {
            clear()
        }
    }
}
