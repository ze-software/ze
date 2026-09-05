# Spec: a persisted DDoS baseline of any age restores as fully mature

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A persisted DDoS traffic baseline restores as fully mature whatever its age, and
its age is not merely unchecked: it is not recoverable.

Found by an ad-hoc audit of `ddos detect` on 2026-08-15, in the source spec's own
words:

**"A persisted baseline of any age restores as fully mature."** `persistedBaseline`
(`internal/plugins/ddos/detect/persist.go`) is `{Version, Pps, Bps}` and
`baselineState` (`baseline.go`) is `{Samples, Count, P99Cache}`. No timestamp is
stored anywhere, and `statestore.Put` takes bytes with no mtime, so the age is not
merely unchecked, it is not recoverable. `loadBaselines` validates presence, JSON
and `Version == 1`; `(*baseline).restore` validates sample count and rejects NaN,
Inf and negative values. Neither considers age. An appliance powered off for a
month restores a month-old traffic profile and treats it as current, so the first
tick after boot is judged against a threshold derived from traffic patterns that
no longer hold, in either direction: a stale-low p99 false-positives, a stale-high
p99 blinds.

The consequence is a detector that is confidently wrong on the first tick after a
long outage, and that reports itself ready. `(*detector).restore` logs "baseline
restored from disk" with the ready flags, so an operator reading the log is told
the detector is warm.

Two decisions belong to this spec. The first is what to store, since nothing
today can answer "how old is this". The second is what to do with an old
baseline: reject it and warm fresh over `BaselineWindow`, or restore it with the
readiness flag held down until enough live samples have replaced the stored ones.
The window that counts as stale is the third, and it is an operator-visible
number.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/plugins.md` - the DDoS detect plugin and its configuration
  → Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- The blocker is the STORED SHAPE: no clock value exists to compare against
- `statestore.Put` takes bytes and keeps no mtime, so the store cannot answer the age either

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ddos/detect/persist.go` - `persistedBaseline` is `{Version, Pps, Bps}` with `baselineStateVersion` fixed at 1. `saveBaselines` marshals it and writes the bytes. `loadBaselines` returns `false` for an absent key, a JSON error, and a version mismatch. Nothing writes or reads a time
- [ ] `internal/plugins/ddos/detect/baseline.go` - `(*baseline).restore` rejects a sample count below `min(minRestoreSamples, b.window)`, rejects NaN, Inf and negative samples, and rejects a NaN, Inf or negative `P99Cache`. It then trims the samples to the window, rebuilds the ring oldest-first, sets `next` to 0, and calls `recalc`. It reads no clock
- [ ] `internal/plugins/ddos/detect/detector.go` - `(*detector).restore` calls `loadBaselines`, hands `blob.Pps` and `blob.Bps` to the two baselines, and logs "ddos-detect: baseline restored from disk" with `pps-ready` and `bps-ready` when either half restored. `(*detector).Stop` calls `saveBaseline` on both reconfigure and shutdown

**Behavior to preserve:**
- The existing validation: version, sample count, NaN, Inf and negative rejection
- Persisting on reconfigure, so a config change keeps a warmed baseline

**Behavior to change:**
- A baseline whose age cannot be established, or is beyond the window, must not present itself as mature

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The DDoS detect plugin starts, at daemon boot or after a config reload, and `(*detector).restore` runs once after construction and before subscribing to rates.
- Format at entry: the JSON `persistedBaseline` blob read from the shared zefs state store.

### Transformation Path
1. `loadBaselines` (`persist.go`) reads the store key and unmarshals the blob
2. `(*detector).restore` (`detector.go`) hands each half to `(*baseline).restore`
3. `(*baseline).restore` (`baseline.go`) validates and rebuilds the ring, then `recalc` sets the p99
4. The first rate tick after subscription is judged against that p99

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ state store | `statestore.Put` and `Get` over bytes | No |

### Integration Points
- `(*baseline).Ready` (`baseline.go`) - the readiness flag the log line reports and the detector gates on

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The appliance clock is trustworthy at the moment `restore` runs | [fill at design time] | A boot before NTP settles reads a wrong age and rejects a good baseline | Read the boot ordering of the plugin against time sync | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Bumping `baselineStateVersion` discards every baseline in the field on upgrade | Every appliance warms fresh after an update | Accept it once, and say so in the release note |
| R-2 | A clock that jumps backwards makes a fresh baseline look aged | Baselines rejected on every boot | Use a monotonic-safe comparison, or bound the age at zero |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The detector false-positives on legitimate traffic, or fails to see an attack, on the first ticks after boot |
| How is it reverted? | Single commit revert; a version bump also invalidates stored baselines |
| Who else touches this path? | `plan/immediate/spec-fixit-ddos-incident-lifecycle-on-teardown.md` touches `(*detector).Stop` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| plugin start with a stored baseline older than the window | → | the age check in `(*detector).restore` | [test name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A stored baseline written beyond the staleness window | The detector does not report itself ready on that baseline |
| AC-2 | A stored baseline written inside the window | The detector restores it and reports ready, as it does today |
| AC-3 | A stored blob written by the previous format, carrying no time | It is treated as of unknown age, never as current |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Powers an appliance back on after a month | boot → `(*detector).restore` → age check → warm fresh | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill at design time] | `internal/plugins/ddos/detect/persist_test.go` | the age check and its boundary | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| baseline age | 0 to the staleness window | the window | N-A | one tick past the window |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill at design time] | `test/plugin/*.ci` | An operator sees the detector warm fresh after a long outage | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is plugin, no wire-visible change | |

## Files to Modify
- `internal/plugins/ddos/detect/persist.go` - the stored shape and its validation
- `internal/plugins/ddos/detect/detector.go` - the restore decision and what the log line claims
- `internal/plugins/ddos/detect/baseline.go` - readiness after a restore, if the design holds it down

## Files to Create
- [fill at design time]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | the staleness window, if it is operator-visible |
| YANG validation constraints | | `range` on the window |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | a counter for a rejected stale baseline |
| BGP family surface (new SAFI / capability / attribute) | | N-A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/immediate/spec-fixit-ddos-baseline-restore-staleness.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- carry a time in the stored blob and read it back, write failing wiring tests
   - Tests: [wiring test names]
   - Files: `persist.go`
   - Verify: the age is recoverable; the wiring test fails because nothing acts on it
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The log line claims only what the restore actually established |
| Rule: `ai/rules/principles.md` | An unknown age is not silently read as current |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [fill at design time] | [command] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The stored time is untrusted input and needs the same NaN, range and sign checks the samples get |

### Failure Routing
| Failure | Route To |
|---------|----------|
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [fill at closure]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
