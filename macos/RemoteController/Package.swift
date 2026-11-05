// swift-tools-version: 5.8

import PackageDescription

let package = Package(
    name: "RemoteController",
    platforms: [.macOS(.v13)],
    products: [
        .library(name: "RemoteProtocol", targets: ["RemoteProtocol"]),
        .executable(name: "RemoteController", targets: ["RemoteController"]),
        .executable(name: "OwnedDesktopController", targets: ["OwnedDesktopController"]),
        .executable(name: "TLSProbeClient", targets: ["TLSProbeClient"]),
        .executable(name: "TLSBridgeProbe", targets: ["TLSBridgeProbe"]),
        .executable(name: "ConnectionIdentityVault", targets: ["ConnectionIdentityVault"]),
        .executable(name: "TailscaleInputMockClient", targets: ["TailscaleInputMockClient"]),
        .executable(name: "InputPreview", targets: ["InputPreview"])
    ],
    targets: [
        .target(name: "RemoteProtocol"),
        .target(name: "RemoteCanvas", dependencies: ["RemoteProtocol"]),
        .executableTarget(name: "OwnedDesktopController", dependencies: ["RemoteProtocol", "RemoteCanvas"]),
        .executableTarget(name: "ConnectionIdentityVault", dependencies: ["RemoteProtocol"]),
        .executableTarget(name: "TLSBridgeProbe", dependencies: ["RemoteProtocol"]),
        .executableTarget(
            name: "RemoteController",
            dependencies: ["RemoteProtocol", "RemoteCanvas"]
        ),
        .executableTarget(
            name: "TLSProbeClient",
            dependencies: ["RemoteProtocol"]
        ),
        .executableTarget(
            name: "TailscaleInputMockClient",
            dependencies: ["RemoteProtocol"]
        ),
        .executableTarget(
            name: "InputPreview",
            dependencies: ["RemoteProtocol"]
        ),
        .testTarget(
            name: "RemoteProtocolTests",
            dependencies: ["RemoteProtocol"]
        )
    ]
)
