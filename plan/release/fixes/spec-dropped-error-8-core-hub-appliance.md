# Spec: dropped-error-8-core-hub-appliance

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | 9 errors in the DNS server core, the managed client's doctor check, the self-update history and the on-device installer's DHCP fallback are caught and logged only below WARN: a DNS listener that fails to close after a bind error, a self-update history that fails to save, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Doors: `ai/doors/core.md`,
`ai/doors/hub-engine.md`, `ai/doors/appliance.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/core/`, `pkg/zefs/`,
the hub-engine door's paths (`cmd/ze`, `internal/component/{engine,hub,managed}`,
`pkg/fleet`) and the appliance door's paths (`internal/appliance`,
`internal/component/gokrazy`, `internal/install`, `cmd/ze-installer`,
`cmd/ze-serial-shell`, `internal/component/config/system`). At HEAD on
2026-11-02: **9 instances in 7 files** (`internal/core/dnsserver` 6,
`internal/component/managed` 1, `internal/component/config/system` 1,
`internal/install/disk` 1).

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid. The managed doctor check `hubReachable` reports the failure in its result, since `ze doctor` exists to show it |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`) |
| 3 | Going on is right and nobody can act (closing a socket after the bind already failed and was returned) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/core.md`
  → Constraint: `internal/core` imports no component; a WARN uses the logger the core package already takes
- [ ] `ai/doors/hub-engine.md`
  → Constraint: the managed client and hub contract is `pkg/fleet`; a doctor result changes no envelope
- [ ] `ai/doors/appliance.md`
  → Constraint: the initrd installer is one static binary with no external tools; a WARN there goes to its console log, and PID 1 cannot exit
  → Constraint: an image in the field is the contract; the self-update history path does not change
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- `Manager.bind` closes the UDP socket after the TCP bind fails and returns the TCP error; the dropped close error is answer 3 (the bind error is already returned).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/core/dnsserver/manager.go` - `Manager.bind`: the UDP close after a TCP bind failure is logged at Debug and the TCP error returned

**Behavior to preserve:**
- DNS wire behavior, the `pkg/fleet` contract, the installer's PID 1 behavior and the update paths a device in the field uses.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 9 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| Doctor checks | `internal/component/managed/doctor.go` `hubReachable`, `internal/core/diagnostic` | Report a runtime dependency to `ze doctor` | Extends: the check reports the failure it drops today |
| Self-update backend | `internal/component/config/system/selfupdate.go` (`docs/architecture/appliance/self-update.md`) | Applies an update and records history | Applies: a history save failure is answer 2 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed close, send, bind, doctor probe, history save or DHCP fallback.

### Transformation Path
1. The branch catches the error.
2. It returns it (the doctor check reports it), WARNs, or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon ↔ operator | Log at WARN, `ze doctor` | No |

### Integration Points
- The package loggers and doctor checks above.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Existing paths |
| No unintended coupling (components stay isolated) | Yes | No new import; core stays a leaf |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new reporting path |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control paths |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 9 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | unvalidated |
| A-2 | The DoH handler and `send` failures are per request | `handler.go` `send`, `secure.go` `dohHandler` | A per-request WARN floods | Read each; count or WARN once per condition | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per DNS query under a client that drops connections | DNS `.ci` log grows per query | Count per query, WARN once per condition |
| R-2 | The doctor check now reports failure in a test environment with no hub | A managed `.ci` turns red | The report is the truth; the `.ci` asserts it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, or a doctor check that newly reports a failure |
| How is it reverted? | Single commit revert |
| Who else touches this path? | DNS and appliance specs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to this child's paths: zero findings of the shape |
| `ze doctor` with an unreachable managed hub | → | `hubReachable` | `TestManagedDoctorReportsUnreachableHub` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | `ze doctor` with an unreachable hub | The managed check reports the failure with its fix |
| AC-3 | Each other answer-1 instance | A unit test where the caller observes the error |
| AC-4 | Each file with an answer-2 WARN | A unit test asserts one WARN line names what, why and the fix |
| AC-5 | Each annotation | Names a guard the file can see |
| AC-6 | Package tests and the DNS, managed and installer `.ci` the change reaches | Green; installer logic through `./le test unit installer` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestManagedDoctorReportsUnreachableHub` | `internal/component/managed/doctor_test.go` | AC-2 | |
| One test per other answer-1 or answer-2 instance | The package of the instance | AC-3, AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `.ci` the change reaches (DNS, managed, installer) | `test/` | Unchanged behavior; the doctor reports the condition | |

## Files to Modify
- `internal/component/config/system/selfupdate.go` - 1: `selfUpdater.saveHistory` 1020
- `internal/component/managed/doctor.go` - 1: `hubReachable` 105
- `internal/core/dnsserver/connlimit.go` - 1: `connLimitListener.Accept` 71
- `internal/core/dnsserver/handler.go` - 1: `send` 138
- `internal/core/dnsserver/manager.go` - 3: `Manager.bind` 221; `Manager.Stop` 326,333
- `internal/core/dnsserver/secure.go` - 1: `Manager.dohHandler.func` 623
- `internal/install/disk/network.go` - 1: `fallbackDHCP` 106

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- Test additions in the packages above; no new source file.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | Yes | The existing `hubReachable` check reports its failure; no new check |
| Prometheus counters/metrics | Yes, if answer 2 counts | The DNS server's existing families; name each counter used at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/architecture/dns/server-harness.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `ai/rules/repo-maintenance.md`, `docs/architecture/appliance/on-device-installer.md`, `docs/architecture/appliance/self-update.md`, `docs/architecture/dns/secure-transports.md`, `docs/architecture/dns/server-harness.md`. A log level changes none of them; a doctor result change is checked with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-8-core-hub-appliance.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; write the managed doctor test red.
2. **Phase: Classify and resolve** -- every instance gets its answer and reason in Design Insights; tests first for answers 1 and 2; annotations last.
3. **Phase: Prove** -- AC-1 to AC-6.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | The installer never exits PID 1 on a newly returned error |
| Rule: user-facing-errors | Each WARN and doctor result names the operation, why and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le job run label core command go test -race ./internal/core/dnsserver/ ./internal/component/managed/ ./internal/component/config/system/`, and `./le test unit installer` |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | A WARN that prints the managed hub token or update credentials |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One child for three small doors | One child per door | 9 instances; three doors read once is the cheaper split |
| The umbrella's three answers, in order | Raise every line to WARN | A WARN does not make a default correct |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-8-core-hub-appliance.md` only, in the same `./le commit create` script
