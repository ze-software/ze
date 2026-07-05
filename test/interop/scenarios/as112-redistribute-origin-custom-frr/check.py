#!/usr/bin/env python3
"""Scenario: AS112 redistribute custom origin ASN -- spec-as112-3 AC-4.

Validates: setting `service { as112 { asn 65001 } }` makes the redistribute
producer originate the AS112 COVERING prefixes with origin AS 65001 instead of
the RFC 7534 default (112). A real external iBGP FRR peer whose AS (64512) is
distinct from both the configured origin (65001) and the default (112) observes
AS_PATH exactly [65001] -- unambiguously the configured override, since iBGP
does not prepend ze's local AS.
Also validates finding H3: only the /24 COVERING prefixes are announced, never
the /32 host addresses.
Prevents: the `asn` leaf being honored only in ze's own harness without being
wire-correct against a real BGP peer.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP, log_fail, log_pass

PREFIX1 = "192.175.48.0/24"
PREFIX2 = "192.31.196.0/24"
HOST1 = "192.175.48.1/32"
HOST2 = "192.31.196.1/32"
CUSTOM_ASN = "65001"


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


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
    frr.wait_route(PREFIX1)

    # AC-4: the configured `asn 65001` is the origin AS on the wire. iBGP (no
    # prepend) + a distinct ze local AS (64512) makes [65001] unambiguous.
    frr_aspath = _frr_aspath(frr, PREFIX1)
    if frr_aspath.split() != [CUSTOM_ASN]:
        log_fail(
            "FRR (iBGP) AS_PATH for %s = %r, expected exactly [%s]"
            % (PREFIX1, frr_aspath, CUSTOM_ASN)
        )
        raise AssertionError("custom origin ASN not observed on the wire")
    log_pass("FRR (iBGP) observes AS_PATH origin [65001] (configured asn override)")

    frr.check_route(PREFIX2)

    # Finding H3: the /32 host addresses are never announced.
    frr.route_absent(HOST1)
    frr.route_absent(HOST2)

    assert frr.session_established(ZE_IP), "FRR session dropped"
    log_pass("AS112 redistribute custom origin ASN interoperates correctly with FRR")
