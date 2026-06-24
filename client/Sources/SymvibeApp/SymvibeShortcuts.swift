import AppIntents
import SymvibeKit

struct SymvibeShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: StartCycleIntent(),
            phrases: [
                "Start cycle on \(.applicationName)",
                "Run cycle on \(.applicationName)",
                "Start \(.applicationName) cycle",
            ],
            shortTitle: "Start Cycle",
            systemImageName: "play.fill"
        )

        AppShortcut(
            intent: PauseRunIntent(),
            phrases: [
                "Pause \(.applicationName)",
                "Pause the cycle on \(.applicationName)",
            ],
            shortTitle: "Pause Run",
            systemImageName: "pause.fill"
        )

        AppShortcut(
            intent: ResumeRunIntent(),
            phrases: [
                "Resume \(.applicationName)",
                "Resume the cycle on \(.applicationName)",
            ],
            shortTitle: "Resume Run",
            systemImageName: "play.fill"
        )
    }
}
