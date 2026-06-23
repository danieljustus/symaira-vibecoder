package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/danieljustus/symaira-vibecoder/internal/version"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the symvibe version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("symvibe %s (%s, %s)\n", version.Version, version.GoVersion(), version.Platform())
		},
	}
}
