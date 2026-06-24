import SwiftUI
import SymvibeKit

struct ContentView: View {
    @State private var connectionStore = ConnectionStore()
    @State private var boardStore: BoardStore?
    @State private var activityStore = ActivityStore()

    var body: some View {
        Group {
            if let boardStore {
                MainTabView(store: boardStore, activityStore: activityStore, connectionStore: connectionStore)
            } else {
                OnboardingView(store: connectionStore)
            }
        }
        .task(id: connectionStore.activeProfileID) {
            if connectionStore.activeProfileID != nil {
                let store = BoardStore(connectionStore: connectionStore)
                store.activityStore = activityStore
                boardStore = store
                await store.connect()
            } else {
                boardStore?.disconnect()
                boardStore = nil
                activityStore.clear()
            }
        }
    }
}

// MARK: - Main Tab View

struct MainTabView: View {
    let store: BoardStore
    let activityStore: ActivityStore
    let connectionStore: ConnectionStore

    var body: some View {
        TabView {
            NavigationStack {
                BoardView(store: store, activityStore: activityStore)
            }
            .tabItem {
                Label("Board", systemImage: "rectangle.stack")
            }

            NavigationStack {
                DoctorView(apiClient: store.client)
            }
            .tabItem {
                Label("Doctor", systemImage: "stethoscope")
            }
        }
        #if os(macOS)
        .frame(minWidth: 700, minHeight: 500)
        #endif
    }
}
