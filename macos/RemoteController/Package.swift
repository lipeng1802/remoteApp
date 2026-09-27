// swift-tools-version: 5.8

import PackageDescription

let package = Package(
    name: "RemoteController",
    platforms: [.macOS(.v13)],
    products: [
        .library(name: "RemoteProtocol", targets: ["RemoteProtocol"]),
        .executable(name: "RemoteController", targets: ["RemoteController"])
    ],
    targets: [
        .target(name: "RemoteProtocol"),
        .executableTarget(
            name: "RemoteController",
            dependencies: ["RemoteProtocol"]
        ),
        .testTarget(
            name: "RemoteProtocolTests",
            dependencies: ["RemoteProtocol"]
        )
    ]
)
