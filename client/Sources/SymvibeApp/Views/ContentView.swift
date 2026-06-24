import SwiftUI
import SymvibeKit

struct ContentView: View {
    @State private var connectionStore = ConnectionStore()
    @State private var boardStore: BoardStore?
    @State private var activityStore = ActivityStore()
    #if os(macOS)
    @State private var engineManager = EngineManager()
    #endif

    var body: some View {
        Group {
            if let boardStore {
                #if os(macOS)
                MainTabView(
                    store: boardStore,
                    activityStore: activityStore,
                    connectionStore: connectionStore,
                    engineManager: engineManager
                )
                #else
                MainTabView(
                    store: boardStore,
                    activityStore: activityStore,
                    connectionStore: connectionStore
                )
                #endif
            } else {
                OnboardingView(store: connectionStore)
            }
        }
        .task(id: connectionStore.activeProfileID) {
            if connectionStore.activeProfileID != nil {
                let store = BoardStore(connectionStore: connectionStore)
                store.activityStore = activityStore
                boardStore = store
                await store.connect()
            } else {
                boardStore?.disconnect()
                boardStore = nil
                activityStore.clear()
            }
        }
    }
}

// MARK: - Main Tab View

#if os(macOS)
struct MainTabView: View {
    let store: BoardStore
    let activityStore: ActivityStore
    let connectionStore: ConnectionStore
    @Bindable var engineManager: EngineManager

    var body: some View {
        TabView {
            NavigationStack {
                BoardView(store: store, activityStore: activityStore)
            }
            .tabItem {
                Label("Board", systemImage: "rectangle.stack")
            }

            NavigationStack {
                DoctorView(apiClient: store.client)
            }
            .tabItem {
                Label("Doctor", systemImage: "stethoscope")
            }

            NavigationStack {
                EngineView(engineManager: engineManager, connectionStore: connectionStore)
            }
            .tabItem {
                Label("Engine", systemImage: "gearshape.2")
            }
        }
        .frame(minWidth: 700, minHeight: 500)
    }
}
#else
struct MainTabView: View {
    let store: BoardStore
    let activityStore: ActivityStore
    let connectionStore: ConnectionStore

    var body: some View {
        TabView {
            NavigationStack {
                BoardView(store: store, activityStore: activityStore)
            }
            .tabItem {
                Label("Board", systemImage: "rectangle.stack")
            }

            NavigationStack {
                DoctorView(apiClient: store.client)
            }
            .tabItem {
                Label("Doctor", systemImage: "stethoscope")
            }
        }
    }
}
#endif

// MARK: - Engine View (macOS only)

#if os(macOS)
struct EngineView: View {
    @Bindable var engineManager: EngineManager
    let connectionStore: ConnectionStore

    @State private var selectedDir: String = ""
    @State private var showRepoPicker = false
    @State private var showPairingQR = false
    @State private var enginePort: Int?

    var body: some View {
        List {
            // Engine Status
            Section("Engine") {
                HStack {
                    Circle()
                        .fill(statusColor)
                        .frame(width: 10, height: 10)
                    Text(statusText)
                        .font(.headline)
                    Spacer()
                    if engineManager.isRunning {
                        Button("Stop") {
                            engineManager.stop()
                            enginePort = nil
                        }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                    } else {
                        Button("Start") {
                            Task { await startEngine() }
                        }
                        .buttonStyle(.borderedProminent)
                        .controlSize(.small)
                        .disabled(selectedDir.isEmpty)
                    }
                }

                if let port = enginePort {
                    LabeledContent("Port", value: "\(port)")
                }
            }

            // Working Directory
            Section("Working Directory") {
                if selectedDir.isEmpty {
                    Button {
                        showRepoPicker = true
                    } label: {
                        Label("Select Repository…", systemImage: "folder.badge.plus")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.bordered)
                } else {
                    HStack {
                        Image(systemName: "folder.fill")
                            .foregroundStyle(.secondary)
                        Text(selectedDir)
                            .font(.caption.monospaced())
                            .lineLimit(1)
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

                    Button {
                        showRepoPicker = true
                    } label: {
                        Text("Change Directory")
                            .font(.caption)
                    }
                    .buttonStyle(.plain)
                    .foregroundStyle(Color.accentColor)
                }
            }

            // Pairing
            if engineManager.isRunning {
                Section("Pairing") {
                    Button {
                        showPairingQR = true
                    } label: {
                        Label("Show Pairing QR Code", systemImage: "qrcode")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.bordered)
                }
            }

            // Engine Logs
            Section("Logs") {
                if engineManager.logs.isEmpty {
                    Text("No logs yet.")
                        .foregroundStyle(.secondary)
                        .font(.caption)
                } else {
                    ScrollViewReader { proxy in
                        ScrollView([.horizontal, .vertical]) {
                            LazyVStack(alignment: .leading, spacing: 2) {
                                ForEach(Array(engineManager.logs.enumerated()), id: \.offset) { _, line in
                                    Text(line)
                                        .font(.caption2.monospaced())
                                        .foregroundStyle(line.contains("ERROR") ? .red : .primary)
                                        .textSelection(.enabled)
                                }
                            }
                            .padding(8)
                        }
                        .frame(minHeight: 120)
                    }
                }
            }
        }
        .navigationTitle("Engine")
        .sheet(isPresented: $showRepoPicker) {
            RepoPickerView(selectedDir: $selectedDir) {
                showRepoPicker = false
            }
            .frame(minWidth: 500, minHeight: 400)
        }
        .sheet(isPresented: $showPairingQR) {
            if let port = enginePort, let url = URL(string: "https://localhost:\(port)") {
                PairingQRView(serverURL: url, deviceName: Host.current().localizedName ?? "Mac")
                    .frame(minWidth: 400, minHeight: 500)
            }
        }
    }

    private var statusColor: Color {
        switch engineManager.state {
        case .stopped: .secondary
        case .starting: .orange
        case .running: .green
        case .failed: .red
        }
    }

    private var statusText: String {
        switch engineManager.state {
        case .stopped: "Stopped"
        case .starting: "Starting…"
        case .running: "Running"
        case .failed(let msg): "Failed: \(msg)"
        }
    }

    private func startEngine() async {
        guard !selectedDir.isEmpty else { return }
        await engineManager.start(dir: selectedDir)
        if let port = engineManager.port {
            enginePort = port
        }
    }
}
#endif
