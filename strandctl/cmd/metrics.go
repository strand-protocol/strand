package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "View and export Nexus metrics",
	Long:  "Show metrics for nodes and export in Prometheus or JSON format.",
}

var metricsShowNodeFlag string

var metricsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show metrics for a node",
	RunE: func(cmd *cobra.Command, args []string) error {
		if metricsShowNodeFlag == "" {
			return fmt.Errorf("--node flag is required")
		}
		metrics, err := client.GetMetrics(metricsShowNodeFlag)
		if err != nil {
			return fmt.Errorf("failed to get metrics: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(metrics))
		return nil
	},
}

var metricsExportFormatFlag string

var metricsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export metrics in Prometheus or JSON format",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Gather metrics from all nodes
		nodes, err := client.ListNodes()
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}

		switch metricsExportFormatFlag {
		case "prometheus":
			for _, n := range nodes {
				m, err := client.GetMetrics(n.ID)
				if err != nil {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "# HELP nexus_connections Number of active connections\n")
				fmt.Fprintf(cmd.OutOrStdout(), "# TYPE nexus_connections gauge\n")
				fmt.Fprintf(cmd.OutOrStdout(), "nexus_connections{node_id=%q} %d\n", m.NodeID, m.Connections)
				fmt.Fprintf(cmd.OutOrStdout(), "# HELP nexus_bytes_sent Total bytes sent\n")
				fmt.Fprintf(cmd.OutOrStdout(), "# TYPE nexus_bytes_sent counter\n")
				fmt.Fprintf(cmd.OutOrStdout(), "nexus_bytes_sent{node_id=%q} %d\n", m.NodeID, m.BytesSent)
				fmt.Fprintf(cmd.OutOrStdout(), "# HELP nexus_bytes_recv Total bytes received\n")
				fmt.Fprintf(cmd.OutOrStdout(), "# TYPE nexus_bytes_recv counter\n")
				fmt.Fprintf(cmd.OutOrStdout(), "nexus_bytes_recv{node_id=%q} %d\n", m.NodeID, m.BytesRecv)
				fmt.Fprintf(cmd.OutOrStdout(), "# HELP nexus_latency_ms Latency in milliseconds\n")
				fmt.Fprintf(cmd.OutOrStdout(), "# TYPE nexus_latency_ms gauge\n")
				fmt.Fprintf(cmd.OutOrStdout(), "nexus_latency_ms{node_id=%q} %.2f\n", m.NodeID, m.Latency)
			}
		case "json":
			var allMetrics []any
			for _, n := range nodes {
				m, err := client.GetMetrics(n.ID)
				if err != nil {
					continue
				}
				allMetrics = append(allMetrics, m)
			}
			data, err := json.MarshalIndent(allMetrics, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal metrics: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
		default:
			return fmt.Errorf("unsupported export format %q (use prometheus or json)", metricsExportFormatFlag)
		}
		return nil
	},
}

func init() {
	metricsShowCmd.Flags().StringVar(&metricsShowNodeFlag, "node", "", "node ID to show metrics for (required)")
	metricsExportCmd.Flags().StringVar(&metricsExportFormatFlag, "format", "prometheus", "export format: prometheus, json")

	metricsCmd.AddCommand(metricsShowCmd)
	metricsCmd.AddCommand(metricsExportCmd)
	rootCmd.AddCommand(metricsCmd)
}
