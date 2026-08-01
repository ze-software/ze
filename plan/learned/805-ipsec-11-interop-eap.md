# 805 - IPsec Interop: EAP Authentication (ipsec-11)

Spec: `plan/spec-ipsec-11-interop-eap.md` (closed)

## What Was Built

IKE engine EAP authentication flow (initiator side) for EAP-MSCHAPv2 and EAP-TLS,
validated against strongSwan in Docker interop lab.

### Engine Changes
- `StateEAPInProgress` added to SA state machine (`sa.go`)
- `buildAuthRequest` omits AUTH payload when auth mode is EAP (RFC 7296 Section 2.16)
- `handleAuthResponse` detects EAP payload, verifies server AUTH first, creates EAP session
- New `handleEAPResponse` drives multi-round EAP exchange loop
- `handleInbound` routes `StateEAPInProgress` to EAP handler
- MSK-derived AUTH sent after EAP-Success (`ComputeAuthFromMSK`)

### EAP Peer Implementation
- `eap/peer.go`: EAP peer session dispatcher for initiator-side exchanges
- `eap/peer_test.go`: unit tests for EAP Identity, MSCHAPv2, and TLS method dispatch
- `eap/eap_tls.go`: extended for proper TLS transport over EAP fragmentation

### Interop Scenarios
- `03-eap-mschapv2`: Ze as initiator, strongSwan as EAP-MSCHAPv2 authenticator
- `04-eap-tls`: Ze as initiator, strongSwan as EAP-TLS authenticator with test PKI
- Shared test PKI in `test/ipsec-interop/pki/` (CA, server cert, client cert)

### Config
- YANG schema extended with EAP auth modes in `ze-ipsec-conf.yang`
- `ipsec/config.go` maps EAP config to engine parameters

## Key Decisions

- **Server AUTH verified before EAP processing.** RFC 7296 Section 2.16 requires initiator
  to verify responder's AUTH payload before processing EAP requests (prevents credential
  harvesting by rogue server).
- **Shared test PKI across scenarios.** Both EAP-MSCHAPv2 and EAP-TLS need strongSwan to
  authenticate with a server certificate. Single `pki/` directory with `gen-pki.sh` avoids
  duplication.
- **Peer module separate from server EAP.** `eap/peer.go` handles initiator-side dispatch;
  `eap/eap.go` remains for server-side (future responder work).

## What Helped

- Existing interop lab framework (`lab.py`) made adding scenarios straightforward.
- EAP crypto primitives from ipsec-9 (MSCHAPv2, MD4, magic constants) worked correctly
  against strongSwan on first try.
- Alpine strongSwan package includes `eap-mschapv2` and `eap-tls` plugins out of the box.

## Gotchas

- EAP initiator MUST NOT include AUTH in first IKE_AUTH request. Omitting AUTH signals
  EAP willingness. The existing `buildAuthRequest` always included AUTH, requiring a
  conditional path.
- Message ID must increment across EAP round-trips within the same IKE_AUTH exchange.

## Files

None recorded.
