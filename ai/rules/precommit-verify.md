# Pre-Commit Verification

**When:** before running precommit-verify, judging its red in a shared checkout, or running the tracked-build check after a commit script
**Severity:** blocking
**Related:** git-safety, commands, completion

## Does `ze-precommit-verify` Apply?

BLOCKING only when the commit could plausibly affect build, tests, or
generated code.

| Files in commit | Run `ze-precommit-verify`? |
|-----------------|------------------|
| Any `.go`, `go.mod`, `go.sum`, `vendor/**` | YES |
| `Makefile`, `scripts/**`, build/CI config | YES |
| `*.yang`, generated code, codegen templates | YES |
| Anything that runs at build time or affects a binary | YES |
| `ai/**/*.md`, `.claude/**/*.md`, `plan/**/*.md`, `docs/**/*.md`, `README.md` | NO |

Mixed commit: one YES row -> run. Do not split a commit to skip.
Decision rule: "could this make a Go test fail or break the build?"
No = skip and note in commit summary. Unsure = run.

## Running The Gate

`make ze-precommit-verify`, in the foreground ("Running ze-precommit-verify" below). Not `go test`,
not any subset.
Before any verify target, check freshness. `scripts/dev/verify-status.sh check`
with no arguments asks about the whole tree, and `check <PATH>...` asks about the
named paths alone. A FRESH answer covers the paths it was asked about and forbids
rerunning `make ze-precommit-verify` or
`make ze-precommit-verify-changed`. The check output is qualified by mode:
`FRESH(ze-precommit-verify)` covers everything; `FRESH(ze-precommit-verify-changed)` is a
weaker pass (no full lint, no vet evidence, no cached full unit pass) --
for commit preparation treat both as FRESH, but when the work explicitly
needs the full gate (release evidence, repo-wide changes), a full
`make ze-precommit-verify` on a FRESH(ze-precommit-verify-changed) tree is permitted. A pass
recorded with skipped suites (`ZE_SKIP_SUITES`) reports STALE. `ze-precommit-verify` uses a two-pass strategy: cached
full pass (no `-race`) + `-race` only on component groups with changed
`.go` files. `ze-precommit-verify-changed` scopes to the packages the change set
reaches: the uncommitted and untracked paths, PLUS the paths committed
since the last green verify (`runSelector`,
`scripts/checks/verify_scope_selector.go`; baseline = `git_sha` in
`tmp/ze-verify.status`), so a package committed before it was verified is
still tested rather than skipped on the now-clean tree. What that
selection answers is below. For reactor concurrency changes, also run `make
ze-unit-reactor-test-race`. Output writes: `tmp/ze-verify.log`, per-stage logs
under that run's own `tmp/verify/run-<start>-<mode>-<id>/` (reach one through the
`detail-log` field of the failure index, never by guessing a path),
`tmp/ze-verify-failures.log`,
`tmp/ze-verify-failures.json`, and `tmp/ze-verify.status`. The full mode writes
`tmp/ze-verify-full.json` as well, which is the coverage record a Go-carrying
commit is gated on: the changed mode never writes it, so a cheaper run cannot
certify one.

- **One producer answers the change set: `runSelector` (`scripts/checks/verify_scope_selector.go`). `scripts/dev/changed-pkgs.sh` holds no selection logic and only dispatches between the two routes to it.**
- **A verify run selects ONCE, before the first stage, writes the answer into that run's own artifact directory, and names that file to every stage in `ZE_VERIFY_SCOPE_PACKAGES`.** Two runs of one checkout therefore never share a scope, and no two stages of one run scope to different trees.
- **A direct `make ze-lint-changed` or `make ze-unit-test-changed` outside a verify run has no published answer, so it selects its own** (2.4 to 2.9s). Both routes reach the same producer.
- **The import graph is built with `ze_core` and every tag in `feature-gates.txt`, so a `//go:build ze_<feature>` importer is selected.** One file under `internal/component/ssh` selects `./cmd/ze`, `./cmd/ze/hub` and `./internal/component/ssh`, and the feature answer is `ze_ssh` alone.
- **The reverse walk stops at two levels of importers, and the selector states on stderr how many packages that bound dropped.** `make ze-verify-scope-selector ARGS=--drop-log=FILE` records which ones.
- **Ask the selector what your own change set answers before you assume the scoped run covered it: `make ze-verify-scope-selector ARGS="--print=both"`.** On a tree several sessions have dirtied the answer is often still wide, and the narrow answer belongs to a change set that touches one feature.

| The change set holds | The scoped stages get |
|----------------------|-----------------------|
| a `.go` file | its package, plus every importer within two levels, with the feature tags on |
| `go.mod`, `go.sum`, or a `vendor/` path | `./...`, and the widening names the path: a dependency moved, so every package that compiles against it is reachable |
| a kind a rule names: `.py`, `rfc/`, `.md` under `ai/`, `plan/` or `docs/`, `Makefile`, `mk/*.mk`, `.github/*.yml`, a `.claude/hooks/` script | the tooling packages whose tests READ that kind, never the whole tree |
| a `.ci`, `.et` or `.wb` body under `test/` | the Go test packages that WALK that corpus. `.ci` selects `./internal/test/runner`, `./scripts/dev`, `./scripts/docvalid` and `./scripts/checks`; `.et` selects `./internal/component/cli/testing` and `./scripts/dev`; `.wb` selects `./scripts/dev` |
| a path under `examples/plugin/go`, matched BEFORE the `.go` rule | no package. It is a separate module, so `go list ./...` never reports it and nothing here compiles or reads it. Ordering is load-bearing: the `.go` rule would seed a directory no package owns and widen the whole run |
| a path under `gokrazy/modcache/` | no package. A third-party module cache every tree walker names in a skip list |
| a `.go` file the unit tag set never compiles, in the module root | `./scripts/dev` and `./scripts/checks`, the tree walkers that read it. `./...` does not compile it either, so widening would buy nothing |
| a `.go` file under `cmd/ze-installer` | `./...`. The row was no-widen until 2026-08-24. `scripts/dev/lint_flavors.py` now lints that package under a `ze_installer` flavor whenever the lint runs over `./...`. As a result, the wide answer is the only one that reports on an edit to the initrd's PID 1 |
| a kind no rule names | the package it sits in when that directory holds Go source, the tooling packages otherwise. The path is NAMED on stderr, which is the evidence for writing it a rule |
| nothing, and `tmp/ze-verify.status` holds no green commit | `./...`, and the widening names the condition. Without a proven commit, every commit in history is unverified, so a clean tree must not select nothing |

```
[ ] 0. `scripts/dev/verify-status.sh check`. FRESH -> MUST NOT run `make ze-precommit-verify` or `make ze-precommit-verify-changed` again; note timestamp. STALE -> continue only if the table above says verification applies.
[ ] 1. `make ze-precommit-verify` (foreground, largest timeout your harness allows, never killed early) only when status is STALE and the table above says YES. On failure read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open the stage log its `detail-log` field names in `tmp/ze-verify-failures.json`. Each run keeps its own directory, so a path from an earlier run is a different run's evidence.
[ ] 2. Failure from current work, or any failure that blocks this commit's goal: fix + re-run. Any other failure, and never a deterministic structural gate, which is fixed before any commit (see "Structural Gates Are Never Known-Red" below): write its spec, finish this commit, ask Thomas whether that spec runs (`ai/rules/completion.md`). A `plan/known-failures/` shard is for a failure you tried and could not reproduce, and it carries the reproduction attempt and the next step.
```

Each directive below is one physical line on purpose. `condense_body`
(`scripts/dev/rules_condensed.py`) emits a bold-led LINE raw into
`ai/rules/CORE.md`, so an instruction that wraps arrives there cut in half.

**Run `make ze-precommit-verify` in the foreground, wait for it, and never poll: the foreground return IS the completion signal.**
No background run, no sleep-and-check loop, no `tail` on a log that is still
growing.

**Do not kill it for being slow. Give the call the largest timeout your harness allows.**
A verify that is still running is not a verify that is hung, and killing one
costs the whole pass rather than the seconds it saves.

**When your harness caps a foreground call BELOW a full pass, you MAY start the run detached, and only where the harness raises a completion event of its own.**
The two directives above are one requirement, not two: wait for the completion
signal, and never substitute a guess for it. A harness that kills the call at a
fixed ceiling turns "run it in the foreground" into an instruction that cannot be
followed, and a truncated run rewrites no verify record, so a commit gate that
reads that record can never be cleared from inside such a harness. Where the
harness itself says when the job ended, that event IS the completion signal the
first directive names, and starting the run detached costs none of the
properties these directives protect.

**Waiting on that event is not polling. MUST NOT sleep-and-check, MUST NOT `tail` a log that is still growing, and MUST NOT start a second run to find out where the first one got to.**
The ban is on inventing a progress signal, never on the run being detached. A
second run is the worst of these: it contends for the same job slot, and
`_scan_and_claim` (`scripts/dev/ze-run.sh`) judges a holder by its log's mtime,
so a live run is never the one displaced.

**A harness that raises no completion event has no detached route: say the cap is in the way and hand the run to the operator.**
Reporting the limit costs one line. A verify nobody watched, whose record nobody
refreshed, is the failure this whole point exists to prevent, and claiming a pass
from a run that was killed at a ceiling is worse than not running it.

**Never edit the tree while the run is detached, and treat the wait as the same blocking wait a foreground call would have been.**
Detached says where the completion signal comes from. It does not license doing
other work in the tree meanwhile, because the run reads the working tree either
way.

**Never take a timeout from a duration written in a rule: read `tmp/.ze-verify-duration.txt` instead.**
How long a full pass takes depends on the machine, and on what else that machine
is doing. "25 to 30 minutes" below and "4-10 minutes" in `ai/rules/testing.md`
are not a contradiction. They are different hardware. A loaded VM is not
deterministic either, so even one machine gives a spread rather than a figure.
`_release` (`scripts/dev/ze-run.sh`) appends the real elapsed
seconds for the machine you are on, and `tmp/*` is gitignored, so that file is
the only per-machine record there is. Read it as an expectation, never as a
threshold: a run past it is a slow run, not a failed one.

**A slow run is never broken for being slow, so there is no threshold to raise.**
A waiter breaks a holder's slot only when that holder is DEAD, or when it has
made no progress for the stall window: `_scan_and_claim` (`scripts/dev/ze-run.sh`)
judges progress by the mtime of the job's log, never by elapsed time. A run still
writing stages is a run still working, however long it has taken. `ZE_JOB_STALL_SECONDS`
sets the window and is bounded to 60..3600; a value outside that range is refused
before the job starts, so raising it past an hour is not a route to anything.

**Never edit the tree while a verify runs, yours or anybody's: it reads the working tree.**
An edit mid-run invalidates the run you are waiting for.

**`make ze-precommit-verify` is a 25-stage full gate and takes 25 to 30 minutes. You MUST run it ONE
time, when the work is finished and you are about to prepare the commit script.**
Running it to "check in" mid-change is the single most expensive habit available in
this repository, and it buys nothing a scoped check does not.

**You MUST run what the change touches.** Every surface has one owning target, and it costs
seconds to minutes rather than half an hour. Find yours in this table, run it after
each edit, and keep `ze-precommit-verify` for the end.

**You MUST go through `make`, or carry `GOCACHE` yourself.** `Makefile` exports
`GOCACHE := $(CURDIR)/cache/go-cache`, and that export reaches make RECIPES only. A
bare `go test` typed into a shell uses the user's own `~/.cache/go-build` instead,
so it rebuilds the world cold, shares nothing with `ze-precommit-verify`, and leaves the
project cache no warmer than it found it. `Makefile` also defines the canonical
invocation (`GO_TEST`, `GO_TEST_RACE`): the feature tags, timeout, and
`GOMAXPROCS`. `GO_TEST` explicitly uses `CGO_ENABLED=0`. The test-only
`GO_TEST_RACE` explicitly uses `CGO_ENABLED=1` with `-race` on Linux and Darwin.
Its race-built test executables never ship or serve as release/build evidence.
A bare `go test` drops all of it (`ai/rules/commands.md`, "Bare `go test` Lies").

`make ze-unit-pkg-test PKG=<pattern>` is the supported way to test ONE package while
you develop it. It carries all of the above. Add `RUN=<regexp>` to narrow, and
`RACE=0` to drop `-race` while iterating -- but a package tested without `-race`
has not been tested the way `ze-precommit-verify` tests it, so put it back before the end.

```
make ze-unit-pkg-test PKG=./internal/component/ike/eap
make ze-unit-pkg-test PKG=./internal/component/ike/... RUN=TestEAPTLS
```

| You changed | Run this |
|-------------|----------|
| A `.go` file | `make ze-unit-pkg-test PKG=<that package>`, or the group target covering it (`ze-unit-bgp-test`, `ze-unit-core-test`, `ze-unit-plugins-test`, `ze-unit-config-test`, `ze-unit-cli-test`, `ze-unit-rest-test`). Then `make ze-lint-changed` (`ai/rules/commands.md`) |
| A `.go` change that alters what the daemon PUTS ON THE WIRE, installs, or shows | the FUNCTIONAL suite owning that surface as well: `make ze-functional-plugin-test`, `ze-functional-encode-test`, `ze-functional-decode-test`, `ze-functional-parse-test`, `ze-functional-reload-test`, `ze-functional-ui-test`, `ze-functional-web-test`. The unit tests of the package you edited are not evidence about the rail |
| Reactor concurrency (`reactor/session*.go`, `forward_pool*.go`, `peer.go`) | `make ze-unit-reactor-test-race` (`ai/rules/testing.md`) |
| A `.ci` or `.et` test | its suite target: `make ze-functional-plugin-test`, `ze-functional-parse-test`, `ze-functional-encode-test`, `ze-functional-editor-test`, `ze-functional-web-test`. Draft first in `test/draft/` |
| Linux-only code (`//go:build linux`) | `make ze-qemu-integration-test`, or `make ze-qemu-needs-linux-test` for a `needs-linux` `.ci` (`ai/rules/platform-linux.md`) |
| `rfc/short/*.md`, an `RFC requirement:` tag, `rfc/extraction/*` | `make ze-rfc-check` |
| `docs/**`, `ai/**`, `plan/**` | `make ze-doc-verify`, and `make ze-doc-wiring-check` for the changed-file gates |
| `ai/rules/*.md` | `make ze-rules-condensed-update` then `make ze-rules-lint`, and commit all three digests with the rule |
| A `*.yang` file or a `ze:command` | `make ze-doc-verify`, `make ze-cli-grammar-check` |
| A plugin `register.go`, or anything generated | `make generate`, `make ze-plugin-imports-check` |
| A new package's placement | `make ze-tier-check` |
| Anything, once the commit script has run and it carried Go | `make ze-repository-tracked-build-check` -- the only check that compiles what git holds |
| A `scripts/dev/*.py` tool | `make ze-unit-pkg-test PKG=./scripts/dev`, which runs the sibling `*_test.py` through `TestPythonUnitTests` and the Go tools' tests in one pass. A raw `python3 <tool>_test.py` is refused by the Bash hook (`ai/rules/commands.md`) |
| Several of the above, and you want breadth | `make ze-precommit-verify-changed` |

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

**When the table has no row for what you touched, you MUST derive it.** `mk/*.mk` names every
target and what it runs, `make help` lists them, and `ai/rules/repo-maintenance.md` maps
each gate to the rule it enforces. When a surface has no owning target, you SHOULD say so
in the report rather than reach for the full gate.

Unless Thomas Owner Override is active, never commit with lint issues and never
commit without test evidence when code changed.

```
[ ] 3. Spec completion gate (if driven by a plan/ spec):
      [ ] Journal row appended to plan/journal/<class>.md, its Spec cell naming the spec stem
      [ ] Spec file staged for deletion (git rm)
      Not done -> STOP.
[ ] 4. Executive Summary Report (rules/planning.md). What was done, what is left.
```

## Reading A Red

**YOU MUST READ THE WHOLE FAILURE SUMMARY BEFORE YOU RE-RUN.** A verify run ends with
`FAIL N verify stage(s) failed` and one line per failing stage, and
`tmp/ze-verify-failures.log` holds the same list. A re-run started from a partial
read costs another half hour and usually reports the same stages. Two specific
traps, both of which have cost a full run:

- **`tail` on the log of a run that is still going.** The stage banner tells you
  where it is (`### Stage 18/22`). You MUST check `scripts/dev/verify-status.sh check` for
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
the last.** `commit_helper.py create` refuses unless
`scripts/dev/verify-status.sh check` reports FRESH, and only a full `ze-precommit-verify`
writes that record. A narrow fix therefore still needs one more full run before a
commit, which is precisely why the full run MUST come AFTER every gate you can
check cheaply is already green: `make ze-lint`, the touched packages' `go test`,
and the gate owning each surface you changed. A run started before those are clean
is a run you will pay for twice.

**You MUST NOT stop to ask which way.** The operator is often not present, and a session
that halts on this question has spent their time to save its own. You MUST clear the cheap
gates, run the full pass once, and commit.

**Several agents work this checkout at once. `make ze-precommit-verify` reads the WORKING
TREE, so it reads their half-finished edits too, and a fully GREEN run is
unreachable by construction. You MUST NOT wait for one and you MUST NOT re-run for one: what is unreachable is the green bar, never the run.**

**Owner directive, 2026-08-17: a commit carrying `.go`, `go.mod`, `go.sum` or `vendor/` MUST be preceded by a full `make ze-precommit-verify` whose run STARTED after your last Go edit. You MUST NOT reach such a commit on scoped gates alone, and you MUST NOT re-run the gate to watch somebody else's red clear.** What the commit owes is the run's COVERAGE, never its exit code: the exit code is read through the attribution table below. `commit_helper.py create` enforces the coverage and names the owner-only escape when no such run exists.

**One full run covers EVERY commit prepared from it. You MUST NOT re-run the gate between back-to-back commits of the same code.** The debt is incurred by an EDIT, never by a commit: one body of work split into three commits owes one run, not three, and the same run answers for all of them. What owes a fresh run is a Go file written again after that run started, and nothing else does.

| The failing path | Whose red | What you do |
|------------------|-----------|-------------|
| In this commit's `--file` list | Yours | Fix it. A red you caused is never attributed away |
| Dirty in `git status --porcelain`, and not in your list | Another session's | Take that code as working. Name it in `--unverified` and commit |
| Clean and tracked, and your diff PRODUCES a symbol the failure names | Yours | Fix it. Ownership follows the producer, not the file that failed |
| Clean and tracked, and unrelated to your diff | Pre-existing | Attribute it against `git log`, name it in `--unverified`, and commit |
| Any deterministic structural gate | Yours until you prove otherwise | Fix it. Those read files rather than a moving tree. The helper drops the charge only when every file the failure groups name lies outside your commit |

**A structural gate red is charged to your commit unless EVERY file its failure groups name lies outside your `--file` list. You MUST NOT expect attribution to drop a red whose groups name no file at all.** `structural_gate_reds` (`scripts/dev/commit_helper.py`) reports three sets: `charged` refuses the commit, `foreign` names each gate the file list ruled out, and `unattributed` names each group that carries a check name, a suite name or the stage's own name. A group that names nothing is charged exactly as before, and the refusal prints which one it was. Attribution reaches the gates whose groups name a file or a package directory, and `ze-doc-wiring-check` is one of them: its sub-checks declare their own failure groups (`declare_failure_group`, `scripts/dev/verify_wiring_docs.py`). Its ci-sleep ratchet and its delegated targets judge a population rather than a file, so those two still charge.

| What the red gate's failure groups name | What the helper does | What you do |
|---|---|---|
| Files, and one of them is in your `--file` list | Charges the gate and refuses | Fix it. The red is yours |
| Files, and every one lies outside your list | Drops the charge and prints the gate name | Commit. The gate is still red for the tree, so say whose it is in your report |
| A check name, a suite name, or the stage itself | Charges the gate and names it as unattributed | Read that stage's log and attribute the red by hand. The file list cannot rule it out for you |

| Gate | What its groups name | Expect |
|------|----------------------|--------|
| `ze-lint`, `ze-lint-changed` | the `.go` file each finding sits in | a drop when none of them is in your `--file` list |
| `ze-evidence-vet` | the package pattern of each red | a drop when your list holds no file under it |
| `ze-doc-wiring-check` | the files each sub-check is about, one declared group per failure | a drop, except for the ci-sleep ratchet and a delegated target, which name no file and charge |
| Every other stage, `ze-generated-files-check`, `ze-doc-links-check` and `ze-test-weakened-check` among them | the stage's own name, through `genericGroup` (`scripts/status/verify_run.go`) | a charge, always. Read that stage's log and attribute the red by hand |

`ze-doc-wiring-check` is the gate most open `structural gates (red)` rows in
`plan/verification-debt/` name, so attribution now reaches the largest single
class of them. It does not reach most of the rest: a large minority of those rows
ALSO name a gate no classifier attributes, led by `ze-generated-files-check`,
`ze-doc-links-check` and `ze-test-weakened-check`, and a further group names no
gate at all in its Reason prose. A commit meeting one of those is refused exactly
as before. Read the live split off the ledger rather than a number written here,
which goes stale as sessions commit.

Attribution also answers a NARROWER question than the ledger asks. It says the
files this commit carries cannot have caused the red. It never says the red is
somebody else's work rather than yours from an earlier session.

The declared-group protocol is available to EVERY stage, not only this one
(`parseDeclaredGroups` and `classifyStage`, `scripts/status/verify_run.go`).
Teaching another producer to declare its groups is separable work with its own
evidence, and until it is done that stage's red charges whoever is committing.

**Owner directive, 2026-08-17: code another session holds uncommitted MUST be taken as WORKING. You MUST NOT fix its red, wait for it, or re-run the gate to see whether it cleared.** Attribution is the whole answer: name the file and say whose it is, put that in `--unverified`, and commit. The row that MUST NOT be attributed away is a red your own diff produced, and the table above is what decides which row you are on.

The full gate saturates every core for half an hour, and that contention is what
makes the functional suites flake, so a run started to prove your own work
reddens somebody else's at the same time. Expect reds you did not cause. That is
the condition the attribution table answers, and it is why the run is made ONCE,
in the foreground, at the end of the work.

**A verify verdict answers about the paths it was ASKED about, and `commit_helper.py create` asks about the commit's own `--file` list. You MUST NOT read a FRESH as a verdict on the whole checkout.** `verify_status` (`scripts/dev/commit_helper.py`) passes that list to `verify-status.sh check <PATH>...`, and `manifest_scoped` (`scripts/dev/verify-status.sh`) compares only the named rows. An edit another session makes to a path your commit does not carry no longer makes your evidence STALE. Three limits come with the narrower question:

- A path that MOVED while the run was in flight is STALE whatever it holds now, because no stage judged the content it holds today (`MOVED_MARKER`, `scripts/dev/verify-status.sh`). The record names which paths moved instead of voiding the run, so this is finer granularity and never leniency.
- `check` reads the run's recorded exit code BEFORE it reads any scope, so a run that FAILED is STALE for every path list. Scoping is no route around a red run, and it is no route around a red structural gate either: that is `structural_gate_reds`, and it still reads every red the run recorded.
- `check` with no path arguments keeps its whole-tree meaning. You MUST use that form when the question is about the tree rather than about one commit.

**A commit that carries NO Go owes no full run. You MUST scope its evidence to YOUR
files, running the narrow gate that owns each surface it changes.** Before preparing
the commit script, on either route:

1. You MUST run the gate the commit owes: `make ze-precommit-verify` when it carries Go,
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
reproducible and yours to fix when your diff caused them: `ze-lint`,
`ze-rules-lint`, `ze-doc-verify`, `ze-rfc-check`, `ze-doc-wiring-check`,
`ze-tier-check`. You MUST always green those. It is the TEST stages -- unit, functional,
web -- whose reds a concurrent tree can manufacture.

When `make ze-precommit-verify` is known-red from failures this session did not cause --
pre-existing reds, or a separate session is actively clearing the global suite --
a commit carrying NO Go is gated on changed scope only. Rerunning the full gate for
it re-surfaces other sessions' noise that is not your regression and blocks progress.
A commit carrying Go still owes the full run above: the known red is ATTRIBUTED
there, and is never a reason to skip it. Gate the rest on changed scope only:

You MUST run these scoped gates instead:

- `make ze-lint-changed`
- the touched packages' `go test` (or `make ze-precommit-verify-changed`)
- `make ze-doc-verify` / `make ze-repository-check` when those surfaces changed
- a QEMU run for any linux-only runtime code touched

Then prepare the user-run commit script listing ONLY this session's files
explicitly in `commit_helper.py create --file ...`, so the commit never pulls in
another session's working-tree edits; exclude other sessions' files when
reviewing `git diff`. This applies only when the global red is not yours -- a red
caused by your own change must be fixed, not scoped around. Activate it only on an
explicit owner direction (e.g. "another session is clearing ze-precommit-verify, check only
what we changed"), never inferred from a red suite alone.

**The red MUST be attributed, not assumed (BLOCKING).** "Known-red" means you
have identified the specific failing stage/test and confirmed it is pre-existing
(logged in `plan/known-failures/`) or owned by another active session. An
*undocumented* red is NOT scope-aroundable: treat it as possibly your own
regression until proven otherwise. Scope-to-changed has a blind spot -- it tests
packages you edited, not packages your edit breaks **transitively**: a new import
can break a different package's compile/test (a real case: `aihelp` broke
`bgp/config` through `plugin/all`), a config-driven gap surfaces only in a
consumer's tests (a missing YANG typedef failed `bgp/config`, not the plugin that
introduced it), and adding a plugin invalidates the `plugin/all` golden
snapshots. Before scoping around a red, `go test`/`vet` the reverse-dependency
closure of your changed packages, or run full `ze-precommit-verify` once.

**You MUST NOT let a red persist.** Scope-to-changed is a temporary bridge while the
global suite is being cleared, not a standing mode. A `ze-precommit-verify` that stays red
across sessions hides newly-introduced breakage under the existing red -- that is
exactly how an import cycle, a YANG typedef gap, and stale registry snapshots all
landed under one persistent red without any gate firing. You MUST log the failing stage in
`plan/known-failures/` with who owns clearing it; if nobody does, clearing it
MUST come before stacking more changes on top.

## What May Be Overridden

The pre-commit checklist's "write its spec, finish this commit, ask" branch, and
its `plan/known-failures/` shard, are for **non-deterministic** failures only.
Those are flaky or environmental TEST reds: load-sensitive races, GC-pressure pool
flakes, host-specific listener probes ("Reading A Red", above). A **deterministic
structural gate** is NEVER eligible: `ze-lint`, `ze-lint-changed`, `ze-tier-check`,
`ze-evidence-vet`, `ze-plugin-boundary-check`, `ze-iface-resolution-check`,
`ze-generated-files-check`, `ze-doc-wiring-check`, and `ze-repository-tracked-build-check`
fail only when the tree is structurally broken (a misplaced module tier, a
lint/vet violation, a broken plugin boundary, an unresolved iface, a stale
generated file, a stale wiring index, a HEAD that does not compile). Such a red
must be fixed at the source before any commit -- do not park it, do not
`--unverified` past it.

**The general escape is owner-only: `--structural-red-ok "<reason>"`** (the
narrow `--broken-head-fix` above is the only other, and it reaches one gate).
It is a
SEPARATE flag from `--unverified` precisely so the flaky-test path can never
reach this branch, it refuses an empty reason, and it prints the red gate names
with the reason to stderr so a red tree can never look green in a transcript.
You MUST use it only when Thomas says so and the red provably belongs to another
session's in-flight work that this commit cannot affect. It exists because a
refusal with NO escape made a green tree the only route to any commit at all,
including one touching no compiled code -- which pushed sessions toward the real
hole this gate was built to close: widening `--unverified`, or editing
`STRUCTURAL_GATES` to drop the failing name. An override that is written down
and shouted is safer than one that is improvised. It is never a substitute for
fixing your own red (`ai/rules/completion.md`).

`ze-generated-files-check` qualifies on the rule's own terms: a stale generated
file is deterministic, reproducible, and fixed by `make ze-generated-files-update` (or the
specific `--fix` the failing check names). It is never flaky or environmental.

Thomas owns the repository and may explicitly override the `ze-precommit-verify`
requirement for commit-script preparation. This override exists because an
OpenAI session blocked Thomas by treating the agent rule as if it also bound
the repository owner. It was added for OpenAI behavior, not for Anthropic.

The override is valid only when Thomas explicitly directs both parts:

Thomas MAY use the override to do two things:

1. prepare a commit script, and
2. skip tests, skip verify, or commit without running tests.

Examples that activate it: `owner override: commit without verify`, `commit no
test`, `commit without running tests`, or an equivalent direct instruction from
Thomas in the active conversation. Do not infer the override from urgency alone.

When the override is active:

- You MUST NOT run `make ze-precommit-verify`, `make ze-precommit-verify-changed`, lint, or tests as a
  late commit gate.
- You MUST inspect only enough state to stage exactly the requested files and avoid
  ignored, generated, unrelated, or user-owned paths.
- You MUST use `scripts/dev/commit_helper.py create` with the normal user-run script
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
- You MUST NOT claim tests, lint, `ze-precommit-verify`, integrations, or behavior were
  verified if they were skipped.

## After The Commit

**Nothing else in this repository COMPILES what git holds.** `make ze-build`,
`ze-precommit-verify`, `ze-lint-changed`, `ze-rfc-check` and every test target build and
run your WORKING TREE, uncommitted and untracked files included. (One gate does
read the commit: `commit_helper.py` judges discovery-index freshness against a
materialized HEAD. It regenerates indexes; it compiles nothing.) So you MUST NOT
commit a CONSUMER while its PRODUCER stays uncommitted: it is green for you and
broken for everybody who builds what git holds. This is a structural blind
spot.

`make ze-repository-tracked-build-check` (`scripts/checks/tracked_build.go`) is the one
check that reads what git holds: it extracts the commit with `git archive` and
compiles six build flavors of the extracted tree. Three rules follow.

| Situation | Do |
|-----------|-----|
| You are about to `--file` a consumer | Name the file that DEFINES every symbol it newly uses, and check that file is in the same `--file` list or already committed (`git log -1 -- <path>`) |
| The commit script has just run and it carried Go | Run `make ze-repository-tracked-build-check`. About 45s. This is step 7 of the commit workflow, not an optional extra |
| It goes red | Commit the producer. Never revert the consumer, and never park it: HEAD is broken for everyone until you do |

`ze-repository-tracked-build-check` is the one entry whose red is cleared BY a commit
rather than before one. It judges what git already holds, so a broken HEAD is
fixed by committing the producer a previous commit left behind, and every other
gate on the list is fixed in the working tree first. Refusing every commit until
it goes green would therefore deadlock: the refusal would block the only commit
that can lift it. **`--broken-head-fix "<reason>"` is that commit's route
through**, and it is narrow by construction: `commit_helper.py` accepts it only
when tracked-build is the ONLY structural red, so a lint, tier or wiring failure
riding alongside still refuses. Run `make ze-repository-tracked-build-check` after the
script and confirm it went green. If it did not, HEAD is still broken for
everybody who builds it.

`REV=<commit-ish>` judges any commit, so a break found later is bisectable:
`make ze-repository-tracked-build-check REV=<commit-ish>`. `ARGS=--keep` leaves the extracted
tree in place for inspection.

**What it does NOT read: test files.** `go build` MUST NOT compile `_test.go`, so a
test file committed without its fixture producer stays invisible here.

Known gap, recorded rather than papered over. Several checks run under BOTH
`ze-doc-verify` and `ze-generated-files-check`. That overlap is harmless: the runner
continues across stage failures, so one underlying red fails both stages in the
same run, `structural_gate_reds` always sees `ze-generated-files-check`, and the
commit is blocked regardless of what `plan/known-failures/` says about
`ze-doc-verify`. The real gap is the checks that run ONLY under `ze-doc-verify` --
`doc_drift.go`, `commands.go`, `digest_check.py`, and `rfc_requirements.py
--check-fresh` (`mk/check-docs.mk`; note the script's `--selftest`/`--check`
invocations DO run as the `ze-rfc-check` stage, so only the `--check-fresh`
ledger-staleness one is doc-test-exclusive). Those are just as deterministic and
structural, and they ARE
parkable, because `ze-doc-verify` is not in the set. Whoever picks this up should
decide whether `ze-doc-verify` belongs in `STRUCTURAL_GATES`; that is where reds
actually escape.

This list is the prose mirror of `STRUCTURAL_GATES` in `scripts/dev/commit_helper.py`,
and every name in it must be a stage `stagesForMode` actually emits
(`scripts/status/verify_run.go`) -- otherwise the entry matches nothing and gates
nothing. `test_structural_gates_are_live_stages` (`scripts/dev/commit_helper_test.py`)
and `TestStructuralGatesAreLiveStages` (`scripts/status/verify_run_test.go`) enforce
that. Every named gate is a live verify stage. The underlying CLI grammar gate
runs through `TestCLIGrammarGateStatic` (`scripts/checks/cli_grammar_test.go`).

This is enforced, not honor-system: `scripts/dev/commit_helper.py create` reads
`tmp/ze-verify-failures.json` (which `verify_run.go` rewrites after every run) and
refuses to prepare a script while a structural gate red is charged to this
commit, even with `--unverified` (`structural_gate_reds` / `STRUCTURAL_GATES`).
A red is charged unless every file its failure groups name lies outside the
commit's `--file` list, and a group that names no file is charged as it always
was ("Reading A Red", above). A green verify rewrites the artifact, so a
fixed-and-reverified gate clears automatically.

**Verification debt MUST be cleared by RUNNING the gate: `make ze-verify-debt-clear`. You MUST NOT edit a row to `cleared`.** Every override on `commit_helper.py create` writes a row into `plan/verification-debt/<session>.md`, and `create --push` refuses while one is open (`ai/rules/completion.md`). The pass re-runs each DISTINCT gate the open rows name, once per pass whatever the row count, and writes `cleared` only on exit 0 (`clear_debt`, `scripts/dev/commit_helper.py`). A gate that exits non-zero leaves its rows open and prints its output. The pass runs whatever the named gates cost, `make ze-precommit-verify` included, so run it in the foreground and let it finish.

**A cleared row says the gate was green over the COMMIT.** Since 2026-08-22 every runnable gate runs inside ONE throwaway worktree at HEAD (`clear_debt`, `scripts/dev/commit_helper.py`). A pass no longer judges the uncommitted files several sessions keep in this checkout. Before that change a `cleared` meant only "green HERE". Such a pass CAN go red on work nobody in it owns, and green on work nobody in it wrote.

**When no worktree can be made, NOTHING clears and the pass exits 1.** You MUST NOT read that as a gate failure. The fallback it refuses is to run the gates against the working tree. That fallback is the defect the worktree removes. Taking it writes `cleared` into the artifact that exists to hold verification evidence (`ai/rules/evidence.md`).

**A pass whose every row names an unrunnable gate materializes no worktree.** A worktree is a full checkout. A pass with nothing to run MUST NOT pay for one.

The pass clears no row whose gate no command produces. A row naming
`independent critical review` prints UNRUNNABLE and stays open, and a row naming
a gate string the runner table does not hold stays open the same way. Those rows
are answered by doing the work the row names, which for a review is `/ze-review`
recorded through `scripts/dev/review_gate.py`. Read what the pass printed before
you report the ledger clear.

## Concurrency

One `make ze-precommit-verify*` (or `ze-chaos-verify`) at a time repo-wide --
parallel runs share build cache + ports + `bin/ze` processes and
trash each other. Every heavy target is wrapped by
`scripts/dev/ze-run.sh`, which runs a job now, queues it behind the
jobs already in flight, or attaches it to an equivalent run;
`scripts/dev/verify-lock.sh` is an alias for it, so a second verify
blocks automatically. The slot count is `ZE_RUN_SLOTS` in the
`Makefile`, beside the `GO_TEST_PROCS` ceiling that sizes each job.

Admission state is one file per running job,
`tmp/.ze-jobs/<label>.<pid>.job`. `tmp/.ze-verify.lock` is dead:
nothing takes that flock any more. A job started INSIDE another job's
slot runs straight through instead of queueing behind its own parent,
which is how every stage of `ze-precommit-verify` runs.

| Do | Don't |
|----|-------|
| Let the second invocation block | Kill the running verify |
| If the run is yours (same tree), read `tmp/ze-verify.log` instead of re-running | Delete the lockfile |
| If "waiting for lock" appears, do other work | Start `go test` / `golangci-lint` / `bin/ze-test` in parallel (bypasses lock) |

Lock releases when the command exits. `flock` is fd-backed, not
PID-backed -- no cleanup after a crash.

**A second `ze-precommit-verify` cannot overlap the first: it blocks on the repo-wide lock**
and runs the whole thing again afterwards, so you MUST NOT start one while another
is live: it does not overlap the work, it doubles the wall clock.

**You MUST NOT edit the tree while a verify runs**, yours or anybody's. Regenerating an
index or touching a rule mid-run invalidates every stage that already read it, and
the failures it produces look exactly like real ones. Measured: one such run
reported five failing stages, all five self-inflicted, and none reproduced on the
settled tree.
