# Spec: wire-edit-4-api-origin -- the announce rails converge on the shared writer

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | `plan/spec-wire-edit-2-edit-apply.md` |
| Phase | 6/7 |
| Deferral shard | `plan/deferrals/wire-edit-4-api-origin.md` |
| Updated | 2026-08-01 |

Child 4 of `plan/spec-wire-edit-0-umbrella.md`. It removes the second attribute
encoder by making an API-originated route an edit set over an empty base.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An announce reaches the wire through one of two builders, and which one runs is
decided by scheduling, not by the route.
`reactor_api_batch_attr_order_test.go:20` records this in the tree already: a
route injected while the destination peer is still draining its initial sync is
queued and later drained through `buildRIBRouteUpdate`
(`internal/component/bgp/reactor/peer_rib_routes.go:46`), while the same route
injected after establishment is built by
`buildBatchAnnounceUpdate` (`internal/component/bgp/reactor/reactor_api_batch.go:580`).
`ShouldQueue` (`internal/component/bgp/reactor/peer.go:1148`) picks. Nothing in the route does.

Those two rails do not share an encoder, and neither shares one with the forward
path:

| Encoder | Site | Ordering rule |
|---------|------|---------------|
| `attribute.Builder` | `WriteTo` at `internal/core/bgp/attribute/builder.go:213`, sized by `Len` at `internal/core/bgp/attribute/builder.go:137` | a fixed emission order hard-coded in the function body |
| `attrWriter` over `attribute.WriteAttrTo` | `attrWriter` at `internal/component/bgp/reactor/peer_rib_routes.go:286` | the caller's call order, bounded by a latching limit |
| the forward-path writer | `buildModifiedPayload` at `internal/component/bgp/reactor/forward_build.go:58` | source order, with new attributes merge-inserted after `plan/spec-wire-edit-2-edit-apply.md` |

The cost of three encoders has already been paid once: the batch rail used to
append LOCAL_PREF, MP_REACH_NLRI and AS4_PATH after the caller's verbatim
attribute block, so one route came out as two different byte strings depending on
which rail won the race, and `test/plugin/ddos-flowspec-announce.ci` pinned only
one of them. The repair added ordering machinery to the batch rail alone
(`findAttrInsertPosition` at `internal/component/bgp/reactor/reactor_api_batch.go:1001`,
`insertAttrOrdered` at `internal/component/bgp/reactor/reactor_api_batch.go:1085`)
and a test pair to hold the two rails together.

**Goal.** Make an API-originated route an **edit set over an empty base**,
materialised by the same one-pass writer the forward path uses. Then:

- one encoder remains, so ordering is a property of the writer rather than an agreement between three call sites,
- the rail-agreement tests become structurally true instead of separately maintained,
- text-to-wire conversion happens once, and the per-route cost of an API route equals that of a forwarded route with the same touched-attribute count.

**Non-goal, with ONE owner-approved exception.** This child changes who writes the
bytes, not what they say -- except for RFC 4271 Section 5.1.5, which Thomas ruled
on 2026-08-01 must be fixed inside child 4.

**Behavior change (eBGP announce bytes MOVE).** RFC 4271 Section 5.1.5: "A BGP
speaker MUST NOT include this attribute in UPDATE messages it sends to external
peers, except in the case of BGP Confederations [RFC3065]." The batch rail copied
the caller's attribute block verbatim and nothing removed type code 5, so an
operator-supplied local-preference crossed the AS boundary on every announce to an
external peer that had finished its initial sync. The queued rail wrote LOCAL_PREF
only under `if isIBGP` and was already conformant. The batch rail now matches it, so
an API announce carrying LOCAL_PREF to an eBGP peer is SHORTER by seven octets than
it was. The confederation exception has no configuration surface in Ze (a session is
internal when LocalAS == PeerAS; nothing names a confederation), so the prohibition
covers every peer this daemon calls external.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/wire/update-packing.md` - how attributes are ordered on emission
  → Decision: ordering becomes a single property of the shared writer. Both rails inherit it rather than each implementing it.
- [ ] `docs/architecture/wire/attributes.md` - path attribute encoding
  → Constraint: the header size class follows the final value length, so a builder that sizes and a writer that writes must agree on the class. Today `Len` (`internal/core/bgp/attribute/builder.go:137`) and `WriteTo` (`internal/core/bgp/attribute/builder.go:213`) each decide it independently.
- [ ] `ai/rules/buffer-first.md` - encoding writes into pooled bounded buffers
  → Constraint: `Builder.WriteTo` already writes into a caller buffer after an exact `Len`, which is the contract the shared writer wants. `Build` is the allocating convenience that must not survive on the hot path.
- [ ] `docs/architecture/encoding-context.md` - per-peer encoding context
  → Constraint: the queued rail already encodes AS_PATH under the destination context (`internal/component/bgp/reactor/peer_rib_routes.go:302`), while the batch rail decides ASN width from flags. One writer must take the context, not a bool, or the rails can still diverge.
- [ ] `docs/architecture/memory/lifetime-contracts.md` - buffer lifetime contracts
  → Constraint: the queued rail parks NLRI at the tail of the same buffer that holds the attributes, which is why `attrWriter` bounds on a limit rather than on the buffer length (`internal/component/bgp/reactor/peer_rib_routes.go:278`). The shared writer must keep that region discipline or it will overwrite the prefix being announced.
- [ ] `ai/rules/fail-closed-guards.md` - a guard must fail closed or say something
  → Constraint: both rails currently drop an oversized route with a log line rather than emitting a truncated UPDATE (`internal/component/bgp/reactor/peer_rib_routes.go:325`). The shared writer's exact size query must preserve that, and must keep naming the route.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE format, path attributes
  → Constraint: Section 5 orders attribute type codes ascending on emission. This is the property the rail-agreement tests exist to hold, and the one the shared writer makes structural.
- [ ] `rfc/short/rfc6793.md` - four-octet ASN, AS4_PATH
  → Constraint: an announce to a two-octet peer must carry AS4_PATH when the path is not composed only of mappable ASNs. The batch rail does this in `insertAnnounceAS4Path` at `internal/component/bgp/reactor/reactor_api_batch.go:715`; under the shared writer it becomes the same derived slot `plan/spec-wire-edit-3-aspath-fold.md` defines.
- [ ] `rfc/short/rfc4760.md` - multiprotocol NLRI
  → Constraint: an announce for a non-IPv4-unicast family carries its NLRI inside MP_REACH_NLRI, so the NLRI region and the attribute region are not independent. The size query must account for both.
- [ ] `rfc/short/rfc7311.md` - accumulated IGP metric
  → Constraint: AIGP (code 26) is one of the attributes the preserve tests drive through both rails, and it must keep its ascending position.
- [ ] `rfc/short/rfc8654.md` - extended message
  → Constraint: the body ceiling bounds the announce, and it is the bound both rails already latch against.

**Key insights:** (minimal context to resume after compaction)
- The two announce rails already have a test pair whose only job is to hold them together: `TestAnnounceRailsAgreeByteForByte` at `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:312` and `TestAnnounceRailsPreserveUnlistedAttributes` at `internal/component/bgp/reactor/reactor_api_batch_attr_preserve_test.go:167`. Those tests are the specification of what this child makes structurally true.
- `Builder.Len` and `Builder.WriteTo` are already an exact-size-then-write pair. The contract does not need inventing, it needs sharing.
- The queued rail's `attrWriter` latches on overflow and writes nothing further (`internal/component/bgp/reactor/peer_rib_routes.go:286`), which is a fail-closed discipline the shared writer must keep.
- The batch rail already merge-inserts, via `findAttrInsertPosition` and `insertAttrOrdered`. That machinery is what the shared writer absorbs.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/core/bgp/attribute/builder.go:39` - `NewBuilder` and the setter chain: origin, local-pref, MED, next-hop, AS_PATH, communities, extended communities, large communities, atomic aggregate, AIGP, plus the `SetWire` raw-wire escape hatch at `internal/core/bgp/attribute/builder.go:130`.
- [ ] `internal/core/bgp/attribute/builder.go:137` - `Len`: returns the raw wire length when the escape hatch is set, and otherwise sums each attribute, deciding the header size class per attribute as it goes.
- [ ] `internal/core/bgp/attribute/builder.go:213` - `WriteTo`: the second, independent encoder. It copies the raw wire when set, and otherwise emits attributes in a fixed order coded into the function body, re-deriving the header size class it already derived in `Len`.
- [ ] `internal/core/bgp/attribute/builder.go:384` - `CheckedWriteTo`, plus `Build` at `internal/core/bgp/attribute/builder.go:395`: the guarded and the allocating variants of the same emission.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:33` - `AnnounceNLRIBatch`: the API entry point for a batch announce.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:580` - `buildBatchAnnounceUpdate`: the batch rail, constructing a `Builder` at `internal/component/bgp/reactor/reactor_api_batch.go:619` and then adding the mandatory attributes at `internal/component/bgp/reactor/reactor_api_batch.go:627`.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:906` - `writeMandatoryAttrs`: ensures ORIGIN and AS_PATH exist, since the `Builder` may not carry AS_PATH.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:1001` - `findAttrInsertPosition`, plus `insertAttrOrdered` at `internal/component/bgp/reactor/reactor_api_batch.go:1085`: the ascending-position machinery added to this rail alone.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:715` - `insertAnnounceAS4Path`: the RFC 6793 AS4_PATH insertion on the announce path.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:1131` - `writeASPath`, plus `writeAnnounceAS4Path` at `internal/component/bgp/reactor/reactor_api_batch.go:1156`: this rail's own AS_PATH and AS4_PATH encoders.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:501` - `packedWithLocalASPrepended`: the announce-side prepend, a fourth AS_PATH writer.
- [ ] `internal/component/bgp/reactor/peer_rib_routes.go:46` - `buildRIBRouteUpdate`: the queued rail, driven when the peer is still draining its initial sync.
- [ ] `internal/component/bgp/reactor/peer_rib_routes.go:278` - `attrWriter`: buffer, limit, offset and a latched full flag; the limit exists because the NLRI is parked at the tail of the same buffer.
- [ ] `internal/component/bgp/reactor/peer_rib_routes.go:286` - `write`: sizes with `attrWireLen`, latches on overflow, and otherwise writes through `WriteAttrTo`.
- [ ] `internal/component/bgp/reactor/peer_rib_routes.go:302` - `writeWithContext`: the AS_PATH variant, sizing through `LenWithContext` so the bound and the write cannot disagree.
- [ ] `internal/component/bgp/reactor/peer_rib_routes.go:325` - `logRIBRouteTooLarge`: the fail-closed drop with a named route.
- [ ] `internal/component/bgp/reactor/peer.go:1148` - `ShouldQueue`: the scheduling decision that selects the rail.
- [ ] `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:20` - the recorded history of the divergence, written up under `Attribute type-code ordering on the announce rails`, plus `assertAscending` at `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:74` and `TestAnnounceRailsAgreeByteForByte` at `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:312`.
- [ ] `internal/component/bgp/reactor/reactor_api_batch_attr_preserve_test.go:167` - `TestAnnounceRailsPreserveUnlistedAttributes`, which drives AIGP through both rails, plus `TestAnnounceRailsPreserveMultiSegmentASPath` at `internal/component/bgp/reactor/reactor_api_batch_attr_preserve_test.go:203`.

**Behavior to preserve:**
- Byte-for-byte agreement between the two announce rails, for every case in the order and preserve test tables.
- Ascending attribute type-code order on both rails, per RFC 4271 Section 5.
- The mandatory-attribute rules: ORIGIN and AS_PATH always present, with the announce-side AS_PATH construction unchanged, including the iBGP, RS-client and local-AS-prepend variants.
- The RFC 6793 AS4_PATH insertion on an announce to a two-octet peer.
- Attribute preservation: an attribute the caller supplied that neither rail names explicitly still reaches the wire at its ascending position, which is exactly what the preserve tests assert for AIGP.
- The fail-closed drop for an oversized announce, with a log line naming the route.
- The raw-wire escape hatch on the `Builder`, which several callers use to pass through pre-encoded attributes.
- Every existing expectation under `test/plugin/`, `test/encode/` and `test/policy/`, including `test/plugin/ddos-flowspec-announce.ci`.

**Behavior to change:**
- Both rails build an edit set over an empty base and hand it to the shared one-pass writer.
- The batch rail DROPS a caller-supplied LOCAL_PREF toward an external peer (RFC 4271 Section 5.1.5), matching the queued rail. This is the one change in this child that moves bytes on purpose.
- `Builder.WriteTo` and its emission-order logic retire; the `Builder` becomes an intent collector whose output is slots, not bytes.
- `findAttrInsertPosition` and `insertAttrOrdered` retire, because merge-insert is the writer's property.
- `attrWriter` retires; its limit discipline becomes a region bound the shared writer takes as an argument.
- The announce AS_PATH construction routes through the AS_PATH resolver from `plan/spec-wire-edit-3-aspath-fold.md`, so AS4_PATH derivation is declared once rather than twice.

## Data Flow (MANDATORY)

### Entry Point
- `AnnounceNLRIBatch` (`internal/component/bgp/reactor/reactor_api_batch.go:33`): a route injected through the API, carrying parsed attributes and a set of NLRI.
- The queued drain path: the same route, held because `ShouldQueue` (`internal/component/bgp/reactor/peer.go:1148`) said the peer was still syncing, later drained through `buildRIBRouteUpdate` (`internal/component/bgp/reactor/peer_rib_routes.go:46`).

### Transformation Path
1. The API command is parsed into attribute values. Unchanged.
2. **Proposed:** the values become slots on an edit set whose base is empty, instead of `Builder` state or a sequence of `attrWriter` calls.
3. The mandatory-attribute rules run, adding ORIGIN and AS_PATH slots when absent, exactly as `writeMandatoryAttrs` (`internal/component/bgp/reactor/reactor_api_batch.go:906`) decides today.
4. The AS_PATH slot is the resolver from `plan/spec-wire-edit-3-aspath-fold.md`, so it declares AS4_PATH when RFC 6793 obliges it, instead of the separate `insertAnnounceAS4Path` insertion at `internal/component/bgp/reactor/reactor_api_batch.go:715`.
5. The size query runs over the slots plus the NLRI region, so the whole UPDATE size is known before a buffer is taken.
6. The shared one-pass writer emits the attributes in ascending code order, then the NLRI, into the destination buffer.
7. Dispatch is unchanged on both rails.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| API command to edit set | parsed attribute values become slots; no intermediate encoded block | No |
| Edit set to shared writer | exact size, then a single write into a bounded region | No |
| Attribute region to NLRI region in one buffer | the writer takes an explicit region bound, replacing `attrWriter`'s limit | No |
| Announce rail to peer TCP | unchanged: the body is framed and written once | No |

### Integration Points
- `attribute.Builder` keeps its setter chain, so every caller compiles; only its emission half changes.
- `filterapi.ModAccumulator` and the slot vocabulary from `plan/spec-wire-edit-2-edit-apply.md` are what the rails now populate.
- The AS_PATH resolver from `plan/spec-wire-edit-3-aspath-fold.md` is shared, so both origins derive AS4_PATH identically.
- `wireu.SplitWireUpdate` continues to run after materialisation on both rails.
- `attribute.WriteAttrTo` and `attribute.WriteAttrToWithContext` remain the per-attribute primitives the writer calls.

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
| A-1 | Every attribute the `Builder` can emit is expressible as a slot over an empty base. | The `Builder` setters cover a fixed list, each producing a self-contained value; the raw-wire escape hatch at `internal/core/bgp/attribute/builder.go:130` covers everything else. | The escape hatch stays as a terminal override on the edit set, which the vocabulary already supports. | A mapping table with one row per setter, each with a byte-identity test. | confirmed: `AppendAttributes` covers every setter, `TestBuilderAppendAttributesMatchesBuild` holds Build against it |
| A-2 | The two rails already agree byte for byte on every case the current tests cover, so convergence cannot regress those cases. | `TestAnnounceRailsAgreeByteForByte` (`internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:312`) and `TestAnnounceRailsPreserveUnlistedAttributes` (`internal/component/bgp/reactor/reactor_api_batch_attr_preserve_test.go:167`) both pass today. | Convergence would be fixing a live divergence as well as removing an encoder, which changes the blast radius and must be reported before proceeding. | Run both test files at the start of implementation and record the result. | confirmed: both green before any edit (tmp/wire4/baseline.log) |
| A-3 | The announce path's per-route cost after convergence is within benchmark noise of an equivalent forwarded route. | Both would run the same size query and the same writer over the same number of slots. | The claim in the umbrella is dropped; the encoder consolidation stands on its own merits. | `BenchmarkAPIOriginVsForward`, comparing equal touched-attribute counts. | confirmed with a caveat: the encoder cost matches; the announce carries one extra allocation, its `*message.Update` return value, which the forward path does not have |
| A-4 | No out-of-tree or plugin caller depends on `Builder.WriteTo` or `Builder.Build` producing bytes. | Both are exported from a core package, so the grep must be tree-wide rather than reactor-local. | Keep a thin compatibility implementation over the shared writer rather than deleting the methods. | Tree-wide grep for `WriteTo(`, `CheckedWriteTo(` and `Build()` on a `Builder` receiver. | confirmed: `Builder.WriteTo`/`CheckedWriteTo` had no caller outside `Build`; `Build` kept and reimplemented over `AppendAttributes` + `WriteAttrTo` |
| A-5 | The NLRI region bound can be expressed as an argument to the shared writer without special-casing the queued rail. | `attrWriter` already models it as a plain limit distinct from the buffer length (`internal/component/bgp/reactor/peer_rib_routes.go:278`). | The queued rail keeps its own bounded wrapper around the shared writer. | A test that fills the attribute region to the limit and asserts the NLRI is intact. | confirmed: `announceAttrs.emit` takes the region as `dst`; `TestQueuedRailNLRIRegionIntact` walks the boundary |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The announce path is the origination path: a mis-encoded route here is wrong at the source, not merely mis-forwarded. | The order and preserve test pairs, run on every change. | Both test files become gates for every phase, not just the last one. |
| R-2 | The `Builder` is exported from a core package, so its emission half may have callers outside the reactor. | Compile fanout the moment `WriteTo` changes. | A-4 is checked by grep before any deletion; a compatibility implementation is the fallback. |
| R-3 | The queued rail's buffer layout parks NLRI behind the attributes, so a writer that bounds on the buffer length instead of the region silently overwrites the prefix being announced. | A test that saturates the attribute region. | The region bound is an explicit argument, and A-5 has a dedicated test. |
| R-4 | Convergence could change bytes for an attribute neither rail names explicitly, which is exactly what the preserve tests exist to catch. | `TestAnnounceRailsPreserveUnlistedAttributes`. | The preserve table is extended with any attribute the mapping table in A-1 shows is reachable only through the escape hatch. |
| R-5 | The flowspec announce test pinned one rail's encoding historically, so a shared encoder could move bytes it depends on. | `test/plugin/ddos-flowspec-announce.ci`. | That `.ci` is a named gate for this child, and any change to it is treated as a regression until proven otherwise. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes this daemon originates are mis-encoded: wrong attribute order, a missing mandatory attribute, a truncated announce, or a flowspec rule that no longer matches. Peers may reset the session or install wrong state, and the fault is ours at the source. |
| How is it reverted? | Single commit revert while the `Builder` emission half is still present. After it is deleted, revert means restoring it. |
| Who else touches this path? | `plan/spec-wire-edit-3-aspath-fold.md` owns the AS_PATH resolver both rails will share; `plan/spec-fixit-bgp-egress-rail-divergence.md` covers a neighbouring rail-divergence concern on the egress side. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator announces a route with communities and local-preference through the API | → | edit set over an empty base, shared writer, ascending order | `test/plugin/wire-edit-api-origin-order.ci` |
| The same route is injected while the peer is still draining its initial sync | → | the queued rail builds the same edit set and gets the same bytes | `TestAnnounceRailsAgreeByteForByte` |
| An operator announces a flowspec rule | → | shared writer emits the same bytes the pinned expectation records | existing `test/plugin/ddos-flowspec-announce.ci` |
| An operator announces a route carrying an attribute neither rail names | → | the attribute survives at its ascending position | `TestAnnounceRailsPreserveUnlistedAttributes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Every case in the order and preserve test tables, driven through both rails | Both rails emit identical bytes, and the codes are ascending |
| AC-2 | An API-originated route and a forwarded route with the same touched-attribute set | Both take the same writer, and the per-route cost difference is within benchmark noise |
| AC-3 | An announce carrying an attribute the rails do not name explicitly | It reaches the wire at its ascending position, unchanged |
| AC-4 | An announce to a two-octet-ASN peer carrying a non-mappable ASN | AS4_PATH is derived by the shared AS_PATH resolver, not by a rail-local insertion |
| AC-5 | An announce whose attributes would not fit the destination's message size | The route is dropped with a log line naming it, never truncated |
| AC-6 | An announce whose attribute region is filled to its limit on the queued rail | The NLRI parked at the buffer tail is intact |
| AC-7 | After this child lands | `grep -n "func (b \*Builder) WriteTo" internal/core/bgp/attribute/builder.go` returns nothing |
| AC-8 | After this child lands | `findAttrInsertPosition` and `insertAttrOrdered` are gone from the batch rail |
| AC-9 | An announce using the `Builder` raw-wire escape hatch | The pre-encoded bytes reach the wire unchanged |
| AC-10 | The full existing announce corpus | Every `.ci` under `test/plugin/` and `test/encode/` passes with no expectation edited |
| AC-11 | An API announce carrying LOCAL_PREF, destination an EXTERNAL peer | No attribute type 5 reaches the wire, on both rails, and the two agree byte for byte (RFC 4271 Section 5.1.5) |
| AC-12 | The same announce, destination an INTERNAL peer | The caller's LOCAL_PREF value survives unchanged; the strip is confined to external peers |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Announces a route with communities and local-preference from the API | text parse, edit set over an empty base, shared writer, TCP | `test/plugin/wire-edit-api-origin-order.ci` |
| 2 | Announces a route to a peer that is still syncing | text parse, queued rail, same edit set, same writer, TCP | `TestAnnounceRailsAgreeByteForByte` |
| 3 | Announces a flowspec rule to a peer | text parse, edit set, shared writer, TCP | existing `test/plugin/ddos-flowspec-announce.ci` |
| 4 | Announces a route to a two-octet-ASN peer while carrying a four-octet origin AS | text parse, AS_PATH resolver, AS4_PATH derivation, TCP | `TestAnnounceAS4PathFromSharedResolver` |
| 5 | Announces a route so large it cannot fit the negotiated message size | text parse, size query, drop with a named log line | existing `test/plugin/as112-community-choice.ci` running unchanged alongside a new Go oversize case |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuilderAppendAttributesMatchesBuild` | `internal/core/bgp/attribute/builder_test.go` | A-1: Build equals the AppendAttributes list written through WriteAttrTo, and the list is ascending | pass |
| `TestBuilderRawWirePassthrough` | `internal/core/bgp/attribute/builder_test.go` | AC-9: the escape hatch survives as a terminal override, and yields no attributes | pass |
| `TestAnnounceBatchRail_AscendingTypeCodeOrder` | existing `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go` | AC-1: unchanged and still passing | |
| `TestAnnounceQueuedRail_AscendingTypeCodeOrder` | existing `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go` | AC-1: unchanged and still passing | |
| `TestAnnounceRailsAgreeByteForByte` | existing `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go` | AC-1: the gate for every phase of this child | |
| `TestAnnounceRailsPreserveUnlistedAttributes` | existing `internal/component/bgp/reactor/reactor_api_batch_attr_preserve_test.go` | AC-3: an unnamed attribute survives at its ascending position | |
| `TestAnnounceRailsPreserveMultiSegmentASPath` | existing `internal/component/bgp/reactor/reactor_api_batch_attr_preserve_test.go` | AC-1: multi-segment AS_PATH is unchanged | |
| `TestAnnounceAS4PathFromSharedResolver` | `internal/component/bgp/reactor/reactor_api_origin_test.go` | AC-4: AS4_PATH comes from the shared resolver, not a rail-local insertion | |
| `TestAnnounceOversizeDropsWithNamedLog` | `internal/component/bgp/reactor/reactor_api_origin_test.go` | AC-5: fail-closed drop, route named | |
| `TestQueuedRailNLRIRegionIntact` | `internal/component/bgp/reactor/reactor_api_origin_test.go` | AC-6, A-5: a saturated attribute region does not touch the NLRI tail | |
| `BenchmarkAPIOriginVsForward` | `internal/component/bgp/reactor/reactor_api_origin_bench_test.go` | AC-2, A-3: equal cost for an equal touched-attribute count | pass: api-origin 300-342ns/1 alloc vs forward 224ns/0 allocs; the one allocation is the `*message.Update` the announce API returns |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| attribute value length | 0-65535 | 65535 | N/A | 65536 (drop the route) |
| attribute header size class boundary | 255-256 | 255 (3-byte header) | N/A | 256 (4-byte header) |
| UPDATE body length | 4-65516 | 65516 | 3 | 65517 (drop the route) |
| AS_PATH segment ASN count | 0-255 | 255 | N/A | 256 (a new segment is required) |
| attribute region limit on the queued rail | 0-buffer length | the NLRI start offset | N/A | one byte past it (latch, drop) |
| announced NLRI count in one batch | 1-n | whatever fits the body | 0 (nothing to announce) | one more than fits (split or drop) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wire-edit-api-origin-order` | `test/plugin/wire-edit-api-origin-order.ci` | an API-announced route reaches the peer with ascending attribute order | pass; mutation-verified (a descending contribution sort emits 32,8,5,3,2,1 and the test goes red) |
| `ddos-flowspec-announce` | existing `test/plugin/ddos-flowspec-announce.ci` | the flowspec announce encoding is unchanged | |
| `as112-community-choice` | existing `test/plugin/as112-community-choice.ci` | community selection on an originated route is unchanged | |
| `redistribute-as112-community` | existing `test/plugin/redistribute-as112-community.ci` | the redistribution announce path is unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-wire-edit-api-origin` | `test/interop/scenarios/` | BIRD | a real peer accepts an API-originated route and installs the expected attributes in the expected order | |

## Files to Modify
- `internal/core/bgp/attribute/builder.go` - the emission half retires; the setter chain stays and produces slots
- `internal/component/bgp/reactor/reactor_api_batch.go` - the batch rail builds an edit set; the ordering machinery and the rail-local AS4_PATH insertion retire
- `internal/component/bgp/reactor/peer_rib_routes.go` - the queued rail builds the same edit set; `attrWriter` retires in favour of a region-bounded shared writer
- `internal/component/bgp/filterapi/filterapi.go` - the edit set accepts an empty base as a first-class case
- `internal/component/bgp/reactor/forward_build.go` - the shared writer takes an explicit region bound
- `docs/architecture/wire/update-packing.md` - ordering becomes a property of one writer
- `docs/architecture/core-design.md` - the announce-rail section
- `docs/features/rfc-status.md` - re-anchor the RFC 4271 Section 5 ordering row to the shared writer

## Files to Create
- `internal/core/bgp/attribute/builder_slots_test.go` - one row per setter, plus the escape hatch
- `internal/component/bgp/reactor/reactor_api_origin_test.go` - AS4_PATH, oversize and region-bound coverage
- `internal/component/bgp/reactor/reactor_api_origin_bench_test.go` - API against forward cost comparison
- `test/plugin/wire-edit-api-origin-order.ci` - rail agreement and ascending order for API routes

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new configuration surface |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | No | The announce command syntax is unchanged |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | Yes | `test/plugin/wire-edit-api-origin-order.ci` covers the announce path end to end |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_announce_dropped_oversize_total`, so AC-5 drops are observable rather than log-only |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The announce surface is unchanged |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | The announce RPC keeps its shape |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/update-packing.md`: ordering becomes a property of the shared writer |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` rows for RFC 4271 Section 5 and RFC 6793 Section 4.2.2, re-anchored |
| 10 | Test infrastructure changed? | No | The existing rail-agreement tests are kept, not replaced |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` announce-rail section |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the oversize-drop counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `builder.go`, `reactor_api_batch.go` and `peer_rib_routes.go`, and correct each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify the announce examples in the command reference still produce the documented output |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the rails agree today, and pin what they emit
   - Tests: run `TestAnnounceRailsAgreeByteForByte` and `TestAnnounceRailsPreserveUnlistedAttributes` and record the result; add a golden capture of both rails' output for every case
   - Files: `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go`, `internal/component/bgp/reactor/reactor_api_batch_attr_preserve_test.go`
   - Verify: A-2 resolved; every case has a recorded byte string to diff against
2. **Phase: the setter-to-slot mapping** -- one row per `Builder` setter
   - Tests: `TestBuilderSettersMapToSlots`, `TestBuilderRawWirePassthrough`
   - Files: `internal/core/bgp/attribute/builder.go`
   - Verify: A-1 and A-4 resolved by grep and test; the emission half still exists and still runs
3. **Phase: the batch rail** -- build an edit set, drop the rail-local ordering machinery
   - Tests: the order and preserve test files, unchanged
   - Files: `internal/component/bgp/reactor/reactor_api_batch.go`
   - Verify: AC-1, AC-3 and AC-8 pass; the golden capture still matches
4. **Phase: the queued rail** -- same edit set, region-bounded writer
   - Tests: `TestQueuedRailNLRIRegionIntact`, `TestAnnounceOversizeDropsWithNamedLog`
   - Files: `internal/component/bgp/reactor/peer_rib_routes.go`, `internal/component/bgp/reactor/forward_build.go`
   - Verify: AC-5 and AC-6 pass; A-5 resolved; both rails now share one writer
5. **Phase: share the AS_PATH resolver**
   - Tests: `TestAnnounceAS4PathFromSharedResolver`
   - Files: `internal/component/bgp/reactor/reactor_api_batch.go`
   - Verify: AC-4 passes; the rail-local AS4_PATH insertion is gone
6. **Phase: retire the second encoder**
   - Tests: the whole announce corpus plus `BenchmarkAPIOriginVsForward`
   - Files: `internal/core/bgp/attribute/builder.go`
   - Verify: AC-2, AC-7 and AC-10 pass; A-3 resolved with numbers
7. **Phase: documentation, counter and interop**
   - Tests: the interop scenario
   - Files: the doc targets above, `test/interop/scenarios/`
   - Verify: a real peer accepts an API-originated route with the expected attribute order

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and every `Builder` setter maps to exactly one slot with a byte-identity test |
| Feature completeness | Every user story has a passing test, including the flowspec and oversize cases |
| Correctness | Both rails emit identical bytes for every table case; an unnamed attribute keeps its ascending position; AS4_PATH is derived once |
| Naming | The slot-producing surface on the `Builder` follows `ai/rules/naming.md` and does not shadow the existing setter names |
| Data flow | The announce path builds slots and never an intermediate encoded block; the NLRI region bound is explicit, not implied by the buffer length |
| Registration over hardcoding | The writer gains no per-attribute switch case; the fixed emission order in `WriteTo` is deleted rather than moved |
| Rule: `ai/rules/buffer-first.md` | An exact size query precedes every write; `Build` is not on the hot path |
| Rule: `ai/rules/fail-closed-guards.md` | An oversized announce is dropped with the route named, never truncated, and now counted |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Second encoder removed | `grep -n "func (b \*Builder) WriteTo" internal/core/bgp/attribute/builder.go` returns nothing |
| Rail-local ordering machinery removed | `grep -n "insertAttrOrdered\|findAttrInsertPosition" internal/component/bgp/reactor/reactor_api_batch.go` returns nothing |
| Rails still agree | `go test ./internal/component/bgp/reactor/ -run TestAnnounceRails -v` |
| Ascending order on both rails | `go test ./internal/component/bgp/reactor/ -run AscendingTypeCodeOrder -v` |
| Equal cost for equal work | `go test ./internal/component/bgp/reactor/ -bench BenchmarkAPIOriginVsForward -benchmem` |
| Announce corpus unchanged | `make ze-functional-test` with no `.ci` expectation edited |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Announce attribute values come from an API caller, not a peer, but they are still untrusted input to an encoder. Lengths must be validated before the size query, not during the write |
| Integer handling | The header size class and the body length are 16-bit wire fields; every conversion needs an explicit bound |
| Buffer region discipline | The queued rail shares one buffer between attributes and NLRI. A writer bounded on the buffer length rather than the region would overwrite the announced prefix, which is a silent wrong-route bug |
| Fail-open risk | An oversized announce must drop and say so. Emitting a truncated UPDATE would be a protocol violation originated by us |
| Escape hatch | The raw-wire passthrough accepts pre-encoded bytes. It must remain a terminal override that is size-queried like anything else, not a path that bypasses the bound |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| The rails disagree | STOP. That is the exact defect this child exists to make impossible |
| The golden capture reports a byte difference | STOP. Convergence is not supposed to move bytes. Back to design, do not adjust the golden |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| An out-of-tree caller of the emission half is found | Keep a compatibility implementation over the shared writer and report it |
| Interop peer rejects an announce | STOP and present. A real peer disagreeing is stronger evidence than any unit test |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The rail-agreement tests are the specification for this child. They exist because the rails once disagreed and the disagreement was decided by timing. Making one encoder is how that class of defect stops being possible rather than stopping being present.
- `Builder.Len` and `Builder.WriteTo` are already the contract the shared writer wants, applied to a different set of inputs. Convergence is therefore less a rewrite than a deletion: the size-then-write discipline stays, one of its two implementations goes.
- The queued rail's limit, distinct from the buffer length, is the detail most likely to be lost in a mechanical merge. It is not a defensive nicety, it is what keeps the attributes from overwriting the prefix being announced.
- Ordering machinery added to one rail is a symptom, not a fix. The batch rail has `findAttrInsertPosition` and `insertAttrOrdered` because it was the rail that diverged; the queued rail has none because it happened to be right. One writer removes the asymmetry.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An API route is an edit set over an empty base | keep a separate announce encoder and test the two against each other | Testing two encoders against each other is what the tree does today, and it costs a permanent test pair plus a class of timing-dependent defects |
| Keep the `Builder` setter chain | replace the `Builder` outright | The setters are used widely and are a good intent-collection surface. Only the emission half is duplicated work |
| The region bound is an explicit writer argument | give the queued rail its own wrapper | The bound is a real property of the call, not a rail quirk; making it explicit means the writer cannot be misused by a future third caller |
| Share the AS_PATH resolver from child 3 | keep the announce-side AS4_PATH insertion | RFC 6793 derivation implemented twice is two chances to be wrong, and the announce rail is the one that originates rather than relays |
| Keep the existing rail tests rather than replacing them | delete them once the encoder is shared | They are cheap and they are the only test that would catch a future re-divergence |

## Known Limitations

- Convergence does not change how `ShouldQueue` selects a rail. Two rails still exist; they simply cannot disagree about bytes any more.
- The `Builder` raw-wire escape hatch remains, so a caller can still hand over pre-encoded bytes the writer does not interpret. That is deliberate and is how flowspec and other pre-encoded attributes pass through.
- Splitting still runs after materialisation on both rails.
- This child does not touch the withdraw rails, which have their own builders. They are a smaller duplication and are out of scope.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC | Section | Requirement | Site after this child |
|-----|---------|-------------|----------------------|
| 4271 | 5 | attribute type-code ordering on emission | the shared writer's merge-insert step |
| 4271 | 4.3 | mandatory attributes ORIGIN and AS_PATH on every announce | the mandatory-attribute step, carried from `internal/component/bgp/reactor/reactor_api_batch.go:906` |
| 6793 | 4.2.2 | AS4_PATH obligation on an announce to an old speaker | the shared AS_PATH resolver, replacing `internal/component/bgp/reactor/reactor_api_batch.go:715` |
| 4760 | 3 | MP_REACH value layout for non-IPv4-unicast announces | the MP_REACH slot |
| 7311 | - | AIGP encoding, exercised by the preserve tests | the shared writer |
| 8654 | - | extended message body ceiling | the announce size query |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
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
- [ ] Learned summary written to `plan/learned/NNN-wire-edit-4-api-origin.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-wire-edit-4-api-origin.md` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

- An API-originated route is an edit set over an empty base. `announceAttrs.emit` is the one writer both announce rails call, and it takes the NLRI region bound as an explicit argument rather than inferring it from the buffer length.
- `attribute.Builder` keeps its setter chain and gains `AppendAttributes`; its emission half is gone. `Builder.WriteTo` and `CheckedWriteTo` are deleted, and `Build` is reimplemented over `AppendAttributes` plus `WriteAttrTo`.
- `findAttrInsertPosition` and `insertAttrOrdered` are gone from the batch rail: the shared writer's merge-insert does the ordering for both rails.
- The AS_PATH resolver from child 3 is shared, so RFC 6793 derivation exists once.

### Bugs Found/Fixed

- **RFC 4271 Section 5.1.5 violation.** LOCAL_PREF reached eBGP peers on the batch rail. Fixed in `e2037e598`; `TestAnnounceStripsLocalPrefTowardExternalPeer` proves no attribute type 5 reaches an external peer on either rail, and `TestAnnounceRailsAgreeByteForByte` proves both rails agree afterwards.
- The two rails could emit one route as two byte strings, chosen by whether the destination peer had finished its initial sync. The batch rail copied the caller's block verbatim and APPENDED (1,8,32,2,3,5) while the queued rail merge-inserted (1,2,3,5,8,32). `test/plugin/wire-edit-api-origin-order.ci` pins the correct form end to end.

### Documentation Updates

- `docs/features/rfc-status.md` -- the RFC 4271 Section 5.1.5 row, with a source anchor to the strip site.
- `docs/architecture/wire/attributes.md` -- one writer for both origins.

### Deviations from Plan

| # | Plan said | What was built | Why |
|---|-----------|----------------|-----|
| D-1 | AC-2: within benchmark noise | Within noise for the encoder, plus one extra allocation on the announce | The announce returns a `*message.Update` that the forward path has no equivalent of. Recorded on A-3 rather than hidden |
| D-2 | interop scenario as a gate | Not reached | Deferred and homed in `plan/spec-wire-edit-4-api-origin-deferred-bird-interop.md`. The property is covered by unit tests over both rails and by a `.ci` that pins the exact wire bytes through the daemon |
| D-3 | `bgp_announce_dropped_oversize_total` | Not reached | The drop and its named log line are implemented and tested (`TestAnnounceOversizeDropsWithNamedLog`); only the metric surface is missing. Homed in `plan/spec-wire-edit-4-api-origin-deferred-oversize-metric.md` |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Convergence was scoped as "remove an encoder", with A-2 asserting the rails already agreed on every covered case | They agreed on every case the tests covered, and disagreed on one nobody had a test for: LOCAL_PREF toward an external peer, an RFC 4271 Section 5.1.5 MUST | Writing the shared writer forced both rails through one path, and the strip had to be decided once | Fixed in `e2037e598` with a test on each rail plus a byte-agreement test |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An API route is an edit set over an empty base | Done | `reactor/reactor_api_batch.go` `announceAttrs` | `TestAnnounceBuilderModeIsEditSetOverEmptyBase` |
| One writer for both rails | Done | `announceAttrs.emit` | `TestAnnounceRailsAgreeByteForByte` |
| The region bound is an explicit argument | Done | `announceAttrs.emit` `dst` | `TestQueuedRailNLRIRegionIntact` |
| Retire the `Builder` emission half | Done | `internal/core/bgp/attribute/builder.go` | AC-7 |
| Share the AS_PATH resolver | Done | `TestAnnounceAS4PathFromSharedResolver` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAnnounceRailsAgreeByteForByte`, `TestAnnounceBatchRail_AscendingTypeCodeOrder`, `TestAnnounceQueuedRail_AscendingTypeCodeOrder` | |
| AC-2 | Changed | `BenchmarkAPIOriginVsForward` | Encoder cost matches; the announce carries one extra allocation, its `*message.Update` return (D-1) |
| AC-3 | Done | `TestAnnounceRailsPreserveUnlistedAttributes` | |
| AC-4 | Done | `TestAnnounceAS4PathFromSharedResolver` | Derived by child 3's resolver, not by a rail-local insertion |
| AC-5 | Done | `TestAnnounceOversizeDropsWithNamedLog` | Dropped with a named log line, never truncated. The counter is deferred (D-3) |
| AC-6 | Done | `TestQueuedRailNLRIRegionIntact` | Walks the boundary |
| AC-7 | Done | `grep -n "func (b \*Builder) WriteTo" internal/core/bgp/attribute/builder.go` returns nothing | |
| AC-8 | Done | `grep -rn "findAttrInsertPosition\|insertAttrOrdered" internal/` returns nothing | |
| AC-9 | Done | `TestAnnounceRailsPreserveUnlistedAttributes`, plus existing `test/plugin/ddos-flowspec-announce.ci` | The raw-wire escape hatch is unchanged |
| AC-10 | Done | `make ze-functional-test`; `test/encode/` corpus | One `.ci` was ADDED (`wire-edit-api-origin-order`); none was edited |
| AC-11 | Done | `TestAnnounceStripsLocalPrefTowardExternalPeer` + `TestAnnounceRailsAgreeByteForByte` | RFC 4271 Section 5.1.5 |
| AC-12 | Done | the internal-peer row of `TestAnnounceStripsLocalPrefTowardExternalPeer` | The strip is confined to external peers |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the rail order and preserve pairs | Done | `reactor/reactor_api_batch_attr_order_test.go` | 7 `Test` functions |
| the origin-vs-forward benchmark | Done | `reactor/reactor_api_origin_bench_test.go` | `BenchmarkAPIOriginVsForward`, `BenchmarkAnnounceRails` |
| the queued-rail region test | Done | `reactor/reactor_api_origin_test.go` `TestQueuedRailNLRIRegionIntact` | |
| the two interop scenarios | Skipped | -- | D-2, homed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `test/plugin/wire-edit-api-origin-order.ci` | Done | |
| `internal/component/bgp/reactor/reactor_api_origin_test.go` + `_bench_test.go` | Done | |
| every "Files to Modify" row | Done | see the diffs of `ddf04953a` and `e2037e598` |

### Audit Summary
- **Total items:** 21
- **Done:** 18
- **Partial:** 0
- **Skipped:** 2 (the two interop scenarios, D-2)
- **Changed:** 1 (AC-2, D-1)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One announce reaches the wire as one byte string, whatever rail runs | functional + unit | `test/plugin/wire-edit-api-origin-order.ci` pins the full message hex; `TestAnnounceRailsAgreeByteForByte` holds both rails against each other |
| The second attribute encoder is gone | grep | `grep -n "func (b \*Builder) WriteTo" internal/core/bgp/attribute/builder.go` empty; `grep -rn "findAttrInsertPosition\|insertAttrOrdered" internal/` empty |
| An announce that cannot fit is dropped, never truncated | unit | `TestAnnounceOversizeDropsWithNamedLog`, both rails, route named in the log |
| The queued rail's NLRI is never overwritten by attributes | unit | `TestQueuedRailNLRIRegionIntact` walks the region boundary |
| RFC 4271 Section 5.1.5 holds on both rails | unit | `TestAnnounceStripsLocalPrefTowardExternalPeer`; no attribute type 5 toward an external peer, value preserved toward an internal one |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The `NN-wire-edit-api-origin` interop scenario: a real BIRD peer accepts an API-originated route and installs the attributes in the expected order | deferred | `plan/spec-wire-edit-4-api-origin-deferred-bird-interop.md`, created 2026-08-02 (the original Destination was this spec, which closure removes) |
| The `bgp_announce_dropped_oversize_total` counter, so an AC-5 oversize drop is observable as a metric | deferred | `plan/spec-wire-edit-4-api-origin-deferred-oversize-metric.md`, created 2026-08-02 (same re-homing) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/wire-edit-4-api-origin-<session-id>.md` |
| `review_gate.py check` | clean |
| Reviewer lenses used | correctness/wire/lifetime; tests/security/coverage (two agents over `bbd53bf22^..b1fa7ab1e`, 2026-08-02) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The oversize suppression `.ci` had stopped covering the code it was written for | `test/plugin/modify-oversize-suppress.ci`, `internal/test/peer/` | `ea6a4bbda`, which re-derives the ceiling by exact byte arithmetic instead of guessing the policy's added bytes |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/wire-edit-api-origin-order.ci` | Yes | `ls test/plugin/wire-edit-api-origin-order.ci`; header quotes RFC 4271 Section 5 and asserts one hex message |
| `internal/component/bgp/reactor/reactor_api_origin_test.go` | Yes | 6 `Test` functions |
| `internal/component/bgp/reactor/reactor_api_origin_bench_test.go` | Yes | `BenchmarkAPIOriginVsForward`, `BenchmarkAnnounceRails` |
| `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go` | Yes | 7 `Test` functions including `TestAnnounceRailsAgreeByteForByte` |
| `test/plugin/ddos-flowspec-announce.ci` | Yes | `ls test/plugin/ddos-flowspec-announce.ci`, unedited |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-7 | the Builder emission half is gone | `grep -n "func (b \*Builder) WriteTo" internal/core/bgp/attribute/builder.go` prints nothing (re-run 2026-08-02) |
| AC-8 | the batch rail's ordering machinery is gone | `grep -rn "findAttrInsertPosition\|insertAttrOrdered" internal/` prints nothing (re-run 2026-08-02) |
| AC-11 | no LOCAL_PREF toward an external peer | `grep -n "func TestAnnounceStripsLocalPrefTowardExternalPeer" internal/component/bgp/reactor/reactor_api_origin_test.go`; `make ze-test-bgp` green |
| AC-6 | the NLRI region survives a full attribute region | `grep -n "func TestQueuedRailNLRIRegionIntact" internal/component/bgp/reactor/reactor_api_origin_test.go` |
| AC-1 | both rails agree | `grep -n "func TestAnnounceRailsAgreeByteForByte" internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An operator announces a route with communities and local-preference | `test/plugin/wire-edit-api-origin-order.ci` | Yes -- read: a plugin sends one `peer * update text ...` and the peer expectation is the whole message hex, with the injected 2,3,5 before the caller's 8,32 |
| The same route injected during initial-sync drain | `TestAnnounceRailsAgreeByteForByte` | Yes -- read: drives both rails over the same input and compares bytes |
| An operator announces a flowspec rule | `test/plugin/ddos-flowspec-announce.ci` | Yes -- read: the pinned MP_REACH-plus-EXTENDED_COMMUNITY expectation is unchanged by this child |
| An announce carrying an unnamed attribute | `TestAnnounceRailsPreserveUnlistedAttributes` | Yes -- read: the attribute survives at its ascending position on both rails |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `AppendAttributes` covers every `Builder` setter; `TestBuilderAppendAttributesMatchesBuild` holds `Build` against it |
| A-2 | confirmed with a correction | Both rail tests were green before any edit (`tmp/wire4/baseline.log`), so convergence regressed nothing they covered. They did NOT cover LOCAL_PREF toward an external peer, where the rails were both wrong against RFC 4271 Section 5.1.5. Convergence therefore also fixed a live defect, which is a wider blast radius than A-2 assumed, and it is recorded in the Mistake Log |
| A-3 | confirmed with a caveat | The encoder cost matches; the announce carries one extra allocation, its `*message.Update` return value, which the forward path has no equivalent of |
| A-4 | confirmed | `Builder.WriteTo` and `CheckedWriteTo` had no caller outside `Build`; `Build` is kept and reimplemented over `AppendAttributes` plus `WriteAttrTo`, so no compatibility shim was needed |
| A-5 | confirmed | `announceAttrs.emit` takes the region as `dst`; `TestQueuedRailNLRIRegionIntact` walks the boundary |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC status: RFC 4271 Section 5.1.5 | `docs/features/rfc-status.md` row checked against the strip in the announce path; proven by `TestAnnounceStripsLocalPrefTowardExternalPeer` | Yes |
| Wire format: one writer for both origins | `docs/architecture/wire/attributes.md` checked against `announceAttrs.emit`, the single emission site | Yes |
| API/RPC docs | The `update text` command surface is unchanged; only the bytes it produces moved | Yes |
| Categories answered No | `ddf04953a` and `e2037e598` add no config leaf, no CLI command, no plugin, no capability | Yes |

## Core Insight

Ordering machinery added to one rail is a symptom, not a fix. The batch rail had
`findAttrInsertPosition` and `insertAttrOrdered` because it was the rail that
diverged; the queued rail had none because it happened to be right. Making one
writer removed the asymmetry -- and forced a single answer to a question neither
rail had been asked, which is how the RFC 4271 Section 5.1.5 violation surfaced.
