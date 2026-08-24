# Spec: fixit-ipsec-peer-reload-ignored

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 1/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-24 |

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
| A-1 | `ipsec.SiteToSitePeer` holds no member that changes on every reload without an operator edit | the type's definition, read whole | a subtraction guard restarts every peer on every commit, which is worse than the defect | a reload with no config change that restarts no peer | confirmed |
| A-2 | Restarting a peer is an acceptable way to apply a selector change | RFC 7296 Section 2.9.2 states the SA "should have been already deleted after the policy change took effect" | the fix needs a live write to `sa.PeerCfg` that races the peer goroutine, which is a larger design | the RFC text read whole, plus a functional test measuring the tunnel re-establish | confirmed |
| A-3 | No path reads `ps.peerCfg` or `sa.PeerCfg` expecting the value the session started with rather than the current one | every reader of both fields, enumerated with `gopls references` | a restart-based fix changes behavior somewhere this spec did not look | the enumeration, recorded in the spec | confirmed |
| A-4 | A config reload delivers the new `vpn` section to the IKE engine, so `peerConfigChanged` runs | the spec's own Data Flow step 1, which reads the reconcile call as the commit's consequence | the whole spec is inert: the guard is reachable from startup and operator `clear` alone, and no edit reaches the wire whatever the guard says | a daemon reloaded with an edited peer, watched for `ike: stopping peer` | **broken** |

**A-1 evidence.** `TestSiteToSitePeerEqualAcrossTwoParses`
(`internal/component/ike/ipsec/config_test.go`) parses one configuration twice and the
two peers compare equal, prefixes and allow list included. A live daemon reloaded with an
added second peer started that peer and logged no `ike: stopping peer` for the running
one.

**A-2 evidence.** Interop scenario 27 measures the whole cycle against strongSwan: the
tunnel drops, Ze re-initiates, and both SPDs hold the narrowed pair.

**A-3 evidence.** Every reader of `sa.PeerCfg` takes it as the peer's CURRENT
configuration: `auth.go`, `cert_payload.go`, `remote_id.go` and `responder_eap.go` read
the `Auth` members, `initiator.go`, `responder.go`, `rekey.go`, `fsm.go` and
`transport_mode.go` read the addresses and the mode, and `ts_narrow.go` reads the
selectors. `ps.peerCfg` is read by `Info` (`reconcile.go`) and by the responder match in
`register.go`. None wants the value the session was born with.

**A-4 was BROKEN, and it blocked every acceptance criterion.** The IKE engine registered
`OnConfigVerify` and `OnConfigure` and no `OnConfigApply`. The SDK answers a config-apply
with OK when no handler is registered (`pkg/plugin/sdk/sdk_callbacks.go`), so every reload
verified the operator's edit, reported success, and applied nothing.
`reconcilePeers` was reachable from startup and from operator `clear` alone. Measured on a
running daemon: SIGHUP printed `config reload completed` and the engine logged neither
`ike: stopping peer` nor a second `ike engine configured`. Fixed in
`internal/component/ike/engine/register.go` by splitting the apply out of `OnConfigure`
and registering the verify-stash plus apply pair every other reloadable plugin uses.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A subtraction guard over a struct holding an unstable member restarts every peer on every commit | the reload functional test bounces a tunnel it should leave alone | A-1 is validated before the guard lands, and the unstable member is classified with a named reason rather than by widening the fix |
| R-2 | Restarting a peer to apply a selector edit drops traffic an operator did not expect to drop | an operator reports a tunnel bounce on an unrelated commit | the guard restarts only for members that genuinely cannot be absorbed, and the user guide says which edits bounce a tunnel |
| R-3 | The test-only override this defect forced into the rekey path outlives the fix | `narrowedRekeyPairs` still has no non-test producer after this spec closes | **OPEN, and this spec does not close it.** The fix applies a narrowing by RESTARTING the peer, so the narrowed selectors travel on a fresh IKE_SA_INIT and never on a CREATE_CHILD_SA against a live SA. RFC 7296 Section 2.9.2's case needs a rekey whose proposal no longer covers the scope in use, and a restart removes that scope before proposing anything. So `narrowedRekeyPairs` (`internal/component/ike/engine/testport.go`) still has no Ze producer, the `RFC7296-2.9.2-2` tag does not move off it, and interop scenario 13's docstring stays correct. Scenario 27 is the Ze-side mirror of scenario 13 rather than a replacement for the override |

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
| The same edit, end to end, on a running daemon | → | `startPeerSession`, `proposeChildTSPayloads` | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |

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
| 1 | Narrows a site-to-site peer's traffic selectors to stop carrying a prefix, and commits | config tree → reconcile → `peerConfigChanged` → `startPeerSession` → `proposeChildTSPayloads` → wire | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |
| 2 | Commits an unrelated change while an IPsec tunnel carries traffic | config tree → reconcile → no restart | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerConfigChangedIsFailClosed` | `internal/component/ike/engine/reconcile_test.go` | an unclassified member of `ipsec.SiteToSitePeer` makes the guard report a change | green; 6 subtests red against the old field list |
| `TestPeerConfigChangedIgnoresNothingSilently` | `internal/component/ike/engine/reconcile_test.go` | every member the guard subtracts is subtracted by name, with a reason | green; all 15 rows red against the old field list |
| `TestPeerConfigChangedNoEditNoChange` | `internal/component/ike/engine/reconcile_test.go` | two identical peer values compare equal, so a no-op reload restarts nothing | green |
| `TestStartPeerSessionWritesTheFreshConfig` | `internal/component/ike/engine/reconcile_test.go` | a restarted session's `peerCfg` is the new value, not the old one | green; red against the old field list |
| `TestPeerConfigChangedKeepsTheMembersItAlreadyNamed` | `internal/component/ike/engine/reconcile_test.go` | AC-4: the eight members the old guard named still force a restart | green |
| `TestSiteToSitePeerEqualAcrossTwoParses` | `internal/component/ike/ipsec/config_test.go` | A-1: two parses of one configuration compare equal, prefixes and allow list included | green |
| `TestIPsecConfigChangedSeesEveryPeerMember` | `internal/component/ike/ipsec/config_test.go` | the sibling omission: `Changed` reports a peer whose only edit is a member the old list left out | green; all 3 rows red against the old field list |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Traffic selectors per peer | 0..N | N | N/A | N/A |
| Selector port | 0-65535 | 65535 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-peer-reload-applies-selectors` | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | an operator narrows the selectors and the kernel policy follows | PASS in QEMU (1.7s); RED with `WIDE-SURVIVED` against the old field list <!-- doc-links: ignore (this spec's own acceptance criteria create this file) --> |
| `ipsec-peer-reload-leaves-tunnel-alone` | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | an unrelated commit does not bounce a live tunnel | PASS in QEMU (9.2s); RED with `BOUNCED` against an always-changed `Equal` <!-- doc-links: ignore (this spec's own acceptance criteria create this file) --> |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `27-peer-reload-narrowing` | `test/interop-ipsec/scenarios/` | strongSwan | a Ze peer whose selectors narrow mid-tunnel proposes the new set, and strongSwan's SPD follows | PASS; RED against the old field list, stopping at "strongSwan Child SA never carried 10.1.0.0/24 <-> 10.2.0.0/25" |

## Files to Modify
- `internal/component/ike/engine/reconcile.go` - `peerConfigChanged` becomes a subtraction; the restart path rewrites `ps.peerCfg`
- `internal/component/ike/ipsec/types.go` - `SiteToSitePeer.Equal` is the one producer of the answer; the second hand-written field list (`peersEqual`) is deleted and `Changed` calls `Equal`
- `internal/component/ike/engine/register.go` - A-4's repair: the apply is split out of `OnConfigure` and reached by `OnConfigApply`, so a reload runs the reconcile at all
- `test/interop-ipsec/lab.py` - `reload_ze_config` rewrites the config the ze container mounts and SIGHUPs it; `render_ze_conf` and `find_pki_dir` are the extracted halves it shares with `Scenario`
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the design doc `reconcile.go` declares: state when a reload restarts a peer and when it leaves the session alone
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - its "a new peer field must join the comparison" paragraph described the shape this spec removes
- `docs/architecture/ike/ipsec-3-data-model.md` - the design doc `types.go` declares; its recorded decision rejected reflection-based equality, which is the shape that now answers
- `docs/guide/ipsec.md` - which peer edits restart a tunnel and which do not

## Files to Create
- `test/ipsec/ipsec-peer-reload-applies-selectors.ci` - the AC-1 and AC-5 proof <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
- `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` - the AC-2 proof <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
- `test/interop-ipsec/scenarios/27-peer-reload-narrowing/` - the strongSwan proof of AC-1 and AC-5

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
| An unrelated commit bounces nothing | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |
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
- **`IPsecConfig.Changed` has no non-test caller, and this spec KEPT it deliberately.**
  Only `config_test.go` calls it. It was already dead when this spec opened, so removing an
  exported method is not this spec's change to make, and leaving it as the second
  hand-written field list was not an option either: it would have kept the very shape the
  spec exists to remove, one caller away. It now asks `SiteToSitePeer.Equal`, so whoever
  deletes it later removes dead code rather than a divergent answer.
- **The transports and the virtual-IP pool are still create-if-nil, so an `interface` or
  `remote-access` edit is accepted and ignored.** `tr`, `trNATT` and `ipPool`
  (`internal/component/ike/engine/register.go`) are built only when the pointer is nil, and
  nothing rebuilds them. That was invisible while reload applied nothing; it is now
  reachable. It is not this spec's regression and not what it exists to fix, so it needs its
  own spec. The DANGEROUS half of the same area WAS a regression this spec created, and it
  is fixed here: a failed interface read on reload used to blank every peer's
  `LocalAddress`, and the guard would then restart every tunnel into a state the engine's
  own message calls "will fail". `applyIPsecConfig` now refuses that configuration, so the
  transaction rolls back and the running tunnels are untouched.

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

## Implementation Summary

### What Was Implemented

The fix landed in two commits before this closure ran: `5e14f7f51` (the spec's own
work) and `a40bedef5` (the verify-path parser). Closure added a third change, the
startup regression the review found. All three are described below.

- **`peerConfigChanged` is a subtraction** (`internal/component/ike/engine/reconcile.go`).
  It compares three whole values: the peer (`ipsec.SiteToSitePeer.Equal`) and the two
  RESOLVED groups (`ipsec.IKEGroup.Equal`, `ipsec.ESPGroup.Equal`). No member is named,
  so no member is ignored by omission.
- **The groups joined the comparison.** A peer holds only the NAMES of its groups, and
  `startPeerSession` copies the resolved groups onto the session with nothing to refresh
  them. Rotating a cipher inside `ike-group` edits no peer block, so the peer half
  compares equal and the tunnel kept negotiating the replaced algorithm.
- **The second field list is gone** (`internal/component/ike/ipsec/types.go`).
  `peersEqual` named eight members and `peerConfigChanged` named a DIFFERENT eight.
  `IPsecConfig.Changed` now asks `SiteToSitePeer.Equal`, so one function produces the
  answer.
- **The engine answers config-apply** (`internal/component/ike/engine/register.go`).
  It registered `OnConfigVerify` and `OnConfigure` and no `OnConfigApply`, and the SDK
  answers config-apply with OK when no handler is registered: the default callback table
  in `pkg/plugin/sdk/sdk_callbacks.go` binds `callbackConfigApply` to `marshalStatusOK`.
  `ikeConfigStaging` carries what verify parsed across to apply, and an apply with
  nothing staged returns `errIKEApplyWithoutVerify`.
- **The verify path uses the parser with no side effects** (`a40bedef5`). The staging
  parse was `parseIPsecSections`, which calls `pki.Load` on a `pki` section and swaps
  the process-wide certificate store. It is `parseVPNSections` now.
- **Startup and reload answer an unbindable peer set differently** (closure fix).
  `applyPhase` carries which delivery is running and `unbindablePeers` names the
  condition. A reload returns it, so the transaction rolls back. Startup logs it and
  applies the configuration, so the peers that carry their own `local-address` still
  come up.

### Bugs Found/Fixed

| Bug | Where | Test that now covers it |
|-----|-------|-------------------------|
| The engine registered no `OnConfigApply`, so every reload verified the edit, reported success, and applied nothing | `internal/component/ike/engine/register.go` | `TestIKEConfigApplyWithoutVerifyIsRefused`, and every reload `.ci` below |
| `peerConfigChanged` named eight of the twelve members of `SiteToSitePeer` | `internal/component/ike/engine/reconcile.go` | `TestPeerConfigChangedIsFailClosed`, `TestPeerConfigChangedIgnoresNothingSilently` |
| `peersEqual` named a DIFFERENT eight, so two guards answered one question two ways | `internal/component/ike/ipsec/types.go` | `TestIPsecConfigChangedSeesEveryPeerMember` |
| A cipher rotation inside a group reached no wire, because the guard compared the peer alone | `internal/component/ike/engine/reconcile.go` | `TestPeerConfigChangedSeesTheResolvedGroups`, `TestReconcilePeersRestartsOnGroupRotation` |
| The verify path called `pki.Load`, swapping the process-wide store for a config that could still be rejected | `internal/component/ike/engine/register.go` | `TestStagingParseDoesNotMutatePKIStore` |
| **Found at closure.** The reload refusal was returned to BOTH deliveries, so an `interface` that could not be read at boot left the engine with no peer session, no IKE socket and no NAT-T socket, including for peers the interface cannot affect | `internal/component/ike/engine/register.go` | `TestUnbindablePeersReportsOnlyTheDependentCase`, `test/ipsec/ipsec-startup-serves-bindable-peers.ci` |
| **Found at closure.** Nothing pinned that a group parses to ONE value. `Proposals` is a slice and `reflect.DeepEqual` is order-sensitive, so the guard's new group half rested on an untested sort | `internal/component/ike/ipsec/config.go` (already sorted; the property was unproven) | `TestGroupsEqualWhateverOrderTheyArriveIn` |

### Documentation Updates

| File | What changed | Source anchor |
|------|--------------|---------------|
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | when a reload restarts a peer, why the groups are compared, the `OnConfigApply` staging, and the startup/reload asymmetry | `register.go -- applyIPsecConfig, applyPhase, ikeConfigStaging, unbindablePeers, peersNeedInterfaceAddress` |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | its "a new peer field must join the comparison" paragraph described the shape this spec removed | `types.go -- SiteToSitePeer.Equal` |
| `docs/architecture/ike/ipsec-3-data-model.md` | the recorded decision rejected reflection-based equality; that decision is reversed and the reason is stated | `types.go -- SiteToSitePeer.Equal, AuthConfig.Equal` |
| `docs/guide/ipsec.md` | which peer edits restart a tunnel, that a group edit bounces every peer naming it, and that a commit is refused when `interface` supplies no address for a dependent peer | `reconcile.go -- reconcilePeers, peerConfigChanged` |

`make ze-doc-verify` result: every check this closure can affect is green. Source
anchors resolve for all five symbols the new `register.go` anchor names, drift
reports none, and the digest anchors resolve. The run exits non-zero on ONE anchor
that belongs to another session's uncommitted work: `docs/guide/web-interface.md`
names `liveAAABundleAuthenticator.Authenticate` in `cmd/ze/hub/aaa_authenticator_web.go`,
that doc is unmodified, and the `.go` file's working-tree copy no longer declares
the symbol (three occurrences at HEAD, one in the tree).

### Deviations from Plan

| Planned | Actual | Why |
|---------|--------|-----|
| Modify `peerConfigChanged` and `types.go` | Also `register.go`, which the spec did not open at design time | A-4 was BROKEN: no reload reached the engine at all, so the guard fix alone would have changed nothing visible. It blocked the goal, so it was in scope (`ai/rules/completion.md`) |
| Compare the peer value | Compare the peer AND both resolved groups | A peer holds group NAMES and no crypto. Comparing the peer alone leaves a cipher rotation unapplied, which is the same defect one indirection along |
| R-3 closed by this spec | R-3 stays OPEN | A restart applies the narrowing on a fresh IKE_SA_INIT, never on a CREATE_CHILD_SA against a live SA, so RFC 7296 Section 2.9.2's rekey case still has no Ze producer and its requirement stays on the test-only override |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4 assumed a config reload delivered the new `vpn` section to the IKE engine, so `peerConfigChanged` ran | The engine registered no `OnConfigApply`, and the SDK answers config-apply OK when no handler is registered. `reconcilePeers` was reachable from startup and from operator `clear` alone | A running daemon was SIGHUPed and logged `config reload completed` with neither `ike: stopping peer` nor a second `ike engine configured` | Fixed in `register.go`: the apply is split out of `OnConfigure` and reached by `OnConfigApply` |
| approach | The reload refusal for an unbindable peer set was placed inside the shared apply body, and `OnConfigure` was left to swallow the error | The early return skipped the transports, the reconcile and the active-config store, so STARTUP applied nothing at all. The code comment and the architecture page both said the opposite | The closure review read `applyIPsecConfig` from source and traced the startup caller | `applyPhase` carries the delivery into the branch. `test/ipsec/ipsec-startup-serves-bindable-peers.ci` reddens on the exact assertion the defect made silent |
| escalation | Two guards over one type each named their own field list, in one package, and disagreed | The recurrence is the point, not the instance: `plan/journal/guard-enumerates-instead-of-subtracting.md` already held `bgp-peer-settings-reload-ignored`, the same shape one component over | The spec's own research read both lists | A second row was added to that journal class rather than a second local fix |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An operator's edit to a live IPsec peer reaches the wire | Done | `peerConfigChanged`, `internal/component/ike/engine/reconcile.go` | The restart is what carries it: `startPeerSession` is the only writer of `ps.peerCfg` |
| The guard must not ignore a member by omission | Done | `SiteToSitePeer.Equal`, `internal/component/ike/ipsec/types.go` | Total comparison; a subtraction is by name with the reason recorded there |
| A reload that changes nothing must restart no peer | Done | `TestPeerConfigChangedNoEditNoChange`, `TestSiteToSitePeerEqualAcrossTwoParses` | R-1's early signal, plus `ipsec-peer-reload-leaves-tunnel-alone.ci` |
| A reload must reach the engine at all | Done | `p.OnConfigApply`, `internal/component/ike/engine/register.go` | A-4's repair |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` PASS 5.5s; interop scenario `27-peer-reload-narrowing` PASS | The kernel policy names the narrowed prefix and no policy names the wide one |
| AC-2 | Done | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` PASS 25.1s | It carries `reject=stderr:pattern=ike: stopping peer` |
| AC-3 | Done | `TestPeerConfigChangedIsFailClosed` | Walks `reflect.VisibleFields`, so a member added tomorrow needs no memory of this test |
| AC-4 | Done | `TestPeerConfigChangedKeepsTheMembersItAlreadyNamed` | The eight the old guard named still force a restart |
| AC-5 | Done | `ipsec-peer-reload-applies-selectors.ci` reads `ip xfrm policy`; scenario 27 reads both SPDs | Both kernels hold the narrowed pair and neither holds the wide one |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPeerConfigChangedIsFailClosed` | Done | `internal/component/ike/engine/reconcile_test.go` | |
| `TestPeerConfigChangedIgnoresNothingSilently` | Done | `internal/component/ike/engine/reconcile_test.go` | |
| `TestPeerConfigChangedNoEditNoChange` | Done | `internal/component/ike/engine/reconcile_test.go` | |
| `TestStartPeerSessionWritesTheFreshConfig` | Done | `internal/component/ike/engine/reconcile_test.go` | |
| `TestPeerConfigChangedKeepsTheMembersItAlreadyNamed` | Done | `internal/component/ike/engine/reconcile_test.go` | |
| `TestSiteToSitePeerEqualAcrossTwoParses` | Done | `internal/component/ike/ipsec/config_test.go` | Two selectors and two allowed prefixes, so slice order is exercised |
| `TestIPsecConfigChangedSeesEveryPeerMember` | Done | `internal/component/ike/ipsec/config_test.go` | |
| `ipsec-peer-reload-applies-selectors` | Done | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | |
| `ipsec-peer-reload-leaves-tunnel-alone` | Done | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | |
| `27-peer-reload-narrowing` | Done | `test/interop-ipsec/scenarios/27-peer-reload-narrowing/` | |
| `TestUnbindablePeersReportsOnlyTheDependentCase` | Changed | `internal/component/ike/engine/reconcile_test.go` | Added at closure for review finding 1; not in the original plan |
| `TestGroupsEqualWhateverOrderTheyArriveIn` | Changed | `internal/component/ike/ipsec/config_test.go` | Added at closure for review finding 2; A-1 for the group half the guard gained |
| `ipsec-startup-serves-bindable-peers` | Changed | `test/ipsec/ipsec-startup-serves-bindable-peers.ci` | Added at closure for review finding 1 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/reconcile.go` | Done | |
| `internal/component/ike/ipsec/types.go` | Done | `peersEqual` deleted |
| `internal/component/ike/engine/register.go` | Done | A-4's repair, plus the closure fix |
| `test/interop-ipsec/lab.py` | Done | `reload_ze_config`, `render_ze_conf`, `find_pki_dir` |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Done | |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Done | |
| `docs/architecture/ike/ipsec-3-data-model.md` | Done | |
| `docs/guide/ipsec.md` | Done | |
| `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | Done | |
| `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | Done | |
| `test/interop-ipsec/scenarios/27-peer-reload-narrowing/` | Done | |
| `internal/component/ike/engine/config_verify_test.go` | Changed | Created by `a40bedef5`, not named in the plan |
| `test/ipsec/ipsec-startup-serves-bindable-peers.ci` | Changed | Created at closure, not named in the plan |

### Audit Summary
- **Total items:** 35
- **Done:** 31
- **Partial:** 0 (each needs user approval)
- **Skipped:** 0 (each needs user approval)
- **Changed:** 4 (three tests and one `.ci` added beyond the plan; each is recorded in Deviations or the Mistake Log)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator who edits a live IPsec peer and commits reaches the wire | functional | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` PASS 5.5s. It reads `ip xfrm policy` before and after the SIGHUP and requires the narrow pair PRESENT and the wide pair ABSENT, and requires the ESP SPIs to be a different pair, so a peer that never restarted fails |
| The same edit is accepted by another implementation | interop | `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=27-peer-reload-narrowing` PASS. strongSwan's Child SA moves from `10.1.0.0/24 <-> 10.2.0.0/24` to `10.1.0.0/24 <-> 10.2.0.0/25`, both SPDs hold the narrowed pair, and the two are compared to each other |
| A commit that touches nothing must not bounce a live tunnel | functional | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` PASS 25.1s, carrying `reject=stderr:pattern=ike: stopping peer` |
| A member added to the peer type cannot be ignored in silence | unit | `TestPeerConfigChangedIsFailClosed` walks `reflect.VisibleFields` and turns a kind it cannot mutate into a failure, so it needs no maintenance to cover a member added later |
| A reload reaches the engine at all | functional | Both reload `.ci` tests drive a real SIGHUP against a running daemon. Against the pre-fix engine the first reports `WIDE-SURVIVED` and the guard never runs |
| Startup still serves the peers it can when the configured interface supplies no address | functional | `test/ipsec/ipsec-startup-serves-bindable-peers.ci` PASS 3.5s. Simulating the landed defect (the refusal returned to both deliveries, `OnConfigure` swallowing it) reddens it at `stderr does not contain "ike: started peer"`, which is the assertion the defect made silent |
| A group parses to ONE value, so the new group comparison cannot bounce every tunnel | unit | `TestGroupsEqualWhateverOrderTheyArriveIn`. Removing the `sort.Slice` in `parseESPGroup` reddens it with `one esp-group parsed to two values` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The spec declares `Deferral shard: -`, and `ls plan/deferrals/` holds no `fixit-ipsec-peer-reload-ignored.md` | done | Nothing to resolve and no shard to remove |
| R-3, the test-only `narrowedRekeyPairs` override | deferred | OPEN and stated in the spec's Risks table. A restart applies a narrowing on a fresh IKE_SA_INIT, so RFC 7296 Section 2.9.2's rekey case still has no Ze producer. `testport.go`'s comment says so and interop scenario 13's docstring stays correct |
| The `tr`, `trNATT` and `ipPool` create-if-nil staleness, so an `interface` or `remote-access` edit is accepted and ignored | deferred | Recorded in Known Limitations and in `plan/journal/stale-artifact-reused.md`. It predates this spec and needs its own work |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ipsec-peer-reload-ignored-9ad8358c-695f-41be-8019-5d92ba08f8e6.md`, 14 files, `verdict=clean` |
| `review_gate.py check` | `review_gate: OK (4 code files, clean, hashes match ...)`, exit 0 |
| Rounds | 2 |
| Reviewer lenses used | wiring and reachability, removed-behavior audit, logic correctness, comment-versus-code agreement, edge cases over the new total comparisons, security over operator-controlled config, allocation and hot path, project rules and `docs/contributing/ze-style.md`, RFC 7296 conformance, interop and goal validation |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The reload refusal for an unbindable peer set was returned to BOTH deliveries. At startup `applyIPsecConfig` returned before the transports, the reconcile and the active-config store, and `OnConfigure` logged the error and returned nil, so an `interface` that could not be read at boot left the engine with NO peer session, NO IKE socket and NO NAT-T socket, including for peers that carry their own `local-address`. The function's own comment and `docs/architecture/ike/ipsec-7-ikev2-engine.md` both claimed the opposite | `internal/component/ike/engine/register.go`, `applyIPsecConfig` interface branch | `applyPhase` carries the delivery into the branch and `unbindablePeers` names the condition. A reload returns it; startup logs it and applies. `TestUnbindablePeersReportsOnlyTheDependentCase` plus `test/ipsec/ipsec-startup-serves-bindable-peers.ci`, the second proven to redden against a faithful simulation of the defect |
| 2 | ISSUE | The guard gained a group half with nothing pinning that a group parses to ONE value. `Proposals` is a slice and `reflect.DeepEqual` is order-sensitive, so the whole group comparison rested on a `sort.Slice` no test named. `TestSiteToSitePeerEqualAcrossTwoParses` covers the peer half only, and `TestPeerConfigChangedSeesTheResolvedGroups` builds its groups in Go rather than parsing them | `internal/component/ike/ipsec/config.go`, `parseIKEGroup` and `parseESPGroup` | `TestGroupsEqualWhateverOrderTheyArriveIn` parses one pair of proposals declared in opposite order. Removing either `sort.Slice` reddens it |
| 3 | NOTE | `register.go` is over the 1000-line advisory in `docs/contributing/ze-style.md`. The staging and interface helpers read as a second concern, but `applyIPsecConfig` is a closure over `runEngine`'s state and cannot move with them, so a split would scatter one concern across two files | `internal/component/ike/engine/register.go` | Recorded, not fixed. NOTEs do not block |
| 4 | NOTE | `IPsecConfig.Changed` has no non-test caller. It was already dead when the spec opened, and it was kept deliberately so the second field list could be deleted rather than left one caller away | `internal/component/ike/ipsec/types.go` | Recorded in Known Limitations. Removing an exported method is not this spec's change |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | Yes | `ls -la` reports 11414 bytes |
| `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | Yes | `ls -la` reports 11532 bytes |
| `test/ipsec/ipsec-startup-serves-bindable-peers.ci` | Yes | `ls -la` reports 4463 bytes |
| `test/interop-ipsec/scenarios/27-peer-reload-narrowing/` | Yes | `ls` reports `check.py`, `swanctl.conf`, `ze.conf`, `ze-narrowed.conf` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A `TrafficSelectors` edit restarts the peer and the next exchange proposes the new set | `ipsec-peer-reload-applies-selectors` PASS 5.5s in a `pass 21/21 100.0% 64.1s` run; scenario 27 PASS against strongSwan |
| AC-2 | A reload that changes no member restarts nothing | `ipsec-peer-reload-leaves-tunnel-alone` PASS 25.1s in the same run |
| AC-3 | An unclassified member forces the conservative answer | `TestPeerConfigChangedIsFailClosed` in `ok github.com/ze-software/ze/internal/component/ike/engine 39.900s` |
| AC-4 | The eight members the old guard named still force a restart | `TestPeerConfigChangedKeepsTheMembersItAlreadyNamed`, same run |
| AC-5 | The kernel policy after the reload equals the newly configured set | `ipsec-peer-reload-applies-selectors` reads `ip xfrm policy` and requires the wide pair ABSENT; scenario 27 compares both containers' SPDs and fails if they differ |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An operator edits a live peer's traffic selectors and commits | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` | Yes. Read: it copies `config2.conf` over `ze-bgp.conf`, reads `daemon.pid`, sends SIGHUP, then polls `ip xfrm policy` and `ip xfrm state` |
| The same edit, end to end, on a running daemon | `TestStartPeerSessionWritesTheFreshConfig` plus the same `.ci` | Yes |
| An unrelated commit under a live tunnel | `test/ipsec/ipsec-peer-reload-leaves-tunnel-alone.ci` | Yes. Read: it rejects `ike: stopping peer` on stderr for the life of the run |
| The daemon starts when `interface` supplies no address | `test/ipsec/ipsec-startup-serves-bindable-peers.ci` | Yes. Read: it asserts `ike: started peer` and `ike engine configured` on stderr, both of which the defect suppressed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestSiteToSitePeerEqualAcrossTwoParses` (two selectors, two allowed prefixes) and `TestGroupsEqualWhateverOrderTheyArriveIn` (two proposals per group, declared in opposite order). `ipsec-peer-reload-leaves-tunnel-alone` is the running-daemon half |
| A-2 | confirmed | Interop scenario 27 measures the whole cycle against strongSwan: the tunnel drops, Ze re-initiates, and both SPDs hold the narrowed pair |
| A-3 | confirmed | Every reader of `sa.PeerCfg` and `ps.peerCfg` is enumerated in the spec's Risks section; none wants the value the session was born with |
| A-4 | **broken** | The engine registered no `OnConfigApply`. Mistake Log row 1; fixed in `register.go` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "A reload restarts a peer whose configuration differs in ANY member" (`ipsec-7-ikev2-engine.md`) | `peerConfigChanged` compares three whole values through `.Equal` | Yes |
| "STARTUP logs the condition and applies the configuration anyway" (`ipsec-7-ikev2-engine.md`) | `applyPhase` and the `unbindable != nil && phase == applyReload` branch, plus `ipsec-startup-serves-bindable-peers.ci` | Yes, corrected at closure. The page previously said this while the code did the opposite |
| "A new peer field needs no edit to the comparison" (`ipsec-8-ikev2-child-xfrm.md`) | `SiteToSitePeer.Equal` is `reflect.DeepEqual` over the whole value | Yes |
| "`Changed()` compares the WHOLE value" (`ipsec-3-data-model.md`) | `IPsecConfig.Changed` calls `oldPeer.Equal(newPeers[name])` | Yes |
| "A commit is REFUSED when `interface` cannot supply an address" (`docs/guide/ipsec.md`) | `unbindablePeers` and the `applyReload` branch; the error text names the interface and the peer count | Yes, added at closure |
| Categories answered No: YANG, CLI, API/RPC, plugin, wire format, plugin SDK, comparison table, route metadata, Prometheus, registered inventory | No leaf, command, RPC, or registration changed. The payloads are unchanged; their content follows the config | Yes |
| Doctor checks | No new runtime dependency: the change adds no file path, socket, kernel module, listen port, external binary or certificate | Yes |
| RFC status | RFC 7296 Section 2.9.2 gains no NEW proven requirement. The restart removes the scope before anything proposes against it, so the rekey case it names still has no Ze producer and its tag stays on the test-only override (R-3) | Yes |

## Core Insight

A guard and the value it protects are two halves of one defect, and a guard that
nothing calls is a third. This spec met all three at once: `peerConfigChanged`
compared a list of names, `ps.peerCfg` was written from one place, and no
`OnConfigApply` was registered, so the first two were unreachable and looked
correct. The unreachability is what let the other two live: a guard nobody runs
produces no wrong answer for anybody to notice.

The closure found the same shape a fourth time, in the fix itself. Making the
reload REFUSE an unbindable peer set was right; putting that refusal in the body
both deliveries share gave startup an answer nobody chose. A decision that
differs by caller belongs at the caller, or in a value the caller passes in.
