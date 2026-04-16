#!/usr/bin/env python3
"""Scenario 01: Bulk IPv4 route injection.

Validates: Ze can receive and process large numbers of IPv4 unicast routes
           without errors. Runs four rounds with increasing prefix counts
           (100k, 250k, 500k, 1M) and reports ingestion throughput.

Each round spawns a fresh `ze-test peer --mode inject` in bb-ns. Different
from the previous BNG-Blaster pipeline, each round is its own BGP session:
the peer dials ze, streams the image, dwells briefly, exits. ze reconnects
on the next round (the peer comes back up with a new listener).
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


# (prefix_count, prefix_base, peer_timeout_s)
ROUNDS = [
    (100_000, "10.0.0.0/24", 120),
    (250_000, "10.64.0.0/24", 180),
    (500_000, "10.128.0.0/24", 300),
    (1_000_000, "11.0.0.0/24", 600),
]


def check():
    for prefix_count, prefix_base, timeout in ROUNDS:
        log_info("")
        log_info("=== Round: %d prefixes ===" % prefix_count)

        with Timer("round %dk" % (prefix_count // 1_000)) as t:
            peer = start_peer_inject(
                prefix_base=prefix_base,
                prefix_count=prefix_count,
                nexthop=BB_IP,
                asn=65100,
                dwell="15s",
            )
            metrics = wait_peer_done(peer, timeout=timeout)

        rate = prefix_count / t.elapsed if t.elapsed > 0 else 0
        if metrics and "mbps" in metrics:
            log_pass(
                "%d prefixes in %.2fs (%.0f routes/s, wire %.1f MB/s)"
                % (prefix_count, t.elapsed, rate, metrics["mbps"])
            )
        else:
            log_pass(
                "%d prefixes in %.2fs (%.0f routes/s)" % (prefix_count, t.elapsed, rate)
            )

    log_pass("all bulk injection rounds completed")
