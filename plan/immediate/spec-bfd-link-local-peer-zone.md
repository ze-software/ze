# Spec: bfd-link-local-peer-zone

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A BFD session to an IPv6 LINK-LOCAL peer cannot be selected for a packet that
carries no discriminator, in either direction, because the address ze holds and
the address ze observes are in different FORMS.

Found during the round-13 review of `spec-bgp-bfd-strict`. The mechanism is
CONFIRMED by reading the producers; the operator impact is PLAUSIBLE rather than
measured, because an Active session still converges through `byDiscr` once the
peer answers.

This is a DIFFERENT failure class from the five repairs that spec made. Those
were about UNSETNESS: a session key field the client left empty, which
`api.SessionRequest.Canonical` could not derive, against a packet that carried a
value. The matching rule those repairs produced is general over that class and
is stated once in `internal/component/bfd/engine/engine.go`. It is narrower than
the class that produced them, and this is the other half: both sides hold the
same address, in two forms that do not compare equal.

## Required Reading

- `internal/component/bfd/transport/udp.go` -- `readLoop`, which builds `Inbound.From`
- `internal/component/bfd/transport/udp.go` -- `(*UDP).Send`
- `internal/component/bfd/engine/engine.go` -- `firstPacketKey`, and the matching rule above it
- `internal/component/bfd/engine/loop.go` -- `handleInbound`
- `rfc/full/rfc5881.txt` Section 2, and RFC 4007 Section 6 on zone indices
- `docs/architecture/bfd.md` -- "One session per neighbor, whatever asks for it"

## Current Behavior (MANDATORY)

Source files read for the statement below:

- [ ] `internal/component/bfd/transport/udp.go`
- [ ] `internal/component/bfd/engine/engine.go`
- [ ] `internal/component/bfd/engine/loop.go`
- [ ] `internal/component/bfd/api/events.go`

A received packet's source address comes from `raddr.Addr().Unmap()`. For an
IPv6 link-local source the kernel reports a ZONE, so `in.From` is
`fe80::1%eth0`; `Unmap` does not remove it. The session's `Peer` comes from
configuration, parsed by `netip.ParseAddr` from a leaf an operator writes
without a zone, so the key holds `fe80::1`.

`firstPacketKey` compares `Peer` exactly, and `Peer` is deliberately never
relaxed: it is the one field a session cannot leave unset, which the parity test
`TestFirstPacketKeyMirrorsEveryKeyField` records. So every relaxation misses,
and no packet whose Your Discriminator is zero can select the session.

`(*UDP).Send` drops the zone in the other direction, so a packet ze sends to a
link-local peer has no zone to route by. Between the two, link-local BFD reads
as unbuilt rather than as broken in one place.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BFD Control packet arriving on UDP 3784 from an IPv6 link-local source, and
  a configured session naming that peer.

### Transformation Path
1. `readLoop` builds `Inbound.From` from `raddr.Addr().Unmap()`, zone included.
2. `handleInbound` builds a `firstPacketKey` from it and walks the relaxations.
3. `firstPacketIndex` built the session's key from `api.Key.Peer`, zone absent.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel to transport | recvmsg source address, zone-qualified for link-local | No |
| Transport to engine | `transport.Inbound` | No |
| Config to engine | `api.SessionRequest.Peer`, parsed from a YANG leaf | No |

### Integration Points
- `firstPacketKey` and the matching rule above it - the reconciliation belongs
  beside that rule, which already owns what makes two keys the same session.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a Control packet from `fe80::1%eth0` with Your Discriminator 0 | → | `Loop.handleInbound` | `TestFirstPacketSelectsALinkLocalPeer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a session configured to an IPv6 link-local peer, and a first packet from it | the session is selected, so the zone is reconciled between the key and the observation rather than being compared raw |
| AC-2 | a session to a link-local peer on eth0 and another to the SAME address on eth1 | a packet from either link selects its own session, which is what the zone is FOR and what makes dropping it unsafe |
| AC-3 | ze sends to a link-local peer | the packet leaves the interface the session names, which `Send` cannot do today |
| AC-4 | the reconciliation | it is stated once, beside the matching rule, rather than as a case inside each comparison |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFirstPacketSelectsALinkLocalPeer` | `internal/component/bfd/engine/rfc5881_test.go` | AC-1, a zone-qualified source selects the session configured without one | |
| `TestTwoLinkLocalSessionsStayApart` | `internal/component/bfd/engine/rfc5881_test.go` | AC-2, the zone still separates two sessions on one address | |
| `TestSendKeepsTheZoneForALinkLocalPeer` | `internal/component/bfd/transport/udp_test.go` | AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A, no numeric input | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bfd-first-packet-link-local` | `test/bfd/bfd-first-packet-link-local.ci` | an operator configures BFD to a link-local neighbor and the session comes up | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bfd-link-local-frr` | `test/interop/scenarios/` | FRR bfdd | a second implementation's link-local BFD session reaches Up against ze | |

## Files to Modify
- `internal/component/bfd/engine/engine.go` - the reconciliation, beside the matching rule
- `internal/component/bfd/transport/udp.go` - `Send`, which drops the zone outbound
- `docs/architecture/bfd.md` - the session-identity section states what a zone means to a key

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- reach the defect from the entry point
   - Tests: `TestFirstPacketSelectsALinkLocalPeer`
   - Files: `internal/component/bfd/engine/rfc5881_test.go`
   - Verify: the test fails against today's exact comparison, which is what
     establishes the impact this spec records as plausible rather than measured
2. **Phase: Reconcile the form** -- state once, beside the matching rule, what a
   zone means to a key, and apply it where the key is built
   - Tests: `TestFirstPacketSelectsALinkLocalPeer`, `TestTwoLinkLocalSessionsStayApart`
   - Files: `internal/component/bfd/engine/engine.go`
3. **Phase: The send direction** -- keep the zone on the way out
   - Tests: `TestSendKeepsTheZoneForALinkLocalPeer`
   - Files: `internal/component/bfd/transport/udp.go`
4. **Phase: Prove it on a kernel** -- the `.ci` and the interop scenario
   - Tests: `bfd-first-packet-link-local`, `bfd-link-local-frr`
   - Files: `test/bfd/`, `test/interop/scenarios/`

## Known Limitations

The impact is PLAUSIBLE, not measured: an Active session that transmits still
brings the peer to Up through `byDiscr`, so the visible failure is confined to
the passive and zero-discriminator paths until someone runs it. Establishing
that is the first implementation step, and it belongs in the QEMU guest, where
`test/bfd/bfd-first-packet-pktinfo.ci` already proves the surrounding path.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
RFC 5881 Section 2 scopes single-hop BFD to one link, and RFC 4007 Section 6
defines the zone index that names it. Whatever reconciliation lands, the
sentence above it says which of the two documents makes the zone part of the
peer's identity and which makes it part of the link's.

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

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Architectural Verification table filled, including registration over hardcoding
