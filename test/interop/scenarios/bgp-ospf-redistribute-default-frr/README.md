# OSPF redistribution with the external BGP default

FRR configures 10.82.0.3/24 on its real loopback and originates the subnet
10.82.0.0/24 through OSPF's point-to-point network type. Ze and FRR also form
an eBGP session, with AS 65001 on Ze and AS 65002 on FRR. Ze's only
redistribution rule imports OSPF into BGP, and neither router originates this
prefix through a BGP network statement, static route or API injection.

The checker first requires FRR's loopback address and OSPF point-to-point
network type on lo, a Full OSPF adjacency,
Ze's exact kernel route through FRR under protocol 250, and an established
BGP session. FRR's per-prefix BGP JSON must then report one valid best path
from external peer Ze with AS_PATH 65001 and exactly one next hop: Ze's
session-local address. The session must remain established afterward.
Other OSPF prefixes are permitted because the configured import accepts the
IPv4 family; this scenario makes no whole-table exclusion claim.

FRR owns the tested subnet as a connected route, so its zebra FIB need not
select the BGP alternative. The acceptance assertion therefore reads FRR's
BGP RIB. A default candidate preserves the IGP gateway until Ze selects the
destination peer; its external default selects Ze's endpoint. No configured
next-hop override masks that choice.

Ze's `Session.writeUpdateGated` in
`internal/component/bgp/reactor/session_write.go` withholds an originated
route whose final next hop is the destination peer's address. The direct
batch writer and both queue drains reach that guard. Preserving FRR's IGP
gateway as an explicit next hop therefore meets Ze's own send refusal;
this scenario must instead produce an accepted path using Ze's endpoint.

FRR 10.7.1's `bgp_update` refuses a host-route NLRI matching a local address,
before input filtering or next-hop validation. `bgp_hostroute_self` performs
that exact lookup in its connected-address hash. Thus FRR's own /32 cannot
serve as an accepted BGP control. The non-host /24 preserves the same real
IGP gateway and destination-specific next-hop behavior without that unrelated
receiver refusal.

The deciding peer source was read at `frr-10.7.1`:

- `ospfd/ospf_vty.c`, `ip_ospf_network`, sets lo's OSPF type to point-to-point;
  `ospf_if_up` in `ospfd/ospf_interface.c` uses that type for interface-up.
- `ospfd/ospf_lsa.c`, `router_lsa_link_set` and `lsa_link_ptop_set`, originate
  a stub subnet using the configured address mask, regardless of neighbors.
- `ospfd/ospf_vty.c`, `show_ip_ospf_interface_sub`, prints the interface's
  Internet Address and Network Type used by the producer readiness check.
- `bgpd/bgp_route.c`, `bgp_update`, and `bgpd/bgp_nexthop.c`,
  `bgp_hostroute_self`, refuse host prefixes matching local interface addresses.
- `bgpd/bgp_nexthop.c`, `bgp_nexthop_self`, looks up the received IPv4
  next hop in FRR's local address hash.
- `bgpd/bgp_route.c`, `bgp_update_martian_nexthop` and `bgp_update`, filter
  the route when that lookup identifies FRR's own address.
- `bgpd/bgp_route.c`, `route_vty_out_detail`, writes the accepted path's
  validity, best-path selection, external peer, AS path and next-hop JSON.

The registration and checker are in
`internal/le/interoplab/bgp/register_ospf_redistribute_default.go` and
`check_ospf_redistribute_default.go`. Its pure predicate and scripted checker
tests verify refusal of incomplete or unrelated evidence. They do not replace
the actual FRR run.

The startup log enables only `bgp.redistribute` and
`redistribute.orchestrator` at debug, preserving the actual consumer command
and orchestrator decisions. A failed assertion retains bounded excerpts from
FRR's full BGP table, neighbor counters and accepted/retained received routes;
Ze's consumer and orchestrator log rings, peer detail/statistics and advertised
RIB, OSPF routes, system-RIB candidates, logger levels and metrics; and Ze's
container log. All failure reads share a separate 15-second deadline. Query
recordings retain complete answers and query errors. These observations locate
the failed boundary without changing the assertion or injecting a route.

Run the exact native selector:

```sh
INTEROP_SCENARIO=bgp-ospf-redistribute-default-frr ./le test integration interop
./le test qemu docker-lab lab "test integration interop" env "INTEROP_SCENARIO=bgp-ospf-redistribute-default-frr" timeout 3600s
```

For discrimination, preserve a pristine consumer file, then change only
`formatAnnounce` in `internal/component/bgp/redistribute/consumer.go` to emit
the gateway directly instead of the `default ` gateway form. Mark that
temporary cut `// MUTATION-APPLIED` and verify its diff before rebuilding.
The OSPF and session controls must still pass, while the positive assertion
for FRR's accepted BGP path must fail. Receiver absence alone does not show
whether Ze withheld a route or FRR received and rejected it. Restore the
pristine bytes and rerun the same exact selector. The mutation must never be
committed. No execution result is claimed by this README.

The corrected /24 topology requires its own fresh old-explicit failure and
restored pass. The earlier /32 failure also met FRR's own-host-prefix refusal,
so it cannot discriminate the corrected topology's next-hop default.
