# Running Commands

**When:** running any test, build, lint, or verification command from Bash, or writing a shell loop that forks or waits
**Severity:** blocking
**Related:** testing, platform-linux, git-safety

## Directives

- **Prefer registered `./le` actions. A bare `go test` omits Ze's feature build tags and produces phantom reds in unrelated packages.**
- **Never pipe a test/build command through `head`/`tail`/`grep`/`awk`/`sed`/`cat` -- run clean, read the log.**
- **Never write a shell for-loop that forks an external command per iteration when a single invocation can process all inputs.**
- **Never poll for work you launched. A Bash command started with `run_in_background` re-invokes the session when it exits, so that notification IS the wait.**
- **A loop that watches the same command adds a process and reports nothing the notification does not already carry.**
- **Never write a `while` or `until` loop that calls `sleep`, and never put `pgrep` in a loop condition.**
- **A poll that is genuinely the only available signal MUST die on its own. Wrap it in `timeout <seconds>`.** An unbounded watcher outlives the reason it was started for, because the session that started it has moved on.
- **Stop a watcher the moment its reason changes.** `TaskStop` the background task. "It will end eventually" is how four of them come to tick at once.
- **One watcher at a time, and never faster than one wake every 30 seconds.** Each wake competes with QEMU, Docker and `./le verify current mode full` for the same cores. That contention is what makes the functional suites flaky, so a watcher can corrupt the run it is watching.
- **Foreground `sleep` is blocked by the harness because waiting is not work.** Reaching for a loop to win the sleep back inverts that intent. Do other work, or end the turn.
- **The harness's own examples are unbounded, and this repo overrides them.** The Bash tool text prescribes an `until` loop when a foreground `sleep` is refused, and the `Monitor` schema shows `until grep -q ...; do sleep 0.5; done`. Both are refused here, and one word fixes both: `timeout`. The 30-second floor governs a watcher that spawns a process per wake (`pgrep`, `docker`, `curl`); a local file test inside a bound can be faster.
- **Run `./le verify lint run` before claiming any Go implementation work is done.**
- **Fix every issue it reports. Do not claim done with lint failures outstanding.**

- **`go test`, lint analysis, and a `ze-test` runner MUST NOT start raw from Bash.** The hook names the registered native action.
- **Use a registered `./le` action when one owns the work.** For otherwise raw work, the exact generic grammar is `./le job run label <label> command <argv...>`.
- **Admission preserves the child exit status.** One job runs and peers queue or attach; the command inside remains the command being judged.
- **A one-off that MUST NOT queue states its reason in the command: `ZE_ADMIT_RAW="<reason>" <command>`.** An empty reason admits nothing, and the reason that is there lands in the transcript, which is what makes the escape auditable by reading the session.
- **A cheap subcommand of a heavy tool stays available: `golangci-lint config verify` runs no analysis and is not refused.**

- **A file under `plan/` or `ai/rules/` MUST NOT be written from Bash.** Use the Write or Edit tool. `bashGovernedWrite` in `internal/le/hookruntime/bash.go` refuses it.
- **The Write/Edit surface runs the native checks in `internal/le/hookruntime/writeedit.go`.** A Bash write reaches none of them.
- **The bypass is common.** Redirects, in-place editors, `tee`, copy, move, and interpreter payloads can all write a governed document. The native Bash guard binds both direct shell verbs and interpreter-shaped writes.
- **The interpreter tier over-matches on purpose.** A payload merely reading `plan/` and writing its result to scratch can be refused. State the governed-write admission reason when that is genuinely the operation.
- **Writing ABOUT these trees trips it too, and that is the shape you will meet first.** A commit-body heredoc that merely NAMES `plan/` or `ai/rules/` beside a write primitive is refused, even when the only file it writes is scratch. The check's own author met this while writing the body for the commit that added the check. The answer is the Write tool for that file, which is the correct tool anyway, or the escape with a reason -- not a reworded sentence that dodges the pattern.
- **A refusal that is wrong is answered by `ZE_ADMIT_GOVERNED_WRITE="<reason>"`, never by rewording the command.** It mirrors `ZE_ADMIT_RAW`: an empty reason admits nothing, and the reason lands in the transcript, so the escape is auditable by READING the session rather than by trusting it. A false positive costs one env assignment; a false negative costs the guard, and that asymmetry is the whole argument.
- **READING from Bash stays free in the shell tier, because it binds on the write.** `grep`, `cat`, `sed -n`, and `./le commit create file plan/spec-x.md dry-run` are not document writes and are not refused; the commit path names those paths constantly and would otherwise refuse itself.
- **A generated artifact under those paths is written by its generator, not by hand, so it is not what this governs.** `./le rules render-update` and `internal/le/commit` write there as tools; the rule is about an agent editing a document.

## CGO-Free Builds

- Non-race first-party Go compilation MUST set `CGO_ENABLED=0` in the process environment.
- This covers binaries, tests, benchmarks, fuzzing, `go run`, nested helpers, and installed project tools.
- A test-only command that uses `-race` MAY set `CGO_ENABLED=1`.
- Race binaries MUST NOT ship or serve as release or build evidence.
- Inherited CGO defaults MUST NOT be used.

## One Owner Runs The Suites

**Suite runs have ONE owner: the main thread, or one agent it dedicates to
running them. Every other agent MUST report the command it wants run, and stop.**
A suite target, the runner binary, a race run, a QEMU target and a Docker
deployment target all count.

**You MUST NOT attribute a suite result taken while another suite ran.** Saying
"that red is another session's" from such a run is a guess wearing evidence's
clothes, and it can dismiss a real defect as somebody else's noise.

**The evidence a fix owes is a single-test mutation, and a suite count MUST NOT
be offered in its place.** Revert the change, watch one named test go red,
restore. That is one `-run` on one package, and it costs an agent almost
nothing. A suite count proves the tree, never the fix, and it is the part that
does not survive contention.

- A known failing test MUST stay at the narrowest runnable scope until it passes. For Go tests, run `./le job run label unit-pkg command go test PKG=./path/to/package RUN='^TestName$' RACE=0`.
- Use `RACE=0` only for non-race iteration. A race or concurrency failure MUST keep race detection enabled.
- Run the required aggregate target, `./le verify worktree` or `./le verify worktree`, only once. Run it after focused tests pass and all edits are complete. You MUST NOT use either aggregate target to rerun one known failure.

- During development, the session MUST start with a focused test sample for the changed code path before it runs a fuller, aggregate, or full suite.
- The sample MAY include the test being developed.
- When that sample finds a failing test, the fix loop MUST use the narrowest command that reproduces that failure.
- The narrow loop MUST NOT stop the session from running more focused sample tests when needed to debug, find the failure boundary, or remove a blocker.
- The fuller, aggregate, or full suite runs after the focused debugging loop no longer finds a relevant failure. It MUST NOT be the first probe.

## Bare `go test` Lies -- Always Pass The Feature Tags

**A registered native action (`./le test-unit`, `./le verify worktree`) is the
route, and a run scoped to packages MUST carry the feature build tags itself.**
A bare `go test` omits them, so plugins never register and unrelated tests fail
with a phantom red. The tag list, the symptom, and the `git archive` variant of
the same trap are `docs/contributing/running-commands.md`.

## No Pipes On Expensive Commands

**`./le`, `go test`, `go build`, `golangci-lint`, `bin/ze*`, and any other test,
verify, or build command MUST NOT be piped through `head`, `tail`, `grep`,
`awk`, `sed`, or `cat`.** Run it clean, then read the log. Losing one failure
line costs the whole re-run. Where each run writes its logs is
`docs/contributing/running-commands.md`.

**Exception:** `| tee <file>` MAY be used -- it is non-lossy and captures
output to a file while still displaying it.

## Write Ad-Hoc Scratch Under Your Per-Session Dir

**Ad-hoc scratch MUST be written under this session's private directory,
`dir=$(./le session scratch ensure)`, and MUST NOT be written at the `tmp/`
root.** `tmp/` is keyed per checkout, so a fixed name there is one file for every
session in the tree and nothing removes it. Both write surfaces refuse it. Which
root names are shared by design, and what outlives a session, is
`docs/contributing/running-commands.md`.

## Your Binaries Live In This Session's Directory -- Ask For The Path

**`bin/ze` MUST NOT be hardcoded in a command, a script, or a doc. MUST ask the
owning native action (`./le functional <suite>`) for the path it built.** Every
binary a native test action builds lives in the current session's private
directory under a bare name, so a sibling session cannot overwrite the binary
under test. Why the path is looked up rather than recomputed, and how the
session store is seeded, is `docs/contributing/running-commands.md`.

## Never Launch a Functional Suite By Running The Runner Binary

**A functional suite MUST NOT be launched by running a `ze-test` binary
directly. MUST use `./le functional <suite>`.** A raw runner rebuilds a daemon
without the test-only surface, so it produces a convincing false red. The
`--server` and `--client` hints the runner prints on failure repeat that same
launch and MUST NOT be followed either. The mechanism, and the table of how to
run one suite, one test, or a VM suite, is
`docs/contributing/running-commands.md`.

## The Bash Hook Matches Your Command Text, Including Search Patterns

**A Bash refusal that fired on a SEARCH PATTERN is a false positive, and rephrasing the ban away or working around the guard's intent MUST NOT be the answer.**
MUST run the scan through the harness `Grep` tool instead, so the
banned verb never enters a Bash command line. Why the guard is coarse, and what
the substitute scan looks like, is `docs/contributing/running-commands.md`.

## No Fork Loops

**The fork-loop ban covers every `Bash` tool call, with no exemption for a one-off.**
First-party repository tooling belongs in native Go packages, and a shell script MUST NOT be added for it.

**When the loop body genuinely needs per-file logic that one command cannot
express, it MUST be batched with `xargs` or `find -exec +` rather than forked per
file.** One recursive `grep`, one glob, or one `find -exec +` is a single fork.
The measured cost is `docs/contributing/running-commands.md`.

## Lint Gate

**You MUST lint through `./le verify lint run`, never by calling
`golangci-lint` directly.** The native action derives the pinned toolchain and
every build flavor through `internal/le/verify/lint`; a bare invocation inherits
host defaults and can report an environment failure as a code finding.

The same rule applies to every tool whose native action configures its
environment. The action is the interface; reaching past it drops the
configuration that makes the result representative.

## Which Packages "Changed" Means

**You MUST read the selector's stderr before you trust a scoped run.** It widens
to `./...` and names the reason whenever it cannot narrow, and one reason is
routine: `tmp/ze-verify.status` holding no green commit. With nothing proven,
every scoped target judges the whole tree until a full run passes. The contract
is `docs/architecture/testing/verify-freshness-scope.md`.
