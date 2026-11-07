package firewall

import (
	"slices"
	"strings"
	"testing"
)

// helperAssignment is one (object, kernel helper, transport, port) a table
// assigns, read back from the lowered tables.
type helperAssignment struct {
	table    string
	family   TableFamily
	object   string
	helper   string
	protocol string
	port     uint16
}

// assignments flattens the lowered tables into what they assign, failing the
// test when the prerouting and output chains disagree or a rule names an
// object the table does not declare.
func assignments(t *testing.T, tables []Table) []helperAssignment {
	t.Helper()
	var out []helperAssignment
	for i := range tables {
		tbl := &tables[i]
		if len(tbl.Chains) != 2 {
			t.Fatalf("table %s has %d chains, want prerouting and output", tbl.Name, len(tbl.Chains))
		}
		for _, ch := range tbl.Chains {
			if !ch.IsBase || ch.Priority != -5 || ch.Type != ChainFilter || ch.Policy != PolicyAccept {
				t.Fatalf("table %s chain %s = %+v, want a filter base chain at priority -5, policy accept", tbl.Name, ch.Name, ch)
			}
		}
		if tbl.Chains[0].Hook != HookPrerouting || tbl.Chains[1].Hook != HookOutput {
			t.Fatalf("table %s hooks %s, %s; want prerouting, output", tbl.Name, tbl.Chains[0].Hook, tbl.Chains[1].Hook)
		}
		if len(tbl.Chains[0].Terms) != len(tbl.Chains[1].Terms) {
			t.Fatalf("table %s: prerouting and output assign different rules", tbl.Name)
		}
		for _, term := range tbl.Chains[0].Terms {
			protoMatch, ok := term.Matches[0].(MatchProtocol)
			if !ok {
				t.Fatalf("table %s term %s: first match %T, want MatchProtocol", tbl.Name, term.Name, term.Matches[0])
			}
			portMatch, ok := term.Matches[1].(MatchDestinationPort)
			if !ok {
				t.Fatalf("table %s term %s: second match %T, want MatchDestinationPort", tbl.Name, term.Name, term.Matches[1])
			}
			action, ok := term.Actions[0].(SetCtHelper)
			if !ok {
				t.Fatalf("table %s term %s: action %T, want SetCtHelper", tbl.Name, term.Name, term.Actions[0])
			}
			proto, port, object := protoMatch.Protocol, portMatch.Ranges[0], action.Name
			idx := slices.IndexFunc(tbl.CtHelpers, func(h CtHelper) bool { return h.Name == object })
			if idx < 0 {
				t.Fatalf("table %s term %s assigns undeclared object %q", tbl.Name, term.Name, object)
			}
			obj := tbl.CtHelpers[idx]
			if obj.Protocol != proto || port.Lo != port.Hi {
				t.Fatalf("table %s term %s: object protocol %s, rule protocol %s, ports %v", tbl.Name, term.Name, obj.Protocol, proto, port)
			}
			out = append(out, helperAssignment{tbl.Name, tbl.Family, object, obj.Helper, proto, port.Lo})
		}
	}
	return out
}

// TestHelperLowering proves each helper lowers to the objects and port rules
// its row declares, in the table of the families its kernel helper registers.
//
// VALIDATES: AC-1 ftp tcp 21 in inet; AC-2 pptp in an ip table only; AC-3
// h323 RAS udp 1719 and Q.931 tcp 1720; AC-4 sip udp and tcp 5060 and 5061;
// AC-12 tftp udp 69 in inet; AC-5 an empty list lowers to no table. Every
// lowered set passes ValidateTables.
// PREVENTS: a listed helper with no assignment, or an IPv4-only helper named
// in an IPv6 path.
func TestHelperLowering(t *testing.T) {
	tests := []struct {
		module string
		want   []helperAssignment
	}{
		{"ftp", []helperAssignment{
			{conntrackHelperTableInet, FamilyInet, "ftp-tcp", "ftp", "tcp", 21},
		}},
		{"pptp", []helperAssignment{
			{conntrackHelperTableIP, FamilyIP, "pptp-tcp", "pptp", "tcp", 1723},
		}},
		{"h323", []helperAssignment{
			{conntrackHelperTableInet, FamilyInet, "h323-udp", "RAS", "udp", 1719},
			{conntrackHelperTableInet, FamilyInet, "h323-tcp", "Q.931", "tcp", 1720},
		}},
		{"sip", []helperAssignment{
			{conntrackHelperTableInet, FamilyInet, "sip-udp", "sip", "udp", 5060},
			{conntrackHelperTableInet, FamilyInet, "sip-udp", "sip", "udp", 5061},
			{conntrackHelperTableInet, FamilyInet, "sip-tcp", "sip", "tcp", 5060},
			{conntrackHelperTableInet, FamilyInet, "sip-tcp", "sip", "tcp", 5061},
		}},
		{"tftp", []helperAssignment{
			{conntrackHelperTableInet, FamilyInet, "tftp-udp", "tftp", "udp", 69},
		}},
		{"irc", []helperAssignment{
			{conntrackHelperTableIP, FamilyIP, "irc-tcp", "irc", "tcp", 6667},
		}},
		{"sane", []helperAssignment{
			{conntrackHelperTableInet, FamilyInet, "sane-tcp", "sane", "tcp", 6566},
		}},
		{"amanda", []helperAssignment{
			{conntrackHelperTableInet, FamilyInet, "amanda-udp", "amanda", "udp", 10080},
		}},
		{"netbios-ns", []helperAssignment{
			{conntrackHelperTableIP, FamilyIP, "netbios-ns-udp", "netbios-ns", "udp", 137},
		}},
		{"snmp", []helperAssignment{
			{conntrackHelperTableIP, FamilyIP, "snmp-udp", "snmp", "udp", 161},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			tables, err := ConntrackHelperTables([]string{tt.module})
			if err != nil {
				t.Fatalf("ConntrackHelperTables: %v", err)
			}
			if err := ValidateTables(tables); err != nil {
				t.Fatalf("ValidateTables: %v", err)
			}
			if len(tables) != 1 {
				t.Fatalf("got %d tables, want 1 (one family group per helper)", len(tables))
			}
			got := assignments(t, tables)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("assignments:\n got %+v\nwant %+v", got, tt.want)
			}
		})
	}

	t.Run("empty", func(t *testing.T) {
		tables, err := ConntrackHelperTables(nil)
		if err != nil {
			t.Fatalf("ConntrackHelperTables(nil): %v", err)
		}
		if len(tables) != 0 {
			t.Fatalf("an empty list lowered to %d tables; no helper may be assigned by default", len(tables))
		}
	})

	t.Run("both-families-and-ipv4-only", func(t *testing.T) {
		tables, err := ConntrackHelperTables([]string{"ftp", "pptp"})
		if err != nil {
			t.Fatalf("ConntrackHelperTables: %v", err)
		}
		if len(tables) != 2 || tables[0].Family != FamilyInet || tables[1].Family != FamilyIP {
			t.Fatalf("tables = %+v, want one inet and one ip table", tables)
		}
		for _, h := range tables[0].CtHelpers {
			if h.Name == "pptp-tcp" {
				t.Fatal("pptp, an IPv4-only helper, is declared in the inet table")
			}
		}
	})
}

// TestHelperLoweringRefusesUnknown proves a name with no declaration row is
// refused with the valid names, never lowered to nothing.
//
// VALIDATES: nfs and sqlnet, which no kernel module provides, are refused.
// PREVENTS: a configured helper that silently assigns nothing.
func TestHelperLoweringRefusesUnknown(t *testing.T) {
	for _, name := range []string{"nfs", "sqlnet"} {
		_, err := ConntrackHelperTables([]string{"ftp", name})
		if err == nil {
			t.Fatalf("ConntrackHelperTables accepted %q", name)
		}
		if !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "ftp") {
			t.Fatalf("error %q does not name the value and the valid helpers", err)
		}
	}
}

// TestValidateCtHelpers proves the table checks a ct helper object and its
// assignment before the backend sees them.
//
// VALIDATES: an assignment of an undeclared object, an object in an arp
// table, a duplicate object and a transport other than tcp or udp are refused.
// PREVENTS: a batch the kernel refuses with ENOENT or EOPNOTSUPP at Apply.
func TestValidateCtHelpers(t *testing.T) {
	good := func() Table {
		tables, err := ConntrackHelperTables([]string{"ftp"})
		if err != nil {
			t.Fatalf("ConntrackHelperTables: %v", err)
		}
		return tables[0]
	}
	if err := ValidateTables([]Table{good()}); err != nil {
		t.Fatalf("valid helper table refused: %v", err)
	}

	undeclared := good()
	undeclared.CtHelpers = nil
	if err := ValidateTables([]Table{undeclared}); err == nil || !strings.Contains(err.Error(), "unknown ct helper") {
		t.Fatalf("assignment of an undeclared object: err = %v", err)
	}

	arp := good()
	arp.Family = FamilyARP
	arp.Chains = nil
	if err := ValidateTables([]Table{arp}); err == nil || !strings.Contains(err.Error(), "family ip, ip6, or inet") {
		t.Fatalf("ct helper in an arp table: err = %v", err)
	}

	dup := good()
	dup.CtHelpers = append(dup.CtHelpers, dup.CtHelpers[0])
	if err := ValidateTables([]Table{dup}); err == nil || !strings.Contains(err.Error(), "declared twice") {
		t.Fatalf("duplicate object: err = %v", err)
	}

	sctp := good()
	sctp.CtHelpers[0].Protocol = "sctp"
	if err := ValidateTables([]Table{sctp}); err == nil || !strings.Contains(err.Error(), "tcp or udp") {
		t.Fatalf("sctp object: err = %v", err)
	}
}
