// Design: docs/architecture/api/process-protocol.md — plugin->engine RPC framing
// RFC: rfc/short/rfc4271.md -- Section 4, the 4096-octet ceiling on one UPDATE
// RFC: rfc/short/rfc8654.md -- Section 3, the extended 65535-octet ceiling
// Overview: rib.go — relayRoutes, which walks a peer-up replay through this chunker
// Related: rib_commands.go — the replay command whose relay this bounds

package adj_rib_in

import "github.com/ze-software/ze/pkg/plugin/rpc"

// relayFrameReserve is what relayChunkBudget leaves free inside
// rpc.MaxMessageSize for the parts of a relay frame that are not routes.
// Those are the JSON-RPC envelope, the method name, the destination address,
// and the brackets around the routes array. They cost a few hundred bytes
// today. The reserve is two orders of magnitude above that, so a field added
// to rpc.RelayStoredRouteInput cannot push a chunk over the ceiling.
const relayFrameReserve = 4096

// relayChunkBudget is the serialized size one relay call MUST stay under.
const relayChunkBudget = rpc.MaxMessageSize - relayFrameReserve

// relayRouteJSONFixed is what one route costs in the serialized routes array
// with every string value empty.
//
// The derivation, for the widest form of each non-string field:
//
//	{"source-peer":"","family":"","attr-hex":"","next-hop-hex":"","nlri-hex":"","path-id":4294967295,"nlri-framing":"source-wire"},
//
// That is 127 characters: 126 for the object and one for the comma that
// separates it from the next route. 128 carries one byte of slack.
// TestRelayRouteJSONMaxBoundsMarshal pins the number against encoding/json, so a
// field added to rpc.StoredRoute reddens a test rather than losing a chunk.
const relayRouteJSONFixed = 128

// relayRouteJSONMax reports how many bytes one route adds to the serialized
// rpc.RelayStoredRouteInput.
//
// An upper bound, never an estimate. The chunker exists to stay under a ceiling
// the framing REFUSES. A bound that reads low loses the whole chunk
// (ai/rules/evidence.md).
//
// Each of the five string fields encodes to its own byte length. They carry an
// IP address, a family name and hex digits, and JSON escapes none of those.
func relayRouteJSONMax(route *rpc.StoredRoute) int {
	return relayRouteJSONFixed +
		len(route.SourcePeer) + len(route.Family) +
		len(route.AttrHex) + len(route.NextHopHex) + len(route.NLRIHex)
}

// relayChunkEnd returns the exclusive end of the chunk that starts at start.
// Caller MUST pass a start inside routes.
//
// The bound is BYTES, because bytes are what the transport refuses.
// WriteMessage (pkg/plugin/rpc/framing.go) rejects a message above
// rpc.MaxMessageSize and does not split one. An oversized chunk therefore
// loses every route in it at once.
//
// A route COUNT bounds nothing here. AttrHex is hex, so it is twice the
// attribute block it carries. The 4096-route chunk this replaced serialized to
// about 33 MB for a table whose routes carry a 4 KiB block, which large
// communities or a long AS_PATH reach.
//
// The count did not bound the engine's reconstruction buffers either.
// RelayStoredRoute (internal/component/bgp/reactor/reactor_api_relay.go)
// releases each route's cache entry before it builds the next one. The retains
// that outlive the call are the per-destination writes, and the chunk size
// never governed those.
//
// A chunk always carries at least one route, so the walk in relayRoutes always
// advances. That escape cannot fire on a real route. RFC 4271 Section 4 caps an
// UPDATE at 4096 octets and RFC 8654 Section 3 at 65535, so one route's
// attribute and NLRI hex together reach 131070 characters at the very most.
// That is an eightieth of the budget.
func relayChunkEnd(routes []rpc.StoredRoute, start, budget int) int {
	used := relayRouteJSONMax(&routes[start])
	for end := start + 1; end < len(routes); end++ {
		cost := relayRouteJSONMax(&routes[end])
		if used+cost > budget {
			return end
		}
		used += cost
	}
	return len(routes)
}
