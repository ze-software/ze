# Deferrals: vrrp-6-interop

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-15 | spec-vrrp-6-interop (Known Limitations) | `accept-mode` is not enforced on the dataplane: the leaf is parsed, validated (rejected under v2) and reported by `show vrrp`, but no RFC 9568 6.4.3 filtering is installed, so an Active non-owner answers traffic to the virtual IP whichever way the leaf is set. Also not implemented: priority-decrement tracking (interface/route/health) that Junos/Nokia/VyOS offer | Scope control: the virtual-MAC control and data plane and keepalived interop are complete; accept-mode filtering and object tracking are follow-on capabilities, documented in `docs/guide/vrrp.md` and the umbrella Known Limitations | `plan/spec-vrrp-deferred-accept-mode-dataplane.md` | deferred |
| 2026-07-15 | spec-vrrp-6-interop | VRRP on the VPP data plane (macvlan + raw sockets are netlink-only; a VPP-backed tree is rejected at validation rather than run degraded) | VPP-native owned-device + raw-socket support is a distinct backend effort; the netlink backend is complete and interop-proven | `plan/spec-vrrp-7-vpp.md` (skeleton) | deferred |

