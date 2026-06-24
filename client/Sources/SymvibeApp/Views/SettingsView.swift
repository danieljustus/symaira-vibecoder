import SwiftUI
import SymvibeKit

struct SettingsView: View {
    let connectionStore: ConnectionStore

    @State private var showDisconnectAlert = false
    @State private var pushManager = PushManager()

    var body: some View {
        Form {
            Section("Account") {
                if let profile = connectionStore.activeProfile {
                    LabeledContent("Connected To", value: profile.name)

                    if profile.isDemo {
                        LabeledContent("Mode", value: "Demo")
                    }

                    Button("Disconnect") {
                        showDisconnectAlert = true
                    }
                    .foregroundStyle(.red)
                } else {
                    Text("Not connected")
                        .foregroundStyle(.secondary)
                }
            }

            Section("Notifications") {
                Toggle(isOn: Binding(
                    get: { pushManager.isEnabled },
                    set: { pushManager.isEnabled = $0 }
                )) {
                    Label("Push Notifications", systemImage: "bell.badge")
                }

                if pushManager.isEnabled {
                    if pushManager.isAuthorized {
                        LabeledContent("Status", value: "Authorized")
                    } else {
                        LabeledContent("Status", value: "Not Authorized")
                            .foregroundStyle(.orange)
                    }

                    if let token = pushManager.deviceTokenHex {
                        LabeledContent("Device Token", value: String(token.prefix(16)) + "…")
                            .font(.caption.monospaced())
                    }
                }

                Text("Receive push notifications when a step completes, fails, or needs review. Requires a server-side push relay (coming soon).")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section("Account Deletion") {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Account deletion will be available in a future update when account login is supported.")
                        .font(.callout)
                        .foregroundStyle(.secondary)

                    Button("Delete Account") {}
                        .disabled(true)
                        .foregroundStyle(.red)
                }
            }

            Section("About") {
                LabeledContent("App", value: "symvibe")
                LabeledContent("Version", value: Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "—")
                LabeledContent("Build", value: Bundle.main.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "—")
            }
        }
        .navigationTitle("Settings")
        .alert("Disconnect", isPresented: $showDisconnectAlert) {
            Button("Cancel", role: .cancel) {}
            Button("Disconnect", role: .destructive) {
                connectionStore.setActive(nil)
            }
        } message: {
            Text("This will remove the current connection. You can reconnect later.")
        }
    }
}
