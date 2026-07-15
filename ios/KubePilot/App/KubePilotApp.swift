import SwiftUI
import SwiftData

@main
struct KubePilotApp: App {
    @State private var appState = AppState()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(appState)
                .modelContainer(PersistenceController.shared.container)
                .preferredColorScheme(.dark)
                .tint(Theme.accent)
        }
    }
}
