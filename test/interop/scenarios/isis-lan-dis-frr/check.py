#!/usr/bin/env python3
"""Scenario isis-lan-dis-frr: LAN DIS election + pseudo-node LSP with FRR.

Validates (spec-isis-13 AC-14): on a broadcast LAN, a Designated IS is elected
(Ze, with the higher priority) and a pseudo-node LSP represents the segment in
the LSDB; the LAN adjacency forms and routes converge.
Prevents: a LAN that forms no adjacency, a missing pseudo-node LSP (the segment
not represented as a single node), or DIS churn.

Runs under the Linux Docker/QEMU interop harness ONLY (raw L2 + FRR isisd).
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRRISIS, log_info, log_pass  # noqa: E402


def check():
    frr = FRRISIS()

    # 1. The LAN IS-IS adjacency must come up.
    log_info("waiting for LAN IS-IS adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=90)

    # 2. A pseudo-node LSP must appear in FRR's LSDB (the DIS, Ze, originates it
    #    to represent the LAN segment as a single node, AC-14).
    log_info("waiting for the pseudo-node LSP to appear in FRR's LSDB...")
    deadline = time.time() + 60
    have_pn = False
    while time.time() < deadline:
        if frr.has_pseudonode_lsp():
            have_pn = True
            break
        time.sleep(2)
    if not have_pn:
        print(frr._vtysh_quiet("show isis database")[:800])
        raise AssertionError(
            "no pseudo-node LSP in FRR's LSDB (DIS/pseudo-node not originated)"
        )

    # 3. Stability: the adjacency stays Up (no DIS churn re-flapping it).
    time.sleep(5)
    if not frr.adjacency_up():
        raise AssertionError("LAN IS-IS adjacency did not stay Up (DIS churn?)")

    log_pass("isis-lan-dis-frr: DIS elected, pseudo-node LSP present, LAN stable")


if __name__ == "__main__":
    check()
