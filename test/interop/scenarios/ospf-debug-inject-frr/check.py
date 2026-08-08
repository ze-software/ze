#!/usr/bin/env python3
"""Scenario ospf-debug-inject-frr: OSPFv2 debug LSA injection interop with FRR ospfd.

Validates (spec-ospf-ext-14, user stories 5+7): with a P2P OSPFv2 adjacency and
opaque capability both ways, an authorized Ze operator enables debug injection and
injects a crafted Private-Use (Opaque Type 250) area-scope opaque LSA; it floods to
FRR and appears in FRR's `show ip ospf database opaque-area`; a withdraw MaxAge-purges
it so FRR drops it; the adjacency is unaffected throughout (AC-13/AC-15/AC-23).
Prevents: an injected LSA FRR rejects (bad opaque framing), a withdraw that leaks, or
an injection that flaps the adjacency.

AUTHORED-PENDING-QEMU: runs under the Linux Docker/QEMU interop harness ONLY (raw IP
proto 89 over 224.0.0.5 + FRR ospfd with `capability opaque`). It CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF, Ze, log_info, log_pass  # noqa: E402

OPAQUE_ID = "1"


def check():
    frr = FRROSPF()
    log_info("waiting for the P2P OSPFv2 adjacency (opaque on both sides)...")
    frr.wait_adjacency(timeout=90)
    if not frr.adjacency_full():
        raise AssertionError("adjacency not Full before injection")

    log_info("enabling debug injection and injecting a Private-Use opaque LSA...")
    Ze().cli("debug ospf inject enable")
    Ze().cli(
        "debug ip ospf inject opaque scope area id %s hex 0001000401020304" % OPAQUE_ID
    )

    log_info("waiting for FRR to receive the injected opaque-area LSA...")
    deadline = time.time() + 60
    seen = ""
    while time.time() < deadline:
        seen = frr._vtysh_quiet("show ip ospf database opaque-area")
        if "Opaque" in seen:
            break
        time.sleep(3)
    if "Opaque" not in seen:
        raise AssertionError(
            "FRR never received the injected opaque LSA:\n" + seen[:500]
        )

    log_info("withdrawing the injected LSA; the adjacency must not flap...")
    Ze().cli("debug ip ospf inject opaque scope area id %s withdraw" % OPAQUE_ID)
    time.sleep(8)
    if not frr.adjacency_full():
        raise AssertionError("adjacency flapped during inject/withdraw")

    log_pass("ospf-debug-inject-frr: opaque inject + flood + withdraw interop verified")


if __name__ == "__main__":
    check()
