package cmd

import (
	"fmt"
	"os"

	"github.com/strand-protocol/strand/strandctl/pkg/api"
	"github.com/strand-protocol/strand/strandctl/pkg/config"
	"github.com/strand-protocol/strand/strandctl/pkg/output"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	cfgFile      string
	outputFormat string
	serverURL    string
	dryRun       bool // --dry-run: print actions without executing them
	yesFlag      bool // --yes: skip confirmation prompts for destructive operations

	// Shared state set during PersistentPreRun
	cfg       *config.Config
	client    api.APIClient
	formatter output.Formatter
)

// rootCmd is the base command for strandctl.
var rootCmd = &cobra.Command{
	Use:   "strandctl",
	Short: "Strand Protocol CLI — manage nodes, routes, trust, firmware, and diagnostics",
	Long: `StrandCtl is the operator-facing CLI tool for the Strand Protocol stack.
It provides a unified interface for deploying, configuring, monitoring,
diagnosing, and managing all Strand components across a fleet of switches,
NICs, and servers.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		path := cfgFile
		if path == "" {
			path = config.DefaultPath()
		}
		var err error
		cfg, err = config.Load(path)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Override config with flags
		if serverURL != "" {
			cfg.ServerURL = serverURL
		}
		if outputFormat != "" {
			cfg.OutputFormat = outputFormat
		}

		// Create API client (mock for now)
		client = &api.MockClient{}

		// Create output formatter
		formatter = output.NewFormatter(cfg.OutputFormat)

		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// SetClient allows tests to inject a mock client.
func SetClient(c api.APIClient) {
	client = c
}

// SetFormatter allows tests to inject a formatter.
func SetFormatter(f output.Formatter) {
	formatter = f
}

// RootCmd returns the root cobra.Command for testing purposes.
func RootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.strand/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "output format: table, json, yaml (default \"table\")")
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "Strand API server URL")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "print actions that would be taken without executing them")
	rootCmd.PersistentFlags().BoolVar(&yesFlag, "yes", false, "skip confirmation prompts for destructive operations")
}
