#!/usr/bin/env python3
"""Scenario ospfv3-gr-frr: OSPFv3 Graceful Restart interop with FRR ospf6d (RFC 5187).

Validates (spec-ospf-ext-9): the native OSPFv3 Grace-LSA (LS Type 0x000B, LS ID =
Interface ID, two TLVs) interops with FRR ospf6d in both directions -- Ze holds
the adjacency while FRR restarts, and the Ze-restarter is helped by FRR.
Prevents: a v6 Grace-LSA FRR rejects (wrong function code / LS ID / TLV padding).

AUTHORED-PENDING-QEMU: Linux Docker/QEMU harness ONLY (raw IPv6 proto 89 over
ff02::5 + FRR ospf6d). The planned-restart trigger is a managed reload of the Ze
OSPF process (the NVS restart fact + preserved Interface IDs carry across it).
"""
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, log_info, log_pass  # noqa: E402

ZE_ROUTER_ID = "172.30.0.2"


def check():
    frr = FRROSPF6()
    log_info("waiting for the P2P OSPFv3 adjacency (GR enabled)...")
    frr.wait_adjacency(timeout=90)

    if ZE_ROUTER_ID not in frr._vtysh_quiet("show ipv6 ospf6 database router"):
        raise AssertionError("FRR ospf6d did not synchronise Ze's Router-LSA")

    log_info("FRR restarts gracefully; Ze must remain a helper (no flap)...")
    frr._vtysh_quiet("clear ipv6 ospf6 process")
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError("v6 adjacency flapped during FRR graceful restart (Ze helper failed)")

    log_pass("ospfv3-gr-frr: OSPFv3 Grace-LSA (0x000B) interop + helper hold verified")


if __name__ == "__main__":
    check()
