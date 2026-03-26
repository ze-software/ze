#!/usr/bin/env python3
"""Scenario 25: IPv6 eBGP route exchange with BIRD via MP_REACH_NLRI.

Validates: Ze can send IPv6 UPDATE messages that BIRD accepts.
Prevents:  MP_REACH encoding bugs specific to BIRD's implementation.
"""

import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD, log_pass


def check():
    bird = BIRD()

    bird.wait_session("ze_peer")

    # Wait for IPv6 routes to arrive.
    bird.wait_route("2001:db8:1::/48")

    bird.check_route("2001:db8:1::/48")
    bird.check_route("2001:db8:2::/48")
    bird.check_route("2001:db8:3::/48")

    assert bird.session_established("ze_peer"), "session dropped after IPv6 route exchange"
    log_pass("IPv6 routes received by BIRD, session stable")
