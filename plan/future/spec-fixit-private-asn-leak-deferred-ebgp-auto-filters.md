# Spec: fixit-private-asn-leak-deferred-ebgp-auto-filters

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - (but SEQUENCES against three in-progress specs -- see "Convergence") |
| Phase | - |
| Updated | 2026-08-14 |

## Provenance

Reclassified as an improvement on 2026-08-14 at Thomas's instruction and moved
from `plan/` to `plan/future/`. Reason: RFC 4271 Section 5.1.5 is already
enforced on the forwarded egress path by `applyFactsLocalPref`
(`internal/component/bgp/reactor/forward_local_pref.go`), which both forward
rails call (`reactor_api_forward.go`, `forward_rs.go`). What remains is whether
the two behaviours become visible auto-added export-chain entries, which is a
config-surface preference and not a conformance question.

Update (2026-07-22 plan review): the Phase 3-4 sequencing gate has mostly
cleared -- three of the four named in-flight specs LANDED
(fixit-as4path-missing-on-rewrite learned 1238, fixit-tombstone-ebgp-transitive
learned 1239, parent fixit-private-asn-leak learned 1231). Only
`spec-perf-next-1-ebgp-wire-lockfree` remains open, and that one is itself
complete-in-code awaiting closure (learned 900), so the prepend half is
effectively unblocked. The Phase 1-2 LOCAL_PREF half is un-landed and
implementable now (`test/plugin/ebgp-localpref-egress-strip.ci` absent, no <!-- doc-links: ignore (test this spec proposes; not written) -->
`auto:ebgp-*` reserved names in `internal/`).

> **Readiness pass 2026-07-17:** design filled from skeleton; every placeholder replaced,
> and R-2/R-3/R-4 + the FilterRef-marker open question resolved append-only (see
> "### Resolutions (readiness pass)" under Risks & Assumptions). ~~Status stays `skeleton`
> because the sequencing call against three in-progress specs is a genuine HARD BLOCKER
> reserved for Thomas (see "### Resolutions"). That sequencing call is now the ONLY
> remaining gate.~~
>
> **Readiness pass 2026-07-17 (sequencing resolved, Thomas-authorized):** Thomas authorized
> resolving the sequencing blocker. Status → `ready`. FULL SCOPE retained — no AC dropped.
> The blocker is re-expressed as an ORDERING CONSTRAINT, not a scope cut: the LOCAL_PREF-
> suppress half (Phases 1-2, AC-1/AC-2) is implementable NOW; the prepend-relocation half
> (Phases 3-4, AC-3/AC-4/AC-5/AC-8), which retires `wireu.rewriteASPathPrepend`, MUST NOT
> START until the four specs editing that seam land (see the "Phase 3-4 Sequencing Gate"
> under Implementation Phases and the "→ SEQUENCING RESOLUTION" under Risks & Assumptions).
> Because this is an ordering constraint, not a design ambiguity, the spec is ready for its
> Phase 1-2 work with Phases 3-4 gated.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/config/peers.go` - `prependDefaultFilters`, the precedent this design copies
4. `internal/component/bgp/reactor/filter_ordered.go` - `runEgressPolicyChainASN4`, the one shared chain body
5. `internal/component/bgp/reactor/reactor_api_forward.go` - the ONLY consumer of `orderedEgressSteps`, and why it is the wrong hook
6. `internal/component/bgp/wireu/aspath_rewrite.go` - `rewriteASPathPrepend`, the seam three other specs are already editing

## Task

Give every EBGP peer two **auto-added** entries on its export filter chain:

1. **prepend our ASN when sending** (RFC 4271 §9.1.2)
2. **remove LOCAL_PREF** (RFC 4271 §5.1.5 MUST NOT)

instead of special-casing both inside the wire path.

**Provenance:** Thomas's design ruling, 2026-07-16, verbatim: *"we should have an
'auto-added' filter added to the chain of ebgp peer to add our ASN to the peer when sending
and another one to remove local pref"*. Given in answer to the EBGP-prepend question that
`plan/spec-fixit-private-asn-leak.md` flagged and deliberately did not answer <!-- doc-links: ignore (parent spec closed and removed) -->
("Flagged for Thomas"; also listed as an approved open item at `:281` and `:409`).

## RE-SCOPED 2026-08-05: the RFC violation is CLOSED. Only the architecture is left.

The section below says RFC 4271 Section 5.1.5 "is not enforced on the forwarded
egress path at all", and gives the grep that proved it. **That is no longer
true.** Re-measured today:

- The spec's own grep, `grep -i localpref internal/component/bgp/wireu/*.go`,
  still returns nothing, but the enforcement moved rather than being absent.
  `applyFactsLocalPref` (`internal/component/bgp/reactor/forward_local_pref.go`)
  records `AttrModSuppress` on LOCAL_PREF for every destination where
  `localPrefAllowedTo` says it may not go, which is exactly the EBGP case. It
  guards both the payload the source carried AND an egress filter that SETS the
  attribute on a route whose source had none, through `modsTouchLocalPref`.
- So the IBGP-source-to-EBGP-destination leak this spec was written around is
  gone.

**Both halves of Thomas's ruling are also already implemented as mod operations,
just not as visible chain entries.** The AS-path prepend is a filter action,
`faASPathPrepend` spelled `as-path-prepend`
(`internal/component/bgp/reactor/filter_chain.go`), and
`ExtractASPathPrependOps` puts it into the export mods
(`internal/component/bgp/reactor/filter_ordered.go`). The LOCAL_PREF removal is
the suppress op above.

### What actually remains

The ruling asked for "an 'auto-added' filter added to the chain of ebgp peer".
Today the two behaviours are applied by the reactor's forward-facts logic, and
`prependDefaultFilters` (`internal/component/bgp/config/peers.go`) still appends
to `ps.ImportFilters` alone: nothing is auto-added to an export chain, which the
section below records correctly and which is still true.

So the remaining work is a CONFIG SURFACE question, not a correctness one: should
these two become visible auto-added entries on the export chain, so an operator
reading the peer's filters sees them, mirroring the import side? That is worth
doing for legibility, and it is no longer urgent, because nothing is leaking
while it waits.

Status moved to `ready` with that narrower scope. Do not re-open it as an RFC
defect; the RFC half is closed and this note is the evidence.

### ~~This is not only a design preference. One half of it is an unenforced RFC MUST NOT~~ SUPERSEDED

**Every claim in this section is superseded by the RE-SCOPED block above. Struck 2026-08-14 at
the heading, because a reader who lands here can miss a correction that sits further up.** The
table below is a 2026-07-16 reading. `applyFactsLocalPref`
(`internal/component/bgp/reactor/forward_local_pref.go`) enforces Section 5.1.5 on BOTH forward
rails today, and it guards the filter-set case as well as the carried-payload case. The
"KEPT ON THE WIRE" row is FALSE now. Kept for history only.

~~The trace (2026-07-16) found that **RFC 4271 §5.1.5 is not enforced on the forwarded egress
path at all**:~~

| Path | LOCAL_PREF to an EBGP peer | Evidence |
|------|---------------------------|----------|
| Ingress (received from EBGP) | discarded | `message/rfc7606.go` `validateLocalPrefAttr`: `if !isIBGP` -> `RFC7606ActionAttributeDiscard` / `DiscardReasonEBGPInvalid` |
| Origination (`update text` / plugin) | IBGP only | `message/update_build_plugin.go` `if ub.IsIBGP { ... attribute.LocalPref(lp) }`; `:79-81` drops a plugin-supplied raw LOCAL_PREF |
| Origination (rib commit) | IBGP only | `rib/commit.go` `includeLocalPref := c.isIBGP()` |
| **Forwarded (IBGP source -> EBGP destination)** | **KEPT ON THE WIRE** | `grep -i localpref internal/component/bgp/wireu/*.go` -> **zero non-test hits**. The EBGP wire path prepends AS_PATH and never touches LOCAL_PREF. No egress strip anywhere in `reactor/*.go` |

The only reason this usually does not bite is that LOCAL_PREF from an EBGP *source* is
discarded at ingress -- which does not cover **IBGP source -> EBGP destination**. That case
leaks LOCAL_PREF onto the EBGP wire today. Thomas's second auto-filter closes it.
`attribute/builder.go` even carries the comment *"LOCAL_PREF (filtered at send time
for eBGP)"* above a function that appends `LocalPref` unconditionally: the send-time filter
it defers to does not exist.

### The precedent already exists -- on the import side only

`config/peers.go` `prependDefaultFilters` prepends loop-detection entries to
`ps.ImportFilters` unless already present (`filterChainContains`, `:667`). Line `:670` only
ever touches `ImportFilters`. **The export chain is 100% user config today; nothing is
auto-added to it.** This design is `prependDefaultFilters`' mirror image, and should look
like it.

`PeerSettings.IsEBGP()` (`reactor/peer_settings.go`, `LocalAS != PeerAS`) is
available at exactly the config-time point where `prependDefaultFilters` runs
(`config/peers.go`, which receives `map[string]*reactor.PeerSettings`), so an
EBGP-gated auto-append is directly expressible.

### Both filters are expressible in today's vocabulary -- no new ops needed

| Auto-filter | Existing op | Evidence |
|-------------|-------------|----------|
| prepend our ASN | `mods.Op(byte(attribute.AttrASPath), filterapi.AttrModPrepend, buf)` | exactly what `ExtractASPathPrependOps` already emits; it builds an AS_SEQUENCE of N x localAS at `filter_delta.go` |
| remove LOCAL_PREF | `mods.Op(byte(attribute.AttrLocalPref), filterapi.AttrModSuppress, nil)` | `AttrModSuppress` = "Remove entire attribute from UPDATE" (`filterapi/filterapi.go`); `{attribute.AttrLocalPref, 0x40}` already has a registered generic handler (`filter_delta_handlers.go`) |

The filter *text* vocabulary already names both concepts: `faASPathPrepend`
("as-path-prepend", `filter_chain.go`) and `faLocalPreference` ("local-preference",
`:35`/`:58`).

### The hook choice is the crux, and one option is a trap

| Candidate hook | Runs on forwarded path? | Runs on originated/injected path? | Verdict |
|----------------|------------------------|-----------------------------------|---------|
| `filterapi` in-process egress step (`buildOrderedEgressSteps`, `filter_ordered.go`) | YES (`reactor_api_forward.go` is its only consumer) | **NO** | **Trap.** `exportFilterForBody` does not iterate `orderedEgressSteps`; it calls `runEgressPolicyChainASN4` directly (`egress_inject_filter.go`). An auto-filter registered here silently misses every originated route -- the exact class of bug `spec-fixit-private-asn-leak` just fixed |
| A `FilterRef` auto-appended to `ps.ExportFilters` at config time, à la `prependDefaultFilters` | YES | YES | **Preferred.** Both paths reach the one shared body `runEgressPolicyChainASN4` (`filter_ordered.go`), so a chain entry is honored by both automatically |

This is not a new invariant: it is the Goal Gate `spec-fixit-private-asn-leak.md`
already banked -- *"one shared chain body; a future outcome added to the chain is honored by
both paths automatically."* This design is the first thing to cash that in.

**Open design question (do not answer in code):** `FilterRef` is a name reference with no
parameters and no origin marker -- `FilterRef{Name string, Inactive bool}`
(`filterapi/filterapi.go`). There is no "auto" bit. An auto entry must therefore
either reserve a name (and `config/peers.go` `ValidateFilterNames` +
`:200-203` `canonicalizeFilterRefs` must accept it), or `FilterRef` grows a marker. Decide
which, and how `show`/config output renders an entry the operator never typed.

→ AUTONOMOUS DEFAULT (2026-07-17): **reserve a name** (`auto:ebgp-localpref-suppress`,
`auto:ebgp-prepend`), do NOT grow `FilterRef`. Rendered as a system/auto entry, distinct
from user config. Full rationale under "### Resolutions (readiness pass)" (R-3). Thomas:
override if a real `Auto` marker is required.

### BLOCKING FINDING (2026-07-27): the auto-append design has an unpriced hot-path cost

Verified against the tree, not inferred. **`runEgressPolicyChainASN4` returns
`egressStepResult{accept: true}` and does nothing when the peer has no export filters**
(`internal/component/bgp/reactor/filter_ordered.go`, and the same early return in
its wrapper `runEgressPolicyChain` at `:225-227`).

Two consequences, and the second is what blocks:

1. **It explains why Thomas's ruling is the RIGHT shape and the cheap alternative is a
   trap.** The hardcoded `ExtractRemovePrivateASOps` / `ExtractASPathPrependOps` steps
   inside that body therefore never run for a peer with no configured export
   filters. Adding an EBGP LOCAL_PREF strip as another hardcoded step in that body would
   be a guard that fails OPEN for exactly the common case (an EBGP peer with no export
   policy) -- banned by `ai/rules/evidence.md`. Auto-APPENDING a `FilterRef`
   is what makes `len(exportFilters) != 0` hold for every EBGP peer, so the ruling is
   load-bearing, not stylistic.

2. **But that is precisely why it is expensive.** Making the chain non-empty for every
   EBGP peer means every EBGP destination of every forwarded UPDATE now enters the body,
   which allocates a 64 KB stack scratch and renders the whole update to TEXT before any
   filter runs (`filter_ordered.go`: `var scratchArr [65536]byte`, then
   `AppendUpdateForFilter`). Today a no-policy EBGP peer skips all of that. For a daemon
   whose forward rail exists to be zero-copy (`ai/rules/performance.md`), turning
   the text-rendering egress chain on for every EBGP peer by default is a throughput
   change, not an implementation detail. The spec never prices it.

**The RFC violation is real and still live**, so this is a blocker to a fix, not a reason
to leave it: an UPDATE received from an IBGP peer carrying LOCAL_PREF and forwarded to an
EBGP peer keeps LOCAL_PREF on the wire. Producer citations, all re-verified 2026-07-27:
`reactor_api_forward.go` (`wireu.RewriteASPath` is the only per-destination EBGP wire
transform), `grep -rn LOCAL_PREF internal/component/bgp/wireu/*.go` (zero non-test hits,
so nothing strips it), and `message/rfc7606.go` (the ingress discard covers an
EBGP *source* only, never an IBGP source with an EBGP destination). The `rib/commit` and
`peer_rib_routes` paths were checked and are NOT leaking: both gate LOCAL_PREF on
`isIBGP` (`peer_rib_routes.go`, `:133`).

**Needs a ruling from Thomas** (recorded rather than guessed, because the two answers
produce materially different code and one of them contradicts the 2026-07-16 ruling):

| Option | Shape | Cost | Against |
|--------|-------|------|---------|
| A: auto-append `FilterRef` as ruled | one chain entry per EBGP peer | every EBGP destination enters the text-rendering egress body | the forward rail's zero-copy premise |
| B: extend the per-destination EBGP wire rewrite | suppress LOCAL_PREF inside the `RewriteASPath` copy that `reactor_api_forward.go` ALREADY pays for and caches per (localAS, asn4) in `ebgpWireCache` | no new allocation, no new pass | the 2026-07-16 ruling ("instead of special-casing both inside the wire path"), and it does not generalise to the prepend half |

Option B was NOT taken unilaterally. It is cheaper and fixes the MUST NOT, but it
contradicts a recorded ruling given before this cost was known.

### Convergence: three in-progress specs are standing on the seam this design retires

`wireu.rewriteASPathPrepend` (`aspath_rewrite.go`) is the de-facto per-destination EBGP
egress hook today. Moving prepend into the chain changes what it is for. **This is a
sequencing decision, and it belongs to Thomas, not to whoever picks this up:**

| Spec | Status | Its claim on the seam | Conflict |
|------|--------|----------------------|----------|
| `spec-fixit-as4path-missing-on-rewrite` | in-progress | owns the *inside* of `rewriteASPathPrepend` / `aspath_transcode.go` / `aspath_as4.go`: `RewriteASPath`/`RewriteASPathDual` substitute AS_TRANS but never emit the AS4_PATH RFC 6793 §4.2.2 requires | actively editing the function this design would bypass or replace for EBGP prepend |
| `spec-perf-next-1-ebgp-wire-lockfree` | in-progress (1/5) | owns the `ReceivedUpdate.EBGPWire` cache that **memoizes prepend output** (`received_update.go`) | moving prepend into the chain changes what that cache caches, or makes it dead |
| `spec-fixit-tombstone-ebgp-transitive` | in-progress | adds the Section 5.3 Transitive-clear **inside `rewriteASPathPrepend`**, because `attr_discard.go` says the egress rule "is enforced per destination on the EBGP wire path, in `wireu.rewriteASPathPrepend`, not here" | same seam; and its `.ci` ruling is explicitly the interim answer *"until we re-engineer how we deal with attributes"* -- **this spec looks like that re-engineering** |

Prepend also exists in **two independent implementations** already, which is context for why
consolidating into the chain is attractive:

- forwarded: `wireu.rewriteASPathPrepend`, reached only via `RewriteASPath` and `RewriteASPathDual`, all callers gated `facts.isEBGP && !facts.rsClient` (`reactor_api_forward.go`, `forward_rs.go`)
- originated: `rib/commit.go` `buildASPathFromExplicit` -- IBGP preserves as-is, else prepends `c.ctx.LocalASN()` to the first AS_SEQUENCE

**Correction worth carrying:** `spec-fixit-private-asn-leak.md` says "the originated
path does not prepend the local AS for EBGP peers". That is true of the
`update text` / `BuildPlugin` path (`update_build_plugin.go`), **not** of the
`rib/commit` path, which does prepend for EBGP (`commit.go`). So "the originated
path" is at least two paths that disagree with each other. Establish which is which before
designing; that disagreement may be the actual bug.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugins.md` - registration over hardcoding
  → Constraint: an auto-added chain entry must go through the existing filter registry, not a new per-feature switch in a core struct
- [ ] `ai/rules/performance.md` - buffer ownership on the forward path
  → Constraint: the received wire is shared by every destination and MUST NOT be mutated per destination. `rewriteASPathPrepend` is the only point already paying for a per-destination buffer -- a chain entry that forces a second copy is a real cost, and `spec-fixit-tombstone-ebgp-transitive` notes the EBGP RS-client case (`facts.isEBGP && facts.rsClient`) has no per-destination buffer at all
- [ ] `ai/rules/config.md` + `ai/patterns/config-option.md` - if the auto entries become visible/disable-able

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - §5.1.5 (`:381`, `:702`: "MUST NOT include LOCAL_PREF in UPDATE messages sent to external peers"), §9.1.2 (prepend)
  → Constraint: §5.1.5 is a MUST NOT and is currently unenforced on the forwarded egress path
- [ ] `rfc/short/rfc6793.md` - AS4_PATH, owned by `spec-fixit-as4path-missing-on-rewrite`
  → Constraint: whatever performs prepend inherits that spec's AS4_PATH obligation

**Key insights:**
- `IsEBGP()` is `LocalAS != PeerAS`, so the `local-as` override modes (`localASNoPrepend`, `localASReplaceAS`, `asOverride` -- `peer_forward_facts.go`) interact with it, and the existing prepend already branches on them (`reactor_api_forward.go`). An auto-filter must reproduce that branching or it is a regression.
- `runEgressPolicyChainASN4`'s internal order is already: `PolicyFilterChain` -> Reject -> raw override -> text-delta -> `textDeltaToModOps` -> `ExtractRemovePrivateASOps` -> `ExtractASPathPrependOps` -> `buildModifiedPayload`. Where the auto entries sit relative to `remove-private-as` is load-bearing: `remove-private-as-export.ci` asserts strip happens **before** prepend and that "EBGP local-AS prepend uses the stripped AS_PATH as its base".
- `orderedEgressSteps` is assembled once (`reactor.go`) and sorted by `filterapi.LessOrder(name, stage, priority)` across stages `FilterStageProtocol=0` / `Policy=100` / `Annotation=200` / `PeerChain=300`. An RFC-mandated auto-filter is conceptually `FilterStageProtocol` ("RFC-mandated checks") -- but see the hook trap: that ladder is not reached by the originated path.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/peers.go` - `:143-168` `ExportFilters = concatFilters(bgpExport, groupExport, peerExport)` (cumulative bgp+group+peer); `:184-191` `ValidateFilterNames`; `:200-203` `canonicalizeFilterRefs` (`<plugin>:<filter>`); `:643-673` `prependDefaultFilters` (import-only precedent)
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `:52-58` `orderedEgressStep`; `:66-69` `egressStepResult{accept, wireOverride}`; `:102-125` `buildOrderedEgressSteps`; `:195-203` / `:221-260` the chain bodies
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go` - `:56` calls `runEgressPolicyChainASN4` directly, bypassing `orderedEgressSteps`
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `:392-394` `ExportFilters []filterapi.FilterRef` (frozen per-peer chain); `:536-543` `IsIBGP`/`IsEBGP`

**Behavior to preserve:**
- `remove-private-as-export.ci`'s assertion that strip precedes prepend, and its AS_PATH `[65000 64496 64497]` byte-exact.
- `attributes.ci` -- peer1 is **IBGP** (`local 1`/`remote 1`, `:66-67`); its `400504000000C8` (LOCAL_PREF=200) and verbatim AS_PATH `[1 2 3 4]` must not move. It is the IBGP keep-LOCAL_PREF / no-prepend baseline and does **not** cover EBGP. (`spec-fixit-private-asn-leak.md` cites it as evidence of "the same verbatim-as-path behavior" -- true only for IBGP.)
- Every `local-as` override mode's current prepend behavior.

**Behavior to change:**
- LOCAL_PREF must stop reaching EBGP peers on the forwarded path (RFC 4271 §5.1.5).
- Prepend for EBGP becomes a chain entry rather than a wire-path special case.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | An auto `FilterRef` on `ExportFilters` reaches both the forwarded and originated paths | both reach `runEgressPolicyChainASN4`; Goal Gate `spec-fixit-private-asn-leak.md` | the design silently misses originated routes -- the leak class all over again | a functional test on each path before building anything else | unvalidated |
| A-2 | Moving prepend off `rewriteASPathPrepend` does not cost a second per-destination copy | that function exists because it is the one place already paying for one | the chain approach costs an extra copy per EBGP destination per UPDATE | benchmark against `spec-perf-next-1-ebgp-wire-lockfree`'s baseline | unvalidated |
| A-3 | IBGP-source -> EBGP-destination LOCAL_PREF leak is real | `grep -i localpref internal/component/bgp/wireu/*.go` = zero non-test hits; no egress strip in `reactor/*.go` | the MUST NOT half of this spec evaporates | **write the failing functional test FIRST** -- it does not exist today | unvalidated (grep-established, not test-established) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Three in-progress specs are editing the seam this retires; landing out of order wastes one of them | any of the three closes while this is in design | Thomas sequences this explicitly. Cheapest order is likely: let the three land, then re-home their EBGP-egress logic into the chain |
| R-2 | The EBGP RS-client case has no per-destination buffer (`forward_rs.go`, `reactor_api_forward.go`) so a chain entry cannot mutate its wire | RS-client tests break, or the auto entries silently skip RS-clients | Same open question `spec-fixit-tombstone-ebgp-transitive` Known Limitations already flags as "Thomas's call": conformance vs RS zero-copy |
| R-3 | `FilterRef` has no "auto" marker, so an auto entry is indistinguishable from a typed one in config/show output | operator sees a filter they never wrote, or cannot see the one protecting them | Design the marker + rendering before implementing |
| R-4 | Prepend's two implementations disagree (`update text` does not prepend, `rib/commit` does) and consolidation silently picks one | the two originated paths produce different AS_PATHs for the same intent | Establish the intended behavior for each first; that disagreement may itself be the bug |

### Resolutions (readiness pass)

→ **R-2 AUTONOMOUS DEFAULT (2026-07-17): CONFORMANCE over RS zero-copy.** The
LOCAL_PREF-suppress auto-entry MUST apply to EBGP RS-clients: an RS<->client session is
EBGP, so RFC 4271 §5.1.5 ("MUST NOT ... LOCAL_PREF ... to external peers") still binds, and
an RS-client must not receive an unfiltered LOCAL_PREF leak. Where an RS-client has no
per-destination buffer today (`forward_rs.go` and `reactor_api_forward.go`
set `peerWire = update.WireUpdate`/`peerBaseWire` unchanged when `facts.isEBGP &&
facts.rsClient` and no ASN4 transcode is needed — verified by reading both funnels),
honouring the suppress forces the same per-update pooled copy the sibling
`spec-fixit-tombstone-ebgp-transitive` Known Limitations already scoped (its line 306 +
Recap 343-346): a third cached slot on `ReceivedUpdate` mirroring `ebgpSlotASN4`/
`ebgpSlotASN2`, plus release plumbing at `recent_cache.go,527`. **Accepted, documented
consequence:** RS attribute-bearing UPDATEs lose zero-copy forwarding; this is the same
tradeoff the sibling defers, and it is inherited here, not re-litigated. The AS_PATH-prepend
auto-entry is the OPPOSITE case: RFC 7947 §2.2.2 (an RS MUST NOT modify AS_PATH for
RS-clients) means prepend MUST be suppressed for RS-clients — exactly what the existing
`!facts.rsClient` gate (`forward_rs.go`, `reactor_api_forward.go`) already does — so
that half needs no per-destination copy for RS-clients. Rationale: correctness (no
LOCAL_PREF leak to an EBGP RS-client) outranks the RS zero-copy fast path; §7947 keeps the
prepend half off RS-clients so conformance and zero-copy only collide for LOCAL_PREF-bearing
UPDATEs. Cross-reference: sibling Known Limitations (`spec-fixit-tombstone-ebgp-transitive.md`
lines 306, 343-346). Thomas: override if wrong.

→ **R-3 AUTONOMOUS DEFAULT (2026-07-17): reserve a name; do NOT grow `FilterRef`.** Use a
reserved-namespace synthetic `FilterRef` name (e.g. `auto:ebgp-localpref-suppress`,
`auto:ebgp-prepend`) auto-appended to `ExportFilters`, mirroring how `prependDefaultFilters`
(`config/peers.go+`) prepends loop-detection refs to `ImportFilters` unless already
present (`filterChainContains`, `:667`). `ValidateFilterNames` and
`canonicalizeFilterRefs` must accept the reserved prefix. `show`/config renders it
as a system/auto-provided entry, visually distinct from user config, never round-tripped as
operator-typed input. Rationale: decision protocol "scope -> smaller, self-contained
option"; a reserved name reuses the existing name-reference plumbing and the existing
import-side precedent, avoiding an `Auto bool` field that would ripple through every
`FilterRef` consumer. Thomas: override to a real `Auto` marker if operator-visibility
requirements demand it.

→ **R-4 AUTONOMOUS DEFAULT (2026-07-17): the conformant reference is `rib/commit`.** RFC 4271
§9.1.2 makes local-AS prepend a MUST on every EBGP export, so the `rib/commit` path
(`commit.go`, which prepends for EBGP) is the intended behavior and the
`update text`/`BuildPlugin` path (`update_build_plugin.go`, which does not) is the one
out of conformance for a normal announce. The auto prepend entry, applied uniformly to both
paths, is the fix and satisfies AC-4 (the two paths agree). **Provisional:** if `update text`
is deliberately a raw/verbatim AS_PATH injection surface, that carve-out must be confirmed
before Phase 3 relocates prepend; it does not affect the LOCAL_PREF half (Phase 2).
Rationale: §9.1.2 is a MUST — the conformant path is the safe default. Thomas: override if
`update text` is meant to inject a verbatim AS_PATH.

→ **SEQUENCING — HARD BLOCKER, not autonomously resolvable (2026-07-17).** The
prepend-relocation half (Phases 3-4) retires `wireu.rewriteASPathPrepend`, the exact seam
three in-progress specs are actively editing (`spec-fixit-as4path-missing-on-rewrite`,
`spec-perf-next-1-ebgp-wire-lockfree` at 1/5, `spec-fixit-tombstone-ebgp-transitive`), and
the parent `spec-fixit-private-asn-leak` is itself in-progress (all four confirmed
in-progress 2026-07-17). Landing this before they land wastes one of them (R-1). The spec
author reserved this sequencing for Thomas (lines 100, 252, 284, 317, and Known Limitations).
This readiness pass CANNOT default a scope reduction to "LOCAL_PREF half only," because that
drops AC-3/AC-4/AC-8 (prepend), and scope reduction requires explicit user approval. ~~Status
therefore stays `skeleton` until Thomas gives the sequencing call.~~ Every other open item is
resolved above, so that sequencing call is the sole remaining gate.

→ **SEQUENCING RESOLUTION — AUTONOMOUS DEFAULT (2026-07-17), Thomas-authorized.** Thomas has
now authorized resolving this blocker. Resolution: keep FULL SCOPE (no AC dropped) and convert
the blocker into an explicit ORDERING CONSTRAINT rather than a scope reduction. Two halves,
one landing order:

- **Phases 1-2 (LOCAL_PREF-suppress, AC-1/AC-2) — implementable NOW.** This half does not
  touch `wireu.rewriteASPathPrepend`; it appends an EBGP-gated `AttrModSuppress` LOCAL_PREF
  auto-entry at config time and closes the currently-unenforced RFC 4271 §5.1.5 MUST NOT.
- **Phases 3-4 (prepend-relocation, AC-3/AC-4/AC-5/AC-8) — GATED.** This half retires
  `wireu.rewriteASPathPrepend` (verified on disk: sole definition at `aspath_rewrite.go`;
  its only callers are `RewriteASPath` `:35` and `RewriteASPathDual` `:52`). It MUST NOT
  START until all four specs actively editing that seam land:
  `spec-fixit-as4path-missing-on-rewrite`, `spec-perf-next-1-ebgp-wire-lockfree`,
  `spec-fixit-tombstone-ebgp-transitive`, and the parent `spec-fixit-private-asn-leak` (all
  four re-confirmed `| Status | in-progress |` on disk 2026-07-17).

See the "Phase 3-4 Sequencing Gate" note under Implementation Phases for the concrete gate,
referenced from AC-3/AC-4/AC-5/AC-8. Because this is an ordering constraint (a fixed landing
order), not a design ambiguity, the spec is `ready` for its Phase 1-2 work with Phases 3-4
gated. Rationale: R-1 is satisfied by ordering — letting the four seam-editing specs land
first means none of them is wasted — while the LOCAL_PREF MUST NOT is closed immediately by
the independent Phase 1-2 half. Thomas: override the order if a different landing sequence is
preferred.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config parse: a peer's `ExportFilters` are assembled from bgp + group + peer (`config/peers.go`), validated, canonicalized to `<plugin>:<filter>`. **This is where the auto entries are appended**, mirroring `prependDefaultFilters`, gated on `ps.IsEBGP()` (`peer_settings.go`)
- Runtime: a route reaches the peer's egress, forwarded or originated

### Transformation Path
1. Chain frozen onto the peer: `PeerSettings.ExportFilters []filterapi.FilterRef` (`peer_settings.go`)
2. Both egress paths converge on the one shared body `runEgressPolicyChainASN4` (`filter_ordered.go`) -- the forwarded path via `reactor_api_forward.go`, the originated path via `egress_inject_filter.go`
3. Inside that body, in order: `PolicyFilterChain` -> Reject -> raw override -> text-delta -> `textDeltaToModOps` -> `ExtractRemovePrivateASOps` -> `ExtractASPathPrependOps` -> `buildModifiedPayload`
4. The auto entries emit ops in the existing vocabulary: `AttrModPrepend` on AS_PATH, `AttrModSuppress` on LOCAL_PREF (`filterapi/filterapi.go`, `:220-224`)
5. `buildModifiedPayload` writes the per-destination wire

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ per-peer runtime | auto `FilterRef` appended at parse time, frozen onto `PeerSettings` | [ ] |
| Wire ↔ per-destination buffer | `buildModifiedPayload` vs `wireu.rewriteASPathPrepend`'s existing per-destination copy | [ ] |
| Shared received wire ↔ RS-client fast path | `forward_rs.go` hands out the SHARED wire with no per-destination buffer (R-2) | [ ] |

### Integration Points
- `internal/component/bgp/config/peers.go` (`prependDefaultFilters`) - the precedent to mirror onto the export side; it is import-only today
- `internal/component/bgp/reactor/filter_delta.go` (`ExtractASPathPrependOps`) - already builds an AS_SEQUENCE of N x localAS; the prepend auto-filter should reuse it, not re-implement it
- `internal/component/bgp/reactor/filter_delta_handlers.go` - `{attribute.AttrLocalPref, 0x40}` handler already registered; `AttrModSuppress` needs no new plumbing
- `internal/component/bgp/wireu/aspath_rewrite.go` (`rewriteASPathPrepend`) - what this design retires or bypasses for EBGP prepend; **three in-progress specs are editing it** (see Convergence)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IBGP peer sends a route with LOCAL_PREF; ze forwards to an EBGP peer | → | auto `AttrModSuppress` LOCAL_PREF entry on `ExportFilters` (EBGP-gated) | `test/plugin/ebgp-localpref-egress-strip.ci` (NEW — MUST be RED before the fix: LOCAL_PREF visible on the EBGP wire) | <!-- doc-links: ignore (test this spec proposes; not written) -->
| API `update text` announces to an EBGP peer | → | auto `AttrModPrepend` AS_PATH entry on `ExportFilters` (EBGP-gated) | `test/plugin/ebgp-prepend-originated.ci` (NEW — originated path prepends the local AS, agreeing with the forwarded path; `remove-private-as-export-originated.ci` stays GREEN) | <!-- doc-links: ignore (test this spec proposes; not written) -->
| Route forwarded from an EBGP peer to an EBGP peer | → | [auto prepend entry] | `remove-private-as-export.ci` (must stay GREEN) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IBGP-sourced route with LOCAL_PREF forwarded to an EBGP peer | LOCAL_PREF is absent from the wire (RFC 4271 §5.1.5) |
| AC-2 | Any route to an IBGP peer | LOCAL_PREF unchanged; `attributes.ci` stays byte-exact |
| AC-3 | Route to an EBGP peer via the forwarded path | Local ASN prepended exactly as today, including every `local-as` override mode. **(Phase 3-4 — GATED by the Phase 3-4 Sequencing Gate; four seam specs land first.)** |
| AC-4 | Route to an EBGP peer via the originated/injected path | Same prepend outcome as AC-3 (the two paths agree). **(Phase 3-4 — GATED by the Phase 3-4 Sequencing Gate.)** |
| AC-5 | `remove-private-as:STRIP` configured on an EBGP peer | Strip still precedes prepend; `remove-private-as-export.ci` AS_PATH byte-exact. **(Phase 3-4 — GATED by the Phase 3-4 Sequencing Gate; depends on the relocated prepend.)** |
| AC-6 | Operator inspects the peer's export chain | The auto entries are discoverable and their origin is unambiguous |
| AC-7 | EBGP RS-client peer | Defined behavior, not an accident (see R-2) |
| AC-8 | Benchmark, forwarded EBGP path | No added allocation per destination vs the `rewriteASPathPrepend` baseline (A-2). **(Phase 3-4 — GATED by the Phase 3-4 Sequencing Gate.)** |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExportAutoFilters_EBGPOnlyAndDeduped` | `internal/component/bgp/config/peers_test.go` | auto entries appended for EBGP peers only, and not duplicated when user-configured | |
| `TestAttrModSuppress_RemovesLocalPref` | `internal/component/bgp/reactor/filter_delta_test.go` | `AttrModSuppress` on `AttrLocalPref` removes the attribute | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| **`ebgp-localpref-egress-strip`** | `test/plugin/ebgp-localpref-egress-strip.ci` | IBGP source -> EBGP destination: LOCAL_PREF must not appear on the wire. **RED before the fix** | | <!-- doc-links: ignore (test this spec proposes; not written) -->
| `remove-private-as-export` | `test/plugin/remove-private-as-export.ci` | strip-then-prepend unchanged | |
| `attributes` | `test/plugin/plugin-attributes.ci` | IBGP baseline unchanged | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ebgp-egress-conformance-frr` (next free index, currently `47-`) | `test/interop/scenarios/` | FRR (pattern: `bgp-remove-private-as-frr`) | a real EBGP peer sees no LOCAL_PREF and a correctly prepended AS_PATH | |

## Files to Modify
- `internal/component/bgp/config/peers.go` - the auto-append, mirroring `prependDefaultFilters` onto the export side; `ValidateFilterNames` and `canonicalizeFilterRefs` must accept the reserved names
- `internal/component/bgp/reactor/filter_ordered.go` - ordering of the auto entries within the chain
- `internal/component/bgp/wireu/aspath_rewrite.go` - only if prepend is relocated (**coordinate: three in-progress specs, see Convergence**)
- `internal/core/bgp/attribute/builder.go` - `:454-457`'s comment promises a send-time eBGP filter that does not exist; make it true or fix the comment

## Implementation Steps

### Implementation Phases

~~Blocked on Thomas sequencing this against the three in-progress specs (R-1), so phases are
sketched, not committed.~~ (SUPERSEDED 2026-07-17: sequencing resolved by Thomas — see
"→ SEQUENCING RESOLUTION" under Risks & Assumptions and the "Phase 3-4 Sequencing Gate"
below.) The ordering below is what the trace already justifies:

> **Phase 3-4 Sequencing Gate (BLOCKING for the prepend half only).** Phases 1-2 (LOCAL_PREF-
> suppress, AC-1/AC-2) may start immediately — they do not touch `wireu.rewriteASPathPrepend`.
> Phases 3-4 (prepend-relocation, AC-3/AC-4/AC-5/AC-8), which retire
> `wireu.rewriteASPathPrepend` (`aspath_rewrite.go`), MUST NOT START until ALL FOUR of the
> following specs have landed (each `in-progress` on disk 2026-07-17):
> `spec-fixit-as4path-missing-on-rewrite`, `spec-perf-next-1-ebgp-wire-lockfree`,
> `spec-fixit-tombstone-ebgp-transitive`, and the parent `spec-fixit-private-asn-leak`.
> Before starting Phase 3, re-check each of the four is CLOSED (spec file gone / status
> `done`); if any is still open, stop and do Phase 1-2 only. This is a fixed landing order,
> not a design open question — it is the whole reason the LOCAL_PREF half is carved out to
> ship first.

0. **Phase 0: sequencing + the hook proof.** Confirm the sequencing call. Then settle A-1
   with a test before building anything: does an auto `FilterRef` on `ExportFilters`
   actually reach BOTH paths? If it does not, the design is a silent no-op on originated
   routes -- the exact bug class `spec-fixit-private-asn-leak` just fixed. Also resolve R-4:
   establish what `update text` vs `rib/commit` should each do, since they disagree today.
1. **Phase 1: Wiring (MANDATORY FIRST)** — the RED functional test for AC-1: IBGP source
   carrying LOCAL_PREF, forwarded to an EBGP destination, must not put LOCAL_PREF on the
   wire. **This test does not exist and the violation is only grep-established** (A-3), so
   this is the phase that turns a claim into a fact.
   - Verify: RED before any fix, for the right reason (LOCAL_PREF visible on the wire)
2. **Phase 2: the LOCAL_PREF auto entry** — `AttrModSuppress` on `AttrLocalPref`, EBGP-gated,
   auto-appended at config time. Smaller and independent of the prepend relocation, so it
   can land alone and close a live RFC MUST NOT.
   - Verify: Phase 1 goes GREEN; `attributes.ci` (IBGP baseline) stays byte-exact
3. **Phase 3: the prepend auto entry** — the contentious half: it touches the seam three
   other specs own. **GATED — do not start until the Phase 3-4 Sequencing Gate above is
   satisfied (all four seam specs landed).** Must reproduce every `local-as` override mode
   (`localASNoPrepend`, `localASReplaceAS`, `asOverride`) and keep strip-before-prepend
   ordering.
   - Verify: `remove-private-as-export.ci` AS_PATH byte-exact; benchmark vs A-2
4. **Phase 4: retire the old path** — **GATED — same Phase 3-4 Sequencing Gate.** Per
   `ai/rules/no-layering.md`, if prepend moves, the old implementation is deleted, not left
   beside the new one.
5. **Functional + interop tests** → a real FRR/BIRD peer sees no LOCAL_PREF and a correct AS_PATH
6. **Full verification** → `make ze-precommit-verify`
7. **Complete spec** → audit tables, learned summary, two-commit closure

### Failure Routing
| Failure | Route To |
|---------|----------|
| The auto entry does not fire on the originated path | STOP. A-1 is broken and the design is wrong; do not paper over it with a second hook |
| Benchmark shows an added per-destination copy | A-2 broken → back to design; the chain may not be the right home for prepend |
| An RS-client test breaks | R-2: this is the conformance-vs-zero-copy call, and it is Thomas's, not the implementer's |
| `remove-private-as-export.ci` AS_PATH changes | You reordered strip and prepend. The test is right; the chain order is wrong |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (RFC 4271 S5.1.5 / S9.1.2 against a real peer)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations
- The RS-client zero-copy tradeoff (R-2) is inherited, not solved here.
- ~~Sequencing against the three in-progress specs is Thomas's call and gates this leaving `skeleton`.~~ (RESOLVED 2026-07-17, Thomas-authorized: converted to a landing-order constraint — Phase 3-4 Sequencing Gate — not a skeleton block; Phase 1-2 ships independently.)
- The RFC 4271 S5.1.5 violation is grep-established, not test-established (A-3). Until Phase 1's RED test exists, it is a strong claim, not a proven one.

## RFC Documentation

The auto-filter chain entries carry the normative citations they enforce; both must appear at
the code seam (`config/peers.go` auto-append and the chain-entry emit site):

- RFC 4271 §5.1.5 — "LOCAL_PREF ... SHALL NOT be included in UPDATE messages sent to external
  (EBGP) peers" — enforced by the `AttrModSuppress` LOCAL_PREF auto-entry (EBGP-gated). This
  is the previously-unenforced MUST NOT on the forwarded IBGP-source -> EBGP-destination path
  (see Current Behavior table).
- RFC 4271 §9.1.2 — prepend the local AS on EBGP export — enforced by the `AttrModPrepend`
  AS_PATH auto-entry (EBGP-gated), reproducing every `local-as` override mode.
- RFC 7947 §2.2.2 — an RS MUST NOT modify AS_PATH for RS-client peers — the prepend auto-entry
  MUST be suppressed for `facts.rsClient` (R-2 resolution); the existing `!facts.rsClient`
  gate (`forward_rs.go`, `reactor_api_forward.go`) is preserved.
- RFC 6793 §4.2.2 — AS4_PATH obligation on any speaker that prepends — inherited by whatever
  performs prepend; owned by `spec-fixit-as4path-missing-on-rewrite` (see Convergence).
