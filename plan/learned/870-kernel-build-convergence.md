# 870 -- Kernel build convergence

## Context
The runtime gokrazy kernel and the installer kernel had diverged into two build systems. Runtime used `gokr-rebuild-kernel` plus a wrapper script on Linux 6.19.x, while the installer already had a better `merge_config.sh`-based flow on Linux 7.0.11. The goal was to converge both entry points on one Ze-owned builder so `make ze-kernel` and `ze appliance kernel` share version, build logic, and test coverage while still emitting different artifacts.

## Decisions
- Chose `tools/kernel-builder/` over keeping `gokr-rebuild-kernel` because the external binary was `package main` and the installer scripts already embodied the wanted build flow.
- Chose standalone Makefiles for runtime and installer over a Go wrapper because kernel builds must work without first building Ze binaries and already fit the installer `make -C` pattern.
- Chose one `build.sh` with `MODULES` and `PATCHES_DIR` over separate runtime and installer scripts so config fragments vary while build logic stays single-source.
- Chose parameterized `qemu-build.py` over duplicated QEMU scripts so runtime and installer pass repo-relative source/output/builder paths into the same VM backend.
- Chose tracked runtime fragments plus a patch series under `gokrazy/kernel/` over scratch `tmp/kernel/_build` state so runtime kernel inputs survive cleans and participate in normal review.

## Consequences
- `make ze-kernel` now overlays runtime `vmlinuz`, modules, DTBs, and overlays into the pinned gokrazy kernel module cache, and `make ze-kernel-clean` restores the backed-up pinned module cache.
- Runtime and installer now share Linux 7.0.11, builder package requirements, Docker/QEMU entrypoints, and cache invalidation logic, so version bumps or host-tool fixes must update one builder surface.
- Runtime config is now expressed as `gokrazy/kernel/kernel.config` plus `runtime.config`, making fragment review and functional checks possible without running a full kernel build.
- Builder package lists must include both compression and module-install host tools (`zstd`, `kmod`) because runtime uses `CONFIG_KERNEL_ZSTD=y` and `modules_install`.

## Gotchas
- The installer scripts were not placeholders; the useful move was to extract them, not rewrite them.
- `gokr-rebuild-kernel` was not importable Go code, so convergence had to happen around shared scripts, not a shared package.
- `ze appliance kernel` lives in the `ze-setup` binary, not the default `bin/ze`, so runtime needed its own Makefile entry point instead of trying to add a runtime profile there.
- Default builder behavior needed explicit functional tests: user paths depend on Docker-first selection for runtime and installer, with QEMU fallback only when Docker is unavailable.

## Files
- `mk/gokrazy.mk`
- `internal/appliance/cache.go`
- `internal/appliance/cmd_kernel.go`
- `internal/appliance/cmd_kernel_test.go`
- `internal/appliance/doctor_checks.go`
- `tools/kernel-builder/Dockerfile`
- `tools/kernel-builder/build.sh`
- `tools/kernel-builder/qemu-build.py`
- `gokrazy/kernel/Makefile`
- `gokrazy/kernel/kernel.config`
- `gokrazy/kernel/runtime.config`
- `gokrazy/kernel/patches/series`
- `gokrazy/kernel/patches/0001-nct6683.patch`
- `gokrazy/kernel/l2tp.config.addendum.txt`
- `gokrazy/kernel/qemu-evidence.config.addendum.txt`
- `tools/installer-kernel/Makefile`
- `tools/installer-kernel/README.md`
- `tools/installer-kernel/hardware.config`
- `tools/kernel-builder/Dockerfile`
- `tools/installer-kernel/build.sh`
- `tools/kernel-builder/qemu-build.py`
- `docs/guide/appliance.md`
- `docs/guide/ze-install.md`
- `test/install/appliance-kernel-auto-docker.ci`
- `test/install/appliance-kernel-auto-qemu.ci`
- `test/install/appliance-kernel-docker.ci`
- `test/install/appliance-kernel-qemu.ci`
- `test/install/kernel-builder-packages.ci`
- `test/install/kernel-compose.ci`
- `test/install/kernel-qemu-arch-alias.ci`
- `test/install/kernel-runtime-deps.ci`
- `test/install/kernel-wiring.ci`
- `test/install/ze-kernel-overlay.ci`
- `plan/learned/870-kernel-build-convergence.md`

## Correction (kernel-build-consolidation)
The `build.sh` referenced above (Decisions, and the Files entries
`tools/kernel-builder/build.sh` / `tools/installer-kernel/build.sh`) was
superseded by `tools/kernel-builder/build.py` in install-11; there is no longer
a `build.sh`. The consolidation that followed also added a single driver
`tools/kernel-builder/run.py` (docker/qemu selection + arch->platform map +
the single build-time `kernel.version` reader), a shared `# ze-include` config
fragment (`tools/kernel-builder/common/efi-console.config`), and removed the
in-place pinned-modcache backup/restore (`gokrazy/modcache/.ze-pinned-kernel`)
in favour of out-of-tree runtime-kernel consumption. The original decisions
above are preserved as the historical record.
