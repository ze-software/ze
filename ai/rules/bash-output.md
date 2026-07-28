# Running Test / Build Commands

**When:** running any test, build, lint, or verification command from Bash
**Severity:** blocking

## Directives

Prefer `make` targets. A bare `go test` omits Ze's feature build
tags and produces phantom reds in unrelated packages. Never pipe a test/build
command through `head`/`tail`/`grep`/`awk`/`sed`/`cat` -- run clean, read the log.

## Bare `go test` Lies -- Always Pass The Feature Tags

`go test ./...` is **NOT** equivalent to `make ze-unit-test`. Ze compiles features
out behind build tags (`//go:build ze_isis`, `ze_ospf`, `ze_ldp`, `ze_rsvpte`,
`ze_web`, `ze_ssh`, ...). The Makefile always supplies them (`Makefile:51`
`ZE_FEATURES` read from `feature-gates.txt`; `Makefile:65`
`GO_TEST_TAGS = ze_core $(ZE_FEATURES) $(ZE_TAGS)`). Omit them and those plugins
never register, so their validators, listeners and schema vanish and **unrelated
tests fail with phantom reds**.

**Prefer a make target** (`make ze-unit-test`, `make ze-verify-changed`). When you
must scope to packages, pass the tags:

```
go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')" ./internal/component/foo/
```

Same for `git archive HEAD` scratch-tree checks: a bare run there reproduces your
own mistake and "confirms" a red that does not exist.

**This has cost real time.** On 2026-07-15 two `plan/known-failures/` entries
(7 tests) were disproven as pure tags artifacts. Both had been logged with a
confident but wrong root cause (a "macOS socket-stack quirk"; a "broken
listener-conflict validator"), and one was "re-confirmed" six days later by
repeating the same flawed invocation. A phantom red is worse than a real one: it
sends the next session hunting a bug that was never there.

Symptom: a test asserting on something registered by another feature
(listeners, validators, plugin names, wire methods, schema) fails, and the
failure says a thing is *missing* or *not produced*. Check the tags before
believing it.

## No Pipes On Expensive Commands

Never pipe `make`, `go test`, `go build`, `golangci-lint`,
`bin/ze*`, or any test/verify/build command through `head`, `tail`,
`grep`, `awk`, `sed`, `cat`. Run clean. Read the log after.

**Exception:** `| tee <file>` is allowed -- it is non-lossy and captures
output to a file while still displaying it.

Losing a failure line to `| head` means re-running the whole thing.
`make ze-verify*` writes to `tmp/ze-verify.log` (+ `-failures.log`
summary) by default. Override with `ZE_VERIFY_LOG=tmp/ze-verify-$$.log`
to avoid collisions between concurrent sessions. Read logs with the
Read tool, with `offset`/`limit` for paging.

## Write Ad-Hoc Scratch Under Your Per-Session Dir

`tmp/` is shared by every concurrent session in this checkout (it is keyed
per-checkout, not per-session -- `scripts/dev/ensure-links.py`). A fixed name at
the `tmp/` root -- `tmp/out.log`, `tmp/stdout`, `tmp/gotest.log` -- collides with
a sibling session writing the same name, and is never cleaned when your session
ends.

Write ad-hoc scratch under this session's private directory instead:

```
dir=$(scripts/dev/session-scratch.sh)          # tmp/s/<session-id>/, created for you
make ze-unit-test-changed > "$dir/unit.log" 2>&1
```

The whole directory is removed at session end (`.claude/hooks/session-end-scratch.sh`,
with a 24h backstop in `session-start.sh`), so your scratch is self-contained and
disposable. Do NOT relocate artifacts that are already session-keyed (commit
scripts `tmp/commit-<sid>.sh`, session state `tmp/session/*-<SID>*`) or shared by
design (`tmp/ze-verify.*`, the durable `cache/`) -- those stay put. `GOCACHE` is
`cache/go-cache` (`Makefile:17`), on the durable side.

## Your Binaries Are Session-Suffixed -- Ask For The Path

Under an AI session every canonical binary is built as `bin/<name>-<session-id>`
(`mk/session.mk`), so a sibling session's `make ze` cannot overwrite the binary
you are testing against. Off-session (a human shell, CI) the name is the plain
`bin/ze` it always was.

**Do not hardcode `bin/ze`** in a command, script, or doc. Ask:

```
$(make ze-path) show version          # bin/ze-<session-id>, or bin/ze off-session
```

The suffixed binaries stay in `bin/` on purpose: a binary's location decides where
`ze` resolves its config and database (`internal/core/paths/paths.go`
`ConfigDirFromBinary`), so moving them under `tmp/s/<id>/` would repoint the daemon
away from the repository's live `etc/ze`. They are swept by name at session end
instead (`scripts/dev/session-scratch.sh` `reap_binaries`).

Test binaries take the opposite trade-off -- a private `bin/` subdir under
`tmp/s/<id>/` -- because `.ci` tests exec them by bare name and an isolated
`etc/ze` is what a test wants (`mk/test-functional.mk`, `internal/test/sessionpath`).

## Never Launch a Functional Suite By Running The Runner Binary

`bin/ze-test-<id> bgp plugin 145` is **not** equivalent to `make ze-plugin-test`,
and the difference produces a convincing false red.

`mk/test-functional.mk:140` builds an ISOLATED, BARE-NAMED pair into
`$(ZE_ALT_BIN)`, and the daemon it builds carries the **`zetest`** build tag
(`ze_core ze_distro ze_setup zetest $(ZE_FEATURES)`). Line 145 then runs the suite
as `env ZE_TEST_NO_BUILD=1 ZE_BIN=$(ZE_ALT_BIN)/ze ZE_TEST_BIN=$(ZE_ALT_BIN)/ze-test
$(ZE_ALT_BIN)/ze-test ...`. Launched directly the runner rebuilds a ze WITHOUT
`zetest`, so a test needing a zetest-only surface times out as
`server likely failed to start or crashed` -- naming none of this.

This is the same trap as bare `go test` above, one layer out: the invocation is
accepted and the failure looks like the code under test. `test/plugin/cos-external-warns.ci`
cost an hour of bisecting innocent changes this way; it passed in 2.0s under the
make target (`plan/learned/HOOK-FRICTION.md` F17).

| Want | Use |
|------|-----|
| A whole suite | `make ze-plugin-test` (or `ze-encode-test`, `ze-parse-test`, ...) |
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
when auditing commit scripts (see
`plan/learned/1244-fixit-shared-plan-file-contention.md`, the sharding work
that hits this).
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
