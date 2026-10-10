# Spec: appliance-ships-ze-kernel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision (Thomas, 2026-10-09): the appliance kernel carries MPLS.

-> Decision (owner, 2026-10-09): "we have our own kernel we should ONLY use it NOTHING ELSE". Every image path (`ze appliance build`, `./le build gokrazy`, the deployment proofs, the QEMU labs) boots ze's own runtime kernel and no other. `github.com/rtr7/kernel` leaves the tree (go.mod, vendor, builddir, deployment skeletons). The `ze.gok.kernel-package` override is DELETED, not kept: a route that lets an image carry a different kernel is the thing this decision forbids. AC-6 and the "`ze.gok.kernel-package` kept" design row are superseded by this decision and must be rewritten to it. Found the same day: crash capture (`plan/spec-crash-capture.md`) cannot work on rtr7 (no `CONFIG_PSTORE`) while the build accepted `image.crash-dump`, because every kernel floor check judges ze's kernel, not the shipped one; with ze's kernel the only kernel, the checks judge what ships.

Today `ze appliance build` ships the pinned stock `github.com/rtr7/kernel`
(`v0.0.0-20260403073601-5a996da3a37b`, Linux 6.19.11), whose embedded config
reads `# CONFIG_MPLS is not set`. Ze already builds its own runtime kernel
(Linux 7.2 plus the tracked patch series) from `gokrazy/kernel/runtime.config`,
verified against `gokrazy/kernel/runtime.require`, which carries MPLS, L2TP,
nftables inet, TCP-MD5 and the rest of the 59-symbol floor. That kernel reaches
an image only through the explicit `ze.gok.kernel-package` route of
`./le build gokrazy` and the L2TP deployment proof. The default image therefore
contradicts `docs/guide/appliance.md` "Runtime Kernel Requirements", and an
appliance whose seed uses MPLS cannot start once the kernel capability gate
(`plan/immediate/spec-kernel-capability-gate.md`, AC-15) refuses it.

Goal: the default `ze appliance build` image boots ze's own runtime kernel,
built from the tracked `runtime.config` at the same commit, and the rtr7 kernel
leaves the build entirely. `plan/immediate/spec-kernel-capability-gate.md`
AC-15 depends on this spec.

### Owner decisions, second round (2026-10-09)

-> Decision (owner, 2026-10-10, recorded as D-5 in `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md`): "docker needs our kernel". Ze's Docker hosts run exactly this appliance kernel, so the options Docker itself needs (moby `contrib/check-config.sh`, "Generally Necessary", plus overlayfs, user namespaces and seccomp) join `runtime.config` and `runtime.require`, declared once. There is no separate docker-host profile. That spec's AC-14 owns the edit; this spec's build and cache carry it unchanged.
-> Decision (owner): kernel route (b) confirmed: "we want to control the modules so we need to compile our own kernel". No published kernel module (option a) and no download tier (option c).
-> Decision (owner): every kernel is built on a host of its own arch, with no CPU emulation: arm64 on the Mac, amd64 on the Linux host. Found at source: today the Docker backend passes `--platform linux/<arch>` (`internal/appliance/kernelbuilder/driver.go`, `dockerPlatforms`), which runs a foreign arch under emulation, and the QEMU backend falls back to `-accel tcg` when neither `hvf` nor `kvm` serves the target binary (`internal/appliance/kernelbuilder/qemu.go`, accelerator selection). Both routes are refused by AC-13.
-> Decision (owner): a long cold first build is acceptable IF the result is cached and the cache is never deleted when space is reclaimed. Found at source, what removes kernel build output today:

| Remover | What it touches | Reaches the runtime kernel cache? |
|---------|-----------------|------------------------------------|
| `evictKeepN` (`internal/appliance/cache.go`), run after every runtime build (`cmd_kernel.go`, `resolveRuntimeKernel`) and by `ze appliance kernel --evict-cache` | keeps the `evictKeepDefault` = 2 most recently modified entries of `~/.cache/ze/runtime-kernel/`, by mtime, ACROSS arches | Yes: two arm64 rebuilds evict the amd64 entry, and the reverse |
| `./le scratch cache-clean` (`internal/le/scratch/cacheclean.go`, `cleanTargets`) | the checkout, shared, ambient and bootstrap Go caches and the lint cache | No today; nothing asserts it stays so |
| store trim (`internal/le/scratch/storetrim.go`, `trimStores`, `livenessStores`) | Go cache entries by budget, dead session dirs and testbin sets under `tmp/` | No today; nothing asserts it |
| every resolver call | rewrites `tmp/kernel/build` (`runtimeKernelOutputDir`); an output dir, not a cache | n/a; redirected by `ZE_KERNEL_TEST_OUTPUT_DIR` |

-> Open (owner): the GPLv2 source offer for the shipped kernel. The owner asked what it is and is having it explained; this spec does not decide it (R-7).
-> Decision (owner, 2026-10-09, closes the open GPLv2 question above and R-7): Ze complies by pointing users to the published Linux source. Each appliance image carries a notice that it contains Linux under GPLv2, naming the exact kernel.org version and the source tarball URL with its SHA-256; the kernel config, the patch series (`gokrazy/kernel/patches/`) and the build scripts are in the public Ze repository. The kernel source is not re-published. The notice, version, URL and checksum derive from the same declaration the build uses (`internal/appliance/kernel.version` and the tarball SHA-256 pin of AC-8/AC-9), so the notice cannot drift from the kernel built (AC-17).
-> Owner 2026-10-09: no AGPLv3 notice in the image; Ze is the owner's own code, published on GitHub.
-> Decision (owner, 2026-10-09): "we should use the latest stable 7.x kernel". Ze pins the latest stable 7.x release by EXACT version plus SHA-256, because the GPLv2 notice and the cache key both need an exact version; a later bump is a deliberate change to that one declaration. Found 2026-10-09: `internal/appliance/kernel.version` pins `7.2` (the 7.2.0 tarball, `linux-7.2.tar.xz`, SHA-256 `f9fef3d14c0df53819026f4be74459835c2a0b0dcbf5b5bbd9ea19f0829402b3`); kernel.org `releases.json` names latest stable `7.2.9` (released 2026-10-03; mainline is 7.3-rc6), `linux-7.2.9.tar.xz` SHA-256 `b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba` from `https://cdn.kernel.org/pub/linux/kernel/v7.x/sha256sums.asc`. The pin is behind, so the bump to 7.2.9 is part of AC-1 (AC-19), and the first rebuild happens there. This also validates A-5: kernel.org publishes `sha256sums.asc` for 7.x.
-> Decision (owner): "ignore n100 atm". N100 (amd64) is a supported BUILD target; booting on N100 hardware is owner-deferred until the owner runs real hardware again. A-3's hardware half is therefore not evidence this spec owes; its QEMU half stands.
-> Related: `plan/immediate/spec-appliance-kernel-vpn-modules.md` is a separate spec by owner decision; its AC-6/AC-7 (default-image claim) depend on this one.
-> Implemented (2026-10-10, owner order to land the bump first): AC-19 and the worker half of AC-8/AC-9. `kernel.version` reads 7.2.9; `kernelSourceSHA256` (`internal/appliance/kernelbuilder/worker.go`) holds `b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba`, which `sha256sum` of the downloaded `linux-7.2.9.tar.xz` matched; `verifyKernelSource` refuses any other digest before extraction (`TestDownloadKernelSourceVerifiesDigest`), and `TestKernelVersionHasDigestPin` sits in `internal/appliance/cmd_kernel_test.go`. The series now applies with `patch --fuzz=0`: on 7.2.9, 0001 hunk 3 needed fuzz and 0002 failed two hunks (`ip6_route.h` gained an `IP6_MAX_MTU` clamp at `out:`; `ip_do_fragment` gained upstream the same `mtu < hlen + 8` guard 0002 carried, so that hunk is dropped). Both patches were regenerated against pristine 7.2.9 and apply with no fuzz and no offset. The GPLv2 notice (AC-17) and the bump runbook edit remain this spec's.

-> Implemented (2026-10-10, phases 1, 2 and 4 code half): rtr7 and `ze.gok.kernel-package` deleted; `gokrazy/ze/config.json` names `ze.invalid/kernel`; `ze appliance build` (`resolveBuildParentDir`) and `./le build gokrazy` (`prepareArgs`) resolve `appliance.RuntimeKernelTree(arch)` and `instance.Prepare` assembles the package from that cache entry inside the prepared parent and replaces the module. AC-4, AC-5, AC-6 (rewritten), AC-7, AC-10 carry unit tests; AC-1/2/3/11 still owe the boot proofs; AC-13, AC-17, the doctor check and the provenance digest are not started.
-> Decision (implementation): the kernel module path is `ze.invalid/kernel`: RFC 2606 reserves `.invalid`, so a prepared instance whose replace went missing fails `go list` instead of fetching (R-3). The assembler lives in `internal/appliance/instance/kernelpkg.go`, not `internal/appliance/kernelpkg.go`, because `Prepare` assembles inside the prepared parent (R-6) and `instance` cannot import `appliance`; it writes the go.mod and Go file itself, so no tracked `gokrazy/kernel/package/` skeleton exists. `TestNoRtr7KernelReference` excludes all of `plan/` (specs describe the removal), `vendor/` and `gokrazy/modcache/`. `./le setup install` skips the kernel module, which has nothing to download.
-> Evidence (2026-10-10, arm64 on the Mac under HVF; AC-1, AC-2, AC-3 and AC-11 for arm64; the amd64 half is owed on the Linux host): `./le test qemu mpls-boot-test` (`ze_vpp_hp_aarch64_bios=/opt/homebrew/share/qemu/edk2-aarch64-code.fd`; the default firmware path is Linux's) built the arm64 appliance with the MPLS seed from the cached runtime kernel, so gokrazy's arch check passed (AC-11), and booted it: console `Linux version 7.2.9-ze ... #1 SMP PREEMPT Sat Oct 10 09:18:07 UTC 2026`, ze started with `fib-kernel: running` and LDP loaded. The proof then FAILED with no SSH answer: the harness gave the arm64 guest an e1000 NIC, which the arm64 kernel builds as a module (`CONFIG_E1000=m`), and gokrazy loads no modules. Fixed in the harness (`Hugepages.qemuArgs`, `internal/le/test/qemu/boot.go`: arm64 takes `virtio-net-pci`, which `runtime.require` makes built in; `TestTheArm64MachineGetsTheNICItsKernelBuildsIn`, red first). The kept image booted with that NIC answered over SSH: `show host kernel | json` gives `"release": "7.2.9-ze"`, `"architecture": "arm64"` (equals `internal/appliance/kernel.version`, AC-2); `show ldp neighbor | json` answers `[]`, JSON from the LDP engine (AC-3); `show doctor | json` answers (a no-NTP error and listener warnings only, no kernel-capability refusal). The harness itself was not re-run after the NIC fix, because its fresh host build hashes the kernel builder sources AC-13 changed and would start the cold arm64 rebuild. AC-1/AC-2 config: every one of the 128 symbols in `kernel.require` plus `runtime.require` reads `=y` in the packaged config of cache entry `7.2.9-runtime-arm64-runtime-7c0d8f79-bf3ac1c9`, including `CONFIG_MPLS_ROUTING=y` and `CONFIG_MPLS_IPTUNNEL=y`; `CONFIG_LOCALVERSION="-ze"`, `CONFIG_IKCONFIG_PROC=y`.
-> Implemented (2026-10-10): AC-13 in 179ca326f2. `validateRequest` refuses a kernel for an arch other than `hostGOARCH`; `qemuArgs` refuses a QEMU with neither hvf nor kvm, with no tcg fallback. This changes the runtime kernel cache key, so the arm64 cache entry above is stale and a cold rebuild (about 30 minutes) is owed before the next arm64 image build.
-> Implemented (2026-10-10): the provenance source digest in 33d6ff0a4e. `writeProvenance` adds `source-url` (`kernelTarballURL`, what `downloadKernelSource` fetches) and `source-sha256` (`SourceDigest`, what `verifyKernelSource` checks), and refuses a version with no tracked digest. AC-17 next: `kernelbuilder.ReadProvenance` and `Provenance.LinuxNotice` (`provenance.go`); `instance.Prepare` reads the kernel tree's provenance and `addLinuxNotice` puts the notice at `/etc/linux-gpl-notice` through the ze package's `ExtraFileContents` (gokrazy copies only `lib/modules` from a kernel package). `TestLinuxNoticeMatchesTheBuiltKernel` builds the notice from the provenance `writeProvenance` wrote and asserts version, URL and digest; `TestPrepareCarriesTheLinuxNotice` and `TestPrepareRefusesAKernelWithoutProvenance` went red with the `addLinuxNotice` call removed. A cache entry built before 33d6ff0a4e has no source lines and is refused, so every existing cache entry needs the rebuild. Found: `kernel-builder-single-driver` and `kernel-wiring` fixtures are red on an arm64 host since 179ca326f2 (their amd64 requests meet the AC-13 refusal).
-> Evidence (2026-10-10, arm64 on the Mac under HVF, after the cold rebuild; AC-1, AC-3 and AC-11 for arm64, with the NIC fix and AC-13 in the tree): the cold rebuild wrote cache entry `7.2.9-runtime-arm64-runtime-7c0d8f79-d826e25e`, whose `kernel.version` carries `source-url=https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.2.9.tar.xz` and `source-sha256=b4c5dfbe...d8ba`; the old entry `...-bf3ac1c9` (no source lines) has a different key and nothing selected it. The first image build after the rebuild FAILED in gok: le exports `GOTOOLCHAIN=go1.27.0` (go.mod's go directive, for its linter), the host go is 1.27.1, and with `GOMODCACHE=gokrazy/modcache` and `GOPROXY=off` every go subprocess answered `toolchain not available`; gok reported its failed `go list` as `go get ze.invalid/kernel`. The replace was in place (the failed builddir's go.mod carries it, and `go list ze.invalid/kernel` resolves under `GOTOOLCHAIN=local`). Fixed in 0313f16ee7: `appliance.SetGokGoEnv` sets `GOTOOLCHAIN=local` for both gok entry points (`TestGokBuildsWithTheHostToolchain`, red first). Re-run: `ze_vpp_hp_aarch64_bios=/opt/homebrew/share/qemu/edk2-aarch64-code.fd ./le --name zek test qemu mpls-boot-test` answered `APPLIANCE-MPLS-QEMU: PASS ze started with MPLS in use; \`show ldp neighbor | json\` answered`, exit 0 (log `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/mpls-boot-arm64-run2.log`). The arm64 fixture red is fixed in ecd9a892ba (`kernel-builder-single-driver`, `kernel-wiring` use `runtime.GOARCH`). `/etc/linux-gpl-notice` was not read back from the booted image.
-> Implemented (2026-10-10): the doctor check. `ze doctor` registers `appliance-runtime-kernel` (code `doctor-appliance-runtime-kernel`, a warning; `checkRuntimeKernel` in `internal/appliance/doctor_checks.go`). It asks about the host arch, because AC-13 builds the kernel only there: it passes when the cache entry `resolveRuntimeKernel` would serve holds its `vmlinuz`, else when `kernelbuilder.UsableBuilder` finds Docker, or QEMU with Go. With neither, it names the cache entry, why `ze appliance build` needs it, and `ze appliance kernel --target runtime --arch <arch>`. Where the kernel config cannot be read (outside the source tree) it warns rather than pass. Unit tests `TestDoctorRuntimeKernelCached`, `TestDoctorRuntimeKernelBuildable`, `TestDoctorRuntimeKernelMissing`, `TestDoctorRuntimeKernelOutsideSourceTree` (`internal/appliance/doctor_checks_test.go`): Missing and OutsideSourceTree went red against a stub returning nil; Cached and Buildable went red under mutations removing the cache test and the builder test. Functional `test/ui/doctor-appliance-runtime-kernel.ci` runs `ze doctor --json` outside the tree and asserts the code and the message; red with the unreadable-config branch mutated to return nil, green restored. Page: `docs/architecture/appliance/build-artifacts.md`.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-13 | `ze appliance kernel` (or a build that resolves it) for an arch other than the host's, with either backend | refuses before building, naming the host arch, the requested arch and that it is built on a host of that arch; never runs Docker with a foreign `--platform` nor QEMU under `tcg` |
| AC-14 | `./le scratch cache-clean` and store trim run with a populated `~/.cache/ze/runtime-kernel/` | every entry survives; a unit test asserts no clean or trim target resolves inside a kernel cache namespace |
| AC-15 | runtime-kernel cache holding an arm64 and an amd64 entry; two further arm64 builds of new variants | the amd64 entry survives, and the entry the current tree's variant names is never evicted for either arch (eviction keeps the newest per arch) |
| AC-16 | `docs/contributing/running-commands.md` "When the disk is full" | names the kernel cache, states it is not a reclamation target and why (a cold rebuild is about 30 minutes) |
| AC-17 | any `ze appliance build` image | carries a GPLv2 notice naming Linux, the exact kernel.org version, the tarball URL and its SHA-256, all derived from `kernel.version` and the tracked digest pin the worker verifies; a test builds the notice and asserts its version equals the built kernel's `kernel.version` provenance and its URL and digest equal the ones `downloadKernelSource` used |
| AC-19 | `internal/appliance/kernel.version` and its digest pin, at AC-1 | pin the exact latest stable 7.x found when AC-1 is implemented (7.2.9 on 2026-10-09, or later if kernel.org has moved), with the SHA-256 from kernel.org's `sha256sums.asc`; the patch series applies cleanly to it; `TestKernelVersionHasDigestPin` (AC-9) covers the pair |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/appliance.md` - "Runtime Kernel Requirements", "L2TP Kernel Support", "Repo layout", "Kernel selection is explicit"
  → Constraint: the guide already states the runtime kernel is pinned by `kernel.require` + `runtime.require`; the default image must make that sentence true, not reword it.
  → Decision: the guide's sentence "`ze appliance build` keeps the pinned `github.com/rtr7/kernel`" is deleted, together with the "the pinned upstream gokrazy kernel is not assumed to provide these options" caveats in the L2TP section.
- [ ] `docs/architecture/appliance/gokrazy-build-pins.md` - builddir modules, modcache defects, boot proofs, GPLv2 source offer
  → Constraint: a builddir module that `gok` cannot resolve locally makes it `go get` from upstream (the "modcache defect" table); the ze kernel module must resolve only through a filesystem replace, and a missing replace target must fail the build, never fetch.
  → Decision: the "eight builddir modules" become seven (rtr7 removed, ze kernel module added only as a prepared replace, see Files); `TestPrepareRealInstanceCarriesEveryModule` and `TestPreparedModulesResolveIdenticallyToTracked` follow.
  → Constraint: the GPLv2 section names the shipped kernel; it must name ze's kernel and its source (kernel.org tarball, `gokrazy/kernel/patches/`, `runtime.config`). The source-offer sign-off stays an owner licensing decision.
- [ ] `docs/architecture/appliance/kernel-profiles.md` - registry, Go-owned guarantee
  → Constraint: requirements are enforced over the emitted `build/config` after the build (`enforceKernelRequirements`), plus the compiled floor (`runtimeKernelRequirements` in `internal/appliance/kernelreq.go`); the default image must pass through that same check, never a second one.
- [ ] `docs/architecture/appliance/build-artifacts.md` - three-tier resolution (cache, download, build)
  → Decision: the runtime target today has two tiers only (cache, then native build); `resolveRuntimeKernel` has no download tier and `ze.appliance.kernel.url` serves the installer target only. This spec adds no download tier.
- [ ] `docs/architecture/testing/qemu-integration.md` - declared by `internal/appliance/kernelbuilder/qemu.go`; QEMU guests boot `tmp/kernel/build/vmlinuz` (`runtimeKernelOutputDir`)
  → Constraint: `tmp/kernel/build` is the QEMU suites' kernel input and is rewritten by every resolver call; the image must assemble its package from the arch-keyed CACHE directory, never from `tmp/kernel/build` (R-6). The page's kernel paths stay valid and need no edit unless the resolver's copy-out changes.
- [ ] `docs/architecture/vpp-host-tuning.md` - declared by `internal/appliance/kernelargs.go`; hugepage and CPU-isolation kernel arguments
  → Constraint: `resolveBuildParentDir` keeps appending hugepage, isolation and crash-dump arguments unchanged; kernel resolution is added beside them, so the page's argument contract holds. `HUGETLBFS` is already in `runtime.require`, so the VPP hugepage boot keeps its kernel support (proof: `vpp-hugepages-test`).
- [ ] `ai/rules/platform-linux.md` - appliance dependency bump runbook
  → Constraint: the runbook names `rtr7/kernel` among the seven builddir pins and as a pin to review; it is rewritten in the same change.

**Key insights:**
- gokrazy consumes a kernel as a Go PACKAGE: `packer.PackageDir` runs `go list -mod=mod -tags gokrazy -f {{.Dir}} <KernelPackage>` inside the builddir, then copies `vmlinuz`, `*.dtb`, `overlays/*.dtbo`, `overlays/overlay_map.dtb`, `boot.scr` to the boot partition and `lib/modules` into root; `validateTargetArchMatchesKernel` reads the vmlinuz header and refuses a GOARCH mismatch. The package needs one `.go` file (rtr7 ships `empty.go`, `package kernel`) and a `go.mod`.
- The runtime kernel tree ze already builds (`~/.cache/ze/runtime-kernel/<variant>/`: `vmlinuz` 17 MB, `lib/modules`, `config`, `kernel.version`) has exactly the files gokrazy copies; it lacks only `go.mod` + a `.go` file.
- `internal/le/test/deployment/gokrazyimage.go` `assembleKernelPackage` already turns that tree into a kernel package, by copying the rtr7 module as a skeleton and overwriting vmlinuz, modules, dtbs, overlays. That skeleton dependency is the layering this spec removes.
- rtr7 ships amd64 only, so `image.arch: arm64` cannot pass gokrazy's arch check on the default kernel today; ze's builder already builds arm64.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/ze/config.json` - `"KernelPackage": "github.com/rtr7/kernel"`
- [ ] `gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod` - requires rtr7/kernel at the pinned pseudo-version, `// indirect`; replaces a stale gokrazy version
- [ ] `internal/appliance/instance/prepare.go` - `Prepare` copies the instance and builddir under `tmp/`; `Options.KernelPackage` (explicit path) calls `replaceKernel`, which adds `replace github.com/rtr7/kernel => <path>` in the PREPARED rtr7 module and fails closed if that module is absent. `KernelModule = "github.com/rtr7/kernel"`
- [ ] `internal/appliance/kernelargs.go` - `resolveBuildParentDir`: `ze appliance build` calls `instance.Prepare(filepath.Abs("gokrazy"), Options{ExtraKernelArgs})` with NO kernel package, so the default image is rtr7. The build runs from a ze checkout (relative `gokrazy`)
- [ ] `internal/le/build/gokrazy/gokrazy.go` - `./le build gokrazy` passes `env.Get("ze.gok.kernel-package")` to `Prepare`; env registered with description "(default: the pinned github.com/rtr7/kernel)"
- [ ] `internal/appliance/cmd_kernel.go` - `ze appliance kernel --target runtime --arch <a> [--builder docker|qemu] [--print-cache-dir]`; `resolveRuntimeKernel`: reserve_mem version floor, cache hit at `kernelTreeCachePath(version, variant)` copied to `tmp/kernel/build`, else `buildKernelArtifact`, then `enforceKernelRequirements(resolved, <out>/config, floor)`, cache the tree, `evictKeepN`. Variant folds registry-derived hashes of profile, config, manifest and builder
- [ ] `internal/appliance/kernel.version` - `7.2`, embedded via `go:embed`
- [ ] `internal/appliance/kernelbuilder/driver.go` - backend selection (Docker image `ze-kernel-builder` from `tools/kernel-builder/Dockerfile`, or QEMU over a SHA-256-verified Alpine 3.21 ISO); `writeProvenance` records version/target/profile/arch/modules/builder only
- [ ] `internal/appliance/kernelbuilder/worker.go` - `downloadKernelSource` fetches `https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.2.tar.xz` with NO checksum or signature check; `applyPatches` (`gokrazy/kernel/patches/series`: nct6683, mpls-ip-mtu); `make defconfig`, `merge_config.sh` over fragments, `olddefconfig`, `enforceRequiredSymbols` BEFORE compile; `copyRuntimeOutputs` writes vmlinuz, `modules_install`, arm64 dtbs, overlays. No `KBUILD_BUILD_TIMESTAMP`/`USER`/`HOST`, no `SOURCE_DATE_EPOCH`
- [ ] `tools/kernel-builder/Dockerfile` - `golang:1.27-bookworm` builder stage, `debian:bookworm-slim` runtime with unpinned `apt-get install build-essential ...`: the compiler follows whatever bookworm ships that day
- [ ] `internal/le/test/deployment/gokrazyimage.go` - `resolveKernelPackage` (KERNEL_PKG env, staged `tmp/kernel/pkg`, else durable cache via `ze-host appliance kernel --print-cache-dir`, `assembleKernelPackage` from the rtr7 modcache skeleton); cold cache refuses with "about 30 minutes, needs docker"
- [ ] `internal/appliance/kernelconfig_pairing_test.go` - every `CONFIG_*=y` in `runtime.config` is in `runtime.require` or in `unverifiedRuntimeSymbols` with a reason
- [ ] `vendor/github.com/gokrazy/tools/internal/packer/{write.go,validatekernel.go,packerbuild.go}`, `vendor/github.com/gokrazy/tools/packer/gotool.go` `PackageDir` - the kernel package contract above
- [ ] `.github/workflows/qemu-nightly.yml` - CI builds the amd64 runtime kernel with `ze-host appliance kernel --target runtime --arch amd64`, cached by `hashFiles(kernel.version, gokrazy/kernel/**, tools/kernel-builder/**)`

**Question 1, how the runtime kernel is built today:** `ze appliance kernel --target runtime --arch <arch>` (Go driver `internal/appliance/kernelbuilder`) downloads the kernel.org tarball named by `kernel.version` (7.2), applies `gokrazy/kernel/patches/series`, merges `runtime.config` over `defconfig`, enforces `kernel.require` + `runtime.require` + the compiled floor, compiles in Docker (`ze-kernel-builder`, Debian bookworm) or a QEMU Alpine guest, and caches the tree in `~/.cache/ze/runtime-kernel/<version>-runtime-<arch>-runtime-<hash>-<hash>/`. It is pinned in its inputs (version string, tracked config, manifests, patches) but NOT reproducible: the tarball is unverified, the toolchain floats with Debian apt, and kbuild embeds the build timestamp, user and host. The symbol set is reproducible (enforced from the emitted config); the bytes are not.

**Behavior to preserve:**
- `ze.gok.kernel-package` remains an explicit operator override for a custom kernel package (it replaces the default, it is not a fallback to rtr7)
- the prepared-instance isolation: the checked-in `gokrazy/` tree is never written; two builds in one checkout do not share state
- `ze appliance kernel --target runtime` CLI, cache layout, `--print-cache-dir`, and the variant hash invalidation
- `ze appliance kernel` for the INSTALLER target (`tools/installer-kernel`, ISO path) is untouched
- requirement enforcement order: symbols checked before compile (worker) and after (Go, over the emitted config)

**Behavior to change:**
- the default image kernel: rtr7 6.19.11 becomes ze's runtime kernel for the image's `image.arch`
- `ze appliance build` and `./le build gokrazy` resolve the runtime kernel (cache hit, else native build) before preparing the instance
- the kernel source tarball is verified against a tracked SHA-256 before extraction
- the rtr7 builddir module, `KernelModule`, the deployment proof's rtr7 skeleton and every doc/rule naming rtr7 as the appliance kernel are removed

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze appliance build <name>` (appliance config `image.arch`) and `./le build gokrazy` (GOARCH, optional `ze.gok.kernel-package`)

### Transformation Path
1. Resolve arch from `image.arch` (or GOARCH for `./le build gokrazy`)
2. `resolveRuntimeKernel(defaultKernelVersion, arch, "runtime", builder, ...)`: cache hit, else verified-source native build plus requirement enforcement, then cache
3. Assemble a gokrazy kernel package directory under the prepared instance from the tracked skeleton (go.mod + one `.go` file) plus the cached tree's `vmlinuz`, `lib/modules`, `*.dtb`, `overlays/`
4. `instance.Prepare` with `KernelPackage` = that directory: the tracked ze kernel builddir module gets its replace pointed at it in the prepared copy only
5. `gok` resolves `KernelPackage` through the replace, checks arch, writes boot and root partitions

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze CLI → kernel builder (Docker/QEMU) | existing `kernelbuilder.Build` request | No |
| ze → gok packer | Go module replace in the prepared builddir, `go list -mod=mod` | No |
| Build host → kernel.org | HTTPS download, new SHA-256 check | No |

### Integration Points
- `resolveRuntimeKernel` / `kernelCachePathFor` (`internal/appliance/cmd_kernel.go`) - reused unchanged as the single resolver
- `instance.Options.KernelPackage` / `replaceKernel` (`internal/appliance/instance/prepare.go`) - retargeted from the rtr7 module to the ze kernel module
- `assembleKernelPackage` (`internal/le/test/deployment/gokrazyimage.go`) - moved into `internal/appliance` as the one assembler; the deployment proof calls it rather than owning a copy

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the image kernel goes through `resolveRuntimeKernel` and its `enforceKernelRequirements`, the same path `ze appliance kernel` uses |
| No unintended coupling (components stay isolated) | Yes | `internal/appliance` owns resolution and assembly; `internal/appliance/instance` keeps only the replace |
| No duplicated functionality (extends existing, does not recreate) | Yes | the deployment proof's assembler is moved, not copied; its resolver collapses onto the default path |
| Zero-copy preserved where applicable (refs, not copies) | N-A | build tooling, no wire path |
| Registration over hardcoding, outbound | N-A | no command, view, family or handler added |
| Registration over hardcoding, inbound | Yes | no list learns a new name: the kernel profile registry (`registeredKernelProfiles`) already holds `runtime`; the builddir module set is derived by `copyBuildDir` walking `go.mod` files |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Everyone who runs `ze appliance build` works from a ze checkout with Docker or QEMU available | `resolveBuildParentDir` uses relative `gokrazy`; the guide's end-to-end flow already runs `ze appliance kernel prod` (installer kernel, Docker/QEMU) before `ze appliance build` | operators without a builder cannot build an image on a cold cache; option (a) below becomes necessary | owner confirmation (open question) | unvalidated |
| A-2 | gokrazy needs nothing from rtr7's package beyond `vmlinuz`, `lib/modules`, `*.dtb`, `overlays`, a `.go` file and `go.mod` (rtr7's `cmdline.txt`, `config.txt` are Raspberry Pi firmware inputs, not read for x86/arm64 VM targets) | `kernelGlobs` in `vendor/.../packer/write.go`; deployment proof images boot with the assembled package | boot partition lacks a file; boot fails | `./le test qemu mpls-boot-test` and `./le test qemu vpp-hugepages-test` boot the new default image | unvalidated |
| A-3 | `runtime.config` boots the gokrazy appliance under QEMU and on N100 hardware (drivers, console, virtio, ext4/squashfs) | the L2TP deployment proof boots it under QEMU; no hardware boot recorded | hardware appliance fails to boot or loses NICs | QEMU proofs here; hardware boot recorded by the owner before release | unvalidated |
| A-4 | an arm64 default image builds once rtr7 is gone (ze's builder supports arm64) | `ze appliance kernel --arch arm64 --builder qemu` documented | arm64 image build fails at gokrazy arch check or boot | one arm64 `ze appliance build` | unvalidated |
| A-5 | kernel.org publishes `sha256sums.asc` for 7.x tarballs, so the tracked pin can be taken from it | kernel.org practice | the pin is computed from a download and the provenance is weaker | read the published file when writing the pin | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | first `ze appliance build` on a cold cache takes about 30 minutes and needs Docker or QEMU | build prints the builder banner | progress output names the step and the cache path; warm cache makes later builds instant; CI keeps its cache key |
| R-2 | offline build with a cold cache fails at the tarball download | download error | error names the missing cache entry and `ze appliance kernel --target runtime --arch <a>` to run once online |
| R-3 | prepared builddir lacks the replace target, so `gok` `go get`s the ze kernel module path from the network | modcache gains an entry for it (the modcache defect table) | the tracked module path is never published; `Prepare` refuses when no kernel package was resolved; a unit test asserts the refusal |
| R-4 | a stale tree in `tmp/kernel/build` or the cache is used for the wrong arch | gokrazy arch check fails | resolution keys on arch (variant already does); the assembler checks the vmlinuz magic per arch (`kernelMagic` moves with it) |
| R-5 | `runtime.config` edits now change the shipped image directly; a symbol dropped by Kconfig ships silently | none at build | `kernelconfig_pairing_test.go` plus `runtime.require` enforcement already refuse; unchanged |
| R-6 | two concurrent builds race on `tmp/kernel/build` | corrupted vmlinuz copy | the package is assembled inside the per-build prepared instance from the CACHE directory, never from `tmp/kernel/build` |
| R-7 | GPLv2 distribution obligation changes owner (rtr7 to ze) | n/a | doc names the corresponding source; sign-off stays with the owner |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | every appliance image: a kernel that does not boot, or lacks a driver the rtr7 kernel had |
| How is it reverted? | single commit revert restores the rtr7 module and config.json |
| Who else touches this path? | `plan/immediate/spec-kernel-capability-gate.md` (AC-15, `./le test qemu mpls-boot-test`, uncommitted `internal/le/test/qemu/mplsboot.go`), the L2TP deployment proof, `vpp-hugepages-test` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance build` | → | `resolveBuildParentDir` → `resolveRuntimeKernel` → kernel package assembly → `instance.Prepare(KernelPackage)` | `TestResolveBuildParentDirUsesRuntimeKernel` (`internal/appliance/kernelargs_test.go`, fake `kernelBuildFn`) |
| `./le build gokrazy` | → | same resolver, then `instance.Prepare` | `TestRunResolvesRuntimeKernelByDefault` (`internal/le/build/gokrazy/gokrazy_test.go`) |
| image boot | → | gokrazy boots ze's vmlinuz, ze starts with an MPLS seed | `./le test qemu mpls-boot-test` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze appliance build` of a default amd64 appliance | the image's kernel (`/proc/config.gz` or the packaged `config` read back from the image's kernel package) has `CONFIG_MPLS_ROUTING=y` and `CONFIG_MPLS_IPTUNNEL=y` |
| AC-2 | same image | every symbol in `gokrazy/kernel/kernel.require` and `runtime.require` reads `=y` in that config, and the kernel version equals `internal/appliance/kernel.version` |
| AC-3 | appliance seed adds `set fib kernel` and `set ldp`, booted under QEMU | ze starts and answers `show ldp neighbor | json` over its CLI (kernel-capability-gate AC-15, `./le test qemu mpls-boot-test`) |
| AC-4 | the tree after the change | no file outside `plan/learned/` and `plan/journal/` names `github.com/rtr7/kernel`; `gokrazy/ze/builddir/github.com/rtr7/kernel/` is gone; `gokrazy/ze/config.json` names the ze kernel module |
| AC-5 | `Prepare` called with no resolved kernel package | refuses with an error naming the kernel package; never falls back to a network-resolved module |
| AC-6 | (rewritten to the owner decision of 2026-10-09) `ze.gok.kernel-package` set in the environment of `./le build gokrazy` | the setting is not registered and changes nothing: the build resolves ze's runtime kernel as always (`TestRunHasNoKernelOverride`) |
| AC-7 | cached runtime kernel present for the arch | `ze appliance build` builds no kernel and starts no container or VM |
| AC-8 | kernel tarball whose SHA-256 differs from the tracked pin | the worker refuses before extraction, naming expected and actual digests |
| AC-9 | `kernel.version` changed without its SHA-256 pin changing | a unit test fails naming both files |
| AC-10 | cold cache, no network | `ze appliance build` fails with a message naming the cache path and the `ze appliance kernel --target runtime --arch <a>` command |
| AC-11 | `image.arch: arm64` | the build resolves the arm64 runtime kernel and gokrazy's arch check passes |
| AC-12 | `runtime.config` gains an `=y` line not in `runtime.require` nor `unverifiedRuntimeSymbols` | `kernelconfig_pairing_test.go` fails (unchanged behavior, reasserted because the default image now depends on it) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveBuildParentDirUsesRuntimeKernel` | `internal/appliance/kernelargs_test.go` | AC-1 wiring, AC-7 with a fake cached tree | |
| `TestPrepareRefusesWithoutKernelPackage` | `internal/appliance/instance/prepare_test.go` | AC-5 | |
| `TestPrepareReplacesZeKernelModule` | `internal/appliance/instance/prepare_test.go` | replace lands on the ze kernel module in the prepared copy only | |
| `TestAssembleKernelPackage` | `internal/appliance/kernelpkg_test.go` | skeleton + vmlinuz + modules + dtbs + overlays; refuses wrong-arch vmlinuz | |
| `TestDownloadKernelSourceVerifiesDigest` | `internal/appliance/kernelbuilder/worker_test.go` | AC-8 | |
| `TestKernelVersionHasDigestPin` | `internal/appliance/cmd_kernel_test.go` | AC-9 | |
| `TestColdOfflineCacheNamesRemedy` | `internal/appliance/cmd_kernel_test.go` | AC-10 | |
| `TestRunResolvesRuntimeKernelByDefault` / `TestRunHonorsKernelPackageOverride` | `internal/le/build/gokrazy/gokrazy_test.go` | AC-6 | |
| `TestPrepareRealInstanceCarriesEveryModule`, `TestPreparedModulesResolveIdenticallyToTracked` | `internal/appliance/instance/prepare_repo_test.go` | updated module set, AC-4 | |
| `TestNoRtr7KernelReference` | `internal/appliance/instance/prepare_repo_test.go` | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A: the feature takes no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mpls-boot-test` | `./le test qemu mpls-boot-test` (`internal/le/test/qemu/mplsboot.go`) | AC-1, AC-3: default image boots with MPLS in use | |
| `vpp-hugepages-test` | `./le test qemu vpp-hugepages-test` | default image still boots and reserves hugepages on the new kernel (A-2) | |
| `gokrazy-l2tp-ppp-test` | `./le test deployment gokrazy-l2tp-ppp-test` | L2TP proof on the default kernel, its private resolver removed | |

### Interop Tests (Scope: protocol)
N-A: tooling scope, no wire-visible change; LDP/MPLS interop belongs to the protocol specs.

## Files to Modify
- `gokrazy/ze/config.json` - `KernelPackage` names the ze kernel module
- `internal/appliance/instance/prepare.go` - `KernelModule` becomes the ze kernel module; `Prepare` requires a kernel package
- `internal/appliance/kernelargs.go` - `resolveBuildParentDir` resolves the runtime kernel for `cfg.Image.Arch` and passes the assembled package
- `internal/appliance/cmd_kernel.go` - expose the resolver result to the build path; cold-offline message
- `internal/appliance/kernelbuilder/worker.go` - SHA-256 verification of the tarball
- `internal/appliance/kernelbuilder/driver.go` - carry the digest in the request and provenance
- `internal/le/build/gokrazy/gokrazy.go` - default resolution; env description without rtr7
- `internal/le/test/deployment/gokrazyimage.go`, `gokrazykernel.go` - drop the private resolver and rtr7 skeleton, use the default path (keep `KERNEL_PKG` only if it maps onto `ze.gok.kernel-package`)
- `internal/le/setup/setup_test.go`, `internal/test/runner/needs_path.go` - drop rtr7 references
- `docs/guide/appliance.md`, `docs/architecture/appliance/gokrazy-build-pins.md`, `docs/architecture/appliance/kernel-profiles.md`, `docs/labs/l2tp-interop.md`
- `ai/rules/platform-linux.md`, `ai/rules/points/platform-linux/appliance-dependency-bumps/*.md` - bump runbook without rtr7, with the kernel.version + digest pair
- `plan/immediate/spec-kernel-capability-gate.md` - Depends on this spec (done with this design commit)

## Files to Create
- `gokrazy/ze/builddir/<ze kernel module path>/go.mod` - tracked builddir module requiring the ze kernel module, replaced in the prepared copy only
- `gokrazy/kernel/package/` (go.mod + one `.go` file) - the tracked kernel package skeleton
- `internal/appliance/kernelpkg.go` + `_test.go` - the one assembler (moved from the deployment proof)
- `internal/appliance/kernel.sha256` (or the pin beside `kernel.version`) - tarball digest

Deleted: `gokrazy/ze/builddir/github.com/rtr7/kernel/{go.mod,go.sum}`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | build tooling, no config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | No | `ze appliance build` keeps its flags; behavior changes only |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no new command |
| Functional test for new RPC/API | N-A | no RPC; QEMU boot proofs cover the path |
| Pipe completeness | N-A | no output command |
| Env var registration | Yes | `internal/le/build/gokrazy/gokrazy.go` `ze.gok.kernel-package` description |
| Doctor check for runtime dependencies | Yes | `internal/appliance/doctor_checks.go`: the build-host check for the runtime kernel cache or a usable builder, since `ze appliance build` now needs one |
| Prometheus counters/metrics | N-A | build tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the appliance feature entry under `features/` (MPLS on the appliance) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | behavior of `ze appliance build` described in `docs/guide/appliance.md` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/appliance/gokrazy-build-pins.md` boot-proof table; `docs/labs/l2tp-interop.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/kernel-profiles.md`, `build-artifacts.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation: `./le spec citation anchors spec plan/pre-release/spec-appliance-ships-ze-kernel.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/appliance.md` repo layout tree and "Kernel selection is explicit" |

Discovery: `ai/INDEX.md` row "runtime kernel profile, kernel config" gains "appliance image kernel, KernelPackage"; the regression guard is `TestNoRtr7KernelReference` plus `TestPrepareRefusesWithoutKernelPackage`; the inventory is the builddir module walk; the proof is `mpls-boot-test`.

## Implementation Steps

1. **Phase: Wiring** -- `Prepare` requires a kernel package; `resolveBuildParentDir` and `./le build gokrazy` call the resolver (fake in tests). Tests: wiring rows. Verify: wiring tests fail on the stub, then pass.
2. **Phase: Kernel package** -- move `assembleKernelPackage` into `internal/appliance/kernelpkg.go` with a tracked skeleton, no rtr7 copy; add the ze kernel builddir module; delete the rtr7 module. Tests: `TestAssembleKernelPackage`, `TestPrepareReplacesZeKernelModule`, repo tests.
3. **Phase: Verified source** -- tarball SHA-256 pin and check (AC-8, AC-9), provenance records the digest.
4. **Phase: Callers and docs** -- deployment proof onto the default path, setup/runner references, docs and rules (same phase, before the next code edit).
5. **Phase: Boot proofs** -- `mpls-boot-test`, `vpp-hugepages-test`, `gokrazy-l2tp-ppp-test`, one arm64 build (AC-11).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the image kernel comes from the CACHE directory, not `tmp/kernel/build`; arch keyed end to end |
| Rule: no-layering | no code path, env default or doc still selects rtr7 |
| Rule: principles (silent wrong value) | no kernel package means a refusal, never an unpinned fetch |
| Data flow | one resolver (`resolveRuntimeKernel`), one assembler |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| rtr7 gone | `grep -rn rtr7/kernel` outside `plan/learned`, `plan/journal`, `vendor`, `gokrazy/modcache` is empty |
| default image carries MPLS | `./le test qemu mpls-boot-test` |
| tarball verified | `TestDownloadKernelSourceVerifiesDigest` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the kernel source tarball is verified before extraction; the existing unsafe-path check in `extractTar` stays |
| Supply chain | no module resolved from the network for the kernel; digest pin reviewed on every `kernel.version` bump |

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

- The runtime kernel tree and the gokrazy kernel package differ by two files (go.mod and a placeholder `.go`); the deployment proof proved the shape by booting it.
- "Pinned" today means pinned inputs, not pinned bytes: the tarball is unverified and kbuild output is not reproducible. AC-8 closes the first; bit reproducibility is what option (b) forecloses.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| (b) `ze appliance build` resolves ze's runtime kernel through the existing cache-or-build resolver and assembles the gokrazy package at build time | (a) publish a ze kernel Go module (separate repo, or per-arch like gokrazy's `kernel.amd64`), vmlinuz committed by CI, pinned in the builddir go.mod/go.sum; (a') commit the kernel package inside this repo with a filesystem replace; (c) add a release-server download tier for the runtime kernel, SHA-256 pinned in the repo, before the local build | (b) reuses every piece that exists (resolver, cache, requirement enforcement, assembler) and pairs `runtime.config` with the shipped kernel BY CONSTRUCTION: the image is always built from the config at the same commit, so no drift check is needed. (a) needs a second repo, a CI publish job, owner-ordered pushes and a drift test tying the module's recorded input hash to `runtime.config`/`runtime.require`/patches/`kernel.version`; a config edit takes two commits in two repos to reach an image; operator cost is zero and offline works from the modcache. (a') puts 17 MB per arch per kernel bump into this repository's history forever. (c) needs release hosting that a pre-release project does not have, and its pin must be bumped after every config edit |
| tarball verified by a tracked SHA-256 pin | trust HTTPS only; GPG-verify `sha256sums.asc` | a tracked digest is the simplest proof that the source is the one reviewed; GPG adds a keyring to the builder for no stronger claim at this stage |
| rtr7 removed, not kept as fallback | keep rtr7 for hosts without a builder | `ai/rules/no-layering.md`; a fallback ships an image that silently lacks MPLS, L2TP and nftables inet, which is the defect being fixed |
| `ze.gok.kernel-package` kept | delete it | it is an explicit custom-kernel override, not a second default |

What (b) forecloses: a cold-cache `ze appliance build` without Docker or QEMU (about 30 minutes, network to kernel.org); bit-identical images across build hosts at one commit (toolchain from unpinned Debian apt, kbuild timestamp/user/host); and automatic upstream kernel updates from rtr7's autoupdate, so `kernel.version` bumps are ze's own job. A later move to (a) or (c), for signed release images, stays open: it adds a tier in front of the same resolver.

## Known Limitations

- Bit-reproducible kernel builds (pinned builder image digest, `KBUILD_BUILD_TIMESTAMP`/`USER`/`HOST`) are not in scope; they matter once images are released and signed.
- The GPLv2 source-offer sign-off remains the owner's licensing decision.
- A hardware (N100) boot of the new kernel is owner-run evidence (A-3).

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
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/pre-release/spec-appliance-ships-ze-kernel.md` only, in the same `./le commit create` script
