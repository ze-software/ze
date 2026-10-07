# IKEv2 Wire Format

## Meta

| Field | Value |
|-------|-------|
| Name | IKEv2 Wire Format |
| Kind | library |
| Scope | complete |
| Level | experimental |
| Components | internal/component/ike/wire |
| Real-path tests | test/ipsec/ipsec-sa-installed.ci, test/ipsec/ipsec-error-notify-no-loop.ci |
| Interop | ipsec/psk-site-to-site, ipsec/responder-psk, ipsec/ike-padded-probe-strongswan |
| RFCs | rfc7296 |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: the anchored package internal/component/ike/wire exists; the payload list is the RFC 7296 set the audit table checked against the codec |
| Defect review | 2026-10-07: audit found no open immediate spec and no journal row against the wire codec |

## Description

RFC 7296 wire codec: all payload types (SA, KE, Nonce, ID, AUTH, CERT, CERTREQ, Notify, Delete, Vendor, TSi/TSr, EAP, Configuration). Header encode/decode, payload chaining, encryption envelope. <!-- source: internal/component/ike/wire/ -- IKEv2 wire format codec -->
