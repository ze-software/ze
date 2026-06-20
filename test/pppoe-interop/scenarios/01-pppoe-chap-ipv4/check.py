#!/usr/bin/env python3
"""Scenario 01: Ze PPPoE client establishes a session against accel-ppp.

Validates the full client path against a real access concentrator:
PADI/PADO/PADR/PADS discovery, LCP, CHAP-MD5 auth, IPCP address assignment,
kernel pppN interface with the server-assigned P2P address, dataplane ping to
the AC gateway through the session, the AC's own view of the session, and a
clean teardown when the client stops.
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    ZE_CONTAINER,
    accel_session_count,
    accel_show_sessions,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
    wait_accel_session,
    wait_accel_session_gone,
    wait_ppp_up,
    ze_ping,
    ze_ppp_addr,
    ze_ppp_links,
)

# accel-ppp [ip-pool]: gw-ip-address is the peer (AC) address Ze should see;
# the first pool address is what the client is assigned.
PEER_ADDR = "10.11.0.1"
LOCAL_ADDR = "10.11.0.2"


def check():
    # Gate on kernel ground truth (the pppN interface + assigned address), not
    # Info-level log markers: the ze pppoe-client brings up the interface only
    # after PPPoE discovery, LCP, CHAP-MD5 auth, and IPCP complete against
    # accel-ppp. Phase logs are surfaced via ze.log.interface=debug in the
    # container logs for diagnostics.
    iface = wait_ppp_up(timeout=75)
    log_pass("ze pppoe-client brought up a PPP interface (%s)" % iface)

    links = ze_ppp_links()
    if len(links) != 1:
        log_fail("expected exactly 1 PPP link in Ze, got %d: %s" % (len(links), links))
        raise AssertionError("unexpected PPP link count: %d" % len(links))
    log_pass("Ze has exactly 1 PPP interface (%s)" % iface)

    # The link appears a beat before AddAddressP2P applies the address; poll.
    addr_output = ""
    deadline = time.time() + 20
    while time.time() < deadline:
        addr_output = ze_ppp_addr(iface)
        if LOCAL_ADDR in addr_output and PEER_ADDR in addr_output:
            break
        time.sleep(1)
    if LOCAL_ADDR not in addr_output or PEER_ADDR not in addr_output:
        log_fail(
            "%s address mismatch: expected local %s peer %s, got: %s"
            % (iface, LOCAL_ADDR, PEER_ADDR, addr_output.strip())
        )
        raise AssertionError("PPP address mismatch on %s" % iface)
    log_pass("%s has %s peer %s" % (iface, LOCAL_ADDR, PEER_ADDR))

    # accel-ppp's own session table must show the established subscriber.
    wait_accel_session(timeout=30)
    log_pass(
        "accel-ppp shows the subscriber session:\n%s" % accel_show_sessions().rstrip()
    )

    time.sleep(2)
    if not ze_ping(PEER_ADDR, count=3):
        log_fail("Ze cannot ping AC gateway %s through the PPPoE session" % PEER_ADDR)
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("dataplane ping failed")
    log_pass("Ze can ping AC gateway %s through the PPPoE session" % PEER_ADDR)

    # Stop the Ze client; accel-ppp must drop the session (PADT / LCP-Term).
    log_info("stopping Ze client to trigger session teardown...")
    subprocess.run(
        ["docker", "stop", "-t", "5", ZE_CONTAINER],
        capture_output=True,
        text=True,
        timeout=30,
    )

    wait_accel_session_gone(timeout=30)
    log_pass(
        "accel-ppp dropped the session after client stop (count=%d)"
        % accel_session_count()
    )
