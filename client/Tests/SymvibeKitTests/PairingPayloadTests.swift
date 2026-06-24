import XCTest
@testable import SymvibeKit

final class PairingPayloadTests: XCTestCase {

    // MARK: - Valid payloads

    func testParseSingleHost() throws {
        let url = "symvibe://pair?n=Mac-Mini&p=4317&h=192.168.1.42&fp=deadbeef&c=ABC123"
        let payload = try PairingPayload.parse(url)
        XCTAssertEqual(payload.name, "Mac-Mini")
        XCTAssertEqual(payload.port, 4317)
        XCTAssertEqual(payload.hostCandidates, ["192.168.1.42"])
        XCTAssertEqual(payload.certFingerprint, "deadbeef")
        XCTAssertEqual(payload.pairingCode, "ABC123")
    }

    func testParseMultipleHosts() throws {
        let url = "symvibe://pair?n=Mac&p=4317&h=192.168.1.42&h=mac-mini.local&h=100.1.2.3&fp=abcd&c=XYZ"
        let payload = try PairingPayload.parse(url)
        XCTAssertEqual(payload.hostCandidates, ["192.168.1.42", "mac-mini.local", "100.1.2.3"])
    }

    func testParseDefaultsPortAndName() throws {
        // Omit p= and n= to test defaults
        let url = "symvibe://pair?h=localhost&fp=abc&c=123"
        let payload = try PairingPayload.parse(url)
        XCTAssertEqual(payload.name, "Mac")
        XCTAssertEqual(payload.port, 4317)
    }

    func testParseURLDecoding() throws {
        // Name with spaces should be URL-decoded
        let url = "symvibe://pair?n=Mac+Mini&p=4317&h=host&fp=fp&c=code"
        let payload = try PairingPayload.parse(url)
        XCTAssertEqual(payload.name, "Mac Mini")
    }

    func testParseEquatable() throws {
        let a = try PairingPayload.parse("symvibe://pair?n=A&p=4317&h=h1&fp=fp&c=c1")
        let b = try PairingPayload.parse("symvibe://pair?n=A&p=4317&h=h1&fp=fp&c=c1")
        XCTAssertEqual(a, b)
    }

    // MARK: - Invalid payloads

    func testRejectNonSymvibeScheme() {
        XCTAssertThrowsError(try PairingPayload.parse("https://example.com/pair?h=x&fp=y&c=z")) { error in
            XCTAssertTrue(error is PairingError)
        }
    }

    func testRejectWrongPath() {
        XCTAssertThrowsError(try PairingPayload.parse("symvibe://other?h=x&fp=y&c=z")) { error in
            XCTAssertTrue(error is PairingError)
        }
    }

    func testRejectMissingHosts() {
        XCTAssertThrowsError(try PairingPayload.parse("symvibe://pair?n=Mac&p=4317&fp=abc&c=xyz")) { error in
            guard let pe = error as? PairingError else {
                return XCTFail("Expected PairingError, got \(error)")
            }
            if case .noHostCandidates = pe { /* ok */ } else {
                XCTFail("Expected .noHostCandidates, got \(pe)")
            }
        }
    }

    func testRejectMissingFingerprint() {
        XCTAssertThrowsError(try PairingPayload.parse("symvibe://pair?n=Mac&p=4317&h=host&c=xyz")) { error in
            guard let pe = error as? PairingError else {
                return XCTFail("Expected PairingError, got \(error)")
            }
            if case .invalidPayload = pe { /* ok */ } else {
                XCTFail("Expected .invalidPayload, got \(pe)")
            }
        }
    }

    func testRejectMissingCode() {
        XCTAssertThrowsError(try PairingPayload.parse("symvibe://pair?n=Mac&p=4317&h=host&fp=abc")) { error in
            guard let pe = error as? PairingError else {
                return XCTFail("Expected PairingError, got \(error)")
            }
            if case .invalidPayload = pe { /* ok */ } else {
                XCTFail("Expected .invalidPayload, got \(pe)")
            }
        }
    }

    func testRejectEmptyString() {
        XCTAssertThrowsError(try PairingPayload.parse("")) { error in
            XCTAssertTrue(error is PairingError)
        }
    }

    // MARK: - baseURL

    func testBaseURLForHost() throws {
        let payload = try PairingPayload.parse(
            "symvibe://pair?n=Mac&p=4317&h=192.168.1.42&fp=fp&c=code"
        )
        let url = payload.baseURL(for: "192.168.1.42")
        XCTAssertEqual(url?.absoluteString, "https://192.168.1.42:4317")
    }
}
