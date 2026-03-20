#!/usr/bin/env python3
"""Scenario 15: Standard and large communities accepted by FRR."""

import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, log_pass


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")

    # Wait for routes with communities.
    frr.wait_route("10.10.0.0/24")
    frr.check_route("10.10.0.0/24")
    frr.check_route("10.10.1.0/24")

    # Verify standard communities on first route.
    frr.check_route_community("10.10.0.0/24", "65001:100")
    frr.check_route_community("10.10.0.0/24", "65001:200")

    # Verify large community on second route.
    # check_route_community handles both colon and FRR's (N,N,N) format.
    frr.check_route_community("10.10.1.0/24", "65001:0:1")


    assert frr.session_established("172.30.0.2"), "session dropped"
    log_pass("communities round-tripped correctly")
