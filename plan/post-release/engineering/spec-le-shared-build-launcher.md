# Spec: le-shared-build-launcher

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Release-independent le shared build launcher work; historical findings do not establish a current product defect. |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision O-1, recorded here because it approves this feature (it was a row
of `ai/META-DECISIONS.md` until 2026-10-22, when that file was kept to meta
decisions only):

| Id | Date | Decision | Thomas's words |
|----|------|----------|----------------|
| O-1 | 2026-10-22 | `bin/` gets a shared-build launcher: `le-<name>` builds are content-addressed and hard-linked to one copy per distinct build | "yes" to "Shared-build launcher for `bin/` (content-addressed, hard-linked `le-<name>`)?" |

O-1: `bin/` gets a shared-build launcher. Every `le-<name>` build is content-addressed and
hard-linked to one copy per distinct build, instead of one 137 MB binary per
name.

The symptom is in `plan/handover/17-exabgp-6-linux-continue.md`, "Owner
decisions" row 2: 77 `bin/le-<name>/le` launchers of 137 MB each, about 10 GB,
were mostly identical builds. The 24 h idle trim kept every one of them because
each was used that day, so the trim cannot reclaim what is duplicated, only
what is idle.

Goal: after this spec, N names whose builds are byte-identical occupy one
binary on disk, every guarantee `./le --name` gives today still holds (the
binary carries the caller's own edits, a running peer keeps its inode,
`cmd/ze/le_build_name.go` names the build correctly), and the store trim
reclaims a stored build no name links to.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the system door, read whole
  → Constraint: le is development tooling; nothing here reaches `pkg/` or a shipped `ze`. The "outside parties" are the concurrent sessions, the owner's Linux and macOS machines and CI (le-tooling door)
- [ ] `ai/doors/le-tooling.md` - the le door, read whole
  → Constraint: "The launcher writes `<target>.new.<pid>` and renames it, so a running le keeps its inode" is a contract with every concurrent session. Publishing a hard link MUST keep the rename-into-place step, never write or link onto the target directly
  → Constraint: the owner's macOS and Linux machines both run the script. Nothing in the script may depend on a GNU-only flag (`stat -c`, `sha256sum` alone, `ln -T`)
- [ ] `ai/rules/commands.md` - the `bin/le` and `--name` rules
  → Constraint: `bin/le` MUST NOT be deleted and is not touched by this spec. `--name` "lands in `bin/le-<name>/le` and is rebuilt on every call": both stay true, the landing becomes a hard link
  → Constraint: `cmd/ze/le_build_name.go` refuses when the running binary is not the one named. The design MUST keep the binary's invoked path `bin/le-<name>/le`, so that refusal keeps working with no code change
- [ ] `docs/contributing/running-commands.md` - "Launcher builds do not overwrite running binaries", the named-build trim paragraph after it, and the store-trim liveness rows paragraph under "When the disk is full"
  → Decision: each build writes a sibling `<target>.new.<pid>` and renames it onto the target; concurrent builders need no lock or retry. This spec keeps "no lock" and adds exactly one bounded retry (R-3)
  → Constraint: the page names `build_le`, `update_le`, `trimLaunchers`, `launcherIdle`, `launcherNamedInArgv` as sources. Each paragraph is edited in the phase that changes its producer
- [ ] `ai/rules/no-layering.md`
  → Decision: no copy fallback when a hard link cannot be made. Replacing per-name copies with links means the per-name copy path is deleted; keeping it as a fallback is the hybrid the rule bans. A filesystem without hard links gets a refusal (AC-9)
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: every refusal the script adds names what failed, why, and the exact way out, and says that `bin/le-<name>/le` is unchanged
- [ ] `ai/rules/principles.md` - "declared once" and "silently wrong value"
  → Constraint: the store path is declared once in Go (`internal/le/go/toolchain/gotoolchain.go`, beside `NamedLauncherDir`) and the script's copy is pinned to it by the existing `TestNamedLauncherMatchesTheShellScript`, extended
  → Constraint: a size the trim cannot attribute is never reported as freed. Removing a name whose `le` is shared frees nothing, and the report MUST say so (AC-12)

**Key insights:**
- Identity is the SHA-256 of the produced binary, not a hash of inputs. The script already rebuilds on every `--name` call, so the produced bytes exist at no extra cost, and an output hash cannot miss an input (an embedded `.yang`, an inherited `GOFLAGS`, a vcs stamp) the way an input list did on 2026-09-13 (`plan/journal/stale-artifact-reused.md`)
- Store layout: one flat file `bin/.store/<sha256>` per distinct build. The file name is the hash, so executing it directly selects no le personality, which makes the name path the only working entry
- Hard link, not symlink: `detachedTrim` (`internal/le/scratch/storetrim.go`) re-executes `os.Executable()`. On Linux that resolves a symlink to the store path, whose parent is not `le-<name>`, so the child's `invokedBuildName` would answer something other than the name and `refuseWrongBuildName` would refuse it. A hard link resolves to the path it was executed through
- POSIX `rename()` between two links of the same inode succeeds and does nothing, so a no-change rebuild would leave its link file behind forever. The script MUST test `-ef` first and skip the relink
- Hard-link count is the trim's liveness signal for a store entry: a count of 1 means only the store holds it. Its ctime moves on every link and unlink, so ctime, not mtime, is the age

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `le` (335L) - the POSIX sh launcher. `build_le out` builds the le personality with feature tags, `CGO_ENABLED=0`, the go.mod toolchain pin and the patched GOROOT (`go run ./internal/le/go/stdpatch/goroot`), then `go build -tags ... -o out ./cmd/ze` with no `-trimpath`. `update_le target` builds into `target.new.$$`, removes it on failure with "le: the build failed, so bin/le is unchanged", and on success `mv -f` onto the target. `--name` (`check_name`, letters/digits/`._-`, no `..`) sets `binary=bin/le-<name>/le`, exports `ZE_LE_BUILD_NAME`, calls `update_le` on EVERY call, then `exec "$binary" "$@"`
  → Constraint: the final file name stays `le`, because `defaultDispatch` picks the personality from the binary's base name
  → Constraint: the sampled staleness warning is exempt for `--name`; unchanged
- [ ] `cmd/ze/le_build_name.go` (75L) - `refuseWrongBuildName` compares `ZE_LE_BUILD_NAME` with `invokedBuildName(os.Args[0])`, which answers the suffix of a parent directory `le-<suffix>` when the base is `le`, else the base name
  → Constraint: unchanged code. It reads `os.Args[0]`, which the script sets to `bin/le-<name>/le` through `exec`; a hard link keeps that path. A direct exec of `bin/.store/<digest>` answers `<digest>`, never a name a caller asked for, so it refuses under `ZE_LE_BUILD_NAME`
- [ ] `internal/le/go/toolchain/gotoolchain.go` (`NamedLauncherDir`, `NamedLauncherName`, `validLauncherName`) - `NamedLauncherDir(root, name)` answers `bin/le-<name>`; `NamedLauncherName(entry)` answers a build name only for `le-<lowercase-start valid name>` entries, never the platform fallback `le-<uname -s>-<uname -m>`; `validLauncherName` mirrors `check_name`. `TestNamedLauncherMatchesTheShellScript` (`gotoolchain_test.go`) pins the script to them
  → Constraint: `.store` does not start with `le-`, so `trimLaunchers` never reaches it. The store gets its own declaration here
- [ ] `internal/le/scratch/livetrim.go` (523L) - `trimLiveness` answers rows `launchers`, `sessions`, `testbins`, and removes nothing from them when no Claude CLI is in the process table. `trimLaunchers` walks `bin/`, and removes `bin/le-<name>/` when `launcherIdle` (directory AND `le` both older than `launcherIdleAge`, 24 h; symlink or non-directory never idle) and no process argv contains `bin/le-<name>/`. `measureEntry` adds `treeBytes(dir)` to the row's before and after, and a removal subtracts it
  → Constraint: `treeBytes` (`cachetrim.go`) sums allocated bytes per file with no inode dedup. With shared links it would report 137 MB freed for a removal that freed nothing: a silently wrong value (`ai/rules/principles.md`)
  → Constraint: `launcherIdle` reads the `le` mtime. A hard link carries the store entry's mtime, which can be days old; the directory mtime still moves on every call because the staging file is created and removed in it, so "both old" still means idle. The `launcherIdleAge` comment "rewrites bin/le-<name>/le on every call" becomes false and is rewritten (stale-comments)
- [ ] `internal/le/scratch/storetrim.go` (`detachedTrim`) - re-executes `os.Executable()` with the inherited environment, so the child carries `ZE_LE_BUILD_NAME`
  → Constraint: this is the call that rules out a symlink (Key insights)
- [ ] `internal/le/go/toolchain/gotoolchain_missing_test.go` (`TestTheLauncherRefusesWithoutGo`) - copies `le`, `feature-tags`, `feature-gates.txt` and `go.mod` into a `t.TempDir()` checkout and runs the copied script
  → Decision: the functional proof uses the same technique with a fixture module small enough to build in seconds, so it drives the real script through its real `--name` entry point without touching this checkout's `bin/`

**Behavior to preserve:**
- `./le --name <name> <command>...` builds from the working tree on every call and execs `bin/le-<name>/le` with `ZE_LE_BUILD_NAME=<name>`
- A failed build leaves `bin/le-<name>/le` as it was and removes its staging file
- A running process keeps the inode it started on; a peer never opens a half-written target
- `bin/le`, `--update`, the platform fallback `bin/le-<OS>-<arch>/le`, `bin/ze*`: untouched
- `refuseWrongBuildName` output and exit code 2
- The `launchers` trim row: same rule, same argv protection, same 24 h floor

**Behavior to change:**
- `--name` publishes `bin/le-<name>/le` as a hard link to `bin/.store/<sha256-of-binary>`, creating that entry when it does not exist
- The store trim gains a `launcher-store` row that removes an entry with link count 1 whose ctime is older than its floor
- The `launchers` row counts a file with link count above 1 as shared: neither its size before nor its removal is attributed to the row

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le --name <name> <command>...` from any shell, on Linux or macOS. Input: the working tree (sources, uncommitted edits, go.mod pin, feature manifest, inherited `GOFLAGS`)

### Transformation Path
1. `check_name` validates `<name>` (unchanged)
2. `build_le` writes `bin/le-<name>/le.new.<pid>` (the staging file, unchanged)
3. The script hashes the staging file with the first of `sha256sum` or `shasum -a 256` on PATH, and checks the answer is 64 lowercase hex
4. It creates `bin/.store/` when absent, then hard-links the staging file to `bin/.store/<digest>`. Link creation is atomic and fails when the entry exists; an existing entry is the same bytes by construction, so it is reused
5. When `bin/le-<name>/le` is already the same inode as the entry (`-ef`), nothing is relinked
6. Otherwise it hard-links the entry to `bin/le-<name>/le.link.<pid>` and renames that onto `bin/le-<name>/le`. If the entry vanished between steps 4 and 6 (a concurrent trim), it returns to step 4 once
7. It removes the staging file, then execs `bin/le-<name>/le`
8. Later, the store trim's `launcher-store` row removes an entry whose link count is 1 and whose ctime is past the floor

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| sh launcher ↔ filesystem | `ln`, `mv -f`, `test -ef` inside one tree (`bin/`), POSIX on Linux and macOS | No |
| sh launcher ↔ le process | `exec bin/le-<name>/le`, `ZE_LE_BUILD_NAME` | No |
| le process ↔ detached trim child | `os.Executable()` re-exec (`detachedTrim`) | No |
| store trim ↔ concurrent launcher | link count and ctime of `bin/.store/<digest>`; launcher retries once on a vanished entry | No |

### Integration Points
- `update_le` in `le` - the named branch calls a new publishing step after `build_le`; the shared `bin/le` branch keeps `update_le` unchanged
- `NamedLauncherDir` / `NamedLauncherName` in `gotoolchain.go` - gain a sibling declaration of the store directory, pinned to the script by `TestNamedLauncherMatchesTheShellScript`
- `trimLiveness` / `trimLaunchers` / `measureEntry` in `livetrim.go` - a new row and shared-link accounting
- `trimReport` rendering in `storetrim.go` - the new row renders through the existing row renderer, no new format

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Publishing stays inside the script's own build path; the trim stays inside `trimLiveness` |
| No unintended coupling (components stay isolated) | Yes | No product package is touched; `cmd/ze/le_build_name.go` is unchanged |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `update_le`'s staging file, `removeLivenessEntry`, `allocatedBytes`, `StoreTrim` rows |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The point of the change: N names, one inode |
| Registration over hardcoding, outbound | Yes | No command, view or handler is added; the trim row joins the row list `livenessRows` already owns |
| Registration over hardcoding, inbound | Yes | Searched: `NamedLauncherName` (the only Go reader of `bin/` entry names), `guestle.go` (writes under `tmp/qemu`, never `bin/`), `hookruntime/bash.go` (a comment only). The store path is declared once in `gotoolchain.go` and the script copy is pinned by test |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Two `go build` runs over byte-identical inputs, same toolchain, same flags and same HEAD produce byte-identical binaries, though the output path (`le.new.<pid>`) differs | Go reproducible builds; the output path is not embedded; the vcs stamp holds revision, time and a modified flag, not the dirty file list | Every name gets its own entry and nothing is saved | `TestNamedBuildsWithIdenticalInputsShareOneInode`, and the Phase 5 run over this checkout | unvalidated |
| A-2 | `os.Executable()` in a process started through a hard link answers the path it was executed through, on Linux (`/proc/self/exe`) and macOS | Linux procfs names the dentry used at exec; macOS answers the exec path | The detached trim child would refuse with "--name asked for" | The fixture binary prints `os.Executable()`; asserted equal to `bin/le-<name>/le` in `TestNamedBuildRunsUnderItsNamePath` | unvalidated |
| A-3 | `ln` (link(2)) and `mv -f` (rename(2)) are atomic on APFS, ext4, xfs and btrfs, and link(2) fails when the entry exists rather than replacing it | POSIX | Two builders could publish a partial entry | `TestConcurrentNamedBuildsConverge` | unvalidated |
| A-4 | A link-count change updates the inode ctime on Linux and macOS | POSIX link(2)/unlink(2) mark st_ctime for update | The trim floor would measure the entry's creation, not its last use | `TestLauncherStoreFloorFollowsTheLastLink` with a real link | unvalidated |
| A-5 | `sha256sum` exists on Linux (coreutils, busybox) and `shasum` on macOS | Both present on this host under `/usr/bin`; macOS ships `shasum` | `--name` refuses on that host | AC-10 refusal test; a manual run on the Mac recorded in Goal Validation | unvalidated |
| A-6 | `bin/le-<name>/` and `bin/.store/` sit on one filesystem | Both are children of `bin/`; nothing in the tree mounts into `bin/` | Hard link fails with EXDEV and AC-9's refusal fires | AC-9 message names the cause either way | unvalidated |
| A-7 | No le code executes a binary under `bin/.store/` directly | grep of `internal/le` and `cmd/ze` for `bin/` consumers (Architectural Verification) | A direct exec answers as a non-le personality | `TestStoreEntryIsNotANamedBuild` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A no-change rebuild renames a link onto another link of the same inode; rename(2) then does nothing and `le.link.<pid>` stays forever | `bin/le-<name>/` accumulates `le.link.*` files | `-ef` test before the relink, and the script removes its own `le.link.<pid>` on every exit path (AC-4) |
| R-2 | Two sessions publish the same digest at once | One `ln` fails | link(2) is atomic: the loser sees the entry exists and reuses it (AC-6) |
| R-3 | The trim removes an orphan entry between the launcher's "entry exists" and its link to the name | `ln` of the entry fails with no such file | One retry from step 4, the staging file is still held; a second vanish is a refusal naming the race (AC-7) |
| R-4 | Hashing 137 MB adds latency to every `--name` call | Wall time of `./le --name x help` | SHA-256 of 137 MB costs well under a second beside a multi-second link; measured once in Goal Validation |
| R-5 | A running process started from a now-orphaned entry loses it to the trim | A long run fails to re-exec itself | Unlink never affects a running image; a re-exec goes through `os.Executable()`, which is the name path (A-2), not the store entry. Same exposure as today's rename-over |
| R-6 | The trim report double-counts shared bytes or claims bytes it did not free | `launchers` SizeBefore larger than the store's real usage | Shared-link accounting (AC-12) |
| R-7 | An existing per-name copy (from before this spec) is never deduplicated | `bin/le-<name>/le` with link count 1 after upgrade | The first `--name <name>` call after this lands relinks it; an unused one ages out under the existing 24 h rule |
| R-8 | A filesystem with no hard links makes `--name` unusable where it worked before | AC-9 refusal seen in a guest or on an unusual mount | Refusal names the filesystem cause and the way out; no copy fallback (`ai/rules/no-layering.md`). Owner question Q-1 |
| R-9 | An entry is corrupted in place by a tool that writes through a link | A named build answers wrongly | Nothing in le writes through a link: every publish renames. The trim never trusts content; the entry's name is its claim only |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session's `./le --name` calls, which interop, functional and gate runs require (`ai/rules/commands.md`). A wrong binary under a name is the worst case: a probe reads another build's answer |
| How is it reverted? | Single commit revert. Existing hard links keep working as ordinary files after a revert; `bin/.store/` is then plain disk the owner can remove |
| Who else touches this path? | `internal/le/scratch` (store trim, other sessions' cache-trim work), `internal/le/go/toolchain` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le --name a help` then `./le --name b help`, identical inputs | → | the script's publishing step after `build_le` | `TestNamedBuildsWithIdenticalInputsShareOneInode` |
| `./le --name a` after a source edit | → | same step, new digest | `TestNamedBuildsWithDifferentEditsDoNotShare` |
| `./le scratch store-trim` | → | `trimLiveness` → `trimLauncherStore` | `TestTrimLauncherStoreRemovesUnlinkedEntries` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le --name a <cmd>` then `./le --name b <cmd>` over the same working tree | `bin/le-a/le` and `bin/le-b/le` report the same device and inode; `bin/.store/` holds exactly one entry, named the SHA-256 of that binary; link count is 3 |
| AC-2 | `./le --name a <cmd>`, a source edit that changes the binary, then `./le --name b <cmd>` | The two names report different inodes; `bin/.store/` holds two entries |
| AC-3 | `./le --name a <cmd>` after an edit that changes the binary | `bin/le-a/le` is a new inode equal to the new entry; a process started from the old binary before the call still runs on the old inode; the old entry still exists with link count 1 |
| AC-4 | `./le --name a <cmd>` twice with no change between | The second call leaves `bin/le-a/le` the same inode and leaves no `le.new.*` or `le.link.*` file in `bin/le-a/` |
| AC-5 | The named binary runs | `os.Args[0]` and `os.Executable()` both answer `bin/le-<name>/le`; `refuseWrongBuildName` answers 0; a detached trim child started through `os.Executable()` is not refused |
| AC-6 | `./le --name a` and `./le --name b` started at the same moment over identical inputs | Both exit 0; both names end on one inode; one store entry; no leftover staging or link file |
| AC-7 | The store entry is removed between the existence check and the name link | The call still publishes, from its own staging file, and exits 0. A second vanish refuses: names the entry, says another process removed it twice, says `bin/le-<name>/le` is unchanged, and says to re-run the same command |
| AC-8 | The build fails | Exit 1, staging removed, `bin/le-<name>/le` and `bin/.store/` unchanged; the existing "the build failed" message names `bin/le-<name>/le` rather than `bin/le` on the named path |
| AC-9 | Hard link creation fails for a reason other than "exists" (filesystem without hard links, EXDEV, permission) | Exit 1; stderr names the link that failed and `ln`'s own reason, says `bin/` must be on a filesystem that supports hard links (APFS, ext4, xfs, btrfs), says `bin/le-<name>/le` is unchanged, and names the way out: move the checkout to such a filesystem, or run the command without `--name` against a refreshed shared binary (`./le --update <cmd>`). Staging removed |
| AC-10 | Neither `sha256sum` nor `shasum` on PATH | Exit 1 before any link; stderr names both programs, the PATH searched, why the launcher needs them (naming the shared build), and how to get them on Linux (coreutils) and macOS (`shasum` ships with the system Perl); `bin/le-<name>/le` unchanged; staging removed |
| AC-11 | `./le scratch store-trim` with one store entry of link count 1 and ctime older than the floor, one of link count 2, and one of link count 1 younger than the floor | Only the first is removed; the `launcher-store` row reports 1 removed, 2 kept, and bytes before and after measured from the entries' allocated size |
| AC-12 | The `launchers` row removes `bin/le-x/` whose `le` has link count above 1 | The row counts that `le` as shared: it adds 0 to SizeBefore for it and the removal subtracts 0; the bytes stay on the `launcher-store` row |
| AC-13 | `bin/.store/` absent | The `launcher-store` row is a skip that says "no bin/.store directory"; exit 0 |
| AC-14 | No Claude CLI in the process table | `launcher-store` follows the same rule as `launchers`: skipped with "no session process visible" |
| AC-15 | `bin/.store/<digest>` executed directly with `ZE_LE_BUILD_NAME=a` | Refused by `refuseWrongBuildName` naming `a` and the digest; exit 2 |
| AC-16 | `./le` without `--name`, and `./le --update` | Behavior unchanged: `bin/le` is never a link into the store and is never relinked |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNamedLauncherMatchesTheShellScript` (extended) | `internal/le/go/toolchain/gotoolchain_test.go` | The script's store path equals the Go declaration `NamedLauncherStoreDir` | |
| `TestTrimLauncherStoreRemovesUnlinkedEntries` | `internal/le/scratch/livetrim_test.go` | AC-11 with real files and real links in `t.TempDir()`; floor via the pass clock | |
| `TestLauncherStoreFloorFollowsTheLastLink` | `internal/le/scratch/livetrim_test.go` | A-4: an entry relinked now is younger than the floor even when its mtime is old | |
| `TestTrimLaunchersCountsSharedBinaryOnce` | `internal/le/scratch/livetrim_test.go` | AC-12 | |
| `TestTrimLauncherStoreSkips` | `internal/le/scratch/livetrim_test.go` | AC-13, AC-14 | |
| `TestStoreEntryIsNotANamedBuild` | `cmd/ze/le_build_name_test.go` | AC-15: `invokedBuildName` of `bin/.store/<64 hex>` answers the digest and `refuseWrongBuildName` refuses | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Store entry link count | 1 to any | 1 is removable, 2 is kept | 0 cannot be observed on a named file | N/A |
| Store entry age against the floor | 0 to any | exactly the floor is removable (`olderThan` uses `>=`) | floor minus 1 s is kept | N/A |
| Digest length | 64 hex | 64 | 63 is not a store entry and is never removed | 65 is not a store entry and is never removed |

### Functional Tests
The launcher is a shell script, so its functional tests run the REAL script file, copied with `feature-tags` and `feature-gates.txt` into a `t.TempDir()` checkout holding a tiny fixture module: a `cmd/ze` main that prints `os.Args[0]`, `os.Executable()` and a compiled-in marker, and an `internal/le/go/stdpatch/goroot` stand-in that prints the toolchain's own GOROOT. The fixture's `cache/` links to this checkout's Go cache so builds stay warm. `TestTheLauncherRefusesWithoutGo` already uses this technique, and it never writes this checkout's `bin/`. The tests run through `./le job run label unit-pkg command go test ... ./internal/le/go/toolchain/`.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestNamedBuildsWithIdenticalInputsShareOneInode` | `internal/le/go/toolchain/launcher_store_test.go` | AC-1, A-1 | |
| `TestNamedBuildsWithDifferentEditsDoNotShare` | same | AC-2, AC-3 (edit the marker; a sleeping child of the old build keeps its inode) | |
| `TestNamedRebuildWithoutChangeLeavesNoStaging` | same | AC-4, R-1 | |
| `TestNamedBuildRunsUnderItsNamePath` | same | AC-5, A-2 | |
| `TestConcurrentNamedBuildsConverge` | same | AC-6, A-3 | |
| `TestNamedBuildSurvivesAVanishedStoreEntry` | same | AC-7 (an `ln` wrapper first on PATH removes the entry before the name link, once, then twice) | |
| `TestNamedBuildFailureLeavesTheNameAlone` | same | AC-8 | |
| `TestNamedBuildRefusesWithoutHardLinks` | same | AC-9 (an `ln` wrapper first on PATH fails with "Operation not permitted") | |
| `TestNamedBuildRefusesWithoutAHashTool` | same | AC-10 (PATH holds the script's tools but neither hash program) | |
| `TestSharedLauncherIsNeverLinked` | same | AC-16 (`./le help` and `./le --update help` leave `bin/le` with link count 1) | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `le` - publishing step for the named branch (hash, store link, `-ef` skip, relink by rename, one retry, the three refusals); the named-path failure message
- `internal/le/go/toolchain/gotoolchain.go` - `NamedLauncherStoreDir(root)` beside `NamedLauncherDir`, and a predicate answering whether a `bin/.store` entry name is a 64-hex digest
- `internal/le/go/toolchain/gotoolchain_test.go` - extend `TestNamedLauncherMatchesTheShellScript`
- `internal/le/scratch/livetrim.go` - the `launcher-store` row and `trimLauncherStore`; shared-link accounting in the launchers row; rewrite the `launcherIdleAge` and `launcherIdle` comments
- `internal/le/scratch/livetrim_test.go` - the unit tests above; existing row-index assertions updated for the new row
- `cmd/ze/le_build_name_test.go` - AC-15
- `docs/contributing/running-commands.md` - "Launcher builds do not overwrite running binaries", the named-build trim paragraph, the store-trim liveness rows paragraph
- `docs/architecture/system-architecture.md` - the paragraph on named builds
- `ai/doors/le-tooling.md` - the `bin/le` row of "Who depends on it from outside" gains the store and its link rule

## Files to Create
- `internal/le/go/toolchain/launcher_store_test.go` - the script-driving functional tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config; le tooling |
| YANG validation constraints | N-A | No YANG leaf |
| YANG custom validators | N-A | No YANG leaf |
| CLI commands/flags | No | `--name` keeps its grammar; no new flag |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No YANG |
| Functional test for new RPC/API | N-A | No RPC; the script tests above are the functional proof |
| Pipe completeness | N-A | The new trim row renders through the existing `trimReport` renderer, `json` shape unchanged apart from the extra row |
| Env var registration | N-A | No new env var; `ZE_LE_BUILD_NAME` unchanged |
| Doctor check for runtime dependencies | No | The new dependency (`sha256sum` or `shasum`) is a development-host program the launcher refuses on by name (AC-10); `ze doctor` covers the shipped product, never le |
| Prometheus counters/metrics | N-A | Tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | Development tooling, no `features/` entry |
| 2 | Config syntax changed? | N-A | No config |
| 3 | CLI command added/changed? | N-A | No ze command |
| 4 | API/RPC added/changed? | N-A | None |
| 5 | Plugin added/changed? | N-A | None |
| 6 | Has a user guide page? | Yes | `docs/contributing/running-commands.md` (the launcher and store-trim sections) |
| 7 | Wire format changed? | N-A | None |
| 8 | Plugin SDK/protocol changed? | N-A | None |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | None |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/running-commands.md` (named builds carry every functional and interop run); `docs/functional-tests.md` does not describe the launcher's storage and needs no edit |
| 11 | Affects daemon comparison? | N-A | Tooling |
| 12 | Internal architecture changed? | Yes | `docs/architecture/system-architecture.md` (named builds paragraph), `ai/doors/le-tooling.md` |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/contributing/running-commands.md` anchors `le -- build_le, update_le`, `gotoolchain.go -- NamedLauncherDir, NamedLauncherName`, `livetrim.go -- trimLaunchers, launcherIdle, launcherNamedInArgv` and `trimLiveness`: all three paragraphs are edited. `cmd/ze/le_build_name.go` is not changed (only its test). `docs/architecture/core-design.md`, declared by `gotoolchain.go`'s `// Design:` header for the Go toolchain environment, is unaffected: the new `NamedLauncherStoreDir` declaration changes no toolchain environment le builds under, and the page describes none of the launcher's storage. `docs/contributing/testing.md` (cites `EnvOptions`, `Environment`) and `docs/architecture/testing/verify-freshness-scope.md` (cites `New`) mention `gotoolchain.go` for symbols this spec does not change, so both are unaffected. Run `./le spec citation anchors spec plan/post-release/engineering/spec-le-shared-build-launcher.md` at implementation start for the derived list |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `./le --name <name> <command>` examples in `ai/rules/commands.md` and `running-commands.md` stay valid; the sentence "lands in `bin/le-<name>/le` and is rebuilt on every call" stays true |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the fixture checkout helper and the three wiring tests, failing
   - Tests: `TestNamedBuildsWithIdenticalInputsShareOneInode`, `TestNamedBuildsWithDifferentEditsDoNotShare`, `TestTrimLauncherStoreRemovesUnlinkedEntries`
   - Files: `launcher_store_test.go`, `livetrim_test.go`, the `NamedLauncherStoreDir` declaration
   - Verify: the identical-inputs test fails on distinct inodes (today's behavior), proving the fixture drives the real script
2. **Phase: Publish in the script** - hash, store link, `-ef` skip, rename, retry, the three refusals
   - Tests: AC-1 to AC-10 and AC-16 functional tests; the extended `TestNamedLauncherMatchesTheShellScript`
   - Files: `le`, `gotoolchain.go`, `gotoolchain_test.go`; then the `running-commands.md` launcher section and `system-architecture.md` in the same phase
   - Verify: each test red against the unchanged script, green after; read each log for the test's own name
3. **Phase: Trim** - the `launcher-store` row, shared-link accounting, comment rewrites
   - Tests: AC-11 to AC-14 unit tests, A-4
   - Files: `livetrim.go`, `livetrim_test.go`; then the two `running-commands.md` trim paragraphs and `le-tooling.md`
   - Verify: tests red, then green; `./le --name <name> scratch store-trim` in this checkout shows the new row
4. **Phase: Build-name guard test** - AC-15
   - Files: `cmd/ze/le_build_name_test.go`
5. **Phase: Goal evidence in this checkout** - two real `./le --name` calls over this tree; `ls -li` of both names and the store shows one inode; `bin/` size before and after recorded; the same on the owner's Mac for A-5

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named function |
| Correctness | No path in the script writes or links onto `bin/le-<name>/le` except by rename; every refusal path removes staging and `le.link.<pid>` |
| Correctness | The trim never removes an entry with link count above 1, a non-regular file, a symlink, or a name that is not 64 hex |
| Naming | Store path declared once in Go; the script's copy pinned by test |
| Portability | No GNU-only flag in the script (`stat -c`, `ln -T`, `readlink -f`); `-ef` is supported by dash, bash, busybox ash and the macOS sh |
| Rule: no-layering | No per-name copy fallback exists anywhere |
| Rule: user-facing-errors | Each refusal names the problem, why, `bin/le-<name>/le` unchanged, and the exact way out |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Two identical named builds share one inode in this checkout | `ls -li` of `bin/le-<a>/le`, `bin/le-<b>/le` and `bin/.store/` |
| Store trim reports the new row | `./le scratch store-trim` output shows `launcher-store` |
| Script tests ran | The job log carries `--- PASS: TestNamedBuildsWithIdenticalInputsShareOneInode` and each sibling |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `<name>` still passes `check_name` before any path is built; the digest is checked to be 64 lowercase hex before it names a path |
| Path traversal | The trim removes only regular files named by 64 hex directly under `bin/.store/`, read with `Lstat` |
| Resource exhaustion | Store entries bounded by the trim; a failed publish removes its staging file |
| Trust | An existing entry is reused by name; the store is writable only by the checkout owner, the same trust `bin/le` already carries |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-1 broken (builds not reproducible) | STOP: the output hash saves nothing; report the differing bytes (`cmp`) and ask which way to fix |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- rename(2) of one link onto another link of the same inode is a successful no-op (POSIX), the trap behind R-1
- An unlink moves the surviving link's ctime, so an entry orphaned by the launchers row is protected by the floor until a later trim pass

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Identity is SHA-256 of the produced binary | A hash of inputs (sources incl. uncommitted edits, tags, toolchain, `GOFLAGS`); the Go build ID from `go tool buildid` | `--name` already builds every call, so the bytes are free. An input list is a second declaration of what the build reads and drifted before (embedded `.yang`, 2026-09-13). The build ID is a Go-internal format; a byte hash is a measured fact about the file |
| Hard link | Symlink | `detachedTrim` re-execs `os.Executable()`, which resolves a symlink on Linux to the store path and would make the child refuse; a hard link is indistinguishable from today's file to every reader |
| Flat `bin/.store/<digest>` | `bin/.store/<digest>/le` | A file named `le` would select the le personality and answer as the shared `bin/le` when executed directly; a digest name selects nothing |
| Refuse when hard links fail | Fall back to a private copy | `ai/rules/no-layering.md` bans keeping the old path as a fallback |
| Link count and ctime judge an entry | A reference list or lock file | A list is state several sessions write; the kernel already counts links |
| One retry on a vanished entry, no lock | A lock around publish and trim | The launcher contract is "no lock"; one retry with the staging file in hand closes the only race the trim adds |
| Shared `bin/le` stays out of the store | Link `bin/le` too | O-1 names `le-<name>` builds; owner question Q-2 |

## Known Limitations
- `bin/le` and the platform fallback `bin/le-<OS>-<arch>/le` keep their own copies (Q-2)
- The `./ze` root launcher's `bin/ze` is not touched; O-1 names le builds only

## Owner Questions

Answered 2026-10-22 (Thomas): spec approved; Q-1 "yes require hardlink".
Q-2 and Q-3 confirmed 2026-10-23 (Thomas chose the readings below: `le-<name>`
only, and a 1 hour floor).

| ID | Question | Reading this spec proceeds under |
|----|----------|----------------------------------|
| Q-1 | On a filesystem without hard links, refuse `--name` (this spec, per no-layering) or keep a private per-name copy? | Refuse |
| Q-2 | Should the shared `bin/le` (and the platform fallback) also publish through the store? | No, O-1 names `le-<name>` only |
| Q-3 | The floor for an unlinked store entry, a new fixed constant beside owner decision D-3's ages | 1 hour: the retry covers the race, the floor bounds churn |

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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-le-shared-build-launcher.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
