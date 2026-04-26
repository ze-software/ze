// Design: plan/spec-web-5-bgp.md -- BGP Peers table page
// Related: workbench_table.go -- Reusable table component
// Related: workbench_detail.go -- Reusable detail panel component
// Related: page_interfaces.go -- Sibling page (pattern reference)

package web

import (
	"fmt"
	"html/template"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

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
		pe.Families = strings.Join(names, ", ")
	}

	// Disabled state: check if the peer is explicitly disabled.
	// In Ze, a deactivated list entry has an inactive marker.
	// For v1 we treat all configured peers as enabled.
	pe.Disabled = false

	// Build edit URL
	if group != "" {
		pe.EditURL = fmt.Sprintf("/show/bgp/group/%s/peer/%s/", group, name)
	} else {
		pe.EditURL = fmt.Sprintf("/show/bgp/peer/%s/", name)
	}

	return pe
}

// Peer state display strings (config-only v1; future spec adds FSM states).
const (
	peerStateConfigured = "Configured"
	peerStateDisabled   = "Disabled"
)

// peerFlag computes the flag string and CSS class for a peer row.
// For v1 (config-only, no live state), configured peers show green,
// disabled peers show grey.
func peerFlag(pe peerEntry) (string, string) {
	if pe.Disabled {
		return "D", flagClassGrey
	}
	// Config-only: mark as configured (green). Once operational state
	// is available (future spec), this will use FSM state colors:
	// green=Established, red=Idle/error, yellow=Active/Connect.
	return "C", flagClassGreen
}

// BuildBGPPeersTableData constructs a WorkbenchTableData from a list
// of peer entries. filterGroup restricts the table to peers in that group.
func BuildBGPPeersTableData(peers []peerEntry, filterGroup string) WorkbenchTableData {
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

		flags, flagClass := peerFlag(pe)

		// State column is placeholder until operational data is available.
		state := peerStateConfigured
		if pe.Disabled {
			state = peerStateDisabled
		}

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
			contextPath := "bgp/peer/" + pe.Name
			if pe.Group != "" {
				contextPath = "bgp/group/" + pe.Group + "/peer/" + pe.Name
			}
			actions = append(actions,
				WorkbenchRowAction{
					Label:  "Detail",
					HxPost: "/tools/related/run",
					Class:  "inspect",
				},
				WorkbenchRowAction{
					Label:  "Teardown",
					HxPost: "/tools/related/run",
					Class:  "danger",
					Confirm: fmt.Sprintf("Tear down BGP session with %s (%s)?",
						pe.Name, pe.RemoteIP),
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
		emptyMsg = fmt.Sprintf("No peers in group %q.", filterGroup)
	}

	return WorkbenchTableData{
		Title:        "BGP Peers",
		AddURL:       "/show/bgp/peer/add",
		AddLabel:     "Add Peer",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: emptyMsg,
		EmptyHint:    "Add a BGP peer to begin exchanging routes.",
	}
}

// HandleBGPPeersPage renders the BGP peers table within the workbench.
func HandleBGPPeersPage(renderer *Renderer, viewTree *config.Tree, filterGroup string) template.HTML {
	peers := collectPeers(viewTree)
	tableData := BuildBGPPeersTableData(peers, filterGroup)
	return renderer.RenderFragment("workbench_table", tableData)
}

// BuildBGPPeerDetailData constructs a WorkbenchDetailData for a single peer.
func BuildBGPPeerDetailData(pe peerEntry) WorkbenchDetailData {
	configHTML := buildPeerConfigHTML(pe)
	statusHTML := buildPeerStatusHTML(pe)
	actionsHTML := buildPeerActionsHTML(pe)

	tabs := []WorkbenchDetailTab{
		{Key: "config", Label: "Config", Content: configHTML, Active: true},
		{Key: "status", Label: "Status", Content: statusHTML},
		{Key: "actions", Label: "Actions", Content: actionsHTML},
	}

	closeURL := "/show/bgp/peer/"
	if pe.Group != "" {
		closeURL = "/show/bgp/group/" + pe.Group + "/peer/"
	}

	return WorkbenchDetailData{
		Title:    pe.Name,
		Tabs:     tabs,
		CloseURL: closeURL,
	}
}

func buildPeerConfigHTML(pe peerEntry) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="wb-detail-section">`)
	b.WriteString(`<table class="wb-detail-kv">`)
	writeKV(&b, "Name", pe.Name)
	writeKV(&b, "Remote IP", valueOrDash(pe.RemoteIP))
	writeKV(&b, "Remote AS", valueOrDash(pe.RemoteAS))
	writeKV(&b, "Local AS", valueOrDash(pe.LocalAS))
	writeKV(&b, "Group", valueOrDash(pe.Group))
	writeKV(&b, "Families", valueOrDash(pe.Families))
	b.WriteString(`</table>`)
	b.WriteString(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

func buildPeerStatusHTML(pe peerEntry) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="wb-detail-section">`)
	b.WriteString(`<table class="wb-detail-kv">`)
	// Operational state placeholder: future spec will populate from reactor.
	state := peerStateConfigured
	if pe.Disabled {
		state = peerStateDisabled
	}
	writeKV(&b, "State", state)
	writeKV(&b, "Uptime", "--")
	writeKV(&b, "Prefixes Received", "--")
	writeKV(&b, "Messages In", "--")
	writeKV(&b, "Messages Out", "--")
	writeKV(&b, "Last Error", "--")
	b.WriteString(`</table>`)
	b.WriteString(`<p class="wb-detail-hint">Operational data requires a running BGP engine.</p>`)
	b.WriteString(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

func buildPeerActionsHTML(pe peerEntry) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="wb-detail-section">`)
	b.WriteString(`<div class="wb-detail-actions">`)
	if pe.RemoteIP != "" {
		contextPath := "bgp/peer/" + pe.Name
		if pe.Group != "" {
			contextPath = "bgp/group/" + pe.Group + "/peer/" + pe.Name
		}
		fmt.Fprintf(&b,
			`<button class="wb-detail-tool" hx-post="/tools/related/run" hx-vals='{"tool_id":"peer-flush","context_path":"%s"}' type="button">Flush</button>`,
			template.HTMLEscapeString(contextPath))
		fmt.Fprintf(&b,
			`<button class="wb-detail-tool wb-detail-tool--danger" hx-post="/tools/related/run" hx-vals='{"tool_id":"peer-teardown","context_path":"%s"}' hx-confirm="Tear down BGP session with %s?" type="button">Teardown</button>`,
			template.HTMLEscapeString(contextPath),
			template.HTMLEscapeString(pe.Name))
	}
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

// valueOrDash returns v if non-empty, "-" otherwise.
func valueOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
