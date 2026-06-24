import SwiftUI

// MARK: - Main Onboarding View

/// Onboarding screen — scan QR (iOS) or enter code manually (macOS/simulator).
public struct OnboardingView: View {
    var store: ConnectionStore
    @State private var showQRScanner = false
    @State private var showManualEntry = false
    @State private var errorMessage: String?
    @State private var isPairing = false

    public init(store: ConnectionStore = ConnectionStore()) {
        self.store = store
    }

    public var body: some View {
        NavigationStack {
            VStack(spacing: 24) {
                Spacer()

                Image(systemName: "bolt.ring")
                    .font(.system(size: 64))
                    .foregroundStyle(.tint)
                    .padding(.bottom, 8)

                Text("Connect to Mac")
                    .font(.largeTitle.bold())

                Text("Scan a QR code from your Mac's symvibe to connect, or enter the pairing payload manually.")
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 32)

                if isPairing {
                    ProgressView("Connecting…")
                        .padding(.top, 16)
                } else {
                    VStack(spacing: 12) {
                        #if os(iOS)
                        Button {
                            showQRScanner = true
                        } label: {
                            Label("Scan QR Code", systemImage: "qrcode.viewfinder")
                                .font(.headline)
                                .frame(maxWidth: .infinity)
                                .padding()
                                .background(.tint)
                                .foregroundStyle(.white)
                                .clipShape(RoundedRectangle(cornerRadius: 12))
                        }
                        #endif

                        Button {
                            showManualEntry = true
                        } label: {
                            Label("Enter Code Manually", systemImage: "keyboard")
                                .font(.headline)
                                .frame(maxWidth: .infinity)
                                .padding()
                                .background(.quaternary)
                                .clipShape(RoundedRectangle(cornerRadius: 12))
                        }
                    }
                    .padding(.horizontal, 32)
                    .padding(.top, 16)
                }

                if let error = errorMessage {
                    Text(error)
                        .foregroundStyle(.red)
                        .font(.callout)
                        .multilineTextAlignment(.center)
                        .padding(.horizontal, 32)
                        .transition(.opacity)
                }

                Spacer()
                Spacer()
            }
            #if os(iOS)
            .sheet(isPresented: $showQRScanner) {
                QRCodeScannerView { payload in
                    Task { await handlePayload(payload) }
                }
            }
            #endif
            .sheet(isPresented: $showManualEntry) {
                ManualCodeEntryView { payload in
                    Task { await handlePayload(payload) }
                }
            }
        }
    }

    // MARK: - Pairing

    private func handlePayload(_ payload: PairingPayload) async {
        isPairing = true
        errorMessage = nil
        defer { isPairing = false }

        do {
            let client = PairingClient()
            let (profile, token) = try await client.completePairing(payload: payload)
            store.add(profile)
            try store.saveDeviceToken(token, for: profile.id)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

// MARK: - QR Code Scanner (iOS only)

#if os(iOS)
import VisionKit

/// Camera-based QR code scanner using VisionKit's DataScannerViewController.
struct QRCodeScannerView: View {
    let onScan: (PairingPayload) -> Void
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        DataScannerRepresentable { scannedStrings in
            guard let first = scannedStrings.first,
                  let payload = try? PairingPayload.parse(first) else { return }
            dismiss()
            onScan(payload)
        }
        .ignoresSafeArea()
        .overlay(alignment: .topTrailing) {
            Button("Cancel") { dismiss() }
                .padding()
                .background(.ultraThinMaterial, in: Capsule())
        }
    }
}

/// UIViewRepresentable wrapper for DataScannerViewController.
struct DataScannerRepresentable: UIViewRepresentable {
    let onScanned: ([String]) -> Void

    func makeUIView(context: Context) -> DataScannerViewController {
        let scanner = DataScannerViewController(
            recognizedDataTypes: [.text()],
            qualityLevel: .accurate,
            recognizesMultipleItems: false,
            isHighlightingEnabled: false
        )
        scanner.delegate = context.coordinator
        try? scanner.startScanning()
        return scanner
    }

    func updateUIView(_ uiView: DataScannerViewController, context: Context) {}

    func makeCoordinator() -> Coordinator {
        Coordinator(onScanned: onScanned)
    }

    final class Coordinator: NSObject, DataScannerViewControllerDelegate {
        let onScanned: ([String]) -> Void

        init(onScanned: @escaping ([String]) -> Void) {
            self.onScanned = onScanned
        }

        func dataScanner(
            _ dataScanner: DataScannerViewController,
            didAdd addedItems: [RecognizedItem],
            allItems: [RecognizedItem]
        ) {
            let strings = addedItems.compactMap { item -> String? in
                if case .text(let text) = item { return text.transcript }
                return nil
            }
            guard !strings.isEmpty else { return }
            onScanned(strings)
        }
    }
}
#endif

// MARK: - Manual Code Entry (macOS + simulator fallback)

/// Paste-the-URL fallback for platforms without camera access.
struct ManualCodeEntryView: View {
    let onParsed: (PairingPayload) -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var codeText = ""
    @State private var errorMessage: String?

    var body: some View {
        NavigationStack {
            VStack(spacing: 16) {
                Text("Paste the full `symvibe://pair` URL from your Mac.")
                    .foregroundStyle(.secondary)
                    .font(.callout)
                    .padding(.top)

                TextEditor(text: $codeText)
                    .font(.system(.body, design: .monospaced))
                    .frame(minHeight: 120)
                    .padding(4)
                    .overlay(
                        RoundedRectangle(cornerRadius: 8)
                            .stroke(.quaternary)
                    )

                if let error = errorMessage {
                    Text(error)
                        .foregroundStyle(.red)
                        .font(.callout)
                }

                Button("Connect") {
                    guard let payload = try? PairingPayload.parse(codeText) else {
                        errorMessage = "Invalid payload. Paste the full symvibe://pair URL."
                        return
                    }
                    dismiss()
                    onParsed(payload)
                }
                .buttonStyle(.borderedProminent)
                .disabled(codeText.isEmpty)
            }
            .padding()
            .navigationTitle("Manual Pairing")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
        }
    }
}
