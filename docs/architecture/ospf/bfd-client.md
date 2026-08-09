# BFD client integration for OSPF

How OSPF subscribes to the in-process BFD engine so that a Down event tears the
adjacency down inside the BFD detection window instead of the
`RouterDeadInterval` window. The BFD engine itself is in
`docs/architecture/bfd.md`.

## Decisions

- **A protocol reaches the BFD engine through an in-process registry, not a text
  protocol.** The BFD API package holds an atomic pointer to the live service,
  published when the plugin starts and cleared on shutdown. A client imports
  only that leaf package and reaches the engine in one atomic load. A dispatch
  round trip would add marshalling to every subscribe, unsubscribe and release.
  <!-- source: internal/component/bfd/api/registry.go -- SetService, GetService -->
- **The client is nil-safe by design.** A configuration that opts in while the
  BFD plugin is absent logs a warning and opens no session. BFD is additive: a
  missing BFD plugin never blocks the protocol.
  <!-- source: internal/plugins/ospf/bfd_client.go -- startBFDSession, bfdNeighborFull -->
- **One long-lived subscriber goroutine per session, never one per event.** It
  drains the state-change channel until the client stops it or the engine closes
  the subscription. This follows `ai/rules/goroutine-lifecycle.md`.
- **Down and AdminDown both tear the adjacency.** Both mean the forwarding path
  is unusable. AdminDown means the operator disabled the session the adjacency
  depends on. The check enumerates the two states explicitly, because a future
  state value is not readable as "link down" by a range comparison.
- **The Down handler drives the existing neighbor-down seam**, not a private
  state transition. Reusing it makes a BFD-driven teardown behave exactly like
  an operator-driven one: same logging, same events, same metrics.
- **The IPv6 divergence is the request builder alone.** The BFD engine already
  carries an IPv6 single-hop session end to end, with
  `IPV6_UNICAST_HOPS=255` on transmit and the `IPV6_RECVHOPLIMIT` control
  message for the receive-side GTSM check. OSPF adds NO transport code: only the
  link-local address pair differs from the IPv4 on-subnet pair.
  <!-- source: internal/plugins/ospf/bfd_client_v6.go -- bfdRequestForNeighborV6 -->
  <!-- source: internal/component/bfd/transport/dual.go -- Dual -->

## Traps

- **The BFD single-hop session uses GTSM with hop limit 255. Base OSPFv3
  multicast uses hop limit 1.** The two are independent. Do not unify them. See
  `ospfv3-3-ipv6-transport.md` for why OSPFv3 multicast must stay at 1.
- **The BFD state enum has `AdminDown = 0` and `Down = 1`.** These are RFC 5880
  wire values and cannot be renumbered, so a zero value is a meaningful state
  and not an absent one.
- A dual-family transport shares one device value, and `SO_BINDTODEVICE` applies
  identically to both families.
