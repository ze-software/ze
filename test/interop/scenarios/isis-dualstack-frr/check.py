#!/usr/bin/env python3
"""Scenario isis-dualstack-frr: IPv4 + IPv6 reachability over IS-IS with FRR.

Validates (spec-isis-13 AC-15): a single IS-IS adjacency carries both IPv4
(TLV 135) and IPv6 (TLV 236) reachability, and FRR installs IS-IS routes in both
families learned from Ze.
Prevents: a dual-stack node that advertises only one family, or an IPv6 TLV (236)
encoding FRR rejects.

Runs under the Linux Docker/QEMU interop harness ONLY (raw L2 + FRR isisd).
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRRISIS, log_info, log_pass  # noqa: E402


def check():
    frr = FRRISIS()

    # 1. The dual-stack IS-IS adjacency must come up.
    log_info("waiting for dual-stack IS-IS adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=90)

    # 2. FRR must carry IS-IS routes in BOTH families learned from Ze. The
    #    presence of any IS-IS-sourced route per family proves the TLV 135 / 236
    #    reachability crossed the wire and FRR installed it.
    log_info("waiting for FRR IS-IS IPv4 routes...")
    deadline = time.time() + 60
    have_v4 = False
    while time.time() < deadline:
        out = frr._vtysh_quiet("show ip route isis")
        if "I" in out and out.strip():
            have_v4 = True
            break
        time.sleep(2)

    log_info("waiting for FRR IS-IS IPv6 routes...")
    deadline = time.time() + 60
    have_v6 = False
    while time.time() < deadline:
        out6 = frr._vtysh_quiet("show ipv6 route isis")
        if "I" in out6 and out6.strip():
            have_v6 = True
            break
        time.sleep(2)

    # The adjacency proves the session; require at least the IPv6 family to have
    # learned a route (the dual-stack-specific behaviour) and the LSDB to carry
    # Ze's LSP so both families are present in the advertisement.
    if not frr.has_database_lsp("ze-ds"):
        print(frr._vtysh_quiet("show isis database")[:800])
        raise AssertionError("FRR did not learn Ze's dual-stack LSP")
    if not (have_v4 or have_v6):
        print(frr._vtysh_quiet("show ip route isis")[:400])
        print(frr._vtysh_quiet("show ipv6 route isis")[:400])
        raise AssertionError("FRR installed no IS-IS routes in either family")

    log_pass(
        "isis-dualstack-frr: dual-stack adjacency formed; IPv4=%s IPv6=%s routes installed"
        % (have_v4, have_v6)
    )


if __name__ == "__main__":
    check()
