# Spec: fixit-ping-monitor-cadence

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/commands.md` - streaming command surface
4. `internal/component/ping/cmd/stream.go` (the whole file, ~145 lines), `internal/component/ping/cmd/ping.go` `doPingCtx`

## Task

`monitor ping <dest> interval 1s timeout 5s` does not probe every second when
the path is lossy. It probes every ~5s. The advertised `interval` silently
degrades to roughly `max(interval, timeout)` exactly when the network is bad,
which is precisely when an operator is watching.

`streamPing` (`internal/component/ping/cmd/stream.go:70-141`) is strictly serial
per probe: build, `WriteTo` (`:82`), then **block in `ReadFrom` until a matching
reply or the deadline** (`:92-117`), then emit, then sleep the remainder of the
interval (`:133-140`). The read deadline is `start.Add(timeout)` (`:78`). A lost
packet therefore costs the full `timeout` before the next probe can be sent, and
the trailing sleep only ever subtracts what the probe already consumed
(`interval - elapsed`), so it cannot claw back the deficit.

Second, related defect: `:102` discards any reply whose `replySeq` is not the
CURRENT `seq`. Once the loop is a probe behind, a late reply for seq N-1 arrives
while the loop waits on seq N, fails the match, and is dropped. A path that is
recovering reports worse loss than it has.

Goal: probe cadence stays at `interval` regardless of loss, and a late reply is
attributed to the probe it answers instead of being discarded.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/api/commands.md` - the streaming command surface this feeds
  → Constraint: `NewPingSession` is consumed by three callers through a channel; the fix must not change that contract.
- [ ] `ai/rules/qemu-testing.md` - linux-only runtime code needs a QEMU proof
  → Constraint: raw ICMP needs `CAP_NET_RAW`. Every claim in this spec is read from source, NOT observed (A-1). A privileged run is what turns it into evidence.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc792.md` - ICMP Echo/Echo Reply identifier and sequence semantics
  → Constraint: RFC 792 defines the Identifier and Sequence Number fields used to match a reply to its request. Matching by sequence is the mechanism this spec relies on; it is already how `:100-102` identifies replies, just scoped to one in-flight probe.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- The display/networking split ALREADY exists and is fine: `NewPingSession` (`stream.go:28-40`) runs `streamPing` on its own goroutine behind a 64-slot buffered channel. Do not "fix" that.
- The coupling that hurts is send<->receive inside `streamPing`. That is what this spec separates.
- This is a concurrency change to code that is trivially correct today. The bar is a real race-detector run, not a plausible design.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/ping/cmd/stream.go` - `NewPingSession` (:28) resolves the target, makes `ch := make(chan map[string]any, 64)` (:34), spawns `streamPing` (:37), returns the channel + cancel.
  → Decision: the 64-slot buffer means a slow display cannot stall the prober for 64 replies. The networking/display split is already correct; this spec does not touch it.
  → Constraint: `streamPing` (:42) does `defer close(out)`; the channel closing is how every consumer detects the end. Any redesign must still close exactly once.
  → Constraint: `conn` is opened at `:55` with `lc.ListenPacket(ctx, network, "")` and closed by `defer` (:59). A receiver goroutine parked in `ReadFrom` is unblocked by that Close; teardown ordering is load-bearing (R-2).
  → Constraint: the loop condition `count <= 0 || seq < count` (:70) and the last-probe early return (:129) implement `count`. Both disappear in a ticker design and must be re-expressed as "all sent AND all resolved-or-expired".
- [ ] `internal/component/ping/cmd/ping.go` - `doPingCtx` ~~(:230)~~ (:242 — corrected 2026-07-17, verified against source) is the batch engine behind `show ping`.
  → Decision: SAME serial structure (send :282, block in `ReadFrom` :288-296, next seq). But `doPing` has NO interval at all -- it sends the next probe as soon as the previous resolves. Its worst case is bounded (`count * timeout`), so it is NOT this bug and is OUT OF SCOPE. See Known Limitations.
  → Constraint: `pingPayload` (:229) is shared by both engines. Do not fork it.
- [ ] `internal/component/ping/cmd/register.go` - `monitorPingLocal` consumes with `for result := range ch`.
- [ ] `internal/component/cli/model_ping.go` - the TUI consumes by polling every `pingDrainInterval` (50ms) via `tea.Tick`, and `PingFactory` (:26) is the contract.
  → Constraint: `cli` deliberately never imports `ping/cmd`; the factory passes primitives. Keep that inversion.

**Behavior to preserve:** (unless user explicitly said to change)
- `NewPingSession`'s signature and channel contract: three callers depend on it (`register.go`, `cmd/ze/hub/session_factory.go`, `internal/component/cli/client/main.go`).
- The per-reply map shape `{"seq":N,"status":"ok"|"timeout","rtt-ms":F}`. `applyPingReply` and `pingReplyToJSON` (cli) parse it; `test/ui/monitor-ping-pipe-resolve-log.ci` asserts on it.
- `count == 0` streams until the context is canceled.
- Replies from an address other than the target are ignored (`:105-111`).
- The channel is closed exactly once when the session ends.

**Behavior to change:** (only if user explicitly requested)
- Probe sends become timer-driven rather than reply-driven.
- A reply is matched to its own `seq`, not only to the newest in-flight one.
- A probe is reported `timeout` when ITS deadline passes, not when the loop gets round to it.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `monitor ping <dest> [interval] [timeout] [count] [size]`, via `monitorPingLocal` (offline) or the TUI's `PingFactory` (daemon).
- Format at entry: parsed args (`parseMonitorPingArgs` / `parsePingMonitorArgs`).

### Transformation Path
1. `NewPingSession` (`stream.go:28`) resolves target -> `netip.Addr`, creates the buffered channel
2. `streamPing` goroutine: ICMP socket, per-probe send/receive loop
3. Per-reply `map[string]any` onto the channel
4. Consumer renders: `monitorPingLocal` line-by-line, or the TUI's `applyPingReply` stats

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ consumer | buffered `chan map[string]any`, closed on end | [ ] |
| Sender ↔ receiver (NEW) | shared in-flight map keyed by seq, mutex-guarded | [ ] |
| Process ↔ kernel | `net.ListenConfig.ListenPacket` raw ICMP; needs `CAP_NET_RAW` | [ ] |

### Integration Points
- `probe.BuildICMPEcho` / `probe.ResolveTarget` (`internal/core/probe`)
- `pingPayload` (`ping.go:229`), shared with the batch engine

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — no new per-feature field, switch case, or factory in a core/shared package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The cadence degradation is real, not just apparent in the source | Read from `stream.go:78,92-117,133-140`. **NEVER OBSERVED** -- raw ICMP needs `CAP_NET_RAW`, unavailable on the dev host, so `streamPing` has never been executed in this analysis | The whole spec is unnecessary | privileged/QEMU run: `monitor ping <blackholed> interval 1s timeout 5s`, timestamp the sends, assert ~1s spacing (currently expect ~5s) | **unvalidated — DO THIS FIRST** |
| A-2 | Late replies are being dropped, inflating reported loss | `:102` `replySeq != uint16(seq&0xffff)` -> `continue`, and the loop only ever waits on the current seq | The matching half of the fix is unnecessary; cadence alone would do | same run: induce delay > interval, assert the reply is reported rather than lost | unvalidated |
| A-3 | One socket can serve concurrent `WriteTo` and `ReadFrom` from two goroutines | Go `net.PacketConn` is documented safe for concurrent use | Needs a single owning goroutine with an internal send queue instead | `go test -race` with both goroutines live | unvalidated |
| A-4 | `conn.Close()` reliably unblocks a receiver parked in `ReadFrom` | Go closes the fd; `ReadFrom` returns an error | The receiver leaks on teardown | `go test -race` + a goroutine-leak assertion after cancel | unvalidated |
| A-5 | No consumer depends on replies arriving in seq order | `applyPingReply` accumulates stats order-independently; `monitorPingLocal` prints per-line | Out-of-order output confuses the TUI stats or a `.ci` assertion | read both consumers; run `test/ui/monitor-ping-pipe-resolve-log.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The rewrite introduces a data race in code that is trivially correct today | `go test -race` failure; flaky `.ci` | Race-detector run is an AC, not a nicety. This repo's memory records that replacing sleeps with real sync is exactly where its live races surfaced |
| R-2 | Teardown deadlock/leak: receiver parked in `ReadFrom` when the context is canceled | test hangs; goroutine count grows | Close the conn to unblock the read; assert no goroutine leak after cancel |
| R-3 | Double-close of the reply channel once two goroutines can end the session | panic: close of closed channel | Exactly one owner closes `out`; the others only signal |
| R-4 | `count` completion becomes subtly wrong: the session must end when all probes are sent AND resolved-or-expired, not when the last is sent | a bounded run exits before reporting its last reply, or hangs | Explicit AC (AC-4, AC-5) covering both edges |
| R-5 | Fixing cadence makes a lossy path emit far more probes than before (1/s instead of 1/5s), changing load | none locally | This is the intended behavior; note it in the commit body |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `monitor ping <dest> interval 1s timeout 5s` under loss | → | sender goroutine on a ticker | `TestStreamPingCadenceHoldsUnderLoss` |
| a reply arriving after its interval | → | seq-keyed in-flight matching | `TestStreamPingMatchesLateReply` |
| `monitor ping <dest> count 5` | → | completion when all resolved-or-expired | `TestStreamPingCountCompletesAfterLastReply` |
| context cancel mid-flight | → | teardown, no leak, single close | `TestStreamPingCancelClosesOnce` |
| `monitor ping` end-to-end, privileged | → | the real ICMP path | `test/ping/monitor-ping-cadence.ci` (QEMU/privileged) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `interval 1s timeout 5s`, target black-holed (100% loss) | Probes are SENT ~1s apart. Today: ~5s apart |
| AC-2 | `interval 1s timeout 5s`, 50% loss | Send cadence stays ~1s; lost probes report `timeout` at their own deadline, not the loop's convenience |
| AC-3 | reply for seq N arrives after probe N+1 was sent | Reported as `ok` for seq N with its true RTT, not dropped (today: dropped at `:102`) |
| AC-4 | `count 5` | Session ends after the 5th probe RESOLVES (reply or timeout), and every one of the 5 appears on the channel |
| AC-5 | `count 5 interval 30s` | No trailing idle after the last reply (the `:129` guard's behavior must survive the rewrite) |
| AC-6 | context canceled mid-flight | Both goroutines exit, channel closed exactly once, no goroutine leak |
| AC-7 | `go test -race` over the new session | No race reported |
| AC-8 | existing consumers | `monitorPingLocal` and the TUI render unchanged; reply map shape identical |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Watches `monitor ping` on a lossy link and reads the loss % | args -> NewPingSession -> sender/receiver -> channel -> TUI stats | `test/ping/monitor-ping-cadence.ci` |
| 2 | Runs `monitor ping <dest> count 5` offline and gets 5 lines | args -> monitorPingLocal -> channel -> stdout | `TestStreamPingCountCompletesAfterLastReply` |
| 3 | Ctrl-C's a running monitor | signal -> context cancel -> teardown | `TestStreamPingCancelClosesOnce` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStreamPingCadenceHoldsUnderLoss` | `internal/component/ping/cmd/stream_test.go` | AC-1, AC-2 | |
| `TestStreamPingMatchesLateReply` | `internal/component/ping/cmd/stream_test.go` | AC-3 | |
| `TestStreamPingCountCompletesAfterLastReply` | `internal/component/ping/cmd/stream_test.go` | AC-4 | |
| `TestStreamPingCountNoTrailingIdle` | `internal/component/ping/cmd/stream_test.go` | AC-5 | |
| `TestStreamPingCancelClosesOnce` | `internal/component/ping/cmd/stream_test.go` | AC-6, AC-7 | |

<!-- TESTABILITY IS THE FIRST PROBLEM. streamPing opens a real raw socket
     (stream.go:55), so none of the above can run unprivileged today, which is
     why this bug survived. Phase 1 exists to fix that: put the send/receive
     behind a seam (an interface with WriteTo/ReadFrom/SetDeadline, satisfied by
     net.PacketConn) so a fake can drive loss, delay and reordering
     deterministically with a fake clock. Without the seam these tests are
     sleep-based and flaky, and this spec makes things worse rather than better. -->

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| interval | 100ms-30s | 30s | 99ms | 31s |
| timeout | 1s-30s | 30s | 999ms | 31s |
| count | 0 (unbounded) or 1-100 | 100 | N/A | 101 |
| size | 1-65507 | 65507 | 0 | 65508 |

<!-- Bounds are already enforced and tested by both parsers; listed here because
     the rewrite must not change them. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `monitor-ping-cadence` | `test/ping/monitor-ping-cadence.ci` | operator watches a lossy target and sees probes keep pacing at `interval` | |
| `monitor-ping-pipe-resolve-log` | `test/ui/monitor-ping-pipe-resolve-log.ci` | existing test must still pass (AC-8 regression guard) | exists |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | ICMP echo against a host, no peer daemon negotiates anything. RFC 792 conformance is covered by the functional test | |

## Files to Modify
- `internal/component/ping/cmd/stream.go` - the sender/receiver split (the whole change)
- `internal/component/ping/cmd/stream_test.go` - new; needs the socket seam to exist first
- `docs/guide/command-reference.md` - if the cadence-under-loss behavior is worth stating for operators

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no - no new argument | - |
| CLI commands/flags | [ ] no - same surface | - |
| Functional test for new RPC/API | [ ] yes | `test/ping/monitor-ping-cadence.ci` |
| Doctor check for runtime dependencies | [ ] no - CAP_NET_RAW already surfaced by the ping error path | - |
| Prometheus counters/metrics | [ ] no | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] no - a fix; behavior matches what the docs already imply | - |
| 3 | CLI command added/changed? | [ ] no - no argument change | - |
| 12 | Internal architecture changed? | [ ] yes - the session becomes two goroutines | `docs/architecture/api/commands.md` if it describes the session |
| 16 | Any changed source file referenced by doc source anchors? | [ ] check | grep `docs/` for `stream.go`, `NewPingSession` |

## Files to Create
- `internal/component/ping/cmd/stream_test.go` - unit tests over the socket seam
- `test/ping/monitor-ping-cadence.ci` - privileged end-to-end cadence proof

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` + `go test -race` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary; two-commit closure |

### Implementation Phases

<!-- Phase 0 is not optional. A-1 is unvalidated: if the degradation is not real,
     phases 1-4 are wasted work on trivially-correct code. -->

0. **Phase: PROVE THE BUG (A-1, A-2)** — before writing any production code
   - Tests: none yet; a privileged manual run is enough
   - Verify: with `CAP_NET_RAW` (QEMU or a privileged host), run `monitor ping <blackholed> interval 1s timeout 5s` and timestamp the sends. Expect ~5s spacing today. If it is ~1s, STOP: A-1 is broken and this spec is void
1. **Phase: Wiring / testability seam (MANDATORY FIRST)** — make the bug testable
   - Tests: the `stream_test.go` set, failing against today's serial loop
   - Files: `stream.go` (extract a small conn interface: `WriteTo`/`ReadFrom`/`SetDeadline`/`Close`, satisfied by `net.PacketConn`; inject a clock)
   - Verify: a fake conn can inject loss, delay and reordering with no root and no sleeps; tests fail for the RIGHT reason (cadence, not compile)
2. **Phase: Sender/receiver split** — the fix
   - Tests: `TestStreamPingCadenceHoldsUnderLoss`, `TestStreamPingMatchesLateReply`
   - Files: `stream.go` -- sender on `time.Ticker(interval)` recording `seq -> sent-at`; receiver looping `ReadFrom`, matching by seq; reaper expiring past `timeout`
   - Verify: cadence holds under injected loss; a late reply is attributed to its seq
3. **Phase: count + teardown** — the edges most likely to break (R-3, R-4)
   - Tests: `TestStreamPingCountCompletesAfterLastReply`, `TestStreamPingCountNoTrailingIdle`, `TestStreamPingCancelClosesOnce`
   - Files: `stream.go`
   - Verify: bounded runs report every probe then end; cancel closes once, no leak
4. **Phase: race + functional** — `go test -race`, then the privileged `.ci`
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit tables + learned summary; two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Exactly one goroutine closes `out` (R-3); the in-flight map is mutex-guarded everywhere |
| Data flow | `NewPingSession`'s signature and the reply map shape are unchanged (AC-8) |
| Concurrency | `go test -race` green; no goroutine leak after cancel; no send on a closed channel |
| Rule: no-layering | The old serial loop is fully deleted, not left behind a flag |
| Rule: no-workarounds | No `time.Sleep` in the tests; the seam makes them deterministic |
| Scope | `doPingCtx` is NOT touched (see Known Limitations) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Cadence holds under loss | `go test ./internal/component/ping/cmd -run TestStreamPingCadence` |
| No race | `go test -race ./internal/component/ping/cmd` |
| Consumers unaffected | `bin/ze-test bgp plugin show-ping` + `test/ui/monitor-ping-pipe-resolve-log.ci` |
| End-to-end | `test/ping/monitor-ping-cadence.ci` under QEMU |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Replies come from the network. The receiver must keep the existing checks: length (`:97`), type (`:97`), id (`:100-102`), and source address (`:105-111`). A seq-keyed map must not let a forged seq resurrect an expired probe |
| Resource exhaustion | The in-flight map is bounded by `interval`/`timeout` (at most `timeout/interval` entries) for unbounded runs. Assert that bound; an unbounded map fed by a hostile responder is a leak |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Phase 0 shows cadence is already ~interval | STOP. A-1 broken; close the spec as void with the evidence |
| Race detector fires | Fix the sharing, not the test |
| `.ci` flaky | The seam is wrong; go back to Phase 1 rather than adding sleeps |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The bug is invisible on a healthy path. `interval` and `timeout` only diverge
  when replies are late or lost, so every happy-path test and every local run
  looks perfect. The failure mode is reserved for the exact conditions the tool
  exists to diagnose.
- Untestable code stays broken. `streamPing` opens a real raw socket, so no unit
  test could reach it and none exists; the hook even flags "no test file:
  stream_test.go". The seam in Phase 1 is the actual deliverable -- the fix is
  small once the code can be driven.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Split sender from receiver | Keep the serial loop, shorten the read deadline to `min(timeout, interval)` | The shortcut breaks `timeout > interval` outright: a reply arriving at 2s with `interval 1s` would be declared lost. It trades a cadence bug for a correctness bug |
| Keep display/networking as-is | Also restructure the consumer side | That split already exists and works (`stream.go:34,37`). Widening scope into the TUI adds risk with no benefit |
| Phase 0 proves the bug before any code | Start with the rewrite | A-1 is read from source and never executed. Rewriting correct concurrent code on an unverified premise is how a fix becomes a regression |
| `doPingCtx` out of scope | Fix both engines together | `show ping` has no interval to violate; its worst case is bounded. Different bug, different spec |

## Known Limitations
- `doPingCtx` ~~(`ping.go:274`)~~ (`ping.go:242` — corrected 2026-07-17, verified against source) shares the serial structure and is deliberately NOT
  fixed here. It has no inter-probe pacing at all: it sends the next probe as
  soon as the previous resolves, so `show ping <blackholed> count 100 timeout
  30s` takes ~50 minutes. Bounded and operator-initiated, so it is a separate
  (real) issue worth its own spec, not scope creep in this one.
- Cadence cannot be proven on the dev host: raw ICMP needs `CAP_NET_RAW`. The
  unit tests prove it against a fake conn; only the QEMU `.ci` proves the real
  socket path.

## RFC Documentation

Add `// RFC 792: "<quoted requirement>"` above the Identifier/Sequence matching
in the receiver, which is the mechanism the fix depends on.

## Implementation Summary

### What Was Implemented
- (not started)

### Bugs Found/Fixed
- (not started)

### Documentation Updates
- (not started)

### Deviations from Plan
- (not started)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Cadence stays at `interval` under loss | functional test (privileged) | `test/ping/monitor-ping-cadence.ci` (not written) |
| Late replies are attributed, not dropped | unit test over the seam | `TestStreamPingMatchesLateReply` (not written) |
| No regression in concurrency | race detector | `go test -race ./internal/component/ping/cmd` (not run) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (not started)

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

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-fixit-ping-monitor-cadence.md`
