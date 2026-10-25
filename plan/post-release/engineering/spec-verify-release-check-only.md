# Spec: verify-release-check-only

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Release-independent tooling: the development loop must stop running the whole-tree verification day to day, which loads the shared machine and blocks every other session. No shipped behavior changes. |
| Depends | - |
| Phase | 4/6 |
| Handoff | - |
| Updated | 2026-10-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner decided (`ai/META-DECISIONS.md`, "Ze is pre-release", commits `fa2e30b6d4` and `c107120b68`):

| # | Decision |
|---|----------|
| O-1 | Day-to-day work, its closure, a commit and a push owe only the tests of the components the change reaches (BGP, OSPF, IS-IS, a plugin, the tooling), found from the change itself: the packages it reaches, the feature tags those packages build under, and the functional suites that exercise them. |
| O-2 | The full verification (`./le verify current`, `./le verify worktree`) is a release check. It is never run day to day, and an agent never runs it: when one is owed, the agent gives Thomas the exact command and he runs it himself, so it runs once. The tooling refuses a full verification started without an interactive terminal. |
| O-3 | The tooling holds no idle check. Several sessions run at once nearly all the time, Thomas starts new ones, agents start agents and he tests by hand, so a check of what runs at start answers nothing about the run that follows. Only Thomas judges when a full verification can run; the human-only gate of O-2 is the control. |

The rules were rewritten to match in those commits. The tooling was not, and it contradicts all three decisions:

| Defect | Where | Effect |
|--------|-------|--------|
| D-1 | `greenBaseline` (`internal/le/repo/changed/selector.go`) anchors the change set at the commit `tmp/ze-verify.status` records, and only when that run exited 0. | With no green certificate (today: `exit=130`, `git_sha=deb6ee4359`), `./le repo changed scope`, `packages`, `groups` and the race pass all answer `./...` and every feature tag. Every "scoped" run is then a full run, and the only way out is a full verify. That is the loop the owner's decision breaks. |
| D-2 | `deriveDebt` (`internal/le/commit/owed.go`) refuses every push that no green full verify covers, and the session-start hook prints that debt in every session. | A push owes the full verify, against O-1 and O-2. |
| D-3 | `currentHere` and `runHere` (`internal/le/verify/actions.go`) and the `verify evidence release-candidate` action start a whole-tree run for any caller, an agent's tool call included. | An agent starts a full verification on its own, against O-2. This is the load the machine carries right now. |
| D-4 | A full run narrows its Staticcheck rows, race pass and functional suites from the change set since the green commit (`publishChangeScope`, `internal/le/verify/engine/scope.go`, which takes no mode). | Once the change set is anchored on git, a release check over a clean, pushed tree would select almost nothing. A release check must judge the whole tree. |
| D-5 | Agent-facing text tells the agent to run the full gate: `clearingRuns`, `unjudgedCause` and `rerunAllStages` in `./le commit create`, the push refusal (`noTargetRefusal`, `verifyWorktreeCommand`), the session-start debt line, the retired-keyword replacements in `internal/le/commit/actions.go`, the reds report, the pretool hook's example, skills, rule points and docs. | Each tells an agent to start the run O-2 forbids it. |

Goal: the change set is anchored on git, so scoped selection never widens merely because no full verify passed; a push owes scoped evidence over the commits it publishes, produced by one native run an agent may start; the full verification judges the whole tree and refuses unless a person at an interactive terminal starts it; every surface that owed an agent the full run names the scoped run, or the exact command to give Thomas.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the certificate, the change-set selection, the green record and the derived debt
  → Constraint: the change-set selector widens and NAMES its reason on stderr; it must never select nothing ("Producer contract"). The new anchor keeps that: a missing anchor refuses, it never answers empty.
  → Constraint: `IsProse` (declared on the `ai/`, `plan/`, `docs/` Markdown entries of `nonGoPathRules`) is the one prose predicate. The push walk keeps using it; the commit package holds no path list.
  → Decision: the push-target walk (`chooseTarget`, `walkFromGreen`, `covered`) is kept as is and fed a different record. Only the record and its anchor condition change.
  → Constraint: the full mode's stage population is named by `StagesForMode`; a mode name it does not know is STALE in `CheckCertificate`. The new scoped mode must be known to `StagesForMode` or its certificate reads as no pass.
- [ ] `docs/contributing/running-commands.md` - "The changed-set selector", "One verify at a time"
  → Constraint: a job started inside another job's slot runs straight through (`job` parent detection). The terminal gate runs at the verb's action handler, before admission, never inside a stage, so the worktree run that `verify current` delegates to is judged once.
  → Decision: the page says "Suite selection is not scoped" while `verify-freshness-scope.md` says the suite map narrows functional suites. One of them is wrong; Phase 2 reads `selectSuites` and repairs the wrong page (A-5).
- [ ] `ai/rules/user-facing-errors.md` - every refusal says what is wrong, why it matters, and the command that fixes it on its own line
  → Constraint: the terminal refusal says the run is a release check only Thomas starts and that this process has no interactive terminal (naming which of stdin and stdout is not one), why (a full verification loads the whole shared machine for its length and only Thomas knows when that is safe), then two command lines: the exact command line this invocation was given, to hand to Thomas, and `./le verify changed`, the scoped run the agent owes itself.
- [ ] `ai/rules/commands.md`, `ai/rules/planning.md`, `plan/README.md` - command discipline, phase rules, bucket
  → Decision: bucket `post-release/engineering`, P3 under the README's priority table (tooling, no release correctness). The owner may raise it.
- [ ] `ai/rules/no-layering.md` - replacing the green-baseline anchor with the git anchor
  → Constraint: `greenBaseline` and `errNoGreenBaseline` are deleted before the anchor is written; no fallback reads the certificate.

**Key insights:**
- The selector's anchor, not the selector, causes the loop: its rules already map each path to the packages and tags it reaches.
- Every narrowing stage already reads one published scope (`ZE_VERIFY_SCOPE_PACKAGES`, `ZE_VERIFY_SCOPE_TAGS`); a full run publishing `./...` judges the whole tree with no stage edited.
- `./le go lint run` already accepts `scope packages` (`internal/le/go/lint/actions.go`), so a scoped lint needs no new linter code.
- The push walk is sound; only what it reads (green full record) is wrong for O-1.
- An agent's Bash tool call runs with stdin and stdout as pipes; Thomas's terminal gives a terminal on both. `golang.org/x/term` is already a dependency (`internal/le/interoplab/process.go`, `sharesTerminal`), so the gate needs no new module.
- CI (`.github/workflows/verify.yml`) never calls a whole-tree entry: it lists the stages with `./le verify list full` and runs each stage action, so the gate does not reach it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/repo/changed/selector.go` - `resolveSelector` calls `changedPaths`, which calls `gitChangedPaths`: the three working-tree queries plus `git diff <greenBaseline> HEAD`. `greenBaseline` reads `tmp/ze-verify.status` (or `ZE_VERIFY_STATUS_FILE`), requires `exit=0` and a `git_sha` the repository holds as a commit, else returns `errNoGreenBaseline`, which `resolveSelector` turns into `failOpen(everyTag)`: `./...` and every feature.
- [ ] `internal/le/repo/changed/changed.go` - `Selector.Select` (race groups): honors a published `./...` scope, then the same `greenBaseline`; on `errNoGreenBaseline` it answers `Rest: ./...`.
- [ ] `internal/le/verify/actions.go` - registers `worktree` (keywords `commit`, `keep`, `first-red`, `part`, `of`, `all`; handler `runHere`) and `current` (`full`, `fast`, `first-red`; handler `currentHere`). Neither handler looks at who called it.
- [ ] `internal/le/verify/current.go` - `runFast` admits label `verify` through `job.NewIn(root).Admit`, attaches to an equivalent run, takes a slot, runs `RunMode`; `current full` runs the worktree run.
- [ ] `internal/le/verify/lifecycle.go` - `Run` and `admitWorktree` admit the worktree run the same way; `run` publishes the certificate and calls `recordGreen`.
- [ ] `internal/le/verify/sweep.go` - `Sweep` runs `all of m` piece by piece.
- [ ] `internal/le/verify/engine/scope.go` - `publishChangeScope(root, logDir)` runs the selector once and publishes its answer; a widened answer is published as it stands; it takes no mode.
- [ ] `internal/le/verify/green.go` - green record `tmp/ze-verify-green.status` (commit, tree, mode full, skipped empty, cut), written only for a full, uncut-or-swept, unskipped green run; parts record for `part i of m`.
- [ ] `internal/le/verify/evidence/evidence.go` - `verify evidence release-candidate`: clones the checkout into a container (`interoplab.OneShotContainer`, no `-t`) and runs `./le verify current` there (`ContainerScript`), skipping the firewall and web suites.
- [ ] `internal/le/interoplab/process.go` - `sharesTerminal` asks `term.IsTerminal` of a process's streams: the precedent for the terminal question.
- [ ] `internal/le/commit/owed.go` - `deriveDebt`: reads the green record, chooses push target P (HEAD when the green tree covers HEAD's tree, else the walk from the green commit over prose-only commits, else none), lists left-behind non-prose commits, reads person-act trailers. `pushRefusal` refuses on any refusal, an empty `Published`, or an open person act. `noTargetRefusal` names `verifyWorktreeCommand` (`./le verify worktree`).
- [ ] `internal/le/commit/verification.go` - `rerunAllStages` is `le verify worktree`; `unjudgedCause` and `clearingRuns` tell the reader an unjudged stage clears when `le verify worktree`, a `part <i> of <n>` piece or `le verify current fast` reaches it; the interrupted-index line says "rewrite it with: le verify current".
- [ ] `internal/le/commit/actions.go` - the retired keywords `unverified` and `missing-full-verify-ok` name `./le verify worktree` as their replacement.
- [ ] `internal/le/commit/prepare.go` - `checkVerificationGates` never refuses a commit for a missing verification; it charges recorded structural reds only.
- [ ] `internal/le/commit/push.go` - `DebtWarning`, the session-start debt line.
- [ ] `internal/le/hookruntime/lifecycle.go` - `reportDerivedDebt` prints `DebtWarning` at session start; a hint at line 801 names `./le verify current fast` as an agent's scoped run.
- [ ] `internal/le/hookruntime/bash.go` - the lossy-filter refusal's example command is `./le verify worktree` redirected to a log.
- [ ] `internal/le/verify/reds.go` - the first-red report ends with "le verify worktree runs every stage".

**Behavior to preserve:**
- Every path rule in `nonGoPathRules` and the "What each changed path selects" table, except its last row; the two-level importer walk; `drop-log`; `go.mod`/`go.sum`/`vendor/` widening.
- The selector widens and names its reason on stderr; it never answers an empty set for a non-empty change.
- `CheckCertificate`, the certificate and manifest format, and the structural-red charging at commit time.
- The push walk's prose rule and the person-act trailers (`Review-owed:`, `RFC-approval-owed:`, `Owner-attested:`), unchanged.
- `part i of m` and `all of m` for the full run, and the green record as the release check's own record.
- The job registry, slots and attach semantics for every job.
- Every stage action CI runs one by one, ungated.
- `./le test unit all` and an unscoped `./le go lint run`, unchanged (decision Q-6).

**Behavior to change:**
- The change set's committed term is anchored on git, not on the certificate (D-1).
- A push owes a scoped verify record covering the commits it publishes, not a green full verify (D-2).
- Every whole-tree verify entry refuses unless stdin and stdout are an interactive terminal (D-3).
- A full run judges the whole tree (D-4).
- Every agent-facing surface names `./le verify changed`, or the exact full-run command to give Thomas (D-5).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le repo changed scope|packages|groups|group-packages`: change set from git.
- `./le verify changed [commit <rev>]` (new): scoped run in a throwaway worktree; an agent may start it.
- `./le verify current`, `./le verify current fast`, `./le verify worktree [...]` (`part i of m`, `all of m` included), `./le verify evidence release-candidate`: whole-tree runs, terminal-gated.
- `./le commit create`, `./le commit push "<authorisation>"`, `./le commit debt-status`, session-start hook: the derived debt and the text that names the clearing run.

### Transformation Path
1. Anchor: `A = merge-base(HEAD, U)`, where U is `@{u}` when the branch has one, else `refs/remotes/origin/HEAD`. Neither resolves: refusal naming `git branch --set-upstream-to=origin/<branch>`.
2. Change set: working-tree queries (unchanged) plus `git diff --name-only A HEAD`; for a run pinned to a commit C, `git diff --name-only A C` and no working-tree term.
3. Classification and importer walk: unchanged (`classifyPaths`, `seedPackages`).
4. Scoped run (`changed` mode): publishes the scope, runs the scope-reading population, writes the certificate (mode `changed`) and the scoped record `tmp/ze-verify-changed.status` (commit C, tree, anchor A, upstream U's SHA at the run, timestamp).
5. Push: `deriveDebt` reads the scoped record instead of the green record. P is chosen by the existing walk from C. The record proves P only when A is an ancestor of, or equal to, the current `@{u}`, so `A..C` holds every commit `@{u}..P` publishes.
6. Full run: the action handler asks whether stdin and stdout are both terminals; either one is not, it refuses with exit 2 before admission, before a worktree or container exists, and prints the exact command line it was given for Thomas. Admitted, the run publishes `./...` and every tag, so every narrowing stage judges the whole tree, and writes the green record as today.
7. Release-candidate evidence: the host side is gated as in step 6; the container is started with a pseudo-terminal (`docker run -t`), so the `./le verify current` inside it meets the same gate and passes, because a person at a terminal started the host side.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ git | `git merge-base`, `git diff --name-only -z`, `git rev-parse` through `runGit` | No |
| le ↔ controlling terminal | `term.IsTerminal` on the action's stdin and stdout file descriptors | No |
| le ↔ docker | `docker run -t` for the release-candidate container | No |
| verify run ↔ stage children | `ZE_VERIFY_SCOPE_PACKAGES`, `ZE_VERIFY_SCOPE_TAGS` published files | No |
| verify ↔ commit | `tmp/ze-verify-changed.status` read by `deriveDebt` | No |

### Integration Points
- `resolveSelector`, `gitChangedPaths`, `Selector.Select` - read the anchor instead of `greenBaseline`.
- `publishChangeScope` - takes the mode, and publishes the whole tree for the full population.
- `StagesForMode`, `fullStages`, `fastStages` - gain a `changed` population.
- `currentHere`, `runHere` and the evidence action handler - call the terminal gate first.
- `deriveDebt`, `readGreen`, `noTargetRefusal`, `pushRefusal`, `DebtWarning` - read and name the scoped record.
- `clearingRuns`, `unjudgedCause`, `rerunAllStages`, the retired-keyword table - name the scoped or focused run, and the full run only as a command for Thomas.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The scoped run goes through the same engine (`RunMode`), certificate and publication as the full run |
| No unintended coupling (components stay isolated) | Yes | The terminal gate is one leaf package, `internal/le/le/human`, beside `tail` and `path`; `verify` and `verify/evidence` import it, it imports neither |
| No duplicated functionality (extends existing, does not recreate) | Yes | The push walk, the selector rules and the worktree lifecycle are reused; the gate is one predicate and one refusal, called by every whole-tree handler |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling, no wire path |
| Registration over hardcoding, outbound | Yes | The `changed` population is declared with `stage(...)` like the others; the verb registers in `internal/le/verify/actions.go` |
| Registration over hardcoding, inbound | Yes | `StagesForMode` is the one mode table. Which verbs are whole-tree is declared on each action where it registers (a field of `leaction.Action` or a wrapper over its handler), never in a list beside the registry; `TestEveryWholeTreeVerbIsGated` walks the registered verify actions and fails on a full-population verb that does not refuse |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Commits already on the upstream were proven when they were pushed, so the anchor may exclude them. | O-1; the push gate this spec writes | A regression already pushed is caught only by the release check | Stated as a Known Limitation; the release check is the backstop by O-2 | unvalidated |
| A-2 | Every narrowing stage reads only the published scope, so publishing `./...` makes the full run whole-tree with no stage edit. | `verify-freshness-scope.md`; `scopeMatrix`, `Selector.Select`, `selectSuites` | A stage that narrows from its own source keeps narrowing the release check | `TestFullModePublishesWholeTree` asserts the three stages' populations; grep for every reader of `greenBaseline` and `ScopeFileKey` | unvalidated |
| A-3 | An agent's tool call reaches the le binary with a stdin or stdout that is not a terminal, and Thomas's shell reaches it with both. The `./le` launcher passes its stdio to the binary unchanged. | The Bash tool runs commands with pipes; `./le` is a POSIX shell script that runs the binary as a child | An agent passes the gate, or Thomas is refused | One run from an agent session (refused) and one from Thomas's terminal (admitted), recorded in the spec at Phase 3 | half confirmed: in an agent Bash call `[ -t 0 ]` and `[ -t 1 ]` are both false, and `./le` ends in `exec "$binary" "$@"`, so the binary gets the tool call's pipes unchanged. Owed: one gated run from Thomas's terminal, admitted |
| A-4 | `./le go lint run scope packages` lints exactly the named packages. | `internal/le/go/lint/actions.go` keyword `scope packages`, `planScope` | The scoped run lints the whole module | Read `planScope` in Phase 4; `TestChangedPopulationReadsScope` | unvalidated |
| A-5 | `selectSuites` narrows functional suites by the suite map today. | `verify-freshness-scope.md` "What a suite REACHED" vs `running-commands.md` "Suite selection is not scoped" | The scoped run runs every functional suite, which is a near-full run | Read `selectSuites` in Phase 2; repair the wrong page | confirmed: `selectSuites` (`internal/le/test/functional/suitemap.go`) narrows from the recorded suite map unless recording, the map is unreadable, or the selector refused or widened; `running-commands.md` "Suite selection is not scoped" was the wrong page and is rewritten |
| A-6 | `refs/remotes/origin/HEAD` exists in a linked worktree with no upstream. | `git clone` sets it; worktrees share refs | A worktree agent's scoped run refuses | `TestAnchorFallsBackToOriginHead`, `TestAnchorRefusesWithNoUpstream` | confirmed: this checkout's `refs/remotes/origin/HEAD` is a symbolic ref to `refs/remotes/origin/main` (`git symbolic-ref`), refs are shared by linked worktrees, and `TestAnchorFallsBackToOriginHead` passes |
| A-7 | `docker run -t` without `-i` gives the container process a terminal on stdin and stdout, and the docker client accepts it when its own stdout is the progress writer. | Docker CLI: `-t` allocates a pseudo-TTY; only `-i` with a non-TTY stdin is refused | The in-container run is refused, and the release-candidate evidence cannot run | `TestEvidenceContainerHasTerminal` asserts `-t` in the arguments; one release-candidate run by Thomas, recorded at Phase 3 | half confirmed: `TestEvidenceContainerHasTerminal` passes and goes red with `-t` removed. Owed: one `./le verify evidence release-candidate` run by Thomas |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A long unpushed range (main is 94 ahead today) reaches `go.mod` or `vendor/` and the scoped run widens to `./...`. | The widening line on stderr names the path | Correct by the existing rule: a dependency moved, every package is reachable. The scoped run is admitted through slots, not terminal-gated |
| R-2 | An agent wraps the command in something that fakes a terminal (`script`, `expect`, a pty library). | A full run in the job registry while no person ran one | The gate stops a tool call, it is not a security boundary; the rules forbid an agent the full run (O-2). No override keyword exists, so no surface teaches a bypass. Stated as a Known Limitation |
| R-3 | CI or a harness that calls a whole-tree verb is refused. | CI red at the verb | CI runs stage actions one by one, never a whole-tree verb (Key insights). The evidence container gets a terminal (A-7). Phase 3 greps `.github/` and `internal/` for every caller of a whole-tree verb. Done: `.github/` names none; `internal/` names three functional fixture calls (`misc_fixture_runner_derived_debt.go` cut forms, `misc_fixture_runner_first_red.go` `worktree part 1 of 2 first-red` and `current fast first-red`), which now run le under a pseudo-terminal (`terminalCommand`); `fastArgv` in `current.go` is a fingerprint, not a call |
| R-4 | Existing tests that drive `runHere` or `currentHere` through the action are refused, because `go test` has no terminal. | Red in `internal/le/verify` tests | The gate takes its terminal question as an injected predicate; each such test answers "interactive" and is named in the phase with why its old setup no longer holds (`ai/rules/testing.md`). No assertion is weakened |
| R-5 | The terminal question itself fails (descriptor unreadable). | Error from the predicate | REFUSE, naming the error; never read as a terminal |
| R-6 | Removing `greenBaseline` breaks a test that drives the selector over a fixture certificate. | Red in `internal/le/repo/changed` tests | Those tests change their fixture to a git anchor; each changed test is named with why its old expectation no longer holds (`ai/rules/testing.md`) |
| R-7 | The push gate accepts a scoped record whose anchor predates commits another session pushed meanwhile. | `@{u}` moved since the run | The anchor condition (A ancestor-or-equal of current `@{u}`) plus the fast-forward requirement refuse; `TestPushRefusesWhenUpstreamMovedPastAnchor` |
| R-8 | Surfaces (skills, template, rule points, le messages) keep telling agents to run the full gate. | grep finds `verify current` / `verify worktree` addressed to an agent | Phase 6 rewrites every surface the grep in Files to Modify names, and `./le ai rules condensed-update` regenerates CORE/TRIGGERS; AC-22 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: a push could go out with too little evidence (wrong anchor), or Thomas could be refused a full run from his own terminal. No shipped binary changes. |
| How is it reverted? | Single-commit revert per phase. |
| Who else touches this path? | `spec-verify-scope-5-suite-coverage-map` (ready; suite map, `selectSuites`), `spec-native-action-job-admission` (skeleton; admission rollout), `spec-parallel-agents-verify-a-tree-nobody-authored` (skeleton). None edits the anchor, the debt or a caller gate. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le repo changed scope print packages` with no certificate | → | anchor in `resolveSelector` | `TestScopeAnchoredOnUpstreamWithNoCertificate` (`internal/le/repo/changed`) |
| `./le repo changed groups` with no certificate | → | anchor in `Selector.Select` | `TestRaceGroupsAnchoredOnUpstream` (`internal/le/repo/changed`) |
| `./le verify worktree` with stdin and stdout pipes | → | terminal gate in `runHere` | `TestVerifyWorktreeRefusesWithoutTerminal` (`internal/le/verify`) |
| `./le verify current` and `current fast` with pipes | → | terminal gate in `currentHere` | `TestVerifyCurrentRefusesWithoutTerminal` (`internal/le/verify`) |
| `./le verify worktree part 2 of 6` and `all of 6` with pipes | → | terminal gate in `runHere` before `Sweep` | `TestSweepRefusesWithoutTerminal` (`internal/le/verify`) |
| `./le verify evidence release-candidate` with pipes | → | host-side terminal gate before `docker run` | `TestEvidenceRefusesBeforeDockerWithoutTerminal` (`internal/le/verify/evidence`) |
| every registered verify verb | → | whole-tree declaration on the action | `TestEveryWholeTreeVerbIsGated` (`internal/le/verify`) |
| `./le verify changed` with pipes | → | `changed` population, scoped record write, no gate | `TestVerifyChangedWritesScopedRecord` (`internal/le/verify`) |
| `./le commit push "<auth>"` after `verify changed` | → | `deriveDebt` over the scoped record | `TestPushAdmitsScopedRecordCoveringHead` (`internal/le/commit`) |
| `./le commit create` over an unjudged stage | → | `unjudgedCause`, `clearingRuns` wording | `TestUnjudgedStageNamesNoAgentFullRun` (`internal/le/commit`) |
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
| AC-8 | Any whole-tree verify entry (`verify current`, `verify current fast`, `verify worktree` with any of `commit`, `keep`, `first-red`, `part i of m`, `all of m`, and the host side of `verify evidence release-candidate`) with stdin or stdout not a terminal | Refused with exit 2 before admission, before a worktree, a job slot or a container is created. The message says the run is a release check only Thomas starts, names which of stdin and stdout is not a terminal, says why (a full run loads the whole shared machine and only Thomas knows when that is safe), then gives on their own lines the exact command line this invocation was given, to hand to Thomas, and `./le verify changed` for the agent's own evidence. |
| AC-9 | Same entries, the terminal question errors | Refused the same way, naming the error; never treated as a terminal. |
| AC-10 | Same entries with stdin and stdout both terminals | Admitted and run, with no keyword or environment variable involved. |
| AC-11 | Any keyword, flag or environment variable offered to skip the gate | None exists: the action's parameter table and the env registry hold no override, which `TestEveryWholeTreeVerbIsGated` asserts for the parameters. |
| AC-12 | A full run admitted | `scope-packages.txt` holds `./...`, `scope-tags.txt` every tag; the Staticcheck matrix judges every row, the race pass races `./...`, every functional suite runs. `verify current fast` keeps its own population and scope. |
| AC-13 | `./le verify changed` (default `commit HEAD`) with stdin and stdout pipes | Not refused. Runs in a throwaway worktree at the commit, admitted through the job slots; scope `A..C`; population: unit tests and the bare `ze_core` compile-out over the scope packages under the scope tags, the race groups, the Staticcheck rows for the scope tags, `go vet` and `go lint run scope packages` over the scope packages, the functional suites `selectSuites` picks. No stage judges the whole tree unless the selector widened, and then stderr names why. |
| AC-14 | `./le verify changed` green | Certificate mode `changed` published to the checkout; `tmp/ze-verify-changed.status` records commit, tree, anchor, upstream SHA, timestamp. `CheckCertificate` knows the `changed` population. A red run writes no scoped record. |
| AC-15 | Push of HEAD with a scoped record for HEAD whose anchor is an ancestor-or-equal of `@{u}` and no open person act | `./le commit push` prepares the script; no full verify is asked. |
| AC-16 | Same, with prose-only commits after the record's commit | The walk carries them, as today. |
| AC-17 | Scoped record whose anchor is not an ancestor of the current `@{u}` | Push refused; the message says the upstream moved past the run's anchor and gives `git pull --rebase --no-autostash` then `./le verify changed`. |
| AC-18 | No scoped record | Push refused; message names `./le verify changed`, never `./le verify worktree`. |
| AC-19 | Session start with unpushed non-prose commits and no covering scoped record | The debt line reads that N commits owe a scoped verify and names `./le verify changed`; it never names a full verify. |
| AC-20 | `./le commit create` | Unchanged: never refuses for a missing verification. |
| AC-21 | `./le commit create` over an unjudged or interrupted structural stage | The `verify=STALE` text names the stage's focused command and `./le verify changed` as what the agent runs; where only a full run clears the entry, it says to give Thomas the exact command (`./le verify worktree`, or the `part <i> of <n>` piece) and never tells the agent to run it. The retired keywords `unverified` and `missing-full-verify-ok` name `./le verify changed`. |
| AC-22 | Every surface listed in Files to Modify (le messages, hook text, docs, rule points, skills, template, `ai/INSTRUCTIONS.md`) | No surface tells an agent to run `./le verify current`, `current fast` or `./le verify worktree`; each names the scoped evidence instead and names the full run only as a release check whose exact command the agent gives Thomas. |

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
| `TestGateRefusesEachNonTerminalStream` | `internal/le/le/human/human_test.go` | AC-8: stdin only, stdout only, both; the refusal names the stream, the reason, the given command line and `./le verify changed` on their own lines | |
| `TestGateRefusesWhenQuestionErrors` | `internal/le/le/human/human_test.go` | AC-9 | |
| `TestGateAdmitsTwoTerminals` | `internal/le/le/human/human_test.go` | AC-10 | |
| `TestVerifyWorktreeRefusesWithoutTerminal` | `internal/le/verify/gate_test.go` | AC-8 through the verb with `commit`, `keep`, `first-red`; no worktree and no job slot created | |
| `TestVerifyCurrentRefusesWithoutTerminal` | `internal/le/verify/gate_test.go` | AC-8 for `current full` and `current fast` | |
| `TestSweepRefusesWithoutTerminal` | `internal/le/verify/sweep_test.go` | AC-8 for `part i of m` and `all of m` | |
| `TestEveryWholeTreeVerbIsGated` | `internal/le/verify/gate_test.go` | AC-8, AC-11 over the registered actions | |
| `TestEvidenceRefusesBeforeDockerWithoutTerminal` | `internal/le/verify/evidence/evidence_test.go` | AC-8, recorder sees no docker call | |
| `TestEvidenceContainerHasTerminal` | `internal/le/verify/evidence/evidence_test.go` | A-7: the container arguments carry `-t` | |
| `TestFullModePublishesWholeTree` | `internal/le/verify/engine/scope_test.go` | AC-12 | |
| `TestChangedPopulationReadsScope` | `internal/le/verify/engine/stages_test.go` | AC-13: every stage of the population reads the published scope | |
| `TestVerifyChangedWritesScopedRecord` | `internal/le/verify/changed_test.go` | AC-13 ungated, AC-14, red writes none | |
| `TestCheckCertificateKnowsChangedMode` | `internal/le/verify/engine/status_test.go` | AC-14 | |
| `TestPushAdmitsScopedRecordCoveringHead` | `internal/le/commit/owed_test.go` | AC-15 | |
| `TestPushCarriesProseAfterScopedRecord` | `internal/le/commit/owed_test.go` | AC-16 | |
| `TestPushRefusesWhenUpstreamMovedPastAnchor` | `internal/le/commit/owed_test.go` | AC-17 | |
| `TestPushRefusalNamesVerifyChanged` | `internal/le/commit/owed_test.go` | AC-18 | |
| `TestUnjudgedStageNamesNoAgentFullRun` | `internal/le/commit/verification_test.go` | AC-21 | |
| `TestRetiredKeywordsNameVerifyChanged` | `internal/le/commit/actions_test.go` | AC-21 | |
| `TestSessionStartDebtLineNamesVerifyChanged` | `internal/le/hookruntime/lifecycle_test.go` | AC-19 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | - | - | - | - |

No new numeric input: `part i of m` keeps its existing parsing and tests; the gate runs before it.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | Tooling: the entry point is the le action `Answer`, which the wiring tests above drive; there is no `.ci` surface for le verbs | |

## Files to Modify
- `internal/le/repo/changed/selector.go` - delete `greenBaseline`, `errNoGreenBaseline`; `gitChangedPaths` uses the anchor; package comment's "A fourth widens" paragraph rewritten.
- `internal/le/repo/changed/changed.go` - `Select` uses the anchor; package comment line 8.
- `internal/le/verify/actions.go` - `currentHere` and `runHere` call the gate first; the whole-tree declaration on `current` and `worktree`; the `changed` verb.
- `internal/le/verify/current.go`, `lifecycle.go`, `sweep.go` - comments and refusal text that name an agent's full run.
- `internal/le/verify/reds.go` - the first-red report names the full run as a command for Thomas.
- `internal/le/verify/evidence/actions.go`, `evidence.go` - host-side gate before docker; `-t` in the container arguments; `ContainerScript` comment says why the inner run passes the gate.
- `internal/le/verify/engine/scope.go` - `publishChangeScope` takes the mode: whole-tree publication for the full population, anchor scope for `changed`.
- `internal/le/verify/engine/stages.go` - `changedStages`, known to `StagesForMode`.
- `internal/le/verify/deps/verifydeps.go` - unit and vet stages over the published packages in `changed` mode.
- `internal/le/verify/green.go` - scoped record writer and reader beside the green record.
- `internal/le/commit/owed.go`, `push.go` - read the scoped record, anchor condition, refusal and debt wording; `verifyWorktreeCommand` replaced by the scoped command.
- `internal/le/commit/verification.go` - `rerunAllStages`, `clearingRuns`, `unjudgedCause` and the interrupted-index line: the `verify=STALE` text `./le commit create` prints today tells the agent an unjudged stage "clears when a verify run reaches it: le verify worktree, ... or le verify current fast" (observed 2026-10-24 on this spec's own commit). It names the focused single-stage command and `./le verify changed`, and the full run only as the command to give Thomas.
- `internal/le/commit/actions.go` - retired keywords `unverified` and `missing-full-verify-ok` name `./le verify changed`.
- `internal/le/hookruntime/lifecycle.go` - the debt line and the line 801 hint name `./le verify changed`, not `current fast`.
- `internal/le/hookruntime/bash.go` - the lossy-filter refusal's example names `./le verify changed`.
- Every other le message found in Phase 6 by grepping `internal/le` for the two full-run command names outside comments (today also `internal/le/setup/setup.go`, `internal/le/setup/claude_server.go`); a line that describes what the release check runs (`internal/le/rfc/*`, `internal/le/test/health/render.go`) stays when it tells no agent to run it.
- `docs/architecture/testing/verify-freshness-scope.md` - "One change-set selection", the path table's last row, "The green record and the cut" (release record), "Verification debt, derived".
- `docs/contributing/running-commands.md` - "The changed-set selector", "One verify at a time", a new "A full verification is Thomas's to start" (the gate, the refusal, the evidence container's terminal).
- `docs/architecture/testing/ci-format.md` - the release-candidate description gains the host-side gate and the container terminal.
- `docs/contributing/committing.md`, `docs/contributing/testing.md`, `docs/contributing/spec-workflow.md`, `docs/contributing/claude-code-cheatsheet.md`, `docs/functional-tests.md` - push evidence and closure.
- `ai/INSTRUCTIONS.md` - the Worktrees bullet that tells agents the full gate runs through `verify current` and that `current fast` is their in-place run; then regenerate `AGENTS.md`/`CLAUDE.md`.
- `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md`, `ai/rules/points/precommit-verify/directives/the-gate-runs-on-a-commit-not-the-working-tree.md`, `ai/rules/precommit-verify.md`, `ai/rules/completion.md`, `ai/rules/commands.md`, `ai/rules/points/git-safety/directives/push-only-on-the-owners-order.md`, then `./le ai rules condensed-update`.
- `plan/templates/TEMPLATE.md` - Goal Gates line naming the full gate at closure.
- `.claude/skills/ze-close/SKILL.md` (step 6f), `ze-verify`, `ze-implement`, `ze-commit-check`, `ze-check`, `ze-commit`, `ze-status`, `ze-progress`, `ze-test`, `ze-fix-alloc` - every step that runs or owes the full gate names the scoped run, or the command to give Thomas.

## Files to Create
- `internal/le/repo/changed/anchor.go`, `anchor_test.go` - the git anchor.
- `internal/le/le/human/human.go`, `human_test.go` - the terminal gate: one predicate over stdin and stdout, injectable for tests, and its refusal text.
- `internal/le/verify/changed.go`, `changed_test.go`, `gate_test.go` - the scoped verb and the verb-level gate tests.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling, no daemon config |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | Yes | `./le verify changed [commit <rev>]` in `internal/le/verify/actions.go`; no flag added to the gated verbs |
| CLI grammar (keyword before value) | Yes | `commit <rev>` keyword form, `ai/rules/cli.md` |
| Editor autocomplete | N-A | le verbs, not the ze editor |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | Yes | `./le verify changed` answers through the action table's `json`/`yaml`/`table` like `verify worktree` |
| Env var registration | N-A | No new variable, and no override variable for the gate (AC-11); `ZE_VERIFY_STATUS_FILE` stays for the certificate readers that remain |
| Doctor check for runtime dependencies | N-A | Not a daemon dependency; the gate is a refusal at le entry (`ai/rules/user-facing-errors.md`) |
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
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/testing.md`, `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` |
| 11 | Affects daemon comparison? | N-A | Tooling |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | No plugin inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/post-release/engineering/spec-verify-release-check-only.md` is run in Phase 1 and every declared doc named here before code; known now: `verify-freshness-scope.md`, `running-commands.md`, `docs/architecture/core-design.md` (job admission, evidence `// Design:` headers); `docs/architecture/testing/ci-format.md` (declared by `internal/le/verify/evidence/evidence.go`). Advisory `<!-- source: -->` mentions, each re-read in the phase that edits its file and corrected if it describes the full gate as day-to-day or as an agent's run: `docs/DESIGN.md`, `docs/architecture/plugin/feature-gates.md`, `docs/architecture/site-facts.md`, `docs/contributing/gh-pages.md` (all via `stages.go`), `docs/architecture/testing/interop.md`, `docs/architecture/testing/runner-architecture.md` (via `evidence.go`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every `./le verify worktree` / `verify current` example in the files above, rewritten to the scoped form or to the release-check command Thomas runs |

## Implementation Steps

1. **Phase: Wiring** -- register `verify changed`, the gate call sites and the anchor function as stubs; write the wiring tests, which fail.
   - Tests: every Wiring Test row.
   - Files: `internal/le/verify/actions.go`, `anchor.go`, `human.go` skeletons.
   - Verify: the verbs resolve; tests fail on behavior, not on missing symbols.
2. **Phase: Anchor** -- delete `greenBaseline`/`errNoGreenBaseline`, write the anchor, wire `resolveSelector`, `gitChangedPaths`, `Select`; repair the path table and the selector page sections in the same phase.
   - Tests: AC-1..AC-7 rows. Read `selectSuites` and settle A-5.
   - Verify: `./le job run label unit-pkg quiet command go test ./internal/le/repo/changed/...`.
3. **Phase: Terminal gate** -- `internal/le/le/human`; the whole-tree declaration on `current` and `worktree`; the gate first in `currentHere`, `runHere` and the evidence action; `-t` on the evidence container; existing action-level tests answer "interactive" (R-4). Grep `.github/` and `internal/` for every caller of a whole-tree verb (R-3). Write the running-commands and ci-format sections. Record A-3 and A-7.
   - Tests: AC-8..AC-11 rows, `TestEvidenceContainerHasTerminal`.
   - Verify: scoped package tests for `le/human`, `verify`, `verify/evidence`.
4. **Phase: Whole-tree full run and the `changed` population** -- `publishChangeScope` takes the mode; `changedStages`; unit/vet/lint narrowing; scoped record; `CheckCertificate` knows the mode.
   - Tests: AC-12..AC-14 rows.
   - Verify: scoped package tests for `verify/engine`, `verify/deps`, `verify`.
5. **Phase: Push debt and commit text** -- `deriveDebt` reads the scoped record; anchor condition; refusal and session-start wording; `clearingRuns`, `unjudgedCause`, the retired keywords; rewrite "Verification debt, derived" and the debt rule point.
   - Tests: AC-15..AC-21 rows.
   - Verify: scoped package tests for `commit`, `hookruntime`.
6. **Phase: Surfaces** -- every le message, hook text, rule, skill, template and doc in Files to Modify; `ai/INSTRUCTIONS.md`; `./le ai rules condensed-update`; grep proves AC-22.
   - Verify: `grep -rnE "verify (current|worktree)" ai .claude/skills plan/templates docs/contributing internal/le` shows only release-check wording addressed to Thomas or descriptions of what the release check runs; `./le ai rules render-check`.
   - Walked-into defect fixed here, not journalled (our code, related, one condition): `carryForward` (`internal/le/verify/engine/artifacts.go`) let a fast run's green on a shared stage replace a red a full run judged, though a fast run publishes the change set's scope as a changed run does. The ChangedMode rule now covers FastMode (`mode == ChangedMode || mode == FastMode`, not `mode != Mode`, because a cut full run's mode is its part name). `TestAFastGreenKeepsAFullRedOnASharedStage` (engine/carry_test.go), red with the rule restricted to ChangedMode.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The anchor never answers empty for a non-empty change; the gate refuses before any worktree, slot or container exists; a failed terminal question refuses |
| Correctness | The push walk accepts a record only when `A` is an ancestor-or-equal of the current `@{u}` |
| Correctness | No keyword, flag or environment variable skips the gate |
| Naming | `changed` is the mode, verb and record name everywhere; refusals follow `ai/rules/user-facing-errors.md` |
| Rule: no-layering | No code path still reads the certificate for an anchor; the green record is read by nothing on the push path |
| Rule: documentation | Each phase lands its page edits with its code |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Git anchor, no certificate anchor | `grep -rn greenBaseline internal/` empty; AC-1 test |
| Terminal refusal on every whole-tree entry | AC-8 tests per verb; `TestEveryWholeTreeVerbIsGated` |
| Scoped push evidence | AC-15..AC-18 tests |
| Surfaces aligned | AC-21 tests, AC-22 grep |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `commit <rev>` resolved with `git rev-parse --verify <rev>^{commit}` before use; argv passed as a slice, never through a shell |
| Fail-open | An unanswerable terminal question, an unreadable record or a missing upstream refuses, never admits or reads as nothing owed |
| Bypass | The gate has no override; the command line echoed in the refusal is printed, never executed |

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

## Implementation Status

Implementation is complete through Phase 6 (2026-10-25). Commits: `1eb9e09b76`,
`e915f3858b` (anchor), `ed18a29817`, `22c25a1f40`, `30fbd1f86e` (terminal gate),
`711d82b4d2` (whole-tree full run, `changed` population), `bb30dbd92c` (push
debt and commit text), `35421aae45` (Phase 6 surfaces, the `carryForward` fast
mode fix), `a10bb89497` (the no-anchor race test's fixture, which `bb30dbd92c`'s
detached-HEAD refusal left reading a detached HEAD), `96135d911c` (Phase 6 docs).

Evidence run, which also fills the Status column of the TDD table: `go test
-count=1 -v -run '^(<every test the TDD and Wiring tables name>)$'` over
`repo/changed`, `le/human`, `verify`, `verify/evidence`, `verify/engine`,
`commit` and `hookruntime`, through `./le job run label unit-pkg`: every package
`ok`, 25 `--- PASS`, 0 `--- FAIL`. `TestRetiredKeywordsNameVerifyChanged` is
named `TestRemovedKeywordsNameReplacement` in `internal/le/commit/push_test.go`
and passes. The run used a `-overlay` holding `internal/le/setup` at HEAD,
because another session's uncommitted `internal/le/setup/shims.go` does not
compile. The whole-package runs of `commit`, `verify/...`, `hookruntime` and
`setup` were all `ok` once `a10bb89497` landed.

| AC | Status | Evidence |
|----|--------|----------|
| AC-1..AC-5 | met | `TestScopeAnchoredOnUpstreamWithNoCertificate`, `TestScopeHoldsCommittedAheadOfUpstream`, `TestScopeEmptyAtUpstream`, `TestAnchorFallsBackToOriginHead`, `TestAnchorRefusesWithNoUpstream`: PASS |
| AC-6 | met | `greenBaseline` and `errNoGreenBaseline` deleted in `1eb9e09b76`; the selector page's last row is the anchor row |
| AC-7 | met | `TestRaceGroupsAnchoredOnUpstream`: PASS |
| AC-8..AC-11 | met | `TestGateRefusesEachNonTerminalStream`, `TestGateRefusesWhenQuestionErrors`, `TestGateAdmitsTwoTerminals`, `TestVerifyWorktreeRefusesWithoutTerminal`, `TestVerifyCurrentRefusesWithoutTerminal`, `TestSweepRefusesWithoutTerminal`, `TestEveryWholeTreeVerbIsGated`, `TestEvidenceRefusesBeforeDockerWithoutTerminal`, `TestEvidenceContainerHasTerminal`: PASS |
| AC-12..AC-14 | met | `TestFullModePublishesWholeTree`, `TestChangedPopulationReadsScope`, `TestVerifyChangedWritesScopedRecord`, `TestCheckCertificateKnowsChangedMode`: PASS |
| AC-15..AC-18 | met | `TestPushAdmitsScopedRecordCoveringHead`, `TestPushCarriesProseAfterScopedRecord`, `TestPushRefusesWhenUpstreamMovedPastAnchor`, `TestPushRefusalNamesVerifyChanged`: PASS |
| AC-19 | met | `TestSessionStartDebtLineNamesVerifyChanged`: PASS |
| AC-20 | met | `TestCreateReportsDerivedDebt` (`internal/le/commit/push_test.go`): PASS, create stages only the named paths and reports the derived count without refusing; live, `./le commit create` prepared every Phase 6 commit with `debt=14 commit(s) owe a scoped verify` and refused none for it |
| AC-21 | met | `TestUnjudgedStageNamesNoAgentFullRun`, `TestRemovedKeywordsNameReplacement`: PASS; the live refusal over the carried `arch tier/check` red read `clears only when the release check reaches it, which you never run: give Thomas the command ./le verify worktree` |
| AC-22 | met | `TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify` and `TestUnbannedRunsJudgesTheSentence` (`internal/le/wholetree_corpus_test.go`): PASS. The first reads every registered `WholeTree` verb from the action tables and refuses any fenced line of a tracked file under `ai/`, `AGENTS.md`, `.claude/`, `plan/templates/`, `docs/contributing/` or `docs/functional-tests.md` that names one, and any prose sentence there that names one with no form tying the run to Thomas or banning it (round 7); it went red with the original `ai/patterns/functional-test.md` lines restored. `TestLossyPipeRefusalNamesTheScopedRun`, `TestAFastGreenKeepsAFullRedOnASharedStage`: PASS. Before round 6 the evidence was a one-off grep for an imperative full run over `ai`, `.claude/skills`, `plan/templates`, `docs/contributing` and `docs/functional-tests.md`, less the lines naming Thomas, a terminal, a ban or the release check, leaves only bans (`ai/agents/ze-work.md`, `ze-commit`) and descriptions (the `HEAD^` paragraph of `rfc-conformance-gates.md`, the `tmp/ze-verify-full.json` row of `functional-tests.md`); `./le doc check verify` PASSED |

### Goal Validation

| Goal | Evidence |
|------|----------|
| The change set is anchored on git and never widens for want of a green full verify | `TestScopeAnchoredOnUpstreamWithNoCertificate` and `TestRaceGroupsAnchoredOnUpstream` drive `./le repo changed scope` and `groups` with no certificate; `scopeFor` (`internal/le/verify/engine/scope.go`) resolves every non-full mode through `repochanged.Scope` |
| A push owes scoped evidence from one run an agent may start | `TestVerifyChangedWritesScopedRecord` runs `./le verify changed` over pipes and gets the record; `TestPushAdmitsScopedRecordCoveringHead` admits the push on it; the live `debt=` line of every Phase 6 commit named `./le verify changed` |
| The full verification refuses unless a person at a terminal starts it | `TestEveryWholeTreeVerbIsGated` over every registered verb and the three verb-level refusal tests, with no override keyword or variable (AC-11) |
| Every surface that owed an agent the full run names the scoped run or the command for Thomas | The AC-22 row above |

### Review Gate

Round 1 ran before Phase 6. Round 2, over `35421aae45`, `a10bb89497` and
`96135d911c`, ran in the parent session with an independent reviewer and found
six defects. All six are fixed in `bc6ade2c02`, and each fix's test was forced
red by breaking its producer, then restored.

| # | Finding | Fix | Test that goes red without it |
|---|---------|-----|-------------------------------|
| 1 | A partial run's red on a stage it judges only over its scope was held as its own, so a full red, then a changed red, then a changed green cleared the full red over tags the scope never judged | `carryForward` carries another population's red on such a stage whatever the partial verdict (green, red or unreached); `redVerdict` and its exemption are gone | `TestAPartialRedThenGreenKeepsAFullRedCharged` (changed and fast subtests) |
| 2 | The unreadable-certificate and unreadable-manifest messages told an agent to run a full verify | `agentRerun` in `status.go`: the next verify run replaces it, `./le verify changed` for an agent, a release check is Thomas's | `TestAnUnreadableVerifyRecordNamesTheScopedRun` |
| 3 | `clearingRuns` tested population membership, which disagreed with the carry rule on a stage judged at full identity | It reads `JudgesInPart(ChangedMode, name)`, the same answer `carryForward` reads | `TestAFullRedOnAFullIdentityStageClearsThroughTheScopedRun` |
| 4 | `noUpstreamError` read a failed or empty `symbolic-ref` answer as a detached HEAD and named the wrong fix | It asks `git rev-parse --abbrev-ref HEAD`: `HEAD` is detached, a name is a branch, and a failure or no answer is a git error naming that command | `TestNoUpstreamNamesTheFixForWhatGitReported` (four cases) |
| 5 | Stages a partial run judges at full identity (fast `go lint run`, `go patch-stamps check`, the race pieces) would have been held back by fix 1 | The rule is keyed on `Stage.Scoped`, so they take the partial verdict | `TestAFullIdentityStageTakesAPartialRunsVerdict` |
| 6 | `ze-close` and `ze-status` still told an agent to run the full gate; the race fixture matched the old `symbolic-ref` literal | Both skills name the scoped verify; `anchorBranchQuery` holds the query once | `TestRaceChangedRefusesWithNoAnchor` |

The carry rule now reads: a stage is judged in part when it sits in both a
partial population (changed or fast) and the full one, and does not
declare `Stage.Whole` (`JudgesInPart`; before round 4 such a stage had to be
marked `Stage.Scoped` instead, which failed open). On such a stage an earlier red another population judged is
carried whatever the partial run said, and only a run that judges the stage in
full replaces it. A partial red over a green, or over its own population's red,
is held as its own. A stage judged at full identity takes the partial verdict.
`docs/architecture/testing/verify-freshness-scope.md` states it.

After the fixes the scoped lint over `verify`, `repo/changed`, `commit` and
`hookruntime` found three `singleCaseSwitch` findings in
`verify/deps/racecoverage_test.go` (left by this spec's `e915f3858b`), fixed in
the spec commit. Its fourth finding, `slicesbackward` in `useEnvironment`
(`repo/changed/actions_test.go`), comes from `533c3080f3`, outside this spec.

Round 3, over `bc6ade2c02`, `1c6784bbc8` and `1476877e57`, ran in the parent
session with an independent reviewer and found one ISSUE and one NOTE, both
fixed in the commit that carries this paragraph.

| # | Finding | Fix | Test that goes red without it |
|---|---------|-----|-------------------------------|
| 7 | ISSUE: `Stage.Scoped` was a second declaration. Whether a stage judges in part is decided inside its action, by whether it reads the published scope, and nothing tied the flag to those readers, so a stage that started reading the scope unmarked would let a partial green replace a full red | The action declares it where its verb registers: `leaction.Action.ReadsChangeScope`, or the same field on a keyword (`go lint run changed`), published on `Row`. `Scoped` stays as the stage-side copy and names that source, because the engine cannot read the registry: areas register only where the composition root links them, so a lookup from the engine would answer false in its own tests. Two checks compare the copies. `TestStageScopedIsTheActionsDeclaration` resolves every stage of every population through `leroot.ActionsOf` and requires its mark to equal the declaration. `TestEveryChangeScopeReaderDeclaresIt` parses the shipped Go under `internal/le` and requires every directory that calls `repochanged.Packages`, or reads `ScopeTagsKey` or `ScopeFileKey` through a `Get`, to be an area that declares a scope-reading action, and every declaring area to hold such a read | Forced red twice: `arch enumeration check` unmarked in `stages.go` reddened the first test (full and fast); its `ReadsChangeScope` removed reddened both, the second naming the `repochanged.Packages` call in the enumeration `runCheck` |
| 8 | NOTE: nothing pinned `JudgesInPart` for a cut full-run mode name | `TestACutFullRunPieceIsAFullJudgement` names every piece of a six-way cut (`full-part-<i>-of-6`, `Part.Name`) and requires a full judgement for each stage it runs, Scoped ones included | The test itself, which also refuses a cut that holds no Scoped stage |

Round 4, over `25b5a7c4fe`, ran in the parent session with an independent
reviewer and found two ISSUEs and one NOTE. Both ISSUEs are fixed in the commit
that carries this paragraph, each test forced red by breaking its producer
(`judgesInPart` answering false, `BaseMode` answering full for an unknown mode,
the gating suite wrapped in `whole()`), then restored.

| # | Finding | Fix | Test that goes red without it |
|---|---------|-----|-------------------------------|
| 9 | ISSUE: `TestEveryChangeScopeReaderDeclaresIt` matched directories, so a second scope reader in a directory that already declared one went unseen, and an unmarked reader still let a partial green replace a full red | The default is inverted so that it fails closed (Key Design Decisions). A stage a partial population shares with the full one is judged in part unless it declares `Stage.Whole`, set only through `whole()` in `stages.go`. `Stage.Scoped`, `leaction.Action.ReadsChangeScope`, `Parameter.ReadsChangeScope`, `Row.ReadsChangeScope`, the five area declarations and both round-3 tests are removed. `TestAWholeStageReachesNoChangeScopeReader` refuses a `Whole` claim on a stage whose action's package holds a scope read, unless `reviewedReach` lists exactly the readers there, each reviewed as never called by that stage: `go lint run` beside `changedScope` (reached only with `changed`), the five whole `verify deps` verbs beside `planUnitChanged` (reached only from the `VerbUnitChanged` case), `exabgp-test` beside `selectSuites` | `TestAnUndeclaredSharedStageIsJudgedInPart`, `TestAPartialRedThenGreenKeepsAFullRedCharged` and the carry tests (break: `judgesInPart` answers false); `TestAWholeStageReachesNoChangeScopeReader` (break: gating declared whole); `TestAWholeClaimOnAScopeReaderIsRefused` covers gating, `go lint run changed` and a Staticcheck part declared Whole |
| 10 | ISSUE: `JudgesInPart` answered "full" for any mode `StagesForMode` does not know, so a piece of a cut changed run (`changed-part-1-of-2`) let its green replace a full red | `BaseMode` parses `<base>-part-<i>-of-<n>` (`Part.Name` round trip, `Part.Valid`) to its population and refuses any other mode; `JudgesInPart` and `carryForward` return the error, and `writeFailureIndex` then publishes nothing. `carryForward` compares populations by base (`samePopulation`), so a cut changed piece's red is the changed population's own, and `clearingRuns` reads a cut changed judge as changed | `TestACutPartialRunPieceIsJudgedAsItsPopulation`, `TestAnUnknownModeIsRefused` (break: `BaseMode` answers full) |
| 11 | NOTE: the shared index lists `internal/le/changescope_test.go` and `verify/engine/carry_partial_test.go` as staged deletions | Not this spec's: a stale `.git/index.lock`, for which Thomas holds a repair script. Neither the lock nor the index was touched | None |

Known bound, for round 5: the scope scan is package-granular and does not
follow imports, so a reader in a package a `Whole` stage's action calls into is
not seen. Following imports refused the claims of ten stages (`doc check`,
`rfc`, `test health` and others reach the Staticcheck and suite-map packages by
import), each of which would need a call-graph review before it could be
accepted.

Round 5, over `3a407dbb83`, ran in the parent session with an independent
reviewer and found one BLOCKER, one ISSUE and one NOTE, and the doc wiring red
that commit caused. All are fixed in the commit that carries this paragraph.
Each new test was forced red by restoring the defect it guards, then the fix
was put back.

| # | Finding | Fix | Test that goes red without it |
|---|---------|-----|-------------------------------|
| 12 | BLOCKER: the `Stage.Whole` doc said the check refuses a claim when a package the action imports reads the scope, and the test doc said the scan closes each package over its imports; `reach` keys only on the action's own directory | Both comments now state the known bound above: the scan is package-granular and does not follow imports | None: a comment. It now agrees with `reach` and `docs/architecture/testing/verify-freshness-scope.md` |
| 13 | ISSUE: `samePopulation` and the cut-judge branch of `clearingRuns` had no regression test | `TestACutPartialRedIsClearedByItsOwnPopulation` (verify/engine): a `changed-part-1-of-2` or `fast-part-1-of-2` red, then the population's green, holds the green. A `changed-part-1-of-2` row in `TestAFullRedOnASharedStageNamesNoChangedClear` (commit) | Red with `judgedBy(was, prior) != mode` in `carryForward` (the red stayed carried), and with `judgedBy == ChangedMode` in `clearingRuns` (the text named the release check) |
| 14 | NOTE: an unknown mode reached `unpublished`, whose fix says to rerun the verify, which cannot fix a code defect | `carryForward`'s refusal goes through `unjudgeable`, which names the defect in le verify, says a rerun meets the same refusal, and names the fix in code (`StagesForMode`, `Part.Name`, `stages.go`) | `TestAnUnknownModeIsRefused` asserts that wording and that the text never says "rerun the verify" |
| 15 | Doc wiring: `3a407dbb83` changed `fullStages` under claims in `feature-gates.md` and `gh-pages.md` | The population and its order are unchanged, so both claims hold: two reviewed rows in `plan/doc-reviewed/e827fce7.md` | `./le doc wiring` no longer names `fullStages` |

`./le doc wiring` still reports two stale reviewed rows, `plan/doc-reviewed/6657e819.md`
(`605ac0a373`) and `plan/doc-reviewed/a4042a10.md` (`f074233e43`), and two dangling
references in `spec-prose-cites-symbols-not-facts.md`. None is this spec's.

Round 6, over the round-5 fix commit, ran in the parent session with an
independent reviewer and found one ISSUE and two NOTEs, all fixed in the commit
that carries this paragraph.

| # | Finding | Fix | Test that goes red without it |
|---|---------|-----|-------------------------------|
| 16 | ISSUE: AC-22 did not hold end to end and rested on a one-off grep. `ai/patterns/functional-test.md` still told an agent to run `./le verify current fast` and `./le verify current` | The pattern names `./le verify changed`, and the full run only as the command an agent gives Thomas. `TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify` (`internal/le/wholetree_corpus_test.go`) derives the verb list from the registered action tables (every `WholeTree` action as `<area> <verb>`, and an area whose every action is `WholeTree` as `<area>`, so `verify evidence`) and judges every sentence of the tracked `ai/`, `AGENTS.md` and `.claude/` files that names one after `le`; a code-block line and a table row are each judged alone. A sentence passes only when it names Thomas or bans the run. It found seventeen more hits, each fixed: `.claude/hooks/README.md`, three `.claude/plan` files, `ai/INDEX.md` (two), `ai/META-DECISIONS.md` (one sentence, both verbs), `ai/rationale/git-safety.md`, `ai/skills/ze-test.md`, and three rule points, re-rendered into `completion.md`, `platform-linux.md` and `writing.md`. `docs/architecture/overview.md` and `docs/guide/developer-setup.md` no longer call `./le verify current` the pre-commit check: each names `./le verify changed` as day-to-day evidence and the full run as the terminal-gated release check | Red with the original two `functional-test.md` lines restored; `TestUnbannedRunsJudgesTheSentence` holds the scanner to its seven cases |
| 17 | NOTE: the doc comment above `readScoped` (`internal/le/commit/owed.go`) said `readGreen` | Renamed | None: a comment |
| 18 | NOTE: the AC-20 closure row named no test | It names `TestCreateReportsDerivedDebt` (`internal/le/commit/push_test.go`) | None: the record |

Round 7, over the round-6 fix commit, ran in the parent session with an
independent reviewer and found three ISSUEs, all fixed in the commit that
carries this paragraph.

| # | Finding | Fix | Test that goes red without it |
|---|---------|-----|-------------------------------|
| 19 | ISSUE: the scanner failed open. Any sentence holding "Thomas" or a ban word passed (`Thomas says run ./le verify worktree before you commit.`, `Run ./le verify current now; do not skip it.`), and a fenced command passed on its own comment (`./le verify current   # never skip this`) | A fenced line naming a whole-tree verb is always refused (`textUnit.code`). A prose sentence or table row passes only on a form in `ownedOrBanned`: one tying the run to Thomas (`give` or `hand Thomas`, `Thomas runs` or `starts`, `Thomas's release check`) or one banning it (`never run` or `start`, `an agent never runs`, `must not` or `do not run`, `run no`, `refuses`). `run no` stays so "you run no command that can take longer: no `./le verify worktree`" is still a ban. The tightened rule flagged eight lines of the old corpus (`ai/META-DECISIONS.md`, `ai/rationale/git-safety.md`, the flake rule point and its render in `ai/rules/testing.md`, `ai/skills/ze-verify.md`, `.claude/plan/ze-plan-repo-audit-2026-07-16.md`), each reworded | The three lines are cases of `TestUnbannedRunsJudgesTheSentence`, beside a fourth (`an agent never runs`) that must pass. Red with the round-6 producer restored (a bare `thomas`, `never` or `do not` match, fenced lines exempt): all three new cases reported no refusal |
| 20 | ISSUE: `agentCorpus` left out `plan/templates` and `docs/contributing` while its comment claimed every path an agent reads | The corpus adds `plan/templates`, `docs/contributing` and `docs/functional-tests.md`, and the comment names exactly the trees judged and says the rest of `docs/` and the specs under `plan/` are not. The widened scan flagged 55 more lines in `docs/contributing/` (`documentation-testing.md`, `rfc-conformance-gates.md`, `rfc-implementation-guide.md`, `running-commands.md`, `testing.md`, `ze-go-style.md`, `ze-go-style-background.md`) and `docs/functional-tests.md`, each reworded so the run is Thomas's release check; four fences lost their whole-tree command to a prose sentence. `running-commands.md` no longer says the gate a commit passes is `./le verify worktree`: `./le commit push` decides it from the recorded runs. `docs/architecture/overview.md` stays outside the corpus, and its line now calls the full run Thomas's release check | `TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify`: red with the widened corpus before the rewording (68 hits on 63 lines over findings 19 and 20), green after |
| 21 | ISSUE: `docs/guide/developer-setup.md` told a fresh clone to verify the setup with `./le verify changed`, which judges an empty change set there | The page names a scoped package test (`./le job run label unit-pkg command go test ./internal/core/textbuf/`) as the smoke check, and says `./le verify changed` judges only a change set, so on a fresh clone it proves nothing | None: prose. `./le doc check verify` PASSED |

Rounds 6 and 7 ran past the five-round review cap without Thomas's
authorisation. Whether a further review round runs, and whether this spec
closes, waits on his ruling. The scoped `./le verify changed` over the closing
commits is owed with that ruling.

## Design Insights

- The certificate (`tmp/ze-verify.status`) is overwritten by every run, red or green, so anchoring scope on it made one interrupted run (exit 130) widen every later scoped run until someone ran a full verify. The anchor belongs to git, which no run can overwrite.
- Phase 2 found a second anchor already in the package: `upstreamBase` in `internal/le/repo/changed/lines.go`, the changed-line reader of `arch compound-guard`, `arch refuse-by-default` and the doc gates, which fell back to `refs/remotes/origin/main`. Two answers to one question drift, so it now reads `ResolveAnchor`, and its fallback is `refs/remotes/origin/HEAD` like the selector's. Its tests gained the `origin/HEAD` symbolic ref a clone leaves; no assertion changed.
- Whether the machine is quiet enough for a full run is a judgement about the next hour, which no check at start can make. Who started the run is a fact the process can read at start: a person at a terminal, or a tool call through pipes.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Anchor = `merge-base(HEAD, @{u})`, else `origin/HEAD`, else refuse | Anchor = last green full verify (today); anchor = HEAD only (working tree); anchor = a scoped-run record | The upstream is what a push compares with and what no run can overwrite. HEAD-only drops committed-unpushed work from scope. A scoped-run anchor reintroduces the certificate loop. Forecloses: re-judging already-pushed commits day to day (A-1) |
| Push owes a recorded scoped run at the pushed commit (`./le verify changed`) | Push owes nothing but person acts, scoped tests left to rule discipline | O-1 says a push owes scoped evidence; a rule nobody checks is the unenforced state the push gate exists to end. Decision Q-1 |
| Scoped push run in a throwaway worktree | In place over the working tree | The shared checkout carries other sessions' edits; the push publishes a commit, so the evidence must judge that commit's tree |
| Full run publishes `./...` and every feature tag | Keep narrowing from the change set | A release check that narrows from an anchor at the upstream judges almost nothing (D-4) |
| Whole-tree entries refuse unless stdin and stdout are terminals, with no override | An idle check at admission; an override keyword; a hook that blocks the command text | O-3: an idle check answers nothing about the run that follows. An override keyword is a bypass any agent can type. A pretool hook sees only the command a Claude tool call types, so a script that calls the verb evades it and another agent harness never meets it; the process's own streams are what the tool call cannot supply. Decisions Q-2, Q-3/Q-4 |
| The whole-tree property is declared on each registered action | A list of gated verbs in the gate package | Registration over hardcoding: a new whole-tree verb is gated where it registers, and `TestEveryWholeTreeVerbIsGated` finds one that is not |
| The declaration is the field `leaction.Action.WholeTree`, and `Area` runs the gate (`Area.admit`, which calls `human.RequireTerminal` with `./le <area> <words>`) before the body, on its own line and in a sweep; `Row.WholeTree` publishes it, and `leaction.SetWholeTreeAdmission` is the Go-only test seam (R-4). Landed in 22c25a1f40 | A wrapper over each handler; the gate called first inside `runHere`, `currentHere` and the evidence handler | One check in the one dispatcher reaches every verb that declares the field, so no handler can forget it; the listing carries the property, so `TestEveryWholeTreeVerbIsGated` reads `Actions()` instead of a list. The gate runs after the grammar parse, so a mistyped keyword still gets its usage refusal |
| A shared stage is judged in part by default; only a full judgement is declared, as `Stage.Whole` through `whole()` in `stages.go`, and `TestAWholeStageReachesNoChangeScopeReader` refuses it for a stage whose action's package reads the scope unless `reviewedReach` lists exactly those readers as reviewed | Declare every scope reader (`ReadsChangeScope` on the action and a checked `Stage.Scoped` copy, round 3); detect every reader precisely by call graph | A reader nobody declared failed open: its partial green replaced a full red. Under the inverted default the same omission fails closed, so the check guards only the one claim that can do harm, and a package-granular scan is sound for that because it can only refuse a claim. Pinning each reviewed entry to the readers the scan finds makes a new reader in the same package refuse the claim until it is reviewed. The round-3 machinery is removed rather than kept beside it (`ai/rules/no-layering.md`, `ai/rules/simplicity.md`; review round 4, finding 9) |
| The evidence container gets `docker run -t` | A keyword the host passes to the inner run; running the inner stages without the whole-tree verb | The host side already proved a person started it; a pseudo-terminal carries that fact into the container without an override anyone else could pass. The inner run stays the same `./le verify current` Thomas would run |

## Known Limitations
- A commit already on the upstream is not re-judged day to day; the release check is the backstop (A-1, by O-2).
- The gate stops a tool call, not a determined wrapper: a program that fakes a terminal passes it (R-2). The rule that an agent never runs a full verification (O-2) covers that case.
- `./le test unit all` and an unscoped `./le go lint run` stay ungated (decision Q-6).

## Decisions

The owner's answers, folded into the body above. They also stand as rules in `ai/META-DECISIONS.md`, "Ze is pre-release".

| ID | Question | Answer | Where it lands in this spec |
|----|----------|--------|-----------------------------|
| Q-1 | What does a push owe? | Only the scoped tests of the components its commits reach, as a green `./le verify changed` record that the push gate checks. Never a full verify. | D-2, Transformation Path 4-5, AC-13..AC-19, Phase 5 |
| Q-2 | Can the full-run refusal be overridden? | No. An agent never runs a full verification. Every full-verify entry point (`verify current`, `current fast`, `worktree`, `part i of m`, `all of m`, and the host side of `verify evidence`) refuses unless stdin and stdout are an interactive terminal. There is no override keyword. Every surface that tells an agent to run `verify worktree` tells it to give Thomas the exact command instead. | D-3, D-5, Transformation Path 6-7, AC-8..AC-11, AC-21, AC-22, Phases 3, 5, 6 |
| Q-3/Q-4 | Which processes make the machine busy, and what happens to a session that starts after admission? | Neither question stands: there is no idle check at all. Several sessions run nearly all the time, Thomas starts new ones, agents start agents and he tests by hand, so a check at admission answers nothing about the run that follows. The terminal gate of Q-2 is the only control. | O-3; no idle gate, `procscan` change, live-session, registry-holder or cache-reader refusal anywhere in this spec |
| Q-5 | Does `./le verify current fast` count as a full verification? | Yes: it is covered by the terminal gate. | AC-8, `TestVerifyCurrentRefusesWithoutTerminal`, the `ai/INSTRUCTIONS.md` and hook line 801 rewrites |
| Q-6 | Do `./le test unit all` and an unscoped `./le go lint run` get the same gate? | No. They stay as they are and are out of this spec's scope; Thomas will raise it if he sees a problem. | Behavior to preserve, Known Limitations |

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
- [ ] AC-1..AC-22 all demonstrated
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
