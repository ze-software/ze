# Spec: container-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P3 |
| Release reason | A new capability: Ze runs OCI container images itself. No shipped behavior is wrong without it, so it is a post-release feature until the owner brings it into the first release. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze cannot run a third-party workload today. An operator who wants a small
service beside the router (a collector, an exporter, a resolver, a vendor
agent) needs a second machine or a general-purpose Linux install, and loses the
appliance's property that the running system is exactly the image that was
installed.

The goal: Ze runs OCI (Docker) images itself, with the same immutability the
appliance has for its own system.

- Each container has its own root filesystem, built from an image pinned by
  digest and never modified in place.
- A container may write anywhere in its root. Those writes land in a layer of
  its own, above the image.
- An update replaces the image and restarts the container. The previous image
  stays available for rollback.
- State meant to survive lives only in volumes the operator declared.
- A container is confined: its own namespaces, a resource-limited cgroup, a
  reduced capability set and a system-call filter, so it cannot starve or
  reach the routing daemon.
- The container's network attachment is an ordinary Ze interface, so Ze's
  firewall and routing apply to it and nothing installs rules beside Ze's own.
  A VRF applies to it once VRF support ships: Ze refuses the `vrf` leaf today.

### Owner decisions

| Decision | Ruling |
|----------|--------|
| Docker daemon | Not shipped. Ze runs the containers itself. |
| Number of binaries | One. No runtime binary beside `ze`. |
| Build | The feature is a compile-time feature of Ze's build: a build without it carries none of its code. |
| Third-party runtime | `runc` is a Go module dependency. Ze writes the glue that dependency lacks. This spec reads that ruling with cgo staying off, as every Ze build has it: the glue is what `runc` implements in C. |
| Operator model | The configuration tree and the commands follow the container model operators already know from an established router operating system: a named container with its image, networks, published ports, volumes, environment, resource limits and restart policy, and image management as operational commands. No Ze page, spec or commit names or quotes that system. |
| Implementation model | Implementation agents for this work run on the strongest tier. |
| Design | Approved on 2026-11-16: the upstream library used whole behind a small build-time diff, with cgo off, and the child-spec split. |
| Release timing (O-3) | After the first release. |
| External-program authorisation (O-4) | Authorised on 2026-11-16, scoped to the container init role executing a container's entrypoint. The row in `ai/allowed-system-commands.md` lands in the commit that adds the code it names, citing this ruling. |
| Feature gate default (O-5) | On in every daemon build, like every other gate. |
| Image store (O-6) | Images are stored unpacked, one folder per image digest. |
| Networks and ports (O-7) | Container networks and published ports are nodes of the container config, realised through Ze's interface and firewall mechanisms in state Ze owns and shows. |
| Shell (O-8) | A shell inside a running container is in scope now, as a child spec of this work, not a later one. |
| Writable layer (O-2) | A container's filesystem persists until its image is updated: the writable layer is on disk, keyed by container name and image digest. |
| Where containers run | On the appliance and under a daemon that runs as root, at first. A host where Ze runs as an unprivileged account, or whose data directory cannot enforce a size cap, refuses a container config and says why. |
| Persistent partition | The appliance grows its persistent partition to the disk at install. This is appliance work of its own and comes before the image store (`spec-appliance-perm-grows-at-install.md`). |
| Daemon restart | When Ze restarts, including for its own update, containers are stopped and started again. Re-adopting running containers is not part of this work. |
| Shell design | Approved on 2026-11-16: entry by extending the build-time diff, refused for a container granted process tracing; one general interactive-command registration any plugin can use, with seven new plugin-protocol messages; a live terminal on SSH and gRPC, a bounded run on REST, web and MCP. A probe decides whether the fallback entry design is needed. |
| Container names over DNS | Not at first. A container gets the router's resolver list. |
| Published ports and the firewall | A published port must also be allowed by the operator's firewall policy. Doctor reports a published port that policy blocks. |
| Stricter than the reference model | No `privileged` node. Volumes are stored by Ze and size-capped, with no host path. |
| The list of containers | The list is named `instance`, because `name` is Ze's selector keyword. |
| Accepted defaults | The fourteen capabilities published images expect; a bridge per network; outbound translation on, with a node to turn it off; a hard CPU limit, which adds one kernel option, and no I/O throttle; the reference model's words wherever a Ze rule does not forbid them, including the workload's `environment` and short image names resolved against a default registry with the resolved digest recorded; memory 512 MiB; restart on failure; rollback by command only; health checks as a later child. |

### Open decisions (owner)

All of these were resolved on 2026-11-16 as the Owner decisions table above states. The table below is kept as the record of what each answer changed; no row is still open.

| # | Question | Recommendation | What each answer changes |
|---|----------|----------------|--------------------------|
| O-1 | Resolved by the owner: `runc` as a dependency, Ze writes the glue (see Owner decisions). Open inside it, for design: whether Ze uses the `runc` library whole, supplying its three C-only pieces in Go through a build-time diff, or imports only its pure-Go leaf packages and writes the init sequence itself | The library whole, if the study shows the diff is small and stable | Using it whole inherits the init sequence and its vulnerability fixes, and ties Ze to a diff that each `runc` release may break. Leaf packages only makes Ze own the sequence |
| O-2 | Owner direction, reading to confirm: a container's filesystem persists until its image is updated, and is stored in a folder named by the image's hash. Read here as: images are stored by digest, and each container's writable layer is stored on disk under its name and its image's digest, so it survives restarts and reboots and a new digest starts from a fresh layer | As directed, plus a command that discards the writable layer on request, and a size cap on it | The writable layer moves from memory to the persistent partition, so it needs a disk quota. The previous digest's folders stay for rollback, which returns both the old image and its old writes. If "the binary" meant Ze's own binary, images would be refetched at every Ze update, which this spec does not adopt |
| O-3 | Release timing: first release, or after | After | Decides the folder this work lives in and its priority |
| O-4 | Running a container's entrypoint is running a program Ze does not carry. `ai/rules/go-standards.md` forbids that without a row in `ai/allowed-system-commands.md`, which only the owner adds | Add the row, scoped to the container init role | Without the row no code that starts a container may land. The only row of this kind today is the operator's own plugin program |
| O-5 | Every declared feature gate is on in every daemon build today (distribution and appliance). Is the container gate on by default like the others, or off unless a build asks for it | On by default, like every other gate | Off by default needs a new mechanism: no gate today is declared and left out of the default builds |
| O-6 | Image store: an unpacked folder per image digest, or an image file per digest | The folder, as the owner's direction reads | See Key Design Decisions. The folder needs no kernel change and no writer; the file gives a single immutable object and needs both |
| O-7 | Container networks and published ports: nodes of the container config that Ze realises through its interface and firewall mechanisms, or left to the operator as a bridge and firewall terms | Nodes of the container config | The first needs a declared way for the feature to contribute interface and firewall state without the engine naming it. The second needs no new mechanism and departs from the reference model |
| O-8 | A shell inside a running container is part of the reference model. Ze has no command with an input stream, and every command must work on every surface | Leave it to a child spec of its own, after the rest works | It needs a design for a bidirectional command, and joining a running container's namespaces, which the upstream library does in C |

### Corrections after the child specs were drafted (2026-11-16)

The agents that drafted the children read the source further than this spec had, and where they disagree with a sentence elsewhere in this file, this table wins.

| This spec said | The source says | Held in |
|----------------|-----------------|---------|
| The build-time diff is about 20 to 30 lines in two functions | 60 to 90 lines, six hunks, five upstream files. The cgroup must exist first and init must be started inside it in one step; otherwise the container's cgroup namespace is rooted at the daemon's own cgroup and the container sees the host's cgroup tree | Runtime |
| The plugin supervisor's forking thread, pid-safe kill, stderr relay and stop grace are reused | None is reusable as code, and a group kill cannot reach a container in a session of its own. A container is killed through its cgroup | Runtime |
| A container is killed after its stop timeout at daemon stop | A whole-daemon stop gives a plugin about three seconds. Containers are stopped in the stop-drain step, which caps a container's stop timeout at 30 seconds | Runtime |
| Checkpoint support is excluded by its upstream build tag | No Ze build can carry a tag that is not a Ze gate. The diff removes it instead | Runtime |
| The filter is one replaced file | The diff turns the stub into a registration seam, and the filter is Ze code in the plugin, where Ze's gates judge it | Runtime |
| The cgroup mount locator moves to a core leaf | The upstream library accepts one mount point only. A host with cgroup v2 mounted elsewhere is refused with a doctor finding | Runtime |
| A crashed daemon is not mentioned | With no parent death signal, a crashed daemon leaves containers running until the next start, which sweeps and restarts them | Runtime |
| A firewall term that matches the host end of a container's link applies to its traffic | With a bridge per network, routed traffic enters by the bridge. A term matches the network's bridge, and one container is matched by its address | Network |
| The firewall applies to container traffic by configuration only | Publishing a port needs a new general firewall match: the destination is one of this router's addresses. Without it a published port would capture transit traffic | Network |
| The interface backend moves the peer end into the namespace on a locked thread | The netlink library creates the peer end directly inside the namespace and configures it through a socket bound there. No Ze thread enters the container's namespace | Network |
| Published-port state is put in place in the created-but-not-running window | It follows config changes, not each container start | Network |
| A quota on each writable layer, as if available | The persistent partition is formatted with no quota feature and the kernel files do not pin quota support. The cap is a project quota, which needs a format change at build and a kernel pin | Image store |
| Whether the persistent partition grows at install is unvalidated (A-6) | It does not grow | Appliance spec |

### Child specs (proposed split, written after the design is approved)

| Child | Holds | Depends on |
|-------|-------|------------|
| Runtime | The upstream library as a dependency in a commit of its own, the build-time diff, the init role, the system-call filter, cgroup limits, supervision and restart, the kernel requirements, the feature gate and its absent test | - |
| Image store | The registry client, the pull command, layer application into a folder per digest, the writable layer, retention, rollback, the disk quota and free-space checks | Runtime |
| Network | Moving a link into a container's namespace through the interface backend, container networks, published ports | Runtime |
| Operator surface | The config schema, the commands, the doctor checks, the guide page. Each child above adds its own nodes and commands; this child holds what spans them | The three above |
| Shell | A command with an input stream on every surface, and entering a running container | Operator surface |
| Persistent partition growth (`spec-appliance-perm-grows-at-install.md`) | The appliance grows its persistent partition to the disk at install, and formats it so a size cap can be enforced | - |

The files are `spec-container-1-runtime.md`, `spec-container-2-image-store.md`, `spec-container-3-network.md`, `spec-container-4-operator-surface.md` and `spec-container-5-shell.md`. The operator surface child holds the one table of config words and command words, so its design is settled before any sibling's wiring phase, and its own implementation phases come after theirs.

### Proposed shape (not approved)

Ze fetches an image by digest, verifies it, and stores it as one read-only
filesystem image on persistent storage. A container's root is an overlay of
that image under a size-capped writable layer. `ze` starts the container by
re-executing itself in an init role inside fresh namespaces; that process
prepares the root, confines itself and then executes the image's entrypoint.
Ze supervises the container the way it supervises the other processes it
depends on. This work is expected to split into child specs (image fetch and
store, runtime and confinement, network attachment, configuration and
commands); the split is decided at design.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every subsystem shares
  → Constraint: a compile tag is a boundary. Always-on code never imports a gated package, and reaches it only through a registry or a seam the feature fills from its own `init()`
  → Constraint: every plugin also runs as a separate process, so the feature reaches interfaces and the firewall through the bus, a typed request or a command, never a Go call into another component's state
  → Constraint: a config a backend cannot apply exactly is refused at verify, never approximated
- [ ] `docs/architecture/plugin/feature-gates.md` - how a feature compiles out
  → Decision: the feature is a self-registering edge plugin, so its gate is lines in `feature-gates.txt` plus a present and an absent build-tag test. No build-matrix row is added
  → Constraint: a shared helper that must compile out stays inside the gated package; a core leaf cannot be gated
- [ ] `ai/doors/system-plugins.md`, `docs/architecture/module-tiers.md` - placement
  → Decision: an engine with its own config root that nothing imports is an edge plugin under `internal/plugins/`, and it owns its commands, schema and doctor checks
- [ ] `ai/doors/core.md`, `ai/rules/architecture.md` - persistence
  → Constraint: runtime state goes through the state store under a registered key. The store rewrites a whole value on change, so image bytes cannot live in it
  → Constraint: a raw write under the config or state directory needs a per-file allowlist entry with a reason
- [ ] `ai/doors/forwarding.md` - interfaces and firewall
  → Constraint: Ze owns only kernel state it marked. A reconciler that reads "not in config" as "mine to delete" once removed state another owner installed, so the container's end of a link must be owned and marked by one party
  → Constraint: interface names go through the shared resolver
- [ ] `ai/rules/go-standards.md`, `ai/allowed-system-commands.md` - external programs
  → Constraint: shipped code runs no external program without the owner's row in the register. A container entrypoint is such a program (O-4)
- [ ] `docs/architecture/appliance/kernel-profiles.md`, `ai/rules/platform-linux.md` - the runtime kernel
  → Decision: the router image and the lab Docker host image boot the same kernel build, so what Docker needed is already there
  → Constraint: every kernel option the feature needs is requested and pinned in the two kernel files and enrolled as a kernel capability
- [ ] `ai/doors/appliance.md` - the device
  → Constraint: an image in the field is the contract for a path under the persistent partition and for a state key. Paths and keys this feature introduces are permanent once shipped
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor
  → Constraint: every problem class the feature can produce has a doctor check with its cause and the command that fixes it

### Upstream source (reference, not a dependency)
- [ ] `runc` at tag v1.5.3, cloned at `~/Code/github.com/opencontainers/runc` - the init sequence a correct runtime performs
  → Constraint: its namespace entry and its system-call filter are C. With cgo off, the library cannot create a container, and a container with a filter profile is refused. Read from build constraints and imports; a confirming build was not run
  → Decision: its init sequence, default mounts, masked and read-only paths and its fixes for published vulnerabilities are the checklist Ze's runtime is written and tested against
- [ ] The Go standard library, `syscall.SysProcAttr` - what a pure-Go parent can do
  → Decision: the parent can start a child in new namespaces with identity mappings, directly inside a cgroup, with a death signal. It cannot run code between fork and exec, so root setup, capability drop and the filter run in a re-executed init role

**Key insights:** (minimal context to resume after compaction)
- The upstream runtime's main library does not import its C code. With cgo off, only its namespace-entry package and its system-call filter are missing, and the library reaches both through a small startup protocol and one function. Ze uses the library whole and supplies those two pieces in Go.
- The library fixes the init command to the running binary with the argument `init`, and Ze already has an `init` command. The container init role is therefore selected by the library's own environment key, before command dispatch and before crash logging starts.
- The library creates a container and leaves it blocked before its entrypoint runs. In that window the network namespace exists and the init process is known, so Ze's interface backend moves and addresses the link there, and the library's own network-device feature is not used.
- The library performs the final exec itself, inside the init role, where Ze's crash-log pipe was never set up. Ze's execve wrapper does not apply to it.
- The reference operator model pulls an image only on an explicit command, and a commit naming an absent image succeeds while that container does not run. Operators of that system report this as its main trap, so Ze reports it in doctor with the command that fixes it.
- The runtime kernel already has namespaces, cgroup v2 (memory, pids, cpuset, devices), overlayfs, the system-call filter, veth and loop devices. It lacks: extended attributes in squashfs, a hard CPU limit, an I/O throttle, verified-read integrity, EROFS and AppArmor.
- The squashfs writer Ze already vendors writes root-owned regular files, directories and symlinks only. It cannot hold a general image (no owners, device nodes, hard links or extended attributes).
- The persistent partition is about 950 MB on the default 2 GiB image. Nothing found grows it at install.
- Ze has three process supervisors and none is shared. The plugin supervisor has the pieces a container needs.
- The interface component creates a veth pair but cannot move one end into another namespace.
- No gate is off by default, no automatic rollback exists, no per-consumer disk accounting exists, and no code in Ze creates a cgroup, drops capabilities for a child or installs a system-call filter.
- A long-lived daemon that re-executes itself as init is exposed to the leaked-descriptor and overwrite-the-runtime vulnerability classes. The upstream defences are: close every descriptor before starting init and again before the final exec, verify the working directory after the last change of directory, and run init from a sealed copy of the binary.
- A container can change its own root while the runtime is still setting it up. Every operation inside the root resolves under a descriptor for that root, never by path.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `feature-gates.txt` - the one declaration of every compile-out-able feature: one line per gated package under a `ze_<feature>` tag; a self-registering plugin whose only composition root is the generated one needs nothing more than its lines
- [ ] `cmd/ze/hub/service_registry.go` - the construction registry for optional daemon services (listeners); a factory registers from a file built only under its tag, and the hub never imports the service package
- [ ] `ai/doors/system.md` - the shared design: a compile tag is a boundary, every plugin can run in another process, runtime state goes through the one store, every command registers once
- [ ] `ai/doors/appliance.md` - the device takes a new system image through one registered update backend with an A/B switch; an image in the field is a contract
- [ ] `ai/doors/forwarding.md` - interfaces, firewall and traffic have pluggable backends; Ze owns only kernel state it marked; a config a backend cannot apply exactly is refused at verify
- [ ] `ai/doors/core.md` - a raw file write under the config or state directory is refused; state goes through the store under a registered key
- [ ] Source files of the other mechanisms this design extends: to be filled during research.

**Behavior to preserve:**
- To be filled during research.

**Behavior to change:**
- None. This is a new feature behind a compile-time gate.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Feature-gate manifest and its generator | `feature-gates.txt`; `internal/le/repo/featuretags/daemontags.go` `DaemonTags`; `docs/architecture/plugin/feature-gates.md` | One line per gated package under a `ze_<feature>` tag. `DaemonTags` returns every declared gate unfiltered, so every gate is on in the distribution and appliance builds. `./le repo update` regenerates the derived tag lists | Extends: the container packages are new lines under one new tag. A gate that is declared and off by default does not exist; O-5 decides whether one is built |
| Absent and present build-tag tests | `cmd/ze/hub/build_tag_vpp_absent_test.go` and its siblings; `./le arch tier check` | Prove a gated plugin is not registered, its config root is refused as unknown, and no always-on package imports it | Extends: one pair for the new tag |
| External plugin process supervisor | `internal/component/plugin/sysproc_linux.go` `NewSysProcAttr`, `StartChild`; `internal/component/plugin/process` `startExternal`, `ProcessManager.Respawn` | Forks the operator's plugin in its own process group with a death signal bound to one long-lived forking thread, relays its stderr to the log, stops it with a grace then a group kill that cannot hit a recycled pid, and respawns up to 5 times per 60 s and 20 in total | Extends in part: the forking thread, the pid-safe kill, the stderr relay and the stop grace are what a container needs. The connect-back and the five startup stages do not apply: a container is not a plugin |
| VPP process supervisor | `internal/component/vpp/vpp.go` `VPPManager.Run`, `runOnce` | Runs VPP with a 1 to 30 s doubling restart backoff and publishes connected and disconnected events. It sets no process attributes and stops by kill | Model only, for the backoff loop and the lifecycle events. Not extended: it has no process group, death signal or graceful stop |
| Self re-execution by role | `cmd/ze/dispatch.go` `dispatchMain`; `internal/core/seedinit/register.go` `Dispatch`; `internal/core/crashlog/exec_unix.go` `Exec` | `ze` selects a role from its first argument, or from an environment key before startup. Every execve in Ze goes through one wrapper, and a raw one is refused by a lint rule | Extends: the container init is one more early role, selected by an environment key before startup as the seed role is. The execve wrapper does not apply: the upstream library performs the final exec inside that role, where the crash-log pipe the wrapper protects was never created. No role today runs inside new namespaces |
| Locked-thread namespace entry | `internal/component/kernelcap/namespace_linux.go` `InThrowawayNetworkNamespace` | Unshares a network namespace on a thread that is locked and never unlocked, for kernel probes | Extends: the same thread discipline configures the container's end of its link inside the container's network namespace |
| Interface veth and bridge | `internal/component/iface/yang/ze-iface-conf.yang` `list veth`, `list bridge`; `internal/component/iface/backend.go` `CreateVeth`; `internal/plugins/iface/netlink/manage_linux.go` | Creates a veth pair in the host namespace with both ends up. The managed end takes addresses and units; the peer end takes none. Nothing moves a link into another namespace | Extends: the host end is already an ordinary Ze interface. The interface backend gains moving the peer end into a named namespace and addressing it there, reached through the interface component's boundary and never a direct call |
| Interface VRF leaf | `internal/component/iface/yang/ze-iface-conf.yang` `leaf vrf`, validator `unimplemented-vrf` | Refuses any config that sets it | Does not apply until VRF support ships (`plan/post-release/features/spec-vrf-0-umbrella.md`) |
| Firewall match by interface | `internal/component/firewall/yang/ze-firewall-conf.yang` `input-interface`, `output-interface` | Matches a rule by interface name, with a prefix wildcard, in tables Ze owns by the `ze_` prefix | Extends by configuration only: the rules apply to the host end of the container's link with no code change |
| URL fetch and digest check | `internal/core/fetch/scheme.go` `schemes`; `ToFile`, `ToDisk`, `SameOrigin`, `CheckSHA256` | Fetches http, https and tftp to a file or a device, with an optional SHA-256, three retries from byte zero, and a refusal of any redirect that leaves the origin. No authentication header, no cancellation, no resume | Extends in part: the digest helpers and the stall timeout are reused. A registry pull needs a token exchange, content negotiation and a blob redirect to another host, which `SameOrigin` refuses by design, so the registry client is new code beside it |
| System update backend | `internal/component/config/system/backend.go` `UpdateBackend`; `selfupdate.go` | One update backend per platform, a singleton. Self-update keeps one previous binary, rolls back by hand, and keeps its history in the state store. On the appliance the A/B switch and test boot belong to gokrazy and the build host | Model, not extended: its registry is a system singleton. The verbs (check, download, apply, rollback, history), the keep-one-previous step and history in the state store carry over. Automatic rollback after a failed start exists nowhere in Ze and is new |
| Runtime state store and its gate | `internal/core/statestore`; `pkg/zefs/keys.go`; `internal/le/arch/fspersistence/fspersistence.go` `fileAllowlist` | Small runtime state under registered keys. A raw file write under the config or state directory is refused unless its file is allowlisted with a reason, as the self-update binary is | Extends for per-container metadata (current digest, previous digest, history). Does not apply to image bytes and volumes: they are artifacts, not state, and need allowlist entries and a data area of their own |
| Free-space helpers and the storage doctor check | `internal/core/diskspace`; `internal/component/doctor/checks_storage.go` | Reads free space; self-update requires twice the binary size; doctor warns when the config partition is nearly full | Extends: the helpers are reused before a fetch. Per-consumer accounting does not exist and is new |
| Doctor checks and diagnostic codes | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck`; `docs/architecture/doctor-and-health-checks.md` | A check registers from the package that owns the dependency; live state is answered by the plugin itself | Extends: one check per dependency and problem class the feature adds |
| Kernel requirement enrolment | `internal/component/kernelcap/kernelcap.go` `MustRegister`; `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`; `internal/appliance/kernelreq.go` `enforceKernelRequirements` | A feature declares a kernel option once. The kernel build refuses without it, and `ze` refuses at start, reload and validate when a config uses the feature on a kernel that lacks it, and reports it in doctor | Extends: the container feature enrols each option it needs, including the ones that are on today only by upstream default and so are not protected |
| cgroup v2 mount and its locator | gokrazy init mounts it at boot; `internal/component/ike/dataplane/override_guard_linux.go` `cgroup2Root` | The appliance mounts cgroup v2 and places no process in a child group. The only Ze code that uses a cgroup finds the mount from the mount table, because a distribution host may mount it elsewhere | Extends: finding the mount is shared by moving the locator to a core leaf. Creating, limiting and removing a cgroup is new |
| Overlay over the read-only root | `internal/appliance/dockerhost/boot.go` `writableOverlays` | In the lab Docker host image, mounts an overlay with the read-only system image below and a memory filesystem above | Model: the same mount shape is the container root. Not extended as code, because it lives in another program |
| Register of external programs Ze may run | `ai/allowed-system-commands.md`; `ai/rules/go-standards.md` | Lists every program outside Ze that shipped code may execute. The default is no, and only the owner adds a row | Extends, by the owner: starting a container entrypoint needs its own row before the code lands (O-4) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a container root in the config file, delivered to the container plugin as its declared root through the config transaction (verify, apply, rollback).
- Commands: image fetch, update, delete; container restart and writable-layer discard; listings and logs. Each is a registered command with a schema node, reached from every surface.
- A registry: image manifests and blobs over HTTPS, read by Ze.

### Transformation Path
1. The config transaction hands the plugin its root. Verify refuses what the kernel or the store cannot honour. Apply computes which containers start, stop or restart.
2. For a container to start, the plugin resolves its image reference to a digest in the store. An absent image is a state, not an error.
3. The plugin prepares the root: an overlay of the image folder below the container's writable layer.
4. The plugin fills the upstream library's container description and asks it to create the container. The library re-executes `ze` in the init role, clones it into new namespaces inside the container's cgroup, runs its init sequence, and blocks before the entrypoint.
5. In that window the plugin asks the interface backend to move the container's link into the new network namespace and address it, and the firewall state for its published ports is put in place.
6. The plugin releases the container. The init role installs the system-call filter and executes the entrypoint.
7. The supervisor waits on the container, records its exit, and applies the restart policy.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to plugin | The config transaction and registered commands. The plugin must also work when run as a separate process | No |
| Plugin to interface component | A typed request or a command, never a Go call, for creating the link and moving its peer end | No |
| Plugin to firewall component | A declared contribution of firewall state in tables Ze owns. The firewall's table registry is process-local today, so a plugin that can run forked needs a request, not a direct registration | No |
| Daemon to container init | The upstream library's startup protocol over inherited descriptors, inside one binary | No |
| Ze to registry | The registry pull protocol over HTTPS | No |
| Ze to kernel | Namespaces, cgroup files, mounts, the system-call filter, netlink | No |

### Changed-Wire Census

**Search evidence:** searched every `*-conf.yang` in the tree for a root named `container` and found none; searched the command schema files for a `container` command subject and found none. The positive control is the interface schema's `list veth`, which the same search finds. No existing wire changes: every wire below is new. The registry protocol's writer is outside the repository.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Container config root | writer | The operator's config file, and the config editor on every surface | No existing root of that name in any config schema | New. Nothing to migrate |
| Container config root | reader | The container plugin's config callbacks, to be created with the child specs | No existing reader | New |
| Registry pull protocol | writer | A container registry, outside the repository | Not carried by any code in the tree: `internal/core/fetch/scheme.go` has http, https and tftp to a file only | New reader only. The registry's behaviour is fixed by its published specification |
| Registry pull protocol | reader | The registry client in the image store child spec | No existing reader | New |
| Upstream library startup protocol | writer and reader | The upstream library on both sides, inside one binary, as changed by the build-time diff | Read at tag v1.5.3: `libcontainer/process_linux.go` `initProcess.start`, `libcontainer/init_linux.go` `startInitialization` | Both ends are the same build, so no version skew exists. The diff is re-verified at each upstream release |
| Per-container state keys | writer and reader | The container plugin through the state store | No existing key for containers in `pkg/zefs/keys.go` | New keys. Once shipped, a key is a contract with images in the field |

### Integration Points
- The feature-gate manifest: new lines under one new tag.
- The plugin process supervisor's forking thread, pid-safe kill and stderr relay: reused for the container's lifecycle.
- The interface backend: gains moving a link into a namespace.
- The firewall component: gains a way for a plugin to contribute state for published ports and outbound translation.
- The kernel capability registry and the two kernel option files: each required option is enrolled.
- The doctor registry: one check per dependency and problem class.
- The state store: per-container metadata.
- The allowed-external-programs register: one row, added by the owner.

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
| A-1 | The upstream Go container runtime library cannot create a container with cgo off | Read at tag v1.5.3: the namespace-entry package has no file that builds without cgo, and the filter stub refuses any profile | O-1 has a third answer: embed the library with no build change | A type-check of that tag with cgo off, run 2026-11-16: the namespace-entry package has no buildable file, the spec-conversion package fails on two undefined user-namespace functions, and the main library, the binary sealing, the capabilities and the filter stub type-check | confirmed |
| A-2 | The runtime kernel can mount a filesystem image file read-only | The resolved config of the current amd64 runtime kernel has the loop device and squashfs built in. Both come from upstream defaults and are not pinned. No arm64 config was read | The image store needs another form, such as an unpacked directory tree | The resolved arm64 config, and a listing of the loop control device on a booted appliance | confirmed on amd64, open on arm64 |
| A-3 | The Go standard library can start a child in new namespaces, with identity mappings, directly inside a cgroup, without C | Read in the standard library source | The pure-Go runtime is not possible as described | Done | confirmed |
| A-4 | A root filesystem with no write support prevents a container from overwriting the `ze` binary through its own process image | Inference. The upstream source says only that a read-only bind mount was unsafe because a privileged container could clear the flag | The appliance needs the sealed-copy defence too | A test that attempts the overwrite from inside a container on the appliance. The design seals the binary on every platform regardless, so this assumption carries no weight | unvalidated |
| A-5 | `ze`, running as root under gokrazy's init in the root cgroup, can create child cgroups and enable their controllers | The mount is made by gokrazy init, and the controllers are listed on a host running the same kernel | The runtime cannot limit a container on the appliance | Reading the controller files in a booted appliance guest | unvalidated |
| A-6 | The persistent partition can hold images and volumes | Default image size and partition layout read from source: about 950 MB by arithmetic. No code was found that grows the partition at install | The feature needs a larger default image or a grow step first | A full read of the installer's disk code | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Ze owns a confinement surface that a mature runtime hardened over years, and misses a step | A review against the upstream runtime's init sequence finds an absent step | Derive the sequence from the upstream source as a checklist, one test per step |
| R-2 | A container exhausts memory, CPU, processes or disk and takes the routing daemon with it | A container without limits is accepted by config | Limits are mandatory with defaults, and the writable layer and volumes are size-capped. A hard CPU limit needs a kernel option the runtime kernel lacks today |
| R-3 | A malicious image, or a running container sharing a volume, redirects a mount or a write the runtime makes inside the root (a symlink at a mount target, a replaced device node) | A test image with a symlink at a mount target starts | Resolve every target under a descriptor for the root, verify the type of what was opened, and mount through descriptors |
| R-4 | The daemon leaks an open descriptor into a container, which uses it to reach the host filesystem | A container lists a descriptor above standard error at its entrypoint | Close every descriptor before starting init and again immediately before the final exec, and verify the working directory |
| R-5 | A container overwrites the `ze` binary through its own process image | The overwrite attempt succeeds in a test on a writable host root | Run the init role from a sealed copy of the binary on every platform |
| R-6 | An image layer writes outside its tree, plants a device node or setuid file, or expands without bound while Ze turns it into a root | A crafted layer changes a host file or fills the disk | Build the root image without writing through host paths, cap the expanded size, and refuse entry types the config did not allow |
| R-7 | The feature's kernel options are present today only by upstream default and disappear on a kernel bump | A kernel build succeeds without one | Request and pin each option, and enrol it as a kernel capability so `ze` refuses at validate, start and reload |
| R-8 | The squashfs format, as this kernel builds it, drops extended attributes, so file capabilities in an image are lost and programs that rely on them fail | A container program that needs a file capability fails to start | Avoided by storing images unpacked on the persistent partition (O-6). It returns if an image file is chosen |
| R-9 | An upstream release changes the two functions the build-time diff touches, and the diff applies cleanly but is now wrong. Those files had 32 and 25 commits in two years | A container fails to start after an upstream version bump | Pin the upstream version, and gate every bump on the functional tests that start a container. Upstream states that its library API is not covered by its compatibility promise |
| R-10 | The upstream library links code Ze never runs: the systemd cgroup driver and its bus client, and checkpoint support | Binary growth and new modules in the dependency list | Exclude checkpoint support by its upstream build tag, and decide at the runtime child spec whether the diff also removes the systemd driver. About 13 modules are new to Ze |
| R-11 | Each container start copies the whole `ze` binary into memory to seal it, when the zero-copy sealing the library tries first is unavailable | Memory use rises by the binary's size at each start | Confirm on the appliance that the zero-copy sealing works on its root filesystem; it needs overlayfs and the new mount interface, which the kernel has |
| R-12 | The upstream library marks every open descriptor of the calling process close-on-exec before it starts init, and Ze is a long-lived daemon with many descriptors | A child process Ze starts afterwards loses a descriptor it was meant to inherit | Go opens descriptors close-on-exec already and passes inherited ones explicitly. A test starts a plugin process after a container has started |
| R-13 | A parent death signal on the container init kills it at once under the diff, because init sees no parent from inside its new process namespace | Every container exits immediately when the option is set | The plugin never sets the option, and the diff refuses it |
| R-14 | The persistent partition fills: images, writable layers and volumes share about 950 MB with the config and crash logs | A fetch or a container write fails for lack of space | A free-space check before a fetch, a quota on each writable layer, the retention rule, and a larger default image size where containers are configured |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | To be filled at design. |
| How is it reverted? | To be filled at design. |
| Who else touches this path? | To be filled at design. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| To be filled at design | → | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A config names a container whose image is in the store, and is committed | The container's entrypoint runs. The container listing shows it running with the digest of its image, and its process is in namespaces and a cgroup of its own | A container whose image is absent stays stopped (AC-2); a container marked disabled stays stopped, because the operator asked for that |
| AC-2 | A config names a container whose image is not in the store | The commit succeeds. The listing shows that container as waiting for its image and names the image, and doctor reports it with the command that fetches it. Fetching the image then starts the container with no further command | Other containers in the same commit start, because one missing image does not hold the rest |
| AC-3 | The operator runs the image fetch command for a reference | The image is fetched, every blob's hash is checked against its digest, and the image appears in the image listing under its digest, in a folder named by that digest | An image already in the store under the same digest is kept and not fetched again |
| AC-4 | A container writes a file in its root, then is restarted, then the device reboots | The file is still there after the restart and after the reboot | A path under a memory-backed mount the image or config declares is lost, because that is what such a mount is |
| AC-5 | The image a container names is updated to a new digest, then rolled back | After the update the container runs the new image from a fresh writable layer. After the rollback it runs the previous image with the writes it had before the update | Images and layers older than the one previous generation are removed, by the retention rule |
| AC-6 | A container allocates memory past its limit, or forks past its process limit | The kernel kills or refuses inside the container, the container's state records why, and the restart policy applies | The routing daemon and the other containers keep running, which is the point of the limit |
| AC-7 | A container runs with the default confinement | From inside, it sees only its own processes and mounts, holds only the default capability set, and a system call outside the default profile is refused with an error | A capability or host namespace the config grants explicitly is present, because the operator asked for it |
| AC-8 | A container is attached to a container network | It reaches the network's gateway address. The network's bridge and the host end of the container's link are listed as Ze interfaces, and a firewall term that matches the bridge, or the container's address, applies to its traffic | Traffic the firewall config permits passes; nothing else creates packet-filter state: every table present is one Ze owns |
| AC-9 | A container publishes a port | A connection to the published address and port reaches the container, and the translation is visible as state Ze owns and shows | A port no container publishes stays closed |
| AC-10 | A container's entrypoint exits with a failure | The container is restarted under its restart policy with a growing delay, and the listing shows the restart count and the last exit status | A container whose policy is "no" stays stopped, as configured |
| AC-11 | The daemon stops or reloads a config that removes a container | The container gets its stop signal, then is killed after its stop timeout, which is at most 30 seconds, and no process, cgroup, mount or link of it remains | Its image and its writable layer remain on disk, so the next start finds them |
| AC-12 | A config uses a container on a kernel that lacks a required option | Validate, start and reload refuse, naming the option; doctor reports the same | A config with no container is accepted on that kernel, because nothing needs the option |
| AC-14 | Owner's closing test: the operator fetches the published Redis image, configures a container for it with a published port, and commits | The Redis server runs in the container, and a client outside the container, connecting to the published address and port, gets the server's reply to a ping and reads back a value it stored. The value is still there after the container is restarted, because Redis wrote it to the container's persistent filesystem | A connection to a port the container does not publish is refused, because only the published port is opened |
| AC-13 | `ze` is built without the container gate | A config with a container root is refused as unknown, the container commands do not exist, and the binary links no symbol of the feature or of the upstream runtime | Every other feature behaves as in a build with the gate |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | To be filled at design | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be filled at design | | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| To be filled at design | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be filled at design | | | |

## Files to Modify
- To be filled at design.

## Files to Create
- To be filled at design.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| To be answered at design | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| To be answered at design | | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- to be filled at design.

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ze runs containers itself | Shipping the Docker daemon, as the lab Docker host image does | A second supervisor with its own mutable store and its own packet-filter rules competes with Ze's firewall and with Ze owning process lifecycle. Owner ruling |
| One binary | A runtime binary shipped beside `ze` | Owner ruling |
| Use the upstream runtime library whole, with a small build-time diff (proposed) | (a) No diff: a Go trampoline in Ze's init role that speaks the library's startup protocol. (b) Import only its leaf packages and write the parent and the init sequence in Ze | The diff is about 20 to 30 lines in two functions (the parent clones init straight into its namespaces), plus one replaced file for the system-call filter, and it fails loudly when it stops applying. (a) needs three starts of the `ze` binary per container, depends on undocumented protocol details that change with no compile error, and still needs a diff or a descriptor-passing shim for the filter. (b) makes Ze re-derive and track by hand several thousand lines of security-critical code |
| A pure-Go system-call filter, replacing the library's stub (proposed) | Running containers unfiltered; a cgo build | The stub refuses any profile. The filter is assembled with the BPF package Ze already vendors, from a per-architecture system-call table, and reproduces the upstream behaviour including "not implemented" for system calls newer than the profile |
| Images stored unpacked, one folder per image digest (proposed, from the owner's direction) | A squashfs or EROFS image file per digest | The persistent partition's filesystem already holds owners, device nodes, hard links and extended attributes. An image file needs a kernel option the runtime kernel lacks, a loop device, and a writer Ze does not have: the vendored squashfs writer holds root-owned regular files only. The cost is that layers are applied through host paths, so extraction resolves every entry under a descriptor for the folder, and that nothing verifies the folder after it is written |
| The writable layer is on disk, keyed by container name and image digest (owner direction) | A memory-backed layer discarded at restart | Owner direction: a container's filesystem persists until its image is updated. It needs a disk quota and a command that discards it |
| Operator model after the reference system, expressed in Ze's grammar (owner ruling) | Its command words verbatim | Its words break Ze's command rules (verb first, selector before action, format as a pipe). The operations and the config tree are kept |
| Container networks and published ports are container config, realised through Ze's interface and firewall mechanisms (proposed, O-7) | Leaving the operator to write a bridge and firewall terms by hand; letting the runtime install its own bridge and packet-filter rules | Operators of the reference system expect the nodes. Realising them through Ze's own mechanisms, in state Ze owns and shows, keeps the rule that nothing is installed beside Ze's firewall |
| Images are pulled only by an explicit command, and a missing image is a reported state (from the reference model) | Pulling at commit | A commit must not depend on a registry being reachable. Ze adds what the reference lacks: the container's state names the missing image, and doctor gives the command |

## Known Limitations
- To be filled at design.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
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
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit plugins` for the package tests, and the functional and QEMU suites this spec names at design. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** remove the spec's path in its bucket, in the same commit script
