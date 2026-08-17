#!/usr/bin/env python3
"""Scenario ospfv3-multiarea-frr: OSPFv3 inter-area summary install across an ABR.

Validates (spec-ospfv3-6 AC-1): Ze is an OSPFv3 ABR (eth0 in the backbone area 0, a
passive dummy interface carrying 2001:db8:a1::/64 in area 1). It originates an
Inter-Area-Prefix-LSA (Type 0x2003) into the backbone, so FRR (a backbone router)
learns 2001:db8:a1::/64 as an OSPFv3 inter-area route across the ABR.
Prevents: an ABR that forms adjacencies in two areas but never advertises one area's
prefix into the other, so the backbone never gains inter-area reachability.

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over ff02::5
+ FRR ospf6d); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, log_info, log_pass  # noqa: E402

# Ze's OSPFv3 Router ID (the ABR advertising the inter-area summary).
ZE_ROUTER_ID = "172.30.0.2"
AREA1_PREFIX = "2001:db8:a1::/64"


def check():
    frr = FRROSPF6()

    # 1. The backbone OSPFv3 adjacency to the ABR (Ze) must reach Full.
    log_info("waiting for the backbone OSPFv3 adjacency to the ABR...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR must see the Inter-Area-Prefix-LSA (Type 0x2003) Ze injected into the backbone.
    log_info("waiting for the Inter-Area-Prefix-LSA from the ABR...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if frr.has_inter_area_prefix_lsa(ZE_ROUTER_ID):
            break
        time.sleep(2)
    else:
        print(frr.inter_area_prefix_dump()[:800])
        raise AssertionError(
            "no Inter-Area-Prefix-LSA from the ABR in FRR's backbone LSDB"
        )

    # 3. FRR must install the area-1 prefix as an OSPFv3 inter-area route (data-plane).
    log_info(f"waiting for FRR to install the inter-area route {AREA1_PREFIX}...")
    frr.wait_ospf6_route(AREA1_PREFIX, timeout=90)

    log_pass(
        "ospfv3-multiarea-frr: ABR originated the Inter-Area-Prefix-LSA and FRR "
        "installed the inter-area route"
    )


if __name__ == "__main__":
    check()
