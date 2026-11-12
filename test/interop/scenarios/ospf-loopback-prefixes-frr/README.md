# Kernel loopback prefix forwarding

Ze's real kernel lo holds 127.0.0.1/8 followed by 10.81.0.2/32 and
10.81.1.2/32. OSPF runs lo as a passive loopback and eth0 as point-to-point.
FRR originates 10.81.0.3/32 from its own lo.

The checker reads Ze's lo address inventory, waits for FRR's Full adjacency,
and requires FRR to install both of Ze's /32s through Ze's lab address.
Each exact route must be selected and installed by OSPF, with the matching
active FIB next hop. Ze must install an exact kernel route to FRR's /32
through FRR's lab address with FIB protocol 250. Each Ze global loopback
then pings FRR's loopback, and FRR pings each one back, using the loopbacks
as source addresses. No checker installs a route.

Reading only lo's primary IPv4 address advertises host127 and loses both
global prefixes. This scenario uses the actual OS loopback so that a dummy
interface with only a global address cannot conceal that defect.

The checker is registered in
`internal/le/interoplab/bgp/register_igp_loopback.go`; its forwarding checks
are in `check_igp_loopback.go`. The installed-route predicate is the existing
`requireRouteInstalledVia`, whose positive and negative tests reject fields
split across different route or next-hop objects.

The peer source was read at tag `frr-10.7.1`:

- `ospfd/ospf_lsa.c`, `lsa_link_loopback_set`: a loopback contributes its
  address as a stub link with a /32 mask.
- `ospfd/ospf_vty.c`, `show_ip_ospf_neighbour_brief` and
  `show_ip_ospf_neighbor_sub`: the brief neighbor output renders the
  neighbor FSM state and excludes the local neighbor.
- `zebra/zebra_vty.c`, `vty_show_ip_route`: each exact prefix's JSON route
  carries `protocol`, `selected`, `destSelected` and `installed`.
- `lib/nexthop.c`, `nexthop_json_helper`: `ip`, `active` and `fib` belong
  to one next-hop object. The predicate requires them on Ze's next hop.

Ze's kernel query retains `show exact <FRR-prefix> proto 250`. iproute2's
`filter_nlmsg` rejects other protocols, and `print_route` omits the redundant
protocol field when that filter is explicit. The checker requires the
destination and gateway from this filtered reply.

Run one exact selector (comma-separated selectors are not supported):

```sh
INTEROP_SCENARIO=ospf-loopback-prefixes-frr ./le test integration interop
./le test qemu docker-lab lab "test integration interop" env "INTEROP_SCENARIO=ospf-loopback-prefixes-frr" timeout 3600s
```
