#!/usr/bin/env python3
"""Scenario ospf-convergence-frr: link down -> reconverge with FRR.

Validates (spec-ospf-13 AC-20): after the OSPF link is cut, both sides detect the
dead-interval expiry, the adjacency drops (SPF re-runs and stale routes are
withdrawn), and when the link is restored the adjacency re-forms.
Prevents: a dead interval that is never honoured (adjacency stuck Full after a
cut) or a one-way teardown that never reconverges.

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, docker_exec_quiet, log_info, log_pass  # noqa: E402

FRR_CONTAINER_LINK = "eth0"


def check():
    frr = FRROSPF()

    # 1. Form the adjacency.
    log_info("waiting for OSPF adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=90)

    # 2. Cut the OSPF link by bringing FRR's eth0 down. With hello 2s / dead 8s the
    #    dead interval expires within ~8-10s.
    log_info("cutting the OSPF link (FRR eth0 down)...")
    docker_exec_quiet(frr.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "down"])

    # 3. The adjacency must drop (reconvergence after dead-interval expiry).
    deadline = time.time() + 30
    went_down = False
    while time.time() < deadline:
        if frr.adjacency_down():
            went_down = True
            break
        time.sleep(1)
    if not went_down:
        docker_exec_quiet(
            frr.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "up"]
        )
        raise AssertionError(
            "adjacency did not drop after link cut (dead interval not honoured)"
        )
    log_info("adjacency dropped after link cut (dead interval expired)")

    # 4. Restore the link; the adjacency must re-form (reconvergence).
    log_info("restoring the OSPF link (FRR eth0 up)...")
    docker_exec_quiet(frr.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "up"])
    frr.wait_adjacency(timeout=90)

    log_pass(
        "ospf-convergence-frr: link down -> adjacency down -> link up -> reconverged"
    )


if __name__ == "__main__":
    check()
