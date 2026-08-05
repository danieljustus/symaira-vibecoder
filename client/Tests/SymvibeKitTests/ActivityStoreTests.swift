import XCTest
@testable import SymvibeKit

@MainActor
final class ActivityStoreTests: XCTestCase {

    func testInitialState() {
        let store = ActivityStore()
        XCTAssertTrue(store.lines.isEmpty)
        XCTAssertNil(store.currentStepID)
    }

    func testAppendLogEvent() {
        let store = ActivityStore()
        let event = Event(type: "log", stepID: "step-1", line: "Building project…")
        store.append(event: event)

        XCTAssertEqual(store.lines.count, 1)
        XCTAssertEqual(store.lines.first?.text, "Building project…")
        XCTAssertEqual(store.lines.first?.kind, .log)
        XCTAssertEqual(store.currentStepID, "step-1")
    }

    func testAppendErrorEvent() {
        let store = ActivityStore()
        let event = Event(type: "error", stepID: "step-1", line: "Compilation failed")
        store.append(event: event)

        XCTAssertEqual(store.lines.count, 1)
        XCTAssertEqual(store.lines.first?.text, "Compilation failed")
        XCTAssertEqual(store.lines.first?.kind, .error)
    }

    func testAppendIgnoresEmptyLine() {
        let store = ActivityStore()
        let event = Event(type: "log", stepID: "step-1", line: "")
        store.append(event: event)

        XCTAssertTrue(store.lines.isEmpty)
    }

    func testAppendIgnoresMissingLine() {
        let store = ActivityStore()
        let event = Event(type: "log", stepID: "step-1")
        store.append(event: event)

        XCTAssertTrue(store.lines.isEmpty)
    }

    func testAppendIgnoresMissingStepID() {
        let store = ActivityStore()
        let event = Event(type: "log", line: "some output")
        store.append(event: event)

        XCTAssertTrue(store.lines.isEmpty)
    }

    func testStepChangeClearsLines() {
        let store = ActivityStore()

        let e1 = Event(type: "log", stepID: "step-1", line: "line 1")
        store.append(event: e1)
        XCTAssertEqual(store.lines.count, 1)

        let e2 = Event(type: "log", stepID: "step-2", line: "line 2")
        store.append(event: e2)

        XCTAssertEqual(store.lines.count, 1)
        XCTAssertEqual(store.lines.first?.text, "line 2")
        XCTAssertEqual(store.currentStepID, "step-2")
    }

    func testSameStepAccumulatesLines() {
        let store = ActivityStore()

        for i in 1...5 {
            let event = Event(type: "log", stepID: "step-1", line: "line \(i)")
            store.append(event: event)
        }

        XCTAssertEqual(store.lines.count, 5)
        XCTAssertEqual(store.currentStepID, "step-1")
    }

    func testClearResetsEverything() {
        let store = ActivityStore()
        store.append(event: Event(type: "log", stepID: "step-1", line: "output"))
        store.clear()

        XCTAssertTrue(store.lines.isEmpty)
        XCTAssertNil(store.currentStepID)
    }

    func testClearIfCurrentMatches() {
        let store = ActivityStore()
        store.append(event: Event(type: "log", stepID: "step-1", line: "output"))
        store.clearIfCurrent(stepID: "step-1")

        XCTAssertTrue(store.lines.isEmpty)
        XCTAssertNil(store.currentStepID)
    }

    func testClearIfCurrentDoesNotMatch() {
        let store = ActivityStore()
        store.append(event: Event(type: "log", stepID: "step-1", line: "output"))
        store.clearIfCurrent(stepID: "step-2")

        XCTAssertEqual(store.lines.count, 1)
        XCTAssertEqual(store.currentStepID, "step-1")
    }

    func testMaxLinesTrims() {
        let store = ActivityStore()

        for i in 1...600 {
            let event = Event(type: "log", stepID: "step-1", line: "line \(i)")
            store.append(event: event)
        }

        XCTAssertLessThanOrEqual(store.lines.count, 500)
        XCTAssertEqual(store.lines.first?.text, "line 101")
    }

    func testLogLineHasTimestamp() {
        let store = ActivityStore()
        let before = Date()
        store.append(event: Event(type: "log", stepID: "step-1", line: "output"))
        let after = Date()

        XCTAssertNotNil(store.lines.first?.timestamp)
        XCTAssertGreaterThanOrEqual(store.lines.first!.timestamp, before)
        XCTAssertLessThanOrEqual(store.lines.first!.timestamp, after)
    }

    func testLogLineHasUniqueID() {
        let store = ActivityStore()
        store.append(event: Event(type: "log", stepID: "step-1", line: "a"))
        store.append(event: Event(type: "log", stepID: "step-1", line: "b"))

        XCTAssertNotEqual(store.lines[0].id, store.lines[1].id)
    }

    // MARK: - Replay merge

    func testMergeReplayBackfillsUnseenEvents() {
        let store = ActivityStore()
        let seen = Event(type: "log", stepID: "step-1", line: "seen", ts: 100)
        let missed = Event(type: "log", stepID: "step-1", line: "missed", ts: 200)
        store.append(event: seen)

        store.mergeReplay([seen, missed])

        // The already-seen event must not be re-appended.
        XCTAssertEqual(store.lines.map(\.text), ["seen", "missed"])
        XCTAssertEqual(store.currentStepID, "step-1")
    }

    func testMergeReplaySkipsEverythingSeen() {
        let store = ActivityStore()
        let events = (1...3).map { Event(type: "log", stepID: "step-1", line: "line \($0)", ts: Int64($0 * 100)) }
        store.mergeReplay(events)

        store.mergeReplay(events)

        XCTAssertEqual(store.lines.count, 3)
        XCTAssertEqual(store.lastSeenTS, 300)
    }

    func testMergeReplayDoesNotResurrectClearedStep() {
        let store = ActivityStore()
        store.append(event: Event(type: "log", stepID: "step-1", line: "old", ts: 100))
        store.clear()

        // Replay must not re-add the cleared step-1 line, but must merge the
        // newer step-2 line (the store only ever shows the current step).
        store.mergeReplay([
            Event(type: "log", stepID: "step-1", line: "old", ts: 100),
            Event(type: "log", stepID: "step-2", line: "new", ts: 200),
        ])

        XCTAssertEqual(store.lines.map(\.text), ["new"])
        XCTAssertEqual(store.currentStepID, "step-2")
    }

    func testMergeReplayOrdersByServerSequence() {
        let store = ActivityStore()
        // Ring order (oldest first) must be preserved through the merge.
        store.mergeReplay([
            Event(type: "log", stepID: "step-1", line: "first", ts: 100),
            Event(type: "log", stepID: "step-1", line: "second", ts: 200),
            Event(type: "error", stepID: "step-1", line: "boom", ts: 300),
        ])

        XCTAssertEqual(store.lines.map(\.text), ["first", "second", "boom"])
        XCTAssertEqual(store.lines.map(\.kind), [.log, .log, .error])
        XCTAssertEqual(store.lastSeenTS, 300)
    }

    func testMergeReplayWithoutTSAlwaysAppends() {
        let store = ActivityStore()
        // Events without ts (legacy/edge case) are not deduped — matching the
        // live path where they were always appended.
        let e = Event(type: "log", stepID: "step-1", line: "output")
        store.mergeReplay([e])
        store.mergeReplay([e])

        XCTAssertEqual(store.lines.count, 2)
    }
}
