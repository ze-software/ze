// Design: docs/architecture/rib/unified-locrib.md -- authoritative selected views.
// Related: route_view.go -- generation worker and deterministic clock seam.

package sysrib

import (
	"context"
	"math"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func routeViewMaintainTest(t *testing.T, authority *routeViewAuthority, at time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	select {
	case authority.maintenance <- routeViewMaintenance{at: at, done: done}:
	case <-ctx.Done():
		t.Fatal("generation worker did not accept its controlled tick")
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("generation worker did not complete its controlled tick")
	}
}

// An abandoned live snapshot releases owned values at its lifetime bound.
// A controlled worker tick advances time; a fresh owner recovers the control.
func TestRouteViewIdleSnapshotExpiry(t *testing.T) {
	rib := locrib.NewRIB()
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: redistevents.RegisterProtocol("static"), Labels: []uint32{100}})
	authority := routeViewAuthority{maintenance: make(chan routeViewMaintenance, 1)}
	owner, opened := routeViewOpenTest(t, &authority, rib)
	fences := 0
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		return session.setFence(func() error { fences++; return nil })
	}); err != nil {
		t.Fatal(err)
	}
	routeViewMaintainTest(t, &authority, time.Now().Add(routeViewSnapshotLifetime+time.Second))
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		if len(session.snapshot) != 0 {
			t.Error("idle expired snapshot still retains selected paths")
		}
		reset, fenceErr := session.resetStatus()
		if !reset {
			t.Error("idle snapshot never became reset-required")
		}
		return fenceErr
	}); err != nil {
		t.Fatal(err)
	}
	if fences != 1 {
		t.Fatalf("idle expiry fenced %d times", fences)
	}
	if err := authority.cleanup(owner); err != nil {
		t.Fatal(err)
	}
	replacement, current := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, replacement, current.Session, 0, 1)
	if !page.Complete || len(page.Entries) != 1 || page.Entries[0].Prefix != prefix {
		t.Fatalf("expiry lost healthy control: %+v", page)
	}
}

// Pending changes expire without the consumer calling Read again.
// The same generation worker fences output and releases its retained journal.
func TestRouteViewIdleJournalExpiry(t *testing.T) {
	rib := locrib.NewRIB()
	authority := routeViewAuthority{maintenance: make(chan routeViewMaintenance, 1)}
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	fences := 0
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		return session.setFence(func() error { fences++; return nil })
	}); err != nil {
		t.Fatal(err)
	}
	rib.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: redistevents.RegisterProtocol("static")})
	routeViewMaintainTest(t, &authority, time.Now().Add(routeViewJournalLifetime+time.Second))
	if fences != 1 {
		t.Fatalf("idle journal never fenced output: %d", fences)
	}
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		if len(session.journal) != 0 {
			t.Error("expired journal retains owned paths")
		}
		reset, fenceErr := session.resetStatus()
		if !reset {
			t.Error("expired pending changes never required reset")
		}
		return fenceErr
	}); err != nil {
		t.Fatal(err)
	}
	reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
	if reset.Phase != rpc.RouteViewPhaseReset {
		t.Fatalf("idle expiry became an empty successful delta: %+v", reset)
	}
}

// A full capture fences an idle owner without waiting for another RPC.
// The controlled tick runs after a 4097-prefix burst with no live reader.
func TestRouteViewIdleOverflowFences(t *testing.T) {
	rib := locrib.NewRIB()
	authority := routeViewAuthority{maintenance: make(chan routeViewMaintenance, 1)}
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	fences := 0
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		return session.setFence(func() error { fences++; return nil })
	}); err != nil {
		t.Fatal(err)
	}
	source := redistevents.RegisterProtocol("static")
	for i := range rpc.RouteViewRecordsMax + 1 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24)
		rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: source})
	}
	routeViewMaintainTest(t, &authority, time.Now())
	if fences != 1 {
		t.Fatalf("idle overflow did not fence: %d", fences)
	}
	reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
	if reset.Phase != rpc.RouteViewPhaseReset {
		t.Fatalf("overflow was silently accepted: %+v", reset)
	}
}

// A distance reset barrier is observable even when its old cursor was consumed.
// The marker advances once and the next explicit Reset reports that frontier.
func TestRouteViewResetBarrierMarker(t *testing.T) {
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	var marker uint64
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		if err := session.invalidate(session.restartRefusal("Controlled view reset", "A new snapshot is required")); err != nil {
			return err
		}
		var err error
		marker, err = session.resetBarrier(context.Background())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if marker <= page.Cursor {
		t.Fatalf("reset barrier %d does not exceed consumed frontier %d", marker, page.Cursor)
	}
	reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
	if reset.Phase != rpc.RouteViewPhaseReset || reset.Cursor != marker {
		t.Fatalf("reset did not expose its distance barrier: %+v", reset)
	}
}

// Cursor exhaustion refuses the marker without wrapping to a valid zero.
// A separately opened healthy view still completes at its ordinary frontier.
func TestRouteViewResetBarrierWrapRefusal(t *testing.T) {
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		if err := session.invalidate(session.restartRefusal("Controlled view reset", "A new snapshot is required")); err != nil {
			return err
		}
		session.assigned = math.MaxUint64
		_, err := session.resetBarrier(context.Background())
		if err == nil {
			t.Fatal("exhausted reset cursor wrapped successfully")
		}
		if !strings.Contains(err.Error(), "maximum value") {
			t.Fatalf("exhaustion was not distinguished: %v", err)
		}
		if session.assigned != math.MaxUint64 {
			t.Fatal("failed marker changed cursor")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := authority.cleanup(owner); err != nil {
		t.Fatal(err)
	}
	replacement, current := routeViewOpenTest(t, &authority, locrib.NewRIB())
	control := routeViewReadTest(t, &authority, replacement, current.Session, 0, 1)
	if !control.Complete || control.Cursor != 1 {
		t.Fatalf("healthy empty control failed: %+v", control)
	}
}
