# Spec: dhcp-server-renew-rebind-time

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | An operator choice of T1 and T2 is an absent capability; the fixed fractions Ze sends today are correct RFC 2131 behavior, so nothing shipped is wrong |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's DHCPv4 server always sends the renewal time T1 (option 58) as half the
lease and the rebinding time T2 (option 59) as seven eighths of it. An operator
cannot change them. Operators set them when:

- clients behind a slow or flapping relay must start renewing earlier than half
  the lease, so a lost renewal leaves time for more attempts;
- a long lease is wanted for address stability, while clients still check in
  often (short T1 under a long lease);
- a migration from another server must keep the client timing that server sent.

The work adds `renew-time` and `rebind-time` leaves to a subnet. Ze sends the
configured value in place of the derived one, and refuses a configuration
where the times are out of order.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, ticket T9289 (`git log
--grep=T9289`): `896bb394f` (`ipoe-server: T9289: expose accel-ppp lease-time
DHCP options`) and `3a4dcfb81` (`ipoe-server: T9289: warn about derived
renew-time and relay lease timings`). VyOS added the leaves to its IPoE server
(accel-ppp), not to its Kea DHCP server. The lessons carry over:

| VyOS lesson | Ze acceptance criterion |
|-------------|-------------------------|
| Only `rebind-time` set: the derived renewal time could exceed it, and VyOS first caught only a configured `renew-time` (`3a4dcfb81`) | AC-4: the order check runs on the values Ze will send, derived or configured |
| Values are bounded to what the backend accepts (signed 32-bit there) | AC-6: YANG ranges bound each leaf; the lease bound stays with `lease-time` |
| A relayed lease takes its timings from the external server | Not applicable: Ze's server answers relayed requests itself. The IPoE spec owns the relay case (`plan/post-release/features/spec-ipoe-subscriber-sessions.md`) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/provisioning/dhcp-server.md` - the DHCP server design
  → Constraint: a PXE client takes the same options; a T1 change reaches it too

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2131.md` - Section 4.4.5 rows RFC2131-4.4.5-2 and RFC2131-4.4.5-3
- [ ] `rfc/short/rfc2132.md` - Sections 9.11 and 9.12

RFC text read for this spec (`rfc/full/rfc2132.txt`, `rfc/short/rfc2131.md`):

- RFC 2132 Section 9.11: "This option specifies the time interval from address
  assignment until the client transitions to the RENEWING state." "The code for
  this option is 58, and its length is 4."
- RFC 2132 Section 9.12: "This option specifies the time interval from address
  assignment until the client transitions to the REBINDING state." "The code
  for this option is 59, and its length is 4."
- RFC 2131 Section 4.4.5 (rows RFC2131-4.4.5-2 and 4.4.5-3): "T1 MUST be
  earlier than T2", "which, in turn, MUST be earlier than the time at which the
  client's lease will expire."

**Key insights:**
- The Section 4.4.5 ordering binds the client, but a server that sends T1 at or
  past T2 hands every client a value it must reject or clamp. Ze refuses that
  configuration at verify rather than send it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/handler.go` - `dhcpHandler.buildReply`
  appends option 51 from `subnet.LeaseTimeSec`, then option 58 as half of it
  and option 59 as seven eighths of it, in every OFFER and ACK.
- [ ] `internal/plugins/dhcpserver/config.go` - `subnetConfig` holds
  `LeaseTimeSec` and no T1 or T2 field.
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - subnet
  leaf `lease-time` (60 to 604800, default 86400), whose description states the
  two fractions. No renew or rebind leaf.

**Behavior to preserve:**
- With neither leaf set, option 58 and option 59 carry today's values, byte for byte.
- Option order in the reply and the PXE options.

**Behavior to change:**
- A configured `renew-time` or `rebind-time` replaces the derived value.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Reply builder | `internal/plugins/dhcpserver/handler.go` `buildReply` | Emits options 51, 58 and 59 for each subnet | extends: reads the configured T1 and T2 before it derives them |
| Subnet config parse and verify | `internal/plugins/dhcpserver/config.go` `parseConfig`, `subnetConfig` | Parses and refuses a subnet at verify | extends: parses the two leaves and checks their order against the lease |
| Arbitrary DHCP options | `plan/post-release/features/spec-dhcp-pool-options.md` AC-3 | Planned: refuses an option code Ze already emits | does not apply: 58 and 59 stay emitted by Ze, so that spec keeps refusing them; these leaves are the one way to set them |
| Lease disposition | `plan/proposals/spec-dhcp-lease-disposition.md` | Owns the lease cap and the T2 arithmetic for long or infinite leases | does not apply here; when it raises the cap, its T1 and T2 arithmetic must keep AC-4 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `service dhcp-server shared-network <n> subnet <p> renew-time | rebind-time`.

### Transformation Path
1. Config parse fills the subnet's T1 and T2, unset meaning derived.
2. Verify computes the values Ze will send and checks T1 < T2 < lease.
3. `buildReply` writes option 58 and option 59 from those values.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin | the plugin's own config root | No |

### Changed-Wire Census

**Search evidence:** to be written at design. Options 58 and 59 keep their
format; only their value changes when a leaf is set.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| DHCP options 58, 59 | writer | `handler.go` `buildReply` | read for this spec | preserve format |

### Integration Points
- `subnetConfig` in the plugin's config parse.

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
| A-1 | ISC dhclient, systemd-networkd and busybox udhcpc honor options 58 and 59 | to be read in their source at design | the leaves change nothing for that client | interop capture | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A lease-time change makes a stored T1 or T2 invalid | commit refused | the refusal names the three values and the leaf to change |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | clients renew at the wrong time |
| How is it reverted? | single commit revert |
| Who else touches this path? | `spec-dhcp-pool-options.md`, `spec-dhcp-server-client-classes.md` (same reply builder) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| subnet `renew-time 600` | → | `buildReply` option 58 | `dhcp-renew-rebind-time` (`test/plugin/`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `lease-time 3600`, `renew-time 600`, `rebind-time 1800` | OFFER and ACK carry option 58 = 600 and option 59 = 1800 | none |
| AC-2 | `lease-time 3600`, `renew-time 600`, no `rebind-time` | option 58 = 600, option 59 = 3150 (seven eighths, as today) | none |
| AC-3 | neither leaf set | options 58 and 59 byte-identical to today | none |
| AC-4 | values Ze would send violate T1 < T2 < lease, configured or derived (for example `rebind-time 100` alone under `lease-time 86400`, which leaves the derived T1 43200 above it) | commit refused; the message names the subnet, the three values Ze would send, and the leaf to change | none |
| AC-5 | `renew-time` equal to `rebind-time`, or `rebind-time` equal to `lease-time` | refused (the order is strict) | none |
| AC-6 | a value of 0 or above the lease maximum | refused by the YANG range | none |
| AC-7 | `show` of the subnet config | both leaves round-trip | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRenewRebindOptions` | `internal/plugins/dhcpserver/handler_test.go` | AC-1 to AC-3 | |
| `TestRenewRebindOrderRefused` | `internal/plugins/dhcpserver/config_test.go` | AC-4, AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| renew-time | 1 to lease-time minus 2 | lease-time minus 2 | 0 | at or above rebind-time |
| rebind-time | 2 to lease-time minus 1 | lease-time minus 1 | at or below renew-time | at or above lease-time |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcp-renew-rebind-time` | `test/plugin/` | the operator sets the times and a client sees them on the wire | |

## Files to Modify
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - the two leaves; the `lease-time` description
- `internal/plugins/dhcpserver/config.go` - parse and verify
- `internal/plugins/dhcpserver/handler.go` - `buildReply`
- `docs/architecture/provisioning/dhcp-server.md` and the DHCP server guide page - the two leaves

## Files to Create
- the functional test `dhcp-renew-rebind-time` under `test/plugin/`

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
- [ ] Targeted tests pass: `./le test functional dhcp` and the `dhcpserver` package test

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
