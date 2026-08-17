# Spec: rfc7911-generate-own-path-id

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `-` |
| Updated | 2026-08-17 |

**The closure review found a BLOCKER, so this spec stays open.** The generator's
table keeps an entry for every identifier a peer ever used, and a peer grows it
from the socket. The finding, its producing function and the two fix shapes are in
the Review Gate section at the end of this file. The fix is Go and was NOT made in
the closure session: the tree did not compile (other sessions mid-TDD) and a Go
commit owes a full `make ze-precommit-verify`.

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
| A destination without ADD-PATH receives bare prefixes | `fwdReencodeNLRIs` early return on `srcAddPath == destAddPath` | Re-framing is per destination and must stay so |
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
| A-2 | An identifier can be held per destination and family without unbounded growth | the RIB already keys paths per peer | the generator needs its own eviction | count the live paths a route server holds | confirmed 2026-08-14, with the key corrected. The identifier is held per (source, received identifier) rather than per destination and family: a client that negotiated no ADD-PATH costs ONE entry for its whole session, and a client that sends ADD-PATH costs one per identifier it uses. Released at peer removal (`doRemovePeer`) |

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
| UPDATE forwarded to an ADD-PATH peer | wire → forward rail → `fwdReencodeNLRIs` → generator | `addpath-readvertise-collision-frr` (interop) |
| Peer-up replay to an ADD-PATH peer | replay rail → `fwdReencodeNLRIs` → same generator | `addpath-rail-agreement-speaker` (interop) |
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
| 1 | Runs ze as a route server between two clients whose paths share one received identifier | wire → egress transform → generator → both paths survive at the third client | `addpath-readvertise-collision-frr` |
| 2 | A client reconnects and receives the peer-up replay | replay rail → same generator → identical bytes | `addpath-rail-agreement-speaker`, and the session-reset half of `addpath-readvertise-collision-frr` |

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
| `test/interop/scenarios/addpath-readvertise-collision-frr` | FRR 10.3.1, with BIRD and GoBGP as the colliding sources | AC-2: FRR keeps both paths, under identifiers 0 and 1. Reverting the generator leaves FRR holding ONE path |
| `test/interop/scenarios/addpath-rail-agreement-speaker` | two independent Python speakers, one live and one replayed | AC-7: both UPDATE bodies are byte-identical, Path Identifier included |

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/bgp/reactor/forward_body.go` | `fwdReencodeNLRIs` reads a generated identifier instead of copying the received one, and `buildFwdBody`'s same-context branch rewrites the identifiers of the raw frame before it splits or forwards it |
| `internal/core/bgp/context/context.go` | `AnyAddPath`, so the forward rail can skip every session that negotiated ADD-PATH for nothing before it reads an UPDATE's sections |
| `internal/component/bgp/reactor/reactor_peers.go` | `doRemovePeer` releases the removed peer's identifiers |
| `scripts/dev/rfc_requirements_test.py` | the judged-count pin drops to 57: RFC 7911's row stops spelling a gap count when its last `{gap}` closes |
| `rfc/short/rfc7911.md` | `RFC7911-2-2` loses its `{gap}` and gains both polarities |
| `docs/features/rfc-status.md` | the RFC 7911 row's Remaining count and prose |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/bgp/reactor/forward_path_id.go` | the generator, keyed by the path at ingress, plus the raw-frame rewriter the same-context branch uses |
| `internal/component/bgp/reactor/forward_path_id_gen_test.go` | the generator's tests: withdraw matching, replacement, non-ADD-PATH source and destination, boundary values, destination agreement, release |
| ~~the two `.ci` named above~~ | Replaced by the two interop scenarios in the TDD Test Plan. The `.ci` harness cannot express either assertion, and the property AC-2 names is a receiver's, so no test inside ze can report it |
| `test/interop/scenarios/addpath-readvertise-collision-frr/` | AC-2 at FRR: `ze.conf`, `frr.conf`, `bird.conf`, `gobgp.toml`, `check.py` (carries the `RFC7911-2-2 positive` interop tag) |
| `test/interop/scenarios/addpath-rail-agreement-speaker/` | AC-7 byte identity: `ze.conf`, `gobgp.toml`, `speaker-args`, `speaker2-args`, `check.py` |

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
| The public ledger agrees | `make ze-rfc-check` green |
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
- [ ] `make ze-rfc-check` green with `RFC7911-2-2` carrying both polarities
- [ ] `make ze-precommit-verify` green, or scoped evidence with attribution

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
- Two interop scenarios: `addpath-readvertise-collision-frr` (AC-2 at FRR) and
  `addpath-rail-agreement-speaker` (AC-7 byte identity), plus `--add-path` on the
  interop speaker engine.

### Bugs Found/Fixed
- `TestForwardSplitSameContextKeepsRawSplit` asserted the emitted NLRI equalled the
  source NLRI, which pinned the violation. Corrected with the owner's approval token
  on 2026-08-14: the prefixes are still compared, the identifiers are now asserted
  to be ze's own and to be unique per path.

### Documentation Updates
- `docs/features/rfc-status.md` (RFC 7911 row), `rfc/short/rfc7911.md`,
  `rfc/requirements/rfc7911.md`, `ai/RFC-REQUIREMENTS.md`.
- `docs/features.md`, `docs/features/bgp-protocol.md` and `docs/guide/add-path.md`
  carry the feature and its source anchors on `forward_path_id.go`, verified against
  `fwdPathIDTable.generate` today.
- STILL OWED, and left for the session that fixes the Review Gate blocker:
  `docs/architecture/bgp/structural-forwarding.md` is anchored to
  `forward_body.go` and says the bucket-merge conditions "exclude every
  copy-on-modify path ... because their bytes differ per destination". The Path
  Identifier rewrite is a copy path that satisfies all three conditions, and
  merging it is safe for the opposite reason: its bytes are the same for every
  destination. That page owes the generator, that sentence, and the release
  contract the blocker's fix will settle.

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
| assumption | A-2 said the table cannot grow without bound because it is released at peer removal | it bounds the identifiers a peer is USING and says nothing about the ones it stops using; a withdraw creates an entry that nothing removes | the closure review, 2026-08-17 | Review Gate BLOCKER 1, spec stays open |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A re-advertised route carries ze's own Path Identifier, never the received one | Done | `fwdPathIDTable.generate`, `fwdPatchPathIDs`, `fwdReencodeNLRIs` | both rails |
| One generator, so replay and live forward agree | Done | package-level `fwdPathIDs`, read by both writers | `TestForwardPathIDIdenticalForEveryDestination` |
| `RFC7911-2-2` stops being a gap and gains both polarities | Done | `rfc/short/rfc7911.md`, `rfc/requirements/rfc7911.md` | no `{gap}` in the summary |
| The identifier state stays bounded | Partial | `fwdPathIDTable.releaseSource`, called only from `doRemovePeer` | Review Gate BLOCKER 1 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestForwardPathIDStableAcrossUpdates` | asserts the emitted value is not the received 0xDEADBEEF |
| AC-2 | Done | `TestForwardPathIDsDifferForCollidingSources`, `TestForwardPathIDDiffersForTwoSourcePeers`, `addpath-readvertise-collision-frr` | |
| AC-3 | Done | `TestForwardPathIDSurvivesAttributeChange` | key is the ingress path, not the attributes |
| AC-4 | Partial | `TestForwardPathIDMatchesAnnounceAndWithdraw`, `TestForwardPathIDReleaseReturnsValues` | reuse ordering is correct and conservative; the release is so late that the table is unbounded (BLOCKER 1) |
| AC-5 | Done | `TestForwardPathIDBoundaryReceivedValues` | 0 and 2^32-1 both regenerated; `mintLocked` issues 0 like any value |
| AC-6 | Done | `TestForwardPathIDLeavesNonAddPathDestinationAlone` | asserts the same backing array and a nil handle |
| AC-7 | Done | `TestForwardPathIDIdenticalForEveryDestination`, `addpath-rail-agreement-speaker` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| all ten unit tests | Done | `forward_path_id_test.go`, `forward_path_id_gen_test.go` | present and green |
| boundary values 0 and 2^32-1 | Done | `TestForwardPathIDBoundaryReceivedValues` | |
| the two interop scenarios | Done | `test/interop/scenarios/addpath-readvertise-collision-frr`, `.../addpath-rail-agreement-speaker` | auto-discovered by `test/interop/run.py`, which lists the scenarios directory |
| a test that drives the table's growth from the socket | Missing | -- | owed with the BLOCKER 1 fix |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `forward_path_id.go`, `forward_path_id_gen_test.go` | Done | created |
| `forward_body.go`, `context.go`, `reactor_peers.go` | Done | changed |
| `rfc/short/rfc7911.md`, `rfc/requirements/rfc7911.md`, `docs/features/rfc-status.md`, `scripts/dev/rfc_requirements_test.py` | Done | judged-count pin dropped to 57 |
| the two `.ci` | Changed | replaced by the interop scenarios, with the reason in the TDD plan |
| `docs/architecture/bgp/structural-forwarding.md` | Missing | see Documentation Updates |

### Audit Summary
- **Total items:** 21
- **Done:** 17
- **Partial:** 2 (the bound on the identifier state, and AC-4 with it)
- **Skipped:** 0
- **Changed:** 1 (the two `.ci`, recorded in Deviations)
- **Missing:** 2 (the architecture page, and a test driving the table's growth)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A route server does not merge two clients' paths for one prefix | interop | `addpath-readvertise-collision-frr/check.py` asserts FRR holds two paths under two identifiers, and its header records the 2026-08-14 lab measurement that reverting the generator leaves FRR holding ONE. NOT re-run in the closure session: it needs docker |
| The replay rail and the live rail put the same bytes on the wire | interop | `addpath-rail-agreement-speaker/check.py` compares the two UPDATE bodies and fails when the NLRI is not ADD-PATH framed, so a lost capability cannot pass it vacuously. NOT re-run in the closure session |
| `RFC7911-2-2` is proven in both polarities | ledger | `rfc/requirements/rfc7911.md` shows positive tags in `forward_path_id_gen_test.go` and the interop check, and the negative in `forward_path_id_test.go` |
| The fix costs the sessions that cannot use it nothing | unit | `TestForwardPathIDLeavesNonAddPathDestinationAlone` asserts the same backing array and a nil buffer handle |
| The state the fix adds stays bounded | NOT MET | Review Gate BLOCKER 1 |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none: the spec's Deferral shard field is `-`, and no `plan/deferrals/rfc7911-generate-own-path-id.md` exists | done | `ls plan/deferrals/` shows no shard for this stem |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc7911-generate-own-path-id-7584d469-e988-48fc-910f-c68d4a139d89.md` |
| `review_gate.py check` | BLOCKED (verdict `findings`), which is correct: the gate must refuse this closure |
| Rounds | 1 |
| Reviewer lenses used | logic+wiring, security+allocation, performance+style, run inline by the closure agent over the committed diff |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The identifier table keeps an entry for every identifier a peer ever used. `generate` inserts one per distinct received identifier; the only release is `releaseSource`, called from `doRemovePeer` alone. A withdraw creates entries too (`fwdPatchPathIDs` runs over the withdrawn sections) and `reactorForwardRS` relays a received UPDATE without consulting a RIB, so withdrawals for paths that were never announced grow the table permanently. A five-octet withdraw NLRI buys about 30 to 40 octets of table, so one full-size UPDATE carries hundreds of them and an established route-server client exhausts the daemon's memory | `internal/component/bgp/reactor/forward_path_id.go` (`fwdPathIDTable.generate`, `fwdPathIDTable.releaseSource`, `fwdPatchPathIDs`), `internal/component/bgp/reactor/reactor_peers.go` (`doRemovePeer`) | NOT FIXED. The fix is Go: release `(source, received)` when ze has relayed the withdraw for that ingress path, which is the reuse point AC-4 names. A session-scoped release is not a substitute, because `addpath-readvertise-collision-frr` pins that a session reset must not renumber. The closure session could not commit Go: the tree did not compile and a Go commit owes a full `make ze-precommit-verify` |
| 2 | NOTE | The same patched payload is copied and re-walked once per destination, though ze's identifiers are destination-independent. `fwdParseCache` is already the per-source memo across that fan-out and does not carry the patched payload | `internal/component/bgp/reactor/forward_path_id.go` (`fwdRegenerateRawPathIDs`), `internal/component/bgp/reactor/forward_rs.go` | Not fixed, not blocking: the buffer is pooled, the shape matches the existing RFC 6793 transcode, and sharing one buffer needs adopt-once handle work |
| 3 | NOTE | The generator's tests share the package global and two source constants, so two of them key on the same (source, received) pair | `internal/component/bgp/reactor/forward_path_id_gen_test.go` | Not fixed, not blocking: both assert only inequality and stability, and the package passes race-instrumented |
| 4 | NOTE | `docs/architecture/bgp/structural-forwarding.md` claims the bucket-merge conditions exclude every copy-on-modify path | `docs/architecture/bgp/structural-forwarding.md` | Not fixed: it belongs with the BLOCKER 1 fix, which settles the release contract the same page owes |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/forward_path_id.go` | Yes | read in full during the review |
| `internal/component/bgp/reactor/forward_path_id_gen_test.go` | Yes | read in full during the review |
| `test/interop/scenarios/addpath-readvertise-collision-frr/` | Yes | `check.py`, `ze.conf`, `frr.conf`, `bird.conf`, `gobgp.toml` |
| `test/interop/scenarios/addpath-rail-agreement-speaker/` | Yes | `check.py`, `ze.conf`, `gobgp.toml`, `speaker-args`, `speaker2-args` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-3 | the emitted identifier is ze's own and stable | `make ze-unit-pkg-test PKG=./internal/component/bgp/reactor` green on 2026-08-17, race-instrumented, 149.5s, `TestForwardPathIDStableAcrossUpdates` included |
| AC-2 | two colliding sources leave under different identifiers | same run, `TestForwardPathIDsDifferForCollidingSources` and `TestForwardPathIDDiffersForTwoSourcePeers` |
| AC-4 | announce and withdraw share one identifier, and release returns values | same run, `TestForwardPathIDMatchesAnnounceAndWithdraw` and `TestForwardPathIDReleaseReturnsValues`; the bound is the Review Gate blocker |
| AC-5 | 0 and 2^32-1 are values, not absences | same run, `TestForwardPathIDBoundaryReceivedValues` |
| AC-6 | a destination without ADD-PATH is untouched | same run, `TestForwardPathIDLeavesNonAddPathDestinationAlone`, which asserts the same backing array |
| AC-7 | one path, one identifier, every destination | same run, `TestForwardPathIDIdenticalForEveryDestination` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| UPDATE forwarded to an ADD-PATH peer | `test/interop/scenarios/addpath-readvertise-collision-frr/check.py` | Yes: read the file. It negotiates ADD-PATH with FRR, checks the capability before asserting, injects the second path from GoBGP and reads FRR's `show bgp ipv4 unicast detail json`, which is the only form carrying the identifier |
| Peer-up replay to an ADD-PATH peer | `test/interop/scenarios/addpath-rail-agreement-speaker/check.py` | Yes: read the file. It asserts speaker2 was NOT established when the route was stored, so its copy can only be the replay, and it fails when the NLRI is not ADD-PATH framed |
| The same path advertised twice | `forward_path_id_test.go` `TestForwardPathIDStableAcrossUpdates` | Yes: drives `buildFwdBody` twice and compares the identifier field of each destination frame |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `buildFwdBody`'s same-context branch is a second writer; both read one table |
| A-2 | broken | the bound holds for identifiers in use and not for identifiers a peer stopped using; Review Gate BLOCKER 1 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 9. RFC compliance | `rfc/short/rfc7911.md` carries `RFC7911-2-2` with no `{gap}`; `rfc/requirements/rfc7911.md` shows both polarities and the interop tag; `docs/features/rfc-status.md` records "Closed 2026-08-14: RFC7911-2-2" | Yes |
| 9. RFC compliance, feature pages | `docs/features.md` and `docs/features/bgp-protocol.md` describe the ingress key, the legality of 0 and the live-value set, each anchored on `forward_path_id.go`; each claim matches `fwdPathIDTable` | Yes |
| 12. Internal architecture | `docs/architecture/bgp/structural-forwarding.md` gained nothing, and one of its claims is now imprecise | No: owed with the BLOCKER 1 fix |
| 16. Source anchors | `grep -rn "forward_body.go\|forward_path_id.go" docs/` names six pages; the five that make a Path Identifier claim are accurate, the sixth is the architecture page above | Partial |
