# Spec: kernel-profile-fixtures-leak-into-registry

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `plan/deferrals/kernel-profile-fixtures-leak-into-registry.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two functional tests write kernel profile fragments into the TRACKED source
directory the product's profile registry scans, and remove them only from an
EXIT trap. A killed run leaves a working, listable, buildable kernel profile
behind, and no gate refuses it.

`registeredKernelProfiles` in `internal/appliance/kernelreg.go` registers a
profile when `<name>.config` and `<name>.require` both sit in
`tools/installer-kernel/`. It reads the real directory, so a leaked pair is
indistinguishable from a shipped profile.

The writers:

| Test | Writes into `tools/installer-kernel/` | Cleanup |
|------|----------------------------------------|---------|
| `test/install/appliance-kernel-registry.ci` | `fixture.config`, `fixture.require`, `missing.config` | `trap cleanup EXIT` |
| `test/install/kernel-compose.ci` | `compose-fixture.config`, `compose-fixture.require` | `trap` at line 28 |

Observed on 2026-08-09 in the main working tree: `fixture.config`,
`fixture.require` and `missing.config` sat untracked, byte-identical to what
`appliance-kernel-registry.ci` writes. So `ze appliance kernel --profile
fixture` resolved on that machine, and `.gitignore` covers none of the three,
so a `git add tools/` would have shipped a fake profile.

Reproduction: run the test and stop it before the trap fires (SIGKILL, a
harness timeout, or an abort in a subshell that outlives the trap), then run
`git status --short tools/installer-kernel`.

Three candidate fixes, ranked for the design phase:

1. **The test points the registry at a temp directory.** The source tree stops
   being test scratch and the trap stops being load-bearing.
2. **The registry refuses an undocumented pair.** `tools/installer-kernel/PROFILES.md`
   already lists the shipped profiles; a pair with no row is an error.
3. **A gate refuses the leak.** `.gitignore` plus a check that the on-disk
   profile set equals the documented set. Weakest: it catches the commit, not
   the wrong build.

## Key Design Decisions

**Option 1 is chosen, and it needs NO product change.** The test builds its own
minimal repository root under the harness work directory and runs the kernel
command from there.

Both resolvers key off a root the test can control. `resolveKernelProfile` in
`internal/appliance/kernelreg.go` joins its `srcDir` argument, and
`kernelTargetFor` in `internal/appliance/cmd_kernel.go` gives the installer
target the relative `tools/installer-kernel`, so the Go side follows the
process working directory. `repo_root` in `tools/kernel-builder/run.py` walks up
from its own file to the first directory holding `go.mod`, and `main` joins
every relative path against it, so the python driver follows the copy the test
made rather than the real tree. A scratch root holding `go.mod`,
`tools/installer-kernel/`, `tools/kernel-builder/` and
`internal/appliance/kernel.version` therefore satisfies both, and the
`make -C tools/installer-kernel` path lands in the same place because its `OUT`
is relative to the Makefile.

**Option 2 is rejected.** Closing the registry against a manifest removes the
property commit `1e75b1911` added on purpose, and R-1 names the operator it
breaks. The defect is that tests write into the scanned directory, not that the
directory is scanned.

**Option 3 is adopted in the form of a unit test, not a `.gitignore` and not a
new gate script.** `TestRegisteredKernelProfilesShippedSet` reads the real
`tools/installer-kernel` and refuses a profile that is not the shipped set. It
catches a stray pair from ANY source, which the `.gitignore` half of option 3
does not, and it costs one test rather than a new check to register and
document.

**The planned `test/install/kernel-profile-scratch-isolation.ci` is dropped for
that unit test.** A functional test asserting that a directory stayed empty is
the absence trap `ai/rules/interop-and-goal-validation.md` names: deleting the
mechanism leaves the same absence, so it would pass with the fix reverted. The
unit test discriminates, because re-creating a fixture pair turns it red.

## Required Reading

### Architecture Docs
- [ ] `tools/installer-kernel/PROFILES.md` - the documented profile set
  → Constraint: the shipped set is documented already, so option 2 needs no new file
- [ ] `ai/rules/testing.md` - what a functional test may mutate
  → Constraint: a test that mutates tracked source needs a reason, and this one has none
- [ ] `internal/appliance/kernelreg.go` - `registeredKernelProfiles`, profile resolution
  → Decision: the open scan is the feature (commit `1e75b1911`); the fix must not close it

**Key insights:**
- The registry is open by design. The defect is that tests write into the
  directory the feature scans, not that the feature scans a directory.

## Current Behavior (MANDATORY)

Source files read:
- [ ] `internal/appliance/kernelreg.go` - `registeredKernelProfiles` walks the
  source directory, keeps every `<name>.config` that is not `kernel.config`,
  requires a sibling `<name>.require`, and returns the set. No manifest, no
  allowlist, no provenance check.
- [ ] `test/install/appliance-kernel-registry.ci` - creates the three fixture
  files at lines 20-25 inside `$repo`, after `cd "$repo"`, and relies on
  `trap cleanup EXIT` at line 16.
- [ ] `test/install/kernel-compose.ci` - the same shape for `compose-fixture`.

Behavior to preserve: the three properties below, unchanged by any of the
three candidate fixes.
- `ze appliance kernel --profile <name>` keeps resolving every profile the
  repository ships, with the same fragment order and the same require checks.
- The registry stays open: adding a profile stays a matter of adding its two
  files, with no core edit.
- Both `.ci` tests keep proving what they prove today (unknown profile fails,
  required symbol missing fails, fragments compose in order).

## Data Flow (MANDATORY)

### Entry Point
`ze appliance kernel --arch <arch> --profile <name>`, and the profile list the
same command prints when the profile is unknown.

### Transformation Path
1. The command resolves the source directory `tools/installer-kernel/`.
2. `registeredKernelProfiles` lists `<name>.config` / `<name>.require` pairs.
3. The named profile's fragment chain is resolved through its `# ze-base:` and
   `# ze-include:` headers.
4. The fragment list is handed to the builder, which composes the kernel config.
5. The `.require` symbols are enforced against the produced config.

### Boundaries Crossed

| From | To | Carried |
|------|----|---------|
| CLI flag | `internal/appliance` profile registry | profile name |
| Registry | filesystem `tools/installer-kernel/` | directory listing |
| Registry | kernel builder | ordered fragment paths |

### Integration Points
- `tools/installer-kernel/Makefile` (`PROFILE=` target) reads the same files.
- `tools/kernel-builder/build.py` consumes the composed fragment list.
- `tools/installer-kernel/PROFILES.md` documents the set for operators.

### Architectural Verification
- Registration over hardcoding: the fix must not replace the directory scan
  with a hardcoded profile switch in a core package. A manifest row is data the
  registry discovers, not a case in a core file.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | Validation | Status |
|----|------------|-------|-----------|--------|
| A-1 | The `.ci` can drive the registry against a directory other than `tools/installer-kernel/` | the resolver takes a source directory argument | design phase reads the entry point and confirms or kills option 1 | confirmed, by a different route than assumed: no flag or env var reaches `srcDir`, `kernelTargetFor` hardcodes the relative `tools/installer-kernel`, so the test moves the ROOT rather than the argument. `repo_root` in `run.py` follows the same move |
| A-2 | No shipped profile is absent from `PROFILES.md` | the file documents the set today | compare the documented set against the directory before option 2 is chosen | broken, and it retires option 2 either way: `PROFILES.md` names `qemu`, `hardware`, `hardware-kms` and `n100` in one Examples sentence, and `n100` ships no files. The page documents the FORM of a profile, never the set, so nothing there is parseable as a manifest |

### Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | Option 2 breaks an operator who adds a local profile | keep the failure a clear error naming the manifest, or pick option 1 |
| R-2 | A leaked pair already sits in someone's tree and the new gate turns their next commit red | the gate names the file and the removal command |

## Blast Radius

`internal/appliance` kernel profile resolution, the two `.ci` tests, and any
developer tree that currently holds a leaked fixture pair.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ze appliance kernel --profile fixture`, run from the scratch root | -> | `resolveKernelProfile` | `test/install/appliance-kernel-registry.ci` |
| `make -C tools/installer-kernel PROFILE=compose-fixture`, run from the scratch root | -> | `repo_root` and `main` in `tools/kernel-builder/run.py` | `test/install/kernel-compose.ci` |
| `registeredKernelProfiles` reading the real `tools/installer-kernel` | -> | the shipped profile set | `TestRegisteredKernelProfilesShippedSet` |

## Mistake Log

### Wrong Assumptions

| ID | What was assumed | What is true | Effect |
|----|------------------|--------------|--------|
| A-2 | `PROFILES.md` documents the shipped profile set | it documents the FORM of a profile and lists `n100`, which ships no files, in the same sentence as the three real ones | option 2 loses the manifest it was going to read, and `TestRegisteredKernelProfilesShippedSet` carries the expected set itself |

## Acceptance Criteria

| AC | Input / Condition | Expected Behavior |
|----|-------------------|-------------------|
| AC-1 | A test run is stopped before its trap fires | `git status --short tools/installer-kernel` is empty |
| AC-2 | A `<name>.config` / `<name>.require` pair sits in the source directory outside the shipped set | `TestRegisteredKernelProfilesShippedSet` fails and names the offending profile; no test in the suite can create the pair there |
| AC-3 | Every profile the repository ships | resolves exactly as it does today, same fragment order |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestRegisteredKernelProfilesShippedSet` | `internal/appliance/kernelreg_test.go` | AC-2 and AC-3: the real directory holds the shipped set and nothing else |

### Functional Tests

| Test | File | Validates |
|------|------|-----------|
| registry behavior | `test/install/appliance-kernel-registry.ci` | AC-1 and AC-3: the open registry still resolves, and the run writes nothing into the repository |
| fragment composition | `test/install/kernel-compose.ci` | AC-1: the make path composes the same fragments from a scratch root |

## Files to Modify

- `test/install/appliance-kernel-registry.ci` - build a scratch root, run the command there
- `test/install/kernel-compose.ci` - same for the `make -C` half of the test
- `internal/appliance/kernelreg_test.go` - add the shipped-set test
- `tools/installer-kernel/PROFILES.md` - say where a test profile goes, so the next test does not land here

## Files to Create

None. The scratch root is built at run time inside the harness work directory.

### Integration Checklist
- [ ] The chosen option is recorded in Key Design Decisions before any code
- [ ] `make ze-test-pkg PKG=./internal/appliance` and the owning install-suite target run green

## Implementation Steps

1. Phase 1: `test/install/appliance-kernel-registry.ci` builds a scratch root and runs `ze` there.
2. Phase 2: `test/install/kernel-compose.ci` does the same for its `make -C` half.
3. Phase 3: add `TestRegisteredKernelProfilesShippedSet`, and the `PROFILES.md` note.

## Implementation Phases

| Phase | Work | Verify |
|-------|------|--------|
| 1 | `test/install/appliance-kernel-registry.ci` stops writing into `tools/installer-kernel/` | the install-suite target for that `.ci`, then `git status --short tools/installer-kernel` is empty |
| 2 | `test/install/kernel-compose.ci` stops writing into `tools/installer-kernel/` and into repo-root `build/kernel/` | the install-suite target for that `.ci`, then the same clean check |
| 3 | `TestRegisteredKernelProfilesShippedSet` plus the `PROFILES.md` note | `make ze-test-pkg PKG=./internal/appliance`, red with a fixture pair present, green without |

All three phases are implemented and green. The combined evidence, run in the
main thread with every phase in the tree: the install suite passes 37 of 37 with
3 environment skips, and `git status --short tools/installer-kernel` afterwards
shows only the intended `PROFILES.md` edit, with repo-root `build/kernel/`
untouched. The suite was driven by a hand-built isolated binary pair, because
no `make ze-install-test` target exists; that gap has its own spec,
`plan/spec-functional-suites-missing-their-make-target.md`.

Each phase proved its isolation by running its body with the trap removed and
the run stopped early: the HEAD body leaves the fixture pair in
`tools/installer-kernel/`, and the new body leaves the repository clean.

### Critical Review Checklist
- [ ] Registration over hardcoding: no per-profile case added to a core package
- [ ] The open registry still admits a new profile without a core edit
- [ ] The failure message names the offending file

## Known Limitations

Option 3 alone would catch the commit and not the wrong build, so it is not
sufficient on its own.

`TestRegisteredKernelProfilesShippedSet` sees PAIRS, because
`registeredKernelProfiles` skips a `<name>.config` with no sibling
`<name>.require`. A lone file such as the `missing.config` the registry test
writes never registers, so it is never named by that test. Only phases 1 and 2
stop it being written at all.

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
