#!/usr/bin/env python3
"""Scenario 01: Full PPP IPv4 proof with real xl2tpd/pppd LAC peer.

Validates: L2TP tunnel, PPP LCP/IPCP, pppN kernel interface, address
assignment, dataplane ping, route inject/withdraw logs, and cleanup.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    ZE_CONTAINER,
    ZE_IP,
    LAC_CONTAINER,
    docker_exec,
    docker_exec_quiet,
    docker_logs,
    lac_ping,
    log_fail,
    log_info,
    log_pass,
    wait_l2tp_clean,
    wait_ppp_up,
    wait_ze_log,
    ze_l2tp_tunnels,
    ze_log_contains,
    ze_ppp_addr,
    ze_ppp_links,
)

LOCAL_ADDR = "10.100.0.1"
PEER_ADDR = "10.100.0.2"


def check():
    wait_ze_log("L2TP listener bound", timeout=10)

    wait_ze_log("session established", timeout=60)
    log_pass("L2TP session established")

    wait_ze_log("PPP session up", timeout=60)
    log_pass("PPP session up")

    wait_ze_log("session IP assigned", timeout=60)
    log_pass("session IP assigned")

    iface = wait_ppp_up(timeout=60)

    addr_output = ze_ppp_addr(iface)
    if LOCAL_ADDR not in addr_output or PEER_ADDR not in addr_output:
        log_fail(
            "%s address mismatch: expected %s peer %s, got: %s"
            % (iface, LOCAL_ADDR, PEER_ADDR, addr_output.strip())
        )
        raise AssertionError("PPP address mismatch on %s" % iface)
    log_pass("%s has %s peer %s" % (iface, LOCAL_ADDR, PEER_ADDR))

    links = ze_ppp_links()
    if len(links) != 1:
        log_fail("expected exactly 1 PPP link in Ze, got %d: %s" % (len(links), links))
        raise AssertionError("unexpected PPP link count: %d" % len(links))
    log_pass("Ze has exactly 1 PPP interface")

    time.sleep(2)
    if not lac_ping(LOCAL_ADDR, count=3):
        log_fail("LAC cannot ping Ze PPP address %s" % LOCAL_ADDR)
        raise AssertionError("dataplane ping failed")
    log_pass("LAC can ping Ze PPP address %s through tunnel" % LOCAL_ADDR)

    if not ze_log_contains("subscriber route inject"):
        log_fail("Ze log missing 'subscriber route inject'")
        raise AssertionError("route inject not logged")
    log_pass("Ze logged subscriber route inject")

    log_info("stopping LAC to trigger session teardown...")
    docker_exec_quiet(LAC_CONTAINER, ["kill", "1"], timeout=10)

    wait_ze_log("subscriber routes withdrawn", timeout=30)
    log_pass("Ze logged subscriber routes withdrawn")

    wait_l2tp_clean(timeout=30)
    log_pass("kernel L2TP/PPP state clean after teardown")
