# Spec: BGP per-peer pre-policy received counter

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-bgp-filtered-route-storage (for the received-vs-accepted gap to be attributable) |
| Phase | - |
| Updated | 2026-08-08 |

## The offered-versus-installed question is ANSWERED (2026-08-07)

`plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` routed one question
here and told whoever took this spec to put it to Thomas before changing the
counter: does a prefix limit count what the peer OFFERED or what ze INSTALLED?
RFC 4271 Section 6.7 does not say.

**Thomas ruled: "offer the selection/option at configuration level and let user
pick what is best for them."** It landed the same day, one layer down, as the
per-family `prefix { count offered|installed; }` leaf
(`internal/component/bgp/yang/ze-bgp-conf.yang`). The default is `offered`,
which is what the counter did before, so nothing this spec measured has moved.

→ Decision: the wire tally has no ONE meaning any more. It has a per-family
meaning the operator selects, and `Session.prefixCounts` is now read through
`applyInstalledPrefixSections` (`internal/component/bgp/reactor/session_prefix.go`,
named `applyInstalledPrefixDeltas` until 2026-08-08) under whichever mode the
family asked for. Any surface this spec adds MUST say
which mode produced the number, or the operator cannot tell a peer that overshot
from a peer that is at its limit. That is a new labelling obligation on the
`PeerInfo` field and the summary row, not a new number.

→ Constraint: this does NOT unblock the spec, and the reason narrowed on
2026-08-08 rather than going away. As written here the claim was that A-4's drift
holds under BOTH modes. That is now true of `offered` ONLY. `installed` was given
the per-prefix identity this paragraph said no mode had, so it no longer drifts.
The spec stays blocked because its Task is a PRE-POLICY RECEIVED count, and
neither mode is one: `offered` counts announcements rather than prefixes, and
`installed` counts prefixes ze delivered rather than prefixes the peer sent.

→ Constraint (REWRITTEN 2026-08-08, and this one narrows the spec rather than
widening it): the `installed` mode no longer keeps a second map of refused
announcements. `prefixCounts.dropped` was measured broken four ways and is
DELETED. The mode now holds `prefixCounts.sets`, one set per installed family
keyed on each NLRI's wire encoding, and `counts[fk]` is that set's size
(`applyInstalledPrefixSections`, `internal/component/bgp/reactor/session_prefix.go`).
A snapshot of `counts` is therefore complete on its own: there is no second
number the enforcement path reads. The evidence and the four measured breakages
are in `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md`.

→ Decision: A-4 is now MODE-DEPENDENT, and the spec's central premise survives
only for `installed`. Under `offered` (the default) the tally still counts wire
events, so it still drifts upward under implicit withdraw exactly as A-4 says.
Under `installed` it does not drift at all: a re-announced prefix is already in
the set. What `installed` counts is the set of prefixes DELIVERED TO PLUGINS and
not since withdrawn, which is taken before import policy runs and is therefore
neither the Adj-RIB-In size nor FRR's `pcount` (post-install) nor BIRD's
`imp_routes` (post-import-filter). A surface built on it must say so, and must
say which mode produced the number.

Correction (2026-07-22 plan review): the BLOCKED note's reason #2 below (the
Depends spec is "superseded" by a Phase-B pre-policy store that "probably
dissolves this spec") is STALE -- both cited dependencies withdrew that
premise on the same day it was written:
`spec-bgp-peer-settings-reload-ignored.md` D-1b says "No new store anywhere"
and D-4 is "WITHDRAWN 2026-07-16. It is NOT superseded";
`spec-bgp-filtered-route-storage` now explicitly rejects any pre-policy
store. The spec REMAINS legitimately blocked, but on reason #1 alone (A-4:
the tally drifts under implicit withdraw -- `session_prefix.go`
`prefixCounts` holds no per-prefix identity and `:196` `add()` does an
unconditional `+= delta`).

## BLOCKED: do not implement as currently written

An implementation attempt on 2026-07-16 stopped during the pre-code audit. Two
findings, the first of which invalidates the spec's central premise. Both are
evidenced below; a future session should re-design, not re-plumb.

1. **The tally is pre-policy (A-1 holds) but it is NOT an accurate count of what
   the peer sent.** It drifts upward without bound under implicit withdraw. See
   A-4 in Risks & Assumptions. The spec's framing (lines "the reactor already
   maintains it internally", "this spec is about safe exposure ... not about
   computing a new number") is WRONG: an honest pre-policy count needs per-prefix
   identity tracking, which does not exist today.
2. **The Depends spec is superseded, and its replacement probably dissolves this
   spec too.** During this audit `spec-bgp-filtered-route-storage` went
   `skeleton` -> `in-progress` -> `blocked` in a concurrent session, which
   concluded (its D-4) that it is **superseded, not merely blocked**, by
   `spec-bgp-peer-settings-reload-ignored`: that spec's Phase B was read as
   building a **pre-policy store**, making `routes_filtered` a QUERY over that
   store rather than separate storage.

   **That reading is VOID, and so is the supersession it produced.**
   `spec-bgp-peer-settings-reload-ignored` withdrew it (its D-4) and then built
   no store of any kind (its D-5): the delivered design is an atomic settings
   swap on the running peer, `peerSettingsSwapPlan`
   (`internal/component/bgp/reactor/peer_settings_apply.go`). It closed on
   2026-08-13. No store exists for `routes_filtered` to query, so this spec owns
   its own storage question.

   **Read that before re-designing this one.** A store keyed by prefix identity is
   exactly what A-4 says the wire tally lacks: with it a re-announce REPLACES
   instead of incrementing, so an honest pre-policy count becomes a query over the
   store -- no new atomics, no `PeerInfo` plumbing, and most of this spec's Data
   Flow and Files-to-Modify sections evaporate. Do NOT implement the atomic-snapshot
   design here until Phase B's shape is known; re-derive against it.

   (Collision note, kept as history: this spec targets `cmd/peer/summary.go`,
   `lg/handler_api.go`, `summary_test.go` and
   `test/plugin/bgp-summary-route-counts.ci`, all of which the superseded spec also
   listed -- expect the same overlap with whatever replaces it, on `main`, in one
   shared working tree.)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/bgp/reactor.md` - reactor concurrency model
4. `internal/component/bgp/reactor/session_prefix.go`, `internal/component/bgp/reactor/reactor_api.go`

## Task

Surface a per-peer PRE-policy received count: how many prefixes the peer advertised
on the wire BEFORE import policy, distinct from the post-policy accepted count that
`show bgp` already reports. Today `routes-received` equals `routes-accepted`
(both are the Adj-RIB-In size), so the number of routes the peer sent before filtering
is invisible in the CLI and in the birdwatcher Looking Glass, even though the reactor
already maintains it internally for prefix-limit enforcement.

This is a deferred item from `spec-bgp-summary-route-counts` (Known Limitations). It was
NOT implemented there because the internal counter is mutated on the session hot path
without the reactor lock, so reading it from the peer-snapshot API is a data race; making
it safe needs new atomics plus session-to-peer plumbing for a signal whose gap cannot yet
be attributed (filtered routes are untracked; see the Depends spec) and that is already
exposed as a Prometheus gauge. This spec records the design so a future session can
implement it deliberately rather than half-land it.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `docs/architecture/bgp/reactor.md` - reactor lock discipline and session goroutine ownership
  → Constraint: `prefixCounts` is owned by the session read goroutine and mutated without `r.mu`; any cross-goroutine read must be lock-free (atomic) or copied under a barrier.
- [ ] `docs/architecture/meta/README.md` - route-count key naming for summary/LG surfaces
  → Constraint: JSON keys are kebab-case; a new pre-policy key must not collide with `routes-received`/`routes-accepted` semantics already consumed by the LG.

### RFC Summaries
- [ ] RFC 4271 Section 3.2 (Adj-RIB-In definition) - cited in prose only
  → Constraint: Adj-RIB-In holds routes accepted by inbound policy (post-policy); the pre-policy count is a wire-level tally, not a RIB size.

**Key insights:**
- The pre-policy count already exists as `prefixCounts` and is already published to Prometheus; this spec is about safe exposure through the CLI/LG boundary, not about computing a new number.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `prefixCounts` struct (line 179) is a `map[uint32]int64` per family; `add()` (line 196) mutates it with no lock; `checkPrefixLimits()` (line 225) counts NLRI straight from the wire UPDATE via `countBodyNLRI`/`countPrefixEntries` BEFORE any import filter, so the tally is pre-policy; `setPrefixCountMetric()` (line 530) publishes it as the `ze_bgp_prefix_count` gauge.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `reactorAPIAdapter.Peers()` (line 89) builds the `[]plugin.PeerInfo` snapshot under `a.r.mu.RLock()` (line 90); it cannot safely read `prefixCounts` because the session write path holds no such lock.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - the import filter reject gate (line 448) returns before caching/dispatching a rejected route, which is why the Adj-RIB-In size is post-policy.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `status()` (lines 709-714) sets each peer's `route-counts.in` from `peerRIB.Len()`/`FamilyLen()`, i.e. the Adj-RIB-In (accepted) size.
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` maps birdwatcher `routes_received` and `routes_imported` from the same accepted number.

**Behavior to preserve:**
- `routes-accepted` / `routes-imported` MUST remain the Adj-RIB-In size (post-policy). This spec ADDS a pre-policy signal; it does not change the accepted semantics.
- The `ze_bgp_prefix_count` Prometheus gauge and its labels stay as-is.
- Prefix-limit enforcement reads of `prefixCounts` on the session goroutine must not regress (no new lock on the hot path).

**Behavior to change:**
- `show bgp` per-peer rows gain a pre-policy received count, distinct from accepted, family-scopable like the existing counts.
- Birdwatcher `routes_received` is remapped to the pre-policy count so its semantics match BIRD (received = pre-import, imported = post-import).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator runs `show bgp` (optionally `show bgp <afi/safi>`) over the CLI, or the Looking Glass calls the birdwatcher protocols endpoint.
- Format at entry: text command dispatched to the peer summary handler; JSON envelope back.

### Transformation Path
1. Session read goroutine tallies pre-policy prefixes per family in `prefixCounts` (`session_prefix.go`), lock-free.
2. A new lock-free snapshot (per-family atomic loads) publishes the current pre-policy count where the reactor peer-snapshot API can read it without racing the writer.
3. `reactorAPIAdapter.Peers()` (`reactor_api.go`) includes the pre-policy count in each `plugin.PeerInfo`.
4. The peer summary handler merges the pre-policy count into each summary row alongside the RIB-sourced accepted/sent counts.
5. `transformProtocols` (`lg/handler_api.go`) maps `routes_received` from the pre-policy count and `routes_imported` from the accepted count.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session goroutine <-> reactor API | lock-free atomic snapshot, no shared map read under a foreign lock | [ ] |
| Reactor <-> peer-summary plugin | `plugin.PeerInfo` field, then merged into the summary row | [ ] |
| Summary JSON <-> Looking Glass | kebab-case route-count keys read by `transformProtocols` | [ ] |

### Integration Points
- `plugin.PeerInfo` (`internal/component/plugin/types_bgp.go`) - carries the new pre-policy count field to consumers.
- `internal/component/bgp/plugins/cmd/peer/summary.go` - merges the count into the row (same shape as the existing route-counts merge).

### Architectural Verification
- [ ] No bypassed layers (pre-policy count flows session -> reactor API -> summary -> LG, no direct plugin reach into the session).
- [ ] No unintended coupling (LG still consumes only JSON keys).
- [ ] No duplicated functionality (reuses `prefixCounts`, does not recompute a wire tally).
- [ ] Zero-copy preserved where applicable (atomic loads, no map copy on the hot path).
- [ ] Registration over hardcoding - the new count rides the existing `PeerInfo` and summary-merge path; no per-feature switch/case added to a core/shared struct (`ai/rules/plugins.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `prefixCounts` is genuinely pre-policy | `session_prefix.go` counts from the wire UPDATE before the reject gate at `reactor_notify.go` | The exposed number would be a duplicate of accepted, not a new signal | re-read producer + race-detector `.ci` that rejects a route via import policy and shows received > accepted | **confirmed** (2026-07-16): `session_read.go` calls `checkPrefixLimits` on the raw wire UPDATE under the comment "Check prefix limits BEFORE delivering to plugins"; plugin dispatch happens later at `session_read.go` via `onMessageReceived`, and the import-filter reject gate is downstream at `reactor_notify.go` (`if !res.accept { return false }`). The tally is strictly pre-policy. |
| A-2 | Per-family atomic snapshot removes the race | `reactor_api.go` reads under `r.mu.RLock`; writer takes no lock | Data race persists | `go test -race` on a reactor test injecting concurrent UPDATEs | unvalidated (no code written). The RACE PREMISE is confirmed: `session.go` documents `prefixCounts` as "Only accessed from session's read goroutine (no synchronization needed)" and `add()` (`session_prefix.go`) mutates the map with no lock, so a read from `Peers()` would race. |
| A-3 | LG consumers tolerate `routes_received` becoming pre-policy | `lg/handler_api.go` already reads the keys; birdwatcher semantics expect received >= imported | Downstream dashboards misreport | LG `.ci` asserting received (pre) >= imported (post) | unvalidated -- and THREATENED by R-3: for VPN/flowspec families the tally can UNDER-count, so `received >= imported` is not guaranteed for all families. |
| A-4 | The tally is an accurate count of what the peer advertised (PARTIALLY REPAIRED 2026-08-08 -- see the note under the row) | Spec Task section assumed the number only needed safe exposure | The exposed number is misleading in production; the whole spec premise collapses | read the producer's data structure and every mutation site | **BROKEN** (2026-07-16): `prefixCounts` (`session_prefix.go`) holds only `counts map[uint32]int64` + `warned map[uint32]bool` -- NO per-prefix identity -- and `add()` (line 196) does an unconditional `pc.counts[fk] += delta`. The only decrements are explicit withdrawals: `countBodyWithdrawn` (line 254) and `MP_UNREACH` (line 267); `applyPrefixDelta`/`applyPrefixCheck` are called ONLY from `checkPrefixLimits`. LSP `findReferences` on `prefixCounts.add` returns exactly 3 hits -- the definition (196:25) plus `applyPrefixDelta` (349:28) and `applyPrefixCheck` (371:28) -- so there is NO third mutation path in the codebase and nothing can decrement on implicit replace. Therefore a re-announced prefix (BGP implicit withdraw, the normal attribute-update path) increments the tally AGAIN while the Adj-RIB-In replaces in place. Under route churn `received` climbs away from reality and never recovers. Acceptable for prefix-limit enforcement (fails safe: over-counts only), NOT acceptable as an operator-facing "routes the peer sent". |


**A-4 note (2026-08-08).** The row below is the state of the WIRE TALLY, and it
still describes `count offered`, which is the default and is unchanged. It no
longer describes `count installed`: that mode holds `prefixCounts.sets`, a set
per family keyed on each NLRI's wire encoding, and `counts[fk]` is that set's
size (`applyInstalledPrefixSections`,
`internal/component/bgp/reactor/session_prefix.go`). A re-announced prefix is
already in the set, so the unbounded upward drift the row names cannot happen
under `installed`. What it counts is the prefixes ze delivered to plugins and has
not since withdrawn, taken before import policy, so it is still not the number
this spec's Task asks for.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Atomics add hot-path cost to prefix counting | benchmark regression on UPDATE throughput | keep the atomic write off the per-NLRI path; publish a per-UPDATE snapshot instead. NOTE (2026-07-16): naturally satisfied -- `applyPrefixDelta`/`applyPrefixCheck` are already called once per family-section per UPDATE (after `countBodyNLRI` tallies all NLRIs), never per-NLRI. A per-UPDATE map-copy snapshot would still allocate on the hot path and must be avoided; prefer atomic slots per family. |
| R-2 | Pre-policy vs accepted gap is unattributable without filtered tracking | operators ask "why received > accepted with no filter configured" | ship together with, or after, the Depends spec; document the gap includes rejects and malformed drops. AGGRAVATED by A-4: with drift, the gap is permanent and unexplainable even WITH filtered tracking, because most of it is not filtering at all -- it is re-announcement. |
| R-3 | The tally can UNDER-count for complex families | `received < accepted` for VPN/flowspec peers | `countPrefixEntries` (`session_prefix.go`) documents itself as possibly inaccurate for VPN/flowspec ("cannot overcount due to prefix-length advancing") -- i.e. it may undercount. Exposing it as `routes_received` breaks the birdwatcher `received >= imported` invariant (A-3) for those families. Needs per-family accuracy before exposure. |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `show bgp` after peer sends N routes, M rejected by import policy | -> | pre-policy count in `PeerInfo` merged into the summary row | `test/plugin/bgp-summary-received-prepolicy.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer advertises N prefixes, import policy rejects M (0<M<N) | `show bgp` reports received = N, accepted = N-M |
| AC-2 | Concurrent UPDATE load while `show bgp` runs | `go test -race` clean; no read of `prefixCounts` under a foreign lock |
| AC-3 | Birdwatcher protocols endpoint after the same exchange | `routes_received` = N (pre-policy), `routes_imported` = N-M (post-policy) |
| AC-4 | `show bgp <afi/safi>` with a two-family peer | pre-policy count is family-scoped, matching the existing accepted/sent scoping |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `show bgp` and sees how many routes a peer sent vs accepted | wire tally -> atomic snapshot -> PeerInfo -> summary row | `test/plugin/bgp-summary-received-prepolicy.ci` |
| 2 | Opens the Looking Glass and sees received >= imported per peer | summary JSON -> `transformProtocols` -> birdwatcher fields | `test/web/lg-received-prepolicy.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixCountsPrePolicySnapshot` | `internal/component/bgp/reactor/session_prefix_test.go` | atomic snapshot returns the pre-policy per-family tally | |
| `TestPeersIncludesPrePolicyCount` | `internal/component/bgp/reactor/reactor_api_test.go` | `Peers()` carries the pre-policy count without racing (`-race`) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pre-policy count | 0..int64 max | int64 max | N/A (clamped at 0) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-summary-received-prepolicy` | `test/plugin/bgp-summary-received-prepolicy.ci` | peer sends N routes, M rejected; summary shows received=N, accepted=N-M | |
| `lg-received-prepolicy` | `test/web/lg-received-prepolicy.ci` | birdwatcher endpoint reports received >= imported | |

### Interop Tests
- N/A for the counter surface itself; the underlying pre-policy tally is already exercised by prefix-limit interop.

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/bgp/reactor/session_prefix.go` - add a lock-free per-family snapshot of the pre-policy tally.
- `internal/component/bgp/reactor/reactor_api.go` - include the pre-policy count in the `Peers()` snapshot.
- `internal/component/plugin/types_bgp.go` - add the pre-policy count field to `PeerInfo`.
- `internal/component/bgp/plugins/cmd/peer/summary.go` - merge the count into the summary row.
- `internal/component/lg/handler_api.go` - map birdwatcher `routes_received` to the pre-policy count.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Prometheus counters/metrics | [ ] | `ze_bgp_prefix_count` already exists; no new metric required |
| Functional test for new RPC/API | [x] | `test/plugin/bgp-summary-received-prepolicy.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` (new summary column) |
| 13 | Route metadata keys added/changed? | [x] | `docs/architecture/meta/README.md` (pre-policy key) |
| 15 | Runtime inventory changed? | [ ] | No new plugin/command registered |

## Files to Create
- `test/plugin/bgp-summary-received-prepolicy.ci` - functional test for the pre-policy count.
- `test/web/lg-received-prepolicy.ci` - LG mapping test.
- `plan/learned/NNN-bgp-prepolicy-received-counter.md` - learned summary at closure.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the `PeerInfo` field and a failing `.ci` that asserts received != accepted after a rejected route.
   - Tests: `bgp-summary-received-prepolicy.ci`
   - Files: `types_bgp.go`, `summary.go`
   - Verify: field exists, `.ci` fails because the count is still equal to accepted.
2. **Phase: Lock-free snapshot** - publish the pre-policy tally without a foreign-lock read.
   - Tests: `TestPrefixCountsPrePolicySnapshot`, `-race`
   - Files: `session_prefix.go`, `reactor_api.go`
   - Verify: `-race` clean, count flows to `Peers()`.
3. **Phase: LG mapping** - remap birdwatcher `routes_received`.
   - Tests: `lg-received-prepolicy.ci`
   - Files: `lg/handler_api.go`
   - Verify: received >= imported.
4. **Functional tests** -> both `.ci` pass.
5. **Full verification** -> `make ze-precommit-verify-changed` (scope to changed while other sessions run).
6. **Complete spec** -> audit tables, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has file:line implementation |
| Correctness | pre-policy tally is read before the reject gate; accepted stays post-policy |
| Data flow | reactor never read under a foreign lock; LG reads JSON only |
| Registration over hardcoding | count rides existing PeerInfo/summary-merge path, no core switch/case added |
| Rule: no-regression | prefix-limit hot path takes no new lock |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| pre-policy field on PeerInfo | grep `types_bgp.go` |
| race-clean snapshot | `go test -race ./internal/component/bgp/reactor/...` |
| functional coverage | run `test/plugin/bgp-summary-received-prepolicy.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | count is bounded by prefix-limit enforcement; no unbounded growth |
| Error leakage | count is a plain integer; no peer-controlled string surfaced |

### Failure Routing
| Failure | Route To |
|---------|----------|
| received == accepted in `.ci` | tally is being read post-policy; re-check the snapshot source |
| `-race` failure | snapshot still reads the shared map; switch to atomics |
| 3 fix attempts fail | STOP, report, ask user |

## Known Limitations
- The received-minus-accepted gap lumps together policy rejects and malformed-route drops; it is only fully attributable once filtered-route tracking exists (Depends spec). Until then the CLI help must state the gap is "not accepted", not "filtered".
- ~~The pre-policy tally resets when the session is rebuilt (new connection), matching `prefixCounts` lifetime; it is current-state, not cumulative.~~
  **CORRECTED 2026-07-16:** the reset-on-rebuild half is right, but "current-state"
  is WRONG. Per A-4 the tally is neither current-state nor cumulative: it counts
  every announced NLRI and only decrements on EXPLICIT withdrawal, so implicit
  withdraw (re-announce) inflates it permanently. This is the blocking finding.

## Audit Findings (2026-07-16, no code written)

Facts established while auditing; carry these into any re-design.

| # | Finding | Evidence |
|---|---------|----------|
| F-1 | `prefixCounts` is allocated for EVERY session, not only when limits are configured | `session.go` initializes it unconditionally in `NewSession`. The spec's data-flow assumed nothing here, but it matters: the pre-policy count exists even for peers with no `prefix-maximum`, so AC-1 does not need a limits-configured peer. |
| F-2 | The field comment at `session.go` is STALE and contradicts the code | It reads "Initialized in NewSession when PrefixMaximum is configured"; `session.go` initializes it unconditionally. A one-line comment fix, deliberately NOT made in this session (user scoped the session to stop, no code). Worth fixing on the way past. |
| F-3 | Peer already has the exact lock-free pattern this spec proposed to invent | `peer_stats.go` `peerCounters` holds `atomic.*` fields; `Peer.Stats()` (line 87) reads them lock-free and `reactor_api.go` already calls it from `Peers()`. A per-family atomic table on `Peer`, published via a Session callback wired in `runOnce()` (the existing `prefixMetrics` / `onNotifSent` pattern, `session.go`), is the idiomatic shape -- not a new snapshot type in `session_prefix.go`. Reset would hang off `ClearStats` (`peer_run.go`). |
| F-4 | `summary.go` documents the CURRENT single-count design as deliberate | The comment states "there is no separate pre-policy received count here" and explains routes-filtered is never emitted. Any implementation must rewrite that comment, and it overlaps the Depends spec's own edit to the same function. |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes (or `make ze-precommit-verify-changed` when scoped, with rationale)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Registration over hardcoding verified

### Design
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Minimal coupling
