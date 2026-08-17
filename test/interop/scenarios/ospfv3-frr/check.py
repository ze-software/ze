#!/usr/bin/env python3
"""Scenario ospfv3-frr: point-to-point OSPFv3 (IPv6) adjacency with FRR ospf6d.

Validates (spec-ospf-af-unify AC-4 / AC-7): the unified OSPF engine's IPv6 family
and FRR ospf6d form a point-to-point OSPFv3 adjacency over the shared bridge and
synchronise the OSPFv3 LSDB -- FRR carries Ze's Router-LSA, proving the v6
Hello/DD/LSR/LSU/LSAck exchange reached Full over the ospfv3 codec + transport.
Prevents: a v6 adjacency that never reaches Full (Hello/DD mismatch over ff02::5)
or an LSDB that never synchronises.

GOAL-VALIDATION TARGET: this is the red TDD target for the v6 bring-up. It passes
once the unified engine's IPv6 family reaches Full (the AFPrefixStrategy / v6 FSM
work in spec-ospf-af-unify); until then it is expected to fail at wait_adjacency.

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over
ff02::5 + FRR ospf6d); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, log_info, log_pass  # noqa: E402

# Ze's OSPFv3 Router ID (its ze.conf router-id; the advertising router in the LSDB).
ZE_ROUTER_ID = "172.30.0.2"


def check():
    frr = FRROSPF6()

    # 1. The P2P OSPFv3 adjacency must reach Full.
    log_info("waiting for P2P OSPFv3 adjacency between Ze and FRR ospf6d...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR's OSPFv3 LSDB must carry Ze's Router-LSA -- proof the DD exchange and
    #    LSDB synchronisation completed (the Full milestone), independent of any
    #    IPv6 route install (the v6 prefix model is a later phase).
    log_info("waiting for FRR to learn Ze's OSPFv3 Router-LSA...")
    deadline = time.time() + 60
    have_lsa = False
    while time.time() < deadline:
        if frr.has_router_lsa(ZE_ROUTER_ID):
            have_lsa = True
            break
        time.sleep(2)
    if not have_lsa:
        print(frr._vtysh_quiet("show ipv6 ospf6 database router")[:800])
        raise AssertionError(
            "FRR did not learn Ze's Router-LSA over the OSPFv3 adjacency"
        )

    # 3. Stability: the adjacency must still be Full after a short settle.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("P2P OSPFv3 adjacency did not stay Full (flapping)")

    log_pass("ospfv3-frr: P2P OSPFv3 adjacency formed and LSDB synchronised")


if __name__ == "__main__":
    check()
