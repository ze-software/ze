#!/usr/bin/env python3
"""Scenario 02: PPP subscriber /32 advertised to FRR via BGP redistribute.

Validates: Ze assigns PPP peer address, RouteObserver emits route-change,
bgp-redistribute-egress announces to FRR, and withdrawal on teardown.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    ZE_IP,
    LAC_CONTAINER,
    FRR,
    docker_exec_quiet,
    log_info,
    log_pass,
    wait_ppp_up,
    wait_ze_log,
    ze_log_contains,
)

PEER_PREFIX = "10.100.0.2/32"


def check():
    frr = FRR()

    frr.wait_session(ZE_IP)
    log_pass("FRR BGP session with Ze is Established")

    wait_ze_log("L2TP listener bound", timeout=10)
    wait_ze_log("session established", timeout=60)
    wait_ze_log("PPP session up", timeout=60)
    wait_ze_log("session IP assigned", timeout=60)

    wait_ppp_up(timeout=60)

    frr.wait_route(PEER_PREFIX, timeout=30)
    frr.check_route(PEER_PREFIX)
    log_pass("FRR received subscriber %s via BGP" % PEER_PREFIX)

    log_info("stopping LAC to trigger session teardown...")
    docker_exec_quiet(LAC_CONTAINER, ["kill", "1"], timeout=10)

    wait_ze_log("subscriber routes withdrawn", timeout=30)

    frr.wait_route_absent(PEER_PREFIX, timeout=30)
    log_pass("FRR no longer has %s after withdrawal" % PEER_PREFIX)

    assert frr.session_established(ZE_IP), "BGP session dropped after withdrawal"
    log_pass("FRR BGP session stable after route withdrawal")
