// Design: docs/architecture/rib/unified-locrib.md -- publication order.
// Related: route_view_worker_output.go -- publish, the one caller.

package sysrib

import (
	"cmp"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

// orderForwarding turns one publication, every forwarding change one consumer
// step produced, into batches a FIB writer can apply entry by entry, in order.
//
// A FIB writer installs each entry the moment it reads it, and the kernel
// refuses a route whose gateway no installed route reaches: Linux answers
// ENETUNREACH (fib_check_nh_v4_gw), and the writer records the prefix as
// failed while sysrib records it as programmed. So an Add or an Update is
// placed after every Add or Update in the same publication that provides
// reachability to one of its gateways.
//
// Only a route with no gateway on any member provides reachability: the kernel
// looks a gateway up at link scope, which a gatewayed route never has. Sysrib
// programs the resolved, directly reachable gateway (resolveMember), so a
// recursive chain A via B via C reaches the writer as A and B each via a
// gateway inside C, and ordering after C orders the whole chain. An on-link
// gateway needs no route. A gateway whose provider is not in the publication
// is already reachable, or not, whatever the order. Every provider whose
// prefix contains a gateway is a predecessor, not only the most specific one:
// an extra predecessor costs nothing and needs no reader lookup.
//
// A withdrawal is never moved. The kernel deletes a route by its key whether
// or not another route's gateway lies inside it, and keeps the routes whose
// gateways it reached, so no deletion order can fail
// (TestFIBKernelDeletesCoverBeforeDependent).
//
// Two entries for one prefix keep their order. A route moves only behind its
// providers, and a provider moves only behind an earlier entry for its own
// prefix, so the order is acyclic unless one publication carries the same
// prefix once with a gateway and once without, ahead of a route that needs it.
// No producer does: applySelectedBatch, the pending set and the replay carry
// each prefix once, and the cascade recomputes a prefix only against the state
// its first change left. Should it happen anyway the publication is refused,
// which stops the routing-view worker and reports doctor-rib-routing-view-unavailable.
//
// Families are emitted in AFI then SAFI order. A publication that does not
// carry both a gateway-less install and a gatewayed one, the common shape of a
// BGP delta or a device-route change, costs one pass and the batch headers. One
// that carries both builds a prefix index of its providers and looks each
// gateway up once per provider prefix length in use; it copies entries only
// when one must move.
func orderForwarding(changes map[family.Family][]outgoingChange, replayID uint64) ([]outgoingBatch, error) {
	batches := make([]outgoingBatch, 0, len(changes))
	for fam, entries := range changes {
		if len(entries) == 0 {
			continue
		}
		batches = append(batches, outgoingBatch{Family: fam, ReplayID: replayID, Changes: entries})
	}
	slices.SortFunc(batches, func(a, b outgoingBatch) int {
		if order := cmp.Compare(a.Family.AFI, b.Family.AFI); order != 0 {
			return order
		}
		return cmp.Compare(a.Family.SAFI, b.Family.SAFI)
	})
	var lengths providerLengths
	dependents := false
	count := 0
	for i := range batches {
		count += len(batches[i].Changes)
		for j := range batches[i].Changes {
			entry := &batches[i].Changes[j]
			if !forwardingInstalls(entry) {
				continue
			}
			if forwardingProvides(entry) {
				lengths.add(entry.Prefix)
				continue
			}
			dependents = true
		}
	}
	if lengths.empty() {
		return batches, nil
	}
	if !dependents {
		return batches, nil
	}
	entries := make([]forwardingOrderEntry, 0, count)
	for i := range batches {
		for j := range batches[i].Changes {
			entries = append(entries, forwardingOrderEntry{family: batches[i].Family, change: &batches[i].Changes[j]})
		}
	}
	providers := make(map[netip.Prefix]int)
	for index := range entries {
		if !forwardingInstalls(entries[index].change) {
			continue
		}
		if forwardingProvides(entries[index].change) {
			providers[entries[index].change.Prefix] = index
		}
	}
	if !forwardingNeedsOrder(entries, providers, &lengths) {
		return batches, nil
	}
	ordered, err := forwardingTopological(entries, providers, &lengths)
	if err != nil {
		return nil, err
	}
	return forwardingBatches(ordered, replayID), nil
}

// forwardingOrderEntry names one change of a publication by its family and
// its position in the caller's batch, which the ordered copy reads from.
type forwardingOrderEntry struct {
	family family.Family
	change *outgoingChange
}

// forwardingInstalls reports whether the writer installs the entry now: an Add
// or an Update. A withdrawal neither needs nor provides reachability.
func forwardingInstalls(entry *outgoingChange) bool {
	verb := entry.Action.Verb()
	if verb == routeaction.VerbInstall {
		return true
	}
	return verb == routeaction.VerbReplace
}

// forwardingProvides reports whether the entry can make a gateway reachable:
// a forwarding route with no gateway on its primary or any member.
func forwardingProvides(entry *outgoingChange) bool {
	if entry.RouteType.Discards() {
		return false
	}
	if entry.NextHop.IsValid() {
		return false
	}
	for i := range entry.ECMPPaths {
		if entry.ECMPPaths[i].NextHop.IsValid() {
			return false
		}
	}
	return true
}

// forwardingGateways calls visit with each gateway of an installed entry the
// kernel must reach through another route: on-link gateways are excluded.
// The primary may repeat as a member; a repeated predecessor is harmless.
func forwardingGateways(entry *outgoingChange, visit func(netip.Addr)) {
	if entry.NextHop.IsValid() {
		if !entry.OnLink {
			visit(entry.NextHop)
		}
	}
	for i := range entry.ECMPPaths {
		member := &entry.ECMPPaths[i]
		if !member.NextHop.IsValid() {
			continue
		}
		if member.OnLink {
			continue
		}
		visit(member.NextHop)
	}
}

// forwardingNeedsOrder reports whether any installed route precedes a
// provider of one of its gateways. False leaves the publication as it came.
func forwardingNeedsOrder(entries []forwardingOrderEntry, providers map[netip.Prefix]int, lengths *providerLengths) bool {
	needs := false
	for index := range entries {
		entry := entries[index].change
		if !forwardingInstalls(entry) {
			continue
		}
		if forwardingProvides(entry) {
			continue
		}
		forwardingGateways(entry, func(gateway netip.Addr) {
			lengths.visitProviders(gateway, providers, func(provider int) {
				if provider > index {
					needs = true
				}
			})
		})
		if needs {
			return true
		}
	}
	return false
}

// forwardingTopological orders entries so every predecessor comes first, and
// otherwise keeps the earliest original position first (Kahn's algorithm over
// a min-heap of positions). Predecessors are the providers of each installed
// route's gateways and the earlier entry for the same prefix.
func forwardingTopological(entries []forwardingOrderEntry, providers map[netip.Prefix]int, lengths *providerLengths) ([]forwardingOrderEntry, error) {
	successors := make([][]int, len(entries))
	waiting := make([]int, len(entries))
	previous := make(map[prefixKey]int, len(entries))
	for index := range entries {
		key := prefixKey{family: entries[index].family, prefix: entries[index].change.Prefix}
		if earlier, exists := previous[key]; exists {
			successors[earlier] = append(successors[earlier], index)
			waiting[index]++
		}
		previous[key] = index
		entry := entries[index].change
		if !forwardingInstalls(entry) {
			continue
		}
		if forwardingProvides(entry) {
			continue
		}
		forwardingGateways(entry, func(gateway netip.Addr) {
			lengths.visitProviders(gateway, providers, func(provider int) {
				successors[provider] = append(successors[provider], index)
				waiting[index]++
			})
		})
	}
	var ready forwardingReady
	for index := range entries {
		if waiting[index] == 0 {
			ready.push(index)
		}
	}
	ordered := make([]forwardingOrderEntry, 0, len(entries))
	for len(ready) > 0 {
		index := ready.pop()
		ordered = append(ordered, entries[index])
		for _, successor := range successors[index] {
			waiting[successor]--
			if waiting[successor] == 0 {
				ready.push(successor)
			}
		}
	}
	if len(ordered) != len(entries) {
		return nil, routeViewRefusal("A forwarding publication has no installable order",
			"One prefix appears both with and without a gateway ahead of a route that needs it, so a FIB writer would refuse a route whatever the order",
			"ze request reload")
	}
	return ordered, nil
}

// forwardingBatches splits an ordered publication into consecutive runs of one
// family. Each batch owns copies of its entries.
func forwardingBatches(ordered []forwardingOrderEntry, replayID uint64) []outgoingBatch {
	var batches []outgoingBatch
	for _, entry := range ordered {
		last := len(batches) - 1
		if last < 0 || batches[last].Family != entry.family {
			batches = append(batches, outgoingBatch{Family: entry.family, ReplayID: replayID})
			last++
		}
		batches[last].Changes = append(batches[last].Changes, *entry.change)
	}
	return batches
}

// providerLengths records which prefix lengths the publication's providers
// use, per address family, so a gateway is looked up once per used length
// rather than once per provider. Its zero value records none.
type providerLengths struct {
	ipv4 uint64
	ipv6 [3]uint64
}

// add ignores an invalid prefix, whose length is -1: it contains no gateway.
func (lengths *providerLengths) add(prefix netip.Prefix) {
	bits := prefix.Bits()
	if bits < 0 {
		return
	}
	if prefix.Addr().Is4() {
		lengths.ipv4 |= 1 << bits
		return
	}
	lengths.ipv6[bits/64] |= 1 << (bits % 64)
}

func (lengths *providerLengths) empty() bool {
	return lengths.ipv4 == 0 && lengths.ipv6 == [3]uint64{}
}

// visitProviders calls visit with the position of each provider whose prefix
// contains gateway. Providers of the other address family never match.
func (lengths *providerLengths) visitProviders(gateway netip.Addr, providers map[netip.Prefix]int, visit func(int)) {
	if gateway.Is4() {
		visitProviderWord(gateway, lengths.ipv4, 0, providers, visit)
		return
	}
	for word, used := range lengths.ipv6 {
		visitProviderWord(gateway, used, word*64, providers, visit)
	}
}

// visitProviderWord looks gateway up at each length set in used, offset by base.
func visitProviderWord(gateway netip.Addr, used uint64, base int, providers map[netip.Prefix]int, visit func(int)) {
	for bit := range 64 {
		if used&(1<<bit) == 0 {
			continue
		}
		cover, err := gateway.Prefix(base + bit)
		if err != nil {
			continue
		}
		if provider, exists := providers[cover]; exists {
			visit(provider)
		}
	}
}

// forwardingReady is a min-heap of entry positions whose predecessors have all
// been emitted. Its zero value is empty.
type forwardingReady []int

func (ready *forwardingReady) push(index int) {
	heap := append(*ready, index)
	for child := len(heap) - 1; child > 0; {
		parent := (child - 1) / 2
		if heap[parent] <= heap[child] {
			break
		}
		heap[parent], heap[child] = heap[child], heap[parent]
		child = parent
	}
	*ready = heap
}

// pop MUST be called only on a non-empty heap.
func (ready *forwardingReady) pop() int {
	heap := *ready
	top := heap[0]
	last := len(heap) - 1
	heap[0] = heap[last]
	heap = heap[:last]
	for parent := 0; ; {
		smallest := parent
		for _, child := range [2]int{2*parent + 1, 2*parent + 2} {
			if child >= len(heap) {
				continue
			}
			if heap[child] < heap[smallest] {
				smallest = child
			}
		}
		if smallest == parent {
			break
		}
		heap[parent], heap[smallest] = heap[smallest], heap[parent]
		parent = smallest
	}
	*ready = heap
	return top
}
