# Spec: gates-share-one-semantic-population

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | Engineering: every row is a development gate, hook or runner in `internal/le/` or `internal/test/` that judges the wrong set of files, units or build configurations. None changes the shipped binary; together they are 117 open journal rows and the main source of false reds and false greens in the gates |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Root cause.** Every gate decides privately WHAT it judges (its population),
and almost every one decides it by a proxy instead of reading the fact:

| Proxy | What it stands in for | Example producer |
|-------|-----------------------|------------------|
| regular expression over text | a Go unit, a comment, a shell write | `dropGoLineComments` cuts at `//` inside a string literal; `governedShellWrite` matches the command text; `rfcTagPattern` matches a fixture string |
| a whole file | the unit that changed | `ChangedTags` returns every tag of a test file when any byte of behaviour in the file moves, so `auditDiff` flags untouched tagged tests |
| a hand list | a registry or a directory the tree already holds | `localCommandRegistryFiles` reads `register.go` only under nine named directories; `heavyArea`, `nonGoPathRules`, `netnsSelections`, `KNOWN_ROOTS`, `CLIPaths`, `goEmitters` |
| the working tree | the commit under judgement | `Proposed` takes the on-disk file as the baseline, so a new untracked test is guarded as if committed; `checkDiscoveryIndex` reads only the working tree |
| one build configuration | every configuration that compiles the file | `observationRunner.argv` passes the feature manifest tags and never the carrier's own `//go:build` constraint; `./le job run ... go test` passes none; lint flavors drop packages an earlier flavor selected |

Each proxy is right on the day it is written and wrong on the first input its
author did not picture. The result is two-sided: a gate fires on what it does not
own (false red, the author routes around it) and misses what it does own (false
green, the defect lands).

**Why it matters.** A gate that cannot be trusted is routed around, and a route
around a gate is how the defect it guards lands. 117 open journal rows are this
one mechanism, and the same proxies recur in five producers each.

**The fix, in one sentence.** One `le` library answers the four population
questions every gate asks (which units, which packages under which tags, which
registered things, which changed things), each derived from the source of truth,
and every gate consumes it; each private proxy is deleted when its gate moves.

**Prefer deletion.** A gate whose only population is a hand list kept beside a
registry is replaced by a read of the registry, and the list is deleted. A check
that exists to keep two private populations agreeing (`TestReservedPeerNamesSyncWithRPCs`,
the carrier-vs-unit mismatch between `./le test weakened check` and `./le commit create`)
is deleted once both read the shared answer.

## Required Reading

### Architecture Docs
- [ ] `docs/features/ai-first.md`, `docs/architecture/testing/test-health.md`, `docs/contributing/testing.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/architecture/core-design.md` (the pages the modified files declare)
  → Constraint: each page that describes a gate's population is corrected in the commit that moves that gate
- [ ] `plan/release/validation/spec-a-gate-reports-the-population-it-judged.md` (design)
  → Decision: that spec makes a gate REPORT the population it judged and refuse an empty one; this spec makes the population CORRECT. It depends on nothing here and this spec reuses its report shape
- [ ] `ai/rules/principles.md`, "central enumeration"
  → Constraint: a population is derived from the registry that holds the fact
- [ ] `ai/META-DECISIONS.md`, "A hook checks what a command, path or file is, never how a message is worded"
  → Decision: hooks judge parsed argv words and resolved paths, never text

### Existing specs this one extends (no duplication)
| Spec | Relation |
|------|----------|
| `plan/release/validation/spec-a-gate-reports-the-population-it-judged.md` | reporting the judged population; this spec supplies the population |
| `plan/post-release/engineering/spec-fixit-netns-test-dut-tags.md` (ready) | the netns DUT tag set; consumes the P2 tag matrix, keeps its own two defects |
| `plan/post-release/engineering/spec-standard-families-register-outside-the-bgp-tag.md` (skeleton) | why many registries are empty without `ze_bgp`; the build-tag rows here need P2 whether or not that spec moves the families |
| `plan/post-release/engineering/spec-prose-cites-symbols-not-facts.md` (design) | consumes the P1 unit identity and body fingerprint |
| `plan/post-release/engineering/spec-shrink-only-baseline-cannot-see-a-relocation.md` (skeleton) | citation baseline keyed by path; P1 unit identity is its relocation answer |

## Current Behavior (MANDATORY)

**Source files read (2026-10-23):**
- [ ] `internal/le/rfc/tags.go` - `dropGoLineComments` removes from any `//` to end of line, string and rune literals included, and leaves `/* */` alone; `behaviorBytes` feeds it; `ChangedTags` returns ALL old tags of the file when the whole-file behaviour bytes differ
- [ ] `internal/le/test/weakened/audit.go` - `auditDiff` calls `rfc.ChangedTags` per file and reports `RFC-TAGGED test changed` for every tag, then filters by `acceptedBy` rows but reads no `RFC-approved:` trailer
- [ ] `internal/le/test/weakened/proposed.go` - `Proposed` reads the baseline through `proposedCurrent`, the working-tree file, so a file absent from HEAD but present on disk is guarded as existing
- [ ] `internal/le/doc/yangcontract/contract.go` - `localCommandRegistryFiles` globs `cmd/ze/*/register.go` and walks `internal/` keeping `register.go` only when its directory is one of `cli`, `client`, `crashes`, `debug`, `diag`, `env`, `explain`, `host`, `plugin`, `skills`, `support`
- [ ] `internal/le/hookruntime/bash.go` - `governedShellWrite` is five regular expressions over the command text plus a runtime-path-write triple; a path in a variable, after `cd`, from `$(cat list)` or inside a script file is invisible
- [ ] `internal/le/rfc/discriminate_observe.go` - `argv` calls `GoTest(gotoolchain.TestOptions{}, ...)`
- [ ] `internal/le/go/toolchain/gotoolchain.go` - `GoTest` always passes `-tags` with `TestTags()` (the feature manifest plus extra tags); a carrier built only under `integration && linux` is not compiled by the unit route
- [ ] `internal/le/commit/rfcchange.go` - `changedRFCUnits` compares tagged function bodies only, so a changed helper they call is unseen
- [ ] `internal/le/commit/prepare.go` - `checkDiscoveryIndex` reads only the working tree, while `isDiscoverySource` joins working tree and HEAD
- [ ] `internal/le/hookruntime/lifecycle.go` - `validateSpec` accepts an enumerated extension list; `hookValidateSpec` runs at write time only, so a committed spec can fail it
- [ ] `internal/le/go/lint/verifylint.go` - flavors lint only packages holding a file no earlier flavor selected

**Behavior to preserve:**
- every refusal each gate makes today on a population member it judges correctly
- the deliberate fail-closed direction: where the shared library cannot resolve a member, the gate refuses with the reason, never passes

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- every gate, hook and runner listed in "Owned journal rows": `./le commit create`, `./le test weakened check`, `./le rfc check`, `./le rfc record`, `./le doc check all`, `./le doc wiring`, `./le repo check`, `./le go lint run`, `./le job run`, `./le test qemu`, the `pretool-bash` and `pretool-writeedit` hooks

### Transformation Path
1. The gate names the population question it asks (P1 to P4 below)
2. The library answers from the source of truth, for the tree the gate judges (commit index, a revision, or the working tree only when the gate is explicitly about uncommitted work)
3. The gate judges that answer and reports it in the shape `spec-a-gate-reports-the-population-it-judged` defines

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| `le` to `go list` | package sets per tag configuration come from `go list -e` (vendored `golang.org/x/tools/go/packages` is available) |
| `le` to the `ze` binary | registry populations come from a full-feature `ze` dump (`ze help ai --json`, `ze schema methods` exist today) |

### Integration Points
- new package `internal/le/population`; consumers in `internal/le/rfc`, `internal/le/test/weakened`, `internal/le/commit`, `internal/le/doc/**`, `internal/le/hookruntime`, `internal/le/go/lint`, `internal/le/job`, `internal/le/qemu`, `internal/le/repository`

### Architectural Verification
| Check | Answer |
|-------|--------|
| Registration over hardcoding | every hand list a gate holds is replaced by a registry read |
| No layering | each private proxy is deleted in the commit that moves its gate; no fallback to the old reader |
| Declared once | one answer per population question; two gates asking the same question get the same set |

## The four population questions

| Question | Answer derived from | Replaces |
|----------|---------------------|----------|
| P1 Which Go units, and what is each one's behaviour | `go/parser` and `go/ast` over the file at the judged revision: unit identity is import path plus receiver plus name (stable across a file move); behaviour is the printed body without comments; a tag is read from comment groups only; a tagged unit's closure includes package-local helpers it calls | `dropGoLineComments`, `behaviorBytes`, whole-file `ChangedTags`, `rfcTagPattern` over raw text, path-keyed discrimination keys, `changedRFCUnits` body-only view, `importName` by last path element |
| P2 Which packages, under which build tags | every `//go:build` line in the tree plus the feature manifest yields the set of tag configurations; for a path, the configurations that compile it; `go list -e` per configuration | the tag set each runner chooses (job run, observation runner, qemu integration, deployment builds, lint flavors), untagged feature tests, the host-only `goList` |
| P3 Which registered things | the live registries dumped by a full-feature `ze` and by `le` itself (commands, local handlers, RPCs, web routes, plugins, `le` areas and actions and their heavy flag, functional suites and their tests) | `localCommandRegistryFiles`, `CLIPaths`, `goEmitters`, `heavyArea`, `netnsSelections`, `golden.RoutePatterns`, `KNOWN_ROOTS`, `CODE_PREFIX`, interop catalog keyed by scenario directory |
| P4 Which things changed | the commit under construction (index tree) or a named revision pair; untracked files enter only when named for commit; units changed are P1 units whose behaviour differs; approvals are read from the commit's own trailers | working-tree baselines in `Proposed`, `checkDiscoveryIndex`, `latestRun`; `auditDiff` ignoring `RFC-approved:`; the only-first-table weakened-row reader |

Hooks get one more answer built on P4: **a shell command is parsed into argv
words**, a write target is resolved through `cd` and literal assignments, and a
write whose target cannot be resolved statically (variable from a command
substitution, interpreter running a script file) into a governed tree is refused
with the reason, instead of being matched or missed by a pattern.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | Validation |
|----|------------|-------|------------|
| A-1 | `go/ast` unit bodies are cheap enough for the commit path | the RFC gates already parse tagged files | measure `./le commit create` on a 50-file commit before and after |
| A-2 | the tag configuration set is small | `feature-gates.txt` plus `linux`, `integration`, `zetest`, personality tags | P2 prints the set; a set above 16 configurations is a finding to discuss before AC-5 |
| A-3 | a full-feature `ze` dump holds every registry a gate needs | `ze help ai --json` and `ze schema methods` already publish commands and RPCs | AC-6 test per consumer |

### Risks
| Risk | Mitigation |
|------|-----------|
| Correct populations light up real debt the proxies hid (`.ci` coverage over plugin commands, 85 undocumented packages) | each gate lands with its newly visible findings either repaired or recorded as journal rows in the same commit; no silent baseline |
| A hook that refuses unresolvable writes blocks legitimate scripts | the refusal names the static form the hook accepts (a literal path argument) |

## Blast Radius

`internal/le/population` (new), and every gate named in the owned rows. No shipped `ze` code changes except test files that gain the build tag their registry needs.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test weakened check` over a commit editing an untagged function in a file that holds an RFC-tagged test | → | P1 unit diff in `auditDiff` | `test/ui/le-commit-audit-unit-granular.ci` |
| `pretool-writeedit` hook on a Write of a new untracked RFC-tagged test | → | P4 baseline in `Proposed` | `test/ui/le-hook-new-tagged-test-not-guarded.ci` |
| `./le rfc record` on a carrier under `integration && linux` | → | P2 tags in `observationRunner.argv` | `test/ui/le-discriminate-carrier-tags.ci` |
| `pretool-bash` hook on `cd plan && cat > x` | → | parsed write target | `test/ui/le-hook-cd-write-refused.ci` |
| `./le doc check all` with a local handler registered outside the nine directories | → | P3 registry read in yangcontract | `TestLocalHandlersReadFromRegistry` |

## Acceptance Criteria

| AC | Given | Then |
|----|-------|------|
| AC-1 | a test file holding an RFC-tagged unit where only an untagged unit changes | `./le test weakened check` and `./le commit create` both report nothing RFC-tagged; both name the same unit identity when a tagged unit does change |
| AC-2 | a tagged unit whose body contains `//` inside a string literal, edited after it | the change is seen; `dropGoLineComments` no longer exists |
| AC-3 | a commit carrying an owner `RFC-approved:` trailer for a unit | `./le test weakened check` reports that unit as approved, not WEAKENED |
| AC-4 | a Write of a test file absent from HEAD | the RFC-tagged-test guard does not apply; an Edit of a file present at HEAD still does |
| AC-5 | any `go test` run started by `./le job run`, `./le rfc record`, `./le test qemu` or a deployment build | its `-tags` come from P2 for the target, including the carrier's own constraint; the feature-test rows owned here go green through `./le job run` with no file-level workaround |
| AC-6 | a gate whose population was a hand list | reads P3; the hand list is deleted; adding a registered item with no edit to the gate puts it in the gate's population (one test per converted gate) |
| AC-7 | a shell command writing into `plan/` or `ai/rules/` through `cd`, a variable, `$(...)`, or an interpreter running a script | the hook refuses it with the reason and the accepted static form |
| AC-8 | `git merge-base` and other verbs that share a prefix with a banned verb | the hook allows them; the check matches whole argv words |
| AC-9 | a committed spec under `plan/` | `./le commit create` runs the spec validator over it, so a spec cannot be committed in a state its own write gate refuses |
| AC-10 | every converted gate | reports its population in the `spec-a-gate-reports-the-population-it-judged` shape |
| AC-11 | every row in "Owned journal rows" | is re-judged at its current producer (rows naming retired `scripts/`, `mk/` or `.claude/hooks/*.py` producers against the Go successor), repaired through P1 to P4, and closed in its journal file in the commit that repairs it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Proves |
|------|------|--------|
| `TestUnitIdentitySurvivesFileMove` | `internal/le/population/units_test.go` | P1, AC-1 |
| `TestCommentInsideStringNotStripped` | same | AC-2 |
| `TestTaggedUnitClosureIncludesHelper` | same | P1 helper closure |
| `TestTagConfigurationsCoverEveryBuildLine` | `internal/le/population/tags_test.go` | P2, AC-5 |
| `TestCarrierConstraintJoinsTestTags` | `internal/le/rfc/discriminate_observe_test.go` | AC-5 |
| `TestLocalHandlersReadFromRegistry` | `internal/le/doc/yangcontract/contract_test.go` | AC-6 |
| `TestProposedBaselineIsHEAD` | `internal/le/test/weakened/proposed_test.go` | AC-4 |
| `TestAuditHonoursApprovalTrailer` | `internal/le/test/weakened/audit_test.go` | AC-3 |
| `TestShellWriteTargetResolved` | `internal/le/hookruntime/bash_test.go` | AC-7, AC-8 |

### Functional Tests
| Test | Location | Proves |
|------|----------|--------|
| the four Wiring Test `.ci` files | `test/ui/` | AC-1, AC-4, AC-5, AC-7 through the real entry points |

## Files to Modify
- `internal/le/rfc/tags.go`, `internal/le/rfc/discriminate_observe.go`, `internal/le/rfc/discriminate_action.go` - P1 units and P2 tags
- `internal/le/test/weakened/audit.go`, `internal/le/test/weakened/proposed.go` - P1 and P4
- `internal/le/commit/rfcchange.go`, `internal/le/commit/prepare.go` - P1 and P4, spec validation at commit
- `internal/le/doc/yangcontract/contract.go`, `internal/le/doc/wiring/sources.go`, `internal/le/repository/repository.go` - P3
- `internal/le/hookruntime/bash.go`, `internal/le/hookruntime/lifecycle.go` - parsed writes, derived extension set
- `internal/le/go/lint/verifylint.go`, `internal/le/job/job.go`, `internal/le/qemu/netns_linux.go` - P2 and P3

## Files to Create
- `internal/le/population/units.go`, `tags.go`, `registry.go`, `changed.go` - the four answers

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | tooling only |
| CLI commands/flags | No | no new verb; existing gates change population |
| Functional test | Yes | `test/ui/*.ci` named in Wiring Test |
| Env var, doctor, metrics, BGP family | N-A | tooling only |

### Documentation Update Checklist (BLOCKING)
| Page | Change |
|------|--------|
| `docs/features/ai-first.md` | hooks judge parsed commands and committed baselines |
| `docs/architecture/testing/test-health.md` | the audit is unit-granular and reads approval trailers |
| `docs/contributing/rfc-implementation-guide.md` | discrimination runs with the carrier's own tags |
| `docs/contributing/testing.md` | `./le job run` passes the tags P2 derives |

## Implementation Steps
1. P1 units library with the move-stable identity; convert `ChangedTags`, `auditDiff`, `changedRFCUnits`, discrimination keys; close the owned `auditDiff`, comment-stripping and fixture-string rows
2. P4 changed set; convert `Proposed`, `checkDiscoveryIndex`, `latestRun`, the weakened-row reader, approval trailers; close the `guard-blocks-its-own-authors-repair` rows
3. P2 tag matrix; route `./le job run`, the observation runner, qemu, deployment builds and lint flavors through it; close the `feature-test-missing-build-tag` rows
4. P3 registry reads; delete each hand list; close the hand-list rows of `gate-excludes-part-of-its-population` and `guard-enumerates-instead-of-subtracting`
5. Parsed shell writes in the hooks; close the governed-write and `hook-existing-patterns-false-positive` rows
6. Run `./le test unit` over `internal/le/population` and each converted package, and `./le test ui` for the Wiring Test files

## Key Design Decisions
| Decision | Why |
|----------|-----|
| One library, four questions | the 117 rows reduce to these four questions; a library per gate is how the proxies multiplied |
| Unit identity is import path plus receiver plus name | a file move is the case path-keyed records break on |
| Tags come from the `//go:build` lines, not a list | the build lines are the declaration; any list is a copy |
| Unresolvable governed writes are refused, not guessed | fail closed, and the owner rule bans judging wording |
| Delete sync tests between private populations | once both sides read P3 there is nothing left to sync |

## Owned journal rows

120 rows. A row belongs here when a development gate, hook or runner judged the wrong population because it chose that population by a text proxy, a hand list, the working tree, a whole file, or one build configuration.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| gate-fires-outside-its-population | 2026-10-25 | weakening detector `skipPattern` (`internal/le/test/weakened/detector.go`), a `t.Skip(` inside a fixture string |
| gate-fires-outside-its-population | 2026-10-21 | `checkPlanningInputs` (`internal/le/go/lint/verifylint.go`) over `job.InputHash` |
| gate-fires-outside-its-population | 2026-10-06 | `countFuzzTargets`, `internal/le/doc/yangcontract/scan.go` |
| gate-fires-outside-its-population | 2026-09-05 | `./le cli-grammar`, its `--flag in YANG` check |
| gate-fires-outside-its-population | 2026-09-03 | `./le doc wiring`, its symbol-to-page binding |
| gate-fires-outside-its-population | 2026-09-02 | `./le hook-check pretool-agent-skill`, `coveredSkill` (`internal/le/hookruntime/agent.go`) |
| gate-fires-outside-its-population | 2026-08-24 | commit helper, pre-commit verification gate |
| gate-fires-outside-its-population | 2026-08-26 | first-party Go producer guard (retired `scripts/checks/checks_test.go`, now `internal/le/repository/`) |
| gate-fires-outside-its-population | 2026-08-22 | test-relaxation audit trigger (retired script, now `internal/le/test/weakened/audit.go`) |
| gate-fires-outside-its-population | 2026-08-13 | commit helper, review gate |
| gate-fires-outside-its-population | 2026-08-17 | commit helper, review gate |
| gate-fires-outside-its-population | 2026-08-18 | `check_destructive_git` over quoted payloads (now `internal/le/hookruntime/bash.go`) |
| gate-fires-outside-its-population | 2026-08-24 | prose gate (retired `ste_check.py`, now `internal/le/ste/ste.go`) |
| gate-fires-outside-its-population | 2026-08-24 | scoped gates in a shared checkout |
| gate-fires-outside-its-population | 2026-08-24 | `c_throwaway_tests` (now `internal/le/hookruntime/writeedit.go`) |
| gate-fires-outside-its-population | 2026-08-25 | `c_auto_py_format` (now `internal/le/hookruntime/postwrite.go`) |
| gate-fires-outside-its-population | 2026-08-29 | `TestReloadCompleteMarkerDoesNotCollide` (`cmd/ze/hub/main_reload_marker_test.go`) |
| gate-fires-outside-its-population | 2026-09-02 | doc-drift check, `./le doc wiring` (`internal/le/doc/wiring/`) |
| gate-fires-outside-its-population | 2026-09-03 | `./le rfc check` discrimination ratchet: `discriminationOwedTags` |
| gate-fires-outside-its-population | 2026-09-05 | `./le commit audit`, RFC-tagged detection in `auditDiff` |
| gate-fires-outside-its-population | 2026-09-08 | `feeds` (`internal/le/discoveryindex/sources.go`), two residual over-fires |
| gate-fires-outside-its-population | 2026-09-08 | `auditDiff`, read by `./le commit audit` |
| gate-fires-outside-its-population | 2026-09-12 | `TestNoPythonLeRemains` (`internal/le/contract_test.go`) |
| gate-fires-outside-its-population | 2026-09-13 | `./le ste check`, `excluded` (`internal/le/ste/ste.go`) over `internal/le/site/testdata/` |
| gate-fires-outside-its-population | 2026-09-15 | `auditDiff` via `rfc.ChangedTags` |
| gate-fires-outside-its-population | 2026-09-28 | `./le doc check retired-commands` (`retiredRecords`, `internal/le/doc/check/retired.go`) |
| gate-fires-outside-its-population | 2026-10-09 | `(*Runner).goList` (`internal/le/go/lint/verifylint.go`) |
| gate-fires-outside-its-population | 2026-10-26 | `./le verify changed` scope widens to everything on a deleted package (`internal/le/repo/changed/selector.go`) |
| gate-excludes-part-of-its-population | 2026-10-07 | RSVP-TE and flow-export interop Go tests missing from the interop catalog |
| gate-excludes-part-of-its-population | 2026-09-21 | the ignored-path check in `./le commit create` (`validateAddPath`, `internal/le/commit/input.go`) |
| gate-excludes-part-of-its-population | 2026-09-20 | `importName` and `dropOrphanedImports` (`internal/le/rfc/discriminate_action.go`) |
| gate-excludes-part-of-its-population | 2026-09-14 | `(*scan).judgeable` (`internal/le/enumeration/enumeration.go`) |
| gate-excludes-part-of-its-population | 2026-09-08 | `governedShellWrite` (`internal/le/hookruntime/bash.go`) |
| gate-excludes-part-of-its-population | 2026-09-08 | `acceptedRows` (`internal/le/testweakened/audit.go`) against commits before `27a41cb32` |
| gate-excludes-part-of-its-population | 2026-09-05 | `test/ospf/ospf-nbma.ci` and `test/ospf/ospf-ptmp.ci` run in no VM phase |
| gate-excludes-part-of-its-population | 2026-09-05 | `checkCrossPackageWiring` (`internal/le/repository/wiring.go`) |
| gate-excludes-part-of-its-population | 2026-09-05 | `goEmitters` and `skipCommand` (`internal/le/cidispatch/cidispatch.go`) |
| gate-excludes-part-of-its-population | 2026-08-30 | `checkSuperseded` (`internal/le/rfc/check_core.go`) |
| gate-excludes-part-of-its-population | 2026-08-30 | `parseStatusLedger` (`internal/le/rfc/ledger.go`) |
| gate-excludes-part-of-its-population | 2026-08-30 | `bashGovernedWrite` (`internal/le/hookruntime/bash.go`), path after `cd` |
| gate-excludes-part-of-its-population | 2026-08-18 | doc drift gate (`functionalGateSuites`, now `internal/le/docvalid/drift.go`) |
| gate-excludes-part-of-its-population | 2026-08-16 | derived per-page asset sets (`attributePattern`) |
| gate-excludes-part-of-its-population | 2026-08-15 | go lint, one build per run |
| gate-excludes-part-of-its-population | 2026-08-11 | qemu integration target with no feature tags |
| gate-excludes-part-of-its-population | 2026-08-10 | doc gates (`checkDesignMD`, 85 packages with no page) |
| gate-excludes-part-of-its-population | 2026-08-10 | verify stages (`./le repository tree-check` changed-file set) |
| gate-excludes-part-of-its-population | 2026-08-10 | RFC evidence (real-daemon evidence carrier kind) |
| gate-excludes-part-of-its-population | 2026-08-14 | web handler golden capture (`golden.RoutePatterns`, one file) |
| gate-excludes-part-of-its-population | 2026-08-14 | web handler golden capture (`webLiveRoutes` in the test binary) |
| gate-excludes-part-of-its-population | 2026-08-14 | doc-links citation grammar (`KNOWN_ROOTS`) |
| gate-excludes-part-of-its-population | 2026-08-15 | ./le repository check cross-package wiring |
| gate-excludes-part-of-its-population | 2026-08-15 | site-health test census |
| gate-excludes-part-of-its-population | 2026-08-15 | repro tooling (`usage_error_signature`) |
| gate-excludes-part-of-its-population | 2026-08-18 | the spec/design write guards |
| gate-excludes-part-of-its-population | 2026-08-18 | the Go write gates |
| gate-excludes-part-of-its-population | 2026-08-18 | `./le repository check` cross-package wiring |
| gate-excludes-part-of-its-population | 2026-08-19 | `check_governed_doc_edit`, the shell-write guard over `plan/` and `ai/rules/` |
| gate-excludes-part-of-its-population | 2026-08-19 | `check_governed_doc_edit`, the shell-write guard over `plan/` and `ai/rules/` |
| gate-excludes-part-of-its-population | 2026-08-23 | `check_governed_doc_edit`, a write inside a script file |
| gate-excludes-part-of-its-population | 2026-08-24 | the QEMU kernel guard's population (now `internal/le/integration/`) |
| gate-excludes-part-of-its-population | 2026-08-24 | spec doc anchors `CODE_PREFIX` (now `internal/le/doc/wiring/sources.go`) |
| gate-excludes-part-of-its-population | 2026-08-26 | the spec acceptance-criteria check `check_spec_ac_completeness` |
| gate-excludes-part-of-its-population | 2026-08-28 | the per-flavor scope of `./le verify lint run` |
| gate-excludes-part-of-its-population | 2026-08-30 | the CLI grammar gate's flag feeder, `checkFlagRegister` (`internal/le/cligrammar/flags.go`) |
| gate-excludes-part-of-its-population | 2026-08-31 | `CheckPipeFlags` (`internal/component/command/grammar/flags.go`) and its feeder |
| gate-excludes-part-of-its-population | 2026-08-30 | the `.ci` coverage gate, `checkCLIHandlerCoverage` (`internal/le/repository/repository.go`) |
| gate-excludes-part-of-its-population | 2026-09-01 | `./le rfc discriminate-record` unit route, `observationRunner.argv` |
| gate-excludes-part-of-its-population | 2026-09-08 | `Run.Execute` (`internal/le/qemu/run_exec.go`), opt-in runtime kernel check |
| gate-excludes-part-of-its-population | 2026-09-08 | `feeds` (`internal/le/discoveryindex/sources.go`), the membership half |
| gate-excludes-part-of-its-population | 2026-09-08 | `checkDiscoveryIndex` (`internal/le/commit/prepare.go`), which reads only the working tree |
| gate-excludes-part-of-its-population | 2026-09-09 | `netnsSelections` and `validateNetnsSelection` (`internal/le/qemu/netns_linux.go`) |
| gate-excludes-part-of-its-population | 2026-09-11 | the weakened-row reader behind `./le commit create`, only the first table |
| gate-excludes-part-of-its-population | 2026-09-25 | the pretool-bash guard on shell writes to governed trees, `$(cat list.txt)` |
| gate-excludes-part-of-its-population | 2026-10-08 | `localCommandRegistryFiles` (`internal/le/doc/yangcontract/contract.go`) |
| gate-excludes-part-of-its-population | 2026-10-13 | `governedWrite` (`internal/le/hookruntime/bash.go`), interpreter running a script |
| check-cannot-see-the-change-it-looks-for | 2026-10-26 | discrimination record `unit-sha`, `behaviorSHA` (`internal/le/rfc/discriminate.go`) |
| check-cannot-see-the-change-it-looks-for | 2026-10-17 | RFC gate `behaviorBytes` comment stripping (`internal/le/rfc/tags.go`) |
| check-cannot-see-the-change-it-looks-for | 2026-10-04 | `changedRFCUnits` (`internal/le/commit/rfcchange.go`) |
| check-cannot-see-the-change-it-looks-for | 2026-09-05 | `./le verify reds file <path>`, `latestRun` (`internal/le/verify/reds.go`) |
| check-cannot-see-the-change-it-looks-for | 2026-09-04 | `TestReservedPeerNamesSyncWithRPCs` (`internal/component/bgp/config/loader_test.go`) |
| check-cannot-see-the-change-it-looks-for | 2026-09-26 | `retiredIDs` (`internal/le/rfc/check_ratchets.go`), read by `checkIDAllocation` |
| check-cannot-see-the-change-it-looks-for | 2026-10-09 | `coverPackages` (`internal/le/rfc/discriminate_observe.go`) |
| guard-enumerates-instead-of-subtracting | 2026-10-04 | `docs/` row of `nonGoPathRules` (`internal/le/repo/changed/selector.go`) |
| guard-enumerates-instead-of-subtracting | 2026-09-02 | `heavyArea` (`internal/le/hookruntime/bash.go`) |
| guard-enumerates-instead-of-subtracting | 2026-08-28 | `validateSpec` (`internal/le/hookruntime/lifecycle.go`), the Current Behavior check |
| guard-message-teaches-the-violation | 2026-08-31 | `./le commit audit` names the carrier, the commit gate wants the unit |
| guard-blocks-its-own-authors-repair | 2026-09-08 | `rfcTagPattern` (`internal/le/commit/rfcchange.go`), read by the write hook |
| guard-blocks-its-own-authors-repair | 2026-09-08 | `Proposed` (`internal/le/testweakened/proposed.go`) |
| guard-blocks-its-own-authors-repair | 2026-09-05 | `changedRFCTests` (`internal/le/commit/rfcchange.go`), read by `./le commit audit` |
| guard-blocks-its-own-authors-repair | 2026-09-04 | `Proposed` (`internal/le/testweakened/proposed.go`) |
| guard-blocks-its-own-authors-repair | 2026-08-30 | `Proposed` (`internal/le/testweakened/proposed.go`) |
| guard-blocks-its-own-authors-repair | 2026-08-12 | hooks (new untracked tagged test) |
| guard-blocks-its-own-authors-repair | 2026-08-02 | hooks (new untracked tagged test) |
| feature-test-missing-build-tag | 2026-10-19 | `internal/component/bgp/plugins/cmd/peer` tests `TestDeclaredShapesReachTheRegistry`, ... |
| feature-test-missing-build-tag | 2026-10-18 | `internal/component/config/cli` tests `TestCmdDeactivateLeaf`, ... |
| feature-test-missing-build-tag | 2026-10-18 | `TestRecoveryQueuedDeliveryTransport` (`plugin/process/recovery_delivery_integration_test.go`) |
| feature-test-missing-build-tag | 2026-10-14 | `TestRPCRegistrationPerModule` (`internal/component/plugin/server/rpc_registration_test.go`) |
| feature-test-missing-build-tag | 2026-10-14 | `interoplab.Discover` over `checkers()` (`internal/le/interoplab/pppoe/pppoe.go`) |
| feature-test-missing-build-tag | 2026-09-13 | the sweep itself: `./le job run label le-full-sweep command go test -tags ze_le ./internal/le/...` |
| feature-test-missing-build-tag | 2026-09-05 | `codes` declared twice in package `doctor` |
| feature-test-missing-build-tag | 2026-09-02 | `./le job run label unit-pkg quiet command go test <package>`, the focused-test recipe |
| feature-test-missing-build-tag | 2026-08-29 | a guard test run without the feature tags its registry needs |
| feature-test-missing-build-tag | 2026-08-24 | `./le deployment l2tp-ppp-test` builds the daemon with a reduced tag set |
| feature-test-missing-build-tag | 2026-09-20 | `registry.Names()` read from a `go test` run with no feature tags |
| feature-test-missing-build-tag | 2026-10-08 | `TestParseEventMonitorArgsExclude`, `TestValidateEventTypeAnyNamespace` |
| feature-test-missing-build-tag | 2026-10-10 | guest `.ci` route `LE_TEST_NO_BUILD=1 ZE_BIN=<ze> le test ui`, a ze without `zetest` |
| feature-test-missing-build-tag | 2026-10-13 | `internal/le/interoplab/bgp/check_ospf_interas.go`, untagged, whose only caller is tagged |
| feature-test-missing-build-tag | 2026-10-11 | `TestFunctionalETFiles` (`internal/component/cli/testing/runner_test.go`) |
| feature-test-missing-build-tag | 2026-10-21 | `internal/exabgp/bridge/addpath_vpn_render_test.go` (d2682e5b19), untagged |
| gate-counts-one-defect-once-per-path | 2026-08-31 | `./le cli-grammar`, R9 over `announce flowspec` |
| committed-spec-fails-its-own-write-gate | 2026-09-08 | `hookValidateSpec` (`internal/le/hookruntime/lifecycle.go`) |
| committed-spec-fails-its-own-write-gate | 2026-09-09 | `hookValidateSpec`, the code-block arm |
| hook-existing-patterns-false-positive | 2026-08-12 | hooks (`cmd=` counted in the coverage counter) |
| hook-existing-patterns-false-positive | 2026-08-18 | hooks (`check_raw_test_invocation` splits on a quoted pipe) |
| hook-existing-patterns-false-positive | 2026-09-02 | `gitVerbRun` and `bashBranchMove` (`internal/le/hookruntime/bash.go`) |
| claim-outlives-the-evidence-it-cites | 2026-09-02 | `interopCarriers` in `internal/le/rfc/carriers.go` (tier derived from a workflow naming the action) |
| gate-red-where-nothing-blocks-on-it | 2026-10-09 | `internal/component/bgp/config` tests red in an untagged export |
| guard-message-teaches-the-violation | 2026-08-31 | `validateID` blocks the whole index write on one section mismatch |
| guard-message-teaches-the-violation | 2026-08-31 | `checkAnnotatedButTested` names the remedy for one of its two causes |
| guard-message-teaches-the-violation | 2026-08-31 | `rfc/extraction/README.md` over `createExtraction` overwriting a first walk |
| plugin-list-hardcoded | 2026-09-09 | `netnsSelections[netnsPPPoE]` hand list against `test/pppoe/*.ci` |
| test-against-broken-path | 2026-08-17 | `internal/component/config/infra` bare-core tests parse optional schemas |
| test-against-broken-path | 2026-09-04 | `TestFlowSpecConfigToPlugin`, `...VPN`, `TestReservedPeerNamesSyncWithRPCs` red together |
| test-against-broken-path | 2026-09-23 | `TestBootstrapDistancesMatchTheDeclaration` source scanner |
| check-cannot-see-the-change-it-looks-for | 2026-09-04 | `./le rfc discriminate-record` mutant route cost scales with the package, not the change |
| feature-test-missing-build-tag | 2026-09-17 | always-on `mtu/cmd/overhead.go` imports `ike/crypto`, kept in the bare `ze_core` binary |
| gate-excludes-part-of-its-population | 2026-08-31 | discriminate-record revert route cannot prove a producer on a background goroutine |
| gate-excludes-part-of-its-population | 2026-09-07 | `deriveTaglessConfig`: four lint flavors run no gocritic |
| unwired-feature | 2026-08-19 | `examples/plugin/go` separate module no gate compiles (`residue` in `internal/le/go/lint/verifylint.go`) |
| unwired-feature | 2026-08-19 | netns `caps=net-admin` tests chosen by the hand list `netnsSelections` (`internal/le/test/qemu/netns_linux.go`) |

## Rows found already fixed

Verified at the producer on 2026-10-23. Another agent closes these in the journal.

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| gate-excludes-part-of-its-population | 2026-10-08 | `contractSatisfied` ignored local-handler orphans | `contractSatisfied` (`internal/le/doc/yangcontract/contract.go`) returns false on `OrphanLocalHandlers` |
| gate-excludes-part-of-its-population | 2026-10-08 | `Validate` opened `-cmd` modules only, no `rpc` read | `contractSatisfied` returns false on `OrphanRPCs`; the 117 unhandled methods are owned by the spec the row names |
| gate-excludes-part-of-its-population | 2026-09-07 | tagless lint flavors ran with no gocritic | `deriveTaglessConfig` (`internal/le/go/lint/verifylint.go`) now rewrites the config-path placeholder to the config directory, so the ruleguard rules file resolves |
| gate-excludes-part-of-its-population | 2026-08-22 | spec inventory globbed `plan/spec-*.md` without recursing | `specpath.All` (`internal/le/spec/path/specpath.go`) walks every bucket and errors on an unreadable one |

## Rows read and not owned

| Routed to | Rows |
|-----------|------|
| S1 (YANG is not the single source of truth) | `gate-excludes` 2026-09-02 `validatedSections`, 2026-09-04 and 2026-10-12 `walkTree`, 2026-08-11 x3 config validate sections and validators, 2026-08-12 hub parser vs validate, 2026-08-23 listener sources |
| S3 or S4 (product identity, ownership, duplicated fact) | `gate-excludes` 2026-08-18, 2026-08-22, 2026-09-09 nft table ownership; 2026-08-14 `PeerKey`; 2026-08-12 listener routing; 2026-08-13 name uniqueness; 2026-09-02 peer-up barrier; 2026-08-25 reload participants; 2026-08-11 vxlan kinds; 2026-09-05 alias barrier; 2026-08-10 filter_community; 2026-08-18 prefix-list; 2026-08-11 forward rails; 2026-08-30 EAP types; `guard-enumerates` 2026-09-21 RSVP NULL class, 2026-10-21 labelled families; `feature-test` 2026-09-17 `mtu/cmd` importing IKE crypto into core |
| S7 (ExaBGP translation drops input) | `gate-excludes` 2026-09-19 `readExaBGPCase`, 2026-09-06 exabgp runner `option=` lines; `check-cannot-see` 2026-09-06 config migrate warnings |
| T1 or T4 (shared host, shared tree) | `gate-fires` 2026-08-24 stall watchdog, 2026-08-22 `--file` path carries foreign content; `gate-excludes` 2026-10-12 qemu static guest binaries, 2026-08-10 shard read at HEAD |
| T5 (checker or discrimination machinery) | `gate-excludes` 2026-10-19 checker population test, 2026-08-31 revert route on goroutines; `check-cannot-see` 2026-09-12 `.wb` runner, 2026-09-04 mutant cost |
| T6 or S6 (message, not population) | `guard-message-teaches-the-violation` 2026-08-31 x2 (`validateID`, `checkAnnotatedButTested`), 2026-08-31 extraction overwrite, 2026-08-19 x2, 2026-08-24 pipe refusal |
| one-off or owner decision | `gate-fires` 2026-10-10 grammar R1 roots; `gate-excludes` 2026-08-22 Files to Modify habit, 2026-08-24 fix population practice, 2026-09-27 unenrolled RFC documents |

## Corrections to the analysis

| Analysis claim | Finding at the producer |
|----------------|-------------------------|
| the observation runner calls `GoTest(TestOptions{})` with "no build tags" | false: `GoTest` always passes `-tags` with `TestTags()`, the feature manifest set. The defect is narrower: the carrier file's own constraint (`integration && linux`) is never added |
| `guard-message-teaches-the-violation` belongs to this family | only the carrier-vs-unit row does; the other six are message defects (T6/S6) |
| `gate-excludes-part-of-its-population` is the tooling part only | 31 of its open rows are routed above, most of them product populations (S1, S3, S4) |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] Targeted tests pass: `./le test unit` over `internal/le/population` and each converted package, `./le test ui` for the Wiring Test `.ci` files, then `./le verify current fast`
- [ ] Every owned row closed in its journal file by the commit that repairs it

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: N-A, tooling with no protocol peer

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal row closures
- [ ] **Commit B:** `remove` the spec only
