# OSPF plugin registration and config

The config-to-engine backbone: the plugin root, the YANG config tree, the SDK
lifecycle callbacks, transport enrollment and config validation.

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
- **One function answers what an interface costs, and every consumer calls it.**
  The Router-LSA topology, the `show ospf interface` runtime config, the
  LDP-sync restore value and the RFC 3630 TE metric fallback each need the same
  number, and each of them held its own copy of the "cost, or 1 when unset"
  rule before auto-cost existed. Auto-cost has two more cases (an unknown link
  speed and an unset reference bandwidth), so four copies would have been four
  chances to price a link differently from the LSA that advertises it.
  <!-- source: internal/plugins/ospf/interface_cost.go -- interfaceCost -->
- **The auto-cost numerator is one router-wide number, inherited by every
  address family.** `reference-bandwidth` is declared in the top-level container
  and the RFC 5838 `ospf-af-topology` grouping carries no leaf of its own, so an
  address-family sub-config holds the seeded default until `parseOSPFConfig`
  inherits the parent value into it, beside the Router ID, the Router
  Information, Graceful Restart and Fast Reroute. The inheritance is
  unconditional because no sub-config can state one. Without it the OSPFv2 and
  OSPFv3 Router-LSAs advertise two different costs for the same physical link,
  and no configuration makes them agree.
  <!-- source: internal/plugins/ospf/config.go -- parseOSPFConfig -->
- **The link speed is read on each origination pass, not cached.** A link
  renegotiates while OSPF runs, and a cached speed needs an invalidation path
  from the iface component into the plugin that nothing else in the plugin
  needs. Re-reading costs two sysfs reads per interface, beside the netlink
  address dumps `lsdbTopology` already performs on the same pass.
  <!-- source: internal/plugins/ospf/interface_cost.go -- interfaceLinkSpeedMbps -->

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
- A `reference-bandwidth` change or an interface `cost` leaf change MUST NOT
  restart the interface. This includes adding an explicit cost and removing it
  to restore auto-cost, in every address family (owner decision, 2026-09-10).
  `interfaceParamsEqual` excludes cost from its restart comparison.
  `repriceInterfaceLocked` replaces the enrolled config in `e.running` and
  updates the runtime cost without clearing neighbors or DR state.
  `lsdbTopology` reads that enrolled config on every origination pass, so both
  explicit and automatic costs reach the self-LSAs, including passive and
  loopback topology. Router ID, area type, and the other interface restart
  boundaries remain unchanged.
  <!-- source: internal/plugins/ospf/instance.go -- interfaceParamsEqual, interfaceGlobalParamsChanged, repriceInterfaceLocked, lsdbTopology -->
- `reconcile` attempts self-LSA origination before it returns. RFC 2328 Section
  12.4 requires a new instance when the described contents change, but
  MinLSInterval can defer publication. The one-second maintenance worker retries
  from the current topology, including passive-only and loopback-only configs.
  Initial enrollment and reload-added enrollment both start that worker.
  Unchanged LSA bodies do not flood.
  <!-- source: internal/plugins/ospf/instance.go -- reconcile, openInterfaces, openConfiguredInterface, startNeighborRetransmitLoop -->
  <!-- source: internal/plugins/ospf/lsdb/origination.go -- OriginateRouter, OriginateNetwork -->
- Which synthetic device reports a link speed is the kernel's decision, and it
  is not "none of them". A veth reports 10000 because its driver declares 10
  Gbit/s, which is what lets a Docker container observe auto-cost at all and is
  why the `ospf-auto-cost-frr` interop scenario exists. That scenario is what
  proves the veth. The other synthetic devices are kernel behavior no test in
  this tree reads: a loopback and a dummy report nothing, and a bridge reports
  -1. What the tree does prove is the mapping, because `parseLinkSpeedDuplex`
  turns an absent, unparseable or negative value into 0, and all three then take
  cost 1. A unit test that wants a speed therefore replaces `interfaceLinkSpeedMbps`,
  because a test that forgets to depends on the host and usually exercises the
  unknown-speed branch instead.
  <!-- source: internal/plugins/ospf/interface_cost_test.go -- stubLinkSpeed -->
  <!-- source: internal/plugins/iface/netlink/show_linux.go -- parseLinkSpeedDuplex -->

The `ospf-auto-cost-frr` checker also reloads an explicit cost of 17 after Full
and then removes it to restore auto-cost 47. It reads both metrics from FRR's
LSDB and rejects Ze's `ll-down` event counter, which records interface resets
even if the adjacency has already recovered. Both configurations enable
Prometheus so the command can read those counters.
<!-- source: internal/le/interoplab/bgp/check_extras.go -- scenarioExtras ospf-auto-cost-frr -->
<!-- source: internal/plugins/ospf/neighbor/table.go -- interfaceDown, recordEventLocked -->
