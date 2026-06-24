import SwiftUI
import SymvibeKit

struct ContentView: View {
    @State private var connectionStore = ConnectionStore()
    @State private var boardStore: BoardStore?

    var body: some View {
        Group {
            if let boardStore {
                MainTabView(store: boardStore, connectionStore: connectionStore)
            } else {
                OnboardingView(store: connectionStore)
            }
        }
        .task(id: connectionStore.activeProfileID) {
            if connectionStore.activeProfileID != nil {
                let store = BoardStore(connectionStore: connectionStore)
                boardStore = store
                await store.connect()
            } else {
                boardStore?.disconnect()
                boardStore = nil
            }
        }
    }
}

// MARK: - Main Tab View

struct MainTabView: View {
    let store: BoardStore
    let connectionStore: ConnectionStore

    var body: some View {
        TabView {
            NavigationStack {
                BoardView(store: store)
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
