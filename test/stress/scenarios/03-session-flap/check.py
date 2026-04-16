#!/usr/bin/env python3
"""Scenario 03: Session flap under load.

Validates: Ze handles repeated BGP session disconnect/reconnect cycles
           gracefully. For each cycle: spawn ze-test peer, let it stream a
           batch of prefixes, exit. The peer exiting tears the session
           down; ze's passive listener accepts the next dial. Across
           FLAP_CYCLES iterations, ze must not crash, leak, or refuse a
           later dial.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from harness import (
    BB_IP,
    Timer,
    log_info,
    log_pass,
    start_peer_inject,
    wait_peer_done,
)


PREFIX_COUNT = 100_000
FLAP_CYCLES = 10


def check():
    for cycle in range(1, FLAP_CYCLES + 1):
        log_info("")
        log_info("=== Flap cycle %d/%d ===" % (cycle, FLAP_CYCLES))

        with Timer("cycle %d" % cycle):
            peer = start_peer_inject(
                prefix_base="10.0.0.0/24",
                prefix_count=PREFIX_COUNT,
                nexthop=BB_IP,
                asn=65100,
                # Short dwell -- the peer exit IS the flap in this scenario.
                dwell="2s",
            )
            wait_peer_done(peer, timeout=180)

        # Brief pause so ze observes the teardown before the next cycle.
        time.sleep(2)
        log_pass("flap cycle %d complete" % cycle)

    # Final verification: one more injection should still succeed.
    log_info("")
    log_info("=== Final verification ===")
    with Timer("final inject"):
        peer = start_peer_inject(
            prefix_base="10.0.0.0/24",
            prefix_count=PREFIX_COUNT,
            nexthop=BB_IP,
            asn=65100,
            dwell="5s",
        )
        wait_peer_done(peer, timeout=180)

    log_pass("%d flap cycles completed, Ze remained responsive" % FLAP_CYCLES)
