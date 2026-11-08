# Spec: pppoe-client-connect-on-demand

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a dialing mode absent from the supported release: a PPPoE session that comes up on the first packet and goes down after an idle time. The always-on client stays shipped and required |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

On a metered or backup uplink, an operator wants the PPPoE session to dial
only when traffic needs it and to hang up after an idle time. VyOS offers
`connect-on-demand` and `idle-timeout` (pppd's `demand` and `idle`). Ze's
client dials at start and redials on loss, always. VyOS's last fix in this
area (T9533) shows the trap: in demand mode the default route is what
attracts the first packet, and VyOS dropped it, so nothing ever dialed, and
`no-default-route` was ignored.

| # | Operator need |
|---|---------------|
| 1 | `connect-on-demand`: no session until a packet is routed to the PPPoE interface |
| 2 | `idle-timeout <seconds>`: hang up after that long with no traffic, then wait for the next packet |
| 3 | Default route through the interface while it waits, unless `no-default-route` is set |
| 4 | Static routes through the interface work while it waits |
| 5 | `show interface pppoe-client` says waiting, dialing, up or idle |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - the `pppoe-client` created kind
  → Constraint: today the PPP unit device exists only while a session is up, and static routes through it wait for the device (`iface.DeclaredDynamicNames`). Demand mode needs the device to exist while no session is up, which changes that statement
  → Constraint: `source-interface` is resolved once into an `InterfaceBinding` (`pppoe.ResolveInterface`); demand mode keeps that
- [ ] `ai/doors/access-security.md` - the PPPoE client against accel-ppp is the interop proof (`./le test deployment docker-pppoe-accel`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/pppoe_client.go` - `PPPoEClientConfig` (name, source interface, credentials, service and AC name, `NoDefaultRoute`, `RoutePriority`, `Timing`, IPv6); `run` loops `runSession` and redials after `reconnectDelay`; there is no idle or demand state
- [ ] `internal/component/l2tp/ppp/devppp_linux.go` - `DevPPPSetup` attaches the channel and creates the unit (named through RTM_NEWLINK kind `ppp`), deferring PPPIOCCONNECT to after LCP; `SetIPv4Mode` sets the NP mode of IPv4 on a unit without detaching it
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list pppoe-client`: source-interface, authentication, service-name, ac-name, `no-default-route`, `route-priority`, keepalive, reconnect, ipv6; no demand or idle leaf

**Behavior to preserve:**
- The always-on client, its redial timing, keepalive and IPv6.

**Behavior to change:**
- With `connect-on-demand`, the PPP unit exists before any session, queues the trigger packet, and a session is dialed on the first packet; `idle-timeout` ends a session with no traffic.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| PPPoE client loop | `pppoe_client.go` `PPPoEClient.run`, `runSession` | Dial, run, redial forever | Extends with a demand state: wait for a packet instead of dialing at once, and an idle state |
| PPP unit and NP mode | `devppp_linux.go` `DevPPPSetup`, `SetIPv4Mode` | Creates the unit per session; switches IPv4 NP mode | Extends: create the unit without a channel at start, NP mode QUEUE until IPCP is up, then PASS; attach the channel per session (pppd demand mode does the same) |
| Default route install after IPCP | `pppoe_client.go` `applyPPPoEIPv4`, `RoutePriority`, `NoDefaultRoute` | Installs the default route after IPCP unless `no-default-route` | Extends: in demand mode the route is installed at start through the waiting unit, still unless `no-default-route` (VyOS T9533) |
| Dynamic device names for static routes | `iface.DeclaredDynamicNames` | Lets static routes wait for a device that comes and goes | Extends: in demand mode the device is present while waiting, so routes install at once |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `connect-on-demand` and `idle-timeout` on `interface pppoe-client <name>`.

### Transformation Path
1. Start: create the PPP unit under the configured name with no channel, a placeholder point-to-point address pair, NP mode QUEUE; install the default route through it unless `no-default-route`.
2. Wait: read the unit for the first queued packet (the trigger).
3. Dial: discovery, LCP, auth, attach the channel to the existing unit (PPPIOCCONNECT), IPCP; replace the placeholder addresses and routes with the negotiated ones; NP mode PASS so the queued packets go out.
4. Idle: no traffic in either direction for `idle-timeout` ends the session with PADT; the unit stays, NP mode back to QUEUE, routes back on the placeholder.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to kernel | /dev/ppp unit ioctls, PPPIOCSNPMODE, RTM_NEWLINK kind ppp | No |
| Ze to access concentrator | RFC 2516 discovery and RFC 1661 LCP, unchanged | No |

### Changed-Wire Census

**Search evidence:** no PPPoE or PPP wire change; `show interface pppoe-client` gains states (census owed at design).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- `PPPoEClient.run`, `DevPPPSetup`, the route install and withdraw helpers.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A PPP unit can exist with no channel and queue IPv4 packets in NPMODE_QUEUE, readable on the unit descriptor, and a channel can be connected to it later | pppd demand mode (`demand.c`) and Linux `ppp_generic.c` | Demand needs another trigger mechanism | Read pppd `demand.c` and `ppp_generic.c` `ppp_send_frame`, quote them; integration test | unvalidated |
| A-2 | The access concentrator assigns an address other than the placeholder, so routes and connections opened during the wait must move to the new address | pppd demand behavior | Stale source addresses | Integration test against accel-ppp | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Background traffic (NTP, DNS, BGP keepalives) dials the link forever | Session never idles | Document; a later filter on which packets trigger and which reset the idle timer (pppd `active-filter`) is a follow-up spec if wanted |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The uplink never dials, or never hangs up |
| How is it reverted? | Remove `connect-on-demand` |
| Who else touches this path? | The PPPoE client specs in `plan/release/fixes/` (home-cpe 12, 13, 17: LCP renegotiation, IPCP DNS, keepalive) change the same loop; this spec starts after they land |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pppoe-client` with `connect-on-demand` committed, then a ping to a remote address | → | demand wait, trigger, dial | `test/plugin/pppoe-client-connect-on-demand.ci` (`option=needs-linux`, Ze access concentrator in a netns) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `connect-on-demand`, no traffic | No PADI is sent; the device exists; a default route through it exists | None |
| AC-2 | Then a ping to an address beyond the concentrator | Discovery starts, the session comes up, the ping's queued packet is delivered after IPCP (the first echo reply arrives) | Packets dropped before IPCP beyond the queue bound, by design |
| AC-3 | `connect-on-demand` with `no-default-route` | No default route is installed at start or after IPCP; a static route through the device still triggers the dial (VyOS T9533) | None |
| AC-4 | `idle-timeout 60`, no traffic for 60 s after the session is up | PADT sent; session down; device and routes stay; the next packet dials again | None |
| AC-5 | Traffic keeps flowing | No hang-up | None |
| AC-6 | Dial fails (no concentrator) | The packet that triggered it is dropped; the next packet retries, paced by the reconnect timing, never a tight loop | None |
| AC-7 | `connect-on-demand` removed on a waiting client | The client dials at once as an always-on client | None |
| AC-8 | `idle-timeout` without `connect-on-demand` | Behavior fixed at design (VyOS allows idle on always-on, which redials at once); never silently ignored | None |
| AC-9 | `show interface pppoe-client` | Reports waiting, dialing, up and idle-since | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets a backup uplink to dial on demand | config, waiting unit, trigger, dial | `test/plugin/pppoe-client-connect-on-demand.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPPPoEDemandStateMachine` | `internal/component/iface/pppoe_client_demand_test.go` | AC-1, AC-4, AC-6, AC-7 with a fake dialer | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| idle-timeout | 1..86400 s (settled at design) | 86400 | 0 | 86401 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-client-connect-on-demand` | `test/plugin/` | AC-1, AC-2, AC-4 | |
| `pppoe-client-demand-no-default-route` | `test/plugin/` | AC-3 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| the existing accel-ppp deployment test | `./le test deployment docker-pppoe-accel` | accel-ppp | A demand dial and an idle hang-up against another concentrator | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the leaves
- `internal/component/iface/pppoe_client.go` - demand and idle states
- `internal/component/l2tp/ppp/devppp_linux.go` - unit without a channel, connect later
- `docs/architecture/iface/logical-name-resolution.md` - the pppoe-client device lifetime paragraph

## Files to Create
- `test/plugin/pppoe-client-connect-on-demand.ci`, `test/plugin/pppoe-client-demand-no-default-route.ci`

## Implementation Steps

1. **Phase: Wiring** - leaves, parse, failing `.ci`.
2. **Phase: Waiting unit** - unit without channel, NP mode QUEUE, routes.
3. **Phase: Trigger and dial** - read the trigger, connect the channel, swap addresses.
4. **Phase: Idle** - idle timer and hang-up.

Proof: `./le test unit plugins`, `./le test pppoe`, `./le test deployment docker-pppoe-accel`.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`.

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T9533, `773ac584b` "pppoe: Fix default route handling in connect-on-demand mode" | The demand config still read a `default-route` option a migration had removed, so pppd never got a default route: nothing attracted a packet, nothing dialed, and `no-default-route` had no effect. Now the default route is installed only when `no-default-route` is not set. AC-1, AC-3 |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- IPv6-triggered dialing waits on the IPv6CP design of the client; the first version triggers on IPv4 and IPv6 alike only if A-1 holds for both NP modes.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
