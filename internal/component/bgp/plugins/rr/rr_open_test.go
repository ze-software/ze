// Tests for the route reflector's received-OPEN reader and its peer row.
//
// VALIDATES: AC-1r and AC-1s of spec-bgp-open-session-rfc-defects for rr.
// PREVENTS: a one-octet Optional Parameters walk misframing an RFC 9072 OPEN,
// and an unreadable OPEN recorded as the ipv4/unicast default.
// Related: rr.go -- handleStructuredOpen, peerStatus

package rr

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const openTestPeer = "192.0.2.1"

// openTestBody is an OPEN body (no header) carrying params, framed by the one
// packer and written by message.Open.WriteTo, so the extended case is the RFC
// 9072 form ze itself sends.
func openTestBody(t *testing.T, params ...capability.OptionalParam) []byte {
	t.Helper()
	optional, extended, err := capability.PackOptionalParams(params)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	open := message.Open{Version: 4, MyAS: 23456, HoldTime: 90, BGPIdentifier: 0x01020304, OptionalParams: optional, ExtendedParams: extended}
	buf := make([]byte, open.Len(nil))
	open.WriteTo(buf, 0, nil)
	return buf[message.HeaderLen:]
}

// openTestParams is several type-2 parameters: Multiprotocol ipv6/unicast and
// four-octet AS 4200000001, then filler capabilities of unknown codes. With
// fillers of 200 octets the parameters exceed 255 octets and take the RFC 9072
// form.
func openTestParams(filler int) []capability.OptionalParam {
	params := []capability.OptionalParam{
		capability.RawParam(2, []byte{1, 4, 0, 2, 0, 1}),
		capability.RawParam(2, []byte{65, 4, 0xfa, 0x56, 0xea, 0x01}),
	}
	for range 2 {
		params = append(params, capability.RawParam(2, append([]byte{0xee, byte(filler)}, make([]byte, filler)...)))
	}
	return params
}

// captureRRLog routes the package logger into a buffer for one test.
func captureRRLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := logger()
	setLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { loggerPtr.Store(prev) })
	return &buf
}

// TestRouteReflectorReadsExtendedOpen proves rr reads a received OPEN through
// the one decoder: the base and the RFC 9072 framing record the same families
// and ASN4, and an OPEN the decoder or message.UnpackOpen refuses records
// nothing (never the ipv4/unicast default) and logs one WARN naming the peer and
// the decoder error.
func TestRouteReflectorReadsExtendedOpen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filler int
	}{{"base form", 10}, {"RFC 9072 form", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			rr := &routeReflector{peers: map[string]*peerState{}}
			body := openTestBody(t, openTestParams(tc.filler)...)
			if extended := body[9] == 255 && body[10] == 255; extended != (tc.filler == 200) {
				t.Fatalf("the test OPEN's framing is not the case's (extended=%v)", extended)
			}
			rr.handleStructuredOpen(&rpc.StructuredEvent{PeerAddress: openTestPeer}, &bgptypes.RawMessage{RawBytes: body})
			peer := rr.peers[openTestPeer]
			if peer == nil {
				t.Fatal("no entry recorded")
			}
			if len(peer.Families) != 1 || !peer.Families[family.IPv6Unicast] {
				t.Fatalf("families %v, want exactly ipv6/unicast", peer.Families)
			}
			if peer.ASN != 4200000001 {
				t.Fatalf("ASN %d, want 4200000001", peer.ASN)
			}
		})
	}

	refused := []struct {
		name string
		body []byte
	}{
		{"a non-type-2 parameter", openTestBody(t, capability.RawParam(2, []byte{1, 4, 0, 2, 0, 1}), capability.RawParam(7, []byte{1}))},
		{"an empty type-2 parameter", openTestBody(t, capability.RawParam(2, []byte{1, 4, 0, 2, 0, 1}), capability.RawParam(2, nil))},
		{"one malformed type-2 parameter among several", openTestBody(t, capability.RawParam(2, []byte{1, 4, 0, 2, 0, 1}), capability.RawParam(2, []byte{65, 10, 0, 0}))},
		{"a body message.UnpackOpen refuses", []byte{4, 0, 1}},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			log := captureRRLog(t)
			rr := &routeReflector{peers: map[string]*peerState{}}
			rr.handleStructuredOpen(&rpc.StructuredEvent{PeerAddress: openTestPeer}, &bgptypes.RawMessage{RawBytes: tc.body})
			if peer := rr.peers[openTestPeer]; peer != nil {
				t.Fatalf("an unreadable OPEN recorded %+v", peer)
			}
			lines := strings.Split(strings.TrimSpace(log.String()), "\n")
			if len(lines) != 1 {
				t.Fatalf("want one log record, got %q", log.String())
			}
			for _, want := range []string{"level=WARN", "cannot read this peer's OPEN message", "does not come up", "Next step:", "peer=" + openTestPeer, "error="} {
				if !strings.Contains(lines[0], want) {
					t.Fatalf("record %q lacks %q", lines[0], want)
				}
			}
		})
	}
}

// TestRouteReflectorPeerStatusFamilies proves the families key of show rr
// peers: present and sorted for an up peer with a recorded set, absent for a
// down peer and for an up peer with no OPEN recorded.
func TestRouteReflectorPeerStatusFamilies(t *testing.T) {
	rr := &routeReflector{peers: map[string]*peerState{
		"192.0.2.1": {Address: "192.0.2.1", Up: true, Families: map[family.Family]bool{family.IPv6Unicast: true, family.IPv4Unicast: true}},
		"192.0.2.2": {Address: "192.0.2.2", Up: true, Families: map[family.Family]bool{family.IPv4Unicast: true}},
		"192.0.2.3": {Address: "192.0.2.3", Up: false, Families: map[family.Family]bool{family.IPv6Unicast: true}},
		"192.0.2.4": {Address: "192.0.2.4", Up: true},
	}}
	want := map[string][]string{
		"192.0.2.1": {"ipv4/unicast", "ipv6/unicast"},
		"192.0.2.2": {"ipv4/unicast"},
	}
	status, ok := rr.peerStatus().(map[string]any)
	if !ok {
		t.Fatal("peerStatus is not a map")
	}
	rows, ok := status["peers"].([]map[string]any)
	if !ok {
		t.Fatal("peers is not a row list")
	}
	for _, row := range rows {
		address, _ := row["address"].(string)
		got, present := row["families"]
		expected, owed := want[address]
		if present != owed {
			t.Fatalf("%s: families present=%v, want %v", address, present, owed)
		}
		if !owed {
			continue
		}
		names, ok := got.([]string)
		if !ok {
			t.Fatalf("%s: families is %T, want []string", address, got)
		}
		if strings.Join(names, ",") != strings.Join(expected, ",") {
			t.Fatalf("%s: families %v, want %v", address, names, expected)
		}
	}
}
