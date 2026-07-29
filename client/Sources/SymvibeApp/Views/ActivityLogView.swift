import SwiftUI
import SymvibeKit

struct ActivityLogView: View {
    let activityStore: ActivityStore
    /// When false the view stops following new output, so the user can read
    /// back through the log while a step keeps writing.
    var autoScroll: Bool = true

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            if activityStore.lines.isEmpty {
                ContentUnavailableView(
                    "No Activity",
                    systemImage: "text.alignleft",
                    description: Text("Log output from the running step will appear here.")
                )
            } else {
                ScrollViewReader { proxy in
                    ScrollView([.horizontal, .vertical]) {
                        LazyVStack(alignment: .leading, spacing: 2) {
                            ForEach(activityStore.lines) { line in
                                LogLineRow(line: line)
                                    .id(line.id)
                            }
                        }
                        .padding(8)
                    }
                    .onChange(of: activityStore.lines.count) {
                        guard autoScroll, let last = activityStore.lines.last else { return }
                        proxy.scrollTo(last.id, anchor: .bottom)
                    }
                }
            }
        }
        .frame(minHeight: 120)
    }
}

private struct LogLineRow: View {
    let line: ActivityStore.LogLine

    var body: some View {
        Text(line.text)
            .font(.caption.monospaced())
            .foregroundStyle(line.kind == .error ? .red : .primary)
            .textSelection(.enabled)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}
