# Build artifacts: kernel and initrd resolution

An ISO build needs an installer kernel and an installer initrd. Operators used
to run two `make -C tools/...` targets by hand and to know which system packages
those targets needed. `ze appliance kernel` and `ze appliance initrd` own that
pipeline now.

<!-- source: internal/appliance/cmd_kernel.go -- installer kernel resolution -->
<!-- source: internal/appliance/cmd_initrd.go -- installer initrd resolution and cpio packing -->
<!-- source: internal/appliance/cache.go -- resolveCacheDir, downloadAndVerify, kernelCacheVariant, initrdCacheVariant -->

## Three-tier resolution

Each command tries the tiers in order and stops at the first success:

| Tier | Source |
|------|--------|
| 1 | XDG cache hit under `~/.cache/ze/` |
| 2 | Download from the release server, SHA-256 verified before caching |
| 3 | Local build: Docker for the kernel, `make` for the initrd |

A downloaded artifact is also copied into the `tools/` build directory, so
`ze appliance iso` finds it with no extra flag. `defaultISOKernelPath` checks
the XDG cache before the `tools/` path, which makes a cached download visible to
the ISO command automatically.

Download URLs come from `ze.appliance.kernel.url` and `ze.appliance.initrd.url`,
with compiled-in defaults. Partial downloads are removed rather than cached.

## Decisions

- **Doctor checks test for the artifact, not for the build tool.** The artifact
  is the result; a build tool is one path to it. An operator who downloads
  pre-built artifacts gets clean doctor output with no Docker installed. Build
  tools are checked only when the build fallback is the tier in use.
- `ze appliance iso --check` calls the same resolution functions as the ISO
  build itself, so the readiness report cannot drift from the build.
- The whole surface lives under the `ze_setup` build tag, so `bin/ze-setup` is
  the complete build-and-deploy tool: kernel, initrd, appliance init, build and
  iso, plus PXE provisioning.
- Doctor checks register from the package `init()`, the same way the plugin
  doctor registers.
- Function variables (`httpGetFn`, `initrdMakeBuildFn`, and their siblings) make
  every tier testable with no Docker and no network.

<!-- source: internal/appliance/doctor_checks.go -- artifact presence checks and their hints -->

## Trap

**Cache invalidation for downloads is manual.** There is no staleness check and
no expiry. The kernel cache variant folds in registry-derived hashes so a
profile, config, manifest, or builder change invalidates a stale kernel; a
plain download has no such signal.

## Related

- `kernel-profiles.md` for what goes into a kernel build
- `installer-initrd.md` for the initrd cache-key trap around build tags
- `iso-installer.md` for the consumer of both artifacts
