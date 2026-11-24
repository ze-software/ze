Protocol lab

# BGP Protocol Interop

These scenarios test Ze's BGP engine by exchanging routes with FRR, BIRD, and GoBGP.

## Two ways to test Ze.

Use the Ze interop harness for protocol assertions, or netlab for reusable multi-node topologies.

**Ze harness**

### [Protocol interop scenarios](https://ze-software.net/architecture/testing/interop/)

The shared routing and network-service suite has 261 scenarios, including 157 primary BGP cases. BGP external-peer scenarios start only the required FRR, BIRD, or GoBGP daemons and check behavior through each peer's own CLI.

`261 shared-suite scenarios` `FRR` `BIRD` `GoBGP` Use the Ze harness scenario assertions

**netlab**

### [Reusable topology lab](https://ze-software.net/labs/netlab/)

Describe a three-node topology in YAML. netlab renders Ze configuration and starts each daemon node under containerlab.

`YAML` `containerlab` `BGP` `OSPF` `IS-IS` `BFD` Use netlab three-node topology `Daemon`

A Docker orchestrator launches Ze and one or more peer daemons on an isolated network, establishes real BGP sessions, and asserts correct behavior through each daemon's own CLI (`vtysh`, `birdc`, `gobgp`). Daemons start conditionally per scenario, so each run only spins up what it needs.

The shared suite’s 261 routing and network-service scenarios include eBGP/iBGP, 4-byte ASN, IPv6, Add-Path, Route Refresh, Graceful Restart, Route Server, standard and extended communities, MD5 auth, BGP Roles, EVPN, VPN, FlowSpec, multihop, BFD and BFD strict mode, update delay, ECMP, SRv6, RPKI, BMP, max-prefix, GTSM, AS112, VRRP against keepalived, and OSPF/IS-IS interop with FRR.

- **What this lab tests:** BGP session and route behavior observed through the peer daemons' own commands
- **Peers:** Real FRR, BIRD, and GoBGP, in Docker containers
- **Requires:** Docker on a kernel with Ze's required features; see the kernel setup in the interop guide
- **Source:** [docs/architecture/testing/interop.md](https://ze-software.net/architecture/testing/interop/)

```
# all BGP interop scenarios
$ ./le test integration interop

# single scenario, verbose
$ INTEROP_SCENARIO=bgp-ebgp-ipv4-frr ./le test integration interop
```

`Prerequisites`

Docker runs the peer containers. On Linux, the harness checks the daemon kernel before starting Ze. On macOS, it uses Ze's Colima profile. The [interop guide](https://ze-software.net/architecture/testing/interop/) explains these prerequisites and the QEMU Docker guest. Set `FRR_IMAGE` to pin a different FRR version, or `NO_BUILD=1` to skip rebuilding images on repeat runs.

- [test/interop/ lab source](https://github.com/ze-software/ze/tree/main/test/interop)
