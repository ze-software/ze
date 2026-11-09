package firewall

import (
	"net/netip"
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

// TestValidateMatchIPv6HopLimitBelow proves the hop-limit floor GTSM builds
// for an IPv6 peer is refused where it would read the wrong byte or match
// nothing, and accepted at both ends of its range.
//
// VALIDATES: floor 0 refused; floors 1 and 255 accepted in ip6 and inet; the
// match refused in an ip table, where offset 7 is the IPv4 protocol byte's
// neighbor rather than a hop limit.
// PREVENTS: a GTSM listen-port term built from an unset floor, which would be
// a rule that never fires and protects nothing.
func TestValidateMatchIPv6HopLimitBelow(t *testing.T) {
	tests := []struct {
		name    string
		floor   uint8
		family  TableFamily
		wantErr string // empty = must accept
	}{
		{"floor 1 in inet", 1, FamilyInet, ""},
		{"floor 255 in inet", 255, FamilyInet, ""},
		{"floor 255 in ip6", 255, FamilyIP6, ""},
		{"floor 0 refused", 0, FamilyInet, "hop-limit-below floor is 0"},
		{"ip table refused", 255, FamilyIP, "hop-limit-below match is IPv6-only"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(tt.family)
			tbl.Chains[0].Terms[0].Matches = []Match{MatchIPv6HopLimitBelow{Floor: tt.floor}}
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
	_, err := ParsePortSpec(spec)
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
	if _, err := ParsePortSpec(strings.Join(distinct, ",")); err != nil {
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
			_, err := ParsePortSpec(tt.spec)
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
		{"src addr + unknown set type stays diagnostic", SetFieldSourceAddr, SetType(99), "set type is unknown(99)"},
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
		{"unset family rejects before compatibility", familyUnknown, SetTypeIPv4, "invalid family 0"},
		{"unknown family rejects before compatibility", TableFamily(99), SetTypeIPv4, "invalid family 99"},
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
// `ip`). An inet chain sees both address families, so lowerTerm
// (internal/plugins/firewall/nft/lower_linux.go) programs the term as one
// rule per family there, each writing its own header layout. Before this
// was allowed, an `inet` table with a `then { dscp-set ... }` action
// rejected at verify.
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

// VALIDATES: AC-1 -- a match whose set another registry owner supplies passes
// the set-reference guard. The owners meet only at ApplyAll, so at verify the
// name is absent from the table's own Sets and the match's ProvidedType is the
// only thing that says the set will exist.
// PREVENTS: the firewall guide's IRR workflow being unusable, which is what
// `match references unknown set "irr_v4_AS13335"` made it.
func TestValidateTablesAcceptsIRRTermMatch(t *testing.T) {
	tbl := makeTable(FamilyInet)
	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInSet{SetName: "irr_v4_AS13335", MatchField: SetFieldSourceAddr, ProvidedType: SetTypeIPv4},
	}
	if err := ValidateTables([]Table{tbl}); err != nil {
		t.Fatalf("a provided set must validate, got %v", err)
	}

	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInSet{SetName: "irr_v6_AS13335", MatchField: SetFieldDestAddr, ProvidedType: SetTypeIPv6},
	}
	if err := ValidateTables([]Table{tbl}); err != nil {
		t.Fatalf("a provided ipv6 set must validate, got %v", err)
	}
}

// VALIDATES: AC-4 -- a set name no owner declares is still refused, INCLUDING
// one an operator spelled to look like an IRR set. Only the config parser sets
// ProvidedType, and only for the four IRR leaves, so `source-address
// "@irr_v4_AS13335"` reaches validation with ProvidedType unset.
// PREVENTS: the fix degrading the guard into a name-prefix exemption. Under an
// exemption keyed on the set name, the second half of this test would pass an
// unresolvable match to the backend, where a missing set aborts the reconcile
// for every owner.
func TestValidateTablesStillRefusesUnknownSet(t *testing.T) {
	tbl := makeTable(FamilyInet)
	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInSet{SetName: "typo_set", MatchField: SetFieldSourceAddr},
	}
	err := ValidateTables([]Table{tbl})
	if err == nil || !strings.Contains(err.Error(), `match references unknown set "typo_set"`) {
		t.Fatalf("an undeclared set must be refused, got %v", err)
	}

	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"t":{"from":{"source-address":"@irr_v4_AS13335"},"then":{"drop":""}}}}}}}}}`
	tables, perr := ParseFirewallConfig(data)
	if perr != nil {
		t.Fatalf("ParseFirewallConfig: %v", perr)
	}
	if got := len(tables[0].Chains[0].Terms); got != 1 {
		t.Fatalf("a hand-written set name must not be expanded to a v6 twin, got %d terms", got)
	}
	err = ValidateTables(tables)
	if err == nil || !strings.Contains(err.Error(), `match references unknown set "irr_v4_AS13335"`) {
		t.Fatalf("a hand-written set name in the IRR namespace must still be refused, got %v", err)
	}
}

// VALIDATES: AC-5 -- the set-type agreement check still runs, for a declared
// set and for a provided one alike. The provided match carries the element type
// its owner will supply, so the family and field checks read a real type rather
// than skipping.
// PREVENTS: ProvidedType turning into a blanket accept that lets an ipv4 set
// lower against an IPv6 header at IPv4 offsets.
func TestValidateTablesStillRefusesSetTypeMismatch(t *testing.T) {
	tbl := makeTable(FamilyInet)
	tbl.Sets = []Set{{Name: "ports", Type: SetTypeInetService}}
	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInSet{SetName: "ports", MatchField: SetFieldSourceAddr},
	}
	err := ValidateTables([]Table{tbl})
	if err == nil || !strings.Contains(err.Error(), "expects an ipv4/ipv6 set") {
		t.Fatalf("a field/type disagreement must be refused, got %v", err)
	}

	tbl = makeTable(FamilyIP6)
	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInSet{SetName: "irr_v4_AS13335", MatchField: SetFieldSourceAddr, ProvidedType: SetTypeIPv4},
	}
	err = ValidateTables([]Table{tbl})
	if err == nil || !strings.Contains(err.Error(), "is invalid in family ip6") {
		t.Fatalf("a provided ipv4 set in an ip6 table must be refused, got %v", err)
	}

	tbl = makeTable(FamilyInet)
	tbl.Chains[0].Terms[0].Matches = []Match{
		MatchInSet{SetName: "irr_v4_AS13335", MatchField: SetFieldSourcePort, ProvidedType: SetTypeIPv4},
	}
	err = ValidateTables([]Table{tbl})
	if err == nil || !strings.Contains(err.Error(), "expects an inet-service set") {
		t.Fatalf("a provided address set behind a port field must be refused, got %v", err)
	}
}

// TestValidateRefusesQuotedDestinationOutsideIPv4 is AC-8 of the
// quoted-destination spec and its boundary rows. The match reads a 4-octet
// IPv4 address at a fixed offset inside a quoted IPv4 header. So a table
// family where no nfproto guard is emitted, an unspecified address, and an
// IPv6 address are each refused. A valid IPv4 address is accepted in `ip` and
// `inet`.
func TestValidateRefusesQuotedDestinationOutsideIPv4(t *testing.T) {
	tests := []struct {
		name    string
		family  TableFamily
		addr    netip.Addr
		wantErr string // empty = accept
	}{
		{"ip accepts", FamilyIP, netip.MustParseAddr("192.0.2.2"), ""},
		{"inet accepts", FamilyInet, netip.MustParseAddr("192.0.2.2"), ""},
		{"last valid address accepts", FamilyInet, netip.MustParseAddr("255.255.255.254"), ""},
		{"ip6 rejects", FamilyIP6, netip.MustParseAddr("192.0.2.2"), "icmp-quoted-destination match is IPv4-only"},
		{"arp rejects", FamilyARP, netip.MustParseAddr("192.0.2.2"), "icmp-quoted-destination match is IPv4-only"},
		{"zero address rejects", FamilyInet, netip.Addr{}, "icmp-quoted-destination names no IPv4 address"},
		{"unspecified address rejects", FamilyInet, netip.MustParseAddr("0.0.0.0"), "icmp-quoted-destination names no IPv4 address"},
		{"ipv6 address rejects", FamilyInet, netip.MustParseAddr("2001:db8::1"), "icmp-quoted-destination names no IPv4 address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := makeTable(tt.family)
			tbl.Chains[0].Terms[0].Matches = []Match{MatchICMPErrorQuotedDestination{Addr: tt.addr}}
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

// TestValidateRefusesARPAndNDMatchesOutsideTheirFamily checks the three
// daemon-only matches VRRP's address owner uses. Each reads a fixed offset
// that means something else in another family, so a table of the wrong family
// is refused, and so is a value that names nothing: the zero opcode, a zero or
// unspecified address, and an address of the other IP version.
func TestValidateRefusesARPAndNDMatchesOutsideTheirFamily(t *testing.T) {
	v4 := netip.MustParseAddr("192.0.2.1")
	v6 := netip.MustParseAddr("2001:db8::1")
	tests := []struct {
		name    string
		family  TableFamily
		match   Match
		wantErr string // empty = accept
	}{
		{"arp op reply in arp accepts", FamilyARP, MatchARPOperation{Operation: ARPOperationReply}, ""},
		{"arp op request in arp accepts", FamilyARP, MatchARPOperation{Operation: ARPOperationRequest}, ""},
		{"arp op in inet rejects", FamilyInet, MatchARPOperation{Operation: ARPOperationReply}, "arp-operation match is valid only in family arp"},
		{"arp op zero rejects", FamilyARP, MatchARPOperation{}, "arp-operation names no opcode"},
		{"arp op three rejects", FamilyARP, MatchARPOperation{Operation: 3}, "arp-operation names no opcode"},
		{"arp sender in arp accepts", FamilyARP, MatchARPSenderAddress{Addr: v4}, ""},
		{"arp sender in ip rejects", FamilyIP, MatchARPSenderAddress{Addr: v4}, "arp-sender-address match is valid only in family arp"},
		{"arp sender zero rejects", FamilyARP, MatchARPSenderAddress{}, "arp-sender-address names no IPv4 address"},
		{"arp sender unspecified rejects", FamilyARP, MatchARPSenderAddress{Addr: netip.IPv4Unspecified()}, "arp-sender-address names no IPv4 address"},
		{"arp sender ipv6 rejects", FamilyARP, MatchARPSenderAddress{Addr: v6}, "arp-sender-address names no IPv4 address"},
		{"nd target in ip6 accepts", FamilyIP6, MatchNDTargetAddress{Addr: v6}, ""},
		{"nd target in inet accepts", FamilyInet, MatchNDTargetAddress{Addr: v6}, ""},
		{"nd target in ip rejects", FamilyIP, MatchNDTargetAddress{Addr: v6}, "nd-target-address match is valid only in family ip6 or inet"},
		{"nd target in arp rejects", FamilyARP, MatchNDTargetAddress{Addr: v6}, "nd-target-address match is valid only in family ip6 or inet"},
		{"nd target zero rejects", FamilyIP6, MatchNDTargetAddress{}, "nd-target-address names no IPv6 address"},
		{"nd target unspecified rejects", FamilyIP6, MatchNDTargetAddress{Addr: netip.IPv6Unspecified()}, "nd-target-address names no IPv6 address"},
		{"nd target ipv4 rejects", FamilyIP6, MatchNDTargetAddress{Addr: v4}, "nd-target-address names no IPv6 address"},
		{"nd target v4-mapped rejects", FamilyIP6, MatchNDTargetAddress{Addr: netip.AddrFrom16(v4.As16())}, "nd-target-address names no IPv6 address"},
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

// VALIDATES: AC-4 and AC-6 of spec-home-cpe-9. verify accepts
// tcp-mss-clamp-to-pmtu exactly where nft_rt_validate (net/netfilter/nft_rt.c)
// accepts NFT_RT_TCPMSS: families ip, ip6 and inet, base chains at forward,
// output and postrouting, and regular chains that only such base chains
// reach. Every refusal names the table, the chain, the term, the hook found
// and the fix.
// PREVENTS: a ruleset verify accepted and the kernel then refuses at commit
// with a netlink error, most easily through a regular chain jumped to or
// gone to from prerouting or input (R-1).
func TestValidateClampToPMTUHook(t *testing.T) {
	clamp := Term{Name: "clamp", Actions: []Action{ClampTCPMSSToPMTU{}}}
	base := func(name string, hook ChainHook, terms ...Term) Chain {
		return Chain{Name: name, IsBase: true, Type: ChainFilter, Hook: hook, Policy: PolicyAccept, Terms: terms}
	}
	regular := func(name string, terms ...Term) Chain {
		return Chain{Name: name, Terms: terms}
	}
	enter := func(action Action) Term {
		return Term{Name: "enter", Actions: []Action{action}}
	}
	tests := []struct {
		name    string
		family  TableFamily
		chains  []Chain
		wantErr []string
	}{
		{"forward accepts", FamilyInet, []Chain{base("fwd", HookForward, clamp)}, nil},
		{"output accepts", FamilyIP, []Chain{base("out", HookOutput, clamp)}, nil},
		{"postrouting accepts", FamilyIP6, []Chain{base("post", HookPostrouting, clamp)}, nil},
		{"prerouting refuses", FamilyInet, []Chain{base("pre", HookPrerouting, clamp)},
			[]string{`table "ze_wan"`, `chain "pre"`, `term "clamp"`, "hook prerouting", "move the term to a chain with hook forward or postrouting"}},
		{"input refuses", FamilyInet, []Chain{base("in", HookInput, clamp)},
			[]string{`chain "in"`, "hook input"}},
		{"ingress refuses", FamilyInet, []Chain{base("ing", HookIngress, clamp)},
			[]string{`chain "ing"`, "hook ingress"}},
		{"regular from forward accepts", FamilyInet,
			[]Chain{base("fwd", HookForward, enter(Jump{Target: "mss"})), regular("mss", clamp)}, nil},
		{"regular reached by nobody accepts", FamilyInet,
			[]Chain{base("fwd", HookForward), regular("mss", clamp)}, nil},
		{"regular jumped from prerouting refuses", FamilyInet,
			[]Chain{base("pre", HookPrerouting, enter(Jump{Target: "mss"})), regular("mss", clamp)},
			[]string{`chain "mss"`, `term "clamp"`, "hook prerouting", `base chain "pre"`, "move the term"}},
		{"regular gone to from input refuses", FamilyInet,
			[]Chain{base("in", HookInput, enter(Goto{Target: "mss"})), regular("mss", clamp)},
			[]string{`chain "mss"`, "hook input", `base chain "in"`}},
		{"regular two jumps from prerouting refuses", FamilyInet,
			[]Chain{
				base("fwd", HookForward, enter(Jump{Target: "mid"})),
				base("pre", HookPrerouting, enter(Jump{Target: "mid"})),
				regular("mid", enter(Jump{Target: "mss"})),
				regular("mss", clamp),
			},
			[]string{`chain "mss"`, "hook prerouting", `base chain "pre"`}},
		{"jump cycle terminates", FamilyInet,
			[]Chain{
				base("pre", HookPrerouting, enter(Jump{Target: "a"})),
				regular("a", enter(Jump{Target: "b"})),
				regular("b", enter(Jump{Target: "a"})),
				base("fwd", HookForward, enter(Jump{Target: "mss"})),
				regular("mss", clamp),
			}, nil},
		{"arp refuses", FamilyARP, []Chain{base("fwd", HookForward, clamp)},
			[]string{"tcp-mss-clamp-to-pmtu requires family ip, ip6, or inet"}},
		{"bridge refuses", FamilyBridge, []Chain{base("fwd", HookForward, clamp)},
			[]string{"tcp-mss-clamp-to-pmtu requires family ip, ip6, or inet"}},
		{"netdev refuses", FamilyNetdev, []Chain{base("fwd", HookForward, clamp)},
			[]string{"tcp-mss-clamp-to-pmtu requires family ip, ip6, or inet"}},
		{"with tcp-mss-set refuses", FamilyInet,
			[]Chain{base("fwd", HookForward, Term{Name: "clamp", Actions: []Action{SetTCPMSS{Size: 1400}, ClampTCPMSSToPMTU{}}})},
			[]string{"tcp-mss-set", "tcp-mss-clamp-to-pmtu", "cannot be combined"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbl := Table{Name: "ze_wan", Family: tt.family, Chains: tt.chains}
			err := ValidateTables([]Table{tbl})
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("expected accept, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected a refusal naming %q, got nil", tt.wantErr)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

// VALIDATES: AC-12 -- in a table of family arp every match whose lowering
// reads the IP header or the transport header behind it is refused at verify,
// naming the leaf and "family arp carries no IP header". The same match in an
// ip table is accepted, so the refusal comes from the family and not the match.
// PREVENTS: a literal source-address in an arp table lowered at IPv4 offsets
// 12 and 16, which compares bytes of the ARP header and fires on packets the
// operator never named.
func TestValidateARPFamilyRefusesIPHeaderMatches(t *testing.T) {
	irrSource := irrSetMatch("AS65000", false, true)
	irrDest := irrSetMatch("AS-EXAMPLE", true, false)
	domainSource := domainSetMatch("cdn", true)
	tests := []struct {
		name  string
		match Match
		leaf  string
	}{
		{"source-address", MatchSourceAddress{Prefix: netip.MustParsePrefix("192.0.2.0/24")}, "source-address"},
		{"destination-address", MatchDestinationAddress{Prefix: netip.MustParsePrefix("192.0.2.1/32")}, "destination-address"},
		{"source-port", MatchSourcePort{Ranges: []PortRange{{Lo: 53, Hi: 53}}}, "source-port"},
		{"destination-port", MatchDestinationPort{Ranges: []PortRange{{Lo: 22, Hi: 22}}}, "destination-port"},
		{"protocol", MatchProtocol{Protocol: "tcp"}, "protocol"},
		{"connection-state", MatchConnState{States: ConnStateEstablished}, "connection-state"},
		{"connection-mark", MatchConnMark{Value: 1, Mask: 0xffffffff}, "connection-mark"},
		{"tcp-flags", MatchTCPFlags{Flags: TCPFlagSYN, Mask: TCPFlagSYN}, "tcp-flags"},
		{"address set", MatchInSet{SetName: "v4", MatchField: SetFieldSourceAddr}, "source-address set"},
		{"port set", MatchInSet{SetName: "ports", MatchField: SetFieldDestPort}, "destination-port set"},
		{"irr source", irrSource, "source-asn or source-as-set"},
		{"irr destination", irrDest, "destination-asn or destination-as-set"},
		{"domain group", domainSource, "source-domain-group"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, family := range []TableFamily{FamilyARP, FamilyIP} {
				tbl := makeTable(family)
				tbl.Sets = []Set{
					{Name: "v4", Type: SetTypeIPv4},
					{Name: "ports", Type: SetTypeInetService},
				}
				tbl.Chains[0].Terms[0].Matches = []Match{tt.match}
				err := ValidateTables([]Table{tbl})
				if family == FamilyIP {
					if err != nil {
						t.Fatalf("family ip: expected accept, got %v", err)
					}
					continue
				}
				if err == nil {
					t.Fatalf("family arp: expected refusal, got nil")
				}
				for _, want := range []string{tt.leaf + " match", "family arp carries no IP header"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("family arp: error %q does not contain %q", err, want)
					}
				}
			}
		})
	}
}
