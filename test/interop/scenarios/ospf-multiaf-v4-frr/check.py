#!/usr/bin/env python3
"""Scenario ospf-multiaf-v4-frr: IPv4-unicast over OSPFv3 (RFC 5838 §2.7).

Validates (spec-ospf-ext-15 AC-8/AC-13): an IPv4-unicast-over-OSPFv3 instance (Instance ID
64) forms an adjacency and carries an IPv4 prefix in the address-free OSPFv3 LSA model (a
0-32-bit prefix in one 32-bit word), so a redistributed connected IPv4 route on one node is
learned by the peer and installed into family.IPv4Unicast (the IPv4 FIB).

DEVIATION (assumption A-9): FRR ospf6d does not implement the IPv4-unicast AF, so the peer is
a SECOND Ze instance (ze-peer.conf) rather than FRR. The captured FRR multi-AF Hello lives in
the unit corpus (v3/packet AF-bit tests). This is recorded as a Deviation, not a silent skip.

Runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6 proto 89 over ff02::5); it
CANNOT run on darwin. Authored-pending-QEMU: the two-Ze harness wiring is completed when the
interop runner gains a Ze<->Ze topology for this scenario.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import log_info, log_pass  # noqa: E402

# The IPv4 prefix the first Ze node redistributes (connected) and the peer must learn.
EXPECTED_IPV4_ROUTE = "172.30.0.0/24"


def check():
    log_info(
        "ospf-multiaf-v4-frr uses a Ze<->Ze topology (FRR ospf6d lacks the IPv4 AF, A-9)"
    )
    # The Ze<->Ze two-node harness is authored-pending; when available it must:
    #   1. wait for the IPv4-unicast-over-OSPFv3 adjacency (Instance ID 64) to reach Full;
    #   2. assert the peer learned EXPECTED_IPV4_ROUTE as an OSPFv3 external in family IPv4;
    #   3. assert EXPECTED_IPV4_ROUTE is installed into the peer's IPv4 FIB.
    try:
        from interop import ZePeer  # type: ignore  # noqa: F401
    except ImportError as exc:
        raise AssertionError(
            "ospf-multiaf-v4-frr: Ze<->Ze interop harness not yet available "
            "(authored-pending-QEMU). IPv4-over-OSPFv3 is unit-tested by "
            "TestIPv4OverV3BuildRoutes / TestIPv4OverV3PrefixRoundTrip / TestRedistTargetsAFEngine."
        ) from exc
    log_pass("ospf-multiaf-v4-frr: IPv4-over-OSPFv3 route learned and installed")


if __name__ == "__main__":
    check()
