package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/danieljustus/symaira-corekit/versionkit"
	"github.com/danieljustus/symaira-vibecoder/internal/version"
)

func versionCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the symvibe version",
		Run: func(cmd *cobra.Command, args []string) {
			info := versionkit.New("symvibe", version.Version, 1)
			if jsonOut {
				info.Write(os.Stdout)
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), info.String())
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output version as JSON for the GUI handshake")
	return cmd
}
