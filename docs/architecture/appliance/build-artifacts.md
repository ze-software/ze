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

The download deadline covers the response body, not only the headers.
`defaultHTTPGet` transfers cancellation to the response body: its caller
must close that body after reading, which releases the request context.

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
on its own line. `./le build gokrazy` asks the same question before it
resolves the kernel (`GokrazyRuntimeKernelReachable`), and its refusal ends
with `GOARCH=<host> ./le build gokrazy <args>`, with each argument quoted for
the shell. A bare `ze appliance kernel --target runtime` for that
architecture ends with the kernel command to run on a host of it. An
architecture Ze builds no kernel for at all (`kernelbuilder.SupportedArches`,
the arches the Docker backend names a platform for) never reaches a host's
cache or builder. `ze appliance kernel --arch` refuses it first, ending with
`ze appliance kernel --target <target> --arch <host>`; the appliance config's
`image.arch` is refused when the config is validated, which
`ze appliance build <name>`, `ze appliance kernel <name>`,
`ze appliance iso <name>` and `ze appliance init <name> --config <file>` do
before they read the arch, each naming the config file and ending with the
`"image": {"arch": "<host>"}` line to set; and `./le build gokrazy`, whose
`GOARCH` arrives unchecked, is refused by `runtimeKernelReachable` before the
cache is asked, ending with the `GOARCH=<host>` command. Each refusal names
the arches Ze builds and offers only the host's own, never a host of the
unsupported one. The host's
architecture is declared once, `kernelbuilder.HostArch`, and the arch check
(`kernelbuilder.CheckArch`) runs before the installer firmware is fetched. A cold cache that cannot build on this
host, for example with no network for the kernel.org tarball, fails with the
cache path and the command that fills it here.
<!-- source: internal/appliance/cmd_iso.go -- resolveISOInput -->
<!-- source: internal/appliance/cmd_init.go -- runInit -->
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
tree it cannot read `go.mod`, and says to run from the top of the tree. A
`go.mod` that belongs to another module is refused the same way: its `go`
directive says nothing about the `go` Ze needs. Ze's module path is read from
the appliance package's own import path, never from the file being judged.

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
| The target, arch and profile | They select the config, and the target decides whether the kernel carries modules (`targetModules`, `recipe.go`) |
| The backend, `docker` or `qemu` | The Docker image compiles with Debian bookworm's gcc, the QEMU guest with Alpine 3.21's, so the two build different bytes from one tree. The `--builder` value names it; with none, the backend a build here would choose (`kernelbuilder.PreferredBackend`: Docker when installed, else QEMU) |
| The profile's config fragments, `.require` manifests, and the target's patch series | They are the config and the source the kernel is built from |
| Every file under `tools/kernel-builder` | The container definition (toolchain and packages), the worker's `main`, and the shared fragments |
| The `kernelbuilder` files named in `kernelbuilder.BuildSources` | `worker.go` is the build. `recipe.go` holds what the host hands it: the `Request` type, the `docker build` and `docker run` argv, the worker's `go build` argv and environment, the QEMU guest's Alpine release, packages and command line, the request defaults and normalisation, and the provenance record |
| Every Ze package those Go files import, followed transitively | The worker and the recipe run that code |

The other `kernelbuilder` files are host-side: they select a backend, probe the
accelerator, measure free space, and word refusals. An edit there keeps every
cached kernel. The rule follows: code that changes what the build produces goes
in a `BuildSources` file, never in a host-side one.
The guard runs both ways. `TestKernelCacheKeyCoversEveryBuildInput` type-checks
the package and refuses a `BuildSources` file that uses anything a host-side
file declares (a function, variable, constant, type, method or field), an
`init` outside them, and a hashed file whose Ze import is not hashed.
`TestHostSideNeverWritesTheRequest` refuses a host-side file that assigns to
or increments a `Request` or a `Request` field (a pointer-receiver method's body
included), takes a pointer into one, builds a `Request` of its own, or passes
`sshRun` a guest command that is not `guestBuildCommand(...)` itself.
`TestRunDockerArgvAndOwnershipRepair` and `TestBuildGuestWorkerTakesTheRecipe`
check the Docker and worker builds' argv and environment as they reach the
`runCommand` and `runWorkerBuild` seams, and no further.
`TestKernelCacheKeyIgnoresHostSideEdits` edits each host-side file and checks
the key holds, then edits each `BuildSources` file and checks it moves.

The return direction is checked only for those syntactic writes and those
seams. These host-side channels reach the build with no guard, and a change
through any of them builds different bytes under an unchanged key (each shown
by a mutation that left every guard green, review rounds 4 and 5, 2026-10-10
and 2026-10-11). The table lists what those mutations found; it is not a bound
on what the guard misses:

| Unguarded channel | Where |
|-------------------|-------|
| Arguments the real `runCommand` appends to the `docker` argv | `kernelbuilder/driver.go` |
| Environment `runWorkerBuild` adds, or inherits from the host through `os.Environ` (`GOFLAGS`, `GOEXPERIMENT`, `GOAMD64`), when it compiles the worker | `kernelbuilder/qemu.go` |
| A `Request` slice changed through an alias or a call rather than an assignment to the field (`f := req.Fragments; f[0], f[1] = f[1], f[0]` in `runDocker`, `slices.Reverse` or `slices.Sort` on `req.Fragments`, `copy` into it) | any host-side file |
| The host-computed arguments passed to `guestBuildCommand` (`workerRel`, `shares`, or a `req` rewritten at the call): the `sshRun` check matches the callee's name only | `kernelbuilder/qemu.go` |
| The body of `sshRun`, which can rewrite or extend the command it is given before `ssh` runs it | `kernelbuilder/qemu.go` |
| The bootstrap commands sent over the QEMU serial console before `sshRun` (network setup, `resolv.conf`, `apk add openssh`) | `kernelbuilder/qemu.go` |

`plan/journal/stale-artifact-reused.md` records them. Until a guard covers
them, an edit to one of these channels owes a removal of the cache entry by
hand (`--print-cache-dir`, below).

A key that cannot be computed (a patch directory, a config fragment or a
builder source that cannot be read) is refused with its fix, never replaced by
a shorter key every config would share.

The key does not cover what each backend fetches from upstream, so the
toolchain floats under one key:

| Input not in the key | What drifts |
|----------------------|-------------|
| `FROM golang:1.27-bookworm` and `FROM debian:bookworm-slim` in `tools/kernel-builder/Dockerfile` | Tags, not digests: a rebuilt image brings a new Go or Debian point release |
| The packages `apt-get install` takes in the image, and `apk add` in the QEMU guest | No versions pinned: gcc, binutils and the rest move with the distribution |
| The Alpine ISO | Its SHA-256 is fetched beside it (`fetchPublishedChecksum`), not pinned in the tree |
| The `hardware-kms` installer profile's i915 firmware blobs | Fetched once by `ensureFirmware` with no digest pinned: a different blob under the same name keeps the cached installer kernel |

The backend in the key is the one the lookup names. A build takes
`--builder`, but the image build, `./le build gokrazy`, the ISO and the
`ze doctor` check look the runtime kernel up with no builder, which keys the
backend a build here would choose (`PreferredBackend`). A kernel built with the
other `--builder`, for example `--builder qemu` on a host with Docker
installed, sits under its own key and is not served to them: the image build
compiles the preferred backend's kernel instead, the ISO finds no installer
kernel in the cache, and the doctor check does not count it as cached.

`plan/journal/stale-artifact-reused.md` records each row. To rebuild against
the current upstream, remove the cache entry that
`ze appliance kernel --target <target> --print-cache-dir` prints.

<!-- source: internal/appliance/runtimekernel.go -- RuntimeKernelTree -->
<!-- source: internal/appliance/cmd_kernel.go -- resolveRuntimeKernel -->
<!-- source: internal/appliance/doctor_checks.go -- checkRuntimeKernel -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- SelectBuilder -->
<!-- source: internal/appliance/cache.go -- kernelBuilderSources -->
<!-- source: internal/appliance/kernelbuilder/recipe.go -- BuildSources -->

## The docker-host image cache

`./le setup docker host build` keeps its own cache under
`~/.cache/ze/docker-host/`: `downloads/` holds each pinned input checked
against its sha256, `<arch>/static-<key>/` the static OpenSSH, sudo and
iptables binaries the builder container made, and `<arch>/<key>/docker-host.img`
the image. `gokrazy/docker-host/binaries.json` is the pin file: the only home of
every third-party URL, version and sha256 the image carries, and of the builder
image digest. The key covers the pins, the runtime kernel key and the source
of every path that shapes the image, so a change to any of them builds a new
entry instead of reusing a stale one (`docker-host.md`, "Building the image").

<!-- source: internal/le/setup/docker/host/build.go -- digestRoots, sourceDigest -->
<!-- source: internal/le/setup/docker/host/pins.go -- LoadPins, ImageKey -->

## Related

- `kernel-profiles.md` for what goes into a kernel build
- `installer-initrd.md` for the initrd cache-key trap around build tags
- `iso-installer.md` for the consumer of both artifacts
