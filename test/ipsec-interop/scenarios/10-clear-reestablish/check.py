#!/usr/bin/env python3
"""Scenario 10: operator clear re-establishes Ze <-> strongSwan (Ze initiator).

Validates: Phase A of spec-fixit-ipsec-clear-reestablish. Ze initiates a PSK tunnel to
           strongSwan (the responder). An operator `clear vpn ipsec sa` on Ze then
           sends strongSwan an authenticated INFORMATIONAL Delete (RFC 7296 Section
           1.4) and re-initiates; strongSwan must accept the Delete and Ze's fresh
           IKE_SA_INIT and re-establish PROMPTLY -- a NEW ESP SA (changed SPI) appears
           within seconds, not after the ~150s DPD path. strongSwan is the
           authoritative witness that Ze's Delete is well-formed and correctly placed.
Prevents:  a malformed / mis-placed Delete on the clear path, and the clear paying a
           full DPD timeout against a live peer.

NOTE: requires the Docker strongSwan interop lab; run under `make ze-ipsec-interop-test`.
      Authored in a parked session that could not run Docker; validate at CI.
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    ZE_CONTAINER,
    docker_exec,
    log_fail,
    log_info,
    log_pass,
)

REESTABLISH_BOUND = 30  # seconds; far under the ~150s DPD fallback


def swan_esp_spis(swan):
    """Return the set of ESP SPIs strongSwan currently has installed."""
    return set(re.findall(r"proto esp spi (0x[0-9a-fA-F]+)", swan.xfrm_state()))


def check():
    swan = StrongSwan()

    # 1. Ze initiator establishes a PSK tunnel with strongSwan (responder).
    swan.wait_sa_established("ze")
    swan.wait_child_sa()
    log_pass("Ze initiator established a PSK tunnel with strongSwan")

    before = swan_esp_spis(swan)
    log_info("ESP SPIs before clear: %s" % (sorted(before) or "none"))

    # 2. Operator bounces the tunnel on Ze. This dispatches clear-all to the engine,
    #    which sends strongSwan an authenticated INFORMATIONAL Delete and re-initiates.
    docker_exec(ZE_CONTAINER, ["ze", "cli", "-c", "clear vpn ipsec sa"])
    log_info("ran `clear vpn ipsec sa` on Ze")

    # 3. strongSwan must accept the Delete + fresh re-init and re-establish with a NEW
    #    ESP SA (a SPI not present before) inside the bound. On unfixed code the clear
    #    would fall back to the ~150s DPD path and fail this window.
    deadline = time.time() + REESTABLISH_BOUND
    while time.time() < deadline:
        now = swan_esp_spis(swan)
        if now - before:
            log_pass(
                "strongSwan re-established a NEW ESP SA after Ze's clear "
                "(SPIs %s -> %s)" % (sorted(before), sorted(now))
            )
            # 4. Stability: the re-established IKE SA stays up.
            swan.wait_sa_established("ze")
            log_pass(
                "Ze clear re-established the tunnel against strongSwan within %ds"
                % REESTABLISH_BOUND
            )
            return
        time.sleep(2)

    log_fail(
        "strongSwan did not re-establish a new ESP SA within %ds after Ze clear "
        "(before=%s) -- Ze's Delete/re-init was not honored"
        % (REESTABLISH_BOUND, sorted(before))
    )
    raise AssertionError("clear did not re-establish against strongSwan")
