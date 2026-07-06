#!/usr/bin/env python3
"""Scenario 05: Child SA rekey Ze <-> strongSwan.

Validates: Ze, on ESP soft-lifetime expiry, initiates a real CREATE_CHILD_SA
           rekey exchange against strongSwan (RFC 7296 1.3.2): it sends the
           REKEY_SA request with the correct post-handshake message ID, processes
           strongSwan's response, installs the new Child SA, and deletes the old
           one (make-before-break). strongSwan is the authoritative witness: it
           must parse the CREATE_CHILD_SA rekey request AND receive Ze's Delete
           for the old ESP SA. Where the kernel supports XFRM the ESP SPIs must
           also change and traffic keep flowing.
Prevents:  regression to the former local-only key roll (which silently desynced
           the tunnel because the peer never learned the new keys/SPIs), stale
           post-handshake message IDs (peer rejects with "expected N, ignored"),
           and make-before-break ordering bugs.

Note: Ze logs at WARN in the container and Docker Desktop's Linux VM (macOS) has
      no XFRM/ESP support, so this asserts on strongSwan's logs (always present)
      rather than Ze's INFO logs or the dataplane; the XFRM checks run only where
      the kernel supports ESP.
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
    docker_exec_quiet,
    docker_logs_all,
    log_pass,
    ze_xfrm_state,
)


def esp_spis():
    """Set of ESP SPIs currently installed in Ze's XFRM state (empty if no XFRM)."""
    return set(re.findall(r"proto esp spi (0x[0-9a-fA-F]+)", ze_xfrm_state()))


def xfrm_bytes(container):
    output = docker_exec_quiet(container, ["ip", "-s", "xfrm", "state"])
    return sum(int(m) for m in re.findall(r"bytes\s+(\d+)", output))


def check():
    swan = StrongSwan()

    # 1. Tunnel establishes.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    initial = esp_spis()  # empty when the platform has no XFRM/ESP support

    # 2. ESP lifetime is 30s: wait until strongSwan has both parsed a
    #    CREATE_CHILD_SA rekey request from Ze AND received Ze's Delete for the
    #    superseded ESP SA (make-before-break). strongSwan is authoritative.
    deadline = time.time() + 90
    rekey_seen = delete_seen = False
    while time.time() < deadline:
        logs = docker_logs_all(SWAN_CONTAINER)
        rekey_seen = "parsed CREATE_CHILD_SA request" in logs and "REKEY_SA" in logs
        delete_seen = "received DELETE for ESP CHILD_SA" in logs
        if "expected 2, ignored" in logs or "INVALID_MESSAGE_ID" in logs:
            raise AssertionError("strongSwan rejected Ze's rekey message ID")
        if rekey_seen and delete_seen:
            break
        time.sleep(2)

    if not rekey_seen:
        raise AssertionError(
            "strongSwan never parsed a CREATE_CHILD_SA REKEY_SA request from Ze"
        )
    if not delete_seen:
        raise AssertionError(
            "strongSwan never received Ze's Delete for the old ESP SA (make-before-break)"
        )

    log_pass(
        "Ze initiated a CREATE_CHILD_SA rekey; strongSwan accepted it and the old SA delete"
    )

    # 3. Dataplane assertions where the kernel supports XFRM/ESP.
    if not initial:
        log_pass(
            "XFRM/ESP unsupported on this host; verified rekey at control plane (peer logs)"
        )
        return

    time.sleep(3)
    new = esp_spis()
    if not new or new == initial:
        raise AssertionError(
            "ESP SPIs did not change after rekey (%s -> %s)" % (initial, new)
        )

    before = xfrm_bytes(ZE_CONTAINER)
    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])
    after = xfrm_bytes(ZE_CONTAINER)
    if after <= before:
        raise AssertionError(
            "no ESP traffic after rekey (bytes before=%d after=%d)" % (before, after)
        )
    log_pass(
        "child SA rekeyed over the wire; SPIs %s -> %s; tunnel forwarding"
        % (sorted(initial), sorted(new))
    )
