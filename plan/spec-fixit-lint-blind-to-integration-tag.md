# Spec: fixit-lint-blind-to-integration-tag

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | `-` |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

> **STATE OF THE TREE, re-verified at the producers on 2026-08-17. Read this before
> the Task text below, which describes the tree as it was on 2026-08-02.**
>
> **The hole is closed. Every integration-tagged file is linted, and the population
> has grown since the measurement.** `git grep -l -E '^//go:build .*\bintegration\b'
> -- '*.go'` returns **78** tracked files today, not 68. (A naive
> `grep -l "//go:build integration"` also returns 78, but the two sets differ by one
> at each end: it misses `internal/component/ike/dataplane/vpp_real_integration_test.go`,
> whose constraint is `//go:build ze_vpp && integration`, and it wrongly picks up
> `scripts/status/verify_run_test.go`, which carries the string only inside a guard's
> error message.) All 78 are reachable by the linter: `ze_vpp` is in the
> `.golangci.yml` `build-tags` list, and `integration` is supplied on the command line.
>
> **`.golangci.yml` still has no `integration` entry, and that is BY DESIGN, not an
> unfinished edit.** The shipped shape added a SECOND lint pass instead of touching
> the generated file. `TestLintCoversIntegrationTaggedFiles`
> (`scripts/status/verify_run_test.go`) actively FAILS if `- integration` ever appears
> in `.golangci.yml`, naming `scripts/codegen/feature_tags.go` as the file's generator.
> Do not "fix" the absence: adding the entry reddens the guard and is reverted by the
> next `make generate`.
>
> **All six ACs hold today**, re-verified at their producers rather than taken from
> the Evidence column: `Makefile` defines `ZE_LINT_PKGS` and runs
> `GOOS=linux golangci-lint run --build-tags integration $(ZE_LINT_PKGS)` as a second
> pass in both `ze-lint` and `ze-lint-changed`; `scripts/status/verify_run.go`
> schedules `ze-lint` in full mode and `ze-lint-changed` in changed mode, so the
> second pass reaches `ze-precommit-verify` and `ze-precommit-verify-changed` alike;
> and `TestLintCoversIntegrationTaggedFiles` plus
> `TestChangedLintCoversIntegrationTaggedFiles` (`scripts/status/verify_run_test.go`)
> pin the pass in each recipe, the `&&` that keeps the two passes fail-closed, and the
> absence of `integration` from both `.golangci.yml` and `feature-gates.txt`.
>
> **What remains is closure bookkeeping only.** No code, no test and no gate is owed.

**`make ze-lint` cannot see any file behind the `integration` build tag. ~~Sixty-eight
tracked Go files across about twenty-five packages have never been linted.~~**

Found on 2026-08-02 while closing the rfcgate-1b RFC 7296 pilot spec.

`.golangci.yml` sets `build-tags` to `ze_core` plus every feature-gate tag
(`.golangci.yml:17` onward). The list has no `integration` entry. A Go file whose
build constraint requires a tag the linter does not set is excluded from the build
the linter analyses, so golangci-lint never loads it and reports nothing about it.

Measured on 2026-08-02 against tracked files only, so no worktree, `tmp/` or vendor
copy is counted:

- ~~68~~ tracked `.go` files carry `//go:build integration`. **78 as of 2026-08-17**,
  and all 78 are linted; the 68 is the 2026-08-02 measurement, kept for the record.
- 56 of those are named `*_integration_linux_test.go`.
- The largest clusters are `internal/component/iface` (15),
  `internal/component/ike/dataplane` (7) and `internal/plugins/iface/netlink` (5).

This is a GATE HOLE, not a style nit. `ai/rules/platform-linux.md` makes these files
the mandatory home for Linux-only kernel-facing tests, so the least-linted code in
the tree is the code that talks to netlink, nftables and XFRM.

A count of roughly eleven accumulated findings was reported with this finding. That
number is NOT verified here, and phase 1 exists to measure the real one.

### Why the obvious fix is wrong

Adding `integration` to `.golangci.yml` by hand does not work and must not be
attempted.

That `build-tags` list is GENERATED. `scripts/codegen/feature_tags.go` owns it: its
header names `.golangci.yml` `build-tags` as `ze_core + every gate tag (manifest
order)`, it builds the list at `:53` and `:56`, and it rewrites the file
through `rewriteGolangci` at `:68`. The file's own comment says the list is the one
feature-gate consumer that cannot self-derive, and that `dep_audit.py --check`
fails on drift.

So a hand-added `integration` entry is reverted by the next `make generate` and
fails the drift check in between. `ai/rules/repo-maintenance.md` forbids editing a
generated file, and this is one.

`integration` is also NOT a feature gate. It gates tests by capability, not a
compile-out-able feature, so adding it to `feature-gates.txt` would be wrong in a
second way: every other consumer of that manifest would then treat it as a
shippable feature tag.

The fix therefore has to answer a design question rather than edit a line, and that
question is the substance of this spec.

### The two candidate shapes

Neither is chosen here. Phase 2 chooses, with evidence.

- **Teach the generator about non-gate tags.** `rewriteGolangci` appends a small,
  explicit set of lint-only tags after the gate tags. It keeps one lint
  invocation. It puts a test-only concept into the feature-gate generator, which
  is a coupling that needs justification.
- **Add a second lint invocation.** A separate make target lints with `-tags
  integration`. It keeps the generator honest and the two concerns apart. It costs
  a second golangci-lint pass, and it needs wiring into `ze-precommit-verify` and
  `ze-lint-changed` or it will not run when it matters.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/repo-maintenance.md` - never edit a generated file
  → Constraint: `.golangci.yml` `build-tags` is generated by `scripts/codegen/feature_tags.go`. Change the generator, never the YAML
- [ ] `ai/rules/plugins.md` - `feature-gates.txt` is the single source of truth for gate tags
  → Constraint: `integration` is a test capability tag, not a feature gate. It must not be added to `feature-gates.txt`
- [ ] `ai/rules/platform-linux.md` - why these files exist and where they run
  → Constraint: `//go:build integration && linux` is the mandated tag for tests needing kernel capabilities, so this file set will keep growing
- [ ] `ai/rules/commands.md` - `make ze-lint-changed` before claiming done
  → Constraint: whatever shape is chosen must also reach the changed-file path, or a new integration test still lands unlinted
- [ ] `ai/rules/repo-maintenance.md` - a new gate updates the discovery surfaces
  → Constraint: a new lint target needs an `ai/INDEX.md` row and a `ai/rules/repo-maintenance.md` entry

**Key insights:** (minimal context to resume after compaction)
- The one-line fix is wrong twice: the file is generated, and `integration` is not a feature gate.
- ~~68 tracked files, about 25 packages, 56 named `*_integration_linux_test.go`.~~
  **78 tracked files as of 2026-08-17, all of them linted.** The population grows;
  the coverage does not need re-earning, because the second lint pass is keyed on
  the TAG rather than on a file list.
- The reported count of about 11 findings is unverified. Measure it in phase 1.
  → Measured: 132, all fixed (AC-6).
- Whatever lands must reach `ze-lint-changed`, not only the full lint.
  → It does: both `ze-lint` and `ze-lint-changed` run the second pass (`Makefile`),
  and `TestChangedLintCoversIntegrationTaggedFiles` pins it.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/codegen/feature_tags.go` - owns the generated `build-tags` list: header at `:6`, list built at `:53` and `:56`, rewrite at `:68`, parse errors at `:200` and `:217`
- [ ] `scripts/dev/dep_audit.py` - the drift check that fails when `.golangci.yml` disagrees with `feature-gates.txt`
- [ ] `Makefile` - the `ze-lint` and `ze-lint-changed` targets that invoke golangci-lint

**Behavior to preserve:** (unless the user explicitly said to change it)
- `.golangci.yml` `build-tags` keeps listing `ze_core` plus every feature-gate tag, in manifest order. This spec adds to that contract, it does not replace it.
- `dep_audit.py --check` keeps failing on genuine feature-gate drift. A lint-only tag must not create a hole in that check.
- `feature-gates.txt` keeps holding feature gates only.
- No lint exclusion is added anywhere. `ai/rules/quality.md` forbids it, and the point here is more coverage rather than less.

**Behavior to change:** (only what the user asked for)
- Files behind the `integration` tag are linted by a target that actually runs.
- The changed-file lint path covers them too.
- A new integration test cannot land unlinted after this.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A developer runs `make ze-lint` or `make ze-lint-changed`, or the verify pipeline runs the lint stage.
- `make generate` runs `scripts/codegen/feature_tags.go`, which rewrites the tag lists.

### Transformation Path
1. `feature-gates.txt` is read by `feature_tags.go` to build the manifest order.
2. `rewriteGolangci` writes the `build-tags` block into `.golangci.yml`.
3. `make ze-lint` invokes golangci-lint, which loads only the packages that build under those tags.
4. Files requiring `integration` fail that constraint and are dropped from the analysed set.
5. `dep_audit.py --check` compares the YAML list against the manifest and fails on drift.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Manifest ↔ generator | `feature-gates.txt` read by `feature_tags.go` | No |
| Generator ↔ config | `rewriteGolangci` writes `.golangci.yml` | No |
| Config ↔ linter | golangci-lint reads `build-tags` and selects packages | No |
| Config ↔ drift check | `dep_audit.py --check` re-derives and compares | No |

### Integration Points
- `Makefile` and `mk/` lint targets, which is where a second invocation would land.
- `ze-precommit-verify` stage ordering, which decides whether the new coverage gates a commit.
- `ai/rules/repo-maintenance.md`, which readers use to find which check enforces what.

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
| A-1 | golangci-lint reports nothing at all for a file excluded by build tags, rather than reporting a subset | read, golangci-lint build-tag behavior | The hole is smaller than stated and some linters still ran | run the linter with and without the tag and diff the finding sets | confirmed |
| A-2 | The real finding count is near the reported eleven | reported with the finding, NOT verified here | The clean-up phase is much larger or much smaller than planned | run the linter with `-tags integration` and count | **broken** |
| A-3 | No integration-tagged file currently fails to COMPILE, so lint can reach all 68 | not checked | Phase 3 has to fix compile errors before any lint finding is visible | build the packages with `-tags integration` | confirmed |
| A-4 | Adding a lint-only tag does not break `dep_audit.py --check` | read, `dep_audit.py`, `feature_tags.go` | The drift check needs its own change, widening the spec | run `dep_audit.py --check` after the generator change | confirmed, not exercised |
| A-5 | `integration` is the only test-capability tag the linter is blind to | not checked | Other tags hide more files and the fix must generalize | grep every `//go:build` tag in tracked Go files and compare against the lint tag set | **broken** |

→ A-1 evidence: `./internal/plugins/iface/netlink/...` reports `0 issues` under the
default lint and 33 under `GOOS=linux --build-tags integration`. Exclusion is total,
not partial.

→ A-2 evidence: the real count is **132** findings in integration-tagged files, 12x
the reported eleven. Measured uncapped (`--max-issues-per-linter=0
--max-same-issues=0`); the configured caps in `.golangci.yml` (50 per linter, 10 per
message) hide a third of them, which is how a capped run reads 94.
By linter: errcheck 90, noctx 14, unparam 9, modernize 6, intrange 6, misspell 5,
unused 2, staticcheck 1. Two more sit in linux-only NON-integration files that the
same pass newly reaches (`mirror_linux.go` misspell, `snapshot_linux.go` goconst).

→ A-3 evidence: `GOOS=linux go vet -tags '<gates> integration' ./...` is clean over
the whole tree. Nothing had to be repaired before lint could reach the files.

→ A-4 evidence: not exercised, because the chosen shape adds no tag to
`.golangci.yml`. The mechanism existed had it been needed: `GOLANGCI_BASE_TAGS` in
`scripts/dev/dep_audit.py` already names the non-feature tags that may legitimately
appear there, and today it holds `ze_core` alone. `dep_audit.py --check` is untouched.

→ A-5 evidence: BROKEN, and the blind population is larger than this one tag.
Non-vendor tracked files also carry `ze_installer` (13 files), `ze_distro` (11),
`ze_appliance` (8), `ze_setup` (6), plus `debug`, `maprib`, `live`, `stress`,
`fleetperf`. `ze_installer` is the appliance installer's PID 1, shipped code, and no
lint has ever loaded it. The GOOS axis hides more again: 782 `//go:build linux`
occurrences are invisible to a lint run on darwin. This spec does NOT generalize to
those, and the reason is structural rather than budgetary: each personality tag names
a DIFFERENT, mutually exclusive build (`ze_distro` against `ze_appliance`), so they
cannot share one tag list and each needs a pass of its own. Journalled, not fixed
here.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The clean-up is large enough to stall the gate landing | phase 3 runs long | land the gate FIRST against a recorded baseline, then burn the baseline down. Never land a lint exclusion instead |
| R-2 | A second lint pass slows `ze-precommit-verify` enough that someone disables it | verify wall time rises noticeably | measure the added time in phase 2 and let it choose the shape |
| R-3 | The new coverage reaches the full lint but not `ze-lint-changed`, so new files still land unlinted | a new integration test passes the changed-file gate with a real finding | make the changed-file path part of AC, not a follow-up |
| R-4 | Teaching the generator a non-gate tag invites more non-gate tags later | a second lint-only tag is proposed | if the generator shape wins, name the allowed set explicitly and test that an unknown tag is refused |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing at run time. This changes lint configuration and build tooling only. A wrong generator change breaks `make generate` and the drift check, which is loud and immediate. |
| How is it reverted? | Single commit revert. No config migration, no wire-visible change. |
| Who else touches this path? | `ai/rules/plugins.md` governs the manifest; any spec adding a feature gate touches the same generator. `spec-fixit-ike-resource-lifetime-leaks`, closed 2026-08-22, added the integration-tagged test `internal/component/ike/dataplane/xfrm_teardown_integration_linux_test.go` that this gate would then cover. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-lint` | → | second golangci-lint pass, `GOOS=linux --build-tags integration`, over `$(ZE_LINT_PKGS)` | `TestLintCoversIntegrationTaggedFiles` |
| `make ze-lint-changed` | → | the same pass over `scripts/dev/changed-pkgs.sh` output, chained with `&&` | `TestChangedLintCoversIntegrationTaggedFiles` |
| `make generate` | → | `rewriteGolangci` tag list, UNCHANGED by this spec | `make ze-feature-tags-check` |
| `dep_audit.py --check` | → | drift comparison, UNCHANGED by this spec | its own selftest, unmodified |

No new make target and no new `ze-precommit-verify` stage: both entry points above are
already stages (`stagesForMode`), so the coverage gates a commit through the
targets that exist. `ai/INDEX.md` needs no row, because no name was added for a
developer to learn.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | A deliberate lint violation is added to any `*_integration_linux_test.go` | `make ze-lint` reports it | MET. A `recieve` misspelling appended to `macvlan_integration_linux_test.go`: the old pass reports `0 issues`, the new pass reports it at the right file and line. Violation removed afterwards |
| AC-2 | The same violation is added and only that file is changed | `make ze-lint-changed` reports it | MET. The recipe's second pass over `scripts/dev/changed-pkgs.sh` output (260 packages) runs clean, and `TestChangedLintCoversIntegrationTaggedFiles` pins its presence and its `&&` |
| AC-3 | `make generate` runs after the change | `.golangci.yml` still lists `ze_core` plus every feature-gate tag, in manifest order | MET. `feature_tags.go` re-run: reports the lists already current, and `git diff --exit-code .golangci.yml` is clean. The file was never hand-edited |
| AC-4 | `dep_audit.py --check` runs after the change | It passes, and it still fails on a genuine feature-gate drift | MET. `dep_audit.py --check` exits 0. It is untouched by this change, so its drift behaviour is unchanged by construction |
| AC-5 | `feature-gates.txt` is read after the change | It contains no `integration` entry | MET. Zero matches, and `TestLintCoversIntegrationTaggedFiles` fails if one ever appears in either `feature-gates.txt` or `.golangci.yml` |
| AC-6 | The full lint runs against the tree | Zero findings remain in integration-tagged files, or the remainder is a recorded, burning-down baseline | MET, with no baseline. All 132 fixed, and the pass reports `0 issues` UNCAPPED (`--max-issues-per-linter=0 --max-same-issues=0`), so the zero is not the caps hiding a remainder. No exclusion was added to `.golangci.yml` |

**Platform coverage of the measurement.** Every number here was measured on darwin
with `GOOS=linux`, which is what selects the files; golangci-lint type-checks the
Linux build from the host, and the whole tree compiles that way (A-3). What is NOT
owed and NOT claimed is a RUN of these tests: they need a kernel, and
`make ze-qemu-integration-test` is what executes them. This spec changes no test
behaviour that a run would reveal, with one exception worth naming: the `noctx`
fixes now pass `t.Context()` to commands that previously had none, so a command
outliving its test is now killed rather than orphaned. The three cleanup helpers
that run after `t.Context()` is canceled were deliberately left context-free for
exactly that reason.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes a new QEMU integration test and runs the changed-file lint | edit → `make ze-lint-changed` → golangci-lint with the integration tag | `TestChangedLintCoversIntegrationTaggedFiles` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLintCoversIntegrationTaggedFiles` | `scripts/status/verify_run_test.go` | AC-1, AC-3, AC-5 | PASS, and proven to discriminate: deleting the pass from the `ze-lint` recipe reddens it |
| `TestChangedLintCoversIntegrationTaggedFiles` | `scripts/status/verify_run_test.go` | AC-2 | PASS, same mutation proof, plus it pins the `&&` that keeps the two passes fail-closed |
| `TestGolangciTagsStillMatchManifest` | `scripts/codegen/feature_tags_test.go` | AC-3, AC-5 | NOT WRITTEN, and not needed. The generator is untouched, so `make ze-feature-tags-check` already owns this: it re-derives all four generated lists and reports them current. `TestLintCoversIntegrationTaggedFiles` adds the half that check cannot see, which is that `integration` did NOT enter either manifest |
| `TestDepAuditAcceptsLintOnlyTag` | `scripts/dev/dep_audit_test.py` | AC-4 | NOT WRITTEN, and not needed. No lint-only tag is added to `.golangci.yml` under the chosen shape, so there is nothing for the drift check to accept. Writing it would test a code path this spec deliberately did not create |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| lint-only tags accepted by the generator | 0..N named tags | the named set | N/A | an unknown tag, which must be refused (R-4) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `scripts/dev/hook-fixture-check.py` | `scripts/dev/` | Tooling-only spec: the driving surface is the make target and the generator test, not a `.ci`. No daemon Go changes here | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | Tooling change, no wire-visible behavior and no peer involved | |

## Files to Modify
- `Makefile` - MODIFIED. `ZE_LINT_PKGS` extracted, and a second pass added to both `ze-lint` and `ze-lint-changed`
- `scripts/status/verify_run_test.go` - MODIFIED. The two guard tests plus `integrationLintPass`
- `ai/rules/points/commands/lint-gate/what-ze-lint-changed-covers-and-what-it-costs.md` - MODIFIED. The old note said 3-10 seconds and described one pass
- 28 integration-tagged test files, plus `snapshot_linux.go` and `mirror_linux.go` - MODIFIED. The 134-finding burn-down
- `scripts/codegen/feature_tags.go` - UNTOUCHED. The generator shape was rejected
- `scripts/dev/dep_audit.py` - UNTOUCHED. No lint-only tag is added, so there is no drift to teach it about
- `.golangci.yml` - UNTOUCHED. Verified by re-running the generator and diffing

## Files to Create
- None. No `mk/` fragment: the change is two recipe lines in the existing targets
- No baseline file. R-1's staged landing was not needed, because the burn-down reached zero in this phase

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling change |
| YANG validation constraints | N-A | Tooling change |
| YANG custom validators | N-A | Tooling change |
| CLI commands/flags | N-A | No `ze` command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | N-A | No new RPC |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No new counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/` and `docs/contributing/` for the new lint coverage |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `feature_tags.go` and `.golangci.yml` at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | | Check the lint documentation in `docs/contributing/` at design time |

## Implementation Steps

1. **Phase: Measure (MANDATORY FIRST)** - size the hole before choosing a shape
   - Tests: none yet. This phase produces the baseline
   - Files: a recorded finding list
   - Verify: A-1, A-2, A-3 and A-5 move off `unvalidated`. The real finding count replaces the reported eleven
2. **Phase: Choose the shape** - generator tag set, or a second lint invocation
   - Tests: none yet
   - Files: the decision lands in Key Design Decisions with the rejected alternative
   - Verify: the choice cites the measured cost from phase 1 and the timing from R-2
3. **Phase: Wiring** - land the gate against the baseline
   - Tests: `TestLintCoversIntegrationTaggedFiles`, `TestChangedLintCoversIntegrationTaggedFiles`
   - Files: the generator or the make targets, plus `dep_audit.py`
   - Verify: a deliberate violation in an integration-tagged file is reported by BOTH lint paths
4. **Phase: Burn down** - fix the findings the gate now sees
   - Tests: the existing suites stay green
   - Files: the integration-tagged files carrying findings
   - Verify: AC-6. No lint exclusion is added, and the baseline reaches zero

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Both the full lint AND the changed-file lint are covered, not just the full one |
| Correctness | `.golangci.yml` was never hand-edited. Confirm by running `make generate` and seeing no diff |
| Naming | The new target name says what it lints, not how |
| Data flow | `feature-gates.txt` still holds feature gates only |
| Rule: `ai/rules/repo-maintenance.md` | The generated file changed only as generator output |
| Rule: `ai/rules/quality.md` | No linter disabled and no exclusion added to reach green |
| Rule: `ai/rules/repo-maintenance.md` | `ai/INDEX.md` and `ai/rules/repo-maintenance.md` name the new coverage |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Integration files are linted | Add a deliberate violation, run `make ze-lint`, confirm it is reported, remove it |
| Changed-file path covered | Same violation, run `make ze-lint-changed` |
| Generator still correct | `make generate` then `git diff --exit-code .golangci.yml` |
| Drift check intact | `python3 scripts/dev/dep_audit.py --check` |
| No exclusions added | `git diff .golangci.yml` shows no new entry under an exclusion key |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The generator must refuse an unknown lint-only tag rather than writing it through (R-4, `ai/rules/evidence.md`) |
| Fail-open authorization | A drift check that silently passes on an unrecognized tag is fail-open. Confirm it still fails on genuine drift |

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
| A second golangci-lint invocation inside the EXISTING `ze-lint` and `ze-lint-changed` targets, run with `GOOS=linux --build-tags integration` | Teach `rewriteGolangci` to append lint-only tags. Rejected on evidence | The generator shape cannot express GOOS, and GOOS is what actually gates these files: 75 of the 77 are `//go:build integration && linux`. Measured on darwin, `--build-tags integration` WITHOUT `GOOS=linux` reports `0 issues` across the netlink, ike/dataplane and vrrp packages that hold 40+ findings. Shape A would therefore have landed a change, passed its own test on Linux CI, and left every dev machine exactly as blind as before |
| The second pass passes ONLY `--build-tags integration`, not a re-derived tag list | Spell `ze_core` + `$(ZE_FEATURES)` again on the command line | `--build-tags` ADDS to the config list rather than replacing it (measured against golangci-lint v2.10.1 with a two-tag probe module: config `aaa` + flag `bbb` reports BOTH files). So the generated gate list still applies and nothing is duplicated, which is what keeps `.golangci.yml` generated-and-untouched. If a future release switched to replace semantics the failure is LOUD, not silent: every `ze_`-gated package would report `build constraints exclude all Go files` |
| No new make target; extend the two that exist | A third target wired into `stagesForMode` | Fewer moving parts for the same coverage. `ze-precommit-verify` and `ze-precommit-verify-changed` already run these two, so no stage list, golden, or `ai/INDEX.md` row changes, and every existing caller (`ze-standard-test`, `ze-smoke-verify`, `all`) inherits the pass. A new target only adds a name a developer must learn to run |
| `ze-lint-changed` chains the two passes with `&&` | `;` between them | The recipe is one shell, so `;` makes the recipe's status the SECOND pass's: a real finding in pass 1 would exit 0. `TestChangedLintCoversIntegrationTaggedFiles` pins the `&&` |

### Cost (R-2)

| Run | Wall time |
|-----|-----------|
| `make ze-lint` before this change (darwin) | 17s |
| The added pass, golangci cache warm | 9s |
| The added pass, golangci cache cold for GOOS=linux | ~6.5 min, once per machine |

On Linux CI both passes analyse the same GOOS and differ only by the tag, so only the
38 integration packages are re-analysed. The cold cost above is a darwin-only, one-off.

## Known Limitations
- The finding count is unverified until phase 1 runs.
- Phase 4 may be large. R-1 states the mitigation: land the gate first, burn the baseline down after, and never add an exclusion.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
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

---

## Implementation Summary

### What Was Implemented

The gate, its two guard tests and the whole burn-down landed in `d0b49fe0d`
("fix(test): clear the lint findings the integration tag was hiding"). Closure
adds only the Makefile comment refresh below.

- `Makefile` -- `ZE_LINT_PKGS` extracted, and a second golangci-lint pass added
  to both lint recipes: `GOOS=linux $(ZE_LINT_RUN) --build-tags integration
  $(ZE_LINT_PKGS)` in `_ze-lint-impl`, and the same over `$$pkgs` in
  `_ze-lint-changed-impl`, `&&`-chained so a pass-1 finding cannot be masked.
- `scripts/status/verify_run_test.go` -- `integrationLintPass` reports whether a
  recipe body carries the pass; `expandMakeVars` resolves `$(ZE_LINT_RUN)` so a
  matcher looking for `golangci-lint` sees through the variable;
  `TestLintCoversIntegrationTaggedFiles` and
  `TestChangedLintCoversIntegrationTaggedFiles` pin one recipe each.
  `recipeBody` follows `$(MAKE)` delegation through `withDelegatedRecipes`, so
  the guards still read the real commands after both targets were routed through
  `scripts/dev/ze-run.sh`.
- 28 integration-tagged test files plus `internal/plugins/traffic/netlink/snapshot_linux.go`
  and `internal/plugins/iface/netlink/mirror_linux.go` -- the 134-finding
  burn-down.
- `ai/rules/commands.md` and its point file
  `ai/rules/points/commands/lint-gate/what-ze-lint-changed-covers-and-what-it-costs.md`
  -- the changed-file lint is two passes, not one, and the first run after a
  checkout pays a cold `GOOS=linux` analysis of minutes.

### Bugs Found/Fixed
- `internal/plugins/traffic/netlink/cs6_integration_linux_test.go`: `rawConn.Control`'s
  own error was discarded. When Control fails its callback never runs, `setErr`
  stays nil, and the test went on to assert a CS6 packet count for packets whose
  TOS was never set. Now fatal. Covered by `TestCS6ClassifyNetns` itself, which
  can no longer pass on unset TOS.
- `nftCounterByLabel` and `uniqueName`: two helpers with a declaration and no
  caller anywhere in the tree, invisible until the integration-tagged build was
  analysed. Removed, each with a `test/weakened.md` row stating why no coverage
  goes with it.

### Documentation Updates
- `ai/rules/commands.md` and the lint-gate point file, in `d0b49fe0d`.
- `Makefile`, at closure: the two stale counts refreshed (77 tracked files to 82,
  measured 2026-08-23; "75 of the 77" to "80 of the 82"), and the citation of
  this spec repointed at `plan/journal/gate-excludes-part-of-its-population.md`,
  which is the durable record and survives commit B.
- No `docs/` page states the lint population. `ai/rules/commands.md` is where it
  is stated, and `make ze-rules-render-check` reports all 29 rules fresh.

### Deviations from Plan
- The generator shape (teach `rewriteGolangci` about lint-only tags) was rejected
  on evidence: it cannot express GOOS, and GOOS is what actually gates these
  files. `.golangci.yml`, `scripts/codegen/feature_tags.go` and
  `scripts/dev/dep_audit.py` are all UNTOUCHED.
- `TestGolangciTagsStillMatchManifest` and `TestDepAuditAcceptsLintOnlyTag` were
  not written. The first is owned by `make ze-feature-tags-check`; the second
  would test a code path the chosen shape deliberately did not create.
- No baseline file. R-1's staged landing was not needed: the burn-down reached
  zero inside phase 4.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 took the reported count of about eleven findings as roughly right | 132, twelve times the report, and a capped run reads 94 because the configured per-linter cap hides a third | ran the pass uncapped in phase 1 | phase 1 exists to measure rather than accept; the spec now records the uncapped flags as the way to count |
| assumption | A-5 assumed `integration` was the only tag the linter is blind to | Re-measured 2026-08-23: 144 tracked own-source Go files are still unreached, from three causes. Personality tags are one of them; the negated `!ze_<feature>` compile-out stubs (47 files, 25 in `cmd/ze/hub`) were never named at all | derived the loaded set with `go list` under each pass's exact GOOS and tag set and subtracted it from `git ls-files` | homed in `plan/spec-fixit-lint-blind-to-every-other-build-tag.md`, with all three causes measured |
| approach | The obvious fix, adding `integration` to `.golangci.yml`, was wrong twice over | The file is GENERATED, and `integration` is not a feature gate | read `scripts/codegen/feature_tags.go` before editing | `TestLintCoversIntegrationTaggedFiles` now FAILS if `- integration` ever appears in either manifest, so the wrong fix cannot land quietly |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Files behind the `integration` tag are linted | Done | `Makefile`, `_ze-lint-impl` second pass | all 82 tracked tagged files verified present in the analysed build |
| The changed-file lint path covers them | Done | `Makefile`, `_ze-lint-changed-impl` second pass | `&&`-chained, pinned by `TestChangedLintCoversIntegrationTaggedFiles` |
| A new integration test cannot land unlinted | Done | `scripts/status/verify_run_test.go`, both guards | the pass keys on the TAG, so a new file is covered on arrival |
| `.golangci.yml` stays generated and untouched | Done | not in the diff | `make ze-feature-tags-check` reports the lists current |
| `feature-gates.txt` keeps holding feature gates only | Done | not in the diff | zero matches for `integration`, and a guard fails if one appears |
| No lint exclusion added | Done | not in the diff | `.golangci.yml` unchanged; the burn-down reached zero uncapped |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | live probe 2026-08-23 | a `recieve` misspelling appended to `internal/plugins/as112/integration_linux_test.go`: pass 1 reports `0 issues`, pass 2 reports it at the right file and line. File restored from a pristine copy saved first |
| AC-2 | Done | `TestChangedLintCoversIntegrationTaggedFiles` | pins the pass in `_ze-lint-changed-impl` and the `&&` that keeps the two fail-closed |
| AC-3 | Done | `make ze-feature-tags-check` | exits 0, "feature-tag lists are current" |
| AC-4 | Done | `python3 scripts/dev/dep_audit.py --check` | exits 0; the script is untouched by this change |
| AC-5 | Done | `grep integration feature-gates.txt` | zero matches, and both guards fail if one appears |
| AC-6 | Done | uncapped lint over the 38 packages holding tagged files | both passes report `0 issues`, rc=0 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLintCoversIntegrationTaggedFiles` | Done | `scripts/status/verify_run_test.go` | PASS; anti-vacuity guard on the `- ze_core` entry, and `t.Fatalf` when the target is absent |
| `TestChangedLintCoversIntegrationTaggedFiles` | Done | `scripts/status/verify_run_test.go` | PASS; also asserts the `&&` chaining |
| `TestGolangciTagsStillMatchManifest` | Changed | not written | `make ze-feature-tags-check` re-derives all four generated lists; the half it cannot see is covered by the guard above |
| `TestDepAuditAcceptsLintOnlyTag` | Changed | not written | no lint-only tag enters `.golangci.yml` under the chosen shape, so there is no path to test |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `Makefile` | Done | second pass in both recipes; comment counts refreshed at closure |
| `scripts/status/verify_run_test.go` | Done | `integrationLintPass` plus the two guards |
| `ai/rules/points/commands/lint-gate/what-ze-lint-changed-covers-and-what-it-costs.md` | Done | two passes, and the cold-cache cost |
| 28 tagged test files, `snapshot_linux.go`, `mirror_linux.go` | Done | the burn-down |
| `scripts/codegen/feature_tags.go` | Done | UNTOUCHED, as designed |
| `scripts/dev/dep_audit.py` | Done | UNTOUCHED, as designed |
| `.golangci.yml` | Done | UNTOUCHED, verified by re-running the generator |

### Audit Summary
- **Total items:** 23
- **Done:** 21
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (the two unwritten tests, each with the check that owns the behavior instead)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every `integration`-tagged file is linted | population enumeration, not a pass-exists check | 82 tracked files carry the tag in their build constraint. `go list` under `GOOS=linux` with the `.golangci.yml` tag list plus `integration`, over `ZE_LINT_PKGS`, reports all 82 in the analysed build: 0 not loaded. All 82 sit under `cmd/ze/` or `internal/`, both inside `ZE_LINT_PKGS`. Constraints are 80 `integration && linux`, 1 `ze_vpp && integration` (`ze_vpp` is in the config list), 1 bare `integration` |
| The second pass really adds to the config tag list rather than replacing it | live probe, both polarities | `make ze-lint ZE_LINT_PKGS=./cmd/ze-installer/...` exits 5 with `no go files to analyze`, so golangci-lint is LOUD about an empty package. `make ze-lint ZE_LINT_PKGS=./internal/component/telemetry/exporter/...`, whose four files all require `//go:build ze_telemetry`, reports `0 issues` under the SECOND pass and exits 0. Under replace semantics it would have reported the installer's error |
| The gate discriminates: it would fail if the behavior were broken | mutation, verified applied | a `recieve` misspelling appended to `internal/plugins/as112/integration_linux_test.go`, confirmed applied by `git diff --stat` before the run. Pass 1 reported `0 issues`; pass 2 reported the misspell at that file, rc=2. Restored by copying back a pristine copy saved first |
| Zero findings remain in integration-tagged files | uncapped lint run | both passes over all 38 packages holding tagged files, with the per-linter and per-message caps set to 0: `0 issues` each, rc=0 |
| The guards themselves are not vacuous | source read | `recipeBody` calls `t.Fatalf("target %q not found ... This test must not pass vacuously")`; `TestLintCoversIntegrationTaggedFiles` `t.Fatalf`s unless `.golangci.yml` still carries a `- ze_core` entry; `integrationLintPass` returns false, not true, when no line matches |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The spec metadata records `Deferral shard: -`, and no shard exists. `grep -rn fixit-lint-blind-to-integration-tag plan/deferrals/` returns nothing, so no foreign shard names this spec as a Destination either |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-lint-blind-to-integration-tag-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | clean (38 files, hashes match) |
| Rounds | 2. Round 1 over the implementation diff: 0 BLOCKER, 0 ISSUE, 2 NOTE. Round 2 over the closure edits: one NOTE, a claim in new Makefile prose that overreached, fixed in place |
| Reviewer lenses used | logic+wiring; guard/ratchet+security; documentation and rules drift. Three lenses because the diff adds a ratchet and the tests that pin it |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE | New Makefile prose claimed every other build tag names a mutually exclusive build. True of the personality tags, false of `debug`, `live` and `stress` | `Makefile`, the `ZE_LINT_PKGS` comment | the comment now names the personality tags explicitly |

No BLOCKER and no ISSUE in either round.

Two pre-existing reds were excluded and are named here rather than diagnosed:
`make ze-repository-check` reports 5 unwired-export ISSUEs, all in
`internal/component/command/answer_shape.go` and
`internal/component/command/pipe.go`, which are another session's in-flight work
and outside this diff.

Two further NOTEs were recorded and did not block. The one worth carrying
forward: the guards match a recipe line on `GOOS=linux`, `golangci-lint` and
`--build-tags integration`, and do not pin the PACKAGE argument, so a future edit
narrowing the second pass alone would keep them green. Not a present defect, since
both recipes hand the same package expression to both passes on adjacent lines.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `Makefile` | yes | `ZE_LINT_PKGS` is defined there, and `_ze-lint-impl` and `_ze-lint-changed-impl` each carry the `GOOS=linux ... --build-tags integration` line |
| `scripts/status/verify_run_test.go` | yes | `gopls symbols` reports `integrationLintPass`, `TestLintCoversIntegrationTaggedFiles` and `TestChangedLintCoversIntegrationTaggedFiles` |
| `ai/rules/points/commands/lint-gate/what-ze-lint-changed-covers-and-what-it-costs.md` | yes | reads "TWICE: once for the host build, then again under `GOOS=linux` with the `integration` build tag" |
| Files to Create | n/a | the spec creates none, by design: no `mk/` fragment and no baseline file |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a violation in a tagged file is reported by `make ze-lint` | live mutation probe 2026-08-23: pass 1 `0 issues`, pass 2 reports the misspell in `internal/plugins/as112/integration_linux_test.go`, rc=2 |
| AC-2 | the changed-file path covers it | `go test ./scripts/status/ -run TestChangedLintCoversIntegrationTaggedFiles`: PASS |
| AC-3 | the generated tag list is unchanged | `make ze-feature-tags-check`: rc=0, "feature-tag lists are current" |
| AC-4 | the drift check still passes | `python3 scripts/dev/dep_audit.py --check`: rc=0 |
| AC-5 | `integration` is in neither manifest | zero matches in `feature-gates.txt`; no `- integration` line in `.golangci.yml`; both guards assert it |
| AC-6 | zero findings remain, uncapped | `make ze-lint` over the 38 packages holding tagged files, with both issue caps set to 0: both passes `0 issues`, rc=0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-lint` | none: a tooling gate has no `.ci`. The driving surface is the recipe | yes -- `recipeBody` follows `$(MAKE)` into `_ze-lint-impl` via `withDelegatedRecipes`, so the guard reads the real command after the target was routed through `scripts/dev/ze-run.sh`. Both guards PASS against today's Makefile |
| `make ze-lint-changed` | none, same reason | yes -- `TestChangedLintCoversIntegrationTaggedFiles` PASSES and additionally asserts the `&&` |
| `make generate` | n/a | `make ze-feature-tags-check` rc=0; `.golangci.yml` is not in the diff |
| `ze-precommit-verify` and `-changed` | n/a | `stagesForMode` (`scripts/status/verify_run.go`) schedules `ze-lint` in full mode and `ze-lint-changed` in changed mode, so no new stage was needed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `./internal/plugins/iface/netlink/...` reports `0 issues` under the default lint and 33 under the integration pass. Exclusion is total, not partial |
| A-2 | broken | 132 findings, not eleven. Uncapped; a capped run reads 94 |
| A-3 | confirmed | `GOOS=linux go vet` with the gate tags plus `integration` is clean tree-wide. Nothing had to be repaired first |
| A-4 | confirmed, not exercised | the chosen shape adds no tag to `.golangci.yml`, so there is nothing for the drift check to accept. `dep_audit.py --check` exits 0 and the script is untouched |
| A-5 | broken | 144 tracked own-source Go files are still unreached, from three causes. Homed in `plan/spec-fixit-lint-blind-to-every-other-build-tag.md` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #10 test infrastructure changed | the lint-gate point file states two passes and the cold-cache cost, and `ai/rules/commands.md` carries the rendered text | yes -- `make ze-rules-render-check` rc=0, 29 rules fresh; `make ze-rules-condensed-check` rc=0 |
| #16 changed files referenced by doc anchors | grepping `docs/` for `feature_tags.go` and `.golangci.yml` finds no anchored claim about the lint population | yes -- `make ze-repository-check` reports no stale-anchor finding |
| #1-#9, #11-#15 | developer tooling only: no user-facing feature, config, CLI, API, plugin, wire format, RFC behavior, metric or registration changed | yes -- `d0b49fe0d` touches no plugin `register.go`, no YANG, no `cmd/ze` handler |
| #17 existing docs with examples for this area | `docs/contributing/testing.md` documents the BUILD flavors a gate covers, not the LINT population, so no example went stale | yes -- read its `ze-repository-tracked-build-check` section |
| citation survival | `git grep spec-fixit-lint-blind-to-integration-tag` found one tracked citer, a `Makefile` comment | yes -- repointed at `plan/journal/gate-excludes-part-of-its-population.md` in commit A, so `make ze-spec-citation-check` and `check_doc_links.py` check 5 stay green after commit B |

## Core Insight

**A gate's population is a claim, and it must be verified as one.** The pass
existing is not evidence that it reaches what it says it reaches. Two things had
to be measured before "every integration-tagged file is linted" could be said:
the file set, enumerated from the tree, and the build the linter actually loads,
derived by running `go list` under the pass's own GOOS and tag set. The two are
compared, and 0 files outside the intersection is the finding.

The same reasoning found the remainder. `--build-tags` ADDS to the config list,
which is what makes one extra pass work for `integration`; it is also why one
extra pass can never reach a `//go:build !ze_<feature>` stub, because no flag can
UNSET a tag the generated config sets. A mechanism that solves one axis is not a
mechanism for the others, and saying so is what
`plan/spec-fixit-lint-blind-to-every-other-build-tag.md` now owns.
