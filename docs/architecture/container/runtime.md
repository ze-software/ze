# The container runtime

How Ze runs an OCI container itself: the upstream `runc` library
(`github.com/opencontainers/runc/libcontainer`) used whole as a vendored Go
module, behind a series of six patches, with the container's init role inside
the one `ze` binary.

**Status: first start.** The patch series and its guard test, the init role,
the `container` plugin with its config root, its static verifier and its
restart command, the seams toward the image store, the participants and the
filter, its doctor codes, its runtime doctor checks, one kernel requirement and
the build gate exist, and so does the start sequence: a configured container is
described, its root mounted, created through the library, released and
watched; it is stopped with its stop signal, then killed after its stop
timeout; a daemon stop drains every container together; what a killed
daemon left is swept at the next start; and every container runs under the
default system-call filter. A container's standard output and standard
error are captured into a bounded buffer, and a container that ends on its
own with a failure is reported by doctor with its last lines ("The output",
below). What is not built yet: the device filter, the limit and grant nodes,
the restart policy, the `log` setting and the commands that read the output,
and the state view. No
shipped build holds an image store yet, so only the test build's store
(below) answers a start. This page states what exists and grows with the work.

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

### The system-call filter

The package `internal/plugins/container/seccomp` fills the seam. Its
`buildAndLoad` takes the profile of the container's description, builds a
classic BPF program for the native architecture and loads it, so a call the
profile refuses fails in every process of the container.

| Part | File | What it does |
|------|------|--------------|
| Default profile | `profile.go`: `DefaultProfile` | Moby's default profile (source, commit and digest in `third_party/moby-seccomp-profile/README.md`): EPERM for an unlisted call, the calls it allows, and the rules of a capability, selected by the container's bounding set as the source selects them. `describeContainer` gives it the container's own set, so without `CAP_SYS_ADMIN` `unshare`, `mount` and `setns` are refused, `clone` is refused a new namespace, and `clone3` answers ENOSYS so a C library falls back to `clone` |
| Tables | `table.go`, `table_x86_64.go`, `table_aarch64.go` | The call numbers of x86-64 and AArch64, generated from the vendored `golang.org/x/sys/unix` `zsysnum_linux_amd64.go` and `zsysnum_linux_arm64.go`. The files carry no `_amd64` suffix, which would be a build constraint: a host of either architecture compiles and tests both |
| Builder | `filter.go`: `assembleFilter` | A call under another audit architecture kills the process: a 32-bit program in an image is killed. A number above the largest the profile names answers ENOSYS, the x32 range included. Then one block per rule in profile order, the first match deciding, then the default. An argument is compared on both 32-bit halves of its 64-bit value. A name the table lacks, or a rule equal to the default, adds no instruction |
| Load | `load_linux.go`: `loadFilter` | The `seccomp` call with the flag that applies the filter to every thread of the init, after no-new-privileges is set |

The profile differs from its source in three ways. A name neither table holds
is left out (the 32-bit calls such as `chown32` and the `*_time64` calls,
`umount`, `stime`), and so are the rules for an architecture Ze ships no
table for. The rule the source applies from kernel 4.8 (`ptrace`,
`process_vm_readv`, `process_vm_writev`) is applied always, because a
container starts only through a clone into its control group, which needs
kernel 5.7.

The builder refuses what it cannot enforce exactly, and a refusal fails the
init, so a container never runs less confined than its description: a flag,
a notify listener or action, another architecture, an argument index above
5, an unknown operator or action, an error number above 16 bits, one call
with rules of two different actions, a jump above 255 and a program longer
than the kernel's 4096 instructions.

<!-- source: internal/plugins/container/seccomp/profile.go -- DefaultProfile, defaultProfileFor, buildSourceRules -->
<!-- source: internal/plugins/container/seccomp/table.go -- syscallTable, nativeTable -->
<!-- source: internal/plugins/container/seccomp/filter.go -- assembleFilter, ruleBlocks, argumentSteps, actionReturn -->
<!-- source: internal/plugins/container/seccomp/load_linux.go -- loadFilter -->
<!-- test: internal/plugins/container/seccomp/filter_test.go TestFilterComparesArguments -->
<!-- test: internal/plugins/container/seccomp/profile_test.go TestDefaultProfileFollowsTheSourceProfile -->
<!-- test: internal/plugins/container/seccomp/tables_test.go TestSyscallTablesMatchVendoredNumbers -->

## The init role

The library starts `/proc/self/exe`, sealed, with the argument `init` and its
own environment keys. In a `ze` binary that process is `ze`. The one call that
turns it into a container's init is `libcontainer.Init`, which reads the
init configuration from the inherited socket, prepares the root and the
confinement, and executes the entrypoint. It never returns.

<!-- source: internal/plugins/container/initrole_linux.go -- runLibraryInit -->

The library marks the process it starts with the environment key
`_LIBCONTAINER_INITPIPE`. The plugin registers that key with the always-on
leaf `internal/core/earlyrole`, and `dispatchMain` asks the leaf before
anything else of `ze` starts (`docs/architecture/system-architecture.md`, "A
role taken before command dispatch"). No always-on file names the plugin or
the library, so a build without `ze_container` holds no such role. The init
role links the filter package, because it is the process that loads a filter.

<!-- source: internal/plugins/container/register_initrole_linux.go -- libraryInitKey, runInitRole -->

## The plugin

The plugin is `container`, in `internal/plugins/container`, gated by
`ze_container` in `feature-gates.txt` with its filter package. It owns the
`container` config root and reaches the engine by registration only.

| Piece | What it does today |
|-------|--------------------|
| Config root | `container instance NAME` with `image` (mandatory, 1 to 255 characters: a short name, a whole reference or a reference by digest), `enabled`, `stop-timeout` (1 to 30 seconds, default 10) and `stop-signal` (`term`, `int`, `quit`, `hup`, `usr1`, `usr2`; absent, the image's signal, else `term`). Every other node of the container work lands with the package that realizes it |
| The image leaf | The schema refuses an instance without it. `parseImage` is the second check of the pair and also runs the store's reference grammar (`registry.ParseReference`), so a reference the store could not complete is refused at the commit, with `set container instance NAME image REFERENCE` as the fix. The runtime parses the leaf, not the image store, because the supervisor builds the resolve request: the value is its `Reference`, as written, and the store completes it again at each resolve. A changed image does not restart a running container: the worker starts a container only when it neither runs nor waits, so the new image is resolved at the container's next start |
| One rule set | `parseContainerSections` parses and judges a delivery: the runtime's own nodes, then the parse of each config sibling, which judges the sibling's nodes ("The seams inside the plugin", below). The registration's static verifier, the reload's verify and the boot configure all call it, so `ze config validate` accepts exactly what starts |
| A removed root | Arrives as an empty body and parses to a config with no container. Installing that config stops every container. Only a delivery with no container section means "unchanged" |
| Apply | Each config sibling applies its part first, then the supervisor records the desired set, wakes the worker of each container concerned and returns. It never waits for a start. The boot configure arrives before the startup handshake ends, when the engine refuses a worker's calls, so it only records: the plugin's `OnStarted` callback applies the siblings' part, then wakes the workers (`configInstaller.begin`, `supervisor.begin`) |
| Dependency | The `sysctl` plugin, which writes the kernel tunable a container's network namespace needs (below) |
| `request container restart name NAME` | A YANG command node bound to a wire method the plugin's package proxies to the plugin's process. A disabled container is refused with the statement that enables it |
| `show container list` | The command node `ze-container:show-list` of the same module, proxied the same way and answered from the supervisor's records (`list_cmd.go`): rows under `containers`, below |
| The image commands | Ten more command nodes of the same module, proxied the same way. `handleCommand` looks each up in the image store's table (`image_cmd.go`) before the restart ([image-store.md](image-store.md)), then a sibling's command in the siblings' table |
| Daemon stop | The plugin declares a drain budget of 37 seconds and stops every running container together in its drain handler, the one call of a daemon stop that waits for a plugin (below, "The stop") |
| Plugin start and exit | Before the engine is asked to start the plugin, the sweep removes what a previous run left. Once the engine connection closed, whatever still runs is killed at once |
| Failure policy | `restart` |

A disabled container is accepted and held in the `disabled` state; an enabled
one is started by its worker.

<!-- source: internal/plugins/container/register.go -- registerContainer, verifyContainerSections, forwardRestart -->
<!-- source: internal/plugins/container/config.go -- parseContainerSections, parseSectionsWith -->
<!-- source: internal/plugins/container/container.go -- runContainerEngine, handleCommand -->
<!-- source: internal/plugins/container/supervise.go -- supervisor.install, supervisor.apply, supervisor.restart, supervisor.drain -->

### The container listing

`show container list` answers one row for each configured container, sorted
by name, with the keys `name`, `state`, `reason`, `image`, `digest` and
`since`. The rows are read under one hold of the supervisor's lock, so a state
and the record that details it come from the same moment, and never under a
container's lifecycle lock, so a start or a stop in progress never delays it.

| State | What the row carries |
|-------|----------------------|
| `running` | `digest`, the image the run started from, and `since`, when its entrypoint was released |
| `waiting-image` | `reason` names the reference the store waits for and why it does not hold it, from the store's absent answer: never fetched, a fetch running, a fetch that failed (with its error) or an image the store lost (with the digest and the fetch that brings it back) |
| `failed` | `reason` is the failed record's cause, the step that failed and why, on one line |
| `disabled`, `stopped`, `restarting` | Nothing more: no restart policy and no exit record exist yet, so the command table's `restart-count` and `last-exit-status` are not in the rows |

`image` is the whole reference the container's image leaf completes to, in
every state. An enabled container no start has ended for holds no state yet;
a start in progress shows the state it left, so it is listed as `stopped`
with the reason that no start of it has ended. A container still in
`waiting-image` once the present notice or a changed image ended its wait
says that a start of it is in progress.

<!-- source: internal/plugins/container/list_cmd.go -- listCommandDecl, listCommandRPC, answerContainerList, supervisor.containerRows, waitingReason -->
<!-- test: internal/plugins/container/list_cmd_test.go TestContainerListShowsEachState -->

## The start sequence

Each configured container has one worker, a goroutine that lives while the
container is in the config. Apply wakes it, and it converges the container to
its config: a start when the container is enabled and neither runs nor waits
for its image, a stop when it runs and the config removed or disabled it. The
sequences of one container are serialized by its lifecycle lock, so the
worker, the restart command and the store's notices never interleave.

| Step | What it does |
|------|--------------|
| Resolve | The supervisor asks the store for the container's name and its configured `image`. The store answers `Ready` or `Absent`. Absent is the `waiting-image` state, and the store's present notice for the completed reference starts the container later |
| Hold | The store keeps the image and the folders until the cleanup ended |
| Contribute | Each participant may add read-only file mounts |
| Root | `mountRoot` mounts an overlay: the image folder is the read-only lower layer, and the upper and work directories are in the writable folder, so what the container writes lands on disk and the image never changes. The mount point is `/run/ze-container/root/NAME` |
| Describe | `describeContainer` fills the library's description: its own pid, mount, network, ipc, uts and cgroup namespaces, the library's default mounts, masked and read-only paths and device nodes, thirteen capabilities (the fourteen published images expect, less `CAP_MKNOD`, which is withheld until the control group's device allowlist exists: without the allowlist, root in the container could make a node for any host device and open it; the library's init makes the default nodes before it drops to this set, and `/dev/ptmx` is a symlink into the container's own devpts), no-new-privileges, the default system-call profile of that capability set ("The system-call filter"), the instance name as host name, the loopback device, and the control group `/ze-container/NAME` with the default limits, 512 MiB of memory with no swap beyond it and 256 processes. `describeEntrypoint` runs the image's entrypoint followed by its command, with its variables, working directory and numeric user |
| Claim | The kernel tunable `net.core.fb_tunnels_only_for_init_net` is claimed at 1 (below), under the lock that keeps a release from falling between the claim and the create |
| Create | `libcontainer.Create`, then `Container.Start`: the patched parent clones `ze` into the namespaces and the control group, and init blocks before the entrypoint |
| Created call | Each participant gets the init process id and the network namespace path |
| Release | `Container.Exec` lets init execute the entrypoint. The state is `running` |
| Output | Before the start, two pipes become the entrypoint's standard output and standard error, and one reader per pipe copies its lines into the container's output buffer ("The output", below). Standard input is not connected: the library opens `/dev/null` |
| Watch | One goroutine per running container waits for init to exit, then cleans up: destroy (the control group and the library's state directory), unmount the root, wait for the output readers, the ended calls, release the hold. An end of its own with a status other than 0 is recorded for the `container-stopped` check |

A failed step after the create destroys the container, makes the ended calls
and releases the hold. A failed `Container.Start` is destroyed by the library
wrapper itself: the patched parent made the control group before init started,
and the library's own cleanup is not in place yet at that point. A destroy
that fails is recorded as `doctor-container-cleanup-incomplete`.

The device filter of the control group is not set: the cgroups module's filter
package links the systemd driver, and the filter arrives with the devices
work. Until then the device nodes are made and no device access is denied. An
image user given by name is refused until the image's account files are read.

<!-- source: internal/plugins/container/supervise.go -- supervisor.work, supervisor.converge, supervisor.startLocked, supervisor.run, supervisor.watch, supervisor.cleanup, supervisor.stopLocked -->
<!-- source: internal/plugins/container/library_linux.go -- patchedLibrary.create, libraryContainer.kill, libraryContainer.destroy -->
<!-- source: internal/plugins/container/describe_linux.go -- describeContainer, describeEntrypoint -->
<!-- source: internal/plugins/container/rootfs_linux.go -- mountRoot, unmountRoot -->
<!-- test: internal/plugins/container/describe_linux_test.go TestDescriptionHasTheDefaultConfinement -->

### The output

The create makes two pipes and hands their write ends to the library's
process as the entrypoint's descriptors 1 and 2, the shape the plugin process
package uses for a forked plugin's stderr: a pipe the command did not create
is never closed by its wait, so a reader sees every byte. The daemon closes
its copy of the write ends once the start returned, so only the container's
processes hold them, and each reader reaches end of file once the control
group is empty. No idle bound applies: every writer dies with the group.

Each reader cuts its stream into lines and adds one row per line to the
container's buffer: a sequence number rising by one from 1, the time, the
stream and the line without its line end. A line longer than 4096 bytes
becomes several rows, cut at a character boundary, so nothing is lost and no
character is split. A byte sequence that is not UTF-8 becomes the replacement
character, and every control character but the tab is stripped, the carriage
return of a CRLF line end and the escape of a terminal sequence included, so
a row can neither drive a terminal nor forge a line of Ze's log.

The buffer holds the newest 1 MiB of rows; each row costs its line and 64
bytes, so a stream of empty lines is bounded too. A reader only copies into
memory, and the buffer drops its oldest rows to make room, so a container
never waits on its output. A read after a sequence number answers the rows
after it and how many rows after it were dropped, so a reader that fell
behind learns how much it missed. A container keeps its buffer across its
starts and loses it when it leaves the configuration; the buffer is in
memory, so a daemon restart loses it too. The commands that read it (`show
container log`, `monitor container log`) and the `log` setting that also
sends the rows to Ze's log, or keeps none, belong to the operator surface and
do not exist yet: every container keeps its buffer.

The destroy waits for both readers, so once the cleanup ended every line the
container wrote is in its buffer. A container whose entrypoint ended on its
own, with no stop asked for, takes the `stopped` state; with a status other
than 0 the watcher records it for the `container-stopped` check and logs it
at `Warn` under `doctor-container-stopped`, both carrying the status and the
newest five rows, each cut at 200 bytes, so the operator reads the
container's own explanation. The record lasts as long as the stopped state.
An end that a stop asked for (a config change, the restart command, the drain
or the plugin's exit) is not recorded.

<!-- source: internal/plugins/container/output.go -- outputBuffer, outputText, runeCut, outputSet, endedCause -->
<!-- source: internal/plugins/container/output_linux.go -- attachOutput, containerOutput, copyOutputLines -->
<!-- source: internal/plugins/container/supervise.go -- supervisor.watch, supervisor.recordFailedExitLocked -->
<!-- test: internal/plugins/container/output_test.go TestOutputBufferKeepsTheNewestBytes, TestOutputBufferBoundsEmptyLines, TestOutputTextStripsControlCharacters, TestContainerEndedOnItsOwnIsReportedWithItsOutput -->
<!-- test: internal/plugins/container/output_linux_test.go TestCopyOutputLinesCutsALongLineAtACharacterBoundary, TestCopyOutputLinesKeepsTheOrderOfAStream, TestOutputPipesNeverBlockTheContainer -->

### The stop

A stop, whether a config change removed or disabled the container, the
restart command asked for it or the daemon stops, sends the container's init
its stop signal: the config's `stop-signal`, else the signal the image
declares (a name, with or without `SIG`, or a number), else the termination
signal. An image signal Ze cannot read refuses the start, with the
`stop-signal` statement as the fix, rather than a guess that would end in a
kill. The signal and the stop timeout are fixed when the container starts, so
the stop of a container the config no longer holds still has them.

The stop then waits the container's `stop-timeout`. A container still running
after it is killed through its control group (`cgroup.kill`), which reaches
every process of it whatever session or group it joined, and the stop waits up
to 5 seconds for the cleanup. A container killed at its timeout is logged at
`Warn` and recorded for the `container-stop-killed` check
(`doctor-container-stop-killed`, with the `stop-timeout` statement as its
fix). Init of a pid namespace gets no signal it set no handler for, so a
workload that installs none is killed at its timeout.

The stop drain is the one call of a daemon stop that waits for a plugin, and
its budget of 37 seconds is declared at Stage 1, before any config. The drain
handler spends it in order: up to 2 seconds for the shell sessions (the shell
work's), up to 30 seconds for the stop signals, which is why `stop-timeout` is
at most 30, and 5 seconds for the kills and the cleanups. It stops every
container together: each gets its stop signal first, then each is waited for
until its own timeout counted from the same moment, so the drain waits the
longest timeout among them, never their sum. From the drain's start no
container starts, and the drain takes every container's lifecycle lock, so a
start in progress ends before the set to stop is read. A second signal during
the drain aborts it: the engine stops waiting and closes the connection, the
drain's context ends, and every container still running is killed at once,
with no stop-killed record, because its stop was cut short. When the
engine connection closes with containers still running, because the stop
skipped the drain, the plugin's exit kills them at once (`killRunning`).

<!-- source: internal/plugins/container/stop.go -- stopSignalOf, killCleanupTimeout -->
<!-- source: internal/plugins/container/supervise.go -- supervisor.stopTogether, awaitEnd, supervisor.drain, supervisor.killRunning -->
<!-- test: internal/plugins/container/stop_test.go TestStopSendsTheSignalThenKillsAfterTheTimeout, TestDrainSignalsEveryContainerBeforeAnyKill, TestForcedDrainKillsAtOnce -->

### The sweep of what a previous run left

A daemon that was killed or crashed leaves its containers running, because a
container takes a session of its own and the patched library refuses a
parent death signal. Containers are never adopted (the owner's ruling): before
the engine is asked to start the plugin, the sweep reads the plugin's own
directories, the control groups under `/sys/fs/cgroup/ze-container`, the root
mounts under `/run/ze-container/root` and the library's state directories
under `/run/ze-container/state`, and for each container it finds there it
kills the processes of its control group, waits up to 5 seconds for the group
to empty, removes it, unmounts the root, removes the mount point and removes
the state directory, which would otherwise refuse a new container of the same
name. Each participant gets the ended call for each leftover. Each is logged
at `Warn` by name under `doctor-container-cleanup-incomplete`. One the kernel
refused to remove whole is kept for the `container-cleanup-incomplete` check,
configured or not, with `ze signal reboot` as its fix, until that container
runs again. A configured container is then started as a new process when the
plugin has started, as at any boot.

<!-- source: internal/plugins/container/sweep_linux.go -- sweepLeftovers, sweepContainer, killControlGroup, removeRootMount -->
<!-- source: internal/plugins/container/supervise.go -- supervisor.sweep, supervisor.problemsOf -->
<!-- test: internal/plugins/container/stop_test.go TestSweepEndsEveryLeftoverBeforeAnyStart -->

### The kernel's fallback tunnel devices

Ze's kernel builds the tunnel drivers in, and the kernel gives every new
network namespace one fallback device of each tunnel kind (`gre0`, `gretap0`,
`erspan0`, `ip6gre0`, `ip6tnl0`, `sit0`, `tunl0`) unless the host-wide tunable
`net.core.fb_tunnels_only_for_init_net` says otherwise: 0 creates them in every
namespace, 1 in the initial namespace only, 2 in none. The kernel reads the
value when it creates a namespace. A container sees only the links Ze gives
it, its loopback device and its network attachments, so before every create
the plugin claims the value 1 through the `sysctl` plugin's default event,
under its own name as the source (`sysctlClaim`, `tunables.go`). 1 keeps the
devices in the initial namespace, where the host's own configuration may use
them. The emit reaches the `sysctl` plugin's handler before it returns, so the
claim reads the kernel's value back: a value other than 1 or 2 refuses the
start. The configuration outranks a plugin's default, so an operator who sets
the key to 0 sees each enabled container in the `failed` state, and `ze show
doctor` reports `doctor-container-failed` with the statement that sets it back
to 1. Once the configuration holds no enabled container, a worker clears the
claim and the `sysctl` plugin restores the kernel's original value, as it does
when the daemon stops. No other shipped code relies on the fallback devices in
a namespace it creates: the only namespaces Ze creates outside a container are
the throwaway namespaces of the kernel capability probes
(`kernelcap.InThrowawayNetworkNamespace`: the MPLS label space and the xfrm
interface kind), which ask for neither.

<!-- source: internal/plugins/container/tunables.go -- sysctlClaim, fallbackTunnelsKey -->
<!-- source: internal/plugins/container/supervise.go -- supervisor.createWithTunables, supervisor.releaseTunablesWhenIdle, supervisor.begin -->
<!-- test: internal/plugins/container/supervise_test.go TestTunablesAreClaimedBeforeEveryCreate, TestBootConfigureStartsNothingBeforeBegin -->

### The test store and its fixture image

A functional test needs an image and no network. A `zetest` build, the
functional-test daemon, has a test store (`storeseam_zetest.go`): every
container resolves to one fixture image, a folder holding `ze-containerprobe`,
which the harness builds beside `ze` and the store copies into the image. The
private key `ze.test.container.store` names the store's directory: the image
is `image/` and each container's writable layer `layer/NAME`. The key also
chooses the store: the engine entry builds the test store when the key is set
and the image store package's store otherwise (`engineStoreFactory`), so a
`zetest` daemon links both and registers only one, and a test that names no
fixture store runs the real store. The probe's `run` mode is the entrypoint: it
appends a line to `/starts`, writes what it sees to `/report`, and records its
stop signal in `/signals` before it exits. A container whose name starts with
`stubborn` gets the `run-ignoring-stop` mode, which records every stop signal
and keeps running until it is killed. A container whose name starts with
`talking` gets the `run-talking` mode, which writes one known line to each of
its standard output and standard error and exits with status 3; on the host,
`stopped-finding` waits until the daemon's doctor answer holds its stopped
finding with both lines. On the host, `verify` judges the report,
`confined` judges the default confinement in it (only its own process, its
cgroup as the root, the default capability mask, and the filter's EPERM for
`unshare(0)` and ENOSYS for call 470, after making both calls on the host to
show the host answers otherwise), `gone` waits until a container left no control group, root mount or library
state and judges the time from its stop signal to its end, `starts` counts
its starts and the processes of its control group, and `drive` runs a daemon
itself, for a second signal during the drain or a config reload.

A test puts its store at `/run/ze-container-test-$PORT`, never in its work
directory. In the QEMU guest the work directory is on the 9p share of the
checkout, and an overlay cannot take a 9p directory as its upper layer; `/run`
is tmpfs there and on the appliance. The probe's `reset` mode runs before the
daemon starts and removes the store, so a report an earlier run left behind
cannot pass the test. It refuses any path outside `/run/ze-container-test-*`.

<!-- source: internal/plugins/container/storeseam_zetest.go -- engineStoreFactory, testStore.Resolve, installProbe -->
<!-- source: internal/plugins/container/storeseam_engine.go -- engineStoreFactory -->
<!-- source: internal/test/containerprobe/main.go -- observe, verify, reset, storePrefix -->
<!-- source: internal/test/containerprobe/confinement_linux.go -- observeConfinement, confined -->
<!-- source: internal/test/containerprobe/network.go -- observeNetwork, reachFact, defaultRoute, judgeNetwork, hostEnd, unlinked -->

## Doctor codes, runtime checks and the kernel requirement

The plugin registers every code it owns from its own package
(`registerContainerDiagnosticCodes`), never from the always-on code table, so
a build without `ze_container` holds none of them. `ze show diagnostic <code>`
explains each one.

| Code | Problem | Reported by |
|------|---------|-------------|
| `doctor-container-not-privileged` | The process that judged a container config does not run as root | No check yet |
| `doctor-container-cgroup-unavailable` | The cgroup v2 hierarchy is not at `/sys/fs/cgroup`, or a controller cannot be enabled | No check yet |
| `doctor-container-seal-copies-binary` | The init binary can only be sealed by copying it into memory | No check yet |
| `doctor-container-out-of-memory` | A container was killed for memory | Runtime check `container-out-of-memory` |
| `doctor-container-process-limit` | A container reached its process limit | Runtime check `container-process-limit` |
| `doctor-container-stop-killed` | A container ignored its stop signal and was killed | Runtime check `container-stop-killed` |
| `doctor-container-cleanup-incomplete` | A container left processes, a control group or a mount behind | Runtime check `container-cleanup-incomplete` |
| `doctor-container-failed` | A container the configuration enables does not run: its last start failed | Runtime check `container-failed`, as an error |
| `doctor-container-stopped` | A container's entrypoint ended on its own with a status other than 0, and the container stays stopped | Runtime check `container-stopped`, with the status and the last output ("The output") |
| `doctor-container-port-blocked` | A published port is blocked by the operator's firewall policy: a base chain at the forward hook has policy drop and no accepting term names the network's bridge or the container's address | The network package's check `container-port-blocked`, as a warning, with the accepting term as `set firewall` lines (`network.md`, "The firewall policy and the port-blocked check") |
| `doctor-container-kernel-cpu-limit-unavailable`, `doctor-container-kernel-cpu-limit-unknown` | The kernel has no hard CPU limit, or Ze could not tell | The kernel enrolment `container-cpu-limit` |

The first three codes are registered and explained. No doctor check reports
them today: their checks arrive with the code that reads the host.

### Runtime checks

The six runtime checks are declared in the plugin's Stage 1 registration
(`engineRegistration`) at the `runtime` phase and answered by its doctor
callback (`supervisor.answerDoctorCheck`), because the records they read live
in the plugin's process, internal and forked alike
(`docs/architecture/doctor-and-health-checks.md`, "The runtime phase"). One
table, `runtimeDoctorChecks`, holds each check's declaration, the problem it
reports and its fix, so a declared check is an answered check. A name the
plugin did not declare is refused, never answered as healthy.

Each finding names the container and carries the command that fixes it: the
`set container instance NAME` statement of the limit or the stop timeout to
raise, or `ze request container restart name NAME` for a cleanup that did not
finish. A container that runs, or ran, is reported as a warning; a container
the configuration enables and that does not run is an error.

The supervisor holds the records: one entry for each container and kind of
problem (`supervisor.recordProblem`, `supervisor.problemsOf`). A record is
refused for a container the configuration does not hold, and a container that
leaves the configuration takes its records with it. The cleanup writes the
`container-cleanup-incomplete` record when a destroy fails or a stop does not
end. Where a start fails, `supervisor.setFailed` sets the `failed` state,
logs the failure at `Warn` under `doctor-container-failed` and writes the
`container-failed` record with its detail: the step that failed, why, the
underlying error, and the fix the refusal names (`failureDetail`). The record
lasts exactly as long as the state: a later start that runs or waits for its
image clears it, and so does a configuration that disables or removes the
container. Ze starts a failed container again at the next configuration commit
and at the restart command. The stop writes the `container-stop-killed`
record when it kills a container at its stop timeout, and the sweep keeps
each leftover it could not remove for the `container-cleanup-incomplete`
check (above). The out-of-memory and process-limit records need the code
that reads the control group's events, which does not exist yet, so those two
checks answer no finding, and the statements their fixes name (`memory`,
`process-limit`) are not in the schema yet; each arrives with the code that
writes its record.

<!-- source: internal/plugins/container/doctor.go -- containerDiagnosticCodes, runtimeDoctorChecks, runtimeDoctorCheckDecls, supervisor.answerDoctorCheck -->
<!-- source: internal/plugins/container/register.go -- registerContainerDiagnosticCodes -->
<!-- source: internal/plugins/container/container.go -- engineRegistration -->
<!-- source: internal/plugins/container/supervise.go -- supervisor.recordProblem, supervisor.problemsOf, supervisor.setFailed, failureDetail -->
<!-- test: internal/plugins/container/doctor_test.go TestContainerDoctorChecksAnswerThroughTheCallback, TestContainerDiagnosticCodesRegisterFromThePlugin, TestFailedStartIsReportedWhileTheContainerIsFailed -->

### The kernel requirement

A container's hard CPU limit needs CPU bandwidth control
(`CONFIG_CFS_BANDWIDTH`): without it a control group has no `cpu.max` file.
The plugin enrols it as the kernel capability `container-cpu-limit`
(`docs/architecture/doctor-and-health-checks.md`, "Kernel capabilities: one
enrolment, three callers"), so `ze config validate`, the start and a reload
refuse a configuration that holds a container to run on a kernel without it,
and doctor names it. A configuration whose containers are all disabled is not
judged: nothing runs.

A capability of this plugin is named `container-NAME`, and its two codes are
`doctor-container-kernel-NAME-unavailable` and
`doctor-container-kernel-NAME-unknown`. The probe reads whether
`/proc/sys/kernel/sched_cfs_bandwidth_slice_us` exists, a setting the
scheduler registers only under that option. It answers unknown, never absent,
when `/proc/sys/kernel` cannot be read, and unknown refuses nothing.

`gokrazy/kernel/runtime.config` requests `CONFIG_CFS_BANDWIDTH` and
`CONFIG_FAIR_GROUP_SCHED`, which it depends on. `gokrazy/kernel/runtime.require`
declares both, with `CONFIG_TMPFS` and `CONFIG_MEMFD_CREATE`, so a kernel
build that loses one fails. A runtime kernel built before this change holds
no `CONFIG_CFS_BANDWIDTH`.

<!-- source: internal/plugins/container/kernelcap_linux.go -- containerKernelCapabilities, containerInUse, cpuLimitProbe, sysctlPresence -->
<!-- source: internal/plugins/container/register_kernelcap_linux.go -- init -->
<!-- test: internal/plugins/container/kernelcap_linux_test.go TestContainerKernelCapabilitiesAreEnrolled, TestSysctlPresenceTellsAbsentFromUnknown -->

## The seams inside the plugin

The image store, the container networks and the shell are sibling packages of
this plugin and run in its process. Each registers from its own `init()`, and
the runtime package imports none of them.

| Seam | Declared in | Direction |
|------|-------------|-----------|
| Image store | `storeseam.go`: `Store`, `StoreNotices`, `SetStoreFactory`; `storeseam_operations.go`: `ImageOperations` | The runtime asks the store to resolve and hold a container (`Ready` or `Absent`, never an empty answer). The store tells the runtime that an image is present, or to restart a container. The image commands ask a store that is also an `ImageOperations`, and refuse when it is not ([image-store.md](image-store.md)) |
| Participants | `participant.go`: `Participant`, `RegisterParticipant` | Before create, a participant contributes read-only file mounts. After create and before the entrypoint runs it gets the created call, with the container's name, its init process id and its network namespace path. After the container's processes are gone it gets the ended call |
| Contributed mounts | `filemount.go`: `checkFileMount`; `describe_linux.go`: `contributedMounts` | The supervisor admits a contribution only when it is a regular host file (a symbolic link is refused), both paths are absolute and clean, the container path is not the root, lies under none of `/dev`, `/proc` and `/sys`, where the runtime mounts its own filesystems, and no other contribution names it. Anything else fails the start before the create, naming the participant. The description binds each admitted file at its container path, read-only, with no execution, no set-user program and no device |
| Config siblings | `sibling.go`: `RegisterConfigSibling`; `sibling_install.go`: `configInstaller` | A sibling's parse runs inside the plugin's one rule set, after the runtime's own nodes, over every section the plugin receives; its refusal is the config's, as the sibling's own operator error. Its apply runs before the supervisor's (below) |
| Config reads | `sibling.go`: `RegisterSiblingConfigRead` | A sibling adds a root the plugin receives without owning it, through the registry's `AddConfigRead`. A read root never loads the plugin |
| Engine client | `sibling_engine.go`: `EngineClient`; `sibling.go`: `RegisterEngineClientReceiver` | The owned bridges and links, the firewall tables, the state of the keys a sibling granted, and `EmitEvent` on the engine's event bus, which carries a sibling's sysctl defaults to the sysctl plugin (the networks' forwarding defaults, [network.md](network.md)), over the plugin's one engine connection. It is handed before the start-up sweep, and refuses every request at once until the engine started the plugin |
| Sibling commands | `sibling.go`: `RegisterSiblingCommand` | A sibling's command is declared at Stage 1 with the plugin's own, its proxy is registered on its wire method, and `handleCommand` answers it with the sibling's handler. The YANG node that binds the wire method is the sibling's to add to `yang/ze-container-cmd.yang` |
| Sibling doctor checks | `sibling.go`: `RegisterSiblingDoctorCheck` | A sibling's check is declared at Stage 1 with the runtime's, and the doctor callback answers it with the sibling's answer. The sibling registers the explanation of each of its codes itself |
| Runtime library | `supervise.go`: `runtimeLibrary`, `createdContainer` | The supervisor orders create, release, wait, kill and destroy through an interface, so the order of a start is proven with a library that creates nothing. `library_linux.go` implements it over the patched library; off Linux there is none and a start is refused |
| Filter | `seccomp/register_linux.go`: `initFilter` | The library's `InitSeccomp` calls the function the filter package registered, which builds and loads the filter of the profile ("The system-call filter", above) |

<!-- source: internal/plugins/container/storeseam.go -- Store, StoreNotices, SetStoreFactory -->
<!-- source: internal/plugins/container/storeseam_operations.go -- ImageOperations -->
A sibling's apply runs before the supervisor's, in registration order, so
what the network makes (its bridges and tables) exists before a container of
the new config starts. The supervisor's apply returns before any container of
the old config stops, so a sibling keeps what a running container uses until
that container's ended call, and removes it then. A sibling's apply is called
at every install and is idempotent. The boot configure arrives before the
engine answers the plugin's requests, so the siblings' part of the boot config
is applied when the engine reports the plugin started, before the supervisor
starts any container; a refusal then fails the plugin's startup, as a boot
config the plugin cannot install does. A refusal at a reload refuses the
commit: the siblings that applied before it get their previous config back,
the supervisor's apply is not made, and when a put-back fails too the error
carries the SDK's running-config-lost mark, so the rollback installs the
running config again.

Go runs the runtime package's `init()` before any sibling's, because a
sibling imports the runtime. The registration the plugin hands the plugin
registry is built in that `init()`, so its command list holds no sibling
command: a sibling's command reaches a running daemon through the Stage 1
declaration only, and the registry's copy (read by the command catalog)
lacks it. The same order keeps a sibling from adding a start-order edge: the
registry has a late call for a config read (`AddConfigRead`) and none for
`Dependencies`, `OptionalDependencies` or `StartAfter`.

<!-- source: internal/plugins/container/participant.go -- Participant, RegisterParticipant -->
<!-- source: internal/plugins/container/filemount.go -- checkFileMount, runtimeMountPoints -->
<!-- source: internal/plugins/container/describe_linux.go -- contributedMounts -->
<!-- source: internal/plugins/container/sibling.go -- RegisterConfigSibling, RegisterSiblingConfigRead, RegisterSiblingCommand, RegisterSiblingDoctorCheck, RegisterEngineClientReceiver -->
<!-- source: internal/plugins/container/sibling_engine.go -- EngineClient, startedEngine -->
<!-- source: internal/plugins/container/sibling_install.go -- configInstaller -->
<!-- test: internal/plugins/container/sibling_test.go TestSiblingParseRefusalRefusesTheConfig, TestSiblingApplyRunsBeforeTheSupervisorsApply, TestSiblingApplyFailureRefusesTheCommit, TestSiblingCommandIsDeclaredAndDispatched, TestSiblingDoctorCheckIsDeclaredAndAnswered, TestEngineClientIsHandedAndRefusesUntilStarted -->
<!-- test: internal/plugins/container/filemount_test.go TestCheckFileMountAdmitsOnlyAReadOnlyRegularFile, TestAdmittedContributionReachesTheCreate, TestInadmissibleContributionFailsTheStart -->
<!-- test: internal/plugins/container/describe_linux_test.go TestDescriptionMountsContributedFilesReadOnly, TestRuntimeMountPointsCoverTheDefaultMounts -->
<!-- source: internal/plugins/container/supervise.go -- runtimeLibrary, createdContainer -->
<!-- source: internal/plugins/container/seccomp/register_linux.go -- initFilter, installFilter -->

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

A bump of `golang.org/x/sys` can add or renumber a system call:
`TestSyscallTablesMatchVendoredNumbers` then fails until the two tables of the
filter are regenerated from the new `zsysnum_linux_amd64.go` and
`zsysnum_linux_arm64.go`. A refresh of the default profile from a newer Moby
commit replaces `third_party/moby-seccomp-profile/default.json` and its
README, and `TestDefaultProfileFollowsTheSourceProfile` fails until
`profile.go` follows it.
