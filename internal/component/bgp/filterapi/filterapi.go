// Design: docs/architecture/api/architecture.md -- BGP route filter pipeline
// RFC: rfc/short/rfc4271.md -- path attribute flags, codes and the value length cap (Section 4.3)
// RFC: rfc/short/rfc6793.md -- the four-octet AS number re-encoding an AttrGenerator exists for
// Detail: editset.go -- the edit set: slots, fragments, the arena and the exact size query
// Related: filterapi_test.go -- ordering and accumulator semantics moved from the generic registry tests
//
// Package filterapi defines the BGP route filter pipeline contract: the
// value types passed to ingress/egress filters, the modification
// accumulator egress filters write to, and the registration of filter
// chains and attribute-modification handlers. It also carries plugin-owned
// reactor capabilities that follow the same init()-time registration model,
// such as route-server (RS) fast-path forwarding (EnableRSForwarding). It is
// a near-leaf package (stdlib plus the core textbuf string builder) so filter
// plugins, the reactor, and protocol filters can all import it without cycles.
//
// These types are BGP-owned: other protocols (OSPF, IS-IS) would define
// their own filter seam packages following the same registration pattern.
// The generic plugin registry (internal/component/plugin/registry) carries
// no protocol filter knowledge.

package filterapi

import (
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
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
	LoopDisabled bool       // Loop detection deactivated for this peer (FilterRef.Inactive)
}

// FilterRef is one entry in a peer's import/export filter chain: the canonical
// filter name plus whether the operator deactivated it. Deactivated refs stay
// in the chain (so loop detection can still suppress the default ingress via
// LoopDisabled) but are never executed. It replaces the former in-band
// "inactive:" name prefix, so deactivation is a structural bool rather than a
// string convention any consumer must parse.
type FilterRef struct {
	Name     string
	Inactive bool
}

// FilterRefStrings renders a filter chain to the canonical string form used at
// display/API boundaries (the peer/policy commands and the birdwatcher-style
// plugin protocol), re-attaching the "inactive:" prefix for deactivated refs so
// that user-facing and plugin-facing output stays byte-identical. This is the
// one presentation seam that reconstructs the prefix; no logic path does.
func FilterRefStrings(refs []FilterRef) []string {
	if len(refs) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	out := make([]string, len(refs))
	for i, ref := range refs {
		if ref.Inactive {
			out[i] = tb.Reset().Str("inactive:").Str(ref.Name).String()
		} else {
			out[i] = ref.Name
		}
	}
	return out
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
//
// Returns false to suppress the route for this destination peer.
//
// KNOWN GAP in the seam, not a design choice: a bare bool cannot say "I could
// not decide". A filter that cannot evaluate its input can only answer accept or
// reject, and a reject is read by every consumer as a POLICY decision. That
// matters at the far end of the forward: forwardUpdateCore reports every
// destination being suppressed as errAllDestinationsSuppressed, and the
// stored-route relay counts that as a route it handled, so a filter that could
// not decide could suppress a whole peer-up replay and have it reported
// complete. safeEgressFilter (reactor/reactor_notify.go) can therefore report
// only the one non-decision it causes itself, a recovered panic.
//
// NO REGISTERED FILTER IS IN THAT STATE TODAY, re-derived from all three on
// 2026-08-03. OTCEgressFilter (plugins/role/otc.go) was the live instance; it
// stopped being one when Thomas ruled that an unrecorded destination role is
// covered by the operator's `unknown` token, which makes its export-set
// suppression a decision (R6-1 / Q-1,
// plan/spec-fixit-stored-route-relay-hardening.md). LLGREgressFilter
// (plugins/gr/gr_egress.go) does have a state it cannot evaluate, its plugin
// state not yet loaded, but it answers ACCEPT there, and a second return would
// not be read for an accepted route. filter_community's egress filter never
// suppresses at all.
//
// So widening this signature is NOT currently owed. What the seam did owe was
// smaller: two of the three call sites of safeEgressFilter DISCARDED the
// panicked return this seam already produces, so on those rails a filter panic
// was reported as policy. Reading the existing return fixed that (2026-08-03,
// AC-7 of plan/spec-fixit-stored-route-relay-hardening.md); a new one would not
// have. All three call sites now read both returns.
type EgressFilterFunc func(source, dest PeerFilterInfo, payload []byte, meta map[string]any, mods *ModAccumulator) bool

const modAccumulatorInlineBytes = 64

// maxGenerators bounds the generators one destination may record. AttrOp.GenIdx
// is one byte and reserves 0 for "no generator", so 255 is the ceiling the
// encoding allows. The AS-path family records at most four (AS_PATH, AS4_PATH,
// AGGREGATOR, AS4_AGGREGATOR), so the inline capacity below is the real figure
// and this constant only stops the index wrapping.
const maxGenerators = 255

// genInline is the number of generators held without a heap allocation. Four
// covers the whole AS-path family, which is the only producer today; eight
// leaves room for a second family without changing the structure.
const genInline = 8

// opsInline is the number of operations held without a heap allocation. The
// measured common case is four per modified UPDATE (opsArr's doc records the
// three allocations a nil slice cost at that size), so eight absorbs it with
// headroom while keeping ModAccumulator small enough to hoist above the
// destination loop. A destination exceeding it falls back to append, which is
// correct and merely allocates.
const opsInline = 8

// ModAccumulator collects per-peer route modifications from egress filters.
// NOT safe for concurrent use. Each peer iteration gets a fresh instance.
type ModAccumulator struct {
	ops              []AttrOp
	withdraw         bool   // convert announce to withdrawal for this peer
	nlriRewrite      []byte // replacement for the announced (reachable) NLRI section
	withdrawnRewrite []byte // replacement for the withdrawn NLRI section
	inline           [modAccumulatorInlineBytes]byte
	inlineOff        int

	// opsArr backs ops until a destination exceeds opsInline operations, so the
	// common modify records every operation without a heap allocation. Growing
	// a nil slice cost one allocation per doubling -- three per modified UPDATE
	// at four operations, multiplied by the export fan-out, which made the
	// append the single largest allocation source on the filter-delta path
	// (spec-perf-next-2-filter-delta-alloc, Phase B).
	//
	// Reset re-slices ops to zero length and does NOT re-zero this array, for
	// the reason Reset's doc gives: the entries are unreachable at length zero
	// and zeroing would make Reset scale with capacity.
	opsArr [opsInline]AttrOp

	// gens holds the value generators recorded for this destination, addressed by
	// AttrOp.GenIdx. It is kept OUT of AttrOp so that struct stays 32 bytes; see
	// the GenIdx doc for why that size is load-bearing.
	gens    []AttrGenerator
	gensArr [genInline]AttrGenerator

	// edit is the per-destination rebuild plan: slots, fragments and arena.
	// It lives here so that hoisting the accumulator above the destination loop
	// hoists the plan storage with it (editset.go).
	edit EditSet

	// grouped records that ops has been sorted by attribute code, so the sort
	// runs once per destination however many times the grouping is asked for.
	grouped bool
}

// EditSet returns the accumulator's reusable rebuild plan.
//
// The plan is separate from the operations: the operations say WHAT to change
// and the plan says how the new bytes relate to the old ones. The rebuild reads
// both, so the plan is cleared by EditSet().Begin() at the start of a rebuild
// rather than by Reset, which would discard the operations being planned.
func (a *ModAccumulator) EditSet() *EditSet { return &a.edit }

// Len returns the number of accumulated attribute ops (excluding withdraw flag
// and NLRI rewrites). Use HasModifications to gate whether the forward path must
// rebuild the payload.
func (a *ModAccumulator) Len() int { return len(a.ops) }

// HasModifications reports whether any payload-rebuilding modification (attribute
// op, NLRI rewrite, or withdrawn rewrite) was accumulated. Announce-to-withdraw
// conversion is handled separately via IsWithdraw and is NOT counted here.
func (a *ModAccumulator) HasModifications() bool {
	return len(a.ops) > 0 || a.nlriRewrite != nil || a.withdrawnRewrite != nil
}

// Reset returns the accumulator to its empty state so one value can serve a
// whole per-destination fan-out. It is the per-destination entry point: the
// forward rails (reactor/reactor_api_forward.go forwardUpdateCore and
// reactor/forward_rs.go reactorForwardRS) declare ONE accumulator above the
// destination loop and call Reset at the top of each iteration.
//
// CALLER OBLIGATION, and it is an isolation boundary rather than a style note.
// A slice returned by Ops(), NLRIRewrite() or WithdrawnRewrite() MUST NOT be
// read after the next Reset. Op stores the caller's slice without copying and
// OpCopy hands back a window into the shared inline arena, so a slice kept
// across a Reset reads the NEXT destination's bytes -- which is one peer being
// sent another peer's attributes. Consume every operation before resetting;
// buildModifiedPayload does, by copying each op value into that destination's
// own output buffer before it returns.
//
// Reset clears every field a later destination can read: the operation count,
// the withdraw flag, both NLRI rewrites and the arena offset. It deliberately
// does NOT re-zero the inline array or the operations backing array. Both are
// unreachable once the offset and the length are zero, and zeroing either would
// make Reset scale with capacity -- which is the whole cost the hoist removes,
// and which grows as the arena grows.
func (a *ModAccumulator) Reset() {
	a.ops = a.ops[:0]
	a.withdraw = false
	a.nlriRewrite = nil
	a.withdrawnRewrite = nil
	a.inlineOff = 0
	a.grouped = false
	// Generators are the one part of the accumulator that MUST be re-zeroed
	// rather than merely re-sliced. A generator holds the previous destination's
	// payload -- an AS_PATH tail window, a parsed path -- so leaving stale
	// interface values in the backing array keeps that destination's buffer alive
	// for the whole of the next one, which is the boundary this type claims not
	// to cross. clear runs over the LENGTH, which is four at most, so the
	// constant-time property of Reset is unaffected.
	clear(a.gens)
	a.gens = nil
	a.edit.reset()
}

// Op accumulates an attribute modification operation.
// Lazily allocates the slice on first call to avoid allocation
// when no filter writes modifications (the common case).
// Multiple calls with the same code are allowed -- the handler
// receives all ops for a given code at once during the progressive build.
//
// CALLER OBLIGATION, list-valued attributes. For an attribute whose value is a
// list of fixed-width wire values -- COMMUNITY (4), EXTENDED_COMMUNITY (8),
// LARGE_COMMUNITY (12) -- buf MUST be a whole number of those values,
// concatenated. One value is the common case; several in one operation is
// explicitly allowed and means "remove/add every one of them".
//
// This is stated here because leaving it unstated cost a live defect. The
// route-server strip path emits every control community as one concatenated
// buffer (internal/component/bgp/wireu/community.go:141, reaching Op at
// reactor/reactor_api_forward.go:635 and reactor/forward_rs.go:342), while the
// COMMUNITY handler's removal helper accepted ONLY a single value and returned
// the data untouched otherwise -- silently, with the comment "caller bug". Any
// route carrying two or more control communities therefore had none of them
// stripped, and nothing anywhere said so. The handler now accepts a whole number
// of values and warns on anything else; see
// internal/component/bgp/plugins/filter_community/handler.go removeValues.
//
// Op does NOT validate this. It has no attribute-width table and runs per
// forwarded UPDATE; the check belongs at the handler that already knows its own
// value width.
func (a *ModAccumulator) Op(code, action uint8, buf []byte) {
	if a.ops == nil {
		a.ops = a.opsArr[:0]
	}
	a.ops = append(a.ops, AttrOp{Code: code, Action: action, Buf: buf})
}

// OpGen accumulates a Set operation whose value is produced by a generator
// rather than by pre-built bytes.
//
// It is the recording half of the generate path: the producer says WHAT the
// attribute becomes without building it, and the generator writes those bytes
// straight into the destination when the rebuild materializes. Use it when
// pre-building would mean staging the value in a scratch buffer first, which is
// a copy the exactly-sized rebuild exists to remove.
//
// The action is Set, so the operation composes with every existing handler
// exactly as a Set carrying Buf does: last Set wins, a later Suppress still
// drops the attribute, and no handler needs to know a generator was involved.
//
// Like Op, the accumulator does not copy: the generator must stay valid, and
// must not be mutated, until the forward call returns.
func (a *ModAccumulator) OpGen(code uint8, gen AttrGenerator) {
	if gen == nil {
		return
	}
	if a.gens == nil {
		a.gens = a.gensArr[:0]
	}
	if len(a.gens) >= maxGenerators {
		// The index is one byte and 0 means "none", so the store is bounded.
		// Refusing here rather than truncating keeps a generator from being
		// recorded and then silently resolved to a different one.
		return
	}
	a.gens = append(a.gens, gen)
	a.ops = append(a.ops, AttrOp{
		Code:   code,
		Action: AttrModSet,
		GenIdx: uint8(len(a.gens)), //nolint:gosec // G115: bounded by the maxGenerators check above
	})
}

// Gens returns the generators recorded for this destination. The rebuild hands
// them to the edit set so a slot can resolve an operation's GenIdx.
func (a *ModAccumulator) Gens() []AttrGenerator { return a.gens }

// OpCopy accumulates an attribute modification after copying buf into
// accumulator-owned storage. Use it when the source buffer is stack-owned or
// otherwise shorter-lived than the accumulator.
func (a *ModAccumulator) OpCopy(code, action uint8, buf []byte) {
	if len(buf) == 0 {
		a.Op(code, action, nil)
		return
	}
	if len(buf) <= len(a.inline)-a.inlineOff {
		start := a.inlineOff
		a.inlineOff += len(buf)
		copy(a.inline[start:a.inlineOff], buf)
		a.Op(code, action, a.inline[start:a.inlineOff])
		return
	}
	copied := make([]byte, len(buf))
	copy(copied, buf)
	a.Op(code, action, copied)
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

// SetNLRIRewrite records a replacement for the announced (reachable) legacy IPv4
// NLRI section of the per-peer UPDATE. The forward path substitutes these bytes
// for the original NLRI when building the peer's UPDATE, enabling per-peer prefix
// translation. A nil rewrite leaves the NLRI unchanged; a zero-length (non-nil)
// slice drops every legacy NLRI prefix. The bytes must be a valid wire NLRI
// section (length-prefixed prefixes). Like Op, the accumulator does not copy the
// slice, so the caller's bytes must stay valid until the forward call returns.
func (a *ModAccumulator) SetNLRIRewrite(b []byte) { a.nlriRewrite = b }

// NLRIRewrite returns the accumulated NLRI rewrite, or nil if none was set.
func (a *ModAccumulator) NLRIRewrite() []byte { return a.nlriRewrite }

// SetWithdrawnRewrite records a replacement for the withdrawn NLRI section of the
// per-peer UPDATE, so a prefix rewritten on announce is withdrawn under the same
// rewritten prefix (adj-rib-out consistency: the peer never sees a withdrawal for
// a prefix it was not sent). A nil rewrite leaves the withdrawn section unchanged.
func (a *ModAccumulator) SetWithdrawnRewrite(b []byte) { a.withdrawnRewrite = b }

// WithdrawnRewrite returns the accumulated withdrawn-NLRI rewrite, or nil.
func (a *ModAccumulator) WithdrawnRewrite() []byte { return a.withdrawnRewrite }

// Attribute modification action constants.
const (
	AttrModSet      uint8 = iota // Replace entire attribute value (or create if absent)
	AttrModAdd                   // Append value to attribute's list (e.g., COMMUNITY)
	AttrModRemove                // Remove value from attribute's list (e.g., COMMUNITY)
	AttrModPrepend               // Prepend value to attribute's sequence (e.g., AS_PATH)
	AttrModSuppress              // Remove entire attribute from UPDATE (handler writes nothing)
)

// LastSetOrSuppress returns the index of the last AttrModSet or AttrModSuppress
// operation and whether that last one was a Suppress. Last wins, which is the
// rule a filter chain's ordering depends on: a later stage must be able to
// override an earlier stage's decision in either direction.
//
// It lives here, in the package every handler already imports, because it is a
// CONTRACT between producers and handlers rather than any one handler's private
// logic. Six handlers used to decide "last op wins" independently and FOUR of
// them silently forgot AttrModSuppress: the community handler
// (filter_community) scanned for Set alone, so a `session { community { send
// none } }` peer received every community anyway; OTC (role), CLUSTER_LIST and
// ORIGINATOR_ID (reactor/filter_delta_handlers.go) had the same blind spot. Only
// genericAttrSetHandler and aspathHandler read the action. A single fold cannot
// disagree with itself.
//
// mpReachNextHopHandler still ignores Suppress, deliberately and with its reason
// written down: suppressing MP_REACH_NLRI would strip the route, which is a
// withdraw, and that is expressed through ModAccumulator.SetWithdraw instead.
//
// idx is -1 when the operations contain neither action, which means the handler
// should fall back to its own Add/Remove/Prepend logic, or keep the source.
func LastSetOrSuppress(ops []AttrOp) (idx int, suppress bool) {
	idx = -1
	for i := range ops {
		switch ops[i].Action {
		case AttrModSet:
			idx, suppress = i, false
		case AttrModSuppress:
			idx, suppress = i, true
		}
	}
	return idx, suppress
}

// Filter stage constants define coarse ordering classes for the filter pipeline.
// Filters are sorted by stage first, then by priority within a stage, then by name.
// Gaps between values allow inserting new stages without renumbering.
const (
	FilterStageProtocol   int = 0   // RFC-mandated checks (loop detection, RFC 4271/4456)
	FilterStagePolicy     int = 100 // Operator-configured filtering (community, prefix, IRR)
	FilterStageAnnotation int = 200 // Protocol modifications that stamp routes (OTC/RFC 9234)
	// FilterStagePeerChain orders the external per-peer configured filter chain
	// (the text/RPC PolicyFilterChain). It runs AFTER all in-process filters,
	// including Annotation, because the reactor historically ran the whole
	// in-process pass before the external chain. No in-process filter registers
	// at this stage; the reactor binds the per-peer chain as a single ordered
	// step here so the cross-system order is a declared property, not code
	// position.
	FilterStagePeerChain int = 300
)

// AttrOp describes a single attribute modification operation.
// Egress filters accumulate AttrOps in the ModAccumulator via Op().
// Multiple AttrOps with the same Code are allowed and are passed together
// to the registered handler during the progressive build.
type AttrOp struct {
	Code   uint8  // Attribute type code (e.g., 35 for OTC, 8 for COMMUNITY)
	Action uint8  // AttrModSet, AttrModAdd, AttrModRemove, AttrModPrepend
	Buf    []byte // Pre-built wire bytes of the VALUE (handler writes the header)

	// GenIdx names this operation's generator: 0 for none, otherwise the
	// one-based index into ModAccumulator.Gens. It is the alternative to Buf,
	// never a companion: a handler that names this operation with AttrPlan.Op
	// takes the generator when one is set and Buf otherwise.
	//
	// A generator exists for a value that cannot be pre-built without staging it
	// first. An AS_PATH re-encoded to the destination's ASN width is the case
	// that needs it: every AS number changes width, so no fragment list over the
	// source can express the result, and building it into a scratch slice to hand
	// over as Buf would copy it twice -- once into the scratch, once into the
	// destination. A generator answers its exact size during the plan and writes
	// its bytes straight into the destination buffer (ai/rules/performance.md).
	//
	// It is an INDEX rather than the interface value itself, and that is
	// load-bearing rather than tidy. An interface field costs two words and grew
	// this struct from 32 to 48 bytes, which pushed buildModifiedPayload past an
	// inlining budget and made its span index escape to the heap -- one
	// allocation per destination, on the exact path the exactly-sized rebuild
	// exists to keep allocation-free (TestModifyPathZeroAlloc). A one-byte index
	// lands in the padding Code and Action already leave, so the struct stays 32
	// bytes and the escape analysis is unchanged.
	GenIdx uint8
}

// AttrGenerator writes an attribute VALUE whose bytes exist in no buffer yet.
//
// It is the size-then-write pair the exactly-sized rebuild is built on: GenLen
// answers during the plan, before any buffer is acquired, and GenWrite fills the
// destination during the write.
//
// CALLER OBLIGATION. A generator MUST be immutable for the whole rebuild.
// GenLen is called during planning and its answer is folded into the attribute's
// declared length, so a generator whose length changes afterwards produces an
// attribute whose header contradicts its contents. The rebuild checks the write
// against the plan and suppresses the route rather than emitting one, but the
// cheaper fix is not to mutate a generator that has been recorded.
//
// GenWrite MUST write exactly GenLen bytes at off and return that count, and
// MUST NOT write outside buf[off:off+GenLen()].
type AttrGenerator interface {
	// GenLen returns the exact number of value bytes GenWrite will write.
	GenLen() int
	// GenWrite writes the value into buf at off and returns the bytes written.
	GenWrite(buf []byte, off int) int
}

// AttrModHandler is a per-attribute-code handler for the rebuild.
//
// It PLANS one attribute and writes nothing. It reads the source attribute
// (p.Source(), p.Value(), nil when the attribute is absent) and the operations
// for its code (p.Ops()), appends fragments with Keep, Op and New, and finishes
// with exactly one of Emit, EmitExtended, Drop or Fail.
//
// The plan carries the exact output length, so "how many bytes will you write"
// is answered before any buffer is acquired and cannot drift from what is then
// written. That is what removes the slack sizing the rebuild used to need, and
// with it the branch that abandoned every modification on overflow and forwarded
// the route unchanged (ai/rules/evidence.md).
//
// A handler that returns without finishing its plan is treated as a refusal:
// silence is not consent when the outcome is a route on the wire.
//
// MUST NOT retain p beyond the call. One planner value serves every attribute of
// every destination.
type AttrModHandler func(p *AttrPlan)

// GroupedOps sorts the accumulated operations by attribute code and returns
// them, so each code's operations occupy one contiguous run.
//
// The sort is stable and in place: it preserves the order producers recorded
// operations in, which the community handler relies on (remove, then add, then
// set), and it allocates nothing. It replaces a grouping that returned a
// 256-entry array of slices BY VALUE and heap-allocated one slice per touched
// code, on every destination of every fan-out.
//
// Insertion sort rather than sort.Slice: the operation count is a handful, and
// sort.Slice would allocate a reflect-based swapper.
func (a *ModAccumulator) GroupedOps() []AttrOp {
	if a.grouped {
		return a.ops
	}
	a.grouped = true
	for i := 1; i < len(a.ops); i++ {
		op := a.ops[i]
		j := i - 1
		for j >= 0 && a.ops[j].Code > op.Code {
			a.ops[j+1] = a.ops[j]
			j--
		}
		a.ops[j+1] = op
	}
	return a.ops
}

// Filter describes one plugin's contribution to the BGP route filter
// pipeline: an optional ingress filter, an optional egress filter, and the
// (stage, priority) pair ordering both within their chains. Name is the
// registering plugin's name and breaks ordering ties.
type Filter struct {
	Name     string
	Stage    int               // Coarse ordering class (FilterStageProtocol/Policy/Annotation)
	Priority int               // Fine ordering within a stage; equal priority sorted by name
	Ingress  IngressFilterFunc // nil = no ingress filtering
	Egress   EgressFilterFunc  // nil = no egress filtering
	// Readvertise opts an egress filter into the RIB stale-readvertise announce
	// rail (AnnounceNLRIBatch) in addition to the ForwardUpdate rail. It exists
	// for the LLGR egress filter (RFC 9494): when a peer's GR window expires its
	// routes are readvertised via AnnounceNLRIBatch, which runs no general egress
	// chain, so only filters that key off route metadata (meta["stale"]) and must
	// re-decide per destination peer opt in here. The reactor runs ONLY the
	// Readvertise-opted egress funcs on stale batches, never the full chain, so a
	// readvertise does not re-apply OTC/community/policy that already ran at the
	// original announce. Requires Egress != nil.
	Readvertise bool
}

var (
	// ErrEmptyFilterName is returned when registering a filter with an empty name.
	ErrEmptyFilterName = errors.New("filterapi: filter name is empty")
	// ErrNoFilterFunc is returned when a filter has neither ingress nor egress func.
	ErrNoFilterFunc = errors.New("filterapi: filter declares neither ingress nor egress function")
	// ErrDuplicateFilterName is returned when a filter name is already registered.
	ErrDuplicateFilterName = errors.New("filterapi: duplicate filter name")
)

var (
	mu sync.RWMutex

	// filters maps plugin name to its pipeline contribution.
	// Populated at init() time by plugins, read at runtime by the reactor.
	filters = make(map[string]Filter)

	// attrModHandlers stores registered attr mod handlers keyed by attribute code.
	// Populated at init() time by plugins, read at runtime by the reactor.
	attrModHandlers = make(map[uint8]AttrModHandler)

	// rsForwarding reports whether a plugin owns and has activated the reactor's
	// route-server (RS) fast-path forwarding. It stays false unless a plugin
	// calls EnableRSForwarding() from its init(), so a binary that does not link
	// the rs plugin (delete-the-folder) leaves the reactor fast path inert.
	// Set at init() time by the rs plugin, read once at reactor construction.
	rsForwarding bool
)

// EnableRSForwarding activates the reactor's route-server fast-path forwarding
// capability. The rs plugin calls it from init() so that removing the plugin
// (deleting its package) removes the only activation, leaving the reactor's RS
// fast path inert. Must be called from init() functions only.
func EnableRSForwarding() {
	mu.Lock()
	defer mu.Unlock()
	rsForwarding = true
}

// RSForwardingEnabled reports whether the RS fast-path forwarding capability has
// been activated by a plugin. The reactor reads it once at construction and
// caches the result; the per-UPDATE gate then checks the cached bool.
func RSForwardingEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return rsForwarding
}

// Register adds a plugin's filter pipeline contribution.
// Must be called from init() functions only.
// Returns an error on empty name, missing functions, or duplicate name.
func Register(f Filter) error {
	if f.Name == "" {
		return ErrEmptyFilterName
	}
	if f.Ingress == nil && f.Egress == nil {
		return fmt.Errorf("%w: %q", ErrNoFilterFunc, f.Name)
	}

	mu.Lock()
	defer mu.Unlock()
	if _, exists := filters[f.Name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateFilterName, f.Name)
	}
	filters[f.Name] = f
	return nil
}

// sortedFilters returns the filters selected by hasFunc, sorted by stage,
// then priority, then name. Caller MUST hold mu (read or write).
func sortedFilters(hasFunc func(Filter) bool) []Filter {
	var entries []Filter
	for _, f := range filters {
		if hasFunc(f) {
			entries = append(entries, f)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return LessOrder(
			entries[i].Name, entries[i].Stage, entries[i].Priority,
			entries[j].Name, entries[j].Stage, entries[j].Priority,
		)
	})
	return entries
}

// LessOrder reports whether a pipeline entry keyed (nameA, stageA, priorityA)
// sorts before one keyed (nameB, stageB, priorityB): stage first, then priority,
// then name. It is the single source of truth for filter-pipeline ordering,
// exported so a consumer (the reactor) can merge-sort a non-registered step -- its
// per-peer policy chain at FilterStagePeerChain -- into the same order as the
// registered filters without duplicating the sort key.
func LessOrder(nameA string, stageA, priorityA int, nameB string, stageB, priorityB int) bool {
	if stageA != stageB {
		return stageA < stageB
	}
	if priorityA != priorityB {
		return priorityA < priorityB
	}
	return nameA < nameB
}

// IngressOrdered returns all registered filters that declare an ingress function,
// sorted by stage, then priority, then name -- the same order as IngressFilters,
// but carrying each filter's ordering keys (Name/Stage/Priority) alongside the
// func so a consumer can merge-sort additional externally-owned steps (the
// reactor's per-peer policy chain at FilterStagePeerChain) into the same order.
func IngressOrdered() []Filter {
	mu.RLock()
	defer mu.RUnlock()
	return sortedFilters(func(f Filter) bool { return f.Ingress != nil })
}

// EgressOrdered is the egress twin of IngressOrdered.
func EgressOrdered() []Filter {
	mu.RLock()
	defer mu.RUnlock()
	return sortedFilters(func(f Filter) bool { return f.Egress != nil })
}

// ReadvertiseEgressFuncs returns the egress filter funcs that opted into the RIB
// stale-readvertise rail (Readvertise == true), in pipeline order. The reactor
// caches these once at construction and runs only them on a stale AnnounceNLRIBatch
// (RFC 9494 LLGR), leaving the common announce path filter-free.
func ReadvertiseEgressFuncs() []EgressFilterFunc {
	mu.RLock()
	defer mu.RUnlock()
	entries := sortedFilters(func(f Filter) bool { return f.Egress != nil && f.Readvertise })
	funcs := make([]EgressFilterFunc, 0, len(entries))
	for _, e := range entries {
		funcs = append(funcs, e.Egress)
	}
	return funcs
}

// IngressFilters returns all registered ingress filter functions,
// sorted by stage, then priority, then by plugin name.
// Called by the reactor to build the ingress filter chain.
func IngressFilters() []IngressFilterFunc {
	mu.RLock()
	defer mu.RUnlock()

	entries := sortedFilters(func(f Filter) bool { return f.Ingress != nil })
	chain := make([]IngressFilterFunc, 0, len(entries))
	for _, e := range entries {
		chain = append(chain, e.Ingress)
	}
	return chain
}

// ingressFilterNames returns the names of plugins with ingress filters,
// in execution order (sorted by stage, then priority, then name).
func ingressFilterNames() []string {
	mu.RLock()
	defer mu.RUnlock()

	entries := sortedFilters(func(f Filter) bool { return f.Ingress != nil })
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names
}

// EgressFilters returns all registered egress filter functions,
// sorted by stage, then priority, then by plugin name.
// Called by the reactor to build the egress filter chain.
func EgressFilters() []EgressFilterFunc {
	mu.RLock()
	defer mu.RUnlock()

	entries := sortedFilters(func(f Filter) bool { return f.Egress != nil })
	chain := make([]EgressFilterFunc, 0, len(entries))
	for _, e := range entries {
		chain = append(chain, e.Egress)
	}
	return chain
}

// egressFilterNames returns the names of plugins with egress filters,
// in execution order (sorted by stage, then priority, then name).
func egressFilterNames() []string {
	mu.RLock()
	defer mu.RUnlock()

	entries := sortedFilters(func(f Filter) bool { return f.Egress != nil })
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names
}

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

// unregisterAttrModHandler removes an attr mod handler. Only for use in tests.
func unregisterAttrModHandler(code uint8) {
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

// PipelineSnapshot holds a copy of the filter pipeline state for test save/restore.
type PipelineSnapshot struct {
	filters         map[string]Filter
	attrModHandlers map[uint8]AttrModHandler
	rsForwarding    bool
}

// Snapshot returns a copy of the current pipeline state. Only for use in tests.
// Use with Restore to safely reset and restore after test-specific registrations.
func Snapshot() PipelineSnapshot {
	mu.RLock()
	defer mu.RUnlock()

	fs := make(map[string]Filter, len(filters))
	maps.Copy(fs, filters)
	ah := make(map[uint8]AttrModHandler, len(attrModHandlers))
	maps.Copy(ah, attrModHandlers)
	return PipelineSnapshot{filters: fs, attrModHandlers: ah, rsForwarding: rsForwarding}
}

// Restore replaces the pipeline state with a previously saved snapshot.
// Only for use in tests.
func Restore(snap PipelineSnapshot) {
	mu.Lock()
	defer mu.Unlock()
	filters = snap.filters
	attrModHandlers = snap.attrModHandlers
	rsForwarding = snap.rsForwarding
}

// ResetForTest clears all registered filters, attr mod handlers, and the
// RS-forwarding capability flag. Only for use in tests, paired with Snapshot/Restore.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	filters = make(map[string]Filter)
	attrModHandlers = make(map[uint8]AttrModHandler)
	rsForwarding = false
}
