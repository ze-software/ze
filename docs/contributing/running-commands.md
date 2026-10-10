# Running Development Commands

<!-- source: internal/le/go/toolchain, internal/le/repo/changed, internal/le/test/functional, internal/le/session, internal/le/job, internal/le/hookruntime, internal/le/verify, internal/le/scratch -->

How the `./le` action surface, the session scratch tree, and the Bash guard
behave. The obligations that follow from this page are `ai/rules/commands.md`.

## Finding a command and what it takes

`./le '|' json` answers le's whole surface as one document. It names every
registered area, the group it renders under, and its description. An area that
registered an action table also carries each action, with its verb, its purpose,
its write flag and its keyword grammar. The bar is quoted because a shell
consumes an unquoted one. `| yaml` and `| table` render the same payload.

`./le` with no argument prints that manifest as the root help page. It writes to
stdout and exits 0. It wrote to stderr and exited 1 until 2026-09-12, so a
caller that reads stdout or tests the status now receives the page.

`./le <area> <verb> --help` prints one action's usage line. The dispatcher
renders it from the registered action table and calls no handler. The help word
therefore starts no work, whatever the area does with its first argument. A
required keyword prints bare, an optional one prints inside brackets, and a
keyword that repeats carries a trailing ellipsis.

The bare word `help` that a declared keyword introduced is that keyword's VALUE,
and the action runs. `./le repo rewrite replace file <path> old beta new help`
replaces `beta` with the word `help`. The registered table tells a value from a
question, so the two never collide in an area that declares one.

A value never begins with a dash. le follows GNU option syntax, and
`ai/rules/cli.md` declares le's whole option set: `--help`, `-h`, `--version`
and `-V`. A dash opens a long option, or a cluster of one-letter short options.
`-help` is therefore not the word `help` with one dash. It is `-h` followed by
`-e`, `-l` and `-p`: a help request, then three options le does not declare.

An action that dispatches through its action table refuses a dash-leading word
in every value slot, and answers 2. `./le verify status check path --help path
internal` refuses and reads no path. `./le repo rewrite replace file <path>
old beta new --help apply` refuses and writes nothing. Only the bare word,
which carries no dash, can be data.

Four spellings ask for help, and the set is closed: `help`, `--help`, `-h` and
`-help`. A trailing one of the four asks the question in every area, so
`./le test stress-repro run suite -help` prints a page and starts no burn.

Every other option travels on. An area with a table refuses it. An area
without one hands it to the child, so `./le job run label x command echo
-html=cover.out` prints `-html=cover.out`.

A shape rule cannot replace the four words. A table-less area publishes no
grammar, so le cannot tell its own option from one you forward to a child.

<!-- source: internal/le/le/root/manifest.go -- Manifest -->
<!-- source: internal/le/le/root/dispatch.go -- Dispatch, helpTrailing -->
<!-- source: internal/le/le/action/leaction.go -- parameterForm, parseArguments -->

An area that hand-rolls its own dispatch registers no table. The manifest then
carries its description and not its grammar, and `--help` answers its node page.
A trailing help word is read as a question there, because the dispatcher has no
grammar to read it against. A value spelled `help`, `--help`, `-h` or `-help` is
therefore unreachable as the last word of such an area's line.
`internal/le/actions_test.go` names those areas and refuses a new one.

Any OTHER option reaches such an area, because the dispatcher cannot read it as
a question and the area's own parser owns the line. That is what `command
<argv...>` means: `./le job run label encode-list command bin/le test bgp encode
--list` hands `--list` to the child.

## Launcher builds do not overwrite running binaries

`./le` keeps an existence cache: a compatible executable is reused until
`--update` requests a rebuild. If the shared executable belongs to another
platform, the launcher selects `bin/le-<OS>-<architecture>/le` instead.
`--name <name>` rebuilds `bin/le-<name>/le` on every invocation and exports
`ZE_LE_BUILD_NAME` so nested calls select the same private build.

Every build, including a cold start and a named rebuild, writes a sibling
`<target>.new.<pid>` and then renames it onto the target. A process already
running keeps its old inode; another caller never opens a partially written
target. Concurrent builders need no lock or retry. The final filename stays
`le`, preserving personality dispatch. A failed build removes its staging file
and leaves any published target unchanged. Preparation failures, including a
missing or empty feature manifest, stop before the compiler runs.

<!-- source: le -- build_le, update_le -->

A named build does not stay forever. The hourly store trim (see "When the disk
is full") removes `bin/le-<name>/` once its `le` and the directory itself are
both a day old and no running process carries the path in its argv; the next
`--name <name>` call rebuilds it, at the cost of one warm build. A process
already running from it keeps its inode. `bin/le`, the shared
`bin/le-<OS>-<architecture>/`, `bin/ze` and every other `bin/ze*` never reach
the removal, and neither does a name the script's own name check would refuse.
A name that starts with a capital letter is never removed either, because that
is the shape of the platform directory.

<!-- source: internal/le/go/toolchain/gotoolchain.go -- NamedLauncherDir, NamedLauncherName -->
<!-- source: internal/le/scratch/livetrim.go -- trimLaunchers, launcherIdle, launcherNamedInArgv -->

## A bare `go test` is not `./le test unit`

Ze compiles features out behind build tags (`//go:build ze_isis`, `ze_ospf`,
`ze_ldp`, `ze_rsvpte`, `ze_web`, `ze_ssh`, and the rest). `internal/le/go/toolchain`
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

## What `./le test unit all` covers

`all` runs `./...` under the feature tag set and the race detector, then each
group whose own build tags compile its test files out of that run. Today that
second part is the `installer` group, because `internal/install`'s tests carry
`ze_installer` and no feature-tag run selects them.

The six named verbs (`bgp`, `core`, `plugins`, `config`, `cli`, `installer`) are
targeted SUBSETS, for iterating inside the one you changed. A green group says
nothing about the rest of the tree. Neither did `all` until 2026-09-03, when it
swept those six verbs and nothing else: a session that ran it read green over
every other component and over all of `internal/le`, `internal/appliance`,
`internal/test`, `pkg` and `cmd`.

`./le test unit` typed alone lists the verbs and starts no run.

## A functional suite is not the runner binary

`./le test functional <suite>` builds an isolated bare-named binary pair into the
session scratch directory (`internal/le/test/functional/binaries.go`). The daemon
carries the test-only tag set, and the suite runs with `LE_TEST_NO_BUILD=1`,
and `ZE_BIN` pointing at that set, as `<set>/le test <suite>`
(`BinarySet.Environment`).

Running `le test` directly skips all of that. The runner can then
rebuild a daemon without the test-only surface, so a fixture times out for a
build-population reason and the failure looks like the code under test. The
`--server` and `--client` hints the runner prints on failure inherit the same
gap: they re-run the same non-equivalent launch.

| Want | Use |
|------|-----|
| A whole suite | `./le test functional plugin` (`./le test functional list` names every suite, and `./le test functional select` says which ones a gating run would start) |
| One test, iterating | The owning compiled fixture's Go test, then rerun the whole `./le test functional <suite>` |
| A kernel-dependent suite in the VM | `./le test qemu netns-test suites <comma-separated-suites>` |

## The lint gate

The native per-edit hook (`internal/le/hookruntime/postwrite.go`) judges changed
lines only. Package-wide analysis is what finds the rest:

- functions and variables left unused by a refactor in another file
- import cycles introduced by a cross-package change
- type mismatches from an interface change
- constants and vars that became unreferenced

`./le go lint run` lints the full tree through each selected build. An explicit
`scope "<packages>"` limits the same matrix to those package patterns.
golangci-lint analyzes one GOOS, one GOARCH and one tag set per invocation, so a
file outside that build is unchecked. The flavor matrix that closes that hole
is `testing.md`, "The builds the linter reads"; its rows are `flavorMatrix` in
`internal/le/go/lint/matrix.go`.
<!-- source: internal/le/go/lint/verifylint.go -- Plan, planScope, plan -->
<!-- source: internal/le/go/lint/actions.go -- runRunner -->

Before querying the matrix, after each flavor query, and after full-tree
coverage enumeration, the planner compares the shared lint input fingerprint.
Source content and build tags, nonignored file additions and deletions, lint
configuration, and module and feature manifests participate. A changed
fingerprint or a failed measurement refuses the plan before any lint child
starts; rerun after source edits stop. It does not retry or remove packages to
obtain a plan. These are observation checks, not an atomic filesystem snapshot.
<!-- source: internal/le/go/lint/verifylint.go -- plan, checkPlanningInputs -->
<!-- source: internal/le/job/treehash.go -- InputHash, writeReadPaths, lintIgnores -->

## The changed-set selector

`./le repo changed scope` is the one selector, and `./le verify current mode changed`
reuses its answer. It reports the changed packages plus two levels of their
importers (`defaultDepth`, `internal/le/repo/changed/selector.go`), and the feature
tags the change can reach.

```
./le repo changed scope print both
./le repo changed scope print packages paths-from FILE
```

A non-Go path seeds the Go packages whose tests read it, so a `.ci` file or a
rule point selects the native tooling packages rather than nothing.

Every route that fails to narrow WIDENS to `./...` and names its reason on stderr
(`widen`, `internal/le/repo/changed/scope.go`). One reason is routine:
`tmp/ze-verify.status` holding no green commit. With nothing proven, every scoped
target judges the whole tree until a full run passes. The contract is
`../architecture/testing/verify-freshness-scope.md`.

One directory answers with the whole tree by design. Every file in
`cmd/ze-installer` carries `//go:build linux && ze_installer`, so `go list` under
the unit tag set reports no package there, and `seedPackages`
(`internal/le/repo/changed/selector.go`) has nothing narrower to name. The wide answer
is what makes the `ze_installer` lint flavor run at all.

A scoped run also judges fewer Staticcheck feature-matrix rows. `scopeMatrix`
(`internal/le/go/staticcheck/staticcheckfeaturematrix.go`) keeps the two
rows that omit no feature tag, plus one row per tag the change reached: 3 of 38
for a `ze_ssh`-local change, with 36 feature tags declared in `feature-gates.txt`.
Those two rows are `all_features` and `core_only`, and `validateScoped` refuses
any scope that subtracts one of them. `./le go staticcheck check`
typed on its own judges every row, because only a verify run publishes the
feature-tag answer that `ZE_VERIFY_SCOPE_TAGS` (`ScopeTagsKey`) names.

Whatever rows the scope leaves are then CUT across six stages,
`check part 1 of 6` through `check part 6 of 6` (`staticcheckParts`,
`internal/le/verify/engine/stages.go`). `Matrix.Part` deals the rows round robin,
so each row is judged by exactly one piece and the pieces together judge them
all. A CI red names the piece it came from, and that command reproduces it:

```
./le go staticcheck check part 3 of 6
```

A piece dealt no row says so and passes: its rows are judged by a sibling piece
of the same run. One run is bounded at 90 seconds for each row it judges, so the
bound follows the size of the piece.

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
./le test unit all > "$dir/unit.log" 2>&1
```

A session directory outlives its session, so a log written today is there
tomorrow. Nothing removes it at session end or from a hook. `./le session reap`
removes only session directories whose owners are provably gone, and the hourly
store trim (see "When the disk is full") removes a directory only when those same
rules call it dead AND its mtime is more than a day old. Inside a directory it
keeps, the trim removes a `testbin-pid-<pid>-<label>/` that is more than six hours
old and whose run is over: the pid is not running, or the process now running
under that pid started after the directory was made. A `testbin-<suffix>/`
(an explicit `ze.suffix`) leaves only with its session; a suffix starting with
`pid-` is refused, because it would name a throwaway set. When the process table
shows no Claude CLI, as inside a QEMU guest or a container, the trim cannot tell a
live session from a dead one and removes none of these. Artifacts that are
already session-keyed, and the shared-by-design ones (`tmp/ze-verify.*`, and
the durable Go build cache `internal/le/go/toolchain` assigns), stay where they
are.
<!-- source: internal/le/test/functional/binaries.go -- binaryRoot, throwawaySetPrefix -->

## When the storage stalls

The Linux development VM keeps the checkout on a network drive that can freeze
for several seconds at a time (owner, 2026-08-31). A stall is not a code defect
and it is not the full-disk case below. It arrives as a test that talks to a
local socket failing on a timeout, several at once, each at exactly its
deadline: a RADIUS CoA run showed eight together at `no response: read udp4
...: i/o timeout`, all at 2.00s. A stage that takes minutes longer than its
neighbours for no visible reason is the same cause.

The tell that separates a stall from a defect is REPRODUCIBILITY, and it costs
one re-run to read. A stall does not survive one: the same command, unchanged,
comes back green. The full-disk case does survive, and `df -h cache/go-cache`
answers it. A real defect survives every run.

Two things follow. A timeout red measured while another session held a lint run
or a job is evidence about the machine, not about the code, so re-run it once
before you write anything down. And a failure you tried to reproduce and could
not is the one kind `ai/rules/completion.md` lets you RECORD instead of fix, so
its journal row carries the reproduction attempt and says the storage stalled.

## When another session cleans the cache under you

`cache/go-cache/...: no such file or directory`, on files that are there, is a
CONCURRENT `./le scratch cache-clean` (owner, 2026-08-31). Sessions share this
checkout and they share one build cache, so a clean run by one session empties
the cache the others are mid-build against. It clears by itself on a retry,
because the next build repopulates what it needs. The store trim is the second
cause: it removes entries older than three hours from the same caches while
other sessions build, so a build that had located such an entry and not yet
opened it meets the same error, and the same single retry clears it.

Do not read it as a full disk: that case says `no space left on device` and
survives a retry. Do not read it as a code defect either. Retry the command
once, and if you are the session about to run `cache-clean`, remember that every
other session in this checkout pays for it.

Store trim charges allocated blocks for directory entries as well as files.
An `ENOTEMPTY` directory remains charged; `ENOENT` releases its recorded bytes
without counting a removal by this run. Contention fixtures budget the measured
retained directory plus the newest file, rather than assuming an empty
directory uses no blocks.
<!-- source: internal/le/scratch/cachetrim.go -- treeBytes, allocatedBytes, walkCacheSubdirectory, removeOldest -->

## When the disk is full

A full cache disk is read as a code defect often enough to have its own class
file. `plan/journal/full-disk-false-red.md` carries one row for each occurrence,
and that file is the count: no page repeats it, because a copy goes stale on the
next row. It arrives as a wave of unrelated failures:

- Packages that do not import each other fail to build.
- The linker says `mapping output file failed: no space left on device`.
- A verification stage reports `cache entry not found`.
- A whole functional suite goes red at once.

Read the device that holds the CACHE, by naming the cache path itself:

```
df -h cache/go-cache          # follows cache/ to the device that fills, linked or not
df -h ~/.cache/ze/go-cache    # the shared per-user cache verify worktrees write
df -h tmp/golangci-lint-cache # the lint cache, which is NOT under cache/
findmnt -T cache/go-cache     # Linux: which device that path is on
```

`df` on the checkout ROOT can answer about the wrong device. The per-user cache
target is `$XDG_CACHE_HOME/ze`, or `~/.cache/ze` (`internal/le/scratch/scratch.go`,
`cacheTarget`), and that target is frequently its own filesystem. The checkout's
`cache/` is a symlink to it only after `./le scratch links-ensure` or
`./le scratch migrate` ran (`Ensure`, `EnsureCache`); nothing at le start
creates the link. A checkout that ran neither has a plain `cache/` directory and
TWO durable Go caches: `cache/go-cache`, which every le action writes, and
`~/.cache/ze/go-cache`, which every verify worktree writes through its own link
(see "A verify worktree shares that cache" below). Naming each cache path rather
than the checkout is what makes `df` answer correctly.

The built kernels under the same per-user target, `~/.cache/ze/runtime-kernel/`
and `~/.cache/ze/installer-kernel/`, are NOT a reclamation target, and no
cleanup removes them: not `./le scratch cache-clean`, not the store trim, and
not a hand `rm` while recovering a full disk. A cold kernel build takes about
thirty minutes, the arm64 kernel is built on the Mac and the amd64 kernel on the
Linux host, and neither host can rebuild the other's. The only thing that
removes an entry is the eviction after a newer build, which keeps the two newest
entries of each architecture and never the entry the current tree resolves to
(`evictKeepN`, `internal/appliance/cache.go`).
`TestCleanTargetsNeverReachTheKernelCache` (`internal/le/scratch`) fails when a
clean or trim target reaches into either directory.
<!-- source: internal/appliance/cache.go -- KernelCacheNamespaces, evictKeepN -->

`stat -f` is NOT the command to reach for, and the 2026-09-12 and 2026-09-13
rows in the class file are both that mistake. On macOS `stat -f` takes a FORMAT
string, so `stat -f cache/go-cache` prints `cache/go-cache` back and diagnoses
nothing. The GNU filesystem mode exists only on Linux, and even there APFS-style
purgeable accounting is what made one reading report terabytes free on a volume
at 98 percent.

Five caches fill, and emptying one leaves the others full. Four are Go caches,
on as many as three filesystems. The fifth is golangci-lint's, which no
`go clean` reaches and which the scratch relocation leaves on the checkout's own
device; it held 9.5G on 2026-09-13, more than the two Go caches measured beside
it. `./le scratch cache-clean` empties all of them and prints what each one
returned:

One row for each cache, in this shape:

```
$ ./le scratch cache-clean
checkout  /path/to/checkout/cache/go-cache                 freed 89.0G, free 123.2G
shared    /Users/thomas/.cache/ze/go-cache                 freed 13.0G, free 136.2G
ambient   /Users/thomas/Library/Caches/go-build            freed 1.2G, free 137.4G
bootstrap /path/to/checkout/tmp/go-cache                   freed 1.3G, free 138.7G
lint      /path/to/checkout/tmp/golangci-lint-cache        freed 9.5G, free 148.2G
```

With `cache/` linked to the per-user target, the shared row reads instead:

```
shared    SKIP     /Users/thomas/.cache/ze/go-cache: cache/ links here, so this is the checkout cache, which this run already emptied
```

The CHECKOUT cache is `cache/go-cache`. Every le action writes it, because
`Overrides` (`internal/le/go/toolchain/gotoolchain.go`) points GOCACHE there, and
`gotoolchain.GoCache` names it. The SHARED cache is the per-user
`cacheTarget()/go-cache`, which every verify worktree writes; when the checkout's
`cache/` links to that same directory, the row says SKIP because it IS the
checkout cache, and it is emptied once. The AMBIENT cache is the one a bare `go build`
writes outside le. The action asks `go env GOCACHE` for that path with the
inherited override removed, so a checkout whose default already IS the checkout
cache gets a SKIP row rather than one cache emptied twice. The BOOTSTRAP cache
is `tmp/go-cache` (`gotoolchain.BootstrapCache`), which the `le` script's own
build, the deployment builds and the QEMU guest write. The LINT cache is
`tmp/golangci-lint-cache`, which `Overrides` names through
`gotoolchain.LintCache`, and it is emptied by deleting the directory because no
`go clean` owns it. The equivalent by hand is:

```
go clean -cache                                    # the ambient cache
env GOCACHE="$PWD/cache/go-cache" go clean -cache   # the checkout cache
env GOCACHE="$HOME/.cache/ze/go-cache" go clean -cache  # the shared cache, unless cache/ links to it
env GOCACHE="$PWD/tmp/go-cache" go clean -cache     # the bootstrap cache
rm -rf tmp/golangci-lint-cache                     # the lint cache
```

The clean costs recompilation, which makes the next run slow once, so le also
bounds four of the five caches by itself, a little at a time. Every le
invocation reads `tmp/store-trim/stamp`. When the stamp is an hour old, one
invocation takes `tmp/store-trim/stamp.lock`, rewrites the stamp and starts
`le scratch store-trim background` as a detached child, so no command, hook
calls included, waits for the walk. The child trims:

| Caches | Budget | Default |
|--------|--------|---------|
| checkout, shared and bootstrap, as ONE union | `ze.le.store.go-cache-budget` | `40G`, one total across the three |
| lint | `ze.le.store.lint-cache-budget` | `10G` |

Each pass removes `-a` and `-d` entries from the 256 two-hex-digit
subdirectories, oldest mtime first, until the total is at or under the budget.
In the Go pass that order runs across all three caches at once, so a cache that
is under 40G on its own can still lose its coldest entries to the others' use.
Nothing else in a cache directory is touched, and no `go clean` runs. No entry
younger than three hours is removed, whatever the budget: when that is what
stands between the cache and its budget, the report says the budget is unmet
and how many bytes the floor kept. An entry a peer removed first counts as gone
but not as removed by this run. The ambient cache is not le's, and Go's own
five-day trim keeps it: its row names the path and is a skip.

A budget is a whole number followed by `M` or `G`, from `1M` to `1048576G`. Any
other spelling, a bare number included, refuses that budget's caches by name and
leaves them untouched, while the rest are trimmed. `ze.le.store.trim=off` stops
le from starting a trim by itself, and the functional test runner gives every
test child that setting (`docs/architecture/testing/runner-architecture.md`).
`./le scratch store-trim` runs one in the
foreground whenever asked, ignoring the stamp, and prints one line for each
budget and one row for each cache (`| json` renders the same report). The
background child writes that report to `tmp/store-trim/last.log`, which holds
the latest run. The action exits 1 when a budget, a cache path or the
`ze.le.store.trim` value was refused; a contended entry or an unmet budget is
reported and exits 0.
<!-- source: internal/le/scratch/storetrim.go -- storeTrimBeforeDispatch, startTrimWhenDue, trimStores, trimReport -->
<!-- source: internal/le/scratch/cachetrim.go -- trimCaches, removeOldest, walkGoFormatCache -->

After the caches, the same child trims the stores a live session may be using,
by age and liveness and never by size: named launchers (`launchers`, see
"Launcher builds do not overwrite running binaries"), session directories
(`sessions`) and orphaned testbins (`testbins`, both in "Scratch files"). It
scans the process table once for all three. When that table shows no Claude
CLI, the three rows say `no session process visible` and nothing is removed from
them; the caches are trimmed either way. A row counts what it removed and what
it kept, and an error in one of these rows also makes the action exit 1. With
no `tmp/session/`, or when the session directories cannot be judged, the
`testbins` row is a skip that says which.
<!-- source: internal/le/scratch/livetrim.go -- trimLiveness, trimSessions, trimTestbins -->
<!-- source: internal/le/session/reap.go -- Judge, ScanProcesses -->

### A verify worktree shares that cache

`./le verify worktree` extracts a detached worktree under `tmp/verify-worktree`
and links its `cache/` to the per-user target, before any stage runs
(`internal/le/verify`, `sharedCacheLink`, calling `EnsureCache` in
`internal/le/scratch`). That target is the checkout's own cache only when the
checkout's `cache/` is linked to it too; otherwise it is the second, shared
cache that `cache-clean` reports on its `shared` row. `tmp/` is NOT linked with it: that one is per-checkout,
and the worktree's own `tmp/verify` is where the run writes the stage logs it
copies out afterwards.

Without that link `Overrides` resolves GOCACHE to `<worktree>/cache/go-cache`
(`internal/le/go/toolchain/gotoolchain.go`), so each run compiled from cold into a
private cache and deleted it unread. Two worktrees measured on 2026-09-03 held
7.4 GiB and 6.7 GiB of private cache against a 0.6 GiB source tree, which is
about 90 percent of each worktree, rebuilt once per run.

The run prints what it did with the link, and the line names the shared path:

```
verify-worktree: shared build cache: created  cache -> /Users/thomas/Unix/cache/ze
```

A cache that cannot be linked is reported on that line and the run continues
against a cold one. The link is speed and disk, never correctness.

### What the gate is holding

A verify worktree that carries uncommitted changes is never deleted, whoever
abandoned it (`sweepAbandoned`, `internal/le/verify/cleanup.go`). The run names
each one it kept, with the path, the age, the disk it holds, and the shape of
its dirt:

```
verify-worktree: 20260903T091500-p123-r1-abcdef012345 was abandoned but holds
uncommitted changes, so it is left alone: /path/to/it, 8h12m0s old, 8.3G,
0 modified, 0 untracked, 264 deleted
```

The shape decides what to do. Modified or untracked content exists only there,
so removing the worktree destroys it. Deletions alone are a tree somebody
emptied, and git restores every one of them from the commit the worktree is
detached at. Remove one by hand with `git worktree remove --force <path>` after
reading that line, never on a schedule.

### A run that judged nothing

`./le verify worktree` answers five statuses, and they are not interchangeable:

| Status | Meaning |
|--------|---------|
| 0 | Every stage ran and the tree passed |
| 1 | A stage judged the tree and found it wrong |
| 2 | The run itself broke before it could judge |
| 3 | The run reached no verdict: a stage could not judge its subject, or a full device defeated the run |
| 130 | An interrupt stopped the run between stages |

A full device is recognized by its typed error at the write site, never by
matching text (`Defeated`, `internal/le/verify/engine/run.go`). Status 3 says
the tree was never cleared, so it is neither a pass to build on nor a red to
debug: free the device and run it again.

The verdict line is the LAST line the run prints, and it states the status the
process exits with:

```
verify-worktree: full exit=3
```

The run publishes its certificate to this checkout's `tmp/ze-verify.status`
before it removes the worktree, so `./le verify status check` answers about the
run that just ended (`../architecture/testing/verify-freshness-scope.md`).

## Session binaries

Native test actions build their binaries inside the current session's private
directory, under bare names, so a sibling session cannot overwrite the binary
under test. Ask the owning action for the path rather than writing `bin/ze`.

The directory carries the session id, so the file name does not. That is what
keeps argv[0] personality dispatch working (`defaultDispatch`,
`cmd/ze/dispatch.go`, looks the base name up as a root, so a file named `le` runs
le) and lets a `.ci` test exec `ze` by bare name off one PATH entry. A binary's location also decides where
`ze` resolves its config and database (`ConfigDirFromBinary`,
`internal/core/paths/paths.go`), so a session's `ze` reads `<session-dir>/etc/ze`
and the repository's `etc/ze` belongs to the human alone.
`internal/test/sessionpath` is what a `.ci` test uses to find it.

That directory is LOOKED UP, never recomputed: every consumer takes the single
directory matching `tmp/session/????-??-??-<id>`, and names a new one with today's
date only on a miss. Recomputing from today's date would move a session's
directory at midnight and orphan the binaries it is running.

The session-local `etc/ze` is seeded once by
`./le session seed-store binary <session-bin>/ze`. `seedStore`
(`internal/le/session/seed.go`) validates that the binary belongs to the current
session directory. Credentials are generated per session: user `admin`, and a
random password at `<session-dir>/etc/ze/.dev-password`, mode 0600. A later seed
preserves the existing store and does not rotate the credentials.
The seeder runs `ze init --seed`, which writes a `database.zefs` artifact and
skips interface discovery on the development host. It then runs
`ze init --from` to import that artifact into the live `etc/ze/database` tree.
A `database.zefs` that is still in `etc/ze` stops the seed with the
`ze init --from` repair, because ze refuses to open a store beside a blob.

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

## Asking a run in flight whether it reddened your file

A full pass takes tens of minutes, and most of it is two whole-tree Go
analyses. Waiting for the published index above to answer "did this run redden a
file of mine" costs that whole hour, and the answer is already on disk: each
stage writes its own log, carrying its `VERIFY FAILURE GROUP:` declaration, the
moment that stage ends.

```
./le verify reds file internal/component/bgp/reactor/peer.go
```

It reads the newest run directory under `tmp/verify/`, whether or not the run
has finished, and answers one of four verdicts
(`internal/le/verify/reds.go`, `verdictOf`):

| Verdict | Means | Exit |
|---------|-------|------|
| `named` | a stage that has reported declared a red naming this path | 1 |
| `undetermined` | nothing that reported names it, and something is unknown: stages still to report, or a red that named no file | 1 |
| `not-named` | every stage reported, every red among them named its files, and none is this path | 0 |
| `no-run` | no run directory exists, so nothing has judged anything | 1 |

**It is a query, never a certificate.** Only `not-named` exits 0, and it demands
a whole run. Every answer carries `stages`, `reported` and `pending`, so a
reader can see how much of the run has judged nothing yet. The gate a commit
passes is still `./le verify worktree` and the freshness certificate
(`ai/rules/precommit-verify.md`); this action tells you whether to keep working
or to go and fix something, and it starts no run.

The answer is structured data, so `| json`, `| yaml` and `| table` each render
it.

The pretool-bash guard refuses that pipe for the areas that RUN something:
`verify`, `verify lock`, `verify deps`, `verify lint`, `functional`,
`integration`, `qemu` and `test-unit` (`heavyArea` in
`internal/le/hookruntime/bash.go`). It reads the two-word area name, so
`./le verify status check` and `./le verify summary` stay pipeable: they read
and write the verification certificate and start no run.

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

Parallel verify runs share the build cache, the ports, and the test binaries. An
admitted job runs now, queues behind the jobs already in flight, or attaches to an
equivalent run. The actions that admit themselves today are `./le go lint run`,
`./le job run`, and every harness RUNNER under `le test`: the 24 suites plus
`bgp`, `editor`, `exabgp`, `vpp` and `web`. A runner takes the slot, then runs
itself again as a child inside it, so the slot's log grows with the run's output
(`RunnerAnswer`, `internal/le/test/harnesstool/harnesstool.go`). A harness HELPER
tool (`peer`, `rpki`, a mock, `fixture`) never admits, because the suite that
started it already holds the slot. `./le test stress-repro` holds ONE slot for
its whole run and names it as the parent of its parallel `le test <suite>`
children, so each repetition runs inside that slot rather than attaching to the
first child's verdict (`admitRun`, `internal/le/test/stressrepro/run.go`). With
no checkout, in a container, a runner runs unadmitted. A QEMU guest names a stand-in for the host's slot as its
parent (`guestJobParent`, `internal/le/test/qemu/guestle.go`), because the guest
sees the host's registry through `/workspace` but not the host's processes. The
pretool hook reads a runner as heavy for the lossy-pipe refusal, and a helper as
light, from the same registration (`leroot.Admits`). Anything else is admitted by
typing it after `./le job run label <label> command`. The rest of the heavy
population joins in
`plan/spec-native-action-job-admission.md`, so a second `./le verify current mode
full` does NOT block on the first: only its lint stage does.

`ZE_RUN_SLOTS` (`SlotsKey`) sets the slot count. It defaults to one slot per core
share this machine holds (`defaultSlots`, `internal/le/job/job.go`), which is four
on the 32-core development box, because `internal/le/go/toolchain` already caps each
job at a quarter of the cores (`CoresPerJob`, which is the same number it uses for
`GOMAXPROCS` and for the linter's `-j`).

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

Everything after the `command` keyword is the child's argv, and `job` passes it
through unchanged (`parseRun`, `internal/le/job/answer.go`; `Admission.Run`,
`internal/le/job/job.go`). The DISPATCHER reads the line first. It answers a
trailing `help`, `--help`, `-h` or `-help` with a usage page, and it does not
run the child (`asksForUsage`, `internal/le/le/root/dispatch.go`). Every other
option travels on, so `command echo -html=cover.out` reaches the child.

The command adds no build tags, no `-race`, no
package pattern and no timeout of its own, so write each of them yourself. The
`PKG=` and `RUN=` spellings belong to `./le test fuzz`, which declares them as
argument aliases; `go test` reads `PKG=./x` as an import path and refuses it.

For a direct `go test` child, the job boundary selects `CGO_ENABLED=1` when the
Go build flags enable `-race`, and `CGO_ENABLED=0` otherwise. This overrides the
launcher's build-only `CGO_ENABLED=0`; no `env` wrapper is needed. Effective
`GOFLAGS` is applied first using Go's whole-field quoting, then explicit argv
wins. If argv does not determine race mode and the OS `GOFLAGS` is absent or
empty, the job asks the selected Go executable for `go env GOFLAGS` in the same
directory/environment, preserving any leading `-C`. This includes persistent
`GOENV` configuration and its platform-specific default location without a
second configuration-file parser. A failed lookup stops the command rather
than inventing a CGO mode. Explicit race flags and nonempty OS `GOFLAGS` need
no configuration subprocess.

An explicit `-race=false` disables the detector and repeated race flags
use the last value. Flags after `-args`, or in a positional test-argument tail
after the completed package list, belong to the test binary rather than the
Go build. Values of other Go flags, such as the pattern in `-run -race`, do not
enable instrumentation. The child's argv and `GOFLAGS` are unchanged and Go
still validates them.

The same direct-child boundary removes every spelling of the outer checkout
root and named-build identity, while preserving the parent job setting. Keep
`go test` as the direct command (an optional leading Go `-C` is supported);
`job` does not parse shell commands or `env` wrappers.
<!-- source: internal/le/job/process.go -- commandEnvironment, goTestDefaults, goTestRace, goFlagsRace -->

Carry the feature tags from the recipe at the top of this page, or the run
judges a tree in which no gated plugin registers.

```
tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
./le job run label unit-pkg quiet command go test -race -tags "$tags" ./internal/core/eap
./le job run label unit-pkg quiet command go test -race -tags "$tags" -run TestEAPTLS ./internal/component/ike/...
```

Drop `-race` while iterating if you must. A package tested without `-race` has
not been tested the way the gate tests it, so put it back before the end.

### `quiet`, and the pattern it replaces

`quiet` sits between the label and the `command` keyword. It sends the child's
merged output to `tmp/session/<date>-<id>/scratch/job-<label>.log` instead of
the terminal, and answers three things: the exit code, that log's path, and up
to twenty failure lines from it, each with its line number.

```
job unit-pkg: exit 1, log tmp/session/2026-09-01-abc/scratch/job-unit-pkg.log
412:--- FAIL: TestEAPIdentity (0.00s)
418:FAIL	github.com/ze-software/ze/internal/core/eap	0.310s
```

Which lines those are is one declaration, `internal/le/runlog`, shared with the
verification failure index, so a stage log and a quiet job name the same lines.

Write no redirect of your own, and no `grep` over the result. A session that
composes a scratch path, a redirect, an exit-code echo and a `grep` is writing
this command by hand, and each hand-written copy picks a different set of lines
to keep.

The job log under `tmp/.ze-jobs/` is not that file. It is the breaker's
liveness evidence and a follower's replay source, and `ticket.Release`
(`internal/le/job/registry.go`) removes it when the job ends, so a reader who
arrives after the run finds nothing. The quiet log is the session's own and
stays until the session's scratch directory goes. Two runs of one label in one
session overwrite it.

An ordinary run still streams to the terminal, which is what every wrapped
recipe needs.

## Which native action owns the documentation gate

`./le doc check verify` and `./le repo generated-check` are separate actions.
`internal/le/doc/wiring.Verify` owns the ordered documentation gate, including the
`internal/le/doc/yangcontract` command and drift checks, the `internal/le/doc/check` links,
and RFC freshness. `internal/le/repo` owns the generated repository artifacts.

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
covers Perl, Ruby, Python, Node, Deno and Bun payloads. It refuses one that
names a governed tree beside a write call: a write mode passed to `open`, a
write method, `writeFileSync`, or a rename, copy or move. A payload that only
reads passes. The tier over-matches on purpose, so a heredoc that merely NAMES those trees beside a write
primitive is refused too. A wrong refusal is answered with
`ZE_ADMIT_GOVERNED_WRITE="<reason>"`, never by rewording the command.

A path that reaches the writer at run time is followed as a taint
(`internal/le/hookruntime/bash_governed_taint.go`). A variable is tainted when
it is assigned a governed path, when a `for` loop iterates over one, or when a
`while read` loop is fed from a command that lists or searches a governed tree
(`find plan`, `git ls-files ai/rules`, `done < <(find plan ...)`). A write
through a tainted `$f` or `${f}` is refused: an in-place editor, a redirect,
`tee`, or a `cp` or `mv` target. The guard also refuses an in-place editor run
by `xargs` or `find -exec` after a command that names a governed tree, one
handed its files by `$(find plan ...)`, and, after a `cd` or `pushd` into a
governed tree, any in-place editor or a relative redirect, `tee`, copy or move
target. A read into scratch passes, because the scratch variable never held a
governed path: `grep x plan/a.md > "$S/out"` is free. A file list read from a
file that is not itself governed, such as `sed -i x $(cat list.txt)`, is not
seen. Reading stays free: `grep`, `cat`, `sed -n` and `./le commit create file plan/spec-x.md
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
