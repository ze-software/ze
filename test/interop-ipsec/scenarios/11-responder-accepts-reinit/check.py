#!/usr/bin/env python3
"""Scenario 11: Ze responder accepts strongSwan's re-init over a stale SA (Phase B).

Validates: Phase B / RFC 7296 Section 2.4 conformance of spec-fixit-ipsec-clear-
           reestablish -- the direction a ze-vs-ze test structurally cannot prove.
           strongSwan (initiator) establishes against Ze (responder). strongSwan then
           loses its SA WITHOUT telling Ze (link broken during teardown, so no Delete
           reaches Ze -- a peer crash/reboot), leaving Ze holding a STALE established
           SA. strongSwan re-initiates a fresh IKE_SA_INIT. Ze MUST accept it in
           PARALLEL (not drop it on the old responderBusy gate), honor strongSwan's
           INITIAL_CONTACT on the authenticated IKE_AUTH, and supersede the stale SA --
           WITHOUT waiting for the ~150s DPD timeout. strongSwan re-establishing
           promptly is the authoritative witness.
Prevents:  the responderBusy-held-across-established-life wedge (AC-7), a naive
           supersede-on-unauthenticated-init (RFC 7296 Section 2.4 violation), and
           INITIAL_CONTACT never being emitted/honored.

NOTE: requires the Docker strongSwan interop lab; run under `make ze-interop-ipsec-test`.
      Authored in a parked session that could not run Docker; validate at CI. The
      "teardown without a Delete" step (break_link + terminate) may need tuning against
      the lab's charon version.
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    docker_exec,
    docker_exec_quiet,
    log_fail,
    log_info,
    log_pass,
)

REESTABLISH_BOUND = 30  # seconds; far under the ~150s DPD fallback


def swan_esp_spis(swan):
    return set(re.findall(r"proto esp spi (0x[0-9a-fA-F]+)", swan.xfrm_state()))


def check():
    swan = StrongSwan()

    # 1. strongSwan (initiator) establishes against Ze (responder).
    swan.wait_sa_established("ze")
    swan.wait_child_sa()
    log_pass("strongSwan established against Ze the responder")
    # The snapshot the whole scenario rests on. `now - before` is non-empty for
    # the SA that already existed if `before` is empty, so an empty snapshot
    # passes step 4 with Ze having ignored the re-init. wait_child_sa above has
    # already proven an ESP SA is installed, so an empty read here is a fault.
    before = swan_esp_spis(swan)
    if not before:
        log_fail("no ESP SPI read from strongSwan before the re-init")
        raise AssertionError("empty ESP SPI snapshot before re-init")
    log_info("ESP SPIs before re-init: %s" % sorted(before))

    # 2. Simulate a peer crash: strongSwan drops its SA WITHOUT a Delete reaching Ze
    #    (break the outbound link, then terminate locally), so Ze is left holding a
    #    STALE established SA -- the exact wedge the fix targets.
    swan.break_link()
    docker_exec_quiet(
        SWAN_CONTAINER, ["swanctl", "--terminate", "--ike", "ze"], timeout=15
    )
    swan.restore_link()
    log_info("strongSwan dropped its SA without a Delete reaching Ze")

    # 3. strongSwan re-initiates a fresh IKE_SA_INIT while Ze still holds the stale SA.
    docker_exec(SWAN_CONTAINER, ["swanctl", "--initiate", "--child", "ze-child"])
    log_info("strongSwan re-initiated toward Ze")

    # 4. Ze must accept the fresh init in parallel and supersede its stale SA on the
    #    authenticated IKE_AUTH (INITIAL_CONTACT), re-establishing inside the bound.
    deadline = time.time() + REESTABLISH_BOUND
    while time.time() < deadline:
        now = swan_esp_spis(swan)
        if now - before:
            log_pass(
                "Ze responder accepted strongSwan's re-init and re-established "
                "(RFC 7296 Section 2.4; SPIs %s -> %s)" % (sorted(before), sorted(now))
            )
            swan.wait_sa_established("ze")
            log_pass(
                "Ze responder re-established within %ds without the DPD timeout"
                % REESTABLISH_BOUND
            )
            return
        time.sleep(2)

    log_fail(
        "Ze did not accept strongSwan's re-init within %ds (before=%s) -- the responder "
        "is still wedged on the stale SA" % (REESTABLISH_BOUND, sorted(before))
    )
    raise AssertionError("Ze responder did not accept the re-initiation")
