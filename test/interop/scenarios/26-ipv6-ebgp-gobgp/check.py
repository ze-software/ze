#!/usr/bin/env python3
"""Scenario 26: IPv6 eBGP route exchange with GoBGP via MP_REACH_NLRI.

Validates: Ze can send IPv6 UPDATE messages that GoBGP accepts.
Prevents:  MP_REACH encoding bugs specific to GoBGP's implementation.
"""

import os, sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import GoBGP, ZE_IP, log_pass


def check():
    gobgp = GoBGP()

    gobgp.wait_session(ZE_IP)

    # Wait for IPv6 routes to arrive.
    gobgp.wait_route("2001:db8:1::/48", family="ipv6 unicast")

    gobgp.check_route("2001:db8:1::/48", family="ipv6 unicast")
    gobgp.check_route("2001:db8:2::/48", family="ipv6 unicast")
    gobgp.check_route("2001:db8:3::/48", family="ipv6 unicast")

    assert gobgp.session_established(ZE_IP), "session dropped after IPv6 route exchange"
    log_pass("IPv6 routes received by GoBGP, session stable")
