#!/usr/bin/env python3
"""Scenario ospfv3-broadcast-frr: OSPFv3 DR/BDR election + Network-LSA on a broadcast LAN.

Validates (spec-ospf-af-unify): on an IPv6 broadcast segment Ze (higher priority) and FRR
ospf6d elect a DR/BDR over the v6 Hello (DR/BDR carried as Router IDs, RFC 5340), the DR (Ze)
originates an OSPFv3 Network-LSA (Type 0x2002) for the segment, and the adjacency reaches
Full. Prevents: a v6 broadcast adjacency stuck in 2-Way (no DR/BDR election), or a missing
Network-LSA (the DR did not originate it / no transit link in the Router-LSA).

Also validates (spec-ospfv3-4): Ze originates a link-local-scope Link-LSA (Type 0x0008) on the
segment and floods it so FRR receives it -- the LSDB-content evidence for the Link-LSA feature
(a route cannot be asserted on a shared LAN where the prefix is connected on both sides).

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over ff02::5/6 +
FRR ospf6d); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, log_info, log_pass  # noqa: E402

# Ze's OSPFv3 Router ID (the DR, advertising router of the segment Network-LSA).
ZE_ROUTER_ID = "172.30.0.2"


def check():
    frr = FRROSPF6()

    # 1. The broadcast OSPFv3 adjacency must reach Full (DR/BDR elected first).
    log_info("waiting for the OSPFv3 DR/BDR adjacency on the broadcast LAN...")
    frr.wait_adjacency(timeout=90)

    # 2. A DR/BDR role must be present in FRR's neighbor table.
    if not frr.has_dr_bdr():
        raise AssertionError("no DR/BDR role elected on the v6 broadcast segment")

    # 3. The DR (Ze) must have originated an OSPFv3 Network-LSA (Type 0x2002).
    log_info("waiting for the OSPFv3 Network-LSA from the DR...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if frr.has_network_lsa(ZE_ROUTER_ID):
            break
        time.sleep(2)
    else:
        print(frr._vtysh_quiet("show ipv6 ospf6 database network")[:800])
        raise AssertionError("no OSPFv3 Network-LSA from the DR in FRR's LSDB")

    # 4. Ze must originate and flood a link-local-scope Link-LSA (Type 0x0008) on the segment;
    #    FRR (on the same link) must receive it (spec-ospfv3-4).
    log_info(
        "waiting for Ze's OSPFv3 Link-LSA (Type 0x0008) in FRR's link-scope LSDB..."
    )
    deadline = time.time() + 60
    while time.time() < deadline:
        if frr.has_link_lsa(ZE_ROUTER_ID):
            break
        time.sleep(2)
    else:
        print(frr.link_lsa_dump()[:800])
        raise AssertionError("no OSPFv3 Link-LSA from Ze in FRR's link-scope LSDB")

    log_pass(
        "ospfv3-broadcast-frr: OSPFv3 DR/BDR elected, Network-LSA + Ze Link-LSA present, "
        "adjacency Full"
    )


if __name__ == "__main__":
    check()
