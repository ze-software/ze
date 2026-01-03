package api

import (
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// TestFormatStateChange tests state event formatting.
//
// VALIDATES: Peer state changes format correctly for both encodings.
//
// PREVENTS: State events not being delivered to processes.
func TestFormatStateChange(t *testing.T) {
	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	tests := []struct {
		name     string
		state    string
		encoding string
		want     string
	}{
		{
			name:     "text established",
			state:    "established",
			encoding: EncodingText,
			want:     "peer 10.0.0.1 asn 65001 state established\n",
		},
		{
			name:     "text down",
			state:    "down",
			encoding: EncodingText,
			want:     "peer 10.0.0.1 asn 65001 state down\n",
		},
		{
			name:     "json established",
			state:    "established",
			encoding: EncodingJSON,
			want:     `{"type":"state","peer":{"address":"10.0.0.1","asn":65001},"state":"established"}` + "\n",
		},
		{
			name:     "json down",
			state:    "down",
			encoding: EncodingJSON,
			want:     `{"type":"state","peer":{"address":"10.0.0.1","asn":65001},"state":"down"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStateChange(peer, tt.state, tt.encoding)
			if got != tt.want {
				t.Errorf("FormatStateChange() = %q, want %q", got, tt.want)
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

	peer := PeerInfo{
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

	attrBytes := ExtractAttributeBytes(body)
	msg := RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  body,
		AttrsWire: attribute.NewAttributesWire(attrBytes, ctxID),
	}

	content := ContentConfig{
		Encoding: EncodingText,
		Format:   FormatParsed,
	}

	got := FormatMessage(peer, msg, content)

	// Format: peer <ip> asn <asn> update <id> announce <attrs> <family> next-hop <ip> nlri <prefixes>
	if !strings.Contains(got, "peer 10.0.0.1 asn 65001 update") {
		t.Errorf("FormatMessage() =\n%q\nshould contain 'peer 10.0.0.1 asn 65001 update'", got)
	}
	if !strings.Contains(got, "announce") {
		t.Error("missing announce")
	}
	if !strings.Contains(got, "origin igp") {
		t.Error("missing origin")
	}
	if !strings.Contains(got, "as-path 65001 65002") {
		t.Error("missing as-path")
	}
	if !strings.Contains(got, "local-preference 100") {
		t.Error("missing local-preference")
	}
	if !strings.Contains(got, "ipv4 unicast next-hop 10.0.0.1 nlri 192.168.1.0/24") {
		t.Error("missing family/next-hop/nlri")
	}
}

// TestFormatMessageJSON tests JSON format output.
//
// VALIDATES: JSON uses announce.nlri structure.
//
// PREVENTS: Wrong JSON structure sent to API processes.
func TestFormatMessageJSON(t *testing.T) {
	ctxID := testEncodingContext()

	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE body with NLRI
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)

	attrBytes := ExtractAttributeBytes(body)
	msg := RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  body,
		AttrsWire: attribute.NewAttributesWire(attrBytes, ctxID),
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	got := FormatMessage(peer, msg, content)

	// Check key parts of the JSON structure
	if !strings.Contains(got, `"type":"update"`) {
		t.Error("missing type:update")
	}
	if !strings.Contains(got, `"peer":{"address":"10.0.0.1","asn":65001}`) {
		t.Error("missing peer info")
	}
	if !strings.Contains(got, `"announce":{`) {
		t.Error("missing announce structure")
	}
	if !strings.Contains(got, `"ipv4 unicast":`) {
		t.Error("missing ipv4 unicast family")
	}
	if !strings.Contains(got, `192.168.1.0/24`) {
		t.Error("missing prefix")
	}
}

// buildTestUpdateBodyWithAttrs builds a BGP UPDATE message body with custom attributes.
// Format: withdrawn_len(2) + withdrawn + attr_len(2) + attrs + nlri.
func buildTestUpdateBodyWithAttrs(prefix netip.Prefix, nextHop netip.Addr, origin uint8, localPref uint32, asPath []uint32) []byte {
	var attrs []byte

	// ORIGIN
	if origin <= 2 {
		attrs = append(attrs, 0x40, 0x01, 0x01, origin)
	}

	// AS_PATH
	if len(asPath) > 0 {
		asPathData := []byte{0x02, byte(len(asPath))} // AS_SEQUENCE
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
	peer := PeerInfo{
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

	msg := RawMessage{
		Type:     message.TypeOPEN,
		RawBytes: openBody,
	}

	content := ContentConfig{
		Encoding: EncodingText,
		Format:   FormatParsed,
	}

	got := FormatMessage(peer, msg, content)

	// Should use FormatOpen with new format: peer X asn Y open ...
	if !strings.Contains(got, "peer 10.0.0.1 asn 65001 open") {
		t.Errorf("FormatMessage() for OPEN =\n%q\nshould contain 'peer 10.0.0.1 asn 65001 open'", got)
	}
	if !strings.Contains(got, "version 4") {
		t.Errorf("FormatMessage() for OPEN =\n%q\nshould contain 'version 4'", got)
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
	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	msg := RawMessage{
		Type:     message.TypeKEEPALIVE,
		RawBytes: []byte{}, // KEEPALIVE has no body
	}

	content := ContentConfig{
		Encoding: EncodingText,
		Format:   FormatParsed,
	}

	got := FormatMessage(peer, msg, content)

	// Should use new format: peer X asn Y keepalive
	if !strings.Contains(got, "peer 10.0.0.1 asn 65001 keepalive") {
		t.Errorf("FormatMessage() for KEEPALIVE =\n%q\nshould contain 'peer 10.0.0.1 asn 65001 keepalive'", got)
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

	// Create AttrsWire and apply filter
	attrBytes := ExtractAttributeBytes(body)
	if attrBytes == nil {
		t.Fatal("Failed to extract attribute bytes")
	}
	wire := attribute.NewAttributesWire(attrBytes, ctxID)

	filter := NewFilterAll()
	nlriFilter := NewNLRIFilterAll()
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

	// Create AttrsWire and apply filter
	attrBytes := ExtractAttributeBytes(body)
	if attrBytes == nil {
		t.Fatal("Failed to extract attribute bytes")
	}
	wire := attribute.NewAttributesWire(attrBytes, ctxID)

	filter := NewFilterAll()
	nlriFilter := NewNLRIFilterAll()
	result, err := filter.ApplyToUpdate(wire, body, nlriFilter)
	if err != nil {
		t.Fatalf("ApplyToUpdate failed: %v", err)
	}

	// Check next-hops via FamilyNLRI iteration
	announced := result.AnnouncedByFamily()
	if len(announced) != 2 {
		t.Fatalf("AnnouncedByFamily len = %d, want 2 (IPv4 + IPv6)", len(announced))
	}

	// Find IPv4 and IPv6 families
	var gotIPv4, gotIPv6 bool
	for _, fam := range announced {
		if fam.Family == "ipv4-unicast" {
			gotIPv4 = true
			if fam.NextHop.String() != "10.0.0.1" {
				t.Errorf("IPv4 NextHop = %v, want 10.0.0.1", fam.NextHop)
			}
		}
		if fam.Family == "ipv6-unicast" {
			gotIPv6 = true
			if fam.NextHop.String() != "2001:db8::1" {
				t.Errorf("IPv6 NextHop = %v, want 2001:db8::1", fam.NextHop)
			}
		}
	}
	if !gotIPv4 {
		t.Error("missing ipv4-unicast family")
	}
	if !gotIPv6 {
		t.Error("missing ipv6-unicast family")
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

	// Create AttrsWire and apply filter
	attrBytes := ExtractAttributeBytes(body)
	if attrBytes == nil {
		t.Fatal("Failed to extract attribute bytes")
	}
	wire := attribute.NewAttributesWire(attrBytes, ctxID)

	filter := NewFilterAll()
	nlriFilter := NewNLRIFilterAll()
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
	var attrs []byte

	// ORIGIN
	attrs = append(attrs, 0x40, 0x01, 0x01, origin)

	// AS_PATH (empty)
	attrs = append(attrs, 0x40, 0x02, 0x00)

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
func buildTestUpdateBodyWithBothFamilies(ipv4Prefix netip.Prefix, ipv4NextHop netip.Addr, ipv6Prefix netip.Prefix, ipv6NextHop netip.Addr) []byte {
	var attrs []byte

	// ORIGIN
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00) // igp

	// AS_PATH (empty)
	attrs = append(attrs, 0x40, 0x02, 0x00)

	// NEXT_HOP for IPv4
	if ipv4NextHop.Is4() {
		b := ipv4NextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// MP_REACH_NLRI for IPv6
	// AFI=2 (IPv6), SAFI=1 (unicast), NH len=16, next-hop, reserved=0, NLRI
	mpReach := []byte{
		0x00, 0x02, // AFI IPv6
		0x01, // SAFI unicast
		0x10, // NH len = 16
	}
	nhBytes := ipv6NextHop.As16()
	mpReach = append(mpReach, nhBytes[:]...)
	mpReach = append(mpReach, 0x00) // reserved

	// IPv6 NLRI
	bits := ipv6Prefix.Bits()
	mpReach = append(mpReach, byte(bits))
	prefixBytes := (bits + 7) / 8
	addr := ipv6Prefix.Addr().As16()
	mpReach = append(mpReach, addr[:prefixBytes]...)

	// MP_REACH_NLRI attribute (optional, transitive)
	attrs = append(attrs, 0x90, 0x0e) // flags=0x90, type=14
	attrs = append(attrs, byte(len(mpReach)>>8), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)

	// IPv4 NLRI
	var nlri []byte
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

// buildTestUpdateBodyWithCommunities builds UPDATE with COMMUNITY attribute.
func buildTestUpdateBodyWithCommunities(prefix netip.Prefix, nextHop netip.Addr, communities []uint32) []byte {
	var attrs []byte

	// ORIGIN
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00) // igp

	// AS_PATH (empty)
	attrs = append(attrs, 0x40, 0x02, 0x00)

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
