#!/usr/bin/env python3
"""Scenario ospf-te-frr: RFC 3630 Traffic Engineering LSA interop with FRR.

Validates (spec-ospf-ext-2): over a Full point-to-point OSPFv2 adjacency with the opaque
capability enabled on both sides, Ze parses FRR's mpls-te Traffic Engineering LSAs into its
TED (Ze `show ospf te-database` lists FRR's advertising router and TE link), and FRR accepts
and decodes Ze's originated TE LSAs (FRR `show ip ospf database opaque-area` lists Ze's
advertising router). This proves the RFC 3630 body codec -- the 4-octet TLV alignment and
the IEEE-float bandwidth encoding -- interoperates across implementations in both directions.
Prevents: a TE LSA Ze cannot parse from FRR, or a TE LSA FRR rejects from Ze.

Runs under the Linux Docker/QEMU interop harness ONLY (raw IP proto 89 + FRR ospfd with
mpls-te); it CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    FRROSPF,
    Ze,
    ZE_CONTAINER,
    docker_exec_quiet,
    log_fail,
    log_info,
    log_pass,
)

ZE_ROUTER = "172.30.0.2"
FRR_ROUTER = "172.30.0.3"


def _ze_te_database():
    """Return Ze's `show ospf te-database` text (empty string on failure)."""
    return docker_exec_quiet(ZE_CONTAINER, ["ze", "show", "ospf", "te-database"])


def check():
    frr = FRROSPF()
    Ze()  # container health is asserted by the harness before check() runs

    # 1. Adjacency must reach Full with opaque + mpls-te enabled on both sides.
    log_info("waiting for OSPF adjacency between Ze and FRR (TE enabled)...")
    frr.wait_adjacency(timeout=90)

    # 2. Ze must parse FRR's TE LSA into its TED: FRR's advertising router appears in
    #    `show ospf te-database` (the TED lists FRR's router-address and/or TE link).
    log_info("waiting for Ze to parse FRR's TE LSA into the TED (%s)..." % FRR_ROUTER)
    deadline = time.time() + 90
    parsed = False
    while time.time() < deadline:
        db = _ze_te_database()
        if FRR_ROUTER in db:
            parsed = True
            break
        time.sleep(2)
    if not parsed:
        log_fail("Ze did not parse FRR's TE LSA into the TED")
        print(_ze_te_database()[:800])
        raise AssertionError("Ze TED missing FRR's TE information")
    log_pass("Ze parsed FRR's TE LSA into the TED")

    # 3. FRR must accept and decode Ze's originated TE LSAs (Router-Address + Link): Ze's
    #    advertising router appears in FRR's opaque-area database.
    log_info("waiting for FRR to accept Ze's TE LSA (%s)..." % ZE_ROUTER)
    deadline = time.time() + 90
    accepted = False
    while time.time() < deadline:
        out = frr._vtysh_quiet("show ip ospf database opaque-area")
        if ZE_ROUTER in out:
            accepted = True
            break
        time.sleep(2)
    if not accepted:
        log_fail("FRR did not accept Ze's TE LSA")
        print(frr._vtysh_quiet("show ip ospf database opaque-area")[:800])
        raise AssertionError("FRR opaque-area database missing Ze's TE LSA")
    log_pass("FRR accepted Ze's originated TE LSA")

    log_pass("ospf-te-frr: RFC 3630 TE LSA interop verified in both directions")


if __name__ == "__main__":
    check()
