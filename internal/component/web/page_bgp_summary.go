// Design: plan/learned/689-web-5-bgp.md -- BGP Summary page
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Peer page (sibling)

package web

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// bgpSummaryPeer holds per-peer operational data from "show bgp summary".
type bgpSummaryPeer struct {
	State  string
	Uptime string
	MsgIn  string
	MsgOut string
}

// fetchBGPSummaryPeers dispatches "show bgp summary" and returns a map
// keyed by peer name. Returns nil when dispatch is unavailable or errors.
func fetchBGPSummaryPeers(r *http.Request, dispatch CommandDispatcher) map[string]bgpSummaryPeer {
	if dispatch == nil {
		return nil
	}
	username := GetUsernameFromRequest(r)
	output, err := dispatch.JSON(context.Background(), plugin.CallerIdentity{Username: username, RemoteAddr: r.RemoteAddr}, "show bgp summary")
	if err != nil || output == "" {
		return nil
	}
	var envelope struct {
		Summary struct {
			Peers []struct {
				Name     string `json:"name"`
				State    string `json:"state"`
				Uptime   string `json:"uptime"`
				UpdatesR uint64 `json:"updates-received"`
				UpdatesS uint64 `json:"updates-sent"`
				KAR      uint64 `json:"keepalives-received"`
				KAS      uint64 `json:"keepalives-sent"`
			} `json:"peers"`
		} `json:"summary"`
	}
	if json.Unmarshal([]byte(output), &envelope) != nil {
		return nil
	}
	m := make(map[string]bgpSummaryPeer, len(envelope.Summary.Peers))
	for _, p := range envelope.Summary.Peers {
		m[p.Name] = bgpSummaryPeer{
			State:  p.State,
			Uptime: p.Uptime,
			MsgIn:  textbuf.StringUint(p.UpdatesR + p.KAR),
			MsgOut: textbuf.StringUint(p.UpdatesS + p.KAS),
		}
	}
	return m
}

// BuildBGPSummaryTableData constructs a summary table from the config tree's
// peer listing, enriched with live operational data when available.
func BuildBGPSummaryTableData(viewTree *config.Tree, live map[string]bgpSummaryPeer) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Peer", Sortable: true},
		{Key: "remote-ip", Label: "Remote IP", Sortable: true},
		{Key: "remote-as", Label: "Remote AS", Sortable: true},
		{Key: "state", Label: "State", Sortable: true},
		{Key: "uptime", Label: "Uptime"},
		{Key: "messages-in", Label: "Msg In"},
		{Key: "messages-out", Label: "Msg Out"},
	}

	peers := collectPeers(viewTree)
	rows := make([]WorkbenchTableRow, 0, len(peers))
	for _, pe := range peers {
		state := peerStateConfigured
		uptime := "--"
		msgIn := "--"
		msgOut := "--"

		if pe.Disabled {
			state = peerStateDisabled
		}

		if op, ok := live[pe.Name]; ok {
			state = op.State
			uptime = op.Uptime
			msgIn = op.MsgIn
			msgOut = op.MsgOut
		}

		flags, flagClass := peerFlagFromState(pe, state)

		rows = append(rows, WorkbenchTableRow{
			Key:       pe.Name,
			URL:       pe.EditURL,
			Flags:     flags,
			FlagClass: flagClass,
			Cells: []string{
				pe.Name,
				valueOrDash(pe.RemoteIP),
				valueOrDash(pe.RemoteAS),
				state,
				uptime,
				msgIn,
				msgOut,
			},
		})
	}

	return WorkbenchTableData{
		Title:        "BGP Summary",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No BGP peers configured.",
		EmptyHint:    "Add peers to see the BGP summary.",
	}
}

// HandleBGPSummaryPage renders the BGP summary table within the workbench.
func HandleBGPSummaryPage(renderer *Renderer, r *http.Request, viewTree *config.Tree, dispatch CommandDispatcher) template.HTML {
	live := fetchBGPSummaryPeers(r, dispatch)
	tableData := BuildBGPSummaryTableData(viewTree, live)
	return renderer.RenderFragment("workbench_table", tableData)
}
