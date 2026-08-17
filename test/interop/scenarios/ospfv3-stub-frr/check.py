#!/usr/bin/env python3
"""Scenario ospfv3-stub-frr: OSPFv3 stub ABR default + Type-5 suppression with FRR.

Validates (spec-ospfv3-6 AC-2): Ze (the OSPFv3 stub ABR) and FRR (a stub internal
router) form an adjacency only when the stub E-bit Hello options agree, and Ze
originates a single Inter-Area-Prefix default (::/0) into the stub so FRR installs
::/0 (RFC 5340 sec 3.5 / RFC 2328 sec 3.6). No AS-External (Type 0x4005) LSA is
flooded into the stub.
Prevents: a v6 stub area whose internal routers have no way out because the ABR
never originated the default (the OSPFv3 origination gap this spec closed), or a
stub-option mismatch that blocks the adjacency.

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over
ff02::5 + FRR ospf6d); it CANNOT run on darwin.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, log_info, log_pass  # noqa: E402


def check():
    frr = FRROSPF6()

    # 1. The stub OSPFv3 adjacency forms only if the stub E-bit Hello options match.
    log_info("waiting for the OSPFv3 stub adjacency (E-bit option match)...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR (stub internal) must install the ::/0 default the ABR (Ze) originates as an
    #    Inter-Area-Prefix-LSA into the stub.
    log_info("waiting for the stub default route (::/0) from the ABR...")
    frr.wait_ospf6_route("::/0", timeout=90)

    # 3. No AS-External (Type 0x4005) LSA is flooded into the stub area.
    if frr.has_as_external_lsa():
        print(frr._vtysh_quiet("show ipv6 ospf6 database as-external")[:800])
        raise AssertionError("Type-5 AS-External-LSA leaked into the v6 stub area")

    log_pass(
        "ospfv3-stub-frr: OSPFv3 stub adjacency formed and ABR default (::/0) "
        "installed, no Type-5 leak"
    )


if __name__ == "__main__":
    check()
