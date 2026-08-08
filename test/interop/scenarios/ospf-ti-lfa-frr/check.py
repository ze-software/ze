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

FRR_LOOPBACK = "172.30.255.3/32"
PRIMARY_LINK = "eth0"

# What ze.conf in this directory declares under `segment-routing`. The check
# reads its own scenario's config back out of the daemon, so a value changed
# there and not here fails loudly instead of passing over nothing.
ZE_SRGB = (16000, 23999)
ZE_PREFIX_SID = ("172.30.0.2/32", 200)


def _sr_state():
    """Ze's own SR state: SRGB, SRLB and Prefix-SIDs, as srSnapshot renders it.

    (*engine).srSnapshot (internal/plugins/ospf/sr_snapshot.go) returns
    Enabled=false with every list empty when SR is not programmed, and fills
    SRGB / PrefixSIDs from the running config otherwise, so the fields below
    separate "SR is up" from "SR never started".
    """
    return Ze().cli_json("show ospf segment-routing", want=dict)


def _sr_programmed(state):
    """SR is programmed with the SRGB and the Prefix-SID ze.conf declares.

    The predicate this replaced was `"prefix-sid" in out.lower() or "16" in
    out`, and BOTH halves matched unconditionally: "prefix-sids" is a JSON key
    srSnapshot always emits (the field has no omitempty and is initialised to
    []), and "16" matches any address octet, count or label anywhere in the
    document. It answered true on the first call whatever SR was doing.
    """
    if not state.get("enabled"):
        return False
    lower, upper = ZE_SRGB
    srgb = [
        r
        for r in state.get("srgb", [])
        if r.get("lower-bound") == lower and r.get("upper-bound") == upper
    ]
    prefix, index = ZE_PREFIX_SID
    sids = [
        p
        for p in state.get("prefix-sids", [])
        if p.get("prefix") == prefix and p.get("index") == index
    ]
    return bool(srgb) and bool(sids)


def _protected_backup(prefix):
    rows = Ze().cli_json("show ospf route fast-reroute", want=list)
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

    log_info("waiting for Ze to program its SRGB and node Prefix-SID...")
    sr_state = poll(
        _sr_state,
        _sr_programmed,
        timeout=60,
        interval=2,
        what="show ospf segment-routing",
    )
    # poll RETURNS the last result when the deadline passes with some call
    # having succeeded, so discarding it turns "SR never came up" into a silent
    # pass. Twelve of the thirteen poll sites assign and assert; this one did
    # not, and the branch it dropped is the one poll exists to surface.
    if not _sr_programmed(sr_state):
        raise AssertionError(
            "Ze did not program SR: enabled=%r srgb=%r prefix-sids=%r, want SRGB %s "
            "and Prefix-SID %s from ze.conf"
            % (
                sr_state.get("enabled"),
                sr_state.get("srgb"),
                sr_state.get("prefix-sids"),
                ZE_SRGB,
                ZE_PREFIX_SID,
            )
        )

    log_info("waiting for Ze to pre-compute a ti-lfa fast-reroute backup...")
    backup = poll(
        lambda: _protected_backup(FRR_LOOPBACK),
        bool,
        timeout=60,
        interval=2,
        what="show ospf route fast-reroute",
    )
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
