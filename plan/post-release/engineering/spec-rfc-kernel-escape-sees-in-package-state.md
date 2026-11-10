# Spec: rfc-kernel-escape-sees-in-package-state

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Development tooling: the kernel-producer escape gate lets an in-package unit take the escape without naming the Ze state it installs; it does not change the shipped binary |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Scope is tooling, so End-to-End User Stories, Interop Tests and RFC
Documentation do not apply.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`kernelZeState` (`internal/le/rfc/discriminate_escape_kernel.go`) refuses a
kernel-producer escape whose unit reaches Ze code that no `ze-state-excluded`
entry answers for. It decides which Ze code the unit reaches from the packages
the unit's file IMPORTS (`kernelZePackages`). A test in the same package as the
producer imports nothing of its own package, so its own package is never asked:
`TestXFRMTunnelPMTUAgesBackToTheLinkMTU`
(`internal/component/ike/dataplane/rfc4301_pmtu_integration_linux_test.go`)
installs SA and policy state through `xfrmBackend.InstallSA` and `InstallPolicy`
and could take the escape with no exclusion, while an exclusion naming
`internal/component/ike/dataplane` is refused as "a package the unit does not
import". Goal: count the unit file's own directory as reached when it is in the
module and outside `kernelTestTree`, so the unit must name the in-package
producers it excludes, and prove it with a fixture test.

Found by the independent test judge on spec
`plan/release/fixes/spec-rfc4301-architecture-gaps.md` (2026-11-10).

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the kernel-producer escape and its refusals
- [ ] `ai/doors/le-tooling.md` - le boundaries and gates

**Key insights:**
- To be written at design.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/discriminate_escape_kernel.go` - `kernelZeState` builds `reached` from `goImportPaths` of the unit file through `kernelZePackages`, refuses an exclusion whose producer directory is not in `reached`, and refuses a reached directory no exclusion names

**Behavior to preserve:**
- An imported Ze package still owes an exclusion or the mutant route
- Test machinery under `kernelTestTree` is never asked

**Behavior to change:**
- The unit file's own package directory is part of `reached`

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Reached-package set | `internal/le/rfc/discriminate_escape_kernel.go` `kernelZePackages` | Repository-relative directories of the in-module imports of the unit file | Extends: adds the unit file's own directory under the same in-module and `kernelTestTree` rules |
| Exclusion check | same file, `kernelZeState` | Matches each `ze-state-excluded` producer directory against the reached set, both ways | Does not change: it reads the extended set |
| Patched-kernel route | `plan/post-release/kernel/spec-rfc-kernel-patch-discrimination-route.md` | Plans proving escaped rows by a kernel patch | Does not apply: that spec proves rows later; this one fixes which Ze code the escape must answer for now |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc record ... route no-break reason kernel-producer`, and `./le rfc check` re-verifying a stored record

### Transformation Path
1. `kernelZeState` reads the unit file and its imports
2. `kernelZePackages` answers the reached set
3. Exclusions are matched against it

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Record file to gate | `rfc/discrimination/<stem>/<id>.json` read from disk | No |

### Integration Points
- To be written at design.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Existing kernel-producer records come from external test packages or name no in-package state | none checked | Records already stored go red under `./le rfc check` and need exclusions | List every stored record with `reason kernel-producer` and its unit's package | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Stored records turn red when the set widens | `./le rfc check` names a kernel-producer record | Add the exclusions in the same commit, each judged |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | RFC records refused or silently accepted; nothing user-visible |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/post-release/kernel/spec-rfc-kernel-patch-discrimination-route.md` (same escape) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A kernel-producer record whose unit is an in-package test with no exclusion | → | `kernelZeState`, `kernelZePackages` | `TestKernelEscapeAsksTheUnitsOwnPackage` (to be written) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An in-package unit takes the kernel-producer escape with no exclusion | The gate refuses it, naming the unit's own package |
| AC-2 | The same record names an exclusion whose producer sits in the unit's own package | The gate accepts it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestKernelEscapeAsksTheUnitsOwnPackage` | `internal/le/rfc/discriminate_escape_kernel_test.go` | AC-1 and AC-2 over a fixture tree | |

## Files to Modify
- `internal/le/rfc/discriminate_escape_kernel.go` - `kernelZePackages` or `kernelZeState` adds the unit file's own directory to the reached set
- `docs/contributing/rfc-conformance-gates.md` - the escape's reached-package rule names the unit's own package

## Implementation Steps
1. Write `TestKernelEscapeAsksTheUnitsOwnPackage` and see it red
2. Extend the reached set
3. Run `./le rfc check` and add a judged exclusion to every stored record it now refuses (R-1)

## Checklist
- [ ] Spec template sections completed at design
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le test unit` scoped to `./internal/le/rfc/` (`TestKernelEscapeAsksTheUnitsOwnPackage`) passes, then `./le rfc check`

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/check-cannot-see-the-change-it-looks-for.md` | 2026-11-10 | `kernelZeState` misses an in-package unit's own directory |
