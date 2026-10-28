# Two owners share one name

Two independent producers draw names from one namespace, and the consumer that
joins them treats an equal name as the same object. Nothing reserves a name to
its producer, so an operator's free-form name can land on a name a daemon
feature builds, and the join merges two things that were never meant to meet.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-28 | firewall-arp-nd-matches | firewall `mergeSameNameTables` | An operator table `vrrp_owner_nd` (family ip6, chain `output`) becomes `ze_vrrp_owner_nd` and is merged into the VRRP owner table, giving one table two `output` base chains. Kernel outcome not measured. The ownership page lists no VRRP table | open: not fixed: `plan/spec-firewall-arp-nd-matches.md` D-3 |
| 2026-10-10 | yang-rpc-declarations-with-no-handler (review round 1 NOTE-3) | bgp reactor `recordCreatedPeerLocked` | `create bgp peer 192.0.2.7` keys its running entry `peer-192.0.2.7`. A configured peer the operator already keyed `peer-192.0.2.7` at another address is overwritten in the running tree (AddPeer checks the address only), so `update bgp config` then drops that configured peer from the file. Predates the spec | open: not fixed |
| 2026-10-28 | spec-l2tp-pool-reload-applies | `poolPlugin.handle` (`l2tp/plugins/pool/register.go`), `ipv4Pool.allocate` (`pool.go`) | Read, not run: a RADIUS Framed-IP-Address is stored with `fromPool: false` and never marked in the pool bitmap, so when it lies inside a configured range `allocate` can hand the same address to a second subscriber while the first still holds it | open: reserve an in-range RADIUS-assigned address in its pool for the session's life |
