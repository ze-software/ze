# Running Test / Build Commands

**BLOCKING:** Prefer `make` targets. A bare `go test` omits Ze's feature build
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

**This has cost real time.** On 2026-07-15 two `plan/known-failures.md` entries
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

**BLOCKING:** Never pipe `make`, `go test`, `go build`, `golangci-lint`,
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
design (`tmp/go-cache`, `tmp/ze-verify.*`, the durable `cache/`) -- those stay put.

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
`plan/spec-fixit-shared-plan-file-contention.md`, whose research does this).
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
