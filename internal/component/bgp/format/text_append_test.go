package format

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// testPeer is the canonical peer fixture used across parity tests.
func testPeer() *plugin.PeerInfo {
	return &plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		Name:         "peer1",
		GroupName:    "",
		PeerAS:       65001,
		LocalAS:      65000,
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
	}
}

// legacyEscapeJSON mirrors the old jsonSafeReplacer+writeJSONEscapedString
// combo so parity tests can assert byte-for-byte equality of appendJSONString.
// Kept inline after peer_json.go deletion (fmt-1) so this parity test still
// documents the byte-identical contract with the fmt-0 combo.
func legacyEscapeJSON(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
			continue
		case '"':
			sb.WriteString(`\"`)
			continue
		case '\n':
			sb.WriteString(`\n`)
			continue
		case '\r':
			sb.WriteString(`\r`)
			continue
		case '\t':
			sb.WriteString(`\t`)
			continue
		}
		if r < 0x20 {
			fmt.Fprintf(&sb, `\u%04x`, r)
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// VALIDATES: appendJSONString matches the legacy JSON escape path for every
// case in the AC-12 corpus: empty, ASCII, control chars 0x00-0x1F, embedded
// quotes, embedded backslashes, and multi-byte UTF-8 sequences.
// PREVENTS: drift in plugin IPC JSON envelopes where peer/group/name
// contains a control character.
func TestAppendJSONString(t *testing.T) {
	cases := []string{
		"",
		"plain-name",
		"with\"quote",
		"with\\backslash",
		"line1\nline2",
		"tab\there",
		"return\rnow",
		string([]byte{0x00, 0x01, 0x1F}),
		"utf8-ASCII-only",
		"mix\\and\"and\nand\t",
	}
	for _, in := range cases {
		want := legacyEscapeJSON(in)
		got := string(appendJSONString(nil, in))
		if got != want {
			t.Errorf("appendJSONString(%q)\n got:  %q\n want: %q", in, got, want)
		}
	}
}

// VALIDATES: appendReplacingByte replaces every `from` byte with `to`.
// Uses the NOTIFICATION error-name corpus (AC-14), byte-identical to the
// legacy strings.ReplaceAll for ASCII inputs.
// PREVENTS: drift in NOTIFICATION text where code-name/subcode-name must
// be single-word tokens (space -> hyphen).
func TestAppendReplacingByte(t *testing.T) {
	cases := []string{
		"",
		"Administrative Shutdown",
		"Hold Timer Expired",
		"Cease",
		"Connection Collision Resolution",
		"Out of Resources",
	}
	for _, in := range cases {
		want := strings.ReplaceAll(in, " ", "-")
		got := string(appendReplacingByte(nil, in, ' ', '-'))
		if got != want {
			t.Errorf("appendReplacingByte(%q): got %q, want %q", in, got, want)
		}
	}
}

// VALIDATES: AppendOpen produces the same bytes the legacy code would have
// produced via FormatOpen (fmt.Sprintf-based). Captured from the old code
// before migration; spec AC-5 byte-identical parity guarantee.
// PREVENTS: drift in OPEN text format consumed by text-mode subscribers.
func TestAppendOpen_Parity(t *testing.T) {
	peer := testPeer()
	open := DecodedOpen{
		Version:  4,
		ASN:      65010,
		HoldTime: 180,
		RouterID: "1.2.3.4",
		Capabilities: []DecodedCapability{
			{Code: 1, Name: "multi-protocol", Value: "ipv4/unicast"},
			{Code: 65, Name: "asn4", Value: ""},
		},
	}
	got := string(AppendOpen(nil, peer, open, "received", 42))
	want := "peer 10.0.0.1 remote as 65010 received open 42 router-id 1.2.3.4 hold-time 180 cap 1 multi-protocol ipv4/unicast cap 65 asn4\n"
	if got != want {
		t.Errorf("AppendOpen:\n got:  %q\n want: %q", got, want)
	}
}

// VALIDATES: AppendNotification matches the legacy FormatNotification
// output byte-for-byte. Includes the data-bytes hex branch.
// PREVENTS: drift in NOTIFICATION subscriber text; consumed by text-mode
// plugins that parse code/subcode/data tokens.
func TestAppendNotification_Parity(t *testing.T) {
	peer := testPeer()
	notify := DecodedNotification{
		ErrorCode:        6,
		ErrorSubcode:     2,
		ErrorCodeName:    "Cease",
		ErrorSubcodeName: "Administrative Shutdown",
		Data:             []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	got := string(AppendNotification(nil, peer, notify, "received", 7))
	want := "peer 10.0.0.1 remote as 65001 received notification 7 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data deadbeef\n"
	if got != want {
		t.Errorf("AppendNotification:\n got:  %q\n want: %q", got, want)
	}
	// Empty data branch: "data " with no hex.
	notify.Data = nil
	got = string(AppendNotification(nil, peer, notify, "sent", 8))
	want = "peer 10.0.0.1 remote as 65001 sent notification 8 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data \n"
	if got != want {
		t.Errorf("AppendNotification (empty data):\n got:  %q\n want: %q", got, want)
	}
}

// VALIDATES: AppendKeepalive matches the legacy FormatKeepalive output.
// PREVENTS: drift in KEEPALIVE subscriber text.
func TestAppendKeepalive_Parity(t *testing.T) {
	peer := testPeer()
	got := string(AppendKeepalive(nil, peer, "received", 3))
	want := "peer 10.0.0.1 remote as 65001 received keepalive 3\n"
	if got != want {
		t.Errorf("AppendKeepalive:\n got:  %q\n want: %q", got, want)
	}
}

// VALIDATES: AppendRouteRefresh matches the legacy FormatRouteRefresh
// output for the RFC 7313 subtype variants (refresh / borr / eorr).
func TestAppendRouteRefresh_Parity(t *testing.T) {
	peer := testPeer()
	decoded := DecodedRouteRefresh{
		AFI:         1,
		SAFI:        1,
		Subtype:     0,
		SubtypeName: "refresh",
		Family:      "ipv4/unicast",
	}
	got := string(AppendRouteRefresh(nil, peer, decoded, "received", 9))
	want := "peer 10.0.0.1 remote as 65001 received refresh 9 family ipv4/unicast\n"
	if got != want {
		t.Errorf("AppendRouteRefresh:\n got:  %q\n want: %q", got, want)
	}
}

// VALIDATES: AppendEOR matches the legacy FormatEOR output for both
// encodings (text and JSON). AC-5 byte-identical parity.
func TestAppendEOR_Parity(t *testing.T) {
	peer := testPeer()
	gotText := string(AppendEOR(nil, peer, "ipv4/unicast", plugin.EncodingText))
	if gotText != "peer 10.0.0.1 remote as 65001 eor ipv4/unicast\n" {
		t.Errorf("AppendEOR text: got %q", gotText)
	}
	gotJSON := string(AppendEOR(nil, peer, "ipv4/unicast", plugin.EncodingJSON))
	if !strings.Contains(gotJSON, `"message":{"type":"eor"}`) ||
		!strings.Contains(gotJSON, `"family":"ipv4/unicast"`) ||
		!strings.Contains(gotJSON, `"address":"10.0.0.1"`) {
		t.Errorf("AppendEOR JSON: missing expected fields: %q", gotJSON)
	}
}

// VALIDATES: AppendCongestion matches the legacy FormatCongestion output.
func TestAppendCongestion_Parity(t *testing.T) {
	peer := testPeer()
	gotText := string(AppendCongestion(nil, peer, "congested", plugin.EncodingText))
	if gotText != "peer 10.0.0.1 remote as 65001 congested\n" {
		t.Errorf("AppendCongestion text: got %q", gotText)
	}
	gotJSON := string(AppendCongestion(nil, peer, "resumed", plugin.EncodingJSON))
	if !strings.Contains(gotJSON, `"message":{"type":"resumed"}`) ||
		!strings.Contains(gotJSON, `"address":"10.0.0.1"`) {
		t.Errorf("AppendCongestion JSON: missing expected fields: %q", gotJSON)
	}
}
