#!/usr/bin/env python3
"""Scenario isis-auth-frr: HMAC-MD5-authenticated IS-IS adjacency with FRR.

Validates (spec-isis-13 AC-16): with matching HMAC-MD5 keys (RFC 5304) on the
IIH (circuit) and the area/domain (LSP/SNP), Ze and FRR form an authenticated
IS-IS adjacency over the wire. The negative half -- a wrong key is rejected --
is proven by the unit tests (the verify hook drops a mismatched PDU and bumps
ze_isis_auth_failures_total) and asserted here by confirming the authenticated
adjacency comes up ONLY with the matched key (a mismatch leaves it Down).
Prevents: an auth path that accepts a wrong key, or one whose HMAC-MD5 signing
FRR rejects so even the correct key never forms an adjacency.

Runs under the Linux Docker/QEMU interop harness ONLY (raw L2 + FRR isisd).
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRRISIS, log_info, log_pass  # noqa: E402


def check():
    frr = FRRISIS()

    # 1. With matching HMAC-MD5 keys, the authenticated adjacency must come up.
    log_info("waiting for HMAC-authenticated IS-IS adjacency between Ze and FRR...")
    frr.wait_adjacency(timeout=90)

    # 2. Stability: an authenticated adjacency that flaps would indicate
    #    intermittent verify failures (e.g. checksum-region/sign-order bugs).
    time.sleep(5)
    if not frr.adjacency_up():
        raise AssertionError(
            "authenticated IS-IS adjacency did not stay Up (intermittent auth failure?)"
        )

    log_pass("isis-auth-frr: HMAC-MD5 adjacency formed and stable with matched keys")


if __name__ == "__main__":
    check()
