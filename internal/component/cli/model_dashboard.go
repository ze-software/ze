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

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// dashboardPollInterval is how often the dashboard refreshes data.
//
// One second is what an operator watching a session expects: uptime counts up
// the way a clock does. At two seconds it advanced in visible jumps, because
// nothing between polls is interpolated.
//
// The rate column does not depend on this value. updateRates divides by the
// MEASURED time between two snapshots, and holds the previous rate when that
// measure is under half a second.
//
// A shorter interval therefore changes how often a rate is recomputed. It
// never changes what the rate reports.
const dashboardPollInterval = 1 * time.Second

// DashboardFactory creates a dashboard polling function.
// The returned function calls commandExecutor("show bgp") and returns the JSON.
// Type alias of contract.DashboardFactory so ssh, web, and hub use the same type.
type DashboardFactory = contract.DashboardFactory

// ViewKeyDashboard is the registry/factory key for the bgp-monitor dashboard
// live view. Consumers inject a DashboardFactory under this key via SetViewFactory.
const ViewKeyDashboard = "dashboard"

// dashboardTickMsg triggers a dashboard data poll.
type dashboardTickMsg struct{}

// dashboardDataMsg carries the result of a poll.
type dashboardDataMsg struct {
	data string
	err  error
}

func (dashboardTickMsg) isViewMsg() {}
func (dashboardDataMsg) isViewMsg() {}

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

// parseDashboardSnapshot parses the JSON output of "show bgp" via commandExecutor.
// The format is: {"router-id": ..., "peers": [...]}, aggregates and rows as siblings.
func parseDashboardSnapshot(data string) (*dashboardSnapshot, error) {
	var raw struct {
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
	}

	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, fmt.Errorf("parse summary: %w", err)
	}

	snap := &dashboardSnapshot{
		RouterID:         raw.RouterID,
		LocalAS:          raw.LocalAS,
		Uptime:           raw.Uptime,
		PeersConfigured:  raw.PeersConfigured,
		PeersEstablished: raw.PeersEstablished,
		Peers:            make([]dashboardPeer, len(raw.Peers)),
	}
	for i, p := range raw.Peers {
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

// sortedPeers returns the peer rows in the order the table shows them, which is
// the only order the selection index means anything in.
func (ds *dashboardState) sortedPeers() []dashboardPeer {
	if ds.snapshot == nil {
		return nil
	}
	return sortDashboardPeers(ds.snapshot.Peers, ds.sortColumn, ds.sortAsc)
}

// followSelection keeps the highlight on the peer it was on after the table has
// been re-ordered or refreshed.
//
// The index is a position in the sorted table. A new sort column, a reversed
// direction, and a peer that came or went each move the peer but not the index.
// An index left alone puts the highlight on a different peer until the next poll
// resolves it again. An Enter inside that second opens a session the operator
// never pointed at.
func (ds *dashboardState) followSelection() {
	peers := ds.sortedPeers()
	if len(peers) == 0 {
		return
	}
	ds.selectedIdx = ds.resolveSelectedIndex(peers)
	ds.selectedAddr = peers[ds.selectedIdx].Address
}

// moveSelection moves the highlight by delta rows of the sorted table. It
// records the peer it landed on, so the next poll finds it again wherever the
// refreshed data puts it.
func (ds *dashboardState) moveSelection(delta int) {
	peers := ds.sortedPeers()
	if len(peers) == 0 {
		return
	}
	ds.selectedIdx = min(max(ds.selectedIdx+delta, 0), len(peers)-1)
	ds.selectedAddr = peers[ds.selectedIdx].Address
}

// dashboardFactory returns the injected DashboardFactory, or nil when none is
// registered or the stored value is the wrong type (fail-closed).
func (m *Model) dashboardFactory() DashboardFactory {
	raw, present := m.viewFactoryRaw(ViewKeyDashboard)
	if !present {
		return nil
	}
	f, ok := raw.(DashboardFactory)
	if !ok {
		return nil
	}
	return f
}

// activeDashboard returns the active dashboard state, or nil when the active
// view is not the dashboard.
func (m *Model) activeDashboard() *dashboardState {
	if v, ok := m.activeView.(*dashboardView); ok {
		return v.st
	}
	return nil
}

// isDashboardCommand returns true if the input should enter dashboard mode.
// Follows verb-first convention: "monitor bgp" = <action> <module>.
func isDashboardCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	return trimmed == "monitor bgp" || strings.HasPrefix(trimmed, "monitor bgp ")
}

// startDashboard enters dashboard mode.
func (m *Model) startDashboard() tea.Cmd {
	factory := m.dashboardFactory()
	if factory == nil {
		m.statusMessage = "dashboard not available (no daemon connection)"
		return nil
	}

	poller, err := factory()
	if err != nil {
		m.err = err
		return nil
	}

	m.activeView = &dashboardView{st: &dashboardState{
		sortAsc: true,
		poller:  poller,
		rates:   make(map[string]*peerRateEntry),
	}}

	// Do initial poll immediately.
	return m.dashboardPollCmd()
}

// stopDashboard exits dashboard mode.
func (m *Model) stopDashboard() {
	m.activeView = nil
	m.statusMessage = "dashboard stopped"
}

// dashboardPollCmd returns a tea.Cmd that polls for data.
func (m *Model) dashboardPollCmd() tea.Cmd {
	ds := m.activeDashboard()
	if ds == nil || ds.poller == nil {
		return nil
	}
	poller := ds.poller
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
	if m.activeDashboard() == nil {
		return m, nil
	}

	now := time.Now()
	m.activeDashboard().lastPollTime = now

	if msg.err != nil {
		m.activeDashboard().pollError = msg.err.Error()
		return m, dashboardScheduleTick()
	}

	m.activeDashboard().pollError = ""
	snap, err := parseDashboardSnapshot(msg.data)
	if err != nil {
		var tb textbuf.Buffer
		m.activeDashboard().pollError = tb.Str("parse error: ").Err(err).String()
		return m, dashboardScheduleTick()
	}

	m.activeDashboard().updateRates(snap, now)
	m.activeDashboard().snapshot = snap

	// Update selected index after data refresh.
	if snap != nil && len(snap.Peers) > 0 {
		m.activeDashboard().followSelection()

		// If in detail view and peer disappeared, return to table.
		// Otherwise refresh detail data.
		if m.activeDashboard().detailAddr != "" {
			found := false
			for _, p := range snap.Peers {
				if p.Address == m.activeDashboard().detailAddr {
					found = true
					break
				}
			}
			if !found {
				m.activeDashboard().detailAddr = ""
				m.activeDashboard().detailData = nil
				m.statusMessage = "peer disconnected"
			} else {
				m.fetchPeerDetail(m.activeDashboard().detailAddr)
			}
		}
	}

	return m, dashboardScheduleTick()
}

// handleDashboardKey handles keyboard input in dashboard mode.
// Returns true if the key was handled.
func (m *Model) handleDashboardKey(keyStr string) bool {
	if m.activeDashboard() == nil {
		return false
	}

	ds := m.activeDashboard()

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
		ds.followSelection()
	case "S":
		ds.sortAsc = !ds.sortAsc
		ds.followSelection()
	case "j", "down":
		ds.moveSelection(1)
	case "k", "up":
		ds.moveSelection(-1)
	case "enter":
		peers := ds.sortedPeers()
		if ds.selectedIdx < len(peers) {
			ds.detailAddr = peers[ds.selectedIdx].Address
			ds.detailData = nil
			m.fetchPeerDetail(ds.detailAddr)
		}
	}

	return true // Absorb all keys in dashboard mode.
}

// fetchPeerDetail fetches extended peer info via commandExecutor.
// Results are stored in ds.detailData for rendering.
func (m *Model) fetchPeerDetail(addr string) {
	if m.commandExecutor == nil || m.activeDashboard() == nil {
		return
	}
	var tb textbuf.Buffer
	data, err := m.commandExecutor(tb.Str("show bgp peer ").Str(addr).Str(" detail").String())
	if data.TransportComplete != nil {
		defer data.TransportComplete()
	}
	if err != nil {
		return
	}
	m.activeDashboard().detailData = parsePeerDetail(data.Text, addr)
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
	ds := m.activeDashboard()
	if ds == nil {
		return ""
	}

	var sb textbuf.Buffer
	width := m.width
	if width <= 0 {
		width = 80
	}

	// Help overlay replaces everything.
	if ds.showHelp {
		return renderDashboardHelp()
	}

	// Header (2 lines).
	sb.Str(renderDashboardHeader(ds.snapshot, width)).Byte('\n')

	// Peer table or detail view.
	if ds.detailAddr != "" {
		sb.Str(renderDashboardDetail(ds))
	} else {
		peers := ds.sortedPeers()
		tableHeight := max(1, m.height-3) // header(1) + footer(1) + separator(1)
		sb.Str(renderDashboardPeerTable(peers, ds, ds.sortColumn, ds.sortAsc, width, tableHeight))
	}

	sb.Byte('\n')

	// Footer (1 line).
	lastUpdate := ""
	if !ds.lastPollTime.IsZero() {
		ago := time.Since(ds.lastPollTime).Truncate(time.Second)
		var tb3 textbuf.Buffer
		lastUpdate = tb3.Str(ago.String()).Str(" ago").String()
	}
	sb.Str(renderDashboardFooter(lastUpdate, width))

	return sb.String()
}
