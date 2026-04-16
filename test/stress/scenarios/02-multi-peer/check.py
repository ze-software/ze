#!/usr/bin/env python3
"""Scenario 02: Mixed IPv4 + IPv6 route injection.

Validates: Ze can handle mixed address-family updates. Spawns an IPv4
inject peer first, waits for it to complete, then an IPv6 inject peer.
Each peer is its own BGP session (ze-test peer negotiates one family
per run; the per-session model makes the scenario stricter -- ze has to
cleanly accept the IPv6 family after the IPv4 session exits).

Note: this differs from the old BNG-Blaster flow which kept a single
multi-family session. If you need multi-family in one session, extend
ze-test peer to accept a second inject spec and stream both in order.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from harness import (
    BB_IP,
    Timer,
    log_info,
    log_pass,
    start_peer_inject,
    wait_peer_done,
)


IPV4_COUNT = 500_000
IPV6_COUNT = 250_000


def check():
    log_info("")
    log_info("=== IPv4: %d prefixes ===" % IPV4_COUNT)
    with Timer("ipv4 inject") as t4:
        peer = start_peer_inject(
            prefix_base="10.0.0.0/24",
            prefix_count=IPV4_COUNT,
            nexthop=BB_IP,
            asn=65100,
            dwell="30s",
        )
        wait_peer_done(peer, timeout=900)

    log_info("")
    log_info("=== IPv6: %d prefixes ===" % IPV6_COUNT)
    with Timer("ipv6 inject") as t6:
        peer = start_peer_inject(
            prefix_base="2001:db8::/48",
            prefix_count=IPV6_COUNT,
            nexthop="2001:db8::3",
            asn=65100,
            dwell="30s",
        )
        wait_peer_done(peer, timeout=600)

    total = IPV4_COUNT + IPV6_COUNT
    elapsed = t4.elapsed + t6.elapsed
    rate = total / elapsed if elapsed > 0 else 0
    log_pass(
        "mixed injection: %d IPv4 + %d IPv6 = %d total (%.0f routes/s)"
        % (IPV4_COUNT, IPV6_COUNT, total, rate)
    )
