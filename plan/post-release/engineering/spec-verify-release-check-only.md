# Spec: verify-release-check-only

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Release-independent tooling: the development loop must stop running the whole-tree verification day to day, which loads the shared machine and blocks every other session. No shipped behavior changes. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner decided (`ai/META-DECISIONS.md`, "Ze is pre-release", commit `fa2e30b6d4`):

| # | Decision |
|---|----------|
| O-1 | Day-to-day work, its closure, a commit and a push owe only the tests of the components the change reaches (BGP, OSPF, IS-IS, a plugin, the tooling), found from the change itself: the packages it reaches, the feature tags those packages build under, and the functional suites that exercise them. |
| O-2 | The full verification (`./le verify current`, `./le verify worktree`) is a release check. It is never run day to day, and never by an agent on its own initiative. |
| O-3 | A full verification runs only on an idle machine: no other agent session live and no development job running. The tooling refuses to start one otherwise and names what is running. |

The rules were rewritten to match in the same commit (`ai/rules/testing.md`, `ai/rules/quality.md` and two rule points). The tooling was not, and it contradicts all three decisions:

| Defect | Where | Effect |
|--------|-------|--------|
| D-1 | `greenBaseline` (`internal/le/repo/changed/selector.go`) anchors the change set at the commit `tmp/ze-verify.status` records, and only when that run exited 0. | With no green certificate (today: `exit=130`, `git_sha=deb6ee4359`), `./le repo changed scope`, `packages`, `groups` and the race pass all answer `./...` and every feature tag. Every "scoped" run is then a full run, and the only way out is a full verify. That is the loop the owner's decision breaks. |
| D-2 | `deriveDebt` (`internal/le/commit/owed.go`) refuses every push that no green full verify covers, and the session-start hook prints that debt in every session. | A push owes the full verify, against O-1 and O-2. |
| D-3 | `admitWorktree` (`internal/le/verify/lifecycle.go`) and `runFast` (`internal/le/verify/current.go`) admit a verify through the job registry, which only stops a second verify of the same fingerprint and takes one of `NumCPU/CoresPerJob` slots. | A full verify starts while other sessions are live and other jobs run, against O-3. This is the load the machine carries right now. |
| D-4 | A full run narrows its Staticcheck rows, race pass and functional suites from the change set since the green commit (`publishChangeScope`, `internal/le/verify/engine/scope.go`, which takes no mode). | Once the change set is anchored on git, a release check over a clean, pushed tree would select almost nothing. A release check must judge the whole tree. |

Goal: the change set is anchored on git, so scoped selection never widens merely because no full verify passed; a push owes scoped evidence over the commits it publishes, produced by one native run; the full verification judges the whole tree and refuses to start unless the machine is idle, naming what runs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the certificate, the change-set selection, the green record and the derived debt
  → Constraint: the change-set selector widens and NAMES its reason on stderr; it must never select nothing ("Producer contract"). The new anchor keeps that: a missing anchor refuses, it never answers empty.
  → Constraint: `IsProse` (declared on the `ai/`, `plan/`, `docs/` Markdown entries of `nonGoPathRules`) is the one prose predicate. The push walk keeps using it; the commit package holds no path list.
  → Decision: the push-target walk (`chooseTarget`, `walkFromGreen`, `covered`) is kept as is and fed a different record. Only the record and its anchor condition change.
  → Constraint: the full mode's stage population is named by `StagesForMode`; a mode name it does not know is STALE in `CheckCertificate`. The new scoped mode must be known to `StagesForMode` or its certificate reads as no pass.
- [ ] `docs/contributing/running-commands.md` - "The changed-set selector", "One verify at a time", "When the disk is full"
  → Constraint: a job started inside another job's slot runs straight through (`job` parent detection). The idle check runs at the verify verb's own entry, before admission, never inside a stage.
  → Constraint: `refuseFullEmpty` (`internal/le/scratch/cacheuse.go`) is the existing "refuse while something runs" precedent: an unreadable process table or registry REFUSES, and the refusal names the processes and the command that is safe instead. The idle refusal follows the same shape.
  → Decision: the page says "Suite selection is not scoped" while `verify-freshness-scope.md` says the suite map narrows functional suites. One of them is wrong; Phase 2 reads `selectSuites` and repairs the wrong page (A-5).
- [ ] `ai/rules/user-facing-errors.md` - every refusal says what is wrong, why it matters, and the command that fixes it on its own line
  → Constraint: the idle refusal names each live session and job (pid, command or label), why a full run must wait, and the command to run instead (`./le verify changed`).
- [ ] `ai/rules/commands.md`, `ai/rules/planning.md`, `plan/README.md` - command discipline, phase rules, bucket
  → Decision: bucket `post-release/engineering`, P3 under the README's priority table (tooling, no release correctness). The owner may raise it.
- [ ] `ai/rules/no-layering.md` - replacing the green-baseline anchor with the git anchor
  → Constraint: `greenBaseline` and `errNoGreenBaseline` are deleted before the anchor is written; no fallback reads the certificate.

**Key insights:**
- The selector's anchor, not the selector, causes the loop: its rules already map each path to the packages and tags it reaches.
- Every narrowing stage already reads one published scope (`ZE_VERIFY_SCOPE_PACKAGES`, `ZE_VERIFY_SCOPE_TAGS`); a full run publishing `./...` judges the whole tree with no stage edited.
- `./le go lint run` already accepts `scope packages` (`internal/le/go/lint/actions.go`), so a scoped lint needs no new linter code.
- The push walk is sound; only what it reads (green full record) is wrong for O-1.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/repo/changed/selector.go` - `resolveSelector` calls `changedPaths`, which calls `gitChangedPaths`: the three working-tree queries plus `git diff <greenBaseline> HEAD`. `greenBaseline` reads `tmp/ze-verify.status` (or `ZE_VERIFY_STATUS_FILE`), requires `exit=0` and a `git_sha` the repository holds as a commit, else returns `errNoGreenBaseline`, which `resolveSelector` turns into `failOpen(everyTag)`: `./...` and every feature.
- [ ] `internal/le/repo/changed/changed.go` - `Selector.Select` (race groups): honors a published `./...` scope, then the same `greenBaseline`; on `errNoGreenBaseline` it answers `Rest: ./...`.
- [ ] `internal/le/verify/current.go` - `runFast` admits label `verify` through `job.NewIn(root).Admit`, attaches to an equivalent run, takes a slot, runs `RunMode`.
- [ ] `internal/le/verify/lifecycle.go` - `admitWorktree` admits the worktree run the same way; `run` publishes the certificate and calls `recordGreen`.
- [ ] `internal/le/verify/engine/scope.go` - `publishChangeScope(root, logDir)` runs the selector once and publishes its answer; a widened answer is published as it stands; it takes no mode.
- [ ] `internal/le/verify/green.go`, `sweep.go` - green record `tmp/ze-verify-green.status` (commit, tree, mode full, skipped empty, cut), written only for a full, uncut-or-swept, unskipped green run; parts record for `part i of m`.
- [ ] `internal/le/verify/evidence/evidence.go` - `verify evidence release-candidate`: clones the checkout into a container and runs `./le verify current` there, skipping the firewall and web suites.
- [ ] `internal/le/job/holders.go` - `Holders` lists registry entries (pid, label) without judging liveness; an unreadable registry is an error, never an empty list.
- [ ] `internal/le/procscan/procscan.go` - `Scan` answers pid, start token, start time, argv and a `CLI` flag for a process named `claude`; it reads no parent pid, from `/proc` stat or from `ps`.
- [ ] `internal/le/session/reap.go` - `judge` treats a session as live when a pin, an argv or a transcript newer than the oldest running Claude CLI names it. It answers which session DIRECTORIES of this checkout are live, not whether the machine is idle.
- [ ] `internal/le/scratch/cacheuse.go` - `refuseFullEmpty` and `cacheUsers`: every process that holds a registry slot or runs a program that reads the Go build cache (`readsCache`), excluding this process and its parent.
- [ ] `internal/le/commit/owed.go` - `deriveDebt`: reads the green record, chooses push target P (HEAD when the green tree covers HEAD's tree, else the walk from the green commit over prose-only commits, else none), lists left-behind non-prose commits, reads person-act trailers. `pushRefusal` refuses on any refusal, an empty `Published`, or an open person act.
- [ ] `internal/le/commit/prepare.go` - `checkVerificationGates` never refuses a commit for a missing verification; it charges recorded structural reds only.
- [ ] `internal/le/hookruntime/lifecycle.go` - `reportDerivedDebt` prints `lecommit.DebtWarning` at session start.

**Behavior to preserve:**
- Every path rule in `nonGoPathRules` and the "What each changed path selects" table, except its last row; the two-level importer walk; `drop-log`; `go.mod`/`go.sum`/`vendor/` widening.
- The selector widens and names its reason on stderr; it never answers an empty set for a non-empty change.
- `CheckCertificate`, the certificate and manifest format, and the structural-red charging at commit time.
- The push walk's prose rule and the person-act trailers (`Review-owed:`, `RFC-approval-owed:`, `Owner-attested:`), unchanged.
- `part i of m` and `all of m` for the full run, and the green record as the release check's own record.
- The job registry, slots and attach semantics for every job.

**Behavior to change:**
- The change set's committed term is anchored on git, not on the certificate (D-1).
- A push owes a scoped verify record covering the commits it publishes, not a green full verify (D-2).
- Every whole-tree verify entry refuses unless the machine is idle (D-3).
- A full run judges the whole tree (D-4).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le repo changed scope|packages|groups|group-packages`: change set from git.
- `./le verify changed [commit <rev>]` (new): scoped run in a throwaway worktree.
- `./le verify current`, `./le verify current fast`, `./le verify worktree [...]`, `./le verify evidence release-candidate`: whole-tree runs, idle-gated.
- `./le commit push "<authorisation>"`, `./le commit debt-status`, session-start hook: the derived debt.

### Transformation Path
1. Anchor: `A = merge-base(HEAD, U)`, where U is `@{u}` when the branch has one, else `refs/remotes/origin/HEAD`. Neither resolves: refusal naming `git branch --set-upstream-to=origin/<branch>`.
2. Change set: working-tree queries (unchanged) plus `git diff --name-only A HEAD`; for a run pinned to a commit C, `git diff --name-only A C` and no working-tree term.
3. Classification and importer walk: unchanged (`classifyPaths`, `seedPackages`).
4. Scoped run (`changed` mode): publishes the scope, runs the scope-reading population, writes the certificate (mode `changed`) and the scoped record `tmp/ze-verify-changed.status` (commit C, tree, anchor A, upstream U's SHA at the run, timestamp).
5. Push: `deriveDebt` reads the scoped record instead of the green record. P is chosen by the existing walk from C. The record proves P only when A is an ancestor of, or equal to, the current `@{u}`, so `A..C` holds every commit `@{u}..P` publishes.
6. Full run: idle check first, then admission; publishes `./...` and every tag, so every narrowing stage judges the whole tree; writes the green record as today.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ git | `git merge-base`, `git diff --name-only -z`, `git rev-parse` through `runGit` | No |
| le ↔ host process table | `procscan.Scan` (`/proc` or one `ps` call), now with the parent pid | No |
| le ↔ job registry | `job.Holders(root)` | No |
| verify run ↔ stage children | `ZE_VERIFY_SCOPE_PACKAGES`, `ZE_VERIFY_SCOPE_TAGS` published files | No |
| verify ↔ commit | `tmp/ze-verify-changed.status` read by `deriveDebt` | No |

### Integration Points
- `resolveSelector`, `gitChangedPaths`, `Selector.Select` - read the anchor instead of `greenBaseline`.
- `publishChangeScope` - takes the mode, and publishes the whole tree for the full population.
- `StagesForMode`, `fullStages`, `fastStages` - gain a `changed` population.
- `cacheUsers`/`readsCache` - the "development job" half of the idle judgement, moved where both scratch and verify read it.
- `deriveDebt`, `readGreen`, `noTargetRefusal`, `pushRefusal`, `DebtWarning` - read and name the scoped record.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The scoped run goes through the same engine (`RunMode`), certificate and publication as the full run |
| No unintended coupling (components stay isolated) | Yes | `procscan` stays a leaf; the idle judgement lives in `internal/le/job`, which `verify` and `scratch` already import |
| No duplicated functionality (extends existing, does not recreate) | Yes | The push walk, the selector rules, `readsCache` and the worktree lifecycle are reused; nothing is copied |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling, no wire path |
| Registration over hardcoding, outbound | Yes | The `changed` population is declared with `stage(...)` like the others; the verb registers in `internal/le/verify/actions.go` |
| Registration over hardcoding, inbound | Yes | `StagesForMode` is the one mode table; the agent-process names the idle check recognizes are one declared list in `procscan` (`claudeProcess` today), searched with `grep -rn claudeProcess internal/le` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Commits already on the upstream were proven when they were pushed, so the anchor may exclude them. | O-1; the push gate this spec writes | A regression already pushed is caught only by the release check | Stated as a Known Limitation; the release check is the backstop by O-2 | unvalidated |
| A-2 | Every narrowing stage reads only the published scope, so publishing `./...` makes the full run whole-tree with no stage edit. | `verify-freshness-scope.md`; `scopeMatrix`, `Selector.Select`, `selectSuites` | A stage that narrows from its own source keeps narrowing the release check | `TestFullModePublishesWholeTree` asserts the three stages' populations; grep for every reader of `greenBaseline` and `ScopeFileKey` | unvalidated |
| A-3 | A Claude CLI process is recognized by `procscan`'s `CLI` flag on both `/proc` and `ps` paths, and the requesting session is an ancestor of the le process. | `procscan.Process.CLI`; the Bash tool runs commands as the CLI's descendants | The requester refuses itself, or a foreign session goes unseen | `TestIdleJudgementExcludesOwnAncestry` over a fixture table; one manual run from a session | unvalidated |
| A-4 | `./le go lint run scope packages` lints exactly the named packages. | `internal/le/go/lint/actions.go` keyword `scope packages`, `planScope` | The scoped run lints the whole module | Read `planScope` in Phase 4; `TestChangedPopulationReadsScope` | unvalidated |
| A-5 | `selectSuites` narrows functional suites by the suite map today. | `verify-freshness-scope.md` "What a suite REACHED" vs `running-commands.md` "Suite selection is not scoped" | The scoped run runs every functional suite, which is a near-full run | Read `selectSuites` in Phase 2; repair the wrong page | unvalidated |
| A-6 | `refs/remotes/origin/HEAD` exists in a linked worktree with no upstream. | `git clone` sets it; worktrees share refs | A worktree agent's scoped run refuses | `TestAnchorFallsBackToOriginHead`, `TestAnchorRefusesWithNoUpstream` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A long unpushed range (main is 50 ahead today) reaches `go.mod` or `vendor/` and the scoped run widens to `./...`. | The widening line on stderr names the path | Correct by the existing rule: a dependency moved, every package is reachable. The run is still admitted through slots, not idle-gated |
| R-2 | The idle check refuses forever on a host where the owner keeps an idle Claude window open. | Refusal lists a pid the owner recognizes | The refusal names the pid and command; closing it is the fix. No override keyword, so no agent can bypass O-3 (decision Q-2) |
| R-3 | A session starts after the full run was admitted. | A new pid appears during the run | Admission-time check only (decision Q-4); the green record is still a valid release record |
| R-4 | The process table or the registry cannot be read. | Error from `procscan.Scan` or `job.Holders` | REFUSE, naming the error, as `refuseFullEmpty` does; never read as idle |
| R-5 | CI (`.github/workflows/verify.yml`) and the `verify evidence` container are refused. | CI red at admission | Neither runs a Claude process or foreign job; the check passes. Inside the evidence container the check sees only the container's processes; the host-side check runs before `docker run` |
| R-6 | Removing `greenBaseline` breaks a test that drives the selector over a fixture certificate. | Red in `internal/le/repo/changed` tests | Those tests change their fixture to a git anchor; each changed test is named with why its old expectation no longer holds (`ai/rules/testing.md`) |
| R-7 | The push gate accepts a scoped record whose anchor predates commits another session pushed meanwhile. | `@{u}` moved since the run | The anchor condition (A ancestor-or-equal of current `@{u}`) plus the fast-forward requirement refuse; `TestPushRefusesWhenUpstreamMovedPastAnchor` |
| R-8 | Surfaces (skills, template, rule points) keep telling agents to run the full gate at closure. | grep finds `verify current` / `verify worktree` in a closure step | Phase 6 rewrites every surface the grep in Files to Modify names, and `./le ai rules condensed-update` regenerates CORE/TRIGGERS |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: a push could go out with too little evidence (wrong anchor), or the full verify could refuse when it should run. No shipped binary changes. |
| How is it reverted? | Single-commit revert per phase. |
| Who else touches this path? | `spec-verify-scope-5-suite-coverage-map` (ready; suite map, `selectSuites`), `spec-native-action-job-admission` (skeleton; admission rollout), `spec-parallel-agents-verify-a-tree-nobody-authored` (skeleton). None edits the anchor, the debt or an idle gate. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le repo changed scope print packages` with no certificate | → | anchor in `resolveSelector` | `TestScopeAnchoredOnUpstreamWithNoCertificate` (`internal/le/repo/changed`) |
| `./le repo changed groups` with no certificate | → | anchor in `Selector.Select` | `TestRaceGroupsAnchoredOnUpstream` (`internal/le/repo/changed`) |
| `./le verify worktree` while a foreign Claude CLI runs | → | idle check before `admitWorktree` | `TestVerifyWorktreeRefusesWhileSessionLive` (`internal/le/verify`) |
| `./le verify current` while a registry job runs | → | idle check before `runFast` admission | `TestVerifyCurrentRefusesWhileJobRuns` (`internal/le/verify`) |
| `./le verify worktree all of 6` while not idle | → | idle check before the first piece | `TestSweepRefusesWhileNotIdle` (`internal/le/verify`) |
| `./le verify evidence release-candidate` while not idle | → | host-side idle check before `docker run` | `TestEvidenceRefusesBeforeDockerWhileNotIdle` (`internal/le/verify/evidence`) |
| `./le verify changed` | → | `changed` population, scoped record write | `TestVerifyChangedWritesScopedRecord` (`internal/le/verify`) |
| `./le commit push "<auth>"` after `verify changed` | → | `deriveDebt` over the scoped record | `TestPushAdmitsScopedRecordCoveringHead` (`internal/le/commit`) |
| session-start hook | → | `DebtWarning` names `./le verify changed` | `TestSessionStartDebtLineNamesVerifyChanged` (`internal/le/hookruntime`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No `tmp/ze-verify.status`, branch with upstream, one `.go` edit under `internal/component/ssh` | `./le repo changed scope print both` answers the ssh package and its importers within two levels and `ze_ssh`, not `./...`; stderr names no "no green commit" widening. |
| AC-2 | Committed change in `internal/plugins/ntp` ahead of `@{u}`, clean tree | The scope holds that package (committed term `merge-base(HEAD,@{u})..HEAD`). |
| AC-3 | Clean tree, HEAD equal to `@{u}` | The scope is empty and the selector says the change set holds nothing; it does not widen. |
| AC-4 | Branch with no upstream, `refs/remotes/origin/HEAD` present | The anchor is `merge-base(HEAD, origin/HEAD)`. |
| AC-5 | Branch with no upstream and no `origin/HEAD` | Exit 2; the message names the condition and the line `git branch --set-upstream-to=origin/<branch>`; no package list is printed. |
| AC-6 | `greenBaseline`, `errNoGreenBaseline` | Deleted; `grep -rn greenBaseline internal/` finds nothing; the "What each changed path selects" last row is replaced by the anchor row. |
| AC-7 | `Selector.Select` with no certificate, one Go edit | The race groups hold that edit's group, not `./...`. |
| AC-8 | Any whole-tree verify entry (`verify current`, `verify current fast`, `verify worktree`, `part i of m`, `all of m`, `verify evidence release-candidate`) while a Claude CLI that is not an ancestor of this process runs | Refused with exit 2 before admission, before a worktree or container is created; the message lists each live session (pid, command), says a full verification needs an idle machine, and gives `./le verify changed` on its own line. |
| AC-9 | Same entries while a live registry holder or a Go-cache-reading process (go, compile, link, golangci-lint, staticcheck) that is not this process, an ancestor or a descendant runs | Refused the same way, each job named by pid and label or command. |
| AC-10 | Same entries when the process table or the registry cannot be read | Refused, naming the read error; never treated as idle. |
| AC-11 | Same entries on an idle machine, started from a Claude session (the requester is an ancestor) | Admitted and run. |
| AC-12 | A full run admitted on an idle machine | `scope-packages.txt` holds `./...`, `scope-tags.txt` every tag; the Staticcheck matrix judges every row, the race pass races `./...`, every functional suite runs. |
| AC-13 | `./le verify changed` (default `commit HEAD`) | Runs in a throwaway worktree at the commit; scope `A..C`; population: unit tests and the bare `ze_core` compile-out over the scope packages under the scope tags, the race groups, the Staticcheck rows for the scope tags, `go vet` and `go lint run scope packages` over the scope packages, the functional suites `selectSuites` picks. No stage judges the whole tree unless the selector widened, and then stderr names why. It is not idle-gated; it is admitted through the job slots. |
| AC-14 | `./le verify changed` green | Certificate mode `changed` published to the checkout; `tmp/ze-verify-changed.status` records commit, tree, anchor, upstream SHA, timestamp. `CheckCertificate` knows the `changed` population. A red run writes no scoped record. |
| AC-15 | Push of HEAD with a scoped record for HEAD whose anchor is an ancestor-or-equal of `@{u}` and no open person act | `./le commit push` prepares the script; no full verify is asked. |
| AC-16 | Same, with prose-only commits after the record's commit | The walk carries them, as today. |
| AC-17 | Scoped record whose anchor is not an ancestor of the current `@{u}` | Push refused; the message says the upstream moved past the run's anchor and gives `git pull --rebase --no-autostash` then `./le verify changed`. |
| AC-18 | No scoped record | Push refused; message names `./le verify changed`, never `./le verify worktree`. |
| AC-19 | Session start with unpushed non-prose commits and no covering scoped record | The debt line reads that N commits owe a scoped verify and names `./le verify changed`; it never names a full verify. |
| AC-20 | `./le commit create` | Unchanged: never refuses for a missing verification. |
| AC-21 | Every surface listed in Files to Modify (docs, rule points, skills, template) | No surface tells an agent to run `./le verify current` or `./le verify worktree` for a commit, a closure or a push; each names the scoped evidence instead and names the full run only as the idle-machine release check. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestScopeAnchoredOnUpstreamWithNoCertificate` | `internal/le/repo/changed/anchor_test.go` | AC-1 | |
| `TestScopeHoldsCommittedAheadOfUpstream` | `internal/le/repo/changed/anchor_test.go` | AC-2 | |
| `TestScopeEmptyAtUpstream` | `internal/le/repo/changed/anchor_test.go` | AC-3 | |
| `TestAnchorFallsBackToOriginHead` | `internal/le/repo/changed/anchor_test.go` | AC-4 | |
| `TestAnchorRefusesWithNoUpstream` | `internal/le/repo/changed/anchor_test.go` | AC-5 | |
| `TestRaceGroupsAnchoredOnUpstream` | `internal/le/repo/changed/changed_test.go` | AC-7 | |
| `TestProcScanReadsParentPID` | `internal/le/procscan/procscan_test.go` | parent pid from `/proc` stat and `ps -o ppid=` | |
| `TestIdleJudgementExcludesOwnAncestry` | `internal/le/job/idle_test.go` | AC-8, AC-11 over a fixture process table | |
| `TestIdleJudgementNamesJobsAndCacheReaders` | `internal/le/job/idle_test.go` | AC-9 | |
| `TestIdleJudgementRefusesUnreadableTable` | `internal/le/job/idle_test.go` | AC-10 | |
| `TestVerifyWorktreeRefusesWhileSessionLive` | `internal/le/verify/idle_test.go` | AC-8 through the verb | |
| `TestVerifyCurrentRefusesWhileJobRuns` | `internal/le/verify/idle_test.go` | AC-9 through the verb, `fast` included | |
| `TestSweepRefusesWhileNotIdle` | `internal/le/verify/sweep_test.go` | AC-8 for `all of m` and `part i of m` | |
| `TestEvidenceRefusesBeforeDockerWhileNotIdle` | `internal/le/verify/evidence/evidence_test.go` | AC-8, recorder sees no docker call | |
| `TestFullModePublishesWholeTree` | `internal/le/verify/engine/scope_test.go` | AC-12 | |
| `TestChangedPopulationReadsScope` | `internal/le/verify/engine/stages_test.go` | AC-13: every stage of the population reads the published scope | |
| `TestVerifyChangedWritesScopedRecord` | `internal/le/verify/changed_test.go` | AC-14, red writes none | |
| `TestCheckCertificateKnowsChangedMode` | `internal/le/verify/engine/status_test.go` | AC-14 | |
| `TestPushAdmitsScopedRecordCoveringHead` | `internal/le/commit/owed_test.go` | AC-15 | |
| `TestPushCarriesProseAfterScopedRecord` | `internal/le/commit/owed_test.go` | AC-16 | |
| `TestPushRefusesWhenUpstreamMovedPastAnchor` | `internal/le/commit/owed_test.go` | AC-17 | |
| `TestPushRefusalNamesVerifyChanged` | `internal/le/commit/owed_test.go` | AC-18 | |
| `TestSessionStartDebtLineNamesVerifyChanged` | `internal/le/hookruntime/lifecycle_test.go` | AC-19 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pid in the idle judgement | 1..max pid | 1 | 0 (never a live holder) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | Tooling: the entry point is the le action `Answer`, which the wiring tests above drive; there is no `.ci` surface for le verbs | |

## Files to Modify
- `internal/le/repo/changed/selector.go` - delete `greenBaseline`, `errNoGreenBaseline`; `gitChangedPaths` uses the anchor; package comment's "A fourth widens" paragraph rewritten.
- `internal/le/repo/changed/changed.go` - `Select` uses the anchor; package comment line 8.
- `internal/le/procscan/procscan.go` - `Process.PPID`, filled by `parseProcStat` and `parsePS` (`ps -eo pid=,ppid=,...`).
- `internal/le/scratch/cacheuse.go` - `readsCache` and `cacheUsers` move to the idle judgement; scratch calls it.
- `internal/le/verify/current.go`, `lifecycle.go`, `sweep.go`, `actions.go` - idle check before admission on every whole-tree entry; `changed` verb.
- `internal/le/verify/evidence/evidence.go` - host-side idle check before docker.
- `internal/le/verify/engine/scope.go` - `publishChangeScope` takes the mode: whole-tree publication for the full population, anchor scope for `changed`.
- `internal/le/verify/engine/stages.go` - `changedStages`, known to `StagesForMode`.
- `internal/le/verify/deps/verifydeps.go` - unit and vet stages over the published packages in `changed` mode.
- `internal/le/verify/green.go` - scoped record writer and reader beside the green record.
- `internal/le/commit/owed.go`, `push.go` - read the scoped record, anchor condition, refusal and debt wording.
- `internal/le/commit/verification.go` - `clearingRuns` and `unjudgedCause`: the `verify=STALE` line `./le commit create` prints today tells the agent an unjudged stage "clears when a verify run reaches it: le verify worktree, ... or le verify current fast" (observed 2026-10-24 on this spec's own commit). It names `./le verify changed` instead, and the focused single-stage command it already prints first.
- `docs/architecture/testing/verify-freshness-scope.md` - "One change-set selection", the path table's last row, "The green record and the cut" (release record), "Verification debt, derived".
- `docs/contributing/running-commands.md` - "The changed-set selector", "One verify at a time", a new "A full verification needs an idle machine".
- `docs/contributing/committing.md`, `docs/contributing/testing.md`, `docs/contributing/spec-workflow.md`, `docs/contributing/claude-code-cheatsheet.md`, `docs/functional-tests.md` - push evidence and closure.
- `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md`, `ai/rules/points/precommit-verify/directives/the-gate-runs-on-a-commit-not-the-working-tree.md`, `ai/rules/precommit-verify.md`, `ai/rules/completion.md`, `ai/rules/commands.md`, `ai/rules/points/git-safety/directives/push-only-on-the-owners-order.md`, then `./le ai rules condensed-update`.
- `plan/templates/TEMPLATE.md` - Goal Gates line naming the full gate at closure.
- `.claude/skills/ze-close/SKILL.md` (step 6f), `ze-verify`, `ze-implement`, `ze-commit-check`, `ze-check`, `ze-commit`, `ze-status`, `ze-progress`, `ze-test`, `ze-fix-alloc` - every step that runs or owes the full gate.

## Files to Create
- `internal/le/repo/changed/anchor.go`, `anchor_test.go` - the git anchor.
- `internal/le/job/idle.go`, `idle_test.go` - the idle judgement (process table + registry + Go-cache readers, own ancestry excluded) and its refusal text.
- `internal/le/verify/changed.go`, `changed_test.go`, `idle_test.go` - the scoped verb and the verb-level idle tests.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling, no daemon config |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | Yes | `./le verify changed [commit <rev>]` in `internal/le/verify/actions.go` |
| CLI grammar (keyword before value) | Yes | `commit <rev>` keyword form, `ai/rules/cli.md` |
| Editor autocomplete | N-A | le verbs, not the ze editor |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | Yes | `./le verify changed` answers through the action table's `json`/`yaml`/`table` like `verify worktree` |
| Env var registration | N-A | No new variable; `ZE_VERIFY_STATUS_FILE` stays for the certificate readers that remain |
| Doctor check for runtime dependencies | N-A | Not a daemon dependency; the host checks are refusals at le entry (`ai/rules/user-facing-errors.md`) |
| Prometheus counters/metrics | N-A | Tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | Development tooling, no `features/` entry |
| 2 | Config syntax changed? | N-A | No config |
| 3 | CLI command added/changed? | Yes | le verbs: `docs/contributing/running-commands.md` (ze `command-reference.md` does not cover le) |
| 4 | API/RPC added/changed? | N-A | None |
| 5 | Plugin added/changed? | N-A | None |
| 6 | Has a user guide page? | N-A | Contributor tooling |
| 7 | Wire format changed? | N-A | None |
| 8 | Plugin SDK/protocol changed? | N-A | None |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | None |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/testing.md`, `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | N-A | Tooling |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | No plugin inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/post-release/engineering/spec-verify-release-check-only.md` is run in Phase 1 and every declared doc named here before code; known now: `verify-freshness-scope.md`, `running-commands.md`, `docs/architecture/core-design.md` (procscan, job admission, evidence `// Design:` headers); `docs/architecture/testing/ci-format.md` (declared by `internal/le/verify/evidence/evidence.go`): its release-candidate description gains the host-side idle refusal. Advisory `<!-- source: -->` mentions, each re-read in the phase that edits its file and corrected if it describes the full gate as day-to-day: `docs/DESIGN.md`, `docs/architecture/plugin/feature-gates.md`, `docs/architecture/site-facts.md`, `docs/contributing/gh-pages.md` (all via `stages.go`), `docs/architecture/testing/interop.md` (via `procscan.go`, `evidence.go`), `docs/architecture/testing/runner-architecture.md` (via `evidence.go`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every `./le verify worktree` / `verify current` example in the files above, rewritten to the scoped or release form |

## Implementation Steps

1. **Phase: Wiring** -- register `verify changed`, the idle check call sites and the anchor function as stubs; write the wiring tests, which fail.
   - Tests: every Wiring Test row.
   - Files: `internal/le/verify/actions.go`, `anchor.go`, `idle.go` skeletons.
   - Verify: the verbs resolve; tests fail on behavior, not on missing symbols.
2. **Phase: Anchor** -- delete `greenBaseline`/`errNoGreenBaseline`, write the anchor, wire `resolveSelector`, `gitChangedPaths`, `Select`; repair the path table and the selector page sections in the same phase.
   - Tests: AC-1..AC-7 rows. Read `selectSuites` and settle A-5.
   - Verify: `./le job run label unit-pkg quiet command go test ./internal/le/repo/changed/...`.
3. **Phase: Idle judgement** -- `Process.PPID`; `internal/le/job/idle.go` with the ancestry exclusion, registry holders and Go-cache readers; scratch reuses it; call it before admission in `runFast`, `admitWorktree`'s callers, `Sweep`, `evidence`. Write the running-commands section.
   - Tests: AC-8..AC-11 rows.
   - Verify: scoped package tests for `procscan`, `job`, `scratch`, `verify`, `verify/evidence`.
4. **Phase: Whole-tree full run and the `changed` population** -- `publishChangeScope` takes the mode; `changedStages`; unit/vet/lint narrowing; scoped record; `CheckCertificate` knows the mode.
   - Tests: AC-12..AC-14 rows.
   - Verify: scoped package tests for `verify/engine`, `verify/deps`, `verify`.
5. **Phase: Push debt** -- `deriveDebt` reads the scoped record; anchor condition; refusal and session-start wording; rewrite "Verification debt, derived" and the debt rule point.
   - Tests: AC-15..AC-20 rows.
   - Verify: scoped package tests for `commit`, `hookruntime`.
6. **Phase: Surfaces** -- every rule, skill, template and doc in Files to Modify; `./le ai rules condensed-update`; grep proves AC-21.
   - Verify: `grep -rnE "verify (current|worktree)" ai .claude/skills plan/templates docs/contributing` shows only release-check wording; `./le ai rules render-check`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The anchor never answers empty for a non-empty change; an unreadable process table refuses; the requester never refuses itself |
| Correctness | The push walk accepts a record only when `A` is an ancestor-or-equal of the current `@{u}` |
| Naming | `changed` is the mode, verb and record name everywhere; refusals follow `ai/rules/user-facing-errors.md` |
| Rule: no-layering | No code path still reads the certificate for an anchor; the green record is read by nothing on the push path |
| Rule: documentation | Each phase lands its page edits with its code |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Git anchor, no certificate anchor | `grep -rn greenBaseline internal/` empty; AC-1 test |
| Idle refusal on every whole-tree entry | AC-8 tests per verb |
| Scoped push evidence | AC-15..AC-18 tests |
| Surfaces aligned | AC-21 grep |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `commit <rev>` resolved with `git rev-parse --verify <rev>^{commit}` before use; argv passed as a slice, never through a shell |
| Fail-open | An unreadable process table, registry, record or upstream refuses, never admits or reads as nothing owed |

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

- The certificate (`tmp/ze-verify.status`) is overwritten by every run, red or green, so anchoring scope on it made one interrupted run (exit 130) widen every later scoped run until someone ran a full verify. The anchor belongs to git, which no run can overwrite.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Anchor = `merge-base(HEAD, @{u})`, else `origin/HEAD`, else refuse | Anchor = last green full verify (today); anchor = HEAD only (working tree); anchor = a scoped-run record | The upstream is what a push compares with and what no run can overwrite. HEAD-only drops committed-unpushed work from scope. A scoped-run anchor reintroduces the certificate loop. Forecloses: re-judging already-pushed commits day to day (A-1) |
| Push owes a recorded scoped run at the pushed commit (`./le verify changed`) | Push owes nothing but person acts, scoped tests left to rule discipline | O-1 says a push owes scoped evidence; a rule nobody checks is the unenforced state the push gate exists to end. Owner decision Q-1 |
| Scoped push run in a throwaway worktree | In place over the working tree | The shared checkout carries other sessions' edits; the push publishes a commit, so the evidence must judge that commit's tree |
| Full run publishes `./...` | Keep narrowing from the change set | A release check that narrows from an anchor at the upstream judges almost nothing (D-4) |
| Idle = no foreign Claude CLI, no live registry holder, no Go-cache reader, outside own ancestry, machine-wide | This checkout's session directories (`session.Judge`); registry only | Load is machine-wide; other checkouts' builds load the same CPU. `session.Judge` answers which session directories to keep, a different question |
| Reuse `readsCache` for "development job" | A new list of heavy programs | One declaration; scratch already decides "a build is running" from it |

## Known Limitations
- A commit already on the upstream is not re-judged day to day; the release check is the backstop (A-1, by O-2).
- The idle check runs at admission only (Q-4).

## Owner Decisions Pending

| ID | Question | Options | What each changes |
|----|----------|---------|-------------------|
| Q-1 | What does a push owe? | (a) a green `./le verify changed` record at the pushed commit (recommended); (b) only the person acts, scoped tests by rule | (a) one scoped worktree run before each push, enforced; (b) no run, nothing checks the evidence |
| Q-2 | Can the idle refusal be overridden? | (a) never (recommended); (b) an owner-only keyword carrying his words, written to the green record | (a) the owner closes sessions to run it; (b) an agent can type the keyword, which O-3 forbids leaving to judgement |
| Q-3 | Which agent processes count as a live session? | (a) any `claude` CLI, idle or not (recommended); (b) only one whose transcript changed in the last N minutes; (c) (a) plus other agent CLIs (`codex`, ...) | (a) an open idle window blocks; (b) needs a window choice and reads transcripts; (c) one more name in the declared list |
| Q-4 | A session that starts after admission | (a) the run continues (recommended); (b) the run aborts | (b) a release check can be killed by any session start |
| Q-5 | Does `./le verify current fast` count as a full verification? | (a) yes, idle-gated (recommended: it lints the whole module); (b) no | (b) leaves one whole-tree run agents can start any time |
| Q-6 | Do `./le test unit all` and an unscoped `./le go lint run` get the same idle gate? | (a) yes, in this spec; (b) no | (a) every whole-tree check `ai/META-DECISIONS.md` names is enforced; (b) those stay rule-only |

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
- [ ] AC-1..AC-21 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The scoped package tests for every package this spec changes pass, run through `./le job run`; no whole-tree run is part of this spec's closure (O-1, O-2)
- [ ] Feature code integrated (`internal/le/*`), not library-only
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
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: tooling)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-verify-release-check-only.md` only, in the same `./le commit create` script
