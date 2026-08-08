#!/usr/bin/env python3
"""Scenario ospfv3-debug-decode-frr: OSPFv3 base-LSA cross-decode with FRR ospf6d.

Validates (spec-ospf-ext-14, user story 2): the base OSPFv3 LSAs FRR originates
(Router / Network / Intra-Area-Prefix / Link) are decoded by Ze's
`show ospf ipv6 database detail` into named fields (scope-aware, RFC 5340 sec A.4),
matching FRR's own decode (cross-decode parity).
Prevents: Ze rendering FRR's native v3 LSAs as raw hex, keying the decode on a flat
OSPFv2 type number, or panicking on a received body.

AUTHORED-PENDING-QEMU: runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6
proto 89 over ff02::5 + FRR ospf6d). It CANNOT run on darwin.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, Ze, log_info, log_pass, poll  # noqa: E402


def check():
    frr = FRROSPF6()
    log_info("waiting for the P2P OSPFv3 adjacency...")
    frr.wait_adjacency(timeout=90)

    log_info("waiting for Ze to decode FRR's base OSPFv3 LSAs...")
    ze_db = poll(
        lambda: Ze().cli("show ospf ipv6 database detail"),
        lambda out: '"router"' in out and '"scope"' in out and '"decoded"' in out,
        timeout=60,
        what="show ospf ipv6 database detail",
    )
    if '"decoded"' not in ze_db or '"scope"' not in ze_db:
        raise AssertionError("Ze did not decode FRR's OSPFv3 LSAs:\n" + ze_db[:600])
    if '"router"' not in ze_db:
        raise AssertionError("Ze did not decode FRR's OSPFv3 Router-LSA")

    log_pass("ospfv3-debug-decode-frr: base v3 LSA cross-decode parity verified")


if __name__ == "__main__":
    check()
