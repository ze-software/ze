#!/usr/bin/env python3
"""Scenario 23: VPN (L3VPN) route exchange with FRR via MP_REACH_NLRI.

Validates: Ze can send VPNv4 (ipv4/mpls-vpn) UPDATE messages that FRR accepts.
Prevents:  VPN NLRI encoding bugs (RD, label), capability negotiation issues.
"""

import json
import os, sys, time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, Ze, ZE_IP, log_info, log_pass, log_fail


def check():
    frr = FRR()
    ze = Ze()

    frr.wait_session(ZE_IP)

    # Verify ipv4/vpn address family was negotiated.
    nbr_output = frr._vtysh_quiet("show bgp neighbor %s json" % ZE_IP)
    if nbr_output.strip():
        try:
            nbr = json.loads(nbr_output)
            peer = nbr.get(ZE_IP, {})
            afis = peer.get("addressFamilyInfo", {})
            assert "ipv4Vpn" in afis, "ipv4/vpn family not negotiated with FRR"
            log_pass("ipv4/vpn address family negotiated")
        except (json.JSONDecodeError, KeyError):
            log_info("could not verify VPN capability (non-fatal)")

    # VPN routes appear under "ipv4 vpn" address family in FRR.
    log_info("waiting for VPN routes to arrive at FRR...")
    deadline = time.time() + 30
    found = False
    while time.time() < deadline:
        if frr.has_route("10.99.0.0/24", family="ipv4 vpn"):
            found = True
            break
        time.sleep(2)

    if not found:
        log_fail("FRR did not receive VPN route 10.99.0.0/24 within 30s")
        print(ze.logs(20))
        raise AssertionError("FRR did not receive VPN route 10.99.0.0/24")

    frr.check_route("10.99.0.0/24", family="ipv4 vpn")
    frr.check_route("10.99.1.0/24", family="ipv4 vpn")

    # Verify VPN-specific content: Route Distinguisher should be present.
    route_data = frr.route("10.99.0.0/24", family="ipv4 vpn")
    route_str = json.dumps(route_data)
    assert "65001:100" in route_str, "RD 65001:100 not found in VPN route data"
    log_pass("VPN route with RD 65001:100 verified")

    assert frr.session_established(ZE_IP), "session dropped after VPN route exchange"
    log_pass("VPN routes received by FRR, session stable")
