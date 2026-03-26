#!/usr/bin/env python3
"""Scenario 29: VPN (L3VPN) route exchange with GoBGP via MP_REACH_NLRI.

Validates: Ze can send VPNv4 (ipv4/mpls-vpn) UPDATE messages that GoBGP accepts.
Prevents:  VPN NLRI encoding bugs specific to GoBGP's implementation.
"""

import json
import os, sys, time
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import GoBGP, Ze, ZE_IP, log_info, log_pass, log_fail


def check():
    gobgp = GoBGP()
    ze = Ze()

    gobgp.wait_session(ZE_IP)

    log_info("waiting for VPN routes to arrive at GoBGP...")
    deadline = time.time() + 30
    raw_data = None
    while time.time() < deadline:
        data = gobgp._gobgp_json(["global", "rib", "-a", "vpnv4"])
        if data and isinstance(data, list) and len(data) > 0:
            raw_data = data
            break
        time.sleep(2)

    if raw_data is None:
        log_fail("GoBGP did not receive VPN routes within 30s")
        print(ze.logs(20))
        raise AssertionError("GoBGP did not receive VPN routes")

    log_pass("VPN routes received by GoBGP")

    routes_str = json.dumps(raw_data)
    assert "65001:100" in routes_str, "RD 65001:100 not found in VPN routes"
    log_pass("VPN route with RD 65001:100 verified")

    assert gobgp.session_established(ZE_IP), "session dropped after VPN exchange"
    log_pass("VPN session with GoBGP stable")
