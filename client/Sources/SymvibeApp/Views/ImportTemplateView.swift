import SwiftUI
import UniformTypeIdentifiers
import SymvibeKit

/// Template import — file, pasted JSON, or a library entry — including the
/// category remap step the server asks for when requirements are unmet.
struct ImportTemplateView: View {
    let store: BoardStore
    let toasts: ToastCenter
    let preloaded: Template?
    let onImported: (() -> Void)?

    @State private var template: Template?
    @State private var pastedJSON = ""
    @State private var sourceLabel = ""
    @State private var parseError: String?
    @State private var isImporting = false
    @State private var missing: MissingRequirements?
    @State private var available = Catalog()
    @State private var remap: [String: String] = [:]
    @State private var showFileImporter = false

    @Environment(\.dismiss) private var dismiss

    init(
        store: BoardStore,
        toasts: ToastCenter,
        preloaded: Template? = nil,
        onImported: (() -> Void)? = nil
    ) {
        self.store = store
        self.toasts = toasts
        self.preloaded = preloaded
        self.onImported = onImported
        _template = State(initialValue: preloaded)
        _sourceLabel = State(initialValue: preloaded == nil ? "" : "Library")
    }

    var body: some View {
        NavigationStack {
            Form {
                if template == nil {
                    sourceSection
                } else {
                    previewSection
                    if let missing {
                        requirementsSection(missing)
                    }
                }
            }
            .formStyle(.grouped)
            .navigationTitle("Import Template")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button(missing == nil ? "Import" : "Import with Remap") {
                        Task { await performImport() }
                    }
                    .disabled(template == nil || isImporting || !remapComplete)
                }
            }
            .overlay {
                if isImporting {
                    ProgressView("Importing…")
                        .padding()
                        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 8))
                }
            }
            .fileImporter(
                isPresented: $showFileImporter,
                allowedContentTypes: [.json],
                allowsMultipleSelection: false
            ) { result in
                handleFileImport(result)
            }
        }
        #if os(macOS)
        .frame(minWidth: 560, minHeight: 520)
        #endif
    }

    // MARK: - Sections

    private var sourceSection: some View {
        Group {
            Section("From File") {
                Button {
                    showFileImporter = true
                } label: {
                    Label("Choose a template JSON file…", systemImage: "folder")
                }
            }

            Section("Paste JSON") {
                TextEditor(text: $pastedJSON)
                    .font(.caption.monospaced())
                    .frame(minHeight: 160)
                    .overlay(
                        RoundedRectangle(cornerRadius: 6)
                            .strokeBorder(Color.cardSeparator, lineWidth: 0.5)
                    )

                Button("Parse Pasted JSON") {
                    parse(Data(pastedJSON.utf8), source: "Pasted JSON")
                }
                .disabled(pastedJSON.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

                if let parseError {
                    Text(parseError)
                        .font(.caption)
                        .foregroundStyle(.red)
                }
            }
        }
    }

    @ViewBuilder
    private var previewSection: some View {
        if let template {
            Section("Template") {
                LabeledContent("Name", value: template.manifest.name.isEmpty ? template.manifest.id : template.manifest.name)
                if !template.manifest.author.isEmpty {
                    LabeledContent("Author", value: template.manifest.author)
                }
                LabeledContent("Version", value: template.manifest.version)
                if !sourceLabel.isEmpty {
                    LabeledContent("Source", value: sourceLabel)
                }
                if !template.manifest.description.isEmpty {
                    Text(template.manifest.description)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                if !template.isValidKind {
                    Label("Unexpected kind “\(template.kind)” — import will likely be rejected.", systemImage: "exclamationmark.triangle")
                        .font(.caption)
                        .foregroundStyle(.orange)
                }
            }

            Section("Contents") {
                ForEach(template.phases) { phase in
                    DisclosureGroup {
                        ForEach(phase.steps) { step in
                            HStack(spacing: 6) {
                                Text(step.name)
                                    .font(.caption)
                                Spacer()
                                if !step.category.isEmpty {
                                    Text(step.category)
                                        .font(.caption2)
                                        .foregroundStyle(.secondary)
                                }
                            }
                        }
                    } label: {
                        HStack {
                            Text(phase.name)
                            Spacer()
                            Text("\(phase.steps.count) steps")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
            }

            Section {
                Label(
                    "Importing replaces the current cycle on the connected machine.",
                    systemImage: "exclamationmark.triangle.fill"
                )
                .font(.caption)
                .foregroundStyle(.orange)

                if preloaded == nil {
                    Button("Choose a different template") {
                        self.template = nil
                        missing = nil
                        remap = [:]
                    }
                }
            }
        }
    }

    @ViewBuilder
    private func requirementsSection(_ missing: MissingRequirements) -> some View {
        Section("Missing Requirements") {
            if !missing.categories.isEmpty {
                Text("Map each missing category onto a local one:")
                    .font(.caption)
                    .foregroundStyle(.secondary)

                ForEach(missing.categories, id: \.self) { category in
                    Picker(category, selection: remapBinding(for: category)) {
                        Text("(choose…)").tag("")
                        ForEach(available.categories, id: \.self) { local in
                            Text(local).tag(local)
                        }
                    }
                }
            }

            ForEach(unmappableGroups(missing), id: \.label) { group in
                VStack(alignment: .leading, spacing: 2) {
                    Text(group.label)
                        .font(.caption.weight(.medium))
                    Text(group.values.joined(separator: ", "))
                        .font(.caption2.monospaced())
                        .foregroundStyle(.secondary)
                }
            }

            if !missing.isRemappable && !hasOnlyCategories(missing) {
                Text("Install the missing skills, agents, or sensors on the machine running the engine — they cannot be remapped from here.")
                    .font(.caption)
                    .foregroundStyle(.orange)
            }
        }
    }

    // MARK: - Helpers

    private func unmappableGroups(_ missing: MissingRequirements) -> [(label: String, values: [String])] {
        [
            ("Skills not installed", missing.skills),
            ("Agents not available", missing.agents),
            ("Sensors unknown", missing.sensors),
        ].filter { !$0.1.isEmpty }.map { (label: $0.0, values: $0.1) }
    }

    private func hasOnlyCategories(_ missing: MissingRequirements) -> Bool {
        missing.skills.isEmpty && missing.agents.isEmpty && missing.sensors.isEmpty
    }

    /// Blocks the import button until every missing category has a target.
    private var remapComplete: Bool {
        guard let missing else { return true }
        return missing.categories.allSatisfy { !(remap[$0] ?? "").isEmpty }
    }

    private func remapBinding(for category: String) -> Binding<String> {
        Binding(get: { remap[category] ?? "" }, set: { remap[category] = $0 })
    }

    private func handleFileImport(_ result: Result<[URL], Error>) {
        switch result {
        case .failure(let error):
            parseError = error.localizedDescription
        case .success(let urls):
            guard let url = urls.first else { return }
            let needsScope = url.startAccessingSecurityScopedResource()
            defer { if needsScope { url.stopAccessingSecurityScopedResource() } }
            do {
                parse(try Data(contentsOf: url), source: url.lastPathComponent)
            } catch {
                parseError = "Could not read file: \(error.localizedDescription)"
            }
        }
    }

    private func parse(_ data: Data, source: String) {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        do {
            let parsed = try decoder.decode(Template.self, from: data)
            guard !parsed.phases.isEmpty else {
                parseError = "That JSON contains no phases."
                return
            }
            template = parsed
            sourceLabel = source
            parseError = nil
        } catch {
            parseError = "Not a valid symvibe template: \(error.localizedDescription)"
        }
    }

    private func performImport() async {
        guard let template else { return }
        isImporting = true
        defer { isImporting = false }

        switch await store.importTemplate(template, remap: remap) {
        case .imported:
            toasts.show("Template imported", kind: .success)
            onImported?()
            dismiss()
        case .missing(let missingRequirements, let catalog):
            missing = missingRequirements
            available = catalog
            // Preselect obvious 1:1 matches so only genuine conflicts need input.
            for category in missingRequirements.categories where remap[category] == nil {
                remap[category] = catalog.categories.first { $0.caseInsensitiveCompare(category) == .orderedSame } ?? ""
            }
            toasts.show("This machine is missing some requirements — remap them below.", kind: .error)
        case .failed(let error):
            toasts.show(error, kind: .error)
        }
    }
}
