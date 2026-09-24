// Design: docs/architecture/core-design.md — semantic UPDATE receive validation
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md — neighbor-AS match
// Related: session_validation.go — firstASMismatch
// Related: session_read.go — processMessage withdrawal synthesis

package reactor

import (
	"bytes"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// firstASSession opens a real receive session. Plugin capabilities enter its
// transmitted OPEN, while peerCaps arrive through ReadAndProcess on the socket.
func firstASSession(t *testing.T, settings *PeerSettings, peerCaps, pluginCaps []capability.Capability) (*Session, net.Conn) {
	t.Helper()
	settings.Connection = ConnectionPassive
	s := NewSession(settings)
	s.SetPluginCapabilityGetter(func() []capability.Capability { return pluginCaps })
	client, server := net.Pipe()
	t.Cleanup(func() {
		s.timers.StopAll()
		s.stopSendHoldTimer()
		_ = client.Close()
		_ = server.Close()
	})
	if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := server.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	startSession(t, s)
	readOpen(t, s, server, client)
	params, extended := buildOptionalParams(peerCaps)
	open := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 0, BGPIdentifier: 0x02020302,
		OptionalParams: params, ExtendedParams: extended,
	}
	if settings.PeerAS != 0 {
		open.ASN4 = settings.PeerAS
		if settings.PeerAS <= 65535 {
			open.MyAS = uint16(settings.PeerAS)
		} else {
			open.MyAS = 23456
		}
	}
	written := make(chan error, 1)
	go func() {
		_, err := client.Write(message.PackTo(open, nil))
		if err == nil {
			var keepalive [message.HeaderLen]byte
			_, err = io.ReadFull(client, keepalive[:])
		}
		written <- err
	}()
	if err := s.ReadAndProcess(); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	go func() {
		_, err := client.Write(message.PackTo(message.NewKeepalive(), nil))
		written <- err
	}()
	if err := s.ReadAndProcess(); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	ctxID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedRecv(s.Negotiated()))
	if err != nil {
		t.Fatal(err)
	}
	s.SetRecvCtxID(ctxID)
	return s, client
}

// firstASReceive captures the semantic UPDATE delivered to consumers, not the
// raw observer bytes. A dropped UPDATE therefore cannot pass as a withdrawal.
func firstASReceive(t *testing.T, s *Session, client net.Conn, body []byte) *wireu.WireUpdate {
	t.Helper()
	var received []*wireu.WireUpdate
	s.SetMessageCallback(func(_ netip.Addr, kind msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64,
	) bool {
		if kind == msgtype.TypeUPDATE && direction == rpc.DirectionReceived {
			received = append(received, wu.Snapshot())
		}
		return false
	})
	written := make(chan error, 1)
	go func() {
		_, err := client.Write(buildUpdateMsg(body))
		written <- err
	}()
	if err := s.ReadAndProcess(); err != nil {
		t.Fatalf("receive: %v", err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if s.State() != fsm.StateEstablished {
		t.Fatalf("session is %v, want Established", s.State())
	}
	if len(received) != 1 {
		t.Fatalf("received %d semantic UPDATEs, want one", len(received))
	}
	return received[0]
}

// firstASAttrs constructs the mandatory IPv4 attributes with one AS_SEQUENCE.
func firstASAttrs(octets int, asns ...uint32) []byte {
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	path := collapseASPathValue(octets, asns...)
	if len(asns) == 0 {
		path = nil
	}
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath), path)...)
	return append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
}

// TestSessionASPAFirstAS replaces a previously announced route with a mismatched
// path and checks the exact synthesized withdrawal, then announces it again.
// A mismatched first AS withdraws the previous route; a matching path remains
// announced with both four-octet and legacy two-octet ASNs.
func TestSessionASPAFirstAS(t *testing.T) {
	cases := []struct {
		name   string
		peer   uint32
		octets int
	}{
		{"four-octet", 65002, 4},
		{"legacy", 65002, 2},
		{"large-neighbor-AS", 4200000123, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, tc.peer, 0x01020301)
			var caps []capability.Capability
			if tc.octets == 4 {
				caps = append(caps, &capability.ASN4{ASN: tc.peer})
			}
			s, client := firstASSession(t, settings, caps, nil)
			prefix := []byte{24, 203, 0, 113}
			for _, asn := range []uint32{tc.peer, 65003, tc.peer} {
				wu := firstASReceive(t, s, client, makeUpdateBody(nil, firstASAttrs(tc.octets, asn), prefix))
				if asn != tc.peer {
					want := makeUpdateBody(prefix, nil, nil)
					if !bytes.Equal(wu.Payload(), want) {
						t.Fatalf("mismatch delivered %x, want withdrawal %x", wu.Payload(), want)
					}
					continue
				}
				nlri, err := wu.NLRI()
				if err != nil || !bytes.Equal(nlri, prefix) {
					t.Fatalf("matching path announced %x, error %v", nlri, err)
				}
			}
		})
	}
}

// TestSessionASPAFirstASRoles tests directionality at OPEN setup. In loose mode
// the local RS-client exception also applies when the server sends no Role.
// A local route server still checks client paths; a remote RS Role alone does
// not grant the local RS-client exception.
func TestSessionASPAFirstASRoles(t *testing.T) {
	cases := []struct {
		name          string
		local, remote byte
		accept        bool
	}{
		{"RS-client-to-RS", 2, 1, true},
		{"RS-client-loose", 2, 255, true},
		{"RS-to-client", 1, 2, false},
		{"remote-RS-only", 255, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
			var local, remote []capability.Capability
			var err error
			if tc.local != 255 {
				local, err = capability.ParseFromOptionalParams([]byte{2, 3, 9, 1, tc.local}, false)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.remote != 255 {
				remote, err = capability.ParseFromOptionalParams([]byte{2, 3, 9, 1, tc.remote}, false)
				if err != nil {
					t.Fatal(err)
				}
			}
			remote = append(remote, &capability.ASN4{ASN: 65002})
			s, client := firstASSession(t, settings, remote, local)
			prefix := []byte{24, 203, 0, 113}
			body := makeUpdateBody(nil, firstASAttrs(4, 65003), prefix)
			wu := firstASReceive(t, s, client, body)
			want := makeUpdateBody(prefix, nil, nil)
			if tc.accept {
				want = body
			}
			if !bytes.Equal(wu.Payload(), want) {
				t.Fatalf("received %x, want %x", wu.Payload(), want)
			}
		})
	}
}

// TestSessionASPAFirstASReconstructed makes the canonical first AS disagree with
// the two-octet AS_PATH, proving validation runs after RFC 6793 reconstruction.
// An AS 0 in AS4_PATH must be discarded before that reconstruction can use it.
func TestSessionASPAFirstASReconstructed(t *testing.T) {
	cases := []struct {
		name  string
		first uint32
	}{
		{"matching", 65002},
		{"mismatching", 65003},
		{"AS-zero-discarded", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
			s, client := firstASSession(t, settings, nil, nil)
			attrs := firstASAttrs(2, 65002, 23456)
			attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Path),
				collapseASPathValue(4, tc.first, 4200000123))...)
			prefix := []byte{24, 203, 0, 113}
			wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, prefix))
			if tc.first == 65003 {
				if !bytes.Equal(wu.Payload(), makeUpdateBody(prefix, nil, nil)) {
					t.Fatalf("reconstructed mismatch was not withdrawn: %x", wu.Payload())
				}
				return
			}
			nlri, err := wu.NLRI()
			if err != nil || !bytes.Equal(nlri, prefix) {
				t.Fatalf("usable path announced %x, error %v", nlri, err)
			}
			path, present := collapseAttrValue(t, wu, attribute.AttrASPath)
			last := uint32(4200000123)
			if tc.first == 0 {
				last = 23456
			}
			if !present || !bytes.Equal(path, collapseASPathValue(4, 65002, last)) {
				t.Fatalf("canonical path is %x", path)
			}
			if _, present := collapseAttrValue(t, wu, attribute.AttrAS4Path); present {
				t.Fatal("AS4_PATH survived reconciliation")
			}
		})
	}
}

// TestSessionASPAFirstASMPAddPath checks the withdrawal's family, prefix and
// Path Identifier after a mismatch, not only the IPv4 body field.
func TestSessionASPAFirstASMPAddPath(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	caps := []capability.Capability{
		&capability.ASN4{ASN: 65002},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		&capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast, Mode: capability.AddPathBoth},
		}},
	}
	settings.Capabilities = caps
	s, client := firstASSession(t, settings, caps, nil)
	prefix := []byte{0, 0, 0, 81, 32, 0x20, 0x01, 0x0d, 0xb8}
	reach := append([]byte{0, 2, 1, 16}, make([]byte, 16)...)
	reach[19] = 1
	reach = append(reach, 0)
	reach = append(reach, prefix...)
	for _, first := range []uint32{65002, 65003} {
		attrs := append(firstASAttrs(4, first), collapseAttr(0x80, 14, reach)...)
		wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, nil))
		if first == 65002 {
			mp, err := wu.MPReach()
			if err != nil || !bytes.Equal(mp.NLRIBytes(), prefix) {
				t.Fatalf("matching MP path lost reachability: %x, %v", wu.Payload(), err)
			}
			continue
		}
		unreach := append([]byte{0, 2, 1}, prefix...)
		want := makeUpdateBody(nil, collapseAttr(0x80, 15, unreach), nil)
		if !bytes.Equal(wu.Payload(), want) {
			t.Fatalf("MP mismatch delivered %x, want withdrawal %x", wu.Payload(), want)
		}
	}
}

// TestSessionASPAFirstASParserRewrites keeps duplicate stripping and attribute
// discard observable while the neighbor-AS rule processes the same UPDATE.
func TestSessionASPAFirstASParserRewrites(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s, client := firstASSession(t, settings, []capability.Capability{&capability.ASN4{ASN: 65002}}, nil)
	prefix := []byte{24, 203, 0, 113}
	for _, first := range []uint32{65002, 65003} {
		attrs := firstASAttrs(4, first)
		attrs = append(attrs, collapseAttr(0x40, 2, collapseASPathValue(4, 65002))...)
		attrs = append(attrs, collapseAttr(0x40, 5, []byte{0, 0, 0, 100})...)
		body := makeUpdateBody(nil, attrs, prefix)
		wu := firstASReceive(t, s, client, body)
		if first == 65003 {
			if !bytes.Equal(wu.Payload(), makeUpdateBody(prefix, nil, nil)) {
				t.Fatalf("a duplicate AS_PATH hid the first mismatch: %x", wu.Payload())
			}
			continue
		}
		nlri, err := wu.NLRI()
		if err != nil || !bytes.Equal(nlri, prefix) {
			t.Fatalf("matching rewritten UPDATE lost announcement: %x, %v", nlri, err)
		}
		attrs = rfc8669PathAttrs(t, wu.Payload())
		if count, _ := countAttrCode(attrs, 2); count != 1 {
			t.Fatalf("kept %d AS_PATH attributes", count)
		}
		if count, _ := countAttrCode(attrs, 5); count != 0 {
			t.Fatal("eBGP LOCAL_PREF survived attribute discard")
		}
		entries := message.ExtractUpstreamAttrDiscard(attrs)
		if len(entries) != 1 || entries[0].Code != 5 || entries[0].Reason != message.DiscardReasonEBGPInvalid {
			t.Fatalf("discard metadata changed: %+v", entries)
		}
	}
}

// TestSessionASPAFirstASEmptyAndASZero ensures semantic checking does not make
// AS 0 legal when it follows a matching neighbor, or accept an empty eBGP path.
func TestSessionASPAFirstASEmptyAndASZero(t *testing.T) {
	for _, path := range [][]uint32{nil, {65002, 0}} {
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
		s, client := firstASSession(t, settings, []capability.Capability{&capability.ASN4{ASN: 65002}}, nil)
		prefix := []byte{24, 203, 0, 113}
		wu := firstASReceive(t, s, client, makeUpdateBody(nil, firstASAttrs(4, path...), prefix))
		if !bytes.Equal(wu.Payload(), makeUpdateBody(prefix, nil, nil)) {
			t.Fatalf("path %v was not withdrawn: %x", path, wu.Payload())
		}
	}
}

// TestSessionASPAFirstASDynamicAndInternal checks the OPEN-derived identity of a
// dynamic neighbor and the iBGP exception, which must not depend on Role.
func TestSessionASPAFirstASDynamicAndInternal(t *testing.T) {
	for _, internal := range []bool{false, true} {
		localAS := uint32(65001)
		if internal {
			localAS = 65002
		}
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), localAS, 0, 0x01020301)
		if internal {
			settings.PeerAS = 65002
		}
		s, client := firstASSession(t, settings, []capability.Capability{&capability.ASN4{ASN: 65002}}, nil)
		prefix := []byte{24, 203, 0, 113}
		attrs := firstASAttrs(4, 65003)
		if internal {
			attrs = append(attrs, collapseAttr(0x40, 5, []byte{0, 0, 0, 100})...)
		}
		body := makeUpdateBody(nil, attrs, prefix)
		wu := firstASReceive(t, s, client, body)
		want := makeUpdateBody(prefix, nil, nil)
		if internal {
			want = body
		}
		if !bytes.Equal(wu.Payload(), want) {
			t.Fatalf("internal=%v: delivered %x, want %x", internal, wu.Payload(), want)
		}
	}
}

// TestSessionASPAFirstASDoesNotDowngradeReset sends both a neighbor-AS mismatch
// and duplicate MP_REACH attributes. The parser's stronger reset still wins.
func TestSessionASPAFirstASDoesNotDowngradeReset(t *testing.T) {
	reach := []byte{0, 1, 1, 4, 192, 0, 2, 254, 0, 24, 203, 0, 113}
	attrs := firstASAttrs(4, 65003)
	attrs = append(attrs, mpAttr(14, reach)...)
	attrs = append(attrs, mpAttr(14, reach)...)
	requireSessionResetOnWire(t, makeUpdateBody(nil, attrs, nil))
}
