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
- A `reference-bandwidth` change re-prices an interface that configures no
  `cost`, and it MUST NOT restart it (owner decision, 2026-09-09). The restart
  is not what publishes the metric: `lsdbTopology` derives the cost from
  `e.cfg.ReferenceBandwidth` on every origination pass and `reconcile` replaces
  `e.cfg` before it reaches an interface, so the new cost is advertised whether
  or not the runtime is recreated. A carrier flap already re-prices a link with
  no restart at all. Recreating the runtime empties its neighbor map and clears
  its DR, so on a router with forty auto-costed links the numerator change cost
  forty adjacencies to publish a number the wire was going to carry anyway.
  `interfaceGlobalParamsChanged` therefore covers only what the runtime stamps
  into a packet, the Router ID and the area type, and `reconcile` re-prices
  every other interface in place through `repriceInterfaceLocked`.
  <!-- source: internal/plugins/ospf/instance.go -- interfaceGlobalParamsChanged, repriceInterfaceLocked, lsdbTopology -->
- `reconcile` originates the self-LSAs before it returns, so the commit
  publishes the reloaded config. A re-priced interface is not restarted, so no
  neighbor transition drives an origination for it, and the Router-LSA would
  otherwise carry the old metric until an unrelated event. The LSDB floods on a
  diff, so a reload that changed nothing emits nothing, and RFC 2328 Appendix B
  MinLSInterval still defers a second origination of one LSA.
  <!-- source: internal/plugins/ospf/instance.go -- reconcile, originateSelfLSAs -->
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
