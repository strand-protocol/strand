package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/strand-protocol/strand/strandctl/pkg/api"
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage Strand nodes",
	Long:  "List, inspect, and manage nodes in the Strand network.",
}

var nodeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all known nodes in the network",
	RunE: func(cmd *cobra.Command, args []string) error {
		nodes, err := client.ListNodes()
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(nodes))
		return nil
	},
}

var nodeDescribeCmd = &cobra.Command{
	Use:   "describe <node-id>",
	Short: "Show detailed info for a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := api.ValidateID(args[0]); err != nil {
			return fmt.Errorf("invalid node-id: %w", err)
		}
		node, err := client.DescribeNode(args[0])
		if err != nil {
			return fmt.Errorf("failed to describe node: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(node))
		return nil
	},
}

var nodeDrainCmd = &cobra.Command{
	Use:   "drain <node-id>",
	Short: "Drain a node (gracefully remove from rotation)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := api.ValidateID(args[0]); err != nil {
			return fmt.Errorf("invalid node-id: %w", err)
		}
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "(dry-run) would drain node %q\n", args[0])
			return nil
		}
		if !yesFlag {
			fmt.Fprintf(cmd.OutOrStdout(), "Drain node %q? This removes it from active rotation. [y/N]: ", args[0])
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}
		if err := client.DrainNode(args[0]); err != nil {
			return fmt.Errorf("failed to drain node: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Node %q drained successfully.\n", args[0])
		return nil
	},
}

func init() {
	nodeCmd.AddCommand(nodeListCmd)
	nodeCmd.AddCommand(nodeDescribeCmd)
	nodeCmd.AddCommand(nodeDrainCmd)
	rootCmd.AddCommand(nodeCmd)
}
