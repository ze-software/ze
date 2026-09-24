// Design: docs/architecture/ospf/ospf-8-spf-rib.md -- completed native SPF reachability.
// RFC 9552 Section 5.9: unreachable-node withdrawal and restoration use the IGP result,
// not whether a node happens to originate a selected IP prefix.
package spf

import "github.com/ze-software/ze/internal/plugins/ospf/types"

// ReachabilitySnapshot is a coherent, read-only view of one completed SPF run.
// Its maps and slices remain owned by the Computer and are never mutated after publication.
// Holding this value across another run preserves the earlier view without copying the tree.
type ReachabilitySnapshot struct {
	root         types.RouterID
	results      map[types.AreaID]*Result
	border       []BorderRouterEntry
	generation   uint64
	knownOrigins map[types.RouterID]struct{}
}

// Reachability captures native reachability without copying the SPF tree. An empty
// snapshot is returned before a completed run, after Stop, or while changed configuration
// has not yet produced a matching run. Queries on the captured value do not take locks.
func (c *Computer) Reachability() ReachabilitySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.reachability.generation != c.configGeneration {
		return ReachabilitySnapshot{}
	}
	return c.reachability
}

// Ready reports whether this value contains a completed result for its configuration.
func (r ReachabilitySnapshot) Ready() bool {
	return r.root != (types.RouterID{}) && r.results != nil
}

// RouterKnown reports whether the completed native area input contained this
// origin. An absent, newly arriving origin is unknown, not proven unreachable.
func (r ReachabilitySnapshot) RouterKnown(area types.AreaID, id types.RouterID) bool {
	if !r.Ready() || id == (types.RouterID{}) {
		return false
	}
	result := r.results[area]
	if result == nil || result.Graph == nil {
		return false
	}
	_, known := result.Graph.knownOrigins[id]
	return known || result.Graph.Routers[id] != nil
}

// RouterKnownAny includes native area inputs, AS-scope input advertisers and
// decoded inter-area ASBR targets, including candidates without a usable path.
func (r ReachabilitySnapshot) RouterKnownAny(id types.RouterID) bool {
	if !r.Ready() || id == (types.RouterID{}) {
		return false
	}
	if _, known := r.knownOrigins[id]; known {
		return true
	}
	for area, result := range r.results {
		if r.RouterKnown(area, id) {
			return true
		}
		if result != nil && result.Graph != nil {
			if _, known := result.Graph.knownASOrigins[id]; known {
				return true
			}
		}
	}
	return false
}

// NetworkKnown reports whether this exact native network identity was consumed,
// even when another v2 DR's Network-LSA won the same graph-local LAN key.
func (r ReachabilitySnapshot) NetworkKnown(area types.AreaID, dr types.RouterID, id types.LinkStateID, v3 bool) bool {
	if !r.Ready() {
		return false
	}
	result := r.results[area]
	if result == nil || result.Graph == nil {
		return false
	}
	if _, known := result.Graph.knownNetworks[networkOrigin{dr: dr, id: id}]; known {
		return true
	}
	_, known := nativeNetworkKey(result.Graph, dr, id, v3)
	return known
}

// RouterReachable reports native intra-area reachability, including routers that
// advertise no stub prefixes. Reachability in another area cannot satisfy this query.
func (r ReachabilitySnapshot) RouterReachable(area types.AreaID, id types.RouterID) bool {
	if !r.Ready() || id == (types.RouterID{}) {
		return false
	}
	return reachedVertex(r.results[area], routerVertex(id))
}

// RouterReachableAny reports native intra-area or resolved inter-area router reachability.
// AS-scope objects may use this query; area-scoped objects must use RouterReachable.
func (r ReachabilitySnapshot) RouterReachableAny(id types.RouterID) bool {
	if !r.Ready() || id == (types.RouterID{}) {
		return false
	}
	for _, result := range r.results {
		if reachedVertex(result, routerVertex(id)) {
			return true
		}
	}
	for _, entry := range r.border {
		if entry.RouterID == id && entry.Metric < LSInfinity && len(entry.NextHops) != 0 {
			return true
		}
	}
	return false
}

// NetworkReachable resolves a transit-network pseudonode in its own area.
// Both families retain the advertising DR identity: OSPFv2 additionally keys
// by Network-LSA ID, while OSPFv3 uses the DR Interface ID instead of the graph's
// synthetic network vertex ID.
func (r ReachabilitySnapshot) NetworkReachable(area types.AreaID, dr types.RouterID, id types.LinkStateID, v3 bool) bool {
	if !r.Ready() {
		return false
	}
	result := r.results[area]
	if result == nil || result.Graph == nil {
		return false
	}
	key, known := nativeNetworkKey(result.Graph, dr, id, v3)
	return known && reachedVertex(result, networkVertex(key))
}

func nativeNetworkKey(graph *Graph, dr types.RouterID, id types.LinkStateID, v3 bool) (types.LinkStateID, bool) {
	if !v3 {
		network := graph.Networks[id]
		return id, network != nil && network.AdvertisingDR == dr
	}
	for key, network := range graph.Networks {
		if network != nil && network.AdvertisingDR == dr && network.DRInterfaceID == id {
			return key, true
		}
	}
	return types.LinkStateID{}, false
}

func reachedVertex(result *Result, vertex VertexID) bool {
	if result == nil {
		return false
	}
	node := result.Nodes[vertex]
	return node != nil && node.Metric < LSInfinity
}
