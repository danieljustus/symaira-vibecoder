import AppIntents

public struct PauseRunIntent: AppIntent {
    public init() {}

    public static let title: LocalizedStringResource = "Pause Run"
    public static let description = IntentDescription(
        "Pauses the currently running symvibe cycle."
    )
    public static var isDiscoverable: Bool { true }

    public func perform() async throws -> some IntentResult {
        guard let client = WidgetShared.makeAPIClient() else {
            throw RunControlIntentError.notConnected
        }
        do {
            try await client.controlRun(action: "pause")
            return .result()
        } catch {
            throw RunControlIntentError.controlFailed("pause", error.localizedDescription)
        }
    }
}
