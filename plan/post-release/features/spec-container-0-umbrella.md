# Spec: container-0-umbrella

| Field | Value |
|-------|-------|
| Status | ready |
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
| External-program authorisation, the shell | Authorised on 2026-11-16, scoped to a program an operator names for a shell inside a container. It is a second use of an external program, beside the entrypoint of O-4. The row in `ai/allowed-system-commands.md` lands in the commit that adds the code it names, citing this ruling (`spec-container-5-shell.md`, R-9, holds the row). |
| Ready and implementation | Approved on 2026-11-16. The children are implemented in dependency order, implementation agents on the strongest tier. |
| Feature gate default (O-5) | On in every daemon build, like every other gate. |
| Image store (O-6) | Images are stored unpacked, one folder per image digest. |
| Networks and ports (O-7) | Container networks and published ports are nodes of the container config, realised through Ze's interface and firewall mechanisms in state Ze owns and shows. |
| Shell (O-8) | A shell inside a running container is in scope now, as a child spec of this work, not a later one. |
| Writable layer (O-2) | A container's filesystem persists until its image is updated: the writable layer is on disk, keyed by container name and image digest. |
| Where containers run | On the appliance and under a daemon that runs as root, at first. A host where Ze runs as an unprivileged account, or whose data directory cannot enforce a size cap, refuses a container config and says why. |
| Persistent partition | The appliance grows its persistent partition to the disk at install. This is appliance work of its own and comes before the image store (`spec-appliance-perm-grows-at-install.md`). |
| Daemon restart | When Ze restarts, including for its own update, containers are stopped and started again. Re-adopting running containers is not part of this work. |
| Shell design | Approved on 2026-11-16: entry by extending the build-time diff; one general interactive-command registration any plugin can use, with seven new plugin-protocol messages; a live terminal on SSH and gRPC, a bounded run on REST, web and MCP. A probe decides whether the fallback entry design is needed. Under the chosen entry design a shell is refused for a container granted process tracing. Under the fallback entry design that refusal does not exist, because no host process image is then visible inside the container (`spec-container-5-shell.md`, "The fallback entry design"). |
| Container names over DNS | Not at first. A container gets the router's resolver list. |
| Published ports and the firewall | A published port must also be allowed by the operator's firewall policy. Doctor reports a published port that policy blocks. |
| Stricter than the reference model | No `privileged` node. Volumes are stored by Ze and size-capped, with no host path. |
| The list of containers | The list is named `instance`, because `name` is Ze's selector keyword. |
| Accepted defaults | The fourteen capabilities published images expect; a bridge per network; outbound translation on, with a node to turn it off; a hard CPU limit, which adds one kernel option, and no I/O throttle; the reference model's words wherever a Ze rule does not forbid them, including the workload's `environment` and short image names resolved against a default registry with the resolved digest recorded; memory 512 MiB; restart on failure; rollback by command only; health checks as a later child. |
| Storage cap (2026-11-17) | "Storage is uncapped: cap can be added post release." The cap on a writable layer and on a volume, and the refusal where the data directory cannot enforce one, no longer block anything. They are built after the release. This replaces the cap in "Where containers run" and "Stricter than the reference model" above until then. |
| The container listing (2026-11-17) | "show container list: needed." Built that day. |
| The published port's words (2026-11-17) | The router's side of a published port is `public-port` and `public-address`, beside `container-port`. The words `listen-port` and `listen-address` are gone, with no alias. |
| Closing test (2026-11-17) | Passed in the guest on the runtime kernel as `test/container/container-redis.ci`, with the deviations the operator surface child records. |

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
| The build-time diff is about 20 to 30 lines in two functions | Six patch files from the runtime child, one per hunk and 60 to 90 changed lines in all, and a seventh patch file from the shell child, which also rewrites the runtime child's file that refuses a second process. All seven are over the same five upstream files. The cgroup must exist first and init must be started inside it in one step; otherwise the container's cgroup namespace is rooted at the daemon's own cgroup and the container sees the host's cgroup tree | Runtime, Shell |
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

### Child specs

Seven specs carry the work, all in `plan/post-release/features/`. Each holds its own acceptance criteria, tests, files and phases. This spec holds no code of its own.

| # | Spec | Holds | Depends on |
|---|------|-------|------------|
| 1 | `spec-appliance-perm-grows-at-install.md` | The appliance grows its persistent partition to the disk at install, formats it so the kernel enforces a size cap, pins the kernel's quota support, and provides the one shared core leaf for project quota | - |
| 2 | `spec-container-1-runtime.md` | The upstream library as a dependency in a commit of its own, the six patch files of the build-time diff, the init role, the system-call filter, cgroup limits, supervision and restart, the kernel requirements, the feature gate with its build-tag tests, and the `container` functional suite, `test/container/`, where every child's container `.ci` tests live | - |
| 3 | `spec-container-2-image-store.md` | The registry client, the fetch command, layer application into a folder per digest, the writable layer, volumes, retention, rollback, free-space checks, and the size caps, set through the appliance spec's quota leaf | Runtime, the appliance spec |
| 4 | `spec-container-3-network.md` | Container networks as bridges Ze owns, a link whose peer end is created inside a container's namespace through the interface backend, published ports and outbound translation as firewall tables Ze owns, name resolution | Runtime |
| 5 | `spec-container-5-shell.md` | A command with an input stream on every surface, entering a running container, and the seventh patch file of the build-time diff | Runtime |
| 6 | `spec-container-4-operator-surface.md` | The one table of config words and command words, the verify rules with their messages, the state view, the listings, the doctor checks that span the children, metrics, the guide page, and the closing Redis test (AC-14) as its last phase | Runtime, image store, network |
| 7 | `spec-container-6-health-check.md` | A container health check, run through the shell child's bounded run. Ruled a later child on 2026-11-16; a skeleton today | Shell |
| 8 | `spec-container-7-followup.md` | The items found while building the work that no child above owns: CI with the seeded image, a plugin command's time bound, a sibling's read of the applied ruleset, and the smaller ones its Task table lists. Added on 2026-11-17; a skeleton today | Image store, network |

Dependency order:

| Step | Work | Why here |
|------|------|----------|
| 0 | The design of the operator surface child is settled | It holds the one table of config words and command words, so no sibling starts its wiring phase before it. No sibling lists it in its own Depends row: a sibling reads its tables and waits for none of its code |
| 1 | The appliance spec and the runtime child, independent of each other | Neither needs the other. The runtime child lands the plugin, the gate and the functional suite every sibling adds to |
| 2 | The image store child and the network child | Each fills a seam the runtime child provides. The image store child's capped storage also needs the appliance spec |
| 3 | The shell child | It needs a running container, and it rewrites one patch file of the runtime child's series |
| 4 | The implementation phases of the operator surface child | They join the facts of its three siblings. The shell child does not gate them |
| 5 | The closing Redis test, the last phase of the operator surface child | It is one path through every child (AC-14) |
| Later | The health check child | It uses the shell child's bounded run |

### Shape (approved on 2026-11-16)

Ze fetches an image by digest, verifies it, and stores it unpacked, in one
folder per image digest on persistent storage (O-6). A container's root is an
overlay of that folder under a size-capped writable layer kept on disk (O-2).
`ze` starts the container by re-executing itself in an init role inside fresh
namespaces; that process prepares the root, confines itself and then executes
the image's entrypoint. Ze supervises the container with a supervisor of its
own and kills it through its cgroup (Corrections). The work is split into the
specs of "Child specs" above.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every subsystem shares
  → Constraint: a compile tag is a boundary. Always-on code never imports a gated package, and reaches it only through a registry or a seam the feature fills from its own `init()`
  → Constraint: every plugin also runs as a separate process, so the feature reaches interfaces and the firewall through the bus, a typed request or a command, never a Go call into another component's state
  → Constraint: a config a backend cannot apply exactly is refused at verify, never approximated
- [ ] `docs/architecture/plugin/feature-gates.md` - how a feature compiles out
  → Decision: the feature is a self-registering edge plugin, so its gate is lines in `feature-gates.txt` plus a present and an absent build-tag test. No build-matrix row is added
  → Constraint: a shared helper that must compile out stays inside the gated package; a core leaf cannot be gated
- [ ] `docs/architecture/container/runtime.md`, `ai/rules/plugins.md` - the pages the `// Design:` headers of `internal/plugins/container` and `cmd/ze/hub/service_registry.go` name
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
  → Constraint: its namespace entry and its system-call filter are C. With cgo off, the library cannot create a container, and a container with a filter profile is refused. Read from build constraints and imports, and confirmed by a type-check of that tag with cgo off on 2026-11-16 (A-1)
  → Decision: its init sequence, default mounts, masked and read-only paths and its fixes for published vulnerabilities are the checklist Ze's runtime is written and tested against
- [ ] The Go standard library, `syscall.SysProcAttr` - what a pure-Go parent can do
  → Decision: the parent can start a child in new namespaces with identity mappings, directly inside a cgroup, with a death signal. It cannot run code between fork and exec, so root setup, capability drop and the filter run in a re-executed init role

**Key insights:** (minimal context to resume after compaction)
- The upstream runtime's main library does not import its C code. With cgo off, only its namespace-entry package and its system-call filter are missing, and the library reaches both through a small startup protocol and one function. Ze uses the library whole behind a build-time diff of 60 to 90 changed lines: six patch files from the runtime child and a seventh from the shell child, over the same five upstream files. The filter is Ze code in the plugin, behind a registration seam the diff adds.
- The library fixes the init command to the running binary with the argument `init`, and Ze already has an `init` command. The container init role is therefore selected by the library's own environment key, before command dispatch and before crash logging starts.
- The library creates a container and leaves it blocked before its entrypoint runs. In that window the network namespace exists and the init process is known, so Ze's interface backend creates the container's end of its link directly inside that namespace, through the netlink library, and addresses it there. No link is moved, no Ze thread enters the namespace, and the library's own network-device feature is not used.
- The library performs the final exec itself, inside the init role, where Ze's crash-log pipe was never set up. Ze's execve wrapper does not apply to it.
- The reference operator model pulls an image only on an explicit command, and a commit naming an absent image succeeds while that container does not run. Operators of that system report this as its main trap, so Ze reports it in doctor with the command that fixes it.
- The runtime kernel already has namespaces, cgroup v2 (memory, pids, cpuset, devices), overlayfs, the system-call filter, veth and loop devices. It lacks: extended attributes in squashfs, a hard CPU limit, an I/O throttle, verified-read integrity, EROFS and AppArmor.
- The squashfs writer Ze already vendors writes root-owned regular files, directories and symlinks only. It cannot hold a general image (no owners, device nodes, hard links or extended attributes).
- The persistent partition is about 950 MB on the default 2 GiB image. It does not grow at install today; the appliance spec adds the growth, and comes before the image store.
- Ze has three process supervisors and none is shared. No piece of the plugin supervisor is reusable as code for a container: the library forks init itself, and a group kill cannot reach a container in a session of its own. A container is killed through its cgroup.
- The interface component creates a veth pair in the host namespace only, and nothing moves a link into another namespace. The container's link needs no move: the backend gains creating a link whose peer end the netlink library makes directly inside the container's namespace.
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
- [ ] The source files of the other mechanisms this design extends were read by the child that extends each one, and are listed in that child's Current Behavior section.

**Behavior to preserve:**
- Every existing config root, command, doctor code, vendor patch, firewall owner's tables and config-declared interface behaves as today, and no file of the plugin supervisor is edited. Each child's Current Behavior section lists what it preserves.
- A build without the container gate behaves as today. The always-on additions are the ones the children name: one early-role dispatch call (runtime), the data directory resolver and the exported stall reader (image store), the interface and firewall requests with the local-address firewall match (network), the interactive-command registry with its protocol messages (shell), and the three core leaves, the storage report and the grow command of the appliance spec.

**Behavior to change:**
- None. This is a new feature behind a compile-time gate.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Feature-gate manifest and its generator | `feature-gates.txt`; `internal/le/repo/featuretags/daemontags.go` `DaemonTags`; `docs/architecture/plugin/feature-gates.md` | One line per gated package under a `ze_<feature>` tag. `DaemonTags` returns every declared gate unfiltered, so every gate is on in the distribution and appliance builds. `./le repo update` regenerates the derived tag lists | Extends: the container packages are new lines under one new tag. A gate that is declared and off by default does not exist, and none is built: the container gate is on in every daemon build (O-5) |
| Absent and present build-tag tests | `cmd/ze/hub/build_tag_vpp_absent_test.go` and its siblings; `./le arch tier check` | Prove a gated plugin is not registered, its config root is refused as unknown, and no always-on package imports it | Extends: one pair for the new tag |
| External plugin process supervisor | `internal/component/plugin/sysproc_linux.go` `NewSysProcAttr`, `StartChild`; `internal/component/plugin/process` `startExternal`, `ProcessManager.Respawn` | Forks the operator's plugin in its own process group with a death signal bound to one long-lived forking thread, relays its stderr to the log, stops it with a grace then a group kill that cannot hit a recycled pid, and respawns up to 5 times per 60 s and 20 in total | Does not apply as code (Corrections): the upstream library forks init itself with no death signal, and a group kill cannot reach a container in a session of its own, so a container is killed through its cgroup. The pipe-as-file shape of the stderr relay is the model for capturing a container's output. The connect-back and the five startup stages do not apply: a container is not a plugin (`spec-container-1-runtime.md`) |
| VPP process supervisor | `internal/component/vpp/vpp.go` `VPPManager.Run`, `runOnce` | Runs VPP with a 1 to 30 s doubling restart backoff and publishes connected and disconnected events. It sets no process attributes and stops by kill | Model only, for the backoff loop and the lifecycle events. Not extended: it has no process group, death signal or graceful stop |
| Self re-execution by role | `cmd/ze/dispatch.go` `dispatchMain`; `internal/core/seedinit/register.go` `Dispatch`; `internal/core/crashlog/exec_unix.go` `Exec` | `ze` selects a role from its first argument, or from an environment key before startup. Every execve in Ze goes through one wrapper, and a raw one is refused by a lint rule | Extends: the container init is one more early role, selected by an environment key before startup as the seed role is. The execve wrapper does not apply: the upstream library performs the final exec inside that role, where the crash-log pipe the wrapper protects was never created. No role today runs inside new namespaces |
| Locked-thread namespace entry | `internal/component/kernelcap/namespace_linux.go` `InThrowawayNetworkNamespace` | Unshares a network namespace on a thread that is locked and never unlocked, for kernel probes | Extends for the network child's kernel probes and for the shell child's namespace join. Does not apply to the container's link (Corrections): the netlink library creates the peer end directly inside the namespace and configures it through a socket bound there, so no Ze thread enters the container's namespace for it |
| Interface veth and bridge | `internal/component/iface/yang/ze-iface-conf.yang` `list veth`, `list bridge`; `internal/component/iface/backend.go` `CreateVeth`; `internal/plugins/iface/netlink/manage_linux.go` | Creates a veth pair in the host namespace with both ends up. The managed end takes addresses and units; the peer end takes none. Nothing moves a link into another namespace | Extends (Corrections): a container network is a bridge Ze owns, and the host end of a container's link is an ordinary Ze interface that is a member of it. The interface backend gains creating an owned bridge, and creating a link whose peer end is made directly inside a named namespace and addressed there. Both are reached through the interface component's boundary and never a direct call (`spec-container-3-network.md`) |
| Interface VRF leaf | `internal/component/iface/yang/ze-iface-conf.yang` `leaf vrf`, validator `unimplemented-vrf` | Refuses any config that sets it | Does not apply until VRF support ships (`plan/post-release/features/spec-vrf-0-umbrella.md`) |
| Firewall match by interface | `internal/component/firewall/yang/ze-firewall-conf.yang` `input-interface`, `output-interface` | Matches a rule by interface name, with a prefix wildcard, in tables Ze owns by the `ze_` prefix | Extends (Corrections): with a bridge per network, routed traffic enters by the bridge, so a term matches the network's bridge, and one container by its address. Publishing a port adds one general match to the firewall model, "the destination is one of this router's addresses", so a published port does not capture transit traffic (`spec-container-3-network.md`) |
| URL fetch and digest check | `internal/core/fetch/scheme.go` `schemes`; `ToFile`, `ToDisk`, `SameOrigin`, `CheckSHA256` | Fetches http, https and tftp to a file or a device, with an optional SHA-256, three retries from byte zero, and a refusal of any redirect that leaves the origin. No authentication header, no cancellation, no resume | Extends in part: the digest helpers and the stall timeout are reused. A registry pull needs a token exchange, content negotiation and a blob redirect to another host, which `SameOrigin` refuses by design, so the registry client is new code beside it |
| System update backend | `internal/component/config/system/backend.go` `UpdateBackend`; `selfupdate.go` | One update backend per platform, a singleton. Self-update keeps one previous binary, rolls back by hand, and keeps its history in the state store. On the appliance the A/B switch and test boot belong to gokrazy and the build host | Model, not extended: its registry is a system singleton. The verbs (check, download, apply, rollback, history), the keep-one-previous step and history in the state store carry over. Automatic rollback after a failed start exists nowhere in Ze and is not built here: rollback is by command only (Accepted defaults) |
| Runtime state store and its gate | `internal/core/statestore`; `pkg/zefs/keys.go`; `internal/le/arch/fspersistence/fspersistence.go` `fileAllowlist` | Small runtime state under registered keys. A raw file write under the config or state directory is refused unless its file is allowlisted with a reason, as the self-update binary is | Extends for per-container metadata (current digest, previous digest, history). Does not apply to image bytes and volumes: they are artifacts, not state, and need allowlist entries and a data area of their own |
| Free-space helpers and the storage doctor check | `internal/core/diskspace`; `internal/component/doctor/checks_storage.go` | Reads free space; self-update requires twice the binary size; doctor warns when the config partition is nearly full | Extends: the helpers are reused before a fetch. Per-consumer accounting does not exist and is new |
| Doctor checks and diagnostic codes | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck`; `docs/architecture/doctor-and-health-checks.md` | A check registers from the package that owns the dependency; live state is answered by the plugin itself | Extends: one check per dependency and problem class the feature adds |
| Kernel requirement enrolment | `internal/component/kernelcap/kernelcap.go` `MustRegister`; `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`; `internal/appliance/kernelreq.go` `enforceKernelRequirements` | A feature declares a kernel option once. The kernel build refuses without it, and `ze` refuses at start, reload and validate when a config uses the feature on a kernel that lacks it, and reports it in doctor | Extends: the container feature enrols each option it needs, including the ones that are on today only by upstream default and so are not protected |
| cgroup v2 mount and its locator | gokrazy init mounts it at boot; `internal/component/ike/dataplane/override_guard_linux.go` `cgroup2Root` | The appliance mounts cgroup v2 and places no process in a child group. The only Ze code that uses a cgroup finds the mount from the mount table, because a distribution host may mount it elsewhere | Does not apply (Corrections): the upstream library accepts one mount point only, so a host with cgroup v2 mounted elsewhere is refused with a doctor finding, and nothing moves to a core leaf. Creating, limiting and removing a cgroup is new |
| Overlay over the read-only root | `internal/appliance/dockerhost/boot.go` `writableOverlays` | In the lab Docker host image, mounts an overlay with the read-only system image below and a memory filesystem above | Model: the same mount shape is the container root. Not extended as code, because it lives in another program |
| Register of external programs Ze may run | `ai/allowed-system-commands.md`; `ai/rules/go-standards.md` | Lists every program outside Ze that shipped code may execute. The default is no, and only the owner adds a row | Extends, by the owner: the row for a container's entrypoint was authorised on 2026-11-16 (O-4) and lands in the commit that adds the code it names. A program an operator names for a shell is a second use, authorised on 2026-11-16, and its row lands with the shell child's code (Owner decisions) |

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
5. In that window the plugin asks the interface component for the container's link: the host end is a member of the network's bridge, and the peer end is created directly inside the new network namespace and addressed there. The firewall state for published ports is not part of a start: it follows config changes (Corrections).
6. The plugin releases the container. The init role installs the system-call filter and executes the entrypoint.
7. The supervisor waits on the container, records its exit, and applies the restart policy.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to plugin | The config transaction and registered commands. The plugin must also work when run as a separate process | No |
| Plugin to interface component | A typed request or a command, never a Go call, for creating the network's bridge and the container's link. The backend creates the link's peer end directly inside the container's namespace; nothing is moved | No |
| Plugin to firewall component | A declared contribution of firewall state in tables Ze owns. The firewall's table registry is process-local today, so a plugin that can run forked needs a request, not a direct registration | No |
| Daemon to container init | The upstream library's startup protocol over inherited descriptors, inside one binary | No |
| Ze to registry | The registry pull protocol over HTTPS | No |
| Ze to kernel | Namespaces, cgroup files, mounts, the system-call filter, netlink sockets | No |

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
- The plugin process supervisor: not an integration point. None of its pieces is reused as code and none of its files is edited; the container plugin has a supervisor of its own and kills a container through its cgroup.
- The interface backend: gains creating an owned bridge, and creating a link whose peer end is made directly inside a named namespace and addressed there.
- The firewall component: gains a way for a plugin to contribute state for published ports and outbound translation, and one general match, "the destination is one of this router's addresses", which publishing a port needs.
- The kernel capability registry and the two kernel option files: each required option is enrolled.
- The doctor registry: one check per dependency and problem class.
- The state store: per-container metadata.
- The allowed-external-programs register: one row for a container's entrypoint, authorised by the owner (O-4). A program named for a shell is a second use, authorised by the owner on 2026-11-16; its row lands with the shell child's code.

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
| A-2 | The runtime kernel can mount a filesystem image file read-only | The resolved config of the current amd64 runtime kernel has the loop device and squashfs built in. Both come from upstream defaults and are not pinned. No arm64 config was read | The image store needs another form, such as an unpacked directory tree | The resolved arm64 config, and a listing of the loop control device on a booted appliance | confirmed on amd64, open on arm64. It carries no weight: images are stored unpacked (O-6), so no image file is mounted |
| A-3 | The Go standard library can start a child in new namespaces, with identity mappings, directly inside a cgroup, without C | Read in the standard library source | The pure-Go runtime is not possible as described | Done | confirmed |
| A-4 | A root filesystem with no write support prevents a container from overwriting the `ze` binary through its own process image | Inference. The upstream source says only that a read-only bind mount was unsafe because a privileged container could clear the flag | The appliance needs the sealed-copy defence too | A test that attempts the overwrite from inside a container on the appliance. The design seals the binary on every platform regardless, so this assumption carries no weight | unvalidated |
| A-5 | `ze`, running as root under gokrazy's init in the root cgroup, can create child cgroups and enable their controllers | The mount is made by gokrazy init, and the controllers are listed on a host running the same kernel | The runtime cannot limit a container on the appliance | Reading the controller files in a booted appliance guest | unvalidated |
| A-6 | The persistent partition can hold images and volumes | Default image size and partition layout read from source: about 950 MB by arithmetic. No code was found that grows the partition at install | The feature needs a larger default image or a grow step first | A full read of the installer's disk code, done by the appliance spec: the partition does not grow at install (Corrections). `spec-appliance-perm-grows-at-install.md` adds the grow step and comes before the image store | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Ze owns a confinement surface that a mature runtime hardened over years, and misses a step | A review against the upstream runtime's init sequence finds an absent step | Derive the sequence from the upstream source as a checklist, one test per step |
| R-2 | A container exhausts memory, CPU, processes or disk and takes the routing daemon with it | A container without limits is accepted by config | Limits are mandatory with defaults, and the writable layer and volumes are size-capped. A hard CPU limit needs a kernel option the runtime kernel lacks today |
| R-3 | A malicious image, or a running container sharing a volume, redirects a mount or a write the runtime makes inside the root (a symlink at a mount target, a replaced device node) | A test image with a symlink at a mount target starts | Resolve every target under a descriptor for the root, verify the type of what was opened, and mount through descriptors |
| R-4 | The daemon leaks an open descriptor into a container, which uses it to reach the host filesystem | A container lists a descriptor above standard error at its entrypoint | Close every descriptor before starting init and again immediately before the final exec, and verify the working directory |
| R-5 | A container overwrites the `ze` binary through its own process image | The overwrite attempt succeeds in a test on a writable host root | Run the init role from a sealed copy of the binary on every platform |
| R-6 | An image layer writes outside its tree, plants a device node or setuid file, or expands without bound while Ze turns it into a root | A crafted layer changes a host file or fills the disk | Images are stored unpacked (O-6), so layers are applied through host paths: resolve every entry under a descriptor for the image's folder, cap the expanded size, and refuse entry types the config did not allow |
| R-7 | The feature's kernel options are present today only by upstream default and disappear on a kernel bump | A kernel build succeeds without one | Request and pin each option, and enrol it as a kernel capability so `ze` refuses at validate, start and reload |
| R-8 | The squashfs format, as this kernel builds it, drops extended attributes, so file capabilities in an image are lost and programs that rely on them fail | A container program that needs a file capability fails to start | Avoided by storing images unpacked on the persistent partition (O-6). It returns if an image file is chosen |
| R-9 | An upstream release changes one of the five upstream files the seven patch files of the build-time diff touch (60 to 90 changed lines from the runtime child, plus the shell child's patch), and a patch applies cleanly but is now wrong. The two files first studied had 32 and 25 commits in two years | A container fails to start after an upstream version bump | Pin the upstream version, and gate every bump on the functional tests that start a container. Upstream states that its library API is not covered by its compatibility promise |
| R-10 | The upstream library links code Ze never runs: the systemd cgroup driver and its bus client, and checkpoint support | Binary growth and new modules in the dependency list | The diff removes checkpoint support, because no Ze build can carry a tag that is not a Ze gate, and the runtime child's hunks also remove the systemd driver and the cgroup v1 manager from the link (its R-10). Its present build-tag test asserts that the binary holds no symbol of them. About 13 modules are new to Ze |
| R-11 | Each container start copies the whole `ze` binary into memory to seal it, when the zero-copy sealing the library tries first is unavailable | Memory use rises by the binary's size at each start | Confirm on the appliance that the zero-copy sealing works on its root filesystem; it needs overlayfs and the new mount interface, which the kernel has |
| R-12 | The upstream library marks every open descriptor of the calling process close-on-exec before it starts init, and Ze is a long-lived daemon with many descriptors | A child process Ze starts afterwards loses a descriptor it was meant to inherit | Go opens descriptors close-on-exec already and passes inherited ones explicitly. A test starts a plugin process after a container has started |
| R-13 | A parent death signal on the container init kills it at once under the diff, because init sees no parent from inside its new process namespace | Every container exits immediately when the option is set | The plugin never sets the option, and the diff refuses it |
| R-14 | The persistent partition fills: images, writable layers and volumes share about 950 MB with the config and crash logs | A fetch or a container write fails for lack of space | The appliance spec grows the persistent partition to the disk at install. A free-space check before a fetch, the retention rule, and a project-quota cap on each writable layer and volume, which needs the format change and the kernel pin of the appliance spec (Corrections) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | This spec holds no code. Each child states its own worst case: a container that escapes its confinement or starves the routing daemon (runtime), a registry that changes a host file as root or a container that fills the persistent partition (image store), a published port that captures traffic meant for the router (network), a wrong word that becomes a contract in the field (operator surface), a container escape through the entering process (shell), an install that does not boot (the appliance spec) |
| How is it reverted? | Remove the gate's manifest lines and regenerate: the plugin leaves every build (the runtime child's Blast Radius). Each child names what its own revert leaves behind |
| Who else touches this path? | The seven specs of "Child specs". Outside them: `plan/post-release/features/spec-vrf-0-umbrella.md` (a container network in a VRF), `plan/post-release/features/spec-streaming-answer-protocol.md` (the surface adapters the shell child extends; how every surface sends the typed line and applies its pipe chain, landed by the closed spec-pipe-chain-honored-on-every-command-surface, is `docs/architecture/api/commands.md`), `plan/release/fixes/spec-crash-capture.md` and `plan/post-release/features/spec-install-9-cloud-init.md` (the persistent partition) |

## Wiring Test (MANDATORY -- NOT deferrable)

This spec holds no code, so each entry point is wired by the child that owns it. The rows below name one wiring test of each child; its own Wiring Test table holds the rest.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A `container` root in a committed config | → | The plugin's config callbacks and one supervisor worker (runtime child) | `container-start` |
| A build with and without `ze_container` | → | The gate (runtime child) | `TestBuildTag_Container_Present`, `TestBuildTag_Container_Absent` |
| `create container image reference` | → | The fetch, the registry client, layer application, publication (image store child) | `container-image-fetch-lists-digest` |
| A config with a network and an attached container | → | The plugin's verify and apply, the runtime seam, the link (network child) | `container-network-reach-gateway` |
| `show container list` over SSH | → | The state view and the list handler (operator surface child) | `container-list-running` |
| `request container shell name` over SSH with a terminal | → | The session route and the library's second-process start (shell child) | `container-shell-ssh-terminal` |
| The installer booted with the HTTP source | → | The table growth and the filesystem resize (the appliance spec) | `TestRunHTTPGrowsPersistentPartition` |
| The owner's closing test | → | Every child, through the operator's own entry points | `container-redis` |

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
| AC-9 | A container publishes a port, and the operator's firewall policy allows it | A connection to an address of the router on the published port reaches the container, and the translation is visible as state Ze owns and shows. Where the policy drops it, doctor reports `doctor-container-port-blocked` with the term to add | A port no container publishes stays closed, and a connection through the router to another host's same port is forwarded untranslated, because only traffic addressed to the router is published |
| AC-10 | A container's entrypoint exits with a failure | The container is restarted under its restart policy with a growing delay, and the listing shows the restart count and the last exit status | A container whose policy is "no" stays stopped, as configured |
| AC-11 | The daemon stops or reloads a config that removes a container | The container gets its stop signal, then is killed after its stop timeout, which is at most 30 seconds, and no process, cgroup, mount or link of it remains | Its image and its writable layer remain on disk, so the next start finds them |
| AC-12 | A config uses a container on a kernel that lacks a required option | Validate, start and reload refuse, naming the option; doctor reports the same | A config with no container is accepted on that kernel, because nothing needs the option |
| AC-14 | Owner's closing test: the operator fetches the published Redis image, configures a container for it on a container network with a published port, allows that port in the firewall policy, and commits. The config declares no volume | The Redis server runs in the container, and a client beyond the router, in a separate network namespace joined to the router by a link as an outside host would be, connecting to the router's address and the published port, gets the server's reply to a ping and reads back a value it stored. The connection is forwarded, so the firewall policy term is what lets it through: with that term removed the client cannot connect and doctor reports the published port as blocked by policy. The value is still there after the container is restarted, because Redis wrote it to the container's persistent filesystem: the client asked for an explicit save, and the image's declared data path, with no configured volume there, is a directory of the writable layer. The test is the last phase of the operator surface child (its AC-15 and section X-1), which names the criterion of each child it rests on | A connection to a port the container does not publish is refused, because only the published port is opened |
| AC-13 | `ze` is built without the container gate | A config with a container root is refused as unknown, the container commands do not exist, and the binary links no symbol of the feature or of the upstream runtime | Every other feature behaves as in a build with the gate |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a Redis server on the router, uses it from another host, restarts it and finds the data (AC-14) | fetch -> store -> config -> verifier -> runtime start -> link and translation -> firewall policy -> client -> restart -> writable layer | `container-redis` (operator surface child, story 7) |
| 2 | Commits a config with one container | config delivery -> description -> store resolve -> root overlay -> library create -> created call -> release | `container-start` (runtime child, story 1) |
| 3 | Fetches an image and sees it listed | command -> fetch worker -> registry client -> layer application -> publication -> listing | `container-image-fetch-lists-digest` (image store child, story 1) |
| 4 | Declares a network, attaches a container and publishes a port | config -> plugin verify -> interface request -> bridge and link; table builder -> firewall request | `container-network-reach-gateway`, `container-port-published` (network child, stories 1 and 2) |
| 5 | Opens a shell in a running container | SSH exec with a terminal -> session route -> container plugin -> second-process start | `container-shell-ssh-terminal` (shell child, story 1) |
| 6 | Installs an appliance on a large disk and finds the space | installer -> table rewrite -> resize -> `show host storage` | `./le test qemu installer` (the appliance spec, story 1) |

## 🧪 TDD Test Plan

### Unit Tests
This spec holds no code, so every unit test is in the TDD Test Plan of the child that owns the code. The rows below are the tests this spec's own criteria name.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTag_Container_Present`, `TestBuildTag_Container_Absent` | `cmd/ze/hub/build_tag_container_present_test.go`, `cmd/ze/hub/build_tag_container_absent_test.go` (runtime child) | AC-13: the gate, present and absent | |
| `TestContainerRootRefusedWithoutTheGate` | The same absent test file (operator surface child) | AC-13: the refusal text of a `container` root, and the unknown command | |

### Boundary Tests (numeric inputs)
Every numeric input is a node of the operator surface child's tables S-1 to S-3, and its Boundary Tests table holds each range. The two rows below are the bounds this spec's corrections and defaults set.

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `stop-timeout`, seconds | 1-30 | 30 | 0 | 31 |
| `memory`, mebibytes. The default is 512, and the floor waits for the runtime child's measurement | 64-1048576 | 1048576 | 63 | 1048577 |

### Functional Tests
Every container `.ci` test lives in the `container` suite, `test/container/`, which the runtime child registers. A test that boots an appliance image is in the appliance suite. Each child's Functional Tests table holds its own.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `container-start` | `test/container/container-start.ci` (runtime child) | AC-1: a committed container runs | |
| `container-redis` | `test/appliance/container-redis.ci` (operator surface child) | AC-14: the closing Redis test | |

## Files to Modify
- None by this spec. Each child lists its own. The existing files more than one child edits follow
- `feature-gates.txt` - the runtime child declares the gate `ze_container` and its own packages. Each sibling adds the line of each package it creates
- `pkg/zefs/keys.go` - the state keys of the runtime, image store and network children
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - the options of the runtime and network children, and the quota options of the appliance spec

## Files to Create
- None by this spec. Each child lists its own. The new files more than one child edits, and who creates them, follow
- `internal/plugins/container/register.go`, `internal/plugins/container/config.go`, `internal/plugins/container/yang/ze-container-conf.yang`, `internal/plugins/container/yang/ze-container-cmd.yang` - created by the runtime child, extended by every sibling with the operator surface child's words
- `cmd/ze/hub/build_tag_container_present_test.go`, `cmd/ze/hub/build_tag_container_absent_test.go` - created by the runtime child. The absent test is extended by the operator surface and shell children
- The seven patch files of the build-time diff - six from the runtime child, a seventh from the shell child
- `test/container/container-start.ci` (runtime child), `test/appliance/container-redis.ci` and `docs/guide/containers.md` (operator surface child) - the files this spec's tables cite
- `plan/post-release/features/spec-container-6-health-check.md` - the later child, a skeleton

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The plugin's `yang` folder. The words are the operator surface child's tables S-1 to S-3 and C-1, and each child lands its rows |
| YANG validation constraints | Yes | The operator surface child's tables |
| YANG custom validators | No | A gated root cannot use `ze:validate`. The cross-leaf rules are the operator surface child's table V-1, judged by the plugin's one function |
| CLI commands/flags | Yes | The operator surface child's table C-1. The grow command is the appliance spec's |
| CLI grammar (keyword before value) | Yes | The operator surface child's table C-1, which lists the shell child's three commands too |
| Editor autocomplete | Yes | Each child's own row |
| Functional test for new RPC/API | Yes | The `container` suite, `test/container/` |
| Pipe completeness | Yes | The operator surface child, "Pipes" |
| Env var registration | Yes | `ze.data.dir` (image store child) |
| Doctor check for runtime dependencies | Yes | Each child's own table: runtime "Problem classes and their doctor codes", image store "Doctor checks and codes", network "Problem classes", operator surface D-1, the shell child's Integration Checklist, the appliance spec's storage check |
| Doctor code per problem class | Yes | The same tables. Every container code starts with `doctor-container-` |
| Prometheus counters/metrics | Yes | The operator surface child's table M-1, the runtime child's out-of-memory counter, the shell child's session metrics |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
This spec changes no page of its own. Each row names the child whose checklist holds the page.

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The container feature declaration: the runtime child creates it and each sibling extends it |
| 2 | Config syntax changed? | Yes | The operator surface child: the `container` root. The network child: the local-address firewall match |
| 3 | CLI command added/changed? | Yes | The operator surface child, and the appliance spec for the grow command |
| 4 | API/RPC added/changed? | Yes | The network child (the interface and firewall requests) and the shell child (a command that holds a session) |
| 5 | Plugin added/changed? | Yes | The runtime child |
| 6 | Has a user guide page? | Yes | The operator surface child: `docs/guide/containers.md` |
| 7 | Wire format changed? | No | No protocol wire format changes |
| 8 | Plugin SDK/protocol changed? | Yes | The network child and the shell child |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC |
| 10 | Test infrastructure changed? | Yes | The runtime child: the `container` suite. The image store child: the registry fixture and the Redis seed |
| 11 | Affects daemon comparison? | Yes | The operator surface child |
| 12 | Internal architecture changed? | Yes | Each child's own design page for the container work, and the appliance pages of the appliance spec |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The operator surface, runtime and shell children |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Each child's own row |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/zefs-format.md`, declared by `pkg/zefs/keys.go`: the runtime, image store and network children each add their keys to its key registry. The rest is derived by each child with `./le spec citation anchors` on its own spec |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The operator surface child (`docs/guide/docker.md`) and the network child (`docs/guide/firewall.md`) |

## Implementation Steps

This spec has no phase of its own. The work runs in the dependency order of "Child specs", and each step below is the phases of the specs it names.

1. **Phase: Wiring (MANDATORY FIRST)** -- the runtime child's phase 1, after the design of the operator surface child is settled: the gate, the plugin skeleton, the seams and the `container` suite
   - Tests: the runtime child's Wiring Test table
2. **Phase: the appliance spec and the runtime child** -- independent of each other
3. **Phase: the image store child and the network child**
4. **Phase: the shell child**
5. **Phase: the operator surface child** -- its last phase is the closing Redis test, `container-redis` (AC-14)

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ze runs containers itself | Shipping the Docker daemon, as the lab Docker host image does | A second supervisor with its own mutable store and its own packet-filter rules competes with Ze's firewall and with Ze owning process lifecycle. Owner ruling |
| One binary | A runtime binary shipped beside `ze` | Owner ruling |
| Use the upstream runtime library whole, with a small build-time diff (approved on 2026-11-16) | (a) No diff: a Go trampoline in Ze's init role that speaks the library's startup protocol. (b) Import only its leaf packages and write the parent and the init sequence in Ze | The diff is 60 to 90 changed lines in six patch files from the runtime child, plus a seventh patch file from the shell child, all over the same five upstream files (Corrections). The parent clones init straight into its namespaces, inside a cgroup that exists first; one hunk turns the system-call filter stub into a registration seam; one removes checkpoint support. The diff fails loudly when it stops applying. (a) needs three starts of the `ze` binary per container, depends on undocumented protocol details that change with no compile error, and still needs a diff or a descriptor-passing shim for the filter. (b) makes Ze re-derive and track by hand several thousand lines of security-critical code |
| A pure-Go system-call filter, as Ze code in the plugin behind a registration seam the diff adds in place of the library's stub (approved with the design) | Running containers unfiltered; a cgo build | The stub refuses any profile. As plugin code the filter is judged by Ze's gates. It is assembled with the BPF package Ze already vendors, from a per-architecture system-call table, and reproduces the upstream behaviour including "not implemented" for system calls newer than the profile |
| Images stored unpacked, one folder per image digest (owner ruling, O-6) | A squashfs or EROFS image file per digest | The persistent partition's filesystem already holds owners, device nodes, hard links and extended attributes. An image file needs a kernel option the runtime kernel lacks, a loop device, and a writer Ze does not have: the vendored squashfs writer holds root-owned regular files only. The cost is that layers are applied through host paths, so extraction resolves every entry under a descriptor for the folder, and that nothing verifies the folder after it is written |
| The writable layer is on disk, keyed by container name and image digest (owner direction) | A memory-backed layer discarded at restart | Owner direction: a container's filesystem persists until its image is updated. It needs a size cap, which is a project quota the partition's format and the kernel do not provide today (Corrections, the appliance spec), and a command that discards it |
| Operator model after the reference system, expressed in Ze's grammar (owner ruling) | Its command words verbatim | Its words break Ze's command rules (verb first, selector before action, format as a pipe). The operations and the config tree are kept |
| Container networks and published ports are container config, realised through Ze's interface and firewall mechanisms (owner ruling, O-7) | Leaving the operator to write a bridge and firewall terms by hand; letting the runtime install its own bridge and packet-filter rules | Operators of the reference system expect the nodes. Realising them through Ze's own mechanisms, in state Ze owns and shows, keeps the rule that nothing is installed beside Ze's firewall |
| Images are pulled only by an explicit command, and a missing image is a reported state (from the reference model) | Pulling at commit | A commit must not depend on a registry being reachable. Ze adds what the reference lacks: the container's state names the missing image, and doctor gives the command |

## Known Limitations
Each child lists its own. The ones that bound the whole work:
- Containers run on the appliance and under a daemon that runs as root, at first. Elsewhere a container config is refused with the reason (owner ruling).
- When Ze restarts, including for its own update, containers are stopped and started again (owner ruling).
- A container network cannot be placed in a VRF until VRF support ships. `plan/post-release/features/spec-vrf-0-umbrella.md` adds the node then.
- A container health check is not built by the six specs above it in "Child specs". It is owed to `plan/post-release/features/spec-container-6-health-check.md`.
- Ze answers no container names over DNS at first, has no I/O throttle, builds and signs no image, and shares no layer between images (owner rulings, and the image store child).
- A live terminal in the browser and a live session over REST are not built: both surfaces run the bounded form (the shell child, O-4 and O-5).

## Remaining work (2026-11-17)

This is the one list of what the container work still owes, written after the closing test passed. Each row names the spec that owns the item and the place in it. An item is specified once, in its owner: this table points and never restates. The state is judged from the code and the tests at worktree commit `d3c799f3ec`, not from spec text.

Abbreviations: S1 runtime, S2 image store, S3 network, S4 operator surface, S5 shell, S6 health check, S7 followup (`spec-container-7-followup.md`), SA `spec-appliance-perm-grows-at-install.md`.

### What an operator would miss first

| Item | Owner and place | State | Remains |
|------|-----------------|-------|---------|
| Reading a container's output: `show container log`, `monitor container log`, the `log` leaf | S4 table C-1 and C-2, AC-10, phase 5; the buffer and the leaf are S1 phase 7, AC-16 | The buffer exists with a sequence cursor. Output reaches an operator only through the stopped finding | Both commands, the stream, `lines` and `after`, the `log` leaf, the refusal for `log none` |
| Restart policy, exit record, `restart-count` and `last-exit-status` in the listing | S1 phase 7, AC-10, AC-20; the columns are S4 table T-1, AC-1, AC-3, phase 4 | The listing has six columns and leaves these two out on purpose. Every exit ends as `stopped` | The `restart` leaf with its delay, the exit record, then the two columns |
| `show container detail name` | S4 table C-1, phase 4 | Not started | The command and its test |
| Volumes and `tmpfs` | Words: S4 table S-1, rules V-3 and V-7. Storage: S2 "Capped storage", phases 6 and 7, AC-14, AC-15, AC-24. Mounts: S1 phase 6, AC-9 | A request with volumes is refused. The two volume commands answer not-implemented | All of it |
| Override words: `entrypoint`, `argument`, `environment`, `hostname`, `user`, `group` | S4 table S-1; S1 phase 6, AC-8, AC-21 | The image's own entrypoint and environment are used. A user named by name is refused | Every word, and the user by name |
| Limits: `memory`, `shared-memory`, `cpu`, `process-limit` | S4 table S-1; S1 phase 5, AC-3, AC-4, AC-5 | Fixed defaults of 512 MiB and 256 processes. No CPU quota | The four leaves, the CPU quota, the three limit tests |
| Grants: `capability`, `capability-drop`, `device`, `sysctl`, `host-namespace` | S4 table S-1, AC-6, AC-19; S1 phase 6 | Thirteen capabilities, the device-node one withheld until the device allowlist exists | Every word, the allowlist |
| Image lifecycle: update, rollback, delete, delete all, clear, retention, a hold that refuses | S2 "Generations", phase 7, AC-11 to AC-14, AC-23; command rows in S4 table C-1 | The commands are declared and answer not-implemented. The binding and the first generation record exist | All of it |
| Registry `trust-anchor` | S4 table S-3; S2 AC-18 | The pull client accepts an anchor. The schema has no leaf | The leaf, the read of the `pki` tree, the refusal test |
| The shell into a container | S5, all phases | Nothing built | A probe gate in the guest, then phases 1 to 8 |
| Health check | S6 | Skeleton: only the task is written | Design, then everything |

### Network

| Item | Owner and place | State | Remains |
|------|-----------------|-------|---------|
| Publishing a port a Ze listener holds | S3 phase 5, rule V-9; message in S4 table E-1 | Not started | The refusal at commit and `doctor-container-port-conflict` |
| Conflicts between two instances' ports | S3 slice 4b, rule V-6 | Refused at verify | The doctor code is not registered |
| Resolver file, `show container network list`, `show container interface list`, the host network mode | S3 phase 4, "Name resolution" (the handover calls it slice 4c) | Not started | All of it |
| Kernel option enrolment and the network's doctor checks | S3 phase 6, "Problem classes" | Of sixteen codes only `doctor-container-port-blocked` is in the code | Four capability pairs and the other codes |
| IPv6 | S3 Design Insights, phase 3; rules V-13 and V-14 | Forwarding is claimed per family. The local-destination match is refused in a dual-stack table, and the transit test covers IPv4 only | The dual-stack match, an IPv6 run of the transit test, router advertisements under forwarding |
| `description` on a network and on an instance | S4 table S-2, phase 2 | Absent | The two leaves |

### Storage, the appliance and proofs

| Item | Owner and place | State | Remains |
|------|-----------------|-------|---------|
| The size cap, `show container store`, `store largest-image` | S2 "Capped storage", phase 6, AC-15, AC-16, AC-22; S4 table S-3 and rule V-12 | No cap is applied. After the release, by the ruling of 2026-11-17 | Carry the ruling into S2 and S4; decide what `show container store` answers with no cap; the `largest-image` leaf, which one doctor fix line already names |
| The persistent partition grows at install | SA phases 4 to 8 | Phases 1 to 3 done. Growth is written and not wired. The storage command is a stub | Phases 4 to 8. One decision blocks it: the kernel's `gpt` boot option |
| Runtime: doctor for privilege, the forked plugin, the appliance boot proof | S1 phases 8 and 9 | Six runtime checks exist. None of the four forked-plugin and privilege tests exists, nor the boot proof | Those tests and the proof |
| Runtime: reload keeps unchanged containers, state across a daemon restart | S1 AC-18, AC-20, AC-22 | Not proven | The tests |
| Image store: the remaining fetch tests, store doctor, the interop registry proof | S2 phase 8 and phase 9, functional table | Seven fetch tests absent. The four interop tests absent | As listed there |
| Operator surface: schema audit, offline verification, the absent-gate proof, surface parity, metrics, the cross-sibling doctor checks | S4 phases 2, 3, 6 and 7, table M-1, table D-1 | Not started. No container metric exists | As listed there |
| Pages: the guide, each child's part of `features/containers.md` | S4 phase 8; the last phase of S1, S2, S3, S5 and SA | The architecture pages exist. `docs/guide/containers.md` does not | The guide and each extension |

### Owned by no child until now

These are the rows of `spec-container-7-followup.md`: CI running the suite with the seeded image, a plugin command declaring its time bound, a sibling reading the applied firewall ruleset, a layer entry written through a link into an opaque folder, a guard that every doctor fix line parses, a seed with more than one pinned image, and two tooling defects.

### Spec text the code now contradicts

Each is recorded as a deviation in the child that holds the text. The text itself is corrected when that child is next edited for its own work.

| Text | Where | What is true |
|------|-------|--------------|
| The closing test runs in the appliance guest, as `test/appliance/container-redis.ci` | This spec, AC-14 and its wiring rows; S4 section X-1, AC-15 and its test rows | It is `test/container/container-redis.ci`, in the container suite. No step of an appliance test runs inside its guest |
| The closing test asserts `restart-count` | S4 section X-1 step 7; this spec AC-10 | The listing has no such column yet. The restart is proven by the server's run id |
| The closing test asserts on the fetch command's answer | S4 section X-1 step 1; S2 "answers once" | The command answers after 20 seconds that the fetch still runs, so the test asserts on the listing row |
| The closing test's client is a package `internal/test/respclient` | S4 Files to Create, phase 9 | The probe `internal/test/containerprobe` speaks the protocol |
| `doctor-container-port-blocked` reads the applied ruleset | S3 "Published ports" | It judges the committed firewall config |
| "The seed action", unnamed | S2 "The owner's closing test" | `./le test container-seed fetch` |

### Decisions still open with the owner

| Decision | Recorded in |
|----------|-------------|
| May CI reach the public registry for the seed | S7, F-1 |
| The state of an enabled container before its first start ends | The handover, open item 1 |
| A time bound a plugin command can declare, and whether a caller leaving cancels it | S2 Deviations; S7, F-2 |
| Does a running container restart when its `image` changes | The handover, open item 3 |
| The word `destination-local`, the dual-stack match, a shifted port range | S3 Design Insights; the handover, open item 4 |
| Interface names that start with `zc` are refused; a network with translation off still needs a firewall section | S3 question Q-1, AC-23 |
| A registry proxy setting; IPv6 forwarding and router advertisements; an unknown state for `show container store` | S2 Deviations; the handover, open item 6 |
| The kernel's `gpt` boot option; the withheld device-node capability | SA assumption A-15; the handover, open item 7 |
| A doctor code of its own for a refused binding write | S2 Deviations |
| What an unhealthy result does | S6 |

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
