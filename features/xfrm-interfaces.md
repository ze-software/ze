# XFRM Interfaces

## Meta

| Field | Value |
|-------|-------|
| Name | XFRM Interfaces |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/iface/netlink/xfrm_linux.go |
| Real-path tests | test/plugin/show-mtu-oversized-tunnels.ci, test/ui/doctor-ipsec-xfrm.ci |
| Interop | ipsec/mtu-tunnel-sizing-strongswan |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: the anchor xfrm_linux.go exists and the if-id leaf is in internal/component/iface/yang/ze-iface-conf.yang |
| Defect review | 2026-10-07: spec-ipsec-vti-bind-installs-if-id-zero is stale; journal rows naming a Component, not each re-verified here: unwired-feature.md:13 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Route-based IPsec via XFRM interfaces (`interface { xfrm <name> { if-id <N> } }`). Traffic routed through the interface is encrypted; traffic arriving is decrypted. Created and deleted via netlink. <!-- source: internal/plugins/iface/netlink/xfrm_linux.go -- XFRM interface lifecycle -->
