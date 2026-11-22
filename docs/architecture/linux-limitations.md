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

### LL-PPP-ORIGINAL-RESOURCE

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-11-18
- **Limitation:** Stock PPP control selects devices, units and peer channels by
  reusable names or numbers. It has no creator-file condition on rtnetlink
  writes, no original-unit-file CONNECT condition and no atomic PPPoX-to-file
  channel capture. Keeping a PPPoX socket open does not keep its registered
  channel live through device-down. Stock IPv4 address deletion matches the
  peer's subnet and can cascade from a primary to other secondaries; stock
  IPv6 deletion does not select by the supplied peer.
  <!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- inet_rtm_deladdr, __inet_del_ifa, inet6_addr_del -->

- **Blocks:** Safe shared PPP session setup, mutation and cleanup when a device
  or transport is renamed, moved, unregistered or replaced while its original
  file remains open. This is a carried-ABI design constraint, not an RFC
  impossibility claim.
- **Evidence:** `drivers/net/ppp/ppp_generic.c` `ppp_unattached_ioctl` selects
  the unit with `ppp = ppp_find_unit(pn, unit);` and the channel with
  `chan = ppp_find_channel(pn, unit);`; its CONNECT path also selects the
  requested numeric unit. `drivers/net/ppp/pppox.c` `pppox_ioctl` returns
  `index = ppp_channel_index(&po->chan);`, then performs the userspace handoff.
  `drivers/net/ppp/pppoe.c` `pppoe_flush_dev` holds `lock_sock(sk);` and
  calls `pppox_unbind_sock(sk);`; `pppox_unbind_sock` calls
  `ppp_unregister_channel(&pppox_sk(sk)->chan);` before setting the socket
  dead. `net/core/rtnetlink.c` `do_setlink` selects the request device and
  applies its fields without a PPP creator-file predicate. These statements
  were read at the upstream `v7.3` tag, before Ze's original-resource patches.
  `net/ipv4/devinet.c` `inet_rtm_deladdr` uses
  `inet_ifa_match(nla_get_in_addr(tb[IFA_ADDRESS]), ifa)`, a subnet test;
  `__inet_del_ifa` takes `IN_DEV_PROMOTE_SECONDARIES(in_dev)` and deletes
  matching secondary entries when promotion is off. `net/ipv6/addrconf.c`
  `inet6_rtm_deladdr` extracts peer_pfx but passes only pfx/prefix to
  `inet6_addr_del`, whose selector compares `ipv6_addr_equal(pfx, &ifp->addr)`
  and the prefix without checking the peer.
  <!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- inet_rtm_deladdr, __inet_del_ifa, inet6_addr_del -->

- **Alternatives:** A name/index readback or PPPIOCGUNIT readback checks a label
  before a later writer. A separately opened file attached to the same unit is
  not its creator file. Retaining only the transport socket cannot prevent the
  unregister path above. None supplies the missing native atomic condition.
- **Re-check:** Read the PPP ioctl UAPI, those selection and unregister
  producers, rtnetlink family dispatch, address writers and IPv4/IPv6 FIB
  writers. Lift this entry only when upstream supplies equivalent original
  file/device validation at every actual affected writer and atomic transport
  capture, plus exact address-tuple deletion that preserves other pairs;
  a new label query alone does not lift it.
  <!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- PPP_OWNER_EXACT_ADDR_PAIR, original-resource ABI -->

- **Ze meanwhile:** The carried original-resource ABI requires positive
  versioned creator/channel/transport probes and per-operation capabilities.
  Its read-only global PPPIOCGSUPPORT declaration on an unbound /dev/ppp file
  reports compiled support without creating a resource; it never replaces
  those exact live-owner probes or proves native acceptance. Guarded pair
  deletion additionally requires bit10 plus its family bit and the complete
  retained LOCAL/ADDRESS/prefix tuple; IPv4 uses internal native promotion
  without a configuration write, and IPv6 validates the actual peer.
  Unsupported kernels or operations are refused without numeric or unguarded
  fallback. The ABI and integration requirements are documented in
  [kernel profiles](appliance/kernel-profiles.md#ppp-original-resource-abi).
  Source admission and a matching native kernel proof remain distinct from
  this verified stock-source limitation.
  <!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- ppp_support_ioctl, PPP_OWNER_EXACT_ADDR_PAIR -->

### LL-XFRM-OUTER-DF-SET-DSCP

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-11-12
- **Limitation:** An XFRM tunnel-mode state writes the outer IPv4 DF bit as a
  copy of the inner bit, or clear with `XFRM_STATE_NOPMTUDISC`; no flag or
  attribute writes it set. It writes the outer DSCP as a copy of the inner one,
  or 0 with `XFRM_SA_XFLAG_DONT_ENCAP_DSCP`; no attribute writes another value.
  (`XFRMA_IPTFS_DONT_FRAG` applies to IP-TFS mode only.)
- **Blocks:** `df-bit set` (RFC4301-8.1-2, "set, clear, copy from inner header")
  and `dscp-map` 1 to 63 (RFC4301-5.1.2.1-1) on the Linux dataplane. Ze refuses
  `df-bit set` and a DSCP 1 to 63 at commit, and again when a Child SA
  installs from a configuration no commit judged, naming the fix. That
  refusal is Ze's choice, not a kernel limit: Linux can write both through
  the alternative below. The owner kept the refusal for the first release and
  placed a Ze kernel patch after it
  (`plan/post-release/kernel/spec-xfrm-outer-df-set-dscp.md`), so both
  requirements stay Ze-owed gaps on Linux until then.
- **Evidence:** `net/xfrm/xfrm_output.c` `xfrm4_tunnel_encap_add`:
  `top_iph->frag_off = (flags & XFRM_STATE_NOPMTUDISC) || small_ipv6 ? 0 : (XFRM_MODE_SKB_CB(skb)->frag_off & htons(IP_DF));`
  and `if (x->props.extra_flags & XFRM_SA_XFLAG_DONT_ENCAP_DSCP) top_iph->tos = 0; else top_iph->tos = XFRM_MODE_SKB_CB(skb)->tos;`.
- **Alternatives:** an nftables rule in the `output` hook rewriting the outer
  ESP packet of one SA (its tunnel addresses and SPI; under NAT-T a raw
  payload match, since ESP sits inside UDP 4500). The encapsulated packet
  always reaches LOCAL_OUT (`xfrm_output_resume`:
  `err = skb_dst(skb)->ops->local_out(net, sk, skb);`), while POST_ROUTING in
  `ip_output` is conditional (`NF_HOOK_COND(... !(IPCB(skb)->flags & IPSKB_REROUTED))`).
  `nft_payload_set_eval` (`net/netfilter/nft_payload.c`) rewrites an IPv4
  header field with its checksum, so `ip dscp set`, `ip6 dscp set` and a
  frag-off write reach the outer header. The IPv4 path sets no `ignore_df`,
  so a DF-set outer packet over the MTU is refused with frag-needed
  (`ip_output.c` `ip_fragment`). Not chosen: the owner chose the kernel patch.
- **Re-check:** read those two statements in `xfrm4_tunnel_encap_add` and the
  `XFRM_STATE_*` and `XFRM_SA_XFLAG_*` lists in `include/uapi/linux/xfrm.h`; the
  entry stays while neither gains a set-DF or a DSCP-value option.
- **Ze meanwhile:** refuses `df-bit set` and `dscp-map` 1 to 63 on Linux at
  commit (`verifyPeerSAOptions`) and at install (`xfrmDFFlag`,
  `xfrmDSCPFlag`).

### LL-PROXY-ARP-MAC

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-10-30
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
- **Checked:** Linux 7.3, 2026-10-30
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
- **Checked:** Linux 7.3, 2026-10-30
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
- **Checked:** Linux 7.3, 2026-10-30
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
- **Checked:** Linux 7.3, 2026-10-30
- **Limitation:** A stock XFRM state holds one selector, and inbound processing
  drops a packet that does not match the selector of the state that
  decrypted it. A transport-mode Child SA therefore carries one selector pair:
  a selector wide enough for several pairs admits traffic no pair negotiated.
- **Blocks:** a transport-mode Child SA with several traffic selector pairs
  through the stock ABI (RFC 7296 Section 2.9 lets a peer propose several).
- **Evidence:** stock `include/net/xfrm.h` `struct xfrm_state` has one
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
- **Re-check:** read `struct xfrm_state` and `__xfrm_policy_check`, including
  Ze's carried state-scope patch. The stock limitation does not describe a
  state installed through that extension's complete directional selector set.
  The carried ABI still requires one transport-mode record identical to the
  legacy selector.
- **Ze meanwhile:** carries `XFRMA_STATE_SCOPE` for complete directional
  tunnel-mode selector sets and checks a fresh full capability receipt before
  installing a scoped state. This does not establish full packet proof; the
  remaining validation is described in the
  [IKE architecture page](ike/ipsec-8-ikev2-child-xfrm.md). Ze still
  refuses a transport-mode peer with more than one
  `traffic-selector` entry; as responder it answers several proposed pairs
  with the first, as initiator it refuses an answer of several pairs.

<!-- source: internal/component/ike/engine/config.go -- validateTransportOnePair -->
<!-- source: internal/component/ike/engine/child_scope.go -- childStateScope -->
<!-- source: internal/component/ike/dataplane/xfrm_state_scope_linux.go -- checkXFRMStateScopeCapabilities -->
<!-- source: gokrazy/kernel/patches/0003-xfrm-state-scope.patch -- xfrm_state_scope_from_user, xfrm_state_scope_match -->

### LL-XFRM-STATE-ONE-ESP-FORM

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-10-30
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
- **Ze meanwhile:** installs the inbound state with the ESP-in-UDP template
  and turns bare ESP for its SPI into that form before XFRM reads it, on two
  paths. Both hand XFRM the state's own form for an IPv4 outer header, on a
  device whose `disable_policy`, the setting that decides the IPv4 check, is 0
  (the real link unless the operator set it, and `ze-espform` always), so the
  inbound policy check of RFC 4301 Section 5.2 runs on the decrypted packet.
  The fast path is a tc ingress program,
  attached to every non-loopback link while an SPI is watched: it inserts the
  UDP header with the template's ports into a whole bare datagram, so the
  packet is received on its real link. The complete path is a raw
  `IPPROTO_ESP` reader for what the fast path leaves alone (fragments,
  read after reassembly, IPv4 options, a link without the program). It writes
  the encapsulated datagram into the Ze TUN `ze-espform`, at most 5000
  datagrams per second. Ze never re-presents a datagram through `lo`, whose
  `disable_policy` is 1 and skips the check. Serving both forms over an IPv6
  outer header is a gap owed after the first release: Ze has no IPv6 IKE
  transport yet, so no Child SA receives both forms over IPv6, and
  `plan/post-release/features/spec-ike-ipv6-transport.md` owns the work.

<!-- source: internal/component/ike/dataplane/espform_linux.go -- espFormReceiver -->
<!-- source: internal/component/ike/dataplane/espform_tun_linux.go -- espFormTUNSysctls -->
<!-- source: internal/component/ike/dataplane/espform_program_linux.go -- espFormFastPathInstructions -->
<!-- source: internal/component/ike/dataplane/espform_fastpath_linux.go -- the fast path's attach lifecycle -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- InstallSA, the IPv6 dual-form refusal -->

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
- **Blocks:** nothing in a Ze build: `netlink-xfrm-06-selector-port-masks.patch` carries the
  fix, which is why the entry constrains design. Without the patch, a selector
  for exactly port 0 (an ICMP type or code of 0, an OPAQUE port) installs as
  any port. Ze's OPAQUE support is not held back by this library any more; it
  is a gap in Ze's own code (see Ze meanwhile).
- **Evidence:** library, `xfrm_policy_linux.go` `selFromPolicy`:
  `if sel.Dport != 0 {` `sel.DportMask = ^uint16(0)`, and the same for
  `Sport`. Kernel (Linux 7.3), `net/xfrm/xfrm_user.c`
  `copy_from_user_policy`: `memcpy(&xp->selector, &p->sel, sizeof(xp->selector));`;
  `net/xfrm/xfrm_policy.c` `__xfrm4_selector_match`:
  `!((xfrm_flowi_dport(fl, &fl4->uli) ^ sel->dport) & sel->dport_mask) &&`.
- **Re-check:** read `selFromPolicy` in the module version `go.mod` requires.
  Lifted when the library takes a mask from the caller, or when Ze builds the
  selector itself.
- **Ze meanwhile:** works around it. `netlink-xfrm-06-selector-port-masks.patch` makes
  `selFromPolicy` send a non-zero `SrcPortMask` or `DstPortMask` as given, and
  Ze writes every selector's mask (`xfrmSelectorPort`, xfrm_linux.go), so an
  exact ICMP type or code of 0 and an ICMP code block install exactly. OPAQUE
  is still refused at commit and at negotiation: its meaning is a port that is
  not visible in the packet, and mapping it is the work of
  `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md`, which
  this patch unblocks. RFC4301-4.4.1.1-2 and RFC7296-3.13.1-3 are owed by
  that spec as a mapping missing from Ze's code, not as a library limit. A
  selector the patch makes expressible is not always one Ze installs: exactly
  port 0, and ICMP type 0 code 0, are what the kernel reads from every
  fragment, so Ze refuses them where a fragment would meet them
  (LL-XFRM-FRAGMENT-SELECTOR).

<!-- source: internal/component/ike/ipsec/traffic_selector.go -- checkPortProgrammable -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmSelectorPort -->

### LL-XFRM-FRAGMENT-SELECTOR

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-11-10
- **Limitation:** Stock XFRM classifies an IP fragment, initial or not, with port 0,
  ICMP type 0 and ICMP code 0: the flow dissector reads no transport header
  from a fragment, and XFRM does not ask it to parse the first one. On the
  forward path the policy lookups run before any reassembly of XFRM's own
  (only conntrack's defragmenter, when loaded, reassembles earlier), and XFRM
  keeps no fragment state. A selector of exactly port 0, or of ICMP type 0 with a code
  range holding 0, therefore matches every forwarded fragment between its
  prefixes, whatever packet the fragment belongs to.
- **Blocks:** installing a Child SA selector of exactly port 0 or ICMP type 0
  code 0 (RFC4301-7.3-3, RFC4301-7.3-4: such an SA would send and accept
  fragments without NON_FIRST_FRAGMENTS_ALSO), and an outbound operator SPD
  entry for ICMP type 0 with a code range holding 0 (the RFC4301-7.4-2
  concern for a bypass). Both requirements stay met, because Ze refuses the
  selector rather than install it.
- **Evidence:** stock `net/core/flow_dissector.c` `__skb_flow_dissect`, IPv4:
  `if (iph->frag_off & htons(IP_OFFSET)) {` `fdret = FLOW_DISSECT_RET_OUT_GOOD;`,
  and for the first fragment `if (!(flags & FLOW_DISSECTOR_F_PARSE_1ST_FRAG)) {`
  `fdret = FLOW_DISSECT_RET_OUT_GOOD;`; after the protocol step
  `if (!(key_control->flags & FLOW_DIS_IS_FRAGMENT))` guards
  `__skb_flow_dissect_ports`. `net/xfrm/xfrm_policy.c`
  `__xfrm_decode_session`: `memset(&flkeys, 0, sizeof(flkeys));` then
  `NULL, 0, 0, 0, FLOW_DISSECTOR_F_STOP_AT_ENCAP);` (no first-fragment parse);
  `decode_session4`: `fl4->fl4_sport = flkeys->ports.src;` and
  `fl4->fl4_icmp_type = flkeys->icmp.type;`. `net/ipv4/ip_forward.c`
  `ip_forward`: `if (!xfrm4_policy_check(NULL, XFRM_POLICY_FWD, skb)) {` and
  `if (!xfrm4_route_forward(skb)) {`, with no defragmentation before them;
  `net/ipv6/ip6_output.c` `ip6_forward`:
  `!xfrm6_policy_check(NULL, XFRM_POLICY_FWD, skb)) {` and
  `if (!xfrm6_route_forward(skb)) {`.
- **Re-check:** read `__xfrm_decode_session` and the fragment cases of
  `__skb_flow_dissect`, together with Ze's carried fragment-context and guard
  patches. While the live guard has references, it gathers fragments before
  port-sensitive policy lookup and repeats deferred raw-output routing using
  the completed packet and captured socket authority. Scope identities also
  separate protected reassembly contexts. The stock limitation alone does
  not describe that guarded path; complete route and context validation
  remains owed as described in the
  [IKE architecture page](ike/ipsec-8-ikev2-child-xfrm.md).
- **Ze meanwhile:** retains its selector refusals. Negotiation narrows a
  proposed single port 0 or
  ICMP type 0 code 0 away and refuses such an answer (`programmableSelector`,
  `engine/ts_narrow.go`), the policy builder installs no pair carrying one
  (`childPolicyPair`, `engine/child.go`), and commit refuses an `out` or `both`
  operator entry for ICMP type 0 with a code range holding 0
  (`validateICMPFragments`, `ipsec/spd_policy.go`). Traffic for Ze itself is
  reassembled before its inbound check (`ip_local_deliver`), so an `in` entry
  is accepted. Any port, which RFC 4301 Section 7.1 lets carry every fragment,
  is unaffected. A port-scoped operator bypass depends on the kernel. On a
  kernel whose K1 receipt carries capability bit 3, K3 gathers forwarded
  fragments at PRE_ROUTING, before the FWD check in `ip_forward`, while a
  port-scoped policy exists. Ze then projects the bypass onto the forward path
  with its port (`spdForwardGathers` and `spdTransitPolicies`,
  `engine/spd_policy.go`). On a kernel without bit 3, Ze stores no
  port-scoped bypass in FWD. A commit refuses such an entry unless its local
  prefix is one address the host holds (`verifyForwardBypass`,
  `engine/spd_forward_bypass.go`). A start installs it for Ze's own traffic,
  logs it at Error and reports it through the `ze doctor` check
  `ipsec-spd-forward-bypass`. The
  [IKE architecture page](ike/ipsec-8-ikev2-child-xfrm.md), "Transit
  traffic", has the projection.

<!-- source: internal/component/ike/engine/ts_narrow.go -- programmableSelector, portMatchesEveryFragment -->
<!-- source: internal/component/ike/engine/child.go -- childPolicyPair -->
<!-- source: internal/component/ike/ipsec/spd_policy.go -- validateICMPFragments -->
<!-- source: internal/component/ike/engine/spd_policy.go -- spdForwardGathers, spdTransitPolicies -->
<!-- source: internal/component/ike/engine/spd_forward_bypass.go -- verifyForwardBypass -->
<!-- source: gokrazy/kernel/patches/0004-ip-fragment-xfrm-context.patch -- inet_frag_xfrm_context_init -->
<!-- source: gokrazy/kernel/patches/0005-xfrm-fragment-guard.patch -- xfrm_fragment_guard_active, xfrm_fragment_local_out, xfrm_fragment_route -->

### LL-RTNL-NEWLINK-NO-ALIAS

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-10-30
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
- **Ze meanwhile:** native VLAN preparation consumes the original CREATE
  echo and ACK, then sets a separate ownership alias under the same creator
  fence. Caller expiry never permits a later alias stamp. An original outcome
  confirmed before that fence is released can roll back the unpublished device;
  a lost or unusable outcome retains an unmarked quarantine. A later device with
  matching selectors supplies no ownership authority. Other interface creators
  also set the alias separately after creation.

<!-- source: internal/plugins/iface/netlink/macvlan_linux.go -- CreateMacvlanDevice -->

### LL-XFRM-ICMP-TRANSIT

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-11-20
- **Limitation:** A stock kernel performs the RFC 4301 Section 6.2 processing
  for an ICMP error addressed to the node or generated by it, but for a
  FORWARDED ICMP error only in two narrow cases. Inbound, an error that
  arrived on an SA is judged by the policy its quoted payload selects only
  when no FWD policy matches the error's own headers, so a FWD catch-all, an
  operator bypass's FWD transit gate or its FWD decision decides it by its
  outer headers instead. Outbound, the error is mapped to the SA of the
  return traffic it quotes only after the OUT lookup passed it
  untransformed: under `unmatched discard` the catch-all answers BLOCK first,
  and so does an operator `out` or `both` discard entry that covers it, and
  the error is dropped. Under `unmatched bypass` the lookup passes it and the
  mapping runs.
  On the out direction the stock mapping also reaches errors Section 6.2
  leaves alone, for forwarded and locally generated errors alike: the kernel
  cannot tell a BYPASS entry from no entry, because a template-free allow
  policy and an absent one both answer the plain route. An error that an
  operator `vpn ipsec policy` bypass entry outranking a peer's PROTECT
  policy, or the catch-all under `unmatched bypass`, would carry in clear is
  sent into the return SA, where the RFC maps an error only "if no SPD entry
  would allow carriage of this outbound ICMP error message".
  On the locally addressed path the same reversed check overrides an inbound
  discard entry. An ICMP message that arrives on a Child SA and that an
  operator `vpn ipsec policy` `in` discard entry refuses is checked again on
  the header it carries after its ICMP header, with addresses and ports
  reversed, when the last state of its secpath carries `XFRM_STATE_ICMP` (the
  peer's `icmp-error-check true`, the default). It is delivered when that
  header matches the SA, whatever the discard entry says, and the kernel
  never checks that the message is an error type, so a non-error message (an
  echo request) whose bytes after its ICMP header are forged to match the SA
  is delivered too. ICMP arriving in the clear (the unprotected side of
  Section 6.1.1, or the protected side of Section 6.1.2) has no secpath, so a
  discard entry always decides it.
  Stock `icmp_err_packet` also answers only Destination Unreachable and Time
  Exceeded for IPv4 (and Packet Too Big besides for IPv6), so neither
  forwarded lookup reads the quote of any other error type, an IPv4
  Parameter Problem among them.
  Ze's runtime kernel carries K1
  (`gokrazy/kernel/patches/0003-xfrm-state-scope.patch`), which advertises
  each lift as a capability bit in its `XFRM_MSG_GETDEFAULT` receipt:
  bit 5 (`XFRM_ZE_CAP_ICMP_REVERSE`) runs the locally addressed reversed
  check for ICMP error types alone, and stops it at an inbound BLOCK policy
  the message matches as received unless that policy carries
  `XFRM_POLICY_ICMP`; bit 6 (`XFRM_ZE_CAP_ICMP_FORWARD_MAP`) has
  `__xfrm_route_forward` try the return SA when its OUT lookup answers
  `-EPERM`, keeping the answer only when it is an XFRM bundle, so an error no
  SA carries stays dropped; bit 7 (`XFRM_ZE_CAP_ICMP_FORWARD_GATE`) has
  `__xfrm_policy_check` judge a forwarded error that arrived on an SA by its
  payload's policy when its own headers meet a template-free FWD allow
  carrying `XFRM_POLICY_ICMP`. K1 also widens `icmp_err_packet` to every type
  `icmp_is_err` names for IPv4 and every ICMPv6 error type. K1 leaves the
  allow branch of `__xfrm_route_forward` and the reversed path of
  `icmp_route_lookup` as they are, so the outbound override of a bypass entry
  holds on Ze's kernel too.
- **Blocks:** shapes the design of RFC4301-6.2-4 and RFC4301-6.2-3 for
  transit ICMP errors (the subject of Section 6.2), and with them
  RFC4301-6.2-2; none of them is blocked, because each case has a design on
  the stock kernel (Alternatives). Under `unmatched bypass` the kernel maps
  the error to the return SA, which RFC4301-6.2-3 asks for only when no SPD
  entry would carry it, and the bypass catch-all is such an entry; the same
  holds for any error a bypass entry covers, on stock Linux and on Ze's
  kernel. On stock Linux it also blocks an inbound discard entry refusing
  ICMP that arrives on a Child SA whose peer has `icmp-error-check true`
  (Section 6.1.2 asks for that control), and the Section 5.2 selector check
  (step 5) for a non-error message forged to look consistent; Ze's kernel
  performs both (K1 bit 5, above).
- **Evidence:** stock `net/xfrm/xfrm_policy.c` `__xfrm_policy_check` calls
  `if (!pol && dir == XFRM_POLICY_FWD) pol = xfrm_in_fwd_icmp(skb, &fl, family, if_id);`,
  so the reversed-payload lookup runs only when the outer lookup found no
  policy. `__xfrm_route_forward` calls
  `if (dst && !dst->xfrm) dst = xfrm_out_fwd_icmp(skb, &fl, family, dst);`
  only after `xfrm_lookup` passed the error untransformed; a block policy
  answers `-EPERM` first, and stock then sets `res = 0;`. `icmp_err_packet`
  tests `(fl4->fl4_icmp_type == ICMP_DEST_UNREACH ||` and
  `fl4->fl4_icmp_type == ICMP_TIME_EXCEEDED))`. The locally addressed and
  locally generated paths have no such condition: `icmp_rcv`
  (`net/ipv4/icmp.c`) re-checks with `xfrm4_policy_check_reverse` whenever
  the last state of the secpath carries `XFRM_STATE_ICMP` (`icmpv6_rcv`
  likewise), and `icmp_route_lookup` / `icmpv6_route_lookup` look the
  reversed flow up with `XFRM_LOOKUP_ICMP`, which `xfrm_lookup_with_ifid`
  honours only for a policy carrying `XFRM_POLICY_ICMP`. Ze sets both flags
  (`icmp-error-check`, default true). `icmp_route_lookup` takes that reversed
  path whenever
  `dst = xfrm_lookup(net, &rt->dst, flowi4_to_flowi(fl4), NULL, 0);` returns
  the route it was given (`if (rt != rt2) return rt;` is the only exit for a
  transformed answer), and `icmpv6_route_lookup` likewise
  (`if (dst != dst2) return dst;`); `xfrm_lookup_with_ifid` returns that same
  route for a policy whose action is allow and which holds no template, the
  shape of every bypass entry, as it does when no policy matches.
  K1, `__xfrm_route_forward`: `if (PTR_ERR(dst) == -EPERM) {`
  `dst = xfrm_out_fwd_icmp_block(skb, &fl, family, route);`, where
  `xfrm_out_fwd_icmp_block` keeps `if (dst->xfrm) return dst;` and otherwise
  releases the route and answers NULL. K1, `__xfrm_policy_check`:
  `if (pol && dir == XFRM_POLICY_FWD && sp && !pol->xfrm_nr &&`
  `pol->action == XFRM_POLICY_ALLOW && (pol->flags & XFRM_POLICY_ICMP)) {`
  then `struct xfrm_policy *payload = xfrm_in_fwd_icmp(skb, &fl, family, if_id);`.
  K1, `icmp_err_packet`: `icmp_is_err(fl4->fl4_icmp_type))` and
  `fl6->fl6_icmp_type < ICMPV6_INFOMSG_MASK)`. K1, `xfrm_user.c`: the receipt
  carries `XFRM_ZE_CAP_ICMP_REVERSE | XFRM_ZE_CAP_ICMP_FORWARD_MAP | XFRM_ZE_CAP_ICMP_FORWARD_GATE`.
- **Re-check:** read stock `__xfrm_policy_check`, `__xfrm_route_forward`
  and `icmp_err_packet`. Lifted on a stock kernel if either of the first two
  consults the reversed payload when the outer lookup finds a block or
  template-free policy, and if `icmp_route_lookup`, `icmpv6_route_lookup`
  and `__xfrm_route_forward` tell an allow policy from no policy before they
  take the reversed path. On a kernel bump, also check that K1's
  `__xfrm_route_forward`, `__xfrm_policy_check` and `icmp_err_packet` hunks
  still apply against the new context.
- **Alternatives:** each case has a design on the stock kernel, so none is a
  kernel limit (independent judgement, 2026-11-12, on
  spec-rfc4301-architecture-gaps).
  Forwarded inbound: the node-wide fwd catch-all expressed as the netns XFRM
  default policy (`XFRM_MSG_SETDEFAULT`) lets `xfrm_in_fwd_icmp` run, since
  `policy_default[dir]` is read only after it; the fwd interface catch-all
  (`interfaceSPDCatchAll`) can be removed, because a secpath packet no
  policy matches is still rejected
  (`if (sp && secpath_has_nontransport(sp, 0, &xerr_idx) && !is_crypto_offload)`).
  Forwarded outbound under a discard: on the stock kernel, allow entries for
  the ICMP error types ranked above the out catch-all let `xfrm_out_fwd_icmp`
  run, and an nftables rule (`rt ipsec missing`, `nft_rt`) drops what stayed
  untransformed; that design cannot outrank an operator discard entry. Ze
  carried it (`516671704b`) until the owner chose the kernel route on
  2026-11-20: K1's bit 6, above, so Ze no longer installs the entries or the
  table. A bypass entry the kernel overrides: a mark set on the errors it
  covers, with PROTECT policies masked to unmarked traffic, makes the
  reversed lookup miss (`fl1->flowi_mark = fl->flowi_mark;`). The inbound
  discard override and the non-error case: K1's bit 5. The forwarded case
  past an operator bypass's FWD transit gate or FWD decision: K1's bit 7.
- **Ze meanwhile:** requires K1's bits 5, 6 and 7 as it requires bits 0 to
  3: `checkXFRMStateScopeCapabilities` refuses every scoped Child SA on a
  kernel whose receipt lacks one, naming the rebuild. The XFRM backend
  expresses the node-wide FWD catch-all as the namespace's FWD default
  (`ApplyForwardDefault` and `ReleaseForwardDefault`), snapshotting and
  restoring only that default; Child FWD PROTECT entries remain and the
  per-SA selector check still runs first. The namespace tests in
  `engine/rfc4301_forward_icmp_integration_linux_test.go` exercise
  consistent and inconsistent quotes, checking enabled and disabled, node
  and interface SPDs, with actual protected-host egress controls. The
  outbound mapping of a forwarded error under the catch-all discard or an
  operator out discard is K1's bit 6. On Ze's kernel (docker-53c57c37)
  `TestRFC4301ForwardedICMPErrorsMapUnderDiscard` and
  `TestRFC4301ForwardedICMPParameterProblemMapsUnderDiscard` prove it for
  the catch-all discard, a Destination Unreachable and an IPv4 Parameter
  Problem, and `TestRFC4301ForwardedICMPErrorUnderOperatorDiscardMapsToReturnSA`
  exercises the operator discard
  (`engine/rfc4301_icmp_return_integration_linux_test.go`). For bit 7 the
  engine sets `XFRM_POLICY_ICMP` (`SPParams.ICMPErrorPayload`) on an
  operator bypass's FWD transit gates and FWD decision, and never on an OUT
  gate, where the flag is what `icmp_route_lookup` reads to source an error
  this node sends (owner ruling 2026-11-20; `spdTransitPolicies`,
  `engine/spd_policy.go`); `TestRFC4301ForwardedICMPPastBypassDecisionUsesPayloadPolicy`
  (`engine/rfc4301_forward_icmp_integration_linux_test.go`) exercises it.
  The outbound override is refused at commit by owner decision (2026-11-12),
  and logged at start for a configuration no commit judged: a bypass entry,
  or `unmatched bypass`, that would carry an ICMP error the kernel maps into
  a Child SA of a peer with `icmp-error-check true` (`refuseICMPErrorBypass`,
  `engine/icmp_overlap.go`). A peer bound to a VTI is exempt: its policies
  carry the interface's if_id, which the reverse lookup (`xfrm_lookup`,
  if_id 0) never matches. The refusal names the fix, `icmp-error-check false`
  on the peer among them. The inbound discard override is not refused: by
  owner decision (2026-11-19, "Kernel honours discard") Ze's kernel honours an
  operator `in` discard entry (K1 bit 5), and the catch-all's inbound discard
  carries `XFRM_POLICY_ICMP` (`catchAllYieldsToICMPErrorCheck`,
  `dataplane/xfrm_linux.go`) so a consistent error still passes it.

<!-- source: rfc/short/rfc4301.md -- RFC4301-6.2-4 -->
<!-- source: gokrazy/kernel/patches/0003-xfrm-state-scope.patch -- XFRM_ZE_CAP_ICMP_REVERSE, XFRM_ZE_CAP_ICMP_FORWARD_MAP, XFRM_ZE_CAP_ICMP_FORWARD_GATE, xfrm_out_fwd_icmp_block, __xfrm_route_forward, __xfrm_policy_check, icmp_err_packet -->
<!-- source: internal/component/ike/engine/spd_policy.go -- spdTransitPolicies -->
<!-- source: internal/component/ike/dataplane/xfrm_state_scope_linux.go -- checkXFRMStateScopeCapabilities -->
<!-- source: internal/component/ike/dataplane/policy_role.go -- catchAllYieldsToICMPErrorCheck -->
<!-- source: internal/component/ike/engine/icmp_overlap.go -- refuseICMPErrorBypass -->

### LL-MPLS-FORWARD-NO-PMTU

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-10-30
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

### LL-SEG6-HEADEND-NO-TTL-DECREMENT

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-11-22
- **Limitation:** Stock Linux `seg6_build_state` sets
  `LWTUNNEL_STATE_INPUT_REDIRECT` on L3 modes (`encap`, `encap.red`,
  `inline`). Forwarded packets therefore bypass `ip_forward` and
  `ip6_forward`. Packets leave with their inner IPv4 TTL or IPv6 Hop Limit
  unchanged, and a packet with TTL or Hop Limit 1 is encapsulated instead of
  answered with ICMP Time Exceeded.
- **Blocks:** RFC 8986 Section 5.1 H.Encaps S05 ("Decrement inner IPv6 Hop
  Limit or IPv4 TTL") on stock kernels.
- **Evidence:** `net/ipv6/seg6_iptunnel.c` `seg6_build_state` sets
  `newts->flags |= LWTUNNEL_STATE_INPUT_REDIRECT;` unconditionally before
  checking the mode. `__seg6_do_srh_encap` carries the false comment
  `"decapsulation will overwrite inner hlim with outer hlim"` (after
  `"hlim will be decremented in ip6_forward() afterwards"`), while
  `net/ipv6/seg6_local.c` contains no hop-limit overwrite.
- **Re-check:** read `seg6_build_state` in `net/ipv6/seg6_iptunnel.c` at the
  new kernel version. Lift this entry if L3 modes drop
  `LWTUNNEL_STATE_INPUT_REDIRECT` or otherwise check and decrement the inner
  TTL and send ICMP Time Exceeded. Then confirm whether
  `gokrazy/kernel/patches/0014-seg6-forward-ttl.patch` still applies or is no
  longer needed.
- **Ze meanwhile:** Ze's runtime kernel carries
  `gokrazy/kernel/patches/0014-seg6-forward-ttl.patch` with marker
  `CONFIG_IPV6_SEG6_FORWARD_TTL=y` pinned in `gokrazy/kernel/runtime.require`.
  The `fib-kernel` plugin enrolls degrading capabilities `srv6-encap` and
  `srv6-forward-ttl`, probed behaviorally in a throwaway namespace
  (`kernelcap.SRv6Encap`, `kernelcap.SRv6ForwardTTL`).

<!-- source: gokrazy/kernel/patches/0014-seg6-forward-ttl.patch -- seg6_build_state, __seg6_do_srh_encap, CONFIG_IPV6_SEG6_FORWARD_TTL -->
<!-- source: gokrazy/kernel/runtime.require -- CONFIG_IPV6_SEG6_FORWARD_TTL -->
<!-- source: internal/plugins/fib/kernel/kernelcap_linux.go -- srv6EncapCapability, srv6ForwardTTLCapability -->
<!-- source: internal/component/kernelcap/probe_srv6_linux.go -- SRv6Encap, SRv6ForwardTTL -->

### LL-IPV6-NO-RP-FILTER

- **Kind:** kernel
- **Effect:** constrains design
- **Status:** verified
- **Checked:** Linux 7.3, 2026-10-30
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
  lose the selector family and port masks. The state writer has no way to
  set `XFRM_STATE_AF_UNSPEC`. The kernel is not the limit.
- **Blocks:** reading back an SA that uses extended sequence numbers, a replay
  window above 32, transport-mode templates without a destination,
  dual-family policy dumps, and a tunnel-mode SA carrying an inner family
  other than its outer one, through the unpatched library.
- **Evidence:** library, `xfrm_state_linux.go`: the writer sends
  `out := nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, writeReplayEsn(state.ReplayWindow))`,
  while the reader's attribute switch handles only `case nl.XFRMA_REPLAY_VAL:`
  and has no `XFRMA_REPLAY_ESN_VAL` case. The full list of corrections is in
  `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, "Vendored netlink patch".
- **Re-check:** at a new module version, check each patch of the series
  `internal/le/vendorpatch/patches/netlink-xfrm-*.patch` against the
  module source; a patch the release already carries is deleted, and the
  entry is lifted when only `netlink-xfrm-90-state-extra-attributes.patch`,
  the generic hook that carries Ze's private kernel attribute, remains. `TestNetlinkXFRMPatchApplied`
  (`internal/le/vendorpatch`) fails when the vendored copy lacks the patch,
  for example after `go mod vendor`.
- **Ze meanwhile:** works around it. Ze vendors the module and applies the
  series in file-name order.

<!-- source: internal/le/vendorpatch/patches/ -- netlink-xfrm-*.patch, XFRM state and policy decoding corrections -->

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

### LL-FD-RECEIVE-DEADLINE

- **Kind:** library
- **Effect:** constrains design
- **Status:** verified
- **Checked:** module github.com/ftrvxmtrx/fd v0.0.0-20150925145434-c6d800382fff, 2026-11-17
- **Limitation:** The unpatched module's `fd.Get` receives descriptors through
  a duplicated socket and a raw blocking syscall. The original
  `net.UnixConn` deadline does not bound that receive, so a peer that withholds
  descriptors can keep statistics connection setup blocked.
- **Blocks:** bounded statistics connection setup for AC-27's physical-process
  ownership check before counter publication
  (`plan/post-release/features/spec-pppoe-vpp-dataplane.md`).
  The configured GoVPP retry deadline alone cannot finish the unpatched receive.
- **Evidence:** in the UNPATCHED upstream `fd.go` at the checked version
  (SHA256 `07e3fc99b7f6934c78e30e2ced97b7e55ebe7d906aefa5730cbf169e768d9e86`),
  `Get` calls `viaf, err := via.File()`, then
  `socket := int(viaf.Fd())` and
  `_, _, _, _, err = syscall.Recvmsg(socket, nil, buf, 0)`.
  These historical lines are retained in `fd-receive-deadline.patch`;
  the patched vendor source now uses `via.ReadMsgUnix(nil, buf)`.
  GoVPP's `StatsClient.connect` sets
  `conn.SetDeadline(time.Now().Add(sc.retryTimeout))` before
  `files, err := fd.Get(conn, 1, nil)` on its pinned path.
- **Re-check:** read `Get` in the upstream module version required by
  `go.mod` and GoVPP's descriptor receive path. Remove the carried patch when
  upstream honors the original connection's deadline and descriptor ownership
  contract. Confirm timeout and successful descriptor receipt on Ze's kernel,
  including zero-payload rights and a missing-descriptor refusal.
- **Ze meanwhile:** carries `fd-receive-deadline.patch`, published through
  the canonical vendor installer. It receives through the original UnixConn's
  poller, so the configured read deadline bounds descriptor receipt. It parses
  only the ancillary bytes received and accepts descriptor transfers with
  zero or one payload byte. An otherwise successful receive without ancillary
  data returns `io.ErrUnexpectedEOF`. The caller closes every returned file,
  including partial results returned with an error.

<!-- source: internal/le/vendorpatch/patches/fd-receive-deadline.patch -- upstream Get receive replacement -->
<!-- source: vendor/github.com/ftrvxmtrx/fd/fd.go -- Get -->
<!-- source: vendor/go.fd.io/govpp/adapter/statsclient/statsclient.go -- StatsClient.connect -->
<!-- source: internal/component/vpp/govpp_stats_identity_linux_test.go -- TestGoVPPPinnedStatsBoundsDescriptorReceipt, TestGoVPPPinnedStatsMapsOriginalDescriptor -->
<!-- source: internal/component/vpp/govpp_stats_zero_payload_linux_test.go -- TestGoVPPPinnedStatsMapsZeroPayloadDescriptor, TestGoVPPPinnedStatsRejectsMissingDescriptor -->

## Considered and left out

| Candidate | Why it is not an entry |
|-----------|------------------------|
| IPv4 fragmentation on an MPLS push route looping on empty fragments | The kernel Ze ships no longer has it. `net/ipv4/ip_output.c` `ip_do_fragment` at Linux 7.3 refuses an MTU below one fragment: `if (mtu < hlen + 8) {` then `err = -EMSGSIZE;`. Older kernels without that check loop in `ip_frag_next`; `mplsfib.PathMTUMinimum` keeps Ze off that path on any host kernel |
| Conntrack runs after IP defragmentation, so it never sees a fragment | The defined order of netfilter, and no feature is blocked: the fragment-flood classifier is planned on packet sampling, which sees fragments (`docs/architecture/ddos/cp-survival-5-detect-5-characterization.md`) |
| Linux has no route tag attribute | No Ze feature needs the tag in the kernel: Ze carries it in its own RIB into redistribution and the OSPF External Route Tag |
