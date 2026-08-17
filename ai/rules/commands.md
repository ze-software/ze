# Running Commands

**When:** running any test, build, lint, or verification command from Bash, or writing a shell loop that forks or waits
**Severity:** blocking
**Related:** testing, platform-linux, git-safety

## Directives

- **Prefer `make` targets. A bare `go test` omits Ze's feature build tags and produces phantom reds in unrelated packages.**
- **Never pipe a test/build command through `head`/`tail`/`grep`/`awk`/`sed`/`cat` -- run clean, read the log.**
- **Never write a shell for-loop that forks an external command per iteration when a single invocation can process all inputs.**
- **Never poll for work you launched. A Bash command started with `run_in_background` re-invokes the session when it exits, so that notification IS the wait.**
- **A loop that watches the same command adds a process and reports nothing the notification does not already carry.**
- **Never write a `while` or `until` loop that calls `sleep`, and never put `pgrep` in a loop condition.**
- **A poll that is genuinely the only available signal MUST die on its own. Wrap it in `timeout <seconds>`.** An unbounded watcher outlives the reason it was started for, because the session that started it has moved on.
- **Stop a watcher the moment its reason changes.** `TaskStop` the background task. "It will end eventually" is how four of them come to tick at once.
- **One watcher at a time, and never faster than one wake every 30 seconds.** Each wake competes with QEMU, Docker and `ze-precommit-verify` for the same cores. That contention is what makes the functional suites flaky, so a watcher can corrupt the run it is watching.
- **Foreground `sleep` is blocked by the harness because waiting is not work.** Reaching for a loop to win the sleep back inverts that intent. Do other work, or end the turn.
- **The harness's own examples are unbounded, and this repo overrides them.** The Bash tool text prescribes an `until` loop when a foreground `sleep` is refused, and the `Monitor` schema shows `until grep -q ...; do sleep 0.5; done`. Both are refused here, and one word fixes both: `timeout`. The 30-second floor governs a watcher that spawns a process per wake (`pgrep`, `docker`, `curl`); a local file test inside a bound can be faster.
- **Run `make ze-lint-changed` before claiming any Go implementation work is done.**
- **Fix every issue it reports. Do not claim done with lint failures outstanding.**

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

- A known failing test MUST stay at the narrowest runnable scope until it passes. For Go tests, run `make ze-unit-pkg-test PKG=./path/to/package RUN='^TestName$' RACE=0`.
- Use `RACE=0` only for non-race iteration. A race or concurrency failure MUST keep race detection enabled.
- Run the required aggregate target, `make ze-precommit-verify` or `make ze-precommit-verify-changed`, only once. Run it after focused tests pass and all edits are complete. You MUST NOT use either aggregate target to rerun one known failure.

- During development, the session MUST start with a focused test sample for the changed code path before it runs a fuller, aggregate, or full suite.
- The sample MAY include the test being developed.
- When that sample finds a failing test, the fix loop MUST use the narrowest command that reproduces that failure.
- The narrow loop MUST NOT stop the session from running more focused sample tests when needed to debug, find the failure boundary, or remove a blocker.
- The fuller, aggregate, or full suite runs after the focused debugging loop no longer finds a relevant failure. It MUST NOT be the first probe.

## Bare `go test` Lies -- Always Pass The Feature Tags

`go test ./...` is **NOT** equivalent to `make ze-unit-test`. Ze compiles features
out behind build tags (`//go:build ze_isis`, `ze_ospf`, `ze_ldp`, `ze_rsvpte`,
`ze_web`, `ze_ssh`, ...). The Makefile always supplies them (`Makefile` reads
`ZE_FEATURES` from `feature-gates.txt` and sets
`GO_TEST_TAGS = ze_core $(ZE_FEATURES) $(ZE_TAGS)`). Omit them and those plugins
never register, so their validators, listeners and schema vanish and **unrelated
tests fail with phantom reds**.

**SHOULD prefer a make target** (`make ze-unit-test`, `make ze-precommit-verify-changed`). When you
MUST scope to packages, MUST pass the tags:

```
go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')" ./internal/component/foo/
```

Same for `git archive HEAD` scratch-tree checks: a bare run there reproduces your
own mistake and "confirms" a red that does not exist.

A bare `go test` omits feature tags and can produce a phantom red with a
plausible but false root cause. Use the make target so the result describes the
real build.

Symptom: a test asserting on something registered by another feature
(listeners, validators, plugin names, wire methods, schema) fails, and the
failure says a thing is *missing* or *not produced*. Check the tags before
believing it.

## No Pipes On Expensive Commands

Never pipe `make`, `go test`, `go build`, `golangci-lint`,
`bin/ze*`, or any test/verify/build command through `head`, `tail`,
`grep`, `awk`, `sed`, `cat`. Run clean. Read the log after.

**Exception:** `| tee <file>` MAY be used -- it is non-lossy and captures
output to a file while still displaying it.

Losing a failure line to `| head` means re-running the whole thing.
`make ze-precommit-verify*` writes to `tmp/ze-verify.log` (+ `-failures.log`
summary) by default. Override with `ZE_VERIFY_LOG=tmp/ze-verify-$$.log`
to avoid collisions between concurrent sessions. Read logs with the
Read tool, with `offset`/`limit` for paging.

## Write Ad-Hoc Scratch Under Your Per-Session Dir

`tmp/` is shared by every concurrent session in this checkout (it is keyed
per-checkout, not per-session -- `scripts/dev/ensure-links.py`). A fixed name at
the `tmp/` root -- `tmp/out.log`, `tmp/stdout`, `tmp/gotest.log` -- collides with
a sibling session writing the same name, and is never cleaned when your session
ends.

**A file at the `tmp/` root is REFUSED, on both surfaces that create one**:
`check_scratch_path` on a Bash redirect or `tee`, `c_scratch_path_we` on Write
and Edit. Both call `.claude/hooks/lib/scratch_path.py`, so the two surfaces
answer alike. A path carrying a directory component passes, which covers
`tmp/session/<YYYY-MM-DD>-<sid>/` and every producer's own folder. The root
names that are session-keyed or shared by design pass too:
`ze-precommit-verify*`, `.ze-verify*`, `commit-*`, `commit-msg-*`, `delete-*`,
`mutation*`, `test-timings*`.

Write ad-hoc scratch under this session's private directory instead:

```
dir=$(scripts/dev/session-scratch.sh)          # <session-dir>/scratch/, created for you
make ze-unit-test-changed > "$dir/unit.log" 2>&1
```

**Nothing under `tmp/session/` is ever deleted automatically**: not at session
end, not on an age timer, not by a hook. Your directory outlives your session,
so a log you wrote is still there tomorrow. Cleanup is the operator's:
`make ze-sessions-clean BEFORE=<YYYY-MM-DD>` removes the session directories
dated strictly before that date, and `make clean` removes your own. Do NOT
relocate artifacts that are already session-keyed (commit scripts
`tmp/commit-<sid>.sh`) or shared by design (`tmp/ze-verify.*`, the durable
`cache/`) -- those stay put. `GOCACHE` is `cache/go-cache` (`Makefile`), on the
durable side.

## Your Binaries Live In This Session's Directory -- Ask For The Path

Under an AI session every canonical binary is built into this session's own
directory, under its BARE name:
`tmp/session/<YYYY-MM-DD>-<session-id>/bin/ze` (`mk/session.mk`). A sibling
session's `make ze-build` therefore cannot overwrite the binary you are testing
against. Off-session (a human shell, CI) the path is the plain `bin/ze` it
always was.

**MUST NOT hardcode `bin/ze`** in a command, script, or doc. Ask:

```
$(make ze-session-binary-path) show version          # <session-dir>/bin/ze, or bin/ze off-session
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

That session-local `etc/ze` is SEEDED, once, by the first ze_core binary this
session builds (`scripts/dev/session-seed-store.sh`, called from the `ze`,
`ze-appliance-build` and `ze-stripped-build` recipes -- the three that link
`internal/plugins/init` and the silent `NewBlob` path). An
unseeded store is not red: `NewBlob`
(`internal/component/config/storage/blob.go`) creates the blob and returns a nil
error when it is absent, so `ze` would start with no users and a fresh SSH host
key rather than fail. The credentials are generated per session -- user `admin`,
and a random password at `<session-dir>/etc/ze/.dev-password`, mode 0600 under a
gitignored root -- so nothing is tracked and two sessions never share one. A
second `make ze-build` reseeds nothing and rotates nothing.

Test binaries take the same shape one level in -- a private `bin/` subdir of a
throwaway directory under `$(ZE_SCRATCH_DIR)` -- because `.ci` tests exec them by
bare name and an isolated `etc/ze` is what a test wants
(`mk/test-functional.mk`, `internal/test/sessionpath`).

## Never Launch a Functional Suite By Running The Runner Binary

Running this session's `ze-test` binary yourself (`$(ZEBIN_TEST) bgp plugin 145`)
is **not** equivalent to `make ze-functional-plugin-test`, and the difference produces a
convincing false red.

`mk/test-functional.mk` builds an ISOLATED, BARE-NAMED pair into
`$(ZE_ALT_BIN)`, and the daemon it builds carries the **`zetest`** build tag
(`ze_core ze_distro ze_setup zetest $(ZE_FEATURES)`). That same makefile then runs
the suite as `env ZE_TEST_NO_BUILD=1 ZE_BIN=$(ZE_ALT_BIN)/ze ZE_TEST_BIN=$(ZE_ALT_BIN)/ze-test
$(ZE_ALT_BIN)/ze-test ...`. Launched directly the runner rebuilds a ze WITHOUT
`zetest`, so a test needing a zetest-only surface times out as
`server likely failed to start or crashed` -- naming none of this.

This is the same trap as bare `go test` above, one layer out: the invocation is
accepted and the failure looks like the code under test. `test/plugin/cos-external-warns.ci`
cost an hour of bisecting innocent changes this way; it passed in 2.0s under the
make target (`plan/learned/HOOK-FRICTION.md` F17).

| Want | Use |
|------|-----|
| A whole suite | `make ze-functional-plugin-test` (or `ze-functional-encode-test`, `ze-functional-parse-test`, ...) |
| One test, iterating | the make target's own invocation: build the isolated pair with its tags, symlink them bare-named, export `ZE_BIN`/`ZE_TEST_BIN` |
| One test in the VM | `make ze-qemu-debug RUN='...'` -- flags BEFORE positional ids (`-v 145`, not `145 -v`) |

The `--server` / `--client` hints the runner prints on failure inherit the same
gap: they re-run the same non-equivalent launch.

## The Bash Hook Matches Your Command Text, Including Search Patterns

`.claude/hooks/pretool-bash.py` blocks the banned git verbs by matching the
command STRING. It cannot tell a verb you are running from a verb you are
searching for, so a read-only grep is rejected when its own pattern spells
one:

```
grep -l "git add -A\|git commit -a" tmp/commit-*.sh   # blocked: "git commit"
```

This is a false positive, not a rule you are violating, and it bites exactly
when auditing commit scripts, which is what the shared-file sharding work spent
its time doing.
Do not rephrase the ban away or work around the hook's intent. Scan with
Python instead, which keeps the verb out of the command line:

```
python3 - <<'PY'
import glob, re
broad = re.compile(r"add\s+(-A|--all|\.)|commit\s+-a")
for s in glob.glob("tmp/commit-*.sh"):
    if broad.search(open(s, errors="replace").read()):
        print("BROAD-STAGE", s)
PY
```

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

Applies to every `Bash` tool call and every shell script written for this
project.

## No Poll Loops

| Waiting for | Mechanism |
|-------------|-----------|
| A command this session launched in the background | Nothing. The completion notification is the wake-up |
| A file or a log line one of your own commands will produce | ONE bounded loop in `run_in_background`: `timeout 300 bash -c 'until [ -f <path> ]; do sleep 30; done'`. It notifies once, then it is gone |
| A repeated event (every ERROR line, every CI step) | The `Monitor` tool, with `persistent` left false so its `timeout_ms` deadline applies. `persistent: true` disables that deadline and rebuilds the problem this rule exists to stop |
| Another session's `ze-precommit-verify` to release the lock | Do other work. `tmp/.ze-verify.lock.owner` names the holder, and `scripts/dev/verify-status.sh check` reports the last run's verdict. Never a watcher |
| Nothing in particular | Do not wait at all |

## Lint Gate

### The Problem

The per-edit hook (`auto-lint` in `.claude/hooks/posttool-writeedit.py`) uses
`--new-from-rev=HEAD`, which only catches issues on lines changed since the last
commit. Cross-file effects slip through: unused functions after refactoring,
import issues from renaming, type mismatches across package boundaries.
`make ze-precommit-verify` catches these but takes minutes (see `ai/rules/testing.md`
for current timings).

### The Rule

Before claiming any Go implementation work is done, run:

```
make ze-lint-changed
```

This lints all packages with uncommitted Go changes, TWICE: once for the host
build, then again under `GOOS=linux` with the `integration` build tag. The second
pass is the only thing that reads a `//go:build integration` file, and on a
non-Linux host it is the only thing that reads a `//go:build linux` file. Takes
3-10 seconds once both caches are warm; the first run after a checkout pays a
cold `GOOS=linux` analysis, which is minutes.

Fix every issue it reports. Do not claim done with lint failures outstanding.

### When to run

| Moment | Action |
|--------|--------|
| After finishing all edits for a task | Run `make ze-lint-changed` |
| After fixing lint issues | Re-run to confirm clean |
| Before `/ze-commit` or `/ze-commit-check` | Already covered if you ran it above |

### What it catches that per-edit hooks miss

- Functions/variables made unused by refactoring another file
- Import cycles introduced by cross-package changes
- Type mismatches from interface changes
- Constants/vars that became unreferenced
- Package-level issues that only manifest with full package analysis

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
