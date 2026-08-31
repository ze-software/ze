# Spec: kernel-capability-gate

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Amendment, 2026-08-31: the module setup registry now exists

`spec-plugin-registration-result` adds a second startup refusal, and this spec
must be read against it before its first phase runs.

| Fact | Where it lives now |
|------|--------------------|
| What a module's own `init()` achieved when it set itself up | `registry.RecordSetup` / `SetupResults` (`internal/component/plugin/registry/setup.go`), replayed by `show module list` |
| The daemon's refusal on a recorded HARD setup failure | `hardSetupFailure` (`cmd/ze/hub/startup_gate.go`), read at the FIRST statement of `hub.run` |

**They are not two paths to one verdict, and this section states the boundary
so a later reader does not merge them.** The setup record is written before
`main()`, so it cannot see the configuration; this spec's verdict is
config-dependent by construction (`mplsInUse`, `ipsecInUse`), and it probes the
environment at read time. A capability probe therefore cannot move into the
setup registry, and a module's `init()` outcome cannot move into the doctor
registry, which re-probes and keeps no memory of a start.

What this spec MUST do instead, and what its review MUST check:

- Its refusal in `runYANGConfig` keeps the SAME idiom as `hardSetupFailure`'s
  caller: one stderr line, one `logStartupFailure` with its own stage name, and
  `return 1`. A third refusal shape in one function is the parallel path.
- Its refusal names EVERY failing subsystem, not the first, for the reason the
  setup gate names every failing module: an operator who repairs one fault and
  restarts to meet the next pays a whole boot for each fault after the first.
- Its docs land in `docs/architecture/doctor-and-health-checks.md`, whose tier
  table now has three rows. A capability gate is the `ze doctor` tier; it MUST
  NOT be described as a fourth.

## Task

**Symptom.** Ze starts on a host whose kernel lacks a feature the running
configuration needs, and the failure surfaces later as a raw error from a layer
that cannot explain it. An operator who configures IPsec on a kernel with no
XFRM gets a daemon that comes up and does not encrypt. An operator who
configures VPP LCP against a build with no `linux_cp_plugin.so` gets the whole
config apply failing at the binapi layer.

**What exists.** `checkMPLSSupport` (`internal/component/doctor/checks_linux.go`)
already implements the correct rule for ONE subsystem. It reads
`/proc/modules` through `loadedKernelModules`, and it returns nil unless
`mplsInUse(tree)` reports that the config actually uses MPLS forwarding: a
labeled BGP family, LDP, RSVP-TE, or a per-interface MPLS enable. A plain
BGP-over-kernel config is not warned about modules it does not need.

**Three gaps.** The diagnostic is `SeverityWarning`, so nothing acts on it.
Nothing gates startup on diagnostics at all: `diagnostic.Severity`
(`internal/core/diagnostic/types.go`) has exactly two values, `error` and
`warning`, and no caller refuses to run on either. And the pattern is written
once, for MPLS, rather than being a thing a subsystem can join.

**Goal.** A subsystem declares the kernel capability it needs and a predicate
that reports whether the configuration uses it. When the configuration uses the
subsystem and the host lacks the capability, `ze doctor` reports it, `ze`
refuses to start, and `ze config validate` fails. When the configuration does
not use the subsystem, nothing is reported and nothing is blocked: an absent
feature nobody asked for is not a fault.

**Owner decisions, 2026-08-14.** All six were answered by Thomas.

| # | Decision |
|---|----------|
| 1 | Shared gate. MPLS and XFRM enrol now. VPP LCP keeps `spec-fixit-vpp-lcp-reachability` and adopts the registry afterwards |
| 2 | NO operator override. Refusal is absolute. A NOS that half-works on a kernel missing a required feature is the hazard this removes, and an override is what an operator reaches for under pressure |
| 3 | Probing is native. Netlink, procfs, or a syscall. Never an external binary |
| 4 | `CONFIG_MPLS_ROUTING` and `CONFIG_MPLS_IPTUNNEL` are ADDED to the appliance kernel, and MPLS enrols at `SeverityError`. Ze ships LDP, RSVP-TE and labeled-unicast, so an appliance that advertises label forwarding must be able to do it |
| 5 | Ze WRITES `net.mpls.platform_labels` when MPLS is in use. The gate then only checks the sysctl exists. A label space of 0 is a dead config ze can fix rather than report |
| 6 | ONE shared `ipsecInUse` predicate, used by the gate, `checkKernelModules`, and `extractIPsecListeners`. Over-reporting is the same defect in all three |

**The registry already exists.** `RegisterDoctorCheck` and `DoctorCheckContext`
(`internal/core/diagnostic/doctor_registry.go`) are the mechanism, and
`checkOSPFv3IPsec` (`internal/plugins/ospf/doctor_ipsec.go`) is already a
registered check of exactly this shape. `Run` (`internal/component/doctor/doctor.go`)
already sets `ready=false` and returns 1 on any `SeverityError`. This spec adds
two CALLERS and a severity. It does not add a mechanism
(`ai/rules/no-layering.md`).

**Found while researching, and in scope because the gate depends on each.**

| Finding | Why it is this spec's |
|---------|----------------------|
| The appliance kernel carries no `CONFIG_MPLS_*` at all | AC-1 cannot pass on the appliance without it, and decision 4 fixes it |
| `/proc/modules` misreads a built-in kernel as absent | Under a refusal this stops a working router. `checkKernelModules` already carries a comment recording the identical defect for XFRM on an appliance whose XFRM was built in |
| Three XFRM probes exist with two behaviors: `xfrmAvailable` (ospf), `openXFRMNetlink` (doctor), `probeXFRM` (ike) | The gate must be ONE probe (`ai/rules/no-layering.md`), and only `openXFRMNetlink` discriminates |
| `net.mpls.platform_labels` is never written by ze | Decision 5 |

**Probing is native.** The capability is read with netlink, procfs, or a
syscall. Ze MUST NOT shell out to `ip`, `mpls`, or any external binary to learn
what the kernel supports. An external binary is a second dependency that can be
absent for its own reasons, which is the defect this spec removes rather than a
way to detect it.

**Provenance.** Commissioned by Thomas on 2026-08-14. Found while classifying
the fail-open call sites for `plan/future/spec-fail-open-call-site-drain.md`:
three IPsec interop scenarios skip their entire dataplane half with
`if not esp_spis(): log_pass("XFRM/ESP unsupported on this host")`, which cannot
tell a kernel without XFRM from an exec that failed. The test was probing a
capability that the product should report.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (Scope: config)
- [ ] N-A - no RFC governs kernel capability detection

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/doctor/checks_linux.go` - `checkMPLSSupport` returns nil
  unless the tree has `fib { kernel { } }`, THEN unless `mplsInUse(tree)` holds.
  It reads `/proc/modules` via `loadedKernelModules` and wants `mpls_router` and
  `mpls_iptunnel`. A nil module map means "could not read", reported as
  `doctor-mpls-unknown` rather than as absence
  → Constraint: a capability that cannot be DETERMINED is a third state, and it
    must be reported rather than folded into present or absent
  → Constraint: AC-7 must preserve BOTH gates. The `fib { kernel { } }` gate is
    first, so a VPP-FIB config must never be refused for missing MPLS modules
  → Decision: `/proc/modules` is the WRONG probe under a refusal. A kernel with
    `CONFIG_MPLS_ROUTER=y` lists no module, so a built-in kernel reads as absent
    and would refuse a working router. The probe changes as part of enrolling
- [ ] `internal/core/diagnostic/types.go` - `Severity` has two values,
  `SeverityError` and `SeverityWarning`
  → Constraint: no third severity is needed; the gate is a CALLER that acts on
    error, not a new level
- [ ] `internal/core/diagnostic/doctor_registry.go` - `RegisterDoctorCheck` and
  `DoctorCheckContext{Tree any, ConfigDir, Plugins, Store, Platform}` already
  exist. `Tree` is `any` because `internal/le/` is
  shrink-only, so `internal/core` cannot import `internal/component/config`
  → Decision: THE REGISTRY ALREADY EXISTS. This spec adds two callers and a
    severity, not a mechanism (`ai/rules/no-layering.md`)
  → Constraint: a check placed in `internal/core` types its tree as `any` and
    type-asserts. A check under `internal/component` may take the typed tree
- [ ] `internal/plugins/ospf/doctor_ipsec.go`, `doctor_ipsec_linux.go` -
  `checkOSPFv3IPsec` is ALREADY this pattern and is already registered:
  predicate `ospfV6IPsecConfigured`, native probe `xfrmAvailable`
  (`netlink.XfrmPolicyList(netlink.FAMILY_ALL)`), `SeverityWarning`
  → Decision: the native XFRM probe exists and does not need writing
  → Constraint: `xfrmAvailable` returns `err == nil`, so it cannot say WHY.
    The refusal is correct either way (ze that cannot list the SPD cannot
    install SAs), but the MESSAGE must separate "kernel lacks XFRM" from
    "process lacks CAP_NET_ADMIN", or an operator rebuilds a kernel for nothing
- [ ] `internal/component/doctor/doctor.go` - `runChecks` calls
  `checkMPLSSupport` directly rather than through the registry. `Run` already
  sets `ready=false` and returns 1 on any `SeverityError`
  → Constraint: `ai/rules/repo-maintenance.md` bans new direct appends to that
    list, so moving `checkMPLSSupport` into the registry is already required
  → Decision: doctor already ACTS on `SeverityError`. AC-2 needs only the
    severity change, not new doctor wiring
- [ ] `cmd/ze/hub/main.go` - `run` reaches `runYANGConfig`; the tree lands at
  `loadResult.Tree` and is REPLACED by `applyEvolutions` before
  `storage.EnsureActiveVersion`. `Run`, `RunWithManagedClient` both reach it;
  `RunWebOnly` parses no tree
  → Decision: the refusal belongs in `runYANGConfig`, after the evolution
    write-back and before `EnsureActiveVersion`. NOT in `zeconfig.LoadConfig`:
    that also runs under `ze doctor` and under offline validation of a config
    meant for a different host, where a host verdict is wrong
  → Constraint: the hub is `package hub`, not a second binary, so it needs no
    separate gate
- [ ] `cmd/ze/hub/main_reload.go` - `runReload` returns before `ReloadConfig`,
  the provider refresh, and `engine.Reload`; `handleSIGHUPReload` logs and
  keeps the loop running
  → Decision: a reload refusal is safe. The daemon keeps serving the config it
    already has, so R-1's "working deployment turned dead" does not apply here
- [ ] `internal/component/config/cli/cmd_validate.go` - `runValidation` walks
  its own list and never touches the doctor registry
  → Constraint: AC-4 is a third call site and needs new code either way

**Behavior to preserve:**

| Behavior | Where | Why it must not change |
|----------|-------|------------------------|
| A VPP or P4 FIB config is never judged on MPLS kernel support | `checkMPLSSupport`'s `fib { kernel { } }` gate | `fib/kernel` is what activates the plugin that programs labels (`ConfigRoots: []string{"fib/kernel"}`). Another backend does its own MPLS. Losing this gate turns every VPP deployment with MPLS into a refusal (AC-7) |
| A plain BGP-over-kernel config warns about no MPLS module | `mplsInUse` | F15 shipped this. Under a refusal, over-reporting stops a working router (R-3) |
| `ze doctor` exits 1 on any `SeverityError` | `internal/component/doctor/doctor.go` `Run` | Already correct. The gate raises a severity; it must not re-implement the exit |
| A refused reload leaves the running config serving | `cmd/ze/hub/main_reload.go` `runReload` returning before `ReloadConfig` | The whole reason a reload refusal is safe (AC-14) |
| Non-Linux builds compile | `doctor_ipsec_other.go`, `doctor_xfrm_other.go` | Deleting the two duplicate probes must delete both halves of each build pair |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Three, all reading one enrolment: `ze doctor` (`internal/component/doctor/doctor.go`
`Run`), the daemon start (`cmd/ze/hub/main.go` `runYANGConfig`), and
`ze config validate` (`internal/component/config/cli/cmd_validate.go`
`runValidation`). A reload adds a fourth through `main_reload.go` `runReload`.

### Transformation Path
1. The config file is parsed to a `*config.Tree`.
2. `applyEvolutions` may REPLACE that tree, so the gate reads the post-evolution
   tree, never `loadResult.Tree` as first returned.
3. For each enrolled subsystem, its `inUse(tree)` predicate answers whether the
   configuration uses it. A false answer ends the subsystem's evaluation, and
   nothing is probed: an absent feature nobody asked for is not a fault.
4. For each subsystem still live, its native probe returns one of three states:
   present, absent, or cannot-determine. The probe reads procfs or opens a
   netlink socket. It executes no binary.
5. Absent yields a `SeverityError` diagnostic naming the subsystem, the kernel
   capability, and the config leaf that required it. Cannot-determine yields
   `SeverityWarning`. Present yields nothing.
6. `ze doctor` renders the diagnostics and `Run` already exits 1 on any error.
   The start path and validate refuse on the same error. The reload path refuses
   before `ReloadConfig`, so the running config keeps serving.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ capability registry | `inUse(tree)` predicate reads the parsed `config.Tree` | No |
| Ze ↔ kernel | netlink, procfs, or syscall probe. Never an external binary | No |
| Doctor ↔ startup | the same registry answers both, so a refusal and a report cannot disagree | No |

### Integration Points
- `diagnostic.RegisterDoctorCheck` (`internal/core/diagnostic/doctor_registry.go`) - the enrolment mechanism, already built. Each subsystem registers from its owning package.
- `diagnostic.DoctorCheckContext` - carries `Tree any`, type-asserted to `*config.Tree`. The `any` is required: `internal/le/` is shrink-only, so `internal/core` cannot import `internal/component/config`.
- `internal/core/sysctl/known_linux.go` - already registers `net.mpls.platform_labels`, so decision 5 writes through a surface ze owns.
- `internal/appliance/kernelreq.go` `enforceKernelRequirements` - the build-time twin. Naming runtime capabilities after the same `CONFIG_` symbols keeps the two legible against each other.

### Architectural Verification
- **Registration over hardcoding.** Each subsystem registers its own capability from its owning package through `RegisterDoctorCheck`. No core or shared package spells a subsystem name, and `runChecks` loses a direct append rather than gaining one (`ai/rules/plugins.md`, `ai/rules/repo-maintenance.md`).
- **Tier.** The enrolment lives under `internal/component/doctor`, so it may take the typed tree. Nothing new is added to `internal/core` and `./le tier check` sees no new core-to-component pair.
- **One mechanism.** Three XFRM probes collapse to one. Two are deleted, not wrapped (`ai/rules/no-layering.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|--------------------------------|----------|--------------|--------|
| A-1 | `mplsInUse` is the right shape for a general `inUse(tree)` predicate | `checks_linux.go`, shipping for F15 | the registry needs a richer signature | read every subsystem's config shape | confirmed, with one correction: the `fib { kernel { } }` backend condition sits OUTSIDE `mplsInUse` and is load-bearing (AC-7). It folds into the predicate or the registry loses it |
| A-2 | A native XFRM probe is reachable without a privileged syscall | `openXFRMNetlink` (`internal/component/doctor/checks_linux.go`) opens `NETLINK_XFRM` through `netlink.NewHandle`, groups 0, no `CAP_NET_ADMIN` | the probe cannot separate absence from denial | run it unprivileged in QEMU | confirmed. `EPROTONOSUPPORT` is absence; the open also autoloads a modular `xfrm_user`. `probeXFRM` and `xfrmAvailable` dump the SPD instead and DO conflate, so neither may be the gate |
| A-3 | No existing caller refuses to start on a diagnostic | no reference to the doctor package in `cmd/` or `internal/component/engine/`; the only consumers are `doctor/cmd/show.go` and `support/support.go` | the gate extends an existing caller instead of adding one | trace the startup path | confirmed. The startup gate is new wiring |
| A-4 | The kernel autoloads `mpls_iptunnel` on first use, so it must not be gated | `lwtunnel_build_state` issues `request_module("rtnl-lwt-MPLS")`; ze's first use is `buildMPLSEncap` (`internal/plugins/fib/kernel/nexthop_linux.go`) | gating it causes a false refusal on every modular kernel | the QEMU run: configure MPLS with the module unloaded and confirm a labeled route installs | unvalidated, and it is the one probe question QEMU must settle |
| A-5 | `/proc/sys/net/mpls/platform_labels` exists exactly when MPLS routing is in the running kernel | `loadMPLSModules` (`internal/plugins/fib/kernel/mplsentry_integration_linux_test.go`) states the `net.mpls` sysctl tree appears only once MPLS is available, and skips on `os.Stat` of that path | the MPLS probe is wrong in the same direction as `/proc/modules` | QEMU, on a built-in and a modular kernel | unvalidated |
| A-6 | An empty `vpn { ipsec { } }` installs no SA | `ParseIPsecConfig` (`internal/component/ike/ipsec/config.go`) fills `Peers` only from `site-to-site/peer` and remote access from `remote-access` | AC-11 is wrong and the loose predicate was right | a unit test over `ParseIPsecConfig` with an empty block | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | The gate refuses to start on a host where the capability is present but undetectable, turning a working deployment into a dead one | the unknown state appears in doctor output on a normal host | the third state (cannot determine) MUST NOT refuse startup; it warns |
| R-2 | An `inUse` predicate under-reports, so a configured subsystem starts ungated | a subsystem is configured and doctor stays silent | each predicate gets a test driven from a real config that uses the subsystem |
| R-3 | An `inUse` predicate over-reports, so an unrelated config refuses to start | a plain BGP config fails to start | each predicate gets a negative test with a config that does not use the subsystem |

## Blast Radius

**Every ze start on Linux.** The gate runs on the path all three entry points
share, so a defect in a predicate or a probe stops a daemon that works today.
That is why R-1, R-2 and R-3 each carry a required test, and why
cannot-determine warns rather than refusing.

| Surface | Effect |
|---------|--------|
| `ze` daemon start | new refusal path. A false positive is an outage |
| `ze doctor` | MPLS rises from warning to error, so a host that reported a warning now fails readiness and exits 1 |
| `ze config validate` | new failure mode, and it is host-dependent. Validating a config written for another host is the case the gate must NOT judge, which is why it is not in `LoadConfig` |
| Config reload | a refused reload keeps the old config serving, verified at `runReload` |
| OSPFv3 IPsec, IKE | both lose their private XFRM probe and read the shared one |
| IPsec listeners | `extractIPsecListeners` stops opening a listener for an empty `vpn { ipsec { } }` |
| Appliance image | kernel gains two `CONFIG_MPLS_*` symbols, so the image is rebuilt and re-booted |
| Non-Linux builds | must still compile. `capability_other.go` carries the stub |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| `ze doctor` | capability registry → probe → diagnostic | `test/plugin/kernel-capability-doctor-reports.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `ze` daemon start | startup gate reads the registry, refuses on error | `test/plugin/kernel-capability-refuses-start.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `ze config validate` | same registry, same verdict | `test/plugin/kernel-capability-validate-fails.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config that uses a subsystem, on a host with the capability | ze starts, doctor reports nothing for that subsystem |
| AC-2 | A config that uses a subsystem, on a host without the capability | `ze doctor` reports it at `SeverityError`, naming the subsystem, the capability, and the config leaf that requires it |
| AC-3 | The same config and host as AC-2 | `ze` refuses to start, with the same message, and exit code 1 |
| AC-4 | The same config and host as AC-2 | `ze config validate` fails with the same message |
| AC-5 | A config that does NOT use the subsystem, on a host without the capability | ze starts, doctor reports nothing, validate passes |
| AC-6 | A host where the capability cannot be DETERMINED, and a config that uses the subsystem | doctor reports the unknown state at `SeverityWarning`, and ze STARTS |
| AC-7 | An MPLS config on a VPP or P4 FIB backend | never refused for missing MPLS kernel support. The `fib { kernel { } }` condition gates the check BEFORE `mplsInUse`, and both survive the move into the registry |
| AC-8 | An MPLS config on a kernel with `CONFIG_MPLS_ROUTING=y` and no loaded module | the capability reads as PRESENT. `/proc/modules` is not the probe |
| AC-9 | Any enrolled probe, on any host | no external binary is executed. Scoped to the enrolled probes, not process-wide: `checkVPPVersion` execs `vppctl` today and VPP is not enrolled |
| AC-10 | An MPLS config in use, on a kernel whose `net.mpls.platform_labels` is 0 | ze writes a non-zero label space at startup, and MPLS works. Ze does not refuse and does not merely warn |
| AC-11 | A `vpn { ipsec { } }` block with no `site-to-site/peer` and no `remote-access` | IPsec is NOT in use. Nothing is gated, no listener is opened, and `checkKernelModules` does not warn |
| AC-12 | A config with one `site-to-site/peer`, on a host with no XFRM | IPsec IS in use, and AC-2 through AC-4 hold for it |
| AC-13 | An unprivileged `ze doctor` on a host that HAS XFRM | reports presence, not absence. The probe opens `NETLINK_XFRM`, which needs no `CAP_NET_ADMIN`; it does not dump the SPD |
| AC-14 | A reload whose new config uses an ungated subsystem | the reload is refused, the message names the subsystem, and the daemon keeps serving the config it already had |
| AC-15 | The appliance image | boots with MPLS configured, because `CONFIG_MPLS_ROUTING` and `CONFIG_MPLS_IPTUNNEL` are in `runtime.config` and `runtime.require` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures IPsec on a kernel with no XFRM and starts ze | config → `ipsecInUse` → `NETLINK_XFRM` open → `EPROTONOSUPPORT` → `SeverityError` → `runYANGConfig` refuses | `test/plugin/kernel-capability-refuses-start.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 2 | Runs ze with no IPsec block on the same kernel | config → `ipsecInUse` false → nothing probed, nothing reported | `test/plugin/kernel-capability-unused-starts.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 3 | Runs `ze doctor` unprivileged on a host that has XFRM | doctor → probe opens the socket, needs no `CAP_NET_ADMIN` → present | `TestXFRMCapabilityUnprivilegedReportsPresence` |
| 4 | Reloads into a config using a subsystem the host cannot support | SIGHUP → `runReload` → gate refuses before `ReloadConfig` → old config still serving | `TestReloadRefusalKeepsRunningConfig` |
| 5 | Boots the appliance image with an MPLS config | image kernel carries `CONFIG_MPLS_ROUTING` → sysctl exists → ze writes `platform_labels` → labeled route installs | the appliance QEMU boot |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMPLSCapabilityReadsBuiltInKernel` | `internal/component/doctor/capability_linux_test.go` | AC-8: a sysctl present with no loaded module reads as PRESENT | [ ] | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| `TestMPLSCapabilityAbsentIsENOENT` | same | absence is `ENOENT`, distinct from a read error | [ ] |
| `TestMPLSCapabilityUnreadableIsUnknown` | same | AC-6: any other read error is cannot-determine | [ ] |
| `TestMPLSCapabilityNotGatedOnVPPBackend` | `internal/component/doctor/capability_test.go` | AC-7: `fib { kernel { } }` gates before `mplsInUse` | [ ] | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| `TestMPLSPlatformLabelsWritten` | `internal/plugins/fib/kernel/` | AC-10: ze writes a non-zero label space when MPLS is in use | [ ] |
| `TestXFRMCapabilityUnprivilegedReportsPresence` | `internal/component/doctor/capability_linux_test.go` | AC-13: the probe needs no `CAP_NET_ADMIN` | [ ] | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| `TestXFRMCapabilityAbsentIsEPROTONOSUPPORT` | same | absence is that errno, and `EPERM` is not absence | [ ] |
| `TestIPsecNotInUseForEmptyBlock` | `internal/component/doctor/capability_test.go` | AC-11 and A-6 | [ ] | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| `TestIPsecInUseForOneSiteToSitePeer` | same | AC-12, the positive case (R-2) | [ ] |
| `TestIPsecInUseForRemoteAccess` | same | the second positive shape | [ ] |
| `TestCapabilityProbeExecsNoBinary` | same | AC-9, scoped to the enrolled probes | [ ] |
| `TestReloadRefusalKeepsRunningConfig` | `cmd/ze/hub/main_reload_test.go` | AC-14 | [ ] |

### Test Discrimination (BLOCKING, `ai/rules/interop-and-goal-validation.md`)

Every test above must FAIL when its mechanism is reverted, and the mutation must
be measured rather than asserted. Two are at particular risk of being vacuous:
`TestCapabilityProbeExecsNoBinary` must fail if a probe is changed to exec, not
merely pass because nothing execs today; and `TestIPsecNotInUseForEmptyBlock`
asserts an ABSENCE, so it must be paired with a positive control that goes red
when the predicate is stubbed to return false.

### Boundary Tests (numeric inputs)
| Test | Input | Expected |
|------|-------|----------|
| N-A | - | no numeric input |

### Functional Tests
| Test | File | Validates |
|------|------|-----------|
| doctor reports a missing capability for a configured subsystem | `test/plugin/kernel-capability-doctor-reports.ci` | AC-2 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| ze refuses to start, non-zero exit | `test/plugin/kernel-capability-refuses-start.ci` | AC-3 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| config validate fails with the same message | `test/plugin/kernel-capability-validate-fails.ci` | AC-4 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| an unconfigured subsystem gates nothing | `test/plugin/kernel-capability-unused-starts.ci` | AC-5 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| cannot-determine warns and still starts | `test/plugin/kernel-capability-unknown-starts.ci` | AC-6 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->

### Interop Tests (Scope: config)
| Test | Peer | Validates |
|------|------|-----------|
| N-A | - | No wire protocol changes. The gate decides whether ze starts, and no peer observes it. `ai/rules/interop-and-goal-validation.md` exempts a config-only feature with no protocol impact |

### QEMU Tests (BLOCKING, `ai/rules/platform-linux.md`)
| Test | Validates |
|------|-----------|
| `./le qemu run command "./le qemu all-tests"` over the five `.ci` | AC-1 through AC-6 against a real kernel |
| MPLS built-in versus modular | A-5, and AC-8's whole point |
| MPLS configured with `mpls_iptunnel` unloaded | A-4: the kernel autoloads it, so it must not be gated |
| Unprivileged `ze doctor` on a kernel with XFRM | AC-13 |
| Appliance image boot with an MPLS config | AC-15 |

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/doctor/checks_linux.go` | `checkMPLSSupport` moves to the registry, gains the sysctl probe, keeps both existing conditions, and rises to `SeverityError`. `checkKernelModules` drops its MPLS rows and adopts the shared `ipsecInUse` |
| `internal/component/doctor/doctor.go` | `runChecks` drops the direct `checkMPLSSupport` append (`ai/rules/repo-maintenance.md` bans new ones and this removes one) |
| `internal/core/diagnostic/codes.go` | new codes for the refusal and the unknown state, per subsystem |
| `internal/plugins/ospf/doctor_ipsec_linux.go`, `doctor_ipsec_other.go` | `xfrmAvailable` is deleted; the plugin consults the one registry probe |
| `internal/component/ike/engine/doctor_xfrm_linux.go`, `doctor_xfrm_other.go` | `probeXFRM` is deleted for the same reason. Its SPD dump conflates absence with denial |
| `internal/component/doctor/checks_listener.go` | `extractIPsecListeners` adopts the shared `ipsecInUse` (decision 6) |
| `cmd/ze/hub/main.go` | `runYANGConfig` refuses after `applyEvolutions` and before `storage.EnsureActiveVersion` |
| `cmd/ze/hub/main_reload.go` | `runReload` calls the gate explicitly, so a refused reload leaves the running config serving |
| `internal/component/config/cli/cmd_validate.go` | `runValidation` consults the registry |
| `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` | add `CONFIG_MPLS_ROUTING` and `CONFIG_MPLS_IPTUNNEL` (decision 4) |
| `internal/plugins/fib/kernel/` | write `net.mpls.platform_labels` when MPLS is in use (decision 5) |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/doctor/capability.go` | the enrolment: subsystem name, capability name, probe, `inUse` predicate, and the three-state result | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| `internal/component/doctor/capability_linux.go` | the two native probes: the MPLS sysctl read and the `NETLINK_XFRM` open, each classifying its errno | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| `internal/component/doctor/capability_other.go` | the non-Linux build, so the gate compiles on darwin | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
| `test/plugin/kernel-capability-*.ci` | the five functional tests named in the Wiring Test table |

### Integration Checklist

| Surface | Answer |
|---------|--------|
| YANG schema, validation, custom validators | N-A. No override leaf exists by decision 2. A `ze:validate` validator runs at parse time on any machine, so it is the wrong mechanism for a host-dependent verdict |
| CLI commands, flags, grammar, completion | N-A. `ze doctor`, `ze config validate` and `ze` start keep their surfaces; only their verdicts change |
| Functional test for new behavior | Yes. The five `test/plugin/kernel-capability-*.ci`; `test/plugin/mpls-doctor.ci` is the precedent for the hermetic half |
| Env var registration | N-A. No new env var. The probes reuse `ze.test.doctor.procfs-root` and `ze.test.doctor.modules-file` |
| Doctor check for runtime dependencies | Yes. `diagnostic.RegisterDoctorCheck` from each owning package, codes in `internal/core/diagnostic/codes.go`, unit plus functional test |
| Prometheus metrics | N-A. No new counter. A refusal is a startup failure, not a runtime rate |
| BGP family surface | N-A. No SAFI, capability, or attribute |
| Appliance kernel requirements | Yes. `gokrazy/kernel/runtime.config` and `runtime.require`, enforced by `enforceKernelRequirements` (`internal/appliance/kernelreq.go`) |

### Documentation Update Checklist (BLOCKING)

| Category | Answer |
|----------|--------|
| 1. New user-facing feature | Yes. `docs/features.md` |
| 6. User guide page | Yes. `docs/guide/operations.md`: the start refusal, its exit code, and what to do about each capability |
| 12. Internal architecture | Yes. `docs/architecture/doctor-and-health-checks.md`: the enrolment, the three states, the three callers |
| 16. Source anchors on changed files | Yes. Grep `docs/` for `checks_linux.go`, `doctor.go`, `cmd_validate.go`, `cmd/ze/hub/main.go` and update each claim |
| 17. Stale examples in this area | Yes. Verify existing `ze doctor` output examples against the new codes and severities |
| 2, 3, 4, 5, 7, 8, 9, 10, 11, 13, 14, 15 | N-A. No config syntax, CLI surface, API, plugin SDK, wire format, RFC, comparison, meta key, or registered-capability change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the enrolment exists and one subsystem joins
   - Tests: the three `.ci` in the Wiring Test table, failing because the gate is a stub
   - Files: `capability.go`, `capability_linux.go`, `capability_other.go`, `codes.go`
   - Verify: `ze doctor`, `ze` start, and `ze config validate` each reach the stub
2. **Phase: The MPLS probe is corrected** -- AC-8, AC-7, AC-10
   - **FIRST TASK, BLOCKING (owner decision 2026-08-14): prove A-5 in QEMU before
     writing any probe or refusal code.** Boot a kernel with `CONFIG_MPLS_ROUTING=y`
     and one with `mpls_router` as an unloaded module. Record for each whether
     `/proc/sys/net/mpls/platform_labels` exists and what it reads. Settle A-4 in
     the same run: configure MPLS with `mpls_iptunnel` unloaded and confirm a
     labeled route installs. **If A-5 is false, STOP and report. The probe needs
     redesign and nothing downstream may be built on it.** This is first because
     a wrong probe here fails exactly as `/proc/modules` does, and this time it
     refuses startup rather than warning
   - Tests: `TestMPLSCapabilityReadsBuiltInKernel`, `TestMPLSCapabilityNotGatedOnVPPBackend`, `TestMPLSPlatformLabelsWritten`
   - Files: `checks_linux.go`, `internal/plugins/fib/kernel/`
   - What blocks it: `/proc/modules` misreads a built-in kernel. The sysctl probe replaces it, and `mpls_iptunnel` is NOT gated (A-4)
   - Verify: hermetic via `ze.test.doctor.procfs-root`; the built-in and modular cases in QEMU, recorded as A-5's and A-4's validation evidence
3. **Phase: One XFRM probe** -- AC-13, and the deletion of two copies
   - Tests: `TestXFRMCapabilityUnprivilegedReportsPresence`, `TestXFRMCapabilityAbsentIsEPROTONOSUPPORT`
   - Files: `capability_linux.go`, `ospf/doctor_ipsec_*.go`, `ike/engine/doctor_xfrm_*.go`
   - What blocks it: `probeXFRM` and `xfrmAvailable` dump the SPD, which needs `CAP_NET_ADMIN`. Both are deleted, not wrapped (`ai/rules/no-layering.md`)
   - Verify: run unprivileged in QEMU and confirm presence is reported
4. **Phase: One `ipsecInUse`** -- AC-11, AC-12
   - Tests: `TestIPsecNotInUseForEmptyBlock`, `TestIPsecInUseForOneSiteToSitePeer`, `TestIPsecInUseForRemoteAccess`
   - Files: the shared predicate, `checks_linux.go`, `checks_listener.go`
   - Verify: all three call sites agree; no listener opens for an empty block
5. **Phase: The three callers refuse** -- AC-2, AC-3, AC-4, AC-6, AC-14
   - Tests: the five `.ci`, plus `TestReloadRefusalKeepsRunningConfig`
   - Files: `main.go`, `main_reload.go`, `cmd_validate.go`, `doctor.go`
   - Verify: exit code 1 and the same message from all three surfaces
6. **Phase: The appliance kernel carries MPLS** -- AC-15
   - Tests: the appliance QEMU boot with an MPLS config
   - Files: `gokrazy/kernel/runtime.config`, `runtime.require`
   - Verify: the image boots and installs a labeled route

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Fail closed, and say so | A capability that cannot be determined never reads as present, and never silently refuses either (`ai/rules/evidence.md`) |
| No external binary | The probe uses netlink, procfs, or a syscall. Nothing execs `ip`, `mpls`, or any binary |
| Predicate symmetry | Every enrolled subsystem has BOTH a positive and a negative predicate test (R-2, R-3) |
| One mechanism | The registry is the only capability gate. `checkMPLSSupport` joins it rather than sitting beside it (`ai/rules/no-layering.md`) |
| No override | No config leaf, env var, or flag starts ze past a refusal (owner decision) |
| Preserved behavior | The `fib { kernel { } }` condition still gates MPLS before `mplsInUse` (AC-7) |
| Registration over hardcoding | Each subsystem registers its capability from its OWNING package through `diagnostic.RegisterDoctorCheck`. No core or shared package spells a subsystem name, and `runChecks` loses a direct append rather than gaining one (`ai/rules/plugins.md`) |

### End-to-End User Stories (filled)

The five rows in the End-to-End User Stories table above are the filled set.
Each names its path and the test that proves it.

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The enrolment, with MPLS and XFRM registered | `grep -rn "RegisterDoctorCheck" internal/component/doctor internal/plugins/fib internal/component/ike` names both |
| `checkMPLSSupport` no longer directly appended in `runChecks` | `grep -n "checkMPLSSupport" internal/component/doctor/doctor.go` returns nothing |
| Exactly ONE XFRM probe in the tree | `grep -rn "func probeXFRM\|func xfrmAvailable\|func openXFRMNetlink" internal/` returns one Linux implementation and its non-Linux twin |
| One shared `ipsecInUse`, three callers | `gopls references` on the predicate names the gate, `checkKernelModules`, and `extractIPsecListeners` |
| Appliance kernel carries MPLS | `grep -n "CONFIG_MPLS" gokrazy/kernel/runtime.config gokrazy/kernel/runtime.require` returns both symbols |
| The five functional tests exist and pass | `./le functional plugin` names each `kernel-capability-*` test |
| Every probe's three states are reachable in test | each `.ci` or unit test drives present, absent, and cannot-determine |
| Diagnostic codes registered | `go test ./internal/component/doctor -run TestDoctorCoverageCodesRegistered` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Probe privilege | A probe that needs privilege must not report "absent" when it means "not permitted" (A-2) |
| Denial of service | A refusal path that an attacker can trigger by influencing config would stop the daemon; check who can write config |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The rule already exists and is proven for one subsystem. This spec is mostly
  about giving it a second caller and a place for a third subsystem to join,
  not about inventing a mechanism.

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| No operator override | Owner decision 2026-08-14. An override is what operators reach for under pressure, and it reinstates the hazard |
| Cannot-determine warns, never refuses | R-1. Refusing on an unreadable probe turns a working deployment into a dead one |

## Known Limitations

| Limitation | Why it is accepted |
|------------|-------------------|
| ESP has no unprivileged runtime probe | `CONFIG_XFRM_USER` gives the netlink interface and `CONFIG_INET_ESP` carries the packets. A kernel with the first and not the second accepts every SA install and drops every ESP packet (`internal/appliance/kernelreq.go`). Only the build-time requirement catches it, so a non-appliance kernel can still pass the gate and fail to encrypt |
| The gate judges the LOCAL host | `ze config validate` run on a workstation, for a config destined elsewhere, answers about the workstation. This is why the gate is not in `LoadConfig`, but validate still carries it (AC-4), so the limitation is real and must be documented |
| Only MPLS and XFRM enrol | L2TP, PPPoE, nftables, policy routing, traffic shaping, eBPF/TCX and several interface kinds all need kernel features that can be absent, and all are module-gated or unprobed today. Each joins the enrolment later. The registry's shape must not assume two subsystems |
| `mpls_iptunnel` is deliberately not gated | The kernel autoloads it on the first encap route (A-4). Gating it would be a false refusal on every modular kernel |

## RFC Documentation (Scope: config)

N-A. No RFC governs kernel capability detection.

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le verify worktree` green, or scoped evidence with attribution

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Review Gate 0 BLOCKER / 0 ISSUE

## Review Gate

### Run 1
| Severity | Finding | Location | Fixed by |
|----------|---------|----------|----------|
| | | | |
