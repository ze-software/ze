package engine

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// recordingTransport keeps the last Outbound the Loop handed it.
type recordingTransport struct {
	last transport.Outbound
	sent int
}

func (*recordingTransport) Start() error                 { return nil }
func (*recordingTransport) Stop() error                  { return nil }
func (*recordingTransport) RX() <-chan transport.Inbound { return nil }
func (r *recordingTransport) Send(out transport.Outbound) error {
	r.last = out
	r.sent++
	return nil
}

// VALIDATES: sendLocked hands the transport the session key's local address as
// the packet's source, for a multi-hop session pinned to a local address and
// for a single-hop one, and hands it no source for a session whose key carries
// none. Method: a Loop over a transport that records the Outbound, one Control
// packet per session. The transport half (the address on the wire) is
// TestUDPSendLeavesFromThePinnedLocal.
// PREVENTS: the L3 defect of spec-bgp-live-receive-binding-and-bfd-keep-session
// (journal unwired-feature.md, 2026-10-12): Outbound carried no source, so a
// session pinned to 127.0.0.2 left from the kernel's choice and its first
// packets demuxed onto the peer's session for 127.0.0.1 (RFC 5883 Section 4.1).
func TestSendLockedCarriesTheKeyLocal(t *testing.T) {
	pinnedMultiHop := reqFor(addrB, "127.0.0.2")
	pinnedMultiHop.Interface = ""
	pinnedMultiHop.Mode = api.MultiHop
	unpinnedMultiHop := pinnedMultiHop
	unpinnedMultiHop.Peer = netip.MustParseAddr("192.0.2.9")
	unpinnedMultiHop.Local = netip.Addr{}
	cases := []struct {
		name string
		req  api.SessionRequest
		want netip.Addr
	}{
		{name: "multi-hop pinned", req: pinnedMultiHop, want: netip.MustParseAddr("127.0.0.2")},
		{name: "single-hop", req: reqFor(addrB, addrA), want: netip.MustParseAddr(addrA)},
		{name: "multi-hop unpinned", req: unpinnedMultiHop, want: netip.Addr{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingTransport{}
			l := NewLoop(rec, &steppedClock{now: time.Unix(1_000_000, 0)})
			if _, err := l.EnsureSession(tc.req); err != nil {
				t.Fatalf("EnsureSession: %v", err)
			}
			l.mu.Lock()
			entry := l.sessions[tc.req.Key()]
			l.sendLocked(entry, entry.machine.Build())
			l.mu.Unlock()
			if rec.sent != 1 {
				t.Fatalf("transport got %d packets, want 1", rec.sent)
			}
			if rec.last.Local != tc.want {
				t.Errorf("Outbound.Local = %v, want %v", rec.last.Local, tc.want)
			}
		})
	}
}
