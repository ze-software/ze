#!/usr/bin/env python3
"""Scenario 19: Bidirectional route exchange with GoBGP.

Validates: Ze can send and receive routes with GoBGP.
Prevents:  UPDATE encoding/decoding issues specific to GoBGP's implementation.

Ze announces 2 routes via process plugin. GoBGP injects 2 routes via CLI.
Both sides verify receipt.
"""

import time
import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import GoBGP, Ze, ZE_IP, log_pass, log_info


def check():
    gobgp = GoBGP()
    ze = Ze()

    gobgp.wait_session(ZE_IP)

    # Wait for Ze's routes to arrive at GoBGP.
    gobgp.wait_route("10.10.0.0/24")
    gobgp.check_route("10.10.0.0/24")
    gobgp.check_route("10.10.1.0/24")

    # Verify route attributes via JSON on at least one route.
    data = gobgp.route_json("10.10.0.0/24")
    if isinstance(data, list) and data:
        paths = data[0].get("paths", [])
        if paths:
            path = paths[0]
            # GoBGP JSON includes attrs with origin, as_path, nexthop, etc.
            attrs = path.get("attrs", [])
            attr_types = [a.get("type", 0) for a in attrs]
            # Type 1 = ORIGIN, Type 2 = AS_PATH, Type 3 = NEXT_HOP
            assert 1 in attr_types, "ORIGIN attribute missing from GoBGP route"
            assert 3 in attr_types or any("nexthop" in str(a).lower() for a in attrs), \
                "NEXT_HOP attribute missing from GoBGP route"
            log_pass("GoBGP route has expected attributes (ORIGIN, NEXT_HOP)")

    log_pass("GoBGP received Ze's routes")

    # Inject routes from GoBGP side (raises on failure).
    log_info("injecting routes from GoBGP...")
    gobgp.inject_route("10.20.0.0/24")
    gobgp.inject_route("10.20.1.0/24")

    # Poll Ze's RIB instead of fixed sleep.
    deadline = time.time() + 30
    while time.time() < deadline:
        try:
            ze.rib_received(2)
            break
        except AssertionError:
            time.sleep(2)
    else:
        ze.rib_received(2)  # Final attempt, let it raise.

    log_pass("Ze received GoBGP's routes")

    assert gobgp.session_established(ZE_IP), "session dropped after route exchange"
    log_pass("bidirectional route exchange with GoBGP successful")
