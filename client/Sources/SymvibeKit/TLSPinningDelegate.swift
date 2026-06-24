import Foundation

/// `URLSessionDelegate` that pins TLS connections to a specific certificate fingerprint.
///
/// Used by ``PairingClient`` during the pairing handshake and should be used for all
/// `APIClient` / `SSEClient` sessions to enforce certificate trust. The delegate accepts
/// a connection only when the leaf certificate's SHA-256 fingerprint matches
/// `expectedFingerprint` (case-insensitive, colon/space tolerant).
///
/// Pass an instance as the `delegate:` when creating a `URLSession`:
///
///     let delegate = TLSPinningDelegate(expectedFingerprint: "ab:cd:…")
///     let session = URLSession(configuration: .ephemeral, delegate: delegate, delegateQueue: nil)
///
/// For App Store builds the fingerprint is embedded at pairing time; for development
/// the self-signed cert from `symvibe serve` is accepted via the QR payload.
public final class TLSPinningDelegate: NSObject, URLSessionDelegate, Sendable {
    public let expectedFingerprint: String

    public init(expectedFingerprint: String) {
        self.expectedFingerprint = expectedFingerprint
    }

    public func urlSession(
        _ session: URLSession,
        didReceive challenge: URLAuthenticationChallenge,
        completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void
    ) {
        guard let serverTrust = challenge.protectionSpace.serverTrust,
              let certificateChain = SecTrustCopyCertificateChain(serverTrust) as? [SecCertificate],
              let certificate = certificateChain.first else {
            completionHandler(.cancelAuthenticationChallenge, nil)
            return
        }
        let data = SecCertificateCopyData(certificate) as Data
        let fingerprint = Self.fingerprint(for: data)
        guard normalize(fingerprint) == normalize(expectedFingerprint) else {
            completionHandler(.cancelAuthenticationChallenge, nil)
            return
        }
        let credential = URLCredential(trust: serverTrust)
        completionHandler(.useCredential, credential)
    }

    static func fingerprint(for data: Data) -> String {
        let hash = SHA256.hash(data: data)
        return hash.map { String(format: "%02x", $0) }.joined()
    }

    private func normalize(_ s: String) -> String {
        s.lowercased().replacingOccurrences(of: ":", with: "").replacingOccurrences(of: " ", with: "")
    }
}

// Minimal SHA-256 implementation using CommonCrypto / CryptoKit depending on platform.
#if canImport(CryptoKit)
import CryptoKit
private enum SHA256 {
    static func hash(data: Data) -> [UInt8] {
        let digest = CryptoKit.SHA256.hash(data: data)
        return Array(digest)
    }
}
#elseif canImport(CommonCrypto)
import CommonCrypto
private enum SHA256 {
    static func hash(data: Data) -> [UInt8] {
        var digest = [UInt8](repeating: 0, count: Int(CC_SHA256_DIGEST_LENGTH))
        data.withUnsafeBytes { buffer in
            _ = CC_SHA256(buffer.baseAddress, CC_LONG(buffer.count), &digest)
        }
        return digest
    }
}
#else
private enum SHA256 {
    static func hash(data: Data) -> [UInt8] {
        fatalError("SHA-256 not available on this platform")
    }
}
#endif
