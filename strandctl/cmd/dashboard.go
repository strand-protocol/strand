package cmd

import (
	"github.com/strand-protocol/strand/strandctl/pkg/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// dashboardCmd launches the interactive TUI dashboard.
var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Launch the interactive TUI dashboard",
	Long: `Launch an interactive terminal dashboard that displays live data
about nodes, routes, and streams in the Strand network. Data is
refreshed every 2 seconds from the Strand Cloud API server.

Key bindings:
  Tab / Shift+Tab  Navigate between tabs
  1 / 2 / 3        Jump directly to Nodes / Routes / Streams
  r                Force an immediate data refresh
  q / Ctrl+C       Quit`,
	// Override PersistentPreRunE so the dashboard does not need a config file.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := cmd.Flags().GetString("server")
		if err != nil {
			server = "http://localhost:8080"
		}
		// Fall back to the global --server flag if the local one was not set.
		if server == "" && serverURL != "" {
			server = serverURL
		}
		if server == "" {
			server = "http://localhost:8080"
		}

		p := tea.NewProgram(tui.New(server), tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	dashboardCmd.Flags().String("server", "", "Strand Cloud server URL (default: http://localhost:8080)")
	rootCmd.AddCommand(dashboardCmd)
}
