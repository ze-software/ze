// Design: docs/architecture/rib/unified-locrib.md -- authoritative selected views.
// Related: route_view.go -- process-bound session authority.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func routeViewOwner() *process.Process {
	return process.NewProcess(plugin.PluginConfig{Name: "selected-owner", Run: "rib", Internal: true})
}

func routeViewOpenTest(t *testing.T, authority *routeViewAuthority, rib *locrib.RIB) (*process.Process, rpc.RouteViewOpenOutput) {
	t.Helper()
	owner := routeViewOwner()
	opened, err := authority.open(context.Background(), owner, rib, rpc.RouteViewOpenInput{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := authority.cleanup(owner); err != nil {
			t.Error(err)
		}
	})
	return owner, opened
}

func routeViewReadTest(t *testing.T, authority *routeViewAuthority, owner *process.Process, session string, cursor uint64, limit uint32) rpc.RouteViewReadOutput {
	t.Helper()
	page, err := authority.read(context.Background(), owner, rpc.RouteViewReadInput{Session: session, Cursor: cursor, Limit: limit})
	if err != nil {
		t.Fatal(err)
	}
	return page
}

// A scan reconciles a withdrawal, an update and an insert before completing.
// The scan seam runs mutations after copying the old values, deterministically.
func TestRouteViewSnapshotWithdrawOrdering(t *testing.T) {
	rib := locrib.NewRIB()
	removed := netip.MustParsePrefix("198.51.100.0/24")
	updated := netip.MustParsePrefix("203.0.113.0/24")
	inserted := netip.MustParsePrefix("192.0.2.0/24")
	path := locrib.Path{Source: redistevents.RegisterProtocol("static"), NextHop: netip.MustParseAddr("10.0.0.1"), Metric: 1}
	rib.Insert(family.IPv4Unicast, removed, path)
	rib.Insert(family.IPv4Unicast, updated, path)
	var authority routeViewAuthority
	authority.scan = func(ctx context.Context, rib *locrib.RIB, visit routeViewVisitor) error {
		if err := routeViewScan(ctx, rib, visit); err != nil {
			return err
		}
		rib.Remove(family.IPv4Unicast, removed, path.Source, path.Instance)
		path.Metric = 9
		rib.Insert(family.IPv4Unicast, updated, path)
		rib.Insert(family.IPv4Unicast, inserted, path)
		return nil
	}
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, opened.Cursor, 4096)
	if page.Phase != rpc.RouteViewPhaseSnapshot || !page.Complete {
		t.Fatalf("incomplete snapshot: %+v", page)
	}
	seen := make(map[netip.Prefix]uint32)
	for i := range page.Entries {
		seen[page.Entries[i].Prefix] = page.Entries[i].Path.Metric
	}
	if _, exists := seen[removed]; exists {
		t.Fatalf("old scan resurrected withdrawn prefix: %+v", page.Entries)
	}
	if seen[updated] != 9 || seen[inserted] != 9 || len(seen) != 2 {
		t.Fatalf("snapshot differs from authority: %v", seen)
	}
	// A stale add followed by withdrawal must collapse to the current tombstone.
	rib.Insert(family.IPv4Unicast, removed, path)
	rib.Remove(family.IPv4Unicast, removed, path.Source, path.Instance)
	delta := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 4096)
	if len(delta.Entries) != 1 || delta.Entries[0].Action != rpc.RouteViewActionWithdraw {
		t.Fatalf("buffered add survived newer withdrawal: %+v", delta)
	}
}

// Selected paths retain every forwarding field and own all mutable slices.
// Mutating the source slices after snapshot must not change its returned page.
func TestRouteViewFieldsRoundTrip(t *testing.T) {
	rib := locrib.NewRIB()
	prefix := netip.MustParsePrefix("2001:db8::/48")
	path := locrib.Path{
		Source: redistevents.RegisterProtocol("static"), Instance: 7, NextHop: netip.MustParseAddr("2001:db8:1::1"),
		Interface: "port1", OnLink: true, Weight: 17, Metric: 34, DistanceOverride: 17, HasDistanceOverride: true,
		Labels: []uint32{100, 200}, RouteType: routetype.Unicast, SRv6SID: netip.MustParseAddr("2001:db8:2::1"),
		IsEBGP: true, IsBGP: true, AIGP: 0, AIGPPresent: true, MetricRecursive: true,
		Tag: 0, HasTag: true, Level: 2, OSPFRouteType: 3,
		ECMP:          []locrib.NextHop{{Addr: netip.MustParseAddr("2001:db8:3::1"), Interface: "port2", OnLink: true, Weight: 21}},
		BackupNextHop: netip.MustParseAddr("2001:db8:4::1"), BackupInterface: "port3", BackupOnLink: true,
		BackupRepairLabels: []uint32{300, 400},
	}
	rib.Insert(family.IPv6Unicast, prefix, path)
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	path.Labels[0] = 999
	path.BackupRepairLabels[0] = 999
	path.ECMP[0].Weight = 99
	page := routeViewReadTest(t, &authority, owner, opened.Session, opened.Cursor, 4096)
	if len(page.Entries) != 1 {
		t.Fatalf("entries = %+v", page.Entries)
	}
	want := rpc.RouteViewPath{
		Protocol: "static", Instance: 7, Distance: 17, Metric: 34, NextHop: path.NextHop,
		Interface: "port1", OnLink: true, Weight: 17, Labels: []uint32{100, 200}, RouteType: uint8(routetype.Unicast),
		SRv6SID: path.SRv6SID, IsEBGP: true, IsBGP: true, AIGPPresent: true, MetricRecursive: true,
		HasTag: true, Level: 2, OSPFRouteType: 3,
		ECMP:          []rpc.RouteViewNextHop{{NextHop: netip.MustParseAddr("2001:db8:3::1"), Interface: "port2", OnLink: true, Weight: 21, Labels: []uint32{100, 200}, SRv6SID: path.SRv6SID}},
		BackupNextHop: path.BackupNextHop, BackupInterface: "port3", BackupOnLink: true,
		BackupRepairLabels: []uint32{300, 400},
	}
	if !reflect.DeepEqual(page.Entries[0].Path, want) {
		t.Fatalf("selected metadata changed:\ngot  %+v\nwant %+v", page.Entries[0].Path, want)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var decoded rpc.RouteViewReadOutput
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, page) {
		t.Fatalf("wire round trip changed page: %+v", decoded)
	}
}

// Invalid tokens, generations and cursors cannot advance the healthy owner.
// A valid read after every refusal still obtains its original pending page.
func TestRouteViewRejectsForeignSession(t *testing.T) {
	rib := locrib.NewRIB()
	rib.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: redistevents.RegisterProtocol("static")})
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	foreign := process.NewProcess(plugin.PluginConfig{Name: "unrelated", Internal: true})
	if _, err := authority.open(context.Background(), foreign, rib, rpc.RouteViewOpenInput{}); err == nil {
		t.Fatal("foreign registration opened selected view")
	}
	for _, test := range []struct {
		name  string
		proc  *process.Process
		input rpc.RouteViewReadInput
	}{
		{"token", owner, rpc.RouteViewReadInput{Session: "foreign", Cursor: 0, Limit: 1}},
		{"generation", routeViewOwner(), rpc.RouteViewReadInput{Session: opened.Session, Cursor: 0, Limit: 1}},
		{"future", owner, rpc.RouteViewReadInput{Session: opened.Session, Cursor: 99, Limit: 1}},
		{"zero-limit", owner, rpc.RouteViewReadInput{Session: opened.Session, Limit: 0}},
		{"large-limit", owner, rpc.RouteViewReadInput{Session: opened.Session, Limit: 4097}},
		{"large-wait", owner, rpc.RouteViewReadInput{Session: opened.Session, Limit: 1, WaitMillis: 1001}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := authority.read(context.Background(), test.proc, test.input); err == nil {
				t.Fatal("invalid read succeeded")
			}
		})
	}
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	if len(page.Entries) != 1 || !page.Complete {
		t.Fatalf("refusals consumed healthy page: %+v", page)
	}
	if _, err := authority.read(context.Background(), owner, rpc.RouteViewReadInput{Session: opened.Session, Cursor: 0, Limit: 1}); err == nil {
		t.Fatal("stale cursor accepted")
	}
	if err := authority.cleanup(routeViewOwner()); err != nil {
		t.Fatal(err)
	}
	control := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
	if control.Phase != rpc.RouteViewPhaseDelta {
		t.Fatalf("old-generation cleanup removed live session: %+v", control)
	}
}

// Overflow fences the session explicitly and allows a clean current snapshot.
// The healthy control survives a 4097-prefix burst after the first snapshot.
func TestRouteViewOverflowReset(t *testing.T) {
	rib := locrib.NewRIB()
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 4096)
	for i := range rpc.RouteViewRecordsMax + 1 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24)
		rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: redistevents.RegisterProtocol("static")})
	}
	reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 4096)
	if reset.Phase != rpc.RouteViewPhaseReset || reset.Complete || len(reset.Entries) != 0 {
		t.Fatalf("overflow silently lost changes: %+v", reset)
	}
	if err := authority.cleanup(owner); err != nil {
		t.Fatal(err)
	}
	owner, opened = routeViewOpenTest(t, &authority, rib)
	count := 0
	for {
		page = routeViewReadTest(t, &authority, owner, opened.Session, opened.Cursor, 4096)
		count += len(page.Entries)
		opened.Cursor = page.Cursor
		if page.Complete {
			break
		}
	}
	if count != rpc.RouteViewRecordsMax+1 {
		t.Fatalf("reset recovery snapshot has %d routes", count)
	}
}

// A waiting read obeys its caller deadline; cleanup also cancels it promptly.
// State channels provide ordering, with the test deadline only as a deadlock bound.
func TestRouteViewReadCancellation(t *testing.T) {
	var authority routeViewAuthority
	waiting := make(chan struct{})
	authority.beforeWait = func() { close(waiting) }
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := authority.read(ctx, owner, rpc.RouteViewReadInput{Session: opened.Session, Cursor: page.Cursor, Limit: 1, WaitMillis: 1000}); err == nil {
		t.Fatal("canceled read succeeded")
	}
	done := make(chan error, 1)
	go routeViewReadUntilStopped(&authority, owner, opened.Session, page.Cursor, done)
	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not enter its bounded wait")
	}
	if err := authority.cleanup(owner); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cleanup left a successful stale reader")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup failed to join waiting read")
	}
	_, replacement := routeViewOpenTest(t, &authority, locrib.NewRIB())
	if replacement.Session == opened.Session || replacement.Epoch == opened.Epoch {
		t.Fatal("replacement reused session authority")
	}
}

// A withdrawal after the first page is reconciled before snapshot completion.
// Applying only snapshot pages must leave the admitted surviving control.
func TestRouteViewSnapshotPagedWithdrawal(t *testing.T) {
	rib := locrib.NewRIB()
	first := netip.MustParsePrefix("192.0.2.0/24")
	second := netip.MustParsePrefix("198.51.100.0/24")
	source := redistevents.RegisterProtocol("static")
	rib.Insert(family.IPv4Unicast, first, locrib.Path{Source: source})
	rib.Insert(family.IPv4Unicast, second, locrib.Path{Source: source})
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	if page.Complete {
		t.Fatal("one record completed a two-prefix snapshot")
	}
	selected := map[netip.Prefix]bool{page.Entries[0].Prefix: true}
	rib.Remove(family.IPv4Unicast, first, source, 0)
	for range 8 {
		page = routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
		if page.Phase != rpc.RouteViewPhaseSnapshot {
			t.Fatalf("reconciliation escaped initial snapshot: %+v", page)
		}
		for i := range page.Entries {
			entry := &page.Entries[i]
			if entry.Action == rpc.RouteViewActionWithdraw {
				delete(selected, entry.Prefix)
				continue
			}
			selected[entry.Prefix] = true
		}
		if page.Complete {
			if selected[first] {
				t.Fatal("completed snapshot resurrected a prefix withdrawn during paging")
			}
			if !selected[second] {
				t.Fatal("completed snapshot lost its admitted control")
			}
			return
		}
	}
	t.Fatal("snapshot never completed after a single withdrawal")
}

// A drained dirty key cannot disappear when the request is canceled.
// The controlled handoff cancels just after capture drain, before inspection.
func TestRouteViewCanceledCatchupResets(t *testing.T) {
	rib := locrib.NewRIB()
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	rib.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: redistevents.RegisterProtocol("static")})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		session.beforeReconcile = cancel
		_, err := session.synchronize(ctx)
		return err
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("catchup did not observe cancellation: %v", err)
	}
	reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
	if reset.Phase != rpc.RouteViewPhaseReset {
		t.Fatalf("canceled catchup silently lost captured key: %+v", reset)
	}
}

// A failed withdrawal fence prevents every successful reset response.
// Retrying scoped cleanup after the producer recovers releases the old owner.
func TestRouteViewFailedResetFenceStaysFailed(t *testing.T) {
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	failure := errors.New("controlled withdrawal failure")
	defer func() { failure = nil }()
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		if err := session.setFence(func() error { return failure }); err != nil {
			return err
		}
		session.overflow.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		_, err := authority.read(context.Background(), owner, rpc.RouteViewReadInput{Session: opened.Session, Cursor: page.Cursor, Limit: 1})
		if !errors.Is(err, failure) {
			t.Fatalf("failed withdrawal became successful reset: %v", err)
		}
	}
	failure = nil
	if _, err := authority.close(owner, opened.Session); err != nil {
		t.Fatal(err)
	}
}

// Replacement retries its predecessor's scoped cleanup under serialization.
// A transient failed emit must not permanently block a healthy new owner.
func TestRouteViewReplacementRetriesCleanup(t *testing.T) {
	rib := locrib.NewRIB()
	rib.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: redistevents.RegisterProtocol("static")})
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	failure := errors.New("controlled one-shot withdrawal failure")
	attempts := 0
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		return session.setFence(func() error {
			attempts++
			if attempts == 1 {
				return failure
			}
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}
	if err := authority.cleanup(owner); !errors.Is(err, failure) {
		t.Fatalf("cleanup error was lost: %v", err)
	}
	replacement, current := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, replacement, current.Session, 0, 4096)
	if !page.Complete || len(page.Entries) != 1 || attempts != 2 {
		t.Fatalf("replacement did not recover its healthy control: page=%+v attempts=%d", page, attempts)
	}
	if err := authority.cleanup(owner); err != nil {
		t.Fatal(err)
	}
	control := routeViewReadTest(t, &authority, replacement, current.Session, page.Cursor, 1)
	if control.Phase != rpc.RouteViewPhaseDelta {
		t.Fatal("old cleanup removed recovered generation")
	}
}

func routeViewReadUntilStopped(authority *routeViewAuthority, owner *process.Process, session string, cursor uint64, done chan<- error) {
	_, err := authority.read(context.Background(), owner, rpc.RouteViewReadInput{Session: session, Cursor: cursor, Limit: 1, WaitMillis: 1000})
	done <- err
}

// Byte-sized pages preserve every prefix when the record limit exceeds a frame.
// Legal label stacks make the frame bound dominate the 4096-record bound.
func TestRouteViewPageFrameBudget(t *testing.T) {
	rib := locrib.NewRIB()
	for i := range 4096 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24)
		labels := make([]uint32, 1024)
		for j := range labels {
			labels[j] = 100000
		}
		rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: redistevents.RegisterProtocol("static"), Interface: "port1", Labels: labels})
	}
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	seen := make(map[netip.Prefix]bool)
	pages := 0
	for {
		page := routeViewReadTest(t, &authority, owner, opened.Session, opened.Cursor, 4096)
		encoded, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) > rpc.RouteViewResultBudget() {
			t.Fatalf("page has %d bytes, budget %d", len(encoded), rpc.RouteViewResultBudget())
		}
		for i := range page.Entries {
			prefix := page.Entries[i].Prefix
			if seen[prefix] {
				t.Fatalf("duplicate prefix %s", prefix)
			}
			seen[prefix] = true
		}
		pages++
		opened.Cursor = page.Cursor
		if page.Complete {
			break
		}
	}
	if len(seen) != 4096 || pages < 2 {
		t.Fatalf("paged %d prefixes in %d pages", len(seen), pages)
	}
}
