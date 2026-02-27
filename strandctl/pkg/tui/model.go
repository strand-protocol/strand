// Package tui provides the interactive terminal dashboard for strandctl.
// It is built on the bubbletea/lipgloss stack and renders three tabs:
// Nodes, Routes, and Streams. Data is refreshed every 2 seconds by calling
// the Strand Cloud REST API.
package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Shared styles
// ---------------------------------------------------------------------------

var (
	// titleStyle renders the application title bar.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("57")).
			Padding(0, 1)

	// activeTabStyle renders the currently selected tab label.
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("57")).
			Padding(0, 2)

	// inactiveTabStyle renders unselected tab labels.
	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Padding(0, 2)

	// headerCellStyle is used for table column headers.
	headerCellStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			PaddingRight(1)

	// rowStyle is used for odd-numbered table rows.
	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			PaddingRight(1)

	// altRowStyle is used for even-numbered table rows (zebra striping).
	altRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("236")).
			PaddingRight(1)

	// dimStyle is used for "no data" messages.
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	// statusBarStyle renders the bottom status bar.
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(1)

	// errorStyle renders error messages.
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true).
			PaddingLeft(1)
)

// ---------------------------------------------------------------------------
// Tab type
// ---------------------------------------------------------------------------

// tab identifies the currently active dashboard tab.
type tab int

const (
	tabNodes tab = iota
	tabRoutes
	tabStreams
	tabCount // sentinel — must stay last
)

// ---------------------------------------------------------------------------
// Tea messages
// ---------------------------------------------------------------------------

// tickMsg is sent every refreshInterval to trigger a data refresh.
type tickMsg time.Time

// dataMsg carries a freshly fetched dataset.
type dataMsg struct {
	nodes   []NodeRow
	routes  []RouteRow
	streams []StreamRow
}

// errMsg carries a fetch or decode error to display in the status bar.
type errMsg error

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

const refreshInterval = 2 * time.Second

// Model is the top-level bubbletea model for the dashboard.
type Model struct {
	tabs      []string
	activeTab tab
	nodes     []NodeRow
	routes    []RouteRow
	streams   []StreamRow
	serverURL string
	width     int
	height    int
	err       error
	loading   bool
	lastFetch time.Time
}

// New returns a Model configured to talk to serverURL.
func New(serverURL string) Model {
	return Model{
		tabs:      []string{"Nodes", "Routes", "Streams"},
		serverURL: serverURL,
		loading:   true,
	}
}

// Init starts the periodic tick and issues the first data fetch.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tick(), fetchData(m.serverURL))
}

// tick schedules a tickMsg after refreshInterval.
func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update processes messages and returns an updated model plus any commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % tabCount
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + tabCount) % tabCount
		case "1":
			m.activeTab = tabNodes
		case "2":
			m.activeTab = tabRoutes
		case "3":
			m.activeTab = tabStreams
		case "r":
			// Manual refresh
			m.loading = true
			m.err = nil
			return m, fetchData(m.serverURL)
		}
		return m, nil

	case tickMsg:
		m.loading = true
		m.err = nil
		return m, tea.Batch(tick(), fetchData(m.serverURL))

	case dataMsg:
		m.loading = false
		m.err = nil
		m.nodes = msg.nodes
		m.routes = msg.routes
		m.streams = msg.streams
		m.lastFetch = time.Now()
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg
		return m, nil
	}

	return m, nil
}

// View renders the entire dashboard to a string.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	var sb strings.Builder

	// --- Title bar ---
	title := titleStyle.Render("  Strand Protocol Dashboard  ")
	sb.WriteString(title)
	sb.WriteString("\n")

	// --- Tab bar ---
	var tabParts []string
	for i, name := range m.tabs {
		label := fmt.Sprintf(" %d: %s ", i+1, name)
		if tab(i) == m.activeTab {
			tabParts = append(tabParts, activeTabStyle.Render(label))
		} else {
			tabParts = append(tabParts, inactiveTabStyle.Render(label))
		}
	}
	sb.WriteString(strings.Join(tabParts, ""))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", m.width))
	sb.WriteString("\n")

	// --- Content area ---
	contentHeight := m.height - 5 // title(1) + tabs(1) + divider(1) + status(2)
	if contentHeight < 1 {
		contentHeight = 1
	}
	content := m.renderActiveTab()
	// Clip content to available height by trimming lines.
	content = clipLines(content, contentHeight)
	sb.WriteString(content)
	sb.WriteString("\n")

	// --- Status bar ---
	sb.WriteString(strings.Repeat("─", m.width))
	sb.WriteString("\n")
	sb.WriteString(m.renderStatus())

	return sb.String()
}

// renderActiveTab renders the content of the currently selected tab.
func (m Model) renderActiveTab() string {
	w := m.width - 2 // leave a small margin
	switch m.activeTab {
	case tabNodes:
		return renderNodes(m.nodes, w)
	case tabRoutes:
		return renderRoutes(m.routes, w)
	case tabStreams:
		return renderStreams(m.streams, w)
	default:
		return ""
	}
}

// renderStatus renders the bottom status bar line.
func (m Model) renderStatus() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	parts := []string{
		fmt.Sprintf("server: %s", m.serverURL),
	}
	if !m.lastFetch.IsZero() {
		parts = append(parts, fmt.Sprintf("last refresh: %s", m.lastFetch.Format("15:04:05")))
	}
	if m.loading {
		parts = append(parts, "refreshing…")
	}
	parts = append(parts, "q: quit  tab: next tab  r: refresh")

	return statusBarStyle.Render(strings.Join(parts, "  |  "))
}

// clipLines limits the string s to at most maxLines newline-delimited lines.
func clipLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

// ---------------------------------------------------------------------------
// Data fetching
// ---------------------------------------------------------------------------

// fetchData issues HTTP requests to the Strand Cloud REST API and returns a
// dataMsg (or errMsg on failure). Missing endpoints (404) are handled
// gracefully by returning empty slices.
func fetchData(serverURL string) tea.Cmd {
	return func() tea.Msg {
		nodes, err := fetchNodes(serverURL)
		if err != nil {
			return errMsg(err)
		}
		routes, err := fetchRoutes(serverURL)
		if err != nil {
			return errMsg(err)
		}
		// Streams are currently not exposed via the REST API; return empty list.
		return dataMsg{nodes: nodes, routes: routes, streams: []StreamRow{}}
	}
}

// nodeAPIResponse mirrors the JSON shape returned by GET /v1/nodes.
type nodeAPIResponse struct {
	ID              string `json:"id"`
	Address         string `json:"address"`
	Status          string `json:"status"`
	FirmwareVersion string `json:"firmware_version"`
	Metrics         struct {
		AvgLatency int64 `json:"avg_latency"`
	} `json:"metrics"`
}

// routeAPIResponse mirrors the JSON shape returned by GET /v1/routes.
type routeAPIResponse struct {
	ID        string `json:"id"`
	Endpoints []struct {
		NodeID  string  `json:"node_id"`
		Address string  `json:"address"`
		Weight  float64 `json:"weight"`
	} `json:"endpoints"`
}

// fetchNodes calls GET <serverURL>/v1/nodes and converts the result.
func fetchNodes(serverURL string) ([]NodeRow, error) {
	url := strings.TrimRight(serverURL, "/") + "/api/v1/nodes"
	resp, err := http.Get(url) //nolint:gosec // URL comes from operator config
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []NodeRow{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: unexpected status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read nodes response: %w", err)
	}

	var raw []nodeAPIResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode nodes JSON: %w", err)
	}

	rows := make([]NodeRow, 0, len(raw))
	for _, n := range raw {
		latency := fmt.Sprintf("%dms", n.Metrics.AvgLatency/int64(time.Millisecond))
		rows = append(rows, NodeRow{
			ID:      n.ID,
			Status:  n.Status,
			Latency: latency,
			Region:  n.Address,
		})
	}
	return rows, nil
}

// fetchRoutes calls GET <serverURL>/v1/routes and converts the result.
func fetchRoutes(serverURL string) ([]RouteRow, error) {
	url := strings.TrimRight(serverURL, "/") + "/api/v1/routes"
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []RouteRow{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: unexpected status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read routes response: %w", err)
	}

	var raw []routeAPIResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode routes JSON: %w", err)
	}

	rows := make([]RouteRow, 0, len(raw))
	for _, r := range raw {
		nextHop := ""
		cap := ""
		score := ""
		if len(r.Endpoints) > 0 {
			nextHop = r.Endpoints[0].NodeID
			score = fmt.Sprintf("%.2f", r.Endpoints[0].Weight)
		}
		rows = append(rows, RouteRow{
			Destination: r.ID,
			NextHop:     nextHop,
			Capability:  cap,
			Score:       score,
		})
	}
	return rows, nil
}
