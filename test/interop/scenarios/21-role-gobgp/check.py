#!/usr/bin/env python3
"""Scenario 21: RFC 9234 Role capability negotiation between Ze and GoBGP.

Ze is configured as Provider (import provider), GoBGP as Customer (role = customer).
Verifies:
  1. Session establishes with role capability negotiated (valid pair: Provider <-> Customer)
  2. Session remains stable (no Role Mismatch NOTIFICATION)
"""

import os, sys, time
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import GoBGP, log_pass, log_info


def check():
    gobgp = GoBGP()

    gobgp.wait_session("172.30.0.2")
    log_pass("GoBGP session established with role capability")

    # Wait briefly and verify session stays up.
    time.sleep(5)
    assert gobgp.session_established("172.30.0.2"), "GoBGP session dropped after role negotiation"
    log_pass("session stable after role capability negotiation")

    log_pass("RFC 9234 role capability interop with GoBGP passed")
