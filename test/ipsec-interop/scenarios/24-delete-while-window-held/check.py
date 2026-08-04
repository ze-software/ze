#!/usr/bin/env python3
"""Scenario 24: Ze raises no second request while one is unanswered (RFC 7296 §2.3).

Validates: strongSwan sees ONE outstanding request from Ze at a time. Section 2.3
           MUST: "An IKE endpoint MUST wait for a response to each of its messages
           before sending a subsequent message unless it has received a
           SET_WINDOW_SIZE Notify message from its peer." Ze sends no
           SET_WINDOW_SIZE, so its window is one. With a liveness probe of Ze's own
           unanswered, the Child SA soft lifetime expires and Ze raises NO
           CREATE_CHILD_SA. Once the window frees, the same rekey goes out and its
           make-before-break Delete follows.
Prevents:  the shape this spec exists for -- two of Ze's requests unanswered at the
           peer at once, which a conforming peer answers only one of, and which Ze
           then reads as a lost message.

How the window is held: strongSwan's replies are dropped in its own OUTPUT chain
(break_link). It still RECEIVES, so its log stays the witness. Ze's DPD probe,
raised every 5s by this scenario's ike-group, goes unanswered and holds the window
for the DPD timeout of 300s. The Child SA soft lifetime falls inside that hold.

How the window frees: restore_link, and then strongSwan's own DPD request (2s)
reaches Ze. An authenticated inbound proves the peer alive, which retires the probe
and the window with it.

The hold is CONFIRMED before the absence is measured, never assumed. strongSwan logs
each request it parses, so a new "parsed INFORMATIONAL request" after the link broke
is the probe Ze is waiting on. Without that step the absence below would also hold
for a Ze that simply never rekeys.

NOTE: requires the Docker strongSwan interop lab; run under
      `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=24-delete-while-window-held`.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    docker_logs_all,
    log_info,
    log_pass,
)

PROBE_BOUND = 40  # seconds; the ike-group probes every 5s

# When the Child SA soft lifetime falls, and how long the absence is measured for.
#
# rekeyLead (engine/rekey.go) is max(jitter, min(7*3s, lifetime/2)). The jitter is under
# a tenth of the lifetime, so this scenario's esp-group lifetime of 30s gives a lead of
# 15s with no randomness in it: the soft trigger falls at 15s and the hard wall at 30s.
#
# A FIRST version measured the absence to 75s against a lifetime of 120s and was VACUOUS.
# The soft trigger sits at 99s there, so Ze had raised no rekey for a reason that had
# nothing to do with the window, and removing startChildRekey's reservation left the
# scenario green. A SECOND version fixed the bound and blacked the link out for 104s,
# which is long enough for the session to tear down and re-establish, and a fresh Child
# SA carries a fresh lifetime. The blackout below is 20s: past the soft trigger, and
# short enough that neither end gives up on the other.
SOFT_TRIGGER = 15
ABSENCE_BOUND = SOFT_TRIGGER + 5
REKEY_BOUND = 12  # seconds after the window frees, and inside the 30s hard wall
DELETE_BOUND = 15  # seconds after the rekey


def count(logs, needle):
    return logs.count(needle)


def check():
    swan = StrongSwan()

    # 1. Ze initiator establishes a PSK tunnel with strongSwan (responder).
    swan.wait_sa_established("ze")
    swan.wait_child_sa()
    established = time.time()
    log_pass("Ze initiator established a PSK tunnel with strongSwan")

    rekeys_before = count(docker_logs_all(SWAN_CONTAINER), "CREATE_CHILD_SA request")
    log_info(
        "CREATE_CHILD_SA requests parsed before the link broke: %d" % rekeys_before
    )

    swan.break_link()
    try:
        # 2. strongSwan can no longer answer, so the next probe holds Ze's request
        #    window. Wait for strongSwan to report that it parsed one.
        informationals = count(
            docker_logs_all(SWAN_CONTAINER), "parsed INFORMATIONAL request"
        )
        deadline = time.time() + PROBE_BOUND
        while time.time() < deadline:
            logs = docker_logs_all(SWAN_CONTAINER)
            if count(logs, "parsed INFORMATIONAL request") > informationals:
                break
            time.sleep(1)
        else:
            raise AssertionError(
                "strongSwan parsed no new INFORMATIONAL request within %ds, so Ze's "
                "request window is not held and this scenario would prove nothing"
                % PROBE_BOUND
            )
        log_pass("Ze's liveness probe reached strongSwan and cannot be answered")

        # 3. POSITIVE for the wait rule. The Child SA soft lifetime passes while the
        #    probe is unanswered. Ze must raise no CREATE_CHILD_SA.
        while time.time() - established < ABSENCE_BOUND:
            logs = docker_logs_all(SWAN_CONTAINER)
            if count(logs, "CREATE_CHILD_SA request") > rekeys_before:
                raise AssertionError(
                    "Ze sent a CREATE_CHILD_SA rekey while its own liveness probe was "
                    "unanswered: two requests outstanding, against RFC 7296 Section 2.3"
                )
            time.sleep(1)
        log_pass(
            "the Child SA soft lifetime passed at %ds and Ze raised no second "
            "request (%ds with the window held)" % (SOFT_TRIGGER, ABSENCE_BOUND)
        )
    finally:
        swan.restore_link()

    # 4. NEGATIVE. The window is not a mute sender. strongSwan's own DPD proves the
    #    peer alive to Ze, which retires the probe and frees the window, and the rekey
    #    Ze deferred goes out.
    deadline = time.time() + REKEY_BOUND
    while time.time() < deadline:
        logs = docker_logs_all(SWAN_CONTAINER)
        if (
            count(logs, "CREATE_CHILD_SA request") > rekeys_before
            and "REKEY_SA" in logs
        ):
            break
        time.sleep(1)
    else:
        raise AssertionError(
            "Ze raised no CREATE_CHILD_SA within %ds of the window freeing, so the "
            "deferred rekey was dropped rather than deferred" % REKEY_BOUND
        )
    log_pass("the deferred rekey went out once the request window freed")

    # 5. The make-before-break Delete follows the rekey response, and strongSwan is the
    #    authority that it arrived.
    deadline = time.time() + DELETE_BOUND
    while time.time() < deadline:
        if "received DELETE for ESP CHILD_SA" in docker_logs_all(SWAN_CONTAINER):
            log_pass("strongSwan received Ze's Delete for the superseded Child SA")
            return
        time.sleep(1)
    raise AssertionError(
        "strongSwan never received Ze's Delete for the old ESP SA within %ds"
        % DELETE_BOUND
    )
