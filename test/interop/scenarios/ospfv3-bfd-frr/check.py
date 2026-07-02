#!/usr/bin/env python3
"""Scenario ospfv3-bfd-frr: RFC 5881 IPv6 single-hop BFD for OSPFv3 with FRR.

Validates (spec-ospf-ext-10 AC-2b, AC-5, user story 2 & 3):
  1. Ze and FRR form a Full OSPFv3 adjacency over IPv6 link-local.
  2. On reaching Full, Ze opens an IPv6 single-hop BFD session (link-local pair,
     Hop-Limit 255) and FRR's bfdd reports a peer Up.
  3. Cutting the link drives a v3 neighbor-down within the BFD detection window,
     well under the 40s RouterDeadInterval.
  4. Restoring the link re-forms the v3 adjacency and the BFD session.

The IPv6 BFD peer is Ze's link-local (dynamic), so the BFD-up check asserts FRR
reports any peer Up rather than matching a fixed address.

Prevents: a v6 BFD session that never comes Up (wrong link-local pair / GTSM), or a
link cut that only reconverges after the dead interval.

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
AUTHORED PENDING QEMU: not yet executed in CI.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRR,
    FRROSPF6,
    docker_exec_quiet,
    log_info,
    log_pass,
)

FRR_CONTAINER_LINK = "eth0"
BFD_DETECT_MAX_SECONDS = 15


def wait_bfd_up_any(frr, timeout=60):
    """Poll until FRR's bfdd reports at least one BFD peer Up (the v6 link-local
    peer address is dynamic, so match on Status rather than a fixed address)."""
    log_info("waiting for FRR to report a BFD peer Up (timeout %ds)..." % timeout)
    deadline = time.time() + timeout
    while time.time() < deadline:
        out = frr._vtysh_quiet("show bfd peers")
        if "Status: up" in out:
            log_pass("FRR reports a BFD peer Up")
            return
        time.sleep(2)
    raise AssertionError("no FRR BFD peer reached Up within %ds" % timeout)


def check():
    frr_ospf6 = FRROSPF6()
    frr_bfd = FRR()

    # 1. Full OSPFv3 adjacency.
    log_info("waiting for OSPFv3 adjacency between Ze and FRR...")
    frr_ospf6.wait_adjacency(timeout=90)

    # 2. IPv6 single-hop BFD session Up.
    wait_bfd_up_any(frr_bfd, timeout=60)

    # 3. Cut the link; the adjacency must drop within the BFD window, not the dead timer.
    log_info(
        "cutting the OSPFv3 link (FRR eth0 down); expecting sub-dead-interval down..."
    )
    cut_at = time.time()
    docker_exec_quiet(
        frr_bfd.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "down"]
    )
    deadline = cut_at + BFD_DETECT_MAX_SECONDS
    went_down = False
    while time.time() < deadline:
        if not frr_ospf6.adjacency_full():
            went_down = True
            break
        time.sleep(0.5)
    detect = time.time() - cut_at
    if not went_down:
        docker_exec_quiet(
            frr_bfd.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "up"]
        )
        raise AssertionError(
            "v3 adjacency did not drop within %ds of the link cut; BFD did not drive the down"
            % BFD_DETECT_MAX_SECONDS
        )
    log_info(
        "v3 adjacency dropped %.1fs after the cut (well under the 40s dead interval)"
        % detect
    )

    # 4. Restore the link; adjacency and BFD session must re-form.
    log_info("restoring the OSPFv3 link (FRR eth0 up)...")
    docker_exec_quiet(
        frr_bfd.container, ["ip", "link", "set", FRR_CONTAINER_LINK, "up"]
    )
    frr_ospf6.wait_adjacency(timeout=90)
    wait_bfd_up_any(frr_bfd, timeout=60)

    log_pass(
        "ospfv3-bfd-frr: v3 adjacency Full + BFD Up -> link cut -> down in %.1fs (< dead 40s) -> reconverged"
        % detect
    )


if __name__ == "__main__":
    check()
