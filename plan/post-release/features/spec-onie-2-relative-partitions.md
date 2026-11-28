# Spec: onie-2 -- Ze finds its partitions relative to its boot partition

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Priority | P3 |
| Release reason | Sharing a disk with other partitions is added platform support; every device installed today owns its whole disk and is unaffected. |
| RFC gaps | - |
| Depends | spec-appliance-perm-grows-at-install (in progress: it wires the growth steps this spec makes relative; this spec changes how the persistent partition is found, never how it grows) |
| Phase | DESIGN |
| Handoff | - |
| Design tier | strongest tier |
| Updated | 2026-11-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-onie-0-umbrella.md` (owner decision D-4: ONIE must survive a Ze install).

## Task

A Ze appliance must be able to live on a disk that already holds other
partitions in front of it. On an ONIE switch the first GPT entries and the
first sectors of the disk belong to ONIE and to the vendor's diagnostics, and
Thomas ruled that both survive a Ze install. Their partitions are never rewritten; the 440 bytes of boot code in the first sector are the one shared thing, and who owns them is `spec-onie-3-installer.md`'s subject.

Today that is impossible. The image writes its four partitions (boot, root A,
root B, persistent) at GPT entries 1 to 4 from sector 8192, and the running
system finds them by those numbers: the gokrazy runtime compares the root
identity with the first GPT entry of each disk only and then addresses
partitions by adding 2, 3 and 4 to the device path; Ze's installer and its
GPT code address the persistent partition as entry 4; and under legacy BIOS the
440 bytes of boot code in the first sector read the kernel from absolute
sectors fixed when the image was built.

After this work Ze's four partitions are a run of four consecutive GPT entries
that may start at any entry and any sector. Everything finds them by counting
from the boot partition, which is identified by the unique ID the kernel
command line already carries. A device that owns its whole disk has its boot
partition at entry 1, so for every device in the field nothing changes.

This spec also provides the one function that lays an image's four partitions
after the existing entries of a disk, because the test that proves the rest
needs it and the ONIE installer (`spec-onie-3-installer.md`) calls it on the
switch. It contains no ONIE code.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/appliance.md`
  → Constraint: "An image already in the field is the contract": a device running the previous image must still take a push of the new one and boot it.
  → Constraint: a gokrazy device does not replace its own ze binary; the update path is gokrazy's updater, which Ze patches.
- [ ] `docs/architecture/appliance/on-device-installer.md`
  → Constraint: `internal/core/gptdisk` is the one reader of a GPT; `Grow` extends partition 4 to the disk's last usable sector and refuses when partition 4 is not the last.
  → Constraint: `Grow` writes the backup table first, then the primary, and leaves the MBR byte for byte.
- [ ] `docs/architecture/appliance/ota-push.md`
  → Decision: "Write only the first 440 MBR bytes": the boot code must match the new FAT layout, and the disk signature, partition table and persistent partition stay intact.
  → Constraint: a push streams root to the inactive slot, then `bootonly`, then the MBR boot code.
- [ ] `docs/architecture/appliance/gokrazy-build-pins.md`
  → Constraint: Ze's changes to the pinned gokrazy runtime are diffs under `gokrazy/patches/`, applied to the module cache, guarded by `TestPinnedGokrazyRuntimeMatchesUpstream`.
- [ ] `ai/META-DECISIONS-SITUATIONAL.md`, "Third-party dependencies and licensing"
  → Constraint: a change Ze needs in a third-party module is kept as a diff against the upstream release and is released under that module's licence.

**Key insights:**
- The kernel's own root lookup is already relative: the command line is `root=PARTUUID=<boot>/PARTNROFF=1`. Only userland assumes entry numbers.
- Three places assume numbers: gokrazy's `rootdev`, Ze's `gptdisk` and installer, and the BIOS boot code's absolute sectors.
- A relocated install can only exist after this work, so the system running on one always carries the patched runtime.
- `rootdev` lives in the module `github.com/gokrazy/internal`. Only `github.com/gokrazy/gokrazy` is checked in under `gokrazy/modcache/` and compared by the patch guard today (`gokrazyRuntimeModule`), so patching `rootdev` means carrying and guarding a second module.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/modcache/github.com/gokrazy/internal@v0.0.0-20260625065634-6994f9152c44/rootdev/rootdev.go` - `findGPTPartUUID` keeps only the first GPT entry of each disk ("Check only the first partition id, as that is all that gokrazy currently can ever match"); `Partition(n)` is the device path plus n; `ActiveRootPartition` panics unless the root is 2 or 3; `PartitionCmdline` is already relative.
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/update.go` - `/update/mbr` writes to `rootdev.BlockDevice()`; `/update/root` writes to `rootdev.Partition(inactiveRootPartition)`.
- [ ] `vendor/github.com/gokrazy/internal/mbr/mbr.go` - `Configure(vmlinuzLba, cmdlineLba, partuuid)` returns the boot code with two absolute sector numbers in it.
- [ ] `internal/core/gptdisk/gptdisk.go`, `grow.go` - `PersistentPartition` is the constant 4; `planGrowth` refuses any entry lying after partition 4.
- [ ] `internal/install/disk/run.go` - `partitionPath(disk, 4)` in `runHTTP` and `growISOPerm`.
- [ ] `internal/appliance/push_image.go` - `pushMBRCodeBytes` is 440; the payload is the image's own first 440 bytes.
- [ ] `internal/le/test/qemu/install_iso.go` - `assertInstallGPT` compares the first four entries by index (located by the design judge at line 526).
- [ ] `internal/le/test/qemu/install_ota.go` - `executeApplianceOTA`: the manual proof that builds two images, pushes one onto the other, reboots and checks previous-slot recovery.
- [ ] `internal/le/test/qemu/install_storage_grow.go` - the `appliance-storage` proof, registered with its body not yet written.
- [ ] `internal/le/vendorpatch/gokrazy_runtime_patch_test.go` - `TestPinnedGokrazyRuntimeMatchesUpstream` compares one module, `github.com/gokrazy/gokrazy`.
- [ ] `internal/le/vendorpatch/patches/` - the patches to the root `vendor/` tree, two of them for gokrazy code the build host runs.

**Behavior to preserve:**
- A device that owns its whole disk boots, mounts its persistent partition, takes a push, switches root slot, test-boots and grows exactly as today.
- The image the builder writes is unchanged: entries 1 to 4 from sector 8192.
- `Grow` still refuses a disk whose persistent partition is not the last partition.

**Behavior to change:**
- The runtime and Ze find the four partitions by counting from the boot partition.
- A device whose boot partition is not at sector 8192 adjusts pushed boot code to its own offset.
- `gptdisk` can lay an image's four partitions after existing entries.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Kernel root lookup | the image's command line, `root=PARTUUID=.../PARTNROFF=1` | Finds the root relative to the boot partition | does not apply: already relative; it is the model the rest follows |
| gokrazy `rootdev` | `gokrazy/internal` `rootdev.go` `findGPTPartUUID`, `Partition` | Finds the disk by the first GPT entry and numbers partitions absolutely | extends, by a patch: every entry is searched and numbers count from the match |
| gokrazy runtime patch route | `gokrazy/patches/reboot-ack.patch`; `gokrazy-build-pins.md` "Why a module cache is checked in"; `TestPinnedGokrazyRuntimeMatchesUpstream` | Carries Ze's diff to `github.com/gokrazy/gokrazy`, the one module checked in under the module cache and compared with upstream | extends: the update handler's change is a second diff to that module. The `rootdev` change needs `github.com/gokrazy/internal` checked in and guarded the same way, which the page does not allow today (a `departs` row) |
| Vendor patch route | `internal/le/vendorpatch/patches/` (`gokrazy-packer-return-errors.patch`, `gokrazy-update-url-userinfo.patch`) | Patches the root `vendor/` tree the build host compiles, including `gokrazy/internal/mbr` | does not apply to the device runtime, which is built from the module cache; the re-target function Ze's installer uses is Ze's own code, not a vendored change |
| The push proof | `internal/le/test/qemu/install_ota.go` `executeApplianceOTA` (`./le test qemu appliance-ota`) | Proves a real push, reboot and previous-slot recovery on a whole-disk device | extends: it is the regression evidence for AC-1, and the relocated proof reuses its push and test-boot steps |
| The growth proof | `internal/le/test/qemu/install_storage_grow.go` (`./le test qemu appliance-storage`) | Registered; its body is owed by `spec-appliance-perm-grows-at-install` | does not change it: the relocated proof's growth leg waits on that spec and calls the same steps |
| The GPT reader and grower | `internal/core/gptdisk` `Parse`, `Table`, `Grow`, `PersistentPartition` | Reads a table and grows entry 4 | extends: the persistent partition is the fourth of Ze's run, and a new function places a run after existing entries |
| The installer's partition addressing | `internal/install/disk/run.go` `partitionPath` | Addresses the persistent partition as number 4 | extends: the number comes from where the run was written |
| OTA push boot code | `internal/appliance/push_image.go` `pushMBRCodeBytes`; gokrazy `update.go` `/update/mbr` | Sends and writes 440 bytes computed for sector 8192 | extends: the device adds its own boot-partition offset to the two sector numbers before writing |
| Boot code generator | `gokrazy/internal/mbr` `Configure` | Builds the 440 bytes from two sector numbers | extends: one function re-targets existing boot code by an offset |
| Installed-layout assertion | `internal/le/test/qemu/install_iso.go` `assertInstallGPT` | Compares entries 1 to 4 by index | extends: compares Ze's run wherever it starts |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Before you write code in a subsystem | the door every change reads | "3. Grep for the pattern that already exists before you design one" | agrees | Placement and the run lookup extend `gptdisk`, the existing GPT leaf, and the runtime change travels the existing patch route |
| `docs/architecture/appliance/on-device-installer.md` | Decisions | `// Design:` page of `gptdisk.go`, `grow.go`, `run.go`, `detect.go`; a page this spec modifies | "partition 4 to the disk's last usable sector, lowered to whole 4096-byte" | departs | The page says growth extends partition 4. After this work growth extends the fourth entry of Ze's run, which is partition 4 only on a disk Ze owns whole. See "Documents departed from" |
| `ai/doors/core.md` | Where it sits | the door covering `internal/core/gptdisk` | "A core package is a library with no" | agrees | Placement is a pure library function in the `gptdisk` leaf, with no config lifecycle and no registry side effect |
| `docs/architecture/appliance/iso-installer.md` | Decisions | anchored to `run.go`, `install_boot.go`, `actions.go` | "Disk safety, checksum validation, and success marking are shared concerns." | agrees | The ISO and PXE modes keep writing the whole image from sector 0; only how the persistent partition is addressed afterwards changes, and it resolves to the same partition |
| `docs/architecture/doctor-and-health-checks.md` | Persistent storage smaller than its disk | anchored to `run.go` | | silent | The section describes the storage-not-grown report and names no partition number; this spec adds no check and keeps that report as it is |
| `ai/doors/appliance.md` | Who depends on it from outside | the door covering `run.go`, `detect.go` | "An image already in the field is the contract." | agrees | A whole-disk device has its run at entry 1 and a shift of zero, so the patched lookup and the re-targeting give today's results; AC-1 and the critical review's field-contract row prove it |
| `docs/architecture/testing/qemu-integration.md` | The two entry points | `// Design:` page of `install.go`, `install_boot.go`; a page this spec modifies | "The own-system command carrier can run a whole population in one VM. These" | agrees | The relocated proof is one more installer proof run by the same harness under Ze's own kernel; the page gains its description |
| `docs/architecture/appliance/installer-initrd.md` | What is proven and what is not | anchored to `install.go`, `install_boot.go`, `actions.go`; a page this spec modifies | "A skip is not a pass." | agrees | The section lists each installer proof and its recorded passes. It gains the relocated proof, listed as having no recorded pass until one is run |
| `ai/doors/le-tooling.md` | Where it sits | the door covering the `internal/le/test/qemu` files | "A tool is one package under `internal/le/` that calls" | agrees | The proof is added inside the existing `test qemu` tool; installer proof kinds become a registered table in that package, so the new kind edits no switch (AC-12) |
| `docs/architecture/core-design.md` | Command engine and composition roots | `// Design:` page of `actions.go` | | silent | The page documents the command engine le shares with ze; this spec adds one test action through it and changes nothing the page states |
| `docs/architecture/appliance/gokrazy-build-pins.md` | Why a module cache is checked in | anchored to `actions.go`; a page this spec modifies | "`gokrazy/modcache/.gitignore` ignores everything except the gokrazy init source" | departs | `rootdev` is in `github.com/gokrazy/internal`, which is neither checked in nor compared by the guard. This design checks that module in and widens the guard. See "Documents departed from" |
| `docs/architecture/testing/ci-workflows.md` | Where each suite runs | anchored to `actions.go`; a page this spec modifies | "`./le test qemu appliance-ota` is a manual appliance-delivery proof, outside" | agrees | This spec changes the appliance update path, so that proof is run and is AC-1's evidence, as the page says it must be. The relocated proof is a manual proof of the same kind, and the page gains it |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `actions.go` | | silent | The page covers the Docker interop suites; this spec adds a QEMU installer proof and no interop scenario |
| `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` | Why the codec was written from the RFC tables alone | anchored to `actions.go` | | silent | The page covers VRRP; it is linked only through the shared QEMU action table |
| `docs/architecture/appliance/ota-push.md` | Decisions | a page this spec modifies | "Write only the first 440 MBR bytes" | agrees | The push still sends and the device still writes exactly 440 bytes; the device re-targets the two sector numbers inside them, and the page gains that sentence |
| `docs/functional-tests.md` | Install Tests (`test/install/`) | a page this spec modifies | "Install functional tests cover the offline install surface and the appliance" | agrees | The relocated proof is listed with the other installer proofs of that section |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec adds no service configuration. Its one new failure, a boot ID no disk carries, happens in the runtime before `ze start` exists as a process and already stops the boot today |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec configures no feature, programs no data plane and touches no filter chain; it reaches this page only because it adds a proof to le's QEMU action table |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | The relocated proof is a QEMU installer proof, not a Docker interop suite, and it declares no kernel capability |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- At boot: the kernel command line carries the boot partition's unique ID.
- At push: the updater receives 440 bytes of boot code computed for a whole-disk layout.
- At install: a caller hands `gptdisk` a disk and an image.

### Transformation Path
1. `rootdev` scans every GPT entry of every disk for the boot ID and records the entry index it matched as the base.
2. Partition n of Ze is entry base plus n minus 1; the active root is the second or third of the run.
3. The update handler computes the device's shift (its boot partition's first sector minus 8192) and adds it to the two sector numbers of pushed boot code before writing.
4. `gptdisk` reports Ze's run for a table given the boot ID, and `Grow` extends the fourth of the run.
5. The placement function first looks for an earlier Ze run: four consecutive entries, the last on the disk, whose unique IDs share the fixed prefix the packer gives every Ze image and end in 01 to 04. If it finds one it removes those four entries. It then validates free space, appends four entries carrying the image's own IDs, types and names after the last remaining entry, and returns their sector ranges. It never rewrites or removes any other entry.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel ↔ runtime | `/proc/cmdline`, the on-disk GPT | No |
| Build host ↔ device | the push protocol's `mbr` stream | No |
| Ze ↔ gokrazy runtime | a patch in `gokrazy/patches/` | No |

### Changed-Wire Census

**Search evidence:** the wires are the on-disk GPT, the 440 bytes of MBR boot code, and the push protocol's `mbr` stream. Scope searched: `internal/core/gptdisk`, `internal/install/disk`, `internal/appliance`, `internal/le/test/qemu`, `gokrazy/modcache/github.com/gokrazy` and `vendor/github.com/gokrazy` for `PersistentPartition`, `partitionPath(`, `rootdev.`, `mbr.Configure`, `update/mbr` and `pushMBRCodeBytes`. Positive control: `update/mbr` is found in gokrazy `update.go` (reader) and `mbr` in `cmd_push.go` (writer). No shell or Python reader of any of the three was found in those trees; `rg` over `*.sh` and `*.py` for `PARTNROFF` and `sgdisk` is owed as a command in the implementation's first phase.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| GPT | writer | gokrazy packer `writeGPT` (build host) | `vendor/github.com/gokrazy/tools/packer/packer.go` | preserved: the image is unchanged |
| GPT | writer | `internal/core/gptdisk` `Grow` | `grow.go` | changed: grows the fourth of Ze's run |
| GPT | writer | `internal/core/gptdisk`, the new placement function | new | new |
| GPT | reader | gokrazy `rootdev` `findGPTPartUUID` | `rootdev.go` | changed by patch: searches every entry |
| GPT | reader | `internal/core/gptdisk` `Parse` and its callers (build search for the persistent partition, push image check, lab formatter, QEMU harness) | `on-device-installer.md` names the four | preserved for whole-disk images; each caller is read and listed in phase 1 |
| GPT | reader | the Linux kernel, UEFI firmware | outside the repository | preserved: a valid GPT with more entries |
| MBR boot code | writer | gokrazy packer `writeMBR` via `mbr.Configure` | `vendor/github.com/gokrazy/internal/mbr/mbr.go` | preserved |
| MBR boot code | writer | gokrazy `update.go` `/update/mbr` handler (device) | `update.go` | changed by patch: re-targets by the device's shift, which is zero on a whole-disk device |
| MBR boot code | reader | the BIOS, then the boot code itself | outside the repository | preserved |
| push `mbr` stream | writer | `internal/appliance/cmd_push.go` `StreamTo` | `cmd_push.go` | preserved: the build host still sends whole-disk boot code |
| push `mbr` stream | reader | gokrazy `update.go` `/update/mbr` | `update.go` | changed by patch as above |

### Integration Points
- `rootdev` is used by gokrazy's mount of the persistent partition, its update handlers, its kexec reboot (`reboot_amd64.go`), its status page (`status.go`), `gokrazy.go` and `eeprom.go`; all follow the patched lookup, and phase 3 reads each call.
- `gptdisk.Grow` is called by the installer's growth step that `spec-appliance-perm-grows-at-install` wires.

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
| A-1 | Checking in `github.com/gokrazy/internal` under the module cache and adding it to the patch guard is all the route needs to carry a `rootdev` diff | The design judge read the guard: it names one module, and `git ls-files` holds nothing of `gokrazy/internal` | The route needs more (the `.gitignore` allow-list, the instance pins, Dependabot exclusions) | Phase 1 reads `gokrazy/modcache/.gitignore`, the guard and the pins before the patch is written | unvalidated |
| A-2 | The two sector numbers sit at fixed offsets in the 440 bytes, so adding a shift re-targets the code | `mbr.Configure` takes them as two arguments | Re-targeting needs the generator and the file sectors read from the FAT instead | Read `mbr.go` and its template; a unit test round-trips `Configure` against the re-target function | unvalidated |
| A-3 | UEFI firmware boots the removable-media path from a system partition that is not the first GPT entry | The UEFI boot manager scans system partitions; unverified on the Dell firmware | A UEFI device needs a firmware boot entry, which `spec-onie-3-installer` writes in any case | The UEFI QEMU proof of this spec | unvalidated |
| A-4 | Every caller of `gptdisk` on the build host only ever sees whole-disk images | `on-device-installer.md` lists the callers | A build-host caller needs the relative lookup too | Phase 1 reads each caller | unvalidated |
| A-5 | gokrazy's kexec reboot reloads the kernel through `rootdev` and so follows the patch | Reported by the research agent (`reboot_amd64.go`) | A reboot by kexec loads from the wrong partition | Read the function; the QEMU proof reboots a relocated device | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The patched lookup changes behaviour on a whole-disk device | The existing installer and push proofs fail | The base is entry 1 there, so every number is the same; the existing QEMU proofs are the regression evidence and are rerun |
| R-2 | A push writes boot code for sector 8192 onto a relocated BIOS device | The device does not boot after a push | The handler re-targets; AC-4 pushes to a relocated device and boots it |
| R-3 | Two disks each hold a partition with the boot ID (a cloned disk) | The runtime picks a different disk than the firmware booted | Today's behaviour with entry 1 already has this property; the patch keeps the first match and logs both |
| R-4 | Placement or growth overwrites a foreign partition | A hash of the foreign partitions changes | Placement removes only an earlier Ze run, recognised by its ID prefix and by being the last four entries, and otherwise only appends; every proof hashes the foreign partitions before and after |
| R-7 | A foreign partition is mistaken for an earlier Ze run | Another system's data is removed | The prefix is the one gokrazy's packer gives every image it builds, so the match means "a gokrazy system", which on a Ze switch is an earlier Ze. All four IDs must match in order, share one host part, and be the last entries; anything less is treated as foreign. Another gokrazy system on the same disk would be replaced as an earlier install is, and the installer's console names the run it is about to replace before it does |
| R-5 | A stop during the table rewrite leaves no valid table | The disk has no partitions after a power cut | The backup table is written first and the primary second, as `Grow` already does; the existing stop-at-every-step test is extended to placement |
| R-6 | The re-target logic exists twice, in the patched runtime and in Ze's installer, and drifts | The two give different bytes | One test feeds both the same inputs and compares the outputs byte for byte |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A device does not find its root or persistent partition at boot, or does not boot after a push. This reaches every appliance, including whole-disk ones, because the patched lookup runs on all of them |
| How is it reverted? | A commit revert and a rebuilt image; a device already bricked needs the ISO or PXE installer |
| Who else touches this path? | `spec-appliance-perm-grows-at-install` (growth), `spec-onie-3-installer` (calls placement), anything editing `gokrazy/patches/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A relocated image boots under BIOS | → | patched `rootdev`, re-targeted boot code | `./le test qemu installer-relocated` (BIOS leg) |
| A relocated image boots under UEFI | → | patched `rootdev` | `./le test qemu installer-relocated` (UEFI leg) |
| `ze-host appliance push` to a relocated device | → | the patched `/update/mbr` and `/update/root` handlers | `./le test qemu installer-relocated` (push leg) |
| The placement function on a disk with two foreign partitions | → | `gptdisk` placement | `TestPlaceAfterForeignPartitions` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A whole-disk device built from this work | It boots, mounts the persistent partition and answers `show version` over SSH (`./le test qemu installer`, `installer-iso`); it takes a push, reboots into the new image and recovers its previous slot (`./le test qemu appliance-ota`); and the boot code a push writes to it equals the pushed bytes | All of today's behaviour survives, because the base is entry 1, the shift is zero and every derived number equals today's |
| AC-2 | An image placed after two foreign partitions, booted under legacy BIOS | The device boots, mounts the persistent partition from the fourth of its run, and answers `show version` over SSH | The two foreign partitions survive byte for byte, which is the purpose |
| AC-3 | The same disk booted under UEFI | Same outcome as AC-2 | Same |
| AC-4 | A push of a newer image to the relocated BIOS device, then a reboot | The device boots the pushed root, and the boot code on disk names sectors inside Ze's boot partition | The foreign partitions, the disk signature and the partition table survive, as the push contract already requires. The first sector's 440 bytes of boot code do not survive: a push rewrites them, as it does today |
| AC-5 | A push with test-boot to a relocated device, then a second reboot without confirmation | The device boots the new slot once and returns to the old slot, as a whole-disk device does | The old root slot survives until the switch is confirmed |
| AC-6 | Growth on a relocated disk larger than the image | The persistent partition, the fourth of Ze's run, extends to the last usable sector | Foreign partitions survive; a disk whose persistent partition is not last is still refused, because growth could overlap what follows |
| AC-7 | Placement on a disk without room for the image's four partitions | Placement refuses before writing anything and names the space needed and the space free | The disk survives unchanged |
| AC-8 | Placement on a disk that already holds a Ze run from an earlier install, recognised as Data Flow step 5 states | The old run's four entries are removed and the new run is appended in their place, and the result boots | Foreign partitions survive; the old Ze run and its persistent data do not, because a second install replaces the first |
| AC-9 | Placement on a disk with no GPT, or with a GPT the reader refuses | Placement refuses and says why the table is unusable | The disk survives unchanged |
| AC-10 | The re-target function given whole-disk boot code and a shift of zero | It returns the same 440 bytes | The identity case survives, which is what keeps whole-disk devices unchanged |
| AC-12 | A new installer proof kind is added to the QEMU harness | The kind registers itself in the harness's own table and runs, with no edit to a switch over proof kinds | The four existing proofs survive and run as before, registered the same way |
| AC-11 | The kernel command line names a boot ID no disk carries | The runtime reports the ID it looked for and that no partition of any disk has it | No silent fallback survives: guessing a disk would mount another system's data |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Boots a device whose disk holds other partitions first | firmware -> boot code or loader -> kernel -> patched runtime -> persistent mount -> SSH | `./le test qemu installer-relocated` |
| 2 | Pushes a new image to such a device | `appliance push` -> update handlers -> re-targeted boot code -> reboot | `./le test qemu installer-relocated` (push leg) |
| 3 | Keeps using a whole-disk device, including pushing to it | unchanged | `./le test qemu installer`, `./le test qemu installer-iso`, `./le test qemu appliance-ota` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPlaceAfterForeignPartitions` | `internal/core/gptdisk/place_test.go` | AC-2 layout: four entries appended, IDs kept, foreign entries untouched | |
| `TestPlaceRefusesWithoutRoom` | `internal/core/gptdisk/place_test.go` | AC-7 | |
| `TestPlaceReplacesAnEarlierRun` | `internal/core/gptdisk/place_test.go` | AC-8 | |
| `TestPlaceRefusesAnUnusableTable` | `internal/core/gptdisk/place_test.go` | AC-9 | |
| `TestPlaceLeavesATableAtEveryStop` | `internal/core/gptdisk/place_test.go` | R-5 | |
| `TestRunFoundFromBootID` | `internal/core/gptdisk/gptdisk_test.go` | Ze's run is reported for a boot ID at entry 1 and at entry 3 | |
| `TestGrowExtendsTheFourthOfTheRun` | `internal/core/gptdisk/grow_test.go` | AC-6 | |
| `TestRetargetIsIdentityAtZeroShift` | `internal/core/bootcode/retarget_test.go` | AC-10 | |
| `TestRetargetMatchesConfigure` | `internal/core/bootcode/retarget_test.go` | A-2: re-targeted code equals code generated for the shifted sectors | |
| `TestRetargetAgreesWithTheRuntimePatch` | `internal/le/vendorpatch/gokrazy_retarget_test.go` | R-6 | |
| `TestRootdevFindsABootIDAtAnyEntry` | `rootdev_test.go` of `github.com/gokrazy/internal`, carried in `gokrazy/patches/relative-partitions.patch` | AC-2, AC-11 | |
| `TestPlaceRecognisesOnlyAWholeZeRun` | `internal/core/gptdisk/place_test.go` | R-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| base entry of the run | 1 to (entry count minus 3) | entry count minus 3 | 0 | entry count minus 2 |
| boot-code shift in sectors | 0 to the largest value that keeps both sector numbers in 32 bits | that value | a negative shift | one sector more |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `installer-relocated` | `internal/le/test/qemu/` (a new installer proof) | An image laid after two foreign partitions boots under BIOS and under UEFI, takes a push, test-boots, grows, and leaves the foreign partitions byte for byte | |

### Interop Tests (Scope: protocol)
N-A: no wire protocol with a peer. The proof against ONIE itself belongs to `spec-onie-3-installer.md`.

## Files to Modify
- `internal/core/gptdisk/gptdisk.go`, `grow.go` - Ze's run found from the boot ID; growth of its fourth entry
- `internal/install/disk/run.go`, `detect.go` - the persistent partition addressed from the run
- `internal/le/test/qemu/install.go`, `install_boot.go`, `install_iso.go`, `actions.go` - installer proof kinds become a registered table; the relocated proof; the layout assertion
- `internal/le/vendorpatch/gokrazy_runtime_patch_test.go` - the guard compares `github.com/gokrazy/internal` too
- `gokrazy/modcache/.gitignore` - the second module is checked in
- `docs/architecture/testing/ci-workflows.md`, `docs/architecture/appliance/installer-initrd.md` - the relocated proof among the manual proofs and the recorded passes
- `docs/architecture/appliance/on-device-installer.md`, `ota-push.md`, `gokrazy-build-pins.md` - relative addressing, the re-targeted boot code, the second patch
- `docs/architecture/testing/qemu-integration.md`, `docs/functional-tests.md` - the new proof

## Files to Create
- `internal/core/gptdisk/place.go` - placement of an image's run after existing entries
- `gokrazy/patches/relative-partitions.patch` - the `rootdev` lookup and its test, and the update handler's re-targeting
- `internal/core/bootcode/retarget.go` - re-targeting 440 bytes of boot code by a sector shift, for Ze's installer
- `internal/le/vendorpatch/gokrazy_retarget_test.go` - the comparison of Ze's function with the runtime patch's
- `internal/le/test/qemu/install_relocated.go` - the relocated proof

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config and no RPC |
| YANG validation constraints | N-A | Same |
| YANG custom validators | N-A | Same |
| CLI commands/flags | No | No command is added; `./le test qemu installer-relocated` is a test action |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No config |
| Functional test for new RPC/API | N-A | No RPC; the QEMU proof is the functional evidence |
| Pipe completeness | N-A | No command output |
| Env var registration | No | None added |
| Doctor check for runtime dependencies | No | No new dependency: the same disk, the same kernel interfaces |
| Doctor code per problem class | Yes | The runtime's "boot ID not found" is a boot failure before Ze starts, so no doctor check can report it; placement refusals are returned to the installer, whose codes `spec-onie-3-installer` owns. Growth keeps `doctor-storage-not-grown` |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Nothing a user asks for by name; `features/installation.md` changes with `spec-onie-3-installer` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | The GPT and boot code formats are unchanged; where Ze's entries sit is on the appliance pages |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/qemu-integration.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `on-device-installer.md`, `ota-push.md`, `gokrazy-build-pins.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors`; the pages of row 12 carry them |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes

A Ze appliance no longer needs to be first on its disk. Its four partitions
are found by counting from its boot partition, wherever that sits in the
partition table, so the same image boots, updates and grows whether it owns the
disk or follows another system's partitions. An operator of a device that owns
its disk sees no change at all.

### Before and after

1. A disk with two foreign partitions followed by Ze's four. Before: the device does not start, because the runtime looks for its boot ID in the first table entry only and stops with "PARTUUID=... not found". After: it boots and `show version` answers.
2. `ze-host appliance push lab` to such a device under legacy BIOS. Before: not reachable. After: the push succeeds and the device boots the new image; the same push to a whole-disk device writes the same bytes as today.
3. A disk with no room. After: placement refuses before writing, with the space needed and the space free.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Renumber the table so Ze holds entries 1 to 4 and the earlier partitions move to later entries at the same sectors | It needs no runtime patch, but it rewrites the entries of partitions that belong to another system. ONIE's contract forbids modifying its partitions, and a boot loader that addresses its partition by number stops finding it |
| Find each of the four partitions by its own unique ID | Four identities to keep in step with the image, where one identity and a count already describe a run the packer always writes together; the kernel's own root lookup uses the count |
| Have the build host compute boot code for the device's offset at push time | The build host would have to learn each device's layout first; the device knows its own offset and the push stays one payload for every device |
| Replace the boot code with a boot loader that reads a filesystem | A second boot path beside the image's own, and it needs GRUB or an equivalent, which the pure-Go ruling excludes |

### Documents departed from

1. `docs/architecture/appliance/on-device-installer.md`, "Decisions". Before: "`Grow` extends partition 4 to the disk's last usable sector", changing "entry 4's last sector", with a protective record that "covers only the sectors before partition 1". After: `Grow` extends the persistent partition, the fourth entry of Ze's run, and changes that entry's last sector. On a disk Ze owns whole the run starts at entry 1, so this is partition 4 and entry 4 exactly as the page says today. The protective-record statement holds only for a disk the packer wrote; on a shared disk the first sector's record is the other system's and is left as found. What an operator sees: nothing on a whole-disk device; on a shared disk, the persistent partition grows where today it could not exist.

2. `docs/architecture/appliance/gokrazy-build-pins.md`, "Why a module cache is checked in". Before: "`gokrazy/modcache/.gitignore` ignores everything except the gokrazy init source", and the guard "compares the complete maintained" runtime source of that one module. After: the module cache also checks in `github.com/gokrazy/internal`, the module that holds `rootdev`, and the guard compares both modules with upstream. What an operator sees: nothing. What a maintainer sees: a second upstream module to review at each gokrazy update.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Whether the `rootdev` change is also offered upstream to gokrazy | (a) yes, open a pull request once it is proven here, recommended, since the patch file's own header has a line for it; (b) keep it as Ze's patch only | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the relocated proof exists and fails at boot
   - Tests: `./le test qemu installer-relocated`, `TestPlaceAfterForeignPartitions`
   - Files: `place.go` (stub), `install_relocated.go`, `actions.go`
   - Verify: assumptions A-1, A-2, A-4 and A-5 are settled by reading the producers, and the census's owed search of shell and Python files is run
2. **Phase: Placement and the run** -- `gptdisk` places a run and reports it from a boot ID
   - Tests: the `TestPlace*` tests, `TestRunFoundFromBootID`, `TestGrowExtendsTheFourthOfTheRun`
   - Files: `place.go`, `gptdisk.go`, `grow.go`, `on-device-installer.md`
   - Verify: the relocated proof now reaches the boot failure in the runtime
3. **Phase: The runtime patch** -- `rootdev` searches every entry
   - Tests: `TestRootdevFindsABootIDAtAnyEntry`, the UEFI leg of the relocated proof
   - Files: `relative-partitions.patch`, `gokrazy-build-pins.md`
   - Verify: the existing installer proofs still pass (AC-1)
4. **Phase: Boot code** -- re-targeting at push
   - Tests: the `TestRetarget*` tests, the BIOS and push legs of the relocated proof
   - Files: `relative-partitions.patch`, the re-target function, `ota-push.md`
   - Verify: AC-4 and AC-5 in the QEMU proof
5. **Phase: The installer and the assertions** -- Ze's own callers use the run
   - Tests: `./le test unit installer`, the growth leg of the relocated proof
   - Files: `run.go`, `detect.go`, `install_boot.go`, `docs/functional-tests.md`, `qemu-integration.md`
   - Verify: `rg` for the literal 4 as a partition number in `internal/install/disk` and `internal/le/test/qemu` finds none

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | The three user stories each have a passing proof |
| Correctness | At base 1 and shift 0 every output equals today's, bit for bit |
| Naming | "run" is used for Ze's four consecutive entries in code, tests and pages alike |
| Data flow | No caller computes a partition number itself; all ask `gptdisk` or the patched `rootdev` |
| Operator errors and doctor codes | Placement refusals name the space needed and free; the runtime's failure names the ID it searched for |
| Rule: field contract | A push from the new build host to a device running the previous image, and the reverse, both boot |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The runtime patch | `gokrazy/patches/relative-partitions.patch` exists and the pin test passes |
| Placement | the `TestPlace*` tests pass |
| The relocated proof | `./le test qemu installer-relocated` passes on amd64 under BIOS and UEFI |
| No regression | `./le test qemu installer`, `./le test qemu installer-iso` and `./le test qemu appliance-ota` pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Placement trusts nothing in an existing table beyond what `Parse` already checks (checksums, bounds, overlaps) |
| Data destruction | No write lands in a sector a foreign entry owns, and only a whole Ze run is ever removed; the test hashes foreign partitions |
| Boot ID collision | Two disks carrying the boot ID are logged, never silently resolved to the second |

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
| One identity plus a count | Four identities; renumbering | Matches the kernel's own lookup and what the packer writes |
| The device re-targets pushed boot code | The build host computes it per device | The device knows its offset; the push stays uniform |
| A second gokrazy module is checked in and guarded | Reimplementing `rootdev`'s callers in Ze; forking | The lookup is used by seven places in the runtime; one patched function is smaller than seven changed callers, and a fork is ruled out by the dependency rules |
| Placement lives in `gptdisk`, not in the ONIE installer | Placement in the installer | It is provable here without ONIE, and `gptdisk` is the one GPT reader and writer |

## Known Limitations
- Ze's run must be the last partitions on the disk, so the persistent partition can grow. A disk where something follows Ze is refused.
- The pull request to gokrazy upstream, if Thomas chooses it, is not part of closing this spec.

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
