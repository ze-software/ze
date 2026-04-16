#!/usr/bin/env python3
"""Scenario 05: FRR announces routes, Ze receives.

Validates: Ze can receive IPv4 unicast UPDATE messages from FRR without error.
Prevents:  UPDATE parsing failures when receiving real routes from FRR.

FRR originates 3 static routes and redistributes them into BGP.
We verify the session stays Established after route exchange (Ze does not
send NOTIFICATION due to parse errors).
"""

import sys, os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, Ze, ZE_IP, log_info, log_pass, log_fail

import time


def check():
    frr = FRR()
    ze = Ze()

    # Wait for session to establish.
    frr.wait_session(ZE_IP)

    # Wait for FRR to redistribute static routes and send them to Ze.
    # FRR's redistribute is asynchronous -- poll instead of using a fixed sleep.
    log_info("waiting for FRR to send routes to Ze...")
    deadline = time.time() + 30
    sent = 0
    while time.time() < deadline:
        sent = frr.route_count(ZE_IP)
        if sent >= 3:
            break
        time.sleep(2)

    if sent >= 3:
        log_pass("FRR sent %d prefixes to Ze" % sent)
    else:
        log_fail("FRR sent %d prefixes to Ze (expected >= 3)" % sent)
        raise AssertionError("FRR sent %d prefixes, expected >= 3" % sent)

    # Verify FRR's BGP table has the expected static routes.
    frr.check_route("10.0.0.0/24")
    frr.check_route("10.0.1.0/24")
    frr.check_route("10.0.2.0/24")

    # Verify Ze's RIB received the routes.
    ze.rib_received(3)

    # Verify session is still Established after route exchange.
    assert frr.session_established(ZE_IP), (
        "session dropped after route exchange (Ze may have sent NOTIFICATION)"
    )
