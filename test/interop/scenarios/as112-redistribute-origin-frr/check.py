#!/usr/bin/env python3
"""Scenario: AS112 redistribute origin-AS -- spec-as112-3 AC-1/AC-3 wire correctness.

Validates: the REAL as112 plugin (not a hand-authored announce-*.py process
plugin) originates the four AS112 COVERING prefixes into BGP via
`redistribute { destination bgp { import as112 } }`, carrying a single-ASN
AS_PATH = the configured `asn` (default 112). Two real external peers observe
the same originated route:
  - an iBGP FRR peer (remote AS == ze local AS 65001) -> AS_PATH exactly [112]
    (no prepend);
  - an eBGP BIRD peer (remote AS 65003) -> AS_PATH [65001, 112] (ze prepends its
    local AS ahead of the producer's origin AS).
Also validates finding H3: only the /24 COVERING prefixes are announced, never
the /32 host addresses bound on lo (192.175.48.1/32, 192.31.196.1/32).
Prevents: the redistribute producer's origin-AS being internally-consistent-only
within ze's own harness without being wire-correct against real BGP stacks.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD, FRR, ZE_IP, log_fail, log_pass

PREFIX1 = "192.175.48.0/24"
PREFIX2 = "192.31.196.0/24"
HOST1 = "192.175.48.1/32"
HOST2 = "192.31.196.1/32"
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

    frr.wait_session(ZE_IP)
    bird.wait_session("ze_peer")

    frr.wait_route(PREFIX1)
    bird.wait_route(PREFIX1)

    # AC-1/AC-3: the iBGP FRR peer (remote AS == ze local AS) observes the
    # producer's origin AS_PATH exactly, with no prepend -> [112].
    frr_aspath = _frr_aspath(frr, PREFIX1)
    if frr_aspath.split() != [AS112]:
        log_fail(
            "FRR (iBGP) AS_PATH for %s = %r, expected exactly [%s]"
            % (PREFIX1, frr_aspath, AS112)
        )
        raise AssertionError("AS112 redistribute origin not observed on iBGP wire")
    log_pass("FRR (iBGP) observes AS_PATH origin [112] (no prepend)")

    # AC-1/AC-3: the eBGP BIRD peer observes ze's local AS prepended ahead of the
    # producer's origin AS -> [65001, 112].
    bird_aspath = _bird_aspath(bird, PREFIX1)
    if bird_aspath is None:
        log_fail("BIRD route %s has no AS_PATH line (cannot verify)" % PREFIX1)
        raise AssertionError("no AS_PATH found for %s on BIRD" % PREFIX1)
    if bird_aspath.split() != [ZE_REAL_AS, AS112]:
        log_fail(
            "BIRD (eBGP) AS_PATH for %s = %r, expected exactly [%s %s]"
            % (PREFIX1, bird_aspath, ZE_REAL_AS, AS112)
        )
        raise AssertionError("eBGP prepend of ze local AS not observed on BIRD wire")
    log_pass("BIRD (eBGP) observes AS_PATH [65001, 112] (local AS prepended)")

    # The second AS112 covering prefix should be present on both sides.
    frr.check_route(PREFIX2)
    bird.check_route(PREFIX2)

    # Finding H3: only the /24 COVERING prefixes are announced, never the /32
    # host addresses that as112 binds on lo.
    frr.route_absent(HOST1)
    frr.route_absent(HOST2)

    assert frr.session_established(ZE_IP), "FRR session dropped"
    assert bird.session_established("ze_peer"), "BIRD session dropped"
    log_pass("AS112 redistribute origin-AS interoperates correctly with FRR and BIRD")
