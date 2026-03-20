#!/usr/bin/env python3
"""Scenario 12: Route refresh handling -- FRR triggers soft-in clear."""

import os, sys, time
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, docker_exec_quiet, log_pass


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")

    # Wait for initial routes from Ze.
    frr.wait_route("10.10.0.0/24")
    frr.check_route("10.10.0.0/24")
    frr.check_route("10.10.1.0/24")
    frr.check_route("10.10.2.0/24")
    log_pass("initial routes received")

    # Trigger route refresh from FRR side.
    docker_exec_quiet(frr.container, ["vtysh", "-c", "clear bgp ipv4 unicast 172.30.0.2 soft in"])
    time.sleep(3)

    # Routes should still be present (re-sent after refresh).
    frr.check_route("10.10.0.0/24")
    frr.check_route("10.10.1.0/24")
    frr.check_route("10.10.2.0/24")

    assert frr.session_established("172.30.0.2"), "session dropped after route refresh"
    log_pass("route refresh handled correctly, all routes intact")
