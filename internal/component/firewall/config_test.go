package firewall

import (
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: AC-11 "Config table name wan produces Table with Name ze_wan".
// PREVENTS: missing ze_ prefix on table names.
func TestTableNamePrefix(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet"}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
	if tables[0].Name != "ze_wan" {
		t.Errorf("Table.Name = %q, want %q", tables[0].Name, "ze_wan")
	}
}

// VALIDATES: AC-1 "Config term with destination port 80 parsed to MatchDestinationPort(80)".
// PREVENTS: from-block keyword parsing broken.
func TestParseFromDestinationPort(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"type":"filter","hook":"input","priority":"0","policy":"drop","term":{"allow-http":{"from":{"destination-port":"80"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	chain := tables[0].Chains[0]
	if len(chain.Terms) != 1 {
		t.Fatalf("got %d terms, want 1", len(chain.Terms))
	}
	term := chain.Terms[0]
	if len(term.Matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(term.Matches))
	}
	m, ok := term.Matches[0].(MatchDestinationPort)
	if !ok {
		t.Fatalf("match type = %T, want MatchDestinationPort", term.Matches[0])
	}
	if len(m.Ranges) != 1 || m.Ranges[0].Lo != 80 || m.Ranges[0].Hi != 80 {
		t.Errorf("MatchDestinationPort.Ranges = %+v, want [{80 80}]", m.Ranges)
	}
}

// VALIDATES: AC-2 "Config term with source address 10.0.0.0/8 parsed to MatchSourceAddress".
// PREVENTS: source address parsing broken.
func TestParseFromSourceAddress(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"block-rfc1918":{"from":{"source-address":"10.0.0.0/8"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchSourceAddress)
	if !ok {
		t.Fatalf("match type = %T, want MatchSourceAddress", term.Matches[0])
	}
	want := netip.MustParsePrefix("10.0.0.0/8")
	if m.Prefix != want {
		t.Errorf("Prefix = %v, want %v", m.Prefix, want)
	}
}

// VALIDATES: AC-3 "Config term with connection state established,related".
// PREVENTS: connection state bitmask parsing broken.
func TestParseFromConnState(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"allow-est":{"from":{"connection-state":"established,related"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchConnState)
	if !ok {
		t.Fatalf("match type = %T, want MatchConnState", term.Matches[0])
	}
	want := ConnStateEstablished | ConnStateRelated
	if m.States != want {
		t.Errorf("States = %d, want %d", m.States, want)
	}
}

// VALIDATES: AC-7 "Config terms with accept, drop, reject".
// PREVENTS: verdict action parsing broken.
func TestParseThenVerdicts(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"accept":""}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	if len(term.Actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(term.Actions))
	}
	if _, ok := term.Actions[0].(Accept); !ok {
		t.Errorf("action type = %T, want Accept", term.Actions[0])
	}
}

func TestParseThenDrop(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"drop":""}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	if _, ok := term.Actions[0].(Drop); !ok {
		t.Errorf("action type = %T, want Drop", term.Actions[0])
	}
}

func TestParseThenReject(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"reject":{"with":"icmp","code":"3"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	r, ok := term.Actions[0].(Reject)
	if !ok {
		t.Fatalf("action type = %T, want Reject", term.Actions[0])
	}
	if r.Type != "icmp" || r.Code != 3 {
		t.Errorf("Reject = {%q %d}, want {icmp 3}", r.Type, r.Code)
	}
}

// VALIDATES: AC-6 "Config term with mark set 0x10".
// PREVENTS: modifier parsing broken.
func TestParseThenMarkSet(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"mark-set":{"value":"0x10"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	if len(term.Actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(term.Actions))
	}
	sm, ok := term.Actions[0].(SetMark)
	if !ok {
		t.Fatalf("action type = %T, want SetMark", term.Actions[0])
	}
	if sm.Value != 0x10 {
		t.Errorf("SetMark.Value = 0x%x, want 0x10", sm.Value)
	}
}

// VALIDATES: AC-4 "Config term with limit rate 10/second".
// PREVENTS: rate limit parsing broken.
func TestParseThenLimitRate(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"limit-rate":{"rate":"10/second","burst":"5"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	lim, ok := term.Actions[0].(Limit)
	if !ok {
		t.Fatalf("action type = %T, want Limit", term.Actions[0])
	}
	if lim.Rate != 10 || lim.Unit != "second" || lim.Burst != 5 {
		t.Errorf("Limit = {Rate:%d Unit:%s Burst:%d}, want {10 second 5}", lim.Rate, lim.Unit, lim.Burst)
	}
}

// VALIDATES: gap-8 -- parseNATSpec accepts `<addr>-<addr>` and
// `<addr>-<addr>:<port>` / `<addr>-<addr>:<portLo>-<portHi>` forms
// and surfaces a non-zero AddressEnd when the operator typed a
// range. A zero AddressEnd means single-address NAT, unchanged
// behavior.
// PREVENTS: an operator typing `snat { to "10.0.0.1-10.0.0.10"; }`
// and getting an SNAT that only rewrites to 10.0.0.1.
func TestParseNATAddressRange(t *testing.T) {
	tests := []struct {
		spec        string
		wantLo      string
		wantHi      string
		wantPort    uint16
		wantPortEnd uint16
	}{
		{"10.0.0.1-10.0.0.10", "10.0.0.1", "10.0.0.10", 0, 0},
		{"10.0.0.1-10.0.0.10:80", "10.0.0.1", "10.0.0.10", 80, 0},
		{"10.0.0.1-10.0.0.10:1024-2048", "10.0.0.1", "10.0.0.10", 1024, 2048},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			addr, addrEnd, port, portEnd, err := parseNATSpec(tt.spec)
			if err != nil {
				t.Fatalf("parseNATSpec: %v", err)
			}
			if addr.String() != tt.wantLo {
				t.Errorf("addr = %s, want %s", addr, tt.wantLo)
			}
			if addrEnd.String() != tt.wantHi {
				t.Errorf("addrEnd = %s, want %s", addrEnd, tt.wantHi)
			}
			if port != tt.wantPort || portEnd != tt.wantPortEnd {
				t.Errorf("port/portEnd = %d/%d, want %d/%d", port, portEnd, tt.wantPort, tt.wantPortEnd)
			}
		})
	}
}

// VALIDATES: gap-8 -- single-address NAT still parses without
// populating AddressEnd (netip.Addr zero value). Regression check
// that the address-range branch does not break the existing forms.
// behavior.
func TestParseNATSpecSingleAddressUnchanged(t *testing.T) {
	addr, addrEnd, port, portEnd, err := parseNATSpec("10.0.0.1:80")
	if err != nil {
		t.Fatalf("parseNATSpec: %v", err)
	}
	if addr.String() != "10.0.0.1" {
		t.Errorf("addr = %s, want 10.0.0.1", addr)
	}
	if addrEnd.IsValid() {
		t.Errorf("addrEnd = %s, want invalid (single-addr)", addrEnd)
	}
	if port != 80 || portEnd != 0 {
		t.Errorf("port/portEnd = %d/%d, want 80/0", port, portEnd)
	}
}

// VALIDATES: gap-8 -- mixed IPv4/IPv6 bounds and inverted ranges
// reject with a clear message. The kernel would silently treat a
// backwards range as empty, so offline rejection gives the operator
// a fighting chance.
func TestParseNATSpecRangeErrors(t *testing.T) {
	tests := []struct {
		spec string
		want string
	}{
		{"10.0.0.10-10.0.0.1", "inverted"}, // hi < lo
		{"not-an-addr-10.0.0.1", "invalid NAT address"},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			_, _, _, _, err := parseNATSpec(tt.spec)
			if err == nil {
				t.Fatalf("expected error for %q", tt.spec)
			}
			// The "inverted" case actually carries "is below"; match
			// on any of the expected tokens so the test documents both
			// forms without false-positive-ing on incidental overlap.
			switch tt.want {
			case "inverted":
				if !strings.Contains(err.Error(), "below") && !strings.Contains(err.Error(), "inverted") {
					t.Errorf("error %q does not indicate inversion", err)
				}
			default:
				if !strings.Contains(err.Error(), tt.want) {
					t.Errorf("error %q does not contain %q", err, tt.want)
				}
			}
		})
	}
}

// VALIDATES: gap-7 -- `N<suffix>/<time>` parses into a Limit with
// Dimension=RateDimensionBytes and Rate scaled by the suffix. Each
// prefix (bytes, kbytes, mbytes, gbytes) multiplies by the matching
// power of 1024 so `1mbytes/second` carries 1048576 in Rate.
// PREVENTS: byte-rate specs silently collapsing to packet rates at
// lowering time.
func TestParseRateSpecBytes(t *testing.T) {
	tests := []struct {
		spec     string
		wantRate uint64
		wantUnit string
	}{
		{"500bytes/second", 500, "second"},
		{"1kbytes/second", 1024, "second"},
		{"2kbytes/minute", 2 * 1024, "minute"},
		{"1mbytes/second", 1024 * 1024, "second"},
		{"500kbytes/minute", 500 * 1024, "minute"},
		{"1gbytes/hour", 1024 * 1024 * 1024, "hour"},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			lim, err := parseRateSpec(tt.spec)
			if err != nil {
				t.Fatalf("parseRateSpec(%q): %v", tt.spec, err)
			}
			if lim.Rate != tt.wantRate {
				t.Errorf("Rate = %d, want %d", lim.Rate, tt.wantRate)
			}
			if lim.Unit != tt.wantUnit {
				t.Errorf("Unit = %q, want %q", lim.Unit, tt.wantUnit)
			}
			if lim.Dimension != RateDimensionBytes {
				t.Errorf("Dimension = %d, want RateDimensionBytes (%d)", lim.Dimension, RateDimensionBytes)
			}
		})
	}
}

// VALIDATES: gap-7 -- packet rate (plain `N/unit`) keeps Dimension=
// RateDimensionPackets so lowerLimit picks LimitTypePkts, not PktBytes.
func TestParseRateSpecPackets(t *testing.T) {
	lim, err := parseRateSpec("10/second")
	if err != nil {
		t.Fatalf("parseRateSpec: %v", err)
	}
	if lim.Rate != 10 || lim.Unit != "second" || lim.Dimension != RateDimensionPackets {
		t.Errorf("Limit = {%d %q Dim=%d}, want {10 second Packets=%d}",
			lim.Rate, lim.Unit, lim.Dimension, RateDimensionPackets)
	}
}

// VALIDATES: gap-7 -- unknown suffix rejects at parse with a message
// naming the valid set.
// PREVENTS: operator typo `1mb/second` (missing trailing `ytes`)
// being accepted and silently rewritten as `1/second` packet rate.
func TestParseRateSpecInvalid(t *testing.T) {
	tests := []string{
		"10/",            // missing unit
		"/second",        // missing number
		"10/fortnight",   // bogus time unit
		"1pbytes/second", // unsupported prefix
		"10mb/second",    // missing `ytes`
	}
	for _, spec := range tests {
		t.Run(spec, func(t *testing.T) {
			if _, err := parseRateSpec(spec); err == nil {
				t.Fatalf("expected parse error for %q", spec)
			}
		})
	}
}

// VALIDATES: AC-5 "Config term with log prefix".
// PREVENTS: log action parsing broken.
func TestParseThenLog(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"log":{"prefix":"DROP: "}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	lg, ok := term.Actions[0].(Log)
	if !ok {
		t.Fatalf("action type = %T, want Log", term.Actions[0])
	}
	if lg.Prefix != "DROP: " {
		t.Errorf("Log.Prefix = %q, want %q", lg.Prefix, "DROP: ")
	}
	// Absent leaves must leave the optional fields as nil so the lowerer
	// knows not to emit their NFTA_LOG_* bits.
	if lg.Level != nil {
		t.Errorf("Log.Level = %v (not nil); the operator did not set level", lg.Level)
	}
	if lg.Group != nil {
		t.Errorf("Log.Group = %v (not nil); the operator did not set group", lg.Group)
	}
	if lg.Snaplen != nil {
		t.Errorf("Log.Snaplen = %v (not nil); the operator did not set snaplen", lg.Snaplen)
	}
}

// VALIDATES: Category A risk 2 -- an explicit `level 0` in config
// survives parsing as a non-nil pointer whose target is 0, so the
// lowerer can distinguish "operator asked for emerg" from "operator
// did not set level" (nil). Same contract for Group and Snaplen.
// PREVENTS: operator writes `log { level "0"; }` and the kernel logs
// at warning because 0 was mistaken for "not set".
func TestParseThenLogExplicitLevelZero(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"log":{"prefix":"EMERG: ","level":"0"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	lg, ok := tables[0].Chains[0].Terms[0].Actions[0].(Log)
	if !ok {
		t.Fatalf("action type = %T, want Log", tables[0].Chains[0].Terms[0].Actions[0])
	}
	if lg.Level == nil {
		t.Fatal("Log.Level is nil; explicit \"0\" must survive as a non-nil pointer")
	}
	if *lg.Level != 0 {
		t.Errorf("*Log.Level = %d, want 0 (emerg)", *lg.Level)
	}
}

// VALIDATES: parseLogAction reads Group and Snaplen from JSON into
// non-nil pointers. The YANG schema does not expose these leaves yet
// (see docs/contributing follow-up), but the parser is ready so they
// can land as a pure YANG + regression-test pair.
func TestParseThenLogGroupAndSnaplen(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"log":{"group":"7","snaplen":"128"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	lg, ok := tables[0].Chains[0].Terms[0].Actions[0].(Log)
	if !ok {
		t.Fatalf("action type = %T, want Log", tables[0].Chains[0].Terms[0].Actions[0])
	}
	if lg.Group == nil || *lg.Group != 7 {
		t.Errorf("Log.Group = %v, want *7", lg.Group)
	}
	if lg.Snaplen == nil || *lg.Snaplen != 128 {
		t.Errorf("Log.Snaplen = %v, want *128", lg.Snaplen)
	}
}

// VALIDATES: invalid numeric strings in log fields produce a parse
// error naming the field, not a silent zero value.
func TestParseThenLogRejectsInvalidNumbers(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "level",
			data: `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"log":{"level":"abc"}}}}}}}}}}`,
			want: "level",
		},
		{
			name: "group",
			data: `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"log":{"group":"99999"}}}}}}}}}}`,
			want: "group",
		},
		{
			name: "snaplen",
			data: `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"log":{"snaplen":"-1"}}}}}}}}}}`,
			want: "snaplen",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFirewallConfig(tt.data)
			if err == nil {
				t.Fatalf("expected error mentioning %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

// VALIDATES: P0 -- a `set` block with static `elements` populates
// `Set.Elements`, which the backend then feeds into nftables.AddSet.
// Before this fix, YANG had no elements leaf-list, the parser did
// not read any, and `ze firewall show group <name>` always listed
// zero members.
// PREVENTS: operator config with `elements = { ... }` silently
// becoming an empty set at Apply.
func TestParseSetElements(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","set":{"blocked":{"type":"ipv4","flags-interval":[null],"element":{"10.0.0.0/24":{},"192.168.1.5":{}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables) != 1 || len(tables[0].Sets) != 1 {
		t.Fatalf("tables/sets = %d/%d, want 1/1", len(tables), len(tables[0].Sets))
	}
	s := tables[0].Sets[0]
	if len(s.Elements) != 2 {
		t.Fatalf("Set.Elements len = %d, want 2", len(s.Elements))
	}
	// Map iteration is nondeterministic: collect values into a set-like
	// structure and compare by membership.
	got := map[string]uint32{}
	for _, e := range s.Elements {
		got[e.Value] = e.Timeout
	}
	if _, ok := got["10.0.0.0/24"]; !ok {
		t.Errorf("Elements missing 10.0.0.0/24, got %v", got)
	}
	if _, ok := got["192.168.1.5"]; !ok {
		t.Errorf("Elements missing 192.168.1.5, got %v", got)
	}
	for v, tmo := range got {
		if tmo != 0 {
			t.Errorf("Element %q default timeout = %d, want 0", v, tmo)
		}
	}
}

// VALIDATES: gap-6 -- a per-element `timeout` leaf reaches the
// SetElement.Timeout field so Apply can pass it to the kernel. Before
// the YANG shape change, elements were a leaf-list of strings with no
// way to attach metadata; `flags-timeout` on the set made the kernel
// accept a default timeout but gave operators no way to express an
// actual value.
// PREVENTS: operator writing `element 10.0.0.1 { timeout 3600 }` and
// getting a set entry that never expires.
func TestParseSetElementTimeout(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","set":{"blocked":{"type":"ipv4","flags-timeout":[null],"element":{"10.0.0.1":{"timeout":3600},"10.0.0.2":{}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	s := tables[0].Sets[0]
	if len(s.Elements) != 2 {
		t.Fatalf("Elements len = %d, want 2", len(s.Elements))
	}
	got := map[string]uint32{}
	for _, e := range s.Elements {
		got[e.Value] = e.Timeout
	}
	if got["10.0.0.1"] != 3600 {
		t.Errorf("10.0.0.1 timeout = %d, want 3600", got["10.0.0.1"])
	}
	if got["10.0.0.2"] != 0 {
		t.Errorf("10.0.0.2 timeout = %d, want 0 (no timeout)", got["10.0.0.2"])
	}
}

// VALIDATES: review-5 -- parseSetElements rejects element maps larger
// than maxSetElements rather than pre-allocating an enormous slice.
// A machine-generated or malicious config with millions of keys would
// otherwise trigger a large upfront make() before we noticed the size.
func TestParseSetElementsCapExceeded(t *testing.T) {
	// Build a map larger than the cap. Values are just distinct keys;
	// the parser rejects on size before descending into the map body.
	m := make(map[string]any, maxSetElements+1)
	for i := range maxSetElements + 1 {
		m[itoa(i)] = map[string]any{}
	}
	_, err := parseSetElements("big", m)
	if err == nil || !strings.Contains(err.Error(), "exceeds cap") {
		t.Fatalf("expected cap rejection, got %v", err)
	}
}

// VALIDATES: review-7 -- parseSetElements returns elements in sorted
// order so reload-with-unchanged-config produces a stable SetElement
// slice (Go map iteration is randomized; LastApplied diffs and `show
// firewall` output would otherwise shuffle).
func TestParseSetElementsOrdered(t *testing.T) {
	input := map[string]any{
		"10.0.0.3/32": map[string]any{},
		"10.0.0.1/32": map[string]any{},
		"10.0.0.2/32": map[string]any{},
	}
	out, err := parseSetElements("s", input)
	if err != nil {
		t.Fatalf("parseSetElements: %v", err)
	}
	want := []string{"10.0.0.1/32", "10.0.0.2/32", "10.0.0.3/32"}
	for i, e := range out {
		if e.Value != want[i] {
			t.Errorf("Elements[%d] = %q, want %q", i, e.Value, want[i])
		}
	}
}

// VALIDATES: review-9 -- parseRateSpec rejects rate = 0 via
// ValidateRate. A zero rate would pass nftables into an invalid rule;
// surfacing it offline is a better operator experience.
func TestParseRateSpecZeroRejects(t *testing.T) {
	if _, err := parseRateSpec("0/second"); err == nil {
		t.Fatal("expected rate-zero rejection")
	}
	if _, err := parseRateSpec("0mbytes/minute"); err == nil {
		t.Fatal("expected rate-zero rejection for byte-rate form")
	}
}

// VALIDATES: review-8 -- parseRateSpec rejects numeric prefixes
// exceeding the 20-digit uint64 domain cap, so a multi-megabyte
// digit string is rejected cheaply rather than walked in full.
func TestParseRateSpecDigitCap(t *testing.T) {
	big := strings.Repeat("9", 40) + "/second"
	if _, err := parseRateSpec(big); err == nil {
		t.Fatal("expected digit-cap rejection")
	}
}

// VALIDATES: gap-6 -- unknown leaves inside an element block reject
// with a message naming the offending key.
// PREVENTS: operator typo `timout` (missing `e`) silently dropping the
// timeout at commit.
func TestParseSetElementUnknownLeaf(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","set":{"blocked":{"type":"ipv4","element":{"10.0.0.1":{"timout":60}}}}}}}}`
	_, err := ParseFirewallConfig(data)
	if err == nil || !strings.Contains(err.Error(), "unknown leaf") {
		t.Fatalf("expected unknown-leaf error, got %v", err)
	}
}

// VALIDATES: set without elements still parses (they remain an optional
// leaf-list so dynamically-populated sets still work).
func TestParseSetNoElements(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","set":{"empty":{"type":"ipv4"}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables[0].Sets[0].Elements) != 0 {
		t.Errorf("Elements = %v, want empty", tables[0].Sets[0].Elements)
	}
}

// firstMatch drills down to tables[0].Chains[0].Terms[0].Matches[0]
// with an explicit guard at every level so a parser regression fails
// the test with a descriptive message instead of panicking. Tests that
// expect a single-table / single-chain / single-term / single-match
// config should call this rather than indexing by hand.
func firstMatch(t *testing.T, tables []Table) Match {
	t.Helper()
	if len(tables) == 0 {
		t.Fatal("no tables parsed")
	}
	tbl := tables[0]
	if len(tbl.Chains) == 0 {
		t.Fatalf("table %q: no chains parsed", tbl.Name)
	}
	chain := tbl.Chains[0]
	if len(chain.Terms) == 0 {
		t.Fatalf("table %q chain %q: no terms parsed", tbl.Name, chain.Name)
	}
	term := chain.Terms[0]
	if len(term.Matches) == 0 {
		t.Fatalf("table %q chain %q term %q: no matches parsed", tbl.Name, chain.Name, term.Name)
	}
	return term.Matches[0]
}

// firstAction is the action-side sibling of firstMatch with the same
// guard cascade.
func firstAction(t *testing.T, tables []Table) Action {
	t.Helper()
	if len(tables) == 0 {
		t.Fatal("no tables parsed")
	}
	tbl := tables[0]
	if len(tbl.Chains) == 0 {
		t.Fatalf("table %q: no chains parsed", tbl.Name)
	}
	chain := tbl.Chains[0]
	if len(chain.Terms) == 0 {
		t.Fatalf("table %q chain %q: no terms parsed", tbl.Name, chain.Name)
	}
	term := chain.Terms[0]
	if len(term.Actions) == 0 {
		t.Fatalf("table %q chain %q term %q: no actions parsed", tbl.Name, chain.Name, term.Name)
	}
	return term.Actions[0]
}

// VALIDATES: spec-fw-8 AC-1 / AC-13 -- `icmp-type` accepts symbolic
// nft-style names and numeric fallback, producing MatchICMPType.
// PREVENTS: LNS replacement blocked by missing ICMP type matching.
func TestParseFromICMPType(t *testing.T) {
	tests := []struct {
		name string
		data string
		want uint8
	}{
		{"symbolic echo-request", `{"firewall":{"table":{"t":{"family":"inet","chain":{"c":{"term":{"r":{"from":{"icmp-type":"echo-request"}}}}}}}}}`, 8},
		{"symbolic echo-reply", `{"firewall":{"table":{"t":{"family":"inet","chain":{"c":{"term":{"r":{"from":{"icmp-type":"echo-reply"}}}}}}}}}`, 0},
		{"numeric", `{"firewall":{"table":{"t":{"family":"inet","chain":{"c":{"term":{"r":{"from":{"icmp-type":"42"}}}}}}}}}`, 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tables, err := ParseFirewallConfig(tt.data)
			if err != nil {
				t.Fatalf("ParseFirewallConfig: %v", err)
			}
			got := firstMatch(t, tables)
			m, ok := got.(MatchICMPType)
			if !ok {
				t.Fatalf("match type = %T, want MatchICMPType", got)
			}
			if m.Type != tt.want {
				t.Errorf("Type = %d, want %d", m.Type, tt.want)
			}
		})
	}
}

// VALIDATES: unknown non-numeric ICMP name rejects rather than silently
// defaulting to zero (echo-reply).
func TestParseFromICMPTypeUnknownRejects(t *testing.T) {
	data := `{"firewall":{"table":{"t":{"family":"inet","chain":{"c":{"term":{"r":{"from":{"icmp-type":"bogus"}}}}}}}}}`
	if _, err := ParseFirewallConfig(data); err == nil {
		t.Error("unknown icmp type must reject")
	}
}

// VALIDATES: spec-fw-8 AC-3 / AC-4 -- ICMPv6 symbolic names.
func TestParseFromICMPv6Type(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want uint8
	}{
		{"echo-request", "echo-request", 128},
		{"neighbor-solicit", "nd-neighbor-solicit", 135},
		{"router-advertisement", "nd-router-advert", 134},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := `{"firewall":{"table":{"t":{"family":"inet","chain":{"c":{"term":{"r":{"from":{"icmpv6-type":"` + tt.spec + `"}}}}}}}}}`
			tables, err := ParseFirewallConfig(data)
			if err != nil {
				t.Fatalf("ParseFirewallConfig: %v", err)
			}
			got := firstMatch(t, tables)
			m, ok := got.(MatchICMPv6Type)
			if !ok {
				t.Fatalf("match type = %T, want MatchICMPv6Type", got)
			}
			if m.Type != tt.want {
				t.Errorf("Type = %d, want %d", m.Type, tt.want)
			}
		})
	}
}

// VALIDATES: spec-fw-8 AC-5 / AC-6 / AC-14 -- trailing `*` marks a
// wildcard, the `*` is stripped from Name, and an exact name stays
// exact.
// PREVENTS: LNS NAT rules that use `l2tp*` matching zero interfaces.
func TestParseFromInterfaceWildcard(t *testing.T) {
	tests := []struct {
		name       string
		spec       string
		wantName   string
		wantWild   bool
		outputForm bool
	}{
		{"wildcard input", "l2tp*", "l2tp", true, false},
		{"exact input", "eth0", "eth0", false, false},
		{"wildcard output", "veth*", "veth", true, true},
		{"exact output", "eth1", "eth1", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "input-interface"
			if tt.outputForm {
				key = "output-interface"
			}
			data := `{"firewall":{"table":{"t":{"family":"inet","chain":{"c":{"term":{"r":{"from":{"` + key + `":"` + tt.spec + `"}}}}}}}}}`
			tables, err := ParseFirewallConfig(data)
			if err != nil {
				t.Fatalf("ParseFirewallConfig: %v", err)
			}
			got := firstMatch(t, tables)
			var name string
			var wild bool
			switch m := got.(type) {
			case MatchInputInterface:
				name, wild = m.Name, m.Wildcard
			case MatchOutputInterface:
				name, wild = m.Name, m.Wildcard
			default:
				t.Fatalf("match type = %T, want MatchInputInterface or MatchOutputInterface", got)
			}
			if name != tt.wantName {
				t.Errorf("Name = %q, want %q", name, tt.wantName)
			}
			if wild != tt.wantWild {
				t.Errorf("Wildcard = %v, want %v", wild, tt.wantWild)
			}
		})
	}
}

// VALIDATES: spec-fw-8 AC-7 -- `exclude` in a then-block emits a
// Return verdict so NAT chain evaluation falls back through without
// translation.
// PREVENTS: LNS NAT exclude rules silently dropping through.
func TestParseThenExclude(t *testing.T) {
	data := `{"firewall":{"table":{"t":{"family":"inet","chain":{"c":{"term":{"r":{"then":{"exclude":[null]}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	got := firstAction(t, tables)
	if _, ok := got.(Return); !ok {
		t.Errorf("action type = %T, want Return", got)
	}
}

// VALIDATES: AC-10 "Config term with source address @blocked".
// PREVENTS: set reference parsing broken.
func TestParseFromSetReference(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"from":{"source-address":"@blocked"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchInSet)
	if !ok {
		t.Fatalf("match type = %T, want MatchInSet", term.Matches[0])
	}
	if m.SetName != "blocked" || m.MatchField != SetFieldSourceAddr {
		t.Errorf("MatchInSet = {%q %v}, want {blocked SetFieldSourceAddr}", m.SetName, m.MatchField)
	}
}

// VALIDATES: gap-5 -- `source-port "@name"` parses into a MatchInSet
// with MatchField=SetFieldSourcePort. The YANG port-spec accepts either
// numeric forms or @name; the parser routes to parsePortMatch which
// branches on the `@` prefix.
// PREVENTS: an operator writing `source-port "@voip"` and having the
// parser try to parsePortSpec it, producing an "empty port spec" or
// "invalid port number" error that masks the real intent.
func TestParseFromSourcePortSetReference(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"t":{"from":{"source-port":"@voip"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchInSet)
	if !ok {
		t.Fatalf("match type = %T, want MatchInSet", term.Matches[0])
	}
	if m.SetName != "voip" || m.MatchField != SetFieldSourcePort {
		t.Errorf("MatchInSet = {%q %v}, want {voip SetFieldSourcePort}", m.SetName, m.MatchField)
	}
}

// VALIDATES: gap-5 -- symmetric destination-port handling.
func TestParseFromDestinationPortSetReference(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"t":{"from":{"destination-port":"@web"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchInSet)
	if !ok {
		t.Fatalf("match type = %T, want MatchInSet", term.Matches[0])
	}
	if m.SetName != "web" || m.MatchField != SetFieldDestPort {
		t.Errorf("MatchInSet = {%q %v}, want {web SetFieldDestPort}", m.SetName, m.MatchField)
	}
}

// VALIDATES: gap-5 -- the set-name inside @name must be a valid
// identifier. An empty name (`@`) or one starting with `-` rejects at
// parse so `ze config validate` surfaces the typo before the reference
// check at verify.
func TestParsePortMatchInvalidSetName(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{"empty", "@"},
		{"leading hyphen", "@-bad"},
		{"space inside", "@good bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parsePortMatch(tt.spec, true); err == nil {
				t.Fatalf("expected parse error for spec %q", tt.spec)
			}
		})
	}
}

// VALIDATES: AC-9 "Config term with flow offload @flowtable-name".
// PREVENTS: flow offload parsing broken.
func TestParseThenFlowOffload(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"fwd":{"term":{"offload":{"then":{"flow-offload":{"flowtable":"ft0"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	fo, ok := term.Actions[0].(FlowOffload)
	if !ok {
		t.Fatalf("action type = %T, want FlowOffload", term.Actions[0])
	}
	if fo.FlowtableName != "ft0" {
		t.Errorf("FlowtableName = %q, want %q", fo.FlowtableName, "ft0")
	}
}

// VALIDATES: AC-16 "Base chain parsed with type, hook, priority, policy".
// PREVENTS: base chain fields lost during parsing.
func TestParseBaseChain(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"type":"filter","hook":"input","priority":"0","policy":"drop"}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	chain := tables[0].Chains[0]
	if !chain.IsBase {
		t.Error("expected base chain")
	}
	if chain.Type != ChainFilter {
		t.Errorf("Type = %v, want filter", chain.Type)
	}
	if chain.Hook != HookInput {
		t.Errorf("Hook = %v, want input", chain.Hook)
	}
	if chain.Policy != PolicyDrop {
		t.Errorf("Policy = %v, want drop", chain.Policy)
	}
}

// VALIDATES: AC-13 "Invalid config rejected".
// PREVENTS: invalid families accepted silently.
func TestParseInvalidFamily(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"invalid"}}}}`
	_, err := ParseFirewallConfig(data)
	if err == nil {
		t.Fatal("expected error for invalid family")
	}
}

// VALIDATES: AC-17 "Empty table list returns nil slice".
// PREVENTS: crash on empty firewall section.
func TestParseEmptyFirewall(t *testing.T) {
	data := `{"firewall":{}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables) != 0 {
		t.Errorf("got %d tables, want 0", len(tables))
	}
}

// VALIDATES: AC-18 "No firewall section returns nil".
// PREVENTS: crash when firewall section absent.
func TestParseNoFirewallSection(t *testing.T) {
	data := `{}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if tables != nil {
		t.Errorf("got %v, want nil", tables)
	}
}

// VALIDATES: AC-8 "Config terms with snat/dnat/masquerade".
// PREVENTS: NAT action parsing broken.
func TestParseThenSNAT(t *testing.T) {
	data := `{"firewall":{"table":{"nat":{"family":"inet","chain":{"post":{"term":{"masq":{"then":{"masquerade":""}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	if _, ok := term.Actions[0].(Masquerade); !ok {
		t.Errorf("action type = %T, want Masquerade", term.Actions[0])
	}
}

// VALIDATES: from-block protocol keyword.
// PREVENTS: protocol match parsing broken.
func TestParseFromProtocol(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"from":{"protocol":"tcp"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchProtocol)
	if !ok {
		t.Fatalf("match type = %T, want MatchProtocol", term.Matches[0])
	}
	if m.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", m.Protocol, "tcp")
	}
}

// VALIDATES: then-block counter keyword (container shape after the
// type-empty → presence-container migration).
// PREVENTS: counter action parsing broken.
func TestParseThenCounter(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"counter":{"name":"my-counter"}}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	c, ok := term.Actions[0].(Counter)
	if !ok {
		t.Fatalf("action type = %T, want Counter", term.Actions[0])
	}
	if c.Name != "my-counter" {
		t.Errorf("Counter.Name = %q, want %q", c.Name, "my-counter")
	}
}

// VALIDATES: anonymous counter (no name leaf) parses into Counter{Name:""}.
// PREVENTS: a regression where the presence-container form with empty
// body fails to emit an action.
func TestParseThenCounterAnonymous(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"then":{"counter":{},"accept":[null]}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	acts := tables[0].Chains[0].Terms[0].Actions
	var counter *Counter
	for i := range acts {
		if c, ok := acts[i].(Counter); ok {
			counter = &c
			break
		}
	}
	if counter == nil {
		t.Fatalf("no Counter action in %v", acts)
	}
	if counter.Name != "" {
		t.Errorf("Counter.Name = %q, want empty", counter.Name)
	}
}

// Boundary: port range parsing.
func TestParsePortRange(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"rule1":{"from":{"destination-port":"5060-5061"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	term := tables[0].Chains[0].Terms[0]
	m, ok := term.Matches[0].(MatchDestinationPort)
	if !ok {
		t.Fatalf("match type = %T, want MatchDestinationPort", term.Matches[0])
	}
	if len(m.Ranges) != 1 || m.Ranges[0].Lo != 5060 || m.Ranges[0].Hi != 5061 {
		t.Errorf("Ranges = %+v, want [{5060 5061}]", m.Ranges)
	}
}

// VALIDATES: comma-separated port specs expand to multiple ranges.
// PREVENTS: silent truncation to the first entry.
func TestParsePortList(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"voip":{"from":{"destination-port":"5060-5061,16384-32767"}}}}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	m, ok := tables[0].Chains[0].Terms[0].Matches[0].(MatchDestinationPort)
	if !ok {
		t.Fatalf("match type = %T, want MatchDestinationPort", tables[0].Chains[0].Terms[0].Matches[0])
	}
	want := []PortRange{{Lo: 5060, Hi: 5061}, {Lo: 16384, Hi: 32767}}
	if len(m.Ranges) != len(want) {
		t.Fatalf("Ranges = %+v, want %+v", m.Ranges, want)
	}
	for i, r := range m.Ranges {
		if r != want[i] {
			t.Errorf("Ranges[%d] = %+v, want %+v", i, r, want[i])
		}
	}
}

func TestParseNamedSet(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","set":{"blocked":{"type":"ipv4","flags-interval":""}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables[0].Sets) != 1 {
		t.Fatalf("got %d sets, want 1", len(tables[0].Sets))
	}
	s := tables[0].Sets[0]
	if s.Name != "blocked" {
		t.Errorf("Set.Name = %q, want %q", s.Name, "blocked")
	}
	if s.Type != SetTypeIPv4 {
		t.Errorf("Set.Type = %v, want SetTypeIPv4", s.Type)
	}
	if s.Flags&SetFlagInterval == 0 {
		t.Error("expected SetFlagInterval set")
	}
}

func TestParseFlowtable(t *testing.T) {
	data := `{"firewall":{"table":{"wan":{"family":"inet","flowtable":{"ft0":{"hook":"ingress","priority":"-100","device":["eth0","eth1"]}}}}}}`
	tables, err := ParseFirewallConfig(data)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	if len(tables[0].Flowtables) != 1 {
		t.Fatalf("got %d flowtables, want 1", len(tables[0].Flowtables))
	}
	ft := tables[0].Flowtables[0]
	if ft.Name != "ft0" {
		t.Errorf("Name = %q, want %q", ft.Name, "ft0")
	}
	if ft.Hook != HookIngress {
		t.Errorf("Hook = %v, want ingress", ft.Hook)
	}
	if ft.Priority != -100 {
		t.Errorf("Priority = %d, want -100", ft.Priority)
	}
	if len(ft.Devices) != 2 {
		t.Errorf("Devices len = %d, want 2", len(ft.Devices))
	}
}
