# Build artifacts: kernel and initrd resolution

An ISO build needs an installer kernel and an installer initrd. Operators used
to build both artifacts by hand and track their system-package dependencies.
`ze appliance kernel` and `ze appliance initrd` own that pipeline now.

<!-- source: internal/appliance/cmd_kernel.go -- installer kernel resolution -->
<!-- source: internal/appliance/cmd_initrd.go -- installer initrd resolution and cpio packing -->
<!-- source: internal/appliance/cache.go -- resolveCacheDir, downloadAndVerify, kernelCacheVariant, initrdCacheVariant -->

## Three-tier resolution

Each command tries the tiers in order and stops at the first success:

| Tier | Source |
|------|--------|
| 1 | XDG cache hit under `~/.cache/ze/` |
| 2 | Download from the release server, SHA-256 verified before caching |
| 3 | Local native Go build for the kernel and initrd |

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
  tools are checked only when the build fallback is the tier in use, which for
  the runtime kernel means a cold cache.
- `ze appliance iso --check` calls the same resolution functions as the ISO
  build itself, so the readiness report cannot drift from the build.
- The build-host surface is registered by `internal/appliance`: kernel,
  initrd, appliance init/build/ISO, and PXE provisioning.
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

## The runtime kernel the appliance image boots

The runtime target has two tiers, not three: its cache entry, else a native
build on a host of the target's architecture. It has no download tier.
`ze appliance build` and `./le build gokrazy` resolve it for the image's
architecture through the same resolver as `ze appliance kernel --target
runtime`, so a warm cache starts no container or VM and a cold one builds and
caches the kernel first. A cold cache that cannot build, for example with no
network for the kernel.org tarball, fails with the cache path and the command
that fills it. The image's kernel package is assembled from the cache entry,
never from `tmp/kernel/build`, which every resolver call rewrites.

`ze doctor` reports a host that cannot produce this kernel, as the warning
`doctor-appliance-runtime-kernel`. The kernel builds only on a host of its own
architecture, so the check asks about this host's: it passes when the cache
entry the resolver would serve holds its `vmlinuz`, and otherwise when Docker,
or QEMU with Go, is there for the cold build. With neither, the warning names
the cache entry and the `ze appliance kernel --target runtime --arch <arch>`
command to run once Docker or QEMU is installed. Run outside the Ze source tree,
where the kernel config cannot be read, the check warns rather than pass.

The builder query the check calls, `kernelbuilder.UsableBuilder`, lives in its
own file, `kernelbuilder/doctor.go`, and that file is the one builder source the
kernel cache variant does not hash. Every other builder source is hashed, so an
edit to it makes each cached kernel stale and costs a cold rebuild per arch;
nothing in the doctor's file runs during a build, so editing it must not.

<!-- source: internal/appliance/runtimekernel.go -- RuntimeKernelTree -->
<!-- source: internal/appliance/cmd_kernel.go -- resolveRuntimeKernel -->
<!-- source: internal/appliance/doctor_checks.go -- checkRuntimeKernel -->
<!-- source: internal/appliance/kernelbuilder/doctor.go -- UsableBuilder, DoctorSourceName -->
<!-- source: internal/appliance/cache.go -- kernelBuilderSources -->

## Related

- `kernel-profiles.md` for what goes into a kernel build
- `installer-initrd.md` for the initrd cache-key trap around build tags
- `iso-installer.md` for the consumer of both artifacts
