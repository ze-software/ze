# L2 point-to-point defaults and loopback forwarding

Ze sets `isis level l2` and omits the level on both eth0 and passive lo.
FRR offers both levels on its point-to-point circuit. Both NETs use area
49.0001. The checker requires FRR's L2
adjacency, an installed IS-IS route to Ze's 10.80.0.2/32 through Ze's lab
address, and Ze's exact kernel route to FRR's 10.80.0.3/32 through FRR's lab
address with FIB protocol 250. Each router then pings the other's loopback
using its own loopback as the source.

The old effective-level defect lets Ze prefer L1 when the peer offers both
levels, while Ze's process maintains its L2 database. The exact L2 adjacency
assertion rejects this mismatch. The installed routes and source-address
pings also require the complete L2 SPF and forwarding path. An L2-only peer
would force the defective Ze to choose L2 and conceal the defect. No checker
installs a route.

Both peers use the real kernel lo. Ze's configured address inventory retains
127.0.0.1/8 alongside its global /32, and the checker reads both from lo.

The checker is registered in
`internal/le/interoplab/bgp/register_igp_loopback.go`; its forwarding checks
are in `check_igp_loopback.go`. It reuses `requireRouteInstalledVia` for FRR's
native route JSON rather than accepting a protocol-table substring.

The peer source was read at tag `frr-10.7.1`:

- `isisd/isis_adjacency.c`, `isis_adj_print_vty`: the detailed adjacency line
  places the interface, numeric level and state together.
- `isisd/isis_pdu.c`, `put_hello_hdr` and `process_p2p_hello`: FRR advertises
  its configured circuit type. A dual-level peer with a matching area
  produces adjacency type 3 when Ze also offers both levels, and type 2
  when Ze correctly offers only L2.
- `isisd/isis_circuit.c`, `isis_circuit_add_addr`: the connected IPv4 inventory
  excludes host127 and retains the global loopback prefix.
- `isisd/isis_lsp.c`, `lsp_build`: each up circuit contributes its IPv4
  prefixes, including passive circuits, to the generated LSP.
- `zebra/zebra_vty.c`, `vty_show_ip_route`: the exact prefix's route object
  carries `protocol`, `selected`, `destSelected` and `installed`.
- `lib/nexthop.c`, `nexthop_json_helper`: the same next-hop object carries
  `ip`, `active` and `fib`. The predicate requires all three for Ze's address.

Ze's kernel query retains `show exact <FRR-prefix> proto 250`. iproute2's
`filter_nlmsg` rejects other protocols, and `print_route` omits the redundant
protocol field when that filter is explicit. The checker therefore requires
the destination and gateway from this filtered reply, without expecting a
printed `proto 250` token.

Run one exact selector (comma-separated selectors are not supported):

```sh
INTEROP_SCENARIO=isis-p2p-level2-default-frr ./le test integration interop
./le test qemu docker-lab lab "test integration interop" env "INTEROP_SCENARIO=isis-p2p-level2-default-frr" timeout 3600s
```
