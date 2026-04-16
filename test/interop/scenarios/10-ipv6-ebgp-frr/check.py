#!/usr/bin/env python3
"""Scenario 10: IPv6 eBGP route exchange with FRR via MP_REACH_NLRI."""

import os, sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, log_pass


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")

    # Wait for IPv6 routes to arrive.
    frr.wait_route("2001:db8:1::/48", family="ipv6 unicast")

    frr.check_route("2001:db8:1::/48", family="ipv6 unicast")
    frr.check_route("2001:db8:2::/48", family="ipv6 unicast")
    frr.check_route("2001:db8:3::/48", family="ipv6 unicast")

    assert frr.session_established("172.30.0.2"), (
        "session dropped after IPv6 route exchange"
    )
    log_pass("IPv6 routes received by FRR, session stable")
