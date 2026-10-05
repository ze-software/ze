# Spec: le-bounds-its-build-stores

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-10-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The build stores that `le` and its sessions fill have no size limit, so the disk
fills and every session reads the result as a code defect.
`plan/journal/full-disk-false-red.md` holds 24 rows of this class (2026-07-30 to
2026-10-04). The 2026-09-13 row records that two repairs landed (cache-clean
reaches the lint cache, `df` on the cache path) and names what is still
unrepaired: `tmp/session` growth, which `./le session reap` handles and nothing
schedules. The 2026-09-24 row adds leftover `testbin-*` directories, never reaped.
`docs/contributing/running-commands.md` says it in one line: "Nothing caps any of
the three, so all of them grow until the disk fills."

Measured on 2026-10-05 (owner's figures plus this session's): the checkout Go
cache reached 89G in the week since 2026-09-28 with about 47 live Claude
processes sharing the checkout; `bin/` held 270 `le-<name>/` directories, 33G,
about 130M each; `~/.cache/ze/go-cache` held a separate 13G.

Owner's decision (Thomas, 2026-10-05): **Option A, the limit lives inside le**,
not a launchd job. Whenever `le` starts, at most once per hour, with a timestamp
file for throttling that is safe under many concurrent sessions, le checks each
store against its limit and trims it. The Go build caches are trimmed least
recently used first (oldest mtime first) until under budget, default 40G,
overridable. The 40G is ONE TOTAL across the checkout, bootstrap and per-user Go
caches (owner, 2026-10-05, see Owner Decisions).
The same mechanism bounds, in the same change, the golangci-lint cache, the
per-name `le` binaries under `bin/le-<name>/`, the `tmp/session/<date>-<id>/`
scratch directories, and leftover `testbin-*` directories. Stores a live session
may be using are judged by age plus liveness, never by size alone. A trim racing
another session's build must not produce a false red beyond what one retry clears.

This spec closes the "nothing caps the cache" ask of the `full-disk-false-red`
class.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/running-commands.md` "Scratch files", "When another session cleans the cache under you", "When the disk is full", "Launcher builds do not overwrite running binaries", "Session binaries" - the page that documents this surface
  → Constraint: the page states "Nothing caps any of the three" and "Nothing under `tmp/session/` is deleted automatically: not at session end, not on an age timer, not by a hook". Both sentences become false with this spec and MUST be rewritten in the same phase that makes them false (`ai/rules/documentation.md`).
  → Constraint: the page states "`cache/` is a symlink to `$XDG_CACHE_HOME/ze`, or to `~/.cache/ze`". That is only true after `./le scratch links-ensure` or `./le scratch migrate` ran. The producer (`internal/le/scratch/scratch.go`, `Ensure` / `EnsureCache` / `ensureSymlink`) is called only by those two actions and by `sharedCacheLink` in `internal/le/verify/lifecycle.go` for a verify worktree; nothing at le start creates the link, and `Ensure` "creates or repairs the links without converting real paths". On the owner's Mac today `cache/` and `tmp/` are both plain directories (`cache/` created 2026-09-28). So the page is wrong for a checkout that never ran the cutover: such a checkout has TWO durable Go caches, `cache/go-cache` (every le action, via `Overrides`) and `~/.cache/ze/go-cache` (every verify worktree, via `sharedCacheLink` calling `EnsureCache`). The page edit names both.
  → Constraint: "When another session cleans the cache under you" already teaches the retry for `cache/go-cache/...: no such file or directory`. The trim's residual race lands in that same retry, so the section gains one sentence naming the trim as a second cause, not a new section.
  → Decision: the per-name launcher `bin/le-<name>/le` is rebuilt on EVERY call (`le` script, `update_le "$binary"` under `--name`, and `update_le` runs `mkdir -p` on the target's directory). Deleting an idle `bin/le-<name>/` costs one warm rebuild and nothing else. A running process keeps its inode after the unlink.
- [ ] `plan/journal/full-disk-false-red.md` - the 24-row class this spec closes
  → Constraint: the 2026-09-04 row records `cache-clean` REFUSED at 97% with `go: unlinkat .../go-cache/21: directory not empty` because peers wrote while `go clean` walked. The trim MUST NOT call `go clean`; it removes individual entries and treats a non-empty-directory refusal as contention (`isWriterRace`), not failure.
  → Constraint: the 2026-09-24 row: `testbin-pid-*` pairs are 252M to 376M each, five in one session dir, landing on the CHECKOUT device while the cache is on another. Session-device stores are part of the budget problem, not only the Go cache.
  → Constraint: the 2026-09-13 row: the lint cache held 9.5G, more than both Go caches combined after a clean. It needs its own budget.
- [ ] `ai/rules/config.md` - env var vs YANG
  → Decision: le reads no config file and has no YANG tree; every le knob is an env-only `env.MustRegister` entry with `Private: true` (`internal/le/go/toolchain/gotoolchain.go`, `tagsEntry`, `timeoutEntry`, `memLimitEntry`). The budgets follow that precedent: env-only is the rule's "bootstrap value read before config parses / internal safety cap" exception, because le has no config to parse.
  → Constraint: names are spelled out in full, and the key's final segment is the setting's name.
- [ ] `ai/rules/principles.md` - single declaration, no silent default
  → Constraint: the list of stores is declared once and both `./le scratch cache-clean` and the trim derive from it. `cleanTargets` (`internal/le/scratch/cacheclean.go`) is that list today; it gains the per-user cache and the trim reads the same list.
  → Constraint: an unparseable budget MUST be refused by name; the trim MUST NOT fall back to the default and delete under a guessed limit.
  → Constraint: the shell script names `bin/le-<name>` before any Go binary exists. The Go side declares the same path once and a test compares it with the script, as `TestBootstrapCacheMatchesTheShellScript` does for `BootstrapCache`.
- [ ] `ai/rules/goroutine-lifecycle.md` - no `go func()` for the trim
  → Decision: the trim runs in a detached child process, not a goroutine, because the le process that triggers it exits in milliseconds (a hook call) and would kill a goroutine mid-walk.
- [ ] `ai/rules/commands.md` - five-minute tool window; no polling loops
  → Constraint: no le invocation may wait for the trim. A walk of an 89G cache takes seconds to tens of seconds; a hook call that paid it would stall every session once an hour.

### Go toolchain source (verified at the producer)
- [ ] `$(go env GOROOT)/src/cmd/go/internal/cache/cache.go`, Go 1.27.1 (`/opt/homebrew/Cellar/go/1.27.1/libexec`)
  → Constraint (mtime approximates last use): the source comment reads "We set the mtime on a cache file on each use, but at most one per mtimeInterval (1 hour), to avoid causing many unnecessary inode updates. The mtimes therefore roughly reflect "time of last use" but may in fact be older by at most an hour." `markUsed` calls `os.Chtimes(file, now, now)` only when `now.Sub(info.ModTime()) >= mtimeInterval`, with `mtimeInterval = 1 * time.Hour`. So oldest-mtime-first is least-recently-used to within one hour, and an entry used at time t carries an mtime of at least t minus one hour.
  → Constraint (Go's own trim does not bound size): `Trim` runs at most once per `trimInterval = 24 * time.Hour` (recorded in `trim.txt`) and deletes only entries unused for `trimLimit = 5 * 24 * time.Hour` plus one `mtimeInterval`. It has no size limit, which is why 89G accumulated inside five days.
  → Constraint (what an entry is): `trimSubdir` removes only names ending `-a` or `-d` in the 256 two-hex-digit subdirectories, and an `-d` that is a directory is an executable entry removed with `os.RemoveAll`. `README` and `trim.txt` are not entries. The le trim removes exactly the same set.
  → Constraint (a missing entry is a miss, not an error): `GetFile` stats the output file and answers `&entryNotFoundError{Err: err}` when it is absent, which the build treats as a cache miss and recompiles. The false-red window is narrower: a file the build has already located and not yet opened.
- [ ] `tmp/golangci-lint-cache/` layout (observed 2026-10-05)
  → Decision: the lint cache uses the same layout: 256 two-hex-digit subdirectories of `-a` / `-d` entries plus `README` and `trim.txt`. One trimmer serves all four caches: one pass over the union of the three Go build caches against one total, and a separate pass over the lint cache against its own budget.

**Key insights:**
- Trigger point is `run` in `internal/le/register.go`, the le root handler every invocation passes through, including the native hooks (`le ai hooks ...`). The check there is a stat of one stamp file.
- The trim itself is a detached child `le scratch store-trim background` so no caller waits.
- Liveness for session-owned stores reuses `session.Reap`'s process scan; no second process scanner.
- An age floor protects in-flight builds: an entry is never removed while its mtime is within the floor, because any use within (floor minus one hour) refreshed it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/scratch/cacheclean.go` (340L) - `CleanCaches` empties four caches named by `cleanTargets` (checkout `GoCache`, ambient `go env GOCACHE` with the override removed, bootstrap `BootstrapCache`, lint `LintCache`); `measureClean` measures `diskspace.Free` before and after; `isWriterRace` treats `ENOTEMPTY` / "directory not empty" as contention; `goCleanCache` runs `go clean -cache` under `GOCACHE=<path>`; `removeCache` `RemoveAll`s the lint cache. Its doc comment says it "answers every cache a checkout at root fills", yet it never reaches `~/.cache/ze/go-cache` when `cache/` is a real directory.
- [ ] `internal/le/scratch/scratch.go` (640L) - `cacheTarget` answers `$XDG_CACHE_HOME/ze` or `$HOME/.cache/ze`; `Ensure` / `EnsureCache` create the `cache` symlink only when asked; `scratchTarget` answers `$TMPDIR/ze/<checkoutID>`.
- [ ] `internal/le/scratch/actions.go` - verbs `links-ensure`, `migrate`, `cache-clean` registered under `./le scratch`.
- [ ] `internal/le/verify/lifecycle.go` `sharedCacheLink` - links a verify worktree's `cache/` to `cacheTarget()`; its comment says "the checkout's shared Go build cache", which holds only when the checkout's own `cache/` is that link.
- [ ] `internal/le/go/toolchain/gotoolchain.go` - `GoCache(root)` = `<root>/cache/go-cache`, `LintCache(root)` = `<root>/tmp/golangci-lint-cache`, `BootstrapCache(root)` = `<root>/tmp/go-cache`; env entries registered with `env.MustRegister`, `Private: true`, keys like `ze.go.test.timeout`, `ze.lint.memlimit`.
- [ ] `internal/le/session/reap.go` (510L) - `Reap(root, configDir, dry)` removes `tmp/session/<YYYY-MM-DD>-<sid>` directories with no live owner. Live means: this process's own session id, a `.sid-by-pid-<pid>-<start>` pin whose pid and start time match a running process, a running process whose argv contains the sid, or (when any Claude CLI runs) a transcript `~/.claude/projects/*/<sid>.jsonl` modified after the oldest running Claude CLI started. When a Claude CLI runs but `projects/` is missing it removes nothing and says so. `scanProcesses` reads `/proc`, else `ps -eo pid=,lstart=,etime=,comm=,args=`.
- [ ] `internal/le/test/functional/binaries.go` `binaryRoot` - names `testbin-pid-<pid>-<label>` (removed by its own run) or, under `ze.suffix`, `testbin-<suffix>` (kept by design) inside the session scratch directory.
- [ ] `le` (shell, 307L) - `--name <name>` or inherited `ZE_LE_BUILD_NAME` selects `bin/le-<name>/le`, rebuilt every call; a foreign-platform `bin/le` falls back to `bin/le-<uname -s>-<uname -m>/le`, shared by every session. `bin/le`, `bin/ze`, `bin/ze-linux-arm64` are shared too.
- [ ] `internal/le/register.go` `run` - calls `leroot.Dispatch(invocationName(), args)`; every le command and hook enters here.
- [ ] `internal/le/le/path/lepath.go` `Root` - resolves the checkout (ZE_REPO_ROOT, then cwd ancestors, then the executable's ancestors).

**Behavior to preserve:**
- `./le scratch cache-clean` output rows (`checkout`, `ambient`, `bootstrap`, `lint`) and its verdict; it gains one row and loses nothing.
- `./le session reap` semantics and output, unchanged; the trim calls its rules, it does not copy them.
- The exit code, stdout and stderr of every le command: the trigger adds no output to the invoking command.
- `bin/le`, `bin/le-<OS>-<arch>/`, `bin/ze*` are never touched.
- `testbin-<suffix>` directories (explicit `ze.suffix`) are kept by design and leave only with their session.

**Behavior to change:**
- le bounds the Go caches, the lint cache, the named launcher binaries, session directories and orphaned testbins, automatically, at most once per hour.
- `cleanTargets` names the per-user cache (`cacheTarget()/go-cache`) when its real path differs from the checkout cache's.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Any `le` invocation (operator command, native hook, nested le call) reaching `run` in `internal/le/register.go`.
- Manual entry: `./le scratch store-trim`, which runs the trim in the foreground and ignores the hourly stamp.

### Transformation Path
1. `run` asks the scratch package whether a trim is due: resolve the checkout root, read `tmp/store-trim/stamp` (unix seconds). Fresh (under one hour) or the switch is `off`: return immediately, no further work.
2. Due: take a non-blocking exclusive lock on `tmp/store-trim/stamp.lock`. Lock held by another process: return. Lock taken: re-read the stamp (a peer may have just written it), and if still due write the current time, release the lock, and spawn the same executable (`os.Executable`) as `scratch store-trim background`, in a new session (`Setsid`), stdin from the null device, stdout and stderr to `tmp/store-trim/last.log`. Do not wait. Then dispatch the operator's command as before.
3. The child takes a non-blocking exclusive lock on `tmp/store-trim/run.lock` for its whole run; held means a previous trim still runs, and the child exits 0 with one log line.
4. Go build caches, one pass over their UNION: take the Go build caches in the shared target list (checkout, bootstrap, per-user), resolve each to its real path and drop duplicates (a linked checkout's `cache/go-cache` IS the per-user cache), walk the 256 subdirectories of each, and collect every `-a` / `-d` entry with its cache, mtime and allocated size into one list. Sum the sizes across all of them. Over the single `ze.le.store.go-cache-budget` total: sort the whole list oldest mtime first, regardless of which cache an entry is in, and remove entries older than the age floor until the combined sum is at or under the total.
5. Lint cache, its own pass: the same walk, sort and floor against its own `ze.le.store.lint-cache-budget`, never mixed with the Go caches' total.
6. In both passes `ENOENT` on remove is ignored (a peer or Go's own trim got there); `isWriterRace` on a directory entry is recorded as contention.
7. Liveness stores, only when the process scan sees at least one Claude CLI (AC-11): scan processes once (the session package's scanner); remove each `bin/le-<name>/` whose `le` mtime and directory mtime are both older than the binary age and whose path no live process argv names; take the directories `session.Reap`'s rules would remove and remove only those also older than the session age; remove each `testbin-pid-<pid>-<label>` whose pid is not running, or is running with a start time after the directory's mtime (pid reuse), and that is older than the testbin age.
8. Write one report: one row per store, with the three Go caches sharing one budget line that gives the combined size before and after with path, size before, size after, entries removed, and a skip or refusal reason. The background child writes it to `last.log`; the foreground action prints it (and renders it through `| json` with kebab-case keys).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le process -> detached le child | `exec` of `os.Executable()` with argv `scratch store-trim background`, `Setsid`, no wait | No |
| le -> filesystem shared with peer sessions and peer `go` processes | per-entry `os.Remove` / `os.RemoveAll`; never `go clean` | No |
| le -> process table | `session` package's existing scan (`/proc` or `ps`) | No |

### Integration Points
- `cleanTargets` (`internal/le/scratch/cacheclean.go`) - becomes the one store list; gains a size budget and kind per target.
- `session` reap rules - an exported entry that answers the removal candidates without removing (the liveness rules stay in `reap`), so the trim can age-filter before removing.
- `session` process scanner - exported (pid, start time, argv, Claude flag) so the bin and testbin checks share it.
- `gotoolchain` - gains the declaration of the named-launcher directory, compared against the `le` script by a test.
- `env.MustRegister` - three new private entries.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | store paths come from `gotoolchain` and `cacheTarget`; session liveness from `session` |
| No unintended coupling (components stay isolated) | Yes | scratch imports session; `go list -deps` checked both directions 2026-10-05, no cycle |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `cleanTargets`, `isWriterRace`, the reap rules, the process scanner |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | Yes | `store-trim` registers as a `./le scratch` action like `cache-clean` |
| Registration over hardcoding, inbound | Yes | the trigger in `run` names no command; the store list is the existing `cleanTargets`, not a second list; recursion is prevented by the stamp, not by matching the child's argv |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Oldest mtime first is LRU to within one hour | `cache.go` `markUsed` and its comment, quoted above (Go 1.27.1) | trim removes hot entries; slower builds, no false reds | unit test: an entry read through `go build` after an aged mtime is refreshed | confirmed 2026-10-05: `TestTrimmedEntryIsAMiss` ages every entry of a temporary GOCACHE two hours, repeats the compile, and finds entries refreshed to now (Go 1.27.1) |
| A-2 | The golangci-lint cache refreshes mtimes the same way | same layout observed; golangci-lint carries a copy of Go's cache package | lint cache trimmed in the wrong order; slower lint, no false red | read golangci-lint's cache source in the module cache during implementation | confirmed 2026-10-05: the installed golangci-lint 2.13.1 carries `internal/go/cache.(*DiskCache).markUsed` (`go tool nm`); its source at tag v2.13.1, `internal/go/cache/cache.go`, sets `mtimeInterval = 1 * time.Hour`, and `markUsed` calls `os.Chtimes(file, now, now)` when `now.Sub(info.ModTime()) >= mtimeInterval`, the same rule as Go 1.27.1 |
| A-3 | A missing `-a`/`-d` entry is a cache miss for every consumer (go build, go vet, golangci-lint, staticcheck) | `GetFile` answers `entryNotFoundError`; build treats it as a miss | a consumer fails hard on a trimmed entry | unit test: trim a warm cache to zero, rebuild and vet the same package, both exit 0 | confirmed 2026-10-05 for go build and go vet only: `TestTrimmedEntryIsAMiss` trims a warm cache with a zero budget and a zero floor, and both exit 0. golangci-lint and staticcheck were not run against a trimmed lint cache; they read it through the same `GetFile` copy (A-2), which is the basis, not the proof |
| A-4 | A process running from `bin/le-<name>/le` shows that path in its argv | the `le` script `exec`s `$binary`, the full path | a running named le is judged idle; its dir is removed; its next nested call rebuilds (script recreates the dir) | unit test over the scanner with a process started from such a path | confirmed 2026-10-05: the script assigns `binary=$root/bin/le-$name/le` with `root=$(cd "$(dirname "$0")" && pwd)` (absolute) and ends `exec "$binary"`; `TestTrimNamedLaunchers/a_process_started_from_a_named_launcher_shows_its_path` copies the test binary to `<tmp>/bin/le-probe/le`, starts it, and `session.ScanProcesses` (ps path, macOS) shows the path, which `launcherNamedInArgv` matches. A copied `/bin/sleep` is SIGKILLed by macOS code signing, so the probe is the test binary |
| A-5 | Every le run inside a QEMU guest or container sees no Claude CLI in its process table | guests run no Claude; `Reap` already relies on the same view | a guest trims host-owned bin/session dirs it cannot judge | AC-11 test; read the `internal/le/test/qemu` guest environment during implementation | confirmed 2026-10-05: a QEMU guest runs its own kernel, so its `/proc` lists guest processes only; `grep -ril claude internal/le/test/qemu/` finds nothing, and no `--pid=host` or `PidMode` appears in any Go, YAML or Dockerfile in the tree. `TestTrimSkipsLivenessWithoutSessions` proves that such a view removes nothing from the three liveness stores |
| A-6 | Allocated size (blocks) is the right measure for the budget | `df` reports blocks, and the class is about a full device | budget reads lower than disk use for small files | unit test compares the walk with `du -sk` on a fixture | confirmed 2026-10-05: `TestTrimGoCacheOldestFirst/allocated_size_matches_du`, `du -k` of a 64KiB entry times 1024 equals `allocatedBytes` (APFS) |
| A-7 | A 40G total across the three Go build caches fits the owner's machine | owner decision D-1 (2026-10-05) | disk still fills, or caches thrash | owner set it; the env entry overrides it without a code change | confirmed by owner |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Trim deletes a cache file a build located and has not yet opened: `no such file or directory` false red | `cache/go-cache/...: no such file or directory` in a stage log within an hour of a trim | age floor (default 3h): any use in the last two hours refreshed the mtime past the floor; the residual case is the retry the page already teaches, and the page names the trim as a cause |
| R-2 | All entries are younger than the floor, so the budget cannot be met and the disk still fills | report row says "budget unmet: N bytes younger than floor" | report it in `last.log` and in the foreground output; never lower the floor at run time |
| R-3 | Two checkouts share the per-user cache, so each counts it inside its own 40G total, and their children may trim it at once | both logs show removals in the same minute; one checkout's total dominated by per-user entries | over-trim costs only misses; ENOENT ignored; oldest-first across the union still removes the coldest entries; no lock across checkouts (documented limitation) |
| R-12 | Under one shared total a hot cache pushes out another's entries: a large test run filling the checkout cache evicts bootstrap or per-user entries | the `bin/le` rebuild or a verify worktree goes cold after a large run | intended by owner decision D-1: oldest-first across the union removes what was used least recently, whichever cache holds it; the report names how many entries each cache lost |
| R-4 | A guest or container trims host-owned liveness stores | removal of a live session dir; red in a running session | liveness stores run only when a Claude CLI is visible (AC-11), and `Reap` already removes nothing in that case |
| R-5 | The trigger slows every le call | hook latency rises | the due check is one read of a 10-byte file; the walk runs in the detached child |
| R-6 | A child left running forever, or many children | `ps` shows several `store-trim background` | `run.lock` makes a second child exit; the child has an overall timeout (the `cleanTimeout` precedent, one hour) |
| R-7 | PID reuse makes a dead testbin look live, or a live one look dead | testbin kept forever / removed under a run | compare the process start time with the directory mtime: a process that started after the directory was created does not own it; age floor on top |
| R-8 | `Reap`'s transcript rule keeps almost every session alive while one Claude CLI has run for days | session directories barely shrink | report the count kept; the design does not weaken Reap's liveness (owner: age plus liveness, never size); owner decision D-4 keeps the rule |
| R-9 | The stamp sits in `tmp/`, which `migrate` may relocate or a peer may delete | trims run more often than hourly | harmless: a missing stamp means due, the lock still serialises |
| R-10 | `bin/le-<name>/` is removed between the script's `mkdir -p` and its `.new.<pid>` write, failing a build | `le: build failed` once, clears on retry | binary age floor (24h) on both the binary and the directory mtime makes this need an idle-for-a-day name reused in the same second |
| R-11 | A stamp written in the future (clock moved back) suppresses trims indefinitely | no `last.log` for days | a stamp more than one hour in the future counts as due, as Go's `trim.txt` check does |
| R-13 | Every `le` a functional test runs is a trigger, so a fixture's throwaway checkout gains `tmp/store-trim/` and a detached child: trees diverge and tree-equality fixtures go red (seen on phase 5: `le-vendor-web-answers`, `le-docvalid-answers`) | a fixture failure naming `tmp/store-trim/last.log` or `stamp` | the runner owns every test child's environment and sets `ze_le_store_trim=off` (`childEnv`, `internal/test/runner/runner_exec_util.go`; `TestChildEnvTurnsTheStoreTrimOff`); `le-store-trim-answers` turns it back on in its own child environment. Per-fixture ignores of `tmp/store-trim` were rejected as a workaround. Both fixtures PASS after the fix |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Development sessions: slower builds after over-trimming, or a false red that a retry clears; at worst a live session's scratch directory or binary removed (guarded by liveness plus age) |
| How is it reverted? | single commit revert; `ze.le.store.trim=off` disables it without a revert |
| Who else touches this path? | `internal/le/scratch` (cache-clean), `internal/le/session` (reap), the `le` script's launcher build, `internal/le/register.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| any `le` command through `run` (`internal/le/register.go`) | → | scratch due-check, then detached `scratch store-trim background` | `TestRunSpawnsStoreTrimWhenDue` in `internal/le/register_test.go` |
| `./le scratch store-trim` | → | foreground trim over the shared store list | `test/ui/le-store-trim-answers.ci` (fixture `ui/le-store-trim-answers`) |
| the built le binary, invoked twice inside one hour against a seeded checkout | → | exactly one background trim, cache under budget afterwards | `test/ui/le-store-trim-answers.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `tmp/store-trim/stamp` absent or older than one hour, the combined `-a`/`-d` size of the checkout, bootstrap and per-user Go caches over the 40G total, any `le` command run | the command's exit code and output are unchanged and it returns before the walk finishes; afterwards the COMBINED size of the three caches is at or under the total, and every removed entry, in whichever cache, is older than every kept entry in any of the three that is older than the floor |
| AC-2 | stamp younger than one hour | no child is spawned and no store is walked |
| AC-3 | twenty `le` invocations start together with the stamp due | exactly one background trim runs (one `last.log` run header, stamp written once) |
| AC-4 | the Go union over its total (or the lint cache over its budget) and every entry younger than the age floor | nothing is removed; the report row says the budget is unmet and how many bytes are younger than the floor |
| AC-5 | cache directory holding `README`, `trim.txt`, a foreign file and entries | only `-a`/`-d` entries in the two-hex-digit subdirectories are removed |
| AC-6 | lint cache over its own 10G budget while the Go union is under its total, and the reverse | the lint cache is trimmed oldest first to its own budget by the AC-1, AC-4, AC-5 rules; its size never counts toward the Go total and Go entries never count toward it |
| AC-7 | three Go caches with interleaved mtimes (the oldest entry in the bootstrap cache, the next in the per-user cache `cacheTarget()/go-cache`, the next in the checkout cache), combined over the total; separately, a checkout whose `cache/` is a symlink to the per-user target | removals follow one global mtime order across the three caches, not cache by cache; under the symlink, the shared cache's entries are counted and listed once and its row says it is the same cache as the checkout's |
| AC-21 | each Go cache alone under 40G but the three together over it (for example 20G, 15G and 10G) | the union is trimmed to 40G in total; a per-cache reading would have removed nothing |
| AC-8 | `bin/le-<name>/` whose `le` and directory mtimes are older than the binary age and that no live process argv names | removed |
| AC-9 | `bin/le-<name>/` named in a live process's argv, or younger than the binary age; `bin/le`, `bin/le-Darwin-arm64/`, `bin/le-Linux-x86_64/`, `bin/ze`, `bin/ze-linux-arm64` | kept, every one |
| AC-10 | `tmp/session/<date>-<id>/` | removed exactly when `session.Reap`'s rules would remove it AND its mtime is older than the session age; kept otherwise |
| AC-11 | process scan shows no Claude CLI | bin, session and testbin stores are skipped with the reason "no session process visible"; the caches are still trimmed |
| AC-12 | `testbin-pid-<pid>-<label>/` inside a kept session directory | removed when the pid is not running, or is running with a start time after the directory's mtime, and the directory is older than the testbin age; kept when the pid is running and started before it; `testbin-<suffix>/` always kept |
| AC-13 | `ze.le.store.go-cache-budget` set to `abc`, `0G`, `-5G`, `5X` or a bare `5` | that store's row is a refusal naming the key and the value; nothing is removed from it; other stores proceed; the foreground action exits 1 |
| AC-14 | `ze.le.store.go-cache-budget=500M` (and `2G`) | the budget is 500 MiB (2 GiB); suffixes `M` and `G` are accepted |
| AC-15 | `ze.le.store.trim=off` | no le invocation spawns a trim; `./le scratch store-trim` still runs when asked, and says the automatic trim is off |
| AC-16 | `./le scratch store-trim` and `./le scratch store-trim '|' json` | one row per store with name, path, size before, size after, entries removed, and skip/refusal/contention text; JSON keys kebab-case; exit 0 unless a row is a refusal |
| AC-17 | an executable `-d` directory entry gains a file while being removed (ENOTEMPTY) | the row records contention, not an error, and the run continues |
| AC-18 | a warm Go cache trimmed to zero budget with a zero floor (test seam), then `go build` and `go vet` of the same package | both exit 0 (a trimmed entry is a miss) |
| AC-19 | `./le scratch cache-clean` on a checkout whose `cache/` is a real directory and `~/.cache/ze/go-cache` exists | a `shared` row empties the per-user cache too; with `cache/` linked to it, that row says skipped as the checkout cache |
| AC-20 | the Go declaration of `bin/le-<name>` and the `le` script | a test fails when the two spell the path differently |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunSpawnsStoreTrimWhenDue` | `internal/le/register_test.go` | AC-1 trigger, AC-2, AC-15 (spawn seam injected) | |
| `TestStoreTrimDueOncePerHour` | `internal/le/scratch/storetrim_test.go` | AC-2, AC-3 (twenty child processes contending on the stamp lock) | |
| `TestTrimGoCacheOldestFirst` | `internal/le/scratch/storetrim_test.go` | AC-1, AC-5 | |
| `TestTrimGoCacheRespectsAgeFloor` | `internal/le/scratch/storetrim_test.go` | AC-4 | |
| `TestTrimLintCache` | `internal/le/scratch/storetrim_test.go` | AC-6 | |
| `TestTrimGoCachesAsOneUnion` | `internal/le/scratch/storetrim_test.go` | AC-1, AC-7, AC-21: global mtime order across three temp caches, combined total, a symlinked duplicate counted once | |
| `TestTrimNamedLaunchers` | `internal/le/scratch/livetrim_test.go` | AC-8, AC-9 (process facts injected) | |
| `TestTrimSessionsAgePlusReap` | `internal/le/scratch/livetrim_test.go` | AC-10 | |
| `TestTrimSkipsLivenessWithoutSessions` | `internal/le/scratch/livetrim_test.go` | AC-11 | |
| `TestTrimOrphanTestbins` | `internal/le/scratch/livetrim_test.go` | AC-12 including pid reuse | |
| `TestBudgetParse` | `internal/le/scratch/storetrim_test.go` | AC-13, AC-14 | |
| `TestTrimReportJSON` | `internal/le/scratch/storetrim_test.go` | AC-16 | |
| `TestTrimWriterRace` | `internal/le/scratch/storetrim_test.go` | AC-17 | |
| `TestTrimmedEntryIsAMiss` | `internal/le/scratch/storetrim_test.go` | AC-18 (real `go build` / `go vet` against a temp GOCACHE) | |
| `TestCleanTargetsNamesSharedCache` | `internal/le/scratch/cacheclean_test.go` | AC-19 | |
| `TestNamedLauncherMatchesTheShellScript` | `internal/le/go/toolchain/gotoolchain_test.go` | AC-20 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze.le.store.go-cache-budget` | 1M to 1048576G | `1M` and `1048576G` | `0M`, `0G`, negative | a value whose byte count overflows int64 |
| `ze.le.store.lint-cache-budget` | 1M to 1048576G | `1M` and `1048576G` | `0M` | overflow |
| combined Go union size against the total | 0 to any | union exactly at the total: nothing removed | N-A | one byte over the total: the single oldest eligible entry across the three caches is removed |
| stamp age | any | 59m59s old is fresh, 60m old is due | N-A | more than one hour in the future is due (R-11) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-store-trim-answers` | `test/ui/le-store-trim-answers.ci` | the fixture builds le, seeds a throwaway checkout (ZE_REPO_ROOT) with an over-budget fake Go cache using small budgets, runs a cheap le command twice: the first leaves the cache under budget (after the child exits) and writes the stamp, the second spawns nothing; `./le scratch store-trim` prints one row per store | written (phase 5): driver `internal/test/fixture/ui_fixture_le_store_trim_answers.go` `leStoreTrimAnswers`, registered in `register_le_store_trim_answers.go`; it also seeds a throwaway per-user cache (XDG_CACHE_HOME), so the union is two caches with interleaved ages. Discrimination 2026-10-05: with `lescratch.StartStoreTrimWhenDue(storeTrimSpawn())` removed from `run`, red: `FAIL: the first le run started no background trim: open .../checkout/tmp/store-trim/last.log: no such file or directory`; with `stampFresh` never fresh, red: `FAIL: the second run inside the hour rewrote the stamp, so it started a trim`; restored, green (PASS 6.4s) |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/register.go` - call the due-check before `leroot.Dispatch`
- `internal/le/scratch/cacheclean.go` - `cleanTargets` gains the per-user cache (skipped when its real path equals the checkout's), a kind (Go-format or other) and a budget key per target
- `internal/le/scratch/actions.go` - register `store-trim` (foreground; `background` keyword for the child)
- `internal/le/session/reap.go` - export the process scan and a candidates-only entry (no rule change)
- `internal/le/go/toolchain/gotoolchain.go` - declare the named-launcher directory once
- `internal/le/verify/lifecycle.go` - correct the `sharedCacheLink` comment ("the checkout's shared Go build cache" holds only for a linked checkout)
- `docs/contributing/running-commands.md` - see Documentation checklist
- `ai/INDEX.md` - the `./le scratch` row names the trim
- `internal/test/runner/harness_exec.go`, `internal/test/runner/runner_exec_util.go` - every test child gets `ze_le_store_trim=off` (R-13, phase 5)
- `internal/le/test/functional/binaries.go` - `binaryRoot` refuses a `ze.suffix` starting `pid-` (closure review, AC-12)
- `docs/architecture/testing/runner-architecture.md` - the runner turns the trim off in every test child

## Files to Create
- `internal/le/scratch/storetrim.go` - due-check, stamp and locks, child spawn, per-store trim, report
- `internal/le/scratch/cachetrim.go` - the Go-format cache passes (walk, oldest-first removal, floor, writer race), split from `storetrim.go` in phase 3 as a second concern
- `internal/le/scratch/livetrim.go` - the liveness stores (named launchers, session directories, orphaned testbins, the no-Claude guard), split from `storetrim.go` in phase 4 as a third concern
- `internal/le/scratch/livetrim_test.go` - the four liveness-store tests
- `internal/le/scratch/storetrim_test.go` - unit tests above
- `test/ui/le-store-trim-answers.ci` and its registered native fixture `ui/le-store-trim-answers` beside the other `ui/le-*` fixtures

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le has no YANG tree; tooling knobs are env-only (`ai/rules/config.md` exception, precedent `gotoolchain` entries) |
| YANG validation constraints | N-A | no YANG; the budget parser validates (AC-13, AC-14) |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le scratch store-trim` in `internal/le/scratch/actions.go` |
| CLI grammar (keyword before value) | Yes | `background` is a keyword; no flags (`ai/rules/cli.md`) |
| Editor autocomplete | N-A | le actions complete from their registration |
| Functional test for new RPC/API | Yes | `test/ui/le-store-trim-answers.ci` |
| Pipe completeness | Yes | the report is a payload through `leroot.Run`, so `| json` works (AC-16) |
| Env var registration | Yes | `ze.le.store.go-cache-budget` (default `40G`, one total across the checkout, bootstrap and per-user Go caches), `ze.le.store.lint-cache-budget` (default `10G`), `ze.le.store.trim` (default `on`), all `env.MustRegister` with `Private: true` |
| Doctor check for runtime dependencies | N-A | le development tool; no ze daemon runtime dependency is added. The new files live under the checkout's `tmp/store-trim/` |
| Prometheus counters/metrics | N-A | development tooling, no daemon |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | developer tooling, not a ze feature |
| 2 | Config syntax changed? | N-A | no config |
| 3 | CLI command added/changed? | N-A | `docs/guide/command-reference.md` covers ze, not le; le's surface is the registered action and `ai/INDEX.md` |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | Yes | `docs/contributing/running-commands.md`: "When the disk is full" (replace "Nothing caps any of the three", correct the `cache/` symlink claim and name the two durable Go caches, add the trim, its budgets, its stamp and log, and `./le scratch store-trim`); "Scratch files" (replace "Nothing under `tmp/session/` is deleted automatically" with the age-plus-liveness rule); "When another session cleans the cache under you" (one sentence: the trim is the second cause); "Launcher builds do not overwrite running binaries" (named builds are reclaimed after a day idle) |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | none |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` only if the new fixture changes how `ui/le-*` fixtures are described; check at implementation |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` is the `// Design:` target of `cacheclean.go` and `scratch.go` but carries no scratch text (grep 2026-10-05); confirm with row 16. `docs/architecture/testing/verify-freshness-scope.md` is declared by `internal/le/verify/lifecycle.go`, whose only change is the `sharedCacheLink` comment: unaffected, no verify freshness behavior changes |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | an le action, not a ze command |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation: `./le spec citation anchors spec plan/spec-le-bounds-its-build-stores.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `cache-clean` sample output block in `running-commands.md` gains the `shared` row |

## Implementation Steps

1. **Phase: Wiring** -- register `./le scratch store-trim` returning an empty report; add the due-check call in `run` behind an injectable spawn seam
   - Tests: `TestRunSpawnsStoreTrimWhenDue`, `TestStoreTrimDueOncePerHour`
   - Files: `internal/le/register.go`, `internal/le/scratch/actions.go`, `internal/le/scratch/storetrim.go`
   - Verify: tests fail on the stub, then pass for stamp, lock and spawn
2. **Phase: store list and budgets** -- `cleanTargets` gains the per-user cache, kind and budget key; env entries; budget parser; cache-clean `shared` row; page edits for cache-clean and the two-cache correction
   - Tests: `TestCleanTargetsNamesSharedCache`, `TestBudgetParse`
3. **Phase: Go-format cache trim** -- the union of the three Go caches against one total, the lint cache against its own budget: walk, sort, floor, remove, ENOENT and writer race, report; page edits for the trim
   - Tests: `TestTrimGoCacheOldestFirst`, `TestTrimGoCacheRespectsAgeFloor`, `TestTrimLintCache`, `TestTrimGoCachesAsOneUnion`, `TestTrimWriterRace`, `TestTrimmedEntryIsAMiss`, `TestTrimReportJSON`
4. **Phase: liveness stores** -- export the session scanner and candidates; named launchers; sessions; testbins; the no-Claude guard; Go declaration of `bin/le-<name>` and its script comparison; page edits to "Scratch files" and "Launcher builds"
   - Tests: `TestTrimNamedLaunchers`, `TestTrimSessionsAgePlusReap`, `TestTrimSkipsLivenessWithoutSessions`, `TestTrimOrphanTestbins`, `TestNamedLauncherMatchesTheShellScript`
5. **Phase: functional** -- fixture and `.ci`; `ai/INDEX.md` row; row 16 anchors
   - Tests: `test/ui/le-store-trim-answers.ci`
   - Verify: revert the trigger in `run`, confirm the `.ci` goes red, restore, green (`ai/rules/interop-and-goal-validation.md` discrimination)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | no entry younger than the floor is removed on any path; no `go clean` call in the trim; ENOENT never reported as failure |
| Correctness | the trigger never waits for the child, never prints, never changes the exit code |
| Liveness | `bin/le`, `bin/le-<OS>-<arch>`, `bin/ze*` and `testbin-<suffix>` cannot reach a remove call |
| Single declaration | one store list (`cleanTargets`); one process scanner (session); one reap rule set; `bin/le-<name>` declared once in Go and compared with the script |
| No silent default | a bad budget refuses its store by name |
| Rule: goroutine-lifecycle | no `go func()`; the trim is a child process |
| Rule: documentation | each page sentence made false is rewritten in the phase that makes it false |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `./le scratch store-trim` exists | `./le scratch store-trim` prints one row per store |
| hourly automatic trim | `test/ui/le-store-trim-answers.ci` |
| page no longer says nothing caps the caches | `grep -n "Nothing caps" docs/contributing/running-commands.md` answers nothing |
| unit tests | `./le job run` over `go test -tags ze_le` for `internal/le/scratch`, `internal/le/session`, `internal/le/go/toolchain`, `internal/le` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path traversal | every removal path is a child of a declared store root; symlinked entries are not followed (`Lstat`), and a store root that is an unexpected symlink is refused, as `Reap` refuses a symlinked session root |
| Input validation | budget values parsed strictly; `bin/le-<name>` names validated with the script's `check_name` rule (letters, digits, dot, underscore, hyphen, no `..`) before removal |
| Resource exhaustion | one child at a time (`run.lock`), overall timeout |

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

## Design Insights
- `cleanTargets` misses `~/.cache/ze/go-cache` whenever `cache/` is a real directory, although its comment says it answers every cache a checkout fills; verify worktrees fill that cache (13G on 2026-10-05). AC-19 fixes it, because the trim derives from the same list.
- `sharedCacheLink`'s comment calls the per-user target "the checkout's shared Go build cache"; for an unlinked checkout it is a second cache. Corrected in this change (stale comment rule).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Limit inside le, triggered at le start, at most hourly | launchd/cron job | owner's choice (Option A, 2026-10-05): works on every machine and in every checkout without per-host setup |
| Detached child process does the walk | synchronous trim in the invoking process; goroutine | a hook call must not stall; a goroutine dies when the le process exits |
| Remove individual `-a`/`-d` entries oldest first | `go clean -cache`; Go's own trim | `go clean` empties everything and races writers (2026-09-04 row); Go's trim has no size limit |
| Age floor (3h) protects in-flight builds | size only | mtime refresh means any use in the last two hours keeps an entry; size-only would delete entries a running build holds |
| One 40G total over the union of the three Go caches, global oldest-first | a budget per cache | owner decision D-1: the disk fills from the sum, and per-cache budgets let three caches hold 120G between them |
| Trim to the budget, no low-water mark | trim to 75% of budget | fewer knobs; at most hourly bounds the churn |
| Session dirs: reap rules plus an age floor | age only; size only | owner: age plus liveness, never size alone; reap is the one liveness rule set |
| Liveness stores skipped when no Claude CLI is visible | always run | a QEMU guest or container sees a different process table and cannot judge host sessions |
| Budgets as env-only private keys | YANG leaf; le config file | le has no YANG or config; precedent `gotoolchain` keys |
| Allocated blocks as the size measure | `st_size` | the class is about a full device; blocks are what `df` counts |

## Known Limitations
- The ambient machine cache (`go env GOCACHE` without the override, `~/Library/Caches/go-build` on macOS) is not trimmed: it is not le's store and Go's own five-day trim covers it (owner decision D-6). `cache-clean` still empties it.
- Out of scope by owner decision D-5, with no spec to follow: the other `tmp/` stores (`tmp/stress-repro/`, `tmp/review/`, `tmp/qemu/`, `tmp/verify-worktree/`, `tmp/kernel/`, `tmp/appliance-build-*`). This is the owner's scope decision, not deferred work.
- Two checkouts trimming the shared per-user cache are not serialised against each other, and each counts it inside its own total (R-3).

## Owner Decisions (Thomas, 2026-10-05)
| # | Question | Decision |
|---|----------|----------|
| D-1 | Go build cache budget | 40G is ONE TOTAL across the checkout, bootstrap and per-user (`~/.cache/ze/go-cache`) caches, not per cache. One LRU pass over the union, oldest mtime first across all three, until the combined size is under 40G. The single entry `ze.le.store.go-cache-budget` names the total |
| D-2 | Lint cache budget | 10G, separate from the 40G |
| D-3 | Ages | fixed constants as drafted: cache-entry floor 3h, named launcher idle 24h, session directory 24h on top of reap, orphan testbin 6h. Not env settings |
| D-4 | Reap's liveness rule | keep the existing rule unchanged |
| D-5 | Other `tmp/` stores | out of scope, no spec |
| D-6 | Machine-wide Go cache | stays with Go's own five-day trim |
| D-7 | The spec | approved for implementation with D-1 to D-6 |

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-le-bounds-its-build-stores.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- Trigger: `run` (`internal/le/register.go`) calls `lescratch.StartStoreTrimWhenDue(storeTrimSpawn())` before `leroot.Dispatch` and never reads the answer. `startTrimWhenDue` (`internal/le/scratch/storetrim.go`) reads `tmp/store-trim/stamp`, takes `stamp.lock` non-blocking, re-reads, writes the stamp by rename and spawns `DetachedTrim` (Setsid, stdin null, output to `last.log`, released, not waited).
- Store list: `cleanTargets` (`internal/le/scratch/cacheclean.go`) has five rows: checkout, shared (per-user, skipped when `cache/` links to it), ambient (`notTrimmed`), bootstrap, lint. Each row carries a kind and a budget group. `./le scratch cache-clean` empties every row, and the trim reads the same list.
- Cache trim: `trimCaches` / `collectGroup` / `removeOldest` (`internal/le/scratch/cachetrim.go`). One oldest-first pass runs over the union of the three Go caches against `ze.le.store.go-cache-budget` (40G), and a separate pass runs over the lint cache against `ze.le.store.lint-cache-budget` (10G). The age floor is 3h. ENOENT counts as freed. ENOTEMPTY counts as contention. The trim never runs `go clean`.
- Liveness stores: `trimLiveness` (`internal/le/scratch/livetrim.go`) covers named launchers (`gotoolchain.NamedLauncherName`, 24h idle, not named in any argv), session directories (`session.Judge`, Reap's rules unchanged, plus 24h) and orphan testbins (6h, pid dead or reused). All three are skipped when no Claude CLI is visible.
- `./le scratch store-trim [background]` (`runStoreTrim`): the foreground run ignores the stamp, and the background child takes `run.lock`. It prints a text report and a `| json` report with kebab-case keys. It exits 1 on a refusal or an error.
- Test runner: `childEnv` gives every test child `ze_le_store_trim=off`, and `le-store-trim-answers` turns the trim back on in its own child.

### Bugs Found/Fixed
- `cleanTargets` never named `~/.cache/ze/go-cache` for an unlinked checkout (13G missed): `TestCleanTargetsNamesSharedCache`.
- `CleanReport.Text` printed `bootstrap/path` glued (name width 9): `TestCleanReportTextStatesEveryOutcome`.
- Closure review: the trim report printed the ambient row with an empty path. `trimStoresScanning` now resolves it with `ambientGoCache`: `TestTrimReportJSON`.
- Closure review: `removeLivenessEntry` counted a directory a peer had already removed as removed by this run. `TestTrimSessionsAgePlusReap/a_directory_a_peer_already_removed...` now covers it.
- Closure review: with no `tmp/session`, or when Judge failed, the testbins row reported an empty store it never looked at. `trimSessions` now answers why, and `trimLiveness` skips the row with that reason: `TestTrimSessionsAgePlusReap/with_no_session_directory...`.
- Closure review: `binaryRoot` accepted a `ze.suffix` such as `pid-9-x`. That named a kept set `testbin-pid-9-x`, which `testbinPID` reads as a throwaway set, so the trim would remove it once pid 9 is dead (AC-12). It is now refused: `TestBinaryRootRefusesASuffixInTheThrowawayNamespace`.
- Closure review: the `cacheclean.go` header still said "The fourth is golangci-lint's" after a fifth cache was added. Corrected.

### Documentation Updates
- `docs/contributing/running-commands.md`: "When the disk is full" (the two durable Go caches, five caches, the trim, budgets, floor, refusal rule, switch, `store-trim`, `last.log`, liveness stores, anchors on `storetrim.go`, `cachetrim.go`, `livetrim.go`, `reap.go`); "Scratch files" (age plus liveness, testbins, the `pid-` suffix refusal, anchored on `binaries.go` `binaryRoot`); "When another session cleans the cache under you" (the trim is the second cause); "Launcher builds..." (named builds reclaimed after a day idle).
- `docs/architecture/testing/runner-architecture.md`: the runner turns the trim off in every test child.
- `ai/INDEX.md`: the `./le scratch` row names cache-clean and store-trim.
- `./le doc check verify` (closure, 2026-10-05): source anchors "checked 2924 code paths, 647 packages, all references valid". The run is red on three drift rows only (`../wiki/command-catalog.md`, `../gh-pages/reference/cli/index.md`, `../gh-pages/llms.txt` disagree on `request bgp rib retain-routes`), which this change does not touch.

### Deviations from Plan
- Files added beyond the plan: `cachetrim.go` and `livetrim.go` split out of `storetrim.go` (phases 3 and 4). The runner gives every test child the trim switch off (R-13). `binaryRoot` refuses a `pid-` suffix (closure review).
- `TrimReport` was renamed to the unexported `trimReport`, because `./le repo check` refuses an exported symbol with no cross-package caller.
- An unreadable `ze.le.store.trim` makes the foreground action exit 1 and still trims every store (phase 3 decision, following AC-13's rule).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Phase 4 wrote the code of `livetrim.go` before its four tests, so those tests never had an observed red against missing code (TDD order broken) | the phase proved each guard discriminates afterwards by eight deliberate breaks (argv guard, platform guard x2, session age, no-Claude guard, pid-reuse comparison, testbin age, launcher dir age, launcher binary age), each red and then restored green | phase 4 handoff, judged at closure | recorded here. The guards are proven, and no rule change is needed: `ai/rules/testing.md` already orders tests first |
| approach | Phase 1 made every `le` invocation a trigger, including the `le` a functional fixture runs in a throwaway checkout, so tree-equality fixtures saw `tmp/store-trim/` appear | the runner owns every test child's environment and is where the switch belongs | phase 5 full ui run (`le-vendor-web-answers`, `le-docvalid-answers` red) | fixed at the source (`childEnv`), R-13 |
| approach | The liveness pass trusted the `testbin-pid-` prefix to mean "throwaway set", and `binaryRoot` let a kept set take that name | the namespace was shared between kept and throwaway sets | closure review | `binaryRoot` refuses a `pid-` suffix |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| limit lives inside le, triggered at start, at most hourly, safe under concurrency | Done | `internal/le/register.go` `run`; `internal/le/scratch/storetrim.go` `startTrimWhenDue` | stamp, flock, re-read |
| Go caches LRU-trimmed to one 40G total across checkout, bootstrap, per-user | Done | `internal/le/scratch/cachetrim.go` `trimCaches`, `removeOldest` | D-1 |
| lint cache bounded | Done | same, `lintCacheBudget` | D-2, 10G |
| named launchers, session dirs, orphan testbins bounded by age plus liveness | Done | `internal/le/scratch/livetrim.go` | D-3, D-4 |
| a trim racing a build produces no false red beyond one retry | Done | 3h floor, ENOENT freed, `isWriterRace` contention | R-1 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRunSpawnsStoreTrimWhenDue`, `TestTrimGoCacheOldestFirst`, `test/ui/le-store-trim-answers.ci` | end to end through the built le |
| AC-2 | Done | `TestStoreTrimDueOncePerHour`, `le-store-trim-answers.ci` | second run rewrites no stamp |
| AC-3 | Done | `TestStoreTrimDueOncePerHour` | 20 re-executed contenders, 1 spawn |
| AC-4 | Done | `TestTrimGoCacheRespectsAgeFloor` | |
| AC-5 | Done | `TestTrimGoCacheOldestFirst` | README, trim.txt, foreign file kept |
| AC-6 | Done | `TestTrimLintCache` | |
| AC-7 | Done | `TestTrimGoCachesAsOneUnion` | symlinked duplicate counted once |
| AC-8 | Done | `TestTrimNamedLaunchers` | |
| AC-9 | Done | `TestTrimNamedLaunchers` | bin/le, platform dirs, bin/ze* kept |
| AC-10 | Done | `TestTrimSessionsAgePlusReap` | |
| AC-11 | Done | `TestTrimSkipsLivenessWithoutSessions` | |
| AC-12 | Done | `TestTrimOrphanTestbins`, `TestBinaryRootRefusesASuffixInTheThrowawayNamespace` | pid reuse, `testbin-<suffix>` kept |
| AC-13 | Done | `TestBudgetParse`, `TestTrimReportJSON` | |
| AC-14 | Done | `TestBudgetParse` | |
| AC-15 | Done | `TestRunSpawnsStoreTrimWhenDue`, `TestStoreTrimSwitch` | |
| AC-16 | Done | `TestTrimReportJSON`, `le-store-trim-answers.ci` | |
| AC-17 | Done | `TestTrimWriterRace` | |
| AC-18 | Done | `TestTrimmedEntryIsAMiss` | |
| AC-19 | Done | `TestCleanTargetsNamesSharedCache` | |
| AC-20 | Done | `TestNamedLauncherMatchesTheShellScript` | |
| AC-21 | Done | `TestTrimGoCachesAsOneUnion` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| 16 unit tests of the TDD table | Done | `internal/le/register_test.go`, `internal/le/scratch/{storetrim,livetrim,cacheclean}_test.go`, `internal/le/go/toolchain/gotoolchain_test.go` | every one PASS in `tmp/session/2026-10-05-69f8d480-7509-4477-a05f-ebac4081d646/scratch/job-close-acs-97157a67.log` |
| `le-store-trim-answers` | Done | `test/ui/le-store-trim-answers.ci` | PASS 13.3s after the closure fixes |
| `TestChildEnvTurnsTheStoreTrimOff`, `TestBinaryRootRefusesASuffixInTheThrowawayNamespace` | Done | `internal/test/runner/runner_exec_util_test.go`, `internal/le/test/functional/functional_test.go` | added beyond the plan |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify (all) | Done | plus the runner, `binaries.go`, runner-architecture.md |
| Files to Create (all) | Done | `storetrim.go`, `cachetrim.go`, `livetrim.go`, both test files, the `.ci` and its fixture |

### Audit Summary
- **Total items:** 21 ACs, 16 planned tests + 1 functional, 5 requirements
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The Go build caches stop growing without limit | live run plus functional test | the live trim in this checkout (`tmp/store-trim/last.log`, 2026-10-05T07:35:33Z): `budget ze.le.store.go-cache-budget=40.0G: 37.3G -> 37.3G`, with checkout 18.4G and shared 18.9G, against 89G plus 13G measured that morning. `le-store-trim-answers.ci` leaves a seeded union under its 1M budget, oldest first across two caches, through the built le binary |
| The lint cache has its own bound | unit plus live | `TestTrimLintCache`; live line `budget ze.le.store.lint-cache-budget=10.0G: 0.7G -> 0.7G` |
| Launchers, sessions and testbins are bounded by age plus liveness, never by size | unit plus live | `TestTrimNamedLaunchers`, `TestTrimSessionsAgePlusReap`, `TestTrimOrphanTestbins`, `TestTrimSkipsLivenessWithoutSessions`; live rows `launchers ... 131 kept`, `sessions ... 11 kept` (nothing yet a day idle and dead) |
| No caller waits and output is unchanged | functional | `le-store-trim-answers.ci`: two runs with identical exit and stdout, and exactly one `run started` |
| A trim racing a build produces no false red beyond one retry | unit | `TestTrimmedEntryIsAMiss` (go build and go vet exit 0 after a trim to zero), `TestTrimWriterRace` (ENOTEMPTY is contention), `TestTrimGoCacheRespectsAgeFloor` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is implemented. The other `tmp/` stores are out of scope by owner decision D-5, and the machine cache by D-6: both are scope decisions, not work left undone | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/le-bounds-its-build-stores-69f8d480-7509-4477-a05f-ebac4081d646.md` (27 files, verdict=clean) |
| `./le spec review check` | clean: "review_gate: OK (24 code files, clean, hashes match ...)" |
| Rounds | 2 |
| Reviewer lenses used | wiring and reachability, logic and edge cases (ENOENT, missing roots, symlinked roots, pid namespace), security (path traversal, symlinks, refusal by name), stale comments and docs, style pass over every changed Go file (no `panic`; loops bounded by the 256 subdirectories, the filesystem, and the 1h context) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A `ze.suffix` starting `pid-<digits>-` named a kept set that `testbinPID` reads as throwaway, so the trim could remove it (AC-12). Root cause: `binaryRoot` let kept and throwaway sets share one namespace | `internal/le/test/functional/binaries.go` `binaryRoot` | refuse the `pid-` prefix (`throwawaySetPrefix`); `TestBinaryRootRefusesASuffixInTheThrowawayNamespace`, red with the guard disabled |
| 2 | ISSUE | The ambient row reported `path: ""`, an empty value standing in for an answer (principles). Root cause: `trimStoresScanning` passed `""` to `cleanTargets` | `internal/le/scratch/storetrim.go` `trimStoresScanning` | resolve with `ambientGoCache`, and append a failure to the skip text; `TestTrimReportJSON` subtest, red when `""` is restored |
| 3 | ISSUE | `removeLivenessEntry` counted ENOENT as `entries-removed`, a removal this run did not make; `removeOldest` does not | `internal/le/scratch/livetrim.go` `removeLivenessEntry` | ENOENT is gone but not counted; new subtest red with the count restored |
| 4 | ISSUE | With no `tmp/session`, or with Judge failing, the testbins row reported an empty store it never walked | `internal/le/scratch/livetrim.go` `trimSessions`, `trimLiveness` | `trimSessions` answers why, and the testbins row skips with it; new subtest red with the skip disabled |
| 5 | ISSUE | The header comment still said "The fourth is golangci-lint's" in a five-cache list (stale-comments) | `internal/le/scratch/cacheclean.go` | now reads "fifth" |

NOTEs (do not block): `reap` now scans processes before `judge` checks the session root. `Reap` returns earlier on a missing root through `newReapScope`, so production never makes that ps call; only a root that vanishes between the two checks pays one. R-3: two checkouts each count the shared per-user cache in their own total and may trim it at once. That is accepted at spec approval as a Known Limitation, and over-trimming costs only misses. The prefix `testbin-pid-` is spelled in both `binaryRoot` and `livetrim.go` `testbinPIDPrefix`, because the scratch package does not import the functional harness. The `binaryRoot` refusal now keeps the two in agreement. `./le repo check` and `./le commit audit` findings (`gr.go` RunGRPlugin, three WEAKENED BGP tests) are in another session's uncommitted files, outside this diff.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/scratch/storetrim.go`, `cachetrim.go`, `livetrim.go`, `storetrim_test.go`, `livetrim_test.go` | yes | `git status` lists each as untracked, and each test file ran in `job-close-acs` |
| `test/ui/le-store-trim-answers.ci`, `internal/test/fixture/ui_fixture_le_store_trim_answers.go`, `register_le_store_trim_answers.go` | yes | `./le test ui le-store-trim-answers`: `PASS 214 le-store-trim-answers` (13.3s, `tmp/session/2026-10-05-69f8d480-7509-4477-a05f-ebac4081d646/scratch/job-close-ui-7b737b92.log`) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-21 | each AC's test passes | `job-close-acs-97157a67.log`: `--- PASS` for all 16 TDD tests plus `TestChildEnvTurnsTheStoreTrimOff` and `TestBinaryRootRefusesASuffixInTheThrowawayNamespace`, `ok` in all five packages |
| AC-1, AC-2, AC-16 | end to end through the built le | `PASS le-store-trim-answers` after the closure fixes |
| closure fixes | each new assertion discriminates | with the four fixes broken, `job-close-break*` logs show the matching `--- FAIL` lines; restored, `job-close-green` exit 0 and golangci-lint `0 issues` over scratch and functional |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| any `le` command through `run` | `test/ui/le-store-trim-answers.ci` | yes: the fixture runs the built le twice and asserts one background run (`leStoreTrimAwaitChild`) and an unchanged stamp on the second run. Phase 5 recorded the red with the call removed from `run` |
| `./le scratch store-trim` | `test/ui/le-store-trim-answers.ci` | yes: `leStoreTrimForeground` checks the text report and the json report, with one row per store |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestTrimmedEntryIsAMiss` refresh check (Go 1.27.1) |
| A-2 | confirmed | golangci-lint v2.13.1 `markUsed`, `mtimeInterval = 1 * time.Hour` |
| A-3 | confirmed for go build and go vet | `TestTrimmedEntryIsAMiss`. For golangci-lint and staticcheck the evidence is the same `GetFile` copy (A-2), read rather than executed |
| A-4 | confirmed | `TestTrimNamedLaunchers/a_process_started_from_a_named_launcher_shows_its_path` |
| A-5 | confirmed | no Claude in `internal/le/test/qemu`, no `--pid=host`; `TestTrimSkipsLivenessWithoutSessions` |
| A-6 | confirmed | `TestTrimGoCacheOldestFirst/allocated_size_matches_du` |
| A-7 | confirmed by owner | D-1 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "Nothing caps" removed | `grep -c "Nothing caps" docs/contributing/running-commands.md` gives 0 | yes |
| trim, budgets, report, exit codes | `storetrim.go` `trimStoresScanning`, `trimReport.verdict`; `cachetrim.go` `removeOldest` | yes, read at closure |
| testbins, `pid-` refusal, missing session root | `livetrim.go` `trimLiveness`, `trimSessions`; `binaries.go` `binaryRoot` | yes |
| source anchors | `./le doc check verify`: "all references valid" | yes |
| row 12 architecture: No | `docs/architecture/core-design.md` carries no trim text; `verify-freshness-scope.md` unaffected (only the `sharedCacheLink` comment changed); `docs/architecture/testing/ci-format.md`, the `// Design:` page of `binaries.go`, says nothing of `ZE_SUFFIX` or testbin naming (`grep -n -i "suffix\|testbin"` finds only the `.src`/`.conf` and ghost-text lines), so the `pid-` refusal makes no sentence there false | yes |
