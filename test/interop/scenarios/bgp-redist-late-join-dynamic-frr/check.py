#!/usr/bin/env python3
"""Scenario bgp-redist-late-join-dynamic-frr: dynamic/inbound redistribute late-join.

Validates: a genuinely-new DYNAMIC inbound peer (FRR, accepted into a Ze dynamic
  peer group on port 179) receives a redistribute-injected route that was
  originated BEFORE the peer connected. FRR is absent from Ze's reactor map at
  redistribute-emit time (Ze startup), has no PluginRoutes and no ribOut, so the
  ONLY path that can feed it 10.99.0.0/24 is the redistribute late-join replay
  fired on the dynamic peer's session establishment.
Prevents:  the late-join replay working in unit/native tests but not against a
  real BGP implementation dialing in as a dynamic peer over the wire.

Runs under the Linux Docker interop harness ONLY.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRR, Ze, ZE_IP, FRR_IP, log_fail, log_info  # noqa: E402

REDIST_ROUTE = "10.99.0.0/24"


def check():
    frr = FRR()
    ze = Ze()

    # FRR dials into Ze's dynamic group and establishes as dyn-<FRR_IP>.
    frr.wait_session(ZE_IP)

    # FRR is accepted as a DYNAMIC peer: no configured peer has remote 172.30.0.3
    # (the anchor is 172.30.0.99), so an established session from 172.30.0.3 can
    # only be a dynamic peer created from the group range. Log the dynamic-peer
    # line if present, informationally.
    logs = ze.logs(200)
    if "dyn-" in logs:
        log_info("Ze created a dynamic peer for FRR (dyn-...)")

    # The static route was redistributed at Ze startup, before FRR connected, so
    # it can only reach this genuinely-new peer via the peer-up replay.
    log_info("waiting for the redistribute late-join replay to reach FRR...")
    deadline = time.time() + 40
    while time.time() < deadline:
        if frr.has_route(REDIST_ROUTE):
            break
        time.sleep(2)
    else:
        log_fail("FRR (dynamic peer) did not receive %s within 40s" % REDIST_ROUTE)
        print(ze.logs(40))
        raise AssertionError("dynamic peer did not receive the redistributed route")

    frr.check_route(REDIST_ROUTE)
    log_info("dynamic inbound peer received the redistributed route via replay")
