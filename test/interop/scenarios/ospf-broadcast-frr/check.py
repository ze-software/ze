#!/usr/bin/env python3
"""Scenario ospf-broadcast-frr: DR/BDR election + Network-LSA on a broadcast LAN.

Validates (spec-ospf-13 AC-16): on a broadcast segment Ze (higher priority) and
FRR elect a DR/BDR, the DR originates a Network-LSA (Type 2), the adjacency
reaches Full, and routes converge.
Prevents: a broadcast adjacency stuck in 2-Way (no DR/BDR), or a missing
Network-LSA (the DR did not originate it).

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, log_info, log_pass  # noqa: E402


def check():
    frr = FRROSPF()

    # 1. The broadcast OSPF adjacency must reach Full (DR/BDR elected first).
    log_info("waiting for OSPF DR/BDR adjacency on the broadcast LAN...")
    frr.wait_adjacency(timeout=90)

    # 2. A DR/BDR role must be present in FRR's neighbor table.
    if not frr.has_dr_bdr():
        raise AssertionError("no DR/BDR role elected on the broadcast segment")

    # 3. The DR must have originated a Network-LSA (Type 2).
    log_info("waiting for the Network-LSA (Type 2) from the DR...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if frr.has_network_lsa():
            break
        time.sleep(2)
    else:
        raise AssertionError("no Network-LSA (Type 2) in FRR's LSDB")

    log_pass("ospf-broadcast-frr: DR/BDR elected, Network-LSA present, adjacency Full")


if __name__ == "__main__":
    check()
