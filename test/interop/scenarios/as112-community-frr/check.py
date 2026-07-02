#!/usr/bin/env python3
"""Scenario: AS112 community wire content -- spec-as112-3 AC-4/AC-5.

Validates: a real external FRR peer observes the configured well-known BGP
community on the AS112 covering-prefix routes it receives -- NO_EXPORT
(RFC 1997) on 192.175.48.0/24 (AC-4) and NOPEER (RFC 3765) on
192.31.196.0/24 (AC-5).
Prevents: the configured community leaf-list being internally-consistent-only
within ze's own test harness without surviving onto the real wire.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP, log_pass

PREFIX_NO_EXPORT = "192.175.48.0/24"
PREFIX_NOPEER = "192.31.196.0/24"


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)

    frr.wait_route(PREFIX_NO_EXPORT)
    frr.check_route(PREFIX_NO_EXPORT)
    frr.check_route(PREFIX_NOPEER)

    # AC-4: NO_EXPORT (RFC 1997, 0xFFFFFF01).
    frr.check_route_community(PREFIX_NO_EXPORT, "no-export")

    # AC-5: NOPEER (RFC 3765, 0xFFFFFF04). Ze's process plugin injects it via
    # its well-known name (nopeer); FRR decodes the wire value and displays
    # it under its own name, "no-peer".
    frr.check_route_community(PREFIX_NOPEER, "no-peer")

    assert frr.session_established(ZE_IP), "session dropped"
    log_pass("AS112 well-known communities (NO_EXPORT, NOPEER) round-tripped correctly")
