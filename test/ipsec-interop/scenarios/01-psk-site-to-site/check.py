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
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    ZE_CONTAINER,
    SWAN_CONTAINER,
    SWAN_IP,
    assert_esp_accepted,
    check_xfrm_sa_count,
    docker_exec_quiet,
    log_pass,
    wait_xfrm_sa,
    xfrm_sa_bytes_by_spi,
)


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
    #    Counters are compared per SPI, so a rekey between the two readings cannot
    #    make the check fail: a deleted SA leaves the intersection rather than
    #    subtracting its counter from a total.
    bytes_before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    peer_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)

    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])

    assert_esp_accepted(
        ZE_CONTAINER,
        bytes_before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "traffic did not flow through the XFRM tunnel",
    )

    # 4. Interop: the peer must ACCEPT what Ze encrypted. Ze's own counter proves
    #    only that Ze encrypted and sent. strongSwan's inbound counter advances
    #    after it looks the SPI up, decrypts, and verifies the GCM tag, so it is
    #    the assertion that proves the two implementations agree on ESP.
    assert_esp_accepted(
        SWAN_CONTAINER,
        peer_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP from Ze",
    )
    log_pass("PSK site-to-site tunnel established and forwarding traffic")
