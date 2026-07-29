import SwiftUI
import SymvibeKit

/// Agent-backed cycle editor (`POST /api/cycle/assist`). The call drives a full
/// runner invocation server-side, so it can take minutes.
struct AssistView: View {
    let store: BoardStore
    let toasts: ToastCenter

    @State private var instruction = ""
    @State private var isRunning = false
    @State private var errorMessage: String?

    @Environment(\.dismiss) private var dismiss

    private let examples = [
        "Add a security review step after the code review phase.",
        "Remove every step that touches releases.",
        "Split the cleaning phase into branch cleanup and commit cleanup.",
    ]

    var body: some View {
        NavigationStack {
            Form {
                Section("Instruction") {
                    TextEditor(text: $instruction)
                        .font(.body)
                        .frame(minHeight: 120)
                        .overlay(
                            RoundedRectangle(cornerRadius: 6)
                                .strokeBorder(Color.cardSeparator, lineWidth: 0.5)
                        )
                        .disabled(isRunning)

                    Text("Describe the change in plain language. The coding agent rewrites the whole cycle and the result is saved immediately.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }

                Section("Examples") {
                    ForEach(examples, id: \.self) { example in
                        Button(example) {
                            instruction = example
                        }
                        .buttonStyle(.plain)
                        .font(.caption)
                        .disabled(isRunning)
                    }
                }

                Section {
                    Label(
                        "The assistant replaces the current cycle. Export a copy first if you want a fallback.",
                        systemImage: "exclamationmark.triangle.fill"
                    )
                    .font(.caption)
                    .foregroundStyle(.orange)
                }

                if let errorMessage {
                    Section("Failed") {
                        Text(errorMessage)
                            .font(.caption)
                            .foregroundStyle(.red)
                            .textSelection(.enabled)
                    }
                }
            }
            .formStyle(.grouped)
            .navigationTitle("Cycle Assistant")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                        .disabled(isRunning)
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Run Assistant") {
                        Task { await run() }
                    }
                    .disabled(isRunning || instruction.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
            .overlay {
                if isRunning {
                    VStack(spacing: 10) {
                        ProgressView()
                        Text("The agent is rewriting the cycle — this can take a few minutes.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .multilineTextAlignment(.center)
                    }
                    .padding(20)
                    .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 10))
                    .padding(40)
                }
            }
        }
        #if os(macOS)
        .frame(minWidth: 520, minHeight: 480)
        #endif
    }

    private func run() async {
        isRunning = true
        errorMessage = nil
        defer { isRunning = false }

        switch await store.assist(instruction: instruction.trimmingCharacters(in: .whitespacesAndNewlines)) {
        case .success:
            toasts.show("Cycle updated by the assistant", kind: .success)
            dismiss()
        case .failure(let error):
            errorMessage = error.message
        }
    }
}
