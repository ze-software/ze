// Design: plan/spec-web-6-firewall.md -- Firewall workbench pages
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Sibling page (pattern reference)

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
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
			URL: fmt.Sprintf("/show/firewall/chain/?table=%s", e.Name),
			Cells: []string{
				e.Name,
				e.Family,
				fmt.Sprintf("%d", e.ChainCount),
				fmt.Sprintf("%d", e.SetCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Chains", URL: fmt.Sprintf("/show/firewall/chain/?table=%s", e.Name)},
				{Label: "Edit", URL: fmt.Sprintf("/show/firewall/table/%s/", e.Name)},
				{Label: "Delete", HxPost: fmt.Sprintf("/show/firewall/table/%s/delete", e.Name),
					Class: "danger", Confirm: fmt.Sprintf("Delete table %q and all its chains, rules, and sets?", e.Name)},
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
			priorityStr = fmt.Sprintf("%d", ce.Priority)
		}

		rows = append(rows, WorkbenchTableRow{
			Key: ce.Table + "/" + ce.Name,
			URL: fmt.Sprintf("/show/firewall/rule/?table=%s&chain=%s", ce.Table, ce.Name),
			Cells: []string{
				ce.Table,
				ce.Name,
				valueOrDash(ce.Type),
				valueOrDash(ce.Hook),
				priorityStr,
				valueOrDash(ce.Policy),
				fmt.Sprintf("%d", ce.RuleCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Rules", URL: fmt.Sprintf("/show/firewall/rule/?table=%s&chain=%s", ce.Table, ce.Name)},
				{Label: "Edit", URL: fmt.Sprintf("/show/firewall/table/%s/chain/%s/", ce.Table, ce.Name)},
				{Label: "Delete", HxPost: fmt.Sprintf("/show/firewall/table/%s/chain/%s/delete", ce.Table, ce.Name),
					Class: "danger", Confirm: fmt.Sprintf("Delete chain %q and all its rules?", ce.Name)},
			},
		})
	}

	emptyMsg := "No chains configured."
	if filterTable != "" {
		emptyMsg = fmt.Sprintf("No chains configured in table %q.", filterTable)
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
						re.Packets = fmt.Sprintf("%d", tc.Packets)
						re.Bytes = fmt.Sprintf("%d", tc.Bytes)
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
			parts = append(parts, fmt.Sprintf("dscp %d", v.Value))
		case firewall.MatchICMPType:
			parts = append(parts, fmt.Sprintf("icmp type %d", v.Type))
		case firewall.MatchICMPv6Type:
			parts = append(parts, fmt.Sprintf("icmpv6 type %d", v.Type))
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
			parts = append(parts, fmt.Sprintf("%d", r.Lo))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.Lo, r.Hi))
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
			parts = append(parts, fmt.Sprintf("redirect :%d", v.Port))
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
			parts = append(parts, fmt.Sprintf("dscp %d", v.Value))
		case firewall.SetTCPMSS:
			parts = append(parts, fmt.Sprintf("tcp-mss %d", v.Size))
		case firewall.Limit:
			parts = append(parts, fmt.Sprintf("limit %d/%s", v.Rate, v.Unit))
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

		rows = append(rows, WorkbenchTableRow{
			Key:       fmt.Sprintf("%s/%s/%s", re.Table, re.Chain, re.Name),
			Flags:     flagStr,
			FlagClass: flagClass,
			Cells: []string{
				fmt.Sprintf("%d", re.Order),
				flagStr,
				re.Chain,
				re.Match,
				re.Action,
				re.Packets,
				re.Bytes,
				valueOrDash(re.Comment),
			},
			Actions: []WorkbenchRowAction{
				{Label: "Edit", URL: fmt.Sprintf("/show/firewall/table/%s/chain/%s/rule/%s/", re.Table, re.Chain, re.Name)},
				{Label: "Toggle", HxPost: fmt.Sprintf("/show/firewall/table/%s/chain/%s/rule/%s/toggle", re.Table, re.Chain, re.Name),
					Confirm: fmt.Sprintf("Toggle rule %q?", re.Name)},
				{Label: "Move Up", HxPost: fmt.Sprintf("/show/firewall/table/%s/chain/%s/rule/%s/move-up", re.Table, re.Chain, re.Name)},
				{Label: "Move Down", HxPost: fmt.Sprintf("/show/firewall/table/%s/chain/%s/rule/%s/move-down", re.Table, re.Chain, re.Name)},
				{Label: "Clone", HxPost: fmt.Sprintf("/show/firewall/table/%s/chain/%s/rule/%s/clone", re.Table, re.Chain, re.Name)},
				{Label: "Delete", HxPost: fmt.Sprintf("/show/firewall/table/%s/chain/%s/rule/%s/delete", re.Table, re.Chain, re.Name),
					Class: "danger", Confirm: fmt.Sprintf("Delete rule %q?", re.Name)},
			},
		})
	}

	emptyMsg := "No rules configured."
	if filterChain != "" {
		emptyMsg = fmt.Sprintf("No rules in chain %q.", filterChain)
	}
	emptyHint := "Add a rule to start filtering traffic."
	if filterChain != "" {
		// Show the chain's default policy in the empty hint.
		emptyHint = fmt.Sprintf("Chain %q has no rules. Traffic follows the chain's default policy.", filterChain)
	}

	addURL := "/show/firewall/rule/add"
	if filterTable != "" && filterChain != "" {
		addURL = fmt.Sprintf("/show/firewall/table/%s/chain/%s/rule/add", filterTable, filterChain)
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
				fmt.Sprintf("%d", se.ElementCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Elements", URL: fmt.Sprintf("/show/firewall/table/%s/set/%s/", se.Table, se.Name)},
				{Label: "Delete", HxPost: fmt.Sprintf("/show/firewall/table/%s/set/%s/delete", se.Table, se.Name),
					Class: "danger", Confirm: fmt.Sprintf("Delete set %q?", se.Name)},
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
