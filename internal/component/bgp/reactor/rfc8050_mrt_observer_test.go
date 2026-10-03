// Design: docs/architecture/mrt.md — reactor callback to MRT framing and context.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/mrt"
	mrtplugin "github.com/ze-software/ze/internal/plugins/mrt"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8050MessageCallbackPreservesEncodingAndFraming sends complete original
// packets through the raw observer boundary to a running MRT component.
// RFC requirement: RFC8050-x-4 positive -- the raw receive/send observer carries its registered encoding context to MRT; each isolated classic or MP NLRI family selects ADDPATH only for its own direction, preserving the complete original packet.
// RFC requirement: RFC8050-x-1 positive -- BGP4MP and BGP4MP_ET writers signal negotiated ADD-PATH for classic and MP announcements and withdrawals through subtypes 9 and 11, preserving the entire framed BGP message.
// RFC requirement: RFC8050-x-1 negative -- a different family's ADD-PATH negotiation leaves ordinary UPDATE messages under their base subtypes 4 and 7 rather than falsely marking their NLRI as carrying Path Identifiers.
// RFC requirement: RFC8050-x-4 negative -- ADD-PATH negotiated solely for a different AFI/SAFI does not mark the current family as ADDPATH; base subtypes retain the ordinary message bytes in both callback directions.
func TestRFC8050MessageCallbackPreservesEncodingAndFraming(t *testing.T) {
	for _, extended := range []bool{false, true} {
		r := New(&Config{})
		addr := netip.MustParseAddr("192.0.2.1")
		settings := NewPeerSettings(addr, 65000, 65001, 0xc0000202)
		settings.LocalAddress = netip.MustParseAddr("192.0.2.2")
		if err := r.AddPeer(settings); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "messages.mrt")
		recorder := mrtplugin.New(mrtplugin.Config{AllPath: path, ExtendedTimestamp: extended}, nil)
		recorder.Start(nil)
		// Start MUST be paired with Stop even when an assertion terminates the test.
		stop := sync.OnceFunc(recorder.Stop)
		t.Cleanup(stop)
		r.addMessageObserver(recorder)
		type expected struct {
			subtype uint16
			message []byte
		}
		var wants []expected
		for _, direction := range []rpc.MessageDirection{rpc.DirectionReceived, rpc.DirectionSent} {
			for _, location := range []string{"announced", "withdrawn", "reach", "unreach"} {
				for _, addPath := range []bool{false, true} {
					fam := family.IPv4Unicast
					other := family.IPv6Unicast
					if location == "reach" || location == "unreach" {
						fam, other = other, fam
					}
					modes := map[family.Family]bool{other: true}
					if addPath {
						modes = map[family.Family]bool{fam: true}
					}
					ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, modes))
					if err != nil {
						t.Fatal(err)
					}
					body := mrtObserverBody(location, addPath)
					subtype := mrt.BGP4MPMessageAS4
					if addPath {
						subtype = mrt.BGP4MPMessageAS4AP
					}
					if direction == rpc.DirectionSent {
						if addPath {
							subtype = mrt.BGP4MPMessageAS4LocalAP
						} else {
							subtype = mrt.BGP4MPMessageAS4Local
						}
					}
					message := bytes.Repeat([]byte{0xff}, 16)
					message = binary.BigEndian.AppendUint16(message, uint16(19+len(body)))
					message = append(message, 2)
					message = append(message, body...)
					r.notifyWireMessage(&plugin.PeerInfo{Address: addr, LocalAddress: settings.LocalAddress, PeerAS: 65001, LocalAS: 65000, MessageContextID: ctxID}, message, direction == rpc.DirectionSent)
					wants = append(wants, expected{subtype: subtype, message: message})
				}
			}
		}
		// The producer MUST flush before the reader inspects its output.
		stop()
		seen := 0
		err := mrt.ReadFile(path, &mrt.Handler{OnMessage: func(h mrt.Header, _ uint32, record *mrt.MessageRecord) error {
			if seen >= len(wants) {
				t.Fatal("unexpected extra MRT record")
			}
			want := wants[seen]
			seen++
			typ := mrt.TypeBGP4MP
			if extended {
				typ = mrt.TypeBGP4MPET
			}
			if h.Type != typ || h.Subtype != want.subtype || !bytes.Equal(record.BGPMessage.Bytes, want.message) {
				t.Fatalf("record %d type/subtype=%d/%d bytes=%x; want %d/%d %x", seen, h.Type, h.Subtype, record.BGPMessage.Bytes, typ, want.subtype, want.message)
			}
			if _, err := mrt.ParseBGPMessage(record.BGPMessage); err != nil {
				t.Fatalf("callback record cannot be parsed: %v", err)
			}
			return nil
		}})
		if err != nil || seen != len(wants) {
			t.Fatalf("MRT reader count=%d want=%d error=%v", seen, len(wants), err)
		}
	}
}

// mrtObserverBody isolates each NLRI location so another family's negotiation
// cannot accidentally make a wrong subtype selection pass.
func mrtObserverBody(location string, addPath bool) []byte {
	var nlri []byte
	if addPath {
		nlri = append(nlri, 0, 0, 0, 1)
	}
	if location == "announced" || location == "withdrawn" {
		nlri = append(nlri, 24, 10, 0, 0)
		if location == "announced" {
			return append([]byte{0, 0, 0, 0}, nlri...)
		}
		body := append([]byte{0, byte(len(nlri))}, nlri...)
		return append(body, 0, 0)
	}
	nlri = append(nlri, 32, 0x20, 1, 0x0d, 0xb8)
	value := []byte{0, 2, 1}
	code := byte(15)
	if location == "reach" {
		value = append(value, 16, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0)
		code = 14
	}
	value = append(value, nlri...)
	body := []byte{0, 0, 0, byte(3 + len(value)), 0x80, code, byte(len(value))}
	return append(body, value...)
}
