# Spec: snapshot-builds-and-built-commits

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Build and gate tooling: a commit that does not compile, or a verdict on another session's half-written file, costs release evidence, but no shipped behaviour changes |
| Depends | - (`spec-verification-debt-derived` closed on 2026-10-23: the derived debt is `internal/le/commit/owed.go`, the green record `internal/le/verify/green.go`, both described in `docs/architecture/testing/verify-freshness-scope.md`) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Root cause.** Every session shares one `main` checkout, by the owner rule in
`ai/META-DECISIONS.md` ("Sessions share the one `main` checkout"). That rule is
the cause of this family and this spec does not change it. What it changes is
that builds and gates read that live working tree, which at any moment holds
other sessions' half-written edits, and that nothing builds the tree a commit
actually records. Three faces of one mechanism:

1. A lab, a verify stage or a gate compiles or judges the live tree, so its
   verdict includes files no one asked it to judge.
2. `./le commit create` never compiles the tree it commits, so a consumer lands
   without its producer and `main` stops compiling for hours.
3. Artifacts built from the tree (`bin/le`, `bin/ze`, kernel and image caches,
   the verify failure index) are reused without proof they match the inputs
   that made them, because keys and staleness checks are hand-listed or
   mtime-based.

**Why it matters.** 61 open journal rows. Each red was attributed to "another
session" and left, because the gate could not say which tree it judged. A
broken `main` blocks every session at once.

**The fix in one sentence.** One snapshot builder turns HEAD plus named paths
into an immutable tree; every build and gate that judges code takes that tree
(never the checkout root), `./le commit create` compiles the tree it commits,
and every reused artifact is keyed by the digest of the inputs the snapshot
gave it.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` - the private index and blob snapshot of a prepared commit
  -> Constraint: since `a9f2207a38` the script seeds a private index from the parent and adds the blobs `snapshotIndexEntries` took at prepare time. That is the snapshot primitive this spec reuses; it already fixed the rows listed under "Rows found already fixed".
- [ ] `docs/architecture/testing/tracked-build-gate.md` - `repo compiles/check`, its flavors
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - certificate tree hash, `job.SnapshotTree`
- [ ] `docs/contributing/running-commands.md` - `bin/le`, `--name`, `--update`
- [ ] `plan/post-release/engineering/spec-le-built-from-head.md` - the le slice of this spec, approved by the owner on 2026-10-25 and superseding spec-le-shared-build-launcher: le builds from a HEAD export, keyed by the git tree hash of its inputs, smoke-tested, kept in `bin/builds/<time>-<sha>/le`, dispatched by `bin/le` and `./le`
  -> Decision: that spec's HEAD export is the first form of this spec's snapshot builder, not a second builder (`ai/rules/no-layering.md`); this spec generalises it to the other artifacts and gates and reuses its input-key rule for `bin/le` rather than defining another.
- [ ] `plan/post-release/engineering/spec-le-build-cache-cap.md` - one Go build cache, capped
  -> Constraint: snapshots share that cache; a snapshot never gets its own.
- [ ] `docs/architecture/testing/verify-freshness-scope.md`, "The green record and the cut" and "Verification debt, derived" - debt and certificate model (landed by the closed `spec-verification-debt-derived`)

## Current Behavior (MANDATORY)

Verified at the producer on 2026-10-23.

- [ ] `internal/le/interoplab/zebuild.go` - `stageBinary` runs `go build` with `Directory: root`, the live checkout; `StageBinaries` is called from `internal/le/verify/evidence/evidence.go` and `radiussubscriber.go` with that root.
- [ ] `internal/le/commit/verification.go` - `structuralGateReds` reads `tmp/ze-verify-failures.json` with no check of which tree or commit it describes, and returns no reds when the file is missing or unparsable, so a missing index reads as clean.
- [ ] `internal/le/commit/prepare.go` - `checkSourceGates` runs the weakened, RFC and closure gates by reading the working tree; no step compiles anything. The compile verdict reaches commit create only through the failure index above.
- [ ] `internal/le/commit/snapshot.go` - `snapshotIndexEntries` hashes the named paths' blobs at prepare time into a temp directory.
- [ ] `internal/le/verify/dispatch/dispatch.go` - `stageExecutable` answers `/proc/self/exe` or `os.Executable()`: every verify stage runs the binary that launched the run, built from the runner's tree, not from the commit under test.
- [ ] `le` (root launcher) - `warn_when_stale` compares mtimes (`le_is_stale`, `committed_change_is_newer`), warns one call in sixteen, and never refuses.
- [ ] `.github/workflows/qemu-nightly.yml` - the runtime-kernel cache key hashes `internal/appliance/kernel.version`, `gokrazy/kernel/**`, `tools/kernel-builder/**`, and omits `internal/appliance/kernelbuilder/**`, which `kernelCacheVariantFor` (`internal/appliance/cache.go`) hashes.
- [ ] `internal/le/verify/lifecycle.go` - `git worktree add --detach <sha>`: the existing materialiser of a commit into a throwaway tree.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`./le commit create`, `./le verify current` and `worktree`, `./le commit debt-clear`, `./le rfc discriminate-record`, `./le test qemu docker-lab`, `./le test integration interop*`, and the `./le` launcher.

### Transformation Path
1. The caller names the paths it owns (the same `file` vocabulary as `./le commit create`), or none for HEAD alone.
2. The snapshot builder seeds a private index from HEAD, adds those paths' current blobs, writes a tree, and wraps it in an unreferenced commit object. The result is a typed snapshot carrying the tree id.
3. The snapshot is materialised once per tree id under the shared scratch (worktree or archive), with the one shared Go build cache.
4. Builds and gates run inside it. Their artifacts are keyed by the tree id, or by the digest of the declared inputs the builder read from it.
5. `./le commit create` builds the snapshot of exactly what it will commit and runs the compile check on it before writing the script.

### Boundaries Crossed
le to git (private index, object store, never the shared index or HEAD); le to the Go toolchain; le to Docker and QEMU (lab images); GitHub Actions cache keys.

### Integration Points
`snapshotIndexEntries` and the private-index script (`internal/le/commit`); `repo compiles` (`internal/le/repo/compiles`); the verify worktree lifecycle; the kernel builder's input set.

### Architectural Verification
- [ ] No build or gate that judges code receives the checkout root as its source
- [ ] Every artifact reuse compares a digest of its inputs, never an mtime
- [ ] Every shared mutable file is written under a lock with an atomic rename

## Risks & Assumptions

### Assumptions
| Id | Assumption | Evidence | If wrong |
|----|------------|----------|----------|
| A-1 | a private index plus `write-tree` never touches the shared index or HEAD | the commit script already does it on every commit | none known |
| A-2 | compiling the packages a commit reaches is fast enough for commit create | `repo compiles` is already a verify stage | compile the changed packages and their importers only, per flavor, and measure; never drop the check |
| A-3 | the owner accepts `./le` refusing a stale `bin/le` | not decided; the launcher refuses nothing today by design | keep the warning, but every gate that judges a commit still runs that commit's le (AC-4 holds either way) |

### Risks
- A session that forgets to name a path builds without it. The snapshot report lists the dirty paths it left out, so the omission is visible.

## Blast Radius
`internal/le/commit`, `internal/le/verify`, `internal/le/interoplab`, `internal/le/test/qemu`, `internal/le/test/deployment`, `internal/le/rfc` (discrimination), `internal/le/job`, `internal/le/hookruntime` (derived rebuild), `internal/appliance` (kernel cache key), the `le` launcher, `.github/workflows/qemu-nightly.yml`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `./le commit create ... file <consumer>` whose producer is uncommitted | -> | snapshot compile check | refused, naming the undefined symbol and the file that would declare it |
| `./le test qemu docker-lab` with a peer's broken untracked `.go` file in the tree | -> | snapshot builder | the lab builds and runs; the peer file is not in the build |
| `./le verify worktree <sha>` | -> | stage binary from the commit | the stages run the le built inside the worktree |

## Acceptance Criteria

| AC | Condition | Observable |
|----|-----------|------------|
| AC-1 | a build or gate that judges code (labs, verify, discrimination, lint planning, functional runs that build) | runs on a snapshot; an unrelated dirty or untracked file in the checkout cannot change its verdict |
| AC-2 | `./le commit create` with a consumer whose producer is not in the commit | refuses before writing the script, naming the missing symbol |
| AC-3 | the commit create gates (weakened, RFC, closure) | judge the same blobs the script commits, not a second read of the working tree |
| AC-4 | `./le verify worktree`, `debt-clear`, `discriminate-record` | run the le built from the commit or snapshot they judge |
| AC-5 | `./le <action>` with `bin/le` built from sources whose committed digest differs from HEAD's | refuses and names `./le --update` (subject to A-3); never a silent run of the old code |
| AC-6 | the verify failure index | records the tree id it judged; `structuralGateReds` refuses a missing, unparsable or mismatched index instead of reading it as clean |
| AC-7 | the runtime-kernel cache key, the golangci-lint cache, the terminal-demo renderer image tag, job admission's share key | each is derived from the builder's declared input set (`kernelbuilder` files, ruleguard rules, the Dockerfile, every parameter); the nightly workflow takes its key from `./le`, not a hand list |
| AC-8 | verify artifact paths, the hook's derived-artifact rebuild | write under a lock with an atomic rename; two concurrent writers never lose a row or a file |
| AC-9 | discrimination breaks the producer | in a snapshot; the shared index and working tree are untouched |
| AC-10 | every row in "Owned journal rows" | re-checked after the change: fixed, or still red with a cause that is not the live tree, an unbuilt commit, or an unkeyed artifact |
| AC-11 | the post-commit shared-index sync meets `.git/index.lock` that no live process holds (an empty file left by a killed git, seen twice on 2026-10-23) | the sync tells a dead lock from a held one, says which it found, and either repairs the index or prints the exact repair; a held lock is retried with bounded backoff, as the 2026-10-23 concurrent-session-corruption row asks |

AC-8 holds no weakening ledger: `spec-weakening-approval-lives-in-the-commit` removes the tracked ledger and its unlocked rewrite (`PruneLanded`), and closes the 2026-10-14 concurrent-session-corruption row that named it. AC-8 does cover the two owner approval files that replaced it, `tmp/commit-weakening-approved-<ns>.md` and `tmp/commit-rfc-approved-<ns>.md`: their writer `rfc.ApproveInto` and the commit script's prune `renderApprovalPrune` share one lock (the 2026-10-25 row).

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Proves |
|------|------|--------|
| `TestSnapshotExcludesUnnamedDirtyFiles` | `internal/le/snapshot/snapshot_test.go` | AC-1 |
| `TestSnapshotNeverTouchesTheSharedIndex` | `internal/le/snapshot/snapshot_test.go` | AC-1, AC-9 |
| `TestCommitCreateRefusesAConsumerWithoutItsProducer` | `internal/le/commit/prepare_test.go` | AC-2 |
| `TestGatesJudgeTheSnapshotBlobs` | `internal/le/commit/prepare_test.go` | AC-3 |
| `TestStageExecutableIsTheCommitsLe` | `internal/le/verify/dispatch/dispatch_test.go` | AC-4 |
| `TestStructuralRedsRefusesAStaleIndex` | `internal/le/commit/verification_test.go` | AC-6 |

### Functional Tests
| Test | Proves |
|------|--------|
| `test/ui/le-commit-refuses-unbuilt-tree.ci` | AC-2 end to end |

## Files to Modify
- `internal/le/commit/prepare.go`, `verification.go`, `snapshot.go`
- `internal/le/verify/dispatch/dispatch.go`, `internal/le/verify/lifecycle.go`
- `internal/le/interoplab/zebuild.go`, `internal/le/verify/evidence/evidence.go`
- `internal/le/hookruntime/bash.go` (derived rebuild)
- `internal/appliance/cache.go`, `.github/workflows/qemu-nightly.yml`
- `le` (launcher)

## Files to Create
- `internal/le/snapshot/snapshot.go`, `snapshot_test.go`

### Integration Checklist
- [ ] Build helpers take a snapshot value, not a root string, so a build over the live tree does not compile
- [ ] One shared Go build cache for every snapshot

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/contributing/committing.md` - commit create compiles what it commits
- [ ] `docs/architecture/testing/tracked-build-gate.md` - the check moves before the commit
- [ ] `docs/contributing/running-commands.md` - which le a gate runs; staleness by digest
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - failure index carries its tree id
- [ ] `docs/architecture/appliance/build-artifacts.md` - kernel cache key derived from the builder's input set
- [ ] `docs/architecture/testing/interop.md` - labs build from a snapshot
- [ ] `docs/architecture/testing/test-health.md` - a red on a snapshot is attributable to that tree
- [ ] `docs/architecture/core-design.md` - read; edited only where it describes `le` or gates reading the checkout
- [ ] `docs/features/ai-first.md` - read; edited where it describes how sessions share builds

## Implementation Steps
1. Extract the private-index snapshot from `internal/le/commit` into `internal/le/snapshot`; give it a typed result.
2. Make the build helpers take the snapshot; move labs, verify, discrimination and lint planning onto it.
3. Add the compile check over the commit's snapshot to `./le commit create`; make the source gates read the snapshot blobs.
4. Run verify stages with the commit's own le; stamp and check the failure index's tree id.
5. Derive cache keys from declared inputs; add locks and atomic writes to the shared files.
6. Change the launcher's staleness to a source digest (after A-3 is decided).
7. Re-check every owned row.

### Failure Routing
| Failure | Route |
|---------|-------|
| the compile check is too slow for commit create | narrow to the reached packages, never remove it |

## Design Insights
- The private index already proved the idea for commits: once the script committed only the blobs it named, five rows stopped. The same primitive, given to builds and gates, removes the rest.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| HEAD plus named paths, as a git tree | copy the working tree; a worktree per session | a tree id is immutable and cheap; the owner rule forbids a worktree per session for development, and a snapshot is a build input, not a place to edit |
| compile in commit create | rely on verify afterwards | the rows show `main` broken for hours between the commit and the next verify |
| keys from declared inputs | hand-listed key paths | the nightly key drifted from `kernelCacheVariantFor` exactly that way |

## Known Limitations
- The six open `key-omits-a-fact-the-builder-uses` rows, which the merged analysis assigned here as cache keys, are product defects (OSPF NSSA, GR helper, RR withdrawal, relay body cache, CLI diff, BFD session lookup); none is a build cache, so none is owned here.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| concurrent-session-corruption.md | 2026-10-25 | owner approval files: `rfc.ApproveInto` against the commit script's `renderApprovalPrune` |
| concurrent-session-corruption.md | 2026-10-24 | `git diff` callers under a timeout outside `gitenv.ReadOnly` |
| concurrent-session-corruption.md | 2026-10-23 | Generated post-commit shared-index sync, handover 20 |
| concurrent-session-corruption.md | 2026-10-23 | ./le test qemu docker-lab, building the docker-host image over the live che |
| concurrent-session-corruption.md | 2026-10-21 | stageBinaries (internal/le/interoplab/zebuild.go) in the docker-lab guest, |
| concurrent-session-corruption.md | 2026-10-18 | ./le commit create gates, closureStem (internal/le/commit/prepare.go) |
| concurrent-session-corruption.md | 2026-10-05 | internal/le/go/lint/verifylint.go, Runner.plan |
| concurrent-session-corruption.md | 2026-09-06 | main at HEAD, over internal/component/ike/engine/fsm.go |
| concurrent-session-corruption.md | 2026-09-04 | ./le functional plugin over the shared checkout, and the three producers a |
| concurrent-session-corruption.md | 2026-09-04 | d0affb5e4, docs/plugin-development/metrics.md |
| concurrent-session-corruption.md | 2026-09-04 | main at HEAD, over internal/component/plugin/process/process.go and interna |
| concurrent-session-corruption.md | 2026-09-03 | ./le repository tracked-build check over HEAD, host and installer flavors |
| concurrent-session-corruption.md | 2026-09-03 | the generated commit script's concurrency guard (./le commit create, intern |
| concurrent-session-corruption.md | 2026-09-01 | internal/core/eap/eap.go, NewSession's default arm, during a hand-applied d |
| concurrent-session-corruption.md | 2026-08-31 | an incomplete signature change in internal/component/l2tp/ppp/lcp_options.g |
| concurrent-session-corruption.md | 2026-08-29 | internal/component/bgp/plugins/nlri/ls/types_descriptor.go, judged by ./le |
| concurrent-session-corruption.md | 2026-08-15 | shared checkout, uncommitted behaviour change |
| concurrent-session-corruption.md | 2026-08-17 | shared spec corpus |
| concurrent-session-corruption.md | 2026-08-19 | parallel editing subagents, one working tree |
| concurrent-session-corruption.md | 2026-08-28 | internal/le/scratch/scratch.go, judged by ./le repository tracked-build che |
| concurrent-session-corruption.md | 2026-08-29 | internal/component/bgp/reactor, judged by building a detached worktree at H |
| concurrent-session-corruption.md | 2026-08-29 | internal/component/bgp/reactor, judged by a race run over the working tree |
| concurrent-session-corruption.md | 2026-08-29 | ./le verify deps alloc over internal/perf/allocgate.go ceilings, producer ( |
| concurrent-session-corruption.md | 2026-08-31 | closureStem (internal/le/commit/review.go) reading plan/journal/<class>. |
| concurrent-session-corruption.md | 2026-09-05 | check (internal/le/site/facts/staleness.go) against derive and warnUncommit |
| concurrent-session-corruption.md | 2026-09-06 | main at HEAD, over internal/le/interoplab/ipsec/checkers.go and test/intero |
| concurrent-session-corruption.md | 2026-09-20 | ./le rfc discriminate-record ... route revert, and the SHARED git index |
| concurrent-session-corruption.md | 2026-10-11 | ./le test qemu docker-lab and the interop-ipsec ze cross-compile, both buil |
| committed-tree-does-not-build.md | 2026-09-23 | Frozen RSVP tests, native staticcheck matrix part 2/6 |
| stale-artifact-reused.md | 2026-10-17 | ./le test deployment docker-l2tp-ppp-test, shared bin/le |
| stale-artifact-reused.md | 2026-10-14 | RuntimeKernelCacheDir (internal/le/test/deployment/gokrazyimage.go), the bu |
| stale-artifact-reused.md | 2026-10-10 | kernelbuilder host-side files, the key's return guard |
| stale-artifact-reused.md | 2026-10-10 | The kernel cache key (kernelCacheVariantFor, internal/appliance/cache.go) a |
| stale-artifact-reused.md | 2026-10-10 | ensureFirmware (internal/appliance/cmd_kernel.go), i915 blobs for the hardw |
| stale-artifact-reused.md | 2026-10-10 | .github/workflows/qemu-nightly.yml kernel cache key |
| stale-artifact-reused.md | 2026-10-08 | stress-repro over bgp parse --draft, inner buildZe (internal/test/cli/cmd_b |
| stale-artifact-reused.md | 2026-10-03 | le_is_stale, committed_change_is_newer (le, the launcher) |
| stale-artifact-reused.md | 2026-10-02 | resolveKernelPackage / kernelPackageProblems, internal/le/test/deployment/ |
| stale-artifact-reused.md | 2026-09-19 | ./le verify worktree and ./le commit debt-clear (internal/le/verify/engine/ |
| stale-artifact-reused.md | 2026-09-13 | structuralGateReds (internal/le/commit/verification.go), gating ./le commit |
| stale-artifact-reused.md | 2026-09-13 | le_is_stale and committed_change_is_newer (le, the launcher) |
| stale-artifact-reused.md | 2026-09-12 | RunAction (internal/le/verify/dispatch/dispatch.go), under ./le commit debt |
| stale-artifact-reused.md | 2026-09-12 | nativeLEBinary (internal/test/fixture/misc_fixture_runner_scope.go), and ev |
| stale-artifact-reused.md | 2026-09-04 | ./le launcher reading bin/le, over ./le integration interop-radius |
| stale-artifact-reused.md | 2026-09-04 | ./le launcher reading bin/le, over ./le integration interop-ipsec |
| stale-artifact-reused.md | 2026-09-01 | ./le launcher, the bin/le existence cache |
| stale-artifact-reused.md | 2026-09-01 | ./le launcher, the bin/le existence cache |
| stale-artifact-reused.md | 2026-08-31 | le (the wrapper script at the repository root), reading bin/le |
| stale-artifact-reused.md | 2026-08-31 | requireRendererImage (internal/le/terminaldemo/render.go), image tag fr |
| stale-artifact-reused.md | 2026-09-02 | ./le launcher, the bin/le existence cache |
| stale-artifact-reused.md | 2026-08-12 | dev workflow |
| stale-artifact-reused.md | 2026-08-18 | verify freshness certificate |
| stale-artifact-reused.md | 2026-08-19 | job admission, parameterized make targets |
| stale-artifact-reused.md | 2026-08-19 | unit-test job admission |
| stale-artifact-reused.md | 2026-08-30 | ./le launcher, the bin/le existence cache |
| stale-artifact-reused.md | 2026-09-07 | golangci-lint result cache |
| stale-artifact-reused.md | 2026-09-08 | bin/ze, the canonical binary every functional suite and ./le stress-repro r |
| stale-artifact-reused.md | 2026-09-12 | ./ze --help, root ze launcher |
| check-mode-mutates-the-tree.md | 2026-09-21 | the pretool-bash derived-artifact rebuild (internal/le/hookruntime/bash.go, |
| gate-verdict-depends-on-the-machine.md | 2026-09-04 | ./le docvalid help-shape, and every ./le gate that reads an embedded YANG m |
| gate-verdict-depends-on-the-machine.md | 2026-08-31 | ./le verify worktree, and every gate stage it runs |
| gate-verdict-depends-on-the-machine.md | 2026-10-14 | ./le repo arch-map check, run by test/ui/le-dev-tools-answers.ci |
| assertion-reads-state-the-scheduler-owns.md | 2026-09-19 | TestConcurrentRunsDoNotShareArtifactPaths (internal/le/verify) against the |
| stale-artifact-reused.md | 2026-08-12 | ipsec interop PKI fixtures not regenerated |
| stale-artifact-reused.md | 2026-08-21 | site generate stamps demo definition hashes, ignoring the exit |
| stale-artifact-reused.md | 2026-08-31 | `renderEngine` and `(*demoCatalog).assetRoot` disagree on the demo media root |
| stale-artifact-reused.md | 2026-09-01 | `trackedAndUntrackedSourceFiles` and `assetRoot`, second occurrence |
| tree-state-claim-published-unverified.md | 2026-09-06 | carrying a peer's hunk out of a shared file |
| tree-state-claim-published-unverified.md | 2026-09-12 | no gate builds `cmd/ze` from HEAD content |
| committed-tree-does-not-build.md | 2026-10-05 | FreeBSD, DragonFly and WASI census: doctor storage, `pkg/zefs`, reactor connection |
| concurrent-rfc-gate-stale.md | 2026-08-15 | 15 next-hop fixtures red on another session's uncommitted `session_write.go` |
| concurrent-session-corruption.md | 2026-09-03 | `./le commit create`, the window between preparing a script and running it |
| concurrent-session-corruption.md | 2026-09-03 | doctor `codes.go` and `doctor.go` uncommitted hunks vanished under a peer |
| concurrent-session-corruption.md | 2026-09-04 | `mustNameImportingProtocol` panic justified against a peer's uncommitted callers |
| concurrent-session-corruption.md | 2026-09-04 | `367b0c359b` committed another session's `interop.md` and journal edits |
| concurrent-session-corruption.md | 2026-09-06 | `0e72b398f2` swept another agent's appliance isolcpus work |
| concurrent-session-corruption.md | 2026-09-06 | `0e72b398f2` swept `cmd/ze/hub/` edits calling an uncommitted function |
| concurrent-session-corruption.md | 2026-09-06 | `./le session scratch ensure` returns the parent's directory to every subagent |
| concurrent-session-corruption.md | 2026-10-14 | `./le rfc approve` writes under a different session than `./le commit session` |
| documentation-stranded-by-a-siblings-hunk.md | 2026-09-06 | `docs/functional-tests.md` `### ze-test dns` section held by a sibling hunk |
| documentation-stranded-by-a-siblings-hunk.md | 2026-09-06 | `command-reference.md` firewall domain-group section held by a sibling hunk |
| documentation-stranded-by-a-siblings-hunk.md | 2026-09-09 | `command-reference.md` `### show plugin declarations` beside an unsafe sibling hunk |
| gate-verdict-depends-on-the-machine.md | 2026-10-21 | `staticRecipe` takes compiler and musl from Alpine's live repository |
| gate-verdict-depends-on-the-machine.md | 2026-10-21 | stale patched-GOROOT cache key widens `le repo changed scope` to `./...` |

## Rows found already fixed

Another agent closes these in the journal; this spec does not edit `plan/journal/`.

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| committed-tree-does-not-build.md | 2026-09-11 | rib test binary, bestprev_shard_test.go calling peerRIB.Insert | HEAD calls `peerRIB.Insert(fam, attrs, prefix)`, matching `(*PeerRIB).Insert` |
| committed-tree-does-not-build.md | 2026-09-11 | reactor_notify.go calling updateDelayEndOfRIB | HEAD declares `(*Peer).updateDelayEndOfRIB` in `update_delay.go` |
| committed-tree-does-not-build.md | 2026-09-11 | strictPeersWithoutEngine reading ps.BFD.Strict | HEAD declares `Strict bool` in `peer_settings.go` |
| committed-tree-does-not-build.md | 2026-09-11 | checkDependencyDrift, go.mod against dependencies.json | HEAD `website/data/dependencies.json` carries `github.com/prometheus/common` |
| committed-tree-does-not-build.md | 2026-09-16 | config test binary, warningsNaming calling Snapshot | HEAD `loader_test.go` reads `entries, err := ...Snapshot(...)` |
| committed-tree-does-not-build.md | 2026-09-22 | PPP manager_test.go duplicate declarations | HEAD has no duplicate `reg :=` / `pair :=` in one scope |
| concurrent-session-corruption.md | 2026-09-04 | the commit script's concurrency guard (renderBlock) | replaced by a private index seeded from the parent (`a9f2207a38`) |
| concurrent-session-corruption.md | 2026-09-03 | the window between preparing and running a commit script | blobs are fixed at prepare by `snapshotIndexEntries` (`a9f2207a38`) |
| concurrent-session-corruption.md | 2026-09-04 | script.go renderBlock, the window inside the script | same private index (`a9f2207a38`) |
| concurrent-session-corruption.md | 2026-09-06 | 0e72b398f2 over internal/appliance kernelargs files | a commit now carries only its named paths (`a9f2207a38`) |
| concurrent-session-corruption.md | 2026-09-06 | 0e72b398f2 over cmd/ze/hub/ | same |
| concurrent-session-corruption.md | 2026-08-31 | test/weakened.md, the same ledger a third time | the ledger is now per-agent shards under `test/weakened/` |
| concurrent-session-corruption.md | 2026-08-31 | the same two ledgers, one session later | same; `test/rfc-changed.md` no longer exists |
| concurrent-session-corruption.md | 2026-08-31 | test/rfc-changed.md and test/weakened.md, replace-per-commit | same |
| concurrent-session-corruption.md | 2026-08-25 | test/weakened.md and test/rfc-changed.md, per-commit ledgers | same |

## Checklist

### Goal Gates (MUST pass)
- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after it
- [ ] `./le test unit` over `internal/le/snapshot`, `internal/le/commit`, `internal/le/verify/...`, `internal/le/test/weakened`, under `./le job run`
- [ ] `./le verify current` over the final commit, by the main thread or a fresh agent
