# Spec: le-build-cache-cap

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Release-independent le build cache cap work; historical findings do not establish a current product defect. |
| Scope | tooling |
| Depends | spec-le-one-go-cache-per-host |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decisions, recorded here because they approve this feature (they were
rows of `ai/META-DECISIONS.md` until 2026-10-22, when that file was
kept to meta decisions only):

| Id | Date | Decision | Thomas's words |
|----|------|----------|----------------|
| O-1 | 2026-10-22 | The Go build cache gets a size cap that every `./le` run enforces | "yes" to "Cap the Go build-cache size, enforced on every `./le` run?" |
| O-2 | 2026-10-22 | The trim limits the ambient Go cache (`~/.cache/go-build`) as well as le's own caches, superseding D-6 of the closed `spec-le-bounds-its-build-stores`; le's caches keep their 40G budget (A-1, AC-10, AC-11). Superseded by O-3: there is no separate ambient cache left to budget | "The trim should limit both cache." and "40 gig is right" |
| O-3 | 2026-10-24 | One Go build cache per host: the per-user `~/.cache/ze` (`$XDG_CACHE_HOME/ze`), never `~/Library/Caches` and never a second copy in the checkout. Every `./le` action, every verify worktree, the bootstrap and lint caches, and every tool outside le write there. Outside le, `go env -w GOCACHE` names it, and gopls, golangci-lint and staticcheck are pointed at it by shell variables (`GOPLSCACHE`, `GOLANGCI_LINT_CACHE`, `STATICCHECK_CACHE`) set in both zsh and NuShell, not by links from their default folders. On the Linux host `~/.cache` is its own drive, which is why the location matters | "We want to use the cache in the .cache folder because on another host it's a different drive", "We should not have anything in the library. we want to remove the duplication", "We also need to support NuShell", and "shell" to links or shell variables |
| O-4 | 2026-10-24 | When any part of O-3 is wrong on a host, le prints what is wrong and repairs it itself: the checkout's `cache/` link, the `go env` GOCACHE setting, and the shell variable lines for zsh and NuShell | "when le cache is wrong, print it and fix itself" |

O-3 and O-4 rework G-3, A-1, A-5, AC-10, AC-11 and AC-14 before implementation:
the ambient budget is dropped because the ambient cache stops existing.

O-3 and O-4 are implemented by `spec-le-one-go-cache-per-host`, which this spec
depends on. There the owner replaced O-3's shell variables with `le exec` and
shims in `~/bin` (its O-2, O-3), and narrowed O-4 so le prints, never writes,
shell lines. Those rows, not O-3 and O-4 here, are current.

O-1: the Go build cache gets a size cap that every `./le` run enforces. Background, handover 17
(`plan/handover/17-exabgp-6-linux-continue.md`, "Owner decisions" row 1): the
Mac disk filled three times on 2026-10-21/22, because the checkout, shared and
ambient Go caches regrew to about 85 GB within hours while many sessions built,
and the safe `./le scratch clean` cannot free young entries by design.

A cap already half exists, and this spec closes the gap between it and O-1
rather than building a second one (`ai/rules/no-layering.md`). Today le starts a
detached store trim at most once an hour, holds the checkout, shared and
bootstrap Go caches to one 40G total and the lint cache to 10G, never removes an
entry younger than a fixed three-hour floor, and leaves the ambient cache alone
(owner decision D-6 of the closed `spec-le-bounds-its-build-stores`). Four gaps
let the caches reach 85 GB under that:

| Gap | Why it lets the cache overrun |
|-----|-------------------------------|
| G-1 The cadence is hourly | Between two trims nothing reacts to growth; tens of GB land inside one hour on a busy machine |
| G-2 The floor is a fixed three hours | Everything built in the last three hours is untouchable whatever the budget, so a busy machine's cache is mostly young and the trim reports "budget unmet" while the disk fills |
| G-3 The ambient cache is outside the cap | `~/Library/Caches/go-build` (macOS) or `~/.cache/go-build` (Linux) is one of the three caches that regrew; only Go's own five-day trim bounds it |
| G-4 An unmet cap is only a line in `tmp/store-trim/last.log` | Nobody reads it, and the line names no holder and no fix |

Goal: on every `./le` invocation, at a cost of one small file read and one
`statfs` per cache device, le decides whether the capped caches may be over
their budget and, when they may be, starts the trim at once (rate-limited to one
start per ten minutes) instead of waiting for the hour. The trim may remove any
entry no running build can still read, which it bounds from the process table
instead of from a fixed three hours, and the ambient cache joins the cap with a
budget of its own. When the cap cannot be met, the report says how far over it
is, which process holds the remainder, and the exact fix.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the four questions every change answers
  → Constraint: no product code may import `internal/le`; the cap lives entirely in `internal/le/scratch` and reads disk space through the existing core leaf `internal/core/diskspace` (`Free(path)`), which product code already shares.
  → Constraint: env vars register with `env.MustRegister` from the owning package; the new budget key is registered beside `goCacheBudgetEntry` in `internal/le/scratch/storetrim.go`, `Private: true` like its siblings.
- [ ] `ai/doors/le-tooling.md` - le's boundaries and its outside parties
  → Constraint: "A before-dispatch hook never prints, never waits, returns nothing" (`leroot.RegisterBeforeDispatch`). The per-run check MUST NOT print, MUST NOT walk a cache, MUST NOT fork (`go env` included), and MUST NOT change any command's exit code; every hook call runs it.
  → Constraint: every concurrent agent session is an outside party: a trim that removes an entry a running build still reads breaks another session's build with `cache entry not found`, which reads as a code defect there.
  → Constraint: the owner's macOS and Linux machines are both targets; the process-start reading must work through `ps -o etime=` on macOS (`procscan.StartTime`), and any Linux-only `/proc` shortcut needs a portable twin.
- [ ] `docs/contributing/running-commands.md`, "When another session cleans the cache under you" and "When the disk is full" (the store-trim table and the `cache-clean` rows)
  → Decision: the store trim's union budget over checkout, shared and bootstrap (D-1, `ze.le.store.go-cache-budget`, default 40G) and the lint budget (D-2, `ze.le.store.lint-cache-budget`, 10G) stay as they are; this spec adds a third budget for the ambient cache and changes when and how far a trim may go.
  → Constraint: the page states the three-hour floor and the hourly cadence as facts in four places (both cleaners, the store-trim paragraph, the safe `cache-clean` step); each is rewritten in the same phase that changes the code (`ai/rules/documentation.md`).
  → Constraint: the kernel caches under `~/.cache/ze/runtime-kernel/` and `installer-kernel/` are never a target; `cleanTargets` already marks `holdsKernel`, and the new ambient budget must not bypass that skip.
- [ ] `ai/rules/user-facing-errors.md` - every message names the problem, why, and the fix
  → Constraint: the unmet-cap line names the bytes over budget, the budget key, the cutoff instant, the process that set it (pid, command, job label), and the exact commands that resolve it.
- [ ] `ai/rules/config.md` - env var registration
  → Constraint: a build-host knob is an env var, never YANG; the value grammar is the existing `parseBudget` (whole number with `M` or `G`, `1M` to `1048576G`), so the new key adds no second parser.

### RFC Summaries (Scope: protocol)
N-A: tooling, no protocol.

**Key insights:** (minimal context to resume after compaction)
- The cap exists (`trimStores`, hourly, 40G union, 10G lint, 3h floor, ambient skipped under D-6). O-1 is met by four changes, not a new mechanism: a per-run due check on a ledger plus `statfs`, a process-bounded cutoff instant in place of the fixed floor, an ambient budget, and an unmet report with holder and fix.
- Go marks an entry used by refreshing its mtime only when the mtime is at least one hour old (`mtimeInterval`, `markUsed`, `cmd/go/internal/cache/cache.go`, Go 1.27.2), and one go command can read an output it looked up at its start much later (a link late in `go test ./...` reads archives looked up in its first minute). So an entry is safe to remove only when its mtime is older than the start of every live cache user minus one hour minus a margin. That is an INSTANT, not a moving floor: a process that starts after the scan can only push mtimes forward.
- The per-run check must not run `go env GOCACHE`; the trim child records every capped cache's path and device in the ledger, and the per-run check reads only that.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/scratch/storetrim.go` (689L) - `storeTrimBeforeDispatch` is registered from `register.go` via `leroot.RegisterBeforeDispatch`; `startTrimWhenDue` reads `tmp/store-trim/stamp`, and when it is older than `trimInterval` (1h) takes `stamp.lock`, re-reads it, writes it, and spawns `le scratch trim background` detached (`detachedTrim`). `trimStoresScanning` resolves the per-user target and the ambient path (one `go env GOCACHE` fork), builds a `cachePass{floor: cacheEntryAgeFloor}`, calls `trimCaches`, then the liveness stores and `test-temp`. Budget keys `ze.le.store.go-cache-budget` (40G) and `ze.le.store.lint-cache-budget` (10G), parsed by `parseBudget`. `trimReport`, `BudgetTrim{Key, Budget, SizeBefore, SizeAfter, EntriesRemoved, Unmet, Refused, Error}`, `StoreTrim`.
- [ ] `internal/le/scratch/cachetrim.go` (454L) - `cacheEntryAgeFloor = 3h`; `trimCaches` skips `notTrimmed` targets with `machineCacheSkip` ("left to Go's own five-day trim"); `collectGroup` walks each cache's 256 subdirectories (`walkGoFormatCache`); `removeOldest` sorts by mtime, removes oldest first until at budget, stops at the first entry younger than the floor and sets `Unmet` from `youngerThanFloor`. It removes by the mtime the walk saw; it does not re-stat before removal. ENOENT counts as freed, ENOTEMPTY as contended.
- [ ] `internal/le/scratch/cacheclean.go` (765L) - `cleanTargets` is the one store list (checkout, shared, ambient with budget `notTrimmed`, bootstrap, lint), with `holdsKernel` and same-directory skips; `cleanOldCacheEntries` is the safe `cache-clean` default over the same `cachePass` floor; `measureClean` reads free space with `diskspace.Free`; `ambientGoCache` forks `go env GOCACHE` with GOCACHE removed from the environment.
- [ ] `internal/le/scratch/cacheuse.go` (151L) - `cacheReaders` (go, compile, link, vet, asm, cgo, cover, staticcheck, golangci-lint) plus compiled `*.test` binaries plus job-registry holders form `cacheUsers`, used only by the `cache-clean all` refusal. gopls is deliberately not a reader.
- [ ] `internal/le/procscan/procscan.go` - `Scan` reads `/proc` or one `ps`; `Process.StartedAt` is filled by the ps scan for every process but by the `/proc` scan only for a Claude CLI; `StartTime(pid)` measures one pid through `ps -o etime=` and answers the zero time for "unknown".
- [ ] `internal/core/diskspace/diskspace.go` - `Free(path)` answers available bytes for the device holding a path, portable across Linux, macOS and OpenBSD.
- [ ] `internal/le/test/qemu/guest_linux.go` - the QEMU guest builds with `GOCACHE` = the checkout's bootstrap cache over 9p, so guest go processes read a host cache while invisible to the host process table; the host sees the driving `le` process and `qemu-system-*`.
- [ ] `/usr/local/go/src/cmd/go/internal/cache/cache.go` (Go 1.27.2) - `mtimeInterval = 1h`, `trimInterval = 24h`, `trimLimit = 5 days`; `markUsed` is called on the `-a` index on every lookup and on the output file in `GetFile`; Go's own `Trim` removes entries unused for `trimLimit + mtimeInterval`, at most daily, with no size bound.
- [ ] `test/ui/le-store-trim-answers.ci` and `internal/test/fixture/ui_fixture_le_store_trim_answers.go` - the built-le proof of the hourly trim, seeding only entries far older than 3h, budgets of 1M, throwaway checkout through `ZE_REPO_ROOT`, `XDG_CACHE_HOME`, `CLAUDE_CONFIG_DIR`.

**Behavior to preserve:**
- The hourly stamp cadence stays as the lower bound: a stamp older than one hour still starts a trim whatever the ledger says.
- `ze.le.store.trim=off` still stops every automatic start, the new early start included; the functional runner still sets it for every test child.
- The before-dispatch hook still never prints, never waits, never changes an exit code; failures still go to `tmp/store-trim/last.log`.
- One global oldest-first order across the union of checkout, shared and bootstrap (D-1); the lint cache's own budget (D-2).
- The kernel caches and the patched GOROOT trees under `cache/stdpatch/` are never trimmed.
- `cache-clean all` keeps its process refusal unchanged; `store-trim` exit codes keep their meaning (1 for a refused key, path or switch; 0 for an unmet budget or a contended entry).
- `| json` output keys stay kebab-case; existing keys keep their names.

**Behavior to change:**
- G-1: the due rule gains a per-run ledger check (AC-1 to AC-4).
- G-2: the fixed three-hour floor is deleted and replaced by a cutoff instant derived from the live cache users (AC-5 to AC-9), in both the store trim and the safe `cache-clean` default, because both read the same `cachePass`.
- G-3: the ambient cache gets budget `ze.le.store.ambient-cache-budget` (AC-10, AC-11), superseding D-6 under O-1 and O-2 (see Q-1 in the report and A-1).
- G-4: an unmet budget names the holder and the fix in the report, in `last.log`, and in the ledger (AC-12, AC-13).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Every `le` (or `ze le`) invocation, hooks included, through le's root handler calling every registered before-dispatch hook; the store trim's is `storeTrimBeforeDispatch`.
- `./le scratch trim` (foreground), `./le scratch trim background` (the detached child), `./le scratch clean` (safe default).

### Transformation Path
1. Per-run due check (in the invoking process, under a millisecond): read `ze.le.store.trim`; read `tmp/store-trim/stamp` (exists today); read `tmp/store-trim/ledger.json` (new, at most a few KB); for each distinct device the ledger names, one `diskspace.Free` call on the recorded path; compute each budget group's growth bound; decide due or not.
2. When due: today's `stamp.lock` protocol, re-check under the lock, write the stamp, spawn the detached child.
3. Detached child: one process scan; measure the start of every cache user; compute the cutoff instant; walk the capped caches (today's walk); remove oldest first while over budget and older than the cutoff, re-stating each entry immediately before removal; write the report to `last.log` and the ledger (sizes, budgets, unmet bytes, holder, cache paths, device free bytes) by write-then-rename.
4. Foreground `store-trim` and `cache-clean` print the same report rows, holder line included.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le invocation to detached trim child | `detachedTrim` (Setsid, existing) | No |
| Trim child to the next le invocation | `tmp/store-trim/ledger.json`, written by rename, read without a lock | No |
| le to other users of the caches | the process table (`procscan.Scan`, `procscan.StartTime`) and the job registry (`job.Holders`) | No |
| le to the go command | Go's cache layout and mtime contract (`markUsed`, `mtimeInterval`) | No |

### Integration Points
- `cleanTargets` (`cacheclean.go`) - the one store list; the ambient row's budget changes from `notTrimmed` to a new `ambientCacheBudget` group, and `budgetEntry` answers its key.
- `cachePass` (`cachetrim.go`) - its `floor` duration is replaced by a `cutoff` instant plus the holder that set it; `removeOldest` and `cleanOldCacheEntries` read the cutoff.
- `cacheUsers`/`cacheReaders` (`cacheuse.go`) - the reader set is reused, and the cutoff set adds le personalities and `qemu-system-*` launchers (A-3) without changing what `cache-clean all` refuses on.
- `trimReport`/`BudgetTrim` (`storetrim.go`) - gain the cutoff and holder fields.
- `diskspace.Free` (`internal/core/diskspace`) - the per-run device reading.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The early start reuses `startTrimWhenDue`'s lock and spawn; no second trim path |
| No unintended coupling (components stay isolated) | Yes | Everything stays in `internal/le/scratch`; `internal/core/diskspace` is a core leaf already imported there |
| No duplicated functionality (extends existing, does not recreate) | Yes | One store list (`cleanTargets`), one budget parser (`parseBudget`), one walk, one removal loop; the fixed floor is deleted, not kept beside the cutoff |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling; no wire path |
| Registration over hardcoding, outbound | Yes | The env key registers with `env.MustRegister` in its own package; no core list learns it |
| Registration over hardcoding, inbound | Yes | Searched: `cleanTargets` (the single store list, edited in place), `budgetEntry` (answers each group's key; gains one arm for the new group, the existing derivation point), `cacheReaders` (reused). No help string or completion table names budgets; `ai/INDEX.md` row 368 derives from the registered `ShortHelp` and is updated with it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | O-1 supersedes D-6, so the ambient cache joins the cap | O-1's background names "checkout, shared and ambient" as the caches that regrew; D-6 predates it (2026-10-05 versus 2026-10-22) | The ambient budget is dropped and the ambient row stays a skip; AC-10 and AC-11 go | The owner, Q-1 | validated 2026-10-22: Thomas, "The trim should limit both cache." and "40 gig is right" (O-2) |
| A-2 | Every live process that can read a Go cache entry is visible in this user's process table as a cache reader, a job holder, a le personality or a `qemu-system-*` process | `cacheuse.go` header; QEMU guest builds run under the le that drives `guestRun` (`guest_linux.go`); Colima container builds read the host-mounted `XDG_CACHE_HOME` only when le starts them | A container or guest build not started by le could lose an entry it looked up and fail with `cache entry not found` once | Grep every `docker run` and VM mount of a cache path in `internal/le` and `internal/test` during Phase 3; record each launcher | unvalidated |
| A-3 | A le or ze-le process's start bounds every build it drives, inside a guest or a container | Each driver is a long-lived parent of the build it waits on (`guestRun`, interoplab builds) | A driver that exits while its build continues detaches the build from the bound | Same grep as A-2, plus reading each driver's wait | unvalidated |
| A-4 | Go never reads a cache output whose lookup was made before the go process started; a go command's lookups all happen inside its own lifetime | `cmd/go/internal/cache`: lookups and `markUsed` happen in the process that reads the file | A tool caching a path across processes (none known) could read a removed file | Grep `GOCACHE` consumers in gopls and golangci-lint docs; gopls keeps its own cache (`cacheuse.go`) | unvalidated |
| A-5 | `go env GOCACHE` honours a `GOENV` file holding `GOCACHE`, so the functional fixture can point the ambient cache at a throwaway tree on macOS, where `XDG_CACHE_HOME` is ignored | Go's `go env -w` writes `GOENV`; `withoutGoCache` drops only the GOCACHE environment variable | The fixture cannot redirect the ambient cache on macOS; it then runs that part with `option=needs-linux` | A first fixture run on the Mac | unvalidated |
| A-6 | `diskspace.Free` costs one `statfs` syscall, microseconds, and never blocks on a healthy local filesystem | `internal/core/diskspace/diskspace.go` | A per-run cost on a stalled network filesystem; see R-5 | `BenchmarkPerRunCapCheck` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A trim removes an entry a running build looked up before the trim's scan and reads after it, failing that build with `cache entry not found` | A session reports the error within minutes of a trim in `last.log` | The cutoff is the oldest live cache user's start minus `mtimeInterval` minus a five-minute margin, never later than now minus the same; the process start is read to the second (`StartTime`) |
| R-2 | The residual race: a go process stats an entry (old mtime) in the microseconds between the trim's re-stat and its unlink | The same error, with no long-running holder in the report | Accepted: Go's own `Trim` carries the same race at its five-day cutoff; the re-stat makes the window one syscall wide; the doc says a retry clears it |
| R-3 | A long-lived cache reader (a stuck `go test`, a forgotten `./le job run`) pins the cutoff and the cap stays unmet for hours | The unmet line names the same pid run after run | The unmet line names that pid, its command, its job label and its age, and the fix names it (AC-12); nothing is removed past it, by design |
| R-4 | The process table cannot be read, or a cache user's start cannot be measured | `Scan` or `StartTime` answers an error or the zero time | The trim refuses the young-entry removal for every capped cache by name and exits 1 from the foreground action; it never assumes a floor (`ai/rules/principles.md`, a silent default) |
| R-5 | The per-run check slows every hook call: a stalled filesystem under a cache path makes `statfs` block | Hooks go slow; `./le ai hooks` timing | The check reads only devices the ledger names; the cost AC bounds the calls; a `statfs` error makes the run not due and is logged once per hour through the stamp, never retried per call |
| R-6 | The early start fires on every call while the cap is unmet, walking the caches every ten minutes | `last.log` shows a trim every ten minutes with the same unmet line | The ten-minute minimum interval bounds it; the walk runs in the detached child |
| R-7 | Free space that drops for another reason (docker, `tmp/`) makes the growth bound over-estimate, starting extra trims | Early trims that remove nothing | Over-estimation is the safe direction; bounded by the ten-minute interval; a trim under budget removes nothing |
| R-8 | Two checkouts trim the shared and ambient caches at once | ENOENT counts in both reports | Already handled: ENOENT counts as freed, removal is idempotent; each run computes its own cutoff from the same process table |
| R-9 | Free space rises (another session deleted), hiding real cache growth from the growth bound | A trim at the hour finds the cache far over budget | The hourly stamp stays as the lower bound (Behavior to preserve) |
| R-10 | The ledger is missing, partial or from an older le | Parse error | A missing or unreadable ledger is not due by the ledger and falls back to the stamp rule only; the trim child rewrites it; it is written by rename so a reader never sees half a file |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Other sessions' builds on the same machine fail with `cache entry not found` (R-1, R-2), or every le call and hook slows (R-5). No shipped binary changes |
| How is it reverted? | A single commit revert; `ze.le.store.trim=off` stops every automatic start at once |
| Who else touches this path? | the le launcher work (`spec-le-built-from-head` (closed; `docs/contributing/running-commands.md` carries its design), which superseded spec-le-shared-build-launcher on 2026-10-25), whenever it starts (the launcher trim in `livetrim.go` runs in the same child); any session editing `internal/le/scratch` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Any `le` invocation (before-dispatch) with a ledger over budget, stamp under an hour old | → | `startTrimWhenDue` early-start branch | `TestLeInvocationSpawnsTrimWhenLedgerIsOverBudget` in `internal/le/scratch/storetrim_wiring_test.go` |
| The built `le` binary, a cheap command, inside the hour | → | per-run check, detached child, ledger | `test/ui/le-store-trim-cap.ci` |
| `./le scratch trim` foreground | → | cutoff from the process table, ambient budget, unmet line | `test/ui/le-store-trim-cap.ci` |
| `./le scratch clean` (safe default) | → | `cleanOldCacheEntries` with the cutoff instant | `TestCacheCleanDefaultUsesTheCutoffNotAFixedFloor` in `internal/le/scratch/cacheclean_safe_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A le invocation, stamp under one hour old and at least ten minutes old, ledger recording a budget group whose recorded size plus the drop in free space on its device since the ledger was written exceeds that group's current budget | The invocation spawns the background trim exactly once and writes the stamp; its own output and exit code are those of the command asked for |
| AC-2 | Same as AC-1 but the stamp is under ten minutes old | No spawn, no file written |
| AC-3 | A le invocation whose ledger is under budget for every group and free space has not dropped by more than the headroom of any group | No spawn; the check reads exactly the stamp, the ledger and one `statfs` per distinct device the ledger names; it opens no cache directory and forks no process |
| AC-4 | The budget env value is lowered between two invocations inside the hour, below the ledger's recorded size | The second invocation spawns the trim (the comparison uses the budget read now, not the one recorded) |
| AC-5 | A trim with no live cache user except itself | Entries whose mtime is older than now minus one hour five minutes are removable; entries newer are kept |
| AC-6 | A trim with a live cache user that started at instant S | No entry whose mtime is at or after S minus one hour five minutes is removed; older entries are removed oldest first until the group is at its budget |
| AC-7 | An entry older than the cutoff at walk time whose mtime a go process refreshes before the trim reaches it | The entry is kept: the trim re-stats it immediately before removal |
| AC-8 | The process scan fails, or a cache user's start time cannot be measured | No capped cache loses an entry; each capped store row is refused with the reason and the fix; the foreground action exits 1; the background child writes the same to `last.log` |
| AC-9 | Cache users counted for the cutoff | The `cacheReaders` programs, compiled `*.test` binaries, job-registry holders, le and ze-le processes, and `qemu-system-*` processes, excluding the trim process itself; `cache-clean all`'s refusal set is unchanged |
| AC-10 | `ze.le.store.ambient-cache-budget` unset | The ambient cache is trimmed to the default `20G`, oldest first, past the cutoff, as its own group, never mixed with the union; its row reports sizes like the others |
| AC-11 | `ze.le.store.ambient-cache-budget` set to a value `parseBudget` refuses (`20`, `0G`, `1048577G`) | The ambient row is refused by key and value; every other cache is still trimmed; exit 1 |
| AC-12 | A group still over budget after every entry older than the cutoff is gone | The budget line reads, in one line: the size, the budget and its key, the bytes kept, the cutoff instant, the holder (pid, command line shortened, job label when it holds one, age), and the fix: wait for that process, raise the key, or `./le scratch clean all` when no build runs. Exit 0 |
| AC-13 | AC-12 happened in the background child | The ledger records the unmet group with the holder; `last.log` carries the same line; the next invocation after ten minutes starts a trim again (AC-1) |
| AC-14 | The kernel cache or a path holding it resolves as the ambient cache | The ambient row is a kernel-cache skip, as today; nothing in it is removed |
| AC-15 | `ze.le.store.trim=off` | No early start and no hourly start; a foreground `./le scratch trim` still trims |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEarlyStartWhenLedgerGrowthExceedsBudget` | `internal/le/scratch/storetrim_test.go` | AC-1, with an injected free-space reader and clock | |
| `TestNoEarlyStartWithinTenMinutes` | `internal/le/scratch/storetrim_test.go` | AC-2 | |
| `TestPerRunCheckOpensNoCacheDirectoryAndForksNothing` | `internal/le/scratch/storetrim_test.go` | AC-3: counts reads and `statfs` calls through injected readers; a tripwire walk and spawn fail the test if called | |
| `TestEarlyStartReadsTheBudgetNow` | `internal/le/scratch/storetrim_test.go` | AC-4 | |
| `TestMissingOrCorruptLedgerFallsBackToTheStamp` | `internal/le/scratch/storetrim_test.go` | R-10 | |
| `TestCutoffWithNoCacheUser` | `internal/le/scratch/cutoff_test.go` | AC-5 | |
| `TestCutoffIsOldestUserStartMinusRefreshAndMargin` | `internal/le/scratch/cutoff_test.go` | AC-6, AC-9: injected processes with start times; self excluded | |
| `TestCutoffRefusesWhenAStartIsUnknown` | `internal/le/scratch/cutoff_test.go` | AC-8 | |
| `TestRemoveOldestKeepsAnEntryRefreshedAfterTheWalk` | `internal/le/scratch/cachetrim_test.go` (new) | AC-7: the walk sees an old mtime, the injected re-stat a fresh one | |
| `TestRemoveOldestStopsAtTheCutoffAndNamesTheHolder` | `internal/le/scratch/storetrim_test.go` | AC-6, AC-12: the line text, key, holder and fix | |
| `TestAmbientCacheHasItsOwnBudget` | `internal/le/scratch/storetrim_test.go` | AC-10: the ambient entries never compete with the union's | |
| `TestAmbientBudgetRefusedAloneByKey` | `internal/le/scratch/storetrim_test.go` | AC-11 | |
| `TestAmbientKernelCacheStillSkipped` | `internal/le/scratch/cacheclean_kernel_test.go` | AC-14 | |
| `TestUnmetGroupIsRecordedInTheLedger` | `internal/le/scratch/storetrim_test.go` | AC-13 | |
| `TestCacheCleanDefaultUsesTheCutoffNotAFixedFloor` | `internal/le/scratch/cacheclean_safe_test.go` | The safe clean removes a 2h-old entry when no user is older, keeps it when one is | |
| `TestFullEmptyRefusalSetUnchanged` | `internal/le/scratch/cacheclean_safe_test.go` | AC-9, second half | |
| `TestStoreTrimOffStopsTheEarlyStart` | `internal/le/scratch/storetrim_test.go` | AC-15 | |
| `BenchmarkPerRunCapCheck` | `internal/le/scratch/storetrim_test.go` | A-6: the check over a real ledger and real `statfs` stays under 1ms | |

The existing tests that assert the fixed floor (`TestTrimGoCacheRespectsAgeFloor` in `storetrim_test.go`, `TestCleanOldCacheEntriesKeepsTheYoung` and `TestCleanCachesDefaultKeepsYoungEntriesInEveryTarget` in `cacheclean_safe_test.go`, and every other use of `cacheEntryAgeFloor` in `storetrim_test.go`, `cacheclean_safe_test.go` and `cacheclean_kernel_test.go`) are rewritten to the cutoff, never deleted; each rewrite keeps its scenario and changes only the bound it asserts, and is recorded in the weakened-test ledger if the hook asks.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze.le.store.ambient-cache-budget` | `1M` to `1048576G` | `1M`, `1048576G` | `0M`, `0G`, bare `20` | `1048577G` |
| Early-start interval | fixed 10 minutes | stamp exactly 10m old starts | 9m59s does not | N/A |
| Cutoff margin | fixed 5 minutes past `mtimeInterval` | entry at cutoff minus 1s removable | entry at cutoff kept | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-store-trim-cap` | `test/ui/le-store-trim-cap.ci` + `internal/test/fixture/ui_fixture_le_store_trim_cap.go` | Through the built le: (1) a first cheap command under budget `1G` writes the stamp and the ledger; (2) inside the hour the fixture lowers `ze.le.store.go-cache-budget` to `1M`, waits past ten minutes by backdating the stamp file, runs a cheap command, and the child trims (AC-1, AC-4); (3) the fixture starts its own long-lived process named `go` (a copied `sleep`), seeds entries aged 10h, 5h, 2h, 30m and 0 in the checkout, shared and ambient (through `GOENV`, A-5) caches, runs `le scratch trim | json`, and asserts: every entry with mtime at or after the reported cutoff remains, the reported cutoff is no later than the fake reader's start minus 65 minutes, older entries went oldest first, the ambient row has its own budget line, and the unmet line names a holder, the key and `cache-clean all`; exit 0 | |
| `le-store-trim-answers` (existing) | `test/ui/le-store-trim-answers.ci` | Unchanged scenario still passes; its header names the 3h floor and is reworded to the cutoff | |

Discrimination (`ai/rules/interop-and-goal-validation.md`): with the per-run check reverted, step 2 sees no trim; with the fixed floor restored, the report carries no cutoff or holder and step 3 fails; with the ambient row back to `notTrimmed`, its budget line is absent. Each is forced once and recorded in the spec at implementation.

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/scratch/storetrim.go` - due rule with ledger and growth bound, ten-minute interval, ambient budget entry, report fields, ledger write
- `internal/le/scratch/cachetrim.go` - delete `cacheEntryAgeFloor`; `cachePass` carries the cutoff instant and holder; `removeOldest` re-stats before removal and writes the unmet line; drop `machineCacheSkip`
- `internal/le/scratch/cacheclean.go` - ambient row's budget group; `budgetEntry` for it; `cleanOldCacheEntries` takes the cutoff; header comment
- `internal/le/scratch/cacheuse.go` - expose the reader classification for the cutoff set without widening the full-empty refusal
- `internal/le/scratch/storetrim_test.go`, `cacheclean_safe_test.go`, `cacheclean_kernel_test.go`, `storetrim_wiring_test.go` - the tests above, and the floor tests rewritten
- `test/ui/le-store-trim-answers.ci` - header wording
- `docs/contributing/running-commands.md` - "When another session cleans the cache under you", "When the disk is full" (store-trim paragraph and table, safe `cache-clean` step, the `kept ... younger than 3h0m0s` sample rows)
- `ai/INDEX.md` row for `./le scratch` and the `ShortHelp` it derives from in `internal/le/scratch/register.go`, if they name three hours or "once an hour" only
- `AGENTS.md` table row on the full disk - only if its wording names the floor (it names `cache-clean`, verified at implementation)

## Files to Create
- `internal/le/scratch/cutoff.go` - cutoff instant from the process scan, the job holders and the measured starts
- `internal/le/scratch/ledger.go` - the ledger's fields, its rename write, its tolerant read
- `internal/le/scratch/cutoff_test.go`, `internal/le/scratch/cachetrim_test.go`
- `test/ui/le-store-trim-cap.ci`, `internal/test/fixture/ui_fixture_le_store_trim_cap.go` and its `register_le_store_trim_cap.go`

The ledger's fields (no code in a spec):

| Field | Type | Description |
|-------|------|-------------|
| `written` | RFC 3339 time | when the trim child wrote it |
| `groups[].key` | string | the budget key |
| `groups[].budget` | bytes | the budget read by that trim |
| `groups[].size-after` | bytes | the group's total after that trim |
| `groups[].unmet` | bytes | bytes over budget the cutoff kept, zero when met |
| `groups[].holder` | pid, command, label, start | the process that set the cutoff, absent when none |
| `devices[].path` | string | one cache path per distinct device, for `statfs` |
| `devices[].free` | bytes | `diskspace.Free` on that path when written |
| `groups[].devices` | indexes | which devices the group's caches lie on |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | A build-host env knob, not product config (`ai/rules/config.md`) |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | No | No new verb; `store-trim` and `cache-clean` keep their grammar |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | Yes | `test/ui/le-store-trim-cap.ci` |
| Pipe completeness | Yes | `store-trim | json` already renders through the shared renderer; the new fields ride it |
| Env var registration | Yes | `ze.le.store.ambient-cache-budget` via `env.MustRegister` in `storetrim.go`, `Private: true` |
| Doctor check for runtime dependencies | N-A | No new runtime dependency of `ze`; the ledger is a le scratch file under `tmp/store-trim/` beside the existing stamp |
| Prometheus counters/metrics | N-A | le exports no metrics |
| BGP family surface | N-A | No BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | Dev tooling has no `features/<id>.md`; checked with `./le feature report` at implementation |
| 2 | Config syntax changed? | No | Env knob only |
| 3 | CLI command added/changed? | No | Output rows gain fields; no command added |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | Yes | `docs/contributing/running-commands.md`, the two sections named in Files to Modify |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/running-commands.md` (the runner's `ze.le.store.trim=off` sentence stays true; checked) |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` names no cache floor; checked by grep at implementation |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/post-release/engineering/spec-le-build-cache-cap.md` at implementation; known now: `running-commands.md` anchors `cachetrim.go`, `cacheclean.go`, `cacheuse.go`, `storetrim.go`, all updated in the same phase. `docs/contributing/patching-the-standard-library.md` anchors `cacheclean.go` for the `cache/stdpatch/` KEEP/DROP rows, which this spec does not change, so it stays as it is |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `cache-clean` and `store-trim` sample rows in `running-commands.md` show `kept ... younger than 3h0m0s`; rewritten to the cutoff shape |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the ledger read in the per-run check and the early-start branch, behind the existing lock
   - Tests: `TestLeInvocationSpawnsTrimWhenLedgerIsOverBudget`, `TestEarlyStartWhenLedgerGrowthExceedsBudget`, `TestNoEarlyStartWithinTenMinutes`
   - Files: `storetrim.go`, `ledger.go`
   - Verify: the wiring test fails until the child writes a ledger and the due rule reads it
2. **Phase: Ledger write and per-run cost** - the child writes the ledger by rename; the per-run check reads only the stamp, the ledger and `statfs`
   - Tests: `TestPerRunCheckOpensNoCacheDirectoryAndForksNothing`, `TestEarlyStartReadsTheBudgetNow`, `TestMissingOrCorruptLedgerFallsBackToTheStamp`, `TestStoreTrimOffStopsTheEarlyStart`, `BenchmarkPerRunCapCheck`
   - Files: `storetrim.go`, `ledger.go`
3. **Phase: Cutoff instant** - delete `cacheEntryAgeFloor`, derive the cutoff, re-stat before removal, refuse on an unreadable table; settle A-2 and A-3 by the grep they name
   - Tests: the `cutoff_test.go` tests, `TestRemoveOldestKeepsAnEntryRefreshedAfterTheWalk`, `TestCacheCleanDefaultUsesTheCutoffNotAFixedFloor`, `TestFullEmptyRefusalSetUnchanged`, the rewritten floor tests
   - Files: `cutoff.go`, `cachetrim.go`, `cacheclean.go`, `cacheuse.go`, `docs/contributing/running-commands.md` (the "cleans the cache under you" section)
4. **Phase: Ambient budget** - the new group and key
   - Tests: `TestAmbientCacheHasItsOwnBudget`, `TestAmbientBudgetRefusedAloneByKey`, `TestAmbientKernelCacheStillSkipped`
   - Files: `storetrim.go`, `cacheclean.go`, `cachetrim.go`, `running-commands.md` (store-trim table)
5. **Phase: Unmet report** - the holder line in the report, `last.log` and the ledger
   - Tests: `TestRemoveOldestStopsAtTheCutoffAndNamesTheHolder`, `TestUnmetGroupIsRecordedInTheLedger`
   - Files: `cachetrim.go`, `storetrim.go`, `running-commands.md` (sample rows)
6. **Phase: Functional proof** - `le-store-trim-cap.ci` and its fixture, then the forced reds of the Discrimination paragraph
   - Files: the fixture, its registration, the `.ci`; the existing `.ci` header

The unmet line, as a text sample (the implementer may reword within AC-12):

    go-caches  61.2G against ze.le.store.go-cache-budget 40G: 21.2G is newer than the cutoff 2026-10-22 08:09:00,
               set by pid 4242 (go test -tags ... ./..., job unit-all), running 1h12m, which may still read any
               entry it looked up. Fix: wait for pid 4242 to end (a trim starts within 10 minutes after), raise
               ze.le.store.go-cache-budget, or run ./le scratch clean all once no build runs.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The cutoff is an instant computed once per pass, never a duration from "now" at removal time; a process started during the pass can only move mtimes forward, so the fixed instant stays safe |
| Correctness | The per-run path forks nothing and opens no cache directory (AC-3), including on a missing ledger |
| Correctness | Removal compares the re-stated mtime, not the walk's (AC-7) |
| Naming | `ze.le.store.ambient-cache-budget` matches its siblings; new json keys kebab-case (`cutoff`, `holder`) |
| Rule: no-layering | No `cacheEntryAgeFloor` and no fixed-floor fallback remain; grep proves it |
| Rule: principles | An unknown start or an unreadable process table refuses; it never assumes a floor |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Fixed floor gone | `grep -rn cacheEntryAgeFloor internal/le` answers nothing |
| Ambient budget registered | `./le env list` with private keys, or grep `ambient-cache-budget` in `storetrim.go` |
| Per-run cost | `BenchmarkPerRunCapCheck` output pasted |
| Real-path proof | `./le test ui` subset running `le-store-trim-cap` green, and the three forced reds recorded |
| Docs | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The ledger is untrusted input from an older or a crashed le: a parse failure is "not due", never a panic; paths it names are only `statfs`ed, never removed |
| Resource exhaustion | The early start is bounded to one per ten minutes; the walk stays in the detached child under `storeTrimTimeout` |
| Removal scope | The trim removes only `-a` and `-d` entries in the 256 subdirectories of a target from `cleanTargets`, never a path read from the ledger |

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

- The three-hour floor's own justification (an entry is safe for two hours after Go last looked it up) is wrong for any go command that runs longer than two hours, because a single go command looks an entry up once and may read it much later. The process-bounded cutoff is safer than the fixed floor for long runs and more aggressive only when no long run exists.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-run check reads a ledger plus `statfs`; the growth bound is the recorded size plus the free-space drop | Walk the caches on every run; stat the 256 subdirectories' mtimes; keep hourly only | A walk costs seconds and every hook call runs the check; subdirectory mtimes say that something was written, not how much; the free-space drop is an upper bound on growth, so it can only over-start, which the ten-minute interval bounds |
| Cutoff instant from the oldest live cache user's start minus `mtimeInterval` minus 5 minutes | Lower the fixed floor to 75 minutes; keep 3 hours | A fixed 75 minutes breaks any go command longer than 15 minutes (a late link reads archives looked up at its start); 3 hours is both too slow for the cap and unsafe for runs over two hours |
| Ambient cache gets its own budget, `20G` | Fold it into the 40G union; keep D-6 | The union is le's own caches (D-1); the ambient cache serves every Go project of the user, so it is bounded separately and its default is smaller than the union because le does not write it on purpose |
| Enforce through Go's cache directory layout | `GOCACHEPROG`, a cache program that enforces size on every put | A cache program only covers go commands that name it, so the ambient cache and bare builds escape; it adds a long-lived helper to every build whose failure breaks every build; eviction safety is the same problem in another process |
| Automated full `go clean -cache` when over cap and no build runs | Same | A busy machine always has a build running, so it never fires when needed, and when it does it costs every session a cold rebuild |
| An unmet cap is reported, never enforced by refusing commands | Refuse cache-hungry commands while over cap | Refusing a hook call would stop every session; the cap is a disk-protection measure, and the device-full preflight is a separate journal item (`plan/journal/full-disk-false-red.md`, 2026-09-19) |

## Known Limitations
- `GOMODCACHE` is not capped: it is not a build cache, it grows per module version rather than per build, and O-1 names the build cache.
- A cache reader outside this user's process table (another user, a container not started by le) is not seen (A-2); the doc names this residual.
- The race R-2 remains, as it does for Go's own trim.
- Owner decision 2026-10-22 (Q-3, Thomas: "ok" to the recommendation): an unmet cap ALSO surfaces in the session-start banner (one line, printed only while the cap is unmet, naming the size, the budget key, the holding process and the fix) and as a `./le setup check` row, both read from the one ledger record the trim writes, so they cannot disagree; no cache walk at session start. The implementer adds the AC, test and wiring for both.
- Owner decision 2026-10-23 (Thomas chose it when asked): the ambient cache budget `ze.le.store.ambient-cache-budget` defaults to 20G.

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: Scope is tooling)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-le-build-cache-cap.md` only, in the same `./le commit create` script
