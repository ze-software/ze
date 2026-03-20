#!/usr/bin/env python3
"""Scenario 14: Ze as route server -- routes forwarded without Ze's ASN."""

import os, sys, time
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, BIRD, log_pass, log_info


def check():
    frr = FRR()
    bird = BIRD()

    frr.wait_session("172.30.0.2")
    bird.wait_session("ze_peer")

    # Wait for FRR's route to propagate through Ze to BIRD.
    log_info("waiting for route to propagate FRR -> Ze -> BIRD...")
    deadline = time.time() + 30
    while time.time() < deadline:
        if bird.has_route("10.99.0.0/24"):
            break
        time.sleep(2)

    bird.check_route("10.99.0.0/24")

    # Verify Ze's ASN (65001) is NOT in the AS_PATH (route server behavior).
    bird.check_route_no_as("10.99.0.0/24", "65001")

    assert frr.session_established("172.30.0.2"), "FRR session dropped"
    log_pass("FRR session stable")
    assert bird.session_established("ze_peer"), "BIRD session dropped"
    log_pass("BIRD session stable")

    log_pass("route server forwarded route without inserting own ASN")
