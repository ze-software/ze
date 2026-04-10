// Design: docs/architecture/api/architecture.md -- BGP route filter pipeline
//
// BGP-specific filter types for ingress/egress route filtering and attribute
// modification. These types are used by the BGP reactor and BGP plugins
// (community filter, OTC, GR, loop detection) to implement the route
// filter pipeline.
//
// Other protocols (OSPF, IS-IS) would define their own filter types in
// their own registry files, following the same registration pattern.

package registry

import (
	"maps"
	"net/netip"
)

// PeerFilterInfo holds BGP peer metadata for filter decisions.
// Passed by the reactor to registered filter functions.
type PeerFilterInfo struct {
	Address      netip.Addr // Peer IP address
	PeerAS       uint32     // Remote AS number
	LocalAS      uint32     // Local AS number (for iBGP detection)
	RouterID     uint32     // Local Router ID (for ORIGINATOR_ID/CLUSTER_LIST loop detection)
	ASN4         bool       // 4-byte ASN negotiated (affects AS_PATH parsing)
	Name         string     // Peer name from config (for filter config lookup)
	GroupName    string     // Group name (empty if standalone peer)
	AllowOwnAS   uint8      // Loop detection: number of own-AS occurrences to tolerate (0 = reject on first)
	ClusterID    uint32     // Loop detection: explicit cluster-id (0 = use RouterID)
	LoopDisabled bool       // Loop detection deactivated for this peer (inactive: prefix)
}

// IngressFilterFunc is called for received UPDATEs before caching and dispatching.
// payload is the UPDATE body (without BGP header).
// meta is a shared metadata map; filters can read and write to it.
// Caller MUST pass a non-nil meta map; writing to a nil meta panics.
// Returns accept=false to drop the route. If modifiedPayload is non-nil,
// it replaces the original payload for caching and event dispatch.
type IngressFilterFunc func(source PeerFilterInfo, payload []byte, meta map[string]any) (accept bool, modifiedPayload []byte)

// EgressFilterFunc is called per destination peer during ForwardUpdate.
// payload is the UPDATE body (without BGP header).
// meta is route metadata set at ingress (read-only); may be nil.
// mods accumulates per-peer modifications applied after all filters pass.
// MUST NOT retain the mods pointer beyond the call -- it is reused per peer.
// Returns false to suppress the route for this destination peer.
type EgressFilterFunc func(source, dest PeerFilterInfo, payload []byte, meta map[string]any, mods *ModAccumulator) bool

// ModAccumulator collects per-peer route modifications from egress filters.
// NOT safe for concurrent use. Each peer iteration gets a fresh instance.
type ModAccumulator struct {
	ops      []AttrOp
	withdraw bool // convert announce to withdrawal for this peer
}

// Len returns the number of accumulated modifications (excluding withdraw flag).
func (a *ModAccumulator) Len() int { return len(a.ops) }

// HasMods returns true if any modifications have been accumulated (ops or withdraw).
func (a *ModAccumulator) HasMods() bool { return len(a.ops) > 0 || a.withdraw }

// Reset clears all accumulated modifications for reuse.
func (a *ModAccumulator) Reset() {
	a.ops = a.ops[:0]
	a.withdraw = false
}

// Op accumulates an attribute modification operation.
// Lazily allocates the slice on first call to avoid allocation
// when no filter writes modifications (the common case).
// Multiple calls with the same code are allowed -- the handler
// receives all ops for a given code at once during the progressive build.
func (a *ModAccumulator) Op(code, action uint8, buf []byte) {
	a.ops = append(a.ops, AttrOp{Code: code, Action: action, Buf: buf})
}

// Ops returns the accumulated attribute modification operations.
// Returns nil if no ops have been accumulated.
func (a *ModAccumulator) Ops() []AttrOp { return a.ops }

// SetWithdraw marks this route for withdrawal conversion.
// The forward path will convert the announce UPDATE to a withdrawal
// for this destination peer. Used by LLGR egress filter (RFC 9494)
// to withdraw stale routes from EBGP non-LLGR peers.
func (a *ModAccumulator) SetWithdraw() { a.withdraw = true }

// IsWithdraw returns true if the route should be converted to a withdrawal.
func (a *ModAccumulator) IsWithdraw() bool { return a.withdraw }

// Attribute modification action constants.
const (
	AttrModSet     uint8 = iota // Replace entire attribute value (or create if absent)
	AttrModAdd                  // Append value to attribute's list (e.g., COMMUNITY)
	AttrModRemove               // Remove value from attribute's list (e.g., COMMUNITY)
	AttrModPrepend              // Prepend value to attribute's sequence (e.g., AS_PATH)
)

// Filter stage constants define coarse ordering classes for the filter pipeline.
// Filters are sorted by stage first, then by priority within a stage, then by name.
// Gaps between values allow inserting new stages without renumbering.
const (
	FilterStageProtocol   int = 0   // RFC-mandated checks (loop detection, RFC 4271/4456)
	FilterStagePolicy     int = 100 // Operator-configured filtering (community, prefix, IRR)
	FilterStageAnnotation int = 200 // Protocol modifications that stamp routes (OTC/RFC 9234)
)

// AttrOp describes a single attribute modification operation.
// Egress filters accumulate AttrOps in the ModAccumulator via Op().
// Multiple AttrOps with the same Code are allowed and are passed together
// to the registered handler during the progressive build.
type AttrOp struct {
	Code   uint8  // Attribute type code (e.g., 35 for OTC, 8 for COMMUNITY)
	Action uint8  // AttrModSet, AttrModAdd, AttrModRemove, AttrModPrepend
	Buf    []byte // Pre-built wire bytes of the VALUE (handler writes the header)
}

// AttrModHandler is a per-attribute-code handler for the progressive build.
// It receives the source attribute bytes (nil if absent in source), all ops
// for this attribute code, the output buffer, and the current write offset.
// It writes the complete attribute (header + value) into buf and returns
// the new offset after the written bytes.
// It cannot reject a route -- only transform. MUST NOT retain buf beyond the call.
type AttrModHandler func(src []byte, ops []AttrOp, buf []byte, off int) int

// attrModHandlers stores registered attr mod handlers keyed by attribute code.
// Populated at init() time by plugins, read at runtime by the reactor.
var attrModHandlers = make(map[uint8]AttrModHandler)

// RegisterAttrModHandler registers a handler for the given attribute code.
// Must be called from init() functions only. Ignores nil handlers.
func RegisterAttrModHandler(code uint8, handler AttrModHandler) {
	if handler == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	attrModHandlers[code] = handler
}

// UnregisterAttrModHandler removes an attr mod handler. Only for use in tests.
func UnregisterAttrModHandler(code uint8) {
	mu.Lock()
	defer mu.Unlock()
	delete(attrModHandlers, code)
}

// AttrModHandlerFor returns the registered handler for the given attribute code, or nil.
func AttrModHandlerFor(code uint8) AttrModHandler {
	mu.RLock()
	defer mu.RUnlock()
	return attrModHandlers[code]
}

// AttrModHandlers returns a snapshot of all registered attr mod handlers.
// Called by the reactor to build the handler map at startup.
func AttrModHandlers() map[uint8]AttrModHandler {
	mu.RLock()
	defer mu.RUnlock()
	result := make(map[uint8]AttrModHandler, len(attrModHandlers))
	maps.Copy(result, attrModHandlers)
	return result
}
