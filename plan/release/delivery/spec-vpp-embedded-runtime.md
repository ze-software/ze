# Spec: vpp-embedded-runtime

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P2 |
| Release reason | Owner decision (2026-11-19, carrier changed 2026-11-28): VPP on the appliance is owed for the first release. Today an appliance whose config selects the VPP backend has no VPP program to start, so the release artifact cannot do what its VPP configuration asks. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The file keeps its name because other specs and pages cite it. Since 2026-11-28 nothing in this design is embedded: the name is history.

## Task

Ze starts and supervises VPP itself, but it does not carry VPP. The VPP component runs the program at a fixed host path, `/usr/bin/vpp`, and expects an operator to have installed it there. The gokrazy appliance has no package manager and no C library, so nothing can put VPP at that path: an appliance configured for the VPP backend cannot start its data plane. On a plain Linux host the operator installs a distribution VPP, which the PPPoE backend then refuses, because it needs Ze's patched build.

Goal: Ze's own VPP build is always installed with Ze. The image build places it in the appliance image. A Ze package installs it on Linux. Ze finds it by scanning its own folder and starts it. Ze starts only that VPP: with none in its folder it starts none (owner, 2026-11-28). That VPP is one version, v26.06, carrying every Ze patch (owner, 2026-11-28).

A second defect is fixed here because the goal depends on it. A VPP that Ze starts loads only the plugins Ze names, and Ze names none for firewall rules, NAT, VXLAN tunnels or GRE tunnels. Ze's backends call the APIs of those plugins. So on a VPP that Ze started, that configuration is accepted and then refused by VPP when applied. It is unnoticed today because the only proofs of those features run against a VPP the lab starts itself with every plugin on.

### What the owner decided

His messages are voice-transcribed and are read for their meaning.

| Date | His words | What it settles here |
|------|-----------|----------------------|
| 2026-11-19 | (recorded by the first version of this spec) | Vendoring a VPP build is accepted. The work is owed for the first release. The bundle is built and cached like the appliance kernel and is not committed. The undeclared plugins are fixed in this spec. A second outside program (the switch chip daemon) must be able to use the same shape later |
| 2026-11-28 | "on the appliance we should manage the VPP binary. Can we generate it and add it to the file system ?" and "VPP bundle placed in the image's file system" | Appliance: the image build places the built bundle as files in the image's root file system. This withdraws the 2026-11-19 decision that the runtime lives inside the `ze` binary |
| 2026-11-28 | "So we should always install our VPP with the software." | Every installation of Ze carries Ze's VPP build |
| 2026-11-28 | "for linux it should be installed in a ze folder for binaries following the LSB" | Linux: the files go in a Ze-owned folder for internal binaries, placed as the standard hierarchy says |
| 2026-11-28 | "ok for having a .deb for installation too follow the usal debian packaging method in repos", "One package", "you can do rpm too" | One package carries `ze` and the VPP files. The DEB and the RPM get the same treatment. There is no second package, so no relation between packages and no version coupling |
| 2026-11-28 | "same for vpp install it in the same folder as ze and make sure ze will scan that folder" | VPP sits in the same Ze folder as `ze`, and Ze finds it by scanning that folder, not by a fixed path |
| 2026-11-28 | "install following the FSB on a linux FS (probably /usr/local/) and then add a script in /usr/local/bin so we can have multiple version installed and change based on a env variable", "old version are kept the ze script providing an uninstall option a bit like we do with ./le", and "lgtm" to a version listing and a version uninstall | An install that is not a package lives under `/usr/local`, one directory per version holding `ze` and its VPP together, with a launcher script. That design is not in this spec (Not in scope). This spec keeps one rule so that it works there: Ze finds VPP relative to the `ze` that runs |
| 2026-11-28 | Asked whether a VPP at `/usr/bin/vpp` is ever started when Ze finds none in its folder: "No, the only start is on VPP." | Ze starts only its own VPP. With none in its folder, VPP stays down and `ze doctor` gives the fix. The fallback to `/usr/bin/vpp` kept on 2026-11-19 is withdrawn, and the constant path is deleted |
| 2026-11-28 | Asked where the Ze folder is (A: `ze` at `<prefix>/bin/ze` and the folder at `<prefix>/lib/ze`; B: `ze` itself inside the folder, with `/usr/bin/ze` a link; C: `/usr/libexec/ze`): "Neither, like B, but the /usr/bin/ze is a script, not a symlink." | The real `ze` sits inside the Ze folder with `vpp` beside it. The entry in the bin directory is a launcher script, which `spec-release-build` owns |
| 2026-11-28 | Asked whether a package install uses the same layout as `/usr/local`, with the example `/usr/bin/ze` (the launcher), `/usr/lib/ze/2026.11.28/ze` and `/usr/lib/ze/2026.11.28/vpp/`: "a" | On a DEB or RPM install the Ze folder is `/usr/lib/ze/<release>/`. The example he was shown used a four-digit year, which he reversed the same day: the directory is named `YY.MM.DD` or `YY.MM.DD.N` |
| 2026-11-28 | Asked which patched VPP is ours ((a) v26.06 with the IPsec patch; (b) v25.10 with the PPPoE patches; (c) both patch sets on one version before the first release): "Oh, we should port all our patches to version 26 and use that version everywhere. C" | Every Ze patch is ported to v26.06 before the first release, and v26.06 is the one VPP Ze builds, ships and tests against. The port is a prerequisite of the bundle and is this spec's first phase. "Everywhere" includes the GoVPP bindings vendored from upstream and the `third_party/vpp-linux-cp` reference copy (choice N-7, taken as (a)); only the port moves into the first release, not the PPPoE feature's acceptance (choice N-8, taken as (a)) |
| 2026-11-28 | Asked whether every image carries VPP, test images included ((a) yes, one path and no setting; (b) a field in the appliance config leaves it out): "A" | Every appliance image carries the bundle, and no field leaves it out. The size check of the image root (AC-7) is this design's consequence, not his words |
| 2026-11-28 | "The automatic update should only be for Linux, for the appliance it should be immutable infrastructure." | On the appliance `ze` and its VPP are files of the image, in one version, and they change only when the image is replaced. This design places no version directory and no launcher there |
| 2026-11-28 | Not asked (the earlier choice C-3). Asked who installs a self-update: "C We do not require root users to run Ze. It could be run as a Ze user, provided the capability required to run the programs is available." | An update on plain Linux installs a version directory, `ze` and its VPP together, so no `ze` runs beside the VPP of another release. `spec-release-build` owns that update; nothing in between is built here |

### The standards he named

He said LSB. The LSB text itself was not read for this spec (its site refused the fetch). The clauses below were read on 2026-11-28 from the FHS 3.0 text as Debian publishes it, and from the Debian Policy source, chapters "The Operating System" and "Declaring relationships between packages".

| Source | Clause, quoted | Use here |
|--------|----------------|----------|
| FHS 3.0, 4.6.1 | "/usr/lib includes object files and libraries. On some systems, it may also include internal binaries that are not intended to be executed directly by users or shell scripts." | VPP is an internal binary: Ze runs it, an operator does not |
| FHS 3.0, 4.6.1 | "Applications may use a single subdirectory under /usr/lib. If an application uses a subdirectory, all architecture-dependent data exclusively used by the application must be placed within that subdirectory." | The whole bundle (program, plugins, libraries) sits under the one Ze subdirectory |
| FHS 3.0, 4.7.1 | "/usr/libexec includes internal binaries that are not intended to be executed directly by users or shell scripts. Applications may use a single subdirectory under /usr/libexec." and "Applications which use /usr/libexec in this way must not also use /usr/lib to store internal binaries" | The alternative home. The bundle mixes a program with libraries and plugins, so splitting it over both trees is what this clause forbids. Decided on 2026-11-28: the folder is under `/usr/lib/ze` |
| FHS 3.0, 4.9.1 | "Locally installed software must be placed within /usr/local rather than /usr unless it is being installed to replace or upgrade software in /usr." | An install that is not a package goes under `/usr/local` |
| Debian Policy, 9.1.1 | "The location of all files and directories must comply with the Filesystem Hierarchy Standard (FHS), version 3.0, with the exceptions noted below" | The DEB follows the FHS clauses above |
| Debian Policy, 9.1.1, exception 1 | "a subdirectory of ``/usr/lib`` may be used by a package (or a collection of packages) to hold a mixture of architecture-independent and architecture-dependent files." | The manifest and licence texts may sit beside the binaries |
| Debian Policy, 7.6.1 | "It is usually an error for a package to contain files which are on the system in another package." | Ze's VPP is never placed at `/usr/bin/vpp`, where a distribution's `vpp` package installs |

Debian Policy 9.1.1 does not mention `/usr/libexec` at all (searched, no match).

### Not in scope

| Item | Why | Where it lives |
|------|-----|----------------|
| Assembling the DEB, the RPM and the tarball | `spec-release-build` owns package assembly. This spec produces the bundle archive and names the amendment that spec owes | `plan/release/delivery/spec-release-build.md`, `plan/release/delivery/spec-release-distribution.md` |
| The versioned install: one directory per version under `/usr/local/lib/ze` and `/usr/lib/ze`, the launcher script, the version listing and removal, the tarball, and a self-update that installs a version directory | A design of its own, none of it VPP. It was written into the spec that owns the tarball, the packages and the self-update refusal on 2026-11-28 | `plan/release/delivery/spec-release-build.md` |
| A second program in the Ze folder (a switch chip daemon) | Owner decision: reusable shape only | None yet; the owner opens it |
| Changing how Ze supervises VPP once it runs | The supervisor exists | `plan/post-release/features/spec-vpp-failure-lifecycle.md` |
| New VPP features | The four plugin declarations make existing backends work on a managed VPP | - |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every change respects
  → Constraint: a readiness check is a doctor check registered by the package that owns the dependency
  → Constraint: a feature registers and is discovered. The code that scans the Ze folder names no program: a feature declares the program it needs
- [ ] `ai/doors/forwarding.md` - the VPP component and its backends
  → Constraint: the `ze_vpp` tag removes `internal/component/vpp` and every VPP backend. A build without it still compiles
  → Constraint: a backend that cannot apply a config exactly refuses it at verify. A plugin that is not loaded is that case
- [ ] `ai/doors/appliance.md` - the gokrazy image and the device
  → Constraint: an image in the field is the contract for any path under `/perm/ze/`. This design adds no path there
  → Constraint: `ze appliance build` links no `internal/le` package, so what resolves the bundle for an image build lives in `internal/appliance`
  → Constraint: a gokrazy device does not replace its own `ze`. An image update replaces the root file system, so `ze` and its VPP change together
- [ ] `ai/doors/le-tooling.md` - the build and test tooling
  → Constraint: a shipped `ze` links no `internal/le` package. The manifest reader the daemon uses lives outside `internal/le`
  → Constraint: a `.ci` that needs a Linux kernel feature carries `option=needs-linux`
- [ ] `ai/doors/core.md` - shared libraries and registries
  → Constraint: a core package imports no component. The program registry holds no VPP name
  → Constraint: with nothing registered a core registry answers an error, never an empty list
- [ ] `ai/doors/system-plugins.md` - the firewall backends
  → Constraint: always-on code never imports a gated plugin. The firewall VPP backend's startup declaration stays in its own gated package
- [ ] `ai/doors/hub-engine.md` - build flavors
  → Decision: no build tag is added. The appliance `ze` and the package `ze` are the binaries they are today
- [ ] `docs/architecture/core-design.md`, "Design decisions", row of 2026-11-26 "Ze always starts"
  → Constraint: a missing or wrong bundle never makes `ze start` exit. The VPP service stays down, and the log and `ze doctor` report it under one code with its fix
- [ ] `docs/architecture/appliance/build-artifacts.md`, "The runtime kernel the appliance image boots"
  → Constraint: a cached artifact has two tiers, its cache entry or a native build on a host of the target's architecture. A build for another architecture is refused before it starts, with the command that works
  → Constraint: a test never builds an artifact
- [ ] `third_party/vpp-patches/README.md` and `third_party/vpp-patches-v26.06/README.md`
  → Constraint: Ze holds two patched VPP builds at two versions: v25.10 with the PPPoE series, and v26.06 with the IPsec patch. No build holds both. Decided on 2026-11-28: every patch moves to v26.06, in this spec's first phase
- [ ] `ai/rules/config.md`, `ai/rules/repo-maintenance.md`
  → Constraint: an environment key is registered with `env.MustRegister` beside its reader
  → Constraint: a new runtime dependency owes a registered doctor check, an `ai/INDEX.md` row and its page in the same work

**Key insights:** (minimal context to resume after compaction)
- `runConfiguredVPP` passes the constant `/usr/bin/vpp` to `NewVPPManager`, and `runOnce` runs that path with `-c <config>`. No leaf and no environment key changes it
- VPP is a tree of files: the program, plugins it loads from a directory, and the C libraries they link. Measured on the lab image (arm64, three plugins, no DPDK): about 63 MB unpacked
- gokrazy places a tar archive or a host directory into the image root through `ExtraFilePaths`. The vendored packer is the code `ze appliance build` runs: `internal/appliance/gok_execute.go` imports `github.com/gokrazy/tools/gok` in process
- The image has two root partitions of 500 MB each, fixed in the vendored packer's partition table. Nothing in `overwriteFile` compares the root file system's size with that
- gokrazy puts `ze` at `/user/ze`, so on the appliance "the folder of ze" is gokrazy's own directory, not a Ze one
- `selfUpdater.stageBinary` renames one file over the running executable. No package-managed update backend exists in the code today: `internal/component/config/system` holds the gokrazy backend and the self-update backend only
- `RegisterStartupFeature` is the existing registry where a feature declares its VPP plugin and API messages. PPPoE is its only user
- The three directives the guide names as v26.06 grammar (`default data-size`, `nl-rx-buffer-size`, `main-heap-size`) are also parsed by upstream v25.10 (read at that tag, 2026-11-28)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/vpp/register.go` - `runConfiguredVPP` builds the manager with the constants `defaultConfDir` (`/etc/vpp`) and `defaultVPPBinary` (`/usr/bin/vpp`)
- [ ] `internal/component/vpp/vpp.go` - `NewVPPManager` stores one program path; `runOnce` runs it with `-c <config>`, wires its output, and hands the command to `startManagedProcess`
- [ ] `internal/component/vpp/startupconf.go` - `GenerateStartupConf` writes a `plugins` section that disables every plugin and enables named ones. Built-in names are literals in the function. It names no plugin directory. It writes one config section per selected feature
- [ ] `internal/component/vpp/startup_feature.go` - `RegisterStartupFeature` takes a plugin file name, a mandatory section name, flags, config roots, a predicate, and optionally API messages with a doctor code
- [ ] `internal/component/vpp/startup_lifecycle.go` - `Start` and `replace` start the manager and wait for its startup receipt within `startupReadyTimeout` (45 seconds); `runVPPEngine` returns that error from the plugin's `OnStarted`
- [ ] `internal/component/vpp/config.go` - `VPPSettings`; no setting names where the program comes from
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - the `vpp` container's description says Ze "starts /usr/bin/vpp"; the `external` leaf names a container sidecar
- [ ] `internal/component/vpp/doctor_linux.go` - the hugepage check: registered by the component, reading system files behind roots a registered private environment key overrides
- [ ] `internal/component/l2tp/plugins/pppoe-vpp/startup.go` - the one startup registration. Its diagnostic tells the operator to build the artifact with `./le build vpp run` and install it
- [ ] `internal/plugins/firewall/vpp/binapi_imports.go` - the firewall backend links the ACL and NAT44 API packages and registers no startup feature
- [ ] `internal/plugins/iface/vpp/doctor.go` - the backend's doctor checks; the version check and the linux-cp check depend on the `vppctl` program
- [ ] `internal/le/build/vpp/register.go` - actions `run` (v25.10 PPPoE lab image, "not an appliance or DPDK package"), `deployment` (v26.06 with the IPsec patch) and `update`
- [ ] `internal/le/repo/compiles/matrix.go` - `buildMatrix`, five flavors. Not changed by this design
- [ ] `internal/appliance/cache.go` - `kernelTreeCachePath` and `KernelCacheNamespaces`, the cache directories cleanup preserves
- [ ] `internal/appliance/cmd_build.go` - `buildOne` calls `runtimeKernelReachable` and refuses before anything is written
- [ ] `internal/appliance/instance/kernelpkg.go` - `addLinuxNotice` rewrites the prepared `config.json`: it adds one `ExtraFileContents` entry to the `cmd/ze` package and refuses an entry already there
- [ ] `internal/appliance/instance/prepare.go` - `Prepare` derives the config, calls `addLinuxNotice`, and builds the prepared instance under the project `tmp/`
- [ ] `vendor/github.com/gokrazy/internal/config/config.go` - `ExtraFilePaths`: root destination path to a host path
- [ ] `vendor/github.com/gokrazy/tools/internal/packer/packer.go` - `extraFilesArchiveCandidates`, `extractArchive`, `FindExtraFiles`
- [ ] `vendor/github.com/gokrazy/tools/internal/packer/write.go` - `addToFileInfo`, `writeFileInfo`, `findBins`
- [ ] `vendor/github.com/gokrazy/tools/internal/packer/packerwrite.go` - `overwriteFile`
- [ ] `vendor/github.com/gokrazy/tools/packer/packer.go` - the partition table: boot 100 MB, two root partitions of 500 MB, then `/perm`
- [ ] `internal/component/config/system/selfupdate.go` - `stageBinary` and `resolveTarget`
- [ ] `internal/plugins/local/native.go` - `packageOwnership` finds `share/ze/install-method` from the prefix of the `bin` directory that holds a binary
- [ ] `internal/le/test/deployment/vppevidencescenarios.go` - the real-VPP lab sets `external true`
- [ ] `internal/le/arch/fspersistence/fspersistence.go`, `internal/core/diagnostic/codes.go` - read for the first version of this design; this design writes no file at run time and adds its codes in the owning package

### What the packer does with `ExtraFilePaths` (read at the producer)

| Question | Answer | Symbol |
|----------|--------|--------|
| Lookup order for a value `P` | `P_<GOARCH>.tar.zst`, `P_<GOARCH>.tar`, `P.tar.zst`, `P.tar`, then the directory `P`. The struct comment names only three of the five | `extraFilesArchiveCandidates` |
| Execution bits, archive | Kept: the mode is the tar header's | `extractArchive`, `writeFileInfo` |
| Execution bits, directory | Kept: the permission bits of the host file | `copyFileSquash` |
| Symbolic links, archive | Kept as links | `extractArchive`, `writeFileInfo` |
| Symbolic links, directory | Followed: a link becomes a full copy of its target | `addToFileInfo` |
| An empty regular file or a hard link in an archive | Becomes an empty directory: an entry with no content, no host path and no link target takes the directory branch | `writeFileInfo` |
| A destination under `/perm` | Refused | `packerbuild.go`, the loop after `FindExtraFiles` |
| Memory | Each archived file is read whole into memory | `extractArchive` |

**Behavior to preserve:** (unless the user explicitly said to change it)
- The whole supervision loop: config generation, NIC binding and restoration, the connect bound, the process witness, the restart backoff, the events
- The operator-supervised mode (`external true`) as it stands today: Ze runs nothing. Its removal is owned by `spec-vpp-failure-lifecycle`
- A build without the VPP tag refuses a `vpp` config exactly as today
- The lab actions `./le build vpp run` and `./le build vpp deployment` and their proofs. After the port phase both build the one VPP version, v26.06
- The PPPoE startup registration and its doctor check
- The appliance image's kernel notice entry and every other field of the prepared config

**Behavior to change:** (only what the user asked for)
- A managed start runs the VPP found in Ze's own folder and no other. `/usr/bin/vpp` is never started
- Ze's patches live on one VPP version, v26.06. The v25.10 patch directory, its pin and its lab image are deleted once the ported series passes its proofs
- Every appliance image carries Ze's VPP bundle in its root file system
- A managed start loads the ACL, NAT, VXLAN and GRE plugins when the configuration uses them
- The linux-cp plugin check stops running `vppctl` and reads the running VPP's API message table

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VPP process supervisor | `internal/component/vpp/vpp.go`, `VPPManager.runWithOps`, `runOnce` | Writes the config, binds NICs, runs one program path, connects, restarts | Extends: it is handed a command of several arguments, of which today's one path is the one-element case. Nothing else in the loop changes |
| Fixed program path | `internal/component/vpp/register.go`, `defaultVPPBinary` | The one source of the program path | Replaces: the program comes from the bundle found in the Ze folder. The constant is deleted (owner, 2026-11-28) |
| Extra files in the image root | `vendor/github.com/gokrazy/internal/config/config.go`, `ExtraFilePaths`; `vendor/github.com/gokrazy/tools/internal/packer/packer.go`, `FindExtraFiles` | Places a tar archive or a host directory at a root path. Used by `gokrazy/docker-host/config.json` | Extends: the appliance image gains one entry, the bundle archive at the Ze folder |
| Prepared image config rewrite | `internal/appliance/instance/kernelpkg.go`, `addLinuxNotice`; `prepare.go`, `Prepare` | Adds the kernel notice to the `cmd/ze` package's `ExtraFileContents` in the prepared config, refusing an entry already there | Extends: a sibling step adds the `ExtraFilePaths` entry and the environment value that names the Ze folder, with the same refusal. The checked-in `gokrazy/ze/config.json` is not changed, because the archive path is a cache path known only at build |
| Environment of the appliance `ze` | `gokrazy/ze/config.json`, `Environment` (`ze.config.dir=/perm/ze`) | Tells `ze` where its directories are on the appliance | Extends: the same list names the Ze folder, because gokrazy puts `ze` in its own `/user` directory |
| Prefix derived from the binary | `internal/plugins/local/native.go`, `packageOwnership`; `internal/component/config/system/selfupdate.go`, `resolveTarget` | Resolves the executable through links, and finds `share/ze/install-method` under the prefix of its `bin` directory | Extends: the Ze folder is the directory that holds the same resolved executable. The prefix rule for the config directory and the marker is `spec-release-build`'s |
| Keyed artifact cache with native builds | `docs/architecture/appliance/build-artifacts.md`, "The runtime kernel the appliance image boots"; `internal/appliance/cache.go`, `kernelTreeCachePath`, `KernelCacheNamespaces` | A slow native build is cached under a key of its inputs, refused for another architecture, preserved by cleanup | Extends: the bundle is one more artifact in that cache, and its directory joins the preserved namespaces |
| Reachability refusal before an image build | `internal/appliance/cmd_build.go`, `buildOne`, `runtimeKernelReachable` | Refuses before anything is written when the kernel can be neither found nor built here | Extends: the bundle is checked at the same place, the same way |
| Patched VPP build actions | `internal/le/build/vpp/register.go`, actions `run` and `deployment`; `third_party/vpp-patches/`, `third_party/vpp-patches-v26.06/` | Build two lab images from two pins, each for the build host's CPU, neither with DPDK | Replaces the two pins with one: the port phase moves the PPPoE series to v26.06 and deletes the v25.10 directory (owner, 2026-11-28). Extends: a `bundle` action in the same area builds the product bundle from that pin |
| Startup feature registry | `internal/component/vpp/startup_feature.go`, `RegisterStartupFeature` | A feature declares its plugin, config section, API messages and doctor code | Extends: firewall, NAT, VXLAN and GRE register. The section becomes optional. The registry is the list the bundle's plugins derive from |
| Per-feature API doctor check | `internal/component/vpp/startup_doctor.go`, `registerStartupDoctor` | Compares the running VPP's message table with a feature's messages | Extends: it stays the proof that a running VPP has the API Ze needs. The bundle check below answers before a start, this one after |
| VPP doctor checks with overridable roots | `internal/component/vpp/doctor_linux.go`, `vppHugepagesDoctorCheck`, `doctorHugepagesRootEnv` | A registered check reads host files behind roots a registered private key overrides | Extends: the bundle check is built the same way |
| Core leaf registry | `ai/doors/core.md`, "Boundaries"; `internal/core/ipsecinventory` | A core package holds a registry, the owner registers from `init()`, and no registration answers an error | Extends: the program registry copies that shape |
| Image validation before a push | `docs/architecture/appliance/ota-push.md`, the row "Validate GPT checksums, partition bounds, FAT and SquashFS lengths before contacting the device" | A push checks the root file system's length against the partition | Extends: the build refuses an oversized root too, so the failure is met at build, not at the first push (A-4) |
| In-place self-update | `internal/component/config/system/selfupdate.go`, `stageBinary` | Renames a new binary over the running one | Does not apply here: it replaces one file and cannot carry a bundle. Its replacement, an update that installs a version directory with `ze` and its VPP together, is designed in `spec-release-build`. Nothing in between is built here |
| The `./le` launcher and its build store | the `le` script at the repository root; `docs/contributing/running-commands.md` | Keeps several builds, selects one by an environment value, sets a failed one aside | Does not apply here: the owner named it as the model for the `ze` launcher, which `spec-release-build` designs |
| Packaging tag `ze_vpp_runtime`, unpack library, `storage` leaf, `/perm/ze/runtime` | The first version of this spec | Never built | Removed from the design: nothing is embedded, so nothing is tagged, unpacked, repaired or cleaned, and no path is added under `/perm` |
| Ze-run container images | `plan/post-release/features/spec-container-0-umbrella.md` | In progress, after the release | Does not apply: the owner chose files on disk |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | the door every change reads | "A doctor check registered by the package that owns the dependency" | agrees | The bundle check is registered by the VPP component |
| `docs/research/vpp-deployment-reference.md` | Generated grammar and upstream consumers | the `// Design:` page of the VPP component's files; a page this spec modifies | "Ze targets the [VPP v26.06 startup format]" | agrees | The generator keeps that grammar. The design adds the plugin path, which `vlib_plugin_config` accepts at v25.10 and v26.06 |
| `ai/doors/forwarding.md` | Where it sits | the door covering the VPP component | "`ze_vpp` removes `component/vpp` and every `vpp` backend" | agrees | Every new file of the component and its backends stays behind that tag |
| `ai/doors/appliance.md` | Who depends on it from outside | the door covering `internal/appliance` | "An image already in the field is the contract." | agrees | No `/perm/ze/` path, zefs key or push step changes |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `internal/le/build/vpp` | "**The rule is directional.** A shipped `ze` never links an `internal/le`" | agrees | The manifest format lives in a core package the tooling imports |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/programbundle` | "A registry holds value types and is the only list." | agrees | The program registry holds value records and names no program |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/firewall/vpp` | "**Always-on code never imports a gated plugin.**" | agrees | The declarations live in the gated backend packages |
| `ai/doors/access-security.md` | Where it sits | the door covering the PPPoE startup registration | | silent | Only the fix text of one diagnostic changes |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Decisions | the `// Design:` page of the PPPoE startup registration | | silent | The page does not say where the VPP program comes from |
| `docs/architecture/firewall/fw-6-firewall-vpp.md` | Startup ordering | anchored to `internal/component/vpp/register.go` | "Connection failure still refuses the VPP firewall apply; no timeout or readiness check is relaxed." | agrees | The section also says "VPP completes its startup tier before firewall configuration applies ACLs" and "The VPP manager starts from `OnStarted`". The dependency edge and the start from `OnStarted` stay. A bundle refusal ends VPP's start without ending `ze start` (A-14), so VPP is down when the firewall applies, and that apply is refused by the connection failure the page names. No timeout or readiness check changes. The ACL plugin is named in the generated config before VPP starts, so the ordering is untouched |
| `docs/architecture/fib/fib-depth-4-srv6.md` | VPP backend | anchored to `internal/component/vpp/vpp.go` | | silent | SRv6 programming over a running VPP; this design changes only which program is started |
| `docs/architecture/appliance/installer-initrd.md` | Host binaries and target binaries | anchored to `internal/appliance/cache.go` | | silent | The initrd is not changed: the bundle goes in the image root, not in the initrd |
| `docs/architecture/appliance/ota-push.md` | Decisions | anchored to `internal/appliance/cmd_build.go` | | silent | The push protocol is not changed. Its length check is the mechanism the build-time root size refusal copies |
| `docs/labs/pppoe-interop.md` | VPP mechanism image | the `// Design:` page of `internal/le/build/vpp/register.go`; a page this spec modifies | "Its recipe and patch provenance live in [`third_party/vpp-patches`](../../third_party/vpp-patches/README.md)." | departs | The port phase deletes `third_party/vpp-patches/` and moves the recipe and the PPPoE series to `third_party/vpp-patches-v26.06/`. The lab image stays and is built from the one v26.06 pin. The page is corrected in the port phase to name the v26.06 directory, and gains one line naming the product bundle action |
| `docs/architecture/appliance/build-artifacts.md` | The runtime kernel the appliance image boots | the `// Design:` page of `internal/appliance/cache.go`; a page this spec modifies | "The runtime target has two tiers, not three: its cache entry, else a native" | agrees | The bundle is resolved by the same two tiers and refused the same way |
| `docs/architecture/appliance/builder.md` | Decisions | the `// Design:` page of `internal/appliance/cmd_build.go` | | silent | The page records build host decisions; none is about what the image root holds |
| `docs/architecture/appliance/gokrazy-build-pins.md` | Decisions | the `// Design:` page of `internal/appliance/instance/kernelpkg.go` | | silent | No module pin changes. The page was not read for this spec; the phase that edits `instance/` reads it first |
| `docs/architecture/doctor-and-health-checks.md` | Where each owner registers its check | a page this spec modifies | "`diagnostic.RegisterDoctorCheck()` from the owning backend or component, with build-tagged files where needed" | agrees | The bundle check and the four plugin checks register that way |
| `ai/rules/repo-maintenance.md` | Directives | the `// Design:` page of `internal/plugins/iface/vpp/doctor.go` | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work: `ai/INDEX.md` for a keyword or a project fact, `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` for a file or a page, the owning `docs/architecture/` page for a contract, and a registered `ze doctor` check for a new runtime dependency." | agrees | The bundle is the new runtime dependency. Its doctor check is registered by the VPP component, and the four plugin checks by the backends that own them (Integration Checklist). `ai/INDEX.md` gains the keywords, and `docs/research/vpp-deployment-reference.md` and `docs/architecture/doctor-and-health-checks.md` are the pages (Files to Modify) |
| `docs/architecture/core-design.md` | Design decisions | an owner rule added beyond the derivation, decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code." | agrees | A missing or wrong bundle keeps VPP down and is reported; Ze starts (AC-4) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-28 | "Ze's own VPP build is always installed with Ze: no installation of Ze is without it. On the appliance the image build places the built VPP bundle as files in the image's root file system. On Linux the VPP files are installed in a Ze-owned folder for internal binaries, placed as the Filesystem Hierarchy Standard says, and Ze finds its VPP by scanning its own folder. One package carries ze and the VPP files, as a DEB built the usual Debian way and as an RPM. In Thomas's words (voice-transcribed): "on the appliance we should manage the VPP binary. Can we generate it and add it to the file system ?", "So we should always install our VPP with the software.", "VPP bundle placed in the image's file system", "for linux it should be installed in a ze folder for binaries following the LSB", "ok for having a .deb for installation too follow the usal debian packaging method in repos", "One package", "you can do rpm too", "same for vpp install it in the same folder as ze and make sure ze will scan that folder". This supersedes his decision of 2026-11-19 that the runtime lives inside the ze binary, and the words "as it does with the embedded runtime" of the row "Ze knows how to manage and restart an external VPP"; correcting that row needs Thomas's decision-change" | agrees | This design is that decision |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-28 | "Ze starts only its own VPP, the one installed with Ze. When Ze finds no VPP in its own folder it starts none: a VPP at /usr/bin/vpp is never started in its place. In Thomas's words (voice-transcribed), asked whether a VPP at /usr/bin/vpp is ever started when Ze finds none in its folder: "No, the only start is on VPP." This withdraws the fallback to an operator-installed /usr/bin/vpp that he kept on 2026-11-19" | agrees | The constant path is deleted, and a missing bundle keeps VPP down (AC-4) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-28 | "Every Ze patch is ported to VPP version 26, and that one version is used everywhere. In Thomas's words (voice-transcribed), asked which patched VPP is ours, (a) v26.06 with the IPsec patch, (b) v25.10 with the PPPoE patches, or (c) both patch sets on one version before the first release: "Oh, we should port all our patches to version 26 and use that version everywhere. C"" | agrees | The port phase is this spec's first phase, and AC-18 holds the tree to one pin. The GoVPP bindings vendored from upstream are regenerated from v26.06 and `third_party/vpp-linux-cp` is refreshed to v26.06 (N-7, taken as (a)) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-28 | "Every appliance image carries Ze's VPP, test images included, by one path and with no setting that leaves it out. In Thomas's words (voice-transcribed), asked whether every image carries VPP, test images included, (a) yes, one path and no setting, or (b) a field in the appliance config leaves it out: "A"" | agrees | The image build places the bundle in every image and adds no field (AC-1, AC-7) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-28 | "The DEB "built the usual Debian way" of the row "Ze's own VPP build is always installed with Ze" means this: nFPM stays the tool that assembles the package, the package follows Debian Policy, and the APT and DNF repository comes after the first release. In Thomas's words (voice-transcribed), asked what "the usual Debian packaging method in repos" changes, (A) nFPM stays, the package follows Debian Policy for file locations, maintainer scripts and configuration kept on removal, and the APT and DNF repository stays after the first release, (B) the repository in the first release, or (C) debhelper in place of nFPM: "a"" | agrees | This spec builds the bundle archive only. The package that carries it is assembled with nFPM by `spec-release-build` |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | This design changes where the program comes from, not who supervises it |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | Ze still starts the binary directly. The words "the embedded runtime" name what the 2026-11-28 row withdrew; that row says so, and correcting this one needs Thomas's decision-change |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | The restart path is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | Untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | agrees | No timer is added or changed |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | agrees | The child Ze starts is still the process the witness binds (A-6) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | agrees | A restart runs the same bundle the first start ran |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | Untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | This design installs no systemd unit for VPP |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | agrees | Untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | Untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | Untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | Untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | Untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | That row is about a VPP that already runs. The bundle check of this design refuses only to start a bundle that is not the one this Ze was built for, before any VPP runs |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 | "D-1, a VPP API loss with the process alive, is resolved by the VPP lifecycle rules: a live VPP that Ze cannot adopt is killed and restarted, so its exit proof always arrives and retires the retained owners" | agrees | Untouched |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 | "When Ze adopts the VPP, retained owners are read back by exact identity over the new connection of the same pidfd-proven process, and cleaned there (option C)" | agrees | Untouched |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 | "Adoption happens only inside the single VPP loss timer: a reconnect to the same pidfd-proven process whose full readback matches (option C) keeps the sessions. A declared loss is never adopted: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This narrows the row "When Ze adopts the VPP" to the single timer, and the D-1 row's "cannot adopt" to every declared loss" | agrees | Untouched |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 | "D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms. Form (1), inside the reconnect-and-verify limit that follows the failed alive test: the same pidfd-proven process answering with the data Ze configured is enough to adopt it, and the PPPoE sessions are kept. The retained owners are read back by exact identity over the new connection of that process and cleaned there (option C), as part of the background reconciliation; a mismatch it finds is repaired, never treated as a loss. Form (2), a forced restart: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This supersedes the row "Adoption happens only inside the single VPP loss timer" (the single timer, adoption on a full readback match, and "a declared loss is never adopted"), and narrows the D-1 row and the row "When Ze adopts the VPP" to these two forms" | agrees | Untouched |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build time: one pinned VPP source with its patches under `third_party/`, and the plugin list derived from the startup feature registry
- Run time: the `vpp` configuration root with `enabled true` and `external false`

### The Ze folder

The Ze folder is the directory of an installation that holds `ze` and the programs Ze runs and an operator does not. Each such program is a subdirectory holding a manifest. VPP is the subdirectory `vpp`.

| Installation | Where `ze` is | The Ze folder | How Ze learns it |
|--------------|---------------|---------------|------------------|
| DEB or RPM | `/usr/lib/ze/<release>/ze`, started by the launcher `/usr/bin/ze` | `/usr/lib/ze/<release>` | It is the directory that holds the resolved executable |
| `/usr/local` install | `/usr/local/lib/ze/<release>/ze`, started by the launcher `/usr/local/bin/ze` | `/usr/local/lib/ze/<release>` | The same rule. The launcher replaces itself with that `ze`, so the resolved executable is the real file |
| Appliance | `/user/ze`, chosen by gokrazy (`findBins`) | `/usr/lib/ze` | The environment value the image build writes, because `/user` is gokrazy's directory |
| A test | the test binary | a fixture directory | The same environment value |

One registered environment key names the Ze folder and wins when set. Unset, the folder is the directory that holds the resolved executable: his "same folder as ze", read literally (owner, 2026-11-28). `<release>` is the release string, `YY.MM.DD` or `YY.MM.DD.N`. The appliance has one version and no launcher, so its folder carries no release name.

### The bundle

| Part | Content |
|------|---------|
| Program and loader | The `vpp` program and the C library loader it was built against. The loader is optional in the manifest: a program linked statically, such as the Go VPP stub the functional tests use, has none, and its command is the program alone |
| Libraries | Every shared library the program and the plugins link, the C library included, in one directory. No hard link and no empty file (the packer turns both into directories) |
| Plugins | Every plugin Ze can name in a generated config: the built-in list and the startup feature registry, DPDK included. The crypto engines |
| Manifest | Name, VPP version, source commit, a digest of the patch series, architecture, the entry program, the loader, the library directory, the plugin directory, and each file's path, mode, size and SHA-256 |
| Licences | Each component's licence text, and the list of components per licence |

The bundle is one archive, the same bytes for the appliance, the package and a version directory. It is built for a portable CPU, not the build host's.

### How the program is run

The first version of this spec chose to run the bundled loader directly with the library directory, because the unpack location varied. That reason was re-examined. The location still varies: `/usr/lib/ze/vpp` on the appliance, and a different directory for every version in a package and under `/usr/local`. A program file cannot name its loader by a relative path, so a bundle linked for one prefix cannot run from a version directory. The decision therefore stands, on its merits: Ze runs the bundled loader, with the bundle's library directory, the program and the config argument. One archive then serves every installation.

Costs accepted: the process's executable is the loader, so `/proc/<pid>/exe` and the short process name show the loader and not `vpp`. The command sets the program name the process reports where the loader allows it (A-7). The bundle check and the witness are built for this (A-6).

### Transformation Path
1. Build. `./le build vpp bundle` builds the one pinned VPP, v26.06 with every Ze patch (the port phase makes that pin), in a container for the host's architecture, collects the parts above, writes the manifest, and stores the archive and the source archive of every LGPL component in the keyed cache
2. Image placement. `ze appliance build` resolves the archive for the image's architecture, and the prepared config gains two things for the `cmd/ze` package: an `ExtraFilePaths` entry from `/usr/lib/ze/vpp` to the cached archive, and the environment value naming `/usr/lib/ze`
3. Package placement. The release build places the same archive's files under `/usr/lib/ze/<release>/vpp` in the one `ze` package, and under `usr/local/lib/ze/<release>/vpp` in the Linux tarball (owned by `spec-release-build`)
4. Scan. At a managed start the VPP component asks the program registry for `vpp`. The registry lists the Ze folder, reads the manifest in the `vpp` subdirectory, and answers the bundle or the reason it has none
5. Start check. Before the program runs, Ze compares the manifest's name, architecture, VPP version, source commit and patch digest with the identity compiled into this `ze`, and checks that the entry program and the loader exist and can be executed. It does not hash every file at each start
6. Run. The component generates the config with the bundle's plugin directory and hands the supervisor the command
7. Supervise. Unchanged: connect, witness, the per-feature API checks, events, restart
8. Doctor. `ze doctor` runs the start check and, in addition, compares every file with the manifest's size and SHA-256

### What Ze checks before it starts a VPP it finds

| Check | When | Why this and not more |
|-------|------|-----------------------|
| The manifest is there and names `vpp` for this architecture | Each managed start, and `ze doctor` | A missing or foreign bundle is the common failure |
| The bundle's identity equals the one this `ze` was built for | Each managed start, and `ze doctor` | Ze's API bindings are generated from one VPP build. A different build is refused, not tried (exact or refuse) |
| The entry program and the loader exist and can be executed | Each managed start, and `ze doctor` | Cheap, and it names the file |
| Every file's size and SHA-256 | `ze doctor` only | The appliance root is read-only and checked at update, and a package's files are the package manager's. Hashing the whole bundle at each restart would delay recovery after a crash |
| The plugin APIs the configuration needs | After connect, as today | The per-feature checks of the startup registry already do it |

A failed check never ends `ze start` (the "Ze always starts" decision). VPP is not started, and one error line and one doctor finding name the cause and the fix.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build host container to cache | An archive and its manifest, keyed by a hash of the inputs | No |
| Cache to image | An `ExtraFilePaths` entry in the prepared config | No |
| Cache to package | The release build's file list (other spec) | No |
| VPP component to program registry | A registration from `init()`; the registry names no program | No |
| Ze to the file system | Reads only | No |
| Ze to VPP | The supervisor's existing child process and API socket | No |

### Changed-Wire Census

**Search evidence:** Scope: the whole tree outside `vendor/`, `tmp/` and `gokrazy/modcache/`. Terms: `defaultVPPBinary` and `vppBinary` (one writer, `register.go`; one reader, `vpp.go` `runOnce`); `/usr/bin/vpp` (the constant, the YANG description, the guide); `ExtraFilePaths` (one user, `gokrazy/docker-host/config.json`; none in `gokrazy/ze/config.json` or `internal/appliance`); `ExtraFileContents` (`gokrazy/ze/config.json` and `kernelpkg.go`); `RegisterStartupFeature` (one caller, the PPPoE backend); `vppctl` in non-test Go (`internal/plugins/iface/vpp/doctor.go`); `install-method` (`internal/plugins/local/native.go`). Positive control: the same searches find the constant, the docker-host entry, the notice entry and the PPPoE registration. Finding: no bundle and no manifest exist today, so both are new wires with no speaker to migrate.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Bundle archive | writer | New: the `bundle` action in `internal/le/build/vpp`, with a new Dockerfile beside the pin it builds | The two existing Dockerfiles end in a container image, not an archive | New wire |
| Bundle archive | reader | The vendored packer, `extractArchive`; the release build (other spec) | Reads tar headers: name, mode, link target | Preserved: the archive holds only what that reader keeps (no hard link, no empty file) |
| Bundle manifest | writer | New: the `bundle` action in `internal/le/build/vpp` | No existing writer | New wire. Its format is declared once, in `internal/core/programbundle`, and the writer uses that declaration |
| Bundle manifest | reader | New: `internal/core/programbundle`, called by the VPP component | No existing reader | New wire |
| Prepared image config | writer | `internal/appliance/instance`, a sibling of `addLinuxNotice` called from `Prepare` | `addLinuxNotice` writes `ExtraFileContents` only | Changes: one `ExtraFilePaths` entry and one environment value are added. Every other field is preserved, and an entry already at that destination is refused |
| Prepared image config | reader | The vendored packer, `FindExtraFiles` | Reads `PackageConfig[...].ExtraFilePaths` | Preserved |
| Program command | writer | `internal/component/vpp/register.go`, `runConfiguredVPP` | Passes `defaultVPPBinary` | Changes: it hands the bundle's command |
| Program command | reader | `internal/component/vpp/vpp.go`, `VPPManager.runOnce` | Runs one path with `-c <config>` | Changes: it runs a command of several arguments |
| Generated VPP config, plugins section | writer | `internal/component/vpp/startupconf.go`, `GenerateStartupConf` | Writes `plugin default`, then one entry per enabled name | Changes: adds the bundle's plugin directory |
| Generated VPP config, plugins section | reader | The VPP program, upstream `vlib_plugin_config` | Accepts `path`, `add-path`, `plugin default`, `plugin <name>` (read at v25.10, 2026-11-28) | Preserved |

The labs under `internal/le/test/deployment/` write their own VPP config for a VPP they start themselves. They do not read Ze's generated file.

### Integration Points
- `vpp.RegisterStartupFeature` - the firewall, NAT, VXLAN and GRE backends register their plugin
- The program registry in `internal/core/programbundle` - the VPP component registers `vpp`
- `Prepare` in `internal/appliance/instance` and `buildOne` in `internal/appliance` - the image build resolves and places the bundle
- `./le build vpp` - gains the `bundle` action
- `plan/release/delivery/spec-release-build.md` - places the bundle's files in the package

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
| A-1 | VPP takes its plugin directory from the config file | Upstream `vlib_plugin_config` at v25.10 matches `path %s` (read 2026-11-28) | The bundle needs one fixed location | The QEMU proof | confirmed by reading |
| A-2 | The bundled loader can be run directly with a library directory, and the libraries that plugins load later resolve the same way | General behavior of the GNU loader, not verified in this repository | The bundle must be linked for one prefix, which a version directory under `/usr/local` cannot be | The QEMU proof and the container proof, in the first phase | unvalidated |
| A-3 | The product bundle with DPDK is small enough. Measured without DPDK: about 63 MB unpacked (lab image, arm64, 2026-11-19). The `ze` binary on the development host is 120 MB (`bin/ze`, not a release build) | Those two measurements | The package and the image grow by more than estimated | The build phase measures the bundle packed and unpacked per architecture and records it on the build-artifacts page | unvalidated |
| A-4 | The image root, with the bundle, fits a 500 MB root partition, and the packer does not refuse a root that does not | `vendor/github.com/gokrazy/tools/packer/packer.go` (two partitions of 500 MB); `overwriteFile` holds no size check. The current root's size was not measured | An image is built whose root overruns its partition | The image build measures the root and refuses above the partition size (AC-7) | unvalidated |
| A-5 | Extra-file roots merge into the image root, so `/usr/lib/ze/vpp` appears beside what gokrazy places | `gokrazy/docker-host/config.json` places a directory at `/`. The merge code was not read | The destination needs another form | The QEMU proof lists the directory on the booted image | unvalidated |
| A-6 | The loader becomes the VPP program inside the process Ze started, so the witness and the API peer check see one process | How a loader works in general | The witness refuses the connection | `TestManagedStartThroughLoaderBindsWitness` and the QEMU proof | unvalidated |
| A-7 | The bundled loader accepts an option that sets the program name the process reports | Not read | `ps` shows the loader's name; the guide says so | Reading the loader's options for the base image's C library version in the build phase | unvalidated |
| A-8 | A VPP built without the native-only option runs on any 64-bit CPU Ze supports | Both existing Dockerfiles set `VPP_BUILD_NATIVE_ONLY=ON` | A bundle built on one CPU crashes on another | The QEMU proof repeated with a different CPU model | unvalidated |
| A-9 | Paths VPP uses beyond its program exist and are writable on the appliance: `/etc/vpp`, `/run/vpp`, the log path under `/var` | `register.go`, `startupconf.go` | VPP starts and then fails on a directory | The QEMU proof. Each failure is fixed in this spec | unvalidated |
| A-10 | The ACL, NAT, VXLAN and GRE APIs are absent from a VPP whose plugin was not loaded | Ze's config disables every plugin it does not name. Not observed in a run | The four declarations are harmless and the second defect does not exist | The QEMU proof applies a firewall rule before and after the declaration; the first run is recorded red | unvalidated |
| A-11 | The licences of DPDK and of each bundled library permit shipping them in the image and the package with their notices | Only LGPL 2.1 and Apache 2.0 were read. The full list is known once the bundle is built | A library cannot be shipped this way | The build phase lists every file's licence in the manifest; each distribution clause relied on is quoted in the bundle's README before the bundle is used | unvalidated |
| A-12 | A bundle carrying its own C library runs on every distribution of the package test matrix | The bundle depends on the kernel only | Linux needs a build per distribution | The container proof runs on the matrix `spec-release-distribution` names | unvalidated |
| A-13 | gokrazy does not start or otherwise read what sits under `/usr/lib/ze` | Not read | A file there is treated as a service | The QEMU proof | unvalidated |
| A-15 | Each of the seven patches under `third_party/vpp-patches/` applies to v26.06 or has an equivalent there, and the PPPoE plugin with Ze's API additions builds and passes its proofs on v26.06 | Not read: no patch was tried against v26.06 for this spec | The port is more than a rebase, and the first release waits on it or the owner decides the one-version question again | The port phase: each patch applied with no fuzz, and the PPPoE proofs re-run on v26.06 | unvalidated |
| A-14 | A failed managed start of VPP can be kept from ending `ze start`. Today `runVPPEngine` returns the error of `vppLifecycle.Start` from the plugin's `OnStarted`, bounded by `startupReadyTimeout`, and `ai/doors/hub-engine.md` says one plugin failure aborts the whole startup tier. What the plugin server does with that error was not traced | `internal/component/vpp/register.go` `runVPPEngine`; `startup_lifecycle.go` `Start`, `replace` | AC-4 and AC-5 need a change outside the VPP component, or the "Ze always starts" decision already has an owner for this path | The second phase traces `OnStarted` to the plugin server before any code. When a bundle refusal would end the start, the component reports it and starts without VPP, and that change is in this spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | VPP has never run on the appliance, so the first real start meets host assumptions this spec did not list | The QEMU proof fails after the program starts | The proof is an acceptance criterion and runs in the first phase. Each failure is fixed inside this spec |
| R-2 | Every appliance image build needs the bundle, so a first build on a machine costs a VPP build, and every test that builds an image needs a warm cache | An image build that started at once now refuses or runs a container | The cache is keyed and preserved. The refusal names the command. CI caches the bundle as it caches the kernel. Every image carries it (owner, 2026-11-28) |
| R-3 | A self-updated `ze` runs beside the bundle of the release it replaced | The start check reports another identity | An update installs a version directory, `ze` and its VPP together (`spec-release-build`). Until that lands, the start check refuses the old bundle and `ze doctor` names the fix |
| R-4 | No build holds both the PPPoE series and the IPsec patch | One backend's API check fails on the product bundle | The port phase moves every patch to v26.06 before the bundle is built (owner, 2026-11-28) |
| R-12 | The port changes the patch directory and the bindings that `spec-pppoe-vpp-dataplane`, in progress in another session, works on | Conflicting edits under `third_party/vpp-patches/` or in the PPPoE bindings | The port lands as one phase. That spec is amended to build on v26.06 (Work Not Done), and its session is told before the phase starts |
| R-5 | The container base image changes between builds | Two builds of the same inputs differ | The Dockerfile pins the base image by digest and every package by version; the cache key includes the Dockerfile |
| R-6 | A backend added later calls a plugin API and declares no startup feature | A feature works on a lab VPP and fails on a managed one | `TestBackendPluginAPIsAreDeclared` |
| R-7 | The image is built for an architecture whose bundle is not cached | `ze appliance build` cannot place a bundle | It refuses before writing anything, with the cache path and the command for a host of that architecture |
| R-8 | An archive entry the packer mishandles (hard link, empty file) reaches the image as a directory | VPP fails to load a library | `TestBundleArchiveHoldsOnlyWhatThePackerKeeps` refuses such an archive at build |
| R-9 | The release build runs on a macOS host and compares its artifacts with a Linux CI rebuild, byte for byte. A natively built C bundle is not rebuilt identically on both | The comparison never matches | The bundle enters the release build as a cached input pinned by its digest, never rebuilt there. The amendment row for `spec-release-build` says so |
| R-10 | Scratch cleanup removes the cached bundle | A warm build runs a container | The bundle's cache directory joins `KernelCacheNamespaces` |
| R-11 | A second session edits the VPP component while this is implemented | A hunk conflict in `register.go` or `startupconf.go` | Small separate files; whole files are committed and the concurrent work named |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A managed VPP does not start, so a host configured for the VPP backend forwards nothing. The operator-supervised mode is untouched |
| How is it reverted? | The program source is one commit: reverting it returns the constant path. The image entry and the four plugin declarations are separate commits |
| Who else touches this path? | `plan/post-release/features/spec-vpp-failure-lifecycle.md`, `plan/post-release/features/spec-pppoe-vpp-dataplane.md`, `plan/release/delivery/spec-release-build.md`, `plan/release/delivery/spec-release-distribution.md`, and the release fixes named `spec-vpp-*` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A `vpp` config with `enabled true`, the Ze folder holding a bundle of the VPP stub | → | The component finds the bundle, checks it, and the supervisor runs its command | `test/vpp/bundle-start.ci` |
| The same config, the Ze folder holding no bundle | → | Ze starts, VPP stays down, the doctor finding names the fix | `test/vpp/bundle-missing.ci` |
| A firewall rule under `backend vpp` | → | The firewall backend's startup registration selects the ACL plugin | `TestFirewallConfigEnablesACLPlugin` |
| `./le build vpp bundle` | → | The bundle build writes an archive and manifest into the keyed cache | `TestBundleActionWritesCacheEntry` |
| `./le build vpp run`, after the port phase | → | The lab image is built from the one v26.06 pin, PPPoE series included | `TestOneVPPPin` |
| `ze appliance build` | → | The prepared config places the bundle and names the Ze folder, or the build refuses | `TestPrepareAddsBundleEntry`, `TestApplianceBuildRefusesMissingBundle` |
| An appliance image booted in QEMU with a managed VPP config | → | The image's `ze` starts real VPP from the image's root | `./le test qemu vpp-bundle` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An appliance image built by `ze appliance build`, booted with a config that enables a managed VPP | The image root holds the bundle under `/usr/lib/ze/vpp`, the image's `ze` starts VPP from it, the API answers with the bundle's VPP version, and Ze publishes `connected` | The operator-supervised mode stays as it is, because its removal belongs to another spec |
| AC-2 | A Linux container holding `/usr/lib/ze/<release>/ze` and the bundle beside it under `/usr/lib/ze/<release>/vpp`, with no C library and no other VPP | `ze` finds the bundle in its own directory, with no setting, starts VPP and the API answers with the bundle's VPP version | A distribution VPP at `/usr/bin/vpp` on the same host stays as installed, because Ze's files do not share its paths |
| AC-3 | The same directory, `ze` and `vpp` together, moved to another path | Ze finds the bundle in the new directory, starts VPP from it, and the API answers with the bundle's VPP version, with every setting unchanged | The bundle at the old prefix, when left in place, is ignored, because the scan follows the `ze` that runs |
| AC-4 | A managed VPP config, and the Ze folder holds no `vpp` bundle | `ze start` succeeds and its other services run. VPP is not started. One error line and one `ze doctor` finding, under the same code, name the folder that was scanned and the fix | A VPP at `/usr/bin/vpp` on the same host is left alone and is not started, because Ze starts only its own (owner, 2026-11-28) |
| AC-5 | The Ze folder holds a bundle whose identity is not the one this `ze` was built for | `ze start` succeeds, VPP is not started, and the finding names both identities and the fix | A bundle of the right identity starts, because the identity is what the API bindings were generated from |
| AC-6 | One bundle file altered after installation | `ze doctor` names the file and its expected checksum, and the fix | A managed start still passes its check when the altered file is neither the program nor the loader, because the full comparison is doctor's |
| AC-7 | `ze appliance build` for an architecture whose bundle is not cached, or whose root file system exceeds the root partition | The build is refused before the image is written, naming the cache path or the two sizes, and ending with the command that works | A warm cache and a root that fits build as today, because neither refusal applies |
| AC-8 | The appliance config directory already places a file at the bundle's destination | The build is refused naming the entry to remove, as the kernel notice entry is | Every other field of the config is preserved, because only the two additions are written |
| AC-9 | `./le build vpp bundle` run twice with unchanged inputs | The second run answers with the cached entry and starts no container. The cache holds the archive, its manifest and the source archive of every LGPL component | A changed input (pin, patch, plugin list, Dockerfile) builds again, because its key differs |
| AC-10 | The bundle's archive | It holds every plugin Ze can name in a generated config, and its manifest lists every file with its checksum and every component with its licence. An archive with a hard link or an empty file is refused by the build | A symbolic link is permitted, because the packer keeps it |
| AC-11 | A config with a firewall rule, a NAT rule, a VXLAN interface or a GRE tunnel under the VPP backend, on a managed VPP | The generated config enables that feature's plugin, VPP loads it, and the rule or interface is applied and read back from VPP | A config that uses none of them enables none of the four, because a plugin Ze does not need is not loaded |
| AC-12 | The same four features on a running VPP whose plugin is not loaded | `ze doctor` reports the missing plugin by name with the feature that needs it and the fix | A VPP that has the plugin loaded reports nothing, because the API table holds its messages |
| AC-13 | A backend package links a VPP plugin's API package and registers no startup feature for it | `TestBackendPluginAPIsAreDeclared` fails and names the package and the API | APIs of VPP's core need no declaration, because no plugin loads for them |
| AC-14 | `vpp.lcp` enabled on a managed VPP on the appliance, where no `vppctl` exists | The linux-cp check answers from the API message table and reports the plugin loaded | The check keeps reporting "unknown" when the API socket cannot be reached, because that is not evidence about plugins |
| AC-15 | The QEMU proof repeated with a CPU model other than the build host's | VPP starts and the API answers with the bundle's VPP version | The build host's own CPU model stays a valid target, because the bundle is portable, not foreign |
| AC-16 | A second program registered with the program registry in a test, its bundle placed in the Ze folder | The registry answers that program's bundle by the same scan, while no file of the registry or of the scan names it | `vpp` is still found beside it, because each program is its own subdirectory |
| AC-18 | The tree after the port phase | One patch directory and one pin, v26.06, hold the IPsec patch and the ported PPPoE series. `./le build vpp run`, `./le build vpp deployment` and `./le build vpp bundle` build from that pin, Ze's own generated bindings are generated from it, the 32 GoVPP binding packages vendored under `vendor/go.fd.io/govpp/binapi` are regenerated from it and their headers name v26.06, `third_party/vpp-linux-cp` holds the v26.06 source and its README says so, and no build action, Dockerfile, lab image, binding or page names another version as a VPP Ze builds or tests against (N-7, taken as (a)) | Dated reading records such as "read at v25.10" in this spec stay, because they record what was read on a day, not a version Ze builds or tests against |
| AC-19 | A PPPoE config under the VPP backend, on a VPP started from the bundle | The PPPoE startup feature's API check passes: the bundle's VPP answers every message the PPPoE backend needs | PPPoE on VPP is not announced as a first-release feature, and `spec-pppoe-vpp-dataplane` stays after the release, because only the port moves into the release (N-8, taken as (a)) |
| AC-17 | An operator reads the VPP guide, the appliance guide and the `vpp` config help after this work | Each says where VPP comes from on the appliance and on Linux, how to tell which VPP a `ze` runs, and what `ze doctor` reports when it is missing | The operator-supervised section stays while that mode stays |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds an appliance image and boots it with VPP enabled | image build resolves the bundle → prepared config places it → `ze` scans `/usr/lib/ze` → supervisor runs it → `connected` | `./le test qemu vpp-bundle` |
| 2 | Installs Ze on a Linux host and enables VPP | `ze` derives the Ze folder from its path → finds `vpp` → runs it | `./le test deployment vpp-bundle` |
| 3 | Adds a firewall rule under the VPP backend | firewall registration selects the ACL plugin → VPP loads it → rule applied | `./le test qemu vpp-bundle` (firewall step) |
| 4 | Runs `ze doctor` on a host whose bundle is missing or damaged | the bundle check scans, compares, reports the fix | `test/vpp/bundle-missing.ci`, `test/vpp/bundle-damaged.ci` |
| 5 | Bumps the pinned VPP | new bundle identity → new cache entry → an image or package built from it starts; an old bundle is refused with both identities named | `TestBundleCacheKeyChangesWithPin`, `test/vpp/bundle-mismatch.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegistryFindsRegisteredProgram` | `internal/core/programbundle/registry_test.go` | A registered name with a bundle in the folder is found | |
| `TestRegistryAnswersErrorWithNothingRegistered` | `internal/core/programbundle/registry_test.go` | An unregistered name answers an error, not an empty result | |
| `TestFolderIsTheExecutableDirectory` | `internal/core/programbundle/folder_test.go` | The folder is the directory that holds the resolved executable, through a link | |
| `TestFolderEnvironmentValueWins` | `internal/core/programbundle/folder_test.go` | The environment value replaces the derived folder | |
| `TestManifestRefusesPathOutsideBundle` | `internal/core/programbundle/manifest_test.go` | A manifest entry naming a path outside the bundle is refused | |
| `TestVerifyNamesAlteredFile` | `internal/core/programbundle/verify_test.go` | One altered file is named with its expected checksum | |
| `TestCommandUsesBundledLoader` | `internal/core/programbundle/command_test.go` | A manifest with a loader yields loader, library directory, program | |
| `TestCommandForProgramWithoutLoader` | `internal/core/programbundle/command_test.go` | A manifest with no loader yields the program alone | |
| `TestManagedStartRunsFoundBundle` | `internal/component/vpp/bundle_test.go` | The supervisor is handed the bundle's command | |
| `TestManagedStartRefusesOtherIdentity` | `internal/component/vpp/bundle_test.go` | A bundle of another identity is not started | |
| `TestExternalModeNeverScans` | `internal/component/vpp/bundle_test.go` | `external true` asks the registry for nothing | |
| `TestStartupConfNamesBundlePluginDirectory` | `internal/component/vpp/managed_startup_test.go` | The plugins section carries the bundle's plugin directory | |
| `TestBuiltinPluginsDeclaredOnce` | `internal/component/vpp/managed_startup_test.go` | The generator's built-in plugins and the exported list are one declaration | |
| `TestStartupFeatureWithoutSection` | `internal/component/vpp/startup_feature_test.go` | A feature with no config section registers and writes no section | |
| `TestManagedStartThroughLoaderBindsWitness` | `internal/component/vpp/process_managed_integration_linux_test.go` | A child started through a loader is the process the witness binds | |
| `TestBundleDoctorCheck` | `internal/component/vpp/bundle_doctor_test.go` | Missing, other identity and altered file each give their finding and fix | |
| `TestFirewallConfigEnablesACLPlugin` | `internal/plugins/firewall/vpp/startup_test.go` | A firewall rule under the VPP backend selects the ACL plugin | |
| `TestNATConfigEnablesNATPlugin` | `internal/plugins/firewall/vpp/startup_test.go` | A NAT rule selects the NAT plugin; no NAT does not | |
| `TestVXLANConfigEnablesVXLANPlugin` | `internal/plugins/iface/vpp/startup_test.go` | A VXLAN interface selects the VXLAN plugin | |
| `TestGREConfigEnablesGREPlugin` | `internal/plugins/iface/vpp/startup_test.go` | A GRE tunnel selects the GRE plugin | |
| `TestLCPPluginCheckReadsAPITable` | `internal/plugins/iface/vpp/doctor_test.go` | The linux-cp check answers from the message table and runs no program | |
| `TestBackendPluginAPIsAreDeclared` | `internal/le/build/vpp/declared_test.go` | Every plugin API a VPP backend links has a startup registration | |
| `TestBundlePluginListMatchesDeclarations` | `internal/le/build/vpp/bundle_test.go` | The committed plugin list equals the built-in list plus the registry | |
| `TestBundlePinMatchesPatchDirectory` | `internal/le/build/vpp/bundle_test.go` | The identity compiled into `ze` equals the pin and patch digest the build reads | |
| `TestBundleActionWritesCacheEntry` | `internal/le/build/vpp/bundle_test.go` | The action writes archive, manifest and source archive under the key | |
| `TestOneVPPPin` | `internal/le/build/vpp/bundle_test.go` | One patch directory and one pin exist, every build action reads that pin, the directory holds the IPsec patch and the PPPoE series, every vendored GoVPP binding header names v26.06, and the `third_party/vpp-linux-cp` README names v26.06 | |
| `TestBundleArchiveHoldsOnlyWhatThePackerKeeps` | `internal/le/build/vpp/bundle_test.go` | An archive with a hard link or an empty file is refused | |
| `TestBundleCacheKeyChangesWithPin` | `internal/appliance/vppbundle_test.go` | Each input changes the key | |
| `TestBundleCacheIsPreservedByCleanup` | `internal/appliance/vppbundle_test.go` | The bundle's cache directory is among the preserved namespaces | |
| `TestPrepareAddsBundleEntry` | `internal/appliance/instance/bundle_test.go` | The prepared config gains the entry and the environment value and keeps every other field | |
| `TestPrepareRefusesTakenDestination` | `internal/appliance/instance/bundle_test.go` | An entry already at the destination is refused by name | |
| `TestApplianceBuildRefusesMissingBundle` | `internal/appliance/vppbundle_test.go` | A cold cache for another architecture refuses with the command | |
| `TestApplianceBuildRefusesOversizedRoot` | `internal/appliance/rootsize_test.go` | A root above the partition size is refused with both sizes | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| A file's size against its manifest size | exact match | the manifest size | one byte fewer | one byte more |
| The root file system against the root partition | at most the partition | exactly the partition size | N/A | one byte more |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bundle-start` | `test/vpp/bundle-start.ci` | A `ze` whose Ze folder holds a bundle of the VPP stub runs it and reports VPP connected | |
| `bundle-missing` | `test/vpp/bundle-missing.ci` | With no bundle, `ze` starts, and `ze doctor` names the folder and the fix | |
| `bundle-mismatch` | `test/vpp/bundle-mismatch.ci` | A bundle of another identity is not started and both identities are named | |
| `bundle-damaged` | `test/vpp/bundle-damaged.ci` | `ze doctor` names an altered file | |
| `vpp-bundle` | `./le test qemu vpp-bundle` | A real image boots, its `ze` starts real VPP, a firewall rule is applied and read back, and the run repeats on a second CPU model | |
| `vpp-bundle` | `./le test deployment vpp-bundle` | A container with no C library runs `ze` and real VPP starts from `/usr/lib/ze/<release>/vpp` | |

The stub tests build a small bundle holding the existing Go VPP stub in a fixture directory and point the Ze folder at it with the environment value. No VPP is built.

## Files to Modify
- `internal/component/vpp/register.go` - `runConfiguredVPP` asks the program registry and hands the supervisor a command; the constant path is deleted
- `internal/component/vpp/vpp.go` - the supervisor runs a command of several arguments
- `internal/component/vpp/startupconf.go` - plugin directory in the plugins section; built-in plugin names read from one declaration
- `internal/component/vpp/startup_feature.go` - the config section becomes optional
- `internal/component/vpp/startup_lifecycle.go` - a bundle refusal is reported and does not end `ze start` (A-14)
- `internal/component/vpp/yang/ze-vpp-conf.yang` - the `vpp` description stops naming `/usr/bin/vpp`; the `external` description stops naming a sidecar
- `internal/component/l2tp/plugins/pppoe-vpp/startup.go` - the diagnostic's fix text names Ze's bundle
- `internal/plugins/iface/vpp/doctor.go` - the linux-cp check reads the API table; the `vppctl` probe is deleted
- `internal/le/build/vpp/register.go` - the `bundle` action
- `internal/appliance/cache.go`, `internal/appliance/cmd_build.go` - bundle cache path, preserved namespace, the reachability refusal and the root size refusal in `buildOne`
- `internal/appliance/instance/prepare.go` - `Prepare` calls the bundle step
- `third_party/vpp-patches-v26.06/README.md` - the one patch directory: the ported PPPoE series, the bundle build, its plugin list, the licences read
- `third_party/vpp-patches-v26.06/Dockerfile` - builds the one pin with every patch, the PPPoE plugin included
- `third_party/vpp-patches/README.md` - removed with its directory by the port phase, once the seven patches, the API Dockerfile and the proofs have their v26.06 form
- `vendor/go.fd.io/govpp/binapi/` - the 32 upstream GoVPP binding packages regenerated from Ze's v26.06 API files, so no binding comes from 25.10 (N-7, taken as (a))
- `third_party/vpp-linux-cp/` and its `README.md` - the reference copy of the linux-cp plugin source refreshed from v25.10 to v26.06 (N-7, taken as (a))
- `docs/guide/vpp.md`, `docs/guide/appliance.md` - where VPP comes from, how to check it
- `docs/research/vpp-deployment-reference.md` - the design page of the VPP component
- `docs/architecture/appliance/build-artifacts.md` - the bundle as a cached artifact, with its measured sizes and the measured root size
- `docs/architecture/doctor-and-health-checks.md` - the new checks
- `docs/labs/pppoe-interop.md` - the recipe and patch provenance sentence names `third_party/vpp-patches-v26.06/` (port phase), and one line names the product bundle action
- `ai/digests/vpp-dataplane.md`, `ai/INDEX.md` - the flow and the keywords

## Files to Create
- `internal/core/programbundle/` - the program registry, the Ze folder rule and its environment key, the manifest format, the checks, the command. No VPP name
- `internal/component/vpp/bundle.go` - registers `vpp`, holds the start check and the doctor check
- `internal/component/vpp/bundle_pin.go` - the identity this `ze` was built for, written by the tooling
- `internal/plugins/firewall/vpp/register_startup.go` - the ACL and NAT declarations
- `internal/plugins/iface/vpp/register_startup.go` - the VXLAN and GRE declarations
- `internal/le/build/vpp/bundle.go` - the bundle build, the plugin list writer and the pin writer
- `internal/appliance/vppbundle.go` - cache key, path and resolution
- `internal/appliance/instance/bundle.go` - the prepared config step
- `internal/appliance/rootsize.go` - the root size refusal
- `third_party/vpp-patches-v26.06/Dockerfile.bundle`, `bundle-plugins.txt` - the product bundle build and its plugin list
- The seven patches of the PPPoE series, in their v26.06 form, under `third_party/vpp-patches-v26.06/`
- `internal/le/test/qemu/vppbundle.go`, `internal/le/test/deployment/vppbundle.go` - the two proofs
- `test/vpp/bundle-start.ci`, `bundle-missing.ci`, `bundle-mismatch.ci`, `bundle-damaged.ci`
- The unit test files named in the test plan

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Two descriptions change in `ze-vpp-conf.yang`. No leaf is added: nothing about the program's location is configuration |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | `./le build vpp bundle` in `internal/le/build/vpp/register.go`. No `ze` command is added |
| CLI grammar (keyword before value) | Yes | The action takes no positional value; `./le cli grammar` judges it |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC is added |
| Pipe completeness | N-A | No `ze` command output is added |
| Env var registration | Yes | One key naming the Ze folder, registered with `env.MustRegister` in `internal/core/programbundle` beside its reader. The appliance image sets it; tests set it. `spec-vpp-failure-lifecycle` plans a `ze.vpp.binary` key for the same purpose and owes a change to use this one (Work Not Done) |
| Doctor check for runtime dependencies | Yes | The bundle check in `internal/component/vpp/bundle.go`, registered by the VPP component, with unit and `.ci` tests |
| Doctor code per problem class | Yes | `doctor-vpp-bundle-missing` (no bundle in the Ze folder, at start and in doctor); `doctor-vpp-bundle-mismatch` (another identity, at start and in doctor); `doctor-vpp-bundle-damaged` (a file differs from the manifest, in doctor); `doctor-vpp-acl-plugin`, `doctor-vpp-nat-plugin`, `doctor-vpp-vxlan-plugin`, `doctor-vpp-gre-plugin` (API absent from the running VPP), each registered by the package that owns the feature |
| Prometheus counters/metrics | N-A | A check at start whose outcome is a doctor finding, not a rate |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The VPP feature page under `features/`, found with `./le feature report` |
| 2 | Config syntax changed? | No | No leaf changes; the two YANG descriptions are help text |
| 3 | CLI command added/changed? | No | No `ze` command changes. The `le` action is documented by its own help |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | Four backends gain a startup declaration, covered by row 15 |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md`, `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | No protocol wire changes |
| 8 | Plugin SDK/protocol changed? | No | The registry is a core package, not in `pkg/` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md` and `docs/functional-tests.md` for the two proofs and the stub bundle fixture |
| 11 | Affects daemon comparison? | No | No comparison row names VPP packaging |
| 12 | Internal architecture changed? | Yes | `docs/research/vpp-deployment-reference.md`, `docs/architecture/appliance/build-artifacts.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/architecture/doctor-and-health-checks.md` for the new codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/release/delivery/spec-vpp-embedded-runtime.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/vpp.md`: every statement about where the VPP program comes from |

## Design presentation

### What changes

Ze stops expecting someone else to install VPP, and starts no VPP but its own. Ze has one patched VPP, v26.06 with every Ze patch. That build is a set of files in the Ze folder, beside `ze`: the appliance image build puts it in the image, and the one Ze package installs it on Linux. Ze finds it by scanning that folder and starts it. When it is missing or is not the build this Ze was made for, Ze still starts, keeps VPP down, and `ze doctor` says what to do. Nothing is embedded in the `ze` binary and nothing is unpacked.

### Before and after

1. **Appliance, VPP enabled.** Before: the image holds no VPP program, so the start of `/usr/bin/vpp` fails and nothing forwards. What the daemon as a whole does when the VPP plugin's start fails was not traced for this spec (A-14). After: the image holds `/usr/lib/ze/vpp`, `ze` starts VPP from it and logs the VPP process started and connected. The operator installs nothing.

2. **Ubuntu, VPP enabled.** Before: the operator installs a VPP package from the distribution or from FD.io so that `/usr/bin/vpp` exists. Ze starts it. A PPPoE config on VPP then fails `ze doctor` with "VPP PPPoE plugin APIs unavailable", because a stock VPP lacks Ze's patches, and the fix is to build a VPP by hand. After: the operator installs the Ze package. It places the launcher `/usr/bin/ze` and the directory `/usr/lib/ze/<release>/`, which holds `ze` and `vpp`. Ze starts that VPP. A distribution `vpp` package can stay installed; Ze does not read it and the two share no file.

3. **A Ze whose VPP files are missing.** Before: not a case, since Ze never looked for its own. After, in `ze doctor`: an error under `doctor-vpp-bundle-missing` saying that Ze's VPP was not found in `/usr/lib/ze/<release>`, that VPP was not started and Ze's other services run, and the command that reinstalls the package (or, on the appliance, the instruction to rebuild the image).

4. **A Ze and a VPP of different builds.** After, in `ze doctor`: an error under `doctor-vpp-bundle-mismatch` naming the build this Ze needs and the build found, and the fix.

5. **Package size.** Before: the package holds `ze`, about 120 MB as built on the development host. After: it also holds the VPP files, measured at about 63 MB without DPDK, so an estimated 190 MB or more unpacked once DPDK is in. Every package install carries VPP whatever data plane it uses. The build phase replaces this estimate with measured figures, packed and unpacked.

6. **Firewall on a VPP that Ze started.** Before: the rule is accepted at commit and VPP refuses it when applied, because Ze did not load the ACL plugin. After: Ze loads the plugin when the configuration has a firewall rule, and the rule is applied.

7. **One VPP version.** Before: v25.10 with the PPPoE patches for the PPPoE lab, and v26.06 with the IPsec patch for the deployment run. After: v26.06 with every patch, for the bundle, for both lab actions and for Ze's generated bindings.

8. **A host with a distribution VPP and no Ze VPP.** Before (the 2026-11-19 design): Ze starts `/usr/bin/vpp` as a fallback. After: Ze starts no VPP, and `ze doctor` reports `doctor-vpp-bundle-missing` with the command that reinstalls Ze.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| A fallback to `/usr/bin/vpp` when Ze's own VPP is missing | Withdrawn by the owner on 2026-11-28 |
| v26.06 for the first release, with the PPPoE patches ported after it | The owner chose both patch sets on one version before the first release |
| `ze` at `<prefix>/bin/ze` with the Ze folder at `<prefix>/lib/ze` | The owner chose the real `ze` inside the folder, behind a launcher script |
| A field that leaves the bundle out of a test image | The owner chose one path and no setting |
| The bundle embedded in the `ze` binary and unpacked at start (the 2026-11-19 design) | Withdrawn by the owner on 2026-11-28. It also needed a packaging tag, an unpack library, a storage setting and a permanent path under `/perm` |
| A bundle linked for one fixed prefix, started as an ordinary program | It cannot run from a version directory under `/usr/local`, where the owner wants several versions side by side. The process would show as `vpp`, which is its only gain |
| A host directory, not an archive, in the image config | The packer follows symbolic links in a directory, so every library link would become a second copy of the library |
| Hashing every bundle file at each managed start | It delays the restart after a VPP crash, on a root that is read-only and already checked at update |
| A VPP path written into the appliance config file in the repository | The archive path is a cache path known only at build time, so the prepared config is where it can be written, as the kernel notice is |
| A VPP built against each distribution's own libraries | One build per distribution and architecture, and the appliance still needs the self-contained one. A-12 is the proof that one bundle is enough |
| A second package for the VPP files | The owner decided one package |

### Documents departed from

| Document | Section | What it states | What this design does instead |
|----------|---------|----------------|-------------------------------|
| `docs/labs/pppoe-interop.md` | VPP mechanism image | "Its recipe and patch provenance live in [`third_party/vpp-patches`](../../third_party/vpp-patches/README.md)." | The port phase deletes `third_party/vpp-patches/`; the recipe and the PPPoE series live in `third_party/vpp-patches-v26.06/`, and the page is corrected in that phase |

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| N-8 | Does accepting PPPoE on VPP as a feature move into the first release with the port? You decided that every patch moves to v26.06 before the first release. The port itself (patches, pin, Ze's generated bindings, the PPPoE proofs re-run) is therefore first-release work, and this spec carries it as its first phase. `plan/post-release/features/spec-pppoe-vpp-dataplane.md` is after the release and in progress. Example: the first release ships a bundle whose VPP has the PPPoE plugin with Ze's patches; does the release also announce and test "PPPoE subscribers terminated in VPP"? | (a) recommended: only the port moves. PPPoE on VPP stays a feature after the release, is not announced, and its spec builds on v26.06 from then on. (b) the feature's acceptance moves into the first release too, and that spec moves to the release bucket | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |
| N-7 | Does "use that version everywhere" include the GoVPP bindings every VPP backend links? They are vendored from upstream GoVPP v0.13.0, which upstream generated from VPP 25.10 (`vendor/go.fd.io/govpp/binapi`, 32 packages, headers "VPP: 25.10-release"; recorded by the server session in handover 64, not re-read here). Ze's own generated bindings (PPPoE, feature, memclnt) are regenerated from v26.06 in any case. A second case of the same question is `third_party/vpp-linux-cp`, a reference copy of the linux-cp plugin source whose README says it is frozen at v25.10. Example: the interface backend calls a message through a binding generated from 25.10 while the VPP it talks to is 26.06 | (a) recommended by this design on 2026-11-28, reading your word "everywhere" (handover 64 recorded no recommendation): regenerate them from Ze's v26.06 into the tree, so no binding comes from another version, and refresh the reference copy to v26.06. Its cost is not measured: which messages differ between 25.10 and 26.06 was not read. (b) keep upstream's bindings and the reference copy at v25.10, and rely on the message check Ze runs against the live VPP at connect | (a), taken on the design's recommendation, 2026-11-28. Not his choice in his words: he declined further questions and may change it |

## Implementation Steps

1. **Phase: Port every patch to v26.06 (FIRST)** -- one patched VPP version (owner, 2026-11-28)
   - Tests: `TestOneVPPPin`, the PPPoE proofs under the patch directory re-run on v26.06, `./le build vpp run` and `./le build vpp deployment` built from the one pin
   - Files: `third_party/vpp-patches-v26.06/` receives the seven patches of `third_party/vpp-patches/`, its Dockerfile gains the PPPoE plugin, Ze's generated bindings are regenerated, the GoVPP bindings vendored under `vendor/go.fd.io/govpp/binapi` are regenerated from v26.06, `third_party/vpp-linux-cp` is refreshed to v26.06 (N-7, taken as (a)), `internal/le/build/vpp/register.go` reads one pin, `third_party/vpp-patches/` is removed, and `docs/labs/pppoe-interop.md` names the v26.06 directory
   - Verify: A-15 settled patch by patch before anything else. A patch with no v26.06 form stops the phase and is reported to the owner. The session working `spec-pppoe-vpp-dataplane` is told before the phase starts (R-12)
2. **Phase: Bundle build and the two real proofs** -- the archive, the cache, real VPP on the appliance and in a container
   - Tests: `TestBundleActionWritesCacheEntry`, `TestBundleArchiveHoldsOnlyWhatThePackerKeeps`, `TestBundleCacheKeyChangesWithPin`, `TestBundleCacheIsPreservedByCleanup`, a first form of `./le test qemu vpp-bundle` and `./le test deployment vpp-bundle` that start VPP by hand from the placed bundle
   - Files: `Dockerfile.bundle`, `bundle.go`, `vppbundle.go`
   - Verify: A-2, A-3, A-4, A-5, A-7, A-8, A-9, A-11, A-12 and A-13 settled before any daemon code depends on them. The sizes recorded on the build-artifacts page
3. **Phase: Program registry and the VPP component** -- the registry, the folder rule, the checks, the command
   - Tests: every `internal/core/programbundle` row, `TestManagedStart*`, `TestExternalModeNeverScans`, `TestStartupConfNamesBundlePluginDirectory`, `TestBuiltinPluginsDeclaredOnce`, `TestBundleDoctorCheck`, `TestBundlePinMatchesPatchDirectory`, the four `.ci` tests
   - Files: `internal/core/programbundle/`, `bundle.go`, `bundle_pin.go`, `register.go`, `vpp.go`, `startupconf.go`, the YANG descriptions
   - Verify: the wiring test passes with the stub bundle. A-6 settled
4. **Phase: Image placement** -- the prepared config, the two refusals
   - Tests: `TestPrepareAddsBundleEntry`, `TestPrepareRefusesTakenDestination`, `TestApplianceBuildRefusesMissingBundle`, `TestApplianceBuildRefusesOversizedRoot`, the full `./le test qemu vpp-bundle`
   - Files: `instance/bundle.go`, `prepare.go`, `cmd_build.go`, `rootsize.go`
   - Verify: the image's own `ze` starts real VPP
5. **Phase: Plugin declarations** -- ACL, NAT, VXLAN, GRE; the linux-cp check
   - Tests: the four plugin-selection rows, `TestStartupFeatureWithoutSection`, `TestBackendPluginAPIsAreDeclared`, `TestBundlePluginListMatchesDeclarations`, `TestLCPPluginCheckReadsAPITable`, the firewall step of the QEMU proof
   - Files: the two `register_startup.go` files, `startup_feature.go`, `iface/vpp/doctor.go`, the PPPoE diagnostic text
   - Verify: the firewall step is run before the declaration and recorded red (A-10), then green. Each declaration is its own commit

Each phase edits the pages its changes make wrong before the next phase starts (`ai/rules/documentation.md`).

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| One program source | No code path tries the bundle and then another path |
| One VPP version | No patch directory, pin, Dockerfile or lab image names a VPP version other than v26.06 |
| No VPP name in the registry | `internal/core/programbundle` names no VPP file, plugin or path |
| One declaration per plugin | No plugin file name appears in two Go literals. The bundle's list is written from the declarations |
| Ze always starts | No bundle failure ends `ze start` |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, and every finding names the code the Integration Checklist lists (`ai/rules/user-facing-errors.md`) |
| Licence | Each component's licence is named in the manifest and its text is in the archive |
| Rule: `ai/rules/platform-linux.md` | Linux-only code carries a QEMU proof and no host binary is cross-compiled |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| An appliance image whose `ze` starts real VPP | `./le test qemu vpp-bundle` |
| A Linux `ze` that starts real VPP from its own folder | `./le test deployment vpp-bundle` |
| The bundle in the cache for each supported architecture | `./le build vpp bundle` answers "cached" on the second run |
| The four plugin declarations | `TestBackendPluginAPIsAreDeclared` |
| Measured sizes on the build-artifacts page | Read the page |
| The guides say where VPP comes from | `./le doc check all` and a read of `docs/guide/vpp.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Manifest entries | Paths are relative and stay inside the bundle |
| Who can write the Ze folder | Root, in the package and in the image. On a `/usr/local` install the `ze` user may own the version directory (owner, 2026-11-28); which owners are accepted is choice Q-4 of `spec-release-build`. The start check refuses a bundle directory that group or others can write |
| The environment value | It selects a directory of programs Ze runs with VPP's privileges. Only whoever starts Ze can set it |
| Build provenance | The VPP source archive checksum is verified, patches apply with no fuzz, the base image is pinned by digest |
| Error text | A finding names paths and causes and never prints file contents |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-2 proves false (the loader cannot be run directly) | DESIGN: the bundle is linked for one prefix, and the version directory design is told it cannot move the bundle |
| A-4 proves false (the root does not fit) | Stop and report the measured sizes to the owner |
| A-15 proves false (a patch has no v26.06 form) | Stop and report the patch and the upstream change to the owner: one version is his decision |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Work Not Done

| Item | Why not here | Spec that owns it |
|------|--------------|-------------------|
| The DEB, the RPM and the Linux tarball carry `<prefix>/lib/ze/<release>/vpp`, and the bundle enters the release build as a digest-pinned input | That spec owns package assembly. Written into it on 2026-11-28 (its Data Flow, "What the release artifacts carry", AC-13 and AC-14); its design is not yet approved | `plan/release/delivery/spec-release-build.md` |
| The versioned install: version directories, the launcher, the listing and removal, the tarball with final paths, a self-update that installs a version directory | A design of its own, none of it VPP. Written into that spec on 2026-11-28; its design is not yet approved | `plan/release/delivery/spec-release-build.md` |
| The package payload rule and AC-10 declare the launcher and the version directory with its VPP, and the LGPL source archives are published beside each package | That spec owns the payload rule. AC-9, AC-10 and its Task were amended on 2026-11-28; the sentence "The package must install only the normal `ze` Linux daemon and its operational support files" sits in its historical record and is superseded by the amended AC-10 | `plan/release/delivery/spec-release-distribution.md` |
| `ze.vpp.binary`, R-2 and the "Embedded runtime start" row name the embedded runtime and a program path | That spec is the reader of this design. Not edited by this work | `plan/post-release/features/spec-vpp-failure-lifecycle.md` |
| That spec's patches, Dockerfiles, proofs and bindings are named at v25.10 (`third_party/vpp-patches/`). After the port phase of this spec they live on v26.06. The feature's acceptance does not move into the first release (N-8, taken as (a)) | It is PPPoE work, in progress in another session. Not edited by this work | `plan/post-release/features/spec-pppoe-vpp-dataplane.md` |

## Design Insights

- The first design embedded the bundle so that one `ze` file carried VPP everywhere. The owner replaced the carrier, not the goal: VPP still always comes with Ze, as files.
- The appliance and Linux differ in one fact only: who decides where `ze` sits. gokrazy puts it in `/user`, so the appliance is told its Ze folder; everywhere else Ze derives it.
- A stock VPP is not enough for Ze: the PPPoE backend refuses one. That is the practical reason Ze carries its own.
- The proofs of firewall and tunnels on real VPP all ran against a VPP the lab started with every plugin on. A feature proven only against a permissive environment says nothing about the restrictive one Ze itself creates.
- Ze has no single patched VPP today. Carrying "our VPP" forces that question, which no lab had to answer.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The bundle is files in a Ze folder beside `ze` | Embedded in `ze`; a container image | Owner decision, 2026-11-28 |
| Ze starts only its own VPP | A fallback to `/usr/bin/vpp` | Owner decision, 2026-11-28 |
| One VPP version, v26.06, with every Ze patch | Two pins; the PPPoE series ported after the release | Owner decision, 2026-11-28 |
| Every image carries the bundle | A field that leaves it out | Owner decision, 2026-11-28 |
| The folder is the directory of the resolved executable, or the one a single environment value names | A compiled-in path; a config leaf; `<prefix>/lib/ze` beside `<prefix>/bin/ze` | Owner decisions ("scan that folder", "like B"). The value exists because gokrazy chooses where `ze` sits |
| The loader is run directly with the library directory | A bundle linked for one prefix | The bundle must run from any version directory |
| The bundle is an archive in the image config, not a directory | A host directory | The packer copies through symbolic links in a directory |
| The start check compares identity; the full comparison is doctor's | Hashing at each start | Restart time after a crash |
| Programs register; the scan names none | A VPP-only lookup in the component | Owner wish for a second program; the registration rule |
| Plugins are declared in the startup feature registry, and the bundle's list derives from it | A hand-written list | One declaration |
| The linux-cp check reads the API message table | Shipping `vppctl` in the bundle | The registry already gives that check |

## Known Limitations
- One VPP version per `ze`.
- A bundle exists only for the architectures Ze builds a runtime kernel for.
- The process started through the loader does not show as `vpp` in every tool.

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
