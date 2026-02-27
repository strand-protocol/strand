package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Network diagnostics",
	Long:  "Run diagnostic commands: ping, traceroute, and benchmark against Nexus nodes.",
}

var diagnosePingCmd = &cobra.Command{
	Use:   "ping <target>",
	Short: "NexStream-level ping to measure latency",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := client.Ping(args[0])
		if err != nil {
			return fmt.Errorf("ping failed: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(result))
		return nil
	},
}

var diagnoseTracerouteCmd = &cobra.Command{
	Use:   "traceroute <target>",
	Short: "Trace the NexRoute path to a target node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Traceroute reuses Ping for now, showing hop count
		result, err := client.Ping(args[0])
		if err != nil {
			return fmt.Errorf("traceroute failed: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Traceroute to %s: %d hops, latency %.2fms, status: %s\n",
			result.Target, result.Hops, result.Latency, result.Status)
		return nil
	},
}

var diagnoseBenchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run a throughput benchmark against the network",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "Running benchmark...")
		fmt.Fprintln(cmd.OutOrStdout(), "Throughput: 9.8 Gbps (mock)")
		fmt.Fprintln(cmd.OutOrStdout(), "Latency p50: 0.42ms, p99: 1.87ms (mock)")
		fmt.Fprintln(cmd.OutOrStdout(), "Benchmark complete.")
		return nil
	},
}

func init() {
	diagnoseCmd.AddCommand(diagnosePingCmd)
	diagnoseCmd.AddCommand(diagnoseTracerouteCmd)
	diagnoseCmd.AddCommand(diagnoseBenchmarkCmd)
	rootCmd.AddCommand(diagnoseCmd)
}
