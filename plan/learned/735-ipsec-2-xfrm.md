# 735 -- ipsec-2-xfrm

## Context

Ze had no XFRM interface support. XFRM interfaces (Linux 4.19+) are the modern
kernel mechanism for route-based IPsec: traffic routed into the interface is
encrypted/decrypted by the kernel XFRM subsystem, bound via if_id. The older VTI
mechanism was deliberately not supported. The spec also required that XFRM interfaces
created outside Ze (by strongSwan, manual `ip link add`) are visible in operational
mode but not modifiable or present in config.

## Decisions

- **XFRM as a standalone type, not a TunnelKind**, over adding it to the tunnel
  encapsulation choice. XFRM has no local/remote endpoints (the SA peer is in IPsec
  config), so it does not fit the tunnel model. Follows the WireguardSpec pattern:
  separate struct, separate Backend methods, separate YANG list.
- **VTI deliberately excluded**, over supporting both old and new mechanisms. XFRM
  interfaces are the kernel-maintainer-recommended replacement; no value in carrying
  the legacy VTI mechanism.
- **GetXFRMInfo added to Backend**, over putting the query in a separate interface.
  Read-only netlink queries for if_id and XFRM policies fit naturally alongside
  GetWireguardDevice. Used by show commands for both managed and unmanaged interfaces.
- **Unmanaged XFRM interfaces are operational-mode only**, over including them in
  config tree. Discovery classifies them at startup; show commands display them with
  full detail; Ze never modifies or deletes them.
- **Recreate-on-reconcile for parameter changes**, matching the tunnel pattern. Linux
  does not support in-place modification of XFRM if_id or parent dev.

## Consequences

- The native IKEv2 engine (ipsec-7/8) binds Child SAs to XFRM interfaces by if_id
  without any changes to the iface component. The interface lifecycle is fully managed.
- The XFRM YANG list uses `interface-common` (no MAC), matching wireguard and L3 tunnels.
- `SupportedTypes()` now returns "xfrm", so UI components enumerate it automatically.
- The `GetXFRMInfo` pattern could be reused for future interface-type-specific queries
  (e.g., bond status, VXLAN FDB) without bloating the generic `InterfaceInfo` struct.
- `XFRMInfo` carries `ParentDev` (resolved name) and `Addresses` (CIDR strings from
  netlink AddrList), so `ze interface scan --config` can emit a complete config block
  for onboarding externally-created XFRM interfaces without reading any external
  program's config file. The same `DiscoverInterfaces` -> `EmitConfig` path that
  `ze init` uses handles XFRM automatically.
- `ze interface show <name>` displays XFRM detail (if_id, dev, policies) when the
  interface type is "xfrm", via a dispatch.GetXFRMInfo call.

## Gotchas

- The vendored `netlink.Xfrmi` struct has `Ifid` but uses `ParentIndex` from
  `LinkAttrs` for the parent device binding (not a dedicated field). Setting
  `ParentIndex` requires a `LinkByName` lookup of the parent device.
- `XfrmPolicyList` returns ALL policies system-wide. Filtering by `Ifid` must be
  done client-side. On systems with many policies this could be slow; acceptable
  for show commands but would need caching if used in a hot path.
- The hook system blocks `fmt.Sprintf` and `strconv.FormatInt` in production code;
  `textbuf.Int()` / `textbuf.Uint()` must be used instead for numeric-to-string
  conversion.

## Files

- `internal/component/iface/xfrm.go` -- XFRMSpec, XFRMInfo, XFRMPolicyInfo types
- `internal/component/iface/backend.go` -- +CreateXFRM, +GetXFRMInfo on Backend interface
- `internal/component/iface/config.go` -- +xfrmEntry, +parseXFRMEntry, XFRM in parseIfaceConfig
- `internal/component/iface/config_apply.go` -- +indexXFRMSpecs, +xfrmSpecEqual, XFRM in desiredState/applyConfig
- `internal/component/iface/discover.go` -- +zeTypeXFRM, infoToZeType mapping, SupportedTypes
- `internal/component/iface/register.go` -- XFRM in DHCP reconciliation and RA suppression
- `internal/component/iface/yang/ze-iface-conf.yang` -- list xfrm
- `internal/plugins/iface/netlink/xfrm_linux.go` -- CreateXFRM, GetXFRMInfo netlink impl
- `internal/plugins/iface/netlink/backend_other.go` -- stubs
- `internal/plugins/iface/vpp/ifacevpp.go` -- stubs
- `internal/component/iface/dispatch.go` -- +GetXFRMInfo dispatch wrapper
- `internal/component/iface/emit.go` -- +emitXFRMBlock, +emitXFRMSet for ze init / scan --config
- `internal/component/iface/iface.go` -- +XFRM field on DiscoveredInterface
- `internal/component/iface/cli/show.go` -- +showXFRMDetail for `ze interface show <xfrm-name>`
- `internal/component/iface/cli/scan.go` -- "xfrm" added to managed filter
- `internal/component/iface/config_test.go` -- 8 XFRM tests
- `internal/component/iface/discover_test.go` -- 2 XFRM tests
- `internal/component/iface/emit_test.go` -- 4 XFRM emit tests
- `internal/component/iface/migrate_linux_test.go` -- mock updated
- `docs/features/interfaces.md` -- capability table + backend table
