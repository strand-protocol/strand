package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StreamRow is a single row in the Streams dashboard table.
type StreamRow struct {
	ID     string // 32-bit stream ID displayed as hex, e.g. "0x00000001"
	Mode   string // Delivery mode: "RO", "RU", "BE", or "PR"
	RTT    string // Smoothed RTT, e.g. "4.2ms"
	Bytes  string // Total bytes transferred, e.g. "1.2 MB"
	Status string // "established", "closing", "closed", etc.
}

// streamModeColor returns a foreground colour representing the delivery mode.
func streamModeColor(mode string) lipgloss.Color {
	switch strings.ToUpper(mode) {
	case "RO":
		return lipgloss.Color("4") // blue — reliable-ordered (most common)
	case "RU":
		return lipgloss.Color("6") // cyan — reliable-unordered
	case "BE":
		return lipgloss.Color("3") // yellow — best-effort
	case "PR":
		return lipgloss.Color("5") // magenta — probabilistic
	default:
		return lipgloss.Color("8") // grey
	}
}

// streamStatusColor returns a foreground colour representing the stream status.
func streamStatusColor(status string) lipgloss.Color {
	switch strings.ToLower(status) {
	case "established":
		return lipgloss.Color("2") // green
	case "closing":
		return lipgloss.Color("3") // yellow
	case "closed":
		return lipgloss.Color("8") // grey
	default:
		return lipgloss.Color("1") // red
	}
}

// renderStreams renders the Streams tab content as a lipgloss-styled table.
func renderStreams(streams []StreamRow, width int) string {
	if len(streams) == 0 {
		return dimStyle.Render("  No active streams.")
	}

	colID := colWidth(width, 0.14)
	colMode := colWidth(width, 0.08)
	colRTT := colWidth(width, 0.12)
	colBytes := colWidth(width, 0.16)
	colStatus := colWidth(width, 0.16)

	header := strings.Join([]string{
		headerCellStyle.Width(colID).Render("STREAM ID"),
		headerCellStyle.Width(colMode).Render("MODE"),
		headerCellStyle.Width(colRTT).Render("RTT"),
		headerCellStyle.Width(colBytes).Render("BYTES"),
		headerCellStyle.Width(colStatus).Render("STATUS"),
	}, "")

	var rows []string
	rows = append(rows, header)
	for i, s := range streams {
		style := rowStyle
		if i%2 == 0 {
			style = altRowStyle
		}
		modeCell := lipgloss.NewStyle().
			Width(colMode).
			Foreground(streamModeColor(s.Mode)).
			Render(truncate(s.Mode, colMode-1))
		statusCell := lipgloss.NewStyle().
			Width(colStatus).
			Foreground(streamStatusColor(s.Status)).
			Render(truncate(s.Status, colStatus-1))

		row := strings.Join([]string{
			style.Width(colID).Render(truncate(s.ID, colID-1)),
			modeCell,
			style.Width(colRTT).Render(truncate(s.RTT, colRTT-1)),
			style.Width(colBytes).Render(truncate(s.Bytes, colBytes-1)),
			statusCell,
		}, "")
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}
