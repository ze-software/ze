#!/usr/bin/env python3
"""Scenario 11: Add-Path capability negotiation with FRR."""

import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, docker_exec_quiet, log_pass, log_fail


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")
    frr.wait_route("10.10.0.0/24")
    frr.check_route("10.10.0.0/24")

    # Check if Add-Path was negotiated.
    output = docker_exec_quiet(frr.container, ["vtysh", "-c", "show bgp neighbor 172.30.0.2"])
    if "add-path" in output.lower() or "Add-Path" in output or "addpath" in output.lower():
        log_pass("Add-Path capability negotiated with Ze")
    else:
        log_fail("Add-Path capability not found in FRR neighbor output")
        for line in output.splitlines()[:30]:
            print("  %s" % line)
        raise AssertionError("Add-Path not negotiated")

    # Check for multiple paths (FRR shows multiple entries with different AS paths).
    route_output = docker_exec_quiet(frr.container, ["vtysh", "-c", "show bgp ipv4 unicast 10.10.0.0/24"])
    path_count = route_output.count("65001")
    if path_count >= 2:
        log_pass("FRR received %d paths for 10.10.0.0/24 via Add-Path" % path_count)
    else:
        log_fail("FRR shows %d path entries for 10.10.0.0/24 (expected >= 2)" % path_count)
        raise AssertionError("Add-Path: expected >= 2 paths, got %d" % path_count)

    assert frr.session_established("172.30.0.2"), "session dropped after Add-Path exchange"
    log_pass("session stable after Add-Path route exchange")
