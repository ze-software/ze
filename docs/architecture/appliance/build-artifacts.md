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
caches the kernel first. A cold cache for another architecture than the
host's is refused before any build starts, because a kernel built elsewhere
fills that host's cache, never this one. `ze appliance build` refuses it
before it writes anything (`runtimeKernelReachable`): the error names the cache
path, the two moves that work, building the image on a host of that
architecture or setting `image.arch` to the host's in the appliance's own
config file, and ends with the `ze appliance --dir <dir> build <name>` command
on its own line. A bare `ze appliance kernel --target runtime` for that
architecture ends with the kernel command to run on a host of it. The host's
architecture is declared once, `kernelbuilder.HostArch`, and the arch check
(`kernelbuilder.CheckArch`) runs before the installer firmware is fetched. A cold cache that cannot build on this
host, for example with no network for the kernel.org tarball, fails with the
cache path and the command that fills it here.
<!-- source: internal/appliance/cmd_kernel.go -- resolveRuntimeKernel, runtimeKernelReachable --> The image's kernel package is assembled from the cache entry,
never from `tmp/kernel/build`, which every resolver call rewrites.

`ze doctor` reports a host that cannot produce this kernel, as the warning
`doctor-appliance-runtime-kernel`. The kernel builds only on a host of its own
architecture, so the check asks about this host's: it passes when the cache
entry the resolver would serve holds its `vmlinuz`, and otherwise when the
cold build has a builder: Docker, or QEMU with Go and a hardware accelerator
this host can use (the same choice a build makes, `kernelbuilder.SelectBuilder`).
Otherwise the warning carries the builder's own reason, for example a QEMU that
lists `kvm` on a host whose `/dev/kvm` cannot be opened, then names the cache
entry and the `ze appliance kernel --target runtime --arch <arch>` command to
run once the builder can run. Run outside the Ze source tree,
where the kernel config cannot be read, the check warns rather than pass.

`ze doctor` also reports, as the warning `doctor-appliance-go-toolchain`, a
host whose `go` cannot compile the image: no `go` on `PATH`, or one older than
the `go` directive in `go.mod`. It asks `CheckGoToolchain`, the question
`ze appliance build` asks before it starts, so a host with Docker and an old
`go` is told before a build rather than after its kernel. Outside the Ze source
tree it cannot read `go.mod`, and says to run from the top of the tree.

<!-- source: internal/appliance/doctor_checks.go -- checkGoToolchain -->
<!-- source: internal/appliance/gokpreflight.go -- CheckGoToolchain -->

### Free space before a Docker kernel build

A Docker kernel build that runs out of disk does not fail cleanly: it fills the
store the daemon writes into, fails twenty minutes in, and names a content blob
rather than the disk. So `runDocker` measures free space before it builds the
builder image, and refuses below 40 GiB (`kernelBuildFreeBytesMin`). The figure
is one measurement, rounded: on 2026-09-02 an arm64 runtime build died part way
through `drivers/` with 13G in its build volume and 41G in use under the
daemon's root.

What it measures is the host filesystem that receives the build's writes:

| Host | Path measured | Why |
|------|---------------|-----|
| macOS | `~/.colima`, else `~/Library/Containers/com.docker.docker` | The daemon runs in a VM whose whole disk is one sparse file there; `/var/lib/docker` is a path inside the guest |
| Linux | The daemon's own root, from `docker info --format {{.DockerRootDir}}` | The data root is configurable, and a host that moved it keeps an empty `/var/lib/docker` on a small root disk |
| Linux, daemon not answering | `/var/lib/docker` | The default; the refusal says the daemon could not name its root, and why |

`docker info` is bounded at fifteen seconds. With `DOCKER_HOST` pointing at a
remote daemon, the path that daemon names is measured on this host, which is
the wrong machine; the guard does not detect that case. When no layout matches,
the build prints that free space was not checked and goes on, rather than read
as measured.

The refusal names both numbers, the path and why it was chosen, then two
commands on their own lines: `df -h <path>` to check the reading, and the same
kernel build with `--builder qemu`. The QEMU backend writes its build tree under
`tmp/qemu` in the checkout and has no free-space check of its own. On a VM-backed
daemon the space a build used is handed back to the host afterwards, pass or
fail (`reclaimBuildSpace`), because the VM's sparse disk never shrinks on its
own.

<!-- source: internal/appliance/kernelbuilder/space.go -- kernelBuildFreeBytesMin, dockerStoreHostPath, dockerRootDir, checkKernelBuildSpace, kernelBuildSpaceError, reclaimBuildSpace -->

The kernel cache variant hashes the inputs that decide a kernel's bytes, and
nothing else. A cold rebuild costs about thirty minutes per arch, so an edit
that changes no kernel must keep the cache. The key covers:

| Input | Why it decides the bytes |
|-------|--------------------------|
| The kernel version | It names the cache entry; the tarball's SHA-256 pin sits in `kernelbuilder/worker.go` |
| The profile's config fragments, `.require` manifests, and the target's patch series | They are the config and the source the kernel is built from |
| Every file under `tools/kernel-builder` | The container definition (toolchain and packages), the worker's `main`, and the shared fragments |
| The `kernelbuilder` files named in `kernelbuilder.BuildSources` | `worker.go` is the build. `recipe.go` holds what the host hands it: the `docker run` argv, the QEMU guest's Alpine release, packages and command line, the request defaults, and the provenance record |
| Every Ze package those Go files import, followed transitively | The worker and the recipe run that code |

The other `kernelbuilder` files are host-side: they select a backend, probe the
accelerator, measure free space, and word refusals. An edit there keeps every
cached kernel. The rule follows: code that changes what the build produces goes
in a `BuildSources` file, never in a host-side one.
`TestKernelCacheKeyCoversEveryBuildInput` refuses a `BuildSources` file that
reaches a function, variable or constant declared in a host-side file, an
`init` outside them, and a hashed file whose Ze import is not hashed.
`TestKernelCacheKeyIgnoresHostSideEdits` edits each host-side file and checks
the key holds, then edits each `BuildSources` file and checks it moves.

One input is not in the key: the `hardware-kms` installer profile's i915
firmware blobs, fetched once into the cache by `ensureFirmware` with no digest
pinned. A different blob under the same name keeps the cached installer kernel.

<!-- source: internal/appliance/runtimekernel.go -- RuntimeKernelTree -->
<!-- source: internal/appliance/cmd_kernel.go -- resolveRuntimeKernel -->
<!-- source: internal/appliance/doctor_checks.go -- checkRuntimeKernel -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- SelectBuilder -->
<!-- source: internal/appliance/cache.go -- kernelBuilderSources -->
<!-- source: internal/appliance/kernelbuilder/recipe.go -- BuildSources -->

## Related

- `kernel-profiles.md` for what goes into a kernel build
- `installer-initrd.md` for the initrd cache-key trap around build tags
- `iso-installer.md` for the consumer of both artifacts
