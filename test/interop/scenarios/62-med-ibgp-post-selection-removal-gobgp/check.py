#!/usr/bin/env python3
"""Scenario 62: raw export MED removal cannot change an IBGP readvertisement.

A real FRR peer EXTERNAL to Ze (65005 against Ze's 65001) announces one prefix
with ORIGIN, AS_PATH [65005] and MULTI_EXIT_DISC 100. Ze readvertises to an
INTERNAL GoBGP peer with next-hop self. The export filter on that GoBGP peer
returns a raw full-payload replacement with MULTI_EXIT_DISC removed. GoBGP must
still see Med 100.

The raw filter sentinel matters: without it, the scenario would pass on the
source route alone. With the sentinel and the GoBGP Med assertion together,
removing the guard in forward_med.go makes the scenario fail.
"""

# VALIDATES: the route-server egress rail keeps MULTI_EXIT_DISC when a raw export replacement tries to remove it after selection.
# VALIDATES: the raw filter's sentinel proves the removal producer ran, so the Med observed by GoBGP is restored by Ze rather than inherited from an untouched destination base.

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    FRR,
    GoBGP,
    ZE_CONTAINER,
    ZE_IP,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)

PREFIX = "10.62.0.0/24"
INJECTED_MED = 100
RAW_SENTINEL = "RAW-MED-DROP: removed MULTI_EXIT_DISC"


def _wait_route(daemon, prefix, timeout=120):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if daemon.has_route(prefix):
            return
        time.sleep(2)
    raise AssertionError("GoBGP missing route %s" % prefix)


def _gobgp_rib(gobgp, prefix):
    output = gobgp._gobgp_quiet(["global", "rib", "-a", "ipv4", prefix])
    if not output.strip() or "not in table" in output:
        log_fail("GoBGP has no path for %s, so the MED check is vacuous" % prefix)
        raise AssertionError("GoBGP missing route %s" % prefix)
    return output


def _check_raw_filter_ran():
    logs = docker_logs(ZE_CONTAINER, 200)
    if RAW_SENTINEL not in logs:
        log_fail("raw export filter did not report MED removal")
        raise AssertionError("raw export MED removal producer did not run")
    log_pass("raw export filter removed MULTI_EXIT_DISC from the destination base")


def _check_route_source(output):
    if "65005" not in output:
        log_fail("GoBGP route has no 65005 in its AS_PATH: %s" % output)
        raise AssertionError("relayed AS_PATH is not the source one")
    if ZE_IP not in output:
        log_fail("GoBGP route was not readvertised with Ze as next hop: %s" % output)
        raise AssertionError("relayed NEXT_HOP is not Ze self")
    log_pass("GoBGP route carries the source AS_PATH and Ze next hop")


def _check_med_restored(output):
    if "Med: %d" % INJECTED_MED not in output:
        log_fail("GoBGP route does not carry Med %d: %s" % (INJECTED_MED, output))
        raise AssertionError("IBGP readvertisement lost MULTI_EXIT_DISC")
    log_pass("GoBGP route still carries Med %d after raw export removal" % INJECTED_MED)


def _check():
    frr = FRR()
    gobgp = GoBGP()
    frr.wait_session(ZE_IP)
    gobgp.wait_session(ZE_IP)

    log_info("waiting for %s at GoBGP..." % PREFIX)
    _wait_route(gobgp, PREFIX)
    gobgp.check_route(PREFIX)
    _check_raw_filter_ran()

    route = _gobgp_rib(gobgp, PREFIX)
    _check_route_source(route)
    _check_med_restored(route)

    log_pass("raw MED removal was repaired before IBGP readvertisement")


def check():
    try:
        _check()
    except Exception:
        gobgp = GoBGP()
        print("--- GoBGP rib ---")
        # fail-open-ok: diagnostic print, the bare `raise` below is unconditional
        print(gobgp._gobgp_quiet(["global", "rib", "-a", "ipv4"]))
        print(docker_logs(ZE_CONTAINER, 80))
        raise
