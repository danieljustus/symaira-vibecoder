import SwiftUI
import SymvibeKit
#if canImport(UIKit)
import UIKit
#endif
#if canImport(AppKit)
import AppKit
#endif

/// Displays a pairing QR code generated from a symvibe server's pairing payload.
///
/// Calls `POST /api/pair/start` to obtain a pairing descriptor,
/// then renders the `symvibe://pair?…` URL as a QR code using `CIQRCodeGenerator`.
struct PairingQRView: View {
    let serverURL: URL
    let deviceName: String

    @State private var qrImage: Image?
    @State private var pairingURL: String?
    @State private var isLoading = true
    @State private var errorMessage: String?
    @State private var countdown: TimeInterval = 120
    @State private var countdownTask: Task<Void, Never>?

    var body: some View {
        VStack(spacing: 20) {
            // Header
            VStack(spacing: 8) {
                Image(systemName: "qrcode")
                    .font(.system(size: 48))
                    .foregroundStyle(.tint)

                Text("Scan to Pair")
                    .font(.title2.bold())

                Text("Open the symvibe app on your iPhone and scan this QR code.")
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                    .font(.callout)
            }
            .padding(.top, 20)

            // QR Code
            if isLoading {
                ProgressView("Generating pairing code…")
                    .frame(height: 200)
            } else if let qrImage {
                qrImage
                    .interpolation(.none)
                    .resizable()
                    .scaledToFit()
                    .frame(width: 220, height: 220)
                    .padding(16)
                    .background(Color.white)
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                    .shadow(color: .black.opacity(0.1), radius: 8, y: 4)
            } else if let error = errorMessage {
                ContentUnavailableView {
                    Label("Pairing Failed", systemImage: "exclamationmark.triangle")
                } description: {
                    Text(error)
                } actions: {
                    Button("Retry") {
                        Task { await generateQR() }
                    }
                    .buttonStyle(.borderedProminent)
                }
            }

            // Countdown / info
            if qrImage != nil {
                VStack(spacing: 4) {
                    Text("Code expires in \(Int(countdown))s")
                        .font(.caption)
                        .foregroundStyle(.secondary)

                    ProgressView(value: countdown, total: 120)
                        .tint(countdown < 30 ? .orange : .accentColor)
                }
                .padding(.horizontal)
            }

            // Copy URL button
            if let url = pairingURL {
                Button {
                    #if os(macOS)
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(url, forType: .string)
                    #else
                    UIPasteboard.general.string = url
                    #endif
                } label: {
                    Label("Copy Pairing URL", systemImage: "doc.on.doc")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }

            Spacer()

            // Refresh button
            Button {
                Task { await generateQR() }
            } label: {
                Label("Generate New Code", systemImage: "arrow.clockwise")
                    .font(.subheadline)
            }
            .buttonStyle(.plain)
                .foregroundStyle(Color.accentColor)
            .padding(.bottom, 20)
        }
        .task {
            await generateQR()
        }
        .onDisappear {
            countdownTask?.cancel()
        }
    }

    // MARK: - Pairing

    private func generateQR() async {
        isLoading = true
        errorMessage = nil
        qrImage = nil
        countdownTask?.cancel()

        let pairURL = serverURL.appendingPathComponent("/api/pair/start")

        var request = URLRequest(url: pairURL)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.timeoutInterval = 10

        struct PairStartResponse: Decodable {
            let url: String
            let code: String
            let expiresAt: Int64?
        }

        do {
            let (data, response) = try await URLSession.shared.data(for: request)
            guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
                let status = (response as? HTTPURLResponse)?.statusCode ?? 0
                errorMessage = "Server returned HTTP \(status)"
                isLoading = false
                return
            }

            let decoded = try JSONDecoder().decode(PairStartResponse.self, from: data)
            pairingURL = decoded.url

            // Generate QR code from the URL
            if let qrImg = generateQRImage(from: decoded.url) {
                qrImage = qrImg
                isLoading = false

                // Start countdown
                startCountdown()
            } else {
                errorMessage = "Failed to generate QR code image"
                isLoading = false
            }
        } catch {
            errorMessage = error.localizedDescription
            isLoading = false
        }
    }

    private func generateQRImage(from string: String) -> Image? {
        guard let data = string.data(using: .utf8),
              let filter = CIFilter(name: "CIQRCodeGenerator") else {
            return nil
        }

        filter.setValue(data, forKey: "inputMessage")
        filter.setValue("H", forKey: "inputCorrectionLevel") // High error correction

        guard let ciImage = filter.outputImage else { return nil }

        // Scale up for crisp rendering
        let scale = 10.0
        let transform = CGAffineTransform(scaleX: scale, y: scale)
        let scaledImage = ciImage.transformed(by: transform)

        // Render to bitmap
        let context = CIContext()
        guard let cgImage = context.createCGImage(scaledImage, from: scaledImage.extent) else {
            return nil
        }

        #if os(macOS)
        let nsImage = NSImage(cgImage: cgImage, size: NSSize(width: scaledImage.extent.width, y: scaledImage.extent.height))
        return Image(nsImage: nsImage)
        #else
        let uiImage = UIImage(cgImage: cgImage)
        return Image(uiImage: uiImage)
        #endif
    }

    private func startCountdown() {
        countdown = 120
        countdownTask = Task {
            while countdown > 0 && !Task.isCancelled {
                try? await Task.sleep(for: .seconds(1))
                countdown -= 1
            }
            if countdown <= 0 {
                // Code expired — offer refresh
                qrImage = nil
                errorMessage = "Pairing code expired. Generate a new one."
            }
        }
    }
}
