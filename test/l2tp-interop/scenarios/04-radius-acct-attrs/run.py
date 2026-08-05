#!/usr/bin/env python3
"""Scenario 04: subscriber attributes in RADIUS auth and accounting.

Validates that ze, running as an LNS with a real xl2tpd/pppd LAC on the other
end of the tunnel, puts the operator's NAS-Port-Id (RFC 2869 Section 5.17) in
the Access-Request and in every Accounting-Request, and reports the address the
subscriber actually negotiated as Framed-IP-Address (RFC 2865 Section 5.8, RFC
2866 Section 4.1) in the accounting records.

Self-contained: the shared Scenario flow mounts only ze.conf, and this scenario
needs a third container (the RADIUS server) attached to the lab network before
the LAC brings a session up. It therefore manages its own lifecycle, and
run.py's dispatcher hands control here rather than to check.py.

The proof is what the RADIUS server received, not what ze logged: the mock
prints one decoded line per packet and the assertions below read those lines.
"""

import os
import re
import subprocess
import sys
import time

SCENARIO_DIR = os.path.dirname(os.path.abspath(__file__))
LAB_DIR = os.path.abspath(os.path.join(SCENARIO_DIR, "..", ".."))
sys.path.insert(0, LAB_DIR)

from lab import (  # noqa: E402
    NETWORK,
    ZE_CONTAINER,
    SESSION_TIMEOUT,
    Scenario,
    docker_logs_all,
    docker_rm,
    docker_run,
    log_fail,
    log_info,
    log_pass,
    preflight_strict,
    wait_ppp_up,
    wait_ze_log,
)
from run import build_images  # noqa: E402

RADIUS_CONTAINER = "ze-l2tp-radius-%s" % os.environ.get(
    "ZE_L2TP_INTEROP_SUFFIX", str(os.getpid())
)
RADIUS_IP = "172.29.0.5"
PEER_ADDR = "10.100.0.2"
NAS_ID = "lns1"


def start_radius():
    """Run the mock on the lab network, from the ze image (it carries python3).

    It must be listening before the LAC starts: ze rejects a session whose
    Access-Request goes unanswered, so a late server would present as a session
    failure rather than as a missing attribute.
    """
    docker_run(
        RADIUS_CONTAINER,
        "ze-l2tp-interop",
        RADIUS_IP,
        volumes=["%s/radius-mock.py:/radius-mock.py:ro" % SCENARIO_DIR],
        extra_args=["--entrypoint", "python3"],
        cmd=["/radius-mock.py"],
    )
    deadline = time.time() + 20
    while time.time() < deadline:
        if "radius-mock listening" in docker_logs_all(RADIUS_CONTAINER):
            log_pass("mock RADIUS server listening on %s:1812" % RADIUS_IP)
            return
        time.sleep(0.5)
    raise RuntimeError("mock RADIUS server did not start")


def radius_lines():
    return [
        line
        for line in docker_logs_all(RADIUS_CONTAINER).splitlines()
        if line.startswith("RADIUS-RX ")
    ]


def wait_radius_line(prefix, timeout=None):
    """Wait for a received-packet line of the given kind, and return it."""
    deadline = time.time() + (timeout or SESSION_TIMEOUT)
    while time.time() < deadline:
        for line in radius_lines():
            if line.startswith("RADIUS-RX %s " % prefix):
                return line
        time.sleep(1)
    log_fail("no %s reached the RADIUS server; received: %s" % (prefix, radius_lines()))
    raise AssertionError("no %s within %ss" % (prefix, timeout or SESSION_TIMEOUT))


def nas_port_id(line):
    m = re.search(r"NAS-Port-Id=(\S+)", line)
    if not m:
        log_fail("no NAS-Port-Id in: %s" % line)
        raise AssertionError("NAS-Port-Id missing from %s" % line)
    return m.group(1)


def check():
    wait_ze_log("session established", timeout=SESSION_TIMEOUT)
    wait_ze_log("session IP assigned", timeout=SESSION_TIMEOUT)
    iface = wait_ppp_up(timeout=SESSION_TIMEOUT)
    log_pass("PPP session up on %s" % iface)

    # RFC 2869 Section 5.17: NAS-Port-Id in the Access-Request. The template is
    # "{nas-id}:{tunnel-id}.{session-id}", so the text is fixed except for the
    # two identifiers the tunnel negotiated.
    access = wait_radius_line("Access-Request")
    auth_port_id = nas_port_id(access)
    if not re.fullmatch(r"%s:\d+\.\d+" % NAS_ID, auth_port_id):
        log_fail(
            "Access-Request NAS-Port-Id %r does not match the template" % auth_port_id
        )
        raise AssertionError("NAS-Port-Id template not applied: %s" % auth_port_id)
    log_pass("Access-Request carries NAS-Port-Id %s" % auth_port_id)

    acct = wait_radius_line("Accounting-Request")
    if "Acct-Status-Type=Start" not in acct:
        log_fail("first Accounting-Request is not a Start: %s" % acct)
        raise AssertionError("expected an Accounting-Start first")

    # RFC 2866 Section 4.1: the Framed-IP-Address in an Accounting-Request MUST
    # be the address actually assigned or negotiated. ze's pool starts at
    # 10.100.0.2 and the LAC's pppd accepted it, which the pppN address check
    # above already proved.
    if "Framed-IP-Address=%s" % PEER_ADDR not in acct:
        log_fail("Accounting-Start does not report %s: %s" % (PEER_ADDR, acct))
        raise AssertionError("Framed-IP-Address missing or wrong in Accounting-Start")
    log_pass("Accounting-Start reports Framed-IP-Address=%s" % PEER_ADDR)

    # One session, one port identity: a billing system joins the Access-Request
    # and the accounting records by this text.
    acct_port_id = nas_port_id(acct)
    if acct_port_id != auth_port_id:
        log_fail("NAS-Port-Id differs: auth %s, acct %s" % (auth_port_id, acct_port_id))
        raise AssertionError("NAS-Port-Id not stable across auth and accounting")
    log_pass("Accounting-Start carries the same NAS-Port-Id %s" % acct_port_id)


def main():
    preflight_strict()
    build_images(
        os.environ.get("FRR_IMAGE", "quay.io/frrouting/frr:10.3.1"),
        no_build=os.environ.get("NO_BUILD", "0") == "1",
        need_frr=False,
    )

    scenario = Scenario(SCENARIO_DIR, "")
    scenario.teardown()
    subprocess.run(
        ["docker", "network", "create", "--subnet=172.29.0.0/24", NETWORK],
        capture_output=True,
        text=True,
        timeout=30,
    )
    try:
        start_radius()
        scenario.setup()
        check()
    except BaseException as exc:  # noqa: BLE001 -- the runner reports the failure
        log_fail("FAIL: %s" % exc)
        log_info("ze log tail:")
        print(docker_logs_all(ZE_CONTAINER)[-4000:])
        log_info("RADIUS server received: %s" % radius_lines())
        return 1
    finally:
        docker_rm(RADIUS_CONTAINER)
        scenario.teardown()
    log_pass("PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
