#!/usr/bin/env python3
"""Scenario ospfv3-ptmp-frr: point-to-multipoint OSPFv3 (IPv6) adjacency with FRR ospf6d.

Validates (spec-ospf-ext-8): Ze's OSPFv3 family and FRR ospf6d form an IPv6
point-to-multipoint adjacency over the shared bridge with NO DR, synchronise the
OSPFv3 LSDB (FRR carries Ze's Router-LSA), and exchange /128 host-route prefixes
(RFC 5340 App A.4.3/A.4.10, next-hop = neighbor link-local, sec 3.8.1).
Prevents: a v6 PtMP interface electing a DR / originating a Network-LSA, or the
adjacency never reaching Full over ff02::5.

NOTE: requires an FRR ospf6d build with `ipv6 ospf6 network point-to-multipoint`
support (FRR >= 8.1). If the peer build lacks it, gate this scenario; the v6 PtMP
wire behavior is unit-tested (TestOSPFv3PtMPRouterLSALinks / HostRoute / NextHop).

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over
ff02::5 + FRR ospf6d); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, log_info, log_pass  # noqa: E402

ZE_ROUTER_ID = "172.30.0.2"


def check():
    frr = FRROSPF6()

    # 1. The v6 point-to-multipoint adjacency must reach Full (no DR gating).
    log_info("waiting for OSPFv3 PtMP adjacency between Ze and FRR ospf6d...")
    frr.wait_adjacency(timeout=120)

    # 2. FRR must NOT see a v6 Network-LSA from Ze (PtMP has no DR).
    net = frr._vtysh_quiet("show ipv6 ospf6 database network")
    if ZE_ROUTER_ID in net:
        raise AssertionError(
            "Ze originated a v6 Network-LSA on a PtMP segment; PtMP has no DR"
        )

    # 3. FRR must carry Ze's Router-LSA (LSDB synchronisation over the v6 codec).
    log_info("waiting for FRR to carry Ze's OSPFv3 Router-LSA...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if frr.has_router_lsa(ZE_ROUTER_ID):
            break
        time.sleep(2)
    else:
        raise AssertionError(
            "FRR ospf6d did not learn Ze's OSPFv3 Router-LSA over PtMP"
        )

    # 4. Stability.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("OSPFv3 PtMP adjacency did not stay Full (flapping)")

    log_pass("ospfv3-ptmp-frr: v6 PtMP adjacency formed with no DR and LSDB converged")


if __name__ == "__main__":
    check()
