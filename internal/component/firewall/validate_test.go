package firewall

import (
	"strings"
	"testing"
)

// makeTable builds a minimal valid Table for tests that only care about
// the field under test. Callers mutate the returned struct in place.
func makeTable(family TableFamily) Table {
	return Table{
		Name:   "ze_wan",
		Family: family,
		Chains: []Chain{{
			Name:   "c",
			IsBase: true,
			Type:   ChainFilter,
			Hook:   HookInput,
			Policy: PolicyAccept,
			Terms: []Term{{
				Name:    "t",
				Actions: []Action{Accept{}},
			}},
		}},
	}
}

// VALIDATES: ISSUE #1 -- MatchICMPType in an ip6 or arp table rejects
// because the type number is ICMPv4-specific. MatchICMPType in ip or
// inet accepts. Same split for MatchICMPv6Type.
// PREVENTS: operator using `icmp-type echo-request` (value 8) in an
// ip6 table, which would match ICMPv6 type 8 = "packet too big".
func TestValidateICMPTypeFamily(t *testing.T) {
	tests := []struct {
		name    string
		match   Match
		family  TableFamily
		wantErr string // empty = must accept
	}{
		{"icmp in ip", MatchICMPType{Type: 8}, FamilyIP, ""},
		{"icmp in inet", MatchICMPType{Type: 8}, FamilyInet, ""},
		{"icmp in ip6 rejects", MatchICMPType{Type: 8}, FamilyIP6, "icmp-type is valid only in family ip or inet"},
		{"icmp in arp rejects", MatchICMPType{Type: 8}, FamilyARP, "icmp-type is valid only in family ip or inet"},
		{"icmp6 in ip6", MatchICMPv6Type{Type: 128}, FamilyIP6, ""},
		{"icmp6 in inet", MatchICMPv6Type{Type: 128}, FamilyInet, ""},
		{"icmp6 in ip rejects", MatchICMPv6Type{Type: 128}, FamilyIP, "icmpv6-type is valid only in family ip6 or inet"},
		{"icmp6 in bridge rejects", MatchICMPv6Type{Type: 128}, FamilyBridge, "icmpv6-type is valid only in family ip6 or inet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(tt.family)
			tbl.Chains[0].Terms[0].Matches = []Match{tt.match}
			err := ValidateTables([]Table{tbl})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected accept, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

// VALIDATES: ISSUE #2 -- an empty interface name (e.g. from
// `input-interface "*"` which strips to Name="" + Wildcard=true) is
// rejected at verify so `ze config validate` surfaces it offline.
func TestValidateEmptyInterfaceName(t *testing.T) {
	tests := []struct {
		name  string
		match Match
	}{
		{"input exact", MatchInputInterface{Name: ""}},
		{"input wildcard", MatchInputInterface{Name: "", Wildcard: true}},
		{"output exact", MatchOutputInterface{Name: ""}},
		{"output wildcard", MatchOutputInterface{Name: "", Wildcard: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(FamilyInet)
			tbl.Chains[0].Terms[0].Matches = []Match{tt.match}
			err := ValidateTables([]Table{tbl})
			if err == nil || !strings.Contains(err.Error(), "must not be empty") {
				t.Fatalf("expected empty-name rejection, got %v", err)
			}
		})
	}
}

// VALIDATES: NOTE #6 -- an interface name longer than IFNAMSIZ-1 (15)
// is rejected at verify. The kernel stores names in a 16-byte register;
// 16-byte or longer names would silently never match.
// PREVENTS: operator writing `input-interface "wan-very-long-name"` and
// wondering why no traffic ever hits the rule.
func TestValidateLongInterfaceName(t *testing.T) {
	tbl := makeTable(FamilyInet)
	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInputInterface{Name: "exceeds-15-chars-yes"},
	}
	err := ValidateTables([]Table{tbl})
	if err == nil || !strings.Contains(err.Error(), "15-byte kernel limit") {
		t.Fatalf("expected length rejection, got %v", err)
	}
	// Exactly 15 characters is allowed.
	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInputInterface{Name: "exactly15chars!"}, // 15 runes, 15 bytes
	}
	if err := ValidateTables([]Table{tbl}); err != nil {
		t.Fatalf("15-char name must accept, got %v", err)
	}
}

// VALIDATES: NOTE #3 -- more than maxPortRanges comma-list entries
// rejects at parse with a clear message.
func TestParsePortSpecCapExceeded(t *testing.T) {
	parts := make([]string, maxPortRanges+1)
	for i := range parts {
		parts[i] = "1" // same port repeated; point is the count, not validity
	}
	spec := strings.Join(parts, ",")
	_, err := parsePortSpec(spec)
	if err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("expected cap rejection, got %v", err)
	}
	// Exactly the cap is allowed (each "1" being a distinct entry, but
	// the adjacency check will fire before we hit the cap message --
	// instead, use distinct values spaced two apart to exercise the
	// cap boundary without tripping the overlap check).
	distinct := make([]string, maxPortRanges)
	for i := range distinct {
		distinct[i] = itoa(i*2 + 1)
	}
	if _, err := parsePortSpec(strings.Join(distinct, ",")); err != nil {
		t.Fatalf("%d-entry spec must accept, got %v", maxPortRanges, err)
	}
}

// VALIDATES: NOTE #4 -- overlapping or adjacent ranges are rejected at
// parse with a message naming both ranges. Nftables would refuse the
// anonymous interval set at flush; surfacing the conflict at verify is
// a better UX.
func TestParsePortSpecOverlapAndAdjacency(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want string
	}{
		{"overlap", "1-10,5-15", "overlaps"},
		{"duplicate single", "22,22", "overlaps"},
		{"duplicate range", "10-20,10-20", "overlaps"},
		{"adjacent", "1-10,11-20", "adjacent"},
		{"contains", "1-100,50", "overlaps"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePortSpec(tt.spec)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q in error, got %v", tt.want, err)
			}
		})
	}
}

// VALIDATES: NOTE #5 -- the snapshot stored by StoreLastApplied is
// independent of the caller's backing array. Mutating a Ranges slice on
// the source config after Store must not alter what LastApplied() later
// returns. Contract for readers (`returned slice is immutable`) is
// enforced separately by convention; this test covers the other
// direction -- writers cannot corrupt the snapshot through aliasing.
func TestStoreLastAppliedRangesIndependent(t *testing.T) {
	src := []Table{{
		Name:   "ze_test",
		Family: FamilyInet,
		Chains: []Chain{{
			Name: "c",
			Terms: []Term{{
				Name: "t",
				Matches: []Match{MatchDestinationPort{
					Ranges: []PortRange{{Lo: 80, Hi: 80}, {Lo: 443, Hi: 443}},
				}},
				Actions: []Action{Accept{}},
			}},
		}},
	}}
	StoreLastApplied(src)
	t.Cleanup(func() { StoreLastApplied(nil) })

	// Mutate the Ranges slice we passed to Store. Before the accessor
	// was taught to clone Ranges, this mutation propagated into the
	// stored snapshot via the shared backing array.
	srcMatch, ok := src[0].Chains[0].Terms[0].Matches[0].(MatchDestinationPort)
	if !ok {
		t.Fatalf("source match type = %T, want MatchDestinationPort", src[0].Chains[0].Terms[0].Matches[0])
	}
	srcMatch.Ranges[0] = PortRange{Lo: 9999, Hi: 9999}

	snap := LastApplied()
	m, ok := snap[0].Chains[0].Terms[0].Matches[0].(MatchDestinationPort)
	if !ok {
		t.Fatalf("snap match type = %T, want MatchDestinationPort", snap[0].Chains[0].Terms[0].Matches[0])
	}
	if m.Ranges[0].Lo != 80 || m.Ranges[0].Hi != 80 {
		t.Fatalf("snap Ranges[0] = %+v, want {80 80}; accessor did not clone the Ranges slice", m.Ranges[0])
	}
}

// VALIDATES: gap-4 -- base chain priority outside [-400, 400] rejects
// at verify with a message naming the valid range. The kernel would
// silently clamp values beyond its reserved regions, so a 500 ends up
// in a hook order the operator did not ask for; surfacing it offline
// is the cheaper diagnostic.
// PREVENTS: operator writing `priority 500` expecting a late evaluator
// and finding rules fire earlier than intended.
func TestValidateChainPriorityRange(t *testing.T) {
	tests := []struct {
		name     string
		priority int32
		ok       bool
	}{
		{"min valid", -400, true},
		{"max valid", 400, true},
		{"zero", 0, true},
		{"just below min", -401, false},
		{"just above max", 401, false},
		{"far below", -500, false},
		{"far above", 500, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(FamilyInet)
			tbl.Chains[0].Priority = tt.priority
			err := ValidateTables([]Table{tbl})
			if tt.ok {
				if err != nil {
					t.Fatalf("priority %d must accept, got %v", tt.priority, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "out of range") {
				t.Fatalf("priority %d must reject with out-of-range message, got %v",
					tt.priority, err)
			}
		})
	}
}

// VALIDATES: gap-4 -- non-base (regular) chains ignore priority range
// because the field is only meaningful for base chains. A regular chain
// with Priority=5000 is harmless (the leaf is unused) and must verify.
func TestValidateChainPriorityIgnoredOnRegularChain(t *testing.T) {
	tbl := makeTable(FamilyInet)
	tbl.Chains[0].IsBase = false
	tbl.Chains[0].Type = 0
	tbl.Chains[0].Hook = 0
	tbl.Chains[0].Policy = 0
	tbl.Chains[0].Priority = 9999
	if err := ValidateTables([]Table{tbl}); err != nil {
		t.Fatalf("regular chain with Priority=9999 must accept, got %v", err)
	}
}

// VALIDATES: review-1 -- MatchInSet where the field and the set's
// element type disagree rejects at verify. Previously the mismatch
// reached the lowering layer (lowerMatchInSet) at Apply; now `ze
// config validate` surfaces it before the daemon ever starts.
// PREVENTS: operator configuring `source-address "@voip-ports"`
// where voip-ports is an inet_service set and getting a
// "firewall config apply" error on the next reload.
func TestValidateSetFieldMatch(t *testing.T) {
	tests := []struct {
		name    string
		field   SetFieldType
		setType SetType
		wantErr string
	}{
		{"src addr + ipv4 set ok", SetFieldSourceAddr, SetTypeIPv4, ""},
		{"src addr + ipv6 set ok", SetFieldSourceAddr, SetTypeIPv6, ""},
		{"src addr + inet-service set rejects", SetFieldSourceAddr, SetTypeInetService, "expects an ipv4/ipv6 set"},
		{"src addr + mark set rejects", SetFieldSourceAddr, SetTypeMark, "expects an ipv4/ipv6 set"},
		{"dst addr + ifname set rejects", SetFieldDestAddr, SetTypeIfname, "expects an ipv4/ipv6 set"},
		{"src port + inet-service set ok", SetFieldSourcePort, SetTypeInetService, ""},
		{"src port + ipv4 set rejects", SetFieldSourcePort, SetTypeIPv4, "expects an inet-service set"},
		{"dst port + mark set rejects", SetFieldDestPort, SetTypeMark, "expects an inet-service set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(FamilyInet)
			tbl.Sets = []Set{{Name: "s", Type: tt.setType}}
			tbl.Chains[0].Terms[0].Matches = []Match{
				MatchInSet{SetName: "s", MatchField: tt.field},
			}
			err := ValidateTables([]Table{tbl})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected accept, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

// VALIDATES: review-4 -- MatchInSet address-field must have a set
// whose element family matches the parent table family (inet accepts
// both, ip rejects ipv6 sets, ip6 rejects ipv4 sets). Without this
// gate, `source-address "@ipv4set"` in an ip6 table would lower to
// Payload(Network,12,4) and read the middle of the IPv6 source addr.
// PREVENTS: silent cross-family misfires; `ze config validate`
// catches them before reload.
func TestValidateSetFamilyCompat(t *testing.T) {
	tests := []struct {
		name    string
		family  TableFamily
		setType SetType
		wantErr string
	}{
		{"inet + ipv4 set ok", FamilyInet, SetTypeIPv4, ""},
		{"inet + ipv6 set ok", FamilyInet, SetTypeIPv6, ""},
		{"ip + ipv4 set ok", FamilyIP, SetTypeIPv4, ""},
		{"ip + ipv6 set rejects", FamilyIP, SetTypeIPv6, "invalid in family ip"},
		{"ip6 + ipv6 set ok", FamilyIP6, SetTypeIPv6, ""},
		{"ip6 + ipv4 set rejects", FamilyIP6, SetTypeIPv4, "invalid in family ip6"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(tt.family)
			tbl.Sets = []Set{{Name: "s", Type: tt.setType}}
			tbl.Chains[0].Terms[0].Matches = []Match{
				MatchInSet{SetName: "s", MatchField: SetFieldSourceAddr},
			}
			err := ValidateTables([]Table{tbl})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected accept, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

// VALIDATES: review-3 -- an unknown SetFieldType rejects at verify.
// The set of valid fields is finite; a future enum addition that
// bypasses validateSetFieldMatch should reject loudly rather than
// silently accept.
func TestValidateUnknownSetField(t *testing.T) {
	tbl := makeTable(FamilyInet)
	tbl.Sets = []Set{{Name: "s", Type: SetTypeIPv4}}
	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInSet{SetName: "s", MatchField: SetFieldType(99)},
	}
	err := ValidateTables([]Table{tbl})
	if err == nil || !strings.Contains(err.Error(), "unknown set field") {
		t.Fatalf("expected unknown-field rejection, got %v", err)
	}
}

// VALIDATES: gap-2 -- MatchDSCP in an ip6/arp/bridge/netdev table
// rejects at verify. DSCP lives in the IPv4 TOS byte; lowering reads
// offset 1 of the network header, which in IPv6 is the low nibble of
// the traffic-class byte plus the high nibble of flow-label. In arp/
// bridge/netdev the payload is not even IP. Before this guard, the
// rule was accepted and silently misfired.
// PREVENTS: an operator writing `from { dscp ef; }` in an `ip6` table
// and wondering why matching traffic is never classified.
func TestValidateDSCPMatchFamily(t *testing.T) {
	tests := []struct {
		name    string
		family  TableFamily
		wantErr string // empty = accept
	}{
		{"ip accepts", FamilyIP, ""},
		{"inet accepts", FamilyInet, ""},
		{"ip6 rejects", FamilyIP6, "dscp match is IPv4-only"},
		{"arp rejects", FamilyARP, "dscp match is IPv4-only"},
		{"bridge rejects", FamilyBridge, "dscp match is IPv4-only"},
		{"netdev rejects", FamilyNetdev, "dscp match is IPv4-only"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(tt.family)
			tbl.Chains[0].Terms[0].Matches = []Match{MatchDSCP{Value: 46}}
			err := ValidateTables([]Table{tbl})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected accept, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

// VALIDATES: gap-3 -- SetDSCP is permitted in `inet` tables (not only
// `ip`) because an `inet` chain dispatches IPv4 packets to the same
// header layout that lowerSetDSCP encodes. Before this fix, an `inet`
// table with a `then { dscp-set ... }` action rejected at verify even
// though the expression would work on the IPv4 path.
// PREVENTS: operator being forced to duplicate filter rules into a
// second ip-only table just to reclassify DSCP.
func TestValidateSetDSCPInet(t *testing.T) {
	tests := []struct {
		name    string
		family  TableFamily
		wantErr string // empty = accept
	}{
		{"ip accepts", FamilyIP, ""},
		{"inet accepts", FamilyInet, ""},
		{"ip6 rejects", FamilyIP6, "dscp-set is IPv4-only"},
		{"arp rejects", FamilyARP, "dscp-set is IPv4-only"},
		{"bridge rejects", FamilyBridge, "dscp-set is IPv4-only"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(tt.family)
			tbl.Chains[0].Terms[0].Actions = []Action{SetDSCP{Value: 34}}
			err := ValidateTables([]Table{tbl})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected accept, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

// VALIDATES: SetTCPMSS family restrictions: ip, ip6, inet accept;
// arp, bridge, netdev reject.
func TestValidateSetTCPMSSFamily(t *testing.T) {
	tests := []struct {
		name    string
		family  TableFamily
		wantErr string
	}{
		{"ip accepts", FamilyIP, ""},
		{"ip6 accepts", FamilyIP6, ""},
		{"inet accepts", FamilyInet, ""},
		{"arp rejects", FamilyARP, "tcp-mss-set requires family ip, ip6, or inet"},
		{"bridge rejects", FamilyBridge, "tcp-mss-set requires family ip, ip6, or inet"},
		{"netdev rejects", FamilyNetdev, "tcp-mss-set requires family ip, ip6, or inet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(tt.family)
			tbl.Chains[0].Terms[0].Actions = []Action{SetTCPMSS{Size: 1400}}
			err := ValidateTables([]Table{tbl})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected accept, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

// itoa is a tiny, allocation-free uint -> decimal helper used by the
// cap boundary test. strconv import would be the only other consumer,
// not worth it here.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [6]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
