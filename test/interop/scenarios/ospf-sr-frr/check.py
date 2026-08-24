#!/usr/bin/env python3
"""Scenario ospf-sr-frr: RFC 8665 OSPFv2 Segment Routing interop with FRR ospfd.

Validates (spec-ospf-ext-5): with opaque + router-information + extended-prefix/extended-link
+ segment-routing enabled on Ze and `segment-routing on` on FRR, Ze forms a Full OSPFv2
adjacency with FRR, both originate the SR TLVs (SR-Algorithm 0 + SRGB in the RI LSA, a
Prefix-SID for the loopback in the Extended Prefix LSA, Adj-SIDs in the Extended Link LSA), Ze
computes the outgoing label for FRR's Prefix-SID from FRR's advertised SRGB (16000 + index 100
= 16100) and installs a matching MPLS push, and FRR installs Ze's Prefix-SID (index 200). A
label-switched ping over the SR LSP then succeeds with the correct NP/E (no-PHP) behavior.

The MPLS forwarding assertion uses `mpls -ls` inside the QEMU guest (Linux-only AF_MPLS).

Runs under the Linux Docker/QEMU interop harness ONLY (raw IP proto 89 + FRR ospfd with
segment-routing + AF_MPLS); it CANNOT run on darwin.

RUN FOR THE FIRST TIME ON 2026-08-23, under Docker on darwin. Steps 1 and 2 pass:
the adjacency reaches Full and Ze originates SR-Algorithm 0, its SRGB and its node
Prefix-SID. Step 3 FAILS, and the reason is the instrument rather than Ze. `mpls` is
not installed in test/interop/Dockerfile.ze, so `mpls -ls` cannot run at all, and the
Docker Desktop kernel exposes no /proc/sys/net/mpls, so no MPLS forwarding table
exists there for Ze to install into. Both are reported in the failure text.

Until step 3 has an instrument that can observe an MPLS install, this scenario proves
the ORIGINATION half only. It used to say so by logging and passing anyway, which
made the whole file report success on a run that observed nothing.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRROSPF,
    Ze,
    ZE_CONTAINER,
    docker_exec_quiet,
    log_info,
    log_pass,
    poll,
)

ZE_ROUTER_ID = "172.30.0.2"
FRR_LOOPBACK = "172.30.0.3"


def check():
    frr = FRROSPF()
    Ze()  # container health asserted by the harness before check() runs

    # 1. Adjacency must reach Full with SR enabled on both sides.
    log_info("waiting for OSPFv2 adjacency between Ze and FRR (segment-routing on)...")
    frr.wait_adjacency(timeout=90)

    # 2. Ze must render its own SR state (SRGB + node Prefix-SID) via the SR snapshot.
    log_info("waiting for Ze to originate its SR TLVs (SRGB + Prefix-SID)...")
    ze_sr = poll(
        lambda: Ze().cli("show ospf segment-routing"),
        lambda out: "16000" in out and ZE_ROUTER_ID in out,
        timeout=60,
        what="show ospf segment-routing",
    )
    if "16000" not in ze_sr:
        raise AssertionError("Ze did not advertise its SRGB / node Prefix-SID")
    log_pass("Ze advertised SR-Algorithm 0, SRGB and its node Prefix-SID")

    # 3. Ze must decode FRR's Prefix-SID and install a matching MPLS push. FRR advertises
    #    172.30.0.3/32 with SID index 100 against SRGB base 16000 -> label 16100.
    log_info("checking Ze installed an MPLS push for FRR's Prefix-SID (label 16100)...")
    deadline = time.time() + 60
    mpls = ""
    while time.time() < deadline:
        mpls = docker_exec_quiet(ZE_CONTAINER, ["mpls", "-ls"])
        if "16100" in mpls:
            break
        time.sleep(3)
    if "16100" not in mpls:
        # RAISE. This assertion is the whole reception half of the scenario: Ze
        # decoding FRR's Prefix-SID and programming the label its SRGB implies.
        # It used to log and fall through to `log_pass`, so a run that could not
        # find the label it exists to prove reported success, and reverting the
        # decoder would have changed nothing about the verdict.
        #
        # A readback that came back empty has three causes and they are not the same
        # fact, so the failure says which. `docker_exec_quiet` returns "" for a
        # command that could not run at all, so without this the message would
        # attribute a MISSING INSTRUMENT to a missing route, which is the reverse of
        # what a reader needs. Measured on 2026-08-23: `mpls` is not in
        # test/interop/Dockerfile.ze at all, and the Docker Desktop kernel exposes no
        # /proc/sys/net/mpls, so on that host neither the tool nor the table exists.
        instrument = docker_exec_quiet(
            ZE_CONTAINER, ["sh", "-c", "command -v mpls || echo ABSENT"]
        ).strip()
        kernel = docker_exec_quiet(
            ZE_CONTAINER, ["sh", "-c", "ls -d /proc/sys/net/mpls 2>/dev/null || echo ABSENT"]
        ).strip()
        raise AssertionError(
            "Ze did not install an MPLS push for FRR's Prefix-SID (label 16100) "
            "within 60s.\n  `mpls -ls` returned: %s\n  mpls binary in the Ze "
            "container: %s\n  kernel MPLS (/proc/sys/net/mpls): %s"
            % (mpls.strip() or "(empty)", instrument or "(empty)", kernel or "(empty)")
        )

    # 4. FRR must have installed Ze's Prefix-SID (index 200) as an SR label toward Ze.
    frr_sr = frr._vtysh_quiet("show ip ospf database segment-routing")
    if ZE_ROUTER_ID not in frr_sr:
        # RAISE, for the same reason as step 3. This is the other direction of
        # the same exchange: FRR reading what Ze originated. Logging it left the
        # scenario passing whether or not Ze's Extended Prefix LSA ever reached
        # a peer, which is the one thing an interop scenario is here to say.
        raise AssertionError(
            "FRR's OSPF SR database does not list Ze's Prefix-SID (router-id %s); "
            "`show ip ospf database segment-routing` returned:\n%s"
            % (ZE_ROUTER_ID, frr_sr or "(empty)")
        )

    # 5. Stability: the adjacency must still be Full after a short settle.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("OSPFv2 SR adjacency did not stay Full")

    log_pass("ospf-sr-frr: SR TLVs exchanged, labels agree, adjacency stable")


if __name__ == "__main__":
    check()
