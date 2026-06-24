import XCTest
@testable import SymvibeKit

@MainActor
final class BoardStoreTests: XCTestCase {

    private func makeStore() -> BoardStore {
        let suiteName = "test-board-store-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suiteName)!
        let connectionStore = ConnectionStore(defaults: defaults)
        return BoardStore(connectionStore: connectionStore)
    }

    private func sampleCycle() -> Cycle {
        Cycle(
            schemaVersion: 1,
            id: "test-cycle",
            name: "Test Cycle",
            description: "A test cycle",
            phases: [
                Phase(
                    id: "phase-1",
                    name: "Phase 1",
                    order: 1,
                    steps: [
                        Step(
                            id: "step-1",
                            name: "Step One",
                            order: 1,
                            skill: "test-skill",
                            category: "quick",
                            agent: nil,
                            promptSuffix: nil,
                            enabled: true,
                            modelOverride: nil,
                            autoSkip: nil,
                            dependsOn: nil,
                            parallelSafe: nil,
                            status: .pending
                        ),
                        Step(
                            id: "step-2",
                            name: "Step Two",
                            order: 2,
                            skill: "",
                            category: "deep",
                            agent: nil,
                            promptSuffix: nil,
                            enabled: true,
                            modelOverride: nil,
                            autoSkip: nil,
                            dependsOn: nil,
                            parallelSafe: nil,
                            status: .inProgress
                        ),
                    ]
                ),
            ]
        )
    }

    // MARK: - Initial State

    func testInitialState() {
        let store = makeStore()
        XCTAssertNil(store.cycle)
        XCTAssertNil(store.runState)
        XCTAssertFalse(store.isConnected)
        XCTAssertNil(store.lastError)
        XCTAssertNil(store.client)
    }

    // MARK: - Step Status Update

    func testUpdateStepStatus() async {
        let store = makeStore()
        store.cycle = sampleCycle()

        let event = Event(type: "step_status", stepID: "step-1", status: "done")
        await store.handleEvent(event)

        XCTAssertEqual(store.cycle?.phases[0].steps[0].status, .done)
        XCTAssertEqual(store.cycle?.phases[0].steps[1].status, .inProgress)
    }

    func testUpdateStepStatusUnknownStep() async {
        let store = makeStore()
        store.cycle = sampleCycle()

        let event = Event(type: "step_status", stepID: "nonexistent", status: "done")
        await store.handleEvent(event)

        XCTAssertEqual(store.cycle?.phases[0].steps[0].status, .pending)
    }

    func testUpdateStepStatusInvalidStatus() async {
        let store = makeStore()
        store.cycle = sampleCycle()

        let event = Event(type: "step_status", stepID: "step-1", status: "bogus")
        await store.handleEvent(event)

        XCTAssertEqual(store.cycle?.phases[0].steps[0].status, .pending)
    }

    // MARK: - Run State Update

    func testRunStateUpdate() async {
        let store = makeStore()

        let event = Event(type: "run_state", state: "running", runID: "run-1", stepID: "step-1")
        await store.handleEvent(event)

        XCTAssertEqual(store.runState?.state, "running")
        XCTAssertEqual(store.runState?.runID, "run-1")
        XCTAssertEqual(store.runState?.currentStep, "step-1")
    }

    func testRunStateUpdateIgnoresIrrelevantEvent() async {
        let store = makeStore()

        let event = Event(type: "other", state: "idle")
        await store.handleEvent(event)

        XCTAssertNil(store.runState)
    }

    // MARK: - Status Glyphs

    func testStatusGlyphs() {
        XCTAssertEqual(StepStatus.pending.glyph, "○")
        XCTAssertEqual(StepStatus.inProgress.glyph, "◐")
        XCTAssertEqual(StepStatus.done.glyph, "✓")
        XCTAssertEqual(StepStatus.skipped.glyph, "–")
        XCTAssertEqual(StepStatus.failed.glyph, "✕")
        XCTAssertEqual(StepStatus.blocked.glyph, "⦸")
        XCTAssertEqual(StepStatus.needsReview.glyph, "!")
    }

    // MARK: - Disconnect

    func testDisconnect() {
        let store = makeStore()
        store.cycle = sampleCycle()
        store.isConnected = true

        store.disconnect()

        XCTAssertNil(store.cycle)
        XCTAssertNil(store.runState)
        XCTAssertFalse(store.isConnected)
        XCTAssertNil(store.lastError)
        XCTAssertNil(store.client)
    }
}
