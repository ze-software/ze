#!/usr/bin/env python3
"""Scenario 44: Ze BMP sender streams FRR peer events and routes.

VALIDATES: AC-7 BMP Initiation, PeerUp, and RouteMonitoring reach a collector.
PREVENTS: BMP config validating without proving wire-visible collector delivery.
"""

import json
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BMP_CONTAINER, FRR, ZE_IP, docker_exec_quiet, log_fail, log_pass


def _collector_status():
    output = docker_exec_quiet(
        BMP_CONTAINER,
        [
            "python3",
            "-c",
            "import pathlib; p=pathlib.Path('/tmp/bmp-collector.json'); print(p.read_text() if p.exists() else '')",
        ],
    )
    if not output.strip():
        return {}
    try:
        return json.loads(output)
    except json.JSONDecodeError:
        return {"error": output}


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
    frr.wait_route("10.44.0.0/24")

    deadline = time.time() + 120
    while time.time() < deadline:
        status = _collector_status()
        types = set(status.get("types", []))
        if {0, 3, 4}.issubset(types):
            log_pass("BMP collector received Initiation, PeerUp, and RouteMonitoring")
            return
        time.sleep(2)

    status = _collector_status()
    log_fail("BMP collector did not receive all required message types")
    print(status)
    raise AssertionError("BMP messages missing")
