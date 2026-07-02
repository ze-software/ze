# 1043 - OSPF Virtual Links (RFC 2328 §15 / RFC 5340)

## Context

Virtual-link adjacencies across a transit area to repair a partitioned or
non-contiguous backbone, both address families. A virtual link is a synthetic
backbone (Area 0) point-to-point interface whose packets are ROUTED (unicast,
TTL>1 / hop-limit>1) across the transit area to the remote ABR. Base-only spec
built in a worktree; the single highest-risk integration because it changes the
dispatcher's receive demux.

## Decisions

- The V-bit goes in the TRANSIT area's Router-LSA (RFC 2328 §A.4.2 / §16.3 TransitCapability); the Type-4 virtual-link RECORD goes in the BACKBONE Router-LSA. (The spec's AC-8/AC-9 wording saying "backbone V-bit" is stale/wrong; code is RFC-correct.)
- A synthetic `ospfiface.Interface` (network type virtual, Area 0, MTUIgnore) is created per resolved virtual link; it sends via a routed transport (`SendPacketRouted`) and its FSM is the normal NSM.
- Received virtual-link packets are demuxed by (source RouterID + backbone Area + arrival on a REAL enrolled transit interface whose area == the vlink's transit area), NOT by ifindex (they arrive on the physical transit interface). This is a `virtualLinkTargetLocked` fallback in `acceptsArea`/`receiveTargetLocked`.
- IPv6 virtual links resolve GLOBAL src/dst from the transit-area Intra-Area-Prefix-LSAs (RFC 5340 §2.9), not the link-local next hop.
- Auth is inherited from the transit area (sends sign on the transit egress, receives verify on the transit interface); no separate virtual-interface key registration.

## Consequences

- The dispatcher `areaOK` signature changed to carry the full Header; the InstanceID drop (ext-12) still runs FIRST, then `acceptsArea` (with the virtual fallback), then authOK, then the handler; the AF-bit gate (ext-15), opaque delivery (ext-1), and IPsec (ext-16) are all preserved.
- `SendPacketRouted` was added to the engine Transport interface + both v4/v6 backends (v4 ignores src / TTL>1; v6 uses the global src / hop-limit>1), coexisting with ext-16's `InterfaceSource`.
- The §16.3 transit-area SPF pass is improve-only (never worsens a reachable route).

## Gotchas

- The virtual demux MUST NOT hijack a packet on a genuine backbone interface, nor accept one on an unknown/non-enrolled ifindex: require a REAL enrolled interface whose area is the vlink's transit area. Verify base Type 1-7 demux is unregressed (the whole-of-OSPF risk).
- The IPv6 half was initially dead (a referenced `virtuallink_v6.go` did not exist; `SendPacketRouted` rejected the zero source); the v6 endpoint resolution must be implemented and unit-tested, not left as a link-local placeholder.
- Adding a 24-byte slice to `ospfConfig` pushed it past the gocritic hugeParam threshold (immutable config-snapshot pass-by-value pattern); bump `.golangci.yml` sizeThreshold following precedent rather than converting 22 call sites to pointers.
- The raw-socket routed SEND (TTL>1 / hop-limit>1) is Linux/QEMU-only; the endpoint resolution + FSM + origination are unit-testable on darwin. AC-16 (v6 `show ospf ipv6` virtual rows) is blocked on the v6 show surface; data path works.

## Files

- `internal/plugins/ospf/virtual_link.go`, `virtuallink_v6.go`, `spf/transitarea.go` (+ tests)
- `internal/plugins/ospf/{instance,config,register,cmd_show,spf_wiring,transport_iface,afstrategy_v6,origination_v6,auth_keystore}.go`, `dispatcher.go` (areaOK), `lsdb/{flooding,origination}.go`, `spf/{afstrategy,computer}.go`, `transport/{transport,backend_linux}.go`, `v3/transport/{transport,backend_linux}.go`
- `internal/plugins/ospf/yang/ze-ospf-{conf,cmd}.yang`
- `test/ospf/ospf-virtual-link-config.ci`, `test/ospfv3/ospfv3-vlink*.ci`, `test/interop/scenarios/{ospf-virtual-link-frr,ospfv3-vlink-frr}/`
