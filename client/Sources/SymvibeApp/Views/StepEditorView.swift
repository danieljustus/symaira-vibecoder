import SwiftUI
import SymvibeKit

struct StepEditorView: View {
    let step: Step
    let cycle: Cycle
    let store: BoardStore
    let onSave: (Cycle) -> Void

    @State private var editedSkill: String
    @State private var editedCategory: String
    @State private var editedEnabled: Bool
    @State private var editedModelOverrideID: String
    @State private var availableSkills: [Skill] = []
    @State private var availableCategories: [String] = []
    @State private var availableModels: [DiscoveredModel] = []
    @State private var isLoading = false
    @State private var errorMessage: String?
    @Environment(\.dismiss) private var dismiss

    init(step: Step, cycle: Cycle, store: BoardStore, onSave: @escaping (Cycle) -> Void) {
        self.step = step
        self.cycle = cycle
        self.store = store
        self.onSave = onSave
        _editedSkill = State(initialValue: step.skill)
        _editedCategory = State(initialValue: step.category)
        _editedEnabled = State(initialValue: step.enabled)
        _editedModelOverrideID = State(initialValue: step.modelOverride?.id ?? "")
    }

    var body: some View {
        NavigationStack {
            Form {
                Section("Basic") {
                    LabeledContent("Name", value: step.name)
                    LabeledContent("ID", value: step.id)

                    Toggle("Enabled", isOn: $editedEnabled)
                }

                Section("Skill") {
                    Picker("Skill", selection: $editedSkill) {
                        Text("(none)").tag("")
                        ForEach(availableSkills, id: \.name) { skill in
                            Text(skill.name).tag(skill.name)
                        }
                    }
                }

                Section("Category") {
                    Picker("Category", selection: $editedCategory) {
                        ForEach(availableCategories, id: \.self) { cat in
                            Text(cat).tag(cat)
                        }
                    }
                }

                Section("Model Override") {
                    Picker("Model", selection: $editedModelOverrideID) {
                        Text("(use category default)").tag("")
                        ForEach(availableModels, id: \.id) { model in
                            Text(model.name ?? model.id).tag(model.id)
                        }
                    }
                }
            }
            .navigationTitle("Edit Step")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(isLoading)
                }
            }
            .overlay {
                if isLoading {
                    ProgressView()
                }
            }
            .alert("Error", isPresented: .constant(errorMessage != nil)) {
                Button("OK") { errorMessage = nil }
            } message: {
                if let errorMessage {
                    Text(errorMessage)
                }
            }
            .task {
                await loadMetadata()
            }
        }
    }

    private func loadMetadata() async {
        guard let apiClient = store.client else { return }
        do {
            async let s = apiClient.skills()
            async let c = apiClient.categories()
            async let m = apiClient.models()
            availableSkills = try await s
            availableCategories = Array((try await c).categories.keys).sorted()
            availableModels = (try await m).discovered
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func save() {
        guard var updatedCycle = store.editableCycle() else { return }
        isLoading = true

        let modelOverride: StepModelOverride? = editedModelOverrideID.isEmpty
            ? nil
            : StepModelOverride(id: editedModelOverrideID, temperature: nil, variant: nil, fallbackModels: nil)

        for phaseIdx in updatedCycle.phases.indices {
            for stepIdx in updatedCycle.phases[phaseIdx].steps.indices {
                if updatedCycle.phases[phaseIdx].steps[stepIdx].id == step.id {
                    updatedCycle.phases[phaseIdx].steps[stepIdx].skill = editedSkill
                    updatedCycle.phases[phaseIdx].steps[stepIdx].category = editedCategory
                    updatedCycle.phases[phaseIdx].steps[stepIdx].enabled = editedEnabled
                    updatedCycle.phases[phaseIdx].steps[stepIdx].modelOverride = modelOverride
                }
            }
        }

        Task {
            let error = await store.saveCycle(updatedCycle)
            isLoading = false
            if let error {
                errorMessage = error
            } else {
                onSave(updatedCycle)
                dismiss()
            }
        }
    }
}
