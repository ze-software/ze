package attribute

import (
	"net/netip"
	"testing"
)

// -----------------------------------------------------------------------------
// Element-level AppendText tests (bare value form).
// -----------------------------------------------------------------------------

// VALIDATES: (*Aggregator).AppendText emits the bare element value "<asn>:<ip>"
// using netip.Addr.AppendTo. Same output for 2-byte and 4-byte ASN representations.
// PREVENTS: regressions in filter-text "aggregator" dispatch when attr is formatted.
func TestAppendText_AggregatorElement(t *testing.T) {
	addr := netip.MustParseAddr("192.0.2.1")
	tests := []struct {
		name string
		agg  Aggregator
		want string
	}{
		{"2-byte-asn", Aggregator{ASN: 65001, Address: addr}, "65001:192.0.2.1"},
		{"4-byte-asn", Aggregator{ASN: 4_200_000_001, Address: addr}, "4200000001:192.0.2.1"},
		{"zero-asn", Aggregator{ASN: 0, Address: addr}, "0:192.0.2.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.agg.AppendText(nil))
			if got != tt.want {
				t.Fatalf("AppendText = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: LargeCommunity.AppendText emits "<ga>:<ld1>:<ld2>" using strconv.AppendUint.
// PREVENTS: regressions in filter-text "large-community" rendering.
func TestAppendText_LargeCommunityElement(t *testing.T) {
	tests := []struct {
		name string
		lc   LargeCommunity
		want string
	}{
		{"zero", LargeCommunity{}, "0:0:0"},
		{"asn-ld1-ld2", LargeCommunity{GlobalAdmin: 65001, LocalData1: 100, LocalData2: 200}, "65001:100:200"},
		{"max", LargeCommunity{GlobalAdmin: 0xFFFFFFFF, LocalData1: 0xFFFFFFFF, LocalData2: 0xFFFFFFFF}, "4294967295:4294967295:4294967295"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.lc.AppendText(nil))
			if got != tt.want {
				t.Fatalf("AppendText = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: ExtendedCommunity.AppendText emits 8-byte lowercase hex via hex.AppendEncode.
// PREVENTS: regressions against legacy fmt.Sprintf("%x", ec[:]) output.
func TestAppendText_ExtendedCommunityElement(t *testing.T) {
	ec := ExtendedCommunity{0x00, 0x02, 0xFD, 0xE8, 0x03, 0xE8, 0x00, 0x64}
	got := string(ec.AppendText(nil))
	want := "0002fde803e80064"
	if got != want {
		t.Fatalf("AppendText = %q, want %q", got, want)
	}
}

// -----------------------------------------------------------------------------
// Attribute-level AppendText tests (parity with legacy Format* output).
// -----------------------------------------------------------------------------

// VALIDATES: Origin.AppendText emits the filter-text token "origin <name>"
// for each RFC 4271 value (0=igp, 1=egp, 2=incomplete). Undefined values
// map to "incomplete" to match the legacy FormatOrigin behavior.
// PREVENTS: filter-text contract drift for "origin <igp|egp|incomplete>".
func TestAppendText_Origin(t *testing.T) {
	cases := []struct {
		o    uint8
		want string
	}{
		{0, "origin igp"},
		{1, "origin egp"},
		{2, "origin incomplete"},
		{255, "origin incomplete"},
	}
	for _, c := range cases {
		got := string(Origin(c.o).AppendText(nil))
		if got != c.want {
			t.Errorf("Origin(%d).AppendText = %q, want %q", c.o, got, c.want)
		}
	}
}

// VALIDATES: ASPath.AppendText output matches "as-path " + FormatASPath for
// empty, single, multi-ASN, and AS_SET mixed paths.
// PREVENTS: byte drift in filter-text "as-path [...]" format.
func TestAppendText_ASPath(t *testing.T) {
	tests := []struct {
		name     string
		segments []ASPathSegment
		want     string
	}{
		{"empty", nil, ""},
		{"single", []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}, "as-path 65001"},
		{"multi", []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001, 65002}}}, "as-path [65001 65002]"},
		{
			"multi-segment",
			[]ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{65001, 65002}},
				{Type: ASSet, ASNs: []uint32{65003}},
			},
			"as-path [65001 65002 65003]",
		},
		{"empty-segment", []ASPathSegment{{Type: ASSequence, ASNs: nil}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ASPath{Segments: tt.segments}
			got := string(p.AppendText(nil))
			if got != tt.want {
				t.Fatalf("ASPath.AppendText = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: NextHop.AppendText matches "next-hop <addr>" for IPv4/IPv6,
// and emits nothing for an invalid address.
// PREVENTS: drift in filter-text "next-hop" tokens.
func TestAppendText_NextHop(t *testing.T) {
	tests := []struct {
		name string
		n    *NextHop
		want string
	}{
		{"ipv4", &NextHop{Addr: netip.MustParseAddr("192.0.2.1")}, "next-hop 192.0.2.1"},
		{"ipv6", &NextHop{Addr: netip.MustParseAddr("2001:db8::1")}, "next-hop 2001:db8::1"},
		{"invalid", &NextHop{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.n.AppendText(nil))
			if got != tt.want {
				t.Fatalf("NextHop.AppendText = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: MED.AppendText and LocalPref.AppendText match "<name> <uint>"
// at boundary values (0 and max uint32).
// PREVENTS: byte drift in filter text for med / local-preference tokens.
func TestAppendText_MED_LocalPref(t *testing.T) {
	medTests := []struct {
		val  MED
		want string
	}{
		{0, "med 0"},
		{42, "med 42"},
		{0xFFFFFFFF, "med 4294967295"},
	}
	for _, tt := range medTests {
		if got := string(tt.val.AppendText(nil)); got != tt.want {
			t.Errorf("MED(%d).AppendText = %q, want %q", tt.val, got, tt.want)
		}
	}
	lpTests := []struct {
		val  LocalPref
		want string
	}{
		{0, "local-preference 0"},
		{100, "local-preference 100"},
		{0xFFFFFFFF, "local-preference 4294967295"},
	}
	for _, tt := range lpTests {
		if got := string(tt.val.AppendText(nil)); got != tt.want {
			t.Errorf("LocalPref(%d).AppendText = %q, want %q", tt.val, got, tt.want)
		}
	}
}

// VALIDATES: AtomicAggregate.AppendText emits the bare "atomic-aggregate" token.
// PREVENTS: accidental name/value-pair output for the value-less attribute.
func TestAppendText_AtomicAggregate(t *testing.T) {
	got := string(AtomicAggregate{}.AppendText(nil))
	if got != "atomic-aggregate" {
		t.Fatalf("AtomicAggregate.AppendText = %q, want %q", got, "atomic-aggregate")
	}
}

// VALIDATES: Communities.AppendText well-known community names match legacy
// FormatCommunity (lowercase: no-export, no-advertise, etc., plus blackhole).
// PREVENTS: filter drift when a community is serialized by its well-known name.
func TestAppendText_Community_WellKnown(t *testing.T) {
	cases := []struct {
		comm uint32
		want string
	}{
		{uint32(CommunityNoExport), "community no-export"},
		{uint32(CommunityNoAdvertise), "community no-advertise"},
		{uint32(CommunityNoExportSubconfed), "community no-export-subconfed"},
		{uint32(CommunityNoPeer), "community nopeer"},
		{0xFFFF029A, "community blackhole"},
	}
	for _, c := range cases {
		got := string(Communities{Community(c.comm)}.AppendText(nil))
		if got != c.want {
			t.Errorf("community 0x%08X: AppendText = %q, want %q", c.comm, got, c.want)
		}
	}
}

// VALIDATES: Communities.AppendText plain communities at 16-bit ASN boundaries
// match legacy "<asn>:<val>" output byte-for-byte.
// PREVENTS: overflow/truncation when rendering 65535:65535 etc.
func TestAppendText_Community_Plain(t *testing.T) {
	cases := []struct {
		comm uint32
		want string
	}{
		{0x00000000, "community 0:0"},
		{0x0000FFFF, "community 0:65535"},
		{0xFFFF0000, "community 65535:0"},
		// 0xFFFFFFFF collides with no well-known name, so renders as 65535:65535.
	}
	for _, c := range cases {
		got := string(Communities{Community(c.comm)}.AppendText(nil))
		if got != c.want {
			t.Errorf("community 0x%08X: AppendText = %q, want %q", c.comm, got, c.want)
		}
	}
}

// VALIDATES: Communities.AppendText parity against legacy FormatCommunities
// (empty returns empty buf, single omits brackets, multi uses [...]).
// PREVENTS: byte drift in the filter "community [...]" list form.
func TestAppendText_Communities(t *testing.T) {
	tests := []struct {
		name  string
		comms []Community
		want  string
	}{
		{"empty", nil, ""},
		{"single", []Community{Community(0x12340005)}, "community 4660:5"},
		{"multi", []Community{Community(0xFFFFFF01), Community(0x12340005)}, "community [no-export 4660:5]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(Communities(tt.comms).AppendText(nil))
			if got != tt.want {
				t.Fatalf("Communities.AppendText = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: LargeCommunities.AppendText parity against legacy FormatLargeCommunities.
// PREVENTS: byte drift in "large-community [...]" filter text.
func TestAppendText_LargeCommunities(t *testing.T) {
	lc1 := LargeCommunity{GlobalAdmin: 65001, LocalData1: 1, LocalData2: 2}
	lc2 := LargeCommunity{GlobalAdmin: 65001, LocalData1: 1, LocalData2: 3}
	tests := []struct {
		name  string
		lcs   LargeCommunities
		want  string
		empty bool
	}{
		{"empty", LargeCommunities{}, "", true},
		{"single", LargeCommunities{lc1}, "large-community 65001:1:2", false},
		{"multi", LargeCommunities{lc1, lc2}, "large-community [65001:1:2 65001:1:3]", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.lcs.AppendText(nil))
			if got != tt.want {
				t.Fatalf("LargeCommunities.AppendText = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: ExtendedCommunities.AppendText matches legacy hex output for empty,
// single, and multi-entry lists.
// PREVENTS: byte drift in "extended-community [hex...]" filter text.
func TestAppendText_ExtendedCommunities(t *testing.T) {
	ec1 := ExtendedCommunity{0x00, 0x02, 0xFD, 0xE8, 0x03, 0xE8, 0x00, 0x64}
	ec2 := ExtendedCommunity{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	tests := []struct {
		name string
		ecs  ExtendedCommunities
		want string
	}{
		{"empty", ExtendedCommunities{}, ""},
		{"single", ExtendedCommunities{ec1}, "extended-community 0002fde803e80064"},
		{"multi", ExtendedCommunities{ec1, ec2}, "extended-community [0002fde803e80064 0102030405060708]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.ecs.AppendText(nil))
			if got != tt.want {
				t.Fatalf("ExtendedCommunities.AppendText = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: ClusterList.AppendText matches "cluster-list <id1> <id2> ..." in
// dotted-decimal with no brackets (legacy filter-text format).
// PREVENTS: drift in cluster-list rendering.
func TestAppendText_ClusterList(t *testing.T) {
	tests := []struct {
		name string
		cl   ClusterList
		want string
	}{
		{"empty", ClusterList{}, ""},
		{"single", ClusterList{0x01020304}, "cluster-list 1.2.3.4"},
		{"multi", ClusterList{0x01020304, 0xAABBCCDD}, "cluster-list 1.2.3.4 170.187.204.221"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.cl.AppendText(nil))
			if got != tt.want {
				t.Fatalf("ClusterList.AppendText = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: Aggregator.AppendText element form is "<asn>:<ip>" for both
// 2-byte and 4-byte ASN. Attribute-form prefix is added by dispatch layer.
// PREVENTS: filter-text "aggregator" drift.
func TestAppendText_Aggregator(t *testing.T) {
	addr := netip.MustParseAddr("192.0.2.1")
	a := &Aggregator{ASN: 65001, Address: addr}
	got := string(a.AppendText(nil))
	if got != "65001:192.0.2.1" {
		t.Fatalf("Aggregator.AppendText = %q, want %q", got, "65001:192.0.2.1")
	}
	b := &Aggregator{ASN: 4_200_000_001, Address: addr}
	got = string(b.AppendText(nil))
	if got != "4200000001:192.0.2.1" {
		t.Fatalf("Aggregator.AppendText = %q, want %q", got, "4200000001:192.0.2.1")
	}
}

// VALIDATES: OriginatorID.AppendText emits "originator-id <addr>" for a
// valid IPv4 address and drops silently for an invalid (zero) address.
// PREVENTS: regression on RFC 4456 filter-text contract; ensures the
// "originator-id" token is dispatched end-to-end (the legacy formatSingleAttr
// had no case for OriginatorID, dropping the attribute silently; this test
// pins the new behavior).
func TestAppendText_OriginatorID(t *testing.T) {
	addr := netip.MustParseAddr("1.2.3.4")
	got := string(OriginatorID(addr).AppendText(nil))
	want := "originator-id 1.2.3.4"
	if got != want {
		t.Fatalf("OriginatorID.AppendText = %q, want %q", got, want)
	}
	// Invalid (zero) address drops.
	var zero OriginatorID
	if got := string(zero.AppendText(nil)); got != "" {
		t.Fatalf("OriginatorID(zero).AppendText = %q, want empty", got)
	}
}

// VALIDATES: (*Aggregator).AppendText drops silently when the Address is
// invalid (zero value), matching NextHop's defensive pattern.
// PREVENTS: "aggregator 0:invalid IP" leaking into the space-delimited
// filter text and breaking downstream parsers.
func TestAppendText_Aggregator_InvalidAddress(t *testing.T) {
	a := &Aggregator{ASN: 65001} // Address is the zero value, invalid.
	got := string(a.AppendText(nil))
	if got != "" {
		t.Fatalf("Aggregator with invalid Address: AppendText = %q, want empty", got)
	}
}

// VALIDATES: After an initial AppendText sizes the buffer, a second
// AppendText(buf[:0]) returns the same underlying array (cap unchanged).
// Demonstrates the "reuse without grow" zero-alloc hot path.
// PREVENTS: regression toward append-reallocates-every-call.
func TestAppendText_BufferReuse_NoGrow(t *testing.T) {
	buf := make([]byte, 0, 128)
	p := &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003}}}}
	buf = p.AppendText(buf)
	firstCap := cap(buf)
	buf = p.AppendText(buf[:0])
	if cap(buf) != firstCap {
		t.Fatalf("cap changed after reuse: got %d, want %d (append grew the slice)", cap(buf), firstCap)
	}
}
