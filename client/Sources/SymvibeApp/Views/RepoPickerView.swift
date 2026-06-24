import SwiftUI

/// A view for selecting a working directory (repository) to use with the symvibe engine.
///
/// On macOS, uses `NSOpenPanel` for native folder selection.
/// On iOS, provides a text field fallback for manual path entry.
struct RepoPickerView: View {
    @Binding var selectedDir: String
    let onConfirm: () -> Void

    @State private var recentDirs: [String] = []
    @State private var manualPath: String = ""
    @State private var showManualEntry = false

    private let recentDirsKey = "com.symvibe.recentDirs"

    var body: some View {
        VStack(spacing: 20) {
            // Header
            VStack(spacing: 8) {
                Image(systemName: "folder.badge.gearshape")
                    .font(.system(size: 48))
                    .foregroundStyle(.tint)

                Text("Choose Repository")
                    .font(.title2.bold())

                Text("Select the directory where your cycle should operate.")
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                #if os(macOS)
                    .font(.callout)
                #endif
            }
            .padding(.top, 20)

            // Current selection
            if !selectedDir.isEmpty {
                HStack {
                    Image(systemName: "folder.fill")
                        .foregroundStyle(.secondary)
                    Text(selectedDir)
                        .font(.caption.monospaced())
                        .lineLimit(2)
                        .truncationMode(.middle)
                    Spacer()
                    Button {
                        selectedDir = ""
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundStyle(.secondary)
                    }
                    .buttonStyle(.plain)
                }
                .padding(12)
                .background(Color(.textBackgroundColor))
                .clipShape(RoundedRectangle(cornerRadius: 8))
                .padding(.horizontal)
            }

            // Platform-specific picker
            #if os(macOS)
            macOSPicker
            #else
            iOSFallback
            #endif

            // Recent directories
            if !recentDirs.isEmpty {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Recent")
                        .font(.headline)
                        .padding(.horizontal)

                    ForEach(recentDirs, id: \.self) { dir in
                        Button {
                            selectedDir = dir
                        } label: {
                            HStack {
                                Image(systemName: "clock.arrow.circlepath")
                                    .foregroundStyle(.secondary)
                                    .frame(width: 20)
                                Text(dir)
                                    .font(.caption.monospaced())
                                    .lineLimit(1)
                                    .truncationMode(.middle)
                                Spacer()
                                if dir == selectedDir {
                                    Image(systemName: "checkmark.circle.fill")
                                        .foregroundStyle(.green)
                                }
                            }
                            .padding(.horizontal, 12)
                            .padding(.vertical, 8)
                        }
                        .buttonStyle(.plain)
                        .background(
                            dir == selectedDir
                                ? Color.accentColor.opacity(0.1)
                                : Color(.controlBackgroundColor)
                        )
                        .clipShape(RoundedRectangle(cornerRadius: 6))
                        .padding(.horizontal)
                    }
                }
            }

            Spacer()

            // Confirm button
            Button(action: onConfirm) {
                Label("Continue", systemImage: "arrow.right")
                    .font(.headline)
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(selectedDir.isEmpty ? Color(.controlColor) : Color.accentColor)
                    .foregroundStyle(selectedDir.isEmpty ? .secondary : Color.white)
                    .clipShape(RoundedRectangle(cornerRadius: 12))
            }
            .disabled(selectedDir.isEmpty)
            .padding(.horizontal)
            .padding(.bottom, 20)
        }
    }

    // MARK: - macOS Picker

    #if os(macOS)
    private var macOSPicker: some View {
        VStack(spacing: 12) {
            Button {
                openFolderPicker()
            } label: {
                Label("Browse…", systemImage: "folder.badge.plus")
                    .font(.headline)
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(Color.accentColor.opacity(0.1))
                    .foregroundStyle(Color.accentColor)
                    .clipShape(RoundedRectangle(cornerRadius: 12))
            }
            .buttonStyle(.plain)

            Button {
                showManualEntry = true
            } label: {
                Label("Enter Path Manually", systemImage: "keyboard")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
            .buttonStyle(.plain)
        }
        .padding(.horizontal)
        .sheet(isPresented: $showManualEntry) {
            manualPathSheet
        }
    }

    private func openFolderPicker() {
        let panel = NSOpenPanel()
        panel.title = "Select Repository Directory"
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.canCreateDirectories = false
        panel.allowsMultipleSelection = false
        panel.directoryURL = selectedDir.isEmpty ? nil : URL(fileURLWithPath: selectedDir)

        panel.begin { response in
            guard response == .OK, let url = panel.url else { return }
            let path = url.path
            selectedDir = path
            addToRecent(path)
        }
    }
    #endif

    // MARK: - iOS Fallback

    private var iOSFallback: some View {
        VStack(spacing: 12) {
            Text("Enter the path to your repository directory on your Mac.")
                .font(.callout)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal)

            HStack {
                TextField("/Users/you/project", text: $manualPath)
                    .textFieldStyle(.roundedBorder)
                    .font(.caption.monospaced())
                    .disableAutocorrection(true)
                    .onSubmit {
                        applyManualPath()
                    }

                Button("Apply") {
                    applyManualPath()
                }
                .buttonStyle(.bordered)
                .disabled(manualPath.isEmpty)
            }
            .padding(.horizontal)
        }
    }

    // MARK: - Manual Path Sheet (macOS)

    private var manualPathSheet: some View {
        NavigationStack {
            VStack(spacing: 16) {
                Text("Enter the absolute path to your repository directory.")
                    .foregroundStyle(.secondary)
                    .font(.callout)

                TextField("/Users/you/project", text: $manualPath)
                    .textFieldStyle(.roundedBorder)
                    .font(.system(.body, design: .monospaced))
                    .disableAutocorrection(true)
                    .padding(.horizontal)

                if !manualPath.isEmpty && !FileManager.default.fileExists(atPath: manualPath) {
                    Label("Directory does not exist", systemImage: "exclamationmark.triangle")
                        .foregroundStyle(.orange)
                        .font(.caption)
                }
            }
            .padding()
            .navigationTitle("Path Entry")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { showManualEntry = false }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Apply") {
                        applyManualPath()
                        showManualEntry = false
                    }
                    .disabled(manualPath.isEmpty)
                }
            }
        }
    }

    // MARK: - Helpers

    private func applyManualPath() {
        let trimmed = manualPath.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty else { return }
        selectedDir = trimmed
        addToRecent(trimmed)
    }

    private func addToRecent(_ path: String) {
        recentDirs.removeAll { $0 == path }
        recentDirs.insert(path, at: 0)
        if recentDirs.count > 5 {
            recentDirs = Array(recentDirs.prefix(5))
        }
        UserDefaults.standard.set(recentDirs, forKey: recentDirsKey)
    }
}
