import Foundation

/// Shared data layer for WidgetKit and App Intents.
///
/// The main app writes step/status data here; the widget extension and
/// App Intents read it. Uses a shared `UserDefaults` suite so the widget
/// process can access the data without IPC.
public enum WidgetShared {
    /// App Group suite name. Both the main app and the widget extension
    /// must be members of this group for data sharing to work.
    public static let suiteName = "group.dev.symaira.Symvibe"

    public enum Keys {
        public static let currentStepName = "widget.currentStepName"
        public static let currentStepID = "widget.currentStepID"
        public static let currentStepStatus = "widget.currentStepStatus"
        public static let currentPhaseName = "widget.currentPhaseName"
        public static let cycleName = "widget.cycleName"
        public static let runState = "widget.runState"
        public static let lastUpdated = "widget.lastUpdated"
        public static let apiBaseURL = "widget.apiBaseURL"
        public static let apiToken = "widget.apiToken"
        public static let pushEnabled = "widget.pushEnabled"
    }

    /// Shared UserDefaults suite. Returns `nil` if the app group is not
    /// configured (e.g. during `swift build` outside Xcode).
    public static var sharedDefaults: UserDefaults? {
        UserDefaults(suiteName: suiteName)
    }

    // MARK: - Write (called by BoardStore)

    /// Write the currently active step info for the widget to display.
    public static func writeCurrentStep(
        name: String,
        stepID: String,
        status: String,
        phaseName: String,
        cycleName: String,
        runState: String
    ) {
        guard let defaults = sharedDefaults else { return }
        defaults.set(name, forKey: Keys.currentStepName)
        defaults.set(stepID, forKey: Keys.currentStepID)
        defaults.set(status, forKey: Keys.currentStepStatus)
        defaults.set(phaseName, forKey: Keys.currentPhaseName)
        defaults.set(cycleName, forKey: Keys.cycleName)
        defaults.set(runState, forKey: Keys.runState)
        defaults.set(Date().timeIntervalSince1970, forKey: Keys.lastUpdated)
    }

    /// Clear all widget data (called on disconnect).
    public static func clearAll() {
        guard let defaults = sharedDefaults else { return }
        for key in [
            Keys.currentStepName, Keys.currentStepID, Keys.currentStepStatus,
            Keys.currentPhaseName, Keys.cycleName, Keys.runState, Keys.lastUpdated,
        ] {
            defaults.removeObject(forKey: key)
        }
    }

    // MARK: - Connection Info (for App Intents)

    /// Store the active connection's base URL and token so App Intents
    /// can reconstruct an `APIClient` independently.
    public static func writeConnection(baseURL: URL, token: String?) {
        guard let defaults = sharedDefaults else { return }
        defaults.set(baseURL.absoluteString, forKey: Keys.apiBaseURL)
        if let token {
            defaults.set(token, forKey: Keys.apiToken)
        } else {
            defaults.removeObject(forKey: Keys.apiToken)
        }
    }

    /// Clear stored connection info.
    public static func clearConnection() {
        guard let defaults = sharedDefaults else { return }
        defaults.removeObject(forKey: Keys.apiBaseURL)
        defaults.removeObject(forKey: Keys.apiToken)
    }

    // MARK: - Read (used by Widget & Intents)

    /// Attempt to reconstruct an `APIClient` from the stored connection info.
    /// Returns `nil` if no connection is stored or the URL is invalid.
    public static func makeAPIClient() -> APIClient? {
        guard let defaults = sharedDefaults,
              let urlString = defaults.string(forKey: Keys.apiBaseURL),
              let baseURL = URL(string: urlString)
        else { return nil }
        let token = defaults.string(forKey: Keys.apiToken)
        return APIClient(baseURL: baseURL, token: token)
    }

    // MARK: - Push

    /// Whether the user has enabled push notifications.
    public static var isPushEnabled: Bool {
        get { sharedDefaults?.bool(forKey: Keys.pushEnabled) ?? false }
        set { sharedDefaults?.set(newValue, forKey: Keys.pushEnabled) }
    }
}
