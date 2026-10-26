# Spec: radius-rfc3162-ipv6-attributes

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | The lns release profile requires RFC 3162. Ze is the RADIUS NAS for L2TP subscribers, so the MUSTs of the attributes and the secret rule this RFC sets are owed by shipped code |
| RFC gaps | RFC3162-2.3-1, RFC3162-3.2-1, RFC3162-5-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Close the RFC 3162 MUST gaps of Ze's RADIUS NAS. The summary walk of
2026-10-26 had marked these rows not-applicable or declined. No owner ruling
names any of these features unwanted (`ai/META-DECISIONS.md`, Goal and scope),
so each is owed work.

| Row | Producer | What is missing |
|-----|----------|-----------------|
| RFC3162-3.2-1 | `internal/component/l2tp/plugins/authradius/handler.go::buildAccessRequestAttrs`, `internal/component/l2tp/ppp/session_run.go` (re-auth ticker calling `runAuthPhase`) | No Access-Request carries Framed-Interface-Id. With `authentication reauth-interval` set, a CHAP re-authentication on an established session sends an Access-Request after IPv6CP negotiated an Interface-Identifier, and it omits the attribute |
| RFC3162-2.3-1 | `buildAccessRequestAttrs`, `internal/component/l2tp/plugins/authradius/acct.go::buildAcctPacket` | No Framed-IPv6-Prefix encoding exists, so the Reserved field is never written |
| RFC3162-5-1 | `internal/component/radius/client.go::Exchange` | An empty shared secret is refused (RFC 2865 Section 3), so RADIUS over IPsec ESP with no configured secret cannot run with the zero-length secret this row requires |

RFC 3162 text, `rfc/full/rfc3162.txt`:

- Section 3.2 (printed for 2.2, see the summary's errata note): "If the Interface-Identifier IPv6CP option [11] has been successfully negotiated, this Attribute MUST be included in an Access-Request packet as a hint by the NAS to the server that it would prefer that value."
- Section 2.3: "This field, which is reserved and MUST be present, is always set to zero."
- Section 5: "However, where RADIUS is run over IPSEC ESP with a non-null transform, the secret shared between the NAS and the RADIUS server MAY NOT be configured. In this case, a shared secret of zero length MUST be assumed."

RFC3162-5-1 meets RFC 2865 Section 3 ("The secret MUST NOT be empty"); design
reconciles the two, so a zero-length secret is admitted only for a server
reached over an ESP-protected path. Design reads accel-ppp
(`accel-pppd/radius/`) and the FreeRADIUS client library, and quotes the
deciding functions.

## Required Reading

- `rfc/short/rfc3162.md`, `rfc/full/rfc3162.txt` Sections 2.3, 3.2 and 5
- `rfc/short/rfc2865.md` (secret and Access-Request rules)
- The RADIUS and L2TP pages named for these files in `ai/CODE-TO-DOCS.md`

## Current Behavior (MANDATORY)

**Source files read:** (2026-10-26; design re-reads each before any decision)
- [ ] `internal/component/l2tp/plugins/authradius/handler.go` - `doRADIUS` builds each Access-Request from a `ppp.EventAuthRequest` with `buildAccessRequestAttrs`; no Framed-Interface-Id, no Framed-IPv6-Prefix
- [ ] `internal/component/l2tp/ppp/session_run.go` - the re-auth ticker reruns `runAuthPhase` on an established session (CHAP only)
- [ ] `internal/component/radius/client.go` - `Exchange` refuses an empty secret before sending

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- PPP Authenticate phase and the periodic re-authentication on an L2TP session

### Transformation Path
1. `ppp.EventAuthRequest` to the RADIUS auth handler
2. `buildAccessRequestAttrs`
3. `radius.Client.Exchange`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPP driver to RADIUS handler | `EventAuthRequest` (needs the negotiated IPv6CP identifier) | No |

### Integration Points
- The IPv6CP negotiated identifiers held by the PPP session

## Risks & Assumptions

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A zero-length secret opens forgery where the path is not ESP-protected | A config with an empty secret and no IPsec policy to the server is accepted | Admit the empty secret only for a server whose path an installed ESP policy covers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CHAP re-authentication after IPv6CP | → | Access-Request with Framed-Interface-Id | `test/l2tp/` functional test (to write) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC3162ReauthCarriesInterfaceID` | `internal/component/l2tp/plugins/authradius/` | AC-1 | |
| `TestRFC3162FramedIPv6PrefixReservedZero` | `internal/component/radius/` | AC-2 | |

Each tagged unit carries a discrimination record. Scoped evidence:
`./le test unit` over the changed packages and the named `.ci` test.

## Files to Modify
- `internal/component/l2tp/plugins/authradius/handler.go` - Framed-Interface-Id once IPv6CP has negotiated
- `internal/component/l2tp/ppp/auth_events.go` - carry the negotiated identifier on a re-auth request
- `internal/component/radius/` - Framed-IPv6-Prefix encoder; zero-length secret over ESP
- `rfc/short/rfc3162.md` - remove the `{gap}` annotations

## Implementation Steps

1. Carry the IPv6CP identifier into the re-auth `EventAuthRequest` and the Access-Request
2. Framed-IPv6-Prefix encoder with the Reserved octet, and the Access-Request hint
3. Zero-length secret for an ESP-protected server path

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `rfc-extension-not-implemented.md` | 2026-10-26 | RADIUS NAS: Framed-Interface-Id, Framed-IPv6-Prefix, zero-length secret over ESP |

## Acceptance Criteria

| AC | Criterion |
|----|-----------|
| AC-1 | An Access-Request sent after IPv6CP negotiated an Interface-Identifier carries Framed-Interface-Id with that value; one sent before IPv6CP carries none |
| AC-2 | Every Framed-IPv6-Prefix Ze writes carries the Reserved octet, set to zero |
| AC-3 | A RADIUS server reached over ESP with a non-null transform and no configured secret is run with a zero-length secret; without that protection the empty secret stays refused |
| AC-4 | Each row is implemented, its `{gap}` removed, with tagged tests and discrimination records |
