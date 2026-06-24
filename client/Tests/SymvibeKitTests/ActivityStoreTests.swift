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
}
