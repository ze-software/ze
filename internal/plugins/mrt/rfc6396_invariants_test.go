// Design: docs/architecture/mrt.md -- MRT format invariants for RFC 6396
// RFC: rfc/short/rfc6396.md -- Sections 4.2, 4.4.2, 4.4.3, and Appendix B.1
// Related: dump.go, component.go

package mrt

import (
	"bytes"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC6396NoTableDumpV1Emitted proves that Ze's RIB dump path never writes
// deprecated Type 12 (TABLE_DUMP) records, so no 4-byte AS_PATH can ever be
// emitted inside a TABLE_DUMP record.
//
// RFC 6396 Section 4.2: "The AS_PATH attribute MUST only consist of 2-byte AS
// numbers."
//
// RFC requirement: RFC6396-4.2-1 positive -- writeTableDumpV2 emits only
// TypeTableDumpV2 (13) records and never TypeTableDump (12), for both 2-byte
// and 4-byte peer ASNs and IPv4/IPv6 prefixes.
func TestRFC6396NoTableDumpV1Emitted(t *testing.T) {
	t.Parallel()

	order, _, ribs := runRIBDumpV2(t, func(v registry.RIBDumpVisitor) {
		p1 := v.OnPeer("192.0.2.1", 65001, [4]byte{1, 1, 1, 1}, false)
		p2 := v.OnPeer("2001:db8::2", 200000, [4]byte{2, 2, 2, 2}, true)
		v.OnRoute(p1, mrtfmt.AFIIPv4, 1, 24, []byte{203, 0, 113}, testWireASPath4())
		v.OnRoute(p2, mrtfmt.AFIIPv6, 1, 48, []byte{0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}, testWireASPath4())
	})
	if len(order) != 3 || len(ribs) != 2 {
		t.Fatalf("got %d records and %d RIBs, want 3 and 2", len(order), len(ribs))
	}
	for i, h := range order {
		if h.Type == mrtfmt.TypeTableDump {
			t.Fatalf("record %d emitted deprecated TABLE_DUMP (type 12)", i)
		}
		if h.Type != mrtfmt.TypeTableDumpV2 {
			t.Fatalf("record %d type = %d, want TABLE_DUMP_V2 (%d)", i, h.Type, mrtfmt.TypeTableDumpV2)
		}
	}
}

// TestRFC6396NoDeprecatedInformationalTypesEmitted proves that Ze's MRT writers
// never emit deprecated informational MRT types 0..4 (NULL, START, DIE,
// I_AM_DEAD, PEER_DOWN), whose message strings RFC 6396 Appendix B.1 governs.
//
// RFC 6396 Appendix B.1: "The message string encoding MUST follow the UTF-8
// transformation format [RFC3629]."
//
// RFC requirement: RFC6396-B.1-1 positive -- on the default configuration (no
// extended timestamp) a one-route RIB snapshot, one BGP4MP state change and one
// BGP4MP KEEPALIVE are each written with Type >= 11 (TypeOSPFv2), never a
// deprecated informational type 0..4.
func TestRFC6396NoDeprecatedInformationalTypesEmitted(t *testing.T) {
	t.Parallel()

	order, _, _ := runRIBDumpV2(t, func(v registry.RIBDumpVisitor) {
		p := v.OnPeer("192.0.2.1", 65001, [4]byte{1, 1, 1, 1}, false)
		v.OnRoute(p, mrtfmt.AFIIPv4, 1, 24, []byte{203, 0, 113}, testWireASPath4())
	})
	for i, h := range order {
		if h.Type < mrtfmt.TypeOSPFv2 {
			t.Fatalf("RIB record %d emitted deprecated informational type %d", i, h.Type)
		}
	}

	c := New(Config{}, nil)
	path := filepath.Join(t.TempDir(), "all.mrt")
	c.allMsgs = newAsyncWriter(mrtfmt.NewWriter(path), c.logger)
	peer := &plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.0.2.1"),
		LocalAddress: netip.MustParseAddr("192.0.2.2"),
		PeerAS:       rpc.RemoteASOf(65001),
		LocalAS:      65002,
	}
	keepalive := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x13, 0x04,
	}
	c.writeStateChange(peer, mrtfmt.FSMIdle, mrtfmt.FSMEstablished)
	c.OnBGPMessage(peer, msgtype.TypeKEEPALIVE, false, keepalive)
	if err := c.allMsgs.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	count := 0
	err := mrtfmt.ReadFile(path, &mrtfmt.Handler{
		OnHeader: func(h mrtfmt.Header, _ uint32, _ []byte) error {
			if h.Type < mrtfmt.TypeOSPFv2 {
				t.Fatalf("BGP4MP record %d emitted deprecated informational type %d", count, h.Type)
			}
			count++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("read back dump: %v", err)
	}
	if count != 2 {
		t.Fatalf("BGP4MP record count = %d, want 2", count)
	}
}

// buildBGPUpdateMessage wraps a raw BGP UPDATE body with the 19-byte BGP header.
func buildBGPUpdateMessage(attrs, nlri []byte) []byte {
	bodyLen := 2 + 2 + len(attrs) + len(nlri)
	total := 19 + bodyLen
	msg := make([]byte, 0, total)
	msg = append(msg, bytes.Repeat([]byte{0xff}, 16)...)
	msg = append(msg, byte(total>>8), byte(total), 2) // length, type=UPDATE
	msg = append(msg, 0, 0)                           // withdrawn len = 0
	msg = append(msg, byte(len(attrs)>>8), byte(len(attrs)))
	msg = append(msg, attrs...)
	msg = append(msg, nlri...)
	return msg
}

// TestRFC6396BGP4MPSubtypeMatchesSessionASWidth verifies that OnBGPMessage picks
// the 2-byte BGP4MP_MESSAGE subtype for 2-byte AS sessions and the 4-byte
// BGP4MP_MESSAGE_AS4 subtype for 4-byte AS sessions, so neither subtype ever
// carries an AS_PATH of the wrong octet width.
//
// RFC 6396 Section 4.4.2: "The BGP4MP_MESSAGE Subtype does not support 4-byte
// AS numbers.  The AS_PATH contained in these messages MUST only consist of
// 2-byte AS numbers."
// RFC 6396 Section 4.4.3: "The AS_PATH in these messages MUST only consist of
// 4-byte AS numbers."
//
// RFC requirement: RFC6396-4.4.2-1 positive -- a 2-byte AS session (ASN4=false) emits BGP4MP_MESSAGE (subtype 1 received, 6 sent) carrying the 2-byte AS_PATH.
// RFC requirement: RFC6396-4.4.2-1 negative -- a 4-byte AS session (ASN4=true) never emits BGP4MP_MESSAGE, using BGP4MP_MESSAGE_AS4 instead.
// RFC requirement: RFC6396-4.4.3-1 positive -- a 4-byte AS session (ASN4=true) emits BGP4MP_MESSAGE_AS4 (subtype 4 received, 7 sent) carrying the 4-byte AS_PATH.
// RFC requirement: RFC6396-4.4.3-1 negative -- a 2-byte AS session (ASN4=false) never emits BGP4MP_MESSAGE_AS4, using BGP4MP_MESSAGE instead so a 2-byte AS_PATH is never mislabeled as AS4.
func TestRFC6396BGP4MPSubtypeMatchesSessionASWidth(t *testing.T) {
	t.Parallel()

	ctx2, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(false))
	if err != nil {
		t.Fatalf("register 2-byte context: %v", err)
	}
	ctx4, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatalf("register 4-byte context: %v", err)
	}

	asPath2 := []byte{0x40, 0x02, 0x06, 0x02, 0x02, 0xfd, 0xe9, 0xfd, 0xea}
	asPath4 := testWireASPath4() // AS_SEQUENCE {65001, 200000}
	nlri := []byte{24, 203, 0, 113}
	update2 := buildBGPUpdateMessage(asPath2, nlri)
	update4 := buildBGPUpdateMessage(asPath4, nlri)

	c := New(Config{}, nil)
	path := filepath.Join(t.TempDir(), "updates.mrt")
	c.allMsgs = newAsyncWriter(mrtfmt.NewWriter(path), c.logger)

	peer2 := &plugin.PeerInfo{
		Address:          netip.MustParseAddr("192.0.2.1"),
		LocalAddress:     netip.MustParseAddr("192.0.2.2"),
		PeerAS:           rpc.RemoteASOf(65001),
		LocalAS:          65002,
		MessageContextID: ctx2,
	}
	peer4 := &plugin.PeerInfo{
		Address:          netip.MustParseAddr("192.0.2.3"),
		LocalAddress:     netip.MustParseAddr("192.0.2.2"),
		PeerAS:           rpc.RemoteASOf(200000),
		LocalAS:          65002,
		MessageContextID: ctx4,
	}

	c.OnBGPMessage(peer2, msgtype.TypeUPDATE, false, update2)
	c.OnBGPMessage(peer2, msgtype.TypeUPDATE, true, update2)
	c.OnBGPMessage(peer4, msgtype.TypeUPDATE, false, update4)
	c.OnBGPMessage(peer4, msgtype.TypeUPDATE, true, update4)
	if err := c.allMsgs.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	var gotSubtypes []uint16
	var gotMessages [][]byte
	err = mrtfmt.ReadFile(path, &mrtfmt.Handler{
		OnMessage: func(h mrtfmt.Header, _ uint32, m *mrtfmt.MessageRecord) error {
			gotSubtypes = append(gotSubtypes, h.Subtype)
			gotMessages = append(gotMessages, bytes.Clone(m.BGPMessage.Bytes))
			return nil
		},
	})
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}

	wantSubtypes := []uint16{
		mrtfmt.BGP4MPMessage,
		mrtfmt.BGP4MPMessageLocal,
		mrtfmt.BGP4MPMessageAS4,
		mrtfmt.BGP4MPMessageAS4Local,
	}
	if len(gotSubtypes) != len(wantSubtypes) {
		t.Fatalf("record count = %d, want %d", len(gotSubtypes), len(wantSubtypes))
	}
	for i, want := range wantSubtypes {
		if gotSubtypes[i] != want {
			t.Errorf("record %d subtype = %d, want %d", i, gotSubtypes[i], want)
		}
	}
	if !bytes.Equal(gotMessages[0], update2) || !bytes.Equal(gotMessages[1], update2) {
		t.Errorf("2-byte session records did not preserve 2-byte UPDATE verbatim")
	}
	if !bytes.Equal(gotMessages[2], update4) || !bytes.Equal(gotMessages[3], update4) {
		t.Errorf("4-byte session records did not preserve 4-byte UPDATE verbatim")
	}
}
