# Spec: le-built-from-head

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release-independent le tooling: one session's half-written edit must never break the le every other session runs |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decisions that approve this work, all 2026-10-25 unless stated:

| Id | Decision | Thomas's words |
|----|----------|----------------|
| O-1 | The goal: no session ever runs an `le` broken by another session's edit, and starting development never leaves a broken `le` | "what we want is to not have a le being edited and broken when the development starts" |
| O-2 | A new build replaces the running one only once it is known to work; the previous one is kept so a failing new one can be replaced | "it could be softlinking a known working le and only replacing the target when we know the new is good", "we should backup the previous one so if a new le fails, we can replace it with the backup" |
| O-3 | Candidates are built from HEAD (a clean export), never from the working tree; "known good" is a smoke test; rollback is a command and also automatic when the newest build fails to start | "HEAD", "Smoke test", "Command, plus automatic on start failure" to the three questions |
| O-4 | Builds are reused by their inputs, not rebuilt per call | "Key on inputs, not output" |
| O-5 | Layout: `bin/le/` is a directory of builds, one per `bin/le/<UTC commit time>-<short HEAD sha>/le`, so the newest sorts last; no symlinks: the `./le` script finds the newest; rollback is deleting a build that does not work, after which the script picks the next | "no need for bin/le-build it can be bin/le/<date>-<sha>", "the ./le script can find it and rollback is deleting a non-working binary", "bin/le/<time>-<sha>/le" |
| O-6 | Builds never live under `cache/` (a cache clean must never break le) and never in a hidden directory | "why in .store", "sounds like a bad location" |
| O-7 | Splitting le from ze is a later, separate spec (`spec-le-own-binary`, to be written) | "Spec the split, finish this first" |

This spec supersedes `spec-le-shared-build-launcher` (content-addressed named
builds with hard links): the owner replaced that design in the same
discussion. That spec's O-4 (a trim row for agent-made files in `bin/`) carries
over here as O-8.

| Id | Decision | Thomas's words |
|----|----------|----------------|
| O-8 | The store trim gains a row for agent-made entries directly in `bin/` that are not le's (`le`, `le-*`, `ze*`): removed once a day idle and named by no running process (2026-10-25: `gomu`, `gomu-rfc-99e7`, `gomu-9b5c1d68-r2/`, `rfc-vet-probe`) | "Fix it as discuss." |
| O-9 | A session that edits le itself builds from its working tree with one name per session, `./le --name session`, derived from the session id | same |
| O-10 | `bin/le` stays a FILE: an identical copy of the root `./le` dispatcher, which `./le` installs by rename whenever the root script changes. Builds live in `bin/builds/<UTC commit time>-<short sha>/le`, a name that does not start with `le`, so tab completion of `bin/le` stays unambiguous (owner: "do not call it le.d as tab completion will be unhappy"). Wherever this spec writes `bin/le/<...>` or `bin/le.d/<...>` for a build, read `bin/builds/<...>`. The 12 direct callers of `bin/le` keep working untouched, and A-3 and A-4 lose their risk | "should ./le can be moved to ./bin and do the dispatch", then "B: bin/le dispatches, builds in bin/le.d/" |

Measured 2026-10-24/25: 129 commits in 24 hours, 54 touching le's inputs
(`internal cmd pkg api vendor go.mod go.sum feature-gates.txt`), so about two
candidate builds an hour, each shared by every session. `le` is `cmd/ze`
built with `ze_le` and links 836 first-party packages.

## Required Reading

### Related specs
- [ ] `plan/post-release/engineering/spec-snapshot-builds-and-built-commits.md` - one snapshot builder from HEAD; every reused artifact keyed by its input digest
  → Decision: this spec is that spec's le slice and lands first; its HEAD export is the first form of the one snapshot builder, which that spec later generalises (`ai/rules/no-layering.md`). The export is written as a reusable step, not inline in the le script alone

### Architecture Docs
- [ ] `docs/contributing/running-commands.md` - "Launcher builds do not overwrite running binaries", the existence cache, `--update`, `--name`, the staleness warning
  → Constraint: a running le keeps its inode; every publish is a rename of a complete file or directory into place, never an in-place write
  → Decision: `--update`, the one-call-in-sixteen staleness walk and its warning go away: the newest build always follows HEAD's inputs (`ai/rules/no-layering.md`)
- [ ] `ai/rules/points/commands/directives/never-delete-bin-le-ask-for-a-named-build.md` - the source of the `bin/le` rule in `ai/rules/commands.md` (generated: edit the point, then `./le ai rules render-update`)
  → Constraint: the rule changes from "never delete bin/le" to "never delete the newest build in `bin/le/` by hand unless it does not work, which is the rollback"
- [ ] `ai/doors/le-tooling.md` - le's boundaries and the launcher
  → Constraint: the launcher is `sh` read while it runs; the script is replaced whole (new file, rename), never edited in place
- [ ] `docs/architecture/core-design.md` - le's run order and personality dispatch (`binaryName`, `LookupRoot`)
  → Constraint: the personality is chosen from the binary's file name, so every build's file is named `le` and the dispatcher execs it by that path; `sh` has no portable `exec -a`, so the name has to come from the file
- [ ] `docs/architecture/system-architecture.md` - the programs and how le is launched
  → Constraint: the page names `bin/le` as the le binary; under O-10 it is the dispatcher and the page says so
- [ ] `docs/architecture/testing/qemu-integration.md` - the guest mounts the checkout and runs `./le`
  → Constraint: the guest's platform differs; its builds carry OS and architecture after the sha, so the script picks the newest build for its own platform

**Key insights:**
- The input key is the git tree hash of le's inputs at HEAD (`git rev-parse HEAD:<path>` per input, combined), so judging "is there a build for HEAD" costs a few git calls and no file walk.
- A commit that changes no input reuses the newest build; its name keeps the commit it was built from.
- The current `bin/le` is a FILE that 12 non-test Go files and several docs name directly; becoming a directory breaks every one, so they move to the script or to a resolver first.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `le` - `build_le`, `update_le` (staging plus rename), `--update`, `--name` (`check_name`, `ZE_LE_BUILD_NAME`), the existence cache, `le_runs_here` platform fallback `bin/le-<OS>-<arch>/le`, `warn_when_stale`, `le_is_stale`, `committed_change_is_newer`, `ensure_cache_link`
- [ ] `cmd/ze/le_build_name.go` - refuses when the running binary is not the named build
- [ ] `internal/le/register.go`, `internal/le/go/toolchain/gotoolchain.go` (`NamedLauncherDir`, `NamedLauncherName`), `internal/le/scratch/livetrim.go` (`trimLaunchers`, `launcherIdle`, `launcherNamedInArgv`), `internal/le/verify/dispatch/dispatch.go`, `internal/le/completion/completion.go`, `internal/le/interoplab/kernelcheck.go`, `internal/le/doc/check/retirednames.go`, `internal/test/fixture/fixture.go`, `internal/test/fixture/misc_fixture_runner.go`, `internal/test/perfrunner/run.go`, `internal/plugins/completion/script/script.go` - each names the `bin/le` path

**Behavior to preserve:**
- A running le keeps the binary it started on
- The personality dispatch: the binary file is named `le`
- `ensure_cache_link` and the patched GOROOT activation
- A QEMU guest finds a build it can run
- `./le --name <name>` keeps working for arbitrary names

**Behavior to change:**
- `bin/le` becomes a directory of builds; the script runs the newest build for its platform
- Builds come from HEAD in the background, are smoke-tested, then renamed in
- Rollback by deletion, by command, and automatically on start failure
- `--update` and the staleness warning are removed
- `--name session`; the `bin/` agent-files trim row

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le <command>` (the script), from every session, hook and agent

### Transformation Path
1. The script lists `bin/le/` for entries of its platform, sorted; the last is the newest
2. It computes HEAD's input key and compares it with the key recorded in the newest build's directory
3. Same key, or a candidate build already running under the lock: exec the newest build
4. Different key: start a detached candidate build (lock `bin/le/.building`, held by flock or an atomic mkdir), then exec the newest build; with no build at all, the first caller builds in the foreground
5. Candidate: export HEAD to a temporary tree (`git archive` or `git worktree`), build with the patched GOROOT into a staging directory beside `bin/le/`, run the smoke test, then rename the staging directory to `bin/le/<time>-<sha>[-<os>-<arch>]/`
6. Smoke test fails: the staging directory is removed, the failure is logged with the commit, and the next caller is told once on stderr
7. Exec of the newest fails to start (exec error, or exit before dispatch with a launcher-reserved code): the script moves that build to `bin/le/.failed-<name>`, says so, and runs the next newest
8. Rollback command: `./le rollback` (or a script flag) moves the newest build aside the same way
9. The trim keeps the newest two builds per platform and removes older ones and `.failed-*` entries once no process uses them

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Script ↔ git | `git rev-parse` for the input key; `git archive` for the export | No |
| Script ↔ build | detached background process under a lock | No |
| Callers ↔ `bin/le` | every direct caller moves to the script or a resolver | No |

### Integration Points
- The `le` script (whole rewrite, replaced by rename)
- `gotoolchain` launcher helpers - one resolver answers "the newest build" for Go callers
- `livetrim.go` - build retention and the `bin/` agent-files row

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every caller goes through the script or the one resolver |
| No unintended coupling (components stay isolated) | Yes | launcher and trim only |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `build_le`, `update_le` staging, `ensure_cache_link` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling |
| Registration over hardcoding, outbound | N-A | no registry involved |
| Registration over hardcoding, inbound | Yes | one resolver; the 12 hardcoded `bin/le` sites are deleted, not wrapped |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A HEAD export builds with the shared Go cache warm, in about a minute | `-trimpath` makes builds path-independent (running-commands.md) | Candidates take long; still detached, so callers do not wait | Time one candidate build | unvalidated |
| A-2 | A smoke test of `le help` plus read-only commands (`le spec current`, `le session scratch ensure`, `le doc check links` scoped) catches a HEAD that compiles but cannot start | owner choice O-3 | A broken build is promoted; rollback covers it | Force a candidate that panics at init; it is not promoted | unvalidated |
| A-3 | Moving the current `bin/le` file aside and creating the directory does not break a running process | rename keeps the inode | A session fails once mid-migration | Migration test with a running le | unvalidated |
| A-4 | The 12 Go sites and the docs are every direct `bin/le` caller | grep of `internal`, `cmd`, `test`, docs | A missed caller breaks after the migration | grep plus the full functional suites in the closing gate | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two sessions start a candidate build at once | Two builds of one key | The lock; the loser execs the newest |
| R-2 | A HEAD that never builds | Every call reports the failure | Reported once per commit, not per call; the newest good build keeps running |
| R-3 | The script itself is edited while sessions run it | Syntax error mid-run | Whole-file replacement by rename |
| R-4 | A guest with a dangling `cache/` | Build fails in the guest | The existing tmp/go-cache fallback |
| R-5 | A failing newest build deleted while a long command still runs it | That command's binary vanishes | Move aside, never delete in the rollback path; the trim removes `.failed-*` only once no process uses it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every `./le` call in every session |
| How is it reverted? | Revert the commit; the old script finds no `bin/le` file and rebuilds one (the directory must be renamed away first, which the revert note states) |
| Who else touches this path? | Every session; `spec-le-build-cache-cap` (trim) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le help` in a fixture checkout with two builds in `bin/le/` | → | newest-build selection in the script | `TestLauncherRunsNewestBuild` (`cmd/ze/root_launcher_test.go`) |
| `./le help` after a commit that changes an input | → | detached candidate build, smoke test, rename in | `TestLauncherBuildsCandidateFromHEAD` |
| `./le help` with a newest build that fails to start | → | automatic rollback | `TestLauncherRollsBackAFailingBuild` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bin/le/` holds builds A (older) and B (newer) for this platform | `./le` execs B |
| AC-2 | HEAD's input key equals B's | No build starts; the call costs the git rev-parse calls only |
| AC-3 | A commit changes an input | The first call starts one detached candidate build from HEAD and still execs B; later calls see the lock and exec B; when the candidate passes, it appears as `bin/le/<commit time>-<sha>/le` and the next call execs it |
| AC-4 | An uncommitted edit in the working tree, even a broken one | Never reaches any build in `bin/le/` |
| AC-5 | The candidate fails to compile or fails the smoke test | Nothing is added to `bin/le/`; one stderr line per commit names the commit and the log; B keeps running |
| AC-6 | The newest build fails to start | The script moves it to `bin/le/.failed-<name>`, prints one line naming it and the next build, and runs the next |
| AC-7 | `./le rollback` | The newest build is moved aside the same way; the next call runs the previous |
| AC-8 | A commit that changes no input | No candidate build |
| AC-9 | No build exists (fresh checkout) | The first call builds in the foreground from HEAD and runs it |
| AC-10 | QEMU guest of another platform | Picks or builds a build named with its OS and architecture |
| AC-11 | `./le --name session` | Builds `bin/le-s-<first 8 of CLAUDE_CODE_SESSION_ID>/le` from the working tree; unset id: exit 2 naming the variable |
| AC-12 | The trim | Keeps the newest two builds per platform; removes older builds and `.failed-*` once no process names them |
| AC-13 | The trim, `bin/` agent files a day idle and unnamed by any process | Removed; `le`, `le-*`, `ze*` untouched |
| AC-14 | Every former direct caller of `bin/le` | Uses the script or the resolver; a grep for the old path finds none outside tests and history |
| AC-15 | `--update` | Removed; `./le --update` exits 2 saying builds now follow HEAD and how to roll back |
| AC-16 | A running le during the migration | Keeps running |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLauncherRunsNewestBuild` | `cmd/ze/root_launcher_test.go` | AC-1, AC-2 | |
| `TestLauncherBuildsCandidateFromHEAD` | `cmd/ze/root_launcher_test.go` | AC-3, AC-4, AC-8 | |
| `TestLauncherRefusesAFailedCandidate` | `cmd/ze/root_launcher_test.go` | AC-5, A-2 | |
| `TestLauncherRollsBackAFailingBuild` | `cmd/ze/root_launcher_test.go` | AC-6, AC-7 | |
| `TestLauncherFreshCheckoutBuildsInForeground` | `cmd/ze/root_launcher_test.go` | AC-9 | |
| `TestLauncherPicksItsPlatform` | `cmd/ze/root_launcher_test.go` | AC-10 | |
| `TestLauncherSessionName` | `cmd/ze/root_launcher_test.go` | AC-11 | |
| `TestTrimKeepsNewestTwoBuilds` | `internal/le/scratch/livetrim_test.go` | AC-12 | |
| `TestTrimRemovesIdleAgentBinFiles` | `internal/le/scratch/livetrim_test.go` | AC-13 | |
| `TestNewestBuildResolver` | `internal/le/go/toolchain/gotoolchain_test.go` | AC-14 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Builds kept | 2 per platform | 2 | N-A | N-A |
| Agent-file idle age | 24h fixed | 24h | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-built-from-head` | `test/ui/le-built-from-head.ci` + fixture | In a fixture git checkout: first call builds; a commit to an input yields a second build; a broken commit is never promoted; `./le rollback` returns to the previous | |

### Interop Tests (Scope: protocol)
N-A: tooling.

## Files to Modify
- `le` - rewritten whole: newest-build selection, input key, detached candidate, smoke test, rollback, `--name session`; `--update` and the staleness walk removed
- `cmd/ze/le_build_name.go` - the build-name guard for the new layout
- `internal/le/go/toolchain/gotoolchain.go` - one resolver for the newest build; the uncommitted `bin/.store` helpers from the superseded spec are deleted
- `internal/le/scratch/livetrim.go` - build retention row; the `bin/` agent-files row
- The 12 direct callers listed in Current Behavior
- `ai/rules/points/commands/directives/never-delete-bin-le-ask-for-a-named-build.md`, then `./le ai rules render-update`
- Docs: `docs/contributing/running-commands.md`, `docs/guide/developer-setup.md`, `docs/guide/quickstart.md`, `docs/functional-tests.md`, `docs/contributing/testing.md`, `docs/architecture/testing/qemu-integration.md`, `ai/doors/le-tooling.md`

## Files to Create
- `test/ui/le-built-from-head.ci`, `internal/test/fixture/ui_fixture_le_built_from_head.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le rollback`; `--update` removed; `--name session` |
| CLI grammar (keyword before value) | Yes | `rollback` is a keyword; `ai/rules/cli.md` |
| Editor autocomplete | Yes | completion lists `rollback` (`internal/le/completion`) |
| Functional test for new RPC/API | Yes | `test/ui/le-built-from-head.ci` |
| Pipe completeness | N-A | no output to pipe |
| Env var registration | No | reads `CLAUDE_CODE_SESSION_ID`, set by the harness |
| Doctor check for runtime dependencies | No | git and go already required |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | developer tooling |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md`, `docs/guide/developer-setup.md` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md`, `docs/guide/quickstart.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/contributing/testing.md`, `docs/architecture/testing/qemu-integration.md` |
| 11 | Affects daemon comparison? | No | tooling |
| 12 | Internal architecture changed? | Yes | `ai/doors/le-tooling.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the le command surface gains `rollback` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/post-release/engineering/spec-le-built-from-head.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `bin/le` and `--update` example |

## Implementation Steps

1. **Phase: Callers audit (O-10)** - check each of the 12 direct callers of `bin/le` still works when `bin/le` is the dispatcher script rather than the binary: callers that exec it are fine; callers that read the binary itself (build info, `os.Executable` comparisons in `cmd/ze/le_build_name.go`, the trim's argv match, `kernelcheck`) move to the one resolver for the newest build in `bin/builds/`
2. **Phase: Wiring** - fixture checkout helper and the failing launcher tests
3. **Phase: Launcher** - the new script, written to a sibling file and renamed into place in one step; migration of the existing `bin/le` file into `bin/le/<time>-<sha>/le`; `--update` removed; `rollback`; `--name session`
4. **Phase: Trim** - build retention and the `bin/` agent-files row
5. **Phase: Rule and docs** - the rule point, render-update, every page
6. **Phase: Evidence** - two real commits in this checkout: a candidate built in the background and run; a forced broken candidate never promoted; a rollback

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No path from the working tree into `bin/le/` |
| Correctness | No caller waits for a candidate build except a checkout with no build at all |
| Data flow | Every Go caller uses the one resolver |
| Rule: no-layering | `--update`, the staleness walk and the existence-cache file are deleted, not kept beside |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Newest-build launcher | `ls bin/le/` shows dated builds; `./le` runs the last |
| No direct callers | grep for the old `bin/le` file path |
| Rollback | `./le rollback` evidence in the spec |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path handling | Build names come from git output and are checked before use as a path |
| Removal scope | Rollback moves, never deletes; trim stays inside `bin/le/` and `bin/` |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Newest dated build found by the script | Symlinks to current and previous | Owner: no symlinks; a sorted listing is the state |
| Build from HEAD | Working tree | No peer's uncommitted edit can reach the shared le |
| Input key from git tree hashes | Output hash after building | Skips the build itself when nothing changed |
| Builds in `bin/le/` | `bin/.store`, `cache/` | Visible, and no cache clean can break le |
| Detached candidate | Foreground build | No caller waits |

## Known Limitations
- le links all of ze, so about two candidate builds an hour; `spec-le-own-binary` (O-7, to be written) narrows the inputs.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: `./le job run ... command go test -race` over `cmd/ze`, `internal/le/scratch`, `internal/le/go/toolchain`, `./le test ui le-built-from-head`, and `./le verify changed`; the full gate (`./le verify current`) runs once over the closing commits
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
