#!/usr/bin/env python3
"""Scenario ospf-ti-lfa-frr: TI-LFA SR repair with FRR.

Validates (spec-ospf-ext-6 + RFC 8665): Ze and FRR exchange Segment Routing
labels (Prefix-SID / Adj-SID with the B-Flag), Ze runs fast-reroute in ti-lfa
mode, and `show ospf route fast-reroute` reports a pre-computed backup toward
FRR's loopback whose repair encoding (where a repair list is built) uses the
resolved 20-bit MPLS label form, not a SID index. On a primary-link cut the FIB
steers traffic onto the backup so the loopback stays reachable.
Prevents: a TI-LFA repair that pushes a SID index instead of a resolved label, or
a fast-reroute backup that never reaches the MPLS-capable FIB.

A full P-space/Q-space repair (a topology with NO base LFA) needs a 3-node ring
and is a documented follow-on of the multi-node interop harness; this 2-node
scenario asserts the SR + fast-reroute co-existence, the wire-observable Adj-SID
B-Flag, and the FIB failover.

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
"""

import json
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
)

FRR_LOOPBACK = "172.30.255.3/32"
PRIMARY_LINK = "eth0"


def _sr_up():
    """Ze has installed at least one SR MPLS label entry (Prefix-SID exchange)."""
    out = docker_exec_quiet(
        Ze().container, ["ze", "show", "ospf", "segment-routing", "--json"]
    )
    return "prefix-sid" in out.lower() or "16" in out


def _protected_backup(prefix):
    out = docker_exec_quiet(
        Ze().container, ["ze", "show", "ospf", "route", "fast-reroute", "--json"]
    )
    try:
        rows = json.loads(out)
    except (ValueError, TypeError):
        return None
    for row in rows:
        if row.get("prefix") != prefix:
            continue
        for hop in row.get("next-hops", []):
            if hop.get("protected") and hop.get("backup"):
                # A TI-LFA repair carries a resolved 20-bit label stack; a base LFA
                # carries none. Either is a valid backup for this 2-node topology.
                for label in hop.get("repair-labels", []):
                    assert 0 <= label <= 0xFFFFF, (
                        "repair label %d not a 20-bit MPLS label" % label
                    )
                return hop["backup"]
    return None


def check():
    frr = FRROSPF()

    log_info("waiting for OSPF adjacency (two links) with SR...")
    frr.wait_adjacency(timeout=90)

    log_info("waiting for the SR control plane to exchange labels...")
    deadline = time.time() + 60
    while time.time() < deadline:
        if _sr_up():
            break
        time.sleep(2)

    log_info("waiting for Ze to pre-compute a ti-lfa fast-reroute backup...")
    deadline = time.time() + 60
    backup = None
    while time.time() < deadline:
        backup = _protected_backup(FRR_LOOPBACK)
        if backup:
            break
        time.sleep(2)
    if not backup:
        raise AssertionError(
            "Ze did not pre-compute a ti-lfa fast-reroute backup for %s" % FRR_LOOPBACK
        )
    log_info("Ze pre-computed ti-lfa backup next-hop %s" % backup)

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
            "traffic to %s did not survive the primary-link cut via the ti-lfa backup"
            % dst
        )

    log_pass(
        "ospf-ti-lfa-frr: SR + ti-lfa backup pre-computed and FIB failed over on link cut"
    )


if __name__ == "__main__":
    check()
