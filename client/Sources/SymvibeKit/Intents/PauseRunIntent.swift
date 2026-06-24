import AppIntents

struct PauseRunIntent: AppIntent {
    static let title: LocalizedStringResource = "Pause Run"
    static let description = IntentDescription(
        "Pauses the currently running symvibe cycle."
    )
    static var isDiscoverable: Bool { true }

    func perform() async throws -> some IntentResult {
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
