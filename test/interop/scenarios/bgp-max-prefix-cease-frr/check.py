#!/usr/bin/env python3
"""Scenario 45: Ze sends Cease on max-prefix exceed and recovers.

VALIDATES: AC-8 prefix maximum triggers teardown and session recovery.
PREVENTS: Max-prefix enforcement being tested only by internal counters or logs.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (
    FRR,
    ZE_CONTAINER,
    ZE_IP,
    docker_exec,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)


def _wait_prefix_limit_event(timeout=90):
    deadline = time.time() + timeout
    logs = ""
    while time.time() < deadline:
        logs = docker_logs(ZE_CONTAINER, 120).lower()
        if "prefix count exceeded maximum" in logs:
            log_pass("Ze logged prefix maximum enforcement")
            return logs
        time.sleep(1)
    log_fail("Ze logs do not show prefix maximum enforcement")
    print(logs)
    raise AssertionError("missing prefix maximum log")


def _wait_session_down(frr, timeout=30):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if not frr.session_established(ZE_IP):
            log_pass("FRR session dropped after prefix maximum was exceeded")
            return
        time.sleep(1)
    raise AssertionError("FRR session did not drop after prefix maximum was exceeded")


def check():
    frr = FRR()

    _wait_prefix_limit_event()
    _wait_session_down(frr)
    log_pass("Ze enforced prefix maximum and sent teardown")

    log_info("removing excess FRR static route before recovery...")
    docker_exec(
        frr.container,
        ["vtysh", "-c", "configure terminal", "-c", "no ip route 10.45.1.0/24 Null0"],
    )

    frr.wait_session(ZE_IP, timeout=60)
    time.sleep(5)
    assert frr.session_established(ZE_IP), (
        "session did not stay established after excess route removal"
    )
    log_pass("session recovered after idle-timeout once prefix count was within limit")
