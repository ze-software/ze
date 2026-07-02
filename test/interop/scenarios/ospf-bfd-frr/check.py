#!/usr/bin/env python3
"""Scenario ospf-bfd-frr: RFC 5881 IPv4 single-hop BFD for OSPFv2 with FRR.

Validates (spec-ospf-ext-10 AC-2, AC-5, user story 1 & 3):
  1. Ze and FRR form a Full OSPFv2 adjacency.
  2. On reaching Full, Ze opens an IPv4 single-hop BFD session and FRR's bfdd
     reports it Up (the three-way handshake completed against Ze's engine).
  3. Cutting the link drives an OSPF neighbor-down within the BFD detection
     window (a few seconds), well under the 40s RouterDeadInterval -- proving
     BFD, not the dead timer, declared the neighbor dead.
  4. Restoring the link re-forms both the adjacency and the BFD session.

Prevents: a BFD session that never comes Up against an independent implementation,
or a link cut that only reconverges after the 40s dead interval (BFD not driving
the down).

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
AUTHORED PENDING QEMU: not yet executed in CI.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRR,
    FRROSPF,
    docker_exec_quiet,
    log_info,
    log_pass,
)

ZE_PEER_IP = "172.30.0.2"
FRR_CONTAINER_LINK = "eth0"
# BFD must detect well before the 40s dead interval; allow generous slack for the
# RFC 5880 slow-start floor (up to multiplier * 1s until fast rates negotiate).
BFD_DETECT_MAX_SECONDS = 15


def check():
    frr_ospf = FRROSPF()
    frr_bfd = FRR()

    # 1. Full OSPFv2 adjacency.
    log_info("waiting for OSPFv2 adjacency between Ze and FRR...")
    frr_ospf.wait_adjacency(timeout=90)

    # 2. BFD session Up (Ze opened the single-hop session on reaching Full).
    frr_bfd.wait_bfd_up(ZE_PEER_IP, timeout=60)

    # 3. Cut the link; the adjacency must drop within the BFD window, not the dead timer.
    log_info(
        "cutting the OSPF link (FRR eth0 down); expecting sub-dead-interval down..."
    )
    cut_at = time.time()
    docker_exec_quiet(
        frr_bfd.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "down"]
    )
    deadline = cut_at + BFD_DETECT_MAX_SECONDS
    went_down = False
    while time.time() < deadline:
        if frr_ospf.adjacency_down():
            went_down = True
            break
        time.sleep(0.5)
    detect = time.time() - cut_at
    if not went_down:
        docker_exec_quiet(
            frr_bfd.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "up"]
        )
        raise AssertionError(
            "adjacency did not drop within %ds of the link cut; BFD did not drive the down"
            % BFD_DETECT_MAX_SECONDS
        )
    log_info(
        "adjacency dropped %.1fs after the cut (well under the 40s dead interval)"
        % detect
    )

    # 4. Restore the link; adjacency and BFD session must re-form.
    log_info("restoring the OSPF link (FRR eth0 up)...")
    docker_exec_quiet(
        frr_bfd.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "up"]
    )
    frr_ospf.wait_adjacency(timeout=90)
    frr_bfd.wait_bfd_up(ZE_PEER_IP, timeout=60)

    log_pass(
        "ospf-bfd-frr: adjacency Full + BFD Up -> link cut -> down in %.1fs (< dead 40s) -> reconverged"
        % detect
    )


if __name__ == "__main__":
    check()
