package version

import (
	"runtime"
	"testing"
)

func TestAPIVersionConstant(t *testing.T) {
	if APIVersion != "v1" {
		t.Fatalf("APIVersion = %q, want %q", APIVersion, "v1")
	}
}

func TestVersionDefaultNotEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}

func TestGoVersion(t *testing.T) {
	got := GoVersion()
	want := runtime.Version()
	if got != want {
		t.Fatalf("GoVersion() = %q, want %q", got, want)
	}
}

func TestPlatform(t *testing.T) {
	got := Platform()
	want := runtime.GOOS + "/" + runtime.GOARCH
	if got != want {
		t.Fatalf("Platform() = %q, want %q", got, want)
	}
}
