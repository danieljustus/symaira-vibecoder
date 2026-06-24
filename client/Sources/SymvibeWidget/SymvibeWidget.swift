import WidgetKit
import SwiftUI
import SymvibeKit

// MARK: - Timeline Entry

struct SymvibeEntry: TimelineEntry {
    let date: Date
    let stepName: String
    let stepStatus: String
    let phaseName: String
    let cycleName: String
    let runState: String
}

// MARK: - Timeline Provider

struct SymvibeTimelineProvider: TimelineProvider {
    func placeholder(in context: Context) -> SymvibeEntry {
        SymvibeEntry(
            date: .now,
            stepName: "Branch Cleanup",
            stepStatus: "○",
            phaseName: "Cleaning",
            cycleName: "symvibe",
            runState: "idle"
        )
    }

    func getSnapshot(in context: Context, completion: @escaping (SymvibeEntry) -> Void) {
        completion(placeholder(in: context))
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<SymvibeEntry>) -> Void) {
        let entry = readEntry()
        // Reload every 5 minutes to pick up status changes.
        let nextUpdate = Calendar.current.date(byAdding: .minute, value: 5, to: .now)!
        let timeline = Timeline(entries: [entry], policy: .after(nextUpdate))
        completion(timeline)
    }

    private func readEntry() -> SymvibeEntry {
        guard let defaults = WidgetShared.sharedDefaults else {
            return SymvibeEntry(
                date: .now,
                stepName: "No Connection",
                stepStatus: "–",
                phaseName: "",
                cycleName: "",
                runState: "idle"
            )
        }

        let stepName = defaults.string(forKey: WidgetShared.Keys.currentStepName) ?? "Idle"
        let statusRaw = defaults.string(forKey: WidgetShared.Keys.currentStepStatus) ?? "pending"
        let phaseName = defaults.string(forKey: WidgetShared.Keys.currentPhaseName) ?? ""
        let cycleName = defaults.string(forKey: WidgetShared.Keys.cycleName) ?? ""
        let runState = defaults.string(forKey: WidgetShared.Keys.runState) ?? "idle"

        let glyph: String
        if let status = StepStatus(rawValue: statusRaw) {
            glyph = status.glyph
        } else {
            glyph = "○"
        }

        return SymvibeEntry(
            date: .now,
            stepName: stepName,
            stepStatus: glyph,
            phaseName: phaseName,
            cycleName: cycleName,
            runState: runState
        )
    }
}

// MARK: - Widget View

struct SymvibeWidgetEntryView: View {
    let entry: SymvibeEntry
    @Environment(\.widgetFamily) var family

    var body: some View {
        switch family {
        case .systemSmall:
            smallView
        case .systemMedium:
            mediumView
        case .accessoryCircular:
            circularView
        case .accessoryRectangular:
            rectangularView
        case .accessoryInline:
            inlineView
        @unknown default:
            smallView
        }
    }

    // MARK: - Small

    private var smallView: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 4) {
                Text(entry.stepStatus)
                    .font(.system(size: 28, weight: .bold, design: .monospaced))
                Spacer()
            }

            Text(entry.stepName)
                .font(.system(size: 13, weight: .semibold, design: .rounded))
                .lineLimit(2)
                .minimumScaleFactor(0.7)

            if !entry.phaseName.isEmpty {
                Text(entry.phaseName)
                    .font(.system(size: 11, weight: .medium, design: .rounded))
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }

            Spacer(minLength: 0)

            if !entry.cycleName.isEmpty {
                Text(entry.cycleName)
                    .font(.system(size: 10, weight: .regular, design: .monospaced))
                    .foregroundStyle(.tertiary)
                    .lineLimit(1)
            }
        }
        .padding(12)
    }

    // MARK: - Medium

    private var mediumView: some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 4) {
                Text(entry.stepStatus)
                    .font(.system(size: 36, weight: .bold, design: .monospaced))

                Text(entry.stepName)
                    .font(.system(size: 15, weight: .semibold, design: .rounded))
                    .lineLimit(1)

                if !entry.phaseName.isEmpty {
                    Text(entry.phaseName)
                        .font(.system(size: 12, weight: .medium, design: .rounded))
                        .foregroundStyle(.secondary)
                }
            }

            Spacer()

            VStack(alignment: .trailing, spacing: 4) {
                runStateBadge

                if !entry.cycleName.isEmpty {
                    Text(entry.cycleName)
                        .font(.system(size: 10, weight: .regular, design: .monospaced))
                        .foregroundStyle(.tertiary)
                        .lineLimit(1)
                }
            }
        }
        .padding(14)
    }

    // MARK: - Circular (watchOS / accessory)

    private var circularView: some View {
        Text(entry.stepStatus)
            .font(.system(size: 22, weight: .bold, design: .monospaced))
            .minimumScaleFactor(0.5)
    }

    // MARK: - Rectangular (lock screen / notification center)

    private var rectangularView: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text("\(entry.stepStatus) \(entry.stepName)")
                .font(.system(size: 13, weight: .semibold, design: .rounded))
                .lineLimit(1)
            if !entry.phaseName.isEmpty {
                Text(entry.phaseName)
                    .font(.system(size: 11, weight: .medium, design: .rounded))
                    .foregroundStyle(.secondary)
            }
        }
    }

    // MARK: - Inline

    private var inlineView: some View {
        Text("\(entry.stepStatus) \(entry.stepName)")
            .font(.system(size: 13, weight: .medium, design: .rounded))
    }

    // MARK: - Helpers

    private var runStateBadge: some View {
        Text(entry.runState.capitalized)
            .font(.system(size: 10, weight: .bold, design: .rounded))
            .padding(.horizontal, 6)
            .padding(.vertical, 2)
            .background(runStateColor.opacity(0.15), in: Capsule())
            .foregroundStyle(runStateColor)
    }

    private var runStateColor: Color {
        switch entry.runState {
        case "running": .green
        case "paused": .orange
        case "failed": .red
        default: .secondary
        }
    }
}

// MARK: - Widget Configuration

struct SymvibeWidget: Widget {
    let kind: String = "SymvibeWidget"

    var body: some WidgetConfiguration {
        StaticConfiguration(kind: kind, provider: SymvibeTimelineProvider()) { entry in
            SymvibeWidgetEntryView(entry: entry)
                .containerBackground(.fill.tertiary, for: .widget)
        }
        .configurationDisplayName("symvibe")
        .description("Current step and run status from your symvibe cycle.")
        .supportedFamilies([
            .systemSmall,
            .systemMedium,
            .accessoryCircular,
            .accessoryRectangular,
            .accessoryInline,
        ])
    }
}

// MARK: - Preview

#if DEBUG
struct SymvibeWidget_Previews: PreviewProvider {
    static var previews: some View {
        SymvibeWidgetEntryView(entry: SymvibeEntry(
            date: .now,
            stepName: "Branch Cleanup",
            stepStatus: "◐",
            phaseName: "Cleaning",
            cycleName: "symvibe",
            runState: "running"
        ))
        .previewContext(WidgetPreviewContext(family: .systemSmall))
    }
}
#endif
