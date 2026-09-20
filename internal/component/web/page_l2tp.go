// Design: docs/architecture/web-workbench-pages.md -- L2TP workbench pages
// Related: handler_l2tp.go -- Existing L2TP handlers (preserved, not modified)
// Related: workbench_table.go -- Table component
// Related: workbench_form.go -- Form component

//go:build ze_l2tp

package web

import (
	"html/template"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// --- L2TP > Sessions ---

// buildL2TPSessionsTableData constructs a WorkbenchTableData for the L2TP
// sessions page. Data comes from the live L2TP service snapshot. When the
// L2TP subsystem is not running, the table shows an empty state.
func buildL2TPSessionsTableData() WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "tunnel-id", Label: "Tunnel ID", Sortable: true},
		{Key: colSessionID, Label: "Session ID", Sortable: true},
		{Key: colUsername, Label: labelUsername, Sortable: true},
		{Key: colPeer, Label: labelPeer, Sortable: true},
		{Key: colState, Label: labelState, Sortable: true},
		{Key: colInterface, Label: labelInterface},
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
					{Label: labelDetail, URL: sessionURL},
					{
						Label:   "Disconnect",
						HxPost:  tb.Reset().Str(sessionURL).Str("/disconnect").String(),
						Class:   wbToolDanger,
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

// handleL2TPSessionsPage renders the L2TP Sessions table for the workbench.
// The outer div polls every 5 seconds so session state updates automatically.
func handleL2TPSessionsPage(renderer *Renderer) template.HTML {
	tableData := buildL2TPSessionsTableData()

	return renderer.renderComponent("l2tp_sessions_panel",
		pollPanel("/show/l2tp/sessions/", workbenchTable(tableData)))
}

// --- L2TP > Configuration ---

// l2tpConfigLeaves names the leaves the L2TP configuration form edits, in the
// order the form renders them. A row carries no description text: the account
// of a leaf is the one ze-l2tp-conf.yang declares, and the form reads it off
// the loaded schema, so the web page and `ze config` never give an operator two
// accounts of one leaf (ai/rules/principles.md).
var l2tpConfigLeaves = []struct {
	path  string
	name  string
	label string
	kind  string
}{
	{path: "l2tp/enabled", name: wbFormEnabledField, label: labelEnabled, kind: wbFormToggleType},
	{path: "l2tp/max-tunnels", name: "max-tunnels", label: "Max Tunnels", kind: wbFormNumberType},
	{path: "l2tp/max-sessions", name: "max-sessions", label: "Max Sessions Per Tunnel", kind: wbFormNumberType},
	{path: "l2tp/shared-secret", name: "shared-secret", label: "Shared Secret", kind: wbFormPasswordType},
	{path: "l2tp/hello-interval", name: "hello-interval", label: "Hello Interval (seconds)", kind: wbFormNumberType},
	{path: "l2tp/hello-retries", name: "hello-retries", label: "Hello Retries (dead-peer threshold)", kind: wbFormNumberType},
	{path: "l2tp/cqm-enabled", name: "cqm-enabled", label: "CQM Enabled", kind: wbFormToggleType},
	{path: "l2tp/max-logins", name: "max-logins", label: "Max Logins", kind: wbFormNumberType},
}

// The listener list the last form field edits. It is a list rather than a leaf,
// so it declares its own description and the form reads that one.
const (
	l2tpServerContainerPath = "environment/l2tp"
	l2tpServerListName      = "server"
	l2tpServerListPath      = l2tpServerContainerPath + "/" + l2tpServerListName
)

// buildL2TPConfigFormData constructs a WorkbenchFormData for the L2TP config.
// Fields match l2tp{} and environment/l2tp in ze-l2tp-conf.yang. The
// shared-secret field uses password type for masking.
func buildL2TPConfigFormData(tree *config.Tree, schema *config.Schema) WorkbenchFormData {
	fields := make([]WorkbenchFormField, 0, len(l2tpConfigLeaves)+1)
	for _, leaf := range l2tpConfigLeaves {
		fields = append(fields, WorkbenchFormField{
			Name:        leaf.name,
			Label:       leaf.label,
			Type:        leaf.kind,
			Value:       getConfigValue(tree, leaf.path),
			Description: schemaDescription(schema, leaf.path),
		})
	}

	fields = append(fields, WorkbenchFormField{
		Name:        wbFormServersField,
		Label:       labelListenEndpoints,
		Type:        wbFormListType,
		Items:       getConfigListItems(tree, l2tpServerContainerPath, l2tpServerListName),
		Description: schemaDescription(schema, l2tpServerListPath),
	})

	return WorkbenchFormData{
		Title:      "L2TP Configuration",
		Fields:     fields,
		SaveURL:    "/config/form/l2tp/",
		DiscardURL: "/show/l2tp/",
	}
}

// schemaDescription answers the description the YANG schema declares for the
// node at a slash-separated config path.
//
// It answers "" when no schema is loaded and when the path names no node. A
// form hint the schema did not write is a second account of the leaf, which is
// the thing this derivation exists to remove, so an unread schema leaves the
// field with no hint rather than with an invented one (ai/rules/principles.md).
func schemaDescription(schema *config.Schema, path string) string {
	if schema == nil {
		return ""
	}

	node, err := walkSchema(schema, splitConfigPath(path))
	if err != nil {
		return ""
	}

	switch n := node.(type) {
	case *config.LeafNode:
		return n.Description
	case *config.ListNode:
		return n.Description
	case *config.ContainerNode:
		return n.Description
	}

	return ""
}

// handleL2TPConfigPage renders the L2TP Configuration form for the workbench.
func handleL2TPConfigPage(renderer *Renderer, viewTree *config.Tree, schema *config.Schema) template.HTML {
	formData := buildL2TPConfigFormData(viewTree, schema)
	return renderer.renderComponent("workbench_form", workbenchForm(formData))
}

// --- L2TP > Health ---

// buildL2TPHealthTableData constructs a WorkbenchTableData for the L2TP
// health page. Without a running L2TP subsystem, this shows an empty state.
// With live data, sessions are listed with their current state.
func buildL2TPHealthTableData() WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "session", Label: "Session", Sortable: true},
		{Key: colUsername, Label: labelUsername, Sortable: true},
		{Key: colPeer, Label: labelPeer, Sortable: true},
		{Key: colState, Label: labelState, Sortable: true},
		{Key: colInterface, Label: labelInterface},
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

// handleL2TPHealthPage renders the L2TP Health table for the workbench.
func handleL2TPHealthPage(renderer *Renderer) template.HTML {
	tableData := buildL2TPHealthTableData()
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

// --- Dispatch ---

// renderL2TPPageContent dispatches L2TP sub-pages. The path slice has the
// leading "l2tp" segment already stripped. Returns (content, true) if a page
// handler matched, or ("", false) to fall through to generic YANG.
func renderL2TPPageContent(renderer *Renderer, path []string, viewTree *config.Tree, schema *config.Schema) (template.HTML, bool) {
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		// /show/l2tp/ defaults to configuration.
		return handleL2TPConfigPage(renderer, viewTree, schema), true
	}

	switch path[0] {
	case "sessions":
		return handleL2TPSessionsPage(renderer), true
	case segHealth:
		return handleL2TPHealthPage(renderer), true
	}

	return "", false
}
