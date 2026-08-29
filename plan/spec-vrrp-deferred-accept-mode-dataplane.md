# Spec: vrrp-deferred-accept-mode-dataplane

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-08-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:** this spec file;
`.claude/rules/planning.md`; `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`
(the closed VRRP work and its Known Limitations);
`internal/plugins/vrrp/instance.go`, `groups.go`, `fsm/fsm.go`.

## Task

Deferral holder. Provenance: `spec-vrrp-6-interop` (Known Limitations), recorded
2026-07-15 in `plan/deferrals.md` row 70. The named destination
(`plan/spec-vrrp-0-umbrella.md`) was closed and removed, so this file is the
work's home. The surviving `plan/future/spec-vrrp-7-vpp.md` covers the VPP dataplane
only and does not own this topic.

Two pieces of deferred work, verified against the producing code on 2026-07-16:

## OWNER RULING 2026-08-05: implement the accept-mode filtering

**Thomas ruled that ze IMPLEMENTS the dataplane filtering.** The two cheaper
answers put beside it were not taken: rejecting `accept-mode false` on a
non-owner at config validation, and leaving the disclosed `{gap}` as it stands.

This closes an open RFC MUST NOT violation rather than adding a feature.
`RFC9568-6.4.3-7` says "Active: never accept packets addressed to the Virtual
Router IPvX address(es) when neither owner nor Accept_Mode True", and the ledger
carries it as a `{gap}`. Under the 2026-07-27 directive that classification was
void and had to be re-raised rather than cited; this is the answer.

**Re-verified at the producers 2026-08-05**, so implementation starts from
measurement:

- The FSM emits `InstallVIPs{VIPs: i.cfg.VIPs}` unconditionally at all three
  promotion sites (`internal/plugins/vrrp/fsm/fsm.go`).
- `doInstallVIPs` registers the whole VIP set through the iface address-owner
  registry with no differentiation (`internal/plugins/vrrp/instance.go`).
- `AcceptMode` reaches config parsing, the version-2 rejection rule, and the show
  snapshot. Nothing else. `fsm/events.go` states it in the struct: "stored for
  the state snapshot only".

So a non-owner Active with `accept-mode false` answers traffic on the virtual
address today, whatever the operator configured.

### What the ruling commits ze to

| Piece | Where |
|-------|-------|
| Per-VIP filtering installed on promotion, removed on demotion | The `InstallVIPs` payload must carry the decision, and `doInstallVIPs` must act on it. Today neither sees `AcceptMode` |
| The Section 6.1 owner exemption | `EffectiveAcceptMode` (`internal/plugins/vrrp/groups.go`) already folds ownership in, so the decision input exists |
| The R014 carve-out: never drop IPv6 NS/NA even with Accept_Mode false | ICMPv6 types 135 and 136 must survive the filter. This is `RFC9568-6.1-1` |
| A tagged test per requirement row | `RFC9568-6.4.3-7` and `RFC9568-6.1-1` |
| A QEMU integration test | Mandatory for linux-only code, never skipped for "needs hardware" (`ai/rules/platform-linux.md`) |
| The YANG description stops disclaiming the gap | `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` |

### The ledger consequence, easy to miss

`RFC9568-6.1-1` is currently `{not-applicable}` and its reason says why: the
prohibition "carves an exception out of Accept_Mode packet filtering, and ze
installs no such filter at all". **Once the filter exists that reason expires.**
The requirement becomes live and needs its own tagged test, so re-classifying it
is part of this spec's closure rather than a later discovery.

### Scope note

Item 2 below, priority-decrement tracking, is untouched by this ruling and stays
open. It is a separate feature with its own YANG surface.

1. **accept-mode is not enforced on the dataplane.** The leaf is parsed
   (`groups.go`), cross-leaf validated (rejected under version 2,
   `groups.go`), carried into the FSM config (`instance.go`) and
   reported in the `show vrrp` payload (`instance.go` into the
   `accept-mode` JSON field, `vrrp.go`). Nothing consumes it beyond the
   snapshot: `fsm/events.go` says so in the config struct itself, and the
   FSM emits `InstallVIPs{VIPs: i.cfg.VIPs}` (`fsm/fsm.go`, `:359`, `:378`)
   without consulting `cfg.AcceptMode`. The executor's `doInstallVIPs`
   (`instance.go`) then registers the full VIP set through the iface
   address-owner registry unconditionally. That is the function that would have
   to differentiate, and it does not. Result: an Active non-owner holds the VIPs
   as ordinary kernel addresses on the macvlan and answers traffic addressed to
   them whichever way the leaf is set, violating RFC 9568 R030/R031
   (`rfc/short/rfc9568.md`). Work: install real filtering, keeping the
   owner exemption (Section 6.1) and the IPv6 NS/NA carve-out (R014,
   `rfc/short/rfc9568.md`), and stop the YANG description
   (`yang/ze-vrrp-conf.yang`) from having to disclaim the gap.

2. **Priority-decrement tracking is not implemented.** No interface, route or
   health tracking object exists: the vrrp YANG has no tracking leaf
   (`yang/ze-vrrp-conf.yang` group list stops at `accept-mode`), and the only
   priority producer is `GroupSpec.EffectivePriority` (`groups.go`),
   which returns 255 for an owner and the configured constant otherwise, with no
   decrement input. Junos, Nokia and VyOS offer it; ze does not.

Design phase must decide whether these stay one spec or split: they share only
the "Active router behavior beyond the election" theme.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/vrrp.md` - the operator-facing statement of the current limitation
  → Constraint: whatever ships here must retire the documented caveat, not add a second one
- [ ] `ai/rules/architecture.md` - exact or reject
  → Constraint: a leaf ze cannot enforce exactly must fail verify, never approximate silently
- [ ] `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` - the macvlan/vmac recipe the filter must not break
  → Constraint: the ARP/ND sysctl recipe makes the macvlan the sole responder; a filter that drops ARP or ND breaks virtual-MAC ownership

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - the conformance target
  → Constraint: R030/R031 (§6.4.3, `rfc/short/rfc9568.md`): Active accepts packets to the virtual addresses only if owner or Accept_Mode is True, otherwise MUST NOT accept them
  → Constraint: R014 (§6.1/§6.4.3, `rfc/short/rfc9568.md`): IPv6 NS/NA are never dropped, even with Accept_Mode False
- [ ] `rfc/short/rfc3768.md` - v2 has no Accept_Mode concept
  → Constraint: the v2 rejection at `groups.go` stays; this spec changes v3 behavior only

**Key insights:**
- The FSM is not the gap. The FSM already carries the flag; the dataplane never reads it.
- Accept_Mode False is the RFC default (`rfc/short/rfc9568.md`), so today's default configuration is the non-conforming one.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-16)
- [ ] `internal/plugins/vrrp/groups.go` - parses `accept-mode` (:411-416), rejects it under v2 (:531-532), computes `EffectiveAcceptMode` as owner-or-configured (:148-154) and `EffectivePriority` with no decrement input (:141-146)
- [ ] `internal/plugins/vrrp/instance.go` - projects the flag into the FSM config (:420), installs the full VIP set unconditionally (:369-374), reports the flag in the show snapshot (:530)
- [ ] `internal/plugins/vrrp/fsm/fsm.go` - emits `InstallVIPs` with `cfg.VIPs` (:146, :359, :378), never reading `cfg.AcceptMode`; snapshot carries it (:398, :429)
- [ ] `internal/plugins/vrrp/fsm/events.go` - config field comment states the flag is stored for the snapshot only (:45-47)
- [ ] `internal/plugins/vrrp/vrrp.go` - `instanceView.AcceptMode` is the `accept-mode` JSON field of `show vrrp` (:95)
- [ ] `internal/plugins/vrrp/dataplane_linux.go` - the sysctl recipe that gives the macvlan ARP/ND ownership (:120-183); no packet filtering of any kind
- [ ] `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` - `accept-mode` leaf, boolean, default false, description discloses "not dataplane-enforced this pass" (:121-129)

**Behavior to preserve:**
- Owner semantics: an address owner accepts regardless of the leaf (`EffectiveAcceptMode`, `groups.go`) and advertises priority 255 (`EffectivePriority`, `groups.go`)
- The v2 plus accept-mode config rejection and its message (`groups.go`, `test/vrrp/vrrp-config-invalid.ci`)
- The doctor check that reports the same cross-leaf violation (`internal/plugins/vrrp/doctor.go`, `test/vrrp/vrrp-doctor-fires.ci`)
- The `accept-mode` field in the `show vrrp` payload (`vrrp.go`)
- The virtual-MAC ARP/ND recipe and its refcounted save/restore (`dataplane_linux.go`)
- Election, timers and failover behavior: this spec touches what an Active router accepts, never who wins

**Behavior to change:**
- With Accept_Mode False and a non-owner Active, packets addressed to a virtual address are no longer accepted (except IPv6 NS/NA)
- Retire the YANG description disclaimer and the `docs/guide/vrrp.md` caveat once enforced
- Priority tracking: new config surface and a decrement path into the advertised priority (design phase decides the shape)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `interface ... unit ... ipv4|ipv6 vrrp group <name> accept-mode <bool>`, parsed at `groups.go` into `GroupSpec.AcceptMode`
- Runtime: the FSM's transition into Active emits `InstallVIPs` (`fsm/fsm.go`, `:359`, `:378`), executed at `instance.go`
- Wire: unicast or multicast frames arriving at the macvlan addressed to a virtual address (the traffic this spec must gate)

### Transformation Path
1. Config tree to `GroupSpec.AcceptMode` (`groups.go`), then cross-leaf verify rejects the v2 combination (`groups.go`)
2. `GroupSpec.EffectiveAcceptMode` folds in ownership (`groups.go`) and `instance.fsmConfig` projects it onto `fsm.Config` (`instance.go`)
3. FSM reaches Active and emits `InstallVIPs`; `instance.doInstallVIPs` registers the VIP CIDRs with the iface address-owner registry (`instance.go`), which reconciles them onto the macvlan
4. Today the chain ends there: the kernel answers for the VIP as for any local address. The missing stage is a per-instance acceptance filter installed and torn down alongside the VIPs, keyed on the effective accept-mode
5. `show vrrp` reads the flag back out of the FSM snapshot (`instance.go`, `vrrp.go`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ vrrp plugin | `GroupSpec` extraction and cross-leaf verify | [ ] |
| vrrp plugin ↔ FSM | `fsm.Config` projection (`instance.go`) | [ ] |
| vrrp plugin ↔ iface address owner | `RegisterOwnedAddresses` via `deps.installVIPs` | [ ] |
| vrrp plugin ↔ kernel filtering | to be decided at design (nftables via the firewall component, socket filter, or per-device sysctl) | [ ] |

### Integration Points
- `instance.doInstallVIPs` / `doRemoveVIPs` (`instance.go`): the filter's install and teardown must share their lifetime, or a demoted Backup keeps a stale rule
- `dataplane_linux.go` sysctl recipe: the filter must sit above ARP/ND resolution so the virtual MAC still answers
- `internal/component/firewall/`: the existing rule-installation surface, if the design chooses nftables rather than a socket-level filter
- `GroupSpec.EffectivePriority` (`groups.go`): the single point a tracking decrement would feed

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Filtering can be installed without breaking the virtual-MAC ARP/ND recipe | `dataplane_linux.go` operates on sysctls only, orthogonal to a packet filter | The recipe and the filter must be co-designed; QEMU proof needed early | QEMU lab: VIP unreachable with accept-mode false, ARP still answered from the virtual MAC | unvalidated |
| A-2 | The existing firewall component can express a per-device destination-address drop | `internal/component/firewall/` installs rules today (surface not yet read for this spec) | A vrrp-owned filter path is needed instead | Design phase: read the firewall install path | unvalidated |
| A-3 | Interop scenarios that ping the VIP set accept-mode true and so keep passing | `internal/le/qemu/vrrp_keepalived_linux.go` sets accept-mode true for QS-1 | Enforcing the flag reds the interop lab | Run the keepalived lab after enforcement | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Enforcement makes the VIP unpingable and looks like a regression to operators | Support reports "ping to VIP stopped working after upgrade" | Release note plus the RFC 9568 §6.1 ping guidance (`rfc/short/rfc9568.md`) |
| R-2 | Dropping IPv6 NS/NA with the filter breaks ND (violates R014) | IPv6 failover leaves stale neighbor entries | Explicit NS/NA carve-out with a dedicated test |
| R-3 | Tracking grows into a large config surface (objects, groups, weights) and stalls the accept-mode fix | Design phase expands past the accept-mode work | Split tracking into its own spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `accept-mode false` on a non-owner Active | → | acceptance filter installed alongside the VIPs | `test/vrrp/vrrp-accept-mode.ci` |
| Active demotes to Backup | → | filter removed with `doRemoveVIPs` | `test/vrrp/vrrp-accept-mode.ci` |
| Priority tracking object goes down | → | decremented priority reaches the advertisement | `test/vrrp/vrrp-track.ci` |

## Acceptance Criteria

Skeleton level; the design phase expands these.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Non-owner Active, accept-mode false, ping to VIP | No reply (RFC 9568 R031) |
| AC-2 | Non-owner Active, accept-mode true, ping to VIP | Reply (RFC 9568 R030) |
| AC-3 | Address owner, accept-mode false | Accepts anyway (RFC 9568 §6.1); `show vrrp` reports effective accept-mode true |
| AC-4 | IPv6 non-owner Active, accept-mode false, NS to VIP | NA is still sent (RFC 9568 R014) |
| AC-5 | Active demotes to Backup | Filter and VIPs disappear together; no stale rule |
| AC-6 | Tracked object goes down | Advertised priority drops by the configured decrement |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAcceptModeFilterSpec` | `internal/plugins/vrrp/` (design phase fixes the file) | effective accept-mode plus VIP set maps to the intended filter rules; owner and IPv6 NS/NA carve-outs | |
| `TestAcceptModeFilterLifecycle` | `internal/plugins/vrrp/` | install on Active, remove on Backup, idempotent under reconfigure | |
| `TestEffectivePriorityWithTracking` | `internal/plugins/vrrp/groups_test.go` | decrement applied, floor respected, owner still forced to 255 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| track decrement (shape to be decided at design) | 1-254 | 254 | 0 | 255 |
| effective priority after decrement | 1-254 (non-owner) | 254 | 0 | 255 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vrrp-accept-mode.ci` | `test/vrrp/` | accept-mode true and false change what the Active answers | |
| `vrrp-track.ci` | `test/vrrp/` | tracked object down decrements the advertised priority | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| accept-mode false vs keepalived | `internal/le/qemu/vrrp_keepalived_linux.go` lab | keepalived | VIP unreachable on the ze Active while election and virtual-MAC ownership are unaffected | |

### Future (if deferring any tests)
- None; the whole spec is future work and this file is its destination.

## Files to Modify
- `internal/plugins/vrrp/instance.go` - install and remove the filter with the VIPs (`doInstallVIPs` / `doRemoveVIPs`)
- `internal/plugins/vrrp/groups.go` - tracking config into `GroupSpec`; decrement into `EffectivePriority`
- `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` - drop the accept-mode disclaimer; add tracking leaves
- `internal/plugins/vrrp/fsm/events.go` - retire the "snapshot only" comment once the flag drives behavior
- `docs/guide/vrrp.md` - retire the limitation; document the ping consequence
- `docs/features/rfc-status.md` - RFC 9568 R014/R030/R031 rows

## Implementation Steps

Stage mapping follows `plan/TEMPLATE.md` unchanged.

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** -- filter seam at the install/remove path plus a failing `vrrp-accept-mode.ci`
2. **Phase: Filter rules** -- effective accept-mode to rules, owner and NS/NA carve-outs
3. **Phase: Lifecycle** -- install, remove, reconfigure, restart-safety
4. **Phase: Tracking** -- config surface, decrement into `EffectivePriority`, advertisement path
5. **Functional and interop tests** -- `.ci` coverage plus the keepalived lab re-run
6. **Full verification** -- `./le verify current mode full`
7. **Complete spec** -- audit, learned summary, two-commit closure

### Failure Routing
| Failure | Route To |
|---------|----------|
| Filter breaks ARP/ND ownership | Back to design: A-1 broken, co-design with the sysctl recipe |
| Interop lab reds | Check A-3; scenario config, not the feature, may need the update |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Known Limitations
- Skeleton: no design done. Filter mechanism, tracking config shape, and whether the two halves split into separate specs are all open.
- Linux only in scope. The VPP dataplane path belongs to `plan/future/spec-vrrp-7-vpp.md`, whose R-1 already names accept-mode as a divergence risk.

## RFC Documentation

At implementation: `// RFC 9568 Section 6.4.3` comments on the filter decision
and `// RFC 9568 Section 6.1` on the owner and NS/NA carve-outs; update the
R014/R030/R031 rows in the `rfc/short/rfc9568.md` checklist.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete, every row a concrete test
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/plugins/vrrp/`)
- [ ] Documentation Update Checklist answered with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled

### Completion (BLOCKING -- before ANY commit)
- [ ] Implementation Summary and Audit filled
- [ ] Learned summary written
- [ ] Two-commit closure per `ai/rules/planning.md`
