import SwiftUI
import UniformTypeIdentifiers
import SymvibeKit

extension Color {
    /// Cross-platform separator color (`.separator` alone resolves to the
    /// SwiftUI `SeparatorShapeStyle` instead of the system color).
    static var cardSeparator: Color {
        #if os(macOS)
        Color(nsColor: .separatorColor)
        #else
        Color(uiColor: .separator)
        #endif
    }

    /// Cross-platform card background color.
    static var cardBackground: Color {
        #if os(macOS)
        Color(nsColor: .controlBackgroundColor)
        #else
        Color(uiColor: .systemBackground)
        #endif
    }

    static func statusColor(_ status: StepStatus) -> Color {
        switch status {
        case .pending: .secondary
        case .inProgress: .blue
        case .done: .green
        case .skipped: .secondary
        case .failed: .red
        case .blocked: .orange
        case .needsReview: .yellow
        }
    }
}

// MARK: - Toast

struct Toast: Identifiable, Equatable {
    enum Kind { case info, success, error }
    let id = UUID()
    let message: String
    let kind: Kind
}

/// Transient status messages — the native counterpart of the web board's
/// `showToast()`.
@Observable
@MainActor
final class ToastCenter {
    private(set) var current: Toast?
    @ObservationIgnored private var dismissTask: Task<Void, Never>?

    func show(_ message: String, kind: Toast.Kind = .info) {
        current = Toast(message: message, kind: kind)
        dismissTask?.cancel()
        dismissTask = Task { [weak self] in
            try? await Task.sleep(for: .seconds(kind == .error ? 6 : 3))
            guard !Task.isCancelled else { return }
            self?.current = nil
        }
    }

    /// Convenience for the `String?`-returning store operations.
    func report(_ error: String?, success: String) {
        if let error {
            show(error, kind: .error)
        } else {
            show(success, kind: .success)
        }
    }

    func dismiss() {
        dismissTask?.cancel()
        current = nil
    }
}

private struct ToastOverlay: View {
    let toast: Toast
    let onDismiss: () -> Void

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: icon)
                .foregroundStyle(tint)
            Text(toast.message)
                .font(.callout)
                .lineLimit(3)
            Spacer(minLength: 0)
            Button {
                onDismiss()
            } label: {
                Image(systemName: "xmark")
                    .font(.caption)
            }
            .buttonStyle(.plain)
            .foregroundStyle(.secondary)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
        .frame(maxWidth: 480)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 10))
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .strokeBorder(tint.opacity(0.35), lineWidth: 1)
        )
        .shadow(radius: 8, y: 2)
        .padding(.bottom, 16)
        .transition(.move(edge: .bottom).combined(with: .opacity))
    }

    private var icon: String {
        switch toast.kind {
        case .info: "info.circle.fill"
        case .success: "checkmark.circle.fill"
        case .error: "exclamationmark.triangle.fill"
        }
    }

    private var tint: Color {
        switch toast.kind {
        case .info: .blue
        case .success: .green
        case .error: .red
        }
    }
}

// MARK: - Board View

struct BoardView: View {
    let store: BoardStore
    let activityStore: ActivityStore

    @State private var toasts = ToastCenter()
    @State private var metadata = BoardMetadata()
    @State private var sheet: BoardSheet?
    @State private var pendingDeletePhase: Phase?
    @State private var showLogConsole = true

    var body: some View {
        boardBody
            .navigationTitle(store.cycle?.name ?? "Board")
            .toolbar { boardToolbar }
            .overlay(alignment: .bottom) {
                if let toast = toasts.current {
                    ToastOverlay(toast: toast) { toasts.dismiss() }
                }
            }
            .animation(.easeInOut(duration: 0.2), value: toasts.current)
            .sheet(item: $sheet) { active in
                sheetContent(active)
            }
            .confirmationDialog(
                "Delete phase “\(pendingDeletePhase?.name ?? "")”?",
                isPresented: Binding(
                    get: { pendingDeletePhase != nil },
                    set: { if !$0 { pendingDeletePhase = nil } }
                ),
                titleVisibility: .visible
            ) {
                Button("Delete Phase", role: .destructive) {
                    if let phase = pendingDeletePhase {
                        pendingDeletePhase = nil
                        Task { toasts.report(await store.deletePhase(phase.id), success: "Phase deleted") }
                    }
                }
                Button("Cancel", role: .cancel) { pendingDeletePhase = nil }
            } message: {
                Text("All steps in this phase are removed. This cannot be undone.")
            }
            .task {
                await metadata.load(store: store)
            }
    }

    @ViewBuilder
    private var boardBody: some View {
        #if os(macOS)
        VSplitView {
            boardScroll
                .frame(minHeight: 240)
            if showLogConsole {
                LogConsole(activityStore: activityStore, toasts: toasts)
                    .frame(minHeight: 120, idealHeight: 200)
            }
        }
        #else
        boardScroll
        #endif
    }

    private var boardScroll: some View {
        ScrollView {
            if store.isDemoMode {
                DemoBanner()
            }

            if let cycle = store.cycle {
                BoardContent(
                    cycle: cycle,
                    store: store,
                    activityStore: activityStore,
                    metadata: metadata,
                    toasts: toasts,
                    onDeletePhase: { pendingDeletePhase = $0 }
                )
                .padding()
            } else if store.isConnected {
                VStack(spacing: 16) {
                    ProgressView()
                    Text("Loading cycle…")
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .padding(.top, 64)
            } else if let error = store.lastError {
                ContentUnavailableView {
                    Label("Connection Error", systemImage: "wifi.slash")
                } description: {
                    Text(error)
                } actions: {
                    Button("Retry") {
                        Task { await store.refresh() }
                    }
                    .buttonStyle(.borderedProminent)
                }
            } else {
                ContentUnavailableView("Not Connected", systemImage: "wifi.slash")
            }
        }
        .refreshable {
            await store.refresh()
        }
    }

    @ToolbarContentBuilder
    private var boardToolbar: some ToolbarContent {
        ToolbarItemGroup(placement: .primaryAction) {
            RunToolbarButtons(store: store, toasts: toasts)

            Menu {
                Button {
                    Task { toasts.report(await store.addPhase(name: "New Phase"), success: "Phase added") }
                } label: {
                    Label("Add Phase", systemImage: "plus.rectangle.on.rectangle")
                }
                .disabled(!store.canEdit)

                Divider()

                Button {
                    sheet = .assist
                } label: {
                    Label("Cycle Assistant…", systemImage: "wand.and.stars")
                }
                .disabled(!store.canEdit)

                Button {
                    sheet = .library
                } label: {
                    Label("Template Library…", systemImage: "square.grid.2x2")
                }
                .disabled(!store.canEdit)

                Divider()

                Button {
                    sheet = .importTemplate
                } label: {
                    Label("Import Template…", systemImage: "square.and.arrow.down")
                }
                .disabled(!store.canEdit)

                Button {
                    Task { await exportCycle() }
                } label: {
                    Label("Export Cycle…", systemImage: "square.and.arrow.up")
                }
                .disabled(store.cycle == nil || store.isDemoMode)

                #if os(macOS)
                Divider()

                Toggle("Show Log Console", isOn: $showLogConsole)
                #endif
            } label: {
                Label("Board Actions", systemImage: "ellipsis.circle")
            }
        }
    }

    @ViewBuilder
    private func sheetContent(_ active: BoardSheet) -> some View {
        switch active {
        case .assist:
            AssistView(store: store, toasts: toasts)
        case .library:
            LibraryView(store: store, toasts: toasts)
        case .importTemplate:
            ImportTemplateView(store: store, toasts: toasts)
        }
    }

    private func exportCycle() async {
        switch await store.exportCycle() {
        case .failure(let error):
            toasts.show(error.message, kind: .error)
        case .success(let export):
            #if os(macOS)
            let panel = NSSavePanel()
            panel.allowedContentTypes = [.json]
            let base = export.template.manifest.id.isEmpty ? "cycle" : export.template.manifest.id
            panel.nameFieldStringValue = "\(base).json"
            panel.canCreateDirectories = true
            guard panel.runModal() == .OK, let url = panel.url else { return }
            do {
                try export.json.write(to: url)
                toasts.show("Exported to \(url.lastPathComponent)", kind: .success)
            } catch {
                toasts.show("Write failed: \(error.localizedDescription)", kind: .error)
            }
            #else
            toasts.show("Export is available on macOS.", kind: .info)
            #endif
        }
    }
}

enum BoardSheet: String, Identifiable {
    case assist, library, importTemplate
    var id: String { rawValue }
}

// MARK: - Board Metadata (skills / categories / models / agents)

/// One-shot cache of the catalog endpoints the editor needs, so opening a step
/// drawer does not re-fetch four endpoints every time.
@Observable
@MainActor
final class BoardMetadata {
    var skills: [Skill] = []
    var categories: [String] = []
    var defaultCategory: String = ""
    var models: [DiscoveredModel] = []
    var agents: [Agent] = []
    var isLoaded = false

    /// Sensors have no discovery endpoint; this mirrors the web board's list.
    let sensors = ["git-dirty", "open-issues", "open-prs"]

    func load(store: BoardStore) async {
        guard !isLoaded, let api = store.client else { return }
        let loadedSkills = try? await api.skills()
        let loadedCategories = try? await api.categories()
        let loadedModels = try? await api.models()

        skills = loadedSkills ?? []
        if let loadedCategories {
            categories = loadedCategories.categories.keys.sorted()
            defaultCategory = loadedCategories.defaultCategory
        }
        if let loadedModels {
            models = loadedModels.discovered
            agents = loadedModels.agents
        }
        isLoaded = !skills.isEmpty || !categories.isEmpty
    }
}

// MARK: - Run Toolbar Buttons

private struct RunToolbarButtons: View {
    let store: BoardStore
    let toasts: ToastCenter
    @State private var doctorRunnable = true

    var body: some View {
        HStack(spacing: 8) {
            if store.isRunning || store.isPaused {
                Button {
                    Task { await pauseResume() }
                } label: {
                    Label(store.isPaused ? "Resume" : "Pause",
                          systemImage: store.isPaused ? "play.fill" : "pause.fill")
                }
                .buttonStyle(.bordered)
                .controlSize(.small)

                Button(role: .destructive) {
                    Task { toasts.report(await store.cancelRun(), success: "Run cancelled") }
                } label: {
                    Label("Cancel", systemImage: "xmark.circle.fill")
                }
                .buttonStyle(.bordered)
                .controlSize(.small)
            } else {
                Button {
                    Task { toasts.report(await store.runCycle(), success: "Cycle started") }
                } label: {
                    Label("Run Cycle", systemImage: "play.fill")
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.small)
                .disabled(!doctorRunnable)
                .help(doctorRunnable ? "Run the whole cycle" : "Runner unavailable — check the Doctor tab")
            }
        }
        .task {
            if let doc = await store.fetchDoctor() {
                doctorRunnable = doc.runnable
            }
        }
    }

    private func pauseResume() async {
        if store.isPaused {
            toasts.report(await store.resumeRun(), success: "Resumed")
        } else {
            toasts.report(await store.pauseRun(), success: "Paused")
        }
    }
}

// MARK: - Board Content

private struct BoardContent: View {
    let cycle: Cycle
    let store: BoardStore
    let activityStore: ActivityStore
    let metadata: BoardMetadata
    let toasts: ToastCenter
    let onDeletePhase: (Phase) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            if !cycle.description.isEmpty {
                Text(cycle.description)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }

            ForEach(cycle.phases) { phase in
                PhaseSection(
                    phase: phase,
                    store: store,
                    metadata: metadata,
                    toasts: toasts,
                    onDelete: { onDeletePhase(phase) }
                )
            }

            if cycle.phases.isEmpty {
                ContentUnavailableView {
                    Label("Empty Cycle", systemImage: "rectangle.dashed")
                } description: {
                    Text("Add a phase, import a template, or pick one from the library.")
                } actions: {
                    Button("Add Phase") {
                        Task { toasts.report(await store.addPhase(name: "New Phase"), success: "Phase added") }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(!store.canEdit)
                }
            }

            #if !os(macOS)
            if store.isRunning || store.isPaused {
                Text("Activity Log")
                    .font(.headline)
                ActivityLogView(activityStore: activityStore)
                    .frame(minHeight: 200)
                    .clipShape(RoundedRectangle(cornerRadius: 10))
                    .overlay(
                        RoundedRectangle(cornerRadius: 10)
                            .strokeBorder(Color.cardSeparator, lineWidth: 0.5)
                    )
            }
            #endif
        }
    }
}

// MARK: - Phase Section

private struct PhaseSection: View {
    let phase: Phase
    let store: BoardStore
    let metadata: BoardMetadata
    let toasts: ToastCenter
    let onDelete: () -> Void

    @State private var isRenaming = false
    @State private var draftName = ""

    private var completedCount: Int { phase.steps.filter(\.status.isTerminal).count }
    private var totalCount: Int { phase.steps.count }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            header

            LazyVGrid(columns: [GridItem(.adaptive(minimum: 300))], spacing: 8) {
                ForEach(Array(phase.steps.enumerated()), id: \.element.id) { index, step in
                    StepCard(
                        step: step,
                        phase: phase,
                        index: index,
                        store: store,
                        metadata: metadata,
                        toasts: toasts
                    )
                }
            }

            AppendDropZone(phase: phase, store: store, metadata: metadata, toasts: toasts)
        }
        .padding(.vertical, 4)
        .alert("Rename Phase", isPresented: $isRenaming) {
            TextField("Phase name", text: $draftName)
            Button("Cancel", role: .cancel) {}
            Button("Rename") {
                let name = draftName.trimmingCharacters(in: .whitespacesAndNewlines)
                guard !name.isEmpty else { return }
                Task { toasts.report(await store.renamePhase(phase.id, to: name), success: "Phase renamed") }
            }
        }
    }

    private var header: some View {
        HStack(spacing: 10) {
            Text(phase.name)
                .font(.headline)

            Text("\(completedCount)/\(totalCount)")
                .font(.caption)
                .foregroundStyle(.secondary)
                .monospacedDigit()

            ProgressView(value: Double(completedCount), total: Double(max(totalCount, 1)))
                .frame(width: 70)

            Spacer()

            Menu {
                Button {
                    Task { toasts.report(await addStep(), success: "Step added") }
                } label: {
                    Label("Add Step", systemImage: "plus")
                }

                Button {
                    draftName = phase.name
                    isRenaming = true
                } label: {
                    Label("Rename Phase…", systemImage: "pencil")
                }

                Divider()

                Button(role: .destructive) {
                    onDelete()
                } label: {
                    Label("Delete Phase…", systemImage: "trash")
                }
            } label: {
                Image(systemName: "ellipsis.circle")
            }
            .fixedSize()
            .disabled(!store.canEdit)
        }
    }

    private func addStep() async -> String? {
        await store.addStep(
            phaseID: phase.id,
            name: "New Step",
            skill: metadata.skills.first?.name ?? "",
            category: metadata.defaultCategory
        )
    }
}

/// Trailing drop target: dragging a step here appends it to the phase.
private struct AppendDropZone: View {
    let phase: Phase
    let store: BoardStore
    let metadata: BoardMetadata
    let toasts: ToastCenter

    @State private var isTargeted = false

    var body: some View {
        HStack {
            Button {
                Task {
                    toasts.report(
                        await store.addStep(
                            phaseID: phase.id,
                            name: "New Step",
                            skill: metadata.skills.first?.name ?? "",
                            category: metadata.defaultCategory
                        ),
                        success: "Step added"
                    )
                }
            } label: {
                Label("Add Step", systemImage: "plus")
                    .font(.caption)
            }
            .buttonStyle(.plain)
            .foregroundStyle(.secondary)
            .disabled(!store.canEdit)

            Spacer()
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .frame(maxWidth: .infinity)
        .background(
            RoundedRectangle(cornerRadius: 8)
                .fill(isTargeted ? Color.accentColor.opacity(0.12) : Color.clear)
        )
        .overlay(
            RoundedRectangle(cornerRadius: 8)
                .strokeBorder(
                    isTargeted ? Color.accentColor : Color.cardSeparator.opacity(0.5),
                    style: StrokeStyle(lineWidth: isTargeted ? 1.5 : 0.5, dash: [4, 3])
                )
        )
        .dropDestination(for: String.self) { items, _ in
            guard store.canEdit, let stepID = items.first else { return false }
            Task {
                toasts.report(
                    await store.moveStep(stepID, toPhaseID: phase.id, toIndex: phase.steps.count),
                    success: "Step moved"
                )
            }
            return true
        } isTargeted: { isTargeted = $0 }
    }
}

// MARK: - Step Card

private struct StepCard: View {
    let step: Step
    let phase: Phase
    let index: Int
    let store: BoardStore
    let metadata: BoardMetadata
    let toasts: ToastCenter

    @State private var showEditor = false
    @State private var confirmDelete = false
    @State private var doctorRunnable = true
    @State private var isTargeted = false

    private var isCurrentStep: Bool { store.runState?.currentStep == step.id }
    private var statusColor: Color { .statusColor(step.status) }

    var body: some View {
        HStack(spacing: 10) {
            Text(step.status.glyph)
                .font(.title3.monospaced())
                .foregroundStyle(statusColor)
                .frame(width: 24, alignment: .center)
                .help(step.status.rawValue)

            VStack(alignment: .leading, spacing: 3) {
                Text(step.name)
                    .font(.subheadline.weight(.medium))
                    .lineLimit(1)

                HStack(spacing: 6) {
                    if !step.category.isEmpty {
                        Text(step.category)
                            .font(.caption2.weight(.medium))
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(statusColor.opacity(0.12))
                            .foregroundStyle(statusColor)
                            .clipShape(Capsule())
                    }
                    if !step.skill.isEmpty {
                        Text("$\(step.skill)")
                            .font(.caption2.monospaced())
                            .foregroundStyle(.secondary)
                    }
                    badges
                    if isCurrentStep {
                        Text("ACTIVE")
                            .font(.caption2.weight(.bold))
                            .padding(.horizontal, 5)
                            .padding(.vertical, 2)
                            .background(.blue.opacity(0.15))
                            .foregroundStyle(.blue)
                            .clipShape(Capsule())
                    }
                }
            }

            Spacer(minLength: 0)

            HStack(spacing: 6) {
                if !step.enabled {
                    Image(systemName: "pause.circle")
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                        .help("Step is disabled")
                }

                Button {
                    Task { toasts.report(await store.runStep(step.id), success: "Running \(step.name)") }
                } label: {
                    Image(systemName: "play.circle.fill")
                        .font(.title3)
                        .foregroundStyle(.green)
                }
                .buttonStyle(.plain)
                .disabled(!doctorRunnable || store.isRunning)
                .help("Run only this step")

                Button {
                    showEditor = true
                } label: {
                    Image(systemName: "pencil.circle")
                        .font(.title3)
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
                .disabled(!store.canEdit)
                .help("Edit step")
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
        .background(isCurrentStep ? Color.blue.opacity(0.04) : Color.cardBackground)
        .clipShape(RoundedRectangle(cornerRadius: 10))
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .strokeBorder(borderColor, lineWidth: isTargeted || isCurrentStep ? 1.5 : 0.5)
        )
        .opacity(step.enabled ? 1.0 : 0.6)
        .contentShape(Rectangle())
        .draggable(step.id) {
            Text(step.name)
                .font(.caption)
                .padding(6)
                .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 6))
        }
        .dropDestination(for: String.self) { items, _ in
            guard store.canEdit, let draggedID = items.first, draggedID != step.id else { return false }
            Task {
                toasts.report(
                    await store.moveStep(draggedID, toPhaseID: phase.id, toIndex: index),
                    success: "Step moved"
                )
            }
            return true
        } isTargeted: { isTargeted = $0 }
        .contextMenu { contextMenu }
        .sheet(isPresented: $showEditor) {
            StepEditorView(step: step, store: store, metadata: metadata, toasts: toasts)
        }
        .confirmationDialog(
            "Delete step “\(step.name)”?",
            isPresented: $confirmDelete,
            titleVisibility: .visible
        ) {
            Button("Delete Step", role: .destructive) {
                Task { toasts.report(await store.deleteStep(step.id), success: "Step deleted") }
            }
            Button("Cancel", role: .cancel) {}
        }
        .task {
            if let doc = await store.fetchDoctor() {
                doctorRunnable = doc.runnable
            }
        }
    }

    @ViewBuilder
    private var badges: some View {
        if let override = step.modelOverride {
            Image(systemName: "cpu")
                .font(.caption2)
                .foregroundStyle(.secondary)
                .help("Model override: \(override.id)")
        }
        if step.autoSkip != nil {
            Image(systemName: "arrow.turn.down.right")
                .font(.caption2)
                .foregroundStyle(.secondary)
                .help("Auto-skip rule active")
        }
        if let backend = step.backendOverride, !backend.isEmpty {
            Image(systemName: "shippingbox")
                .font(.caption2)
                .foregroundStyle(.secondary)
                .help("Backend override: \(backend)")
        }
    }

    @ViewBuilder
    private var contextMenu: some View {
        Button {
            Task { toasts.report(await store.runStep(step.id), success: "Running \(step.name)") }
        } label: {
            Label("Run Step", systemImage: "play.fill")
        }
        .disabled(!doctorRunnable || store.isRunning)

        Button {
            showEditor = true
        } label: {
            Label("Edit…", systemImage: "pencil")
        }
        .disabled(!store.canEdit)

        Button {
            Task {
                var toggled = step
                toggled.enabled.toggle()
                toasts.report(
                    await store.saveStep(toggled),
                    success: toggled.enabled ? "Step enabled" : "Step disabled"
                )
            }
        } label: {
            Label(step.enabled ? "Disable" : "Enable", systemImage: step.enabled ? "pause" : "play")
        }
        .disabled(!store.canEdit)

        Button {
            Task { toasts.report(await store.duplicateStep(step.id), success: "Step duplicated") }
        } label: {
            Label("Duplicate", systemImage: "plus.square.on.square")
        }
        .disabled(!store.canEdit)

        Divider()

        Button(role: .destructive) {
            confirmDelete = true
        } label: {
            Label("Delete…", systemImage: "trash")
        }
        .disabled(!store.canEdit)
    }

    private var borderColor: Color {
        if isTargeted { return .accentColor }
        if isCurrentStep { return .blue.opacity(0.3) }
        return step.enabled ? .cardSeparator : .cardSeparator.opacity(0.4)
    }
}

// MARK: - Log Console (macOS)

#if os(macOS)
private struct LogConsole: View {
    let activityStore: ActivityStore
    let toasts: ToastCenter

    @State private var scrollLock = true

    var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 10) {
                Label("Activity Log", systemImage: "terminal")
                    .font(.caption.weight(.medium))

                if let stepID = activityStore.currentStepID {
                    Text(stepID)
                        .font(.caption2.monospaced())
                        .foregroundStyle(.secondary)
                }

                Spacer()

                Toggle(isOn: $scrollLock) {
                    Image(systemName: scrollLock ? "arrow.down.to.line" : "arrow.down.to.line.slash")
                }
                .toggleStyle(.button)
                .controlSize(.small)
                .help(scrollLock ? "Auto-scroll on" : "Auto-scroll off")

                Button {
                    let text = activityStore.lines.map(\.text).joined(separator: "\n")
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(text, forType: .string)
                    toasts.show("Log copied", kind: .success)
                } label: {
                    Image(systemName: "doc.on.doc")
                }
                .controlSize(.small)
                .disabled(activityStore.lines.isEmpty)
                .help("Copy log")

                Button {
                    activityStore.clear()
                } label: {
                    Image(systemName: "trash")
                }
                .controlSize(.small)
                .disabled(activityStore.lines.isEmpty)
                .help("Clear log")
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 6)
            .background(.bar)

            Divider()

            ActivityLogView(activityStore: activityStore, autoScroll: scrollLock)
        }
    }
}
#endif

// MARK: - Demo Banner

private struct DemoBanner: View {
    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: "info.circle.fill")
                .foregroundStyle(.orange)
            Text("Demo Mode — Sample data. No actions are executed and edits are disabled.")
                .font(.caption.weight(.medium))
            Spacer()
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(Color.orange.opacity(0.1))
        .clipShape(RoundedRectangle(cornerRadius: 8))
        .padding(.horizontal)
        .padding(.top, 8)
    }
}
