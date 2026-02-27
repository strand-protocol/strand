package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const nexctlVersion = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show nexctl and Nexus API versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "nexctl version %s\n", nexctlVersion)

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
