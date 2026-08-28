# Spec: interop-image-tag-race -- concurrent interop runs overwrite each other's daemon image

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The interop harness namespaces its CONTAINERS per run and its IMAGES not at all.
`_SUFFIX` (`test/interop/interop.py`) is the process id, and every container name
derives from it, so two concurrent runs never collide on a container, a network,
or a lab address. `build_images` (`test/interop/run.py`) then builds a bare,
shared tag: `ze-interop`.

**So two concurrent runs race on the image, and the loser runs the winner's
daemon.** The second build overwrites the tag between the first run's build and
its container start, and nothing downstream can tell.

**Measured on 2026-08-05, twice, during the round 2 review of
`spec-wire-edit-4-api-origin-deferred-bird-interop` (closed 2026-08-07 in `2cc75ab5f`):**

| What happened | Consequence |
|---------------|-------------|
| A reviewer ran a mutation build to prove an interop scenario discriminates. Its run PASSED | The exact inversion of the expected result. Another session's build had landed between build and container start, so the containers ran an unmutated daemon |
| A second reviewer's runs silently used the first reviewer's mutated image | Its measurements described a daemon it had not built |
| The mutated daemon stayed in the shared tag after the source tree was clean | Any later `NO_BUILD=1` run would have reported a false RED on code that was correct |

The first row is the dangerous one. **A false GREEN from a mutation run is how a
vacuous test gets certified as discriminating** (`ai/rules/interop-and-goal-validation.md`
requires reverting the behavior and confirming the test fails; that procedure is
only as trustworthy as the guarantee that the container ran the reverted build).

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `test/interop/run.py` - `build_images`, and the `NO_BUILD=1` path
- [ ] `test/interop/interop.py` - `_SUFFIX` and every `ze-interop` reference
- [ ] `ai/rules/interop-and-goal-validation.md` - why mutation evidence must name its binary

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `test/interop/run.py` (`build_images` builds the bare `ze-interop` tag)
- [ ] `test/interop/interop.py` (containers carry `_SUFFIX`; the image reference does not, at six call sites)

**Behavior to preserve:** `NO_BUILD=1` must keep working. It exists so a developer
can iterate on a scenario without paying the image build, and it depends on a tag
that survives between runs. A per-run tag naively applied would break it, which is
the design question this spec exists to answer.

## Data Flow (MANDATORY)

### Entry Point
`python3 test/interop/run.py [scenario]`, or `./le integration interop`.

### Transformation Path
(fill during design)

### Boundaries Crossed
| From | To | Format |
|------|----|--------|
| (fill during design) | (fill during design) | (fill during design) |

### Integration Points
| Point | Component |
|-------|-----------|
| (fill during design) | (fill during design) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A per-run image tag is compatible with `NO_BUILD=1`, given a documented way to name the tag to reuse. | `build_images` returns early under `NO_BUILD=1`, so the tag it would have written is the tag the run then uses. | The fix breaks the iterate-fast path and gets reverted. | Read `build_images` and every `ze-interop` reference, then run both paths. | unvalidated |
| A-2 | Nightly CI runs one interop suite at a time, so CI is not currently mis-reporting. | `.github/workflows/evidence-nightly.yml` | The nightly evidence tier has been reading unknown binaries. | Read the workflow. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A per-run tag leaks images, filling the disk on a dev machine. | `docker images` grows without bound. | Remove the run's image at teardown, as `Scenario.teardown` already does for containers. |
| R-2 | The fix silently changes what CI builds. | The nightly job starts failing at setup. | Land the tag change and the workflow read together. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Two interop runs start concurrently | -> | the image naming in `build_images` and the run's container start | a test asserting two runs cannot share an image reference, named at design time |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two `run.py` invocations overlap | Each run's containers run the binary that run built. Neither can observe the other's |
| AC-2 | `NO_BUILD=1` | Still works, and the tag it reuses is documented and discoverable |
| AC-3 | A scenario run records its evidence | The image identity is recoverable from the run output, so a mutation result names the binary it measured |
| AC-4 | A run finishes | It leaves no unbounded image residue (R-1) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | `test/interop/run_test.go` | AC-1 | |

### Functional Tests
<!-- Tooling scope: no daemon Go changes, so the driving surface is the runner. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| two concurrent `test/interop/run.py` invocations, each with a distinguishable daemon build | `test/interop/` | overlapping runs do not share a daemon image, and each run's result describes the binary it built | |
| `test/interop/run.py` under `NO_BUILD=1` | `test/interop/` | the iterate-fast path still finds an image to run (AC-2) | |

## Files to Modify
- `test/interop/run.py` - `build_images` and the tag it writes
- `test/interop/interop.py` - the six `ze-interop` references
- `docs/architecture/testing/interop.md` - the harness contract, including how to name the tag for `NO_BUILD=1`

## Files to Create
- (fill during design)

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | **Yes** | `docs/architecture/testing/interop.md`, and `docs/functional-tests.md` if the invocation changes |
| 1-9, 11-17 | - | N-A | No daemon, user, config, CLI, API, plugin, wire or RFC surface is touched |

## Implementation Steps

1. (fill during design)

## Known Limitations

- Scope is the image tag. Other shared-state hazards in the harness, if any, are
  separate work.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `./le verify current mode full` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated, not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)
