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

### Documentation Update Checklist (BLOCKING)

Added at closure: the spec was written at `skeleton` depth mid-session and
carried none of the three checklists this table belongs to.

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No -- `git show --stat 1acf1a627` carries no product Go file; the only Go file is `internal/appliance/kernelreg_test.go` | - |
| 2 | Config syntax changed? | No -- no `*.yang` and no parser in the diff | - |
| 3 | CLI command added/changed? | No -- `kernelTargetFor` and the `ze appliance kernel` flag set are untouched | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No -- `docs/guide/appliance.md` and `docs/guide/ze-install.md` describe the command, and the command did not change | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No -- no protocol code in the diff | - |
| 10 | Test infrastructure changed? | No -- the harness is unchanged; two `.ci` bodies changed where they write. `docs/functional-tests.md` anchors `test/install/kernel-compose.ci` as the runtime fragment contract, and that contract is unchanged | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes -- the registry's directory scan makes a fixture pair in the tracked directory a real profile, and that trap was written nowhere | `docs/architecture/appliance/kernel-profiles.md`, Traps section, with anchors on `kernelInstallerConfigDir`, `repo_root`, and `TestRegisteredKernelProfilesShippedSet` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No -- the profile set on disk is unchanged: `hardware`, `hardware-kms`, `qemu` | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes -- `docs/functional-tests.md` anchors `test/install/kernel-compose.ci` as the runtime fragment contract; re-read and still true. `ai/CODE-TO-DOCS.md` regenerated by `make ze-doc-index` for the new `kernelreg_test.go` anchor | `ai/CODE-TO-DOCS.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No update needed -- `docs/guide/ze-install.md` and `docs/guide/appliance.md` show the kernel build examples against `resolveKernelProfile` and the installer Makefile, and no command, flag, or profile name changed | - |

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

### Deliverables Checklist

Added at closure, with the same note as the Documentation table above.

| Deliverable | Verification method |
|-------------|---------------------|
| `test/install/appliance-kernel-registry.ci` runs from a scratch root and carries no cleanup trap | `grep -n 'cd "\$root"' test/install/appliance-kernel-registry.ci` and `grep -c 'trap' test/install/appliance-kernel-registry.ci` |
| `test/install/kernel-compose.ci` builds under a scratch root, and its trap removes only its own temporary file | `grep -n 'make -C "\$scratch/tools/installer-kernel"' test/install/kernel-compose.ci` and read the `cleanup` function |
| `TestRegisteredKernelProfilesShippedSet` exists, passes, and carries a discrimination subtest | `go test -run TestRegisteredKernelProfilesShippedSet -v ./internal/appliance` |
| `shippedKernelProfiles` equals the on-disk set | `ls tools/installer-kernel/*.require` against the literal in `internal/appliance/kernelreg_test.go` |
| `tools/installer-kernel/PROFILES.md` says where a test profile goes | `grep -n 'scratch directory' tools/installer-kernel/PROFILES.md` |
| `docs/architecture/appliance/kernel-profiles.md` carries the trap | `grep -n 'scratch repository root' docs/architecture/appliance/kernel-profiles.md` |
| No test writes a profile pair into the tracked scanned directory | `grep -rn 'tools/installer-kernel' test/`, then read every write site: each one must be under a scratch root, never under the repository |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Shell path handling in both `.ci` bodies | every expansion quoted; every `cp -R`, `rm -f`, `mkdir -p` and redirection target stays under `$work`; `ZE_REPO_ROOT` is read only, never a write target |
| Source-tree integrity | no test writes a `<name>.config` plus `<name>.require` pair into a directory `registeredKernelProfiles` scans, so no interrupted run leaves a buildable profile |
| The guard fails closed | `TestRegisteredKernelProfilesShippedSet` calls `t.Fatalf` when `registeredKernelProfiles` returns an error, so an unreadable directory is a red test rather than an empty pass |
| Profile name injection | `validateKernelProfileName` still rejects `../x`, an empty name, and illegal characters, and nothing in the diff reaches `resolveKernelProfile` around it |
| Copy fidelity | the scratch root copies the builder directory rather than linking it, because `Path(__file__).resolve()` in `run.py` follows a symlink back into the real repository and defeats the isolation |

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
## Implementation Summary

### What Was Implemented
- `test/install/appliance-kernel-registry.ci` builds a scratch repository root
  under the harness work directory (`go.mod`, a copy of `tools/installer-kernel/`,
  a copy of `tools/kernel-builder/`, `internal/appliance/kernel.version`), runs
  `ze appliance kernel` from there, and carries no cleanup trap at all.
- `test/install/kernel-compose.ci` does the same for its `make -C` half. Its
  fragment assertions still read the tracked files in place, because they only
  read. Its remaining trap removes one temporary file and no shared state.
- `TestRegisteredKernelProfilesShippedSet` in `internal/appliance/kernelreg_test.go`
  reads the real `tools/installer-kernel` through `registeredKernelProfiles` and
  fails on any profile outside `shippedKernelProfiles`. Its second subtest seeds
  a temp directory with a stray pair, so the first subtest is proven to go red.
- `tools/installer-kernel/PROFILES.md` says a test profile goes in a scratch
  directory and names the test that enforces it.
- No product code changed. The registry stays open, which is the property commit
  `1e75b1911` added.

### Bugs Found/Fixed
- The defect itself: an interrupted run left `fixture.config`, `fixture.require`
  and `missing.config` in the tracked directory, and `registeredKernelProfiles`
  cannot tell that pair from a shipped profile. Covered by
  `TestRegisteredKernelProfilesShippedSet` for any pair from any source, and
  removed at the root by the two `.ci` bodies.
- The builder directory must be COPIED, not symlinked: `repo_root` in
  `tools/kernel-builder/run.py` starts from `Path(__file__).resolve()`, and a
  symlink resolves back into the real repository, which defeats the isolation.

### Documentation Updates
- `tools/installer-kernel/PROFILES.md`: where a test profile goes, and the test
  that enforces it.
- `docs/architecture/appliance/kernel-profiles.md`, Traps section: the scratch
  root rule, with anchors on `internal/appliance/cmd_kernel.go`
  (`kernelInstallerConfigDir`), `tools/kernel-builder/run.py` (`repo_root`), and
  `internal/appliance/kernelreg_test.go`
  (`TestRegisteredKernelProfilesShippedSet`).
- `ai/CODE-TO-DOCS.md` regenerated by `make ze-doc-index` for the new anchor.
- `make ze-doc-test` exit 0 after the regeneration.

### Deviations from Plan
- The planned `test/install/kernel-profile-scratch-isolation.ci` was dropped for
  `TestRegisteredKernelProfilesShippedSet`. A functional test asserting a
  directory stayed empty is the absence trap `ai/rules/interop-and-goal-validation.md`
  names: it passes with the fix reverted.
- Option 2 (a manifest the registry enforces) was killed by A-2, not by taste.
  `PROFILES.md` documents the FORM of a profile and names `n100`, which ships no
  files, so nothing on that page is parseable as a manifest. The expected set now
  lives in `shippedKernelProfiles` in the test.
- Three checklists (Deliverables, Security Review, Documentation Update) were
  absent: the spec was written at `skeleton` depth mid-session and never got its
  design-time sections. They were added at closure and filled with real evidence,
  because the reviews they drive are the point of the sections.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed no shipped profile is absent from `tools/installer-kernel/PROFILES.md`, so the page could act as a manifest | the page documents the FORM of a profile and names `qemu`, `hardware`, `hardware-kms` and `n100` in one Examples sentence; `n100` ships no files | comparing the page against `ls tools/installer-kernel/*.require` during design | option 2 retired; `shippedKernelProfiles` in `internal/appliance/kernelreg_test.go` carries the set, and `PROFILES.md` now names that test |
| approach | A-1 assumed the `.ci` could point the resolver at another directory through a flag or an environment variable | no flag or environment variable reaches `srcDir`. `kernelInstallerConfigDir` in `internal/appliance/cmd_kernel.go` is the literal relative `tools/installer-kernel` | reading `kernelTargetFor` rather than its callers | the test moves the process working directory instead of the argument; `repo_root` in `run.py` follows the same move |
| escalation | the spec reached closure with no Deliverables, Security Review, or Documentation Update checklist | a `skeleton`-depth spec written mid-session skips the design-time sections `/ze-close` consumes | `/ze-close` step 1 found all three absent | added and filled at closure rather than stopping; recorded here so the next mid-session spec gets them at creation |

## Implementation Audit

<!-- BLOCKING before the learned summary. See ai/rules/completion.md.
     Status: Done (with file:line) | Partial | Skipped | Changed.
     Partial and Skipped both require explicit user approval. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Stop `test/install/appliance-kernel-registry.ci` writing profile fragments into the tracked directory | Done | `test/install/appliance-kernel-registry.ci`, the `root=$work/root` block and `cd "$root"` | the trap is gone as well, because nothing needs cleaning |
| Stop `test/install/kernel-compose.ci` doing the same | Done | `test/install/kernel-compose.ci`, the `scratch=$work/scratch-root` block and `make -C "$scratch/tools/installer-kernel"` | its trap now removes one temporary file |
| A gate refuses a leaked pair from any source | Done | `TestRegisteredKernelProfilesShippedSet` in `internal/appliance/kernelreg_test.go` | reads the real directory through `registeredKernelProfiles` |
| Keep the registry open (no manifest in the product) | Done | `registeredKernelProfiles` in `internal/appliance/kernelreg.go` is unchanged | the commit touches no product Go file |
| Keep every shipped profile resolving with the same fragment order | Done | `TestResolveProfileFragments` and `TestResolveSharedInclude` in `internal/appliance/kernelreg_test.go`, both unchanged and green | `make ze-test-pkg PKG=./internal/appliance` exit 0 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | the `.ci` body run from a clean work directory and stopped part way: the three fixture files land in `<work>/root/tools/installer-kernel/` and `git status --short tools/installer-kernel` is empty | the same extraction of the pre-fix body, SIGKILLed after the writes (shell exit 137), leaves all three in the repository root it was pointed at |
| AC-2 | Done | `TestRegisteredKernelProfilesShippedSet` | the `real directory` subtest names the offending profile and gives both remedies; the `stray pair is registered` subtest proves the first would go red |
| AC-3 | Done | `TestResolveProfileFragments`, `TestResolveSharedInclude`, `TestEnumerateRegistry`, and the `shipped profile` half of `TestRegisteredKernelProfilesShippedSet` | `hardware`, `hardware-kms` and `qemu` still register and still resolve in order |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRegisteredKernelProfilesShippedSet` | Done | `internal/appliance/kernelreg_test.go` | two subtests: the real directory, and the discrimination proof in a temp directory |
| registry behavior | Done | `test/install/appliance-kernel-registry.ci` | unknown profile fails, missing manifest fails, requirement floor enforced, all from the scratch root |
| fragment composition | Done | `test/install/kernel-compose.ci` | the `make -C` half runs against the scratch root; one assertion ADDED, none changed or removed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `test/install/appliance-kernel-registry.ci` | Done | scratch root, no trap |
| `test/install/kernel-compose.ci` | Done | scratch root for the build half |
| `internal/appliance/kernelreg_test.go` | Done | `shippedKernelProfiles` plus the new test |
| `tools/installer-kernel/PROFILES.md` | Done | says where a test profile goes |
| `docs/architecture/appliance/kernel-profiles.md` | Changed | not in the plan; added at closure through the Documentation Update Checklist, because the trap was written nowhere |
| `ai/CODE-TO-DOCS.md` | Changed | generated by `make ze-doc-index` for the new anchor |

### Audit Summary
- **Total items:** 17 (5 requirements, 3 AC, 3 tests, 6 files)
- **Done:** 15
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (`docs/architecture/appliance/kernel-profiles.md` and `ai/CODE-TO-DOCS.md`, both recorded in Deviations and in Documentation Updates)

## Goal Validation (BLOCKING)

<!-- Maps each goal from the Task section to proof it was achieved. "Tests pass"
     is not evidence for a goal; a named test with its output is.
     See ai/rules/interop-and-goal-validation.md for the required evidence per
     goal type, and for the vacuity traps: a test that would still pass with the
     behavior reverted proves nothing. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A killed test run leaves no kernel profile in the source tree | functional, run against the real `.ci` body | the `run.sh` body of `test/install/appliance-kernel-registry.ci` extracted and run from a clean work directory, then stopped part way: `fixture.config`, `fixture.require` and `missing.config` land under `<work>/root/tools/installer-kernel/`, and `git status --short tools/installer-kernel` prints nothing. The test carries no trap, so there is no cleanup to skip |
| The evidence discriminates | functional, differential | the same extraction of the pre-fix body (`git show 1acf1a627^:test/install/appliance-kernel-registry.ci`), SIGKILLed after its writes (shell exit 137) with `ZE_REPO_ROOT` pointed at a stand-in repository: all three files remain in that repository. The trap did not run. Reverting the fix therefore turns this evidence red |
| A leaked pair from ANY source is refused | unit, with its own discrimination proof | `TestRegisteredKernelProfilesShippedSet` in `internal/appliance/kernelreg_test.go`. `real directory` reads `tools/installer-kernel` through `registeredKernelProfiles` and names any profile outside `shippedKernelProfiles`; `stray pair is registered` seeds a temp directory with a `fixture` pair and asserts `registeredKernelProfiles` returns it, which is what makes the first subtest capable of going red |
| Every shipped profile still resolves unchanged | unit | `make ze-test-pkg PKG=./internal/appliance` exit 0. `TestResolveProfileFragments`, `TestResolveSharedInclude`, `TestResolveNestedIncludeRejected`, `TestResolveProfileRequiresManifest` and `TestEnumerateRegistry` are untouched by this change and green |
| The registry stays open | source, by absence of change | `git show --stat 1acf1a627` carries no product Go file. `registeredKernelProfiles` in `internal/appliance/kernelreg.go` still scans the directory and takes no manifest, so adding a profile is still adding two files |

## Deferrals Resolved

<!-- Closure must leave no dangling row: deferral_unassigned_problems in
     scripts/dev/commit_helper.py WARNS (it does not block) on a live row with no
     destination -- act on the warning here, because nothing else will.
     The spec's own shard is git rm'd at closure ONLY when every row in it is
     terminal; a shard still holding a live row outlives its source spec and
     deferral_shard_removal_problems blocks its removal
     (ai/rules/planning.md). Account for every row here.
     If resolving a row empties a FOREIGN shard (its last live row becomes
     terminal), that shard is now residue and this closure removes it too. -->
| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `test/install/appliance-kernel-registry.ci` and `test/install/kernel-compose.ci` write kernel profile fragments into the tracked directory `registeredKernelProfiles` scans, cleaned only by an EXIT trap | done | this spec. Both tests now build a scratch repository root; the registry test carries no trap and the compose test's trap removes one temporary file. `TestRegisteredKernelProfilesShippedSet` refuses a leaked pair from any source |

Every row in `plan/deferrals/kernel-profile-fixtures-leak-into-registry.md` is
terminal, so commit B removes that shard.

No FOREIGN shard was emptied by this resolution. The row is the only one in the
file, and it is the only row anywhere whose Destination is this spec.
`grep -rl 'kernel-profile-fixtures-leak-into-registry' plan/deferrals/` returns
four files: this spec's own shard, and the three shards below, which name this
spec as their SOURCE and are each homed at a spec of their own.

Three defects found while doing this work are homed in their own specs and their
own shards, none commissioned. They are not rows of this spec's shard and they do
not hold it open:

| Defect | Spec | Shard |
|--------|------|-------|
| the relax-token gate binds per file, not per change | `plan/spec-relax-token-gate-is-per-file-not-per-change.md` | `plan/deferrals/relax-token-gate-is-per-file-not-per-change.md` |
| three functional suites have no make target | `plan/spec-functional-suites-missing-their-make-target.md` | `plan/deferrals/functional-suites-missing-their-make-target.md` |
| the `make -q` assertion in `kernel-compose.ci` is vacuous | `plan/spec-kernel-compose-make-q-assertion-is-vacuous.md` | `plan/deferrals/kernel-compose-make-q-assertion-is-vacuous.md` |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). The review is INDEPENDENT: reviewer
     subagents or a fresh session over the actual diff, never your own inline
     reasoning about code you just wrote.

     The machine-checked artifact is the deliverable, not this table:
     scripts/dev/review_gate.py record --spec <spec> --rounds <N> ... then check.
     --rounds is the pass count and is required; more than three needs
     --rounds-reason naming the PRODUCT defect a later round found. A false
     statement in this record is a NOTE, never a reason for another round
     (ai/rules/planning.md).
     commit_helper.py runs `review_gate.py check` on the closure commit and
     refuses without a fresh, hash-pinned, CLEAN artifact. Record the artifact
     first; this table exists only to carry what was FOUND and FIXED forward
     into the learned summary. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/kernel-profile-fixtures-leak-into-registry-86ae859e-de6d-4ba5-88c2-2d92e836363b.md` |
| `review_gate.py check` | clean |
| Rounds | 1. Two independent reviewer subagents ran the same round in parallel over the same diff, and both returned 0 BLOCKER and 0 ISSUE. No product defect was found, so no later round was earned |
| Reviewer lenses used | logic + wiring + isolation correctness; security + residual risk + record truthfulness |

Scope of the review: the union of commit `1acf1a627` and the closure edits in
the working tree. The implementation was already committed when closure started,
so a review of "uncommitted changes" alone would have read none of it.

Automated pre-checks: `make ze-validate` exit 0. `make ze-doc-test` exit 0 after
`make ze-doc-index`. `python3 scripts/dev/audit-test-relaxation.py --base HEAD~2`
reports 0 deleted, 0 weakened, and one documented `[RELAXED]` in
`test/install/kernel-compose.ci`. Both reviewers checked that reason against the
diff assertion by assertion and both found it TRUE: no assertion was changed,
removed, or lowered, and one was added (`grep '^profile=compose-fixture$'` on the
provenance sidecar `write_provenance` in `tools/kernel-builder/run.py` writes into
`--out-dir`).

### Findings fixed

No finding reached BLOCKER or ISSUE in either lens, so this table is empty by
result rather than by omission.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

### Notes recorded

NOTEs do not block. Five were record defects and each was fixed in one edit. Two
were product observations: one is now a spec, one is answered here.

| # | Finding | Location | Disposition |
|---|---------|----------|-------------|
| 1 | The header claimed the test "writes nothing under the repository". The harness work directory sits under the repository's gitignored `tmp/` (`DefaultScratchRoot` in `internal/test/sessionpath/sessionpath.go`) | `test/install/appliance-kernel-registry.ci` | fixed: the claim now says "nothing into the tracked tree" and names the work directory |
| 2 | The `test-relax:` reason said "the 12 deleted lines". 12 is the audit tool's NET line delta; 34 lines were deleted | `test/install/kernel-compose.ci` | fixed: the count is dropped, the substance is unchanged |
| 3 | `PROFILES.md` said "this directory holds exactly the profiles the repository ships" two lines under an Examples sentence naming `n100`, which ships no files | `tools/installer-kernel/PROFILES.md` | fixed: the Examples sentence now says these are valid name forms, and the shipped set is named |
| 4 | The same paragraph told a reader to add "a paragraph on this page", and the page carries no per-profile paragraph | `tools/installer-kernel/PROFILES.md` | fixed: it now says to add the name to the shipped-set sentence |
| 5 | Three source anchors split the Traps list into two rendered lists, and one added sentence was a semicolon splice | `docs/architecture/appliance/kernel-profiles.md` | fixed: anchors moved below the list, splice split into two sentences |
| 6 | `stray pair is registered` overlaps `TestEnumerateRegistry`, which already proves `registeredKernelProfiles` returns an unshipped pair | `internal/appliance/kernelreg_test.go` | kept. It is the discrimination proof this spec's Goal Validation names, it sits beside the guard it discriminates for, and deleting a test to save 20 lines is the move `ai/rules/testing.md` refuses |
| 7 | The `make -q` block cannot fail: `$(OUT)/Image` lists the phony `FORCE` as a prerequisite, so `make -q` always reports work to do. Found independently by both lenses | `test/install/kernel-compose.ci`, `tools/installer-kernel/Makefile` | not fixed here. `FORCE` is on that line at `1acf1a627^`, so the vacuity predates this work and this work does not depend on it. Spec written: `plan/spec-kernel-compose-make-q-assertion-is-vacuous.md`, Status `skeleton`, with `plan/deferrals/kernel-compose-make-q-assertion-is-vacuous.md` |

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row run a command (ls, grep, go test -run) now.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in scripts/dev/commit_helper.py checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
<!-- Every file in "Files to Create", and every .ci named in Wiring Test and
     Functional Tests. Paste the ls output. -->
| File | Exists | Evidence |
|------|--------|----------|
| `test/install/appliance-kernel-registry.ci` | yes | `grep -n 'cd "$root"'` returns the `cd` into the scratch root; `grep -c trap` returns 1, and that one hit is the word inside a comment, not a `trap` command |
| `test/install/kernel-compose.ci` | yes | `grep -n 'make -C "$scratch/tools/installer-kernel"'` returns both make invocations |
| `internal/appliance/kernelreg_test.go` | yes | `go test -run TestRegisteredKernelProfilesShippedSet -v ./internal/appliance` lists both subtests and passes |
| `tools/installer-kernel/PROFILES.md` | yes | `grep -n 'scratch directory'` returns the added paragraph |
| `docs/architecture/appliance/kernel-profiles.md` | yes | `grep -n 'scratch repository root'` returns the added Traps bullet |
| Files to Create | none planned, none created | the scratch root is built at run time inside the harness work directory |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a run stopped before any cleanup leaves `tools/installer-kernel` clean | the `run.sh` body was extracted from the `.ci` and run from a clean work directory. It stopped part way on an unbound variable, which is the interrupted case. `find` shows `fixture.config`, `fixture.require` and `missing.config` under `<work>/root/tools/installer-kernel/`, and `git status --short tools/installer-kernel` printed nothing |
| AC-1 | the evidence discriminates | the same extraction of `git show 1acf1a627^:test/install/appliance-kernel-registry.ci`, with `kill -9 $$` appended after the writes, exited 137 and left all three files in the stand-in repository root it was pointed at. The trap never ran |
| AC-2 | a pair outside the shipped set fails the test and is named | `TestRegisteredKernelProfilesShippedSet/real_directory` calls `t.Errorf` with the profile name and both file paths; `TestRegisteredKernelProfilesShippedSet/stray_pair_is_registered` proves `registeredKernelProfiles` returns such a pair, so the first subtest can go red. Both pass |
| AC-2 | no test in the suite can create the pair there | `grep -rn 'tools/installer-kernel' test/`, then each write site read: `appliance-kernel-registry.ci` writes after `cd "$root"`, `kernel-compose.ci` writes to `"$scratch/..."`, and `test/appliance/appliance-iso-arm64.ci` and `appliance-iso-default-paths.ci` write relative to the harness work directory with no `cd` into the repository |
| AC-3 | every shipped profile resolves as before | `make ze-test-pkg PKG=./internal/appliance` exit 0, `ok github.com/ze-software/ze/internal/appliance 64.323s`. `ls tools/installer-kernel/*.require` gives `hardware`, `hardware-kms`, `kernel`, `qemu`, and `registeredKernelProfiles` excludes `kernel.config` by name, which is exactly `shippedKernelProfiles` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze appliance kernel --profile fixture`, run from the scratch root | `test/install/appliance-kernel-registry.ci` | read: `cd "$root"` precedes the fixture writes, and the invocation asserts the docker argv carries `--fragment /src/kernel.config --fragment /src/qemu.config --fragment /src/fixture.config`, which is the resolved chain through `# ze-base: qemu` |
| `make -C tools/installer-kernel PROFILE=compose-fixture`, run from the scratch root | `test/install/kernel-compose.ci` | read: both `make -C` calls target `"$scratch/tools/installer-kernel"`, and the assertions read `"$scratch/build/kernel/config"`, which is where `OUT := ../../build/kernel` lands from that Makefile |
| `registeredKernelProfiles` reading the real `tools/installer-kernel` | `internal/appliance/kernelreg_test.go` | read: the `real directory` subtest passes the relative `../../tools/installer-kernel`, which resolves from the package directory a Go test runs in |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, by a different route than assumed | no flag and no environment variable reaches `srcDir`. `kernelInstallerConfigDir` in `internal/appliance/cmd_kernel.go` is the literal `"tools/installer-kernel"` and `kernelTargetFor` hands it to `configDir` unchanged, so the resolver follows the process working directory. `repo_root` in `tools/kernel-builder/run.py` walks up from `Path(__file__).resolve().parent` to the first `go.mod`. The test moves the root, not the argument |
| A-2 | broken | `tools/installer-kernel/PROFILES.md` named `qemu`, `hardware`, `hardware-kms` and `n100` in one Examples sentence, and `ls tools/installer-kernel/` shows `n100` ships neither file. The page documents the FORM of a profile, so nothing on it was parseable as a manifest. Recorded in the Mistake Log and in Deviations; option 2 was retired and `shippedKernelProfiles` carries the set |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 12, Yes: the Traps bullet in `docs/architecture/appliance/kernel-profiles.md` | `kernelInstallerConfigDir = "tools/installer-kernel"` read in `internal/appliance/cmd_kernel.go`; `repo_root` read in `tools/kernel-builder/run.py`; `TestRegisteredKernelProfilesShippedSet` read and run | yes, each of the three anchors names a symbol that exists and produces the stated behavior |
| Row 16, Yes: `ai/CODE-TO-DOCS.md` | `make ze-doc-index` regenerated it, and the whole diff is the one new `kernelreg_test.go` row plus its two counters | yes |
| Row 16, Yes: the existing anchor on `test/install/kernel-compose.ci` in `docs/functional-tests.md` | it claims the file is the runtime fragment contract; the `require_enabled` and `require_absent` loops that make it that contract are untouched by this change | yes, still true, no edit needed |
| Rows 1-11, 13-15, 17: No | `git show --stat 1acf1a627` carries no product Go file, no `*.yang`, no plugin `register.go`, and no protocol code. `grep -rn 'source: .*kernelreg\|source: .*installer-kernel\|source: .*cmd_kernel' docs/ ai/` returns the anchors in `docs/guide/ze-install.md`, `docs/guide/appliance.md`, `docs/architecture/appliance/build-artifacts.md` and `kernel-profiles.md`, and each claims resolution or build behavior that did not change | yes |
| `make ze-doc-test` | exit 0 after `make ze-doc-index` | yes |

## Core Insight

A registry that discovers its members by scanning a directory makes that
directory an API surface, and a test that writes into it is publishing, not
staging. The trap that removed the fixture was not a safety net: it was the only
thing standing between an interrupted test and a fake kernel profile that builds.
The fix is not to close the registry, which is the feature, but to give the test
its own root. Both resolvers already followed a root the caller controls, one
through the process working directory and one through the nearest `go.mod`, so
the isolation cost no product change at all.
