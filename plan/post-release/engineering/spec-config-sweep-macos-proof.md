# Spec: config-sweep-macos-proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Post-release by the owner's standing rule: "Production is Linux. A check, test or defect that exists only on macOS waits until after the first release." (`ai/META-DECISIONS.md`, Goal and scope). Both items below run only on macOS |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two executions that only a macOS host can perform were left unproven when
`spec-config-validate-every-repo-config` closed on Linux:

| # | Item | Why only macOS |
|---|------|----------------|
| 1 | `TestRepositoryConfigsValidate` (`internal/component/config/cli/validate_corpus_test.go`) under the full feature-tag set on macOS, reporting zero findings, and its log naming each host-verdict code it skipped with counts | On Linux no host-verdict skip occurs, so the darwin skip counts and the claim that every darwin-only finding is a host verdict are unobserved. The Linux run injects a plan9 schema (`TestSweepDerivesEachExclusion`, host verdict 11), which is not a macOS execution |
| 2 | `test ui doctor-bgp-md5` | The `.ci` declares `skip-os=linux,freebsd`, so it never runs on the Linux host |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/le-tooling.md`
  -> Constraint: a gate whose verdict depends on the host is a known defect class (le-tooling door, `plan/journal/gate-verdict-depends-on-the-machine.md`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/cli/validate_corpus_test.go` - `isHostVerdict` reads `diagnostic.Lookup(code).HostVerdict`; `internal/core/diagnostic/codes.go` marks `config-kernel-capability`, `config-node-other-os` and `config-host-backend`

**Behavior to preserve:** the Linux sweep result (zero findings, no host-verdict skip).

**Behavior to change:** none planned; this spec records two macOS runs and fixes what they find.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | This spec |
|-----------|-------|-----------|
| Host-verdict codes | `internal/core/diagnostic/codes.go`, read by `isHostVerdict` in `validate_corpus_test.go` | Runs unchanged on macOS |
| Scoped package test | `./le job run label <l> command go test -tags <full set> -run '^TestRepositoryConfigsValidate$' -v ./internal/component/config/cli/` (`docs/contributing/running-commands.md`) | The proof route on the Mac |
| UI functional suite | `./le test ui doctor-bgp-md5` | The proof route on the Mac |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The two commands above, on a macOS host.

### Transformation Path
1. The sweep reads the repository configs and validates each through `runValidation`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None new | - | - |

### Integration Points
- None new.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No new code planned | Yes | Task |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Repository files on macOS | → | `TestRepositoryConfigsValidate` | itself |
| `ze doctor` on macOS | → | BGP MD5 doctor check | `test/ui/doctor-bgp-md5.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Item 1 on a macOS host | Zero findings; the summary line and each host-verdict count are recorded. Any finding is fixed at its producer, never excused |
| AC-2 | Item 2 on a macOS host | PASS with no skip, recorded |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRepositoryConfigsValidate` | `internal/component/config/cli/validate_corpus_test.go` | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-bgp-md5` | `test/ui/` | AC-2 | |

## Files to Modify
- None planned. A finding the macOS runs expose names its producer, and the fix lands under that producer.

## Implementation Steps
1. Run AC-1 and AC-2 on a macOS host and record both results here.
2. Fix at its producer any finding either run exposes.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
