package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X github.com/strand-protocol/strand/strandctl/cmd.strandctlVersion=x.y.z"
var strandctlVersion = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show strandctl and Strand API versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "strandctl version %s\n", strandctlVersion)

		apiVersion, err := client.Version()
		if err != nil {
			return fmt.Errorf("failed to get API version: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "API server: %s\n", apiVersion)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
