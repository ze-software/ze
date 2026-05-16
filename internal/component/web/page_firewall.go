// Design: plan/spec-web-6-firewall.md -- Firewall workbench pages
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Sibling page (pattern reference)

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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

// BuildFirewallTablesTableData constructs a WorkbenchTableData for the tables page.
func BuildFirewallTablesTableData(entries []tableEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "family", Label: "Family", Sortable: true},
		{Key: "chains", Label: "Chains", Sortable: true},
		{Key: "sets", Label: "Sets", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, WorkbenchTableRow{
			Key: e.Name,
			URL: "/show/firewall/chain/?table=" + e.Name,
			Cells: []string{
				e.Name,
				e.Family,
				strconv.Itoa(e.ChainCount),
				strconv.Itoa(e.SetCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Chains", URL: "/show/firewall/chain/?table=" + e.Name},
				{Label: "Edit", URL: "/show/firewall/table/" + e.Name + "/"},
				{Label: "Delete", HxPost: "/show/firewall/table/" + e.Name + "/delete",
					Class: "danger", Confirm: "Delete table " + strconv.Quote(e.Name) + " and all its chains, rules, and sets?"},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "Firewall Tables",
		AddURL:       "/show/firewall/table/add",
		AddLabel:     "Add Table",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No firewall tables configured.",
		EmptyHint:    "Create a table to start defining packet filtering rules.",
	}
}

// HandleFirewallTablesPage renders the firewall tables table within the workbench.
func HandleFirewallTablesPage(renderer *Renderer) template.HTML {
	entries := collectTables()
	tableData := BuildFirewallTablesTableData(entries)
	return renderer.RenderFragment("workbench_table", tableData)
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

// BuildFirewallChainsTableData constructs a WorkbenchTableData for the chains page.
func BuildFirewallChainsTableData(entries []chainEntry, filterTable string) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "table", Label: "Table", Sortable: true},
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "hook", Label: "Hook", Sortable: true},
		{Key: "priority", Label: "Priority", Sortable: true},
		{Key: "policy", Label: "Policy", Sortable: true},
		{Key: "rules", Label: "Rule Count", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	for _, ce := range entries {
		priorityStr := "-"
		if ce.IsBase {
			priorityStr = strconv.Itoa(int(ce.Priority))
		}

		rows = append(rows, WorkbenchTableRow{
			Key: ce.Table + "/" + ce.Name,
			URL: "/show/firewall/rule/?table=" + ce.Table + "&chain=" + ce.Name,
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
				{Label: "View Rules", URL: "/show/firewall/rule/?table=" + ce.Table + "&chain=" + ce.Name},
				{Label: "Edit", URL: "/show/firewall/table/" + ce.Table + "/chain/" + ce.Name + "/"},
				{Label: "Delete", HxPost: "/show/firewall/table/" + ce.Table + "/chain/" + ce.Name + "/delete",
					Class: "danger", Confirm: "Delete chain " + strconv.Quote(ce.Name) + " and all its rules?"},
			},
		})
	}

	emptyMsg := "No chains configured."
	if filterTable != "" {
		emptyMsg = "No chains configured in table " + strconv.Quote(filterTable) + "."
	}

	return WorkbenchTableData{
		Title:        "Firewall Chains",
		AddURL:       "/show/firewall/chain/add",
		AddLabel:     "Add Chain",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: emptyMsg,
		EmptyHint:    "Add a chain with a hook point to start filtering traffic.",
	}
}

// HandleFirewallChainsPage renders the firewall chains table within the workbench.
func HandleFirewallChainsPage(renderer *Renderer, r *http.Request) template.HTML {
	filterTable := r.URL.Query().Get("table")
	filterHook := r.URL.Query().Get("hook")
	filterType := r.URL.Query().Get("type")
	entries := collectChains(filterTable, filterHook, filterType)
	tableData := BuildFirewallChainsTableData(entries, filterTable)
	return renderer.RenderFragment("workbench_table", tableData)
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
			parts = append(parts, "in set "+v.SetName)
		case firewall.MatchTCPFlags:
			parts = append(parts, fmt.Sprintf("tcp flags 0x%x/0x%x", v.Flags, v.Mask))
		default:
			parts = append(parts, fmt.Sprintf("%T", m))
		}
	}
	return strings.Join(parts, " ")
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
	return strings.Join(parts, ",")
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
	return strings.Join(parts, ",")
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
	return strings.Join(parts, " ")
}

// BuildFirewallRulesTableData constructs a WorkbenchTableData for the rules page.
func BuildFirewallRulesTableData(entries []ruleEntry, filterTable, filterChain string) WorkbenchTableData {
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

		ruleBase := "/show/firewall/table/" + re.Table + "/chain/" + re.Chain + "/rule/" + re.Name
		rows = append(rows, WorkbenchTableRow{
			Key:       re.Table + "/" + re.Chain + "/" + re.Name,
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
				{Label: "Edit", URL: ruleBase + "/"},
				{Label: "Toggle", HxPost: ruleBase + "/toggle",
					Confirm: "Toggle rule " + strconv.Quote(re.Name) + "?"},
				{Label: "Move Up", HxPost: ruleBase + "/move-up"},
				{Label: "Move Down", HxPost: ruleBase + "/move-down"},
				{Label: "Clone", HxPost: ruleBase + "/clone"},
				{Label: "Delete", HxPost: ruleBase + "/delete",
					Class: "danger", Confirm: "Delete rule " + strconv.Quote(re.Name) + "?"},
			},
		})
	}

	emptyMsg := "No rules configured."
	if filterChain != "" {
		emptyMsg = "No rules in chain " + strconv.Quote(filterChain) + "."
	}
	emptyHint := "Add a rule to start filtering traffic."
	if filterChain != "" {
		emptyHint = "Chain " + strconv.Quote(filterChain) + " has no rules. Traffic follows the chain's default policy."
	}

	addURL := "/show/firewall/rule/add"
	if filterTable != "" && filterChain != "" {
		addURL = "/show/firewall/table/" + filterTable + "/chain/" + filterChain + "/rule/add"
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

// HandleFirewallRulesPage renders the firewall rules table within the workbench.
func HandleFirewallRulesPage(renderer *Renderer, r *http.Request) template.HTML {
	filterTable := r.URL.Query().Get("table")
	filterChain := r.URL.Query().Get("chain")
	entries := collectRules(filterTable, filterChain)
	tableData := BuildFirewallRulesTableData(entries, filterTable, filterChain)
	return renderer.RenderFragment("workbench_table", tableData)
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
	return strings.Join(parts, ", ")
}

// BuildFirewallSetsTableData constructs a WorkbenchTableData for the sets page.
func BuildFirewallSetsTableData(entries []setEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "table", Label: "Table", Sortable: true},
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "flags", Label: "Flags"},
		{Key: "elements", Label: "Elements", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	for _, se := range entries {
		rows = append(rows, WorkbenchTableRow{
			Key: se.Table + "/" + se.Name,
			Cells: []string{
				se.Table,
				se.Name,
				se.Type,
				se.Flags,
				strconv.Itoa(se.ElementCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Elements", URL: "/show/firewall/table/" + se.Table + "/set/" + se.Name + "/"},
				{Label: "Delete", HxPost: "/show/firewall/table/" + se.Table + "/set/" + se.Name + "/delete",
					Class: "danger", Confirm: "Delete set " + strconv.Quote(se.Name) + "?"},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "Firewall Sets",
		AddURL:       "/show/firewall/set/add",
		AddLabel:     "Add Set",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No named sets.",
		EmptyHint:    "Named sets allow grouping addresses, ports, or interfaces for use in rules.",
	}
}

// HandleFirewallSetsPage renders the firewall sets table within the workbench.
func HandleFirewallSetsPage(renderer *Renderer, r *http.Request) template.HTML {
	filterTable := r.URL.Query().Get("table")
	entries := collectSets(filterTable)
	tableData := BuildFirewallSetsTableData(entries)
	return renderer.RenderFragment("workbench_table", tableData)
}

// --- Connections page ---

// BuildFirewallConnectionsTableData constructs a WorkbenchTableData for the
// connections (conntrack) page. For v1, conntrack data requires runtime command
// dispatch, so this shows a placeholder empty state.
func BuildFirewallConnectionsTableData() WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "protocol", Label: "Protocol", Sortable: true},
		{Key: "source", Label: "Source", Sortable: true},
		{Key: "destination", Label: "Destination", Sortable: true},
		{Key: "state", Label: "State", Sortable: true},
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

// HandleFirewallConnectionsPage renders the firewall connections table within the workbench.
func HandleFirewallConnectionsPage(renderer *Renderer) template.HTML {
	tableData := BuildFirewallConnectionsTableData()
	return renderer.RenderFragment("workbench_table", tableData)
}

// --- Dispatch ---

// renderFirewallPageContent dispatches firewall sub-pages. The path slice has
// the leading "firewall" segment already stripped. Returns (content, true) if
// a page handler matched, or ("", false) to fall through to generic YANG.
func renderFirewallPageContent(renderer *Renderer, r *http.Request, path []string) (template.HTML, bool) {
	// /show/firewall/ (no sub-path or empty) defaults to tables.
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		return HandleFirewallTablesPage(renderer), true
	}

	switch path[0] {
	case "chain":
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return HandleFirewallChainsPage(renderer, r), true
		}
	case "rule":
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return HandleFirewallRulesPage(renderer, r), true
		}
	case "set":
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return HandleFirewallSetsPage(renderer, r), true
		}
	case "connections":
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return HandleFirewallConnectionsPage(renderer), true
		}
	}

	return "", false
}
