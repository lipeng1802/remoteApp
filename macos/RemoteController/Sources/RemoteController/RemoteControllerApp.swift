import SwiftUI

@main
struct RemoteControllerApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
        }
        .windowResizability(.contentSize)
    }
}

private struct ContentView: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Personal Remote Desktop")
                .font(.title2.bold())
            Text("Protocol baseline ready")
                .foregroundStyle(.secondary)
            Text("Networking and remote control are intentionally disabled in P0.")
                .font(.callout)
        }
        .padding(24)
        .frame(minWidth: 440)
    }
}
