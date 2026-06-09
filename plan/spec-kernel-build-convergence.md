# Spec: kernel-build-convergence

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 8/8 |
| Updated | 2026-06-09 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `ai/rules/planning.md` - workflow rules
3. `tools/kernel-builder/build.sh` - shared builder (created by this spec, extracted from tools/installer-kernel/build.sh)
4. `tools/kernel-builder/qemu-build.py` - shared QEMU backend (extracted, parameterized)
5. `tools/installer-kernel/Makefile` - installer orchestrator (delegates to ../kernel-builder/)
6. `gokrazy/kernel/Makefile` - runtime orchestrator (delegates to ../../tools/kernel-builder/)
7. `mk/gokrazy.mk` - make ze-kernel target (calls gokrazy/kernel/Makefile, then copies to modcache)
8. `internal/appliance/cmd_kernel.go` - ze appliance kernel command (three-tier resolve)
9. `internal/appliance/cache.go` - cache variant with config-hash
10. `gokrazy/ze/config.json` - KernelPackage reference (rtr7/kernel)

## Task

Converge the runtime kernel (`make ze-kernel`) and installer kernel (`ze appliance kernel`) onto a single build system with Linux 7.0.11. Extract the installer's build scripts into a shared `tools/kernel-builder/` directory. Replace the external `gokr-rebuild-kernel` binary and `gokrazy-kernel-build.sh` with the shared builder. Both kernels use the same builder scripts (build.sh, qemu-build.py, Dockerfile), same kernel version, differing only in config fragments and output artifacts.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/578-gokrazy-3-build.md` - gokrazy build config decisions
  -> Decision: explicit GokrazyPackages with only randomd and heartbeat
  -> Constraint: gokrazy config.json KernelPackage must point to a Go module with vmlinuz
- [ ] `plan/learned/856-install-10-iso-prerequisites.md` - installer kernel scaffolding decisions
  -> Decision: three-tier resolution: XDG cache, download from release server, local build
  -> Constraint: build functions are injectable via function vars for testing
- [ ] `plan/learned/854-install-8-appliance-iso.md` - ISO pipeline consuming the installer kernel
  -> Constraint: ISO consumes kernel from `tools/installer-kernel/build/Image` or XDG cache
- [ ] `plan/learned/599-l2tp-5-kernel.md` - L2TP kernel config requirements
  -> Constraint: CONFIG_PPP, CONFIG_PPPOL2TP, CONFIG_L2TP, CONFIG_L2TP_V3, CONFIG_L2TP_NETLINK must be =y

### RFC Summaries (MUST for protocol work)
N/A - no protocol work.

**Key insights:**
- gokr-rebuild-kernel is `package main`, not importable. Ze replaces it with its own shared builder
- The installer already has a complete build pipeline: build.sh (125 lines) uses `defconfig` + `merge_config.sh -m` + per-profile verification. This is better than gokr-rebuild-kernel's `mod2noconfig` approach
- `merge_config.sh -m` overlays fragments onto defconfig preserving the ability to set `=m` for modules
- The runtime kernel needs modules (gokrazy loads them at boot). The installer kernel does not
- build.sh outputs `Image` (arm64 or amd64 bzImage copied to uniform name). Runtime needs vmlinuz + modules + DTBs instead
- `qemu-build.py:427-428` hardcodes `SRC_DIR=/workspace/tools/installer-kernel`. Must be parameterized for shared use
- `ze appliance kernel` lives in `bin/ze-setup` (ze_setup build tag), not `bin/ze`
- CONFIG_FB_SIMPLE is in both `tools/installer-kernel/hardware.config:22` and the runtime addendum. Replacement: CONFIG_DRM_SIMPLEDRM=y + CONFIG_X86_SYSFB=y (simpledrm replaces simplefb in 7.0; earlier 6.x issues may be resolved)
- Cache invalidation currently hashes build.sh + config files (cache.go:74-87). After extraction, cache key must hash the new fragment paths
- Docker builds must pass an explicit target platform matching ARCH for both `docker build` and `docker run`: amd64 -> `linux/amd64`, arm64 -> `linux/arm64`. The builder does not rely on implicit host architecture.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `mk/gokrazy.mk` - ze-kernel target: clones rtr7/kernel, appends L2TP addendum, builds via gokr-rebuild-kernel in Docker, copies vmlinuz+modules+DTBs to modcache. KVER=6.19.11. ze-kernel-clean restores .ze-pinned-kernel backup, removes tmp/kernel.
  -> Constraint: KVER_MAJOR derivation auto-handles v6.x/v7.x URL
  -> Constraint: ze-kernel copies vmlinuz + modules + DTBs into KERNEL_MODCACHE_DIR
- [ ] `scripts/dev/gokrazy-kernel-build.sh` - Wraps gokr-rebuild-kernel: builds binary via `go install`, generates Dockerfile.ze, Docker build+run. Pipeline: defconfig, mod2noconfig, append addendum, olddefconfig, make.
- [ ] `tools/installer-kernel/build.sh` (125 lines) - Downloads tarball, extracts, `defconfig` + `merge_config.sh -m`, verifies required options per profile, `make Image`. Parameterized: SRC_DIR, OUT_DIR, LINUX_VERSION, ARCH, PROFILE, JOBS.
  -> Constraint: output always named `Image` (arm64 Image, amd64 bzImage both copied to Image)
  -> Constraint: verifies required options per profile before building
- [ ] `tools/installer-kernel/qemu-build.py` (528 lines) - QEMU Alpine VM backend. Hardcodes SRC_DIR and OUT_DIR to `/workspace/tools/installer-kernel` at lines 427-428 and build.sh path at line 432.
  -> Constraint: Must be parameterized to support different source directories
- [ ] `tools/installer-kernel/Makefile` (82 lines) - BUILDER=qemu|docker, PROFILE, ARCH. Docker: docker build + docker run with build.sh. QEMU: calls qemu-build.py.
  -> Constraint: default BUILDER=qemu, ARCH=arm64
- [ ] `tools/installer-kernel/kernel.config` - Installer base (38 lines): IP autoconfig, SCSI, ext4, initrd, serial
- [ ] `tools/installer-kernel/qemu.config` - Virtio only (11 lines)
- [ ] `tools/installer-kernel/hardware.config` - Real hardware (69 lines). CONFIG_FB_SIMPLE=y at line 22
- [ ] `tools/installer-kernel/hardware-kms.config` - i915 KMS (22 lines)
- [ ] `tools/installer-kernel/Dockerfile` - Debian bookworm with build tools
- [ ] `internal/appliance/cmd_kernel.go` - Three-tier resolve. Profiles: qemu, hardware, hardware-kms. Injectable kernelDockerBuildFn/kernelQEMUBuildFn. Docker mounts tools/installer-kernel as /src:ro.
  -> Constraint: kernelToolsDir = "tools/installer-kernel" (constant, line 23)
- [ ] `internal/appliance/cache.go` - kernelCacheVariant (line 74-87) hashes kernel.config + profile.config + build.sh from kernelToolsDir.
- [ ] `internal/appliance/cmd_kernel_test.go` - 25 Test* functions (exact names listed in TDD plan)
- [ ] `cmd/ze/setup_features_setup.go` - `//go:build ze_setup`. Imports internal/appliance. Binary is bin/ze-setup.
- [ ] `gokrazy/kernel/l2tp.config.addendum.txt` - 11 config lines
- [ ] `gokrazy/kernel/qemu-evidence.config.addendum.txt` - 17 config lines
- [ ] `tmp/kernel/_build/config.addendum.txt` - 279 lines. Scratch file, cleaned by ze-kernel-clean.
- [ ] `tmp/kernel/_build/0001-nct6683.patch` - Hwmon driver patch. Scratch tree.
- [ ] `gokrazy/modcache/.ze-pinned-kernel/` - Backup of rtr7/kernel module with 6.19.11 vmlinuz

**Behavior to preserve:**
- Three-tier resolution in cmd_kernel.go (cache, download, build)
- Installer profile system (qemu, hardware, hardware-kms)
- `--builder docker|qemu` flag and auto-detection
- Config-hash cache invalidation
- Injectable build functions for test isolation
- `make ze-gokrazy` picks up kernel from modcache
- Installer artifacts at `tools/installer-kernel/build/Image`
- `make ze-kernel-clean` restores pinned kernel
- build.sh verification of required options per profile
- QEMU build backend (qemu-build.py) as working fallback
- `make -C tools/installer-kernel` standalone operation

**Behavior to change:**
- Runtime kernel: 6.19.11 -> 7.0.11
- Extract builder scripts to shared `tools/kernel-builder/`
- Parameterize qemu-build.py (SRC_DIR, OUT_DIR, BUILDER_DIR)
- Runtime build: replace gokr-rebuild-kernel with shared builder
- Extend build.sh: MODULES=yes flag for vmlinuz + modules output; PATCHES_DIR for patch application
- Runtime config fragments under `gokrazy/kernel/` (tracked, not tmp/)
- nct6683 patch: move to `gokrazy/kernel/patches/`
- Remove dead config options from all fragments
- Replace CONFIG_FB_SIMPLE with CONFIG_DRM_SIMPLEDRM=y + CONFIG_X86_SYSFB=y in both runtime and installer
- Merge l2tp/qemu-evidence fragments into runtime fragment
- `make ze-kernel` calls `make -C gokrazy/kernel` then copies artifacts to modcache
- `make ze-kernel-clean` also runs `make -C gokrazy/kernel clean` (removes gokrazy/kernel/build/) in addition to restoring pinned modcache and removing tmp/kernel
- `cmd_kernel.go`: split `kernelToolsDir` into `kernelBuilderDir`, `kernelInstallerConfigDir`, `kernelInstallerOutputDir`

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Runtime kernel: `make ze-kernel` -> `make -C gokrazy/kernel BUILDER=docker|qemu` -> `tools/kernel-builder/build.sh` -> modcache install
- Installer kernel: `ze appliance kernel --profile X` -> `tools/kernel-builder/build.sh` via Docker/QEMU (existing three-tier resolve)
- Installer standalone: `make -C tools/installer-kernel` -> `tools/kernel-builder/build.sh`

### Transformation Path
1. Profile selection: Makefile variable or CLI flag
2. Builder selection: BUILDER=docker|qemu
3. Docker: `tools/kernel-builder/Dockerfile` + build.sh mounted, docker run with env vars
4. QEMU: `tools/kernel-builder/qemu-build.py` boots Alpine VM, runs build.sh via virtio-9p
5. Inside build env: download tarball, extract, apply patches (if PATCHES_DIR set), defconfig + merge_config.sh -m, olddefconfig, verify required options, make, copy artifacts
6. Runtime post-build: `mk/gokrazy.mk` copies vmlinuz + modules + DTBs to modcache
7. Installer post-build: Image cached to XDG + copied to tools/installer-kernel/build/Image

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Makefile -> Docker/QEMU | make target or exec.Command via injectable build functions | [ ] |
| Config fragments -> merged config | merge_config.sh -m inside build env | [ ] |
| Docker container -> host | Volume mount at build output dir | [ ] |
| QEMU VM -> host | virtio-9p mount | [ ] |
| Kernel artifacts -> gokrazy modcache | File copy (mk/gokrazy.mk post-step) | [ ] |
| Kernel artifacts -> XDG cache | File copy (ze appliance kernel) | [ ] |

### Integration Points
- `tools/kernel-builder/build.sh` - shared build script. Inputs: SRC_DIR (dir containing config fragments), OUT_DIR, MODULES (yes/no, default no), PATCHES_DIR (optional, dir containing series file + .patch files). Accepted profile merge rules: qemu/hardware/runtime -> `kernel.config` + `${PROFILE}.config`; hardware-kms -> `kernel.config` + `hardware.config` + `hardware-kms.config`. Runtime fragments are named `gokrazy/kernel/kernel.config` + `gokrazy/kernel/runtime.config` to match this convention. After `olddefconfig`, build.sh verifies required options for the selected profile, including runtime-only L2TP/PPP/module requirements.
- `tools/kernel-builder/qemu-build.py` - shared QEMU backend. Flags: --src-dir, --out-dir, --builder-dir, --modules (yes/no), --patches-dir. Path flags are repo-relative. Passes MODULES and PATCHES_DIR as env vars to build.sh inside the VM.
- `cmd_kernel.go:resolveKernel()` - three-tier resolution (installer path)
- `mk/gokrazy.mk:ze-kernel` - build + install to modcache
- `cmd_iso.go` - reads kernel from tools/installer-kernel/build/Image or XDG cache
- `cache.go:kernelCacheVariant()` - hashes config files for cache key

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality (single builder, two config sets)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make -C gokrazy/kernel BUILDER=docker` | -> | `tools/kernel-builder/build.sh` with MODULES=yes, PATCHES_DIR, and Docker `--platform` matching ARCH | `test-runtime-makefile-delegates` in `test/install/kernel-wiring.ci` |
| `make -C gokrazy/kernel BUILDER=qemu` | -> | `tools/kernel-builder/qemu-build.py` with repo-relative src/out/builder/patch paths and MODULES=yes | `test-runtime-makefile-delegates` in `test/install/kernel-wiring.ci` |
| `ze appliance kernel --profile qemu` | -> | `resolveKernel` -> `defaultDockerBuild` -> `tools/kernel-builder/build.sh` | `TestDockerBuildMountsSharedBuilder` (new: verifies defaultDockerBuild constructs docker run with kernelBuilderDir mounted at /builder and /builder/build.sh entrypoint) |
| Config fragment change | -> | Cache variant changes | `TestKernelConfigHashInvalidatesCache` (existing) |
| `make -C tools/installer-kernel` | -> | `tools/kernel-builder/build.sh` | `test-installer-makefile-delegates` in `test/install/kernel-wiring.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-kernel` with KVER=7.0.11 | Calls `make -C gokrazy/kernel`, which invokes `tools/kernel-builder/build.sh` via Docker or QEMU. Produces vmlinuz + modules. mk/gokrazy.mk copies artifacts to modcache. |
| AC-2 | `ze appliance kernel --profile qemu` with empty cache and no download URL | Falls through cache and download tiers, builds locally via Docker or QEMU using `tools/kernel-builder/build.sh`, caches result |
| AC-3 | `ze appliance kernel --profile hardware` with empty cache | Builds with hardware fragments via shared builder, includes real NIC drivers, produces `Image` artifact |
| AC-4 | Runtime config fragments (base + runtime) | Cover all options from current 279-line addendum minus dead options. No feature regression for gokrazy appliance kernel. |
| AC-5 | Installer profiles (qemu, hardware, hardware-kms) | Required installer options resolve to `=y` per build.sh verification. No modules artifact is shipped (no lib/modules/ copied into ISO pipeline). |
| AC-6 | Runtime profile with MODULES=yes | vmlinuz + lib/modules/ + DTBs produced. Modules available for gokrazy to load at boot. |
| AC-7 | Dead config options | Removed from ALL fragments (runtime and installer): 6 nftables options (NF_NAT_IPV4, NF_NAT_MASQUERADE_IPV4, NFT_MASQ_IPV4, NFT_CHAIN_NAT_IPV4, NFT_CHAIN_ROUTE_IPV4, NFT_CHAIN_ROUTE_IPV6; gone since 5.x) + CONFIG_USB_DEVICEFS (gone since 3.5) |
| AC-8 | CONFIG_FB_SIMPLE | Replaced with `CONFIG_DRM_SIMPLEDRM=y` + `CONFIG_X86_SYSFB=y` in BOTH `gokrazy/kernel/kernel.config` AND `tools/installer-kernel/hardware.config`. Old `CONFIG_FB_SIMPLE=y` and `CONFIG_DRM_SIMPLEDRM=n` / `CONFIG_X86_SYSFB=n` lines removed. |
| AC-9 | `gokrazy-kernel-build.sh` removed | make ze-kernel no longer uses the shell script or gokr-rebuild-kernel binary |
| AC-10 | Three-tier resolution preserved | Cache hit, download, build fallback all work for installer. `--builder docker\|qemu` flag preserved. |
| AC-11 | Existing cmd_kernel_test.go tests pass | All 25 tests updated and passing |
| AC-12 | nct6683 patch tracked | Patch at `gokrazy/kernel/patches/0001-nct6683.patch`. Rebased for 7.0. If upstream covers the use case, implementer documents the evidence (git log / Kconfig diff) in the spec's Mistake Log, removes the patch, and adjusts the deliverable. |
| AC-13 | Cache invalidation after build extraction | `kernelCacheVariant()` hashes config files from `kernelInstallerConfigDir` ("tools/installer-kernel") and build.sh from `kernelBuilderDir` ("tools/kernel-builder"). Split replaces single `kernelToolsDir`. |
| AC-14 | `tools/installer-kernel/` standalone make still works | `make -C tools/installer-kernel PROFILE=hardware ARCH=amd64` works. Makefile delegates to `../kernel-builder/`. |
| AC-15 | Shared builder via both Docker and QEMU | `make -C gokrazy/kernel BUILDER=docker` and `make -C gokrazy/kernel BUILDER=qemu` both work. Docker build/run pass `--platform` matching ARCH. qemu-build.py accepts --src-dir, --out-dir, --builder-dir, --modules, --patches-dir and passes MODULES + PATCHES_DIR as env vars to build.sh inside the VM. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test-runtime-makefile-delegates` | `test/install/kernel-wiring.ci` | gokrazy/kernel/Makefile dry-run covers Docker and QEMU: Docker passes `--platform`, BUILDER_DIR, SRC_DIR, OUT_DIR, MODULES=yes, PATCHES_DIR=patches; QEMU passes repo-relative --src-dir, --out-dir, --builder-dir, --modules yes, --patches-dir | |
| `test-installer-makefile-delegates` | `test/install/kernel-wiring.ci` | tools/installer-kernel/Makefile dry-run covers Docker and QEMU: Docker passes shared builder/config/output mounts, target platform, and no MODULES/PATCHES_DIR; QEMU passes --src-dir tools/installer-kernel, --out-dir tools/installer-kernel/build, --builder-dir tools/kernel-builder | |
| `TestDockerBuildMountsSharedBuilder` | `internal/appliance/cmd_kernel_test.go` | New: injects fake docker exec, verifies docker build/run use kernelBuilderDir, kernelInstallerConfigDir, kernelInstallerOutputDir, `/builder/build.sh`, and `--platform` matching ARCH | |
| `TestQEMUBuildPassesBuilderDir` | `internal/appliance/cmd_kernel_test.go` | New: injects fake python3 exec, verifies qemu-build.py is called with --builder-dir, --src-dir, --out-dir for installer profiles, and does not pass MODULES/PATCHES_DIR | |
| `TestKernelConfigHashInvalidatesCache` | `internal/appliance/cmd_kernel_test.go` | Existing test, verify it passes with updated paths | |
| `TestKernelBuildScriptInvalidatesCache` | `internal/appliance/cmd_kernel_test.go` | Existing test, verify it passes with build.sh from kernelBuilderDir | |

Existing 25 tests (all must pass, updated for any path changes):
`TestRunDispatchesKernel`, `TestKernelResolvesCache`, `TestKernelCacheHitCopiesToToolsPath`, `TestKernelDownloadsAndCaches`, `TestKernelDownloadChecksumMismatch`, `TestKernelFallsBackToQEMU`, `TestKernelFailsWithoutBuilders`, `TestKernelArchFlag`, `TestKernelProfileFlag`, `TestKernelVersionFlag`, `TestKernelCopiesToToolsPath`, `TestKernelReadsArchFromAppliance`, `TestKernelReadsProfileFromAppliance`, `TestKernelConfigHashInvalidatesCache`, `TestKernelBuildScriptInvalidatesCache`, `TestKernelEnvURL`, `TestSelectBuilderExplicitDocker`, `TestSelectBuilderExplicitDockerUnavailable`, `TestSelectBuilderExplicitQEMU`, `TestSelectBuilderExplicitQEMUUnavailable`, `TestSelectBuilderAutoDocker`, `TestSelectBuilderAutoFallsBackToQEMU`, `TestSelectBuilderAutoNoneAvailable`, `TestKernelBuilderFlag`, `TestKernelFallsBackToDocker`

### Boundary Tests (MANDATORY for numeric inputs)
N/A.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-kernel-fragment-compose` | `test/install/kernel-compose.ci` | Verify runtime fragments compose to expected superset of required options | |

### Interop Tests (MANDATORY for protocol features)
N/A.

### Future (if deferring any tests)
- Docker/QEMU kernel build integration test (~5 min). Manual verification. Not in CI.

## Files to Modify

- `mk/gokrazy.mk` - KVER=7.0.11. ze-kernel calls `make -C gokrazy/kernel`, then copies vmlinuz+modules+DTBs to modcache. Remove gokr-rebuild-kernel dependency and dead variables: `KERNEL_DIR`, `KERNEL_UPSTREAM_URL`, `KERNEL_BUILDDIR_MOD`, `KERNEL_L2TP_CONFIG`, `KERNEL_CONTAINER`, `KERNEL_FLAVOR`, `KERNEL_REBUILD_VER`. Retain: `KVER`, `KVER_MAJOR`, `KERNEL_MODULE`, `KERNEL_MODULE_VERSION`, `KERNEL_MODCACHE_DIR`, `KERNEL_PINNED_BACKUP`, `KERNEL_ARCH`. ze-kernel-clean adds `make -C gokrazy/kernel clean` before restoring pinned backup.
- `tools/installer-kernel/Makefile` - Delegate to `../kernel-builder/` for build.sh, qemu-build.py, Dockerfile. Keep config fragments in place. Docker path uses target-platform build/run, mounts `../kernel-builder` at `/builder:ro`, installer config dir at `/src:ro`, `build/` at `/out`, and runs `sh /builder/build.sh` without MODULES or PATCHES_DIR. QEMU path passes `--src-dir tools/installer-kernel --out-dir tools/installer-kernel/build --builder-dir tools/kernel-builder` plus arch/profile/version.
- `tools/installer-kernel/hardware.config` - Line 22: replace `CONFIG_FB_SIMPLE=y` with `CONFIG_DRM_SIMPLEDRM=y` + `CONFIG_X86_SYSFB=y`
- `tools/installer-kernel/README.md` - Update build.sh path references (now at ../kernel-builder/build.sh). Update usage examples.
- `internal/appliance/cmd_kernel.go` - Split `kernelToolsDir` into three constants: `kernelBuilderDir = "tools/kernel-builder"` (build.sh, qemu-build.py, Dockerfile), `kernelInstallerConfigDir = "tools/installer-kernel"` (kernel.config, profile configs), `kernelInstallerOutputDir = "tools/installer-kernel/build"` (Image output). Update `resolveKernel()` to copy to `filepath.Join(kernelInstallerOutputDir, kernelFileName)`. Update `defaultDockerBuild`: `docker build --platform <target-platform> -t $(kernelDockerImage) $(kernelBuilderDir)`, then `docker run --platform <target-platform>` with mounts `-v $(kernelBuilderDir):/builder:ro -v $(kernelInstallerConfigDir):/src:ro -v $(kernelInstallerOutputDir):/out`, env `SRC_DIR=/src OUT_DIR=/out PROFILE=$(profile) ARCH=$(arch) LINUX_VERSION=$(version)`, entrypoint `sh /builder/build.sh`. No MODULES or PATCHES_DIR for installer. Update `defaultQEMUBuild`: pass `--builder-dir $(kernelBuilderDir) --src-dir $(kernelInstallerConfigDir) --out-dir $(kernelInstallerOutputDir)` to qemu-build.py.
  Build function contract stays unchanged: defaultDockerBuild and defaultQEMUBuild must copy the produced installer Image into the `destPath` cache file before returning. `resolveKernel()` then copies that cached file back to `kernelInstallerOutputDir` for ISO consumers.
- `internal/appliance/cache.go` - Restructure `kernelCacheVariant()`: config files hashed from `kernelInstallerConfigDir`, build.sh hashed from `kernelBuilderDir`. Current `cacheFileHash(kernelToolsDir, inputs)` takes a single base dir, so either use two `cacheFileHash` calls and combine the hashes, or switch to full-path inputs. The result must change when either a config fragment or build.sh changes.
- `internal/appliance/cmd_kernel_test.go` - Update path references for split constants. All 25 tests must pass.

### Files to Delete/Move
- DELETE `scripts/dev/gokrazy-kernel-build.sh` - replaced by shared builder
- DELETE `gokrazy/kernel/l2tp.config.addendum.txt` - merged into `gokrazy/kernel/runtime.config`
- DELETE `gokrazy/kernel/qemu-evidence.config.addendum.txt` - merged into `gokrazy/kernel/runtime.config`
- MOVE `tools/installer-kernel/build.sh` -> `tools/kernel-builder/build.sh` (extended with MODULES, PATCHES_DIR)
- MOVE `tools/installer-kernel/qemu-build.py` -> `tools/kernel-builder/qemu-build.py` (parameterized)
- MOVE `tools/installer-kernel/Dockerfile` -> `tools/kernel-builder/Dockerfile`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No (existing profiles unchanged) | |
| Doctor check | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No (installer CLI unchanged) | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` - update kernel build commands, version. `docs/guide/ze-install.md` - update `make -C tools/installer-kernel` refs (now delegates to kernel-builder), kernel version, build.sh refs. |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - check for stale kernel build references |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, etc. changed? | No | |
| 16 | Any changed source file referenced by doc source anchors? | Yes | Grep `docs/` for `gokrazy-kernel-build.sh`, `make -C tools/installer-kernel`, `build.sh`, `tools/installer-kernel/build/Image`, `make ze-kernel`. Update stale refs. |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ze-install.md:175-176` shows `make -C tools/installer-kernel` examples. Verify they still work after Makefile changes. |

## Files to Create

- `tools/kernel-builder/build.sh` - Shared builder: extracted from tools/installer-kernel/build.sh. Extended with MODULES=yes/no (default no) and PATCHES_DIR (default empty).
  Profile dispatch: qemu/hardware/runtime use `kernel.config` + `${PROFILE}.config`; hardware-kms uses `kernel.config` + `hardware.config` + `hardware-kms.config`.
  Verification: installer profiles keep the current required built-in checks. Runtime verifies after `olddefconfig` at least `CONFIG_MODULES=y`, `CONFIG_PPP=y`, `CONFIG_PPPOL2TP=y`, `CONFIG_L2TP=y`, `CONFIG_L2TP_V3=y`, `CONFIG_L2TP_NETLINK=y`, `CONFIG_DEVTMPFS_MOUNT=y`, `CONFIG_BLK_DEV_INITRD=y`, and `CONFIG_VIRTIO_NET=y`; the full 200+ option superset remains covered by `test-kernel-fragment-compose`.
  Patch handling: when PATCHES_DIR is set, forces a fresh source tree extraction (removes and re-extracts from tarball, ignoring any cached tree), then reads `$PATCHES_DIR/series` and applies each listed .patch via `patch -p1` before configure. This avoids double-apply failures when reusing a cached build tree with already-applied patches.
  Runtime output: when MODULES=yes, runs `make modules -j$JOBS`, `make INSTALL_MOD_PATH=$OUT_DIR modules_install`, removes stale `$OUT_DIR/lib/modules/*/build` and `$OUT_DIR/lib/modules/*/source` symlinks, copies vmlinuz (amd64: `arch/x86/boot/bzImage`, arm64: `arch/arm64/boot/Image`) to `$OUT_DIR/vmlinuz`, recursively flattens `arch/arm64/boot/dts/**/*.dtb` into `$OUT_DIR/`, and copies `overlays/` if present.
  Installer output: when MODULES=no, behavior matches current build.sh: copies kernel image to `$OUT_DIR/Image`, writes resolved config, no modules, no DTBs.
- `tools/kernel-builder/qemu-build.py` - Extracted from tools/installer-kernel/qemu-build.py. New flags: `--src-dir` (dir with kernel.config + profile.config), `--out-dir` (output), `--builder-dir` (dir with build.sh + Dockerfile), `--modules` (yes/no), `--patches-dir` (optional). Constructs build_env from flag values instead of hardcoding. All path flags are **repo-relative** (e.g. `--src-dir gokrazy/kernel`, `--builder-dir tools/kernel-builder`, `--out-dir gokrazy/kernel/build`). Makefiles pass repo-relative paths: runtime Makefile computes `REPO_ROOT := $(shell git -C $(dir $(lastword $(MAKEFILE_LIST))) rev-parse --show-toplevel)` and passes `--src-dir` as path relative to repo root. VM 9p mounts (unchanged topology): `workspace` -> repo root, `builddir` -> tmp/qemu/build/ (tarball cache), `ccache` -> tmp/qemu/ccache/. build_env maps: `SRC_DIR=/workspace/${src_dir}`, `OUT_DIR=/workspace/${out_dir}`, `MODULES=${modules}`, `PATCHES_DIR=/workspace/${patches_dir}` (if set). Entrypoint: `sh /workspace/${builder_dir}/build.sh`.
- `tools/kernel-builder/Dockerfile` - Moved from tools/installer-kernel/Dockerfile. Add `patch` to `apt-get install` (required for PATCHES_DIR; current Dockerfile lacks it). Also add `patch` to Alpine BUILD_PACKAGES constant in qemu-build.py (`apk add` list, currently at line 40-43).
- `gokrazy/kernel/Makefile` - Runtime kernel orchestrator. BUILDER=docker|qemu, KVER=7.0.11, ARCH. Computes REPO_ROOT and BUILDER_DIR=$(REPO_ROOT)/tools/kernel-builder. PROFILE=runtime.
  Docker path: ARCH=amd64 -> `--platform=linux/amd64`; ARCH=arm64 -> `--platform=linux/arm64`. Run `docker build --platform ... -t ze-runtime-kernel-builder $(BUILDER_DIR)`, then `docker run --platform ...` mounting `$(BUILDER_DIR):/builder:ro`, `$(CURDIR):/src:ro`, `$(CURDIR)/build:/out`, with env `SRC_DIR=/src OUT_DIR=/out MODULES=yes PATCHES_DIR=/src/patches PROFILE=runtime`, entrypoint `sh /builder/build.sh`.
  QEMU path: `python3 $(BUILDER_DIR)/qemu-build.py --src-dir gokrazy/kernel --out-dir gokrazy/kernel/build --builder-dir tools/kernel-builder --modules yes --patches-dir gokrazy/kernel/patches --profile runtime --arch $(ARCH) --version $(KVER)`.
  Output: build/vmlinuz, build/lib/modules/, build/*.dtb.
  Clean target: `clean: rm -rf build` (required by `make ze-kernel-clean` which calls `make -C gokrazy/kernel clean`).
- `gokrazy/kernel/kernel.config` - Runtime base config (matches build.sh convention: `${SRC_DIR}/kernel.config`). Contents: IPv6, squashfs, fuse, nftables core, EFI, framebuffer (DRM_SIMPLEDRM, X86_SYSFB), /proc/config.gz, kexec, WERROR=n, zstd, landlock, debug
- `gokrazy/kernel/runtime.config` - Runtime profile (matches build.sh convention: `${SRC_DIR}/${PROFILE}.config` where PROFILE=runtime). Contents: NIC drivers (I40E, ICE, BNXT, MLX5, IGB, IGC), NVMe, USB, KVM, containers, BBR, WireGuard, bridge, macvlan, tun, L2TP/PPP, QEMU evidence, sensors, watchdog, virtio, Ryzen, IRQ, namespaces
- `gokrazy/kernel/patches/0001-nct6683.patch` - Rebased hwmon patch (moved from tmp/kernel/_build/)
- `gokrazy/kernel/patches/series` - Patch series file
- `test/install/kernel-wiring.ci` - Wiring test: verifies both Makefiles delegate to shared builder with correct env vars (dry-run, no actual kernel build). Runtime Makefile must pass MODULES=yes, PATCHES_DIR, and repo-relative paths. Installer Makefile must not pass MODULES or PATCHES_DIR.
- `test/install/kernel-compose.ci` - Functional test: verifies runtime config fragments compose to expected superset of required options

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8-10. Fix/reverify loop | Until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**.

1. **Phase: Extract shared builder + update Go paths** -- Move build.sh, qemu-build.py, Dockerfile from tools/installer-kernel/ to tools/kernel-builder/. Extend build.sh with MODULES, PATCHES_DIR, runtime profile support, runtime required-option verification, and Docker target-platform support in callers. Parameterize qemu-build.py with --src-dir, --out-dir, --builder-dir, --modules, --patches-dir flags. Update tools/installer-kernel/Makefile to delegate to ../kernel-builder/. Simultaneously update cmd_kernel.go (split `kernelToolsDir` into `kernelBuilderDir`, `kernelInstallerConfigDir`, `kernelInstallerOutputDir`), cache.go (restructure `kernelCacheVariant` for two dirs), and cmd_kernel_test.go (update path references). File moves and Go path updates must be atomic: if build.sh moves but cmd_kernel.go still points to the old path, cache hash tests break.
   - Tests: `test-installer-makefile-delegates` (kernel-wiring.ci), `TestDockerBuildMountsSharedBuilder`, `TestQEMUBuildPassesBuilderDir`, all 25 existing cmd_kernel tests
   - Files: `tools/kernel-builder/build.sh`, `tools/kernel-builder/qemu-build.py`, `tools/kernel-builder/Dockerfile`, `tools/installer-kernel/Makefile`, `internal/appliance/cmd_kernel.go`, `internal/appliance/cache.go`, `internal/appliance/cmd_kernel_test.go`
   - Verify: `make -C tools/installer-kernel PROFILE=qemu ARCH=arm64` works (dry-run or actual). All 25 existing tests pass. Cache hash tests use new paths.

2. **Phase: Runtime config fragments** -- Split 279-line addendum into base + runtime under gokrazy/kernel/. Move nct6683 patch. Remove dead options. Replace CONFIG_FB_SIMPLE with CONFIG_DRM_SIMPLEDRM=y + CONFIG_X86_SYSFB=y.
   - Tests: `test-kernel-fragment-compose`
   - Files: `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/patches/`. Delete `gokrazy/kernel/l2tp.config.addendum.txt`, `gokrazy/kernel/qemu-evidence.config.addendum.txt`.
   - Verify: `cat gokrazy/kernel/kernel.config gokrazy/kernel/runtime.config` equivalent to current addendum minus dead options plus FB_SIMPLE fix.

3. **Phase: Runtime Makefile** -- Create gokrazy/kernel/Makefile delegating to tools/kernel-builder/ with MODULES=yes, PATCHES_DIR=patches. Wire make ze-kernel in mk/gokrazy.mk to call it.
   - Tests: `test-runtime-makefile-delegates` (kernel-wiring.ci)
   - Files: `gokrazy/kernel/Makefile`, `mk/gokrazy.mk`
   - Verify: `make -C gokrazy/kernel BUILDER=docker ARCH=amd64` produces vmlinuz + modules (dry-run or actual).

4. **Phase: Fix installer CONFIG_FB_SIMPLE** -- Update hardware.config line 22.
   - Files: `tools/installer-kernel/hardware.config`
   - Verify: `! grep FB_SIMPLE tools/installer-kernel/*.config gokrazy/kernel/kernel.config gokrazy/kernel/runtime.config`

5. **Phase: Remove old runtime kernel build** -- Delete gokrazy-kernel-build.sh. Update mk/gokrazy.mk KVER to 7.0.11.
   - Files: delete `scripts/dev/gokrazy-kernel-build.sh`, `mk/gokrazy.mk`
   - Verify: no references to gokr-rebuild-kernel in codebase

6. **Phase: Documentation** -- Update docs. Create functional test.
   - Files: `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `docs/functional-tests.md`, `test/install/kernel-compose.ci`

7. **Full verification** -> `make ze-verify`
8. **Complete spec** -> Audit tables, learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Runtime fragments compose to equivalent of current addendum minus dead options |
| Correctness | CONFIG_FB_SIMPLE replaced with DRM_SIMPLEDRM + X86_SYSFB in both runtime and installer |
| Correctness | Cache invalidation hashes correct files from new paths |
| Data flow | make ze-kernel -> gokrazy/kernel/Makefile -> tools/kernel-builder/build.sh -> modcache |
| Single builder | tools/kernel-builder/build.sh is the ONLY build script. No duplicated logic. |
| Profile dispatch | build.sh accepts runtime profile without exit 2. Installer profiles keep current verification. Runtime profile verifies the mandatory module and L2TP/PPP options after olddefconfig. |
| Artifact naming | Runtime: vmlinuz + modules. Installer: Image. |
| Standalone | make -C tools/installer-kernel and make -C gokrazy/kernel both work independently |
| QEMU | qemu-build.py works with both source dirs via parameterized flags |
| Module cleanup | build.sh removes lib/modules/*/build and */source symlinks before copy (matches old gokrazy-kernel-build.sh behavior) |
| DTB copy | arm64 runtime build copies *.dtb and overlays/ to OUT_DIR (matches old gokrazy-kernel-build.sh behavior) |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Shared builder exists | `ls tools/kernel-builder/build.sh tools/kernel-builder/qemu-build.py tools/kernel-builder/Dockerfile` |
| Runtime config fragments exist | `ls gokrazy/kernel/kernel.config gokrazy/kernel/runtime.config` |
| Runtime Makefile exists | `ls gokrazy/kernel/Makefile` |
| Old fragments deleted | `! test -f gokrazy/kernel/l2tp.config.addendum.txt && ! test -f gokrazy/kernel/qemu-evidence.config.addendum.txt` |
| Old shell script removed | `! test -f scripts/dev/gokrazy-kernel-build.sh` |
| Old builder files moved from installer | `! test -f tools/installer-kernel/build.sh && ! test -f tools/installer-kernel/qemu-build.py && ! test -f tools/installer-kernel/Dockerfile` |
| Wiring test exists | `test -f test/install/kernel-wiring.ci` |
| KVER is 7.0.11 | `grep 'KVER.*7.0.11' mk/gokrazy.mk` |
| nct6683 patch tracked | `test -f gokrazy/kernel/patches/0001-nct6683.patch` (or documented removal evidence in Mistake Log) |
| No dead config options | `! grep -E 'NF_NAT_IPV4\|NF_NAT_MASQUERADE_IPV4\|NFT_MASQ_IPV4\|NFT_CHAIN_NAT_IPV4\|NFT_CHAIN_ROUTE_IPV4\|NFT_CHAIN_ROUTE_IPV6\|USB_DEVICEFS' gokrazy/kernel/kernel.config gokrazy/kernel/runtime.config tools/installer-kernel/*.config` |
| No CONFIG_FB_SIMPLE | `! grep FB_SIMPLE gokrazy/kernel/kernel.config gokrazy/kernel/runtime.config tools/installer-kernel/*.config` |
| Installer standalone works | `make -C tools/installer-kernel PROFILE=qemu ARCH=arm64 BUILDER=docker` (manual) |
| Runtime standalone works | `make -C gokrazy/kernel BUILDER=docker ARCH=amd64` (manual) |
| Runtime QEMU standalone works | `make -C gokrazy/kernel BUILDER=qemu ARCH=arm64` (manual or dry-run for wiring) |
| Tests pass | `go test ./internal/appliance/...` |
| Functional test exists | `test -f test/install/kernel-compose.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Docker command injection | version/arch/profile validated before exec.Command |
| Path traversal | SRC_DIR, PATCHES_DIR resolved relative to known directories |
| Tarball URL | KVER validated (digits and dots only) before kernel.org URL |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source -> RESEARCH |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Shared builder at tools/kernel-builder/ | (A) Keep gokr-rebuild-kernel, (B) Mirrored scripts in gokrazy/kernel/ and tools/installer-kernel/, (C) Ze-owned Go package | Single source of truth for build logic. build.sh, qemu-build.py, Dockerfile exist once. Both runtime and installer Makefiles delegate to them. No duplication. |
| Standalone Makefile at gokrazy/kernel/ for runtime | (A) ze-setup appliance kernel --profile runtime, (B) Go binary wrapper | Makefile is consistent with installer pattern (make -C tools/installer-kernel). No Go build dependency for kernel builds. make ze-kernel stays in Makefile land. |
| Extend build.sh with MODULES and PATCHES_DIR | (A) Separate runtime build.sh, (B) Pass all options as env vars | Single script, two modes. MODULES=yes adds make modules + modules_install + vmlinuz copy. PATCHES_DIR applies patches. Installer uses defaults (MODULES=no, no patches). |
| Parameterize qemu-build.py | (A) Duplicate for runtime, (B) Keep installer-only | Single script. --src-dir, --out-dir, --builder-dir, --modules, and --patches-dir replace hardcoded installer paths. Both Makefiles can use it. |
| CONFIG_FB_SIMPLE replacement: DRM_SIMPLEDRM=y + X86_SYSFB=y | (A) efifb only, (B) Remove both and accept no HDMI | simpledrm is the upstream replacement for simplefb. Earlier 6.x issues ("logo never disappears") may be resolved in 7.0. Keep efifb options too as fallback. |
| nct6683 patch: required tracked patch, conditional removal with evidence | (A) Always include, (B) Always remove | Upstream may have added the customer ID. Implementer checks 7.0 source; if covered, documents evidence and removes patch. If not, rebases and tracks. |

## Known Limitations

- Pinned kernel backup (.ze-pinned-kernel) retains 6.19.11 until rebuilt
- CONFIG_DRM_SIMPLEDRM + CONFIG_X86_SYSFB replacing CONFIG_FB_SIMPLE needs hardware testing on 7.0
- Kernel 7.0.11 is the target ceiling version

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Installer build.sh/qemu-build.py were placeholders | Both fully implemented (125 + 528 lines) | Review found files at tools/installer-kernel/ | Spec reuses installer pattern |
| gokr-rebuild-kernel importable as Go library | package main, not importable | Go constraint | Shared builder scripts instead |
| ze appliance kernel available in bin/ze | Only in bin/ze-setup (ze_setup tag) | Checked setup_features_setup.go | Standalone Makefile path |
| cmd_kernel_test.go had 22 tests | Has 25 Test* functions | grep -c | Updated spec |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Import gokr-rebuild-kernel | package main | Shared builder scripts |
| ze-setup as runtime entry point | ze_setup build tag not in bin/ze | Standalone Makefile |
| Mirrored scripts | Two build systems, not one | Shared tools/kernel-builder/ |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The installer's build pattern (defconfig + merge_config.sh + verification) is strictly better than gokr-rebuild-kernel's (defconfig + mod2noconfig + append)
- Factoring builder scripts into a shared directory achieves "single build system" while keeping config fragments separate per use case
- Standalone Makefiles are the right entry point for kernel builds: no Go dependency, consistent with existing installer pattern, works on any dev machine

## Implementation Summary

### What Was Implemented

- Extracted the shared kernel builder into `tools/kernel-builder/` with one `build.sh`, one `qemu-build.py`, and one `Dockerfile`
- Replaced runtime `gokr-rebuild-kernel` usage with `gokrazy/kernel/Makefile` plus `mk/gokrazy.mk` overlay/restore wiring
- Split runtime config into tracked `gokrazy/kernel/kernel.config` + `runtime.config` fragments and tracked the `nct6683` patch series under `gokrazy/kernel/patches/`
- Repointed installer kernel builds and `ze appliance kernel` to the shared builder while preserving cache/download/build fallback and explicit Docker/QEMU selection
- Added installer/runtime kernel functional tests for explicit builders, auto builder selection, fragment composition, rebuild dependencies, package prerequisites, and default `make ze-kernel`

### Bugs Found/Fixed

- Shared builder was initially missing `zstd` and `kmod` coverage for runtime `CONFIG_KERNEL_ZSTD=y` plus `modules_install`; fixed in the Dockerfile and QEMU package list and pinned with `kernel-builder-packages.ci`
- Default builder behavior was only covered for forced builder flags; fixed by adding `appliance-kernel-auto-docker.ci`, `appliance-kernel-auto-qemu.ci`, and making `ze-kernel-overlay.ci` exercise the default path

### Documentation Updates

- Updated `docs/guide/appliance.md` for runtime `make ze-kernel` Docker/QEMU usage and installer builder fallback behavior
- Updated `docs/guide/ze-install.md` for explicit installer Docker/QEMU build commands and shared-builder notes

### Deviations from Plan

- `docs/functional-tests.md` ended up unchanged on disk because the relevant kernel test inventory text already matched the final state before commit preparation
- `ai/CODE-TO-DOCS.md` was left out of this commit because its remaining drift fixes are not required to explain or use the kernel builder changes

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Converge runtime and installer kernels on one build system | Done | `tools/kernel-builder/`, `mk/gokrazy.mk`, `tools/installer-kernel/Makefile` | Shared builder extracted and both entry points delegate to it |
| Move both kernels to Linux 7.0.11 | Done | `mk/gokrazy.mk`, `gokrazy/kernel/Makefile`, `tools/kernel-builder/build.sh` | Runtime and installer now default to 7.0.11 |
| Replace `gokr-rebuild-kernel` and `gokrazy-kernel-build.sh` | Done | `mk/gokrazy.mk`, deleted `scripts/dev/gokrazy-kernel-build.sh` | Runtime Makefile now owns the build |
| Keep runtime and installer differing only by fragments and outputs | Done | `gokrazy/kernel/*.config`, `tools/installer-kernel/*.config`, `tools/kernel-builder/build.sh` | Shared builder mode switches on fragments plus `MODULES`/patches |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/install/ze-kernel-overlay.ci`, `mk/gokrazy.mk`, `gokrazy/kernel/Makefile` | Default `make ze-kernel` now uses the shared runtime builder |
| AC-2 | Done | `internal/appliance/cmd_kernel_test.go`, `test/install/appliance-kernel-auto-docker.ci`, `test/install/appliance-kernel-auto-qemu.ci` | Cache/download/build builder selection preserved |
| AC-3 | Done | `test/install/appliance-kernel-qemu.ci`, `tools/installer-kernel/hardware.config` | Hardware installer profile still builds through the shared path |
| AC-4 | Done | `test/install/kernel-compose.ci`, `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.config` | Runtime fragments are tracked and checked directly |
| AC-5 | Done | `tools/kernel-builder/build.sh`, `tools/installer-kernel/Makefile` | Installer keeps `Image` output and no modules copy path |
| AC-6 | Done | `tools/kernel-builder/build.sh`, `test/install/ze-kernel-overlay.ci` | Runtime produces `vmlinuz`, modules, DTBs, and overlays |
| AC-7 | Done | `test/install/kernel-compose.ci` | Removed dead nftables and USB config symbols from tracked fragments |
| AC-8 | Done | `gokrazy/kernel/kernel.config`, `tools/installer-kernel/hardware.config`, `test/install/kernel-compose.ci` | `simpledrm` + `sysfb` replace `FB_SIMPLE` |
| AC-9 | Done | `mk/gokrazy.mk`, deleted `scripts/dev/gokrazy-kernel-build.sh` | Runtime no longer shells out through the old wrapper |
| AC-10 | Done | `internal/appliance/cmd_kernel.go`, `test/install/appliance-kernel-auto-docker.ci`, `test/install/appliance-kernel-auto-qemu.ci` | Three-tier installer resolution preserved |
| AC-11 | Done | `internal/appliance/cmd_kernel_test.go` | Kernel unit tests updated for new paths and builder args |
| AC-12 | Done | `gokrazy/kernel/patches/0001-nct6683.patch`, `gokrazy/kernel/patches/series` | Patch is tracked in the runtime tree |
| AC-13 | Done | `internal/appliance/cache.go`, `internal/appliance/cmd_kernel_test.go` | Cache invalidation now hashes installer fragments plus shared `build.sh` |
| AC-14 | Done | `tools/installer-kernel/Makefile`, `test/install/kernel-wiring.ci` | Standalone installer Makefile delegates to `../kernel-builder/` |
| AC-15 | Done | `tools/kernel-builder/qemu-build.py`, `test/install/kernel-wiring.ci`, `test/install/kernel-qemu-arch-alias.ci` | Shared builder works for Docker and QEMU with explicit path flags |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDockerBuildMountsSharedBuilder` | Done | `internal/appliance/cmd_kernel_test.go` | Verifies installer Docker argv/mount construction |
| `TestQEMUBuildPassesBuilderDir` | Done | `internal/appliance/cmd_kernel_test.go` | Verifies installer QEMU argv/path construction |
| `appliance-kernel-docker.ci` | Done | `test/install/appliance-kernel-docker.ci` | Explicit installer Docker path |
| `appliance-kernel-qemu.ci` | Done | `test/install/appliance-kernel-qemu.ci` | Explicit installer QEMU path |
| `appliance-kernel-auto-docker.ci` | Done | `test/install/appliance-kernel-auto-docker.ci` | Installer default builder prefers Docker |
| `appliance-kernel-auto-qemu.ci` | Done | `test/install/appliance-kernel-auto-qemu.ci` | Installer default builder falls back to QEMU |
| `kernel-builder-packages.ci` | Done | `test/install/kernel-builder-packages.ci` | Builder package prerequisites for runtime compression/module install |
| `kernel-compose.ci` | Done | `test/install/kernel-compose.ci` | Runtime fragment contract |
| `kernel-qemu-arch-alias.ci` | Done | `test/install/kernel-qemu-arch-alias.ci` | `aarch64` normalizes to `arm64` |
| `kernel-runtime-deps.ci` | Done | `test/install/kernel-runtime-deps.ci` | Runtime Makefile rebuild dependencies |
| `kernel-wiring.ci` | Done | `test/install/kernel-wiring.ci` | Shared builder delegation for runtime and installer |
| `ze-kernel-overlay.ci` | Done | `test/install/ze-kernel-overlay.ci` | Default `make ze-kernel` overlays and restore flow |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `tools/kernel-builder/{build.sh,qemu-build.py,Dockerfile}` | Done | Created |
| `gokrazy/kernel/{Makefile,kernel.config,runtime.config}` | Done | Created |
| `gokrazy/kernel/patches/{series,0001-nct6683.patch}` | Done | Created |
| `mk/gokrazy.mk` | Done | Updated runtime entry point |
| `internal/appliance/{cache.go,cmd_kernel.go,cmd_kernel_test.go,doctor_checks.go}` | Done | Updated installer builder path, cache hashing, tests, doctor |
| `tools/installer-kernel/{Makefile,README.md,hardware.config}` | Done | Updated |
| `tools/installer-kernel/{Dockerfile,build.sh,qemu-build.py}` | Done | Removed after extraction |
| `gokrazy/kernel/{l2tp.config.addendum.txt,qemu-evidence.config.addendum.txt}` | Done | Removed after fragment split |
| `docs/guide/{appliance.md,ze-install.md}` | Done | Updated |
| `test/install/*kernel*.ci` | Done | Added kernel functional coverage |

### Audit Summary
- **Total items:** 31
- **Done:** 31
- **Partial:** 0
- **Skipped:** 0
- **Changed:** Runtime builder path, installer builder path, tracked runtime fragments, kernel docs, kernel tests

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Both kernels on 7.0.11 | source + tests | `mk/gokrazy.mk`, `gokrazy/kernel/Makefile`, and `tools/kernel-builder/build.sh` all pin 7.0.11; kernel install tests cover both runtime and installer entry points |
| Single build system | file inventory | Shared builder lives only under `tools/kernel-builder/`; old runtime wrapper and installer-local builder copies are removed |
| Fragment composition | functional test | `test/install/kernel-compose.ci` verifies tracked runtime fragments and removed symbols |
| No feature regression | functional test | `test/install/ze-kernel-overlay.ci` validates runtime artifact overlay/restore; installer path tests validate cache + output writes |
| QEMU backend works | functional test | `test/install/appliance-kernel-qemu.ci`, `test/install/appliance-kernel-auto-qemu.ci`, `test/install/kernel-qemu-arch-alias.ci` |
| Installer standalone | functional test | `test/install/kernel-wiring.ci`, `test/install/appliance-kernel-docker.ci`, `test/install/appliance-kernel-qemu.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | Shared runtime builder missing `zstd`/`kmod` host tools | `tools/kernel-builder/Dockerfile`, `tools/kernel-builder/qemu-build.py` | Added required packages and pinned with `kernel-builder-packages.ci` |

### Fixes applied

- Added `zstd` to the Docker builder image
- Added `kmod` and `zstd` to the QEMU backend package list
- Added functional coverage for default builder selection and package prerequisites

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Web `/config/form/` browser flow still needs daemon-level coverage | delegated web work outside this kernel commit | Not part of the kernel/gokrazy commit scope |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")
Kernel/gokrazy scope is clean for this commit; the remaining NOTE is delegated web work outside the staged file set.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `tools/kernel-builder/build.sh` | Yes | present in working tree |
| `tools/kernel-builder/qemu-build.py` | Yes | present in working tree |
| `tools/kernel-builder/Dockerfile` | Yes | present in working tree |
| `gokrazy/kernel/Makefile` | Yes | present in working tree |
| `gokrazy/kernel/kernel.config` | Yes | present in working tree |
| `gokrazy/kernel/runtime.config` | Yes | present in working tree |
| `gokrazy/kernel/patches/series` | Yes | present in working tree |
| `gokrazy/kernel/patches/0001-nct6683.patch` | Yes | present in working tree |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Runtime goes through shared builder | `test/install/ze-kernel-overlay.ci` plus `mk/gokrazy.mk` -> `gokrazy/kernel/Makefile` |
| AC-2 | Installer cache/download/build fallback preserved | `internal/appliance/cmd_kernel.go` and `appliance-kernel-auto-{docker,qemu}.ci` |
| AC-4 | Runtime fragments tracked and composed | `test/install/kernel-compose.ci` |
| AC-6 | Runtime modules/DTBs emitted | `tools/kernel-builder/build.sh`, `test/install/ze-kernel-overlay.ci` |
| AC-10 | Explicit/implicit builder selection preserved | `internal/appliance/cmd_kernel.go`, `appliance-kernel-{docker,qemu}.ci`, `appliance-kernel-auto-{docker,qemu}.ci` |
| AC-14 | Installer Makefile still works standalone | `test/install/kernel-wiring.ci` |
| AC-15 | Docker and QEMU both use shared builder | `test/install/kernel-wiring.ci`, `test/install/kernel-qemu-arch-alias.ci` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze appliance kernel` default Docker path | `test/install/appliance-kernel-auto-docker.ci` | Yes |
| `ze appliance kernel` default QEMU fallback | `test/install/appliance-kernel-auto-qemu.ci` | Yes |
| `ze appliance kernel --builder docker` | `test/install/appliance-kernel-docker.ci` | Yes |
| `ze appliance kernel --builder qemu` | `test/install/appliance-kernel-qemu.ci` | Yes |
| `make ze-kernel` | `test/install/ze-kernel-overlay.ci` | Yes |
| Runtime/installer Makefiles | `test/install/kernel-wiring.ci` | Yes |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Runtime `make ze-kernel` Docker/QEMU usage | `docs/guide/appliance.md`, source anchors to `mk/gokrazy.mk` and `gokrazy/kernel/Makefile` | Yes |
| Installer `ze appliance kernel` builder usage | `docs/guide/appliance.md`, `docs/guide/ze-install.md`, source anchors to `internal/appliance/cmd_kernel.go` and `tools/kernel-builder/qemu-build.py` | Yes |
| Installer standalone Makefile usage | `docs/guide/ze-install.md`, `tools/installer-kernel/README.md`, anchor to `tools/installer-kernel/Makefile` | Yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>`
