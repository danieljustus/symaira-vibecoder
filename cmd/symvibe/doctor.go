package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

func doctorCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check opencode/git/gh availability and config sanity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load("")
			if err != nil {
				return err
			}
			run := runner.NewOpenCodeRunner(cfg.Runner.OpencodeBin, 5*time.Second)
			ocOK, info := run.Available(context.Background())
			git := onPath("git")
			gh := onPath("gh")

			// Cross-check configured model ids against what opencode exposes.
			discovered, _ := config.DiscoverModels(cfg.Runner.OpencodeBin)
			have := map[string]bool{}
			for _, m := range discovered {
				have[m.ID] = true
			}
			var missing []string
			if len(discovered) > 0 {
				for _, id := range configuredModelIDs(cfg) {
					if !have[id] {
						missing = append(missing, id)
					}
				}
				sort.Strings(missing)
			}

			if asJSON {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"opencode": info, "opencode_ok": ocOK, "git": git, "gh": gh,
					"runnable": ocOK, "missing_models": missing,
				})
			}

			fmt.Println("symvibe doctor")
			fmt.Printf("  opencode : %s\n", status(ocOK, info.Version, info.Path, info.Detail))
			fmt.Printf("  git      : %s\n", status(git, "", "", "not found on PATH"))
			fmt.Printf("  gh       : %s\n", status(gh, "", "", "not found on PATH (GitHub sensors degrade)"))
			if !ocOK {
				fmt.Println("\n  ⚠ opencode is required to RUN steps. The board still works read-only.")
			}
			if len(missing) > 0 {
				fmt.Printf("\n  ⚠ %d configured model id(s) not found in `opencode models`:\n", len(missing))
				for _, m := range missing {
					fmt.Printf("      - %s\n", m)
				}
				fmt.Println("    Edit ~/.config/symvibe/config.toml or your opencode providers.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable output")
	return cmd
}

func configuredModelIDs(cfg *config.Config) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, m := range cfg.Models {
		add(m.ID)
		for _, f := range m.FallbackModels {
			add(f)
		}
	}
	return out
}

func status(ok bool, version, path, missDetail string) string {
	if !ok {
		return "✕ " + missDetail
	}
	s := "✓"
	if version != "" {
		s += " v" + version
	}
	if path != "" {
		s += "  (" + path + ")"
	}
	return s
}

func onPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}
