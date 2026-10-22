// Design: docs/architecture/exabgp-bridge.md -- a message as its packet: packets, consolidate, parsed
// Related: bridge_event_packets.go -- the forms under test

package bridge

import (
	"log/slog"
	"strings"
	"testing"
)

// packetEvent reads a `full` ze event of one message kind, the way the bridge
// meets it on its subscription.
func packetEvent(t *testing.T, kind, member, body string) Event {
	t.Helper()
	zebgp := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": kind, "direction": "received"},
			"peer": map[string]any{
				"remote": map[string]any{"address": "10.0.0.2", "as": float64(64512)},
				"local":  map[string]any{"address": "10.0.0.17", "as": float64(64512)},
			},
			kind:  map[string]any{},
			"raw": map[string]any{member: body},
		},
	}
	return ReadEvent(zebgp)
}

// TestFormsFor proves the forms of each direction follow the pin's
// Protocol._tell_api_received and Protocol._to_api.
//
// VALIDATES: consolidate wins on receive; a sent message is consolidated only
// with packets; packets and parsed together write both; no word writes the
// decoded line.
// PREVENTS: a script that asked for the packet reading the decoded line, or
// the reverse.
func TestFormsFor(t *testing.T) {
	cases := []struct {
		name      string
		direction string
		words     []string
		want      Forms
	}{
		{"none", modeReceive, nil, FormParsed},
		{"parsed", modeReceive, []string{"parsed"}, FormParsed},
		{"packets", modeReceive, []string{"packets"}, FormPackets},
		{"packets parsed", modeReceive, []string{"packets", "parsed"}, FormPackets | FormParsed},
		{"consolidate", modeReceive, []string{"consolidate"}, FormConsolidated},
		{"consolidate packets", modeReceive, []string{"consolidate", "packets"}, FormConsolidated},
		{"send consolidate", modeSend, []string{"consolidate"}, FormParsed},
		{"send consolidate packets", modeSend, []string{"consolidate", "packets"}, FormConsolidated},
		{"send packets", modeSend, []string{"packets"}, FormPackets},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			granted := map[string]bool{}
			for _, word := range tc.words {
				granted[tc.direction+"-"+word] = true
			}
			got := FormsFor(tc.direction, func(key string) bool { return granted[key] })
			if got != tc.want {
				t.Errorf("FormsFor(%s, %v) = %b, want %b", tc.direction, tc.words, got, tc.want)
			}
		})
	}
}

// TestPacketLines proves the packet and consolidated lines of a KEEPALIVE and
// a ROUTE-REFRESH are the lines 5.0 wrote (qa/old/responses-5.0.json.gz at the
// pin, the `keepalive packets`, `keepalive consolidate` and `refresh packets`
// events).
//
// VALIDATES: the header is derived from the body and the type; an empty body
// is left out of a text line and of a consolidated JSON line, and written `0x`
// in a packet JSON line; a ROUTE-REFRESH packet is typed `route-refresh`.
// PREVENTS: a helper parsing `header` and `body` off a line that carries
// neither.
func TestPacketLines(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	keepalive := packetEvent(t, msgTypeKeepalive, "message", "")
	if got, want := string(keepalive.Lines(FormatV4Text, FormPackets, log)),
		"neighbor 10.0.0.2 receive 4 header 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304\n"; got != want {
		t.Errorf("keepalive packets text:\n got %q\nwant %q", got, want)
	}
	if got, want := string(keepalive.Lines(FormatV4Text, FormConsolidated, log)),
		"neighbor 10.0.0.2 receive keepalive header 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304\n"; got != want {
		t.Errorf("keepalive consolidate text:\n got %q\nwant %q", got, want)
	}
	packetJSON := string(keepalive.Lines(FormatV4JSON, FormPackets, log))
	for _, want := range []string{`"type": "keepalive"`, `"category": 4`, `"body": "0x"`,
		`"header": "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"`, `"address": { "local": "10.0.0.17", "peer": "10.0.0.2" }`} {
		if !strings.Contains(packetJSON, want) {
			t.Errorf("keepalive packets JSON lacks %s:\n%s", want, packetJSON)
		}
	}
	consolidated := string(keepalive.Lines(FormatV4JSON, FormConsolidated, log))
	if strings.Contains(consolidated, `"body"`) {
		t.Errorf("a consolidated KEEPALIVE writes no body:\n%s", consolidated)
	}

	refresh := packetEvent(t, "refresh", "message", "00010001")
	refreshJSON := string(refresh.Lines(FormatV4JSON, FormPackets, log))
	for _, want := range []string{`"type": "route-refresh"`, `"category": 5`, `"body": "0x00010001"`,
		`"header": "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001705"`} {
		if !strings.Contains(refreshJSON, want) {
			t.Errorf("refresh packets JSON lacks %s:\n%s", want, refreshJSON)
		}
	}
	both := string(refresh.Lines(FormatV4Text, FormPackets|FormParsed, log))
	if want := "neighbor 10.0.0.2 receive 5 header 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001705 body 0x00010001\n" +
		"neighbor 10.0.0.2 receive route-refresh"; !strings.HasPrefix(both, want) {
		t.Errorf("packets and parsed write the packet first:\n got %q\nwant prefix %q", both, want)
	}
}

// TestConsolidatedUpdateText proves a consolidated UPDATE writes its header and
// body on a line of their own before `update end`, as Text.update does.
//
// VALIDATES: the line sits between the route lines and the end line.
// PREVENTS: the header and body glued to the end line, where a helper that
// reads `update end` no longer finds it.
func TestConsolidatedUpdateText(t *testing.T) {
	event := packetEvent(t, msgTypeUpdate, msgTypeUpdate, "00000000")
	got := string(event.Lines(FormatV4Text, FormConsolidated, slog.New(slog.DiscardHandler)))
	want := " header 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001702 body 0x00000000\nneighbor 10.0.0.2 receive update end\n"
	if !strings.HasSuffix(got, want) {
		t.Errorf("consolidated UPDATE text:\n got %q\nwant suffix %q", got, want)
	}
}

// TestReadPacketRefusesForeignBytes proves an event whose raw member is not a
// message body carries no packet.
//
// VALIDATES: odd length and a non-hex digit are refused; the zero Packet is
// what an event with no raw member carries.
// PREVENTS: a script written a header whose length the body does not have.
func TestReadPacketRefusesForeignBytes(t *testing.T) {
	for _, body := range []string{"0", "zz"} {
		if event := packetEvent(t, msgTypeKeepalive, "message", body); event.Packet.Present() {
			t.Errorf("raw %q read as a packet", body)
		}
	}
	if !packetEvent(t, msgTypeKeepalive, "message", "").Packet.Present() {
		t.Error("an empty KEEPALIVE body is a packet")
	}
	state := ReadEvent(map[string]any{"type": "bgp", "bgp": map[string]any{
		"message": map[string]any{"type": "state"}, "state": "up",
	}})
	if state.Packet.Present() {
		t.Error("a state event carries no packet")
	}
}
