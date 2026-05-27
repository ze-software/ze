#!/usr/bin/env python3
"""Scenario 39: Ze import and export policy interoperates with FRR.

VALIDATES: AC-2 import local-pref, export MED/community, and export deny behavior.
PREVENTS: Policy tests passing without proving peer-visible route attributes.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD, FRR, ZE_IP, log_fail, log_pass


def _wait_bird_route(bird, prefix, timeout=30):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if bird.has_route(prefix):
            return bird._birdc_quiet("show route for %s all" % prefix)
        time.sleep(2)
    raise AssertionError("BIRD route %s not found" % prefix)


def check():
    frr = FRR()
    bird = BIRD()

    frr.wait_session(ZE_IP)
    bird.wait_session("ze_policy")

    output = _wait_bird_route(bird, "10.39.1.0/24")
    lower = output.lower()
    if "bgp.local_pref: 250" not in lower:
        log_fail("reflected route missing imported local-pref 250")
        print(output)
        raise AssertionError("import local-pref missing")
    if "bgp.med: 77" not in lower:
        log_fail("reflected route missing export MED 77")
        print(output)
        raise AssertionError("export MED missing")
    if "65000,39" not in output and "65000:39" not in output:
        log_fail("reflected route missing export community 65000:39")
        print(output)
        raise AssertionError("export community missing")
    log_pass("import local-pref and export MED/community are visible on BIRD")

    if bird.has_route("10.39.2.0/24"):
        output = bird._birdc_quiet("show route for 10.39.2.0/24 all")
        log_fail("BIRD received route denied by export prefix-list")
        print(output)
        raise AssertionError("export deny failed")
    log_pass("export prefix-list denied non-matching route")

    assert frr.session_established(ZE_IP), "FRR policy session dropped"
    assert bird.session_established("ze_policy"), "BIRD policy session dropped"
    log_pass("policy import/export scenario stable")
