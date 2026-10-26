# Spec: ike-eap-only-authentication

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | - (RFC 5998 is reached only by the vpn-concentrator profile, which is not a release profile) |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 5998 lets an IKEv2 responder authenticate with an EAP method alone. Ze
implements none of it: no EAP_ONLY_AUTHENTICATION (16417) notify is defined in
`internal/component/ike/wire/payload_notify.go`, the initiator never sends it,
the responder ignores it as an unrecognised status notify, and
`internal/component/ike/engine/eap_auth.go` holds no list of the methods
Section 4 allows and no AUTH computed from the EAP key alone. Rows
RFC5998-3-1, 3-2, 3-3, 3-4 and 4-1 of `rfc/short/rfc5998.md` are `{gap}`.
RFC5998-3-5 holds because Ze never sends the notify, and its tagged test
(`TestRFC5998IKEAuthCarriesNoEAPOnlyNotify`) must keep holding once the
initiator learns to send it outside a resumed session.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `rfc-extension-not-implemented.md` | 2026-10-26 | RFC 5998 EAP-only authentication absent |

## Required Reading

| Read | Why |
|------|-----|
| `rfc/full/rfc5998.txt` Sections 3 and 4 | The notify, the exchange and the properties a method must have |
| `rfc/full/rfc7296.txt` Sections 2.16 and 3.10.1 | EAP inside IKE_AUTH, and status notify handling |
| `rfc/short/rfc5998.md` | The five `{gap}` rows and the one tagged row |
| `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` | How Ze runs EAP inside IKE_AUTH today |

Skeleton: the design pass reads strongSwan's EAP-only handling before the
spec moves to `ready`.

## Current Behavior

Read at skeleton time to fill the Task paragraph:

- [ ] `internal/component/ike/wire/payload_notify.go` -- defines no EAP_ONLY_AUTHENTICATION (16417) status type
- [ ] `internal/component/ike/engine/eap_auth.go` -- no allowed-method list, and no AUTH computed from the EAP key alone

## Data Flow

### Entry Point

The IKE_AUTH request, where the initiator sends the notify, and the IKE_AUTH
response, where the responder sends its EAP request without an AUTH payload.

### Transformation Path

Initiator config puts the notify in message 3; the responder reads it, omits
its own AUTH and runs EAP; both ends check the method and compute AUTH from
the EAP-generated key.

### Boundaries Crossed

Wire to engine, and engine to the EAP methods in `internal/core/eap`.

### Integration Points

`internal/component/ike/engine/eap_auth.go`, the notify types in
`internal/component/ike/wire`, and the peer config leaf that enables the
extension.

## Wiring Test

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| IKE_AUTH with EAP_ONLY_AUTHENTICATION -> responder EAP without AUTH | `engine/eap_auth.go` | named at design, plus a strongSwan scenario |
| resumed-session IKE_AUTH -> no notify sent | the initiator IKE_AUTH builder | `TestRFC5998IKEAuthCarriesNoEAPOnlyNotify` (exists) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| one tagged test per `{gap}` row, named at design | the row's requirement in both polarities, recorded with `./le rfc record` |

### Functional Tests

| Test | Asserts |
|------|---------|
| a strongSwan interop scenario, named at design | an EAP-only IKE_AUTH completes against strongSwan |

## Files to Modify

Skeleton, confirmed at design:

- `internal/component/ike/wire/payload_notify.go`
- `internal/component/ike/engine/eap_auth.go`
- `rfc/short/rfc5998.md`

## Implementation Steps

Skeleton: written at design.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Scoped unit tests of `internal/component/ike/engine`, then `./le verify changed`
- [ ] `./le rfc check` names no rfc5998 row
