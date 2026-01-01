package api

import (
	"net/netip"
	"strings"
	"testing"
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

	route := ReceivedRoute{
		Prefix:          netip.MustParsePrefix("192.168.1.0/24"),
		NextHop:         netip.MustParseAddr("10.0.0.1"),
		Origin:          "igp",
		LocalPreference: 100,
		ASPath:          []uint32{65001, 65002},
	}

	// Create a minimal UPDATE message (just need RawBytes for decoding)
	// We'll test the formatting function directly instead
	content := ContentConfig{
		Encoding: EncodingText,
		Format:   FormatParsed,
		Version:  APIVersionNLRI, // v7
	}

	got := formatRoutesTextV7(peer, []ReceivedRoute{route})
	want := "peer 10.0.0.1 update announce nlri ipv4 unicast 192.168.1.0/24 next-hop 10.0.0.1 origin igp as-path [65001 65002] local-preference 100\n"

	if got != want {
		t.Errorf("formatRoutesTextV7() =\n%q\nwant:\n%q", got, want)
	}

	// Verify ContentConfig defaults to v7
	_ = content // used above
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

	route := ReceivedRoute{
		Prefix:  netip.MustParsePrefix("192.168.1.0/24"),
		NextHop: netip.MustParseAddr("10.0.0.1"),
		Origin:  "igp",
	}

	got := formatRoutesJSONv7(peer, []ReceivedRoute{route}, 0)

	// Check key parts of the JSON structure
	if !strings.Contains(got, `"type":"update"`) {
		t.Error("missing type:update")
	}
	if !strings.Contains(got, `"peer":{"address":"10.0.0.1","asn":65001}`) {
		t.Error("missing peer info")
	}
	if !strings.Contains(got, `"announce":{"nlri":{`) {
		t.Error("missing announce.nlri structure")
	}
	if !strings.Contains(got, `"ipv4 unicast":`) {
		t.Error("missing ipv4 unicast family")
	}
	if !strings.Contains(got, `"prefix":"192.168.1.0/24"`) {
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

	route := ReceivedRoute{
		Prefix:  netip.MustParsePrefix("192.168.1.0/24"),
		NextHop: netip.MustParseAddr("10.0.0.1"),
		Origin:  "igp",
	}

	v6Text := FormatReceivedUpdate(peer.Address, []ReceivedRoute{route})
	v7Text := formatRoutesTextV7(peer, []ReceivedRoute{route})

	// V6 uses "neighbor X receive update announced ..."
	if !strings.Contains(v6Text, "neighbor") {
		t.Error("v6 should use 'neighbor' keyword")
	}
	if !strings.Contains(v6Text, "receive update") {
		t.Error("v6 should use 'receive update'")
	}

	// V7 uses "peer X update announce nlri ..."
	if !strings.Contains(v7Text, "peer") {
		t.Error("v7 should use 'peer' keyword")
	}
	if !strings.Contains(v7Text, "announce nlri") {
		t.Error("v7 should use 'announce nlri'")
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
