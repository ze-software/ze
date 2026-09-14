# OSPFv3 IPsec authentication

RFC 4552 manual-keyed IPsec AH and ESP for OSPFv3, as a separate path from the
in-packet RFC 7166 authentication trailer. Kernel XFRM state and policy are
wired onto the OSPFv3 IPv6 transport, with per-interface SPI and key config.
This path is Linux-only.

## Decisions

- **One transport-mode security association per OSPF destination, all on the
  interface's one SPI and key.** The kernel resolves a transport-mode state by
  the destination of the flow and nothing widens that: `xfrm_state_find` demands
  the state's own `id.daddr` equal it, and `__xfrm6_state_addr_check` wildcards
  the SOURCE alone. So every state carries source `::`, one destination, the
  reqid that binds it to the policies, and the `{::/0, ::/0, proto 89}` selector
  that keeps non-OSPF flows off it. RFC 4552 Section 7 gives them all the same
  SPI and key, because IKE cannot key a multicast group and every router on the
  link has to read what any other sent.
  <!-- source: internal/plugins/ospf/ipsec_install.go -- ipsecInstaller, buildIPsecSA -->
- **Three states when the interface opens, one more for each neighbor.**
  `ff02::5` and `ff02::6` carry what this router multicasts and what arrives
  addressed to the groups, so each serves both directions and is installed with
  no direction. The interface's own link-local is the destination of every
  unicast Database Description, Link State Request, Link State Update and
  acknowledgement a neighbor sends this router, and is inbound. A neighbor's
  link-local is where this router unicasts the same exchange back, so it is
  outbound, and it is known from the neighbor's first Hello and not before.
  <!-- source: internal/plugins/ospf/ipsec_install.go -- buildIPsecInterfaceSAs, installLocked -->
- **The neighbor state follows the adjacency, through the interface state
  machine's neighbor sink.** `nsmAdapter` is that sink. Its `NeighborHello`
  installs the neighbor's outbound state BEFORE it runs the neighbor state
  machine, because the machine sends the first Database Description inline and
  the kernel drops a packet whose policy resolves no state (RFC 4552 Section 9:
  "the routing module must install the corresponding SPD/SAD entries before
  starting these exchanges"). Every Hello repeats the call and a neighbor already
  keyed at the same address costs one map lookup; a neighbor that moved to a
  different link-local has its stale state removed first. `NeighborDown` removes
  the state on every drop the machine performs, the dead-interval expiry
  included, and `InterfaceDown` removes every neighbor's when the machine
  restarts on a config change. On a link-down the engine removes the whole
  interface first, neighbor states included. The IPv4 family and a virtual link
  carry no installer in their sink.
  <!-- source: internal/plugins/ospf/instance.go -- nsmAdapter, startInterfaceLocked -->
  <!-- source: internal/plugins/ospf/ipsec_install.go -- onNeighborSeen, onNeighborLost, clearNeighbors -->
- **Interface scoping uses the POLICY SELECTOR ifindex, not the XFRM interface
  id.** An interface id needs an xfrm-interface device, and a regular packet
  carries id 0, so setting it to an ifindex would match NO packet and disable
  IPsec silently.
- **Three interface-scoped protocol-89 policies**: out, in and forward, each
  with source and destination `::/0`. One policy per direction serves every
  destination: the template carries the reqid and no address, so the flow's own
  destination picks which of the interface's states resolves it.
- **One installer per v6 engine**, hooked into the per-address-family engine
  spawn rather than a single engine. Metric registration is name-idempotent, so
  per-engine registration is safe.
  <!-- source: internal/plugins/ospf/ipsec_metrics.go -- ipsecMetrics -->
  <!-- source: internal/plugins/ospf/config_ipsec.go -- parseIPsec -->
- **The SA and the policies are installed BEFORE the interface state machine
  starts**, so outbound traffic is protected before the first Hello.
- **Anti-replay is a per-interface leaf that defaults to OFF.** RFC 4302 §3.4.3 and
  RFC 4303 §3.4.3 make the service mandatory to implement and leave its use to the
  receiver, per SA, so `replay-window` is that switch and it serves AH and ESP alike.
  Zero is the default because RFC 4302 §5 says a compliant implementation SHOULD NOT
  provide anti-replay with a manually keyed SA, and every SA on this path is manually
  keyed. A non-zero window is 32 to 255: 32 is the RFC minimum, and 255 is the width of
  `dataplane.SAParams.ReplayWin`.
  <!-- source: internal/plugins/ospf/config_ipsec.go -- validateIPsecReplayWindow -->
  <!-- source: internal/plugins/ospf/ipsec_install.go -- buildIPsecSA -->
- **The shared IKE dataplane gained additive, zero-valued fields** for the
  selector, the upper protocol, the ifindex, policy removal and the algorithm
  plan. IKE is byte-identical with them at their zero values.

## Constraints on callers

- Two IPsec interfaces on one node require DISTINCT per-interface SPIs, because
  a state's identity is the destination, the SPI and the protocol, and the two
  multicast groups have the same destination on every link.
- The installer MUST be attached before the first interface starts
  (`installIPsecHooks` refuses otherwise): the interface's neighbor sink copies
  the installer when the interface starts, and a sink built without one would
  key no neighbor and say nothing.
- The doctor check reads the ONE kernel XFRM probe in the tree, so its row and
  the daemon's startup refusal cannot disagree about the same kernel. Only an
  ABSENT verdict warns: the shared probe opens the XFRM netlink socket rather
  than dumping the Security Policy Database, so it separates a kernel without
  XFRM from a process without CAP_NET_ADMIN, and the second is not evidence of an
  unprotected adjacency. The drop counters keep their own per-platform split.
  <!-- source: internal/plugins/ospf/doctor_ipsec.go -- checkOSPFv3IPsec, xfrmProbe -->
  <!-- source: internal/component/kernelcap/probe_linux.go -- XFRM, classifyXFRM -->
  <!-- source: internal/plugins/ospf/ipsec_drops_linux.go -- readXfrmDropsPlatform -->
  <!-- source: internal/plugins/ospf/ipsec_drops_other.go -- readXfrmDropsPlatform -->

## Traps

- **Two models shipped broken and the unit tests could not see either.** The
  first installed a multicast destination out and a link-local destination in
  with no traffic selector, so it could carry neither the unicast Database
  Description nor the ff02::6 group. The second installed ONE state under `::`
  for both directions, and a state under `::` is reachable by no flow at all:
  measured in QEMU, one datagram to ff02::5 advanced `XfrmOutNoStates` by
  exactly one. Every unit test asserted the parameters ze BUILDS, which are
  true of a state no packet can reach. Only a test that reads the kernel's own
  answer, or an adjacency that reaches Full through the kernel, can tell.
- **Half a model is no model.** The per-destination installer landed with the
  neighbor methods written and uncalled, so the multicast Hello exchange was
  protected and every unicast packet this router sent was dropped: FRR sat in
  `ExStart/PointToPoint` for the whole 90-second wait. The interop scenarios
  `ospf-ipsec-frr` and `ospf-ipsec-ah-frr` are the goal validators for this
  page: the checker keys FRR's kernel with `ip xfrm` first (FRR ospf6d cannot
  from its own config), waits for Full on FRR's side, then reads the installed
  state and policy back out of both kernels. Before them the two scenarios named
  interfaces the lab does not create and installed nothing on FRR, so they never
  reached IPsec at all.
  <!-- source: internal/le/interoplab/bgp/checkers.go -- ospfIPsecPeerSetup -->
- **A startup window remains open.** The socket opens and joins ff02::5 before
  the inbound require-policy installs. Closing it needs the shared engine
  start hook split into a pre-join and a post-open half.
- **An enabled replay window breaks the adjacency when the PEER restarts.** The peer's
  sequence counter returns to 1 while the receiver's window does not, so every packet
  looks replayed until the receiving SA is reinstalled. Nothing rekeys a manual SA, which
  is the reason RFC 4302 §5 advises against anti-replay here and the reason the leaf
  defaults to zero.
- A null encryption name maps to `ecb(cipher_null)`, and some kernels expect
  `cipher_null`. Verify against the target kernel.
