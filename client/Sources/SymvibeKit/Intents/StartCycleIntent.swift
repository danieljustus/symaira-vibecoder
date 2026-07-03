import AppIntents

public struct StartCycleIntent: AppIntent {
    public init() {}

    public static let title: LocalizedStringResource = "Start Cycle"
    public static let description = IntentDescription(
        "Starts the symvibe cycle run on the connected server."
    )
    public static var isDiscoverable: Bool { true }

    public func perform() async throws -> some IntentResult {
        guard let client = WidgetShared.makeAPIClient() else {
            throw StartCycleIntentError.notConnected
        }
        do {
            try await client.runCycle()
            return .result()
        } catch {
            throw StartCycleIntentError.runFailed(error.localizedDescription)
        }
    }
}

enum StartCycleIntentError: Swift.Error, CustomLocalizedStringResourceConvertible {
    case notConnected
    case runFailed(String)

    var localizedStringResource: LocalizedStringResource {
        switch self {
        case .notConnected:
            "Not connected to a symvibe server."
        case .runFailed(let detail):
            "Run failed: \(detail)"
        }
    }
}
