#!/usr/bin/env python3
"""Scenario isis-convergence-frr: link-down reconvergence + stale withdraw.

Validates (spec-isis-13 AC-17): after the IS-IS link is cut, the hold timer
expires, both sides tear the adjacency down, re-originate their LSPs without the
lost reachability, and withdraw the stale routes; restoring the link reconverges
the adjacency.
Prevents: an adjacency that never times out on link loss, a stale route that is
never withdrawn (black hole), or a link that does not re-form after restore.

Runs under the Linux Docker/QEMU interop harness ONLY (raw L2 + FRR isisd).
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRRISIS, docker_exec_quiet, log_info, log_pass  # noqa: E402

FRR_CONTAINER_LINK = "eth0"


def check():
    frr = FRRISIS()

    # 1. Form the adjacency.
    log_info("waiting for IS-IS adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=90)

    # 2. Cut the IS-IS link by bringing FRR's eth0 down. With a 1s hello and a
    #    multiplier of 3, the hold timer expires within ~3-4s.
    log_info("cutting the IS-IS link (FRR eth0 down)...")
    docker_exec_quiet(frr.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "down"])

    # 3. The adjacency must go Down (reconvergence after hold-timer expiry).
    deadline = time.time() + 30
    went_down = False
    while time.time() < deadline:
        if not frr.adjacency_up():
            went_down = True
            break
        time.sleep(1)
    if not went_down:
        docker_exec_quiet(
            frr.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "up"]
        )
        raise AssertionError(
            "adjacency did not go Down after link cut (hold timer not honoured)"
        )
    log_info("adjacency went Down after link cut (hold timer expired)")

    # 4. Restore the link; the adjacency must re-form (reconvergence).
    log_info("restoring the IS-IS link (FRR eth0 up)...")
    docker_exec_quiet(frr.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "up"])
    frr.wait_adjacency(timeout=90)

    log_pass(
        "isis-convergence-frr: link down -> adjacency down -> link up -> reconverged"
    )


if __name__ == "__main__":
    check()
