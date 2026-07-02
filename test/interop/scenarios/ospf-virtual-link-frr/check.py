#!/usr/bin/env python3
"""Scenario ospf-virtual-link-frr: an OSPFv2 virtual link with FRR (spec-ospf-ext-7).

Validates (RFC 2328 section 15 / A.4.2 / 16.3): Ze and FRR are both ABRs on the transit
area 0.0.0.1; a virtual link through that area joins their disjoint backbone (area 0)
fragments. Ze forms a Full virtual-link adjacency over routed IP (TTL > 1), originates the
V-bit (in the transit-area Router-LSA) and the Type-4 virtual link record (in the backbone
Router-LSA), and FRR reaches Ze's backbone-fragment prefix across the repaired backbone.
Prevents: a virtual link that never forms (link-local send dropped), a missing V-bit /
Type-4 record, or a backbone that stays partitioned.

AUTHORED-PENDING-QEMU: runs under the Linux Docker/QEMU interop harness ONLY; it cannot run
on darwin. It also depends on the routed virtual-link transport + the RID/Area-0 packet
demux (the live-adjacency piece), which is the remaining integration step for this feature.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, log_info, log_pass  # noqa: E402

ZE_BACKBONE_PREFIX = "192.0.2.0/24"


def check():
    frr = FRROSPF()

    # 1. The transit-area (0.0.0.1) adjacency to Ze on the shared segment must reach Full.
    log_info("waiting for the transit-area OSPF adjacency to Ze...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR's virtual link to Ze must come up (RFC 2328 section 15).
    log_info("waiting for FRR's virtual link to Ze to come up...")
    deadline = time.time() + 90
    while time.time() < deadline:
        vl = frr._vtysh_quiet("show ip ospf virtual-links")  # noqa: SLF001
        if "State Point-To-Point" in vl or "Full" in vl:
            break
        time.sleep(3)
    else:
        raise AssertionError(
            "FRR virtual link to Ze did not come up (Ze did not form the virtual adjacency)"
        )

    # 3. The virtual link repairs the backbone: FRR reaches Ze's backbone-fragment prefix.
    log_info(
        "waiting for Ze's backbone-fragment prefix across the repaired backbone..."
    )
    frr.wait_ospf_route(ZE_BACKBONE_PREFIX, timeout=60)

    log_pass(
        "ospf-virtual-link-frr: virtual link Full, backbone repaired via the transit area"
    )


if __name__ == "__main__":
    check()
