#!/usr/bin/env python3
"""Scenario: AS112 origin-AS override -- spec-as112-3 AC-6/AC-7 wire-correctness.

Validates: a peer-group configured with session.asn.local 112 + local-options
[replace-as] presents AS_PATH origin 112 to a real external FRR peer, while a
second peer-group (same injected AS112 routes, no asn.local override)
presents ze's real local AS to a real external BIRD peer -- both controlled
independently from the SAME underlying route.
Prevents: asn.local/replace-as being internally-consistent-only within ze's
own test harness without being wire-correct against real BGP implementations
(AC-6, AC-7).
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD, FRR, ZE_IP, log_fail, log_pass

PREFIX1 = "192.175.48.0/24"
PREFIX2 = "192.31.196.0/24"
ZE_REAL_AS = "65001"
AS112 = "112"


def _frr_aspath(frr, prefix):
    """Return FRR's AS_PATH string for prefix (JSON), or "" if unavailable."""
    data = frr.route(prefix)
    paths = data.get("paths", [])
    if not paths:
        return ""
    aspath = paths[0].get("aspath", {})
    if isinstance(aspath, dict):
        return aspath.get("string", "")
    if isinstance(aspath, str):
        return aspath
    return ""


def _bird_aspath(bird, prefix):
    """Return BIRD's AS_PATH string for prefix, or None if not found."""
    output = bird._birdc_quiet("show route for %s all" % prefix)
    for line in output.splitlines():
        if "BGP.as_path" in line:
            return line.split(":", 1)[1].strip()
    return None


def check():
    frr = FRR()
    bird = BIRD()

    frr.wait_session("172.30.0.2")
    bird.wait_session("ze_peer")

    frr.wait_route(PREFIX1)
    bird.wait_route(PREFIX1)

    # AC-6: the FRR peer-group has asn.local 112 + local-options [replace-as]
    # -> AS_PATH origin is 112, not ze's real local AS.
    frr_aspath = _frr_aspath(frr, PREFIX1)
    if frr_aspath.split() != [AS112]:
        log_fail(
            "FRR AS_PATH for %s = %r, expected exactly [%s]"
            % (PREFIX1, frr_aspath, AS112)
        )
        raise AssertionError("AS112 origin override not observed on FRR wire")
    log_pass("FRR observes AS_PATH origin 112 (asn.local replace-as honored)")

    # AC-7: the BIRD peer-group has no asn.local override -> AS_PATH carries
    # ze's real local AS, controlled independently of the FRR peer-group.
    bird_aspath = _bird_aspath(bird, PREFIX1)
    if bird_aspath is None:
        log_fail("BIRD route %s has no AS_PATH line (cannot verify)" % PREFIX1)
        raise AssertionError("no AS_PATH found for %s on BIRD" % PREFIX1)
    if bird_aspath.split() != [ZE_REAL_AS]:
        log_fail(
            "BIRD AS_PATH for %s = %r, expected exactly [%s]"
            % (PREFIX1, bird_aspath, ZE_REAL_AS)
        )
        raise AssertionError("ze's real local AS not observed on BIRD wire")
    log_pass(
        "BIRD observes ze's real local AS %s (no override, independent of FRR group)"
        % ZE_REAL_AS
    )

    # Second AS112 covering prefix should be consistent on both sides.
    frr.check_route(PREFIX2)
    bird.check_route(PREFIX2)

    assert frr.session_established("172.30.0.2"), "FRR session dropped"
    assert bird.session_established("ze_peer"), "BIRD session dropped"
    log_pass("AS112 origin-AS override interoperates correctly with FRR and BIRD")
