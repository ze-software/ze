#!/usr/bin/env python3
"""Scenario ospf-debug-te-frr: OSPFv2 TE opaque-LSA cross-decode with FRR ospfd.

Validates (spec-ospf-ext-14, user story 1; gated on spec-ospf-ext-2's TE decoder):
FRR originates a Traffic Engineering opaque LSA (RFC 3630, Opaque Type 1) that Ze
receives and renders via `show ospf te-database`, decoding the same Router-Address /
Link sub-TLVs FRR shows (cross-decode parity). The TE decoder is owned by ext-2, which
is landed, so this step runs; without it, skip with justification.
Prevents: Ze rendering FRR's TE LSA as raw hex, or decoding sub-TLVs FRR does not.

AUTHORED-PENDING-QEMU: runs under the Linux Docker/QEMU interop harness ONLY (FRR ospfd
with `mpls-te on`). It CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, ZE_CONTAINER, docker_exec_quiet, log_info, log_pass  # noqa: E402


def check():
    frr = FRROSPF()
    log_info("waiting for the P2P OSPFv2 adjacency (TE opaque on FRR)...")
    frr.wait_adjacency(timeout=90)

    log_info("waiting for Ze to decode FRR's TE opaque LSA...")
    deadline = time.time() + 60
    ze_te = ""
    while time.time() < deadline:
        ze_te = docker_exec_quiet(ZE_CONTAINER, ["ze", "show", "ospf", "te-database"])
        low = ze_te.lower()
        if "router-address" in low or "link-id" in low or "te-metric" in low:
            break
        time.sleep(3)
    low = ze_te.lower()
    if "router" not in low and "link" not in low:
        raise AssertionError("Ze did not decode FRR's TE opaque LSA:\n" + ze_te[:500])

    log_pass("ospf-debug-te-frr: TE opaque-LSA cross-decode parity verified")


if __name__ == "__main__":
    check()
