package runner

import (
	"strings"
	"testing"
)

func TestExtractVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"1.17.9", "1.17.9"},
		{"opencode version 1.17.9", "1.17.9"},
		{"v1.17.9", "1.17.9"},
		{"1.17.9-beta", "1.17.9-beta"},
		{"no version here", ""},
	}
	for _, c := range cases {
		got := extractVersion(c.in)
		if got != c.want {
			t.Errorf("extractVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCheckOpencodeVersion(t *testing.T) {
	cases := []struct {
		ver     string
		wantOK  bool
		wantSub string
	}{
		{"1.17.9", true, "version 1.17.9"},
		{"1.17.0", true, "version 1.17.0"},
		{"1.16.9", false, "older than required"},
		{"1.2.3", false, "older than required"},
		{"garbage", false, "could not parse"},
	}
	for _, c := range cases {
		ok, detail := CheckOpencodeVersion(c.ver)
		if ok != c.wantOK {
			t.Errorf("CheckOpencodeVersion(%q) ok=%v, want %v", c.ver, ok, c.wantOK)
		}
		if !strings.Contains(detail, c.wantSub) {
			t.Errorf("CheckOpencodeVersion(%q) detail=%q, want substring %q", c.ver, detail, c.wantSub)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.17.9", "1.17.9", 0},
		{"1.17.9", "1.17.10", -1},
		{"1.18.0", "1.17.9", 1},
		{"1.17.9", "1.17.0", 1},
		{"1.17", "1.17.0", 0},
	}
	for _, c := range cases {
		got := compareVersions(c.a, c.b)
		if got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
