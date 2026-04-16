#!/usr/bin/env python3
"""Scenario 02: Mixed IPv4 + IPv6 route injection.

Validates: Ze can handle mixed address family updates from BNG Blaster.
           Tests both IPv4 and IPv6 unicast in the same session.
"""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from bngblaster import (
    BNGBlaster,
    Ze,
    Timer,
    generate_updates,
    BB_IP,
    log_info,
    log_pass,
    log_fail,
)


IPV4_COUNT = 500_000
IPV6_COUNT = 250_000


def check():
    bb = BNGBlaster()
    ze = Ze()

    bb.wait_session_established()

    # Generate IPv4 updates.
    ipv4_file = generate_updates(
        prefix_base="10.0.0.0/24",
        prefix_count=IPV4_COUNT,
        nexthop=BB_IP,
        asn=65100,
        filename="ipv4.bgp",
    )

    # Generate IPv6 updates.
    ipv6_file = generate_updates(
        prefix_base="2001:db8::/48",
        prefix_count=IPV6_COUNT,
        nexthop="2001:db8::3",
        asn=65100,
        filename="ipv6.bgp",
    )

    # Inject IPv4 first.
    log_info("injecting %d IPv4 prefixes..." % IPV4_COUNT)
    with Timer("ipv4 injection") as t4:
        bb.bgp_raw_update(ipv4_file)
        bb.wait_raw_update_done(timeout=900)

    # Then IPv6.
    log_info("injecting %d IPv6 prefixes..." % IPV6_COUNT)
    with Timer("ipv6 injection") as t6:
        bb.bgp_raw_update(ipv6_file)
        bb.wait_raw_update_done(timeout=600)

    # Wait for Ze to process all routes.
    total = IPV4_COUNT + IPV6_COUNT
    with Timer("ze processing") as tp:
        ze.wait_rib_count(total, timeout=1200)

    # Verify session stability.
    sessions = bb.bgp_sessions()
    for s in sessions.get("bgp-sessions", []):
        if s.get("state") != "established":
            log_fail("session dropped during mixed injection")
            raise AssertionError("session dropped")

    rate = total / tp.elapsed if tp.elapsed > 0 else 0
    log_pass(
        "mixed injection: %d IPv4 + %d IPv6 = %d total (%.0f routes/s)"
        % (IPV4_COUNT, IPV6_COUNT, total, rate)
    )
