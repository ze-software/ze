# Spec: fixit-ipsec-peer-reload-ignored

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**An operator can edit a live IPsec peer, commit, see success, and reach no
wire.** The tunnel keeps the configuration it was born with until the daemon
restarts.

Two producers were read on 2026-08-22 and both hold today.

`peerConfigChanged` (`internal/component/ike/engine/reconcile.go`) decides
whether a reload touched a peer. It compares eight named members of
`ipsec.SiteToSitePeer`: the two addresses, the two group names, the connection
type, and three `Auth` members. `TrafficSelectors`, `Mode` and every other
member are absent from that list, so a reload that changes only those reports
"nothing changed". The peer lands in neither the removal list nor the start
list, and the running session is left alone.

`startPeerSession` (same file) is the only writer of `ps.peerCfg`. A session the
guard leaves alone therefore keeps the config value it was constructed with.
`sa.PeerCfg` is copied from that value (`initiator.go`, `responder.go`),
inherited across every rekey (`newRekeyedChild`, `internal/component/ike/engine/rekey.go`),
and read by `proposeChildTSPayloads` (same file) to build the TSi and TSr of
every CREATE_CHILD_SA this node initiates.

So the edit reaches neither the running SA nor the next rekey. `show vpn ipsec`
reads the re-parsed config, the enforcement reads the running session, and the
two disagree with nothing to say so.

**Why the failure direction is the dangerous one.** An operator narrows a peer's
traffic selectors to stop carrying a prefix. The commit succeeds. The tunnel
keeps carrying it. This is the same shape the BGP reload defect had, where an
operator tightening an import policy got the old permissive chain
(`plan/journal/guard-enumerates-instead-of-subtracting.md`, 2026-08-13).

**A second consequence, and the reason this was found.** RFC 7296 Section 2.9.2
names the case where "the policy was changed in a way such that the currently
used SA is against the policy". Reaching it needs a peer whose configured
selectors have narrowed since the tunnel came up. No Ze daemon can be that peer,
so the responder path that refuses such a rekey has no Ze-to-Ze producer and its
functional test reaches the case through a test-only override instead
(`narrowedRekeyPairs`, `internal/component/ike/engine/testport.go`).

Found while writing AC-4 of `spec-fixit-child-rekey-answer-vs-installed-selectors`,
which closed the same day. The stem is written bare rather than as a `plan/` path:
that spec's closure removes the file, and `find_dangling`
(`scripts/dev/spec-citation-check.py`) reads a `plan/spec-*.md` spelling as a live
citation of a file the tree no longer holds.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA programming and policy ownership
  → Constraint: the selector set and its orientation travel together on `ChildSA`. A reload that changes selectors changes what the dataplane must hold.
- [ ] `docs/architecture/ike/ipsec-13-rekey-wire.md` - the rekey exchange on the wire
  → Constraint: make-before-break. A replacement carries traffic before the retired pair is deleted, so a selector change cannot be applied late inside one exchange.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - Sections 2.8 and 2.9.2
  → Constraint: Section 2.9.2 states that when a policy change makes the SA in use non-conformant, "the SA should have been already deleted after the policy change took effect". Applying a narrowing by tearing the peer down is what the document expects, not by narrowing the live SA.

**Key insights:**
- The guard and the stored copy are two halves of one defect. Fixing the guard alone leaves `ps.peerCfg` stale for any path that reaches the session without a restart.
- The repository has already solved this exact shape once, in `peerSettingsEqual` and `peerSettingsSwapPlan` (`internal/component/bgp/reactor/`). Extend that answer rather than inventing a second one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/reconcile.go` - `peerConfigChanged` compares eight named fields; `startPeerSession` is the only writer of `ps.peerCfg`
- [ ] `internal/component/ike/engine/initiator.go` - copies `ps.peerCfg` into `sa.PeerCfg` at session start
- [ ] `internal/component/ike/engine/responder.go` - the same copy on the responder path
- [ ] `internal/component/ike/engine/rekey.go` - `newRekeyedChild` inherits the config across a rekey; `proposeChildTSPayloads` builds TSi and TSr from `sa.PeerCfg`
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `peerSettingsSwapPlan`, the swap-or-restart decision this fix mirrors
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `peerSettingsEqual`, the subtraction shape this fix mirrors

**Behavior to preserve:**
- A reload that changes nothing must restart no peer. A bouncing tunnel on every commit is a worse defect than the one being fixed.
- The eight fields the guard already names keep forcing the action they force today.
- `test/ipsec/ipsec-child-rekey-xfrm.ci` and `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` stay green.

**Behavior to change:**
- `peerConfigChanged` becomes a SUBTRACTION from the whole `ipsec.SiteToSitePeer` value, so a member added tomorrow forces the conservative action until somebody classifies it on purpose.
- A peer whose config changed in a way the running session cannot absorb is restarted, and the restart rewrites `ps.peerCfg`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator edits a `vpn ipsec site-to-site peer` block and commits.
- Format at entry: the resolved config tree, delivered to the IKE engine's reconcile path as `ipsec.SiteToSitePeer` values.

### Transformation Path
1. The config commit produces a new peer set and calls the engine's reconcile entry point.
2. `peerConfigChanged` (`internal/component/ike/engine/reconcile.go`) compares each peer against the running session's `ps.peerCfg` -- the defect is here.
3. A peer reported changed is stopped and restarted through `startPeerSession`, which writes the fresh `ps.peerCfg`.
4. `initiator.go` or `responder.go` copies that value into `sa.PeerCfg`.
5. `proposeChildTSPayloads` (`internal/component/ike/engine/rekey.go`) reads `sa.PeerCfg` to build the TS payloads of the next CREATE_CHILD_SA.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config commit ↔ IKE engine | the reconcile call carrying the new peer set | No |
| Engine ↔ peer daemon | TSi and TSr in the next CREATE_CHILD_SA | No |
| Engine ↔ dataplane | `dataplane.SPParams` for the replacement Child SA | No |

### Integration Points
- `peerConfigChanged` - the single decision point for whether a reload touched a peer
- `startPeerSession` - the single writer of `ps.peerCfg`
- `peerSettingsSwapPlan` (`internal/component/bgp/reactor/peer_settings_apply.go`) - the precedent to follow, not to copy verbatim

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ipsec.SiteToSitePeer` holds no member that changes on every reload without an operator edit | the type's definition, read whole | a subtraction guard restarts every peer on every commit, which is worse than the defect | a reload with no config change that restarts no peer | unvalidated |
| A-2 | Restarting a peer is an acceptable way to apply a selector change | RFC 7296 Section 2.9.2 states the SA "should have been already deleted after the policy change took effect" | the fix needs a live write to `sa.PeerCfg` that races the peer goroutine, which is a larger design | the RFC text read whole, plus a functional test measuring the tunnel re-establish | unvalidated |
| A-3 | No path reads `ps.peerCfg` or `sa.PeerCfg` expecting the value the session started with rather than the current one | every reader of both fields, enumerated with `gopls references` | a restart-based fix changes behavior somewhere this spec did not look | the enumeration, recorded in the spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A subtraction guard over a struct holding an unstable member restarts every peer on every commit | the reload functional test bounces a tunnel it should leave alone | A-1 is validated before the guard lands, and the unstable member is classified with a named reason rather than by widening the fix |
| R-2 | Restarting a peer to apply a selector edit drops traffic an operator did not expect to drop | an operator reports a tunnel bounce on an unrelated commit | the guard restarts only for members that genuinely cannot be absorbed, and the user guide says which edits bounce a tunnel |
| R-3 | The test-only override this defect forced into the rekey path outlives the fix | `narrowedRekeyPairs` still has no non-test producer after this spec closes | once a Ze peer can narrow its own selectors, the AC-4 functional test of the sibling spec is rewritten to use that path and the override is deleted |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Either a live tunnel bounces on an unrelated commit, or an operator's selector edit continues to reach no wire |
| How is it reverted? | Single commit revert. No config migration, no persisted state |
| Who else touches this path? | `spec-fixit-child-rekey-answer-vs-installed-selectors` owns the rekey answer this config feeds |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator edits a live peer's traffic selectors and commits | → | `peerConfigChanged` | `TestPeerConfigChangedIsFailClosed` |
| The same edit, end to end, on a running daemon | → | `startPeerSession`, `proposeChildTSPayloads` | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A reload that changes a peer's `TrafficSelectors` | The peer is restarted, and the next CREATE_CHILD_SA proposes the new selectors |
| AC-2 | A reload that changes no member of a peer | No peer is restarted and no tunnel bounces |
| AC-3 | A member is added to `ipsec.SiteToSitePeer` and classified nowhere | The guard reports the peer changed, so the conservative action is the default |
| AC-4 | A reload that changes one of the eight members the guard already names | The action taken today is the action taken after the fix |
| AC-5 | AC-1 on a running daemon against the real dataplane | The kernel policy selectors after the reload equal the newly configured set |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Narrows a site-to-site peer's traffic selectors to stop carrying a prefix, and commits | config tree → reconcile → `peerConfigChanged` → `startPeerSession` → `proposeChildTSPayloads` → wire | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
| 2 | Commits an unrelated change while an IPsec tunnel carries traffic | config tree → reconcile → no restart | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerConfigChangedIsFailClosed` | `internal/component/ike/engine/reconcile_test.go` | an unclassified member of `ipsec.SiteToSitePeer` makes the guard report a change | |
| `TestPeerConfigChangedIgnoresNothingSilently` | `internal/component/ike/engine/reconcile_test.go` | every member the guard subtracts is subtracted by name, with a reason | |
| `TestPeerConfigChangedNoEditNoChange` | `internal/component/ike/engine/reconcile_test.go` | two identical peer values compare equal, so a no-op reload restarts nothing | |
| `TestStartPeerSessionWritesTheFreshConfig` | `internal/component/ike/engine/reconcile_test.go` | a restarted session's `peerCfg` is the new value, not the old one | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Traffic selectors per peer | 0..N | N | N/A | N/A |
| Selector port | 0-65535 | 65535 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-peer-reload-applies-selectors` | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | an operator narrows the selectors and the kernel policy follows | | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
| `ipsec-peer-reload-leaves-tunnel-alone` | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | an unrelated commit does not bounce a live tunnel | | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-peer-reload-narrowing` | `test/interop-ipsec/scenarios/` | strongSwan | a Ze peer whose selectors narrow mid-tunnel proposes the new set, and strongSwan's SPD follows | |

## Files to Modify
- `internal/component/ike/engine/reconcile.go` - `peerConfigChanged` becomes a subtraction; the restart path rewrites `ps.peerCfg`
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the design doc `reconcile.go` declares: state when a reload restarts a peer and when it leaves the session alone
- `docs/guide/ipsec.md` - which peer edits restart a tunnel and which do not

## Files to Create
- `test/ipsec/ipsec-peer-reload-applies-selectors.ci` - the AC-1 and AC-5 proof <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
- `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` - the AC-2 proof <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf changes; the existing peer block is what stops being ignored |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | no new env var |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | a peer restart is already logged; a counter would be a second record of one event |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a defect is removed |
| 2 | Config syntax changed? | No | no leaf changes |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`: which peer edits restart a tunnel |
| 7 | Wire format changed? | No | the payloads are unchanged; their content follows the config |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 7296 Section 2.9.2's policy-change case gains a Ze producer; check whether a tag can move off the test-only override |
| 10 | Test infrastructure changed? | No | existing runners |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md`, the design doc `reconcile.go` declares in its `// Design:` header: the reload contract is an engine property and the page must say when a peer restarts. Also `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` if the reload contract is described there |
| 13 | Route metadata keys added/changed? | N-A | not routing |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `engine/reconcile.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify the peer examples in `docs/guide/ipsec.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the defect and the guard's shape before changing it
   - Tests: `TestPeerConfigChangedIsFailClosed`, `TestPeerConfigChangedNoEditNoChange`
   - Files: `internal/component/ike/engine/reconcile_test.go`
   - Verify: the fail-closed test is red against today's eight-field guard, and the no-op test is green before and after
2. **Phase: Validate A-1 and A-3 before the guard changes** -- read `ipsec.SiteToSitePeer` whole, and enumerate every reader of `ps.peerCfg` and `sa.PeerCfg`
   - Files: the spec's Assumptions table
   - Verify: each A-N flips to `confirmed` or `broken`. A broken A-1 stops the phase and goes to the user
3. **Phase: Subtraction** -- the guard compares the whole value minus members classified by name
   - Tests: the four unit tests above
   - Files: `internal/component/ike/engine/reconcile.go`
   - Verify: red before, green after, and reverting the change reddens the fail-closed test
4. **Phase: The stale copy** -- a restarted session carries the new config through to the wire
   - Tests: `TestStartPeerSessionWritesTheFreshConfig`, `ipsec-peer-reload-applies-selectors`
   - Files: `internal/component/ike/engine/reconcile.go`
   - Verify: the next CREATE_CHILD_SA proposes the edited selectors
5. **Phase: Interop** -- the strongSwan scenario with a Ze peer that narrows
   - Tests: `test/interop-ipsec/scenarios/NN-peer-reload-narrowing` <!-- doc-links: ignore (this spec's own acceptance criteria create this artifact; the spec is ready and not yet authorised to run) -->
   - Verify: strongSwan's SPD follows Ze's new proposal

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The guard subtracts by name with a reason, and nothing is ignored by omission |
| Data flow | One value produces the guard's verdict and the restarted session's config |
| Rule: `ai/rules/evidence.md` | The guard fails closed: an unclassified member forces the conservative action |
| Rule: `ai/rules/simplicity.md` | The fix reuses the BGP precedent's shape and adds no layer the defect does not need |
| Rule: `ai/rules/interop-and-goal-validation.md` | Reverting the guard change reddens the functional test, not only the unit test |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The guard fails closed on an unclassified member | `make ze-unit-pkg-test PKG=./internal/component/ike/engine` |
| A selector edit reaches the wire | `make ze-qemu-integration-test` |
| An unrelated commit bounces nothing | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
| A strongSwan peer follows the new proposal | `make ze-interop-ipsec-test` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The peer config is operator-controlled. A narrowing that is ignored keeps carrying traffic the operator asked Ze to stop carrying, which is the fail-open direction |
| Resource exhaustion | A guard that reports a change too eagerly restarts every peer on every commit; A-1 bounds that |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A guard that lists what matters is correct on the day it is written and wrong on the day a field is added. The shape that survives compares the whole value and subtracts what one decision proved it can ignore.
- A guard and the copy it protects are two halves of one defect. Fixing the comparison while leaving the stored value writable from one place only moves the failure rather than removing it.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Compare the whole `ipsec.SiteToSitePeer` value and subtract classified members, then restart the peer to apply what changed | **B. Write the new config into the live session.** REJECTED for the first release: `sa.PeerCfg` is read by the peer goroutine, so a live write needs a synchronisation design this defect does not require. **C. Add `TrafficSelectors` to the existing eight-field list.** REJECTED: it fixes today's missing field and leaves tomorrow's, which is the defect rather than an instance of it | The subtraction shape already shipped one component over (`peerSettingsEqual`, `internal/component/bgp/reactor/reactor_api.go`) and is proven there. RFC 7296 Section 2.9.2 states that a policy change should already have deleted the SA, so a restart is what the document expects |

## Known Limitations

- The VPP IPsec backend cannot be driven by IKE, so any fix is proven on XFRM only.
- A restart drops the tunnel briefly. Applying a selector change without a drop is a larger design and is not needed for the first release.

## RFC Documentation (Scope: protocol)

Add `// RFC 7296 Section 2.9.2: "<quoted requirement>"` above the code that
restarts a peer whose policy changed. Tag the functional test with
`RFC requirement: RFC7296-2.9.2-2 positive` once a Ze peer can produce the
narrowing without the test-only override.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
