import SwiftUI
import SymvibeKit

/// Community template browser — the native counterpart of the web board's
/// library panel, with search and tag filtering.
struct LibraryView: View {
    let store: BoardStore
    let toasts: ToastCenter

    @State private var entries: [LibraryEntry] = []
    @State private var searchText = ""
    @State private var selectedTag: String?
    @State private var isLoading = true
    @State private var loadError: String?
    @State private var pendingTemplate: Template?
    @State private var busyEntryID: String?

    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            content
                .navigationTitle("Template Library")
                #if os(iOS)
                .navigationBarTitleDisplayMode(.inline)
                #endif
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Close") { dismiss() }
                    }
                    ToolbarItem(placement: .primaryAction) {
                        Button {
                            Task { await load(force: true) }
                        } label: {
                            Label("Reload", systemImage: "arrow.clockwise")
                        }
                        .disabled(isLoading)
                    }
                }
                .searchable(text: $searchText, prompt: "Search templates")
                .task { await load(force: false) }
                .sheet(item: templateSheetBinding) { template in
                    ImportTemplateView(store: store, toasts: toasts, preloaded: template) {
                        dismiss()
                    }
                }
        }
        #if os(macOS)
        .frame(minWidth: 640, minHeight: 520)
        #endif
    }

    @ViewBuilder
    private var content: some View {
        if isLoading {
            ProgressView("Loading library…")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if let loadError {
            ContentUnavailableView {
                Label("Library Unavailable", systemImage: "icloud.slash")
            } description: {
                Text(loadError)
            } actions: {
                Button("Retry") { Task { await load(force: true) } }
                    .buttonStyle(.borderedProminent)
            }
        } else if filtered.isEmpty {
            ContentUnavailableView.search(text: searchText)
        } else {
            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    if !allTags.isEmpty {
                        tagFilter
                    }

                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 260), spacing: 12)], spacing: 12) {
                        ForEach(filtered) { entry in
                            card(for: entry)
                        }
                    }
                }
                .padding()
            }
        }
    }

    private var tagFilter: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 6) {
                tagChip(title: "All", isSelected: selectedTag == nil) { selectedTag = nil }
                ForEach(allTags, id: \.self) { tag in
                    tagChip(title: tag, isSelected: selectedTag == tag) {
                        selectedTag = selectedTag == tag ? nil : tag
                    }
                }
            }
            .padding(.horizontal, 2)
        }
    }

    private func tagChip(title: String, isSelected: Bool, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(.caption)
                .padding(.horizontal, 10)
                .padding(.vertical, 4)
                .background(isSelected ? Color.accentColor : Color.cardSeparator.opacity(0.25))
                .foregroundStyle(isSelected ? Color.white : Color.primary)
                .clipShape(Capsule())
        }
        .buttonStyle(.plain)
    }

    private func card(for entry: LibraryEntry) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(entry.name.isEmpty ? entry.id : entry.name)
                .font(.headline)
                .lineLimit(2)

            if !entry.author.isEmpty {
                Text("by \(entry.author)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            if !entry.description.isEmpty {
                Text(entry.description)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(4)
            }

            if !entry.tags.isEmpty {
                HStack(spacing: 4) {
                    ForEach(entry.tags.prefix(4), id: \.self) { tag in
                        Text(tag)
                            .font(.caption2)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(Color.cardSeparator.opacity(0.25))
                            .clipShape(Capsule())
                    }
                }
            }

            Spacer(minLength: 0)

            Button {
                Task { await preview(entry) }
            } label: {
                if busyEntryID == entry.id {
                    ProgressView()
                        .controlSize(.small)
                        .frame(maxWidth: .infinity)
                } else {
                    Text("Preview & Import")
                        .frame(maxWidth: .infinity)
                }
            }
            .buttonStyle(.bordered)
            .disabled(busyEntryID != nil || entry.url.isEmpty)
        }
        .padding(12)
        .frame(maxWidth: .infinity, minHeight: 150, alignment: .topLeading)
        .background(Color.cardBackground)
        .clipShape(RoundedRectangle(cornerRadius: 10))
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .strokeBorder(Color.cardSeparator, lineWidth: 0.5)
        )
    }

    // MARK: - Data

    private var allTags: [String] {
        Array(Set(entries.flatMap(\.tags))).sorted()
    }

    private var filtered: [LibraryEntry] {
        let query = searchText.trimmingCharacters(in: .whitespaces).lowercased()
        return entries.filter { entry in
            if let selectedTag, !entry.tags.contains(selectedTag) { return false }
            guard !query.isEmpty else { return true }
            return entry.name.lowercased().contains(query)
                || entry.description.lowercased().contains(query)
                || entry.author.lowercased().contains(query)
                || entry.tags.contains { $0.lowercased().contains(query) }
        }
    }

    private var templateSheetBinding: Binding<Template?> {
        Binding(get: { pendingTemplate }, set: { pendingTemplate = $0 })
    }

    private func load(force: Bool) async {
        if !force && !entries.isEmpty { return }
        isLoading = true
        loadError = nil
        switch await store.libraryIndex() {
        case .success(let list):
            entries = list
        case .failure(let error):
            loadError = error.message
        }
        isLoading = false
    }

    /// Downloads the template so the user reviews it before anything is
    /// written to the board.
    private func preview(_ entry: LibraryEntry) async {
        busyEntryID = entry.id
        defer { busyEntryID = nil }
        switch await store.fetchLibraryTemplate(url: entry.url) {
        case .success(let template):
            pendingTemplate = template
        case .failure(let error):
            toasts.show(error.message, kind: .error)
        }
    }
}

extension Template: @retroactive Identifiable {
    public var id: String { manifest.id.isEmpty ? manifest.name : manifest.id }
}
