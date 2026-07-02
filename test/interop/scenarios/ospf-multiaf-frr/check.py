#!/usr/bin/env python3
"""Scenario ospf-multiaf-frr: RFC 5838 multi-AF interop with FRR ospf6d.

Validates (spec-ospf-ext-15 AC-11 / AC-2): Ze runs two OSPFv3 address families
(IPv6-unicast at Instance ID 0 and IPv4-unicast-over-OSPFv3 at Instance ID 64) and
therefore sets the RFC 5838 AF-bit in its Hello/DD Options. FRR ospf6d runs only the
default IPv6-unicast AF and, per RFC 5838 §2.6, ignores the AF-bit -- so the IPv6-unicast
adjacency must still reach Full and synchronise the LSDB. This proves multi-AF does not
regress the IPv6-base interop (the AF-bit is transparent to a legacy IPv6-unicast peer).

Prevents: the AF-bit breaking the default IPv6-unicast adjacency with FRR, or the
multi-AF spawn interfering with the IPv6-unicast instance.

FRR ospf6d does not implement the IPv4-unicast AF (assumption A-9), so the IPv4-unicast
instance has no FRR peer here; it is validated by ospf-multiaf-v4-frr (Ze<->Ze fallback).

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over ff02::5 +
FRR ospf6d); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, log_info, log_pass  # noqa: E402

ZE_ROUTER_ID = "172.30.0.2"


def check():
    frr = FRROSPF6()

    # 1. The default IPv6-unicast OSPFv3 adjacency must reach Full despite Ze setting the
    #    RFC 5838 AF-bit (FRR ignores it, §2.6).
    log_info("waiting for the IPv6-unicast OSPFv3 adjacency (Ze multi-AF <-> FRR)...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR must learn Ze's Router-LSA (LSDB synchronised over the IPv6-unicast instance).
    log_info("waiting for FRR to learn Ze's OSPFv3 Router-LSA...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if frr.has_router_lsa(ZE_ROUTER_ID):
            break
        time.sleep(2)
    else:
        print(frr._vtysh_quiet("show ipv6 ospf6 database router")[:800])
        raise AssertionError("FRR did not learn Ze's Router-LSA with the AF-bit set")

    # 3. Stability: the adjacency stays Full after a short settle.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError(
            "IPv6-unicast adjacency did not stay Full with multi-AF enabled"
        )

    log_pass(
        "ospf-multiaf-frr: IPv6-unicast adjacency Full with the AF-bit set (RFC 5838 §2.6)"
    )


if __name__ == "__main__":
    check()
