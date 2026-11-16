# Spec: container-0-umbrella

| Field | Value |
|-------|-------|
| Status | skeleton |
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
| Third-party runtime | Onboarding a Go OCI runtime is permitted. |
| Implementation model | Implementation agents for this work run on the strongest tier. |

### Open decisions (owner)

| # | Question | Recommendation | What each answer changes |
|---|----------|----------------|--------------------------|
| O-1 | Runtime: Ze's own pure-Go runtime, or a build flavor with cgo that embeds a third-party runtime library | Pure Go. Every Ze build forces cgo off today | Pure Go keeps the build as it is and makes Ze own the confinement code. The cgo flavor reuses a hardened runtime and needs a static C cross-toolchain per target architecture |
| O-2 | Writes to a container's root: discarded at restart, or kept until the next image update | Discarded at restart | Discarding makes every start identical to the image and forces state into declared volumes. Keeping them tolerates images that expect a persistent root and lets a container drift from its image |
| O-3 | Release timing: first release, or after | After | Decides the folder this work lives in and its priority |
| O-4 | Running a container's entrypoint is running a program Ze does not carry. `ai/rules/go-standards.md` forbids that without a row in `ai/allowed-system-commands.md`, which only the owner adds | Add the row, scoped to the container init role | Without the row no code that starts a container may land. The only row of this kind today is the operator's own plugin program |
| O-5 | Every declared feature gate is on in every daemon build today (distribution and appliance). Is the container gate on by default like the others, or off unless a build asks for it | On by default, like every other gate | Off by default needs a new mechanism: no gate today is declared and left out of the default builds |

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
- Embedding the upstream runtime library is impossible with cgo off, and every Ze build forces cgo off. One binary therefore means Ze's own runtime, or a cgo build flavor (O-1).
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
| Self re-execution by role | `cmd/ze/dispatch.go` `dispatchMain`; `internal/core/seedinit/register.go` `Dispatch`; `internal/core/crashlog/exec_unix.go` `Exec` | `ze` selects a role from its first argument, or from an environment key before startup. Every execve in Ze goes through one wrapper, and a raw one is refused by a lint rule | Extends: the container init is one more early role, and it reaches the image entrypoint through that wrapper. No role today runs inside new namespaces |
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
- To be filled during research.

### Transformation Path
1. To be filled during research.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| To be filled | | No |

### Changed-Wire Census

**Search evidence:** to be filled during design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- To be filled during research.

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
| A-1 | The upstream Go container runtime library cannot create a container with cgo off | Read at tag v1.5.3: the namespace-entry package has no file that builds without cgo, and the filter stub refuses any profile | O-1 has a third answer: embed the library with no build change | A build of that tag with cgo off. A hook refused the build outside the repository's output directory, so it was not run | read from source, not built |
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
| R-8 | The squashfs format, as this kernel builds it, drops extended attributes, so file capabilities in an image are lost and programs that rely on them fail | A container program that needs a file capability fails to start | Decide the image format at design: enable the kernel option, or use another format |

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
| AC-1 | To be filled at design | | |

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
