#!/usr/bin/env python3
"""Scenario 28: EVPN Type-2 route exchange with GoBGP via MP_REACH_NLRI.

Validates: Ze can send EVPN (l2vpn/evpn) UPDATE messages that GoBGP accepts.
Prevents:  EVPN NLRI encoding bugs specific to GoBGP's implementation.
"""

import json
import os, sys, time
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import GoBGP, Ze, ZE_IP, log_info, log_pass, log_fail


def check():
    gobgp = GoBGP()
    ze = Ze()

    gobgp.wait_session(ZE_IP)

    log_info("waiting for EVPN routes to arrive at GoBGP...")
    deadline = time.time() + 30
    raw_data = None
    while time.time() < deadline:
        data = gobgp._gobgp_json(["global", "rib", "-a", "evpn"])
        if data and (isinstance(data, list) and len(data) > 0 or isinstance(data, dict) and len(data) > 0):
            raw_data = data
            break
        time.sleep(2)

    if raw_data is None:
        log_fail("GoBGP did not receive EVPN routes within 30s")
        print(ze.logs(20))
        raise AssertionError("GoBGP did not receive EVPN routes")

    log_pass("EVPN routes received by GoBGP")

    routes_str = json.dumps(raw_data)
    assert "00:11:22:33:44:55" in routes_str, \
        "MAC 00:11:22:33:44:55 not found in EVPN routes"
    log_pass("EVPN Type-2 route with MAC 00:11:22:33:44:55 verified")

    assert gobgp.session_established(ZE_IP), "session dropped after EVPN exchange"
    log_pass("EVPN session with GoBGP stable")
