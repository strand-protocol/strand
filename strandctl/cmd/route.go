package cmd

import (
	"fmt"
	"strings"

	"github.com/nexus-protocol/nexus/nexctl/pkg/api"
	"github.com/spf13/cobra"
)

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Manage NexRoute SAD routing entries",
	Long:  "List, inspect, and add routes in the Nexus routing table.",
}

var routeListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show the routing table",
	RunE: func(cmd *cobra.Command, args []string) error {
		routes, err := client.ListRoutes()
		if err != nil {
			return fmt.Errorf("failed to list routes: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(routes))
		return nil
	},
}

var routeDescribeCmd = &cobra.Command{
	Use:   "describe <sad>",
	Short: "Show detailed info for a route",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		route, err := client.DescribeRoute(args[0])
		if err != nil {
			return fmt.Errorf("failed to describe route: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(route))
		return nil
	},
}

var (
	routeAddModelType    string
	routeAddContextWindow string
	routeAddLatencySLA   string
)

var routeAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new route entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build SAD from flags
		parts := []string{"sad"}
		if routeAddModelType != "" {
			parts = append(parts, routeAddModelType)
		}
		if routeAddContextWindow != "" {
			parts = append(parts, routeAddContextWindow)
		}
		if routeAddLatencySLA != "" {
			parts = append(parts, routeAddLatencySLA)
		}
		sad := strings.Join(parts, ":")

		route := api.RouteInfo{
			SAD:       sad,
			Endpoints: []string{},
			Weight:    100,
			TTL:       300,
		}
		if err := client.AddRoute(route); err != nil {
			return fmt.Errorf("failed to add route: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Route %q added successfully.\n", sad)
		return nil
	},
}

func init() {
	routeAddCmd.Flags().StringVar(&routeAddModelType, "model-type", "", "model type for the route")
	routeAddCmd.Flags().StringVar(&routeAddContextWindow, "context-window", "", "context window size")
	routeAddCmd.Flags().StringVar(&routeAddLatencySLA, "latency-sla", "", "latency SLA target")

	routeCmd.AddCommand(routeListCmd)
	routeCmd.AddCommand(routeDescribeCmd)
	routeCmd.AddCommand(routeAddCmd)
	rootCmd.AddCommand(routeCmd)
}
