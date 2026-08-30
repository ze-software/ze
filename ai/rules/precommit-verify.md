# Pre-Commit Verification

**When:** before running precommit-verify, judging its red in a shared checkout, or running the tracked-build check after a commit script
**Severity:** blocking
**Related:** git-safety, commands, completion

## Does `./le verify current mode full` Apply?

**A commit whose files fall on a YES row MUST have the gate run over it; a
commit whose files all fall on the NO row MUST NOT pay for one.**

| Files in commit | Run `./le verify current mode full`? |
|-----------------|------------------|
| Any `.go`, `go.mod`, `go.sum`, `vendor/**` | YES |
| `internal/le/**`, `internal/test/**`, `internal/appliance/**`, build/CI config | YES |
| `*.yang`, generated code, codegen templates | YES |
| Anything that runs at build time or affects a binary | YES |
| `ai/**/*.md`, `.claude/**/*.md`, `plan/**/*.md`, `docs/**/*.md`, `README.md` | NO |

**A mixed commit with one YES row MUST be verified, and it MUST NOT be split to
escape that.** The question to answer is "could this make a Go test fail or break
the build?". A no skips the gate and says so in the commit summary. Anything
short of certainty MUST be treated as a yes.

## Running The Gate

**Freshness MUST be checked before any verify target, and a FRESH answer MUST NOT
be answered with another run.** `./le verify status check` with no arguments asks
about the whole tree; `check <PATH>...` asks about the named paths alone. Treat
`FRESH(full)` and `FRESH(changed)` alike for commit preparation. A full run on a
`FRESH(changed)` tree is permitted when the work explicitly needs the full gate.
**A per-stage log MUST be reached through the `detail-log` field of the failure
index, never by constructing a path**: each run gets its own directory under
`tmp/verify/`. What each mode covers and what each run writes is
`docs/architecture/testing/verify-freshness-scope.md` and
`docs/contributing/running-commands.md`.

- **Step 0. Run `./le verify status check`. A FRESH answer MUST NOT be answered with another run; note its timestamp. A STALE answer continues only when the table above says verification applies.**
- **Step 1. Run `./le verify worktree` in the foreground, with the largest timeout your harness allows, and never kill it early. On failure you MUST read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open the stage log its `detail-log` field names in `tmp/ze-verify-failures.json`.**
- **Step 2. A failure from the current work, or any failure blocking this commit's goal, MUST be fixed and re-run. Any other failure gets its spec, this commit finished, and the question to Thomas whether that spec runs (`ai/rules/completion.md`). A deterministic structural gate is NEVER on this branch and MUST be fixed before any commit. A `plan/known-failures/` shard is only for a failure you tried and could not reproduce, and it carries the reproduction attempt and the next step.**

**MUST run `./le verify worktree` in the foreground, wait for it, and never poll: the foreground return IS the completion signal.**
**MUST NOT kill it for being slow, and MUST give the call the largest timeout the harness allows.** A verify that is still running is not a verify that is hung, and killing one costs the whole pass rather than the seconds it saves.
**When the harness caps a foreground call BELOW a full pass, the run MAY be started detached, and only where the harness raises a completion event of its own.** A truncated run rewrites no verify record, so a commit gate reading that record could never be cleared from inside such a harness. Where the harness itself says when the job ended, that event IS the completion signal.
**Waiting on that event is not polling: MUST NOT sleep-and-check, MUST NOT `tail` a log that is still growing, and MUST NOT start a second run to find out where the first one got to.** A second run contends for the same job slot, and a live run is never the one displaced.
**A harness that raises no completion event has no detached route: MUST say the cap is in the way and hand the run to the operator.** Claiming a pass from a run that was killed at a ceiling is worse than not running it.
**MUST NOT edit the tree while the run is detached.** Detached says where the completion signal comes from. It does not license other work in the tree meanwhile, because the run reads the working tree either way.
**MUST NOT take a timeout from a duration written in any rule: read `tmp/.ze-verify-duration.txt` instead.** Two rules quoting different figures are different hardware, not a contradiction, and a slow run is never broken for being slow. What writes that file, and the stall window that actually displaces a holder, are `docs/contributing/running-commands.md`.

**`./le verify worktree` is the full gate, and you MUST run it ONE time, when the
work is finished and you are about to prepare the commit script.** What it costs
on this machine is `tmp/.ze-verify-duration.txt`, never a figure written here.
Running it to "check in" mid-change is the single most expensive habit available in
this repository, and it buys nothing a scoped check does not.

**You MUST run what the change touches.** Every surface has one owning native action,
and it costs seconds to minutes rather than half an hour. Find yours in this table, run it after
each edit, and keep `./le verify current mode full` for the end.

**You MUST use the owning native action, or carry its Go build environment yourself.**
`internal/le/gotoolchain` derives the pinned toolchain, repository build cache,
module cache, feature tags, timeout, and process ceiling used by `./le test-unit`
and verification. A bare `go test` uses the user's defaults and can compile a
different feature population. If no native action owns the focused run, pass
the required tags, `GOCACHE`, `GOMODCACHE`, `CGO_ENABLED`, and race setting
explicitly. Race-built test executables never ship or serve as release evidence
(`ai/rules/commands.md`, "Bare `go test` Lies").

**The action on the matching row MUST be run for the surface the change touched,
after each edit.**

| You changed | Run this |
|-------------|----------|
| A `.go` file | `./le job run label unit-pkg command go test <that package>`, or the component group covering it (`./le test-unit bgp`, `core`, `plugins`, `config`, or `cli`). Then run `./le verify lint run` (`ai/rules/commands.md`) |
| A `.go` change that alters what the daemon PUTS ON THE WIRE, installs, or shows | The owning functional action as well: `./le functional plugin`, `encode`, `decode`, `parse`, `reload`, `ui`, or `web`. Unit tests of the package are not evidence about the rail |
| Reactor concurrency (`reactor/session*.go`, `forward_pool*.go`, `peer.go`) | `./le job run label reactor-race command go test -race -count=20 ./internal/component/bgp/reactor/...` (`ai/rules/testing.md`) |
| A `.ci` or `.et` test | Its native suite action: `./le functional plugin`, `parse`, `encode`, `editor`, or `web`. Draft first in `test/draft/` |
| Linux-only code (`//go:build linux`) | `./le qemu all-tests`, or `./le qemu netns-test suites <names>` for focused kernel-dependent `.ci` suites (`ai/rules/platform-linux.md`) |
| `rfc/short/*.md`, an `RFC requirement:` tag, `rfc/extraction/*` | `./le rfc check` |
| `docs/**`, `ai/**`, `plan/**` | `./le doc check verify`, and `./le doc wiring` for the changed-file gates |
| `ai/rules/points/**` | `./le rules render-update`, `./le rules condensed-update`, then `./le rules lint` |
| A `*.yang` file or a `ze:command` | `./le doc check verify`, `./le cli-grammar` |
| A plugin `register.go` or generated composition root | `./le plugin imports write`, then `./le plugin imports check` |
| A new package's placement | `./le tier check` |
| Anything, once the commit script has run and it carried Go | `./le repository tracked-build check`, the only check that compiles what git holds |
| A native tool package under `internal/le/` | `./le test-unit`; the permanent package tests compile and call the Go implementation directly |
| Several of the above, and you want breadth | `./le verify worktree` |

**A change to what the daemon PUTS ON THE WIRE, installs, or shows MUST run the
functional suite that owns that surface before commit. The package's unit tests
are not that evidence.** A unit test proves the function answers correctly. Only a
running daemon proves the rail carries the answer to a peer.

**The fixture that catches the regression is named after ANOTHER feature.** A rail
every feature crosses is observed by every fixture that crosses it, so the file
that goes red carries a name with no connection to what you changed. Searching the
suite for a fixture named after your change finds nothing, and finding nothing is
not evidence. Run the whole suite that owns the surface.

**A guard is the case this bites hardest.** It changes the answer for every caller
of the rail at once, including the callers whose fixtures were written before it
existed and assert the answer it now refuses.

**A surface is usually observed by MORE THAN ONE suite, so "the" suite is the
wrong question.** Naming the suite you already know about is a guess, and it
answers only for the fixtures that suite happens to hold. DERIVE the set instead:
search the whole fixture corpus for what exercises the rail, by the configuration
that reaches it or by the bytes it emits, and run every suite that turns up. A
guard on an egress rail is reached by the announce fixtures, the relay fixtures
and the encoding fixtures alike, and they do not live together.

**A gate's population is defined by where a file LIVES, not by what you edited, so
before commit you MUST run the repo-wide counters, inventories and ratchets whose
population your NEW files join.** Adding a file to a directory such a gate walks
puts you inside it, even when the gate was written for a concern your change has
nothing to do with, and even when every gate for the surface you edited is green.

**Scoped evidence is keyed on the surface; the gate that catches you is keyed on
the directory.** That asymmetry is why a careful, fully verified commit can still
turn a shared gate red for every other session: the author ran what their change
was about, and the gate counts what their change added.

**Ask it as a question about paths.** For each path in the commit that did not
exist before, name the repo-wide checks that walk its directory, and run them. A
new test fixture, a new scenario, a new script and a new generated artifact are
the usual carriers, because each lands in a tree something else counts.

**When the table has no row for what you touched, you MUST derive it.** The bare
`./le <area>` command lists every native action in that area and what it runs.
`ai/rules/repo-maintenance.md` maps each gate to the rule it enforces. When a
surface has no owning action, you SHOULD say so in the report rather than reach for the full gate.

**A commit MUST NOT carry a lint issue its own diff caused**: lint costs seconds
and says the tree is broken. **The test evidence a commit owes is the focused
test for what it changed, run once.** A test that stays red is named in the
commit body, and it does not hold the commit (`ai/rules/pre-release.md`).

- **Step 3. A commit driven by a `plan/` spec MUST carry a journal row appended to `plan/journal/<class>.md` whose Spec cell names the spec stem, and the spec file staged for deletion. Neither done, STOP.**
- **Step 4. An Executive Summary Report MUST be written (`ai/rules/planning.md`): what was done, and what is left.**

## Reading A Red

**YOU MUST READ THE WHOLE FAILURE SUMMARY BEFORE YOU RE-RUN.** A verify run ends with
`FAIL N verify stage(s) failed` and one line per failing stage, and
`tmp/ze-verify-failures.log` holds the same list. A re-run started from a partial
read costs another full pass and usually reports the same stages. Two traps
follow, and each has cost one.

- **`tail` on the log of a run that is still going.** The stage banner tells you
  where it is (`### Stage 18/22`). You MUST check `./le verify status check` for
  the verdict instead: it says FRESH, or names the failure and its time.
- **Grepping for `--- FAIL` only.** Lint, tier, doc and inventory stages fail with
  their own wording and no `FAIL` token, so a test-shaped grep reads a red lint
  stage as a clean run. You MUST read the summary block, not a pattern you chose.

**A NARROW FAILURE MUST GET A NARROW RE-RUN; IT MUST NOT GET A SECOND FULL PASS.** When the
end-of-development run comes back with one or two failing stages and you fix
exactly those, you MUST re-run the GATE THAT FAILED and the tests of the package you
touched. Twenty-three green stages that nothing has touched since do not become
more green by being run again.

**The status record is what forces the second pass, so plan the FIRST one to be
the last.** `./le commit create` refuses unless
`./le verify status check` reports FRESH, and only a full `./le verify current mode full`
writes that record. A narrow fix therefore still needs one more full run before a
commit, which is precisely why the full run MUST come AFTER every gate you can
check cheaply is already green: `./le verify lint run`, the touched packages' `go test`,
and the gate owning each surface you changed. A run started before those are clean
is a run you will pay for twice.

**You MUST NOT stop to ask which way.** The operator is often not present, and a session that halts on this question has spent their time to save its own.
**Clear the cheap structural gates your own diff caused, then commit.** The full pass is owed before a push, not before this commit (`ai/rules/pre-release.md`).

**Several agents work this checkout at once. `./le verify worktree` reads the WORKING
TREE, so it reads their half-finished edits too, and a fully GREEN run is
unreachable by construction. You MUST NOT wait for one and you MUST NOT re-run for one: what is unreachable is the green bar, never the run.**

**The 2026-08-17 directive requiring a full `./le verify worktree` before every Go commit is SUPERSEDED and MUST NOT be followed.** Two later owner directives replace it: the gate gates PUSHING rather than committing (2026-08-21, `ai/rules/git-safety.md`), and ze is pre-release so a commit owes no green (2026-08-30, `ai/rules/pre-release.md`).
**A Go commit owes the FOCUSED test for what it changed, run once, and nothing more.** A missing full run records verification debt, which `./le commit create` writes and a push refuses to carry. You MUST NOT re-run the gate to watch somebody else's red clear, and you MUST NOT hold finished work waiting for one.

**One full run covers EVERY commit prepared from it. You MUST NOT re-run the gate between back-to-back commits of the same code.** The debt is incurred by an EDIT, never by a commit: one body of work split into three commits owes one run, not three, and the same run answers for all of them. What owes a fresh run is a Go file written again after that run started, and nothing else does.

**Every red a full run reports MUST be placed on one of these rows before it is
acted on.**

| The failing path | Whose red | What you do |
|------------------|-----------|-------------|
| In this commit's `--file` list | Yours | Fix it. A red you caused is never attributed away |
| Dirty in `git status --porcelain`, and not in your list | Another session's | Take that code as working. Name it in `--unverified` and commit |
| Clean and tracked, and your diff PRODUCES a symbol the failure names | Yours | Fix it. Ownership follows the producer, not the file that failed |
| Clean and tracked, and unrelated to your diff | Pre-existing | Attribute it against `git log`, name it in `--unverified`, and commit |
| Any deterministic structural gate | Yours until you prove otherwise | Fix it. Those read files rather than a moving tree. The helper drops the charge only when every file the failure groups name lies outside your commit |

**A structural gate red is charged to your commit unless EVERY file its failure groups name lies outside your `--file` list. You MUST NOT expect attribution to drop a red whose groups name no file at all.** `structuralGateReds` (`internal/le/commit/verification.go`) reports three sets, and the refusal prints which group carried the charge. Attribution reaches the gates whose groups name a file or a package directory; `./le doc wiring` is one, through `declareFailureGroup` (`internal/le/doc/wiring/groups.go`), except for its ci-sleep ratchet and its delegated targets, which judge a population rather than a file. Which gates that leaves charging every time is `docs/architecture/testing/verify-freshness-scope.md`.

**Owner directive, 2026-08-17: code another session holds uncommitted MUST be taken as WORKING. You MUST NOT fix its red, wait for it, or re-run the gate to see whether it cleared.** Attribution is the whole answer: name the file and say whose it is, put that in `--unverified`, and commit. The row that MUST NOT be attributed away is a red your own diff produced, and the table above is what decides which row you are on.

**A verify verdict answers about the paths it was ASKED about, and `./le commit create` asks about the commit's own `--file` list. You MUST NOT read a FRESH as a verdict on the whole checkout.** `verificationState` (`internal/le/commit/verification.go`) passes that list to `./le verify status check <PATH>...`, and `CheckCertificate` (`internal/le/verify/engine/status.go`) compares only the named rows. An edit another session makes to a path your commit does not carry no longer makes your evidence STALE.
**A run that FAILED is STALE for every path list, so scoping MUST NOT be used as a route around a red run, and it is no route around a red structural gate either.** That is `structuralGateReds` (`internal/le/commit/verification.go`), which still reads every red the run recorded.
**`check` with no path arguments keeps its whole-tree meaning, and you MUST use that form when the question is about the tree rather than about one commit.** The other limits of the narrower question are `docs/architecture/testing/verify-freshness-scope.md`.

**A commit that carries NO Go owes no full run. You MUST scope its evidence to YOUR
files, running the narrow gate that owns each surface it changes.** That evidence
is gathered before the commit script is prepared, on either route.

1. You MUST run the gate the commit owes: `./le verify worktree` when it carries Go,
   and the narrow gate owning each changed surface (the table below) when it does not.
2. You MUST ATTRIBUTE every red you saw, by the table above: name the file, and say
   whose it is. `git status --porcelain` plus a modification time settles it in seconds.
3. You MUST prepare the script and let the helper judge your own paths first: `create` scopes
   the freshness question to your `--file` list, so an edit outside it changes nothing.
   Since 2026-08-21 a stale record does NOT refuse the commit: it records a
   verification-debt row and proceeds, and `--push` is what refuses while a row is
   open (`ai/rules/git-safety.md`, "Verify a Commit, Not the Working Tree"). You
   MUST still pass `--unverified "<attribution>"`, because that reason is the Reason
   cell of the row: give the gates you ran and their verdicts, and name the paths
   whose reds you attributed away. A row with no attribution leaves the next reader
   with a debt nobody can judge. The one red that still REFUSES the commit is a
   structural gate charged to it, and `--unverified` never cleared that.

**`--unverified` is the CORRECT path in a shared checkout, not a shortcut.** It
exists for exactly this: a full-tree gate whose red belongs to somebody else's
in-flight work. Its own text names the owner override and a failure you tried and could not reproduce;
concurrent-session interference is the third case, and the reason MUST say so.

**Since 2026-08-21 it unlocks nothing, and that is what makes it worth writing.**
A stale verify records a verification-debt row whether or not the flag is given,
and `--push` refuses while that row is open. The flag fills the row's Reason
cell. The checker can say the record is STALE; only a caller can say whose red it
is and which run will cover this commit, and every judgement in this rule is
built on that attribution.

**A deterministic STRUCTURAL gate MUST NOT be waved through** (see "Structural
Gates Are Never Known-Red"). Those read files, not a moving tree, so they are
reproducible and yours to fix when your diff caused them: `./le verify lint run`,
`./le rules lint`, `./le doc check verify`, `./le rfc check`, `./le doc wiring`,
`./le tier check`. You MUST always green those. It is the TEST stages -- unit, functional,
web -- whose reds a concurrent tree can manufacture.

**When `./le verify worktree` is known-red from failures this session did not
cause, a commit carrying NO Go MUST be gated on changed scope alone.** Re-running
the full gate for it re-surfaces other sessions' noise that is not your
regression. **A commit carrying Go still owes the run named above: a known red is
ATTRIBUTED there and is never a reason to skip it.**

You MUST run these scoped gates instead:

- `./le verify lint run`
- the touched packages' `go test` (or `./le verify worktree`)
- `./le doc check verify` / `./le repository check` when those surfaces changed
- a QEMU run for any linux-only runtime code touched

**The commit script MUST list ONLY this session's files, as repeated `file
<path>` pairs, so the commit never pulls in another session's working-tree
edits**, and other sessions' files MUST be excluded when reviewing `git diff`.
**This route MUST NOT be taken for a red your own change caused, which is fixed
rather than scoped around, and it MUST NOT be inferred from a red suite alone.**
It needs an explicit owner direction naming the other session's clearing run.

**A red you have not looked at MUST NOT be assumed to be somebody else's.** Read the failing assertion once, and decide whether your own diff produces the symbol it names.
**A change that could break a package you did not edit MUST have its reverse-dependency closure compiled once.** Scope-to-changed tests the packages you edited, not the packages your edit breaks TRANSITIVELY: a new import broke `bgp/config` through `plugin/all`, a missing YANG typedef failed a consumer rather than the plugin that introduced it, and adding a plugin invalidates the `plugin/all` golden snapshots. `go vet` over that closure answers it in seconds, and the full gate is not the way to ask.

**A red that says the PRODUCT is wrong MUST NOT be left to persist across sessions.** A standing product red hides newly-introduced breakage underneath it: an import cycle, a YANG typedef gap and stale registry snapshots each landed under one persistent red with no gate firing.
**A red that says the SCAFFOLDING is wrong MAY persist indefinitely, and clearing it MUST NOT be made a precondition of further work** (`ai/rules/pre-release.md`). Name it once where the next reader will meet it. Ze has no release and no user, so an unclear instrument costs only the session that stops for it.

## What May Be Overridden

**The checklist's "write its spec, finish this commit, ask" branch and its
`plan/known-failures/` shard are for NON-DETERMINISTIC failures only**: flaky or
environmental TEST reds such as load-sensitive races, GC-pressure pool flakes,
and host-specific listener probes. **A deterministic structural gate is NEVER
eligible, MUST be fixed at the source before any commit, and MUST NOT be parked
or passed with `--unverified`.** Those gates (`./le verify lint run`, `./le
changed scope`, `./le tier check`, `./le verify deps evidence-vet`, `./le plugin
boundary check`, `./le iface-resolution`, `./le repository generated-check`,
`./le doc wiring`, `./le repository tracked-build check`) fail only when the tree
is structurally broken. Which stages carry that status is DERIVED rather than
listed: `docs/architecture/testing/verify-freshness-scope.md`.

**The general escape is owner-only: `--structural-red-ok "<reason>"`** (the
narrow `--broken-head-fix` above is the only other, and it reaches one gate).
It is a
SEPARATE flag from `--unverified` precisely so the flaky-test path can never
reach this branch, it refuses an empty reason, and it prints the red gate names
with the reason to stderr so a red tree can never look green in a transcript.
You MUST use it only when Thomas says so and the red provably belongs to another
session's in-flight work that this commit cannot affect. An override that is
written down and shouted is safer than one that is improvised, and it is never a
substitute for fixing your own red (`ai/rules/completion.md`).

**A `./le repository generated-check` red MUST NOT be treated as flaky or
environmental.** A stale generated file is deterministic and reproducible, and it
is fixed by `./le repository generate`, or by the specific `--fix` the failing
check names.

**Thomas owns the repository and MAY explicitly override the
`./le verify current mode full` requirement for commit-script preparation.** The
requirement binds the agent, never the owner.

Thomas MAY use the override to do two things:

1. prepare a commit script, and
2. skip tests, skip verify, or commit without running tests.

**The override MUST come from a direct instruction by Thomas in the active
conversation, and it MUST NOT be inferred from urgency.** `owner override: commit
without verify`, `commit no test` and `commit without running tests` activate it;
an equivalent direct instruction does too.

- You MUST NOT run `./le verify worktree`, `./le verify worktree`, lint, or tests as a
  late commit gate.
- You MUST inspect only enough state to stage exactly the requested files and avoid
  ignored, generated, unrelated, or user-owned paths.
- You MUST use `./le commit create` with the normal user-run script
  path. The override changes verification requirements only.
- You MUST carry the override into the helper: `--unverified "<reason>"`, and
  `--missing-full-verify-ok "<reason>"` as well when the commit carries Go. Since
  2026-08-21 neither flag unlocks the commit, which proceeds either way and records
  a verification-debt row: they name the row's REASON, and the owner's authority is
  what that reason states. `--push` refuses while a row is open.
- You MUST NOT run `git add`, `git commit`, `git rm`, `git stash`, or prohibited git
  commands from an AI tool.
- You MUST NOT add `--no-verify`, `--no-gpg-sign`, disabled hooks, or any bypass to
  the generated script.
- You MUST report `Verification skipped by Thomas owner override` in the final response
  and, when useful, in the commit body.
- You MUST NOT claim tests, lint, `./le verify current mode full`, integrations, or behavior were
  verified if they were skipped.

## After The Commit

**Nothing else in this repository COMPILES what git holds.** `go build`,
`./le verify current mode full`, `./le changed scope`, `./le rfc check`, and every native test action build and
run your WORKING TREE, uncommitted and untracked files included. (One gate does
read the commit: `internal/le/commit` judges discovery-index freshness against a
materialized HEAD. It regenerates indexes; it compiles nothing.) So you MUST NOT
commit a CONSUMER while its PRODUCER stays uncommitted: it is green for you and
broken for everybody who builds what git holds. This is a structural blind
spot.

**A consumer MUST NOT be committed without the file that DEFINES every symbol it
newly uses, and a commit script that carried Go MUST be followed by
`./le repository tracked-build check`.**

| Situation | Do |
|-----------|-----|
| You are about to `--file` a consumer | Name the file that DEFINES every symbol it newly uses, and check that file is in the same `--file` list or already committed (`git log -1 -- <path>`) |
| The commit script has just run and it carried Go | Run `./le repository tracked-build check`. About 45s. This is step 7 of the commit workflow, not an optional extra |
| It goes red | Commit the producer. Never revert the consumer, and never park it: HEAD is broken for everyone until you do |

**`./le repository tracked-build check` is the one entry whose red is cleared BY
a commit rather than before one, so a broken HEAD MUST be fixed by committing the
producer a previous commit left behind.** Every other gate on the list is fixed in
the working tree first.
**`--broken-head-fix "<reason>"` is that commit's route through, and it MUST NOT
be reached for while any other structural gate is red**: `internal/le/commit`
accepts it only when tracked-build is the ONLY structural red.
**After the script runs, `./le repository tracked-build check` MUST be run again
and confirmed green.** If it is not, HEAD is still broken for everybody who builds
it. The gate's design is `docs/architecture/testing/tracked-build-gate.md`.

**What it does NOT read: test files.** `go build` MUST NOT compile `_test.go`, so a
test file committed without its fixture producer stays invisible here.

**Verification debt MUST be cleared by RUNNING the gate: `./le commit debt-clear`. You MUST NOT edit a row to `cleared`.** Every override on `./le commit create` writes a row into `plan/verification-debt/<session>.md`, and `create push <remote>` refuses while one is open (`ai/rules/completion.md`). The pass re-runs each DISTINCT gate the open rows name, once per pass whatever the row count, and writes `cleared` only on exit 0 (`clearDebt`, `internal/le/commit/actions.go`). A gate that exits non-zero leaves its rows open and prints its output. The pass runs whatever the named gates cost, `./le verify worktree` included, so run it in the foreground and let it finish.

**A cleared row says the gate was green over the COMMIT**, because every runnable gate runs inside ONE throwaway worktree at HEAD. A pass does not judge the uncommitted files several sessions keep in this checkout.

**When no worktree can be made, NOTHING clears and the pass exits 1. You MUST NOT read that as a gate failure.** The fallback it refuses is to run the gates against the working tree, and taking it would write `cleared` into the artifact that exists to hold verification evidence (`ai/rules/evidence.md`).

**A row the pass leaves open MUST be answered by doing the work the row names, never by editing the row.** What the pass runs, what it refuses to run, and why an unrunnable row keeps the debt open are `docs/architecture/testing/verify-freshness-scope.md`.

## Concurrency

**One `./le verify worktree` (or `./le test-chaos`) MUST run at a time
repo-wide.** Parallel runs share the build cache, the ports and the test
binaries. Every heavy native action is admitted through `./le job run`, which runs
a job now, queues it, or attaches it to an equivalent run, so a second verify
blocks automatically. The admission registry, its slot and stall settings, and
what a job entry holds are `docs/contributing/running-commands.md`.

**While another verify holds the slot, the second invocation MUST be left to
block.**

| Do | MUST NOT |
|----|----------|
| Let the second invocation block | Kill the running verify |
| When the run is yours in the same tree, read `tmp/ze-verify.log` rather than re-running | Delete a job entry to take the slot |
| When the wait message appears, do other work | Start `go test`, `golangci-lint` or a `ze-test` binary in parallel, which bypasses admission |

**A second `./le verify current mode full` cannot overlap the first: it blocks on the repo-wide lock**
and runs the whole thing again afterwards, so you MUST NOT start one while another
is live: it does not overlap the work, it doubles the wall clock.

**You MUST NOT edit the tree while a verify runs**, yours or anybody's. Regenerating an
index or touching a rule mid-run invalidates every stage that already read it, and
the failures it produces look exactly like real ones. Measured: one such run
reported five failing stages, all five self-inflicted, and none reproduced on the
settled tree.
