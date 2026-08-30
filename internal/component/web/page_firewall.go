// Design: docs/architecture/web-workbench-pages.md -- Firewall workbench pages
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Sibling page (pattern reference)

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// --- Tables page ---

// tableEntry holds display fields for one firewall table.
type tableEntry struct {
	Name       string
	Family     string
	ChainCount int
	SetCount   int
}

// collectTables reads applied firewall tables and converts them to display entries.
func collectTables() []tableEntry {
	tables := firewall.LastApplied()
	if len(tables) == 0 {
		return nil
	}

	entries := make([]tableEntry, 0, len(tables))
	for _, t := range tables {
		entries = append(entries, tableEntry{
			Name:       firewall.StripZeTablePrefix(t.Name),
			Family:     t.Family.String(),
			ChainCount: len(t.Chains),
			SetCount:   len(t.Sets),
		})
	}
	return entries
}

// buildFirewallTablesTableData constructs a WorkbenchTableData for the tables page.
func buildFirewallTablesTableData(entries []tableEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: colName, Label: labelName, Sortable: true},
		{Key: segFamily, Label: labelFamily, Sortable: true},
		{Key: "chains", Label: "Chains", Sortable: true},
		{Key: "sets", Label: "Sets", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	var tb textbuf.Buffer
	for _, e := range entries {
		rows = append(rows, WorkbenchTableRow{
			Key: e.Name,
			URL: tb.Reset().Str("/show/firewall/chain/?table=").Str(e.Name).String(),
			Cells: []string{
				e.Name,
				e.Family,
				strconv.Itoa(e.ChainCount),
				strconv.Itoa(e.SetCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Chains", URL: tb.Reset().Str("/show/firewall/chain/?table=").Str(e.Name).String()},
				{Label: labelEdit, URL: tb.Reset().Str("/show/firewall/table/").Str(e.Name).Byte('/').String()},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "Firewall Tables",
		AddURL:       "/show/firewall/table/",
		AddLabel:     "Add Table",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No firewall tables configured.",
		EmptyHint:    "Create a table to start defining packet filtering rules.",
	}
}

// handleFirewallTablesPage renders the firewall tables table within the workbench.
func handleFirewallTablesPage(renderer *Renderer) template.HTML {
	entries := collectTables()
	tableData := buildFirewallTablesTableData(entries)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

// --- Chains page ---

// chainEntry holds display fields for one firewall chain.
type chainEntry struct {
	Table     string
	Name      string
	IsBase    bool
	Type      string
	Hook      string
	Priority  int32
	Policy    string
	RuleCount int
}

// collectChains reads applied firewall tables and flattens chains.
// filterTable restricts to chains in a specific table.
// filterHook restricts to chains with a specific hook point.
// filterType restricts to chains with a specific chain type.
func collectChains(filterTable, filterHook, filterType string) []chainEntry {
	tables := firewall.LastApplied()
	if len(tables) == 0 {
		return nil
	}

	var entries []chainEntry
	for _, t := range tables {
		tableName := firewall.StripZeTablePrefix(t.Name)
		if filterTable != "" && tableName != filterTable {
			continue
		}

		for _, c := range t.Chains {
			if filterHook != "" && c.Hook.String() != filterHook {
				continue
			}
			if filterType != "" && c.Type.String() != filterType {
				continue
			}

			ce := chainEntry{
				Table:     tableName,
				Name:      c.Name,
				IsBase:    c.IsBase,
				RuleCount: len(c.Terms),
			}
			if c.IsBase {
				ce.Type = c.Type.String()
				ce.Hook = c.Hook.String()
				ce.Priority = c.Priority
				ce.Policy = c.Policy.String()
			}

			entries = append(entries, ce)
		}
	}
	return entries
}

// buildFirewallChainsTableData constructs a WorkbenchTableData for the chains page.
func buildFirewallChainsTableData(entries []chainEntry, filterTable string) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "table", Label: "Table", Sortable: true},
		{Key: colName, Label: labelName, Sortable: true},
		{Key: colType, Label: labelType, Sortable: true},
		{Key: "hook", Label: "Hook", Sortable: true},
		{Key: "priority", Label: "Priority", Sortable: true},
		{Key: "policy", Label: "Policy", Sortable: true},
		{Key: segRules, Label: "Rule Count", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	var tb textbuf.Buffer
	for _, ce := range entries {
		priorityStr := "-"
		if ce.IsBase {
			priorityStr = strconv.Itoa(int(ce.Priority))
		}

		rows = append(rows, WorkbenchTableRow{
			Key: tb.Reset().Str(ce.Table).Byte('/').Str(ce.Name).String(),
			URL: tb.Reset().Str("/show/firewall/rule/?table=").Str(ce.Table).Str("&chain=").Str(ce.Name).String(),
			Cells: []string{
				ce.Table,
				ce.Name,
				valueOrDash(ce.Type),
				valueOrDash(ce.Hook),
				priorityStr,
				valueOrDash(ce.Policy),
				strconv.Itoa(ce.RuleCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Rules", URL: tb.Reset().Str("/show/firewall/rule/?table=").Str(ce.Table).Str("&chain=").Str(ce.Name).String()},
				{Label: labelEdit, URL: tb.Reset().Str("/show/firewall/table/").Str(ce.Table).Str("/chain/").Str(ce.Name).Byte('/').String()},
			},
		})
	}

	emptyMsg := "No chains configured."
	if filterTable != "" {
		emptyMsg = tb.Reset().Str("No chains configured in table ").Str(strconv.Quote(filterTable)).Byte('.').String()
	}

	addURL := ""
	if filterTable != "" {
		addURL = tb.Reset().Str("/show/firewall/table/").Str(filterTable).Str("/chain/").String()
	}

	return WorkbenchTableData{
		Title:        "Firewall Chains",
		AddURL:       addURL,
		AddLabel:     "Add Chain",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: emptyMsg,
		EmptyHint:    "Select a firewall table before adding a chain.",
	}
}

// handleFirewallChainsPage renders the firewall chains table within the workbench.
func handleFirewallChainsPage(renderer *Renderer, r *http.Request) template.HTML {
	filterTable := r.URL.Query().Get("table")
	filterHook := r.URL.Query().Get("hook")
	filterType := r.URL.Query().Get("type")
	entries := collectChains(filterTable, filterHook, filterType)
	tableData := buildFirewallChainsTableData(entries, filterTable)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

// --- Rules page ---

// ruleEntry holds display fields for one firewall rule (term).
type ruleEntry struct {
	Order    int
	Name     string
	Table    string
	Chain    string
	Disabled bool
	Match    string
	Action   string
	Comment  string
	Packets  string
	Bytes    string
}

// collectRules reads applied firewall tables and flattens rules.
// filterTable and filterChain restrict the scope.
func collectRules(filterTable, filterChain string) []ruleEntry {
	tables := firewall.LastApplied()
	if len(tables) == 0 {
		return nil
	}

	// Collect counters if a backend is loaded.
	counterMap := make(map[string]map[string]firewall.TermCounter) // table -> term -> counter
	if backend := firewall.GetBackend(); backend != nil {
		for _, t := range tables {
			tableName := firewall.StripZeTablePrefix(t.Name)
			chainCounters, err := backend.GetCounters(t.Name)
			if err == nil {
				for _, cc := range chainCounters {
					for _, tc := range cc.Terms {
						if counterMap[tableName] == nil {
							counterMap[tableName] = make(map[string]firewall.TermCounter)
						}
						counterMap[tableName][tc.Name] = tc
					}
				}
			}
		}
	}

	var entries []ruleEntry
	for _, t := range tables {
		tableName := firewall.StripZeTablePrefix(t.Name)
		if filterTable != "" && tableName != filterTable {
			continue
		}

		for _, c := range t.Chains {
			if filterChain != "" && c.Name != filterChain {
				continue
			}

			for i, term := range c.Terms {
				re := ruleEntry{
					Order:   i + 1,
					Name:    term.Name,
					Table:   tableName,
					Chain:   c.Name,
					Match:   matchSummary(term.Matches),
					Action:  actionSummary(term.Actions),
					Packets: "-",
					Bytes:   "-",
				}

				// Look up counters for this term.
				if tableCounters, ok := counterMap[tableName]; ok {
					if tc, ok := tableCounters[term.Name]; ok {
						re.Packets = strconv.Itoa(int(tc.Packets))
						re.Bytes = strconv.Itoa(int(tc.Bytes))
					}
				}

				entries = append(entries, re)
			}
		}
	}
	return entries
}

// matchSummary converts a slice of Match values into a human-readable string.
func matchSummary(matches []firewall.Match) string {
	if len(matches) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		switch v := m.(type) {
		case firewall.MatchProtocol:
			parts = append(parts, v.Protocol)
		case firewall.MatchSourceAddress:
			parts = append(parts, "saddr "+v.Prefix.String())
		case firewall.MatchDestinationAddress:
			parts = append(parts, "daddr "+v.Prefix.String())
		case firewall.MatchSourcePort:
			parts = append(parts, "sport "+formatPortRanges(v.Ranges))
		case firewall.MatchDestinationPort:
			parts = append(parts, "dport "+formatPortRanges(v.Ranges))
		case firewall.MatchInputInterface:
			name := v.Name
			if v.Wildcard {
				name += "*"
			}
			parts = append(parts, "iif "+name)
		case firewall.MatchOutputInterface:
			name := v.Name
			if v.Wildcard {
				name += "*"
			}
			parts = append(parts, "oif "+name)
		case firewall.MatchConnState:
			parts = append(parts, "ct state "+connStateStr(v.States))
		case firewall.MatchConnMark:
			parts = append(parts, fmt.Sprintf("ct mark 0x%x/0x%x", v.Value, v.Mask))
		case firewall.MatchMark:
			parts = append(parts, fmt.Sprintf("mark 0x%x/0x%x", v.Value, v.Mask))
		case firewall.MatchDSCP:
			var bDscp textbuf.Buffer
			parts = append(parts, bDscp.Reset().Str("dscp ").Int(int64(v.Value)).String())
		case firewall.MatchICMPType:
			var bIcmp textbuf.Buffer
			parts = append(parts, bIcmp.Reset().Str("icmp type ").Int(int64(v.Type)).String())
		case firewall.MatchICMPv6Type:
			var bIcmp6 textbuf.Buffer
			parts = append(parts, bIcmp6.Reset().Str("icmpv6 type ").Int(int64(v.Type)).String())
		case firewall.MatchInSet:
			var bSet textbuf.Buffer
			parts = append(parts, bSet.Str("in set ").Str(v.SetName).String())
		case firewall.MatchTCPFlags:
			parts = append(parts, fmt.Sprintf("tcp flags 0x%x/0x%x", v.Flags, v.Mask))
		default:
			parts = append(parts, fmt.Sprintf("%T", m))
		}
	}
	return textbuf.Join(parts, " ")
}

// formatPortRanges formats port ranges as a comma-separated string.
func formatPortRanges(ranges []firewall.PortRange) string {
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.Lo == r.Hi {
			parts = append(parts, strconv.Itoa(int(r.Lo)))
		} else {
			var bRange textbuf.Buffer
			parts = append(parts, bRange.Reset().Int(int64(r.Lo)).Byte('-').Int(int64(r.Hi)).String())
		}
	}
	return textbuf.Join(parts, ",")
}

// connStateStr converts a ConnState bitmask to a human-readable string.
func connStateStr(s firewall.ConnState) string {
	var parts []string
	if s&firewall.ConnStateNew != 0 {
		parts = append(parts, "new")
	}
	if s&firewall.ConnStateEstablished != 0 {
		parts = append(parts, "established")
	}
	if s&firewall.ConnStateRelated != 0 {
		parts = append(parts, "related")
	}
	if s&firewall.ConnStateInvalid != 0 {
		parts = append(parts, "invalid")
	}
	if len(parts) == 0 {
		return "none"
	}
	return textbuf.Join(parts, ",")
}

// actionSummary extracts the terminal action name from an Action slice.
func actionSummary(actions []firewall.Action) string {
	if len(actions) == 0 {
		return "-"
	}

	var parts []string
	for _, a := range actions {
		switch v := a.(type) {
		case firewall.Accept:
			parts = append(parts, "accept")
		case firewall.Drop:
			parts = append(parts, "drop")
		case firewall.Reject:
			if v.Type != "" {
				parts = append(parts, "reject ("+v.Type+")")
			} else {
				parts = append(parts, "reject")
			}
		case firewall.Jump:
			parts = append(parts, "jump "+v.Target)
		case firewall.Goto:
			parts = append(parts, "goto "+v.Target)
		case firewall.Return:
			parts = append(parts, "return")
		case firewall.SNAT:
			parts = append(parts, "snat "+v.Address.String())
		case firewall.DNAT:
			parts = append(parts, "dnat "+v.Address.String())
		case firewall.Masquerade:
			parts = append(parts, "masquerade")
		case firewall.Redirect:
			var bRedir textbuf.Buffer
			parts = append(parts, bRedir.Reset().Str("redirect :").Int(int64(v.Port)).String())
		case firewall.Notrack:
			parts = append(parts, "notrack")
		case firewall.FlowOffload:
			parts = append(parts, "flow offload "+v.FlowtableName)
		case firewall.Counter:
			parts = append(parts, "counter")
		case firewall.Log:
			if v.Prefix != "" {
				parts = append(parts, "log "+v.Prefix)
			} else {
				parts = append(parts, "log")
			}
		case firewall.SetMark:
			parts = append(parts, fmt.Sprintf("mark 0x%x", v.Value))
		case firewall.SetConnMark:
			parts = append(parts, fmt.Sprintf("ct mark 0x%x", v.Value))
		case firewall.SetDSCP:
			var bDscp2 textbuf.Buffer
			parts = append(parts, bDscp2.Reset().Str("dscp ").Int(int64(v.Value)).String())
		case firewall.SetTCPMSS:
			var bMss textbuf.Buffer
			parts = append(parts, bMss.Reset().Str("tcp-mss ").Int(int64(v.Size)).String())
		case firewall.Limit:
			var bLim textbuf.Buffer
			parts = append(parts, bLim.Reset().Str("limit ").Int(int64(v.Rate)).Byte('/').Str(v.Unit).String())
		default:
			parts = append(parts, fmt.Sprintf("%T", a))
		}
	}
	return textbuf.Join(parts, " ")
}

// buildFirewallRulesTableData constructs a WorkbenchTableData for the rules page.
func buildFirewallRulesTableData(entries []ruleEntry, filterTable, filterChain string) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "order", Label: "#"},
		{Key: "flags", Label: "Flags"},
		{Key: "chain", Label: "Chain", Sortable: true},
		{Key: "match", Label: "Match"},
		{Key: "action", Label: "Action"},
		{Key: "packets", Label: "Packets"},
		{Key: "bytes", Label: "Bytes"},
		{Key: "comment", Label: "Comment"},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	for _, re := range entries {
		flagStr := ""
		flagClass := ""
		if re.Disabled {
			flagStr = "X"
			flagClass = flagClassGrey
		}

		var tb textbuf.Buffer
		ruleBase := tb.Str("/show/firewall/table/").Str(re.Table).Str("/chain/").Str(re.Chain).Str("/rule/").Str(re.Name).String()
		rows = append(rows, WorkbenchTableRow{
			Key:       tb.Reset().Str(re.Table).Byte('/').Str(re.Chain).Byte('/').Str(re.Name).String(),
			Flags:     flagStr,
			FlagClass: flagClass,
			Cells: []string{
				strconv.Itoa(re.Order),
				flagStr,
				re.Chain,
				re.Match,
				re.Action,
				re.Packets,
				re.Bytes,
				valueOrDash(re.Comment),
			},
			Actions: []WorkbenchRowAction{
				{Label: labelEdit, URL: tb.Reset().Str(ruleBase).Byte('/').String()},
				{Label: "Toggle", HxPost: tb.Reset().Str(ruleBase).Str("/toggle").String(),
					Confirm: tb.Reset().Str("Toggle rule ").Str(strconv.Quote(re.Name)).Byte('?').String()},
				{Label: "Move Up", HxPost: tb.Reset().Str(ruleBase).Str("/move-up").String()},
				{Label: "Move Down", HxPost: tb.Reset().Str(ruleBase).Str("/move-down").String()},
				{Label: "Clone", HxPost: tb.Reset().Str(ruleBase).Str("/clone").String()},
				{Label: "Delete", HxPost: tb.Reset().Str(ruleBase).Str("/delete").String(),
					Class: wbToolDanger, Confirm: tb.Reset().Str("Delete rule ").Str(strconv.Quote(re.Name)).Byte('?').String()},
			},
		})
	}

	emptyMsg := "No rules configured."
	var tb2 textbuf.Buffer
	if filterChain != "" {
		emptyMsg = tb2.Str("No rules in chain ").Str(strconv.Quote(filterChain)).Byte('.').String()
	}
	emptyHint := "Add a rule to start filtering traffic."
	if filterChain != "" {
		emptyHint = tb2.Reset().Str("Chain ").Str(strconv.Quote(filterChain)).Str(" has no rules. Traffic follows the chain's default policy.").String()
	}

	addURL := ""
	if filterTable != "" && filterChain != "" {
		addURL = tb2.Reset().Str("/show/firewall/table/").Str(filterTable).Str("/chain/").Str(filterChain).Str("/term/").String()
	}

	return WorkbenchTableData{
		Title:        "Firewall Rules",
		AddURL:       addURL,
		AddLabel:     "Add Rule",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: emptyMsg,
		EmptyHint:    emptyHint,
	}
}

// handleFirewallRulesPage renders the firewall rules table within the workbench.
func handleFirewallRulesPage(renderer *Renderer, r *http.Request) template.HTML {
	filterTable := r.URL.Query().Get("table")
	filterChain := r.URL.Query().Get("chain")
	entries := collectRules(filterTable, filterChain)
	tableData := buildFirewallRulesTableData(entries, filterTable, filterChain)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

// --- Sets page ---

// setEntry holds display fields for one firewall set.
type setEntry struct {
	Table        string
	Name         string
	Type         string
	Flags        string
	ElementCount int
}

// collectSets reads applied firewall tables and flattens sets.
func collectSets(filterTable string) []setEntry {
	tables := firewall.LastApplied()
	if len(tables) == 0 {
		return nil
	}

	var entries []setEntry
	for _, t := range tables {
		tableName := firewall.StripZeTablePrefix(t.Name)
		if filterTable != "" && tableName != filterTable {
			continue
		}

		for _, s := range t.Sets {
			entries = append(entries, setEntry{
				Table:        tableName,
				Name:         s.Name,
				Type:         s.Type.String(),
				Flags:        setFlagsStr(s.Flags),
				ElementCount: len(s.Elements),
			})
		}
	}
	return entries
}

// setFlagsStr converts a SetFlags bitmask to a human-readable string.
func setFlagsStr(f firewall.SetFlags) string {
	var parts []string
	if f&firewall.SetFlagInterval != 0 {
		parts = append(parts, "interval")
	}
	if f&firewall.SetFlagTimeout != 0 {
		parts = append(parts, "timeout")
	}
	if f&firewall.SetFlagConstant != 0 {
		parts = append(parts, "constant")
	}
	if f&firewall.SetFlagDynamic != 0 {
		parts = append(parts, "dynamic")
	}
	if len(parts) == 0 {
		return "-"
	}
	return textbuf.Join(parts, ", ")
}

// buildFirewallSetsTableData constructs a WorkbenchTableData for the sets page.
func buildFirewallSetsTableData(entries []setEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "table", Label: "Table", Sortable: true},
		{Key: colName, Label: labelName, Sortable: true},
		{Key: colType, Label: labelType, Sortable: true},
		{Key: "flags", Label: "Flags"},
		{Key: "elements", Label: "Elements", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	var tb textbuf.Buffer
	for _, se := range entries {
		rows = append(rows, WorkbenchTableRow{
			Key: tb.Reset().Str(se.Table).Byte('/').Str(se.Name).String(),
			Cells: []string{
				se.Table,
				se.Name,
				se.Type,
				se.Flags,
				strconv.Itoa(se.ElementCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Elements", URL: tb.Reset().Str("/show/firewall/table/").Str(se.Table).Str("/set/").Str(se.Name).Byte('/').String()},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "Firewall Sets",
		AddURL:       "",
		AddLabel:     "Add Set",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No named sets.",
		EmptyHint:    "Named sets allow grouping addresses, ports, or interfaces for use in rules.",
	}
}

// handleFirewallSetsPage renders the firewall sets table within the workbench.
func handleFirewallSetsPage(renderer *Renderer, r *http.Request) template.HTML {
	filterTable := r.URL.Query().Get("table")
	entries := collectSets(filterTable)
	tableData := buildFirewallSetsTableData(entries)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

// --- Connections page ---

// buildFirewallConnectionsTableData constructs a WorkbenchTableData for the
// connections (conntrack) page. For v1, conntrack data requires runtime command
// dispatch, so this shows a placeholder empty state.
func buildFirewallConnectionsTableData() WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "protocol", Label: labelProtocol, Sortable: true},
		{Key: "source", Label: "Source", Sortable: true},
		{Key: "destination", Label: "Destination", Sortable: true},
		{Key: colState, Label: labelState, Sortable: true},
		{Key: "timeout", Label: "Timeout", Sortable: true},
		{Key: "packets", Label: "Packets", Sortable: true},
		{Key: "bytes", Label: "Bytes", Sortable: true},
	}

	emptyMsg := "Connection tracking data requires a running firewall."
	if firewall.ActiveBackendName() != "" {
		emptyMsg = "No active connections."
	}

	return WorkbenchTableData{
		Title:        "Firewall Connections",
		Columns:      columns,
		Rows:         nil,
		EmptyMessage: emptyMsg,
		EmptyHint:    "Conntrack entries will appear here when the firewall backend is active and tracking connections.",
	}
}

// handleFirewallConnectionsPage renders the firewall connections table within the workbench.
func handleFirewallConnectionsPage(renderer *Renderer) template.HTML {
	tableData := buildFirewallConnectionsTableData()
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

// --- Dispatch ---

// renderFirewallPageContent dispatches firewall sub-pages. The path slice has
// the leading "firewall" segment already stripped. Returns (content, true) if
// a page handler matched, or ("", false) to fall through to generic YANG.
func renderFirewallPageContent(renderer *Renderer, r *http.Request, path []string) (template.HTML, bool) {
	// /show/firewall/ (no sub-path or empty) defaults to tables.
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		return handleFirewallTablesPage(renderer), true
	}

	switch path[0] {
	case "chain":
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return handleFirewallChainsPage(renderer, r), true
		}
	case "rule":
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return handleFirewallRulesPage(renderer, r), true
		}
	case "set":
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return handleFirewallSetsPage(renderer, r), true
		}
	case "connections":
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return handleFirewallConnectionsPage(renderer), true
		}
	}

	return "", false
}
