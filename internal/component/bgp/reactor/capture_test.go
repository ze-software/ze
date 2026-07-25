package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/test/sim"
)

var testStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestBGPCaptureRingAppend(t *testing.T) {
	c := sim.NewFakeClock(testStart)
	r := NewBGPCaptureRing(c)
	peer := netip.MustParseAddr("192.0.2.1")

	r.Append(false, peer, msgtype.TypeOPEN, 100, 0, 0)
	r.Append(true, peer, msgtype.TypeOPEN, 120, 0, 0)

	snap := r.Snapshot(0, netip.Addr{})
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	if snap[0].MsgType != "OPEN" {
		t.Errorf("newest = %s, want OPEN", snap[0].MsgType)
	}
	if snap[0].Direction != "out" {
		t.Errorf("newest direction = %s, want out", snap[0].Direction)
	}
	if snap[1].Direction != "in" {
		t.Errorf("oldest direction = %s, want in", snap[1].Direction)
	}
}

func TestBGPCaptureRingOverflow(t *testing.T) {
	c := sim.NewFakeClock(testStart)
	r := NewBGPCaptureRing(c)
	peer := netip.MustParseAddr("192.0.2.1")

	for range bgpCaptureRingCapacity + 10 {
		r.Append(false, peer, msgtype.TypeKEEPALIVE, 19, 0, 0)
	}
	snap := r.Snapshot(0, netip.Addr{})
	if len(snap) != bgpCaptureRingCapacity {
		t.Fatalf("count = %d, want %d", len(snap), bgpCaptureRingCapacity)
	}
}

func TestBGPCaptureRingFilterPeer(t *testing.T) {
	c := sim.NewFakeClock(testStart)
	r := NewBGPCaptureRing(c)
	peer1 := netip.MustParseAddr("192.0.2.1")
	peer2 := netip.MustParseAddr("198.51.100.1")

	r.Append(false, peer1, msgtype.TypeUPDATE, 200, 0, 0)
	r.Append(false, peer2, msgtype.TypeUPDATE, 300, 0, 0)
	r.Append(false, peer1, msgtype.TypeKEEPALIVE, 19, 0, 0)

	snap := r.Snapshot(0, peer1)
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	for _, e := range snap {
		if e.PeerAddr != "192.0.2.1" {
			t.Errorf("peer = %s, want 192.0.2.1", e.PeerAddr)
		}
	}
}

func TestBGPCaptureRingLimit(t *testing.T) {
	c := sim.NewFakeClock(testStart)
	r := NewBGPCaptureRing(c)
	peer := netip.MustParseAddr("192.0.2.1")

	for range 10 {
		r.Append(false, peer, msgtype.TypeKEEPALIVE, 19, 0, 0)
	}
	snap := r.Snapshot(3, netip.Addr{})
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
}

func TestBGPCaptureRingEmpty(t *testing.T) {
	c := sim.NewFakeClock(testStart)
	r := NewBGPCaptureRing(c)
	snap := r.Snapshot(0, netip.Addr{})
	if snap == nil {
		t.Fatal("snapshot should be non-nil empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("count = %d, want 0", len(snap))
	}
}

func TestBGPCaptureRingDirection(t *testing.T) {
	c := sim.NewFakeClock(testStart)
	r := NewBGPCaptureRing(c)
	peer := netip.MustParseAddr("192.0.2.1")

	r.Append(false, peer, msgtype.TypeOPEN, 100, 0, 0)
	r.Append(true, peer, msgtype.TypeOPEN, 100, 0, 0)

	snap := r.Snapshot(0, netip.Addr{})
	if snap[0].Direction != "out" {
		t.Errorf("direction = %s, want out", snap[0].Direction)
	}
	if snap[1].Direction != "in" {
		t.Errorf("direction = %s, want in", snap[1].Direction)
	}
}

func TestBGPCaptureRingErrorCode(t *testing.T) {
	c := sim.NewFakeClock(testStart)
	r := NewBGPCaptureRing(c)
	peer := netip.MustParseAddr("192.0.2.1")

	r.Append(false, peer, msgtype.TypeNOTIFICATION, 23, 6, 3)
	r.Append(false, peer, msgtype.TypeKEEPALIVE, 19, 0, 0)

	snap := r.Snapshot(0, netip.Addr{})
	if snap[1].ErrorCode != 6 {
		t.Errorf("errorCode = %d, want 6", snap[1].ErrorCode)
	}
	if snap[1].ErrorSub != 3 {
		t.Errorf("errorSub = %d, want 3", snap[1].ErrorSub)
	}
	if snap[0].ErrorCode != 0 {
		t.Errorf("errorCode = %d, want 0", snap[0].ErrorCode)
	}
}

func TestBGPCaptureRingTimestamp(t *testing.T) {
	c := sim.NewFakeClock(testStart)
	r := NewBGPCaptureRing(c)
	peer := netip.MustParseAddr("192.0.2.1")

	r.Append(false, peer, msgtype.TypeKEEPALIVE, 19, 0, 0)

	snap := r.Snapshot(0, netip.Addr{})
	if snap[0].Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
}
