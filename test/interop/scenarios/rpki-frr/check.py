#!/usr/bin/env python3
"""Scenario 43: Ze validates FRR routes through ze-test rpki RTR."""

import json
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_CONTAINER, ZE_IP, docker_exec, docker_exec_quiet, log_info, log_pass


def _wait_status(timeout=60):
    deadline = time.time() + timeout
    last = ""
    while time.time() < deadline:
        last = docker_exec_quiet(
            ZE_CONTAINER,
            [
                "python3",
                "-c",
                "import pathlib; p=pathlib.Path('/tmp/rpki-check.json'); print(p.read_text() if p.exists() else '')",
            ],
        )
        if not last.strip():
            time.sleep(1)
            continue
        data = json.loads(last)
        if data.get("status") == "ok":
            return data
        if data.get("status") == "fail":
            break
        time.sleep(1)
    raise AssertionError("RPKI check failed or timed out: %s" % last)


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)

    log_info("injecting FRR static routes after Ze plugins are ready...")
    docker_exec(
        frr.container,
        [
            "vtysh",
            "-c",
            "configure terminal",
            "-c",
            "ip route 9.43.0.0/24 Null0",
            "-c",
            "ip route 10.43.0.0/24 Null0",
            "-c",
            "ip route 11.43.0.0/24 Null0",
        ],
    )

    status = _wait_status()
    detail = status.get("detail", {})
    routes = detail.get("routes", {})
    if routes.get("9.43.0.0/24") != 1:
        raise AssertionError("valid route missing validation-state=1: %s" % routes)
    if "10.43.0.0/24" in routes:
        raise AssertionError("invalid route was installed: %s" % routes)
    if routes.get("11.43.0.0/24") != 2:
        raise AssertionError("not-found route missing validation-state=2: %s" % routes)

    log_pass("RPKI RTR session validated valid, invalid, and not-found FRR routes")
