#!/usr/bin/env python3
"""Scenario 26: the ORIGINAL RESPONDER raises its own exchange, Ze -> strongSwan.

Validates: RFC 7296 Section 2.2's two independent Message IDs, witnessed by a foreign
           implementation. strongSwan dials the tunnel, so Ze answers IKE_AUTH and is
           the original responder. Ze's 30s ESP lifetime then expires and ZE raises the
           CREATE_CHILD_SA rekey. Section 2.2 gives each end its own request counter,
           so that first self-initiated request carries Message ID 0 and charon must
           accept it. charon is the authoritative witness: it parses the REKEY_SA
           request and receives Ze's Delete for the superseded ESP SA.
Prevents:  the defect commit 86b6aa291 fixed. finishResponderEstablish (engine/
           responder.go) set sa.NextMsgID from the PEER's IKE_AUTH id, so a
           responder-role Ze raised its first request at id 2 while a conforming peer
           expected 0. classifyInbound (engine/msgid.go) matches exactly, so the peer
           answered nothing: no DPD probe, no Delete, no Child SA rekey and no IKE SA
           rekey ever completed from this side.

Why scenario 05 does not cover it. There Ze is the connection INITIATOR, whose counter
was always right, and reverting the responder fix leaves 05 green. Every other
responder scenario has Ze ANSWERING an exchange charon started (07, 09, 11, 25). This
is the only direction where a responder-role Ze speaks first.

Note: requires the Docker strongSwan interop lab; run under
      `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=26-responder-raises-child-rekey`.
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    SWAN_IP,
    ZE_CONTAINER,
    assert_esp_accepted,
    docker_exec,
    docker_exec_quiet,
    docker_logs_all,
    log_pass,
    xfrm_sa_bytes_by_spi,
)

# charon's out-of-window refusal, kept as a diagnostic and not as the assertion.
#
# MEASURED 2026-08-22 with the old counter write restored: charon logged NO refusal at
# all and simply never parsed the request, so this scenario fails on rekey_seen below.
# That absence is the discrimination. This pattern turns a charon that does say why into
# a message naming the cause, which is worth the two lines a silent timeout costs the
# next reader.
OUT_OF_WINDOW = re.compile(r"expected \d+, ignored")


def esp_spis():
    """Every ESP SPI the kernel in Ze's container currently holds a state for.

    It reads through `docker_exec`, which RAISES and names the container, the command
    and the stderr when the read fails. `lab.ze_xfrm_state` uses the quiet form instead,
    whose "" is the same value for a failed read and for a kernel holding no state. An
    empty set from here is therefore a reading: the kernel has no ESP state. Step 3
    below treats that as a host without XFRM and skips the dataplane assertions, and
    that decision is only sound because a broken read cannot produce it.
    """
    return set(
        re.findall(
            r"proto esp spi (0x[0-9a-fA-F]+)",
            docker_exec(ZE_CONTAINER, ["ip", "xfrm", "state"]),
        )
    )


def check():
    swan = StrongSwan()

    # 1. charon dials, so Ze is the original responder.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    initial = esp_spis()  # empty when the platform has no XFRM/ESP support

    # 2. Ze's ESP lifetime is 30s and every charon timer is off, so the only rekey on
    #    the wire is Ze's. charon must parse it and must answer the Delete that
    #    follows. A Message ID charon did not expect shows up as the refusal above.
    deadline = time.time() + 120
    rekey_seen = delete_seen = False
    while time.time() < deadline:
        logs = docker_logs_all(SWAN_CONTAINER)
        rekey_seen = "parsed CREATE_CHILD_SA request" in logs and "REKEY_SA" in logs
        delete_seen = "received DELETE for ESP CHILD_SA" in logs
        if OUT_OF_WINDOW.search(logs) or "INVALID_MESSAGE_ID" in logs:
            raise AssertionError(
                "strongSwan refused a Message ID from a responder-role Ze: each end "
                "keeps its own request counter (RFC 7296 Section 2.2), so Ze's first "
                "self-initiated request must be id 0"
            )
        if rekey_seen and delete_seen:
            break
        time.sleep(2)

    if not rekey_seen:
        raise AssertionError(
            "strongSwan never parsed a CREATE_CHILD_SA REKEY_SA request from a "
            "responder-role Ze; its request reached the peer at no id the peer accepts"
        )
    if not delete_seen:
        raise AssertionError(
            "strongSwan never received Ze's Delete for the old ESP SA, so the "
            "make-before-break exchange Ze started never completed"
        )

    log_pass(
        "a responder-role Ze raised its own CREATE_CHILD_SA rekey and strongSwan "
        "accepted it at the Message ID it expected"
    )

    # 3. Dataplane assertions where the kernel supports XFRM/ESP.
    if not initial:
        log_pass(
            "XFRM/ESP unsupported on this host; verified at the control plane (peer logs)"
        )
        return

    time.sleep(3)
    new = esp_spis()
    if not new or new == initial:
        raise AssertionError(
            "ESP SPIs did not change after the rekey (%s -> %s)" % (initial, new)
        )

    # Counters are compared per SPI, never as a total: the 30s lifetime can retire an
    # SA between the two readings, and a summed reading then FALLS while traffic flows
    # (the reason scenario 05 carries the same note).
    before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    peer_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)
    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])

    assert_esp_accepted(
        ZE_CONTAINER,
        before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "no ESP traffic after the rekey Ze raised",
    )
    assert_esp_accepted(
        SWAN_CONTAINER,
        peer_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP from Ze after the rekey Ze raised",
    )
    log_pass(
        "child SA rekeyed from the responder side; SPIs %s -> %s; tunnel forwarding"
        % (sorted(initial), sorted(new))
    )
