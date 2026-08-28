# Deferrals: vrrp-0-umbrella

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-vrrp-0-umbrella (Goal Validation AC-3) | Automated keepalived interop scenarios for RFC 3768 VRRPv2 (QS-5) and IPv6 VRRPv3 (QS-6) are NOT IMPLEMENTED in the retired `scripts/evidence/effective-vrrp-keepalived.py` (current producer: `internal/le/qemu/vrrp_keepalived_linux.go`) (enumerated in its `PENDING_SCENARIOS`). v3 IPv4 is proven against keepalived 2.3.1 (QS-1/2/3); v2 has codec + config-rejection coverage but no wire-exchange interop; IPv6 interop was demonstrated via ad-hoc scripts, not committed automation | Scope control: the v3 IPv4 interop lab is complete and the v2/IPv6 wire formats are golden-tested; automating the v2 and IPv6 keepalived scenarios is follow-on test infrastructure, not a protocol gap | `plan/spec-vrrp-deferred-keepalived-interop-scenarios.md` (tracks QS-4/5/6/7/8) | deferred |

