# Deferrals: followup-vpp-iface

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-followup-vpp-iface A-4 | BGP netns-aware listener: teach the BGP TCP listener to bind inside a named network namespace, so LCP TAPs in a non-root netns (`vpp.lcp.netns`) are reachable by BGP without forcing the operator to a root-reachable netns | Out of scope for the iface spec (BGP has zero netns awareness today, `reactor/listener.go`); `doctor-vpp-lcp-netns` makes the constraint visible meanwhile | `plan/spec-fixit-vpp-lcp-reachability.md` | done |
| 2026-07-10 | spec-followup-vpp-iface | Doctor check for VPP linux-cp plugin presence (parallel to `doctor-vpp-wireguard`): warn pre-apply when `lcp.enabled` but the VPP build lacks `linux_cp_plugin.so`, instead of failing the whole apply at the binapi layer | Needs a runtime GoVPP probe; the netns doctor check (delivered) does not cover plugin absence | `plan/spec-fixit-vpp-lcp-reachability.md` | done |

