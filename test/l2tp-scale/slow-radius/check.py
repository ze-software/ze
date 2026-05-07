#!/usr/bin/env python3
"""Scale scenario: degraded RADIUS performance.

Validates AC-related behavior under slow RADIUS: sessions still
establish (with delay) and no timeout cascade occurs. Uses 500ms
artificial RADIUS latency with a small session count.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from harness import ScaleScenario, log_info, log_pass  # noqa: E402

TUNNELS = 2
SESSIONS_PER_TUNNEL = 5
EXPECTED_TOTAL = TUNNELS * SESSIONS_PER_TUNNEL
RADIUS_DELAY = "500ms"


def run():
    s = ScaleScenario(os.path.dirname(__file__))
    try:
        s.setup()
        result = s.run_scale(
            tunnels=TUNNELS,
            sessions=SESSIONS_PER_TUNNEL,
            radius_delay=RADIUS_DELAY,
            dwell="1s",
        )
    finally:
        s.teardown()

    if result is None:
        raise RuntimeError("no result from ze-test l2tp-scale")

    sessions_up = result.get("sessions-up", 0)
    errors = result.get("errors", [])

    if errors:
        for e in errors[:5]:
            log_info("error: %s" % e)

    if sessions_up != EXPECTED_TOTAL:
        raise RuntimeError(
            "sessions: %d/%d (with %s RADIUS delay)"
            % (sessions_up, EXPECTED_TOTAL, RADIUS_DELAY)
        )

    log_pass(
        "sessions established with %s RADIUS delay: %d/%d"
        % (RADIUS_DELAY, sessions_up, EXPECTED_TOTAL)
    )

    setup_ns = result.get("setup-time-ns", 0)
    log_info("setup time: %s (elevated due to RADIUS delay)" % setup_ns)


if __name__ == "__main__":
    run()
