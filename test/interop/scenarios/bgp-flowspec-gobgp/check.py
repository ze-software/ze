#!/usr/bin/env python3
"""Scenario 30: FlowSpec rule exchange with GoBGP via MP_REACH_NLRI.

Validates: Ze can send FlowSpec (ipv4/flow) UPDATE messages that GoBGP accepts.
Prevents:  FlowSpec NLRI encoding bugs specific to GoBGP's implementation.
"""

import json
import os, sys, time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import GoBGP, Ze, ZE_IP, log_info, log_pass, log_fail


def check():
    gobgp = GoBGP()
    ze = Ze()

    gobgp.wait_session(ZE_IP)

    log_info("waiting for FlowSpec rules to arrive at GoBGP...")
    deadline = time.time() + 30
    raw_data = None
    while time.time() < deadline:
        data = gobgp._gobgp_json(["global", "rib", "-a", "ipv4-flowspec"])
        if data and (
            isinstance(data, list)
            and len(data) > 0
            or isinstance(data, dict)
            and len(data) > 0
        ):
            raw_data = data
            break
        time.sleep(2)

    if raw_data is None:
        log_fail("GoBGP did not receive FlowSpec rules within 30s")
        print(ze.logs(20))
        raise AssertionError("GoBGP did not receive FlowSpec rules")

    routes_str = json.dumps(raw_data)
    log_pass("FlowSpec rules received by GoBGP")

    assert "10.99.0" in routes_str, (
        "destination 10.99.0.0/24 not found in FlowSpec rules"
    )
    log_pass("FlowSpec rule with destination 10.99.0.0/24 verified")

    assert gobgp.session_established(ZE_IP), "session dropped after FlowSpec exchange"
    log_pass("FlowSpec session with GoBGP stable")
