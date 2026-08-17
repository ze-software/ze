#!/usr/bin/env python3
"""Scenario ospfv3-nssa-redist-frr: OSPFv3 NSSA redistribution to FRR.

Validates spec-ospfv3-5 Part B: a real BGP peering feeds Ze an IPv6 route, Ze
redistributes it into an OSPFv3 NSSA as a Type-7 NSSA-LSA (0x2007), FRR installs
it as an NSSA route, and the NSSA does not receive a Type-5 AS-External-LSA leak.
"""

import os
import re
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


REDISTRIBUTED_PREFIX = "2001:db8:7e5::/48"


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


def assert_ospf6_nssa_route(frr, prefix):
    ospf6_routes = frr._vtysh_quiet("show ipv6 ospf6 route")
    nssa_code = re.compile(r"(^|[\s>*])N([12]|\s|$)")
    for line in ospf6_routes.splitlines():
        if prefix not in line:
            continue
        text = line.strip()
        if nssa_code.search(text) or "NSSA" in text:
            return
    print(ospf6_routes[:800])
    print(frr._vtysh_quiet("show ipv6 route ospf6")[:800])
    raise AssertionError(f"FRR installed {prefix}, but not as an OSPFv3 NSSA route")


def assert_no_type5_leak(frr, prefix):
    external_db = frr._vtysh_quiet("show ipv6 ospf6 database external")
    if prefix in external_db:
        print(external_db[:800])
        raise AssertionError(f"Type-5 AS-External-LSA for {prefix} leaked into the NSSA")


def check():
    frr = FRROSPF6()
    gobgp = GoBGP()

    log_info("waiting for the OSPFv3 NSSA adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=90)

    log_info("waiting for GoBGP to establish the redistribution-source peering...")
    gobgp.wait_session(ZE_IP)

    log_info(f"injecting {REDISTRIBUTED_PREFIX} from GoBGP into Ze...")
    inject_gobgp_ipv6(REDISTRIBUTED_PREFIX)

    log_info(f"waiting for FRR to install NSSA route {REDISTRIBUTED_PREFIX}...")
    frr.wait_ospf6_route(REDISTRIBUTED_PREFIX, timeout=90)
    assert_ospf6_nssa_route(frr, REDISTRIBUTED_PREFIX)
    assert_no_type5_leak(frr, REDISTRIBUTED_PREFIX)

    log_pass(
        "ospfv3-nssa-redist-frr: FRR installed Ze's redistributed IPv6 prefix as "
        "an NSSA route with no Type-5 leak"
    )


if __name__ == "__main__":
    check()
