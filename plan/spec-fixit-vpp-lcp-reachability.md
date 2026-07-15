# spec-fixit-vpp-lcp-reachability

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-15 |

**SKELETON.** This spec tracks known-but-unstarted work. It is NOT ready to implement.
Run `/ze-spec` first: the Open Questions below must be answered before any design. Test
names, files, and steps listed here are CANDIDATES from a read of the current code, not a
settled design.

## Task

VPP's Linux Control Plane (LCP) creates TAP mirrors of VPP interfaces so routing daemons can
use Linux TCP on VPP-managed NICs. Two gaps stop that from working end to end. First, the
TAPs land in the namespace named by `vpp.lcp.netns` (default `"dataplane"`), but ze's BGP
listener has zero network-namespace awareness, so it cannot bind on them unless the operator
moves LCP to a root-reachable namespace. Second, when the VPP build lacks
`linux_cp_plugin.so`, nothing warns pre-apply: the whole config apply fails at the binapi
layer with a raw VPP error. Goal: VPP LCP interfaces are actually usable by BGP, and LCP
misconfiguration is diagnosed by `ze doctor` before apply.

Group rationale: both are about making VPP LCP interfaces reachable and diagnosable. They
are grouped for tracking, not because they share a root cause. Problem A is a BGP listener
capability; Problem B is a doctor check. DESIGN may split them.

## Origin

Both from `plan/deferrals.md` rows dated 2026-07-10, source `spec-followup-vpp-iface`:
- Problem A: the "spec-followup-vpp-iface A-4" row (BGP netns-aware listener). Destination
  recorded as "none yet (future `spec-bgp-netns` when picked up)".
- Problem B: the "spec-followup-vpp-iface" row (doctor check for VPP linux-cp plugin
  presence). Destination recorded as "none yet (future doctor follow-up)".

## Required Reading

### Source (read before designing)

- [ ] `internal/component/bgp/reactor/listener.go` - lines 32-46: the `Listener` struct holds
      `addr string` and `listenerFactory network.ListenerFactory`, nothing namespace-related
  → Constraint: CONFIRMED no netns awareness. A grep for netns/namespace across
    `internal/component/bgp/reactor/` returns only EVENT-namespace hits (`bgpevents.Namespace`),
    never a network namespace. The deferrals row's claim holds.
- [ ] `internal/component/bgp/reactor/listener.go` - line 108: `StartWithContext` calls
      `l.listenerFactory.Listen(ctx, "tcp", l.addr)`; `SetListenerFactory` at lines 66-68
  → Decision: an injection seam ALREADY exists. A netns-aware listener is plausibly a new
    `ListenerFactory` implementation rather than surgery on `Listener` itself.
- [ ] `internal/core/network/network.go` - lines 32-40: the `ListenerFactory` interface;
      `RealListenerFactory` at lines 147-149 with `Listen` at line 167 using `net.ListenConfig`
  → Constraint: this is the seam every listener goes through. A netns variant belongs here
    or beside it, not inline in the reactor.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 100-131: `checkVPPLCPNetns`, the
      DELIVERED netns doctor check (code `doctor-vpp-lcp-netns`, `SeverityWarning` at line 128)
  → Constraint: this check is the "meanwhile" mitigation named by the deferrals row. If
    Problem A is fixed, this check's premise changes and it must be revisited, not left.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 136-143: `lcpNetnsIsRootReachable`
      treats only "", "host", "root" as root-reachable
  → Constraint: the YANG default `"dataplane"` is NOT in this set, so the default config
    warns. Confirm this is intended before designing.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 62-85: `checkVPPWireguardPlugin`, the
      check Problem B is told to parallel; `wireguardPluginEnabled` at lines 88-95
  → Constraint: it is a CONFIG-ONLY check (reads the config tree for the
    `vpp.plugins.wireguard` toggle). It never probes the running VPP. The LCP case has no
    equivalent toggle, so "parallel to doctor-vpp-wireguard" is true in INTENT only, not in
    mechanism.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 27-56: `registerDoctorChecks`, both
      checks at `DoctorPhasePostConfig`, Order 740 and 741, Component "vpp"
  → Constraint: a third check follows this registration shape and travels with the plugin
    (`ai/rules/doctor-checks.md`).
- [ ] `internal/component/vpp/startupconf.go` - lines 82-91: `linux_cp_plugin.so` and
      `linux_nl_plugin.so` are enabled in startup.conf whenever `s.LCP.Enabled`
  → Constraint: ze already ASKS for the plugin. The gap is a build that lacks the `.so`,
    which no config-level check can see. This is why a runtime probe is required.
- [ ] `internal/plugins/iface/vpp/lcp.go` - lines 87-102: `lcpItfPair` issues
      `LcpItfPairAddDel`; failure surfaces at line 97 (error) or line 100 (retval)
  → Constraint: this is the "binapi layer" where the whole apply fails today. The doctor
    check must fire BEFORE this.
- [ ] `internal/plugins/iface/vpp/lcp.go` - lines 105-115: `lcpPairNetns` maps a
      root-reachable name to "" (VPP's host namespace) and passes any other name through
  → Decision: the existing product already bends toward "put the TAP where BGP can reach
    it". Problem A is the other half of that same decision.
- [ ] `internal/component/doctor/checks_linux.go` - line 251: `checkVPPVersion` runs
      `vppctl show version` (line 266) with a 3s timeout
  → Decision: a runtime VPP probe precedent EXISTS, but via `vppctl` exec from the CENTRAL
    doctor package, not GoVPP from the owning plugin. The deferrals row asks for GoVPP.
    That tension is a real design question, not a detail.
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - lines 171-205: the `lcp` container;
      `enabled` defaults true (lines 176-181), `netns` defaults "dataplane" (lines 198-205)
  → Constraint: LCP is ON by default and its netns default is NOT root-reachable. Any check
    keyed on `lcp.enabled` fires for essentially every VPP deployment.

### Architecture Docs

- [ ] `ai/rules/doctor-checks.md` - cited by `doctor.go:1` as the design rule for
      self-contained checks owned by the plugin that owns the runtime dependency
  → Constraint: the new check belongs in `internal/plugins/iface/vpp/`, and a new code must
    be registered in `internal/core/diagnostic/codes.go`.
- [ ] `plan/learned/1098-followup-vpp-iface.md` - the learned summary of the spec both rows
      came from
  → Decision: read for why A-4 was deferred rather than solved.
- [ ] `docs/guide/vpp.md` - the operator-facing VPP guide (mentions the doctor codes)
  → Constraint: update if the netns constraint or the diagnosis changes.
- [ ] `ai/rules/qemu-testing.md` - linux-only code needs QEMU integration tests
  → Constraint: a netns-binding listener is linux-only and MUST be QEMU-tested; "needs
    hardware" is not an accepted skip.
- [ ] `ai/rules/plugin-self-containment.md` - constrains where the check and any netns code live
  → Constraint: no plugin spelling in generic packages.

**Key insights:**
- The BGP listener already has a `ListenerFactory` injection seam; netns support likely
  belongs there, not in the reactor.
- The existing `doctor-vpp-wireguard` check is config-only; the LCP plugin-presence check
  cannot copy it, because LCP has no config toggle to read. It needs a runtime probe.
- The only runtime VPP probe precedent uses `vppctl` exec from the central doctor package,
  which conflicts with both the "GoVPP probe" phrasing and plugin self-containment.
- `vpp.lcp.netns` defaults to "dataplane", which is NOT root-reachable, so the shipped
  default already trips the netns warning.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/bgp/reactor/listener.go` - lines 50-56: `NewListener(addr)` stores
      the address and a `network.RealListenerFactory{}`. Line 108: `StartWithContext` calls
      `l.listenerFactory.Listen(ctx, "tcp", l.addr)`. Nothing anywhere in the file, or in
      `internal/component/bgp/reactor/`, references a network namespace.
- [ ] `internal/core/network/network.go` - lines 147-167: `RealListenerFactory.Listen` binds
      via `net.ListenConfig` in whatever namespace the calling thread happens to be in.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 100-131: `checkVPPLCPNetns` warns when
      `bgp` is configured, `vpp/lcp` exists, `enabled` is not "false", and the netns is not
      root-reachable. Emits `doctor-vpp-lcp-netns` at `SeverityWarning`.
- [ ] `internal/plugins/iface/vpp/doctor.go` - lines 62-85: `checkVPPWireguardPlugin` emits
      `doctor-vpp-wireguard` at `SeverityError` when a wireguard interface is configured
      under backend vpp but the `vpp.plugins.wireguard` toggle is off. Config-only.
- [ ] `internal/plugins/iface/vpp/lcp.go` - lines 87-102: `lcpItfPair` sends
      `LcpItfPairAddDel` and returns a raw error (line 97) or a retval error (line 100) when
      the plugin is not loaded in VPP. Nothing catches this earlier.
- [ ] `internal/component/vpp/startupconf.go` - lines 82-91: startup.conf enables
      `linux_cp_plugin.so` and `linux_nl_plugin.so` whenever LCP is enabled, on top of
      `plugin default { disable }`.
- [ ] `internal/component/doctor/checks_linux.go` - line 251: `checkVPPVersion` shows the
      only runtime VPP probe pattern in the tree: `vppctl show version` via `exec` (line 266).
- [ ] `internal/core/diagnostic/codes.go` - lines 288-299: the registry rows for
      `doctor-vpp-wireguard` and `doctor-vpp-lcp-netns`.

**Behavior to preserve:**

- The `doctor-vpp-lcp-netns` warning while BGP cannot bind in a non-root netns
  (`doctor.go:100-131`); it is the only thing making the constraint visible today.
- `lcpPairNetns` mapping host/root/empty to "" so VPP places the TAP in its host namespace
  (`lcp.go:109-115`), and passing any other name through deliberately.
- The `doctor-vpp-wireguard` check's current config-toggle behavior (`doctor.go:62-85`).
- Doctor check registration shape and ordering (`doctor.go:27-56`, Order 740/741).
- BGP listener behavior for every non-LCP deployment: default binding must not change.

**Behavior to change:**

- None yet, research first. Candidate directions are captured in the Open Questions; the
  choice between "teach BGP netns" and "make the constraint impossible to hit" is a real
  design fork that is not settled here.

## Data Flow

### Entry Point

- Problem A: config `vpp { lcp { netns "dataplane" } }` plus a `bgp` stanza with a listen
  address. The netns value enters via the vpp component config
  (`internal/component/vpp/yang/ze-vpp-conf.yang:198-205`, default "dataplane") and reaches
  VPP through `lcp_itf_pair_add_del` (`lcp.go:87-102`). The BGP listen address enters via the
  bgp config and reaches `NewListener` (`listener.go:50`).
- Problem B: the same `vpp.lcp` config, plus the VPP binary's actual plugin set, which is a
  property of the RUNNING system and not of any config file.

### Transformation Path

1. Config parse: `vpp.lcp.netns` and `lcp.enabled` land in the vpp component's settings.
2. startup.conf generation: `linux_cp_plugin.so` and `linux_nl_plugin.so` are enabled when
   LCP is on (`startupconf.go:82-91`).
3. Doctor, post-config phase: `checkVPPLCPNetns` warns if the netns is not root-reachable
   (`doctor.go:100-131`). No check exists for plugin presence.
4. Apply: `lcpItfPair` sends `LcpItfPairAddDel` with `lcpPairNetns(settings.Netns)`
   (`lcp.go:62`, `:87-102`). A missing `linux_cp_plugin.so` fails HERE, failing the apply.
5. VPP creates the TAP in the named namespace.
6. BGP: `NewListener(addr)` (`listener.go:50`) then `StartWithContext` calls
   `listenerFactory.Listen(ctx, "tcp", addr)` (`listener.go:108`) in ze's own namespace, which
   is not the TAP's namespace. The bind cannot see the TAP.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config to vpp component | YANG `vpp/lcp` container, `netns` leaf (`ze-vpp-conf.yang:198-205`) | [ ] |
| ze to VPP (control) | GoVPP binapi `LcpItfPairAddDel` (`lcp.go:88-95`) | [ ] |
| ze to VPP (startup) | generated startup.conf plugin stanzas (`startupconf.go:82-91`) | [ ] |
| VPP to Linux | LCP creates a TAP inside the `netns` named per pair | [ ] |
| BGP to kernel socket | `net.ListenConfig` via `RealListenerFactory.Listen` (`network.go:167`), in ze's namespace | [ ] |
| Namespace boundary (THE GAP) | none: no code crosses it; BGP binds where its thread runs | [ ] |
| Doctor to running VPP | today only `vppctl` exec (`checks_linux.go:266`); no GoVPP probe from the plugin | [ ] |

### Integration Points

- `network.ListenerFactory` (`internal/core/network/network.go:32-40`) - the existing
  abstraction a netns-aware listener would most plausibly implement.
- `Listener.SetListenerFactory` (`internal/component/bgp/reactor/listener.go:66-68`) - the
  existing injection point; no reactor surgery needed to swap the factory.
- `diagnostic.RegisterDoctorCheck` (`internal/plugins/iface/vpp/doctor.go:51`) - where a
  third check registers.
- `internal/core/diagnostic/codes.go` (lines 288-299) - where a new code's registry row goes.
- `vppcomp.GetActiveConnector` (used by `internal/plugins/static/backend_vpp_linux.go:31`) -
  an existing way to reach a live VPP connector, a candidate for the GoVPP probe.

### Architectural Verification

- [ ] No bypassed layers (a netns listener goes through `ListenerFactory`, not raw syscalls
      in the reactor)
- [ ] No unintended coupling (BGP does not learn about VPP or LCP; it learns about namespaces)
- [ ] No duplicated functionality (the probe reuses an existing VPP connector rather than a
      second connection path)
- [ ] Zero-copy preserved where applicable (N/A: control-plane path, not wire encoding)
- [ ] Registration over hardcoding: the new doctor check registers via
      `diagnostic.RegisterDoctorCheck` from the owning plugin and the core discovers it; no
      per-check switch case or field is added to a core/shared package
      (`ai/rules/doctor-checks.md`, `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | BGP has zero netns awareness today | Read of `listener.go` (lines 32-46, 108) plus a grep across `internal/component/bgp/reactor/` returning only event-namespace hits | The work is smaller than believed | Re-grep at design time | confirmed by read, 2026-07-15 |
| A-2 | A netns-aware listener can be a `ListenerFactory` implementation with no reactor surgery | `SetListenerFactory` (`listener.go:66-68`) and the factory call site (`listener.go:108`) | The change reaches into reactor lifecycle and grows a lot | Prototype a factory that binds in a named netns | unvalidated |
| A-3 | Binding in a named netns needs OS-thread locking for the socket's lifetime, not just at bind | `setns` semantics are per-thread; `runtime.LockOSThread` is used in the tree's netns tests (`internal/plugins/static/resolve_integration_linux_test.go:65-86`) | The design needs a dedicated thread or a helper process; blast radius grows | Prototype plus a QEMU test | unvalidated |
| A-4 | A missing `linux_cp_plugin.so` is detectable from a live VPP before apply | `startupconf.go:82-91` asks for the plugin; the failure lands at `lcp.go:97` | The check cannot exist pre-apply and the answer is a better error at the binapi layer | Probe a VPP built without the plugin | unvalidated |
| A-5 | A GoVPP probe is the right mechanism, as the deferrals row states | deferrals 2026-07-10 row; but the only precedent (`checks_linux.go:266`) uses `vppctl` exec | The check uses vppctl instead, which conflicts with plugin self-containment | DESIGN decision with the user | unvalidated |
| A-6 | Doctor runs with a live VPP available at `DoctorPhasePostConfig` | `checkVPPVersion` execs vppctl at doctor time (`checks_linux.go:251-266`) | A runtime probe is impossible in that phase; the check needs a different phase or trigger | Trace the doctor phase's runtime context | unvalidated |
| A-7 | Operators actually want LCP TAPs in a non-root netns, so teaching BGP netns is worth it rather than forcing host netns | The `netns` leaf exists and defaults to "dataplane" (`ze-vpp-conf.yang:198-205`) | The cheaper answer is to default LCP to the host netns and keep the doctor warning | User / operator input | unvalidated |
| A-8 | The two problems belong in one spec | Both concern LCP usability | Split into `spec-bgp-netns` (as deferrals anticipated) plus a doctor follow-up | DESIGN review | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Netns binding requires thread-pinning that fights the Go runtime and destabilizes the listener | Flaky accept loop or wrong-namespace binds under load | Prototype early; consider a bind-only helper that passes the fd back |
| R-2 | Fixing A-4 silently invalidates `doctor-vpp-lcp-netns`, leaving a check that warns about a solved problem | The check still fires after BGP can bind in the netns | Treat the check's fate as an explicit AC, not an afterthought |
| R-3 | "Parallel to doctor-vpp-wireguard" misleads: the wireguard check is config-only and the LCP one cannot be | A design that greps the config tree for a non-existent LCP toggle | Confirmed already by reading `doctor.go:62-95`; the LCP check needs a runtime probe |
| R-4 | A GoVPP probe from the doctor path opens a VPP connection at diagnosis time, with its own failure modes (socket absent, VPP down) | Doctor errors on a box where VPP simply is not running yet | Degrade to a warning when VPP is unreachable, as `checkVPPVersion` does (`checks_linux.go:268-274`) |
| R-5 | LCP is enabled by default (`ze-vpp-conf.yang:176-181`), so a new error-severity check could fail apply for existing working deployments | Deployments that were fine start failing doctor | Choose severity deliberately; the netns check chose Warning (`doctor.go:128`) |
| R-6 | Linux-only work lands without QEMU proof | The netns leg is proven only by unit tests | `ai/rules/qemu-testing.md` is mandatory; identify the rail during research |
| R-7 | The `doctor-vpp-wireguard` code description already over-claims (see Design Insights); copying its wording into a new code repeats the error | The new code's description promises a runtime check it does not do | Write the description to match the mechanism actually implemented |

## Wiring Test (MANDATORY)

Candidate rows. Every row is a proposal from a code read, to be confirmed by `/ze-spec`.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `vpp { lcp { netns dataplane } }` plus a `bgp` stanza, `ze doctor` | -> | `checkVPPLCPNetns` (`doctor.go:100`) | `test/ui/doctor-vpp-lcp-netns.ci` (verify whether it already exists) |
| `vpp { lcp { enabled true } }` on a VPP build without `linux_cp_plugin.so`, `ze doctor` | -> | `checkVPPLCPPlugin` (new, `internal/plugins/iface/vpp/doctor.go`) | `test/ui/doctor-vpp-lcp-plugin.ci` (new) |
| `bgp { listen { netns dataplane } }` (candidate config surface), listener start | -> | netns-aware `ListenerFactory` (`internal/core/network/`) then `listener.go:108` | `TestNetnsListenerFactoryBindsInNamedNamespace` |
| BGP peer establishing over an LCP TAP in a non-root netns | -> | full listener plus reactor accept path (`listener.go:149-202`) | QEMU integration test, rail to be identified per `ai/rules/qemu-testing.md` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `vpp.lcp.netns` names a non-root namespace and BGP is configured | BGP binds its listener inside that namespace and accepts sessions on the LCP TAP |
| AC-2 | No netns is configured anywhere (every non-LCP deployment) | Listener behavior is byte-for-byte unchanged; no thread pinning, no new failure mode |
| AC-3 | BGP can bind in the LCP netns | The `doctor-vpp-lcp-netns` warning's fate is explicitly decided: removed, narrowed, or kept with a new rationale. It does not silently survive as a stale warning |
| AC-4 | `lcp.enabled` is true and the running VPP lacks `linux_cp_plugin.so` | `ze doctor` reports an actionable diagnostic BEFORE apply, naming the missing plugin |
| AC-5 | `lcp.enabled` is true and the plugin IS present | No diagnostic |
| AC-6 | `lcp.enabled` is true and VPP is unreachable at doctor time | Degrades to a warning about the probe, never a false "plugin missing" claim |
| AC-7 | The new diagnostic code | Registered in `internal/core/diagnostic/codes.go` with a description matching what the check actually does, and `ze explain <code>` works |
| AC-8 | Netns listener on linux | Proven by a QEMU integration test, not only unit tests (`ai/rules/qemu-testing.md`) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Runs VPP with default LCP settings and expects BGP to peer over a VPP NIC | config -> startup.conf -> lcp_itf_pair_add_del -> TAP in "dataplane" netns -> BGP listener binds in that netns -> peer established | QEMU integration test (rail to identify) |
| 2 | Runs `ze doctor` on a VPP build lacking linux_cp_plugin.so | config tree + live VPP probe -> `checkVPPLCPPlugin` -> diagnostic code | `test/ui/doctor-vpp-lcp-plugin.ci` (new) |
| 3 | Runs `ze explain doctor-vpp-lcp-plugin` | code registry (`codes.go`) -> explain output | `test/ui/doctor-vpp-lcp-plugin.ci` (new) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckVPPLCPPluginMissingWarns` | `internal/plugins/iface/vpp/doctor_test.go` | `lcp.enabled` true plus a probe reporting no linux_cp yields the diagnostic | proposed |
| `TestCheckVPPLCPPluginPresentSilent` | `internal/plugins/iface/vpp/doctor_test.go` | Plugin present yields no diagnostic | proposed |
| `TestCheckVPPLCPPluginProbeUnavailable` | `internal/plugins/iface/vpp/doctor_test.go` | VPP unreachable degrades to a probe warning, never a false negative claim | proposed |
| `TestCheckVPPLCPPluginDisabledSkips` | `internal/plugins/iface/vpp/doctor_test.go` | `lcp.enabled` false skips the check entirely | proposed |
| `TestNetnsListenerFactoryBindsInNamedNamespace` | `internal/core/network/netns_linux_test.go` | The factory binds inside the named namespace | proposed |
| `TestNetnsListenerFactoryUnknownNamespaceErrors` | `internal/core/network/netns_linux_test.go` | An absent namespace errors clearly rather than silently binding in the host netns | proposed |
| `TestListenerUsesInjectedFactory` | `internal/component/bgp/reactor/listener_test.go` | The existing injection seam carries a netns factory through `StartWithContext` | proposed |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `vpp.lcp.netns` | string, YANG default "dataplane" (`ze-vpp-conf.yang:198-205`) | any existing namespace name; "", "host", "root" mean the host netns (`doctor.go:136-143`) | N/A (no numeric range) | N/A |
| Probe timeout | candidate 3s, matching `checkVPPVersion` (`checks_linux.go:264`) | 3s | a timeout so short the probe flaps | a timeout that stalls `ze doctor` |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-vpp-lcp-plugin` | `test/ui/doctor-vpp-lcp-plugin.ci` | An operator on a VPP build without linux_cp_plugin.so is warned before apply | proposed |
| `doctor-vpp-wireguard` | `test/ui/doctor-vpp-wireguard.ci` | The existing sibling test to model the new one on | exists |
| BGP over LCP TAP in a non-root netns | QEMU rail, to be identified | BGP peers over a VPP-managed NIC with default LCP settings | proposed |

### Interop Tests

Deferred to DESIGN. This spec changes no BGP wire behavior (the listener binds in a
different namespace; the protocol is untouched), so interop is likely N/A. Confirm during
research rather than assuming: `ai/rules/interop-and-goal-validation.md` governs.

### Future

None deferred. This is a skeleton; scope is set at DESIGN.

## Files to Modify

Candidates from a code read; confirm during research.

- `internal/plugins/iface/vpp/doctor.go` - add the LCP plugin-presence check and register it
  alongside the existing two (`registerDoctorChecks`, lines 27-56)
- `internal/core/diagnostic/codes.go` - add the new code's registry row beside lines 288-299
- `internal/core/network/network.go` - add a netns-aware `ListenerFactory` beside
  `RealListenerFactory` (lines 147-167), or a new file in the same package
- `internal/component/bgp/reactor/listener.go` - likely unchanged if A-2 holds; the factory
  seam already exists (lines 66-68, 108)
- `internal/component/bgp/reactor/config.go` - only if a BGP-side netns config surface is added
- `internal/component/bgp/yang/` - only if a netns leaf is added; read `ai/rules/config-surface.md`
  first
- `docs/guide/vpp.md` - the netns constraint is operator-facing and documented there
- `plan/learned/1098-followup-vpp-iface.md` - the origin summary; update if A-4's premise changes

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Only if BGP gains a netns leaf. Read `ai/rules/config-surface.md` (YANG vs env var) and `ai/rules/config-naming.md` | `internal/component/bgp/yang/` |
| YANG validation constraints | [ ] If a netns leaf lands: pattern/length, not a bare `type string` | per `ai/patterns/config-option.md` |
| Doctor check for runtime dependencies | [ ] Yes, that is Problem B | `internal/plugins/iface/vpp/doctor.go`, `internal/core/diagnostic/codes.go` |
| Functional test | [ ] Yes | `test/ui/doctor-vpp-lcp-plugin.ci` |
| CLI commands/flags | [ ] No | - |
| Prometheus counters | [ ] No | - |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes if BGP gains netns binding | `docs/features.md` |
| 2 | Config syntax changed? | [ ] Only if a netns leaf is added | `docs/guide/configuration.md` |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/vpp.md` |
| 12 | Internal architecture changed? | [ ] If the listener gains a namespace concept | `docs/architecture/core-design.md` |
| 15 | Registered diagnostic code changed? | [ ] Yes, a new doctor code | `internal/core/diagnostic/codes.go`, relevant guide |
| 16 | Any changed source file referenced by doc source anchors? | [ ] Grep `docs/` for the changed files | per grep |

## Files to Create

- `internal/core/network/netns_linux.go` - proposed netns-aware `ListenerFactory`
- `internal/core/network/netns_linux_test.go` - proposed unit tests
- `test/ui/doctor-vpp-lcp-plugin.ci` - proposed functional test for the new check

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

**BLOCKING: this spec is a skeleton. Phase 0 is research via `/ze-spec`; the phases below
are placeholders whose shape depends on the Open Questions.**

1. **Phase 0: Research (`/ze-spec`)**: answer the Open Questions, especially the netns
   binding mechanism (A-3), the probe mechanism fork (A-5), and whether the two problems stay
   in one spec (A-8). Then rewrite the phases below and move Status to `design`.
2. **Phase: Doctor check (Problem B, the bounded half)**: probe for `linux_cp_plugin.so` and
   register the check plus its code.
   - Tests: `TestCheckVPPLCPPluginMissingWarns`, `TestCheckVPPLCPPluginPresentSilent`,
     `TestCheckVPPLCPPluginProbeUnavailable`, `TestCheckVPPLCPPluginDisabledSkips`
   - Files: `internal/plugins/iface/vpp/doctor.go`, `internal/core/diagnostic/codes.go`
   - Verify: `test/ui/doctor-vpp-lcp-plugin.ci` passes
3. **Phase: Netns listener factory (Problem A)**: bind inside a named namespace behind the
   existing `ListenerFactory` seam.
   - Tests: `TestNetnsListenerFactoryBindsInNamedNamespace`,
     `TestNetnsListenerFactoryUnknownNamespaceErrors`
   - Files: `internal/core/network/netns_linux.go`
4. **Phase: Wire the factory to BGP**: decide and add the config surface, then inject.
   - Tests: `TestListenerUsesInjectedFactory`
   - Files: `internal/component/bgp/reactor/`, possibly `internal/component/bgp/yang/`
5. **Phase: Netns doctor check reconciliation**: decide the fate of `doctor-vpp-lcp-netns`
   (AC-3) and implement that decision.
6. **Functional and QEMU tests**: prove BGP peers over an LCP TAP in a non-root netns.
7. **Full verification**: `make ze-verify`
8. **Complete spec**: learned summary, two-commit closure.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | A netns bind failure NEVER silently falls back to the host namespace (that would peer on the wrong interface) |
| Data flow | BGP learns about namespaces, not about VPP or LCP; no VPP spelling in the reactor |
| Registration over hardcoding | The new doctor check registers via `diagnostic.RegisterDoctorCheck` from the owning plugin; no switch case or field added to a core/shared package (`ai/rules/doctor-checks.md`, `ai/rules/plugin-self-containment.md`) |
| Doctor checks | New runtime dependency has a check, a code in `codes.go`, a unit test, and a functional test per `ai/rules/doctor-checks.md` |
| YANG validation | If a netns leaf lands: maximum native constraints, no bare `type string` |
| Rule: no-workarounds | The netns constraint is fixed at the source, not documented away |
| Stale check | `doctor-vpp-lcp-netns` does not survive as a warning for a solved problem (AC-3) |
| Code description accuracy | The new code's description in `codes.go` matches the mechanism actually implemented (see R-7) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| LCP plugin-presence doctor check | `test/ui/doctor-vpp-lcp-plugin.ci` passes |
| New diagnostic code registered | `ze explain <new-code>` returns the entry; grep `internal/core/diagnostic/codes.go` |
| BGP binds in a named netns | QEMU integration test showing an established peer over the LCP TAP |
| Default deployments unchanged | Existing BGP tests pass with no listener behavior change |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A config-supplied namespace name reaches a `setns`-style operation; validate it and never traverse to an arbitrary path |
| Privilege | Entering a namespace needs CAP_SYS_ADMIN; confirm the failure mode when ze lacks it is a clear error, not a silent host-netns bind |
| Resource exhaustion | Thread pinning per listener must not leak OS threads across restarts |
| Error leakage | Probe and bind errors name the namespace; confirm they carry no unexpected host detail |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Netns bind works in unit tests but not under QEMU | Back to RESEARCH on thread pinning (A-3); do not weaken the test |
| The GoVPP probe cannot run at doctor time | Re-open A-5/A-6 with the user; vppctl exec is the fallback precedent |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The deferrals row frames Problem B as "parallel to `doctor-vpp-wireguard`". A code read
  shows the parallel is intent-only: `checkVPPWireguardPlugin` (`doctor.go:62-85`) is a
  CONFIG check reading the `vpp.plugins.wireguard` toggle, while LCP has no such toggle
  (`startupconf.go:82-91` enables the plugin straight from `lcp.enabled`). The new check
  cannot be modeled on the old one's mechanism, only on its shape and registration.
- FINDING (pre-existing, unrelated to this spec's scope): the `doctor-vpp-wireguard` code
  description in `internal/core/diagnostic/codes.go:291` claims the check covers "not enabled
  (vpp.plugins.wireguard) **or not loaded in the running VPP**", but `checkVPPWireguardPlugin`
  only reads the config toggle and never probes VPP. The description over-claims. Decide
  during research whether to correct it here or in its own fixit.
- `vpp.lcp.netns` defaults to "dataplane" (`ze-vpp-conf.yang:198-205`) and
  `lcpNetnsIsRootReachable` (`doctor.go:136-143`) accepts only "", "host", "root". So the
  SHIPPED DEFAULT trips the netns warning whenever BGP is configured. That reframes A-4 from
  an edge case to the default experience, and is the strongest argument for fixing it.
- The BGP listener already takes an injectable `ListenerFactory` (`listener.go:36`, `:66-68`,
  `:108`). The netns work may need no reactor change at all, which would make A-4 far cheaper
  than "BGP has zero netns awareness" suggests.

## Known Limitations

- To be set at DESIGN. Candidate: only the BGP listener gains netns awareness; outbound BGP
  connections, and other listeners (web, gnmi, lg), stay in ze's namespace unless research
  shows they must move together.

## Open Questions (research before design)

- What is the right mechanism for binding a listener inside a named netns from Go: a
  thread-pinned `setns` around the bind, a dedicated OS thread owned for the socket's
  lifetime, or a helper that binds and passes the fd back? A-3 is the crux of the netns leg.
- Does the socket need to STAY in the namespace after bind, or is the namespace only relevant
  at bind time? This decides whether thread pinning is per-bind or lifetime-long.
- Should the netns be a BGP config leaf (`bgp { listen { netns ... } }`), inherited from
  `vpp.lcp.netns`, or a process-level setting? Read `ai/rules/config-surface.md` before
  choosing. Inheriting couples BGP to VPP, which the current architecture avoids.
- Do outbound BGP connections (the dialer) need the same namespace treatment, or is the
  listener enough for LCP TAP peering?
- Do the other listeners (web, gnmi, looking glass) need this too, or is BGP genuinely special?
- If BGP can bind in the LCP netns, what happens to `doctor-vpp-lcp-netns`
  (`doctor.go:100-131`)? Removed, narrowed to "netns configured but BGP not told about it",
  or kept? AC-3 must not be answered by accident.
- Should `vpp.lcp.netns`'s default stay "dataplane" given it trips the warning out of the box?
- GoVPP probe or `vppctl show plugins`? The deferrals row says GoVPP; the only precedent
  (`checks_linux.go:251-266`) uses vppctl exec from the central doctor package. Which one
  respects `ai/rules/doctor-checks.md` and `ai/rules/plugin-self-containment.md`?
- Is a live VPP connection available during `DoctorPhasePostConfig`? If not, when does the
  probe run, and is `vppcomp.GetActiveConnector` usable there?
- What severity should the LCP plugin check use? The wireguard check chose Error
  (`doctor.go:82`), the netns check chose Warning (`doctor.go:128`). LCP is enabled by
  default, so Error could break existing deployments (R-5).
- Does `test/ui/doctor-vpp-lcp-netns.ci` already exist? Only `test/ui/doctor-vpp-wireguard.ci`
  was found; confirm before writing the wiring table's first row.
- What QEMU rail can prove BGP peering over an LCP TAP, and does any VPP QEMU test exist today?
- Should this spec be split into `spec-bgp-netns` (as the deferrals row anticipated) plus a
  bounded doctor follow-up? The doctor half is small and shippable; the netns half is not.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete (every row has a concrete test name, none deferred)
- [ ] `/ze-review` gate clean (Review Gate section filled: 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] QEMU integration test for the linux-only netns leg (`ai/rules/qemu-testing.md`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass, defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes: all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
