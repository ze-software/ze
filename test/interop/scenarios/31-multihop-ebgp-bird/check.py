#!/usr/bin/env python3
"""Scenario 31: Multi-hop eBGP with BIRD using outgoing-ttl.

Validates: Ze's outgoing-ttl config works with BIRD's multihop setting.
Prevents:  TTL configuration bugs with BIRD as peer.
"""

import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD, log_pass


def check():
    bird = BIRD()

    bird.wait_session("ze_peer")
    log_pass("multi-hop eBGP session established with BIRD (outgoing-ttl 2)")

    bird.wait_route("10.10.0.0/24")
    bird.check_route("10.10.0.0/24")
    bird.check_route("10.10.1.0/24")

    assert bird.session_established("ze_peer"), "session dropped after route exchange"
    log_pass("multi-hop eBGP routes received by BIRD, session stable")
