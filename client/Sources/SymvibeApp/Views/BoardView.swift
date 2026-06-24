import SwiftUI
import SymvibeKit

struct BoardView: View {
    let store: BoardStore
    let activityStore: ActivityStore

    var body: some View {
        ScrollView {
            if store.isDemoMode {
                DemoBanner()
            }

            if let cycle = store.cycle {
                BoardContent(cycle: cycle, store: store, activityStore: activityStore)
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
        .navigationTitle(store.cycle?.name ?? "Board")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                RunToolbarButtons(store: store)
            }
        }
        .refreshable {
            await store.refresh()
        }
    }
}

// MARK: - Run Toolbar Buttons

private struct RunToolbarButtons: View {
    let store: BoardStore
    @State private var doctorRunnable = true
    @State private var alertMessage: String?
    @State private var showRunAlert = false

    var body: some View {
        HStack(spacing: 8) {
            if store.isRunning {
                Button {
                    Task { await pauseResume() }
                } label: {
                    Label(store.isPaused ? "Resume" : "Pause",
                          systemImage: store.isPaused ? "play.fill" : "pause.fill")
                }
                .buttonStyle(.bordered)
                .controlSize(.small)

                Button(role: .destructive) {
                    Task { await cancelRun() }
                } label: {
                    Label("Cancel", systemImage: "xmark.circle.fill")
                }
                .buttonStyle(.bordered)
                .controlSize(.small)
            } else {
                Button {
                    Task { await runCycle() }
                } label: {
                    Label("Run Cycle", systemImage: "play.fill")
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.small)
                .disabled(!doctorRunnable)
            }
        }
        .task {
            if let doc = await store.fetchDoctor() {
                doctorRunnable = doc.runnable
            }
        }
        .alert("Run Error", isPresented: $showRunAlert) {
            Button("OK") {}
        } message: {
            Text(alertMessage ?? "Unknown error")
        }
    }

    private func runCycle() async {
        if let err = await store.runCycle() {
            alertMessage = err
            showRunAlert = true
        }
    }

    private func pauseResume() async {
        let err: String?
        if store.isPaused {
            err = await store.resumeRun()
        } else {
            err = await store.pauseRun()
        }
        if let err {
            alertMessage = err
            showRunAlert = true
        }
    }

    private func cancelRun() async {
        if let err = await store.cancelRun() {
            alertMessage = err
            showRunAlert = true
        }
    }
}

// MARK: - Board Content

private struct BoardContent: View {
    let cycle: Cycle
    let store: BoardStore
    let activityStore: ActivityStore

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            if !cycle.description.isEmpty {
                Text(cycle.description)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }

            ForEach(cycle.phases) { phase in
                PhaseSection(phase: phase, store: store, activityStore: activityStore)
            }

            if store.isRunning || store.isPaused {
                SectionHeader(title: "Activity Log")
                ActivityLogView(activityStore: activityStore)
                    .frame(minHeight: 200)
                    .clipShape(RoundedRectangle(cornerRadius: 10))
                    .overlay(
                        RoundedRectangle(cornerRadius: 10)
                            .strokeBorder(Color(.separator), lineWidth: 0.5)
                    )
            }
        }
    }
}

private struct SectionHeader: View {
    let title: String
    var body: some View {
        Text(title)
            .font(.headline)
    }
}

// MARK: - Phase Section

private struct PhaseSection: View {
    let phase: Phase
    let store: BoardStore
    let activityStore: ActivityStore

    private var completedCount: Int {
        phase.steps.filter(\.status.isTerminal).count
    }

    private var totalCount: Int {
        phase.steps.count
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text(phase.name)
                    .font(.headline)
                Spacer()
                Text("\(completedCount)/\(totalCount)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                ProgressView(value: Double(completedCount), total: Double(totalCount))
                    .frame(width: 60)
            }

            LazyVGrid(
                columns: [GridItem(.adaptive(minimum: 260))],
                spacing: 8
            ) {
                ForEach(phase.steps) { step in
                    StepCard(step: step, store: store, activityStore: activityStore)
                }
            }
        }
    }
}

// MARK: - Step Card

private struct StepCard: View {
    let step: Step
    let store: BoardStore
    let activityStore: ActivityStore

    @State private var showEditor = false
    @State private var showRunAlert = false
    @State private var runErrorMessage: String?
    @State private var doctorRunnable = true

    private var isCurrentStep: Bool {
        store.runState?.currentStep == step.id
    }

    var body: some View {
        HStack(spacing: 10) {
            Text(step.status.glyph)
                .font(.title3.monospaced())
                .foregroundStyle(statusColor)
                .frame(width: 24, alignment: .center)

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
                }

                Button {
                    Task { await runSingleStep() }
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
                .disabled(store.isRunning)
                .help("Edit step")
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
        .background(isCurrentStep ? Color.blue.opacity(0.04) : Color(.background))
        .clipShape(RoundedRectangle(cornerRadius: 10))
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .strokeBorder(
                    isCurrentStep
                        ? Color.blue.opacity(0.3)
                        : step.enabled
                            ? Color(.separator)
                            : Color(.separator).opacity(0.4),
                    lineWidth: isCurrentStep ? 1.5 : 0.5
                )
        )
        .opacity(step.enabled ? 1.0 : 0.6)
        .sheet(isPresented: $showEditor) {
            StepEditorView(
                step: step,
                cycle: store.cycle ?? emptyCycle(),
                store: store,
                onSave: { _ in }
            )
        }
        .alert("Run Error", isPresented: $showRunAlert) {
            Button("OK") {}
        } message: {
            Text(runErrorMessage ?? "Unknown error")
        }
        .task {
            if let doc = await store.fetchDoctor() {
                doctorRunnable = doc.runnable
            }
        }
    }

    private func runSingleStep() async {
        if let err = await store.runStep(step.id) {
            runErrorMessage = err
            showRunAlert = true
        }
    }

    private func emptyCycle() -> Cycle {
        Cycle(
            schemaVersion: 1,
            id: "",
            name: "",
            description: "",
            phases: []
        )
    }

    private var statusColor: Color {
        switch step.status {
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

// MARK: - Demo Banner

private struct DemoBanner: View {
    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: "info.circle.fill")
                .foregroundStyle(.orange)
            Text("Demo Mode — Sample data. No actions are executed.")
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
