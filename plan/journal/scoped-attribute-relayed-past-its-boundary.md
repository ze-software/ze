| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | bgp-rpki-ov-community-ebgp-boundary | BGP extended-community relay (`bgp/reactor`) | Type 0x43 origin validation state community from/to EBGP was relayed across the AS boundary instead of dropped/stripped by default (RFC 8097 §2) | fixed: EBGP inbound drop (`accept-origin-validation-state`, default false) and outbound strip (`send-origin-validation-state`, default false); `rfc8097_origin_validation_state_test.go`, `rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` |
