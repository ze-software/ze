// Design: docs/guide/route-policy.md -- inspecting stored policy verdicts.
// Related: route_view_candidates.go -- separately pinned candidate scans.

package sysrib

import (
	"context"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func routeViewCandidateEntry(t *testing.T, page rpc.RouteViewCandidatesOutput, index int) candidateEntry {
	t.Helper()
	if index >= len(page.Entries) {
		t.Fatalf("candidate page lacks entry %d: %+v", index, page)
	}
	var entry candidateEntry
	if err := json.Unmarshal(page.Entries[index], &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

// The complete reply budget pages large valid prefixes without truncating rows.
func TestRouteViewCandidateCompleteFrameBudget(t *testing.T) {
	previous := ribpolicy.Current()
	snapshot, err := ribpolicy.NewSnapshot(nil, candidateChain(func(r ribpolicy.Route) ribpolicy.Verdict {
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	}))
	if err != nil {
		t.Fatal(err)
	}
	ribpolicy.Set(snapshot)
	t.Cleanup(func() { ribpolicy.Set(previous) })
	rib := locrib.NewRIB()
	source := redistevents.RegisterProtocol("static")
	device := strings.Repeat("x", 512<<10)
	for index := range 40 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, byte(index), 0}), 24)
		rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: source, Interface: device})
	}
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Limit: 4096})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > rpc.RouteViewResultBudget() {
		t.Fatalf("candidate reply exceeds the complete frame: %d > %d", len(encoded), rpc.RouteViewResultBudget())
	}
	if page.Complete || len(page.Entries) == 0 {
		t.Fatalf("large scan did not produce a useful bounded first page: count %d complete %t", len(page.Entries), page.Complete)
	}
	count := len(page.Entries)
	next, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Snapshot: page.Snapshot, Cursor: page.Cursor, Limit: 4096})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > rpc.RouteViewResultBudget() || !next.Complete || count+len(next.Entries) != 40 {
		t.Fatalf("candidate continuation lost rows or exceeded its frame: first %d next %d bytes %d complete %t", count, len(next.Entries), len(encoded), next.Complete)
	}
}

// Expiration drops retained data and refuses the old cursor while a fresh scan
// continues to answer the healthy owner's current control.
func TestRouteViewCandidateExpiration(t *testing.T) {
	rib := locrib.NewRIB()
	source := redistevents.RegisterProtocol("static")
	for _, value := range []string{"192.0.2.0/24", "198.51.100.0/24"} {
		rib.Insert(family.IPv4Unicast, netip.MustParsePrefix(value), locrib.Path{Source: source})
	}
	var authority routeViewAuthority
	maintenance := make(chan routeViewMaintenance, 1)
	authority.maintenance = maintenance
	owner, opened := routeViewOpenTest(t, &authority, rib)
	routeViewReadTest(t, &authority, owner, opened.Session, 0, 4096)
	page, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	var pin *routeViewCandidateSnapshot
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		pin = session.candidates
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	maintenance <- routeViewMaintenance{at: pin.deadline.Add(time.Nanosecond), done: done}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("candidate lifetime worker did not acknowledge maintenance")
	}
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		if len(pin.entries) != 0 || !pin.expired {
			t.Fatalf("expired scan retained %d prefixes, expired %t", len(pin.entries), pin.expired)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Snapshot: page.Snapshot, Cursor: page.Cursor, Limit: 1}); err == nil {
		t.Fatal("expired snapshot cursor was accepted")
	}
	fresh, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Limit: 4096})
	if err != nil || !fresh.Complete || len(fresh.Entries) != 2 || fresh.Snapshot == page.Snapshot {
		t.Fatalf("fresh control after expiration: %+v, %v", fresh, err)
	}
}

func TestRouteViewHiddenCandidatesStayFresh(t *testing.T) {
	previous := ribpolicy.Current()
	snapshot, err := ribpolicy.NewSnapshot(nil, candidateChain(func(r ribpolicy.Route) ribpolicy.Verdict {
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	}))
	if err != nil {
		t.Fatal(err)
	}
	ribpolicy.Set(snapshot)
	t.Cleanup(func() { ribpolicy.Set(previous) })
	rib := locrib.NewRIB()
	source := redistevents.RegisterProtocol("static")
	first := netip.MustParsePrefix("192.0.2.0/24")
	second := netip.MustParsePrefix("198.51.100.0/24")
	path := locrib.Path{Source: source, Metric: 1, HasTag: true}
	rib.Insert(family.IPv4Unicast, first, path)
	rib.Insert(family.IPv4Unicast, second, path)
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	selected := routeViewReadTest(t, &authority, owner, opened.Session, 0, 4096)
	page, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Complete || page.Snapshot == "" || page.Snapshot == opened.Snapshot {
		t.Fatalf("candidate scan is not separately pinned: %+v", page)
	}
	entry := routeViewCandidateEntry(t, page, 0)
	if len(entry.Paths) != 1 || !entry.Paths[0].Hidden || entry.Paths[0].Selected || entry.Paths[0].Tag == nil || *entry.Paths[0].Tag != 0 || entry.Paths[0].HiddenBy != "central" || entry.Paths[0].Chain != "rib import" {
		t.Fatalf("hidden candidate values lost: %+v", entry)
	}
	revision := rib.Revision()
	path.Metric = 9
	rib.Insert(family.IPv4Unicast, second, path)
	if rib.Revision() != revision {
		t.Fatal("fixture did not produce a hidden-only edit")
	}
	next, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Snapshot: page.Snapshot, Cursor: page.Cursor, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !next.Complete || routeViewCandidateEntry(t, next, 0).Paths[0].Metric != 1 {
		t.Fatalf("mutation changed the pinned scan: %+v", next)
	}
	fresh, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Limit: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Snapshot == page.Snapshot || !fresh.Complete || routeViewCandidateEntry(t, fresh, 1).Paths[0].Metric != 9 {
		t.Fatalf("fresh scan missed hidden-only edit: %+v", fresh)
	}
	unchanged := routeViewReadTest(t, &authority, owner, opened.Session, selected.Cursor, 4096)
	if len(unchanged.Entries) != 0 || unchanged.Cursor != selected.Cursor {
		t.Fatalf("candidate scan changed selected feed: %+v", unchanged)
	}
}

func TestRouteViewCandidatePrefix4097Paths(t *testing.T) {
	rib := locrib.NewRIB()
	source := redistevents.RegisterProtocol("static")
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	for instance := range uint32(4097) {
		rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: source, Instance: instance, Metric: instance, HasTag: true})
	}
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Prefix: prefix, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	entry := routeViewCandidateEntry(t, page, 0)
	if !page.Complete || len(page.Entries) != 1 || len(entry.Paths) != 4097 {
		t.Fatalf("prefix paths were truncated: entries %d, paths %d, complete %t", len(page.Entries), len(entry.Paths), page.Complete)
	}
	selected := 0
	for _, path := range entry.Paths {
		if path.Selected {
			selected++
		}
		if path.Tag == nil || *path.Tag != 0 {
			t.Fatal("present tag zero lost")
		}
	}
	if selected != 1 {
		t.Fatalf("selected flags: %d, want 1", selected)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > rpc.RouteViewResultBudget() {
		t.Fatalf("complete candidate frame exceeded: %d", len(encoded))
	}
}

func TestRouteViewCandidateRefusalsPreservePin(t *testing.T) {
	rib := locrib.NewRIB()
	source := redistevents.RegisterProtocol("static")
	for _, prefix := range []string{"192.0.2.0/24", "198.51.100.0/24"} {
		rib.Insert(family.IPv4Unicast, netip.MustParsePrefix(prefix), locrib.Path{Source: source})
	}
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []rpc.RouteViewCandidatesInput{
		{Session: opened.Session, Limit: 0},
		{Session: opened.Session, Limit: 4097},
		{Session: opened.Session, Snapshot: "foreign", Cursor: page.Cursor, Limit: 1},
		{Session: opened.Session, Snapshot: page.Snapshot, Cursor: page.Cursor + 1, Limit: 1},
		{Session: opened.Session, Snapshot: page.Snapshot, Cursor: 0, Limit: 1},
		{Session: opened.Session, Snapshot: page.Snapshot, Cursor: page.Cursor, Limit: 1, Prefix: netip.MustParsePrefix("203.0.113.0/24")},
		{Session: opened.Session, Limit: 1, AFI: 65535, SAFI: 1},
		{Session: opened.Session, Limit: 1, AFI: 1},
		{Session: opened.Session, Limit: 1, AFI: 1, SAFI: 1, Prefix: netip.MustParsePrefix("2001:db8::/32")},
	} {
		if _, err := authority.readCandidates(context.Background(), owner, input); err == nil {
			t.Fatalf("invalid candidate request accepted: %+v", input)
		}
	}
	if _, err := authority.readCandidates(context.Background(), routeViewOwner(), rpc.RouteViewCandidatesInput{Session: opened.Session, Limit: 1}); err == nil {
		t.Fatal("foreign process read candidates")
	}
	next, err := authority.readCandidates(context.Background(), owner, rpc.RouteViewCandidatesInput{Session: opened.Session, Snapshot: page.Snapshot, Cursor: page.Cursor, Limit: 1})
	if err != nil || !next.Complete || routeViewCandidateEntry(t, next, 0).Prefix != netip.MustParsePrefix("198.51.100.0/24") {
		t.Fatalf("refusals mutated valid pin: %+v, %v", next, err)
	}
}
