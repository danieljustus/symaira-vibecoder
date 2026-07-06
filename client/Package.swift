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
    dependencies: [
        .package(url: "https://github.com/danieljustus/symaira-appkit.git", exact: "0.2.0"),
    ],
    targets: [
        .target(
            name: "SymvibeKit",
            dependencies: [
                .product(name: "SymairaKeychain", package: "symaira-appkit"),
            ]
        ),
        .testTarget(
            name: "SymvibeKitTests",
            dependencies: ["SymvibeKit"]
        ),
    ]
)
