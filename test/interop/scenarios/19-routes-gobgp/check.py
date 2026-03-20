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
    log_pass("GoBGP received Ze's routes")

    # Inject routes from GoBGP side.
    log_info("injecting routes from GoBGP...")
    gobgp.inject_route("10.20.0.0/24")
    gobgp.inject_route("10.20.1.0/24")
    time.sleep(3)

    # Verify Ze received GoBGP's routes.
    ze.rib_received(2)
    log_pass("Ze received GoBGP's routes")

    assert gobgp.session_established(ZE_IP), "session dropped after route exchange"
    log_pass("bidirectional route exchange with GoBGP successful")
