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
- **An interface `bfd` block does not start the BFD plugin; the top-level `bfd`
  container does.** The plugin owns the `bfd` config root, and config-path
  auto-load matches top-level roots only, so `ospf { interfaces { interface eth0
  { bfd { enabled true } } } }` alone leaves the service unpublished and OSPF on
  its Hello/Dead timers. A configuration that wants OSPF BFD also carries
  `bfd { enabled true; }`, as the `ospf-bfd-frr` interop scenario does. OSPFv3
  also needs `bind-v6 true` in that container: it defaults to false, and without
  it the loop holds only an IPv4 socket, so every Control packet to an IPv6
  neighbour fails to send (`ospfv3-bfd-frr` sets both).
  <!-- source: internal/component/plugin/server/startup_autoload.go -- getConfigPathPlugins -->
  <!-- source: internal/component/bfd/register.go -- init -->
  <!-- source: internal/component/bfd/bfd.go -- newTransport -->
- **One long-lived subscriber goroutine per session, never one per event.** It
  drains the state-change channel until the client stops it or the engine closes
  the subscription. This follows `ai/rules/goroutine-lifecycle.md`.
- **Only a path failure tears the adjacency; AdminDown does not.** RFC 5882
  Section 4.2: "If a BFD session transitions from Up state to AdminDown, or the
  session transitions from Up to Down because the remote system is indicating
  that the session is in state AdminDown, clients SHOULD NOT take any control
  protocol action." A local `AdminDown`, and a `Down` whose
  `api.StateChange.RemoteAdminDown` is set, are logged at debug and change no
  OSPF state: one end switched BFD off, and the path may be fine. Any other
  `Down` (detection time expired, neighbor signaled Down) declares the neighbor
  down. A path that really fails while BFD is off still drops the adjacency on
  the RouterDeadInterval, the independent liveness detection Section 3.2
  assumes. The check names each state explicitly, because a future state value
  is not readable as "link down" by a range comparison.
  <!-- source: internal/plugins/ospf/bfd_client.go -- runBFDSubscriber -->
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
