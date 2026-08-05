import Foundation
import Observation

@Observable
@MainActor
public final class BoardStore {
    public var cycle: Cycle?
    public var runState: RunState?
    public var isConnected = false
    public var isDemoMode = false
    public var lastError: String?

    private let connectionStore: ConnectionStore
    @ObservationIgnored private var apiClient: APIClient?
    @ObservationIgnored private var sseClient: SSEClient?
    @ObservationIgnored private var sseTask: Task<Void, Never>?
    @ObservationIgnored public var activityStore: ActivityStore?

    public var client: APIClient? { apiClient }

    public init(connectionStore: ConnectionStore) {
        self.connectionStore = connectionStore
    }

    // MARK: - Connection

    public func connect() async {
        guard let profile = connectionStore.activeProfile else {
            lastError = "No active connection"
            return
        }

        if profile.isDemo {
            await connectDemo()
            return
        }

        let token = try? connectionStore.deviceToken(for: profile.id)

        for host in profile.hostCandidates {
            guard let baseURL = profile.baseURL(for: host) else { continue }

            let api = APIClient(baseURL: baseURL, token: token)
            let sse = SSEClient(baseURL: baseURL, token: token)

            do {
                _ = try await api.version()
                self.apiClient = api
                self.sseClient = sse
                self.isConnected = true
                self.lastError = nil

                WidgetShared.writeConnection(baseURL: baseURL, token: token)

                await load()
                startSSE()
                return
            } catch {
                continue
            }
        }

        lastError = "Could not reach any host"
        isConnected = false
    }

    public func disconnect() {
        sseTask?.cancel()
        sseTask = nil
        apiClient = nil
        sseClient = nil
        isConnected = false
        isDemoMode = false
        cycle = nil
        runState = nil
        lastError = nil
        WidgetShared.clearAll()
        WidgetShared.clearConnection()
    }

    // MARK: - Demo Mode

    private func connectDemo() async {
        cycle = DemoData.sampleCycle
        runState = DemoData.sampleRunState
        isConnected = true
        isDemoMode = true
        lastError = nil
    }

    public func refresh() async {
        sseTask?.cancel()
        sseTask = nil
        isConnected = false
        await connect()
    }

    // MARK: - Data Loading

    public func load() async {
        guard let apiClient else {
            lastError = "Not connected"
            return
        }
        do {
            cycle = try await apiClient.cycle()
            lastError = nil
            syncWidgetData()
        } catch {
            lastError = error.localizedDescription
        }
    }

    // MARK: - SSE

    private func startSSE() {
        guard let sseClient else { return }
        sseTask?.cancel()
        sseTask = Task { [weak self] in
            guard let self else { return }
            var retryDelay: Duration = .seconds(1)
            while !Task.isCancelled {
                do {
                    // Backfill: after every (re)connect, fetch the server's
                    // bounded log buffer and merge events missed while
                    // disconnected. ActivityStore dedupes by ts, so re-merging
                    // on a retry is idempotent.
                    await self.replayLogs()
                    let stream = await sseClient.events(reconnect: false)
                    for try await event in stream {
                        if Task.isCancelled { break }
                        await self.handleEvent(event)
                    }
                    break
                } catch is CancellationError {
                    break
                } catch {
                    await MainActor.run {
                        self.lastError = "Reconnecting…"
                    }
                    try? await Task.sleep(for: retryDelay)
                    retryDelay = min(retryDelay * 2, .seconds(30))
                }
            }
            await MainActor.run {
                self.isConnected = false
                if self.lastError == "Reconnecting…" {
                    self.lastError = "Connection lost"
                }
            }
        }
    }

    // MARK: - Event Handling

    /// Fetches `GET /api/logs` and merges missed log/error events into the
    /// activity store. Failures are swallowed — the SSE reconnect loop retries
    /// the whole cycle, and the ts-based dedupe in ActivityStore makes
    /// re-merging idempotent.
    private func replayLogs() async {
        guard let sseClient, let activityStore else { return }
        do {
            let snapshot = try await sseClient.fetchLogs()
            activityStore.mergeReplay(snapshot.entries)
        } catch {
            // Not fatal: live SSE events still flow; the next reconnect
            // attempt will retry the backfill.
        }
    }

    func handleEvent(_ event: Event) async {
        switch event.type {
        case "board":
            await load()
            syncWidgetData()
        case "run_state":
            if let state = event.state {
                runState = RunState(
                    state: state,
                    runID: event.runID,
                    currentStep: event.stepID,
                    cycle: nil,
                    mode: nil
                )
                if state == "idle" {
                    activityStore?.clear()
                }
                syncWidgetData()
            }
        case "step_status":
            if let stepID = event.stepID,
               let statusStr = event.status,
               let newStatus = StepStatus(rawValue: statusStr) {
                updateStepStatus(stepID: stepID, status: newStatus)
                syncWidgetData()
            }
        case "log", "error":
            activityStore?.append(event: event)
        default:
            break
        }
    }

    func updateStepStatus(stepID: String, status: StepStatus) {
        guard var currentCycle = cycle else { return }
        for phaseIdx in currentCycle.phases.indices {
            for stepIdx in currentCycle.phases[phaseIdx].steps.indices {
                if currentCycle.phases[phaseIdx].steps[stepIdx].id == stepID {
                    currentCycle.phases[phaseIdx].steps[stepIdx].status = status
                    cycle = currentCycle
                    return
                }
            }
        }
    }

    private func syncWidgetData() {
        guard let cycle else {
            WidgetShared.clearAll()
            return
        }

        var activeStepName = "Idle"
        var activeStepID = ""
        var activeStatus = StepStatus.pending.rawValue
        var activePhase = ""

        for phase in cycle.phases {
            for step in phase.steps {
                if step.status == .inProgress {
                    activeStepName = step.name
                    activeStepID = step.id
                    activeStatus = step.status.rawValue
                    activePhase = phase.name
                    break
                }
            }
            if !activeStepID.isEmpty { break }
        }

        if activeStepID.isEmpty {
            for phase in cycle.phases {
                for step in phase.steps {
                    if step.status == .needsReview || step.status == .failed || step.status == .blocked {
                        activeStepName = step.name
                        activeStepID = step.id
                        activeStatus = step.status.rawValue
                        activePhase = phase.name
                        break
                    }
                }
                if !activeStepID.isEmpty { break }
            }
        }

        WidgetShared.writeCurrentStep(
            name: activeStepName,
            stepID: activeStepID,
            status: activeStatus,
            phaseName: activePhase,
            cycleName: cycle.name,
            runState: runState?.state ?? "idle"
        )
    }

    // MARK: - Run Control

    public var isRunning: Bool {
        runState?.state == "running"
    }

    public var isPaused: Bool {
        runState?.state == "paused"
    }

    /// Board edits are rejected by the server with 409 while a run is active,
    /// and demo mode has no server at all.
    public var canEdit: Bool {
        apiClient != nil && !isRunning && !isPaused
    }

    public func runCycle() async -> String? {
        guard let apiClient else { return "Not connected" }
        do {
            try await apiClient.runCycle()
            return nil
        } catch {
            return friendlyError(error)
        }
    }

    public func runStep(_ stepID: String) async -> String? {
        guard let apiClient else { return "Not connected" }
        do {
            try await apiClient.runStep(stepID)
            return nil
        } catch {
            return friendlyError(error)
        }
    }

    public func pauseRun() async -> String? {
        guard let apiClient else { return "Not connected" }
        do {
            try await apiClient.controlRun(action: "pause")
            return nil
        } catch {
            return friendlyError(error)
        }
    }

    public func resumeRun() async -> String? {
        guard let apiClient else { return "Not connected" }
        do {
            try await apiClient.controlRun(action: "resume")
            return nil
        } catch {
            return friendlyError(error)
        }
    }

    public func cancelRun() async -> String? {
        guard let apiClient else { return "Not connected" }
        do {
            try await apiClient.controlRun(action: "cancel")
            return nil
        } catch {
            return friendlyError(error)
        }
    }

    // MARK: - Step Editing

    /// Save the entire cycle (PUT /api/cycle). Returns nil on success or a user-facing error string.
    public func saveCycle(_ updatedCycle: Cycle) async -> String? {
        guard let apiClient else { return "Not connected" }
        do {
            cycle = try await apiClient.updateCycle(updatedCycle)
            return nil
        } catch {
            return friendlyError(error)
        }
    }

    /// Returns a mutable copy of the cycle for editing.
    public func editableCycle() -> Cycle? {
        cycle
    }

    /// Persists a single edited step without touching the rest of the board.
    public func saveStep(_ step: Step) async -> String? {
        guard var updated = cycle else { return "No cycle loaded" }
        updated.replace(step)
        return await saveCycle(updated)
    }

    // MARK: - Cycle Structure

    public func addStep(phaseID: String, name: String, skill: String, category: String) async -> String? {
        await mutate { api in
            let step = Step(id: "", name: name, order: 0, skill: skill, category: category)
            _ = try await api.addStep(phaseID: phaseID, step: step)
        }
    }

    public func deleteStep(_ stepID: String) async -> String? {
        await mutate { api in try await api.deleteStep(stepID) }
    }

    public func duplicateStep(_ stepID: String) async -> String? {
        await mutate { api in _ = try await api.duplicateStep(stepID) }
    }

    public func moveStep(_ stepID: String, toPhaseID: String, toIndex: Int) async -> String? {
        await mutate { api in try await api.moveStep(stepID, toPhaseID: toPhaseID, toIndex: toIndex) }
    }

    public func addPhase(name: String) async -> String? {
        await mutate { api in _ = try await api.addPhase(name: name) }
    }

    public func deletePhase(_ phaseID: String) async -> String? {
        await mutate { api in try await api.deletePhase(phaseID) }
    }

    /// Renames a phase. The server has no dedicated endpoint, so this goes
    /// through a full-cycle PUT.
    public func renamePhase(_ phaseID: String, to name: String) async -> String? {
        guard var updated = cycle else { return "No cycle loaded" }
        guard let idx = updated.phases.firstIndex(where: { $0.id == phaseID }) else { return "Phase not found" }
        updated.phases[idx].name = name
        return await saveCycle(updated)
    }

    // MARK: - Import / Export / Library / Assist

    public func exportCycle() async -> Result<(template: Template, json: Data), OperationError> {
        guard let apiClient else { return .failure(OperationError("Not connected")) }
        do {
            return .success(try await apiClient.exportCycle())
        } catch {
            return .failure(OperationError(friendlyError(error)))
        }
    }

    /// Imports a template, replacing the active cycle. Returns `.missing` when
    /// the server rejected it for unmet requirements so the caller can offer a
    /// category remap.
    public func importTemplate(_ template: Template, remap: [String: String] = [:]) async -> ImportOutcome {
        guard let apiClient else { return .failed("Not connected") }
        do {
            cycle = try await apiClient.importCycle(template: template, remap: remap)
            return .imported
        } catch SymvibeError.missingRequirements(let missing, let available) {
            return .missing(missing, available)
        } catch {
            return .failed(friendlyError(error))
        }
    }

    public enum ImportOutcome: Sendable {
        case imported
        case missing(MissingRequirements, Catalog)
        case failed(String)
    }

    public func libraryIndex() async -> Result<[LibraryEntry], OperationError> {
        guard let apiClient else { return .failure(OperationError("Not connected")) }
        do {
            return .success(try await apiClient.libraryIndex())
        } catch {
            return .failure(OperationError(friendlyError(error)))
        }
    }

    public func fetchLibraryTemplate(url: String) async -> Result<Template, OperationError> {
        guard let apiClient else { return .failure(OperationError("Not connected")) }
        do {
            return .success(try await apiClient.fetchLibraryTemplate(from: url))
        } catch {
            return .failure(OperationError(friendlyError(error)))
        }
    }

    /// Runs the agent-backed cycle assistant. Returns the proposed cycle — it is
    /// already persisted server-side, so the board is refreshed on success.
    public func assist(instruction: String) async -> Result<Cycle, OperationError> {
        guard let apiClient, let current = cycle else { return .failure(OperationError("Not connected")) }
        do {
            let updated = try await apiClient.assistCycle(cycle: current, instruction: instruction)
            cycle = updated
            syncWidgetData()
            return .success(updated)
        } catch {
            return .failure(OperationError(friendlyError(error)))
        }
    }

    /// Runs a structure-changing call and reloads the board from the server,
    /// which is the source of truth for ids and ordering.
    private func mutate(_ body: (APIClient) async throws -> Void) async -> String? {
        guard let apiClient else { return "Not connected" }
        do {
            try await body(apiClient)
            await load()
            return nil
        } catch {
            return friendlyError(error)
        }
    }

    // MARK: - Doctor

    public func fetchDoctor() async -> DoctorResponse? {
        guard let apiClient else { return nil }
        return try? await apiClient.doctor()
    }

    // MARK: - Helpers

    private func friendlyError(_ error: Error) -> String {
        if let symvibeError = error as? SymvibeError {
            switch symvibeError {
            case .http(let status, _):
                switch status {
                case 409:
                    return "Edits are locked while a run is active."
                case 503:
                    return "Run is not available (check doctor status)."
                default:
                    return symvibeError.errorDescription ?? "Unknown error"
                }
            default:
                return symvibeError.errorDescription ?? "Unknown error"
            }
        }
        return error.localizedDescription
    }
}
