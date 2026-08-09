# OSPF plugin registration and config

The config-to-engine backbone: the plugin root, the YANG config tree, the SDK
lifecycle callbacks, transport enrolment and config validation.

## Decisions

- **OSPF is a self-contained plugin under `internal/plugins/ospf`, not a central
  component.** Protocol runtime, YANG, doc anchors and tests stay together, as
  they do for IS-IS and LDP.
  <!-- source: internal/plugins/ospf/register.go -- runOSPFEngine -->
- **Areas bind per interface, not by `network <prefix> area`.** Ze config names
  the interface that owns OSPF state. Hidden prefix matching is rejected.
  <!-- source: internal/plugins/ospf/config.go -- parseOSPFConfig -->
- **Router-id and area-id validators live in the central config package.** The
  config package owns custom YANG validation and cannot import the OSPF plugin
  without an import cycle.
  <!-- source: internal/component/config/validators.go -- ospfRouterIDValidator, ospfAreaIDValidator -->
- **The dispatcher validates by the RECEIVING interface area, not by the
  declared area alone.** ISM, NSM and LSDB code must never see a packet from the
  wrong area on a valid interface.
  <!-- source: internal/plugins/ospf/dispatcher.go -- dispatcher -->
- **The Go parser enforces the IPv4 range as well as YANG.** Unit tests and SDK
  paths call the resolver directly, outside native YANG validation.

## Constraints on callers

- `ze config validate` needs a static allow-list entry for each new top-level
  config root. A new YANG module alone does not make it validate through the
  CLI.
- OSPF depends on `interface` as well as `fib-kernel` and `sysctl`. Router-id
  derivation calls the iface backend during config verification when the
  operator omits `router-id`.
- The transport exposes link-up and link-down callbacks because engine running
  state tracks raw socket lifecycle across carrier flaps and config reloads.

## Traps

- A reload-added interface must call `startReceiveLoop` from `openInterface`,
  not only at startup. A config that starts empty otherwise opens sockets that
  dispatch nothing.
  <!-- source: internal/plugins/ospf/transport/transport.go -- Transport -->
- Area IDs need duplicate detection after canonical parsing. `area 0` and
  `area 0.0.0.0` are distinct YANG keys and the same OSPF area.
  <!-- source: internal/plugins/ospf/area.go -- area -->
- `zt:ip-prefix` accepts IPv4 and IPv6. OSPFv2 range leaves use
  `zt:prefix-ipv4` and the Go parser still rejects IPv6.
- `0.0.0.0` passes a naive dotted-quad validator and is the zero `RouterID`
  value in Go. The custom validator rejects the unspecified router id.
- A slow `HandleLinkUp` races `DisableInterface`. Recheck `enabled` under the
  transport lock before publishing the socket, or the interface stays joined
  after removal.
