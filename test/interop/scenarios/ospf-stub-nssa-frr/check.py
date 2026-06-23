#!/usr/bin/env python3
"""Scenario ospf-stub-nssa-frr: NSSA option match + ABR default with FRR.

Validates (spec-ospf-13 AC-18): Ze (the NSSA ABR) and FRR (an NSSA internal
router) form an adjacency only when the NSSA N-bit Hello options agree, and Ze
originates a Type 7 default route into the NSSA so FRR installs 0.0.0.0/0
(RFC 3101). Type 5 AS-external LSAs are suppressed inside the NSSA.
Prevents: an NSSA option mismatch that blocks the adjacency, or a missing
ABR-originated default so the NSSA internal router has no way out.

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, log_info, log_pass  # noqa: E402


def check():
    frr = FRROSPF()

    # 1. The NSSA adjacency forms only if the N-bit options match (RFC 3101).
    log_info("waiting for the NSSA OSPF adjacency (N-bit option match)...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR (NSSA internal) must install the Type 7 default route the ABR (Ze)
    #    originates into the NSSA.
    log_info("waiting for the NSSA default route (0.0.0.0/0) from the ABR...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if frr.has_ospf_route("0.0.0.0/0"):
            break
        time.sleep(2)
    else:
        raise AssertionError(
            "FRR did not install the NSSA ABR default route (0.0.0.0/0)"
        )

    # 3. No AS-external (Type 5) LSAs are flooded into the NSSA.
    if frr.has_external_lsa():
        raise AssertionError("Type 5 AS-external LSA leaked into the NSSA")

    log_pass(
        "ospf-stub-nssa-frr: NSSA adjacency formed and ABR default installed, no Type 5 leak"
    )


if __name__ == "__main__":
    check()
