#!/usr/bin/env python3
"""Scenario ospf-ldp-sync-frr: RFC 5443 LDP-IGP synchronization interop with FRR.

Validates (spec-ospf-ext-11): FRR observes Ze advertise the eth0 point-to-point link
at LSInfinity (0xFFFF) while LDP is converging on that link, and at the configured cost
(10) after the LDP session is operational and Ze's hold-down expires. RFC 5443 defines
no wire format, so this validates the externally observable Router-LSA metric that
FRR's SPF acts on -- not a protocol negotiation.

Prevents: Ze never costing out the un-labeled link (transient black hole), or never
restoring the cost after LDP synchronizes (link costed out forever).

Runs under the Linux Docker/QEMU interop harness ONLY: it needs FRR ospfd + ldpd, the
kernel MPLS modules, and raw IP proto 89 + the LDP TCP/UDP paths, none of which run on
darwin. Authored-pending-QEMU.
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, log_info, log_pass, log_fail  # noqa: E402

ZE_ROUTER_ID = "172.30.0.2"
FRR_ROUTER_ID = "172.30.0.3"
CONFIGURED_COST = 10
LS_INFINITY = 65535


def ze_p2p_metric(frr):
    """Return the metric of Ze's point-to-point link to FRR in FRR's LSDB, or None.

    Parses `show ip ospf database router <ze-rid>`: the point-to-point link block whose
    Neighboring Router ID is FRR carries `TOS 0 Metric: N`.
    """
    out = frr._vtysh_quiet("show ip ospf database router %s" % ZE_ROUTER_ID)
    # Split into per-link blocks; find the point-to-point link to FRR.
    blocks = re.split(r"Link connected to:", out)
    for b in blocks:
        if "point-to-point" not in b:
            continue
        if FRR_ROUTER_ID not in b:
            continue
        m = re.search(r"TOS 0 Metric:\s*(\d+)", b)
        if m:
            return int(m.group(1))
    return None


def wait_metric(frr, want, timeout):
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        last = ze_p2p_metric(frr)
        if last == want:
            return
        time.sleep(2)
    log_fail(
        "Ze eth0 link metric in FRR LSDB = %r, want %d after %ds"
        % (last, want, timeout)
    )
    print(frr._vtysh_quiet("show ip ospf database router %s" % ZE_ROUTER_ID)[:800])
    raise AssertionError("Ze eth0 link metric never reached %d" % want)


def check():
    frr = FRROSPF()

    # 1. The P2P OSPF adjacency must reach Full so FRR carries Ze's Router-LSA.
    frr.wait_adjacency(timeout=90)

    # 2. While LDP is converging, Ze costs the link out to LSInfinity: FRR must observe
    #    0xFFFF for the eth0 link before the hold-down (20s) restores it.
    log_info("expecting Ze to advertise eth0 at LSInfinity while LDP converges...")
    wait_metric(frr, LS_INFINITY, timeout=45)
    log_pass("FRR observes Ze eth0 at LSInfinity (0xFFFF) during LDP convergence")

    # 3. After the LDP session is operational and the hold-down expires, Ze restores the
    #    configured cost: FRR must observe the link at 10.
    log_info(
        "expecting Ze to restore the configured cost after LDP sync + hold-down..."
    )
    wait_metric(frr, CONFIGURED_COST, timeout=120)
    log_pass(
        "FRR observes Ze eth0 restored to the configured cost %d" % CONFIGURED_COST
    )

    log_pass(
        "ospf-ldp-sync-frr: LDP-IGP sync cost-out and restore interoperate with FRR"
    )


if __name__ == "__main__":
    check()
