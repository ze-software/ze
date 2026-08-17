#!/usr/bin/env python3
"""Scenario 36: remove-private-as export policy, FRR destination.

Validates: Ze strips RFC 6996 Private Use ASN 64512 from an exported route
before advertising it to FRR, then applies normal EBGP local-AS prepend.
Prevents: FRR receiving leaked private ASNs or an UPDATE rejected after policy
AS_PATH rewrite.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, GoBGP, ZE_IP, log_fail, log_info, log_pass


def _route_aspath(frr, prefix):
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
    gobgp = GoBGP()

    frr.wait_session("172.30.0.2")
    gobgp.wait_session(ZE_IP)

    log_info("injecting private-AS route from GoBGP...")
    gobgp.inject_route("10.99.0.0/24")

    frr.wait_route("10.99.0.0/24")
    frr.check_route("10.99.0.0/24")
    frr.check_route_no_as("10.99.0.0/24", 64512)

    aspath = _route_aspath(frr, "10.99.0.0/24")
    if "65001" not in aspath.split():
        log_fail("FRR route AS_PATH %r does not contain Ze local AS 65001" % aspath)
        raise AssertionError("Ze local AS missing from exported AS_PATH")
    log_pass("FRR route AS_PATH contains Ze local AS 65001 after stripping")

    assert frr.session_established("172.30.0.2"), "FRR session dropped"
    assert gobgp.session_established(ZE_IP), "GoBGP session dropped"
    log_pass("remove-private-as export policy interoperates with FRR")
