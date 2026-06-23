// Package version holds the build-time version string.
package version

import "runtime"

// Version is overridden at build time via
//
//	-ldflags "-X github.com/danieljustus/symaira-vibecoder/internal/version.Version=v1.2.3"
//
// and defaults to "dev" for `go run` / source builds.
var Version = "dev"

// GoVersion returns the Go toolchain that built the binary.
func GoVersion() string { return runtime.Version() }

// Platform returns the os/arch the binary was built for.
func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }
