import Foundation

/// Static sample data for the app's demo mode.
///
/// Demo mode allows App Store reviewers and prospective users to evaluate
/// the app's UI and flow without pairing with a real symvibe server.
/// All data is purely illustrative — no network calls are made.
public enum DemoData {
    // MARK: - Cycle

    public static let sampleCycle = Cycle(
        schemaVersion: 1,
        id: "demo-cycle",
        name: "Demo Cycle",
        description: "A sample cycle showcasing the symvibe workflow. No actions are executed in demo mode.",
        phases: [
            Phase(
                id: "phase-1",
                name: "Cleaning",
                order: 1,
                steps: [
                    Step(
                        id: "step-1.1",
                        name: "Branch Cleanup",
                        order: 1,
                        skill: "00-sync",
                        category: "quick",
                        agent: nil,
                        promptSuffix: nil,
                        enabled: true,
                        modelOverride: nil,
                        autoSkip: nil,
                        dependsOn: nil,
                        parallelSafe: nil,
                        status: .done
                    ),
                    Step(
                        id: "step-1.2",
                        name: "Commit Hygiene",
                        order: 2,
                        skill: "git-master",
                        category: "git",
                        agent: nil,
                        promptSuffix: nil,
                        enabled: true,
                        modelOverride: nil,
                        autoSkip: AutoSkip(sensor: "git-dirty", when: "== 0"),
                        dependsOn: nil,
                        parallelSafe: nil,
                        status: .done
                    ),
                ]
            ),
            Phase(
                id: "phase-2",
                name: "Code Review",
                order: 2,
                steps: [
                    Step(
                        id: "step-2.1",
                        name: "Quality Audit",
                        order: 1,
                        skill: "01-code-review",
                        category: "deep",
                        agent: nil,
                        promptSuffix: nil,
                        enabled: true,
                        modelOverride: nil,
                        autoSkip: nil,
                        dependsOn: ["step-1.1"],
                        parallelSafe: nil,
                        status: .inProgress
                    ),
                    Step(
                        id: "step-2.2",
                        name: "Simplification",
                        order: 2,
                        skill: "simplify",
                        category: "deep",
                        agent: nil,
                        promptSuffix: nil,
                        enabled: true,
                        modelOverride: nil,
                        autoSkip: nil,
                        dependsOn: ["step-2.1"],
                        parallelSafe: nil,
                        status: .pending
                    ),
                ]
            ),
            Phase(
                id: "phase-3",
                name: "Development",
                order: 3,
                steps: [
                    Step(
                        id: "step-3.1",
                        name: "Implement Issues",
                        order: 1,
                        skill: "03-gh-go",
                        category: "deep",
                        agent: nil,
                        promptSuffix: nil,
                        enabled: true,
                        modelOverride: nil,
                        autoSkip: AutoSkip(sensor: "open-issues", when: "== 0"),
                        dependsOn: ["step-2.1"],
                        parallelSafe: nil,
                        status: .pending
                    ),
                    Step(
                        id: "step-3.2",
                        name: "PR Review",
                        order: 2,
                        skill: "04-gh-pr-go",
                        category: "deep",
                        agent: nil,
                        promptSuffix: nil,
                        enabled: true,
                        modelOverride: nil,
                        autoSkip: nil,
                        dependsOn: ["step-3.1"],
                        parallelSafe: nil,
                        status: .pending
                    ),
                ]
            ),
            Phase(
                id: "phase-4",
                name: "Pre-Release",
                order: 4,
                steps: [
                    Step(
                        id: "step-4.1",
                        name: "Security Scan",
                        order: 1,
                        skill: "05-gh-security-fix",
                        category: "deep",
                        agent: nil,
                        promptSuffix: nil,
                        enabled: true,
                        modelOverride: nil,
                        autoSkip: nil,
                        dependsOn: ["step-3.2"],
                        parallelSafe: nil,
                        status: .pending
                    ),
                    Step(
                        id: "step-4.2",
                        name: "Prerelease Gate",
                        order: 2,
                        skill: "06-gh-prerelease",
                        category: "quick",
                        agent: nil,
                        promptSuffix: nil,
                        enabled: true,
                        modelOverride: nil,
                        autoSkip: nil,
                        dependsOn: ["step-4.1"],
                        parallelSafe: nil,
                        status: .pending
                    ),
                ]
            ),
        ]
    )

    // MARK: - Run State

    public static let sampleRunState = RunState(
        state: "idle",
        runID: nil,
        currentStep: "step-2.1",
        cycle: "demo-cycle",
        mode: nil
    )

    // MARK: - Doctor

    public static let sampleDoctor = DoctorResponse(
        opencode: RunnerInfo(
            name: "opencode",
            version: "1.2.0",
            path: "/usr/local/bin/opencode",
            detail: "Demo mode — no real server"
        ),
        opencodeOk: true,
        git: true,
        gh: true,
        runnable: false,
        hints: [
            "demo": "Running in demo mode. Pair with a real symvibe server to execute cycles."
        ]
    )

    // MARK: - Version

    public static let sampleVersion = VersionResponse(
        apiVersion: "1.0",
        serverVersion: "0.1.0-demo",
        capabilities: ["demo", "board", "sse"],
        goVersion: "go1.26",
        platform: "demo"
    )

    // MARK: - SSE Events (for simulated activity)

    public static let sampleEvents: [Event] = [
        Event(type: "log", runID: "demo-run", stepID: "step-2.1", status: nil, kind: "log",
              line: "[deep] Starting quality audit…", state: nil, ts: demoTS(0)),
        Event(type: "log", runID: "demo-run", stepID: "step-2.1", status: nil, kind: "log",
              line: "[deep] Scanning 14 Swift files for issues", state: nil, ts: demoTS(2)),
        Event(type: "log", runID: "demo-run", stepID: "step-2.1", status: nil, kind: "log",
              line: "[deep] Found 2 suggestions in BoardStore.swift", state: nil, ts: demoTS(4)),
        Event(type: "step_status", runID: "demo-run", stepID: "step-2.1", status: "done",
              kind: nil, line: nil, state: nil, ts: demoTS(6)),
        Event(type: "run_state", runID: "demo-run", stepID: nil, status: nil,
              kind: nil, line: nil, state: "idle", ts: demoTS(6)),
    ]

    // MARK: - Helpers

    private static func demoTS(_ offset: Int64) -> Int64 {
        Int64(Date().timeIntervalSince1970) + offset
    }
}
