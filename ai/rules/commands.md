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

The reason is attribution, not speed and not memory. Suites share the build
cache, the ports and the `bin/ze` processes. A concurrent run therefore makes a
red that belongs to nobody. A killed process and a real defect read the same in
a log.

The repo-wide verify lock says this for one target. This says it for every
suite, and it names who holds the right to run one.

**You MUST NOT attribute a suite result taken while another suite ran.** Saying
"that red is another session's" from such a run is a guess wearing evidence's
clothes, and it can dismiss a real defect as somebody else's noise.

This costs an agent almost nothing. The evidence a fix owes is a single-test
mutation: revert the change, watch one named test go red, restore. That is one
`-run` on one package.

A suite count proves the tree, never the fix. It is also the part that does not
survive contention.

- A known failing test MUST stay at the narrowest runnable scope until it passes. For Go tests, run `./le job run label unit-pkg command go test PKG=./path/to/package RUN='^TestName$' RACE=0`.
- Use `RACE=0` only for non-race iteration. A race or concurrency failure MUST keep race detection enabled.
- Run the required aggregate target, `./le verify worktree` or `./le verify worktree`, only once. Run it after focused tests pass and all edits are complete. You MUST NOT use either aggregate target to rerun one known failure.

- During development, the session MUST start with a focused test sample for the changed code path before it runs a fuller, aggregate, or full suite.
- The sample MAY include the test being developed.
- When that sample finds a failing test, the fix loop MUST use the narrowest command that reproduces that failure.
- The narrow loop MUST NOT stop the session from running more focused sample tests when needed to debug, find the failure boundary, or remove a blocker.
- The fuller, aggregate, or full suite runs after the focused debugging loop no longer finds a relevant failure. It MUST NOT be the first probe.

## Bare `go test` Lies -- Always Pass The Feature Tags

`go test ./...` is **NOT** equivalent to `./le test-unit`. Ze compiles features
out behind build tags (`//go:build ze_isis`, `ze_ospf`, `ze_ldp`, `ze_rsvpte`,
`ze_web`, `ze_ssh`, ...). `internal/le/gotoolchain` derives the feature set from
`feature-gates.txt` for native unit and verification actions. Omit those tags and the plugins
never register, so their validators, listeners and schema vanish and **unrelated
tests fail with phantom reds**.

**SHOULD prefer a registered native action** (`./le test-unit`, `./le verify worktree`). When you
MUST scope to packages, MUST pass the tags:

```
go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')" ./internal/component/foo/
```

Same for `git archive HEAD` scratch-tree checks: a bare run there reproduces your
own mistake and "confirms" a red that does not exist.

A bare `go test` omits feature tags and can produce a phantom red with a
plausible but false root cause. Use the owning native action so the result
describes the real build.

Symptom: a test asserting on something registered by another feature
(listeners, validators, plugin names, wire methods, schema) fails, and the
failure says a thing is *missing* or *not produced*. Check the tags before
believing it.

## No Pipes On Expensive Commands

Never pipe `./le`, `go test`, `go build`, `golangci-lint`, `bin/ze*`, or any
test, verify, or build command through `head`, `tail`,
`grep`, `awk`, `sed`, `cat`. Run clean. Read the log after.

**Exception:** `| tee <file>` MAY be used -- it is non-lossy and captures
output to a file while still displaying it.

Losing a failure line to `| head` means re-running the whole thing.
`./le verify worktree*` writes to `tmp/ze-verify.log` (+ `-failures.log`
summary) by default. Override with `ZE_VERIFY_LOG=tmp/ze-verify-$$.log`
to avoid collisions between concurrent sessions. Read logs with the
Read tool, with `offset`/`limit` for paging.

## Write Ad-Hoc Scratch Under Your Per-Session Dir

`tmp/` is shared by every concurrent session in this checkout (it is keyed
per-checkout, not per-session -- `internal/le/scratch/scratch.go`). A fixed name at
the `tmp/` root -- `tmp/out.log`, `tmp/stdout`, `tmp/gotest.log` -- collides with
a sibling session writing the same name, and is never cleaned when your session
ends.

**A file at the `tmp/` root is REFUSED, on both surfaces that create one**:
`bashScratch` and the Write/Edit path check in `internal/le/hookruntime` answer
alike. A path carrying a directory component passes, including a session's
private scratch path and a producer-owned subdirectory.
Session-keyed names and producer-owned root artifacts are explicit exceptions in
`internal/le/hookruntime.IsAdHocScratch`.

Write ad-hoc scratch under this session's private directory instead:

```
dir=$(./le session scratch ensure)          # <session-dir>/scratch/, created for you
./le test-unit > "$dir/unit.log" 2>&1
```

**Nothing under `tmp/session/` is ever deleted automatically**: not at session
end, not on an age timer, not by a hook. Your directory outlives your session,
so a log you wrote is still there tomorrow. `./le session reap` removes only
session directories whose owners are provably gone. Do NOT relocate artifacts
that are already session-keyed (commit scripts under the session directory) or
shared by design (`tmp/ze-verify.*` and the durable cache); those stay put.
`internal/le/gotoolchain` assigns the repository Go build cache to native test
and verification actions.

## Your Binaries Live In This Session's Directory -- Ask For The Path

Native test actions build their binaries inside the current session's private
directory, under bare names. `internal/le/functional/binaries.go` resolves that
location and supplies the pair to the suite. A sibling session therefore cannot
overwrite the binary under test.

**MUST NOT hardcode `bin/ze`** in a command, script, or doc. Ask:

```
./le functional <suite>
```

The directory carries the id, so the file name does not. That is what keeps
argv[0] personality dispatch working (`cmd/ze/dispatch.go` `binarySuffixRoot`
reads the segment after the last `-`) and lets a `.ci` test exec `ze` by bare
name off one PATH entry. A binary's location also decides where `ze` resolves
its config and database (`internal/core/paths/paths.go` `ConfigDirFromBinary`),
so a session's `ze` reads `<session-dir>/etc/ze` and the repository's `etc/ze`
is the human's alone.

The directory is LOOKED UP, never recomputed: every consumer takes the single
directory matching `tmp/session/????-??-??-<id>`, and names a new one with
today's date only on a miss. Recomputing from today's date would move a
session's directory at midnight and orphan the binaries it is running.

The session-local `etc/ze` is seeded once by
`./le session seed-store binary <session-bin>/ze`. `SeedStore` in
`internal/le/session/seed.go` validates that the binary belongs to the current
session directory. Credentials are generated per session, with user `admin`
and a random password at `<session-dir>/etc/ze/.dev-password`, mode 0600.
A later seed preserves the existing store and does not rotate the credentials.

Test binaries live in a private `bin/` under the native functional action's
session scratch directory. `.ci` tests execute bare names and need an isolated
`etc/ze` (`internal/le/functional/binaries.go`, `internal/test/sessionpath`).

## Never Launch a Functional Suite By Running The Runner Binary

Running a raw `ze-test` binary is not equivalent to `./le functional <suite>`.
The native action builds the isolated tagged pair, sets `ZE_BIN` and
`ZE_TEST_BIN`, and owns the scratch environment. Bypassing it can produce a convincing false red.

`internal/le/functional/binaries.go` builds an isolated bare-named pair into the
session scratch directory. The daemon carries the test-only tag set, and the
suite runs with `ZE_TEST_NO_BUILD=1`, `ZE_BIN`, and `ZE_TEST_BIN` set to that
pair. A directly launched runner can rebuild a daemon without the test-only
surface, so a fixture can time out for a build-population error.

This is the same trap as bare `go test` above, one layer out: the invocation is
accepted and the failure looks like the code under test. `test/plugin/cos-external-warns.ci`
cost an hour of bisecting innocent changes this way; it passed in 2.0s through
the native action (`plan/learned/HOOK-FRICTION.md` F17).

| Want | Use |
|------|-----|
| A whole suite | `./le functional plugin` (or `encode`, `parse`, and the other actions listed by `./le functional list`) |
| One test, iterating | Use the owning compiled fixture's Go test, then rerun the complete `./le functional <suite>` action |
| A kernel-dependent suite in the VM | `./le qemu netns-test suites <comma-separated-suites>` |

The `--server` / `--client` hints the runner prints on failure inherit the same
gap: they re-run the same non-equivalent launch.

## The Bash Hook Matches Your Command Text, Including Search Patterns

`internal/le/hookruntime/bash.go` judges the command string. It cannot distinguish
a forbidden verb being executed from the same token appearing in a search
pattern, so a read-only search can be refused.

```
grep -l "git add -A\|git commit -a" tmp/commit-*.sh   # blocked: "git commit"
```

This is a false positive, not a rule violation, and it appears when auditing
commit scripts. Do not rephrase the ban away or work around the hook's intent.
Use the harness `Grep` tool instead of putting the banned verb in a Bash command.

Use `Grep` with pattern `add\s+(-A|--all|\.)|commit\s+-a` and path
`tmp/commit-*.sh`. The query never enters a Bash command line, and the result
still names every matching script.

Same class as the pipe ban above: the hook is coarse on purpose. The cost of
one extra round-trip is lower than the cost of a real bare `git commit`.

## No Fork Loops

### Bad

```bash
for f in test/plugin/*.ci; do grep -n 'pattern' "$f"; done       # 400 forks
for f in *.go; do grep -l 'Foo' "$f" | xargs sed -n '1p'; done  # 800 forks
```

### Good

```bash
grep -rn 'pattern' test/plugin/ --include='*.ci'                 # 1 fork
grep -n 'pattern' test/plugin/*.ci                                # 1 fork (glob)
```

### When a loop is unavoidable

If the loop body genuinely needs per-file logic that a single command cannot
express, batch with `xargs` or `find -exec +` instead of per-file forks:

```bash
find test/plugin -name '*.ci' -exec grep -l 'pattern' {} +
```

### Scope

Applies to every `Bash` tool call. First-party repository tooling belongs in
native Go packages and MUST NOT add a shell script.

## No Poll Loops

| Waiting for | Mechanism |
|-------------|-----------|
| A command this session launched in the background | Nothing. The completion notification is the wake-up |
| A file or a log line one of your own commands will produce | ONE bounded loop in `run_in_background`: `timeout 300 bash -c 'until [ -f <path> ]; do sleep 30; done'`. It notifies once, then it is gone |
| A repeated event (every ERROR line, every CI step) | The `Monitor` tool, with `persistent` left false so its `timeout_ms` deadline applies. `persistent: true` disables that deadline and rebuilds the problem this rule exists to stop |
| Another session's heavy job to free a slot | Do other work. `tmp/.ze-jobs/` holds one entry per running job, with its label, pid and log, and `./le verify status check` reports the last verify's verdict. `tmp/.ze-verify.lock.owner` is a copy of ONE entry, so read the directory when more than one job can run. Never a watcher |
| Nothing in particular | Do not wait at all |

## Lint Gate

### The Problem

The native per-edit hook in `internal/le/hookruntime/postwrite.go` judges changed
lines only. Cross-file effects can slip through: unused functions after
refactoring, import issues after renaming, and type mismatches across packages.
`./le verify worktree` catches these but takes minutes (see `ai/rules/testing.md`
for current timings).

### The Rule

Before claiming any Go implementation work is done, run:

```
./le verify lint run
```

**You MUST lint through `./le verify lint run`, never by calling
`golangci-lint` directly.** The native action derives the pinned toolchain and
every build flavor through `internal/le/verifylint`; a bare invocation inherits
host defaults and can report an environment failure as a code finding.

The same rule applies to every tool whose native action configures its
environment. The action is the interface; reaching past it drops the
configuration that makes the result representative.

This lints all packages with uncommitted Go changes, once for each BUILD, not
once. golangci-lint analyzes one GOOS, one GOARCH and one tag set for each run.
As a result, a file outside that build is not merely unchecked: the pass exits 0
and reads as clean over it.

The native action starts with the host build, then runs `GOOS=linux` with the
`integration` build tag. The second pass is the only one that reads a
`//go:build integration` file. On a non-Linux host it is also the only one that
reads a `//go:build linux` file. The rest come from
`internal/le/verifylint/matrix.go`, one for each personality tag (`ze_installer`,
`ze_distro`, `ze_appliance`, `ze_setup`), the capability tags, `tinygo`, and each
GOOS and GOARCH a tracked file names. Each flavor lints only the packages holding
a file the first two passes do not load. That package set is derived from the
tree on every run rather than written down.

One directory answers with the whole tree, on purpose. Every file in
`cmd/ze-installer` carries `//go:build linux && ze_installer`, so `go list` under
the unit tag set reports no package there. The change-set selector then has
nothing narrower to name (`internal/le/changed/scope.go`,
`uncompiledTreeReaders`). It widens to `./...`, and the wide answer is what makes
the `ze_installer` flavor run at all. A narrow answer would hand the driver a
scope the initrd's PID 1 is not in. The gate would then exit 0 over it.

Takes 3-10 seconds once the caches are warm, plus about 2 seconds for each flavor
whose packages the change reaches. The first run after a checkout pays a cold
analysis for each build, which is minutes.

Fix every issue it reports. Do not claim done with lint failures outstanding.

### When to run

| Moment | Action |
|--------|--------|
| After finishing all edits for a task | Run `./le verify lint run` |
| After fixing lint issues | Re-run to confirm clean |
| Before `/ze-commit` or `/ze-commit-check` | Already covered if you ran it above |

### What it catches that per-edit hooks miss

- Functions/variables made unused by refactoring another file
- Import cycles introduced by cross-package changes
- Type mismatches from interface changes
- Constants/vars that became unreferenced
- Package-level issues that only manifest with full package analysis

## Which Packages "Changed" Means

`./le changed scope` and `./le verify current mode changed` both scope to one
native answer, and `./le changed scope` prints it. The answer is
the changed packages plus two levels of their importers, and the feature tags the
change can reach.

```
./le changed scope print both
./le changed scope print packages paths-from FILE
```

A non-Go path seeds the Go packages whose tests read it, so a `.ci` or rule
point selects native tooling packages rather than nothing. The `paths-from`
keyword asks `./le changed scope` about a supplied path list.

**You MUST read the selector's stderr before you trust a scoped run.** It widens
to `./...` and names the reason whenever it cannot narrow, and one reason is
routine: `tmp/ze-verify.status` holding no green commit. With nothing proven,
every scoped target judges the whole tree until a full run passes. The contract
is `docs/architecture/testing/verify-freshness-scope.md`.

**A scoped run judges fewer Staticcheck matrix rows too.** `scopeFeatureMatrix` (`internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go`) keeps the two rows that omit no feature tag, plus one row per tag the change reached: 3 of 38 for a `ze_ssh`-local change. `all_features` and `core_only` judge the combinations Ze ships, and `validateScopedMatrix` refuses any scope that subtracts one of them.

**`./le staticcheck-feature-matrix check` typed on its own judges every row**, because only a verify run publishes the feature-tag answer that `ZE_VERIFY_SCOPE_TAGS` names. So does an answer that cannot be read, one naming a tag `feature-gates.txt` does not declare, and one naming every tag. An EMPTY answer is a real answer and judges the two shipped rows.

**Suite selection is not scoped: every functional suite runs on every verify, whatever the change set says.** `go list -deps ./cmd/ze` links 562 of the module's 646 packages, so no static signal attributes a `.ci` file to a Go package.

## Rationale

### Fork cost

On macOS, each `fork+exec` costs ~4-5 ms. A loop over 400 files x one `grep`
per iteration = ~2 seconds of pure fork overhead before any real work. Add a
second command per iteration (pipe to `sed`, call `awk`) and it doubles. Nested
loops make it quadratic.

### Poll cost

An abandoned poll loop keeps taking CPU after its answer is no longer needed.
That contention can make concurrent QEMU, Docker, and verification work fail.

The harm is not the fork cost measured above. It is the wake and its lifetime: a
poll loop keeps taking CPU on a loaded box long after anybody wants its answer.
