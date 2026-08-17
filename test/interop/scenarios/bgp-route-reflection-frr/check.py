#!/usr/bin/env python3
"""Scenario 38: Ze reflects an iBGP route from FRR to another RR client.

VALIDATES: AC-1 route reflection interop includes ORIGINATOR_ID and CLUSTER_LIST.
PREVENTS: RR forwarding working only inside Ze without real peer attribute evidence.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD, FRR, ZE_IP, log_fail, log_pass


def check():
    frr = FRR()
    bird = BIRD()

    frr.wait_session(ZE_IP)
    bird.wait_session("ze_rr")

    bird.wait_route("10.38.0.0/24")
    bird.check_route("10.38.0.0/24")

    output = bird._birdc_quiet("show route for 10.38.0.0/24 all")
    lower = output.lower()
    if "originator" not in lower or "cluster" not in lower:
        log_fail("BIRD route is missing ORIGINATOR_ID or CLUSTER_LIST")
        print(output)
        raise AssertionError("route reflection attributes missing")

    assert frr.session_established(ZE_IP), "FRR RR source session dropped"
    assert bird.session_established("ze_rr"), "BIRD RR client session dropped"
    log_pass("route reflected with ORIGINATOR_ID and CLUSTER_LIST")
