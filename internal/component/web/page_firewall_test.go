package web

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

// storeTestFirewallState stores test firewall tables in the global applied state
// and returns a cleanup function that clears it.
func storeTestFirewallState(tables []firewall.Table) func() {
	firewall.StoreLastApplied(tables)
	return func() { firewall.StoreLastApplied(nil) }
}

// buildTestFirewallTables creates a representative set of firewall tables for testing.
func buildTestFirewallTables() []firewall.Table {
	return []firewall.Table{
		{
			Name:   "ze_filter",
			Family: firewall.FamilyInet,
			Chains: []firewall.Chain{
				{
					Name:     "input",
					IsBase:   true,
					Type:     firewall.ChainFilter,
					Hook:     firewall.HookInput,
					Priority: 0,
					Policy:   firewall.PolicyDrop,
					Terms: []firewall.Term{
						{
							Name: "allow-established",
							Matches: []firewall.Match{
								firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateRelated},
							},
							Actions: []firewall.Action{
								firewall.Accept{},
							},
						},
						{
							Name: "allow-ssh",
							Matches: []firewall.Match{
								firewall.MatchProtocol{Protocol: "tcp"},
								firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}},
								firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
							},
							Actions: []firewall.Action{
								firewall.Counter{},
								firewall.Accept{},
							},
						},
						{
							Name: "allow-icmp",
							Matches: []firewall.Match{
								firewall.MatchProtocol{Protocol: "icmp"},
							},
							Actions: []firewall.Action{
								firewall.Accept{},
							},
						},
					},
				},
				{
					Name:     "forward",
					IsBase:   true,
					Type:     firewall.ChainFilter,
					Hook:     firewall.HookForward,
					Priority: 0,
					Policy:   firewall.PolicyAccept,
				},
				{
					Name: "custom-chain",
				},
			},
			Sets: []firewall.Set{
				{
					Name:  "blocklist",
					Type:  firewall.SetTypeIPv4,
					Flags: firewall.SetFlagInterval | firewall.SetFlagTimeout,
					Elements: []firewall.SetElement{
						{Value: "192.168.1.0/24"},
						{Value: "10.0.0.0/8"},
					},
				},
			},
		},
		{
			Name:   "ze_nat",
			Family: firewall.FamilyIP,
			Chains: []firewall.Chain{
				{
					Name:     "postrouting",
					IsBase:   true,
					Type:     firewall.ChainNAT,
					Hook:     firewall.HookPostrouting,
					Priority: 100,
					Policy:   firewall.PolicyAccept,
					Terms: []firewall.Term{
						{
							Name: "masq-wan",
							Matches: []firewall.Match{
								firewall.MatchOutputInterface{Name: "eth0", Wildcard: false},
							},
							Actions: []firewall.Action{
								firewall.Masquerade{},
							},
						},
					},
				},
			},
		},
	}
}

// --- Tables page tests ---

func TestFirewallTablesRendersTable(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectTables()
	require.Len(t, entries, 2, "should have filter and nat tables")

	data := BuildFirewallTablesTableData(entries)
	assert.Equal(t, "Firewall Tables", data.Title)
	assert.Len(t, data.Rows, 2)
	assert.Equal(t, "Add Table", data.AddLabel)

	// Verify columns
	assert.Len(t, data.Columns, 4)
	assert.Equal(t, "Name", data.Columns[0].Label)
	assert.Equal(t, "Family", data.Columns[1].Label)
	assert.Equal(t, "Chains", data.Columns[2].Label)
	assert.Equal(t, "Sets", data.Columns[3].Label)
}

func TestFirewallTablesJSON(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectTables()
	data := BuildFirewallTablesTableData(entries)

	// The first table (filter) should have 3 chains and 1 set.
	require.Len(t, data.Rows, 2)
	filterRow := data.Rows[0]
	assert.Equal(t, "filter", filterRow.Key)
	assert.Equal(t, "filter", filterRow.Cells[0])
	assert.Equal(t, "inet", filterRow.Cells[1])
	assert.Equal(t, "3", filterRow.Cells[2]) // 3 chains
	assert.Equal(t, "1", filterRow.Cells[3]) // 1 set

	natRow := data.Rows[1]
	assert.Equal(t, "nat", natRow.Key)
	assert.Equal(t, "ip", natRow.Cells[1])
	assert.Equal(t, "1", natRow.Cells[2]) // 1 chain
	assert.Equal(t, "0", natRow.Cells[3]) // 0 sets
}

func TestFirewallTablesEmpty(t *testing.T) {
	cleanup := storeTestFirewallState(nil)
	defer cleanup()

	entries := collectTables()
	data := BuildFirewallTablesTableData(entries)
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No firewall tables configured.", data.EmptyMessage)
	assert.Contains(t, data.EmptyHint, "Create a table")
	assert.NotEmpty(t, data.AddURL)
}

func TestFirewallTableViewModel(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectTables()
	require.Len(t, entries, 2)

	// Filter table
	assert.Equal(t, "filter", entries[0].Name)
	assert.Equal(t, "inet", entries[0].Family)
	assert.Equal(t, 3, entries[0].ChainCount)
	assert.Equal(t, 1, entries[0].SetCount)

	// NAT table
	assert.Equal(t, "nat", entries[1].Name)
	assert.Equal(t, "ip", entries[1].Family)
	assert.Equal(t, 1, entries[1].ChainCount)
	assert.Equal(t, 0, entries[1].SetCount)
}

func TestFirewallTablesRowActions(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectTables()
	data := BuildFirewallTablesTableData(entries)

	require.NotEmpty(t, data.Rows)
	row := data.Rows[0]
	require.Len(t, row.Actions, 3)
	assert.Equal(t, "View Chains", row.Actions[0].Label)
	assert.Contains(t, row.Actions[0].URL, "table=filter")
	assert.Equal(t, "Edit", row.Actions[1].Label)
	assert.Equal(t, "Delete", row.Actions[2].Label)
	assert.Equal(t, "danger", row.Actions[2].Class)
	assert.NotEmpty(t, row.Actions[2].Confirm)
}

func TestFirewallTablesStripZePrefix(t *testing.T) {
	cleanup := storeTestFirewallState([]firewall.Table{
		{Name: "ze_my-table", Family: firewall.FamilyInet},
	})
	defer cleanup()

	entries := collectTables()
	require.Len(t, entries, 1)
	assert.Equal(t, "my-table", entries[0].Name)
}

// --- Chains page tests ---

func TestFirewallChainsRendersTable(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectChains("", "", "")
	require.Len(t, entries, 4, "filter(3) + nat(1) = 4 chains")

	data := BuildFirewallChainsTableData(entries, "")
	assert.Equal(t, "Firewall Chains", data.Title)
	assert.Len(t, data.Rows, 4)
	assert.Equal(t, "Add Chain", data.AddLabel)

	// Verify columns
	assert.Len(t, data.Columns, 7)
	assert.Equal(t, "Table", data.Columns[0].Label)
	assert.Equal(t, "Name", data.Columns[1].Label)
	assert.Equal(t, "Type", data.Columns[2].Label)
	assert.Equal(t, "Hook", data.Columns[3].Label)
	assert.Equal(t, "Priority", data.Columns[4].Label)
	assert.Equal(t, "Policy", data.Columns[5].Label)
	assert.Equal(t, "Rule Count", data.Columns[6].Label)
}

func TestFirewallChainsFilterByTable(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectChains("filter", "", "")
	require.Len(t, entries, 3)
	for _, e := range entries {
		assert.Equal(t, "filter", e.Table)
	}

	data := BuildFirewallChainsTableData(entries, "filter")
	assert.Len(t, data.Rows, 3)
}

func TestFirewallChainsFilterByHook(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectChains("", "input", "")
	require.Len(t, entries, 1)
	assert.Equal(t, "input", entries[0].Name)
	assert.Equal(t, "input", entries[0].Hook)
}

func TestFirewallChainsFilterByType(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectChains("", "", "nat")
	require.Len(t, entries, 1)
	assert.Equal(t, "postrouting", entries[0].Name)
	assert.Equal(t, "nat", entries[0].Type)
}

func TestFirewallChainsEmpty(t *testing.T) {
	cleanup := storeTestFirewallState(nil)
	defer cleanup()

	entries := collectChains("", "", "")
	data := BuildFirewallChainsTableData(entries, "")
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No chains configured.", data.EmptyMessage)
}

func TestFirewallChainsEmptyFilteredTable(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectChains("nonexistent", "", "")
	data := BuildFirewallChainsTableData(entries, "nonexistent")
	assert.Empty(t, data.Rows)
	assert.Contains(t, data.EmptyMessage, "nonexistent")
}

func TestFirewallChainsBaseChainFields(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectChains("filter", "", "")
	// Find input chain (base chain)
	var inputChain chainEntry
	for _, e := range entries {
		if e.Name == "input" {
			inputChain = e
			break
		}
	}

	assert.True(t, inputChain.IsBase)
	assert.Equal(t, "filter", inputChain.Type)
	assert.Equal(t, "input", inputChain.Hook)
	assert.Equal(t, int32(0), inputChain.Priority)
	assert.Equal(t, "drop", inputChain.Policy)
	assert.Equal(t, 3, inputChain.RuleCount)
}

func TestFirewallChainsRegularChainFields(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectChains("filter", "", "")
	// Find custom-chain (regular chain, not base)
	var custom chainEntry
	for _, e := range entries {
		if e.Name == "custom-chain" {
			custom = e
			break
		}
	}

	assert.False(t, custom.IsBase)
	assert.Equal(t, "", custom.Type)
	assert.Equal(t, "", custom.Hook)
	assert.Equal(t, 0, custom.RuleCount)
}

func TestFirewallChainsRowActions(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectChains("", "", "")
	data := BuildFirewallChainsTableData(entries, "")

	require.NotEmpty(t, data.Rows)
	row := data.Rows[0]
	require.Len(t, row.Actions, 3)
	assert.Equal(t, "View Rules", row.Actions[0].Label)
	assert.Equal(t, "Edit", row.Actions[1].Label)
	assert.Equal(t, "Delete", row.Actions[2].Label)
	assert.Equal(t, "danger", row.Actions[2].Class)
}

// --- Rules page tests ---

func TestFirewallRulesRendersTable(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectRules("", "")
	// filter: input(3) + forward(0) + custom(0) + nat: postrouting(1) = 4 rules
	require.Len(t, entries, 4)

	data := BuildFirewallRulesTableData(entries, "", "")
	assert.Equal(t, "Firewall Rules", data.Title)
	assert.Len(t, data.Rows, 4)
}

func TestFirewallRulesOrderPreserved(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectRules("filter", "input")
	require.Len(t, entries, 3)

	// Order must match config order, not alphabetical.
	assert.Equal(t, 1, entries[0].Order)
	assert.Equal(t, "allow-established", entries[0].Name)
	assert.Equal(t, 2, entries[1].Order)
	assert.Equal(t, "allow-ssh", entries[1].Name)
	assert.Equal(t, 3, entries[2].Order)
	assert.Equal(t, "allow-icmp", entries[2].Name)
}

func TestFirewallRulesFilterByChain(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectRules("filter", "input")
	require.Len(t, entries, 3)
	for _, e := range entries {
		assert.Equal(t, "input", e.Chain)
		assert.Equal(t, "filter", e.Table)
	}
}

func TestFirewallRulesFilterByTable(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectRules("nat", "")
	require.Len(t, entries, 1)
	assert.Equal(t, "masq-wan", entries[0].Name)
	assert.Equal(t, "nat", entries[0].Table)
}

func TestFirewallRulesEmpty(t *testing.T) {
	cleanup := storeTestFirewallState(nil)
	defer cleanup()

	entries := collectRules("", "")
	data := BuildFirewallRulesTableData(entries, "", "")
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No rules configured.", data.EmptyMessage)
}

func TestFirewallRulesEmptyChain(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectRules("filter", "forward")
	data := BuildFirewallRulesTableData(entries, "filter", "forward")
	assert.Empty(t, data.Rows)
	assert.Contains(t, data.EmptyMessage, "forward")
	assert.Contains(t, data.EmptyHint, "default policy")
}

func TestFirewallRulesRowActions(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectRules("filter", "input")
	data := BuildFirewallRulesTableData(entries, "filter", "input")

	require.NotEmpty(t, data.Rows)
	row := data.Rows[0]
	require.Len(t, row.Actions, 6)
	assert.Equal(t, "Edit", row.Actions[0].Label)
	assert.Equal(t, "Toggle", row.Actions[1].Label)
	assert.Equal(t, "Move Up", row.Actions[2].Label)
	assert.Equal(t, "Move Down", row.Actions[3].Label)
	assert.Equal(t, "Clone", row.Actions[4].Label)
	assert.Equal(t, "Delete", row.Actions[5].Label)
	assert.Equal(t, "danger", row.Actions[5].Class)
}

// --- Match summary tests ---

func TestFirewallMatchSummary(t *testing.T) {
	tests := []struct {
		name    string
		matches []firewall.Match
		want    string
	}{
		{
			name:    "empty",
			matches: nil,
			want:    "-",
		},
		{
			name:    "protocol only",
			matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}},
			want:    "tcp",
		},
		{
			name: "tcp dport 22 saddr 10/8",
			matches: []firewall.Match{
				firewall.MatchProtocol{Protocol: "tcp"},
				firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}},
				firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
			},
			want: "tcp dport 22 saddr 10.0.0.0/8",
		},
		{
			name: "port range",
			matches: []firewall.Match{
				firewall.MatchDestinationPort{Ranges: []firewall.PortRange{
					{Lo: 80, Hi: 80},
					{Lo: 443, Hi: 443},
					{Lo: 8000, Hi: 9000},
				}},
			},
			want: "dport 80,443,8000-9000",
		},
		{
			name: "conn state",
			matches: []firewall.Match{
				firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateRelated},
			},
			want: "ct state established,related",
		},
		{
			name: "input interface wildcard",
			matches: []firewall.Match{
				firewall.MatchInputInterface{Name: "eth", Wildcard: true},
			},
			want: "iif eth*",
		},
		{
			name: "output interface exact",
			matches: []firewall.Match{
				firewall.MatchOutputInterface{Name: "eth0", Wildcard: false},
			},
			want: "oif eth0",
		},
		{
			name: "dscp",
			matches: []firewall.Match{
				firewall.MatchDSCP{Value: 46},
			},
			want: "dscp 46",
		},
		{
			name: "icmp type",
			matches: []firewall.Match{
				firewall.MatchICMPType{Type: 8},
			},
			want: "icmp type 8",
		},
		{
			name: "icmpv6 type",
			matches: []firewall.Match{
				firewall.MatchICMPv6Type{Type: 128},
			},
			want: "icmpv6 type 128",
		},
		{
			name: "in set",
			matches: []firewall.Match{
				firewall.MatchInSet{SetName: "blocklist", MatchField: firewall.SetFieldSourceAddr},
			},
			want: "in set blocklist",
		},
		{
			name: "mark",
			matches: []firewall.Match{
				firewall.MatchMark{Value: 0x100, Mask: 0xff00},
			},
			want: "mark 0x100/0xff00",
		},
		{
			name: "conn mark",
			matches: []firewall.Match{
				firewall.MatchConnMark{Value: 0x1, Mask: 0xf},
			},
			want: "ct mark 0x1/0xf",
		},
		{
			name: "tcp flags",
			matches: []firewall.Match{
				firewall.MatchTCPFlags{Flags: firewall.TCPFlagSYN, Mask: firewall.TCPFlagSYN | firewall.TCPFlagACK},
			},
			want: "tcp flags 0x2/0x12",
		},
		{
			name: "source port",
			matches: []firewall.Match{
				firewall.MatchSourcePort{Ranges: []firewall.PortRange{{Lo: 1024, Hi: 65535}}},
			},
			want: "sport 1024-65535",
		},
		{
			name: "destination address",
			matches: []firewall.Match{
				firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("192.168.0.0/16")},
			},
			want: "daddr 192.168.0.0/16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchSummary(tt.matches)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- Action summary tests ---

func TestFirewallActionSummary(t *testing.T) {
	tests := []struct {
		name    string
		actions []firewall.Action
		want    string
	}{
		{
			name:    "empty",
			actions: nil,
			want:    "-",
		},
		{
			name:    "accept",
			actions: []firewall.Action{firewall.Accept{}},
			want:    "accept",
		},
		{
			name:    "drop",
			actions: []firewall.Action{firewall.Drop{}},
			want:    "drop",
		},
		{
			name:    "reject with type",
			actions: []firewall.Action{firewall.Reject{Type: "tcp-reset"}},
			want:    "reject (tcp-reset)",
		},
		{
			name:    "reject without type",
			actions: []firewall.Action{firewall.Reject{}},
			want:    "reject",
		},
		{
			name:    "jump",
			actions: []firewall.Action{firewall.Jump{Target: "custom-chain"}},
			want:    "jump custom-chain",
		},
		{
			name:    "goto",
			actions: []firewall.Action{firewall.Goto{Target: "other"}},
			want:    "goto other",
		},
		{
			name:    "return",
			actions: []firewall.Action{firewall.Return{}},
			want:    "return",
		},
		{
			name:    "snat",
			actions: []firewall.Action{firewall.SNAT{Address: netip.MustParseAddr("1.2.3.4")}},
			want:    "snat 1.2.3.4",
		},
		{
			name:    "dnat",
			actions: []firewall.Action{firewall.DNAT{Address: netip.MustParseAddr("10.0.0.1")}},
			want:    "dnat 10.0.0.1",
		},
		{
			name:    "masquerade",
			actions: []firewall.Action{firewall.Masquerade{}},
			want:    "masquerade",
		},
		{
			name:    "redirect",
			actions: []firewall.Action{firewall.Redirect{Port: 8080}},
			want:    "redirect :8080",
		},
		{
			name:    "notrack",
			actions: []firewall.Action{firewall.Notrack{}},
			want:    "notrack",
		},
		{
			name:    "flow offload",
			actions: []firewall.Action{firewall.FlowOffload{FlowtableName: "ft"}},
			want:    "flow offload ft",
		},
		{
			name:    "counter + accept",
			actions: []firewall.Action{firewall.Counter{}, firewall.Accept{}},
			want:    "counter accept",
		},
		{
			name:    "log with prefix",
			actions: []firewall.Action{firewall.Log{Prefix: "DROP:"}},
			want:    "log DROP:",
		},
		{
			name:    "log without prefix",
			actions: []firewall.Action{firewall.Log{}},
			want:    "log",
		},
		{
			name:    "set mark",
			actions: []firewall.Action{firewall.SetMark{Value: 0x42}},
			want:    "mark 0x42",
		},
		{
			name:    "set conn mark",
			actions: []firewall.Action{firewall.SetConnMark{Value: 0x1}},
			want:    "ct mark 0x1",
		},
		{
			name:    "set dscp",
			actions: []firewall.Action{firewall.SetDSCP{Value: 46}},
			want:    "dscp 46",
		},
		{
			name:    "set tcp mss",
			actions: []firewall.Action{firewall.SetTCPMSS{Size: 1400}},
			want:    "tcp-mss 1400",
		},
		{
			name:    "limit",
			actions: []firewall.Action{firewall.Limit{Rate: 100, Unit: "second"}},
			want:    "limit 100/second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := actionSummary(tt.actions)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- Sets page tests ---

func TestFirewallSetsRendersTable(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectSets("")
	require.Len(t, entries, 1)

	data := BuildFirewallSetsTableData(entries)
	assert.Equal(t, "Firewall Sets", data.Title)
	assert.Len(t, data.Rows, 1)
	assert.Equal(t, "Add Set", data.AddLabel)
}

func TestFirewallSetsElementCount(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectSets("")
	require.Len(t, entries, 1)

	assert.Equal(t, "blocklist", entries[0].Name)
	assert.Equal(t, "filter", entries[0].Table)
	assert.Equal(t, "ipv4_addr", entries[0].Type)
	assert.Equal(t, 2, entries[0].ElementCount)

	data := BuildFirewallSetsTableData(entries)
	require.Len(t, data.Rows, 1)
	// Elements column
	assert.Equal(t, "2", data.Rows[0].Cells[4])
}

func TestFirewallSetsFlags(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectSets("")
	require.Len(t, entries, 1)
	assert.Equal(t, "interval, timeout", entries[0].Flags)
}

func TestFirewallSetsEmpty(t *testing.T) {
	cleanup := storeTestFirewallState(nil)
	defer cleanup()

	entries := collectSets("")
	data := BuildFirewallSetsTableData(entries)
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No named sets.", data.EmptyMessage)
}

func TestFirewallSetsFilterByTable(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectSets("nat")
	assert.Empty(t, entries) // nat table has no sets

	entries = collectSets("filter")
	require.Len(t, entries, 1)
	assert.Equal(t, "blocklist", entries[0].Name)
}

func TestFirewallSetsRowActions(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectSets("")
	data := BuildFirewallSetsTableData(entries)

	require.NotEmpty(t, data.Rows)
	row := data.Rows[0]
	require.Len(t, row.Actions, 2)
	assert.Equal(t, "View Elements", row.Actions[0].Label)
	assert.Equal(t, "Delete", row.Actions[1].Label)
	assert.Equal(t, "danger", row.Actions[1].Class)
}

// --- Connections page tests ---

func TestFirewallConnectionsRendersTable(t *testing.T) {
	data := BuildFirewallConnectionsTableData()
	assert.Equal(t, "Firewall Connections", data.Title)
	assert.Empty(t, data.Rows)
	assert.Len(t, data.Columns, 7)
}

func TestFirewallConnectionsNoBackend(t *testing.T) {
	// No backend loaded: show placeholder.
	data := BuildFirewallConnectionsTableData()
	assert.Contains(t, data.EmptyMessage, "requires a running firewall")
}

// --- Set flags string tests ---

func TestSetFlagsStr(t *testing.T) {
	tests := []struct {
		flags firewall.SetFlags
		want  string
	}{
		{0, "-"},
		{firewall.SetFlagInterval, "interval"},
		{firewall.SetFlagTimeout, "timeout"},
		{firewall.SetFlagConstant, "constant"},
		{firewall.SetFlagDynamic, "dynamic"},
		{firewall.SetFlagInterval | firewall.SetFlagTimeout, "interval, timeout"},
		{firewall.SetFlagInterval | firewall.SetFlagTimeout | firewall.SetFlagDynamic, "interval, timeout, dynamic"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, setFlagsStr(tt.flags))
		})
	}
}

// --- ConnState string tests ---

func TestConnStateStr(t *testing.T) {
	tests := []struct {
		state firewall.ConnState
		want  string
	}{
		{0, "none"},
		{firewall.ConnStateNew, "new"},
		{firewall.ConnStateEstablished, "established"},
		{firewall.ConnStateRelated, "related"},
		{firewall.ConnStateInvalid, "invalid"},
		{firewall.ConnStateNew | firewall.ConnStateEstablished, "new,established"},
		{firewall.ConnStateEstablished | firewall.ConnStateRelated, "established,related"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, connStateStr(tt.state))
		})
	}
}

// --- Port range formatting tests ---

func TestFormatPortRanges(t *testing.T) {
	tests := []struct {
		name   string
		ranges []firewall.PortRange
		want   string
	}{
		{"single port", []firewall.PortRange{{Lo: 22, Hi: 22}}, "22"},
		{"range", []firewall.PortRange{{Lo: 1024, Hi: 65535}}, "1024-65535"},
		{"multiple", []firewall.PortRange{
			{Lo: 80, Hi: 80},
			{Lo: 443, Hi: 443},
		}, "80,443"},
		{"mixed", []firewall.PortRange{
			{Lo: 22, Hi: 22},
			{Lo: 8000, Hi: 9000},
		}, "22,8000-9000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatPortRanges(tt.ranges))
		})
	}
}

// --- Workbench dispatch integration tests ---

func TestWorkbench_FirewallTablesPageDispatch(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/firewall/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	assert.Contains(t, body, "Add Table", "must contain add table button")
	assert.Contains(t, body, `id="workbench-shell"`, "must be inside workbench shell")
}

func TestWorkbench_FirewallTablesHTMXPartial(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/firewall/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "HTMX partial must contain table")
	assert.NotContains(t, body, `id="workbench-shell"`, "HTMX partial must not contain shell")
}

func TestWorkbench_FirewallChainsPageDispatch(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/firewall/chain/?table=filter", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	assert.Contains(t, body, "Add Chain", "must contain add chain button")
}

func TestWorkbench_FirewallRulesPageDispatch(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/firewall/rule/?table=filter&chain=input", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	assert.Contains(t, body, "Add Rule", "must contain add rule button")
}

func TestWorkbench_FirewallSetsPageDispatch(t *testing.T) {
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/firewall/set/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	assert.Contains(t, body, "Add Set", "must contain add set button")
}

func TestWorkbench_FirewallConnectionsPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/firewall/connections/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	assert.Contains(t, body, "requires a running firewall", "must show placeholder message")
}

func TestWorkbench_FirewallTablesEmptyPageDispatch(t *testing.T) {
	cleanup := storeTestFirewallState(nil)
	defer cleanup()

	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/firewall/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "No firewall tables configured", "must show empty state")
	assert.Contains(t, body, "Add Table", "must show add button")
}

// --- Navigation tests ---

func TestFirewallSubNavigation(t *testing.T) {
	sections := WorkbenchSections([]string{"firewall"})

	var fwSection *WorkbenchSection
	for i := range sections {
		if sections[i].Key == segFirewall {
			fwSection = &sections[i]
			break
		}
	}
	require.NotNil(t, fwSection, "Firewall section must exist")
	assert.True(t, fwSection.Selected, "Firewall section must be selected for firewall path")
	assert.True(t, fwSection.Expanded, "Firewall section must be expanded")

	// Default sub-page (tables) should be selected.
	var tablesChild *WorkbenchSubPage
	for i := range fwSection.Children {
		if fwSection.Children[i].Key == "tables" {
			tablesChild = &fwSection.Children[i]
			break
		}
	}
	require.NotNil(t, tablesChild, "Tables sub-page must exist")
	assert.True(t, tablesChild.Selected, "Tables sub-page must be selected")
}

func TestFirewallChainsSubNavigation(t *testing.T) {
	sections := WorkbenchSections([]string{"firewall", "chain"})

	var fwSection *WorkbenchSection
	for i := range sections {
		if sections[i].Key == segFirewall {
			fwSection = &sections[i]
			break
		}
	}
	require.NotNil(t, fwSection)
	assert.True(t, fwSection.Selected)

	var chainsChild *WorkbenchSubPage
	for i := range fwSection.Children {
		if fwSection.Children[i].Key == "chains" {
			chainsChild = &fwSection.Children[i]
			break
		}
	}
	require.NotNil(t, chainsChild, "Chains sub-page must exist")
	assert.True(t, chainsChild.Selected, "Chains sub-page must be selected")
}

func TestFirewallContextNavigation(t *testing.T) {
	// Verify that View Chains action from a table row includes the table parameter.
	cleanup := storeTestFirewallState(buildTestFirewallTables())
	defer cleanup()

	entries := collectTables()
	data := BuildFirewallTablesTableData(entries)

	require.NotEmpty(t, data.Rows)
	viewChainsAction := data.Rows[0].Actions[0]
	assert.Equal(t, "View Chains", viewChainsAction.Label)
	assert.Contains(t, viewChainsAction.URL, "table=filter")

	// Verify that View Rules action from a chain row includes table and chain params.
	chainEntries := collectChains("filter", "", "")
	chainData := BuildFirewallChainsTableData(chainEntries, "filter")

	require.NotEmpty(t, chainData.Rows)
	viewRulesAction := chainData.Rows[0].Actions[0]
	assert.Equal(t, "View Rules", viewRulesAction.Label)
	assert.Contains(t, viewRulesAction.URL, "table=filter")
	assert.Contains(t, viewRulesAction.URL, "chain=")
}
