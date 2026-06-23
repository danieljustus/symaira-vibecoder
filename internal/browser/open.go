// Package browser opens a URL in the user's default browser. Best-effort: a
// failure is never fatal (symvibe prints the URL regardless).
package browser

import (
	"os/exec"
	"runtime"
)

// Open launches url in the default browser. Returns an error only for logging;
// callers ignore it.
func Open(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
		args = []string{url}
	case "windows":
		name = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // linux, *bsd
		name = "xdg-open"
		args = []string{url}
	}
	return exec.Command(name, args...).Start()
}
