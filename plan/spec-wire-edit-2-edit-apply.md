# Spec: wire-edit-2-edit-apply -- the edit set, fragment values, exact sizing, one-pass writer

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | `plan/spec-wire-edit-1-base-index.md` |
| Phase | 2 |
| Deferral shard | `plan/deferrals/spec-wire-edit-2-edit-apply.md` |
| Updated | 2026-08-01 |

Child 2 of `plan/spec-wire-edit-0-umbrella.md`. It turns the existing
accumulator into the full edit set and replaces the progressive build with a
single exactly-sized merge writer.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ModAccumulator` (`internal/component/bgp/filterapi/filterapi.go:98`) already
records "what to change" and applies it later. What it cannot express is *how* a
new attribute value relates to the old one, so every handler that keeps most of
an attribute still rebuilds the whole value in an intermediate buffer.

Three consequences, each visible in code today:

| Consequence | Evidence |
|-------------|----------|
| An intermediate copy per touched attribute | `genericCommunityHandler` copies the whole list with `data = make([]byte, len(val))` at `internal/component/bgp/plugins/filter_community/handler.go:69`, grows it with `data = append(data, op.Buf...)` at `internal/component/bgp/plugins/filter_community/handler.go:107`, and may replace it wholesale with `data = make([]byte, len(op.Buf))` at `internal/component/bgp/plugins/filter_community/handler.go:112`; `removeValues` appends into a fresh slice with `result = append(result, data[i:i+valueSize]...)` at `internal/component/bgp/plugins/filter_community/handler.go:172` |
| No exact size is ever known, so the buffer is over-sized and overflow is handled by abandoning every modification | `needSize` is `len(payload) + 256` at `internal/component/bgp/reactor/forward_build.go:104`, and the `overflow` branch returns nil at `internal/component/bgp/reactor/forward_build.go:236`, which the caller reads as "nothing to do" and forwards the route unmodified |
| New attributes are appended after every source attribute rather than merge-inserted | `internal/component/bgp/reactor/forward_build.go:242` |

One handler already solves the first problem by hand.
`mpReachNextHopHandler` (`internal/component/bgp/reactor/filter_delta_handlers.go:241`)
writes the AFI and SAFI from the source value, then the new next-hop length and
next-hop, then the reserved byte and the whole NLRI tail straight from the source
(`internal/component/bgp/reactor/filter_delta_handlers.go:358`,
`internal/component/bgp/reactor/filter_delta_handlers.go:362`,
`internal/component/bgp/reactor/filter_delta_handlers.go:365`). That is a
fragment list, written out longhand because the accumulator has no word for it.

**Goal.** Give the accumulator that word.

- A **slot** per touched attribute code, of one of three kinds: fragments, delete, or generate.
- A **fragment** naming a source (the base payload or the accumulator's arena), an offset and a length.
- An **arena** holding only bytes that did not already exist in the base.
- An exact **size delta**, maintained per edit, so the output size is known before a buffer is acquired.
- A handler contract that answers "how many bytes will you write" before it writes.

With an exact size, `buildModifiedPayload`
(`internal/component/bgp/reactor/forward_build.go:58`) becomes a single merge
walk into a buffer of exactly the right size, the abandon-on-overflow branch
disappears, and an edit that genuinely cannot fit becomes a logged suppression
rather than a silent unmodified forward.

**Non-goal.** The AS_PATH family stays outside this child. Folding it in is
`plan/spec-wire-edit-3-aspath-fold.md`. The generate slot kind is defined here
because the writer must know about it, but the only resolver registered in this
child is the existing AS_PATH handler behaviour, unchanged.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `ai/rules/buffer-first.md` - all wire encoding writes into pooled bounded buffers
  → Constraint: `append(buf, ...)`, `make([]byte, N)` inside helpers, and `buildFoo() ([]byte, error)` shapes are banned in encoding code. The writer must be a write-at-offset into a caller buffer, preceded by an exact size query.
- [ ] `docs/architecture/wire/messages.md` - UPDATE body layout
  → Constraint: the NLRI section length is implicit, so any change to the attribute length shifts where NLRI starts. The size query must account for the attribute-length field itself, not just the attributes.
- [ ] `docs/architecture/wire/attributes.md` - path attribute encoding
  → Constraint: the header is 3 bytes, or 4 with the Extended Length flag. A value crossing 255 bytes changes the header size class and therefore the total by one byte, so the size query must decide the class, not guess it.
- [ ] `docs/architecture/wire/update-packing.md` - how attributes are ordered on emission
  → Decision: merge-insert a new attribute at the first base position whose code exceeds it. Untouched base attributes keep base order, so a pure forward stays byte-identical.
- [ ] `docs/architecture/memory/lifetime-contracts.md` - buffer lifetime contracts
  → Constraint: an edit set holds base offsets, so it inherits the base's boundary and must never outlive it. A per-peer pool buffer has exactly one return point.
- [ ] `ai/rules/fail-closed-guards.md` - a guard must fail closed or say something
  → Constraint: the overflow path at `internal/component/bgp/reactor/forward_build.go:236` returns nil, and the caller at `internal/component/bgp/reactor/reactor_api_forward.go:793` reads nil as "no modification needed" and forwards the original. That is fail-open. Exact sizing removes the branch; what remains must suppress and speak.
- [ ] `ai/rules/plugin-design.md` - plugin-facing contracts
  → Constraint: `AttrModHandler` (`internal/component/bgp/filterapi/filterapi.go:253`) is registered by plugins via `RegisterAttrModHandler` (`internal/component/bgp/filterapi/filterapi.go:471`). Changing its shape is a plugin-facing break and must be documented as one.
- [ ] `ai/rules/design-principles.md` - abstraction gate, single responsibility
  → Decision: the gate is cleared. Four call sites reach `buildModifiedPayload` today, and one handler already implements fragments by hand.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE format, path attributes
  → Constraint: Section 5 orders attribute type codes ascending on emission, which is what merge-insert preserves. Section 4.3 caps an attribute value at 65535 octets.
- [ ] `rfc/short/rfc4760.md` - multiprotocol NLRI
  → Constraint: the MP_REACH value is AFI(2) + SAFI(1) + next-hop-length(1) + next-hop + reserved(1) + NLRI. A next-hop rewrite is a sub-value edit whose NLRI tail must be copied exactly once.
- [ ] `rfc/short/rfc4456.md` - route reflection
  → Constraint: Section 8 requires ORIGINATOR_ID set-if-absent and CLUSTER_LIST prepend. Both are new attributes on an iBGP forward, so both exercise merge-insert.
- [ ] `rfc/short/rfc1997.md`, `rfc/short/rfc4360.md`, `rfc/short/rfc8092.md` - community, extended community, large community
  → Constraint: each value is a list of fixed-width entries (4, 8, 12 octets). Removing a subset yields a subsequence of the original bytes, so it needs no new bytes at all and is expressible purely as base fragments.
- [ ] `rfc/short/rfc8654.md` - extended message
  → Constraint: the body is capped at 65516 octets, so the size delta is bounded and an exact size always fits a 16-bit field.
- [ ] `rfc/short/rfc9234.md` - Only to Customer
  → Constraint: OTC (code 35) is stamped per destination by the role plugin's registered handler (`internal/component/bgp/plugins/role/register.go:21`), so that handler migrates with the contract.

**Key insights:** (minimal context to resume after compaction)
- The deferred-edit idea already exists as `ModAccumulator`. This child does not introduce it, it completes it.
- `mpReachNextHopHandler` is the existence proof for fragments: it writes base bytes, new bytes, then base bytes, starting at `copy(buf[w:], val[:3])` (`internal/component/bgp/reactor/filter_delta_handlers.go:358`).
- The community handler pays three heap allocations and three copies per attribute per destination for the absence of that vocabulary.
- Today `mods` is declared **inside** the destination loop at `internal/component/bgp/reactor/reactor_api_forward.go:632`, so Go zeroes it once per peer. A larger inline value must be hoisted above the loop or this change makes the hot path slower.
- `groupOpsByCode` (`internal/component/bgp/reactor/forward_build.go:538`) returns a 256-entry array of slices **by value** and heap-allocates one slice per touched code. Slots replace it outright.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/filterapi/filterapi.go:98` - `ModAccumulator`: an ops slice, a withdraw flag, an announce-NLRI rewrite, a withdrawn rewrite, and a 64-byte inline arena whose size is `modAccumulatorInlineBytes` at `internal/component/bgp/filterapi/filterapi.go:94`.
- [ ] `internal/component/bgp/filterapi/filterapi.go:115` - `HasModifications`: true when any op, NLRI rewrite or withdrawn rewrite exists; the withdraw conversion is deliberately not counted.
- [ ] `internal/component/bgp/filterapi/filterapi.go:119` - `Reset`: truncates the ops slice and clears four scalars.
- [ ] `internal/component/bgp/filterapi/filterapi.go:153` - `Op`: appends an `AttrOp`; its doc block carries the unwritten arity contract for list-valued attributes that a live defect already cost.
- [ ] `internal/component/bgp/filterapi/filterapi.go:160` - `OpCopy`: copies into the inline arena when it fits, and heap-allocates with `copied := make([]byte, len(buf))` at `internal/component/bgp/filterapi/filterapi.go:172` when it does not.
- [ ] `internal/component/bgp/filterapi/filterapi.go:212` - the five actions: Set, Add, Remove, Prepend, Suppress.
- [ ] `internal/component/bgp/filterapi/filterapi.go:241` - `AttrOp`: code, action, and a pre-built value buffer; the handler writes the header.
- [ ] `internal/component/bgp/filterapi/filterapi.go:253` - `AttrModHandler`: source bytes, ops, output buffer, offset, returns the new offset. There is no size query and no way to refuse.
- [ ] `internal/component/bgp/filterapi/filterapi.go:471` - `RegisterAttrModHandler`, plus `AttrModHandlerFor` at `internal/component/bgp/filterapi/filterapi.go:488`: the registry the reactor consults per code.
- [ ] `internal/component/bgp/reactor/forward_build.go:58` - `buildModifiedPayload`: the whole progressive build. It groups ops with `groupOpsByCode(ops)` at `internal/component/bgp/reactor/forward_build.go:79`, sizes with `len(payload) + 256` at `internal/component/bgp/reactor/forward_build.go:104`, acquires a buffer through `acquireModBuf` at `internal/component/bgp/reactor/forward_build.go:105`, walks source attributes one at a time from `for srcOff < attrEnd` at `internal/component/bgp/reactor/forward_build.go:171`, abandons every modification on `overflow` at `internal/component/bgp/reactor/forward_build.go:236`, appends unconsumed new attributes from `for codeInt := range opsByCode` at `internal/component/bgp/reactor/forward_build.go:242`, backfills `newAttrLen` at `internal/component/bgp/reactor/forward_build.go:262`, and copies out with `make([]byte, off)` at `internal/component/bgp/reactor/forward_build.go:307` when no per-peer buffer was available.
- [ ] `internal/component/bgp/reactor/forward_build.go:392` - `acquireModBuf`: per-peer pool, then the shared pool, then a bare allocation for an oversized payload.
- [ ] `internal/component/bgp/reactor/forward_build.go:528` - `safeCopy`, plus `safeAttrModHandler` at `internal/component/bgp/reactor/forward_build.go:560`: the bounds and panic guards that exist because handlers can neither size nor refuse.
- [ ] `internal/component/bgp/reactor/forward_build.go:538` - `groupOpsByCode`: returns `[256][]filterapi.AttrOp` by value and allocates one slice per touched code.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go:20` - `genericAttrSetHandler`, covering the codes listed in `genericAttrCodes` at `internal/component/bgp/reactor/filter_delta_handlers.go:82`.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go:103` - `originatorIDHandler`, plus `clusterListHandler` at `internal/component/bgp/reactor/filter_delta_handlers.go:136`: the two RFC 4456 injections.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go:241` - `mpReachNextHopHandler`: parses the source header, recomputes `newValLen := srcValLen - nhLen + newNHLen` at `internal/component/bgp/reactor/filter_delta_handlers.go:317`, chooses the output header class on `newValLen > 255` at `internal/component/bgp/reactor/filter_delta_handlers.go:335`, then writes `copy(buf[w:], val[:3])` at `internal/component/bgp/reactor/filter_delta_handlers.go:358`, `copy(buf[w:], setBuf)` at `internal/component/bgp/reactor/filter_delta_handlers.go:362` and `copy(buf[w:], val[nhEnd:])` at `internal/component/bgp/reactor/filter_delta_handlers.go:365`.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go:468` - `attrModHandlersWithDefaults`: the map the reactor builds from the registry plus its internal handlers.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go:64` - `genericCommunityHandler`: copies the source value with `data = make([]byte, len(val))` at `internal/component/bgp/plugins/filter_community/handler.go:69`, applies Remove then Add then Set, appends with `data = append(data, op.Buf...)` at `internal/component/bgp/plugins/filter_community/handler.go:107`, reallocates on Set with `data = make([]byte, len(op.Buf))` at `internal/component/bgp/plugins/filter_community/handler.go:112`, and copies the result out with `copy(buf[off+4:], data)` at `internal/component/bgp/plugins/filter_community/handler.go:136`.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go:165` - `removeValues`: refuses a buffer that is not a whole number of values, and otherwise appends every retained value with `result = append(result, data[i:i+valueSize]...)` at `internal/component/bgp/plugins/filter_community/handler.go:172`.
- [ ] `internal/component/bgp/plugins/role/otc.go:739` - `otcAttrModHandler`: the fourth registered handler, registered at `internal/component/bgp/plugins/role/register.go:21`.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go:228` - `applyFactsNextHop`, which emits `mods.Op(40, filterapi.AttrModSuppress, nil)` at `internal/component/bgp/reactor/peer_forward_facts.go:243`, plus `applyFactsSendCommunity` at `internal/component/bgp/reactor/peer_forward_facts.go:246`, which emits `mods.Op(8, filterapi.AttrModSuppress, nil)` at `internal/component/bgp/reactor/peer_forward_facts.go:251`, `mods.Op(16, filterapi.AttrModSuppress, nil)` at `internal/component/bgp/reactor/peer_forward_facts.go:254` and `mods.Op(32, filterapi.AttrModSuppress, nil)` at `internal/component/bgp/reactor/peer_forward_facts.go:257`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:632` - `mods` declared inside the destination loop; the route-server community strip at `internal/component/bgp/reactor/reactor_api_forward.go:643`; the RFC 4456 injections at `internal/component/bgp/reactor/reactor_api_forward.go:691`; the facts-driven next-hop and community steps at `internal/component/bgp/reactor/reactor_api_forward.go:695`; the modify call at `internal/component/bgp/reactor/reactor_api_forward.go:793`.
- [ ] `internal/component/bgp/reactor/forward_rs.go:347` - the route-server rail's own community strip op, the second producer of the same edit.
- [ ] `internal/component/bgp/reactor/filter_ordered.go:203` - the ingress policy chain's `buildModifiedPayload` call, and `internal/component/bgp/reactor/filter_ordered.go:303` - the egress chain's.

**Behavior to preserve:**
- Every transform currently applied, with identical wire output: next-hop modes, community strip, tag and suppress, ORIGINATOR_ID and CLUSTER_LIST injection, OTC stamping, PrefixSID suppression, and the announce-to-withdrawal conversion.
- The list-valued arity contract documented at `internal/component/bgp/filterapi/filterapi.go:153`: a Remove buffer is a whole number of wire values, and a buffer that is not is refused loudly, counted, and leaves the attribute's other operations intact.
- `Op`, `OpCopy`, `SetWithdraw`, `IsWithdraw`, `SetNLRIRewrite`, `NLRIRewrite`, `SetWithdrawnRewrite`, `WithdrawnRewrite`, `Len` and `HasModifications` keep their names and semantics, so in-process filter plugins compile unchanged.
- `RegisterAttrModHandler` keeps its registry shape and its per-code lookup.
- The panic guard around handler invocation: a handler that panics must not take the daemon down.
- Same-context zero-copy forwarding for an UPDATE with no modifications: no edit set is built and no byte is copied.
- Every existing expectation under `test/plugin/`, `test/policy/` and `test/encode/`.

**Behavior to change:**
- `AttrModHandler` gains an exact size query. This is a plugin-facing break affecting the four registered handlers (three community codes plus OTC) and the internal generic handlers.
- New attributes are merge-inserted at their ascending type-code position instead of appended after all source attributes.
- An oversize modification becomes an explicit, logged, counted suppression for that destination instead of a silent unmodified forward.
- `groupOpsByCode` and its per-code slice allocation are removed in favour of slots.
- The accumulator is hoisted above the destination loop and reset in constant time.
- The community handler produces fragments over the base value instead of copying the list.

## Data Flow (MANDATORY)

### Entry Point
- A received UPDATE that has passed `enforceRFC7606` and been published as an immutable base by `plan/spec-wire-edit-1-base-index.md`.
- A destination peer whose forward facts, export policy chain, and in-process egress filters describe what must change for that peer.

### Transformation Path
1. The destination loop (`internal/component/bgp/reactor/reactor_api_forward.go:604`) resets the hoisted edit set for this peer.
2. Producers record intent: the route-server strip (`internal/component/bgp/reactor/reactor_api_forward.go:643`), the RFC 4456 injections (`internal/component/bgp/reactor/reactor_api_forward.go:691`), the facts-driven next-hop and send-community steps (`internal/component/bgp/reactor/reactor_api_forward.go:695`), and the ordered egress filter chain.
3. Each producer's op resolves to a slot. A slot's value is a fragment list over the base and the arena, a delete, or a generate resolver.
4. The size query runs: sum the untouched base attributes, plus each slot's exact contribution, plus the header size class each slot's final length implies. This yields the exact output length before any buffer is acquired.
5. A buffer of exactly that size is acquired from the per-peer pool (`internal/component/bgp/reactor/forward_build.go:392`).
6. One merge walk writes the output: untouched base attributes copied in base order, touched ones written by their slot, new ones merge-inserted at their ascending position.
7. The withdrawn and NLRI sections are written, honouring the accumulator's rewrites.
8. The result is dispatched to the forward pool; the worker writes it through `writeRawUpdateBody` (`internal/component/bgp/reactor/session_write.go:319`) and the buffer is returned once.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to registered attribute handler | the handler receives base value bytes plus its ops, answers a size, then writes fragments at an offset | No |
| Edit set to base | fragments hold base offsets, never base bytes; the set must not outlive the base | No |
| Forward loop to forward-pool worker | the exactly-sized buffer is handed over by index and returned after the TCP write | No |
| Reactor to external filter plugin | unchanged by this child; a terminal raw override still replaces the base | No |

### Integration Points
- `filterapi.ModAccumulator` is the public face of the edit set, so in-process filter plugins keep compiling.
- `filterapi.RegisterAttrModHandler` keeps its shape; only the handler signature changes.
- `buildModifiedPayload` keeps its position in the call graph, so the ingress call at `internal/component/bgp/reactor/filter_ordered.go:203` and the egress call at `internal/component/bgp/reactor/filter_ordered.go:303` stay call-compatible.
- `wireu.SplitWireUpdate` is unchanged and still consumes materialised output.
- The immutable base and span index from `plan/spec-wire-edit-1-base-index.md` are what make the size query cheap: the merge walk reads spans, not bytes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every edit the engine applies today is expressible as a fragment list, a delete, or a generate resolver. | Census of every `Op` and `OpCopy` producer; `mpReachNextHopHandler` (`internal/component/bgp/reactor/filter_delta_handlers.go:241`) is already a hand-written fragment list. | The inexpressible edit keeps the terminal raw-override escape hatch external filters already use, `decodeFilterRawOverride` at `internal/component/bgp/reactor/filter_ordered.go:289`. | A per-producer mapping table, one row per call site, with one test per row. | unvalidated |
| A-2 | Changing the `AttrModHandler` signature affects only four registered handlers plus the internal generics. | `RegisterAttrModHandler` call sites are `internal/component/bgp/plugins/role/register.go:21` and `internal/component/bgp/plugins/filter_community/register.go:15`; the generic handlers are internal, assembled by `attrModHandlersWithDefaults` at `internal/component/bgp/reactor/filter_delta_handlers.go:468`. | A wider but still mechanical migration. | Grep for `RegisterAttrModHandler` and `AttrModHandler` across the tree as the first implementation action. | unvalidated |
| A-3 | 12 slots, 24 fragments and a 128-byte arena cover the realistic worst case without a heap spill. | Static census: ORIGINATOR_ID(9), CLUSTER_LIST(10), NEXT_HOP(3), MP_REACH(14), COMMUNITY(8), EXTENDED(16), LARGE(32), OTC(35), PrefixSID(40), AS_PATH(2), AS4_PATH(17). Common case is five. | Raise the constants; the structure is unchanged. | An instrumented run over a route-server fan-out recording the per-destination distribution of slots, fragments and arena bytes. | unvalidated |
| A-4 | Hoisting the accumulator above the destination loop and resetting only used prefixes keeps the hot path at least as fast as today. | Today `var mods filterapi.ModAccumulator` is declared inside the loop at `internal/component/bgp/reactor/reactor_api_forward.go:632`, so Go zeroes roughly 150 bytes per peer; the new value is roughly 500. | If reset dominates, shrink the inline capacities and accept a spill. | `BenchmarkForwardModifiedPerDestination` at fan-out 1, 10 and 100, plus a reset microbenchmark whose cost must not scale with inline capacity. | unvalidated |
| A-7 | The two preconditions this child depends on (the hoist, and the modify-failure signal) are already in place when it starts, because `plan/spec-hotpath-alloc-round-4.md` Tier 1 landed first. | That spec owns T1-3 (hoist both rails, wire the existing `Reset` at `internal/component/bgp/filterapi/filterapi.go:119`, which has no production caller today) and T1-1 (split "could not modify" from "nothing to modify", which the caller currently conflates on the `buildModifiedPayload` line at `internal/component/bgp/reactor/reactor_api_forward.go:793`). | If hotpath Tier 1 has NOT landed, this child must do both itself and its benchmark baseline is wrong: measuring a hoisted-and-enlarged accumulator against a per-iteration small one attributes the hoist's win to the fragment model. | Read `plan/spec-hotpath-alloc-round-4.md` status and grep for `mods.Reset()` in `internal/component/bgp/reactor/` as the first implementation action. | unvalidated |
| A-5 | The size query can be made exactly equal to the bytes written, for every slot kind and every combination. | Every kind has a computable length: fragments sum, delete is zero, generate has a paired size function. The only subtlety is the header size class at the 255-to-256 boundary. | The writer keeps a bounds check and the mismatch becomes a hard test failure rather than a silent truncation. | `TestEditSizeIsExact` over a generated corpus covering both header classes. | unvalidated |
| A-6 | Oversize modifications are rare enough that turning a silent unmodified forward into a suppression will not drop production routes. | The current slack is 256 bytes over the payload (`internal/component/bgp/reactor/forward_build.go:104`), and the abandon branch has not been reported firing. | The counter lands first and the behaviour change waits until the observed rate is known. | The suppression counter, deployed and observed, before the behaviour flips. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A wire-output regression is invisible to unit tests and only shows against a real peer. | Interop scenario diff against FRR or BIRD. | The golden byte-comparison harness from the umbrella's Phase 1 runs over a corpus times a transform matrix, and must stay byte-identical for every existing transform before any old code is deleted. |
| R-2 | Merge-insert changes byte output for a received UPDATE whose attributes are not already ascending. | Golden corpus diff on the pure-forward path. | Merge-insert applies only when a new attribute is actually added. An UPDATE with no added attribute keeps base order unconditionally, so the zero-copy identity survives. |
| R-3 | The handler contract change breaks an out-of-tree plugin that registers a handler. | Compile fanout the moment the signature changes. | The break is documented in the plugin rules and the guide, and the old handler shape is not silently accepted. |
| R-4 | A bigger inline accumulator makes the hot path slower rather than faster. | The reset microbenchmark. | Reset clears only used prefixes: slot length, fragment length, arena offset, touched bitset, delta. It never clears the inline arrays. A-4 is the gate. |
| R-5 | Exact sizing surfaces latent oversize cases as new route drops. | The new suppression counter firing in soak. | Land the counter before the behaviour change, so the frequency is known first. |
| R-6 | The community handler's arity refusal is subtle and easy to lose in a rewrite. | The existing multi-value strip tests. | The refusal, its warning and its counter are moved verbatim into the fragment producer, and `test/plugin/bgp-rs-community-strip-multi.ci` must keep passing. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes are mis-encoded on the wire: duplicated or missing attributes, a corrupted MP_REACH next-hop, a truncated community list. A peer may reset the session under RFC 7606, or install wrong forwarding state silently. |
| How is it reverted? | Single commit revert, as long as the golden byte-comparison harness is in place. Once a peer has accepted mis-encoded routes the wire effect is not revertible from our side. |
| Who else touches this path? | `plan/spec-hotpath-alloc-round-4.md` Tier 1 lands BEFORE this child and owns two of its preconditions (see A-7): T1-3 hoists the accumulator on both rails, T1-1 splits the modify-failure signal from the no-op signal. This child consumes both and must not re-implement either. `plan/spec-wire-edit-3-aspath-fold.md` adds a generate slot here; `plan/spec-filter-wire-0-umbrella.md` owns the filter IPC over the same apply engine; `plan/spec-perf-next-2-filter-delta-alloc.md` has a deferred phase on the same encoders. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer sends an UPDATE to an eBGP peer configured with next-hop-self and a community tag | → | one edit set, one size query, one materialisation into a per-peer buffer | `test/plugin/wire-edit-single-materialise.ci` |
| A route reflector forwards an iBGP route to a client | → | ORIGINATOR_ID and CLUSTER_LIST slots merge-inserted at their ascending positions | `test/plugin/wire-edit-rr-attr-order.ci` |
| An export policy produces a modification that cannot fit the destination's maximum message size | → | the size query fails, the route is suppressed for that destination, the counter increments | `test/plugin/wire-edit-oversize-suppress.ci` |
| A route server strips several control communities for a client | → | community slot emits the retained values as base fragments, arity refusal intact | existing `test/plugin/bgp-rs-community-strip-multi.ci` |
| A received UPDATE forwarded unchanged to a same-context peer | → | no edit set built, zero-copy passthrough | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A corpus of received UPDATEs replayed through every transform this child owns | Byte-identical output to the current path, for every case, before any old code is deleted |
| AC-2 | Any edit set, of any slot-kind combination | The size query equals the number of bytes the writer then writes, exactly |
| AC-3 | A modified UPDATE forwarded to one destination | Zero heap allocations on the modify path; today the path allocates in `groupOpsByCode`, in `genericCommunityHandler` and in `removeValues` |
| AC-4 | A modification whose result would exceed the destination's buffer | The route is suppressed for that destination with a warning naming the peer and the size, and is never forwarded unmodified |
| AC-5 | An UPDATE that gains a new attribute whose code sorts before an existing one | The emitted sequence is ascending by type code |
| AC-6 | An MP_REACH next-hop rewrite on a route carrying many prefixes | The NLRI tail is copied exactly once, straight into the output buffer, with no intermediate |
| AC-7 | A community removal of a subset of a list | No heap allocation and no intermediate buffer; retained values are emitted as base fragments |
| AC-8 | A Remove buffer that is not a whole number of wire values | The operation is refused, warned and counted, the attribute's other operations still apply, and the route still goes out |
| AC-9 | A destination whose edit set touches no attribute | No buffer is acquired and the forward stays zero-copy |
| AC-10 | The accumulator reset, measured against two different inline capacities | Reset cost does not scale with inline capacity |
| AC-11 | An attribute value that crosses the 255-octet boundary as a result of an edit | The output header size class is correct and the size query predicted it |
| AC-12 | A registered handler that panics | The panic is contained, the route is suppressed for that destination, and the daemon stays up |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures next-hop-self and a community tag on an eBGP peer, then receives a route | wire, base, destination edit set, one materialisation, TCP | `test/plugin/wire-edit-single-materialise.ci` |
| 2 | Runs a route reflector with clients | wire, RR slots merge-inserted, one materialisation per client | `test/plugin/wire-edit-rr-attr-order.ci` |
| 3 | Runs a route server that strips its control communities | wire, community slot as base fragments, TCP | existing `test/plugin/bgp-rs-community-strip-multi.ci` |
| 4 | Writes an export policy whose result cannot fit the peer's message size | wire, size query, suppression, warning and counter | `test/plugin/wire-edit-oversize-suppress.ci` |
| 5 | Rewrites the next-hop on an IPv6 route carrying many prefixes | wire, MP_REACH fragments, single NLRI copy, TCP | existing `test/plugin/nexthop-self-ipv6-forward.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEditSizeIsExact` | `internal/component/bgp/filterapi/editset_test.go` | AC-2, AC-11: the size query equals the bytes written for every slot kind and both header classes | |
| `TestSlotKindsCoverEveryProducer` | `internal/component/bgp/filterapi/editset_test.go` | A-1: one row per edit producer in the census, each mapping to exactly one slot kind | |
| `TestResetIsConstantTime` | `internal/component/bgp/filterapi/editset_test.go` | AC-10, A-4: reset touches only used prefixes | |
| `TestArenaHoldsOnlyNewBytes` | `internal/component/bgp/filterapi/editset_test.go` | a fragment over base bytes never enters the arena | |
| `TestEditGoldenByteIdentity` | `internal/component/bgp/reactor/forward_build_golden_test.go` | AC-1: corpus times transform matrix, new output equals current output byte for byte | |
| `TestMergeInsertAscendingOrder` | `internal/component/bgp/reactor/forward_build_test.go` | AC-5: an added attribute lands at its ascending position, not at the end | |
| `TestUntouchedAttributesKeepBaseOrder` | `internal/component/bgp/reactor/forward_build_test.go` | R-2: an UPDATE with no added attribute is not reordered | |
| `TestOversizeModificationSuppresses` | `internal/component/bgp/reactor/forward_build_test.go` | AC-4: suppression, warning and counter, never an unmodified forward | |
| `TestModifyPathZeroAlloc` | `internal/component/bgp/reactor/forward_build_test.go` | AC-3: no heap allocation per destination on the modify path | |
| `TestNoEditSetNoBuffer` | `internal/component/bgp/reactor/forward_build_test.go` | AC-9: an empty edit set acquires nothing | |
| `TestHandlerPanicSuppressesRoute` | `internal/component/bgp/reactor/forward_build_test.go` | AC-12: the panic guard survives the contract change | |
| `TestFragmentListNoIntermediateCopy` | `internal/component/bgp/reactor/filter_delta_handlers_test.go` | AC-6: the MP_REACH NLRI tail is copied exactly once | |
| `TestCommunityRemoveZeroAlloc` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-7: removing a subset allocates nothing | |
| `TestCommunityRemoveArityRefusal` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-8: the arity contract, its warning and its counter survive the rewrite | |
| `BenchmarkForwardModifiedPerDestination` | `internal/component/bgp/reactor/forward_build_bench_test.go` | AC-3, A-4: allocations and copies per destination, before and after | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| attribute value length | 0-65535 | 65535 | N/A | 65536 (refuse the edit, suppress the route) |
| attribute header size class boundary | 255-256 | 255 (3-byte header) | N/A | 256 (4-byte header) |
| UPDATE body length | 4-65516 | 65516 | 3 | 65517 |
| edit slots | 0-n | 12 inline | N/A | 13 (heap spill, counted) |
| fragments per edit set | 0-n | 24 inline | N/A | 25 (heap spill, counted) |
| arena bytes | 0-n | 128 inline | N/A | 129 (heap spill, counted) |
| size delta | -65516 to +65516 | +65516 | -65517 | +65517 |
| MP_REACH next-hop length | 4, 16, 32 | 32 | 3 | 33 |
| community remove buffer length | multiples of 4, 8 or 12 | any exact multiple | N/A | any non-multiple (refused, warned, counted) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wire-edit-single-materialise` | `test/plugin/wire-edit-single-materialise.ci` | an eBGP peer with next-hop-self and a community tag receives a correctly modified route, built once | |
| `wire-edit-rr-attr-order` | `test/plugin/wire-edit-rr-attr-order.ci` | a reflector client sees ORIGINATOR_ID and CLUSTER_LIST in ascending code order | |
| `wire-edit-oversize-suppress` | `test/plugin/wire-edit-oversize-suppress.ci` | an oversize modification suppresses the route rather than leaking an unmodified one | |
| `bgp-rs-community-strip-multi` | existing `test/plugin/bgp-rs-community-strip-multi.ci` | multi-value control-community strip still removes every value | |
| `nexthop-self-ipv6-forward` | existing `test/plugin/nexthop-self-ipv6-forward.ci` | IPv6 next-hop rewrite over MP_REACH is unchanged | |
| `community-strip` | existing `test/plugin/community-strip.ci` | community strip is unchanged | |
| `community-tag` | existing `test/plugin/community-tag.ci` | community tag is unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-wire-edit-ebgp-modify` | `test/interop/scenarios/` | BIRD | a real peer accepts the modified UPDATE and installs the expected attributes | |
| `NN-wire-edit-rr-reflection` | `test/interop/scenarios/` | GoBGP | a reflector client accepts ORIGINATOR_ID and CLUSTER_LIST in the emitted order | |

## Files to Modify
- `internal/component/bgp/filterapi/filterapi.go` - the accumulator gains slots, fragments, the arena, the exact delta and the size query; the handler contract changes
- `internal/component/bgp/reactor/forward_build.go` - `buildModifiedPayload` becomes the one-pass merge writer; `groupOpsByCode` and the slack sizing retire
- `internal/component/bgp/reactor/filter_delta_handlers.go` - the generic, ORIGINATOR_ID, CLUSTER_LIST and MP_REACH handlers move to the new contract; the MP_REACH one becomes a declared fragment list
- `internal/component/bgp/reactor/reactor_api_forward.go` - the accumulator hoists above the destination loop; the modify call site handles suppression
- `internal/component/bgp/reactor/forward_rs.go` - the route-server rail hoists its accumulator the same way
- `internal/component/bgp/reactor/filter_ordered.go` - the two policy-chain modify call sites move to the new return shape
- `internal/component/bgp/reactor/peer_forward_facts.go` - the suppress producers emit delete slots
- `internal/component/bgp/plugins/filter_community/handler.go` - fragment-based, allocation-free, arity refusal preserved
- `internal/component/bgp/plugins/role/otc.go` - the OTC handler moves to the new contract
- `docs/architecture/core-design.md` - the modification-accumulator section
- `docs/architecture/wire/attributes.md` - the fragment model and the header size class rule
- `docs/architecture/wire/update-packing.md` - the merge-insert ordering rule
- `ai/rules/plugin-design.md` - the handler size query
- `docs/guide/plugins.md` - the plugin-facing handler change

## Files to Create
- `internal/component/bgp/filterapi/editset.go` - slots, fragments, the arena, the size query and the reset discipline
- `internal/component/bgp/filterapi/editset_test.go` - sizing, slot-kind coverage and reset cost
- `internal/component/bgp/reactor/forward_build_golden_test.go` - the corpus times transform matrix byte-identity harness
- `test/plugin/wire-edit-single-materialise.ci` - one materialisation per destination
- `test/plugin/wire-edit-rr-attr-order.ci` - ascending order on reflection
- `test/plugin/wire-edit-oversize-suppress.ci` - fail-closed on oversize

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new configuration surface |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | No | No new RPC; the three new `.ci` files cover the changed behaviour |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_update_edit_suppressed_total` (oversize, labelled by peer), `bgp_update_edit_spill_total` (labelled slot, fragment, arena). The existing remove-buffer refusal counter stays as it is |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Internal representation change, plus one behaviour change on oversize |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | The filter IPC is out of scope; it is owned by `plan/spec-filter-wire-0-umbrella.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: the registered attribute handler contract gains a size query |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` and `docs/architecture/wire/update-packing.md` for the merge-insert ordering rule |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugin-design.md` for the handler size query |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` row for RFC 4271 Section 5 ordering, with a source anchor |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` modification-accumulator section |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the two new counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The registered handler shape changes; record it where the handler registry is documented |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `filterapi.go`, `forward_build.go`, `filter_delta_handlers.go` and the community handler, and correct each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No user-facing syntax here |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- pin today's bytes before changing anything
   - Tests: `TestEditGoldenByteIdentity`, written so both sides call the current code and it passes trivially
   - Files: `internal/component/bgp/reactor/forward_build_golden_test.go`, corpus under `internal/component/bgp/reactor/testdata/`
   - Verify: the harness pins current output for every transform, so any later divergence is caught at the byte level
2. **Phase: the counter first** -- make the oversize rate observable before changing behaviour
   - Tests: `TestOversizeModificationSuppresses` in its counting-only form
   - Files: `internal/component/bgp/reactor/forward_build.go`
   - Verify: the abandon branch at `internal/component/bgp/reactor/forward_build.go:236` now counts and logs while still behaving as today; A-6 becomes answerable
3. **Phase: the edit set structure** -- slots, fragments, arena, exact delta, constant-time reset
   - Tests: `TestEditSizeIsExact`, `TestArenaHoldsOnlyNewBytes`, `TestResetIsConstantTime`, `TestSlotKindsCoverEveryProducer`
   - Files: `internal/component/bgp/filterapi/editset.go`, `internal/component/bgp/filterapi/filterapi.go`
   - Verify: AC-2, AC-10 and AC-11 pass; A-1 and A-3 resolved; nothing consumes it yet
4. **Phase: the handler contract** -- add the size query and migrate every handler
   - Tests: `TestFragmentListNoIntermediateCopy`, `TestCommunityRemoveZeroAlloc`, `TestCommunityRemoveArityRefusal`, `TestHandlerPanicSuppressesRoute`
   - Files: `internal/component/bgp/reactor/filter_delta_handlers.go`, `internal/component/bgp/plugins/filter_community/handler.go`, `internal/component/bgp/plugins/role/otc.go`
   - Verify: AC-6, AC-7, AC-8 and AC-12 pass; A-2 resolved by grep; the golden harness is still byte-identical
5. **Phase: the one-pass writer** -- exact sizing, merge-insert, suppression
   - Tests: `TestMergeInsertAscendingOrder`, `TestUntouchedAttributesKeepBaseOrder`, `TestOversizeModificationSuppresses`, `TestModifyPathZeroAlloc`, `TestNoEditSetNoBuffer`
   - Files: `internal/component/bgp/reactor/forward_build.go`
   - Verify: AC-1, AC-3, AC-4, AC-5 and AC-9 pass; `groupOpsByCode` is gone
6. **Phase: hoist and wire the call sites**
   - Tests: `BenchmarkForwardModifiedPerDestination`, plus the three new `.ci` files
   - Files: `internal/component/bgp/reactor/reactor_api_forward.go`, `internal/component/bgp/reactor/forward_rs.go`, `internal/component/bgp/reactor/filter_ordered.go`, `internal/component/bgp/reactor/peer_forward_facts.go`
   - Verify: A-4 resolved with numbers; every new and existing `.ci` passes
7. **Phase: documentation and counters**
   - Tests: the existing docs gate
   - Files: the four doc targets plus `ai/rules/plugin-design.md`
   - Verify: every Documentation row marked Yes is done with source anchors

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and every edit producer in the census maps to exactly one slot kind with a test |
| Feature completeness | Every user story has a passing `.ci`, including the oversize and multi-value strip cases |
| Correctness | The size query equals the bytes written for every kind; merge-insert never reorders untouched attributes; the header size class is decided from the final length, not the source length |
| Naming | Slot, fragment and arena names follow `ai/rules/naming.md`; the accumulator keeps its exported names so plugins compile |
| Data flow | Fragments hold offsets, never bytes; the edit set never outlives its base; the per-peer buffer has exactly one return point |
| Registration over hardcoding | Handlers stay registry-driven per code; the writer gains no per-attribute switch case |
| Rule: `ai/rules/buffer-first.md` | No `append` and no bare allocation in the writer; an exact size query precedes every write |
| Rule: `ai/rules/fail-closed-guards.md` | Nothing returns a nil a caller can read as "nothing to do"; oversize suppresses and speaks; the arity refusal keeps both its log and its counter |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Golden byte-identity harness | `go test ./internal/component/bgp/reactor/ -run TestEditGoldenByteIdentity -v` |
| Exact sizing | `go test ./internal/component/bgp/filterapi/ -run TestEditSizeIsExact` |
| Zero allocations on the modify path | `go test ./internal/component/bgp/reactor/ -bench BenchmarkForwardModifiedPerDestination -benchmem` reports 0 allocs/op |
| Per-code op grouping removed | `grep -n "groupOpsByCode" internal/component/bgp/reactor/forward_build.go` returns nothing |
| Slack sizing removed | `grep -n "len(payload) + 256" internal/component/bgp/reactor/forward_build.go` returns nothing |
| Oversize suppresses | `go test ./internal/component/bgp/reactor/ -run TestOversizeModificationSuppresses` |
| Community removal allocates nothing | `go test ./internal/component/bgp/plugins/filter_community/ -run TestCommunityRemoveZeroAlloc -benchmem` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Fragments name offsets into peer-controlled bytes. Every offset and length is validated against the base at slot construction, so the writer cannot be handed a fragment that slices out of range |
| Resource exhaustion | Slot, fragment and arena spills are attacker-influenceable through attribute count and community list length. Spills are bounded by the RFC 8654 body ceiling and are counted |
| Integer handling | The size delta is signed and must not underflow when an edit shrinks an attribute; every 16-bit wire field is bounds-checked before conversion to a Go int |
| Fail-open risk | Today an oversize modification forwards the route unmodified, which can leak exactly what a policy exists to strip. The replacement must suppress |
| Contract violation handling | A Remove buffer of the wrong arity must stay a refusal that speaks and counts, never a silent no-op |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Golden harness reports a byte difference | STOP. The transform is not equivalent. Back to design, do not adjust the golden |
| The size query disagrees with the bytes written | STOP. This is the core invariant of the child |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Interop peer rejects an UPDATE | STOP and present. A real peer disagreeing is stronger evidence than any unit test |
| An edit producer that no slot kind can express | Record it against A-1, use the terminal raw override, and report before proceeding |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The fragment model is not new to this codebase, it is just undeclared. `mpReachNextHopHandler` writes base bytes, then new bytes, then base bytes, because that is the only way to rewrite a next-hop without copying the NLRI twice. Every other handler lacks the vocabulary to say that, and the community handler pays three allocations for the absence.
- A community removal is the cleanest case in the whole set: the retained values are a subsequence of the original bytes, so the correct implementation copies nothing new at all and allocates nothing.
- Exact sizing is not primarily a performance change, it is a correctness change: it is what lets the fail-open overflow branch be deleted rather than merely made louder.
- The reset discipline is the load-bearing detail. A 500-byte inline value re-zeroed per destination would make this child a regression, which is why the hoist and the used-prefix reset are requirements, not optimisations.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fragment lists as the slot value | compose each touched attribute's final value into the arena | Composing copies the MP_REACH NLRI tail twice. Fragments generalise what one handler already does by hand |
| Three slot kinds, including a generate resolver | fragments only | An ASN4 transcode re-encodes every ASN and cannot be fragments; staging it through the arena reintroduces the double move. The kind is defined here so child 3 needs no writer change |
| Exact size query on the handler | keep slack sizing and bounds-check every write | Slack sizing is what forces the abandon-everything branch, and that branch is fail-open |
| Merge-insert new attributes | append at the end (current), or re-sort every emission | Append diverges from the announce rails; re-sorting changes bytes on the pure-forward path and breaks the zero-copy identity |
| Length-neutral edits are a sizing fast path, not a mechanism | patch equal-length values in place | The base is the shared receive buffer, and the same slice is appended for every destination with no copy. An in-place patch would corrupt every other destination's view |
| Constant-time reset with the value hoisted above the loop | keep declaring the accumulator per iteration | A 500-byte inline value re-zeroed per destination is slower than today's 150-byte one |
| Oversize suppresses | keep forwarding unmodified, but log it | Forwarding unmodified can leak exactly what the policy exists to strip. The counter lands first so the rate is known before the behaviour flips |

## BLOCKED: three RFC-tagged test call sites need Thomas's approval (2026-08-01)

The handler contract change (`AttrModHandler` now plans instead of writing) forces
a CALL-SHAPE change in three test functions that carry `RFC requirement:` tags.
`.claude/hooks/pretool-writeedit.py` refuses the edit, and only Thomas may
authorize it (`ai/rules/rfc-compliance.md`). No assertion changes; the call shape
does.

| Site | Tag | Change |
|------|-----|--------|
| `internal/component/bgp/reactor/rfc8277_test.go` `TestLabeledPropagationUnchangedNextHopKeepsLabels` | RFC8277-3.2.1-1 positive | `mpReachNextHopHandler()(src, ops, buf, 0)` -> `planHandlerBytes(...)`; asserted offset becomes asserted length |
| `internal/component/bgp/plugins/role/otc_test.go` `TestOTCAttrModHandlerNewAttr` | RFC9234-5-6 negative | `otcAttrModHandler(nil, ops, buf, 0)` -> `planOTCBytes(nil, ops)`; `buf[N]` -> `out[N]` |
| `internal/component/bgp/plugins/role/otc_test.go` `TestOTCAttrModHandlerExistingPreserved` | RFC9234-5-6 positive | same shape; 65001 still wins over the op's 65000 |

**This is urgent rather than cosmetic.** While those three sites stay on the old
shape, `reactor` and `role` do not compile their test binaries, so all 95 RFC
requirements tagged in those two packages currently prove NOTHING
(`make ze-rfc-check` says so explicitly).

The three edits were proven safe without touching the files, using
`go test -overlay tmp/_editapply/overlay.json`: both packages pass in full.
On approval, apply them and add
`// rfc-test-change-approved: <date> <what Thomas approved>` to each.

## Known Limitations

- The AS_PATH family still runs as a separate pass, so an eBGP peer with policy still pays two payload copies until `plan/spec-wire-edit-3-aspath-fold.md` lands.
- The filter IPC representation is unchanged; external plugins still exchange text. That is `plan/spec-filter-wire-0-umbrella.md`.
- A terminal raw override from an external filter replaces the base rather than composing with prior edits, matching today's terminal semantics at `internal/component/bgp/reactor/filter_ordered.go:289`.
- The three inline capacities come from a static census, not a traffic histogram. A-3 is the measurement gate; only the constants would change.
- Splitting still runs after materialisation, unchanged.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC | Section | Requirement | Site after this child |
|-----|---------|-------------|----------------------|
| 4271 | 5 | attribute type-code ordering on emission | the merge-insert step in `internal/component/bgp/reactor/forward_build.go` |
| 4271 | 4.3 | attribute value length cap and the Extended Length header class | the size query in `internal/component/bgp/filterapi/editset.go` |
| 4456 | 8 | ORIGINATOR_ID set-if-absent and CLUSTER_LIST prepend | `internal/component/bgp/reactor/filter_delta_handlers.go:103` and `internal/component/bgp/reactor/filter_delta_handlers.go:136` |
| 4760 | 3 | MP_REACH value layout | `internal/component/bgp/reactor/filter_delta_handlers.go:241` |
| 8654 | - | extended message body ceiling, which bounds the size delta | the size query in `internal/component/bgp/filterapi/editset.go` |
| 9234 | - | Only to Customer stamping | `internal/component/bgp/plugins/role/otc.go:739` |
| 1997 | - | community list of 4-octet values | `internal/component/bgp/plugins/filter_community/handler.go` |
| 4360 | - | extended community list of 8-octet values | `internal/component/bgp/plugins/filter_community/handler.go` |
| 8092 | - | large community list of 12-octet values | `internal/component/bgp/plugins/filter_community/handler.go` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-wire-edit-2-edit-apply.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-wire-edit-2-edit-apply.md` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

- `internal/component/bgp/filterapi/editset.go`: `EditSet` with per-code `editSlot`s, a `fragment` list naming a source (base or arena) plus offset and length, an inline arena, an exact size delta, and the `AttrPlan` handler contract (`KeepAll`, `Keep`, `Op`, `New`, `NewByte`, `Emit`, `EmitExtended`, `Drop`, `Fail`).
- `buildModifiedPayload` (`internal/component/bgp/reactor/forward_build.go`) is a single exactly-sized merge walk. `groupOpsByCode` and the `len(payload)+256` slack are gone, and the abandon-on-overflow branch is replaced by a counted, logged suppression.
- Merge-insert: a newly added attribute lands at its ascending type-code position instead of after every source attribute.
- `genericCommunityHandler` and `removeValues` are fragment-based and allocation-free; the arity refusal, its warning and its counter moved across verbatim.
- The three `AttrModHandler` call-shape changes in RFC-tagged tests were approved and applied, so `reactor` and `role` compile their test binaries again and their 95 tagged requirements prove something once more.
- RFC 7606 Section 5.4 discard of unrecognized NLRI landed in the same commit (`plan/spec-fixit-rfc7606-5-4-discard-unrecognized-nlri.md`, closed separately).

### Bugs Found/Fixed

- The fail-open overflow branch. `buildModifiedPayload` returned nil on overflow and the caller read that as "nothing to modify", forwarding the route UNMODIFIED and leaking exactly what the policy existed to strip. Now suppressed, warned and counted; `test/plugin/modify-oversize-suppress.ci`.
- `countModifyFailure` read wall time rather than the reactor's injected clock, which reddened `TestNoDirectTimeCalls` (`internal/core/clock/audit_test.go`) and left the suppression window wall-driven under simulation. Fixed at closure: `Reactor.nowUnixNano`, with `TestCountModifyFailureUsesInjectedClock`.

### Documentation Updates

- `docs/architecture/wire/attributes.md` -- the fragment model and the header size-class rule.
- `docs/architecture/memory/lifetime-contracts.md` -- the arena and base-fragment lifetimes.
- `docs/features/rfc-status.md` and `rfc/short/rfc7606.md` -- the Section 5.4 row that landed in the same commit.
- `ai/RFC-REQUIREMENTS.md` regenerated for the re-tagged call sites.

### Deviations from Plan

| # | Plan said | What was built | Why |
|---|-----------|----------------|-----|
| D-1 | `test/plugin/wire-edit-oversize-suppress.ci` | `test/plugin/modify-oversize-suppress.ci`, hardened in `ea6a4bbda` so it covers the code again | The file already existed from the T1-1 fix and pins the ceiling by exact byte arithmetic. A second file asserting the same property would be duplication |
| D-2 | `test/plugin/wire-edit-rr-attr-order.ci` | `test/plugin/wire-edit-api-origin-order.ci` (child 4) | RR adds ORIGINATOR_ID (9) and CLUSTER_LIST (10) to a base of ORIGIN (1), AS_PATH (2), NEXT_HOP (3), LOCAL_PREF (5), so appending is ALREADY ascending: the RR case cannot discriminate merge-insert from append. The announce case can -- it injects 2, 3 and 5 before the caller's 8 and 32 -- and it asserts the whole message by hex |
| D-3 | `test/plugin/wire-edit-single-materialise.ci` | not created | The wire result is pinned by existing `nexthop-self.ci` and `community-tag.ci`; the "exactly once" half is an allocation claim a `.ci` cannot see, and is asserted by `TestModifyPathZeroAlloc` |
| D-4 | `forward_build_golden_test.go` as the byte-identity harness | `goldenModifyCorpus` in `forward_modify_failure_test.go`, plus `TestGoldenBytesUnchangedCrossContextTranscode` | The corpus is small enough to live beside the failure taxonomy it shares fixtures with |
| D-5 | AC-1 byte-identical for every transform | One golden moved: `set-local-pref-and-add-med` now emits MED before LOCAL_PREF | That IS merge-insert working. RFC 4271 Section 5 orders attributes ascending on emission, so the previous append order was the defect. Deliberate wire change |
| D-6 | `TestSlotKindsCoverEveryProducer` | not written as a single table test | Coverage is per producer instead: `TestEditSizeIsExact`, `TestArenaHoldsOnlyNewBytes`, the community handler suite, `TestFragmentListNoIntermediateCopy` and the OTC suite each pin one producer's slot kind |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The handler contract change was applied to three RFC-tagged test call sites, which the edit hook refuses without Thomas's approval | The three edits were call-SHAPE only, no assertion change, and were proven safe with `go test -overlay` before any file was touched | The hook blocked the edit | Approval obtained, edits applied, both packages compile and pass |
| escalation | `countModifyFailure` was written with `time.Now()` on a reactor path | The reactor has an injectable clock and a gate that says so | `make ze-test-core` red at closure, `TestNoDirectTimeCalls` | `Reactor.nowUnixNano` plus a behavioural test; mutation-verified by an independent reviewer |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A slot per touched attribute code, of three kinds | Done | `filterapi/editset.go` `slotKind`, `editSlot` | fragments, delete, generate |
| A fragment naming source, offset and length | Done | `filterapi/editset.go` `fragment`, `fragmentSource` | adjacent fragments coalesce (`TestAdjacentFragmentsCoalesce`) |
| An arena holding only new bytes | Done | `filterapi/editset.go` `EditSet` arena | `TestArenaHoldsOnlyNewBytes` |
| An exact size delta maintained per edit | Done | `AttrPlan.ValueLen`, `editSlot.outLen` | `TestEditSizeIsExact` |
| One-pass merge writer, slack sizing retired | Done | `reactor/forward_build.go` `buildModifiedPayload` | `groupOpsByCode` gone |
| Overflow suppresses instead of forwarding unmodified | Done | `reactor/forward_modify_failure.go` `modifyFailureOverflow` | `test/plugin/modify-oversize-suppress.ci` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Changed | `goldenModifyCorpus` (`forward_modify_failure_test.go`), `TestGoldenBytesUnchangedCrossContextTranscode` | Byte-identical everywhere EXCEPT one golden, `set-local-pref-and-add-med`, where MED now precedes LOCAL_PREF. That is merge-insert correcting RFC 4271 Section 5 order, and it is a deliberate wire change |
| AC-2 | Done | `TestEditSizeIsExact` | Both header size classes |
| AC-3 | Done | `TestModifyPathZeroAlloc` | |
| AC-4 | Done | `test/plugin/modify-oversize-suppress.ci`, `modifyFailureOverflow` counter | The `.ci` places the body EXACTLY on the RFC 8654 ceiling, so it does not guess how many bytes the policy adds |
| AC-5 | Done | `TestMergeInsertAscendingOrder`; end-to-end by `test/plugin/wire-edit-api-origin-order.ci` | |
| AC-6 | Done | `TestFragmentListNoIntermediateCopy` | MP_REACH NLRI tail copied once |
| AC-7 | Done | `TestCommunityRemoveZeroAllocAndCorrect` | |
| AC-8 | Done | `TestRemoveValuesNonMultipleRefusedLoudly`, `TestGenericCommunityHandlerCountsRefusals`, `TestGenericCommunityHandlerWarnsOnNonMultiple` | Refused, warned, counted; other operations still apply |
| AC-9 | Done | `TestNoEditSetNoBuffer` | |
| AC-10 | Done | `TestResetIsConstantTime` | Reset clears used prefixes only |
| AC-11 | Done | `TestEditSizeIsExact` 255/256 rows; `AttrPlan.EmitExtended` | |
| AC-12 | Done | `TestProgressiveBuildHandlerPanic`, `TestProgressiveBuildNewAttrHandlerPanic`, `TestForwardUpdate_ModHandlerPanic` | Panic contained, route suppressed, daemon up |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestEditSizeIsExact`, `TestResetIsConstantTime`, `TestArenaHoldsOnlyNewBytes` | Done | `filterapi/editset_test.go` | plus `TestFragmentBoundsAreRefused`, `TestUnfinishedPlanRefuses`, `TestAdjacentFragmentsCoalesce`, `TestGroupedOpsIsStableAndAllocationFree` (added) |
| `TestMergeInsertAscendingOrder`, `TestUntouchedAttributesKeepBaseOrder`, `TestModifyPathZeroAlloc`, `TestNoEditSetNoBuffer`, `TestFragmentListNoIntermediateCopy` | Done | `reactor/forward_build_merge_test.go` | |
| `TestSlotKindsCoverEveryProducer` | Changed | per-producer coverage instead | D-6 |
| `TestEditGoldenByteIdentity` | Changed | `goldenModifyCorpus`, `TestGoldenBytesUnchangedCrossContextTranscode` | D-4 |
| `TestOversizeModificationSuppresses` | Changed | `test/plugin/modify-oversize-suppress.ci` | D-1 |
| `TestHandlerPanicSuppressesRoute` | Changed | three existing panic tests | AC-12 |
| `TestCommunityRemoveZeroAlloc` / `ArityRefusal` | Done | `TestCommunityRemoveZeroAllocAndCorrect`; `TestRemoveValuesNonMultipleRefusedLoudly` | |
| `BenchmarkForwardModifiedPerDestination` | Changed | `BenchmarkForwardDirect`, `BenchmarkForwardDirect_Batch` | Existing per-destination forward benchmarks carry the measurement |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/filterapi/editset.go` + `_test.go` | Done | |
| `forward_build_golden_test.go` | Changed | D-4 |
| `test/plugin/wire-edit-single-materialise.ci` | Skipped | D-3 |
| `test/plugin/wire-edit-rr-attr-order.ci` | Changed | D-2 |
| `test/plugin/wire-edit-oversize-suppress.ci` | Changed | D-1 |
| every "Files to Modify" row | Done | see the diff of `a1aec5e6c` |

### Audit Summary
- **Total items:** 30
- **Done:** 22
- **Partial:** 0
- **Skipped:** 1 (`wire-edit-single-materialise.ci`, D-3)
- **Changed:** 7 (D-1 to D-6 plus the benchmark substitution)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A handler can say "keep these base bytes, insert these new ones" instead of rebuilding the value | unit | `TestFragmentListNoIntermediateCopy`, `TestCommunityRemoveZeroAllocAndCorrect`, `TestArenaHoldsOnlyNewBytes` |
| The output size is known before a buffer is acquired | unit | `TestEditSizeIsExact` over both header classes; the `len(payload)+256` slack is gone from `forward_build.go` |
| An edit that cannot fit is a suppression, never a silent unmodified forward | functional | `test/plugin/modify-oversize-suppress.ci`, body placed exactly on the RFC 8654 ceiling |
| Attributes reach the wire in ascending type-code order | functional | `test/plugin/wire-edit-api-origin-order.ci`, whole message asserted by hex |
| No heap allocation on the modify path | unit | `TestModifyPathZeroAlloc`, `TestGroupedOpsIsStableAndAllocationFree` |
| The arity refusal survives the rewrite | functional + unit | existing `test/plugin/bgp-rs-community-strip-multi.ci`; `TestGenericCommunityHandlerCountsRefusals` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none -- `plan/deferrals/spec-wire-edit-2-edit-apply.md` was never created | done | `ls plan/deferrals/ \| grep wire-edit-2` returns nothing |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/wire-edit-2-edit-apply-<session-id>.md` |
| `review_gate.py check` | clean |
| Reviewer lenses used | correctness/wire/lifetime; tests/security/coverage (two agents over `bbd53bf22^..b1fa7ab1e`, 2026-08-02). Run 2 and run 3: two further agents over the closure-time clock fix and its test |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `countModifyFailure` read wall time on a reactor path, reddening `TestNoDirectTimeCalls` and leaving the suppression window wall-driven under simulation | `reactor/forward_modify_failure.go` `Reactor.countModifyFailure` | `Reactor.nowUnixNano` reading `r.clock` |
| 2 | ISSUE | That fix had no behavioural test; only a textual grep gate reddened on revert | `reactor/forward_modify_failure_test.go` | `TestCountModifyFailureUsesInjectedClock`, mutation-verified by an independent reviewer with `go test -overlay`: the test fails on the third assertion when the helper reverts to `time.Now()` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/filterapi/editset.go` | Yes | `grep -n "^type \|^func " ...` lists `fragment`, `slotKind`, `editSlot`, `EditSet`, `AttrPlan` and 30 methods |
| `internal/component/bgp/filterapi/editset_test.go` | Yes | 7 `Test` functions |
| `internal/component/bgp/reactor/forward_build_merge_test.go` | Yes | 6 `Test` functions |
| `test/plugin/modify-oversize-suppress.ci` | Yes | `ls test/plugin/modify-oversize-suppress.ci` |
| `test/plugin/bgp-rs-community-strip-multi.ci` | Yes | `ls test/plugin/bgp-rs-community-strip-multi.ci` |
| `test/plugin/wire-edit-single-materialise.ci` | No | Deliberate (D-3). `ls` returns "no matches found" |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | one golden moved, and only for merge-insert | `grep -n "set-local-pref-and-add-med" internal/component/bgp/reactor/forward_modify_failure_test.go` -> line 457, the single moved row |
| AC-2 | size query equals bytes written | `grep -rl "func TestEditSizeIsExact(" internal/` -> `filterapi/editset_test.go`; `make ze-test-bgp` green |
| AC-4 | oversize suppresses | `sed -n '1,18p' test/plugin/modify-oversize-suppress.ci` shows the body placed on 65516 = `maxUpdateBody` exactly |
| AC-5 | ascending order on the wire | `test/plugin/wire-edit-api-origin-order.ci` asserts the full message hex with the injected 2,3,5 before the caller's 8,32 |
| AC-8 | arity refusal, warned and counted | `grep -n "func TestGenericCommunityHandlerCountsRefusals" internal/component/bgp/plugins/filter_community/handler_test.go` |
| AC-12 | handler panic contained | `grep -n "func TestForwardUpdate_ModHandlerPanic" internal/component/bgp/reactor/forward_update_test.go` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| eBGP peer with next-hop-self and a community tag | `test/plugin/nexthop-self.ci` + `test/plugin/community-tag.ci` | Yes -- read: both drive a real session and pin the modified wire. The "built once" half is `TestModifyPathZeroAlloc` (D-3) |
| Route reflector forwards to a client | `test/plugin/wire-edit-api-origin-order.ci` | Yes -- read: it is the discriminating order case; the RR codes 9 and 10 append into ascending order anyway and cannot discriminate (D-2) |
| An export policy result exceeds the message size | `test/plugin/modify-oversize-suppress.ci` | Yes -- read: exact byte arithmetic to the ceiling, asserts suppression rather than an unmodified forward |
| Route server strips several control communities | `test/plugin/bgp-rs-community-strip-multi.ci` | Yes -- read: multi-value strip, unedited by this child |
| Received UPDATE forwarded unchanged | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | Yes -- read: pins the zero-copy passthrough by hex |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Every producer migrated to `AttrPlan`. Fragments cover the community family and MP_REACH, delete covers the suppress producers, generate covers AS_PATH. The terminal raw override at `filter_ordered.go` survives for the inexpressible case and is unchanged |
| A-2 | confirmed | The contract change reached exactly the registered handlers plus the internal generics, and three RFC-tagged test call sites. No out-of-tree break; `reactor` and `role` compile and pass |
| A-3 | confirmed for the tested corpus, not for a traffic histogram | `EditSet.Spilled()` reports slot, fragment and arena spill separately, so the census can be checked live. No fixture in the suite spills. Known Limitations already records that only the constants would change |
| A-4 | confirmed | `TestResetIsConstantTime` asserts reset cost does not scale with inline capacity; `TestGroupedOpsIsStableAndAllocationFree` and `TestModifyPathZeroAlloc` hold the hoisted path allocation-free |
| A-5 | confirmed | `TestEditSizeIsExact` over both header classes; `AttrPlan.Fail` makes a mismatch a hard failure rather than a silent truncation, and `TestUnfinishedPlanRefuses` pins it |
| A-6 | confirmed | The counter landed first (`modifyFailureOverflow` on `ze_bgp_update_modify_failed_total`), and the behaviour change is proven by a `.ci` that puts the body exactly on the ceiling rather than by guessing a rate |
| A-7 | confirmed | `plan/spec-hotpath-alloc-round-4.md` landed first and is closed. `grep -n "mods.Reset()" internal/component/bgp/reactor/` finds the hoisted reset; T1-1's modify-failure split is the `modifyFailure` type this child extends |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Wire format: the fragment model and header size class | `docs/architecture/wire/attributes.md` checked against `AttrPlan.emit`, which chooses the class from `outLen` against 255 | Yes |
| Memory: arena and base-fragment lifetimes | `docs/architecture/memory/lifetime-contracts.md` checked against `fragmentSource` -- a base fragment borrows the shared receive buffer and is never patched in place | Yes |
| RFC compliance: RFC 7606 Section 5.4 | `rfc/short/rfc7606.md` and `docs/features/rfc-status.md`, both updated in `a1aec5e6c`; proven by `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | Yes |
| Plugin SDK: the handler size query | `ai/rules/plugin-design.md` and `docs/guide/plugins.md` describe the plan-then-write contract that `AttrPlan` implements | Yes |
| Categories answered No | No config leaf, no CLI command, no new RPC, no new family or capability in `a1aec5e6c`'s diff | Yes |

## Core Insight

Exact sizing is not primarily a performance change. It is what lets a fail-open
branch be deleted rather than merely made louder: while the buffer was
over-sized by a guess, overflow had nowhere to go but "abandon every
modification", and the caller could not tell that from "nothing to modify".
