# Linux Limitations Register

This page is the one place where Ze records a Linux kernel behavior, or a
library behavior on the path to the kernel, that stops Ze from implementing a
feature or an RFC requirement the way the standard asks. Every other surface
(an `rfc/short/` annotation, a guide, a code comment, an error message) names
the entry id and does not restate the limitation.

Owner order, 2026-10-20: "correctly document any linux limitation preventing us
to implement a feature in single global register so that on kernel update we
can check it gain, record what kernel was checked so we can perform regular
review."

## What belongs here

An entry is a behavior of the kernel Ze ships, or of a library version
`go.mod` requires, read in its source, that falls in one of two effects:

| Effect | Inclusion rule |
|--------|----------------|
| `blocks` | Ze cannot implement a feature or an RFC requirement on Linux the way the standard asks, whatever Ze's own code does |
| `constrains design` | Ze can implement the feature, but only in a shape the behavior forces: a workaround, an extra step, a carried patch, or a refusal Ze chose because the direct design would be wrong. A kernel or library change would let Ze simplify or lift it |

Left out: a gap whose fix lies in Ze's code with the kernel as it is, a Linux
behavior no Ze feature needs, and a behavior the kernel version Ze ships no
longer has. Each candidate that was considered and left out is listed under
"Considered and left out", with the reason, so the next pass does not redo it.

## How an entry is written

Each entry is a `###` heading that holds its id, followed by these fields:

| Field | Content |
|-------|---------|
| Kind | `kernel` or `library` (a Go module, never the kernel) |
| Effect | `blocks` or `constrains design`, by the rule above |
| Status | `verified` (the source was read at the version in `Checked`) or `unverified` (what would settle it is stated) |
| Checked | `Linux <version>, <date>` or `module <path> <version>, <date>` on one line |
| Limitation | The behavior, in plain words |
| Blocks | The feature, and the RFC requirement ids where one exists |
| Evidence | Each file and function read, with the line quoted from that source |
| Alternatives | Optional: a design that looks like it avoids the limitation, and the source line that defeats it |
| Re-check | What to read, and what result lifts or keeps the limitation |
| Ze meanwhile | What Ze does now: refuse, work around, or carry a gap |

`TestLinuxLimitsCheckedAtShippedVersions` (`internal/appliance/linux_limits_test.go`)
fails when a `kernel` entry's `Checked` version differs from
`internal/appliance/kernel.version`, or a `library` entry's version differs
from the requirement in `go.mod`. Both this test and the RFC gate read entries
through `internal/le/linuxlimit.Parse`. The parser refuses a heading under
`## Entries` that is not an `LL-` id, duplicate entry ids, missing `Kind`,
`Effect`, `Status` or `Checked` lines, repeated fields, malformed or unknown
field names, values outside the forms above, and dates that are not calendar
dates. It returns no entries when any form is invalid. Indented continuation
lines belong to the preceding field; `Blocks`, `Evidence` and `Re-check` retain
them, so a requirement id or source quote on a continuation is not lost.
A kernel bump therefore owes a pass over this page: re-read each entry at the
new version, update `Checked`, and lift any entry the new source no longer
supports.

<!-- source: internal/appliance/linux_limits_test.go -- TestLinuxLimitsCheckedAtShippedVersions -->
<!-- source: internal/le/linuxlimit/parse.go -- Parse, ValidID, readLimitChecked -->

An RFC row can name an entry as `{linux-limit: LL-<ID>; <reason>}` only when
its `Kind` is `kernel`, its `Effect` is `blocks`, and its `Status` is `verified`.
The entry's `Blocks` must name that exact requirement id, and `Evidence` must
not be empty. A library limitation can be fixed by carrying a library change;
a design constraint means Ze can implement the behavior in another shape.
Neither establishes that Linux cannot meet the requirement. An unverified
entry's `Re-check` states what must be settled before it can support the claim.
An annotation names the entry rather than copying its evidence, and does not
make the requirement implemented or proven.
<!-- source: internal/le/rfc/linux_limit.go -- linuxLimitEntryRefusal -->
<!-- source: internal/le/rfc/profilecoverage.go -- rowCounts -->

## How to fetch the source for a re-check

Fetch one file at the release tag into the session scratch directory, never
into `/tmp`:

```
curl -sfL -o <scratch>/<file> \
  "https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/plain/<file>?h=v<version>"
```

For a `library` entry, read the module in the module cache:
`$(go env GOMODCACHE)/<module>@<version>/<file>`.

## Entries

### LL-PROXY-ARP-MAC

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** A proxy ARP reply carries the hardware address of the
  device that received the request (`dev->dev_addr`), with no setting that
  names another MAC. A broadcast request that arrives on a macvlan's parent is
  also passed up the parent, so a parent with proxy ARP on answers with the
  parent's MAC.
- **Blocks:** proxy ARP answered by the parent of a VRRP group's virtual-MAC
  macvlan. RFC9568-8.1.3-1, RFC5798-8.1.3-1, RFC3768-8.3-1 ("the VRRP Router
  MUST advertise the Virtual Router MAC address in the Proxy ARP message")
  hold only for a design that answers from somewhere else; see Re-check.
- **Evidence:** `net/ipv4/arp.c` `arp_process`, the forwarding branch taken
  when `arp_fwd_proxy(in_dev, dev, rt) || arp_fwd_pvlan(in_dev, dev, rt, sip, tip)`
  holds, replies with `arp_send_dst(ARPOP_REPLY, ETH_P_ARP, sip, dev, tip, sha, dev->dev_addr, sha, reply_dst);`,
  where `dev` is the receiving device.
  `drivers/net/macvlan.c` `macvlan_handle_frame` queues a broadcast to the
  macvlans with `macvlan_broadcast_enqueue(port, src, skb);` and then
  `return RX_HANDLER_PASS;`, so the parent processes the frame too.
- **Re-check:** read the `arp_send_dst(ARPOP_REPLY` calls in `arp_process`;
  the entry stays while each takes `dev->dev_addr`. Two designs are untested,
  and either could make Ze answer with the Virtual Router MAC on this kernel:
  (a) proxy ARP (`proxy_arp`, or a `ip neigh add proxy` entry) on the
  virtual-MAC macvlan only, with the parent off. `arp_fwd_proxy` answers when
  `rt->dst.dev != dev` and `IN_DEV_PROXY_ARP(in_dev)` holds, and the reply would
  then carry the macvlan's MAC, the Virtual Router MAC. A Backup router keeps
  its macvlan up with the same MAC, so it would answer too unless an `arp`
  family rule drops the request there; the Backup filter `ze_vrrp_backup` is an
  `inet` table and carries no ARP. (b) an nftables `arp` family rule on the
  output hook that rewrites the sender MAC, `arp saddr ether set <vmac>`; the
  appliance kernel has `CONFIG_NF_TABLES_ARP=y` and `CONFIG_NFT_PAYLOAD=y`
  (`gokrazy/kernel/kernel.config`). The test that settles both is a QEMU
  functional test, `test/vrrp/vrrp-proxy-arp-vmac.ci` (not yet written), on <!-- doc-links: ignore (planned test, not an existing fixture) -->
  the pattern of `vrrp-track.ci`: an Active and a Backup router, a host that
  sends an ARP request for a proxied address, and an AF_PACKET capture that
  reads the sender MAC of every reply and checks the Backup sends none.
- **Ze meanwhile:** refuses proxy ARP on every unit that carries an IPv4 VRRP
  group, at config verify and on a runtime `set sysctl`. This is Ze's choice,
  approved by the owner on 2026-10-20, not something the kernel forces: the
  parent design is wrong by the source above, and (a) and (b) are unproven.

<!-- source: internal/plugins/vrrp/proxyarp.go -- validateNoProxyARP, refuseLiveProxyARP -->

### LL-NS-SOURCE-MAC

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** A Neighbor Solicitation the kernel sends carries, in its
  source link-layer address option, the MAC of the device that holds the
  neighbor entry. That is the device the route to the host resolves through.
  No option selects the MAC by source address.
- **Blocks:** RFC9568-8.2.2-2, RFC9568-8.2.2-3, RFC5798-8.2.2-2,
  RFC5798-8.2.2-3: the Active Router's solicitation for a host carries the
  parent's physical MAC while the subnet route stays on the parent.
- **Evidence:** `net/ipv6/ndisc.c` `ndisc_solicit` takes
  `struct net_device *dev = neigh->dev;`, and `ndisc_ns_create` fills the
  option with `ndisc_fill_addr_option(skb, ND_OPT_SOURCE_LL_ADDR, dev->dev_addr,`.
- **Re-check:** read `ndisc_ns_create`. The limitation is lifted only if the
  option takes an address other than `dev->dev_addr`.
- **Ze meanwhile:** conformance gap. The fix is to make the solicitation
  leave the virtual-MAC macvlan, which is routing, not a kernel change:
  `plan/release/fixes/spec-vrrp-owner-arp-rfc-defects.md`.

<!-- source: internal/plugins/vrrp/ownerfilter.go -- owner filter drops parent Neighbor Advertisements only -->

### LL-IPV6-LINKLOCAL-FROM-MAC

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** On link-up, a device whose `addr_gen_mode` is `eui64` (the
  default) gets a link-local address built from its MAC (Modified EUI-64). A
  macvlan carrying the Virtual Router MAC would get a link-local derived from
  it unless the mode is changed to `none` before link-up.
- **Blocks:** RFC9568-7.4-1 ("The Virtual Router MAC MUST NOT be used for the
  Net_Iface parameter used in the Interface Identifier (IID) derivation
  algorithms") under the default mode.
- **Evidence:** `net/ipv6/addrconf.c` `addrconf_addr_gen`:
  `case IN6_ADDR_GEN_MODE_EUI64:` then
  `if (ipv6_generate_eui64(addr.s6_addr + 8, idev->dev) == 0)`, and
  `case IN6_ADDR_GEN_MODE_NONE:` with `/* will not add any link local address */`.
  `ipv6_generate_eui64` answers `case ARPHRD_ETHER:` with
  `return addrconf_ifid_eui48(eui, dev);`, and `include/net/addrconf.h`
  `addrconf_ifid_eui48` builds the identifier from the MAC:
  `addrconf_addr_eui48_base(eui, dev->dev_addr);`.
- **Re-check:** read `addrconf_addr_gen`. The workaround stays valid while
  `IN6_ADDR_GEN_MODE_NONE` adds no address.
- **Ze meanwhile:** works around it. iface sets `addr_gen_mode` none on every
  owned macvlan before link-up.

<!-- source: internal/plugins/iface/netlink/macvlan_linux.go -- setMacvlanAddrGenNone -->

### LL-IPV4-CONF-ALL-WINS

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** For `rp_filter`, `arp_ignore` and `arp_announce` the
  effective value is the larger of `conf.all` and the device's own, so a
  device cannot go below `all`. `proxy_arp` is the OR of the two, so
  `all.proxy_arp=1` turns proxy ARP on for every device.
- **Blocks:** disabling reverse-path filtering on one device (the VRRP
  virtual-MAC macvlan, an OSPF unnumbered device) while `all.rp_filter` is
  1 or 2.
- **Evidence:** `include/linux/inetdevice.h`:
  `#define IN_DEV_MAXCONF(in_dev, attr) \ (max(IPV4_DEVCONF_ALL_RO(dev_net(in_dev->dev), attr), \ IN_DEV_CONF_GET((in_dev), attr)))`,
  `#define IN_DEV_RPFILTER(in_dev)		IN_DEV_MAXCONF((in_dev), RP_FILTER)`,
  `#define IN_DEV_ARP_IGNORE(in_dev)	IN_DEV_MAXCONF((in_dev), ARP_IGNORE)`,
  `#define IN_DEV_PROXY_ARP(in_dev)	IN_DEV_ORCONF((in_dev), PROXY_ARP)`.
- **Re-check:** read those macros in `include/linux/inetdevice.h`.
- **Ze meanwhile:** works around it. The VRRP plugin sets `all.rp_filter=0`
  while an IPv4 group exists and restores the saved value on teardown; the
  OSPF guide tells the operator to keep `rp_filter` 0 on `all` and the
  unnumbered device.

<!-- source: internal/plugins/vrrp/dataplane_linux.go -- globalDataplaneSysctls, applyDataplaneSysctls -->

### LL-XFRM-STATE-ONE-SELECTOR

- **Kind:** kernel
- **Effect:** blocks
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** An XFRM state holds one selector, and inbound processing
  drops a packet that does not match the selector of the state that
  decrypted it. A transport-mode Child SA therefore carries one selector pair:
  a selector wide enough for several pairs admits traffic no pair negotiated.
- **Blocks:** a transport-mode Child SA with several traffic selector pairs
  (RFC 7296 Section 2.9 lets a peer propose several).
- **Evidence:** `include/net/xfrm.h` `struct xfrm_state` has one
  `struct xfrm_selector	sel;`. `net/xfrm/xfrm_policy.c` `__xfrm_policy_check`
  checks each state that decrypted the packet:
  `if (!xfrm_selector_match(&x->sel, &fl, family)) {`, then excuses an ICMP
  error whose inner packet matches, `if (x->props.flags & XFRM_STATE_ICMP &&`
  `xfrm_selector_inner_icmp_match(skb, family, &x->sel, &fl))` `ret = 0;`,
  and otherwise counts `XFRM_INC_STATS(net, LINUX_MIB_XFRMINSTATEMISMATCH);`
  and drops.
- **Alternatives:** a covering selector does not help: one state whose selector covers
  every pair, with one inbound policy per pair, still admits traffic no pair
  negotiated. In the same function a packet that matches no policy is accepted
  unless it went through a non-transport state:
  `if (sp && secpath_has_nontransport(sp, 0, &xerr_idx) && !is_crypto_offload) {`
  rejects, and otherwise `return 1;`. A matching policy with no template (a
  bypass) runs no template check, and the closing
  `if (secpath_has_nontransport(sp, k, &xerr_idx)) {` again rejects only a
  non-transport state. The one exception is a namespace whose default input
  policy is block (`XFRM_USERPOLICY_BLOCK`), which drops every packet that
  matches no policy, protected or not.
- **Re-check:** read `struct xfrm_state` and `__xfrm_policy_check`. Lifted
  if a state gains a selector list, or if a transport-mode decrypted packet
  that matches no policy, or a template-less one, is rejected.
- **Ze meanwhile:** refuses a transport-mode peer with more than one
  `traffic-selector` entry; as responder it answers several proposed pairs
  with the first, as initiator it refuses an answer of several pairs.

<!-- source: internal/component/ike/engine/config.go -- validateTransportOnePair -->

### LL-XFRM-STATE-ONE-ESP-FORM

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** An XFRM state receives one ESP form: UDP-encapsulated or
  bare, as its encapsulation template says. A second state for the other form
  cannot be added, because the state is keyed by destination, SPI and
  protocol.
- **Blocks:** receiving both ESP forms on one Child SA through the kernel
  alone (RFC 7296 Section 2.23).
- **Evidence:** `net/xfrm/xfrm_input.c` `xfrm_input`:
  `if ((x->encap ? x->encap->encap_type : 0) != encap_type) {` then
  `XFRM_INC_STATS(net, LINUX_MIB_XFRMINSTATEMISMATCH);`.
  `net/xfrm/xfrm_state.c` `xfrm_state_add`:
  `x1 = __xfrm_state_locate(x, use_spi, family);` then `err = -EEXIST;`.
- **Re-check:** read the encapsulation check in `xfrm_input` and the lookup in
  `xfrm_state_add`. `TestEncapKernelBindsOneESPFormPerState` and the dual-form
  integration test measure it on a real kernel.
- **Ze meanwhile:** works around it. A raw `IPPROTO_ESP` reader receives the
  form the state refuses and re-presents it.

<!-- source: internal/component/ike/dataplane/espform_linux.go -- espFormReceiver -->

### LL-NETLINK-XFRM-PORT-MASK

- **Kind:** library
- **Effect:** constrains design
- **Status:** verified
- **Checked:** module github.com/vishvananda/netlink v1.3.1, 2026-10-20
- **Limitation:** The netlink library Ze programs XFRM policies through derives
  each port mask from the port value: a non-zero port gets mask `0xffff`, port 0
  gets mask 0. "Exactly port 0" (OPAQUE) cannot be built through it and would
  install as any port. The kernel is not the limit: it stores the selector
  masks it is given and matches with them.
- **Blocks:** OPAQUE port selectors. RFC4301-4.4.1.1-2, RFC 7296 Section
  3.13.1 (RFC7296-3.13.1-3).
- **Evidence:** library, `xfrm_policy_linux.go` `selFromPolicy`:
  `if sel.Dport != 0 {` `sel.DportMask = ^uint16(0)`, and the same for
  `Sport`. Kernel (Linux 7.2.9), `net/xfrm/xfrm_user.c`
  `copy_from_user_policy`: `memcpy(&xp->selector, &p->sel, sizeof(xp->selector));`;
  `net/xfrm/xfrm_policy.c` `__xfrm4_selector_match`:
  `!((xfrm_flowi_dport(fl, &fl4->uli) ^ sel->dport) & sel->dport_mask) &&`.
- **Re-check:** read `selFromPolicy` in the module version `go.mod` requires.
  Lifted when the library takes a mask from the caller, or when Ze builds the
  selector itself.
- **Ze meanwhile:** refuses OPAQUE at commit and at negotiation. The fix:
  `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md`.

<!-- source: internal/component/ike/ipsec/traffic_selector.go -- checkPortProgrammable -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmSelectorPort -->

### LL-RTNL-NEWLINK-NO-ALIAS

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** An `RTM_NEWLINK` that creates a device ignores
  `IFLA_IFALIAS`. Only a later change to an existing device sets the alias,
  so a device cannot be created already carrying Ze's ownership alias.
- **Blocks:** creating an ownership-marked device in one atomic message.
  No RFC requirement.
- **Evidence:** `net/core/rtnetlink.c` `rtnl_create_link` reads
  `IFLA_MTU`, `IFLA_ADDRESS`, `IFLA_TXQLEN` and other attributes but not
  `IFLA_IFALIAS`; only `do_setlink` applies it:
  `if (tb[IFLA_IFALIAS]) {` then `err = netif_set_alias(dev, nla_data(tb[IFLA_IFALIAS]),`.
- **Re-check:** read `rtnl_create_link` and `rtnl_newlink_create`. Lifted if
  either applies `IFLA_IFALIAS`.
- **Ze meanwhile:** works around it. iface sets the alias right after create
  and deletes the device if that fails, so no unmarked device is left.

<!-- source: internal/plugins/iface/netlink/macvlan_linux.go -- CreateMacvlanDevice -->

### LL-XFRM-ICMP-RETURN-SA

- **Kind:** kernel
- **Effect:** blocks
- **Status:** unverified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** Possibly none. The question is whether the kernel maps an
  outbound ICMP error that no SA or SPD entry carries to the SA that carries
  the return traffic of the packet that triggered it. The IPv4 source suggests
  it does; the IPv6 path is not read and no test observes either.
- **Blocks:** RFC4301-6.2-3, until settled.
- **Evidence:** `net/ipv4/icmp.c` `icmp_route_lookup`: when the forward
  `xfrm_lookup` returns the plain route, it runs
  `err = xfrm_decode_session_reverse(net, skb_in, flowi4_to_flowi(&fl4_dec), AF_INET);`
  and looks the route up again on the reversed flow of the triggering packet.
- **Re-check:** read the rest of `icmp_route_lookup` (the second
  `xfrm_lookup` on the reversed flow) and the IPv6 counterpart in
  `net/ipv6/icmp.c`, then run a namespace test that provokes an ICMP error
  toward a protected peer and reads which SA carried it. If the kernel does
  it, this entry is removed and the RFC row becomes a lower-layer row with
  that test.
- **Ze meanwhile:** conformance gap with no Ze producer:
  `plan/release/fixes/spec-rfc4301-architecture-gaps.md`.

<!-- source: rfc/short/rfc4301.md -- RFC4301-6.2-3 -->

### LL-MPLS-FORWARD-NO-PMTU

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** Upstream `mpls_forward` drops a labeled frame too big for
  the outgoing device without reading the inner IP header, so it neither
  fragments a DF-clear IPv4 datagram nor sends ICMP Fragmentation Needed or
  Packet Too Big. An `AF_MPLS` route also refuses `RTA_METRICS`, so a transit
  swap or pop cannot carry the LSP's path MTU.
- **Blocks:** enforcing an RSVP-TE LSP's path MTU at a transit router (RFC
  3209 Section 2.6) on a kernel without Ze's patch.
- **Evidence:** `net/mpls/af_mpls.c` `mpls_forward`:
  `if (mpls_pkt_too_big(skb, mtu - new_header_size))` then `goto tx_err;`,
  which counts `tx_errors` and ends at `kfree_skb(skb);`.
  `rtm_to_route_config` answers any attribute it does not list, `RTA_METRICS`
  among them, with `NL_SET_ERR_MSG_ATTR(extack, nla, "Unknown attribute");`
  and `goto errout;`.
- **Re-check:** read `mpls_forward` and the attribute switch in
  `rtm_to_route_config` at the new version; the entry is lifted if upstream
  fragments or answers with ICMP and accepts `RTA_METRICS`. Then check that
  `gokrazy/kernel/patches/0002-mpls-ip-mtu.patch` still applies at the new
  version, or is no longer needed.
- **Ze meanwhile:** works around it. The appliance kernel carries
  `gokrazy/kernel/patches/0002-mpls-ip-mtu.patch` (`CONFIG_MPLS_IP_MTU`). On a
  kernel without it, the forwarding owner probes once, installs transit routes
  without the MTU, logs a warning, and `ze doctor` reports
  `doctor-mpls-transit-mtu-unenforced`.

<!-- source: internal/plugins/fib/kernel/mplsentry_linux.go -- transitRouteMTU, askTransitMTU -->

### LL-IPV6-NO-RP-FILTER

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** IPv6 has no per-device reverse-path filter sysctl; IPv4's
  `rp_filter` has no IPv6 counterpart.
- **Blocks:** a sysctl-only IPv6 reverse-path check (the `rpf-check` leaf
  under a unit's `ipv6`). No RFC requirement.
- **Evidence:** `include/linux/inetdevice.h` defines
  `#define IN_DEV_RPFILTER(in_dev)		IN_DEV_MAXCONF((in_dev), RP_FILTER)` for
  IPv4. A case-insensitive search for `rp_filter` and `rpfilter` in
  `include/uapi/linux/ipv6.h` (the `DEVCONF_` list of IPv6 per-device
  settings) and `net/ipv6/addrconf.c` (the `addrconf_sysctl` table) finds no
  line.
- **Re-check:** repeat that search at the new version; lifted if IPv6 gains a
  per-device reverse-path setting.
- **Ze meanwhile:** works around it. iface enforces the IPv6 check with an
  nftables `fib` lookup in a prerouting chain of its own table,
  `ip6 ze_iface_rpf`.

<!-- source: internal/plugins/iface/netlink/rpf_linux.go -- ReplaceIPv6RPF -->

### LL-NETLINK-XFRM-DECODE

- **Kind:** library
- **Effect:** constrains design
- **Status:** verified
- **Checked:** module github.com/vishvananda/netlink v1.3.1, 2026-10-20
- **Limitation:** The netlink library's XFRM code mis-reads and mis-writes
  state and policy messages Ze relies on: the state reader ignores the
  extended sequence number replay attribute, and the policy and state readers
  lose the selector family and port masks. The kernel is not the limit.
- **Blocks:** reading back an SA that uses extended sequence numbers, a replay
  window above 32, transport-mode templates without a destination, and
  dual-family policy dumps, through the unpatched library.
- **Evidence:** library, `xfrm_state_linux.go`: the writer sends
  `out := nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, writeReplayEsn(state.ReplayWindow))`,
  while the reader's attribute switch handles only `case nl.XFRMA_REPLAY_VAL:`
  and has no `XFRMA_REPLAY_ESN_VAL` case. The full list of corrections is in
  `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, "Vendored netlink patch".
- **Re-check:** at a new module version, check each hunk of
  `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` against the
  module source; a hunk the release already carries is dropped, and the
  entry is lifted when no hunk remains. `TestNetlinkXFRMPatchApplied`
  (`internal/le/vendorpatch`) fails when the vendored copy lacks the patch,
  for example after `go mod vendor`.
- **Ze meanwhile:** works around it. Ze vendors the module and applies
  `netlink-xfrm-fixes.patch`.

<!-- source: internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch -- XFRM state and policy decoding corrections -->

### LL-NETLINK-NO-CTA-ID

- **Kind:** library
- **Effect:** constrains design
- **Status:** verified
- **Checked:** module github.com/vishvananda/netlink v1.3.1, 2026-10-20
- **Limitation:** The netlink library's conntrack reader skips `CTA_ID`, so a
  flow carries no kernel id that survives a tuple being reused.
- **Blocks:** keying per-flow deltas on the kernel's conntrack id in flow
  export. No RFC requirement.
- **Evidence:** library, `conntrack_linux.go` `parseRawData`:
  `case nl.CTA_ID, nl.CTA_STATUS, nl.CTA_USE:` then
  `skipNfAttrValue(reader, l)`.
- **Re-check:** read the conntrack attribute switch in the module version
  `go.mod` requires; lifted when `CTA_ID` is decoded into the flow.
- **Ze meanwhile:** works around it. Flow export keys each flow by its
  5-tuple.

<!-- source: internal/plugins/flowexport/conntrack/delta.go -- flowKey -->

## Considered and left out

| Candidate | Why it is not an entry |
|-----------|------------------------|
| IPv4 fragmentation on an MPLS push route looping on empty fragments | The kernel Ze ships no longer has it. `net/ipv4/ip_output.c` `ip_do_fragment` at Linux 7.2.9 refuses an MTU below one fragment: `if (mtu < hlen + 8) {` then `err = -EMSGSIZE;`. Older kernels without that check loop in `ip_frag_next`; `mplsfib.PathMTUMinimum` keeps Ze off that path on any host kernel |
| Conntrack runs after IP defragmentation, so it never sees a fragment | The defined order of netfilter, and no feature is blocked: the fragment-flood classifier is planned on packet sampling, which sees fragments (`docs/architecture/ddos/cp-survival-5-detect-5-characterization.md`) |
| Linux has no route tag attribute | No Ze feature needs the tag in the kernel: Ze carries it in its own RIB into redistribution and the OSPF External Route Tag |
