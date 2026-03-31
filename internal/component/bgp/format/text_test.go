package format

import (
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// testEncCtx is an empty encoding context for tests (no ADD-PATH).
var testEncCtx = bgpctx.EncodingContextForASN4(true)

// TestFormatStateChange tests state event formatting.
//
// VALIDATES: Peer state changes format correctly for both encodings.
//
// PREVENTS: State events not being delivered to processes.
func TestFormatStateChange(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	tests := []struct {
		name     string
		state    string
		reason   string
		encoding string
		want     string
	}{
		{
			name:     "text established",
			state:    "established",
			encoding: plugin.EncodingText,
			want:     "peer 10.0.0.1 remote as 65001 state established\n",
		},
		{
			name:     "text down no reason",
			state:    "down",
			encoding: plugin.EncodingText,
			want:     "peer 10.0.0.1 remote as 65001 state down\n",
		},
		{
			name:     "text down with reason",
			state:    "down",
			reason:   "tcp-failure",
			encoding: plugin.EncodingText,
			want:     "peer 10.0.0.1 remote as 65001 state down reason tcp-failure\n",
		},
		{
			name:     "json established",
			state:    "established",
			encoding: plugin.EncodingJSON,
			want:     `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","name":"","remote":{"as":65001}},"state":"established"}}` + "\n",
		},
		{
			name:     "json down no reason",
			state:    "down",
			encoding: plugin.EncodingJSON,
			want:     `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","name":"","remote":{"as":65001}},"state":"down"}}` + "\n",
		},
		{
			name:     "json down with reason",
			state:    "down",
			reason:   "notification",
			encoding: plugin.EncodingJSON,
			want:     `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","name":"","remote":{"as":65001}},"state":"down","reason":"notification"}}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStateChange(peer, tt.state, tt.reason, tt.encoding)
			if got != tt.want {
				t.Errorf("FormatStateChange() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPeerJSONNameGroup verifies that peer name and group appear in JSON output when set.
//
// VALIDATES: JSON events include "name" and "group" in the peer object.
// PREVENTS: Peer identity limited to address+asn only.
func TestPeerJSONNameGroup(t *testing.T) {
	tests := []struct {
		name string
		peer plugin.PeerInfo
		want string
	}{
		{
			name: "name only",
			peer: plugin.PeerInfo{
				Address: netip.MustParseAddr("10.0.0.1"),
				PeerAS:  65001,
				Name:    "upstream1",
			},
			want: `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","name":"upstream1","remote":{"as":65001}},"state":"established"}}` + "\n",
		},
		{
			name: "name and group",
			peer: plugin.PeerInfo{
				Address:   netip.MustParseAddr("10.0.0.2"),
				PeerAS:    65002,
				Name:      "peer_east",
				GroupName: "transit",
			},
			want: `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.2","group":"transit","name":"peer_east","remote":{"as":65002}},"state":"established"}}` + "\n",
		},
		{
			name: "group only",
			peer: plugin.PeerInfo{
				Address:   netip.MustParseAddr("10.0.0.3"),
				PeerAS:    65003,
				GroupName: "edge",
			},
			want: `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.3","group":"edge","name":"","remote":{"as":65003}},"state":"established"}}` + "\n",
		},
		{
			name: "no name no group",
			peer: plugin.PeerInfo{
				Address: netip.MustParseAddr("10.0.0.4"),
				PeerAS:  65004,
			},
			want: `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.4","name":"","remote":{"as":65004}},"state":"established"}}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStateChange(tt.peer, "established", "", plugin.EncodingJSON)
			if got != tt.want {
				t.Errorf("FormatStateChange() =\n  %s\nwant:\n  %s", got, tt.want)
			}
		})
	}
}

// TestFormatEOR tests End-of-RIB marker event formatting.
//
// VALIDATES: EOR events format correctly for both encodings with family info.
// PREVENTS: EOR events not being delivered or formatted incorrectly.
func TestFormatEOR(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	tests := []struct {
		name     string
		family   string
		encoding string
		want     string
	}{
		{
			name:     "ipv4/unicast text",
			family:   "ipv4/unicast",
			encoding: plugin.EncodingText,
			want:     "peer 10.0.0.1 remote as 65001 eor ipv4/unicast\n",
		},
		{
			name:     "ipv6/unicast text",
			family:   "ipv6/unicast",
			encoding: plugin.EncodingText,
			want:     "peer 10.0.0.1 remote as 65001 eor ipv6/unicast\n",
		},
		{
			name:     "ipv4/unicast json",
			family:   "ipv4/unicast",
			encoding: plugin.EncodingJSON,
			want:     `{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"address":"10.0.0.1","name":"","remote":{"as":65001}},"eor":{"family":"ipv4/unicast"}}}` + "\n",
		},
		{
			name:     "ipv6/unicast json",
			family:   "ipv6/unicast",
			encoding: plugin.EncodingJSON,
			want:     `{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"address":"10.0.0.1","name":"","remote":{"as":65001}},"eor":{"family":"ipv6/unicast"}}}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatEOR(peer, tt.family, tt.encoding)
			if got != tt.want {
				t.Errorf("FormatEOR(%q, %q)\n  got:  %q\n  want: %q", tt.family, tt.encoding, got, tt.want)
			}
		})
	}
}

// TestFormatCongestion tests congestion event formatting for both encodings.
//
// VALIDATES: Congestion events use correct JSON envelope and text format.
// PREVENTS: Incorrect JSON keys or missing fields in congestion events.
func TestFormatCongestion(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	tests := []struct {
		name      string
		eventType string
		encoding  string
		want      string
	}{
		{
			name:      "congested text",
			eventType: "congested",
			encoding:  plugin.EncodingText,
			want:      "peer 10.0.0.1 remote as 65001 congested\n",
		},
		{
			name:      "resumed text",
			eventType: "resumed",
			encoding:  plugin.EncodingText,
			want:      "peer 10.0.0.1 remote as 65001 resumed\n",
		},
		{
			name:      "congested json",
			eventType: "congested",
			encoding:  plugin.EncodingJSON,
			want:      `{"type":"bgp","bgp":{"message":{"type":"congested"},"peer":{"address":"10.0.0.1","name":"","remote":{"as":65001}}}}` + "\n",
		},
		{
			name:      "resumed json",
			eventType: "resumed",
			encoding:  plugin.EncodingJSON,
			want:      `{"type":"bgp","bgp":{"message":{"type":"resumed"},"peer":{"address":"10.0.0.1","name":"","remote":{"as":65001}}}}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCongestion(peer, tt.eventType, tt.encoding)
			if got != tt.want {
				t.Errorf("FormatCongestion(%q, %q)\n  got:  %q\n  want: %q", tt.eventType, tt.encoding, got, tt.want)
			}
		})
	}
}

// TestFormatMessageText tests text format output.
//
// VALIDATES: Text format uses "peer X update announce nlri ..." syntax.
//
// PREVENTS: Wrong format sent to API processes.
func TestFormatMessageText(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE body with NLRI: 192.168.1.0/24, next-hop 10.0.0.1, origin igp, local-pref 100, as-path [65001 65002]
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0,   // igp
		100, // local-pref
		[]uint32{65001, 65002},
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}
	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingText,
		Format:   plugin.FormatParsed,
	}

	got := FormatMessage(peer, msg, content, "")

	// Format: peer <ip> remote as <asn> <direction> update <id> <attrs> family <family> next-hop <ip> nlri add <prefixes>
	if !strings.Contains(got, "peer 10.0.0.1 remote as 65001 received update") {
		t.Errorf("FormatMessage() =\n%q\nshould contain 'peer 10.0.0.1 remote as 65001 received update'", got)
	}
	if strings.Contains(got, "announce") {
		t.Error("should not contain 'announce' keyword (replaced by family + nlri add)")
	}
	if !strings.Contains(got, "origin igp") {
		t.Error("missing origin")
	}
	if !strings.Contains(got, "path 65001,65002") {
		t.Error("missing comma-separated as-path (short form: path)")
	}
	if !strings.Contains(got, "pref 100") {
		t.Error("missing local-preference (short form: pref)")
	}
	if !strings.Contains(got, "next 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24") {
		t.Error("missing next-hop/nlri add (short form: next)")
	}
}

// TestFormatMessageJSON tests JSON format output.
//
// VALIDATES: JSON uses command-style family → operations format.
//
// PREVENTS: Wrong JSON structure sent to API processes.
func TestFormatMessageJSON(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE body with NLRI
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}
	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	got := FormatMessage(peer, msg, content, "")

	// Check key parts of the ze-bgp JSON JSON structure
	// Outer wrapper: {"type":"bgp","bgp":{...}}
	if !strings.Contains(got, `"type":"bgp"`) {
		t.Error("missing top-level type:bgp")
	}
	if !strings.Contains(got, `"bgp":{`) {
		t.Error("missing bgp payload wrapper")
	}
	// Event type in payload: bgp.type = "update"
	if !strings.Contains(got, `"type":"update"`) {
		t.Error("missing type:update in bgp payload")
	}
	// Direction should be inside message wrapper
	if !strings.Contains(got, `"direction":"received"`) {
		t.Error("missing direction:received in message wrapper")
	}
	if !strings.Contains(got, `"peer":{"address":"10.0.0.1","name":"","remote":{"as":65001}}`) {
		t.Error("missing peer info")
	}
	// NLRIs under "nlri" object with family key
	if !strings.Contains(got, `"nlri":{`) {
		t.Error("missing nlri object")
	}
	if !strings.Contains(got, `"ipv4/unicast":[`) {
		t.Error("missing ipv4/unicast family array")
	}
	if !strings.Contains(got, `"action":"add"`) {
		t.Error("missing action:add")
	}
	if !strings.Contains(got, `"next-hop":"10.0.0.1"`) {
		t.Error("missing next-hop")
	}
	if !strings.Contains(got, `192.168.1.0/24`) {
		t.Error("missing prefix")
	}
}

// buildTestUpdateBodyWithAttrs builds a BGP UPDATE message body with custom attributes.
// Format: withdrawn_len(2) + withdrawn + attr_len(2) + attrs + nlri.
//
//nolint:unparam // origin is a valid parameter even if tests always pass 0
func buildTestUpdateBodyWithAttrs(prefix netip.Prefix, nextHop netip.Addr, origin uint8, localPref uint32, asPath []uint32) []byte {
	var attrs []byte

	// ORIGIN
	if origin <= 2 {
		attrs = append(attrs, 0x40, 0x01, 0x01, origin)
	}

	// AS_PATH
	if len(asPath) > 0 {
		asPathData := make([]byte, 0, 2+4*len(asPath))
		asPathData = append(asPathData, 0x02, byte(len(asPath))) // AS_SEQUENCE
		for _, asn := range asPath {
			b := make([]byte, 4)
			binary.BigEndian.PutUint32(b, asn)
			asPathData = append(asPathData, b...)
		}
		attrs = append(attrs, 0x40, 0x02, byte(len(asPathData)))
		attrs = append(attrs, asPathData...)
	} else {
		// Empty AS_PATH
		attrs = append(attrs, 0x40, 0x02, 0x00)
	}

	// NEXT_HOP (IPv4)
	if nextHop.Is4() {
		b := nextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// LOCAL_PREF
	if localPref > 0 {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, localPref)
		attrs = append(attrs, 0x40, 0x05, 0x04)
		attrs = append(attrs, b...)
	}

	// NLRI (IPv4)
	var nlri []byte
	if prefix.Addr().Is4() {
		bits := prefix.Bits()
		nlri = append(nlri, byte(bits))
		prefixBytes := (bits + 7) / 8
		addr := prefix.Addr().As4()
		nlri = append(nlri, addr[:prefixBytes]...)
	}

	// Build body
	body := make([]byte, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs))) //nolint:gosec // test data
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlri)

	return body
}

// TestFormatNonUpdateRoutesToDedicatedFormatters tests that non-UPDATE messages
// are formatted using dedicated formatters, not just raw hex output.
//
// VALIDATES: OPEN messages are formatted via FormatOpen.
// PREVENTS: API processes receiving raw hex instead of parsed content.
func TestFormatNonUpdateRoutesToDedicatedFormatters(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build OPEN message body: version(1) + AS(2) + hold(2) + router-id(4) + opt-len(1)
	openBody := []byte{
		4,     // version
		0, 42, // AS 42
		0, 180, // hold time 180
		10, 0, 0, 1, // router-id 10.0.0.1
		0, // opt params len
	}

	msg := bgptypes.RawMessage{
		Type:      message.TypeOPEN,
		RawBytes:  openBody,
		Direction: "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingText,
		Format:   plugin.FormatParsed,
	}

	got := FormatMessage(peer, msg, content, "")

	// Should use FormatOpen with uniform header: peer X remote as Y received open <msg-id> router-id R hold-time T cap ...
	if !strings.Contains(got, "peer 10.0.0.1 remote as 42 received open") {
		t.Errorf("FormatMessage() for OPEN =\n%q\nshould contain 'peer 10.0.0.1 remote as 42 received open'", got)
	}
	if !strings.Contains(got, "router-id 10.0.0.1") {
		t.Errorf("FormatMessage() for OPEN =\n%q\nshould contain 'router-id 10.0.0.1'", got)
	}
	if !strings.Contains(got, "hold-time 180") {
		t.Errorf("FormatMessage() for OPEN =\n%q\nshould contain 'hold-time 180'", got)
	}
}

// TestFormatNonUpdateKeepalive tests that KEEPALIVE messages are formatted properly.
//
// VALIDATES: KEEPALIVE produces expected format.
// PREVENTS: KEEPALIVE being shown as raw hex.
func TestFormatNonUpdateKeepalive(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	msg := bgptypes.RawMessage{
		Type:      message.TypeKEEPALIVE,
		RawBytes:  []byte{}, // KEEPALIVE has no body
		Direction: "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingText,
		Format:   plugin.FormatParsed,
	}

	got := FormatMessage(peer, msg, content, "")

	// Should use uniform header: peer X remote as Y received keepalive
	if !strings.Contains(got, "peer 10.0.0.1 remote as 65001 received keepalive") {
		t.Errorf("FormatMessage() for KEEPALIVE =\n%q\nshould contain 'peer 10.0.0.1 remote as 65001 received keepalive'", got)
	}
}

// TestFilterResultZeroValues tests that LOCAL_PREF=0 and MED=0 are included.
//
// VALIDATES: Zero values for LOCAL_PREF and MED are valid and should be output.
// PREVENTS: RFC-valid zero values being filtered out.
func TestFilterResultZeroValues(t *testing.T) {
	ctxID := testEncodingContext()

	// Build UPDATE with LOCAL_PREF=0 and MED=0
	body := buildTestUpdateBodyWithMEDAndLocalPref(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, // origin igp
		0, // LOCAL_PREF = 0 (valid)
		0, // MED = 0 (valid)
	)

	// Create WireUpdate and apply filter
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	wire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}
	if wire == nil {
		t.Fatal("Failed to extract attribute bytes")
		return
	}

	filter := bgpfilter.NewFilterAll()
	nlriFilter := bgpfilter.NewNLRIFilterAll()
	result, err := filter.ApplyToUpdate(wire, body, nlriFilter)
	if err != nil {
		t.Fatalf("ApplyToUpdate failed: %v", err)
	}

	// Check LOCAL_PREF is present (even at 0)
	if _, ok := result.Attributes[attribute.AttrLocalPref]; !ok {
		t.Error("LOCAL_PREF=0 should be in attributes, but is missing")
	}

	// Check MED is present (even at 0)
	if _, ok := result.Attributes[attribute.AttrMED]; !ok {
		t.Error("MED=0 should be in attributes, but is missing")
	}
}

// TestFilterResultBothNextHops tests extraction of both IPv4 and IPv6 next-hops.
//
// VALIDATES: When UPDATE has both IPv4 NLRI and IPv6 MP_REACH_NLRI, both next-hops extracted.
// PREVENTS: Wrong next-hop used for IPv6 prefixes.
func TestFilterResultBothNextHops(t *testing.T) {
	ctxID := testEncodingContext()

	// Build UPDATE with both IPv4 and IPv6 NLRI
	// IPv4 NEXT_HOP: 10.0.0.1
	// IPv6 MP_REACH next-hop: 2001:db8::1
	body := buildTestUpdateBodyWithBothFamilies(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParseAddr("2001:db8::1"),
	)

	// Create WireUpdate and apply filter
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	wire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}
	if wire == nil {
		t.Fatal("Failed to extract attribute bytes")
		return
	}

	filter := bgpfilter.NewFilterAll()
	nlriFilter := bgpfilter.NewNLRIFilterAll()
	result, err := filter.ApplyToUpdate(wire, body, nlriFilter)
	if err != nil {
		t.Fatalf("ApplyToUpdate failed: %v", err)
	}

	// Check next-hops via FamilyNLRI iteration
	announced := result.AnnouncedByFamily(testEncCtx)
	if len(announced) != 2 {
		t.Fatalf("AnnouncedByFamily len = %d, want 2 (IPv4 + IPv6)", len(announced))
	}

	// Find IPv4 and IPv6 families
	var gotIPv4, gotIPv6 bool
	for _, fam := range announced {
		if fam.Family == "ipv4/unicast" {
			gotIPv4 = true
			if fam.NextHop.String() != "10.0.0.1" {
				t.Errorf("IPv4 NextHop = %v, want 10.0.0.1", fam.NextHop)
			}
		}
		if fam.Family == "ipv6/unicast" {
			gotIPv6 = true
			if fam.NextHop.String() != "2001:db8::1" {
				t.Errorf("IPv6 NextHop = %v, want 2001:db8::1", fam.NextHop)
			}
		}
	}
	if !gotIPv4 {
		t.Error("missing ipv4/unicast family")
	}
	if !gotIPv6 {
		t.Error("missing ipv6/unicast family")
	}
}

// TestFilterResultCommunities tests that communities are parsed via AttrsWire.
//
// VALIDATES: COMMUNITY attribute is included in FilterResult.
// PREVENTS: Communities missing from API output.
func TestFilterResultCommunities(t *testing.T) {
	ctxID := testEncodingContext()

	// Build UPDATE with COMMUNITY attribute
	body := buildTestUpdateBodyWithCommunities(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		[]uint32{0xFDE80064}, // 65000:100
	)

	// Create WireUpdate and apply filter
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	wire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}
	if wire == nil {
		t.Fatal("Failed to extract attribute bytes")
		return
	}

	filter := bgpfilter.NewFilterAll()
	nlriFilter := bgpfilter.NewNLRIFilterAll()
	result, err := filter.ApplyToUpdate(wire, body, nlriFilter)
	if err != nil {
		t.Fatalf("ApplyToUpdate failed: %v", err)
	}

	if _, ok := result.Attributes[attribute.AttrCommunity]; !ok {
		t.Error("COMMUNITY should be in attributes, but is missing")
	}
}

// buildTestUpdateBodyWithMEDAndLocalPref builds UPDATE body with explicit MED and LOCAL_PREF.
// Always includes both attributes even when 0.
func buildTestUpdateBodyWithMEDAndLocalPref(prefix netip.Prefix, nextHop netip.Addr, origin uint8, localPref, med uint32) []byte {
	// ORIGIN + AS_PATH (empty)
	var attrs []byte
	attrs = append(attrs, 0x40, 0x01, 0x01, origin, 0x40, 0x02, 0x00)

	// NEXT_HOP (IPv4)
	if nextHop.Is4() {
		b := nextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// MED (always include)
	medBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(medBytes, med)
	attrs = append(attrs, 0x80, 0x04, 0x04) // optional, transitive
	attrs = append(attrs, medBytes...)

	// LOCAL_PREF (always include)
	lpBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lpBytes, localPref)
	attrs = append(attrs, 0x40, 0x05, 0x04)
	attrs = append(attrs, lpBytes...)

	// NLRI (IPv4)
	var nlri []byte
	if prefix.Addr().Is4() {
		bits := prefix.Bits()
		nlri = append(nlri, byte(bits))
		prefixBytes := (bits + 7) / 8
		addr := prefix.Addr().As4()
		nlri = append(nlri, addr[:prefixBytes]...)
	}

	// Build body
	body := make([]byte, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs))) //nolint:gosec // test data
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlri)

	return body
}

// buildTestUpdateBodyWithBothFamilies builds UPDATE with both IPv4 NLRI and IPv6 MP_REACH_NLRI.
//
//nolint:dupl // Test helper, similar structure to buildTestUpdateBodyWithDualIPv4NextHop is intentional
func buildTestUpdateBodyWithBothFamilies(ipv4Prefix netip.Prefix, ipv4NextHop netip.Addr, ipv6Prefix netip.Prefix, ipv6NextHop netip.Addr) []byte {
	// ORIGIN (igp) + AS_PATH (empty)
	var attrs []byte
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00)

	// NEXT_HOP for IPv4
	if ipv4NextHop.Is4() {
		b := ipv4NextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// MP_REACH_NLRI for IPv6
	// AFI=2 (IPv6), SAFI=1 (unicast), NH len=16, next-hop, reserved=0, NLRI
	// Capacity: 4 (header) + 16 (NH) + 1 (reserved) + 1 (prefix len) + 16 (max prefix) = 38
	mpReach := make([]byte, 0, 38)
	mpReach = append(mpReach, 0x00, 0x02, 0x01, 0x10) // AFI IPv6, SAFI unicast, NH len = 16
	nhBytes := ipv6NextHop.As16()
	mpReach = append(mpReach, nhBytes[:]...)

	// Reserved + IPv6 NLRI
	bits := ipv6Prefix.Bits()
	prefixBytes := (bits + 7) / 8
	addr := ipv6Prefix.Addr().As16()
	mpReach = append(mpReach, 0x00, byte(bits))
	mpReach = append(mpReach, addr[:prefixBytes]...)

	// MP_REACH_NLRI attribute (optional, transitive)
	attrs = append(attrs, 0x90, 0x0e, byte(len(mpReach)>>8), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)

	// IPv4 NLRI
	// Capacity: 1 (prefix len) + 4 (max IPv4 prefix bytes) = 5
	nlri := make([]byte, 0, 5)
	bits = ipv4Prefix.Bits()
	nlri = append(nlri, byte(bits))
	prefixBytes = (bits + 7) / 8
	addr4 := ipv4Prefix.Addr().As4()
	nlri = append(nlri, addr4[:prefixBytes]...)

	// Build body
	body := make([]byte, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs))) //nolint:gosec // test data
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlri)

	return body
}

// TestFormatOpenWithDirection tests that OPEN messages include direction.
//
// VALIDATES: FormatOpen uses direction parameter ("sent"/"received").
// PREVENTS: API output missing direction indicator.
func TestFormatOpenWithDirection(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	open := DecodedOpen{
		ASN:      65001,
		RouterID: "1.1.1.1",
		HoldTime: 90,
	}

	tests := []struct {
		name      string
		direction string
		want      string
	}{
		{
			name:      "sent",
			direction: "sent",
			want:      "peer 10.0.0.1 remote as 65001 sent open 42 router-id 1.1.1.1 hold-time 90\n",
		},
		{
			name:      "received",
			direction: "received",
			want:      "peer 10.0.0.1 remote as 65001 received open 42 router-id 1.1.1.1 hold-time 90\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatOpen(peer, open, tt.direction, 42)
			if got != tt.want {
				t.Errorf("FormatOpen() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormatKeepaliveWithDirection tests that KEEPALIVE messages include direction.
//
// VALIDATES: FormatKeepalive uses direction parameter ("sent"/"received").
// PREVENTS: API output missing direction indicator.
func TestFormatKeepaliveWithDirection(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	tests := []struct {
		name      string
		direction string
		want      string
	}{
		{
			name:      "sent",
			direction: "sent",
			want:      "peer 10.0.0.1 remote as 65001 sent keepalive 42\n",
		},
		{
			name:      "received",
			direction: "received",
			want:      "peer 10.0.0.1 remote as 65001 received keepalive 42\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatKeepalive(peer, tt.direction, 42)
			if got != tt.want {
				t.Errorf("FormatKeepalive() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormatNotificationWithDirection tests that NOTIFICATION messages include direction.
//
// VALIDATES: FormatNotification uses direction parameter ("sent"/"received").
// PREVENTS: API output missing direction indicator.
func TestFormatNotificationWithDirection(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	notify := DecodedNotification{
		ErrorCode:        6,
		ErrorSubcode:     2,
		Data:             nil,
		ErrorCodeName:    "Cease",
		ErrorSubcodeName: "Administrative Shutdown",
	}

	tests := []struct {
		name      string
		direction string
		want      string
	}{
		{
			name:      "sent",
			direction: "sent",
			want:      "peer 10.0.0.1 remote as 65001 sent notification 42 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data \n",
		},
		{
			name:      "received",
			direction: "received",
			want:      "peer 10.0.0.1 remote as 65001 received notification 42 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data \n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatNotification(peer, notify, tt.direction, 42)
			if got != tt.want {
				t.Errorf("FormatNotification() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormatNLRIJSONWithPathID verifies JSON includes path-id when present.
//
// VALIDATES: RFC 7911 path-id is included in structured JSON.
// PREVENTS: Path-id being lost in JSON output.
func TestFormatNLRIJSONWithPathID(t *testing.T) {
	n := NewTestNLRI(netip.MustParsePrefix("10.0.0.0/24"), 42)

	var sb strings.Builder
	formatNLRIJSON(&sb, n)

	want := `{"prefix":"10.0.0.0/24","path-id":42}`
	if got := sb.String(); got != want {
		t.Errorf("formatNLRIJSON() = %q, want %q", got, want)
	}
}

// TestFormatNLRIJSONNoPathID verifies JSON omits path-id when zero.
//
// VALIDATES: path-id field omitted when 0 (no ADD-PATH).
// PREVENTS: Unnecessary path-id:0 in output.
func TestFormatNLRIJSONNoPathID(t *testing.T) {
	n := NewTestNLRI(netip.MustParsePrefix("10.0.0.0/24"), 0)

	var sb strings.Builder
	formatNLRIJSON(&sb, n)

	want := `{"prefix":"10.0.0.0/24"}`
	if got := sb.String(); got != want {
		t.Errorf("formatNLRIJSON() = %q, want %q", got, want)
	}
}

// TestFormatNLRIJSONPathIDMax verifies max uint32 path-id works.
//
// VALIDATES: Max path-id value (0xFFFFFFFF) handled correctly.
// PREVENTS: Integer overflow issues.
func TestFormatNLRIJSONPathIDMax(t *testing.T) {
	n := NewTestNLRI(netip.MustParsePrefix("192.168.1.0/24"), 4294967295)

	var sb strings.Builder
	formatNLRIJSON(&sb, n)

	want := `{"prefix":"192.168.1.0/24","path-id":4294967295}`
	if got := sb.String(); got != want {
		t.Errorf("formatNLRIJSON() = %q, want %q", got, want)
	}
}

// TestWriteJSONEscapedString verifies JSON string escaping.
//
// VALIDATES: Special characters escaped per JSON spec.
// PREVENTS: Invalid JSON output from complex NLRI String().
func TestWriteJSONEscapedString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"simple", "hello", "hello"},
		{"backslash", `a\b`, `a\\b`},
		{"quote", `a"b`, `a\"b`},
		{"newline", "a\nb", `a\nb`},
		{"carriage_return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"control_char", "a\x00b", `a\u0000b`},
		{"mixed", "line1\nline2\ttab", `line1\nline2\ttab`},
		{"ip_prefix", "10.0.0.0/24", "10.0.0.0/24"},
		{"ipv6_prefix", "2001:db8::/32", "2001:db8::/32"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			writeJSONEscapedString(&sb, tt.input)
			if got := sb.String(); got != tt.want {
				t.Errorf("writeJSONEscapedString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// buildTestUpdateBodyWithCommunities builds UPDATE with COMMUNITY attribute.
func buildTestUpdateBodyWithCommunities(prefix netip.Prefix, nextHop netip.Addr, communities []uint32) []byte {
	// ORIGIN (igp) + AS_PATH (empty)
	var attrs []byte
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00)

	// NEXT_HOP (IPv4)
	if nextHop.Is4() {
		b := nextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// COMMUNITY (type 8)
	if len(communities) > 0 {
		commData := make([]byte, len(communities)*4)
		for i, c := range communities {
			binary.BigEndian.PutUint32(commData[i*4:], c)
		}
		attrs = append(attrs, 0xc0, 0x08, byte(len(commData))) // optional, transitive
		attrs = append(attrs, commData...)
	}

	// NLRI (IPv4)
	var nlriBytes []byte
	if prefix.Addr().Is4() {
		bits := prefix.Bits()
		nlriBytes = append(nlriBytes, byte(bits))
		prefixBytes := (bits + 7) / 8
		addr := prefix.Addr().As4()
		nlriBytes = append(nlriBytes, addr[:prefixBytes]...)
	}

	// Build body
	body := make([]byte, 4+len(attrs)+len(nlriBytes))
	binary.BigEndian.PutUint16(body[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs))) //nolint:gosec // test data
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlriBytes)

	return body
}

// TestFormatHexMatchesRaw verifies FormatHex produces identical output to FormatRaw.
//
// VALIDATES: AC-1 (FormatHex treated same as FormatRaw), AC-10 (raw hex output, not parsed).
//
// PREVENTS: FormatHex falling through to parsed format (the most expensive path).
func TestFormatHexMatchesRaw(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 100, []uint32{65001, 65002},
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	// FormatRaw output
	rawContent := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatRaw,
	}
	rawOut := FormatMessage(peer, msg, rawContent, "")

	// FormatHex output — should be identical to FormatRaw
	hexContent := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatHex,
	}
	hexOut := FormatMessage(peer, msg, hexContent, "")

	if hexOut != rawOut {
		t.Errorf("FormatHex and FormatRaw produce different output:\nhex: %s\nraw: %s", hexOut, rawOut)
	}

	// Both should contain raw hex (not parsed attributes)
	if strings.Contains(hexOut, `"origin"`) {
		t.Error("FormatHex should produce raw hex, not parsed attributes")
	}
	if !strings.Contains(hexOut, `"raw"`) {
		t.Error("FormatHex should contain raw hex data")
	}
}

// TestFormatFilterResultTextEmptyUpdate verifies text formatter handles UPDATEs with no NLRI.
//
// VALIDATES: Empty UPDATEs (End-of-RIB, attribute-only) produce a minimal text line.
//
// PREVENTS: Empty string delivered to plugins causing parse errors.
func TestFormatFilterResultTextEmptyUpdate(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}
	// Empty FilterResult: no announced, no withdrawn NLRIs
	result := bgpfilter.FilterResult{}
	text := formatFilterResultText(peer, result, 7, "received", nil)
	if text == "" {
		t.Fatal("empty UPDATE should produce a non-empty text line")
	}
	if !strings.HasPrefix(text, "peer 10.0.0.1 remote as 65001") {
		t.Errorf("expected uniform header 'peer 10.0.0.1 remote as 65001...', got: %q", text)
	}
	if !strings.Contains(text, "update") {
		t.Errorf("expected 'update' in output, got: %q", text)
	}
}

// TestFormatMessageTextEncoding verifies text encoding produces text output, not JSON.
//
// VALIDATES: AC-2 (text encoding produces text, not JSON).
//
// PREVENTS: Text encoding path accidentally producing JSON output.
func TestFormatMessageTextEncoding(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 100, []uint32{65001, 65002},
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("Attrs() error = %v", err)
	}

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	textContent := bgptypes.ContentConfig{
		Encoding: plugin.EncodingText,
		Format:   plugin.FormatParsed,
	}
	textOut := FormatMessage(peer, msg, textContent, "")

	// Text output should NOT be JSON
	if strings.HasPrefix(textOut, "{") {
		t.Error("text encoding should not produce JSON")
	}
	// Text output should start with "peer"
	if !strings.HasPrefix(textOut, "peer ") {
		t.Errorf("text encoding should start with 'peer', got: %s", textOut)
	}

	// JSON encoding should produce JSON
	jsonContent := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}
	jsonOut := FormatMessage(peer, msg, jsonContent, "")
	if !strings.HasPrefix(jsonOut, "{") {
		t.Error("json encoding should produce JSON")
	}
}

// TestFormatTextUpdate_ShortAliases verifies text output uses short-form keywords.
//
// VALIDATES: AC-13: event text formatter uses short aliases (next, path, pref, s-com, l-com, x-com).
//
// PREVENTS: Long-form keywords in API text output making messages unnecessarily verbose.
func TestFormatTextUpdate_ShortAliases(t *testing.T) {
	// Build a single attribute and verify text uses short form.
	tests := []struct {
		name string
		code attribute.AttributeCode
		attr attribute.Attribute
		want string // expected text output fragment
	}{
		{
			name: "next-hop uses next",
			code: attribute.AttrNextHop,
			attr: func() attribute.Attribute {
				nh := attribute.NextHop{Addr: netip.MustParseAddr("10.0.0.1")}
				return &nh
			}(),
			want: "next 10.0.0.1",
		},
		{
			name: "as-path uses path",
			code: attribute.AttrASPath,
			attr: func() attribute.Attribute {
				ap := attribute.ASPath{Segments: []attribute.ASPathSegment{
					{Type: 2, ASNs: []uint32{65001, 65002}},
				}}
				return &ap
			}(),
			want: "path 65001,65002",
		},
		{
			name: "local-preference uses pref",
			code: attribute.AttrLocalPref,
			attr: func() attribute.Attribute {
				lp := attribute.LocalPref(100)
				return &lp
			}(),
			want: "pref 100",
		},
		{
			name: "community uses s-com",
			code: attribute.AttrCommunity,
			attr: func() attribute.Attribute {
				c := attribute.Communities{attribute.Community(65000<<16 | 100)}
				return &c
			}(),
			want: "s-com 65000:100",
		},
		{
			name: "origin unchanged",
			code: attribute.AttrOrigin,
			attr: func() attribute.Attribute {
				o := attribute.Origin(0)
				return &o
			}(),
			want: "origin igp",
		},
		{
			name: "med unchanged",
			code: attribute.AttrMED,
			attr: func() attribute.Attribute {
				m := attribute.MED(42)
				return &m
			}(),
			want: "med 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			var scratch [64]byte
			formatAttributeText(&sb, tt.code, tt.attr, scratch[:])
			got := sb.String()
			if got != tt.want {
				t.Errorf("formatAttributeText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// NewTestNLRI creates a test NLRI with prefix and optional path-id.
func NewTestNLRI(prefix netip.Prefix, pathID uint32) nlri.NLRI {
	family := nlri.IPv4Unicast
	if prefix.Addr().Is6() {
		family = nlri.IPv6Unicast
	}
	return nlri.NewINET(family, prefix, pathID)
}
