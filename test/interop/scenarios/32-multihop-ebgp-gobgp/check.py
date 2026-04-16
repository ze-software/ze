#!/usr/bin/env python3
"""Scenario 32: Multi-hop eBGP with GoBGP using outgoing-ttl.

Validates: Ze's outgoing-ttl config works with GoBGP's ebgp-multihop.
Prevents:  TTL configuration bugs with GoBGP as peer.
"""

import os, sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import GoBGP, ZE_IP, log_pass


def check():
    gobgp = GoBGP()

    gobgp.wait_session(ZE_IP)
    log_pass("multi-hop eBGP session established with GoBGP (outgoing-ttl 2)")

    gobgp.wait_route("10.10.0.0/24")
    gobgp.check_route("10.10.0.0/24")
    gobgp.check_route("10.10.1.0/24")

    assert gobgp.session_established(ZE_IP), "session dropped after route exchange"
    log_pass("multi-hop eBGP routes received by GoBGP, session stable")
