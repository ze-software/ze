// Design: docs/architecture/api/commands.md — dashboard session lifecycle.
// Overview: model.go — editor model and update loop.
// Related: model_dashboard_sort.go — sort column enum and sort logic.
// Related: model_dashboard_render.go — dashboard rendering.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
)

// dashboardPollInterval is how often the dashboard refreshes data.
const dashboardPollInterval = 2 * time.Second

// DashboardFactory creates a dashboard polling function.
// The returned function calls commandExecutor("bgp summary") and returns the JSON.
// DashboardFactory creates a dashboard poller.
// Type alias of contract.DashboardFactory so ssh, web, and hub use the same type.
type DashboardFactory = contract.DashboardFactory

// dashboardTickMsg triggers a dashboard data poll.
type dashboardTickMsg struct{}

// dashboardDataMsg carries the result of a poll.
type dashboardDataMsg struct {
	data string
	err  error
}

// dashboardPeer holds per-peer data parsed from the summary RPC response.
type dashboardPeer struct {
	Address            string
	RemoteAS           uint32
	State              string
	Uptime             string
	UpdatesReceived    uint32
	UpdatesSent        uint32
	KeepalivesReceived uint32
	KeepalivesSent     uint32
	EORReceived        uint32
	EORSent            uint32
}

// dashboardSnapshot holds the parsed summary RPC response.
type dashboardSnapshot struct {
	RouterID         string
	LocalAS          uint32
	Uptime           string
	PeersConfigured  int
	PeersEstablished int
	Peers            []dashboardPeer
}

// parseDashboardSnapshot parses the JSON output of "bgp summary" via commandExecutor.
// The format is: {"summary": {"router-id": ..., "peers": [...]}}.
func parseDashboardSnapshot(data string) (*dashboardSnapshot, error) {
	var raw struct {
		Summary struct {
			RouterID         string `json:"router-id"`
			LocalAS          uint32 `json:"local-as"`
			Uptime           string `json:"uptime"`
			PeersConfigured  int    `json:"peers-configured"`
			PeersEstablished int    `json:"peers-established"`
			Peers            []struct {
				Address            string `json:"address"`
				RemoteAS           uint32 `json:"remote-as"`
				State              string `json:"state"`
				Uptime             string `json:"uptime"`
				UpdatesReceived    uint32 `json:"updates-received"`
				UpdatesSent        uint32 `json:"updates-sent"`
				KeepalivesReceived uint32 `json:"keepalives-received"`
				KeepalivesSent     uint32 `json:"keepalives-sent"`
				EORReceived        uint32 `json:"eor-received"`
				EORSent            uint32 `json:"eor-sent"`
			} `json:"peers"`
		} `json:"summary"`
	}

	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, fmt.Errorf("parse summary: %w", err)
	}

	snap := &dashboardSnapshot{
		RouterID:         raw.Summary.RouterID,
		LocalAS:          raw.Summary.LocalAS,
		Uptime:           raw.Summary.Uptime,
		PeersConfigured:  raw.Summary.PeersConfigured,
		PeersEstablished: raw.Summary.PeersEstablished,
		Peers:            make([]dashboardPeer, len(raw.Summary.Peers)),
	}
	for i, p := range raw.Summary.Peers {
		snap.Peers[i] = dashboardPeer{
			Address:            p.Address,
			RemoteAS:           p.RemoteAS,
			State:              p.State,
			Uptime:             p.Uptime,
			UpdatesReceived:    p.UpdatesReceived,
			UpdatesSent:        p.UpdatesSent,
			KeepalivesReceived: p.KeepalivesReceived,
			KeepalivesSent:     p.KeepalivesSent,
			EORReceived:        p.EORReceived,
			EORSent:            p.EORSent,
		}
	}

	return snap, nil
}

// peerRateEntry tracks per-peer counter and timestamp for rate computation.
type peerRateEntry struct {
	counter   uint32
	timestamp time.Time
	rate      string // formatted rate or "--"
}

// dashboardState holds the dashboard's mutable state.
type dashboardState struct {
	snapshot     *dashboardSnapshot
	sortColumn   dashboardSortColumn
	sortAsc      bool
	selectedAddr string // peer address for selection persistence
	selectedIdx  int
	lastPollTime time.Time
	pollError    string
	detailAddr   string         // non-empty when in detail view
	detailData   map[string]any // extended peer detail from RPC
	showHelp     bool           // help overlay visible
	poller       func() (string, error)
	rates        map[string]*peerRateEntry
}

// updateRates computes per-peer update rates from counter diffs between polls.
func (ds *dashboardState) updateRates(snap *dashboardSnapshot, now time.Time) {
	if ds.rates == nil {
		ds.rates = make(map[string]*peerRateEntry)
	}

	// Build set of current peers for cleanup.
	currentPeers := make(map[string]bool, len(snap.Peers))
	for i := range snap.Peers {
		addr := snap.Peers[i].Address
		currentPeers[addr] = true
		counter := snap.Peers[i].UpdatesReceived

		prev, exists := ds.rates[addr]
		if !exists {
			// First time seeing this peer.
			ds.rates[addr] = &peerRateEntry{
				counter:   counter,
				timestamp: now,
				rate:      "--",
			}
			continue
		}

		elapsed := now.Sub(prev.timestamp).Seconds()
		if elapsed < 0.5 {
			// Too short interval, keep previous rate, update counter.
			prev.counter = counter
			continue
		}

		if counter < prev.counter {
			// Counter decreased (peer restart). Reset baseline.
			prev.counter = counter
			prev.timestamp = now
			prev.rate = "--"
			continue
		}

		diff := counter - prev.counter
		rate := float64(diff) / elapsed
		prev.counter = counter
		prev.timestamp = now
		prev.rate = fmt.Sprintf("%.1f/s", rate)
	}

	// Remove entries for peers that disappeared.
	for addr := range ds.rates {
		if !currentPeers[addr] {
			delete(ds.rates, addr)
		}
	}
}

// peerRate returns the formatted rate string for a peer, or "--" if unknown.
func (ds *dashboardState) peerRate(addr string) string {
	if entry, ok := ds.rates[addr]; ok {
		return entry.rate
	}
	return "--"
}

// resolveSelectedIndex finds the index of the selected peer in the given slice.
// If the selected peer is not found, returns 0.
func (ds *dashboardState) resolveSelectedIndex(peers []dashboardPeer) int {
	for i, p := range peers {
		if p.Address == ds.selectedAddr {
			return i
		}
	}
	return 0
}

// SetDashboardFactory sets the factory used to create dashboard sessions.
func (m *Model) SetDashboardFactory(f DashboardFactory) {
	m.dashboardFactory = f
}

// IsDashboard returns true if the dashboard is active.
func (m Model) IsDashboard() bool {
	return m.dashboard != nil
}

// isDashboardCommand returns true if the input should enter dashboard mode.
// Follows verb-first convention: "monitor bgp" = <action> <module>.
func isDashboardCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	return trimmed == "monitor bgp" || strings.HasPrefix(trimmed, "monitor bgp ")
}

// startDashboard enters dashboard mode.
func (m *Model) startDashboard() tea.Cmd {
	if m.dashboardFactory == nil {
		m.statusMessage = "dashboard not available (no daemon connection)"
		return nil
	}

	poller, err := m.dashboardFactory()
	if err != nil {
		m.err = err
		return nil
	}

	m.dashboard = &dashboardState{
		sortAsc: true,
		poller:  poller,
		rates:   make(map[string]*peerRateEntry),
	}

	// Do initial poll immediately.
	return m.dashboardPollCmd()
}

// stopDashboard exits dashboard mode.
func (m *Model) stopDashboard() {
	m.dashboard = nil
	m.statusMessage = "dashboard stopped"
}

// dashboardPollCmd returns a tea.Cmd that polls for data.
func (m *Model) dashboardPollCmd() tea.Cmd {
	if m.dashboard == nil || m.dashboard.poller == nil {
		return nil
	}
	poller := m.dashboard.poller
	return func() tea.Msg {
		data, err := poller()
		return dashboardDataMsg{data: data, err: err}
	}
}

// dashboardScheduleTick returns a tea.Cmd that schedules the next poll.
func dashboardScheduleTick() tea.Cmd {
	return tea.Tick(dashboardPollInterval, func(time.Time) tea.Msg { return dashboardTickMsg{} })
}

// handleDashboardData processes a poll result.
func (m Model) handleDashboardData(msg dashboardDataMsg) (tea.Model, tea.Cmd) {
	if m.dashboard == nil {
		return m, nil
	}

	now := time.Now()
	m.dashboard.lastPollTime = now

	if msg.err != nil {
		m.dashboard.pollError = msg.err.Error()
		return m, dashboardScheduleTick()
	}

	m.dashboard.pollError = ""
	snap, err := parseDashboardSnapshot(msg.data)
	if err != nil {
		m.dashboard.pollError = "parse error: " + err.Error()
		return m, dashboardScheduleTick()
	}

	m.dashboard.updateRates(snap, now)
	m.dashboard.snapshot = snap

	// Update selected index after data refresh.
	if snap != nil && len(snap.Peers) > 0 {
		sorted := sortDashboardPeers(snap.Peers, m.dashboard.sortColumn, m.dashboard.sortAsc)
		m.dashboard.selectedIdx = m.dashboard.resolveSelectedIndex(sorted)
		if m.dashboard.selectedIdx < len(sorted) {
			m.dashboard.selectedAddr = sorted[m.dashboard.selectedIdx].Address
		}

		// If in detail view and peer disappeared, return to table.
		// Otherwise refresh detail data.
		if m.dashboard.detailAddr != "" {
			found := false
			for _, p := range snap.Peers {
				if p.Address == m.dashboard.detailAddr {
					found = true
					break
				}
			}
			if !found {
				m.dashboard.detailAddr = ""
				m.dashboard.detailData = nil
				m.statusMessage = "peer disconnected"
			} else {
				m.fetchPeerDetail(m.dashboard.detailAddr)
			}
		}
	}

	return m, dashboardScheduleTick()
}

// handleDashboardKey handles keyboard input in dashboard mode.
// Returns true if the key was handled.
func (m *Model) handleDashboardKey(keyStr string) bool {
	if m.dashboard == nil {
		return false
	}

	ds := m.dashboard

	// Help overlay: ? toggles, any other key dismisses.
	if ds.showHelp {
		ds.showHelp = false
		return true
	}

	// Detail view: Esc or Backspace returns to table.
	if ds.detailAddr != "" {
		switch keyStr {
		case "esc", "backspace":
			ds.detailAddr = ""
			ds.detailData = nil
		case "q", "ctrl+c":
			m.stopDashboard()
		case "?":
			ds.showHelp = true
		}
		return true // Absorb all keys in detail view.
	}

	// Peer table view.
	switch keyStr {
	case "q", "ctrl+c", "esc":
		m.stopDashboard()
	case "?":
		ds.showHelp = true
	case "s":
		ds.sortColumn = ds.sortColumn.next()
	case "S":
		ds.sortAsc = !ds.sortAsc
	case "j", "down":
		if ds.snapshot != nil && len(ds.snapshot.Peers) > 0 {
			ds.selectedIdx = min(ds.selectedIdx+1, len(ds.snapshot.Peers)-1)
			sorted := sortDashboardPeers(ds.snapshot.Peers, ds.sortColumn, ds.sortAsc)
			if ds.selectedIdx < len(sorted) {
				ds.selectedAddr = sorted[ds.selectedIdx].Address
			}
		}
	case "k", "up":
		if ds.selectedIdx > 0 {
			ds.selectedIdx--
		}
		if ds.snapshot != nil && len(ds.snapshot.Peers) > 0 {
			sorted := sortDashboardPeers(ds.snapshot.Peers, ds.sortColumn, ds.sortAsc)
			if ds.selectedIdx < len(sorted) {
				ds.selectedAddr = sorted[ds.selectedIdx].Address
			}
		}
	case "enter":
		if ds.snapshot != nil && len(ds.snapshot.Peers) > 0 {
			sorted := sortDashboardPeers(ds.snapshot.Peers, ds.sortColumn, ds.sortAsc)
			if ds.selectedIdx < len(sorted) {
				ds.detailAddr = sorted[ds.selectedIdx].Address
				ds.detailData = nil
				m.fetchPeerDetail(ds.detailAddr)
			}
		}
	}

	return true // Absorb all keys in dashboard mode.
}

// fetchPeerDetail fetches extended peer info via commandExecutor.
// Results are stored in ds.detailData for rendering.
func (m *Model) fetchPeerDetail(addr string) {
	if m.commandExecutor == nil || m.dashboard == nil {
		return
	}
	data, err := m.commandExecutor("peer " + addr + " detail")
	if err != nil {
		return
	}
	m.dashboard.detailData = parsePeerDetail(data, addr)
}

// parsePeerDetail extracts the detail map for a specific peer from the RPC response.
// Response format: {"peers": {"<ip>": {...}}}.
func parsePeerDetail(data, addr string) map[string]any {
	var raw struct {
		Peers map[string]map[string]any `json:"peers"`
	}
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil
	}
	return raw.Peers[addr]
}

// renderDashboard renders the full dashboard screen.
func (m Model) renderDashboard() string {
	ds := m.dashboard
	if ds == nil {
		return ""
	}

	var sb strings.Builder
	width := m.width
	if width <= 0 {
		width = 80
	}

	// Help overlay replaces everything.
	if ds.showHelp {
		return renderDashboardHelp()
	}

	// Header (2 lines).
	sb.WriteString(renderDashboardHeader(ds.snapshot, ds.pollError, width))
	sb.WriteString("\n")

	// Peer table or detail view.
	if ds.detailAddr != "" {
		sb.WriteString(renderDashboardDetail(ds))
	} else {
		var peers []dashboardPeer
		if ds.snapshot != nil {
			peers = sortDashboardPeers(ds.snapshot.Peers, ds.sortColumn, ds.sortAsc)
		}
		tableHeight := max(1, m.height-4) // header(2) + footer(1) + separator(1)
		sb.WriteString(renderDashboardPeerTable(peers, ds, ds.sortColumn, ds.sortAsc, width, tableHeight))
	}

	sb.WriteString("\n")

	// Footer (1 line).
	lastUpdate := ""
	if !ds.lastPollTime.IsZero() {
		ago := time.Since(ds.lastPollTime).Truncate(time.Second)
		lastUpdate = ago.String() + " ago"
	}
	sb.WriteString(renderDashboardFooter(lastUpdate, width))

	return sb.String()
}
