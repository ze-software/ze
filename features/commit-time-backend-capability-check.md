# Commit-Time Backend Capability Check

## Meta

| Field | Value |
|-------|-------|
| Name | Commit-Time Backend Capability Check |
| Kind | daemon |
| Scope | complete |
| Level | supported |
| Components | internal/component/config/backend_gate.go |
| Real-path tests | test/parse/iface-vpp-rejects-bridge.ci, test/parse/iface-vpp-rejects-tunnel.ci, test/parse/iface-vpp-accepts-wireguard.ci, test/parse/iface-vpp-rejects-veth.ci, test/traffic/traffic-vpp-reject-hfsc.ci |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: re-read at promotion; the row named wireguard and mirror as netlink-only, but ze-iface-conf.yang annotates both `ze:backend "netlink vpp"` and iface-vpp-accepts-wireguard.ci asserts vpp accepts wireguard, so both were removed; veth, bridge and tunnel carry `ze:backend "netlink"`, ze-firewall-conf.yang carries seven `ze:backend "nft"` |
| Defect review | 2026-10-07: audit found no open immediate spec and no journal row against backend_gate.go |

## Description

YANG nodes that correspond to backend-specific features carry a `ze:backend "<names>"` annotation. On commit (daemon reload, first-apply, and `ze config validate`), the walker rejects the config with the YANG path and the list of supporting backends whenever the active backend does not implement the feature -- instead of letting an Apply-time "not supported" error fire inside the backend. The gate covers `interface` (netlink-only bridge, tunnel and veth under the vpp backend), `traffic/control` (the backend-leaf CALL wired into `OnConfigure`/`OnConfigVerify`; tc-only feature annotations ship with `spec-fw-7-traffic-vpp`), and `firewall` (seven `ze:backend "nft"` annotations on conntrack-driven matches and nft-only action/modifier leaves). <!-- source: internal/component/config/yang/modules/ze-extensions.yang -- extension backend --> <!-- source: internal/component/config/backend_gate.go -- ValidateBackendFeatures, walkBackendNode --> <!-- source: internal/component/iface/register.go -- validateBackendGate wired into OnConfigure and OnConfigVerify --> <!-- source: internal/component/traffic/register.go -- validateBackendGate wired into traffic OnConfigure and OnConfigVerify --> <!-- source: internal/component/firewall/engine.go -- validateBackendGate wired into firewall OnConfigure and OnConfigVerify -->
