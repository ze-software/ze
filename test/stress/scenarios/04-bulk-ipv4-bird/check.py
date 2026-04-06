#!/usr/bin/env python3
"""Scenario 04: Bulk IPv4 route injection -- BIRD baseline.

Same test as 01-bulk-ipv4 but with BIRD 2.x as the DUT instead of Ze.
Provides a performance baseline for comparison.

Validates: BIRD can receive and process large numbers of IPv4 unicast routes
           from BNG Blaster without errors or session drops.
"""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from bngblaster import (
    BNGBlaster, BIRD, Timer,
    generate_updates, BB_IP,
    log_info, log_pass, log_fail,
)


ROUNDS = [
    (100_000,   "10.0.0.0/24",    300),
    (250_000,   "10.64.0.0/24",   600),
    (500_000,   "10.128.0.0/24",  900),
    (1_000_000, "11.0.0.0/24",   1800),
]


def check():
    bb = BNGBlaster()
    bird = BIRD()

    # Wait for initial BGP session.
    bb.wait_session_established()

    for prefix_count, prefix_base, timeout in ROUNDS:
        log_info("")
        log_info("=== Round: %d prefixes ===" % prefix_count)

        # Generate update file.
        update_file = generate_updates(
            prefix_base=prefix_base,
            prefix_count=prefix_count,
            nexthop=BB_IP,
            asn=65100,
            filename="round-%dk.bgp" % (prefix_count // 1_000),
        )

        # Inject updates via control socket.
        with Timer("injection") as t_inject:
            bb.bgp_raw_update(update_file)
            bb.wait_raw_update_done(timeout=timeout)

        # Wait for BIRD to finish processing routes.
        with Timer("bird processing") as t_process:
            bird.wait_route_count(prefix_count, timeout=min(timeout, 120))

        # Verify session is still established.
        sessions = bb.bgp_sessions()
        for s in sessions.get("bgp-sessions", []):
            if s.get("state") != "established":
                log_fail("session dropped during %d-prefix injection" % prefix_count)
                raise AssertionError("session dropped")

        rate = prefix_count / t_inject.elapsed if t_inject.elapsed > 0 else 0
        log_pass("%d prefixes: injection %.2fs, processed %.2fs (%.0f routes/s)"
                 % (prefix_count, t_inject.elapsed, t_process.elapsed, rate))

    log_pass("all bulk injection rounds completed (BIRD baseline)")
