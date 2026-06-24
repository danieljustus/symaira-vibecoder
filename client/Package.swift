// swift-tools-version:6.0
import PackageDescription

let package = Package(
    name: "SymvibeClient",
    platforms: [
        .iOS(.v17),
        .macOS(.v14),
    ],
    products: [
        .library(name: "SymvibeKit", targets: ["SymvibeKit"]),
    ],
    targets: [
        .target(name: "SymvibeKit"),
        .testTarget(
            name: "SymvibeKitTests",
            dependencies: ["SymvibeKit"]
        ),
    ]
)
