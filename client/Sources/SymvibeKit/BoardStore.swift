import Foundation
import Observation

@Observable
@MainActor
public final class BoardStore {
    public var cycle: Cycle?
    public var runState: RunState?
    public var isConnected = false
    public var lastError: String?

    private let connectionStore: ConnectionStore
    @ObservationIgnored private var apiClient: APIClient?
    @ObservationIgnored private var sseClient: SSEClient?
    @ObservationIgnored private var sseTask: Task<Void, Never>?

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
        cycle = nil
        runState = nil
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

    func handleEvent(_ event: Event) async {
        switch event.type {
        case "board":
            await load()
        case "run_state":
            if let state = event.state {
                runState = RunState(
                    state: state,
                    runID: event.runID,
                    currentStep: event.stepID,
                    cycle: nil,
                    mode: nil
                )
            }
        case "step_status":
            if let stepID = event.stepID,
               let statusStr = event.status,
               let newStatus = StepStatus(rawValue: statusStr) {
                updateStepStatus(stepID: stepID, status: newStatus)
            }
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
}
