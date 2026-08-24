#!/usr/bin/env python3
"""Scenario ipsec-bgp-redistribute-frr: IPsec tunnel routes redistributed into BGP.

Validates: Ze establishes an IKE SA and Child SA with strongSwan,
negotiates traffic selectors for the remote protected network
(10.200.0.0/24), redistributes that route into BGP, and FRR
receives it. On tunnel teardown, verifies the route is withdrawn
from FRR and the BGP session remains stable.

Topology:
  Ze (172.28.0.2, AS 65001)  <--IPsec-->  strongSwan (172.28.0.3)
  Ze (172.28.0.2, AS 65001)  <---BGP--->  FRR (172.28.0.4, AS 65002)
  strongSwan protects 10.200.0.0/24 (its local_ts).
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    ZE_CONTAINER,
    ZE_IP,
    SWAN_CONTAINER,
    FRR,
    StrongSwan,
    docker_exec_quiet,
    log_fail,
    log_info,
    log_pass,
    wait_xfrm_sa,
)

REMOTE_SITE = "10.200.0.0/24"


def wait_ze_xfrm_policy(remote_prefix, timeout=30):
    net_addr = remote_prefix.split("/")[0]
    log_info(
        "waiting for Ze XFRM policy for %s (timeout %ds)..." % (remote_prefix, timeout)
    )
    deadline = time.time() + timeout
    while time.time() < deadline:
        output = docker_exec_quiet(ZE_CONTAINER, ["ip", "xfrm", "policy"])
        if net_addr in output:
            log_pass("Ze has XFRM policy for %s" % remote_prefix)
            return
        time.sleep(2)
    log_fail(
        "Ze XFRM policy for %s did not appear within %ds" % (remote_prefix, timeout)
    )
    raise AssertionError("Ze XFRM policy for %s not found" % remote_prefix)


def check():
    swan = StrongSwan()
    frr = FRR()

    # 1. BGP session between Ze and FRR (independent of IPsec).
    frr.wait_session(ZE_IP)
    log_pass("FRR BGP session with Ze is Established")

    # 2. IKE SA and Child SA with strongSwan.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    # 3. XFRM state and policy on BOTH ends.
    #
    # These two assertions used to sit inside `except (AssertionError, Exception)`, so the
    # scenario passed whether or not Ze programmed anything. Its "expected on Docker for
    # Mac" reason is stale: scenario psk-site-to-site runs in this same lab with the
    # Ze-side XFRM SA present and ESP counters advancing on both peers, measured
    # 2026-08-01.
    wait_xfrm_sa(SWAN_CONTAINER)
    wait_xfrm_sa(ZE_CONTAINER)
    wait_ze_xfrm_policy(REMOTE_SITE)

    # 4. FRR received the route via BGP redistribute.
    frr.wait_route(REMOTE_SITE, timeout=30)
    frr.check_route(REMOTE_SITE)
    log_pass("FRR received %s via BGP" % REMOTE_SITE)

    # 5. Tear down the tunnel by stopping strongSwan.
    log_info("stopping strongSwan to trigger tunnel teardown...")
    docker_exec_quiet(SWAN_CONTAINER, ["kill", "1"], timeout=10)

    # 6. Route withdrawn from FRR.
    frr.wait_route_absent(REMOTE_SITE, timeout=30)
    log_pass("FRR no longer has %s after tunnel down" % REMOTE_SITE)

    # 7. BGP session survives the tunnel teardown.
    assert frr.session_established(ZE_IP), "BGP session dropped after tunnel down"
    log_pass("FRR BGP session stable after route withdrawal")
