#!/usr/bin/env python3
"""Scenario 13: Graceful Restart capability negotiation with FRR."""

import json
import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, Ze, docker_exec_quiet, log_pass, log_fail


def check():
    frr = FRR()
    ze = Ze()

    frr.wait_session("172.30.0.2")

    # Wait for routes from Ze.
    frr.wait_route("10.10.0.0/24")
    frr.check_route("10.10.0.0/24")

    # Check if GR capability was negotiated via JSON.
    output = docker_exec_quiet(frr.container, ["vtysh", "-c", "show bgp neighbor 172.30.0.2 json"])
    gr_found = False
    if output.strip():
        try:
            data = json.loads(output)
            peer = data.get("172.30.0.2", {})
            gr_info = peer.get("gracefulRestartInfo", {})
            if gr_info:
                gr_found = True
            # Also check neighborCapabilities.
            caps = peer.get("neighborCapabilities", {})
            if "gracefulRestart" in caps or "gracefulRestartCapability" in caps:
                gr_found = True
        except json.JSONDecodeError:
            pass

    if not gr_found:
        # Fallback to text check.
        output = docker_exec_quiet(frr.container, ["vtysh", "-c", "show bgp neighbor 172.30.0.2"])
        if "graceful restart" in output.lower():
            gr_found = True

    if gr_found:
        log_pass("Graceful Restart capability negotiated with Ze")
    else:
        log_fail("Graceful Restart capability not found in FRR neighbor output")
        raise AssertionError("GR not negotiated")

    ze.rib_received(1)

    assert frr.session_established("172.30.0.2"), "session dropped"
    log_pass("GR negotiated, routes exchanged, session stable")
