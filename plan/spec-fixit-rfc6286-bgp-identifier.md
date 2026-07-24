# Spec: fixit-rfc6286-bgp-identifier

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-07-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/full/rfc6286.txt` Sections 2.1-2.3 (the whole normative text is 60 lines)
4. `internal/component/bgp/reactor/{routerid_unique.go,peer.go,session_handlers.go,session_connection.go,session.go}`

## Task

Close the three RFC 6286 limitations recorded in `plan/learned/1270-fixit-load-dependent-functional-failures.md`
("Known limitations (recorded, not fixed here)"), which the owner asked to fix:

1. **Section 2.2 OPEN error handling is unimplemented.** An OPEN whose BGP Identifier is zero, or
   whose BGP Identifier equals this speaker's own identifier when the peer is internal, is accepted.
   The RFC requires the OPEN Message Error / Bad BGP Identifier subcode.
2. **The default-path check-then-act race.** `checkRouterIDConflict` only sees peers that already
   reached Established, so two same-AS peers presenting the same BGP Identifier concurrently are
   BOTH accepted (the load-dependent 345 failure); which one is rejected -- if either -- depends on
   scheduling.
3. **No `rfc/short/rfc6286.md` summary and no `docs/features/rfc-status.md` row**, so ze's partial
   RFC 6286 support is undisclosed.

While tracing (2) it became clear that Section 2.3 (connection collision with identical BGP
Identifiers) is a two-line delta on an existing code path and is the RFC's own answer for the shared
identifier that `allow-shared-router-id` permits, so it is in scope rather than left as the only
open corner of a small RFC.

Goals:
- G-1: ze rejects an RFC-invalid BGP Identifier in a received OPEN, on every OPEN rail.
- G-2: exactly one peer wins a same-AS identifier, decided atomically at OPEN validation.
- G-3: RFC 6286 support is disclosed with a summary and a status-ledger row.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` - reactor/session/peer split, where OPEN validation sits
  → Constraint: the reactor owns cross-peer state; a Session validates only its own connection.
  → Decision: cross-peer identifier state therefore lives on the Reactor, reached from the peer's
    `validateOpen` callback, not inside Session.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/full/rfc6286.txt` - the RFC itself (no summary existed; writing one is deliverable G-3)
  → Constraint: Section 2.1 -- "The BGP Identifier is a 4-octet, unsigned, non-zero integer that
    should be unique within an AS." Uniqueness is lowercase should, so `allow-shared-router-id` is
    conformant; non-zero is part of the definition.
  → Constraint: Section 2.2 -- "If the BGP Identifier field of the OPEN message is zero, or if it is
    the same as the BGP Identifier of the local BGP speaker and the message is from an internal
    peer, then the Error Subcode is set to 'Bad BGP Identifier'." The self-identifier half is gated
    on the peer being INTERNAL; an external peer legitimately may share it.
  → Constraint: Section 2.3 -- "If the BGP Identifiers of the peers involved in the connection
    collision are identical, then the connection initiated by the BGP speaker with the larger AS
    number is preserved." Applicable to external peers only.
- [ ] `rfc/short/rfc4271.md` - Section 6.2 Bad BGP Identifier is the subcode RFC 6286 re-defines
  → Constraint: RFC 4271 Section 6.8 collision resolution compares identifiers; RFC 6286 Section 2.3
    extends it for the equal case, so the extension belongs in `DetectCollision`, not beside it.

**Key insights:**
- RFC 6286 is 3 normative statements; ze implemented none of them, while already carrying a
  stricter-than-RFC AS-wide uniqueness check that RFC 6286 Section 2.1 only makes a SHOULD.
- Section 2.2 is about ze's OWN identity and about zero; `allow-shared-router-id` is about two
  PEERS sharing one identifier. They are independent, so the opt-in does not relax Section 2.2.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/session_handlers.go:41` - `handleOpen`: validates Version,
  then `ValidateHoldTime`, stores `peerOpen`, then calls `openValidator` (plugins + router-id).
  No BGP Identifier check of any kind.
- [ ] `internal/component/bgp/reactor/session_connection.go:131` - `processOpen`, the second
  OPEN rail (collision winner replay via `AcceptWithOpen`). Duplicates the Version and hold-time
  checks and **never calls `openValidator` at all**, so RFC 9234 role validation and the router-id
  check are both skipped on that rail today.
- [ ] `internal/component/bgp/reactor/peer.go:652` - `validateOpen`: stores `remoteRouterID`,
  and unless `AllowSharedRouterID` calls `checkRouterIDConflict` under `r.mu.RLock`.
- [ ] `internal/component/bgp/reactor/routerid_unique.go:55` - `checkRouterIDConflict` scans
  `r.peers` for a same-AS peer whose session is `fsm.StateEstablished` and whose stored `peerOpen`
  carries the same identifier. This is the check-then-act: nothing is recorded at validation time,
  so a peer that has not yet established is invisible to the peer validating concurrently.
- [ ] `internal/component/bgp/reactor/session.go:626` - `DetectCollision`: RFC 4271 Section 6.8,
  `localID < remoteBGPID` accepts the pending connection. The equal case falls through to "keep
  existing", with no AS comparison (RFC 6286 Section 2.3).
- [ ] `internal/component/bgp/reactor/peer_run.go:227` - every session teardown runs
  `clearEncodingContexts()` (runOnce defer) and `cleanup()`; these are the release points a claim
  registry needs.
- [ ] `internal/component/bgp/message/open.go:237` - `ValidateHoldTime` returns a
  `*message.Notification`; the house pattern for a per-field OPEN validator.
- [ ] `internal/test/peer/expect.go:153` + `internal/test/peer/peer.go:667` - the test peer
  mirrors ze's OPEN and sets its BGP Identifier to ze's last octet + 1; `option=open:value=...`
  inside a `stdin=<name>` block reaches the peer verbatim, so a new `router-id` sub-option is the
  natural way to drive an invalid identifier from a `.ci` test.

**Behavior to preserve:**
- `AllowSharedRouterID` stays fail-closed: the zero value enforces AS-wide uniqueness.
- A conflict is still reported as `routerIDConflictError` -> OPEN Message Error / Bad BGP Identifier,
  carrying the identifier, the ASN, and the conflicting peer's address in its message.
- Peers in different ASes may reuse an identifier; the same peer re-opening is not a conflict.
- `test/plugin/redistribute-as112-announce.ci` (one speaker, v4+v6, shared identifier, opted in)
  keeps passing unchanged.

**Behavior to change:** (user requested)
- A zero or self (internal-peer) identifier is rejected with 2/3 instead of accepted.
- A same-AS duplicate identifier is decided at OPEN validation, not at Established: a peer in
  OpenSent/OpenConfirm now holds its identifier against a later peer. This is the point of the fix,
  and it inverts `TestRouterIDConflictNotEstablished`'s old assertion.
- The collision-winner rail (`processOpen`) now runs the OPEN validator, so plugin validation and
  the identifier claim apply there too.
- `DetectCollision` gains the RFC 6286 Section 2.3 equal-identifier branch.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Wire bytes: a BGP OPEN message read from a peer's TCP connection.
- Format at entry: `*message.Open` (`UnpackOpen`), fields Version / MyAS / HoldTime / BGPIdentifier.

### Transformation Path
1. `session_read.go:304` dispatches an OPEN body to `handleOpen`; the collision-winner replay enters
   at `session_connection.go:126` `processOpen` with an already-parsed OPEN.
2. Both rails call `Session.validateOpenIdentifier(open)` (new, `session_validation.go`), which asks
   `message.Open.ValidateBGPIdentifier(localID, internal)` (new) and, on failure, sends
   NOTIFICATION OPEN Message Error / Bad BGP Identifier, logs the FSM error event, and closes.
3. Both rails then call `Session.runOpenValidator(open)` (new, extracted from `handleOpen`), which
   invokes the peer's `validateOpen` callback.
4. `Peer.validateOpen` stores `remoteRouterID` and, unless `AllowSharedRouterID`, claims
   `(peerAS, bgpID)` in the reactor's `routerIDClaims` registry. A claim held by a different peer
   returns `routerIDConflictError`, which the rail turns into the same 2/3 NOTIFICATION.
5. `Peer.releaseRouterIDClaim()` runs on every session teardown (runOnce defer, cleanup), so the
   identifier is available to the next peer.
6. On a connection collision, `DetectCollision` compares identifiers (RFC 4271 Section 6.8) and, when
   they are equal, the AS numbers (RFC 6286 Section 2.3).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ Session | `UnpackOpen` then per-field validators returning `*message.Notification` | [ ] |
| Session ↔ Peer | existing `openValidator` callback (`SetOpenValidator`), now on both rails | [ ] |
| Peer ↔ Reactor | `r.routerIDs` claim registry, own leaf mutex (never takes `r.mu` or `p.mu`) | [ ] |

### Integration Points
- `message.Open.ValidateHoldTime` - sibling validator; same signature shape and NOTIFICATION return.
- `Session.openValidator` - existing callback, now reached from `processOpen` as well.
- `Reactor.routerIDs` - new field; `Config.AllowSharedRouterID` still gates whether it is consulted.

### Architectural Verification
- [ ] No bypassed layers (both OPEN rails share one validator pair)
- [ ] No unintended coupling (registry is reactor-local; Session stays per-connection)
- [ ] No duplicated functionality (`checkRouterIDConflict` is deleted, not left beside the registry)
- [ ] Zero-copy preserved where applicable (validation reads parsed scalars only)
- [ ] Registration over hardcoding - n/a (no new command, family, or handler)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No existing test peer sends a zero identifier or ze's own identifier on an iBGP session | `internal/test/peer/peer.go:672` mirrors ze's id + 1; `internal/test/peer/inject.go:420` derives it from the dial host | new rejects break unrelated suites | full functional suite run | broken (see Assumptions Resolved) |
| A-2 | Every session that reaches `validateOpen` later runs the runOnce defer, so a claim cannot leak | `internal/component/bgp/reactor/peer_run.go:227` (defer set before `session.Start()`), `:517-521` cleanup | a leaked claim rejects a legitimate later peer forever | `TestRouterIDClaimReleasedOnTeardown` + reconnect `.ci` | confirmed |
| A-3 | `p.settings.PeerAS` is the right AS for the claim key and for "internal peer" | `session_validation.go:103` uses the same `LocalAS == PeerAS` test for iBGP | a dynamic peer with an unresolved AS keys the claim under AS 0 | read `reactor_dynamic.go` resolution order | broken (see Assumptions Resolved) |
| A-4 | `option=open:value=...` inside a `stdin=<name>` block reaches ze-peer verbatim (sub-keys intact) | `internal/test/runner/record_parse.go:353` reconstructs only top-level options; block bodies are passed through | the `.ci` cannot drive an invalid identifier | run the new `.ci` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Claiming at OPEN rejects a reconnecting peer whose old session has not yet torn down | reconnect flaps in functional suites | claim is keyed per peer, so the SAME peer re-opening refreshes it; only a DIFFERENT peer is rejected, exactly as when the old peer was Established |
| R-2 | Running the OPEN validator on the collision rail newly rejects a connection that used to be accepted | role/OTC suites go red | that rail was skipping RFC 9234 validation, which is a defect, not a feature; suites prove it |
| R-3 | Section 2.2 rejects a peer an operator deliberately configured with ze's own router-id over iBGP | operator report | it is exactly what the RFC mandates; disclosed in `docs/guide/configuration.md` next to `allow-shared-router-id` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer OPEN with BGP Identifier 0 on the wire | → | `Session.validateOpenIdentifier` -> NOTIFICATION 2/3 | `test/plugin/rfc6286-bad-bgp-identifier.ci` |
| Peer OPEN with ze's own identifier over iBGP | → | `message.Open.ValidateBGPIdentifier` internal branch | `test/plugin/rfc6286-bad-bgp-identifier.ci` |
| Two same-AS peers, same identifier, concurrent OPEN | → | `routerIDClaims.claim` | `TestRouterIDClaimConcurrentOnlyOneWins` |
| Session teardown | → | `Peer.releaseRouterIDClaim` | `TestRouterIDClaimReleasedOnTeardown` |
| Collision with equal identifiers | → | `Session.DetectCollision` Section 2.3 branch | `TestDetectCollisionEqualIdentifierPrefersLargerAS` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Received OPEN with BGP Identifier 0 | NOTIFICATION OPEN Message Error (2) / Bad BGP Identifier (3) sent, connection closed, no session |
| AC-2 | Received OPEN whose identifier equals the local router-id, peer AS == local AS | same 2/3 rejection |
| AC-3 | Received OPEN whose identifier equals the local router-id, peer AS != local AS | accepted (RFC 6286 Section 2.2 gates the self case on an internal peer) |
| AC-4 | Two peers in one AS present the same identifier, neither Established | exactly one is accepted; the other gets 2/3, regardless of scheduling |
| AC-5 | The peer holding an identifier tears its session down | the identifier is released; a different peer may then claim it |
| AC-6 | `allow-shared-router-id true` | no AS-wide uniqueness check at all (unchanged), but AC-1/AC-2 still reject |
| AC-7 | Connection collision, identifiers identical, remote AS > local AS | the remote-initiated (pending) connection is preserved |
| AC-8 | Connection collision, identifiers identical, remote AS < local AS | the local-initiated (existing) connection is preserved |
| AC-9 | `rfc/short/rfc6286.md` + `docs/features/rfc-status.md` | summary exists with a compliance checklist; ledger row states the support level with source anchors |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers a misconfigured router that advertises BGP Identifier 0.0.0.0 | wire -> handleOpen -> validateOpenIdentifier -> NOTIFICATION 2/3 | `test/plugin/rfc6286-bad-bgp-identifier.ci` |
| 2 | Accidentally gives an iBGP neighbour ze's own router-id | wire -> handleOpen -> validateOpenIdentifier (internal branch) -> 2/3 | `test/plugin/rfc6286-bad-bgp-identifier.ci` |
| 3 | Boots two same-AS routers with a duplicated router-id at the same instant | wire -> validateOpen -> routerIDClaims.claim -> one 2/3 | `TestRouterIDClaimConcurrentOnlyOneWins` |
| 4 | Runs one anycast speaker over v4+v6 with `allow-shared-router-id true` | validateOpen skips the claim entirely | `test/plugin/redistribute-as112-announce.ci` (existing, unchanged) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateBGPIdentifier` | `internal/component/bgp/message/open_test.go` | zero rejected; self+internal rejected; self+external accepted; normal accepted; subcode is 2/3 | |
| `TestHandleOpenRejectsBadBGPIdentifier` | `internal/component/bgp/reactor/session_handlers_test.go` | the NOTIFICATION reaches the wire and the session does not advance | |
| `TestRouterIDClaimConcurrentOnlyOneWins` | `internal/component/bgp/reactor/routerid_unique_test.go` | N goroutines claim one key; exactly one succeeds | |
| `TestRouterIDClaimReleasedOnTeardown` | `internal/component/bgp/reactor/routerid_unique_test.go` | release frees the key for another peer | |
| `TestValidateOpenAllowSharedRouterID` | `internal/component/bgp/reactor/routerid_unique_test.go` | existing opt-in/opt-out gate, now over the claim registry | |
| `TestRouterIDConflict*` (7 existing) | `internal/component/bgp/reactor/routerid_unique_test.go` | same scenarios re-pointed at the registry | |
| `TestDetectCollisionEqualIdentifierPrefersLargerAS` | `internal/component/bgp/reactor/session_test.go` | RFC 6286 Section 2.3 both directions | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP Identifier | 1..0xFFFFFFFF (non-zero, RFC 6286 Section 2.1) | 0xFFFFFFFF | 0 | N/A (field is 4 octets) |
| BGP Identifier vs local | any except local-when-internal | local-1, local+1 | local (internal) | local (internal) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc6286-bad-bgp-identifier` | `test/plugin/rfc6286-bad-bgp-identifier.ci` | a peer offering identifier 0.0.0.0 gets NOTIFICATION 2/3 on the wire | |
| `redistribute-as112-announce` | `test/plugin/redistribute-as112-announce.ci` | the opt-in shared-identifier case still establishes (regression) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| existing BGP scenarios | `test/interop/scenarios/` | FRR / BIRD / GoBGP | conformant identifiers still establish; the new rejects are for RFC-invalid input no conformant daemon sends, so no new scenario is warranted | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/bgp/message/open.go` - `ValidateBGPIdentifier` (RFC 6286 Section 2.2)
- `internal/component/bgp/reactor/session_validation.go` - `validateOpenIdentifier`, `runOpenValidator`
- `internal/component/bgp/reactor/session_handlers.go` - use both helpers
- `internal/component/bgp/reactor/session_connection.go` - use both helpers on the collision rail
- `internal/component/bgp/reactor/session.go` - `DetectCollision` Section 2.3 branch
- `internal/component/bgp/reactor/routerid_unique.go` - claim registry replaces the established scan
- `internal/component/bgp/reactor/reactor.go` - `routerIDs` field
- `internal/component/bgp/reactor/peer.go` - claim in `validateOpen`, `releaseRouterIDClaim`
- `internal/component/bgp/reactor/peer_run.go` - release on teardown
- `internal/test/peer/{expect.go,peer.go,config.go}` - `option=open:value=router-id:id=<a.b.c.d>`
- `docs/guide/configuration.md` - Section 2.2 note beside `allow-shared-router-id`
- `docs/features/rfc-status.md` - RFC 6286 row
- `docs/architecture/testing/ci-format.md` - the new peer option

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | no | no new config leaf; `allow-shared-router-id` already exists |
| CLI commands/flags | no | no operator-facing command changes |
| Functional test for new RPC/API | yes | `test/plugin/rfc6286-bad-bgp-identifier.ci` |
| Env var registration | no | none added |
| Doctor check for runtime dependencies | no | no new external dependency |
| Prometheus counters/metrics | no | rejection is visible as a NOTIFICATION and a log line |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | no | rejection of invalid input, not a feature entry |
| 2 | Config syntax changed? | yes | `docs/guide/configuration.md` -- what `allow-shared-router-id` does NOT relax |
| 3 | CLI command added/changed? | no | - |
| 4 | API/RPC added/changed? | no | - |
| 5 | Plugin added/changed? | no | - |
| 6 | Has a user guide page? | no | covered by configuration.md |
| 7 | Wire format changed? | no | no new encoding; an existing subcode is now emitted |
| 8 | Plugin SDK/protocol changed? | no | - |
| 9 | RFC behavior implemented, changed, or newly proven? | yes | `rfc/short/rfc6286.md` (new), `docs/features/rfc-status.md` (new row) |
| 10 | Test infrastructure changed? | yes | `docs/architecture/testing/ci-format.md` -- `option=open:value=router-id` |
| 11 | Affects daemon comparison? | no | no comparison row claims RFC 6286 |
| 12 | Internal architecture changed? | no | claim registry is internal to the reactor |
| 13 | Route metadata keys added/changed? | no | - |
| 14 | Prometheus counters added/changed? | no | - |
| 15 | Registered plugin/event/command inventory changed? | no | - |
| 16 | Changed source referenced by doc source anchors? | check | grep `docs/` for the changed files during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | check | `docs/guide/configuration.md` router-id section |

## Files to Create
- `rfc/short/rfc6286.md` - the RFC summary with a compliance checklist
- `test/plugin/rfc6286-bad-bgp-identifier.ci` - functional proof of the wire rejection

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify / Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify` |
| 6-9. Critical review | Critical Review Checklist |
| 10-12. Deliverables / security / docs | Checklists below |
| 13. `/ze-review` gate | Review Gate |
| 14. Summary + close | Executive Summary, two-commit closure |

### Implementation Phases

1. **Phase: Wiring** -- `validateOpenIdentifier` + `runOpenValidator` reachable from BOTH OPEN rails
   - Tests: `TestHandleOpenRejectsBadBGPIdentifier`
   - Files: `session_validation.go`, `session_handlers.go`, `session_connection.go`
   - Verify: test fails while the validator is a stub, passes once Section 2.2 lands
2. **Phase: Section 2.2 validator** -- `message.Open.ValidateBGPIdentifier`
   - Tests: `TestValidateBGPIdentifier`
   - Files: `internal/component/bgp/message/open.go`
3. **Phase: claim registry** -- replace the established scan; release on teardown
   - Tests: `TestRouterIDClaim*`, rewritten `TestRouterIDConflict*`
   - Files: `routerid_unique.go`, `reactor.go`, `peer.go`, `peer_run.go`
4. **Phase: Section 2.3 collision** -- equal identifiers compare AS numbers
   - Tests: `TestDetectCollisionEqualIdentifierPrefersLargerAS`
   - Files: `session.go`
5. **Phase: functional + docs** -- `.ci` test, peer `router-id` option, RFC summary, status row
   - Tests: `test/plugin/rfc6286-bad-bgp-identifier.ci`
   - Files: `internal/test/peer/*`, `rfc/short/rfc6286.md`, `docs/*`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The self-identifier branch is gated on INTERNAL; the zero branch is not |
| Lock order | The claim registry mutex is a leaf: it never takes `r.mu` or `p.mu` while held |
| Claim leaks | Every path that reaches `validateOpen` also reaches a release |
| Sibling call sites | Both OPEN rails validate; no third rail parses an OPEN unvalidated |
| Rule: no-layering | `checkRouterIDConflict` is deleted, not left beside the registry |
| Rule: fail-closed | `AllowSharedRouterID` zero value still enforces; a claim failure denies |
| Test sensitivity | Mutation-verify: disabling the validator turns the `.ci` red |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Section 2.2 enforced on both rails | `grep -n validateOpenIdentifier internal/component/bgp/reactor/session_*.go` |
| Claim registry replaces the scan | `grep -rn checkRouterIDConflict internal/` returns nothing |
| Functional proof | `./bin/ze-test bgp plugin -p rfc6286-bad-bgp-identifier` |
| RFC summary + ledger row | `ls rfc/short/rfc6286.md`; `grep "RFC 6286" docs/features/rfc-status.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | The identifier is a parsed 4-octet scalar; no allocation, no unbounded state |
| Resource exhaustion | The claim registry grows at most one entry per (AS, identifier) with a live peer, and every entry is released on teardown |
| Denial of service | A hostile peer cannot claim another peer's identifier: the claim is only made after the OPEN passes the earlier rails, and it is released when its session dies |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Functional suite regression from A-1 | Re-check which peer sends the offending identifier before touching the validator |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-3: the peer's configured AS is available at OPEN validation | A dynamic peer's PeerAS is published at establishment, after validateOpen | Read `resolveDynamicPeerSettings` call site (peer_run.go:353) while writing the claim key | Added `claimPeerAS`, which falls back to the OPEN's advertised AS |
| A-1: no existing fixture shares a BGP Identifier between two peers of one AS | Three did (223, 254, 351); they passed only because detection waited for Established | Full plugin suite run after the change | Fixtures given distinct identifiers with the new `.ci` option |
| The new ze-peer option would take effect once parsed | `cmd_peer.go` merges fileConfig field by field and silently dropped it | Manual two-process reproduction showed the peer still sending the mirrored identifier | Added the merge line; the option now has its own unit test |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Extend `checkRouterIDConflict` to also match non-Established peers via `p.remoteRouterID` | Symmetric detection: both concurrent peers see each other and BOTH reject, which is worse than the double-accept it replaces | An atomic first-writer-wins claim registry |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The check-then-act was not fixable by widening the predicate; a shared-state test needs an atomic
  claim, otherwise two symmetric evaluators reach the same verdict about each other.
- `processOpen` silently skipping `openValidator` means any per-peer OPEN policy (role, router-id)
  has always been bypassable by winning a connection collision.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Atomic claim at OPEN validation | keep the Established scan; add a tie-break | Only a first-writer-wins registry makes the outcome independent of scheduling |
| Registry has its OWN leaf mutex | reuse `r.mu` | `RemovePeer` holds reactor state while stopping a peer; a release taking `r.mu` would risk a self-deadlock |
| Owner identity is the `*Peer` pointer | the `netip.AddrPort` peer key | A dynamic peer's settings can be replaced; the pointer is stable for the peer's whole life, so a release can never miss |
| Section 2.2 is NOT relaxed by `allow-shared-router-id` | gate it too | The opt-in is about two PEERS sharing an identifier; Section 2.2 is about ze's own identity and about zero |
| Implement Section 2.3 here | leave it as the last open corner | Two lines on an existing path, and it is the RFC's answer for the shared identifier the opt-in permits |

## Known Limitations
- ze still does not detect a collision in OpenSent (RFC 4271 Section 6.8 MAY), so Section 2.3 applies
  only where RFC 4271 collision detection already applies (OpenConfirm).
- The independent review required by `ai/rules/critical-review.md` has NOT been run: this session was
  instructed not to spawn agents. The spec cannot be closed until it is.

## RFC Documentation

`// RFC 6286 Section 2.2: "If the BGP Identifier field of the OPEN message is zero, or if it is the
same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then
the Error Subcode is set to 'Bad BGP Identifier'."` above the validator; Section 2.1 above the claim
registry; Section 2.3 above the collision branch.

## Implementation Summary

### What Was Implemented
- `message.Open.ValidateBGPIdentifier(localID, internal)` (internal/component/bgp/message/open.go):
  RFC 6286 Section 2.2, returning a `*Notification` carrying OPEN Message Error / Bad BGP
  Identifier, with an empty Data field (RFC 4271 Section 6.2 defines none for that subcode).
- `Session.validateOpenIdentifier` + `Session.runOpenValidator`
  (internal/component/bgp/reactor/session_open_validation.go), called from BOTH OPEN rails:
  `handleOpen` (session_handlers.go) and `processOpen` (session_connection.go). The second rail
  previously ran no OPEN validator at all, so plugin validation (RFC 9234 role) and the identifier
  check were skippable by winning connection collision resolution.
- `routerIDClaims` (internal/component/bgp/reactor/routerid_unique.go): the AS-wide identifier
  registry, claimed during OPEN validation and released on session teardown, replacing
  `checkRouterIDConflict`'s established-peers scan. Its mutex is a leaf; ownership is keyed on the
  `*Peer` pointer so a dynamic peer's settings replacement cannot leak a claim.
- `Peer.claimPeerAS` / `Peer.releaseRouterIDClaim` (peer.go) + release at both teardown sites
  (peer_run.go runOnce defer and cleanup).
- RFC 6286 Section 2.3 in `Session.DetectCollision` (session.go): identical identifiers preserve the
  connection initiated by the larger AS number.
- `parseRouterID` (config.go): one helper for the global and per-peer `router-id` leaves, rejecting
  0.0.0.0 per Section 2.1; both leaves carry `ze:validate "nonzero-ipv4"` in the YANG.
- Test infrastructure: `option=open:value=router-id:id=<a.b.c.d>` for ze-peer
  (internal/test/peer/{expect.go,peer.go}, internal/test/cli/cmd_peer.go).
- `rfc/short/rfc6286.md` + enrolment in `rfc/enrolled.txt`; `docs/features/rfc-status.md` row;
  `docs/guide/configuration.md` and `docs/architecture/testing/ci-format.md` updates.

### Bugs Found/Fixed
- **`processOpen` never ran the OPEN validator.** Found while auditing the sibling call sites of the
  new check. Fixed by routing both rails through `runOpenValidator`; covered by
  `TestProcessOpenRunsOpenValidator`.
- **Three multi-peer fixtures relied on the undetected duplicate.** 223, 254 and 351 run two
  ze-peers in ONE AS, and ze-peer's default OPEN mirrors ze's router-id + 1, so both presented the
  same BGP Identifier. The old check-then-act only caught it when the first peer had already
  reached Established, which under these tests it had not. Each second peer now gets an explicit
  distinct identifier via the new option. This is the race the spec exists to fix, observed.
- **The new ze-peer option was inert until wired.** `Config.RouterID` parsed from the expect file
  was dropped by the field-by-field merge in `internal/test/cli/cmd_peer.go`; the `.ci` test passed
  the option and the peer still sent the default. Fixed, and `TestLoadExpectFileRouterIDOverride`
  plus the two `.ci` tests now bind it end to end.

### Documentation Updates
- `docs/features/rfc-status.md`: new RFC 6286 row (Supported) with per-section source anchors.
- `docs/guide/configuration.md`: what `allow-shared-router-id` does and does NOT relax, plus the
  Section 2.2/2.3 behaviour, with source anchors.
- `docs/architecture/testing/ci-format.md`: the `router-id` OPEN option.
- `rfc/short/rfc6286.md` (new) and `ai/RFC-REQUIREMENTS.md` (regenerated by `make ze-rfc-index`).

### Deviations from Plan
- Section 2.1's non-zero property is enforced for ze's OWN identifier as well (`parseRouterID`),
  which the plan had left out. Owner asked for the YANG check to stay, so the enforcement was put
  where the BGP config path actually runs: `ze config validate` and daemon start both reject
  `router-id 0.0.0.0` now. The YANG `ze:validate` annotation documents the same rule for the
  surfaces that run custom validators.
- Two `.ci` files were created instead of one (`rfc6286-zero-bgp-identifier.ci`,
  `rfc6286-self-bgp-identifier.ci`): one daemon config cannot express both scenarios.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Section 2.2 reception check | Done | internal/component/bgp/message/open.go `ValidateBGPIdentifier`; internal/component/bgp/reactor/session_open_validation.go `validateOpenIdentifier` | both OPEN rails |
| 2. check-then-act race | Done | internal/component/bgp/reactor/routerid_unique.go `routerIDClaims` | claim at OPEN validation, release at teardown |
| 3. summary + status row | Done | rfc/short/rfc6286.md, docs/features/rfc-status.md | enrolled in rfc/enrolled.txt |
| (in scope, added) Section 2.3 collision | Done | internal/component/bgp/reactor/session.go `DetectCollision` | |
| (owner request) non-zero local router-id | Done | internal/component/bgp/reactor/config.go `parseRouterID` + ze-bgp-conf.yang | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 zero identifier rejected | Done | `test/plugin/rfc6286-zero-bgp-identifier.ci`, `TestHandleOpenRejectsBadBGPIdentifier`, `TestOpenValidateBGPIdentifier` | NOTIFICATION 2/3 asserted on the wire |
| AC-2 self identifier, internal peer | Done | `test/plugin/rfc6286-self-bgp-identifier.ci`, `TestHandleOpenRejectsBadBGPIdentifier` | |
| AC-3 self identifier, external peer accepted | Done | `TestHandleOpenRejectsBadBGPIdentifier` ("own identifier from external peer"), `TestOpenValidateBGPIdentifier` | |
| AC-4 concurrent duplicate: exactly one wins | Done | `TestRouterIDClaimConcurrentOnlyOneWins` (16 goroutines) | |
| AC-5 claim released on teardown | Done | `TestRouterIDClaimReleasedOnTeardown`, `TestRouterIDClaimKeyedByPeerNotSettings` | |
| AC-6 opt-in skips only the AS-wide check | Done | `TestValidateOpenAllowSharedRouterID`, `test/plugin/redistribute-as112-announce.ci` (unchanged, green) | Section 2.2 still enforced |
| AC-7/AC-8 collision tie-break both ways | Done | `TestDetectCollisionEqualIdentifierPrefersLargerAS` | plus the updated `TestCollisionBGPIDComparison` row |
| AC-9 summary + ledger row | Done | rfc/short/rfc6286.md, docs/features/rfc-status.md, ai/RFC-REQUIREMENTS.md | `python3 scripts/dev/rfc_requirements.py --check` reports no RFC6286 violation |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestOpenValidateBGPIdentifier` | Done | internal/component/bgp/message/open_test.go | 8 cases incl. boundaries 1 and 0xFFFFFFFF |
| `TestHandleOpenRejectsBadBGPIdentifier` | Done | internal/component/bgp/reactor/session_open_validation_test.go | asserts wire NOTIFICATION + FSM state |
| `TestProcessOpenRejectsBadBGPIdentifier` / `TestProcessOpenRunsOpenValidator` | Done | same file | second rail |
| `TestRouterIDClaim*` (3) | Done | internal/component/bgp/reactor/routerid_unique_test.go | concurrency, release, settings-replacement |
| `TestRouterIDConflict*` (7, re-pointed) | Done | same file | `NotEstablished` inverted by design |
| `TestDetectCollisionEqualIdentifierPrefersLargerAS` | Done | internal/component/bgp/reactor/collision_test.go | 4 cases |
| `TestLoadExpectFileRouterIDOverride` | Done | internal/test/peer/expect_test.go | 6 cases |
| `rfc6286-zero-bgp-identifier` / `rfc6286-self-bgp-identifier` | Done | test/plugin/ | mutation-verified (both go red when the validator is stubbed) |
| `rfc6286-zero-router-id-rejected` | Done | test/parse/ | config rejection |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All "Files to Modify" / "Files to Create" rows | Done | plus internal/test/cli/cmd_peer.go (merge wiring) and internal/component/bgp/reactor/config.go (parseRouterID), neither in the original plan |

### Audit Summary
- **Total items:** 9 ACs + 5 task requirements
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** Section 2.1 local-identifier enforcement added at owner request (see Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| G-1 invalid identifier rejected on every rail | functional test + mutation check | `ze-test bgp plugin 378 379` -> PASS; with `validateOpenIdentifier` stubbed to return nil both TIME OUT waiting for the NOTIFICATION (tmp/t-mut1.log), so the tests gate the producer |
| G-2 exactly one peer wins an identifier | unit test (concurrent) | `TestRouterIDClaimConcurrentOnlyOneWins`: 16 goroutines, `accepted == 1`, winner holds the registry entry; `go test -race` clean |
| G-3 RFC 6286 disclosed | doc artifact + gate | `rfc/short/rfc6286.md` enrolled in `rfc/enrolled.txt`; `python3 scripts/dev/rfc_requirements.py --check` reports no RFC6286 violation; `ai/RFC-REQUIREMENTS.md` binds all five requirements |
| (bonus) the race is real, not theoretical | observed | three multi-peer fixtures (223/254/351) that shared one identifier were accepted before and are now deterministically rejected -- the exact double-accept the fix removes |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `rfc/short/rfc6286.md` | yes | `-rw-r--r-- 7.5K Jul 24 23:19 rfc/short/rfc6286.md` |
| `internal/component/bgp/reactor/session_open_validation.go` | yes | `-rw-r--r-- 3.7K Jul 24 22:56 .../session_open_validation.go` |
| `test/plugin/rfc6286-zero-bgp-identifier.ci` | yes | `-rw-r--r-- 1.4K Jul 24 22:41 .../rfc6286-zero-bgp-identifier.ci` |
| `test/plugin/rfc6286-self-bgp-identifier.ci` | yes | `-rw-r--r-- 1.7K Jul 24 22:41 .../rfc6286-self-bgp-identifier.ci` |
| `test/parse/rfc6286-zero-router-id-rejected.ci` | yes | `-rw-r--r-- 1019 Jul 24 23:18 .../rfc6286-zero-router-id-rejected.ci` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | zero identifier -> 2/3, no session | `ze-test bgp plugin 379` PASS; peer wire log shows the OPEN carrying 00000000 then `msg recv ...0015:03:0203` (tmp/rfc6286/peer3.out) |
| AC-2 | self identifier from internal peer -> 2/3 | `ze-test bgp plugin 378` PASS (iBGP 65000/65000, peer presents ze's 1.2.3.4) |
| AC-3 | self identifier from external peer accepted | `TestHandleOpenRejectsBadBGPIdentifier/own_identifier_from_external_peer`: FSM reaches OpenConfirm, no NOTIFICATION |
| AC-4 | concurrent duplicate: exactly one wins | `go test -race -run TestRouterIDClaimConcurrentOnlyOneWins` PASS (accepted == 1 over 16 goroutines) |
| AC-5 | claim released on teardown | `TestRouterIDClaimReleasedOnTeardown` PASS: release -> holder() false -> a different peer claims |
| AC-6 | opt-in only skips the AS-wide check | `TestValidateOpenAllowSharedRouterID` both subtests PASS; full plugin suite keeps `redistribute-as112-announce` green |
| AC-7/8 | collision tie-break both directions | `TestDetectCollisionEqualIdentifierPrefersLargerAS` 4/4 subtests PASS |
| AC-9 | summary + ledger row | `python3 scripts/dev/rfc_requirements.py --check`: no RFC6286 violation; `ai/RFC-REQUIREMENTS.md` binds all five requirement ids |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Peer OPEN with identifier 0 | `test/plugin/rfc6286-zero-bgp-identifier.ci` | yes -- drives the identifier with the router-id option, asserts NOTIFICATION hex `...0015030203`; goes RED when `validateOpenIdentifier` is stubbed |
| Peer OPEN with ze's own identifier, iBGP | `test/plugin/rfc6286-self-bgp-identifier.ci` | yes -- same shape with id 1.2.3.4 against `asn { local 65000; remote 65000 }`; RED under the same stub |
| Config with a zero router-id | `test/parse/rfc6286-zero-router-id-rejected.ci` | yes -- `ze config validate -` exits 1 and prints the Section 2.1 message |
| Concurrent duplicate identifiers | unit (`TestRouterIDClaimConcurrentOnlyOneWins`) | yes -- no `.ci` can express a simultaneous OPEN race; the unit test drives `validateOpen` from 16 goroutines |
| Collision with equal identifiers | unit (`TestDetectCollisionEqualIdentifierPrefersLargerAS`) | yes -- collision resolution has no `.ci` harness; the existing collision suite is unit-level |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/rfc-status.md` RFC 6286 row | anchors name `session_open_validation.go`, `routerid_unique.go`, `session.go DetectCollision`, `config.go parseRouterID`; each read while writing the row | yes, doc-test PASSED |
| `docs/guide/configuration.md` opt-out semantics | "does not relax Section 2.2" checked against `Peer.validateOpen` (claim gated on `AllowSharedRouterID`) and `Session.validateOpenIdentifier` (ungated) | yes |
| `docs/architecture/testing/ci-format.md` router-id option | checked against `internal/test/peer/expect.go` parse, `peer.go generateOpen`, `cmd_peer.go` merge | yes |
| `ai/CODE-TO-DOCS.md` / `ai/DOCS-TO-CODE.md` | regenerated after the new anchors | yes, ze-doc-check-stale PASSED |
| Whole gate | `python3 scripts/dev/verify_wiring_docs.py` exit 0 (wiring, doc-test, stale, discovery, digest, inventory, command-list, plugin-imports, spec-citation all PASSED) | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-fixit-rfc6286-bgp-identifier.md`
