// Design: docs/architecture/mrt.md -- MRT BGP4MP 2-byte AS ADD-PATH emission
// RFC: rfc/short/rfc8050.md -- Section 2 (BGP4MP_MESSAGE_ADDPATH / LOCAL_ADDPATH)
// Related: component.go, dump.go

package mrt

import (
	"bytes"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8050BGP4MPAddPathAS2Subtypes verifies that OnBGPMessage emits the
// 2-byte AS ADD-PATH subtypes (BGP4MP_MESSAGE_ADDPATH = 8 received and
// BGP4MP_MESSAGE_LOCAL_ADDPATH = 10 sent) for 2-byte AS ADD-PATH sessions
// across both BGP4MP and BGP4MP_ET, and emits plain 2-byte BGP4MP_MESSAGE (1) /
// BGP4MP_MESSAGE_LOCAL (6) when ADD-PATH is not negotiated.
//
// RFC 8050 Section 2: "The Advertisement of Multiple Paths [RFC7911] extension
// for BGP alters the encoding of the BGP Network Layer Reachability Information
// (NLRI) format for withdraws and announcements. Therefore, new
// BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to
// an MRT parser how to parse the NLRI."
//
// RFC requirement: RFC8050-x-1 positive -- a 2-byte AS session (ASN4=false) carrying ADD-PATH IPv4 or IPv6 NLRI emits BGP4MP_MESSAGE_ADDPATH (subtype 8 received) and BGP4MP_MESSAGE_LOCAL_ADDPATH (subtype 10 sent) for both BGP4MP and BGP4MP_ET records.
// RFC requirement: RFC8050-x-1 negative -- a 2-byte AS session (ASN4=false) without ADD-PATH emits BGP4MP_MESSAGE (subtype 1 received) and BGP4MP_MESSAGE_LOCAL (subtype 6 sent) rather than ADD-PATH or AS4 subtypes.
func TestRFC8050BGP4MPAddPathAS2Subtypes(t *testing.T) {
	t.Parallel()

	ctxAS2AddPath, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(false, map[family.Family]bool{
		family.IPv4Unicast: true,
		family.IPv6Unicast: true,
	}))
	if err != nil {
		t.Fatalf("register 2-byte ADD-PATH context: %v", err)
	}
	ctxAS2Plain, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(false))
	if err != nil {
		t.Fatalf("register 2-byte plain context: %v", err)
	}

	asPath2 := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0xfd, 0xe9} // AS_SEQUENCE {65001}
	ipv4AddPathNLRI := []byte{0x00, 0x00, 0x00, 0x07, 24, 203, 0, 113}
	ipv4PlainNLRI := []byte{24, 203, 0, 113}

	// MP_REACH_NLRI for IPv6 unicast carrying Path ID 9 + 2001:db8:1::/48.
	mpReachVal := []byte{
		0x00, 0x02, 0x01, // AFI=2 (IPv6), SAFI=1 (Unicast)
		0x10,                                           // NH len=16
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, // 2001:db8::1
		0x00,                                           // reserved
		0x00, 0x00, 0x00, 0x09,                         // Path ID = 9
		48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01,         // 2001:db8:1::/48
	}
	mpReachAttr := append([]byte{0x80, 0x0e, byte(len(mpReachVal))}, mpReachVal...)

	updV4AP := buildBGPUpdateMessage(asPath2, ipv4AddPathNLRI)
	updV6AP := buildBGPUpdateMessage(append(bytes.Clone(asPath2), mpReachAttr...), nil)
	updV4Plain := buildBGPUpdateMessage(asPath2, ipv4PlainNLRI)

	for _, tc := range []struct {
		name     string
		extTS    bool
		wantType uint16
	}{
		{name: "BGP4MP", extTS: false, wantType: mrtfmt.TypeBGP4MP},
		{name: "BGP4MP_ET", extTS: true, wantType: mrtfmt.TypeBGP4MPET},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := New(Config{ExtendedTimestamp: tc.extTS}, nil)
			path := filepath.Join(t.TempDir(), "as2-addpath.mrt")
			c.allMsgs = newAsyncWriter(mrtfmt.NewWriter(path), c.logger)

			peerAP := &plugin.PeerInfo{
				Address:          netip.MustParseAddr("192.0.2.1"),
				LocalAddress:     netip.MustParseAddr("192.0.2.2"),
				PeerAS:           rpc.RemoteASOf(65001),
				LocalAS:          65002,
				MessageContextID: ctxAS2AddPath,
			}
			peerPlain := &plugin.PeerInfo{
				Address:          netip.MustParseAddr("192.0.2.3"),
				LocalAddress:     netip.MustParseAddr("192.0.2.2"),
				PeerAS:           rpc.RemoteASOf(65003),
				LocalAS:          65002,
				MessageContextID: ctxAS2Plain,
			}

			c.OnBGPMessage(peerAP, msgtype.TypeUPDATE, false, updV4AP)
			c.OnBGPMessage(peerAP, msgtype.TypeUPDATE, true, updV4AP)
			c.OnBGPMessage(peerAP, msgtype.TypeUPDATE, false, updV6AP)
			c.OnBGPMessage(peerAP, msgtype.TypeUPDATE, true, updV6AP)
			c.OnBGPMessage(peerPlain, msgtype.TypeUPDATE, false, updV4Plain)
			c.OnBGPMessage(peerPlain, msgtype.TypeUPDATE, true, updV4Plain)
			if err := c.allMsgs.Close(); err != nil {
				t.Fatalf("close writer: %v", err)
			}

			type gotRec struct {
				typ     uint16
				subtype uint16
				peerAS  uint32
				localAS uint32
				msg     []byte
			}
			var got []gotRec
			err := mrtfmt.ReadFile(path, &mrtfmt.Handler{
				OnMessage: func(h mrtfmt.Header, _ uint32, m *mrtfmt.MessageRecord) error {
					got = append(got, gotRec{
						typ:     h.Type,
						subtype: h.Subtype,
						peerAS:  m.PeerAS,
						localAS: m.LocalAS,
						msg:     bytes.Clone(m.BGPMessage.Bytes),
					})
					return nil
				},
			})
			if err != nil {
				t.Fatalf("read back MRT: %v", err)
			}

			want := []struct {
				subtype uint16
				peerAS  uint32
				msg     []byte
			}{
				{mrtfmt.BGP4MPMessageAP, 65001, updV4AP},
				{mrtfmt.BGP4MPMessageLocalAP, 65001, updV4AP},
				{mrtfmt.BGP4MPMessageAP, 65001, updV6AP},
				{mrtfmt.BGP4MPMessageLocalAP, 65001, updV6AP},
				{mrtfmt.BGP4MPMessage, 65003, updV4Plain},
				{mrtfmt.BGP4MPMessageLocal, 65003, updV4Plain},
			}
			if len(got) != len(want) {
				t.Fatalf("record count = %d, want %d", len(got), len(want))
			}
			for i, w := range want {
				if got[i].typ != tc.wantType {
					t.Errorf("record %d type = %d, want %d", i, got[i].typ, tc.wantType)
				}
				if got[i].subtype != w.subtype {
					t.Errorf("record %d subtype = %d, want %d", i, got[i].subtype, w.subtype)
				}
				if got[i].peerAS != w.peerAS || got[i].localAS != 65002 {
					t.Errorf("record %d AS = (%d,%d), want (%d,65002)", i, got[i].peerAS, got[i].localAS, w.peerAS)
				}
				if !bytes.Equal(got[i].msg, w.msg) {
					t.Errorf("record %d BGP message bytes mismatch", i)
				}
			}
		})
	}
}
