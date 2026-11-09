# Spec: ra-vrrp-virtual-router

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | RFC 9568 is declared by no release profile (`rfc/short/rfc9568.md` Use cases `-`), and suppressing the final advertisement is an operator option Ze lacks, so both wait until after the first release |
| RFC gaps | RFC9568-6.4.3-4, RFC9568-6.4.2-3, RFC9568-8.2.3-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two routers that share an IPv6 default gateway through VRRP need Router
Advertisements that survive a failover. Today they do not, for two reasons:

1. **Ze sends no Router Advertisement for a VRRP virtual router.** RFC 9568
   Section 6.4.3 says the Active Router "MUST send ND Router Advertisements for
   the Virtual Router", and Section 6.4.2 says a Backup "MUST NOT send ND Router
   Advertisement messages for the Virtual Router". Hosts can only learn the
   virtual link-local address as their default router from those messages.
2. **Every stop sends a final Router Advertisement with Router Lifetime 0.**
   When the advertisement is the virtual router's, that final message tells
   every host the shared router is gone, and hosts remove their default route
   until the new Active Router's next advertisement. VyOS measured 1.6 seconds of
   IPv6 loss on each Active Router reboot (T9413). The same message costs a
   single router an outage on a daemon restart: hosts drop the route at the
   stop and learn it again only from the first advertisement after the start.

The operator value: a VRRP pair that hands over the IPv6 default router with no
host-visible loss, and a per-unit choice not to retire the router on stop.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, commit `dd82e3b98`
(`router-advert: T9413: add no-remove-on-exit and no-flush-* options`). VyOS
drives radvd and exposes three valueless leaves per interface:

| VyOS leaf | radvd directive | Applies to Ze? |
|-----------|-----------------|----------------|
| `no-remove-on-exit` | `RemoveAdvOnExit off` | Yes: Ze always sends the final advertisement (Current Behavior) |
| `no-flush-name-server` | `FlushRDNSS off` | No: Ze's final advertisement keeps every option as configured; only Router Lifetime becomes 0 (`zeroLifetime`) |
| `no-flush-dnssl` | `FlushDNSSL off` | No: same reason, and Ze sends no DNSSL option |

Pitfall from the VyOS work: VyOS renders the directive only when the leaf is
set, so an existing configuration produces the same daemon file. Ze keeps the
same property: with the new leaf absent, the stop sends exactly what it sends
today (AC-5).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` - how the Active
  Router owns the virtual MAC and the virtual addresses
  → Constraint: the virtual link-local lives on the VRRP macvlan; a Router
  Advertisement for the virtual router leaves from it

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9568.md` - rows RFC9568-6.4.2-3, RFC9568-6.4.3-4, RFC9568-8.2.3-1, all `{gap}` today
- [ ] `rfc/short/rfc4861.md` - Section 6.2.5, the final advertisement

RFC text read for this spec:

- RFC 4861 Section 6.2.5: "In such cases, the router SHOULD transmit one or
  more (but not more than MAX_FINAL_RTR_ADVERTISEMENTS) final multicast Router
  Advertisements on the interface with a Router Lifetime field of zero." The
  final advertisement is a SHOULD, so an operator may turn it off.
- RFC 9568 Section 6.4.3: "It MUST send ND Router Advertisements for the
  Virtual Router."
- RFC 9568 Section 8.2.3: "When a Backup VRRP Router has become the Active
  Router for a Virtual Router, it is responsible for sending Router
  Advertisements for the Virtual Router, as specified in Section 6.4.3.  The
  Backup Routers MUST be configured to send the same Router Advertisement
  options as the address owner."

**Key insights:**
- A final advertisement from the virtual link-local is always wrong while
  another router stays Active for that virtual router: it retires a router that
  still exists. The VRRP case therefore needs no knob; the knob serves the
  single-router restart.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/ra/sender_linux.go` - `Sender.run` calls
  `sendFinal` when its context ends (a commit that removes the unit's
  advertisement, or a daemon stop). `sendFinal` sends ONE advertisement with
  Router Lifetime 0 unless the link is down, after `ndp.CeaseWait`.
  `zeroLifetime` changes only Router Lifetime.
- [ ] `internal/component/iface/reconcile_ra.go` - `RASenderSpec` carries the
  interface, unit, advertisement, intervals, WANs and delegated prefixes. It
  has no field for the source address and none for the final advertisement.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - container
  `router-advertisement`: `enabled`, intervals, `router-lifetime`, `hop-limit`,
  `managed`, `other-config`, timers, `prefix` list, `rdnss`. No source-address
  leaf and no final-advertisement leaf.
- [ ] `internal/plugins/vrrp/transport/na.go` - `BuildNA`, the only IPv6 ND
  message the VRRP plugin sends. No Router Advertisement path exists under
  `internal/plugins/vrrp`.
- [ ] `internal/plugins/vrrp/instance.go` - the instance that reacts to VRRP
  state and tracking changes.

**Behavior to preserve:**
- With no new leaf set and no VRRP group, every advertisement and the final
  advertisement are byte-identical to today.
- The single final advertisement, its rate limit (`ndp.CeaseWait`) and the
  link-down silence.

**Behavior to change:**
- A VRRP IPv6 group in the Active state advertises the virtual router; in
  Backup or Initialize it does not, and it never sends a final advertisement
  from the virtual link-local on a handover.
- A unit may turn the final advertisement off.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Interface RA sender | `internal/plugins/iface/ra/sender_linux.go` `Sender.run`, `sendFinal` | Schedules, answers solicitations and retires one unit's advertisement | extends: the final-advertisement switch is one more field of `RASenderSpec`, read in `sendFinal` |
| RA reconcile | `internal/component/iface/reconcile_ra.go` `RASenderSpec`, `Equal` | Turns config into one sender per unit | extends: carries the new leaf; `Equal` must compare it |
| ND encoder | `ndp.RAConfig`, `ndp.CeaseWait` (core ND package) | Encodes the advertisement and rate limits the final one | extends: used unchanged for the virtual router's advertisement |
| VRRP state machine and announcer | `internal/plugins/vrrp/instance.go`, `internal/plugins/vrrp/transport/na.go` `BuildNA` | Elects Active and Backup, sends unsolicited NA from the virtual addresses | extends: the Active transition starts the virtual router's advertisement, the Backup transition stops it without a final advertisement. The design phase decides whether the VRRP plugin asks the iface RA sender over the bus or owns a sender; a Go call from `vrrp` into `iface` is refused because either plugin can run forked (`ai/doors/system.md`) |
| PPP subscriber RA sender | `raSchedule` in the PPP package | Advertises on subscriber sessions | does not apply: a subscriber session has no VRRP pair and its link ends with the session |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a unit's `router-advertisement` container, and a VRRP IPv6 group on the same interface.
- VRRP state events: Active, Backup, Initialize.

### Transformation Path
1. Config parse builds `RASenderSpec` with the final-advertisement switch.
2. To be designed: how a VRRP state change reaches the sender for the virtual link-local.
3. `Sender.run` sends from the chosen source; `sendFinal` reads the switch.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VRRP plugin to iface RA sender | to be designed: EventBus event or a typed request, never a Go call | No |

### Changed-Wire Census

**Search evidence:** to be written at design. The ICMPv6 Router Advertisement
format does not change; only the source address and whether a final message is
sent.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| ICMPv6 RA | writer | `sender_linux.go` `Sender.run` | to be written | preserve format |

### Integration Points
- to be designed

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be designed |
| No unintended coupling (components stay isolated) | No | to be designed |
| No duplicated functionality (extends existing, does not recreate) | No | to be designed |
| Zero-copy preserved where applicable (refs, not copies) | No | to be designed |
| Registration over hardcoding, outbound | No | to be designed |
| Registration over hardcoding, inbound | No | to be designed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Hosts keep a default route learned from the virtual link-local across a failover when no zero-lifetime advertisement is sent | RFC 9568 Section 8.2.3 and the VyOS T9413 measurement | failover still loses IPv6 | interop test AC-2 | unvalidated |
| A-2 | FRR and keepalived Active Routers advertise the virtual router from the virtual link-local | to be read in their source at design | the interop scenario expects the wrong source | source read named at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Both routers advertise the virtual router during a split brain | two sources for the virtual link-local in a capture | VRRP owns the state; the sender follows only the Active state |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IPv6 hosts lose or keep a stale default router |
| How is it reverted? | single commit revert |
| Who else touches this path? | `plan/release/fixes/spec-rfc4861-ze-owned-must-gaps.md` (Router Lifetime on forwarding loss) and `spec-home-cpe-8-ipv6-cpe-chain` (closed; its CE Router Lifetime matrix is `(*advertiser).routerLifetime` in `internal/plugins/iface/ra/advertiser.go`) edit the same sender |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| unit config with the final advertisement off, then the unit removed | → | `Sender.sendFinal` | `ra-final-advertisement-off` (`test/plugin/`) |
| VRRP IPv6 group goes Active | → | the virtual router's advertisement | `TestVRRPActiveAdvertisesVirtualRouter` (QEMU integration) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | VRRP IPv6 group Active on an interface whose unit has `router-advertisement` | Router Advertisements leave from the virtual link-local, with the unit's configured options (RFC9568-6.4.3-4, RFC9568-8.2.3-1) | the unit's own link-local advertisement keeps running only if the design shows a host needs it; otherwise none |
| AC-2 | The Active Router stops or loses mastership while the peer becomes Active | No advertisement with Router Lifetime 0 leaves from the virtual link-local; a host on the segment keeps its IPv6 default route through the handover (interop against FRR or keepalived) | none |
| AC-3 | VRRP group in Backup or Initialize | No Router Advertisement from the virtual link-local (RFC9568-6.4.2-3) | none |
| AC-4 | Unit with the final advertisement off, then the unit removed or the daemon stopped | No final advertisement is sent; the capture shows the last periodic advertisement and nothing after it | none |
| AC-5 | Unit without the new leaf | The stop sends one advertisement with Router Lifetime 0, as today (RFC 4861 Section 6.2.5) | none |
| AC-6 | The RA show output for a unit with the final advertisement off | The state is visible to the operator | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendFinalOff` | `internal/plugins/iface/ra/sender_linux_test.go` | AC-4, AC-5 | |
| `TestRASenderSpecEqualFinal` | `internal/component/iface/reconcile_ra_test.go` | a change of the leaf updates the sender | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ra-final-advertisement-off` | `test/plugin/` | the operator turns the final advertisement off and restarts | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vrrp-ipv6-ra-failover` | `test/interop/scenarios/` | FRR or keepalived as the other VRRP router, a Linux host | AC-1 to AC-3: the host keeps its default route across a failover | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the final-advertisement leaf
- `internal/component/iface/reconcile_ra.go` - carry the leaf
- `internal/plugins/iface/ra/sender_linux.go` - honor the leaf; virtual-router source
- `internal/plugins/vrrp/instance.go` - start and stop the virtual router's advertisement on state change
- `rfc/short/rfc9568.md` - the three rows leave `{gap}`

## Files to Create
- the functional test `ra-final-advertisement-off` under `test/plugin/`
- the interop scenario `vrrp-ipv6-ra-failover`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| to be answered at design | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| to be answered at design | | | |

## Implementation Steps

1. **Phase: Wiring** -- to be designed

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Targeted tests pass: `./le test integration iface`, `./le test functional plugin`, and the `vrrp-ipv6-ra-failover` scenario through `./le test integration`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
