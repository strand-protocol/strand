package tui

import "strings"

// RouteRow is a single row in the Routes dashboard table.
type RouteRow struct {
	Destination string // SAD or CIDR destination string
	NextHop     string // Next-hop node ID or address
	Capability  string // Primary SAD capability selector (e.g. "inference")
	Score       string // Resolution score, e.g. "0.92"
}

// renderRoutes renders the Routes tab content as a lipgloss-styled table.
func renderRoutes(routes []RouteRow, width int) string {
	if len(routes) == 0 {
		return dimStyle.Render("  No routes found.")
	}

	colDest := colWidth(width, 0.30)
	colHop := colWidth(width, 0.28)
	colCap := colWidth(width, 0.22)
	colScore := colWidth(width, 0.10)

	header := strings.Join([]string{
		headerCellStyle.Width(colDest).Render("DESTINATION"),
		headerCellStyle.Width(colHop).Render("NEXT HOP"),
		headerCellStyle.Width(colCap).Render("CAPABILITY"),
		headerCellStyle.Width(colScore).Render("SCORE"),
	}, "")

	var rows []string
	rows = append(rows, header)
	for i, r := range routes {
		style := rowStyle
		if i%2 == 0 {
			style = altRowStyle
		}
		row := strings.Join([]string{
			style.Width(colDest).Render(truncate(r.Destination, colDest-1)),
			style.Width(colHop).Render(truncate(r.NextHop, colHop-1)),
			style.Width(colCap).Render(truncate(r.Capability, colCap-1)),
			style.Width(colScore).Render(truncate(r.Score, colScore-1)),
		}, "")
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}
