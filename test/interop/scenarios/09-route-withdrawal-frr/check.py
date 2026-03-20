#!/usr/bin/env python3
"""Scenario 09: Ze announces 3 routes, then withdraws 1 -- FRR removes it."""

import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, log_pass


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")

    # Wait for all 3 routes.
    frr.wait_route("10.10.0.0/24")
    frr.wait_route("10.10.1.0/24")
    frr.wait_route("10.10.2.0/24")

    frr.check_route("10.10.0.0/24")
    frr.check_route("10.10.1.0/24")
    frr.check_route("10.10.2.0/24")
    log_pass("all 3 routes present before withdrawal")

    # Wait for withdrawal (plugin withdraws after 5s delay).
    frr.wait_route_absent("10.10.1.0/24", timeout=30)

    frr.route_absent("10.10.1.0/24")
    frr.check_route("10.10.0.0/24")
    frr.check_route("10.10.2.0/24")

    assert frr.session_established("172.30.0.2"), "session dropped after withdrawal"
    log_pass("withdrawal correct: 10.10.1.0/24 removed, others intact, session stable")
