# Spec: isis-spf-lsp-timers

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/isis/spf/computer.go` - the SPF debounce
4. `internal/plugins/isis/spf_wiring.go` - where the Computer is constructed
5. `internal/plugins/ospf/yang/ze-ospf-conf.yang` - the OSPF SPF-timer precedent to mirror

## Task

Ze's IS-IS lets an operator configure LSP lifetime and LSP refresh interval, but
NOT the two throttles that control convergence timing under churn:
- The SPF run is debounced by a single hardcoded 200ms window; there is no
  configurable initial delay, hold floor, or maximum hold (no exponential
  back-off), so a flapping topology cannot be damped and a stable topology
  cannot converge faster than 200ms.
- There is no LSP-generation throttle (a minimum interval between successive
  re-originations of the router's own LSP); origination is event-driven with
  only a refresh-interval coalescing guard.

Sibling OSPF already exposes configurable SPF timers (`spf-delay-ms`,
`spf-hold-ms`, `spf-max-hold-ms`). This is a parity gap. Add to IS-IS:
- Configurable SPF throttle (initial delay, hold floor, maximum hold) mirroring
  the OSPF leaves and back-off shape, wired through the SPF Computer.
- A configurable `lsp-gen-interval` LSP-generation throttle (default disabled so
  current behaviour is preserved).

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/isis/spf/computer.go` - `DefaultDebounce` and `NewComputer`; the single debounce timer.
  -> Constraint: the Computer already exposes a `Debounce` config field that the engine never sets; extend it to a delay/hold/max back-off rather than adding a parallel timer.
- [ ] `internal/plugins/isis/spf_wiring.go` - `initSPF` constructs the Computer without any timer config.
  -> Constraint: this is the seam that must pass the configured throttle into the Computer.
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` and `internal/plugins/ospf/config.go` - the OSPF `spf-delay-ms`/`spf-hold-ms`/`spf-max-hold-ms` leaves + parse.
  -> Decision: mirror the OSPF leaf names, units (ms), and back-off semantics for cross-protocol consistency.
- [ ] `ai/rules/config.md` - kebab-case leaf naming.
  -> Constraint: reuse the OSPF names so operators see one convention.

**Key insights:**
- The SPF Computer already has an unused `Debounce` hook, so wiring is mostly plumbing plus the back-off math.
- `lsp-gen-interval` default 0 (disabled) keeps today's origination timing; setting it only adds a floor between generations.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/isis/spf/computer.go` - `DefaultDebounce = 200 * time.Millisecond` (computer.go); `NewComputer` sets `deb := cfg.Debounce; if deb <= 0 { deb = DefaultDebounce }` (computer.go). A single flat debounce, no delay/hold/max back-off.
- [ ] `internal/plugins/isis/spf_wiring.go` - `initSPF` (spf_wiring.go) builds `spf.NewComputer(spf.Config{Source, Resolver, Levels, Installer, ResolverV6, InstallerV6})` (spf_wiring.go); it never sets `Debounce`, so the hardcoded 200ms is always used.
- [ ] `internal/plugins/isis/config.go` - `LSPLifetime` / `LSPRefreshInterval` are parsed (config.go, config.go) into the config struct (config.go); there is no SPF-timer or `lsp-gen-interval` field.
- [ ] `internal/plugins/isis/yang/ze-isis-conf.yang` - `lsp-lifetime` (yang:48) and `lsp-refresh-interval` (yang:55) exist; there is no `spf-*` or `lsp-gen-interval` leaf.

**Behavior to preserve:**
- LSP lifetime and refresh-interval config behave exactly as today.
- With no SPF-timer config, convergence is at least as fast as today (the 200ms flat debounce becomes the default hold floor).
- With `lsp-gen-interval` unset (0), own-LSP origination timing is unchanged.
- The Loc-RIB install path and the L1/L2 SPF results are unchanged; only WHEN SPF runs changes.

**Behavior to change:**
- The SPF throttle is configurable (initial delay, hold floor, maximum hold) with an exponential back-off under repeated triggers, mirroring OSPF.
- A configurable minimum interval between successive own-LSP generations can be set.

## Data Flow (MANDATORY)

### Entry Point
- Config: new IS-IS leaves `spf-delay-ms`, `spf-hold-ms`, `spf-max-hold-ms`, and `lsp-gen-interval`.

### Transformation Path
1. Config parse loads the four leaves into the IS-IS config struct (with defaults).
2. `initSPF` passes the SPF throttle values into `spf.Config` when constructing the Computer.
3. The Computer arms its debounce as an initial delay that grows toward the maximum hold under repeated LSDB changes and relaxes back to the delay when stable.
4. The own-LSP origination path enforces the `lsp-gen-interval` floor between successive regenerations (0 = no floor).
5. SPF and origination otherwise proceed exactly as today; only their timing changes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> IS-IS config | four timer leaves parsed into the struct | [ ] |
| IS-IS config <-> SPF Computer | throttle passed via `spf.Config` at `initSPF` | [ ] |
| IS-IS config <-> LSP origination | `lsp-gen-interval` floor applied between generations | [ ] |

### Integration Points
- `internal/plugins/isis/yang/ze-isis-conf.yang` - the four new leaves.
- `internal/plugins/isis/config.go` - parse + struct fields + defaults.
- `internal/plugins/isis/spf/computer.go` - back-off using the passed throttle.
- `internal/plugins/isis/spf_wiring.go` - pass the throttle into `spf.Config`.
- IS-IS own-LSP origination path - the generation floor.

### Architectural Verification
- [ ] No bypassed layers (throttle flows config -> struct -> spf.Config -> Computer)
- [ ] No unintended coupling (timers live in IS-IS config + Computer, not a global)
- [ ] No duplicated functionality (extend the existing `Debounce` hook, do not add a parallel timer)
- [ ] Registration over hardcoding - the 200ms constant becomes a config-driven default, not a hardcoded window.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Computer's single debounce can become a delay/hold/max back-off without reworking SPF | computer.go already has a `Debounce` hook | a larger Computer change is needed | prototype the back-off in a unit test | unvalidated |
| A-2 | The OSPF back-off semantics transfer to IS-IS unchanged | ospf spf-timer leaves + parse | IS-IS needs different bounds | compare against the OSPF throttle during design | unvalidated |
| A-3 | An own-LSP generation floor can be added without starving legitimate updates | IS-IS origination is event-driven | a floor delays a critical LSP | default 0 (off); cap the floor; test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A too-large SPF hold delays convergence on a real change | slow failover in test | sane default (delay <= today's 200ms); max-hold only under repeated churn |
| R-2 | `lsp-gen-interval` floor hides a genuine topology change | stale LSDB on a peer | default off; keep the floor small; never delay a purge |
| R-3 | Divergence from OSPF timer semantics confuses operators | inconsistent behaviour across protocols | mirror OSPF leaf names + back-off shape |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| set `spf-delay-ms`/`spf-hold-ms`/`spf-max-hold-ms` | -> | the Computer uses the configured throttle | `TestISISSPFThrottleConfigured` |
| repeated LSDB changes | -> | the hold grows toward max-hold (back-off) | `TestISISSPFBackoffGrows` |
| set `lsp-gen-interval` | -> | successive own-LSP generations respect the floor | `TestISISLSPGenIntervalFloor` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `spf-delay-ms` set | the first SPF after a change waits the configured initial delay |
| AC-2 | rapid repeated changes | the SPF hold backs off toward `spf-max-hold-ms` |
| AC-3 | topology stabilises | the hold relaxes back toward the initial delay |
| AC-4 | no SPF-timer config | convergence is at least as fast as today's 200ms |
| AC-5 | `lsp-gen-interval` set | own-LSP regenerations are spaced by at least the floor |
| AC-6 | `lsp-gen-interval` unset (0) | origination timing unchanged from today |
| AC-7 | existing lsp-lifetime / refresh config | unchanged behaviour |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | tunes IS-IS convergence to damp a flapping link | config timers -> SPF Computer back-off -> fewer SPF runs under churn | `test/qemu/isis-spf-timers.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestISISSPFThrottleConfigured` | `internal/plugins/isis/spf/computer_test.go` | configured delay/hold/max used instead of the 200ms default | |
| `TestISISSPFBackoffGrows` | `internal/plugins/isis/spf/computer_test.go` | hold grows toward max under repeated triggers, relaxes when stable | |
| `TestISISLSPGenIntervalFloor` | `internal/plugins/isis/config_test.go` | generation floor spacing enforced; 0 disables | |
| `TestISISTimerLeavesParse` | `internal/plugins/isis/config_test.go` | the four leaves parse into the config struct with defaults | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| spf-delay-ms | 0..600000 | 600000 | n/a | 600001 |
| spf-hold-ms | 0..600000 | 600000 | n/a | 600001 |
| spf-max-hold-ms | 0..600000 | 600000 | n/a | 600001 |
| lsp-gen-interval (ms) | 0..65535 | 65535 | n/a | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-spf-timers` | `test/qemu/isis-spf-timers.ci` | configured timers change convergence timing on a real adjacency | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - local scheduling behaviour; the wire is unchanged, so it is validated by unit + QEMU, not a peer | - | - | timers change WHEN SPF/origination run, not the packets | - |

### Future (if deferring any tests)
- If the full back-off proves large, phase in a single configurable debounce first, then the delay/hold/max shape.

## Files to Modify
- `internal/plugins/isis/yang/ze-isis-conf.yang` - add `spf-delay-ms`, `spf-hold-ms`, `spf-max-hold-ms`, `lsp-gen-interval`
- `internal/plugins/isis/config.go` - parse + struct fields + defaults
- `internal/plugins/isis/spf/computer.go` - back-off using the passed throttle
- `internal/plugins/isis/spf_wiring.go` - pass the throttle into `spf.Config`

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/qemu/isis-spf-timers.ci` - functional test
- (unit tests in existing `_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the four leaves (parsed but unused) + a failing `test/qemu/isis-spf-timers.ci`.
2. **Phase: Config plumbing** - parse into the struct with defaults; pass through `spf.Config` at `initSPF`.
   - Tests: `TestISISTimerLeavesParse`, `TestISISSPFThrottleConfigured`
3. **Phase: SPF back-off** - grow/relax the hold in the Computer.
   - Tests: `TestISISSPFBackoffGrows`
4. **Phase: LSP-gen floor** - enforce the generation interval.
   - Tests: `TestISISLSPGenIntervalFloor`
5. **Functional (QEMU)** - timers change convergence on a real adjacency.
6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | default convergence not slower than today; purge never delayed |
| Naming | leaves mirror the OSPF `spf-*-ms` names |
| Registration over hardcoding | the 200ms constant becomes a config default |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 demonstrated
- [ ] End-to-End User Stories: working path + passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (the four timers)
- [ ] Functional tests for end-to-end behavior (QEMU)
