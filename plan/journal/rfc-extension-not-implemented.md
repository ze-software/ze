| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-26 | rfc-profile-declared-membership | ike | RFC 5998 EAP-only authentication is absent: no EAP_ONLY_AUTHENTICATION (16417) notify, no safe-method list, no EAP-key-only AUTH; the vpn-concentrator profile reaches the RFC (internal/component/ike/wire/payload_notify.go, internal/component/ike/engine/eap_auth.go); rows RFC5998-3-1 to 3-4 and 4-1 | open: implement the extension in the IKE engine |
