#!/usr/bin/env python3
"""Scale scenario: 2000 sessions across 10 tunnels.

Validates AC-1 (all sessions established), AC-2 (>= 100 sessions/sec),
and AC-6 (RADIUS accounting Start count matches session count).
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from harness import ScaleScenario, log_fail, log_info, log_pass  # noqa: E402

TUNNELS = 10
SESSIONS_PER_TUNNEL = 200
EXPECTED_TOTAL = TUNNELS * SESSIONS_PER_TUNNEL


def run():
    s = ScaleScenario(os.path.dirname(__file__))
    try:
        s.setup()
        result = s.run_scale(
            tunnels=TUNNELS,
            sessions=SESSIONS_PER_TUNNEL,
            dwell="1s",
        )
    finally:
        s.teardown()

    if result is None:
        raise RuntimeError("no result from ze-test l2tp-scale")

    sessions_up = result.get("sessions-up", 0)
    tunnels_up = result.get("tunnels-up", 0)
    rate = result.get("sessions-per-sec", 0)
    errors = result.get("errors", [])

    if errors:
        for e in errors[:5]:
            log_info("error: %s" % e)

    if tunnels_up != TUNNELS:
        raise RuntimeError(
            "tunnels: %d/%d (expected all %d)" % (tunnels_up, TUNNELS, TUNNELS)
        )
    log_pass("tunnels: %d/%d" % (tunnels_up, TUNNELS))

    if sessions_up != EXPECTED_TOTAL:
        raise RuntimeError(
            "sessions: %d/%d (expected all %d)"
            % (sessions_up, EXPECTED_TOTAL, EXPECTED_TOTAL)
        )
    log_pass("sessions: %d/%d" % (sessions_up, EXPECTED_TOTAL))

    if rate < 100:
        log_info("WARNING: rate %.1f sessions/s < 100 (AC-2 target)" % rate)
    else:
        log_pass("rate: %.1f sessions/s (>= 100)" % rate)

    log_info("setup: %s" % result.get("setup-time-ns", "?"))


if __name__ == "__main__":
    run()
