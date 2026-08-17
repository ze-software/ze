#!/usr/bin/env python3
"""Scenario 46: prefix teardown is per family, proven against a real peer.

VALIDATES: AC-2 of spec-fixit-bgp-per-family-prefix-enforcement. ze is
configured warn-only on ipv4/unicast and teardown on ipv6/unicast. FRR
announces two IPv4 routes against a maximum of one. ipv4/unicast asked for
warn-only, so FRR's session MUST survive.

PREVENTS: The defect this spec fixes returning. The three enforcement leaves
were per-peer scalars written inside a sorted per-family loop, so the last
family in key order governed every family. "ipv4/unicast" sorts before
"ipv6/unicast", so ipv6/unicast's `teardown true` reached ipv4/unicast and a
real peer was dropped for a limit the operator had asked to warn about.

Scenario 45 is the other half: single family, teardown enabled, session drops.
The pair is what binds the Cease to the OFFENDING family's own setting rather
than to any family's.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    FRR,
    ZE_CONTAINER,
    ZE_IP,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)

# How long the session must stay up after the overflow. A teardown reaches FRR
# within a second or two of the UPDATE, so this only has to outlast that.
SURVIVAL_SECONDS = 20


def _wait_overflow_logged(timeout=90):
    """Block until ze reports the ipv4/unicast overflow, and return that line."""
    deadline = time.time() + timeout
    logs = ""
    while time.time() < deadline:
        logs = docker_logs(ZE_CONTAINER, 200)
        for line in logs.splitlines():
            if "prefix count exceeded maximum" in line:
                log_pass("ze logged the ipv4/unicast overflow")
                return line
        time.sleep(1)
    log_fail("ze logs do not show prefix maximum enforcement")
    print(logs)
    raise AssertionError("missing prefix maximum log")


def check():
    frr = FRR()
    frr.wait_session(ZE_IP, timeout=90)
    log_pass("session established")

    line = _wait_overflow_logged()

    # The overflow was judged with ipv4/unicast's own setting. Against the
    # per-peer scalar this same line reads teardown=true, because
    # ipv6/unicast's value reached this family.
    assert "family=ipv4/unicast" in line, (
        "the overflow must be attributed to ipv4/unicast, got: %s" % line
    )
    assert "teardown=false" in line, (
        "ipv4/unicast asked for warn-only, so the decision must read "
        "teardown=false, got: %s" % line
    )
    log_pass("the decision used ipv4/unicast's own teardown setting")

    # The session survives. This is the assertion the defect fails.
    log_info(
        "holding for %ds to prove the session is not torn down..." % SURVIVAL_SECONDS
    )
    deadline = time.time() + SURVIVAL_SECONDS
    while time.time() < deadline:
        if not frr.session_established(ZE_IP):
            log_fail("FRR session dropped, but ipv4/unicast asked for warn-only")
            print(docker_logs(ZE_CONTAINER, 200))
            raise AssertionError("warn-only family tore the session down")
        time.sleep(2)

    log_pass("FRR session survived the ipv4/unicast overflow")

    assert frr.session_established(ZE_IP), "session must still be established"

    # What this scenario proves, and what it does NOT.
    #
    # It proves against a real peer that the decision came from ipv4/unicast's
    # own `teardown false` and that FRR's session survived it. That is the
    # per-family half, and it is the half only a peer daemon can show.
    #
    # It does NOT assert that the excess route is absent from ze's RIB. Reading
    # ze's RIB from here needs `ze show bgp rib status` inside the container, and
    # that command answers `unknown command` there: the client in the interop
    # image resolves no BGP show verbs at all. `Ze.rib_count` (interop.py) then
    # returns 0, which an upper-bound assertion would read as a pass, so the
    # assertion would be worth less than none. Scenario 05 is red on the same
    # cause today; the deferral shard of
    # plan/spec-fixit-bgp-per-family-prefix-enforcement carries it.
    #
    # The drop itself is proven elsewhere, and deliberately so:
    # test/plugin/prefix-warn-only-drops-nlri.ci reads ze's Adj-RIB-In after an
    # over-limit UPDATE and asserts the excess is absent, mutation-verified.
    # test/plugin/prefix-per-family-teardown.ci proves the DECISION and the
    # surviving session. TestPrefixTeardownPerFamilyEnforcement carries the unit
    # half, asserting drop=true from checkPrefixLimits.
    log_pass("per-family prefix enforcement holds against FRR")
