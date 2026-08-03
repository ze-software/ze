# Spec: radius-subscriber-attributes

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/l2tp/plugins/authradius/handler.go` - Access-Request attributes (`buildAuthAttrs`)
4. `internal/component/l2tp/plugins/authradius/acct.go` - Accounting-Request attributes (`buildAcctPacket`)
5. `internal/component/radius/dict.go` - attribute constants

## Task

**Skeleton created from the osvbng comparison refresh (2026-07-10). Full design not started.**

Ze's subscriber RADIUS messages (L2TP/PPP auth + accounting) omit attributes that
billing and provisioning systems commonly key on. Two groups:

**1. NAS-Port-Id (attribute 87, RFC 2869).** Not emitted anywhere; the constant
does not even exist in the dictionary. Today `NAS-Port` carries the raw L2TP
session ID and `NAS-Port-Type` is `Virtual`, which does not identify the physical
or logical access circuit. Add a templated NAS-Port-Id string on both
Access-Request and Accounting-Request. Reference: osvbng 426970e uses a
configurable `nas_port_id_format` (default `{interface}:{svlan}.{cvlan}`); the Ze
design must decide which placeholders make sense for an LNS (tunnel ID, session
ID, LAC address, PPP interface at minimum; VLANs only if/where visible).

**2. Address attributes in accounting.** `buildAcctPacket` never reports what the
subscriber was assigned:

- `Framed-IP-Address` (8): constant exists in the dictionary but is not emitted
  even though the session's IPv4 assignment is known at IP-assigned time.
- `Framed-IPv6-Prefix` (97, RFC 3162): no constant, not emitted.
- `Delegated-IPv6-Prefix` (123, RFC 4818): no constant, not emitted.
- `Framed-Interface-Id` (96, RFC 3162): no constant, not emitted (design decides
  if it is worth carrying for IPv6CP-negotiated interface identifiers).

Design should also decide whether the adjacent verified gaps join this scope or
are recorded as out of scope: `Calling-Station-Id` (31) and `Event-Timestamp`
(55) exist in the dictionary but are not emitted; `Acct-Delay-Time` (41) and
`Acct-Terminate-Cause` (49) are also absent from accounting packets.

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` - RADIUS accounting design context (referenced by acct.go).
  → Constraint: accounting failures MUST NOT tear down sessions (RFC 2866, enforced at acct.go).
- [ ] `ai/rules/config.md` - the NAS-Port-Id format template is operator config.
  → Constraint: decide YANG leaf placement in the authradius plugin schema.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2869 (RADIUS extensions) - NAS-Port-Id semantics.
- [ ] RFC 3162 (RADIUS and IPv6) - Framed-IPv6-Prefix, Framed-Interface-Id.
- [ ] RFC 4818 (Delegated-IPv6-Prefix) - PD reporting.
  → Constraint: create missing `rfc/short/` summaries during DESIGN.

**Key insights:**
- The attribute encoding layer (`radius.Attr`, `AttrString`, `AttrUint32`) already
  handles arbitrary attributes; the work is dictionary constants, session data
  plumbing (which addresses/prefixes the session actually holds), config for the
  NAS-Port-Id template, and emission in both packet builders.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-10; re-read at design time)
- [ ] `internal/component/l2tp/plugins/authradius/handler.go` - `buildAuthAttrs` (:153) emits User-Name, Service-Type, Framed-Protocol, NAS-Port-Type=Virtual, NAS-Port=SessionID (:154-159), NAS-IP-Address (:163), NAS-Identifier (:167), PAP/CHAP material. No NAS-Port-Id, no Calling-Station-Id.
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `buildAcctPacket` (:193-252) emits User-Name, Acct-Status-Type, Acct-Session-Id, Service-Type, Framed-Protocol, NAS-Port-Type, NAS-Port, NAS-IP-Address, NAS-Identifier, and on Stop/Interim the octets/packets/gigawords counters (:231-245) from per-session PPP interface kernel stats (`acctGetStats` = `iface.GetStats`, :20,:216). No address attributes at all.
- [ ] `internal/component/radius/dict.go` - has `AttrFramedIPAddress=8` (:35), `AttrCallingStationID=31` (:43), `AttrEventTimestamp=55` (:55), `AttrFramedIPv6Route=99` (:61). The block jumps 85→88 and 99→101: attributes 87 (NAS-Port-Id), 96 (Framed-Interface-Id), 97 (Framed-IPv6-Prefix), 123 (Delegated-IPv6-Prefix) do not exist.
- [ ] `internal/component/l2tp/events/` - `SessionIPAssignedPayload` carries the data available at accounting start (verify which v4/v6 fields exist at design time).

**Behavior to preserve:**
- Existing attribute set and ordering (billing systems may pattern-match); new attributes are additive.
- Interim/Stop counter semantics unchanged.

**Behavior to change:**
- Access-Request and Accounting-Request gain NAS-Port-Id; Accounting-Request gains address attributes.

## Data Flow (MANDATORY)

### Entry Point
- Session lifecycle events (`SessionIPAssigned`, `SessionDown`) into the authradius plugin; NAS-Port-Id template from plugin config.

### Transformation Path
1. Session events carry assigned v4 address and v6 prefix/PD data (audit what the payload holds today; extend if missing).
2. NAS-Port-Id template resolved once per session from tunnel/session/interface facts.
3. Both packet builders append the new attributes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| l2tp events ↔ authradius | session payload fields | [ ] |
| Config ↔ authradius | NAS-Port-Id template leaf | [ ] |
| authradius ↔ radius | new dictionary constants + Attr values | [ ] |

### Integration Points
- `internal/component/radius/dict.go` - add attribute constants 87/96/97/123.
- `internal/component/l2tp/plugins/authradius/{handler.go,acct.go}` - emission.
- authradius plugin YANG - template leaf.

### Architectural Verification
- [ ] No bypassed layers (session data via events, not reach-around)
- [ ] No unintended coupling (dictionary stays generic; formatting in the plugin)
- [ ] No duplicated functionality (reuse Attr encoding)
- [ ] Registration over hardcoding - no core changes; plugin-local config

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Session events (or session metadata) expose the assigned IPv4 address and IPv6 prefix/PD at accounting time | `SessionIPAssignedPayload` name suggests v4 at least | extend the event payloads first | read `internal/component/l2tp/events/` producers | unvalidated |
| A-2 | An LNS-meaningful NAS-Port-Id template can be built from tunnel/session/LAC/interface facts | osvbng precedent + available session fields | reduced placeholder set | enumerate session fields at design | unvalidated |
| A-3 | Framed-IPv6-Prefix encoding (prefix-length + padded prefix, RFC 3162 Section 2.3) needs a new Attr value helper | dict.go has no prefix-typed values | small encoder addition | read `internal/component/radius/packet.go` value encoding | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Billing systems mis-parse newly added attributes | interop test diff against FreeRADIUS dictionary | functional test asserts exact wire encoding; attributes additive and standard |
| R-2 | Template mis-config produces empty NAS-Port-Id | empty attr in test pcap | verify-time template validation; omit attr when resolved value is empty |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PPP session authenticates via RADIUS with a NAS-Port-Id template configured | → | Access-Request carries attr 87 with the resolved string | `test/plugin/radius-nas-port-id.ci` |
| session with v4 + v6 assignment reaches Interim/Stop | → | Accounting-Request carries attrs 8/97/123 as assigned | `test/plugin/radius-acct-address-attrs.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | NAS-Port-Id template configured | attr 87 present in Access-Request and all Accounting-Requests, resolved per session |
| AC-2 | session holds an IPv4 address | Framed-IP-Address (8) in Start/Interim/Stop |
| AC-3 | session holds an IPv6 prefix (SLAAC/IA_NA path) | Framed-IPv6-Prefix (97) emitted, RFC 3162 encoding |
| AC-4 | session holds a delegated prefix | Delegated-IPv6-Prefix (123) emitted, RFC 4818 encoding |
| AC-5 | no template configured, v4-only session | packets identical to today except the new address attribute decisions from design |
| AC-6 | FreeRADIUS receives the packets | attributes decode against the standard dictionary (interop) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | operator configures a NAS-Port-Id template; subscriber connects | config → template resolve → auth+acct packets | `test/plugin/radius-nas-port-id.ci` |
| 2 | subscriber gets v4+v6+PD; billing reads interim records | session events → acct builder → attrs 8/97/123 on the wire | `test/plugin/radius-acct-address-attrs.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNASPortIdTemplate` | `internal/component/l2tp/plugins/authradius/` | placeholder resolution | |
| `TestAcctAddressAttrs` | `internal/component/l2tp/plugins/authradius/acct_test.go` | attrs 8/97/123 encoding + presence | |
| `TestFramedIPv6PrefixEncoding` | `internal/component/radius/` | RFC 3162 Section 2.3 wire format | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix-length (attr 97/123) | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-nas-port-id` | `test/plugin/radius-nas-port-id.ci` | Access-Request + accounting carry the templated NAS-Port-Id | |
| `radius-acct-address-attrs` | `test/plugin/radius-acct-address-attrs.ci` | accounting reports assigned v4/v6/PD addresses | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| extend existing RADIUS interop | `test/` (see spec-radius-admin-interop-freeradius for harness precedent) | FreeRADIUS | attributes decode against the standard dictionary | |

### Future (if deferring any tests)
- None planned (skeleton; refine at design).

## Files to Modify
- `internal/component/radius/dict.go` - constants 87/96/97/123
- `internal/component/radius/` value encoding - IPv6-prefix-typed attribute helper (if A-3 confirms)
- `internal/component/l2tp/plugins/authradius/handler.go` - NAS-Port-Id in Access-Request
- `internal/component/l2tp/plugins/authradius/acct.go` - NAS-Port-Id + address attrs in accounting
- authradius plugin YANG - NAS-Port-Id template leaf
- `internal/component/l2tp/events/` - session payload fields (if A-1 breaks)

## Files to Create
- `test/plugin/radius-nas-port-id.ci` - functional coverage (NAS-Port-Id)
- `test/plugin/radius-acct-address-attrs.ci` - functional coverage (address attributes)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** - run the `/ze-spec` workflow: audit session event payloads for available address data, enumerate template placeholders, settle the scope question on attrs 31/41/49/55, then fill ACs/tests above.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN.
- Interim-update SCHEDULING is out of scope here: `plan/spec-radius-acct-timewheel.md`.

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] `make ze-test` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] RFC 2869/3162/4818 summaries exist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
