# Spec: ipoe-subscriber-sessions

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | IPoE is an access method Ze does not offer; L2TP and PPPoE subscriber access ship without it |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-dhcp-server-renew-rebind-time.md`, `plan/post-release/features/spec-dhcp-server-client-classes.md` (option 82 reading and echo) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Many access networks run IP over Ethernet without PPP: a subscriber's CPE sends
DHCP on its VLAN, the BNG authenticates the line (by MAC address, by VLAN, or by
the relay's option 82 circuit-id and remote-id), hands out an address, applies
the subscriber's rate limit, and accounts for the session in RADIUS. VyOS
offers this as `service ipoe-server` over accel-ppp. Ze offers L2TP and PPPoE
subscriber access only.

The fit is reasonable because Ze already has the parts a session needs, shared
by L2TP and PPPoE through the subscriber model: one session type with an access
type, RADIUS authentication and accounting with CoA and Disconnect, an address
pool, a shaper and the show commands. IPoE adds a third access type whose
session starts on DHCP rather than on PPP.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, `service ipoe-server`
(accel-ppp `ipoe` module). Ticket T9289 (`git log --grep=T9289`, commits
`896bb394f` and `3a4dcfb81`) added `lease-time`, `max-lease-time`,
`renew-time` and `rebind-time`. Its pitfalls become acceptance criteria here:

| VyOS lesson | Ze acceptance criterion |
|-------------|-------------------------|
| Only `rebind-time` set: the derived renewal time could exceed it | AC-6: the T1 < T2 < lease order is checked on the values sent (shared with `spec-dhcp-server-renew-rebind-time.md`) |
| With an external DHCP relay, the lease timings come from the external server, and the configured ones only fill in | AC-7: in relay mode Ze states which side owns the timings, and refuses a configuration that sets timings it will not send |
| A lease time plus the default margin overflowed the server's session expiry | AC-8: the session expiry is computed without overflow, and its bound is refused at commit |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - the shared session type and handler registries
  → Constraint: a lifecycle consumer subscribes to the subscriber topic alone
  → Constraint: the auth and pool handler types are PPP-typed today (below)
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` - the RADIUS attributes a subscriber session sends
- [ ] `docs/architecture/provisioning/dhcp-server.md` - the existing DHCPv4 server

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2131.md`, `rfc/short/rfc2132.md` - the DHCP server role
- [ ] `rfc/short/rfc2865.md` - RADIUS Access-Request for a session with no PPP credentials
- RFC 3046 has no summary and none is written here: no release profile lists
  it (`ai/META-DECISIONS.md`). Its text is `rfc/full/rfc3046.txt`.

RFC text read for this spec:

- RFC 3046 Section 2.2: "DHCP servers claiming to support the Relay Agent
  Information option SHALL echo the entire contents of the Relay Agent
  Information option in all replies."

**Key insights:**
- The design question is where the DHCP exchange of an IPoE session lives. Two
  homes exist: the `dhcpserver` plugin, an edge plugin nothing may import, and
  the subscriber component. A shared DHCPv4 codec moves to a core leaf before
  either imports it (`ai/doors/system-plugins.md`, "Always-on code never
  imports a gated plugin").

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/subscriber/session.go` - `AccessType` has two
  values, `pppoe` and `l2tp`; `Session.PPPKey` branches on them.
- [ ] `internal/component/l2tp/subscriber/handler_registry.go` -
  `AuthHandler` takes a `ppp.EventAuthRequest`, `PoolHandler` a
  `ppp.EventIPRequest`; one slot each. An IPoE session has no PPP request.
- [ ] `internal/plugins/dhcpserver/segment.go` - the DHCPv4 server state
  machine: DISCOVER, REQUEST, RELEASE, DECLINE, one lease table per subnet.

**Behavior to preserve:**
- L2TP and PPPoE sessions, their auth, pool, shaper and accounting, unchanged.
- The standalone DHCP server.

**Behavior to change:**
- None today; IPoE is new.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Subscriber session model | `internal/component/l2tp/subscriber/session.go` `Session`, `AccessType`; `registry.go` | One session type and registry for L2TP and PPPoE | extends: a third access type, `ipoe` |
| Auth, pool and shaper handler registries | `internal/component/l2tp/subscriber/handler_registry.go` | One PPP-typed handler per kind | extends: the request types become transport-generic, so RADIUS and the pool serve an IPoE session; the one-slot trap (`ai/doors/access-security.md`, Traps) is fixed or kept, by design decision |
| RADIUS subscriber auth and accounting | `internal/component/l2tp/plugins/authradius` | Access-Request, accounting, CoA and Disconnect for PPP sessions | extends: an IPoE Access-Request carries the MAC or option 82 identity as User-Name |
| Address pool and shaper plugins | `internal/component/l2tp/plugins/pool`, `internal/component/l2tp/plugins/shaper` | Allocate and shape per session | extends: used unchanged once the request type is generic |
| DHCPv4 server | `internal/plugins/dhcpserver` `segment`, `dhcpHandler.buildReply` | Serves leases from configured subnets | extends or does not apply, decided at design: its codec is the candidate to move to a core leaf; its lease table is per subnet, where an IPoE lease belongs to a session |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: a DHCPDISCOVER on an IPoE access interface or VLAN, direct or relayed with option 82.
- Config: an IPoE access container naming interfaces, the identity used for auth, pool and gateway.

### Transformation Path
1. DISCOVER creates a session in `authenticating`.
2. The auth handler answers (RADIUS or local); a reject leaves the client unanswered.
3. The pool handler allocates; OFFER and ACK carry the address and the gateway.
4. The session goes `active`; the shaper applies RADIUS rates; accounting starts.
5. RELEASE, lease expiry, idle timeout, CoA Disconnect or a config removal ends the session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IPoE access to subscriber registry | subscriber events | No |
| Subscriber to RADIUS plugin | the handler registry, typed request | No |

### Changed-Wire Census

**Search evidence:** to be written at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| DHCPv4 | writer and reader | to be designed | to be written | new |
| RADIUS Access-Request | writer | `authradius` | to be written | new identity source |

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
| A-1 | FreeRADIUS accepts an Access-Request with no User-Password for MAC or circuit-id auth | to be read in FreeRADIUS and accel-ppp `ipoe` source at design | auth needs a fixed password, as accel-ppp offers | interop | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two DHCP servers answer on one interface (IPoE and the standalone server) | duplicate OFFERs | commit refuses an interface claimed by both |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IPoE subscribers get no address, or the wrong rate |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | every subscriber spec under `internal/component/l2tp` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| DHCPDISCOVER on an IPoE interface | → | session creation and auth | `ipoe-session-local` (`test/l2tp/` or the suite the design names) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | DISCOVER from an authorized MAC on an IPoE interface | OFFER and ACK with a pool address; `show subscriber sessions` lists it with access type `ipoe` | none |
| AC-2 | RADIUS Access-Accept with a rate | the shaper applies it; accounting Start, Interim and Stop reach the server | none |
| AC-3 | RADIUS Access-Reject | no OFFER; the reject is logged with the identity and the RADIUS reason | none |
| AC-4 | relayed DISCOVER with option 82 and identity `circuit-id` | User-Name carries the circuit-id; every reply echoes option 82 (RFC 3046 Section 2.2) | none |
| AC-5 | RELEASE, lease expiry, CoA Disconnect | the session ends, the address returns to the pool, accounting Stop is sent | none |
| AC-6 | lease timings with T1 not below T2, or T2 not below the lease | commit refused, naming the values sent | none |
| AC-7 | relay mode to an external DHCP server with local timings set | commit refused or the timings are sent, by design decision; never silently ignored | none |
| AC-8 | the largest accepted lease | session expiry computed without overflow; a larger value refused at commit | none |
| AC-9 | an interface configured for both IPoE and the DHCP server | commit refused, naming both | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be designed | `internal/component/l2tp/` | AC-1 to AC-9 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipoe-session-local` | to be designed | a CPE gets an address and a rate | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ipoe-radius` | `test/interop/scenarios/` | FreeRADIUS, a dhclient CPE, a relay inserting option 82 | AC-2, AC-4, AC-5 | |

## Files to Modify
- `internal/component/l2tp/subscriber/session.go` - access type `ipoe`
- `internal/component/l2tp/subscriber/handler_registry.go` - transport-generic requests
- `internal/component/l2tp/plugins/authradius` - IPoE identity
- `docs/architecture/l2tp/subscriber-session-model.md` - the third access type

## Files to Create
- the IPoE access package, its YANG and its tests, placed at design

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
- [ ] Targeted tests pass: `./le test functional l2tp`, `./le test integration interop-radius-subscriber`, and the `ipoe-radius` scenario

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
