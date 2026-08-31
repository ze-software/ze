# Running Development Commands

<!-- source: internal/le/gotoolchain, internal/le/changed, internal/le/functional, internal/le/session, internal/le/job, internal/le/hookruntime -->

How the `./le` action surface, the session scratch tree, and the Bash guard
behave. The obligations that follow from this page are `ai/rules/commands.md`.

## A bare `go test` is not `./le test-unit`

Ze compiles features out behind build tags (`//go:build ze_isis`, `ze_ospf`,
`ze_ldp`, `ze_rsvpte`, `ze_web`, `ze_ssh`, and the rest). `internal/le/gotoolchain`
derives the feature set from `feature-gates.txt` and passes it to every native
unit and verification action. Omit those tags and the plugins never register, so
their validators, listeners and schema vanish, and unrelated tests fail.

The failure is a phantom red with a plausible but false cause. Its symptom is a
test asserting on something another feature registers (a listener, a validator, a
plugin name, a wire method, a schema entry) failing with a message that a thing is
*missing* or *not produced*. Check the tags before believing it.

A `git archive HEAD` scratch tree carries the same trap: a bare run there
reproduces the same mistake and confirms a red that does not exist.

To scope a run to one package and keep the tags, read the tag list out of
`feature-gates.txt` and pass it with `-tags`, prefixed by `ze_core`:

```
tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
```

## A functional suite is not the runner binary

`./le functional <suite>` builds an isolated bare-named binary pair into the
session scratch directory (`internal/le/functional/binaries.go`). The daemon
carries the test-only tag set, and the suite runs with `ZE_TEST_NO_BUILD=1`,
`ZE_BIN` and `ZE_TEST_BIN` pointing at that pair (`BinarySet.Environment`).

Running a `ze-test` binary directly skips all of that. The runner can then
rebuild a daemon without the test-only surface, so a fixture times out for a
build-population reason and the failure looks like the code under test. The
`--server` and `--client` hints the runner prints on failure inherit the same
gap: they re-run the same non-equivalent launch.

| Want | Use |
|------|-----|
| A whole suite | `./le functional plugin` (`./le functional list` names every suite) |
| One test, iterating | The owning compiled fixture's Go test, then rerun the whole `./le functional <suite>` |
| A kernel-dependent suite in the VM | `./le qemu netns-test suites <comma-separated-suites>` |

## The lint gate

The native per-edit hook (`internal/le/hookruntime/postwrite.go`) judges changed
lines only. Package-wide analysis is what finds the rest:

- functions and variables left unused by a refactor in another file
- import cycles introduced by a cross-package change
- type mismatches from an interface change
- constants and vars that became unreferenced

`./le verify lint run` lints every package holding an uncommitted Go change, once
for each BUILD rather than once. golangci-lint analyzes one GOOS, one GOARCH and
one tag set for each run, so a file outside that build is not merely unchecked:
the pass exits 0 and reads as clean over it. The flavor matrix that closes that
hole is `testing.md`, "The builds the linter reads"; the rows themselves are
`flavorMatrix` in `internal/le/verify/lint/matrix.go`.

Cost: 3 to 10 seconds once the caches are warm, plus about 2 seconds for each
flavor whose packages the change reaches. The first run after a checkout pays a
cold analysis for each build, which is minutes.

## The changed-set selector

`./le changed scope` is the one selector, and `./le verify current mode changed`
reuses its answer. It reports the changed packages plus two levels of their
importers (`defaultDepth`, `internal/le/changed/selector.go`), and the feature
tags the change can reach.

```
./le changed scope print both
./le changed scope print packages paths-from FILE
```

A non-Go path seeds the Go packages whose tests read it, so a `.ci` file or a
rule point selects the native tooling packages rather than nothing.

Every route that fails to narrow WIDENS to `./...` and names its reason on stderr
(`widen`, `internal/le/changed/scope.go`). One reason is routine:
`tmp/ze-verify.status` holding no green commit. With nothing proven, every scoped
target judges the whole tree until a full run passes. The contract is
`../architecture/testing/verify-freshness-scope.md`.

One directory answers with the whole tree by design. Every file in
`cmd/ze-installer` carries `//go:build linux && ze_installer`, so `go list` under
the unit tag set reports no package there, and `seedPackages`
(`internal/le/changed/selector.go`) has nothing narrower to name. The wide answer
is what makes the `ze_installer` lint flavor run at all.

A scoped run also judges fewer Staticcheck feature-matrix rows. `scopeMatrix`
(`internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go`) keeps the two
rows that omit no feature tag, plus one row per tag the change reached: 3 of 38
for a `ze_ssh`-local change, with 36 feature tags declared in `feature-gates.txt`.
Those two rows are `all_features` and `core_only`, and `validateScoped` refuses
any scope that subtracts one of them. `./le staticcheck-feature-matrix check`
typed on its own judges every row, because only a verify run publishes the
feature-tag answer that `ZE_VERIFY_SCOPE_TAGS` (`ScopeTagsKey`) names.

Suite selection is not scoped: every functional suite runs on every verify,
whatever the change set says. `go list -deps ./cmd/ze` links most of the module,
so no static signal attributes a `.ci` file to a Go package.

## Scratch files

`tmp/` is keyed per CHECKOUT, not per session (`internal/le/scratch/scratch.go`),
so every concurrent session in the tree shares it. A fixed name at the `tmp/` root
(`tmp/out.log`, `tmp/stdout`, `tmp/gotest.log`) collides with a sibling session
writing the same name, and nothing removes it when either session ends.

A file written directly at the `tmp/` root is refused on both surfaces that create
one: `bashScratch` in `internal/le/hookruntime/bash.go` and the Write/Edit path
check in `internal/le/hookruntime/writeedit.go` both call `isAdHocScratch`
(`internal/le/hookruntime/runtime.go`). A path carrying a directory component
passes, and so do the root names that are shared by design: `ze-verify*`,
`commit-*`, `delete-*`, `mutation*`, `test-timings*`.

```
dir=$(./le session scratch ensure)          # <session-dir>/scratch/, created for you
./le test-unit > "$dir/unit.log" 2>&1
```

Nothing under `tmp/session/` is deleted automatically: not at session end, not on
an age timer, not by a hook. The directory outlives the session, so a log written
today is there tomorrow. `./le session reap` removes only session directories
whose owners are provably gone. Artifacts that are already session-keyed, and the
shared-by-design ones (`tmp/ze-verify.*`, and the durable Go build cache
`internal/le/gotoolchain` assigns), stay where they are.

## When the disk is full

A full cache disk has been read as a code defect four times
(`plan/journal/full-disk-false-red.md`). It arrives as a wave of unrelated
failures:

- Packages that do not import each other fail to build.
- The linker says `mapping output file failed: no space left on device`.
- A verification stage reports `cache entry not found`.
- A whole functional suite goes red at once.

`df` on the checkout answers about the wrong device. `cache/` is a symlink to
`$XDG_CACHE_HOME/ze`, or to `~/.cache/ze` (`internal/le/scratch/scratch.go`,
`cacheTarget`), and that target is frequently its own filesystem. Read the device
that holds the cache:

```
stat -f cache/go-cache        # blocks available on the cache device
findmnt -T cache/go-cache     # Linux: which device that path is on
```

Two Go build caches fill, on two filesystems, and emptying one leaves the other
full. `./le scratch cache-clean` empties both and prints what each one returned:

```
$ ./le scratch cache-clean
checkout /Users/thomas/Unix/cache/ze/go-cache    freed 256.0G, free 34.2G
ambient  /Users/thomas/Library/Caches/go-build   freed 1.2G, free 34.2G
```

The CHECKOUT cache is `cache/go-cache`. Every le action writes it, because
`Overrides` (`internal/le/gotoolchain/gotoolchain.go`) points GOCACHE there, and
`gotoolchain.GoCache` names it. The AMBIENT cache is the one a bare `go build`
writes outside le. The action asks `go env GOCACHE` for that path with the
inherited override removed. The two rows are therefore the two real caches,
never one cache twice. The equivalent by hand is:

```
go clean -cache                                   # the ambient cache
env GOCACHE="$PWD/cache/go-cache" go clean -cache  # the checkout cache
```

Nothing caps either cache, so both grow until the disk fills. The clean costs
recompilation, which makes the next run slow once.

## Session binaries

Native test actions build their binaries inside the current session's private
directory, under bare names, so a sibling session cannot overwrite the binary
under test. Ask the owning action for the path rather than writing `bin/ze`.

The directory carries the session id, so the file name does not. That is what
keeps argv[0] personality dispatch working (`binarySuffixRoot`,
`cmd/ze/dispatch.go`, reads the segment after the last `-`) and lets a `.ci` test
exec `ze` by bare name off one PATH entry. A binary's location also decides where
`ze` resolves its config and database (`ConfigDirFromBinary`,
`internal/core/paths/paths.go`), so a session's `ze` reads `<session-dir>/etc/ze`
and the repository's `etc/ze` belongs to the human alone.
`internal/test/sessionpath` is what a `.ci` test uses to find it.

That directory is LOOKED UP, never recomputed: every consumer takes the single
directory matching `tmp/session/????-??-??-<id>`, and names a new one with today's
date only on a miss. Recomputing from today's date would move a session's
directory at midnight and orphan the binaries it is running.

The session-local `etc/ze` is seeded once by
`./le session seed-store binary <session-bin>/ze`. `SeedStore`
(`internal/le/session/seed.go`) validates that the binary belongs to the current
session directory. Credentials are generated per session: user `admin`, and a
random password at `<session-dir>/etc/ze/.dev-password`, mode 0600. A later seed
preserves the existing store and does not rotate the credentials.

## Verify logs

Each verify run writes its own directory under `tmp/verify/<mode>-<random>/`
(`internal/le/verify/engine/run.go`), so concurrent runs never collide. It holds
one log per stage plus the combined `ze-verify.log`. The latest run is also
published at four stable paths (`internal/le/verify/engine/artifacts.go`):

| Path | Holds |
|------|-------|
| `tmp/ze-verify.log` | the combined log of the latest run |
| `tmp/ze-verify-failures.log` | the human failure index |
| `tmp/ze-verify-failures.json` | the machine failure index |
| `tmp/ze-verify-full.json` | the latest FULL-mode machine index, kept when a changed run publishes a cheaper result |

Piping such a run through `head` or `grep` loses the failure line and costs a
re-run. Run it clean, then read the log with paging.

## Why one owner runs the suites

The reason is attribution, not speed and not memory. Suites share the build
cache, the TCP ports, and the `ze` processes they start. A run taken while
another suite is running therefore produces a red that belongs to nobody: a
killed process and a real defect read the same in a log. The repository-wide
verify lock says this for one target; the one-owner rule says it for every suite
and names who holds the right to run one.

## How long a verify takes, and when a slow one is broken

Never take a timeout from a duration written in a rule. How long a full pass takes
depends on the machine and on what else that machine is doing, so the figures in
two different rules are different hardware rather than a contradiction. `ticket.Release`
(`internal/le/job/job.go`) appends the real elapsed seconds to
`tmp/.ze-verify-duration.txt` for the machine you are on, and `tmp/` is gitignored,
so that file is the only per-machine record there is. Read it as an expectation,
never as a threshold.

A slow run is not a broken run, and there is no threshold to raise. A waiter breaks
a holder's slot only when that holder is DEAD, or when it has made no progress for
the stall window: `scanAndClaim` (`internal/le/job/registry.go`) judges progress by
the mtime of the job's log, never by elapsed time. `ZE_JOB_STALL_SECONDS`
(`StallKey`) sets that window, defaults to 1800 seconds, and is bounded to 60..3600
(`StallMin`, `StallMax`, `internal/le/job/job.go`). A value outside the range is
refused before the job starts.

## One verify at a time

Parallel verify runs share the build cache, the ports, and the test binaries. Every
heavy native action is admitted through `./le job run`, which runs a job now, queues
it behind the jobs already in flight, or attaches it to an equivalent run, so a
second verify blocks rather than overlapping. `ZE_RUN_SLOTS` (`SlotsKey`) sets the
slot count and defaults to 1 (`SlotsDefault`); `internal/le/gotoolchain` derives the
per-process `GOMAXPROCS` ceiling.

Admission state is one file per running job, `tmp/.ze-jobs/<label>.<pid>.job`. There
is no `tmp/.ze-verify.lock`: nothing takes that flock. The only flock in the
registry is held for the length of one scan (`registryLock`,
`internal/le/job/registry.go`). A job started INSIDE another job's slot runs straight
through instead of queueing behind its own parent, which is how every stage of a
verify run runs.

## Testing one package while you develop it

`./le job run label unit-pkg command <argv...>` is the supported route. It takes
the admission slot, so one heavy job runs while the peers queue, and it tees the
child's merged output to the job log.

Everything after the `command` keyword is the child's argv, passed through
unchanged (`parseRun`, `internal/le/job/answer.go`; `Admission.Run`,
`internal/le/job/job.go`). The command adds no build tags, no `-race`, no
package pattern and no timeout of its own, so write each of them yourself. The
`PKG=` and `RUN=` spellings belong to `./le fuzz`, which declares them as
argument aliases; `go test` reads `PKG=./x` as an import path and refuses it.

Carry the feature tags from the recipe at the top of this page, or the run
judges a tree in which no gated plugin registers.

```
tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
./le job run label unit-pkg command go test -race -tags "$tags" ./internal/component/ike/eap
./le job run label unit-pkg command go test -race -tags "$tags" -run TestEAPTLS ./internal/component/ike/...
```

Drop `-race` while iterating if you must. A package tested without `-race` has
not been tested the way the gate tests it, so put it back before the end.

## Which native action owns the documentation gate

`./le doc check verify` and `./le repository generated-check` are separate actions.
`internal/le/doc/wiring.Verify` owns the ordered documentation gate, including the
`internal/le/docvalid` command and drift checks, the `internal/le/doc/check` links,
and RFC freshness. `internal/le/repository` owns the generated repository artifacts.

## Waiting for another session's job

`tmp/.ze-jobs/<label>.<pid>.job` holds one entry per running job, with its label,
pid and log (`JobsDir`, `internal/le/job/job.go`). It is a registry, not a lock.
`tmp/.ze-verify.lock.owner` (`OwnerFile`) is a copy of ONE entry, so read the
directory when more than one job can run. `./le verify status check` reports the
last verify's verdict.

Raw heavy work that no registered action owns is admitted with
`./le job run label <label> command <argv...>`. One job runs and its peers queue
or attach; the child's exit status is preserved, so the command inside remains the
command being judged. A cheap subcommand of a heavy tool needs no admission:
`golangci-lint config verify` runs no analysis. A one-off that must not queue
states its reason in the command, as `ZE_ADMIT_RAW="<reason>"`. An empty reason
admits nothing, and the reason that is there lands in the transcript, which is
what makes the escape auditable by reading the session.

## The Bash guard matches your command text

`internal/le/hookruntime/bash.go` judges the command STRING. It reads a command
POSITION rather than a substring, so a verb at the start of the command or after a
separator is a run, and a verb inside quotes is prose: a search pattern, an echo,
or a commit message explaining the rule. `gitVerbRun` decides it, and the quote
exemption is withdrawn for a command that hands a string to another shell to run
(`bash -c`, `eval`), which is the one place quotes open a command position.

Git's pre-verb options are stripped before the comparison (`gitInvocation`), so
`git -C /other/tree commit` and `git -c commit.gpgsign=false commit` are refused
exactly as the bare verb is.

Two guards read the result. `bashDestructiveGit` refuses the verbs that discard
work or publish it, staging included, and names `./le commit create` as the route
that works. `bashBranchMove` refuses the verbs that create, switch, rename, delete
or integrate a branch, and says the branch is the user's to move; `git branch`
with no mutating flag is a read and passes.

The guard is still coarse where quoting cannot help: an unquoted verb in a
pipeline reads as a run. One extra round-trip costs less than one real bare
staging or commit verb in a shared checkout. Running the scan through the harness
`Grep` tool avoids the question, because the query never enters a command line.

The same guard refuses a write to `plan/` or `ai/rules/` from Bash
(`bashGovernedWrite`), because the Write and Edit tools are where the document
checks in `internal/le/hookruntime/writeedit.go` run. The interpreter tier
over-matches on purpose, so a heredoc that merely NAMES those trees beside a write
primitive is refused too. A wrong refusal is answered with
`ZE_ADMIT_GOVERNED_WRITE="<reason>"`, never by rewording the command. Reading
stays free: `grep`, `cat`, `sed -n` and `./le commit create file plan/spec-x.md
dry-run` bind on the write, not on the path.

## What a fork and a poll cost

On macOS each `fork+exec` costs about 4 to 5 ms. A loop over 400 files running one
`grep` per iteration spends about 2 seconds on fork overhead before any real work.
A second command per iteration doubles it, and a nested loop makes it quadratic.
One recursive `grep`, one glob, or one `find -exec +` is a single fork.

A poll loop's cost is not the fork. It is the wake and its lifetime: an abandoned
loop keeps taking CPU long after anybody wants its answer, and that contention is
what makes concurrent QEMU, Docker and verification work fail.

| Waiting for | Mechanism |
|-------------|-----------|
| A command this session launched in the background | Nothing. The completion notification is the wake-up |
| A file or log line one of your own commands will produce | ONE bounded loop in the background, wrapped in `timeout`. It notifies once, then it is gone |
| A repeated event (every ERROR line, every CI step) | A monitor with a deadline, never a persistent one |
| Another session's heavy job to free a slot | Do other work, and read `tmp/.ze-jobs/`. Never a watcher |
| Nothing in particular | Do not wait at all |
