// Design: docs/architecture/web-workbench-pages.md -- BGP Peers table page
// Related: workbench_table.go -- Reusable table component
// Related: workbench_detail.go -- Reusable detail panel component
// Related: page_interfaces.go -- Sibling page (pattern reference)

package web

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// bgpPeerPathPrefix is the base path for per-peer show/edit/action URLs
// (e.g. /show/bgp/peer/<name>/...).
const bgpPeerPathPrefix = "/show/bgp/peer/"

// peerEntry holds extracted fields for one BGP peer from the config tree.
type peerEntry struct {
	Name     string
	RemoteIP string
	RemoteAS string
	LocalAS  string
	Group    string
	Families string
	Disabled bool
	EditURL  string
}

// collectPeers walks the config tree and returns all BGP peers from both
// standalone bgp/peer and grouped bgp/group/*/peer locations.
func collectPeers(viewTree *config.Tree) []peerEntry {
	if viewTree == nil {
		return nil
	}
	bgpTree := viewTree.GetContainer("bgp")
	if bgpTree == nil {
		return nil
	}

	var peers []peerEntry

	// Standalone peers at bgp/peer
	for _, entry := range bgpTree.GetListOrdered("peer") {
		pe := extractPeerEntry(entry.Key, entry.Value, "")
		peers = append(peers, pe)
	}

	// Grouped peers at bgp/group/*/peer
	for _, groupEntry := range bgpTree.GetListOrdered("group") {
		groupName := groupEntry.Key
		groupTree := groupEntry.Value
		if groupTree == nil {
			continue
		}
		for _, peerItem := range groupTree.GetListOrdered("peer") {
			pe := extractPeerEntry(peerItem.Key, peerItem.Value, groupName)
			peers = append(peers, pe)
		}
	}

	return peers
}

// extractPeerEntry reads relevant fields from one peer's config sub-tree.
func extractPeerEntry(name string, peerTree *config.Tree, group string) peerEntry {
	pe := peerEntry{
		Name:  name,
		Group: group,
	}
	if peerTree == nil {
		return pe
	}

	// connection/remote/ip
	if conn := peerTree.GetContainer("connection"); conn != nil {
		if remote := conn.GetContainer("remote"); remote != nil {
			if ip, ok := remote.Get("ip"); ok {
				pe.RemoteIP = ip
			}
		}
	}

	// session/asn/local and session/asn/remote
	if sess := peerTree.GetContainer("session"); sess != nil {
		if asn := sess.GetContainer("asn"); asn != nil {
			if local, ok := asn.Get("local"); ok {
				pe.LocalAS = local
			}
			if remote, ok := asn.Get("remote"); ok {
				pe.RemoteAS = remote
			}
		}

		// Collect families from session/family list
		families := sess.GetListOrdered("family")
		names := make([]string, 0, len(families))
		for _, f := range families {
			names = append(names, f.Key)
		}
		pe.Families = textbuf.Join(names, ", ")
	}

	// Disabled state: check if the peer is explicitly disabled.
	// In Ze, a deactivated list entry has an inactive marker.
	// For v1 we treat all configured peers as enabled.
	pe.Disabled = false

	// Build edit URL
	var tb textbuf.Buffer
	if group != "" {
		pe.EditURL = tb.Str("/show/bgp/group/").Str(group).Str("/peer/").Str(name).Byte('/').String()
	} else {
		pe.EditURL = tb.Str(bgpPeerPathPrefix).Str(name).Byte('/').String()
	}

	return pe
}

// Peer state display strings (config-only v1; future spec adds FSM states).
const (
	peerStateConfigured = "Configured"
	peerStateDisabled   = "Disabled"
)

// peerFlagFromState computes the flag using live FSM state when available.
func peerFlagFromState(pe peerEntry, state string) (string, string) {
	if pe.Disabled {
		return "D", flagClassGrey
	}
	switch state {
	case "Established":
		return "E", flagClassGreen
	case "Idle", "":
		return "I", flagClassRed
	case peerStateConfigured:
		return "C", flagClassGreen
	default:
		return "A", flagClassYellow
	}
}

// buildBGPPeersTableData constructs a WorkbenchTableData from a list
// of peer entries. filterGroup restricts the table to peers in that group.
// live provides operational state from "show bgp" (nil when unavailable).
func buildBGPPeersTableData(peers []peerEntry, filterGroup string, live map[string]bgpSummaryPeer) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "remote-ip", Label: "Remote IP", Sortable: true},
		{Key: "remote-as", Label: "Remote AS", Sortable: true},
		{Key: "local-as", Label: "Local AS", Sortable: true},
		{Key: "group", Label: "Group", Sortable: true},
		{Key: "state", Label: "State"},
		{Key: "families", Label: "Families"},
	}

	var rows []WorkbenchTableRow
	for _, pe := range peers {
		if filterGroup != "" && pe.Group != filterGroup {
			continue
		}

		state := peerStateConfigured
		if pe.Disabled {
			state = peerStateDisabled
		}
		if op, ok := live[pe.Name]; ok {
			state = op.State
		}

		flags, flagClass := peerFlagFromState(pe, state)

		cells := []string{
			pe.Name,
			valueOrDash(pe.RemoteIP),
			valueOrDash(pe.RemoteAS),
			valueOrDash(pe.LocalAS),
			valueOrDash(pe.Group),
			state,
			valueOrDash(pe.Families),
		}

		actions := []WorkbenchRowAction{
			{Label: "Edit", URL: pe.EditURL},
		}

		// Row-level operational tools (dispatch through /tools/related/run).
		// The tool_id and context_path are sent; the server resolves the
		// actual command from the YANG ze:related annotations.
		if pe.RemoteIP != "" {
			var tb textbuf.Buffer
			contextPath := tb.Str("bgp/peer/").Str(pe.Name).String()
			if pe.Group != "" {
				contextPath = tb.Reset().Str("bgp/group/").Str(pe.Group).Str("/peer/").Str(pe.Name).String()
			}
			actions = append(actions,
				WorkbenchRowAction{
					Label:  "Detail",
					HxPost: "/tools/related/run",
					Class:  "inspect",
				},
				WorkbenchRowAction{
					Label:   "Teardown",
					HxPost:  "/tools/related/run",
					Class:   "danger",
					Confirm: tb.Reset().Str("Tear down BGP session with ").Str(pe.Name).Str(" (").Str(pe.RemoteIP).Str(")?").String(),
				},
			)
			_ = contextPath // context_path sent via hidden form fields in the template
		}

		rows = append(rows, WorkbenchTableRow{
			Key:       pe.Name,
			URL:       pe.EditURL,
			Flags:     flags,
			FlagClass: flagClass,
			Cells:     cells,
			Actions:   actions,
		})
	}

	emptyMsg := "No BGP peers configured."
	if filterGroup != "" {
		var tb textbuf.Buffer
		emptyMsg = tb.Str("No peers in group ").Str(strconv.Quote(filterGroup)).Byte('.').String()
	}

	addURL := bgpPeerPathPrefix
	if filterGroup != "" {
		var tb textbuf.Buffer
		addURL = tb.Str("/show/bgp/group/").Str(filterGroup).Str("/peer/").String()
	}

	return WorkbenchTableData{
		Title:        "BGP Peers",
		AddURL:       addURL,
		AddLabel:     "Add Peer",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: emptyMsg,
		EmptyHint:    "Add a BGP peer to begin exchanging routes.",
	}
}

// handleBGPPeersPage renders the BGP peers table within the workbench.
func handleBGPPeersPage(renderer *Renderer, r *http.Request, viewTree *config.Tree, filterGroup string, dispatch CommandDispatcher) template.HTML {
	peers := collectPeers(viewTree)
	live := fetchBGPSummaryPeers(r, dispatch)
	tableData := buildBGPPeersTableData(peers, filterGroup, live)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

// buildBGPPeerDetailData constructs a WorkbenchDetailData for a single peer.
func buildBGPPeerDetailData(renderer *Renderer, pe peerEntry) WorkbenchDetailData {
	configHTML := buildPeerConfigHTML(renderer, pe)
	statusHTML := buildPeerStatusHTML(renderer, pe)
	actionsHTML := buildPeerActionsHTML(renderer, pe)

	tabs := []WorkbenchDetailTab{
		{Key: "config", Label: "Config", Content: configHTML, Active: true},
		{Key: "status", Label: "Status", Content: statusHTML},
		{Key: "actions", Label: "Actions", Content: actionsHTML},
	}

	closeURL := bgpPeerPathPrefix
	if pe.Group != "" {
		var tb textbuf.Buffer
		closeURL = tb.Str("/show/bgp/group/").Str(pe.Group).Str("/peer/").String()
	}

	return WorkbenchDetailData{
		Title:    pe.Name,
		Tabs:     tabs,
		CloseURL: closeURL,
	}
}

// peerActionsData is what peerDetailActions renders. ContextPath addresses the
// peer in the config tree and both buttons carry it to the tool runner. A peer
// with no remote address reaches no session, so HasRemoteIP withholds them.
type peerActionsData struct {
	Name        string
	ContextPath string
	HasRemoteIP bool
}

// peerToolVals is the hx-vals payload naming the tool a button runs and the
// peer it runs against.
func peerToolVals(toolID, contextPath string) string {
	var tb textbuf.Buffer

	return tb.Str(`{"tool_id":`).Quoted(toolID).Str(`,"context_path":`).Quoted(contextPath).Byte('}').String()
}

// peerTeardownConfirm is the question teardown asks before it drops a session.
func peerTeardownConfirm(name string) string {
	var tb textbuf.Buffer

	return tb.Str("Tear down BGP session with ").Str(name).Byte('?').String()
}

func buildPeerConfigHTML(renderer *Renderer, pe peerEntry) template.HTML {
	rows := []detailKV{
		{Key: "Name", Value: pe.Name},
		{Key: "Remote IP", Value: valueOrDash(pe.RemoteIP)},
		{Key: "Remote AS", Value: valueOrDash(pe.RemoteAS)},
		{Key: "Local AS", Value: valueOrDash(pe.LocalAS)},
		{Key: "Group", Value: valueOrDash(pe.Group)},
		{Key: "Families", Value: valueOrDash(pe.Families)},
	}

	return renderer.renderComponent("peer_detail_config", detailKVSection(rows))
}

func buildPeerStatusHTML(renderer *Renderer, pe peerEntry) template.HTML {
	state := peerStateConfigured
	if pe.Disabled {
		state = peerStateDisabled
	}

	rows := []detailKV{
		{Key: "State", Value: state},
		{Key: "Uptime", Value: "--"},
		{Key: "Prefixes Received", Value: "--"},
		{Key: "Messages In", Value: "--"},
		{Key: "Messages Out", Value: "--"},
		{Key: "Last Error", Value: "--"},
	}

	return renderer.renderComponent("peer_detail_status", peerDetailStatus(rows))
}

func buildPeerActionsHTML(renderer *Renderer, pe peerEntry) template.HTML {
	var tb textbuf.Buffer

	contextPath := tb.Str("bgp/peer/").Str(pe.Name).String()
	if pe.Group != "" {
		contextPath = tb.Reset().Str("bgp/group/").Str(pe.Group).Str("/peer/").Str(pe.Name).String()
	}

	return renderer.renderComponent("peer_detail_actions", peerDetailActions(peerActionsData{
		Name:        pe.Name,
		ContextPath: contextPath,
		HasRemoteIP: pe.RemoteIP != "",
	}))
}

// valueOrDash returns v if non-empty, "-" otherwise.
func valueOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
