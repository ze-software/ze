# Spec: exabgp-compat-allowance-checks-every-frame

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | ExaBGP compatibility evidence: an allowed `exabgp-compat` case reports SKIP after checking only the frames up to its first divergence, so a regression in any later frame of the same case reads as the allowed difference |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Goal: an allowed `exabgp-compat` case still checks every frame after the
allowed divergence.

Found while closing `spec-config-validate-every-repo-config` AC-12: the judge
of `test/exabgp-compat/old-4.2/conf-flow.ci` and `old-5.0/conf-flow.ci`
(88e7a340ab) observed that both cases are allowed on frame 1 of 11, and that a
mutation of any of frames 2..11 still reads SKIP. The mock returns at the
first frame it cannot match, the run fails with that frame's text, the
`option=allowed-error:` text matches it, and the case is recorded SKIP with
the ten later frames never compared.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the harness and the mock live under `internal/le` and `internal/test`
  -> Constraint: a `.ci` that names a case the harness cannot judge fails loudly; it never vanishes (`docs/architecture/testing/ci-format.md`, "An unparseable test file fails")
- [ ] `docs/architecture/testing/interop.md` - the `exabgp-compat` mock and its case format
- [ ] `docs/architecture/config/exabgp-syntax.md` - the documented differences an allowance names (redirect and copy, traffic class above 63, flow label width)

**Key insights:**
- The mock already has a mode that keeps matching after a mismatch: the recorder (`-s` save) continues past a divergent frame and counts it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/bgp/exabgp_server.go` - `serveExaBGPConnection`: in the frame loop, `found < 0` returns "unexpected message = <hex>; N expected frames remain (first: <hex>)" unless `recorder.capturing()`, in which case it calls `recorder.mismatch()` and continues
- [ ] `internal/le/interoplab/bgp/exabgp_case.go` - `readExaBGPCase` reads `option=allowed:` and `option=allowed-error:`
- [ ] `internal/le/interoplab/bgp/exabgp_frame_equal.go` - `bgpFrameEqual`, the frame comparison
- [ ] `internal/test/cli/cmd_exabgp_old.go` - a failed allowed case is SKIP when `exabgpFailureCarries` finds the allowed-error text in the verdict or either process's output; a passing allowed case fails, asking for the allowance to be deleted

**Behavior to preserve:**
- An allowed case whose only differences are the allowed ones is SKIP, with the allowance text as its reason
- An allowed case that passes fails, naming the allowance line to delete
- An allowed case that fails with any other text fails

**Behavior to change:**
- After an allowed divergence the mock keeps comparing the frames that follow, and a later frame that matches no expected frame fails the case even though the first divergence was allowed

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Allowance options | `internal/test/cli/cmd_exabgp.go` `exabgpAllowedOption`, `exabgpAllowedErrorOption`; `exabgp_case.go` `readExaBGPCase` | Declare why a case differs from the ExaBGP release and the failure text that difference produces | Extends: the allowance names the divergent frames it excuses, so the rest stay checked |
| Allowed-failure judgement | `internal/test/cli/cmd_exabgp_old.go` `exabgpFailureCarries` | Matches the allowed text anywhere in the verdict or process output | Extends: the verdict must also carry no unexcused frame |
| Frame comparison | `internal/le/interoplab/bgp/exabgp_frame_equal.go` `bgpFrameEqual` | Compares an actual frame with an expected one | Does not change: the same comparison judges the later frames |
| Save mode | `serveExaBGPConnection` `recorder.capturing()` / `recorder.mismatch()` (`-s`) | Keeps reading after a mismatch and counts the mismatches | Extends: the allowed path continues the same way instead of returning |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test exabgp` over `test/exabgp-compat/old-*/` cases carrying `option=allowed:`

### Transformation Path
1. `readExaBGPCase` reads the expected frames and the allowance
2. `serveExaBGPConnection` matches each frame the speaker sends
3. An excused divergence is recorded and matching continues; an unexcused one fails the case
4. `cmd_exabgp_old.go` records SKIP only when every divergence was excused

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| mock to harness | the mock's verdict and output, read by `exabgpFailureCarries` | No |

### Integration Points
- `serveExaBGPConnection` - the `found < 0` arm
- `exabgpFailureCarries` - the allowed verdict

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, test tooling |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Frames after an allowed divergence still arrive in the recorded order | the save mode reads them all | A later frame reads as a mismatch for an order reason | Run conf-flow with the change | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Other allowed cases carry later differences nobody saw | cases that were SKIP go red | Each is a real difference: document it and name it in its allowance, judged by `ze-test-judge` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `exabgp-compat` verdicts only |
| How is it reverted? | Per-commit revert |
| Who else touches this path? | Specs that add allowances to `test/exabgp-compat/old-*` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test exabgp` on an allowed case | → | `serveExaBGPConnection` continuing past an excused frame | a unit test in `internal/le/interoplab/bgp` driving a case with an excused frame 1 and a mutated frame 5 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `test/exabgp-compat/old-4.2/conf-flow.ci` and `old-5.0/conf-flow.ci` as committed | SKIP with their allowance, after comparing all 11 frames |
| AC-2 | The same cases with one of frames 2..11 mutated (in the speaker's output or in the expected recording) | FAIL, naming the mutated frame |
| AC-3 | An allowed case whose allowed divergence is gone | FAIL, asking for the allowance to be deleted (unchanged) |
| AC-4 | Every other allowed `exabgp-compat` case | Still SKIP, or a newly visible difference is documented and named in its allowance under `ze-test-judge` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a test driving an excused first frame and a mutated later frame | `internal/le/interoplab/bgp/exabgp_server_test.go` | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `conf-flow` | `test/exabgp-compat/old-4.2/`, `old-5.0/` | AC-1, AC-2 | |

## Files to Modify
- `internal/le/interoplab/bgp/exabgp_server.go` - continue past an excused divergence
- `internal/le/interoplab/bgp/exabgp_case.go` - the allowance names the frames it excuses
- `internal/test/cli/cmd_exabgp_old.go` - SKIP only when every divergence was excused
- `docs/architecture/testing/interop.md` - the allowance grammar

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | test tooling |
| CLI commands/flags | No | none |
| Functional test for new RPC/API | N-A | no RPC |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |

## Implementation Steps

1. Unit test red: an excused frame 1 and a mutated frame 5 read SKIP today
2. The allowance names its frames; the mock continues past an excused one
3. conf-flow red with a late mutation, then green as committed
4. Run every allowed `exabgp-compat` case

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | A divergence the allowance does not name fails the case |
| Rule: testing | Every allowance change carries a `ze-test-judge` verdict |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Late-frame mutation fails | conf-flow run with a mutated frame |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A, test tooling |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The save mode already proves the mock can read past a mismatch; the allowed path returns where the save mode continues.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `green-that-could-not-have-been-red.md` | 2026-11-01 | exabgp-compat allowed case stops comparing at its first divergent frame |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The allowance names the frames it excuses | Allow any text match, as today | A text match excuses the whole rest of the case |

## Known Limitations
- none

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] The targeted tests pass: the scoped package tests of `internal/le/interoplab/bgp` and `internal/test/cli`, and `./le test exabgp`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows
- [ ] **Commit B:** `remove plan/release/validation/spec-exabgp-compat-allowance-checks-every-frame.md` only
