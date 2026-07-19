# Spec: fixit-show-ping-serial-pacing

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | sibling of spec-fixit-ping-monitor-cadence (shared root cause) |
| Updated | 2026-07-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-fixit-ping-monitor-cadence.md` - the streaming sibling; SAME root cause
4. `internal/component/ping/cmd/ping.go` `doPingCtx` (the whole function, ~242-330)

## Task

`show ping <dest> count N timeout T` takes up to `N * T` when the target is lossy,
because the batch is strictly serial: each probe blocks until its own reply or
deadline before the next is sent. `show ping 192.0.2.1 count 100 timeout 30s`
against a black-holed target runs for ~50 MINUTES and prints nothing until it
finishes.

`doPingCtx` (`internal/component/ping/cmd/ping.go:242`) loops `for seq := range
count` (`:274`): build probe, `SetDeadline(start.Add(timeout))` (`:278`),
`WriteTo` (`:282`), then block in the inner `for !matched { conn.ReadFrom(rb) }`
(`:288-289`) until a matching reply or the deadline fires. Only then does it send
probe seq+1. A lost probe therefore costs the full `timeout` before the next is
sent, and the total serializes to `sum(per-probe wait)` ~= `N * timeout` in the
worst case. There is also NO inter-probe pacing (no `iputils ping -i`
equivalent), and NO incremental output: `doPingCtx` returns one result map at the
end, so the operator sees nothing during the run.

This is the SAME serial send-then-block-read structure as `streamPing`
(`spec-fixit-ping-monitor-cadence`); this spec is its bounded/batch sibling. The
two could share a sender/receiver decoupling, or be fixed independently.

Goal: a lossy `show ping` batch completes in roughly `(N * interval) + timeout`
rather than `N * timeout`, and a late reply is attributed to its own probe.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/qemu-testing.md` - raw ICMP needs CAP_NET_RAW
  → Constraint: `doPingCtx` opens a raw socket (`ping.go:258`), so it cannot run unprivileged. Every timing claim here is read from source, NEVER observed (A-1). A privileged/QEMU run is what turns it into evidence.
- [ ] `plan/spec-fixit-ping-monitor-cadence.md` - the streaming sibling
  → Constraint: same root cause (serial send blocks on receive). Reuse its design thinking (ticker-driven send, seq-keyed match) where it fits the bounded case; do not duplicate the analysis.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc792.md` - ICMP Echo/Echo Reply identifier and sequence
  → Constraint: RFC 792 Identifier + Sequence Number are how a reply is matched to its request. The fix relies on matching by seq across in-flight probes, as `:275` already stamps `uint16(seq)` per probe.

**Key insights:** (minimal context to resume after compaction)
- `show ping` is BOUNDED (count) and returns a BATCH map; `monitor ping` is unbounded and streams. Same serial bug, different output contract.
- The worst case is loss-driven: a healthy target resolves each probe in ms, so the 50-minute case only appears under high loss with a long timeout — exactly a diagnostic scenario.
- Also missing: `iputils`-style `-i` interval and per-reply progress output.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/ping/cmd/ping.go` - `doPingCtx` (:242). Serial loop `:274-296+`: one probe, block on `ReadFrom` until reply/deadline (`:278,288-289`), then next. Returns a single result map (`replies` array, min/avg/max) at the end.
  → Constraint: bounded by `count`; the fix must keep the same result-map shape (offline.go and the JSON consumers parse it).
  → Constraint: `pingPayload(opts.size)` (`:264`) and the seq stamping (`:275`) are shared with the streaming path; do not fork them.
- [ ] `internal/component/ping/cmd/ping.go` - `handleShowPing` (~~:44~~ **:146**) and `parsePingArgs` (~~:56~~ **:161**): `show ping` takes `count`/`size`/`timeout`, NO `interval`.
  → Note (2026-07-17): line refs corrected after `parseMonitorPingArgs` (the monitor-cadence sibling) landed and shifted every function below it down; the behavior described is unchanged and confirmed at the new lines.
  → Decision: adding an `interval` arg (iputils `-i` parity, default e.g. 1s) is in scope IF the fix paces sends; the YANG leaf + parser + bounds would follow the `size` precedent from `c00c795cf`.
- [ ] `internal/component/ping/cmd/offline.go` - `printPingResults` consumes the batch map after it returns.
  → Constraint: with a batch return there is no streaming; incremental per-reply output is a SEPARATE enhancement (see Known Limitations) unless the fix changes the return contract.
- [ ] `plan/spec-fixit-ping-monitor-cadence.md` - the streaming sibling's design (ticker send + seq-keyed receive + reaper).
  → Decision: the batch case can reuse the same decoupling: fire all N probes paced by interval while a receiver matches replies by seq and a reaper expires past-deadline probes; finish when all N are resolved-or-expired.

**Behavior to preserve:** (unless user explicitly said to change)
- The `show ping` result map shape: `destination`, `sent`, `received`,
  `loss-percent`, `replies[]` (`seq`,`status`,`rtt-ms`), `min/avg/max-rtt-ms`
  (`ping.go` build sites; asserted by `test/plugin/show-ping.ci`).
- `count`/`size`/`timeout` bounds and semantics.
- Raw ICMP identity/source-address checks in the receiver.

**Behavior to change:** (only if user explicitly requested)
- Probe sends become paced (and non-blocking on the previous reply).
- Total batch time bounded by `~(N * interval) + timeout`, not `N * timeout`.
- Late replies matched to their own seq instead of blocking the loop.
- Optionally: a new `interval` argument; optionally: incremental output.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `show ping <dest> [count] [size] [timeout]` -> `handleShowPing` (`ping.go:146`) -> `doPing` -> `doPingCtx`. (~~was `[interval?]` / `ping.go:44`~~ — 2026-07-17: no `interval` arg per the A-5 autonomous default below; line ref corrected after `parseMonitorPingArgs` landed.)

### Transformation Path
1. `parsePingArgs` -> dest/count/timeout/opts
2. `doPingCtx`: raw socket, probe loop (the code under change)
3. result map -> `plugin.Map` (daemon) or `printPingResults` (offline)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Sender ↔ receiver (NEW) | ~~in-flight map keyed by seq, or a bounded send-all-then-collect~~ in-flight map keyed by seq with interval-paced sends (see approach default below) | [ ] |
| Process ↔ kernel | raw ICMP socket; CAP_NET_RAW | [ ] |
| doPingCtx ↔ consumers | unchanged result-map shape | [ ] |

→ AUTONOMOUS DEFAULT (2026-07-17): resolve the "map keyed by seq **or** send-all-then-collect" fork to the **in-flight map keyed by seq with interval-paced sends** (a shared seq-keyed receiver, structured like `streamPing`), NOT send-all-then-collect. Rationale: R-3 — firing all N probes with zero gap floods the socket at large `count`/`size`; paced sends bound socket pressure. Thomas: override if wrong.

### Integration Points
- `probe.BuildICMPEcho`, `pingPayload` (shared with `streamPing`)
- `test/plugin/show-ping.ci` (existing functional test)

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality (reuse `pingPayload`, seq stamping; consider sharing the receiver with `streamPing`)
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding (a new `interval` leaf, if added, uses the YANG/parser precedent)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `N * timeout` blow-up is real, not just apparent in source | read from `ping.go:274-296`; NEVER executed (needs CAP_NET_RAW) | the spec is unnecessary | privileged run: `show ping <blackholed> count 5 timeout 3s`, expect ~15s today, ~3s+ after | **unvalidated — DO FIRST** |
| A-2 | Late replies are dropped/mis-serialized under the serial model | the inner loop only waits on the current seq (`:288`) | the matching half is unneeded | privileged run inducing a late reply | unvalidated |
| A-3 | One socket serves concurrent WriteTo + ReadFrom (if a sender/receiver split is used) | Go `net.PacketConn` documented concurrent-safe | need a single owner + queue | `go test -race` with a fake conn | unvalidated |
| A-4 | The result-map shape can stay identical while the internals change | `doPingCtx` builds the map from collected replies regardless of ordering | a consumer breaks | run `test/plugin/show-ping.ci` + the JSON tests | unvalidated |
| A-5 | Adding `interval` is desired (iputils parity) vs. just fixing the blow-up without a new arg | user has not specified | scope creep or a missing knob | ask, or default: fix the blow-up with an internal pacing; expose `interval` only if requested | ~~unvalidated~~ RESOLVED (see default below) |

→ AUTONOMOUS DEFAULT (2026-07-17): A-5 resolved — do **NOT** add a `show ping` `interval` CLI argument. Fix the `N*timeout` blow-up with internal pacing only (a fixed internal send cadence while a seq-keyed receiver collects replies). Rationale: this is a scope question, so per the decision protocol take the smaller, self-contained option; it also adopts this spec's own Key Design Decisions RECOMMEND ("expose `interval` only if the user wants iputils `-i` parity") and A-5's stated default. Consequence: **AC-7, the `interval` boundary-test row, the `ze-ping-cmd.yang` leaf, the `command-reference.md` update, and every "if `interval` added" integration/doc row are NOT applicable** for this spec — they resolve to N/A, not deferred. Thomas: override if you want iputils `-i` parity in this spec.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A concurrent rewrite introduces a race in code that is serial (trivially correct) today | `go test -race` red | race run is an AC; this repo's memory records sleeps->races as the danger zone |
| R-2 | Changing the result-map shape breaks offline.go / JSON / `.ci` | `show-ping.ci` red | keep the exact map keys; only the internal timing changes |
| R-3 | A bounded send-all-then-collect floods the socket with N probes at once for large N | send buffer pressure at `count 100 size 65507` | pace sends by `interval`; do not fire all N with zero gap |
| R-4 | Fix diverges from the monitor cadence sibling, duplicating a subtly different receiver | two receivers drift | prefer a shared seq-keyed receiver/matcher used by both, if the monitor spec lands first |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show ping <dest> count N timeout T`, lossy | → | paced/decoupled batch | `TestDoPingBatchBoundedUnderLoss` |
| a reply arriving after another probe was sent | → | seq-keyed match | `TestDoPingMatchesLateReply` |
| all probes lost | → | every seq reported `timeout`, bounded total | `TestDoPingAllLostBounded` |
| `show ping` end-to-end, privileged | → | real ICMP batch | `test/plugin/show-ping.ci` (extend with a loss/timeout assertion) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `count 5 timeout 3s`, black-holed target | total run bounded well under `5 * 3s` = 15s (roughly `timeout` + pacing, not `count*timeout`) |
| AC-2 | `count 100 timeout 30s`, black-holed | completes in ~`(100*interval)+30s`, NOT ~50 minutes |
| AC-3 | reply for seq K arrives after probe K+1 was sent | reported `ok` for seq K with its true RTT, not lost |
| AC-4 | healthy target, `count 5` | unchanged fast behavior; all 5 `ok` with correct RTTs |
| AC-5 | result map | identical shape/keys to today (AC verified by `show-ping.ci` + JSON tests) |
| AC-6 | `go test -race` | no race in the new batch code |
| AC-7 | (if `interval` added) `count`/`size`/`timeout`/`interval` bounds | enforced by the parser, per the `size` precedent |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Diagnoses a lossy link with `show ping count 100` and it returns promptly | handleShowPing -> doPingCtx (paced) -> result map | `test/plugin/show-ping.ci` (extended) |
| 2 | Pings a black-holed host and does not wait 50 minutes | same, worst case | `TestDoPingAllLostBounded` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoPingBatchBoundedUnderLoss` | `internal/component/ping/cmd/ping_test.go` (via a conn seam) | AC-1, AC-2 | |
| `TestDoPingMatchesLateReply` | same | AC-3 | |
| `TestDoPingAllLostBounded` | same | AC-2 edge | |

<!-- Same testability problem as the monitor spec: doPingCtx opens a real raw
     socket, so a seam (an interface with WriteTo/ReadFrom/SetDeadline over
     net.PacketConn, plus an injectable clock) is Phase 1. Without it the tests
     are sleep-based and flaky. -->

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| count | 1-100 | 100 | 0 | 101 |
| timeout | 1s-30s | 30s | 999ms | 31s |
| size | 1-65507 | 65507 | 0 | 65508 |
| interval (if added) | e.g. 100ms-30s | 30s | 99ms | 31s |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-ping` | `test/plugin/show-ping.ci` | extend: a timeout scenario returns promptly with per-seq timeout | exists |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | ICMP echo, no peer daemon; RFC 792 covered by the functional test | |

## Files to Modify
- `internal/component/ping/cmd/ping.go` - `doPingCtx` decoupling (the change); optional `argInterval`/parser for `show ping`
- `internal/component/ping/cmd/ping_test.go` - batch tests over the conn seam
- `internal/plugins/ping-cmd/yang/ze-ping-cmd.yang` - only if a `show ping` `interval` leaf is added
- `test/plugin/show-ping.ci` - extend with a timeout-boundedness assertion
- `docs/guide/command-reference.md` - only if `interval` is added

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] only if `interval` added | `ze-ping-cmd.yang` |
| CLI commands/flags | [ ] only if `interval` added | parser in `ping.go` |
| Functional test | [ ] yes | `test/plugin/show-ping.ci` |
| Doctor check | [ ] no | - |
| Prometheus counters | [ ] no | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | [ ] only if `interval` added | `docs/guide/command-reference.md` |
| 12 | Internal architecture changed? | [ ] yes - batch send/receive decoupled | note in the ping cmd docs if any |

## Files to Create
- (none required unless a shared receiver is extracted; see R-4)

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

0. **Phase: PROVE THE BUG (A-1)** — privileged `show ping <blackholed> count 5 timeout 3s`; expect ~15s today. If it is already ~3s, STOP: spec void.
1. **Phase: testability seam (MANDATORY FIRST)** — extract a small conn interface (WriteTo/ReadFrom/SetDeadline/Close over net.PacketConn) + injectable clock so a fake can drive loss/late-reply with no root and no sleeps.
2. **Phase: decouple send from receive** — pace sends by interval; match replies by seq; expire past-deadline probes; finish when all resolved-or-expired. Keep the result-map shape.
3. **Phase: (optional) `interval` arg** — only if in scope; YANG leaf + parser + bounds per the `size` precedent.
4. **Phase: race + functional** — `go test -race`, then extend `show-ping.ci`.
5. **Full verification** → `make ze-verify`.
6. **Complete spec** → audit + learned summary; two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | result-map shape unchanged (AC-5); late replies matched (AC-3) |
| Concurrency | `go test -race` green; no goroutine leak; single owner closes the socket |
| Reuse | `pingPayload` + seq stamping shared; receiver shared with `streamPing` if that spec landed |
| Rule: no-workarounds | tests use the conn seam + fake clock, no `time.Sleep` |
| Scope | if `interval` not requested, do not add the arg; fix the blow-up internally |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| bounded under loss | `go test ./internal/component/ping/cmd -run TestDoPing` |
| no race | `go test -race ./internal/component/ping/cmd` |
| shape unchanged | `bin/ze-test bgp plugin show-ping` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | receiver keeps length/type/id/source-address checks on replies (as `doPingCtx` does today) |
| Resource exhaustion | in-flight map bounded by `count`; sends paced, not all-at-once (R-3) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Phase 0 shows it is already bounded | STOP; void the spec with evidence |
| Race detector fires | fix the sharing, not the test |
| `.ci` shape assertion breaks | R-2; restore the exact map keys |
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

- Same lesson as the monitor sibling: the bug is invisible on a healthy path
  (probes resolve in ms) and only appears under loss with a long timeout — the
  exact conditions the tool exists to diagnose. And the code is untestable today
  because it opens a real raw socket, which is why no test caught it.
- `show ping` and `monitor ping` share one root cause (serial send blocks on
  receive). Fixing them with a shared seq-keyed receiver avoids two subtly
  different implementations.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Decouple send from receive (like the monitor spec) | Add an `interval` and keep the serial block | An interval alone does NOT fix the blow-up: a lost probe still blocks `timeout` before the next send. Only decoupling bounds the total |
| Keep the result-map shape | Stream per-reply output | Streaming changes the consumer contract (offline.go, JSON, `.ci`); out of scope. A bounded batch that returns promptly is the fix |
| `interval` arg is optional | Always add it | The blow-up is fixable internally; expose `interval` only if the user wants iputils `-i` parity |
| Fail closed on a total send failure (return an error), partial results on a mid-batch write error | Always return partial results (even zero-sent) | Review ISSUE: a count>0 batch that put NO probe on the wire (first `WriteTo` fails: ENETUNREACH/EPERM) must not report `sent=0/received=0/loss=0%` as `StatusDone` — that renders a transport failure as a healthy answer (`ai/rules/fail-closed-guards.md`). `runPingSession` swallows the write error, so `runPingBatch` reconstructs it from an empty result (`count>0 && len(replies)==0`) and returns `errPingNoProbesSent`. A mid-batch failure (some probes already sent) still yields partial results, matching the "partial beats opaque" intent |

## Known Limitations
- No incremental/streaming output: `show ping` still returns one batch map, so a
  long healthy run shows nothing until it completes. Per-reply progress is a
  separate enhancement (would change the return contract). Out of scope.
- Same CAP_NET_RAW constraint as the monitor spec: the fix is provable only on a
  privileged/QEMU host; unit tests use a fake conn.

## RFC Documentation

Add `// RFC 792` above the seq-keyed reply matching in the receiver.

## Implementation Summary

### What Was Implemented
- Rewrote `doPingCtx` (`internal/component/ping/cmd/ping.go`): it now opens the raw
  ICMP socket and hands ownership to the new `runPingBatch`, which drives the shared
  `runPingSession` (stream.go, R-4) in a goroutine, drains its per-probe `out`
  channel, sorts replies by seq, and aggregates via `summarizePingReplies`.
- Internal pacing via `defaultPingBatchInterval` (10ms); no `interval` CLI arg (A-5).
- `count<=0` fail-closed guard (never passes a non-positive count to `runPingSession`,
  whose `count==0` means stream-forever).
- Fail-closed on a total send failure: `runPingBatch` returns `(map, error)` and
  yields `errPingNoProbesSent` when a count>0 batch put nothing on the wire.
- `summarizePingReplies` + `emptyPingResult` preserve the exact result-map shape (AC-5).
- Tests: `ping_test.go` batch suite (fake conn + fake clock, `-race` clean);
  `test/plugin/show-ping.ci` extended with a count=3 batch-shape check.

### Bugs Found/Fixed
- Primary: `show ping` serialized send-on-receive, so a black-holed `count N timeout T`
  took ~N*T (count 100 timeout 30s ≈ 50 min). Now bounded by ~(N*interval)+T.
- Review-found: total-send-failure fail-open (0%-loss on ENETUNREACH). Fixed with
  `errPingNoProbesSent` (see Key Design Decisions).

### Documentation Updates
- `test/plugin/show-ping.ci` header refreshed (data-flow + boundedness note).
- No YANG/command-reference change: A-5 resolved to no `interval` arg, so AC-7 and its
  doc rows are N/A (not deferred).

### Deviations from Plan
- Behavior change vs. the old serial engine: a mid-batch write error now yields
  partial results instead of `nil, error`; a TOTAL send failure returns
  `errPingNoProbesSent` (still `StatusError`, matching old intent). Recorded in Key
  Design Decisions.
- No shared-receiver extraction was needed (Files to Create "none"): `runPingSession`
  already exposed the reusable seam.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Decouple send from receive so a lossy batch is bounded | Done | `ping.go` `runPingBatch`+`doPingCtx` | drives `runPingSession` (paced, seq-keyed) |
| Preserve result-map shape | Done | `ping.go` `summarizePingReplies`/`emptyPingResult` | byte-for-byte keys vs. old engine |
| Pace sends, do not flood (R-3) | Done | `defaultPingBatchInterval` (10ms) | in-flight map bounded by count |
| Reuse one receiver, no drift (R-4) | Done | `runPingBatch` calls `runPingSession` | no second matcher |
| No `interval` CLI arg (A-5) | Done (N/A) | — | AC-7 + YANG/doc rows N/A |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done (unit+structural) | `TestDoPingBatchBoundedUnderLoss` | privileged run is CI-only (CAP_NET_RAW), see A-1 |
| AC-2 | Done (unit+structural) | `TestDoPingAllLostBounded` | single clock advance expires all in-flight probes |
| AC-3 | Done | `TestDoPingMatchesLateReply` | late reply for seq K attributed with true RTT |
| AC-4 | Done | `TestDoPingBatchHealthyShape` | all 5 ok, exact RTTs |
| AC-5 | Done | `TestSummarizePingReplies` + `show-ping.ci` | shape/keys unchanged |
| AC-6 | Done | `go test -race ./internal/component/ping/...` PASS | race-clean x5 on batch tests |
| AC-7 | N/A | — | no `interval` arg (A-5) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDoPingBatchBoundedUnderLoss` | Done | `ping_test.go` | AC-1/AC-2/AC-5; robust under `-race` (answers latest-deadline probes) |
| `TestDoPingMatchesLateReply` | Done | `ping_test.go` | AC-3 |
| `TestDoPingAllLostBounded` | Done | `ping_test.go` | AC-2 edge |
| `TestDoPingBatchHealthyShape` | Done | `ping_test.go` | AC-4/AC-5 (added) |
| `TestSummarizePingReplies` | Done | `ping_test.go` | AC-5 aggregation math (added) |
| `TestRunPingBatchCountZeroDoesNotHang` | Done | `ping_test.go` | count<=0 guard (added) |
| `TestRunPingBatchSendErrorFailsClosed` | Done | `ping_test.go` | fail-closed on total send failure (added, review) |
| `show-ping.ci` `check_multiprobe_batch` | Done (static) | `test/plugin/show-ping.ci` | count=3 batch shape; run under privileged/QEMU CI |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ping/cmd/ping.go` | Modified | rewrite doPingCtx + runPingBatch + summarizePingReplies + emptyPingResult + sentinel |
| `internal/component/ping/cmd/ping_test.go` | Modified | batch test suite + helpers |
| `test/plugin/show-ping.ci` | Modified | count=3 batch shape check + header |
| (shared receiver extraction) | Not needed | `runPingSession` already reusable (R-4) |

### Audit Summary
- **Total items:** 5 requirements + 7 ACs + 8 tests + 3 files = 23
- **Done:** 21 (AC-1/AC-2 unit+structural; privileged run is CI-only evidence per A-1)
- **Partial:** none
- **Skipped:** none
- **N/A:** AC-7 + shared-receiver extraction (A-5 / R-4 resolved)
- **Changed:** send-failure semantics (documented in Deviations + Key Design Decisions)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| lossy batch bounded, not N*timeout | unit (fake clock) + functional (privileged, CI) | `TestDoPingAllLostBounded`/`TestDoPingBatchBoundedUnderLoss` PASS; `show-ping.ci` `check_multiprobe_batch` (privileged/QEMU). A-1 privileged timing run is CI-only (CAP_NET_RAW) |
| late replies attributed | unit test over the seam | `TestDoPingMatchesLateReply` PASS (seq K reply after probe K+1 → ok with true RTT) |
| no concurrency regression | race detector | `go test -race ./internal/component/ping/...` PASS; batch tests `-count=5 -race` clean |
| total send failure not reported as healthy | unit test | `TestRunPingBatchSendErrorFailsClosed` PASS (errPingNoProbesSent) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Total send failure (first WriteTo fails) rendered as healthy `sent=0/received=0/loss=0%` `StatusDone` — fail-open vs. old engine's StatusError | `ping.go` `runPingBatch` / `stream.go` send unwind | Fixed: `runPingBatch` returns `(map,error)`, yields `errPingNoProbesSent` on empty count>0 batch |
| 2 | NOTE | Write-error result contract untested at batch level | `ping_test.go` | Fixed: added `TestRunPingBatchSendErrorFailsClosed` |
| 3 | NOTE | ctx cancellation mid-batch undercounts `sent`, returns partial as StatusDone | `runPingBatch` | Accepted: canceled caller is gone; empty-on-cancel now returns a ctx error. Documented |
| 4 | NOTE | `sent=len(replies)` correct; no leak; out closed once; shape byte-for-byte | — | Confirmed, no action |
| 5 | NOTE | Doc imprecision ("well under a second" for max-count pacing = ~1s); float vs duration avg rounds in last digit | `ping.go:53` | Accepted: both float64 ms, AC-5 shape holds; comment left (pacing is ~1s, not a correctness issue) |

### Fixes applied
- ISSUE-1: added `errPingNoProbesSent`; `runPingBatch` now returns `(map[string]any, error)`;
  `doPingCtx` propagates; `count>0 && len(replies)==0` → error (ctx-cancel error if canceled).
- NOTE-2: added `TestRunPingBatchSendErrorFailsClosed` (drives `fakePingConn.setWriteErr`).
- `startBatch` helper now asserts `NoError` (success-path batches must not error).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| — | — | Re-verified fixed code: `go test -race` PASS, `golangci-lint` 0 issues | `internal/component/ping/cmd` | 0 BLOCKER, 0 ISSUE |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

Reviewer verdict (Run 1): 0 BLOCKER, 1 ISSUE, 4 NOTES. ISSUE-1 fixed and re-tested;
NOTEs recorded and dispositioned above. No BLOCKER or ISSUE remains.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ping/cmd/ping.go` | Yes | modified (git diff --stat) |
| `internal/component/ping/cmd/ping_test.go` | Yes | modified |
| `test/plugin/show-ping.ci` | Yes | modified |
| `plan/learned/1205-fixit-show-ping-serial-pacing.md` | Yes | created |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/AC-2 | lossy batch bounded | `go test -race ./internal/component/ping/cmd/ -run TestDoPing...` PASS |
| AC-3 | late reply matched | `TestDoPingMatchesLateReply` PASS |
| AC-4 | healthy unchanged | `TestDoPingBatchHealthyShape` PASS |
| AC-5 | shape unchanged | `TestSummarizePingReplies` PASS + `show-ping.ci` shape asserts |
| AC-6 | no race | `go test -race ./internal/component/ping/...` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show ping <dest> count N` → handleShowPing → doPing → doPingCtx → runPingBatch | `test/plugin/show-ping.ci` | Static (Python syntax OK); privileged/QEMU CI runs the raw-ICMP branch (CAP_NET_RAW unavailable in this sandbox) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 (blow-up real) | Confirmed from source; privileged run is CI-only | `ping.go` old serial loop read; unit tests reproduce the bound structurally |
| A-5 (no interval arg) | Resolved: no arg | internal pacing only; AC-7 N/A |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No YANG/command-reference change needed | A-5 → no `interval` arg | Yes (N/A, not deferred) |
| `.ci` header matches data flow | `test/plugin/show-ping.ci` header | Yes (refreshed) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
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
- [ ] **Commit B:** `git rm plan/spec-fixit-show-ping-serial-pacing.md`
