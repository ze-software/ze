#!/usr/bin/env python3
"""Scenario 01: PSK site-to-site IKEv2 tunnel Ze <-> strongSwan.

Validates: Ze's IKEv2 engine negotiates IKE_SA_INIT and IKE_AUTH with
           strongSwan using pre-shared key authentication. Verifies
           that both the IKE SA and Child SA are established, XFRM
           state (ESP SAs) is installed in both containers, and that
           traffic between the peers flows through the ESP tunnel
           (verified via XFRM SA byte counters).
Prevents:  IKEv2 wire format regressions, proposal negotiation drift,
           PSK authentication failures, Child SA installation bugs,
           and dataplane wiring issues that control-plane-only checks
           would miss.
"""

import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    ZE_CONTAINER,
    SWAN_CONTAINER,
    SWAN_IP,
    check_xfrm_sa_count,
    docker_exec_quiet,
    log_fail,
    log_pass,
    wait_xfrm_sa,
)


def xfrm_bytes(container):
    """Sum ESP SA byte counters from ip -s xfrm state output."""
    output = docker_exec_quiet(container, ["ip", "-s", "xfrm", "state"])
    total = 0
    for m in re.finditer(r"bytes\s+(\d+)", output):
        total += int(m.group(1))
    return total


def check():
    swan = StrongSwan()

    # 1. Control plane: IKE SA and Child SA established.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    # 2. Kernel state: ESP SAs installed on both sides.
    wait_xfrm_sa(ZE_CONTAINER)
    wait_xfrm_sa(SWAN_CONTAINER)
    check_xfrm_sa_count(SWAN_CONTAINER, 2)

    # 3. Data plane: send traffic and verify it flows through ESP.
    #    Ping alone is insufficient: both containers share a Docker bridge,
    #    so ICMP succeeds regardless of XFRM. The real proof is that XFRM
    #    SA byte counters increase after sending traffic.
    bytes_before = xfrm_bytes(ZE_CONTAINER)

    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])

    bytes_after = xfrm_bytes(ZE_CONTAINER)
    if bytes_after <= bytes_before:
        log_fail(
            "XFRM SA byte counters did not increase (before=%d after=%d)"
            % (bytes_before, bytes_after)
        )
        raise AssertionError("traffic did not flow through XFRM tunnel")

    log_pass(
        "traffic verified through ESP tunnel (XFRM bytes: %d -> %d)"
        % (bytes_before, bytes_after)
    )
    log_pass("PSK site-to-site tunnel established and forwarding traffic")
