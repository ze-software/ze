# Spec: radius-subscriber-attributes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | implementation and review complete; closure owed |
| Updated | 2026-08-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/l2tp/plugins/authradius/handler.go` - Access-Request attributes (`buildAccessRequestAttrs`, `buildAuthAttrs`)
4. `internal/component/l2tp/plugins/authradius/acct.go` - Accounting-Request attributes (`buildAcctPacket`, `subscriberIPv4`)
5. `internal/component/l2tp/plugins/authradius/nasportid.go` - NAS-Port-Id template
6. `internal/component/radius/dict.go` - attribute constants

## Task

Ze's subscriber RADIUS messages (L2TP/PPP auth + accounting) omit attributes that
billing and provisioning systems key on. Two groups were named at skeleton time:

**1. NAS-Port-Id (attribute 87, RFC 2869 Section 5.17).** Not emitted anywhere;
the constant did not exist in the dictionary. `NAS-Port` carries the raw L2TP
session id and `NAS-Port-Type` is `Virtual`, so nothing identifies the access
circuit.

**2. Address attributes in accounting.** `buildAcctPacket` never reported what
the subscriber was assigned, though the IPv4 assignment is known at
IP-assigned time.

**Design decision (2026-08-03): scope is what ze can source.** An attribute is
implemented when a value for it exists at runtime, and is recorded below with
the place its value would first exist when it does not. Three of the six
attributes the skeleton listed have no producer in ze at all, which is a
finding about the DHCPv6-PD and IPv6CP paths rather than a choice about
RADIUS. See Known Limitations.

### Placeholder set for NAS-Port-Id

The template resolves against facts known at BOTH emission points, because a
NAS-Port-Id that differs between the Access-Request and the accounting records
cannot be joined by the system that reads them. `ppp.EventAuthRequest` carries
the tunnel and session ids and no interface: pppN does not exist until NCP
completes, which is after authentication. The placeholder set is therefore
`{nas-id}`, `{tunnel-id}`, `{session-id}`, and osvbng's `{interface}`,
`{svlan}` and `{cvlan}` are not offered. An unknown placeholder is refused when
the config is parsed rather than sent to the server as literal text.

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` - RADIUS accounting design context.
  → Constraint: accounting failures MUST NOT tear down sessions (RFC 2866, enforced at acct.go).
- [ ] `ai/rules/config.md` - the NAS-Port-Id format template is operator config.
  → Decision: leaf `nas-port-id-format` in the authradius YANG, beside `nas-identifier`.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2865 (RADIUS) Section 5.8 - Framed-IP-Address: Type 8, Length 6, four octets.
  → Constraint: only an IPv4 assignment can be reported; the field has no other width.
- [ ] RFC 2866 (RADIUS Accounting) Section 4.1 - "If the Accounting-Request packet
  includes a Framed-IP-Address, that attribute MUST contain the IP address of the
  user ... MUST contain the actual IP address assigned or negotiated."
  → Constraint: source the value from the negotiated address, never from config.
- [ ] RFC 2866 Section 5.13 - Table of Attributes: Framed-IP-Address 0-1 in Accounting-Request.
- [ ] RFC 2869 (RADIUS Extensions) Section 5.17 - NAS-Port-Id: Type 87, Length >= 3,
  UTF-8 text, "only used in Access-Request and Accounting-Request packets".
  → Constraint: an empty resolution is not an attribute; Length >= 3 means at least one text octet.

The two IPv6 attribute RFCs the skeleton listed are NOT read, NOT summarised and
NOT in the repository, and nothing here is sourced from them. See L-2.

**Key insights:**
- The attribute encoding layer (`radius.Attr`, `AttrString`, `AttrUint32`) already
  handles arbitrary attributes. The work was a dictionary constant, config for the
  template, and emission in both packet builders.
- `SessionIPAssignedPayload.PeerAddr` already carried the assigned address to the
  accounting plugin. No event payload change was needed for IPv4 (A-1 held for v4).

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-08-03)
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `buildAcctPacket` emitted
  User-Name, Acct-Status-Type, Acct-Session-Id, Service-Type, Framed-Protocol,
  NAS-Port-Type, NAS-Port, NAS-IP-Address, NAS-Identifier, and on Stop/Interim the
  octet/packet/gigaword counters. No address attribute at all, though `acctSession.peerAddr`
  already held the assigned address.
- [ ] `internal/component/l2tp/plugins/authradius/handler.go` - `buildAuthAttrs` emitted
  User-Name, Service-Type, Framed-Protocol, NAS-Port-Type, NAS-Port, NAS-IP-Address,
  NAS-Identifier and the credential material. No NAS-Port-Id.
- [ ] `internal/component/radius/dict.go` - the constant block jumped 85 to 88 and 99 to
  101: attributes 87, 96, 97 and 123 did not exist.
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handleSessionIPAssigned` fills
  `SessionIPAssignedPayload.PeerAddr` from `ppp.EventSessionIPAssigned.Peer`, the
  IPCP-negotiated subscriber address, and emits only when that address is valid.
- [ ] `internal/component/l2tp/ppp/ncp.go` - `onNCPOpened` sets `Peer` for the IPv4
  family and only `InterfaceID` for IPv6, so the IPv6 family emits no l2tp event.

**Behavior preserved:**
- Existing attribute set and ordering; the new attributes are appended.
- With no `nas-port-id-format` configured and no assigned address, packets are
  unchanged.

**Behavior changed:**
- Access-Request gains NAS-Port-Id when a format is configured.
- Accounting-Request gains NAS-Port-Id and Framed-IP-Address.

## Data Flow (MANDATORY)

### Entry Point
- PPP authentication (`ppp.EventAuthRequest` through the registered auth handler) for
  the Access-Request; the `(l2tp, session-ip-assigned)` and `(l2tp, session-down)`
  events for accounting; `nas-port-id-format` from the plugin config tree.

### Transformation Path
1. Config parse validates the template and stores it on `radiusConfig`.
2. One reload applies the client and the template together, so no packet can mix a
   new client with an old template.
3. The auth path resolves the template from the tunnel and session ids of the request.
4. The accounting path resolves it ONCE, when `onSessionIPAssigned` creates the
   session, and stores the text on `acctSession.nasPortID`. Every record of that
   session repeats it, so a config reload cannot move the text a billing system
   joins the records by. `buildAcctPacket` reads the stored text and parses
   `acctSession.peerAddr` for the four octets of Framed-IP-Address.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| l2tp events ↔ authradius | `SessionIPAssignedPayload.PeerAddr` | [x] carries the IPCP-negotiated address; no payload change needed |
| Config ↔ authradius | `nas-port-id-format` leaf | [x] parsed and validated in `parseConfigFromTree` |
| authradius ↔ radius | `AttrNASPortID`, `AttrFramedIPAddress` | [x] encoded through the existing `radius.Attr` path |

### Integration Points
- `internal/component/radius/dict.go` - constant 87.
- `internal/component/l2tp/plugins/authradius/{handler.go,acct.go,config.go,nasportid.go,register.go}`.
- authradius YANG - `nas-port-id-format`.

### Architectural Verification
- [x] No bypassed layers (the address arrives by event, not by reaching into the reactor)
- [x] No unintended coupling (the dictionary stays generic; formatting lives in the plugin)
- [x] No duplicated functionality (reuses `radius.Attr` and `AttrString`)
- [x] Registration over hardcoding - no core changes; plugin-local config

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Session events expose the assigned IPv4 address at accounting time | `SessionIPAssignedPayload` | extend the payload first | read `handleSessionIPAssigned` and `onNCPOpened` | confirmed for IPv4; false for IPv6 (see L-1) |
| A-2 | An LNS-meaningful NAS-Port-Id can be built from tunnel/session/NAS facts | osvbng precedent, available session fields | reduced placeholder set | enumerated the fields of `ppp.EventAuthRequest` and `acctSession` | confirmed, with a reduced set |
| A-3 | Framed-IPv6-Prefix needs a new prefix-typed value helper | dict.go has no prefix-typed values | small encoder addition | not reached | moot: no prefix value exists to encode (L-1) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Billing systems mis-parse newly added attributes | interop scenario decodes what ze sent | the interop scenario asserts the decoded values; attributes are standard and additive |
| R-2 | Template mis-config produces an empty NAS-Port-Id | empty attr in the scenario output | config-time validation; the attribute is omitted when the resolution is empty or over-long |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| operator commits a NAS-Port-Id template | → | config verify accepts it, or names the supported placeholders | `test/l2tp/radius-nas-port-id.ci`, `test/l2tp/radius-nas-port-id-invalid.ci` |
| PPP session authenticates via RADIUS with a NAS-Port-Id template configured | → | Access-Request carries attr 87 with the resolved string | `test/l2tp/radius-acct-wire.ci`, `test/l2tp-interop/scenarios/04-radius-acct-attrs` |
| session with an IPv4 assignment reaches Accounting-Start | → | Accounting-Request carries attrs 8 and 87 | `test/l2tp/radius-acct-wire.ci`, `test/l2tp-interop/scenarios/04-radius-acct-attrs` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `nas-port-id-format` configured | attr 87 present in the Access-Request and in every Accounting-Request, resolved per session |
| AC-2 | one session, auth then accounting | the attr 87 text is identical in both, so the records can be joined; a reload after the session starts does not move it |
| AC-3 | format naming an unknown placeholder, longer than 253 octets, or using {nas-id} with no nas-identifier | config parse fails and says which; nothing is sent |
| AC-4 | format resolving to 0 octets, or to more than 253 | no attribute, never a truncated one (RFC 2865 Section 5, RFC 2869 Section 5.17) |
| AC-5 | session holds an IPv4 address | Framed-IP-Address (8) present in Start, Interim and Stop, equal to the negotiated address (RFC 2866 Section 4.1) |
| AC-6 | session holds no address, or a non-IPv4 one | no Framed-IP-Address rather than a wrong four-octet value |
| AC-7 | no format configured and no assigned address | packets identical to before this change |
| AC-8 | a real pppd/xl2tpd LAC establishes a session | a RADIUS server decodes attrs 8 and 87 off the wire with the expected values |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | operator configures a NAS-Port-Id template; subscriber connects | config → template resolve → auth and accounting packets | `test/l2tp-interop/scenarios/04-radius-acct-attrs` |
| 2 | subscriber gets an address; billing reads the accounting records | session event → acct builder → attrs 8 and 87 on the wire | `test/l2tp-interop/scenarios/04-radius-acct-attrs` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNASPortIDResolve` | `authradius/nasportid_test.go` | placeholder resolution | PASS |
| `TestValidateNASPortIDFormatRejects` | `authradius/nasportid_test.go` | unknown placeholder, unterminated brace, bare brace, empty name | PASS |
| `TestAccessRequestCarriesNASPortID` | `authradius/nasportid_test.go` | attr 87 in the Access-Request | PASS |
| `TestAcctRequestCarriesNASPortID` | `authradius/nasportid_test.go` | attr 87 in Start, Interim and Stop | PASS |
| `TestNASPortIDSameInAuthAndAcct` | `authradius/nasportid_test.go` | one text per session across both packets | PASS |
| `TestParseConfigRejectsUnknownPlaceholder` | `authradius/nasportid_test.go` | config-time refusal of an unknown placeholder | PASS |
| `TestParseConfigRejectsNASIDPlaceholderWithoutNASIdentifier` | `authradius/nasportid_test.go` | config-time refusal of a template that could only resolve empty | PASS |
| `TestParseConfigNASPortIDFormatLengthBoundary` | `authradius/nasportid_test.go` | 253 commits, 254 does not | PASS |
| `TestSessionEventDrivesAddressAndPortID` | `authradius/acct_address_test.go` | the session event fills the session, and a later reload does not move its NAS-Port-Id | PASS |
| `TestAcctFramedIPAddressPresent` | `authradius/acct_address_test.go` | attr 8 in Start, Interim and Stop | PASS |
| `TestAcctFramedIPAddressIsSubscriberNotNAS` | `authradius/acct_address_test.go` | the subscriber address, not the NAS address | PASS |
| `TestAcctFramedIPAddressOmittedWhenNotIPv4` | `authradius/acct_address_test.go` | unset, v6 and unparseable values emit nothing | PASS |
| `TestAcctFramedIPAddressIPv4Mapped` | `authradius/acct_address_test.go` | v4-mapped form encodes as four octets | PASS |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NAS-Port-Id value length | 1-253 octets | 253 | 0 (omitted) | 254 (omitted) |
| nas-port-id-format length | 1-253 characters | 253 | -- | 254 (config refused) |

Covered by `TestNASPortIDLengthBoundary` (0, 1, 253, 254). Attribute 8 has one
valid width, four octets, and `TestAcctFramedIPAddressOmittedWhenNotIPv4` covers
every input that cannot produce it.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-nas-port-id` | `test/l2tp/radius-nas-port-id.ci` | an operator's template is accepted by the same verify path the daemon runs at boot | PASS |
| `radius-nas-port-id-invalid` | `test/l2tp/radius-nas-port-id-invalid.ci` | a template ze cannot resolve is refused, and the error names what is supported | PASS |
| `radius-acct-wire` | `test/l2tp/radius-acct-wire.ci` (`needs-linux:caps=net-admin`) | a real kernel PPP session; the RADIUS server decodes attrs 8 and 87, and the peer cross-checks the reported address against the one IPCP settled | PASS in QEMU |
| `04-radius-acct-attrs` | `test/l2tp-interop/scenarios/04-radius-acct-attrs` | the same proof against a real xl2tpd/pppd LAC | PASS |

The wire emission needs a PPP session, a PPP session needs the kernel PPPoL2TP
channel, and that needs CAP_NET_ADMIN. That rules it out on an unprivileged dev
host, NOT in a `.ci`: `option=needs-linux` tests run in QEMU as root, which is
what `radius-acct-wire` uses. It therefore SKIPs on the dev host and runs under
`make ze-qemu-needs-linux-test`, giving the obligation verify-tier evidence
rather than interop-tier only. Its LAC peer drives the tunnel, an ICCN carrying
proxy LCP AVPs, and IPCP, so ze reaches a real assigned address.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `04-radius-acct-attrs` | `test/l2tp-interop/scenarios/` | xl2tpd + pppd (LAC), mock RADIUS server | the attributes decode off the wire, and the address reported is the one pppd negotiated | PASS |

`radius-acct-wire.ci` proves the same behavior against ze's own kernel data
plane in QEMU; the interop scenario proves it against an independent
implementation. Both are kept: they catch different failure shapes.

### Future (if deferring any tests)
- None for the implemented scope. The unimplemented attributes have no runtime value
  to test; see Known Limitations.

## Files to Modify
- `rfc/short/rfc2866.md` - Section 4.1 section and the gated MUST `RFC2866-4.1-1`
- `rfc/short/rfc2869.md` - Section 5.17 section, attribute-87 row, `RFC2869-5.17-1`
- `ai/RFC-REQUIREMENTS.md` - regenerated once (derived; `make ze-rfc-index`)
- `internal/component/radius/dict.go` - `AttrNASPortID` (87)
- `internal/component/l2tp/plugins/authradius/handler.go` - `buildAccessRequestAttrs`
- `internal/component/l2tp/plugins/authradius/acct.go` - attrs 8 and 87, `subscriberIPv4`
- `internal/component/l2tp/plugins/authradius/config.go` - `nas-port-id-format`
- `internal/component/l2tp/plugins/authradius/register.go` - one atomic apply
- `internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang` - the leaf
- `test/l2tp-interop/Dockerfile.ze` - build the lab image with the personality and feature tags
- `docs/guide/l2tp.md` - the new leaf and the accounting address

## Files to Create
- `internal/component/l2tp/plugins/authradius/nasportid.go`
- `internal/component/l2tp/plugins/authradius/nasportid_test.go`
- `internal/component/l2tp/plugins/authradius/acct_address_test.go`
- `test/l2tp-interop/scenarios/04-radius-acct-attrs/` - interop scenario
- `test/l2tp/radius-nas-port-id.ci`, `test/l2tp/radius-nas-port-id-invalid.ci` - config surface
- `test/l2tp/radius-acct-wire.ci` - the wire, in QEMU, for a real PPP session

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Verify current behavior | Current Behavior |
| 3. Red tests | TDD Test Plan |
| 4. Implement | Files to Modify / Create |
| 5. Prove | Goal Validation |

### Implementation Phases
1. **Dictionary and template** - constant 87, resolver, config leaf and validation. DONE
2. **Emission** - Access-Request and Accounting-Request. DONE
3. **Proof** - unit tests, mutation runs, interop scenario. DONE
4. **Review gate** - `/ze-review`, then closure. OPEN

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The L2TP interop lab was usable as-is | Its image built `cmd/ze` with no build tags, so the binary had no `start` root command and no L2TP feature; the lab could not boot ze at all | first run of the new scenario printed "unknown command: start" | fixed in `test/l2tp-interop/Dockerfile.ze`, the same fix `test/interop/Dockerfile.ze` already carried |
| A delegated IPv6 prefix exists somewhere in a session | The allocator is never wired: `startIPv6Service` is called with a nil prefix handler and `l2tp.GetPrefixHandler` has no caller at all | traced the producer of `IPv6Service.delegatedPrefix` | attributes 97 and 123 have no value to report (L-1) |

## Known Limitations

**L-1. Three attributes from the skeleton are NOT implemented, because no value
for them exists at runtime.** Each is recorded with the place its value would
first exist.

| Attribute | Where a value would first exist | Why it does not reach accounting |
|-----------|--------------------------------|----------------------------------|
| Framed-Interface-Id (96, RFC 3162) | `ppp.(*pppSession).onNCPOpened`, IPv6 branch, sets `EventSessionIPAssigned.InterfaceID` from `peerInterfaceID` | `L2TPReactor.handleSessionIPAssigned` emits the l2tp event only when an address is valid, and the IPv6 branch of `onNCPOpened` sets neither `Local` nor `Peer`. No l2tp event is emitted for the IPv6 family, and `SessionIPAssignedPayload` has no field for an interface identifier. Carrying it needs a payload field AND an emission for the IPv6 family, which today would start a SECOND accounting session for a dual-stack subscriber because `onSessionIPAssigned` creates one per event |
| Framed-IPv6-Prefix (97, RFC 3162) | nowhere | `BuildRA` emits no Prefix Information option ("No prefix information" in its own comment) and IPv6CP negotiates only an interface identifier, so ze assigns no /64 to a subscriber |
| Delegated-IPv6-Prefix (123, RFC 4818) | `ppp.(*IPv6Service).handleSolicit` would store one in `delegatedPrefix` | the allocator is never supplied: `session_run.go` calls `startIPv6Service` with a nil prefix handler, and `l2tp.GetPrefixHandler` / `GetPrefixReleaser` have NO callers, so the pool plugin's registered handler is unreachable. Every DHCPv6 Solicit is answered NoPrefixAvail with "no pool configured" |

The dictionary constants 96, 97 and 123 were deliberately NOT added: an exported
constant with no producer is an unwired symbol, and adding it would suggest a
support level ze does not have.

**L-2. RFC 3162 and RFC 4818 are not in the repository.** Neither `rfc/full/`
nor `rfc/short/` has them. Nothing in this spec was sourced from either RFC, and
nothing needing them was implemented.

**L-3. The two obligations this change makes live are extracted and gated.**
Emitting Framed-IP-Address makes RFC 2866 Section 4.1 ("MUST contain the actual
IP address assigned or negotiated") a live obligation for ze, so it is now the
checklist row `RFC2866-4.1-1` in `rfc/short/rfc2866.md`, carrying two positive
and two negative tagged tests. RFC 2869 Section 5.17 is extracted beside it as
`RFC2869-5.17-1`; it is a SHOULD, which the gate does not require to be tagged.
`ai/RFC-REQUIREMENTS.md` is DERIVED from those tags, so it is regenerated once,
after the work is finished, rather than after each edit.

**L-4. Scope of the adjacent attributes the skeleton asked about.** Not
implemented, with the value location for each:

| Attribute | Status | Where the value is |
|-----------|--------|--------------------|
| Calling-Station-Id (31) | out of scope | `session_fsm.go` parses AVP 22 into `callingNumber`, but neither `ppp.EventAuthRequest` nor `acctSession` carries it; plumbing it crosses the l2tp/ppp boundary |
| Event-Timestamp (55) | out of scope | available at any time from the clock; deliberately left out to keep this change single-focus |
| Acct-Delay-Time (41) | out of scope | would have to be recomputed per retransmission, and `radius.Client` re-sends a pre-encoded buffer, so the value cannot be updated without changing the client |
| Acct-Terminate-Cause (49) | out of scope | `ppp.EventSessionDown.Reason` is human-readable and its consumers are told not to parse it; `SessionDownPayload` has no cause field, so there is no enum to map |

**L-5. Interim-update scheduling** stays out of scope: `plan/spec-radius-acct-timewheel.md`.

**L-6. `test/pppoe-interop/Dockerfile.ze` has the same untagged-build defect** the
L2TP lab had (see Mistake Log). It was not changed here because no run in this
session could verify a pppoe lab fix.

## Implementation Summary
### What Was Implemented
- `AttrNASPortID` (87) in the RADIUS dictionary, cited to RFC 2869 Section 5.17.
- `nasportid.go`: the placeholder list, the resolver, the config-time validator, and
  the attribute builder that omits an empty or over-long value.
- `nas-port-id-format` leaf in the authradius YANG, parsed and validated in
  `parseConfigFromTree`, applied atomically with the client in `swapClient` and
  `setClient`.
- NAS-Port-Id in the Access-Request (`buildAccessRequestAttrs`) and in every
  Accounting-Request (`buildAcctPacket`).
- Framed-IP-Address in every Accounting-Request, from the negotiated subscriber
  address, guarded by `subscriberIPv4` so only an IPv4 value is ever encoded.
- Commit-time refusal of a template ze could not honor: an unknown placeholder, a
  format past 253 octets (also bounded in the YANG leaf), and `{nas-id}` with no
  `nas-identifier` set. Each would otherwise have produced no attribute and no
  diagnostic.
- `test/l2tp-interop/Dockerfile.ze` now builds with `ze_core` plus the feature tags
  from `feature-gates.txt`. Without this the lab could not start ze at all.
- Interop scenario `04-radius-acct-attrs`: xl2tpd/pppd LAC, ze as LNS, a mock RADIUS
  server that decodes and prints what it received.

## Goal Validation

| Goal | Evidence |
|------|----------|
| The Access-Request carries the operator's NAS-Port-Id | interop run: `Access-Request carries NAS-Port-Id lns1:1702.46818` |
| Accounting reports the subscriber's actual address | interop run: `Accounting-Start reports Framed-IP-Address=10.100.0.2`, the address pppd negotiated and the address on ppp0 |
| One session yields one port identity | interop run: `Accounting-Start carries the same NAS-Port-Id` |
| The gated path proves the wire, not only the Docker lab | `test/l2tp/radius-acct-wire.ci` PASSes in QEMU (`ze-qemu-debug`, suite id 10) and SKIPs on the unprivileged host; the ledger binds it to `RFC2866-4.1-1` as functional/verify evidence |
| The tests would fail if the code were removed | nine unit mutations each turned the owning tests red: drop attr 8; report the NAS address instead; accept a non-IPv4 value; drop attr 87 from accounting; drop attr 87 from auth; drop the format validation; resolve the session's NAS-Port-Id from empty facts; take the session address from the wrong payload field; make the nas-identifier guard never fire. Two interop mutations each turned the scenario red, and two `.ci` mutations (remove the YANG leaf, stop validating) each turned the owning `.ci` red |
| Nothing regressed | `make ze-test-pkg` green for `./internal/component/radius`, `./internal/component/l2tp/plugins/authradius` and `./internal/component/l2tp`; `make ze-lint-changed` 0 issues; `make ze-cli-grammar-check` and `make ze-yang-glue-check` OK |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Every RFC anchor the change adds cites a section its summary does not carry: `rfc2869.md` has no attribute 87 row, `rfc2866.md` no Section 4.1. The obligations are implemented and tested but unextracted | `rfc/short/rfc2866.md`, `rfc/short/rfc2869.md` | fixed: `rfc2866.md` gains a Section 4.1 section and the gated MUST `RFC2866-4.1-1`; `rfc2869.md` gains a Section 5.17 section, the attribute-87 table row, and the SHOULD `RFC2869-5.17-1`. `rfc_requirements.py --check` reports neither |
| 2 | ISSUE | Neither new test file carries an `RFC requirement:` tag, so `make ze-rfc-check` cannot see the MUST | `acct_address_test.go` | fixed: `RFC2866-4.1-1` carries two positive and two negative tags. The derived ledger is regenerated ONCE at the end of the work, not after each edit |
| 3 | ISSUE | `TestAcctFramedIPAddressFromSessionEvent` was vacuous: its comment claimed the wire value follows the session event, but it hand-built the session and never called `onSessionIPAssigned`. Deleting `peerAddr: payload.PeerAddr` left every test green | `acct_address_test.go` | fixed: replaced by `TestSessionEventDrivesAddressAndPortID`, which drives `onSessionIPAssigned` against a live accounting server and asserts the stored session and the built packet |
| 4 | ISSUE | NAS-Port-Id was re-resolved from live config per packet, so a mid-session reload moved the text and broke the join the feature exists to provide | `acct.go` `buildAcctPacket`, `setClient` | fixed: resolved once in `onSessionIPAssigned` and stored on `acctSession.nasPortID`, as `acctSessID` already was. `buildAcctPacket` no longer reads config or takes the lock |
| 5 | ISSUE | No gated test proves emission: both `.ci` files only run `ze config validate`, and the wire proof runs under the Docker interop target | `test/l2tp/radius-nas-port-id*.ci` | fixed: `test/l2tp/radius-acct-wire.ci` (`needs-linux:caps=net-admin`) drives a real kernel PPP session in QEMU and asserts on what the RADIUS server decoded. It binds `RFC2866-4.1-1` at functional/verify tier. Two mutants (drop the template, move the pool address) were run beside it in the VM and both failed, so neither assertion is vacuous |
| 6 | NOTE | Empty and over-long resolutions were dropped silently, with no diagnostic for the operator | `nasportid.go` `nasPortIDAttr` | fixed at commit time instead of at runtime: `validateNASPortIDFormat` refuses a format longer than 253 octets, and the config parse refuses `{nas-id}` when `nas-identifier` is unset. The runtime guard stays as defense in depth |
| 7 | NOTE | No length bound on the YANG leaf, so a multi-kilobyte template committed and was expanded per packet before being discarded | `yang/ze-l2tp-auth-radius-conf.yang` | fixed: `length "1..253"` on the leaf, and the same bound in the validator |
| 8 | NOTE | `subscriberIPv4`'s comment describes an IPv6 branch that is currently dead | `acct.go` | acknowledged. The comment explains why the guard exists; the branch is dead only because the reactor drops the IPv6 event (L-1). Keeping both |
| 9 | NOTE | `nas-port-id-format` absent from `docs/guide/configuration.md` | `docs/guide/configuration.md` | not applicable: that file documents the root `l2tp {}` schema and enumerates no authradius leaf (`nas-identifier` is absent too). The plugin's leaves live in `docs/guide/l2tp.md`, which was updated |

### Fixes applied
- `acct.go`: `acctSession.nasPortID` resolved once in `onSessionIPAssigned`; `buildAcctPacket` reads the stored text through `nasPortIDAttrFromText` and no longer locks.
- `nasportid.go`: `nasPortIDAttr` split so both call paths share one length guard; `validateNASPortIDFormat` bounds the raw format at 253 octets.
- `config.go`: refuses `{nas-id}` when `nas-identifier` is unset.
- YANG: `length "1..253"` on the leaf, description updated.
- `acct_address_test.go`: the vacuous test replaced by an event-driven one that also pins the reload behavior.
- `nasportid_test.go`: two config-guard tests, including the 253/254 boundary.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | -- | not re-run; findings 1, 2 and 5 are recorded above with their reasons rather than fixed | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE -- run 1 raised 5 ISSUEs and 4 NOTEs; all five ISSUEs are now fixed, three of them (1, 2, 5) after the first answer was wrong. Findings 1 and 2 were first recorded as blocked on not touching `ai/`: that was an overstatement, since the summaries and tags live under `rfc/` and only the DERIVED ledger needs one regeneration, done once at the end. Finding 5 was first recorded as an environment limit: also wrong, since `option=needs-linux` runs a `.ci` in QEMU as root
- [x] All NOTEs recorded above

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] `make ze-test` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] RFC 2866 and RFC 2869 summaries carry the two obligations this change makes live; RFC 3162 and RFC 4818 summaries do not exist and nothing needing them is implemented (L-2)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
