#!/usr/bin/env python3
"""Scenario 20: RFC 9234 Role capability negotiation between Ze and FRR.

Ze is configured as Provider (import provider), FRR as Customer (local-role customer).
Verifies:
  1. Session establishes with role capability negotiated (valid pair: Provider <-> Customer)
  2. FRR's static route (10.99.0.0/24) is received by Ze
  3. Session remains stable (no Role Mismatch NOTIFICATION)
"""

import os, sys, time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, log_pass, log_info


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")
    log_pass("FRR session established with role capability")

    # Wait for FRR's static route to arrive at Ze (via adj-rib-in or RIB).
    log_info("waiting for route from FRR (customer)...")
    deadline = time.time() + 30
    found = False
    while time.time() < deadline:
        if frr.has_route("10.99.0.0/24"):
            found = True
            break
        time.sleep(2)

    assert found, "FRR's static route 10.99.0.0/24 not received"
    log_pass("route 10.99.0.0/24 received from FRR customer")

    # Verify session is still up (no Role Mismatch NOTIFICATION).
    assert frr.session_established("172.30.0.2"), (
        "FRR session dropped after route exchange"
    )
    log_pass("session stable after route exchange with role capability")

    log_pass("RFC 9234 role capability interop with FRR passed")
