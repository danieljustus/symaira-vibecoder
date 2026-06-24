import XCTest
@testable import SymvibeKit

@MainActor
final class ConnectionStoreTests: XCTestCase {

    private func makeStore() -> ConnectionStore {
        let suiteName = "test-symvibe-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suiteName)!
        return ConnectionStore(defaults: defaults)
    }

    private func sampleProfile(name: String = "Test Mac") -> ConnectionProfile {
        ConnectionProfile(
            name: name,
            hostCandidates: ["192.168.1.10"],
            port: 4317,
            certFingerprint: "aabb"
        )
    }

    // MARK: - Add / Remove

    func testAddProfile() {
        let store = makeStore()
        let profile = sampleProfile()
        store.add(profile)

        XCTAssertEqual(store.profiles.count, 1)
        XCTAssertEqual(store.profiles.first?.name, "Test Mac")
        // First added profile becomes active
        XCTAssertEqual(store.activeProfileID, profile.id)
    }

    func testAddMultipleProfiles() {
        let store = makeStore()
        let a = sampleProfile(name: "Mac 1")
        let b = sampleProfile(name: "Mac 2")
        store.add(a)
        store.add(b)

        XCTAssertEqual(store.profiles.count, 2)
        // First profile is still active
        XCTAssertEqual(store.activeProfileID, a.id)
    }

    func testRemoveProfile() {
        let store = makeStore()
        let profile = sampleProfile()
        store.add(profile)
        store.remove(profile.id)

        XCTAssertTrue(store.profiles.isEmpty)
        XCTAssertNil(store.activeProfileID)
    }

    func testRemoveActiveProfileFallsBackToNext() {
        let store = makeStore()
        let a = sampleProfile(name: "A")
        let b = sampleProfile(name: "B")
        store.add(a)
        store.add(b)
        store.remove(a.id)

        XCTAssertEqual(store.profiles.count, 1)
        XCTAssertEqual(store.activeProfileID, b.id)
    }

    // MARK: - Active Profile

    func testSetActive() {
        let store = makeStore()
        let a = sampleProfile(name: "A")
        let b = sampleProfile(name: "B")
        store.add(a)
        store.add(b)
        store.setActive(b.id)

        XCTAssertEqual(store.activeProfileID, b.id)
        XCTAssertEqual(store.activeProfile?.name, "B")
    }

    func testActiveProfileNilWhenEmpty() {
        let store = makeStore()
        XCTAssertNil(store.activeProfile)
    }

    // MARK: - Update

    func testUpdateProfile() {
        let store = makeStore()
        var profile = sampleProfile()
        store.add(profile)

        profile.name = "Renamed"
        store.update(profile)

        XCTAssertEqual(store.profiles.first?.name, "Renamed")
    }

    // MARK: - Persistence (round-trip through UserDefaults)

    func testPersistenceRoundTrip() {
        let suiteName = "test-symvibe-persist-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suiteName)!

        // Write
        let store1 = ConnectionStore(defaults: defaults)
        store1.add(sampleProfile(name: "Persisted"))
        store1.add(sampleProfile(name: "Persisted 2"))

        // Read in a fresh store
        let store2 = ConnectionStore(defaults: defaults)
        XCTAssertEqual(store2.profiles.count, 2)
        XCTAssertEqual(store2.profiles[0].name, "Persisted")
        XCTAssertEqual(store2.profiles[1].name, "Persisted 2")
    }

    // MARK: - KeychainAccount

    func testKeychainAccountDerivedFromID() {
        let id = UUID()
        let profile = ConnectionProfile(
            id: id,
            name: "X",
            hostCandidates: ["h"],
            port: 4317,
            certFingerprint: "fp"
        )
        XCTAssertEqual(profile.keychainAccount, "symvibe.device.\(id.uuidString)")
    }

    func testCustomKeychainAccount() {
        let profile = ConnectionProfile(
            name: "X",
            hostCandidates: ["h"],
            port: 4317,
            certFingerprint: "fp",
            keychainAccount: "custom-key"
        )
        XCTAssertEqual(profile.keychainAccount, "custom-key")
    }
}
