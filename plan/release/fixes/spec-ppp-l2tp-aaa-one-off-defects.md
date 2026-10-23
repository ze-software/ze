# Spec: PPP, L2TP, PPPoE and RADIUS one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Each row is incorrect shipped subscriber behaviour (LCP and NCP negotiation, L2TP teardown and dialling, RADIUS attribute handling) |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 15 that sit in PPP, L2TP, PPPoE,
the subscriber auth registry and RADIUS. Each row is a claim until a red test
reproduces it at HEAD; a row found fixed moves to "Rows found already fixed".

Left to root-cause specs: `verifyResponseMessageAuthenticator` (S2, absent
attribute verifies as true).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and the L2TP door named in `ai/doors/README.md`
- [ ] `docs/architecture/l2tp/` pages named by `ai/CODE-TO-DOCS.md` for each file below

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc1661.txt` Sections 4.1, 5.3, 5.4, 6.4
- [ ] `rfc/full/rfc2865.txt` Sections 5.8, 5.26
- [ ] `rfc/full/rfc2661.txt` (CDN handling)

## Current Behavior (MANDATORY)

Spot-checked on 2026-10-23: `EncodeVSA` in `internal/component/radius/attr.go`
still returns the whole attribute (Type and Length included) while `DecodeVSA`
reads from the Vendor-Id, so row 2026-09-20 is present.

- [ ] `internal/component/l2tp/pppoeclient/session.go` - `negotiateLCP`
- [ ] `internal/component/l2tp/ppp/lcp_options.go` - `NegotiatePeerOptions`
- [ ] `internal/component/l2tp/ppp/ppp_fsm.go` - `LCPDoTransition`, `codeToEvent`
- [ ] `internal/component/l2tp/ppp/ncp.go` - `absorbIPCPReject`
- [ ] `internal/component/l2tp/ppp/ipv6cp.go` - `generateIPv6CPInterfaceID`
- [ ] `internal/component/l2tp/session_fsm.go` - `handleCDN`
- [ ] `internal/component/l2tp/outgoing_call.go` - `PlaceOutgoingCall`
- [ ] `internal/component/l2tp/subsystem.go` - metrics binding
- [ ] `internal/component/l2tp/config.go` - `ExtractParameters`
- [ ] `internal/component/l2tp/subscriber/` - `RegisterAuthHandler`
- [ ] `internal/component/l2tp/pppoe/` - `InterfaceServer.handlePADT`
- [ ] `internal/component/radius/attr.go` - `EncodeVSA`, `DecodeVSA`
- [ ] `internal/component/l2tp/` authradius - `isValidSubscriberIP`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A PPP frame (LCP, IPCP, IPv6CP) on a PPPoE or L2TP session; an L2TP control
message (CDN, ICRQ/OCRQ); a RADIUS Access-Accept; a config load.

### Transformation Path
1. PPPoE discovery or L2TP control, session creation.
2. PPP FSM per RFC 1661 drives LCP then NCPs.
3. Auth handler (local or RADIUS) authorises and assigns an address.

### Boundaries Crossed
Wire to the L2TP reactor; reactor to the kernel (pppol2tp); subscriber to RADIUS.

### Integration Points
The subscriber auth registry, the metrics registry, session teardown events.

### Architectural Verification
- [ ] Teardown, from any site, emits the same subscriber event

## Risks & Assumptions

### Assumptions
- The RFC 1661 state table in `ppp_fsm.go` is the single transition owner.

### Risks
- `spec-pppoe-subscribers-produce-no-accounting-or-telemetry` touches the same teardown path; coordinate on `handleCDN`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer Configure-Request with Magic-Number 0 | → | `negotiateLCP` | unit asserting a Configure-Nak |
| Peer CDN on an established session | → | `handleCDN` | `.ci` under `test/l2tp/` asserting the subscriber down event |
| Access-Accept Framed-IP-Address 0xFFFFFFFF | → | authradius address choice | unit asserting negotiated address |

## Acceptance Criteria

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | pppoe client LCP | The client judges every peer option. RFC 1661 Section 6.4: "Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero." A received Magic-Number of zero is Nak'd (Section 6.4 Nak rule, quote owed at design from `rfc/full/rfc1661.txt`) |
| AC-2 | LCP Configure-Nak construction | A Configure-Request repeating an option draws one reply entry per option Type; a Magic-Number of Length 2 is refused as malformed. Unit with hex in and hex out |
| AC-3 | `LCPDoTransition` | The RXJ- arms in Closed, Closing and Stopping emit tlf and enter states 2, 2 and 3; AckRcvd with RTA or RXJ+ enters 6. A table-driven unit covers every RFC 1661 Section 4.1 cell |
| AC-4 | NCP Configure-Reject | RFC 1661 Section 5.4: "Invalid packets are silently discarded." A malformed Configure-Reject is discarded and the session stays up |
| AC-5 | `generateIPv6CPInterfaceID` | The dead `crypto/rand.Read` error branch is removed (Go 1.27 documents it never returns an error); no test asserts that branch |
| AC-6 | `RegisterAuthHandler` | With RADIUS configured, RADIUS authenticates; with it unconfigured, local does; the order does not depend on init and configure timing. `.ci` under `test/l2tp/` for both |
| AC-7 | `PlaceOutgoingCall` | With several `environment l2tp server` blocks, an outgoing call dials from the listener that reaches the remote; unit with two reactors |
| AC-8 | Framed-IP-Address 0xFFFFFFFF | RFC 2865 Section 5.8: "The value 0xFFFFFFFF indicates that the NAS Should allow the user to select an address (e.g. Negotiated)." The pool does not assign; IPCP accepts the peer's address. Unit plus an `rfc/short/rfc2865.md` row for Section 5.8 |
| AC-9 | `TestReactorReauthIntervalFromParams` | The helper builds an initialised tunnel map; the full L2TP package run does not panic |
| AC-10 | peer CDN teardown | A CDN-driven teardown emits the same subscriber down event and accounting stop as the local teardown sites |
| AC-11 | `handlePADT` | A PADT MAC mismatch logs at Debug with a reason, like PADI and PADR refusals, and is rate-limited |
| AC-12 | L2TP metrics binding | L2TP metrics bind through `InjectPluginMetrics` like the PPP and PPPoE counters, so they exist when the registry is created late |
| AC-13 | `radius-coa-duplicate-freeradius` | The scenario runs Ze's authradius at the log level its checker reads; the CoA teardown is asserted from Ze's output |
| AC-14 | `EncodeVSA`/`DecodeVSA` | The two are inverses on the same octets (the attribute value); a round-trip unit passes and every caller writes one Type and Length |
| AC-15 | `ExtractParameters` | `ze config validate` runs the relay and dial-source checks start runs; a config relaying to an undeclared remote fails validate with the same message |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit per AC | package of the producer | AC-1 to AC-15 |

### Functional Tests

| Test | AC |
|------|----|
| new `.ci` under `test/l2tp/` | AC-6, AC-10, AC-15 |

### Interop Tests (Scope: protocol)

| Scenario | AC |
|----------|----|
| `radius-coa-duplicate-freeradius` | AC-13 |

## Files to Modify
- The producer files under Current Behavior; `rfc/short/rfc2865.md`.

### Integration Checklist
- [ ] Each fix reached through a PPP frame, an L2TP message or config

### Documentation Update Checklist (BLOCKING)
- [ ] The L2TP and PPP pages `ai/CODE-TO-DOCS.md` names for each edited file

## Implementation Steps
1. Read each producer at HEAD; move a fixed row to "Rows found already fixed".
2. Red test, fix, green, page edit, journal cell closed with the commit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `blanket-mechanism-hid-missing-cases` | 2026-08-31 | pppoe client LCP |
| `crypto-rand-read-error-branch-is-dead-code` | 2026-09-09 | `generateIPv6CPInterfaceID` |
| `design-claim-never-measured` | 2026-09-03 | `subscriber.RegisterAuthHandler` |
| `dial-ignores-which-listener-reaches-the-remote` | 2026-10-17 | `Subsystem.PlaceOutgoingCall` |
| `field-carries-two-meanings` | 2026-09-30 | Framed-IP-Address 0xFFFFFFFF |
| `fixture-encodes-an-impossible-state` | 2026-09-23 | `TestReactorReauthIntervalFromParams` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-04 | l2tp teardown on peer CDN |
| `peer-refusal-logged-at-warn` | 2026-10-09 | `InterfaceServer.handlePADT` |
| `registry-read-outruns-its-lazy-creation` | 2026-09-09 | `l2tp/subsystem.go` metrics |
| `reply-sized-by-the-peer-it-answers` | 2026-08-31 | LCP Configure-Nak construction |
| `silent-fall-through` | 2026-10-20 | `zePeer` radius interop env |
| `silent-fall-through` | 2026-10-05 | `LCPDoTransition` |
| `silent-fall-through` | 2026-09-20 | `EncodeVSA` |
| `transient-failure-treated-as-fatal` | 2026-08-31 | PPP NCP Configure-Reject |
| `validate-accepts-what-start-refuses` | 2026-10-17 | `l2tp.ExtractParameters` |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| none in this group at the 2026-10-23 sweep | - | - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run go test ./internal/component/l2tp/... ./internal/component/radius/...`, `./le test functional l2tp`
