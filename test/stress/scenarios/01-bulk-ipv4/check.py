#!/usr/bin/env python3
"""Scenario 01: Bulk IPv4 route injection.

Validates: Ze can receive and process large numbers of IPv4 unicast routes
           from BNG Blaster without errors or session drops.

Runs four rounds with increasing prefix counts (100k, 250k, 500k, 1M) and
reports ingestion rates for each.
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


ROUNDS = [
    (100_000, "10.0.0.0/24", 300),
    (250_000, "10.64.0.0/24", 600),
    (500_000, "10.128.0.0/24", 900),
    (1_000_000, "11.0.0.0/24", 1800),
]


def check():
    bb = BNGBlaster()
    ze = Ze(bb)

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

        # Wait for Ze to finish processing.
        with Timer("ze processing") as t_process:
            ze.wait_settled(timeout=min(timeout, 60))

        # Verify session is still established.
        sessions = bb.bgp_sessions()
        for s in sessions.get("bgp-sessions", []):
            if s.get("state") != "established":
                log_fail("session dropped during %d-prefix injection" % prefix_count)
                raise AssertionError("session dropped")

        rate = prefix_count / t_inject.elapsed if t_inject.elapsed > 0 else 0
        log_pass(
            "%d prefixes: injection %.2fs, settled %.2fs (%.0f routes/s)"
            % (prefix_count, t_inject.elapsed, t_process.elapsed, rate)
        )

    log_pass("all bulk injection rounds completed")
