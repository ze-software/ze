Protocol lab

# BGP Protocol Interop

Ze's BGP engine against real FRR, BIRD, and GoBGP, scenario by scenario.

## Two ways to test Ze.

Use the Ze interop harness for protocol assertions, or netlab for reusable multi-node topologies.

**Ze harness**

### [Protocol interop scenarios](https://ze-software.net/architecture/testing/interop/)

Run 68 curated scenarios against FRR, BIRD, and GoBGP. The harness starts only the required daemons and checks behavior through each peer's own CLI.

`68 scenarios` `FRR` `BIRD` `GoBGP` Use the Ze harness scenario assertions

**netlab**

### [Reusable topology lab](https://ze-software.net/labs/netlab/)

Describe a three-node topology in YAML. netlab renders Ze configuration and starts each daemon node under containerlab.

`YAML` `containerlab` `BGP` `OSPF` `IS-IS` `BFD` Use netlab three-node topology `Daemon`

A Docker orchestrator launches Ze and one or more peer daemons on an isolated network, establishes real BGP sessions, and asserts correct behavior through each daemon's own CLI (`vtysh`, `birdc`, `gobgp`). Daemons start conditionally per scenario, so each run only spins up what it needs.

68 scenarios: eBGP/iBGP, 4-byte ASN, IPv6, Add-Path, Route Refresh, Graceful Restart, Route Server, standard and extended communities, MD5 auth, BGP Roles, EVPN, VPN, FlowSpec, multihop, BFD, ECMP, SRv6, RPKI, BMP, max-prefix, GTSM, AS112, and OSPF/IS-IS interop with FRR.

- **Proves:** Real BGP sessions against production daemon implementations, not mocks
- **Peers:** Real FRR, BIRD, and GoBGP, in Docker containers
- **Requires:** Docker, Python 3, ~1.5 GB disk for daemon images
- **Source:** [docs/architecture/testing/interop.md](https://ze-software.net/architecture/testing/interop/)

```
# all 68 scenarios
$ make ze-interop-test

# single scenario, verbose
$ python3 test/interop/run.py 01-ebgp-ipv4-frr
```

`Prerequisites`

Docker only, no QEMU path. Set `FRR_IMAGE` to pin a different FRR version, or `NO_BUILD=1` to skip rebuilding images on repeat runs.

- [test/interop/ lab source](https://github.com/ze-software/ze/tree/main/test/interop)
