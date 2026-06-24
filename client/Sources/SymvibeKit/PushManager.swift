import Foundation
#if canImport(UIKit)
import UIKit
#endif
#if canImport(UserNotifications)
import UserNotifications
#endif

/// Manages push notification registration and device token handling.
///
/// This is the **client-side only** component. Actual APNs push delivery
/// requires a server-side push relay that sends payloads to Apple's push
/// service — that component is out of scope for this implementation.
///
/// Usage:
/// ```swift
/// let pushManager = PushManager()
/// await pushManager.requestAuthorization()
/// // Token is automatically stored in UserDefaults for later retrieval.
/// ```
@Observable
@MainActor
public final class PushManager {
    /// The raw device token Data, hex-encoded for server transmission.
    public private(set) var deviceTokenHex: String?

    /// Whether push notifications are authorized.
    public private(set) var isAuthorized = false

    /// Whether push is enabled by the user (persisted).
    public var isEnabled: Bool {
        get { WidgetShared.isPushEnabled }
        set {
            WidgetShared.isPushEnabled = newValue
            if newValue {
                Task { await requestAuthorization() }
            }
        }
    }

    private let tokenKey = "com.symvibe.pushDeviceToken"

    public init() {
        self.deviceTokenHex = UserDefaults.standard.string(forKey: tokenKey)
        checkAuthorizationStatus()
    }

    // MARK: - Authorization

    /// Request push notification authorization from the user.
    public func requestAuthorization() async {
        #if canImport(UserNotifications)
        do {
            let granted = try await UNUserNotificationCenter.current()
                .requestAuthorization(options: [.alert, .badge, .sound])
            isAuthorized = granted
            if granted {
                await registerForRemoteNotifications()
            }
        } catch {
            isAuthorized = false
        }
        #endif
    }

    /// Check the current authorization status.
    private func checkAuthorizationStatus() {
        #if canImport(UserNotifications)
        Task {
            let settings = await UNUserNotificationCenter.current().notificationSettings()
            self.isAuthorized = settings.authorizationStatus == .authorized
        }
        #endif
    }

    // MARK: - Registration

    /// Register for remote notifications with the platform.
    private func registerForRemoteNotifications() async {
        #if os(iOS)
        await MainActor.run {
            UIApplication.shared.registerForRemoteNotifications()
        }
        #elseif os(macOS)
        // macOS does not support registerForRemoteNotifications().
        // APNs on macOS requires explicit token retrieval via
        // NSApplication.registerForRemoteNotifications() which is
        // available but rarely used. For now, skip automatic registration.
        #endif
    }

    // MARK: - Token Handling

    /// Called by the app delegate / scene delegate when a device token
    /// is received from APNs.
    public func didRegisterForRemoteNotifications(withToken tokenData: Data) {
        let hex = tokenData.map { String(format: "%02x", $0) }.joined()
        deviceTokenHex = hex
        UserDefaults.standard.set(hex, forKey: tokenKey)
    }

    /// Called when remote notification registration fails.
    public func didFailToRegisterWithError(_ error: Error) {
        deviceTokenHex = nil
    }

    /// Clear the stored device token.
    public func clearToken() {
        deviceTokenHex = nil
        UserDefaults.standard.removeObject(forKey: tokenKey)
    }
}
