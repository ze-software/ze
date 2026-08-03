#!/usr/bin/env python3
"""Scenario 50: a real peer that goes silent is torn down by ze's hold timer.

VALIDATES: spec-fixit-bgp-session-fsm-lifecycle AC-1 against a real daemon --
an established peer that stops sending is torn down by the receive hold timer.
VALIDATES: RFC 4271 Section 8.2.2, HoldTimer_Expires (Event 10) -- ze sends a
NOTIFICATION with the error code Hold Timer Expired before it drops the TCP
connection, and FRR decodes it and reports it as RECEIVED. A peer daemon
reading the reason off the wire is the evidence a unit test cannot produce.

PREVENTS: three defects at once.
  1. Dead-peer detection not working at all, so a peer that went quiet was
     never dropped. That shows up here as a session that stays Established
     forever and a run that hits TEARDOWN_DEADLINE.
  2. The receive hold timer dropping the connection with no NOTIFICATION at
     all, leaving the peer with a bare FIN and no reason. That shows up here as
     FRR reporting a reset it originated rather than one it received.
  3. The bounded reprieve ze used to grant when the read loop had seen recent
     traffic, which tore the session down only on the SECOND expiry. That shows
     up here as a teardown past TEARDOWN_CEILING: measured at 17.6 s on this
     9 s hold before Thomas ruled it removed on 2026-08-03.

HOW THE PEER IS SILENCED, and why it is `docker pause`.

The peer must go silent AT THE BGP LAYER while its kernel keeps ACKing, because
ze has to be able to keep sending -- its own KEEPALIVEs, and then the
NOTIFICATION this scenario exists to observe. `docker pause` uses the cgroup
freezer: FRR's processes stop, the kernel TCP stack does not, so ze's writes are
ACKed and buffered while FRR sends nothing. That is exactly a dead peer.

An earlier version dropped FRR's egress with `tc netem loss 100%` and was
VACUOUS: with no ACKs coming back, ze's TCP will not put new data on the wire
either, so FRR stopped reading too and its OWN hold timer fired first. FRR then
reported `lastResetDueTo: BGP Notification send` with reason "Hold Timer
Expired" -- its own notification, not ze's -- and the scenario passed with ze's
NOTIFICATION deleted from the source. Measured, not reasoned: the mutation run
is what showed it. Any mechanism that stops the ACKs has this flaw.

`docker exec` cannot reach a paused container, so ze is the only side that can
be polled during the pause; FRR is read after the unpause, where the buffered
NOTIFICATION is waiting for it.

Not `iptables`: the pinned image (quay.io/frrouting/frr:10.3.1) ships no
iptables binary, which is worth knowing before reaching for the harness's
`FRR.break_link`.
"""

import json
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (
    FRR,
    ZE_CONTAINER,
    ZE_IP,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)

# Must match `timer { receive-hold-time 9; }` in ze.conf. RFC 4271 Section 8.2.2
# Event 10 tears the session down on the FIRST expiry, so the teardown lands at
# about ONE hold time.
HOLD_TIME = 9
TEARDOWN_DEADLINE = 90

# elapsed below is measured from the moment FRR FREEZES, not from the moment
# ze's hold timer was armed. The timer was last reset by FRR's last KEEPALIVE,
# which RFC 4271 Section 4.4 puts at hold/3, so up to one keepalive interval of
# the hold time is already spent when the clock starts. The teardown therefore
# lands between two hold times minus a keepalive interval and one hold time.
#
# Measured: 8.3 s on this 9 s hold.
KEEPALIVE_INTERVAL = HOLD_TIME / 3
TEARDOWN_FLOOR = HOLD_TIME - KEEPALIVE_INTERVAL

# Ze used to grant one bounded reprieve when the read loop had seen recent
# traffic, which put the teardown at about two hold times (measured at 17.6 s on
# this same 9 s hold). Thomas ruled that deviation removed on 2026-08-03. This
# ceiling is what makes the scenario notice if it comes back: it sits above one
# hold time plus the 1 s log-poll granularity, and below the two the reprieve
# cost.
TEARDOWN_CEILING = 1.5 * HOLD_TIME


def _docker(*args):
    return subprocess.run(
        ["docker", *args], capture_output=True, text=True, timeout=30, check=False
    )


def _freeze(frr):
    r = _docker("pause", frr.container)
    if r.returncode != 0:
        raise RuntimeError("docker pause %s failed: %s" % (frr.container, r.stderr))
    log_info("FRR frozen: it sends nothing, its kernel keeps ACKing")


def _thaw(frr):
    _docker("unpause", frr.container)


def _wait_ze_notification():
    """Wait for ze to send the Hold Timer Expired NOTIFICATION, timing it.

    Read from ze's own log because FRR is frozen and cannot be asked anything.
    The two substrings must appear on ONE line: "hold timer expired" alone also
    appears as a close reason, and matching that would pass without a
    NOTIFICATION ever being written.
    """
    started = time.monotonic()
    deadline = started + TEARDOWN_DEADLINE
    while time.monotonic() < deadline:
        for line in docker_logs(ZE_CONTAINER, 400).split("\n"):
            low = line.lower()
            if "notification sent" in low and "hold timer expired" in low:
                return time.monotonic() - started, line.strip()
        time.sleep(1)
    log_fail("ze sent no Hold Timer Expired NOTIFICATION in %ds" % TEARDOWN_DEADLINE)
    print(docker_logs(ZE_CONTAINER, 200))
    raise AssertionError(
        "RFC 4271 Section 8.2.2 Event 10: ze must send a NOTIFICATION with the "
        "error code Hold Timer Expired before it drops the connection. The peer "
        "was silent for %ds with a %ds hold time and ze wrote none"
        % (TEARDOWN_DEADLINE, HOLD_TIME)
    )


def _neighbor_json(frr):
    out = frr._vtysh_quiet("show bgp neighbor %s json" % ZE_IP)
    if not out.strip():
        return {}
    try:
        return json.loads(out).get(ZE_IP, {})
    except json.JSONDecodeError:
        return {}


def _wait_frr_saw_notification(frr, timeout=60):
    """Wait for FRR to report a RECEIVED Hold Timer Expired notification.

    The direction is the whole point. FRR renders a reset it originated as
    `BGP Notification send` and one it was told about as `... receive`, and the
    reason string is identical either way -- so a check that reads only the
    reason passes on FRR's own timer and proves nothing about ze.
    """
    deadline = time.time() + timeout
    peer = {}
    while time.time() < deadline:
        peer = _neighbor_json(frr)
        due_to = str(peer.get("lastResetDueTo", ""))
        reason = str(peer.get("lastNotificationReason", ""))
        if "receive" in due_to.lower() and "hold timer expired" in reason.lower():
            return peer, due_to, reason
        time.sleep(1)
    log_fail("FRR never reported a RECEIVED Hold Timer Expired notification")
    print(json.dumps(peer, indent=2, sort_keys=True)[:4000])
    raise AssertionError(
        "FRR did not decode a Hold Timer Expired NOTIFICATION from ze "
        "(lastResetDueTo=%r, lastNotificationReason=%r): either ze sent none, "
        "or FRR reset the session itself"
        % (peer.get("lastResetDueTo"), peer.get("lastNotificationReason"))
    )


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
    log_pass("session established with FRR")

    try:
        _freeze(frr)
        elapsed, line = _wait_ze_notification()
    finally:
        _thaw(frr)

    # The NOTIFICATION cannot arrive before the hold time that was still left to
    # run when FRR froze. Anything sooner means something other than the hold
    # timer produced it.
    assert elapsed >= TEARDOWN_FLOOR, (
        "ze sent Hold Timer Expired after %.1fs, sooner than the %.1fs still left "
        "on a %ds hold time when FRR froze: the hold timer is not what produced it"
        % (elapsed, TEARDOWN_FLOOR, HOLD_TIME)
    )
    # RFC 4271 Section 8.2.2 Event 10 runs on the FIRST expiry. A teardown that
    # waits for a second one is the reprieve ze used to grant, and this is where
    # it would show: the peer stays reachable for a hold time longer than the
    # protocol promises, and every route it fed stays in the RIB with it.
    assert elapsed < TEARDOWN_CEILING, (
        "ze sent Hold Timer Expired after %.1fs on a %ds hold time, past the "
        "%.1fs ceiling: Event 10 tears the session down on the FIRST expiry, so "
        "a teardown this late means an expiry was survived"
        % (elapsed, HOLD_TIME, TEARDOWN_CEILING)
    )
    log_pass(
        "ze sent Hold Timer Expired after %.1fs, inside one hold time of %ds "
        "(window %.1fs..%.1fs): %s"
        % (elapsed, HOLD_TIME, TEARDOWN_FLOOR, TEARDOWN_CEILING, line)
    )

    peer, due_to, reason = _wait_frr_saw_notification(frr)
    log_pass(
        "FRR decoded it off the wire: lastResetDueTo=%r, reason=%r, code/subcode=%r"
        % (due_to, reason, peer.get("lastErrorCodeSubcode"))
    )

    # The teardown must be a teardown, not a wedge: once FRR is running again
    # the peering comes back on its own.
    frr.wait_session(ZE_IP, timeout=90)
    log_pass("session re-established once FRR was running again")
