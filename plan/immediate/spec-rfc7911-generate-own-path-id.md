# Spec: rfc7911-generate-own-path-id

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-09-05 |

**BLOCKER 1 is FIXED (2026-08-29).** The identifier is now keyed on the path
rather than on the source alone whenever the SOURCE frames Path Identifiers, and
it is freed when the recent-update cache evicts the UPDATE that withdrew that
path. The release contract, the design it rejects and the tests that prove it are
in the Review Gate section at the end of this file. The spec stays open for its
closure review.

**Answered 2026-08-14, kept for the record.** The phase-2 question was whether
`TestForwardPathIDsDifferForCollidingSources`
(`internal/component/bgp/reactor/forward_path_id_test.go`) could set a `SourceID`
on its two frames. The receive path sets one on every UPDATE it accepts
(`session_read.go`), and the generator keys on it, so two frames without it are
one source announcing one path twice rather than two peers. Thomas approved the
fixture change on 2026-08-14 and it carries the `rfc-test-change-approved` token.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The requirement.** RFC 7911 Section 2: a BGP speaker that re-advertises a route
MUST generate its own Path Identifier, and MUST NOT reuse the one it received.
The obligation is captured as `RFC7911-2-2` in `rfc/short/rfc7911.md`, where it
is currently annotated `{gap}`.

**What ze does.** `fwdReencodeNLRIs` (`internal/component/bgp/reactor/forward_body.go`)
reads each `(prefix, pathID)` pair from the source frame and writes the RECEIVED
`pathID` straight into the destination frame. Nothing in
`internal/component/bgp/reactor/` mints a Path Identifier. So every re-advertised
path carries its ingress identifier.

**The operator consequence, and why this is release-gating.** On a route server,
Path Identifiers are chosen independently by each client. Two clients that both
choose path-id 1 for the same prefix are advertised to a third client as the same
`(prefix, path identifier)` pair twice. The receiver treats the second as a
replacement for the first, so one path is lost. The identifier is the only thing
distinguishing the two paths on the wire, and ze is reusing a value it does not
own. Route servers are the deployment ADD-PATH exists to serve.

**Why the annotation is not authority.** `ai/rules/rfc-compliance.md` voided every
earlier answer pointing away from full compliance on 2026-07-27, and names a
`{gap}` in `rfc/short/*.md` as one of the places such an answer hides. The
annotation records that the gap was seen. It does not record a decision to keep it.

**Scope.** The Path Identifier is generated at the ONE egress transform, so both
the live forward rail and the stored-route replay inherit it from the same place.
`spec-fixit-bgp-egress-rail-divergence` established that invariant and this spec
must not break it: minting an identifier in the relay alone would give a replayed
route a different identifier from the live one for the same path.

**Not in scope.** The ADD-PATH receive path, the capability negotiation, and the
`{gap}` annotations of the other eight RFC 7911 requirements, which are gated and
proven.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/structural-forwarding.md` - the one-egress-transform invariant
  → Constraint: a replayed route and a live forward MUST produce identical wire bytes for the same path

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7911.md` - `RFC7911-2-2` is the subject; the other eight requirements bound the change
  → Constraint: the identifier is per (destination, family), and RFC 7911 permits the value 0

**Key insights:** (minimal context to resume after compaction)
- The generator belongs at the egress transform, not at either rail.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_body.go` - `fwdReencodeNLRIs` iterates
  `(prefix, pathID)` and writes the received `pathID` when the destination context
  declares ADD-PATH. It is the only wire producer of a re-advertised NLRI
  → Constraint: it converts framing per destination and already runs on both rails,
    so it is the correct and only site for the generator

**Behavior to preserve:**

| Behavior | Where | Why it must not change |
|----------|-------|------------------------|
| A destination without ADD-PATH receives bare prefixes | `fwdReencodeNLRIs` returns its input unchanged when neither side frames identifiers, and strips the field when only the source does | Re-framing is per destination and must stay so. The early return on `srcAddPath == destAddPath` had to go: a shared context frames alike on both sides and the VALUE still has to change |
| A replayed route is byte-identical to the live forward of the same path | the single egress transform | `spec-fixit-bgp-egress-rail-divergence` closed on this invariant |
| Path identifier 0 is a legal value | RFC 7911 Section 3 | A generator that treats 0 as unset reintroduces the defect it fixes |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An UPDATE received from one peer and forwarded to another, on either the live
forward rail or the peer-up stored-route replay.

### Transformation Path
1. The received NLRI is parsed to `(prefix, pathID)` pairs by the source context.
2. The egress transform decides the destination framing.
3. For a destination that declares ADD-PATH, an identifier is chosen for this
   `(destination, family, path)` rather than copied from the source.
4. The chosen identifier is stable for as long as the path is advertised, and is
   released when the path is withdrawn.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer to peer | the re-advertised identifier is ze's, not the source peer's | No |
| Live rail to replay rail | both read one generator, so both agree | No |

### Integration Points
- `fwdReencodeNLRIs` (`internal/component/bgp/reactor/forward_body.go`) - the only wire producer of a re-advertised NLRI.

### Architectural Verification
- The generator is state per destination and family. It is NOT per message, so it cannot live in a per-call buffer.
- **Registration over hardcoding.** Both rails reach the generator through the egress transform they already call, so nothing registers a second path and no core or shared package spells a family, a peer role, or a destination kind. The set of ADD-PATH families comes from the negotiated `bgpctx.EncodingContext`, which is the registry that already holds it, and is never re-enumerated (`ai/rules/plugins.md`, `ai/rules/evidence.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|--------------------------------|----------|--------------|--------|
| A-1 | `fwdReencodeNLRIs` is the only producer of re-advertised NLRI bytes | read at the producer 2026-08-14 | a second site needs the same generator, or the rails diverge | `gopls references` over the NLRI writers | broken 2026-08-14. `buildFwdBody`'s same-context branch emits re-advertised NLRI bytes without ever calling `fwdReencodeNLRIs`, and that branch is the route-server case. Two WRITERS now exist, `fwdRegenerateRawPathIDs` and `fwdReencodeNLRIs`; the design still holds because both read ONE generator, `fwdPathIDs` |
| A-2 | An identifier can be held per destination and family without unbounded growth | the RIB already keys paths per peer | the generator needs its own eviction | count the live paths a route server holds | broken 2026-08-14 as first written, then corrected twice. FINAL (2026-08-29): the key follows what the SOURCE frames. A client that frames no identifier costs ONE entry for its whole session, released at peer removal (`doRemovePeer`). A client that frames one costs one entry per (family, identifier, prefix) it currently advertises, released when the cache evicts the UPDATE that withdrew that path (`fwdReleaseWithdrawnPathIDs`). The table is bounded by the paths ze advertises |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A regenerated identifier that is not stable across UPDATEs makes a destination treat one path as many | a peer's table grows on every refresh | the identifier is keyed by the path, not by the message |
| R-2 | Allocating per message on the forward hot path | a benchmark regression in forward throughput | the identifier is computed once per path and stored, never per message (`ai/rules/performance.md`) |
| R-3 | The replay rail and the live rail choose differently for one path | a replayed route differs from the live one in a capture | both read one generator; a test asserts byte-identity |

## Blast Radius

Every ADD-PATH session ze forwards to. A wrong identifier is wire-visible and can
lose a path at the receiver, which is the defect this fixes, so a regression here
costs what the bug costs.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| UPDATE forwarded to an ADD-PATH peer | wire → forward rail → `fwdReencodeNLRIs` → generator | `bgp-addpath-readvertise-collision-frr` (interop) |
| Peer-up replay to an ADD-PATH peer | replay rail → `fwdReencodeNLRIs` → same generator | `bgp-addpath-rail-agreement-speaker` (interop) |
| The same path advertised twice | generator → held per path → same identifier | `TestForwardPathIDStableAcrossUpdates` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A route received with path-id N, re-advertised to an ADD-PATH peer | the advertised identifier is ze's own, and the test asserts it is not simply N |
| AC-2 | Two peers advertise the same prefix, both choosing path-id 1 | a third ADD-PATH peer receives two paths with DIFFERENT identifiers and keeps both |
| AC-3 | The same path re-advertised in a later UPDATE | it carries the same identifier as before, so the receiver replaces rather than duplicates |
| AC-4 | A path withdrawn and a new path learned | the identifier of the withdrawn path may be reused only after the withdraw is sent |
| AC-5 | A route received with path-id 0 | it is re-advertised with a generated identifier, and 0 is treated as a legal received value throughout |
| AC-6 | A destination that did NOT negotiate ADD-PATH | receives bare prefixes, unchanged from today |
| AC-7 | The same path delivered by the live rail and by peer-up replay | both carry the same identifier, and the wire bytes are identical |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs ze as a route server between two clients whose paths share one received identifier | wire → egress transform → generator → both paths survive at the third client | `bgp-addpath-readvertise-collision-frr` |
| 2 | A client reconnects and receives the peer-up replay | replay rail → same generator → identical bytes | `bgp-addpath-rail-agreement-speaker`, and the session-reset half of `bgp-addpath-readvertise-collision-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardPathIDsDifferForCollidingSources` | `internal/component/bgp/reactor/forward_path_id_test.go` | AC-2 | [ ] Green since 2026-08-14, once each frame carried a `SourceID` (owner-approved fixture change). Without it the two frames were one source replacing its own path rather than two peers |
| `TestForwardPathIDStableAcrossUpdates` | same | AC-1, AC-3 | [ ] |
| `TestForwardPathIDDiffersForTwoSourcePeers` | `internal/component/bgp/reactor/forward_path_id_gen_test.go` | AC-2 | [ ] |
| `TestForwardPathIDMatchesAnnounceAndWithdraw` | same | AC-4 | [ ] |
| `TestForwardPathIDSurvivesAttributeChange` | same | AC-3 | [ ] |
| `TestForwardPathIDSeparatesNonAddPathSources` | same | AC-1, AC-5 | [ ] |
| `TestForwardPathIDBoundaryReceivedValues` | same | AC-5 | [ ] |
| `TestForwardPathIDIdenticalForEveryDestination` | same | AC-7 | [ ] |
| `TestForwardPathIDLeavesNonAddPathDestinationAlone` | same | AC-6 | [ ] |
| `TestForwardPathIDReleaseReturnsValues` | same | AC-4 | [ ] |

### Boundary Tests (numeric inputs)
| Test | Input | Expected |
|------|-------|----------|
| received identifier at 0 | 0 | legal, regenerated |
| received identifier at 2^32-1 | max uint32 | legal, regenerated |

### Functional Tests
| Test | File | Validates |
|------|------|-----------|
| colliding identifiers both survive | the interop scenario below, NOT a `.ci` | AC-2 |
| replay matches live byte for byte | the interop scenario below, NOT a `.ci` | AC-7 |

Neither `.ci` was written, and neither should be. `parseExpectRule`
(`internal/test/peer/checker.go`) offers `hex`, `prefix`, `contains` and `ordered`
with no negation and no wildcard, so "two identifiers, values unknown, must differ"
is not expressible. Pinning literal values is worse than useless here: the generator
mints from 0 (`fwdPathIDTable.mintLocked`), so the FIRST re-advertised path in a
process carries identifier 0, which is the same value the defect produces for a
source that negotiated no ADD-PATH. A `.ci` pinned to it passes with the fix
reverted. The route loss is a RECEIVER's behaviour (RFC 7911 Section 5), so only a
daemon holding a RIB can report it.

### Interop Tests (Scope: protocol)
| Test | Peer | Validates |
|------|------|-----------|
| `test/interop/scenarios/bgp-addpath-readvertise-collision-frr` | FRR 10.3.1, with BIRD and GoBGP as the colliding sources | AC-2: FRR keeps both paths, under identifiers 0 and 1. Reverting the generator leaves FRR holding ONE path |
| `test/interop/scenarios/bgp-addpath-rail-agreement-speaker` | two independent Python speakers, one live and one replayed | AC-7: both UPDATE bodies are byte-identical, Path Identifier included |

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/bgp/reactor/forward_body.go` | `fwdReencodeNLRIs` reads a generated identifier instead of copying the received one, and `buildFwdBody`'s same-context branch rewrites the identifiers of the raw frame before it splits or forwards it |
| `internal/core/bgp/context/context.go` | `AnyAddPath`, so the forward rail can skip every session that negotiated ADD-PATH for nothing before it reads an UPDATE's sections |
| `internal/component/bgp/reactor/reactor_peers.go` | `doRemovePeer` releases the removed peer's identifiers |
| `internal/le/` | the judged-count pin drops to 57: RFC 7911's row stops spelling a gap count when its last `{gap}` closes |
| `rfc/short/rfc7911.md` | `RFC7911-2-2` loses its `{gap}` and gains both polarities |
| `docs/features/rfc-status.md` | the RFC 7911 row's Remaining count and prose |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/bgp/reactor/forward_path_id.go` | the generator, keyed by the path at ingress, plus the raw-frame rewriter the same-context branch uses |
| `internal/component/bgp/reactor/forward_path_id_gen_test.go` | the generator's tests: withdraw matching, replacement, non-ADD-PATH source and destination, boundary values, destination agreement, release |
| ~~the two `.ci` named above~~ | Replaced by the two interop scenarios in the TDD Test Plan. The `.ci` harness cannot express either assertion, and the property AC-2 names is a receiver's, so no test inside ze can report it |
| `test/interop/scenarios/bgp-addpath-readvertise-collision-frr/` | AC-2 at FRR: `ze.conf`, `frr.conf`, `bird.conf`, `gobgp.toml`, `check.py` (carries the `RFC7911-2-2 positive` interop tag) |
| `test/interop/scenarios/bgp-addpath-rail-agreement-speaker/` | AC-7 byte identity: `ze.conf`, `gobgp.toml`, `speaker-args`, `speaker2-args`, `check.py` |

### Integration Checklist

| Surface | Answer |
|---------|--------|
| Functional test for new behavior | Yes, the two `.ci` above |
| Interop test | Yes, protocol scope: an ADD-PATH peer must keep both paths |
| RFC status ledger | Yes, `docs/features/rfc-status.md` and `rfc/short/rfc7911.md` |
| YANG, CLI, env var, doctor, metrics | N-A. No operator-facing surface changes |

### Documentation Update Checklist (BLOCKING)

| Category | Answer |
|----------|--------|
| 9. RFC compliance | Yes. `rfc/short/rfc7911.md` and `docs/features/rfc-status.md` |
| 12. Internal architecture | Yes. The forward-rail doc gains the generator and its stability contract |
| 16. Source anchors | Yes. Grep `docs/` for `forward_body.go` |
| all others | N-A. No config, CLI, API, plugin, or wire-format surface change beyond the identifier value |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the generator exists and the egress transform reads it, returning the received identifier unchanged so no behavior moves yet. The `.ci` are written and fail.
2. **Phase: Generation and stability** - AC-1, AC-3, AC-5. The identifier is chosen per path and held.
3. **Phase: Collision** - AC-2. Two sources choosing one value both survive at the destination.
4. **Phase: Rail agreement** - AC-7. The replay and the live forward produce identical bytes.
5. **Phase: Release and reuse** - AC-4. An identifier returns to the pool only after its withdraw is sent.
6. **Phase: Ledger** - the `{gap}` is removed, both polarities are tagged, and the public row is corrected.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| One generator | Both rails read the same one. Minting in the relay alone is the failure `spec-fixit-bgp-egress-rail-divergence` closed |
| Stability | The identifier is keyed by the path, never by the message, or a refresh reads as new paths (R-1) |
| Zero is legal | Nothing treats a received or generated 0 as unset (AC-5) |
| Hot path | The identifier is computed once per path and stored. No allocation per message (`ai/rules/performance.md`) |
| Reuse ordering | An identifier is reused only after the withdraw that released it is on the wire (AC-4) |
| Tagged proof | `RFC7911-2-2` carries a positive AND a negative `RFC requirement:` tag |
| Registration over hardcoding | The generator is reached through the egress transform every rail already calls. No family, peer role, or destination kind is spelled in a core or shared package, and nothing enumerates what a registry already holds (`ai/rules/plugins.md`, `ai/rules/evidence.md`) |

### End-to-End User Stories (filled)

The two rows above are the filled set.

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The generator has one caller | `gopls references` names only the egress transform |
| `RFC7911-2-2` is no longer a gap | `grep -n "RFC7911-2-2" rfc/short/rfc7911.md` shows both polarities and no `{gap}` |
| The public ledger agrees | `./le rfc check` green |
| Colliding identifiers survive | the `.ci` passes, and fails when the generator is reverted to copying |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A peer that announces many paths must not grow the identifier state without bound (A-2, R-2) |
| Predictability | The identifier is not a secret and needs no unpredictability, but it must not leak the source peer's choice |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails on behavior mismatch | Re-read the producer in Current Behavior; if misunderstood → RESEARCH |
| Interop peer raises a NOTIFICATION | The framing is wrong, not the identifier. Re-read RFC 7911 Section 3 |
| 3 fix attempts failed | STOP. Report all three approaches. Ask the user |

## Design Insights

- The identifier is the only thing distinguishing two paths for one prefix on the
  wire. Reusing a value chosen by someone else is how two paths become one.

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Generate at the egress transform | It is the one place both rails already pass through, so the invariant holds by construction |
| Key the identifier by the path | Stability across UPDATEs is what stops a receiver treating a refresh as new paths (R-1) |

## Known Limitations

| Limitation | Why it is accepted |
|------------|-------------------|
| The receive path is untouched | RFC7911-2-2 governs re-advertisement only. The other eight requirements are gated and proven |

## RFC Documentation (Scope: protocol)

`rfc/short/rfc7911.md`, `RFC7911-2-2`, Section 2. The requirement is enrolled and
currently annotated `{gap}`. This spec removes the annotation and replaces it with
a tagged test in both polarities.

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le rfc check` green with `RFC7911-2-2` carrying both polarities
- [ ] `./le verify worktree` green, or scoped evidence with attribution

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Review Gate 0 BLOCKER / 0 ISSUE

## Implementation Summary

### What Was Implemented
- `fwdPathIDTable` (`internal/component/bgp/reactor/forward_path_id.go`) holds ze's
  own Path Identifier per ingress path, where an ingress path is
  (source peer, received identifier). `generate` assigns on first sight and
  returns the same value after; `mintLocked` steps over any value a live path
  holds; `releaseSource` drops a peer's entries.
- Both egress writers read that one table. The raw same-context relay goes through
  `fwdRegenerateRawPathIDs`, which patches a pooled copy of the payload in place,
  and the re-encode goes through `fwdReencodeNLRIs` with a `fwdPathIDMemo`.
- `EncodingContext.AnyAddPath` (`internal/core/bgp/context/context.go`) lets the
  raw relay skip every session that negotiated ADD-PATH for nothing before it
  reads an UPDATE's sections.
- `Reactor.doRemovePeer` (`internal/component/bgp/reactor/reactor_peers.go`)
  releases a removed peer's identifiers.
- Two interop scenarios: `bgp-addpath-readvertise-collision-frr` (AC-2 at FRR) and
  `bgp-addpath-rail-agreement-speaker` (AC-7 byte identity), plus `--add-path` on the
  interop speaker engine.
- The release runs at ONE point, the recent-update cache evicting the UPDATE that
  carried the withdraw (`fwdReleaseWithdrawnPathIDs`, called from
  `RecentUpdateCache.evictLocked` and `Delete`), and it walks the entry's body
  BEFORE the buffer goes back to the pool. `fwdAnnouncedPaths` keys the announced
  section once so a pair the same UPDATE announces is kept without a scan per
  withdrawn NLRI.

### Bugs Found/Fixed
- `TestForwardSplitSameContextKeepsRawSplit` asserted the emitted NLRI equalled the
  source NLRI, which pinned the violation. Corrected with the owner's approval token
  on 2026-08-14: the prefixes are still compared, the identifiers are now asserted
  to be ze's own and to be unique per path.
- Three forward rebuild sites dropped the ingress `SourceID` onto the rebuilt wire,
  which is the key the identifier and its release rest on. Both `forward_rs.go`
  branches and `reactor_api_forward.go`'s withdraw branch now preserve it
  (`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-08-29 row).
- The release parsed the entry's Withdrawn Routes and MP_UNREACH sections AFTER
  returning the buffer that backs them, so a goroutine taking the pool slot in
  between made the walk read another UPDATE. Fixed at both cache removal sites
  (`plan/journal/concurrent-session-corruption.md`, 2026-09-02 row).
- The release answered its one exception with a scan of the announced section per
  withdrawn NLRI, a peer-controlled quadratic under the cache lock. Round 2 of the
  Review Gate found it; `plan/journal/membership-answered-by-a-scan-per-item.md`
  carries the class.

### Documentation Updates
- `docs/features/rfc-status.md` (RFC 7911 row), `rfc/short/rfc7911.md`,
  `rfc/requirements/rfc7911.md`, `ai/RFC-REQUIREMENTS.md`.
- `docs/features.md`, `docs/features/bgp-protocol.md` and `docs/guide/add-path.md`
  carry the feature and its source anchors on `forward_path_id.go`, verified against
  `fwdPathIDTable.generate` today.
- `docs/architecture/bgp/structural-forwarding.md` gained "How long a Path
  Identifier lives", and its bucket-merge exclusion claim is corrected: the Path
  Identifier rewrite is a copy path that satisfies all three merge conditions,
  and merging it is safe for the opposite reason to the one the page gave,
  because its bytes are the same for every destination.
- `docs/architecture/memory/lifetime-contracts.md` gained "Whatever reads the
  bytes runs before whatever frees them", the ordering `d949eec53` restored at
  both cache removal sites.

### Deviations from Plan
- The identifier is keyed on the ingress path, not on (destination, family): one
  value per path for every destination is what AC-7 needs.
- Two egress writers exist rather than one (A-1 broke). Both read one table, so the
  invariant the spec cared about holds.
- No `.ci` was written. The harness cannot express "two identifiers, values
  unknown, must differ", and the property is a receiver's: the two interop
  scenarios carry it.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 said `fwdReencodeNLRIs` was the only producer of re-advertised NLRI bytes | `buildFwdBody`'s same-context branch emits them without ever calling it, and that is the route-server case | `gopls references` over the NLRI writers, 2026-08-14 | the generator sits behind both writers, and `TestForwardPathIDsDifferForCollidingSources` drives `buildFwdBody` rather than the re-encode |
| documentation | The 2026-08-29 key change edited `docs/architecture/bgp/structural-forwarding.md` and stopped there, so `docs/features.md`, `docs/features/bgp-protocol.md` and `docs/guide/add-path.md` kept describing the superseded key and the superseded release for a week | The key mirrors what the SOURCE framed, and a framing source's pair is freed at the relayed withdraw rather than only at peer removal | round 3 of the closure review, reading each page against the function its anchor names | all three corrected in the closure commit. The lesson: the pages a change makes wrong are found by grepping `docs/` for the CHANGED SYMBOL, and the 2026-08-29 change added `generatePath` and `fwdReleaseWithdrawnPathIDs` while every anchor still named `generate` |
| assumption | A-2 said the table cannot grow without bound because it is released at peer removal | it bounds the identifiers a peer is USING and says nothing about the ones it stops using; a withdraw creates an entry that nothing removes | the closure review, 2026-08-17 | Review Gate BLOCKER 1, spec stays open and, since 2026-08-29, the fix: the prefix-less key was the cause, so the key now carries the path for a source that frames identifiers, and the withdraw frees exactly the path it withdraws |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A re-advertised route carries ze's own Path Identifier, never the received one | Done | `fwdPathIDTable.generate`, `fwdPatchPathIDs`, `fwdReencodeNLRIs` | both rails |
| One generator, so replay and live forward agree | Done | package-level `fwdPathIDs`, read by both writers | `TestForwardPathIDIdenticalForEveryDestination` |
| `RFC7911-2-2` stops being a gap and gains both polarities | Done | `rfc/short/rfc7911.md`, `rfc/requirements/rfc7911.md` | no `{gap}` in the summary |
| The identifier state stays bounded | Done | `fwdPathIDTable.generatePath` and `releasePath`, `fwdReleaseWithdrawnPathIDs`, called from `RecentUpdateCache.evictLocked` and `Delete` | `TestForwardPathIDsFreedOnRelayedWithdraw`, `TestForwardPathIDWithdrawOfUnknownPathLeavesNothing` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestForwardPathIDStableAcrossUpdates` | asserts the emitted value is not the received 0xDEADBEEF |
| AC-2 | Done | `TestForwardPathIDsDifferForCollidingSources`, `TestForwardPathIDDiffersForTwoSourcePeers`, `bgp-addpath-readvertise-collision-frr` | |
| AC-3 | Done | `TestForwardPathIDSurvivesAttributeChange` | key is the ingress path, not the attributes |
| AC-4 | Done | `TestForwardPathIDMatchesAnnounceAndWithdraw`, `TestForwardPathIDReleaseReturnsValues`, `TestForwardPathIDsFreedOnRelayedWithdraw`, `TestForwardPathIDWithdrawCarriesTheAnnouncedValue` | the value returns to the pool at the relayed withdraw, and the withdraw carries the value the announcement carried |
| AC-5 | Done | `TestForwardPathIDBoundaryReceivedValues` | 0 and 2^32-1 both regenerated; `mintLocked` issues 0 like any value |
| AC-6 | Done | `TestForwardPathIDLeavesNonAddPathDestinationAlone` | asserts the same backing array and a nil handle |
| AC-7 | Done | `TestForwardPathIDIdenticalForEveryDestination`, `bgp-addpath-rail-agreement-speaker` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| all ten unit tests | Done | `forward_path_id_test.go`, `forward_path_id_gen_test.go` | present and green |
| boundary values 0 and 2^32-1 | Done | `TestForwardPathIDBoundaryReceivedValues` | |
| the two interop scenarios | Done | `test/interop/scenarios/bgp-addpath-readvertise-collision-frr`, `.../bgp-addpath-rail-agreement-speaker` | auto-discovered by `test/interop/run.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->, which lists the scenarios directory |
| `TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt` | Done | `internal/component/bgp/reactor/forward_path_id_churn_test.go` | Round 2 of the Review Gate added it: the release's one exception had no test, so deleting the branch reddened nothing |
| a test that drives the table's growth from the socket | Done | `internal/component/bgp/reactor/forward_path_id_churn_test.go`, `internal/component/bgp/reactor/zz_pathid_growth_probe_test.go` | the first drives `reactorForwardRS` with a published cache entry and asserts EXACT entry counts across announce/withdraw cycles. The second is the 2026-08-17 measurement probe, REWRITTEN rather than deleted: it asserts what it used to print, at the scale that makes the leak an attack. It now carries two `RFC7911-2-2` tags where it carried none |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `forward_path_id.go`, `forward_path_id_gen_test.go` | Done | created |
| `forward_body.go`, `context.go`, `reactor_peers.go` | Done | changed |
| `rfc/short/rfc7911.md`, `rfc/requirements/rfc7911.md`, `docs/features/rfc-status.md`, `internal/le/` | Done | judged-count pin dropped to 57 |
| the two `.ci` | Changed | replaced by the interop scenarios, with the reason in the TDD plan |
| `docs/architecture/bgp/structural-forwarding.md` | Done | gained "How long a Path Identifier lives", and its bucket-merge exclusion claim is corrected (Review Gate NOTE 4) |

### Audit Summary
- **Total items:** 22
- **Done:** 21
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the two `.ci`, recorded in Deviations)
- **Missing:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A route server does not merge two clients' paths for one prefix | interop | `bgp-addpath-readvertise-collision-frr/check.py` asserts FRR holds two paths under two identifiers, and its header records the 2026-08-14 lab measurement that reverting the generator leaves FRR holding ONE. NOT re-run in the closure session: it needs docker |
| The replay rail and the live rail put the same bytes on the wire | interop | `bgp-addpath-rail-agreement-speaker/check.py` compares the two UPDATE bodies and fails when the NLRI is not ADD-PATH framed, so a lost capability cannot pass it vacuously. NOT re-run in the closure session |
| `RFC7911-2-2` is proven in both polarities | ledger | `rfc/requirements/rfc7911.md` shows positive tags in `forward_path_id_gen_test.go` and the interop check, and the negative in `forward_path_id_test.go` |
| The fix costs the sessions that cannot use it nothing | unit | `TestForwardPathIDLeavesNonAddPathDestinationAlone` asserts the same backing array and a nil buffer handle |
| The work the release costs is bounded by one message, not by the product of two peer-chosen counts | unit + measurement | `fwdAnnouncedPaths` walks the announced section once, so an UPDATE costs one pass over each section rather than one pass per withdrawn NLRI. The scan it replaces was measured at 137ms for two 32000-octet sections, with `RecentUpdateCache.mu` held. `TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt` pins the behavior the set has to preserve |
| The state the fix adds stays bounded | unit | `TestForwardPathIDsFreedOnRelayedWithdraw` drives eight announce/withdraw cycles through `reactorForwardRS`, each under a fresh received identifier, and asserts the table holds EXACTLY one entry after the announce and EXACTLY zero after the withdraw. Removing the release from `evictLocked` and `Delete` reddens it at cycle 0 |

## Work Not Done

The section replaces this spec's Deferrals Resolved table. `plan/deferrals/` was
deleted on 2026-09-05 and the spec's Deferral shard field was `-`, so it held no
shard to resolve. <!-- doc-links: ignore (the directory was deleted on 2026-09-05 and this sentence is the record of it) -->

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| nothing. Every AC is implemented, and the two `.ci` the plan named were replaced by two interop scenarios for the reason the TDD Test Plan gives | - | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc7911-generate-own-path-id-zeclose-pathid.md` |
| `./le spec session review check` | CLEAN over 12 files, hashes matching. The command warns that it could not determine the running model, so the review-model boundary is UNCHECKED (`ai/rules/planning.md`): this agent runs Opus 5, and the artifact records no proof of it |
| Rounds | 4 |
| Reviewer lenses used | logic+wiring, security+allocation, performance+style, RFC conformance, documentation. Round 1 read the committed diff of `8c5bcc191`; round 2 read the complete diff, `2e22aebb1` and `d949eec53` included, which is the fix round 1's BLOCKER produced and which no round had ever read; round 3 read round 2's fixes and found the three stale pages; round 4 read round 3's fix and was clean |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The identifier table keeps an entry for every identifier a peer ever used. `generate` inserts one per distinct received identifier; the only release is `releaseSource`, called from `doRemovePeer` alone. A withdraw creates entries too (`fwdPatchPathIDs` runs over the withdrawn sections) and `reactorForwardRS` relays a received UPDATE without consulting a RIB, so withdrawals for paths that were never announced grow the table permanently. A five-octet withdraw NLRI buys about 30 to 40 octets of table, so one full-size UPDATE carries hundreds of them and an established route-server client exhausts the daemon's memory | `internal/component/bgp/reactor/forward_path_id.go` (`fwdPathIDTable.generate`, `fwdPathIDTable.releaseSource`, `fwdPatchPathIDs`), `internal/component/bgp/reactor/reactor_peers.go` (`doRemovePeer`) | FIXED 2026-08-29; the analysis below is kept because it is what forced the key change, and it is answered under "BLOCKER 1: the release contract, settled". ~~The fix is Go: release `(source, received)` when ze has relayed the withdraw for that ingress path, which is the reuse point AC-4 names.~~ A session-scoped release is not a substitute either, because `bgp-addpath-readvertise-collision-frr` pins that a session reset must not renumber. The closure session could not commit Go: the tree did not compile and a Go commit owes a full `./le verify current mode full` |

### BLOCKER 1: the release contract, settled (2026-08-29)

**A withdraw is not enough on its own, and the missing half was the KEY, not the
trigger.** The 2026-08-17 analysis below is correct and stands: one
`(source, received identifier)` key carried many prefixes, so releasing it on one
prefix's withdraw renumbered every other prefix still advertised under it. The
answer is to stop that key from carrying many prefixes.

**RFC 7911 Section 2, the sentence the contract rests on** (read in
`rfc/full/rfc7911.txt`, lines 132-136):

> "The assignment of the Path Identifier for a path by a BGP speaker is purely a
> local matter. However, the Path Identifier MUST be assigned in such a way that
> the BGP speaker is able to use the (Prefix, Path Identifier) to uniquely
> identify a path advertised to a neighbor."

The obligation is over the pairs currently ADVERTISED. A value is free to name a
second path once no advertisement of the first one is outstanding, and nothing in
the section makes an identifier permanent.

**The contract.** Ze mirrors the key the SOURCE uses to name a path.

| The source | Ze's key | Freed when |
|------------|----------|-----------|
| framed no Path Identifier (no ADD-PATH for the family) | the source | the peer is removed (`releaseSource`) |
| framed one | the source, the family, that identifier, and the NLRI bytes | ze has relayed the withdraw of that pair |

The first row cannot grow: every path of such a source arrives under received
identifier 0, so the source holds ONE entry for its whole session however many
prefixes it sends or churns. The second row is bounded by the paths ze
advertises, which is the exposure this feature exists for and the smallest exact
state that answers "is this pair still advertised".

**The release runs at ONE point, and it is neither rail: the recent-update cache
evicting the UPDATE that carried the withdraw** (`RecentUpdateCache.evictLocked`
and `Delete`, calling `fwdReleaseWithdrawnPathIDs`). One UPDATE reaches BOTH
rails -- `reactorForwardRS` serves the destinations it can and hands the rest to
the rs plugin as `FastPathSkipped`, which forwards them through
`forwardUpdateCore` -- so a release at the end of the first rail would mint a
fresh identifier for the second rail's destinations, and each would hold a route
ze can never withdraw. The same argument rules out releasing inside the
per-destination rewrite.

**Two edges, and how they are closed.**

| Edge | Answer |
|------|--------|
| A withdraw for a pair ze never advertised | mints an identifier no live path holds, writes it, and frees it at the same eviction. The destination has never seen the pair and RFC 7911 Section 5 has it silently ignore the withdraw. The cost is bounded by one UPDATE's NLRI count, not by the session (`TestForwardPathIDWithdrawOfUnknownPathLeavesNothing`) |
| An UPDATE that withdraws AND announces one pair (RFC 7606 Section 5.1 forbids the sender, ze must still accept it) | the release skips a pair the same UPDATE announces, so the destination's surviving route keeps the identifier that names it. The scan costs nothing for every conforming sender, whose UPDATE carries one of the two fields |

**Why it does not break `bgp-addpath-readvertise-collision-frr`.** That scenario
resets the DESTINATION's session (`clear bgp <ze>` at FRR) and asserts the
identifiers do not move. Its two colliding sources are bird and gobgp, neither of
which carries an add-path stanza, so both sit in the first row of the table above:
one permanent entry each, nothing released by a session reset and nothing
released by a withdraw. The destination's reset frees nothing because a
destination holds no entries at all.

**The 2026-08-17 measurement probe was rewritten, not deleted.**
`zz_pathid_growth_probe_test.go` is TRACKED (it landed in `df44d8d27`, and its own
header claiming otherwise was wrong). Its
`TestProbeOneReceivedIDCoversManyPrefixes` asserted "one key, one identifier,
several prefixes", which is the property this fix deliberately removes, so it had
to move with the contract. Its subject survives the change: how many prefixes one
received identifier covers is still the question, and the answer is now decided by
what the SOURCE framed. It is `TestPathIDKeyFollowsWhatTheSourceFramed`, and it
pins all three cases -- a framing source on the raw rail, a framing source on the
re-encode rail, and a source that frames none. Its sibling is
`TestWithdrawOnlyUpdateFreesEveryIdentifierItBuys`, which asserts the number it
used to log: one withdraw-only UPDATE of 1623 octets buys 200 entries and holds
none once the cache evicts it. That is 8.1 octets of wire per entry, cheaper for
the sender than the 30 to 40 octets the Review Gate finding estimated.

**What is NOT observed, stated plainly.** The release is driven by the withdraw
ze RELAYS. A path a source stops advertising WITHOUT sending a withdraw -- the
session goes down, or the peer is torn down -- is not observed here, and its
entries live until `doRemovePeer` calls `releaseSource`. That is bounded by the
peer's advertised paths and it is deliberate: an entry freed at session down
would renumber the paths a reconnecting peer re-announces, which is the property
the collision scenario pins.

### BLOCKER 1: why the PREFIX-LESS key made "release on the relayed withdraw" wrong (2026-08-17, answered above)

**One table key carries MANY prefixes, by design, so releasing it on one
prefix's withdraw renumbers every other prefix still advertised under it.**

The key is `(source, received identifier)` and holds no prefix. The table's own
doc comment (`fwdPathIDTable`, `internal/component/bgp/reactor/forward_path_id.go`)
states the intent: "A source that reuses one identifier across prefixes
therefore costs one entry rather than one per prefix, which keeps the ordinary
case -- a client that negotiated no ADD-PATH, whose every path arrives with
identifier 0 -- at a single entry for its whole session." `fwdReencodeNLRIs`
(`forward_body.go`) says the same from the read side: identifier 0 "is then the
ingress key of every path that source sends". RFC 7911 also permits an ADD-PATH
source to reuse one identifier across prefixes, and FRR and BIRD do.

Releasing on a relayed withdraw therefore trades an unbounded table for
PERMANENT PHANTOM ROUTES: the next re-advertisement of any prefix still using
that key mints a fresh identifier, the destination keeps `(prefix, old id)`
forever with no withdraw ze can ever send, and a duplicate appears under the new
identifier. Both properties this spec protects break too — announce and withdraw
share one identifier only until some OTHER prefix's withdraw releases the key.

→ Decision: refcounting the key does not rescue it. Announce-twice/withdraw-once
leaves the count above zero forever, which is the unbounded case again, and a
withdraw for a never-announced prefix under a live key drives the count below the
true value, which is the phantom-route case. Both are cheap peer input.

→ Decision: a stateless deterministic mapping is REFUSED on security grounds. A
route-server client sees ze's identifiers for its own paths, inverts the mapping,
and announces a colliding identifier to displace another client's path.

→ Decision: a per-source cap stays refused. It drops routes, which is the defect
this spec exists to remove.

**Three candidate designs remain, and the choice is open for the next session.**

| # | Design | Cost, and the catch |
|---|--------|---------------------|
| a | The reactor keeps a per-`(source, received)` set of the live advertised NLRIs and releases the key when that set empties | Exact, and bounded by advertised paths. Costs one map operation per NLRI on the forward path, plus adj-rib-in-sized memory |
| b | Drive the release from the per-source announced-NLRI set the route-server plugin already keeps (`applyNLRIRecords`, `internal/component/bgp/plugins/rs/server_inventory.go`, whose own comment records that it runs after forwarding and off the critical path) | No new memory, but its unicast key is the prefix alone with no identifier, so it cannot answer "is any path still using received id r"; and a core memory bound would then depend on a plugin |
| c | Assign identifiers per prefix from stored paths, the FRR and BIRD model | The largest change, and it ends the prefix-less key that causes this |

**Unmeasured, and it must be measured before (a) is sized.** That withdrawals
create entries is a CODE READ, not a measurement: `fwdRegenerateRawPathIDs`
calls `fwdPatchPathIDs` on the withdrawn section and on `MPUnreach`'s withdrawn
bytes, and `fwdPatchPathIDs` reaches `fwdPathIDTable.generate`, which inserts.
No growth number has been observed, and the growth test AC-4 needs must drive
from the SOCKET — a withdraw-only UPDATE arriving and being relayed — because a
test that calls `generate` directly proves nothing about reachability from the
wire, and reachability is what makes this a defect.

→ Constraint: growth needs a source that uses MANY DISTINCT received
identifiers. A non-ADD-PATH source sends every path under identifier 0 and costs
ONE entry for its whole session, so the exposure is the ADD-PATH route-server
case this feature exists for, not every peer.
| 2 | NOTE | The same patched payload is copied and re-walked once per destination, though ze's identifiers are destination-independent. `fwdParseCache` is already the per-source memo across that fan-out and does not carry the patched payload | `internal/component/bgp/reactor/forward_path_id.go` (`fwdRegenerateRawPathIDs`), `internal/component/bgp/reactor/forward_rs.go` | Not fixed, not blocking: the buffer is pooled, the shape matches the existing RFC 6793 transcode, and sharing one buffer needs adopt-once handle work |
| 3 | NOTE | The generator's tests share the package global and two source constants, so two of them key on the same (source, received) pair | `internal/component/bgp/reactor/forward_path_id_gen_test.go` | Not fixed, not blocking: both assert only inequality and stability, and the package passes race-instrumented |
| 4 | NOTE | `docs/architecture/bgp/structural-forwarding.md` claims the bucket-merge conditions exclude every copy-on-modify path | `docs/architecture/bgp/structural-forwarding.md` | Fixed 2026-08-29: the claim now excepts the Path Identifier rewrite and says why merging it is correct, and the page gained "How long a Path Identifier lives" |

### Run 2 (2026-09-05, over the complete diff)

Round 1 read `8c5bcc191` alone and its BLOCKER was fixed in two later commits,
`2e22aebb1` and `d949eec53`, that no round had read. Round 2 read all four
commits of the spec together, `2061e80cb` and `2383e2c99` included, so the
release contract, the eviction ordering and the source-preserving rebuilds were
judged for the first time.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 5 | ISSUE | The release answered "does this same UPDATE also announce the pair I am about to free" by walking the announced section once per withdrawn NLRI. Both counts are chosen by one peer inside one message, and the walk runs with `RecentUpdateCache.mu` held, so an ADD-PATH client stalls every peer's UPDATE ingest. Two 32000-octet sections of an RFC 8654 extended UPDATE (`message.ExtMsgLen`) cost 41 million comparisons, measured at 137ms, repeatable at line rate. The exception itself is correct and must stay: freeing a pair the same UPDATE announces strands it at the destination | `internal/component/bgp/reactor/forward_path_id.go` (`fwdReleaseSection`, `fwdSectionCarries`) | `fwdSectionCarries` is deleted and `fwdAnnouncedPaths` keys the announced section once into a `map[fwdPathKey]struct{}`, so the release answers with a lookup and each section is walked once. A conforming withdraw carries no NLRI (RFC 7606 Section 5.1) and builds no map at all |
| 6 | ISSUE | The exception nothing asserted. `fwdSectionCarries` existed only to keep a pair the same UPDATE announces, and no test drove an UPDATE carrying both fields, so deleting the branch reddened nothing | `internal/component/bgp/reactor/forward_path_id_churn_test.go` | `TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt` relays announce, announce, withdraw-both-announce-one, re-announce, and asserts the surviving pair keeps the identifier the destination already holds for it |
| 7 | ISSUE | The `RFC7911-2-2 negative` tag on `TestForwardPathIDStableAcrossUpdates` still carried its TDD narration: "That half fails today: ze copies the ingress value". A tag's prose is what `rfc/requirements/rfc7911.md` publishes, so the ledger stated a failure the fix had already closed | `internal/component/bgp/reactor/forward_path_id_test.go` | The claim is kept and the two false present-tense sentences are replaced by what the body asserts |

### Run 2 verified, and found nothing to fix

| Lens | What was read | Result |
|------|---------------|--------|
| Release completeness | `evictLocked` and `Delete` are the only two sites that remove an entry, and every received UPDATE reaches the cache: `notifyMessageReceiver` gates on `buf.Buf != nil`, and the treat-as-withdraw split gives each extra family a `noPoolBufID` handle for exactly that reason (`session_read.go`) | Clean |
| Eviction ordering | `d949eec53` moved the walk before `ReturnReadBuffer` at both sites, and `TestEvictionWalksTheBodyBeforeItFreesTheBuffer` reads the source and fails on the old order | Clean |
| Key agreement across the rails | `NLRIIterator.Next` returns the length octet with the prefix, which is the framing `fwdPatchPathIDs` and `fwdReleaseSection` slice, so the raw rail, the re-encode rail and the release compute one key | Clean |
| Allocation on the forward path | Escape analysis reports `key does not escape` in `generatePath` and `releasePath`, and `m does not escape` in `fwdPathIDMemo.framed`, so no NLRI costs a heap object (R-2). The two heap lines are the documented pool fallback and the `copied` wrapper the file's own comment names | Clean |
| Peer-reachable panic | `forward_path_id.go` and `recent_cache.go` carry no `panic`, and every truncated section returns an error | Clean |
| RFC text | RFC 7911 Section 2 and Section 5 quoted in the code and in this spec match `rfc/full/rfc7911.txt` lines 133-140 and 231-233 word for word | Clean |

### Run 3 scope, declared before it ran

Round 3 reads ONLY what round 2's fixes changed, plus the eight always-in-scope
classes anywhere in the diff (`ai/rules/planning.md`): `fwdAnnouncedPaths` and
the rewritten `fwdReleaseSection`,
`TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt`, the corrected
`RFC7911-2-2 negative` tag prose, the corrected enrolment reason in
`rfc/short/rfc7911.md`, and the paragraph added to
`docs/architecture/bgp/structural-forwarding.md`.

### Run 3 result: 1 ISSUE, fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 8 | ISSUE | Three user-facing pages still described the PRE-2026-08-29 identifier. Each said the key is "(source peer, received identifier)" and that identifiers are "released at peer removal, not at session down", which is now only the unframed half: a source that frames identifiers is keyed per (family, identifier, prefix) and freed at the relayed withdraw. `docs/features.md` also anchored on `fwdPathIDTable.generate`, which since the key change serves only the unframed case | `docs/features.md` (Path Identifier regeneration row), `docs/features/bgp-protocol.md`, `docs/guide/add-path.md` | All three now state both halves of the key and both release points, and the anchors name `generatePath` and `fwdReleaseWithdrawnPathIDs` beside `generate` |

The finding sits outside the declared round-3 scope and was fixed inside it
anyway: `ai/rules/documentation.md` is always-on, and a page a change made wrong
is repaired in the work that broke it rather than reported.


| Lens | What was read | Result |
|------|---------------|--------|
| Behavior of the replacement | A nil map answers every lookup with "no", which is what an empty announced section must produce, and it is what `fwdAnnouncedPaths` returns for one. A malformed tail ends the walk, so the pairs before it are keyed and the pairs after it are not, which is at least what `fwdSectionCarries` gave: it read a malformed tail as absent | Clean |
| The one call-order change | `fwdPathKeyFor` now runs for every withdrawn pair rather than only for the pairs not announced. Its single error is an NLRI longer than 33 octets, which one length octet cannot describe, so no wire input reaches it | Clean |
| Always-in-scope: unwired symbol | `fwdAnnouncedPaths` has one non-test caller, `fwdReleaseSection`, and `fwdSectionCarries` is gone rather than left behind | Clean |
| Always-in-scope: vacuous test | The new test was observed RED by name under a Go overlay that removes the exception, and the three other release tests stayed green in that same run | Clean |
| Always-in-scope: removed guard | The exception is preserved, not removed. Only how it is answered changed | Clean |
| Allocation | The map is the deliberate cost and is documented with its bound. Escape analysis over the package reports `key does not escape` in `fwdAnnouncedPaths`, so the key stays on the stack and only the map reaches the heap | Clean |
| Style | Guard clauses, an invariant stated positively, the bound stated as a number, and no `panic` on a path a peer reaches (`docs/contributing/ze-go-style.md`) | Clean |

### Run 4 scope, declared before it ran

Round 4 reads ONLY round 3's fix: the three pages above, each claim checked
against the function that produces it, plus the eight always-in-scope classes.

### Run 4 result: 0 BLOCKER, 0 ISSUE

| Lens | What was read | Result |
|------|---------------|--------|
| The key, per page | `fwdPathIDTable.generate` keys `(source, received)` for a source that frames none and `generatePath` keys `fwdPathKey{family, received, nlri}` for one that frames them. All three pages now say both, and no page states one as the whole rule | Clean |
| The release, per page | `releaseSource` runs from `doRemovePeer` alone, and `releasePath` runs from `fwdReleaseSection` under `fwdReleaseWithdrawnPathIDs`, whose only callers are `evictLocked` and `Delete`. Nothing releases at session down, which all three pages still say | Clean |
| Anchors resolve | Every symbol named in a `<!-- source: -->` line this round added is declared in the file the line names | Clean |
| Always-in-scope | No code changed in round 3, so no unwired symbol, no vacuous test and no guard moved | Clean |

## Pre-Commit Verification

### Gate Result, with attribution

`./le verify current mode full` exits 1 over this checkout (2026-09-05, log at
`tmp/session/2026-09-05-zeclose-pathid/scratch/verify-full.log`). Every failure
is another session's or the loaded box's, and each was checked rather than
assumed:

| Red | Attribution |
|-----|-------------|
| lint typecheck: `internal/le/staticcheckfeaturematrix` cannot import `internal/le/changed` | Another session has `internal/le/changed/selector_test.go` and two new files under `internal/le/verify/engine/` open in this checkout |
| 9 functional suites, 64 tests | A sibling closure agent ran the same gate concurrently over a tree WITHOUT this change and failed 61 tests, 48 of them the same ones. The 16 that failed only here were re-run on a quiet box: `ipv4-announce-withdraw`, `modify-oversize-suppress`, `wire-edit-api-origin-order` and `bgp-rs-reactor-fastpath-fallback` pass, and `bgp-rs-fastpath-ebgp-shared` passes 3 times out of 3. Its recorded failure type was `timeout` with the expected and received bytes IDENTICAL, which is the shape a loaded box produces |
| `cli-grammar`, `install`, `appliance`, `web` | `--flag in YANG` findings in `internal/component/hub/yang/ze-hub-conf.yang`, `gokrazy/modcache looks unpopulated`, and `net::ERR_CERT_AUTHORITY_INVALID`. None names a file this spec touched |
| `./le doc check verify` exit 1 | Two summary rules over the command tree, `../gh-pages/` surfaces not published here, and three anchors at `docs/features.md:44` and `:100`. No finding names a Path Identifier page |

The reactor's own package is green: `go test -race` over
`./internal/component/bgp/reactor` under `./le job run` exits 0 (1.775s), and the
four release tests pass on the intact tree while the new one fails by name under
the overlay that removes the exception.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/forward_path_id.go` | Yes | read in full during the review |
| `internal/component/bgp/reactor/forward_path_id_gen_test.go` | Yes | read in full during the review |
| `internal/component/bgp/reactor/forward_path_id_churn_test.go` | Yes | read in full in round 2; it gained `TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt` |
| `internal/component/bgp/reactor/recent_cache_order_test.go` | Yes | read in full in round 2 |
| `plan/journal/membership-answered-by-a-scan-per-item.md` | Yes | the class round 2's ISSUE 5 opened |
| `test/interop/scenarios/bgp-addpath-readvertise-collision-frr/` | Yes | `check.py`, `ze.conf`, `frr.conf`, `bird.conf`, `gobgp.toml` |
| `test/interop/scenarios/bgp-addpath-rail-agreement-speaker/` | Yes | `check.py`, `ze.conf`, `gobgp.toml`, `speaker-args`, `speaker2-args` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-3, AC-4 | a pair the same UPDATE withdraws and announces keeps the identifier the destination holds for it | 2026-09-05, race-instrumented, and observed in BOTH phases. Under a Go overlay that makes `fwdReleaseSection` ignore the announced section, `TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt` FAILS by name at `forward_path_id_churn_test.go:344`, "expected: 1, actual: 0", while the three other release tests stay green: nothing else covered the exception. Over the intact tree the same four pass in 2.155s. The overlay changed no file, so the five other sessions sharing this checkout saw nothing |
| AC-1, AC-3 | the emitted identifier is ze's own and stable | the retired `ze-unit-pkg-test PKG=./internal/component/bgp/reactor` (current: `go test -race ./internal/component/bgp/reactor`) green on 2026-08-17, race-instrumented, 149.5s, `TestForwardPathIDStableAcrossUpdates` included |
| AC-2 | two colliding sources leave under different identifiers | same run, `TestForwardPathIDsDifferForCollidingSources` and `TestForwardPathIDDiffersForTwoSourcePeers` |
| AC-4 | announce and withdraw share one identifier, and release returns values | same run, `TestForwardPathIDMatchesAnnounceAndWithdraw` and `TestForwardPathIDReleaseReturnsValues`; the bound is the Review Gate blocker |
| AC-5 | 0 and 2^32-1 are values, not absences | same run, `TestForwardPathIDBoundaryReceivedValues` |
| AC-6 | a destination without ADD-PATH is untouched | same run, `TestForwardPathIDLeavesNonAddPathDestinationAlone`, which asserts the same backing array |
| AC-7 | one path, one identifier, every destination | same run, `TestForwardPathIDIdenticalForEveryDestination` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| UPDATE forwarded to an ADD-PATH peer | `test/interop/scenarios/bgp-addpath-readvertise-collision-frr/check.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> | Yes: read the file. It negotiates ADD-PATH with FRR, checks the capability before asserting, injects the second path from GoBGP and reads FRR's `show bgp ipv4 unicast detail json`, which is the only form carrying the identifier |
| Peer-up replay to an ADD-PATH peer | `test/interop/scenarios/bgp-addpath-rail-agreement-speaker/check.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> | Yes: read the file. It asserts speaker2 was NOT established when the route was stored, so its copy can only be the replay, and it fails when the NLRI is not ADD-PATH framed |
| The same path advertised twice | `forward_path_id_test.go` `TestForwardPathIDStableAcrossUpdates` | Yes: drives `buildFwdBody` twice and compares the identifier field of each destination frame |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `buildFwdBody`'s same-context branch is a second writer; both read one table |
| A-2 | confirmed 2026-08-29 | the bound holds once the key carries the path: one entry per session for a source that frames no identifier, one per advertised path for a source that frames one |

### Round 2 fixes verified
| Fix | Fresh Evidence |
|-----|----------------|
| The release walks each section once | `fwdSectionCarries` is gone from `internal/component/bgp/reactor/forward_path_id.go`, and `fwdAnnouncedPaths` is its one replacement, read by `fwdReleaseSection` alone |
| The stale ledger prose | `grep -c "{gap}" rfc/short/rfc7911.md` is 0, and the Enrolment reason no longer says ze preserves the ingress identifier |
| No new allocation on the forward path | escape analysis over the package reports `key does not escape` for `generatePath` and `releasePath`, so the map lookup key stays on the stack |

### Documentation Verified

`./le doc check verify` exits 1 over this checkout, and every finding is another
session's: two summary rules over the command tree, `../gh-pages/` per-command
surfaces that are not published here, and three source-anchor CLAIMs at
`docs/features.md:44` and `:100` naming `getHelpExtension`, `Node.Help` and
`answerZeroTunnelIDSCCRQ`, the last of which is in `internal/component/l2tp/reactor.go`,
a file another session has open right now. No finding names a Path Identifier
page, a Path Identifier anchor, or any file this spec touched, and the run
reports "checked 3026 anchors across 23 digests, all resolve".

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 9. RFC compliance | `rfc/short/rfc7911.md` carries `RFC7911-2-2` with no `{gap}`; `rfc/requirements/rfc7911.md` shows both polarities and the interop tag; `docs/features/rfc-status.md` records "Closed 2026-08-14: RFC7911-2-2" | Yes |
| 9. RFC compliance, feature pages | `docs/features.md` and `docs/features/bgp-protocol.md` describe the ingress key, the legality of 0 and the live-value set, each anchored on `forward_path_id.go`; each claim matches `fwdPathIDTable` | Yes |
| 12. Internal architecture | `docs/architecture/bgp/structural-forwarding.md` gained "How long a Path Identifier lives", and its bucket-merge exclusion claim is corrected | Yes, 2026-08-29 |
| 16. Source anchors | `grep -rn "forward_path_id.go" docs/` names five pages. Round 3 found three of them describing the pre-2026-08-29 key and release and corrected all three, so every Path Identifier claim in `docs/` now matches `fwdPathIDTable.generate`, `generatePath` and `fwdReleaseWithdrawnPathIDs` | Yes, 2026-09-05 |
| 12. Internal architecture, the release | `docs/architecture/bgp/structural-forwarding.md` gained the one exception the release makes and why the announced section is keyed rather than walked, and `docs/architecture/memory/lifetime-contracts.md` carries the walk-before-free ordering | Yes, 2026-09-05 |
