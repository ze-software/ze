# Spec: kernel-compose-make-q-assertion-is-vacuous

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/1 |
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

## Key Design Decisions

**Decision: option 2, and it is forced rather than chosen.**

The design phase found the vacuity is deeper than the Task section states.
`rm -f X; make -q` reports work to do for EVERY Makefile, because a missing
target always needs remaking. The assertion would be vacuous with `FORCE`
deleted and nothing put in its place. So no rewrite of the assertion, option 1
included, can discriminate while the Makefile always rebuilds. The only state
that discriminates is the steady state: a build with nothing changed must report
NOTHING to do. `test/install/kernel-runtime-deps.ci` already asserts exactly
that against `gokrazy/kernel/Makefile` with `expect_status 0`, so the shape is
established in this suite.

**A-1 is BROKEN, and its correction removes R-1's premise.** `FORCE` was never
the `PROFILE=` / `ARCH=` trigger. `30a24a874` introduced it as the trigger of a
separate rule, `$(VARIANT): FORCE`, whose recipe rewrote the stamp ONLY when the
content differed, and `$(OUT)/Image` then took `$(VARIANT)` as an ordinary file
prerequisite whose mtime advanced on a real change. `1e75b1911` deleted that
rule, moved the stamp write into the build recipe unconditionally, and put
`FORCE` where `$(VARIANT)` had been in the prerequisite list. That converted a
change-triggered rebuild into an unconditional one. The commit message does not
mention it: the change is collateral of replacing the shell recipe with a Python
builder. This belongs in the Mistake Log as a wrong assumption at closure.

**Two constraints bound the fix.**

| Constraint | Producer | What it forbids |
|-----------|----------|-----------------|
| The Makefile must not read `kernel.version` | `test/install/kernel-version-single-reader.ci`, the `cat[^\|]*kernel\.version` grep | a parse-time `$(shell cat ...)` of the version. `run.py` is the single build-time reader |
| `.variant` content stays `<arch>-<profile>-<version>-<builder>` from the BUILT provenance | `installerKernelBuildMatches` in `internal/appliance/cmd_iso.go` | repurposing `.variant` as a parse-time stamp, and any change to what the recipe writes into it |

**The shape.** Delete `FORCE` and its `.PHONY` entry. Add
`../../internal/appliance/kernel.version` as an ordinary file prerequisite of
`$(OUT)/Image`, which covers a version bump the make-native way and reads
nothing. Add one parse-time stamp `$(OUT)/.request` holding
`$(ARCH)-$(PROFILE)-$(BUILDER)`, written by a `$(shell ...)` only when the
content differs, and list it as a prerequisite. It covers the three triggers
that are variables rather than files.

→ Decision: a NEW file `$(OUT)/.request`, not a reuse of `$(OUT)/.variant`. The
recipe writes `.variant` AFTER `run.py` produces `Image`, so `.variant` is
always newer than `Image` and reusing it as a prerequisite would rebuild
forever unless the recipe also touched `Image`. `.request` is written at parse
time, before the build, so it is older than `Image` in the steady state. The
two files carry two different facts: `.request` is what was asked for and only
make reads it, `.variant` is what was built and only Go reads it.

→ Constraint: the `$(shell ...)` writes at parse time, so `make -q` can create
`$(OUT)` and rewrite `.request`. That is outside `-q`'s "changes nothing"
contract and it is what makes `make -q PROFILE=other` answer correctly. The
Makefile comment must say so.

**Correction, made during implementation: the stamp is COMPARED at parse time,
never listed as a prerequisite.** The shape above puts `$(OUT)/.request` on the
prerequisite line, which makes the trigger an mtime comparison. That does not
work on this repository's dev platform. `make --version` on macOS is GNU Make
3.81, which resolves mtimes to one second, so a stamp written in the same second
as the build it follows reads as "not newer". Measured in the scratch tree:
`.request` at `1786312764.639256556` against `Image` at `1786312764.185367635`,
and the second build answered `make: Nothing to be done for 'all'` while
`.variant` still held `amd64-qemu-7.1.4-docker`. The profile switch was silently
lost, which is R-1 happening.

The mechanism that ships compares instead of stamping. `RECORDED := $(shell cat
$(REQUEST) 2>/dev/null)` reads the record at parse time and `ifneq
($(REQUESTED),$(RECORDED))` adds the phony `FORCE` to `$(OUT)/Image` only when
the request changed. The recipe writes `$(REQUEST)` beside `$(VARIANT)`, so the
record states what was BUILT. No mtime is compared for the variable triggers, so
the answer does not depend on timestamp granularity.

→ Decision: `FORCE` stays in the file, gated. The spec's shape said to delete it,
and what the shape was buying is the steady state answering "nothing to do". A
`FORCE` that is a prerequisite only when the request changed delivers that, and
an ungated `FORCE` does not. AC-1 is the test that tells the two apart.

→ Decision: R-3 is void, and `make -q` now changes nothing at all. The parse-time
write is gone, so no `-q` run creates `$(OUT)` or rewrites a stamp. The probes in
`kernel-compose.ci` are order-independent because of it.

**AC-2 needs no new test.** `kernel-compose.ci` already builds `PROFILE=qemu`
and then `PROFILE=compose-fixture` against the same output directory, and
asserts the second build produced `CONFIG_COMPOSE_FIXTURE=y`. Today that passes
because `FORCE` rebuilds unconditionally. With `FORCE` gone and `.request`
absent, the second `make` would do nothing and the assertion would go red. The
existing assertion becomes the discrimination proof for the stamp.

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
  → Constraint: lines 176-186 already build `PROFILE=qemu` then
  `PROFILE=compose-fixture` into the same scratch out-dir and assert the second
  build won. That pair is the profile-switch discrimination proof.
- [ ] `internal/appliance/cmd_iso.go` - `installerKernelBuildMatches` reads
  `build/kernel/.variant` and matches the prefix
  `<arch>-<profile>-<version>-`. → Constraint: `.variant` content is a product
  contract, not a build-system stamp.
- [ ] `test/install/kernel-version-single-reader.ci` - greps the three Makefiles
  for `cat[^|]*kernel\.version` and for `KVER|LINUX_VERSION`.
  → Constraint: no parse-time read of the version file.
- [ ] `test/install/kernel-runtime-deps.ci` - seven `make -q -C gokrazy/kernel`
  calls asserting `expect_status 0` and `expect_status 1`.
  → Decision: copy this assertion shape.
- [ ] `gokrazy/kernel/Makefile` - the sibling. `.PHONY: all clean`, no `FORCE`,
  pure file prerequisites. Already incremental and already tested for it.
- [ ] `tools/kernel-builder/build.py` - caches the built kernel tree as
  `linux-<version>-<modules>.built.tar`. → Constraint: a repeat build is minutes
  rather than a full kernel compile, so the cost of `FORCE` is real but bounded.

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
| A-1 | `FORCE` exists to force a rebuild when `PROFILE=` or `ARCH=` changes | `$(VARIANT)` records exactly arch, profile, version and builder, and no file prerequisite can express a variable change | design phase reads the commit that added `FORCE` | **broken** -- `30a24a874` put `FORCE` on `$(VARIANT)`, whose recipe rewrote the stamp only on a real change; `1e75b1911` deleted that rule and moved `FORCE` onto `$(OUT)/Image`, making the rebuild unconditional. The variable trigger was `$(VARIANT)`, never `FORCE` |
| A-2 | No other test depends on `make -q` against this Makefile | one grep over `test/` | design phase runs it | **confirmed** -- `grep -rn "make -q"` finds `test/install/kernel-runtime-deps.ci` only, and every one of its seven calls targets `gokrazy/kernel`, a different Makefile with no `FORCE`. `test/install/kernel-wiring.ci` uses `make -n -B`, and `-B` is unaffected by a prerequisite change |
| A-3 | Listing `internal/appliance/kernel.version` as a prerequisite does not count as a Makefile reading it | `test/install/kernel-version-single-reader.ci` greps `cat[^\|]*kernel\.version`; a bare path on a prerequisite line names the file without reading its contents | implementation runs that `.ci` | **confirmed** -- the path is held in `VERSION_FILE`, so no line carries both `cat` and `kernel.version`, and `kernel-version-single-reader` passes (install suite id 30) |
| A-4 | A stamp file listed as an ordinary prerequisite makes a changed `PROFILE=` / `ARCH=` / `BUILDER=` rebuild | make compares the stamp's mtime against the target's | implementation builds two profiles in a row and reads `.variant` | **broken** -- GNU Make 3.81, the make macOS ships, resolves mtimes to one second. A stamp written in the same second as the build reads as not newer, so the second build printed `Nothing to be done for 'all'` and kept the first profile's kernel. The trigger must be a parse-time COMPARISON, not an mtime. See Key Design Decisions, "Correction" |

### Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | `FORCE` is ungated and a profile switch silently reuses the previous kernel | the recipe records `<arch>-<profile>-<builder>` in `$(OUT)/.request`, and the parse-time `ifneq` re-gates `FORCE` when the request differs. `kernel-compose.ci` builds two profiles in a row and asserts the second one won: that assertion went red when the comparison was disabled (discrimination proof 1). **This risk MATERIALIZED once**, under the mtime-prerequisite shape A-4 records |
| R-2 | `FORCE` is gated and a version bump silently reuses the previous kernel | `../../internal/appliance/kernel.version` is an ordinary file prerequisite, and the last assertion in `kernel-compose.ci` gives it an explicit later timestamp and expects `make -q` to report work. Removing the prerequisite turns that assertion red (discrimination proof 2) |
| R-3 | The parse-time `$(shell ...)` runs under `make -q` and under `make clean` | **void.** The shipped mechanism reads `$(REQUEST)` and writes nothing at parse time, so `make -q` changes no file and `make clean` has nothing extra to remove |

## Blast Radius

`tools/installer-kernel/Makefile`, `test/install/kernel-compose.ci`, and any
build path that relies on the current always-rebuild behavior.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make -q -C tools/installer-kernel PROFILE=compose-fixture` straight after a successful build | -> | the `$(OUT)/Image` prerequisite list, with `FORCE` gone | `test/install/kernel-compose.ci`, the replacement for the `make -q` block |
| `make -C tools/installer-kernel PROFILE=<other>` after a build of a different profile | -> | `$(OUT)/.request` and its parse-time writer | the existing `CONFIG_COMPOSE_FIXTURE=y` assertion in the same `.ci`, which goes red without the stamp |
| `make -q` after touching `internal/appliance/kernel.version` | -> | the version file as an ordinary prerequisite | a new assertion in the same `.ci` |

## Acceptance Criteria

| AC | Input / Condition | Expected Behavior |
|----|-------------------|-------------------|
| AC-1 | A build has just succeeded and nothing has changed | `make -q` exits 0. It exits 1 today, because `FORCE` is a prerequisite of `$(OUT)/Image` |
| AC-2 | A build of one profile is followed by a build of another | the second build runs the recipe, and the existing `CONFIG_COMPOSE_FIXTURE=y` assertion proves it |
| AC-3 | `internal/appliance/kernel.version` is newer than the built image | `make -q` exits non-zero |
| AC-4 | A build has succeeded, and `make -q` is asked for a different `PROFILE=` | it exits non-zero |
| AC-5 | `build/kernel/.variant` after a build | unchanged content and unchanged writer, so `installerKernelBuildMatches` in `internal/appliance/cmd_iso.go` still matches |
| AC-6 | The existing fragment and composition assertions in `kernel-compose.ci` | unchanged and still green |

AC-1 replaces the AC this spec was written with ("the built image is removed and
make is asked whether work remains"). That AC encoded the vacuity itself:
`rm -f X; make -q` cannot fail for any Makefile. The replacement is strictly
stronger, and it is the only form that discriminates. See Key Design Decisions.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| - | - | the defect lives in a Makefile and a `.ci`, which no Go unit test reaches |

### Functional Tests

| Test | File | Validates |
|------|------|-----------|
| steady state is up to date | `test/install/kernel-compose.ci` | AC-1, proven to discriminate because it fails against the Makefile as it stands today |
| profile switch rebuilds | `test/install/kernel-compose.ci` | AC-2 and AC-4, proven to discriminate by deleting `$(OUT)/.request` from the prerequisite list and seeing the existing fixture assertion go red |
| version bump rebuilds | `test/install/kernel-compose.ci` | AC-3, proven to discriminate by removing the version file from the prerequisite list |
| single version reader | `test/install/kernel-version-single-reader.ci` | A-3: the Makefile still does not read `kernel.version` |
| gokrazy `-q` behaviour | `test/install/kernel-runtime-deps.ci` | A-2: unaffected, and the `expect_status 0` / `expect_status 1` pattern this spec copies |

## Files to Modify

- `tools/installer-kernel/Makefile` - delete `FORCE`, add the version file and `$(OUT)/.request` as prerequisites, add the parse-time stamp writer
- `test/install/kernel-compose.ci` - replace the vacuous `make -q` block with the steady-state, profile-switch and version-bump assertions
- `docs/guide/ze-install.md` - the `make -C tools/installer-kernel` section, which carries a `<!-- source: tools/installer-kernel/Makefile -- all -->` anchor and now describes an incremental build
- `docs/guide/appliance.md` - the same anchor at the kernel build section

## Files to Create

None. `$(OUT)/.request` is build output under the gitignored `build/`.

### Integration Checklist
- [ ] The chosen option is recorded in Key Design Decisions before any code
- [ ] Each new assertion is proven to fail with its trigger removed
- [ ] `.variant` content and its writer are untouched

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | the build was always meant to be incremental; this restores it. No entry in `docs/features.md` claimed the old behavior |
| 2 | Config syntax changed? | No | no YANG leaf, no config file, no parser is touched |
| 3 | CLI command added/changed? | No | `ze appliance kernel` keeps its flags and its output. Only `build/kernel/.request` handling changes, which no flag exposes |
| 4 | API/RPC added/changed? | No | no RPC, no wire method |
| 5 | Plugin added/changed? | No | `tools/` and `internal/appliance/` hold no plugin |
| 6 | Has a user guide page? | **Yes** | `docs/guide/appliance.md` and `docs/guide/ze-install.md`, both at the `<!-- source: tools/installer-kernel/Makefile -- all -->` anchor |
| 7 | Wire format changed? | No | no protocol code in the diff |
| 8 | Plugin SDK/protocol changed? | No | as row 5 |
| 9 | RFC behavior implemented, changed, or newly proven? | No | a kernel build system implements no RFC |
| 10 | Test infrastructure changed? | No | `test/install/kernel-compose.ci` is one test, not the harness. `expect_status` and `kmake` are local shell functions in that file, matching the shape `test/install/kernel-runtime-deps.ci` already uses |
| 11 | Affects daemon comparison? | No | not a daemon feature |
| 12 | Internal architecture changed? | No | `docs/architecture/appliance/kernel-profiles.md` describes profile resolution, which is untouched. `docs/architecture/appliance/build-artifacts.md` is named by the `// Design:` line in `cmd_kernel.go`: checked, it describes the cache and the artifacts, not the rebuild trigger |
| 13 | Route metadata keys added/changed? | No | no route code |
| 14 | Prometheus counters added/changed? | No | no metric |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | **Yes** | `grep -rn 'installer-kernel/Makefile' docs/ ai/` returns exactly the two anchors in row 6, both updated. `internal/appliance/cmd_kernel.go` and `cmd_kernel_test.go` carry no doc anchor pointing at them |
| 17 | Existing docs show config/CLI/API examples for this area? | **Yes** | the same two guides. Their `make -C tools/installer-kernel` and `bin/ze-setup appliance kernel` examples still run unchanged; the wrong output paths `build/Image` and `build/config` found in the same paragraphs were corrected to `build/kernel/...` |

## Implementation Steps

Implementation phases: ONE. The Makefile and the `.ci` cannot be verified apart: every new
assertion is about the Makefile's dependency graph, and the discrimination proof
edits the Makefile and re-runs the `.ci`.

| Step | Work | Verify |
|------|------|--------|
| 1 | Add the steady-state `make -q` assertion to `kernel-compose.ci` FIRST, and watch it FAIL against the current Makefile | the `.ci` goes red on the new assertion, not on an earlier one |
| 2 | Edit the Makefile: drop `FORCE` and its `.PHONY` entry, add `$(VERSION_FILE)` and `$(REQUEST)` prerequisites, add the parse-time `$(shell ...)` writer | the `.ci` goes green, the existing fixture assertion included |
| 3 | Add the version-bump and profile-switch `make -q` assertions | green |
| 4 | Prove discrimination three times: remove `$(REQUEST)` from the prerequisite list, remove `$(VERSION_FILE)`, restore `FORCE`. Each must turn a named assertion red | three recorded reds, each naming the assertion |
| 5 | Run the sibling gates: `kernel-version-single-reader.ci`, `kernel-runtime-deps.ci`, `kernel-wiring.ci`, `kernel-arch-mapping-single.ci`, `kernel-builder-single-driver.ci`, and the whole `install` suite | green |
| 6 | Update the two docs and their source anchors | `make ze-doc-test` |

### Critical Review Checklist
- [ ] `make -q` exits 0 in the steady state and non-zero for each of the three triggers
- [ ] Each new assertion was seen RED before the Makefile change that makes it green
- [ ] No existing assertion in `kernel-compose.ci` is changed, removed, or lowered, and the deleted `rm -f Image` block carries a `test-relax:` reason saying it was unfalsifiable
- [ ] `.variant` content, and the recipe line that writes it, are byte-identical
- [ ] The Makefile still does not read `kernel.version`
- [ ] The parse-time `$(shell ...)` is idempotent: two `make -q` runs in a row both exit 0

## Known Limitations

The install suite has no `make` target, so running this `.ci` needs a hand-built
isolated binary pair. That gap is
`plan/spec-functional-suites-missing-their-make-target.md`. The recipe, taken
from `ZE_ALT_BUILD` and `ZE_TEST_RUN` in `mk/test-functional.mk`:

```
FEATURES=$(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')
go build -tags "ze_core ze_distro ze_setup zetest $FEATURES" -o bin/kq-ze ./cmd/ze
go build -tags "ze_test $FEATURES" -o bin/kq-ze-test ./cmd/ze
env ZE_TEST_NO_BUILD=1 ZE_BIN=bin/kq-ze ZE_TEST_BIN=bin/kq-ze-test \
    bin/kq-ze-test install --all
```

`$(ZE_FEATURES)` is not optional. `ZE_ALT_BUILD` in `mk/test-functional.mk`
carries it, and this recipe first omitted it: six install tests then failed with
`no such module: ze-ddos-detect-conf`, which is the same symptom
`mk/test-integration.mk` records at its `ZE_QEMU_DUT_TAGS` comment. None of the
six touches the kernel.

`-o bin/...` is required: `.claude/hooks/pretool-bash.py` refuses a `go build`
that writes anywhere else. Delete both binaries when the run is done.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-6 each proven by a named test
- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix

### Quality Gates
- [ ] `make ze-verify`
- [ ] `make ze-lint-changed`

### Closure
- [ ] Deferral shard row closed

---

## Implementation Summary

### What Was Implemented
- `tools/installer-kernel/Makefile`: `FORCE` is no longer an unconditional
  prerequisite of `$(OUT)/Image`. `REQUESTED := $(ARCH)-$(PROFILE)-$(BUILDER)`
  and `RECORDED := $(shell cat $(REQUEST) 2>/dev/null)` are compared at parse
  time, and `ifneq` adds the phony `FORCE` to `$(OUT)/Image` only when the
  request differs from the record. `$(VERSION_FILE)`
  (`../../internal/appliance/kernel.version`), `$(MAKEFILE_LIST)` and
  `$(BUILDER_SCRIPTS)` (`$(wildcard $(BUILDER_DIR)/*.py)`, replacing four
  hand-written names) are ordinary prerequisites. The recipe writes
  `$(REQUEST)` beside `$(VARIANT)`.
- `internal/appliance/cmd_kernel.go`: `invalidateInstallerKernelRequest` deletes
  `build/kernel/.request`, and `resolveInstallerKernel` calls it. That path
  writes `build/kernel/Image` without running the Makefile, so a surviving
  record would describe an image the Makefile did not produce.
- `test/install/kernel-compose.ci`: the unfalsifiable `rm -f Image` plus
  `if make -q` block is replaced by the `expect_status` and `kmake` helpers and
  nine `make -q` probes: two steady-state probes, three single-variable probes
  (`PROFILE=`, `ARCH=`, `BUILDER=`), a steady-state re-probe, and one probe per
  file trigger (the version file, the Makefile, a builder script), each under a
  fixed-past-date scheme that makes exactly one prerequisite newer.
- `internal/appliance/cmd_kernel_test.go`: `TestKernelInvalidatesInstallerRequest`
  drives `resolveKernel` and asserts the record does not survive.
- `docs/guide/appliance.md` and `docs/guide/ze-install.md`: both sections that
  carry the `<!-- source: tools/installer-kernel/Makefile -- all -->` anchor
  now state that the build is incremental, name the rebuild triggers, and say
  that `ze appliance kernel` deletes the record when it writes the image.

### Bugs Found/Fixed
- The product defect the spec was written for: `$(OUT)/Image` listed the phony
  `FORCE` unconditionally, so every `make` rebuilt the kernel and every
  `make -q` answered "work to do". The AC-1 steady-state probe in
  `test/install/kernel-compose.ci` covers it, and that probe is red against the
  Makefile as it stands at HEAD.
- The mtime-granularity defect found during implementation: a stamp file used
  as an ordinary prerequisite loses a profile switch that happens inside the
  same second as the build it follows. See the Mistake Log, kind `assumption`,
  A-4.
- Three defects the review gate found in this diff, each a rebuild trigger the
  removal of the unconditional `FORCE` left with no replacement: the external
  writer (`resolveInstallerKernel`), the Makefile itself, and a builder module
  added to `tools/kernel-builder/`. See the Review Gate findings table.
- One defect in the first fix for the external writer: it printed a warning and
  carried on, so the record survived exactly when the image was being replaced.
  It now returns the error and the resolve stops. Review Gate finding 6.

### Documentation Updates
- `docs/guide/appliance.md`, the `ze appliance kernel` section carrying
  `<!-- source: tools/installer-kernel/Makefile -- all -->`: five sentences on
  the incremental build and `build/kernel/.request`.
- `docs/guide/ze-install.md`, the installer-kernel Makefile section carrying
  the same anchor: three sentences on the same facts.
- `make ze-doc-test` exit 0.

### Deviations from Plan
- The shape in Key Design Decisions listed `$(OUT)/.request` as an ordinary
  file prerequisite. What ships compares the record at parse time instead. The
  spec records the correction in place ("Correction, made during
  implementation"); A-4 carries the evidence.
- `FORCE` is not deleted. It stays in the file, gated by the `ifneq`. What the
  deletion was buying is the steady state answering "nothing to do", and a
  gated `FORCE` delivers exactly that.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: `FORCE` was taken to be the `PROFILE=` / `ARCH=` rebuild trigger, so the spec framed the fix as "replace what `FORCE` does" | `30a24a874` put `FORCE` on a separate `$(VARIANT): FORCE` rule whose recipe rewrote the stamp only on a real content change, and `$(OUT)/Image` took `$(VARIANT)` as an ordinary file prerequisite. `1e75b1911` deleted that rule and moved `FORCE` onto `$(OUT)/Image`, converting a change-triggered rebuild into an unconditional one. Its commit message does not mention it: the change is collateral of replacing the shell recipe with a Python builder | design phase read the two commits that touched the `FORCE` line | R-1's premise removed, the fix re-establishes the change-triggered rebuild rather than inventing one, and a row is added to `plan/journal/refactor-removes-feature.md` |
| assumption | A-4: the design specified `$(OUT)/.request` as an ordinary file prerequisite, on the assumption that make compares its mtime against the target | GNU Make 3.81, the make Apple ships and the make this machine runs, resolves mtimes to one second. Measured in the scratch tree: `.request` at `1786312764.639256556` against `Image` at `1786312764.185367635`, and the second build printed `make: Nothing to be done for 'all'` while `.variant` still held `amd64-qemu-7.1.4-docker`. The profile switch was silently lost, which is R-1 happening | implementation phase ran the discrimination proof and read `.variant` after the second build | the mtime comparison is replaced by a parse-time string comparison, and a row is added to `plan/journal/mtime-granularity-stamp.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| the `make -q` assertion must be able to fail | Done | `test/install/kernel-compose.ci`, the `expect_status 0 kmake -q ARCH=amd64 PROFILE=compose-fixture BUILDER=docker` probe | red against the Makefile at HEAD, where `FORCE` is unconditional |
| the rebuild trigger for a changed `PROFILE=` / `ARCH=` / `BUILDER=` must not be weakened | Done | `tools/installer-kernel/Makefile`, `REQUESTED` / `RECORDED` / the `ifneq` block | the three single-variable `expect_status 1` probes in `test/install/kernel-compose.ci` |
| `.variant` content and its writer stay unchanged | Done | `tools/installer-kernel/Makefile`, the `@ver=$$(sed -n ...)` recipe line | byte-identical to the same line in `git show HEAD:tools/installer-kernel/Makefile` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/install/kernel-compose.ci`, three `expect_status 0 kmake -q` probes | the first, its immediate repeat, and the re-probe after the `expect_status 1` group |
| AC-2 | Done | `test/install/kernel-compose.ci`, the `PROFILE=qemu` then `PROFILE=compose-fixture` build pair and its `grep -q '^CONFIG_COMPOSE_FIXTURE=y$'` | goes red when the request comparison is disabled |
| AC-3 | Done | `test/install/kernel-compose.ci`, the version file dated `202601030000` against an image dated `202601020000`, then `expect_status 1` | |
| AC-4 | Done | `test/install/kernel-compose.ci`, one `expect_status 1` probe per variable | |
| AC-5 | Done | `tools/installer-kernel/Makefile` `$(VARIANT)` recipe line, and `installerKernelBuildMatches` in `internal/appliance/cmd_iso.go` | the recipe line is byte-identical to HEAD |
| AC-6 | Done | `test/install/kernel-compose.ci`, the `require_enabled` / `require_absent` loops and the two build assertions | unchanged |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| steady state is up to date | Done | `test/install/kernel-compose.ci`, `expect_status 0 kmake -q` | AC-1 |
| profile switch rebuilds | Done | `test/install/kernel-compose.ci`, the build pair and the `PROFILE=qemu` probe | AC-2 and AC-4 |
| version bump rebuilds | Done | `test/install/kernel-compose.ci`, the `touch -t` probe | AC-3 |
| single version reader | Done | `test/install/kernel-version-single-reader.ci` | unchanged, still green |
| gokrazy `-q` behaviour | Done | `test/install/kernel-runtime-deps.ci` | unchanged, targets a different Makefile |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `tools/installer-kernel/Makefile` | Changed | `FORCE` gated rather than deleted; see Deviations |
| `test/install/kernel-compose.ci` | Done | |
| `docs/guide/ze-install.md` | Done | |
| `docs/guide/appliance.md` | Done | |
| `internal/appliance/cmd_kernel.go` | Changed | not in the plan. Added by review finding 1, which is a rebuild trigger this diff would otherwise have removed |
| `internal/appliance/cmd_kernel_test.go` | Changed | not in the plan. The regression tests for findings 1, 6 and 7 |

### Audit Summary
- **Total items:** 20
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (`tools/installer-kernel/Makefile` in Deviations, and the two `internal/appliance` files the Review Gate added)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| the `make -q` assertion in `kernel-compose.ci` can fail | functional | the `expect_status 0 kmake -q ARCH=amd64 PROFILE=compose-fixture BUILDER=docker` probe is red against the Makefile as it stands at HEAD, where `FORCE` is an unconditional prerequisite (discrimination proof 3) |
| a removed rebuild trigger is caught | functional | proof 1: with the `ifneq` comparison disabled, the existing `CONFIG_COMPOSE_FIXTURE=y` assertion goes red. Proof 2: with `$(VERSION_FILE)` removed from the prerequisite list, the `touch -t` probe goes red |
| the kernel build is incremental and the operator can rely on it | functional | the whole `install` suite, 37 tests, 37 pass with 3 pre-existing skips, run with the isolated binary pair from Known Limitations |
| `ze install ... iso` still finds a matching build | functional | `.variant` is written by a recipe line byte-identical to HEAD and read by `installerKernelBuildMatches` in `internal/appliance/cmd_iso.go`, which this diff does not touch |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `kernel-compose.ci` deletes the built image then asserts `make -q` reports work to do, which cannot fail while `$(OUT)/Image` lists the phony `FORCE` | done | this spec. `FORCE` is gated by the `ifneq` in `tools/installer-kernel/Makefile`, the block is replaced by the `expect_status` probes in `test/install/kernel-compose.ci`, and the AC-1 probe is red against the Makefile at HEAD |

The shard's only row is terminal, so this closure removes the shard. No foreign
shard names this spec and none is emptied by this resolution:
`grep -rln 'kernel-compose-make-q' plan/` returns the spec and its own shard
only.

Three defects found by the review gate are NOT homed here. The owner's
instruction of 2026-08-10 replaces the write-a-spec route with "fix it KISS, or
say so and leave it", and each of these is outside the files this spec owns:

| Found | Why it is left | Where it is |
|-------|----------------|-------------|
| `gokrazy/kernel/Makefile` selects its build with `ARCH ?= amd64` and `BUILDER ?= docker` while the `$(OUT)/vmlinuz` prerequisite list names files only, so `make ARCH=arm64` after an amd64 build reuses the amd64 `vmlinuz` | the same class this spec fixed in the sibling Makefile, but in a second Makefile with a second test file. Real, and a product defect | reported to the owner at closure |
| `resolveInstallerKernel` replaces `build/kernel/Image` and never updates `build/kernel/.variant`, which `installerKernelBuildMatches` reads, so the record can claim one profile over another profile's image | pre-existing, and this diff does not widen it: the `.request` deletion makes the next `make` rebuild and rewrite `.variant`. Deleting `.variant` too would disable the `installerKernelFallbackPath` route, which is a design question rather than a small edit | reported to the owner at closure |
| `internal/test/runner/record_parse.go` fails staticcheck ST1005 at HEAD, so `make ze-lint-changed` exits non-zero for every session | one character, but in a file this spec does not own, from commit `2cff2050a` | reported to the owner at closure |
| `kernelCacheVariantFor` in `internal/appliance/cache.go` hashes the hand-named builder list `build.py`, `run.py`, `ksource.py`. `qemu-build.py` is absent, so editing the QEMU backend does not invalidate the `ze appliance kernel` cache key. The same file's `initrdSourceDirs` comment records this exact lesson for the initrd | tried as a KISS glob and REVERTED. Round 4 found the fix had no regression test (every fixture in `cmd_kernel_test.go` writes only the old three names, so reverting the fix leaves the package green) and that the glob newly reaches a fallback which discards the CONFIG hash as well. `cache.go` is byte-identical to HEAD | reported to the owner at closure |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/kernel-compose-make-q-assertion-is-vacuous-86ae859e-de6d-4ba5-88c2-2d92e836363b.md` |
| `review_gate.py check` | clean. `review_gate: OK (4 code files, clean, hashes match ...)` |
| Rounds | 4. Rounds 1 and 2 found the nine defects below and round 3 reported 0 BLOCKER and 0 ISSUE. Round 4 was earned by a PRODUCT defect round 3 found outside the diff: `kernelCacheVariantFor` in `internal/appliance/cache.go` hashes a hand-named builder list that omits `qemu-build.py`, so editing the QEMU backend does not invalidate the `ze appliance kernel` cache key. Round 4 reviewed the fix for it, found the fix had no regression test, and that is why `cache.go` was reverted to HEAD and the defect reported rather than shipped unproven |
| Reviewer lenses used | A: GNU Make semantics, logic, removed-behavior audit. B: documentation accuracy, simplicity, security, writing style. C: the Go contract and its guard. D: the Makefile and functional-test fixes. E: the round-2 fixes. F: the `cache.go` fix |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `ze appliance kernel` writes `build/kernel/Image` without running the Makefile, so the record survives and describes an image the Makefile did not produce. `make PROFILE=qemu` then reports nothing to do while another profile's kernel sits at the output path. The unconditional `FORCE` covered this before the change | `resolveInstallerKernel` in `internal/appliance/cmd_kernel.go` | `invalidateInstallerKernelRequest` deletes the record on every resolve, so the next make rebuilds. `TestKernelInvalidatesInstallerRequest` proves it, and fails with the call removed |
| 2 | ISSUE | The Makefile is not a prerequisite of its own output. Its recipe names the profile, the arch, the builder and the `run.py` argument list, so editing it left a stale image with a matching record. The unconditional `FORCE` covered this too | the `$(OUT)/Image` prerequisite list in `tools/installer-kernel/Makefile` | `$(MAKEFILE_LIST)` added to that list, with a probe in `kernel-compose.ci` that goes red without it |
| 3 | ISSUE | The builder modules were named one by one, so a module added to `tools/kernel-builder/` would stop triggering a rebuild | the same prerequisite list | `$(BUILDER_SCRIPTS) := $(wildcard $(BUILDER_DIR)/*.py)` replaces the four names, with a probe that goes red without it. Behavior-identical today: the glob matches exactly `build.py`, `ksource.py`, `qemu-build.py`, `run.py` |
| 4 | ISSUE | `touch -t 202701010000` on the version file works only while that date is in the future. From 2027-01-01 the suite would go red on the calendar rather than on a defect | the AC-3 probe in `test/install/kernel-compose.ci` | fixed past dates: every prerequisite to `202601010000`, the image to `202601020000`, and one file at a time to `202601030000`, restored after each probe |
| 5 | ISSUE | The Makefile header named `build/Image`, `build/.variant` and `build/.request`, but `OUT := ../../build/kernel`, so every path was wrong. The diff extended the error to `.request` while the two guides it also edited used the correct path | the header comment in `tools/installer-kernel/Makefile` | all five paths corrected to `build/kernel/...` |

| 6 | BLOCKER | The record deletion failed OPEN. On any `os.Remove` error other than "not exist" it printed a warning and the resolve continued, replacing the image and returning success, which is exactly the state the deletion exists to prevent. Reachable when `build/kernel` is left unwritable by a docker builder run, or when `.request` is a non-empty directory | `invalidateInstallerKernelRequest` in `internal/appliance/cmd_kernel.go` | it returns the error and `resolveInstallerKernel` returns before any copy. `TestKernelUndeletableRequestStopsResolve` drives the failing path from `resolveKernel` with a non-empty directory at the record path, which `os.Remove` refuses for every user, root included |
| 7 | ISSUE | The regression test set a fresh `XDG_CACHE_HOME`, so only the BUILD branch ran. Moving the deletion next to `buildKernelArtifact` kept it green while the cache-hit branch, the common repeat path, leaked the record | `TestKernelInvalidatesInstallerRequest` in `internal/appliance/cmd_kernel_test.go` | a two-case table drives `build` and `cache-hit`. Measured: with the call moved next to the build, the `cache-hit` case fails |
| 8 | ISSUE | The builder-module probe touched `run.py`, which the four hand-written names covered too, so it could not tell the glob from the list it replaced | the builder-module probe in `test/install/kernel-compose.ci` | it creates a new `newmod.py` the old list never named. Measured: with `$(BUILDER_SCRIPTS)` reverted to the four names, the probe fails |
| 9 | ISSUE | `docs/guide/ze-install.md` was corrected to `build/kernel/Image` three lines below while the same paragraph still read "reads the emitted `build/config`" | the `ze appliance kernel` paragraph in `docs/guide/ze-install.md` | corrected to `build/kernel/config`, against `kernelInstallerOutputDir` and the `enforceKernelRequirements` call in `internal/appliance/cmd_kernel.go` |

NOTEs recorded and acted on: `.request` was spelled in two languages with no
gate, so `kernel-compose.ci` now asserts the record by name and content after a
build. Both guides and the Makefile comment said the record is deleted "when it
writes the image", while the code deletes it on every run, before any write:
the sentences now say what the code does.

NOTEs recorded and not acted on: the `$(VARIANT)` recipe line does not check
that `sed` produced a version, so a malformed `build/kernel/kernel.version`
would write `amd64-qemu--docker` and no later make would repair it (the
unconditional `FORCE` used to). The fix is a guard on that line, and this spec's
Critical Review Checklist requires the line to stay byte-identical, so it is not
taken here. `tools/installer-kernel/README.md` lists three build outputs and
mentions neither sidecar. Both are pre-existing and neither blocks this goal.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `tools/installer-kernel/Makefile` | yes | `git status --porcelain` reports ` M tools/installer-kernel/Makefile` |
| `test/install/kernel-compose.ci` | yes | `git status --porcelain` reports ` M test/install/kernel-compose.ci` |
| `docs/guide/appliance.md` | yes | ` M docs/guide/appliance.md` |
| `docs/guide/ze-install.md` | yes | ` M docs/guide/ze-install.md` |
| `test/install/kernel-version-single-reader.ci` | yes | read in full during closure; unchanged by this diff, and it passes as install suite id 30 |
| `test/install/kernel-runtime-deps.ci` | yes | named in the TDD plan; unchanged by this diff, and it passes as install suite id 26 |
| `internal/appliance/cmd_kernel.go` | yes | `git status --porcelain` reports ` M internal/appliance/cmd_kernel.go`; the package was otherwise clean |
| `internal/appliance/cmd_kernel_test.go` | yes | ` M internal/appliance/cmd_kernel_test.go` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `make -q` exits 0 in the steady state | `test/install/kernel-compose.ci` carries `expect_status 0 kmake -q ARCH=amd64 PROFILE=compose-fixture BUILDER=docker`, and the install suite passes 37/37 |
| AC-2 | a profile switch rebuilds | the second `make` in `test/install/kernel-compose.ci` and its `grep -q '^CONFIG_COMPOSE_FIXTURE=y$'` |
| AC-3 | a newer `kernel.version` is work to do | every prerequisite dated `202601010000` and the image `202601020000`, then the version file alone to `202601030000` and `expect_status 1`. Proven to discriminate: with `$(VERSION_FILE)` off the prerequisite list the suite fails with `wanted exit 1, got 0` |
| AC-4 | a different `PROFILE=`, `ARCH=` or `BUILDER=` is work to do | three `expect_status 1 kmake -q` probes, one per variable |
| AC-5 | `.variant` content and writer unchanged | the `@ver=$$(sed -n 's/^version=//p' ...)` line is the same bytes at HEAD and in the worktree; `installerKernelBuildMatches` in `internal/appliance/cmd_iso.go` reads `.variant` and is untouched |
| AC-6 | every other assertion is unchanged | `python3 scripts/dev/audit-test-relaxation.py`: 0 deleted, 0 weakened, 1 documented relaxation naming the replaced coverage |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make -q -C tools/installer-kernel PROFILE=compose-fixture` after a build | `test/install/kernel-compose.ci` | yes: `kmake` runs `make -C "$scratch/tools/installer-kernel"` with the fake docker on PATH, and the first probe asserts exit 0 |
| `make -C tools/installer-kernel PROFILE=<other>` after a build of another profile | `test/install/kernel-compose.ci` | yes: `PROFILE=qemu` builds first, `PROFILE=compose-fixture` builds into the same out-dir, and the fixture grep asserts the second won |
| `make -q` after touching `internal/appliance/kernel.version` | `test/install/kernel-compose.ci` | yes: the `202601030000` probe on the version file |
| `make -q` after editing the Makefile, and after editing a builder module | `test/install/kernel-compose.ci` | yes: one probe each, both proven red with their prerequisite removed |
| `ze appliance kernel` writing `build/kernel/Image`, then `make -q` | `internal/appliance/cmd_kernel_test.go`, `TestKernelInvalidatesInstallerRequest` | yes: it drives `resolveKernel`, the production entry point, not the helper. Red with the invalidation removed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `30a24a874` and `1e75b1911` read in the design phase; Mistake Log row 1 |
| A-2 | confirmed | `grep -rn "make -q" test/` finds `test/install/kernel-runtime-deps.ci` only, and all seven of its calls target `gokrazy/kernel` |
| A-3 | confirmed | `VERSION_FILE := ../../internal/appliance/kernel.version` holds the path, so no line carries both `cat` and `kernel.version`, and `test/install/kernel-version-single-reader.ci` passes |
| A-4 | broken | GNU Make 3.81 one-second mtime resolution; Mistake Log row 2 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "The kernel build is incremental" (`docs/guide/appliance.md`) | the `$(OUT)/Image` prerequisite line in `tools/installer-kernel/Makefile`, which carries no unconditional `FORCE` | yes |
| "rebuilds when the requested arch, profile, or builder is different from the last build, which it records in `build/kernel/.request`" (both guides) | `REQUESTED`, `RECORDED` and the `ifneq` block, and the `@echo "$(REQUESTED)" > $(REQUEST)` recipe line | yes |
| "rebuilds when ... the Makefile, or `internal/appliance/kernel.version` is newer" (both guides) | `VERSION_FILE` and `$(MAKEFILE_LIST)`, both on the `$(OUT)/Image` prerequisite line | yes |
| "`ze appliance kernel` ... deletes `build/kernel/.request` when it writes `build/kernel/Image`" (both guides) | `invalidateInstallerKernelRequest`, called by `resolveInstallerKernel` in `internal/appliance/cmd_kernel.go` | yes |
| every anchor and doc claim in the repository | `make ze-validate` exit 0. `make ze-doc-test` was exit 0 after these doc edits and went red later on `ai/CODE-TO-DOCS.md` and `ai/RFC-REQUIREMENTS.md` being stale. Neither is this diff: `git diff -- docs/guide/appliance.md docs/guide/ze-install.md \| grep '<!-- source:'` returns one anchor delta and it is the foreign `imageserver` hunk, and no `rfc/` path is in the diff | yes |
| CLI reference, API/RPC, plugin SDK, wire format, RFC status, comparison table | no CLI, RPC, plugin, wire, or protocol surface is in the diff: the changed files are one Makefile, one `.ci`, and two guides | not applicable |

## Core Insight

A rebuild trigger for a value that is not a file cannot be a file. Every attempt
to make it one fails on the same edge: the stamp and the artifact it guards are
written by the same recipe, in the same second, and one second is the resolution
GNU Make 3.81 has. The trigger has to be a comparison make makes while it
parses, before any mtime exists to compare.
