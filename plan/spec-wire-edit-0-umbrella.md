# Spec: wire-edit-0-umbrella -- Deferred UPDATE edits over an immutable wire base

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/spec-wire-edit-0-umbrella.md` |
| Updated | 2026-07-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`WireUpdate` (`internal/component/bgp/wireu/wire_update.go:40`) is a read-only
view over UPDATE payload bytes. It has no mutation surface at all. Every
modification the engine applies on the way to a peer (AS_PATH prepend, ASN4
transcode, next-hop rewrite, ORIGINATOR_ID / CLUSTER_LIST injection, community
strip or tag, OTC stamping, PrefixSID suppression, policy-filter edits) is
therefore a **full-payload rewrite into a fresh buffer**.

There are two independent rewrite mechanisms and they do not compose:

| Mechanism | Producer | Expresses |
|-----------|----------|-----------|
| AS-path rewrite family | `wireu/aspath_rewrite.go:35`, `:52`, `wireu/aspath_transcode.go:35` | AS_PATH prepend, ASN4 transcode, and the RFC 6793 derived AS4_PATH / AGGREGATOR / AS4_AGGREGATOR edits |
| Accumulator plus progressive build | `filterapi/filterapi.go:98`, `reactor/forward_build.go:58` | per-code Set / Add / Remove / Prepend / Suppress, plus NLRI and withdrawn-section overrides |

Because they are separate passes, an eBGP peer with any policy pays **two full
payload copies**, back to back: `getEBGPWire` at `reactor_api_forward.go:704`
produces a rewritten payload into a read-pool buffer, and
`buildModifiedPayload` at `reactor_api_forward.go:785` copies that rewritten
payload again into the per-peer pool buffer. The second copy is not amortised:
the accumulator differs per destination.

The attribute TLV sequence is also walked by at least five scanners that share
nothing (`session_validation.go:108`, `attribute/wire.go:291`,
`aspath_rewrite.go:95` and `:155`, `forward_build.go:171`, `community.go:70` and
`:141`), so one UPDATE fanned out to N peers walks its own attributes roughly
`5 + 2N` times.

**Goal.** Replace the single read-only `WireUpdate` with a pair:

- an immutable, shared **base** carrying the wire bytes plus an attribute span
  index built exactly once, and
- a per-destination **edit set** that records intent without moving bytes, is
  materialised into the output buffer exactly once, and can be fingerprinted so
  destinations with identical policy share one materialisation.

The same edit set, applied over an empty base, is how an API-originated route is
encoded, so text-to-wire conversion happens once and the per-route CPU cost of an
API route equals that of a forwarded route with the same number of touched
attributes.

**Non-goal.** This umbrella does not change the filter IPC representation. That
is `plan/spec-filter-wire-0-umbrella.md` (status `design`), which removes the
wire-to-text-to-delta round-trip for external plugins. The two are complementary:
filter-wire changes *who produces* edits, this set changes *what an edit is and
how it is applied*. See "Relationship to existing specs" below.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/memory/lifetime-contracts.md` - the four buffer lifetime contracts; contract A is `WireUpdate`
  → Constraint: a borrow is valid only until a named boundary; to hold bytes past the boundary a consumer must Retain or Own, always before the boundary. An edit set holds base offsets, so it inherits the base's boundary and must never outlive it.
  → Decision: contract A is "eager copy on retain" via `Snapshot()`. Edit sets are not snapshots and must not be treated as one.
- [ ] `ai/rules/buffer-first.md` - all wire encoding writes into pooled bounded buffers
  → Constraint: `append(buf, ...)`, `make([]byte, N)` in helpers, and `buildFoo() ([]byte, error)` shapes are banned in encoding code. The materialise step must be a write-at-offset into a caller buffer with a prior exact size query.
- [ ] `docs/architecture/wire/messages.md` - UPDATE body layout
  → Constraint: body is withdrawn-length(2) + withdrawn + attr-length(2) + attrs + NLRI. NLRI length is implicit (remaining bytes), so any attribute-length change shifts the NLRI section start.
- [ ] `docs/architecture/wire/attributes.md` - path attribute encoding
  → Constraint: header is 3 bytes, or 4 when the Extended Length flag is set. A value crossing 255 bytes changes the header size class, which changes the total by one byte.
- [ ] `docs/architecture/encoding-context.md` - per-peer encoding context (ASN4, ADD-PATH)
  → Constraint: zero-copy forwarding is legal only when source and destination `ContextID` match (`forward_body.go:47`). An edit set is bound to one destination context.
- [ ] `ai/rules/design-principles.md` - abstraction gate, single responsibility, minimal coupling
  → Decision: abstract at two or more use cases. There are two rewrite mechanisms and four materialise call sites today, so unification clears the gate.
- [ ] `ai/rules/fail-closed-guards.md` - a guard must fail closed or say something
  → Constraint: the current overflow path returns nil and the caller reads nil as "nothing to do", forwarding the route unmodified. Exact sizing removes the branch rather than making it louder.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE format, path attributes, eBGP AS_PATH prepend
  → Constraint: Section 9.1.2 requires prepending the local AS when propagating to an eBGP peer. Section 5 orders attribute type codes ascending on emission.
- [ ] `rfc/short/rfc6793.md` - four-octet ASN, AS4_PATH, AS_TRANS
  → Constraint: Section 4.2.2 obliges an AS4_PATH when the path is not composed only of mappable four-octet ASNs. An AS_PATH edit can therefore *induce* AS4_PATH, AGGREGATOR and AS4_AGGREGATOR edits; the AS_PATH resolver must own that derivation.
- [ ] `rfc/short/rfc4456.md` - route reflection
  → Constraint: Section 8 requires ORIGINATOR_ID set-if-absent and CLUSTER_LIST prepend. Both are per-destination edits.
- [ ] `rfc/short/rfc4760.md` - multiprotocol NLRI
  → Constraint: MP_REACH value is AFI(2) + SAFI(1) + next-hop-length(1) + next-hop + reserved(1) + NLRI. A next-hop rewrite is a sub-value edit that must not copy the NLRI tail twice.
- [ ] `rfc/short/rfc7606.md` - error handling, revised
  → Constraint: Section 5.1 forbids more than one NLRI-bearing field per UPDATE. Splitting stays a post-materialise step.
- [ ] `rfc/short/rfc8654.md` - extended message
  → Constraint: body is capped at 65516 octets, so every offset and length inside one UPDATE fits in a 16-bit field.
- [ ] `rfc/short/rfc7911.md` - ADD-PATH
  → Constraint: NLRI framing differs per family and direction; NLRI rewrites are context-bound.
- [ ] `rfc/short/rfc1997.md`, `rfc/short/rfc4360.md`, `rfc/short/rfc8092.md` - community, extended community, large community
  → Constraint: each is a list of fixed-width values (4, 8, 12). Removal of a subset is a subsequence of the original bytes, so it needs no new bytes at all.
- [ ] `rfc/short/rfc7947.md` - route server
  → Constraint: Section 2.2.2 forbids the route server modifying AS_PATH for RS-client peers; the AS-path edit must be skippable per destination.
- [ ] `rfc/short/rfc9234.md` - Only to Customer
  → Constraint: OTC (code 35) is stamped per destination by the role plugin's registered handler (`plugins/role/register.go:21`).

**Key insights:** (minimal context to resume after compaction)
- The deferred-edit idea already exists in half the engine as `ModAccumulator`. The gap is that the AS-path family is outside it, so the two run as sequential copies.
- `mpReachNextHopHandler` (`filter_delta_handlers.go:358-366`) already writes a base fragment, then new bytes, then another base fragment. It is a hand-written fragment list. The design generalises what one handler already does.
- `genericCommunityHandler` (`plugins/filter_community/handler.go:64`) lacks that concept and pays for it with three heap allocations and three copies per attribute per peer.
- The attribute walk needed to build a span index already happens unconditionally at `session_validation.go:108`, so an eager index is free.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/wireu/wire_update.go:40` - `WireUpdate`: payload, `sync.Once`-guarded section offsets, cached `AttributesWire`, cached mixed-shape verdict. No mutation surface.
- [ ] `internal/core/bgp/wire/update_sections.go:37` - `UpdateSections` value type holding four offsets plus a validity bool; accessors return zero-copy slices into the payload.
- [ ] `internal/core/bgp/attribute/wire.go:34` - `AttributesWire`: `sync.RWMutex`, packed bytes, lazily built `[]attrIndex`. `ensureIndexLocked` at `:291` builds with `make([]attrIndex, 0, 8)` at `:298`. `Get`/`Has`/`GetRaw` take the lock at `:64`, `:133`, `:182`.
- [ ] `internal/core/bgp/attribute/wire.go:19` - `attrIndex` fields: code, offset(uint16), length(uint16), hdrLen(uint8), `parsed Attribute` (interface).
- [ ] `internal/core/bgp/attribute/iterator.go:132` - `AttrFind`: standalone zero-allocation single-lookup scan.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:63` - `rewriteASPathPrepend`: scans attributes at `:95-127`, then on the slow path scans them **again** at `:155-195`. Fast path `tryDirectPrepend` at `:289` copies the whole payload with one shifted attribute. Slow path `rewritePrependASPathFull` at `:374` is 200 lines of interleaved offset arithmetic covering AS4_PATH, AGGREGATOR, AS4_AGGREGATOR and tombstone flag clearing.
- [ ] `internal/component/bgp/wireu/aspath_transcode.go:35` - `TranscodeASPath`: same shape for the no-prepend RS-client case.
- [ ] `internal/component/bgp/wireu/split.go:36` - `SplitWireUpdate`: operates on a materialised payload; returns the identical pointer when no split is needed.
- [ ] `internal/component/bgp/wireu/community.go:51` - `ParseCommunityPolicy`: full independent attribute walk.
- [ ] `internal/component/bgp/wireu/community.go:158` - `StripControlCommunities`: second full independent attribute walk, accumulating **all** matching 4-byte values into one appended slice with `result = append(result, payload[i:i+4]...)` at `internal/component/bgp/wireu/community.go:206`.
- [ ] `internal/component/bgp/filterapi/filterapi.go:98` - `ModAccumulator`: `[]AttrOp`, withdraw flag, NLRI and withdrawn rewrites, `[64]byte` inline arena at `:94`, `OpCopy` heap spill at `:172`.
- [ ] `internal/component/bgp/filterapi/filterapi.go:241` - `AttrOp`: code, action, and a pre-built value buffer; the actions begin with `AttrModSet` at `internal/component/bgp/filterapi/filterapi.go:213`.
- [ ] `internal/component/bgp/filterapi/filterapi.go:253` - `AttrModHandler` signature: source bytes, ops, buffer, offset, returns new offset. No size query.
- [ ] `internal/component/bgp/reactor/forward_build.go:58` - `buildModifiedPayload`: sizes at `len(payload)+256` (`:104`), walks source attributes copying one at a time (`:226-230`), appends unconsumed new attributes after all source attributes (`:242`), backfills attr length (`:268`), and abandons every modification on overflow by returning nil (`:236-239`).
- [ ] `internal/component/bgp/reactor/forward_build.go:538` - `groupOpsByCode` returns `[256][]AttrOp` **by value** (6144 bytes) and heap-allocates one slice per touched code at `:546`.
- [ ] `internal/component/bgp/reactor/forward_build.go:392` - `acquireModBuf`: per-peer pool, then `modBufPool`, then a bare `make` for oversized payloads.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go:20` - `genericAttrSetHandler` covering 12 codes listed at `:82-98`.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go:241` - `mpReachNextHopHandler`: writes base AFI/SAFI, new length byte, new next-hop, then base reserved-plus-NLRI at `:358-366`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:428` - `getEBGPWire` closure: per-key cache, `getReadBuf` acquisition, `adoptFwdHandle` at `:459` and `:537`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:604-839` - the per-destination loop: `mods` declared **inside** the loop at `:632`, community strip op at `:643`, RR ops at `:691-692`, next-hop and community-suppress ops at `:983-1075`, AS-override applied at `:704` and its `mods.Op(2, ...)` at `:1100`, withdrawal conversion at `:781`, modify at `:793`, body build at `:818`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:569` - `fwdBodyCacheKey` keys on destination context, the materialised wire **pointer**, and the extended flag.
- [ ] `internal/component/bgp/reactor/forward_body.go:37` - `buildFwdBody`: same-context branch appends `peerWire.Payload()` with no copy at `:72`; cross-context branch unpacks, re-encodes, and allocates `make([]byte, len(payload)*2+1024)` at `:158`.
- [ ] `internal/component/bgp/reactor/received_update.go:52` - `ReceivedUpdate`: inline `WireUpdate` at `:61`, pool handle, two atomic eBGP wire slots at `:89`/`:93`, `fwdHandles` adoption and drain at `:121`/`:136`.
- [ ] `internal/component/bgp/reactor/received_update.go:170` - `EBGPWire`: lock-free hit, double-checked miss, `RewriteASPath` into a fresh read-pool buffer at `:193`.
- [ ] `internal/component/bgp/reactor/recent_cache.go:69` - `RecentUpdateCache`: no TTL (`:59`), eviction on full ack or safety valve, soft `maxEntries`.
- [ ] `internal/component/bgp/reactor/reactor.go:389-391` - default `maxEntries` is 1,000,000 when unset.
- [ ] `internal/component/bgp/reactor/session_validation.go:38` - `enforceRFC7606`: walks the whole attribute section via `ValidateUpdateRFC7606AddPath` at `:108`, then walks again via `AttrFind` for PrefixSID at `:112`. Runs on every received UPDATE before any consumer sees it.
- [ ] `internal/component/bgp/reactor/session_write.go:319` - `writeRawUpdateBody`: copies the body into the session write buffer, then into the buffered writer.
- [ ] `runIngressPolicyChain` at `internal/component/bgp/reactor/filter_ordered.go:142` and `runEgressPolicyChainASN4` at `internal/component/bgp/reactor/filter_ordered.go:250` - the ingress and egress policy chain steps; both end in `buildModifiedPayload`.
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go:43` - `exportFilterForBody`: the single egress gate for originated routes; copies the override at `:88`.
- [ ] `internal/core/bgp/attribute/builder.go:213` - `WriteTo` on `Builder`: a **second, independent** attribute encoder with its own fixed order and its own header-size-class logic.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go:64` - `genericCommunityHandler`: `make`+copy of the whole list at `:69-70`, allocating `removeValues` at `:165`, `append` growth at `:107`, second `make` on Set at `:112`, final copy at `:136`.
- [ ] `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:20-47` - records that the two announce rails once produced different byte strings for one route, "chosen by timing", and pins ascending type-code order plus rail agreement.

**Behavior to preserve:**
- Same-context zero-copy forwarding: an unmodified UPDATE forwarded to a peer sharing the source encoding context must reach the wire byte-identical, with no copy (`forward_body.go:72`).
- Every RFC-mandated transform currently applied, with identical wire output: eBGP prepend, dual-AS prepend, ASN4 transcode with AS4_PATH and AGGREGATOR derivation, RR ORIGINATOR_ID and CLUSTER_LIST, next-hop modes, community strip and tag, OTC, PrefixSID suppression, LLGR withdrawal conversion, tombstone transitive clearing at the eBGP boundary.
- Ascending attribute type-code order on both announce rails, and byte-for-byte agreement between them (`reactor_api_batch_attr_order_test.go`).
- RFC 8654 size splitting and RFC 7606 Section 5.1 shape splitting, including the identical-pointer fast return for compliant UPDATEs (`split.go:44`).
- The buffer lifetime contracts in `docs/architecture/memory/lifetime-contracts.md`, in particular contract A (`Snapshot()` eager copy) and the single return point for entry-owned pool handles.
- Every existing `.ci` expectation under `test/plugin/`, `test/policy/`, `test/encode/`.

**Behavior to change:**
- Attribute-index construction moves from lazy-under-mutex to eager-at-receive, folded into the walk `enforceRFC7606` already performs.
- New attributes are merge-inserted at their ascending type-code position instead of appended after all source attributes (`forward_build.go:242`).
- An oversize modification becomes an explicit, logged suppression instead of a silent unmodified forward.
- The `AttrModHandler` contract gains an exact size query. This is a plugin-facing break affecting four registered handlers.

## Data Flow (MANDATORY)

### Entry Point
- Peer-received UPDATE: TCP bytes into a pooled read buffer, framed by the session read loop, validated by `enforceRFC7606` (`session_validation.go:38`), wrapped as `WireUpdate` and cached as a `ReceivedUpdate` (`received_update.go:52`).
- API-originated route: text command parsed and encoded into an UPDATE body by the announce rails (`reactor_api_batch.go`, `peer_rib_routes.go`).

### Transformation Path
1. Receive and validate. The RFC 7606 walk visits every attribute (`session_validation.go:108`). **Proposed:** emit the span index and presence bitset as a by-product of that walk, before publication.
2. Ingress policy chain. Runs once per UPDATE, not per peer (`filter_ordered.go:142`). Produces an edit set over the received base; a non-empty set materialises a new base once.
3. Per-destination loop (`reactor_api_forward.go:604`). For each peer, build an edit set: RS community strip, RFC 4456 reflection attributes, next-hop mode, send-community suppression, AS-override, OTC, export policy chain. **Proposed:** the eBGP AS_PATH prepend and the ASN4 transcode join this set as generate-kind slots instead of running as a prior pass.
4. Fingerprint and dedup. **Proposed:** hash the edit set; destinations with an equal fingerprint over the same base share one materialisation.
5. Materialise once. Exact size query, acquire a per-peer pool buffer of exactly that size, single merge-walk writing the output.
6. Split if required (`split.go:36`), unchanged, operating on materialised bytes.
7. Dispatch to the forward pool; the worker writes the body through `writeRawUpdateBody` (`session_write.go:319`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read goroutine to forward loop | shared immutable base, borrowed bytes, boundary is cache eviction | No |
| Reactor to registered attribute handler | handler receives base value bytes and edit ops, returns a size and writes fragments | No |
| Reactor to external filter plugin | unchanged by this umbrella; owned by `spec-filter-wire-0-umbrella.md` | No |
| Forward loop to forward-pool worker goroutine | materialised buffer handed over via `fwdItem.peerBufIdx`, released after the TCP write | No |
| Engine to peer TCP | `writeRawUpdateBody` frames the 19-byte header and copies the body once | No |

### Integration Points
- `wireu.WireUpdate` accessors (`Payload`, `Attrs`, `NLRI`, `Withdrawn`, `MPReach`, `MPUnreach`, `MixesNLRIFields`, `IsEOR`) stay as the read surface over the base, so the roughly 47 importers named in `wireu/doc.go:7-9` are not forced to change at once.
- `filterapi.ModAccumulator` becomes the public face of the edit set, keeping `Op`, `OpCopy`, `SetWithdraw`, `SetNLRIRewrite`, `SetWithdrawnRewrite` so in-process filter plugins compile unchanged.
- `filterapi.RegisterAttrModHandler` keeps its registry shape; only the handler signature changes.
- `attribute.AttributesWire` keeps its `Get`/`All`/`ForEach` surface for the text, JSON and show paths, backed by the base's span index and a side table for parsed values.
- `wireu.SplitWireUpdate` is unchanged and consumes materialised output.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Proposed Structure

### The base (immutable, shared, one per received UPDATE)

| Field | Width | Purpose |
|-------|-------|---------|
| payload | slice header, 24 B | borrowed wire bytes; never owned, never written |
| sections | value, existing `wire.UpdateSections` | the four section offsets already computed today |
| spans | slice header, 24 B | attribute index in wire order, backed by the inline array below |
| inline spans | 8 entries times 8 B = 64 B | covers the common attribute count without a heap allocation |
| presence | 4 times uint64 = 32 B | bitset over the 256 attribute codes |
| context id | 2 B | destination-compatibility check, as today |

One span entry:

| Field | Width | Note |
|-------|-------|------|
| header offset | uint16 | RFC 8654 caps the body at 65516, so 16 bits suffice |
| value length | uint16 | |
| code | uint8 | |
| flags | uint8 | header length is `3 + extended-length bit`, derived not stored |

Total 8 bytes, no padding.

### The edit set (per destination, cheap to build and discard)

| Field | Width | Purpose |
|-------|-------|---------|
| base pointer | 8 B | may be replaced by a terminal raw override |
| touched | 4 times uint64 = 32 B | which codes carry a slot |
| slots | slice header plus 12 inline entries times 8 B = 96 B | one per touched attribute code |
| fragments | slice header plus 24 inline entries times 8 B = 192 B | the value pieces each slot emits |
| arena | slice header plus 128 B inline | holds only bytes that did not exist in the base |
| as-path intent | value | ordered prepend ASNs, transcode target, remove-private flag |
| NLRI and withdrawn overrides | two slice headers | as today |
| size delta | int32 | exact running difference against the base, maintained per edit |

A fragment names a source, an offset and a length. The source is either the base
payload or the arena. That is the whole mechanism: an edit that keeps most of an
attribute keeps it as base fragments and never copies it into an intermediate.

### Slot kinds

| Kind | Value comes from | Covers | Exact size known by |
|------|------------------|--------|---------------------|
| fragments | ordered fragment list over base and arena | Set, Add, Remove, sub-value edits such as the MP_REACH next-hop | summing fragment lengths |
| delete | nothing emitted | Suppress of codes 8, 16, 32, 40 (`peer_forward_facts.go:243-257`) | zero |
| generate | a resolver writes into the destination buffer at materialise time | AS_PATH prepend, ASN4 transcode, remove-private-as | a paired size query, which already exists as `LenWithASN4` (used at `aspath_rewrite.go:411`) |

The generate kind exists because an ASN4 transcode re-encodes every ASN in the
path: it cannot be fragments, and staging it through the arena would reintroduce
the double move. It is the same narrow set of operations
`spec-filter-wire-0-umbrella.md` already identifies as reactor-only (its risk
R-4 asks for a typed intent rather than a text token, which this is).

### Why these widths

| Choice | Alternative rejected | Reason |
|--------|---------------------|--------|
| 8-byte span | keep the existing 24-byte `attrIndex` (`attribute/wire.go:19`) | eight attributes fit in one 64-byte cache line instead of three. The materialise pass walks this index once per destination. The `parsed Attribute` interface field also retains a heap pointer for the life of a cache entry that has no TTL (`recent_cache.go:59`), so it moves to a side table used only by the text, JSON and show paths. |
| 32-byte presence bitset | 512-byte `[256]int16` code table | default cache ceiling is 1,000,000 entries (`reactor.go:389-391`), so the table would be 512 MB of index against 32 MB. The O(1) lookup buys nothing when the whole index is one cache line. |
| eager index at receive | keep lazy-under-mutex (`attribute/wire.go:291`) | the walk already happens unconditionally at `session_validation.go:108`, and again at `:112` for PrefixSID. Building eagerly on the receive goroutine before publication makes the object immutable, which removes the `sync.RWMutex` at `attribute/wire.go:35` and the lock taken on every `Get`, `Has` and `GetRaw`. |
| 12 inline slots | 8 | census of every edit producer gives a worst realistic simultaneous set of 11: ORIGINATOR_ID(9), CLUSTER_LIST(10), NEXT_HOP(3), MP_REACH(14), COMMUNITY(8), EXT(16), LARGE(32), OTC(35), PrefixSID(40), AS_PATH(2), AS4_PATH(17). Common case is five. Measurement gate A-4 below. |
| 24 inline fragments | 12 | most slots hold one fragment; MP_REACH next-hop holds two or three; a community removal of k scattered values holds up to k+1. |
| 128-byte arena | keep the current 64 (`filterapi.go:94`) | the reflector-plus-next-hop-self case already needs about 51 bytes; one community add plus an IPv6 global-and-link-local next-hop overflows 64 and takes the heap spill at `filterapi.go:151`. |
| exact size delta | keep `len(payload)+256` slack (`forward_build.go:104`) | removes the abandon-all-modifications branch at `:236-239` rather than making it louder. |

### Why length-neutral edits are not a separate mechanism

An in-place patch of a length-preserving value such as LOCAL_PREF looks free, but
the base bytes are the shared receive buffer: the same slice is appended for
every destination with no copy (`forward_body.go:72`), and the memory contract at
`received_update.go:43-51` states that all derived slices share it. Patching four
bytes there would corrupt every other destination's view of the route. In-place
is safe only after materialisation, by which point the copy is already paid.

Length-neutrality is therefore kept as a **sizing fast path**: when the size delta
is zero the output length equals the input length and the merge-walk skips its
resize bookkeeping. Same code path, cheaper arithmetic.

### Ordering rule

New attributes are merge-inserted at the first base position whose code exceeds
theirs, rather than appended after all base attributes. Untouched base attributes
keep base order, so a pure forward stays byte-identical and the zero-copy
identity survives. Re-sorting every emission was the alternative and is rejected:
it changes bytes on the forward path and RFC 4271 does not require ordering on
receive. Merge-insert also converges the two announce rails whose divergence
`reactor_api_batch_attr_order_test.go:20-38` documents.

### Reset must be constant-time

The edit set is roughly 500 bytes inline against the accumulator's current 150.
Today `mods` is declared **inside** the destination loop (`reactor_api_forward.go:632`),
so Go zeroes the whole value once per peer. The value must be hoisted above the
loop and `Reset` must clear only the used prefixes (slot length, fragment length,
arena offset, touched bitset, delta), never the inline arrays. Without this rule a
larger inline buffer makes the hot path slower, not faster.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every edit the engine applies today is expressible as a fragment list, a delete, or a generate resolver. | Census of all `Op`/`OpCopy` producers plus the AS-path family; `mpReachNextHopHandler` (`filter_delta_handlers.go:358-366`) is already a hand-written fragment list. | An inexpressible edit keeps a terminal raw-override escape hatch, as external raw overrides already do (`filter_ordered.go:186`). | Per-producer mapping table in child 2, one row per call site, plus a test per row. | unvalidated |
| A-2 | The span index can be emitted from the existing RFC 7606 walk with no measurable added cost. | `ValidateUpdateRFC7606AddPath` already visits every attribute (`session_validation.go:108`), and `AttrFind` walks again at `:112`. | Fall back to lazy build, keeping the mutex; the rest of the design is unaffected. | Go benchmark on the receive path, before and after, in child 1. | unvalidated |
| A-3 | Removing the `AttributesWire` mutex is safe once the index is built before publication. | The mutex exists to guard the lazy index build (`attribute/wire.go:291`); parsed-value caching moves to a side table. | Keep the mutex only on the side table, which the forward path never touches. | Race detector over the forward-path suites plus a concurrent-fanout unit test. | unvalidated |
| A-4 | 12 slots, 24 fragments and a 128-byte arena cover the realistic worst case without a heap spill. | Static census of edit producers listed under "Why these widths". | Raise the constants; the structure is unchanged. | Instrumented run over a route-server fan-out recording the distribution of slots, fragments and arena bytes per destination. | unvalidated |
| A-5 | Fingerprint-based dedup pays off from a fan-out of two upward. | Hashing a few hundred bytes replaces a full payload copy of 100 to 4000 bytes plus a pool-buffer acquisition. | Gate dedup on a measured fan-out threshold, or drop child 5 entirely; children 1 to 4 stand alone. | Go benchmark comparing per-destination cost with dedup on and off at fan-out 1, 2, 10, 100. | unvalidated |
| A-6 | Changing the `AttrModHandler` signature affects only the four registered handlers. | `RegisterAttrModHandler` call sites: `plugins/role/register.go:21`, `plugins/filter_community/register.go:15-17`. Generic handlers are internal (`filter_delta_handlers.go:468`). | More call sites means a wider but still mechanical migration. | grep for `RegisterAttrModHandler` and `AttrModHandler` across the tree at the start of child 2. | unvalidated |
| A-7 | This redesign is not required to recover the throughput regression measured against the 2026-06-05 baseline. | The regression is un-bisected; the perf benchmark is a single-peer 100k-route convergence run with almost no fan-out, which is where this design pays. | If a bisect points at this path, reorder the children to lead with the guilty one. | Bisect of the commit range between the two measurements, run independently of this spec. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Blast radius: the forward path, the two announce rails, the filter apply engine, four plugin handlers and the buffer lifetime contracts all move. | Compile fanout the moment the handler signature changes. | Five children, each independently landable and independently revertible. Child 1 is pure addition and changes no output bytes. |
| R-2 | A wire-output regression is invisible to unit tests and only shows against a real peer. | Interop scenario diff against FRR or BIRD. | Byte-for-byte golden tests over a corpus of received UPDATEs, asserting the new path produces output identical to the current path for every existing transform, before any deletion. |
| R-3 | Retiring the intermediate read-pool variants changes buffer lifetimes, the exact area a prior fix already had to repair (the `fwdHandles` adoption at `received_update.go:121`). | Debug-build poison reads under `memguard` (`docs/architecture/memory/lifetime-contracts.md`). | Keep `adoptFwdHandle` until child 3 proves no intermediate variant remains; delete it only when the grep is clean. |
| R-4 | Merge-insert changes byte output for any received UPDATE whose attributes are not already ascending. | Golden corpus diff on the pure-forward path. | Merge-insert applies only when a new attribute is added. An UPDATE with no added attribute keeps base order unconditionally. |
| R-5 | Exact sizing turns today's silent unmodified forward into a visible suppression, which will surface latent oversize cases as new route drops. | New warning counter firing in soak. | Land the counter first, in the optimisation spec, so the frequency is known before behaviour changes. |
| R-6 | Scope creep into the filter IPC redesign. | A child spec starts editing `pkg/plugin/rpc`. | The umbrella owns the representation only. The IPC boundary is `spec-filter-wire-0-umbrella.md`. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes are mis-encoded on the wire: wrong AS_PATH, missing or duplicated attributes, a corrupted MP_REACH next-hop. Peers may reset the session under RFC 7606, or silently install wrong forwarding state. |
| How is it reverted? | Per child, single commit revert, as long as the golden byte-comparison harness from child 1 is still in place. Once a peer has accepted mis-encoded routes the wire effect is not revertible from our side. |
| Who else touches this path? | `plan/spec-filter-wire-0-umbrella.md` (filter IPC over the same apply engine), `plan/spec-perf-next-2-filter-delta-alloc.md` (deferred Phase B on the same encoders), `plan/spec-hotpath-alloc-round-4.md` (the findings this research produced), and `plan/spec-fixit-rs-community-strip-arity.md` (a live bug in the same accumulator contract). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer sends an UPDATE to an eBGP peer with next-hop-self and a community tag configured | → | edit set built in the destination loop, materialised once, written by the forward pool | `test/plugin/wire-edit-single-materialise.ci` |
| Route reflector forwards an iBGP route to a client | → | ORIGINATOR_ID and CLUSTER_LIST slots merge-inserted at ascending positions | `test/plugin/wire-edit-rr-attr-order.ci` |
| API `announce route` with communities and local-preference | → | edit set over an empty base, same writer as the forward path | `test/plugin/wire-edit-api-origin-order.ci` |
| One route fanned out to peers in two policy groups | → | fingerprint dedup, one materialisation per group | `test/plugin/wire-edit-fanout-dedup.ci` |
| Received UPDATE forwarded unchanged to a same-context peer | → | zero-copy passthrough, no edit set built | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A corpus of received UPDATEs replayed through every existing transform | The new path produces byte-identical output to the current path for every case, before any old code is deleted |
| AC-2 | An eBGP peer with an export policy that modifies attributes | Exactly one full payload copy occurs per destination, down from two; asserted by an allocation and copy counter in a Go benchmark |
| AC-3 | A modified UPDATE forwarded to one destination | Zero heap allocations on the modify path; the current path allocates in `groupOpsByCode`, `genericCommunityHandler` and `removeValues` |
| AC-4 | A modification whose result would exceed the buffer | The route is suppressed for that destination with a warning naming the peer and the size, never forwarded unmodified |
| AC-5 | An UPDATE gaining a new attribute whose code sorts before an existing one | The emitted attribute sequence is ascending by type code, on both announce rails and on the forward-modify path |
| AC-6 | An MP_REACH next-hop rewrite on a route carrying many prefixes | The NLRI tail is copied exactly once, into the output buffer; no intermediate copy exists |
| AC-7 | A community removal of a subset of a list | No heap allocation and no intermediate buffer; the retained values are emitted as base fragments |
| AC-8 | One route fanned out to N destinations in G distinct policy groups | Exactly G materialisations occur, not N |
| AC-9 | An API-originated route and a forwarded route with the same touched attribute set | Both take the same encoder; the per-route cost difference is within benchmark noise |
| AC-10 | Concurrent forwards of the same received UPDATE to many peers | No data race under `-race`; the attribute index is never written after publication |
| AC-11 | An RS-client destination | AS_PATH is not modified (RFC 7947 Section 2.2.2), and an ASN4 transcode still applies when required (RFC 6793 Section 4.2.2) |
| AC-12 | An oversize or mixed-shape UPDATE | Splitting behaves exactly as today, including the identical-pointer fast return for compliant UPDATEs |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures next-hop-self plus a community tag on an eBGP peer and receives a route | wire, receive index, destination edit set, one materialisation, TCP | `test/plugin/wire-edit-single-materialise.ci` |
| 2 | Runs a route reflector with clients | wire, RR slots merge-inserted, one materialisation per client policy | `test/plugin/wire-edit-rr-attr-order.ci` |
| 3 | Announces a route from the API with attributes | text parse, edit set over empty base, shared writer, TCP | `test/plugin/wire-edit-api-origin-order.ci` |
| 4 | Runs a route server with two peer groups and one hundred peers | wire, per-destination edit sets, fingerprint dedup, two materialisations | `test/plugin/wire-edit-fanout-dedup.ci` |
| 5 | Peers with a two-octet-ASN speaker while carrying four-octet paths | wire, AS_PATH generate slot, AS4_PATH derivation, one materialisation | `test/plugin/wire-edit-asn4-transcode-single-copy.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSpanIndexMatchesAttributesWire` | `internal/core/bgp/attribute/` | the eager span index yields the same code, offset and length set as the current lazy index, over a fuzz corpus | |
| `TestSpanIndexOneCacheLine` | `internal/core/bgp/attribute/` | a span is 8 bytes and the inline array is 64 bytes, pinned by `unsafe.Sizeof` | |
| `TestBaseImmutableAfterPublication` | `internal/component/bgp/wireu/` | no write to the base occurs after construction; run under `-race` with concurrent readers | |
| `TestEditSizeIsExact` | `internal/component/bgp/wireu/` | the size query equals the bytes written, for every slot kind and every combination, over a generated corpus | |
| `TestEditGoldenByteIdentity` | `internal/component/bgp/reactor/` | for a corpus of received UPDATEs times a matrix of transforms, new output equals current output byte for byte | |
| `TestMergeInsertAscendingOrder` | `internal/component/bgp/reactor/` | an added attribute lands at its ascending position, not at the end | |
| `TestFragmentListNoIntermediateCopy` | `internal/component/bgp/wireu/` | an MP_REACH next-hop rewrite copies the NLRI tail exactly once | |
| `TestCommunityRemoveZeroAlloc` | `internal/component/bgp/plugins/filter_community/` | removing a subset allocates nothing | |
| `TestOversizeModificationSuppresses` | `internal/component/bgp/reactor/` | an oversize edit suppresses the route and logs, and does not forward it unmodified | |
| `TestResetIsConstantTime` | `internal/component/bgp/filterapi/` | reset touches only the used prefixes; asserted by a benchmark whose cost does not scale with inline capacity | |
| `TestFingerprintEqualForEqualEdits` | `internal/component/bgp/wireu/` | equal edit sets hash equal, unequal ones differ, including generate-slot parameters | |
| `BenchmarkForwardModifiedPerDestination` | `internal/component/bgp/reactor/` | allocations and copies per destination, before and after | |
| `BenchmarkReceivePathIndexBuild` | `internal/component/bgp/reactor/` | A-2: eager index build adds no measurable receive cost | |
| `BenchmarkAPIOriginVsForward` | `internal/component/bgp/reactor/` | AC-9: equal cost for equal touched-attribute count | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| attribute value length | 0-65535 | 65535 | N/A | 65536 (refuse the edit, preserve source) |
| attribute header size class boundary | 255-256 | 255 (3-byte header) | N/A | 256 (4-byte header) |
| UPDATE body length | 4-65516 | 65516 | 3 | 65517 |
| span index entries | 0-n | 8 inline | N/A | 9 (heap spill) |
| edit slots | 0-n | 12 inline | N/A | 13 (heap spill) |
| arena bytes | 0-n | 128 inline | N/A | 129 (heap spill) |
| AS_PATH segment ASN count | 0-255 | 255 | N/A | 256 (new segment) |
| MP_REACH next-hop length | 4, 16, 32 | 32 | 3 | 33 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wire-edit-single-materialise` | `test/plugin/wire-edit-single-materialise.ci` | eBGP peer with next-hop-self and a community tag receives a correctly modified route | |
| `wire-edit-rr-attr-order` | `test/plugin/wire-edit-rr-attr-order.ci` | reflector client sees ORIGINATOR_ID and CLUSTER_LIST in ascending code order | |
| `wire-edit-api-origin-order` | `test/plugin/wire-edit-api-origin-order.ci` | API-announced route reaches the peer with ascending attribute order, identical on both rails | |
| `wire-edit-fanout-dedup` | `test/plugin/wire-edit-fanout-dedup.ci` | two policy groups, many peers, correct per-group wire | |
| `wire-edit-asn4-transcode-single-copy` | `test/plugin/wire-edit-asn4-transcode-single-copy.ci` | two-octet peer receives a correctly transcoded path with AS4_PATH | |
| `wire-edit-oversize-suppress` | `test/plugin/wire-edit-oversize-suppress.ci` | an oversize modification suppresses rather than leaking an unmodified route | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-wire-edit-ebgp-modify` | `test/interop/scenarios/` | BIRD | a real peer accepts the modified UPDATE and installs the expected attributes | |
| `NN-wire-edit-asn4-transcode` | `test/interop/scenarios/` | FRR | a two-octet-ASN speaker accepts the transcoded path with AS4_PATH | |
| `NN-wire-edit-rr-reflection` | `test/interop/scenarios/` | GoBGP | a reflector client accepts ORIGINATOR_ID and CLUSTER_LIST in the emitted order | |

## Files to Modify
- `internal/component/bgp/wireu/wire_update.go` - split into the immutable base plus read accessors
- `internal/component/bgp/wireu/aspath_rewrite.go` - becomes the AS_PATH generate resolver; the payload-copying entry points retire
- `internal/component/bgp/wireu/aspath_transcode.go` - folds into the same resolver
- `internal/component/bgp/wireu/aspath_as4.go` - AS4_PATH derivation moves under the resolver
- `internal/component/bgp/wireu/community.go` - the two independent walks consume the shared span index
- `internal/core/bgp/attribute/wire.go` - index moves to the base; mutex and lazy build retire; parsed values move to a side table
- `internal/core/bgp/attribute/builder.go` - the second encoder retires in favour of the shared writer
- `internal/component/bgp/filterapi/filterapi.go` - accumulator gains slots, fragments, arena, exact delta; handler contract gains a size query
- `internal/component/bgp/reactor/forward_build.go` - `buildModifiedPayload` becomes the one-pass merge writer
- `internal/component/bgp/reactor/forward_body.go` - cross-context re-encode uses a pooled buffer and the shared writer
- `internal/component/bgp/reactor/reactor_api_forward.go` - AS-path prepend becomes an edit; the accumulator hoists out of the loop; fingerprint dedup replaces pointer-keyed body caching
- `internal/component/bgp/reactor/forward_rs.go` - same changes on the route-server rail
- `internal/component/bgp/reactor/received_update.go` - the eBGP wire slots and the forward-handle adoption retire once no intermediate variant remains
- `internal/component/bgp/reactor/session_validation.go` - emit the span index from the existing RFC 7606 walk
- `internal/component/bgp/reactor/filter_delta_handlers.go` - the 12 generic handlers move to the new contract
- `internal/component/bgp/reactor/reactor_api_batch.go` - the announce rail uses the shared writer
- `internal/component/bgp/reactor/peer_rib_routes.go` - the queued announce rail uses the shared writer
- `internal/component/bgp/plugins/filter_community/handler.go` - fragment-based, allocation-free
- `internal/component/bgp/plugins/role/otc.go` - moves to the new contract
- `docs/architecture/wire/attributes.md` - document the base plus edit-set model
- `docs/architecture/core-design.md` - update the modification accumulator section
- `docs/architecture/memory/lifetime-contracts.md` - contract A gains the edit-set boundary rule

## Files to Create
- `plan/spec-wire-edit-1-base-index.md` - the immutable base and eager span index
- `plan/spec-wire-edit-2-edit-apply.md` - the edit set, fragments, exact sizing, one-pass writer
- `plan/spec-wire-edit-3-aspath-fold.md` - AS_PATH and transcode as generate slots
- `plan/spec-wire-edit-4-api-origin.md` - API text converges on the shared writer
- `plan/spec-wire-edit-5-fanout-dedup.md` - fingerprint-based materialisation sharing
- `test/plugin/wire-edit-single-materialise.ci` - one copy per destination
- `test/plugin/wire-edit-rr-attr-order.ci` - ascending order on reflection
- `test/plugin/wire-edit-api-origin-order.ci` - rail agreement for API routes
- `test/plugin/wire-edit-fanout-dedup.ci` - per-group materialisation
- `test/plugin/wire-edit-asn4-transcode-single-copy.ci` - transcode without a second copy
- `test/plugin/wire-edit-oversize-suppress.ci` - fail-closed on oversize

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new configuration surface; the change is internal representation |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | No | No new RPC; existing forward and announce paths are covered by the `.ci` files above |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_update_materialisations_total` (labelled by destination), `bgp_update_edit_spill_total` (slot, fragment, arena), `bgp_update_edit_suppressed_total` (oversize). Names and labels to be finalised in child 2 |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Internal representation change only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | The filter IPC is out of scope; owned by `spec-filter-wire-0-umbrella.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if the handler contract change is user-visible to plugin authors |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md`, and `docs/architecture/wire/update-packing.md` for the merge-insert ordering rule |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugin-design.md` for the `AttrModHandler` size query |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` rows for RFC 4271 Section 5 ordering and RFC 6793 Section 4.2.2, with source anchors |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` modification-accumulator section, `docs/architecture/memory/lifetime-contracts.md` contract A |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the three counters above |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `wire_update.go`, `aspath_rewrite.go`, `forward_build.go`, `attribute/wire.go`, `filterapi.go` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No user-facing syntax in this area |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- golden byte-comparison harness before any structural change
   - Tests: `TestEditGoldenByteIdentity` with a corpus of received UPDATEs times a transform matrix; it passes trivially at first because both sides call the current code
   - Files: `internal/component/bgp/reactor/` test harness, corpus under `internal/component/bgp/reactor/testdata/`
   - Verify: the harness runs and pins current output for every transform, so any later divergence is caught at the byte level
2. **Phase: child 1, base and span index** (`plan/spec-wire-edit-1-base-index.md`)
   - Tests: `TestSpanIndexMatchesAttributesWire`, `TestSpanIndexOneCacheLine`, `TestBaseImmutableAfterPublication`, `BenchmarkReceivePathIndexBuild`
   - Files: `attribute/wire.go`, `wireu/wire_update.go`, `reactor/session_validation.go`
   - Verify: pure addition, zero output-byte change, mutex removed, A-2 and A-3 resolved
3. **Phase: child 2, edit set and one-pass writer** (`plan/spec-wire-edit-2-edit-apply.md`)
   - Tests: `TestEditSizeIsExact`, `TestFragmentListNoIntermediateCopy`, `TestCommunityRemoveZeroAlloc`, `TestOversizeModificationSuppresses`, `TestMergeInsertAscendingOrder`, `TestResetIsConstantTime`, `BenchmarkForwardModifiedPerDestination`
   - Files: `filterapi/filterapi.go`, `reactor/forward_build.go`, `reactor/filter_delta_handlers.go`, `plugins/filter_community/handler.go`, `plugins/role/otc.go`
   - Verify: AC-3, AC-4, AC-5, AC-6, AC-7 pass; golden harness still byte-identical
4. **Phase: child 3, AS-path fold** (`plan/spec-wire-edit-3-aspath-fold.md`)
   - Tests: `wire-edit-asn4-transcode-single-copy.ci`, plus the existing `wireu` AS-path suites unchanged
   - Files: `wireu/aspath_rewrite.go`, `wireu/aspath_transcode.go`, `wireu/aspath_as4.go`, `reactor/reactor_api_forward.go`, `reactor/forward_rs.go`, `reactor/received_update.go`
   - Verify: AC-2 and AC-11 pass; the second full copy is gone; `adoptFwdHandle` call sites reduce to zero
5. **Phase: child 4, API origin convergence** (`plan/spec-wire-edit-4-api-origin.md`)
   - Tests: `wire-edit-api-origin-order.ci`, `BenchmarkAPIOriginVsForward`, existing rail-agreement tests unchanged
   - Files: `attribute/builder.go`, `reactor/reactor_api_batch.go`, `reactor/peer_rib_routes.go`
   - Verify: AC-9 passes; one encoder remains; both rails still agree byte for byte
6. **Phase: child 5, fan-out dedup** (`plan/spec-wire-edit-5-fanout-dedup.md`)
   - Tests: `TestFingerprintEqualForEqualEdits`, `wire-edit-fanout-dedup.ci`
   - Files: `reactor/reactor_api_forward.go`, `reactor/forward_rs.go`
   - Verify: AC-8 passes; A-5 resolved with numbers at fan-out 1, 2, 10, 100

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and every edit producer in the census maps to exactly one slot kind |
| Feature completeness | Every user story has a passing `.ci`, including the RS-client and two-octet-peer paths |
| Correctness | Merge-insert never reorders untouched base attributes; the size query equals the bytes written for every slot kind; the AS_PATH resolver emits the RFC 6793 derived attributes in every case the current slow path does |
| Naming | The base and edit-set types follow `ai/rules/naming.md`; the package stays `wireu` per the recorded 2026-07-08 decision at `wireu/doc.go:7-9` |
| Data flow | The base is never written after publication; edit sets never outlive their base; no intermediate materialisation survives a child |
| Rule: `ai/rules/buffer-first.md` | No `append` or bare `make` in the writer; exact size query precedes every write |
| Rule: `ai/rules/fail-closed-guards.md` | Oversize and unexpressible edits suppress and speak; nothing returns a nil that a caller can read as "nothing to do" |
| Rule: `docs/architecture/memory/lifetime-contracts.md` | Every retired pool handle has exactly one return point; debug poison catches a surviving borrow |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Golden byte-comparison harness | `go test ./internal/component/bgp/reactor/ -run TestEditGoldenByteIdentity -v` |
| Span index is 8 bytes | `go test ./internal/core/bgp/attribute/ -run TestSpanIndexOneCacheLine` |
| `AttributesWire` mutex removed | `grep -n "sync.RWMutex" internal/core/bgp/attribute/wire.go` returns nothing |
| One copy per destination | `go test -bench BenchmarkForwardModifiedPerDestination -benchmem` shows the copy count halved |
| Zero allocations on the modify path | same benchmark reports 0 allocs/op |
| Second encoder removed | `grep -n "func (b \*Builder) WriteTo" internal/core/bgp/attribute/builder.go` returns nothing |
| Intermediate wire variants removed | `grep -rn "adoptFwdHandle" internal/` returns nothing |
| Five children exist | `ls plan/spec-wire-edit-*.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The span index is built from peer-controlled bytes. Every offset and length must be bounds-checked at index-build time so no later consumer can construct an out-of-range slice from a trusted-looking index |
| Resource exhaustion | Slot, fragment and arena spills are attacker-influenceable through attribute count and community list length. Spills must be bounded by the UPDATE size, which RFC 8654 already caps, and the spill counters must be observable |
| Integer handling | Every offset and length is 16-bit on the wire but used as a Go int for slicing. Conversions need explicit bounds, and the size delta is signed and must not underflow when an edit shrinks an attribute |
| Fail-open risk | The current overflow path forwards the route unmodified, which can leak whatever a policy exists to strip. The replacement must suppress, not pass through |
| Fingerprint collision | A hash collision would send one destination another's wire. The fingerprint must be a dedup *hint* confirmed by a full edit-set equality check before sharing a materialisation |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Golden harness reports a byte difference | STOP. The transform is not equivalent. Back to the child's design, do not adjust the golden |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Interop peer rejects an UPDATE | STOP and present. A real peer disagreeing is stronger evidence than any unit test |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The deferred-edit model is not new to this codebase. `ModAccumulator` is exactly "record the change, apply once", and it has been in place since the filter pipeline was built. What is missing is that the AS-path family sits outside it, which is what forces two sequential copies. The redesign is therefore a completion of an existing idea, not a replacement of it.
- `mpReachNextHopHandler` (`filter_delta_handlers.go:358-366`) is the existence proof for the fragment model. It writes base bytes, then new bytes, then base bytes, because that is the only way to rewrite a next-hop without copying the NLRI twice. Every other handler lacks the vocabulary to say that, and `genericCommunityHandler` (`plugins/filter_community/handler.go:64`) pays three allocations for the absence.
- The single largest structural cost is not the copies, it is that five scanners re-derive the same attribute offsets. One shared index removes roughly `4 + 2N` walks per UPDATE at fan-out N.
- The 1,000,000-entry cache default (`reactor.go:389-391`) with no TTL (`recent_cache.go:59`) is the constraint that decides every field width in the base. A 512-byte code table would have been the natural choice and is 512 MB at the ceiling.
- Fingerprinting the edit set rather than the materialised pointer moves fan-out dedup upstream of the copy. Today `fwdBodyCacheKey` (`reactor_api_forward.go:569`) keys on the wire pointer, which can only dedupe destinations that already produced the same pointer.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fragment lists as the slot value | compose each touched attribute's final value into an arena | Composing copies the MP_REACH NLRI tail twice. `mpReachNextHopHandler` already avoids this by hand; fragments generalise it. |
| Three slot kinds including a generate resolver | fragments only | An ASN4 transcode re-encodes every ASN and cannot be fragments. Staging it through the arena reintroduces the double move. |
| Presence bitset, 32 bytes | 256-entry code table, 512 bytes | 512 MB versus 32 MB at the cache ceiling, and the O(1) lookup buys nothing when the whole index is one cache line. |
| 8-byte span, parsed values in a side table | keep the 24-byte `attrIndex` with its interface field | one cache line instead of three for a typical UPDATE, and no heap pointer retained for the life of a TTL-less cache entry. |
| Eager index built during the RFC 7606 walk | keep the lazy build under a mutex | the walk is already unconditional (`session_validation.go:108`), and eager construction converts a shared-mutable into a shared-immutable, removing the mutex. |
| Merge-insert new attributes at ascending position | append at the end (current), or re-sort every emission | append diverges from the announce rails; re-sorting changes bytes on the pure-forward path and breaks the zero-copy identity. |
| Length-neutral edits are a sizing fast path, not a mechanism | in-place patching of equal-length values | the base is the shared receive buffer; an in-place patch corrupts every other destination's view. |
| Exact size delta maintained per edit | keep the slack-plus-abandon approach | removes a fail-open branch that currently forwards routes unmodified. |
| Constant-time reset with the value hoisted above the loop | keep declaring the accumulator per iteration | a 500-byte inline value re-zeroed per destination would make the hot path slower than today. |
| Fingerprint is a hint, confirmed by full equality | trust the hash | a collision would send one destination another destination's wire. |
| Package stays `wireu` | rename to a spelled-out concern name | the 2026-07-08 user decision recorded at `wireu/doc.go:7-9` declined the rename over roughly 47 importers. |

## Known Limitations

- The filter IPC representation is unchanged. External plugins still exchange text. That is `plan/spec-filter-wire-0-umbrella.md`.
- Splitting still runs after materialisation. A drafted UPDATE is not splittable, and splitting is rare enough (oversize or mixed shape) not to justify it.
- A terminal raw override from an external filter replaces the base rather than composing with prior edits, matching today's terminal semantics (`filter_ordered.go:186`, `:289`).
- The three inline capacities are derived from a static census, not from a traffic histogram. A-4 is the measurement gate; only the constants would change.
- This umbrella makes no claim about the throughput regression measured against the 2026-06-05 baseline. See A-7.

## Relationship to existing specs

| Spec | Status | Overlap | Resolution |
|------|--------|---------|------------|
| `plan/spec-filter-wire-0-umbrella.md` | design | Its open axis A-1 asks whether to unify on raw or keep a dual text representation. This umbrella answers unify-on-raw structurally, by giving both origins one encoder. | This umbrella lands first and becomes filter-wire's substrate. Filter-wire keeps ownership of the IPC boundary and the seven plugins. Its A-1 row should be updated to reference child 4 rather than re-benchmarked independently. |
| `plan/spec-perf-next-2-filter-delta-alloc.md` | in-progress, Phase A landed, Phase B deferred | Phase B is "pooled scratch for the 14 encoder sites", which is the same encoders child 2 replaces. | Phase B folds into child 2. At perf-next-2 closure, its deferral row must name child 2 as destination rather than a generic later. |
| `plan/spec-perf-next-0-umbrella.md` | in-progress, awaiting closure | Records "profile before coding" as a blocking gate and lists candidates that profiling rejected. | The same gate applies here: each child locates its frames in a fresh profile before implementation. Child 5's win will not appear in the single-peer 100k-route baseline; its benchmark is the proof. |
| `plan/spec-hotpath-alloc-round-4.md` | design | The independently-fixable findings from the same research. Its Tier 1 items T1-1 (the fail-open modify failure) and T1-3 (hoisting the accumulator above the destination loop) are **preconditions of child 2**, not duplicates of it. | Hotpath Tier 1 lands FIRST, in full. Child 2 consumes both as preconditions and must not re-implement either. The ordering is load-bearing for T1-3: child 2 grows the accumulator from roughly 150 to roughly 500 inline bytes, so if the hoist is not already in place child 2 regresses the very loop it exists to speed up. Hotpath Tier 2 is the non-overlap boundary: every T2 row names the child that removes it. |
| `plan/spec-fixit-rs-community-strip-arity.md` | **implemented and committed 2026-07-28** (`4730deb84`, learned summary `plan/learned/1280-fixit-rs-community-strip-arity.md`); awaiting two-commit closure | A live bug in the accumulator's unwritten arity contract, on the route-server path. | Landed independently of this umbrella, as planned. Two consequences for this spec set. First, it moved lines in six files that every wire-edit spec cites; citations here were re-pointed on 2026-07-28 and any future session must re-check rather than trust them. Second, it is the empirical case for child 2's typed slot: the arity rule it violated lived only in a comment, and two of four producers broke it silently for months. |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. The
requirements this set must carry into the new code, each currently documented at
the site it moves from:

| RFC | Section | Requirement | Current site |
|-----|---------|-------------|--------------|
| 4271 | 9.1.2 | prepend the local AS when propagating to an eBGP peer | `aspath_rewrite.go:19-25` |
| 4271 | 5 | attribute type-code ordering on emission | `reactor_api_batch_attr_order_test.go:36` |
| 4271 | 4.3 | duplicate attributes are a malformed attribute list | `attribute/wire.go:308-312` |
| 6793 | 4.2.2 | AS4_PATH obligation, AGGREGATOR set to AS_TRANS | `aspath_rewrite.go:231-239`, `:296-308`, `:502-504` |
| 6793 | 6 | discard a malformed AS4_PATH and continue | `aspath_rewrite.go:396-400` |
| 4456 | 8 | ORIGINATOR_ID and CLUSTER_LIST on reflection | `filter_delta_handlers.go:100-135` |
| 4760 | 3 | MP_REACH value layout | `filter_delta_handlers.go:292` |
| 7606 | 5.1 | one NLRI-bearing field per UPDATE | `wireu/wire_update.go:175-189`, `split.go:41-44` |
| 7947 | 2.2.2 | a route server must not modify AS_PATH for RS clients | `reactor_api_forward.go:700-701` |
| 8654 | - | extended message body ceiling | `wire/update_sections.go:51` |
| 9234 | - | Only to Customer stamping | `plugins/role/otc.go:18` |
| draft-mangin-idr-attr-tombstone-00 | 5.3 | clear the transitive bit at the eBGP boundary | `aspath_rewrite.go:528-542` |

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
- [ ] Learned summary written to `plan/learned/NNN-wire-edit-0-umbrella.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-wire-edit-0-umbrella.md` only (commit A preserves the spec in history)
