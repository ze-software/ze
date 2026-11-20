# Spec: vpp-embedded-runtime

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P2 |
| Release reason | Owner decision (2026-11-19): VPP on the appliance is owed for the first release. Today an appliance whose config selects the VPP backend has no VPP program to start, so the release artifact cannot do what its VPP configuration asks. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze starts and supervises VPP itself, but it does not carry VPP. The VPP component runs the program at a fixed host path and expects an operator to have installed it there. The gokrazy appliance has no package manager and no C library, so nothing can put VPP at that path: an appliance configured for the VPP backend cannot start its data plane. The appliance image only prepares the host side (hugepage reservation, CPU isolation), and the VPP guide points appliance users at a sidecar image that nothing in the repository builds.

Symptom today: on an appliance, a config with the VPP component enabled and managed by Ze finds no VPP program to run.

Goal: a `ze` built with the VPP runtime carries VPP inside its own binary and starts it with nothing pre-installed, on the appliance and on a plain Linux host alike.

A second defect is fixed here because the goal depends on it. A VPP that Ze starts loads only the plugins Ze names, and Ze names none for firewall rules, NAT, VXLAN tunnels or GRE tunnels. Those four are separate plugins in VPP 25.10 and Ze's backends call their APIs. So on a VPP that Ze started, that configuration is accepted and then refused by VPP when applied. It is unnoticed today because the only proofs of those features run against a VPP the lab starts itself with every plugin on. Without this fix the appliance would start VPP and accept firewall configuration that does nothing.

### What the owner decided (2026-11-19)

| Decision | What it fixes here |
|----------|--------------------|
| Where the runtime lives | Inside the `ze` binary, through Go's embedded file system. Not as loose files in the appliance image, and not as a container image |
| What is embedded | VPP is a bundle, not one static program: the `vpp` program, its plugins and the C libraries it loads. The bundle is embedded whole and unpacked by Ze before it starts VPP |
| Vendoring | Building and carrying VPP inside the Ze project is accepted as a cost of business |
| Release | Owed for the first release (`plan/release/delivery/`) |
| Reuse | This spec ships VPP only. The embedding and unpacking is shaped so a second outside program can use it later (the switch chip daemon discussed on 2026-11-19 is the named candidate). That second program is out of scope here |
| Unpack location | An operator chooses disk or memory in the configuration. The default is disk |
| Where the built bundle is kept | Built and cached like the appliance kernel. Not committed to git |
| An operator-installed VPP | Kept as the fallback for a `ze` built without the runtime: a managed start there runs the program at the standard host path, as today. A `ze` that carries the runtime always runs its own. This is the owner's exception to the no-fallback rule (`ai/rules/no-layering.md`), made for this case |
| The undeclared plugins | Fixed in this spec |

### Not in scope

| Item | Why |
|------|-----|
| A second embedded program (a switch chip daemon) | Owner decision above: reusable shape only |
| Changing how Ze supervises VPP once it runs | The supervisor exists; this spec changes only where the program comes from |
| The operator-supervised mode where Ze only connects to a VPP something else runs | Stays as it is |
| New VPP features | The four plugin declarations make existing backends work on a managed VPP. They add no feature |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every change respects
  → Constraint: a compile tag is a boundary. Always-on code never imports a gated package, so the embedded bundle lives only in a package a tag removes, and the VPP component reaches it through a seam the package fills from its own `init()`
  → Constraint: a readiness check is a doctor check registered by the package that owns the dependency
  → Constraint: a feature registers and is discovered. No central list learns its name
- [ ] `ai/doors/forwarding.md` - the VPP component and its backends
  → Constraint: the `ze_vpp` tag removes `internal/component/vpp` and every VPP backend. A build without it must still compile and must carry no VPP bytes
  → Constraint: a backend that cannot apply a config exactly refuses it at verify. A plugin that is not loaded is that case
- [ ] `ai/doors/appliance.md` - the gokrazy image and the device
  → Constraint: an image in the field is the contract for any path under `/perm/ze/`. The directory this spec creates there is permanent once shipped
  → Constraint: a gokrazy device does not replace its own `ze` binary, so the bundle a device runs is always the one inside the image's `ze`
  → Constraint: `ze appliance build` links no `internal/le` package, so what resolves the bundle for an image build lives in `internal/appliance`
- [ ] `ai/doors/le-tooling.md` - the build and test tooling
  → Constraint: a shipped `ze` links no `internal/le` package. What the daemon needs to unpack and run the bundle lives outside `internal/le`
  → Constraint: a `.ci` that needs a Linux kernel feature carries `option=needs-linux` and runs in the QEMU guest
- [ ] `ai/doors/core.md` - shared libraries and runtime state
  → Constraint: a core package imports no component. The unpack library holds no VPP knowledge
  → Decision: the raw-write gate scans components, plugins and `cmd/ze`; core infrastructure is outside that rule. The unpack library is a core package and writes program files, not daemon state, so it owes no exemption there
- [ ] `ai/doors/hub-engine.md` and `internal/le/repo/compiles/matrix.go` - build flavors
  → Constraint: the `distro` and `appliance` flavors of `buildMatrix` set `Features`, which appends every tag declared in `feature-gates.txt`. Development builds and the test runs take the same expansion. A tag declared there reaches every one of those builds
  → Decision: the runtime tag is therefore a packaging tag, like `ze_appliance`, and is not declared in `feature-gates.txt`. Only the appliance image build and the release build set it
- [ ] `ai/doors/access-security.md` - the PPPoE backend lives under the L2TP component
  → Constraint: `l2tp` and its PPPoE plugins are behind `ze_l2tp`. The edit to the PPPoE startup registration's fix text stays inside that gate
- [ ] `ai/rules/config.md` and `ai/patterns/config-option.md` - the new setting
  → Constraint: a tunable setting is a YANG leaf with a `ze:help` line and a `description`. Each enum value carries its own `ze:help`. Leaf names are spelled out in full
  → Constraint: every delivered config value arrives as a string
- [ ] `ai/rules/repo-maintenance.md` - what a new surface owes
  → Constraint: a new runtime dependency owes a registered doctor check, an `ai/INDEX.md` row and its architecture page in the same work
- [ ] `ai/digests/vpp-dataplane.md` - the current flow of the VPP component
  → Decision: on gokrazy Ze is the only process supervisor, so Ze writes the VPP config, binds the NICs, runs the program, connects, and restarts it
  → Constraint: the operator-supervised mode inverts every step: Ze writes no config, runs no program and stops nothing
- [ ] `docs/architecture/appliance/build-artifacts.md`, "The runtime kernel the appliance image boots" - how a built artifact is cached and reused
  → Decision: the runtime kernel has two tiers, its keyed cache entry or a native build on a host of the target's architecture, and no download tier
  → Constraint: a build for another architecture than the host's is refused before it starts, with the command that works
  → Constraint: an artifact is never rebuilt when a build of the same inputs exists, and a test never builds one (`ai/META-DECISIONS.md`, "Test systems and labs")
- [ ] `third_party/vpp-patches/README.md` - the only VPP build the repository holds
  → Decision: VPP is built from a pinned upstream commit (v25.10) with its archive checksum verified, plus retained patches applied with no fuzz. No copy of the VPP source tree is kept in Ze
  → Constraint: the PPPoE backend requires this patched VPP. A stock VPP is refused by its API check, so the bundle is built from the same source and patches
- [ ] Upstream `src/vlib/unix/plugin.c` at the pinned commit (read 2026-11-19)
  → Decision: the config section `plugins` accepts `path <dir>`, so the bundle needs no fixed install prefix. No environment variable overrides it
  → Constraint: with no path configured and no built-in default, VPP loads no plugin at all

**Key insights:** (minimal context to resume after compaction)
- The supervisor exists and is not the problem. `runConfiguredVPP` passes the constant `/usr/bin/vpp` to `NewVPPManager`, and `runOnce` runs that path with the generated config as its one argument
- VPP is a bundle. Measured on the local lab image (arm64, three plugins, no DPDK): about 63 MB unpacked, 50 MB of it one library. Plugins are separate shared objects VPP loads from a directory
- The bundle also needs what gokrazy lacks: the C library, its loader, and the system libraries VPP and its plugins link
- The appliance `ze` is already built with the VPP tag (`gokrazy/ze/config.json`, `GoBuildTags`)
- On the appliance `/perm` is ext4 mounted with no flags (execution allowed), `/tmp` and `/run` are tmpfs, `/etc` is a 1 MB tmpfs, the root is read-only
- VPP has never run on the appliance. `./le test qemu vpp-hugepages` checks the boot command line and the hugepage count only
- `RegisterStartupFeature` (`internal/component/vpp/startup_feature.go`) is the existing registry where a feature declares its VPP plugin, its API messages and its doctor code. PPPoE is its only user
- `./le build vpp run` (`internal/le/build/vpp`) already builds the pinned, patched VPP as a lab image. The bundle build is a new action beside it
- `buildOne` (`internal/appliance/cmd_build.go`) refuses an image build whose runtime kernel is unreachable before anything is written. The bundle gets the same check at the same place
- A tag in `feature-gates.txt` is appended to the development, test, `distro` and `appliance` builds alike. The runtime tag stays out of that file so that only image and release builds embed the bundle
- Licence texts read on 2026-11-19: LGPL 2.1 section 4 lets the C library and libnl binaries be distributed "provided that you accompany it with the complete corresponding machine-readable source code", or by "offering equivalent access to copy the source code from the same place". VPP's Apache 2.0 section 4(a) requires giving recipients "a copy of this License"

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/vpp/register.go` - registers the component; `runConfiguredVPP` builds the manager with the constants `defaultConfDir` (`/etc/vpp`) and `defaultVPPBinary` (`/usr/bin/vpp`). No config leaf or environment variable changes either
- [ ] `internal/component/vpp/vpp.go` - `VPPManager.runWithOps` writes the config, binds NICs and loops; `runOnce` runs `vppBinary -c <config>`, wires its output to Ze's own, connects GoVPP within 30 seconds, binds the child to a process witness, and restarts with backoff capped at 30 seconds
- [ ] `internal/component/vpp/startupconf.go` - `GenerateStartupConf` writes a `plugins` section that disables every plugin by default and enables named ones: DPDK and af_packet by setting, uRPF always, linux-cp and linux-nl when LCP is on, WireGuard by setting, and each selected startup feature's plugin. Those built-in names are literals inside the function. It names no plugin directory. It writes one config section per selected feature
- [ ] `internal/component/vpp/startup_feature.go` - `RegisterStartupFeature` takes a plugin file name, a config section name (mandatory), flags, the config roots the feature reads, a predicate over the candidate config, and optionally the plugin's API messages with a doctor code. With API messages it registers a doctor check that compares the running VPP's message table
- [ ] `internal/component/vpp/config.go` - `VPPSettings` holds the parsed settings; `PluginSettings` has three toggles (WireGuard, DPDK, af_packet). No setting names where the program comes from
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - the `vpp` container's description tells the operator Ze "starts /usr/bin/vpp"; the `external` leaf names a container sidecar as one of its uses
- [ ] `internal/component/vpp/doctor_linux.go` - the hugepage doctor check, registered by the component, reading system files behind overridable roots so tests run against fixtures
- [ ] `internal/component/l2tp/plugins/pppoe-vpp/startup.go` - the one registration: plugin `pppoe_plugin.so`, section `pppoe`, its messages and the code `doctor-vpp-pppoe-plugin`. Its description tells the operator to build the artifact with `./le build vpp run` and install it
- [ ] `internal/plugins/firewall/vpp/binapi_imports.go` - the firewall backend links the ACL and NAT44 API packages. It registers no startup feature
- [ ] `internal/plugins/iface/vpp/doctor.go` - `vppctlShowPlugins` runs the `vppctl` program to learn whether the linux-cp plugin is loaded. It reports a warning when `vppctl` is absent
- [ ] `internal/le/build/vpp/register.go` - the `build vpp` actions: `run` builds the lab image from `third_party/vpp-patches/`, `update` generates the PPPoE Go bindings
- [ ] `internal/le/repo/compiles/matrix.go` - `buildMatrix` holds five flavors. `distro` and `appliance` set `Features`, which appends the feature manifest's tags; `setup`, `host` and `installer` do not
- [ ] `third_party/vpp-patches/Dockerfile` - builds VPP from the pinned commit in a container with three plugins and a native-only CPU target, installs it under `/usr`, and copies it into a distribution base image that supplies the C library and system libraries
- [ ] `internal/le/test/deployment/vppevidencescenarios.go` - the lab that proves ACLs on real VPP sets `external true` in its Ze config
- [ ] `internal/appliance/cache.go` - `kernelTreeCachePath` places a built tree under a directory named by version and a variant hashed from the build's inputs; `KernelCacheNamespaces` lists the cache directories that scratch cleanup must preserve
- [ ] `internal/appliance/cmd_build.go` - `buildOne` validates the appliance config, then calls `runtimeKernelReachable` and refuses before anything is written when the kernel can be neither found nor built here
- [ ] `internal/le/arch/fspersistence/fspersistence.go` - the raw-write rule scans components, plugins and `cmd/ze`; core is scanned for the live-store rule only
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@.../mount.go` - the mounts the appliance boots with
- [ ] `feature-gates.txt` - the `ze_vpp` gate and the seven packages it removes
- [ ] `internal/core/diagnostic/codes.go` - the VPP doctor codes that exist. None reports a missing or damaged VPP program

**Behavior to preserve:** (unless the user explicitly said to change it)
- The whole supervision loop: config generation, NIC binding and restoration, the GoVPP connect bound, the process witness, the restart backoff, the `connected` / `disconnected` / `reconnected` events
- The operator-supervised mode (`external true`): Ze runs nothing and unpacks nothing
- A `ze` built without the runtime keeps today's managed start: it runs the program at `/usr/bin/vpp`, and its generated config is byte-identical to today's (owner decision)
- A build without the VPP tag carries no VPP bytes and refuses a `vpp` config exactly as today
- Development builds and test binaries stay the size they are: none embeds the bundle
- The lab image, `./le build vpp run` and its proofs under `third_party/vpp-patches/`
- The PPPoE startup registration and its doctor check

**Behavior to change:** (only what the user asked for)
- A `ze` that carries the runtime starts VPP from its own bundle and never looks at the host path
- A managed start loads the ACL, NAT, VXLAN and GRE plugins when the configuration uses them
- The linux-cp plugin check stops running `vppctl` and reads the running VPP's API message table, the way a registered feature's check does

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VPP process supervisor | `internal/component/vpp/vpp.go`, `VPPManager.runWithOps`, `runOnce` | Writes the config, binds NICs, runs the program, connects, restarts with backoff | Extends: the supervisor is handed how to run the unpacked program instead of a host path. Nothing else in the loop changes |
| Fixed program path | `internal/component/vpp/register.go`, `defaultVPPBinary`, `runConfiguredVPP` | The one source of the program path, a constant | Extends: stays the source for a `ze` without the runtime (owner decision). A `ze` with the runtime takes the program from the bundle |
| Operator-supervised mode | `internal/component/vpp/config.go`, `VPPSettings.External` | Ze only connects to a VPP something else runs | Does not apply: preserved untouched |
| VPP config generation | `internal/component/vpp/startupconf.go`, `GenerateStartupConf`, the `plugins` section | Names which plugins load, never where they are. Built-in plugin names are literals in the function | Extends: when running from the bundle the section names the bundle's plugin directory. The built-in names move to one declaration that the generator and the bundle's plugin list both read |
| Startup feature registry | `internal/component/vpp/startup_feature.go`, `RegisterStartupFeature` | A feature declares its plugin, config section, API messages and doctor code. One user, PPPoE | Extends: firewall, NAT, VXLAN and GRE register here. The config section becomes optional, because those plugins need no section. The registry becomes the list the bundle's plugins derive from |
| Per-feature API doctor check | `internal/component/vpp/startup_doctor.go`, `registerStartupDoctor` | Compares the running VPP's message table with a registered feature's messages | Extends: each new registration gets it. Replaces the `vppctl` probe of the linux-cp check |
| Patched VPP build action | `internal/le/build/vpp/register.go`, action `run`; `third_party/vpp-patches/Dockerfile` | Builds the pinned, patched VPP v25.10 as a lab image with three plugins | Extends: a new action in the same area builds the product bundle from the same source and patches. The lab image stays |
| Keyed artifact cache with native builds | `docs/architecture/appliance/build-artifacts.md`, "The runtime kernel the appliance image boots"; `internal/appliance/cache.go`, `kernelTreeCachePath`, `KernelCacheNamespaces` | A slow native build is cached under a key of its inputs, never rebuilt for the same inputs, refused for another architecture, and preserved by scratch cleanup | Extends: the bundle is one more artifact in that cache, keyed the same way, resolved by the same callers, and added to the preserved namespaces |
| Reachability refusal before an image build | `internal/appliance/cmd_build.go`, `buildOne`, `runtimeKernelReachable` | Refuses before anything is written when the kernel can be neither found nor built on this host | Extends: the bundle is checked at the same place, the same way |
| Data embedded in the owning package | `internal/component/vpp/yang/embed.go` | The component embeds its own YANG file | Extends: same pattern, a large binary artifact, in a package of its own behind its own tag |
| Feature gates and their expansion into flavors | `feature-gates.txt`, `ze_vpp`; `internal/le/repo/compiles/matrix.go`, `buildMatrix`, `Flavor.Features` | A declared gate removes its packages, and its tag is appended to the development, test, `distro` and `appliance` builds | Does not apply to the bundle: a gate declared there would put the bundle in every one of those builds. The bundle package stays under `ze_vpp` only by importing nothing outside it |
| Packaging tags | `internal/le/repo/compiles/matrix.go` (`ze_appliance`, `ze_distro`); `gokrazy/ze/config.json`, `GoBuildTags` | A flavor's own tags select what that packaging carries, apart from the feature manifest | Extends: `ze_vpp_runtime` is a packaging tag set by the appliance image build and the release build. Nothing else sets it |
| Appliance host tuning for VPP | `internal/appliance/kernelargs.go`; `internal/le/test/qemu/hugepages.go`, action `vpp-hugepages` | Reserves hugepages and isolates CPUs at boot; the proof checks those two | Extends: a sibling QEMU action boots the same kind of image and starts real VPP from the image's `ze` |
| VPP doctor checks | `internal/core/diagnostic/codes.go`; `internal/component/vpp/doctor_linux.go` | Report an unreachable socket, lost process ownership, missing hugepages, reading system files behind overridable roots | Extends: each new problem class owes its own check and code, built the same testable way |
| Raw-write gate | `internal/le/arch/fspersistence/fspersistence.go`, `scanRoots`, `dirAllowlist` | Refuses a raw file write of daemon state in components, plugins and `cmd/ze` | Does not apply: the unpack library is a core package, which that rule does not scan, and it writes program files, not state. No exemption is added |
| Ze-run container images | `plan/post-release/features/spec-container-0-umbrella.md` and its children | In progress, after the release: Ze fetches and runs OCI images | Does not apply: the owner chose embedding over a container image on 2026-11-19 |
| Process ownership witness | `internal/component/vpp/vpp.go`, `startManagedProcess`, `bindManagedProcess` | Binds the API connection to the exact child Ze started | Preserved: the bundle's loader replaces itself with VPP inside the same process, so the child Ze started is still the one the witness binds (A-10) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build time: the pinned VPP source and patches under `third_party/vpp-patches/`, and the plugin list derived from the startup feature registry
- Run time: the `vpp` configuration root, delivered to the VPP component as a config section, with `enabled true` and `external false`

### Transformation Path
1. Build. `./le build vpp bundle` builds VPP in a container for the host's architecture with the listed plugins and a portable CPU target, collects the program, the plugins, the crypto engines, every shared library they link and the C library loader, adds the licence texts, and writes one compressed archive plus its description (name, version, architecture, entry program, loader, library directory, plugin directory, and each file's path, mode, size and SHA-256). It also writes an archive of the corresponding source of every LGPL component. Both go into the keyed cache
2. Placement. The same action, and the appliance image build, copy the cached archive and description into the bundle package's data directory, which git ignores
3. Embedding. A `ze` built with `ze_vpp_runtime` embeds that archive. The bundle package registers it with the VPP component from its `init()`
4. Selection. At a managed start the VPP component asks its runtime seam for a bundle. None registered means the host path, as today
5. Unpack. The unpack library writes the files under a directory named by the archive's SHA-256, inside the root the `storage` setting selects, through a temporary directory renamed into place. A directory already present is verified file by file against the description and reused; a file that fails is rewritten from the archive
6. Run. The VPP component generates the config with the bundle's plugin directory and hands the supervisor the command the description yields: the bundled loader, the library directory, the program, then the config argument
7. Supervise. Unchanged: connect, witness, events, restart

### Unpack roots

| `storage` | Root | Why this place |
|-----------|------|----------------|
| `disk` (default) | `<Ze's configuration directory>/runtime/<bundle name>/<archive SHA-256>/`. The configuration directory is the one `ze.config.dir` selects, `/perm/ze` on the appliance | It is the one directory Ze already owns and persists on every host, and on the appliance it survives an image update. Once an image ships, `runtime/` under `/perm/ze` is a permanent path |
| `memory` | `/run/ze/runtime/<bundle name>/<archive SHA-256>/` | `/run` is a memory filesystem on the appliance and on a plain Linux host, and it is empty after a reboot, so nothing persists |

The bundle name is `vpp` here. A later program registers under its own name beside it. A directory named by the checksum means a new bundle never overwrites the one a running VPP was started from.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build host container to cache | An archive and its description, keyed by a hash of the inputs | No |
| Cache to Go build | Files copied into the bundle package's data directory before the build | No |
| Bundle package to VPP component | A registration from `init()` through a seam in the VPP component. The component never imports the bundle package | No |
| Ze to the filesystem | Program files written by the unpack library only | No |
| Ze to VPP | The supervisor's existing child process and API socket | No |

### Changed-Wire Census

**Search evidence:** Scope: the whole tree outside `vendor/` and `tmp/`. Terms: `go:embed` under `internal/component/vpp` and the VPP backends (only the YANG file is embedded today); `defaultVPPBinary` and `vppBinary` (one writer, `register.go`, one reader, `vpp.go` `runOnce`); `/usr/bin/vpp` in `test/` and `internal/appliance` (no producer places a program there); `RegisterStartupFeature` (one caller, the PPPoE backend); `vppctl` in non-test Go (one caller, `internal/plugins/iface/vpp/doctor.go`). Positive control: the same searches find the YANG embed, the constant, the PPPoE registration and the `vppctl` call. Finding: no bundle format exists today, so the embedded bundle is a new wire with no existing speaker to migrate.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Embedded program bundle | writer | New: the `bundle` action in `internal/le/build/vpp`, with a new `third_party/vpp-patches/Dockerfile.bundle` | No existing writer: the existing Dockerfile's final stage produces a container image, not an archive | New wire. Its description format is defined once, in the unpack library, and the writer uses that definition |
| Embedded program bundle | reader | New: the unpack library `internal/core/programbundle`, called by the VPP component before `VPPManager.runOnce` | No existing reader: `runOnce` reads only the constant path | New wire |
| Program command handed to the supervisor | writer | `internal/component/vpp/register.go`, `runConfiguredVPP` | Passes `defaultVPPBinary` to `NewVPPManager` | Changes: it hands the bundle's command when a runtime is registered, and the constant otherwise |
| Program command handed to the supervisor | reader | `internal/component/vpp/vpp.go`, `VPPManager.runOnce` | Runs the path it was given with the generated config as its one argument | Changes: it runs a command of several arguments, of which today's single path is the one-element case |
| Generated VPP config, plugins section | writer | `internal/component/vpp/startupconf.go`, `GenerateStartupConf` | Writes `plugin default`, then one `plugin` entry per enabled name | Changes: adds the plugin directory when running from the bundle. Byte-identical output otherwise, pinned by a test |
| Generated VPP config, plugins section | reader | The VPP program, upstream `vlib_plugin_config` | Accepts `path`, `add-path`, `plugin default`, `plugin <name>` (read at the pinned commit) | Preserved: only keywords it already accepts are written |

The labs under `internal/le/test/deployment/` write their own VPP config text for a VPP they supervise themselves. They do not read Ze's generated file, so they are not speakers of this wire.

### Integration Points
- `vpp.RegisterStartupFeature` - the firewall, NAT, VXLAN and GRE backends register their plugin from their own `register*.go`
- A new runtime seam in the VPP component - the bundle package registers; the component asks at each managed start
- `VPPSettings` - gains the `storage` choice parsed from the new leaf
- `buildOne` and the cache in `internal/appliance` - the image build resolves the bundle the way it resolves the runtime kernel
- `./le build vpp` - gains the `bundle` action
- `gokrazy/ze/config.json` and the release build - set the packaging tag

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | VPP loads its plugins as separate files from a directory, so it cannot be one static program | Upstream `plugin.c` at the pinned commit, read 2026-11-19: it splits a plugin path and loads the files it finds | A single static program could run from memory with no unpack step | Confirmed by reading the source | confirmed |
| A-2 | VPP takes its plugin directory from the config file | Upstream `vlib_plugin_config` matches `path` and stores it as the plugin path | The bundle would need one fixed unpack location | Confirmed by reading the source; proven end to end by the QEMU proof | confirmed |
| A-3 | The bundled C library loader can be run directly with a library directory, so nothing is needed under `/lib` on the appliance, and the libraries VPP loads later resolve the same way | General behaviour of the GNU loader, not verified in this repository | Every file in the bundle needs its paths rewritten at build time, which fixes the unpack location | The QEMU proof on a real appliance image, in the build phase before any other code depends on it | unvalidated |
| A-4 | The product bundle is small enough to embed. The lab build measures about 63 MB unpacked without DPDK | `docker run` on `ze-vpp-pppoe-lab:latest`, 2026-11-19, arm64 | A much larger bundle changes the cost of every runtime build of `ze` and of each unpack | The build phase measures it, packed and unpacked, per architecture, and records both in the build-artifacts page. A packed size above 100 MB returns to the owner before the next phase | unvalidated |
| A-5 | `/perm` allows execution and has room for the unpacked bundle | gokrazy `mount.go` mounts `/perm` with no flags; its size comes from the image config | Disk storage fails on the appliance and the default must be memory | The QEMU proof | unvalidated |
| A-6 | Every plugin Ze can write into the config is known from two declarations: the built-in list and the startup feature registry | `startupconf.go` and `startup_feature.go` are the only places a plugin name reaches the generated config | A plugin Ze enables is missing from the bundle and VPP refuses to start | The plugin list test (`TestBundlePluginListMatchesDeclarations`) | unvalidated |
| A-7 | The licences of DPDK and of each bundled system library beyond the C library and libnl permit shipping them inside `ze` with their notices | Only LGPL 2.1 and Apache 2.0 were read for this spec. The full library list is known only once the bundle is built | A library cannot be shipped this way, or owes an obligation the build does not meet | The build phase lists every file's licence in the description; each licence text is read and its distribution clause quoted in the bundle's README before the bundle is used | unvalidated |
| A-8 | A VPP built without the native-only option runs on any 64-bit CPU the appliance supports | The lab build sets a native-only option; upstream builds its CPU-specific code as selectable variants | A bundle built on one CPU crashes on another with an illegal instruction | The QEMU proof repeated with a different CPU model | unvalidated |
| A-9 | Paths VPP uses beyond its program exist and are writable on the appliance: `/etc/vpp` (a 1 MB tmpfs), `/run/vpp`, the log path under `/var` | `register.go`, `startupconf.go`, gokrazy `mount.go` | VPP starts from the bundle and then fails on a missing or full directory | The QEMU proof. Each failure it shows is fixed in this spec | unvalidated |
| A-10 | The loader becomes the VPP program inside the process Ze started, so the process witness and the API peer check see the same process | How a program loader works in general; the witness compares the API socket's peer with the started child | The witness refuses the connection and a managed start never completes | `TestManagedStartThroughLoaderBindsWitness` and the QEMU proof | unvalidated |
| A-11 | The ACL, NAT, VXLAN and GRE APIs are absent from a VPP whose plugin was not loaded | The plugins are separate directories under `src/plugins` at the pinned commit, and Ze's config disables every plugin it does not name. Not observed in a run | The four declarations are harmless but the defect described in the Task does not exist | The QEMU proof applies a firewall rule before and after the declaration, and the first run is recorded red | unvalidated |
| A-12 | The crypto engines VPP 25.10 needs for IPsec are loadable files in a directory of their own, found relative to the plugin directory or named in the config | They are not under `src/plugins` at the pinned commit. How VPP finds them was not read | IPsec on a managed VPP finds no crypto engine | Reading the crypto engine loader at the pinned commit in the build phase, then the description's file list | unvalidated |
| A-13 | An embedded, separately executed VPP inside an AGPL `ze` is an aggregate in the AGPL's sense | The AGPL text (`LICENSE`, the aggregate paragraph) read 2026-11-19. Whether it applies is a legal judgment this spec does not make | The licence position of the release artifact needs the owner's or a lawyer's decision | The owner confirms before the first release that carries the bundle | unvalidated |
| A-14 | The file name of each plugin (which shared object carries the NAT44 API, for instance) is what the built bundle's file list says | Directory names at the pinned commit; file names not read | A declaration names a file the bundle does not hold | Each declaration takes its file name from the built bundle's description, and `TestBundlePluginListMatchesDeclarations` compares them | unvalidated |
| A-15 | The check that compares the gokrazy tag list with the feature manifest accepts one more packaging tag beside `ze_appliance` | `gokrazy/ze/config.json` already lists `ze_core` and `ze_appliance`, which are not feature gates. The check itself was not read | The check refuses the image config until it learns the tag | Running `./le repo feature-tags check` in the wiring phase. If it refuses, the packaging tags it accepts become one declaration it reads | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | VPP has never run on the appliance, so the first real start surfaces host assumptions this spec did not list | The QEMU proof fails after the program starts | The proof is an acceptance criterion and runs in the build phase. Each failure it shows is fixed inside this spec |
| R-2 | The unpack directory fills `/perm`, or the directory forbids execution | The unpack returns a write error, or the program fails to start with a permission error | The unpack checks free space against the description's total before writing and checks the mount's flags, and each refusal is an operator error naming the `storage` setting to change. A doctor check reports both ahead of a start |
| R-3 | Every runtime build of `ze` grows by the packed bundle, and the appliance image with it | `ze` size and image size after the first bundle build | Only image and release builds set the packaging tag. A-4 sets the size at which the owner is asked again |
| R-4 | Power is lost during an unpack and a half-written directory is taken for a complete one | A start after an interrupted unpack fails to run VPP | Files are written into a temporary directory and renamed into place only when complete, and every start verifies every file |
| R-5 | A file under the unpack directory is changed after it was unpacked | Verification at start reports a mismatch | The file is rewritten from the archive, the event is reported with its own doctor code, and VPP starts from the repaired tree |
| R-6 | Two `ze` processes unpack the same bundle at once | Both write at the same time | Each unpack uses its own temporary directory and the rename is atomic, so the loser finds a complete directory and verifies it |
| R-7 | Old unpacked bundles accumulate across image updates | `/perm` usage grows by one bundle per update | After a successful start the unpack library removes every sibling directory that is not the current bundle |
| R-8 | The container base image changes between builds and the bundle is no longer reproducible | Two builds of the same inputs produce different archives | The Dockerfile pins the base image by digest and every package by version, and the cache key includes the Dockerfile |
| R-9 | A backend added later calls a VPP plugin API and declares no startup feature | A feature works against a lab VPP and fails on a managed one | `TestBackendPluginAPIsAreDeclared` fails the build of that change |
| R-10 | The appliance image is built for an architecture whose bundle is not in the cache | `ze appliance build` cannot embed a bundle | It refuses before writing anything, with the cache path and the command to run on a host of that architecture, the way the runtime kernel does |
| R-11 | A second session edits the VPP component while this spec is implemented (`spec-pppoe-vpp-dataplane` is in progress there) | A hunk conflict in `register.go` or `startupconf.go` | The seam and the plugin declarations are small, separate files. Commit whole files and name the concurrent work, per the shared-checkout rule |
| R-12 | Scratch cleanup removes the cached bundle, and the next image build rebuilds VPP | A build that was warm runs a container for a long time | The bundle's cache directory joins `KernelCacheNamespaces`, which cleanup already preserves |
| R-13 | No flavor of the compile gate sets the packaging tag, so the bundle package can stop compiling unnoticed | The first appliance image build after a refactor fails | The bundle package holds one embed file per architecture and one registration, and nothing else. `./le build vpp bundle` ends by compiling it, and the QEMU proof builds an image with it |
| R-14 | Every appliance image build now needs the bundle, so a first build on a machine costs a VPP build | An image build that used to start at once runs a container first | The cache is keyed and preserved, so the cost is paid once per architecture per VPP pin. The refusal for an unreachable bundle names the command |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A managed VPP does not start on a `ze` that carries the runtime, so an appliance configured for the VPP backend forwards nothing. A `ze` without the runtime and the operator-supervised mode are untouched by construction |
| How is it reverted? | Removing `ze_vpp_runtime` from a build's tags returns that build to today's behaviour with no code change. The four plugin declarations are separate commits and revert alone |
| Who else touches this path? | `plan/post-release/features/spec-pppoe-vpp-dataplane.md` (in progress, VPP process ownership), `plan/release/validation/spec-finish-vpp-stub.md`, and the release fixes named `spec-vpp-*` and `spec-firewall-vpp-apply-before-connect` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A `vpp` config with `enabled true`, on a `ze` whose runtime seam holds a bundle | → | The VPP component unpacks the bundle and the supervisor runs its command | `test/vpp/embedded-runtime-start.ci` |
| The same config on a `ze` with no bundle registered | → | The supervisor runs the host path, config byte-identical to today | `TestManagedStartUsesHostPathWithoutRuntime` |
| `vpp runtime storage memory` in the config | → | The unpack root is the memory filesystem | `test/parse/vpp-runtime-storage.ci`, `TestStorageSettingSelectsUnpackRoot` |
| A firewall rule under `backend vpp` | → | The firewall backend's startup registration selects the ACL plugin and the generated config enables it | `TestFirewallConfigEnablesACLPlugin` |
| `./le build vpp bundle` | → | The bundle build writes an archive and description into the keyed cache | `TestBundleActionWritesCacheEntry` |
| `ze appliance build` for an image with the runtime tag | → | The image build resolves the bundle from the cache or refuses with the command | `TestApplianceBuildResolvesBundle`, `TestApplianceBuildRefusesMissingBundle` |
| An appliance image booted in QEMU with a managed VPP config | → | The image's `ze` starts real VPP from its own bundle | `./le test qemu vpp-embedded` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An appliance image built by `ze appliance build` with no VPP installed anywhere, booted with a config that enables the VPP component in the mode Ze manages | The image's own `ze` starts VPP from the bundle it carries, the VPP API answers with the bundle's VPP version, and Ze publishes the VPP `connected` event | The operator-supervised mode stays as it is: there Ze runs and unpacks nothing, because another supervisor owns VPP |
| AC-2 | A Linux container image holding only a runtime-carrying `ze` and its config, with no C library and no VPP | `ze` starts VPP from its bundle and the API answers | A host with its own VPP installed is unaffected: a runtime-carrying `ze` does not read the host path |
| AC-3 | A `ze` built without `ze_vpp_runtime`, config enabling a managed VPP, a VPP program at the standard host path | `ze` runs the host program, and the config file it generates is byte-identical to the one generated before this spec | This fallback is permitted by owner decision, for builds that carry no runtime only |
| AC-4 | A development build of `ze` and the VPP component's test binaries, on a machine whose bundle cache is empty | Each builds and its VPP tests pass with no VPP build started, and the built `ze` is the size it was before this spec, within the size of the new Go code. A build without `ze_vpp` still refuses a `vpp` config at parse | Only an appliance image build or a release build embeds the bundle, because only those set the packaging tag |
| AC-5 | A second managed start on the same host, the unpacked directory intact | VPP starts from the existing directory after every file is verified, and the files on disk keep their modification times | A reused directory is permitted because each file's checksum matched the description |
| AC-6 | One unpacked file altered or deleted before a start | The file is restored from the archive, VPP starts, and the event is reported under its own doctor code with the file's path | The other files stay as they are, because they verified |
| AC-7 | `vpp runtime storage memory` | The bundle is unpacked under the memory filesystem and VPP starts from there, and the persistent directory holds no bundle | Disk storage stays the default because it works on hosts whose memory filesystems forbid execution |
| AC-8 | The unpack directory's filesystem forbids execution, or has less free space than the description's total size | The start is refused before any file is written, with an error that names the directory, the cause and the `storage` setting that fixes it. `ze doctor` reports the same condition before a start | A start on a suitable directory is unaffected |
| AC-9 | A config with a firewall rule, a NAT rule, a VXLAN interface or a GRE tunnel under the VPP backend, on a managed VPP | The generated config enables that feature's plugin, VPP loads it, and the rule or interface is applied and can be read back from VPP | A config that uses none of them enables none of the four, because a plugin Ze does not need is not loaded |
| AC-10 | The same four features on a VPP whose plugin is not loaded (an operator-supervised VPP started without it) | `ze doctor` reports the missing plugin by name with the feature that needs it and the fix, before the config is applied | A VPP that has the plugin loaded reports nothing |
| AC-11 | A backend package links a VPP plugin's API package and registers no startup feature for it | `TestBackendPluginAPIsAreDeclared` fails and names the package and the API | APIs that belong to VPP's core are permitted without a declaration, because no plugin has to load for them |
| AC-12 | `./le build vpp bundle` run twice with unchanged inputs | The second run answers with the cached entry and starts no container. The cache holds the archive, its description and the source archive of every LGPL component | A run with a changed input (source pin, a patch, the plugin list, the Dockerfile) builds again, because its key differs |
| AC-13 | `ze appliance build` for an architecture whose bundle is not cached | The build is refused before anything is written, naming the cache path and ending with the command to run on a host of that architecture | A warm cache starts no container |
| AC-14 | The bundle's description | It lists every file with its checksum and names the licence of every component, and the archive carries each licence text. The bundle's README quotes the distribution clause of each licence it relies on | None |
| AC-15 | The QEMU proof repeated with a CPU model other than the build host's | VPP starts and the API answers | None |
| AC-16 | `vpp.lcp` enabled on a managed VPP on the appliance, where no `vppctl` program exists | The linux-cp plugin check answers from the running VPP's API message table and reports the plugin loaded | The check keeps reporting "unknown" when the API socket cannot be reached, because that is not evidence about plugins |
| AC-17 | An operator reads the VPP guide, the appliance guide and the `vpp` config help after this work | Each describes the embedded runtime, the `storage` setting, the fallback for builds without the runtime, and how to tell which VPP a `ze` runs. The PPPoE diagnostic's fix names the bundle, and the config help describes where the program comes from in place of a fixed host path | The operator-supervised section stays, because that mode stays |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds an appliance image and boots it with the VPP component enabled | image build resolves the bundle → `ze` embeds it → VPP component unpacks to `/perm/ze` → supervisor runs it → `connected` | `./le test qemu vpp-embedded` |
| 2 | Runs a release `ze` on a plain Linux host with no VPP installed | VPP component unpacks under Ze's directory → supervisor runs it | `./le test deployment vpp-embedded` |
| 3 | Adds a firewall rule under the VPP backend on that appliance | firewall registration selects the ACL plugin → generated config enables it → VPP loads it → rule applied | `./le test qemu vpp-embedded` (firewall step) |
| 4 | Sets `vpp runtime storage memory` on a box with little disk | setting parsed → unpack root is memory → VPP runs from there | `test/vpp/embedded-runtime-memory.ci` |
| 5 | Runs `ze doctor` on a box whose unpack directory forbids execution | doctor check reads the mount flags → reports the cause and the setting | `test/vpp/embedded-runtime-noexec.ci` |
| 6 | Bumps the pinned VPP version | `./le build vpp bundle` builds a new entry → image build embeds it → old unpacked directory removed at next start | `TestUnpackRemovesStaleSiblings`, `TestBundleCacheKeyChangesWithPin` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnpackWritesEveryFileWithItsMode` | `internal/core/programbundle/unpack_test.go` | A described archive becomes a directory holding each file with its mode | |
| `TestUnpackRefusesDescriptionMismatch` | `internal/core/programbundle/unpack_test.go` | An archive whose file differs from its described checksum is refused | |
| `TestUnpackRefusesPathOutsideRoot` | `internal/core/programbundle/unpack_test.go` | An entry naming a path outside the unpack directory, or a link, is refused | |
| `TestUnpackReusesVerifiedDirectory` | `internal/core/programbundle/unpack_test.go` | A complete directory is verified and no file is rewritten | |
| `TestUnpackRepairsAlteredFile` | `internal/core/programbundle/unpack_test.go` | One altered file is restored and reported | |
| `TestUnpackSurvivesInterruptedWrite` | `internal/core/programbundle/unpack_test.go` | A leftover temporary directory is not taken for a complete one | |
| `TestUnpackRemovesStaleSiblings` | `internal/core/programbundle/unpack_test.go` | Sibling directories of other bundles are removed after success | |
| `TestUnpackRefusesInsufficientSpace` | `internal/core/programbundle/unpack_linux_test.go` | Free space below the described total refuses before any write | |
| `TestUnpackRefusesNoExecMount` | `internal/core/programbundle/unpack_linux_test.go` | A directory on a mount that forbids execution is refused | |
| `TestCommandUsesBundledLoader` | `internal/core/programbundle/command_test.go` | A description with a loader yields loader, library directory, program | |
| `TestCommandForStaticProgram` | `internal/core/programbundle/command_test.go` | A description with no loader yields the program alone | |
| `TestManagedStartUsesRegisteredRuntime` | `internal/component/vpp/runtime_source_test.go` | With a bundle registered, the supervisor is handed the bundle's command | |
| `TestManagedStartUsesHostPathWithoutRuntime` | `internal/component/vpp/runtime_source_test.go` | With none registered, the supervisor is handed the host path | |
| `TestExternalModeNeverUnpacks` | `internal/component/vpp/runtime_source_test.go` | `external true` asks the seam for nothing | |
| `TestStartupConfNamesBundlePluginDirectory` | `internal/component/vpp/managed_startup_test.go` | The plugins section carries the bundle's plugin directory | |
| `TestStartupConfUnchangedForHostProgram` | `internal/component/vpp/managed_startup_test.go` | Output for the host path is byte-identical to the recorded text | |
| `TestStorageSettingSelectsUnpackRoot` | `internal/component/vpp/config_test.go` | `disk` and `memory` select their roots; the default is `disk` | |
| `TestBuiltinPluginsDeclaredOnce` | `internal/component/vpp/managed_startup_test.go` | The generator's built-in plugins and the exported list are the same declaration | |
| `TestStartupFeatureWithoutSection` | `internal/component/vpp/startup_feature_test.go` | A feature with no config section registers and writes no section | |
| `TestManagedStartThroughLoaderBindsWitness` | `internal/component/vpp/process_managed_integration_linux_test.go` | A child started through a loader is the process the witness binds | |
| `TestRuntimeDirectoryDoctorCheck` | `internal/component/vpp/doctor_linux_test.go` | The check reports a directory that forbids execution or lacks space, against fixtures | |
| `TestFirewallConfigEnablesACLPlugin` | `internal/plugins/firewall/vpp/startup_test.go` | A firewall rule under the VPP backend selects the ACL plugin | |
| `TestNATConfigEnablesNATPlugin` | `internal/plugins/firewall/vpp/startup_test.go` | A NAT rule selects the NAT plugin; a config with no NAT does not | |
| `TestVXLANConfigEnablesVXLANPlugin` | `internal/plugins/iface/vpp/startup_test.go` | A VXLAN interface selects the VXLAN plugin | |
| `TestGREConfigEnablesGREPlugin` | `internal/plugins/iface/vpp/startup_test.go` | A GRE tunnel selects the GRE plugin | |
| `TestLCPPluginCheckReadsAPITable` | `internal/plugins/iface/vpp/doctor_test.go` | The linux-cp check answers from the message table and runs no program | |
| `TestBackendPluginAPIsAreDeclared` | `internal/le/build/vpp/declared_test.go` | Every plugin API a VPP backend links has a startup registration | |
| `TestBundlePluginListMatchesDeclarations` | `internal/le/build/vpp/bundle_test.go` | The committed plugin list equals the built-in list plus the registry | |
| `TestBundleActionWritesCacheEntry` | `internal/le/build/vpp/bundle_test.go` | The action writes archive, description and source archive under the key | |
| `TestBundleCacheKeyChangesWithPin` | `internal/appliance/vppbundle_test.go` | Each input (pin, patch, plugin list, Dockerfile, architecture) changes the key | |
| `TestBundleCacheIsPreservedByCleanup` | `internal/appliance/vppbundle_test.go` | The bundle's cache directory is among the preserved namespaces | |
| `TestApplianceBuildResolvesBundle` | `internal/appliance/vppbundle_test.go` | A warm cache places the bundle and starts no build | |
| `TestApplianceBuildRefusesMissingBundle` | `internal/appliance/vppbundle_test.go` | A cold cache for another architecture refuses with the command | |
| `TestOnlyPackagingBuildsLinkTheBundle` | `internal/le/build/vpp/bundle_test.go` | No package outside the bundle package imports it, and no flavor of the compile gate sets its tag | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| A file's size against its described size | exact match | the described size | one byte fewer | one byte more |
| Free space against the description's total | at least the total | exactly the total | one byte fewer | N/A |

The `storage` setting is an enumeration and takes no number.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `embedded-runtime-start` | `test/vpp/embedded-runtime-start.ci` | A `ze` holding a test bundle of the VPP stub program unpacks it, runs it and reports VPP connected | |
| `embedded-runtime-memory` | `test/vpp/embedded-runtime-memory.ci` | With `storage memory` the stub runs from the memory filesystem | |
| `embedded-runtime-repair` | `test/vpp/embedded-runtime-repair.ci` | A deleted unpacked file is restored and reported, and the stub runs | |
| `embedded-runtime-noexec` | `test/vpp/embedded-runtime-noexec.ci` | `ze doctor` names a directory that forbids execution and the setting to change (`option=needs-linux`) | |
| `vpp-runtime-storage` | `test/parse/vpp-runtime-storage.ci` | Both values parse; an unknown value is refused with the valid ones | |
| `vpp-embedded` | `./le test qemu vpp-embedded` | A real image boots, its `ze` starts real VPP, a firewall rule is applied and read back, and the run repeats on a second CPU model | |
| `vpp-embedded` | `./le test deployment vpp-embedded` | A container with no C library runs `ze` and real VPP starts | |

The stub tests need a `ze` that carries a test bundle. The test fixture registers a small bundle holding the existing Go VPP stub through the same seam the real bundle uses, so the unpack, the command and the supervisor are exercised with no VPP build and without the packaging tag.

## Files to Modify
- `internal/component/vpp/register.go` - `runConfiguredVPP` asks the runtime seam and hands the supervisor a command
- `internal/component/vpp/vpp.go` - the supervisor runs a command of several arguments
- `internal/component/vpp/startupconf.go` - plugin directory in the plugins section; built-in plugin names read from one declaration
- `internal/component/vpp/startup_feature.go` - the config section becomes optional
- `internal/component/vpp/config.go` - the `storage` setting in `VPPSettings`
- `internal/component/vpp/yang/ze-vpp-conf.yang` - container `runtime`, leaf `storage`, enumeration `disk` / `memory`, default `disk`; the `vpp` container's description stops naming a fixed host path
- `internal/component/vpp/doctor_linux.go` - the unpack directory check
- `internal/core/diagnostic/codes.go` - the new codes
- `internal/component/l2tp/plugins/pppoe-vpp/startup.go` - the diagnostic's fix text names the bundle
- `internal/plugins/iface/vpp/doctor.go` - the linux-cp check reads the API table; the `vppctl` probe is deleted
- `internal/le/build/vpp/register.go` - the `bundle` action
- `internal/appliance/cache.go`, `internal/appliance/cmd_build.go` - bundle cache path, preserved namespace, and the reachability refusal in `buildOne`
- `gokrazy/ze/config.json` - `ze_vpp_runtime` in `GoBuildTags`
- `.gitignore` - the bundle package's data directory
- `third_party/vpp-patches/README.md` - the bundle build, its plugin list, the licences read
- `docs/guide/vpp.md`, `docs/guide/appliance.md`, `docs/guide/configuration.md` - the runtime, the setting, the fallback; the sidecar advice removed
- `docs/research/vpp-deployment-reference.md` - the design page the VPP component's files declare
- `docs/architecture/appliance/build-artifacts.md` - the bundle as a cached artifact, with its measured sizes
- `docs/architecture/plugin/feature-gates.md` - the packaging tag, and why it is not a feature gate
- `docs/architecture/doctor-and-health-checks.md` - the new checks
- `docs/architecture/vpp-host-tuning.md` - declared by `doctor_linux.go`: the unpack directory check joins the host readiness checks it lists
- `docs/architecture/l2tp/bng-5-pppoe.md` - declared by the PPPoE startup registration: wherever it tells the operator to build and install the patched VPP, it names the embedded bundle instead
- `docs/labs/pppoe-interop.md` - declared by `internal/le/build/vpp/register.go`: the lab image and `./le build vpp run` are unchanged, and the page gains one line saying the product bundle is built from the same pin by the new action
- `docs/features/ai-first.md` - declared by `internal/core/diagnostic/codes.go`. Not read for this spec. Expected unaffected, because the work adds entries to the existing diagnostic catalogue and changes none of its rules; the phase that adds the codes reads the page and edits it if that expectation is wrong
- `ai/digests/vpp-dataplane.md`, `ai/INDEX.md` - the flow and the keywords
- `plan/release/delivery/spec-release-distribution.md` - the release build sets the packaging tag, and publishes the LGPL source archive beside each artifact that carries the bundle

## Files to Create
- `internal/core/programbundle/` - the description format, unpack, verify, repair, stale removal, the command; no VPP knowledge
- `internal/component/vpp/runtime_source.go` - the seam: register a bundle, ask for one
- `internal/component/vpp/runtime/` - the bundle package behind `ze_vpp_runtime`: one embed file per architecture, `register.go`, and a git-ignored `data/` directory
- `internal/plugins/firewall/vpp/register_startup.go` - the ACL and NAT declarations
- `internal/plugins/iface/vpp/register_startup.go` - the VXLAN and GRE declarations
- `internal/le/build/vpp/bundle.go` - the bundle build and the plugin list writer
- `internal/appliance/vppbundle.go` - cache key, path and resolution
- `third_party/vpp-patches/Dockerfile.bundle` - the product build
- `third_party/vpp-patches/bundle-plugins.txt` - the plugin list, written by the tooling from the declarations
- `internal/le/test/qemu/vppembedded.go` - the QEMU proof
- `internal/le/test/deployment/vppembedded.go` - the container proof
- `test/vpp/embedded-runtime-start.ci`, `embedded-runtime-memory.ci`, `embedded-runtime-repair.ci`, `embedded-runtime-noexec.ci`
- `test/parse/vpp-runtime-storage.ci`
- The unit test files named in the test plan

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/vpp/yang/ze-vpp-conf.yang`: container `runtime`, leaf `storage` |
| YANG validation constraints | Yes | An enumeration of two values with a default; each value carries `ze:help` |
| YANG custom validators | N-A | The enumeration is the whole constraint |
| CLI commands/flags | Yes | `./le build vpp bundle` in `internal/le/build/vpp/register.go`. No `ze` command is added |
| CLI grammar (keyword before value) | Yes | The action takes no positional value; `./le cli grammar` judges it |
| Editor autocomplete | Yes | Automatic for the enumeration |
| Functional test for new RPC/API | N-A | No RPC is added |
| Pipe completeness | N-A | No `ze` command output is added |
| Env var registration | Yes | No operator variable: the setting is a YANG leaf under `vpp`, not under `environment`. One private test variable overrides the roots the directory check reads, registered beside the check the way the hugepage check registers its own |
| Doctor check for runtime dependencies | Yes | The unpack directory (execution allowed, free space) in `internal/component/vpp/doctor_linux.go`, with unit and `.ci` tests |
| Doctor code per problem class | Yes | `doctor-vpp-runtime-directory` (execution forbidden or too little space, before start); `doctor-vpp-runtime-unpack` (a write failed, at start); `doctor-vpp-runtime-repaired` (a file was restored, at start); `doctor-vpp-acl-plugin`, `doctor-vpp-nat-plugin`, `doctor-vpp-vxlan-plugin`, `doctor-vpp-gre-plugin` (API absent from the running VPP, before apply), each registered by the package that owns the feature |
| Prometheus counters/metrics | N-A | A one-time step at start whose outcome is a doctor finding, not a rate |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The VPP feature page under `features/`, found with `./le feature report` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/guide/vpp.md` |
| 3 | CLI command added/changed? | No | No `ze` command changes. The `le` action is documented by its own help |
| 4 | API/RPC added/changed? | No | None added |
| 5 | Plugin added/changed? | No | No plugin is added; four backends gain a startup declaration, covered by row 15 |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md`, `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | No protocol wire changes |
| 8 | Plugin SDK/protocol changed? | No | The seam is inside the VPP component, not in `pkg/` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md` and `docs/functional-tests.md` for the two new proofs and the stub bundle fixture |
| 11 | Affects daemon comparison? | No | No comparison row names VPP packaging |
| 12 | Internal architecture changed? | Yes | `docs/research/vpp-deployment-reference.md`, `docs/architecture/appliance/build-artifacts.md`, `docs/architecture/plugin/feature-gates.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/architecture/doctor-and-health-checks.md` for the new codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/release/delivery/spec-vpp-embedded-runtime.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/vpp.md`: the sidecar advice and every example of a managed start |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the seam, the setting, failing wiring tests
   - Tests: `test/vpp/embedded-runtime-start.ci`, `TestManagedStartUsesRegisteredRuntime`, `TestManagedStartUsesHostPathWithoutRuntime`, `test/parse/vpp-runtime-storage.ci`
   - Files: `runtime_source.go`, the YANG leaf, the test fixture that registers a stub bundle
   - Verify: the seam exists and the config parses. The start test fails because nothing unpacks. A-15 settled with `./le repo feature-tags check`
2. **Phase: Unpack library** -- `internal/core/programbundle`
   - Tests: every `TestUnpack*` and `TestCommand*` row
   - Files: the new package
   - Verify: tests fail, implement, tests pass
3. **Phase: VPP component integration** -- the supervisor runs a command; the plugin directory; the fallback
   - Tests: `TestStartupConfNamesBundlePluginDirectory`, `TestStartupConfUnchangedForHostProgram`, `TestStorageSettingSelectsUnpackRoot`, `TestExternalModeNeverUnpacks`, `TestBuiltinPluginsDeclaredOnce`, `TestManagedStartThroughLoaderBindsWitness`, `TestRuntimeDirectoryDoctorCheck`, the four stub `.ci` tests
   - Files: `register.go`, `vpp.go`, `startupconf.go`, `config.go`, `doctor_linux.go`, `codes.go`
   - Verify: the wiring test passes with the stub bundle
4. **Phase: Bundle build** -- the product build, the cache, the bundle package, the measurements
   - Tests: `TestBundleActionWritesCacheEntry`, `TestBundleCacheKeyChangesWithPin`, `TestBundleCacheIsPreservedByCleanup`, `TestBundlePluginListMatchesDeclarations`, `TestOnlyPackagingBuildsLinkTheBundle`
   - Files: `Dockerfile.bundle`, `bundle.go`, `vppbundle.go`, `bundle-plugins.txt`, the bundle package, the README
   - Verify: one bundle per architecture built natively. A-4 measured and recorded, A-7, A-12 and A-14 settled by reading, before the next phase
5. **Phase: Appliance and real VPP** -- the image carries the runtime; the proofs
   - Tests: `TestApplianceBuildResolvesBundle`, `TestApplianceBuildRefusesMissingBundle`, `./le test qemu vpp-embedded`, `./le test deployment vpp-embedded`
   - Files: `gokrazy/ze/config.json`, `cmd_build.go`, `vppembedded.go` (both)
   - Verify: real VPP starts in QEMU and in the container. A-3, A-5, A-8, A-9 and A-10 settled. Each failure found is fixed here
6. **Phase: Plugin declarations** -- ACL, NAT, VXLAN, GRE; the linux-cp check
   - Tests: the four plugin-selection rows, `TestStartupFeatureWithoutSection`, `TestBackendPluginAPIsAreDeclared`, `TestLCPPluginCheckReadsAPITable`, the firewall step of the QEMU proof
   - Files: the two `register_startup.go` files, `startup_feature.go`, `iface/vpp/doctor.go`, the PPPoE diagnostic text
   - Verify: the firewall step is run before the declaration and recorded red (A-11), then green after. Each declaration is its own commit
7. **Phase: Release publication** -- the release build's tag and the LGPL source archive
   - Tests: the release artifact list test that `spec-release-distribution` names
   - Files: `plan/release/delivery/spec-release-distribution.md` and the artifact list it owns
   - Verify: the release build sets the packaging tag, and the release artifact set names the source archive beside each bundle-carrying artifact

Each phase edits the pages its changes make wrong before the next phase starts (`ai/rules/documentation.md`).

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| The fallback is exactly the owner's | The host path is reachable only when no bundle is registered. No code path tries the bundle and then the host path, or the reverse |
| The tag stays a packaging tag | `ze_vpp_runtime` is absent from `feature-gates.txt` and from every flavor of `buildMatrix`. The VPP component compiles and passes its tests without it. Nothing outside the bundle package imports it |
| No VPP knowledge in the unpack library | `internal/core/programbundle` names no VPP file, plugin or path |
| One declaration per plugin | No plugin file name appears in two Go literals. The bundle's list is written from the declarations |
| Unpack safety | No entry writes outside the unpack directory; links are refused; the rename is the only way a directory appears under its final name |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Licence | Each bundled component's licence is named in the description and its text is in the archive. Each distribution clause the README relies on is quoted from the text |
| Rule: `ai/rules/platform-linux.md` | Linux-only code carries a QEMU proof and no host binary is cross-compiled |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| An appliance image whose `ze` starts real VPP | `./le test qemu vpp-embedded` |
| A plain Linux `ze` that starts real VPP with nothing installed | `./le test deployment vpp-embedded` |
| The bundle in the cache for each supported architecture | `./le build vpp bundle` answers "cached" on the second run |
| Development and test builds carry no bundle | `TestOnlyPackagingBuildsLinkTheBundle` |
| The four plugin declarations | `TestBackendPluginAPIsAreDeclared` |
| Measured bundle sizes on the build-artifacts page | Read the page; the numbers match the description's totals |
| The guide no longer advises a sidecar image | `./le doc check all` and a read of `docs/guide/vpp.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Archive entries | Paths are relative, stay inside the unpack directory, and are regular files only |
| Directory ownership and modes | The unpack directory and its files are writable by Ze's own user only |
| Verification before execution | Every file is checked against the description at each managed start, before the program runs |
| Build provenance | The VPP source archive checksum is verified, patches apply with no fuzz, the base image is pinned by digest |
| Error text | A refusal names paths and causes and never prints file contents |
| What runs as root | The bundle's program runs with the privileges VPP already has today; this spec adds none |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A-3 proves false (the loader cannot be run directly) | DESIGN: the bundle needs build-time path rewriting, which fixes the unpack location and changes AC-7 |
| A-4 exceeds 100 MB packed | Stop and report the measured sizes to the owner |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The session that opened this spec first recommended shipping the program as files in the appliance image. The owner chose embedding instead, because it serves a plain Linux install of `ze` as well as the appliance.
- Running a program from memory (a sealed in-memory file) fits one static program. It does not fit VPP, which is a tree of files that load each other by name. The description allows a static program with no loader, so a later single-binary program can use the same library.
- The proofs of firewall and tunnels on real VPP all ran against a VPP the lab started with every plugin on. A feature proven only against a permissive environment says nothing about the restrictive one Ze itself creates.
- A stock VPP is not enough for Ze: the PPPoE backend refuses one. That makes a Ze-built VPP the only one that satisfies every backend, which is the practical reason to carry it.
- A feature gate and a packaging tag look alike and behave differently. A gate is appended to every development and test build, so anything heavy behind a gate is paid for everywhere. The first draft of this design declared the runtime as a gate; reading the flavor matrix corrected it.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The VPP runtime is embedded in the `ze` binary | Files in the appliance image; a container image run by Ze | Owner decision, 2026-11-19: one `ze` runs VPP with nothing pre-installed, on the appliance and on plain Linux |
| The bundle sits behind a packaging tag, `ze_vpp_runtime`, set only by image and release builds | Behind the `ze_vpp` feature gate; behind a new feature gate | Either gate is appended to every development and test build, so each would embed the bundle, and each would fail to compile on a machine that has not built it |
| The bundle is built and cached, not committed | Committing the packed bundles | Owner decision. Git history stays small; the kernel cache already gives the reuse rule |
| The loader is run directly with the library directory | Rewriting paths inside every binary at build time | Rewritten paths fix the unpack location for good, which rules out the owner's disk-or-memory choice |
| The unpack location is an operator setting, default disk | Disk only; memory only | Owner decision. Disk works on every host; memory avoids a permanent path where disk is scarce |
| The host path stays for a `ze` without the runtime | Removing it, so managed always means embedded | Owner decision, an exception to the no-fallback rule. The two never mix in one build |
| Plugins are declared in the existing startup feature registry, and the bundle's list derives from it | A plugin list written by hand beside the build | One declaration: a backend cannot call a plugin the bundle lacks, and the list cannot drift |
| The linux-cp check reads the API message table | Shipping `vppctl` in the bundle to keep the probe | The registry already provides that check; the appliance has no `vppctl`, and one mechanism replaces two |
| The unpack library is generic and holds no VPP knowledge | A VPP-only unpacker inside the component | Owner decision on reuse: a second program uses the same library with its own bundle |

## Known Limitations
- One VPP version per `ze`. An operator who needs another VPP runs it themselves in the operator-supervised mode.
- A bundle exists only for the architectures Ze builds a runtime kernel for. A `ze` for another architecture is built without the runtime.
- The switch chip daemon is not embedded by this spec. No spec exists for it yet; it is the owner's to open.
- Setting the packaging tag in the release build and publishing the LGPL source archive are done through `plan/release/delivery/spec-release-distribution.md`, which this spec amends in its last phase.

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
- [ ] Every user story has a working path and a passing test
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
