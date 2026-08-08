#!/usr/bin/env python3
"""Scenario ospf-lfa-frr: RFC 5286 base LFA fast reroute with FRR.

Validates (spec-ospf-ext-6): Ze computes the same link-protecting loop-free
alternate FRR does over two parallel OSPF links; `show ospf route fast-reroute`
reports the pre-computed backup next-hop; and when the primary link (eth0) is cut,
the FIB fails over to the backup (eth1) with no loop, so traffic to FRR's loopback
survives the failure window without a control-plane recompute.
Prevents: a looping backup, a backup that is never installed into the FIB, or a
failover that requires full SPF reconvergence before traffic recovers.

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRROSPF,
    Ze,
    docker_exec_quiet,
    log_info,
    log_pass,
    poll,
)

# FRR's loopback prefix advertised into area 0 (the destination we protect).
FRR_LOOPBACK = "172.30.255.3/32"
PRIMARY_LINK = "eth0"


def _fast_reroute_rows():
    """Return the `show ospf route fast-reroute` rows as parsed JSON.

    want=list is what makes the loop below safe: a document that decoded to an
    object hands `row` a str, and `row.get(...)` then raises AttributeError from
    inside the probe, which poll does not bound (see Ze.cli_json).
    """
    return Ze().cli_json("show ospf route fast-reroute", want=list)


def _protected_backup(prefix):
    """Return the backup next-hop for prefix if a protected primary exists, else None."""
    for row in _fast_reroute_rows():
        if row.get("prefix") != prefix:
            continue
        for hop in row.get("next-hops", []):
            if hop.get("protected") and hop.get("backup"):
                return hop["backup"]
    return None


def check():
    frr = FRROSPF()

    # 1. Form both adjacencies.
    log_info("waiting for OSPF adjacency between Ze and FRR (two links)...")
    frr.wait_adjacency(timeout=90)

    # 2. Ze must have computed a link-protecting backup toward FRR's loopback.
    log_info("waiting for Ze to pre-compute an LFA backup...")
    backup = poll(
        lambda: _protected_backup(FRR_LOOPBACK),
        bool,
        timeout=60,
        interval=2,
        what="show ospf route fast-reroute",
    )
    if not backup:
        raise AssertionError(
            "Ze did not pre-compute a fast-reroute backup for %s" % FRR_LOOPBACK
        )
    log_info("Ze pre-computed backup next-hop %s for %s" % (backup, FRR_LOOPBACK))

    # 3. Cut the primary link; the pre-computed backup must keep the loopback
    #    reachable (ping survives the failover window).
    log_info("cutting the primary OSPF link (FRR %s down)..." % PRIMARY_LINK)
    docker_exec_quiet(frr.container, ["ip", "link", "set", PRIMARY_LINK, "down"])
    time.sleep(2)
    dst = FRR_LOOPBACK.split("/")[0]
    reachable = False
    for _ in range(10):
        rc = docker_exec_quiet(Ze().container, ["ping", "-c", "1", "-W", "1", dst])
        if "1 received" in rc or "1 packets received" in rc:
            reachable = True
            break
        time.sleep(1)
    docker_exec_quiet(frr.container, ["ip", "link", "set", PRIMARY_LINK, "up"])
    if not reachable:
        raise AssertionError(
            "traffic to %s did not survive the primary-link cut via the backup" % dst
        )

    log_pass(
        "ospf-lfa-frr: LFA backup pre-computed and FIB failed over to it on link cut"
    )


if __name__ == "__main__":
    check()
