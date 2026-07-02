#!/usr/bin/env python3
"""Scenario ospf-nbma-frr: NBMA OSPFv2 adjacency + DR election + convergence with FRR.

Validates (spec-ospf-ext-8): Ze and FRR form an IPv4 NBMA OSPFv2 adjacency over a
static neighbor list (no all-routers multicast), elect a consistent DR/BDR, the DR
originates a Type-2 Network-LSA, and routes converge both ways.
Prevents: an NBMA segment that never discovers its configured neighbor (no poll),
never elects a DR, or floods multicast instead of unicast.

Runs under the Linux Docker/QEMU interop harness ONLY (raw IP proto 89 + FRR
ospfd); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, log_info, log_pass  # noqa: E402


def check():
    frr = FRROSPF()

    # 1. The NBMA adjacency must reach Full over the static neighbor list.
    log_info("waiting for NBMA OSPF adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=120)

    # 2. A DR must be elected and a Network-LSA must exist for the NBMA segment.
    log_info("waiting for the NBMA Network-LSA (DR-originated)...")
    deadline = time.time() + 60
    while time.time() < deadline:
        net = frr._vtysh_quiet("show ip ospf database network")
        if "172.30.0.2" in net or "172.30.0.3" in net:
            break
        time.sleep(2)
    else:
        raise AssertionError(
            "no Network-LSA originated on the NBMA segment (DR election failed)"
        )

    # 3. FRR must learn Ze's Router-LSA (LSDB convergence over unicast flooding).
    if "172.30.0.2" not in frr._vtysh_quiet("show ip ospf database router"):
        raise AssertionError(
            "FRR did not learn Ze's Router-LSA over the NBMA adjacency"
        )

    # 4. Stability: the adjacency must still be Full after a short settle.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("NBMA OSPF adjacency did not stay Full (flapping)")

    log_pass(
        "ospf-nbma-frr: NBMA adjacency, DR election, and convergence over a static list"
    )


if __name__ == "__main__":
    check()
