#!/usr/bin/env python3
"""Scenario ospf-auth-frr: keyed-MD5 authenticated OSPFv2 adjacency with FRR.

Validates (spec-ospf-13 AC-19): Ze and FRR form a Full OSPFv2 adjacency only when
their keyed-MD5 keys match (shared key-id 1 / secret), proving Ze signs and
verifies the cryptographic authentication (RFC 2328 Appendix D) compatibly with
FRR. Wrong-key rejection is unit-covered (TestOSPFAuthStoreSignVerify).
Prevents: an MD5 digest mismatch with FRR (wrong key bytes, wrong checksum
handling, wrong digest framing) that would keep the adjacency from forming.

Runs under the Linux Docker/QEMU interop harness ONLY; it CANNOT run on darwin.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, log_info, log_pass  # noqa: E402


def check():
    frr = FRROSPF()

    # The authenticated OSPF adjacency reaches Full only if the MD5 digests match.
    log_info("waiting for the keyed-MD5 authenticated OSPF adjacency...")
    frr.wait_adjacency(timeout=90)

    log_pass("ospf-auth-frr: keyed-MD5 authenticated adjacency reached Full")


if __name__ == "__main__":
    check()
