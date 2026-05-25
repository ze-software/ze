#!/usr/bin/env python3
"""Scenario 37: remove-private-as strips private ASNs carried via AS4_PATH.

Validates: A route from a four-octet private ASN source is exported to FRR
without leaking that private ASN when Ze receives it on a session where ASN4 is
disabled and AS4_PATH is used for reconstruction.
Prevents: RFC 6996 AS4_PATH stripping being skipped while AS_PATH stripping
appears to work.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD, BIRD_CONTAINER, FRR, docker_exec, log_fail, log_info, log_pass


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
    bird = BIRD()

    frr.wait_session("172.30.0.2")
    bird.wait_session("ze_peer")

    log_info("enabling BIRD AS4_PATH static route...")
    docker_exec(BIRD_CONTAINER, ["birdc", "enable static_routes"])

    frr.wait_route("10.99.1.0/24")
    frr.check_route("10.99.1.0/24")
    frr.check_route_no_as("10.99.1.0/24", 4200000000)

    aspath = _route_aspath(frr, "10.99.1.0/24")
    if "65001" not in aspath.split():
        log_fail("FRR route AS_PATH %r does not contain Ze local AS 65001" % aspath)
        raise AssertionError("Ze local AS missing from exported AS_PATH")
    log_pass("FRR route AS_PATH contains Ze local AS and not the AS4 private ASN")

    assert frr.session_established("172.30.0.2"), "FRR session dropped"
    assert bird.session_established("ze_peer"), "BIRD session dropped"
    log_pass("remove-private-as AS4_PATH handling interoperates with FRR")
