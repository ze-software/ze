## BGP Protocol Interop

Ze's BGP engine against real FRR, BIRD, and GoBGP, scenario by scenario.

 `Daemon`

A Docker orchestrator launches Ze and one or more peer daemons on an isolated network, establishes real BGP sessions, and asserts correct behavior through each daemon's own CLI (`vtysh`, `birdc`, `gobgp`). Daemons start conditionally per scenario, so each run only spins up what it needs.

68 scenarios: eBGP/iBGP, 4-byte ASN, IPv6, Add-Path, Route Refresh, Graceful Restart, Route Server, standard and extended communities, MD5 auth, BGP Roles, EVPN, VPN, FlowSpec, multihop, BFD, ECMP, SRv6, RPKI, BMP, max-prefix, GTSM, AS112, and OSPF/IS-IS interop with FRR.

 - **Proves:** Real BGP sessions against production daemon implementations, not mocks
 - **Peers:** Real FRR, BIRD, and GoBGP, in Docker containers
 - **Requires:** Docker, Python 3, ~1.5 GB disk for daemon images
 - **Source:** [docs/architecture/testing/interop.md](https://ze-software.github.io/ze/docs/architecture/testing/interop/)

```
# all 68 scenarios
$ make ze-interop-test

# single scenario, verbose
$ python3 test/interop/run.py 01-ebgp-ipv4-frr
```

 `Prerequisites`

Docker only, no QEMU path. Set `FRR_IMAGE` to pin a different FRR version, or `NO_BUILD=1` to skip rebuilding images on repeat runs.

 [test/interop/ lab source](https://github.com/ze-software/ze/tree/main/test/interop)
