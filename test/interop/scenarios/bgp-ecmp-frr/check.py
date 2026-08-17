#!/usr/bin/env python3
"""Scenario 34: ECMP -- Ze and GoBGP both advertise the same prefix to FRR.

Validates: FRR installs ECMP with both next-hops when receiving the same
           prefix from Ze and GoBGP (spec-fib-depth-2-ecmp AC-4).
Prevents:  Ze's UPDATE wire encoding being incompatible with ECMP selection
           in real implementations.
"""

import json
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, GoBGP, Ze, ZE_IP, GOBGP_IP, log_info, log_pass, log_fail


PREFIX = "10.100.0.0/24"


def check():
    frr = FRR()
    gobgp = GoBGP()
    ze = Ze()

    frr.wait_session(ZE_IP)
    frr.wait_session(GOBGP_IP)

    log_info("injecting %s from GoBGP..." % PREFIX)
    gobgp.inject_route(PREFIX, nexthop="172.30.0.5")

    log_info("waiting for Ze to announce %s..." % PREFIX)
    frr.wait_route(PREFIX, timeout=30)

    # Wait for both paths to appear (ECMP needs both peers' routes).
    log_info("waiting for ECMP convergence...")
    deadline = time.time() + 30
    ecmp_ok = False
    while time.time() < deadline:
        data = frr.route(PREFIX)
        paths = data.get("paths", [])
        if len(paths) >= 2:
            ecmp_ok = True
            break
        time.sleep(2)

    if not ecmp_ok:
        data = frr.route(PREFIX)
        paths = data.get("paths", [])
        log_fail("FRR has %d path(s) for %s, expected 2 (ECMP)" % (len(paths), PREFIX))
        print("  route data: %s" % json.dumps(data, indent=2)[:500])
        print(ze.logs(20))
        raise AssertionError("FRR did not install ECMP for %s" % PREFIX)

    # Verify the next-hop set.
    nexthops = set()
    data = frr.route(PREFIX)
    for path in data.get("paths", []):
        nh = path.get("nexthops", [{}])
        for n in nh:
            ip = n.get("ip", "")
            if ip:
                nexthops.add(ip)

    expected = {ZE_IP, GOBGP_IP}
    if nexthops != expected:
        log_fail("FRR ECMP next-hops: %s, expected %s" % (nexthops, expected))
        raise AssertionError(
            "FRR ECMP next-hops mismatch: got %s, want %s" % (nexthops, expected)
        )

    log_pass("FRR has ECMP for %s with next-hops %s" % (PREFIX, sorted(nexthops)))
    log_pass("ECMP interop: Ze + GoBGP -> FRR multipath verified")
