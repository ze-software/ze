package l2tp

import (
	"net/netip"
	"testing"
)

var testPeer = netip.MustParseAddrPort("192.0.2.1:1701")

func TestCaptureRingAppend(t *testing.T) {
	r := newCaptureRing()
	r.appendInbound(10, 20, MsgSCCRQ, testPeer, 100, 0)
	r.appendOutbound(10, 20, MsgSCCRP, testPeer, 120)

	snap := r.Snapshot(0, 0, "")
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	if snap[0].MsgType != "SCCRP" {
		t.Errorf("newest = %s, want SCCRP", snap[0].MsgType)
	}
	if snap[1].MsgType != "SCCRQ" {
		t.Errorf("oldest = %s, want SCCRQ", snap[1].MsgType)
	}
}

func TestCaptureRingOverflow(t *testing.T) {
	r := newCaptureRing()
	for i := range captureRingCapacity + 10 {
		r.appendInbound(uint16(i), 1, MsgHello, testPeer, 50, 0)
	}
	snap := r.Snapshot(0, 0, "")
	if len(snap) != captureRingCapacity {
		t.Fatalf("count = %d, want %d", len(snap), captureRingCapacity)
	}
	if snap[0].TunnelID != captureRingCapacity+10-1 {
		t.Errorf("newest tunnelID = %d, want %d", snap[0].TunnelID, captureRingCapacity+10-1)
	}
}

func TestCaptureRingFilterTunnelID(t *testing.T) {
	r := newCaptureRing()
	r.appendInbound(10, 1, MsgSCCRQ, testPeer, 100, 0)
	r.appendInbound(20, 2, MsgICRQ, testPeer, 80, 0)
	r.appendInbound(10, 3, MsgHello, testPeer, 50, 0)

	snap := r.Snapshot(0, 10, "")
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	for _, e := range snap {
		if e.TunnelID != 10 {
			t.Errorf("tunnelID = %d, want 10", e.TunnelID)
		}
	}
}

func TestCaptureRingFilterPeer(t *testing.T) {
	peer2 := netip.MustParseAddrPort("198.51.100.1:1701")
	r := newCaptureRing()
	r.appendInbound(10, 1, MsgSCCRQ, testPeer, 100, 0)
	r.appendInbound(20, 2, MsgSCCRQ, peer2, 100, 0)
	r.appendInbound(10, 3, MsgHello, testPeer, 50, 0)

	snap := r.Snapshot(0, 0, "192.0.2.1")
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	for _, e := range snap {
		if e.PeerAddr != testPeer.String() {
			t.Errorf("peer = %s, want %s", e.PeerAddr, testPeer.String())
		}
	}
}

func TestCaptureRingLimit(t *testing.T) {
	r := newCaptureRing()
	for range 10 {
		r.appendInbound(1, 1, MsgHello, testPeer, 50, 0)
	}
	snap := r.Snapshot(3, 0, "")
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
}

func TestCaptureRingEmpty(t *testing.T) {
	r := newCaptureRing()
	snap := r.Snapshot(0, 0, "")
	if snap == nil {
		t.Fatal("snapshot should be non-nil empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("count = %d, want 0", len(snap))
	}
}

func TestCaptureRingDirection(t *testing.T) {
	r := newCaptureRing()
	r.appendInbound(1, 1, MsgSCCRQ, testPeer, 100, 0)
	r.appendOutbound(1, 1, MsgSCCRP, testPeer, 120)

	snap := r.Snapshot(0, 0, "")
	if snap[0].Direction != "out" {
		t.Errorf("outbound direction = %s, want out", snap[0].Direction)
	}
	if snap[1].Direction != "in" {
		t.Errorf("inbound direction = %s, want in", snap[1].Direction)
	}
}

func TestCaptureRingResultCode(t *testing.T) {
	r := newCaptureRing()
	r.appendInbound(1, 1, MsgStopCCN, testPeer, 100, 42)
	r.appendInbound(2, 1, MsgHello, testPeer, 50, 0)

	snap := r.Snapshot(0, 0, "")
	if snap[1].ResultCode != 42 {
		t.Errorf("resultCode = %d, want 42", snap[1].ResultCode)
	}
	if snap[0].ResultCode != 0 {
		t.Errorf("resultCode = %d, want 0", snap[0].ResultCode)
	}
}
