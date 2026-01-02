package api

import (
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// TestFormatReceivedUpdate tests ExaBGP text format for received UPDATE messages.
//
// VALIDATES: Received UPDATE messages are formatted correctly for text encoder.
// Format: "neighbor <ip> receive update announced <prefix> next-hop <nh> <attrs>"
//
// PREVENTS: Process scripts not receiving expected UPDATE format, causing check test timeout.
func TestFormatReceivedUpdate(t *testing.T) {
	tests := []struct {
		name       string
		peerAddr   string
		prefix     string
		nextHop    string
		origin     string
		localPref  uint32
		wantOutput string
	}{
		{
			name:      "basic ipv4 announce",
			peerAddr:  "127.0.0.1",
			prefix:    "0.0.0.0/32",
			nextHop:   "127.0.0.1",
			origin:    "igp",
			localPref: 100,
			wantOutput: "neighbor 127.0.0.1 receive update start\n" +
				"neighbor 127.0.0.1 receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100\n" +
				"neighbor 127.0.0.1 receive update end\n",
		},
		{
			name:      "different prefix and nexthop",
			peerAddr:  "192.168.1.1",
			prefix:    "10.0.0.0/8",
			nextHop:   "192.168.1.1",
			origin:    "egp",
			localPref: 200,
			wantOutput: "neighbor 192.168.1.1 receive update start\n" +
				"neighbor 192.168.1.1 receive update announced 10.0.0.0/8 next-hop 192.168.1.1 origin egp local-preference 200\n" +
				"neighbor 192.168.1.1 receive update end\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peerAddr := netip.MustParseAddr(tt.peerAddr)
			prefix := netip.MustParsePrefix(tt.prefix)
			nextHop := netip.MustParseAddr(tt.nextHop)

			route := ReceivedRoute{
				Prefix:          prefix,
				NextHop:         nextHop,
				Origin:          tt.origin,
				LocalPreference: tt.localPref,
			}

			got := FormatReceivedUpdate(peerAddr, []ReceivedRoute{route})
			if got != tt.wantOutput {
				t.Errorf("FormatReceivedUpdate() =\n%q\nwant:\n%q", got, tt.wantOutput)
			}
		})
	}
}

// TestFormatReceivedUpdateMultipleRoutes tests formatting multiple routes in one UPDATE.
//
// VALIDATES: Multiple routes in one UPDATE are formatted correctly.
//
// PREVENTS: Missing routes when UPDATE contains multiple NLRIs.
func TestFormatReceivedUpdateMultipleRoutes(t *testing.T) {
	peerAddr := netip.MustParseAddr("127.0.0.1")
	routes := []ReceivedRoute{
		{
			Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:         netip.MustParseAddr("192.168.1.1"),
			Origin:          "igp",
			LocalPreference: 100,
		},
		{
			Prefix:          netip.MustParsePrefix("10.0.1.0/24"),
			NextHop:         netip.MustParseAddr("192.168.1.1"),
			Origin:          "igp",
			LocalPreference: 100,
		},
	}

	got := FormatReceivedUpdate(peerAddr, routes)
	want := "neighbor 127.0.0.1 receive update start\n" +
		"neighbor 127.0.0.1 receive update announced 10.0.0.0/24 next-hop 192.168.1.1 origin igp local-preference 100\n" +
		"neighbor 127.0.0.1 receive update announced 10.0.1.0/24 next-hop 192.168.1.1 origin igp local-preference 100\n" +
		"neighbor 127.0.0.1 receive update end\n"

	if got != want {
		t.Errorf("FormatReceivedUpdate() =\n%q\nwant:\n%q", got, want)
	}
}

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
			want:     "neighbor 10.0.0.1 state established\n",
		},
		{
			name:     "text down",
			state:    "down",
			encoding: EncodingText,
			want:     "neighbor 10.0.0.1 state down\n",
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

// TestFormatMessageV7Text tests v7 text format output.
//
// VALIDATES: V7 format uses "peer X update announce nlri ..." syntax.
//
// PREVENTS: Wrong format sent to v7-expecting processes.
func TestFormatMessageV7Text(t *testing.T) {
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

	msg := RawMessage{
		Type:     message.TypeUPDATE,
		RawBytes: body,
	}

	content := ContentConfig{
		Encoding: EncodingText,
		Format:   FormatParsed,
		Version:  APIVersionNLRI, // v7
	}

	got := FormatMessage(peer, msg, content)

	// V7 format: peer <ip> asn <asn> update <id> announce <attrs> <family> next-hop <ip> nlri <prefixes>
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

// TestFormatMessageV7JSON tests v7 JSON format output.
//
// VALIDATES: V7 JSON uses announce.nlri structure.
//
// PREVENTS: Wrong JSON structure sent to v7-expecting processes.
func TestFormatMessageV7JSON(t *testing.T) {
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

	msg := RawMessage{
		Type:     message.TypeUPDATE,
		RawBytes: body,
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
		Version:  APIVersionNLRI, // v7
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

// TestFormatMessageV6VsV7 tests that v6 and v7 produce different output.
//
// VALIDATES: Version field affects output format.
//
// PREVENTS: Version being ignored.
func TestFormatMessageV6VsV7(t *testing.T) {
	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE body with NLRI
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 100, nil,
	)

	msg := RawMessage{
		Type:     message.TypeUPDATE,
		RawBytes: body,
	}

	v6Content := ContentConfig{
		Encoding: EncodingText,
		Format:   FormatParsed,
		Version:  APIVersionLegacy, // v6
	}

	v7Content := ContentConfig{
		Encoding: EncodingText,
		Format:   FormatParsed,
		Version:  APIVersionNLRI, // v7
	}

	v6Text := FormatMessage(peer, msg, v6Content)
	v7Text := FormatMessage(peer, msg, v7Content)

	// V6 uses "neighbor X receive update announced ..."
	if !strings.Contains(v6Text, "neighbor") {
		t.Error("v6 should use 'neighbor' keyword")
	}
	if !strings.Contains(v6Text, "receive update") {
		t.Error("v6 should use 'receive update'")
	}

	// V7 uses "peer <ip> asn <asn> update <id> announce <attrs> <family> next-hop <ip> nlri <prefixes>"
	if !strings.Contains(v7Text, "peer") {
		t.Error("v7 should use 'peer' keyword")
	}
	if !strings.Contains(v7Text, "asn 65001") {
		t.Error("v7 should include 'asn <number>'")
	}
	if !strings.Contains(v7Text, "nlri") {
		t.Error("v7 should use 'nlri'")
	}

	// They should be different
	if v6Text == v7Text {
		t.Error("v6 and v7 output should be different")
	}
}

// TestContentConfigVersionDefault tests that version defaults to 7.
//
// VALIDATES: Empty version field defaults to APIVersionNLRI (7).
//
// PREVENTS: Legacy format being used unintentionally.
func TestContentConfigVersionDefault(t *testing.T) {
	content := ContentConfig{}.WithDefaults()

	if content.Version != APIVersionNLRI {
		t.Errorf("Version default = %d, want %d (APIVersionNLRI)", content.Version, APIVersionNLRI)
	}
	if content.Version != 7 {
		t.Errorf("Version default = %d, want 7", content.Version)
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
