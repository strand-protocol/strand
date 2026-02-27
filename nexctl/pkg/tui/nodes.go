package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// NodeRow is a single row in the Nodes dashboard table.
type NodeRow struct {
	ID      string // Short node identifier shown in the table
	Status  string // "active", "draining", "offline", etc.
	Latency string // Round-trip latency string, e.g. "1.2ms"
	Region  string // Geographic or logical region label
}

// nodeStatusColor returns a lipgloss foreground colour for a node status string.
func nodeStatusColor(status string) lipgloss.Color {
	switch strings.ToLower(status) {
	case "active":
		return lipgloss.Color("2") // green
	case "draining":
		return lipgloss.Color("3") // yellow
	case "offline":
		return lipgloss.Color("1") // red
	default:
		return lipgloss.Color("8") // grey
	}
}

// renderNodes renders the Nodes tab content as a lipgloss-styled table and
// returns it as a string. width constrains the overall column layout.
func renderNodes(nodes []NodeRow, width int) string {
	if len(nodes) == 0 {
		return dimStyle.Render("  No nodes found.")
	}

	// Column widths — scale with terminal width but cap sensibly.
	colID := colWidth(width, 0.28)
	colStatus := colWidth(width, 0.14)
	colLatency := colWidth(width, 0.14)
	colRegion := colWidth(width, 0.30)

	// Header
	header := strings.Join([]string{
		headerCellStyle.Width(colID).Render("NODE ID"),
		headerCellStyle.Width(colStatus).Render("STATUS"),
		headerCellStyle.Width(colLatency).Render("LATENCY"),
		headerCellStyle.Width(colRegion).Render("REGION"),
	}, "")

	// Rows
	var rows []string
	rows = append(rows, header)
	for i, n := range nodes {
		style := rowStyle
		if i%2 == 0 {
			style = altRowStyle
		}
		statusCell := lipgloss.NewStyle().
			Width(colStatus).
			Foreground(nodeStatusColor(n.Status)).
			Render(truncate(n.Status, colStatus-1))

		row := strings.Join([]string{
			style.Width(colID).Render(truncate(n.ID, colID-1)),
			statusCell,
			style.Width(colLatency).Render(truncate(n.Latency, colLatency-1)),
			style.Width(colRegion).Render(truncate(n.Region, colRegion-1)),
		}, "")
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

// colWidth converts a fractional width into an integer column width, leaving a
// small gutter between columns.
func colWidth(totalWidth int, fraction float64) int {
	w := int(float64(totalWidth) * fraction)
	if w < 8 {
		w = 8
	}
	return w
}

// truncate shortens s to maxLen runes, appending "…" if truncation occurred.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return fmt.Sprintf("%s…", string(runes[:maxLen-1]))
}
