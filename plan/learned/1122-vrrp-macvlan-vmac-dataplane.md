# vrrp-macvlan-vmac-dataplane

How to make a macvlan the sole L2 owner of an IP whose parent shares the subnet.
Written closing `spec-vrrp-6-interop`; the mechanism is reusable for any
virtual-MAC / floating-IP feature, not just VRRP.

## The problem

Put a virtual IP on a bridge-mode macvlan that carries a virtual MAC, expect a
host to resolve that IP to the virtual MAC, and it does NOT: when the macvlan's
**parent** holds a real address in the same subnet, the Linux kernel answers ARP
for the VIP from the PARENT with its real MAC, and the macvlan never replies. The
macvlan *receives* the broadcast who-has (tcpdump on it shows the frames) but the
kernel picks the parent as the responder. Silencing the parent with `arp_ignore=1`
alone makes the VIP 100% unreachable (nothing answers) -- a worse state, not a fix.

## The recipe (proven byte-identical to keepalived's use_vmac)

Applied by the VRRP engine at macvlan create, restored on teardown
(`internal/plugins/vrrp/dataplane_linux.go`):

- macvlan **private** mode, not bridge.
- install the VIP with the **parent's subnet prefix** (e.g. /24), NOT /32, so the
  macvlan owns the subnet's connected route.
- `conf.<parent>.arp_ignore=1  arp_filter=1  rp_filter=1`
- `conf.<macvlan>.arp_ignore=1  rp_filter=0`
- `conf.all.rp_filter=0` -- REQUIRED: effective rp_filter is `max(all, iface)`, so
  the macvlan cannot reach 0 unless `all` is 0 too. This is the ingredient people
  miss (arp_ignore-alone attempts fail here).

Each ingredient was proven necessary by isolating it in QEMU. `disable_ipv6=1` on
the vmac (keepalived also sets it) is NOT needed and was dropped.

## How it was cracked (the debugging method matters)

Reasoning from ARP semantics went in circles for a dozen probes. What actually
worked:

1. **Freeze test.** `kill -STOP` the working keepalived and re-arping: the VIP was
   still answered with the vMAC -> proves the KERNEL answers, not a userspace ARP
   responder. So a pure-kernel recipe must exist.
2. **Exhaustive state diff.** Dump every `/proc/sys/net/ipv4` knob + routes for a
   working keepalived netns vs a failing hand-built one and `diff`. The delta was
   the recipe. Do not eyeball a subset -- diff everything.
3. **Isolate each ingredient** by toggling it off and re-testing, in the SAME
   topology as production (a bridge, not a direct veth -- the topology changed the
   race outcome).

## Traps for the next agent

- **Cold-start race is inherent, and keepalived has it too.** The very FIRST
  resolution after a neighbour-cache flush can cache the parent's real MAC once;
  every resolution after is the vMAC. Do not treat this as a bug to eliminate --
  keepalived's use_vmac shows the identical race. Interop assertions must flush +
  re-resolve to observe the convergent steady state, not read once.
- **The address owner is the exception.** If the VIP equals a real parent address
  (RFC owner), it is served by the parent's real MAC and vMAC ownership is
  unachievable (the address is local to the parent, so `arp_ignore` cannot muzzle
  it). Install such a VIP as a host route (/32), never the subnet prefix, or you
  add a duplicate connected route. `vipMaskBits` (`register.go`) does this.
- **IPv6 needs none of the ARP-flux recipe** (ND resolves the VIP to the vMAC
  natively -- NS targets the VIP's solicited-node multicast group, which only the
  macvlan joins, so the parent never competes; no cold race) **but it DOES need
  DAD disabled on the macvlan** (`net.ipv6.conf.<vmac>.accept_dad=0`). A VRRP VIP
  lives on one router at a time, so DAD is pointless; leaving it on makes the VIP
  tentative (unreachable) for ~1s after every promotion. Proven end to end against
  keepalived IPv6 (QEMU): adverts accepted both ways, election, node-death
  failover, unsolicited NA burst, and the observer resolving the VIP to
  00:00:5e:00:02:{vrid}.
- **The FIRST v6 advert sources from the macvlan's auto (EUI-64) link-local, not
  `fe80::1` -- and this CANNOT be fixed by FSM action ordering. Do not try.**
  Ground truth (QEMU, captured resolver output): the kernel gives the macvlan a
  transient EUI-64 link-local (`fe80::200:5eff:fe00:20a`) at creation; installing
  the VIPs REPLACES it, so at steady state the macvlan holds only `fe80::1` +
  the global VIP (the auto link-local is gone). The source resolver
  (`macvlanLinkLocal`) returns the first non-tentative link-local, so once VIPs
  are installed it yields `fe80::1`. The first advert predates that: `InstallVIPs`
  calls `iface.RegisterOwnedAddresses`, which is ASYNCHRONOUS (it registers the
  addr in a map + fires a reconcile trigger and returns; the kernel apply happens
  on a LATER reconcile pass -- `address_owner.go`). So the `SendAdvert` that
  runs in the same dispatcher loop resolves before `fe80::1` exists and picks the
  auto link-local. Reordering `promoteToMaster` to `InstallVIPs, SendAdvert, ...`
  was tried and PROVEN INEFFECTIVE against the keepalived IPv6 lab (first advert
  still from the auto link-local) because ordering cannot win an async race; it
  only churns the spec-annotated AC-3/R-4 order. A real fix would gate the first
  advert on `fe80::1` being present (delays the mastership claim by ~1 advert
  interval -- a failover-latency regression) or make the shared address-owner
  registry apply synchronously (invasive). Both are disproportionate: the auto
  link-local is a valid RFC 9568 link-local source, keepalived accepts it, and
  election/failover/dataplane are unaffected. Leave it; it is genuinely cosmetic.
- **You are fighting the iface component for the parent's sysctls.** iface emits
  `arp_ignore`/`arp_filter`/`rp_filter` from unit config on every apply, which
  clobbers the recipe. The engine re-asserts on every config apply
  (`reassertDataplaneSysctls`) so it self-heals.
- **`all.rp_filter=0` is host-global** and is not restored on SIGKILL. Same as
  keepalived; document it, do not try to make cleanup crash-safe.

## Files

None recorded.
