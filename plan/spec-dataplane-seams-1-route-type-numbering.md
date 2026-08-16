# Spec: dataplane-seams-1 -- Route Type Values Independent of Linux (Skeleton)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `plan/spec-dataplane-seams-0-umbrella.md` (finding F-1) |
| Phase | - |
| Deferral shard | `plan/deferrals/dataplane-seams.md` (create on the first deferral) |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`RouteType` in `internal/component/sysrib/events/events.go` uses the values 1,
6, 7 and 8 for unicast, blackhole, unreachable and prohibit. Its comment says
"Values match Linux RTN_ constants for direct mapping in the kernel backend".

That payload is not internal. `BestChangeBatch.MarshalJSON` exists because
external FIB plugin processes decode it. So a Linux kernel constant is currently
part of a contract offered to any dataplane backend, including ones with no
relationship to Linux.

Give `RouteType` its own numbering, and map to RTN_ inside `fibkernel` where the
kernel is actually being addressed. The mapping does not disappear; it moves to
the one place that owns it.

**Cheap now.** There is one consumer today. Each new FIB backend that copies the
numbering makes this more expensive, and the numbering is published to
out-of-tree plugin authors who cannot be migrated.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - what external plugin processes are promised
- [ ] `docs/architecture/meta/README.md` - route metadata keys and their documented values

### Related Specs
- [ ] `plan/spec-dataplane-seams-0-umbrella.md` - the parent, finding F-1
- [ ] `plan/spec-fib-depth.md` - in-progress, owns FIB programming depth

**Key insights:** (minimal context to resume after compaction)
- The change is small in code and is a published-contract change in effect. Decide the compatibility story before touching the values.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-08-07)
- [ ] `internal/component/sysrib/events/events.go` - declares `RouteType uint8` with the four constants and the RTN_ comment, and `BestChangeEntry.RouteType`

**Source files to read before design:**
- [ ] `internal/plugins/fib/kernel/` - every reader of `RouteType`, and how it reaches netlink
- [ ] `internal/plugins/fib/vpp/` - whether it reads `RouteType` at all
- [ ] `internal/component/sysrib/` - every producer that sets the field

**Behavior to preserve:**
- The externally visible JSON field name `route-type` and its `omitempty` behavior, unless the compatibility decision says otherwise.
- Route programming outcomes on both dataplanes: a blackhole route stays a blackhole route.

**Behavior to change:**
- The numeric values `RouteType` carries, and where the RTN_ mapping lives.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A route source (static, bgp, connected, an IGP) sets a route type when publishing into `sysrib`.

### Transformation Path
1. `sysrib` selects the best path and populates `BestChangeEntry.RouteType`.
2. The entry is emitted on `(system-rib, best-change)`, in-process as a Go value and out-of-process as JSON.
3. `fibkernel` translates it to a netlink route type; today the translation is the identity.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| sysrib ↔ in-process FIB plugin | Go typed payload | No |
| sysrib ↔ external FIB plugin process | JSON `route-type` field | No |
| fibkernel ↔ kernel | netlink `rtm_type` | No |

### Integration Points
- `internal/component/sysrib/events` - where the type is declared
- `internal/plugins/fib/kernel` - where the RTN_ mapping should live after this change

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
| A-1 | `fibkernel` is the only consumer that depends on the values being RTN_ numbers | The comment names only the kernel backend | An unmigrated consumer mis-programs routes | Enumerate every reader before changing values | unvalidated |
| A-2 | Out-of-tree FIB plugins either do not exist or can absorb a versioned change | Not established; the JSON contract is public | The change breaks a user with no warning | Check `docs/architecture/api/process-protocol.md` for a stated stability promise | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An external plugin silently mis-programs blackhole or prohibit routes because it kept the old numbering | Routes install as unicast when they should discard | Decide whether to version the field or keep the wire values and change only the internal constants |
| R-2 | The change is made internally but the doc still publishes RTN_ values | Docs and code disagree | Update `docs/architecture/meta/` in the same commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Blackhole, unreachable and prohibit routes program as the wrong type, which silently forwards traffic that should be dropped |
| How is it reverted? | Single-commit revert, unless an external plugin has already adopted new values |
| Who else touches this path? | `spec-fib-depth` (in-progress) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill at design) a static blackhole route in config | → | (fill at design) the `fibkernel` mapping function | (fill at design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A blackhole, unreachable or prohibit route is configured | It programs with the same effect as before the change, on both dataplanes |
| AC-2 | The route payload is inspected as an external FIB plugin process sees it | The route type value is one this repository defines and documents, not a Linux kernel constant |
| AC-3 | The Linux mapping is looked for | It exists exactly once, inside `fibkernel` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill at design) | `internal/plugins/fib/kernel/*_test.go` | Each route type maps to the correct RTN_ value | |
| (fill at design) | `internal/component/sysrib/events/*_test.go` | The JSON encoding of each route type | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `RouteType` | (fill at design) | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `002-fib-route` (existing, must not regress) | `test/vpp/002-fib-route.ci` | Routes still program on VPP | |
| `001-boot-apply`, `002-reload-add` (existing, must not regress) | `test/static/001-boot-apply.ci`, `test/static/002-reload-add.ci` | Static routes still program on the kernel | |
| new: discard route round trip | `test/static/*.ci` | A configured blackhole route drops traffic, and the route type an external plugin sees is the documented one | |

## Files to Modify
- `internal/component/sysrib/events/events.go` - the `RouteType` constants and their comment
- `internal/plugins/fib/kernel/` - add the RTN_ mapping (exact file at design)
- `docs/architecture/meta/README.md` - published value list

## Files to Create
- `test/static/*.ci` - the discard-route functional test (name at design)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface changes; route types are already configurable |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/static/*.ci`, named above |
| Pipe completeness | N-A | No new CLI output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | Behavior is unchanged for the operator |
| 2 | Config syntax changed? | N-A | No syntax change |
| 3 | CLI command added/changed? | N-A | No command change |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if the route payload is documented there |
| 5 | Plugin added/changed? | N-A | No plugin added |
| 6 | Has a user guide page? | N-A | Not user-facing |
| 7 | Wire format changed? | N-A | No protocol wire format changes; this is the plugin payload, row 8 |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC obligation touched |
| 10 | Test infrastructure changed? | N-A | Uses existing `.ci` infrastructure |
| 11 | Affects daemon comparison? | N-A | No externally comparable behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` if it describes the route payload |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md` |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | The event type is unchanged; only a field's values change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on `sysrib/events/events.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check documented route-payload examples for hardcoded values |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the failing functional test that reads the route type as an external plugin process sees it.
2. **Phase: decide compatibility** -- version the field, or keep the wire values and change only the internal constants. This decision comes before any renumbering.
3. **Phase: renumber and map** -- give `RouteType` its own values and add the RTN_ mapping inside `fibkernel`.
4. **Phase: docs** -- update the published value list in the same commit.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every reader of `RouteType` migrated, none left assuming RTN_ |
| Correctness | The mapping is total: every declared route type has a kernel value, and an unknown value is rejected rather than defaulted to unicast |
| Data flow | The RTN_ mapping exists in exactly one place |
| Rule: `ai/rules/evidence.md` | The mapping fails closed. An unmapped type must not silently become unicast |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No RTN_ reference outside `fibkernel` | `grep -rn "RTN_" internal/ --include=*.go` |
| Published values match code | Compare `docs/architecture/meta/README.md` against the constants |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed mapping | An unrecognized route type must be rejected, never defaulted to unicast. A discard route that silently forwards is a traffic leak |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Out-of-tree FIB plugin authors cannot be migrated by this repository. The compatibility decision in step 2 is what protects them.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
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
