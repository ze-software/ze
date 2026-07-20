#!/usr/bin/env python3
"""Scenario 47: Ze (configured as a route server) relays UPDATEs FRR accepts and installs.

Ze is a general BGP speaker; route-server is one configured mode. In that mode Ze relays a
client's routes to other clients without inserting its own AS (RFC 7947). This scenario drives
two relay paths through a raw injector -- a peer that emits wire bytes no conforming daemon
produces -- and asserts a real FRR installs the result.

Path 1, REPLAY. The injector announces 10.0.0.0/24 before FRR connects, so FRR receives it via
Ze's replay-on-peer-up. Asserting FRR has 10.0.0.0/24 proves the relay works, but it is NOT the
NEXT_HOP-dedup discriminator: this replay is forwarded verbatim (buildFwdBody), so FRR still
installs 10.0.0.0/24 even with the buildWireModeUpdate de-duplication reverted (empirically
verified). Keep the assertion as a relay-liveness check, not as the discriminating signal.

Path 2, LIVE relay + split. Once FRR is up the injector sends a single UPDATE mixing Withdrawn
Routes with NLRI; Ze splits it per RFC 7606 Section 5.1 and relays the announced half
(203.0.113.0/24). THIS is the discriminating assertion: reverting the buildWireModeUpdate
NEXT_HOP de-duplication makes FRR lose 203.0.113.0/24 (verified: revert -> 203.0.113.0/24
absent, RED; fix -> present, GREEN), because the live-relayed announce is re-encoded through the
wire-mode builder. It does NOT prove FRR would reject the UNSPLIT form: Section 5.1's third
bullet obliges every receiver to accept any combination, so split-vs-unsplit discrimination is
done by the unit tests and the .ci (see the spec's Goal Validation), not here.

The independent-speaker scenario 48 exercises the NEXT_HOP dedup directly and unambiguously via
the adj-rib-in delta-replay; this scenario's Path 2 is a corroborating FRR check.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    FRR,
    INJECT_CONTAINER,
    ZE_CONTAINER,
    docker_logs,
    log_info,
    log_pass,
)


def _wait_route(frr, prefix, timeout=100):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if frr.has_route(prefix):
            return
        time.sleep(2)


def _check():
    frr = FRR()
    frr.wait_session("172.30.0.2")

    # Path 1: the replayed route (relay-liveness check). Forwarded verbatim (buildFwdBody), so
    # this stays installed even with the NEXT_HOP dedup reverted -- it is NOT the discriminator.
    log_info("waiting for the REPLAYED route 10.0.0.0/24 (injector -> Ze -> FRR)...")
    _wait_route(frr, "10.0.0.0/24")
    frr.check_route("10.0.0.0/24")
    frr.check_route_no_as(
        "10.0.0.0/24", "65001"
    )  # route server does not prepend its AS

    # Path 2: the live-relayed, split announce. THIS is the discriminating assertion: reverting
    # the buildWireModeUpdate NEXT_HOP dedup makes FRR lose 203.0.113.0/24 (re-encoded through the
    # wire-mode builder, duplicate NEXT_HOP -> treat-as-withdraw). Revert -> absent (RED).
    log_info("waiting for the LIVE-relayed split announce 203.0.113.0/24...")
    _wait_route(frr, "203.0.113.0/24")
    frr.check_route("203.0.113.0/24")

    # The withdrawn half of the mixed UPDATE was never announced, so it must not appear.
    assert not frr.has_route("198.51.100.0/24"), (
        "FRR learned 198.51.100.0/24, which the mixed UPDATE only withdrew"
    )
    log_pass("withdrawn prefix absent from FRR")

    # No NOTIFICATION from either side: Ze accepting the mixed shape on receive (Section 5.1
    # third bullet) and FRR accepting Ze's split output are both required.
    assert frr.session_established("172.30.0.2"), "FRR session dropped"
    log_pass("FRR session stable, no NOTIFICATION")


def check():
    """Entry point the runner calls. Dumps FRR state and both sidecar logs on failure,
    because a silent injector (bad expect file, refused dial) looks like a relay bug."""
    try:
        _check()
    except Exception:
        frr = FRR()
        print("--- FRR bgp table ---")
        print(frr._vtysh_quiet("show bgp ipv4 unicast"))
        print("--- FRR neighbor 172.30.0.2 ---")
        print(frr._vtysh_quiet("show bgp neighbor 172.30.0.2"))
        from interop import FRR_CONTAINER, docker_exec_quiet

        print("--- FRR log ---")
        print(docker_exec_quiet(FRR_CONTAINER, ["sh", "-c", "tail -60 /tmp/frr.log"]))
        print(docker_logs(INJECT_CONTAINER, 40))
        print(docker_logs(ZE_CONTAINER, 60))
        raise
