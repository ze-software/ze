# Spec: appliance-ships-ze-kernel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P2 |
| Release reason | Required release artifacts, safe installation or appliance/runtime-kernel delivery and their outstanding boot evidence. |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision (Thomas, 2026-10-09): the appliance kernel carries MPLS.

-> Decision (owner, 2026-10-09): "we have our own kernel we should ONLY use it NOTHING ELSE". Every image path (`ze appliance build`, `./le build gokrazy`, the deployment proofs, the QEMU labs) boots ze's own runtime kernel and no other. `github.com/rtr7/kernel` leaves the tree (go.mod, vendor, builddir, deployment skeletons). The `ze.gok.kernel-package` override is DELETED, not kept: a route that lets an image carry a different kernel is the thing this decision forbids. AC-6 and the "`ze.gok.kernel-package` kept" design row are superseded by this decision and must be rewritten to it. Found the same day: crash capture (`plan/release/fixes/spec-crash-capture.md`) cannot work on rtr7 (no `CONFIG_PSTORE`) while the build accepted `image.crash-dump`, because every kernel floor check judges ze's kernel, not the shipped one; with ze's kernel the only kernel, the checks judge what ships.

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
(`plan/release/validation/spec-kernel-capability-gate.md`, AC-15) refuses it.

Goal: the default `ze appliance build` image boots ze's own runtime kernel,
built from the tracked `runtime.config` at the same commit, and the rtr7 kernel
leaves the build entirely. `plan/release/validation/spec-kernel-capability-gate.md`
AC-15 depends on this spec.

### Owner decisions, second round (2026-10-09)

-> Decision (owner, 2026-10-10, recorded as D-5 in `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md`): "docker needs our kernel". Ze's Docker hosts run exactly this appliance kernel, so the options Docker itself needs (moby `contrib/check-config.sh`, "Generally Necessary", plus overlayfs, user namespaces and seccomp) join `runtime.config` and `runtime.require`, declared once. There is no separate docker-host profile. That spec's AC-14 owns the edit; this spec's build and cache carry it unchanged.
-> Decision (owner): kernel route (b) confirmed: "we want to control the modules so we need to compile our own kernel". No published kernel module (option a) and no download tier (option c).
-> Decision (owner): every kernel is built on a host of its own arch, with no CPU emulation: arm64 on the Mac, amd64 on the Linux host. Found at source: today the Docker backend passes `--platform linux/<arch>` (`internal/appliance/kernelbuilder/driver.go`, `dockerPlatforms`), which runs a foreign arch under emulation, and the QEMU backend falls back to `-accel tcg` when neither `hvf` nor `kvm` serves the target binary (`internal/appliance/kernelbuilder/qemu.go`, accelerator selection). Both routes are refused by AC-13.
-> Decision (owner): a long cold first build is acceptable IF the result is cached and the cache is never deleted when space is reclaimed. Found at source, what removes kernel build output today:

| Remover | What it touches | Reaches the runtime kernel cache? |
|---------|-----------------|------------------------------------|
| `evictKeepN` (`internal/appliance/cache.go`), run after every runtime build (`cmd_kernel.go`, `resolveRuntimeKernel`) and by `ze appliance kernel --evict-cache` | keeps the `evictKeepDefault` = 2 most recently modified entries of `~/.cache/ze/runtime-kernel/`, by mtime, ACROSS arches | Yes: two arm64 rebuilds evict the amd64 entry, and the reverse |
| `./le scratch clean` (`internal/le/scratch/cacheclean.go`, `cleanTargets`) | the checkout, shared, ambient and bootstrap Go caches and the lint cache | No today; nothing asserts it stays so |
| store trim (`internal/le/scratch/storetrim.go`, `trimStores`, `livenessStores`) | Go cache entries by budget, dead session dirs and testbin sets under `tmp/` | No today; nothing asserts it |
| every resolver call | rewrites `tmp/kernel/build` (`runtimeKernelOutputDir`); an output dir, not a cache | n/a; redirected by `ZE_KERNEL_TEST_OUTPUT_DIR` |

-> Open (owner): the GPLv2 source offer for the shipped kernel. The owner asked what it is and is having it explained; this spec does not decide it (R-7).
-> Decision (owner, 2026-10-09, closes the open GPLv2 question above and R-7): Ze complies by pointing users to the published Linux source. Each appliance image carries a notice that it contains Linux under GPLv2, naming the exact kernel.org version and the source tarball URL with its SHA-256; the kernel config, the patch series (`gokrazy/kernel/patches/`) and the build scripts are in the public Ze repository. The kernel source is not re-published. The notice, version, URL and checksum derive from the same declaration the build uses (`internal/appliance/kernel.version` and the tarball SHA-256 pin of AC-8/AC-9), so the notice cannot drift from the kernel built (AC-17).
-> Owner 2026-10-09: no AGPLv3 notice in the image; Ze is the owner's own code, published on GitHub.
-> Decision (owner, 2026-10-09): "we should use the latest stable 7.x kernel". Ze pins the latest stable 7.x release by EXACT version plus SHA-256, because the GPLv2 notice and the cache key both need an exact version; a later bump is a deliberate change to that one declaration. Found 2026-10-09: `internal/appliance/kernel.version` pins `7.2` (the 7.2.0 tarball, `linux-7.2.tar.xz`, SHA-256 `f9fef3d14c0df53819026f4be74459835c2a0b0dcbf5b5bbd9ea19f0829402b3`); kernel.org `releases.json` names latest stable `7.2.9` (released 2026-10-03; mainline is 7.3-rc6), `linux-7.2.9.tar.xz` SHA-256 `b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba` from `https://cdn.kernel.org/pub/linux/kernel/v7.x/sha256sums.asc`. The pin is behind, so the bump to 7.2.9 is part of AC-1 (AC-19), and the first rebuild happens there. This also validates A-5: kernel.org publishes `sha256sums.asc` for 7.x.
-> Decision (owner): "ignore n100 atm". N100 (amd64) is a supported BUILD target; booting on N100 hardware is owner-deferred until the owner runs real hardware again. A-3's hardware half is therefore not evidence this spec owes; its QEMU half stands.
-> Related: `spec-appliance-kernel-vpn-modules` remained separate by owner decision. Its native kernel traffic and default-image config criteria are demonstrated; the built-in contract is in `docs/architecture/appliance/kernel-profiles.md`. This does not close this spec's other image or L2TP/PPP obligations.
-> Implemented (2026-10-10, owner order to land the bump first): AC-19 and the worker half of AC-8/AC-9. `kernel.version` reads 7.2.9; `kernelSourceSHA256` (`internal/appliance/kernelbuilder/worker.go`) holds `b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba`, which `sha256sum` of the downloaded `linux-7.2.9.tar.xz` matched; `verifyKernelSource` refuses any other digest before extraction (`TestDownloadKernelSourceVerifiesDigest`), and `TestKernelVersionHasDigestPin` sits in `internal/appliance/cmd_kernel_test.go`. The series now applies with `patch --fuzz=0`: on 7.2.9, 0001 hunk 3 needed fuzz and 0002 failed two hunks (`ip6_route.h` gained an `IP6_MAX_MTU` clamp at `out:`; `ip_do_fragment` gained upstream the same `mtu < hlen + 8` guard 0002 carried, so that hunk is dropped). Both patches were regenerated against pristine 7.2.9 and apply with no fuzz and no offset. The GPLv2 notice (AC-17) and the bump runbook edit remain this spec's.

-> Implemented (2026-10-10, phases 1, 2 and 4 code half): rtr7 and `ze.gok.kernel-package` deleted; `gokrazy/ze/config.json` names `ze.invalid/kernel`; `ze appliance build` (`resolveBuildParentDir`) and `./le build gokrazy` (`prepareArgs`) resolve `appliance.RuntimeKernelTree(arch)` and `instance.Prepare` assembles the package from that cache entry inside the prepared parent and replaces the module. AC-4, AC-5, AC-6 (rewritten), AC-7, AC-10 carry unit tests; AC-1/2/3/11 still owe the boot proofs; AC-13, AC-17, the doctor check and the provenance digest are not started.
-> Decision (implementation): the kernel module path is `ze.invalid/kernel`: RFC 2606 reserves `.invalid`, so a prepared instance whose replace went missing fails `go list` instead of fetching (R-3). The assembler lives in `internal/appliance/instance/kernelpkg.go`, not `internal/appliance/kernelpkg.go`, because `Prepare` assembles inside the prepared parent (R-6) and `instance` cannot import `appliance`; it writes the go.mod and Go file itself, so no tracked `gokrazy/kernel/package/` skeleton exists. `TestNoRtr7KernelReference` excludes all of `plan/` (specs describe the removal), `vendor/` and `gokrazy/modcache/`. `./le setup install` skips the kernel module, which has nothing to download.
-> Evidence (2026-10-10, arm64 on the Mac under HVF; AC-1, AC-2, AC-3 and AC-11 for arm64; the amd64 half is owed on the Linux host): `./le test qemu mpls-boot` (`ze_vpp_hp_aarch64_bios=/opt/homebrew/share/qemu/edk2-aarch64-code.fd`; the default firmware path is Linux's) built the arm64 appliance with the MPLS seed from the cached runtime kernel, so gokrazy's arch check passed (AC-11), and booted it: console `Linux version 7.2.9-ze ... #1 SMP PREEMPT Sat Oct 10 09:18:07 UTC 2026`, ze started with `fib-kernel: running` and LDP loaded. The proof then FAILED with no SSH answer: the harness gave the arm64 guest an e1000 NIC, which the arm64 kernel builds as a module (`CONFIG_E1000=m`), and gokrazy loads no modules. Fixed in the harness (`Hugepages.qemuArgs`, `internal/le/test/qemu/boot.go`: arm64 takes `virtio-net-pci`, which `runtime.require` makes built in; `TestTheArm64MachineGetsTheNICItsKernelBuildsIn`, red first). The kept image booted with that NIC answered over SSH: `show host kernel | json` gives `"release": "7.2.9-ze"`, `"architecture": "arm64"` (equals `internal/appliance/kernel.version`, AC-2); `show ldp neighbor | json` answers `[]`, JSON from the LDP engine (AC-3); `show doctor | json` answers (a no-NTP error and listener warnings only, no kernel-capability refusal). The harness itself was not re-run after the NIC fix, because its fresh host build hashes the kernel builder sources AC-13 changed and would start the cold arm64 rebuild. AC-1/AC-2 config: every one of the 128 symbols in `kernel.require` plus `runtime.require` reads `=y` in the packaged config of cache entry `7.2.9-runtime-arm64-runtime-7c0d8f79-bf3ac1c9`, including `CONFIG_MPLS_ROUTING=y` and `CONFIG_MPLS_IPTUNNEL=y`; `CONFIG_LOCALVERSION="-ze"`, `CONFIG_IKCONFIG_PROC=y`. That entry was superseded (no provenance source lines); re-checked 2026-10-10 against the entry the passing boot below ran on, `7.2.9-runtime-arm64-runtime-7c0d8f79-d826e25e`: all 128 symbols of the two require files read `=y` in its `config`, with the same four lines. 45834f22d7 and 6aa76ad8a0 changed the kernel builder sources, so d826e25e is itself stale for the next build: the next arm64 image rebuilds the kernel, and its config owes the same check.
-> Implemented (2026-10-10): AC-13 in 179ca326f2. `validateRequest` refuses a kernel for an arch other than `hostGOARCH`; `qemuArgs` refuses a QEMU with neither hvf nor kvm, with no tcg fallback. This changes the runtime kernel cache key, so the arm64 cache entry above is stale and a cold rebuild (about 30 minutes) is owed before the next arm64 image build.
-> Implemented (2026-10-10): the provenance source digest in 33d6ff0a4e. `writeProvenance` adds `source-url` (`kernelTarballURL`, what `downloadKernelSource` fetches) and `source-sha256` (`SourceDigest`, what `verifyKernelSource` checks), and refuses a version with no tracked digest. AC-17 next: `kernelbuilder.ReadProvenance` and `Provenance.LinuxNotice` (`provenance.go`); `instance.Prepare` reads the kernel tree's provenance and `addLinuxNotice` puts the notice at `/etc/linux-gpl-notice` through the ze package's `ExtraFileContents` (gokrazy copies only `lib/modules` from a kernel package). `TestLinuxNoticeMatchesTheBuiltKernel` builds the notice from the provenance `writeProvenance` wrote and asserts version, URL and digest; `TestPrepareCarriesTheLinuxNotice` and `TestPrepareRefusesAKernelWithoutProvenance` went red with the `addLinuxNotice` call removed. A cache entry built before 33d6ff0a4e has no source lines and is refused, so every existing cache entry needs the rebuild. Found: `kernel-builder-single-driver` and `kernel-wiring` fixtures are red on an arm64 host since 179ca326f2 (their amd64 requests meet the AC-13 refusal).
-> Evidence (2026-10-10, arm64 on the Mac under HVF, after the cold rebuild; AC-1, AC-3 and AC-11 for arm64, with the NIC fix and AC-13 in the tree): the cold rebuild wrote cache entry `7.2.9-runtime-arm64-runtime-7c0d8f79-d826e25e`, whose `kernel.version` carries `source-url=https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.2.9.tar.xz` and `source-sha256=b4c5dfbe...d8ba`; the old entry `...-bf3ac1c9` (no source lines) has a different key and nothing selected it. The first image build after the rebuild FAILED in gok: le exports `GOTOOLCHAIN=go1.27.0` (go.mod's go directive, for its linter), the host go is 1.27.1, and with `GOMODCACHE=gokrazy/modcache` and `GOPROXY=off` every go subprocess answered `toolchain not available`; gok reported its failed `go list` as `go get ze.invalid/kernel`. The replace was in place (the failed builddir's go.mod carries it, and `go list ze.invalid/kernel` resolves under `GOTOOLCHAIN=local`). Fixed in 0313f16ee7: `appliance.SetGokGoEnv` sets `GOTOOLCHAIN=local` for both gok entry points (`TestGokBuildsWithTheHostToolchain`, red first). Re-run: `ze_vpp_hp_aarch64_bios=/opt/homebrew/share/qemu/edk2-aarch64-code.fd ./le --name zek test qemu mpls-boot-test` answered `APPLIANCE-MPLS-QEMU: PASS ze started with MPLS in use; \`show ldp neighbor | json\` answered`, exit 0 (log `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/mpls-boot-arm64-run2.log`). The arm64 fixture red is fixed in ecd9a892ba (`kernel-builder-single-driver`, `kernel-wiring` use `runtime.GOARCH`). `/etc/linux-gpl-notice` was not read back from the booted image.
-> Implemented (2026-10-10): the doctor check. `ze doctor` registers `appliance-runtime-kernel` (code `doctor-appliance-runtime-kernel`, a warning; `checkRuntimeKernel` in `internal/appliance/doctor_checks.go`). It asks about the host arch, because AC-13 builds the kernel only there: it passes when the cache entry `resolveRuntimeKernel` would serve holds its `vmlinuz`, else when `kernelbuilder.UsableBuilder` finds Docker, or QEMU with Go. With neither, it names the cache entry, why `ze appliance build` needs it, and `ze appliance kernel --target runtime --arch <arch>`. Where the kernel config cannot be read (outside the source tree) it warns rather than pass. Unit tests `TestDoctorRuntimeKernelCached`, `TestDoctorRuntimeKernelBuildable`, `TestDoctorRuntimeKernelMissing`, `TestDoctorRuntimeKernelOutsideSourceTree` (`internal/appliance/doctor_checks_test.go`): Missing and OutsideSourceTree went red against a stub returning nil; Cached and Buildable went red under mutations removing the cache test and the builder test. Functional `test/ui/doctor-appliance-runtime-kernel.ci` runs `ze doctor --json` outside the tree and asserts the code and the message; red with the unreadable-config branch mutated to return nil, green restored. Page: `docs/architecture/appliance/build-artifacts.md`.
-> Evidence (2026-10-10, AC-17 read back from a built image): `ze appliance build t1` (arm64, tree at 3fa832a60c, `ZE_APPLIANCE_DIR` absolute) served cache entry `7.2.9-runtime-arm64-runtime-7c0d8f79-d826e25e` with no kernel build and wrote `ze-20261010-140153.img` (image SHA-256 `33958148aee423ff94b2643d498083837e4da32a56127b568c67aecd3b7896dd`, exit 0, log `scratch/appl-build-notice3.log`). `7zz` extracted the root squashfs (GPT partition 1) and its `etc/linux-gpl-notice` reads `Version: 7.2.9`, `Source: https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.2.9.tar.xz`, `SHA-256: b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba`, plus the GPLv2 statement and the pointer to `gokrazy/kernel/`, `gokrazy/kernel/patches/` and `tools/kernel-builder/`. The three values equal the cache entry's provenance (`~/.cache/ze/runtime-kernel/7.2.9-runtime-arm64-runtime-7c0d8f79-d826e25e/kernel.version`: `version=7.2.9`, the same `source-url` and `source-sha256`) and `kernelSourceSHA256["7.2.9"]` in `internal/appliance/kernelbuilder/worker.go`. Found on the way: a relative `ZE_APPLIANCE_DIR` loses the image to in-process gok's `os.Chdir` (journal row in `plan/journal/option-set-for-one-caller-changes-another.md`, 6c4e143c3e; predates this spec).
-> Evidence (2026-10-11, round 11 NOTE 4; supersedes the d826e25e citations above): `...-7c0d8f79-d826e25e` was a QEMU-built entry under an older key, and eviction has since removed it; the three evidence lines above stand as the record of that run only. The current tree keys arm64 to `7.2.9-runtime-arm64-runtime-docker-7c0d8f79-34cba457` (builder "" and "docker"; built by Docker 2026-10-11 00:13, `scratch/kernel-arm64-build5.log`), and the edits of this round leave that key unchanged (`scratch/r11/probe_r11_key_test.go` re-run). AC-1/AC-2 config on 34cba457: 128 of 128 symbols in `kernel.require` + `runtime.require` read `=y` in its `config`, plus `CONFIG_MPLS_ROUTING=y`, `CONFIG_MPLS_IPTUNNEL=y`, `CONFIG_LOCALVERSION="-ze"`, `CONFIG_IKCONFIG_PROC=y` (`scratch/r11/out_symbols.log`); this discharges "the arm64 config owes the 128-symbol check again" of Round 2 and Round 3 fixes. Boot on 34cba457: `ze_vpp_hp_aarch64_bios=/opt/homebrew/share/qemu/edk2-aarch64-code.fd ./le --name zek11 test qemu mpls-boot-test` served the cached entry with no kernel build (the cache directory is unchanged, no builder container or VM ran) and answered `APPLIANCE-MPLS-QEMU: PASS ze started with MPLS in use; \`show ldp neighbor | json\` answered`, exit 0 (`scratch/r11close-mpls-boot-arm64.log`).

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-13 | `ze appliance kernel` (or a build that resolves it) for an arch other than the host's, with either backend | refuses before building, naming the host arch, the requested arch and that it is built on a host of that arch; never runs Docker with a foreign `--platform` nor QEMU under `tcg` |
| AC-14 | `./le scratch clean` and store trim run with a populated `~/.cache/ze/runtime-kernel/` | every entry survives; a unit test asserts no clean or trim target resolves inside a kernel cache namespace |
| AC-15 | runtime-kernel cache holding an arm64 and an amd64 entry; two further arm64 builds of new variants | the amd64 entry survives, and the entry the current tree's variant names is never evicted for either arch (eviction keeps the newest per arch) |
| AC-16 | `docs/contributing/running-commands.md` "When the disk is full" | names the kernel cache, states it is not a reclamation target and why (a cold rebuild is about 30 minutes) |
| AC-17 | any `ze appliance build` image | carries a GPLv2 notice naming Linux, the exact kernel.org version, the tarball URL and its SHA-256, all derived from `kernel.version` and the tracked digest pin the worker verifies; a test builds the notice and asserts its version equals the built kernel's `kernel.version` provenance and its URL and digest equal the ones `downloadKernelSource` used |
| AC-19 | `internal/appliance/kernel.version` and its digest pin, at AC-1 | pin the exact latest stable 7.x found when AC-1 is implemented (7.2.9 on 2026-10-09, or later if kernel.org has moved), with the SHA-256 from kernel.org's `sha256sums.asc`; the patch series applies cleanly to it; `TestKernelVersionHasDigestPin` (AC-9) covers the pair |

AC-18 was never assigned. The gap is kept rather than renumbered, because commits and evidence lines already cite AC-19 by number.

-> Decision (review round 1 NOTE 3, 2026-10-10): the notice does NOT name a Ze commit. The runtime kernel cache key hashes the working-tree build inputs (`kernelCacheVariantFor` in `internal/appliance/cache.go`: the profile's config fragments and manifests, the patches and `kernelBuilderSources`, beside the version in the entry name), not a commit: a dirty tree has no commit to name, and a cached kernel is served unchanged across later commits that do not touch those inputs, so a commit recorded at build time would name a tree that is not what the kernel was built from. What the notice names (version, tarball URL, SHA-256) is what the provenance records from the build itself. Recording the commit would also mean editing `internal/appliance/kernelbuilder`, which changes the cache key once more. This is an engineering decision about drift, not a licence reading; the owner's GPLv2 decision above is unchanged.

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

**Question 1, how the runtime kernel is built today:** `ze appliance kernel --target runtime --arch <arch>` (Go driver `internal/appliance/kernelbuilder`) downloads the kernel.org tarball named by `kernel.version` (7.2), applies `gokrazy/kernel/patches/series`, merges `runtime.config` over `defconfig`, enforces `kernel.require` + `runtime.require` + the compiled floor, compiles in Docker (`ze-kernel-builder`, Debian bookworm) or a QEMU Alpine guest, and caches the tree in `~/.cache/ze/runtime-kernel/<version>-runtime-<arch>-runtime-<backend>-<config-hash>-<builder-hash>/` (corrected in round 4: `kernelCacheVariantFor`, `internal/appliance/cache.go`, keys the backend since 590760235b; entries cited below from before it, such as `...-7c0d8f79-d826e25e`, carry no backend). It is pinned in its inputs (version string, tracked config, manifests, patches) but NOT reproducible: the tarball is unverified, the toolchain floats with Debian apt, and kbuild embeds the build timestamp, user and host. The symbol set is reproducible (enforced from the emitted config); the bytes are not.

**Behavior to preserve:**
- (superseded by the owner decision of 2026-10-09 in Task: `ze.gok.kernel-package` is deleted, and no route lets an image carry a kernel other than ze's own; AC-6)
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
- `ze appliance build <name>` (appliance config `image.arch`) and `./le build gokrazy` (GOARCH; no kernel override exists, AC-6)

### Transformation Path
1. Resolve arch from `image.arch` (or GOARCH for `./le build gokrazy`)
2. `resolveRuntimeKernel(defaultKernelVersion, arch, "runtime", builder, ...)`: cache hit, else verified-source native build plus requirement enforcement, then cache
3. `instance.Prepare` with `KernelTree` = that cache entry and `Arch`: `checkKernelArch` reads the vmlinuz magic, `addLinuxNotice` puts `/etc/linux-gpl-notice` into the ze package's `ExtraFileContents` from the entry's provenance, and `assembleKernelPackage` (`internal/appliance/instance/kernelpkg.go`) writes the module's go.mod and Go file itself (no tracked skeleton) and copies `vmlinuz`, `lib/modules`, `*.dtb`, `overlays/`, `cmdline.txt` and `config.txt` into the prepared parent
4. The prepared builddir's `ze.invalid/kernel` module gets its replace pointed at that directory, in the prepared copy only
5. `gok` resolves `ze.invalid/kernel` through the replace, checks arch, writes boot and root partitions

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze CLI → kernel builder (Docker/QEMU) | existing `kernelbuilder.Build` request | No |
| ze → gok packer | Go module replace in the prepared builddir, `go list -mod=mod` | No |
| Build host → kernel.org | HTTPS download, new SHA-256 check | No |

### Integration Points
- `resolveRuntimeKernel` / `kernelCachePathFor` (`internal/appliance/cmd_kernel.go`) - reused unchanged as the single resolver
- `instance.Options.KernelTree` (`internal/appliance/instance/prepare.go`) - replaces `KernelPackage`: `Prepare` assembles the package itself and replaces `ze.invalid/kernel` (was the rtr7 module)
- `assembleKernelPackage` - moved from `internal/le/test/deployment/gokrazyimage.go` into `internal/appliance/instance/kernelpkg.go` as the one assembler, because `Prepare` assembles inside the prepared parent and `instance` cannot import `appliance`; the deployment proof goes through `Prepare` rather than owning a copy

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the image kernel goes through `resolveRuntimeKernel` and its `enforceKernelRequirements`, the same path `ze appliance kernel` uses |
| No unintended coupling (components stay isolated) | Yes | `internal/appliance` owns resolution; `internal/appliance/instance` owns assembly, provenance notice and prepared-module replacement |
| No duplicated functionality (extends existing, does not recreate) | Yes | the deployment proof's assembler is moved, not copied; its resolver collapses onto the default path |
| Zero-copy preserved where applicable (refs, not copies) | N-A | build tooling, no wire path |
| Registration over hardcoding, outbound | N-A | no command, view, family or handler added |
| Registration over hardcoding, inbound | Yes | no list learns a new name: the kernel profile registry (`registeredKernelProfiles`) already holds `runtime`; the builddir module set is derived by `copyBuildDir` walking `go.mod` files |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Everyone who runs `ze appliance build` works from a ze checkout with Docker or QEMU available | `resolveBuildParentDir` uses relative `gokrazy`; the guide's end-to-end flow already runs `ze appliance kernel prod` (installer kernel, Docker/QEMU) before `ze appliance build` | operators without a builder cannot build an image on a cold cache; option (a) below becomes necessary | owner confirmation (open question) | validated: owner chose route (b) ("we need to compile our own kernel") and accepted a cached cold build; `ze doctor` warns when neither a cache entry nor a builder exists |
| A-2 | gokrazy needs nothing from rtr7's package beyond `vmlinuz`, `lib/modules`, `*.dtb`, `overlays`, a `.go` file and `go.mod` (rtr7's `cmdline.txt`, `config.txt` are Raspberry Pi firmware inputs, not read for x86/arm64 VM targets) | `kernelGlobs` in `vendor/.../packer/write.go`; deployment proof images boot with the assembled package | boot partition lacks a file; boot fails | `./le test qemu mpls-boot` and `./le test qemu vpp-hugepages` boot the new default image | validated by historical arm64 boot and fresh native amd64 MPLS/hugepage boot on the POLICE kernel |
| A-3 | `runtime.config` boots the gokrazy appliance under QEMU and on N100 hardware (drivers, console, virtio, ext4/squashfs) | native QEMU boot evidence below; no hardware boot recorded | hardware appliance fails to boot or loses NICs | QEMU proofs here; hardware boot recorded by the owner before release | QEMU half validated on native amd64 KVM and historical arm64 HVF; hardware half owner-deferred ("ignore n100 atm") |
| A-4 | an arm64 default image builds once rtr7 is gone (ze's builder supports arm64) | `ze appliance kernel --arch arm64 --builder qemu` documented | arm64 image build fails at gokrazy arch check or boot | one arm64 `ze appliance build` | validated: arm64 `ze appliance build t1` exit 0 (`ze-20261010-140153.img`) and `mpls-boot-test` PASS |
| A-5 | kernel.org publishes `sha256sums.asc` for 7.x tarballs, so the tracked pin can be taken from it | kernel.org practice | the pin is computed from a download and the provenance is weaker | read the published file when writing the pin | validated: the 7.2.9 digest came from `sha256sums.asc` and matched the download (c9e17d79ea) |

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
| Who else touches this path? | `plan/release/validation/spec-kernel-capability-gate.md` (AC-15, `./le test qemu mpls-boot`, uncommitted `internal/le/test/qemu/mplsboot.go`), the L2TP deployment proof, `vpp-hugepages-test` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance build` | → | `resolveBuildParentDir` → `resolveRuntimeKernel` → kernel package assembly → `instance.Prepare(KernelPackage)` | `TestResolveBuildParentDirUsesRuntimeKernel` (`internal/appliance/kernelargs_test.go`, fake `kernelBuildFn`) |
| `./le build gokrazy` | → | same resolver, then `instance.Prepare` | `TestRunResolvesRuntimeKernelByDefault` (`internal/le/build/gokrazy/gokrazy_test.go`) |
| image boot | → | gokrazy boots ze's vmlinuz, ze starts with an MPLS seed | `./le test qemu mpls-boot` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze appliance build` of a default amd64 appliance | the image's kernel (`/proc/config.gz` or the packaged `config` read back from the image's kernel package) has `CONFIG_MPLS_ROUTING=y` and `CONFIG_MPLS_IPTUNNEL=y` |
| AC-2 | same image | every symbol in `gokrazy/kernel/kernel.require` and `runtime.require` reads `=y` in that config, and the kernel version equals `internal/appliance/kernel.version` |
| AC-3 | appliance seed adds `set fib kernel` and `set ldp`, booted under QEMU | ze starts and answers `show ldp neighbor | json` over its CLI (kernel-capability-gate AC-15, `./le test qemu mpls-boot`) |
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
| `TestAssembleKernelPackage` | `internal/appliance/instance/kernelpkg_test.go` (moved; no skeleton, see Deviations) | skeleton + vmlinuz + modules + dtbs + overlays; refuses wrong-arch vmlinuz | |
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
| `mpls-boot-test` | `./le test qemu mpls-boot` (`internal/le/test/qemu/mplsboot.go`) | AC-1, AC-3: default image boots with MPLS in use | |
| `vpp-hugepages-test` | `./le test qemu vpp-hugepages` | default image still boots and reserves hugepages on the new kernel (A-2) | |
| `gokrazy-l2tp-ppp-test` | `./le test deployment gokrazy-l2tp-ppp` | L2TP proof on the default kernel, its private resolver removed | |

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
- `plan/release/validation/spec-kernel-capability-gate.md` - Depends on this spec (done with this design commit)

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
| Env var registration | Yes | removed `ze.gok.kernel-package`; `internal/le/build/gokrazy/gokrazy.go` `prepareArgs` always resolves Ze's runtime kernel |
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation: `./le spec citation anchors spec plan/release/delivery/spec-appliance-ships-ze-kernel.md` |
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
| default image carries MPLS | `./le test qemu mpls-boot` |
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
| `ze.gok.kernel-package` deleted (owner decision 2026-10-09, replacing the first design's "kept") | keep it as an explicit custom-kernel override | "we have our own kernel we should ONLY use it NOTHING ELSE": a route that lets an image carry another kernel is what the decision forbids, and every kernel floor check then judges the kernel that ships (`TestRunHasNoKernelOverride`) |

What (b) forecloses: a cold-cache `ze appliance build` without Docker or QEMU (about 30 minutes, network to kernel.org); bit-identical images across build hosts at one commit (toolchain from unpinned Debian apt, kbuild timestamp/user/host); and automatic upstream kernel updates from rtr7's autoupdate, so `kernel.version` bumps are ze's own job. A later move to (a) or (c), for signed release images, stays open: it adds a tier in front of the same resolver.

## Known Limitations

- Bit-reproducible kernel builds (pinned builder image digest, `KBUILD_BUILD_TIMESTAMP`/`USER`/`HOST`) are not in scope; they matter once images are released and signed.
- The owner chose the GPLv2 notice/source-pointer policy on 2026-10-09; its implementation is complete. This is not an independent legal opinion.
- N100 hardware boot was excluded from this spec's owed evidence by the owner's "ignore n100 atm" decision; no hardware boot or hardware support validation is claimed.

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
- [ ] Scoped verification covers this change; full `./le verify worktree` remains the owner's release check, never an agent-run closure gate
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
- [ ] **Commit B:** `remove plan/release/delivery/spec-appliance-ships-ze-kernel.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- rtr7 removed and `ze.gok.kernel-package` deleted; `gokrazy/ze/config.json` names `ze.invalid/kernel`; `ze appliance build` (`resolveBuildParentDir`) and `./le build gokrazy` resolve `RuntimeKernelTree(arch)`, and `instance.Prepare` assembles the kernel package inside the prepared parent (36c851fc58, 0a221fb741).
- Linux pinned to 7.2.9 by exact version and SHA-256; `verifyKernelSource` refuses any other tarball; patch series regenerated to apply with no fuzz (c9e17d79ea).
- Kernel cache eviction keeps the newest per arch and never the current entry; no cleanup target reaches the kernel caches; the disk-full page says why (afa87ac498).
- Native-only kernel builds: foreign arch refused, QEMU without hvf or kvm refused (179ca326f2).
- Provenance names the verified source URL and digest (33d6ff0a4e); every image carries `/etc/linux-gpl-notice` derived from it (5fcbae2a10).
- `ze doctor` check `appliance-runtime-kernel` (1927823c9f), kept out of the kernel cache hash (3fa832a60c).
- Harness and toolchain fixes the boot proofs needed: arm64 guest NIC (1ec402852a), gok error reaching stderr (0a221fb741), gok on the host toolchain (0313f16ee7), fixtures on `runtime.GOARCH` (ecd9a892ba), the QEMU proofs naming a failed step's exit status and finding Homebrew's keg-only e2fsprogs through the build's own resolver (ed5d5e5310).
- Review round 1 fixes: see the Review Gate's round-1 table.

### Bugs Found/Fixed
- arm64 QEMU guest had an e1000 NIC its kernel builds as a module: `TestTheArm64MachineGetsTheNICItsKernelBuildsIn`.
- gok's `ERROR:` line lost to the crashlog relay on `os.Exit`: `TestGokExitReachesStderr`.
- assembled kernel package lacked `cmdline.txt` and `config.txt`: `TestAssembleKernelPackageWritesCmdline`.
- le's `GOTOOLCHAIN=go1.27.0` pin broke every gok go call: `TestGokBuildsWithTheHostToolchain`.
- `kernel-builder-single-driver` and `kernel-wiring` fixtures red on arm64 after AC-13: fixed in ecd9a892ba.
- 1927823c9f staled every cached kernel: `TestKernelBuilderSourcesSkipTheDoctorQuery`.
- Recorded, not fixed (predates this spec): relative `ZE_APPLIANCE_DIR` loses the image to in-process gok's `os.Chdir`, journal row 6c4e143c3e.

### Documentation Updates
- `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `docs/architecture/appliance/gokrazy-build-pins.md`, `kernel-profiles.md`, `build-artifacts.md`, `docs/contributing/running-commands.md` ("When the disk is full"), `docs/labs/l2tp-interop.md`, `ai/INDEX.md`, `ai/rules/platform-linux.md` and its two appliance-dependency-bump points; `features/installation.md` (default image boots ze's kernel, GPLv2 notice).
- `./le doc check all` (2026-10-11, `scratch/r11close-doccheck.log`): every source anchor of this spec's pages resolves (3025 anchors across 23 digests); the run exits 1 on two claims in `docs/guide/config-editor.md` (`insertRefStaleConflict`, `memberToggleStaleConflict` in `internal/component/cli/editor_commit.go`), another session's uncommitted CLI work, not this spec's. Round 11 NOTE 3: `kernel-profiles.md` now names the host probe limit (`hostProbeTimeout`), and `availableAccelerators` and `dockerRootDir` word it through `LimitWords`.

### Deviations from Plan
- `ze.gok.kernel-package` deleted rather than kept (owner decision 2026-10-09); AC-6 and the design row rewritten.
- Assembler lives in `internal/appliance/instance/kernelpkg.go` and writes the module files itself; no tracked `gokrazy/kernel/package/` skeleton and no tracked digest file beside `kernel.version` (the pin is `kernelSourceSHA256` in `worker.go`).
- AC-13 to AC-17 and AC-19 were added by the owner's second round of decisions.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | `UsableBuilder` added to `kernelbuilder/driver.go` for the doctor check (1927823c9f) | every non-test file in `kernelbuilder` is hashed into the cache variant, so the edit staled every cached kernel (a 40G cold rebuild was asked for) | the next arm64 build refused for disk space | moved to `kernelbuilder/doctor.go`, excluded from the hash, test red first (3fa832a60c) |
| assumption | the harness NIC suits every guest arch | arm64 builds e1000 as a module and gokrazy loads none | first arm64 boot booted with no SSH answer | `virtio-net-pci` for arm64 (1ec402852a) |
| assumption | in-process gok inherits a usable go toolchain | le's `GOTOOLCHAIN` pin plus `GOPROXY=off` refused every go call | first build after the cold rebuild | `SetGokGoEnv` sets `GOTOOLCHAIN=local` (0313f16ee7) |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| default image boots ze's own runtime kernel | Done | `resolveBuildParentDir` (`internal/appliance/kernelargs.go`), `RuntimeKernelTree` (`runtimekernel.go`) | Historical arm64 boot plus native amd64 default-image readback and KVM boot below; owner-run default-appliance L2TP/IPv4 PPP proof passed on 2026-10-28 |
| rtr7 leaves the build | Done | `TestNoRtr7KernelReference` | |
| `ze.gok.kernel-package` deleted | Done | `TestRunHasNoKernelOverride` | |
| build natively, never emulated | Done | `validateRequest`, `qemuArgs` (`kernelbuilder`) | |
| cache never deleted by reclamation | Done | `evictKeepN`, `TestCleanTargetsNeverReachTheKernelCache` | |
| GPLv2 notice in every image | Done | `addLinuxNotice` (`instance/kernelpkg.go`) | read back from a built image |
| latest stable 7.x pinned by digest | Done | `internal/appliance/kernel.version`, `kernelSourceSHA256` | 7.2.9 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Native amd64 default-image readback has both MPLS symbols built in; `police-default-readback.json` | Kernel bytes match cache `...-5db21ebf-34cba457` |
| AC-2 | Done | Same image: 150/150 required built-ins; native boot console identifies `7.2.9-ze` | Includes POLICE; earlier 128/149-symbol results are historical |
| AC-3 | Done | Native amd64 `mpls-boot` verdict `pass`, KVM, CLI JSON answer `[]`; `police-mpls-boot.log` | Startup and CLI proof, not LDP forwarding traffic |
| AC-4 | Done | `TestNoRtr7KernelReference` | |
| AC-5 | Done | `TestPrepareRefusesWithoutKernelPackage` | |
| AC-6 | Done | `TestRunHasNoKernelOverride` | rewritten to the owner decision |
| AC-7 | Done | `TestResolveBuildParentDirUsesRuntimeKernel`; arm64 build served the cache entry with no kernel build | |
| AC-8 | Done | `TestDownloadKernelSourceVerifiesDigest` | |
| AC-9 | Done | `TestKernelVersionHasDigestPin` | |
| AC-10 | Done | `TestColdOfflineCacheNamesRemedy` | |
| AC-11 | Done | arm64 build passes gokrazy's arch check and boots | |
| AC-12 | Done | `TestEveryRuntimeConfigSymbolIsAssertedOrAcknowledged`, `TestUnverifiedRuntimeSymbolsHasNoStaleEntries` | |
| AC-13 | Done | `TestValidateRequestRefusesForeignArch`, `TestQEMUArgsRefusesWithoutHardwareAccelerator` | |
| AC-14 | Done | `TestCleanTargetsNeverReachTheKernelCache` | |
| AC-15 | Done | `TestEvictKeepsNewestPerArchAndTheCurrentEntry` | |
| AC-16 | Done | `docs/contributing/running-commands.md`, "When the disk is full" | |
| AC-17 | Done | `TestLinuxNoticeMatchesTheBuiltKernel`, `TestPrepareCarriesTheLinuxNotice`; notice read back from `ze-20261010-140153.img` | |
| AC-19 | Done | `kernel.version` 7.2.9, `TestKernelVersionHasDigestPin` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestResolveBuildParentDirUsesRuntimeKernel` | Done | `internal/appliance/runtimekernel_test.go` | file moved from plan |
| `TestPrepareRefusesWithoutKernelPackage`, `TestPrepareReplacesZeKernelModule`, `TestAssembleKernelPackage` | Done | `internal/appliance/instance/kernelpkg_test.go` | |
| `TestDownloadKernelSourceVerifiesDigest` | Done | `internal/appliance/kernelbuilder/worker_test.go` | |
| `TestKernelVersionHasDigestPin` | Done | `internal/appliance/cmd_kernel_test.go` | |
| `TestColdOfflineCacheNamesRemedy` | Done | `internal/appliance/runtimekernel_test.go` | |
| `TestRunResolvesRuntimeKernelByDefault` / `TestRunHonorsKernelPackageOverride` | Changed | `internal/le/build/gokrazy/gokrazy_test.go` | override test replaced by `TestRunHasNoKernelOverride` |
| `TestPrepareRealInstanceCarriesEveryModule`, `TestPreparedModulesResolveIdenticallyToTracked`, `TestNoRtr7KernelReference` | Done | `internal/appliance/instance/prepare_repo_test.go` | |
| `mpls-boot-test` | Done | `./le test qemu mpls-boot` | Historical arm64 PASS; fresh amd64 KVM PASS on the POLICE kernel |
| `vpp-hugepages-test` | Done | `./le test qemu vpp-hugepages` | Fresh amd64 KVM PASS, 64 two-MiB pages; no VPP traffic claim |
| `gokrazy-l2tp-ppp-test` | Done | `./le test deployment gokrazy-l2tp-ppp`, invoked through the retained patched binary | Owner-run native amd64 KVM PASS at 15:04 UTC, 2026-10-28; peer kernel addresses, PPP traffic, route withdrawal and peer kernel cleanup asserted; UFW active with no lab rules afterward |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `gokrazy/ze/config.json` | Done | `ze.invalid/kernel` |
| `internal/appliance/instance/prepare.go` | Done | |
| `internal/appliance/kernelargs.go`, `cmd_kernel.go` | Done | |
| `internal/appliance/kernelbuilder/worker.go`, `driver.go` | Done | |
| `internal/le/build/gokrazy/gokrazy.go` | Done | |
| `internal/le/test/deployment/gokrazyimage.go`, `gokrazykernel.go` | Done | private resolver removed in 36c851fc58 |
| `internal/appliance/kernelpkg.go` | Changed | lives at `internal/appliance/instance/kernelpkg.go` |
| `gokrazy/kernel/package/`, `kernel.sha256` | Changed | not created: the assembler writes the module, the digest sits in `kernelSourceSHA256` |
| docs and rules listed in Files to Modify | Done | see Documentation Updates |

### Audit Summary
- **Total items:** 18 ACs, 7 requirements
- **Done:** 18 ACs, 7 requirements, with historical and fresh evidence distinguished below.
- **Closure:** independent resumed review is CLEAN (see Closure resumption); all runtime criteria are complete. Post-commit scoped verification is a separate main-thread step, not a runtime acceptance gap.
- **Skipped:** none
- **Changed:** AC-6, assembler location, no skeleton, digest location (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| the default image boots ze's own runtime kernel | functional (QEMU boot) | Historical arm64 HVF evidence is recorded in the Task section. Fresh amd64 KVM `mpls-boot` passed on cache `...-5db21ebf-34cba457`, returning `[]` from the LDP CLI; hugepage boot passed with 64 two-MiB pages. See native amd64 evidence below. Neither run proves forwarding traffic. |
| the kernel is built from the tracked `runtime.config` at the same commit, carrying MPLS | data correctness | Native amd64 build produced kernel SHA-256 `4910bc4600ba8d76d6250f09aca22c8eaf2c4b4459baadf9f9e52aec27c429b4`; actual default-image kernel bytes match it, with 150/150 required built-ins and both MPLS symbols. `police-default-readback.json` records the image and cache identity. Historical arm64 128-symbol evidence remains in the Task section. |
| rtr7 leaves the build entirely | unit (tree scan) | `TestNoRtr7KernelReference` PASS 2026-10-10; `gokrazy/ze/builddir/github.com/rtr7` absent |
| the image honours GPLv2 by pointing at the exact source | data correctness | `/etc/linux-gpl-notice` extracted with `7zz` from the root squashfs of `ze-20261010-140153.img` reads Version 7.2.9, `https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.2.9.tar.xz`, SHA-256 `b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba`, equal to the cache entry's `kernel.version` provenance |
| a long cold build is acceptable because the cache is never deleted | unit | `TestCleanTargetsNeverReachTheKernelCache`, `TestEvictKeepsNewestPerArchAndTheCurrentEntry` PASS; the arm64 build at 14:01 served `...-7c0d8f79-d826e25e` with no kernel build (that entry, under an older key, was evicted since; keep-newest eviction is AC-15, reclamation never reaches the cache); the 2026-10-11 `mpls-boot-test` served `...-docker-7c0d8f79-34cba457` with no kernel build |
| kernels are built natively only | unit (negative) | `TestValidateRequestRefusesForeignArch`, `TestQEMUArgsRefusesWithoutHardwareAccelerator` PASS |
| the default kernel supports the appliance L2TP PPP proof | interop and dataplane | Owner-run xl2tpd 1.3.18 replay on native amd64 Linux 7.2.9-ze returned `proven: true`; `gokrazyL2TP.assertKernelState` checked peer addresses and two pings through PPP, and `observe` required route withdrawal and `awaitTeardown` restoring the peer kernel baseline. See the 2026-10-28 passing receipt below. |

## Work Not Done

No in-scope implementation or runtime proof remains unassigned.

The former rows mixed two different boundaries into unfinished scope:
N100 hardware evidence was explicitly excluded by the owner ("ignore n100
atm"), and scoped verification is the post-commit step of `ze-close`, not an
acceptance item to transfer to another spec. Neither is silently marked
passed. Verification debt remains derived from git, not stored in this table.
The already-recorded selector defect belongs to
`plan/post-release/engineering/spec-verify-scope-0-umbrella.md` (commit
`bfc1ec9876`); closure neither repairs it nor repeats its unbounded run.

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|

## Review Gate

The latest verdict is the independent resumed closure pass of 2026-10-28,
recorded under **Closure resumption** below. The table and eleven rounds
that follow are preserved historical evidence, including only the owner
authorisations actually supplied.

| Resumed gate field | Result |
|--------------------|--------|
| Artifact | `tmp/review/appliance-ships-ze-kernel-30667194-e251-46e5-93e8-56ed151de886.md`; native `spec review record`, CLEAN, one resumed pass, reviewer `ApplianceClosureFinal` |
| Hash check | Native `spec review check`: OK, 3 code files clean and hashes match (`gokrazylab.go`, `gokrazyl2tp.go`, `pppstate.go`); five closure/citation Markdown files also named |
| Scope and findings | Closure resumption below; three lenses, 0 BLOCKER, 0 ISSUE; record-only NOTE corrected without another round |
| Citation population | Whole tracked-tree roots searched, with ignored-heavy roots narrowed to their tracked files: four external citer files updated, no Go Design-header hit. The generated ignored `plan/release/roadmap.md` is not part of the tracked population. |
| Journal sweep | Only foreign `plan/journal/declared-format-contradicts-payload.md` is dirty; no journal row was authored by this closure or left out of its commit. |

| Field | Value |
|-------|-------|
| Artifact | latest: round 11 over e440a9fe58, da76ba1fe5, d2902fc19d, `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-r11-appliance-ships-ze-kernel.md` (CLEAN: 0 BLOCKER, 0 ISSUE, 4 NOTE), probes in `scratch/r11/`, every NOTE answered in "Round 11 fixes"; recorded with `./le spec review record` after the round-11 code edits were committed. Round 10: `scratch/review-r10-appliance-ships-ze-kernel.md` (0 BLOCKER, 1 ISSUE, 3 NOTE). Earlier rounds: round 1 `scratch/review-r1-appliance-ships-ze-kernel.md` (0 BLOCKER, 6 ISSUE, 8 NOTE), rounds 2 to 9 in the Rounds row |
| `./le spec review check` | 2026-10-11 after 6cada4212d: `review_gate: OK (3 code files, clean, hashes match tmp/review/appliance-ships-ze-kernel-5620b26f-603e-4d57-826d-6ef92b7fcd64.md)`; the artifact covers 9 code files (the round-10 fix files and the round-11 edits) and is gitignored under `tmp/`, so it is not committed |
| Rounds | 11 completed, round 11 CLEAN. Round 1 was preceded by a partial pass the owner interrupted, whose ISSUE is row 1 below; round 2 artifact `scratch/review-r2-appliance-ships-ze-kernel.md` (0 BLOCKER, 3 ISSUE, 6 NOTE), fixes in "Round 2 fixes"; round 3 artifact `scratch/review-r3-appliance-ships-ze-kernel.md` (0 BLOCKER, 4 ISSUE, 9 NOTE) over dc501faa9f, 5ffef6eedc, 248d1ed071, 8949969c96, 9bcaef2d4e, fixes in "Round 3 fixes"; round 4 over 590760235b, e308e4cdb2, b3f4b0c3bb (1 BLOCKER, 5 NOTE, findings and mutation evidence in `scratch/r4mut/`), fixes in "Round 4 fixes"; round 5 over 2182b2c521 (0 BLOCKER, 2 ISSUE, 4 NOTE, mutation evidence in `scratch/r5mut/`), fixes in "Round 5 fixes"; round 6 over f5648a798d, 466fe5a94e, 4de8913046, 9990605cb9 (0 BLOCKER, 1 ISSUE, 3 NOTE, mutation evidence in `scratch/r6mut/`), fixes in "Round 6 fixes"; round 7 over b8490bde67, 691ede5314 (0 BLOCKER, 2 ISSUE, 3 NOTE, artifact `scratch/review-r7-appliance-ships-ze-kernel.md`, probes in `scratch/r7/`), fixes in "Round 7 fixes"; round 8 over f1b437f7c8, 7629b7c783 (0 BLOCKER, 3 ISSUE, 3 NOTE, artifact `scratch/review-r8-appliance-ships-ze-kernel.md`, probes in `scratch/r8/`), fixes in "Round 8 fixes"; round 9 over 42eb15b049, 1a0294116d (0 BLOCKER, 2 ISSUE, 1 NOTE, artifact `scratch/review-r9-appliance-ships-ze-kernel.md`, probes in `scratch/r9/`), fixes in "Round 9 fixes"; round 10 over 885e6979dd, 8cb1f9aa7c (0 BLOCKER, 1 ISSUE, 3 NOTE, artifact `scratch/review-r10-appliance-ships-ze-kernel.md`, probes in `scratch/r10/`), fixes in "Round 10 fixes"; round 11 over e440a9fe58, da76ba1fe5, d2902fc19d (CLEAN: 0 BLOCKER, 0 ISSUE, 4 NOTE, artifact `scratch/review-r11-appliance-ships-ze-kernel.md`, probes in `scratch/r11/`), NOTEs answered in "Round 11 fixes" |
| Rounds reason | round 5 found two product defects: the unsupported-arch refusals of `ze appliance kernel --arch` and of `image.arch` carried a hardcoded list and no fix, so the round-4 refusals were unreachable from either entry point (ISSUE 2); and a cancel during the QEMU SSH bootstrap was reported as "SSH never answered" (NOTE 2). Round 5 also found the key guard's known-gap list incomplete (ISSUE 1). Round 6 found one product defect: `ze appliance kernel <name>` took an unsupported `image.arch` from a config it never validated, and refused it as an `--arch` the operator never typed, with a fix that dropped the name and never named the config file (ISSUE 1); and a stop in either pause after the QEMU login prompt was reported as a bare context error (NOTE 1). Round 7 found two product defects, each a sibling of a round-6 fix: `ze appliance iso <name>` refused an unsupported `image.arch` without naming the config file (ISSUE 1), and a stop during the guest kernel build, including the 4-hour `buildTimeout`, was reported as `kernel build failed: signal: killed`, which reads as a compile failure and gives no next step (ISSUE 2). Round 8 found the same class open on the Docker backend and the QEMU worker build, where a stop read `kernel container failed: signal: killed` (ISSUE 1); the build limit declared twice, so the 4-hour limit round 7 reported could not be reached from the CLI and the 2-hour one the CLI meets was never named (ISSUE 2); and the guest-build deadline sending the operator to a QEMU command that boots no build (ISSUE 3). Round 9 found the class still open on the downloads, where net/http answers `context.Cause(ctx)`, so Ze's limit read as the limit's own noun phrase with no step (ISSUE 1); and a Ctrl-C on the kernel container step dropping the line that finds a container still compiling (ISSUE 2). Round 10 found Build's backstop call untested, every gate package green with it removed (ISSUE 1), and `SelectBuilder`'s error outside it, so a Ctrl-C during the hvf probe read as a refusal of the host (NOTE 1) |
| Owner authorised | round 6: Thomas, 2026-10-11, "Authorise round 6"; round 7: Thomas, 2026-10-11, "Authorise round 7"; round 8: Thomas, 2026-10-11, "Authorise round 8"; round 9: Thomas, 2026-10-11, "Authorise round 9"; round 10: Thomas, 2026-10-11, "Authorise round 10"; round 11: Thomas, 2026-10-11, "Authorise round 11" |
| Reviewer lenses used | round 1: logic+wiring (resolver to image), supply chain (digest pin, notice), cache safety; rounds 2 to 6: no lens recorded in their artifacts; round 7: discrimination of the round-6 fixes (`-overlay` against the pre-fix files), sibling entry points of each fix, operator-facing errors (`ai/rules/user-facing-errors.md`), cache key; round 8: discrimination of the round-7 fixes, the stop class on the Docker backend, the reachability of each limit from the CLI, the next step each stage gives, cache key; round 9: discrimination of the round-8 fixes, the stop class on the download stages, the wrapping of Ze's limit under a caller's deadline, the truth of the ssh advice, cache key; round 10: the stop class through the backend entry (`runQEMU` plus `stopReport`), discrimination of the backstop's call site, mislabel and double-wrap of `stopReport`, curl targets against the reader paths, the liveness of `defaultHTTPGet`, cache key; round 11: discrimination of each round-10 fix (three `-overlay` mutations, each red only under its own), what Build prints under each stop (probe), `LimitWords` values and edges, doc-to-code match for the two changed pages, cache key and the entry the tree keys now, whole-spec owed list |

### Round 1 fixes
| Finding | Fixed by | What changed |
|---------|----------|--------------|
| ISSUE 1 (accelerator listed is not usable) + NOTE 5 (Alpine ISO fetched before the accelerator check) | 6aa76ad8a0 | `hardwareAccelerator` probes `/dev/kvm` and `kern.hv_support`, `SelectBuilder` runs it, `runQEMU` asks before `ensureAlpineISO`, QEMU stderr kept in the boot failure |
| ISSUE 2 (cold-cache fix impossible across arches) + ISSUE 6 (arm64 examples with no host condition) | 28e49feda7 | foreign-arch cold cache refused before building with a fix that names building on that arch or setting `image.arch`; `docs/guide/appliance.md` examples carry the host condition |
| ISSUE 3 (missing or old go found after the kernel build) | 04aec23668 | `checkGokHost` / `CheckGoToolchain` run before the kernel is resolved, in `ze appliance build` and `./le build gokrazy` |
| ISSUE 4 (cache key hashes code no build runs) | 6aa76ad8a0 | provenance reader and notice moved to `internal/appliance/instance/provenance.go`; `kernelBuilderSources` hashes every non-test kernelbuilder file with no exclusion |
| ISSUE 5 (errors with no fix) + NOTE 8 (nonexistent KernelTree untested) | 388c67ac73 | provenance, vmlinuz-open and magic refusals print the command that deletes the cache entry; notice collision names the config entry; tests for each |
| NOTE 1 (`KernelCacheNamespaces` had no production caller) | 95c7a2a05f | `cleanTargets` skips any target within or holding a kernel cache, and the store trim now honours that skip too (`TestTrimSkipsATargetHoldingTheKernelCache`, red first) |
| NOTE 2 (spec drift) | this spec commit | Data Flow, Integration Points, Wiring and TDD rows match the code; AC-1/AC-2 symbols re-checked on `...-d826e25e`; AC-18 gap stated; ed5d5e5310 in the Implementation Summary |
| NOTE 3 (notice names no Ze commit) | this spec commit | decision recorded after the AC-13..AC-19 table: no commit in the notice, because the cache key hashes working-tree inputs |
| NOTE 4 (nightly CI kernel cache key omits `internal/appliance/kernelbuilder/**`) | this spec commit | one row in `plan/journal/stale-artifact-reused.md`; predates the spec, not fixed (the workflow is outside this spec) |
| NOTE 6 (arm64 proofs need the firmware path by hand) | 3271485c9d | `newHugepages` defaults to `hostArm64BIOS`, the package's own `Run.arm64BIOS` lookup; nothing copied from kernelbuilder |
| NOTE 7 (`WorkerRequest.SourceSHA256` also set by `install_fixture.go`) | 0e887b69de | a correction to the record, no defect; the install fixtures now ask for the host arch and assert the foreign arch refused |
| Doctor `.ci` header and fix-line assertion | 8257fcace4 | `test/ui/doctor-appliance-runtime-kernel.ci` asserts the source-tree remedy; passed under `./le test ui` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `availableAccelerators` swallowed the `<qemu> -accel help` exec error and returned an empty set, so a missing or crashing QEMU was reported as "offers neither hvf nor kvm" | `internal/appliance/kernelbuilder/qemu.go` | 45834f22d7: returns the error with exit status and QEMU's stderr; `TestQEMUArgsNamesABrokenQEMU` red before the fix. qemu.go is hashed into the runtime kernel cache variant, so every cached runtime kernel (both arches) rebuilds on next use |

### Round 1 addendum: found during the amd64 build, not by the reviewer
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 2 | ISSUE | `dockerStoreHostPath` returned `/var/lib/docker` on native Linux whenever it existed, without asking the daemon. On the amd64 host (2026-10-10) the daemon's root is `/home/thomas/.cache/docker` (295G disk, 118G free) and an empty `/var/lib/docker` sits on the 98G root disk (20G free), so the build was refused with a silently wrong reading. The refusal also carried no fix command on a line of its own | `internal/appliance/kernelbuilder/space.go`, `driver.go` | 6ae709558c: on Linux the guard asks `docker info --format {{.DockerRootDir}}` through `runCommandOutput`, measures `/var/lib/docker` only when the daemon cannot answer or names a missing path, and says which path it measured and why; the refusal ends with `df -h <path>` and `ze appliance kernel --builder qemu ...` on their own lines; the macOS VM-disk branch is unchanged. `TestDockerStoreHostPathAsksTheLinuxDaemon` and `TestCheckKernelBuildSpaceMeasuresTheDaemonRoot` red with the daemon query disabled, green with it. No page described the check, so no page edit. kernelbuilder files are hashed into the cache variant, so both cached runtime kernels rebuild |
| 3 | NOTE | lint `misspell`: "honours" in the comment of `TestTrimSkipsATargetHoldingTheKernelCache` | `internal/le/scratch/cacheclean_kernel_test.go` | e9ab56300d. The other finding in the same lint log, a `godot` in `internal/test/fixture/plugin_fixture_rfc2545_joint_subnet_frr_oracle_linux.go`, is in another session's RFC 2545 work, not this spec's files, and was left alone |

Ready for independent review round 2, over the round-1 fix commits and 6ae709558c, e9ab56300d. Scoped lint (`./internal/appliance/... ./internal/le/scratch/...`): 0 issues on darwin and linux-integration, 2026-10-10.

### Round 2 fixes
Artifact: `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-r2-appliance-ships-ze-kernel.md` (0 BLOCKER, 3 ISSUE, 6 NOTE). Every round-1 finding was found fixed at the producer.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| NOTE 1 (cache key hashed host-side kernelbuilder code that decides no kernel bytes; `notBuildTime` was a hand list) | dc501faa9f | `kernelbuilder.BuildSources` names `recipe.go` (new: `docker run` argv, QEMU guest Alpine release, packages and command, `normalizeRequest`, `writeProvenance`, `workerGOARCH`) and `worker.go`; `kernelBuilderSources` hashes those, every file under `tools/kernel-builder`, and the transitive Ze import closure (`zeImportClosure`, module path read from `go.mod`). `TestKernelCacheKeyCoversEveryBuildInput` (both mutations red) and `TestKernelCacheKeyIgnoresHostSideEdits`; `TestKernelBuilderSourcesHoldOnlyBuildCode` removed with its row in `test/weakened/d1987d95.md`; `build-artifacts.md` states the rule; journal row for the unpinned, unkeyed i915 firmware in `plan/journal/stale-artifact-reused.md` |
| ISSUE 1 (QEMU stderr tail read before `cmd.Wait`) + NOTE 6 second half (stderr lost when the VM dies after login) | 5ffef6eedc | `stopQEMU` waits up to `qemuExitGrace`, then reads the tail (test red with read-before-Wait); SSH and build failures carry QEMU's stderr |
| ISSUE 3 (refusals without the exact fix command on its own line) | 5ffef6eedc, 248d1ed071 | a silent QEMU prints its exact command line on its own line (5ffef6eedc); `foreignArchKernelError` names the instance file `ConfigPath(dir, name)` and ends with `ze appliance --dir <dir> build <name>`, the offline cold-cache command sits on its own line, and `buildOne` refuses through `runtimeKernelReachable` before building (248d1ed071) |
| ISSUE 2 (no `ze doctor` check calls `CheckGoToolchain`) | 8949969c96 | doctor check `appliance-go-toolchain` (diagnostic `doctor-appliance-go-toolchain`); `TestDoctorGoToolchainReportsAnOldGo` |
| NOTE 2 (host arch declared twice) | 248d1ed071 | `kernelbuilder.HostArch` is the single declaration, used by `resolveRuntimeKernel` |
| NOTE 3 (no timeouts on host probes) | 5ffef6eedc | `hostProbeTimeout` (15 s) bounds `-accel help`, the KVM/HVF probes and `docker info` |
| NOTE 4 (`usermod -aG kvm` offered for every open error) | 5ffef6eedc | the group fix is given only for `fs.ErrPermission` |
| NOTE 5 (Docker-root edge cases; no page documented the guard) | recorded; page 8949969c96 | not changed in code: with a remote `DOCKER_HOST` the path the daemon names is measured on this host, the wrong machine, and the guard does not detect it; a daemon that does not answer falls back to `/var/lib/docker` and the refusal says why. Both are now stated in `build-artifacts.md`, "Free space before a Docker kernel build" (8949969c96) |
| NOTE 6 first half (i915 firmware downloaded before a foreign-arch refusal) | 248d1ed071 | `kernelbuilder.CheckArch` runs before `ensureFirmware` (test red with the check removed) |

Docker-root evidence on a real host: measured 2026-10-10 20:15 on `ssh claude` (amd64), with 6ae709558c the free-space guard passed and the build reached the container, confirming the round-1 addendum row 2 fix outside tests.

-> Decision: the kernel cache key covers only the inputs that determine the kernel's bytes (`kernelbuilder.BuildSources`, `tools/kernel-builder`, their Ze import closure, the version, fragments, manifests and patches). Host-side code (space checks, accelerator probes, error text) is left out, so editing it keeps a thirty-minute cached build. `TestKernelCacheKeyCoversEveryBuildInput` guards the inclusion side.

Both cached runtime kernels (arm64 `...-d826e25e` and the amd64 entries) are stale since dc501faa9f; the next build of each arch rebuilds, and the arm64 config owes the 128-symbol check again.

Ready for independent review round 3, over dc501faa9f, 5ffef6eedc, 248d1ed071, 8949969c96.

### Round 3 fixes
Artifact: `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-r3-appliance-ships-ze-kernel.md` (0 BLOCKER, 4 ISSUE, 9 NOTE), over dc501faa9f, 5ffef6eedc, 248d1ed071, 8949969c96, 9bcaef2d4e.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| ISSUE 1 (key guard one-way: methods, types and host-side code that writes what the build receives went unchecked) | 590760235b, e308e4cdb2 | `recipe.go` holds `Request`, `containerBuildArgs` (the `docker build` argv), `guestWorkerBuild` (the worker's compile argv and env), `normalizeRequest` (the request rewrite and defaults) and `targetModules`; the appliance passes no builder dir, common dir or modules. `TestKernelCacheKeyCoversEveryBuildInput` reads `go/types` uses, so a type, method or field reference to host-side code is caught; `TestHostSideNeverWritesTheRequest`, exact argv in `TestRunDockerArgvAndOwnershipRepair`, `TestBuildGuestWorkerTakesTheRecipe`, `TestTargetDecidesModules`; 8 mutations red (`scratch/r3fix/mut.log`, `mut7.log`); weakened row for the guard's rewrite. e308e4cdb2 is the lint pass over `recipe.go` (`buildVerb`), the last keyed edit |
| ISSUE 2 (backend shares a key; page said firmware was the only unkeyed input) | 590760235b | `kernelCacheVariantFor` keys `kernelbuilder.PreferredBackend(builder)`; `TestKernelCacheKeyCarriesTheBackend`. `build-artifacts.md` carries the key table and a not-in-key table (firmware, unpinned `FROM` images, apt, apk, the fetched ISO checksum); journal row in `plan/journal/stale-artifact-reused.md` for the unpinned toolchain |
| ISSUE 3 (foreign-arch fix filled another host's cache; the gokrazy image command never pre-checked) | b3f4b0c3bb | `GokrazyRuntimeKernelReachable` runs in `prepareArgs` before the kernel resolver; `foreignArchKernelError.forGokrazy` ends with `GOARCH=<host>` and the same args, shell-quoted; `TestForeignArchRefusedBeforeTheKernel` |
| ISSUE 4 (SSH never answered, no command) | b3f4b0c3bb | `errSSHNeverAnswered` ends with the QEMU command that boots the same VM and the `sshd -d` hint |
| NOTE 1 (an exited QEMU reported as a boot timeout) | b3f4b0c3bb | `errBootFailed` takes the process state and reports an exit with its status |
| NOTE 2 (hashing failure fell back to an unhashed key) | 590760235b | `kernelCacheVariantFor` returns an error with its fix; no fallback key; `TestKernelCacheKeyRefusesWhatItCannotHash` (3 steps) |
| NOTE 3 (third-party imports and `go.sum` not keyed) | recorded | not changed: `recipe.go` and `worker.go` import only the standard library and `internal/core` `textbuf` and `tmplink`, so no third-party package reaches the build today. A future third-party import owes `go.sum` in the key |
| NOTE 4 (var initializers with side effects in host-only imports unchecked) | recorded | not changed: the guard checks `init` only, and no host-only import compiled into the worker has a side-effecting var initializer today |
| NOTE 5 (no replacement for "hashed files hold no non-build code") | recorded | not changed: host-side code in a hashed file costs a needless rebuild, never a wrong kernel, so correctness does not depend on it |
| NOTE 6 (CheckArch refusals named no command; `--dir` unquoted) | b3f4b0c3bb | refusals end with `ze appliance kernel --arch <host>`; `forInstance` quotes dir and name through `shellArg` |
| NOTE 7 (`runtime.GOARCH` for the installer kernel lookup) | 590760235b | `cmd_iso.go` uses `kernelbuilder.HostArch` |
| NOTE 8 (doctor code outside `codes.go`; any `go.mod` in the working directory trusted) | b3f4b0c3bb; first half recorded | `goDirective` refuses a `go.mod` whose module is not Ze's, read from the appliance package's import path. The code stays in `doctor_checks.go` with its five appliance siblings, none of which is in `internal/core/diagnostic/codes.go`; e308e4cdb2 names it once as `doctorKernelCode` |
| NOTE 9 (a canceled build reported as a hung probe) | b3f4b0c3bb | `availableAccelerators` and `dockerRootDir` say the kernel build was canceled on `context.Canceled` |

-> Decision: the build backend is part of the key, since Docker (Debian bookworm gcc) and QEMU (Alpine 3.21 gcc) compile different bytes. Toolchain drift upstream (unpinned images and packages) is journaled, not pinned: pinning is outside the owner's agreed scope for this spec.

Cache key final at e308e4cdb2 (last commit touching `recipe.go`, `worker.go` or `tools/kernel-builder`). Both cached runtime kernels are stale again since 590760235b; the next build of each arch rebuilds.

Scoped lint (`./internal/appliance/... ./internal/le/build/gokrazy/...`): 0 issues on darwin and linux-integration, 2026-10-10 (`scratch/r3fix/lint5.log`). `internal/appliance`, `internal/appliance/kernelbuilder` and `internal/le/build/gokrazy` pass `-race` with the gate tags (`scratch/r3fix/t4.log`).

Ready for independent review round 4, over 590760235b, e308e4cdb2, b3f4b0c3bb.

### Round 4 fixes
Review over 590760235b, e308e4cdb2, b3f4b0c3bb: 1 BLOCKER, 5 NOTE. Mutation evidence `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/r4mut/` (`out_me_runcommand_argv.log`, `out_md_worker_env.log`, `out_mb_reverse_call.log` green under a build-changing mutation). No fix touches `recipe.go`, `worker.go`, `tools/kernel-builder`, `internal/core/textbuf` or `internal/core/tmplink`, so the cache key is unchanged from e308e4cdb2 and the arm64 build keyed on it stays valid.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| BLOCKER 1 (the return-direction guard claimed more than it proves) | 2182b2c521 | Owner-thread decision (a): narrow the claim. The `TestHostSideNeverWritesTheRequest` comment and `build-artifacts.md` now say what is checked (assignment, increment, pointer into, own `Request` literal, the `sshRun` command; argv and env up to the `runCommand` and `runWorkerBuild` seams) and list four unguarded channels: arguments appended inside the real `runCommand`, env added or inherited through `os.Environ` in `runWorkerBuild`, `Request` slices changed by a call, the QEMU console bootstrap. Row in `plan/journal/stale-artifact-reused.md`. `recipe.go`'s `normalizeRequest` comment ("the only code that writes a Request field") still reads wider than the guard; it is a keyed file, left for after the arm64 build |
| NOTE 1 (an unsupported arch sent to "build it on a host of that arch") | 2182b2c521, corrected in round 5 | `runtimeKernelReachable` refuses an arch `kernelbuilder.ArchSupported` rejects (derived from `dockerPlatforms`) before the cache is asked, and `kernelbuilder.SupportedArches` names the arches. Round 5 ISSUE 2 found this row overclaimed: `runKernel` and `applianceConfig.Validate` refused such an arch earlier with their own hardcoded "must be amd64 or arm64" and no fix, so only the `./le build gokrazy` refusal of the three was reachable, and the test drove the error values, not the entry points. Fixed in round 5 (below) |
| NOTE 2 (lookups key builder "", so a `--builder` kernel of the other backend is invisible) | 2182b2c521 | documented in `build-artifacts.md` (image build, `./le build gokrazy`, ISO, doctor) and on `runtimeKernelReachable`; behaviour unchanged |
| NOTE 3 (no `.variant` writer since eae2825926; `installerKernelBuildMatches` / `installerKernelFallbackPath` dead outside tests) | recorded | row in `plan/journal/parameter-no-caller-ever-fills.md`; not deleted, an owner decision |
| NOTE 4 (a canceled build during the boot wait reported as a boot timeout) | 2182b2c521 | `errBootFailed` (`qemu.go`, host-side, not in `BuildSources`) takes the context error and says the build was canceled, or ran out of its caller's time, before any stderr or exit wording. `TestCanceledBootWaitIsNotATimeout`, red with the branch disabled (`scratch/r4fix/red.log`) |
| NOTE 5 (research cache path lacks the backend) | 2182b2c521 | Current Behavior, Question 1, names `<backend>` and says which earlier entries predate it |

`go test -race` with the gate tags over `./internal/appliance/... ./internal/le/build/gokrazy/...`: all four packages ok (`scratch/r4fix/gate.log`). Owed to the main thread: scoped lint over the same packages.

Ready for independent review round 5, over 2182b2c521.

### Round 5 fixes
Review over 2182b2c521: 0 BLOCKER, 2 ISSUE, 4 NOTE. Mutation evidence `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/r5mut/`; red logs for the fixes in `scratch/r5fix/`. No fix touches `recipe.go`, `worker.go`, `tools/kernel-builder`, `internal/core/textbuf` or `internal/core/tmplink`, so the cache key is unchanged from e308e4cdb2 and the cached builds keyed on it stay valid. The owner authorised round 6 on 2026-10-11.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| ISSUE 1 (the known-gap list of the key's return guard was incomplete) | f5648a798d | `build-artifacts.md`, the `TestHostSideNeverWritesTheRequest` comment and the journal row in `plan/journal/stale-artifact-reused.md` add a `Request` slice written through a local alias (`f := req.Fragments; f[0], f[1] = f[1], f[0]` in `runDocker`), the host-computed arguments to `guestBuildCommand` (the guard matches the callee name only) and the body of `sshRun`; the slice row reads "through an alias or a call"; each list says it is what mutations found, not a bound |
| ISSUE 2 (the early arch refusals bypassed the round-4 unsupported-arch refusal and carried no fix) | 466fe5a94e | `runKernel` and `applianceConfig.Validate` check `kernelbuilder.SupportedArches`, name the arches Ze builds through `unsupportedArch`, and end with `ze appliance kernel --target <target> --arch <host>` or the `"image": {"arch": "<host>"}` line; the `--arch` help derives its list from the same call; the now-unreachable unsupported branches of `foreignArchKernelError.Error` and `forInstance` are deleted, `forGokrazy` keeps its own (GOARCH reaches `runtimeKernelReachable` unchecked). `TestUnsupportedArchRefusedBeforeAnyHost` drives `runKernel` and `buildOne`, red with either check disabled (`scratch/r5fix/red-kernel.log`, `red-config.log`). The page sentence and the round-4 NOTE 1 row are corrected |
| NOTE 1 (`normalizeRequest` comment in `recipe.go` overclaims) | f5648a798d (recorded) | row in `plan/journal/stale-artifact-reused.md`: fix at the next keyed edit of `recipe.go`, since an edit there moves the cache key |
| NOTE 2 (a cancel during the `SSHD_READY` bootstrap loop reported as "SSH never answered") | 4de8913046 | the loop returns `errBuildStopped` at once when the build's context ended, naming the stage; `TestCanceledSSHSetupIsNotAnSSHFailure`, red with the check removed (`scratch/r5fix/red-note2.log`) |
| NOTE 3 (the `DeadlineExceeded` branch of `errBootFailed` untested and with no fix) | 4de8913046 | `errBuildStopped`, shared with `errBootFailed`, ends a build out of its caller's time with the QEMU command that boots the same VM; `TestBootWaitOutOfTimeEndsWithTheQEMUCommand`, red with the old wording (`scratch/r5fix/red-note3.log`) |
| NOTE 4 (`refusal_r4_test.go` named after a review round) | 466fe5a94e, 4de8913046 | `internal/appliance/unsupported_arch_test.go`; the kernelbuilder file split into `qemu_stopped_test.go` and `driver_test.go`; moves recorded in `test/weakened/d1987d95.md` |
| Coordinator addition (stale source anchors from dc501faa9f) | 9990605cb9 | `Request.Root` and `guestSetup` anchors point at `recipe.go`; `./le doc check all` no longer reports them. The seven other stale anchors it reports belong to other areas |

`go test -race` with the gate tags over `./internal/appliance/... ./internal/le/build/gokrazy/...`: all four packages ok (`scratch/r5fix/gate.log`). Owed to the main thread: scoped lint over the same packages; two runs produced no plan because other sessions' source edits changed the lint inputs mid-planning (`scratch/r5fix/lint.log`, `lint2.log`).

Ready for independent review round 6, over f5648a798d, 466fe5a94e, 4de8913046, 9990605cb9.

### Round 6 fixes
Review over f5648a798d, 466fe5a94e, 4de8913046, 9990605cb9: 0 BLOCKER, 1 ISSUE, 3 NOTE. Mutation evidence `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/r6mut/`. No fix touches `recipe.go`, `worker.go`, `tools/kernel-builder/`, `internal/core/textbuf` or `internal/core/tmplink`; `qemu.go` is host-side and not in `kernelbuilder.BuildSources`, so the cache key does not move.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| ISSUE 1 (`ze appliance kernel <name>` refused an unsupported `image.arch` as `--arch`, with a fix that dropped the name and never named the config file) | b8490bde67 | `runKernel` calls `applianceConfig.Validate` after `LoadConfig`, the one declaration of the arch check `ze appliance build` already runs; the refusal names `image.arch` and `ConfigPath(dir, name)` and ends with the `"image": {"arch": "<host>"}` line. `buildOne` now names the config file too (the sibling path). `TestUnsupportedArchRefusedBeforeAnyHost` drives `runKernel([]string{"lab"})` and asserts the field, the file and the last line, for both entry points; red against HEAD's `cmd_kernel.go` under `-overlay` |
| NOTE 1 (the 1 s and 3 s pauses after `login:` returned a bare `ctx.Err()`) | b8490bde67 | both return `errBuildStopped(ctx.Err(), "after its login prompt", ...)`; `TestStoppedAfterLoginPromptNamesTheStage` stops the build in each pause, red against HEAD's `qemu.go` under `-overlay` |
| NOTE 2 (`cliErrorf` wrote refusals to stdout) | b8490bde67 | `cliErrorf` writes to stderr (`ai/rules/cli.md`); `captureStdout` removed, its one use now `captureStderr` (row in `test/weakened/d1987d95.md`); no `.ci` test reads this output from stdout |
| NOTE 3 (the Artifact row described round 1 only) | this spec commit | the Artifact row names the latest round and points at the earlier ones |

`go test -race` with the gate tags over `./internal/appliance/... ./internal/le/build/gokrazy/...`: all four packages ok. Scoped lint (`./le go lint run scope ./internal/appliance/`): 0 issues on darwin and linux-integration, 2026-10-11.

Ready for independent review round 7, over b8490bde67.

### Round 7 fixes
Review over b8490bde67, 691ede5314: 0 BLOCKER, 2 ISSUE, 3 NOTE. Artifact `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-r7-appliance-ships-ze-kernel.md`; red proofs, overlays and gate logs in `scratch/r7fix/`. No fix touches `recipe.go`, `worker.go`, `tools/kernel-builder/`, `internal/core/textbuf` or `internal/core/tmplink`; `qemu.go` is host-side and not in `kernelbuilder.BuildSources`, so the cache key does not move.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| ISSUE 1 (`ze appliance iso <name>` refused an unsupported `image.arch` without naming the config file) | f1b437f7c8 | `resolveISOInput` wraps the `Validate` refusal with `ConfigPath(dir, name)`, as `buildOne` and `runKernel` do. `TestUnsupportedArchRefusedBeforeAnyHost` drives `buildISO("lab", ...)` and asserts the field, the file and the last line; red against HEAD's `cmd_iso.go` under `-overlay` ("iso refusal does not name the config file"). `build-artifacts.md` names `ze appliance iso <name>` among the validating entry points |
| ISSUE 2 (a stop during the guest kernel build read `kernel build failed: signal: killed`) | f1b437f7c8 | after `sshRun` fails, a stopped caller context returns `errBuildStopped(ctx.Err(), "while it built the kernel", ...)`, and an expired `buildTimeout` returns `errBuildTimedOut`, which names the limit and ends with the QEMU command to log in and watch the build. `buildTimeout` became a variable so a test can expire it. `errBuildStopped`'s deadline wording no longer says the VM is stuck, which a long build stage is not. `TestStoppedDuringGuestBuildNamesTheStage` (canceled, caller deadline, build timeout) runs a fake QEMU and a fake ssh that sleeps on the build command; all three subtests red under `-overlay` with the pre-fix branch. `kernel-profiles.md` describes the stop stages and the limit. Corrected by round 8 (ISSUE 2): the 4-hour `buildTimeout` could not be reached from `ze appliance kernel`, whose 120-minute `kernelBuildTimeout` always fired first, so `errBuildTimedOut` never ran on the CLI path. Round 8 deleted `buildTimeout` and `errBuildTimedOut`; the limit an operator meets is the 2-hour `kernelBuildTimeout` ("Round 8 fixes") |
| NOTE 1 (`ze appliance init <name> --config <file>` did not name the file on a `Validate` refusal) | f1b437f7c8 | `runInit` wraps the refusal with the `--config` path; the same test drives `runInit` with a riscv64 config file and asserts field, file and last line; red against HEAD's `cmd_init.go` under `-overlay` |
| NOTE 2 (Review Gate Rounds count and lenses stale) | this spec commit | the Rounds row leads with 7 completed and round 8 owed; the lenses row covers rounds 1 to 7 |
| NOTE 3 (`cmd_kernel.go` comment spoke only of the arch) | f1b437f7c8 | the comment says `Validate` checks every field, so any invalid field refuses the kernel build |

`go test -race` with the gate tags over `./internal/appliance/... ./internal/le/build/gokrazy/...`: all four packages ok (appliance, instance, kernelbuilder, gokrazy), 2026-10-11. Scoped lint (`./le go lint run scope ./internal/appliance/...`): 0 issues on darwin and linux-integration, 2026-10-11.

Round 8 is owed over f1b437f7c8; owner authorised on 2026-10-11 (Review Gate row).

### Round 8 fixes
Review over f1b437f7c8, 7629b7c783: 0 BLOCKER, 3 ISSUE, 3 NOTE. Artifact `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-r8-appliance-ships-ze-kernel.md`. Red proofs, overlays and gate logs are in `scratch/r8fix/`.

Owner decisions, 2026-10-11:
- Build limit: "2 hours". One limit is kept, the CLI's 120-minute `kernelBuildTimeout`. The 4-hour `buildTimeout` and `errBuildTimedOut` are deleted.
- Round 9: "Authorise round 9" (Review Gate row).

No fix touches `recipe.go`, `worker.go`, `tools/kernel-builder/`, `internal/core/textbuf` or `internal/core/tmplink` (`git diff --stat` over them is empty). `stopped.go`, `driver.go` and `qemu.go` are not in `kernelbuilder.BuildSources`, so the cache key does not move.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| ISSUE 1 (a stop on the Docker backend, and in the QEMU worker compile, read `... failed: signal: killed`) | 42eb15b049 | `runDocker` checks `ctx.Err()` after the image build fails and, through `containerFailure`, after `docker run` fails; `buildGuestWorker` does the same. Each returns `errBuildStopped` (moved to `stopped.go`, now taking the context), which names the step and says canceled, Ze's limit (named), or the caller's deadline. `TestDockerStopNamesTheStep` (image and container, times canceled, caller deadline, build limit) and `TestWorkerBuildStopNamesTheStep` (the three) use the probe's `runCommand` stub. All six Docker subtests and `TestRunDockerRepairsOwnershipAfterFailure` are red with HEAD's `driver.go` under `-overlay` (`red_docker.log`), and all three worker subtests are red with the worker check removed (`red_worker.log`). `TestRunDockerRepairsOwnershipAfterFailure` now asserts the cancel rather than the step's error, and still asserts the repair ran |
| ISSUE 2 (two build limits, the 4-hour one unreachable from the CLI, the 2-hour one never named) | 42eb15b049 | `kernelBuildTimeout` (`cmd_kernel.go`) is the only declaration. `defaultKernelBuild` hands it over as `kernelbuilder.WithBuildLimit`, a deadline whose cause is Ze's limit. A build that runs out of it says "did not finish within the 2-hour limit Ze gives one kernel build". It no longer guesses "stalled" or "too slow". A deadline that another caller sets still reads "ran out of the time its caller gave it". `TestDefaultKernelBuildNamesZesLimit` is red with `context.WithTimeout` restored in `cmd_kernel.go` (`red_cli.log`). The `build limit` subtests are red with the cause check removed (`red_limit.log`, `red2_limit.log`). `TestBuildLimitNamesHours` pins "2-hour". `kernel-profiles.md` and the Round 7 ISSUE 2 row are corrected |
| ISSUE 3 (the guest-build deadline sent the operator to a QEMU command that boots no build) | 42eb15b049 | the stage "while it built the kernel" ends with the build output above and `ssh -t -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p <port> root@localhost top`. The port is the one the next run prints on its `>>> booting Alpine VM` line. Correction (round-9 NOTE 1): the options were only the host-key options of `sshOptions`, not all of them; since round 9 the advice is built from `sshClientOptions`, the options `sshOptions` itself uses, so it carries all four. The QEMU command stays for the boot, login and SSH stages (`qemuNextStep`). `TestStoppedDuringGuestBuildNamesTheStage` asserts the ssh last line, no QEMU command, and no "stall" or "too slow". The caller-deadline and build-limit subtests are red with the old QEMU advice under `-overlay` (`red2_next.log`) |
| NOTE 1 (download and console-write waits) | none owed | the reviewer accepted these. Correction (round-9 ISSUE 1): this row said "Naming the limit is covered by ISSUE 2", and it was not. net/http answers `context.Cause(ctx)`, so a download stopped by Ze's limit read as the limit's own noun phrase, with no step and no next step. The downloads are fixed in "Round 9 fixes" |
| NOTE 2 (a stopped `docker run --rm` probably leaves the container compiling) | 42eb15b049 | one row in `plan/journal/stop-leaves-the-work-running.md`. The container-step error now tells the operator to look with `docker ps --filter ancestor=<image>` |
| NOTE 3 (Round 7 ISSUE 2 row named the unreachable 4-hour limit) | this spec commit | the row carries the correction |

`go test -race` with the gate tags over `./internal/appliance/... ./internal/le/build/gokrazy/...`: all four packages ok (appliance, instance, kernelbuilder, gokrazy), 2026-10-11 (`gate-test.log`). Scoped lint (`./le go lint run scope ./internal/appliance/...`): 0 issues on darwin and linux-integration, 2026-10-11 (`gate-lint.log`).

Round 9 is owed over 42eb15b049; owner authorised on 2026-10-11, "Authorise round 9" (Review Gate row).

### Round 9 fixes
Review over 42eb15b049, 1a0294116d: 0 BLOCKER, 2 ISSUE, 1 NOTE. Artifact `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-r9-appliance-ships-ze-kernel.md`. Red proofs, overlays and gate logs are in `scratch/r9fix/`.

No fix touches `recipe.go`, `worker.go`, `tools/kernel-builder/`, `internal/core/textbuf` or `internal/core/tmplink` (`git diff --stat` over them is empty). `stopped.go`, `driver.go`, `qemu.go` and `space.go` are not in `kernelbuilder.BuildSources`, so the cache key does not move.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| ISSUE 1 (a stopped download read as a bare `context canceled`, or as the limit's own noun phrase) | 885e6979dd | `ensureAlpineISO` checks `ctx.Err()` after the checksum fetch and after the ISO download fail, and `fetchKernelTarball` (the tarball download, moved out of `runQEMUBuild` so a test reaches it) does the same. Each returns `errBuildStopped` with its step ("the Alpine ISO checksum download was stopped", "the Alpine ISO download was stopped", "the kernel tarball download was stopped"). The next step is `curl -fL -o /dev/null <url>` for the Alpine mirror, and for the tarball the `curl` that fetches it through a `.part` file to the path the next build reads. `TestDownloadStopNamesTheStep` runs the three steps over `stopCases()` against the probe's slow httptest server; all nine subtests are red with the three checks disabled under `-overlay` (`red_nodl.log`). The sweep: `Build` now passes every backend error through `stopReport`, which names any stop a step returned without naming it (QEMU's `Start`, `freeTCPPort`'s `Listen`, a console write to a killed QEMU), with the step's error as evidence, so no stop is reported by its cause marker alone. Corrected by round 10 (ISSUE 1, NOTE 1): that call had no test of its own, since `TestStopReportNamesAnUnnamedStop` calls `stopReport` directly and every gate package stayed green with Build's call removed; `SelectBuilder`'s error did not pass through it, so a Ctrl-C during the `sysctl` hvf probe read as a refusal of the host; and `freeTCPPort`'s `Listen` of a literal address does not read a done context, so the raw step Build meets is QEMU's `Start`. Build now passes `SelectBuilder`'s error through `stopReport` as well, and both calls carry a test that drives Build ("Round 10 fixes"). `errBuildStopped` returns a `*buildStopped`, which `stopReport` reads to leave a named stop alone; the two probe reports that named a cancel by hand (`-accel help`, `docker info`) now use `errBuildStopped`. `TestStopReportNamesAnUnnamedStop` is red over all three stops with the backstop disabled (`red_nobackstop.log`). Outside `kernelbuilder`: `downloadFirmwareBlob` (`cmd_kernel.go`) returned net/http's error bare, and `runISOBuilder` (`cmd_iso.go`) said "GRUB EFI builder timed out" or exec's "exit status N". `firmwareDownloadFailed` names the URL, the 10-minute `downloadTimeout` when that ran out, and the `curl` that fetches the blob to where `ensureFirmware` looks; `isoToolFailed` names the tool, the limit, and its command line, after `--keep-staging`. `TestFirmwareDownloadNamesTheFetch` and both cases of `TestISOToolFailureNamesTheTool` are red with the raw errors restored (`red_raw.log`). `downloadFile` now names its URL on a failed body read. `http.DefaultClient` stays: a client timeout would be a second limit on the build, and the one limit is `kernelBuildTimeout`; a stalled mirror is now reported at its download step, with its URL, when the build runs out of it. `kernel-profiles.md` gains the download rows and the backstop row; `iso-installer.md` gains the ISO tool limit |
| ISSUE 2 (a Ctrl-C on the kernel container step dropped the `docker ps` line) | 885e6979dd | `errBuildStoppedLeaving` takes a fourth argument, what the stop leaves running and how to clear it, printed on every branch after the next step, a canceled build included, because it is cleanup and not a retry. `containerFailure` passes the `docker ps --filter ancestor=<image>` line and the volumes it holds. `TestDockerStopNamesTheStep` asserts that line ends every kernel-container case; `kernel_container/canceled` is red with the line dropped on cancel (`red_nocleanup.log`). The `kernel-profiles.md` table says the line prints on every stop |
| NOTE 1 (the ssh advice claimed to match `sshOptions` and carried two of its four options) | 885e6979dd | `sshClientOptions` declares the four options once; `sshOptions` appends the port to them, and `guestBuildNextStep` builds its `ssh -t` line from them, so the advice adds `PreferredAuthentications=none` and `LogLevel=ERROR`. `TestGuestBuildAdviceLogsInAsTheBuildDoes` is red with the old two-option literal (`red_oldssh.log`). The comment, the doc row and the Round 8 ISSUE 3 row are corrected |
| Review Gate claims (the Round 8 NOTE 1 row, "Naming the limit is covered by ISSUE 2"; the Round 8 ISSUE 3 row, "the options match `sshOptions`") | this spec commit | both rows carry a correction |

Found on the way, not fixed: `defaultHTTPGet` (`cache.go`) cancels its request's context before the caller reads the body. One row in `plan/journal/lifetime-ends-before-its-last-reader.md`.

`go test -race` with the gate tags over `./internal/appliance/... ./internal/le/build/gokrazy/... ./internal/test/fixture/...`: all five packages ok (appliance, instance, kernelbuilder, gokrazy, fixture), 2026-10-11 (`gate-test.log`). Scoped lint (`./le go lint run scope ./internal/appliance/...`): 0 issues on darwin and linux-integration, 2026-10-11 (`gate-lint.log`).

Round 10 is owed over 885e6979dd. It is past five rounds, so it needs the owner's authorisation, recorded in the Review Gate's Owner authorised row.

### Round 10 fixes
Review over 885e6979dd, 8cb1f9aa7c: 0 BLOCKER, 1 ISSUE, 3 NOTE. Artifact `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-r10-appliance-ships-ze-kernel.md`, probes in `scratch/r10/`. Red proofs, overlays and gate logs are in `scratch/r10fix/`.

No fix touches `recipe.go`, `worker.go`, `tools/kernel-builder/`, `internal/core/textbuf` or `internal/core/tmplink` (`git diff --stat` over them is empty). `driver.go`, `stopped.go`, `cmd_iso.go` and `cmd_kernel.go` are not in `kernelbuilder.BuildSources`, so the cache key does not move.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| ISSUE 1 (no test drove Build's backstop call) | e440a9fe58 | `TestBuildNamesAStopAStepReturnedRaw` drives `Build` itself on the QEMU path: a cached Alpine ISO under `XDG_CACHE_HOME`, a stubbed `runWorkerBuild` that returns once the stop has landed, and a PATH QEMU that lists hvf and kvm. The step it reaches is QEMU's `Start` ("start QEMU: context canceled"); `freeTCPPort`'s `Listen` of a literal address does not read a done context. Over canceled, caller deadline and build limit it asserts "the kernel build was stopped part way (its last step said:", the stop's own words, `errors.Is`, and, on a deadline, the backstop's next step as the last line. All three subtests are red with Build's `return stopReport(ctx, err)` changed to `return err` under `-overlay` (`red_nobackstopcall.log`) |
| NOTE 1 (SelectBuilder's error bypassed stopReport) | e440a9fe58 | `Build` passes `SelectBuilder`'s error through `stopReport` too. `TestBuildNamesAStopDuringBuilderSelection` plays a Mac whose QEMU lists hvf only and stops the build while the real `probeHVF` runs `sysctl`; all three stops read "stopped part way (its last step said: no builder available: ...". All three are red with that call removed (`red_noselectstop.log`) |
| NOTE 2 (the `defaultHTTPGet` journal row said unprobed) | e440a9fe58 | the row in `plan/journal/lifetime-ends-before-its-last-reader.md` names the reviewer's probe and its result: every multi-buffer body fails, 3 of 3 on an 8 MiB TLS body |
| NOTE 3 (limit wording differed; the ISO tool path was unquoted) | e440a9fe58 | `kernelbuilder.LimitWords`, exported and declared once, writes whole hours as "2-hour" and whole minutes as "10-minute"; `buildLimit`, `firmwareDownloadFailed` and `isoToolFailed` all use it. `isoToolFailed` quotes the tool path with `shellArg`. `TestLimitWordsIsOneWording` pins the forms; `TestFirmwareDownloadNamesTheFetch` expects "the 10-minute limit", and `TestISOToolFailureNamesTheTool` puts the tool under a directory with a space and expects the quoted path. Both appliance tests are red with the old `%s` limit and the unquoted path restored (`red_oldwording.log`). Corrected by round 11 (NOTE 2): `TestISOToolFailureNamesTheTool` runs with `isoBuildTimeout` at 300ms, where `LimitWords` and `%s` both print "300ms", so it is red from the unquoted path only (`scratch/r11/red_noquote.log`); the ISO tools' "10-minute" wording is pinned by the 10m case of `TestLimitWordsIsOneWording`, not by an ISO test. `kernel-profiles.md` and `iso-installer.md` say so |
| Review Gate claims (the Round 9 ISSUE 1 sweep sentence; `kernel-profiles.md`'s "no stop is reported as a library's error alone") | this spec commit, e440a9fe58 | the Round 9 row carries a correction; the doc row now covers `SelectBuilder`'s error as well as every backend error |

`go test -race` with the gate tags over `./internal/appliance/... ./internal/le/build/gokrazy/... ./internal/test/fixture/...`: all five packages ok (appliance, instance, kernelbuilder, gokrazy, fixture), 2026-10-11 (`gate-test.log`). Scoped lint (`./le go lint run scope ./internal/appliance/...`): 0 issues on darwin and linux-integration, 2026-10-11 (`gate-lint.log`).

Round 11 ran over e440a9fe58 with the owner's authorisation (Review Gate, Owner authorised row) and was CLEAN.

### Round 11 fixes
Review over e440a9fe58, da76ba1fe5, d2902fc19d: CLEAN, 0 BLOCKER, 0 ISSUE, 4 NOTE. Artifact `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-r11-appliance-ships-ze-kernel.md`, probes in `scratch/r11/`. No fix touches `recipe.go`, `worker.go`, `tools/kernel-builder/`, `internal/core/textbuf` or `internal/core/tmplink`; the reviewer's key probe, re-run over the edited tree, still keys arm64 to `...-docker-7c0d8f79-34cba457`.

| Finding | Fixed by | What changed |
|---------|----------|--------------|
| NOTE 1 (the raw-step test pinned no step; its PREVENTS named `freeTCPPort`'s `Listen`) | the round-11 code commit | `TestBuildNamesAStopAStepReturnedRaw` passes `stoppedAtStep(req)` to `checkStoppedPartWay`: "start QEMU: " on any host except an arm64 host with no aarch64 firmware, where it is `findAArch64Firmware`'s refusal, which `qemuArgs` returns raw. PREVENTS names QEMU's `Start` only. Green on this Mac; still red on all three stops under `scratch/r11/ov_nobackstop.json`, with the selection test green |
| NOTE 2 (the Round 10 NOTE 3 row overstated the ISO test) | this spec commit | the row carries a correction: `TestISOToolFailureNamesTheTool` is red from the unquoted path only; the 10-minute wording is pinned by `TestLimitWordsIsOneWording` |
| NOTE 3 (`kernel-profiles.md` "every limit a build names" wider than the code) | the round-11 code commit | `availableAccelerators` (`qemu.go`) and `dockerRootDir` (`space.go`) word `hostProbeTimeout` through `LimitWords` ("15s", unchanged output), and `kernel-profiles.md` names that limit beside the other three, with a `space.go` source anchor |
| NOTE 4 (128-symbol check owed; d826e25e citations; Audit row) | this spec commit | the round-11 evidence line in the Task section records 128/128 `=y` plus the four lines on 34cba457, marks d826e25e as evicted, and records the arm64 `mpls-boot-test` PASS on 34cba457 with no kernel build; Goal Validation rows 1, 2 and 5 cite 34cba457; the Audit requirement row is Partial |

Gates: `go test -race` with the gate tags over `./internal/appliance/... ./internal/le/build/gokrazy/...` (`scratch/r11close-gate-test.log`); `./le doc check all` as recorded in Documentation Updates.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/appliance/instance/kernelpkg.go` | yes | `addLinuxNotice` found by `git grep` 2026-10-10 |
| `internal/appliance/runtimekernel.go` | yes | `RuntimeKernelTree` found by `git grep` 2026-10-10 |
| `test/ui/doctor-appliance-runtime-kernel.ci` | yes | committed in 1927823c9f |
| `internal/le/scratch/cacheclean_kernel_test.go` | yes | `TestCleanTargetsNeverReachTheKernelCache`; historical test evidence above, current cleanup producer `cleanTargets` in `internal/le/scratch/cacheclean.go` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-4..AC-17, AC-19 | unit tests named in the audit pass | `./le job run label unit-pkg command go test -v -tags "<gate tags>" -run '<30 tests>'` over `internal/appliance`, `.../instance`, `.../kernelbuilder`, `internal/le/build/gokrazy`, `internal/le/scratch`, `internal/le/test/qemu`: 30 `--- PASS`, 0 FAIL, 2026-10-10 |
| AC-17 | notice in a built image | read back from `ze-20261010-140153.img`, see Goal Validation |
| AC-1..AC-3 | default-image readback and native amd64 KVM boot | `police-default-readback.json`, `police-mpls-boot.log`; historical arm64 evidence retained separately |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze appliance build` | `TestResolveBuildParentDirUsesRuntimeKernel`, historical arm64 build and fresh amd64 default `t2` image/readback | yes; amd64 image assembly reused the newly compiled kernel cache |
| `./le build gokrazy` | `TestRunResolvesRuntimeKernelByDefault` | unit only |
| image boot | `./le test qemu mpls-boot` | historical arm64 PASS; fresh amd64 KVM PASS on the POLICE kernel |
| `ze doctor` | `test/ui/doctor-appliance-runtime-kernel.ci` | red-proven by mutation in 1927823c9f |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | validated | owner decisions in Task (route b, cached cold build accepted) |
| A-2 | validated on native arm64 and amd64 | Historical arm64 boot; fresh amd64 MPLS and hugepage boot logs below |
| A-3 | QEMU half validated; hardware owner-deferred | Native amd64 KVM and historical arm64 HVF evidence; Task decision "ignore n100 atm" |
| A-4 | validated | arm64 build exit 0 and boot |
| A-5 | validated | c9e17d79ea, digest from `sha256sums.asc` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| feature entry: default image boots ze's kernel and carries the notice | `resolveBuildParentDir`, `RuntimeKernelTree`, `addLinuxNotice` exist (`git grep`) | yes |
| disk-full page names the kernel cache | `evictKeepN`, `TestCleanTargetsNeverReachTheKernelCache` | yes |
| doctor check page | `checkRuntimeKernel` in `internal/appliance/doctor_checks.go` | yes (1927823c9f) |

## Native amd64 evidence, 2026-10-27

The default image build passed in 215.04 seconds. Readback from its actual boot
partition found all 149 then-declared built-ins, MPLS routing and encapsulation,
and kernel SHA-256 `b0b7d1156d9dc2240fb760f3d2c12e36ed7912626360df02a9927223167731d4`.
The root partition's GPL notice matched Linux 7.2.9 and the source digest.
Native KVM MPLS boot passed in 356.17 seconds; hugepage boot passed in
274.40 seconds with 64 two-MiB pages. These prove boot/CLI and reservation,
not LDP or VPP traffic. The existing warm kernel cache was reused, not rebuilt.
Evidence: session `2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641`,
`scratch/appliance-amd64-proof/{report.txt,readback-result.json,mpls-boot.log,vpp-hugepages.log}`.

The native default-appliance L2TP proof refused before boot because this
session has neither root nor host `CAP_NET_ADMIN`; its host bridge, TAP and
LAC setup genuinely need that privilege. No L2TP pass is claimed.

The subsequent access investigation found a missing product requirement:
subscriber upload policing reaches `ingressPolicerFilter` in
`internal/plugins/traffic/netlink/policer_linux.go`, but `NET_ACT_POLICE`
was absent from the runtime requirements. The 149-symbol result above remains
historical evidence for that image, not proof of complete shaping support.
The fragment, manifest and compiled-floor correction landed in `80175d768d`;
the public bounded job-count plumbing landed in `b7e2ace8ed`. The access spec
owns the native CoA scenario and the userspace shaping defect it exposed.

### Corrected POLICE kernel and fresh native proofs

The native amd64 kernel build used `--jobs 4` and passed in 1678.19 seconds,
within the existing two-hour limit. Its cache is
`7.2.9-runtime-amd64-runtime-docker-5db21ebf-34cba457`.
Source and build caches were retained. The subsequent default `t2` image build
passed in 239.15 seconds using that warm kernel cache; this is not a claim
that a cold kernel build was triggered through the appliance-build entry point.

Actual default-image readback found 150/150 required built-ins, including
POLICE and both MPLS symbols, with kernel SHA-256
`4910bc4600ba8d76d6250f09aca22c8eaf2c4b4459baadf9f9e52aec27c429b4`.
The image's kernel bytes equal the cache bytes, and its GPL notice matches the
pinned Linux 7.2.9 source URL and digest. Image SHA-256:
`67905c2940931ac5bf41cc9c64eadb6c17e094256cab415dca81deda4c1443b2`.

Fresh native KVM MPLS boot passed in 365.90 seconds, with CLI answer `[]`.
Hugepage boot passed in 332.02 seconds, reserving 64 two-MiB pages.
These prove startup/CLI and reservation, not LDP or VPP traffic.
The default-appliance L2TP privilege blocker above remains unchanged.

Evidence under the session's `scratch/appliance-amd64-proof/`:
`police-native-commands.json`, `police-kernel-build.log`,
`police-default-readback.json`, `police-mpls-boot.log`,
`police-vpp-hugepages.log` and `police-final-report.json`.
Original images, kernel/config/notice and logs are retained for review and
reproduction; extracted partition intermediates were removed by their owner.

The runtime POLICE journal occurrence is now resolved: the real
`radius-coa-disconnect-freeradius` scenario passed on the corrected native Ze
guest after `0d9da69ff3` repaired userspace TBF units and classless handling.
The unchanged checker read initial kernel rates of 10M/2M, then 20M/5M after
CoA, and retained both rates after invalid CoA. A separate HTB kernel readback
passed. This is shaping-state and connectivity proof, not throughput evidence;
it does not clear the default-appliance L2TP host-privilege blocker.

### Privileged L2TP failure and bridge repair, 2026-10-28

Thomas ran `./le --name orchestration test deployment gokrazy-l2tp-ppp`
as root, with the user cache and `/usr/local/go/bin` on PATH. The native
amd64 image booted Linux 7.2.9-ze under KVM, bound UDP 1701, and answered
the LAC namespace's readiness ping at 172.31.0.10. The result was
`proven: false`: `xl2tpd did not establish an incoming L2TP session with
the appliance`. Neither PPP nor dataplane completion is proved.

| Diagnosis | Evidence or decision |
|-----------|----------------------|
| Symptom | xl2tpd sent SCCRQ from 172.31.0.2:1702; the session marker was not observed. |
| Producer | `gokrazyLab.allowBridgeThroughFirewall` in `internal/le/test/deployment/gokrazylab.go` installed only an INPUT rule and assumed bridged traffic bypassed filtering. |
| Observed cause | Host kernel journal at 14:29:21, :22, :24, :28 and :36 records UFW BLOCK from 172.31.0.2:1702 to 172.31.0.10:1701, IN and OUT zebr547760, PHYSIN zgokh547760, PHYSOUT zetap547760. `net.bridge.bridge-nf-call-iptables` reads 1. |
| Owning layer | Host network setup for the appliance proof; no product protocol defect is established by this failure. |
| Rejected workaround | Disabling UFW or global bridge netfilter would change protection for other host workloads. |
| Source repair | Permit UFW forwarding only when both interfaces are this run's bridge; remove that exact route rule with the INPUT rule during cleanup. Refuse setup if UFW status or either installation fails. |
| Verification boundary | Superseded by the owner-run privileged PASS at 15:04 UTC below. The unchanged session, PPP, traffic and teardown assertions passed, and UFW readback showed no lab rules. |

The journal was read with `journalctl -k --since '2026-10-28 14:29:00'
--until '2026-10-28 14:30:30' --grep '172\.31\.0\.|ze-gok|UFW'
--no-pager`. The failed image remains at
`tmp/evidence/gokrazy-l2tp-ppp-3417732332/ze.img`; preserve it for the
existing-image replay. The harness's later SIGTERM messages are cleanup,
not the cause of the failed exchange. The earlier privilege-blocked status
is superseded by this executed failure, not by a passing result.

The scoped deployment package passed with the feature-gate tags and `-race`
through the native job runner (exit 0, package time 2.201s; receipt
`tmp/session/2026-10-28-30667194-e251-46e5-93e8-56ed151de886/scratch/job-l2tp-deployment-unit-be847c44.log`).
Independent review of the bridge patch found no correctness or isolation
defect and confirmed the installed UFW parser accepts the matching deletion.
Neither result proves live forwarding or rule removal.

`./le doc check all` found no documentation drift but exited 1 for three
source anchors outside this change: `commandWordsMax` in
`docs/architecture/core-design.md`, `Available` in
`docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, and `workflowNestedLe` in
`docs/architecture/testing/ci-workflows.md`. No clean aggregate result is
claimed. The previously recorded unbounded `verify changed` path was not
repeated.

### Privileged L2TP passing receipt, 2026-10-28

Thomas replayed the repaired harness against the same image and returned the
following result. The first launcher retry did not execute the proof:
`internal/le/rfc/checkgroups.go:80` failed to compile because `Finding.Check`
was absent. The successful run invoked `bin/le-l2tp-30667194/le` directly
with `ZE_GOKRAZY_SKIP_BUILD=1`; its SHA-256, checked before the run, was
`1713790f1efd8b36f8b73dd466a5f6188ca043cd508bbdeed1e1d3a5b53f6b2f`.

| Receipt field | Owner-observed value |
|---------------|----------------------|
| Time | 2026-10-28 15:03–15:04 UTC |
| Verdict | `proven: true`, empty reason, `log-tail: null` |
| Kernel and accelerator | Linux 7.2.9-ze, amd64, KVM |
| Guest build | 2026-10-28T14:25:58Z; original `tmp/evidence/gokrazy-l2tp-ppp-3417732332/ze.img` |
| Peer and namespace | xl2tpd 1.3.18; `ze-gokrazy-lac-3723203` |
| PPP addresses | LNS 10.100.0.1, peer 10.100.0.2 |
| Reported interfaces | Appliance JSON value `ppp0\r`, peer `ppp0`; raw carriage return preserved in the receipt |
| Teardown console | Subscriber route withdrawn, pool address released and kernel tunnel 1702 deleted at 15:04:10 UTC |
| Host firewall afterward | UFW active; the same 16 baseline INPUT rules, no lab bridge INPUT or forwarding rule |

The producing path is `gokrazyL2TP.observe` in
`internal/le/test/deployment/gokrazyl2tp.go`. It sets `Proven` only after
`assertKernelState` verifies the peer interface and addresses and runs two
dataplane pings through PPP, then observes subscriber-route withdrawal and
calls `awaitTeardown`. `teardownSettled` in
`internal/le/test/deployment/pppstate.go` requires the peer's PPP links and
L2TP state to match their pre-session baseline.

The user supplied this evidence; the agent did not repeat the privileged
run. The machine-readable transcription is retained at
`tmp/session/2026-10-28-30667194-e251-46e5-93e8-56ed151de886/scratch/l2tp-owner-pass.json`.
The same guest failed before the bridge fix and passed after it, without
changing assertions. This clears the L2TP runtime blocker, not IPv6CP,
authentication, throughput, full RFC coverage or committed-tree verification.

## Closure resumption, 2026-10-28

The independent closure worker is available. The earlier service-capacity
blocker is cleared; no privileged proof is rerun. This resumed review covers
the bridge repair in `d036685b5e`, the receipt in `4e888514a6`, their callers
and teardown, current acceptance producers, and closure citations. It uses
logic/wiring, security/isolation, and evidence/completeness lenses. The eleven
historical review rounds and their actual owner authorisations remain history;
this is one resumed closure pass, not an invented authorisation for round 12.
No further owner-run privileged L2TP proof is owed.

### Current acceptance producers

| Criteria | Producer read during this closure | Evidence retained |
|----------|-----------------------------------|-------------------|
| AC-1, AC-2, AC-3, AC-7, AC-11 | `internal/appliance/kernelargs.go` `resolveBuildParentDir`; `internal/appliance/runtimekernel.go` `RuntimeKernelTree`; `internal/appliance/cmd_kernel.go` `resolveRuntimeKernel`; `internal/appliance/cache.go` `kernelCacheVariantFor`; `internal/appliance/instance/kernelpkg.go` `assembleKernelPackage` | The native amd64 150-symbol image readback, matching kernel digest, MPLS-seed KVM boot and historical arm64 image/HVF boot recorded above; no run repeated |
| AC-4, AC-5, AC-6 | `internal/appliance/instance/prepare.go` `Prepare`; `internal/le/build/gokrazy/gokrazy.go` `prepareArgs`; `internal/appliance/instance/prepare_repo_test.go` `TestNoRtr7KernelReference` | Mandatory cache tree, private package replacement, no override lookup; historical rejection and module-scan tests |
| AC-8, AC-9, AC-19 | `internal/appliance/kernelbuilder/worker.go` `downloadKernelSource`, `verifyKernelSource`, `SourceDigest`; `internal/appliance/cmd_kernel_test.go` `TestKernelVersionHasDigestPin` | Download and cached-tarball digest checks precede extraction; exact 7.2.9 pin and recorded clean patch application |
| AC-10 | `internal/appliance/cmd_kernel.go` `resolveRuntimeKernel` | Cold-build failure names the missing cache and the exact native build command; historical `TestColdOfflineCacheNamesRemedy` |
| AC-12 | `internal/appliance/kernelconfig_pairing_test.go` `TestEveryRuntimeConfigSymbolIsAssertedOrAcknowledged`, `TestUnverifiedRuntimeSymbolsHasNoStaleEntries` | Both assertions retain their config/manifest pairing checks |
| AC-13 | `internal/appliance/kernelbuilder/driver.go` `validateRequest`, `CheckArch`; `internal/appliance/kernelbuilder/qemu.go` `hardwareAccelerator` | Foreign architecture refused; only usable HVF/KVM returned, never TCG; historical negative tests |
| AC-14, AC-15, AC-16 | `internal/le/scratch/cacheclean.go` `cleanTargets`; `internal/le/scratch/storetrim.go` `trimStoresScanning`; `internal/appliance/cache.go` `evictKeepN` | Cleanup excludes kernel namespaces; eviction groups by architecture and protects the current entry; the disk-full guide retains this contract |
| AC-17 | `internal/appliance/instance/kernelpkg.go` `addLinuxNotice`; `internal/appliance/instance/provenance.go` `readProvenance`, `Provenance.LinuxNotice` | Notice derives from built provenance; native image notice readback and historical notice tests |
| L2TP runtime goal | `internal/le/test/deployment/gokrazyl2tp.go` `proveAppliance`, `gokrazyL2TP.observe`, `gokrazyL2TP.assertKernelState`; `internal/le/test/deployment/pppstate.go` `awaitTeardown`, `teardownSettled` | Owner-supplied same-image PASS after `d036685b5e`; peer addresses and two PPP pings, route withdrawal, peer baseline restored; actual UFW readback separately confirms no lab rules |

### Security, documentation and review verdict

Three lenses read the bridge diff and its caller/cleanup path: logic and
wiring; security and host isolation; evidence and completeness. Final
verdict: **CLEAN, 0 BLOCKER, 0 ISSUE** in this closure's scope.

`gokrazyLab.allowBridgeThroughFirewall` in
`internal/le/test/deployment/gokrazylab.go` fails on an unreadable or unknown
UFW status and on rule installation failure. Its forwarding rule names this
run's bridge for both interfaces; it neither disables UFW nor changes a
global sysctl. `proveAppliance` defers `removeLab` before setup, so partial
setup also reaches `clearBridgeFromFirewall`. Cleanup remains best-effort;
the owner-observed post-run rule listing, not `Proven` alone, proves removal
on this run. No shell interpolation, peer-controlled allocation, new panic,
loop, goroutine or protocol change was introduced. The Go style pass covered
names, bounds, paired MUST obligations, initialization and return shape.

The kernel supply-chain boundary remains `verifyKernelSource` before the
source is used; `Prepare` requires an architecture-checked kernel tree and
provenance before creating its private instance. `checkRuntimeKernel` in
`internal/appliance/doctor_checks.go` remains the registered build-readiness
producer. The repair adds no runtime dependency: UFW was already optional.

Documentation matched the producers: the bridge paragraph and source anchor
landed in `d036685b5e` in `docs/architecture/testing/qemu-integration.md`;
`docs/guide/appliance.md`, `docs/architecture/appliance/build-artifacts.md`
and `gokrazy-build-pins.md` retain the cache, native-build, assembly and notice
contracts. No config syntax, CLI grammar, RPC, plugin/SDK, wire format,
comparison level or RFC support claim changes in this closure. Citations in
the Docker-host spec, crash-capture spec, kernel-capability spec and Mac
handover are repointed or restated; sibling owner pauses and unproved
criteria are preserved. No new lesson or journal row is warranted.

The required administrative prechecks used the retained native binary:
`repo check` returned four findings outside this closure (RADIUS `EAPType`,
and numeric parsing in the subscriber policer checker, DNS resolver and DNS
mock); `test weakened check` returned fifteen findings in foreign RADIUS,
authradius and health-test edits, none carried here. `rfc check` returned
95 unrelated audit/discrimination violations; this tooling change adds or
changes no RFC-tagged unit. These are not aggregate green results and no
approval or reseal was manufactured. The prior `doc check all` result
(three unrelated anchors, detailed above) is reused without rerunning.
No test, build, formatter, lint, privileged proof or full verification ran
in this resumed closure. Existing targeted race and runtime receipts stand.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-10-27 | Fixed: runtime POLICE requirement and native subscriber CoA rate proof |

