#!/usr/bin/env python3
"""Administrative shutdown tells the peer WHY, and the peer drops the routes.

SIGTERM to Ze must put a Cease / Administrative Shutdown NOTIFICATION on the
wire before the socket closes (RFC 4271 Section 8.2.2 ManualStop, RFC 4486
subcode 2).

THE NOTIFICATION IS THE DISCRIMINATOR, AND IT IS THE ONLY ONE. Measured against
this same lab on 2026-08-10, before the fix: FRR withdrew 10.10.0.0/24 0.1s
after the bare FIN, reported `Notifications: 0 0`, and gave the operator
`Last reset ... Peer closed the session (n/a)` as the reason. So the route
assertion at the foot of this file passes with or without the fix and proves
nothing on its own -- it is kept as a guard against the OPPOSITE regression, a
shutdown that leaves the peer holding routes.

Why the routes went anyway: Ze advertises Graceful Restart with a Restart Time
and NO per-AFI/SAFI tuple, so FRR reads `Remote GR Mode: NotApplicable` and
never enters helper mode for this peer (RFC 4724 Section 3: the tuple list is
what says which families a speaker preserves). FRR is configured with
`bgp graceful-restart` here on purpose: it is the configuration under which a
stale-route hold WOULD happen if Ze ever advertised those tuples, so this
scenario is where that regression would surface.
"""

import json
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (
    FRR,
    FRR_CONTAINER,
    ZE_CONTAINER,
    ZE_IP,
    docker_exec_quiet,
    docker_signal,
    log_fail,
    log_info,
    log_pass,
)

# Wall-clock ceiling for the whole stop. The reactor gives peers 2s and the hub
# gives the engine 3s, so a graceful NOTIFICATION that fits its budget leaves
# the exit well inside this. The bound is loose on purpose: it is here to catch
# a HANG, not to time a healthy exit on a shared machine.
EXIT_BUDGET = 15


def notification_seen():
    """Report how FRR says the session ended.

    Three probes, tried in order, because FRR words the answer differently per
    version. Returns the reason line, or "" when FRR ANSWERED and named no
    administrative shutdown.

    Raises when no probe answered at all. `docker_exec_quiet` returns "" for a
    command that failed as readily as for one that had nothing to say, and the
    caller reads "" as "ze sent no NOTIFICATION" -- so a broken vtysh, a
    container that is gone, or a docker daemon that refused would be reported
    as a protocol defect in ze. Every probe coming back empty is the one state
    in which that verdict is unavailable, and saying so is the whole point.
    """
    answered = False

    output = docker_exec_quiet(
        FRR_CONTAINER, ["vtysh", "-c", "show bgp neighbor %s json" % ZE_IP]
    )
    if output.strip():
        answered = True
        try:
            peer = json.loads(output).get(ZE_IP, {})
            for key in ("lastNotificationReason", "lastErrorCodeSubcode"):
                value = peer.get(key)
                if value and "dministrat" in str(value):
                    return "%s=%s" % (key, value)
        except json.JSONDecodeError:
            pass

    output = docker_exec_quiet(
        FRR_CONTAINER, ["vtysh", "-c", "show bgp neighbor %s" % ZE_IP]
    )
    if output.strip():
        answered = True
        for line in output.splitlines():
            low = line.lower()
            if "notification received" in low and "dministrat" in low:
                return line.strip()

    output = docker_exec_quiet(FRR_CONTAINER, ["cat", "/tmp/frr.log"])
    if output.strip():
        answered = True
        for line in output.splitlines():
            low = line.lower()
            if "notification" in low and "recv" in low and "dministrat" in low:
                return line.strip()

    if not answered:
        raise RuntimeError(
            "no answer from any of the three FRR probes (`show bgp neighbor "
            "%s json`, the plain form, and /tmp/frr.log): the query failed, so "
            "whether ze sent a NOTIFICATION is unknown rather than answered no" % ZE_IP
        )
    return ""


def check():
    frr = FRR()

    frr.wait_session(ZE_IP)
    # 60s, not the 30s default: the route comes from an external process plugin
    # that has to start, declare itself and flush, and one run on 2026-08-10
    # missed the 30s mark on a host that was building container images at the
    # same time. The shutdown assertions below are what this scenario is for;
    # losing them to a slow announce would be a false red.
    frr.wait_route("10.10.0.0/24", timeout=60)
    frr.check_route("10.10.0.0/24")

    log_info("sending SIGTERM to the ze container")
    started = time.time()
    docker_signal(ZE_CONTAINER, "TERM")

    # docker wait returns the exit status once the container's PID 1 is gone.
    try:
        subprocess.run(
            ["docker", "wait", ZE_CONTAINER],
            capture_output=True,
            text=True,
            timeout=EXIT_BUDGET + 15,
            check=False,
        )
    except subprocess.TimeoutExpired:
        log_fail("ze did not exit within %ds of SIGTERM" % (EXIT_BUDGET + 15))
        raise AssertionError("ze did not exit after SIGTERM")
    elapsed = time.time() - started

    if elapsed > EXIT_BUDGET:
        log_fail(
            "ze took %.1fs to exit after SIGTERM (budget %ds)" % (elapsed, EXIT_BUDGET)
        )
        raise AssertionError("shutdown exceeded its budget")
    log_pass("ze exited %.1fs after SIGTERM" % elapsed)

    # FRR needs a moment to process the NOTIFICATION and update its table.
    deadline = time.time() + 30
    reason = ""
    while time.time() < deadline:
        reason = notification_seen()
        if reason:
            break
        time.sleep(2)

    if not reason:
        log_fail("FRR reports no Cease/Administrative Shutdown NOTIFICATION from Ze")
        raise AssertionError("no administrative shutdown NOTIFICATION observed at FRR")
    log_pass("FRR received Cease/Administrative Shutdown: %s" % reason)

    # Guard, not discriminator: see the module docstring. The routes must be
    # gone, and they were already gone before the fix.
    frr.wait_route_absent("10.10.0.0/24", timeout=30)
    log_pass("FRR withdrew 10.10.0.0/24 rather than holding it as a stale GR route")
