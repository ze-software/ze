#!/usr/bin/env python3
"""Scenario 57: RFC 1997 NO_EXPORT on egress, judged by FRR and BIRD.

Validates: a route Ze RECEIVES carrying NO_EXPORT reaches the internal observer
           (BIRD, AS 65001) and never reaches the external one (FRR, AS 65002),
           while a route identical except for an ordinary community reaches both.
Prevents:  a silent outbound route leak. Before this, Ze's egress gate consulted
           only the operator's export filter chain and never inspected community
           values, so a peer's "do not advertise beyond your AS" was ignored.
"""

# RFC requirement: RFC1997-Well-1 positive -- "All routes received carrying a communities attribute containing this value [NO_EXPORT] MUST NOT be advertised outside a BGP confederation boundary" (RFC 1997, Well-known Communities). An independent conforming receiver (FRR) never learns 10.10.0.0/24, while it does learn 10.11.0.0/24 relayed by the same Ze over the same session in the same run.
# RFC requirement: RFC1997-Well-1 negative -- the clause's condition is "outside a BGP confederation boundary", and Ze runs a stand-alone AS, which RFC 1997 says to consider a confederation itself. A second independent receiver INSIDE that boundary (BIRD, AS 65001) learns the same 10.10.0.0/24, so the prohibition is scoped rather than a blanket refusal.

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, BIRD, ZE_IP, log_pass

NO_EXPORT_PREFIX = "10.10.0.0/24"
FENCE_PREFIX = "10.11.0.0/24"


def check():
    frr = FRR()
    bird = BIRD()

    frr.wait_session(ZE_IP)
    bird.wait_session("ze_peer")

    # The CONTROL first, on both observers. Everything below is an absence
    # assertion on FRR, and an absence proves nothing until this run is shown to
    # deliver routes at all.
    frr.wait_route(FENCE_PREFIX)
    frr.check_route(FENCE_PREFIX)
    bird.wait_route(FENCE_PREFIX)
    bird.check_route(FENCE_PREFIX)
    log_pass("both observers learned the ordinary-community route")

    # The INTERNAL half: BIRD is inside the confederation boundary, so it gets
    # the NO_EXPORT route. Waiting for it here also orders the FRR check below:
    # the two routes were relayed back to back, so BIRD holding this one means Ze
    # has already decided what to do with it for every destination.
    bird.wait_route(NO_EXPORT_PREFIX)
    bird.check_route(NO_EXPORT_PREFIX)
    log_pass("the internal observer learned the NO_EXPORT route")

    # The EXTERNAL half: FRR is outside the boundary and must never learn it.
    frr.wait_route_absent(NO_EXPORT_PREFIX)
    frr.route_absent(NO_EXPORT_PREFIX)
    log_pass("the external observer never learned the NO_EXPORT route")

    assert frr.session_established(ZE_IP), "FRR session dropped"
    assert bird.session_established("ze_peer"), "BIRD session dropped"
    log_pass("RFC 1997 NO_EXPORT honored on egress, both sessions stable")
