// Design: docs/architecture/api/architecture.md -- the send half of an attach block, owner-relayed peer-up replay
// Overview: rib.go -- walkReplayRoutes, the walk a page is cut from
// Related: relay_chunk.go -- relayRouteJSONMax, the byte measure a page is bounded by
// Related: ../../../../core/bgp/ribevents/replay.go -- the page contract and the owner's relay loop

package adj_rib_in

import (
	"cmp"
	"fmt"
	"net/netip"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// replayRoutesCommand handles ribevents.ReplayRoutesCommand:
// "<target> <from-index> [<max-msg-id>]", the arguments of replayCommand, with
// the routes ANSWERED as one page instead of relayed.
//
// The replay owner relays the page itself, because the target's
// `send [ update ]` grant is the owner's and the operator gives this store none.
// replayCommand's relay goes out under this store's name, so the engine judges
// this store's grant and refuses it toward a peer that attached only the owner.
func (r *AdjRIBInManager) replayRoutesCommand(args []string) (string, any, error) {
	if len(args) < 2 {
		return statusError, "", errAdjRibInReplayRoutesArgs
	}
	targetPeer, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("adj-rib-in replay-routes: invalid target peer address %q: %w (expected an IP address)", args[0], err)
	}
	fromIndex, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return statusError, "", fmt.Errorf("adj-rib-in replay-routes: invalid from-index %q: %w (expected a non-negative integer)", args[1], err)
	}
	cut := unboundedReplay()
	if len(args) > 2 {
		maxMsgID, parseErr := strconv.ParseUint(args[2], 10, 64)
		if parseErr != nil {
			return statusError, "", fmt.Errorf("adj-rib-in replay-routes: invalid max-msg-id %q: %w (expected a non-negative integer)", args[2], parseErr)
		}
		cut = replayUpTo(maxMsgID)
	}

	page := r.buildReplayPage(targetPeer, fromIndex, cut, ribevents.ReplayPageBudget)
	// Omitted rather than zeroed when untracked: 0 is the real "nothing
	// ingested yet" (replay_cut.go), and bgp-rs waits on the difference.
	if pos, tracked := r.ingestPosition(); tracked {
		page.IngestedMsgID = &pos
	}
	return statusDone, page, nil
}

// replaySeqRoute is one replayed route with the store sequence it sits at,
// which orders a page and is the cursor the next page resumes from.
type replaySeqRoute struct {
	seq   uint64
	route rpc.StoredRoute
}

// buildReplayPage answers ribevents.ReplayRoutesCommand: the routes
// buildReplayRoutes would replay, in ascending sequence, cut to the first
// budget bytes of serialized routes. More says the walk left routes out; the
// caller resumes from LastIndex, which the next walk skips strictly (fromIndex
// is a resume cursor, see buildReplayRoutes), so no route is sent twice. A page
// always carries at least one route when the store holds one past fromIndex,
// so a paging caller always advances.
//
// The sequence counter is shared by every source, so one source's routes
// interleave with another's and the page is cut from their merge. One bound
// holds across every source: held keeps the lowest sequences seen so far that
// fit the budget together, and evicts its highest whenever a route pushes it
// over. Once it has evicted, a route above its highest sequence cannot belong
// to the page, and neither can any later route of that source, so the source's
// walk ends there. The highest held sequence only falls after the first
// eviction, so every route left out sits above the final LastIndex, and the
// page holds every eligible route at or below it: no route is skipped.
//
// A page therefore keeps at most budget bytes of routes in hand, plus the one
// it is about to evict, and sorts only those. The walk still visits the first
// route of each source past fromIndex, and the routes of a source that sit
// below the page's boundary before the boundary is known.
func (r *AdjRIBInManager) buildReplayPage(targetPeer netip.Addr, fromIndex uint64, cut replayCut, budget int) ribevents.ReplayPage {
	var held replayPageHeap
	// evicted records that held dropped a route past the budget. It is the
	// guard that lets a source's walk end: before it, held is not full and a
	// higher sequence may still belong to the page. It is also More, because
	// an evicted or unvisited route is still owed.
	evicted := false
	r.walkReplayRoutes(targetPeer, fromIndex, cut, func(seq uint64, from string, rt *RawRoute) bool {
		if evicted && seq > held.seqMax() {
			return false
		}
		held.push(replaySeqRoute{seq: seq, route: replayStoredRoute(from, rt)})
		// A lone route larger than the budget stays: the page carries it
		// alone, so the caller still advances past it.
		for held.octets > budget && len(held.routes) > 1 {
			held.popMax()
			evicted = true
		}
		return true
	})
	page := ribevents.ReplayPage{Routes: []rpc.StoredRoute{}}
	if len(held.routes) == 0 {
		return page
	}
	slices.SortFunc(held.routes, func(a, b replaySeqRoute) int { return cmp.Compare(a.seq, b.seq) })
	page.Routes = make([]rpc.StoredRoute, len(held.routes))
	for i := range held.routes {
		page.Routes[i] = held.routes[i].route
	}
	page.LastIndex = held.routes[len(held.routes)-1].seq
	page.More = evicted
	return page
}

// replayPageHeap is a max-heap on sequence of the routes one page holds, with
// their serialized size (relayRouteJSONMax) summed in octets. Not safe for
// concurrent use; buildReplayPage owns one for one walk.
type replayPageHeap struct {
	routes []replaySeqRoute
	octets int
}

// seqMax is the highest held sequence. Caller MUST hold at least one route.
func (h *replayPageHeap) seqMax() uint64 {
	return h.routes[0].seq
}

// push adds item. The sift-up loop is bounded by the heap's depth, log2 of the
// held count.
func (h *replayPageHeap) push(item replaySeqRoute) {
	h.routes = append(h.routes, item)
	i := len(h.routes) - 1
	h.octets += relayRouteJSONMax(&h.routes[i].route)
	for i > 0 {
		parent := (i - 1) / 2
		if h.routes[parent].seq > h.routes[i].seq {
			return
		}
		h.routes[parent], h.routes[i] = h.routes[i], h.routes[parent]
		i = parent
	}
}

// popMax removes the route with the highest sequence. Caller MUST hold at
// least one route. The sift-down loop is bounded by the heap's depth.
func (h *replayPageHeap) popMax() {
	last := len(h.routes) - 1
	h.octets -= relayRouteJSONMax(&h.routes[0].route)
	h.routes[0] = h.routes[last]
	h.routes[last] = replaySeqRoute{} // Drop the strings the slot still references.
	h.routes = h.routes[:last]
	i := 0
	for {
		largest := i
		if left := 2*i + 1; left < last {
			if h.routes[left].seq > h.routes[largest].seq {
				largest = left
			}
		}
		if right := 2*i + 2; right < last {
			if h.routes[right].seq > h.routes[largest].seq {
				largest = right
			}
		}
		if largest == i {
			return
		}
		h.routes[largest], h.routes[i] = h.routes[i], h.routes[largest]
		i = largest
	}
}
