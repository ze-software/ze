# Spec: kernel-compose-make-q-assertion-is-vacuous

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/kernel-compose-make-q-assertion-is-vacuous.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/install/kernel-compose.ci` removes the built kernel image and then asserts
that `make -q` reports work to do. The assertion can never fail.

The test body is:

```sh
rm -f "$scratch/build/kernel/Image"
if PATH="$work/fakebin:$PATH" make -q -C "$scratch/tools/installer-kernel" PROFILE=compose-fixture BUILDER=docker ARCH=amd64; then
	echo "missing build/Image unexpectedly considered up to date" >&2
	exit 1
fi
```

`tools/installer-kernel/Makefile` declares `FORCE` in `.PHONY` and lists it as
the last prerequisite of `$(OUT)/Image`. A phony prerequisite is always remade,
so the target is always out of date and `make -q` exits non-zero whatever the
state of `$(OUT)/Image`. The `if` branch is unreachable, the `rm -f` before it
changes nothing, and the test would stay green with the rebuild trigger broken.

The defect predates the fixture-isolation work: `FORCE` sits on that prerequisite
line at `1acf1a627^` as well. The closing review of the
kernel-profile-fixtures-leak-into-registry spec (commit `1acf1a627`) found it.
Two independent lenses each checked that the diff lowered no assertion, and each
reported this one. That work does not depend on the assertion being real, so the
defect gets a spec rather than a fix folded into the closing commit
(`ai/rules/completion.md`).

Two candidate fixes, for the design phase to choose between:

1. **Assert on `make -n` output instead.** `make -n` prints the recipe it would
   run, so the test can assert the `run.py` invocation appears with the expected
   `--profile`. This measures what the operator cares about (a removed image is
   rebuilt) and does not depend on `-q` semantics.
2. **Drop `FORCE` from the `$(OUT)/Image` prerequisite list.** This makes `-q`
   meaningful, and it changes the product. `FORCE` is what forces a rebuild when
   `PROFILE=` or `ARCH=` changes, because a file prerequisite list cannot express
   a variable change. `$(VARIANT)` already records
   `<arch>-<profile>-<version>-<builder>`, so a design phase must show that
   record can drive the rebuild before this option is taken.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/interop-and-goal-validation.md` - "Prove the test discriminates"
  → Constraint: a test that would pass with the mechanism removed proves nothing
- [ ] `ai/rules/testing.md` - what a functional test CAN assert
- [ ] `docs/architecture/appliance/kernel-profiles.md` - the build path this test covers

**Key insights:**
- The vacuity comes from the Makefile, not from the test. Reading the test alone
  makes the assertion look correct.

## Current Behavior (MANDATORY)

Source files read:
- [ ] `tools/installer-kernel/Makefile` - `.PHONY: all clean FORCE`, `FORCE:`
  with no recipe and no prerequisites, and `$(OUT)/Image: $(RUN) ...
  kernel.config $(PROFILE_DEPS) $(COMMON_DEPS) FORCE`. The recipe writes
  `$(VARIANT)` from `$(OUT)/kernel.version` after each build.
- [ ] `test/install/kernel-compose.ci` - the `make -q` block quoted above.

### Behavior to preserve
- `make -C tools/installer-kernel PROFILE=<name> ARCH=<arch>` must still rebuild
  when the profile or the architecture changes, not only when a file is newer.
- `$(VARIANT)` keeps recording `<arch>-<profile>-<version>-<builder>` after every
  build, because `internal/appliance/cmd_iso.go` reads it.
- Every other assertion in `kernel-compose.ci` keeps proving what it proves.

## Data Flow (MANDATORY)

### Entry Point
`make -C tools/installer-kernel PROFILE=<name> BUILDER=<b> ARCH=<a>`, and the
`ze appliance kernel` path that drives the same builder.

### Transformation Path
1. Make evaluates the prerequisites of `$(OUT)/Image`.
2. `FORCE` is phony, so the target is always out of date.
3. The recipe runs `tools/kernel-builder/run.py`.
4. The recipe writes `$(VARIANT)` from `$(OUT)/kernel.version`.

### Boundaries Crossed

| From | To | Carried |
|------|----|---------|
| operator or `.ci` | `tools/installer-kernel/Makefile` | `PROFILE`, `ARCH`, `BUILDER` |
| Makefile | `tools/kernel-builder/run.py` | source dir, out dir, arch, profile, builder |
| Makefile | `$(VARIANT)` | the built `<arch>-<profile>-<version>-<builder>` record |

### Integration Points
- `internal/appliance/cmd_iso.go` reads `build/kernel/.variant` to decide whether
  a built kernel matches the requested appliance.
- `mk/appliance.mk` drives the same Makefile.

### Architectural Verification
- The fix must not make the rebuild trigger weaker than it is today. Removing
  `FORCE` with no replacement for the `PROFILE=` and `ARCH=` change trigger is a
  correctness regression, not a simplification.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | Validation | Status |
|----|------------|-------|-----------|--------|
| A-1 | `FORCE` exists to force a rebuild when `PROFILE=` or `ARCH=` changes | `$(VARIANT)` records exactly arch, profile, version and builder, and no file prerequisite can express a variable change | design phase reads the commit that added `FORCE` | unvalidated |
| A-2 | No other test depends on `make -q` against this Makefile | one grep over `test/` | design phase runs it | unvalidated |

### Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | Option 2 removes `FORCE` and a profile switch silently reuses the previous kernel | make `$(VARIANT)` drive the rebuild first, then remove `FORCE`, then prove the switch with a test |
| R-2 | Option 1 matches on `make -n` text, which is a fragile assertion | assert on the `--profile <name>` argument, which this `.ci` already greps for on the real build |

## Blast Radius

`tools/installer-kernel/Makefile`, `test/install/kernel-compose.ci`, and any
build path that relies on the current always-rebuild behavior.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make -C tools/installer-kernel PROFILE=compose-fixture` after the image is removed | -> | the `$(OUT)/Image` prerequisite list | `test/install/kernel-compose.ci`, the replacement for the `make -q` block |
| `make -C tools/installer-kernel PROFILE=<other>` after a build of a different profile | -> | the rebuild trigger | a new assertion in the same `.ci` |

## Acceptance Criteria

| AC | Input / Condition | Expected Behavior |
|----|-------------------|-------------------|
| AC-1 | The built image is removed and make is asked whether work remains | the assertion fails when the rebuild trigger is broken, and passes when it works |
| AC-2 | A build of one profile is followed by a build of another | the second build runs the recipe, and a named test proves it |
| AC-3 | The existing fragment and composition assertions in `kernel-compose.ci` | unchanged and still green |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| - | - | the defect lives in a Makefile and a `.ci`, which no Go unit test reaches |

### Functional Tests

| Test | File | Validates |
|------|------|-----------|
| rebuild trigger | `test/install/kernel-compose.ci` | AC-1 and AC-2, proven to discriminate by breaking the trigger and seeing the test go red |

## Files to Modify

- `test/install/kernel-compose.ci` - replace the vacuous `make -q` block
- `tools/installer-kernel/Makefile` - only if the design phase takes option 2

## Files to Create

None expected.

### Integration Checklist
- [ ] The chosen option is recorded in Key Design Decisions before any code
- [ ] The replacement assertion is proven to fail with the trigger broken

## Implementation Steps

1. Read the commit that added `FORCE` and settle A-1.
2. Choose between the two options and record the reason.
3. Replace the assertion, then prove it discriminates.

### Critical Review Checklist
- [ ] The new assertion fails when the rebuild trigger is broken
- [ ] No existing assertion in the file is changed, removed, or lowered
- [ ] A `PROFILE=` switch still forces a rebuild

## Known Limitations

The install suite has no `make` target, so running this `.ci` needs a hand-built
isolated binary pair. That gap is
`plan/spec-functional-suites-missing-their-make-target.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1, AC-2, AC-3 each proven by a named test
- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix

### Quality Gates
- [ ] `make ze-verify`
- [ ] `make ze-lint-changed`

### Closure
- [ ] Deferral shard row closed
