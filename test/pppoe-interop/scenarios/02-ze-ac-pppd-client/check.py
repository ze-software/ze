#!/usr/bin/env python3
"""Scenario 02: Ze as PPPoE access concentrator, pppd/rp-pppoe as the client.

Ze's AC role is what a BNG deploys, and it faces thousands of third-party CPE
stacks. This scenario takes one real client all the way through and asserts each
stage separately, so a failure names the stage that broke:

  discovery  PADI/PADO/PADR/PADS, Ze allocates a session id
  lcp        Configure-Request and Configure-Ack both directions, MRU and magic
             number agreed, and Ze's own request carries the Auth-Protocol
  auth       CHAP-MD5 accepted for the configured credential, REFUSED for a
             wrong one
  ipcp       Ze assigns an address from its pool and the client installs it
  data       ICMP crosses the session to Ze's gateway address
  teardown   pppd's PADT empties Ze's session table

The data assertion is the one that cannot be faked: discovery, LCP, auth and
IPCP can all succeed over a session that forwards nothing.

Ze's session table is read over its REST API, so what is asserted is the AC's
own state and not a log line about it.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (  # noqa: E402
    client_ping,
    client_ppp_addr,
    client_ppp_links,
    log_fail,
    log_info,
    log_pass,
    pppd_dial,
    pppd_failure_stage,
    pppd_log,
    pppd_running,
    pppd_stop,
    wait_client_ppp_up,
    wait_ze_rest_ready,
    ze_pppoe_sessions,
)

# From ze.conf, l2tp.pool.ipv4: the gateway is Ze's own side of every session,
# and the first pool address is what the first subscriber is assigned.
GATEWAY = "10.20.0.1"
FIRST_POOL_ADDR = "10.20.0.2"

USERNAME = "alice"
GOOD_PASSWORD = "s3cr3t"
BAD_PASSWORD = "wrong-secret"
SERVICE_NAME = "internet"


def require(condition, message, detail=None):
    if condition:
        log_pass(message)
        return
    log_fail(message)
    if detail:
        print(detail)
    raise AssertionError(message)


def log_line_with(log, prefix, needle):
    """True when one line of the pppd trace starts with prefix and holds needle.

    Both parts must be on the SAME line: "Ze asked for CHAP" is a claim about
    one Configure-Request, and two substrings found anywhere in the log would
    also match Ze's request plus the client's unrelated reply.
    """
    for line in log.splitlines():
        if line.startswith(prefix) and needle in line:
            return True
    return False


def wait_ze_session(timeout=45):
    deadline = time.time() + timeout
    sessions = []
    while time.time() < deadline:
        sessions = ze_pppoe_sessions()
        if sessions:
            return sessions
        if not pppd_running():
            stage = pppd_failure_stage()
            log_fail("pppd exited before Ze allocated a session -- %s" % stage)
            print(pppd_log())
            raise AssertionError("pppd exited: %s" % stage)
        time.sleep(1)
    return sessions


def wait_ze_sessions_gone(timeout=30):
    deadline = time.time() + timeout
    sessions = ze_pppoe_sessions()
    while time.time() < deadline:
        sessions = ze_pppoe_sessions()
        if not sessions:
            return True, sessions
        time.sleep(1)
    return False, sessions


def wait_client_address(iface, timeout=60):
    """Wait for the client to install the address Ze assigned over IPCP."""
    deadline = time.time() + timeout
    addr = ""
    while time.time() < deadline:
        addr = client_ppp_addr(iface)
        if FIRST_POOL_ADDR in addr and GATEWAY in addr:
            return addr
        if not pppd_running():
            stage = pppd_failure_stage()
            log_fail("pppd exited before it installed an address -- %s" % stage)
            print(pppd_log())
            raise AssertionError("pppd exited: %s" % stage)
        time.sleep(1)
    return addr


def wait_pppd_exit(timeout=60):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if not pppd_running():
            return True
        time.sleep(1)
    return False


def check_discovery():
    log_info("dialling Ze with the configured credential...")
    pppd_dial(USERNAME, GOOD_PASSWORD, service_name=SERVICE_NAME)

    sessions = wait_ze_session(timeout=45)
    require(
        len(sessions) == 1,
        "Ze allocated exactly one PPPoE session (got %d)" % len(sessions),
        pppd_log(),
    )
    session = sessions[0]
    require(
        int(session.get("sid", 0)) > 0,
        "Ze's session carries a non-zero session id (sid=%s)" % session.get("sid"),
    )
    require(
        session.get("service-name") == SERVICE_NAME,
        "Ze recorded the requested Service-Name (%r)" % session.get("service-name"),
    )
    require(
        session.get("interface") == "eth0",
        "Ze bound the session to its access interface (%r)" % session.get("interface"),
    )


def check_lcp_auth_ipcp():
    """One wait, then one read of the trace: LCP, CHAP and IPCP in order.

    The wait is for the assigned address rather than for the interface: pppd
    creates pppN when the PPPoE channel attaches, which is before LCP has even
    started, so a trace read at that moment shows nothing that follows.
    """
    iface = wait_client_ppp_up(timeout=75)
    addr = wait_client_address(iface, timeout=75)
    log = pppd_log()

    require(
        "sent [LCP ConfReq" in log and "rcvd [LCP ConfAck" in log,
        "LCP: Ze acked the client's Configure-Request",
        log,
    )
    require(
        "rcvd [LCP ConfReq" in log and "sent [LCP ConfAck" in log,
        "LCP: the client acked Ze's Configure-Request",
        log,
    )
    require(
        log_line_with(log, "rcvd [LCP ConfReq", "<mru 1492>")
        and log_line_with(log, "sent [LCP ConfReq", "<mru 1492>"),
        "LCP: MRU 1492 requested in both directions",
        log,
    )
    require(
        log_line_with(log, "rcvd [LCP ConfReq", "<magic 0x")
        and log_line_with(log, "sent [LCP ConfReq", "<magic 0x"),
        "LCP: both ends offered a magic number",
        log,
    )
    require(
        log_line_with(log, "rcvd [LCP ConfReq", "<auth chap MD5>"),
        "LCP: Ze's own Configure-Request demanded CHAP-MD5",
        log,
    )

    require("rcvd [CHAP Challenge" in log, "auth: Ze sent a CHAP-MD5 Challenge", log)
    require(
        log_line_with(log, "sent [CHAP Response", 'name = "%s"' % USERNAME),
        "auth: the client answered with a CHAP Response for %s" % USERNAME,
        log,
    )
    require("rcvd [CHAP Success" in log, "auth: Ze accepted the CHAP Response", log)

    require(
        FIRST_POOL_ADDR in addr and GATEWAY in addr,
        "ipcp: client holds %s peer %s on %s, assigned from Ze's pool"
        % (FIRST_POOL_ADDR, GATEWAY, iface),
        "ip addr: %s\n%s" % (addr.strip(), log),
    )
    return iface


def check_data(iface):
    require(
        client_ping(GATEWAY, count=3, source_iface=iface),
        "data: ICMP crosses the PPPoE session, client to %s" % GATEWAY,
        pppd_log(),
    )


def check_teardown():
    log_info("stopping pppd so it sends LCP Terminate and PADT...")
    require(pppd_stop(timeout=30), "teardown: pppd exited on SIGTERM")
    require("Sent PADT" in pppd_log(), "teardown: the client sent PADT", pppd_log())

    gone, sessions = wait_ze_sessions_gone(timeout=30)
    require(gone, "teardown: Ze's session table is empty (%r)" % sessions)
    require(not client_ppp_links(), "teardown: the client's PPP interface is gone")


def check_rejected_credential():
    log_info("dialling again with a wrong password; Ze must refuse...")
    pppd_dial(USERNAME, BAD_PASSWORD, service_name=SERVICE_NAME)

    require(
        wait_pppd_exit(timeout=60), "auth-reject: pppd exited rather than staying up"
    )
    log = pppd_log()
    require(
        "rcvd [CHAP Challenge" in log, "auth-reject: Ze challenged this dial too", log
    )
    require(
        "rcvd [CHAP Failure" in log,
        "auth-reject: Ze refused the wrong CHAP secret",
        log,
    )
    require(
        "rcvd [IPCP ConfAck" not in log,
        "auth-reject: the refused session never reached IPCP",
        log,
    )
    require(
        not client_ppp_links(), "auth-reject: the refused dial left no PPP interface"
    )

    gone, sessions = wait_ze_sessions_gone(timeout=30)
    require(gone, "auth-reject: Ze dropped the refused session (%r)" % sessions)


def check():
    wait_ze_rest_ready(timeout=60)

    check_discovery()
    iface = check_lcp_auth_ipcp()
    check_data(iface)
    check_teardown()
    check_rejected_credential()
