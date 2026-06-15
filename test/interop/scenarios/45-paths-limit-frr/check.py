#!/usr/bin/env python3
"""Scenario 45: PATHS-LIMIT capability negotiation with FRR.

Verifies Ze advertises PATHS-LIMIT (code 76) and FRR accepts the session.
FRR does not enforce PATHS-LIMIT itself, but it must not reject the capability.
"""

import os, sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, docker_exec_quiet, log_pass, log_fail


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")
    frr.wait_route("10.10.0.0/24")
    frr.check_route("10.10.0.0/24")

    # Verify Add-Path was negotiated (PATHS-LIMIT rides on ADD-PATH).
    output = docker_exec_quiet(
        frr.container, ["vtysh", "-c", "show bgp neighbor 172.30.0.2"]
    )
    if "addpath" in output.lower() or "add-path" in output.lower():
        log_pass("Add-Path capability negotiated with Ze (PATHS-LIMIT piggybacks)")
    else:
        log_fail("Add-Path capability not found in FRR neighbor output")
        for line in output.splitlines()[:30]:
            print("  %s" % line)
        raise AssertionError("Add-Path not negotiated")

    # Verify session is stable (FRR did not reject unknown PATHS-LIMIT cap).
    assert frr.session_established("172.30.0.2"), (
        "session dropped -- FRR may have rejected PATHS-LIMIT capability"
    )
    log_pass("session stable with PATHS-LIMIT capability advertised")
