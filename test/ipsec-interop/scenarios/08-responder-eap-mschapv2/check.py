#!/usr/bin/env python3
"""Scenario 08: EAP-MSCHAPv2 responder Ze <- strongSwan (charon is the EAP client).

Validates: Ze as the IKEv2 responder AND EAP authenticator (spec-ipsec-14 AC-4).
           strongSwan initiates with auth=eap-mschapv2 and expects the responder to
           authenticate with a certificate (auth=pubkey). Ze answers IKE_SA_INIT, then
           in the first IKE_AUTH sends IDr + its server certificate + AUTH (its
           long-term credential) + the first EAP-Request; it drives the real eap.Session
           (Begin/Process) through the MSCHAPv2 challenge/response, sends EAP-Success,
           derives the MSK, exchanges MSK-derived AUTH both ways, and installs the first
           Child SA. strongSwan reaching ESTABLISHED with its Child SA INSTALLED proves
           Ze's EAP-server path end to end.

           Part two validates the mid-EAP RETRANSMISSION obligation. RFC 7296 Section
           2.1: "the responder MUST ignore the retransmitted request except insofar as
           it causes a retransmission of the response". The EAP exchange holds the
           request window open across several round trips, so a lost response there is
           answered by a duplicate of the request.

Prevents:  the "NewEAPSession has zero callers" regression, EAP-server round desync,
           and MSK-AUTH direction bugs on the responder. Part two prevents the defect
           that kept this scenario from ever passing: handleResponderEAP re-processed
           the duplicate, found neither an EAP nor an AUTH payload in it, and set
           StateDead. strongSwan then retransmitted into an SA that no longer existed
           and the IKE SA never established.

Note: macOS Docker has no XFRM for the Ze container, so this asserts the control-plane
      EAP establishment from strongSwan (authoritative); the deterministic full EAP
      handshake also runs host-independently in engine TestResponderEAPSessionWired.
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    ZE_CONTAINER,
    ZE_IP,
    docker_exec,
    docker_exec_quiet,
    docker_logs_all,
    log_info,
    log_pass,
)

# How long ze's IKE_AUTH responses are dropped. charon's first retransmit of an
# unanswered request falls at about 4s, so this window covers at least one duplicate
# with room for a second. It stays well inside the timeout either end applies to a
# half-open IKE SA, so the exchange resumes rather than restarting.
BLACKOUT = 8

# How long the SA is given to establish after the blackout lifts. Each further charon
# retransmit backs off, so the answer that completes the exchange can be a few seconds
# behind the restore.
RESUME_BOUND = 60

# The line handleResponderEAP writes when it re-processes a duplicate it should have
# replayed. It is the direct signature of the defect this part exists for.
REPROCESSED = "EAP round missing EAP payload"

# Ze abandons a half-open responder handshake at responderHandshakeTimeout, 30s from the
# SA's creation (engine/fsm.go). The blackout MUST stay well inside that: a teardown by
# that timer would fail this scenario for a reason that has nothing to do with how a
# duplicate is handled, and it would do so whether or not the replay works.
ZE_HANDSHAKE_TIMEOUT = 30


def swan_drop_ze_ike_auth(enable):
    """Drop (or stop dropping) what ze sends from UDP 4500, inside strongSwan's netns.

    The port is the selector, and it needs no timing. IKE_SA_INIT is exchanged on port
    500 and this scenario floats to 4500 for IKE_AUTH, so a rule on sport 4500 removes
    exactly the EAP-bearing responses and leaves the SA_INIT that precedes them. A rule
    armed on a clock would have to fire inside the few milliseconds an EAP round takes.
    """
    action = "-I" if enable else "-D"
    args = ["iptables", action, "INPUT"]
    if enable:
        args.append("1")
    args += ["-s", ZE_IP, "-p", "udp", "--sport", "4500", "-j", "DROP"]
    if enable:
        docker_exec(SWAN_CONTAINER, args)
    else:
        docker_exec_quiet(SWAN_CONTAINER, args)


def check():
    swan = StrongSwan()

    # Part one. Control plane: strongSwan (EAP client) establishes against Ze (EAP
    # server). This only completes if Ze answered IKE_SA_INIT, presented a valid server
    # cert + AUTH, ran the EAP-MSCHAPv2 exchange as authenticator, and verified the
    # peer's MSK-derived AUTH.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()
    log_pass(
        "strongSwan (EAP-MSCHAPv2 client) established against Ze the EAP authenticator"
    )

    # Part two. The same exchange again, with Ze's IKE_AUTH responses dropped so that
    # charon retransmits its request into a live EAP exchange.
    docker_exec_quiet(SWAN_CONTAINER, ["swanctl", "--terminate", "--ike", "ze"])
    deadline = time.time() + 30
    while time.time() < deadline:
        if "ESTABLISHED" not in swan.list_sas():
            break
        time.sleep(1)
    else:
        raise AssertionError("the first IKE SA never went away, so part two cannot run")
    log_info("first IKE SA terminated; re-initiating with ze's IKE_AUTH blacked out")

    retransmits_before = docker_logs_all(SWAN_CONTAINER).count(
        "retransmit 1 of request"
    )
    reprocessed_before = docker_logs_all(ZE_CONTAINER).count(REPROCESSED)

    swan_drop_ze_ike_auth(True)
    try:
        # Detached: with the answers dropped this call cannot succeed inside the
        # blackout, and it must not hold the scenario while charon retransmits.
        subprocess.run(
            [
                "docker",
                "exec",
                "-d",
                SWAN_CONTAINER,
                "swanctl",
                "--initiate",
                "--child",
                "ze-child",
            ],
            check=False,
            timeout=30,
        )
        # A fixed wait, not a poll of charon's log. charon block-buffers its output, so
        # `docker logs` returns a frozen snapshot while the exchange is in flight and
        # catches up only later. A loop that waited for the retransmit line to APPEAR
        # would therefore run to its own deadline no matter what charon did, and the
        # blackout would overrun ZE_HANDSHAKE_TIMEOUT. The count is taken at the end
        # instead, off a settled log.
        #
        # charon's first retransmit of an unanswered request falls at about 4s, so this
        # window carries at least one duplicate into ze.
        time.sleep(BLACKOUT)
    finally:
        swan_drop_ze_ike_auth(False)

    # Ze must have replayed the cached response rather than re-processed the duplicate.
    # This line is the defect's own signature, and it is checked before the establish
    # below so the failure names the cause rather than the symptom.
    if docker_logs_all(ZE_CONTAINER).count(REPROCESSED) > reprocessed_before:
        raise AssertionError(
            "ze re-processed the retransmitted IKE_AUTH (%r) instead of replaying its "
            "cached response, against RFC 7296 Section 2.1" % REPROCESSED
        )

    # The exchange resumes and completes. Ze's SA survived the duplicates, so charon's
    # next retransmit draws the cached response and the EAP exchange finishes.
    #
    # This is the assertion the fix is measured by. Before it, the first duplicate set
    # the SA to StateDead, every later datagram was dropped as belonging to no SA, and
    # the exchange could not finish however long it was given.
    swan.wait_sa_established("ze", timeout=RESUME_BOUND)
    swan.wait_child_sa()
    log_pass(
        "the IKE SA survived the mid-EAP retransmissions and established "
        "(RFC 7296 Section 2.1)"
    )

    # Vacuity guard, read LAST off a settled log. An establishment with no duplicate in
    # it says nothing about how a duplicate is handled, and charon's buffering means
    # this count is only trustworthy once the exchange has finished.
    retransmits_after = docker_logs_all(SWAN_CONTAINER).count("retransmit 1 of request")
    if retransmits_after <= retransmits_before:
        raise AssertionError(
            "strongSwan retransmitted nothing during the %ds blackout, so the SA above "
            "established without ever meeting a duplicate and this check is vacuous"
            % BLACKOUT
        )
    log_pass(
        "strongSwan retransmitted its IKE_AUTH into the live EAP exchange "
        "(%d retransmitted requests, was %d)" % (retransmits_after, retransmits_before)
    )
