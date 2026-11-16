# The container runtime

How Ze runs an OCI container itself: the upstream `runc` library
(`github.com/opencontainers/runc/libcontainer`) used whole as a vendored Go
module, behind a series of six patches, with the container's init role inside
the one `ze` binary.

**Status: wiring.** The patch series, its guard test and the call that enters
the library's init exist. The plugin that describes, starts, supervises and
removes a container is not built yet, so no `ze` build starts a container
today. This page states what exists and grows with the work.

## Why the library needs a diff

Every Ze build has cgo off. Upstream's runtime starts its own binary again as
the container's init, and the first code that binary runs is a C stage
(`libcontainer/nsenter`), linked through cgo. The C stage reads bootstrap data
from its parent, creates or joins the namespaces, forks twice and only then
lets the Go runtime start. A `ze` binary has no C stage. Without a change, the
parent writes bootstrap data nobody reads and waits for a child pid nobody
sends.

The change Ze needs is kept as a diff against the upstream release, applied to
the vendored tree, never as a fork (`ai/META-DECISIONS.md`, "Code and
dependencies"; `docs/contributing/patching-the-standard-library.md`, "Patching
a vendored module").

## The diff

One series, one module, every file stamped
`Ze-Reviewed-Against: github.com/opencontainers/runc v1.5.3`. Each hunk is a
patch file of its own under `internal/le/vendorpatch/patches/`, so a module
bump shows which concern broke.

| Patch | Upstream function or file | What it changes |
|-------|---------------------------|-----------------|
| `runc-01-direct-init.patch` | `container_linux.go` `newInitProcess` | Sets the clone flags of the init command from the configured namespaces, and a new session. Creates the container cgroup with no process, opens its directory and has the standard library clone init into it. Builds no bootstrap data |
| `runc-02-start-without-bootstrap.patch` | `process_linux.go` `initProcess.start` | Removes the write of the bootstrap data, the read of the child pid and the wait for the first child. The started process is init. Writes init's out-of-memory score, which the C stage wrote |
| `runc-03-refuse-setns.patch` | `container_linux.go` `newSetnsProcess` | Refuses, with an error that names the missing namespace join. The upstream body is kept under `newSetnsProcessWithJoin`, which nothing calls |
| `runc-04-cgroup-v2-manager.patch` | `factory_linux.go` `Create`, `Load` | Builds the cgroup v2 manager directly, so the package that links the systemd driver and the cgroup v1 manager is not imported |
| `runc-05-criu-constraint.patch` | `criu_linux.go`, `criu_disabled_linux.go` | The checkpoint client compiles only under the tag `runc_criu`, which no Ze build sets. Upstream builds it unless `runc_nocriu` is set |
| `runc-06-seccomp-seam.patch` | `seccomp/seccomp_unsupported.go` | `RegisterInit` stores the function that builds and loads a filter, and `InitSeccomp` calls it for a profile |

<!-- source: internal/le/vendorpatch/runc_direct_init_patch_test.go -- TestRuncDirectInitPatchApplied, runcDirectInitSeries -->

### What the patched library refuses

A request only the C stage could serve is refused with an error. It is never
ignored, because an ignored request would start a container with less than its
description asked for.

| Request | Refused by | Why |
|---------|------------|-----|
| A namespace with a path (join an existing namespace) | `runc-01` | The standard library creates namespaces at clone and cannot join one |
| Time namespace offsets | `runc-01` | Nothing writes them before init runs |
| User and group identity mappings | `runc-01` | Nothing writes them before init runs. Upstream's validation accepts a user namespace only with a path or mappings, so a user namespace is refused too |
| A parent death signal | `runc-01` | The standard library makes the child compare its parent pid with the one read before the clone. Inside a new pid namespace that pid reads 0, so init would kill itself at once |
| A process started inside a running container | `runc-03` | It needs the namespace join |
| A cgroup config that asks for the systemd driver | `runc-04` | Only the cgroup v2 filesystem manager is built |
| A host that is not in cgroup v2 unified mode | `runc-04` | The same |
| A seccomp profile when no filter is registered | `runc-06` | As upstream does with cgo off |

### Init is cloned into its cgroup

A cgroup namespace is rooted at the cgroup its creator is in at that moment.
Upstream starts init, attaches it to the container's cgroup from the parent,
and lets the C stage unshare the cgroup namespace afterwards. A clone that
carries the cgroup namespace flag has no "afterwards": init would be in the
daemon's cgroup when its namespace is made, and the container would read the
host's cgroup tree.

`runc-01` therefore creates the cgroup first, with no process in it, and
clones init straight into it through the standard library's cgroup descriptor
(`CLONE_INTO_CGROUP`). The descriptor of the cgroup directory is closed once
init is spawned, with the library's sealed copy of the binary.

`runc-02` leaves upstream's later attach of init in place. It writes a pid
that is already in the cgroup, which changes nothing.

### The filter seam

Upstream's filter needs libseccomp through cgo, so with cgo off the library
compiles a stub that refuses every profile. Ze's filter is Go code in Ze's own
tree, where it is linted and tested, never a file inside a patch. `runc-06`
adds the seam only: one registration function in the stub. A nil function or a
second registration panics, so nothing replaces the loader after start-up.
The stub's `Enabled`, `FlagSupported` and `Version` are unchanged and still
answer that libseccomp is absent.

## The init role

The library starts `/proc/self/exe`, sealed, with the argument `init` and its
own environment keys. In a `ze` binary that process is `ze`. The one call that
turns it into a container's init is `libcontainer.Init`, which reads the
init configuration from the inherited socket, prepares the root and the
confinement, and executes the entrypoint. It never returns.

<!-- source: internal/plugins/container/initrole_linux.go -- runLibraryInit -->

## What a module bump owes

The review stamp refuses a new `runc` version until each patched function is
read again at that version
(`docs/contributing/patching-the-standard-library.md`, "The review stamp").
Read these, then restamp or regenerate each patch:

| File of `libcontainer` | Functions |
|------------------------|-----------|
| `container_linux.go` | `newParentProcess`, `newInitProcess`, `newSetnsProcess`, `bootstrapData` |
| `process_linux.go` | `initProcess.start`, `getChildPid`, `waitForChildExit`, `setnsProcess.prepareCgroupFD` |
| `factory_linux.go` | `Create`, `Load` |
| `init_linux.go` | `Init`, `startInitialization` |
| `criu_linux.go`, `criu_disabled_linux.go` | The build constraints |
| `seccomp/seccomp_unsupported.go` | The whole file |

Also read `manager/new.go` (`NewWithPaths`) and `fs2/fs2.go` (`Manager.Apply`)
of `github.com/opencontainers/cgroups` at the version the bump brings:
`runc-04` copies the checks of the first, and `runc-01` relies on the second
creating a cgroup with no process when the pid is -1.
