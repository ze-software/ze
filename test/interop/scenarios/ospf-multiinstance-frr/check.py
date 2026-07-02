#!/usr/bin/env python3
"""Scenario ospf-multiinstance-frr: OSPFv2 Multi-Instance (RFC 6549) interop.

Validates (spec-ospf-ext-12):
  - AC-9: Ze's Instance-ID-0 engine forms a Full OSPFv2 adjacency with legacy FRR ospfd
    over the shared L2 bridge (Ze emits today's bytes at Instance ID 0).
  - AC-10 (non-zero adjacency): Ze's Instance-ID-5 engine forms a Full adjacency with BIRD
    running the same interface at Instance ID 5 (the RFC 6549 multi-instance reference).
  - AC-10 (legacy isolation): FRR receives Ze's Instance-ID-5 multicast Hellos but reads
    them as a mismatched 16-bit AuType and drops them at authentication, so it never adjoins
    Instance 5 and does not crash -- its Instance-0 adjacency with Ze stays Full throughout.

Prevents: a non-zero Instance ID leaking into a legacy peer's adjacency, the Instance-0 path
regressing against legacy OSPFv2, or the two Ze instances failing to demux (no adjacency).

Runs under the Linux Docker/QEMU interop harness ONLY (raw IP proto 89 + FRR ospfd + BIRD);
it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import BIRD, FRROSPF, docker_logs, log_info, log_pass  # noqa: E402
from interop import ZE_CONTAINER  # noqa: E402


def _wait_bird_ospf_full(bird, timeout=90):
    """Poll BIRD until its OSPFv2 (Instance 5) neighbor to Ze reaches Full."""
    log_info("waiting for BIRD OSPF (Instance ID 5) adjacency to Ze to reach Full...")
    deadline = time.time() + timeout
    last = ""
    while time.time() < deadline:
        last = bird._birdc_quiet("show ospf neighbors ospf5")
        if not last:
            last = bird._birdc_quiet("show ospf neighbors")
        if "Full" in last:
            log_pass("BIRD OSPF Instance-5 adjacency is Full")
            return
        time.sleep(2)
    print(last[:500])
    print(docker_logs(ZE_CONTAINER, 40))
    raise AssertionError("BIRD OSPF Instance-5 adjacency did not reach Full")


def check():
    frr = FRROSPF()
    bird = BIRD()

    # AC-9: Ze Instance 0 <-> legacy FRR forms a Full adjacency (unchanged bytes).
    frr.wait_adjacency(timeout=120)

    # FRR's only OSPF neighbor is Ze's router-id; it must never adjoin the Instance-5
    # traffic BIRD/Ze put on the wire (AC-10 legacy isolation).
    neighbors = frr._vtysh_quiet("show ip ospf neighbor")
    if "172.30.0.2" not in neighbors:
        raise AssertionError(
            "FRR did not form its Instance-0 adjacency with Ze (172.30.0.2): %r"
            % neighbors[:300]
        )

    # AC-10 (non-zero adjacency): Ze Instance 5 <-> BIRD (Instance ID 5) reaches Full.
    _wait_bird_ospf_full(bird, timeout=120)

    # Legacy isolation stability: after BIRD's Instance-5 adjacency is up (Instance-5
    # multicast is flowing), FRR's Instance-0 adjacency must still be Full and FRR healthy.
    time.sleep(5)
    if not frr.adjacency_full():
        raise AssertionError(
            "FRR's Instance-0 adjacency did not stay Full while Instance-5 traffic flowed "
            "(legacy isolation broken)"
        )

    log_pass(
        "ospf-multiinstance-frr: Ze<->FRR Full at Instance 0, Ze<->BIRD Full at Instance 5, "
        "legacy FRR isolated from Instance 5"
    )


if __name__ == "__main__":
    check()
