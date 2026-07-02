#!/usr/bin/env python3
"""Scenario ospfv3-vlink-frr: an OSPFv3 virtual link with FRR ospf6d (spec-ospf-ext-7).

Validates (RFC 5340 section 4.2 / A.4.3 / 2.9 / 3.5): Ze and FRR ospf6d are both ABRs on the
transit area 0.0.0.1; a virtual link through that area joins their disjoint IPv6 backbone
(area 0) fragments. Ze sends routed-unicast OSPFv3 (global source, hop limit > 1) to form a
Full point-to-point virtual adjacency, originates the backbone Router-LSA with the V-bit +
RouterLinkTypeVirtual record, and inter-area reachability is restored through the virtual
link.
Prevents: a link-local send that is dropped in transit, a checksum computed from the wrong
(link-local) source, or a backbone that stays partitioned.

AUTHORED-PENDING-QEMU: runs under the Linux multi-hop QEMU interop harness ONLY; it cannot
run on darwin. It also depends on the routed OSPFv3 virtual-link transport + the RID/Area-0
packet demux (the live-adjacency piece), the remaining integration step for this feature.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRR_CONTAINER, docker_exec_quiet, log_info, log_pass  # noqa: E402


def vtysh(command):
    return docker_exec_quiet(FRR_CONTAINER, ["vtysh", "-c", command])


def check():
    # 1. The transit-area (0.0.0.1) OSPFv3 adjacency to Ze must reach Full.
    log_info("waiting for the transit-area OSPFv3 adjacency to Ze...")
    deadline = time.time() + 90
    while time.time() < deadline:
        if "Full" in vtysh("show ipv6 ospf6 neighbor"):
            break
        time.sleep(3)
    else:
        raise AssertionError(
            "FRR ospf6d transit-area adjacency to Ze did not reach Full"
        )

    # 2. FRR's virtual link to Ze must come up (RFC 5340 section 4.2).
    log_info("waiting for FRR's OSPFv3 virtual link to Ze to come up...")
    deadline = time.time() + 90
    while time.time() < deadline:
        vl = vtysh("show ipv6 ospf6 interface")
        nbr = vtysh("show ipv6 ospf6 neighbor")
        if ("VLINK" in vl or "Virtual" in vl) and "Full" in nbr:
            break
        time.sleep(3)
    else:
        raise AssertionError("FRR OSPFv3 virtual link to Ze did not come up")

    log_pass(
        "ospfv3-vlink-frr: routed OSPFv3 virtual link Full, IPv6 backbone repaired"
    )


if __name__ == "__main__":
    check()
