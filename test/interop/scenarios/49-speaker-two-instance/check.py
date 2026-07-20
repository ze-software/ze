#!/usr/bin/env python3
"""Scenario 49: two independent speaker engines with distinct router-ids both establish with Ze
without colliding (spec-bgp-plugin-speaker AC-5).

The two speakers dial Ze from 172.30.0.10 (router-id 172.30.0.10) and 172.30.0.11 (router-id
172.30.0.11). Each reports `established: yes` on its own stdout. This proves the ExaBGP-style
per-instance router-id lets multiple engines run in one lab without one clobbering the other:
if the router-ids collided or the engines shared session state, at least one would fail to reach
Established.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    SPEAKER2_CONTAINER,
    SPEAKER_CONTAINER,
    ZE_CONTAINER,
    docker_logs,
    log_info,
    log_pass,
)


def _report(container, timeout=90):
    deadline = time.time() + timeout
    while time.time() < deadline:
        logs = docker_logs(container, 40)
        if "result:" in logs:
            return logs
        time.sleep(2)
    return docker_logs(container, 40)


def _established(logs):
    for line in logs.splitlines():
        if "established:" in line:
            return line.split("established:", 1)[1].strip() == "yes"
    return False


def _check():
    log_info("waiting for both speaker instances to establish...")
    r1 = _report(SPEAKER_CONTAINER)
    r2 = _report(SPEAKER2_CONTAINER)
    assert _established(r1), (
        "speaker1 (router-id 172.30.0.10) never reached Established"
    )
    assert _established(r2), (
        "speaker2 (router-id 172.30.0.11) never reached Established"
    )
    log_pass("both speaker instances established without colliding")


def check():
    try:
        _check()
    except Exception:
        print("--- speaker1 log ---")
        print(docker_logs(SPEAKER_CONTAINER, 40))
        print("--- speaker2 log ---")
        print(docker_logs(SPEAKER2_CONTAINER, 40))
        print("--- ze log ---")
        print(docker_logs(ZE_CONTAINER, 60))
        raise
