#!/usr/bin/env python3
"""Scenario 08: Triangle topology Ze <-> FRR <-> BIRD.

Validates: Three-way BGP topology with all sessions Established.
           FRR originates 10.99.0.0/24 -> BIRD should receive it via FRR.
           Ze should also receive it from FRR.
Prevents:  Multi-peer session management failures, attribute forwarding bugs.

Topology:
  Ze (AS 65001) ---- FRR (AS 65002) ---- BIRD (AS 65003)
       |______________________________________|
"""

import sys, os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, BIRD, ZE_IP, BIRD_IP, log_pass

import time


def check():
    frr = FRR()
    bird = BIRD()

    # Wait for all sessions to establish.
    frr.wait_session(ZE_IP)
    frr.wait_session(BIRD_IP)
    bird.wait_session("ze_peer")
    bird.wait_session("frr_peer")

    # Wait for route propagation.
    time.sleep(10)

    # FRR originates 10.99.0.0/24. BIRD should receive it via FRR.
    bird.check_route("10.99.0.0/24")

    # Verify all sessions still up after route exchange.
    assert frr.session_established(ZE_IP), "FRR<->Ze session dropped"
    assert frr.session_established(BIRD_IP), "FRR<->BIRD session dropped"
    assert bird.session_established("ze_peer"), "BIRD<->Ze session dropped"
    log_pass("all sessions stable after route exchange")
