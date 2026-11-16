# Spec: container-1-runtime

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P3 |
| Release reason | A post-release feature by owner ruling (O-3 of `spec-container-0-umbrella.md`): Ze runs OCI container images itself. No shipped behavior is wrong without it. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This is the Runtime child of `plan/post-release/features/spec-container-0-umbrella.md`. The umbrella holds the owner's rulings and the approved design: the upstream runtime library used whole, behind a diff, with cgo off. This child makes one container start, stay confined, be supervised and be removed without a trace. It takes its root filesystem and its volumes from the image-store child, and it leaves the network, the image store and the words an operator types to the other children.

Symptom today: Ze has no code that creates a cgroup, enters a namespace for a child, drops capabilities for a child or installs a system-call filter, and no plugin owns a container.

Goal: a container that an operator configured runs its entrypoint inside its own namespaces and a limited cgroup, with reduced capabilities and a system-call filter. Ze restarts it under its policy, stops it with a signal then a kill, and removes every process, cgroup and mount of it.

### Rules this child takes from the owner (2026-11-16)

| Rule | What it fixes here |
|------|--------------------|
| Words | The operator-surface child (`spec-container-4-operator-surface`) holds the one table of config words and command words. Its design is settled before this child's wiring phase starts. Depends stays `-`: this child lands first, with that table's words, and designs no grammar |
| Where containers run | On the appliance and under a daemon that runs as root. A daemon that runs as an unprivileged account refuses a container config and says why. No systemd unit work is in scope |
| Capabilities | The default set is the fourteen that published images expect |
| Limits | Mandatory, with defaults: memory 512 MiB, processes 256. A hard CPU limit is offered and adds one kernel option. No I/O throttle |
| Daemon restart | When Ze restarts, including for its own update, containers are stopped and started again. A crashed daemon's leftovers are swept at the next start. Running containers are never re-adopted |
| Volumes | Stored by Ze, with no host path. They are the image-store child's |

### What this child builds

| # | Piece | Where it lands |
|---|-------|----------------|
| 1 | The upstream runtime as a module dependency, in a commit of its own | `go.mod`, `go.sum`, `vendor/` |
| 2 | The diff against the upstream release, with its review stamp and guard tests | `internal/le/vendorpatch/patches/`, the patched files under `vendor/` |
| 3 | The container init role, selected by the library's environment key before command dispatch | A new always-on leaf under `internal/core`, filled by the gated plugin |
| 4 | A pure-Go system-call filter, its per-architecture tables and the default profile | A `seccomp` package inside the plugin |
| 5 | The container description, the root overlay, the cgroup limits, the volume and memory-backed mounts | The plugin, under `internal/plugins` |
| 6 | Supervision: start, the created call, release, wait, restart, stop, cleanup, leftover sweep | The plugin |
| 7 | The plugin registration with its config root and verifier entry point, the gate, the build-tag tests, kernel enrolment, its doctor checks and codes, state keys, output capture | `feature-gates.txt`, `cmd/ze/hub`, the plugin, `pkg/zefs/keys.go` |
| 8 | The `container` functional suite, since this child lands first | A new suite directory under `test`, and its registration in `le` |

### The diff against the upstream release (runc v1.5.3)

One patch series, one module, stamped `Ze-Reviewed-Against: github.com/opencontainers/runc v1.5.3`. Each hunk is a patch file of its own: six files from this child. The shell child rewrites the file of H-3 and adds a seventh file, over the same five upstream files.

| Hunk | Upstream function or file | Change | Why |
|------|---------------------------|--------|-----|
| H-1 | `libcontainer/container_linux.go` `newInitProcess` | Sets the clone flags from the configured namespaces and a new session on the init command. Creates the container cgroup without attaching a process, opens its directory, and asks the standard library to clone init into it. Builds no bootstrap data. Refuses a namespace with a path, time offsets, identity mappings and a non-zero parent death signal | The C stage did the namespace entry. The standard library does it at clone |
| H-2 | `libcontainer/process_linux.go` `initProcess.start` | Removes the write of the bootstrap data, the read of the child pid and the wait for the first child. Uses the started process as init. Writes the out-of-memory score of init | Those three steps are the C stage's protocol |
| H-3 | `libcontainer/container_linux.go` `newSetnsProcess` | Refuses, with an error naming the missing join | Entering a running container needs the C join. The shell child owns it |
| H-4 | `libcontainer/factory_linux.go` `Create`, `Load` | Builds the cgroup v2 manager directly and refuses a host that is not in unified mode | It is the only importer of the manager constructor that links the systemd driver and the cgroup v1 manager |
| H-5 | `libcontainer/criu_linux.go`, `criu_disabled_linux.go` | Inverts the build constraint, so the checkpoint client compiles only under a tag no Ze build sets | Ze derives build tags from `feature-gates.txt` and only for the `ze_` prefix, so no Ze build can carry the upstream opt-out tag |
| H-6 | `libcontainer/seccomp/seccomp_unsupported.go` | `InitSeccomp` calls a function a Go package registers, and refuses a profile when none registered | The filter itself is Ze code, linted and tested in Ze's tree, never a file inside the patch |

### Seams with the other children

Every sibling is a package of the one `container` plugin and runs in its process, so each seam is a Go boundary inside the plugin tree.

Image store (its section "The seam to the runtime child"). The seam has one record, and both specs state it field for field.

The request, "resolve this container":

| Field | Type | Description |
|-------|------|-------------|
| instance name | text | The container's name |
| reference | text | The image reference the config names, as written |
| storage size | number | The `storage` value: the cap of the writable layer |
| volumes | list of name, destination and size | One entry for each `volume` the config holds |

The answer "ready":

| Field | Type | Description |
|-------|------|-------------|
| digest | text | The image digest the container runs |
| image folder | absolute path | The unpacked image, the read-only lower layer of the root |
| writable layer folder | absolute path | One directory for this container and this digest, holding the upper and work directories of the overlay |
| volume folders | list of name, destination and absolute path | One folder for each volume of the request, created and capped |
| entrypoint | list of text | The image's entrypoint, as fetched |
| command | list of text | The image's default command, as fetched. A list of its own, never joined to the entrypoint |
| environment | list of text | The variables the image declares |
| working directory | text | The directory the entrypoint starts in |
| user | text | The image's user as written: a name, a number, either with a group, or empty. The store resolves nothing |
| stop signal | text | The signal the image declares for a stop, empty when none |
| exposed ports | list of text | The ports the image declares, for display only |
| declared volumes | list of text | The paths the image declares as volumes. No folder is made for them |

The answer "absent" carries the completed reference and the reason: never fetched, fetch running, or the last fetch failed and why. It is never an empty value. Absent is the `waiting-image` state.

| Direction | Question or notice | Answer, and what this child does |
|-----------|--------------------|----------------------------------|
| Runtime to store | Resolve this container | Ready or absent, as above |
| Runtime to store | Hold this resolution; later, release it | Held from the start of a container to the end of its cleanup, then released. The store refuses to remove what is held |
| Store to runtime | The image for this reference is now present | Starts each container that waited for it and answers their names |
| Store to runtime | Restart this container on its new generation | Stops it, resolves again, starts it, and answers when it runs or why it does not |
| Store to runtime | Stop this container, then start it again | The same, for the clear command |

What this child does with the image's configuration, which the store hands as fetched:

| Image field | Rule |
|-------------|------|
| Entrypoint and command | The config's `entrypoint` replaces the image's entrypoint, and `argument` replaces the image's command. Each stands when the config is silent |
| Environment | The image's variables, then the config's `environment` entries, which win on the same name |
| Working directory | The image's. It is created in the root when absent |
| User | The config's `user` and `group` when set. Otherwise the image's, as written: a number is used as it is, and a name is resolved in the image's own account files, opened under a descriptor for the root. A name those files do not hold fails the start |
| Stop signal | The config's `stop-signal` when set. Otherwise the image's, or the termination signal when the image names none |
| Declared volumes | A path the image declares and the config's `volume` list does not is not mounted: it stays a directory of the writable layer, which persists until the image is updated. The image-store child states the same rule, and the owner's closing test relies on it |
| Exposed ports | Carried in the record for display. This child does not use them |
| Labels | Not in the record |

Network (its section "The seam with the runtime child", and its assumption A-3):

| # | The network child needs | This child provides |
|---|-------------------------|---------------------|
| 1 | A call after the container is created and before its entrypoint runs, with the container name and the init process id. The start continues only on success; a failure destroys the container and records the reason | The created call: each registered participant is called once, under a deadline, with the name, the init process id and the network namespace path. An error destroys the container and puts the participant's name and reason in the container's state |
| 2 | The network mode as an input when the container is built | `host-namespace` holding `network` builds no network namespace. The created call is not made for that container, because it has no network namespace of its own to attach. Its description contribution and its ended call are made as for any container |
| 3 | A call when a container has exited or is destroyed | The ended call: made once for every container this child created, after its processes are gone, and for each leftover the sweep removes |
| 4 | A place in the root for a resolver file, written before the entrypoint runs | The description contribution: before create, each participant may contribute read-only file mounts, and nothing else. The resolver file is one. The network child keeps it in a directory of its own and hands this child the host path and the path inside the container. This child mounts it read-only and removes nothing it did not create: the file and its directory are the network child's to remove |
| - | The loopback device up before call 1 | The description names the loopback device, and the library's init brings it up at the start of its sequence, before it blocks |

Shell (its assumption A-12): the supervisor keeps the library's container object for every container that is not ended, and gives it to a caller inside the plugin by container name. The drain handler ends the shell child's sessions first, then stops the containers, inside the one drain budget whose arithmetic is in Key Design Decisions. When the shell child lands, it rewrites the patch file of H-3, adds a seventh file to the series, extends the named guard test, replaces `TestPatchedLibraryRefusesToEnterAContainer`, and removes this spec's limitation about entering a container.

Operator surface: the words below, and one read function that answers this child's facts for the state view (state, reason, digest, process id, restart count, last exit status, since, the limits in force, resource use), plus the output rows.

### The words this child uses, from the operator-surface child's tables

Config nodes this child realizes, under `container instance NAME` (its table S-1, rows marked "runtime"):

| Node | What this child does with it |
|------|------------------------------|
| `enabled` | False keeps the container stopped, in the `disabled` state |
| `entrypoint`, `argument` | Merge with the image's, by the rule above |
| `environment` | The workload's variables. The owner fixed this word; a value never reaches a listing, a log line or a finding |
| `hostname` | The host name in the container's own UTS namespace. The instance name when absent |
| `restart` | `never`, `on-failure`, `always`. Default `on-failure` |
| `stop-timeout` | Seconds between the stop signal and the kill, 1 to 30 |
| `memory` | The memory limit in mebibytes. Default 512. Floor 64 |
| `shared-memory` | The size of the shared-memory mount, counted in `memory` |
| `cpu` | The hard CPU limit in percent of one core. Default 100 |
| `process-limit` | The most processes and threads at once. Default 256 |
| `capability` | Capabilities added to the default fourteen |
| `capability-drop` | Capabilities removed from the default fourteen. `all` removes the whole set, and `capability` then adds back what is named |
| `stop-signal` | The signal a stop sends, in place of the image's |
| `host-namespace` | `network`, `pid`, `cgroup`: the container shares that namespace of the host and gets none of its own |
| `user`, `group` | Numeric identity of the entrypoint, in place of the image's |
| `log` | `buffer`: the bounded output buffer. `system`: the buffer and Ze's log. `none`: the output is read and dropped |
| `sysctl` | A kernel setting written inside the container's own namespaces |
| `device` | A host device node made in the container's device directory and allowed in its device filter |
| `tmpfs` | A memory-backed mount of a mandatory size, counted in `memory` |
| `volume` (the mount only) | The folder the store answers for the volume is mounted at `destination`, read-only when `read-only` is true |

Verify rules of its table V-1 that land here: V-2 (a `sysctl` outside the per-namespace settings), V-3 (memory-backed sizes above `memory`), V-7 (one path named twice), V-8 (`cpu` above 100 times the host's processors), V-10 (one capability both added and dropped) and V-11 (a container config under a daemon that does not run as root). The message of each rule is that child's table E-1. Its code and its tests are this child's, in the plugin's package. V-11 is judged where the daemon runs, at boot, reload and commit, and by doctor (AC-15, and the Key Design Decisions row on host privilege). That child's table lists V-11 among the rules `ze config validate` judges with no daemon. The two specs differ on that one point, and it waits for a ruling.

Commands: this child lands `request container restart name`. It stops and starts a running container, starts one in `stopped` or `failed`, and is refused for `disabled` and `waiting-image` with the fix that table names. There is no start command and no stop command: `enabled` states whether a container runs. The listing, the detail, the log and its stream are the operator-surface child's; they read this child's facts.

### Container state

The states an operator sees are the six of the operator-surface child's table T-1. This child produces them:

| State | When this child sets it | Leaves to |
|-------|-------------------------|-----------|
| `waiting-image` | The store answered absent | `running` or `failed`, when the store says the reference is present |
| `running` | The entrypoint was released. Covers a container that received its stop signal and has not ended yet | `restarting`, `stopped`, `failed` |
| `restarting` | The entrypoint ended, the policy restarts it, and the delay is running. `reason` gives the time of the next start | `running`, `failed`, `stopped` |
| `stopped` | The entrypoint ended and the policy keeps it stopped | `running`, by the restart command or a config change |
| `disabled` | `enabled` is false | `running`, by config |
| `failed` | A start step failed five times in a row, and Ze stopped trying. `reason` names the step in plain words | `running`, by the restart command or a config change |

A start in progress shows the state it left, and a failed start that will be tried again shows `restarting`. Only five failed starts in a row give `failed`. An entrypoint that keeps exiting is restarted under its policy with a growing delay and no bound, and the delay returns to one second after a run of one minute. These internal steps map onto the six states and add none: the operator-surface child's table T-1 states the six in the same words.

### Problem classes and their doctor codes

The operator-surface child owns the codes that follow the state (`doctor-container-image-absent`, `doctor-container-restarting`, `doctor-container-stopped`, `doctor-container-failed`); this child supplies the state and the reason they read. This child owns these:

| Problem class | Code | Phase | Fix the finding names |
|---------------|------|-------|-----------------------|
| The daemon cannot create namespaces or cgroups | `doctor-container-not-privileged` | config | Run the daemon as root |
| The cgroup v2 hierarchy is not at `/sys/fs/cgroup`, or a controller cannot be enabled | `doctor-container-cgroup-unavailable` | config | The host setting that mounts or delegates it |
| A kernel option is absent or cannot be probed | `doctor-container-kernel-feature-absent`, `doctor-container-kernel-feature-unknown` | config, through the kernel enrolment | Rebuild the kernel with the named option |
| The init binary can only be sealed by copying it into memory | `doctor-container-seal-copies-binary` | config | The host setting that allows the overlay mount |
| A container was killed for memory | `doctor-container-out-of-memory` | runtime | `set container instance NAME memory` with a larger value |
| A container hit its process limit | `doctor-container-process-limit` | runtime | `set container instance NAME process-limit` with a larger value |
| A container ignored its stop signal and was killed | `doctor-container-stop-killed` | runtime | `set container instance NAME stop-timeout` with a larger value |
| A previous run left processes, cgroups or mounts, or a cleanup could not finish | `doctor-container-cleanup-incomplete` | runtime | `ze request container restart name NAME`, or a reboot when the kernel refuses the removal |

### The owner's closing test (umbrella AC-14)

The operator-surface child owns that test: the published Redis image runs as a container and is reached from outside. Nothing here prevents it, and it rests on these rows:

| What that image does | Rests on |
|----------------------|----------|
| Starts as root and switches user in its entrypoint, after changing the owner of its data directory | AC-7: the default fourteen capabilities hold the three it needs (set user, set group, change owner), and the default filter profile allows those calls. No-new-privileges does not stop it: the switch is a system call by root, not a set-user program |
| Declares a volume for its data directory, which the config may leave undeclared | The declared-volume rule above: the directory stays in the writable layer |
| Writes to its data directory and finds the data after a restart | AC-9 |
| Listens on a TCP port | AC-2: the created call gives the network child its namespace. Publishing the port is that child's |
| Runs inside the default limits | AC-3, AC-4: 512 MiB and 256 processes |

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/features/spec-container-0-umbrella.md` - the approved design, the owner's rulings, and the corrections table
  → Decision: the library is used whole behind a diff, with cgo off; images are folders; the writable layer is on disk; the gate is on by default
  → Constraint: the container's link is made by Ze's interface backend at the created call, so this child never fills the library's network-device list
  → Constraint: the operator-surface child's tables hold every word, and the image-store child holds volumes, the size caps and the data directory
- [ ] `ai/doors/system.md`, `ai/doors/system-plugins.md`, `ai/doors/plugin-infra.md` - the plugin contract
  → Constraint: the plugin runs internal or forked with the same code, so state goes through the SDK state calls, the stop through the drain call, live doctor state through the doctor callback
  → Constraint: always-on code never imports the gated package, so the init role reaches the library through a registration in an always-on leaf
- [ ] `ai/doors/forwarding.md` - shared kernel state and the report bus
  → Constraint: the report bus is process-local, so the plugin raises and clears its warnings through the SDK issue calls, never through the core report package
- [ ] `ai/doors/hub-engine.md`, `docs/architecture/api/process-protocol.md` "The stop drain", "Shutdown" - how a daemon stop reaches a plugin
  → Decision: a whole-daemon stop closes connections with no `bye`, and a forked plugin is killed one second later. The only call that waits for a plugin before the stop is the drain, so containers are stopped in the drain handler
  → Constraint: Stage 1 refuses a drain budget above 60 seconds, and the budget is declared before config arrives. The stop timeout is therefore at most 30 seconds
- [ ] `ai/doors/config-yang.md` - config delivery and offline validation
  → Constraint: `ze config validate` never calls a live verify callback, so every config rule lives in one function that the in-process verifier, the live verify and the boot configure all call
  → Constraint: a removed root arrives as an empty body, which the parser reads as "stop every container", never as "no change"
- [ ] `ai/doors/core.md`, `ai/doors/appliance.md` - state and the device contract
  → Constraint: a state key and a path under the persistent partition are permanent once an image ships, so this child adds one key template and no path of its own
- [ ] `docs/contributing/patching-the-standard-library.md` "Patching a vendored module", "The review stamp" - how Ze changes a third-party module
  → Decision: the diff is a file under `internal/le/vendorpatch/patches/`, applied by `./le setup install` when it re-vendors, and the patched vendor files are committed. A vendored module is not patched at each build
  → Constraint: one patch changes one module and carries a review stamp. A module bump refuses until the upstream functions are read again and the stamp is changed
- [ ] `docs/contributing/dependency-approval.md` - the dependency commit
  → Constraint: the commit gate reads `go.mod` from HEAD, so the module lands before any commit that imports it
- [ ] `docs/architecture/plugin/feature-gates.md` - the gate
  → Decision: plugin compile-out. Manifest lines under `ze_container`, no source tag on the plugin's files, a present and an absent build-tag test
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor and kernel enrolment
  → Constraint: a check over the plugin's live state is declared at Stage 1 and answered by the doctor callback. A registry check function carries a config phase only
  → Constraint: a kernel enrolment that cannot be determined never refuses a start
- [ ] `ai/rules/platform-linux.md`, `docs/architecture/testing/qemu-integration.md` - the proof route
  → Decision: a `.ci` marked `option=needs-linux` runs in Ze's own-system guest through `./le test qemu ci test <path>`. The daemon binary there sits on a memory filesystem, so a claim about the squashfs root needs a booted appliance image
- [ ] `ai/rules/go-standards.md`, `ai/allowed-system-commands.md` - external programs
  → Constraint: the entrypoint is a program Ze does not carry. Its register row lands in the commit that adds the init role (O-4, authorised 2026-11-16)

### Upstream source (read at the tag, outside this repository)
- [ ] `runc` v1.5.3, the files `container_linux.go` (`newParentProcess`, `newInitProcess`) and `process_linux.go` (`initProcess.start`) of its `libcontainer` package
  → Constraint: the init command is fixed to the sealed binary with the argument `init`, and the caller cannot set process attributes. Clone flags can only come from a diff
  → Constraint: the parent attaches init to its cgroup after the start, and the C stage creates the namespaces after that attach. A clone that carries the cgroup namespace flag must therefore put init in its cgroup at the clone, or the container's cgroup namespace is rooted at the daemon's cgroup
- [ ] `runc` v1.5.3, `standard_init_linux.go` (`linuxStandardInit.Init`) and the stub file of its `seccomp` package
  → Constraint: the filter is loaded by init at the library's own point, immediately before the exec when no-new-privileges is on. The stub refuses any profile
- [ ] `runc` v1.5.3 `factory_linux.go` (`Create`); the cgroups module v0.1.0, its manager constructor, its v2 default path file and its root config file
  → Decision: one hunk removes the systemd driver and the cgroup v1 manager from the link. The cgroups root package still names a systemd property type, so two modules arrive as types
  → Constraint: the library fixes the cgroup v2 mount point at `/sys/fs/cgroup`
- [ ] Go 1.27.4, the standard library's Linux process-start file, `SysProcAttr`
  → Decision: clone flags and a cgroup descriptor combine in one clone call
  → Constraint: a parent death signal makes the child compare its parent pid, which reads zero inside a new process namespace, and kill itself

**Key insights:** (minimal context to resume after compaction)
- The diff has six hunks in five upstream files, one patch file per hunk.
- The filter is Ze code behind a registration seam in the library's stub file.
- A container is killed through its cgroup, forked by the library with no death signal, and its output ends at end of file once its cgroup is empty. No file of the plugin supervisor is edited.
- The daemon stop reaches containers through the stop drain call, on both transports.
- A daemon that died leaves containers running. The next start removes them before it starts anything. Nothing is re-adopted.
- The init role runs the whole `ze` binary's start-up inside the container's cgroup, so the memory limit has a floor.
- The store's answer carries the image's configuration as fetched. This child merges it with the config and resolves a user name.
- Every word an operator types is the operator-surface child's. Tests live in the `container` suite, which this child registers.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `cmd/ze/dispatch.go` - `dispatchMain` calls `seedinit.Dispatch` first, then starts crash logging, then dispatches the command
- [ ] `internal/core/seedinit/register.go` - an always-on leaf that selects a role from an environment key and runs a handler another package registered
- [ ] `internal/core/crashlog/exec_unix.go` - the one execve wrapper, needed only where crash logging redirected standard error
- [ ] `internal/component/plugin/sysproc_linux.go` - `NewSysProcAttr` sets a process group and a death signal; `StartChild` forks from one thread that lives as long as the daemon
- [ ] `internal/component/plugin/process/exitwait_linux.go` - `waitExitedUnreaped` holds an exited pid so a group kill cannot reach a recycled pid; `newPluginShell` starts the plugin with no context so only the group kill signals it
- [ ] `internal/component/plugin/server/drain.go` - `BeginDrain` and `DrainRound.Run` ask every plugin that declared a drain budget to finish before connections close
- [ ] `internal/core/statestore/permissions.go` - `RegisterPluginKeys` grants a plugin its state keys from compiled registration
- [ ] `pkg/zefs/keys.go` - the registered key templates; a per-plugin record is a `meta/` key with one template parameter
- [ ] `internal/le/repo/featuretags/daemontags.go` - `DaemonTags` returns only the `ze_` tags of the manifest
- [ ] `cmd/ze/hub/build_tag_vpp_absent_test.go` - the shape of an absent build-tag test
- [ ] `internal/le/test/qemu/mplsboot.go`, `test/appliance/appliance-mpls-qemu.ci` - a proof that boots an appliance image with a seed config and asks the Ze CLI
- [ ] `ai/allowed-system-commands.md` - the register, with one row of the same kind today: the operator's plugin program
- [ ] `docs/contributing/patching-the-standard-library.md` - the vendor patch mechanism, its stamp and its guard tests

**Behavior to preserve:**
- `ze init` and the session seed role keep their behavior: the container role is selected by an environment key no other caller sets, before either.
- The plugin supervisor, its respawn bounds and its stop are unchanged. No file of the plugin process package is edited.
- Every existing vendor patch still applies, and `TestEveryVendorPatchIsApplied` stays green.
- A build without `ze_container` behaves as today.

**Behavior to change:**
- None. This is a new feature behind a compile-time gate. The one always-on edit is a second early dispatch call in `dispatchMain`, which answers "not handled" when the key is absent.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Diff against a third-party module | `docs/contributing/patching-the-standard-library.md` "Patching a vendored module"; `./le setup install` | Holds one diff per change with a review stamp, applies it when the vendor tree is rebuilt, and proves with guard tests that it is applied and that the vendor tree equals upstream plus the patches | Extends: one new patch series for the runtime module and one named guard test. No new tooling |
| Appliance build bound to vendored sources | `docs/contributing/patching-the-standard-library.md` and the gokrazy build pins page, "Binding the appliance to vendored product sources" | Makes the appliance build compile the root `vendor/` tree, patches included | Extends by use: the patched runtime reaches the appliance binary with no change |
| Role selected by an environment key before startup | `cmd/ze/dispatch.go` `dispatchMain`; `internal/core/seedinit/register.go` `Dispatch`, `Register` | Runs one registered handler when its key is set, before crash logging starts | Extends: a second early dispatch through a new always-on leaf that holds roles by key, so always-on code names no feature. The seed role stays where it is: its contract checks arguments and descriptors the container role has none of |
| One execve wrapper | `internal/core/crashlog/exec_unix.go` `Exec` | Drains the crash-log pipe before any execve | Does not apply: the library performs the final exec inside the init role, which runs before crash logging starts, so no pipe exists to drain |
| Plugin process attributes and forking thread | `internal/component/plugin/sysproc_linux.go` `NewSysProcAttr`, `StartChild` | Forks a plugin in a process group with a death signal, from a thread that never exits | Does not apply: the library forks init itself and the diff refuses a death signal, so no thread binding exists. A container leaves the daemon's process group by taking a new session |
| Pid-safe group kill | `internal/component/plugin/process/exitwait_linux.go` `waitExitedUnreaped`, and the group kill its comments name | Kills a plugin's process group without reaching a recycled pid | Replaced for containers by the cgroup: every process of a container is in its cgroup whatever group it joined, and the kernel's cgroup kill reaches them all with no pid to recycle |
| Stderr relay of a forked plugin | `docs/architecture/api/process-protocol.md`, the external plugin start; the relay reader of the plugin process package | Hands the child a pipe as a file and reads it into the engine log, with an idle bound for a descendant that outlives the plugin | Model only: the same pipe-as-file shape captures a container's two streams. The idle bound does not apply, because every writer dies with the cgroup |
| Stop drain | `internal/component/plugin/server/drain.go` `BeginDrain`, `DrainRound.Run`; the SDK's `OnDrain` | Gives a plugin up to its declared budget, at most 60 seconds, before a daemon stop closes its connection, on both transports | Extends by use: the plugin declares a budget and stops every container in its drain handler |
| Failure policy and respawn bounds | `docs/architecture/api/process-protocol.md` "Failure-Policy Declaration", "A restart is bounded" | Restarts a plugin that declared `restart`, up to 5 times in 60 seconds and 20 in total | Extends by use: the plugin declares `restart`. Each new generation sweeps what the last one left |
| Restart delay loop of the VPP manager | The umbrella's row "VPP process supervisor" | Restarts VPP with a delay that doubles from 1 to 30 seconds | Model: the same numbers for a container's restart delay. Not extended as code: it belongs to another component and has no stop signal |
| Plugin state calls | The SDK's `StatePut`, `StateGet`; `internal/core/statestore/permissions.go` `RegisterPluginKeys`; `pkg/zefs/keys.go` | Persist small values under registered keys, for an internal or a forked plugin | Extends: one key template, `meta/container/{name}`, for a container's record |
| Doctor checks and codes | `docs/architecture/doctor-and-health-checks.md` "The runtime phase", "The phase decides what a check may read" | A config-phase check registers from its owner; a live-state check is answered by the plugin | Extends: the checks of the problem-class table |
| Kernel requirement enrolment | `docs/architecture/doctor-and-health-checks.md` "Kernel capabilities: one enrolment, three callers"; `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` | Refuses a build without a required option, and refuses validate, start and reload on a kernel that lacks one | Extends: one enrolment per option, and one option added to the kernel files |
| cgroup v2 mount locator | The umbrella's row "cgroup v2 mount and its locator" (`cgroup2Root` in the IKE dataplane package) | Finds the mount point from the mount table | Does not apply: the library fixes the mount point at `/sys/fs/cgroup`, so a host that mounts it elsewhere is refused with a doctor code. Nothing is moved to a core leaf |
| Feature-gate manifest and build-tag tests | `feature-gates.txt`; `cmd/ze/hub/build_tag_vpp_absent_test.go` and its siblings | Declares a gate and proves it present and absent | Extends: the plugin's lines and one test pair |
| Functional suites | `ai/doors/le-tooling.md` "Tests"; `docs/architecture/testing/qemu-integration.md` "The two entry points" | A suite is a directory under `test` that `./le test <suite>` runs, and the guest runs each suite's `needs-linux` tests | Extends: one new suite, `container`, registered in `le` and in the guest's suite list. The sibling children put their tests in it |
| Appliance boot proof | `internal/le/test/qemu/mplsboot.go` `MPLSBoot`, `buildSeededImage` | Builds an appliance image with a seed config, boots it and asks the Ze CLI | Extends: one new proof that reuses `buildSeededImage` |
| Register of external programs | `ai/allowed-system-commands.md` | Lists every program outside Ze that shipped code may run | Extends: one row for the container entrypoint, citing the owner's ruling |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: the `container` root, delivered to the plugin as its declared root, at boot (configure), on reload (verify, apply, rollback) and offline (the in-process verifier).
- Commands: `request container restart name`, and the read function the operator-surface child's listings call.
- The store's notices: a reference is present, restart on a new generation, stop then start.
- The daemon stop: the stop drain call.
- The kernel: a container's exit, its cgroup events.

### Transformation Path
1. The config callbacks parse the root into one validated description per container. One function holds every config rule and three callers use it.
2. Apply compares the descriptions with the running set and records what must start, stop or restart. It returns before any container starts.
3. One supervisor worker per container owns its lifecycle. To start, it asks the store to resolve the container. "Absent" is the `waiting-image` state.
4. The worker merges the image's configuration with the config, and mounts the root: an overlay of the image folder below the writable layer, at a mount point under the plugin's state directory in the run-time tree.
5. The worker fills the library's container description: namespaces, default mounts, masked and read-only paths, devices, capabilities, no-new-privileges, hostname, limits, the volume folders, the memory-backed mounts, the loopback device, the filter profile, and what the participants contributed.
6. The library creates the container. The patched parent clones `ze` into the namespaces and the cgroup. The early role dispatch sees the library's key and runs the library's init, which brings the loopback device up, prepares the root and blocks before the entrypoint.
7. The worker makes the created call to each participant, with the init process id and the network namespace path.
8. The worker releases the container. Init loads the filter through the registered function and executes the entrypoint.
9. Two readers copy the container's standard output and standard error into its bounded output buffer, each line with a sequence number, a time and its stream.
10. The worker waits. On exit it records the status, reads the cgroup's events to name an out-of-memory kill, removes the cgroup and the mount, makes the ended call, releases the store's resolution, stores the record, and applies the restart policy with its delay.
11. On a stop it sends the stop signal, waits the stop timeout, kills the cgroup, and cleans up the same way.
12. At plugin start, before step 3, the plugin removes every container the previous generation left: it kills and removes each cgroup under its parent group, unmounts each root under its state directory, and makes the ended call for each.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to plugin | Config delivery, registered commands, the drain call, the doctor callback, the state calls, the issue calls. The same on both transports | No |
| Always-on code to the gated plugin | A registration in the early-role leaf, filled from the plugin's `init()` | No |
| Library to Ze's filter | A registration in the library's patched stub file | No |
| Daemon to container init | The library's startup protocol over inherited descriptors, as changed by the diff | No |
| This child to its siblings | The seams of the Task section, inside the plugin tree | No |
| Ze to kernel | Clone, cgroup files, mounts, the filter, signals | No |

### Changed-Wire Census

**Search evidence:** searched `pkg/zefs/keys.go` and `feature-gates.txt` for `meta/container` and `ze_container`: no hit. Searched every non-vendor Go file under `internal`, `cmd` and `pkg` for `_LIBCONTAINER` and `opencontainers`: no hit, so no reader or writer of the library's protocol exists in the tree. Searched the same files for a configuration of the global `logrus` logger (`SetOutput`, `AddHook`, `StandardLogger`): no hit; the one `logrus` user found, in the VPP component, builds a private logger. The positive control is `Ze-Reviewed-Against`, which the same kind of search finds as the first line of the netlink route-probe patch in the vendor patch directory. The upstream functions were read at tag v1.5.3 in a clone of the upstream repository. The census of the config root, the command paths, the answer payloads and the state-following doctor codes is the operator-surface child's.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Library startup protocol | writer | The vendored library's `initProcess.start`, as patched | Read at v1.5.3: it writes a netlink blob, then the init config as JSON, on one socket | Changed by H-2: the blob is no longer written. Both ends are one binary, so no version skew exists |
| Library startup protocol | reader | The vendored library's `startInitialization`, unchanged | Read at v1.5.3: it decodes JSON from the socket and never the blob, which the C stage consumed | Preserved: with the blob gone the JSON is the first thing on the socket |
| Role key `_LIBCONTAINER_INITPIPE` | writer | The vendored library's `newParentProcess` | Read at v1.5.3: set on every init command | Preserved |
| Role key `_LIBCONTAINER_INITPIPE` | reader | The new early-role leaf, called from `cmd/ze/dispatch.go` `dispatchMain`; then the library's `startInitialization` | No reader in the tree today | New reader, ahead of every other dispatch |
| Per-container state key | writer | The container plugin through the SDK state calls | No key of that name in `pkg/zefs/keys.go` | New. Permanent once an image ships |
| Per-container state key | reader | The same plugin at start, and `ze data registered` | No existing reader | New |
| Patch review stamp | writer | The new patch files under `internal/le/vendorpatch/patches/` | The positive control above | Preserved format, new files |
| Patch review stamp | reader | The stamp check of the vendor patch package, `./le setup install`, `./le go patch-stamps check` and the Dependabot patch workflow, as the patching page names them | `docs/contributing/patching-the-standard-library.md` "The review stamp" | Preserved: the set of patches is the files present, so no reader is edited |
| Stage 1 declaration | writer | The container plugin's registration: drain budget, doctor checks, failure policy | Existing fields | Preserved format, one more declarer |
| Stage 1 declaration | reader | The plugin server: `internal/component/plugin/server/drain.go` `startDrainCaller` reads the budget | Read in that file | Preserved |

### Integration Points
- `cmd/ze/dispatch.go` `dispatchMain` - gains one call to the early-role leaf, before `seedinit.Dispatch`.
- `feature-gates.txt` - the plugin's lines under `ze_container`; `./le repo update` writes the derived files.
- The kernel capability registry - the enrolments.
- `pkg/zefs/keys.go` - one key template; `RegisterPluginKeys` grants it.
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - one option added, three pinned.
- The functional suite table of `le` and the guest's suite list - the `container` suite.
- The QEMU test area of `le` - one appliance boot proof and one integration package.
- `ai/allowed-system-commands.md` - one row.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-4 (umbrella) | A root filesystem with no write support prevents a container from overwriting the `ze` binary | Inference in the umbrella. The design does not rely on it: init always runs from the library's sealed copy | Nothing changes: the seal is the defence on every platform | `TestContainerInitRunsFromASealedBinary`: at the created call, the init process's executable link does not name the `ze` file on disk, on a writable filesystem (the guest) and on squashfs (`./le test qemu container-boot`) | unvalidated |
| A-5 (umbrella) | `ze`, as root in the root cgroup under gokrazy's init, can create child cgroups and enable their controllers | The root cgroup is exempt from the rule that a cgroup with processes cannot enable controllers for children. Not read in a kernel tree | The runtime cannot limit a container on the appliance | The `container-start` functional test in the own-system guest, where PID 1 is gokrazy's init, and the cgroup doctor row read by `./le test qemu container-boot` on an appliance image | unvalidated |
| A-7 | The standard library's clone accepts the namespace flags together with a cgroup descriptor, and the cloned child can execute the sealed binary through its descriptor path | The standard library builds one `clone3` call from both. The descriptor path resolves through the host's process filesystem, which the child still sees at that point. Not run | H-1 needs another way to place init in its cgroup before the namespace is made | The `container-start` functional test | unvalidated |
| A-8 | The init role's peak memory, charged to the container's cgroup, stays below 32 MiB | Not measured. Every `init()` of the `ze` binary runs before the role is selected | The floor of `memory`, 64 MiB, is raised to twice the measured peak, and the owner is told | `TestContainerInitFootprintIsBelowTheMemoryFloor`, which reads the cgroup's peak memory before the release | unvalidated |
| A-9 | The device filter package of the cgroups module compiles and loads against the eBPF module version Ze already vendors | The cgroups module was written against an older version of that module. Not compiled | The device rules need a second patch, or the dependency commit pins what both accept | The plugin's package test compiling, then the `container-default-devices` functional test | unvalidated |
| A-10 | The default filter profile can be derived from the Moby project's published default profile under its license | Named in the research. The file was not fetched | The profile is written from the library's test profiles and the kernel's system-call list | Reading the file and its license before phase 4, and recording both in the notice file | unvalidated |
| A-11 | Pointing the global `logrus` logger at Ze's logger disturbs no other user | The census found no other user of the global logger | The library's log lines are bridged another way | The census search, repeated at implementation | unvalidated |
| A-12 | The user-account module that arrives with the runtime resolves a name from files opened under a root descriptor | That module is in the upstream runtime's requirements at v1.5.3. Its functions were not read | The resolution is a small parser of the two account files in the plugin | Reading the module after the dependency commit, and `TestImageUserNameIsResolvedInsideTheRoot` | unvalidated |
| A-13 | A new functional suite needs only a directory, one row in the suite table of `le` and one in the guest's suite list | The le tooling door and the QEMU page describe suites that way. The two tables were not read | The registration is larger, and its files are named at the wiring phase | Reading both tables at the wiring phase | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-9 (umbrella) | An upstream release changes a patched function, and the diff applies but is now wrong | A container fails to start after a module bump | The review stamp refuses the bump until the functions are read again (`./le go patch-stamps check`, `./le setup install`). `TestRuncDirectInitPatchApplied` and `TestPatchedVendorModulesMatchUpstream` prove the patch is applied whole. The gate on a bump is the guest tests `container-start`, `container-default-confinement` and `container-stop-on-config-removal` |
| R-10 (umbrella) | The library links code Ze never runs | New symbols in the binary | H-4 removes the systemd driver and the cgroup v1 manager, H-5 the checkpoint client. `TestBuildTag_Container_Present` asserts that the binary holds no symbol of those three packages. Their modules still arrive in `go.mod` and `vendor/`, uncompiled or as types |
| R-11 (umbrella) | Each start copies the whole `ze` binary into memory, when the overlay sealing is unavailable | Memory rises by the binary's size at each start | The config-phase check `container-init-seal` runs the library's sealing once and reports `doctor-container-seal-copies-binary` when only the copy works. `./le test qemu container-boot` requires that row to be absent on an appliance image, where `ze` is on squashfs |
| R-12 (umbrella) | The library marks every descriptor of the daemon close-on-exec before it starts init | A process Ze starts later loses a descriptor it was meant to inherit | The `container-then-external-plugin` functional test starts a forked plugin after a container is running and requires its connect-back and its stderr relay to work |
| R-13 (umbrella) | A parent death signal kills init at once under the diff | Every container exits immediately | H-1 refuses a non-zero value, proven by `TestPatchedLibraryRefusesAParentDeathSignal`. `TestDescriptionNeverSetsAParentDeathSignal` proves the plugin never sets one |
| R-15 | The cgroup namespace is rooted at the daemon's cgroup, so a container reads the host's cgroup tree | The container's cgroup mount lists other containers | H-1 clones init into its cgroup. The `container-default-confinement` test requires the container's own cgroup path to read as the root |
| R-16 | A daemon that died leaves containers running with no supervisor | Container processes exist while no daemon runs | The sweep at plugin start, proven by the `container-leftover-swept` test. Accepted by the owner |
| R-17 | The init role is charged to the container and a small memory limit kills it before the entrypoint | A start fails with the library's out-of-memory error | The floor of `memory` (A-8), and `doctor-container-out-of-memory` names the limit |
| R-18 | The diff is 60 to 90 changed lines in five files | Review of the patch | Each hunk is one concern in one patch file, so a bump shows which concern broke |
| R-19 | The operator-surface child's tables change after this child's wiring | A node or a path here is absent from those tables | Its design is settled before this child's wiring phase, by the owner's rule. The wiring phase re-reads its tables first |
| R-20 | A container's root is uid 0 on the host, because no user namespace is used | A container with an added capability reaches more than intended | Capabilities, the filter, masked paths, no-new-privileges and the device filter are the confinement. A capability outside the default set, a shared host namespace and a device are each an explicit config grant |
| R-21 | `cpu` has a default, so every container needs the kernel option for a hard CPU limit | A container config is refused on a kernel without it | The option is added and pinned in the two kernel files, and its enrolment is in use whenever a container is configured |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A container escapes its confinement or starves the routing daemon. With no container configured, the only always-on change is one early dispatch call, which a wrong edit would turn into a `ze` that does not start |
| How is it reverted? | Remove the manifest lines and run `./le repo update`: the plugin leaves every build. The dependency and the patch are reverted by their own commits. The kernel option stays harmless |
| Who else touches this path? | The image-store, network, operator-surface and shell children of the umbrella add packages, nodes, commands and patch files to what this child creates. `cmd/ze/dispatch.go` is shared with every personality of the binary |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| The library's environment key on a `ze` start | → | The early-role leaf, then the library's init | `TestEarlyRoleRunsBeforeCommandDispatch` |
| A `container` root in a committed config | → | The plugin's config callbacks and one supervisor worker | `container-start` (functional) |
| The library's filter call in init | → | Ze's filter builder, through the stub's registration | `TestSeccompSeamIsFilledByThePlugin` |
| The daemon stop | → | The plugin's drain handler | `container-stop-on-daemon-stop` (functional) |
| `request container restart name` | → | The supervisor worker | `container-restart-command` (functional) |
| `show doctor` | → | The plugin's doctor callback | `TestContainerDoctorChecksAnswerThroughTheCallback` |
| Validate, start and reload on a kernel without an option | → | The kernel enrolments | `kernel-capability-container` (functional) |
| A registered participant | → | The created call and the ended call | `TestCreatedCallRunsBeforeTheRelease` |
| The store answering "absent", then "present" | → | The `waiting-image` state, then the start | `TestStoreAbsentThenPresentStartsTheContainer` |
| A build with and without `ze_container` | → | The gate | `TestBuildTag_Container_Present`, `TestBuildTag_Container_Absent` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A config names an enabled container whose image the store resolves, and is committed | The entrypoint runs. This child's facts show `running` with the digest the store gave. Its process is alone in its pid, mount, network, ipc, uts and cgroup namespaces and in a cgroup of its own, and its loopback device is up | A container whose image is absent shows `waiting-image`; one with `enabled false` shows `disabled`, because the operator asked for that |
| AC-2 | A participant is registered and a container starts | The participant gets the created call once, after create and before the entrypoint runs, with the container name, the init process id and a network namespace path that names a namespace other than the daemon's. It gets the ended call once after the container's processes are gone. A container that shares the host's network namespace is the exception: the created call is not made for it, and it still gets the ended call | A participant that returns an error makes the start fail with its name and reason in the state, and the container is destroyed, because a container without its network must not run |
| AC-3 | A container allocates memory past `memory` | The kernel kills inside the container. The state records an out-of-memory exit, doctor reports `doctor-container-out-of-memory` with the `set` line that raises the limit, and the restart policy applies | The daemon and every other container keep running, which is the purpose of the limit |
| AC-4 | A container forks past `process-limit` | The fork fails inside the container with the kernel's resource error, the container keeps running, and doctor reports `doctor-container-process-limit` | Processes below the limit keep running, because only the excess is refused |
| AC-5 | A container with `cpu 50` runs a busy loop on every processor | The cgroup's CPU statistics show throttled periods above zero, and its CPU time over ten seconds is at most five seconds plus one period | A container with a larger `cpu` uses more, up to its own value, because the limit is per container |
| AC-6 | A container is granted `host-namespace pid`, a `device` and a `sysctl` | From inside: the process list holds the host's processes, the device node opens, and the kernel setting reads the configured value. The same container without the grants sees only its own processes, fails to open the device, and reads the kernel's default | Every confinement the config did not lift stays, because each grant names one thing |
| AC-7 | A container runs with the default confinement | From inside: the process list holds only its own processes, its own cgroup reads as the root, the capability set equals the fourteen defaults, a system call outside the default profile fails with a permission error, and a system-call number above the profile's largest fails with "not implemented" | A capability the config's `capability` adds is present, because the operator granted it |
| AC-8 | A container runs with the default description | From inside: each masked path reads as empty, a write to each read-only path is refused, the device directory holds exactly the default devices, no-new-privileges reads as set, and the hostname is the instance name or the configured `hostname` | A path the image itself provides elsewhere is untouched, because only the listed paths are masked |
| AC-9 | A container writes a file in its root, in a path its image declares as a volume, in a configured `volume` and in a `tmpfs`, then is restarted | The first two files are in the writable layer and the image folder's content is unchanged. After the restart the first three files are there, and the `tmpfs` is empty | A `volume` with `read-only true` refuses the write, as configured |
| AC-10 | A container's entrypoint exits with a failure, repeatedly | Under `on-failure` and `always` it is restarted with a delay that doubles from 1 to 30 seconds, and returns to 1 second after a run of 60 seconds. The state shows `restarting` then `running`, with the restart count and the last exit status | Under `never` the container shows `stopped` with its exit status, as configured. A clean exit under `on-failure` also shows `stopped` |
| AC-11 | A config change removes a container, or the daemon stops | The container gets its stop signal. A container still alive after `stop-timeout`, at most 30 seconds, is killed through its cgroup. Afterwards the cgroup, the root mount and the state directory are gone and its processes have exited | The image folder, the writable layer and the volumes remain on disk, so the next start finds them |
| AC-12 | The daemon is killed while a container runs, then starts again | Before any container starts, the leftover processes are killed and the leftover cgroup and mount removed, with a warning that names the container. Each configured container then runs exactly once, as a new process | The writable layer keeps what the container wrote before the kill, because it is on disk |
| AC-13 | A config holds a container on a kernel that lacks an option it needs, the hard CPU limit's included | Validate, start and reload refuse and name the option. Doctor reports the same | A config with no container is accepted on that kernel, because nothing needs the option |
| AC-14 | `ze` is built without `ze_container` | A config with the `container` root is refused as unknown with the build tag in the message, the plugin is not registered, and the binary holds no symbol of the plugin or of the upstream runtime | Every other feature behaves as in a build with the gate |
| AC-15 | A daemon that runs as an unprivileged account, or a host whose cgroup v2 hierarchy is not at `/sys/fs/cgroup`, loads a config with a container | The daemon refuses the config at boot, reload and commit, with the reason, and doctor reports the cause with its fix | A config with no container loads on that host, because nothing needs the privilege |
| AC-16 | A container writes lines to standard output and standard error | The output read answers them in order, each with a rising sequence number, a time and its stream, from a buffer that holds the newest 1 MiB per container, and answers only the rows after a given sequence number when asked. With `log system` each line is also in Ze's log | Rows older than the buffer are dropped, the output of a removed container or a previous daemon run is gone, and `log none` keeps nothing, because the buffer is in memory and the operator chose |
| AC-17 | The plugin runs as a separate process | AC-1, AC-10, AC-11 and AC-12 hold with the same observations | Containers are children of the plugin process and not of the daemon, which no operator surface shows |
| AC-18 | A config reload leaves one container unchanged, changes another and adds a third | The unchanged container keeps its process, the changed one is stopped and started with the new description, and the added one starts | A change to a container's `enabled` alone stops or starts it without touching the others |
| AC-19 | A container runs, and a forked plugin is started afterwards | The container's entrypoint holds only its three standard descriptors. The forked plugin connects back and its error output reaches the engine log | Descriptors the config passes on purpose do not exist in this child, so none is expected |
| AC-20 | The daemon restarts cleanly | Every enabled container is stopped in the drain and started again as a new process. For a `stopped` container, the facts show the exit status and time recorded before the restart | A running container's start time is the new one, because it was started again and never re-adopted |
| AC-21 | An image names its user by name, and another names a user its account files do not hold | The first entrypoint runs with the numeric user and group that name has in the image's own account files. The second container shows a failed start whose reason names the user | A config `user` replaces the image's user and skips the lookup, because the operator gave a number |
| AC-22 | `request container restart name` for a running, a `stopped`, a `failed`, a `disabled` and a `waiting-image` container | The first is stopped and started as a new process, the second and third start, and the last two are refused with the fix the operator-surface child's table names | A bare `request container restart` is refused and lists the complete form, by that child's rule |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Commits a config with one container | config delivery -> description -> store resolve -> root overlay -> library create -> created call -> release | `container-start` |
| 2 | Removes the container from the config | config delivery -> supervisor stop -> signal, timeout, cgroup kill -> cleanup -> ended call | `container-stop-on-config-removal` |
| 3 | Restarts a container that keeps failing after fixing its cause | restart command -> supervisor -> start | `container-restart-command` |
| 4 | Stops the daemon | stop drain -> drain handler -> every supervisor stops | `container-stop-on-daemon-stop` |
| 5 | Boots an appliance whose config names a container | boot configure -> kernel enrolment -> cgroup and seal checks -> `waiting-image` -> `show doctor` | `appliance-container-qemu` |

## 🧪 TDD Test Plan

Test files are listed with their paths under Files to Create. The File column names the package.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEarlyRoleRunsBeforeCommandDispatch` | `cmd/ze`, early role test | With a registered key set, the role runs and no command dispatches | |
| `TestEarlyRoleKeyWithNoOwnerRefuses` | early-role leaf test | A key no role registered answers "not handled" | |
| `TestDescriptionHasTheDefaultConfinement` | plugin, describe test | The filled description holds the default namespaces, mounts, masked paths, read-only paths, devices, the fourteen capabilities and the loopback device | |
| `TestDescriptionNeverSetsAParentDeathSignal` | plugin, describe test | No config produces a description with a parent death signal | |
| `TestDescriptionNeverFillsTheLibraryNetworkDevices` | plugin, describe test | The library's network-device list stays empty | |
| `TestDescriptionSharesOnlyTheNamedHostNamespaces` | plugin, describe test | Each `host-namespace` value removes exactly its namespace | |
| `TestImageConfigurationMergesWithTheConfig` | plugin, describe test | Entrypoint, argument, environment, working directory and stop signal, each with the config silent and set | |
| `TestImageUserNameIsResolvedInsideTheRoot` | plugin, describe test | A name, a number, an unknown name, and an account file that is a link out of the root | |
| `TestDeclaredVolumeWithoutConfigIsNotMounted` | plugin, describe test | An image-declared path absent from `volume` adds no mount | |
| `TestConfigRulesRefuseALimitBelowItsFloor` | plugin, config test | Each limit below its floor is refused with the offending value | |
| `TestConfigRulesOfTheVerifyTable` | plugin, config test | V-2, V-3, V-7, V-8 and V-10, each with its message. V-11 is proven by `container-not-privileged-refused` | |
| `TestDescriptionDropsNamedCapabilities` | plugin, describe test | `capability-drop` removes each named capability, `all` removes the default set, and `capability` adds back what is named | |
| `TestStopSignalComesFromTheConfigThenTheImage` | plugin, describe test | The config's `stop-signal`, else the image's, else the termination signal | |
| `TestOneRuleSetServesValidateVerifyAndConfigure` | plugin, config test | The in-process verifier, the live verify and configure give one verdict for one config | |
| `TestRemovedRootStopsEveryContainer` | plugin, config test | An empty body yields "stop all", never "no change" | |
| `TestApplyReturnsBeforeContainersStart` | plugin, supervise test | Apply records the desired set and returns while a start is blocked | |
| `TestRestartDelayDoublesAndResets` | plugin, supervise test | The delay sequence and its return to one second | |
| `TestRestartPolicyDecidesAfterEachExitKind` | plugin, supervise test | Each policy against a clean exit, a failure and a config stop | |
| `TestFiveFailedStartsGiveTheFailedState` | plugin, supervise test | The fifth failed start in a row stops the attempts, and a success resets the count | |
| `TestStoreAbsentThenPresentStartsTheContainer` | plugin, supervise test | "Absent" gives `waiting-image` and no start; the present notice starts it and answers its name | |
| `TestStoreResolutionIsHeldUntilCleanupEnds` | plugin, supervise test | Hold at start, release after cleanup, on every path out | |
| `TestCreatedCallRunsBeforeTheRelease` | plugin, participant test | Order of create, created call, release and ended call, through a fake library | |
| `TestOutputBufferKeepsTheNewestBytes` | plugin, output test | The buffer bound, the sequence numbers, the stream mark and the read after a sequence number | |
| `TestStateRecordRoundTrips` | plugin, state test | The stored record decodes to what was stored | |
| `TestContainerDoctorChecksAnswerThroughTheCallback` | plugin, doctor test | Each runtime check is declared at Stage 1 and answered by the callback, with its fix | |
| `TestContainerKernelCapabilitiesAreEnrolled` | plugin, kernel enrolment test | Each option of the kernel list is enrolled and in use whenever a container is configured | |
| `TestSeccompSeamIsFilledByThePlugin` | filter package, seam test | The library's `InitSeccomp` reaches Ze's builder | |
| `TestFilterReturnsEachActionForItsCall` | filter package, filter test | The assembled program, run in the BPF package's virtual machine, returns the profile's action for a listed call | |
| `TestFilterReturnsTheDefaultForAnUnlistedCall` | filter package, filter test | The default action and its error number | |
| `TestFilterReturnsNotImplementedAboveTheProfile` | filter package, filter test | A number above the profile's largest, the x32 range included, returns "not implemented" | |
| `TestFilterKillsAnotherArchitecture` | filter package, filter test | A call under another audit architecture gets the kill action | |
| `TestFilterComparesArguments` | filter package, filter test | Each argument operator, on both halves of a 64-bit value | |
| `TestFilterSkipsUnknownNamesAndDefaultRules` | filter package, filter test | An unknown name and a rule equal to the default add no instruction | |
| `TestSyscallTablesMatchVendoredNumbers` | filter package, tables test | The amd64 and arm64 tables equal the vendored system-call number files of the `golang.org/x/sys` module | |
| `TestDefaultProfileNamesOnlyKnownCalls` | filter package, profile test | Every name in the default profile is in at least one table | |
| `TestRuncDirectInitPatchApplied` | vendor patch package, named guard | Every file of the series is present and applied | |
| `TestBuildTag_Container_Present` | `cmd/ze/hub`, present test | The plugin is registered, the root parses, and the binary holds no systemd-driver, cgroup v1 or checkpoint symbol | |
| `TestBuildTag_Container_Absent` | `cmd/ze/hub`, absent test | The plugin is absent, the root is refused naming the tag, and the binary holds no plugin or runtime symbol | |
| `TestPatchedLibraryRefusesAParentDeathSignal` | plugin, library integration test (`integration && linux`) | A direct library start with the value set is refused by H-1 | |
| `TestPatchedLibraryRefusesToEnterAContainer` | same file | A non-init process is refused by H-3 | |
| `TestContainerInitRunsFromASealedBinary` | same file | The init executable link at the created call | |
| `TestContainerInitFootprintIsBelowTheMemoryFloor` | same file | The cgroup's peak memory before the release | |

### Boundary Tests (numeric inputs)
The ranges are the operator-surface child's, except the two this child's design bounds: the floor of `memory` and the top of `stop-timeout`.

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `memory`, mebibytes | 64 to 1048576 | 64 and 1048576 | 63 | 1048577 |
| `shared-memory`, mebibytes | 1 to 1048576, and V-3 | 1 | 0 | a value that breaks V-3 |
| `process-limit` | 1 to 32768 | 32768 | 0 | 32769 |
| `cpu`, percent of one core | 1 to 102400, and V-8 | 100 times the host's processors | 0 | one above that |
| `stop-timeout`, seconds | 1 to 30 | 30 | 0 | 31 |
| `tmpfs` `size`, mebibytes | 1 to 1048576, and V-3 | 1 | 0 | a value that breaks V-3 |
| Restart delay, seconds | 1 to 30, set by the code | 30 | N/A | N/A |
| Failed starts in a row before `failed` | 5, set by the code | 4 keeps trying | N/A | N/A |

### Functional Tests
Every test below except the last is a `.ci` file of the container suite, carries `option=needs-linux` with the capability marker for root that the `.ci` format page declares, and runs in Ze's own-system guest on the fixture image of the test store. The last one is a `.ci` file of the appliance suite and drives a boot proof.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `container-start` | container suite | A committed container runs its entrypoint in its own namespaces and cgroup, with its loopback device up (AC-1) | |
| `container-created-call` | container suite | A test participant sees the init process id and namespace before the entrypoint, and the ended call after (AC-2) | |
| `container-memory-limit` | container suite | An allocation past the limit is killed and reported (AC-3) | |
| `container-process-limit` | container suite | A fork past the limit is refused (AC-4) | |
| `container-cpu-limit` | container suite | A busy loop is throttled (AC-5) | |
| `container-grants` | container suite | A shared host namespace, a device and a kernel setting, with and without the grant (AC-6) | |
| `container-default-confinement` | container suite | Process list, cgroup root, the fourteen capabilities and the two filter answers (AC-7) | |
| `container-default-devices` | container suite | Masked and read-only paths, devices, no-new-privileges, hostname (AC-8) | |
| `container-root-and-volumes` | container suite | Writes land where they must and survive a restart (AC-9) | |
| `container-restart-on-failure` | container suite | A failing entrypoint is restarted with a growing delay (AC-10) | |
| `container-restart-never` | container suite | A failing entrypoint stays stopped under `never` (AC-10) | |
| `container-stop-on-config-removal` | container suite | A removed container is signaled, then gone (AC-11) | |
| `container-stop-timeout-kills` | container suite | An entrypoint that ignores the signal is killed at the timeout (AC-11) | |
| `container-stop-on-daemon-stop` | container suite | A daemon stop leaves no process, cgroup or mount (AC-11) | |
| `container-drain-second-signal-kills` | container suite | A second signal during the drain kills every container at once | |
| `container-leftover-swept` | container suite | A killed daemon's containers are removed at the next start (AC-12) | |
| `kernel-capability-container` | container suite | A forced-absent option refuses validate, start and reload (AC-13) | |
| `container-not-privileged-refused` | container suite | An unprivileged daemon refuses the config and doctor names the fix (AC-15) | |
| `container-output` | container suite | Both streams reach the buffer, and `log system` reaches Ze's log (AC-16) | |
| `container-external-plugin` | container suite | The start, restart and stop scenarios with the plugin forked (AC-17) | |
| `container-plugin-killed-external` | container suite | The forked plugin is killed, its next generation sweeps and restarts (AC-17, AC-12) | |
| `container-reload-keeps-unchanged` | container suite | A reload touches only the changed containers (AC-18) | |
| `container-then-external-plugin` | container suite | Descriptors in the container and in a plugin forked later (AC-19) | |
| `container-state-survives-restart` | container suite | A daemon restart starts each container again, and a stopped one keeps its last exit status (AC-20) | |
| `container-image-user` | container suite | An image user given by name, and an unknown one (AC-21) | |
| `container-restart-command` | container suite | The restart command against each state (AC-22) | |
| `appliance-container-qemu` | appliance suite | An appliance image with a container in its seed boots, and doctor shows the cgroup row healthy and no seal warning (A-5, R-11) | |

Lifecycle scenarios of `ai/rules/testing.md`:

| Scenario | Reached? | Test, or the reason it is not reached |
|----------|----------|---------------------------------------|
| A plugin's process ends after it declared | Yes, forked only | `container-plugin-killed-external`. An internal plugin has no process of its own to end |
| A plugin fails a startup stage after it declared | Yes | `container-not-privileged-refused` |
| A restarting plugin passes its restart bound | No | The bound is the plugin server's and this child edits none of it |
| The `respawn` leaf disagrees with the declaration | No | The leaf belongs to an operator's external plugin block, which this child does not add |
| A plugin exits during the stop drain or teardown | Yes | `container-drain-second-signal-kills` |
| A signal arrives during the stop | Yes, in the drain | `container-drain-second-signal-kills`. The fence and the teardown hold no container work |
| A reload or commit arrives after shutdown began | No | The refusal is the hub's, before any plugin is asked |
| A plugin crashes during verify or apply, misses its deadline, or reports broken | Yes, the deadline | `TestApplyReturnsBeforeContainersStart`, a seam test: a `.ci` cannot hold a start blocked for the length of the deadline |
| A reload's context ends during a startup handshake | No | The handshake is the plugin server's |
| A plugin dispatches another plugin's command at startup | No | This child sends no command to another plugin. The network child does, and owns that test |

### Interop Tests (Scope: protocol)
Not applicable: Scope is plugin and no wire protocol changes.

## Files to Modify
- `go.mod`, `go.sum`, `vendor/` - the runtime module and what it requires, in the dependency commit and nothing else
- `cmd/ze/dispatch.go` - one early-role call at the top of `dispatchMain`
- `feature-gates.txt` - the gate `ze_container`, declared here, and the lines of the packages this child creates: the plugin, its `seccomp` package and its `yang` package. Each sibling adds the line of each package it creates, under the same tag
- `pkg/zefs/keys.go` - the per-container state key template
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - add `CONFIG_CFS_BANDWIDTH`; pin `CONFIG_FAIR_GROUP_SCHED`, `CONFIG_TMPFS` and `CONFIG_MEMFD_CREATE`, which are on today only by default
- `ai/allowed-system-commands.md` - the entrypoint row, citing the ruling of 2026-11-16
- The functional suite table of `le` and the guest's suite list - the `container` suite
- The integration package list and the action table of the QEMU test area of `le` - the plugin's package and the `container-boot` action
- The two hand-kept plugin name lists `ai/doors/hub-engine.md` names (`TestAvailablePlugins`, `TestAllPluginsRegistered`) and their snapshots - the new plugin name
- `docs/contributing/patching-the-standard-library.md` - the new named guard test
- `docs/architecture/doctor-and-health-checks.md` - the runtime checks table
- `docs/architecture/system-architecture.md` - the early role in the dispatch order
- `docs/architecture/zefs-format.md` - the container key in the key registry
- `docs/architecture/testing/qemu-integration.md` - the `container` suite, the `container-boot` proof and the fixture image
- The functional tests page, the appliance guide's kernel requirement rows and the kernel profiles page - the new suite, the new and the pinned options
- The plugin guide, the keyword index and the notice file - the plugin, its keywords, the new modules and the profile's source
- Derived by `./le repo update`, never by hand: the gated composition root, the gated-plugin table, the lint tag list, the appliance tag list, the quickstart install line, the CodeQL build combinations

## Files to Create
- `internal/le/vendorpatch/patches/runc-01-direct-init.patch`, `runc-02-start-without-bootstrap.patch`, `runc-03-refuse-setns.patch`, `runc-04-cgroup-v2-manager.patch`, `runc-05-criu-constraint.patch`, `runc-06-seccomp-seam.patch` in the same directory - one file per hunk H-1 to H-6
- `internal/le/vendorpatch/runc_direct_init_patch_test.go` - the named guard
- `internal/core/earlyrole/earlyrole.go`, `internal/core/earlyrole/earlyrole_test.go` - the always-on leaf: register a role by environment key, dispatch before startup
- `cmd/ze/early_role_test.go` - the dispatch order test
- `cmd/ze/hub/build_tag_container_present_test.go`, `cmd/ze/hub/build_tag_container_absent_test.go` - the gate's test pair, which the siblings extend
- `internal/plugins/container/register.go` - the registration, its config root and verifier entry point, the early role, the state key grant, the diagnostic codes
- `internal/plugins/container/container.go` - the engine entry, the logger, the config callbacks, the drain handler, the restart command
- `internal/plugins/container/config.go`, `internal/plugins/container/config_test.go` - the parser and the one rule set, which the siblings add rules to
- `internal/plugins/container/describe_linux.go`, `internal/plugins/container/describe_test.go` - the library description and the merge with the image's configuration
- `internal/plugins/container/rootfs_linux.go` - the overlay mount and unmount
- `internal/plugins/container/supervise.go`, `internal/plugins/container/supervise_test.go` - the worker, the states, the restart delay, the container object kept for the shell child
- `internal/plugins/container/sweep_linux.go` - the leftover removal
- `internal/plugins/container/participant.go`, `internal/plugins/container/participant_test.go` - the created call, the ended call and the contributions
- `internal/plugins/container/storeseam.go`, `internal/plugins/container/storeseam_zetest.go` - the seam to the store, and the test store with its fixture image
- `internal/plugins/container/output.go`, `internal/plugins/container/output_test.go` - the bounded buffer and its readers
- `internal/plugins/container/state.go`, `internal/plugins/container/state_test.go` - the stored record and the read function for the state view
- `internal/plugins/container/doctor.go`, `internal/plugins/container/doctor_test.go` - the checks
- `internal/plugins/container/kernelcap_linux.go`, `internal/plugins/container/register_kernelcap_linux.go`, `internal/plugins/container/kernelcap_linux_test.go` - the enrolments
- `internal/plugins/container/logbridge.go` - the library's log lines into Ze's logger
- `internal/plugins/container/library_integration_linux_test.go` - the library-level integration tests
- `internal/plugins/container/yang/ze-container-conf.yang`, `internal/plugins/container/yang/ze-container-cmd.yang` - the nodes and the one command this child realizes, with the operator-surface child's words
- `internal/plugins/container/seccomp/filter.go`, `internal/plugins/container/seccomp/filter_test.go`, `internal/plugins/container/seccomp/seam_test.go` - the filter builder
- `internal/plugins/container/seccomp/tables_amd64.go`, `internal/plugins/container/seccomp/tables_arm64.go`, `internal/plugins/container/seccomp/tables_test.go` - the system-call tables
- `internal/plugins/container/seccomp/profile.go`, `internal/plugins/container/seccomp/profile_test.go`, `internal/plugins/container/seccomp/load_linux.go` - the default profile and the load
- `internal/test/containerprobe/main.go` - the static fixture program the fixture image holds
- `internal/le/test/qemu/containerboot.go` - the appliance boot proof
- `docs/architecture/container/runtime.md` - the design page the plugin's files name
- `features/container.md` - the feature declaration, which the siblings extend
- `internal/component/plugin/all/all_ze_container.go` - generated by `./le repo update`
- `test/container/container-start.ci`, `test/container/container-created-call.ci`, `test/container/container-memory-limit.ci`, `test/container/container-process-limit.ci`, `test/container/container-cpu-limit.ci`, `test/container/container-grants.ci`
- `test/container/container-default-confinement.ci`, `test/container/container-default-devices.ci`, `test/container/container-root-and-volumes.ci`, `test/container/container-restart-on-failure.ci`, `test/container/container-restart-never.ci`
- `test/container/container-stop-on-config-removal.ci`, `test/container/container-stop-timeout-kills.ci`, `test/container/container-stop-on-daemon-stop.ci`, `test/container/container-drain-second-signal-kills.ci`, `test/container/container-leftover-swept.ci`
- `test/container/kernel-capability-container.ci`, `test/container/container-not-privileged-refused.ci`, `test/container/container-output.ci`, `test/container/container-external-plugin.ci`, `test/container/container-plugin-killed-external.ci`
- `test/container/container-reload-keeps-unchanged.ci`, `test/container/container-then-external-plugin.ci`, `test/container/container-state-survives-restart.ci`, `test/container/container-image-user.ci`, `test/container/container-restart-command.ci`
- `test/appliance/appliance-container-qemu.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The plugin's `yang` directory: the nodes and the command this child realizes, with the words of the operator-surface child's tables S-1 and C-1 |
| YANG validation constraints | Yes | The ranges of that child's tables, with the floor of `memory` and the top of `stop-timeout` this child's design sets |
| YANG custom validators | No | Every rule that needs code is in the plugin's one rule set, which the in-process verifier runs |
| CLI commands/flags | Yes | `request container restart name`, registered by the plugin |
| CLI grammar (keyword before value) | Yes | Fixed by the operator-surface child; `./le cli grammar` judges it |
| Editor autocomplete | Yes | Automatic for the enumerations. A container name completes from the configured instances |
| Functional test for new RPC/API | Yes | The `container-restart-command` and `container-start` functional tests |
| Pipe completeness | Yes | The restart command answers a document through the shared renderer |
| Env var registration | Yes | The `zetest`-only private variable of the test store, registered beside its reader |
| Doctor check for runtime dependencies | Yes | The plugin's doctor file: privilege, the cgroup hierarchy, the seal route, each kernel option |
| Doctor code per problem class | Yes | The problem-class table of the Task section |
| Prometheus counters/metrics | Yes | `ze_container_instance_oom_kills_total`, labelled by `name`. The state metrics are the operator-surface child's |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/container.md` (new): the runtime's level and evidence |
| 2 | Config syntax changed? | Yes | The configuration guide: the `container` root, owned jointly with the operator-surface child |
| 3 | CLI command added/changed? | Yes | The command reference: `request container restart name` |
| 4 | API/RPC added/changed? | Yes | The API commands page: the same command |
| 5 | Plugin added/changed? | Yes | The plugin guide: the `container` plugin |
| 6 | Has a user guide page? | No | The guide page is the operator-surface child's |
| 7 | Wire format changed? | N-A | No wire format |
| 8 | Plugin SDK/protocol changed? | No | The plugin uses existing calls only |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC |
| 10 | Test infrastructure changed? | Yes | The functional tests page and `docs/architecture/testing/qemu-integration.md`: the `container` suite, the `container-boot` proof and the fixture image |
| 11 | Affects daemon comparison? | No | The comparison page is updated when the whole feature ships |
| 12 | Internal architecture changed? | Yes | `docs/architecture/container/runtime.md` (new); the core design page: the early-role leaf |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The plugin metrics page: the one counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The plugin overview, the plugin feature page and the status guide |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/system-architecture.md`, declared by `cmd/ze/dispatch.go`: add the early role to the dispatch order. `docs/architecture/zefs-format.md`, declared by `pkg/zefs/keys.go`: add the container key to the key registry. The rest is derived at implementation by `./le spec citation anchors` on this spec |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No page shows a container example today |

## Implementation Steps

0. **Dependency commit (before any use)** -- the main thread runs `./le setup install` with the importing file present in the working tree and commits only `go.mod`, `go.sum` and `vendor/`
   - Verify: `go.mod` names the runtime module; no file outside those three is in the commit
1. **Phase: Wiring (MANDATORY FIRST)** -- after the operator-surface child's design is settled: the patch series, the gate, the early-role leaf, the plugin skeleton with its config root and verifier entry point, the schema nodes, the seams, the `container` suite
   - Tests: every test of the Wiring Test table
   - Files: the patches and their guard, `feature-gates.txt`, the early-role leaf, `cmd/ze/dispatch.go`, the plugin's registration, engine entry, participant and store seam files, its `yang` directory, the two build-tag tests, the suite registration
   - Verify: each entry point exists and its test fails because the feature is a stub
2. **Phase: A container that starts, in the guest** -- description, root overlay, cgroup, create, release, wait, and the register row
   - Tests: `container-start`, `TestDescriptionHasTheDefaultConfinement`, `TestContainerInitRunsFromASealedBinary`, `TestContainerInitFootprintIsBelowTheMemoryFloor`
   - Files: the description, root and supervise files, the test store, the fixture program, `ai/allowed-system-commands.md`
   - Verify: the test fails, the code is written, the test passes in the own-system guest. This phase retires A-5, A-7 and A-8, or stops with what it found
3. **Phase: Stop, cleanup and the leftover sweep**
   - Tests: the three stop tests, `container-drain-second-signal-kills`, `container-leftover-swept`
   - Files: the supervise, sweep and engine entry files
4. **Phase: The system-call filter**
   - Tests: the tests of the filter package, then `container-default-confinement`
   - Files: the filter package
5. **Phase: Limits and the kernel**
   - Tests: the three limit tests, `kernel-capability-container`, `TestContainerKernelCapabilitiesAreEnrolled`, `TestConfigRulesOfTheVerifyTable`
   - Files: the description and enrolment files, the two kernel files
6. **Phase: Mounts, devices, grants and the image's configuration**
   - Tests: `container-default-devices`, `container-root-and-volumes`, `container-grants`, `container-image-user`
7. **Phase: Restart policy, the restart command, state and output**
   - Tests: the two restart tests, `container-restart-command`, `container-output`, `container-state-survives-restart`, `container-reload-keeps-unchanged`
   - Files: the supervise, state and output files, `pkg/zefs/keys.go`
8. **Phase: Doctor, privilege and the forked plugin**
   - Tests: `container-not-privileged-refused`, `container-external-plugin`, `container-plugin-killed-external`, `container-then-external-plugin`, `TestContainerDoctorChecksAnswerThroughTheCallback`
   - Files: the doctor file
9. **Phase: The appliance boot proof and the pages**
   - Tests: `appliance-container-qemu`
   - Files: the boot proof, the pages of the Documentation checklist

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Init is in its cgroup before its cgroup namespace exists. Apply never waits for a start. A removed root stops every container. The store's resolution is released on every path |
| Naming | Every node, command path, state and answer key is spelled as the operator-surface child's tables spell it. JSON keys are kebab-case |
| Data flow | Only the plugin talks to the library. No always-on file names the plugin or the library. No Go call leaves the plugin tree for another component |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn` and `Error` line names a code of the problem-class table, and each doctor check carries `Fix` at the phase that holds its state |
| Rule: `ai/rules/plugins.md` | Declare mode starts nothing: no cgroup, no mount, no sweep before the engine entry runs |
| Rule: `ai/rules/goroutine-lifecycle.md` | One worker per container and two readers per running container, each ended by its container's removal. No goroutine per exit or per line |
| Rule: `ai/rules/go-standards.md` | The library's environment key is the only non-Ze variable read, with its reason on the line. A probe that cannot run answers unknown |
| The diff | Each hunk matches its row of the diff table and nothing else in the vendor tree differs from upstream |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The module landed alone | `git show --stat <dependency commit>` lists only `go.mod`, `go.sum` and paths under `vendor/` |
| The patch series is applied and stamped | `./le test unit pkg path ./internal/le/vendorpatch/` and `./le go patch-stamps check` |
| A container starts in the guest | `./le test qemu ci test test/container/container-start.ci` |
| The `container` suite runs | `./le test container` lists and runs the suite's tests |
| The gate compiles out | `TestBuildTag_Container_Absent` under a tag set without `ze_container` |
| The kernel carries the new option | A runtime kernel build passes its requirement check, and `TestEnrolledKernelSymbolsAreInRuntimeRequire` is green |
| The register row exists | `grep -n "container" ai/allowed-system-commands.md` |
| Every problem site names a code | `./le arch doctor-coverage` on the changed lines |
| No call crosses the process boundary by Go | `./le plugin boundary check` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Confinement defaults | The description equals the upstream defaults listed in the umbrella's research, plus the fourteen capabilities, with no host namespace shared unless `host-namespace` names it |
| The cgroup namespace | A container's own cgroup reads as the root, and no other container's cgroup is visible |
| The filter | A non-native architecture is killed, the x32 range is refused, an argument comparison checks both halves of a 64-bit value, and the filter is loaded at the library's own point |
| Descriptors | The entrypoint holds three descriptors. The created call passes a process id and a path, never an open descriptor into the container |
| Paths | The root mount point and the state directory are root-owned and hold no symbolic link. A volume is mounted only from a folder the store answered. The image's account files are opened under a descriptor for the root |
| The init binary | Init runs from the sealed copy on every platform |
| Resource exhaustion | Every container has a memory, a process and a CPU limit. A memory-backed mount has a size counted in `memory`. The output buffer and the line length are bounded |
| Privilege grants | A capability outside the default set, a shared host namespace, a device and a kernel setting appear only when the config names them |
| Operator text | A container name, an image-declared string and a line of container output are length-bounded and stripped of control characters before they reach a terminal or the log. A variable's value reaches none of them |
| Cleanup | A failed start leaves no cgroup, mount or state directory, and releases the store's resolution |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| An assumption breaks in phase 2 | STOP. Report what the guest showed. The diff's shape is a design decision |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Init is cloned into its cgroup (H-1) | Attach after the start, as upstream does; or keep the cgroup namespace out of the clone and have init unshare it | Upstream creates the namespaces after the attach. A clone before the attach roots the cgroup namespace at the daemon's cgroup. An unshare inside the Go init changes one thread only and would break without a compile error |
| The checkpoint client and the systemd driver are removed by hunks | The upstream opt-out build tag; leaving both linked | Ze derives build tags from the manifest for the `ze_` prefix only, so a tag would need new tooling and every build site, and a missed site would link the client in silence. A hunk cannot be missed |
| The filter is Ze code behind a seam in the stub file | The whole filter as a file inside the patch | Code inside a patch is not linted, not tested by Ze's gates and is reviewed as a diff. The seam is ten lines |
| The syscall tables are data checked against the vendored number files | A generator command; tables from kernel headers | The vendored module holds every architecture's numbers. A test compares, so no new tool is added. A call newer than the tables gets "not implemented", which is the upstream behavior for a call newer than the profile |
| The filter covers the native architecture only | Tables for the 32-bit ABIs | A profile that lists one architecture kills calls under any other, as upstream does. Two tables are what the two shipped architectures need |
| Containers are stopped in the stop drain | Stopping them when the connection closes | A daemon stop gives a plugin 3 seconds in the daemon and 1 second forked, with no `bye`. The drain is the one call that waits, up to 60 seconds, on both transports |
| The drain budget is 37 seconds and `stop-timeout` at most 30 | A budget from config | The budget is declared at Stage 1, before config arrives. The drain handler spends it in order: up to 2 seconds to end shell sessions (a hang-up, then a kill after that grace, which is the shell child's step), up to 30 seconds for the containers to obey their stop signals, and 5 seconds for the cgroup kill and the cleanup |
| A dead daemon's containers are removed at the next start (owner ruling) | Adopting them; a parent death signal set inside init | The owner ruled that containers restart with Ze and are never re-adopted, and the diff refuses a death signal |
| Containers are killed through their cgroup | The plugin supervisor's process-group kill | A container takes a new session and may create groups of its own. The cgroup holds every process whatever it does |
| A new always-on leaf holds early roles by key | A hook variable for containers in `cmd/ze`; a second role inside the seed-init leaf | Always-on code must not name a feature. The seed role's checks do not fit a role with no arguments |
| No file of the plugin supervisor is edited | Moving its relay reader and group kill to a shared leaf | Neither fits: the idle bound and the pid hold exist for cases a cgroup removes. An edit there would owe the whole lifecycle regression table for no gain |
| Host privilege is judged in the daemon and by doctor, never by offline validation | An enrolment like a kernel option | Offline validation runs as the operator, who may not be root while the daemon is. A kernel option reads the same from any process; a privilege does not |
| One kernel option is added, for the hard CPU limit (owner ruling) | Also an I/O throttle; a CPU weight only | The owner ruled the hard limit in and the I/O throttle out. A weight shares and does not cap, and the operator-surface child's table has no weight node |
| `memory` has a floor of 64 MiB | The schema's smaller floor | Init's start-up is charged to the container, and upstream names the same failure in its own error text |
| A start that fails five times in a row gives `failed`; an entrypoint that keeps exiting is restarted without a bound | A bound on both; no bound on either | A start that fails needs an operator: the image or the config is wrong. An entrypoint that exits is the workload's behavior, and the policy the operator chose says to start it again. The delay is capped, and doctor reports the loop |
| A user name is resolved by this child, inside the image root | The store resolving it | The store hands the image's configuration as fetched and resolves nothing. The lookup reads files a container controls, so it opens them under a descriptor for the root |
| A volume the image declares and the config does not stays in the writable layer | An automatic volume | The owner's direction keeps a container's filesystem until its image is updated, and a volume is something the operator declares |

## Known Limitations
- Running containers are never re-adopted. When Ze restarts, including for its own update, every container is stopped and started again; after a crash, the next start removes what was left and starts each container anew. Re-adoption is not part of this work, by owner ruling.
- Containers run on the appliance and under a daemon that runs as root. A daemon that runs as an unprivileged account refuses a container config, by owner ruling.
- No user namespace: a container's root is uid 0 on the host, confined by the other mechanisms. The diff refuses identity mappings.
- A 32-bit program in an image is killed by the filter.
- No I/O throttle and no CPU weight, by owner ruling: the limits are memory, processes and a hard CPU limit.
- Captured output is in memory: it is lost when the container is removed or the daemon restarts.
- A host whose cgroup v2 hierarchy is not at `/sys/fs/cgroup` cannot run containers: the library fixes that path.
- Entering a running container is refused by H-3 until the shell child of the umbrella replaces it.
- The size caps of the writable layer and the volumes, the data directory, and the growth and format of the persistent partition are not here: they belong to the image-store child and to `spec-appliance-perm-grows-at-install`.

## RFC Documentation (Scope: protocol)
Not applicable: Scope is plugin.

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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
