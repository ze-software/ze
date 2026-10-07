# IPsec Data Model

## Meta

| Field | Value |
|-------|-------|
| Name | IPsec Data Model |
| Kind | daemon |
| Scope | partial |
| Scope gaps | close-action and dead-peer-detection action values all produce the same behavior, key-exchange ikev1 still negotiates IKEv2 |
| Level | experimental |
| Components | internal/component/ike/ipsec |
| Real-path tests | test/parse/ipsec-eap-auth.ci, test/parse/ipsec-psk-hex.ci, test/parse/ipsec-dh-group-range.ci, test/parse/ipsec-traffic-selector-port-needs-protocol.ci, test/ipsec/ipsec-traffic-selector-config.ci, test/reload/tx-ipsec-x509-requires-ca.ci |
| RFCs | rfc4301 |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: every source anchor resolves; the row now states that close-action, the DPD action and key-exchange ikev1 do not change behavior, matching the open immediate specs |
| Defect review | 2026-10-07: open: plan/immediate/spec-ipsec-close-action-and-dpd-action-select-nothing.md, spec-ipsec-key-exchange-ikev1-runs-ikev2.md; journal unwired-feature.md and documentation-shows-config-the-parser-refuses.md rows name this config; journal rows naming a Component, not each re-verified here: guard-enumerates-instead-of-subtracting.md:22, silent-fall-through.md:24 |
| Extra criteria | supported: every enum value of an action leaf has a test proving distinct behavior = test/ipsec/ipsec-close-action-values.ci; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

YANG `vpn { ipsec {} }` config for site-to-site VPN: ESP groups (proposals, lifetime, PFS), IKE groups (proposals, dead-peer detection, key-exchange, close-action; `close-action` and the dead-peer-detection `action` are parsed but every value produces the same behavior, and `key-exchange ikev1` still negotiates IKEv2), site-to-site peers (X.509 and PSK auth, VTI bind, group references). Algorithm enums match strongSwan naming. Cross-reference validation (group names, PKI certificates, interface binding, local-id/CN match). Operator-authored Security Policy Database entries under `vpn ipsec policy`, carrying the BYPASS and DISCARD dispositions of RFC 4301 Section 4.4.1 with a selector, a direction and a total order. Config diff detection for reload. <!-- source: internal/component/ike/ipsec/config.go -- IPsec config parser --> <!-- source: internal/component/ike/ipsec/types.go -- algorithm enums and struct types --> <!-- source: internal/component/ike/ipsec/validate.go -- cross-reference validation -->
