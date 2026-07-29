#!/usr/bin/env python3
"""Scenario 14: Ze as route server -- routes forwarded without Ze's ASN.

FRR originates 10.99.0.0/24, Ze relays it to BIRD, and BIRD's own AS_PATH view is asserted.
Two foreign implementations, so the transparency claim is not read back out of Ze.
"""

# RFC requirement: RFC7947-x-1 positive -- a route server does not prepend its own AS to a
# relayed route. Asserted at BIRD, a foreign daemon parsing the wire Ze emitted, rather than
# from Ze's own RIB view: an AS-path transparency claim read back out of the speaker that
# built the path proves the least interesting half of it.
#
# `session/rs-client true` in ze.conf is load-bearing and defaults to FALSE. It is the ONLY
# thing that selects the non-prepending path (reactor_api_forward.go:711 gates on
# `facts.isEBGP && !facts.rsClient`; facts.rsClient comes only from that leaf via
# peer_forward_facts.go:111 <- reactor/config.go:266). Without it this scenario asserts
# route-server transparency of two plain eBGP peers and fails on Ze behaving correctly --
# which is exactly what it did until 2026-07-29.

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, BIRD, log_pass, log_info


def check():
    frr = FRR()
    bird = BIRD()

    frr.wait_session("172.30.0.2")
    bird.wait_session("ze_peer")

    # Wait for FRR's route to propagate through Ze to BIRD.
    log_info("waiting for route to propagate FRR -> Ze -> BIRD...")
    deadline = time.time() + 30
    while time.time() < deadline:
        if bird.has_route("10.99.0.0/24"):
            break
        time.sleep(2)

    bird.check_route("10.99.0.0/24")

    # Verify Ze's ASN (65001) is NOT in the AS_PATH (route server behavior).
    bird.check_route_no_as("10.99.0.0/24", "65001")

    assert frr.session_established("172.30.0.2"), "FRR session dropped"
    log_pass("FRR session stable")
    assert bird.session_established("ze_peer"), "BIRD session dropped"
    log_pass("BIRD session stable")

    log_pass("route server forwarded route without inserting own ASN")
