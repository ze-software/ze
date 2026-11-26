# Spec: config-bound-authority

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Accepted but ineffective configuration: a ring size the driver refuses passes `ze config validate` and fails at apply with no number, and a DPDK queue count is accepted with no bound at all. That is a defect in an implemented capability, so it needs a release owner (`plan/README.md`, "Split fixes from features") |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| ID | Decision |
|----|----------|
| D-1 (boot / reload behavior — split by severity) | Treat buffer/tuning bounds (`system tuning ethtool ring rx/tx` and `vpp dpdk interface rx-queues/tx-queues`) as optimization: refused at validate/commit, warning / non-fatal skip at boot/reload. Treat `interface <name> mtu` as protocol-correctness failure (a wrong MTU breaks OSPF and similar protocols): MUST fail hard rather than warn-and-skip. |
| D-2 (ring-check seam) | Follow R-a: register a `system/tuning` transaction participant owning the ring check and tuning apply (removing the hub's direct `applyHostTuning` bypass, per `ai/doors/system.md` and `ai/rules/no-layering.md`). |
| D-3 (`ze doctor` for DPDK NICs held by VPP on `vfio-pci`) | Stay silent for DPDK NICs already bound to `vfio-pci` and held by VPP (do not emit a permanent false-alarm warning on healthy VPP routers). |

This spec is the release-fix half of the 2026-09-08 design. The other half, a
declaration of who owns every numeric YANG bound and a `./le` gate over it, moved
post-release on 2026-11-14 by Thomas's decision ("Keep half (a) as a release fix,
rebuilt on the host-verdict mechanism. Move half (b) post-release, as opt-in for
host-owned leaves."):
`plan/post-release/engineering/spec-numeric-bound-authority-declaration.md`.
Every acceptance criterion, risk, assumption and decision of the old spec lives
in one of the two files; the map is in "Work Not Done".

## Task

Three config leaves carry a bound that the host owns, not a standard and not Ze:

| Leaf | Static bound today | Who knows the real bound |
|------|--------------------|--------------------------|
| `system tuning ethtool <if> ring rx` / `ring tx` | `uint16`, `range "1..65535"` | the driver, through `ETHTOOL_GRINGPARAM` |
| `vpp dpdk interface <pci> rx-queues` / `tx-queues` | `uint8`, no range | the NIC, through `ETHTOOL_GCHANNELS` while it is on its kernel driver |
| `interface <name> mtu` | `uint16`, `range "68..16000"` | the kernel device, through `IFLA_MIN_MTU` and `IFLA_MAX_MTU` |

Today `ze config validate` accepts a ring above the driver's maximum, and the
operator meets `ETHTOOL_SRINGPARAM failed for eth0` at apply: no number, no
errno, no way to learn what the driver takes. A queue count is never compared
with the NIC. The MTU config leaf stops at 16000 while the `request interface
mtu` argument and its handler accept up to 65535, so two surfaces disagree
about one fact.

VyOS shipped `vyos/vyos-1x#5448` on the other polarity of the same defect: its
CLI refused a VPP ring of 32768 that the ConnectX card reported it could take.
Both polarities are a bound written in a place that does not own it.

The goal: each of the three is judged against the number the host reports at
`ze config validate` and at every commit surface, with the configured value,
the host's number, the subject, the host and measuring process, and the exact
`set` fix in the message. A host Ze cannot read refuses nothing; doctor warns
for unreadable kernel-driver NICs and existing kernel devices, while staying
silent for DPDK NICs already bound to `vfio-pci` and held by VPP (D-3). At boot
and reload (D-1), optimization bounds (`ring rx/tx` and `rx-queues/tx-queues`)
warn and skip the out-of-range direction non-fatally without blocking startup
or reload, whereas `interface <name> mtu` is a protocol-correctness failure
(a wrong MTU breaks OSPF and similar protocols) and MUST fail hard at boot and
reload. Host tuning joins the config transaction protocol through a `host-tuning`
participant owning `system/tuning`, deleting the hub's direct `applyHostTuning`
bypass (D-2).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - kernelcap's verdict states and callers; which process a config-phase check measures; where each owner registers its check and codes
  → Constraint: "`ze config validate` answers about the host running the command. A config written for another machine and validated on a workstation is judged against the workstation. That is why the gate is not inside `LoadConfig`, which also runs under `ze doctor` and under offline validation." A host read inside the `ze:validate` walk would run in `LoadConfig` at start and reload.
  → Constraint: "`ReadConfig` still runs all load validation; a refusal remains an error and prevents every `post-config` check from running." A host refusal in the loader would hide every post-config doctor row, this spec's own included.
  → Constraint: "A capability that could not be determined never refuses." The same rule binds an unreadable driver, NIC or device here.
  → Decision: a config phase runs in whichever process answers doctor (the daemon on the daemon route), and its message says which process it measured. Feature-owned `doctor-*` codes register via `diagnostic.Register` from the owning package's `init()`, not in `internal/core/diagnostic/codes.go`.
- [ ] `docs/architecture/config/yang-config-design.md`, "Where a validator actually runs" and "Layer 3: Plugin config verifiers"
  → Constraint: the `ze:validate` walk has three callers, `LoadConfig` at daemon start and reload, `ze config validate`, and the SSH editor's `commit`, "so they reach one verdict on the same bytes".
  → Constraint: the in-process verifier "does not run at daemon startup". It runs in `ze config validate`, the SSH editor commit (`cli/validator.go` `ConfigValidator.Validate` / `ValidateTransition`), the web editor and upload paths (`ValidateContent`), the REST and gRPC commit (`cmd/ze/hub/api.go` `configValidationHook`), MCP/artifact commits (`artifact_session.go`), and doctor's `config-semantics` check (`config.ValidateSemantics`).
- [ ] `docs/architecture/config/transaction-protocol.md`, "Plugin SDK" and "Phase 1: Verify"
  → Constraint: at reload the live `OnConfigVerify` of every participant runs over its full candidate config (Phase 1). Keeping optimization host reads (`ring`, `queues`) out of live `OnConfigVerify` and `OnConfigure` ensures a lowered driver or NIC limit never blocks boot or reload (D-1), while putting the MTU check in live `OnConfigVerify` and `OnConfigure` makes out-of-bounds MTU fail hard before kernel mutation (D-1).
- [ ] `docs/architecture/host/tuning.md`
  → Constraint: currently states "Tuning errors are non-fatal. They are reported, and they do not block a config commit." and "A ring-size write can fail on a virtual NIC or on a driver with no ring parameter support. Those failures are expected, reported, and not retried." Apply-time non-fatal behavior stays true; commit-time refusal of an over-maximum ring departs from the first sentence, so this page is updated in the same work.
- [ ] `docs/architecture/plugin/plugin-system.md`, "Registration fields"
  → Constraint: currently describes `InProcessConfigVerifier` as "Side-effect-free equivalent of `OnConfigVerify`, used by static, API and CLI validation" and `BootEffect` as running "after host tuning". This spec departs from both under D-1 (optimization host verdicts run only in `InProcessConfigVerifier`) and D-2 (`host-tuning` participant replaces the hub's direct call), so the page is updated in the same work.
- [ ] `docs/architecture/hub-architecture.md`, "Boot effects"
  → Constraint: currently says `RunBootEffects` runs "After host tuning and before the smart manager starts". Deleting `applyHostTuning` from the hub under D-2 departs from that sentence, so the page is updated in the same work.
- [ ] `docs/guide/vpp.md` - the DPDK queue leaves an operator writes
  → Constraint: the page gains the commit-time refusal, the `vfio-pci` silence rule (D-3), and the boot/reload warn-and-skip rule (D-1).

**Key insights:**
- The 2026-09-08 design used `ze:validate`, which runs inside `LoadConfig`. Read against the doors, that seam refuses daemon start and reload for all leaves indiscriminately and hides the post-config doctor rows.
- `diagnostic.HostVerdict` (commit 5f65d1f35c) marks a verifier refusal as a fact about this host, and `ze config validate` reports it under `config-host-backend`.
- The live callbacks and the in-process verifier are separate registration fields, which directly expresses D-1's split by severity: optimization checks (`ring`, `queues`) live in `InProcessConfigVerifier` only (with non-fatal skip + warning at apply), while protocol-correctness checks (`interface mtu`) run in both `InProcessConfigVerifier` and live `OnConfigVerify`/`OnConfigure`.
- `internal/core/diagnostic/doctor_registry.go` imports `internal/component/host` (grandfathered) for platform types, `internal/component/config/system` imports `internal/component/host`, `internal/le/plugin/imports/pluginimports.go` (`pluginDirs`) does not list `internal/component/host/tuning`, and `./le arch tier check` classifies `internal/component/host` as `host-service` (so an `sdk.NewWithConn` engine under `internal/component/host/*` fails `engine-placement` and `non-engine-categories`). Placing the new `host-tuning` participant in edge plugin `internal/plugins/host-tuning` keeps the low-level ethtool/tuning readers and `ApplyTuning` in `internal/component/host` while putting the config-driven engine under `internal/plugins` (already in `pluginDirs`), letting `internal/plugins/host-tuning` import both `internal/component/host` and `internal/core/diagnostic` with zero import cycles and passing `./le repo update` and `./le arch tier check`.
- `internal/component/config/doctor.go` (`semanticsDoctorCheck().Codes`) declares every diagnostic code `config-semantics` can emit, so adding `diagnostic.CodeConfigHostBackend` to `ValidateSemantics` also adds it to `semanticsDoctorCheck().Codes`.
- `VPPManager.runWithOps` (`internal/component/vpp/vpp.go`) writes `startup.conf` BEFORE calling `DPDKBinder.BindAll`, so at boot/reload apply time each NIC that has not yet been bound is still on its kernel driver and readable by `ETHTOOL_GCHANNELS`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/host/tuning_linux.go` - `applyEthtoolRing` calls `setEthtoolRing`, which reads the whole `ringparam` through `getEthtoolRingParam`, overwrites `rxPending` and `txPending` with the configured values and fires the set. `rxMaxPending` and `txMaxPending` are never read. The four casts carry `nolint:gosec // bounded by YANG range`, which names the static range as the authority. `applyEthtoolRing` turns the error into a non-fatal `TuningError`.
- [ ] `internal/component/host/ethtool_linux.go` - `ringparam` declares the four maxima. `ethtoolRingparam` (the inventory read) returns only `rxPending` and `txPending`. `ethtoolIoctl` "Returns false on any errno", so `getEthtoolRingParam` and `setEthtoolRingParam` build errors that carry neither a number nor the errno.
- [ ] `internal/component/host/tuning.go` - `EthtoolConfig` carries the interface and the two configured sizes; `TuningError` carries operation, subject and error.
- [ ] `cmd/ze/hub/main_system.go` - `applyHostTuning` (start, called from `cmd/ze/hub/main.go:1384`) and `applyHostTuningFromMap` (reload, called from `cmd/ze/hub/main_reload.go:1033`) call `host.ApplyTuning` and log each error as a Warn with no doctor code. No registration owns `system/tuning`, so no verifier sees tuning and `internal/component/config/claims/allowlist.json` lists `ExtractTuningFromMap` under `system`.
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - the `ethtool` list keyed by `interface`, "the kernel interface name" (not a logical name), holding `ring rx` and `ring tx`, each `uint16` with `range "1..65535"`, whose descriptions say "a driver refuses a size it cannot serve".
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - `rx-queues` and `tx-queues` are `uint8` with no range. Their `description` (not their `ze:help`) ends "Ze does not compare the count against the queues the NIC holds."
- [ ] `internal/component/vpp/register.go` - `InProcessConfigVerifier: verifyVPPConfig`, while `runVPPEngine` uses `newVPPConfigTx(runtime)` (`internal/component/vpp/startup_lifecycle.go`), whose `parseVPPTransaction` calls `ParseConfigSections` and `settings.Validate()`. Placing the queue check in `verifyVPPConfig` alone (not `VPPSettings.Validate`) keeps it out of live `OnConfigure` and `OnConfigVerify`.
- [ ] `internal/component/vpp/vpp.go` and `internal/component/vpp/startupconf.go` - `runWithOps` calls `m.writeStartupConf(confPath)` BEFORE `ops.bindAll(m.settings.DPDK.Interfaces)`. `GenerateStartupConf` emits `num-rx-queues` and `num-tx-queues` verbatim.
- [ ] `internal/component/vpp/cpuset.go` - `CPUSettings.validate` reads `hostCPUInventory` and calls `validateAgainst`. An unreadable online-CPU file is REFUSED, wrapped in `diagnostic.HostVerdict`. It is reached from `VPPSettings.Validate`, so it runs in both the in-process verifier and live `parseVPPTransaction`. Its test seam is an env-named sysfs root (`cpuRootEnv`).
- [ ] `internal/component/vpp/register_linux.go` and `internal/component/vpp/register_doctor_codes.go` - registers the hugepage, CPU isolation and DPDK bind doctor checks through `diagnostic.RegisterDoctorCheck` and registers all VPP `doctor-*` codes via `diagnostic.Register` from `init()`.
- [ ] `internal/component/vpp/doctor_dpdk_linux.go` - `vppDPDKDoctorCheck` runs in `DoctorPhasePostConfig` (Order 2220) and checks VFIO modules, PCI sysfs presence, and original driver state.
- [ ] `internal/component/iface/register.go` and `internal/component/iface/config_apply.go` - `InProcessConfigVerifier: verifyIfaceConfig` calls `parseAndVerifyIfaceSections`. Live `OnConfigVerify` calls `parseAndVerifyIfaceSections`. Live `OnConfigure` calls `parseResolvedIfaceSections` and then `applyAndPublish`, which calls `b.SetMTU` and fails hard via `joinApplyErrors` if the kernel refuses the MTU.
- [ ] `internal/component/iface/doctor_codes.go` - registers all `iface` `doctor-*` codes via `diagnostic.Register` in `registerDoctorCodes()`.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `leaf mtu`, `uint16`, `range "68..16000"`.
- [ ] `internal/component/iface/yang/ze-iface-cmd.yang` - `request interface mtu` `leaf bytes`, `uint16`, `range "68..65535"`.
- [ ] `internal/component/iface/cmd/manage.go` and `internal/component/iface/cli/link.go` - `handleInterfaceMTU` and `cmdMTU` check `MTUMin` 68 and `MTUMax` 65535, then call `iface.SetMTU`.
- [ ] `internal/core/diagnostic/host_verdict.go` - `HostVerdict` wraps an error; `IsHostVerdict` answers through `errors.As`.
- [ ] `internal/core/diagnostic/codes.go` - declares only `config-*` codes, core TLS helper codes, and cross-cutting `doctor-*` codes; feature-owned `doctor-*` codes register via `diagnostic.Register` in their owning package's `init()`. `CodeConfigHostBackend` (`config-host-backend`) currently describes only a missing OS default backend or absent CPU inventory.
- [ ] `internal/component/config/cli/cmd_validate.go` - `runValidation` reports each `config.VerifyPluginConfig` error under `config-plugin-verify`, or `config-host-backend` when `IsHostVerdict`. `ValidateContent` calls `runValidation`, so every caller of `ValidateContent` (web editor, HTTP upload, SSH upload, REST/gRPC `configValidationHook`, artifact session pre-commit) runs `VerifyPluginConfigMapTransition`.
- [ ] `internal/component/cli/validator.go`, `cmd/ze/hub/api.go`, `cmd/ze/hub/config_upload.go`, `internal/component/config/cli/artifact_session.go`, `internal/component/config/cli/config_detached.go` - every commit surface (SSH editor, web editor, REST, gRPC, SSH/HTTP upload, MCP/artifact session commit) reaches `VerifyPluginConfigMapTransition`.
- [ ] `internal/component/config/validate_semantic.go` and `internal/component/config/doctor.go` - `ValidateSemantics`, which doctor's `config-semantics` check runs, reports every `VerifyPluginConfig` error under `CodeConfigPluginVerify`, a host verdict included, and `semanticsDoctorCheck().Codes` lists `CodeConfigMCPInvalid`, `CodeConfigGNMIInvalid`, `CodeConfigPluginVerify`, and `CodeConfigHubInvalid` (omitting `CodeConfigHostBackend`). So doctor and validate give one refusal two codes today.
- [ ] `internal/component/config/plugin_verify.go` - `VerifyPluginConfigMapTransition` calls every registration's `InProcessConfigVerifier` whose `ConfigSectionRoots` are present; `ExtractConfigSubtree` handles nested roots such as `system/tuning` (as `firewall` already uses `system/conntrack`).
- [ ] `internal/component/plugin/registry/registry.go` - the `InProcessConfigVerifier` comment calls it the "side-effect-free equivalent of the plugin's OnConfigVerify callback"; `Register` refuses a nil `RunEngine`.
- [ ] `internal/component/config/cli/register.go` - `show config validation` is a `MustRegisterLocalData` command, so SSH, MCP, REST and web reach it through the daemon and it reads the daemon's host; the local offline `ze config validate` reads the caller's machine.
- [ ] `internal/component/kernelcap/kernelcap.go` - `Refuse` names every failing subsystem; the verdict is about whether a feature exists, never a number.

**Behavior to preserve:**
- Ring apply stays non-fatal at apply time (`docs/architecture/host/tuning.md`); an unreadable driver, missing interface, or unsupported ioctl never blocks boot or reload.
- A host fact Ze cannot read never refuses validate or commit.
- `test/plugin/config-validate-agrees-with-boot.ci` keeps passing: validate refuses only a config defect or a marked host verdict.
- The VPP CPU check keeps its behavior; it is the precedent, and its differences are recorded under Design Insights.
- For DPDK NICs already bound to `vfio-pci` and held by VPP, `ze doctor` stays silent (D-3).

**Behavior to change:**
- A ring above the driver's reported maximum is refused at validate and at every commit surface via the new `host-tuning` (`system/tuning`) participant, naming the configured value, the maximum, the interface, the host, the measuring process, and the `set` fix.
- `applyHostTuning` and `applyHostTuningFromMap` are deleted from the hub (`cmd/ze/hub/main_system.go`, `main.go`, `main_reload.go`) and replaced by the `host-tuning` transaction participant (D-2).
- At apply (boot/reload), `setEthtoolRing` skips any direction above the maximum it has just read, logs a non-fatal warning under `doctor-host-tuning-not-applied` with both numbers and the interface, writes the other direction if it fits, and includes the errno in every ioctl error.
- A DPDK queue count above the NIC's maximum is refused at validate and commit while the device is on its kernel driver; at boot/reload apply (`VPPManager.writeStartupConf`, before `bindAll`), an over-maximum direction is skipped (omitted from `startup.conf` so VPP uses its default for that direction) with a non-fatal `Warn` under `doctor-vpp-dpdk-queues-skipped` (D-1).
- An `interface mtu` outside the device's `IFLA_MIN_MTU` to `IFLA_MAX_MTU` is refused at validate, commit, AND fails hard at boot (`OnConfigure`) and reload (`OnConfigVerify` / `OnConfigApply`) (D-1); the static ceiling widens from 16000 to 65535 (matching `request interface mtu` and `ze interface mtu`), and both operational MTU commands refuse outside the device bounds via the same reader.
- Doctor reports every host verdict under `config-host-backend`, matching validate, and registers feature-owned unreadable/skip codes via `diagnostic.Register` in their owning packages.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `ze:validate` and `ValidatorRegistry` | `internal/component/config/yang/validator_registry.go` `CustomValidator`; `validators_register.go` `RegisterValidators` (29 registrations); `ze-extensions.yang` (35 extensions) | A leaf names a Go validator; the walk runs it in `LoadConfig` (start and reload), `ze config validate` and the editor commit | Does not apply: the walk runs inside `LoadConfig`, so a host read there would refuse start and reload indiscriminately (contradicting D-1 for rings and DPDK queues) and hide every post-config doctor row. This was the superseded 2026-09-08 seam |
| In-process config verifier | `registry.Registration.InProcessConfigVerifier`; `config.VerifyPluginConfigMapTransition`; nested root `ConfigRoots` (as `firewall` owns `system/conntrack`) | Side-effect-free verify of a registration's roots, run by validate, every commit surface (SSH, web, REST, gRPC, upload, MCP/artifact) and doctor's `config-semantics`, never at start | Extends: `host-tuning` (`system/tuning`) owns the ring check, VPP owns the queue check (both verifier-only per D-1), and `iface` owns the MTU check (verifier plus live `OnConfigVerify`/`OnConfigure` per D-1); the field comment, `plugin-system.md` and Layer 3 of `yang-config-design.md` document the host-verdict split |
| Hub direct tuning apply | `cmd/ze/hub/main_system.go` `applyHostTuning`, `applyHostTuningFromMap`; call sites `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go` | Applies `system/tuning` directly from the hub at boot and after reload promotion, bypassing the config transaction protocol and leaving no owner for an in-process verifier | Replaces (D-2): deleted outright (`ai/rules/no-layering.md`) and replaced by the `host-tuning` transaction participant (`internal/plugins/host-tuning/register.go`, edge plugin under `internal/plugins/` already in `pluginDirs`) owning `ConfigRoots: []string{"system/tuning"}` and `sdk.Registration{WantsConfig: []string{"system/tuning"}}` |
| Host verdict | `internal/core/diagnostic/host_verdict.go` `HostVerdict`, `IsHostVerdict`; `cmd_validate.go` `runValidation`; `codes.go` `CodeConfigHostBackend`; `config/doctor.go` `semanticsDoctorCheck` | Marks a refusal as a fact about this host; validate reports it under `config-host-backend` | Extends: every bound refusal here is marked `HostVerdict`; `ValidateSemantics` learns the same code mapping and `semanticsDoctorCheck().Codes` declares `CodeConfigHostBackend` so doctor and validate agree; `CodeConfigHostBackend`'s description covers host-reported numeric bounds |
| `kernelcap` | `internal/component/kernelcap/kernelcap.go` `Capability`, `Evaluate`, `Refuse`; `Refuse`'s three callers `cmd_validate.go` `runValidation`, `cmd/ze/hub/main.go` `runYANGConfig`, `cmd/ze/hub/main_reload.go` `runReloadContext` | Present, absent, unknown or denied for a kernel feature, refusing at validate, start and reload | Does not apply: it answers whether a kernel feature exists and carries no subject or numeric bound, and refuses start unconditionally. Adding numeric fields to `Capability` would overload one struct with two disjoint meanings (`plan/journal/field-carries-two-meanings.md`) |
| Doctor check and code registries | `internal/core/diagnostic` `RegisterDoctorCheck`, `diagnostic.Register`; `doctor-and-health-checks.md`, "Where each owner registers its check" | Each owner registers its post-config checks and feature-owned `doctor-*` codes from `init()` (`vpp/register_doctor_codes.go`, `iface/doctor_codes.go`, `host/trend/register.go`) | Extends: VPP registers `vpp-dpdk-queues` (`doctor-vpp-dpdk-queues-unreadable`, silent on `vfio-pci` per D-3) and `doctor-vpp-dpdk-queues-skipped`; `iface` registers `iface-mtu-bounds` (`doctor-iface-mtu-bounds-unreadable`); `internal/plugins/host-tuning` registers `doctor-host-tuning-not-applied`; over-maximum facts report once through `config-semantics` |
| VPP CPU placement check | `internal/component/vpp/cpuset.go` `CPUSettings.validate`, `validateAgainst`, reached from `VPPSettings.Validate` in `config.go`, which `verifyVPPConfig` and `parseVPPTransaction` call; `doctor_cpu_linux.go` `vppCPUIsolationDoctorCheck` | Refuses a configured core the host does not run, and refuses an unreadable inventory as a host verdict, at validate, commit, reload and start | Extends its testable shape (passing the host read as an argument so tests state the host); differs on two points recorded in Design Insights: an unreadable queue/ring/MTU read refuses nothing, and optimization checks (`ring`, `queues`) stay out of live `OnConfigure`/`OnConfigVerify` |
| Planned host-tuning doctor check | `plan/release/validation/spec-doctor-coverage-15-system.md` AC-14, `host-tuning`, code `doctor-host-tuning-not-applied` | Not built. Will compare configured tuning with held values, the ethtool read included, warn per mismatch, and warn for a setting the host has no file for | Does not apply as a second check: that spec owns the `host-tuning` doctor check and its unreadable-ring warning. This spec registers `doctor-host-tuning-not-applied` in `internal/plugins/host-tuning/doctor_codes.go` for the apply-time warning and owns the over-maximum ring fact reported once through `config-semantics`; the `host-tuning` check stays silent for a ring above the reported maximum (reciprocal line in AC-14 there) |
| Ethtool ioctl path | `internal/component/host/ethtool_linux.go` `ethtoolIoctl`, `getEthtoolRingParam` | The one `SIOCETHTOOL` wrapper; bounds the interface name | Extends: the ring read keeps `rxMaxPending` and `txMaxPending`, an `ETHTOOL_GCHANNELS` channel read is added, and the wrapper returns the errno |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads; covers `internal/component/host/*` | "A config change is a transaction of stream events (verify, apply, rollback) that each plugin answers for its own roots" | agrees | D-2 moves `system/tuning` from hub-direct calls into the `host-tuning` transaction participant; all new doctor checks and codes register from their owners' `init()`; all refusals state problem, why, and exact `set` fix |
| `docs/architecture/host/inventory.md` | Traps | `// Design:` and anchor of `ethtool_linux.go`; modified | "The ioctl targets the running kernel's netdev namespace, not the testdata tree." | agrees | Extends `ethtool_linux.go` to keep ring maxima, add `ETHTOOL_GCHANNELS`, and propagate errno; unit tests pass stated host readers and real ioctls are tested in QEMU rather than against fixture paths |
| `docs/architecture/host/tuning.md` | Consequences and traps | `// Design:` of `tuning.go`, `tuning_linux.go`; anchored to `ethtool_linux.go`; modified | "Tuning errors are non-fatal. They are reported, and they do not block a config commit." | departs | Under D-1 and D-2, an ethtool ring size above the driver's reported maximum now blocks `ze config validate` and every config commit under `config-host-backend`, while apply-time tuning errors at boot/reload remain non-fatal and warn-and-skip under `doctor-host-tuning-not-applied`. Page updated in this work |
| `docs/architecture/hub-architecture.md` | Boot effects | `// Design:` and anchor of `cmd/ze/hub/main.go`, `main_reload.go`, `main_system.go`; modified | "After host tuning and before the smart manager starts, the hub runs the boot effect every registered plugin declared (`Registration.BootEffect`), once, in plugin name order" | departs | Under D-2, hub-direct `applyHostTuning` and `applyHostTuningFromMap` are deleted and host tuning moves into the `host-tuning` (`system/tuning`) transaction participant (`OnConfigure`/`OnConfigApply`), so `RunBootEffects` no longer follows a hub-level tuning call. Page updated in this work |
| `docs/architecture/core-design.md` | 14a. Commit-Time Backend Capability Gate | anchored to `cmd/ze/hub/*`, `iface/register.go`, `ze-system-conf.yang` | "is marked `diagnostic.HostVerdict`, so `ze config validate` reports it under the host-verdict code `config-host-backend` rather than as a config defect." | agrees | All three numeric host-bound refusals wrap `diagnostic.HostVerdict` and report under `config-host-backend` in both `ze config validate` and `ze doctor` (`ValidateSemantics`) |
| `docs/architecture/diagnostics/path-mtu.md` | The module is removable | anchored to `cmd/ze/hub/main_system.go` | - | silent | Covers read-only `show mtu` path-MTU probing; silent on interface/ring/queue config bound validation and `system/tuning` transaction ownership |
| `docs/architecture/host/observability.md` | The trend monitor | anchored to `cmd/ze/hub/main_system.go` | "The monitor is the package `internal/component/host/trend`, under the host component rather than in it, because `internal/core/diagnostic` imports the host component for the platform type, and the monitor registers doctor checks." | agrees | Follows the same separation for the same `diagnostic -> host` cycle reason: low-level ethtool/tuning readers and `ApplyTuning` stay in `internal/component/host`, while the config-driven `host-tuning` participant and its doctor code registration live outside `internal/component/host` in edge plugin `internal/plugins/host-tuning` (unlike `trend`, which has no config engine, `host-tuning` uses `sdk.NewWithConn` and belongs under `internal/plugins/` to satisfy `pluginDirs` and `./le arch tier check`) |
| `docs/architecture/resolve.md` | Structure | anchored to `cmd/ze/hub/main_system.go`, `ze-system-conf.yang` | - | silent | Covers DNS, Cymru, PeeringDB, and IRR resolution; silent on ethtool tuning, DPDK queues, and interface MTU bounds |
| `ai/doors/hub-engine.md` | Boundaries | door covering `cmd/ze/hub/main.go`, `main_reload.go`, `main_system.go` | "SIGHUP and a managed push both reach `doReload` (`cmd/ze/hub/main_reload.go`)." | agrees | Removing `applyHostTuning` and `applyHostTuningFromMap` eliminates the hub bypass so `system/tuning` participates in the single transaction reload path; no gated import enters `cmd/ze/hub`; `plugin/all` snapshots update for `host-tuning` |
| `docs/architecture/api/architecture.md` | Implementation Status | `// Design:` of `plugin/registry/registry.go`; anchored to `main.go`, `main_reload.go`, `registry.go` | - | silent | Covers REST/gRPC/gNMI transports, bus participant filters, and wire events; silent on host-reported numeric bounds and `InProcessConfigVerifier` vs `OnConfigVerify` optimization splits |
| `docs/architecture/api/commands.md` | Overview | anchored to `main.go`, `main_reload.go`, `registry.go` | - | silent | Covers command dispatch keys, answer shapes, and pipe operators; no command path, verb, or answer shape changes |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `main.go`, `registry.go` | - | silent | Covers plugin wire framing, five-stage handshake, and runtime RPCs; `host-tuning` uses existing SDK config callbacks with no wire change |
| `docs/architecture/appliance/remote-operations.md` | Decisions | anchored to `cmd/ze/hub/main.go` | - | silent | Covers authenticated SSH config upload transport and receipts; upload already validates via `ValidateContent` |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `cmd/ze/hub/main.go`, `main_reload.go` | - | silent | Covers Unix signal handling (SIGHUP, SIGTERM) and graceful shutdown; signal dispatch and exit semantics are unchanged |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `cmd/ze/hub/main.go` | - | silent | Covers CLI tab completion; dynamic completion of host-reported maxima is explicitly deferred to half (b) (`spec-numeric-bound-authority-declaration.md`) |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | `// Design:` of `validate_semantic.go`; anchored to `main.go`, `registry.go` | - | silent | Covers hierarchical and set config grammar; no config syntax changes (only accepted MTU range widens to `68..65535` and `ValidateSemantics` aligns host-verdict code mapping) |
| `docs/architecture/config/yang-config-design.md` | Layer 3: Plugin config verifiers | anchored to `main.go`, `ze-iface-conf.yang`; modified | "A plugin declares `InProcessConfigVerifier` on its registration and receives its own config subtree, before and after, at `VerifyPluginConfig` and `VerifyPluginConfigContentTransition`." | agrees | All three host-bound checks run in their owners' `InProcessConfigVerifier` rather than `ze:validate` inside `LoadConfig`; `InProcessConfigVerifier` still does not run at daemon startup (the `iface` MTU boot/reload check runs in live `OnConfigure`/`OnConfigVerify`). Layer 3 updated to document the host-verdict split |
| `docs/architecture/diagnostics/crash-capture.md` | The kernel half | anchored to `cmd/ze/hub/main.go`; modified | "the daemon start path runs every registered boot effect once through `registry.RunBootEffects`, after host tuning and before the smart manager starts." | departs | Under D-2, direct hub `applyHostTuning` is deleted and replaced by the `host-tuning` (`system/tuning`) transaction participant, so `RunBootEffects` no longer follows a hub-level tuning call. Page updated in this work |
| `docs/architecture/doctor-and-health-checks.md` | Where each owner registers its check | `// Design:` of `vpp/register_doctor_codes.go`, `iface/doctor_codes.go`; anchored to `main.go`, `main_reload.go`; modified | "`diagnostic.RegisterDoctorCheck()` from the owning backend or component, with build-tagged files where needed" | agrees | VPP registers `vpp-dpdk-queues` (silent on `vfio-pci` per D-3), `iface` registers `iface-mtu-bounds`, all feature-owned `doctor-*` codes register via `diagnostic.Register` from their owning packages (`internal/plugins/host-tuning`, `vpp`, `iface`), `config-semantics` (`internal/component/config/doctor.go`) declares `CodeConfigHostBackend` in `semanticsDoctorCheck().Codes`, unreadable hosts never refuse, and over-max bounds report once via `config-semantics` under `config-host-backend` |
| `docs/architecture/fleet-config.md` | Vision | anchored to `cmd/ze/hub/main.go`, `main_reload.go` | - | silent | Covers managed fleet client/hub config sync and inventory reporting; untouched by local host-bound validation |
| `docs/architecture/pki/pki-store.md` | Decisions | anchored to `cmd/ze/hub/main.go` | - | silent | Covers PKI certificate storage and trust chains; untouched |
| `docs/architecture/pki/tls-listeners.md` | Decision: the leaf comes first, then the intermediates | anchored to `cmd/ze/hub/main.go`, `main_reload.go` | - | silent | Covers TLS listener configuration for web, looking glass, and APIs; untouched |
| `docs/architecture/storage-backends.md` | Opening and ownership | anchored to `cmd/ze/hub/main.go` | - | silent | Covers ZeFS persistent storage backend and locking; untouched |
| `docs/architecture/system-architecture.md` | System RIB process boundary | anchored to `cmd/ze/hub/main.go` | - | silent | Covers high-level system architecture and component overview; silent on host numeric bound validation and `applyHostTuning` placement |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `cmd/ze/hub/main.go` | - | silent | Covers `.ci` functional test file format; new `.ci` tests follow the existing format without changing it |
| `docs/architecture/web-interface.md` | Source Files | anchored to `cmd/ze/hub/main.go`, `main_reload.go` | - | silent | Covers web UI routes, HTMX, and workbench pages; the web editor already calls `ValidateContent` |
| `docs/architecture/aaa-tacacs.md` | A registry, not a hardcoded chain | anchored to `cmd/ze/hub/main_reload.go` | - | silent | Covers AAA authentication and TACACS+; untouched |
| `docs/architecture/config/transaction-protocol.md` | Phase 1: Verify | anchored to `main_reload.go`, `iface/register.go`, `registry.go` | "Every plugin that owns affected config roots validates the candidate config against its constraints. Verification is non-destructive: no state changes, no side effects." | agrees | `host-tuning` joins the transaction protocol on `system/tuning` with a side-effect-free `OnConfigVerify`; keeping optimization host reads out of live `OnConfigVerify` lets boot/reload warn-and-skip while `iface` `OnConfigVerify` refuses out-of-bounds MTU before apply |
| `docs/architecture/plugin/component-boundaries.md` | The four components | anchored to `cmd/ze/hub/main_reload.go` | - | silent | Covers public `pkg/ze` engine interfaces (`Engine`, `ConfigProvider`, `PluginManager`, `Subsystem`, `EventBus`); none change |
| `ai/doors/config-yang.md` | Boundaries | door covering `config/claims/allowlist.json`, `config/validate_semantic.go`, `config/doctor.go` | "A change is a transaction, not a reload call." | agrees | `host-tuning` (`internal/plugins/host-tuning`) joins the transaction protocol on `ConfigRoots: []string{"system/tuning"}` and `WantsConfig: []string{"system/tuning"}`, `allowlist.json` removes the stale `ExtractTuningFromMap` mention, `ValidateSemantics` and `semanticsDoctorCheck().Codes` align host verdicts on `config-host-backend`, and `host-tuning` parses `ToPluginMap` string values cleanly |
| `docs/architecture/command-ownership.md` | The Folder Test | anchored to `internal/component/plugin/all/all.go` | - | silent | Covers command YANG and handler ownership rules; no command is added or moved, and `plugin/all/all.go` is regenerated by `./le repo update` |
| `ai/doors/plugin-infra.md` | Boundaries | door covering `plugin/all/all.go`, `plugin/registry/registry.go` | "The engine never enumerates plugins." | agrees | `internal/plugins/host-tuning` sits under `internal/plugins` (already in `pluginDirs`), self-registers via `registry.Register` in `init()`, and enters `plugin/all/all.go` via `./le repo update`; no wire type, SDK stage, or central list is added |
| `ai/doors/system-plugins.md` | Where it sits | door covering `internal/plugins/host-tuning/*` | "A package lands in `internal/plugins/` when it is a config-driven engine that nothing else depends on." | agrees | `internal/plugins/host-tuning` is a config-driven edge engine owning `system/tuning` that no other package imports, delegates low-level ethtool/tuning I/O to `internal/component/host`, enters `plugin/all/all.go` via `pluginDirs` and `./le repo update`, and passes `./le arch tier check` (`engine-placement` and `non-engine-categories`) |
| `docs/research/vpp-deployment-reference.md` | Production Architecture (IPng AS8298) | `// Design:` of `vpp/register.go`, `vpp/vpp.go`, `vpp/startupconf.go` | - | silent | Research reference documenting VPP `startup.conf` directives (`num-rx-queues`, `num-tx-queues`) and supervision; does not constrain Ze's config validation or boot-time skip of over-maximum queue counts |
| `docs/architecture/firewall/fw-6-firewall-vpp.md` | What VPP programs, everything else rejected at commit | anchored to `internal/component/vpp/register.go` | - | silent | Covers VPP firewall ACL/policer lowering; untouched |
| `ai/doors/forwarding.md` | Boundaries | door covering `internal/component/vpp/*`, `internal/component/iface/*` | "Interface names go through the shared resolver" | agrees | The MTU bounds reader resolves logical names via the shared resolver (`./le arch iface-resolution`), VPP queue checks stay inside `ze_vpp`, and `interface mtu` fails hard at boot/reload rather than approximating |
| `docs/architecture/vpp-host-tuning.md` | An empty isolated set is not an unreadable one | `// Design:` and anchor of `vpp/register_linux.go` | "An unreadable `isolated` file is not an error, because isolation governs placement quality rather than validity; it leaves `IsolationKnown` false, and the doctor check says so in its own words." | agrees | Mirrors this exact unreadable-vs-zero distinction for ring, DPDK queue, and MTU probes: unreadable never collapses to zero or refuses a commit, and the owning Linux-tagged post-config doctor checks report unreadable states |
| `docs/architecture/fib/fib-depth-4-srv6.md` | Where the SID comes from | anchored to `internal/component/vpp/vpp.go` | - | silent | Covers SRv6 FIB programming; untouched |
| `docs/features/interfaces.md` | Interface Discovery | `// Design:` of `iface/register.go`, `iface/cmd/manage.go`, `iface/cli/link.go`; modified | "Operator-facing interface operations resolve the logical name to its kernel device through the shared resolver before touching the kernel" | agrees | Both the MTU config/live checks and `request interface mtu` / `ze interface mtu` resolve logical names through the shared resolver before reading `IFLA_MIN_MTU`/`IFLA_MAX_MTU`; static range widens to `68..65535` matching `ze-iface-cmd.yang`. Page updated in this work |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `internal/component/iface/register.go` | - | silent | Covers operation-graph address-move ordering across plugins; host bound checks do not alter operation ordering |
| `docs/architecture/iface/logical-name-resolution.md` | Where the resolver lives, and what it returns | anchored to `internal/component/iface/register.go` | "The resolver lives in the iface component, not in a standalone service and not as per-consumer translation." | agrees | `iface/mtu_check.go` and `doctor_mtu_linux.go` live inside `internal/component/iface` and resolve logical names (`os-name`, `mac/match`) through the shared resolver |
| `docs/architecture/iface/management.md` | Two layers, not one flat list | anchored to `iface/register.go`, `ze-iface-conf.yang` | "Validation happens at the YANG layer, so an invalid MTU, an out-of-range VLAN ID or an unknown interface kind is rejected before any netlink call." | agrees | Static `68..65535` MTU range continues to validate at the YANG layer, and device-specific `IFLA_MIN_MTU`/`IFLA_MAX_MTU` bounds additionally reject invalid MTUs before any `LinkSetMTU` write |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | RFC obligations carried by this code | anchored to `internal/component/iface/register.go` | - | silent | Covers PPPoE client lifecycle and session creation; untouched |
| `docs/architecture/plugin/plugin-relationships.md` | 1. System Plugins | anchored to `internal/component/iface/register.go` | - | silent | Covers cross-plugin dependencies and role claims; `host-tuning` declares no cross-plugin dependencies or claims |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `iface/register.go`, `validate_semantic.go`, `registry.go`; modified | "Side-effect-free equivalent of `OnConfigVerify`, used by static, API and CLI validation" | departs | Under D-1, optimization host-bound checks (`ring rx/tx`, `rx-queues/tx-queues`) run only in `InProcessConfigVerifier` while live `OnConfigVerify` omits them so boot/reload warn-and-skip rather than fail hard (protocol-correctness bounds like `interface mtu` still run in both); also `BootEffect` no longer runs "after host tuning" once `host-tuning` owns `system/tuning` (D-2). Page updated in this work |
| `docs/architecture/traffic/cos-plugin.md` | Decisions | anchored to `internal/component/iface/register.go` | - | silent | Covers class-of-service profiles and `ConfigReads` of `class-of-service`; untouched |
| `docs/architecture/cli/command-verbs.md` | The axis: does the command change the system | anchored to `internal/component/iface/cmd/manage.go` | - | silent | Covers verb taxonomy (`show`, `request`, `create`, `delete`); `request interface mtu` keeps its existing verb and grammar |
| `docs/architecture/iface/offload.md` | Absence means "keep the OS default" | anchored to `internal/component/iface/yang/ze-iface-conf.yang` | - | silent | Covers per-interface offload features (GRO/GSO/TSO/LRO/RPS/RFS); ring sizing lives under `system/tuning`, not `interface/offload` |
| `docs/architecture/iface/vlan-qos-map.md` | Decision: two maps, because the kernel has two | anchored to `internal/component/iface/yang/ze-iface-conf.yang` | - | silent | Covers 802.1p VLAN ingress/egress QoS maps; untouched |
| `ai/doors/appliance.md` | Where it sits | door covering `internal/component/config/system/yang/ze-system-conf.yang` | "On the device, the daemon is ordinary `ze`." | agrees | Only `ze-system-conf.yang` ring leaf descriptions change; appliance initrd, builder, zefs keys, and update backends are untouched |
| `docs/architecture/api/update-syntax.md` | Status | anchored to `internal/component/plugin/registry/registry.go` | - | silent | Covers BGP `update text` command grammar; untouched |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `internal/component/plugin/registry/registry.go` | - | silent | Covers BGP UPDATE forwarding hot path; untouched |
| `docs/architecture/decisions/001-pull-model-metrics.md` | The Question | anchored to `internal/component/plugin/registry/registry.go` | - | silent | Covers pull-model Prometheus metrics; no metrics added |
| `docs/architecture/dns/geodns.md` | Config model | anchored to `internal/component/plugin/registry/registry.go` | - | silent | Covers GeoDNS plugin; untouched |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/plugin/registry/registry.go` | - | silent | Covers IKEv2 engine; untouched |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to `internal/component/plugin/registry/registry.go` | - | silent | Covers BGP NLRI wire encoding; untouched |
| `docs/features/ai-first.md` | Structured Diagnostics | `// Design:` of `internal/core/diagnostic/codes.go`, `internal/component/config/doctor.go` | "The readiness checks `ze doctor` runs report codes from the same table, which the package owning each dependency declares" | agrees | `config-host-backend` stays in `codes.go` with expanded host-bound description, `config-semantics` (`config/doctor.go`) declares `CodeConfigHostBackend` in `semanticsDoctorCheck().Codes`, and every new `doctor-*` code (`doctor-host-tuning-not-applied`, `doctor-vpp-dpdk-queues-unreadable`, `doctor-vpp-dpdk-queues-skipped`, `doctor-iface-mtu-bounds-unreadable`) registers from its owning package (`internal/plugins/host-tuning`, `vpp`, `iface`) via `diagnostic.Register` |
| `ai/doors/core.md` | Boundaries | door covering `internal/core/diagnostic/codes.go` | "Where core needs something a component owns, the component registers into core." | agrees | Core `diagnostic` adds no component imports and no feature-owned `doctor-*` codes; `host-tuning` sits in edge plugin `internal/plugins/host-tuning` (outside `internal/component/host`) to respect the grandfathered `diagnostic -> host` edge without creating a cycle, and `internal/plugins/host-tuning`, `vpp`, and `iface` register their own checks and codes into `diagnostic` from `init()` |
| `ai/doors/le-tooling.md` | Boundaries | door covering `test/ui/*.ci`, `test/vpp/*.ci` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | Functional `.ci` tests use a `zetest`-only override to keep CI deterministic, while real kernel `ETHTOOL_GRINGPARAM`, `ETHTOOL_GCHANNELS`, and `RTM_GETLINK` proofs run in QEMU integration tests |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Does not alter kernel-vs-VPP data-plane mismatch refusal or `fib kernel`/`sysctl` exemptions |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Leaves data-plane mismatch evaluation untouched (superseded for start by the 2026-11-26 row below) |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Leaves VPP firewall verifier and `ze:backend` annotations untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Uses `kernel` and `vpp` data-plane names consistently; introduces no legacy aliases |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Touches no BGP export filter or next-hop configuration |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Touches no BGP policy chain authoring forms |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Touches no BGP engine filter chain compilation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Touches no BGP next-hop folding or `wireOverride` provenance |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Touches no BGP `policy { modify }` IPv4/IPv6 next-hop validation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Touches no BGP route origination or forwarding next-hop handling |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Touches no BGP filter chain or `\| implied` display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Optimization tuning (`ring rx/tx`, `rx-queues/tx-queues`) warns and skips at start so `ze start` never exits over tuning; `interface <name> mtu` is core networking itself (not an optional web/looking-glass service), already fails `iface.OnConfigure` at `b.SetMTU`, and was explicitly ruled fail-hard in D-1 (2026-11-24) because a wrong MTU breaks OSPF; reload refuses an invalid MTU whole |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Does not change kernel-feature-on-VPP start, commit, reload, or validate mismatch handling |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Touches no `\| implied` rendering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `host-tuning` registers through `registry.Register` with its `system/tuning` root and declares no custom bus bypass |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Touches no bus `send > to` routing rules |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Touches no bus `receive > from` routing rules |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Touches no bus message allow-lists |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Touches no bus filter compilation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Introduces no bus arbitration rules |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Touches no bus default display |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no command paths or non-pipe presentation flags |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Touches no generic pipe operators or overrides |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Touches no pipe operator behavior |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine operations; `host-tuning` uses existing config transaction callbacks |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Touches no event delivery queues |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Touches no fleet managed listener certificate reload |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Touches no fleet web provisioning surface |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | Touches no looking-glass HTTP/HTTPS listener rules |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | Touches no TLS material precedence |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-26 | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | Touches no web or looking-glass TLS startup/reload behavior |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | Touches no VPP supervision ownership rules |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | Touches no VPP restart mechanism |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | Touches no VPP reconnect recovery |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | Touches no VPP crash reflection |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | agrees | Touches no VPP liveness timers |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | agrees | Touches no VPP loss-handling or pidfd kill |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | agrees | Touches no VPP recovery forms |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | Touches no VPP restart route withdrawal |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | Touches no VPP systemd unit adaptation |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | agrees | Touches no VPP reconnect-and-verify timers |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | Touches no VPP data-presence adoption |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | Touches no VPP crash-loop or recovery deadline |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | Touches no VPP post-withdrawal retry backoff |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | Touches no VPP protocol timer behavior during recovery |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | Touches no VPP API-socket pidfd unit check |
| `docs/architecture/vpp-host-tuning.md` | Design decisions | decision of 2026-11-23 | "When Ze does not believe the host has enough resources for VPP to remain stable, it warns and still starts VPP. The requirement changes with the configuration and with what else runs on the host (on a general Linux install other programs take memory too), so the check runs at every start against the machine as it is then, and a warning tells the operator they are taking a risk. A measurement the operator recorded on their own hardware stops the warning on that hardware, and the operator can silence the warning with an environment value, set in the configuration or in the OS environment" | agrees | At start/reload, an over-maximum DPDK queue count warns under `doctor-vpp-dpdk-queues-skipped` and skips that direction so VPP still starts (D-1), matching the warn-and-start principle |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | Touches no `ze init` interface discovery or claim behavior |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | Touches no `ze init --confirm` prompt handling |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | `host-tuning` is a compiled-in plugin (`internal/plugins/host-tuning`); external plugin query-mode schema discovery is untouched |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator runs `ze config validate <file>` locally, or `show config validation` through the daemon on SSH, MCP, REST or web.
- An operator commits in the SSH or web editor, through the REST or gRPC config API, via HTTP/SSH config upload, or through MCP / artifact session commit.
- An operator runs `ze doctor` or `show doctor`.
- An operator runs `request interface <name> mtu <bytes>` or `ze interface mtu <name> <bytes>`.
- The daemon boots (`OnConfigure`) or reloads (`OnConfigVerify` / `OnConfigApply`) `host-tuning`, `vpp`, and `iface`.

### Transformation Path
1. The candidate becomes a `config.Tree`; `ToPluginMap` lowers it.
2. On validate and commit, `VerifyPluginConfigMapTransition` hands each owner its roots: `system/tuning` (`host-tuning`), `vpp`, and `interface`.
3. Each owner's in-process verifier reads the host once per subject: ring and channel counts through the ethtool wrapper, MTU bounds through raw `RTM_GETLINK` (`IFLA_MIN_MTU`, `IFLA_MAX_MTU`) after resolving logical interface names through the shared resolver.
4. A value outside the host's bound becomes an `opererr` error marked `HostVerdict`, carrying the configured value, the host's number, the subject, the host name, the measuring process (`local process` or `daemon`), and the exact `set` fix. An unreadable subject produces no error.
5. `runValidation` and `ValidateSemantics` report every `HostVerdict` under `config-host-backend`; every commit surface refuses the commit with that message before anything is written.
6. Each owner's post-config doctor check reads the same host and warns for a subject it could not read: `iface-mtu-bounds` (`doctor-iface-mtu-bounds-unreadable`) for existing kernel devices, and `vpp-dpdk-queues` (`doctor-vpp-dpdk-queues-unreadable`) for DPDK NICs not bound to `vfio-pci` (staying silent when bound to `vfio-pci` per D-3).
7. At boot and reload (D-1):
   - `host-tuning` (`system/tuning`): live `OnConfigVerify` carries no host read; at `OnConfigure` / `OnConfigApply`, `setEthtoolRing` compares each direction with the maximum it just read, skips any direction above its maximum with a non-fatal `TuningError` logged at `Warn` under `doctor-host-tuning-not-applied`, and writes the other direction if it fits.
   - `vpp`: live `parseVPPTransaction` carries no host queue read; at `VPPManager.writeStartupConf` (before `bindAll`), if a NIC on its kernel driver reports a channel max below the configured `rx-queues` or `tx-queues`, `writeStartupConf` skips emitting that direction's line in `startup.conf` and logs a `Warn` under `doctor-vpp-dpdk-queues-skipped`.
   - `iface`: both live `OnConfigVerify` (reload) and `OnConfigure` (boot) run the MTU device-bounds check and fail hard on an out-of-bounds MTU before kernel mutation (and for newly created devices, `applyAndPublish` -> `b.SetMTU` fails hard via `joinApplyErrors`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Verifier and apply ↔ driver | `SIOCETHTOOL` through `ethtoolIoctl` (`ETHTOOL_GRINGPARAM`, `ETHTOOL_GCHANNELS`) | No |
| Verifier, live callbacks, and CLI ↔ kernel | `RTM_GETLINK`, attributes `IFLA_MIN_MTU`, `IFLA_MAX_MTU` | No |
| Config ↔ owner component | `InProcessConfigVerifier` and transaction callbacks (`OnConfigure`, `OnConfigVerify`, `OnConfigApply`), by registration | No |
| Doctor registry ↔ owner | `diagnostic.RegisterDoctorCheck` and `diagnostic.Register` from each owner's `init()` | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`; terms `config-host-backend`, `CodeConfigPluginVerify`, `HostVerdict`, `num-rx-queues`, `ETHTOOL_SRINGPARAM`. Positive control: `runValidation` (`internal/component/config/cli/cmd_validate.go:656-659`) writes `config-host-backend` while `ValidateSemantics` (`internal/component/config/validate_semantic.go:68`) writes `config-plugin-verify` and `semanticsDoctorCheck().Codes` (`internal/component/config/doctor.go:31-36`) omits `CodeConfigHostBackend` for the same verifier error. Finding: no wire format or schema changes. Refusals travel in the existing validation result and `diagnostic.Diagnostic` shapes, doctor JSON keys (`code`, `severity`, `message`, `fix`, ...) are unchanged, `host-tuning` (`internal/plugins/host-tuning`) uses the existing `rpc.ConfigSection` transaction protocol with `ConfigRoots: []string{"system/tuning"}` and `WantsConfig: []string{"system/tuning"}` and adds no RPC fields, and `startup.conf`'s `num-rx-queues`/`num-tx-queues` lines keep their existing syntax (omitted per direction only on a D-1 boot/reload skip). The one emitted-value change is paired across producer and consumers: producer `ValidateSemantics` (`validate_semantic.go`) plus its declared code list `semanticsDoctorCheck().Codes` (`config/doctor.go`) emit `config-host-backend` (instead of `config-plugin-verify`) for `HostVerdict` errors; consumers are the doctor text/JSON renderers (`internal/component/doctor/doctor.go` `outputText`, `outputJSON`, which render the `Code` field generically) and any `.ci` test asserting doctor's code for a host verdict, which the implementation greps and updates in the same work.

### Integration Points
- `internal/plugins/host-tuning/register.go` - registers the `host-tuning` participant (`ConfigRoots: []string{"system/tuning"}`, `sdk.Registration{WantsConfig: []string{"system/tuning"}}`); its `InProcessConfigVerifier` runs the ring check and its `OnConfigure`/`OnConfigApply` call `host.ApplyTuning`, replacing `cmd/ze/hub/main_system.go`'s `applyHostTuning` and `applyHostTuningFromMap` while keeping `internal/component/host` free of `internal/core/diagnostic` imports and satisfying `pluginDirs` and `./le arch tier check`.
- `internal/component/vpp/register.go` and `internal/component/vpp/vpp.go` - `verifyVPPConfig` (`InProcessConfigVerifier`) runs the queue check; live `parseVPPTransaction` does not; `writeStartupConf` applies the D-1 warn-and-skip before `bindAll`.
- `internal/component/iface/register.go` - `verifyIfaceConfig`, live `OnConfigVerify`, and `OnConfigure` run the MTU device-bounds check so validate, commit, reload, and boot all fail hard (D-1).
- `internal/component/iface/cmd/manage.go` and `internal/component/iface/cli/link.go` - share the MTU device-bounds reader before calling `iface.SetMTU`.
- `internal/component/config/validate_semantic.go` and `internal/component/config/doctor.go` - map `diagnostic.IsHostVerdict` errors to `diagnostic.CodeConfigHostBackend` and declare `CodeConfigHostBackend` in `semanticsDoctorCheck().Codes`.
- `internal/core/diagnostic/codes.go` - expands `CodeConfigHostBackend` description; feature-owned `doctor-*` codes register via `diagnostic.Register` in `internal/plugins/host-tuning/doctor_codes.go`, `vpp/register_doctor_codes.go`, and `iface/doctor_codes.go`.

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
| A-1 | `ETHTOOL_GRINGPARAM` fills `rxMaxPending` and `txMaxPending` on the drivers Ze runs on, and a zero there means the driver reports no ring support rather than a limit of zero | `internal/component/host/ethtool_linux.go` declares the fields; the kernel ethtool ABI defines them | A zero read as a limit refuses every ring on that driver | Unit test over a fake `ringparam`, then a QEMU run on virtio-net, the driver the appliance boots with | unvalidated |
| A-2 | `ETHTOOL_GCHANNELS` reports the maximum receive and transmit queue counts for a NIC still on its kernel driver, and the netdev is found from the PCI address under `/sys/bus/pci/devices/<pci>/net/`; once `DPDKBinder.bindPCI` moves it to `vfio-pci` the read fails cleanly | `internal/component/vpp/dpdk.go` saves the previous driver, and `VPPManager.runWithOps` runs `writeStartupConf` before `bindAll`, so before the bind is the normal validate and boot-apply state | The queue check is unreadable in the common case and only warns | QEMU integration test binding and unbinding a device, asserting refusal before the bind, silence in doctor after the bind (D-3), and no refusal after | unvalidated |
| A-3 | `IFLA_MAX_MTU` and `IFLA_MIN_MTU` are present in `RTM_GETLINK` replies on Ze's kernel, and a `max_mtu` of zero means the device sets no ceiling of its own rather than a ceiling of zero | `vendor/golang.org/x/sys/unix/ztypes_linux.go` declares both; the vendored `vishvananda/netlink` link attributes do not expose them, so the read is a raw attribute read | The MTU check reads nothing, or refuses every MTU on a device with no ceiling | Unit test over a captured reply plus a QEMU assertion on a real link and on a dummy device; read the kernel's `dev_validate_mtu` in Ze's kernel source for the zero case | unvalidated |
| A-4 | Every commit surface reaches `VerifyPluginConfigMapTransition`: `ze config validate`, the SSH editor (`ConfigValidator.Validate` / `ValidateTransition`), the web editor (`newEditorFactory(zeconfigcmd.ValidateContent)`), HTTP/SSH config upload (`ValidateContent`), REST and gRPC (`configValidationHook` calling `ValidateContent` + `VerifyPluginConfigContentTransition`), and MCP / artifact commits (`artifact_session.go` calling `ValidateContent` + `ValidateTransition`, `config_detached.go` calling `ValidateTransition`) | Source verified 2026-11-26: `cmd_validate.go:38,650`, `validator.go:130,179`, `service_web.go:446,554`, `config_upload.go:119`, `api.go:37-40`, `artifact_session.go:60,113`, `config_detached.go:344`; MCP has no separate handcrafted commit handler and dispatches through the shared command registry | A surface would bypass `InProcessConfigVerifier` and commit an over-large ring or queue count | Verified by code trace across all seven surfaces; functional `.ci` tests cover validate, editor commit, and doctor | validated |
| A-5 | `os.Hostname` plus the measuring process label (`local process` vs `daemon`) names the measured host clearly enough for an operator to spot a wrong-host verdict | The daemon route (`show config validation` over SSH/API/MCP/web) runs in the daemon's process; offline `ze config validate` runs in the caller's process | Two hosts sharing a hostname could be confused without process provenance | Unit test `TestRingCheckMessageNamesEveryFact` and sibling queue/MTU message tests assert hostname and process role appear in every refusal | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `ze config validate` becomes machine-dependent: the same file validates on one host and not another | A `.ci` that passes locally and fails in CI | That dependence is the point, bounded by the unreadable state. Functional tests force the host answer in a `zetest` build; only QEMU tests touch a real device |
| R-2 | Wrong host: an operator validates a router's file on a workstation with an interface of the same name on another driver, and gets a refusal or a pass about the workstation | A refusal naming the workstation's host name | Ze cannot know which host a file is for. The message names the host and the process that measured, so the operator sees it; validating on the router (`show config validation` over SSH) reads the router. "Unreadable refuses nothing" covers an absent or unreadable device only, never a present device on the wrong machine |
| R-3 | Widening the MTU ceiling to 65535 loses the completion hint the old ceiling gave | Completion offers 65535 on a NIC that takes 9216 | The refusal names the device's real maximum. Offering the host's maximum as completion is half (b)'s question, which is why R-3 is copied there |
| R-4 | A host probe per validate on a config with 200 interfaces issues 200 ioctls | Commit latency on a large config | One read per subject per verify, the result passed to the comparison, as `hostCPUInventory` is read once and passed to `validateAgainst` |
| R-5 | A driver or firmware change lowers a maximum, so an unchanged value is refused at the next commit of anything, or met at boot/reload | A commit of an unrelated change refused, or a boot/reload encountering a lowered limit | At commit: validate and commit agree, and the message names the new maximum; doctor reports it first through `config-semantics`. At boot/reload (D-1): ring and DPDK queue bounds warn and skip non-fatally so the router keeps booting; `interface mtu` fails hard because a wrong MTU breaks OSPF and similar protocols |
| R-6 | Keeping optimization host reads (`ring`, `queues`) out of live `OnConfigVerify` departs from `InProcessConfigVerifier` being the exact equivalent of `OnConfigVerify` | A reviewer reads the split as drift | Recorded as an intentional departure in `## Design documents` and documented in `registry.go`, `plugin-system.md`, and Layer 3 of `yang-config-design.md`: optimization host verdicts run in `InProcessConfigVerifier` only, while protocol-correctness bounds (`interface mtu`) run in both. `TestOptimizationLiveVerifyCarriesNoHostRead` and `TestIfaceMTUFailsHardAtBootAndReload` pin both halves |
| R-7 | Adding the `host-tuning` participant (`internal/plugins/host-tuning/register.go`) changes startup ordering relative to `RunBootEffects` and adds `internal/plugins/host-tuning` to `plugin/all` | `TestRegisteredPluginNames` / `TestConfigSchemaRootsClaimed` or `arch tier check` fails | Placing `host-tuning` in edge plugin `internal/plugins/host-tuning` (under `internal/plugins/`, already in `pluginDirs`) avoids both the `diagnostic -> host` and `config/system -> host` import cycles and satisfies `./le arch tier check` (`engine-placement` and `non-engine-categories`); `host-tuning` parses `system/tuning` directly, updates `allowlist.json` and `plugin/all` snapshots via `./le repo update`, and updates `hub-architecture.md`, `crash-capture.md` and `plugin-system.md` in the same work |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config that used to commit is refused on the three subjects only. A wrong probe refuses a legal ring, queue count or MTU and blocks a commit; for `interface mtu`, a wrong probe also fails boot or reload (D-1). Ring and DPDK queue bounds warn-and-skip at boot/reload and cannot stop a router from starting. Nothing on the wire changes and no session drops |
| How is it reverted? | Single commit revert per phase |
| Who else touches this path? | `spec-doctor-coverage-15-system` (the `host-tuning` doctor check, sharing `doctor-host-tuning-not-applied`), `spec-iface-ethernet-speed-duplex` (a NIC read in iface verify), `spec-yang-single-source-of-truth` (the same YANG leaves) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` over a config setting `system tuning ethtool <if> ring rx` | → | ring check in `host-tuning` `InProcessConfigVerifier` | `TestRingCheckRefusesAboveTheDriverMaximum` |
| `ze config validate` over `vpp dpdk interface <pci> rx-queues` | → | queue check in VPP's `InProcessConfigVerifier` | `TestQueueCheckRefusesAboveTheNICMaximum` |
| `ze config validate` over `interface <name> mtu` | → | MTU device-bounds check in `verifyIfaceConfig` | `TestMTUCheckRefusesOutsideTheDeviceBounds` |
| Live `OnConfigVerify` and `OnConfigure` of `host-tuning` and VPP | → | no host ring/queue refusal; warn-and-skip at apply (D-1) | `TestOptimizationLiveVerifyCarriesNoHostRead` |
| Live `OnConfigVerify` and `OnConfigure` of `iface` with out-of-bounds MTU | → | MTU device-bounds check fails hard at boot and reload (D-1) | `TestIfaceMTUFailsHardAtBootAndReload` |
| `ze doctor` over an over-large ring | → | `config-semantics` under `config-host-backend` | `TestDoctorReportsAHostVerdictUnderTheHostCode` |
| An operator commits an over-large ring in the editor | → | the whole chain | `test/ui/config-validate-ring-above-driver-max.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `system tuning ethtool eth0 ring rx N`, N above the driver's reported receive maximum | `ze config validate` fails under `config-host-backend`; the message names N, the maximum, `eth0`, the host name and the measuring process, and its last line is the `set` command at the maximum | A ring at or below the maximum validates (AC-3) |
| AC-2 | The same for `ring tx` against the transmit maximum | The same refusal with the transmit maximum | As AC-1 |
| AC-3 | A ring at or below the driver's maximum | Validate passes and the `host-tuning` participant applies the value | Every value the driver reports it takes, because the check never narrows what the driver accepts |
| AC-4 | The ring parameters cannot be read: no such interface, a driver with no ring support, or a reported maximum of zero (A-1) | Validate refuses nothing; the `host-tuning` doctor check of `spec-doctor-coverage-15-system` AC-14 warns naming the interface and the reason under `doctor-host-tuning-not-applied` | Every value, because Ze has no number to judge against |
| AC-5 | The same over-large ring committed in the SSH editor, the web editor, REST, gRPC, HTTP/SSH upload, or MCP/artifact commit | The commit is refused with the AC-1 message before anything is written | Commits that leave the ring at or below the maximum |
| AC-6 | Boot or reload apply of `system tuning ethtool <if> ring rx/tx` with a direction above the maximum just read | `setEthtoolRing` skips writing any direction above its maximum, writes the other direction if it fits, and returns a `TuningError` naming the configured value, the maximum, the interface and the fix; `host-tuning` logs it as a non-fatal `Warn` under `doctor-host-tuning-not-applied`; any failed ioctl names the errno | A direction inside its maximum is still written, because the two directions are independent |
| AC-7 | `vpp dpdk interface <pci> rx-queues N`, N above the NIC's maximum receive queue count, read while the device is on its kernel driver | Validate and commit fail under `config-host-backend`, naming N, the maximum, the PCI address, the netdev, the host, the measuring process and the `set` fix | A count at or below the maximum validates; an explicit 0 keeps today's meaning |
| AC-8 | The same for `tx-queues` | The same refusal with the transmit maximum | As AC-7 |
| AC-9 | A configured DPDK PCI device is bound to `vfio-pci` (held by VPP), or is on a kernel driver / unbound with no readable netdev or `ETHTOOL_GCHANNELS` answer | Validate refuses nothing in both cases. In `ze doctor`, the `vpp-dpdk-queues` check stays silent for a device already bound to `vfio-pci` (D-3), and warns under `doctor-vpp-dpdk-queues-unreadable` naming the PCI address, the reason and the fix when the device is not bound to `vfio-pci` and its queue maximum cannot be read | Every count passes validate because the NIC cannot be asked; healthy VPP routers with `vfio-pci`-bound NICs carry no false-alarm doctor warning |
| AC-10 | `interface <name> mtu N` on a device that exists, N above its `IFLA_MAX_MTU` or below its `IFLA_MIN_MTU` | Validate and commit fail under `config-host-backend`, naming N, the bound crossed, the logical name and the OS device name, the host, the measuring process and the `set` fix | An MTU inside the device bounds validates |
| AC-11 | `interface mtu` above 16000 that the device's bounds allow, up to 65535 | Validate passes and the value is applied; `ze-iface-conf.yang`, `ze-iface-cmd.yang`, `handleInterfaceMTU` and `cmdMTU` agree on one static bound `68..65535` | Every value the device accepts, because the static range no longer copies a host fact |
| AC-12 | `request interface <name> mtu <bytes>` or `ze interface mtu <name> <bytes>` outside the device bounds | The command is refused before the backend call, with the same bounds and wording as AC-10, from the same reader | An MTU inside the bounds is applied as today |
| AC-13 | The device does not exist yet (one Ze creates: dummy, veth, bridge, tunnel), is held by VPP, or its reply carries no bound attribute (or `max_mtu == 0`) | Validate refuses nothing; for an existing kernel device whose bounds could not be read, `iface-mtu-bounds` doctor check warns under `doctor-iface-mtu-bounds-unreadable` naming the device, the reason and the fix | Every value passes validate when there is no kernel device to ask; devices Ze creates are judged by the kernel at apply |
| AC-14 | Daemon start (`OnConfigure`), and `ze signal reload` / `request reload` (`OnConfigVerify` / `OnConfigApply`) of a hand-edited file, with a value outside a host bound that changed since the last commit (D-1) | Optimization bounds (`system tuning ethtool ring rx/tx` and `vpp dpdk interface rx-queues/tx-queues`) are NOT refused by live `OnConfigure` or `OnConfigVerify`: `host-tuning` warns and skips the over-maximum ring direction under `doctor-host-tuning-not-applied` (AC-6), and `VPPManager.writeStartupConf` (before `bindAll`) skips the over-maximum `num-rx-queues`/`num-tx-queues` line and logs `Warn` under `doctor-vpp-dpdk-queues-skipped`. Protocol-correctness bound `interface <name> mtu` MUST fail hard at both boot (`OnConfigure`) and reload (`OnConfigVerify` before kernel mutation, and `applyAndPublish` -> `b.SetMTU` for newly created devices), refusing the bad MTU with the AC-10 wording rather than warning and skipping | Optimization values that still fit apply unchanged; on reload, a hard MTU refusal preserves the running interface configuration |
| AC-15 | Any refusal above, seen through `ze doctor` | `config-semantics` (`ValidateSemantics` and `semanticsDoctorCheck().Codes` in `internal/component/config/doctor.go`) reports it once, under `config-host-backend`, with the same message; the `host-tuning` check of `spec-doctor-coverage-15-system` is silent for a ring above the reported maximum | Other `config-plugin-verify` refusals keep their code |
| AC-16 | Local offline `ze config validate` on a workstation whose `eth0` sits on another driver vs `show config validation` through the daemon over SSH | The offline verdict is the workstation's and names its hostname and `local process`; through the daemon the verdict is the router's and names its hostname and `daemon` | Nothing is hidden: Ze cannot know which host a file was written for |
| AC-17 | `ze show diagnostic` for `config-host-backend` and each new doctor code (`doctor-host-tuning-not-applied`, `doctor-vpp-dpdk-queues-unreadable`, `doctor-vpp-dpdk-queues-skipped`, `doctor-iface-mtu-bounds-unreadable`) | `config-host-backend` is updated in `internal/core/diagnostic/codes.go` to cover host bounds and declared in `semanticsDoctorCheck().Codes` (`internal/component/config/doctor.go`); each feature-owned `doctor-*` code is registered via `diagnostic.Register` in its owning package's `init()` (`internal/plugins/host-tuning/doctor_codes.go`, `vpp/register_doctor_codes.go`, `iface/doctor_codes.go`) rather than in `codes.go`; each new doctor check declares the exact code it emits | None: every emitted code must resolve in `ze show diagnostic <code>` and disappear if its owning component is compiled out |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets a ring larger than the NIC supports and commits | editor → tree → `host-tuning` in-process verifier → ethtool read → refusal naming the maximum | `test/ui/config-validate-ring-above-driver-max.ci` |
| 2 | Sets a ring the NIC supports | same path, no refusal, `host-tuning` `OnConfigApply` writes it via `applyEthtoolRing` | `test/ui/config-validate-ring-at-driver-max.ci` |
| 3 | Sets a DPDK queue count above the NIC maximum before VPP binds the device | validate → VPP in-process verifier → channel read → refusal naming the PCI address | `test/vpp/vpp-validate-queues-above-nic-max.ci` |
| 4 | Runs `ze doctor` with an over-large ring in the file | doctor → `config-semantics` → `config-host-backend` row | `test/ui/doctor-ring-above-driver-max.ci` |
| 5 | Sets an MTU of 9216 on a NIC whose maximum is 9000, then 9000 | validate → iface verifier → `RTM_GETLINK` → refusal, then pass | `test/ui/config-validate-mtu-device-bounds.ci` |
| 6 | Boots or reloads a router whose ring or DPDK queue count now exceeds a lowered driver/NIC maximum | `host-tuning` `OnConfigure`/`OnConfigApply` → AC-6 warning under `doctor-host-tuning-not-applied`; `VPPManager.writeStartupConf` → skip + warning under `doctor-vpp-dpdk-queues-skipped`; no start/reload refusal | `TestApplyRingRefusesAboveTheReadMaximum`, `TestOptimizationLiveVerifyCarriesNoHostRead`, `TestVPPWriteStartupConfSkipsOverMaxQueues`, plus QEMU run |
| 7 | Boots or reloads a router whose `interface <name> mtu` exceeds the device's `IFLA_MAX_MTU` | `iface` `OnConfigure` / `OnConfigVerify` → `RTM_GETLINK` bounds check → hard failure naming device bounds before kernel mutation | `TestIfaceMTUFailsHardAtBootAndReload` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRingCheckRefusesAboveTheDriverMaximum` | `internal/plugins/host-tuning/ring_check_test.go` | AC-1, AC-2 against a stated host | |
| `TestRingCheckAcceptsAtTheDriverMaximum` | same | AC-3 | |
| `TestRingCheckRefusesNothingWhenUnreadable` | same | AC-4, including a zero maximum | |
| `TestRingCheckMessageNamesEveryFact` | same | AC-1, AC-16: value, maximum, interface, host, process, `set` fix line | |
| `TestHostTuningParticipantAppliesAndReloads` | `internal/plugins/host-tuning/register_test.go` | D-2: `host-tuning` participant owns `system/tuning` (`ConfigRoots` + `WantsConfig`), runs verifier, logs `doctor-host-tuning-not-applied` on apply error | |
| `TestRingParamReadKeepsTheMaxima` | `internal/component/host/ethtool_test.go` | the read stops discarding the maxima and propagates errno | |
| `TestApplyRingRefusesAboveTheReadMaximum` | `internal/component/host/tuning_test.go` | AC-6, both directions independent, errno in the error | |
| `TestQueueCheckRefusesAboveTheNICMaximum` | `internal/component/vpp/queue_check_test.go` | AC-7, AC-8 | |
| `TestQueueCheckRefusesNothingWhenDPDKBound` | same | AC-9 validate behavior | |
| `TestVPPQueueDoctorSilentOnVFIOAndWarnsWhenUnreadable` | `internal/component/vpp/doctor_queue_linux_test.go` | AC-9, D-3: silent on `vfio-pci`, emits `doctor-vpp-dpdk-queues-unreadable` on unreadable kernel-driver NIC | |
| `TestVPPWriteStartupConfSkipsOverMaxQueues` | `internal/component/vpp/startupconf_test.go` | AC-14, D-1: skips over-max direction, keeps fitting direction, warns under `doctor-vpp-dpdk-queues-skipped` | |
| `TestMTUCheckRefusesOutsideTheDeviceBounds` | `internal/component/iface/mtu_check_test.go` | AC-10, both `IFLA_MIN_MTU` and `IFLA_MAX_MTU` | |
| `TestMTUCheckAcceptsAboveTheOldCeiling` | same | AC-11 | |
| `TestMTUCheckRefusesNothingWithoutTheDevice` | same | AC-13, including a zero `max_mtu` | |
| `TestMTUCommandAndLeafShareOneReader` | same | AC-12 (`request interface mtu` and `ze interface mtu`) | |
| `TestIfaceMTUDoctorWarnsWhenBoundsUnreadable` | `internal/component/iface/doctor_mtu_linux_test.go` | AC-13: emits `doctor-iface-mtu-bounds-unreadable` for unreadable existing kernel device | |
| `TestOptimizationLiveVerifyCarriesNoHostRead` | `internal/plugins/host-tuning/register_test.go`, `internal/component/vpp/register_test.go` | AC-14, D-1: `host-tuning` and VPP live `OnConfigure`/`OnConfigVerify` carry no host ring/queue read | |
| `TestIfaceMTUFailsHardAtBootAndReload` | `internal/component/iface/register_test.go` | AC-14, D-1: `iface` `OnConfigure` and `OnConfigVerify` fail hard on out-of-bounds MTU | |
| `TestDoctorReportsAHostVerdictUnderTheHostCode` | `internal/component/config/validate_semantic_test.go` | AC-15, AC-17: `ValidateSemantics` emits `CodeConfigHostBackend` and `semanticsDoctorCheck().Codes` declares it | |
| `TestEveryNewCodeIsDeclared` | `internal/plugins/host-tuning/doctor_codes_test.go`, `vpp/register_doctor_codes_test.go`, `iface/doctor_codes_test.go` | AC-17: each feature-owned code resolves via `diagnostic.Lookup` and matches its check declaration | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `system tuning ethtool <if> ring rx` | 1 to the driver's receive maximum | the maximum | 0 | the maximum plus 1 |
| `system tuning ethtool <if> ring tx` | 1 to the driver's transmit maximum | the maximum | 0 | the maximum plus 1 |
| `vpp dpdk interface <pci> rx-queues` | 0 to the NIC's receive queue maximum | the maximum | N/A | the maximum plus 1 |
| `vpp dpdk interface <pci> tx-queues` | 0 to the NIC's transmit queue maximum | the maximum | N/A | the maximum plus 1 |
| `interface <name> mtu` | the device's minimum to its maximum, inside 68..65535 | the device maximum | the device minimum minus 1 | the device maximum plus 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-validate-ring-above-driver-max` | `test/ui/config-validate-ring-above-driver-max.ci` | Story 1, with the host answer forced in the `zetest` build | |
| `config-validate-ring-at-driver-max` | `test/ui/config-validate-ring-at-driver-max.ci` | Story 2 | |
| `config-validate-ring-unreadable` | `test/ui/config-validate-ring-unreadable.ci` | AC-4 | |
| `doctor-ring-above-driver-max` | `test/ui/doctor-ring-above-driver-max.ci` | Story 4 | |
| `vpp-validate-queues-above-nic-max` | `test/vpp/vpp-validate-queues-above-nic-max.ci` | Story 3 | |
| `config-validate-mtu-device-bounds` | `test/ui/config-validate-mtu-device-bounds.ci` | Story 5 | |
| QEMU ring, channel and MTU reads | `./le test qemu`, integration tests in `internal/component/host`, `vpp`, and `iface` | A-1, A-2, A-3 on a real virtio-net device and a dummy device | |

The forced host answer follows `ze.test.kernelcap.force`: a private variable
registered and read only in a `zetest` build, so a shipped `ze` cannot be told
a false maximum.

### Interop Tests (Scope: protocol)
Not applicable: nothing reaches the wire. This is config validation and host
apply over local kernel/driver reads, the case `ai/rules/interop-and-goal-validation.md`
exempts.

## Files to Modify
- `internal/component/host/ethtool_linux.go` - keep `rxMaxPending` and `txMaxPending` on ring read, export ring/channel bounds readers (`ETHTOOL_GRINGPARAM`, `ETHTOOL_GCHANNELS`), return errno in every error (stays free of `internal/core/diagnostic`)
- `internal/component/host/tuning_linux.go` - `setEthtoolRing` compares each direction with the maximum it read, skips any over-maximum direction with a non-fatal `TuningError`, and removes the stale YANG-range `nolint` reasons
- `internal/component/host/tuning.go` - export the ring bounds types/reader contract consumed by `internal/plugins/host-tuning` (no `internal/core/diagnostic` import)
- `cmd/ze/hub/main_system.go` - delete `applyHostTuning` and `applyHostTuningFromMap`
- `cmd/ze/hub/main.go` - remove the `applyHostTuning(&sc)` startup call
- `cmd/ze/hub/main_reload.go` - remove the `applyHostTuningFromMap(newTree)` reload call
- `internal/component/config/claims/allowlist.json` - update `system` entry to drop `ExtractTuningFromMap` now that `host-tuning` claims `system/tuning`
- `internal/component/plugin/all/all.go` - regenerated via `./le repo update` to blank-import `internal/plugins/host-tuning` (and update `testdata/` snapshots)
- `internal/component/vpp/register.go` - `verifyVPPConfig` (`InProcessConfigVerifier`) runs the queue check; live `parseVPPTransaction` stays free of host queue reads
- `internal/component/vpp/register_linux.go` - add `vppDPDKQueuesDoctorCheck()` to `vppLinuxDoctorChecks()`
- `internal/component/vpp/register_doctor_codes.go` - register `doctor-vpp-dpdk-queues-unreadable` and `doctor-vpp-dpdk-queues-skipped` via `diagnostic.Register`
- `internal/component/vpp/vpp.go` - `writeStartupConf` reads each DPDK NIC's channel max before `bindAll` and skips over-maximum directions with a `Warn` under `doctor-vpp-dpdk-queues-skipped`
- `internal/component/vpp/startupconf.go` - omit skipped `num-rx-queues`/`num-tx-queues` directions when instructed by `writeStartupConf`
- `internal/component/vpp/yang/ze-vpp-conf.yang` - update `rx-queues` and `tx-queues` descriptions to state NIC-max validation and boot/reload warn-and-skip
- `internal/component/iface/register.go` - run the MTU device-bounds check in `verifyIfaceConfig`, live `OnConfigVerify`, and `OnConfigure` (fail hard at boot and reload per D-1); register `ifaceMTUBoundsDoctorCheck`
- `internal/component/iface/doctor_codes.go` - register `doctor-iface-mtu-bounds-unreadable` via `diagnostic.Register`
- `internal/component/iface/cmd/manage.go` - `handleInterfaceMTU` refuses outside the device bounds with the shared reader, sharing the single `68..65535` static declaration
- `internal/component/iface/cli/link.go` - `cmdMTU` refuses outside the device bounds with the shared reader
- `internal/component/iface/yang/ze-iface-conf.yang` - widen `mtu` range to `68..65535`, update description to name device bounds and fail-hard boot/reload behavior
- `internal/component/config/system/yang/ze-system-conf.yang` - update `ring rx` and `ring tx` descriptions to state driver-max validation and apply-time warn-and-skip
- `internal/component/config/validate_semantic.go` - report `diagnostic.IsHostVerdict` errors under `diagnostic.CodeConfigHostBackend`
- `internal/component/config/doctor.go` - declare `diagnostic.CodeConfigHostBackend` in `semanticsDoctorCheck().Codes`
- `internal/component/plugin/registry/registry.go` - update `InProcessConfigVerifier` comment to state the host-verdict exception for optimization bounds
- `internal/core/diagnostic/codes.go` - expand `CodeConfigHostBackend` (`config-host-backend`) Title and Description to cover host-reported numeric bounds
- `docs/architecture/host/tuning.md` - document commit-time ring refusal beside non-fatal apply-time skip (`doctor-host-tuning-not-applied`) and `host-tuning` participant ownership in `internal/plugins/host-tuning`
- `docs/architecture/host/inventory.md` - document ring maxima, `ETHTOOL_GCHANNELS` channel read, and errno propagation in `ethtool_linux.go`
- `docs/architecture/hub-architecture.md` - update `Boot effects` after removing direct hub `applyHostTuning`
- `docs/architecture/diagnostics/crash-capture.md` - update `The kernel half` to remove the stale "after host tuning" phrasing for `RunBootEffects`
- `docs/architecture/plugin/plugin-system.md` - update `Registration fields` (`InProcessConfigVerifier` host-verdict exception for optimization bounds; `BootEffect` wording after `host-tuning` participant)
- `docs/architecture/config/yang-config-design.md` - update Layer 3 (`InProcessConfigVerifier`) to document the host-verdict severity split
- `docs/architecture/doctor-and-health-checks.md` - document `vpp-dpdk-queues` (silent on `vfio-pci`) and `iface-mtu-bounds` checks, their feature-owned codes, `config-semantics` declaring `config-host-backend`, and the single-row rule with `host-tuning`
- `docs/features/interfaces.md` - update MTU bounds (`68..65535` static, `IFLA_MIN_MTU`/`IFLA_MAX_MTU` device bounds, fail-hard boot/reload)
- `docs/guide/vpp.md` - document DPDK queue refusal, `vfio-pci` silence, and boot/reload warn-and-skip
- `docs/guide/configuration.md` - document device-owned MTU bounds and `68..65535` ceiling
- `docs/guide/command-reference.md` - update `config-host-backend` row and `request interface mtu` device-bounds refusal
- `docs/guide/plugins.md` - list the `host-tuning` participant (`system/tuning`)
- `docs/guide/status.md` - list the new doctor checks and codes
- `features/<id>.md` for host tuning, VPP and interfaces - update changed behavior

## Files to Create
- `internal/plugins/host-tuning/register.go` - `host-tuning` transaction participant (edge plugin under `internal/plugins/`, already in `pluginDirs`, passing `./le arch tier check` and avoiding the `diagnostic -> host` import cycle) owning `ConfigRoots: []string{"system/tuning"}` and `sdk.Registration{WantsConfig: []string{"system/tuning"}}`, with `InProcessConfigVerifier` (ring check) and live `OnConfigure`/`OnConfigVerify`/`OnConfigApply`
- `internal/plugins/host-tuning/doctor_codes.go` - feature-owned `diagnostic.Register` registration of `doctor-host-tuning-not-applied`
- `internal/plugins/host-tuning/ring_check.go` - ring bounds comparison and `opererr` message marked `HostVerdict`
- `internal/component/vpp/queue_check.go` - PCI netdev + `ETHTOOL_GCHANNELS` reader, queue comparison, and `opererr` message marked `HostVerdict`
- `internal/component/vpp/doctor_queue_linux.go` - `vpp-dpdk-queues` post-config doctor check (silent on `vfio-pci`, warns under `doctor-vpp-dpdk-queues-unreadable` otherwise)
- `internal/component/iface/mtu_check.go` - raw `RTM_GETLINK` `IFLA_MIN_MTU`/`IFLA_MAX_MTU` reader via shared resolver, comparison, and `opererr` message marked `HostVerdict`
- `internal/component/iface/doctor_mtu_linux.go` - `iface-mtu-bounds` post-config doctor check (`doctor-iface-mtu-bounds-unreadable`)
- `test/ui/config-validate-ring-above-driver-max.ci` - Story 1
- `test/ui/config-validate-ring-at-driver-max.ci` - Story 2
- `test/ui/config-validate-ring-unreadable.ci` - AC-4
- `test/ui/doctor-ring-above-driver-max.ci` - Story 4
- `test/vpp/vpp-validate-queues-above-nic-max.ci` - Story 3
- `test/ui/config-validate-mtu-device-bounds.ci` - Story 5

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No node is added |
| YANG validation constraints | Yes | `interface mtu` widens to `68..65535` matching `ze-iface-cmd.yang`; ring leaves already equal `uint16` (`1..65535`); queue leaves keep `uint8` |
| YANG custom validators | No | Host-bound checks live in `InProcessConfigVerifier` (plus live `iface` callbacks for MTU), not in `ze:validate` |
| CLI commands/flags | No | `request interface mtu` and `ze interface mtu` keep their grammar and gain device-bounds refusal |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | Completion of a host maximum is half (b)'s question (`spec-numeric-bound-authority-declaration.md`) |
| Functional test for new RPC/API | Yes | The six `.ci` files above |
| Pipe completeness | N-A | No command answer changes shape |
| Env var registration | Yes | The `zetest`-only forced-answer variable, registered via `env.MustRegister` only in `zetest` builds |
| Doctor check for runtime dependencies | Yes | `vpp-dpdk-queues` (`internal/component/vpp/doctor_queue_linux.go`) and `iface-mtu-bounds` (`internal/component/iface/doctor_mtu_linux.go`); unreadable ring is `spec-doctor-coverage-15-system` AC-14 |
| Doctor code per problem class | Yes | Over-maximum refusal: `config-host-backend` (`internal/core/diagnostic/codes.go`, via `config-semantics` with `semanticsDoctorCheck().Codes` in `internal/component/config/doctor.go`). Feature-owned via `diagnostic.Register`: `doctor-host-tuning-not-applied` (`internal/plugins/host-tuning/doctor_codes.go`, apply-time ring skip/error `Warn`), `doctor-vpp-dpdk-queues-unreadable` (`vpp/register_doctor_codes.go`, post-config `vpp-dpdk-queues`, silent on `vfio-pci`), `doctor-vpp-dpdk-queues-skipped` (`vpp/register_doctor_codes.go`, boot/reload `writeStartupConf` `Warn`), `doctor-iface-mtu-bounds-unreadable` (`iface/doctor_codes.go`, post-config `iface-mtu-bounds`) |
| Prometheus counters/metrics | N-A | A refusal is a diagnostic |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/<id>.md` for host tuning, VPP and interfaces |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`: accepted MTU values (`68..65535` static, device-bounded), no syntax change |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: `config-host-backend` row and `request interface mtu` refusal outside device bounds |
| 4 | API/RPC added/changed? | No | No RPC changes |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: lists the `host-tuning` participant (`system/tuning`) |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` |
| 7 | Wire format changed? | No | See Changed-Wire Census |
| 8 | Plugin SDK/protocol changed? | No | The SDK is untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the `zetest`-only forced host answer |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: the VyOS case |
| 12 | Internal architecture changed? | Yes | `docs/architecture/host/tuning.md`, `docs/architecture/host/inventory.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/diagnostics/crash-capture.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` for the new doctor checks and codes; `docs/guide/plugins.md` for `host-tuning` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time by `./le spec citation anchors spec plan/release/fixes/spec-config-bound-authority.md`; known anchored pages updated above (`host/tuning.md`, `host/inventory.md`, `hub-architecture.md`, `plugin-system.md`, `yang-config-design.md`, `doctor-and-health-checks.md`, `features/interfaces.md`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/host/tuning.md` "expected, reported, and not retried" gains the numbers and commit-time refusal |

## Design presentation

### What changes

Three host-owned numeric bounds — `system tuning ethtool <if> ring rx/tx`, `vpp dpdk interface <pci> rx-queues/tx-queues`, and `interface <name> mtu` — are validated against the live host limits reported by the driver or kernel (`ETHTOOL_GRINGPARAM`, `ETHTOOL_GCHANNELS`, and `RTM_GETLINK` `IFLA_MIN_MTU`/`IFLA_MAX_MTU`) at `ze config validate`, across every commit surface (SSH editor, web editor, REST, gRPC, HTTP/SSH upload, MCP/artifact commit), and in `ze doctor` under `config-host-backend`. Every refusal names the configured value, the host's reported bound, the interface or PCI device, the hostname and measuring process (`local process` or `daemon`), and ends with the exact `set` fix. Host tuning moves from direct hub calls (`applyHostTuning`/`applyHostTuningFromMap`) into a registered `host-tuning` transaction participant owning `system/tuning`. At boot and reload, optimization bounds (`ring rx/tx` and `rx-queues/tx-queues`) warn and skip over-maximum directions non-fatally under feature-owned doctor codes (`doctor-host-tuning-not-applied` and `doctor-vpp-dpdk-queues-skipped`), while `interface <name> mtu` fails hard because a wrong MTU breaks OSPF and similar protocols. Unreadable hosts refuse nothing; doctor warns under `doctor-vpp-dpdk-queues-unreadable` and `doctor-iface-mtu-bounds-unreadable` for unreadable kernel devices while staying silent for DPDK NICs already bound to `vfio-pci`.

### Before and after

1. **Ethtool ring size above driver maximum**
   - Before: `ze config validate` passes `system tuning ethtool eth0 ring rx 8192`; at boot or reload, `applyHostTuning` logs a bare `ETHTOOL_SRINGPARAM failed for eth0` warning with no maximum and no errno.
   - After: `ze config validate` and every commit surface refuse under `config-host-backend`, reporting configured `8192`, driver max `4096`, interface `eth0`, hostname and process (`local process` or `daemon`), and `set system tuning ethtool eth0 ring rx 4096`. At boot/reload on a lowered driver max, `host-tuning` skips the over-max direction with a non-fatal `Warn` under `doctor-host-tuning-not-applied` naming both numbers, and still applies the other direction if it fits.
2. **VPP DPDK queue count above NIC maximum**
   - Before: `vpp dpdk interface 0000:03:00.0 rx-queues 32` passes validate and commit with no comparison against the NIC, and fails only when VPP rejects the device.
   - After: while the NIC is on its kernel driver, validate and commit refuse under `config-host-backend` naming `32`, the NIC's channel max, `0000:03:00.0`, the netdev, hostname, process, and `set` fix. At boot/reload, `VPPManager.writeStartupConf` (before `bindAll`) skips an over-max direction with a `Warn` under `doctor-vpp-dpdk-queues-skipped` so VPP still starts. In `ze doctor`, NICs already bound to `vfio-pci` stay silent; unreadable kernel-driver NICs warn under `doctor-vpp-dpdk-queues-unreadable`.
3. **Interface MTU bounds and surface agreement**
   - Before: `ze-iface-conf.yang` caps `mtu` at `16000` while `request interface mtu` and `ze interface mtu` allow `65535`; setting `9216` on a `9000`-max NIC passes validate and fails mid-apply with a raw netlink error.
   - After: all surfaces share the static ceiling `68..65535`, and validate, commit, `request interface mtu`, `ze interface mtu`, boot (`OnConfigure`), and reload (`OnConfigVerify`) all check `IFLA_MIN_MTU`/`IFLA_MAX_MTU` through the shared resolver and fail hard with both numbers, logical and OS names, hostname, process, and fix.
4. **Doctor code alignment**
   - Before: `ze config validate` reported host verdicts under `config-host-backend` while `ze doctor` (`ValidateSemantics`) reported the same verifier error under `config-plugin-verify`.
   - After: both surfaces report every host verdict once under `config-host-backend`, and all new `doctor-*` codes register via `diagnostic.Register` from their owning packages.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| `ze:validate` custom validators (the 2026-09-08 design) | Runs inside `LoadConfig` at boot and reload, refusing daemon startup on optimization bounds (contradicting D-1) and preventing post-config doctor checks from running |
| Numeric fields on `kernelcap.Capability` | `kernelcap` models feature presence (`present`/`absent`/`unknown`/`denied`) with no subject or numeric bound and refuses start unconditionally; merging two disjoint shapes into one struct creates a dual-meaning type (`plan/journal/field-carries-two-meanings.md`) |
| Keeping `applyHostTuning` in `cmd/ze/hub/main_system.go` beside a custom verifier hook (R-c) or `ze:validate` (R-b) | R-b breaks `yang-config-design.md`'s single-verdict walk and hides doctor rows; R-c adds a one-off verifier seam beside `InProcessConfigVerifier`. Thomas selected R-a (`host-tuning` participant on `system/tuning`, D-2) |
| Warning in `ze doctor` for DPDK NICs already bound to `vfio-pci` | Every healthy VPP router binds its DPDK NICs to `vfio-pci`, so warning would create permanent false-alarm noise. Thomas ruled silence for `vfio-pci`-bound NICs (D-3) |
| Querying VPP over binary API for queue maximum | Vendored `go.fd.io/govpp v0.13.0` binapi has no `dpdk` package and no `cli_inband` message |

### Documents departed from

- `docs/architecture/host/tuning.md`, section `Consequences and traps`: currently states `"Tuning errors are non-fatal. They are reported, and they do not block a config commit."` After this work, an ethtool ring size above the driver's reported maximum IS refused at validate and commit under `config-host-backend` (blocking the commit before anything is written), while apply-time tuning errors at boot and reload remain non-fatal and warn-and-skip under `doctor-host-tuning-not-applied`; tuning is applied by the `host-tuning` participant rather than direct hub calls.
- `docs/architecture/plugin/plugin-system.md`, section `Registration fields`: currently states for `InProcessConfigVerifier` `"Side-effect-free equivalent of OnConfigVerify, used by static, API and CLI validation"` and for `BootEffect` `"runs every registered effect in plugin name order, once, after host tuning"`. After this work, optimization host-bound checks (`HostVerdict`: rings and DPDK queues) run in `InProcessConfigVerifier` only while live `OnConfigVerify` omits them so boot/reload warn-and-skip (D-1), whereas protocol-correctness bounds (`interface mtu`) run in both; and `BootEffect` no longer references a hub-side host-tuning step because `host-tuning` owns `system/tuning` as a transaction participant (D-2).
- `docs/architecture/hub-architecture.md`, section `Boot effects`: currently states `"After host tuning and before the smart manager starts, the hub runs the boot effect every registered plugin declared (Registration.BootEffect), once, in plugin name order"`. After this work, the hub's `applyHostTuning` and `applyHostTuningFromMap` are deleted and host tuning runs in the `host-tuning` transaction participant (`OnConfigure`/`OnConfigApply`), so `Boot effects` describes `RunBootEffects` running before the smart manager without a preceding direct hub tuning call.
- `docs/architecture/diagnostics/crash-capture.md`, section `The kernel half`: currently states `"the daemon start path runs every registered boot effect once through registry.RunBootEffects, after host tuning and before the smart manager starts."` After this work, direct hub host tuning is removed in favor of the `host-tuning` participant, so `The kernel half` describes `RunBootEffects` running before the smart manager without referencing a preceding direct hub tuning call.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - create `host-tuning` participant in edge plugin `internal/plugins/host-tuning` (`register.go`, `doctor_codes.go`), wire stub checks into `host-tuning`, VPP (`vpp/register.go`), and `iface` (`iface/register.go`), delete `applyHostTuning`/`applyHostTuningFromMap` from `cmd/ze/hub/`, update `allowlist.json` and `plugin/all`
   - Tests: the Wiring Test rows (`TestRingCheckRefusesAboveTheDriverMaximum`, `TestQueueCheckRefusesAboveTheNICMaximum`, `TestMTUCheckRefusesOutsideTheDeviceBounds`, `TestOptimizationLiveVerifyCarriesNoHostRead`, `TestIfaceMTUFailsHardAtBootAndReload`, `TestDoctorReportsAHostVerdictUnderTheHostCode`)
   - Files: `internal/plugins/host-tuning/register.go`, `internal/plugins/host-tuning/doctor_codes.go`, `cmd/ze/hub/main_system.go`, `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go`, `internal/component/config/claims/allowlist.json`, `internal/component/plugin/all/all.go`, `internal/component/vpp/register.go`, `internal/component/iface/register.go`
   - Verify: each wiring test compiles and fails on the stub
2. **Phase: The ring** - read keeps the maxima and errno, `host-tuning` verifier refuses above driver max, `setEthtoolRing` skips over-max directions non-fatally under `doctor-host-tuning-not-applied`, `ValidateSemantics` maps `HostVerdict` to `config-host-backend` and `semanticsDoctorCheck().Codes` declares it, expand `config-host-backend` description
   - Tests: ring, ethtool, tuning, `host-tuning` participant, `ValidateSemantics`/`semanticsDoctorCheck` and code-registration unit tests; `config-validate-ring-*`, `doctor-ring-above-driver-max`
   - Files: `internal/component/host/ethtool_linux.go`, `internal/component/host/tuning_linux.go`, `internal/component/host/tuning.go`, `internal/plugins/host-tuning/ring_check.go`, `internal/plugins/host-tuning/register.go`, `internal/plugins/host-tuning/doctor_codes.go`, `internal/component/config/system/yang/ze-system-conf.yang`, `internal/component/config/validate_semantic.go`, `internal/component/config/doctor.go`, `internal/component/plugin/registry/registry.go`, `internal/core/diagnostic/codes.go`, `docs/architecture/host/tuning.md`, `docs/architecture/host/inventory.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/diagnostics/crash-capture.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/config/yang-config-design.md`, `docs/guide/plugins.md`, `docs/guide/command-reference.md`
   - Verify: refusal prints the driver's number; doctor shows one `config-host-backend` row; boot/reload skips over-max direction with `doctor-host-tuning-not-applied`
3. **Phase: The DPDK queues** - PCI netdev + `ETHTOOL_GCHANNELS` read, VPP in-process verifier check, `vpp-dpdk-queues` doctor check (silent on `vfio-pci`, warns under `doctor-vpp-dpdk-queues-unreadable` otherwise), `writeStartupConf` D-1 skip + `doctor-vpp-dpdk-queues-skipped` warning
   - Tests: queue check, VPP doctor check, `writeStartupConf` skip, and `TestOptimizationLiveVerifyCarriesNoHostRead` unit tests; `vpp-validate-queues-above-nic-max`
   - Files: `internal/component/vpp/queue_check.go`, `internal/component/vpp/doctor_queue_linux.go`, `internal/component/vpp/register.go`, `internal/component/vpp/register_linux.go`, `internal/component/vpp/register_doctor_codes.go`, `internal/component/vpp/vpp.go`, `internal/component/vpp/startupconf.go`, `internal/component/vpp/yang/ze-vpp-conf.yang`, `docs/guide/vpp.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/guide/status.md`
4. **Phase: The MTU** - raw `RTM_GETLINK` bounds read through the shared resolver, `verifyIfaceConfig` + live `OnConfigVerify`/`OnConfigure` fail-hard check (D-1), `iface-mtu-bounds` doctor check (`doctor-iface-mtu-bounds-unreadable`), `handleInterfaceMTU` + `cmdMTU` refusal, widened static `68..65535` declaration
   - Tests: MTU check, boot/reload fail-hard, doctor check, and CLI command unit tests; `config-validate-mtu-device-bounds`
   - Files: `internal/component/iface/mtu_check.go`, `internal/component/iface/doctor_mtu_linux.go`, `internal/component/iface/register.go`, `internal/component/iface/doctor_codes.go`, `internal/component/iface/cmd/manage.go`, `internal/component/iface/cli/link.go`, `internal/component/iface/yang/ze-iface-conf.yang`, `docs/features/interfaces.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/guide/status.md`
5. **Phase: QEMU proof** - prove A-1, A-2, A-3 on real virtio-net and dummy devices (including DPDK bind/unbind for A-2 and D-3)
6. Each documentation and `features/` page edit lands in the phase whose code made it wrong.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-17 has an implementation at file:line |
| Correctness | Every refusal carries the configured value, the host's number, the subject, the hostname and measuring process (`local process` or `daemon`), and ends on its `set` fix |
| Correctness | Unreadable and zero are distinct answers: no probe returns zero, or the type maximum, as a limit (`ai/rules/principles.md`) |
| Data flow | The host is read once per subject per verify; no host ring/queue read in `host-tuning` or VPP live `OnConfigure`/`OnConfigVerify`; `iface` MTU bounds check runs in both `verifyIfaceConfig` and live `OnConfigVerify`/`OnConfigure` (D-1) |
| Data flow | The MTU check resolves logical names through the shared resolver (`./le arch iface-resolution`) |
| Operator errors and doctor codes | Every refusal is an `opererr` error marked `HostVerdict`; every `Warn` line names its registered code (`doctor-host-tuning-not-applied`, `doctor-vpp-dpdk-queues-skipped`); feature-owned `doctor-*` codes register via `diagnostic.Register` in their owning package's `init()`, not in `codes.go` |
| Rule: `ai/rules/no-layering.md` | Hub's `applyHostTuning` and `applyHostTuningFromMap` are deleted, not kept beside the `host-tuning` participant |
| Rule: `ai/rules/stale-comments.md` | The `nolint` reasons, the two queue descriptions, the ring descriptions, the MTU description, and `allowlist.json` are updated so none stays false |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The ring maxima are read | `git grep rxMaxPending internal/component/host/` shows a reader outside the struct |
| Hub direct tuning bypass deleted | `git grep applyHostTuning cmd/ze/` returns no hits |
| Three checks reach validate and commit | the wiring tests (`TestRingCheckRefusesAboveTheDriverMaximum`, `TestQueueCheckRefusesAboveTheNICMaximum`, `TestMTUCheckRefusesOutsideTheDeviceBounds`) |
| D-1 severity split holds | `TestOptimizationLiveVerifyCarriesNoHostRead` and `TestIfaceMTUFailsHardAtBootAndReload` |
| D-3 `vfio-pci` silence holds | `TestVPPQueueDoctorSilentOnVFIOAndWarnsWhenUnreadable` |
| Doctor and validate agree on `config-host-backend` | `TestDoctorReportsAHostVerdictUnderTheHostCode` |
| Feature-owned codes registered via `diagnostic.Register` | `TestEveryNewCodeIsDeclared` across `internal/plugins/host-tuning`, `vpp`, `iface`; `git grep CodeConfigHostBackend internal/core/diagnostic/codes.go internal/component/config/doctor.go` |
| Evidence | scoped runs only (by main thread): `./le test unit pkg path ./internal/component/host/...`, `./internal/plugins/host-tuning/`, `./internal/component/vpp/`, `./internal/component/iface/`, `./internal/component/config/`, `./le config claims`, `./le plugin imports check`, `./le arch iface-resolution`, `./le arch tier check`, the named `.ci` files through `./le test ui` and `./le test vpp`, and `./le test qemu` for the real-device reads |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The subject name from a config path reaches a fixed-length ioctl name field, bounded as `ethtoolIoctl` bounds it; a PCI address is matched against its strict pattern before it builds a sysfs path |
| Fail closed or say something | An unreadable probe returns an explicit unknown, never a zero or a type maximum |
| Resource exhaustion | One probe per subject per verify |
| Error leakage | A refusal names an interface, a PCI address, a hostname, a process role, and two numbers; none is secret |
| Forced answers | The forced-answer variable exists only in a `zetest` build |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A-1, A-2 or A-3 breaks on a real device | STOP; the design of that check changes, report the reading |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The two polarities are one bug: VyOS refused a value the hardware accepted, Ze accepts a value the driver refuses. Both are a bound written where its owner is not.
- The 2026-09-08 design chose `ze:validate` because the extension's text says "runtime-determined valid sets". Read against the doctor page and the config door, that seam runs inside `LoadConfig`, refuses start and reload indiscriminately, and hides the post-config doctor rows. The host-verdict mechanism plus `InProcessConfigVerifier` (paired with live `iface` callbacks for MTU under D-1) gives the exact severity split Thomas ruled.
- The VPP CPU check is the precedent and departs from this spec twice: it refuses an unreadable inventory (as a host verdict), and it runs at start and reload because `VPPSettings.Validate` is called from `parseVPPTransaction`. This spec does not change it; whether those two points are defects is outside this spec's scope.
- Doctor already runs every in-process verifier (`config-semantics`), so a second doctor check for the over-maximum fact would report one fault twice. The owner checks report only what the verifier cannot say: that a readable-expected host device could not be read (staying silent for `vfio-pci`-bound DPDK NICs per D-3).
- The 2026-09-08 read of `internal/component/bgp/reactor/session_connection.go` (`socketRecvBufSize`) found the BGP buffer leaves are not host-bounded; that classification moves with half (b).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Judge ring, DPDK queue, and MTU bounds in their owners' `InProcessConfigVerifier`, marked `HostVerdict` | `ze:validate` (2026-09-08 choice); numeric sibling of `kernelcap` | `ze:validate` runs inside `LoadConfig`; `kernelcap` refuses start unconditionally and carries no subject. `InProcessConfigVerifier` + `HostVerdict` reaches validate, every commit surface, and doctor's `config-semantics` |
| Split boot/reload severity by failure class (D-1): warn-and-skip for rings and DPDK queues; fail hard for `interface mtu` | Uniform no-fail at boot/reload; uniform fail-hard at boot/reload | Buffer/queue sizing is optimization (a router must still boot on a lowered driver/NIC max); wrong MTU breaks OSPF and similar protocols and must fail hard |
| Register `host-tuning` transaction participant in `internal/plugins/host-tuning` owning `system/tuning` (`ConfigRoots` + `WantsConfig`) and delete hub `applyHostTuning`/`applyHostTuningFromMap` (D-2 R-a) | `ze:validate` on ring leaves (R-b); ad-hoc verifier hook in `config` (R-c); placing the participant in `internal/component/host` or `internal/component/host/tuning` | Brings `system/tuning` into the standard config transaction protocol and `InProcessConfigVerifier` seam without layering (`ai/rules/no-layering.md`), avoids the `diagnostic -> host` import cycle, lives under `internal/plugins` (already in `pluginDirs` for `./le repo update`), and passes `./le arch tier check` (`engine-placement` and `non-engine-categories`) |
| Stay silent in `ze doctor` for DPDK NICs bound to `vfio-pci` (D-3) | Permanent warning on every healthy VPP router | Bound to `vfio-pci` is the normal steady state of a running VPP router; warning teaches operators to ignore doctor |
| Register all new `doctor-*` codes via `diagnostic.Register` in their owning packages (`internal/plugins/host-tuning`, `vpp`, `iface`) | Adding feature `doctor-*` codes to `internal/core/diagnostic/codes.go` | `codes.go` holds only `config-*` and cross-cutting codes; feature-owned registration preserves compile-out and plugin ownership (`ai/rules/plugins.md`) |
| Do not extend `kernelcap` | Numeric fields on `Capability` | Its verdict is present, absent, unknown or denied with no subject; a merged struct carries two disjoint field sets (`plan/journal/field-carries-two-meanings.md`) |
| Widen `interface mtu` to `68..65535` here, with one static declaration across leaf, command argument, and handlers | Leave 16000 and defer | The bounds are readable (`IFLA_MAX_MTU`, `IFLA_MIN_MTU` vendored), and three copies disagreeing is part of the defect |
| Report the over-maximum fact once, through `config-semantics` | One doctor check per subsystem for it (2026-09-08) | `ValidateSemantics` already runs every in-process verifier in doctor, so a second check repeats the row; each owner registers only its unreadable-state check |
| The ring's over-maximum row belongs here, the `host-tuning` doctor check to `spec-doctor-coverage-15-system` | A second ring check in this spec | One check, one owner; that spec's AC-14 builds the check and stays silent for a ring above the reported maximum, sharing `doctor-host-tuning-not-applied` |
| Ask the NIC, never VPP, for the queue maximum | A VPP binary API call | The vendored `go.fd.io/govpp v0.13.0` binapi set holds no `dpdk` package and no `cli_inband` message |

## Known Limitations

- VPP is never asked. The queue check reads the NIC through `ETHTOOL_GCHANNELS` while it is on its kernel driver; once VPP holds the device on `vfio-pci` the count cannot be judged, and `ze doctor` stays silent (D-3).
- A device Ze creates (dummy, veth, bridge, tunnel) does not exist at validate time, so its MTU is judged by the kernel at apply (failing hard on refusal per D-1).
- The DPDK descriptor counts VyOS hit (`num-rx-desc`, `num-tx-desc`) are not leaves in Ze; nothing here adds them.

## Work Not Done

| Item | Home |
|------|------|
| The `ze:authority` declaration, the `./le config authority check` gate, the population count, the classification of every numeric leaf, `arp-ignore` (old A-4), old AC-1 to AC-6, old phases 1, 2 and 5, old A-5, A-6, R-2, R-4, R-7, and the declaration decisions | `plan/post-release/engineering/spec-numeric-bound-authority-declaration.md` |
| The `host-tuning` doctor check and its unreadable-ring row | `plan/release/validation/spec-doctor-coverage-15-system.md` AC-14 |

Where the old criteria went: old AC-7, AC-8 are AC-1, AC-2; AC-9 is AC-3;
AC-10 is AC-4; AC-11 and AC-12 are AC-15 (one row through validation, under
`config-host-backend`, instead of a separate doctor check); AC-13, AC-14 are
AC-7, AC-8; AC-15 is AC-9; AC-16 is AC-10; AC-17 is AC-11 (the loopback 65536
story is dropped, because a `uint16` cannot hold 65536); AC-18 is AC-13; AC-19
is AC-11 and AC-12 without `ze:authority`; AC-20 is AC-17. Old R-1, R-3, R-5,
R-6 are R-1, R-3, R-4, R-5 here; R-3 is copied to the post-release spec. Old
A-1, A-2, A-3 are kept.

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. GitHub CI verifies the pushed commit
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
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-bound-authority.md` only, in the same `./le commit create` script
