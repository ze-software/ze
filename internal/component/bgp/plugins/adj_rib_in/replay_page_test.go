// Tests for the paged replay answer the replay owner relays itself. They drive
// the store through handleCommand and the owner's loop (ribevents.RelayReplay),
// so the property under test is the one a late destination sees: every stored
// route arrives exactly once, in frames the transport accepts, whatever the
// table's size.

package adj_rib_in

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReplayRoutesWholeTableAcrossPages proves that a table larger than one
// page reaches the destination whole.
//
// Method: store 130 routes whose attributes are each 131 KB of hex, about
// 17 MB serialized, so the walk must cross at least two page boundaries. Answer
// the owner's reads from handleCommand, marshaled as the transport carries
// them, and collect what the owner relays.
//
// VALIDATES: every stored route is relayed exactly once, by the caller, in
// ascending store order; each answer and each relay frame stays under
// rpc.MaxMessageSize; the store relays nothing itself.
// PREVENTS: a replay that loses the routes past the first frame, or sends a
// page boundary route twice.
func TestReplayRoutesWholeTableAcrossPages(t *testing.T) {
	r := newTestManager(t)
	r.routeRelayer = func(string, []rpc.StoredRoute) error {
		t.Error("the store must answer the routes, never relay them itself")
		return nil
	}

	attrHex := strings.Repeat("ab", 65535)
	const routeCount = 130
	m := seqmap.New[compactRouteKey, *RawRoute]()
	for i := range routeCount {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i / 256), byte(i % 256), 0}), 24)
		m.Put(routeKeyFromStrings(family.IPv4Unicast, prefix.String(), 0), uint64(i+1), &RawRoute{
			Family: family.IPv4Unicast, AttrHex: attrHex, NHopHex: "0a000001",
			NLRIHex: "18" + hex.EncodeToString(prefix.Addr().AsSlice()[:3]),
		})
	}
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	answers := 0
	dispatch := func(_ context.Context, command string, args ...string) (string, json.RawMessage, error) {
		status, data, err := r.handleCommand(command, args, "")
		if err != nil {
			return status, nil, err
		}
		encoded, err := json.Marshal(data)
		require.NoError(t, err)
		assert.Less(t, len(encoded), rpc.MaxMessageSize, "a page answer must fit one frame")
		answers++
		return status, encoded, nil
	}
	var relayed []string
	relay := func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
		assert.Equal(t, "127.0.0.2", destination)
		frame, err := json.Marshal(rpc.RelayStoredRouteInput{Destination: destination, Routes: routes})
		require.NoError(t, err)
		assert.Less(t, len(frame), rpc.MaxMessageSize, "the owner's relay of one page must fit one frame")
		for i := range routes {
			relayed = append(relayed, routes[i].NLRIHex)
		}
		return nil
	}

	progress, err := ribevents.RelayReplay(t.Context(), dispatch, relay, "127.0.0.2", 0, "")
	require.NoError(t, err)

	require.Greater(t, answers, 2, "the fixture must cross a page boundary, or it proves nothing")
	assert.Equal(t, routeCount, progress.Replayed)
	assert.Equal(t, uint64(routeCount), progress.LastIndex)
	require.Len(t, relayed, routeCount)
	for i := range routeCount {
		prefix := netip.AddrFrom4([4]byte{10, byte(i / 256), byte(i % 256), 0})
		assert.Equal(t, "18"+hex.EncodeToString(prefix.AsSlice()[:3]), relayed[i], "route %d out of store order", i)
	}
}

// TestReplayRoutesRefusesMissingCursor pins the argument contract: the owner
// always names its cursor, so a call without one is a caller defect, not a
// request for the whole table.
//
// VALIDATES: replay-routes with only a target answers statusError.
// PREVENTS: an owner that dropped its cursor silently restarting from 0 on
// every page and looping.
func TestReplayRoutesRefusesMissingCursor(t *testing.T) {
	r := newTestManager(t)
	status, _, err := r.handleCommand(ribevents.ReplayRoutesCommand, []string{"127.0.0.2"}, "")
	require.ErrorIs(t, err, errAdjRibInReplayRoutesArgs)
	assert.Equal(t, statusError, status)
}

// replayTestRoute is the stored route the page tests put at index i: a /24
// under 10/8 whose attributes are attrHex.
func replayTestRoute(i int, attrHex string) (compactRouteKey, *RawRoute) {
	prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i / 256), byte(i % 256), 0}), 24)
	return routeKeyFromStrings(family.IPv4Unicast, prefix.String(), 0), &RawRoute{
		Family: family.IPv4Unicast, AttrHex: attrHex, NHopHex: "0a000001",
		NLRIHex: "18" + hex.EncodeToString(prefix.Addr().AsSlice()[:3]),
	}
}

// TestReplayRoutesInterleavedSourcesAcrossPages proves that a page is the
// lowest store sequences across EVERY source, not one source's run, and that
// the store changing between pages neither loses nor repeats a route.
//
// Method: two sources hold 130 large routes whose sequences interleave (the
// even indexes on one, the odd on the other), so a page cut from one source's
// walk alone would skip the other's lower sequences when the next page resumes
// past it. After the first page the test deletes a route not yet relayed and
// puts a new one past every stored sequence, as live ingest does mid-replay.
//
// VALIDATES: every route stored when its page is read is relayed exactly once,
// in ascending store sequence across both sources; the deleted route is not
// relayed; the route put mid-replay is.
// PREVENTS: a page that sorts or bounds per source and so strands the other
// source's routes behind the cursor.
func TestReplayRoutesInterleavedSourcesAcrossPages(t *testing.T) {
	r := newTestManager(t)
	attrHex := strings.Repeat("ab", 65535)
	sources := [2]*seqmap.Map[compactRouteKey, *RawRoute]{
		seqmap.New[compactRouteKey, *RawRoute](), seqmap.New[compactRouteKey, *RawRoute](),
	}
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = sources[0]
	r.ribIn[netip.MustParseAddr("10.0.0.3")] = sources[1]
	const routeCount = 130
	for i := range routeCount {
		key, route := replayTestRoute(i, attrHex)
		sources[i%2].Put(key, uint64(i+1), route)
	}
	const deleted, added = 127, routeCount // index 127 sits on sources[1]

	answers := 0
	dispatch := func(_ context.Context, command string, args ...string) (string, json.RawMessage, error) {
		status, data, err := r.handleCommand(command, args, "")
		if err != nil {
			return status, nil, err
		}
		answers++
		if answers == 1 {
			key, _ := replayTestRoute(deleted, attrHex)
			require.True(t, sources[deleted%2].Delete(key), "the fixture must delete a stored route")
			key, route := replayTestRoute(added, attrHex)
			sources[added%2].Put(key, uint64(added+1), route)
		}
		encoded, err := json.Marshal(data)
		require.NoError(t, err)
		return status, encoded, nil
	}
	var relayed []string
	relay := func(_ context.Context, _ string, routes []rpc.StoredRoute) error {
		for i := range routes {
			relayed = append(relayed, routes[i].NLRIHex)
		}
		return nil
	}

	progress, err := ribevents.RelayReplay(t.Context(), dispatch, relay, "127.0.0.2", 0, "")
	require.NoError(t, err)

	var want []string
	for i := range added + 1 {
		if i == deleted {
			continue
		}
		_, route := replayTestRoute(i, attrHex)
		want = append(want, route.NLRIHex)
	}
	assert.Equal(t, want, relayed, "every route once, in ascending store sequence across both sources")
	assert.Equal(t, len(want), progress.Replayed)
	assert.Equal(t, uint64(added+1), progress.LastIndex)
	assert.Greater(t, answers, 2, "the fixture must cross a page boundary, or it proves nothing")
}

// TestReplayPageAsymmetricSourcesAcrossPages proves that one page bound holds
// across every source when the sources differ in route size: a source of large
// routes fills its share of a page in a few routes while a source of small
// routes would fit far more, and the page must still not reach past a large
// route it left out.
//
// Method: source A holds every fourth sequence with a 1,500-character
// attribute block, one of them past the page budget on its own; source B holds
// the rest with an 8-byte block. buildReplayPage is read page by page at a
// 6,000-byte budget, resuming from LastIndex. After the first page the test
// deletes one unsent route from each source and puts one past every stored
// sequence on each, as live ingest does mid-replay. The walk runs 32 times,
// because the store visits sources in Go's random map order, and the bound
// must hold in every order.
//
// VALIDATES: every route stored when its page is read arrives exactly once, in
// ascending sequence across both sources; the deleted routes do not, the added
// ones do; each page stays inside the budget unless it is one route larger
// than the budget; the last page says More false.
// PREVENTS: a bound that lets the small source carry the page past a large
// route the large source's walk left out, so the next page resumes beyond it
// and the route is never sent.
func TestReplayPageAsymmetricSourcesAcrossPages(t *testing.T) {
	const (
		routeCount = 200
		budget     = 6000
		oversized  = 100 // On source A, larger than budget on its own.
	)
	large, small := strings.Repeat("ab", 750), strings.Repeat("ab", 8)
	attrFor := func(i int) string {
		if i == oversized {
			return strings.Repeat("ab", budget)
		}
		if i%4 == 0 {
			return large
		}
		return small
	}
	sourceFor := func(i int) int {
		if i%4 == 0 {
			return 0
		}
		return 1
	}
	deleted := [2]int{136, 137}                 // Unsent after page one, on A and on B.
	added := [2]int{routeCount, routeCount + 1} // Past every stored sequence, on A and on B.
	target := netip.MustParseAddr("127.0.0.2")

	for round := range 32 {
		r := newTestManager(t)
		sources := [2]*seqmap.Map[compactRouteKey, *RawRoute]{
			seqmap.New[compactRouteKey, *RawRoute](), seqmap.New[compactRouteKey, *RawRoute](),
		}
		r.ribIn[netip.MustParseAddr("10.0.0.1")] = sources[0]
		r.ribIn[netip.MustParseAddr("10.0.0.3")] = sources[1]
		for i := range routeCount {
			key, route := replayTestRoute(i, attrFor(i))
			sources[sourceFor(i)].Put(key, uint64(i+1), route)
		}

		var relayed []string
		var cursor uint64
		pages := 0
		for {
			page := r.buildReplayPage(target, cursor, unboundedReplay(), budget)
			pages++
			require.Less(t, pages, 1000, "round %d: the paging walk must end", round)
			octets := 0
			for i := range page.Routes {
				octets += relayRouteJSONMax(&page.Routes[i])
				relayed = append(relayed, page.Routes[i].NLRIHex)
			}
			if len(page.Routes) > 1 {
				assert.LessOrEqual(t, octets, budget, "round %d page %d: past the budget", round, pages)
			}
			if pages == 1 {
				for _, i := range deleted {
					key, _ := replayTestRoute(i, attrFor(i))
					require.True(t, sources[sourceFor(i)].Delete(key), "the fixture must delete a stored route")
				}
				for n, i := range added {
					key, route := replayTestRoute(i, attrFor(n*4+1))
					sources[n].Put(key, uint64(i+1), route)
				}
			}
			if !page.More {
				break
			}
			require.NotEmpty(t, page.Routes, "round %d page %d: More with no route", round, pages)
			cursor = page.LastIndex
		}

		var want []string
		for i := range routeCount + len(added) {
			if i == deleted[0] || i == deleted[1] {
				continue
			}
			_, route := replayTestRoute(i, small)
			want = append(want, route.NLRIHex)
		}
		require.Equal(t, want, relayed, "round %d: every route once, in ascending sequence across both sources", round)
		require.Greater(t, pages, 10, "the fixture must cross many page boundaries, or it proves nothing")
	}
}

// benchmarkReplayStore builds a store of routes routes spread over
// sourceCount sources with interleaved sequences, each with a typical 64-byte
// attribute block.
func benchmarkReplayStore(routes, sourceCount int) *AdjRIBInManager {
	r := &AdjRIBInManager{ribIn: make(map[netip.Addr]*seqmap.Map[compactRouteKey, *RawRoute])}
	sources := make([]*seqmap.Map[compactRouteKey, *RawRoute], sourceCount)
	for s := range sources {
		sources[s] = seqmap.New[compactRouteKey, *RawRoute]()
		r.ribIn[netip.AddrFrom4([4]byte{10, 1, byte(s / 256), byte(s % 256)})] = sources[s]
	}
	attrHex := strings.Repeat("ab", 64)
	for i := range routes {
		key, route := replayTestRoute(i, attrHex)
		sources[i%len(sources)].Put(key, uint64(i+1), route)
	}
	return r
}

// BenchmarkBuildReplayRoutes measures the operator replay's walk. Its
// allocations MUST NOT grow with the route count beyond the result slice's
// doublings: one allocation per route is the regression it guards.
func BenchmarkBuildReplayRoutes(b *testing.B) {
	r := benchmarkReplayStore(10000, 4)
	target := netip.MustParseAddr("127.0.0.2")
	b.ReportAllocs()
	for b.Loop() {
		routes, _ := r.buildReplayRoutes(target, 0, unboundedReplay())
		if len(routes) != 10000 {
			b.Fatalf("replayed %d routes, want 10000", len(routes))
		}
	}
}

// BenchmarkBuildReplayPage measures reading a whole table page by page, as the
// replay owner does. A page's work MUST stay bounded by the page, not by the
// table it is cut from.
func BenchmarkBuildReplayPage(b *testing.B) {
	r := benchmarkReplayStore(10000, 4)
	target := netip.MustParseAddr("127.0.0.2")
	const budget = 64 << 10
	b.ReportAllocs()
	for b.Loop() {
		var cursor uint64
		total := 0
		for {
			page := r.buildReplayPage(target, cursor, unboundedReplay(), budget)
			total += len(page.Routes)
			if !page.More {
				break
			}
			cursor = page.LastIndex
		}
		if total != 10000 {
			b.Fatalf("paged %d routes, want 10000", total)
		}
	}
}

// BenchmarkBuildReplayPageManySmallSources measures one page of a route
// server's table: 500 sources of 2,000 or 4,000 routes each, the shape of an
// IXP, read at the real page budget. No source alone fills a page, so a bound
// that ends a walk only once one source passes the budget walks and sorts the
// whole table for every page. One page's work MUST stay bounded by the page:
// doubling the routes above the page boundary MUST NOT double its cost.
func BenchmarkBuildReplayPageManySmallSources(b *testing.B) {
	const sourceCount = 500
	for _, perSource := range []int{2000, 4000} {
		b.Run(strconv.Itoa(perSource), func(b *testing.B) {
			r := benchmarkReplayStore(sourceCount*perSource, sourceCount)
			target := netip.MustParseAddr("127.0.0.2")
			b.ReportAllocs()
			for b.Loop() {
				page := r.buildReplayPage(target, 0, unboundedReplay(), ribevents.ReplayPageBudget)
				if !page.More {
					b.Fatal("the fixture must exceed one page, or it measures nothing")
				}
				if len(page.Routes) == 0 {
					b.Fatal("a page of a non-empty table must carry a route")
				}
			}
		})
	}
}
