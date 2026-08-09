# OSPFv3 IPsec authentication

RFC 4552 manual-keyed IPsec AH and ESP for OSPFv3, as a separate path from the
in-packet RFC 7166 authentication trailer. Kernel XFRM state and policy are
wired onto the OSPFv3 IPv6 transport, with per-interface SPI and key config.
This path is Linux-only.

## Decisions

- **ONE shared wildcard transport-mode security association per interface**,
  with source and destination `::`, bound by reqid and carrying the OSPF
  protocol-89 traffic selector. It serves BOTH egress protection and ingress
  verification (RFC 4552 Section 7 shares the SA, the SPI and the key). The
  naive one-in and one-out pair cannot work: two identical wildcard states with
  the same SPI collide.
  <!-- source: internal/plugins/ospf/ipsec_install.go -- ipsecInstaller, newIPsecInstaller -->
- **Interface scoping uses the POLICY SELECTOR ifindex, not the XFRM interface
  id.** An interface id needs an xfrm-interface device, and a regular packet
  carries id 0, so setting it to an ifindex would match NO packet and disable
  IPsec silently.
- **Three interface-scoped protocol-89 policies**: out, in and forward, each
  with source and destination `::/0`.
- **One installer per v6 engine**, hooked into the per-address-family engine
  spawn rather than a single engine. Metric registration is name-idempotent, so
  per-engine registration is safe.
  <!-- source: internal/plugins/ospf/ipsec_metrics.go -- ipsecMetrics -->
  <!-- source: internal/plugins/ospf/config_ipsec.go -- parseIPsec -->
- **The SA and the policies are installed BEFORE the interface state machine
  starts**, so outbound traffic is protected before the first Hello.
- **The shared IKE dataplane gained additive, zero-valued fields** for the
  selector, the upper protocol, the ifindex, policy removal and the algorithm
  plan. IKE is byte-identical with them at their zero values.

## Constraints on callers

- Two IPsec interfaces on one node require DISTINCT per-interface SPIs, because
  the shared wildcard state's identity is the destination, the SPI and the
  protocol.
- The doctor check has a per-platform split, and the drop counters do too.
  <!-- source: internal/plugins/ospf/doctor_ipsec.go -- checkOSPFv3IPsec -->
  <!-- source: internal/plugins/ospf/doctor_ipsec_linux.go -- xfrmAvailable -->
  <!-- source: internal/plugins/ospf/doctor_ipsec_other.go -- xfrmAvailable -->
  <!-- source: internal/plugins/ospf/ipsec_drops_linux.go -- readXfrmDropsPlatform -->
  <!-- source: internal/plugins/ospf/ipsec_drops_other.go -- readXfrmDropsPlatform -->

## Traps

- **The first model shipped broken and the tests could not see it.** Two SAs
  with a multicast destination out and a link-local destination in, and no
  traffic selector, cannot carry unicast Database Description, the ff02::6
  group, or a unicast LS Update retransmit, so the adjacency stalls before Full.
  The interop tests were skip no-ops for an unvalidated Linux feature, so only
  review caught it. A skip is not coverage.
- **A startup window remains open.** The socket opens and joins ff02::5 before
  the inbound require-policy installs. Closing it needs the shared engine
  start hook split into a pre-join and a post-open half.
- A null encryption name maps to `ecb(cipher_null)`, and some kernels expect
  `cipher_null`. Verify against the target kernel.
