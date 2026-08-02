#!/usr/bin/env python3
"""Scenario ospf-stub-nssa-frr: NSSA option match + mandatory ABR default with FRR.

Validates that Ze and FRR form an NSSA adjacency only when their N-bit options
agree. Ze is the NSSA border router and originates a Type 7 default without
`default-originate`, so FRR installs 0.0.0.0/0 (RFC 3101 Section 2.4). Type 5
AS-external LSAs stay out of the NSSA.

Prevents an NSSA option mismatch or an operator-gated border-router default.

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
    # RFC requirement: RFC3101-2.4-5 positive -- the NSSA border router
    # originates a default into every directly attached NSSA without a config gate.
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
