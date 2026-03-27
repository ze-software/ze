#!/usr/bin/env python3
"""Scenario 24: FlowSpec rule exchange with FRR via MP_REACH_NLRI.

Validates: Ze can send FlowSpec (ipv4/flow) UPDATE messages that FRR accepts.
Prevents:  FlowSpec NLRI encoding bugs, capability negotiation issues.
"""

import json
import os, sys, time
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, Ze, ZE_IP, log_info, log_pass, log_fail


def check():
    frr = FRR()
    ze = Ze()

    frr.wait_session(ZE_IP)

    # Verify ipv4/flowspec address family was negotiated.
    nbr_output = frr._vtysh_quiet("show bgp neighbor %s json" % ZE_IP)
    if nbr_output.strip():
        try:
            nbr = json.loads(nbr_output)
            peer = nbr.get(ZE_IP, {})
            afis = peer.get("addressFamilyInfo", {})
            assert "ipv4Flowspec" in afis, "ipv4/flowspec family not negotiated with FRR"
            log_pass("ipv4/flowspec address family negotiated")
        except (json.JSONDecodeError, KeyError):
            log_info("could not verify FlowSpec capability (non-fatal)")

    # FlowSpec routes may take a moment to appear.
    log_info("waiting for FlowSpec rules to arrive at FRR...")
    deadline = time.time() + 30
    raw_data = None
    while time.time() < deadline:
        output = frr._vtysh_quiet("show bgp ipv4 flowspec json")
        if output.strip():
            try:
                data = json.loads(output)
                total = data.get("totalRoutes", 0)
                if total >= 2:
                    raw_data = data
                    break
            except json.JSONDecodeError:
                pass
        time.sleep(2)

    if raw_data is None:
        log_fail("FRR did not receive FlowSpec rules within 30s")
        print(ze.logs(20))
        raise AssertionError("FRR did not receive FlowSpec rules")

    count = raw_data.get("numPrefix", len(raw_data.get("routes", {})))
    log_pass("FlowSpec rules received by FRR (count: %d)" % count)

    # Verify FlowSpec rule content: route keys should reference announced destinations.
    routes_str = json.dumps(raw_data)
    assert "10.99.0" in routes_str, \
        "destination 10.99.0.0/24 not found in FlowSpec rules"
    log_pass("FlowSpec rule with destination 10.99.0.0/24 verified")

    assert "10.99.1" in routes_str, \
        "destination 10.99.1.0/24 not found in FlowSpec rules"
    log_pass("FlowSpec rule with destination 10.99.1.0/24 verified")

    assert frr.session_established(ZE_IP), "session dropped after FlowSpec exchange"
    log_pass("FlowSpec session with FRR stable")
