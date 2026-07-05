#!/usr/bin/env python3
"""Scenario: AS112 redistribute community wire content -- spec-as112-3 AC-5.

Validates: `service { as112 { community [ nopeer ] } }` makes the redistribute
producer attach NOPEER (RFC 3765, 0xFFFFFF04) to every announced AS112 COVERING
prefix, and a real external FRR peer decodes it off the wire (65535:65284) and
displays it under its own name, "no-peer".
Prevents: the configured community leaf-list being internally-consistent-only
within ze's own harness without surviving onto the real wire.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP, log_pass

PREFIX1 = "192.175.48.0/24"
PREFIX2 = "192.31.196.0/24"


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)

    frr.wait_route(PREFIX1)
    frr.check_route(PREFIX1)
    frr.check_route(PREFIX2)

    # AC-5: NOPEER (RFC 3765, 0xFFFFFF04) is attached to every covering prefix by
    # the producer's single community list. Ze emits it via its well-known name
    # (nopeer); FRR decodes the wire value and displays it as "no-peer".
    frr.check_route_community(PREFIX1, "no-peer")
    frr.check_route_community(PREFIX2, "no-peer")

    assert frr.session_established(ZE_IP), "session dropped"
    log_pass("AS112 redistribute NOPEER community round-tripped correctly onto the wire")
