package cmd

import (
	"fmt"
	"os"

	"github.com/strand-protocol/strand/strandctl/pkg/api"
	"github.com/spf13/cobra"
)

var trustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Manage StrandTrust certificates and CAs",
	Long:  "Issue, verify, and manage Model Identity Certificates (MICs) and Certificate Authorities.",
}

var trustIssueMICNodeFlag string

var trustIssueMICCmd = &cobra.Command{
	Use:   "issue-mic",
	Short: "Issue a new Model Identity Certificate",
	RunE: func(cmd *cobra.Command, args []string) error {
		if trustIssueMICNodeFlag == "" {
			return fmt.Errorf("--node flag is required")
		}
		if err := api.ValidateID(trustIssueMICNodeFlag); err != nil {
			return fmt.Errorf("invalid --node value: %w", err)
		}
		mic, err := client.IssueMIC(trustIssueMICNodeFlag)
		if err != nil {
			return fmt.Errorf("failed to issue MIC: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(mic))
		return nil
	},
}

var trustVerifyCmd = &cobra.Command{
	Use:   "verify <mic-file>",
	Short: "Verify a MIC file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cleanPath, err := api.ValidateFilePath(args[0])
		if err != nil {
			return fmt.Errorf("invalid file path: %w", err)
		}
		data, err := os.ReadFile(cleanPath)
		if err != nil {
			return fmt.Errorf("failed to read MIC file: %w", err)
		}
		mic, err := client.VerifyMIC(data)
		if err != nil {
			return fmt.Errorf("MIC verification failed: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(mic))
		return nil
	},
}

var trustListCAsCmd = &cobra.Command{
	Use:   "list-cas",
	Short: "List all Certificate Authorities",
	RunE: func(cmd *cobra.Command, args []string) error {
		cas, err := client.ListCAs()
		if err != nil {
			return fmt.Errorf("failed to list CAs: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(cas))
		return nil
	},
}

func init() {
	trustIssueMICCmd.Flags().StringVar(&trustIssueMICNodeFlag, "node", "", "node ID to issue MIC for (required)")

	trustCmd.AddCommand(trustIssueMICCmd)
	trustCmd.AddCommand(trustVerifyCmd)
	trustCmd.AddCommand(trustListCAsCmd)
	rootCmd.AddCommand(trustCmd)
}
