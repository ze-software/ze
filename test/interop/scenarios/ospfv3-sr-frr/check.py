#!/usr/bin/env python3
"""Scenario ospfv3-sr-frr: RFC 8666 OSPFv3 Segment Routing interop with FRR ospf6d.

Validates (spec-ospf-ext-5): with the OSPFv3 RI LSA + segment-routing enabled on Ze's IPv6
family and SR-MPLS enabled on FRR ospf6d, Ze forms a Full OSPFv3 adjacency with FRR over
ff02::5, both originate valid RFC 8666 SR LSAs (SR-Algorithm 0 + SRGB in the OSPFv3 RI Opaque
LSA; a Prefix-SID for the loopback in an RFC 8362 E-Intra-Area-Prefix-LSA; Adj-SIDs in the
E-Router-LSA), Ze parses FRR's SR LSAs and computes the same label for FRR's Prefix-SID (FRR
SRGB base 16000 + index 100 = 16100), and both program matching MPLS forwarding for an
end-to-end labelled IPv6 path.

The MPLS forwarding assertion uses `mpls -ls` inside the QEMU guest (Linux-only AF_MPLS).

NOT RUN as at 2026-08-23. Its OSPFv2 twin (ospf-sr-frr) was, and step 3 there failed on
the instrument rather than on Ze: `mpls` is not installed in test/interop/Dockerfile.ze,
and the Docker Desktop kernel exposes no /proc/sys/net/mpls. This file makes the same
call, so expect the same boundary until step 3 has an instrument a kernel with AF_MPLS
can answer.

NOTE: requires an FRR ospf6d build with OSPFv3 SR-MPLS support. If the peer build lacks it,
gate this scenario; the RFC 8666 wire + install behavior is unit-tested
(TestOSPFv3ERouterBodyCarriesAdjSID / TestOSPFv3PrefixSIDInstallsPush / v3/packet SR codec).

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over ff02::5 + FRR
ospf6d + AF_MPLS); it CANNOT run on darwin. Authored pending QEMU execution.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRROSPF6,
    Ze,
    ZE_CONTAINER,
    docker_exec_quiet,
    log_info,
    log_pass,
    poll,
)

ZE_ROUTER_ID = "172.30.0.2"


def check():
    frr = FRROSPF6()
    Ze()  # container health asserted by the harness before check() runs

    # 1. The OSPFv3 adjacency must reach Full with SR enabled on both sides.
    log_info("waiting for OSPFv3 adjacency between Ze and FRR ospf6d (SR-MPLS on)...")
    frr.wait_adjacency(timeout=120)

    # 2. Ze must render its own IPv6 SR state (SRGB + node Prefix-SID) via the SR snapshot.
    log_info("waiting for Ze to originate its IPv6 SR TLVs (SRGB + Prefix-SID)...")
    ze_sr = poll(
        lambda: Ze().cli("show ospf ipv6 segment-routing"),
        lambda out: "16000" in out,
        timeout=60,
        what="show ospf ipv6 segment-routing",
    )
    if "16000" not in ze_sr:
        raise AssertionError("Ze did not advertise its IPv6 SRGB / node Prefix-SID")
    log_pass(
        "Ze advertised the OSPFv3 RI SRGB and its node Prefix-SID (E-Intra-Area-Prefix)"
    )

    # 3. Ze must decode FRR's Prefix-SID and install a matching MPLS push. FRR advertises
    #    2001:db8:cafe::3/128 with SID index 100 against SRGB base 16000 -> label 16100.
    log_info(
        "checking Ze installed an MPLS push for FRR's IPv6 Prefix-SID (label 16100)..."
    )
    deadline = time.time() + 60
    mpls = ""
    while time.time() < deadline:
        # fail-open-ok: poll loop, "" never matches 16100 and the check below raises
        mpls = docker_exec_quiet(ZE_CONTAINER, ["mpls", "-ls"])
        if "16100" in mpls:
            break
        time.sleep(3)
    if "16100" not in mpls:
        # RAISE, the same as the OSPFv2 twin. This is the reception half of the
        # scenario, and logging it left the run reporting success whether or not
        # Ze ever decoded FRR's Prefix-SID. "FRR ospf6d SR-MPLS support varies by
        # build" is a reason to pin the FRR version the scenario runs against, or
        # to state the boundary; it is not a reason to pass either way.
        #
        # A readback that came back empty has three causes and they are not the same
        # fact, so the failure says which. `docker_exec_quiet` returns "" for a
        # command that could not run at all, so without this the message would
        # attribute a MISSING INSTRUMENT to a missing route, which is the reverse of
        # what a reader needs. Measured on 2026-08-23: `mpls` is not in
        # test/interop/Dockerfile.ze at all, and the Docker Desktop kernel exposes no
        # /proc/sys/net/mpls, so on that host neither the tool nor the table exists.
        # fail-open-ok: diagnostic read, the AssertionError below is unconditional
        instrument = docker_exec_quiet(
            ZE_CONTAINER, ["sh", "-c", "command -v mpls || echo ABSENT"]
        ).strip()
        # fail-open-ok: diagnostic read, the AssertionError below is unconditional
        kernel = docker_exec_quiet(
            ZE_CONTAINER, ["sh", "-c", "ls -d /proc/sys/net/mpls 2>/dev/null || echo ABSENT"]
        ).strip()
        raise AssertionError(
            "Ze did not install an MPLS push for FRR's IPv6 Prefix-SID (label "
            "16100) within 60s.\n  `mpls -ls` returned: %s\n  mpls binary in the "
            "Ze container: %s\n  kernel MPLS (/proc/sys/net/mpls): %s"
            % (mpls.strip() or "(empty)", instrument or "(empty)", kernel or "(empty)")
        )

    # 4. Stability: the adjacency must still be Full after a short settle.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("OSPFv3 SR adjacency did not stay Full")

    log_pass(
        "ospfv3-sr-frr: RFC 8666 SR LSAs exchanged, labels agree, adjacency stable"
    )


if __name__ == "__main__":
    check()
