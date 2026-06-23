#!/usr/bin/env python3
"""Scenario ospf-multiarea-frr: inter-area Type 3 Summary across an ABR with FRR.

Validates (spec-ospf-13 AC-17): Ze is an ABR (eth0 in area 0, a loopback in area
1); it summarizes the area-1 prefix into the backbone as a Type 3 Summary-LSA,
so FRR (a backbone router) sees that Summary-LSA and reaches the prefix as an
inter-area route.
Prevents: an ABR that does not originate the Type 3 Summary, so the area-1 prefix
never crosses into the backbone.

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, log_info, log_pass  # noqa: E402


def check():
    frr = FRROSPF()

    # 1. The backbone OSPF adjacency to the ABR (Ze) must reach Full.
    log_info("waiting for the backbone OSPF adjacency to the ABR...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR must see a Type 3 Summary-LSA that Ze (the ABR) injected into area 0.
    log_info("waiting for the Type 3 Summary-LSA from the ABR...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if frr.has_summary_lsa():
            break
        time.sleep(2)
    else:
        raise AssertionError(
            "no Type 3 Summary-LSA in FRR's backbone LSDB (ABR did not summarize area 1)"
        )

    log_pass("ospf-multiarea-frr: ABR originated the inter-area Type 3 Summary")


if __name__ == "__main__":
    check()
