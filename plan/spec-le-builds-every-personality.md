# Spec: le-builds-every-personality

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

AC-6's parser consolidation was recorded delivered on September 13.
`skeleton` describes the remaining feature's planning readiness: personality
actions, the named second axis, build logs and the guard message still need
research and design. It does not mean that the parser change is unimplemented.

Owner observation, 2026-09-13: building a tagged binary is a workflow `le`
should own, and today the caller assembles it by hand.

To build the functional test runner a developer writes this, and gets no help
from anything:

```
./le job run label build command go build -tags "ze_test ze_bgp" -o bin/le-test ./cmd/ze
```

Four facts have to be right and nothing checks any of them: the tag set, the
output path, the personality-to-name convention, and job admission. A fifth is
invisible until it bites, which is that a HOST binary must never be
cross-compiled (`ai/INSTRUCTIONS.md`, "Binary naming convention": a target-arch
`ze-host` cannot exec on the build host).

**The duplication is real but it is not the one this spec first named.**
Research on 2026-09-13 enumerated the call sites properly and corrected the
opening measurement. The first count, 26 FILES containing a pattern, is not the
number of invocations: there are about 60 `go build` and `go test -c` lines
across 48 sites. More importantly, many already derive their tags correctly
through `featuretags.DaemonBuildTags`, which is the helper the spec had not
found: `qemu/guest_linux.go`, `tracked/tracked.go`, `interoplab/zebuild.go`,
`deployment/daemonbuild.go`, `test/cli/cmd_bgp.go` and the `trackedbuild` matrix
all go through it.

**What is actually duplicated is the PARSER, and the population is NINETEEN
readers rather than eight.** Every candidate was read on 2026-09-13 rather than
matched by pattern. FOUR are shell: the root `ze` and `le` launchers carry one
byte-identical awk walk, and `docker/Dockerfile` and `docker/Dockerfile.lab`
carry a second one. FIFTEEN are Go: `featuretags` itself, and fourteen
open-coded walks. Eleven of those derive a tag list, in
`docvalid/command_surfaces.go`, `qemu/alltests.go`, `repository/trackedbuild`,
`stressrepro/run.go`, `internal/test/runner`, `internal/test/fixture` twice
(the shared `uiLEFeatureTags` and the docvalid fixture's own),
`staticcheckfeaturematrix`, `changed/selector.go`,
`internal/appliance/cmd_build_test.go` and `cmd/ze/ze_le_personality_test.go`.
Three derive the `<tag> <pkg>` rows instead, in `plugin/imports`,
`tier/gates.go` and `internal/component/plugin/all/config_claims_test.go`.

Two members of the original eight were wrong. Three of the four
`ui_fixture_le_*.go` files WRITE a fixture manifest rather than parse one, and
`terminaldemo/actions.go` parses nothing: it takes its tags from `gotoolchain`,
which takes them from `featuretags`. Each real parser is a second reader of one
file, and that is the defect worth removing rather than the count of typed tag
strings.

**A personality name cannot express every tag set, so the action cannot be the
only route.** Three classes resist it, and A-1 and A-2 below record why.

**The pieces already exist and nobody joined them.** The `le` launcher's own
`build_le()` does this correctly in shell: it derives the tag set from
`feature-gates.txt`, resolves `GOCACHE` and proves the directory is usable,
pins `GOTOOLCHAIN` from the `toolchain` line of `go.mod` with the `go` directive
as fallback, builds to a staging path and renames so a peer executing the old
inode is not corrupted. None of that is reachable as an `le` action. Deriving
the tag set from `feature-gates.txt` also exists in Go already, in
`internal/le/repository/trackedbuild` and `internal/le/staticcheckfeaturematrix`.

**A guard stands where the command should be.** `./le ai hooks pretool-bash`
refuses `go build` without `-o bin/` and offers `go build -o bin/<name>
./cmd/<name>`, which is the wrong answer for every tagged personality. A guard
that refuses the raw form without providing the right one teaches the caller to
work around it.

`./le build-artifacts` is the right home and holds three appliance-specific
actions today: `host`, `installer-amd64`, `installer-arm64`.

## Required Reading

### Architecture Docs
- [ ] `ai/INSTRUCTIONS.md` "Programs" and "Binary naming convention" - host versus target families
  → Constraint: [fill during research]
- [ ] `ai/rules/commands.md` - job admission and scratch
  → Constraint: [fill during research]
- [ ] `docs/contributing/running-commands.md` - what a caller is told today
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- The shell `build_le()` in the root `le` launcher is the reference implementation; read it before designing.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `le` (the root launcher) - `build_le`, `update_le`, the staleness walk
- [ ] `internal/le/buildartifacts/` - the three existing actions
- [ ] `internal/le/repository/trackedbuild/` - tag derivation from `feature-gates.txt` in Go
- [ ] `internal/le/hookruntime/bash.go` - the guard that refuses a raw build

**Behavior to preserve:**
- Every existing `./le build-artifacts` action keeps its name and its output path.
- The launcher keeps building `bin/le` itself; this spec does not move the bootstrap.

**Behavior to change:**
- A personality is named, not spelled out as a tag list.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le build-artifacts <personality>`, and the hook message that points at it.

### Transformation Path
1. [fill during research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Caller ↔ toolchain | argv and environment (`GOCACHE`, `GOTOOLCHAIN`, `CGO_ENABLED`) | No |

### Integration Points
- [fill during design: whether the shell `build_le` becomes a caller of the Go action or stays the bootstrap]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every personality's tag set is derivable from `feature-gates.txt` plus the personality name | the launcher derives `ze_le` that way; `trackedbuild` derives the shipped flavors | Some personality needs a hand-written list and the duplication survives | enumerate every tag set and derive each | **PARTLY BROKEN**, 2026-09-13. Derivable: `ze_distro`, `ze_le`, `ze_test`, `ze_appliance` (base plus the 36 gates, through `featuretags.DaemonBuildTags`); `ze_setup` and `ze_core ze_setup` (base ALONE, because `daemontags.go` states the gates select daemon features and do not affect setup); `ze_installer` (a fixed literal, own main package). NOT derivable: `ze_perf ze_bgp`, `ze_chaos ze_bgp` and `ze_core ze_ssh`, each of which is a base plus ONE hand-picked feature. That is a third axis, WHICH feature, that a personality name does not carry |
| A-2 | No caller needs a tag combination the personality names cannot express | nothing measured yet | The action needs an escape hatch, which reopens the duplication | read every call site | **BROKEN**, 2026-09-13. Six sites: `perfbench/bench.go` and `perfrunner/run.go` (`ze_perf ze_bgp`); `testchaos/actions.go`, `functional/binaries.go` and `test/cli/cmd_web.go` (`ze_chaos ze_bgp`); `functional/binaries.go` again (`ze_core ze_ssh`, a deliberately minimal DUT); `deployment/vppevidence.go` (`ze_core ze_vpp integration`, a `go test -c` binary rather than a personality); `integration/gates.go` (marker tags `stress`, `fleetperf`, `integration`, `live` mixed with `ze_core`); and `docs/guide/ubuntu-build-install.md` (`ze_core ze_ssh ze_lg ze_web`, an instructional minimal build) |
| A-3 | The bootstrap can stay in shell without duplicating the new action | `bin/le` must exist before any `le` action can run | Two implementations of one derivation, which is the defect this spec removes | design decision, recorded | **CONFIRMED with a correction, 2026-09-13.** The honest end state is TWO readers, one shell and one Go, not one: `bin/le` does not exist before the launcher builds it, so the launcher cannot ask a Go helper which tags to compile it with. The four shell copies are now ONE, `feature-tags`, sourced by `ze`, `le`, `docker/Dockerfile` and `docker/Dockerfile.lab`. `TestTheShellWalkAndTheGoReaderAnswerTheSameTags` (`internal/le/featuretags/shellagreement_test.go`) holds it to the same answer as `featuretags.DaemonTags` over the real manifest and over one carrying comments, blanks, repeats and an unsorted order, and `TestOnlyOneShellFileParsesTheFeatureManifest` refuses a third shell reader |
| A-4 | The existing `build-artifacts` actions already stream their output where a caller can read it | assumed when AC-4 was written | AC-4 is new work for the three existing actions, not only for the added ones | read `gaterun.Stream` | **BROKEN**, 2026-09-13: `gaterun.Stream` writes straight to the terminal, so none of `host`, `installer-amd64` or `installer-arm64` writes a scratch log today |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A personality builds with a different tag set than before and something silently changes | a feature compiled out of a binary that had it | Reinventory the current build sites, then compare every derivable tag set before migrating its caller. The initial 26-file count was superseded by the September 13 research |
| R-2 | Cross-compiling a host binary, which the naming rule exists to prevent | `exec format error` | The action refuses `GOARCH` on a host personality rather than trusting the caller |
| R-3 | The hook keeps pointing at the wrong command | a caller told to use `-o bin/<name>` for a tagged build | The hook message names the new action; that edit lands with the action |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A developer builds a binary missing a feature, or one that cannot exec. No operator surface |
| How is it reverted? | A single commit revert; the hand-written forms stay valid until they are deleted |
| Who else touches this path? | `internal/le/qemu` builds `ze-host`; `internal/le/buildartifacts` owns the three appliance actions |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le build-artifacts <personality>` | → | the derived tag set and the named output | `TestEveryPersonalityDerivesItsTagSet` |
| a host personality given a target arch | → | the refusal | `TestAHostPersonalityRefusesACrossCompile` |
| the pre-tool bash guard | → | the message naming the action | `TestTheBuildGuardNamesTheActionThatBuilds` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le build-artifacts <personality>` for every personality in the table | Builds it with the tag set derived from `feature-gates.txt`, to the path the naming convention predicts |
| AC-2 | Each hand-written tag list whose personality IS derivable | The derived set is identical, proven before any of them is deleted |
| AC-6 | `feature-gates.txt` | **DONE, 2026-09-13.** Two readers parse it, one Go and one shell, and the shell one exists because the bootstrap has no `bin/le` yet. `internal/le/featuretags` is the Go reader: `Gates` answers the rows, `DaemonTags` and `DaemonBuildTags` derive from it, and every other Go file now calls one of the three. `feature-tags` is the shell reader, sourced by `ze`, `le` and the two Docker images. The two are held to one answer by `TestTheShellWalkAndTheGoReaderAnswerTheSameTags`, and a third shell reader is refused by `TestOnlyOneShellFileParsesTheFeatureManifest` |
| AC-7 | A tag set no personality name expresses (`ze_perf ze_bgp`, `ze_chaos ze_bgp`, `ze_core ze_ssh`, `ze_core ze_vpp integration`) | Reachable through a named second axis rather than a raw tag string, or deliberately left alone with the reason recorded. What MUST NOT happen is a general escape hatch taking an arbitrary tag string, which would reopen the duplication this spec exists to close |
| AC-3 | A host personality with `GOARCH` set to a target | Refused by name, because a cross-compiled host binary cannot exec |
| AC-4 | A build that fails | The log is under this session's scratch and the action names it, so the caller does not lose the compiler output to a pipe |
| AC-5 | `go build` typed raw without `-o bin/` | The hook names the action that builds a personality, not `go build -o bin/<name>` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryPersonalityDerivesItsTagSet` | `internal/le/buildartifacts/` | AC-1, AC-2 | |
| `TestAHostPersonalityRefusesACrossCompile` | `internal/le/buildartifacts/` | AC-3 | |
| `TestTheBuildGuardNamesTheActionThatBuilds` | `internal/le/hookcheck/` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | this spec adds no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill during design] | `test/` | a developer names a personality and gets a binary that runs | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | Scope is tooling | N-A | N-A | |

## Files to Modify
- `internal/le/buildartifacts/` - the personality actions
- `internal/le/hookruntime/bash.go` - the guard message
- current build callers identified by the resumed inventory; the September 13 parser consolidation is already recorded under AC-6
- `docs/contributing/running-commands.md`, `ai/INSTRUCTIONS.md`

## Files to Create
- [fill during design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | Yes | new actions on `build-artifacts` |
| CLI grammar (keyword before value) | Yes | `./le cli grammar` is the gate |
| Pipe completeness | Yes | the action answers structured data |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md` |
| 12 | Internal architecture changed? | Yes | `ai/INSTRUCTIONS.md` "Binary naming convention" |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-le-builds-every-personality.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- one personality builds through a registered action, and the test that proves its tag set matches the hand-written one is red first
2. **Phase: derive every personality** -- refresh the build-site inventory and prove AC-2 for every derivable caller before replacing its hand-written form; retain AC-7's named second-axis decision for the remaining tag sets
3. **Phase: the guard points at the action**
4. **Phase: delete the hand-written forms**

## Known Limitations
- [fill during design]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
