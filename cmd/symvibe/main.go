// Command symvibe is a graphical "Vibe Coding" Baukasten: it serves an editable
// cycle board on localhost and drives opencode (via a swappable Runner) to walk
// the cycle autonomously, streaming live status to the board.
//
// Subcommands:
//
//	symvibe serve     start the board on 127.0.0.1 and open it
//	symvibe doctor    check opencode/git/gh availability and config sanity
//	symvibe version   print the version
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/danieljustus/symaira-vibecoder/internal/version"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	root := &cobra.Command{
		Use:           "symvibe",
		Short:         "Graphical Vibe-Coding Baukasten that drives opencode",
		Long:          "symvibe serves an editable cycle board (Baukasten) on localhost and drives opencode\nto run the phases/steps autonomously, with per-step model bindings and live status.",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(serveCmd(), doctorCmd(), versionCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
