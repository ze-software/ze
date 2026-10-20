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

## How an entry is written

Each entry is a `###` heading that holds its id, followed by these fields:

| Field | Content |
|-------|---------|
| Kind | `kernel` or `library` (a Go module, never the kernel) |
| Status | `verified` (the source was read at the version in `Checked`) or `unverified` (what would settle it is stated) |
| Checked | `Linux <version>, <date>` or `module <path> <version>, <date>` on one line |
| Limitation | The behavior, in plain words |
| Blocks | The feature, and the RFC requirement ids where one exists |
| Evidence | Each file and function read, with the line quoted from that source |
| Re-check | What to read, and what result lifts or keeps the limitation |
| Ze meanwhile | What Ze does now: refuse, work around, or carry a gap |

`TestLinuxLimitsCheckedAtShippedVersions` (`internal/appliance/linux_limits_test.go`)
fails when a `kernel` entry's `Checked` version differs from
`internal/appliance/kernel.version`, or a `library` entry's version differs
from the requirement in `go.mod`. A kernel bump therefore owes a pass over
this page: re-read each entry at the new version, update `Checked`, and lift
any entry the new source no longer supports.

<!-- source: internal/appliance/linux_limits_test.go -- TestLinuxLimitsCheckedAtShippedVersions -->

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
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** A proxy ARP reply always carries the hardware address of
  the device that received the request. No sysctl or proxy mode
  (`proxy_arp`, `proxy_arp_pvlan`, a `pneigh` entry, `medium_id`) names
  another MAC. A broadcast request that arrives on a macvlan's parent is also
  passed up the parent, so a parent with proxy ARP on answers with the
  physical MAC.
- **Blocks:** proxy ARP on a VRRP router. RFC9568-8.1.3-1, RFC5798-8.1.3-1,
  RFC3768-8.3-1 ("the VRRP Router MUST advertise the Virtual Router MAC
  address in the Proxy ARP message").
- **Evidence:** `net/ipv4/arp.c` `arp_process`, the forwarding branch taken
  when `arp_fwd_proxy(in_dev, dev, rt) || arp_fwd_pvlan(in_dev, dev, rt, sip, tip)`
  holds, replies with `arp_send_dst(ARPOP_REPLY, ETH_P_ARP, sip, dev, tip, sha, dev->dev_addr, sha, reply_dst);`.
  `drivers/net/macvlan.c` `macvlan_handle_frame` queues a broadcast to the
  macvlans with `macvlan_broadcast_enqueue(port, src, skb);` and then
  `return RX_HANDLER_PASS;`, so the parent processes the frame too.
- **Re-check:** read the three `arp_send_dst(ARPOP_REPLY` calls in
  `arp_process`. The limitation is lifted only if one takes a hardware address
  other than `dev->dev_addr`. Not yet tested: proxy ARP on the virtual-MAC
  macvlan alone, with the parent off, would answer with the virtual MAC by
  the same code; a QEMU test that sends a proxied request and reads the reply
  MAC on a Backup and an Active router settles whether that is a usable design.
- **Ze meanwhile:** refuses proxy ARP on every unit that carries an IPv4 VRRP
  group, at config verify and on a runtime `set sysctl`.

<!-- source: internal/plugins/vrrp/proxyarp.go -- validateNoProxyARP, refuseLiveProxyARP -->

### LL-NS-SOURCE-MAC

- **Kind:** kernel
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
  `plan/immediate/spec-vrrp-owner-arp-rfc-defects.md`.

<!-- source: internal/plugins/vrrp/ownerfilter.go -- owner filter drops parent Neighbor Advertisements only -->

### LL-IPV6-LINKLOCAL-FROM-MAC

- **Kind:** kernel
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** On link-up the kernel adds a link-local address built from
  the device MAC (Modified EUI-64) unless the device's `addr_gen_mode` is
  `none` (or a stable or random mode) before that moment. A macvlan carrying
  the Virtual Router MAC would get a link-local derived from it.
- **Blocks:** RFC9568-7.4-1 ("The Virtual Router MAC MUST NOT be used for the
  Net_Iface parameter used in the Interface Identifier (IID) derivation
  algorithms").
- **Evidence:** `net/ipv6/addrconf.c` `addrconf_addr_gen`:
  `case IN6_ADDR_GEN_MODE_EUI64:` then
  `if (ipv6_generate_eui64(addr.s6_addr + 8, idev->dev) == 0)`, and
  `case IN6_ADDR_GEN_MODE_NONE:` with `/* will not add any link local address */`.
  `addrconf_ifid_eui48` reads the MAC: `memcpy(eui, dev->dev_addr, 3);`.
- **Re-check:** read `addrconf_addr_gen`. The workaround stays valid while
  `IN6_ADDR_GEN_MODE_NONE` adds no address.
- **Ze meanwhile:** works around it. iface sets `addr_gen_mode` none on every
  owned macvlan before link-up.

<!-- source: internal/plugins/iface/netlink/macvlan_linux.go -- setMacvlanAddrGenNone -->

### LL-IPV4-CONF-ALL-WINS

- **Kind:** kernel
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
- **Status:** verified
- **Checked:** Linux 7.2.9, 2026-10-20
- **Limitation:** An XFRM state holds one selector, and inbound processing
  drops a packet that does not match the selector of the state that
  decrypted it. A transport-mode Child SA therefore carries one selector pair:
  a selector wide enough for several pairs admits traffic no pair negotiated.
- **Blocks:** a transport-mode Child SA with several traffic selector pairs
  (RFC 7296 Section 2.9 lets a peer propose several).
- **Evidence:** `include/net/xfrm.h` `struct xfrm_state` has one
  `struct xfrm_selector	sel;`. `net/xfrm/xfrm_policy.c` `__xfrm_policy_check`:
  `if (!xfrm_selector_match(&x->sel, &fl, family)) {` then
  `XFRM_INC_STATS(net, LINUX_MIB_XFRMINSTATEMISMATCH);`.
- **Re-check:** read `struct xfrm_state` and the state loop in
  `__xfrm_policy_check`. Lifted only if a state gains a selector list.
- **Ze meanwhile:** refuses a transport-mode peer with more than one
  `traffic-selector` entry; as responder it answers several proposed pairs
  with the first, as initiator it refuses an answer of several pairs.

<!-- source: internal/component/ike/engine/config.go -- validateTransportOnePair -->

### LL-XFRM-STATE-ONE-ESP-FORM

- **Kind:** kernel
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
  `plan/spec-ipsec-opaque-selector-port-mask.md`.

<!-- source: internal/component/ike/ipsec/traffic_selector.go -- checkPortProgrammable -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmSelectorPort -->

### LL-RTNL-NEWLINK-NO-ALIAS

- **Kind:** kernel
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
  `plan/immediate/spec-rfc4301-architecture-gaps.md`.

<!-- source: rfc/short/rfc4301.md -- RFC4301-6.2-3 -->
