# Spec: le-one-go-cache-per-host

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release-independent le tooling: one Go build cache per host, under `~/.cache/ze`, which is its own drive on the Linux host |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decisions that approve this work:

| Id | Date | Decision | Thomas's words |
|----|------|----------|----------------|
| O-1 | 2026-10-24 | One Go build cache per host: the per-user `~/.cache/ze` (`$XDG_CACHE_HOME/ze`), never `~/Library/Caches` and never a second copy in the checkout. Every `./le` action, every verify worktree, the bootstrap and lint caches, and every tool outside le write there. On the Linux host `~/.cache` is its own drive, which is why the location matters | "We want to use the cache in the .cache folder because on another host it's a different drive", "We should not have anything in the library. we want to remove the duplication" |
| O-2 | 2026-10-24 | gopls, golangci-lint and staticcheck reach the shared cache through le: `le exec <path> [args]` sets the cache environment and replaces itself with the program, so the program inherits it. Not through links from the tools' default cache folders | "you can change your own process ENV and then fork and the fork will have the env, so le can be used as a launcher", "can we have le exec <path> - which would change the env and run the path command ?" |
| O-3 | 2026-10-24 | `le setup` writes `gopls`, `golangci-lint` and `staticcheck` shims in `~/bin` that run `le exec` on the real tool, and makes sure `~/bin` comes first on PATH. le never writes the owner's shell files: when PATH is wrong it prints the exact line for zsh and for NuShell | "le setup make sure we have ~/bin in the path first and create shim there run first", "Print only", and "Yes, as stated" to the whole design |
| O-4 | 2026-10-24 | When the cache setup is wrong, le prints what is wrong and repairs what it owns: the checkout's `cache/` link and the `go env` GOCACHE setting | "when le cache is wrong, print it and fix itself" |
| O-5 | 2026-10-24 | `le exec` gives a program ze's patched GOROOT only when the working directory is inside a ze checkout; anywhere else it sets the cache environment alone | "Approve, patched GOROOT in-checkout only" |
| O-6 | 2026-10-24 | The QEMU guest keeps its own Go cache at `cache/qemu-go-cache`, on the per-user drive and trimmed with the rest, rather than sharing the host's cache over the mount; the one bounded exception to O-1. Superseded by O-7 | "Save; guest own cache under cache/" |
| O-7 | 2026-10-24 | The QEMU guest reuses the host's `cache/go-cache` over the 9p share. Implementation proves it with `./le test qemu` while the host builds; only if that fails does the guest fall back to `cache/qemu-go-cache`, and the spec records the observed failure | "we currently have a cache folder we should reuse it", and "Reuse, fallback if it fails" |

Split out of `spec-le-build-cache-cap` (its O-3 and O-4, recorded 2026-10-24),
whose G-3 ambient budget this work replaces: once every writer uses one cache
there is no ambient cache left to budget. O-2 and O-3 here replace the shell
variables that O-3 of that spec named.

Problem, measured on the Mac on 2026-10-24 before any change: 53G free of 927G.
Three Go build caches held about 100G: `~/Library/Caches/go-build` 59G (every
Go command outside le), the checkout's real `cache/go-cache` 20G, and the
shared `~/.cache/ze/go-cache` 21G (verify worktrees). The checkout's `cache/`
had never been linked, because only `./le scratch links-ensure` and
`./le scratch migrate` create the link. gopls, golangci-lint and staticcheck
kept their own caches under `~/Library/Caches` (2.1G). By hand the same day:
the checkout's `cache/` was replaced by the link and the duplicate removed,
`go env -w GOCACHE` set, and shell variables added to `~/.zshenv` and NuShell's
`env.nu`; 154G free after. This spec makes le hold that state on every host.

Goal: on every host, every Go build, lint and staticcheck cache that le, its
children, an editor's language server, or a tool started from any shell writes
lands under `~/.cache/ze`, and le repairs a host where that stops being true.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/running-commands.md` - "Where a Go test keeps its temp dirs", "When the disk is full", "When another session cleans the cache under you", "A verify worktree shares that cache": the cache layout this spec changes
  → Constraint: the page states "nothing at le start creates the link" and describes "seven caches" on up to three filesystems; both statements become false and are rewritten in the phase that changes them
  → Decision: the per-user target stays `$XDG_CACHE_HOME/ze`, else `~/.cache/ze` (`cacheTarget`); this spec adds no second place to name it
  → Constraint: the kernel caches under the per-user target (`runtime-kernel/`, `installer-kernel/`) are never a clean or trim target; `cleanTargets` marks `holdsKernel` and this spec must keep that skip
- [ ] `docs/architecture/core-design.md` - le root dispatch and before-dispatch hooks
  → Constraint: a before-dispatch hook MUST NOT print and MUST NOT wait for slow work (`internal/le/le/root/beforedispatch.go`); the link repair is therefore not a hook, it runs in `run` ahead of `activatePatchedGOROOT`, the same way that step is deliberately outside the hook contract
- [ ] `docs/contributing/patching-the-standard-library.md` - the patched GOROOT every le-driven Go command uses
  → Decision: `activatePatchedGOROOT` sets GOROOT for le and every child; gopls, golangci-lint and staticcheck read the patched sources from disk, so a tool launched by `le exec` inside the checkout analyses what Ze compiles
  → Constraint: `gen.CacheDir` creates `cache/stdpatch` with `MkdirAll`; it runs before any hook, so a missing `cache/` becomes a real directory unless the link is made first
- [ ] `docs/guide/developer-setup.md` - `./le setup`: tool installs, GOBIN on PATH, LSP plugins
  → Decision: the PATH advice reuses the existing pattern (`goToolOffPath`: a PENDING row plus one printed line naming the profile file); NuShell gets its own printed line beside it
- [ ] `docs/architecture/testing/qemu-integration.md` - the `zescratch-cache` share and the guest's Go cache
  → Constraint: the guest builds with `GOCACHE=/workspace/tmp/qemu/go-cache`; the host's `cache/` is reachable in the guest through the cache share, so the guest can build into the shared cache instead (A-3)
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - verify worktree links (`sharedCacheLink`, `sharedLintCacheLink`)
  → Decision: once the lint cache lives under `cache/`, the worktree's `cache/` link carries it and `sharedLintCacheLink` is deleted (`ai/rules/no-layering.md`)

**Key insights:** (minimal context to resume after compaction)
- Every le run creates a real `cache/` before any hook: the `le` launcher's `build_le` runs `mkdir -p $root/cache/go-cache`, and `run` in `internal/le/register.go` calls `activatePatchedGOROOT`, whose `gen.CacheDir` runs `MkdirAll(cache/stdpatch)`. The link must be ensured before both.
- `ensureSymlink` answers SKIP for a real directory and never converts it; `Migrate` refuses when the target already holds `go-cache`. A repair needs a new path: rename the real `cache/` aside, link, leave the renamed copy for the trim.
- `lepath.Root` falls back to the le executable's location when the working directory is in no checkout, so "inside a checkout" for O-5 must be decided from the working directory alone (`ancestorWithMarkers(cwd)`), not from `lepath.Root`.
- Eight sites start `go` with the inherited environment and no GOCACHE; they use `go env GOCACHE`, which `le setup` repairs. `repositorytrackedbuild.go` sets `GOENV=off` but names GOCACHE explicitly.
- The ambient-row skip in `cleanTargets` compares path strings (`ambient == checkout`), so a host whose `go env GOCACHE` is the shared cache cleans one directory twice under two rows.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `le` (launcher script) - `build_le` sets `GOCACHE=$root/cache/go-cache`, falls back to `$root/tmp/go-cache` when `mkdir` fails (a dangling link in a QEMU guest), and sets `GOLANGCI_LINT_CACHE=$root/tmp/golangci-lint-cache`; ends with `exec "$binary" "$@"`
  → Constraint: `TestBootstrapCacheMatchesTheShellScript` (`internal/le/go/toolchain/gotoolchain_test.go`) compares the script's fallback with `BootstrapCache`; any path change moves both together
- [ ] `internal/le/register.go` - `run`: `activatePatchedGOROOT`, then `leroot.RunBeforeDispatch`, then `leroot.Dispatch`
- [ ] `internal/le/scratch/scratch.go` - `cacheTarget` (XDG, else HOME/.cache/ze); `Ensure` (tmp and cache links), `EnsureCache` (cache link only, used by verify worktrees), `ensureSymlink` (creates a missing link, SKIP on a real path), `ensureExistingSymlink` (compares the link text to the target as raw strings; MISMATCH unless repoint), `Migrate` (moves entries, REFUSE when one exists in the target)
- [ ] `internal/le/scratch/cacheclean.go` - `cleanTargets`: rows checkout, shared, ambient (`notTrimmed`), bootstrap, lint, staticcheck, shared staticcheck; shared rows skipped by `sameDirectory`; ambient skipped only by string equality; `ambientGoCache` runs `go env GOCACHE` with GOCACHE removed from the environment
- [ ] `internal/le/scratch/storetrim.go` - hourly detached trim over `cleanTargets`; budgets `ze.le.store.go-cache-budget`, `lint-cache-budget`, `staticcheck-cache-budget`
- [ ] `internal/le/go/toolchain/gotoolchain.go` - `GoCache` (`root/cache/go-cache`), `LintCache` (`root/tmp/golangci-lint-cache`), `BootstrapCache` (`root/tmp/go-cache`), `Overrides` naming them through `gen.Landing`
- [ ] `internal/le/go/stdpatch/gen/stdpatch.go` - `CacheDirs`, `CacheDir` (`cache/stdpatch`, fallback `tmp/stdpatch`), `Landing`, `Activate`
- [ ] `internal/le/le/path/lepath.go` - `Root`: `ze.repo.root`, else the working directory's marker ancestor, else the executable's
- [ ] `internal/le/verify/lifecycle.go` - `sharedCacheLink` (calls `EnsureCache`), `sharedLintCacheLink` (links the worktree's `tmp/golangci-lint-cache` to the checkout's)
- [ ] `internal/le/test/qemu/run.go`, `run_exec.go`, `guest_linux.go` - guest `GOCACHE=/workspace/tmp/qemu/go-cache`; `guest_linux.go` falls back to `BootstrapCache`
- [ ] `internal/le/deployment/daemonbuild.go`, `vppevidence.go` - `BootstrapCache` when the inherited GOCACHE is empty
- [ ] `internal/le/setup/setup.go`, `gobin.go`, `tools.go`, `servers.go` - `Run` visits required tools (gopls, golangci-lint, staticcheck installed by `go install` into `goBinDir`), the gopls probe, LSP plugins; `onPath`, `shellProfile` (`~/.zshenv` for zsh, else `~/.profile`), `goToolOffPath` prints the PATH line once
- [ ] `internal/le/go/staticcheck/judge.go` - `CacheDir(tree)` = `tree/cache/staticcheck-matrix`; `judgeEnvironment` sets `STATICCHECK_CACHE`

**Behavior to preserve:**
- The kernel caches are never removed by any clean, trim or repair
- A QEMU guest with a dangling `cache/` still builds (the launcher's fallback stays)
- `ze.repo.root` still wins in `lepath.Root`
- `./le scratch links-ensure` and `./le scratch migrate` keep working for `tmp/`
- Every le action keeps the patched GOROOT inside a checkout

**Behavior to change:**
- le makes and repairs the checkout's `cache/` link on every run (O-4)
- The lint and bootstrap caches move under `cache/` (O-1)
- New `le exec` verb (O-2, O-5); `le setup` shims, PATH check, GOCACHE repair (O-3, O-4)
- `cleanTargets` loses the rows that can no longer differ, and the ambient row compares resolved directories

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le <anything>`: the launcher script, then `bin/le` `run`
- `le exec <path> [args]`: started by a shim in `~/bin` when an editor, the Claude LSP or a shell runs `gopls`, `golangci-lint` or `staticcheck`
- `./le setup`: the developer, once per host and after tool upgrades

### Transformation Path
1. Launcher script: before `build_le` creates anything under `cache/`, a missing `cache/` becomes a link to the per-user target; a real `cache/` directory is renamed to `cache.stale-<timestamp>` and replaced by the link, with one stderr line naming what was moved
2. `run`: the same check in Go (`scratch` package, one `Lstat` when the link is right), ahead of `activatePatchedGOROOT`, for runs that start `bin/le` directly and for exports and worktrees
3. Every le-driven Go command: `Overrides` names `cache/go-cache`, `cache/golangci-lint-cache`, `cache/staticcheck-matrix`, all under the link
4. `le exec`: sets GOCACHE, GOPLSCACHE, GOLANGCI_LINT_CACHE and STATICCHECK_CACHE to directories under the per-user target, creating them; keeps the patched GOROOT only when the working directory has a checkout ancestor, and otherwise removes the GOROOT `activatePatchedGOROOT` set; then replaces the process with the program
5. `./le setup`: writes the three shims, reports `~/bin` position on PATH, repairs `go env -w GOCACHE`
6. Store trim and `cache-clean`: one Go cache row, one lint row, one staticcheck row, ambient compared by resolved directory

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Launcher shell ↔ le binary | Both decide the link; the shell copy is compared with the Go copy by a test | No |
| le ↔ child tool | Environment inherited across `execve` | No |
| Host ↔ QEMU guest | Cache share mount; guest builds into the shared Go cache | No |
| le ↔ Go's user config | `go env -w GOCACHE` writes Go's `GOENV` file | No |

### Integration Points
- `scratch.Manager` (`scratch.go`) - gains the repairing ensure used by the launcher's Go side and `run`; `EnsureCache` for worktrees uses it too
- `gotoolchain.LintCache`, `BootstrapCache` - paths move under `cache/`; `BootstrapCache` remains only as the launcher's unusable-cache fallback
- `setup.Run` - three new visits: shims, `~/bin` on PATH, GOCACHE setting
- le action registry - `exec` registers like every other le action

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every cache path still comes from `gotoolchain` and `scratch.cacheTarget` |
| No unintended coupling (components stay isolated) | Yes | `exec` reads `scratch` for the target and nothing else |
| No duplicated functionality (extends existing, does not recreate) | Yes | Link repair extends `scratch`; PATH advice extends `goToolOffPath` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling |
| Registration over hardcoding, outbound | Yes | `exec` registers through the le action registry |
| Registration over hardcoding, inbound | Yes | The shim tool list is the existing `requiredTools` rows with `GoInstall`, filtered by a field on `Tool`, not a new list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Renaming a real `cache/` aside while another session builds through it costs that build at most a cache miss or one failed build, never a wrong result | Go's cache is content-addressed and written by atomic rename; journal `full-disk-false-red.md` shows only "cache entry not found" style failures when entries vanish | A concurrent build fails confusingly once per host | The repair prints a stderr line naming the rename, so a failure in that minute is attributable; one manual run on a checkout with a live build | unvalidated |
| A-2 | gopls, golangci-lint and staticcheck honour GOPLSCACHE, GOLANGCI_LINT_CACHE and STATICCHECK_CACHE | Measured 2026-10-24: with the variables set, `~/.cache/ze/gopls` and `~/.cache/ze/golangci-lint` appeared | A tool keeps writing `~/Library/Caches` | Functional test asserts the variables; one manual gopls start after setup shows no `~/Library/Caches/gopls` | unvalidated |
| A-3 | The guest can build into the host's `cache/go-cache` over the 9p `zescratch-cache` share while the host builds into it (O-7) | `qemu-integration.md` describes the share; `guestPatchedTree` relies on `cache/` resolving in the guest; Go's cache tolerates concurrent processes on one filesystem, unproven across 9p | Guest or host builds fail, or report missing or corrupt entries | One `./le test qemu` run while a host `go test` builds through the same cache; on failure, the O-7 fallback `cache/qemu-go-cache` with its own trim row, and the failure pasted here | unvalidated |
| A-4 | Editors and the Claude LSP find gopls by PATH lookup, so a shim earlier on PATH is used | `ai/rules/context-economy.md` LSP setup; VS Code and Zed default to PATH | An editor configured with an absolute gopls path bypasses the shim | `./le setup` gopls probe runs through PATH and reports which binary answered | unvalidated |
| A-5 | Starting `le` for each gopls launch is fast enough | le warm start plus `activatePatchedGOROOT` stamp check | Editor start feels slow | Measure `le exec /usr/bin/true` warm; acceptable under 200ms | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A shim runs `le exec` on itself and loops | Process storm, editor hang | `le exec` refuses a path that resolves to a file it wrote as a shim (marker line in the shim) and names `./le setup` |
| R-2 | The checkout a shim points at moves or is deleted | Shim exits with "le not found" | The shim's message names the missing path and `./le setup` from the new checkout |
| R-3 | A real tool is reinstalled to another path | `le exec` cannot find the recorded path | `le exec` error names the missing path and `./le setup`; setup rewrites shims each run |
| R-4 | The renamed `cache.stale-*` copy fills the disk | `df` after repair | The store trim removes `cache.stale-*` directories older than the 3h floor; first repair prints its size |
| R-5 | `go env -w` fails (read-only GOENV) | setup row error | Row reports the error and the exact command; other rows still run |
| R-6 | Shell and Go copies of the link rule drift | Test red | A test compares the launcher's target expression with `cacheTarget` |
| R-7 | Two le runs repair the same real `cache/` at once | Rename fails with ENOENT in one | ENOENT on rename, or a link already present, is success |
| R-8 | `le exec` with no checkout leaks Ze's GOROOT into another project | gopls on another repo reports crypto/tls diffs | O-5: GOROOT removed unless the working directory has a checkout ancestor; unit test covers both |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Builds in a checkout (cache misses or one failed build during the repair); gopls in editors (shim failure means no language server until `./le setup` reruns) |
| How is it reverted? | Single commit revert; shims removed by deleting them from `~/bin`; `go env -u GOCACHE` |
| Who else touches this path? | `spec-le-build-cache-cap` (trim cadence and floor, same `cleanTargets`), `spec-test-verdicts-independent-of-load` (owns the full-disk journal rows) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le <action>` on a checkout with no `cache/` | → | launcher link ensure, `scratch` repair in `run` | `test/ui/le-cache-link-repair.ci` |
| `./le <action>` on a checkout whose `cache/` is a real directory | → | rename aside and link | `test/ui/le-cache-link-repair.ci` |
| `le exec <probe>` inside and outside a checkout | → | `exec` action | `test/ui/le-exec-cache-env.ci` |
| `./le setup` with `~/bin` absent from PATH | → | shim visit, PATH visit, GOCACHE visit | `TestSetupWritesShimsAndReportsPath` (`internal/le/setup/shims_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any `./le` action in a checkout with no `cache/` | `cache/` is a symlink to `$XDG_CACHE_HOME/ze` (else `~/.cache/ze`) before any build; no real `cache/` is ever created |
| AC-2 | Any `./le` action in a checkout whose `cache/` is a real directory | It is renamed to `cache.stale-<timestamp>`, `cache/` becomes the link, one stderr line names both paths and the size; the action continues and its stdout is unchanged |
| AC-3 | `cache/` is already the correct link, written with or without a trailing slash | No rename, no output, no fork |
| AC-4 | `bin/le` started directly (no launcher script) in a git-archive export | AC-1 holds: the export's `cache/` is the link, no private `go-cache` grows in it |
| AC-5 | A le-driven lint run | golangci-lint writes under `cache/golangci-lint-cache`; nothing is written under `tmp/golangci-lint-cache` |
| AC-6 | Launcher build of le with `cache/` usable | GOCACHE is `cache/go-cache`; `tmp/go-cache` is used only when `cache/` cannot be created, with the existing stderr line |
| AC-7 | `le exec <path> args` with the working directory inside a checkout | The program runs with the given args, its exit code is le's, and its environment has GOCACHE, GOPLSCACHE, GOLANGCI_LINT_CACHE, STATICCHECK_CACHE under the per-user target and the patched GOROOT |
| AC-8 | `le exec <path>` with the working directory outside any checkout | Same cache variables; GOROOT is whatever the caller had, never Ze's patched tree |
| AC-9 | `le exec` given a path that is one of its own shims, or a path that does not exist | Exit 1, message names the path, why, and `./le setup`; nothing is executed |
| AC-10 | `./le setup` | `~/bin/gopls`, `~/bin/golangci-lint`, `~/bin/staticcheck` exist, are executable, and each runs `<checkout>/le exec <real tool path>` with all arguments; rerunning rewrites them with current paths |
| AC-11 | `./le setup` with `~/bin` absent from PATH or after a directory holding a real tool | PENDING row; one printed line for zsh (`~/.zshenv`) and one for NuShell (`env.nu`, from `nu -c '$nu.env-path'` when `nu` exists) that put `~/bin` first; no shell file is written |
| AC-12 | `./le setup` with `go env GOCACHE` not the shared cache | `go env -w GOCACHE=<target>/go-cache` runs; the row prints the old and new value |
| AC-13 | `./le scratch cache-clean` and the store trim on a linked host | One Go cache row, one lint row, one staticcheck row; the ambient row is a skip when `go env GOCACHE` resolves to the same directory; no directory is cleaned twice |
| AC-14 | Store trim with a `cache.stale-*` older than the floor | It is removed and reported on its own row |
| AC-15 | Any repair, clean or trim | The kernel caches are untouched |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnsureCacheLinkCreatesMissing` | `internal/le/scratch/scratch_test.go` | AC-1 | |
| `TestEnsureCacheLinkRenamesRealDirectory` | `internal/le/scratch/scratch_test.go` | AC-2, R-7 | |
| `TestEnsureCacheLinkAcceptsTrailingSlash` | `internal/le/scratch/scratch_test.go` | AC-3 | |
| `TestLauncherLinkMatchesCacheTarget` | `internal/le/go/toolchain/gotoolchain_test.go` | R-6 | |
| `TestLintCacheIsUnderCache` | `internal/le/go/toolchain/gotoolchain_test.go` | AC-5 | |
| `TestExecSetsCacheEnvironment` | `internal/le/exec/exec_test.go` | AC-7 | |
| `TestExecOutsideCheckoutKeepsCallerGOROOT` | `internal/le/exec/exec_test.go` | AC-8, R-8 | |
| `TestExecRefusesShimAndMissingPath` | `internal/le/exec/exec_test.go` | AC-9, R-1 | |
| `TestSetupWritesShimsAndReportsPath` | `internal/le/setup/shims_test.go` | AC-10, AC-11 | |
| `TestSetupRepairsGoCacheSetting` | `internal/le/setup/shims_test.go` | AC-12 | |
| `TestCleanTargetsOneRowPerCache` | `internal/le/scratch/cacheclean_test.go` | AC-13 | |
| `TestAmbientSkippedWhenSameDirectory` | `internal/le/scratch/cacheclean_test.go` | AC-13 | |
| `TestTrimRemovesStaleCacheCopy` | `internal/le/scratch/storetrim_test.go` | AC-14 | |
| `TestCleanTargetsNeverReachTheKernelCache` (existing) | `internal/le/scratch/cacheclean_kernel_test.go` | AC-15 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The feature takes no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-cache-link-repair` | `test/ui/le-cache-link-repair.ci` + fixture `internal/test/fixture/ui_fixture_le_cache_link_repair.go` | A throwaway checkout copy with `XDG_CACHE_HOME` pointed at a temp dir: (1) no `cache/`, run `le help`, `cache/` is the link (AC-1); (2) replace with a real directory holding a file, run again, stderr names the rename, link restored, the file is in `cache.stale-*` (AC-2); (3) run again, no stderr (AC-3) | |
| `le-exec-cache-env` | `test/ui/le-exec-cache-env.ci` + fixture | `le exec` on `env` inside the checkout and from a temp directory: the four variables point under the temp target in both; GOROOT is the patched tree only inside (AC-7, AC-8); `le exec` on a shim and on a missing path exits 1 with the setup command (AC-9) | |

Discrimination: with the launcher and `run` link step removed, step 1 sees a real `cache/`; with the GOROOT removal dropped, the outside run shows the patched tree. Each forced once and recorded at implementation.

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `le` - link ensure before `build_le` creates `cache/`; lint cache path under `cache/`
- `internal/le/register.go` - link ensure in `run` ahead of `activatePatchedGOROOT`
- `internal/le/scratch/scratch.go` - repairing ensure (rename aside, link), cleaned-path comparison
- `internal/le/scratch/cacheclean.go` - remove shared Go, bootstrap and shared staticcheck rows; ambient compared with `sameDirectory`; `cache.stale-*` row
- `internal/le/scratch/storetrim.go` - trim the `cache.stale-*` row
- `internal/le/go/toolchain/gotoolchain.go` - `LintCache` under `cache/`; `BootstrapCache` kept only as the launcher fallback, comments updated
- `internal/le/verify/lifecycle.go` - delete `sharedLintCacheLink` and its call
- `internal/le/test/qemu/run.go`, `run_exec.go`, `guest_linux.go` - guest GOCACHE `/workspace/cache/go-cache` (O-7); `/workspace/cache/qemu-go-cache` with its own trim row only if A-3 fails
- `internal/le/deployment/daemonbuild.go`, `vppevidence.go` - fall back to `GoCache`, not `BootstrapCache`
- `internal/le/setup/setup.go`, `gobin.go`, `tools.go` - shim, PATH and GOCACHE visits; NuShell line
- Tests pinning old paths: `go/vetplatforms/platformvet_test.go`, `go/lint/parity_test.go`, `test/unit/testunit_test.go`, `chaos/selftest/actions_test.go`, `build/compile/parity_test.go`, `cmd/ze/root_launcher_test.go`, `go/toolchain/gotoolchain_test.go`, `gotoolchain_cacheenv_test.go`, `verify/lifecycle_test.go`, `scratch/cacheclean_test.go`, `cacheclean_safe_test.go`, `storetrim_test.go`, `livetrim_test.go`, `internal/test/fixture/ui_fixture_le_store_trim_answers.go`
- Docs: `docs/contributing/running-commands.md`, `docs/guide/developer-setup.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/testing/verify-freshness-scope.md`, `docs/architecture/testing/tracked-build-gate.md`, `docs/architecture/core-design.md`

## Files to Create
- `internal/le/exec/register.go`, `internal/le/exec/exec.go`, `internal/le/exec/exec_test.go` - the `exec` action
- `internal/le/setup/shims.go`, `internal/le/setup/shims_test.go` - shim writing and the PATH and GOCACHE rows
- `test/ui/le-cache-link-repair.ci`, `internal/test/fixture/ui_fixture_le_cache_link_repair.go`
- `test/ui/le-exec-cache-env.ci`, `internal/test/fixture/ui_fixture_le_exec_cache_env.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling, no Ze config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `le exec` in `internal/le/exec/register.go`; follows `ai/rules/cli.md` for exit codes and messages |
| CLI grammar (keyword before value) | Yes | `le exec <path> [args...]`: the path is positional and everything after it passes through untouched |
| Editor autocomplete | N-A | le actions get completion from the registry |
| Functional test for new RPC/API | Yes | `test/ui/le-exec-cache-env.ci` |
| Pipe completeness | N-A | `exec` replaces the process; no le output to pipe |
| Env var registration | No | Sets third-party variables (GOPLSCACHE and siblings) in the child only; no `ze.*` key is added |
| Doctor check for runtime dependencies | Yes | `./le setup` rows are le's doctor for developer hosts: shims, `~/bin` on PATH, GOCACHE setting |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | developer tooling, not a Ze feature |
| 2 | Config syntax changed? | No | no config |
| 3 | CLI command added/changed? | Yes | `le exec` and the new setup rows in `docs/guide/developer-setup.md`; `docs/contributing/running-commands.md` for the link and cache layout |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md` (guest cache), `verify-freshness-scope.md` (lint link removed), `tracked-build-gate.md` |
| 11 | Affects daemon comparison? | No | tooling |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: the link step ahead of `activatePatchedGOROOT` in le's `run` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the le command surface gains `exec`; regenerated le help surfaces |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/post-release/engineering/spec-le-one-go-cache-per-host.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `df` examples in `running-commands.md` name `tmp/golangci-lint-cache`; updated with the path change |

## Implementation Steps

1. **Phase: Wiring** - `exec` action registered and reachable; link step called from the launcher and `run`; failing wiring tests
   - Tests: `le-cache-link-repair`, `le-exec-cache-env`
   - Files: `internal/le/exec/register.go`, `le`, `internal/le/register.go`
   - Verify: both `.ci` reach the code and fail on the stubs
2. **Phase: Link repair** - repairing ensure in `scratch`, shell copy in the launcher, comparison test; docs for the link in `running-commands.md` and `core-design.md`
   - Tests: `TestEnsureCacheLink*`, `TestLauncherLinkMatchesCacheTarget`, `le-cache-link-repair`
3. **Phase: One cache** - lint and bootstrap paths under `cache/`, QEMU guest, deployment fallbacks, delete `sharedLintCacheLink`, update pinned tests, cleanTargets rows and stale-copy trim; docs for each moved path
   - Tests: `TestLintCacheIsUnderCache`, `TestCleanTargetsOneRowPerCache`, `TestAmbientSkippedWhenSameDirectory`, `TestTrimRemovesStaleCacheCopy`, kernel-cache tests, one `./le test qemu` run (A-3)
4. **Phase: le exec** - environment, GOROOT rule, refusals
   - Tests: `TestExec*`, `le-exec-cache-env`
5. **Phase: le setup** - shims, PATH rows with zsh and NuShell lines, GOCACHE repair; `developer-setup.md`
   - Tests: `TestSetup*`; manual: gopls started from an editor writes nothing under `~/Library/Caches` (A-2, A-4)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No path under `tmp/` remains as a Go, lint or staticcheck cache except the launcher's unusable-cache fallback |
| Correctness | The link repair never deletes; it only renames |
| Naming | `cache.stale-<timestamp>` is the one spelling, shared by the repair and the trim |
| Data flow | The per-user target is named only by `cacheTarget`, and the launcher copy is test-compared |
| Rule: no-layering | `sharedLintCacheLink`, the bootstrap row, the shared rows are deleted, not left beside the new path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `exec` action | `./le help` lists it; `test/ui/le-exec-cache-env.ci` passes |
| Link repair | `test/ui/le-cache-link-repair.ci` passes |
| No `tmp/` caches | grep for `tmp/golangci-lint-cache` and `tmp/go-cache` in `internal/` and `le` finds only the fallback |
| Setup rows | `TestSetup*` pass; `./le setup` on this host reports shims PRESENT |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `le exec` executes only an absolute or PATH-resolved existing regular file; refuses its own shims |
| Shim content | Paths in shims are single-quoted with `shellSingleQuote`; no unquoted expansion of a path |
| File writes | Shims are written only in `~/bin`, mode 0755, never over a file that is not a le shim without saying so |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| full-disk-false-red.md | 2026-10-23 | store-trim and cache-clean miss ~/.cache/ze/staticcheck-matrix |
| full-disk-false-red.md | 2026-10-21 | each git archive export grew its own cache/ |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `le exec` plus shims in `~/bin` | Shell variables in zsh and NuShell files; links from `~/Library/Caches/<tool>`; shims replacing the binaries in the Go bin directory | Owner chose le as the launcher (O-2, O-3); shell files miss editors started from the Dock; a `go install` would overwrite shims placed in the Go bin directory |
| Repair renames a real `cache/` aside | Merge entries into the target; refuse and print | Owner asked le to fix itself (O-4); a rename is atomic and loses nothing; the trim removes the copy later |
| Link step outside the before-dispatch hooks | A new hook | The hook contract forbids printing, and the step must precede `activatePatchedGOROOT` |
| PATH advice printed, never written | le edits `~/.zshenv` and `env.nu` | Owner: "Print only" for shell files |
| Patched GOROOT only inside a checkout | Always | O-5 |

## Known Limitations
- A tool started by absolute path, bypassing PATH, keeps its default cache; `./le setup`'s gopls probe reports which binary answered.

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
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify current fast`; the full gate runs once over the closing commits
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
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
