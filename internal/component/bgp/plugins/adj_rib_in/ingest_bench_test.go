// VALIDATES: what storing one UPDATE costs on handleReceivedStructured, the bridge
// ingest path, for 800 plain IPv4 unicast prefixes and for 800 labeled unicast and
// 800 VPN routes, and what a VPN announcement followed by its withdrawal costs.
// PREVENTS: a route key or framing change for one family taxing the ingest of
// every family unnoticed. The unicast key is the per-route map key of the
// commonest family, so its size is paid on every prefix of a full table.

package adj_rib_in

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"unsafe"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRouteKeyStaysFortyEightBytes pins the size of the per-route map key. Every
// stored route of every family carries one, so sixteen more bytes cost a full
// IPv4 table 16 MB.
// MUTATION: moving PathID after Prefix pads the key to 56 bytes; a string for
// NLRI grows it to 64.
func TestRouteKeyStaysFortyEightBytes(t *testing.T) {
	if got := unsafe.Sizeof(compactRouteKey{}); got != 48 {
		t.Fatalf("compactRouteKey is %d bytes, want 48", got)
	}
}

// benchRoutes is the route count of one benchmark UPDATE.
const benchRoutes = 800

func benchEvent(b *testing.B, peer netip.Addr, body []byte) *rpc.StructuredEvent {
	b.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		b.Fatal(err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	if err != nil {
		b.Fatal(err)
	}
	return &rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: peer.String(),
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	}
}

// unicastBody is an UPDATE announcing benchRoutes IPv4 /24s in its NLRI field.
func unicastBody() []byte {
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 10, 0, 0, 1}
	body := []byte{0, 0, 0, 0}
	binary.BigEndian.PutUint16(body[2:], uint16(len(attrs)))
	body = append(body, attrs...)
	for i := range benchRoutes {
		body = append(body, 24, 10, byte(i>>8), byte(i))
	}
	return body
}

func labeledRoutes() [][]byte {
	out := make([][]byte, 0, benchRoutes)
	for i := range benchRoutes {
		out = append(out, labeledNLRI(labelOneBottom, 10, byte(i>>8), byte(i)))
	}
	return out
}

func vpnRoutes() [][]byte {
	out := make([][]byte, 0, benchRoutes)
	for i := range benchRoutes {
		out = append(out, vpnNLRI(labelOneBottom, rd(1), 10, byte(i>>8), byte(i)))
	}
	return out
}

// vpnWithdrawn is each of vpnRoutes with the RFC 8277 Compatibility field where
// the label stack was.
func vpnWithdrawn() [][]byte {
	out := make([][]byte, 0, benchRoutes)
	for i := range benchRoutes {
		n := append([]byte{24 + 64 + 24}, compatibility...)
		n = append(n, rd(1)...)
		out = append(out, append(n, 10, byte(i>>8), byte(i)))
	}
	return out
}

func benchIngest(b *testing.B, body []byte) {
	b.Helper()
	r := benchManager(b)
	source := netip.MustParseAddr("192.0.2.1")
	se := benchEvent(b, source, body)
	r.handleReceivedStructured(se)
	if got := r.ribIn[source].Len(); got != benchRoutes {
		b.Fatalf("stored %d routes, want %d", got, benchRoutes)
	}
	b.ReportAllocs()
	for b.Loop() {
		r.handleReceivedStructured(se)
	}
}

func BenchmarkIngestIPv4Unicast800(b *testing.B) {
	benchIngest(b, unicastBody())
}

func BenchmarkIngestLabeled800(b *testing.B) {
	benchIngest(b, mpAnnounce(ipv4Labeled, []byte{10, 0, 0, 1}, labeledRoutes()...))
}

func BenchmarkIngestVPN800(b *testing.B) {
	benchIngest(b, mpAnnounce(ipv4VPN, make([]byte, 12), vpnRoutes()...))
}

// BenchmarkAnnounceWithdrawVPN800 stores 800 VPN routes and withdraws them, so
// every withdrawal names a stored route.
func BenchmarkAnnounceWithdrawVPN800(b *testing.B) {
	r := benchManager(b)
	source := netip.MustParseAddr("192.0.2.1")
	announce := benchEvent(b, source, mpAnnounce(ipv4VPN, make([]byte, 12), vpnRoutes()...))
	withdraw := benchEvent(b, source, mpWithdraw(ipv4VPN, vpnWithdrawn()...))
	r.handleReceivedStructured(announce)
	r.handleReceivedStructured(withdraw)
	if got := r.ribIn[source].Len(); got != 0 {
		b.Fatalf("%d routes left after the withdrawal, want 0", got)
	}
	b.ReportAllocs()
	for b.Loop() {
		r.handleReceivedStructured(announce)
		r.handleReceivedStructured(withdraw)
	}
}
