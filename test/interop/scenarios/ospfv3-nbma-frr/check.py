#!/usr/bin/env python3
"""Scenario ospfv3-nbma-frr: NBMA OSPFv3 (IPv6) with FRR ospf6d -- GATED.

Intent (spec-ospf-ext-8): Ze's OSPFv3 family elects a DR over a static neighbor
list, originates the 0x2002 Network-LSA + Link-LSA, and floods unicast (link-local).

GATE: FRR ospf6d does NOT implement an OSPFv3 non-broadcast (NBMA) network type
(there is no `ipv6 ospf6 network non-broadcast`), so a true NBMA-to-NBMA adjacency
with FRR cannot be established. Per spec-ospf-ext-8's interop note, this scenario is
gated: it documents the peer limitation and defers to the unit tests, which fully
cover the v6 NBMA wire behavior (TestOSPFv3NBMAElection, TestOSPFv3NBMANetworkLSA,
TestOSPFv3NBMALinkLSA, TestOSPFv3NBMAUnicastHello, TestOSPFv3NBMAFloodUnicast).

Re-enable the adjacency assertions here once an ospf6d build ships OSPFv3 NBMA.

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, log_info, log_pass  # noqa: E402


def check():
    frr = FRROSPF6()
    ver = frr._vtysh_quiet("show version")
    if "ospf6" not in ver.lower() and "FRR" not in ver:
        log_info("FRR version unavailable; proceeding with the documented gate")
    # ospf6d has no NBMA network type: do not assert an adjacency. The Ze daemon must
    # still be running (its NBMA v6 config booted the OSPFv3 engine).
    log_info(
        "FRR ospf6d does not implement OSPFv3 NBMA; scenario gated per spec-ospf-ext-8"
    )
    log_pass(
        "ospfv3-nbma-frr: gated (FRR ospf6d lacks OSPFv3 NBMA; v6 NBMA covered by unit tests)"
    )


if __name__ == "__main__":
    check()
