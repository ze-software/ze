# 882 -- VLAN 802.1p QoS Maps (Ingress/Egress PCP-Priority)

## Context

Ze's VLAN sub-interface creation via netlink set only VlanId and ParentIndex. The kernel
supports per-VLAN QoS mapping between the 802.1p PCP field (3 bits in the 802.1Q tag
header) and the internal packet priority, via IFLA_VLAN_INGRESS_QOS and
IFLA_VLAN_EGRESS_QOS netlink attributes. The vendored vishvananda/netlink library already
exposes IngressQosMap and EgressQosMap on the Vlan struct. This spec wired the full
config-to-kernel path.

## Decisions

- VLANSpec struct over bare parameters, following the TunnelSpec precedent. Three backend
  implementations (netlink, stub, VPP) and three additional call sites (dispatch, CLI, cmd)
  would have needed a 6-site flag-day on every future field addition. The struct keeps the
  Backend interface signature stable.
- YANG lists keyed by the "from" side (ingress: key pcp, value priority; egress: key
  priority, value pcp), mirroring the kernel's from-to direction semantics.
- nil map means "unconfigured, do not emit netlink attribute." The vendor lib serializes
  any non-nil map including empty ones, so parseQoSMap normalizes empty lists to nil.
- Duplicate canonical key detection: "06" and "6" are distinct JSON keys but the same
  uint32 value. Silent last-write-wins would be nondeterministic (Go map iteration order).
  Reject at parse time.
- Defense in depth: validation at three layers (YANG range 0..7, parsePCPValue, netlink
  validateQoSMap) because the netlink backend is a trust boundary from config.
- VPP backend rejects non-empty QoS maps (exact-or-reject) because VPP's QoS
  record+mark pipeline is not wired. Silently dropping maps would misconfigure the network.
- dispatch.go public CreateVLAN(parent, vid) builds VLANSpec with nil maps internally,
  keeping CLI/cmd callers unchanged. QoS maps are config-only, not settable via
  interactive commands.

## Consequences

- Any future VLAN field (e.g., vlan-protocol for 802.1ad QinQ) adds to VLANSpec without
  touching the Backend interface signature or any call site except the netlink implementation.
- Kernel modify caveat (A-3): LinkModify for ingress maps ADDS entries, it does not replace
  the whole map. There is no per-entry delete via this netlink attribute. Config changes
  that remove an ingress mapping entry may require interface recreation to take effect.
  Documented in spec Known Limitations.

## Gotchas

- The vendor lib field names use "Qos" (capital Q, lowercase os) while Ze uses "QoS"
  (capital Q, capital S). Field assignment crosses the naming convention boundary:
  `IngressQosMap: spec.IngressQoSMap`. This is not a typo.
- PCP 0 maps to priority 0 by default in the kernel (identity mapping). Configuring
  `ingress-qos-map 0 { priority 0; }` is a no-op but not rejected. This is consistent
  with how iproute2 handles it.
- Maps with all 8 entries (0-7) are valid. The make() call caps at min(len, 8) to avoid
  oversizing from crafted JSON with duplicate-canonical keys that would be rejected anyway.

## Test Coverage

- 4 unit tests (parse, invalid table-driven with 10 cases, empty/nil normalization, apply wiring)
- 1 show_linux linkToInfo test
- 3 QEMU integration tests (create+read-back, absent-maps, modify probe) with netns isolation
- 3 .ci functional tests (parse valid, parse invalid, plugin full BNG chain)

## Files

None recorded.
