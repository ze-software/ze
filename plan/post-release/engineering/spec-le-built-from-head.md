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
  -> Note (phase 4, trim): `launcherIdle` became `treeIdle` and `launcherNamedInArgv` became `pathNamedInArgv` in `livetrim.go`; `NamedLauncherName` no longer refuses a capitalised name, so a leftover `bin/le-<uname>` platform directory is trimmed like a named build.

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
| A-1 | A HEAD export builds with the shared Go cache warm, in about a minute | `-trimpath` makes builds path-independent (running-commands.md) | Candidates take long; still detached, so callers do not wait | Time one candidate build | confirmed 2026-10-25, slower than assumed: the real candidate `bin/builds/20261025T114143Z-00b9273a2a/` built in the background from a HEAD export in about 2 minutes cold; the export path is fixed at `bin/builds/.building/src` so later builds hit the Go cache. Callers did not wait |
| A-2 | A smoke test of `le help` plus read-only commands (`le spec current`, `le session scratch ensure`, `le doc check links` scoped) catches a HEAD that compiles but cannot start | owner choice O-3 | A broken build is promoted; rollback covers it | Force a candidate that panics at init; it is not promoted | unvalidated: the smoke test is `le help`, `le spec wip`, `le spec current` (`le spec status` took over 2 minutes and was dropped, `smoke_test` in `le`); `TestLauncherRefusesAFailedCandidate/smoke-fails` proves a fixture candidate failing it is not published. The closure context ruled against staging a real forced-panic commit on main, which every session would build; the fixture evidence is the proof, and the real-checkout evidence is a good build of HEAD (`bin/builds/20261025T114143Z-00b9273a2a/`) built in the background, smoke-tested and run by `./le` and `bin/le` |
| A-3 | Moving the current `bin/le` file aside and creating the directory does not break a running process | rename keeps the inode | A session fails once mid-migration | Migration test with a running le | confirmed 2026-10-25 under O-10: the real migration renamed the dispatcher copy over `bin/le` and kept the old binary as `bin/builds/.previous-legacy/le` while sessions ran le; `TestLauncherMigrationKeepsTheLegacyBuild` pins it (AC-16) |
| A-4 | The 12 Go sites and the docs are every direct `bin/le` caller | grep of `internal`, `cmd`, `test`, docs | A missed caller breaks after the migration | grep plus the full functional suites in the closing gate | confirmed by grep 2026-10-25, risk removed by O-10: phase 1 classified every site and none reads the `bin/le` file as the binary (completion runs `bin/le completion words`, which the dispatcher answers on its fast path, AC-17); the remaining `bin/le test ...` lines in the docs exec the dispatcher, which execs the newest build. The full functional suites in the closing gate stay owed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two sessions start a candidate build at once | Two builds of one key | The lock; the loser execs the newest |
| R-2 | A HEAD that never builds | Every call reports the failure | Reported once per commit, not per call; the newest good build keeps running |
| R-3 | The script itself is edited while sessions run it | Syntax error mid-run | Whole-file replacement by rename |
| R-4 | A guest with a dangling `cache/` | Build fails in the guest | The existing tmp/go-cache fallback |
| R-5 | A failing newest build deleted while a long command still runs it | That command's binary vanishes | Move aside, never delete in the rollback path; the trim removes `.failed-*` only once no process uses it |
| R-6 | A QEMU guest runs `./le` over the shared checkout and has no `git` | The guest cannot read the input key, and a guest with no build of its own platform cannot build one | `./le test qemu run` installs `git` (`apk add --no-cache git curl musl-dev`) and marks every directory safe in `setupCommand` (`internal/le/test/qemu/run.go`); a checkout git cannot read has no key, and the dispatcher runs the newest build it can start. Documented in `docs/architecture/testing/qemu-integration.md` |
| R-7 | No launcher-reserved exit code exists, so automatic rollback triggers only on a failure to start | A build that passes its smoke test and then crashes after `exec` is not rolled back: `exec` replaces the shell, so nothing reads a post-exec crash | Every published build passed the smoke test, a hand-placed build with no `smoke` marker is smoke-tested before its first exec, pre-exec failures (not executable, wrong header) roll back automatically, and `./le rollback` covers the rest by hand |
| R-8 | The build lock is a pid that `kill -0` judges, and a QEMU guest shares `bin/builds/.building` with the host across pid namespaces | A guest reclaims a live host lock (the host pid is dead to the guest) and `rm -rf`s the export under the host's build, which then fails and is reported once as a failed commit; or a guest pid that happens to be live keeps a dead lock busy | FIXED 2026-10-25: the lock holds `host` (`uname -n` and the boot id) before `pid`; `lock_held` in `le` judges the pid only on its own host, and another host's lock is busy until thirty minutes old. `TestLauncherJudgesALockByItsHost` |
| R-9 | A QEMU guest during the migration window finds no linux build and execs `bin/builds/.previous-legacy/le`, the host's binary | `exec` fails with an exec format error instead of building | FIXED 2026-10-25: the legacy branch in `le` requires `le_runs_here "$legacy"` and otherwise falls through to the foreground build. `TestLauncherMigrationPassesOverAForeignLegacy` |

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
| AC-17 | Shell completion calls `bin/le` (the dispatcher) on every Tab (`internal/le/completion/completion.go`, `Binary: "bin/le"`) | The dispatcher recognises a completion call and execs the newest build with no input-key check and no candidate build: one directory listing per keystroke. The completion files are another session's work and are not edited |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLauncherRunsNewestBuild` | `cmd/ze/root_launcher_test.go` | AC-1, AC-2 | |
| `TestLauncherBuildsCandidateFromHEAD` | `cmd/ze/root_launcher_test.go` | AC-3, AC-4, AC-8 | |
| `TestLauncherRefusesAFailedCandidate` | `cmd/ze/root_launcher_test.go` | AC-5, A-2 | |
| `TestLauncherRollsBackAFailingBuild` | `cmd/ze/root_launcher_test.go` | AC-6, AC-7 | |
| `TestLauncherFreshCheckoutBuildsInForeground` | `cmd/ze/root_launcher_test.go` | AC-9 | |
| `TestLauncherRefusesAnUnwritableBuildsDirectory` | `cmd/ze/root_launcher_test.go` | B-4 (added 2026-10-25) | |
| `TestLauncherPicksItsPlatform` | `cmd/ze/root_launcher_test.go` | AC-10 | |
| `TestLauncherSessionName` | `cmd/ze/root_launcher_test.go` | AC-11 | |
| `TestTrimKeepsNewestTwoBuilds` | `internal/le/scratch/livetrim_test.go` | AC-12 | |
| `TestTrimRemovesIdleAgentBinFiles` | `internal/le/scratch/livetrim_test.go` | AC-13 | |
| `TestNewestBuildResolver`, `TestNewestBuildMissingDirectory` | `internal/le/go/toolchain/builds_test.go` | AC-14 | Note 2026-10-25 (B-3): both deleted with `NewestBuild`, `NewestPlatformBuild` and `newestBuild`, which had no non-test caller. AC-14 is now held by `TestParseBuildName` and `TestRunnableBuild` (same file) and by `TestLauncherAgreesWithTheResolver` (`cmd/ze/root_launcher_test.go`), which holds the script's newest-build choice against `ParseBuildName` and `RunnableBuild` over sorted names and the platform filter |

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

## Blocker

Handed back by the closure context on 2026-10-25, after its deliverables
review and its first `/ze-review` pass over the working-tree diff. The work
below is implementation, which `/ze-close` does not do. Each item names the
producer and what clears it. Everything else reviewed held: the `le` script
(selection, input key, candidate, smoke test, set-aside, rollback, lock with
host identity, legacy branch, `--name session`, `--update` refusal, completion
fast path), the trim rows in `internal/le/scratch/livetrim.go`, and the
resolver grammar in `internal/le/go/toolchain/builds.go`.

| # | Severity | Finding | Producer | What clears it |
|---|----------|---------|----------|----------------|
| B-1 | ISSUE | `rollback` cannot be found from le: the Integration Checklist says completion lists it, and `./le help` and `le completion words` know nothing of it, because the action exists only in the `le` script | `le`, the `rollback` branch; no registration under `internal/le` | Register `rollback` in le's command tree so help and completion list it; its handler, reached only when a build runs without the dispatcher, says to run `./le rollback`. `internal/le/le/root` and `internal/le/le/action` carry another session's uncommitted work: coordinate |
| B-2 | ISSUE | The planned functional test does not exist: `test/ui/le-built-from-head.ci` and `internal/test/fixture/ui_fixture_le_built_from_head.go` (TDD Plan, Files to Create, Goal Gates `./le test ui le-built-from-head`) | none written | Write them. The `cmd/ze/root_launcher_test.go` launcher tests already drive the real `le` script in fixture git checkouts; replacing the `.ci` with them is a scope change the owner decides, never this phase |
| B-3 | ISSUE | `NewestBuild` and `NewestPlatformBuild` have no non-test caller: only `TestLauncherAgreesWithTheResolver` calls them, and `docs/contributing/running-commands.md` says "Go callers ask `gotoolchain.NewestBuild` for the same answer", which none does | `internal/le/go/toolchain/builds.go`, `NewestBuild`, `NewestPlatformBuild`, `newestBuild` | Delete them and let the agreement test hold the script against `ParseBuildName` and `RunnableBuild`, or name the caller; correct the page sentence either way |
| B-4 | ISSUE | With no build and a `bin/builds` that cannot be created (a read-only `bin/`), the foreground path says "another le is building HEAD" and waits fifteen minutes: `take_lock` answers 1 both for a busy lock and for a failed `mkdir -p "$builds"` | `le`, `take_lock` and the foreground `while ! take_lock` loop | Tell the two apart, and refuse at once naming the directory and the cause (`ai/rules/user-facing-errors.md`); a launcher test pins it |
| N-1 | NOTE | After `./le rollback` of the only build, the next call builds HEAD in the foreground again, ignoring the rejection: the no-build path never calls `head_rejected` | `le`, the `if [ -z "$build_dir" ]` branch | Acceptable, because some le must run; say so on the page beside `./le rollback` |

Owner rulings on the items above, 2026-10-25:

| Item | Ruling | Thomas's words |
|------|--------|----------------|
| B-2 | The 13 `cmd/ze/root_launcher_test.go` launcher tests, which run the real `le` script in fixture git checkouts, are the end-to-end proof; `test/ui/le-built-from-head.ci` and its fixture are dropped from TDD Plan, Files to Create and Goal Gates | "Yes, launcher tests suffice" |
| weakening | Deleting `ze.forcedSampleLauncher` is approved and recorded with `./le test weakened approve` | "Approve" |
| B-1 | Coordinate with the better-cli session, which holds uncommitted work in `internal/le/le/root`, before registering `rollback` | "Yes, message better-cli" |

Gates owed beside the items above, and why they did not run here:

| Gate | State on 2026-10-25 |
|------|---------------------|
| `go test -race -run 'Launcher\|ShellScript' ./cmd/ze/` in the main tree | blocked: `cmd/ze` does not build, because another session's uncommitted `internal/le/rfc/check_core.go` passes `*Requirement` where `Requirement` is expected. The fix agent's run passed in a scratch tree carrying HEAD's `internal/le/rfc` |
| `./le --name session go lint run scope "./internal/le/go/toolchain/ ./internal/le/scratch/ ./cmd/ze/"` | refused a fourth time: "lint inputs changed during planning", other sessions editing |
| `./le verify changed` | owed over the commit that closes this spec |
| `./le commit create` of this work | refused: `cmd/ze/root_launcher_test.go weakens ze.forcedSampleLauncher, and no owner approval names it`. The helper forced le's one-call-in-sixteen staleness sample (`le_sampled`, `$$ % 16`) so the test could cover the branch that ran the binary as a child. This spec deleted that branch with the staleness walk (Required Reading, running-commands.md decision), so the helper's predicate no longer exists, and the test now asserts no launcher runs the binary as a child. Only Thomas approves a test removal: `./le test weakened approve unit ze.forcedSampleLauncher reason "<his words>"` |

Resolution of every item above, recorded by the second closure context on
2026-10-25 against the working tree it reviewed:

| Item | State | Evidence |
|------|-------|----------|
| B-1 | resolved | `internal/le/rollback/` registers `rollback` (`register.go` `init` calls `leroot.Register`; blank import in `internal/le/register.go`). `rollback.go` `Answer` refuses every line but help with exit 2 and names `./le rollback`. `TestRootHelpListsRollback` and `TestCompletionOffersRollback` drive `leroot.Dispatch` and `leroot.Complete`, both in HEAD (`internal/le/le/root/dispatch.go`, `complete.go`), so the commit does not depend on the better-cli session's uncommitted work |
| B-2 | resolved by owner ruling | the fourteen `TestLauncher*` tests in `cmd/ze/root_launcher_test.go` run the tracked `le` script in fixture git checkouts and are the end-to-end proof; `test/ui/le-built-from-head.ci` and its fixture are not written |
| B-3 | resolved | `NewestBuild`, `NewestPlatformBuild` and `newestBuild` are deleted from `internal/le/go/toolchain/builds.go`; `ParseBuildName`, `RunnableBuild` and `BuildsDir` each have a non-test caller in `internal/le/scratch/livetrim.go` (`publishedBuilds`, `livenessRows`); the page sentence names those two |
| B-4 | resolved | `le` `take_lock` answers 0, 1 or 2, and the foreground loop exits 1 with `$lock_error` on 2; `TestLauncherRefusesAnUnwritableBuildsDirectory` |
| N-1 | resolved | `docs/contributing/running-commands.md` states the only-build exception beside `./le rollback` |
| weakening | approved | removal of `ze.forcedSampleLauncher` approved by the owner; `./le commit audit` lists no finding in this spec's files |
| gates | resolved | see Pre-Commit Verification: the tests ran over an export of HEAD carrying only this commit's files, so no other session's uncommitted work decides them |

## Implementation Summary

### What Was Implemented
- `le` rewritten as a dispatcher: runs the newest build in `bin/builds/<UTC commit time>-<short sha>[-<goos>-<goarch>]/le`; input key from `git ls-tree` of le's inputs; one detached candidate build of a `git archive` export of HEAD under a host-aware lock; smoke test before publishing; set-aside to `.failed-<name>` on a start failure and on `./le rollback`; reporting a rejected HEAD once; legacy-binary migration; `--name session`; `--update` refusal; completion fast path; `bin/le` installed as a copy of the dispatcher.
- `internal/le/go/toolchain/builds.go`: `BuildsDir`, `ParseBuildName`, `RunnableBuild` and the entry-name constants the script and the trim share.
- `internal/le/scratch/livetrim.go`: `builds` and `agent-files` liveness rows (`trimBuilds`, `trimAgentFiles`), `treeIdle`, `pathNamedInArgv`.
- `internal/le/rollback/`: `rollback` in le's command tree for help and completion; the handler refuses outside the launcher.
- Rule point `le-follows-head-never-refresh-it-by-hand` replaces `refresh-a-stale-shared-binary-with-le-update`; `ai/rules/commands.md` re-rendered.

### Bugs Found/Fixed
- `TestToolchainOverridesLeaveTheToolchainToTheProcess/refusal` read a stamp literal the go1.27.3 bump restamped; now reads it with `gen.ReviewStamp` (test maintenance, not a product fix).
- R-8 (lock judged across pid namespaces): `TestLauncherJudgesALockByItsHost`. R-9 (foreign legacy binary): `TestLauncherMigrationPassesOverAForeignLegacy`. B-4: `TestLauncherRefusesAnUnwritableBuildsDirectory`.
- Closure round 1: `le` `build_name_valid` comment named the deleted `gotoolchain.buildNamePlatform`; it now names `ParseBuildName`.

### Documentation Updates
- `docs/contributing/running-commands.md` (launcher section, completion paragraph, store-trim rows; anchors on `le`, `builds.go`, `livetrim.go`, `rollback.go`), `docs/guide/developer-setup.md`, `docs/architecture/{system-architecture,core-design}.md`, `docs/architecture/plugin/feature-gates.md`, `docs/architecture/testing/{ci-format,verify-freshness-scope,qemu-integration}.md`, `docs/contributing/patching-the-standard-library.md`, `ai/INDEX.md`, `ai/doors/le-tooling.md`, two rule points and `ai/rules/commands.md`.
- `./le doc check verify`: every stage passes but rules-points, which names the old point file that commit A removes; `./le ai rules render-check` over an export of HEAD plus this commit, without that file: "34 rules are fresh".

### Deviations from Plan
- O-10 moved the builds from `bin/le/` to `bin/builds/`, with `bin/le` kept as the dispatcher's copy; the ACs keep the original wording.
- Phase 6 evidence on the real checkout: a real build of HEAD built in the background and ran (`bin/builds/20261025T114143Z-00b9273a2a/`, and every commit since); the forced-broken candidate and the rollback are proved by fixture tests (`TestLauncherRefusesAFailedCandidate`, `TestLauncherRollsBackAFailingBuild`), because a broken commit or a rollback on the shared main checkout moves every session onto an older le.
- The functional `.ci` was dropped by owner ruling (B-2).
- No launcher-reserved exit code (R-7).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The resolver `NewestBuild` was written for Go callers before any existed | No Go caller needed the newest build; the trim needs only the grammar and the runnable test | First closure review (B-3) | Deleted; the agreement test holds the script to `ParseBuildName` and `RunnableBuild` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| O-1 no session runs an le broken by another session's edit | Done | `le` `build_head`, `export_commit` | builds come from a HEAD export |
| O-2 replace only once known good, keep the previous | Done | `le` `build_staging` (smoke test), livetrim `buildsKept` | |
| O-3 HEAD, smoke test, rollback command plus automatic | Done | `le` `smoke_test`, `build_starts`, `set_aside`, rollback branch | |
| O-4 key on inputs | Done | `le` `le_input_key` | |
| O-5/O-10 dated build directories, no symlinks | Done | `le` `newest_build`, `builds=$root/bin/builds` | |
| O-6 never under cache/ nor hidden | Done | `bin/builds/` | |
| O-7 split spec written | Done | `plan/post-release/engineering/spec-le-own-binary.md` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLauncherRunsNewestBuild` | |
| AC-2 | Done | `TestLauncherRunsNewestBuild` | |
| AC-3 | Done | `TestLauncherBuildsCandidateFromHEAD` | |
| AC-4 | Done | `TestLauncherBuildsCandidateFromHEAD` | |
| AC-5 | Done | `TestLauncherRefusesAFailedCandidate` | |
| AC-6 | Done | `TestLauncherRollsBackAFailingBuild` | |
| AC-7 | Done | `TestLauncherRollsBackAFailingBuild/rollback_command` | |
| AC-8 | Done | `TestLauncherBuildsCandidateFromHEAD` | |
| AC-9 | Done | `TestLauncherFreshCheckoutBuildsInForeground` | |
| AC-10 | Done | `TestLauncherPicksItsPlatform` | |
| AC-11 | Done | `TestLauncherSessionName` | |
| AC-12 | Done | `TestTrimKeepsNewestTwoBuilds`, `TestTrimBuildLeftovers` | |
| AC-13 | Done | `TestTrimRemovesIdleAgentBinFiles` | |
| AC-14 | Done | grep below; `TestLauncherAgreesWithTheResolver` | |
| AC-15 | Done | `TestLauncherRefusesUpdate` | |
| AC-16 | Done | `TestLauncherMigrationKeepsTheLegacyBuild`, the real migration (A-3) | |
| AC-17 | Done | `TestLauncherCompletionRunsTheNewestWithoutAKeyCheck` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every `TestLauncher*` row | Done | `cmd/ze/root_launcher_test.go` | |
| `TestTrimKeepsNewestTwoBuilds`, `TestTrimRemovesIdleAgentBinFiles` | Done | `internal/le/scratch/livetrim_test.go` | |
| `TestNewestBuildResolver`, `TestNewestBuildMissingDirectory` | Changed | `internal/le/go/toolchain/builds_test.go` | replaced by `TestParseBuildName`, `TestRunnableBuild` (B-3) |
| `le-built-from-head` functional | Changed | none | dropped by owner ruling B-2 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `le` | Done | |
| `cmd/ze/le_build_name.go` | Done | comments only: the guard needed no change |
| `internal/le/go/toolchain/gotoolchain.go` | Done | resolver lives in new `builds.go` |
| `internal/le/scratch/livetrim.go` | Done | |
| rule point and docs | Done | `docs/contributing/testing.md`, `docs/functional-tests.md`, `docs/guide/quickstart.md` checked: their `bin/le` lines exec the dispatcher, no change |
| `test/ui/le-built-from-head.ci`, fixture | Changed | dropped (B-2) |

### Audit Summary
- **Total items:** 7 requirements, 17 ACs, 4 test rows, 6 file rows
- **Done:** 31
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| No session runs an le broken by another session's edit (O-1) | functional (fixture checkout, real script) | `TestLauncherBuildsCandidateFromHEAD` asserts an uncommitted broken edit never reaches a build; real checkout: `bin/builds/` holds one build per input-changing commit, each from a HEAD export |
| Starting development never leaves a broken le (O-1, O-2) | functional | `TestLauncherRefusesAFailedCandidate` (a candidate failing compile or smoke is never published, the newest keeps running), `TestLauncherRollsBackAFailingBuild` (a build that does not start is set aside and the next runs) |
| Rollback by command and automatically (O-3) | functional | `TestLauncherRollsBackAFailingBuild` subtests `not executable`, `crashes at start`, `rollback command` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Split le from ze so builds key on le's own inputs (O-7) | owner decision: a later, separate spec | `plan/post-release/engineering/spec-le-own-binary.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/le-built-from-head-62a8da2c-8767-4626-9bb9-3c23af536f4c.md` (30 files, verdict clean) |
| `./le spec review check` | clean: "OK (14 code files, clean, hashes match)" |
| Rounds | 3 (round 3: the scoped lint finding below, re-recorded after the fix) |
| Reviewer lenses used | wiring and functional coverage; logic and guard audit of `le` (lock, candidate, rollback, legacy, completion) and of the trim rows; removed-behaviour and test-rewrite audit; documentation drift; security (paths from git and directory names, removal scope); style pass over every changed Go file; scoped `golangci-lint` over this spec's packages |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | stale comment: `build_name_valid` named `gotoolchain.buildNamePlatform`, which `ParseBuildName` replaced | `le` line 185 | comment names `ParseBuildName` (round 2 clean) |
| 2 | ISSUE | `unparam`: `runLauncher`'s `launcher` parameter always received `"le"` | `cmd/ze/root_launcher_test.go` `runLauncher` | parameter dropped, the helper runs the fixture's `le`; callers updated, no assertion changed (round 3) |

NOTEs, recorded and not blocking: `RunnableBuild`'s doc comment carries one over-long line (`builds.go`); `git archive` honours `export-ignore` (two vendored files of `go-mnd/v2` today, neither compiled), so an export is HEAD minus such paths; with skewed commit clocks HEAD's own build can sort older than the newest, which rebuilds and then finds it published; the trim's liveness rows are indexed by position (`rows[3]`, `rows[4]`), as the earlier rows were; a wrap and a double blank line in `running-commands.md`, fixed in round 1.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/go/toolchain/builds.go`, `builds_test.go` | yes | `git status`: untracked, 180L and 125L |
| `internal/le/rollback/{register,rollback,rollback_test}.go` | yes | 24L, 76L, 120L |
| `ai/rules/points/commands/directives/le-follows-head-never-refresh-it-by-hand.md` | yes | 6L |
| `plan/post-release/engineering/spec-le-own-binary.md` | yes | 165L |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-11, AC-15..AC-17 | launcher behaviour | `./le job run label close4-head command sh c4-head.sh` over an export of HEAD 7dc5beeddf carrying this commit's Go files and `le`: `go test -race -count=1 -run 'Launcher\|ShellScript\|Build' ./cmd/ze/ ./internal/le/go/toolchain/` answered `ok cmd/ze 80.924s`, `ok internal/le/go/toolchain 1.685s`; rerun after the round 3 `runLauncher` fix (`close5-head2`, same export with the fixed test file): `ok cmd/ze 54.415s`, `ok internal/le/go/toolchain 1.575s` |
| AC-12, AC-13 | trim rows | same run, `go test -race -count=1 ./internal/le/scratch/`: every livetrim test passed; the package is red on `TestCleanTargetsOneRowPerCache` alone, which is red on HEAD with `livetrim.go` and `livetrim_test.go` restored to HEAD too (a /var against /private/var comparison since 3894cc7f89; journal row in `plan/journal/gate-verdict-depends-on-the-machine.md`, owned by `spec-test-sandbox`) |
| AC-14 | no Go caller reads `bin/le` as the binary | `grep -rn 'bin/le"' internal cmd --include=*.go`, non-test: only `internal/le/completion/completion.go` `Binary: "bin/le"`, which execs the dispatcher (AC-17), and `retirednames.go` rows naming `/usr/local/bin/le` |
| B-1 | rollback in help and completion | same run: `go vet ./internal/le/ ./internal/le/rollback/` clean, `ok internal/le/rollback 1.292s` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le help` with two builds | none (B-2): `TestLauncherRunsNewestBuild` runs the tracked `le` in a fixture checkout | yes, read: it execs the newest build and records the argv |
| `./le help` after an input commit | none (B-2): `TestLauncherBuildsCandidateFromHEAD` | yes, read: commits in the fixture, waits for the lock to go, asserts the new build runs |
| `./le help` with a failing newest build | none (B-2): `TestLauncherRollsBackAFailingBuild` | yes, read: asserts `.failed-<name>/le` exists and the older build ran |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | about two minutes cold from a HEAD export; `bin/builds/` gained a build within minutes of each commit on 2026-10-25 |
| A-2 | confirmed | a build that cannot start fails `le help`, the first smoke command; `TestLauncherRefusesAFailedCandidate/smoke-fails` proves such a candidate is not published. Proof by fixture, not by a broken commit on main (Deviations) |
| A-3 | confirmed | real migration; `TestLauncherMigrationKeepsTheLegacyBuild` |
| A-4 | confirmed | phase 1 classification and the AC-14 grep |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| CLI (`rollback`, `--update`, `--name session`) | `le` rollback branch, `--update` branch, `session_name`; `internal/le/rollback/rollback.go` `refusal` | yes |
| Store trim rows table | `livetrim.go` `buildEntryRemovable`, `failedSuperseded`, `leOwnedBinEntry`, `buildsKept` | yes |
| QEMU guest | `le` `le_runs_here`, `choose_build`; `internal/le/test/qemu/run.go` `setupCommand` | yes |
| Rules | `./le ai rules render-check` over the commit's tree: 34 rules fresh | yes |
