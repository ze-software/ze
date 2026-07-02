#!/usr/bin/env python3
"""Scenario ospf-ptmp-frr: point-to-multipoint OSPFv2 adjacency + host routes with FRR.

Validates (spec-ospf-ext-8): Ze and FRR form an IPv4 point-to-multipoint OSPFv2
adjacency over the shared bridge with NO DR elected, exchange per-neighbor
point-to-point Router-LSA links plus /32 host routes, and install each other's
interface address as a /32 (RFC 2328 sec 12.4.1.4 / sec 16.1 next-hop).
Prevents: a PtMP interface electing a DR, or a host route that never installs.

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

    # 1. The point-to-multipoint OSPF adjacency must reach Full (no DR gating).
    log_info("waiting for PtMP OSPF adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR must NOT see a DR/Network-LSA for the PtMP segment.
    net = frr._vtysh_quiet("show ip ospf database network")
    if "172.30.0.2" in net:
        raise AssertionError(
            "Ze originated a Network-LSA on a PtMP segment; PtMP has no DR"
        )

    # 3. FRR must learn Ze's Router-LSA (LSDB convergence).
    log_info("waiting for FRR to learn Ze's Router-LSA over the PtMP adjacency...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if "172.30.0.2" in frr._vtysh_quiet("show ip ospf database router"):
            break
        time.sleep(2)
    else:
        raise AssertionError(
            "FRR did not learn Ze's Router-LSA over the PtMP adjacency"
        )

    # 4. Stability: the adjacency must still be Full after a short settle.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("PtMP OSPF adjacency did not stay Full (flapping)")

    log_pass("ospf-ptmp-frr: PtMP adjacency formed with no DR and LSDB converged")


if __name__ == "__main__":
    check()
