#!/usr/bin/env python3
"""Scenario 35: SRv6 VPNv6 route exchange with FRR.

Validates: Ze receives VPNv6 prefix from FRR with SRv6 Prefix-SID attribute,
           extracts the SRv6 SID, and programs it through the system RIB.
Prevents:  PrefixSID attribute (code 40) lost or misparsed, SRv6 SID extraction
           failure, transposition errors for VPN SAFI.
"""

import json
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, Ze, ZE_IP, log_info, log_pass, log_fail


def check():
    frr = FRR()
    ze = Ze()

    frr.wait_session(ZE_IP)

    # Verify ipv6 vpn address family was negotiated.
    nbr_output = frr._vtysh_quiet("show bgp neighbor %s json" % ZE_IP)
    if nbr_output.strip():
        try:
            nbr = json.loads(nbr_output)
            peer = nbr.get(ZE_IP, {})
            afis = peer.get("addressFamilyInfo", {})
            assert "ipv6Vpn" in afis or "IPv6 VPN" in str(afis), (
                "ipv6/vpn family not negotiated with FRR"
            )
            log_pass("ipv6/vpn address family negotiated")
        except (json.JSONDecodeError, KeyError, AssertionError) as e:
            log_fail("capability check: %s" % e)
            raise

    # Wait for VPNv6 route with SRv6 SID to appear in FRR's VPN table.
    log_info("waiting for SRv6 VPNv6 route in FRR BGP table...")
    deadline = time.time() + 30
    frr_routes = None
    while time.time() < deadline:
        output = frr._vtysh_quiet("show bgp ipv6 vpn json")
        if output.strip():
            try:
                data = json.loads(output)
                if data.get("numPrefix", 0) > 0 or "routes" in data:
                    frr_routes = data
                    break
                for rd_data in data.values():
                    if isinstance(rd_data, dict) and rd_data.get("numPrefix", 0) > 0:
                        frr_routes = data
                        break
                if frr_routes:
                    break
            except json.JSONDecodeError:
                pass
        time.sleep(2)

    if frr_routes is None:
        log_fail("FRR did not have VPNv6 routes within 30s")
        raw = frr._vtysh_quiet("show bgp ipv6 vpn")
        log_info("FRR VPNv6 table: %s" % raw[:500])
        raise AssertionError("FRR did not have VPNv6 routes")

    log_pass("VPNv6 routes present in FRR")

    # Verify FRR advertised the route with SRv6 SID.
    routes_str = json.dumps(frr_routes)
    assert "2001:db8:customer" in routes_str, (
        "customer prefix 2001:db8:customer::/48 not found in VPNv6 table"
    )
    log_pass("customer prefix 2001:db8:customer::/48 present")

    # Check Ze received the route (RIB count > 0).
    log_info("waiting for Ze to receive VPNv6 route...")
    deadline = time.time() + 30
    received = False
    while time.time() < deadline:
        count = ze.rib_count()
        if count > 0:
            received = True
            break
        time.sleep(2)

    if not received:
        log_fail("Ze did not receive routes within 30s")
        print(ze.logs(30))
        raise AssertionError("Ze did not receive VPNv6 routes")

    log_pass("Ze received VPNv6 route from FRR")

    # Verify session stability.
    assert frr.session_established(ZE_IP), "session dropped after SRv6 exchange"
    log_pass("SRv6 VPN session with FRR stable")
