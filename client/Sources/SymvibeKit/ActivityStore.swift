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

    /// Highest event `ts` already ingested. Replay merges skip anything at or
    /// below it, so a reconnect can never re-append lines the store has seen
    /// (including lines from a previous step that were cleared).
    public private(set) var lastSeenTS: Int64 = 0

    private let maxLines = 500

    public init() {}

    // MARK: - Public

    /// Append a new log/error event. Automatically filters by the active step.
    public func append(event: Event) {
        if let ts = event.ts, ts > lastSeenTS {
            lastSeenTS = ts
        }
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

    /// Merge a backfilled log snapshot (from `GET /api/logs`) into the store.
    /// Entries already seen (by `ts`) are skipped; the rest are appended in
    /// server order, which re-establishes the active step exactly like live
    /// events would. Call once after each (re)connect.
    public func mergeReplay(_ events: [Event]) {
        for event in events {
            // Skip anything already ingested (at or below the highest ts seen
            // so far) — including lines from a previous step that were cleared.
            if let ts = event.ts, ts <= lastSeenTS {
                continue
            }
            append(event: event)
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
