# Spec: le-lab-storage-trim

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P3 |
| Release reason | Release-independent le tooling: QEMU and Docker lab artifacts fill the development disk and nothing trims them |
| Depends | spec-lab-docker-cleanup-and-go-test-containers |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decisions that approve this work:

| Id | Date | Decision | Thomas's words |
|----|------|----------|----------------|
| O-1 | 2026-10-25 | Write a spec for cleaning what QEMU and Docker runs leave on disk; do not implement it in the session that writes it | "write a spec for cleaning qemu and docker but do not implement them" |
| O-2 | 2026-10-25 | The spec covers storage only, as a new spec. Orphaned containers, networks and run-owned image tags stay with `spec-lab-docker-cleanup-and-go-test-containers` (G-1 there), which this spec builds on | "Storage only, new spec" |
| O-3 | 2026-10-25 | Research understanding approved: the Docker build cache is bounded by a daemon configuration, never a prune command; QEMU run directories get an owner marker; the Colima disk reclaim is proven before anything relies on it | "Yes, design it" |
| O-5 | 2026-10-25 | The spec was saved as ready without the final write-gate review, which the owner delegated; its weakest part is A-1 (the Colima disk may not shrink), and Phase 1 stops and asks if it does not | "i am going to be afk, complete this work without me" |
| O-4 | 2026-10-25 | Design approved with budgets: build cache 10G in the daemon configuration; docker-host images keep the live, the current and the newest two; QEMU run directories removed when their owner is dead and past a one-hour floor; QEMU logs older than seven days; Go downloads keep the current and the previous release | "Approve, 10G build cache" |

Measured on the Mac on 2026-10-25:

| Store | Size | Today |
|-------|------|-------|
| Colima VM disk `~/.colima/_lima/colima-ze/disk` | 16G allocated (20G apparent) | holds every Docker image and the build cache; nothing shrinks it |
| Docker build cache (inside that disk) | 8.2G, 5.0G private | no label filter exists for it, so `./le` never trims it |
| Docker images (inside that disk) | 4.7G, 14 images, 99% unused | `CleanupOrphans` removes only retained tags with a dead owner |
| `~/.cache/ze/docker-host` | 4.0G: 12 keys of about 344M | `(*builder).build` never evicts |
| `tmp/appliance-mpls-qemu/run-*` | 3 x 1.3G | only killed or kept runs leave one; nothing removes it |
| `tmp/qemu/go-cache` | 1.4G | orphaned by commit 3894cc7f89; no writer, no row |
| `tmp/qemu/go-dl` | 192M, three Go releases | old releases pile up |
| `tmp/qemu/drain-setpgid` | 298M | hand-made agent scratch, no writer |
| `tmp/vpp-hugepages-qemu/run-*` | 8 x 2.5M | as appliance runs |
| `tmp/qemu/serial-*.log`, `.qemu-errors-*`, `docker-lab-console-*.log` | small, many | kept until a run passes, then pile up |
| `~/.cache/ze/alpine-iso` | 395M | both versions live (3.24.3 QEMU, 3.21.3 kernel builder); not a target |

Goal: on every host, the hourly store trim and `./le scratch store-trim`
hold these stores to the budgets of O-4, removing only what no live run uses,
and the Colima disk gives freed space back to the host.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/running-commands.md` - "When the disk is full", the store trim rows, the kernel caches never trimmed
  → Constraint: the kernel caches (`runtime-kernel/`, `installer-kernel/`) are never a trim target; every new row calls the kernel guard (`heldKernelCache`), which today runs only inside `cleanTargets`
  → Decision: new stores join the existing trim (`trimStoresScanning`) and its report; no second cleaner
- [ ] `docs/architecture/testing/qemu-integration.md` - guest layout, `tmp/qemu`, `vm-record-<pid>.json`, the docker-lab `snapshot=on`
  → Constraint: the guest reaches the host checkout over 9p; a trim started in the guest sees host pids as absent, so no trim runs in a guest
  → Constraint: `vm-record-<pid>.json` names the QEMU and le pids but not the run's directory; a directory needs its own owner marker
- [ ] `docs/architecture/testing/interop.md` - image build, shared tags, retained tags, `NO_BUILD=1`
  → Constraint: a shared tag (`ze-interop:latest` and siblings) is reused by `NO_BUILD=1` and is never a removal candidate
- [ ] `docs/architecture/appliance/docker-host.md` - the docker-host image, its daemon configuration (`/etc/docker/daemon.json`, line 162), the cache layout `~/.cache/ze/docker-host/<arch>/<key>/docker-host.img` keyed by `ImageKey`, `setup.lock`
  → Decision: the build cache bound is a BuildKit garbage-collection setting in the image's daemon configuration (O-3); the daemon trims itself, and no `builder prune` runs on the shared daemon (`ai/META-DECISIONS.md`)
  → Constraint: eviction of `<arch>/<key>/` entries takes `setup.lock`, the lock `EnsureImage` holds while it builds, so an entry being written is never removed
- [ ] `docs/architecture/appliance/build-artifacts.md` - the XDG cache tiers under `~/.cache/ze/` and "Cache invalidation for downloads is manual"
  → Constraint: the page states downloads have no expiry; the docker-host eviction makes that false for docker-host images and the page is updated in that phase
- [ ] `plan/post-release/engineering/spec-lab-docker-cleanup-and-go-test-containers.md` - G-1 owner labels and the orphan sweep
  → Constraint: owner liveness is judged by `ownerJudge.judge` from the `ze.lab.*` labels; this spec reuses it, never a second liveness rule
- [ ] `ai/META-DECISIONS.md` - "Many sessions share one Docker daemon: never run a host-wide destructive Docker command"
  → Constraint: every Docker removal names its object and carries `./le`'s owner label with a dead owner

**Key insights:**
- The largest store is the Colima VM disk, and it shrinks only if the guest discards freed blocks (A-1); every Docker-side removal is worthless on the host until that holds.
- The docker-host image cache needs the eviction rule the kernel cache already has (`evictKeepN`, `internal/appliance/cache.go`), plus the key `colima-ze.started` records.
- `listResources` (`internal/le/interoplab/orphans.go`) skips `:<none>` rows, so an untagged lab image is invisible to the sweep.
- `kernelbuilder` builds `ze-kernel-builder` and mounts volumes `ze-kernel-build`, `ze-kernel-work` without any label.
- A new trim store is a hand-appended call in `trimStoresScanning`; a Docker row runs commands under `storeTrimTimeout` and must go through `interoplab.Docker` (the arch tier check confines docker invocation to interoplab).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/scratch/storetrim.go` - `trimStoresScanning` runs `cleanTargets` rows, then `livenessStores` (launchers, sessions, testbins), then `testTempRow`; `storeTrimTimeout`; budget entries via `readBudget`
- [ ] `internal/le/scratch/cacheclean.go` - `cleanTargets`, `heldKernelCache`, `pathWithin`, `cachePass`, `staleCacheCopies`
- [ ] `internal/le/scratch/livetrim.go` - liveness rows, `trimTestTemp`, `olderThan`, `removeLivenessEntry`, `measureEntry`, `sessionProcessVisible`
- [ ] `internal/le/interoplab/docker.go` - `(*Docker).Build` (shared tag, `ze-*-retained:<rand>`, owner labels), `ReleaseImage`, `Pull`
- [ ] `internal/le/interoplab/orphans.go` - `CleanupOrphans`, `listResources` (skips `:<none>`), `ownerIdentity.labelArguments`, `ownerJudge.judge`
- [ ] `internal/le/setup/dockerhost/build.go` - `(*builder).build`, `EnsureImage`, `ImageKey`, `sourceDigest`; no eviction
- [ ] `internal/le/setup/dockerhost/colima.go` - `readColimaRecord` (`colima-ze.started`), `colimaHost.start`, `zeDockerContext`
- [ ] `internal/le/test/qemu/run.go` - `ensureScratch`, `setupCommand` (`go-dl`), `keepAliveProfile`
- [ ] `internal/le/test/qemu/vmrecord.go` - `recordVM`, `reapOrphanVMs`, `reapOne`
- [ ] `internal/le/test/qemu/mplsboot.go` - `workDir` (`MkdirTemp run-`), deferred removal unless `Keep`
- [ ] `internal/le/test/qemu/hugepages.go` - `workDir`; its comment "A failed run leaves the image" contradicts the unconditional defer
- [ ] `internal/le/test/qemu/dockerlab.go` - `docker-lab-<n>` with deferred removal, console logs
- [ ] `internal/appliance/kernelbuilder/recipe.go` - `ze-kernel-builder` build and `containerRunArgs` volumes, unlabelled
- [ ] `internal/appliance/cache.go` - `evictKeepN`, `KernelCacheNamespaces`

**Behavior to preserve:**
- Shared lab image tags, pinned upstream images, and the alpine ISOs are never removed
- The kernel caches are never removed
- A run's own teardown keeps working as today
- `CleanupOrphans` keeps its rules for containers, networks and retained tags

**Behavior to change:**
- New trim rows for the docker-host image cache, QEMU run directories, QEMU leftovers, untagged dead-owner lab images
- An owner marker in every QEMU run directory
- Labels on the kernel builder's image and volumes
- A build cache bound in the docker-host image's daemon configuration
- The Colima disk gives freed blocks back to the host
- The stale `workDir` comment in `hugepages.go` is corrected

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The hourly detached store trim (`storeTrimBeforeDispatch`), and `./le scratch store-trim` on demand

### Transformation Path
1. `trimStoresScanning` runs the existing groups, then the new rows
2. docker-host row: list `<arch>/<key>/` under the docker-host cache, keep the `colima-ze.started` key, the current `ImageKey`, the two newest; remove the rest
3. QEMU run-directory row: for each `run-*`, `docker-lab-*` under the QEMU work roots, read its owner marker; remove when the owner is dead (`procscan`, pid plus start time) and the directory is older than one hour; a directory with no marker older than the floor is removed only when no QEMU or le process of this checkout is alive
4. QEMU leftovers row: remove `tmp/qemu/go-cache`; keep the current and previous release under `tmp/qemu/go-dl`; remove logs older than seven days
5. Docker image row, through `interoplab.Docker`: untagged images carrying `ze.lab=1` whose owner is dead, and the kernel builder's labelled image and volumes with a dead owner, removed by id
6. Colima disk: after a Docker-side removal, run the guest's discard (A-1) so the host file shrinks
7. Build cache: no row; the daemon's own garbage collection holds it to 10G

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ Docker daemon | `interoplab.Docker` over the `colima-ze` context | No |
| le ↔ Colima guest | the guest discard command | No |
| Host ↔ QEMU guest | the trim never runs in a guest | No |

### Integration Points
- `trimStoresScanning` (`storetrim.go`) - the new rows
- `ownerJudge.judge`, `ownerIdentity` (`interoplab/orphans.go`) - liveness for images and run directories
- `evictKeepN` (`appliance/cache.go`) - reused for the docker-host image cache
- docker-host image build (`setup/dockerhost`) - the daemon configuration gains the build cache bound

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Docker access only through `interoplab.Docker` |
| No unintended coupling (components stay isolated) | Yes | scratch calls an interoplab entry point for the Docker row |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `evictKeepN`, `ownerJudge`, the trim report |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling |
| Registration over hardcoding, outbound | Yes | rows join the existing trim |
| Registration over hardcoding, inbound | No | `trimStoresScanning` is a hand-appended list today; this spec adds to it and does not fix that (Known Limitations) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Colima guest can discard freed blocks so `~/.colima/_lima/colima-ze/disk` shrinks on the host | Lima disks are sparse; discard depends on the virtio disk and the guest filesystem | Docker-side removal frees nothing on the host; STOP and ask the owner whether recreating the VM over a budget is acceptable | Phase 1: remove an image, run the guest discard, compare `du` of the disk before and after | unvalidated |
| A-2 | BuildKit honours a garbage-collection storage bound from `/etc/docker/daemon.json` in the docker-host image | Docker documents `builder.gc` | The build cache stays unbounded | `docker system df` after builds past 10G | unvalidated |
| A-3 | The daemon configuration can be changed in the docker-host image without breaking the data-disk layout | `docker-host.md` line 162 | Image rebuild breaks Docker start | Rebuild and `./le test integration` one lab | unvalidated |
| A-4 | An untagged `ze.lab=1` image whose owner is dead is never needed by a live run | Shared tags point at live images; retained tags hold run images | A live run loses an image | Unit test over `listResources` with live and dead owners | unvalidated |
| A-5 | The QEMU guest never starts the hourly trim | Not verified; only `internal/test/runner` sets `storeTrimOffEntry` | A guest trim judges host runs dead | Grep plus a guest run with the trim stamp due | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Evicting the docker-host image the Colima VM booted from | Colima start fails | Keep the `colima-ze.started` key always; test |
| R-2 | Removing a live slow run's directory | A run fails mid-way | Owner marker plus one-hour floor; no-marker directories only when no QEMU or le process is alive |
| R-3 | A trim inside a QEMU guest | Host run directories vanish | A-5; set the trim off in the guest environment |
| R-4 | Docker daemon down or slow | Row times out | Skipped row, never failed; `storeTrimTimeout` bounds it |
| R-5 | Guest discard is slow or locks the VM | Trim takes minutes | Run discard detached and only after a Docker-side removal |
| R-6 | Removing a kernel cache through a new row | Kernel rebuild | Every new row calls `heldKernelCache`; test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A lab or QEMU run loses its directory or image and fails; a lab image rebuilds |
| How is it reverted? | Single commit revert; the daemon configuration change needs a docker-host image rebuild |
| Who else touches this path? | `spec-lab-docker-cleanup-and-go-test-containers` (labels, orphan sweep), `spec-le-build-cache-cap` (store trim cadence) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le scratch store-trim` with seeded docker-host keys, QEMU run directories and leftovers | → | the new rows in `trimStoresScanning` | `test/ui/le-lab-storage-trim.ci` |
| `./le scratch store-trim` with a reachable Docker daemon and labelled dead-owner images | → | the Docker image row | `TestTrimRemovesUntaggedDeadOwnerLabImages` (`internal/le/interoplab/orphans_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | docker-host cache holds the recorded key, the current key and five older keys | The recorded, current and two newest remain; three are removed; the row reports sizes |
| AC-2 | A QEMU run directory whose marker names a dead owner, older than one hour | Removed |
| AC-3 | A run directory whose owner is alive, or younger than one hour | Kept |
| AC-4 | A run directory without a marker, older than one hour, while a QEMU or le process of this checkout is alive | Kept and named on the row |
| AC-5 | `tmp/qemu/go-cache` present | Removed |
| AC-6 | `tmp/qemu/go-dl` holds three releases | The current and the previous remain |
| AC-7 | QEMU logs older than seven days | Removed; younger kept |
| AC-8 | Untagged image with `ze.lab=1` and a dead owner | Removed by id; a live-owner or unlabelled image is kept |
| AC-9 | The kernel builder builds and runs | Its image and volumes carry the owner labels; a dead-owner volume is removed by name |
| AC-10 | Builds push the build cache past 10G | `docker system df` shows it held near 10G by the daemon; no prune command ran |
| AC-11 | A Docker-side removal | The Colima disk's allocated size on the host drops (A-1) |
| AC-12 | Docker daemon down | Docker rows skipped with the reason; other rows run; exit 0 |
| AC-13 | Trim inside a QEMU guest | No trim runs |
| AC-14 | Any new row | Kernel caches untouched |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTrimDockerHostImagesKeepsRecordedCurrentAndNewest` | `internal/le/setup/dockerhost/build_test.go` | AC-1, R-1 | |
| `TestQEMURunDirCarriesOwnerMarker` | `internal/le/test/qemu/mplsboot_test.go` | marker written | |
| `TestTrimQEMURunDirs` | `internal/le/scratch/storetrim_test.go` | AC-2, AC-3, AC-4 | |
| `TestTrimQEMULeftovers` | `internal/le/scratch/storetrim_test.go` | AC-5, AC-6, AC-7 | |
| `TestTrimRemovesUntaggedDeadOwnerLabImages` | `internal/le/interoplab/orphans_test.go` | AC-8, A-4 | |
| `TestKernelBuilderLabelsImageAndVolumes` | `internal/appliance/kernelbuilder/recipe_test.go` | AC-9 | |
| `TestDockerRowsSkipWhenDaemonDown` | `internal/le/scratch/storetrim_test.go` | AC-12 | |
| `TestNoTrimInsideQEMUGuest` | `internal/le/test/qemu/run_test.go` | AC-13, A-5 | |
| `TestNewRowsNeverReachTheKernelCache` | `internal/le/scratch/cacheclean_kernel_test.go` | AC-14 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Run-directory floor | 1h fixed | 1h | N-A | N-A |
| Log age | 7 days fixed | 7d | N-A | N-A |
| docker-host keep | newest 2 plus live and current | 2 | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-lab-storage-trim` | `test/ui/le-lab-storage-trim.ci` + fixture | Seeded docker-host keys, run directories with live and dead markers, go-dl releases and old logs; `./le scratch store-trim` removes exactly AC-1 to AC-7's targets and reports each row | |

Docker and Colima behaviour (AC-8 to AC-11) is proven on the Mac against `colima-ze`, recorded in the spec, with a forced red per row.

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/scratch/storetrim.go`, `livetrim.go`, `cacheclean.go` - new rows, kernel guard on each
- `internal/le/setup/dockerhost/build.go` - eviction entry point; daemon configuration with the build cache bound
- `internal/le/interoplab/orphans.go` - untagged dead-owner images; volumes by label
- `internal/appliance/kernelbuilder/recipe.go` - owner labels on image and volumes
- `internal/le/test/qemu/mplsboot.go`, `hugepages.go`, `dockerlab.go` - owner marker; `hugepages.go` stale comment
- `internal/le/test/qemu/run.go` - trim off inside the guest
- Docs: `docs/contributing/running-commands.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/appliance/docker-host.md`, `docs/architecture/appliance/build-artifacts.md`, `docs/architecture/testing/interop.md`

## Files to Create
- `test/ui/le-lab-storage-trim.ci`, `internal/test/fixture/ui_fixture_le_lab_storage_trim.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | No | rows join `./le scratch store-trim`; no new verb |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no new command |
| Functional test for new RPC/API | Yes | `test/ui/le-lab-storage-trim.ci` |
| Pipe completeness | N-A | existing report |
| Env var registration | No | budgets are fixed by O-4 |
| Doctor check for runtime dependencies | No | no new runtime dependency; Docker and Colima already have setup rows |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | developer tooling |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md`: the new trim rows |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | contributor docs only |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md`, `docs/architecture/appliance/docker-host.md`, `docs/architecture/appliance/build-artifacts.md`, `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | No | tooling |
| 12 | Internal architecture changed? | No | rows inside the existing trim |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/post-release/engineering/spec-le-lab-storage-trim.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the trim report examples in `running-commands.md` |

## Implementation Steps

1. **Phase: Prove A-1** - remove one dead image on `colima-ze`, run the guest discard, measure the host disk; record the result. If the disk does not shrink, STOP and ask the owner
2. **Phase: Wiring** - the new rows registered in `trimStoresScanning` as stubs; `le-lab-storage-trim.ci` failing
3. **Phase: docker-host eviction** - AC-1, R-1
4. **Phase: QEMU** - owner marker, run-directory row, leftovers row, guest trim off; AC-2 to AC-7, AC-13; `hugepages.go` comment
5. **Phase: Docker** - untagged dead-owner images, kernel builder labels and volumes, build cache bound in the daemon configuration, guest discard after removal; AC-8 to AC-12
6. **Phase: Docs and kernel guard** - AC-14, every page in the Documentation checklist

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No Docker command removes anything without naming it and checking a dead owner |
| Correctness | No prune command, host-wide or builder, anywhere in the diff |
| Data flow | Docker access only through `interoplab.Docker` |
| Rule: no-layering | One liveness judge (`ownerJudge`), one eviction rule (`evictKeepN`) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| New trim rows | `./le scratch store-trim` report lists them |
| A-1 proven | Recorded before and after `du` of the Colima disk |
| No prune | grep the diff for `prune` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path handling | Directory names from the filesystem are matched by prefix, never as glob patterns |
| Removal scope | Every removal is under a known root and passes the kernel guard |
| Shared daemon | No command touches an unlabelled or live-owner resource |

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
| full-disk-false-red.md | 2026-09-02 | kernel builder Docker volumes; colima datadisk unbounded and never shrinks |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Rows in the hourly store trim | A separate `./le lab clean` command | The full-disk history shows on-demand cleaners are not run |
| Build cache bounded by daemon configuration | `docker builder prune` | No label filter exists for build cache; a prune is daemon-wide, banned on the shared daemon |
| Owner marker per run directory | Age-only removal | Age alone removes a slow live run's directory |
| docker-host eviction reuses `evictKeepN` | A new eviction rule | One rule for both image caches |

## Known Limitations
- `trimStoresScanning` stays a hand-appended list; making stores register themselves is not in this spec.
- `tmp/qemu/drain-setpgid` and similar hand-made scratch have no writer; the leftovers row does not guess at them.

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
- [ ] The targeted tests for what changed pass; the full gate runs once over the closing commits
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
