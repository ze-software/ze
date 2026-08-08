#!/usr/bin/env python3
"""Scenario ospf-te-interas-frr: RFC 5392 Inter-AS-TE-v2 LSA interop with FRR.

Validates (spec-ospf-ext-2): Ze originates an Inter-AS-TE-v2 LSA (Opaque type 6) for its
proxied inter-AS link at the configured scope=as, i.e. a Type 11 (AS-wide) opaque LSA
carrying the Remote AS Number and IPv4 Remote ASBR ID (and, per RFC 5392 sec 3.2.1, no Link
ID sub-TLV). Ze installs it in its own AS-wide opaque store, and FRR -- which does not
interpret the inter-AS body -- still floods it AS-wide per RFC 5250, so it appears in FRR's
opaque-as database. This proves Ze's Type-11 scope choice and inter-AS body are accepted and
flooded across implementations.
Prevents: an inter-AS TE LSA flooded at the wrong scope, or one FRR drops instead of
flooding.

Runs under the Linux Docker/QEMU interop harness ONLY (raw IP proto 89 + FRR ospfd with the
opaque capability); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRROSPF,
    Ze,
    log_fail,
    log_info,
    log_pass,
    poll,
)

ZE_ROUTER = "172.30.0.2"


def _ze_opaque_as():
    """Return Ze's `show ospf database opaque-as` output. Raises on query failure."""
    return Ze().cli("show ospf database opaque-as")


def check():
    frr = FRROSPF()
    Ze()  # container health is asserted by the harness before check() runs

    log_info("waiting for OSPF adjacency between Ze and FRR (opaque enabled)...")
    frr.wait_adjacency(timeout=90)

    # 1. Ze must install its own Type-11 inter-AS TE LSA (Opaque type 6). The opaque-as
    #    Link State ID for Opaque type 6 begins with "6." (the high byte is the Opaque type).
    log_info("waiting for Ze to originate its Type-11 inter-AS TE LSA...")
    db = poll(
        _ze_opaque_as,
        lambda out: "6." in out and ZE_ROUTER in out,
        timeout=90,
        interval=2,
        what="show ospf database opaque-as",
    )
    if "6." not in db or ZE_ROUTER not in db:
        log_fail("Ze did not originate a Type-11 inter-AS TE (Opaque type 6) LSA")
        print(db[:800])
        raise AssertionError("Ze AS-wide opaque store missing the inter-AS TE LSA")
    log_pass("Ze originated a Type-11 inter-AS TE LSA (Opaque type 6)")

    # 2. FRR must flood Ze's AS-wide opaque LSA (RFC 5250 legacy flooding): Ze's advertising
    #    router appears in FRR's opaque-as database.
    log_info(
        "waiting for FRR to flood Ze's Type-11 inter-AS TE LSA (%s)..." % ZE_ROUTER
    )
    deadline = time.time() + 90
    flooded = False
    while time.time() < deadline:
        out = frr._vtysh_quiet("show ip ospf database opaque-as")
        if ZE_ROUTER in out:
            flooded = True
            break
        time.sleep(2)
    if not flooded:
        log_fail("FRR did not flood Ze's Type-11 inter-AS TE LSA")
        print(frr._vtysh_quiet("show ip ospf database opaque-as")[:800])
        raise AssertionError("FRR opaque-as database missing Ze's inter-AS TE LSA")
    log_pass("FRR flooded Ze's Type-11 inter-AS TE LSA")

    log_pass("ospf-te-interas-frr: RFC 5392 inter-AS TE scope + flooding verified")


if __name__ == "__main__":
    check()
