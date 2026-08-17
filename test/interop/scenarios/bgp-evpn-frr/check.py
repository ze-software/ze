#!/usr/bin/env python3
"""Scenario 22: EVPN Type-2 route exchange with FRR via MP_REACH_NLRI.

Validates: Ze can send EVPN (l2vpn/evpn) UPDATE messages that FRR accepts.
Prevents:  EVPN NLRI encoding bugs (Type-2 MAC/IP), capability negotiation issues.
"""

import json
import os, sys, time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, Ze, ZE_IP, log_info, log_pass, log_fail


def check():
    frr = FRR()
    ze = Ze()

    frr.wait_session(ZE_IP)

    # Verify l2vpn/evpn address family was negotiated.
    nbr_output = frr._vtysh_quiet("show bgp neighbor %s json" % ZE_IP)
    if nbr_output.strip():
        try:
            nbr = json.loads(nbr_output)
            peer = nbr.get(ZE_IP, {})
            afis = peer.get("addressFamilyInfo", {})
            assert "l2VpnEvpn" in afis, "l2vpn/evpn family not negotiated with FRR"
            log_pass("l2vpn/evpn address family negotiated")
        except (json.JSONDecodeError, KeyError):
            log_info("could not verify EVPN capability (non-fatal)")

    # EVPN routes in FRR are queried via "show bgp l2vpn evpn json".
    # FRR structures EVPN routes under the RD key (e.g., "1:1"), not "routes".
    log_info("waiting for EVPN routes to arrive at FRR...")
    deadline = time.time() + 30
    raw_data = None
    while time.time() < deadline:
        output = frr._vtysh_quiet("show bgp l2vpn evpn json")
        if output.strip():
            try:
                data = json.loads(output)
                # EVPN routes are under RD keys; check numPrefix or any RD key
                if data.get("numPrefix", 0) > 0:
                    raw_data = data
                    break
            except json.JSONDecodeError:
                pass
        time.sleep(2)

    if raw_data is None:
        log_fail("FRR did not receive EVPN routes within 30s")
        raw = frr._vtysh_quiet("show bgp l2vpn evpn")
        log_info("FRR EVPN table: %s" % raw[:500])
        print(ze.logs(20))
        raise AssertionError("FRR did not receive EVPN routes")

    log_pass("EVPN routes received by FRR")

    # Verify EVPN route content: check for announced MAC addresses.
    routes_str = json.dumps(raw_data)
    assert "00:11:22:33:44:55" in routes_str, (
        "MAC 00:11:22:33:44:55 not found in EVPN routes"
    )
    log_pass("EVPN Type-2 route with MAC 00:11:22:33:44:55 verified")

    assert "00:11:22:33:44:66" in routes_str, (
        "MAC 00:11:22:33:44:66 not found in EVPN routes"
    )
    log_pass("EVPN Type-2 route with MAC 00:11:22:33:44:66 verified")

    assert frr.session_established(ZE_IP), "session dropped after EVPN exchange"
    log_pass("EVPN session with FRR stable")
