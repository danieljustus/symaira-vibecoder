import SwiftUI
import SymvibeKit

struct DoctorView: View {
    let apiClient: APIClient?

    @State private var doctor: DoctorResponse?
    @State private var version: VersionResponse?
    @State private var isLoading = true
    @State private var error: String?

    var body: some View {
        List {
            if let doctor {
                Section("Server Status") {
                    DoctorRow(label: "opencode", ok: doctor.opencodeOk)
                    DoctorRow(label: "git", ok: doctor.git)
                    DoctorRow(label: "gh", ok: doctor.gh)
                    DoctorRow(label: "Runnable", ok: doctor.runnable)
                }

                Section("opencode") {
                    LabeledContent("Version", value: doctor.opencode.version ?? "—")
                    LabeledContent("Path", value: doctor.opencode.path ?? "—")
                    if let detail = doctor.opencode.detail {
                        LabeledContent("Detail", value: detail)
                    }
                }

                if let hints = doctor.hints, !hints.isEmpty {
                    Section("Hints") {
                        ForEach(hints.sorted(by: { $0.key < $1.key }), id: \.key) { key, value in
                            VStack(alignment: .leading, spacing: 2) {
                                Text(key)
                                    .font(.caption.weight(.semibold))
                                Text(value)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }

            if let version {
                Section("Version") {
                    LabeledContent("Server", value: version.serverVersion)
                    LabeledContent("API", value: version.apiVersion)
                    LabeledContent("Go", value: version.goVersion)
                    LabeledContent("Platform", value: version.platform)
                }

                if !version.capabilities.isEmpty {
                    Section("Capabilities") {
                        ForEach(version.capabilities, id: \.self) { cap in
                            Text(cap)
                                .font(.caption.monospaced())
                        }
                    }
                }
            }

            if isLoading {
                Section {
                    HStack {
                        Spacer()
                        ProgressView()
                        Spacer()
                    }
                }
            }

            if let error {
                Section {
                    Text(error)
                        .foregroundStyle(.red)
                }
            }
        }
        .navigationTitle("Doctor")
        .task {
            await load()
        }
    }

    private func load() async {
        guard let client = apiClient else {
            self.error = "Not connected to server"
            isLoading = false
            return
        }

        do {
            async let d: DoctorResponse = client.doctor()
            async let v: VersionResponse = client.version()
            self.doctor = try await d
            self.version = try await v
        } catch {
            self.error = error.localizedDescription
        }
        isLoading = false
    }
}

// MARK: - Doctor Row

private struct DoctorRow: View {
    let label: String
    let ok: Bool

    var body: some View {
        HStack {
            Text(label)
            Spacer()
            Image(systemName: ok ? "checkmark.circle.fill" : "xmark.circle.fill")
                .foregroundStyle(ok ? .green : .red)
        }
    }
}
