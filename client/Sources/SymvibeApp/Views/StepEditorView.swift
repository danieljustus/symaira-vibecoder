import SwiftUI
import SymvibeKit

/// Full step drawer — the native equivalent of the web board's edit drawer,
/// covering name, skill, category, agent, prompt suffix, model override,
/// backend override and the auto-skip rule.
struct StepEditorView: View {
    let step: Step
    let store: BoardStore
    let metadata: BoardMetadata
    let toasts: ToastCenter

    @State private var draft: Step
    @State private var useModelOverride: Bool
    @State private var temperatureText: String
    @State private var fallbackText: String
    @State private var useBackendOverride: Bool
    @State private var useAutoSkip: Bool
    @State private var isSaving = false
    @State private var errorMessage: String?

    @Environment(\.dismiss) private var dismiss

    /// Backends the server can dispatch a step to (`internal/runner`).
    private let backends = ["opencode", "api", "aider", "claudecode", "cline", "local_api"]

    init(step: Step, store: BoardStore, metadata: BoardMetadata, toasts: ToastCenter) {
        self.step = step
        self.store = store
        self.metadata = metadata
        self.toasts = toasts
        _draft = State(initialValue: step)
        _useModelOverride = State(initialValue: step.modelOverride != nil)
        _temperatureText = State(initialValue: step.modelOverride?.temperature.map { "\($0)" } ?? "")
        _fallbackText = State(initialValue: (step.modelOverride?.fallbackModels ?? []).joined(separator: ", "))
        _useBackendOverride = State(initialValue: !(step.backendOverride ?? "").isEmpty)
        _useAutoSkip = State(initialValue: step.autoSkip != nil)
    }

    var body: some View {
        NavigationStack {
            Form {
                basicSection
                skillSection
                modelOverrideSection
                backendOverrideSection
                autoSkipSection
                if step.requiresReview != nil {
                    Section("Review Gate") {
                        LabeledContent("Requires review when", value: step.requiresReview?.when ?? "")
                        Text("Configured server-side in the cycle definition; not editable here.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            .formStyle(.grouped)
            .navigationTitle("Edit Step")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(isSaving || trimmedName.isEmpty)
                }
            }
            .overlay {
                if isSaving {
                    ProgressView()
                        .padding()
                        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 8))
                }
            }
            .alert("Error", isPresented: .constant(errorMessage != nil)) {
                Button("OK") { errorMessage = nil }
            } message: {
                Text(errorMessage ?? "")
            }
            .task {
                await metadata.load(store: store)
            }
        }
        #if os(macOS)
        .frame(minWidth: 520, minHeight: 560)
        #endif
    }

    private var trimmedName: String {
        draft.name.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    // MARK: - Sections

    private var basicSection: some View {
        Section("Basic") {
            TextField("Name", text: $draft.name)
            LabeledContent("ID") {
                Text(step.id)
                    .font(.caption.monospaced())
                    .foregroundStyle(.secondary)
                    .textSelection(.enabled)
            }
            LabeledContent("Status", value: "\(step.status.glyph)  \(step.status.rawValue)")
            Toggle("Enabled", isOn: $draft.enabled)
        }
    }

    private var skillSection: some View {
        Section("Execution") {
            Picker("Skill", selection: $draft.skill) {
                Text("(none)").tag("")
                ForEach(skillOptions, id: \.self) { name in
                    Text(name).tag(name)
                }
            }

            Picker("Category", selection: $draft.category) {
                Text("(default)").tag("")
                ForEach(categoryOptions, id: \.self) { name in
                    Text(name).tag(name)
                }
            }

            Picker("Agent", selection: agentBinding) {
                Text("(default)").tag("")
                ForEach(agentOptions, id: \.self) { name in
                    Text(name).tag(name)
                }
            }

            VStack(alignment: .leading, spacing: 4) {
                Text("Prompt Suffix")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                TextEditor(text: promptSuffixBinding)
                    .font(.body.monospaced())
                    .frame(minHeight: 70)
                    .overlay(
                        RoundedRectangle(cornerRadius: 6)
                            .strokeBorder(Color.cardSeparator, lineWidth: 0.5)
                    )
                Text("Appended to the generated prompt for this step.")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private var modelOverrideSection: some View {
        Section("Model Override") {
            Toggle("Override the category model", isOn: $useModelOverride)

            if useModelOverride {
                Picker("Model", selection: modelIDBinding) {
                    Text("(category default)").tag("")
                    ForEach(modelOptions, id: \.self) { id in
                        Text(id).tag(id)
                    }
                }

                TextField("Temperature (e.g. 0.2)", text: $temperatureText)
                    #if os(iOS)
                    .keyboardType(.decimalPad)
                    #endif

                TextField("Variant", text: variantBinding)

                VStack(alignment: .leading, spacing: 4) {
                    TextField("Fallback models (comma-separated)", text: $fallbackText)
                    Text("Tried in order if the primary model fails.")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
            }
        }
    }

    private var backendOverrideSection: some View {
        Section("Backend Override") {
            Toggle("Run this step on a specific backend", isOn: $useBackendOverride)

            if useBackendOverride {
                Picker("Backend", selection: backendBinding) {
                    Text("(default)").tag("")
                    ForEach(backendOptions, id: \.self) { name in
                        Text(name).tag(name)
                    }
                }
            }
        }
    }

    private var autoSkipSection: some View {
        Section("Auto-Skip") {
            Toggle("Skip based on a sensor", isOn: $useAutoSkip)

            if useAutoSkip {
                Picker("Sensor", selection: sensorBinding) {
                    ForEach(sensorOptions, id: \.self) { name in
                        Text(name).tag(name)
                    }
                }

                VStack(alignment: .leading, spacing: 4) {
                    TextField("Skip when (e.g. ==0)", text: whenBinding)
                    Text("Operator + value, e.g. ==0, >0, !=0.")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
            }
        }
    }

    // MARK: - Option lists
    //
    // Each list keeps the step's current value even when the server no longer
    // advertises it, so opening the editor can never silently drop a setting.

    private func options(_ discovered: [String], current: String?) -> [String] {
        var all = discovered
        if let current, !current.isEmpty, !all.contains(current) {
            all.append(current)
        }
        return all
    }

    private var skillOptions: [String] {
        options(metadata.skills.map(\.name), current: draft.skill)
    }

    private var categoryOptions: [String] {
        options(metadata.categories, current: draft.category)
    }

    private var agentOptions: [String] {
        options(metadata.agents.map { $0.name ?? $0.id }, current: draft.agent)
    }

    private var modelOptions: [String] {
        options(metadata.models.map(\.id), current: draft.modelOverride?.id)
    }

    private var backendOptions: [String] {
        options(backends, current: draft.backendOverride)
    }

    private var sensorOptions: [String] {
        options(metadata.sensors, current: draft.autoSkip?.sensor)
    }

    // MARK: - Bindings for optional fields

    private var agentBinding: Binding<String> {
        Binding(get: { draft.agent ?? "" }, set: { draft.agent = $0.isEmpty ? nil : $0 })
    }

    private var promptSuffixBinding: Binding<String> {
        Binding(get: { draft.promptSuffix ?? "" }, set: { draft.promptSuffix = $0.isEmpty ? nil : $0 })
    }

    private var modelIDBinding: Binding<String> {
        Binding(
            get: { draft.modelOverride?.id ?? "" },
            set: { newValue in
                var override = draft.modelOverride ?? StepModelOverride(id: "")
                override.id = newValue
                draft.modelOverride = override
            }
        )
    }

    private var variantBinding: Binding<String> {
        Binding(
            get: { draft.modelOverride?.variant ?? "" },
            set: { newValue in
                var override = draft.modelOverride ?? StepModelOverride(id: "")
                override.variant = newValue.isEmpty ? nil : newValue
                draft.modelOverride = override
            }
        )
    }

    private var backendBinding: Binding<String> {
        Binding(get: { draft.backendOverride ?? "" }, set: { draft.backendOverride = $0.isEmpty ? nil : $0 })
    }

    private var sensorBinding: Binding<String> {
        Binding(
            get: { draft.autoSkip?.sensor ?? metadata.sensors.first ?? "" },
            set: { draft.autoSkip = AutoSkip(sensor: $0, when: draft.autoSkip?.when ?? "==0") }
        )
    }

    private var whenBinding: Binding<String> {
        Binding(
            get: { draft.autoSkip?.when ?? "==0" },
            set: { draft.autoSkip = AutoSkip(sensor: draft.autoSkip?.sensor ?? metadata.sensors.first ?? "", when: $0) }
        )
    }

    // MARK: - Save

    private func save() {
        var updated = draft
        updated.name = trimmedName

        if useModelOverride {
            var override = updated.modelOverride ?? StepModelOverride(id: "")
            let temperature = temperatureText.trimmingCharacters(in: .whitespaces)
            if temperature.isEmpty {
                override.temperature = nil
            } else if let value = Double(temperature) {
                override.temperature = value
            } else {
                errorMessage = "Temperature must be a number (e.g. 0.2)."
                return
            }
            let fallbacks = fallbackText
                .split(separator: ",")
                .map { $0.trimmingCharacters(in: .whitespaces) }
                .filter { !$0.isEmpty }
            override.fallbackModels = fallbacks.isEmpty ? nil : fallbacks
            guard !override.id.isEmpty else {
                errorMessage = "Pick a model or turn the override off."
                return
            }
            updated.modelOverride = override
        } else {
            updated.modelOverride = nil
        }

        if !useBackendOverride {
            updated.backendOverride = nil
        } else if (updated.backendOverride ?? "").isEmpty {
            errorMessage = "Pick a backend or turn the override off."
            return
        }

        if useAutoSkip {
            let rule = updated.autoSkip ?? AutoSkip(sensor: metadata.sensors.first ?? "", when: "==0")
            guard !rule.sensor.isEmpty, !rule.when.trimmingCharacters(in: .whitespaces).isEmpty else {
                errorMessage = "Auto-skip needs a sensor and a condition."
                return
            }
            updated.autoSkip = rule
        } else {
            updated.autoSkip = nil
        }

        isSaving = true
        Task {
            let error = await store.saveStep(updated)
            isSaving = false
            if let error {
                errorMessage = error
            } else {
                toasts.show("Step saved", kind: .success)
                dismiss()
            }
        }
    }
}
