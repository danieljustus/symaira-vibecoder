import XCTest
@testable import SymvibeKit

/// The board sends the whole cycle back on every edit (`PUT /api/cycle`), so
/// any field the client drops on decode is silently deleted server-side.
final class CycleRoundTripTests: XCTestCase {

    private func makeDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return decoder
    }

    private func makeEncoder() -> JSONEncoder {
        let encoder = JSONEncoder()
        encoder.keyEncodingStrategy = .convertToSnakeCase
        return encoder
    }

    private let stepJSON = """
    {
      "id": "1.1",
      "name": "Branch Cleanup",
      "order": 1,
      "skill": "gh-branch-clean",
      "category": "cleaning",
      "agent": "build",
      "prompt_suffix": "be careful",
      "enabled": true,
      "model_override": {
        "id": "anthropic/claude-opus",
        "temperature": 0.2,
        "variant": "thinking",
        "fallback_models": ["a", "b"]
      },
      "backend_override": "aider",
      "auto_skip": {"sensor": "git-dirty", "when": "==0"},
      "requires_review": {"when": "category == release"},
      "depends_on": ["1.0"],
      "parallel_safe": true,
      "status": "needs_review"
    }
    """

    func testStepDecodesEveryField() throws {
        let step = try makeDecoder().decode(Step.self, from: Data(stepJSON.utf8))

        XCTAssertEqual(step.id, "1.1")
        XCTAssertEqual(step.agent, "build")
        XCTAssertEqual(step.promptSuffix, "be careful")
        XCTAssertEqual(step.backendOverride, "aider")
        XCTAssertEqual(step.modelOverride?.temperature, 0.2)
        XCTAssertEqual(step.modelOverride?.variant, "thinking")
        XCTAssertEqual(step.modelOverride?.fallbackModels, ["a", "b"])
        XCTAssertEqual(step.autoSkip, AutoSkip(sensor: "git-dirty", when: "==0"))
        XCTAssertEqual(step.requiresReview, RequiresReview(when: "category == release"))
        XCTAssertEqual(step.dependsOn, ["1.0"])
        XCTAssertEqual(step.parallelSafe, true)
        XCTAssertEqual(step.status, .needsReview)
    }

    /// Regression guard: a decode → encode → decode cycle must not lose the
    /// fields the client never shows, above all `requires_review`.
    func testStepSurvivesEncodeDecodeCycle() throws {
        let original = try makeDecoder().decode(Step.self, from: Data(stepJSON.utf8))
        let reencoded = try makeEncoder().encode(original)
        let roundTripped = try makeDecoder().decode(Step.self, from: reencoded)

        XCTAssertEqual(original, roundTripped)
    }

    func testStepDecodesWithOnlyTemplateFields() throws {
        let json = """
        {"id": "2.1", "name": "Review", "skill": "review", "category": "code"}
        """
        let step = try makeDecoder().decode(Step.self, from: Data(json.utf8))

        XCTAssertEqual(step.status, .pending, "a template step has no runtime status")
        XCTAssertTrue(step.enabled, "steps default to enabled")
        XCTAssertNil(step.autoSkip)
    }
}

final class TemplateTests: XCTestCase {

    private func makeDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return decoder
    }

    func testDecodesExportedTemplate() throws {
        let json = """
        {
          "kind": "symvibe.template",
          "schema_version": 1,
          "manifest": {
            "id": "default",
            "name": "Default Cycle",
            "version": "1.0.0",
            "author": "symvibe",
            "tags": ["cycle"],
            "description": "the seed cycle"
          },
          "requires": {"skills": ["commit"], "categories": ["cleaning"]},
          "phases": [
            {"id": "p1", "name": "Cleaning", "order": 1, "steps": [
              {"id": "1.1", "name": "Branch", "skill": "gh-branch-clean", "category": "cleaning"}
            ]}
          ]
        }
        """
        let template = try makeDecoder().decode(Template.self, from: Data(json.utf8))

        XCTAssertTrue(template.isValidKind)
        XCTAssertEqual(template.manifest.tags, ["cycle"])
        XCTAssertEqual(template.requires.skills, ["commit"])
        XCTAssertEqual(template.phases.first?.steps.first?.skill, "gh-branch-clean")
    }

    func testDecodesMissingRequirementsRejection() throws {
        let json = """
        {
          "error": "missing requirements",
          "missing": {"categories": ["planning"]},
          "available": {
            "skills": ["commit"],
            "categories": ["cleaning", "coding"],
            "agents": ["build"],
            "sensors": ["git-dirty"]
          }
        }
        """
        let response = try makeDecoder().decode(MissingRequirementsResponse.self, from: Data(json.utf8))

        XCTAssertEqual(response.missing.categories, ["planning"])
        XCTAssertTrue(response.missing.skills.isEmpty)
        XCTAssertFalse(response.missing.isEmpty)
        XCTAssertTrue(response.missing.isRemappable, "only categories are missing, so a remap can fix it")
        XCTAssertEqual(response.available.categories, ["cleaning", "coding"])
    }

    func testMissingRequirementsWithSkillsIsNotRemappable() {
        let missing = MissingRequirements(skills: ["nope"], categories: ["planning"])

        XCTAssertFalse(missing.isRemappable)
        XCTAssertFalse(missing.isEmpty)
    }

    func testLibraryEntryToleratesSparseIndex() throws {
        let json = """
        [{"id": "a", "name": "A", "url": "https://example.com/a.json"}]
        """
        let entries = try makeDecoder().decode([LibraryEntry].self, from: Data(json.utf8))

        XCTAssertEqual(entries.count, 1)
        XCTAssertTrue(entries[0].tags.isEmpty)
        XCTAssertTrue(entries[0].author.isEmpty)
    }
}

final class CycleLookupTests: XCTestCase {

    private func cycle() -> Cycle {
        Cycle(
            schemaVersion: 1,
            id: "c",
            name: "C",
            description: "",
            phases: [
                Phase(id: "p1", name: "One", order: 1, steps: [
                    Step(id: "1.1", name: "A", order: 1, skill: "s", category: "cat"),
                ]),
                Phase(id: "p2", name: "Two", order: 2, steps: [
                    Step(id: "2.1", name: "B", order: 1, skill: "s", category: "cat"),
                ]),
            ]
        )
    }

    func testFindsStepAcrossPhases() {
        XCTAssertEqual(cycle().step(id: "2.1")?.name, "B")
        XCTAssertNil(cycle().step(id: "nope"))
    }

    func testFindsOwningPhase() {
        XCTAssertEqual(cycle().phase(containing: "2.1")?.id, "p2")
    }

    func testReplaceUpdatesInPlace() {
        var subject = cycle()
        var step = subject.step(id: "1.1")!
        step.name = "Renamed"
        subject.replace(step)

        XCTAssertEqual(subject.step(id: "1.1")?.name, "Renamed")
        XCTAssertEqual(subject.phases[0].steps.count, 1)
    }

    func testReplaceIgnoresUnknownStep() {
        var subject = cycle()
        let before = subject
        subject.replace(Step(id: "ghost", name: "X", order: 1, skill: "", category: ""))

        XCTAssertEqual(subject, before)
    }
}
