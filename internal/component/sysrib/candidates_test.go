package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type candidateChain func(r ribpolicy.Route) ribpolicy.Verdict

func (f candidateChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict { return f(r) } //nolint:gocritic // hugeParam: the Chain signature passes Route by value

// candidateRequests records the route-view-candidates requests that reached
// the daemon authority. Safe for concurrent use: the peer goroutine records,
// the test reads.
type candidateRequests struct {
	mu     sync.Mutex
	inputs []rpc.RouteViewCandidatesInput
}

func (r *candidateRequests) add(input rpc.RouteViewCandidatesInput) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inputs = append(r.inputs, input)
}

func (r *candidateRequests) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.inputs)
}

// candidatesThroughAuthority starts a routing-view client whose
// route-view-candidates requests a real routeViewAuthority over rib answers.
// That is the path `show rib candidates` takes in both placements: the
// command handler calls routeViewClient.Candidates, which reads the arguments
// (candidatesPrefix), and its worker pages the RPC that the daemon answers
// with readCandidates, which renders each prefix with candidateEntryOf. The
// client's lease comes from the runtime feed, so its session token and epoch
// differ from the authority's own; the bridge translates those two identities
// and nothing else, so the selector, snapshot, cursor and rows pass unchanged.
// The caller MUST close the returned client.
func candidatesThroughAuthority(t *testing.T, ctx context.Context, rib *locrib.RIB) (*routeViewClient, *candidateRequests) {
	t.Helper()
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	requests := &candidateRequests{}
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		if request.Method != rpc.MethodRouteViewCandidates {
			return nil, false, nil
		}
		var input rpc.RouteViewCandidatesInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, true, err
		}
		requests.add(input)
		clientSession := input.Session
		input.Session = opened.Session
		page, err := authority.readCandidates(ctx, owner, input)
		if err != nil {
			return nil, true, err
		}
		page.Session, page.Epoch = clientSession, "daemon"
		return page, true, nil
	})
	feed.pages <- runtimeSnapshot(1, true)
	client := openRuntimeFeedClient(t, ctx, feed)
	if err := client.Start(ctx); err != nil {
		closeRuntimeFeedClient(t, ctx, client)
		t.Fatal(err)
	}
	return client, requests
}

// candidateAnswer decodes the rows routeViewClient.Candidates answered.
func candidateAnswer(t *testing.T, answer any) ([]candidateEntry, []byte) {
	t.Helper()
	rows, ok := answer.([]json.RawMessage)
	if !ok {
		t.Fatalf("answer is %T, want []json.RawMessage", answer)
	}
	entries := make([]candidateEntry, len(rows))
	var raw []byte
	for i := range rows {
		if err := json.Unmarshal(rows[i], &entries[i]); err != nil {
			t.Fatalf("row %d %s: %v", i, rows[i], err)
		}
		raw = append(raw, rows[i]...)
	}
	return entries, raw
}

// TestShowRIBCandidatesListsHiddenPaths is the `show rib candidates` contract,
// through routeViewClient.Candidates and the daemon's readCandidates: a prefix
// the central chain hides still appears, with every source's path marked
// hidden by the central chain and named by its attachment, a selected prefix
// shows its selected path, a tagged path shows its tag (zero included), the
// answer is sorted by prefix, the `prefix` selector narrows the answer to one
// prefix, and the JSON keys are kebab-case.
func TestShowRIBCandidatesListsHiddenPaths(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	static := redistevents.RegisterProtocol("static")
	bogon := netip.MustParsePrefix("192.0.2.0/24")
	kept := netip.MustParsePrefix("10.70.0.0/24")
	previous := ribpolicy.Current()
	snap, err := ribpolicy.NewSnapshot(nil, candidateChain(func(r ribpolicy.Route) ribpolicy.Verdict {
		if r.Prefix == bogon {
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}))
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	ribpolicy.Set(snap)
	t.Cleanup(func() { ribpolicy.Set(previous) })

	loc := locrib.NewRIB()
	loc.Insert(family.IPv4Unicast, bogon, locrib.Path{Source: static, NextHop: netip.MustParseAddr("192.0.2.1")})
	loc.Insert(family.IPv4Unicast, kept, locrib.Path{Source: static, NextHop: netip.MustParseAddr("192.0.2.1"), HasTag: true})

	client, _ := candidatesThroughAuthority(t, ctx, loc)
	defer closeRuntimeFeedClient(t, ctx, client)

	data, err := client.Candidates(ctx, nil)
	if err != nil {
		t.Fatalf("show rib candidates: %v", err)
	}
	entries, raw := candidateAnswer(t, data)
	if len(entries) != 2 {
		t.Fatalf("answer lists %d prefixes, want 2 (the hidden one included): %+v", len(entries), entries)
	}
	hidden := entries[1].Paths
	if entries[1].Prefix != bogon || len(hidden) != 1 {
		t.Fatalf("second entry %+v, want %s with one path", entries[1], bogon)
	}
	if !hidden[0].Hidden || hidden[0].Selected {
		t.Fatalf("bogon path %+v: want hidden and not selected", hidden[0])
	}
	if hidden[0].HiddenBy != "central" || hidden[0].Chain != "rib import" {
		t.Fatalf("bogon path hidden by %q chain %q, want central / rib import", hidden[0].HiddenBy, hidden[0].Chain)
	}
	if hidden[0].Source != "static" {
		t.Fatalf("bogon path source %q, want static", hidden[0].Source)
	}
	sel := entries[0].Paths
	if entries[0].Prefix != kept || len(sel) != 1 || !sel[0].Selected || sel[0].Hidden {
		t.Fatalf("first entry %+v, want %s with its one path selected", entries[0], kept)
	}
	if sel[0].Tag == nil || *sel[0].Tag != 0 {
		t.Fatalf("tag %v, want a present tag 0: zero is a real tag", sel[0].Tag)
	}
	if sel[0].HiddenBy != "" || sel[0].Chain != "" {
		t.Fatalf("selected path names hiding chain %q/%q", sel[0].HiddenBy, sel[0].Chain)
	}

	for _, key := range []string{`"hidden-by":"central"`, `"chain":"rib import"`, `"next-hop":"192.0.2.1"`, `"selected":true`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("JSON %s lacks %s", raw, key)
		}
	}

	one, err := client.Candidates(ctx, []string{"prefix", "192.0.2.0/24"})
	if err != nil {
		t.Fatalf("show rib candidates prefix: %v", err)
	}
	got, _ := candidateAnswer(t, one)
	if len(got) != 1 || got[0].Prefix != bogon {
		t.Fatalf("prefix selector answered %+v, want only %s", got, bogon)
	}
}

// TestShowRIBCandidatesRefusals proves the grammar refuses what it cannot
// answer, through routeViewClient.Candidates, and that a refused argument
// never reaches the daemon. A valid selector afterwards does reach it, so the
// zero request count is a refusal and not a broken bridge.
func TestShowRIBCandidatesRefusals(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	client, requests := candidatesThroughAuthority(t, ctx, locrib.NewRIB())
	defer closeRuntimeFeedClient(t, ctx, client)
	// Each refusal also names its own reason, so a case refused by the wrong
	// check (a bare value caught only by the arity check) is red.
	cases := map[string]struct {
		args   []string
		reason string
	}{
		"bare value":       {[]string{"10.0.0.0/24"}, "unknown argument 10.0.0.0/24"},
		"missing value":    {[]string{"prefix"}, "takes exactly one prefix"},
		"not a prefix":     {[]string{"prefix", "10.0.0.1"}, "10.0.0.1 is not a prefix"},
		"trailing garbage": {[]string{"prefix", "10.0.0.0/24", "extra"}, "takes exactly one prefix"},
	}
	for name, tc := range cases {
		_, err := client.Candidates(ctx, tc.args)
		if err == nil {
			t.Errorf("%s %v: accepted", name, tc.args)
			continue
		}
		if !strings.Contains(err.Error(), "show rib candidates prefix 10.0.0.0/24") {
			t.Errorf("%s: error %q does not show the correct form", name, err)
		}
		if !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s: error %q does not give the reason %q", name, err, tc.reason)
		}
	}
	if n := requests.count(); n != 0 {
		t.Fatalf("%d refused arguments reached the daemon", n)
	}
	if _, err := client.Candidates(ctx, []string{"prefix", "10.0.0.0/24"}); err != nil {
		t.Fatalf("valid selector refused: %v", err)
	}
	if n := requests.count(); n != 1 {
		t.Fatalf("valid selector sent %d candidate requests, want 1", n)
	}
}

// TestShowRIBCandidatesNoLocRIBRefuses proves that a daemon with no Loc-RIB
// says so rather than answer an empty candidate list: the routing view a
// system RIB must open before `show rib candidates` has a handler is refused,
// and no lease is left behind for a read to answer from.
func TestShowRIBCandidatesNoLocRIBRefuses(t *testing.T) {
	var authority routeViewAuthority
	owner := routeViewOwner()
	_, err := authority.open(context.Background(), owner, nil, rpc.RouteViewOpenInput{})
	if err == nil {
		if cleanupErr := authority.cleanup(owner); cleanupErr != nil {
			t.Error(cleanupErr)
		}
		t.Fatal("no Loc-RIB opened a routing view")
	}
	requireRouteViewRefusal(t, err, "The daemon's selected routing table is unavailable", "Call route-view-open on the daemon's authenticated plugin connection")
	if refusal, _ := errors.AsType[*opererr.Error](err); refusal.Why() != "A complete selected view is required before forwarding" {
		t.Fatalf("refusal why = %q, want the nil Loc-RIB consequence", refusal.Why())
	}
	authority.mu.Lock()
	active := authority.active
	authority.mu.Unlock()
	if active != nil {
		t.Fatal("a refused open left a routing-view lease")
	}
}
