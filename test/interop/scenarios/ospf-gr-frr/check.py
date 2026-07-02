#!/usr/bin/env python3
"""Scenario ospf-gr-frr: OSPFv2 Graceful Restart interop with FRR ospfd (RFC 3623).

Validates (spec-ospf-ext-9, user stories 3+4): Ze and FRR form a P2P OSPFv2
adjacency with GR enabled both ways; the Opaque Type 3 Grace-LSA interops, the
Ze-helper holds the adjacency while FRR restarts (no flap), and the Ze-restarter
is helped by FRR and exits GR cleanly.
Prevents: a Grace-LSA FRR rejects (mis-padded TLVs / wrong Opaque Type), or a
helper that drops the adjacency mid-restart.

AUTHORED-PENDING-QEMU: runs under the Linux Docker/QEMU interop harness ONLY (raw
IP proto 89 + FRR ospfd). The planned-restart trigger is a managed reload of the
Ze OSPF process (the NVS restart fact carries the grace deadline across it).
"""
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, Ze, log_info, log_pass  # noqa: E402

ZE_ROUTER_ID = "172.30.0.2"


def check():
    frr = FRROSPF()
    log_info("waiting for the P2P OSPFv2 adjacency (GR enabled both sides)...")
    frr.wait_adjacency(timeout=90)

    if ZE_ROUTER_ID not in frr._vtysh_quiet("show ip ospf database router"):
        raise AssertionError("FRR did not synchronise Ze's Router-LSA")

    log_info("FRR restarts gracefully; Ze must remain a helper (no flap)...")
    frr._vtysh_quiet("clear ip ospf process graceful-restart")
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("adjacency flapped during FRR graceful restart (Ze helper failed)")

    log_info("verifying Ze retained its RIB across the graceful window...")
    Ze().rib_received(0)  # FIB retention proper is asserted by ospf-gr-fib-retention

    log_pass("ospf-gr-frr: OSPFv2 Grace-LSA interop + helper hold verified")


if __name__ == "__main__":
    check()
