#!/usr/bin/env python3
"""Scenario 18: strongSwan completes a handshake Ze answered with a COOKIE.

Validates: RFC 7296 Section 2.6. Ze is the responder at cookie-threshold 0,
           so it answers strongSwan's IKE_SA_INIT with a COOKIE notification
           and commits no state. strongSwan retries with the cookie as the
           first payload and everything else unchanged, Ze verifies it, and
           the tunnel establishes and forwards ESP.
Prevents:  a cookie a third-party implementation cannot use. Ze-to-Ze tests
           prove only that Ze agrees with itself; a cookie whose length,
           position or notify type were wrong would still round-trip between
           two Ze instances and fail against every real peer. It also proves
           the availability defect is closed without breaking interop: a
           spoofed datagram can no longer take the peer's only half-open
           slot, and a genuine peer still connects.

Discriminating power: establishment alone would pass over a responder that
never challenged. The assertion below requires Ze to have logged the
challenge, so reverting the gate in tryResponderSAInit turns this red.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    SWAN_IP,
    ZE_CONTAINER,
    assert_esp_accepted,
    docker_exec_quiet,
    log_pass,
    wait_xfrm_sa,
    wait_swan_log,
    xfrm_sa_bytes_by_spi,
)


def check():
    swan = StrongSwan()

    # 1. Ze challenged, and strongSwan answered. Both are asserted from the PEER's
    #    log rather than Ze's, for two reasons. The lab's Ze container emits nothing
    #    below WARN, so a Ze-side assertion cannot see the challenge at all. And the
    #    peer's own parse is stronger evidence: it says a third-party implementation
    #    read the cookie and rebuilt its request around it.
    #
    #    RFC 7296 Section 2.6 requires the cookie to be "the first payload, and all
    #    other payloads unchanged". strongSwan prints the payload chain in wire order,
    #    so "N(COOKIE) SA KE No" IS that ordering assertion against a real peer.
    #    Both needles are WAITED for, not read once. check() runs as soon as the
    #    containers are up, so a single docker_logs_all here races charon's very
    #    first exchange and the scenario fails on timing rather than on behaviour.
    try:
        wait_swan_log("parsed IKE_SA_INIT response 0 [ N(COOKIE) ]")
    except AssertionError:
        raise AssertionError(
            "strongSwan never parsed a COOKIE notification from Ze, so the responder "
            "did not challenge; check cookie-threshold and the gate in tryResponderSAInit"
        )
    try:
        wait_swan_log("generating IKE_SA_INIT request 0 [ N(COOKIE) SA KE No")
    except AssertionError:
        raise AssertionError(
            "strongSwan did not rebuild its IKE_SA_INIT with the COOKIE as the FIRST "
            "payload followed by SA, KE and No; Ze's cookie is not usable by a real peer"
        )

    # 2. A real third-party initiator answered the challenge and the exchange
    #    completed. strongSwan reports the SA only once IKE_AUTH is verified, so
    #    this also proves the cookie retry did not disturb the signed octets.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    # 3. Kernel state and data plane, as scenario 01 asserts them.
    wait_xfrm_sa(ZE_CONTAINER)
    wait_xfrm_sa(SWAN_CONTAINER)

    ze_before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    peer_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)
    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])
    assert_esp_accepted(
        ZE_CONTAINER,
        ze_before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "traffic did not flow through the tunnel built after the COOKIE challenge",
    )
    assert_esp_accepted(
        SWAN_CONTAINER,
        peer_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP after answering Ze's COOKIE challenge",
    )
    log_pass("strongSwan answered Ze's COOKIE challenge and the tunnel forwards")
