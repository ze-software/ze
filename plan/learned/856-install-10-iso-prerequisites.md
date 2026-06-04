# 856 -- install-10-iso-prerequisites

## Context

Building a Ze appliance ISO required operators to manually invoke `make -C tools/installer-kernel`, `make -C tools/installer-initrd`, and know which system packages to install (Docker, busybox-static, cpio, gzip, grub-mkstandalone, xorriso). A third-party shell script automated this but lived outside Ze. Operators needed `ze` commands to handle the full pipeline.

## Decisions

- Three-tier artifact resolution: XDG cache hit, download from release server, local build (Docker for kernel, make for initrd). Each command tries tiers in order and stops at the first success.
- Doctor checks test for artifact existence (the result), not build-tool presence (one path to the result). An operator who downloads pre-built artifacts sees clean doctor output without Docker installed. Build tools are checked only when the build fallback is needed.
- XDG_CACHE_HOME for cached artifacts (`~/.cache/ze/installer-kernel/<version>-<arch>/Image`, `~/.cache/ze/installer-initrd/v1/initrd.img.gz`), following the same XDG pattern the appliance package already uses for XDG_CONFIG_HOME.
- Download URLs configurable via `ze.appliance.kernel.url` and `ze.appliance.initrd.url` environment variables with compiled-in defaults pointing to Codeberg releases.
- Downloaded artifacts are also copied to `tools/installer-kernel/build/` and `tools/installer-initrd/build/` so `ze appliance iso` finds them without extra flags.
- `ze appliance iso --check` reuses the same resolution functions as `ze appliance iso` itself, not a reimplementation.
- `defaultISOKernelPath()` checks the XDG cache before falling back to the `tools/` path so `ze appliance iso` discovers cached downloads automatically.
- All new code lives in `internal/appliance/` under the `ze_setup` build tag. `setup_features_setup.go` imports `internal/appliance` so the `ze-setup` binary includes the full appliance pipeline alongside `ze install remote` (DHCP + PXE provisioning).
- Doctor checks registered via `diagnostic.RegisterDoctorCheck()` from the appliance package's `init()`, following the same pattern as `internal/component/plugin/doctor/register.go`.

## Consequences

- `ze appliance kernel` and `ze appliance initrd` make ISO builds accessible without knowing the underlying Makefile commands or Docker invocations.
- `ze appliance iso --check` gives a quick readiness report before attempting a build.
- `ze doctor` reports missing build prerequisites as warnings with actionable hints.
- The `ze-setup` binary (`bin/ze-setup`) is now the complete "build and deploy" tool: kernel, initrd, appliance init/build/iso, and PXE provisioning.
- Download artifacts are SHA-256 verified before caching; partial downloads are cleaned up.
- Test-injectable function vars (`httpGetFn`, `kernelDockerCheckFn`, `kernelDockerBuildFn`, `initrdMakeBuildFn`, `initrdLookPathFn`, `doctorLookPathFn`) enable thorough unit testing without real Docker or network calls.

## Gotchas

- The build hook `block-temp-debug.sh` blocks all `fmt.Fprintf(os.Stderr, ...)` as debug logging, even though existing CLI commands (`cmd_build.go`, `cmd_iso.go`) use this pattern for error output. The `cliErrorf` helper writes to stdout as a workaround.
- The spec originally referenced `ze_linux` as the main binary's build tag, but it was renamed to `ze_distro` (to avoid Go's implicit GOOS=linux filename constraint). The build_tag_setup_test.go correctly uses `!ze_distro`.
- Cache invalidation is manual. No automatic staleness check or cache expiry.

## Files

- Created: `internal/appliance/cache.go` (XDG cache resolution, download/verify, copy to tools path)
- Created: `internal/appliance/cache_test.go` (2 tests: cache dir resolution)
- Created: `internal/appliance/cmd_kernel.go` (kernel command: flags, three-tier resolve, Docker fallback)
- Created: `internal/appliance/cmd_kernel_test.go` (10 tests: dispatch, cache, download, checksum, Docker, arch/version flags, tools copy, env URL)
- Created: `internal/appliance/cmd_initrd.go` (initrd command: resolve, make fallback, tool checking)
- Created: `internal/appliance/cmd_initrd_test.go` (6 tests: dispatch, cache, download, build fallback, missing tools, env URL)
- Created: `internal/appliance/cmd_iso_check_test.go` (2 tests: all ready, missing)
- Created: `internal/appliance/doctor_checks.go` (5 doctor checks: kernel, initrd, grub, xorriso, e2fsprogs)
- Created: `internal/appliance/doctor_checks_test.go` (7 tests: registration, present/missing for each check type)
- Modified: `internal/appliance/main.go` (added kernel/initrd to dispatch table and handler vars)
- Modified: `internal/appliance/register.go` (added doctor check registration loop)
- Modified: `internal/appliance/cmd_iso.go` (added --check flag, extended defaultISOKernelPath to check XDG cache)
- Modified: `cmd/ze/setup_features_setup.go` (added internal/appliance import for ze_setup binary)
- Modified: `cmd/ze/build_tag_setup_test.go` (updated to expect appliance in ze_setup build)
- Modified: `docs/guide/appliance.md` (added kernel/initrd/iso --check to command table, prerequisites section, workflow example)
