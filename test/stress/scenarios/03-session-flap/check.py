#!/usr/bin/env python3
"""Scenario 03: Session flap under load.

Validates: Ze handles repeated BGP session disconnect/reconnect cycles
           gracefully. Routes are injected, session is torn down, reconnected,
           and routes re-injected. Ze must not crash, leak memory, or fail to
           re-establish the session.
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


PREFIX_COUNT = 100_000
FLAP_CYCLES = 10


def check():
    bb = BNGBlaster()
    ze = Ze()

    # Wait for initial session.
    bb.wait_session_established()

    # Generate update file once.
    update_file = generate_updates(
        prefix_base="10.0.0.0/24",
        prefix_count=PREFIX_COUNT,
        nexthop=BB_IP,
        asn=65100,
        filename="flap.bgp",
    )

    for cycle in range(1, FLAP_CYCLES + 1):
        log_info("")
        log_info("=== Flap cycle %d/%d ===" % (cycle, FLAP_CYCLES))

        # Inject routes.
        with Timer("inject cycle %d" % cycle):
            bb.bgp_raw_update(update_file)
            bb.wait_raw_update_done(timeout=300)

        # Verify session is established.
        sessions = bb.bgp_sessions()
        established = False
        for s in sessions.get("bgp-sessions", []):
            if s.get("state") == "established":
                established = True
                break
        if not established:
            log_fail("session not established before flap cycle %d" % cycle)
            raise AssertionError("session not established before flap")

        # Disconnect.
        log_info("disconnecting BGP session...")
        bb.bgp_disconnect()
        import time

        time.sleep(2)

        # BNG Blaster reconnects automatically (reconnect: true).
        with Timer("reconnect cycle %d" % cycle):
            bb.wait_session_established(timeout=60)

        log_pass("flap cycle %d complete" % cycle)

    # Final injection to verify everything still works.
    log_info("")
    log_info("=== Final verification ===")
    with Timer("final inject"):
        bb.bgp_raw_update(update_file)
        bb.wait_raw_update_done(timeout=300)

    ze.wait_rib_count(PREFIX_COUNT, timeout=300)
    log_pass("all %d flap cycles completed, Ze stable" % FLAP_CYCLES)
