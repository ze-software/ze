#!/usr/bin/env python3
"""Scenario 07: Ze announces routes, FRR receives.

Validates: Ze can send valid UPDATE messages that FRR accepts.
Prevents:  Wire encoding bugs producing UPDATEs that real implementations reject.

Ze runs a process plugin that announces 3 prefixes via JSON RPC.
We verify FRR receives them in its BGP table.
"""

import sys, os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, Ze, ZE_IP, log_info, log_fail

import time


def check():
    frr = FRR()
    ze = Ze()

    # Wait for session to establish.
    frr.wait_session(ZE_IP)

    # Wait for routes to propagate (process plugin needs peer-up event + FRR processing).
    log_info("waiting for Ze to announce routes to FRR...")
    deadline = time.time() + 30
    received = False
    while time.time() < deadline:
        if frr.has_route("10.10.0.0/24"):
            received = True
            break
        time.sleep(2)

    if not received:
        log_fail("FRR did not receive 10.10.0.0/24 from Ze within 30s")
        print(ze.logs(20))
        raise AssertionError("FRR did not receive 10.10.0.0/24 from Ze")

    # Verify all 3 prefixes arrived.
    frr.check_route("10.10.0.0/24")
    frr.check_route("10.10.1.0/24")
    frr.check_route("10.10.2.0/24")
