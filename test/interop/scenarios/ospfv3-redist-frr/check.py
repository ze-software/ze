#!/usr/bin/env python3
"""Scenario ospfv3-redist-frr: OSPFv3 ASBR redistribution to FRR.

Validates spec-ospfv3-5 Part B: a real BGP peering feeds Ze an IPv6 route, Ze
redistributes it into OSPFv3 as an AS-External-LSA (0x4005), and FRR installs the
route from OSPFv3. This prevents an adjacency-only scenario from passing without
exercising the redistribution framework.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRROSPF6,
    GOBGP_CONTAINER,
    GoBGP,
    ZE_IP,
    docker_exec,
    docker_exec_quiet,
    log_info,
    log_pass,
)


REDISTRIBUTED_PREFIX = "2001:db8:5e5::/48"


def gobgp_v6_nexthop():
    out = docker_exec_quiet(
        GOBGP_CONTAINER,
        ["ip", "-6", "-o", "addr", "show", "dev", "eth0", "scope", "global"],
    )
    for token in out.split():
        if ":" in token and "/" in token:
            return token.split("/", 1)[0]
    raise AssertionError("GoBGP has no global IPv6 address to use as MP_REACH nexthop")


def inject_gobgp_ipv6(prefix):
    nh = gobgp_v6_nexthop()
    docker_exec(
        GOBGP_CONTAINER,
        ["gobgp", "global", "rib", "add", prefix, "-a", "ipv6", "nexthop", nh],
    )


def check():
    frr = FRROSPF6()
    gobgp = GoBGP()

    log_info("waiting for the OSPFv3 adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=90)

    log_info("waiting for GoBGP to establish the redistribution-source peering...")
    gobgp.wait_session(ZE_IP)

    log_info(f"injecting {REDISTRIBUTED_PREFIX} from GoBGP into Ze...")
    inject_gobgp_ipv6(REDISTRIBUTED_PREFIX)

    log_info(f"waiting for FRR to install redistributed OSPFv3 route {REDISTRIBUTED_PREFIX}...")
    frr.wait_ospf6_route(REDISTRIBUTED_PREFIX, timeout=90)
    log_pass(
        "ospfv3-redist-frr: FRR installed Ze's BGP-sourced IPv6 route from OSPFv3"
    )


if __name__ == "__main__":
    check()
