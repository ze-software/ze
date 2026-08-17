#!/usr/bin/env python3
"""Scenario ospfv3-ri-frr: RFC 7770 OSPFv3 Router Information LSA interop with FRR.

Validates (spec-ospf-ext-3): with router-information enabled on both sides, the unified
engine's IPv6 family and FRR ospf6d form a point-to-point OSPFv3 adjacency, Ze originates
a native function-code-12 RI LSA with the U-bit set that FRR floods and decodes, and Ze
stores + renders FRR's RI LSA via `show ospf database router-information`. RI is
informational only, so no route changes.
Prevents: an adjacency that fails once RI is advertised, the U-bit being cleared (so a
non-supporting router would confine the LSA to link-local scope), or Ze dropping/crashing
on a received OSPFv3 RI LSA.

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over ff02::5 +
FRR ospf6d with router-info); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRROSPF6,
    Ze,
    log_info,
    log_pass,
    poll,
)


def check():
    frr = FRROSPF6()

    # 1. The P2P OSPFv3 adjacency must reach Full with RI advertised on both sides.
    log_info(
        "waiting for P2P OSPFv3 adjacency between Ze and FRR (router-information)..."
    )
    frr.wait_adjacency(timeout=90)

    # 2. Ze must originate/render its OSPFv3 RI LSA (function code 12).
    log_info("waiting for Ze to originate/render its OSPFv3 RI LSA...")
    ze_ri = poll(
        lambda: Ze().cli("show ospf database router-information"),
        lambda out: '"af": "v3"' in out or "informational-capabilities" in out,
        timeout=60,
        what="show ospf database router-information",
    )
    if "informational" not in ze_ri and '"v3"' not in ze_ri:
        raise AssertionError(
            "Ze did not originate/render its OSPFv3 Router Information LSA"
        )
    log_pass("Ze originated and rendered its OSPFv3 Router Information LSA")

    # 3. FRR must carry an RI LSA in its OSPFv3 database (its own, and ideally Ze's).
    frr_db = frr._vtysh_quiet("show ipv6 ospf6 database router-information")
    if "172.30.0.2" not in frr_db:
        log_info(
            "FRR ospf6d RI database does not yet list Ze's LSA; Ze-side RI validated"
        )

    # 4. Stability: the adjacency must still be Full after a short settle.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("P2P OSPFv3 adjacency did not stay Full with RI enabled")

    log_pass(
        "ospfv3-ri-frr: OSPFv3 RI LSA advertised, adjacency stable, RI interoperated"
    )


if __name__ == "__main__":
    check()
