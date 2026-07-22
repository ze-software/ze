# Spec: gokrazy-builddir-tmp

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-22 |

> **The confirmed bug (A-1/A-1b) was fixed ahead of this spec on 2026-07-22**, out
> of a disk-usage investigation that found its fallout in `gokrazy/modcache`. The
> derived parent now carries a copied `builddir` with its filesystem-path replaces
> rewritten to absolute paths, and materializes under project `tmp/`. What that
> closes, and what is still this spec's work, is recorded under "Landed ahead of
> this spec" below. Read that section before planning implementation: three of the
> Acceptance Criteria are already satisfied and their evidence is recorded.

> **This spec was rewritten on 2026-07-22 after independent review.** The first
> draft proposed deleting the tracked ze builddir module and synthesizing it into
> `tmp/`. Three independent reviewers found that design unsound: it would have
> broken the documented offline build, and three of its mechanical premises about
> `gok` were misreadings. The killed premises are recorded in the Mistake Log
> below rather than deleted, because the reasoning that produced them is the most
> useful thing this spec has to hand to the next reader. The scope below is
> roughly a fifth of the original.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/appliance-dep-bumps.md`, `ai/rules/qemu-testing.md`
4. `mk/gokrazy.mk`, `internal/appliance/kernelargs.go`, `internal/appliance/cmd_build.go`, `vendor/github.com/gokrazy/tools/packer/gotool.go`, `vendor/github.com/gokrazy/tools/internal/gok/overwrite.go`

## Task

Make every appliance image build run from a **prepared instance under the project
`tmp/`**, so no build step writes to a tracked path, and collapse the two
implementations of "prepare an instance elsewhere" into one.

**The one live problem.** `make ze-kernel` writes an out-of-tree kernel `replace`
into a **tracked** file: `mk/gokrazy.mk:257` runs `go mod edit -replace` against
`gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod`, and the operator must run
`ze-kernel-clean` (`mk/gokrazy.mk:262`) to revert it. In a checkout shared by
concurrent sessions, a build step that edits a tracked file is a cross-commit
hazard of the same class as a shared plan log. The relative replace path and its
warning about leaking a home directory into a tracked `go.mod`
(`mk/gokrazy.mk:184-187`) is a workaround that exists only because the target is
tracked.

**A second, smaller problem.** "Prepare the instance somewhere else" is
implemented twice and neither knows about the other: `materializeDerivedParent`
(`internal/appliance/kernelargs.go:107-155`), used only when an image config
requests hugepages, and `prepare_instance`
(`scripts/evidence/effective-gokrazy-l2tp-ppp.py:609-649`), used by the L2TP QEMU
evidence. The first writes to the **system** temp dir, which this project's own
rules ban.

**A confirmed bug this work fixes.** `materializeDerivedParent` excludes
`builddir` from what it materializes (`kernelargs.go:136`), leaving gok a
builddir-less instance, so **every pin is discarded** and gok falls back to
`go get`. This is not a hypothesis: an A/B build on 2026-07-22 settled it (A-1).
With `GOPROXY=off`, a plain `ze appliance build` succeeds and the same build with
`image.hugepages` set fails at `go get github.com/rtr7/kernel: module lookup
disabled by GOPROXY=off`, running out of `/tmp/ze-appliance-build-<n>/ze/`.

Two consequences, and the second is the serious one:

| Condition | Effect |
|-----------|--------|
| Offline | The hugepage build fails outright, breaking the documented offline guarantee |
| Online | The build silently succeeds against **unpinned** versions of `github.com/rtr7/kernel` and the gokrazy init, resolved at whatever `go get` returns, rather than the versions `gokrazy/ze/builddir/*/go.mod` deliberately pin |

The silent case means an appliance image whose *kernel* is not the pinned one,
produced by the exact feature (`image.hugepages`, used by VPP host tuning) that
an operator would reach for in production. The fix is to stop excluding
`builddir`, which is the core of this spec.

## Landed ahead of this spec (2026-07-22)

The fallout of A-1 was found from the other end: `gokrazy/modcache` had grown to
2.1 GB, of which 1.23 GB was **nine extracted copies of ze itself** at nine
consecutive pseudo-versions (one per pushed commit, 18-22 July), fetched from the
proxy despite the tracked builddir replacing ze with the working tree. The same
builds also pulled `github.com/rtr7/kernel` at **two versions newer than the
pin**, which settles A-1b: the silent-online case substitutes the appliance's
kernel.

| Landed | Where | Evidence |
|--------|-------|----------|
| `materializeDerivedParent` copies `builddir` instead of excluding it | `internal/appliance/kernelargs.go` | `TestMaterializeDerivedParent`, mutation-verified (re-adding the exclusion turns it red) |
| Filesystem-path replaces rewritten to absolute paths on copy | `absolutizeReplaces` in the same file | same test asserts the rewritten target equals the repo root; mutation-verified |
| Empty builddir is a loud error, not a silent network build (AC-6, R-1) | `copyBuildDir` | `TestCopyBuildDirFailsClosedWithoutModules` |
| The derived parent materializes under project `tmp/`, not the system temp dir | same file | `TestMaterializeDerivedParent` asserts the prefix |
| Distinct dir per build (AC-11) | same file | `TestMaterializeDerivedParentIsolatesConcurrentBuilds` |
| A hugepage build resolves offline (AC-10) | end-to-end | `ze appliance build` with `image.hugepages` and `GOPROXY=off` exits 0, no `go get` in the log, `gokrazy/modcache` unchanged at 811 MB, only the pinned kernel present. Log: `tmp/hp-proof-build-offline.log` (ephemeral; the outcome is quoted here because `tmp/` does not survive). This is the exact A/B that failed on 2026-07-22 |
| The kernel args still reach the image (AC-9) | end-to-end | `default_hugepagesz=2M hugepagesz=2M hugepages=512` present in the built `.img` cmdline |

**Not proven:** the QEMU boot. The host user is not in the `kvm` group, so
`effective-vpp-hugepages-qemu.py` would run under tcg and time out exactly as it
did earlier the same day. The image builds and carries the right cmdline; it was
not booted. R-2 stands.

**Still this spec's work:** AC-1 (prepare for *every* build, not only hugepage
builds), AC-2 (the tree-clean functional test), AC-7/AC-8 (kernel package as an
explicit parameter so `make ze-kernel` stops editing a tracked `go.mod`), D-1
(how `make ze-gokrazy` reaches the preparer), deleting the third preparer in
`scripts/evidence/effective-gokrazy-l2tp-ppp.py`, and correcting the false boot
proof cited in `ai/rules/appliance-dep-bumps.md`.

**Explicit non-goals.**
- Dropping `vendor/` or collapsing the two committed dependency stores. That is
  `plan/spec-unify-dep-stores.md` (deferred, blocked on
  `spec-relocate-scratch-and-cache`).
- Removing any tracked builddir `go.mod` or `go.sum`. All eight modules and their
  seven tracked sums stay exactly as they are. See the Mistake Log for why the
  first draft's attempt to remove one was wrong.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `ai/rules/appliance-dep-bumps.md` - the runbook whose mechanics this touches
  → Constraint: step 4 (`:40-48`) deletes and regenerates the builddir sums via `make ze-gokrazy-deps`; that loop is what populates the module cache, so the modules it iterates must keep existing
  → Constraint: step 7 (`:55-59`) makes a real image build **plus** a QEMU boot proof blocking, and says the build alone is insufficient
  → Decision: `:57` of that step names `test/appliance/serial-login.ci` as the boot proof. That test boots nothing (see Current Behavior). The rule is wrong and this spec corrects it
- [ ] `ai/rules/qemu-testing.md` - what proves an appliance change works
  → Constraint: runtime behaviour is proven in the QEMU harness, never inferred from a successful compile
- [ ] `ai/rules/testing.md` "Temporary Files" - project `tmp/`, never `/tmp`
  → Constraint: the code this spec extends currently violates this (`kernelargs.go:118`)
- [ ] `mk/gokrazy.mk` header (lines 1-32) - the documented build contract
  → Constraint: `make ze-gokrazy-deps` is a one-time post-clone step and builds are expected to work offline afterwards

### RFC Summaries (MUST for protocol work)
Not applicable. This spec changes build orchestration and touches no wire protocol.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- gok chdirs into the instance directory before packing, so a relocated instance genuinely relocates the builddir. Everything gok reads by relative path is read from the instance dir, not the repository root.
- The tracked builddir modules are load-bearing for two unrelated reasons: the ze module carries a self-replace gok never synthesizes, and it is also the input that makes `ze-gokrazy-deps` populate the cache with ze's ~94 dependencies.
- Only one of the eight modules has a filesystem-path replace. The design does not need a general path-rewriting step.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `vendor/github.com/gokrazy/tools/internal/gok/overwrite.go` - the packing entry point
  → Constraint: `:143` runs `os.Chdir(r.inst.InstancePath())` before `pack.Main`. This is why `--parent_dir` relocates the builddir, and why every relative path gok reads afterwards is instance-relative
- [ ] `vendor/github.com/gokrazy/tools/packer/gotool.go` - builddir resolution and bootstrap
  → Constraint: `BuildDir` (`:86-107`) joins the relative string `builddir/<importPath>` and walks from most specific to least looking for a `go.mod`
  → Constraint: `BuildDirOrMigrate` (`:109-190`) creates a module only when none exists; the whole bootstrap, including the `go.sum` write at `:181-187`, is inside the `os.IsNotExist(goMod)` guard opened at `:119` and closed at `:188`
  → Constraint: inside that guard, `:120` reads `go.mod` relative to the CWD, which is the instance dir. `gokrazy/ze/` has no `go.mod`, so gok writes a bare module named `gokrazy/build/` plus the instance name (`:138`, `:148`) with no requires, and an empty `go.sum`
  → Constraint: nothing in that path adds a require or a working-tree replace for the ze package. The tracked ze module is hand-written and gok cannot reproduce it
- [ ] `gokrazy/ze/builddir/*/go.mod` - the eight modules, read directly
  → Constraint: only the ze module has a filesystem-path replace (`:7`, `=> ../../../../../../`). `serial-busybox:11` and `rtr7/kernel:11` carry module-to-module **version** replaces, which are depth-independent; the other five have none
  → Constraint: `gokrazy/go.mod` and `rtr7/kernel/go.mod` both declare `module gokrazy/build/ze`, confirming gok names a generated module from the instance, not the checkout directory
- [ ] `mk/gokrazy.mk` - the make build path
  → Constraint: `:63-66` populates the module cache by iterating `find gokrazy/ze/builddir -name go.mod` and running `go mod download all` in each. The ze module is what pulls ze's own dependency graph into the cache
  → Constraint: `:117` invokes `bin/gok --parent_dir gokrazy -i ze overwrite`, then `:121-126` do mkfs and credential injection in shell
  → Constraint: `:179` resolves the pinned kernel version by changing into the tracked kernel builddir; `:236` stages `vmlinuz` **before** `:257` writes the replace, so the replace is not on the path any `tmp/kernel/vmlinuz` consumer exercises
  → Constraint: `:257` edits a tracked `go.mod`; `:262` is the target that reverts it
- [ ] `internal/appliance/kernelargs.go` - the existing preparer
  → Constraint: `resolveBuildParentDir` (`:81-99`) returns the checked-in `gokrazy` dir unless the config requests hugepages
  → Constraint: `materializeDerivedParent` (`:107-155`) symlinks every instance entry except `config.json` and `builddir` (`:136`)
  → Constraint: `:118` calls `os.MkdirTemp("", "ze-appliance-build-")`. The empty first argument means the **system** temp dir, not project `tmp/`
- [ ] `internal/appliance/cmd_build.go` - the Go image build
  → Constraint: this is already a full image build, not just a gok wrapper: `:1` says "assemble + gok + ext4", `:56-59` require mkfs.ext4 and debugfs, `:144` assembles the ZeFS, `:294-301` runs gok. `make ze-gokrazy` duplicates this flow in shell
  → Constraint: `:269` calls `resolveBuildParentDir` and `:294-301` passes the result as `--parent_dir`; `:246` resolves the modcache independently of the instance location
- [ ] `scripts/evidence/effective-gokrazy-l2tp-ppp.py` - the working precedent
  → Constraint: `prepare_instance` (`:609-649`) copies the **whole** builddir including every `go.sum` (`:615`, `symlinks=True`), then rewrites the ze replace (`:626-633`) and the kernel replace (`:639-647`) to absolute paths. It synthesizes nothing and deletes nothing
- [ ] `test/appliance/serial-login.ci` - cited by the dep-bump rule as the boot proof
  → Constraint: it boots nothing. Its own header says the QEMU plan applies "when appliance serial test infrastructure is ready"; what it asserts is the argv[0] shell-invocation gate offline
- [ ] `scripts/evidence/effective-vpp-hugepages-qemu.py` - the real appliance boot proof
  → Constraint: `:148` sets `image["hugepages"]`, which is exactly the condition that triggers the derived-parent path; `:158` runs a real `ze appliance build`; `:173-198` boots it in QEMU; `:201` asserts `/proc/cmdline`. This test already exercises the code path A-1 suspects
  → Constraint: it has a self-skip contract, and the `.ci` accepts `PASS|SKIP`, so a skipped run proves nothing

**Behavior to preserve:** (unless user explicitly said to change)
- All eight builddir `go.mod` files and all seven tracked `go.sum` files stay tracked and unchanged in content.
- `make ze-gokrazy-deps` keeps iterating all eight modules and keeps populating `gokrazy/modcache`, so builds stay offline-capable afterwards (`mk/gokrazy.mk:8-9`).
- `GOMODCACHE` keeps resolving to the checked-in `gokrazy/modcache`, with `-modcacherw` preserved.
- The image's resolved module versions are identical before and after this change, module by module.
- `make ze-kernel` keeps producing an image that boots the out-of-tree kernel; `make ze-kernel-clean` keeps returning to a pinned-kernel build.
- Hugepage kernel arguments still reach the instance `config.json`.

**Behavior to change:** (only if user explicitly requested)
- Every build prepares an instance under project `tmp/`, not only hugepage builds, and not in the system temp dir.
- The prepared instance contains a full `builddir`, copied rather than excluded.
- The out-of-tree kernel package becomes an explicit build parameter. `make ze-kernel` writes no state anywhere, and the replace is injected into the prepared copy at build time.
- No build step writes to a tracked path. (Scoped honestly: `make ze-gokrazy-deps` is a maintenance step, not a build step, and it keeps regenerating the seven tracked sums in place by design. See Known Limitations.)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Three callers request an image: `make ze-gokrazy` (`mk/gokrazy.mk:73`), `ze appliance build` (`internal/appliance/cmd_build.go:269`), and the L2TP evidence script (`scripts/evidence/effective-gokrazy-l2tp-ppp.py:609`).
- Inputs: an instance identity (parent dir plus name), an optional hugepage kernel-argument patch, and an optional out-of-tree kernel package path.

### Transformation Path
1. **Read the tracked instance**: `gokrazy/ze/config.json` and the whole `gokrazy/ze/builddir` tree, `go.sum` files included.
2. **Materialize a prepared instance** under project `tmp/`, in a unique directory per build, with a cleanup func.
3. **Copy the builddir verbatim**, exactly as the evidence script does at `:615`. Nothing is synthesized and nothing is omitted.
4. **Rewrite the ze self-replace to an absolute path** resolved through the prepared parent's real path. This is the only filesystem-path replace among the eight modules, so no general rewriting pass is needed.
5. **Inject the kernel replace when a kernel package was passed**, into the prepared copy only.
6. **Apply the config patch** when hugepage arguments were requested, preserving today's `deriveInstanceConfigJSON` behaviour.
7. **Point gok at the prepared parent**, with `GOMODCACHE` still resolving to `gokrazy/modcache`.
8. **Build, then clean up.** Nothing is copied back.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Tracked instance ↔ prepared instance | one-directional copy plus two targeted replace rewrites; never written back | [ ] |
| Preparer ↔ gok | `--parent_dir` argument plus `GOMODCACHE` env, unchanged in form | [ ] |
| Make ↔ Go preparer | see the open decision D-1 below; either make delegates to `ze appliance build`, or a new subcommand prepares and prints a path | [ ] |
| Python evidence script ↔ preparer | the script drops its private `prepare_instance` and uses the shared one | [ ] |
| Kernel package ↔ prepared instance | absolute `replace` written into the prepared copy only, from an explicit parameter | [ ] |

### Integration Points
- `resolveBuildParentDir` (`internal/appliance/kernelargs.go:81`) becomes the single unconditional entry to the preparer.
- `materializeDerivedParent` (`:107`) is extended: it stops excluding `builddir` (`:136`) and stops using the system temp dir (`:118`).
- `mk/gokrazy.mk:117` and `:257` stop addressing tracked paths.
- `prepare_instance` (`effective-gokrazy-l2tp-ppp.py:609`) is deleted.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Open Decision (blocks approval)

**D-1: how does `make ze-gokrazy` reach the preparer?** `make ze-gokrazy`
(`mk/gokrazy.mk:73-130`) does not go through `ze appliance build`; it calls gok
directly at `:117` and then does mkfs and credential injection in shell at
`:121-126`. But `internal/appliance/cmd_build.go` already implements that whole
flow in Go (`:1`, `:56-59`, `:144`, `:294-301`). Two options:

| Option | What it means | Cost | Risk |
|--------|---------------|------|------|
| D-1a: make delegates to `ze appliance build` | One build path. The preparer question disappears, because the only caller already owns `resolveBuildParentDir` | Larger: the make path carries `GOKRAZY_ARCH` cross-compilation, the config template, cert caching and external-ZeFS handling that must be proven equivalent first | Higher up front, lower forever |
| D-1b: add a subcommand that prepares an instance and prints its path | make calls it, uses the path for gok, and removes it afterwards | Smaller | Keeps two build implementations and adds a CLI surface whose only consumer is a Makefile |

Recommendation: **D-1a**, gated on an explicit equivalence audit of the two paths
as the first implementation phase. If the audit finds a genuine gap, fall back to
D-1b. This decision must be made before implementation starts; it changes the
Files to Modify list and the CLI integration answer below.

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The hugepage build path is broken: `materializeDerivedParent` gives gok a builddir-less instance, so every pin is discarded and gok falls back to `go get` | `internal/appliance/kernelargs.go:136` read against `vendor/github.com/gokrazy/tools/packer/gotool.go:119-190`, then executed | n/a, confirmed | A/B build 2026-07-22 with `GOPROXY=off`: plain `ze appliance build` exits 0; the same build with `image.hugepages` exits 1 at `go get github.com/rtr7/kernel: module lookup disabled by GOPROXY=off`, running from `/tmp/ze-appliance-build-<n>/ze/`. Logs: `tmp/a1-plain.log`, `tmp/a1-hugepages.log` | **confirmed** |
| A-1b | The failing module is the pinned kernel, not the ze package as first hypothesized, so the silent-online case substitutes the appliance's kernel | The A/B failure names `github.com/rtr7/kernel` while `codeberg.org/thomas-mangin/ze/cmd/ze` is listed as building normally | The blast radius is different from the one recorded and R-1's wording needs revising | `gokrazy/modcache` held `rtr7/kernel@v0.0.0-20260705070647-eeea0c47d01b` (fetched 2026-07-18 15:02) and `@v0.0.0-20260719062436-25aab92d2b39` (fetched 2026-07-20 11:16), both NEWER than the only version this repo ever pinned (`20260403073601-5a996da3a37b`, added in 86960d858 and never changed). Each fetch is one to two minutes before an extracted ze self-copy of the same build. Online derived builds shipped an unpinned kernel; it is BOTH modules, not one | **confirmed** |
| A-2 | Copying the builddir verbatim into a prepared instance and rewriting only the ze self-replace produces an identical image | `scripts/evidence/effective-gokrazy-l2tp-ppp.py:609-649` does exactly this today and the L2TP appliance evidence is built on it | The copy approach is not transparent and each difference needs investigation | **partly confirmed** 2026-07-22: a derived build with `GOPROXY=off` completes and produces a bootable-shaped image carrying the pinned kernel and the derived cmdline. Image-to-image equality against a non-derived build was NOT compared, and the image was not booted | partly confirmed |
| A-3 | Absolute replace paths resolve identically to today's relative one | Go resolves a relative `replace` against the `go.mod`'s own directory; the evidence script already rewrites to absolute at `:626-633` and `:639-647` | The rewrite changes resolution and the prepared depth must match the tracked depth instead | **confirmed** 2026-07-22: with ze's proxy copies deleted from `gokrazy/modcache` and `GOPROXY=off`, the derived build still resolved ze, which is only possible through the rewritten absolute replace pointing at the working tree | **confirmed** |
| A-4 | `make ze-gokrazy-deps` keeps populating the cache unchanged, because no builddir module is removed | `mk/gokrazy.mk:63-66` iterates `find ... -name go.mod`; this session observed that running it is what placed `grpc@v1.82.1` into `gokrazy/modcache` | The offline guarantee regresses | Run `make ze-gokrazy-deps` on a tree with a cleared cache entry and confirm repopulation | unvalidated |
| A-5 | A prepared build completes with no network | `cmd/ze-gok/main.go:34-37` and `cmd_build.go:246` both point `GOMODCACHE` at the checked-in cache, and the copied `go.sum` files come with the modules | The documented offline guarantee (`mk/gokrazy.mk:8-9`) regresses | **confirmed** 2026-07-22: `ze appliance build` on a hugepage appliance with `GOPROXY=off` exits 0; `gokrazy/modcache` is byte-stable across the run and the log contains no `go get`. Log: `tmp/hp-proof-build-offline.log` (ephemeral; the outcome is quoted here because `tmp/` does not survive) | **confirmed** |
| A-6 | Nothing outside the build reads the tracked builddir `go.sum` files | grep over `mk/`, `Makefile`, `scripts/`, `.github/` finds no consumer; `mk/gokrazy.mk:63` matches `-name go.mod` only | A consumer breaks | Repeat the grep at implementation time | unvalidated |
| A-7 | `ze appliance build` is functionally equivalent to `make ze-gokrazy` (needed only if D-1a is chosen) | `cmd_build.go:1`, `:56-59`, `:144`, `:294-301` implement assemble plus gok plus ext4, the same steps `mk/gokrazy.mk:73-130` performs | D-1a is not viable and D-1b is required | The D-1 equivalence audit in Phase 1 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A module missing from the prepared instance makes gok synthesize an empty module and fetch from the network, silently building a different source than the working tree | An image that builds only with network access, or a resolved version differing from the tracked module | Assert per-module resolved versions in the preparer's test and fail the build naming the module. This is exactly the failure A-1 suspects is already live |
| R-2 | The boot proof is skipped rather than run, and a regression ships behind a green bar | A `SKIP` in the evidence output, or `ZE_GOKRAZY_SKIP_BUILD=1` set | Goal Validation requires a PASS, not a PASS-or-SKIP, and requires the skip variable unset |
| R-3 | Project `tmp/` is a symlink after `make ze-migrate-scratch` (`.gitignore:9-16`), so absolute replaces could traverse it | Prepared builds fail only on machines with relocated scratch | Resolve the prepared parent to its real path before writing any absolute replace; cover a symlinked `tmp/` in the preparer test |
| R-4 | Two concurrent builds in one checkout collide | Interleaved or corrupted builds | Unique prepared directory per build, with the existing cleanup contract |
| R-5 | The kernel parameter changes operator behaviour: today a stale replace persists until `ze-kernel-clean`, and an implicit "use `tmp/kernel/pkg` if present" rule would persist silently forever | Operators getting a custom kernel they did not ask for | Make the kernel package an explicit parameter, never an implicit filesystem probe. Document the behaviour change |
| R-6 | Scope creep into `spec-unify-dep-stores`, or back into the first draft's ambition | The diff starts deleting tracked builddir files or touching `vendor/` | Non-goals are stated in the Task section; the Mistake Log records why the deletion was wrong |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance build <name>` | → | `resolveBuildParentDir` prepares for every build, not only hugepage builds | `TestResolveBuildParentDirAlwaysPrepares` |
| `ze appliance build <name>` | → | the prepared instance carries a complete builddir, all eight modules with their sums | `TestPrepareInstanceCopiesFullBuilddir` |
| `ze appliance build <name>` with hugepages | → | full path: preparer to gok to a booting image | `test/appliance/vpp-hugepages-qemu.ci` |
| `make ze-kernel` then an image build | → | the kernel replace reaches the prepared copy and no tracked path is touched | `TestPrepareInstanceInjectsKernelReplace` plus `test/appliance/appliance-build-leaves-tree-clean.ci` |
| gokrazy image boot | → | an image built from a prepared instance boots and serves | `ze-deployment-gokrazy-l2tp-ppp-test` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any appliance build | gok is invoked with a `--parent_dir` under the project `tmp/`, never the tracked `gokrazy` dir and never the system temp dir |
| AC-2 | An appliance build runs | The set of tracked paths under `gokrazy/` is byte-identical before and after, compared inside the test rather than by inspecting the developer's working tree |
| AC-3 | A prepared instance is materialized | It contains all eight builddir modules with their `go.mod` and, where tracked, their `go.sum` |
| AC-4 | The prepared ze module is inspected | Its replace is an absolute path resolving to the repository root, and its `go` directive and requires are otherwise identical to the tracked file |
| AC-5 | `go list -m all` in each prepared module | Every resolved version equals the version resolved in the corresponding tracked module |
| AC-6 | A module is missing from the prepared instance | The build fails naming the module, instead of letting gok synthesize an empty one and fetch from the network |
| AC-7 | A build is given an out-of-tree kernel package | The replace appears in the prepared copy, no tracked file is modified, and the image boots that kernel |
| AC-8 | A build is given no kernel package | The pinned kernel is used, with no dependence on leftover state from a previous `make ze-kernel` |
| AC-9 | An image config requesting hugepages | Kernel arguments still reach `config.json` and the prepared instance still has a complete builddir |
| AC-10 | A prepared build with `GOPROXY=off` on a tree that has run `make ze-gokrazy-deps` | The build completes offline |
| AC-11 | Two builds run concurrently in one checkout | Each uses a distinct prepared directory |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds an appliance with hugepages | `ze appliance build` → preparer → gok → image → boots → `/proc/cmdline` asserted | `test/appliance/vpp-hugepages-qemu.ci` (must PASS, not SKIP) |
| 2 | `make ze-gokrazy USER=... PASS=...` | make → (D-1a or D-1b) → preparer → gok → image | `ze-deployment-gokrazy-l2tp-ppp-test` |
| 3 | `make ze-kernel` then an image build | kernel package passed as a parameter → absolute replace in the prepared copy → image boots the custom kernel | `ze-deployment-gokrazy-l2tp-ppp-test` after `make ze-kernel` |
| 4 | Inspects the repo after any build | no tracked path under `gokrazy/` changed | `test/appliance/appliance-build-leaves-tree-clean.ci` |
| 5 | Runs the L2TP QEMU evidence | script → shared preparer → gok → boots | `ze-deployment-gokrazy-l2tp-ppp-test` |
| 6 | Builds offline after a one-time deps download | preparer → copied sums → cached modules → image | `TestPreparedBuildResolvesOffline` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveBuildParentDirAlwaysPrepares` | `internal/appliance/kernelargs_test.go` | AC-1: every build prepares, hugepages or not | |
| `TestPrepareInstanceUsesProjectTmp` | `internal/appliance/kernelargs_test.go` | AC-1: not the system temp dir | |
| `TestPrepareInstanceCopiesFullBuilddir` | `internal/appliance/kernelargs_test.go` | AC-3: all eight modules and their sums | |
| `TestPrepareInstanceRewritesZeReplaceAbsolute` | `internal/appliance/kernelargs_test.go` | AC-4: absolute replace, `go` directive and requires preserved | |
| `TestPrepareInstanceResolvedVersionsMatchTracked` | `internal/appliance/kernelargs_test.go` | AC-5: per-module `go list -m all` equality | |
| `TestPrepareInstanceFailsOnMissingModule` | `internal/appliance/kernelargs_test.go` | AC-6 and R-1: loud failure, not silent bootstrap | |
| `TestPrepareInstanceInjectsKernelReplace` | `internal/appliance/kernelargs_test.go` | AC-7: replace in the prepared copy only | |
| `TestPrepareInstanceNoKernelPackageUsesPin` | `internal/appliance/kernelargs_test.go` | AC-8: no leftover-state dependence | |
| `TestPrepareInstancePreservesHugepageArgs` | `internal/appliance/kernelargs_test.go` | AC-9: `deriveInstanceConfigJSON` patch still lands | |
| `TestPreparedBuildResolvesOffline` | `internal/appliance/kernelargs_test.go` | AC-10: resolution with `GOPROXY=off` | |
| `TestPrepareInstanceConcurrentBuildsIsolated` | `internal/appliance/kernelargs_test.go` | AC-11: distinct directories | |
| `TestPrepareInstanceUnderSymlinkedTmp` | `internal/appliance/kernelargs_test.go` | R-3: real-path resolution before absolute rewrite | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| None | This spec introduces no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `appliance-build-leaves-tree-clean` | `test/appliance/appliance-build-leaves-tree-clean.ci` | AC-2: a build leaves every tracked path under `gokrazy/` unchanged, compared inside the test | |
| `vpp-hugepages-qemu` | `test/appliance/vpp-hugepages-qemu.ci` | A real build with hugepages boots and `/proc/cmdline` carries the derived arguments | |

### Interop Tests (MANDATORY for protocol features)
Not applicable: no wire protocol changes. The equivalent obligation is the QEMU
boot proof, recorded in Goal Validation.

### Future (if deferring any tests)
- None. Every acceptance criterion has a named test above.

## Files to Modify
- `internal/appliance/kernelargs.go` - `resolveBuildParentDir` always prepares; `materializeDerivedParent` copies the builddir, uses project `tmp/`, rewrites the ze replace, and injects an optional kernel replace
- `internal/appliance/cmd_build.go` - kernel package parameter plumbed through; cleanup always runs
- `mk/gokrazy.mk` - `ze-kernel` stops editing a tracked `go.mod` and passes the package path instead; `ze-kernel-clean` loses the tracked revert; the relative-path workaround (`:184-187`) is removed; `ze-gokrazy` follows D-1
- `scripts/evidence/effective-gokrazy-l2tp-ppp.py` - `prepare_instance` deleted in favour of the shared preparer
- `ai/rules/appliance-dep-bumps.md` - correct `:57`, which names a test that boots nothing, and describe the prepared-instance flow
- `.gitignore` - the entry added by `ccdc8483f` stays; nothing in this spec removes a tracked file

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no | not applicable |
| YANG validation constraints | [ ] no | not applicable |
| YANG custom validators | [ ] no | not applicable |
| CLI commands/flags | [ ] depends on D-1 | D-1a: a kernel-package flag on `ze appliance build`. D-1b: additionally a prepare subcommand, which must follow `ai/rules/cli-grammar.md` |
| CLI grammar (action before identifier) | [ ] yes if D-1b | `ai/rules/cli-grammar.md` |
| Editor autocomplete | [ ] no | not applicable |
| Functional test for new RPC/API | [ ] yes | `test/appliance/appliance-build-leaves-tree-clean.ci` |
| Pipe completeness | [ ] no | no new command output |
| Env var registration | [ ] no | no new env var |
| Doctor check for runtime dependencies | [ ] no | no new runtime dependency |
| Prometheus counters/metrics | [ ] no | build-time only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] no | build mechanics only |
| 2 | Config syntax changed? | [ ] no | - |
| 3 | CLI command added/changed? | [ ] depends on D-1 | `docs/guide/command-reference.md` if a flag or subcommand is added |
| 4 | API/RPC added/changed? | [ ] no | - |
| 5 | Plugin added/changed? | [ ] no | - |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/appliance.md` describes the builddir at `:44` and `:306`; both must match the new flow |
| 7 | Wire format changed? | [ ] no | - |
| 8 | Plugin SDK/protocol changed? | [ ] no | - |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] no | - |
| 10 | Test infrastructure changed? | [ ] yes | `docs/functional-tests.md` for the new `.ci` |
| 11 | Affects daemon comparison? | [ ] no | - |
| 12 | Internal architecture changed? | [ ] yes | the `mk/gokrazy.mk` header contract (lines 1-32) |
| 13 | Route metadata keys added/changed? | [ ] no | - |
| 14 | Prometheus counters added/changed? | [ ] no | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] no | - |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] yes | grep `docs/` for anchors naming `kernelargs.go`, `cmd_build.go`, `mk/gokrazy.mk` |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] yes | verify documented `gokrazy/ze/builddir` paths against the new flow |

## Files to Create
- `test/appliance/appliance-build-leaves-tree-clean.ci` - functional proof for AC-2

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist below |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Settle D-1 and A-1b before writing code** — A-1 is already confirmed by the 2026-07-22 A/B build; what remains is the online blast radius and the make-path decision
   - Tests: re-run the A/B with network available, comparing the resolved `rtr7/kernel` version against the pin (A-1b); audit `make ze-gokrazy` against `ze appliance build` for equivalence (A-7)
   - Files: none; this phase produces evidence recorded in Assumptions and D-1
   - Verify: A-1b and A-7 leave `unvalidated`; D-1 is decided
   - Note: `scripts/evidence/effective-vpp-hugepages-qemu.py` SKIPped on 2026-07-22 (`appliance did not reach SSH within the timeout`) because the host user is not in the `kvm` group and tcg was too slow. Fix the group membership before relying on it as the boot proof, or the Goal Validation row cannot be satisfied
2. **Phase: Wiring (MANDATORY FIRST)** — every build path prepares
   - Tests: `TestResolveBuildParentDirAlwaysPrepares`, `TestPrepareInstanceUsesProjectTmp`, `test/appliance/appliance-build-leaves-tree-clean.ci`
   - Files: `internal/appliance/kernelargs.go`, `internal/appliance/cmd_build.go`
   - Verify: the entry point always prepares and the wiring test fails because the builddir is not yet copied
3. **Phase: Copy the builddir and rewrite the ze replace**
   - Tests: `TestPrepareInstanceCopiesFullBuilddir`, `TestPrepareInstanceRewritesZeReplaceAbsolute`, `TestPrepareInstanceResolvedVersionsMatchTracked`, `TestPrepareInstanceFailsOnMissingModule`, `TestPrepareInstanceUnderSymlinkedTmp`, `TestPrepareInstanceConcurrentBuildsIsolated`, `TestPrepareInstancePreservesHugepageArgs`, `TestPreparedBuildResolvesOffline`
   - Files: `internal/appliance/kernelargs.go`
   - Verify: per-module resolved versions match the tracked instance, and the wiring test passes
4. **Phase: Kernel package as a parameter** — `ze-kernel` writes no state; the replace is injected into the prepared copy
   - Tests: `TestPrepareInstanceInjectsKernelReplace`, `TestPrepareInstanceNoKernelPackageUsesPin`, then `make ze-kernel` plus `ze-deployment-gokrazy-l2tp-ppp-test`
   - Files: `mk/gokrazy.mk`, `internal/appliance/cmd_build.go`
   - Verify: `git status --porcelain gokrazy/` is clean after a kernel build, and the image boots the custom kernel
5. **Phase: Collapse the third implementation** — delete `prepare_instance` from the evidence script
   - Tests: `ze-deployment-gokrazy-l2tp-ppp-test`
   - Files: `scripts/evidence/effective-gokrazy-l2tp-ppp.py`
   - Verify: the L2TP evidence passes with no private copy of the logic
6. **Phase: Correct the runbook** — fix the false boot-proof citation and describe the new flow
   - Tests: `make ze-doc-test`
   - Files: `ai/rules/appliance-dep-bumps.md`, `docs/guide/appliance.md`
   - Verify: no rule or doc names a test that does not do what it is cited for
7. **Functional tests** → Create after the feature works.
8. **RFC refs** → Not applicable.
9. **Full verification** → `make ze-verify`, plus the QEMU boot proofs at a PASS.
10. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`. TWO commits: commit A saves code + tests + spec + learned summary; commit B does `git rm` of the spec.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | All three build entry points use one preparer; none retains a private copy |
| Correctness | Per-module resolved versions in the prepared instance are identical to the tracked ones, not merely "the build succeeded" |
| Silent-substitution guard | Delete a module from a prepared instance and confirm a loud failure rather than a network fetch (R-1) |
| Test discrimination | For each named proof, break the production path and confirm the test goes red. The first draft of this spec named three tests that could not fail |
| Naming | The preparer is named for what it does, not for the hugepage case it grew out of |
| Data flow | The copy is one-directional; nothing is written back into `gokrazy/` |
| Rule: no-layering | The evidence script's `prepare_instance` is deleted, not left beside the shared preparer |
| Rule: testing (tmp) | The preparer writes under project `tmp/`, never the system temp dir |
| Rule: qemu-testing | The boot proof was run to a PASS, not inferred and not a SKIP |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Every build path uses the preparer | `grep -n 'parent_dir' mk/gokrazy.mk internal/appliance/*.go scripts/evidence/*.py` shows no tracked `gokrazy` parent |
| No tracked mutation during a build | `make ze-kernel && <build> && git status --porcelain gokrazy/` prints nothing |
| The evidence script has no private preparer | `grep -c 'def prepare_instance' scripts/evidence/effective-gokrazy-l2tp-ppp.py` is 0 |
| All eight builddir modules still tracked | `git ls-files gokrazy/ze/builddir | grep -c go.mod` is 8 |
| The image boots | `test/appliance/vpp-hugepages-qemu.ci` output showing PASS, and `ze-deployment-gokrazy-l2tp-ppp-test` output |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Path handling | Absolute paths are built from resolved real paths, so a symlinked `tmp/` cannot redirect a replace outside the intended tree |
| Temp directory permissions | The prepared instance is created with restrictive permissions and cleaned up |
| Pin integrity | The prepared instance cannot build a different upstream version than the tracked pins. This is the security half of R-1, and A-1 suspects it is already violated today |
| Source integrity | The prepared build resolves ze from the working tree, never from the published repository |
| Module verification | `GOMODCACHE` and `-modcacherw` handling is unchanged |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Image builds but does not boot | Phase 4 or 5; never accept a build-only proof (R-2) |
| Boot proof SKIPs | Not an answer. Install the missing prerequisite and rerun |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The tracked builddir `go.sum` had silently drifted, pinning grpc v1.80.0 after the root module moved | No committed drift ever existed. At `be16f6ed4^` the builddir sum and root `go.mod` both named v1.80.0, and `be16f6ed4` moved root `go.mod`, root `go.sum` and the builddir sum together. The drift was a transient state created and fixed within that commit | Independent review, then `git show be16f6ed4^` | Removed one of the first draft's three problem statements. The residual truth is a hazard, not a defect: nothing gates the builddir sums against the root module |
| gok seeds a missing builddir module from the **root** `go.mod` and copies the root `go.sum` | gok chdirs to the instance dir first (`overwrite.go:143`), so `gotool.go:120` and `:181` read from `gokrazy/ze/`, which has neither file. gok writes a bare module with no requires and an empty `go.sum`, then resolves over the network. The copy is also inside the `os.IsNotExist` guard (`:119-188`), so it never runs once a `go.mod` exists | Independent review, verified at `overwrite.go:143` | Invalidated the first draft's "synthesize the ze module, gok supplies the sum" design step |
| The synthetic module name derives from the checkout directory, so it is not reproducible across checkouts | It derives from the instance name. `gokrazy/go.mod` and `rtr7/kernel/go.mod` both declare `module gokrazy/build/ze` | Independent review, verified by reading the tracked modules | Removed a false reproducibility concern |
| The tracked ze builddir `go.mod` could be deleted and synthesized at build time | It is the input to `mk/gokrazy.mk:63-66`, the loop that populates `gokrazy/modcache` with ze's ~94 dependencies. Deleting it breaks the documented offline build | Independent review; corroborated by this session observing `make ze-gokrazy-deps` place `grpc@v1.82.1` into the cache | Killed the first draft's central move. All eight modules stay tracked |
| Path-bearing replaces across the pinned modules needed a general rewriting pass | Only the ze module has a filesystem-path replace. `serial-busybox:11` and `rtr7/kernel:11` carry depth-independent version replaces; the other five have none | Independent review, verified by reading all eight modules | Removed a whole design step and a vacuous assumption |
| `os.MkdirTemp` at `kernelargs.go:118` already materializes under project `tmp/` | Its empty first argument means the system temp dir, which this project's rules ban | Independent review, verified by reading the call | Turned an assumed-satisfied property into explicit work |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Delete the tracked ze builddir module and synthesize it into the prepared instance | Breaks `ze-gokrazy-deps` cache population and the offline guarantee; also relied on a `go.sum` copy that never runs | Copy the whole builddir verbatim and rewrite only the ze self-replace, which is what the working evidence script already does |
| Prove the work with `test/appliance/serial-login.ci` and `ze-qemu-l2tp-ppp-test` | Neither builds a gokrazy image. Both would stay green with the feature absent | `test/appliance/vpp-hugepages-qemu.ci` and `ze-deployment-gokrazy-l2tp-ppp-test`, which do build and boot |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A BLOCKING rule cited a test as a boot proof when that test boots nothing (`ai/rules/appliance-dep-bumps.md:57`) | 1 known, inherited by this spec at first draft | When a rule names a test as evidence for an obligation, the rule must name what the test asserts, so a stale citation is visible | Fixed in Phase 6; consider a doc gate that checks named tests against their content |
| A spec's design premises were three misreadings of vendored upstream code | 1 | Design premises drawn from a dependency's source need the same "read the producer" discipline as claims about our own code | Recorded here; the existing `ai/rules/no-fabrication.md` already covers it, the failure was applying it to `vendor/` |

## Design Insights

- The repository's builddir is an **override of a gok default**, but the override
  is load-bearing twice over, for unrelated reasons: the ze module carries a
  self-replace gok cannot synthesize, and it is simultaneously the input that
  makes `ze-gokrazy-deps` populate the module cache. The first draft saw the
  first reason and missed the second, which is what made "just delete it" look
  safe.
- gok chdirs into the instance directory before packing
  (`overwrite.go:143`). Every conclusion about what gok reads by relative path
  follows from that one line, and getting it wrong produced three separate
  downstream errors in the first draft.
- The strongest argument for this work is narrow and concrete: a build step
  writes to a tracked file. Everything broader that the first draft claimed
  either was already fixed or turned out not to be true.

## Core Insight

Three call sites had independently reached for "build somewhere else". The work
is not inventing a mechanism, it is choosing one of the three that already exist
and deleting the other two. The first draft's error was treating that as licence
to redesign what the chosen mechanism does, rather than just relocating it.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Copy the builddir verbatim into the prepared instance | Synthesize the ze module; symlink the builddir | Copy-and-rewrite is the one approach already exercised in this repo (`effective-gokrazy-l2tp-ppp.py:615`). Synthesis breaks cache population. Symlinking cannot work: the relative self-replace is depth-sensitive |
| Keep all eight builddir modules and their sums tracked | Remove the derived-looking ones; generate from a manifest | They are deliberate pins and, for the ze module, the cache-population input. The first draft's removal is recorded as a failed approach |
| Prepare unconditionally, for every build | Prepare only when a modifier demands it | A conditional path is exercised rarely and rots. A-1 suspects that is exactly how today's hugepage path came to be broken |
| Kernel package as an explicit parameter | Keep the tracked `go mod edit`; probe for `tmp/kernel/pkg` | The parameter is what actually kills the one real problem. A filesystem probe would silently persist a custom kernel forever (R-5) |
| Prepare under project `tmp/` | Keep the system temp dir the code uses today | `ai/rules/testing.md` bans `/tmp`, and the `tmp/go.mod` sentinel already stops `go list ./...` descending into a materialized instance |
| Extend `materializeDerivedParent` | A new package for instance preparation | It already builds a parent and owns the cleanup contract; a new one would be the fourth copy of this idea |

## Known Limitations

- `make ze-gokrazy-deps` still regenerates the seven tracked `go.sum` files in
  place (`mk/gokrazy.mk:63-66`). That is a maintenance step, not a build step,
  and it is deliberate per `ai/rules/appliance-dep-bumps.md:40-48`. The "no build
  step writes to a tracked path" goal is scoped to builds accordingly.
- Nothing gates the tracked builddir sums against the root module, which is the
  residual hazard behind the corrected drift claim. Adding that gate is not in
  scope here and has no destination spec yet; it needs one before this spec
  closes.
- The repository directory keeps the name `builddir` even though the build no
  longer runs there. Renaming is a mechanical follow-up, not bundled here, and
  also needs a destination before closure.
- This spec does not reduce the number of committed dependency stores. That is
  `plan/spec-unify-dep-stores.md`.

## RFC Documentation

Not applicable. This spec changes build orchestration and enforces no protocol requirement.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation; A-1 may become the first entry)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| No build step writes to a tracked path | functional test | `test/appliance/appliance-build-leaves-tree-clean.ci` output, plus a clean `git status --porcelain gokrazy/` after `make ze-kernel` and a build |
| An image built the new way boots | QEMU boot proof | `test/appliance/vpp-hugepages-qemu.ci` output showing PASS, with `ZE_GOKRAZY_SKIP_BUILD` unset. A SKIP is not evidence |
| The custom-kernel path still works end to end | QEMU boot proof | `ze-deployment-gokrazy-l2tp-ppp-test` output after `make ze-kernel` |
| The pins are preserved exactly | version comparison | Per-module `go list -m all` diff, tracked against prepared, empty for all eight |
| Offline builds still work | functional evidence | A prepared build with `GOPROXY=off` completing after `make ze-gokrazy-deps` |
| One preparer, not three | absence check | `grep -c 'def prepare_instance' scripts/evidence/effective-gokrazy-l2tp-ppp.py` is 0, and no build path names a tracked `gokrazy` parent |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | (fill from /ze-review) | file:line | fixed / deferred (id) / acknowledged |

### Fixes applied
- (fill during review)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] D-1 decided and recorded before implementation began
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] Every named proof mutation-verified: break the production path, confirm the test goes red
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] QEMU boot proofs run to a PASS, never a SKIP
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary
- [ ] Known Limitations needing a destination spec have one before closure

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (N/A for this spec)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A — no numeric input)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A — QEMU boot proof stands in)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
