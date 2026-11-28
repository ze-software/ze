# Spec: onie-3 -- the ONIE installer: one Go program that ONIE runs

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Priority | P3 |
| Release reason | Installing from ONIE is added platform support for switches; the ISO and PXE installers already cover the first release. |
| RFC gaps | - |
| Depends | spec-onie-1-hardware-model (the model names the switches an installer accepts), spec-onie-2-relative-partitions (Ze beside other partitions, and the placement function), spec-appliance-perm-grows-at-install (growth) |
| Phase | DESIGN |
| Handoff | - |
| Design tier | strongest tier |
| Updated | 2026-11-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-onie-0-umbrella.md` (owner decisions D-1 to D-5, the research on ONIE's contract and on both switches, and the commands to run on the switches).

## Task

Give Ze an installer that ONIE runs, as the bootable ISO is the installer a PC
firmware runs. Thomas asked for "a similar installer like we have for iso",
studied from NOSaic's, and ruled three things: it is pure Go, ONIE survives the
install, and the first switches are the Dell S4048-ON and S5232F-ON.

On the build host, `ze-host appliance onie <name>` produces one file for an
appliance whose model is a switch. The file is Ze's existing installer program
with the compressed image and a small description appended to it.

On the switch, the operator types `onie-nos-install` with the URL of that
file. ONIE downloads it, marks it executable and runs it. The program checks
that the switch is one its model accepts, adds Ze's partitions after ONIE's and
the vendor diagnostics, writes and verifies them, switches the boot path to
Ze, records in ONIE's boot environment that an operating system is installed,
and reboots. The boot switch and that record are the last two writes, in that
order, so that ONIE stays in charge until everything else has succeeded.

On the running switch, one Ze command starts ONIE again, in install, rescue or
uninstall mode, and a read command says what ONIE state the switch is in. Ze
starts ONIE by loading ONIE's own kernel from ONIE's partition and jumping to
it, so the boot path on disk is not touched: leaving ONIE's rescue mode with a
reboot comes straight back to Ze.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`
  → Constraint: every RPC carries a YANG registration, a command is declared once and reaches every interface, and a plugin can run in another process.
  → Constraint: a compile-out-able feature is declared once in `feature-gates.txt` and always-on code never imports it.
- [ ] `ai/doors/appliance.md`
  → Constraint: the installer is one static Go binary with zero external binaries; it never adds an `exec.Command` of a tool and does not import the `iface` component.
  → Constraint: a target binary is cross-compiled, a host binary never is.
- [ ] `docs/architecture/appliance/iso-installer.md`
  → Decision: "Extend the one initrd with an ISO source mode rather than ship a second init." The ONIE installer is a third source mode of the same program.
  → Decision: a missing checksum is a hard error with no override; the image is gzip-compressed for transport and the installed bytes do not change.
  → Constraint: implicit target selection is refused on a machine with more than one candidate disk.
- [ ] `docs/architecture/appliance/installer-initrd.md`
  → Constraint: "PID 1 cannot exit": a fatal error routes to a rescue console. Under ONIE the program is an ordinary process, and ONIE re-runs a failed installer every 20 seconds, so its fatal path is a non-zero exit.
  → Constraint: every line that names a URL goes through `redact.URL`.
- [ ] `docs/architecture/appliance/on-device-installer.md`
  → Constraint: build-time and on-device writes share verifiers and validators, not one write function.
- [ ] `ai/rules/cli.md`
  → Constraint: the verb follows the effect on live state: `show` for a read, `request` for an operational action; a keyword from a closed set comes before any value; the `--flag` form belongs to offline tooling only.
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: every message says what is wrong, why, and the exact command that fixes it. Under ONIE the fix is often an ONIE command (`onie-uninstall`).
- [ ] `ai/META-DECISIONS-SITUATIONAL.md`, "Test systems and labs"
  → Decision: ONIE's virtual machine image may boot in the QEMU harness, pinned by checksum, as the third party under test (owner rule of 2026-11-28).
- [ ] `ai/rules/interop-and-goal-validation.md`
  → Constraint: before a check expects the other implementation to accept or do something, read that behaviour in its source at the version the lab runs and name the function; a test's discrimination is proven by reverting the change and seeing it go red.

**Key insights:**
- ONIE runs the file directly (`exec_installer` `run_installer`); its only content check refuses a file carrying its updater marker string.
- ONIE keeps ONIE-BOOT mounted read-write while the installer runs, so the kernel cannot reread the whole partition table; new partitions are announced to the kernel one by one.
- Under legacy BIOS there is no boot menu between ONIE and Ze: whichever boot code sits in the first sector wins. Under UEFI the firmware's boot order decides. A way back that rewrites either one makes a visit to ONIE one-way.
- Ze's runtime kernel can load another kernel from a file and jump to it (`CONFIG_KEXEC_FILE=y` in `gokrazy/kernel/kernel.config`; gokrazy's own reboot uses it). ONIE's kernel and initial ramdisk are files on ONIE-BOOT.
- In install mode ONIE reinstalls its own boot code in the first sector by itself (`bios_boot_onie_install`), so reinstalling and uninstalling need no help from Ze to give the disk back to ONIE.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/install/disk/run.go`, `validate.go`, `cmdline.go` - two source modes, HTTP and ISO, chosen by switches in `Run`, `RunInitrd`, `validateSource` and `selectFatalBranch`; inputs come only from the kernel command line; HTTP ends in a reboot and ISO in a poweroff.
- [ ] `internal/install/disk/detect.go` - `findTargetDisk` refuses several candidate disks; `partitionPath`.
- [ ] `cmd/ze-installer/main.go` - the entry point, built only under the `ze_installer` tag, calls `RunInitrd`.
- [ ] `internal/appliance/cmd_iso.go` - `runIso`, `resolveISOInput`, `readRequiredImageChecksum`, `compressOpenFileGzip`, the temp-then-rename output.
- [ ] `internal/appliance/cmd_initrd.go` - `defaultInitrdMakeBuild` cross-compiles the installer program for the appliance's architecture.
- [ ] `internal/appliance/main.go` - `applianceCommands`, the package's command table.
- [ ] ONIE `rootconf/default/bin/exec_installer` (upstream master, read 2026-11-28) - `chmod +x`, direct execution, `get_image_type`, `finish_nos_install`.
- [ ] ONIE `rootconf/grub-arch/sysroot-lib-onie/onie-blkdev-common`, `nos-mode-arch`, `boot-mode-arch`, `installer/grub-arch/install-arch` (reported by the research agent with quoted lines) - the partition layout, the type GUID of ONIE-BOOT, the boot environment file and its two variables.
- [ ] `internal/core/ipc/yang/ze-system-cmd.yang` - the existing `reboot` command and its RPC, whose graceful shutdown the return command reuses.
- [ ] `internal/install/disk/initrd_linux.go` - a third switch on the source (line 58), and the HTTP mode as the default (line 20) (read by the design judge).
- [ ] `cmd/ze/setup_features_appliance.go` - links on-device runtime features under the `ze_appliance` tag; `internal/plugins/coldplug` is linked there.
- [ ] `internal/appliance/cmd_initrd.go` - `defaultInitrdMakeBuild` runs the Go toolchain as a child process to cross-compile the installer program (line 152).

**Behavior to preserve:**
- The ISO and PXE installers behave exactly as today, including the rescue console and the poweroff.
- `ze appliance iso` is unchanged.
- The installer program stays one static binary with no external tool.

**Behavior to change:**
- The installer's source modes become a registered table, and a third mode reads the image appended to the program itself.
- `ze appliance onie` exists.
- A running Ze on an ONIE switch can report ONIE's state and reboot into ONIE.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The installer and its source modes | `internal/install/disk/run.go` `Run`, `RunInitrd`; `iso-installer.md` "Decisions" | Two modes behind switches | extends: the modes become registered values and a third is added |
| The installer program | `cmd/ze-installer`; `cmd_initrd.go` `defaultInitrdMakeBuild` | A static target binary, PID 1 of the initrd | extends: the same binary, built the same way, is the head of the ONIE file and starts as an ordinary process there |
| The ISO builder command | `internal/appliance/cmd_iso.go` | Validates, requires the checksum, compresses, wraps | extends: `onie` reuses the first three steps; the wrapping is an append instead of GRUB and xorriso |
| Checksum discipline | `cmd_iso.go` `readRequiredImageChecksum`; `internal/install/disk` `extractSHA` | Requires and verifies SHA-256 of the image | extends: the appended description carries the digests, verified before the boot path changes |
| Target safety | `internal/install/disk/detect.go` `findTargetDisk`, `validate.go` `isSkippedDisk` | Refuses ambiguous or unsuitable targets | extends: under ONIE the target is the one disk holding ONIE-BOOT, and ambiguity is refused the same way |
| Placement and the run | `internal/core/gptdisk` placement (`spec-onie-2-relative-partitions`) | Lays an image's four partitions after existing entries | does not change it: this spec calls it |
| Hardware model | the model registry (`spec-onie-1-hardware-model`) | Names a box and its facts | extends: the model format gains the switch's memory, which bounds the installer file |
| Kernel partition announcements | `internal/core/gptdisk/kernel_linux.go` `ResizeInKernel` | Tells the kernel a partition's new size while the disk is in use | extends: the sibling call that adds a partition |
| The reboot command | `internal/core/ipc/yang/ze-system-cmd.yang` `reboot`, RPC `daemon-reboot` | Shuts the daemon down gracefully, then reboots the device | extends: the return command runs the same graceful shutdown and ends in a jump to ONIE's kernel instead of a firmware reboot |
| Kernel-to-kernel jump | `gokrazy/kernel/kernel.config` `CONFIG_KEXEC_FILE=y`; gokrazy `reboot_amd64.go` `kexecReboot` | Reloads Ze's own kernel from its boot partition without going through the firmware | extends: the plugin loads ONIE's kernel and ramdisk from ONIE-BOOT the same way |
| On-device runtime features | `cmd/ze/setup_features_appliance.go`, tag `ze_appliance`; `ai/doors/appliance.md` "`ze_appliance` is reserved for on-device runtime features" | Links features that only make sense on the appliance (`coldplug`) | extends: the ONIE plugin is linked there. No new feature gate is added |
| Installer QEMU proofs | `internal/le/test/qemu`, proof kinds registered by `spec-onie-2-relative-partitions` | Boot an installer, install, boot the result | extends: a proof that boots ONIE's virtual machine and installs through it |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Adding a command to one interface by" | agrees | The two runtime commands are declared once, with YANG, in the plugin that owns them, and reach every interface through the command registry |
| `docs/architecture/appliance/on-device-installer.md` | Decisions | `// Design:` page of `run.go`, `validate.go`; a page this spec modifies | "- The installer registers as `ze install disk` under the existing subdispatch," | agrees | The ONIE mode is a source mode of that same installer, not a new binary; the page gains the mode and the registered table of modes |
| `docs/architecture/appliance/iso-installer.md` | Decisions | anchored to `run.go`, `actions.go`; a page this spec modifies | "- Extend the one initrd with an ISO source mode rather than ship a second init." | agrees | The same reasoning gives a third source mode. The page's refusal of an implicit target on a machine with several candidate disks holds here too: the target is the one disk holding ONIE-BOOT, and two such disks are refused (AC-21). The page's statements about the ISO (the raw image written unchanged, no partition rebuilt, poweroff at the end) stay true of the ISO; the page gains a pointer to the ONIE page, where the image's partitions are placed after existing ones and the end is a reboot |
| `docs/architecture/doctor-and-health-checks.md` | Where each owner registers its check | anchored to `run.go` | "A `runtime` `rpc.DoctorCheckDecl` in the `DoctorChecks` of the `sdk.Registration` it sends at Stage 1, answered by its `OnDoctorCheck` handler." | agrees | The two ONIE codes are checks over state the plugin's own process reads, so they are declared exactly that way |
| `ai/doors/appliance.md` | Boundaries | the door covering the `internal/install/disk` and `internal/appliance` files | "The initrd is one static Go binary with zero external binaries." | agrees | The ONIE file's head is that same binary, and the mode adds no `exec.Command`; the critical review greps for it |
| `docs/architecture/appliance/installer-initrd.md` | PID 1 cannot exit | `// Design:` page of `rescue_linux.go`, `initrd_linux.go`, `cmd/ze-installer/main.go`; a page this spec modifies | "PID 1 exiting panics the kernel, so a fatal error routes to a recovery console" | agrees | The rule is about PID 1 and stays as written for the initrd. Under ONIE the program is not PID 1, ONIE re-runs a failed installer, and the mode's fatal path is a non-zero exit; the page gains that case |
| `docs/architecture/appliance/builder.md` | Constraint the code does not state | `// Design:` page of `main.go` | "The dispatch table is built when it is called, not at package load. The" | agrees | `onie` is added the way `iso` is: a handler variable assigned from the command file's `init()` and a row in the package's own table |
| `ai/doors/core.md` | Where it sits | the door covering the `internal/core` packages | "A core package is a library with no" | agrees | The environment block, the EFI boot entry and the saved record are three leaf libraries with no config lifecycle and no registry side effect |
| `docs/architecture/core-design.md` | Command engine and composition roots | `// Design:` page of `actions.go` | | silent | The page documents the command engine le shares with ze; this spec adds one test action through it and changes nothing the page states |
| `docs/architecture/appliance/gokrazy-build-pins.md` | Why a module cache is checked in | anchored to `actions.go` | | silent | This spec adds no patch and changes no pin; the runtime patch belongs to `spec-onie-2-relative-partitions` |
| `docs/architecture/testing/ci-workflows.md` | Where each suite runs | anchored to `actions.go`; a page this spec modifies | "`./le test qemu appliance-ota` is a manual appliance-delivery proof, outside" | agrees | The ONIE proof is a manual appliance proof of the same kind, outside the scheduled populations, because it needs the pinned ONIE image. The page gains it in this work, with what it is run for |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `actions.go` | | silent | The page covers the Docker interop suites; the ONIE proof is a QEMU installer proof |
| `docs/architecture/testing/qemu-integration.md` | The two entry points | anchored to `actions.go`; a page this spec modifies | "The own-system command carrier can run a whole population in one VM. These" | agrees | The installed switch runs Ze's own kernel and system. The proof also boots ONIE's image as the third party under test, which the owner rule of 2026-11-28 admits; the page gains that proof and that rule's wording |
| `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` | Why the codec was written from the RFC tables alone | anchored to `actions.go` | | silent | The page covers VRRP; it is linked only through the shared QEMU action table |
| `ai/doors/le-tooling.md` | Where it sits | the door covering the `internal/le/test/qemu` and `test/` files | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | The three `.ci` files sit in existing suites, and the proof registers as an installer proof kind in the existing tool |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/onie` | "- **A plugin owns its whole surface.** Its commands, YANG, doctor checks and help" | agrees | The ONIE plugin holds its two commands, their YANG, and its two doctor codes, and compiles out under `ze_onie` |
| `docs/architecture/appliance/README.md` | Appliance architecture | a page this spec modifies | "Start with `builder.md` for the build host and `installer-initrd.md` for the" | agrees | The table gains a row for `onie-installer.md`; the reading order the page gives is unchanged |
| `docs/functional-tests.md` | Install Tests (`test/install/`) | a page this spec modifies | "Install functional tests cover the offline install surface and the appliance" | agrees | The ONIE proof and the two builder tests are listed with the other installer tests |
| `docs/architecture/cli/plugin-modes.md` | Modes Overview | `// Design:` page of `cmd/ze/setup_features_appliance.go` | | silent | The page covers the operating modes of a plugin; this spec adds one blank import to that feature file, and the plugin runs in the standard modes the page describes |
| `docs/architecture/appliance/kernel-profiles.md` | Loading driver modules on the device | anchored to `cmd/ze/setup_features_appliance.go` | | silent | The section covers the module loader linked from the same file; this spec adds an import beside it and changes nothing about how modules load |
| `ai/doors/hub-engine.md` | Where it sits | the door covering `cmd/ze/setup_features_appliance.go` | | silent | The door covers the hub and the engine. This spec adds one blank import to a feature file and changes no engine or hub behaviour; the plugin reaches the daemon through registration only |
| `docs/plugin-overview.md` | Plugin Architecture | a page this spec modifies | "An overview of Ze's plugin system. For writing your own plugins, see" | agrees | The ONIE plugin is an ordinary system plugin registered the way the page describes; the page's inventory gains it |
| `docs/architecture/api/commands.md` | Overview | a page this spec modifies | "Ze uses verb-first command paths with JSON or text encoding." | agrees | The two commands are declared once with YANG in the plugin that owns them and reach every interface through the registry; the page gains them |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show onie` names its subject and stops, and its answer is structured data for the pipes. In `request onie start mode <value>` the value is the action's own parameter, which mode ONIE starts in, not a scope, a presentation or a format |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `show onie` overrides no pipe operation; the generic operators act on its answer |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | No pipe changes what either command does; `request onie start mode` acts only through its own keyword and value |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | The ONIE plugin configures no service. On a device with no ONIE, or with a damaged return path, the daemon starts and the plugin reports the state under its doctor code, which is what this decision asks |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec configures no data-plane feature and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | The ONIE proof is a QEMU installer proof, not a Docker interop suite, and it declares no kernel capability |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build host: `ze-host appliance onie <name>`, reading the appliance config, its model, its built image and the image's checksum file.
- Switch, install: ONIE executes the file; the program reads its own executable, `/etc/machine.conf`, the disk's partition table, and `/sys/firmware/efi`.
- Switch, runtime: `show onie` and `request onie start mode <install|rescue|uninstall>` through the daemon's command registry.

### Transformation Path
1. Build: the installer program is cross-compiled for the model's architecture by the Go toolchain the builder already runs for the initrd; the image is gzip-compressed; a description is written (format version, model, accepted platform strings, firmware modes, the image's four partition entries, SHA-256 of the compressed payload and of each partition's bytes); program, payload and description are concatenated, with a fixed-size footer locating them.
2. Build refusals: a model with no ONIE platform strings; a file larger than half the model's stated memory; a file containing ONIE's updater marker string.
3. Install, checks (nothing written yet): the footer and description are read and the payload's digest verified; the machine file's platform string must be one the model accepts; the firmware mode is read; the one disk holding an ONIE-BOOT partition is found; every other partition is classified as ONIE's, the vendor's diagnostics, an earlier Ze run, or foreign, and a foreign partition is refused.
4. Install, writes that leave ONIE bootable: Ze's run is placed after the existing entries; each new partition is announced to the kernel; the four partitions are streamed from the payload and their digests verified; the persistent partition is grown.
5. Install, the last two writes: first the boot switch. Under BIOS the first sector's 440 bytes of ONIE boot code are saved in a checksummed record, read back, and replaced by Ze's boot code re-targeted for the run's position; under UEFI a firmware boot entry for Ze's boot partition is created and put first, ONIE's entry staying in the list. Then ONIE's boot environment is marked as holding an installed operating system. If the mark fails after the switch, the program exits non-zero; ONIE re-runs it, and the repeat is safe.
6. Install, end: buffers are flushed and the program reboots the switch itself.
7. Start ONIE: the plugin mounts ONIE-BOOT read-only, reads ONIE's kernel and initial ramdisk and the kernel arguments ONIE's own boot files state, appends the boot reason for the requested mode, runs the daemon's graceful shutdown, and jumps to ONIE's kernel. Nothing on disk is written.
8. Back to Ze: from rescue mode, ONIE's own reboot goes through the firmware to the boot path, which is still Ze's. From install mode ONIE takes the boot path back itself and the installer runs again. From uninstall mode ONIE erases Ze's partitions.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build host ↔ switch | the installer file's footer and description | No |
| Installer ↔ ONIE | exit status, `/etc/machine.conf`, the GPT, ONIE's boot environment file, EFI variables | No |
| Daemon ↔ runtime plugin | the command registry and its RPC; the plugin may run in another process | No |
| Runtime plugin ↔ ONIE | ONIE-BOOT read-only, ONIE's kernel started in memory | No |

### Changed-Wire Census

**Search evidence:** the wires are the installer file (new), ONIE's boot environment block, the EFI boot variables, the saved boot code record (new), the first sector's boot code, and the GPT. Scope searched for existing readers and writers: this repository for `grubenv`, `efivars`, `BootOrder` and `machine.conf` (no hit in product code; `efivarfs` is built into the kernel and unused, as the research agent reported), and upstream ONIE for the writers and readers named below. Positive control: `grubenv` is found in ONIE `onie-blkdev-common` `onie_setenv`. The GPT and boot code rows of `spec-onie-2-relative-partitions` are not repeated; this spec adds one writer to each.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| installer file | writer | `internal/appliance/cmd_onie.go` (new) | new | versioned by the description's format number; a program refuses a description it does not know |
| installer file | reader | the ONIE source mode of `internal/install/disk` (new) | new | same |
| installer file | reader | ONIE `exec_installer` `get_image_type`, `run_installer` | read upstream: the marker check and the direct execution | preserved: the file is an executable without the marker |
| ONIE boot environment block | writer | ONIE `onie_setenv` through `grub-editenv`; `onie-boot-mode`; `onie-nos-mode` | `onie-blkdev-common`, `nos-mode-arch` | preserved: Ze writes the same block format and only the two variables ONIE's own tools write |
| ONIE boot environment block | writer | the installer of this spec | new | new; the runtime plugin only reads it |
| ONIE boot environment block | reader | ONIE's GRUB (`load_env`), ONIE `boot-mode-arch` `check_nos_mode_arch` | read upstream | preserved: the block keeps its signature, size and padding |
| EFI boot variables | writer | ONIE `uefi_boot_onie_install` through `efibootmgr` | `onie-blkdev-common` | preserved: Ze adds its own entry and reorders, never deleting ONIE's |
| EFI boot variables | writer | the installer of this spec | new | new; the runtime plugin only reads them |
| EFI boot variables | writer | ONIE's uninstall, which removes a NOS entry | ONIE `uninstall-arch` (reported through a summarising fetch, not read verbatim; assumption A-10) | preserved: Ze's entry is an ordinary boot entry ONIE can remove |
| EFI boot variables | reader | the UEFI firmware | outside the repository | the entry layout is read from the UEFI specification in phase 1 (assumption A-4) |
| saved boot code record | writer | the installer of this spec | new | versioned and checksummed |
| saved boot code record | reader | the runtime plugin of this spec, for `show onie` only | new | same |
| first-sector boot code | writer | the installer (Ze's code) | a new writer beside those of `spec-onie-2` | 440 bytes only; the disk signature and MBR partition records stay |
| first-sector boot code | writer | ONIE in install mode, which reinstalls its own boot code on every boot until an operating system is marked installed | ONIE `onie-blkdev-common` `bios_boot_onie_install` | relied on: it is how a reinstall or an uninstall gives the boot path back to ONIE |
| GPT | writer | the installer, through `gptdisk` placement | `spec-onie-2` | ONIE's and the diagnostics entries are never rewritten |
| GPT | writer | ONIE's uninstall, which deletes every partition that is not ONIE's or diagnostics | ONIE `uninstall-arch` (assumption A-10) | relied on by AC-10 |
| first-sector boot code | reader | the BIOS, then ONIE's or Ze's boot code itself | outside the repository | preserved: the disk signature and the MBR partition records are untouched, and only one of the two known 440-byte programs is ever in place |
| GPT | reader | the Linux kernel, the UEFI firmware, and ONIE's own tools (`sgdisk`, `blkid`, its `discover-arch` partition scan) | ONIE `discover-arch` `skip_parts_arch`, `onie-blkdev-common` | preserved: a valid table whose first entries are byte for byte what ONIE wrote |
| `/etc/machine.conf` | writer | ONIE `gen-config.sh` and its image build | `rootconf/default/etc/init.d/gen-config.sh` | preserved: Ze never writes it |
| `/etc/machine.conf` | reader | the installer | written by ONIE `gen-config.sh` | read-only |

### Integration Points
- `gptdisk` placement and growth, the boot-code re-target function - called, not changed.
- The model registry - read at build; its accepted platform strings travel in the description.
- The daemon's graceful shutdown, as `daemon-reboot` runs it - reused before the jump to ONIE.

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
| A-1 | A static Go program starts under the factory ONIE of the S4048-ON | Go's release notes require Linux 3.2 (stated for 1.24; the 1.25 to 1.27 notes name no higher floor); upstream ONIE for this machine builds kernel 3.2.69; the unit was not seen | Pure Go cannot install on that switch under its factory ONIE; the route is then to update the switch's ONIE first, which is Thomas's decision | `uname -r` on the switch, then running a trivial static Go program there | unvalidated |
| A-2 | The factory ONIE of each switch behaves as upstream master does (direct execution, the machine file, the layout, the boot environment file) | Only upstream source was read | The installer is refused or misreads the switch | The umbrella's switch commands, run by Thomas; and a recorded install on each unit | unvalidated |
| A-3 | The sectors between the partition table and the first partition are unused on an ONIE disk, so the saved boot code record can live there | ONIE's installer starts its first partition at sector 2048 (`install-arch`); with a GPT, GRUB embeds in the GRUB-BOOT partition, not in that gap | The record needs another home | Read the gap on the ONIE virtual machine's disk and on each unit (it must be zero); the installer refuses a non-zero record sector it did not write | unvalidated |
| A-4 | The layout of an EFI boot entry and of the boot order variable is as the research agent recalled | From memory, marked unverified in the report | The firmware ignores the entry | Read the UEFI specification's load-option and boot-manager sections before phase 5; the UEFI leg of the proof | unvalidated |
| A-5 | ONIE's kernel accepts single-partition additions while ONIE-BOOT is mounted | The block-layer partition ioctl family exists in 3.2; `gptdisk` already uses its resize member | The new partitions have no device nodes; the installer would write by disk offset instead | A unit test against a loop device, and the ONIE proof | unvalidated |
| A-6 | ONIE's virtual machine image, as published by the SONiC project, runs an unsigned installer in BIOS mode and in UEFI mode with Secure Boot not enforcing | The machine is built with Secure Boot extensions; the SONiC build installs through it | The UEFI leg needs firmware without Secure Boot enforcement, or another ONIE build | The first run of the proof | unvalidated |
| A-7 | The S5232F-ON does not enforce Secure Boot | Unverified | Ze's unsigned loader does not start there | Secure Boot state read on the switch | unvalidated |
| A-8 | The compressed image fits in half the S4048-ON's memory | The image is 2 GiB and mostly empty; no compressed size is recorded; the switch's memory is unverified | The download fails | The build measures the file and refuses above the model's limit | unvalidated |
| A-9 | Ze's kernel can load ONIE's kernel and ramdisk from ONIE-BOOT and start it in the requested mode, with arguments built from ONIE's own boot files | Ze's kernel has file-based kernel loading; ONIE's GRUB starts the same two files with a `boot_reason` argument (`50_onie_grub`, `grub-common.cfg`, reported by the research agent) | The start command cannot work, and the fallback is restoring the saved boot code, which makes a visit one-way under BIOS | Phase 1 reads ONIE's boot files on the virtual machine's ONIE-BOOT and starts ONIE by hand from a Ze guest, under both firmwares; the S4048-ON's 3.2 kernel is the harder case and needs the unit | unvalidated |
| A-10 | ONIE's uninstall removes every partition that is not ONIE's or diagnostics, and Ze's EFI boot entry | Reported through a summarising fetch of `uninstall-arch`, not read verbatim | AC-10 fails: Ze's partitions or its boot entry outlive an uninstall | Read `uninstall-arch` verbatim in phase 1; the uninstall leg of the proof | unvalidated |
| A-11 | Ze's kernel mounts ONIE-BOOT, an ext4 filesystem made by ONIE's old tools, read-only | ext4 is forward compatible | The plugin reads the two files and the environment block through its own minimal reader instead | The state leg of the proof | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A failure mid-install leaves neither system bootable | Any step before the boot switch fails | The boot switch is the last write; before it ONIE boots as before and re-runs the installer, whose earlier steps are repeatable (placement replaces an earlier Ze run) |
| R-2 | The boot switch is half done at a power cut | The switch does not boot | Under BIOS the write is one sector, done after the saved record is verified on disk; under UEFI the entry is created before the order is changed |
| R-3 | The installer runs on the wrong switch | A platform string outside the model's list | Refused before any write, naming the string found and the strings accepted |
| R-4 | The disk holds another operating system | A partition that is neither ONIE's, diagnostics, nor Ze's | Refused before any write, with the instruction to run `onie-uninstall` first |
| R-5 | The file contains ONIE's updater marker by accident | ONIE reports an update image in install mode | The build scans the finished file and refuses |
| R-6 | A credential in the URL reaches the console | The installer prints ONIE's `onie_exec_url` | Every printed URL goes through `redact.URL` |
| R-7 | ONIE's own reboot fails after a successful install because its search path lacks the reboot applet (the SSH case NOSaic recorded) | The switch sits at the ONIE prompt, installed | The installer reboots the switch itself |
| R-8 | The start command puts the switch in ONIE's uninstall mode by a mistyped word | The disk is wiped | `uninstall` is a distinct enumeration value that must be typed whole, and the command's help says it erases Ze |
| R-11 | The jump to ONIE's kernel hangs on a unit | The console goes silent after the command | Nothing on disk was changed, so a power cycle boots Ze again; the saved boot code record remains as the manual route to ONIE |
| R-12 | A switch has two disks that each hold an ONIE-BOOT partition | The installer cannot tell which is ONIE's | Refused before any write, naming both disks, as the ISO refuses an ambiguous target |
| R-9 | The proof passes in the virtual machine and fails on a unit | Kernel 5.4 in the VM, 3.2 on the S4048-ON | The feature page says "proven in ONIE's virtual machine" until a recorded install on each unit is added |
| R-10 | A model's installer silently carries the wrong console or driver set | The switch boots Ze with no console or no management port | `spec-onie-1`'s refusals at build; this spec's proof logs in over the management network |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A switch that neither boots Ze nor ONIE, recoverable only with a vendor recovery stick. No existing device is affected: the ISO and PXE modes keep their code paths |
| How is it reverted? | A commit revert removes the command, the mode and the plugin; a switch already installed keeps running, and returns to ONIE with the saved record by hand |
| Who else touches this path? | `spec-appliance-perm-grows-at-install`, `spec-install-9-cloud-init` (`internal/appliance`), `spec-onie-1` and `spec-onie-2` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-host appliance onie lab` | → | `cmd_onie.go`: validation, compression, append, refusals | `test/appliance/appliance-onie-build.ci` |
| `ze-host appliance onie` for a model with no ONIE platform | → | the build refusal | `test/appliance/appliance-onie-not-a-switch.ci` |
| ONIE executes the file | → | the ONIE source mode of `internal/install/disk` | `./le test qemu installer-onie` |
| `show onie` on the installed switch | → | the runtime plugin's read handler | `./le test qemu installer-onie` (state leg), `test/plugin/onie-show-absent.ci` |
| `request onie start mode rescue` | → | the runtime plugin's action handler: graceful shutdown, then the jump to ONIE's kernel | `./le test qemu installer-onie` (return leg) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `ze-host appliance onie lab` for a built appliance whose model is a switch | One file is written, named after the appliance and its model; it starts with the installer program and carries the compressed image and its description | The appliance image and its checksum file survive untouched, because the command only reads them |
| AC-2 | The same command for an appliance whose model has no ONIE platform string (`generic`, `n100`) | The command refuses, names the model, and names `ze-host appliance iso` as the installer for that box | No file survives, because it could install nowhere |
| AC-3 | The same command when the image's checksum file is missing | The command refuses as `appliance iso` does, with the same fix | No override survives; the ISO's decision applies |
| AC-4 | A file that would exceed half the model's stated memory, or that contains ONIE's updater marker string | The command refuses, naming the size and the limit, or the marker, and what to change | A file within the limit and without the marker is written, which is the useful outcome |
| AC-5 | ONIE's virtual machine in install mode, BIOS firmware, with a partition labelled as vendor diagnostics added to its disk, given the file's URL | ONIE runs the file; the switch reboots into Ze; an SSH login with the embedded credentials answers `show version` | GRUB-BOOT and the diagnostics partition survive byte for byte. On ONIE-BOOT every file keeps its content except the boot environment file, where two variables change; the filesystem's own metadata may differ, because ONIE keeps it mounted read-write during the install. D-4 requires all of this to survive |
| AC-6 | The same under UEFI firmware | Same outcome, and the firmware's boot order has Ze first and ONIE's entry still present | Same |
| AC-7 | On the installed switch, `show onie` | It reports that ONIE is present, the firmware mode, ONIE's recorded mode, whether ONIE's kernel and ramdisk are readable, and whether the manual fallback is intact (the saved boot code record under BIOS, ONIE's boot entry under UEFI) | Read-only: nothing changes |
| AC-8 | `request onie start mode rescue`, then `reboot` typed at ONIE's rescue prompt | ONIE's rescue prompt appears; after the reboot the switch boots Ze and answers `show version` over SSH, under BIOS and under UEFI | Ze's partitions, its boot code and its boot entry survive the visit, because starting ONIE writes nothing to disk and rescue installs nothing |
| AC-9 | `request onie start mode install`, then a second run of the installer from ONIE | The switch reinstalls Ze in place: the earlier run is replaced and the result boots | ONIE and the diagnostics partition survive; the earlier Ze does not, because a reinstall replaces it |
| AC-10 | `request onie start mode uninstall` | ONIE starts in uninstall mode, erases Ze's partitions and its boot entry, takes the boot path back, and returns to its install prompt | ONIE and the diagnostics partition survive; Ze does not, because that is what uninstall means |
| AC-11 | The file run on a machine whose platform string the model does not list | It exits non-zero before any write, naming the string found, the strings accepted, and the appliance model to build for | The disk survives unchanged |
| AC-12 | The file run on a disk that holds a foreign operating system's partitions | It exits non-zero before any write, names the partitions, and gives `onie-uninstall` as the fix | The disk survives unchanged |
| AC-13 | The file run on a disk without room after the existing partitions | It exits non-zero before any write, with the space needed and free | The disk survives unchanged |
| AC-14 | A corrupted payload (one byte changed) | It exits non-zero before any write, with the expected and found digests | The disk survives unchanged |
| AC-15 | A failure injected after the partitions are written and before the boot switch | The program exits non-zero; the switch still boots ONIE; ONIE's re-run of the file completes the install | ONIE's boot path survives until the last step, by design |
| AC-16 | The installed switch takes `ze-host appliance push` of a newer image | It boots the new image, and `show onie` still reports the way back intact | ONIE survives the push |
| AC-17 | A Ze device that was not installed from ONIE | `show onie` reports that no ONIE is present, and `request onie start mode ...` refuses, saying so | Every other command survives; the plugin does nothing on such a device |
| AC-18 | A `ze` built without `ze_appliance` (a standalone Linux install) | The two commands are absent and the daemon starts | Everything else survives, because the plugin is linked only with the on-device runtime features |
| AC-19 | A new installer source mode is added | It registers itself, with no edit to a switch in `run.go`, `validate.go`, `rescue_linux.go` or `initrd_linux.go` | The HTTP and ISO modes survive unchanged, registered the same way |
| AC-21 | The file run on a switch where two disks each hold an ONIE-BOOT partition | It exits non-zero before any write and names both disks | Both disks survive unchanged |
| AC-20 | An install on a real S4048-ON and on a real S5232F-ON | A recorded pass on each unit: install, boot, login, start ONIE's rescue mode, reboot back to Ze | Until a unit's pass is recorded, the feature page names that model as proven in the virtual machine only |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds an installer for a switch and installs from the ONIE prompt | `appliance onie` -> file -> ONIE -> source mode -> placement -> boot switch -> Ze | `appliance-onie-build.ci`, `./le test qemu installer-onie` |
| 2 | Goes to ONIE to run diagnostics and comes back, or to reinstall | `request onie start mode` -> plugin -> ONIE's kernel started in memory -> ONIE -> reboot -> Ze | `./le test qemu installer-onie` (return leg) |
| 3 | Removes Ze from the switch | `request onie start mode uninstall` -> ONIE uninstall | `./le test qemu installer-onie` (uninstall leg) |
| 4 | Is stopped before damaging a switch that is not the model's, or that holds another system | source mode checks -> non-zero exit with the fix | `TestONIERefusesAnUnlistedPlatform`, `TestONIERefusesForeignPartitions` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInstallerFileRoundTrip` | `internal/install/onieimage/onieimage_test.go` | The footer and description written by the builder are read back by the installer | |
| `TestInstallerFileRefusesUnknownFormat` | same | A description with a newer format number is refused by name | |
| `TestInstallerFileRefusesTheUpdaterMarker` | `internal/appliance/cmd_onie_test.go` | AC-4, marker half | |
| `TestONIEBuildRefusesAboveTheMemoryLimit` | same | AC-4, size half | |
| `TestMachineConfParses` | `internal/install/disk/onie_test.go` | Quoted and unquoted values, comments, unknown keys | |
| `TestONIERefusesAnUnlistedPlatform` | same | AC-11 | |
| `TestONIEClassifiesPartitions` | same | ONIE-BOOT by type GUID, GRUB-BOOT, the system partition, a diagnostics label, a Ze run, a foreign partition | |
| `TestONIERefusesForeignPartitions` | same | AC-12 | |
| `TestONIERefusesACorruptedPayload` | same | AC-14 | |
| `TestONIEBootSwitchIsTheLastWrite` | same | AC-15: with a write recorder, no write to the first sector or to an EFI variable precedes the last partition's verification | |
| `TestEnvBlockRoundTrip` | `internal/core/grubenv/grubenv_test.go` | The block keeps its signature, size and padding; set and unset of one variable; escaping | |
| `TestEnvBlockRefusesAForeignFile` | same | A file without the signature is refused, not rewritten | |
| `TestBootEntryEncoding` | `internal/core/efiboot/efiboot_test.go` | A boot entry and a boot order encode and decode against vectors taken from the specification and from the ONIE virtual machine | |
| `TestSavedBootCodeRecord` | `internal/core/bootcoderecord/record_test.go` | The record round-trips; a wrong checksum or version is refused; a non-zero sector not written by Ze is refused | |
| `TestSourceModesRegister` | `internal/install/disk/run_test.go` | AC-19 | |
| `TestONIERefusesTwoONIEDisks` | `internal/install/disk/onie_test.go` | AC-21 | |
| `TestONIEKernelArgumentsFromBootFiles` | `internal/plugins/onie/onie_test.go` | The arguments are read from fixture copies of ONIE's boot files, and the boot reason is appended for each mode | |
| `TestONIEStartRefusesWithoutONIE` | `internal/plugins/onie/onie_test.go` | AC-17 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| installer file size against the model's memory | 1 byte to half the stated memory | exactly half | 0 (no payload) | half plus one byte |
| description format number | the one known number | that number | 0 | the next number |
| environment block size | 1024 bytes | 1024 | 1023 | 1025 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `appliance-onie-build` | `test/appliance/appliance-onie-build.ci` | Building an installer for a switch model writes one file with the expected name | |
| `appliance-onie-not-a-switch` | `test/appliance/appliance-onie-not-a-switch.ci` | A model that is not a switch is refused and pointed at the ISO | |
| `onie-show-absent` | `test/plugin/onie-show-absent.ci` | On a device with no ONIE, the read command says so and the action refuses | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `installer-onie` | `internal/le/test/qemu/` (an installer proof, not a Docker scenario) | ONIE, `kvm_x86_64` machine, the recovery image the SONiC project publishes, pinned by SHA-256 | Under BIOS and under UEFI, on a disk that also carries a partition labelled as vendor diagnostics: ONIE executes the file and Ze boots (AC-5, AC-6); ONIE's and the diagnostics partitions are unchanged; `show onie` (AC-7); ONIE's rescue mode started from Ze and a reboot back to Ze (AC-8); reinstall (AC-9); uninstall by ONIE (AC-10); a failure before the boot switch leaves ONIE in charge (AC-15); a push keeps ONIE reachable (AC-16). Discrimination is recorded by reverting the boot switch and by reverting the environment mark, each of which must turn the proof red | |

## Files to Modify
- `internal/install/disk/run.go`, `validate.go`, `rescue_linux.go`, `initrd_linux.go` - source modes as a registered table, replacing the four switches on the source; the fatal path of a mode that is not PID 1
- `cmd/ze-installer/main.go` - start as an ordinary process when the program carries a payload
- `internal/appliance/main.go` - the `onie` command row
- `internal/appliance/modelreg.go` and the two Dell model files - the switch's memory
- `internal/core/gptdisk/kernel_linux.go` - announcing an added partition to the kernel
- `cmd/ze/setup_features_appliance.go` - the plugin's blank import, beside `coldplug`
- `internal/le/test/qemu/actions.go` - the proof's action row
- `docs/architecture/appliance/README.md`, `iso-installer.md`, `installer-initrd.md`, `on-device-installer.md` - the third mode, and the program outside PID 1
- `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `docs/guide/command-reference.md`, `docs/guide/plugins.md`, `docs/guide/status.md`, `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/architecture/api/commands.md`, `docs/functional-tests.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/testing/ci-workflows.md`, `features/installation.md`

## Files to Create
- `internal/appliance/cmd_onie.go` - `ze appliance onie`
- `internal/install/onieimage/onieimage.go` - the installer file's footer and description, shared by builder and installer
- `internal/install/disk/onie.go`, `onie_linux.go` - the ONIE source mode
- `internal/core/grubenv/grubenv.go` - GRUB's environment block
- `internal/core/efiboot/efiboot.go` - EFI boot entries and boot order through efivarfs
- `internal/core/bootcoderecord/record.go` - the saved boot code record
- `internal/plugins/onie/register.go`, `onie.go`, `yang/`, `register_doctor_codes.go` - the runtime plugin: `show onie`, `request onie start mode`
- `tools/appliance-models/onie-kvm-x86-64.model` - the model of ONIE's virtual machine
- `internal/le/test/qemu/install_onie.go` - the proof
- `docs/architecture/appliance/onie-installer.md` - the design page
- `test/appliance/appliance-onie-build.ci`, `appliance-onie-not-a-switch.ci`, `test/plugin/onie-show-absent.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/onie/yang/`: the two commands and their RPCs. No config leaf |
| YANG validation constraints | Yes | the mode is an enumeration of `install`, `rescue`, `uninstall` |
| YANG custom validators | No | the enumeration is enough |
| CLI commands/flags | Yes | `internal/appliance/cmd_onie.go` (offline, flag-based like `iso`); the plugin's two daemon commands |
| CLI grammar (keyword before value) | Yes | `show onie`; `request onie start mode <value>`: `mode` is the keyword, the value an enumeration; run through `./le cli grammar` |
| Editor autocomplete | Yes | automatic for the enumeration |
| Functional test for new RPC/API | Yes | `test/plugin/onie-show-absent.ci`, and the proof's state and return legs |
| Pipe completeness | Yes | `show onie` returns structured data through the shared pipes |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | Yes | `internal/plugins/onie`, as `runtime` check declarations in the plugin's registration: ONIE's kernel and ramdisk on ONIE-BOOT, its environment file, and the saved boot code record or ONIE's boot entry |
| Doctor code per problem class | Yes | `doctor-onie-return-path` (runtime phase: ONIE's kernel or ramdisk is unreadable, or the fallback is damaged: the saved boot code record is missing or fails its checksum, or ONIE's boot entry is gone), `doctor-onie-boot-environment` (runtime phase: the environment file is unreadable or not a GRUB block); the installer's refusals are operator errors on ONIE's console, before any Ze doctor exists |
| Prometheus counters/metrics | N-A | no continuous state to observe |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/installation.md` |
| 2 | Config syntax changed? | No | no config leaf |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md`, `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | no protocol wire; the installer file is on the new design page |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/qemu-integration.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, if it has a row for installation routes; verified in the documentation phase |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/onie-installer.md` (new) and the pages listed under Files to Modify |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived by `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ze-install.md`, `docs/guide/appliance.md` |

## Design presentation

### What changes

An operator builds one file for a switch and installs Ze from the switch's own
ONIE prompt. ONIE and the vendor diagnostics stay on the disk, Ze is added
after them, and one command on the running switch starts ONIE to run
diagnostics, to reinstall, or to remove Ze; a reboot from ONIE's rescue mode
comes back to Ze. The file is a Go program with the image appended. On the
switch nothing is a shell script or an external tool; on the build host the
command runs the Go toolchain it already uses for the initrd, and nothing
else.

### Before and after

1. Building. Before: `ze-host appliance iso lab` is the only installer, and it needs GRUB's tools and xorriso. After: `ze-host appliance onie lab` writes `lab-dell-s4048-on.bin`, needing only the Go toolchain.
2. Installing. Before: a Dell ONIE switch cannot take Ze. After: at the ONIE prompt, `onie-nos-install http://10.0.0.1/lab-dell-s4048-on.bin`; the console shows the model check, the partitions added, the verification, and the reboot into Ze.
3. A wrong switch. After: the installer stops before writing and prints the platform string it found, the strings its model accepts, and the model to build for.
4. A switch holding another system. After: the installer stops before writing, lists the partitions it does not know, and says to run `onie-uninstall` first.
5. Going to ONIE and back. Before: not applicable. After: `request onie start mode rescue` shuts Ze down and starts ONIE's rescue prompt; typing `reboot` there returns to Ze. `show onie` says beforehand whether ONIE can be started.
6. Removing Ze. After: `request onie start mode uninstall` starts ONIE's uninstall, which erases Ze and leaves the switch at ONIE's install prompt.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| A shell script with an appended archive, as NOSaic and ONIE's reference installer use | Thomas ruled the solution pure Go; it would also be a second installer beside the Go one, which the ISO page's first decision rejects |
| Write the whole image over the disk, as NOSaic does | It erases ONIE and the vendor diagnostics on these switches, where ONIE lives on the same disk; Thomas ruled that ONIE survives |
| Have the file load Ze's installer kernel in memory and jump to it, so the install runs in the ISO's own environment (raised by Thomas: "Can we update the kernel when we do an onie install?") | Attractive, because disk writes would then run under Ze's kernel and the initrd path would be shared whole. Set aside for now: on ONIE's 3.2 kernel the only kernel-switch call needs a hand-built boot hand-off (boot parameters, memory map, an entry stub) that Ze would have to write or import, and under UEFI it must also carry the firmware tables. The program must start under ONIE's kernel in either design. Revisit if a unit shows ONIE's kernel cannot write its disk reliably or cannot announce partitions (A-5) |
| Install GRUB as ONIE's reference installer does, with a menu entry that chainloads ONIE | Needs GRUB's tools or prebuilt GRUB binaries, which is not pure Go, and adds a boot path beside the image's own |
| Return to ONIE by restoring its saved boot code (BIOS) or setting the firmware's next boot (UEFI), then rebooting | The first design, refused by the design judge: under BIOS the first sector then holds ONIE's code, so leaving rescue mode does not come back to Ze, and the visit is one-way until a reinstall. Starting ONIE's kernel from Ze writes nothing and is the same under both firmwares. The saved record is still written at install, as the manual fallback `show onie` reports on |
| A new feature gate, `ze_onie`, for the plugin | The appliance door reserves `ze_appliance` for on-device runtime features, and `coldplug` is already linked that way; a second gate would be a second way to say the same thing |
| Keep the saved ONIE boot code in a file on ONIE-BOOT | ONIE's contract says a NOS does not modify ONIE's partitions beyond the two boot environment variables its own tools write |
| Keep the saved boot code only in Ze's persistent store | The installer would have to mount Ze's ext4 persistent partition under ONIE's old kernel, which may not support the filesystem features Ze formats it with |
| Leave the reboot to ONIE by exiting 0 | ONIE calls the reboot applet by bare name, which fails when the installer was started over SSH (NOSaic's recorded lesson, confirmed in the SSH daemon's default path) |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | The command words | (a) `show onie` and `request onie start mode <install\|rescue\|uninstall>`, recommended; (b) other words | |
| C-2 | Whether `request onie start mode uninstall` exists at all | (a) yes, recommended: it is how an operator hands the switch back, and it must be typed whole; (b) no: an operator who wants that goes to ONIE's install or rescue mode first and runs `onie-uninstall` there | |
| C-3 | Your question, "Can we update the kernel when we do an onie install?": whether the installer should jump to Ze's installer kernel before writing, so the install runs under Ze's kernel | (a) no, install under ONIE's kernel, recommended: the jump from ONIE's 3.2 kernel needs a hand-built hand-off, and the program must start under ONIE's kernel either way; (b) yes, design the jump now | |
| C-4 | What happens if `uname -r` on the S4048-ON shows a kernel older than 3.2, where a Go program cannot start | (a) that model is marked as needing an ONIE update first, recommended; (b) another route is designed for it | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the command row, the source mode table, the plugin skeleton and the proof exist and fail
   - Tests: `appliance-onie-build.ci`, `TestSourceModesRegister`, `onie-show-absent.ci`, `./le test qemu installer-onie`
   - Files: `main.go`, `cmd_onie.go`, `run.go`, `validate.go`, `rescue_linux.go`, `initrd_linux.go`, `internal/plugins/onie/`, `setup_features_appliance.go`, `install_onie.go`
   - Verify: assumptions A-3, A-4, A-6, A-9 and A-10 are settled first: the gap on the virtual machine's disk, the UEFI specification's sections, one manual boot of the pinned ONIE image under both firmwares, ONIE started by hand from a Ze guest, and `uninstall-arch` read verbatim
2. **Phase: The installer file** -- footer, description, build refusals
   - Tests: `TestInstallerFileRoundTrip`, `TestInstallerFileRefusesUnknownFormat`, `TestInstallerFileRefusesTheUpdaterMarker`, `TestONIEBuildRefusesAboveTheMemoryLimit`, both `appliance-onie-*.ci`
   - Files: `onieimage.go`, `cmd_onie.go`, `modelreg.go`, the model files, `onie-installer.md`
   - Verify: the built file's size is recorded in the design page as the first measured figure
3. **Phase: Checks before any write** -- machine file, platform, disk, classification, digests
   - Tests: `TestMachineConfParses`, `TestONIERefusesAnUnlistedPlatform`, `TestONIEClassifiesPartitions`, `TestONIERefusesForeignPartitions`, `TestONIERefusesACorruptedPayload`
   - Files: `onie.go`, `onie_linux.go`, `cmd/ze-installer/main.go`
   - Verify: each refusal ends with its fix
4. **Phase: Writing beside ONIE** -- placement, announcements, streaming, verification, growth
   - Tests: `TestONIEBootSwitchIsTheLastWrite`, the proof up to the reboot
   - Files: `onie_linux.go`, `kernel_linux.go`
   - Verify: assumption A-5 on a loop device
5. **Phase: The boot switch and ONIE's environment** -- BIOS and UEFI
   - Tests: `TestEnvBlockRoundTrip`, `TestEnvBlockRefusesAForeignFile`, `TestBootEntryEncoding`, `TestSavedBootCodeRecord`, AC-5 and AC-6 in the proof
   - Files: `grubenv.go`, `efiboot.go`, `record.go`, `onie_linux.go`
   - Verify: AC-15 by fault injection in the proof
6. **Phase: The runtime plugin** -- `show onie`, `request onie start mode`, the doctor checks
   - Tests: `onie-show-absent.ci`, `TestONIEKernelArgumentsFromBootFiles`, `TestONIEStartRefusesWithoutONIE`, the state, return, reinstall, uninstall and push legs of the proof
   - Files: `internal/plugins/onie/`, `cmd/ze/setup_features_appliance.go`
   - Verify: `./le cli grammar`; `./le plugin boundary check`; `./le repo update` regenerates the composition root
7. **Phase: Documentation and the units** -- the pages, and AC-20
   - Tests: none new
   - Files: the documentation files listed above
   - Verify: Thomas runs the install on each unit with the commands the guide page gives; each pass is recorded on `features/installation.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | The four user stories each have a passing proof |
| Correctness | No write precedes the checks; the boot switch is the last write; each digest is compared before it is trusted |
| Naming | JSON keys kebab-case; the plugin and its commands say `onie` |
| Data flow | The builder and the installer share the file format through one package; the plugin never calls into the installer package |
| Operator errors and doctor codes | Every installer refusal ends with the ONIE or `ze-host` command that fixes it; the two doctor codes carry their fix |
| Rule: pure Go | `rg` for `exec.Command` in `internal/install`, `internal/plugins/onie`, `internal/core/grubenv`, `efiboot` and `bootcoderecord` finds none |
| Rule: interop | The function of ONIE that each proof leg relies on is named, at the version of the pinned image |
| Rule: process boundary | The plugin works when run as an external process |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The builder command | `./le test functional appliance` runs both `appliance-onie-*` tests |
| The proof | `./le test qemu installer-onie` passes under BIOS and UEFI, with its two recorded red runs |
| The runtime commands | `./le cli grammar` accepts them; `onie-show-absent` passes |
| No regression | `./le test qemu installer`, `installer-iso` and `installer-relocated` pass |
| The unit passes | two dated rows on `features/installation.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The description is bounded in size and parsed strictly; `/etc/machine.conf` is read with a size bound; a partition label never reaches a path |
| Integrity | The payload digest is verified before any write and each partition's digest after it; the file is not authenticated beyond that, which is ONIE's own model: the operator names the URL |
| Credentials | URLs are redacted; the image's embedded credentials are those the builder already injects, nothing new |
| Authorization | `request onie start mode` is an action command under the daemon's authorization; `uninstall` erases the system and is listed with the destructive commands |
| Error leakage | Console lines name partitions and sizes, never file contents |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The file is the installer program with the image appended | A shell archive; a separate small loader | ONIE executes the file directly, and one program keeps disk safety and verification shared |
| The boot switch is the last write | Switching first and writing after | ONIE re-runs a failed installer, which only helps while ONIE still boots |
| ONIE's boot code is saved in a checksummed record in the unused sectors before the first partition | A file on ONIE-BOOT; only in Ze's store | No ONIE partition is modified, and no filesystem of Ze's has to be mounted under ONIE's kernel |
| The installer reboots the switch itself | Exit 0 and let ONIE reboot | ONIE's bare-name reboot fails over SSH |
| ONIE is started from Ze by loading its kernel, not by changing the boot path | Restoring ONIE's boot code; the firmware's next-boot variable | Nothing on disk changes, so a visit to rescue mode returns to Ze by a plain reboot, under both firmwares |
| The plugin is linked with the on-device runtime features | A new feature gate | `ze_appliance` already exists for exactly this, and `coldplug` is linked the same way |

## Known Limitations
- Front-panel ports do not forward: the switch ASIC needs a datapath, which is not this set's work. An installed switch is reachable on its management port only, and the feature page says so.
- arm64 and U-Boot ONIE machines are out of scope (owner decision D-1).
- Secure Boot is not supported: Ze's boot loader is unsigned.
- Updating ONIE itself on a switch is the vendor's operation and is never done by Ze.
- The saved boot code record has no command that restores it. It is the manual route to ONIE if ONIE's kernel cannot be started from Ze on some unit; a restore command is designed only if a unit needs it.

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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no protocol)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove` the spec's path only, in the same `./le commit create` script
