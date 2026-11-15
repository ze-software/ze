# Spec: rfc-weak-verdict-proof-gaps

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P2 |
| Release reason | Adds proof and changes no producer. AC-1 is owed before the first release: `rfc/short/rfc4271.md` names `route-server` in its Use cases row, and `rfc/profiles/route-server.md` is a release profile. AC-2 may wait until after the first release: the Use cases row of `rfc/short/rfc2205.md` is `-` |
| RFC gaps | RFC4271-5.1.2-2, RFC2205-3-40 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Provenance: AC-11 of `spec-rfc-verdict-test-fix-pass` (closed 2026-11-14,
`plan/learned/051-rfc-verdict-test-fix-pass.md`) needs every `weak` audit
verdict named in an owning spec's acceptance criteria. Each row below has a
guard that exists in the producer, but no tagged unit goes red when the guard
is broken. This spec adds that proof. It changes no producer.

The mechanism specs own the journal row and the producer. The proof is homed
here because their edit gates refuse an edit today for gaps of their own:

- `plan/release/fixes/spec-bgp-refresh-reapplies-export-policy.md` owns the
  RFC4271-5.1.2-2 journal row. Refused: "Incomplete section: ### Changed-Wire
  Census ... omits a speaker" and "AC-9 states only an absence".
- `plan/release/fixes/spec-rsvp-routing-and-interface-integration.md` owns the
  RFC2205-3-40 journal row and the transport. Refused: "Missing required
  section: ## Existing Mechanisms ... (Status design)".

### RFC4271-5.1.2-2

RFC 4271 Section 5.1.2: "When a given BGP speaker advertises the route to an
internal peer, the advertising speaker SHALL NOT modify the AS_PATH attribute
associated with the route."

Producer, read 2026-11-14: `reactorForwardRSSection`
(`internal/component/bgp/reactor/forward_rs.go`) hands `!facts.isEBGP` to
`runEgressPolicyChain`, and gates as-override and the AS_PATH prepend on
`facts.isEBGP`. Internal destinations reach this route-server rail. Every
tagged forward unit passes `rs` false, and the other tagged units call the
chain, the writers or the dry run directly. So changing either guard reddens no
tagged unit (`rfc/audit/rfc4271/RFC4271-5.1.2-2.json`).

### RFC2205-3-40

RFC 2205 Section 3.11.5: "On a router or multi-homed host, the identity of the
interface (real or virtual) on which a diverted message is received, as well as
the IP source address and IP TTL with which it arrived, must also be available
to the RSVP process."

Producer, read 2026-11-14: `receivedIPv4Packet`
(`internal/plugins/rsvpte/transport_linux.go`) reads the IP TTL from `data[8]`,
which is correct. The tagged unit `TestIntegrationRSVPPathCarriage` already
enforces the interface and the source. But the lab sets the frame's IP TTL from
Send_TTL and diverts it before any decrement, so a producer that reported the
RSVP common header's Send_TTL instead would pass too
(`rfc/audit/rfc2205/RFC2205-3-40.json`).

### RFC1332-2.1-3 and RFC1332-2.1-4

The full LCP restart-counter closure found these pre-existing proof gaps on
2026-11-15 when its required fixture admission made their audits stale.
This spec owns their proof as AC-3 and AC-4. They are outside the four LCP
goals and 22 acceptance criteria; no LCP scope was removed.
These two additions concern distinct-polarity and whole-requirement proof.
The unobserved-guard description above applies to the original AC-1/AC-2.

RFC 1332 Section 2.1 ties the maximum transmitted IP packet to the PPP
Information-field maximum and requires fragmentation of larger datagrams.
`pppSession.afterLCPOpen` in `internal/component/l2tp/ppp/session_run.go`
installs the transmit MTU through `Backend.SetMTU` before connecting the
channel. `TestIPMTUInstalledFromNegotiatedMRU` in
`internal/component/l2tp/ppp/rfc1332_mtu_test.go` records one fake-backend MTU
call for peer MRU 1400, requires exactly 1400 and then redundantly rejects a
value above 1400. Both polarity tags use that same input and observation.
The recorded constant-1500 break proves that installed-value assertion.
It does not observe transmitted IP lengths or Linux fragmentation.

The independent audit therefore records `weak` for both requirements. The
LCP work added only `LCP: testLCPConfig()` to admit the existing fixture;
it changed no prior assertion or tag. These findings establish a missing
proof, not a production fragmentation defect. The producer stays unchanged
unless the real-kernel proof exposes such a defect.

### RFC1661-6.4-7 client repeated-draw control

The same 2026-11-15 audit found a pre-existing client proof gap, owned here
as AC-5. In `internal/component/l2tp/pppoeclient/session.go`,
`generateDifferentMagic` correctly rejects a random draw equal to the prior
Magic-Number and tries again. `TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`
in `rfc1661_lcp_reply_test.go` checks the new request and subsequent Echo,
but its random source almost never repeats the old number. Removing the
client's `magic != previous` guard therefore usually leaves that test green.
The server's deterministic repeated-draw test drives another implementation
and cannot prove this client guard.

Retain the positive client wire assertions and native reachability records.
The audit records `weak` until deterministic client proof exercises the old
draw followed by a fresh draw through the real negotiator/helper path and
rejects a targeted removal of that guard. The LCP counter work added only an
explicit Max-Failure fixture value to this unit. It changed neither the
random-number producer nor these assertions. This is outside all 22 LCP
counter ACs and establishes no runtime Magic-Number defect.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/routing-protocols.md` - the doors for the two producers

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` row RFC4271-5.1.2-2; `rfc/full/rfc4271.txt` Section 5.1.2
- [ ] `rfc/short/rfc2205.md` row RFC2205-3-40; `rfc/full/rfc2205.txt` Section 3.11.5

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-14, skeleton survey only)
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `reactorForwardRSSection`, the two `isEBGP` guards
- [ ] `internal/plugins/rsvpte/transport_linux.go` - `receivedIPv4Packet`, `TTL: data[8]`

**Behavior to preserve:** both producers, unchanged.

**Behavior to change:** none. This spec adds tests and records.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Discrimination records | `./le rfc record`, `docs/contributing/rfc-conformance-gates.md` | Observes a break of the producer turning a tagged unit red, and stores the record | Extends: one record per guard |
| Independent re-judge | `./le rfc audit-stamp ... mode rejudge` | Writes a verdict from an agent that did not write the test | Extends: one re-judge per row |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- AC-1: a route forwarded on the route-server rail. AC-2: a diverted RSVP PATH message on the Linux transport.

### Transformation Path
1. The test drives the real entry point, and the record breaks the guard named in its AC.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor → peer wire | the route-server forward rail | [ ] |
| Kernel → RSVP transport | raw socket receive, `receivedIPv4Packet` | [ ] |

### Integration Points
- `internal/component/bgp/reactor/` and `internal/plugins/rsvpte/` test files only

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Route forwarded on the route-server rail to an internal peer | → | `reactorForwardRSSection` | AC-1 unit, named at implementation |
| Diverted PATH with IP TTL unequal to Send_TTL | → | `receivedIPv4Packet` | AC-2 unit, named at implementation |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Row `RFC4271-5.1.2-2`, audit verdict `weak`. A route forwarded by `reactorForwardRSSection` to an internal and to an RFC 7705 migration-internal destination, through an export chain with an as-path-prepend and a remove-private member and with as-override set, plus an external destination as the control in the same fan-out | The test tells the guarded paths apart: toward each internal destination the AS_PATH is byte for byte the received one, and toward the external control it carries the edits. Proven when that unit carries the `RFC4271-5.1.2-2` tag, `./le rfc record` observes it red under a recorded break of each guard (the `!facts.isEBGP` handed to `runEgressPolicyChain`, and the `facts.isEBGP` gate on as-override and the prepend), and an agent that wrote neither re-judges the verdict `enforced` |
| AC-2 | Row `RFC2205-3-40`, audit verdict `weak`. A diverted PATH message whose IP TTL differs from its RSVP common header Send_TTL, raw-injected or sent through a non-RSVP forwarding hop in the lab | The test tells the IP TTL from the Send_TTL: `Packet.TTL` equals the IP TTL. Proven when the tagged unit asserts it, `./le rfc record` observes it red with the TTL read moved from `data[8]` to the Send_TTL octet, and an agent that wrote neither re-judges the verdict `enforced` |
| AC-3 | Row `RFC1332-2.1-3`, audit verdict `weak`. A real connected PPP unit with negotiated peer MRU 1400, datagrams at and above that boundary, and a default-MRU 1500 control | Observe transmitted IP packet lengths against the PPP Information-field capacity, excluding the Protocol field and framing. Distinct observations must reject a fixed 1500 limit and a wrongly reduced default limit. Preserve the existing installed-MTU test and records. Tagged proof, meaningful producer discrimination and an independent whole-requirement rejudgment are required |
| AC-4 | Row `RFC1332-2.1-4`, audit verdict `weak`. A real Linux PPP unit with MTU 1400 receives an oversized DF-clear IPv4 datagram, plus a smaller datagram as the control | Observe actual fragments on the PPP wire, each within the negotiated Information-field capacity, and reassemble the original payload. The smaller control remains unfragmented. Preserve existing evidence, add distinct tagged observations and record a meaningful break that makes the fragmentation assertion fail. An independent auditor rejudges the whole requirement |
| AC-5 | Row `RFC1661-6.4-7`, audit verdict `weak`. The PPPoE client receives its looped-back Magic Nak with deterministic entropy that yields the prior Magic-Number and then a fresh nonzero value | The real client negotiator/helper path rejects the repeated draw, sends the fresh value in its next request and carries the negotiated value to Echo. A fresh-first-draw control also succeeds. Retain the existing positive wire assertions and records. A targeted removal of the client `magic != previous` guard must fail the new assertion before an independent rejudgment restores whole-requirement enforcement |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| named at implementation | `internal/component/bgp/reactor/` | AC-1 | planned |
| named at implementation, or `TestIntegrationRSVPPathCarriage` extended | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | AC-2 | planned |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| none | - | The proof is at the producer; the guards are not new behaviour | |

## Files to Modify
- Test files only, in `internal/component/bgp/reactor/` and `internal/plugins/rsvpte/`
- `rfc/discrimination/` and `rfc/audit/` records written by the tools

## Implementation Steps
1. Write each unit, tag it, and run it green over the current producer.
2. Record the break with `./le rfc record`.
3. Independent re-judge of each verdict.

## Known Limitations
- Skeleton: test shapes are chosen at implementation.

## RFC Documentation (Scope: protocol)
Each tag claims only what its test body checks.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 and AC-2 demonstrated
- [ ] `./le test unit pkg path ./internal/component/bgp/reactor/` passes
- [ ] `./le test unit pkg path ./internal/plugins/rsvpte/` passes (the AC-2 unit runs in QEMU with its integration tag)
- [ ] `./le rfc check` clean for `rfc4271` and `rfc2205`
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
