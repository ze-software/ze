# Spec: fixit-show-ping-serial-pacing

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | sibling of spec-fixit-ping-monitor-cadence (shared root cause) |
| Updated | 2026-07-17 |

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
| lossy batch bounded, not N*timeout | functional test (privileged) | `test/plugin/show-ping.ci` extended (not written) |
| late replies attributed | unit test over the seam | `TestDoPingMatchesLateReply` (not written) |
| no concurrency regression | race detector | `go test -race ./internal/component/ping/cmd` (not run) |

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
