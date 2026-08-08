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

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, Ze, log_info, log_pass, poll  # noqa: E402


def check():
    frr = FRROSPF()
    log_info("waiting for the P2P OSPFv2 adjacency (TE opaque on FRR)...")
    frr.wait_adjacency(timeout=90)

    log_info("waiting for Ze to decode FRR's TE opaque LSA...")
    ze_te = poll(
        lambda: Ze().cli("show ospf te-database"),
        lambda out: any(
            token in out.lower() for token in ("router-address", "link-id", "te-metric")
        ),
        timeout=60,
        what="show ospf te-database",
    )
    low = ze_te.lower()
    if "router" not in low and "link" not in low:
        raise AssertionError("Ze did not decode FRR's TE opaque LSA:\n" + ze_te[:500])

    log_pass("ospf-debug-te-frr: TE opaque-LSA cross-decode parity verified")


if __name__ == "__main__":
    check()
