import AppIntents

public struct ResumeRunIntent: AppIntent {
    public init() {}

    public static let title: LocalizedStringResource = "Resume Run"
    public static let description = IntentDescription(
        "Resumes the paused symvibe cycle run."
    )
    public static var isDiscoverable: Bool { true }

    public func perform() async throws -> some IntentResult {
        guard let client = WidgetShared.makeAPIClient() else {
            throw RunControlIntentError.notConnected
        }
        do {
            try await client.controlRun(action: "resume")
            return .result()
        } catch {
            throw RunControlIntentError.controlFailed("resume", error.localizedDescription)
        }
    }
}

enum RunControlIntentError: Swift.Error, CustomLocalizedStringResourceConvertible {
    case notConnected
    case controlFailed(String, String)

    var localizedStringResource: LocalizedStringResource {
        switch self {
        case .notConnected:
            "Not connected to a symvibe server."
        case .controlFailed(let action, let detail):
            "Could not \(action): \(detail)"
        }
    }
}
