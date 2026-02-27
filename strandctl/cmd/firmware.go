package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var firmwareCmd = &cobra.Command{
	Use:   "firmware",
	Short: "Manage StrandLink firmware",
	Long:  "List available firmware, flash devices, and check firmware status.",
}

var firmwareListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available firmware images",
	RunE: func(cmd *cobra.Command, args []string) error {
		fws, err := client.ListFirmware()
		if err != nil {
			return fmt.Errorf("failed to list firmware: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(fws))
		return nil
	},
}

var firmwareFlashCmd = &cobra.Command{
	Use:   "flash <device> <image>",
	Short: "Flash firmware to a device",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		device := args[0]
		image := args[1]
		if err := client.FlashFirmware(device, image); err != nil {
			return fmt.Errorf("firmware flash failed: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Firmware %q flashed to device %q successfully.\n", image, device)
		return nil
	},
}

var firmwareStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show firmware status for all nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		nodes, err := client.ListNodes()
		if err != nil {
			return fmt.Errorf("failed to get node info: %w", err)
		}
		type fwStatus struct {
			NodeID   string `json:"node_id" yaml:"node_id"`
			Firmware string `json:"firmware" yaml:"firmware"`
			Status   string `json:"status" yaml:"status"`
		}
		statuses := make([]fwStatus, 0, len(nodes))
		for _, n := range nodes {
			statuses = append(statuses, fwStatus{
				NodeID:   n.ID,
				Firmware: n.Firmware,
				Status:   n.Status,
			})
		}
		fmt.Fprint(cmd.OutOrStdout(), formatter.Format(statuses))
		return nil
	},
}

func init() {
	firmwareCmd.AddCommand(firmwareListCmd)
	firmwareCmd.AddCommand(firmwareFlashCmd)
	firmwareCmd.AddCommand(firmwareStatusCmd)
	rootCmd.AddCommand(firmwareCmd)
}
