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
| Updated | 2026-11-24 |

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

The goal: each of the three is judged against the number the host reports, at
`ze config validate` and at commit, with the configured value, the host's number,
the subject and the host that was measured in the message. A host Ze cannot read
refuses nothing and doctor warns. The ring apply stops writing a value above the
maximum it has just read.

## Decisions for Thomas

Open. Nothing here is decided by this spec, and the spec stays `design` until
each row is answered.

| ID | Decision | Options | What each changes for an operator |
|----|----------|---------|-----------------------------------|
| D-1 | Do the host checks run at daemon start and at reload (`ze signal reload`, `request reload` of a hand-edited file)? Raised to Thomas on 2026-11-14, not yet ruled. The main thread's suggestion was: "the host-verdict mechanism does not run at daemon startup. So an over-large ring would be refused at commit and at validate, but a config already on disk would still fail at apply as it does today. I think that is acceptable, since boot should not depend on a driver's answer, but it is a visible choice." Reload is the same question: it runs the live callbacks, not the in-process verifier | A: no. Only validate, the editor and API commits, and doctor judge the bound. B: yes, refuse, as `kernelcap.Refuse` refuses at start and reload. C: start and reload run the check in warning mode: log under the doctor code, skip the over-large value, keep the rest | A: a router keeps booting after a driver or firmware change; a file edited by hand and reloaded meets the apply-time behavior (ring: AC-6 refuses to write and reports both numbers; MTU: the kernel refuses `LinkSetMTU`; queues: VPP fails to start the device). B: a driver update can stop a router from booting until the config is edited, which is the outcome `kernelcap` avoids for its "cannot determine" state. C: like A, plus one log line at start, and a rule per subsystem about what "skip" means (for an MTU, the device keeps its old MTU) |
| D-2 | Which seam carries the ring check? No registration owns the `system` root: the hub applies tuning itself (`cmd/ze/hub/main_system.go` `applyHostTuning`, `applyHostTuningFromMap`), so the in-process verifier the other two checks use has no owner here, and `registry.Register` refuses a registration with no `RunEngine` | R-a: the host component registers a participant owning `system/tuning` (a nested root, as `fib/p4` is), whose in-process verifier carries the ring check and whose `OnConfigure`/`OnConfigApply` apply tuning; the two hub functions are deleted (`ai/rules/no-layering.md`). R-b: `ze:validate` on the two leaves, and `LoadConfig` drops a validator error marked `diagnostic.HostVerdict`. R-c: a registration hook in `internal/component/config` that an owner fills from `init()`, called from `VerifyPluginConfigMapTransition` | R-a: tuning joins the transaction protocol (verify, apply, rollback), which the system door asks of every config change; the largest change, and it moves the tuning Warn lines `spec-doctor-coverage-15-system` AC-15 codes. R-b: the smallest diff, and the editor's completion could offer the driver's maximum through `CompleteFn`; it breaks the documented rule that the walk's three callers "reach one verdict on the same bytes" (`yang-config-design.md`, "Where a validator actually runs"), and doctor would not see the refusal because `LoadConfig` drops it. R-c: a second verifier mechanism beside `InProcessConfigVerifier`, with one user. The spec recommends R-a: it extends the mechanism the other two checks use and departs from no page |
| D-3 | A DPDK device that VPP already holds (bound to `vfio-pci`) has no netdev, so its queue maximum cannot be read. On a running VPP router that is the normal state | Warn on every doctor run, as the required property "an unreadable host refuses nothing and doctor warns" reads; or report no row for a device VPP holds, because its count was judged at commit while it was still on its kernel driver | Warn: every healthy VPP router carries a permanent doctor warning, which teaches the operator to ignore warnings. Silent: a queue count edited into the file after VPP took the device is never judged until VPP refuses it |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - kernelcap's verdict states and callers; which process a config-phase check measures
  → Constraint: "`ze config validate` answers about the host running the command. A config written for another machine and validated on a workstation is judged against the workstation. That is why the gate is not inside `LoadConfig`, which also runs under `ze doctor` and under offline validation." A host read inside the `ze:validate` walk would run in `LoadConfig` at start and reload.
  → Constraint: "`ReadConfig` still runs all load validation; a refusal remains an error and prevents every `post-config` check from running." A host refusal in the loader would hide every post-config doctor row, this spec's own included.
  → Constraint: "A capability that could not be DETERMINED never refuses." The same rule binds an unreadable driver, NIC or device here.
  → Decision: a config phase runs in whichever process answers doctor (the daemon on the daemon route), and its message says which process it measured.
- [ ] `docs/architecture/config/yang-config-design.md`, "Where a validator actually runs" and "Layer 3: Plugin config verifiers"
  → Constraint: the `ze:validate` walk has three callers, `LoadConfig` at daemon start and reload, `ze config validate`, and the SSH editor's `commit`, "so they reach one verdict on the same bytes".
  → Constraint: the in-process verifier "does not run at daemon startup". It runs in `ze config validate`, the SSH editor commit (`cli/validator.go` `ConfigValidator.Validate`), the REST and gRPC commit (`cmd/ze/hub/api.go` `configValidationHook`) and doctor's `config-semantics` check (`config.ValidateSemantics`).
- [ ] `docs/architecture/config/transaction-protocol.md`, "Plugin SDK"
  → Constraint: at reload the live `OnConfigVerify` of every participant runs over its full candidate config (Phase 1), so a host read inside a live verify refuses a reload whose bound did not change.
- [ ] `docs/architecture/host/tuning.md`
  → Constraint: "Tuning errors are non-fatal. They are reported, and they do not block a config commit." and "A ring-size write can fail on a virtual NIC or on a driver with no ring parameter support. Those failures are expected, reported, and not retried." Both stay true at apply; the commit-time refusal is added beside them and the page is edited in the same work.
- [ ] `docs/guide/vpp.md` - the DPDK queue leaves an operator writes
  → Constraint: the page gains the refusal and the unreadable case.

**Key insights:**
- The 2026-09-08 design used `ze:validate`, which runs inside `LoadConfig`. Read against the doors, that seam refuses daemon start and reload when a driver's answer changes, and hides the post-config doctor rows.
- `diagnostic.HostVerdict` (commit 5f65d1f35c) marks a verifier refusal as a fact about this host, and `ze config validate` reports it under `config-host-backend`.
- The live callbacks and the in-process verifier are separate registration fields, so a check placed only in the in-process verifier reaches validate, commit and doctor and never reaches start or reload.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/host/tuning_linux.go` - `applyEthtoolRing` calls `setEthtoolRing`, which reads the whole `ringparam` through `getEthtoolRingParam`, overwrites `rxPending` and `txPending` with the configured values and fires the set. `rxMaxPending` and `txMaxPending` are never read. The four casts carry `nolint:gosec // bounded by YANG range`, which names the static range as the authority. `applyEthtoolRing` turns the error into a non-fatal `TuningError`.
- [ ] `internal/component/host/ethtool_linux.go` - `ringparam` declares the four maxima. `ethtoolRingparam` (the inventory read) returns only `rxPending` and `txPending`. `ethtoolIoctl` "Returns false on any errno", so `getEthtoolRingParam` and `setEthtoolRingParam` build errors that carry neither a number nor the errno.
- [ ] `internal/component/host/tuning.go` - `EthtoolConfig` carries the interface and the two configured sizes; `TuningError` carries operation, subject and error.
- [ ] `cmd/ze/hub/main_system.go` - `applyHostTuning` (start) and `applyHostTuningFromMap` (reload) call `host.ApplyTuning` and log each error as a Warn with no code. No registration owns `system`, so no verifier sees tuning.
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - the `ethtool` list keyed by `interface`, "the kernel interface name" (not a logical name), holding `ring rx` and `ring tx`, each `uint16` with `range "1..65535"`, whose descriptions say "a driver refuses a size it cannot serve".
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - `rx-queues` and `tx-queues` are `uint8` with no range. Their `description` (not their `ze:help`) ends "Ze does not compare the count against the queues the NIC holds."
- [ ] `internal/component/vpp/register.go` - `InProcessConfigVerifier: verifyVPPConfig`, and the live `p.OnConfigVerify(verifyVPPConfig)` is the same function. `OnConfigure` runs `ParseConfigSection` and `VPPSettings.Validate` too, so whatever `Validate` checks runs at plugin start.
- [ ] `internal/component/vpp/cpuset.go` - `CPUSettings.validate` reads `hostCPUInventory` and calls `validateAgainst`. An unreadable online-CPU file is REFUSED, wrapped in `diagnostic.HostVerdict`. It is reached from `VPPSettings.Validate` (`config.go`), so it runs in the in-process verifier, in the live `OnConfigVerify` at reload and in `OnConfigure` at start, not through `ze:validate`. Its test seam is an env-named sysfs root (`cpuRootEnv`).
- [ ] `internal/component/vpp/startupconf.go` - `GenerateStartupConf` takes the inventory as a parameter; the queue counts go into the dpdk dev entry verbatim.
- [ ] `internal/component/vpp/register_linux.go` - registers the hugepage, CPU isolation and DPDK bind doctor checks through `diagnostic.RegisterDoctorCheck`.
- [ ] `internal/component/iface/register.go` - `InProcessConfigVerifier: verifyIfaceConfig`, which calls `parseAndVerifyIfaceSections`. The live `OnConfigVerify` calls `parseAndVerifyIfaceSections` directly, and `OnConfigure` calls neither. So a check added to `verifyIfaceConfig` alone reaches the in-process doors only.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `leaf mtu`, `uint16`, `range "68..16000"`.
- [ ] `internal/component/iface/yang/ze-iface-cmd.yang` - `request interface mtu` `leaf bytes`, `uint16`, `range "68..65535"`.
- [ ] `internal/component/iface/cmd/manage.go` - `handleInterfaceMTU` checks its own constants `MTUMin` 68 and `MTUMax` 65535, a third copy of the bound, then calls `iface.SetMTU`.
- [ ] `internal/core/diagnostic/host_verdict.go` - `HostVerdict` wraps an error; `IsHostVerdict` answers through `errors.As`.
- [ ] `internal/core/diagnostic/codes.go` - `config-host-backend` is described as "No backend this host can provide" (a missing backend or hardware inventory); its text must grow to cover a host bound.
- [ ] `internal/component/config/cli/cmd_validate.go` - `runValidation` reports each `config.VerifyPluginConfig` error under `config-plugin-verify`, or `config-host-backend` when `IsHostVerdict`.
- [ ] `internal/component/config/validate_semantic.go` - `ValidateSemantics`, which doctor's `config-semantics` check runs, reports every `VerifyPluginConfig` error under `CodeConfigPluginVerify`, a host verdict included. So doctor and validate give one refusal two codes today.
- [ ] `internal/component/config/plugin_verify.go` - `VerifyPluginConfigMapTransition` calls every registration's `InProcessConfigVerifier` whose `ConfigSectionRoots` (owned plus read roots) are present; a registration with no `ConfigRoots` is skipped. `ExtractConfigSubtree` takes a nested root such as `system/tuning`.
- [ ] `internal/component/plugin/registry/registry.go` - the `InProcessConfigVerifier` comment calls it the "side-effect-free equivalent of the plugin's OnConfigVerify callback"; `Register` refuses a nil `RunEngine`.
- [ ] `internal/component/config/cli/register.go` - `show config validation` is a `MustRegisterLocalData` command, so SSH, MCP, REST and web reach it through the daemon and it reads the daemon's host; the local offline `ze config validate` reads the caller's machine.
- [ ] `internal/component/kernelcap/kernelcap.go` - `Refuse` names every failing subsystem; the verdict is about whether a feature exists, never a number.

**Behavior to preserve:**
- Ring apply stays non-fatal at apply time (`docs/architecture/host/tuning.md`).
- A host fact Ze cannot read never refuses a commit.
- `test/plugin/config-validate-agrees-with-boot.ci` keeps passing: validate refuses only a config defect or a marked host verdict.
- The VPP CPU check keeps its behavior; it is the precedent, and its differences are recorded under Design Insights.

**Behavior to change:**
- A ring above the driver's reported maximum is refused at validate and at commit, naming the configured value, the maximum, the interface and the host measured.
- The ring apply refuses to write a direction above the maximum it has just read, and every ring error carries the numbers and the errno.
- A DPDK queue count above the NIC's maximum is refused the same way while the device is on its kernel driver.
- An `interface mtu` outside the device's `IFLA_MIN_MTU` to `IFLA_MAX_MTU` is refused the same way; the static ceiling widens from 16000 to 65535, which the command argument already uses.
- Doctor reports a host verdict under `config-host-backend`, the code validate uses.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `ze:validate` and `ValidatorRegistry` | `internal/component/config/yang/validator_registry.go` `CustomValidator`; `validators_register.go` `RegisterValidators` (29 registrations); `ze-extensions.yang` (35 extensions) | A leaf names a Go validator; the walk runs it in `LoadConfig` (start and reload), `ze config validate` and the editor commit | Does not apply, unless D-2 picks R-b: the walk runs in `LoadConfig`, so a host read there refuses start and reload and hides the post-config doctor rows. This was the 2026-09-08 seam |
| In-process config verifier | `registry.Registration.InProcessConfigVerifier`; `config.VerifyPluginConfigMapTransition` | Side-effect-free verify of a registration's roots, run by validate, the SSH editor commit, the REST and gRPC commit and doctor's `config-semantics`, never at start | Extends: the queue check joins VPP's in-process verifier and the MTU check joins `verifyIfaceConfig`, each kept out of the live callbacks; the field comment and Layer 3 of `yang-config-design.md` gain the host-verdict exception. The ring joins it under D-2 R-a |
| Host verdict | `internal/core/diagnostic/host_verdict.go` `HostVerdict`, `IsHostVerdict`; `cmd_validate.go` `runValidation`; `codes.go` `CodeConfigHostBackend` | Marks a refusal as a fact about this host; validate reports it under `config-host-backend` | Extends: every refusal here is marked; `ValidateSemantics` learns the same code mapping so doctor and validate agree; the code's description covers a host bound |
| `kernelcap` | `internal/component/kernelcap/kernelcap.go` `Capability`, `Evaluate`, `Refuse`; `Refuse`'s three callers `cmd_validate.go` `runValidation`, `cmd/ze/hub/main.go` `runYANGConfig`, `cmd/ze/hub/main_reload.go` `runReloadContext` | Present, absent, unknown or denied for a kernel FEATURE, refusing at validate, start and reload | Does not apply: it answers whether a feature exists and carries no subject or number, and its start and reload callers are what D-1 option B would copy. Numeric fields on `Capability` are the field-carries-two-meanings shape |
| Doctor check registry | `internal/core/diagnostic` `RegisterDoctorCheck`; `doctor-and-health-checks.md`, "Where each owner registers its check" | Each owner registers its check from `init()` at a phase | Extends: VPP and iface each register a post-config check for the unreadable state; the over-maximum fact is reported once, through `config-semantics` |
| VPP CPU placement check | `internal/component/vpp/cpuset.go` `CPUSettings.validate`, `validateAgainst`, reached from `VPPSettings.Validate` in `config.go`, which `verifyVPPConfig` (in-process and live verify) and `OnConfigure` call; `doctor_cpu_linux.go` `vppCPUIsolationDoctorCheck` | Refuses a configured core the host does not run, and refuses an unreadable inventory as a host verdict, at validate, commit, reload and start | Extends its shape (the host read passed as an argument so a test states the host); departs from it on two points named in Design Insights: an unreadable read refuses nothing, and the check stays out of start and reload |
| Planned host-tuning doctor check | `plan/release/validation/spec-doctor-coverage-15-system.md` AC-14, `host-tuning`, code `doctor-host-tuning-not-applied` | Not built. Will compare configured tuning with held values, the ethtool read included, warn per mismatch, and warn for a setting the host has no file for | Does not apply as a second check: that spec owns the check and the unreadable-ring warning. This spec owns the over-maximum ring fact, reported once through `config-semantics`; the `host-tuning` row stays silent for a ring above the reported maximum. The reciprocal line is in AC-14 there |
| Ethtool ioctl path | `internal/component/host/ethtool_linux.go` `ethtoolIoctl`, `getEthtoolRingParam` | The one `SIOCETHTOOL` wrapper; bounds the interface name | Extends: the read keeps the maxima, a channel read is added, and the wrapper returns the errno |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator runs `ze config validate <file>` locally, or `show config validation` through the daemon on SSH, MCP, REST or web.
- An operator commits in the SSH or web editor, through the REST or gRPC config API, or uploads a config.
- An operator runs `ze doctor` or `show doctor`.
- The daemon applies tuning at start and at reload.

### Transformation Path
1. The candidate becomes a `config.Tree`; `ToPluginMap` lowers it.
2. `VerifyPluginConfigMapTransition` hands each owner its roots: `vpp`, `interface`, and `system/tuning` under D-2 R-a.
3. The owner's in-process verifier reads the host once per subject: ring and channel counts through the ethtool wrapper, MTU bounds through `RTM_GETLINK` after the logical name is resolved through the shared resolver.
4. A value outside the host's bound becomes an operator error marked `HostVerdict`, carrying the value, the host's number, the subject, the host name, the measuring process and the fix. An unreadable subject produces no error.
5. `runValidation` and `ValidateSemantics` report it under `config-host-backend`; the editors and the API refuse the commit with it.
6. Each owner's post-config doctor check reads the same host and warns for a subject it could not read.
7. At apply, `setEthtoolRing` compares each direction with the maximum it read and writes only what fits.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Verifier ↔ driver | `SIOCETHTOOL` through `ethtoolIoctl` (`ETHTOOL_GRINGPARAM`, `ETHTOOL_GCHANNELS`) | No |
| Verifier ↔ kernel | `RTM_GETLINK`, attributes `IFLA_MIN_MTU`, `IFLA_MAX_MTU` | No |
| Config ↔ owner component | `InProcessConfigVerifier`, by registration | No |
| Doctor registry ↔ owner | `diagnostic.RegisterDoctorCheck` from the owner's `init()` | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`; terms `config-host-backend`, `CodeConfigPluginVerify`, `HostVerdict`, `num-rx-queues`, `ETHTOOL_SRINGPARAM`. Positive control: `runValidation` writes `config-host-backend` and `ValidateSemantics` writes `config-plugin-verify` for the same verifier error. Finding: no wire format changes. The refusal travels in the existing validation result and `diagnostic.Diagnostic` shapes, the doctor JSON keys are unchanged, and `startup.conf`'s `num-rx-queues` line is written exactly as today. What changes is the code VALUE doctor gives a host verdict, from `config-plugin-verify` to `config-host-backend`; its readers are the doctor renderers and any `.ci` expecting the old code, which the implementation greps and converts in the same work.

### Integration Points
- `internal/component/vpp/register.go` - the in-process verifier calls the queue check; the live callbacks do not.
- `internal/component/iface/register.go` - `verifyIfaceConfig` calls the MTU check; `OnConfigVerify` and `OnConfigure` do not.
- The ring check's owner per D-2.
- `internal/component/config/validate_semantic.go` - the host-verdict code mapping.
- `internal/core/diagnostic/codes.go` - the codes below.

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
| A-1 | `ETHTOOL_GRINGPARAM` fills `rxMaxPending` and `txMaxPending` on the drivers Ze runs on, and a zero there means the driver reports no ring support rather than a limit of zero | `ethtool_linux.go` declares the fields; the kernel ethtool ABI defines them | A zero read as a limit refuses every ring on that driver | Unit test over a fake `ringparam`, then a QEMU run on virtio-net, the driver the appliance boots with | unvalidated |
| A-2 | `ETHTOOL_GCHANNELS` reports the maximum receive and transmit queue counts for a NIC still on its kernel driver, and the netdev is found from the PCI address under `/sys/bus/pci/devices/<pci>/net/`; once `DPDKBinder.bindPCI` moves it to `vfio-pci` the read fails cleanly | `internal/component/vpp/dpdk.go` saves the previous driver, so before the bind is the normal validate-time state | The queue check is unreadable in the common case and only warns | QEMU integration test binding and unbinding a device, asserting refusal before the bind and no refusal after | unvalidated |
| A-3 | `IFLA_MAX_MTU` and `IFLA_MIN_MTU` are present in `RTM_GETLINK` replies on Ze's kernel, and a `max_mtu` of zero means the device sets no ceiling of its own rather than a ceiling of zero | `vendor/golang.org/x/sys/unix/ztypes_linux.go` declares both; the vendored `vishvananda/netlink` link attributes do not expose them, so the read is a raw attribute read | The MTU check reads nothing, or refuses every MTU on a device with no ceiling | Unit test over a captured reply plus a QEMU assertion on a real link and on a dummy device; read the kernel's `dev_validate_mtu` in Ze's kernel source for the zero case | unvalidated |
| A-4 | Every commit surface reaches `VerifyPluginConfigMapTransition`: the SSH editor (`ConfigValidator.Validate`), the web editor (`newEditorFactory(zeconfigcmd.ValidateContent)`), REST and gRPC (`configValidationHook`), the config upload (`cmd/ze/hub/config_upload.go`), and MCP's config commit | The callers listed in Current Behavior; `ValidateContent`'s own path not yet read | A surface commits an over-large value, which reload then accepts under D-1 A | Read `ValidateContent` and the MCP commit handler before Phase 2; a `.ci` per surface not already proven | unvalidated |
| A-5 | `os.Hostname` names the measured host well enough for an operator to see a wrong-host verdict | The daemon route runs in the daemon's process; the local route in the caller's | Two hosts with one name read as one | The message also says which process measured (daemon or local), as doctor's process-dependent checks do | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `ze config validate` becomes machine-dependent: the same file validates on one host and not another | A `.ci` that passes locally and fails in CI | That dependence is the point, bounded by the unreadable state. Functional tests force the host answer in a `zetest` build; only QEMU tests touch a real device |
| R-2 | Wrong host: an operator validates a router's file on a workstation with an interface of the same name on another driver, and gets a refusal or a pass about the workstation | A refusal naming the workstation's host name | Ze cannot know which host a file is for. The message names the host and the process that measured, so the operator sees it; validating on the router (`show config validation` over SSH) reads the router. "Unreadable refuses nothing" covers an absent or unreadable device only, never a present device on the wrong machine |
| R-3 | Widening the MTU ceiling to 65535 loses the completion hint the old ceiling gave | Completion offers 65535 on a NIC that takes 9216 | The refusal names the device's real maximum. Offering the host's maximum as completion is half (b)'s question, which is why R-3 is copied there |
| R-4 | A host probe per validate on a config with 200 interfaces issues 200 ioctls | Commit latency on a large config | One read per subject per verify, the result passed to the comparison, as `hostCPUInventory` is read once and passed to `validateAgainst` |
| R-5 | A driver or firmware change lowers a maximum, so an unchanged value is refused at the next commit of anything | A commit of an unrelated change refused | Correct at commit: validate and commit agree, and the message names the new maximum. Doctor reports it first through `config-semantics`. Start and reload are D-1 |
| R-6 | Splitting the in-process verifier from the live verify contradicts the registry comment "side-effect-free equivalent of the plugin's OnConfigVerify callback" | A reviewer reads the split as drift | The exception is named once, in the field comment and in Layer 3 of `yang-config-design.md`: a host verdict is judged at validate and commit only. A test asserts the live verify carries no host read |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config that used to commit is refused on the three subjects only. A wrong probe refuses a legal ring, queue count or MTU and blocks a commit. Nothing on the wire changes and no session drops; under D-1 A start and reload cannot be affected |
| How is it reverted? | Single commit revert per phase |
| Who else touches this path? | `spec-doctor-coverage-15-system` (the `host-tuning` check, the tuning Warn lines), `spec-iface-ethernet-speed-duplex` (a NIC read in iface verify), `spec-yang-single-source-of-truth` (the same YANG leaves) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` over a config setting `ring rx` | → | the ring check through its D-2 seam | `TestRingCheckRefusesAboveTheDriverMaximum` |
| `ze config validate` over `vpp dpdk interface <pci> rx-queues` | → | the queue check in VPP's in-process verifier | `TestQueueCheckRefusesAboveTheNICMaximum` |
| `ze config validate` over `interface <name> mtu` | → | the MTU check in `verifyIfaceConfig` | `TestMTUCheckRefusesOutsideTheDeviceBounds` |
| The live `OnConfigVerify` and `OnConfigure` of VPP and iface | → | no host read | `TestLiveVerifyCarriesNoHostRead` |
| `ze doctor` over an over-large ring | → | `config-semantics` under `config-host-backend` | `TestDoctorReportsAHostVerdictUnderTheHostCode` |
| An operator commits an over-large ring in the editor | → | the whole chain | `test/ui/config-validate-ring-above-driver-max.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `system tuning ethtool eth0 ring rx N`, N above the driver's reported receive maximum | `ze config validate` fails under `config-host-backend`; the message names N, the maximum, `eth0`, the host name and the measuring process, and its last line is the `set` command at the maximum | A ring at or below the maximum validates (AC-3) |
| AC-2 | The same for `ring tx` against the transmit maximum | The same refusal with the transmit maximum | As AC-1 |
| AC-3 | A ring at or below the driver's maximum | Validate passes and the value is applied | Every value the driver reports it takes, because the check never narrows what the driver accepts |
| AC-4 | The ring parameters cannot be read: no such interface, a driver with no ring support, or a reported maximum of zero (A-1) | Validate refuses nothing; the `host-tuning` doctor check of `spec-doctor-coverage-15-system` AC-14 warns naming the interface and the reason | Every value, because Ze has no number to judge against |
| AC-5 | The same over-large ring committed in the SSH editor, the web editor, REST, gRPC or a config upload | The commit is refused with the AC-1 message before anything is written | Commits that leave the ring at or below the maximum |
| AC-6 | Apply (start, reload, or a commit) of a ring above the maximum just read | `setEthtoolRing` writes neither direction above its maximum and returns a `TuningError` naming the configured value, the maximum and the interface; a failed ioctl names the errno. The apply stays non-fatal | A direction inside its maximum is still written, because the two directions are independent |
| AC-7 | `vpp dpdk interface <pci> rx-queues N`, N above the NIC's maximum receive queue count, read while the device is on its kernel driver | Validate fails under `config-host-backend`, naming N, the maximum, the PCI address, the netdev, the host and the fix | A count at or below the maximum validates; an explicit 0 keeps today's meaning |
| AC-8 | The same for `tx-queues` | The same refusal with the transmit maximum | As AC-7 |
| AC-9 | The device is bound to `vfio-pci`, or has no netdev | Validate refuses nothing; doctor behaves as D-3 decides, and the row, when there is one, names the binding as the reason | Every count, because the NIC cannot be asked |
| AC-10 | `interface <name> mtu N` on a device that exists, N above its `IFLA_MAX_MTU` or below its `IFLA_MIN_MTU` | Validate fails under `config-host-backend`, naming N, the bound crossed, the logical name and the OS device name, the host and the fix | An MTU inside the device bounds validates |
| AC-11 | `interface mtu` above 16000 that the device's bounds allow, up to 65535 | Validate passes and the value is applied; the config leaf, the `request interface mtu` argument and `handleInterfaceMTU` take their static bound from one declaration, `68..65535` | Every value the device accepts, because the static range no longer copies a host fact |
| AC-12 | `request interface mtu <name> bytes N` outside the device bounds | The command is refused before the backend call, with the same bounds and wording as AC-10, from the same reader | An MTU inside the bounds is applied as today |
| AC-13 | The device does not exist yet (one ze creates: dummy, veth, bridge, tunnel), is held by VPP, or its reply carries no bound attribute | Validate refuses nothing; for a device that exists and could not be read, doctor warns naming it and the reason | Every value, because there is no device to ask |
| AC-14 | Daemon start, and `ze signal reload` of a hand-edited file, with a value above a bound that changed since the last commit | Start and reload are not refused by this spec's checks: `OnConfigure` and the live `OnConfigVerify` carry no host read. Apply behaves as D-1 decides, and the ring as AC-6 | Everything the daemon started with before this spec |
| AC-15 | Any refusal above, seen through `ze doctor` | `config-semantics` reports it once, under `config-host-backend`, with the same message; the `host-tuning` row is silent for a ring above the reported maximum | Other `config-plugin-verify` refusals keep their code |
| AC-16 | The local offline `ze config validate` on a workstation whose `eth0` sits on another driver | The verdict is the workstation's, and the message says so by host name and "local process"; through the daemon (`show config validation` over SSH) the verdict is the router's and says "daemon" | Nothing is hidden: Ze cannot know which host a file was written for |
| AC-17 | `ze show diagnostic config-host-backend` and each new doctor code | Each code is declared in `internal/core/diagnostic/codes.go` and its description covers the host bound; each check declares the codes it emits | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets a ring larger than the NIC supports and commits | editor → tree → in-process verifier → ethtool read → refusal naming the maximum | `test/ui/config-validate-ring-above-driver-max.ci` |
| 2 | Sets a ring the NIC supports | same path, no refusal, `applyEthtoolRing` writes it | `test/ui/config-validate-ring-at-driver-max.ci` |
| 3 | Sets a DPDK queue count above the NIC maximum before VPP binds the device | validate → VPP in-process verifier → channel read → refusal naming the PCI address | `test/vpp/vpp-validate-queues-above-nic-max.ci` |
| 4 | Runs `ze doctor` with an over-large ring in the file | doctor → `config-semantics` → `config-host-backend` row | `test/ui/doctor-ring-above-driver-max.ci` |
| 5 | Sets an MTU of 9216 on a NIC whose maximum is 9000, then 9000 | validate → iface verifier → `RTM_GETLINK` → refusal, then pass | `test/ui/config-validate-mtu-device-bounds.ci` |
| 6 | Boots a router whose ring now exceeds a lowered driver maximum | start → `applyHostTuning` → AC-6 warning; no start refusal | `TestApplyRingRefusesAboveTheReadMaximum` plus the QEMU run |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRingCheckRefusesAboveTheDriverMaximum` | `internal/component/host/ring_check_test.go` | AC-1, AC-2 against a stated host | |
| `TestRingCheckAcceptsAtTheDriverMaximum` | same | AC-3 | |
| `TestRingCheckRefusesNothingWhenUnreadable` | same | AC-4, including a zero maximum | |
| `TestRingCheckMessageNamesEveryFact` | same | value, maximum, interface, host, process, fix line | |
| `TestRingParamReadKeepsTheMaxima` | `internal/component/host/ethtool_test.go` | the read stops discarding the maxima | |
| `TestApplyRingRefusesAboveTheReadMaximum` | `internal/component/host/tuning_test.go` | AC-6, both directions independent, errno in the error | |
| `TestQueueCheckRefusesAboveTheNICMaximum` | `internal/component/vpp/queue_check_test.go` | AC-7, AC-8 | |
| `TestQueueCheckRefusesNothingWhenDPDKBound` | same | AC-9 | |
| `TestMTUCheckRefusesOutsideTheDeviceBounds` | `internal/component/iface/mtu_check_test.go` | AC-10, both bounds | |
| `TestMTUCheckAcceptsAboveTheOldCeiling` | same | AC-11 | |
| `TestMTUCheckRefusesNothingWithoutTheDevice` | same | AC-13, including a zero `max_mtu` | |
| `TestMTUCommandAndLeafShareOneReader` | same | AC-12 | |
| `TestLiveVerifyCarriesNoHostRead` | `internal/component/vpp/register_test.go`, `internal/component/iface/register_test.go` | AC-14 | |
| `TestDoctorReportsAHostVerdictUnderTheHostCode` | `internal/component/config/validate_semantic_test.go` | AC-15 | |
| `TestEveryNewCodeIsDeclared` | each owner's doctor test | AC-17 | |

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
| `config-validate-ring-above-driver-max` | `test/ui/*.ci` | Story 1, with the host answer forced in the `zetest` build | |
| `config-validate-ring-at-driver-max` | `test/ui/*.ci` | Story 2 | |
| `config-validate-ring-unreadable` | `test/ui/*.ci` | AC-4 | |
| `doctor-ring-above-driver-max` | `test/ui/*.ci` | Story 4 | |
| `vpp-validate-queues-above-nic-max` | `test/vpp/*.ci` | Story 3 | |
| `config-validate-mtu-device-bounds` | `test/ui/*.ci` | Story 5 | |
| QEMU ring, channel and MTU reads | `./le test qemu`, an integration package for `internal/component/host` and `iface` | A-1, A-2, A-3 on a real virtio-net device and a dummy device | |

The forced host answer follows `ze.test.kernelcap.force`: a private variable
registered and read only in a `zetest` build, so a shipped `ze` cannot be told
a false maximum.

### Interop Tests (Scope: protocol)
Not applicable: nothing reaches the wire. This is config validation over host
reads, the case `ai/rules/interop-and-goal-validation.md` exempts.

## Files to Modify
- `internal/component/host/ethtool_linux.go` - the read keeps the maxima, a channel read, the errno in every error
- `internal/component/host/tuning_linux.go` - `setEthtoolRing` compares with the maximum it read; the `nolint` reasons stop naming the YANG range
- `internal/component/host/tuning.go` - the ring check's host reader, passed as an argument
- `cmd/ze/hub/main_system.go` - under D-2 R-a, `applyHostTuning` and `applyHostTuningFromMap` are deleted and the participant applies tuning
- `internal/component/vpp/register.go` - the in-process verifier runs the queue check; the live callbacks keep `verifyVPPConfig`
- `internal/component/vpp/yang/ze-vpp-conf.yang` - `rx-queues` and `tx-queues` descriptions say Ze compares the count with the NIC
- `internal/component/iface/register.go` - `verifyIfaceConfig` runs the MTU check
- `internal/component/iface/cmd/manage.go` - `handleInterfaceMTU` refuses outside the device bounds with the same reader, and its static bound comes from the one declaration
- `internal/component/iface/yang/ze-iface-conf.yang` - `mtu` range `68..65535`, description names the device bounds
- `internal/component/config/system/yang/ze-system-conf.yang` - the ring descriptions say Ze compares the size with the driver's maximum
- `internal/component/config/validate_semantic.go` - host verdicts under `config-host-backend`
- `internal/component/plugin/registry/registry.go` - the `InProcessConfigVerifier` comment names the host-verdict exception
- `internal/core/diagnostic/codes.go` - `config-host-backend` covers a host bound; the doctor codes for the unreadable queue and MTU states
- `docs/architecture/host/tuning.md` - the commit-time refusal beside the apply-time report, and the apply comparison
- `docs/architecture/config/yang-config-design.md` - Layer 3 gains the host-verdict exception
- `docs/architecture/doctor-and-health-checks.md` - the new checks and the one-row rule with `host-tuning`
- `docs/architecture/host/inventory.md` - declared by `ethtool_linux.go`: the read gains the maxima and the channel read
- `docs/guide/vpp.md` - the queue refusal and the unreadable case
- `docs/guide/configuration.md` - the MTU ceiling is the device's
- `features/<id>.md` for host tuning, VPP and interfaces - the changed behavior

**Design documents declared by the files above, and what each owes.** A
`// Design:` header names a page as the authority for a file, so each is named
here whether or not it changes. The implementing phase re-reads each before its
first edit of the declaring file:
- `docs/features/interfaces.md` - declared by the iface files. The MTU bounds an operator can set change, so it is edited.
- `docs/features/ai-first.md` - declared by `host_verdict.go` and `codes.go` for the diagnostic-code contract. The new codes follow that contract; the page is edited only where it describes what `config-host-backend` covers.
- `docs/architecture/hub-architecture.md` - declared by `cmd/ze/hub` files. Unaffected under D-2 R-b or R-c; under R-a, checked for any sentence saying the hub applies tuning.
- `docs/architecture/api/architecture.md` - declared by the plugin registry and SDK-facing files. Unaffected: no event, command, RPC or SDK field is added; under R-a the new participant uses existing callbacks only.
- `docs/architecture/config/syntax.md` - declared by the YANG modules. Unaffected: no syntax changes, only the accepted MTU values.
- `docs/research/vpp-deployment-reference.md` - declared by the VPP files. Unaffected: a research reference that states no Ze behavior this spec changes.

## Files to Create
- `internal/component/host/ring_check.go` - the ring comparison and its message
- `internal/component/vpp/queue_check.go`, `doctor_queue_linux.go` - the queue comparison and the unreadable-state check
- `internal/component/iface/mtu_check.go`, `doctor_mtu_linux.go` - the `RTM_GETLINK` bounds read, the comparison, the unreadable-state check
- the D-2 R-a registration file in `internal/component/host/`, if chosen
- the six `.ci` files above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No node is added |
| YANG validation constraints | Yes | `interface mtu` widens to the command's `68..65535`; the ring range already equals its type; the queue leaves keep `uint8` |
| YANG custom validators | No | Unless D-2 picks R-b |
| CLI commands/flags | No | `request interface mtu` keeps its grammar and gains a refusal |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | Completion of a host maximum is half (b)'s question |
| Functional test for new RPC/API | Yes | The `.ci` files above |
| Pipe completeness | N-A | No command answer changes shape |
| Env var registration | Yes | The `zetest`-only forced-answer variable, registered only in that build |
| Doctor check for runtime dependencies | Yes | The VPP and iface unreadable-state checks; the ring's is `spec-doctor-coverage-15-system` AC-14 |
| Doctor code per problem class | Yes | Over-maximum: `config-host-backend` through `config-semantics`; unreadable queue and MTU: one new code each; unreadable ring: `doctor-host-tuning-not-applied` (AC-14 there); the ring apply's Warn line carries that code too |
| Prometheus counters/metrics | N-A | A refusal is a diagnostic |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/<id>.md` for host tuning, VPP and interfaces |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`: the accepted MTU values, no syntax change |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: `request interface mtu` refuses outside the device bounds |
| 4 | API/RPC added/changed? | No | No RPC changes |
| 5 | Plugin added/changed? | Yes under D-2 R-a | `docs/guide/plugins.md` lists the host tuning participant |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` |
| 7 | Wire format changed? | No | See the census |
| 8 | Plugin SDK/protocol changed? | No | The SDK is untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the forced host answer |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: the VyOS case |
| 12 | Internal architecture changed? | Yes | `yang-config-design.md` Layer 3, `doctor-and-health-checks.md`, `host/tuning.md`, `host/inventory.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` for the doctor checks |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time by `./le spec citation anchors spec plan/release/fixes/spec-config-bound-authority.md`; `host/tuning.md` anchors `tuning_linux.go` and `ethtool_linux.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `host/tuning.md` "expected, reported, and not retried" gains the numbers |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - after D-1 and D-2 are answered: the three checks exist as stubs in their owners' in-process verifiers, and the live callbacks are proven free of them
   - Tests: the Wiring Test rows
   - Files: the D-2 owner, `vpp/register.go`, `iface/register.go`
   - Verify: each wiring test fails on the stub
2. **Phase: The ring** - read keeps the maxima, the check refuses, the apply compares
   - Tests: the ring and ethtool unit tests, `config-validate-ring-*`, `doctor-ring-above-driver-max`
   - Files: `ethtool_linux.go`, `tuning_linux.go`, `ring_check.go`, the D-2 files, `validate_semantic.go`, `codes.go`, `host/tuning.md`, `host/inventory.md`
   - Verify: the refusal prints the driver's number; doctor shows one row
3. **Phase: The DPDK queues** - channel read, check, unreadable state
   - Tests: the queue tests, `vpp-validate-queues-above-nic-max`
   - Files: `queue_check.go`, `doctor_queue_linux.go`, `vpp/register.go`, `ze-vpp-conf.yang`, `docs/guide/vpp.md`
4. **Phase: The MTU** - bounds read through the resolver, check, command refusal, one static declaration
   - Tests: the MTU tests, `config-validate-mtu-device-bounds`
   - Files: `mtu_check.go`, `doctor_mtu_linux.go`, `iface/register.go`, `iface/cmd/manage.go`, `ze-iface-conf.yang`, `docs/guide/configuration.md`
5. **Phase: QEMU proof** - A-1, A-2, A-3 on real devices
6. Each page edit lands in the phase whose code made it wrong.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A refusal carries the value, the host's number, the subject, the host and the measuring process, and ends on its fix |
| Correctness | Unreadable and zero are distinct answers: no probe returns zero, or the type maximum, as a limit (`ai/rules/principles.md`) |
| Data flow | The host is read once per subject per verify; no host read in `OnConfigure` or the live `OnConfigVerify` |
| Data flow | The MTU check resolves logical names through the shared resolver (`./le arch iface-resolution`) |
| Operator errors and doctor codes | Every refusal is an `opererr` error marked `HostVerdict`; every Warn line names its code |
| Rule: `ai/rules/no-layering.md` | Under D-2 R-a the hub's tuning apply is deleted, not kept beside the participant |
| Rule: `ai/rules/stale-comments.md` | The `nolint` reasons, the two queue descriptions, the ring descriptions and the MTU description are each false after the change |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The ring maxima are read | grep for `rxMaxPending` in `internal/component/host/` shows a reader outside the struct |
| Three checks reach validate | the three wiring tests |
| No host read at start or reload | `TestLiveVerifyCarriesNoHostRead` |
| Doctor and validate agree on the code | `TestDoctorReportsAHostVerdictUnderTheHostCode` |
| Codes declared | grep each code in `internal/core/diagnostic/codes.go` |
| Evidence | scoped runs only: `./le test unit pkg path ./internal/component/host/`, `./internal/component/vpp/`, `./internal/component/iface/`, `./internal/component/config/`, the named `.ci` files through `./le test ui` and `./le test vpp`, and `./le test qemu` for the real-device reads |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The subject name from a config path reaches a fixed-length ioctl name field, bounded as `ethtoolIoctl` bounds it; a PCI address is matched against its pattern before it builds a sysfs path |
| Fail closed or say something | An unreadable probe returns an explicit unknown, never a zero or a type maximum |
| Resource exhaustion | One probe per subject per verify |
| Error leakage | A refusal names an interface, a PCI address, a host name and two numbers; none is secret |
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
- The 2026-09-08 design chose `ze:validate` because the extension's text says "runtime-determined valid sets". Read against the doctor page and the config door, that seam runs inside `LoadConfig`, refuses start and reload, and hides the post-config doctor rows. The host-verdict mechanism, which did not exist then, plus the in-process verifier is the seam that reaches validate and commit only.
- The VPP CPU check is the precedent and departs from this spec twice: it refuses an unreadable inventory (as a host verdict), and it runs at start and reload because `VPPSettings.Validate` is called from `OnConfigure` and the live `OnConfigVerify`. This spec does not change it; whether those two points are defects is for the main thread to judge.
- Doctor already runs every in-process verifier (`config-semantics`), so a second doctor check for the over-maximum fact would report one fault twice. The owner checks report only what the verifier cannot say: that the host could not be read.
- The 2026-09-08 read of `internal/component/bgp/reactor/session_connection.go` (`socketRecvBufSize`) found the BGP buffer leaves are not host-bounded; that classification moves with half (b).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Judge the queue and MTU bounds in the owners' in-process verifiers, marked `HostVerdict` | `ze:validate` (the 2026-09-08 choice); a numeric sibling of `kernelcap` | `ze:validate` runs in `LoadConfig`; `kernelcap` refuses at start and reload and carries no subject. The verifier and the marker exist and reach every commit surface |
| Keep the host read out of `OnConfigure` and the live `OnConfigVerify` | One function for both, as `verifyVPPConfig` is today | The live verify runs at every reload over the full config, so a driver change would refuse a reload whose bound did not change. Pending D-1 |
| Do not extend `kernelcap` | Numeric fields on `Capability` | Its verdict is present, absent, unknown or denied with no subject; a merged struct carries two disjoint field sets (`plan/journal/field-carries-two-meanings.md`) |
| Widen `interface mtu` to `68..65535` here, with one static declaration for the leaf, the argument and the handler | Leave 16000 and defer | The bounds are readable (`IFLA_MAX_MTU`, `IFLA_MIN_MTU` vendored), and three copies disagreeing is part of the defect |
| Report the over-maximum fact once, through `config-semantics` | One doctor check per subsystem for it (2026-09-08) | `ValidateSemantics` already runs the verifier in doctor, so a second check repeats the row; each owner registers only its unreadable-state check |
| The ring's over-maximum row belongs here, the `host-tuning` check to `spec-doctor-coverage-15-system` | A second ring check in this spec | One check, one owner; that spec's AC-14 builds the check and stays silent for a ring above the reported maximum |
| Ask the NIC, never VPP, for the queue maximum | A VPP binary API call | The vendored `go.fd.io/govpp v0.13.0` binapi set holds no `dpdk` package and no `cli_inband` message |

## Known Limitations

- VPP is never asked. The queue check reads the NIC through ethtool while it is on its kernel driver; once VPP holds the device the count cannot be judged (D-3).
- A device ze creates (dummy, veth, bridge, tunnel) does not exist at validate time, so its MTU is judged by the kernel at apply.
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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
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
