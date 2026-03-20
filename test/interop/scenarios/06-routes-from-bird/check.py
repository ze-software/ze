#!/usr/bin/env python3
"""Scenario 06: BIRD announces routes, Ze receives.

Validates: Ze can receive IPv4 unicast UPDATE messages from BIRD without error.
Prevents:  UPDATE parsing failures when receiving real routes from BIRD.

BIRD originates 3 static routes and exports them via BGP.
We verify the session stays Established after route exchange.
"""

import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD, Ze, log_info, log_pass, log_fail

import time


def check():
    bird = BIRD()
    ze = Ze()

    # Wait for session to establish.
    bird.wait_session("ze_peer")

    # Wait for BIRD to export static routes to Ze.
    log_info("waiting for BIRD to export routes to Ze...")
    deadline = time.time() + 30
    exported = 0
    while time.time() < deadline:
        exported = bird.exported_count("ze_peer")
        if exported >= 3:
            break
        time.sleep(2)

    if exported >= 3:
        log_pass("BIRD exported %d routes to Ze" % exported)
    else:
        log_fail("BIRD exported %d routes to Ze (expected >= 3)" % exported)
        raise AssertionError("BIRD exported %d routes, expected >= 3" % exported)

    # Verify BIRD's routing table has the expected static routes.
    bird.check_route("10.0.0.0/24")
    bird.check_route("10.0.1.0/24")
    bird.check_route("10.0.2.0/24")

    # Verify Ze's RIB received the routes.
    ze.rib_received(3)

    # Verify session is still Established after route exchange.
    assert bird.session_established("ze_peer"), \
        "session dropped after route exchange (Ze may have sent NOTIFICATION)"
