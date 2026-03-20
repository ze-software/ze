#!/usr/bin/env python3
"""Scenario 16: Extended communities accepted by FRR."""

import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, docker_exec_quiet, log_pass, log_fail


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")
    frr.wait_route("10.10.0.0/24")
    frr.check_route("10.10.0.0/24")

    # Verify extended community. FRR shows it as "RT:65001:100" in the route detail.
    output = docker_exec_quiet(frr.container, ["vtysh", "-c", "show bgp ipv4 unicast 10.10.0.0/24"])
    if "65001:100" in output:
        log_pass("FRR route has extended community target:65001:100")
    else:
        log_fail("extended community not found in FRR route output")
        print("  %s" % output[:500])
        raise AssertionError("extended community missing")

    assert frr.session_established("172.30.0.2"), "session dropped"
    log_pass("extended community round-tripped correctly")
