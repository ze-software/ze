#!/usr/bin/env python3
"""Scenario 04: Bulk IPv4 route injection -- BIRD baseline.

Same shape as 01-bulk-ipv4 but BIRD 2.x is the DUT instead of ze. Each
round spawns a fresh `ze-test peer --mode inject` in bb-ns which dials
BIRD; BIRD's RIB is polled via birdc to confirm each round's count.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from harness import (
    BB_IP,
    BIRD,
    Timer,
    log_info,
    log_pass,
    start_peer_inject,
    wait_peer_done,
)


ROUNDS = [
    (100_000, "10.0.0.0/24", 120),
    (250_000, "10.64.0.0/24", 180),
    (500_000, "10.128.0.0/24", 300),
    (1_000_000, "11.0.0.0/24", 600),
]


def check():
    bird = BIRD()

    for prefix_count, prefix_base, timeout in ROUNDS:
        log_info("")
        log_info("=== Round: %d prefixes ===" % prefix_count)

        with Timer("round %dk" % (prefix_count // 1_000)) as t_round:
            peer = start_peer_inject(
                prefix_base=prefix_base,
                prefix_count=prefix_count,
                nexthop=BB_IP,
                asn=65100,
                dwell="30s",
            )
            wait_peer_done(peer, timeout=timeout)

        with Timer("bird ingest") as t_ingest:
            bird.wait_route_count(prefix_count, timeout=min(timeout, 120))

        rate = prefix_count / t_round.elapsed if t_round.elapsed > 0 else 0
        log_pass(
            "%d prefixes: inject %.2fs, bird ingest %.2fs (%.0f routes/s)"
            % (prefix_count, t_round.elapsed, t_ingest.elapsed, rate)
        )

    log_pass("all bulk injection rounds completed (BIRD baseline)")
