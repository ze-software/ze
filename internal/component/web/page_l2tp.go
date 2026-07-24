// Design: plan/learned/691-web-7-system-services.md -- L2TP workbench pages
// Related: handler_l2tp.go -- Existing L2TP handlers (preserved, not modified)
// Related: workbench_table.go -- Table component
// Related: workbench_form.go -- Form component

//go:build ze_l2tp

package web

import (
	"html/template"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// --- L2TP > Sessions ---

// BuildL2TPSessionsTableData constructs a WorkbenchTableData for the L2TP
// sessions page. Data comes from the live L2TP service snapshot. When the
// L2TP subsystem is not running, the table shows an empty state.
func BuildL2TPSessionsTableData() WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "tunnel-id", Label: "Tunnel ID", Sortable: true},
		{Key: "session-id", Label: "Session ID", Sortable: true},
		{Key: "username", Label: "Username", Sortable: true},
		{Key: "peer", Label: "Peer", Sortable: true},
		{Key: "state", Label: "State", Sortable: true},
		{Key: "interface", Label: "Interface"},
	}

	svc := l2tp.LookupService()
	if svc == nil {
		return WorkbenchTableData{
			Title:        "L2TP Sessions",
			Columns:      columns,
			Rows:         nil,
			EmptyMessage: "L2TP subsystem is not running.",
			EmptyHint:    "Enable the L2TP subsystem in the configuration to manage sessions.",
		}
	}

	snap := svc.Snapshot()

	rows := make([]WorkbenchTableRow, 0, snap.SessionCount)
	for i := range snap.Tunnels {
		t := &snap.Tunnels[i]
		for j := range t.Sessions {
			s := &t.Sessions[j]
			sidStr := strconv.Itoa(int(s.LocalSID))
			var tb textbuf.Buffer
			sessionURL := tb.Str("/l2tp/").Str(sidStr).String()
			rows = append(rows, WorkbenchTableRow{
				Key: tb.Reset().Int(int64(t.LocalTID)).Byte('/').Str(sidStr).String(),
				URL: sessionURL,
				Cells: []string{
					strconv.Itoa(int(t.LocalTID)),
					sidStr,
					s.Username,
					t.PeerAddr.String(),
					s.State,
					s.PppInterface,
				},
				Actions: []WorkbenchRowAction{
					{Label: "Detail", URL: sessionURL},
					{
						Label:   "Disconnect",
						HxPost:  tb.Reset().Str(sessionURL).Str("/disconnect").String(),
						Class:   "danger",
						Confirm: tb.Reset().Str("Disconnect session ").Str(sidStr).Str(" (user ").Str(s.Username).Str(")?").String(),
					},
				},
			})
		}
	}

	return WorkbenchTableData{
		Title:        "L2TP Sessions",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No active L2TP sessions.",
		EmptyHint:    "Sessions appear here when subscribers connect via L2TP tunnels.",
	}
}

// HandleL2TPSessionsPage renders the L2TP Sessions table for the workbench.
// The outer div polls every 5 seconds so session state updates automatically.
func HandleL2TPSessionsPage(renderer *Renderer) template.HTML {
	tableData := BuildL2TPSessionsTableData()
	table := renderer.RenderFragment("workbench_table", tableData)
	return template.HTML(`<div hx-get="/show/l2tp/sessions/" hx-trigger="every 5s" hx-swap="innerHTML">`) + //nolint:gosec // trusted builder output
		table +
		template.HTML(`</div>`) //nolint:gosec // trusted builder output
}

// --- L2TP > Configuration ---

// BuildL2TPConfigFormData constructs a WorkbenchFormData for the L2TP config.
// Fields match l2tp{} and environment/l2tp in ze-l2tp-conf.yang. The
// shared-secret field uses password type for masking.
func BuildL2TPConfigFormData(tree *config.Tree) WorkbenchFormData {
	return WorkbenchFormData{
		Title: "L2TP Configuration",
		Fields: []WorkbenchFormField{
			{
				Name:        "enabled",
				Label:       "Enabled",
				Type:        "toggle",
				Value:       getConfigValue(tree, "l2tp/enabled"),
				Description: "Enable L2TP subsystem",
			},
			{
				Name:        "max-tunnels",
				Label:       "Max Tunnels",
				Type:        "number",
				Value:       getConfigValue(tree, "l2tp/max-tunnels"),
				Description: "Maximum concurrent L2TP tunnels (0 = unlimited)",
			},
			{
				Name:        "max-sessions",
				Label:       "Max Sessions Per Tunnel",
				Type:        "number",
				Value:       getConfigValue(tree, "l2tp/max-sessions"),
				Description: "Maximum concurrent sessions per tunnel (0 = unlimited)",
			},
			{
				Name:        "shared-secret",
				Label:       "Shared Secret",
				Type:        "password",
				Value:       getConfigValue(tree, "l2tp/shared-secret"),
				Description: "CHAP-MD5 shared secret (sensitive)",
			},
			{
				Name:        "hello-interval",
				Label:       "Hello Interval (seconds)",
				Type:        "number",
				Value:       getConfigValue(tree, "l2tp/hello-interval"),
				Description: "Seconds of peer silence before sending HELLO (1-3600)",
			},
			{
				Name:        "hello-retries",
				Label:       "Hello Retries (dead-peer threshold)",
				Type:        "number",
				Value:       getConfigValue(tree, "l2tp/hello-retries"),
				Description: "Unanswered HELLO intervals before a peer is declared dead (0 disables; detection = retries x interval)",
			},
			{
				Name:        "cqm-enabled",
				Label:       "CQM Enabled",
				Type:        "toggle",
				Value:       getConfigValue(tree, "l2tp/cqm-enabled"),
				Description: "Enable Customer Quality Monitor observer",
			},
			{
				Name:        "max-logins",
				Label:       "Max Logins",
				Type:        "number",
				Value:       getConfigValue(tree, "l2tp/max-logins"),
				Description: "Maximum concurrent PPP logins tracked by CQM (1-1000000)",
			},
			{
				Name:        "servers",
				Label:       "Listen Endpoints",
				Type:        "list",
				Items:       getConfigListItems(tree, "environment/l2tp", "server"),
				Description: "L2TP server listen endpoints (UDP)",
			},
		},
		SaveURL:    "/config/form/l2tp/",
		DiscardURL: "/show/l2tp/",
	}
}

// HandleL2TPConfigPage renders the L2TP Configuration form for the workbench.
func HandleL2TPConfigPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := BuildL2TPConfigFormData(viewTree)
	return renderer.RenderFragment("workbench_form", formData)
}

// --- L2TP > Health ---

// BuildL2TPHealthTableData constructs a WorkbenchTableData for the L2TP
// health page. Without a running L2TP subsystem, this shows an empty state.
// With live data, sessions are listed with their current state.
func BuildL2TPHealthTableData() WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "session", Label: "Session", Sortable: true},
		{Key: "username", Label: "Username", Sortable: true},
		{Key: "peer", Label: "Peer", Sortable: true},
		{Key: "state", Label: "State", Sortable: true},
		{Key: "interface", Label: "Interface"},
	}

	svc := l2tp.LookupService()
	if svc == nil {
		return WorkbenchTableData{
			Title:        "L2TP Health",
			Columns:      columns,
			Rows:         nil,
			EmptyMessage: "L2TP subsystem is not running.",
			EmptyHint:    "Enable the L2TP subsystem to monitor session health.",
		}
	}

	snap := svc.Snapshot()

	rows := make([]WorkbenchTableRow, 0, snap.SessionCount)
	for i := range snap.Tunnels {
		t := &snap.Tunnels[i]
		for j := range t.Sessions {
			s := &t.Sessions[j]
			rows = append(rows, WorkbenchTableRow{
				Key: strconv.Itoa(int(s.LocalSID)),
				Cells: []string{
					strconv.Itoa(int(s.LocalSID)),
					s.Username,
					t.PeerAddr.String(),
					s.State,
					s.PppInterface,
				},
			})
		}
	}

	return WorkbenchTableData{
		Title:        "L2TP Health",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No active L2TP sessions.",
		EmptyHint:    "Session health metrics appear here when subscribers are connected.",
	}
}

// HandleL2TPHealthPage renders the L2TP Health table for the workbench.
func HandleL2TPHealthPage(renderer *Renderer) template.HTML {
	tableData := BuildL2TPHealthTableData()
	return renderer.RenderFragment("workbench_table", tableData)
}

// --- Dispatch ---

// renderL2TPPageContent dispatches L2TP sub-pages. The path slice has the
// leading "l2tp" segment already stripped. Returns (content, true) if a page
// handler matched, or ("", false) to fall through to generic YANG.
func renderL2TPPageContent(renderer *Renderer, path []string, viewTree *config.Tree) (template.HTML, bool) {
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		// /show/l2tp/ defaults to configuration.
		return HandleL2TPConfigPage(renderer, viewTree), true
	}

	switch path[0] {
	case "sessions":
		return HandleL2TPSessionsPage(renderer), true
	case "health":
		return HandleL2TPHealthPage(renderer), true
	}

	return "", false
}
