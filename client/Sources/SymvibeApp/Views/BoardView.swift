import SwiftUI
import SymvibeKit

struct BoardView: View {
    let store: BoardStore

    var body: some View {
        ScrollView {
            if let cycle = store.cycle {
                BoardContent(cycle: cycle)
                    .padding()
            } else if store.isConnected {
                VStack(spacing: 16) {
                    ProgressView()
                    Text("Loading cycle…")
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .padding(.top, 64)
            } else if let error = store.lastError {
                ContentUnavailableView {
                    Label("Connection Error", systemImage: "wifi.slash")
                } description: {
                    Text(error)
                } actions: {
                    Button("Retry") {
                        Task { await store.refresh() }
                    }
                    .buttonStyle(.borderedProminent)
                }
            } else {
                ContentUnavailableView("Not Connected", systemImage: "wifi.slash")
            }
        }
        .navigationTitle(store.cycle?.name ?? "Board")
        .refreshable {
            await store.refresh()
        }
    }
}

// MARK: - Board Content

private struct BoardContent: View {
    let cycle: Cycle

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            if !cycle.description.isEmpty {
                Text(cycle.description)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }

            ForEach(cycle.phases) { phase in
                PhaseSection(phase: phase)
            }
        }
    }
}

// MARK: - Phase Section

private struct PhaseSection: View {
    let phase: Phase

    private var completedCount: Int {
        phase.steps.filter(\.status.isTerminal).count
    }

    private var totalCount: Int {
        phase.steps.count
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text(phase.name)
                    .font(.headline)
                Spacer()
                Text("\(completedCount)/\(totalCount)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                ProgressView(value: Double(completedCount), total: Double(totalCount))
                    .frame(width: 60)
            }

            LazyVGrid(
                columns: [GridItem(.adaptive(minimum: 260))],
                spacing: 8
            ) {
                ForEach(phase.steps) { step in
                    StepCard(step: step)
                }
            }
        }
    }
}

// MARK: - Step Card

private struct StepCard: View {
    let step: Step

    var body: some View {
        HStack(spacing: 10) {
            Text(step.status.glyph)
                .font(.title3.monospaced())
                .foregroundStyle(statusColor)
                .frame(width: 24, alignment: .center)

            VStack(alignment: .leading, spacing: 3) {
                Text(step.name)
                    .font(.subheadline.weight(.medium))
                    .lineLimit(1)

                HStack(spacing: 6) {
                    if !step.category.isEmpty {
                        Text(step.category)
                            .font(.caption2.weight(.medium))
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(statusColor.opacity(0.12))
                            .foregroundStyle(statusColor)
                            .clipShape(Capsule())
                    }
                    if !step.skill.isEmpty {
                        Text("$\(step.skill)")
                            .font(.caption2.monospaced())
                            .foregroundStyle(.secondary)
                    }
                }
            }

            Spacer(minLength: 0)

            if !step.enabled {
                Image(systemName: "pause.circle")
                    .font(.caption)
                    .foregroundStyle(.tertiary)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
        .background(.background)
        .clipShape(RoundedRectangle(cornerRadius: 10))
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .strokeBorder(
                    step.enabled ? Color(.separator) : Color(.separator).opacity(0.4),
                    lineWidth: 0.5
                )
        )
        .opacity(step.enabled ? 1.0 : 0.6)
    }

    private var statusColor: Color {
        switch step.status {
        case .pending: .secondary
        case .inProgress: .blue
        case .done: .green
        case .skipped: .secondary
        case .failed: .red
        case .blocked: .orange
        case .needsReview: .yellow
        }
    }
}
